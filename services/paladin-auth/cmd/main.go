package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/auth"
	"github.com/paladinai/paladinai/internal/logger"
	"github.com/paladinai/paladinai/internal/telemetry"
	cfg "github.com/paladinai/paladinai/services/paladin-auth/config"
	"github.com/paladinai/paladinai/services/paladin-auth/internal/handler"
	"github.com/paladinai/paladinai/services/paladin-auth/internal/store"
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

	log := logger.Must("paladin-auth")
	defer log.Sync() //nolint:errcheck

	// Fail fast on weak secrets — better than a runtime panic later.
	if err := auth.ValidateSecret([]byte(conf.JWTSecret)); err != nil {
		return fmt.Errorf("JWT_SECRET: %w", err)
	}
	if conf.AdminSecret == "" {
		return fmt.Errorf("ADMIN_SECRET is required")
	}

	ctx := context.Background()
	otel, err := telemetry.Init(ctx, "paladin-auth", conf.Base.ServiceVersion, conf.Base.OtelEndpoint, log)
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer otel.Shutdown(ctx) //nolint:errcheck

	// Use Postgres if DATABASE_URL is set, otherwise fall back to in-memory
	// (useful for unit tests and dev without a local Postgres).
	var tenantStore store.Store
	if conf.Base.DatabaseURL != "" {
		pool, poolErr := pgxpool.New(ctx, conf.Base.DatabaseURL)
		if poolErr != nil {
			return fmt.Errorf("auth: postgres connect: %w", poolErr)
		}
		defer pool.Close()
		if pingErr := pool.Ping(ctx); pingErr != nil {
			log.Warn("postgres ping failed — falling back to in-memory store", zap.Error(pingErr))
			tenantStore = store.NewMemStore()
		} else {
			log.Info("auth: using postgres tenant store")
			tenantStore = store.NewPostgresStore(pool)
		}
	} else {
		log.Info("auth: DATABASE_URL not set — using in-memory tenant store")
		tenantStore = store.NewMemStore()
	}

	h := handler.New(tenantStore, []byte(conf.JWTSecret), conf.AdminSecret, conf.TokenTTL, log)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	r.Route("/api/v1", func(r chi.Router) {
		h.Register(r)
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

	serveErr := make(chan error, 1)
	go func() {
		log.Info("paladin-auth listening", zap.Int("port", conf.Server.Port))
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	select {
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("server: %w", err)
		}
	case <-quit:
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), conf.Server.ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
