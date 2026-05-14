package agent_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/cache"
	"github.com/paladinai/paladinai/internal/qdrant"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
)

// errorL1 is an L1Cache that fails both Get and Set, exercising the
// degraded cache-tier path in CachedTriager.Triage.
type errorL1 struct{}

func (errorL1) Get(_ context.Context, _ string) ([]byte, error) {
	return nil, errors.New("valkey down")
}

func (errorL1) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return errors.New("valkey down")
}

// corruptL1 returns invalid JSON bytes from Get — exercises the unmarshal-error branch.
type corruptL1 struct{}

func (corruptL1) Get(_ context.Context, _ string) ([]byte, error) {
	return []byte("not valid json"), nil
}

func (corruptL1) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}

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

func TestCachedTriager_GetErrorFallsThroughToInner(t *testing.T) {
	inner := &countingTriager{result: &agent.TriageResult{ConfirmedSeverity: "P2"}}
	ct := agent.NewCachedTriager(inner, errorL1{}, "model-a", zap.NewNop())

	env := newEnvelope("fp-err-get", "corr-err-get")
	result, err := ct.Triage(context.Background(), env)
	require.NoError(t, err, "L1 Get error must not fail the request")
	assert.Equal(t, "P2", result.ConfirmedSeverity)
	assert.Equal(t, int64(1), inner.calls.Load(), "inner must be called when L1 errors")
}

func TestCachedTriager_CorruptCachedEntryFallsThrough(t *testing.T) {
	inner := &countingTriager{result: &agent.TriageResult{ConfirmedSeverity: "P3"}}
	ct := agent.NewCachedTriager(inner, corruptL1{}, "model-a", zap.NewNop())

	env := newEnvelope("fp-corrupt", "corr-corrupt")
	result, err := ct.Triage(context.Background(), env)
	require.NoError(t, err)
	assert.Equal(t, "P3", result.ConfirmedSeverity)
	assert.Equal(t, int64(1), inner.calls.Load(), "corrupt cache should fall through to inner")
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

func TestCachedTriager_WithRAGDelegatesToWrappedTriageAgent(t *testing.T) {
	ctx := context.Background()
	stub := &stubModel{response: `{
		"confirmed_severity": "P2",
		"summary": "High error rate on api service",
		"likely_cause": "Connection pool exhausted",
		"affected_services": ["api"],
		"recommended_action": "Follow database pool runbook",
		"needs_human": true
	}`}
	inner, err := agent.NewTriageAgent(ctx, stub, zap.NewNop())
	require.NoError(t, err)

	ct := agent.NewCachedTriager(inner, cache.NewMemL1(), "model-a", zap.NewNop())
	ct.WithRAG(agent.NewRAGContextBuilder(triageRAGRetriever{
		chunks: []qdrant.RunbookChunk{
			{Source: "db-pool-runbook", Content: "restart api pods after reducing pool size"},
		},
	}, zap.NewNop()))

	_, err = ct.Triage(ctx, &alert.AlertEnvelope{
		TenantID:    "t1",
		Fingerprint: "fp-rag-cache",
		Title:       "API 5xx spike",
		Severity:    alert.SeverityP2,
		Status:      alert.StatusFiring,
		Labels:      map[string]string{"service": "api"},
		StartsAt:    time.Now(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, stub.lastMessages)
	assert.Contains(t, stub.lastMessages[len(stub.lastMessages)-1].Content, "Relevant Runbooks")
	assert.Contains(t, stub.lastMessages[len(stub.lastMessages)-1].Content, "db-pool-runbook")
}
