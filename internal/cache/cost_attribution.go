// cost_attribution.go implements Stage 9 §18: cost attribution per cache hit.
//
// Each L1/L2 cache hit saves the cost of an LLM call. CostAttributor computes
// that saving per hit and accumulates per-tenant totals. The accumulated totals
// are intended to be flushed to a cache_events table (or equivalent) by the
// caller on a regular schedule.
package cache

import (
	"sync"
	"time"
)

// ModelCostProfile holds the per-model pricing used to estimate savings.
// Prices are in USD per million tokens (matching OpenRouter / Anthropic pricing).
type ModelCostProfile struct {
	Name             string
	InputPricePerM   float64 // USD per 1M input tokens
	OutputPricePerM  float64 // USD per 1M output tokens
	AvgInputTokens   int     // typical input size for this query type
	AvgOutputTokens  int     // typical output size for this query type
}

// Predefined profiles matching Stage 9 §18 estimates.
var (
	ProfileOpusRCA = ModelCostProfile{
		Name: "claude-opus-4", InputPricePerM: 15.0, OutputPricePerM: 75.0,
		AvgInputTokens: 8000, AvgOutputTokens: 600,
	}
	ProfileSonnetTriage = ModelCostProfile{
		Name: "claude-sonnet-4", InputPricePerM: 3.0, OutputPricePerM: 15.0,
		AvgInputTokens: 4000, AvgOutputTokens: 300,
	}
	ProfileQwenClassify = ModelCostProfile{
		Name: "qwen3-8b", InputPricePerM: 0.06, OutputPricePerM: 0.06,
		AvgInputTokens: 1000, AvgOutputTokens: 50,
	}
)

// EstimateSaving returns the estimated USD cost avoided by a cache hit for
// the given profile.
func EstimateSaving(p ModelCostProfile) float64 {
	input := float64(p.AvgInputTokens) / 1e6 * p.InputPricePerM
	output := float64(p.AvgOutputTokens) / 1e6 * p.OutputPricePerM
	return input + output
}

// CacheHitEvent records a single cache hit with the saving attributed.
type CacheHitEvent struct {
	TenantID     string
	TierHit      string // "l1", "l2", "l3"
	ModelProfile ModelCostProfile
	CostSavedUSD float64
	HitAt        time.Time
}

// CostAttributor accumulates cache hit events per tenant. Thread-safe.
// Callers flush accumulated totals to persistent storage on their own schedule.
type CostAttributor struct {
	mu      sync.Mutex
	totals  map[string]float64 // tenantID → total USD saved
	hitCnt  map[string]int     // tenantID → total hit count
}

// NewCostAttributor returns a ready-to-use CostAttributor.
func NewCostAttributor() *CostAttributor {
	return &CostAttributor{
		totals: make(map[string]float64),
		hitCnt: make(map[string]int),
	}
}

// Record records a cache hit event and accumulates the saving.
func (c *CostAttributor) Record(evt CacheHitEvent) {
	saving := evt.CostSavedUSD
	if saving == 0 {
		saving = EstimateSaving(evt.ModelProfile)
	}
	c.mu.Lock()
	c.totals[evt.TenantID] += saving
	c.hitCnt[evt.TenantID]++
	c.mu.Unlock()
}

// TotalSaved returns the accumulated USD cost saved for tenantID.
func (c *CostAttributor) TotalSaved(tenantID string) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.totals[tenantID]
}

// HitCount returns the total number of cache hits recorded for tenantID.
func (c *CostAttributor) HitCount(tenantID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hitCnt[tenantID]
}

// Flush returns a snapshot of all tenant totals and resets the accumulators.
// Callers should persist the returned snapshot before Flush returns to avoid
// double-counting on retry.
func (c *CostAttributor) Flush() map[string]float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]float64, len(c.totals))
	for k, v := range c.totals {
		out[k] = v
	}
	c.totals = make(map[string]float64)
	c.hitCnt = make(map[string]int)
	return out
}
