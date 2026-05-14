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
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/logger"
	jwtmw "github.com/paladinai/paladinai/internal/middleware"
	inats "github.com/paladinai/paladinai/internal/nats"
	"github.com/paladinai/paladinai/internal/telemetry"
	cfg "github.com/paladinai/paladinai/services/paladin-ws/config"
	wshandler "github.com/paladinai/paladinai/services/paladin-ws/internal/handler"
	"github.com/paladinai/paladinai/services/paladin-ws/internal/hub"
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

	log := logger.Must("paladin-ws")
	defer log.Sync() //nolint:errcheck

	ctx := context.Background()
	otel, err := telemetry.Init(ctx, "paladin-ws", conf.Base.ServiceVersion, conf.Base.OtelEndpoint, log)
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer func() {
		if err := otel.ShutdownWithTimeout(telemetry.DefaultShutdownTimeout); err != nil {
			log.Warn("otel shutdown failed", zap.Error(err))
		}
	}()

	natsClient, err := inats.Connect(conf.Base.NatsURL, log)
	if err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	defer natsClient.Close()

	h := hub.New()

	consumerCtx, cancelConsumer := context.WithCancel(ctx)
	if err := startNATSConsumer(consumerCtx, natsClient, conf, h, log); err != nil {
		cancelConsumer()
		return fmt.Errorf("nats consumer: %w", err)
	}

	wsH := wshandler.New(h, log)

	publicRouter := newPublicRouter(conf, log, wsH)
	publicServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", conf.Server.Port),
		Handler: publicRouter,
		// WriteTimeout is intentionally 0 for long-lived WebSocket connections.
		// Health and metrics responses complete well within IdleTimeout.
		WriteTimeout: 0,
		ReadTimeout:  conf.Server.ReadTimeout,
		IdleTimeout:  conf.Server.IdleTimeout,
	}
	adminServer := newAdminServer(conf)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("paladin-ws listening", zap.Int("port", conf.Server.Port))
		if err := publicServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()
	if adminServer != nil {
		go func() {
			log.Info("paladin-ws admin listening", zap.Int("port", conf.AdminPort))
			if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatal("admin server error", zap.Error(err))
			}
		}()
	}

	<-quit
	log.Info("shutting down")

	// Stop the NATS consumer first so no new messages arrive.
	cancelConsumer()

	// Evict all WebSocket clients so their pump goroutines send CloseGoingAway
	// and exit cleanly, instead of waiting for http.Server.Shutdown's timeout.
	h.CloseAll()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), conf.Server.ShutdownTimeout)
	defer cancel()
	if adminServer != nil {
		if err := adminServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown admin server: %w", err)
		}
	}
	if err := publicServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown public server: %w", err)
	}
	return nil
}

func newPublicRouter(conf cfg.Config, log *zap.Logger, wsH http.Handler) http.Handler {
	r := newBaseRouter()
	if adminRoutesOnPublicPort(conf) {
		mountAdminRoutes(r)
	}

	// WebSocket alerts endpoint: JWT required, tenantID derived from claims.
	r.With(jwtmw.JWTMiddleware(conf.JWTSecret, log)).
		Get("/v2/ws/alerts", wsH.ServeHTTP)
	return r
}

func newAdminServer(conf cfg.Config) *http.Server {
	if adminRoutesOnPublicPort(conf) {
		return nil
	}
	return &http.Server{
		Addr:         fmt.Sprintf(":%d", conf.AdminPort),
		Handler:      newAdminRouter(),
		ReadTimeout:  conf.Server.ReadTimeout,
		WriteTimeout: conf.Server.WriteTimeout,
		IdleTimeout:  conf.Server.IdleTimeout,
	}
}

func adminRoutesOnPublicPort(conf cfg.Config) bool {
	return conf.AdminPort == 0 || conf.AdminPort == conf.Server.Port
}

func newAdminRouter() http.Handler {
	r := newBaseRouter()
	mountAdminRoutes(r)
	return r
}

func newBaseRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	return r
}

func mountAdminRoutes(r chi.Router) {
	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)
}

// startNATSConsumer creates a durable push consumer on the alerts stream.
func startNATSConsumer(ctx context.Context, nc *inats.Client, conf cfg.Config, h *hub.Hub, log *zap.Logger) error {
	js := nc.JS()
	consumer, err := js.CreateOrUpdateConsumer(ctx, inats.StreamAlerts, jetstream.ConsumerConfig{
		Name:           conf.NATSConsumer,
		Durable:        conf.NATSConsumer,
		FilterSubjects: conf.NATSSubjects,
		AckPolicy:      jetstream.AckExplicitPolicy,
		MaxDeliver:     3,
		AckWait:        30 * time.Second,
		DeliverPolicy:  jetstream.DeliverNewPolicy,
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	broadcast := wshandler.NATSHandler(h, log)

	cc, err := consumer.Consume(func(msg jetstream.Msg) {
		if broadcast(msg.Data()) {
			msg.Ack() //nolint:errcheck
		} else {
			// Permanently bad message (malformed JSON, missing tenant_id).
			// Term prevents pointless redeliveries.
			msg.Term() //nolint:errcheck
		}
	})
	if err != nil {
		return fmt.Errorf("start consume: %w", err)
	}

	go func() {
		<-ctx.Done()
		cc.Stop()
	}()

	log.Info("nats consumer started",
		zap.Strings("subjects", conf.NATSSubjects),
		zap.String("consumer", conf.NATSConsumer),
	)
	return nil
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
}

func readyz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`)) //nolint:errcheck
}
