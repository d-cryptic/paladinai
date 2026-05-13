package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/services/paladin-memory/config"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"GRPC_ADDR", "MEMORY_HTTP_ADDR", "DATABASE_URL", "VALKEY_URL", "WORKING_MEMORY_TTL_SECONDS"} {
		t.Setenv(k, "")
	}
}

func TestLoad_RequiresDatabaseURL(t *testing.T) {
	clearEnv(t)

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL")
}

func TestLoad_DefaultsWhenSet(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/memory")

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, ":9010", c.GRPCAddr)
	assert.Equal(t, ":9011", c.HTTPAddr)
	assert.Equal(t, "redis://localhost:6379", c.ValkeyURL)
	assert.Equal(t, 1800, c.WorkingTTL)
	assert.Equal(t, "postgres://user:pass@localhost/memory", c.DatabaseURL)
}

func TestLoad_OverridesFromEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://x/y")
	t.Setenv("GRPC_ADDR", ":9999")
	t.Setenv("MEMORY_HTTP_ADDR", ":9998")
	t.Setenv("VALKEY_URL", "redis://valkey:6380")
	t.Setenv("WORKING_MEMORY_TTL_SECONDS", "60")

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, ":9999", c.GRPCAddr)
	assert.Equal(t, ":9998", c.HTTPAddr)
	assert.Equal(t, "redis://valkey:6380", c.ValkeyURL)
	assert.Equal(t, 60, c.WorkingTTL)
}

func TestLoad_RejectsNonPositiveTTL(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://x/y")
	t.Setenv("WORKING_MEMORY_TTL_SECONDS", "0")

	_, err := config.Load()
	require.Error(t, err)
}
