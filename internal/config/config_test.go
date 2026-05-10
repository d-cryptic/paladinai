package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/internal/config"
)

// clearBaseEnv unsets all LoadBase env vars so tests start from a clean slate.
func clearBaseEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"ENV", "LOG_LEVEL", "NATS_URL", "DATABASE_URL",
		"VALKEY_URL", "QDRANT_URL", "VAULT_ADDR", "VAULT_TOKEN",
		"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_SERVICE_VERSION"} {
		t.Setenv(k, "")
	}
}

func clearLLMEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"OPENROUTER_API_KEY", "LLM_GATEWAY_URL",
		"LLM_TIER_A", "LLM_TIER_B", "LLM_TIER_C"} {
		t.Setenv(k, "")
	}
}

// ── LoadBase ──────────────────────────────────────────────────────────────────

func TestLoadBase_DefaultsWhenEnvUnset(t *testing.T) {
	clearBaseEnv(t)

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
	clearBaseEnv(t)
	t.Setenv("ENV", "production")
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("NATS_URL", "nats://nats.prod:4222")
	t.Setenv("DATABASE_URL", "postgres://db/prod")
	t.Setenv("OTEL_SERVICE_VERSION", "v1.2.3")

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
	clearLLMEnv(t)

	_, err := config.LoadLLM()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OPENROUTER_API_KEY")
}

func TestLoadLLM_DefaultTiersWhenUnset(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "sk-test")

	llm, err := config.LoadLLM()
	require.NoError(t, err)

	assert.Equal(t, "sk-test", llm.OpenRouterKey)
	assert.Equal(t, "https://openrouter.ai/api/v1", llm.GatewayURL)
	assert.Equal(t, "qwen/qwen3-1.7b", llm.TierA)
	assert.Equal(t, "qwen/qwen3-8b", llm.TierB)
	assert.Equal(t, "deepseek/deepseek-v3", llm.TierC)
}

func TestLoadLLM_EnvVarsOverrideTiers(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "sk-prod")
	t.Setenv("LLM_TIER_A", "custom/fast")
	t.Setenv("LLM_TIER_C", "custom/powerful")

	llm, err := config.LoadLLM()
	require.NoError(t, err)

	assert.Equal(t, "custom/fast", llm.TierA)
	assert.Equal(t, "qwen/qwen3-8b", llm.TierB, "TierB should remain default when not overridden")
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
	t.Setenv("HTTP_PORT", "9090")

	srv, err := config.LoadServer("HTTP_PORT", 8080)
	require.NoError(t, err)

	assert.Equal(t, 9090, srv.Port)
	assert.Equal(t, 9190, srv.GRPCPort)
}

// TestLoadServer_InvalidPortFallsBackToDefault documents the current silent-fallback
// contract. If the production code is ever changed to return an error on bad input,
// update this test to require.Error.
func TestLoadServer_InvalidPortFallsBackToDefault(t *testing.T) {
	t.Setenv("HTTP_PORT", "not-a-number")

	srv, err := config.LoadServer("HTTP_PORT", 8080)
	require.NoError(t, err)
	assert.Equal(t, 8080, srv.Port)
}

func TestLoadServer_Timeouts(t *testing.T) {
	srv, err := config.LoadServer("HTTP_PORT", 8080)
	require.NoError(t, err)

	assert.Equal(t, 15*time.Second, srv.ReadTimeout)
	assert.Equal(t, 30*time.Second, srv.WriteTimeout)
	assert.Equal(t, 60*time.Second, srv.IdleTimeout)
	assert.Equal(t, 10*time.Second, srv.ShutdownTimeout)
}
