// Command paladin-comms consumes triage results from NATS and dispatches
// outbound notifications to Slack and/or PagerDuty.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/paladinai/paladinai/internal/logger"
	internalnats "github.com/paladinai/paladinai/internal/nats"
	commscfg "github.com/paladinai/paladinai/services/paladin-comms/config"
	"github.com/paladinai/paladinai/services/paladin-comms/internal/handler"
	"github.com/paladinai/paladinai/services/paladin-comms/internal/notifier"
	"go.uber.org/zap"
)

func main() {
	cfg, err := commscfg.Load()
	if err != nil {
		log, _ := zap.NewProduction()
		log.Fatal("config load failed", zap.Error(err))
	}

	log, err := logger.New("paladin-comms")
	if err != nil {
		boot, _ := zap.NewProduction()
		boot.Fatal("logger init failed", zap.Error(err))
	}
	defer log.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── Build multi-notifier from enabled channels ────────────────────────────
	var notifiers []notifier.Notifier
	if cfg.SlackWebhookURL != "" {
		notifiers = append(notifiers, notifier.NewSlackNotifier(cfg.SlackWebhookURL))
		log.Info("comms: slack notifier enabled")
	}
	if cfg.PagerDutyKey != "" {
		notifiers = append(notifiers, notifier.NewPagerDutyNotifier(cfg.PagerDutyKey))
		log.Info("comms: pagerduty notifier enabled")
	}
	if len(notifiers) == 0 {
		log.Warn("comms: no notifiers configured — set SLACK_WEBHOOK_URL or PAGERDUTY_ROUTING_KEY")
	}
	multi := notifier.NewMultiNotifier(notifiers...)

	// ── Handler ───────────────────────────────────────────────────────────────
	h := handler.New(multi, cfg.MinSeverity, log)

	// ── NATS ──────────────────────────────────────────────────────────────────
	natsClient, err := internalnats.Connect(cfg.NatsURL, log)
	if err != nil {
		log.Fatal("nats connect failed", zap.Error(err))
	}
	defer natsClient.Close()

	cons, err := natsClient.JS().CreateOrUpdateConsumer(ctx, internalnats.StreamAlerts, jetstream.ConsumerConfig{
		Name:          cfg.NATSConsumerName,
		Durable:       cfg.NATSConsumerName,
		FilterSubject: internalnats.SubjectAlertsTriaged,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    3,
		AckWait:       30 * time.Second,
	})
	if err != nil {
		log.Fatal("comms: create consumer failed", zap.Error(err))
	}

	msgs, err := cons.Messages(jetstream.PullMaxMessages(cfg.Workers))
	if err != nil {
		log.Fatal("comms: start message fetch failed", zap.Error(err))
	}
	defer msgs.Stop()

	// ── Health endpoint ───────────────────────────────────────────────────────
	commsPort := commsHTTPPort()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	httpSrv := &http.Server{
		Addr:         ":" + commsPort,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		log.Info("paladin-comms HTTP listening", zap.String("port", commsPort))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP server error", zap.Error(err))
		}
	}()

	log.Info("paladin-comms starting",
		zap.String("consumer", cfg.NATSConsumerName),
		zap.Int("workers", cfg.Workers),
	)

	// ── Consume loop ──────────────────────────────────────────────────────────
	sem := make(chan struct{}, cfg.Workers)
	for {
		select {
		case <-ctx.Done():
			goto shutdown
		default:
		}

		msg, err := msgs.Next()
		if err != nil {
			if errors.Is(err, jetstream.ErrMsgIteratorClosed) || errors.Is(err, context.Canceled) {
				break
			}
			log.Warn("comms: message fetch error", zap.Error(err))
			continue
		}

		sem <- struct{}{}
		go func(m jetstream.Msg) {
			defer func() { <-sem }()
			if procErr := h.ProcessMessage(ctx, m); procErr != nil {
				log.Warn("comms: process error", zap.Error(procErr))
			}
		}(msg)
	}

shutdown:
	// drain semaphore
	for i := 0; i < cfg.Workers; i++ {
		sem <- struct{}{}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP shutdown error", zap.Error(err))
	}

	log.Info("paladin-comms stopped")
}

func commsHTTPPort() string {
	if port := os.Getenv("PALADIN_COMMS_PORT"); port != "" {
		return port
	}
	return "9009"
}
