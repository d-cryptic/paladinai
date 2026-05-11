package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testGraphAlert() alert.AlertEnvelope {
	return alert.AlertEnvelope{
		ID:          "alert-graph-1",
		Fingerprint: "fp-graph",
		TenantID:    "tenant-graph",
		Title:       "High memory usage",
		Severity:    alert.SeverityP2,
		Status:      alert.StatusFiring,
		Labels:      map[string]string{"service": "api", "namespace": "prod"},
	}
}

func TestBuildGraph_RoutesToTriage(t *testing.T) {
	ctx := context.Background()
	a := testGraphAlert()

	triager := &fakeTriager{result: &agent.TriageResult{
		ConfirmedSeverity: "P2", Summary: "disk io spike", NeedsHuman: false,
	}}

	cg, err := agent.BuildTestGraph(ctx, "triage", triager, nil)
	require.NoError(t, err)

	state := &agent.IncidentState{TenantID: "tenant-graph", Alert: a}
	out, err := cg.Run(ctx, state)
	require.NoError(t, err)

	assert.Equal(t, "triage", out.AgentType)
	require.NotNil(t, out.TriageResult)
	assert.Equal(t, "P2", out.TriageResult.ConfirmedSeverity)
	assert.Equal(t, 1, triager.calls)
}

func TestBuildGraph_RoutesToRCA(t *testing.T) {
	ctx := context.Background()
	a := testGraphAlert()
	a.Severity = alert.SeverityP1

	triager := &fakeTriager{result: &agent.TriageResult{ConfirmedSeverity: "P1", NeedsHuman: true}}
	rcaAgent := &fakeRCA{result: &agent.RCAResult{
		RootCauseHypothesis: "memory leak in api service",
		Confidence:          "HIGH",
	}}

	cg, err := agent.BuildTestGraph(ctx, "rca", triager, rcaAgent)
	require.NoError(t, err)

	state := &agent.IncidentState{TenantID: "tenant-graph", Alert: a}
	out, err := cg.Run(ctx, state)
	require.NoError(t, err)

	assert.Equal(t, "rca", out.AgentType)
	require.NotNil(t, out.RCAResult)
	assert.Equal(t, "memory leak in api service", out.RCAResult.RootCauseHypothesis)
	assert.True(t, out.NeedsHuman)
	assert.Equal(t, 1, rcaAgent.calls)
}

func TestBuildGraph_RCAFallsBackToTriageWhenNilRCA(t *testing.T) {
	ctx := context.Background()
	a := testGraphAlert()
	a.Severity = alert.SeverityP1

	triager := &fakeTriager{result: &agent.TriageResult{ConfirmedSeverity: "P1", NeedsHuman: true}}

	cg, err := agent.BuildTestGraph(ctx, "rca", triager, nil) // nil RCA → fallback
	require.NoError(t, err)

	state := &agent.IncidentState{TenantID: "tenant-graph", Alert: a}
	out, err := cg.Run(ctx, state)
	require.NoError(t, err)

	// Should fall back to triage result when RCA agent is not configured
	assert.True(t, out.NeedsHuman)
	require.NotNil(t, out.TriageResult)
}

func TestBuildGraph_ClassifyError(t *testing.T) {
	ctx := context.Background()
	a := testGraphAlert()

	cg, err := agent.BuildTestGraphWithClassifyError(ctx, errors.New("classifier failed"), nil, nil)
	require.NoError(t, err)

	state := &agent.IncidentState{TenantID: "tenant-graph", Alert: a}
	_, err = cg.Run(ctx, state)
	assert.Error(t, err, "should propagate classifier error")
}

func TestBuildGraph_TriageError(t *testing.T) {
	ctx := context.Background()
	a := testGraphAlert()

	triager := &fakeTriager{err: errors.New("triage failed")}
	cg, err := agent.BuildTestGraph(ctx, "triage", triager, nil)
	require.NoError(t, err)

	state := &agent.IncidentState{TenantID: "tenant-graph", Alert: a}
	_, err = cg.Run(ctx, state)
	assert.Error(t, err, "should propagate triage error")
}

func TestBuildGraph_UnknownAgentTypeDefaultsToTriage(t *testing.T) {
	ctx := context.Background()
	a := testGraphAlert()

	triager := &fakeTriager{result: &agent.TriageResult{ConfirmedSeverity: "P3", NeedsHuman: false}}

	cg, err := agent.BuildTestGraph(ctx, "unknown", triager, nil)
	require.NoError(t, err)

	state := &agent.IncidentState{TenantID: "tenant-graph", Alert: a}
	out, err := cg.Run(ctx, state)
	require.NoError(t, err)

	// "unknown" agentType → branch defaults to triage
	require.NotNil(t, out.TriageResult)
	assert.Equal(t, 1, triager.calls)
}
