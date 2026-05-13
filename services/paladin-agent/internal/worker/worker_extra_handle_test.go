// White-box coverage: error-path log branches in handleMsg (Ack failure,
// NakWithDelay failure, DLQ publish failure).
// Must stay in package worker to call unexported handleMsg.
package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
)

// ─── fakeMsg variants ─────────────────────────────────────────────────────────

// errAckMsg is a fakeMsg whose Ack() returns an error.
type errAckMsg struct{ fakeMsg }

func (e *errAckMsg) Ack() error { e.acked = true; return errors.New("jetstream: ack failed") }

// errNakMsg is a fakeMsg whose NakWithDelay() returns an error.
type errNakMsg struct{ fakeMsg }

func (e *errNakMsg) NakWithDelay(_ time.Duration) error {
	e.nakCalled = true
	return errors.New("jetstream: nak failed")
}

// ─── DLQ publish failure ──────────────────────────────────────────────────────

// errAfterCountPublisher fails only when a specific subject is published.
type errAfterCountPublisher struct {
	calls    int
	failOnN  int
	subjects []string
}

func (p *errAfterCountPublisher) Publish(_ context.Context, subject string, _ []byte) (PublishResult, error) {
	p.calls++
	p.subjects = append(p.subjects, subject)
	if p.calls >= p.failOnN {
		return PublishResult{}, fmt.Errorf("nats: publish failed (call %d)", p.calls)
	}
	return PublishResult{}, nil
}

func TestHandleMsg_AckFailure_LogsOnly(t *testing.T) {
	// Ack returning an error must be handled gracefully (log-only path).
	pub := &fakePublisher{}
	triager := &fakeTriager{result: &agent.TriageResult{ConfirmedSeverity: "P2"}}
	w := New(triager, pub, 5*time.Second, 1, zap.NewNop())

	msg := &errAckMsg{}
	msg.payload = validEnvJSON("t1", "fp-ack-err")
	msg.subject = "paladin.alerts.correlated.t1.alertmanager"
	msg.meta = &jetstream.MsgMetadata{NumDelivered: 1}

	// Should not panic, even though Ack returns an error.
	assert.NotPanics(t, func() {
		w.handleMsg(context.Background(), msg) // pass *errAckMsg, not &msg.fakeMsg
	})
	assert.True(t, msg.acked)
}

func TestHandleMsg_NakFailureOnPublishError_LogsOnly(t *testing.T) {
	// When publish fails AND NakWithDelay returns an error, the log path must not panic.
	pub := &fakePublisher{err: errors.New("nats: publish failed")}
	triager := &fakeTriager{result: &agent.TriageResult{ConfirmedSeverity: "P3"}}
	w := New(triager, pub, 5*time.Second, 1, zap.NewNop())

	msg := &errNakMsg{}
	msg.payload = validEnvJSON("t1", "fp-nak-err")
	msg.subject = "paladin.alerts.correlated.t1.alertmanager"
	msg.meta = &jetstream.MsgMetadata{NumDelivered: 1}

	assert.NotPanics(t, func() {
		w.handleMsg(context.Background(), msg) // pass *errNakMsg, not &msg.fakeMsg
	})
	assert.True(t, msg.nakCalled)
}

func TestHandleMsg_DLQPublishFailure_TermsMsg(t *testing.T) {
	// When maxDeliveries is reached AND DLQ publish fails, the message is still termed.
	pub := &errAfterCountPublisher{failOnN: 1} // fail on first Publish call (the DLQ one)
	triager := &fakeTriager{err: errors.New("triage failed")}
	w := New(triager, pub, 5*time.Second, 1, zap.NewNop())

	msg := &fakeMsg{
		payload: validEnvJSON("t1", "fp-dlq-fail"),
		subject: "paladin.alerts.correlated.t1.alertmanager",
		meta:    &jetstream.MsgMetadata{NumDelivered: maxDeliveries},
	}

	assert.NotPanics(t, func() {
		w.handleMsg(context.Background(), msg)
	})
	assert.True(t, msg.termed, "message must be termed even when DLQ publish fails")
	assert.False(t, msg.acked)
}

// ─── NakWithDelay error path in triage-failure branch ────────────────────────

func TestHandleMsg_NakFailureOnTriageError_LogsOnly(t *testing.T) {
	pub := &fakePublisher{}
	triager := &fakeTriager{err: errors.New("triage error")}
	w := New(triager, pub, 5*time.Second, 1, zap.NewNop())

	msg := &errNakMsg{}
	msg.payload = validEnvJSON("t1", "fp-nak-triage")
	msg.subject = "paladin.alerts.correlated.t1.alertmanager"
	msg.meta = &jetstream.MsgMetadata{NumDelivered: 1} // below maxDeliveries

	assert.NotPanics(t, func() {
		w.handleMsg(context.Background(), msg) // pass *errNakMsg, not &msg.fakeMsg
	})
	assert.True(t, msg.nakCalled)
}

// ─── RCA with zero timeout uses triageTimeout ─────────────────────────────────

func TestHandleMsg_RCAZeroTimeout_FallsBackToTriageTimeout(t *testing.T) {
	pub := &fakePublisher{}
	triager := &fakeTriager{result: &agent.TriageResult{ConfirmedSeverity: "P2"}}
	rca := &fakeRCA{result: &agent.RCAResult{Confidence: "HIGH"}}

	// rcaTimeout=0 in the Worker means "use triageTimeout"
	w := New(triager, pub, 5*time.Second, 1, zap.NewNop()).
		WithRCA(rca).
		WithRCATimeout(0)

	msg := msgWithDeliveries(validEnvJSON("t1", "fp-rca-zero-to"), 1)

	assert.NotPanics(t, func() {
		w.handleMsg(context.Background(), msg)
	})
	assert.True(t, msg.acked)
	require.Len(t, pub.subjects, 1)
	assert.Contains(t, pub.subjects[0], "paladin.alerts.analyzed.")
}
