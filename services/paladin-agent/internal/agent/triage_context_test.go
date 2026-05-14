package agent_test

import (
	"testing"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
	"github.com/stretchr/testify/assert"
)

func TestTriageContextFromAlert_UsesClassifierSeverityAndAlertContext(t *testing.T) {
	env := &alert.AlertEnvelope{
		TenantID:    "tenant-1",
		Severity:    alert.SeverityP3,
		Title:       "Payments API 5xx surge",
		Description: "bad deployment introduced regression",
		Labels:      map[string]string{"service": "payments-api"},
		Annotations: map[string]string{"description": "recent deployment regression"},
	}

	result := agent.TriageContextFromAlert(env, "P1", "service_down")

	assert.Equal(t, "P1", result.ConfirmedSeverity)
	assert.Equal(t, "Payments API 5xx surge", result.Summary)
	assert.Equal(t, "recent deployment regression", result.LikelyCause)
	assert.Equal(t, []string{"payments-api"}, result.AffectedServices)
	assert.True(t, result.NeedsHuman)
	assert.Contains(t, result.RecommendedAction, "restore service health")
}

func TestTriageContextFromAlert_NormalizesBadSeverity(t *testing.T) {
	env := &alert.AlertEnvelope{
		Severity:    alert.Severity("bad"),
		Title:       "Unknown alert",
		Labels:      map[string]string{"job": "worker"},
		Annotations: map[string]string{},
	}

	result := agent.TriageContextFromAlert(env, "bad", "metric_spike")

	assert.Equal(t, "P3", result.ConfirmedSeverity)
	assert.Equal(t, []string{"worker"}, result.AffectedServices)
	assert.False(t, result.NeedsHuman)
}
