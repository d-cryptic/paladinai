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
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/logger"
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
	defer otel.Shutdown(ctx) //nolint:errcheck

	natsClient, err := inats.Connect(conf.Base.NatsURL, log)
	if err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	defer natsClient.Close()

	h := hub.New()

	// Start the NATS consumer in the background.
	consumerCtx, cancelConsumer := context.WithCancel(ctx)
	defer cancelConsumer()

	if err := startNATSConsumer(consumerCtx, natsClient, conf, h, log); err != nil {
		return fmt.Errorf("nats consumer: %w", err)
	}

	// HTTP router.
	wsH := wshandler.New(h, log)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"X-Tenant-ID", "Authorization"},
	}))

	r.Get("/healthz", healthz)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)
	r.Get("/v2/ws/alerts", wsH.ServeHTTP)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", conf.Server.Port),
		Handler:      r,
		ReadTimeout:  conf.Server.ReadTimeout,
		WriteTimeout: 0, // WebSocket connections are long-lived; no write timeout on HTTP layer
		IdleTimeout:  conf.Server.IdleTimeout,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("paladin-ws listening", zap.Int("port", conf.Server.Port))
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

// startNATSConsumer creates a push consumer on the alerts stream and calls
// hub.Broadcast for each message.
func startNATSConsumer(ctx context.Context, nc *inats.Client, conf cfg.Config, h *hub.Hub, log *zap.Logger) error {
	js := nc.JS()
	consumer, err := js.CreateOrUpdateConsumer(ctx, inats.StreamAlerts, jetstream.ConsumerConfig{
		Name:          conf.NATSConsumer,
		Durable:       conf.NATSConsumer,
		FilterSubject: conf.NATSSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    3,
		AckWait:       30 * time.Second,
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	broadcast := wshandler.NATSHandler(h, log)

	cc, err := consumer.Consume(func(msg jetstream.Msg) {
		broadcast(msg.Data())
		msg.Ack() //nolint:errcheck
	})
	if err != nil {
		return fmt.Errorf("start consume: %w", err)
	}

	// Stop consuming when the context is done.
	go func() {
		<-ctx.Done()
		cc.Stop()
	}()

	log.Info("nats consumer started",
		zap.String("subject", conf.NATSSubject),
		zap.String("consumer", conf.NATSConsumer),
	)
	return nil
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
}
