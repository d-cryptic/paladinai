package config_test

import (
	"testing"
	"time"

	"github.com/paladinai/paladinai/services/paladin-agent/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearAgentEnv unsets variables owned by this loader so each test is hermetic.
func clearAgentEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"ENV", "LOG_LEVEL", "NATS_URL", "DATABASE_URL", "VALKEY_URL",
		"QDRANT_URL", "VAULT_ADDR", "VAULT_TOKEN", "OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_SERVICE_VERSION", "OPENROUTER_API_KEY", "LLM_GATEWAY_URL",
		"LLM_TIER_A", "LLM_TIER_B", "LLM_TIER_C",
		"AGENT_WORKERS", "AGENT_CONSUMER_NAME",
	} {
		t.Setenv(key, "")
	}
}

func TestLoad_DefaultAgentWorkers(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 4, cfg.AgentWorkers)
}

func TestLoad_EnvAgentWorkers(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	t.Setenv("AGENT_WORKERS", "8")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 8, cfg.AgentWorkers)
}

func TestLoad_InvalidAgentWorkersFallsBackToDefault(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	t.Setenv("AGENT_WORKERS", "not-a-number")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 4, cfg.AgentWorkers)
}

func TestLoad_ZeroAgentWorkersFallsBackToDefault(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	t.Setenv("AGENT_WORKERS", "0")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 4, cfg.AgentWorkers, "zero is not a valid worker count; must fall back to default")
}

func TestLoad_NegativeAgentWorkersFallsBackToDefault(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	t.Setenv("AGENT_WORKERS", "-1")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 4, cfg.AgentWorkers, "negative value must fall back to default")
}

func TestLoad_DefaultConsumerName(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "paladin-agent-runtime", cfg.NATSConsumerName)
}

func TestLoad_EnvConsumerName(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	t.Setenv("AGENT_CONSUMER_NAME", "agent-instance-2")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "agent-instance-2", cfg.NATSConsumerName)
}

func TestLoad_DefaultTimeouts(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, cfg.TriageTimeout)
	assert.Equal(t, 60*time.Second, cfg.RCATimeout, "RCATimeout must be 2× TriageTimeout")
}

func TestLoad_RCATimeoutIsTwiceTriageTimeout(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 2*cfg.TriageTimeout, cfg.RCATimeout)
}

func TestLoad_MissingOpenRouterKeyReturnsError(t *testing.T) {
	clearAgentEnv(t)
	// OPENROUTER_API_KEY is intentionally not set.
	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OPENROUTER_API_KEY")
}

func TestLoad_BaseLLMFieldsPopulated(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "or-key-abc")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.LLM.GatewayURL)
	assert.NotEmpty(t, cfg.LLM.TierA)
	assert.NotEmpty(t, cfg.LLM.TierB)
	assert.NotEmpty(t, cfg.LLM.TierC)
	assert.Equal(t, "or-key-abc", cfg.LLM.OpenRouterKey)
}
