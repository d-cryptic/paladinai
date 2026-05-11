package normalizer_test

import (
	"encoding/json"
	"testing"

	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/normalizer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pdHappyPath = `{
  "event": {
    "id": "evt-abc123",
    "event_type": "incident.triggered",
    "resource_type": "incident",
    "occurred_at": "2026-05-11T10:00:00Z",
    "data": {
      "id": "Q1234ABCDEF",
      "summary": "High CPU on prod-web-01",
      "status": "triggered",
      "urgency": "high",
      "incident_number": 123,
      "title": "High CPU on prod-web-01",
      "service": {"id": "SVC123", "name": "web-service", "summary": "Web Service"},
      "teams": [{"id": "T123", "name": "infra-team"}],
      "priority": {"id": "P1", "name": "P1"}
    }
  }
}`

func TestNormalizePagerDuty_HappyPath(t *testing.T) {
	envelopes, err := normalizer.NormalizePagerDuty("tenant-123", json.RawMessage(pdHappyPath), zap.NewNop())
	require.NoError(t, err)
	require.Len(t, envelopes, 1)

	env := envelopes[0]
	assert.Equal(t, "tenant-123", env.TenantID)
	assert.Equal(t, alert.SourcePagerDuty, env.Source)
	assert.Equal(t, "High CPU on prod-web-01", env.Title)
	assert.Equal(t, "High CPU on prod-web-01", env.Description)
	assert.Equal(t, alert.StatusFiring, env.Status)
	assert.Equal(t, alert.SeverityP1, env.Severity)
	assert.Equal(t, "web-service", env.Labels["service"])
	assert.Equal(t, "infra-team", env.Labels["team"])
	assert.Equal(t, "high", env.Labels["urgency"])
	assert.Equal(t, 2026, env.StartsAt.Year())
	assert.NotEmpty(t, env.Fingerprint)
}

func TestNormalizePagerDuty_SeverityMapping(t *testing.T) {
	cases := []struct {
		name      string
		urgency   string
		status    string
		eventType string
		want      alert.Severity
	}{
		{"high triggered -> P1", "high", "triggered", "incident.triggered", alert.SeverityP1},
		{"low -> P3", "low", "triggered", "incident.triggered", alert.SeverityP3},
		{"high acknowledged -> P2", "high", "acknowledged", "incident.acknowledged", alert.SeverityP2},
		{"missing urgency -> P2", "", "triggered", "incident.triggered", alert.SeverityP2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := json.RawMessage(`{"event":{"event_type":"` + tc.eventType + `","data":{"title":"x","urgency":"` + tc.urgency + `","status":"` + tc.status + `"}}}`)
			envelopes, err := normalizer.NormalizePagerDuty("t1", payload, zap.NewNop())
			require.NoError(t, err)
			assert.Equal(t, tc.want, envelopes[0].Severity)
		})
	}
}

func TestNormalizePagerDuty_StatusMapping(t *testing.T) {
	cases := []struct {
		eventType string
		want      alert.Status
	}{
		{"incident.triggered", alert.StatusFiring},
		{"incident.acknowledged", alert.StatusFiring},
		{"incident.resolved", alert.StatusResolved},
	}
	for _, tc := range cases {
		t.Run(tc.eventType, func(t *testing.T) {
			payload := json.RawMessage(`{"event":{"event_type":"` + tc.eventType + `","data":{"title":"x"}}}`)
			envelopes, err := normalizer.NormalizePagerDuty("t1", payload, zap.NewNop())
			require.NoError(t, err)
			assert.Equal(t, tc.want, envelopes[0].Status)
		})
	}
}

func TestNormalizePagerDuty_ResolvedSetsEndsAt(t *testing.T) {
	payload := json.RawMessage(`{"event":{"event_type":"incident.resolved","data":{"title":"x"}}}`)
	envelopes, err := normalizer.NormalizePagerDuty("t1", payload, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, envelopes[0].EndsAt)
}

func TestNormalizePagerDuty_TitleFallsBackToSummary(t *testing.T) {
	payload := json.RawMessage(`{"event":{"event_type":"incident.triggered","data":{"summary":"only-summary"}}}`)
	envelopes, err := normalizer.NormalizePagerDuty("t1", payload, zap.NewNop())
	require.NoError(t, err)
	assert.Equal(t, "only-summary", envelopes[0].Title)
}

func TestNormalizePagerDuty_InvalidJSON(t *testing.T) {
	_, err := normalizer.NormalizePagerDuty("t1", json.RawMessage(`{bad`), zap.NewNop())
	assert.Error(t, err)
}

func TestNormalizePagerDuty_NilLogger(t *testing.T) {
	assert.NotPanics(t, func() {
		_, _ = normalizer.NormalizePagerDuty("t1", json.RawMessage(pdHappyPath), nil)
	})
}
