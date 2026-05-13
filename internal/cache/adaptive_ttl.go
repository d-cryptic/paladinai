// adaptive_ttl.go implements Stage 9 §16: adaptive L1 TTL based on response stability.
//
// On each L1 hit the hit counter for the cache key is incremented. On each
// L1 write AdjustTTL checks the hit count accumulated since the previous write
// and scales the requested TTL:
//
//	hits > StableThreshold → stable response → extend by StableMultiplier (×1.5)
//	hits == 0              → unused response → shrink by UnusedMultiplier  (×0.5)
//	otherwise              → no change
//
// The result is clamped to [AdaptiveTTLFloor, AdaptiveTTLCeiling].
// HitCounter is safe for concurrent use.
package cache

import (
	"sync"
	"time"
)

const (
	// StableThreshold is the number of hits above which a response is considered stable.
	StableThreshold = 5
	// StableMultiplier extends TTL for stable responses.
	StableMultiplier = 1.5
	// UnusedMultiplier shrinks TTL for responses that were never re-used.
	UnusedMultiplier = 0.5
	// AdaptiveTTLFloor is the minimum TTL the adaptive layer will produce.
	AdaptiveTTLFloor = 10 * time.Minute
	// AdaptiveTTLCeiling is the maximum TTL the adaptive layer will produce.
	AdaptiveTTLCeiling = 8 * time.Hour
)

// HitCounter tracks per-key L1 hit counts for adaptive TTL computation.
// It is separate from the L1Cache interface so it can be composed without
// modifying existing cache implementations.
type HitCounter struct {
	mu   sync.Mutex
	hits map[string]int
}

// NewHitCounter returns a ready-to-use HitCounter.
func NewHitCounter() *HitCounter {
	return &HitCounter{hits: make(map[string]int)}
}

// RecordHit increments the hit counter for key. Call on every L1 cache hit.
func (h *HitCounter) RecordHit(key string) {
	h.mu.Lock()
	const maxHitKeys = 50_000
	if len(h.hits) >= maxHitKeys {
		if _, exists := h.hits[key]; !exists {
			// Evict a random existing entry to make room for the new key.
			for k := range h.hits {
				delete(h.hits, k)
				break
			}
		}
	}
	h.hits[key]++
	h.mu.Unlock()
}

// AdjustTTL returns the adaptive TTL for key given the requested base TTL.
// After returning it resets the hit counter so the next write starts fresh.
func (h *HitCounter) AdjustTTL(key string, base time.Duration) time.Duration {
	h.mu.Lock()
	count := h.hits[key]
	delete(h.hits, key) // reset for the next write cycle
	h.mu.Unlock()

	var ttl time.Duration
	switch {
	case count > StableThreshold:
		ttl = time.Duration(float64(base) * StableMultiplier)
	case count == 0:
		ttl = time.Duration(float64(base) * UnusedMultiplier)
	default:
		ttl = base
	}

	if ttl < AdaptiveTTLFloor {
		ttl = AdaptiveTTLFloor
	}
	if ttl > AdaptiveTTLCeiling {
		ttl = AdaptiveTTLCeiling
	}
	return ttl
}

// Hits returns the current hit count for key without modifying state.
// Intended for observability / testing only.
func (h *HitCounter) Hits(key string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hits[key]
}
