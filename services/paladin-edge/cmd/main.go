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
	sharedmiddleware "github.com/paladinai/paladinai/internal/middleware"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	otel, err := telemetry.Init(ctx, "paladin-edge", conf.Base.ServiceVersion, conf.Base.OtelEndpoint, log)
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer func() {
		if err := otel.ShutdownWithTimeout(telemetry.DefaultShutdownTimeout); err != nil {
			log.Warn("otel shutdown failed", zap.Error(err))
		}
	}()

	// Valkey for rate limiting
	valkeyAddr, err := parseRedisAddr(conf.Base.ValkeyURL)
	if err != nil {
		return fmt.Errorf("valkey URL: %w", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: valkeyAddr})
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		return fmt.Errorf("valkey ping: %w", err)
	}
	defer rdb.Close() //nolint:errcheck

	// JWT_SECRET must be at least 32 bytes for HS256. Fail fast on misconfiguration
	// rather than silently registering a broken middleware.
	jwtSecret := []byte(conf.JWTSecret)
	if err := auth.ValidateSecret(jwtSecret); err != nil {
		return fmt.Errorf("JWT_SECRET: %w", err)
	}

	// OPA sidecar — optional; fail-open when OPA_URL is unset (dev/test convenience).
	var opaClient middleware.OPAClient
	opaEnabled := false
	if opaURL := os.Getenv("OPA_URL"); opaURL != "" {
		opaClient = middleware.NewHTTPOPAClient(opaURL)
		opaEnabled = true
		log.Info("opa: policy enforcement enabled", zap.String("url", opaURL))
	}

	rateLimiter := ratelimit.NewValkeyLimiter(rdb, conf.RateLimitRPS, log)
	health := &handler.HealthHandler{}

	// Build backend proxies. When a URL is empty, routes return 502 so the
	// router is always complete (avoids 404 confusion in dev).
	hubProxy, err := buildProxy(conf.HubURL, "/api/v1/mcp", "/api/v1/mcp", log)
	if err != nil {
		return fmt.Errorf("hub proxy: %w", err)
	}
	runbookProxy, err := buildProxy(conf.HubURL, "/api/v1/runbooks", "/api/v1/runbooks", log)
	if err != nil {
		return fmt.Errorf("runbook proxy: %w", err)
	}
	incidentProxy, err := buildProxy(conf.AgentURL, "/api/v1/incidents", "/api/v1/incidents", log)
	if err != nil {
		return fmt.Errorf("incident proxy: %w", err)
	}
	ingestProxy, err := buildProxy(conf.IngestURL, "/api/v1/ingest", "/api/v1/ingest", log)
	if err != nil {
		return fmt.Errorf("ingest proxy: %w", err)
	}
	authProxy, err := buildProxy(conf.AuthURL, "/api/v1/auth", "/api/v1", log)
	if err != nil {
		return fmt.Errorf("auth proxy: %w", err)
	}

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(sharedmiddleware.Observability("paladin-edge", log))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: conf.AllowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "X-Request-ID", "X-Tenant-ID"},
	}))
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

		// Auth routes — unauthenticated (auth manages its own admin secret).
		// /api/v1/auth/* → paladin-auth /api/v1/*
		r.Group(func(r chi.Router) {
			r.Use(chimiddleware.Timeout(30 * time.Second))
			r.Mount("/auth", authProxy)
		})

		// JWT-protected routes. Timeout is intentionally omitted here so that
		// long-lived proxy responses (SSE, streaming JSON-RPC) are not cut short.
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTMiddleware(jwtSecret, log))
			r.Use(ratelimit.Middleware(rateLimiter, log))
			if opaEnabled {
				r.Use(middleware.OPAMiddleware(opaClient, true, log))
			}

			// MCP server registry — proxied to paladin-hub.
			// /api/v1/mcp/* → paladin-hub /api/v1/*  (prefix stripped in Director)
			r.Mount("/mcp", hubProxy)

			// Runbook import/search — proxied to paladin-hub.
			// /api/v1/runbooks/* → paladin-hub /api/v1/runbooks/*
			r.Mount("/runbooks", runbookProxy)

			// Alert ingest — proxied to paladin-ingest.
			// /api/v1/ingest/* → paladin-ingest /api/v1/ingest/*
			r.Mount("/ingest", ingestProxy)

			// Incident management — proxied to paladin-agent HTTP API.
			// /api/v1/incidents/* → paladin-agent /api/v1/incidents/*
			r.Mount("/incidents", incidentProxy)
		})
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", conf.Server.Port),
		Handler:      r,
		ReadTimeout:  conf.Server.ReadTimeout,
		WriteTimeout: conf.Server.WriteTimeout,
		IdleTimeout:  conf.Server.IdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("paladin-edge listening", zap.Int("port", conf.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server: %w", err)
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), conf.Server.ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// buildProxy constructs a reverse-proxy Handler for a backend service.
// When rawURL is empty, a stub that always returns 502 is returned so routes
// are still registered and return a clear error rather than 404.
// When rawURL is non-empty it is validated (must be http/https with a host).
func buildProxy(rawURL, prefix, backendPrefix string, log *zap.Logger) (http.Handler, error) {
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
	return proxy.NewWithBackendPrefix(target, prefix, backendPrefix, log), nil
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
