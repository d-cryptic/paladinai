// Package pipeline wires the alert processing stages: dedup → correlate → publish.
// The Pipeline is NATS-agnostic at its core — it consumes and produces AlertEnvelopes
// through narrow interfaces so it can be tested without a real NATS server.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/correlation"
	"go.uber.org/zap"
)

// Deduplicator is the interface satisfied by dedup.Deduplicator.
type Deduplicator interface {
	IsDuplicate(ctx context.Context, env *alert.AlertEnvelope) (bool, error)
	Reset(ctx context.Context, tenantID, fingerprint string) error
}

// Correlator is the interface satisfied by correlation.Correlator.
type Correlator interface {
	Correlate(ctx context.Context, env *alert.AlertEnvelope) error
}

// Publisher publishes a serialised envelope to a NATS subject.
type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte) (*jetstream.PubAck, error)
}

// Pipeline processes raw alerts: dedup → correlate → publish to correlated subject.
type Pipeline struct {
	dedup  Deduplicator
	corr   Correlator
	pub    Publisher
	log    *zap.Logger
}

// New creates a Pipeline. All dependencies are required.
func New(dedup Deduplicator, corr Correlator, pub Publisher, log *zap.Logger) *Pipeline {
	return &Pipeline{dedup: dedup, corr: corr, pub: pub, log: log}
}

// Process applies dedup + correlation to a single AlertEnvelope and publishes it.
// Resolved alerts bypass dedup but still get correlated and published.
// Duplicate alerts are silently dropped (not published).
// Returns an error only for infrastructure failures — caller should nack or redeliver.
func (p *Pipeline) Process(ctx context.Context, env *alert.AlertEnvelope) error {
	env.ReceivedAt = time.Now().UTC()

	if env.Status == alert.StatusResolved {
		// Reset dedup so the alert can re-fire after resolution.
		if err := p.dedup.Reset(ctx, env.TenantID, env.Fingerprint); err != nil {
			p.log.Warn("dedup reset failed (continuing)",
				zap.String("fingerprint", env.Fingerprint),
				zap.Error(err),
			)
			// Non-fatal: still process the resolved alert
		}
	} else {
		dup, err := p.dedup.IsDuplicate(ctx, env)
		if err != nil {
			p.log.Warn("dedup check failed, failing open",
				zap.String("fingerprint", env.Fingerprint),
				zap.Error(err),
			)
			// Fail open — don't suppress real alerts on store errors
		} else if dup {
			p.log.Debug("alert suppressed (duplicate)",
				zap.String("fingerprint", env.Fingerprint),
				zap.String("tenant", env.TenantID),
			)
			return nil
		}
	}

	if err := p.corr.Correlate(ctx, env); err != nil {
		return fmt.Errorf("pipeline correlate: %w", err)
	}

	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("pipeline marshal: %w", err)
	}

	subject := correlation.CorrelatedSubject(env.TenantID, string(env.Source))
	if _, err := p.pub.Publish(ctx, subject, data); err != nil {
		return fmt.Errorf("pipeline publish to %s: %w", subject, err)
	}

	p.log.Info("alert processed",
		zap.String("fingerprint", env.Fingerprint),
		zap.String("correlation_id", env.CorrelationID),
		zap.String("tenant", env.TenantID),
		zap.String("status", string(env.Status)),
	)
	return nil
}

// Run starts a JetStream consumer on paladin.alerts.raw.> and calls Process for each message.
// It blocks until ctx is cancelled. Uses a push consumer with manual ack.
func (p *Pipeline) Run(ctx context.Context, js jetstream.JetStream, consumerName string) error {
	cons, err := js.CreateOrUpdateConsumer(ctx, "PALADIN_ALERTS", jetstream.ConsumerConfig{
		Name:           consumerName,
		Durable:        consumerName,
		FilterSubject:  "paladin.alerts.raw.>",
		DeliverPolicy:  jetstream.DeliverNewPolicy,
		AckPolicy:      jetstream.AckExplicitPolicy,
		MaxDeliver:     3,
		AckWait:        30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("pipeline consumer create: %w", err)
	}

	msgCh := make(chan jetstream.Msg, 64)
	cc, err := cons.Messages()
	if err != nil {
		return fmt.Errorf("pipeline consumer messages: %w", err)
	}
	defer cc.Stop()

	go func() {
		for {
			msg, err := cc.Next()
			if err != nil {
				close(msgCh)
				return
			}
			msgCh <- msg
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-msgCh:
			if !ok {
				return nil
			}
			p.handleMsg(ctx, msg)
		}
	}
}

func (p *Pipeline) handleMsg(ctx context.Context, msg jetstream.Msg) {
	var env alert.AlertEnvelope
	if err := json.Unmarshal(msg.Data(), &env); err != nil {
		p.log.Error("pipeline: invalid message, nacking",
			zap.String("subject", msg.Subject()),
			zap.Error(err),
		)
		_ = msg.Term()
		return
	}

	if err := p.Process(ctx, &env); err != nil {
		p.log.Error("pipeline: process error, nacking for redelivery",
			zap.String("fingerprint", env.Fingerprint),
			zap.Error(err),
		)
		_ = msg.Nak()
		return
	}

	_ = msg.Ack()
}
