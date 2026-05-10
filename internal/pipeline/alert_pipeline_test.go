package pipeline_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ─── In-memory fakes ─────────────────────────────────────────────────────────

type memDedup struct {
	mu   sync.Mutex
	seen map[string]bool
}

func newMemDedup() *memDedup { return &memDedup{seen: make(map[string]bool)} }

func (m *memDedup) IsDuplicate(_ context.Context, env *alert.AlertEnvelope) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
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
	delete(m.seen, tenantID+":"+fingerprint)
	return nil
}

type memCorrelator struct{}

func (memCorrelator) Correlate(_ context.Context, env *alert.AlertEnvelope) error {
	env.CorrelationID = "corr-test"
	return nil
}

type memPublisher struct {
	mu       sync.Mutex
	messages []publishedMsg
}

type publishedMsg struct {
	subject string
	data    []byte
}

func (p *memPublisher) Publish(_ context.Context, subject string, data []byte) (*jetstream.PubAck, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.messages = append(p.messages, publishedMsg{subject: subject, data: data})
	return &jetstream.PubAck{}, nil
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

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestPipeline_FirstAlertIsPublished(t *testing.T) {
	_, pub, p := newPipeline()
	ctx := context.Background()

	env := newTestEnv("t1", "fp1", alert.StatusFiring)
	require.NoError(t, p.Process(ctx, env))

	assert.Equal(t, 1, pub.count(), "first alert should be published")
	assert.Equal(t, "corr-test", env.CorrelationID)
}

func TestPipeline_DuplicateAlertIsSuppressed(t *testing.T) {
	_, pub, p := newPipeline()
	ctx := context.Background()

	env1 := newTestEnv("t1", "fp1", alert.StatusFiring)
	env2 := newTestEnv("t1", "fp1", alert.StatusFiring)

	require.NoError(t, p.Process(ctx, env1))
	require.NoError(t, p.Process(ctx, env2))

	assert.Equal(t, 1, pub.count(), "duplicate should be suppressed")
}

func TestPipeline_ResolvedAlertBypassesDedup(t *testing.T) {
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
	_ = dedup // used implicitly through Pipeline
}

func TestPipeline_PublishSubjectContainsTenantAndSource(t *testing.T) {
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

func TestPipeline_DifferentTenantsAreIndependent(t *testing.T) {
	_, pub, p := newPipeline()
	ctx := context.Background()

	envA := newTestEnv("tenant-a", "fp1", alert.StatusFiring)
	envB := newTestEnv("tenant-b", "fp1", alert.StatusFiring) // same fingerprint, different tenant

	require.NoError(t, p.Process(ctx, envA))
	require.NoError(t, p.Process(ctx, envB))

	assert.Equal(t, 2, pub.count(), "same fingerprint under different tenants should both publish")
}

func TestPipeline_CorrelationIDIsAssigned(t *testing.T) {
	_, _, p := newPipeline()
	ctx := context.Background()

	env := newTestEnv("t1", "fp1", alert.StatusFiring)
	require.NoError(t, p.Process(ctx, env))

	assert.NotEmpty(t, env.CorrelationID)
}
