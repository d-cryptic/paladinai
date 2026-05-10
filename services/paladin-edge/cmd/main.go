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
	"github.com/paladinai/paladinai/services/paladin-edge/internal/middleware"
	"github.com/paladinai/paladinai/services/paladin-edge/internal/proxy"
	"github.com/paladinai/paladinai/services/paladin-edge/internal/ratelimit"

	"github.com/paladinai/paladinai/internal/auth"
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

	// JWT_SECRET must be at least 32 bytes for HS256. Fail fast on misconfiguration
	// rather than silently registering a broken middleware.
	jwtSecret := []byte(conf.JWTSecret)
	if err := auth.ValidateSecret(jwtSecret); err != nil {
		return fmt.Errorf("JWT_SECRET: %w", err)
	}

	rateLimiter := ratelimit.NewValkeyLimiter(rdb, conf.RateLimitRPS, log)
	health := &handler.HealthHandler{}

	// Build the hub proxy. When HUB_URL is empty we still register the routes
	// so that the router is complete; they return 502 until the hub is configured.
	hubProxy, err := buildProxy(conf.HubURL, "/api/v1/mcp", log)
	if err != nil {
		return fmt.Errorf("hub proxy: %w", err)
	}

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://localhost:3000", "http://localhost:3001"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "X-Request-ID", "X-Tenant-ID"},
	}))
	r.Use(ratelimit.Middleware(rateLimiter, log))

	r.Get("/healthz", health.Liveness)
	r.Get("/readyz", health.Readiness)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	r.Route("/api/v1", func(r chi.Router) {
		// Unauthenticated routes carry the global 30 s timeout.
		r.Group(func(r chi.Router) {
			r.Use(chimiddleware.Timeout(30 * time.Second))
			r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintln(w, `{"message":"pong","service":"paladin-edge"}`)
			})
		})

		// JWT-protected routes. Timeout is intentionally omitted here so that
		// long-lived proxy responses (SSE, streaming JSON-RPC) are not cut short.
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTMiddleware(jwtSecret, log))

			// MCP server registry — proxied to paladin-hub.
			// /api/v1/mcp/* → paladin-hub /api/v1/*  (prefix stripped in Director)
			r.Mount("/mcp", hubProxy)
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

// buildProxy constructs a reverse-proxy Handler for a backend service.
// When rawURL is empty, a stub that always returns 502 is returned so routes
// are still registered and return a clear error rather than 404.
// When rawURL is non-empty it is validated (must be http/https with a host).
func buildProxy(rawURL, prefix string, log *zap.Logger) (http.Handler, error) {
	if rawURL == "" {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			//nolint:errcheck
			fmt.Fprint(w, `{"error":{"code":"BACKEND_NOT_CONFIGURED","message":"backend not configured"}}`)
		}), nil
	}
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", rawURL, err)
	}
	if err := proxy.Validate(target); err != nil {
		return nil, err
	}
	return proxy.New(target, prefix, log), nil
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
