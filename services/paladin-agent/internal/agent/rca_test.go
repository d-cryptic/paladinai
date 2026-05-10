package agent_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func makeEnv(t *testing.T, sev alert.Severity) *alert.AlertEnvelope {
	t.Helper()
	return &alert.AlertEnvelope{
		TenantID:    "tenant-rca",
		Fingerprint: "fp-rca-" + string(sev),
		Severity:    sev,
		Status:      alert.StatusFiring,
		Title:       "High error rate on checkout service",
		Labels:      map[string]string{"service": "checkout", "namespace": "prod"},
		Annotations: map[string]string{"runbook": "https://wiki/checkout-errors"},
		StartsAt:    time.Now(),
		Source:      alert.SourceAlertmanager,
	}
}

func makeTriageResult(sev string) *agent.TriageResult {
	return &agent.TriageResult{
		ConfirmedSeverity: sev,
		Summary:           "Checkout service error rate above threshold",
		LikelyCause:       "Database connection pool exhausted",
		AffectedServices:  []string{"checkout", "payments"},
		NeedsHuman:        sev == "P1" || sev == "P2",
	}
}

func rcaStub(fields map[string]any) *stubModel {
	b, _ := json.Marshal(fields)
	return &stubModel{response: string(b)}
}

func TestRCAAgent_ParsesValidJSON(t *testing.T) {
	ctx := context.Background()
	stub := rcaStub(map[string]any{
		"root_cause_hypothesis": "Database connection pool exhausted due to slow queries from a recent schema migration",
		"evidence":              []string{"p99 latency spike at 02:14 UTC", "pg_stat_activity shows 200 waiting connections"},
		"confidence":            "HIGH",
		"recommended_fix":       "Roll back migration 0042, increase pool_size to 50",
		"escalation_path":       "On-call DBA",
		"runbook_keywords":      []string{"postgres", "connection pool", "checkout"},
	})

	ra, err := agent.NewRCAAgent(ctx, stub, zap.NewNop())
	require.NoError(t, err)

	result, err := ra.Analyze(ctx, makeEnv(t, alert.SeverityP2), makeTriageResult("P2"))
	require.NoError(t, err)

	assert.Equal(t, "HIGH", result.Confidence)
	assert.Len(t, result.Evidence, 2)
	assert.Contains(t, result.RunbookKeywords, "postgres")
	assert.NotEmpty(t, result.RecommendedFix)
	assert.Equal(t, "On-call DBA", result.EscalationPath)
	assert.False(t, result.Degraded)
}

func TestRCAAgent_GracefulDegradationOnInvalidJSON(t *testing.T) {
	ctx := context.Background()
	stub := &stubModel{response: "The root cause appears to be a database issue with the connection pool."}

	ra, err := agent.NewRCAAgent(ctx, stub, zap.NewNop())
	require.NoError(t, err)

	result, err := ra.Analyze(ctx, makeEnv(t, alert.SeverityP1), makeTriageResult("P1"))
	require.NoError(t, err, "non-JSON response should degrade gracefully, not error")
	assert.True(t, result.Degraded)
	assert.NotEmpty(t, result.RootCauseHypothesis)
}

func TestRCAAgent_InvalidConfidenceNormalisedToMedium(t *testing.T) {
	ctx := context.Background()
	stub := rcaStub(map[string]any{
		"root_cause_hypothesis": "Memory leak in worker goroutines",
		"evidence":              []string{"memory grew 4x in 1 hour"},
		"confidence":            "VERY_HIGH",
		"recommended_fix":       "Restart service",
		"runbook_keywords":      []string{"memory", "leak"},
	})

	ra, err := agent.NewRCAAgent(ctx, stub, zap.NewNop())
	require.NoError(t, err)

	result, err := ra.Analyze(ctx, makeEnv(t, alert.SeverityP2), makeTriageResult("P2"))
	require.NoError(t, err)
	assert.Equal(t, "MEDIUM", result.Confidence)
	assert.False(t, result.Degraded)
}

func TestRCAAgent_EmptySlicesCoerced(t *testing.T) {
	ctx := context.Background()
	stub := rcaStub(map[string]any{
		"root_cause_hypothesis": "Timeout in upstream API",
		"evidence":              []string{},
		"confidence":            "LOW",
		"recommended_fix":       "Investigate upstream API logs",
		"runbook_keywords":      []string{},
	})

	ra, err := agent.NewRCAAgent(ctx, stub, zap.NewNop())
	require.NoError(t, err)

	result, err := ra.Analyze(ctx, makeEnv(t, alert.SeverityP3), makeTriageResult("P3"))
	require.NoError(t, err)
	assert.NotNil(t, result.Evidence)
	assert.NotNil(t, result.RunbookKeywords)
}

func TestRCAAgent_RunbookKeywordsCappedAt20(t *testing.T) {
	ctx := context.Background()

	kws := make([]string, 60)
	for i := range kws {
		kws[i] = "keyword"
	}
	stub := rcaStub(map[string]any{
		"root_cause_hypothesis": "overflow",
		"evidence":              []string{"x"},
		"confidence":            "LOW",
		"recommended_fix":       "fix",
		"runbook_keywords":      kws,
	})

	ra, err := agent.NewRCAAgent(ctx, stub, zap.NewNop())
	require.NoError(t, err)

	result, err := ra.Analyze(ctx, makeEnv(t, alert.SeverityP4), makeTriageResult("P4"))
	require.NoError(t, err)
	assert.LessOrEqual(t, len(result.RunbookKeywords), 20)
}

func TestRCAAgent_FieldLengthsClamped(t *testing.T) {
	ctx := context.Background()
	longStr := strings.Repeat("A", 600)

	stub := rcaStub(map[string]any{
		"root_cause_hypothesis": longStr,
		"evidence":              []string{longStr},
		"confidence":            "HIGH",
		"recommended_fix":       longStr,
		"runbook_keywords":      []string{"ok"},
	})

	ra, err := agent.NewRCAAgent(ctx, stub, zap.NewNop())
	require.NoError(t, err)

	result, err := ra.Analyze(ctx, makeEnv(t, alert.SeverityP2), makeTriageResult("P2"))
	require.NoError(t, err)
	assert.LessOrEqual(t, len(result.RootCauseHypothesis), 500)
	assert.LessOrEqual(t, len(result.RecommendedFix), 200)
	for _, ev := range result.Evidence {
		assert.LessOrEqual(t, len(ev), 200)
	}
}
