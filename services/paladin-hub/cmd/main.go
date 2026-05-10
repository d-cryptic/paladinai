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
	hubcfg "github.com/paladinai/paladinai/services/paladin-hub/config"
	"github.com/paladinai/paladinai/services/paladin-hub/internal/handler"
	"github.com/paladinai/paladinai/services/paladin-hub/internal/store"
	"go.uber.org/zap"
)

func main() {
	cfg, err := hubcfg.Load()
	if err != nil {
		log, _ := zap.NewProduction()
		log.Fatal("config load failed", zap.Error(err))
	}

	log, err := logger.New("paladin-hub")
	if err != nil {
		panic(err)
	}
	defer log.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Use PostgresStore when DATABASE_URL is set; fall back to MemStore in dev.
	var s store.Store
	if cfg.DatabaseURL != "" {
		pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatal("postgres pool init failed", zap.Error(err))
		}
		defer pool.Close()

		pgStore := store.NewPostgresStore(pool)
		if err := pgStore.MigrateUp(ctx); err != nil {
			log.Fatal("postgres migration failed", zap.Error(err))
		}
		s = pgStore
		log.Info("paladin-hub using PostgresStore")
	} else {
		s = store.NewMemStore()
		log.Warn("paladin-hub using MemStore (set DATABASE_URL for production)")
	}

	h := handler.New(s, log)

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
