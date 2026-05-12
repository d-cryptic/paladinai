package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("SLACK_WEBHOOK_URL", "")
	t.Setenv("PAGERDUTY_ROUTING_KEY", "")
	t.Setenv("COMMS_CONSUMER_NAME", "")
	t.Setenv("COMMS_WORKERS", "")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "", cfg.SlackWebhookURL)
	require.Equal(t, "", cfg.PagerDutyKey)
	require.Equal(t, "paladin-comms", cfg.NATSConsumerName)
	require.Equal(t, 4, cfg.Workers)
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("SLACK_WEBHOOK_URL", "https://hooks.slack.com/services/x/y/z")
	t.Setenv("PAGERDUTY_ROUTING_KEY", "pd-key-123")
	t.Setenv("COMMS_CONSUMER_NAME", "comms-prod")
	t.Setenv("COMMS_WORKERS", "8")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "https://hooks.slack.com/services/x/y/z", cfg.SlackWebhookURL)
	require.Equal(t, "pd-key-123", cfg.PagerDutyKey)
	require.Equal(t, "comms-prod", cfg.NATSConsumerName)
	require.Equal(t, 8, cfg.Workers)
}

func TestLoad_InvalidWorkersFallsBack(t *testing.T) {
	t.Setenv("COMMS_WORKERS", "not-a-number")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, 4, cfg.Workers)
}

func TestLoad_ZeroWorkersFallsBack(t *testing.T) {
	t.Setenv("COMMS_WORKERS", "0")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, 4, cfg.Workers)
}
