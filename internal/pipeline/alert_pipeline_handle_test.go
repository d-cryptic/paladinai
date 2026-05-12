// White-box tests for Pipeline.handleMsg (unexported — package pipeline access required).
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	nats "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// Compile-time assertion: fakeMsg must implement the full jetstream.Msg interface.
var _ jetstream.Msg = (*fakeMsg)(nil)

// fakeMsg is a minimal implementation of jetstream.Msg for unit testing.
// Mutating fields are guarded so the Run test (which observes them from a
// separate goroutine) is race-free under -race.
type fakeMsg struct {
	data     []byte
	subject  string
	metadata *jetstream.MsgMetadata
	metaErr  error

	mu        sync.Mutex
	acked     bool
	termed    bool
	nakDelay  time.Duration
	nakCalled bool
}

func (f *fakeMsg) Data() []byte         { return f.data }
func (f *fakeMsg) Subject() string      { return f.subject }
func (f *fakeMsg) Headers() nats.Header { return nil }
func (f *fakeMsg) Reply() string        { return "" }
func (f *fakeMsg) Ack() error {
	f.mu.Lock()
	f.acked = true
	f.mu.Unlock()
	return nil
}
func (f *fakeMsg) DoubleAck(_ context.Context) error { return f.Ack() }
func (f *fakeMsg) Nak() error {
	f.mu.Lock()
	f.nakCalled = true
	f.mu.Unlock()
	return nil
}
func (f *fakeMsg) NakWithDelay(d time.Duration) error {
	f.mu.Lock()
	f.nakCalled = true
	f.nakDelay = d
	f.mu.Unlock()
	return nil
}
func (f *fakeMsg) InProgress() error { return nil }
func (f *fakeMsg) Term() error {
	f.mu.Lock()
	f.termed = true
	f.mu.Unlock()
	return nil
}
func (f *fakeMsg) TermWithReason(_ string) error { return f.Term() }
func (f *fakeMsg) Metadata() (*jetstream.MsgMetadata, error) {
	return f.metadata, f.metaErr
}

func (f *fakeMsg) isAcked() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.acked
}

func (f *fakeMsg) isTermed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.termed
}

func (f *fakeMsg) isNakCalled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.nakCalled
}

func (f *fakeMsg) nakDelayObserved() time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.nakDelay
}

// ─── handleMsg tests ──────────────────────────────────────────────────────────

func TestHandleMsg_InvalidJSON_Terms(t *testing.T) {
	t.Parallel()
	dedup := &localDedup{}
	pub := &localPub{}
	p := New(dedup, localCorr{}, pub, zap.NewNop())

	msg := &fakeMsg{data: []byte("not json"), subject: "paladin.alerts.raw.t1.alertmanager"}
	p.handleMsg(context.Background(), msg)

	assert.True(t, msg.isTermed(), "invalid JSON must term the message (not nak — would loop)")
	assert.False(t, msg.isAcked())
	assert.Equal(t, 0, pub.count)
}

func TestHandleMsg_ValidAlert_Acks(t *testing.T) {
	t.Parallel()
	dedup := &localDedup{}
	pub := &localPub{}
	p := New(dedup, localCorr{}, pub, zap.NewNop())

	env := alert.AlertEnvelope{
		TenantID:    "t1",
		Fingerprint: "fp1",
		Source:      alert.SourceAlertmanager,
		Status:      alert.StatusFiring,
		Labels:      map[string]string{"namespace": "prod"},
		StartsAt:    time.Now(),
	}
	data, _ := json.Marshal(env)
	msg := &fakeMsg{data: data}
	p.handleMsg(context.Background(), msg)

	assert.True(t, msg.isAcked(), "successfully processed message must be acked")
	assert.False(t, msg.isTermed())
	assert.Equal(t, 1, pub.count)
}

func TestHandleMsg_ProcessError_NaksWithDelay(t *testing.T) {
	t.Parallel()
	dedup := &localDedup{}
	pub := &localPub{err: errors.New("publish failed")}
	p := New(dedup, localCorr{}, pub, zap.NewNop())

	env := alert.AlertEnvelope{
		TenantID:    "t1",
		Fingerprint: "fp1",
		Source:      alert.SourceAlertmanager,
		Status:      alert.StatusFiring,
		StartsAt:    time.Now(),
	}
	data, _ := json.Marshal(env)
	// Provide metadata so the backoff delay can be computed.
	msg := &fakeMsg{
		data:     data,
		metadata: &jetstream.MsgMetadata{NumDelivered: 1},
	}
	p.handleMsg(context.Background(), msg)

	assert.True(t, msg.isNakCalled(), "process failure must nak for redelivery")
	assert.False(t, msg.isTermed())
	// NumDelivered=1 → shift=1 → 1<<1 * Second = 2s.
	assert.Equal(t, 2*time.Second, msg.nakDelayObserved(), "backoff for delivery 1 must be 2s")
}

func TestHandleMsg_MetadataError_StillNaksWithDelay(t *testing.T) {
	t.Parallel()
	dedup := &localDedup{}
	pub := &localPub{err: errors.New("publish failed")}
	p := New(dedup, localCorr{}, pub, zap.NewNop())

	env := alert.AlertEnvelope{
		TenantID:    "t1",
		Fingerprint: "fp-meta-err",
		Source:      alert.SourceAlertmanager,
		Status:      alert.StatusFiring,
		StartsAt:    time.Now(),
	}
	data, _ := json.Marshal(env)
	// metaErr causes delivered to stay 0 → shift=0 → delay=1s
	msg := &fakeMsg{
		data:    data,
		metaErr: errors.New("no metadata available"),
	}
	p.handleMsg(context.Background(), msg)

	assert.True(t, msg.isNakCalled(), "process failure with metadata error must still nak")
	assert.False(t, msg.isTermed())
	// NumDelivered=0 → shift=0 → 1<<0 * Second = 1s.
	assert.Equal(t, time.Second, msg.nakDelayObserved())
}

func TestHandleMsg_HighDeliveryCount_ClampsShift(t *testing.T) {
	t.Parallel()
	dedup := &localDedup{}
	pub := &localPub{err: errors.New("publish failed")}
	p := New(dedup, localCorr{}, pub, zap.NewNop())

	env := alert.AlertEnvelope{
		TenantID:    "t1",
		Fingerprint: "fp-high-deliver",
		Source:      alert.SourceAlertmanager,
		Status:      alert.StatusFiring,
		StartsAt:    time.Now(),
	}
	data, _ := json.Marshal(env)
	// NumDelivered=10 > 6 → clamp shift to 6 → 1<<6 = 64s → capped at 60s
	msg := &fakeMsg{
		data:     data,
		metadata: &jetstream.MsgMetadata{NumDelivered: 10},
	}
	p.handleMsg(context.Background(), msg)

	assert.True(t, msg.isNakCalled())
	assert.Equal(t, 60*time.Second, msg.nakDelayObserved(), "shift clamped at 6 → 64s > 60s cap → 60s")
}

// ─── Minimal local fakes (separate from pipeline_test package fakes) ─────────

type localDedup struct{}

func (d *localDedup) IsDuplicate(_ context.Context, _ *alert.AlertEnvelope) (bool, error) {
	return false, nil
}
func (d *localDedup) Reset(_ context.Context, _, _ string) error { return nil }

type localCorr struct{}

func (c localCorr) Correlate(_ context.Context, env *alert.AlertEnvelope) error {
	env.CorrelationID = "corr-local"
	return nil
}

type localPub struct {
	count int
	err   error
}

func (p *localPub) Publish(_ context.Context, _ string, _ []byte) (PublishResult, error) {
	if p.err != nil {
		return PublishResult{}, p.err
	}
	p.count++
	return PublishResult{Sequence: uint64(p.count)}, nil
}
