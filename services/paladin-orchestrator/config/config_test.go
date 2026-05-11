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
