package agent_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/cache"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
)

// countingTriager counts how many times Triage is called.
type countingTriager struct {
	calls  atomic.Int64
	result *agent.TriageResult
	err    error
}

func (t *countingTriager) Triage(_ context.Context, _ *alert.AlertEnvelope) (*agent.TriageResult, error) {
	t.calls.Add(1)
	return t.result, t.err
}

func newEnvelope(fingerprint, corrID string) *alert.AlertEnvelope {
	return &alert.AlertEnvelope{
		Fingerprint:   fingerprint,
		CorrelationID: corrID,
		Title:         "Test alert",
		Severity:      alert.SeverityP2,
		ReceivedAt:    time.Now(),
	}
}

func TestCachedTriager_CacheMissCallsInner(t *testing.T) {
	inner := &countingTriager{result: &agent.TriageResult{ConfirmedSeverity: "P2"}}
	ct := agent.NewCachedTriager(inner, cache.NewMemL1(), "model-a", zap.NewNop())

	env := newEnvelope("fp1", "corr1")
	result, err := ct.Triage(context.Background(), env)
	require.NoError(t, err)
	assert.Equal(t, "P2", result.ConfirmedSeverity)
	assert.Equal(t, int64(1), inner.calls.Load())
}

func TestCachedTriager_CacheHitSkipsInner(t *testing.T) {
	inner := &countingTriager{result: &agent.TriageResult{ConfirmedSeverity: "P2"}}
	ct := agent.NewCachedTriager(inner, cache.NewMemL1(), "model-a", zap.NewNop())

	env := newEnvelope("fp-hit", "corr-hit")
	// First call — miss.
	_, err := ct.Triage(context.Background(), env)
	require.NoError(t, err)
	// Second call — should hit cache.
	result, err := ct.Triage(context.Background(), env)
	require.NoError(t, err)
	assert.Equal(t, "P2", result.ConfirmedSeverity)
	assert.Equal(t, int64(1), inner.calls.Load(), "inner should only be called once")
}

func TestCachedTriager_DifferentEnvelopesAreDifferentKeys(t *testing.T) {
	inner := &countingTriager{result: &agent.TriageResult{ConfirmedSeverity: "P3"}}
	ct := agent.NewCachedTriager(inner, cache.NewMemL1(), "model-a", zap.NewNop())

	ctx := context.Background()
	_, err := ct.Triage(ctx, newEnvelope("fp-a", "corr-a"))
	require.NoError(t, err)
	_, err = ct.Triage(ctx, newEnvelope("fp-b", "corr-b"))
	require.NoError(t, err)
	assert.Equal(t, int64(2), inner.calls.Load(), "different envelopes should each call inner")
}

func TestCachedTriager_InnerErrorPropagated(t *testing.T) {
	inner := &countingTriager{err: assert.AnError}
	ct := agent.NewCachedTriager(inner, cache.NewMemL1(), "model-a", zap.NewNop())

	_, err := ct.Triage(context.Background(), newEnvelope("fp-err", "corr-err"))
	assert.ErrorIs(t, err, assert.AnError)
}

func TestCachedTriager_ModelIDChangesKey(t *testing.T) {
	inner := &countingTriager{result: &agent.TriageResult{ConfirmedSeverity: "P2"}}
	l1 := cache.NewMemL1()
	ctA := agent.NewCachedTriager(inner, l1, "model-a", zap.NewNop())
	ctB := agent.NewCachedTriager(inner, l1, "model-b", zap.NewNop())

	env := newEnvelope("fp-model", "corr-model")
	ctx := context.Background()
	_, err := ctA.Triage(ctx, env)
	require.NoError(t, err)
	_, err = ctB.Triage(ctx, env)
	require.NoError(t, err)
	assert.Equal(t, int64(2), inner.calls.Load(), "different models = different keys")
}
