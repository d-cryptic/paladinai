package cache_test

import (
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/cache"
	"github.com/stretchr/testify/assert"
)

// ─── TTLForQueryType ──────────────────────────────────────────────────────────

func TestTTLForQueryType_Classification(t *testing.T) {
	assert.Equal(t, 30*time.Minute, cache.TTLForQueryType(cache.QueryTypeClassification))
}

func TestTTLForQueryType_RCA(t *testing.T) {
	assert.Equal(t, 4*time.Hour, cache.TTLForQueryType(cache.QueryTypeRCA))
}

func TestTTLForQueryType_Unknown(t *testing.T) {
	// Unknown query type falls back to 1 hour
	assert.Equal(t, 1*time.Hour, cache.TTLForQueryType("unknown"))
}

// ─── ComputeTTL ───────────────────────────────────────────────────────────────

func TestComputeTTL_ExpensiveOpus(t *testing.T) {
	// Opus: $15/1M input, $75/1M output, 3000 input tokens, 1000 output tokens
	// cost = 3000/1e6*15 + 1000/1e6*75 = 0.045 + 0.075 = 0.12 → ≥0.05 → 4× multiplier
	req := cache.TTLRequest{
		QueryType:    cache.QueryTypeRCA,
		InputTokens:  3000,
		InputPrice:   15, // $15/1M
		OutputTokens: 1000,
		OutputPrice:  75, // $75/1M
	}
	ttl := cache.ComputeTTL(req)
	// base=4h × 4 = 16h, but max=16h
	assert.Equal(t, 16*time.Hour, ttl)
}

func TestComputeTTL_MediumSonnet(t *testing.T) {
	// Sonnet: $3/1M input, $15/1M output, 1000 input, 500 output
	// cost = 1000/1e6*3 + 500/1e6*15 = 0.003 + 0.0075 = 0.0105 → ≥0.01 → 2× multiplier
	req := cache.TTLRequest{
		QueryType:    cache.QueryTypeTriage,
		InputTokens:  1000,
		InputPrice:   3,
		OutputTokens: 500,
		OutputPrice:  15,
	}
	ttl := cache.ComputeTTL(req)
	// base=2h × 2 = 4h
	assert.Equal(t, 4*time.Hour, ttl)
}

func TestComputeTTL_CheapQwen(t *testing.T) {
	// Qwen: $0.10/1M input, $0.30/1M output, 500 input, 100 output
	// cost = 500/1e6*0.1 + 100/1e6*0.3 = 0.00005 + 0.00003 = 0.000008 → <0.001 → 0.25× multiplier
	req := cache.TTLRequest{
		QueryType:    cache.QueryTypeClassification,
		InputTokens:  500,
		InputPrice:   0.1,
		OutputTokens: 100,
		OutputPrice:  0.3,
	}
	ttl := cache.ComputeTTL(req)
	// base=30min × 0.25 = 7.5min → clamped to 10min
	assert.Equal(t, 10*time.Minute, ttl)
}

func TestComputeTTL_1xMultiplier(t *testing.T) {
	// Cost in [$0.001, $0.01) → 1× multiplier
	req := cache.TTLRequest{
		QueryType:    cache.QueryTypeRunbook,
		InputTokens:  1000,
		InputPrice:   1, // $1/1M = $0.001 per 1000 tokens
		OutputTokens: 100,
		OutputPrice:  3,
	}
	// cost = 0.001 + 0.0003 = 0.0013 → ≥0.001 → 1× multiplier
	ttl := cache.ComputeTTL(req)
	// base=1h × 1 = 1h
	assert.Equal(t, 1*time.Hour, ttl)
}

func TestComputeTTL_MinimumClamp(t *testing.T) {
	req := cache.TTLRequest{QueryType: cache.QueryTypeClassification, InputTokens: 0, InputPrice: 0, OutputTokens: 0, OutputPrice: 0}
	ttl := cache.ComputeTTL(req)
	assert.Equal(t, 10*time.Minute, ttl, "zero-cost response should clamp to 10min minimum")
}

func TestComputeTTL_MaximumClamp(t *testing.T) {
	req := cache.TTLRequest{
		QueryType:    cache.QueryTypeRCA,
		InputTokens:  100000,
		InputPrice:   100, // astronomically expensive
		OutputTokens: 100000,
		OutputPrice:  100,
	}
	ttl := cache.ComputeTTL(req)
	assert.Equal(t, 16*time.Hour, ttl, "very expensive response should clamp to 16h maximum")
}

// ─── BuildSystemBlocks ────────────────────────────────────────────────────────

func TestBuildSystemBlocks_HasCacheControl(t *testing.T) {
	blocks := cache.BuildSystemBlocks("you are a helpful agent")
	assert.Len(t, blocks, 1)
	assert.Equal(t, "text", blocks[0].Type)
	assert.NotNil(t, blocks[0].CacheControl)
	assert.Equal(t, "ephemeral", blocks[0].CacheControl.Type)
}

// ─── AppendToolCacheControl ───────────────────────────────────────────────────

func TestAppendToolCacheControl_LastToolGetsControl(t *testing.T) {
	tools := []cache.ToolEntry{
		{Name: "mcp-prometheus"},
		{Name: "mcp-loki"},
		{Name: "mcp-k8s"},
	}
	result := cache.AppendToolCacheControl(tools)
	assert.Nil(t, result[0].CacheControl)
	assert.Nil(t, result[1].CacheControl)
	assert.NotNil(t, result[2].CacheControl)
	assert.Equal(t, "ephemeral", result[2].CacheControl.Type)
}

func TestAppendToolCacheControl_DoesNotMutateInput(t *testing.T) {
	tools := []cache.ToolEntry{{Name: "mcp-prometheus"}}
	_ = cache.AppendToolCacheControl(tools)
	// Original slice should be unchanged
	assert.Nil(t, tools[0].CacheControl)
}

func TestAppendToolCacheControl_EmptySlice(t *testing.T) {
	result := cache.AppendToolCacheControl(nil)
	assert.Nil(t, result)

	result = cache.AppendToolCacheControl([]cache.ToolEntry{})
	assert.Empty(t, result)
}
