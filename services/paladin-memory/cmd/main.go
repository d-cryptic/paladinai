// Command paladin-memory is the Stage 5 memory service.
// It exposes a gRPC API (MemoryService) over working memory (Valkey) and
// episodic memory (Postgres).
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	memoryv1 "github.com/paladinai/paladinai/gen/go/memory/v1"
	"github.com/paladinai/paladinai/internal/logger"
	sharedmiddleware "github.com/paladinai/paladinai/internal/middleware"
	"github.com/paladinai/paladinai/internal/qdrant"
	"github.com/paladinai/paladinai/internal/telemetry"
	"github.com/paladinai/paladinai/internal/topology"
	memcfg "github.com/paladinai/paladinai/services/paladin-memory/config"
	"github.com/paladinai/paladinai/services/paladin-memory/internal/handler"
	"github.com/paladinai/paladinai/services/paladin-memory/internal/store"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// sanitizeDSN returns only the host+dbname from a Postgres DSN for safe logging.
func sanitizeDSN(dsn string) string {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return "<invalid DSN>"
	}
	return fmt.Sprintf("%s/%s", cfg.ConnConfig.Host, cfg.ConnConfig.Database)
}

func main() {
	cfg, err := memcfg.Load()
	if err != nil {
		boot, _ := zap.NewProduction()
		boot.Fatal("config load failed", zap.Error(err))
	}

	log, err := logger.New("paladin-memory")
	if err != nil {
		boot, _ := zap.NewProduction()
		boot.Fatal("logger init failed", zap.Error(err))
	}
	defer log.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	otel, err := telemetry.Init(ctx, "paladin-memory", cfg.Base.ServiceVersion, cfg.Base.OtelEndpoint, log)
	if err != nil {
		log.Fatal("telemetry init failed", zap.Error(err))
	}
	defer func() {
		if err := otel.ShutdownWithTimeout(telemetry.DefaultShutdownTimeout); err != nil {
			log.Warn("otel shutdown failed", zap.Error(err))
		}
	}()

	// Postgres pool — episodic memory.
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("postgres pool init failed",
			zap.String("db", sanitizeDSN(cfg.DatabaseURL)),
			zap.Error(err),
		)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatal("postgres ping failed",
			zap.String("db", sanitizeDSN(cfg.DatabaseURL)),
			zap.Error(err),
		)
	}
	episodic := store.NewPostgresEpisodicStore(pool)

	// Valkey client — working memory.
	redisOpts, err := redis.ParseURL(cfg.ValkeyURL)
	if err != nil {
		log.Fatal("valkey url parse failed", zap.Error(err))
	}
	rdb := redis.NewClient(redisOpts)
	defer rdb.Close() //nolint:errcheck
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("valkey ping failed", zap.Error(err))
	}
	working := store.NewRedisWorkingStore(rdb)

	workingTTL := time.Duration(cfg.WorkingTTL) * time.Second
	h := handler.New(working, episodic, workingTTL, log)

	// Procedural + semantic memory (Qdrant) — optional, wired when QDRANT_URL is set.
	if cfg.QdrantURL != "" {
		qc := qdrant.New(cfg.QdrantURL, cfg.QdrantAPIKey, log)

		embedder, err := memoryEmbedder(cfg, log)
		if err != nil {
			log.Fatal("qdrant embedder config failed", zap.Error(err))
		}

		baseIdx := qdrant.NewIndexer(qc, embedder)

		if err := qc.EnsureCollection(ctx, qdrant.RunbookCollection, qdrant.EmbeddingDim); err != nil {
			log.Warn("qdrant: procedural collection unavailable",
				zap.String("collection", qdrant.RunbookCollection),
				zap.Error(err),
			)
		} else {
			h.WithProcedural(baseIdx)
			log.Info("procedural memory enabled (qdrant)",
				zap.String("url", cfg.QdrantURL),
				zap.String("collection", qdrant.RunbookCollection),
			)
		}

		if err := qc.EnsureCollection(ctx, qdrant.SemanticCollection, qdrant.EmbeddingDim); err != nil {
			log.Warn("qdrant: semantic collection unavailable",
				zap.String("collection", qdrant.SemanticCollection),
				zap.Error(err),
			)
		} else {
			h.WithSemantic(baseIdx.WithCollection(qdrant.SemanticCollection))
			log.Info("semantic memory enabled (qdrant)",
				zap.String("url", cfg.QdrantURL),
				zap.String("collection", qdrant.SemanticCollection),
			)
		}
	}

	// Topology memory (FalkorDB) — optional, wired when FALKORDB_URL is set.
	if cfg.FalkorDBURL != "" {
		fdbOpts, fdbErr := redis.ParseURL(cfg.FalkorDBURL)
		if fdbErr != nil {
			log.Warn("topology: invalid FALKORDB_URL, skipping", zap.Error(fdbErr))
		} else {
			fdb := redis.NewClient(fdbOpts)
			defer fdb.Close() //nolint:errcheck
			if pingErr := fdb.Ping(ctx).Err(); pingErr != nil {
				log.Warn("topology: falkordb ping failed, skipping", zap.Error(pingErr))
				_ = fdb.Close()
			} else {
				graphName := fmt.Sprintf("topology:%s", "global")
				topoStore := topology.NewFalkorDBStore(fdb, graphName)
				h.WithTopology(topoStore)
				log.Info("topology memory enabled (falkordb)", zap.String("url", cfg.FalkorDBURL))
			}
		}
	}
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatal("grpc listen failed", zap.String("addr", cfg.GRPCAddr), zap.Error(err))
	}

	grpcServer := grpc.NewServer()
	memoryv1.RegisterMemoryServiceServer(grpcServer, h)

	httpSrv := memoryHealthServer(cfg.HTTPAddr, pool, rdb, log)
	serverErr := make(chan error, 2)
	go func() {
		log.Info("paladin-memory HTTP listening", zap.String("addr", cfg.HTTPAddr))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("http server: %w", err)
		}
	}()

	go func() {
		log.Info("paladin-memory listening",
			zap.String("grpc_addr", cfg.GRPCAddr),
			zap.String("http_addr", cfg.HTTPAddr),
			zap.String("db", sanitizeDSN(cfg.DatabaseURL)),
		)
		if err := grpcServer.Serve(lis); err != nil {
			serverErr <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	var runErr error
	select {
	case runErr = <-serverErr:
	case <-ctx.Done():
	}

	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP shutdown error", zap.Error(err))
	}
	cancel()

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		log.Warn("graceful stop timed out; forcing")
		grpcServer.Stop()
	}
	if runErr != nil {
		log.Error("paladin-memory stopped after server error", zap.Error(runErr))
		_ = log.Sync()
		os.Exit(1)
	}
	log.Info("paladin-memory stopped")
}

func memoryEmbedder(cfg *memcfg.Config, log *zap.Logger) (qdrant.Embedder, error) {
	gatewayURL := os.Getenv("LLM_GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "https://openrouter.ai/api/v1"
	}
	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" && !qdrant.IsMockGatewayURL(gatewayURL) {
		embedModel := os.Getenv("EMBED_MODEL")
		if embedModel == "" {
			embedModel = "BAAI/bge-m3"
		}
		log.Info("using HTTP embedder", zap.String("model", embedModel))
		return qdrant.NewHTTPEmbedder(gatewayURL, apiKey, embedModel, qdrant.EmbeddingDim), nil
	}

	if stubEmbedderAllowed(cfg) {
		log.Warn("using deterministic stub embedder",
			zap.String("env", cfg.Env),
			zap.Bool("explicit_allow", cfg.AllowStubEmbedder),
		)
		return &qdrant.StubEmbedder{}, nil
	}

	return nil, fmt.Errorf("OPENROUTER_API_KEY is required when QDRANT_URL is set outside development/test")
}

func stubEmbedderAllowed(cfg *memcfg.Config) bool {
	if cfg.AllowStubEmbedder {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Env)) {
	case "", "development", "dev", "test", "local":
		return true
	default:
		return false
	}
}

func memoryHealthServer(addr string, pool *pgxpool.Pool, rdb *redis.Client, log *zap.Logger) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      sharedmiddleware.Observability("paladin-memory", log)(memoryHealthHandler(pool.Ping, func(ctx context.Context) error { return rdb.Ping(ctx).Err() })),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func memoryHealthHandler(pgPing func(context.Context) error, valkeyPing func(context.Context) error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pgPing(ctx); err != nil {
			http.Error(w, "postgres unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := valkeyPing(ctx); err != nil {
			http.Error(w, "valkey unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/metrics", promhttp.Handler())
	return mux
}
