// Package worker implements the NATS consumer that dispatches correlated alerts to agents.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"runtime/debug"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
	"go.uber.org/zap"
)

const (
	maxDeliveries = 5
	dlqSubjectFmt = "paladin.alerts.triage.dlq.%s"
)

// Triager is satisfied by agent.TriageAgent, agent.CachedTriager, and test fakes.
type Triager = agent.Triager

// RCAAnalyzer is satisfied by agent.RCAAgent and test fakes.
type RCAAnalyzer = agent.RCAAnalyzer

// PublishResult is returned by a successful ResultPublisher.Publish call.
// It is a value type so the ResultPublisher interface does not leak jetstream types.
type PublishResult struct{ Sequence uint64 }

// ResultPublisher publishes triage results downstream.
type ResultPublisher interface {
	Publish(ctx context.Context, subject string, data []byte) (PublishResult, error)
}

// Worker consumes correlated alerts from NATS, triages them, and publishes results.
//
// Copy-safety invariant: Worker fields are interfaces, scalars, or pointer types
// with no embedded sync.Mutex or mutable maps. WithRCA relies on shallow copy.
// If you add a mutex or map directly to this struct, update WithRCA accordingly.
type Worker struct {
	triager       Triager
	rcaAnalyzer   RCAAnalyzer               // optional; nil skips RCA step
	supervisor    *agent.SupervisorPipeline // optional; when set, used in place of direct triager/rca calls
	pub           ResultPublisher
	log           *zap.Logger
	triageTimeout time.Duration
	rcaTimeout    time.Duration // 0 means use triageTimeout
	concurrency   int
}

// New creates a Worker. concurrency controls the bounded goroutine pool size.
func New(triager Triager, pub ResultPublisher, triageTimeout time.Duration, concurrency int, log *zap.Logger) *Worker {
	if concurrency <= 0 {
		concurrency = 4
	}
	return &Worker{
		triager:       triager,
		pub:           pub,
		log:           log,
		triageTimeout: triageTimeout,
		rcaTimeout:    2 * triageTimeout, // Tier C is slower; default 2× triage
		concurrency:   concurrency,
	}
}

// WithRCA returns a copy of the Worker with the RCA analyzer wired in.
// When set, ProcessEnvelope/handleMsg run RCA after triage and publish to
// paladin.alerts.analyzed.* instead of paladin.alerts.triaged.*.
func (w *Worker) WithRCA(rca RCAAnalyzer) *Worker {
	cp := *w
	cp.rcaAnalyzer = rca
	return &cp
}

// WithSupervisor returns a copy of the Worker with the Stage 3 supervisor pipeline wired in.
// When set, handleMsg and ProcessEnvelope run the supervisor (classify → route → triage/rca)
// instead of invoking the triager and RCA analyzer directly. On supervisor failure, the
// worker falls back to the direct triage path for resilience.
func (w *Worker) WithSupervisor(sp *agent.SupervisorPipeline) *Worker {
	cp := *w
	cp.supervisor = sp
	return &cp
}

// WithRCATimeout overrides the default RCA operation timeout.
func (w *Worker) WithRCATimeout(d time.Duration) *Worker {
	cp := *w
	cp.rcaTimeout = d
	return &cp
}

// Run starts a bounded pool of workers consuming from paladin.alerts.correlated.>.
// Blocks until ctx is cancelled, returning ctx.Err() on clean shutdown.
func (w *Worker) Run(ctx context.Context, js jetstream.JetStream, consumerName string) error {
	cons, err := js.CreateOrUpdateConsumer(ctx, "PALADIN_ALERTS", jetstream.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		FilterSubject: "paladin.alerts.correlated.>",
		DeliverPolicy: jetstream.DeliverNewPolicy,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    maxDeliveries,
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

	// Bounded worker pool — semaphore limits concurrency.
	sem := make(chan struct{}, w.concurrency)
	var wg sync.WaitGroup

	msgCh := make(chan jetstream.Msg, w.concurrency*2)
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
			wg.Wait() // drain in-flight handlers before returning
			return ctx.Err()
		case msg, ok := <-msgCh:
			if !ok {
				wg.Wait()
				return fmt.Errorf("worker: consumer stopped unexpectedly")
			}
			sem <- struct{}{}
			wg.Add(1)
			go func(m jetstream.Msg) {
				defer wg.Done()
				defer func() { <-sem }()
				w.handleMsg(ctx, m)
			}(msg)
		}
	}
}

func (w *Worker) handleMsg(ctx context.Context, msg jetstream.Msg) {
	// Recover from any panic in triage (e.g. nil-deref in Eino) to prevent
	// crashing the whole consumer pool.
	defer func() {
		if r := recover(); r != nil {
			w.log.Error("worker: triage panic",
				zap.Any("panic", r),
				zap.ByteString("stack", debug.Stack()),
			)
			_ = msg.Term()
		}
	}()

	var env alert.AlertEnvelope
	if err := json.Unmarshal(msg.Data(), &env); err != nil {
		w.log.Error("worker: invalid message, terming",
			zap.String("subject", msg.Subject()),
			zap.Error(err),
		)
		_ = msg.Term()
		return
	}

	if err := alert.ValidateTenantID(env.TenantID); err != nil {
		w.log.Error("worker: invalid tenant ID, terming",
			zap.String("subject", msg.Subject()),
			zap.Error(err),
		)
		_ = msg.Term()
		return
	}

	// Check delivery count for DLQ routing.
	md, _ := msg.Metadata()
	var deliveries uint64
	if md != nil {
		deliveries = md.NumDelivered
	}

	pctx, cancel := context.WithTimeout(ctx, w.triageTimeout)
	defer cancel()

	result, err := w.triager.Triage(pctx, &env)
	if err != nil {
		if deliveries >= maxDeliveries {
			w.log.Error("worker: triage failed at max deliveries, routing to DLQ",
				zap.String("fingerprint", env.Fingerprint),
				zap.Uint64("deliveries", deliveries),
				zap.Error(err),
			)
			w.publishDLQ(ctx, &env, err)
			_ = msg.Term()
			return
		}
		delay := nakDelay(deliveries)
		w.log.Warn("worker: triage failed, nacking with backoff",
			zap.String("fingerprint", env.Fingerprint),
			zap.Duration("delay", delay),
			zap.Error(err),
		)
		if nakErr := msg.NakWithDelay(delay); nakErr != nil {
			w.log.Warn("worker: NakWithDelay failed", zap.Error(nakErr))
		}
		return
	}

	// Optionally run RCA after triage. On failure, fall back to triage-only publish.
	var rcaResult *agent.RCAResult
	if w.rcaAnalyzer != nil {
		timeout := w.rcaTimeout
		if timeout <= 0 {
			timeout = w.triageTimeout
		}
		rcaCtx, rcaCancel := context.WithTimeout(ctx, timeout)
		rcaResult, err = w.rcaAnalyzer.Analyze(rcaCtx, &env, result)
		rcaCancel() // cancel immediately; do not defer (timer would run until handleMsg returns)
		if err != nil {
			w.log.Warn("worker: rca failed, publishing triage-only result",
				zap.String("fingerprint", env.Fingerprint),
				zap.Error(err),
			)
			rcaResult = nil // ensure fallback to triaged subject
		}
	}

	// Publish downstream before Acking.
	if err := w.publishCombined(ctx, &env, result, rcaResult); err != nil {
		w.log.Error("worker: publish result failed, nacking",
			zap.String("fingerprint", env.Fingerprint),
			zap.Error(err),
		)
		if nakErr := msg.NakWithDelay(nakDelay(deliveries)); nakErr != nil {
			w.log.Warn("worker: NakWithDelay failed", zap.Error(nakErr))
		}
		return
	}

	if ackErr := msg.Ack(); ackErr != nil {
		w.log.Warn("worker: Ack failed",
			zap.String("fingerprint", env.Fingerprint),
			zap.Error(ackErr),
		)
	}

	w.log.Info("worker: pipeline complete",
		zap.String("fingerprint", env.Fingerprint),
		zap.String("correlation_id", env.CorrelationID),
		zap.String("tenant", env.TenantID),
		zap.String("severity", result.ConfirmedSeverity),
		zap.Bool("needs_human", result.NeedsHuman),
		zap.Bool("rca_ran", rcaResult != nil),
	)
}

// publishCombined publishes the triage result, and the optional RCA result when available.
// When rca is non-nil: publishes to paladin.alerts.analyzed.<tenantID>.<source>
// When rca is nil:     publishes to paladin.alerts.triaged.<tenantID>.<source>
func (w *Worker) publishCombined(ctx context.Context, env *alert.AlertEnvelope, triage *agent.TriageResult, rca *agent.RCAResult) error {
	var payload []byte
	var subject string
	var err error

	if rca != nil {
		payload, err = json.Marshal(struct {
			Envelope *alert.AlertEnvelope `json:"envelope"`
			Triage   *agent.TriageResult  `json:"triage"`
			RCA      *agent.RCAResult     `json:"rca"`
		}{Envelope: env, Triage: triage, RCA: rca})
		subject = fmt.Sprintf("paladin.alerts.analyzed.%s.%s", env.TenantID, string(env.Source))
	} else {
		payload, err = json.Marshal(struct {
			Envelope *alert.AlertEnvelope `json:"envelope"`
			Triage   *agent.TriageResult  `json:"triage"`
		}{Envelope: env, Triage: triage})
		subject = fmt.Sprintf("paladin.alerts.triaged.%s.%s", env.TenantID, string(env.Source))
	}
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	if _, pubErr := w.pub.Publish(ctx, subject, payload); pubErr != nil {
		return fmt.Errorf("publish to %s: %w", subject, pubErr)
	}
	return nil
}

func (w *Worker) publishDLQ(ctx context.Context, env *alert.AlertEnvelope, triageErr error) {
	payload, _ := json.Marshal(map[string]any{
		"fingerprint":    env.Fingerprint,
		"tenant_id":      env.TenantID,
		"correlation_id": env.CorrelationID,
		"error":          triageErr.Error(),
	})
	subject := fmt.Sprintf(dlqSubjectFmt, env.TenantID)
	if _, err := w.pub.Publish(ctx, subject, payload); err != nil {
		w.log.Error("worker: DLQ publish failed", zap.Error(err))
	}
}

// nakDelay returns exponential backoff for NATS Nak: 10s, 30s, 90s, 270s, capped at 10m.
func nakDelay(deliveries uint64) time.Duration {
	const (
		base     = 10.0      // seconds
		maxDelay = 10 * 60.0 // seconds
	)
	d := base * math.Pow(3, float64(deliveries))
	if d > maxDelay {
		d = maxDelay
	}
	return time.Duration(d) * time.Second
}

// ProcessEnvelope runs the full pipeline (triage, optional RCA) for a single envelope.
// Exported for tests that bypass the NATS consumer.
func (w *Worker) ProcessEnvelope(ctx context.Context, env *alert.AlertEnvelope) error {
	triage, err := w.triager.Triage(ctx, env)
	if err != nil {
		return err
	}

	var rca *agent.RCAResult
	if w.rcaAnalyzer != nil {
		timeout := w.rcaTimeout
		if timeout <= 0 {
			timeout = w.triageTimeout
		}
		rcaCtx, rcaCancel := context.WithTimeout(ctx, timeout)
		rca, err = w.rcaAnalyzer.Analyze(rcaCtx, env, triage)
		rcaCancel()
		if err != nil {
			w.log.Warn("ProcessEnvelope: rca failed, falling back to triage-only",
				zap.String("fingerprint", env.Fingerprint),
				zap.Error(err),
			)
			rca = nil
		}
	}

	return w.publishCombined(ctx, env, triage, rca)
}

// TriageEnvelope runs only the triage agent on the envelope and returns the result.
// Exported for tests that need to inspect the result directly.
func (w *Worker) TriageEnvelope(ctx context.Context, env *alert.AlertEnvelope) (*agent.TriageResult, error) {
	return w.triager.Triage(ctx, env)
}
