package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/internal/config"
)

// setenv sets an env var for the test and restores the original value on cleanup.
func setenv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

// ── LoadBase ──────────────────────────────────────────────────────────────────

func TestLoadBase_DefaultsWhenEnvUnset(t *testing.T) {
	// Explicitly unset env vars that might be set in CI.
	for _, k := range []string{"ENV", "LOG_LEVEL", "NATS_URL", "DATABASE_URL",
		"VALKEY_URL", "QDRANT_URL", "VAULT_ADDR", "VAULT_TOKEN",
		"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_SERVICE_VERSION"} {
		t.Setenv(k, "")
	}

	b, err := config.LoadBase()
	require.NoError(t, err)

	assert.Equal(t, "development", b.Env)
	assert.Equal(t, "info", b.LogLevel)
	assert.Equal(t, "nats://localhost:4222", b.NatsURL)
	assert.Equal(t, "redis://localhost:6379", b.ValkeyURL)
	assert.Equal(t, "http://localhost:6333", b.QdrantURL)
	assert.Equal(t, "http://localhost:8200", b.VaultAddr)
	assert.Equal(t, "dev", b.ServiceVersion)
	assert.Empty(t, b.DatabaseURL)
	assert.Empty(t, b.VaultToken)
	assert.Empty(t, b.OtelEndpoint)
}

func TestLoadBase_EnvVarsOverrideDefaults(t *testing.T) {
	setenv(t, "ENV", "production")
	setenv(t, "LOG_LEVEL", "warn")
	setenv(t, "NATS_URL", "nats://nats.prod:4222")
	setenv(t, "DATABASE_URL", "postgres://db/prod")
	setenv(t, "OTEL_SERVICE_VERSION", "v1.2.3")

	b, err := config.LoadBase()
	require.NoError(t, err)

	assert.Equal(t, "production", b.Env)
	assert.Equal(t, "warn", b.LogLevel)
	assert.Equal(t, "nats://nats.prod:4222", b.NatsURL)
	assert.Equal(t, "postgres://db/prod", b.DatabaseURL)
	assert.Equal(t, "v1.2.3", b.ServiceVersion)
}

// ── LoadLLM ───────────────────────────────────────────────────────────────────

func TestLoadLLM_MissingAPIKeyReturnsError(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")

	_, err := config.LoadLLM()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OPENROUTER_API_KEY")
}

func TestLoadLLM_DefaultTiersWhenUnset(t *testing.T) {
	setenv(t, "OPENROUTER_API_KEY", "sk-test")
	for _, k := range []string{"LLM_GATEWAY_URL", "LLM_TIER_A", "LLM_TIER_B", "LLM_TIER_C"} {
		t.Setenv(k, "")
	}

	llm, err := config.LoadLLM()
	require.NoError(t, err)

	assert.Equal(t, "sk-test", llm.OpenRouterKey)
	assert.Equal(t, "https://openrouter.ai/api/v1", llm.GatewayURL)
	assert.Equal(t, "qwen/qwen3-1.7b", llm.TierA)
	assert.Equal(t, "qwen/qwen3-8b", llm.TierB)
	assert.Equal(t, "deepseek/deepseek-v3", llm.TierC)
}

func TestLoadLLM_EnvVarsOverrideTiers(t *testing.T) {
	setenv(t, "OPENROUTER_API_KEY", "sk-prod")
	setenv(t, "LLM_TIER_A", "custom/fast")
	setenv(t, "LLM_TIER_C", "custom/powerful")

	llm, err := config.LoadLLM()
	require.NoError(t, err)

	assert.Equal(t, "custom/fast", llm.TierA)
	assert.Equal(t, "custom/powerful", llm.TierC)
}

// ── LoadServer ────────────────────────────────────────────────────────────────

func TestLoadServer_UsesDefaultPortWhenEnvUnset(t *testing.T) {
	t.Setenv("HTTP_PORT", "")

	srv, err := config.LoadServer("HTTP_PORT", 8080)
	require.NoError(t, err)

	assert.Equal(t, 8080, srv.Port)
	assert.Equal(t, 8180, srv.GRPCPort, "gRPC port should be HTTP port + 100")
}

func TestLoadServer_EnvPortOverridesDefault(t *testing.T) {
	setenv(t, "HTTP_PORT", "9090")

	srv, err := config.LoadServer("HTTP_PORT", 8080)
	require.NoError(t, err)

	assert.Equal(t, 9090, srv.Port)
	assert.Equal(t, 9190, srv.GRPCPort)
}

func TestLoadServer_InvalidPortFallsBackToDefault(t *testing.T) {
	setenv(t, "HTTP_PORT", "not-a-number")

	srv, err := config.LoadServer("HTTP_PORT", 8080)
	require.NoError(t, err)
	assert.Equal(t, 8080, srv.Port)
}

func TestLoadServer_TimeoutsAreNonZero(t *testing.T) {
	srv, err := config.LoadServer("HTTP_PORT", 8080)
	require.NoError(t, err)

	assert.Positive(t, srv.ReadTimeout)
	assert.Positive(t, srv.WriteTimeout)
	assert.Positive(t, srv.IdleTimeout)
	assert.Positive(t, srv.ShutdownTimeout)
}
