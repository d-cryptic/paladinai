package agent_test

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// stubModel is an in-memory ToolCallingChatModel that returns a fixed response.
type stubModel struct {
	response string
}

var _ model.ToolCallingChatModel = (*stubModel)(nil)

func (s *stubModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage(s.response, nil), nil
}

func (s *stubModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (s *stubModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return s, nil
}

func (s *stubModel) BindTools(_ []*schema.ToolInfo) error {
	return nil
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestTriageAgent_ParsesValidJSON(t *testing.T) {
	ctx := context.Background()
	stub := &stubModel{response: `{
		"confirmed_severity": "P2",
		"summary": "High error rate on api service",
		"likely_cause": "Memory leak in api pod",
		"affected_services": ["api", "gateway"],
		"recommended_action": "Restart api deployment",
		"needs_human": true
	}`}

	ta, err := agent.NewTriageAgent(ctx, stub, zap.NewNop())
	require.NoError(t, err)

	env := &alert.AlertEnvelope{
		TenantID:    "t1",
		Fingerprint: "fp1",
		Severity:    alert.SeverityP2,
		Status:      alert.StatusFiring,
		Labels:      map[string]string{"namespace": "prod", "job": "api"},
		StartsAt:    time.Now(),
	}

	result, err := ta.Triage(ctx, env)
	require.NoError(t, err)

	assert.Equal(t, "P2", result.ConfirmedSeverity)
	assert.Equal(t, "High error rate on api service", result.Summary)
	assert.True(t, result.NeedsHuman)
	assert.Contains(t, result.AffectedServices, "api")
}

func TestTriageAgent_GracefulDegradationOnInvalidJSON(t *testing.T) {
	ctx := context.Background()
	stub := &stubModel{response: "The API service appears to be experiencing issues"}

	ta, err := agent.NewTriageAgent(ctx, stub, zap.NewNop())
	require.NoError(t, err)

	env := &alert.AlertEnvelope{
		TenantID:    "t1",
		Fingerprint: "fp2",
		Severity:    alert.SeverityP1,
		Status:      alert.StatusFiring,
	}

	result, err := ta.Triage(ctx, env)
	require.NoError(t, err, "invalid JSON response should degrade gracefully")
	assert.Equal(t, string(alert.SeverityP1), result.ConfirmedSeverity)
	assert.True(t, result.NeedsHuman, "P1 should always need human")
}

func TestTriageAgent_P3DoesNotNeedHumanByDefault(t *testing.T) {
	ctx := context.Background()
	stub := &stubModel{response: `{
		"confirmed_severity": "P3",
		"summary": "Minor CPU spike",
		"likely_cause": "Batch job running",
		"affected_services": ["batch"],
		"recommended_action": "Monitor for 30 minutes",
		"needs_human": false
	}`}

	ta, err := agent.NewTriageAgent(ctx, stub, zap.NewNop())
	require.NoError(t, err)

	env := &alert.AlertEnvelope{
		TenantID:    "t1",
		Fingerprint: "fp3",
		Severity:    alert.SeverityP3,
		Status:      alert.StatusFiring,
	}

	result, err := ta.Triage(ctx, env)
	require.NoError(t, err)
	assert.Equal(t, "P3", result.ConfirmedSeverity)
	assert.False(t, result.NeedsHuman)
}
