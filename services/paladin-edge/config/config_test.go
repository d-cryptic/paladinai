package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCORSOrigins_DefaultsWhenEmpty(t *testing.T) {
	got := parseCORSOrigins("")
	assert.Equal(t, []string{"http://localhost:3000", "http://localhost:3001"}, got)
}

func TestParseCORSOrigins_Single(t *testing.T) {
	got := parseCORSOrigins("https://app.example.com")
	assert.Equal(t, []string{"https://app.example.com"}, got)
}

func TestParseCORSOrigins_Multiple(t *testing.T) {
	got := parseCORSOrigins("https://app.example.com,https://admin.example.com")
	assert.Equal(t, []string{"https://app.example.com", "https://admin.example.com"}, got)
}

func TestParseCORSOrigins_TrimsSpaces(t *testing.T) {
	got := parseCORSOrigins("  https://a.com , https://b.com  ")
	assert.Equal(t, []string{"https://a.com", "https://b.com"}, got)
}

func TestParseCORSOrigins_AllBlankFallsBackToDefault(t *testing.T) {
	got := parseCORSOrigins("   ,   ")
	assert.Equal(t, []string{"http://localhost:3000", "http://localhost:3001"}, got)
}

// ─── Load ─────────────────────────────────────────────────────────────────────

func clearEdgeEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"PALADIN_EDGE_PORT", "JWT_SECRET", "HUB_URL", "CORS_ALLOWED_ORIGINS", "OPENROUTER_API_KEY"} {
		t.Setenv(k, "")
	}
}

func TestLoad_DefaultsWhenEnvUnset(t *testing.T) {
	clearEdgeEnv(t)

	c, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 9002, c.Server.Port)
	assert.Equal(t, 60, c.RateLimitRPS)
	assert.Empty(t, c.JWTSecret)
	assert.Empty(t, c.HubURL)
	assert.Equal(t, []string{"http://localhost:3000", "http://localhost:3001"}, c.AllowedOrigins)
}

func TestLoad_JWTSecretFromEnv(t *testing.T) {
	clearEdgeEnv(t)
	t.Setenv("JWT_SECRET", "my-signing-secret")

	c, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "my-signing-secret", c.JWTSecret)
}

func TestLoad_HubURLFromEnv(t *testing.T) {
	clearEdgeEnv(t)
	t.Setenv("HUB_URL", "http://paladin-hub:8082")

	c, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "http://paladin-hub:8082", c.HubURL)
}

func TestLoad_CORSOriginsFromEnv(t *testing.T) {
	clearEdgeEnv(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")

	c, err := Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"https://app.example.com"}, c.AllowedOrigins)
}

func TestLoad_LLMFallbackWhenKeyMissing(t *testing.T) {
	clearEdgeEnv(t)
	// OPENROUTER_API_KEY is unset → LoadLLM returns error → empty LLM fallback
	c, err := Load()
	require.NoError(t, err)
	assert.Empty(t, c.LLM.OpenRouterKey, "LLM should be zeroed when OPENROUTER_API_KEY is missing")
}
