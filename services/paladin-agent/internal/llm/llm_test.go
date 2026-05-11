package llm_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/paladinai/paladinai/services/paladin-agent/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Secret ────────────────────────────────────────────────────────────────────

func TestSecret_String_IsRedacted(t *testing.T) {
	s := llm.Secret("super-secret-key")
	assert.Equal(t, "[REDACTED]", s.String())
}

func TestSecret_Reveal_ReturnsRawValue(t *testing.T) {
	s := llm.Secret("super-secret-key")
	assert.Equal(t, "super-secret-key", s.Reveal())
}

func TestSecret_MarshalJSON_IsRedacted(t *testing.T) {
	s := llm.Secret("super-secret-key")
	b, err := json.Marshal(s)
	require.NoError(t, err)
	assert.Equal(t, `"[REDACTED]"`, string(b))
}

func TestSecret_MarshalJSON_InStruct_IsRedacted(t *testing.T) {
	type payload struct {
		Key llm.Secret `json:"key"`
	}
	b, err := json.Marshal(payload{Key: llm.Secret("do-not-leak")})
	require.NoError(t, err)
	assert.NotContains(t, string(b), "do-not-leak")
	assert.Contains(t, string(b), "[REDACTED]")
}

func TestSecret_EmptyString_StillRedacted(t *testing.T) {
	s := llm.Secret("")
	assert.Equal(t, "[REDACTED]", s.String())
	assert.Equal(t, "", s.Reveal())
}

// ── Tier constants ────────────────────────────────────────────────────────────

func TestTier_ConstantValues(t *testing.T) {
	assert.Equal(t, llm.Tier("A"), llm.TierA)
	assert.Equal(t, llm.Tier("B"), llm.TierB)
	assert.Equal(t, llm.Tier("C"), llm.TierC)
}

func TestTier_Distinct(t *testing.T) {
	tiers := []llm.Tier{llm.TierA, llm.TierB, llm.TierC}
	seen := make(map[llm.Tier]bool, len(tiers))
	for _, tier := range tiers {
		assert.False(t, seen[tier], "duplicate tier value: %q", tier)
		seen[tier] = true
	}
}

// ── New — URL validation (no network required) ────────────────────────────────

func TestNew_HTTPBaseURLReturnsError(t *testing.T) {
	cfg := llm.Config{
		BaseURL:    "http://openrouter.ai/api/v1",
		APIKey:     llm.Secret("key"),
		ModelTierA: "model-a",
		ModelTierB: "model-b",
		ModelTierC: "model-c",
	}
	_, err := llm.New(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPS")
}

func TestNew_EmptyBaseURLReturnsError(t *testing.T) {
	cfg := llm.Config{
		BaseURL:    "",
		APIKey:     llm.Secret("key"),
		ModelTierA: "model-a",
		ModelTierB: "model-b",
		ModelTierC: "model-c",
	}
	_, err := llm.New(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPS")
}

func TestNew_PlainHostWithoutSchemeReturnsError(t *testing.T) {
	cfg := llm.Config{
		BaseURL:    "openrouter.ai/api/v1",
		APIKey:     llm.Secret("key"),
		ModelTierA: "model-a",
		ModelTierB: "model-b",
		ModelTierC: "model-c",
	}
	_, err := llm.New(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPS")
}
