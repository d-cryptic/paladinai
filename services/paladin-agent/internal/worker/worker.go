// Package worker implements the NATS consumer that dispatches correlated alerts to agents.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
	"go.uber.org/zap"
)

// Triager is satisfied by agent.TriageAgent (and test fakes).
type Triager interface {
	Triage(ctx context.Context, env *alert.AlertEnvelope) (*agent.TriageResult, error)
}

// Worker consumes correlated alerts from NATS and dispatches them to agents.
type Worker struct {
	triager      Triager
	log          *zap.Logger
	triageTimeout time.Duration
}

// New creates a Worker with the given Triager.
func New(triager Triager, triageTimeout time.Duration, log *zap.Logger) *Worker {
	return &Worker{triager: triager, log: log, triageTimeout: triageTimeout}
}

// Run starts a JetStream consumer on paladin.alerts.correlated.> and processes each alert.
// Blocks until ctx is cancelled, returning ctx.Err() on clean shutdown.
func (w *Worker) Run(ctx context.Context, js jetstream.JetStream, consumerName string) error {
	cons, err := js.CreateOrUpdateConsumer(ctx, "PALADIN_ALERTS", jetstream.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		FilterSubject: "paladin.alerts.correlated.>",
		DeliverPolicy: jetstream.DeliverNewPolicy,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    3,
		AckWait:       60 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("worker consumer create %s: %w", consumerName, err)
	}

	cc, err := cons.Messages()
	if err != nil {
		return fmt.Errorf("worker consumer messages %s: %w", consumerName, err)
	}
	defer cc.Stop()

	msgCh := make(chan jetstream.Msg, 32)
	go func() {
		defer close(msgCh)
		for {
			msg, err := cc.Next()
			if err != nil {
				if !errors.Is(err, jetstream.ErrMsgIteratorClosed) {
					w.log.Error("worker iterator error", zap.Error(err))
				}
				return
			}
			select {
			case msgCh <- msg:
			case <-ctx.Done():
				_ = msg.Nak()
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-msgCh:
			if !ok {
				return fmt.Errorf("worker: consumer stopped unexpectedly")
			}
			w.handleMsg(ctx, msg)
		}
	}
}

// ProcessEnvelope runs the full processing pipeline for a single alert envelope.
// Exported for use in tests that bypass the NATS consumer.
func (w *Worker) ProcessEnvelope(ctx context.Context, env *alert.AlertEnvelope) error {
	_, err := w.TriageEnvelope(ctx, env)
	return err
}

// TriageEnvelope runs the triage agent on the envelope and returns the result.
// Exported for tests that need to inspect the result directly.
func (w *Worker) TriageEnvelope(ctx context.Context, env *alert.AlertEnvelope) (*agent.TriageResult, error) {
	return w.triager.Triage(ctx, env)
}

func (w *Worker) handleMsg(ctx context.Context, msg jetstream.Msg) {
	var env alert.AlertEnvelope
	if err := json.Unmarshal(msg.Data(), &env); err != nil {
		w.log.Error("worker: invalid message, terming",
			zap.String("subject", msg.Subject()),
			zap.Error(err),
		)
		_ = msg.Term()
		return
	}

	pctx, cancel := context.WithTimeout(ctx, w.triageTimeout)
	defer cancel()

	result, err := w.triager.Triage(pctx, &env)
	if err != nil {
		w.log.Error("worker: triage failed, nacking",
			zap.String("fingerprint", env.Fingerprint),
			zap.Error(err),
		)
		_ = msg.NakWithDelay(10 * time.Second)
		return
	}

	w.log.Info("worker: triage complete",
		zap.String("fingerprint", env.Fingerprint),
		zap.String("correlation_id", env.CorrelationID),
		zap.String("severity", result.ConfirmedSeverity),
		zap.String("summary", result.Summary),
		zap.Bool("needs_human", result.NeedsHuman),
	)

	_ = msg.Ack()
}
