// Command paladin-orchestrator consumes raw alerts from NATS, runs the
// dedup → correlate pipeline, and publishes correlated envelopes for agents.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/correlation"
	"github.com/paladinai/paladinai/internal/dedup"
	"github.com/paladinai/paladinai/internal/logger"
	internalnats "github.com/paladinai/paladinai/internal/nats"
	"github.com/paladinai/paladinai/internal/pipeline"
	"github.com/paladinai/paladinai/internal/workflow"
	orchestratorcfg "github.com/paladinai/paladinai/services/paladin-orchestrator/config"
)

// triagePostProcess returns a pipeline post-process hook that fires the durable
// triage workflow for each successfully published alert. Failures are logged but
// never bubble up — the alert is already on the wire, so workflow trigger errors
// must not cause NATS redelivery.
func triagePostProcess(trigger workflow.Trigger, log *zap.Logger) pipeline.PostProcessFunc {
	return func(ctx context.Context, env *alert.AlertEnvelope) {
		// Use correlation ID as the incident ID until the incident service mints
		// its own identifiers (Stage 6).
		runID, err := trigger.TriggerTriage(ctx, workflow.TriagePayload{
			TenantID:      env.TenantID,
			IncidentID:    env.CorrelationID,
			Fingerprint:   env.Fingerprint,
			Severity:      string(env.Severity),
			CorrelationID: env.CorrelationID,
			Labels:        env.Labels,
		})
		switch {
		case err != nil:
			log.Warn("workflow trigger failed (non-fatal)",
				zap.String("fingerprint", env.Fingerprint),
				zap.String("tenant", env.TenantID),
				zap.Error(err),
			)
		case runID != "":
			log.Info("triage workflow triggered",
				zap.String("run_id", runID),
				zap.String("correlation_id", env.CorrelationID),
			)
		}
	}
}

// natsPublisher adapts internalnats.Client to pipeline.Publisher.
type natsPublisher struct{ client *internalnats.Client }

func (p *natsPublisher) Publish(ctx context.Context, subject string, data []byte) (pipeline.PublishResult, error) {
	ack, err := p.client.Publish(ctx, subject, data)
	if err != nil {
		return pipeline.PublishResult{}, err
	}
	if ack == nil {
		return pipeline.PublishResult{}, nil
	}
	return pipeline.PublishResult{Sequence: ack.Sequence}, nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := orchestratorcfg.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	log := logger.Must("paladin-orchestrator")
	defer func() { _ = log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── NATS ─────────────────────────────────────────────────────────────────
	natsClient, err := internalnats.Connect(cfg.NatsURL, log)
	if err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	defer natsClient.Close()

	// ── Valkey (Redis-compatible) ─────────────────────────────────────────────
	// redis.ParseURL handles scheme, auth, TLS, and DB index correctly.
	rdbOpts, err := redis.ParseURL(cfg.ValkeyURL)
	if err != nil {
		return fmt.Errorf("valkey URL: %w", err)
	}
	rdb := redis.NewClient(rdbOpts)
	defer rdb.Close()

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if _, err := rdb.Ping(pingCtx).Result(); err != nil {
		return fmt.Errorf("valkey ping: %w", err)
	}

	// ── Pipeline: dedup → correlate → publish ─────────────────────────────────
	deduplicator := dedup.New(dedup.NewValkeyStore(rdb), log)
	correlator := correlation.New(correlation.NewValkeyStore(rdb), log)
	pub := &natsPublisher{client: natsClient}

	// ── Workflow trigger (Hatchet) ────────────────────────────────────────────
	// When HATCHET_API_KEY is unset the orchestrator uses a no-op trigger so
	// Hatchet is effectively disabled in local/dev environments.
	var wfTrigger workflow.Trigger = workflow.NoopClient{}
	if cfg.HatchetAPIKey != "" {
		wfTrigger = workflow.NewClient(cfg.HatchetURL, cfg.HatchetAPIKey, log)
		log.Info("hatchet workflows enabled", zap.String("url", cfg.HatchetURL))
	} else {
		log.Info("hatchet not configured, using noop workflow trigger")
	}

	pipe := pipeline.New(deduplicator, correlator, pub, log).
		WithPostProcess(triagePostProcess(wfTrigger, log))

	log.Info("paladin-orchestrator starting",
		zap.String("consumer", cfg.ConsumerName),
		zap.Int("workers", cfg.Workers),
	)

	if err := pipe.Run(ctx, natsClient.JS(), cfg.ConsumerName); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Info("paladin-orchestrator stopped")
			return nil
		}
		return fmt.Errorf("pipeline: %w", err)
	}

	log.Info("paladin-orchestrator stopped")
	return nil
}
