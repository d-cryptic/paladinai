// Package cache provides caching layers for PaladinAI.
//
// L1 — Exact LLM cache: SHA-256 keyed, Valkey-backed, content-addressed.
// No tenant prefix on keys — responses are content-addressed and tenant-agnostic.
// Two backends ship:
//   - ValkeyL1: production, backed by go-redis/v9
//   - MemL1: thread-safe in-memory, for unit tests
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "llm:l1:"

// L1Cache is the interface for the exact LLM response cache.
type L1Cache interface {
	// Get returns the cached value for the given key, or (nil, nil) on miss.
	Get(ctx context.Context, key string) ([]byte, error)
	// Set stores value under key with the given TTL.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// CacheKey returns the canonical L1 cache key for a raw payload.
// The key is sha256(payload) with a fixed prefix.
func CacheKey(payload []byte) string {
	sum := sha256.Sum256(payload)
	return keyPrefix + hex.EncodeToString(sum[:])
}

// TriageRequest is the canonical struct used as the L1 cache lookup key.
// Only stable, deterministic fields are included; received_at and request IDs
// are excluded to maximise hit rate.
// encoding/json marshals map[string]string with sorted keys (Go 1.12+),
// so Labels and Annotations produce stable output regardless of insertion order.
type TriageRequest struct {
	Fingerprint   string            `json:"fingerprint"`
	CorrelationID string            `json:"correlation_id"`
	Title         string            `json:"title"`
	Severity      string            `json:"severity"`
	Labels        map[string]string `json:"labels,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"`
	Description   string            `json:"description,omitempty"`
	ModelID       string            `json:"model_id"`
}

// MarshalKey serialises r to a deterministic JSON representation and returns
// the L1 cache key. Identical TriageRequest values always produce the same key.
func (r TriageRequest) MarshalKey() (string, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return CacheKey(b), nil
}

// ValkeyL1 is the production Valkey-backed L1 cache.
type ValkeyL1 struct {
	rdb *redis.Client
}

// NewValkeyL1 wraps an existing go-redis client.
func NewValkeyL1(rdb *redis.Client) *ValkeyL1 {
	return &ValkeyL1{rdb: rdb}
}

func (c *ValkeyL1) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return val, err
}

func (c *ValkeyL1) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// memMaxEntries is the maximum number of entries MemL1/MemL3 will hold.
// New writes are silently dropped when the cap is reached to prevent test OOMs.
const memMaxEntries = 10_000

// MemL1 is a thread-safe in-memory L1 cache used in unit tests.
// Expired entries are only evicted on Get; there is no background sweeper.
type MemL1 struct {
	mu      sync.RWMutex
	entries map[string]memEntry
}

type memEntry struct {
	value     []byte
	expiresAt time.Time
}

// NewMemL1 creates a fresh in-memory L1 cache.
func NewMemL1() *MemL1 {
	return &MemL1{entries: make(map[string]memEntry)}
}

func (c *MemL1) Get(_ context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	if time.Now().After(e.expiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return nil, nil
	}
	return e.value, nil
}

func (c *MemL1) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	_, exists := c.entries[key]
	if exists || len(c.entries) < memMaxEntries {
		c.entries[key] = memEntry{value: value, expiresAt: time.Now().Add(ttl)}
	}
	c.mu.Unlock()
	return nil
}
