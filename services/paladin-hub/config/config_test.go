package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/services/paladin-hub/config"
)

func clearHubEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"DATABASE_URL", "HUB_PORT"} {
		t.Setenv(k, "")
	}
}

func TestLoad_DefaultsWhenEnvUnset(t *testing.T) {
	clearHubEnv(t)

	c, err := config.Load()
	require.NoError(t, err)
	assert.Empty(t, c.DatabaseURL, "empty DATABASE_URL means MemStore mode")
	assert.Equal(t, 8082, c.Server.Port)
}

func TestLoad_DatabaseURLFromEnv(t *testing.T) {
	clearHubEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/hub")

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "postgres://user:pass@localhost/hub", c.DatabaseURL)
}

func TestLoad_PortOverrideFromEnv(t *testing.T) {
	clearHubEnv(t)
	t.Setenv("HUB_PORT", "8088")

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 8088, c.Server.Port)
}
