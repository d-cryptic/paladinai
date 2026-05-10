package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	cfg "github.com/paladinai/paladinai/services/paladin-edge/config"
	"github.com/paladinai/paladinai/services/paladin-edge/internal/handler"
	"github.com/paladinai/paladinai/services/paladin-edge/internal/ratelimit"

	"github.com/paladinai/paladinai/internal/logger"
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

	log := logger.Must("paladin-edge")
	defer log.Sync() //nolint:errcheck

	ctx := context.Background()
	otel, err := telemetry.Init(ctx, "paladin-edge", conf.Base.ServiceVersion, conf.Base.OtelEndpoint, log)
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer otel.Shutdown(ctx) //nolint:errcheck

	// Valkey for rate limiting
	valkeyAddr, err := parseRedisAddr(conf.Base.ValkeyURL)
	if err != nil {
		return fmt.Errorf("valkey URL: %w", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: valkeyAddr})
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		return fmt.Errorf("valkey ping: %w", err)
	}
	defer rdb.Close()

	rateLimiter := ratelimit.NewValkeyLimiter(rdb, conf.RateLimitRPS, log)
	health := &handler.HealthHandler{}

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		// Restrict in production via ALLOWED_ORIGINS env var.
		// This is an internal service — CORS is for the dashboard SPA only.
		AllowedOrigins: []string{"http://localhost:3000", "http://localhost:3001"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "X-Request-ID", "X-Tenant-ID"},
	}))
	r.Use(ratelimit.Middleware(rateLimiter, log))

	r.Get("/healthz", health.Liveness)
	r.Get("/readyz", health.Readiness)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	// API v1 — placeholder routes (filled as services are implemented)
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"message":"pong","service":"paladin-edge"}`)
		})
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", conf.Server.Port),
		Handler:      r,
		ReadTimeout:  conf.Server.ReadTimeout,
		WriteTimeout: conf.Server.WriteTimeout,
		IdleTimeout:  conf.Server.IdleTimeout,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("paladin-edge listening", zap.Int("port", conf.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	<-quit
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), conf.Server.ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func parseRedisAddr(rawURL string) (string, error) {
	if rawURL == "" {
		return "localhost:6379", nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse %q: %w", rawURL, err)
	}
	if u.Host != "" {
		return u.Host, nil
	}
	return rawURL, nil
}
