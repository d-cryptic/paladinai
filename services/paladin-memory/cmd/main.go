// Command paladin-memory is the Stage 5 memory service.
// It exposes a gRPC API (MemoryService) over working memory (Valkey) and
// episodic memory (Postgres).
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	memoryv1 "github.com/paladinai/paladinai/gen/go/memory/v1"
	"github.com/paladinai/paladinai/internal/logger"
	"github.com/paladinai/paladinai/internal/qdrant"
	memcfg "github.com/paladinai/paladinai/services/paladin-memory/config"
	"github.com/paladinai/paladinai/services/paladin-memory/internal/handler"
	"github.com/paladinai/paladinai/services/paladin-memory/internal/store"
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
		panic(err)
	}
	defer log.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("valkey ping failed", zap.Error(err))
	}
	working := store.NewRedisWorkingStore(rdb)

	workingTTL := time.Duration(cfg.WorkingTTL) * time.Second
	h := handler.New(working, episodic, workingTTL, log)

	// Procedural memory (Qdrant) is optional — wired only when QDRANT_URL is set.
	if cfg.QdrantURL != "" {
		qc := qdrant.New(cfg.QdrantURL, cfg.QdrantAPIKey, log)
		if err := qc.EnsureCollection(ctx, qdrant.RunbookCollection, qdrant.EmbeddingDim); err != nil {
			log.Warn("qdrant ensure collection failed; procedural memory disabled",
				zap.String("url", cfg.QdrantURL),
				zap.Error(err),
			)
		} else {
			idx := qdrant.NewIndexer(qc, &qdrant.StubEmbedder{})
			h.WithProcedural(idx)
			log.Info("procedural memory enabled (qdrant)",
				zap.String("url", cfg.QdrantURL),
				zap.String("collection", qdrant.RunbookCollection),
			)
		}
	}

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatal("grpc listen failed", zap.String("addr", cfg.GRPCAddr), zap.Error(err))
	}

	grpcServer := grpc.NewServer()
	memoryv1.RegisterMemoryServiceServer(grpcServer, h)

	go func() {
		log.Info("paladin-memory listening",
			zap.String("grpc_addr", cfg.GRPCAddr),
			zap.String("db", sanitizeDSN(cfg.DatabaseURL)),
		)
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("grpc serve error", zap.Error(err))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

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
	log.Info("paladin-memory stopped")
}
