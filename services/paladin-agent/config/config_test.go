package config_test

import (
	"testing"
	"time"

	"github.com/paladinai/paladinai/services/paladin-agent/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearAgentEnv blanks every env variable consumed by this loader so each
// test is hermetic. Note: t.Setenv("", "") is equivalent to unset for this
// codebase because all getEnv/envInt helpers treat "" as "not set". Tests
// using this helper must NOT call t.Parallel() — t.Setenv forbids it.
func clearAgentEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"ENV", "LOG_LEVEL", "NATS_URL", "DATABASE_URL", "VALKEY_URL",
		"QDRANT_URL", "VAULT_ADDR", "VAULT_TOKEN", "OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_SERVICE_VERSION", "OPENROUTER_API_KEY", "LLM_GATEWAY_URL",
		"LLM_ALLOW_INSECURE_GATEWAY", "LLM_TIER_A", "LLM_TIER_B", "LLM_TIER_C",
		"AGENT_WORKERS", "AGENT_CONSUMER_NAME",
	} {
		t.Setenv(key, "")
	}
}

func TestLoad_AgentWorkers(t *testing.T) {
	cases := []struct {
		name   string
		setEnv string // empty means "leave unset"
		want   int
	}{
		{"default when unset", "", 4},
		{"valid override", "8", 8},
		{"non-numeric falls back", "not-a-number", 4},
		{"float falls back", "4.0", 4},
		{"whitespace falls back", " 8 ", 4},
		{"zero falls back", "0", 4},
		{"negative falls back", "-1", 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearAgentEnv(t)
			t.Setenv("OPENROUTER_API_KEY", "test-key")
			if tc.setEnv != "" {
				t.Setenv("AGENT_WORKERS", tc.setEnv)
			}
			cfg, err := config.Load()
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg.AgentWorkers)
		})
	}
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

func TestLoad_Timeouts(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, cfg.TriageTimeout)
	// RCATimeout is always derived from TriageTimeout — assert the invariant,
	// not a hard-coded value, so changes to TriageTimeout still surface here.
	assert.Equal(t, 2*cfg.TriageTimeout, cfg.RCATimeout)
}

func TestLoad_MissingOpenRouterKeyReturnsError(t *testing.T) {
	clearAgentEnv(t)
	// OPENROUTER_API_KEY is intentionally not set.
	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OPENROUTER_API_KEY")
}

func TestLoad_OpenRouterKeyPropagated(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "or-key-abc")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "or-key-abc", cfg.LLM.OpenRouterKey)
}
