package pipeline_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ─── In-memory fakes ─────────────────────────────────────────────────────────

type memDedup struct {
	mu       sync.Mutex
	seen     map[string]bool
	dedupErr error
	resetErr error
}

func newMemDedup() *memDedup { return &memDedup{seen: make(map[string]bool)} }

func (m *memDedup) IsDuplicate(_ context.Context, env *alert.AlertEnvelope) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dedupErr != nil {
		return false, m.dedupErr
	}
	k := env.TenantID + ":" + env.Fingerprint
	if m.seen[k] {
		return true, nil
	}
	m.seen[k] = true
	return false, nil
}

func (m *memDedup) Reset(_ context.Context, tenantID, fingerprint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.resetErr != nil {
		return m.resetErr
	}
	delete(m.seen, tenantID+":"+fingerprint)
	return nil
}

type memCorrelator struct{ err error }

func (c memCorrelator) Correlate(_ context.Context, env *alert.AlertEnvelope) error {
	if c.err != nil {
		return c.err
	}
	env.CorrelationID = "corr-test"
	return nil
}

type memPublisher struct {
	mu       sync.Mutex
	messages []publishedMsg
	err      error
}

type publishedMsg struct {
	subject string
	data    []byte
}

func (p *memPublisher) Publish(_ context.Context, subject string, data []byte) (pipeline.PublishResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return pipeline.PublishResult{}, p.err
	}
	p.messages = append(p.messages, publishedMsg{subject: subject, data: data})
	return pipeline.PublishResult{Sequence: uint64(len(p.messages))}, nil
}

func (p *memPublisher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.messages)
}

func (p *memPublisher) last() publishedMsg {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.messages[len(p.messages)-1]
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func newTestEnv(tenantID, fingerprint string, status alert.Status) *alert.AlertEnvelope {
	return &alert.AlertEnvelope{
		TenantID:    tenantID,
		Fingerprint: fingerprint,
		Source:      alert.SourceAlertmanager,
		Status:      status,
		Labels:      map[string]string{"namespace": "prod", "job": "api"},
		StartsAt:    time.Now(),
	}
}

func newPipeline() (*memDedup, *memPublisher, *pipeline.Pipeline) {
	dedup := newMemDedup()
	pub := &memPublisher{}
	p := pipeline.New(dedup, memCorrelator{}, pub, zap.NewNop())
	return dedup, pub, p
}

// ─── Tests — happy paths ──────────────────────────────────────────────────────

func TestPipeline_FirstAlertIsPublished(t *testing.T) {
	t.Parallel()
	_, pub, p := newPipeline()
	ctx := context.Background()

	env := newTestEnv("t1", "fp1", alert.StatusFiring)
	require.NoError(t, p.Process(ctx, env))

	assert.Equal(t, 1, pub.count(), "first alert should be published")
	assert.Equal(t, "corr-test", env.CorrelationID)
}

func TestPipeline_DuplicateAlertIsSuppressed(t *testing.T) {
	t.Parallel()
	_, pub, p := newPipeline()
	ctx := context.Background()

	env1 := newTestEnv("t1", "fp1", alert.StatusFiring)
	env2 := newTestEnv("t1", "fp1", alert.StatusFiring)

	require.NoError(t, p.Process(ctx, env1))
	require.NoError(t, p.Process(ctx, env2))

	assert.Equal(t, 1, pub.count(), "duplicate should be suppressed")
}

func TestPipeline_ResolvedAlertBypassesDedup(t *testing.T) {
	t.Parallel()
	dedup, pub, p := newPipeline()
	ctx := context.Background()

	// Mark fp1 as seen
	firing := newTestEnv("t1", "fp1", alert.StatusFiring)
	require.NoError(t, p.Process(ctx, firing))
	require.Equal(t, 1, pub.count())

	// Resolved should still publish even though fp1 is in dedup
	resolved := newTestEnv("t1", "fp1", alert.StatusResolved)
	require.NoError(t, p.Process(ctx, resolved))
	assert.Equal(t, 2, pub.count(), "resolved alert should always be published")

	// Dedup key should be cleared — next firing should publish
	refiring := newTestEnv("t1", "fp1", alert.StatusFiring)
	require.NoError(t, p.Process(ctx, refiring))
	assert.Equal(t, 3, pub.count(), "re-fire after resolve should publish")
	_ = dedup
}

func TestPipeline_PublishSubjectContainsTenantAndSource(t *testing.T) {
	t.Parallel()
	_, pub, p := newPipeline()
	ctx := context.Background()

	env := newTestEnv("acme", "fp-sub", alert.StatusFiring)
	require.NoError(t, p.Process(ctx, env))

	require.Equal(t, 1, pub.count())
	msg := pub.last()
	assert.Contains(t, msg.subject, "acme", "subject should contain tenant ID")
	assert.Contains(t, msg.subject, "alertmanager", "subject should contain source")
	assert.Contains(t, msg.subject, "paladin.alerts.correlated.", "subject should use correlated prefix")
}

func TestPipeline_InvalidSourceSubjectReturnsErrorBeforePublish(t *testing.T) {
	t.Parallel()
	_, pub, p := newPipeline()
	ctx := context.Background()

	env := newTestEnv("acme", "fp-bad-source", alert.StatusFiring)
	env.Source = alert.Source("bad.source")

	err := p.Process(ctx, env)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "correlated subject")
	assert.Equal(t, 0, pub.count())
}

func TestPipeline_DifferentTenantsAreIndependent(t *testing.T) {
	t.Parallel()
	_, pub, p := newPipeline()
	ctx := context.Background()

	envA := newTestEnv("tenant-a", "fp1", alert.StatusFiring)
	envB := newTestEnv("tenant-b", "fp1", alert.StatusFiring) // same fingerprint, different tenant

	require.NoError(t, p.Process(ctx, envA))
	require.NoError(t, p.Process(ctx, envB))

	assert.Equal(t, 2, pub.count(), "same fingerprint under different tenants should both publish")
}

func TestPipeline_CorrelationIDIsAssigned(t *testing.T) {
	t.Parallel()
	_, _, p := newPipeline()
	ctx := context.Background()

	env := newTestEnv("t1", "fp1", alert.StatusFiring)
	require.NoError(t, p.Process(ctx, env))

	assert.NotEmpty(t, env.CorrelationID)
}

// ─── Tests — error paths ──────────────────────────────────────────────────────

func TestPipeline_DedupErrorFailsOpen(t *testing.T) {
	t.Parallel()
	// Dedup returns an error — pipeline must NOT suppress the alert (fail open).
	dedup := newMemDedup()
	dedup.dedupErr = errors.New("valkey unavailable")
	pub := &memPublisher{}
	p := pipeline.New(dedup, memCorrelator{}, pub, zap.NewNop())

	env := newTestEnv("t1", "fp1", alert.StatusFiring)
	err := p.Process(context.Background(), env)

	require.NoError(t, err, "dedup error must not be propagated — pipeline fails open")
	assert.Equal(t, 1, pub.count(), "alert must still be published when dedup fails")
}

func TestPipeline_CorrelateErrorIsReturned(t *testing.T) {
	t.Parallel()
	corrErr := errors.New("correlator: store timeout")
	dedup := newMemDedup()
	pub := &memPublisher{}
	p := pipeline.New(dedup, memCorrelator{err: corrErr}, pub, zap.NewNop())

	env := newTestEnv("t1", "fp1", alert.StatusFiring)
	err := p.Process(context.Background(), env)

	require.Error(t, err)
	assert.ErrorIs(t, err, corrErr)
	assert.Contains(t, err.Error(), "pipeline correlate")
	assert.Equal(t, 0, pub.count(), "publish must not be called if correlation fails")
}

func TestPipeline_PublishErrorIsReturned(t *testing.T) {
	t.Parallel()
	pubErr := errors.New("jetstream: stream not found")
	dedup := newMemDedup()
	pub := &memPublisher{err: pubErr}
	p := pipeline.New(dedup, memCorrelator{}, pub, zap.NewNop())

	env := newTestEnv("t1", "fp1", alert.StatusFiring)
	err := p.Process(context.Background(), env)

	require.Error(t, err)
	assert.ErrorIs(t, err, pubErr)
	assert.Contains(t, err.Error(), "pipeline publish to")
}

func TestPipeline_ResolvedResetErrorIsNonFatal(t *testing.T) {
	t.Parallel()
	// Reset fails — should log and continue publishing the resolved event.
	dedup := newMemDedup()
	dedup.resetErr = errors.New("valkey: connection refused")
	pub := &memPublisher{}
	p := pipeline.New(dedup, memCorrelator{}, pub, zap.NewNop())

	resolved := newTestEnv("t1", "fp1", alert.StatusResolved)
	err := p.Process(context.Background(), resolved)

	require.NoError(t, err, "Reset error must not propagate — it is non-fatal")
	assert.Equal(t, 1, pub.count(), "resolved alert must still be published after Reset failure")
}

func TestPipeline_ReceivedAtIsSetOnProcess(t *testing.T) {
	t.Parallel()
	_, _, p := newPipeline()

	env := newTestEnv("t1", "fp1", alert.StatusFiring)
	env.ReceivedAt = time.Time{} // zero out
	require.NoError(t, p.Process(context.Background(), env))
	assert.False(t, env.ReceivedAt.IsZero(), "Process must set ReceivedAt")
}

func TestPipeline_PostProcessHookFiresOnSuccessfulPublish(t *testing.T) {
	t.Parallel()
	_, _, p := newPipeline()

	var called int
	var seen *alert.AlertEnvelope
	p.WithPostProcess(func(_ context.Context, env *alert.AlertEnvelope) {
		called++
		seen = env
	})

	env := newTestEnv("t1", "fp1", alert.StatusFiring)
	require.NoError(t, p.Process(context.Background(), env))
	assert.Equal(t, 1, called, "post-process hook must fire exactly once on success")
	require.NotNil(t, seen)
	assert.Equal(t, "fp1", seen.Fingerprint)
}

func TestPipeline_PostProcessHookSkippedOnDuplicate(t *testing.T) {
	t.Parallel()
	dedup, _, p := newPipeline()
	dedup.seen["t1:fp1"] = true // pre-populate so the alert is a duplicate

	var called int
	p.WithPostProcess(func(_ context.Context, _ *alert.AlertEnvelope) { called++ })

	env := newTestEnv("t1", "fp1", alert.StatusFiring)
	require.NoError(t, p.Process(context.Background(), env))
	assert.Equal(t, 0, called, "post-process hook must not fire when alert is deduplicated")
}
