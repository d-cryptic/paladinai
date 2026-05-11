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

func cloudWatchPayload(t *testing.T, newState string) json.RawMessage {
	t.Helper()
	innerMsg := map[string]any{
		"AlarmName":        "HighCPU",
		"AlarmDescription": "CPU above 90%",
		"AWSAccountId":     "123456789",
		"NewStateValue":    newState,
		"NewStateReason":   "Threshold crossed",
		"StateChangeTime":  "2026-05-11T10:00:00.000+0000",
		"Region":           "us-east-1",
		"AlarmArn":         "arn:aws:cloudwatch:us-east-1:123456789:alarm:HighCPU",
		"OldStateValue":    "OK",
		"Trigger": map[string]any{
			"MetricName":         "CPUUtilization",
			"Namespace":          "AWS/EC2",
			"StatisticType":      "Statistic",
			"Statistic":          "AVERAGE",
			"Period":             300,
			"Threshold":          90.0,
			"ComparisonOperator": "GreaterThanOrEqualToThreshold",
			"Dimensions":         []map[string]string{{"name": "InstanceId", "value": "i-abc123"}},
		},
	}
	innerBytes, err := json.Marshal(innerMsg)
	require.NoError(t, err)

	sns := map[string]any{
		"Type":      "Notification",
		"MessageId": "abc123",
		"TopicArn":  "arn:aws:sns:us-east-1:123456789:paladin-alerts",
		"Subject":   "ALARM: \"HighCPU\" in US East (N. Virginia)",
		"Message":   string(innerBytes),
		"Timestamp": "2026-05-11T10:00:00.000Z",
	}
	b, err := json.Marshal(sns)
	require.NoError(t, err)
	return b
}

func TestNormalizeCloudWatch_HappyPath(t *testing.T) {
	payload := cloudWatchPayload(t, "ALARM")
	envelopes, err := normalizer.NormalizeCloudWatch("tenant-123", payload, zap.NewNop())
	require.NoError(t, err)
	require.Len(t, envelopes, 1)

	env := envelopes[0]
	assert.Equal(t, "tenant-123", env.TenantID)
	assert.Equal(t, alert.SourceCloudWatch, env.Source)
	assert.Equal(t, "HighCPU", env.Title)
	assert.Equal(t, "CPU above 90%", env.Description)
	assert.Equal(t, alert.StatusFiring, env.Status)
	assert.Equal(t, alert.SeverityP2, env.Severity)
	assert.Equal(t, "AWS/EC2", env.Labels["namespace"])
	assert.Equal(t, "CPUUtilization", env.Labels["metric_name"])
	assert.Equal(t, "us-east-1", env.Labels["region"])
	assert.Equal(t, "i-abc123", env.Labels["InstanceId"])
	assert.Equal(t, 2026, env.StartsAt.Year())
	assert.NotEmpty(t, env.Fingerprint)
}

func TestNormalizeCloudWatch_SeverityMapping(t *testing.T) {
	cases := []struct {
		state string
		want  alert.Severity
	}{
		{"ALARM", alert.SeverityP2},
		{"INSUFFICIENT_DATA", alert.SeverityP3},
		{"OK", alert.SeverityP4},
	}
	for _, tc := range cases {
		t.Run(tc.state, func(t *testing.T) {
			payload := cloudWatchPayload(t, tc.state)
			envelopes, err := normalizer.NormalizeCloudWatch("t1", payload, zap.NewNop())
			require.NoError(t, err)
			assert.Equal(t, tc.want, envelopes[0].Severity)
		})
	}
}

func TestNormalizeCloudWatch_StatusMapping(t *testing.T) {
	cases := []struct {
		state string
		want  alert.Status
	}{
		{"ALARM", alert.StatusFiring},
		{"INSUFFICIENT_DATA", alert.StatusFiring},
		{"OK", alert.StatusResolved},
	}
	for _, tc := range cases {
		t.Run(tc.state, func(t *testing.T) {
			payload := cloudWatchPayload(t, tc.state)
			envelopes, err := normalizer.NormalizeCloudWatch("t1", payload, zap.NewNop())
			require.NoError(t, err)
			assert.Equal(t, tc.want, envelopes[0].Status)
		})
	}
}

func TestNormalizeCloudWatch_ResolvedSetsEndsAt(t *testing.T) {
	payload := cloudWatchPayload(t, "OK")
	envelopes, err := normalizer.NormalizeCloudWatch("t1", payload, zap.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, envelopes[0].EndsAt)
}

func TestNormalizeCloudWatch_InvalidJSON(t *testing.T) {
	_, err := normalizer.NormalizeCloudWatch("t1", json.RawMessage(`not-json`), zap.NewNop())
	assert.Error(t, err)
}

func TestNormalizeCloudWatch_InvalidInnerMessage(t *testing.T) {
	sns := map[string]any{"Type": "Notification", "Message": "not-valid-json-inside"}
	b, _ := json.Marshal(sns)
	_, err := normalizer.NormalizeCloudWatch("t1", b, zap.NewNop())
	assert.Error(t, err)
}

func TestNormalizeCloudWatch_EmptyMessage(t *testing.T) {
	sns := map[string]any{"Type": "Notification"}
	b, _ := json.Marshal(sns)
	_, err := normalizer.NormalizeCloudWatch("t1", b, zap.NewNop())
	assert.Error(t, err)
}

func TestNormalizeCloudWatch_NilLogger(t *testing.T) {
	payload := cloudWatchPayload(t, "ALARM")
	assert.NotPanics(t, func() {
		_, _ = normalizer.NormalizeCloudWatch("t1", payload, nil)
	})
}
