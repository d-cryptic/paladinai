// Package pipeline wires the alert processing stages: dedup → correlate → publish.
// The Pipeline is NATS-agnostic at its core — it consumes and produces AlertEnvelopes
// through narrow interfaces so it can be tested without a real NATS server.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/correlation"
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

// PublishResult is returned by a successful Publisher.Publish call.
// It is a value type so the Publisher interface does not leak jetstream types.
type PublishResult struct{ Sequence uint64 }

// Publisher publishes a serialised envelope to a NATS subject.
type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte) (PublishResult, error)
}

// PostProcessFunc runs after Process successfully publishes a correlated envelope.
// It is invoked synchronously inside Process; errors are intentionally not
// returned — the alert has already been published, so a hook failure must not
// trigger NATS redelivery.
type PostProcessFunc func(ctx context.Context, env *alert.AlertEnvelope)

// Pipeline processes raw alerts: dedup → correlate → publish to correlated subject.
type Pipeline struct {
	dedup       Deduplicator
	corr        Correlator
	pub         Publisher
	log         *zap.Logger
	postProcess PostProcessFunc
}

// New creates a Pipeline. All dependencies are required.
func New(dedup Deduplicator, corr Correlator, pub Publisher, log *zap.Logger) *Pipeline {
	return &Pipeline{dedup: dedup, corr: corr, pub: pub, log: log}
}

// WithPostProcess registers a hook fired after each successful publish. It is
// intended for side effects like triggering durable workflows. A nil hook clears
// any previously-registered hook. The pipeline is returned for chaining.
func (p *Pipeline) WithPostProcess(hook PostProcessFunc) *Pipeline {
	p.postProcess = hook
	return p
}

// Process applies dedup + correlation to a single AlertEnvelope and publishes it.
//
// Resolved alerts: reset dedup key, skip dedup check, then correlate (look up existing
// group only — if no group exists, a new one is created) and publish.
// Duplicate firing alerts: silently dropped.
// Infrastructure errors: fail open on dedup (don't suppress real alerts), propagate
// on correlate/publish (caller should nak for redelivery).
func (p *Pipeline) Process(ctx context.Context, env *alert.AlertEnvelope) error {
	if err := alert.ValidateTenantID(env.TenantID); err != nil {
		return fmt.Errorf("pipeline: invalid envelope tenant_id: %w", err)
	}

	env.ReceivedAt = time.Now().UTC()

	if env.Status == alert.StatusResolved {
		// Reset dedup so the alert can re-fire after resolution.
		if err := p.dedup.Reset(ctx, env.TenantID, env.Fingerprint); err != nil {
			p.log.Warn("dedup reset failed (continuing)",
				zap.String("fingerprint", env.Fingerprint),
				zap.Error(err),
			)
			// Non-fatal — still publish the resolved event
		}
	} else {
		dup, err := p.dedup.IsDuplicate(ctx, env)
		if err != nil {
			// Fail open: a Valkey outage must not suppress real alerts.
			// The caller's dedup window TTL will self-heal once Valkey recovers.
			p.log.Warn("dedup check failed, failing open",
				zap.String("fingerprint", env.Fingerprint),
				zap.Error(err),
			)
		} else if dup {
			p.log.Debug("alert deduplicated",
				zap.String("tenant", env.TenantID),
				zap.String("fingerprint", env.Fingerprint),
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

	if p.postProcess != nil {
		p.postProcess(ctx, env)
	}
	return nil
}

// Run starts a JetStream consumer on paladin.alerts.raw.> and calls Process for each message.
// It blocks until ctx is cancelled, returning ctx.Err() on clean shutdown.
// Each message is processed with a per-message timeout (25s, under AckWait of 30s).
// Transient errors use exponential nak-delay; fatal unmarshal errors term the message.
func (p *Pipeline) Run(ctx context.Context, js jetstream.JetStream, consumerName string) error {
	cons, err := js.CreateOrUpdateConsumer(ctx, "PALADIN_ALERTS", jetstream.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		FilterSubject: "paladin.alerts.raw.>",
		DeliverPolicy: jetstream.DeliverNewPolicy,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    5,
		AckWait:       30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("pipeline consumer create %s: %w", consumerName, err)
	}

	cc, err := cons.Messages()
	if err != nil {
		return fmt.Errorf("pipeline consumer messages %s: %w", consumerName, err)
	}
	var stopOnce sync.Once
	stop := func() { stopOnce.Do(cc.Stop) }
	defer stop()

	msgCh := make(chan jetstream.Msg, 64)
	go func() {
		defer close(msgCh)
		for {
			msg, err := cc.Next()
			if err != nil {
				if !errors.Is(err, jetstream.ErrMsgIteratorClosed) {
					p.log.Error("consumer iterator error", zap.Error(err))
				}
				return
			}
			select {
			case msgCh <- msg:
			case <-ctx.Done():
				// Return unprocessed message to JetStream for redelivery.
				_ = msg.Nak()
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			stop() // close iterator so the producer goroutine can exit
			for m := range msgCh {
				_ = m.Nak()
			}
			return ctx.Err()
		case msg, ok := <-msgCh:
			if !ok {
				return fmt.Errorf("pipeline: consumer stopped unexpectedly")
			}
			p.handleMsg(ctx, msg)
		}
	}
}

func (p *Pipeline) handleMsg(ctx context.Context, msg jetstream.Msg) {
	var env alert.AlertEnvelope
	if err := json.Unmarshal(msg.Data(), &env); err != nil {
		p.log.Error("alert unmarshal failed, terming message",
			zap.String("subject", msg.Subject()),
			zap.Error(err),
		)
		_ = msg.Term()
		return
	}

	// Bound processing to stay under AckWait.
	pctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	if err := p.Process(pctx, &env); err != nil {
		md, mdErr := msg.Metadata()
		var delivered uint64
		if mdErr != nil {
			p.log.Debug("msg metadata unavailable for backoff calc", zap.Error(mdErr))
		} else if md != nil {
			delivered = md.NumDelivered
		}
		// Exponential backoff capped at 60s. Clamp shift to 6 (64s > cap) to
		// prevent left-shift overflow if NumDelivered ever exceeds 63.
		shift := delivered
		if shift > 6 {
			shift = 6
		}
		delay := time.Duration(math.Min(float64(time.Duration(1<<shift)*time.Second), float64(60*time.Second)))
		p.log.Error("pipeline process error, scheduling redelivery",
			zap.String("fingerprint", env.Fingerprint),
			zap.String("tenant", env.TenantID),
			zap.String("status", string(env.Status)),
			zap.Duration("retry_delay", delay),
			zap.Error(err),
		)
		_ = msg.NakWithDelay(delay)
		return
	}

	if ackErr := msg.Ack(); ackErr != nil {
		p.log.Warn("pipeline: ack failed; message may be redelivered",
			zap.String("fingerprint", env.Fingerprint),
			zap.Error(ackErr),
		)
	}
}
