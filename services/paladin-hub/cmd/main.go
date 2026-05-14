// Command paladin-hub is the PaladinAI MCP server registry.
// Agents query the registry to discover available tools/servers for a tenant.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/paladinai/paladinai/internal/logger"
	"github.com/paladinai/paladinai/internal/qdrant"
	hubcfg "github.com/paladinai/paladinai/services/paladin-hub/config"
	"github.com/paladinai/paladinai/services/paladin-hub/internal/handler"
	"github.com/paladinai/paladinai/services/paladin-hub/internal/store"
	"go.uber.org/zap"
)

// sanitizeDSN returns only the host+dbname from a DSN for safe logging.
func sanitizeDSN(dsn string) string {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return "<invalid DSN>"
	}
	return fmt.Sprintf("%s/%s", cfg.ConnConfig.Host, cfg.ConnConfig.Database)
}

func main() {
	cfg, err := hubcfg.Load()
	if err != nil {
		log, _ := zap.NewProduction()
		log.Fatal("config load failed", zap.Error(err))
	}

	log, err := logger.New("paladin-hub")
	if err != nil {
		boot, _ := zap.NewProduction()
		boot.Fatal("logger init failed", zap.Error(err))
	}
	defer log.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Use PostgresStore when DATABASE_URL is set; fall back to MemStore in dev.
	var s store.Store
	if cfg.DatabaseURL != "" {
		pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			// Log only sanitized host/db — never the full DSN which may contain credentials.
			log.Fatal("postgres pool init failed",
				zap.String("db", sanitizeDSN(cfg.DatabaseURL)),
				zap.Error(err),
			)
		}
		defer pool.Close()

		// Verify connectivity before proceeding — pgxpool.New is lazy.
		if err := pool.Ping(ctx); err != nil {
			log.Fatal("postgres ping failed",
				zap.String("db", sanitizeDSN(cfg.DatabaseURL)),
				zap.Error(err),
			)
		}

		pgStore := store.NewPostgresStore(pool)
		if err := pgStore.MigrateUp(ctx); err != nil {
			log.Fatal("postgres migration failed", zap.Error(err))
		}
		s = pgStore
		log.Info("paladin-hub using PostgresStore",
			zap.String("db", sanitizeDSN(cfg.DatabaseURL)),
		)
	} else {
		s = store.NewMemStore()
		log.Warn("paladin-hub using MemStore (set DATABASE_URL for production)")
	}

	h := handler.New(s, log)

	// Wire runbook indexer: use real Qdrant when QDRANT_URL is set.
	var rbHandler *handler.RunbookHandler
	{
		var embedder qdrant.Embedder
		qdrantURL := os.Getenv("QDRANT_URL")
		openrouterKey := os.Getenv("OPENROUTER_API_KEY")
		embedModel := os.Getenv("EMBED_MODEL")
		if embedModel == "" {
			embedModel = "BAAI/bge-m3"
		}
		llmGateway := os.Getenv("LLM_GATEWAY_URL")
		if llmGateway == "" {
			llmGateway = "https://openrouter.ai/api/v1"
		}

		if openrouterKey != "" && !qdrant.IsMockGatewayURL(llmGateway) {
			embedder = qdrant.NewHTTPEmbedder(llmGateway, openrouterKey, embedModel, qdrant.EmbeddingDim)
			log.Info("paladin-hub using HTTPEmbedder", zap.String("model", embedModel))
		} else {
			embedder = &qdrant.StubEmbedder{}
			log.Warn("paladin-hub using StubEmbedder (set OPENROUTER_API_KEY and non-mock LLM_GATEWAY_URL for real embeddings)")
		}

		var pointStore qdrant.PointStore
		if qdrantURL != "" {
			pointStore = qdrant.NewClient(qdrantURL, os.Getenv("QDRANT_API_KEY"), log)
			log.Info("paladin-hub using Qdrant", zap.String("url", qdrantURL))
		} else {
			pointStore = &qdrant.NoopClient{}
			log.Warn("paladin-hub using NoopClient (set QDRANT_URL for real vector search)")
		}

		indexer := qdrant.NewIndexer(pointStore, embedder)
		rbHandler = handler.NewRunbookHandler(indexer, log)
	}

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Route("/api/v1", func(r chi.Router) {
		h.Routes(r)
		rbHandler.RunbookRoutes(r)
	})

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		log.Info("paladin-hub listening", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", zap.Error(err))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", zap.Error(err))
	}
	log.Info("paladin-hub stopped")
}
