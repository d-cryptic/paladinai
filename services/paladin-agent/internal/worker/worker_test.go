package worker_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ─── Fakes ───────────────────────────────────────────────────────────────────

type stubRCA struct {
	mu      sync.Mutex
	calls   int
	result  *agent.RCAResult
	callErr error
}

func (s *stubRCA) Analyze(_ context.Context, _ *alert.AlertEnvelope, _ *agent.TriageResult) (*agent.RCAResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.callErr != nil {
		return nil, s.callErr
	}
	return s.result, nil
}

func (s *stubRCA) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type stubTriager struct {
	mu      sync.Mutex
	calls   []*alert.AlertEnvelope
	result  *agent.TriageResult
	callErr error
}

func (s *stubTriager) Triage(_ context.Context, env *alert.AlertEnvelope) (*agent.TriageResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, env)
	if s.callErr != nil {
		return nil, s.callErr
	}
	return s.result, nil
}

func (s *stubTriager) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

type stubPublisher struct {
	mu       sync.Mutex
	messages []publishedMsg
	err      error
}

type publishedMsg struct {
	subject string
	data    []byte
}

func (p *stubPublisher) Publish(_ context.Context, subject string, data []byte) (worker.PublishResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return worker.PublishResult{}, p.err
	}
	p.messages = append(p.messages, publishedMsg{subject: subject, data: data})
	return worker.PublishResult{}, nil
}

func (p *stubPublisher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.messages)
}

func (p *stubPublisher) lastSubject() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.messages) == 0 {
		return ""
	}
	return p.messages[len(p.messages)-1].subject
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func newWorker(triager *stubTriager, pub *stubPublisher) *worker.Worker {
	return worker.New(triager, pub, 5*time.Second, 2, zap.NewNop())
}

func firingEnv(tenantID, fingerprint string) *alert.AlertEnvelope {
	return &alert.AlertEnvelope{
		TenantID:    tenantID,
		Fingerprint: fingerprint,
		Source:      alert.SourceAlertmanager,
		Severity:    alert.SeverityP2,
		Status:      alert.StatusFiring,
	}
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestWorker_TriageCalledOnValidMessage(t *testing.T) {
	triager := &stubTriager{result: &agent.TriageResult{
		ConfirmedSeverity: "P2",
		Summary:           "High error rate on api",
		NeedsHuman:        true,
	}}
	pub := &stubPublisher{}
	w := newWorker(triager, pub)

	require.NoError(t, w.ProcessEnvelope(context.Background(), firingEnv("t1", "fp1")))
	assert.Equal(t, 1, triager.callCount())
	assert.Equal(t, 1, pub.count(), "result should be published after triage")
}

func TestWorker_TriageErrorIsHandled(t *testing.T) {
	triager := &stubTriager{callErr: assert.AnError}
	pub := &stubPublisher{}
	w := newWorker(triager, pub)

	err := w.ProcessEnvelope(context.Background(), firingEnv("t1", "fp1"))
	assert.Error(t, err, "triage error should propagate")
	assert.Equal(t, 0, pub.count(), "no publish on triage error")
}

func TestWorker_P1AlertNeedsHuman(t *testing.T) {
	triager := &stubTriager{result: &agent.TriageResult{
		ConfirmedSeverity: "P1",
		NeedsHuman:        true,
	}}
	pub := &stubPublisher{}
	w := newWorker(triager, pub)

	env := &alert.AlertEnvelope{
		TenantID:    "t1",
		Fingerprint: "fp-p1",
		Severity:    alert.SeverityP1,
		Status:      alert.StatusFiring,
		Source:      alert.SourceAlertmanager,
	}
	result, err := w.TriageEnvelope(context.Background(), env)
	require.NoError(t, err)
	assert.True(t, result.NeedsHuman)
	assert.Equal(t, "P1", result.ConfirmedSeverity)
}

func TestWorker_PublishSubjectContainsTenantAndSource(t *testing.T) {
	triager := &stubTriager{result: &agent.TriageResult{ConfirmedSeverity: "P3"}}
	pub := &stubPublisher{}
	w := newWorker(triager, pub)

	env := firingEnv("acme-corp", "fp-sub")
	require.NoError(t, w.ProcessEnvelope(context.Background(), env))

	subject := pub.lastSubject()
	assert.Contains(t, subject, "acme-corp", "subject must contain tenant ID")
	assert.Contains(t, subject, "alertmanager", "subject must contain source")
	assert.Contains(t, subject, "paladin.alerts.triaged.", "subject must use triaged prefix")
}

func TestWorker_PublishFailurePropagatesError(t *testing.T) {
	triager := &stubTriager{result: &agent.TriageResult{ConfirmedSeverity: "P3"}}
	pub := &stubPublisher{err: assert.AnError}
	w := newWorker(triager, pub)

	err := w.ProcessEnvelope(context.Background(), firingEnv("t1", "fp1"))
	assert.Error(t, err, "publish failure should propagate so caller can Nak")
}

// ─── RCA chaining tests ───────────────────────────────────────────────────────

func TestWorkerWithRCA_ChainsRCAAfterTriage(t *testing.T) {
	triager := &stubTriager{result: &agent.TriageResult{ConfirmedSeverity: "P2", NeedsHuman: true}}
	rca := &stubRCA{result: &agent.RCAResult{
		RootCauseHypothesis: "DB connection pool exhausted",
		Confidence:          "HIGH",
		Evidence:            []string{"pg_stat shows 200 waiting"},
		RunbookKeywords:     []string{"postgres", "pool"},
	}}
	pub := &stubPublisher{}

	w := worker.New(triager, pub, 5*time.Second, 2, zap.NewNop()).WithRCA(rca)
	require.NoError(t, w.ProcessEnvelope(context.Background(), firingEnv("t1", "fp-rca")))

	assert.Equal(t, 1, triager.callCount(), "triage should run once")
	assert.Equal(t, 1, rca.callCount(), "rca should run once")
	assert.Equal(t, 1, pub.count(), "one combined publish")
	assert.Contains(t, pub.lastSubject(), "paladin.alerts.analyzed.", "analyzed subject when RCA runs")
	assert.Contains(t, pub.lastSubject(), "t1")
}

func TestWorkerWithRCA_RCAFailureFallsBackToTriagedSubject(t *testing.T) {
	triager := &stubTriager{result: &agent.TriageResult{ConfirmedSeverity: "P3"}}
	rca := &stubRCA{callErr: assert.AnError}
	pub := &stubPublisher{}

	w := worker.New(triager, pub, 5*time.Second, 2, zap.NewNop()).WithRCA(rca)
	require.NoError(t, w.ProcessEnvelope(context.Background(), firingEnv("t2", "fp-rca-err")))

	assert.Equal(t, 1, pub.count(), "should still publish on RCA error")
	assert.Contains(t, pub.lastSubject(), "paladin.alerts.triaged.", "falls back to triaged subject on RCA error")
}

func TestWorkerWithRCA_TriageErrorSkipsRCA(t *testing.T) {
	triager := &stubTriager{callErr: assert.AnError}
	rca := &stubRCA{result: &agent.RCAResult{Confidence: "HIGH"}}
	pub := &stubPublisher{}

	w := worker.New(triager, pub, 5*time.Second, 2, zap.NewNop()).WithRCA(rca)
	err := w.ProcessEnvelope(context.Background(), firingEnv("t3", "fp-triage-err"))
	assert.Error(t, err)
	assert.Equal(t, 0, rca.callCount(), "RCA must not run if triage failed")
	assert.Equal(t, 0, pub.count())
}

func TestWorkerWithRCA_NilResultTreatedAsFallback(t *testing.T) {
	// Analyze returning (nil, nil) must be treated like an RCA skip —
	// worker should publish to triaged subject, not analyzed.
	triager := &stubTriager{result: &agent.TriageResult{ConfirmedSeverity: "P3"}}
	rca := &stubRCA{result: nil, callErr: nil} // returns (nil, nil)
	pub := &stubPublisher{}

	w := worker.New(triager, pub, 5*time.Second, 2, zap.NewNop()).WithRCA(rca)
	require.NoError(t, w.ProcessEnvelope(context.Background(), firingEnv("t4", "fp-nil-rca")))

	assert.Equal(t, 1, pub.count())
	assert.Contains(t, pub.lastSubject(), "paladin.alerts.triaged.", "nil RCA result should fall back to triaged subject")
}

func TestWorkerWithSupervisor_ReturnsCopyWithSupervisor(t *testing.T) {
	triager := &stubTriager{result: &agent.TriageResult{ConfirmedSeverity: "P2"}}
	pub := &stubPublisher{}
	// WithSupervisor returns a new Worker with the supervisor field set;
	// nil SupervisorPipeline exercises the copy path without calling Supervisor.Run.
	w := worker.New(triager, pub, 5*time.Second, 2, zap.NewNop()).WithSupervisor(nil)
	require.NotNil(t, w, "WithSupervisor should return a non-nil Worker")

	// ProcessEnvelope still uses the direct triager path when called directly.
	err := w.ProcessEnvelope(context.Background(), firingEnv("t5", "fp-sup"))
	require.NoError(t, err)
	assert.Equal(t, 1, triager.callCount())
}

func TestWorkerConcurrencyDefault_ZeroConcurrency(t *testing.T) {
	triager := &stubTriager{result: &agent.TriageResult{ConfirmedSeverity: "P3"}}
	pub := &stubPublisher{}
	// Concurrency=0 should default to 4 internally without panicking.
	w := worker.New(triager, pub, 5*time.Second, 0, zap.NewNop())
	require.NotNil(t, w)
	err := w.ProcessEnvelope(context.Background(), firingEnv("t6", "fp-conc"))
	require.NoError(t, err)
}
