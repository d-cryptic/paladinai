package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/services/paladin-ingest/config"
)

func clearIngestEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"WEBHOOK_SECRET", "PALADIN_INGEST_PORT"} {
		t.Setenv(k, "")
	}
}

func TestLoad_DefaultsWhenEnvUnset(t *testing.T) {
	clearIngestEnv(t)

	c, err := config.Load()
	require.NoError(t, err)
	assert.Empty(t, c.WebhookSecret, "WebhookSecret is optional; empty means dev mode")
	assert.Equal(t, 9001, c.Server.Port)
}

func TestLoad_WebhookSecretFromEnv(t *testing.T) {
	clearIngestEnv(t)
	t.Setenv("WEBHOOK_SECRET", "my-hmac-secret")

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "my-hmac-secret", c.WebhookSecret)
}

func TestLoad_PortOverrideFromEnv(t *testing.T) {
	clearIngestEnv(t)
	t.Setenv("PALADIN_INGEST_PORT", "9100")

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 9100, c.Server.Port)
}
