package workflow_test

import (
	"context"
	"testing"

	"github.com/paladinai/paladinai/internal/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeIncidentTrigger records which method was last called.
type fakeIncidentTrigger struct {
	lastMethod string
	runID      string
	err        error
}

func (f *fakeIncidentTrigger) TriggerTriage(_ context.Context, _ workflow.TriagePayload) (string, error) {
	f.lastMethod = "triage"
	return f.runID, f.err
}
func (f *fakeIncidentTrigger) TriggerP1(_ context.Context, _ workflow.P1Payload) (string, error) {
	f.lastMethod = "p1"
	return f.runID, f.err
}
func (f *fakeIncidentTrigger) TriggerP2(_ context.Context, _ workflow.P2Payload) (string, error) {
	f.lastMethod = "p2"
	return f.runID, f.err
}

func basePayload() workflow.TriagePayload {
	return workflow.TriagePayload{
		TenantID:    "t1",
		IncidentID:  "inc-1",
		Fingerprint: "fp",
		Severity:    "critical",
	}
}

func TestSeverityRouter_Critical_TriggersP1(t *testing.T) {
	f := &fakeIncidentTrigger{runID: "run-p1"}
	r := workflow.NewSeverityRouter(f)

	runID, err := r.Route(context.Background(), "critical", basePayload())
	require.NoError(t, err)
	assert.Equal(t, "run-p1", runID)
	assert.Equal(t, "p1", f.lastMethod)
}

func TestSeverityRouter_P0_TriggersP1(t *testing.T) {
	f := &fakeIncidentTrigger{runID: "run-p1"}
	r := workflow.NewSeverityRouter(f)
	_, err := r.Route(context.Background(), "p0", basePayload())
	require.NoError(t, err)
	assert.Equal(t, "p1", f.lastMethod)
}

func TestSeverityRouter_P1_TriggersP1(t *testing.T) {
	f := &fakeIncidentTrigger{runID: "run-p1"}
	r := workflow.NewSeverityRouter(f)
	_, err := r.Route(context.Background(), "p1", basePayload())
	require.NoError(t, err)
	assert.Equal(t, "p1", f.lastMethod)
}

func TestSeverityRouter_CriticalUppercase_TriggersP1(t *testing.T) {
	f := &fakeIncidentTrigger{}
	r := workflow.NewSeverityRouter(f)
	_, err := r.Route(context.Background(), "CRITICAL", basePayload())
	require.NoError(t, err)
	assert.Equal(t, "p1", f.lastMethod)
}

func TestSeverityRouter_Warning_TriggersP2(t *testing.T) {
	f := &fakeIncidentTrigger{runID: "run-p2"}
	r := workflow.NewSeverityRouter(f)

	runID, err := r.Route(context.Background(), "warning", basePayload())
	require.NoError(t, err)
	assert.Equal(t, "run-p2", runID)
	assert.Equal(t, "p2", f.lastMethod)
}

func TestSeverityRouter_P2_TriggersP2(t *testing.T) {
	f := &fakeIncidentTrigger{}
	r := workflow.NewSeverityRouter(f)
	_, err := r.Route(context.Background(), "p2", basePayload())
	require.NoError(t, err)
	assert.Equal(t, "p2", f.lastMethod)
}

func TestSeverityRouter_High_TriggersP2(t *testing.T) {
	f := &fakeIncidentTrigger{}
	r := workflow.NewSeverityRouter(f)
	_, err := r.Route(context.Background(), "high", basePayload())
	require.NoError(t, err)
	assert.Equal(t, "p2", f.lastMethod)
}

func TestSeverityRouter_Unknown_FallsBackToTriage(t *testing.T) {
	f := &fakeIncidentTrigger{runID: "run-triage"}
	r := workflow.NewSeverityRouter(f)

	runID, err := r.Route(context.Background(), "info", basePayload())
	require.NoError(t, err)
	assert.Equal(t, "run-triage", runID)
	assert.Equal(t, "triage", f.lastMethod)
}

func TestSeverityRouter_Empty_FallsBackToTriage(t *testing.T) {
	f := &fakeIncidentTrigger{}
	r := workflow.NewSeverityRouter(f)
	_, err := r.Route(context.Background(), "", basePayload())
	require.NoError(t, err)
	assert.Equal(t, "triage", f.lastMethod)
}

func TestSeverityRouter_PropagatesError(t *testing.T) {
	f := &fakeIncidentTrigger{err: assert.AnError}
	r := workflow.NewSeverityRouter(f)
	_, err := r.Route(context.Background(), "critical", basePayload())
	require.Error(t, err)
	assert.Equal(t, "p1", f.lastMethod)
}

func TestSeverityRouter_NoopClient_AllSeverities(t *testing.T) {
	noop := workflow.NoopClient{}
	r := workflow.NewSeverityRouter(noop)

	for _, sev := range []string{"critical", "p0", "p1", "p2", "warning", "high", "info", ""} {
		_, err := r.Route(context.Background(), sev, basePayload())
		assert.NoError(t, err, "severity %q should not error with NoopClient", sev)
	}
}
