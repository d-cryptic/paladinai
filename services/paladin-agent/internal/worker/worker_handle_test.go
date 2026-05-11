// White-box tests for the unexported handleMsg method.
// Must stay in package worker (not worker_test) to access handleMsg directly.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	nats "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ─── Compile-time interface assertion ────────────────────────────────────────

var _ jetstream.Msg = (*fakeMsg)(nil)

// ─── fakeMsg ─────────────────────────────────────────────────────────────────

type fakeMsg struct {
	payload   []byte
	subject   string
	acked     bool
	termed    bool
	nakCalled bool
	nakDelay  time.Duration
	meta      *jetstream.MsgMetadata
	metaErr   error
}

func (f *fakeMsg) Data() []byte                              { return f.payload }
func (f *fakeMsg) Subject() string                          { return f.subject }
func (f *fakeMsg) Reply() string                            { return "" }
func (f *fakeMsg) Headers() nats.Header                      { return nil }
func (f *fakeMsg) Ack() error                               { f.acked = true; return nil }
func (f *fakeMsg) Nak() error                               { f.nakCalled = true; return nil }
func (f *fakeMsg) NakWithDelay(d time.Duration) error       { f.nakCalled = true; f.nakDelay = d; return nil }
func (f *fakeMsg) Term() error                              { f.termed = true; return nil }
func (f *fakeMsg) TermWithReason(_ string) error            { f.termed = true; return nil }
func (f *fakeMsg) InProgress() error                        { return nil }
func (f *fakeMsg) Metadata() (*jetstream.MsgMetadata, error) { return f.meta, f.metaErr }
func (f *fakeMsg) DoubleAck(_ context.Context) error        { f.acked = true; return nil }

// ─── fakeTriager ─────────────────────────────────────────────────────────────

type fakeTriager struct {
	result *agent.TriageResult
	err    error
	panics bool
}

func (ft *fakeTriager) Triage(_ context.Context, _ *alert.AlertEnvelope) (*agent.TriageResult, error) {
	if ft.panics {
		panic("simulated triage panic")
	}
	return ft.result, ft.err
}

// ─── fakePublisher ───────────────────────────────────────────────────────────

type fakePublisher struct {
	err      error
	subjects []string
}

func (fp *fakePublisher) Publish(_ context.Context, subject string, _ []byte) (PublishResult, error) {
	if fp.err != nil {
		return PublishResult{}, fp.err
	}
	fp.subjects = append(fp.subjects, subject)
	return PublishResult{}, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func newHandleWorker(t *agent.TriageResult, triageErr error, pubErr error) *Worker {
	return New(&fakeTriager{result: t, err: triageErr}, &fakePublisher{err: pubErr}, 5*time.Second, 1, zap.NewNop())
}

func validEnvJSON(tenantID, fingerprint string) []byte {
	env := alert.AlertEnvelope{
		TenantID:    tenantID,
		Fingerprint: fingerprint,
		Source:      alert.SourceAlertmanager,
		Status:      alert.StatusFiring,
		Severity:    alert.SeverityP2,
	}
	b, _ := json.Marshal(env)
	return b
}

func msgWithDeliveries(data []byte, n uint64) *fakeMsg {
	return &fakeMsg{
		payload: data,
		subject: "paladin.alerts.correlated.t1.alertmanager",
		meta:    &jetstream.MsgMetadata{NumDelivered: n},
	}
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestHandleMsg_InvalidJSON_Terms(t *testing.T) {
	w := newHandleWorker(&agent.TriageResult{ConfirmedSeverity: "P2"}, nil, nil)
	msg := &fakeMsg{payload: []byte("not-json"), subject: "paladin.alerts.correlated.t1.alertmanager"}

	w.handleMsg(context.Background(), msg)

	assert.True(t, msg.termed, "invalid JSON should term the message")
	assert.False(t, msg.acked)
}

func TestHandleMsg_TriageSuccess_Acks(t *testing.T) {
	w := newHandleWorker(&agent.TriageResult{ConfirmedSeverity: "P3"}, nil, nil)
	msg := msgWithDeliveries(validEnvJSON("t1", "fp-ack"), 1)

	w.handleMsg(context.Background(), msg)

	assert.True(t, msg.acked, "successful triage should ack the message")
	assert.False(t, msg.termed)
	assert.False(t, msg.nakCalled)
}

func TestHandleMsg_TriageError_LowDeliveries_NaksWithBackoff(t *testing.T) {
	w := newHandleWorker(nil, errors.New("llm timeout"), nil)
	msg := msgWithDeliveries(validEnvJSON("t1", "fp-nak"), 1)

	w.handleMsg(context.Background(), msg)

	assert.True(t, msg.nakCalled, "triage error below maxDeliveries should nak")
	assert.False(t, msg.termed)
	assert.False(t, msg.acked)
	// delay for delivery=1: 10 * 3^1 = 30s
	assert.Equal(t, 30*time.Second, msg.nakDelay)
}

func TestHandleMsg_TriageError_AtMaxDeliveries_RoutesToDLQ(t *testing.T) {
	pub := &fakePublisher{}
	w := New(&fakeTriager{err: errors.New("triage failed")}, pub, 5*time.Second, 1, zap.NewNop())
	msg := msgWithDeliveries(validEnvJSON("acme", "fp-dlq"), maxDeliveries)

	w.handleMsg(context.Background(), msg)

	assert.True(t, msg.termed, "max deliveries should term after DLQ publish")
	assert.False(t, msg.acked)
	assert.False(t, msg.nakCalled)

	require.Len(t, pub.subjects, 1)
	assert.Contains(t, pub.subjects[0], "paladin.alerts.triage.dlq.acme")
}

func TestHandleMsg_PublishFailure_NaksWithBackoff(t *testing.T) {
	w := newHandleWorker(&agent.TriageResult{ConfirmedSeverity: "P2"}, nil, errors.New("nats disconnected"))
	msg := msgWithDeliveries(validEnvJSON("t1", "fp-pub-err"), 1)

	w.handleMsg(context.Background(), msg)

	assert.True(t, msg.nakCalled, "publish failure should nak")
	assert.False(t, msg.acked)
	assert.False(t, msg.termed)
}

func TestHandleMsg_PanicInTriage_Terms(t *testing.T) {
	w := New(&fakeTriager{panics: true}, &fakePublisher{}, 5*time.Second, 1, zap.NewNop())
	msg := msgWithDeliveries(validEnvJSON("t1", "fp-panic"), 1)

	// Should not panic the test itself.
	assert.NotPanics(t, func() {
		w.handleMsg(context.Background(), msg)
	})
	assert.True(t, msg.termed, "panic in triage should term the message")
	assert.False(t, msg.acked)
}

func TestHandleMsg_InvalidTenantID_Terms(t *testing.T) {
	w := newHandleWorker(&agent.TriageResult{ConfirmedSeverity: "P2"}, nil, nil)
	// Tenant IDs with spaces or special chars are invalid per alert.ValidateTenantID.
	env := alert.AlertEnvelope{TenantID: "bad tenant!", Fingerprint: "fp", Source: alert.SourceAlertmanager}
	b, _ := json.Marshal(env)
	msg := &fakeMsg{payload: b, subject: "paladin.alerts.correlated.bad.alertmanager"}

	w.handleMsg(context.Background(), msg)

	assert.True(t, msg.termed, "invalid tenant ID should term the message")
	assert.False(t, msg.acked)
}

func TestHandleMsg_NoMsgMetadata_StillNaks(t *testing.T) {
	// When Metadata() returns an error, deliveries=0; should still nak with delay.
	w := newHandleWorker(nil, errors.New("timeout"), nil)
	msg := &fakeMsg{
		payload: validEnvJSON("t1", "fp-no-meta"),
		subject: "paladin.alerts.correlated.t1.alertmanager",
		metaErr: errors.New("no metadata"),
	}

	w.handleMsg(context.Background(), msg)

	assert.True(t, msg.nakCalled)
	// delay for delivery=0: 10 * 3^0 = 10s
	assert.Equal(t, 10*time.Second, msg.nakDelay)
}

// ─── RCA path in handleMsg ────────────────────────────────────────────────────

type fakeRCA struct {
	result *agent.RCAResult
	err    error
}

func (r *fakeRCA) Analyze(_ context.Context, _ *alert.AlertEnvelope, _ *agent.TriageResult) (*agent.RCAResult, error) {
	return r.result, r.err
}

func TestHandleMsg_WithRCA_Success_UsesAnalyzedSubject(t *testing.T) {
	pub := &fakePublisher{}
	triager := &fakeTriager{result: &agent.TriageResult{ConfirmedSeverity: "P2"}}
	rca := &fakeRCA{result: &agent.RCAResult{RootCauseHypothesis: "DB exhausted", Confidence: "HIGH"}}

	w := New(triager, pub, 5*time.Second, 1, zap.NewNop()).WithRCA(rca)
	msg := msgWithDeliveries(validEnvJSON("tenant1", "fp-rca"), 1)

	w.handleMsg(context.Background(), msg)

	assert.True(t, msg.acked)
	require.Len(t, pub.subjects, 1)
	assert.Contains(t, pub.subjects[0], "paladin.alerts.analyzed.", "RCA success should use analyzed subject")
	assert.Contains(t, pub.subjects[0], "tenant1")
}

func TestHandleMsg_WithRCA_Failure_FallsBackToTriaged(t *testing.T) {
	pub := &fakePublisher{}
	triager := &fakeTriager{result: &agent.TriageResult{ConfirmedSeverity: "P3"}}
	rca := &fakeRCA{err: errors.New("rca timeout")}

	w := New(triager, pub, 5*time.Second, 1, zap.NewNop()).WithRCA(rca)
	msg := msgWithDeliveries(validEnvJSON("tenant2", "fp-rca-fail"), 1)

	w.handleMsg(context.Background(), msg)

	assert.True(t, msg.acked)
	require.Len(t, pub.subjects, 1)
	assert.Contains(t, pub.subjects[0], "paladin.alerts.triaged.", "RCA failure should fall back to triaged subject")
}

func TestWithRCATimeout_SetsTimeout(t *testing.T) {
	pub := &fakePublisher{}
	triager := &fakeTriager{result: &agent.TriageResult{ConfirmedSeverity: "P2"}}
	rca := &fakeRCA{result: &agent.RCAResult{Confidence: "LOW"}}

	w := New(triager, pub, 5*time.Second, 1, zap.NewNop()).
		WithRCA(rca).
		WithRCATimeout(30 * time.Second)

	assert.Equal(t, 30*time.Second, w.rcaTimeout)
}

// ─── nakDelay unit tests ──────────────────────────────────────────────────────

func TestNakDelay_Formula(t *testing.T) {
	cases := []struct {
		deliveries uint64
		want       time.Duration
	}{
		{0, 10 * time.Second},
		{1, 30 * time.Second},
		{2, 90 * time.Second},
		{3, 270 * time.Second},
		{4, 600 * time.Second},  // 10*81=810 → capped at 600s (10 min)
		{10, 600 * time.Second}, // way above cap
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("deliveries=%d", tc.deliveries), func(t *testing.T) {
			got := nakDelay(tc.deliveries)
			assert.Equal(t, tc.want, got)
		})
	}
}
