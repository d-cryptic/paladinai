// White-box tests for Pipeline.handleMsg (unexported — package pipeline access required).
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	nats "github.com/nats-io/nats.go"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// fakeMsg is a minimal implementation of jetstream.Msg for unit testing.
type fakeMsg struct {
	data      []byte
	subject   string
	acked     bool
	termed    bool
	nakDelay  time.Duration
	nakCalled bool
	metadata  *jetstream.MsgMetadata
	metaErr   error
}

func (f *fakeMsg) Data() []byte                    { return f.data }
func (f *fakeMsg) Subject() string                 { return f.subject }
func (f *fakeMsg) Headers() nats.Header            { return nil }
func (f *fakeMsg) Reply() string                   { return "" }
func (f *fakeMsg) Ack() error                      { f.acked = true; return nil }
func (f *fakeMsg) DoubleAck(_ context.Context) error { f.acked = true; return nil }
func (f *fakeMsg) Nak() error                      { f.nakCalled = true; return nil }
func (f *fakeMsg) NakWithDelay(d time.Duration) error {
	f.nakCalled = true
	f.nakDelay = d
	return nil
}
func (f *fakeMsg) InProgress() error               { return nil }
func (f *fakeMsg) Term() error                     { f.termed = true; return nil }
func (f *fakeMsg) TermWithReason(_ string) error   { f.termed = true; return nil }
func (f *fakeMsg) Metadata() (*jetstream.MsgMetadata, error) {
	return f.metadata, f.metaErr
}

// ─── handleMsg tests ──────────────────────────────────────────────────────────

func TestHandleMsg_InvalidJSON_Terms(t *testing.T) {
	dedup := &localDedup{}
	pub := &localPub{}
	p := New(dedup, localCorr{}, pub, zap.NewNop())

	msg := &fakeMsg{data: []byte("not json"), subject: "paladin.alerts.raw.t1.alertmanager"}
	p.handleMsg(context.Background(), msg)

	assert.True(t, msg.termed, "invalid JSON must term the message (not nak — would loop)")
	assert.False(t, msg.acked)
	assert.Equal(t, 0, pub.count)
}

func TestHandleMsg_ValidAlert_Acks(t *testing.T) {
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

	assert.True(t, msg.acked, "successfully processed message must be acked")
	assert.False(t, msg.termed)
	assert.Equal(t, 1, pub.count)
}

func TestHandleMsg_ProcessError_NaksWithDelay(t *testing.T) {
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

	assert.True(t, msg.nakCalled, "process failure must nak for redelivery")
	assert.False(t, msg.termed)
	assert.Greater(t, msg.nakDelay, time.Duration(0), "nak delay must be positive")
}

// ─── Minimal local fakes (separate from pipeline_test package fakes) ─────────

type localDedup struct{}

func (d *localDedup) IsDuplicate(_ context.Context, env *alert.AlertEnvelope) (bool, error) {
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
