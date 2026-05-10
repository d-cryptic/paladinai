package worker_test

import (
	"context"
	"encoding/json"
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

// ─── In-memory NATS message ───────────────────────────────────────────────────

type memMsg struct {
	data    []byte
	acked   bool
	termed  bool
	nacked  bool
	mu      sync.Mutex
}

func newMemMsg(env *alert.AlertEnvelope) *memMsg {
	data, _ := json.Marshal(env)
	return &memMsg{data: data}
}

func (m *memMsg) Subject() string { return "paladin.alerts.correlated.test.alertmanager" }
func (m *memMsg) Data() []byte    { return m.data }
func (m *memMsg) Ack() error {
	m.mu.Lock(); defer m.mu.Unlock(); m.acked = true; return nil
}
func (m *memMsg) Term() error {
	m.mu.Lock(); defer m.mu.Unlock(); m.termed = true; return nil
}
func (m *memMsg) NakWithDelay(time.Duration) error {
	m.mu.Lock(); defer m.mu.Unlock(); m.nacked = true; return nil
}

// ─── Worker direct-call tests (bypass NATS consumer) ─────────────────────────

// processMsg calls the unexported handleMsg by using the exported interface.
// We test Worker.Run end-to-end indirectly via a mock that feeds a single message.

func TestWorker_TriageCalledOnValidMessage(t *testing.T) {
	triager := &stubTriager{result: &agent.TriageResult{
		ConfirmedSeverity: "P2",
		Summary:           "High error rate on api",
		NeedsHuman:        true,
	}}

	w := worker.New(triager, 5*time.Second, zap.NewNop())
	env := &alert.AlertEnvelope{
		TenantID:      "t1",
		Fingerprint:   "fp1",
		CorrelationID: "corr-abc",
		Severity:      alert.SeverityP2,
		Status:        alert.StatusFiring,
		Source:        alert.SourceAlertmanager,
	}

	require.NoError(t, w.ProcessEnvelope(context.Background(), env))
	assert.Equal(t, 1, triager.callCount())
}

func TestWorker_TriageErrorIsHandled(t *testing.T) {
	triager := &stubTriager{callErr: assert.AnError}
	w := worker.New(triager, 5*time.Second, zap.NewNop())
	env := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1"}

	err := w.ProcessEnvelope(context.Background(), env)
	assert.Error(t, err, "triage error should propagate")
}

func TestWorker_P1AlertNeedsHuman(t *testing.T) {
	triager := &stubTriager{result: &agent.TriageResult{
		ConfirmedSeverity: "P1",
		NeedsHuman:        true,
	}}
	w := worker.New(triager, 5*time.Second, zap.NewNop())
	env := &alert.AlertEnvelope{
		TenantID:  "t1",
		Fingerprint: "fp-p1",
		Severity:  alert.SeverityP1,
		Status:    alert.StatusFiring,
	}

	result, err := w.TriageEnvelope(context.Background(), env)
	require.NoError(t, err)
	assert.True(t, result.NeedsHuman)
	assert.Equal(t, "P1", result.ConfirmedSeverity)
}
