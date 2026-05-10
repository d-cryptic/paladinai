package normalizer_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/normalizer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAlertmanager_SingleAlert(t *testing.T) {
	payload := alertmanagerPayload(t, "firing", map[string]string{
		"alertname": "HighCPU",
		"severity":  "critical",
		"namespace": "production",
	}, map[string]string{
		"description": "CPU > 90% for 5m",
		"runbook_url": "https://runbook.example.com/high-cpu",
	})

	envelopes, err := normalizer.NormalizeAlertmanager("tenant-123", payload)
	require.NoError(t, err)
	require.Len(t, envelopes, 1)

	env := envelopes[0]
	assert.Equal(t, "tenant-123", env.TenantID)
	assert.Equal(t, alert.SeverityP1, env.Severity)
	assert.Equal(t, alert.StatusFiring, env.Status)
	assert.Equal(t, alert.SourceAlertmanager, env.Source)
	assert.Equal(t, "HighCPU", env.Title)
	assert.Equal(t, "CPU > 90% for 5m", env.Description)
	assert.Equal(t, "https://runbook.example.com/high-cpu", env.Runbook)
	assert.NotEmpty(t, env.ID)
	assert.NotEmpty(t, env.Fingerprint)
}

func TestNormalizeAlertmanager_MultipleAlerts(t *testing.T) {
	payload := multiAlertPayload(t)
	envelopes, err := normalizer.NormalizeAlertmanager("t1", payload)
	require.NoError(t, err)
	assert.Len(t, envelopes, 2)
}

func TestNormalizeAlertmanager_ResolvedAlert(t *testing.T) {
	endsAt := time.Now().Add(time.Minute)
	payload := alertmanagerPayloadResolved(t, endsAt)

	envelopes, err := normalizer.NormalizeAlertmanager("t1", payload)
	require.NoError(t, err)
	require.Len(t, envelopes, 1)

	env := envelopes[0]
	assert.Equal(t, alert.StatusResolved, env.Status)
	assert.NotNil(t, env.EndsAt)
}

func TestNormalizeAlertmanager_UnknownSeverity(t *testing.T) {
	payload := alertmanagerPayload(t, "firing", map[string]string{
		"alertname": "SomeAlert",
		"severity":  "unrecognised",
	}, nil)

	envelopes, err := normalizer.NormalizeAlertmanager("t1", payload)
	require.NoError(t, err)
	assert.Equal(t, alert.SeverityUnknown, envelopes[0].Severity)
}

// ─── helpers ────────────────────────────────────────────────────────────────

func alertmanagerPayload(t *testing.T, status string, labels, annotations map[string]string) json.RawMessage {
	t.Helper()
	type amAlert struct {
		Status      string            `json:"status"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		StartsAt    time.Time         `json:"startsAt"`
		EndsAt      time.Time         `json:"endsAt"`
	}
	payload := map[string]interface{}{
		"version":  "4",
		"status":   status,
		"receiver": "paladin",
		"alerts": []amAlert{{
			Status:      status,
			Labels:      labels,
			Annotations: annotations,
			StartsAt:    time.Now(),
		}},
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	return b
}

func alertmanagerPayloadResolved(t *testing.T, endsAt time.Time) json.RawMessage {
	t.Helper()
	type amAlert struct {
		Status      string            `json:"status"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		StartsAt    time.Time         `json:"startsAt"`
		EndsAt      time.Time         `json:"endsAt"`
	}
	payload := map[string]interface{}{
		"version":  "4",
		"status":   "resolved",
		"receiver": "paladin",
		"alerts": []amAlert{{
			Status:   "resolved",
			Labels:   map[string]string{"alertname": "Done"},
			StartsAt: time.Now().Add(-5 * time.Minute),
			EndsAt:   endsAt,
		}},
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	return b
}

func multiAlertPayload(t *testing.T) json.RawMessage {
	t.Helper()
	type amAlert struct {
		Status   string            `json:"status"`
		Labels   map[string]string `json:"labels"`
		StartsAt time.Time         `json:"startsAt"`
		EndsAt   time.Time         `json:"endsAt"`
	}
	payload := map[string]interface{}{
		"version":  "4",
		"status":   "firing",
		"receiver": "paladin",
		"alerts": []amAlert{
			{Status: "firing", Labels: map[string]string{"alertname": "A1"}, StartsAt: time.Now()},
			{Status: "firing", Labels: map[string]string{"alertname": "A2"}, StartsAt: time.Now()},
		},
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	return b
}
