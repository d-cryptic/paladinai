package llm_test

import (
	"context"
	"encoding/json"
	"fmt"
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

	// Pointer form must also redact (guards against accidental pointer-receiver conversion).
	b2, err := json.Marshal(&s)
	require.NoError(t, err)
	assert.Equal(t, `"[REDACTED]"`, string(b2))
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

// TestSecret_FmtVerbs ensures all common format verbs redact the value.
// A missed verb would leak credentials into zap-structured logs.
func TestSecret_FmtVerbs_Redacted(t *testing.T) {
	s := llm.Secret("leak-me")
	assert.Equal(t, "[REDACTED]", fmt.Sprintf("%s", s))
	assert.Equal(t, "[REDACTED]", fmt.Sprintf("%v", s))
	assert.NotContains(t, fmt.Sprintf("%+v", struct{ K llm.Secret }{s}), "leak-me")
}

// ── Tier constants ────────────────────────────────────────────────────────────

func TestTier_ConstantValues(t *testing.T) {
	assert.Equal(t, llm.Tier("A"), llm.TierA)
	assert.Equal(t, llm.Tier("B"), llm.TierB)
	assert.Equal(t, llm.Tier("C"), llm.TierC)
}

func TestTier_Distinct(t *testing.T) {
	assert.NotEqual(t, llm.TierA, llm.TierB)
	assert.NotEqual(t, llm.TierB, llm.TierC)
	assert.NotEqual(t, llm.TierA, llm.TierC)
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
