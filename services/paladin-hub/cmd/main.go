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

	// In production, swap MemStore for a Postgres-backed store.
	s := store.NewMemStore()
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
