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

func TestNormalizeDatadog_HappyPath(t *testing.T) {
	payload := json.RawMessage(`{
		"id": "1234567890",
		"title": "[Triggered on host] CPU High",
		"text": "CPU usage is above 90% on host prod-web-01",
		"date_happened": 1715000000,
		"priority": "normal",
		"host": "prod-web-01",
		"alert_metric": "system.cpu.user",
		"alert_status": "Triggered",
		"alert_type": "metric alert",
		"tags": "env:prod,service:web,team:infra",
		"aggregation_key": "cpu_high_prod-web-01",
		"event_type": "metric_alert_monitor"
	}`)

	envelopes, err := normalizer.NormalizeDatadog("tenant-123", payload, zap.NewNop())
	require.NoError(t, err)
	require.Len(t, envelopes, 1)

	env := envelopes[0]
	assert.Equal(t, "tenant-123", env.TenantID)
	assert.Equal(t, alert.SourceDatadog, env.Source)
	assert.Equal(t, "[Triggered on host] CPU High", env.Title)
	assert.Equal(t, "CPU usage is above 90% on host prod-web-01", env.Description)
	assert.Equal(t, alert.StatusFiring, env.Status)
	assert.Equal(t, alert.SeverityP2, env.Severity)
	assert.Equal(t, "prod", env.Labels["env"])
	assert.Equal(t, "web", env.Labels["service"])
	assert.Equal(t, "infra", env.Labels["team"])
	assert.Equal(t, "prod-web-01", env.Labels["host"])
	assert.Equal(t, "metric alert", env.Labels["alert_type"])
	assert.Equal(t, int64(1715000000), env.StartsAt.Unix())
	assert.NotEmpty(t, env.ID)
	assert.NotEmpty(t, env.Fingerprint)
}

func TestNormalizeDatadog_SeverityMapping(t *testing.T) {
	cases := []struct {
		name     string
		status   string
		priority string
		want     alert.Severity
	}{
		{"urgent triggered -> P1", "Triggered", "urgent", alert.SeverityP1},
		{"normal triggered -> P2", "Triggered", "normal", alert.SeverityP2},
		{"recovered -> P4", "Recovered", "normal", alert.SeverityP4},
		{"low priority triggered -> P3", "Triggered", "low", alert.SeverityP3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := json.RawMessage(`{"title":"x","alert_status":"` + tc.status + `","priority":"` + tc.priority + `"}`)
			envelopes, err := normalizer.NormalizeDatadog("t1", payload, zap.NewNop())
			require.NoError(t, err)
			require.Len(t, envelopes, 1)
			assert.Equal(t, tc.want, envelopes[0].Severity)
		})
	}
}

func TestNormalizeDatadog_StatusMapping(t *testing.T) {
	cases := []struct {
		status string
		want   alert.Status
	}{
		{"Triggered", alert.StatusFiring},
		{"Recovered", alert.StatusResolved},
		{"Resolved", alert.StatusResolved},
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			payload := json.RawMessage(`{"title":"x","alert_status":"` + tc.status + `"}`)
			envelopes, err := normalizer.NormalizeDatadog("t1", payload, zap.NewNop())
			require.NoError(t, err)
			assert.Equal(t, tc.want, envelopes[0].Status)
		})
	}
}

func TestNormalizeDatadog_ResolvedSetsEndsAt(t *testing.T) {
	payload := json.RawMessage(`{"title":"x","alert_status":"Recovered"}`)
	envelopes, err := normalizer.NormalizeDatadog("t1", payload, zap.NewNop())
	require.NoError(t, err)
	require.Len(t, envelopes, 1)
	assert.NotNil(t, envelopes[0].EndsAt)
}

func TestNormalizeDatadog_InvalidJSON(t *testing.T) {
	_, err := normalizer.NormalizeDatadog("t1", json.RawMessage(`not-json`), zap.NewNop())
	assert.Error(t, err)
}

func TestNormalizeDatadog_NilLogger(t *testing.T) {
	payload := json.RawMessage(`{"title":"x","alert_status":"Triggered"}`)
	assert.NotPanics(t, func() {
		_, _ = normalizer.NormalizeDatadog("t1", payload, nil)
	})
}
