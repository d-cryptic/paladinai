package config_test

import (
	"testing"

	orchestratorcfg "github.com/paladinai/paladinai/services/paladin-orchestrator/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := orchestratorcfg.Load()
	require.NoError(t, err)
	assert.Equal(t, "paladin-orchestrator", cfg.ConsumerName)
	assert.Equal(t, 8, cfg.Workers)
	assert.Equal(t, "nats://localhost:4222", cfg.NatsURL)
	assert.Equal(t, "http://localhost:7077", cfg.HatchetURL)
	assert.Equal(t, "", cfg.HatchetAPIKey)
}

func TestLoad_HatchetEnvOverrides(t *testing.T) {
	t.Setenv("HATCHET_URL", "http://hatchet.internal:7077")
	t.Setenv("HATCHET_API_KEY", "secret-key")

	cfg, err := orchestratorcfg.Load()
	require.NoError(t, err)
	assert.Equal(t, "http://hatchet.internal:7077", cfg.HatchetURL)
	assert.Equal(t, "secret-key", cfg.HatchetAPIKey)
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("ORCHESTRATOR_CONSUMER_NAME", "my-consumer")
	t.Setenv("ORCHESTRATOR_WORKERS", "16")

	cfg, err := orchestratorcfg.Load()
	require.NoError(t, err)
	assert.Equal(t, "my-consumer", cfg.ConsumerName)
	assert.Equal(t, 16, cfg.Workers)
}

func TestLoad_InvalidWorkers_UsesDefault(t *testing.T) {
	t.Setenv("ORCHESTRATOR_WORKERS", "not-a-number")

	cfg, err := orchestratorcfg.Load()
	require.NoError(t, err)
	assert.Equal(t, 8, cfg.Workers, "non-integer ORCHESTRATOR_WORKERS falls back to default")
}

func TestLoad_ZeroWorkers_UsesDefault(t *testing.T) {
	t.Setenv("ORCHESTRATOR_WORKERS", "0")

	cfg, err := orchestratorcfg.Load()
	require.NoError(t, err)
	assert.Equal(t, 8, cfg.Workers, "zero ORCHESTRATOR_WORKERS falls back to default")
}
