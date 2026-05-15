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
	"github.com/paladinai/paladinai/services/paladin-agent/internal/incident"
	"go.uber.org/zap"
)

const (
	maxDeliveries        = 5
	defaultOperationWait = 60 * time.Second
	ackWaitMargin        = 15 * time.Second
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
	incidents     *incident.Store           // optional; records successfully processed alert groups
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

// WithIncidentStore returns a copy of the Worker that records successfully
// processed envelopes to the agent incident API store.
func (w *Worker) WithIncidentStore(store *incident.Store) *Worker {
	cp := *w
	cp.incidents = store
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
		AckWait:       w.ackWait(),
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
			if !acquireSlot(ctx, sem) {
				_ = msg.Nak()
				wg.Wait()
				return ctx.Err()
			}
			wg.Add(1)
			go func(m jetstream.Msg) {
				defer wg.Done()
				defer func() { <-sem }()
				w.handleMsg(ctx, m)
			}(msg)
		}
	}
}

func acquireSlot(ctx context.Context, sem chan<- struct{}) bool {
	select {
	case sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
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

	pctx, cancel := context.WithTimeout(ctx, w.operationTimeout())
	defer cancel()

	result, rcaResult, err := w.process(pctx, &env)
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

	// Publish downstream before Acking.
	if err := w.publishCombined(pctx, &env, result, rcaResult); err != nil {
		w.log.Error("worker: publish result failed, nacking",
			zap.String("fingerprint", env.Fingerprint),
			zap.Error(err),
		)
		if nakErr := msg.NakWithDelay(nakDelay(deliveries)); nakErr != nil {
			w.log.Warn("worker: NakWithDelay failed", zap.Error(nakErr))
		}
		return
	}
	w.recordIncident(&env, result, rcaResult)

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
	var subject string
	var err error

	if rca != nil {
		subject, err = analyzedSubject(env.TenantID, env.Source)
	} else {
		subject, err = triagedSubject(env.TenantID, env.Source)
	}
	if err != nil {
		return fmt.Errorf("build result subject: %w", err)
	}

	payload, err := json.Marshal(workerResultPayload{Envelope: env, Triage: triage, RCA: rca})
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	if _, pubErr := w.pub.Publish(ctx, subject, payload); pubErr != nil {
		return fmt.Errorf("publish to %s: %w", subject, pubErr)
	}
	return nil
}

type workerResultPayload struct {
	Envelope *alert.AlertEnvelope `json:"envelope"`
	Triage   *agent.TriageResult  `json:"triage"`
	RCA      *agent.RCAResult     `json:"rca,omitempty"`
}

func (w *Worker) publishDLQ(ctx context.Context, env *alert.AlertEnvelope, triageErr error) {
	payload, _ := json.Marshal(map[string]any{
		"fingerprint":    env.Fingerprint,
		"tenant_id":      env.TenantID,
		"correlation_id": env.CorrelationID,
		"error":          triageErr.Error(),
	})
	subject, err := triageDLQSubject(env.TenantID)
	if err != nil {
		w.log.Error("worker: invalid DLQ subject", zap.Error(err))
		return
	}
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
	pctx, cancel := context.WithTimeout(ctx, w.operationTimeout())
	defer cancel()

	triage, rca, err := w.process(pctx, env)
	if err != nil {
		return err
	}
	if err := w.publishCombined(pctx, env, triage, rca); err != nil {
		return err
	}
	w.recordIncident(env, triage, rca)
	return nil
}

func (w *Worker) recordIncident(env *alert.AlertEnvelope, triage *agent.TriageResult, rca *agent.RCAResult) {
	if w.incidents == nil || env == nil || triage == nil {
		return
	}
	payload, err := json.Marshal(struct {
		Triage *agent.TriageResult `json:"triage"`
		RCA    *agent.RCAResult    `json:"rca,omitempty"`
	}{Triage: triage, RCA: rca})
	if err != nil {
		w.log.Warn("worker: incident record marshal failed",
			zap.String("fingerprint", env.Fingerprint),
			zap.Error(err),
		)
		return
	}
	w.incidents.RecordFromEnvelope(env, triage.ConfirmedSeverity, payload)
}

func (w *Worker) process(ctx context.Context, env *alert.AlertEnvelope) (*agent.TriageResult, *agent.RCAResult, error) {
	if w.supervisor != nil {
		state, err := w.supervisor.Process(ctx, &agent.IncidentState{
			TenantID: env.TenantID,
			Alert:    *env,
		})
		if err == nil {
			if state.TriageResult == nil {
				return nil, nil, errors.New("supervisor: missing triage result")
			}
			return state.TriageResult, state.RCAResult, nil
		}
		w.log.Warn("supervisor pipeline failed, falling back to direct triage",
			zap.String("fingerprint", env.Fingerprint),
			zap.Error(err),
		)
	}

	triageCtx, triageCancel := context.WithTimeout(ctx, w.effectiveTriageTimeout())
	triage, err := w.triager.Triage(triageCtx, env)
	triageCancel()
	if err != nil {
		return nil, nil, err
	}

	var rca *agent.RCAResult
	if w.rcaAnalyzer != nil {
		timeout := w.effectiveRCATimeout()
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

	return triage, rca, nil
}

func (w *Worker) effectiveTriageTimeout() time.Duration {
	if w.triageTimeout > 0 {
		return w.triageTimeout
	}
	return defaultOperationWait
}

func (w *Worker) effectiveRCATimeout() time.Duration {
	if w.rcaTimeout > 0 {
		return w.rcaTimeout
	}
	return w.effectiveTriageTimeout()
}

func (w *Worker) operationTimeout() time.Duration {
	timeout := w.effectiveTriageTimeout()
	if w.rcaAnalyzer != nil || w.supervisor != nil {
		timeout += w.effectiveRCATimeout()
	}
	return timeout
}

func (w *Worker) ackWait() time.Duration {
	wait := w.operationTimeout() + ackWaitMargin
	if wait < defaultOperationWait {
		return defaultOperationWait
	}
	return wait
}

// TriageEnvelope runs only the triage agent on the envelope and returns the result.
// Exported for tests that need to inspect the result directly.
func (w *Worker) TriageEnvelope(ctx context.Context, env *alert.AlertEnvelope) (*agent.TriageResult, error) {
	return w.triager.Triage(ctx, env)
}
