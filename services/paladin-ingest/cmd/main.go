package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	cfg "github.com/paladinai/paladinai/services/paladin-ingest/config"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/dedup"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/handler"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/publisher"

	"github.com/paladinai/paladinai/internal/logger"
	inats "github.com/paladinai/paladinai/internal/nats"
	"github.com/paladinai/paladinai/internal/storm"
	"github.com/paladinai/paladinai/internal/telemetry"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	conf, err := cfg.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	log := logger.Must("paladin-ingest")
	defer log.Sync() //nolint:errcheck

	// Telemetry
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	otel, err := telemetry.Init(ctx, "paladin-ingest", conf.Base.ServiceVersion, conf.Base.OtelEndpoint, log)
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer otel.Shutdown(ctx) //nolint:errcheck

	// NATS
	natsClient, err := inats.Connect(conf.Base.NatsURL, log)
	if err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	defer natsClient.Close()

	// Valkey (Redis-compatible) — parse full URL to preserve TLS, auth, and DB.
	rdbOpts, err := redis.ParseURL(conf.Base.ValkeyURL)
	if err != nil {
		// Fall back to treating the value as a bare host:port.
		rdbOpts = &redis.Options{Addr: conf.Base.ValkeyURL}
	}
	if rdbOpts.Addr == "" {
		rdbOpts.Addr = "localhost:6379"
	}
	rdb := redis.NewClient(rdbOpts)
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		return fmt.Errorf("valkey ping: %w", err)
	}
	defer rdb.Close() //nolint:errcheck

	// Wire dependencies
	pub := publisher.New(natsClient, log)
	ded := dedup.New(dedup.NewValkeyStore(rdb), log)
	stormDet := storm.New(&valkeyStormStore{rdb: rdb}, log)
	webhooks := handler.NewWebhookHandler(pub, ded, log).WithStormDetector(stormDet)
	health := handler.NewHealthHandler("paladin-ingest",
		handler.NewNATSChecker(natsClient.Conn()),
		handler.NewValkeyChecker(rdb),
	)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"*"},
	}))

	r.Get("/healthz", health.Liveness)
	r.Get("/readyz", health.Readiness)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)
	r.Mount("/webhook", webhooks.Routes())

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", conf.Server.Port),
		Handler:      r,
		ReadTimeout:  conf.Server.ReadTimeout,
		WriteTimeout: conf.Server.WriteTimeout,
		IdleTimeout:  conf.Server.IdleTimeout,
	}

	// Graceful shutdown
	go func() {
		log.Info("paladin-ingest listening", zap.Int("port", conf.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), conf.Server.ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// valkeyStormStore adapts *redis.Client to storm.Store.
type valkeyStormStore struct{ rdb *redis.Client }

func (v *valkeyStormStore) Incr(ctx context.Context, key string) (int64, error) {
	return v.rdb.Incr(ctx, key).Result()
}

func (v *valkeyStormStore) ExpireNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return v.rdb.ExpireNX(ctx, key, ttl).Result()
}

