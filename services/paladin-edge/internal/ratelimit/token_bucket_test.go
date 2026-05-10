package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/paladinai/paladinai/services/paladin-edge/internal/ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValkeyLimiter_UnderLimit(t *testing.T) {
	store := newMemLimitStore()
	limiter := ratelimit.NewTestLimiter(store, 5) // 5 req/window
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		allowed, remaining, _, err := limiter.Allow(ctx, "tenant-1")
		require.NoError(t, err)
		assert.True(t, allowed, "request %d should be allowed", i+1)
		assert.Equal(t, 5-i-1, remaining)
	}
}

func TestValkeyLimiter_OverLimit(t *testing.T) {
	store := newMemLimitStore()
	limiter := ratelimit.NewTestLimiter(store, 3)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		allowed, _, _, _ := limiter.Allow(ctx, "tenant-x")
		assert.True(t, allowed)
	}

	allowed, remaining, _, err := limiter.Allow(ctx, "tenant-x")
	require.NoError(t, err)
	assert.False(t, allowed, "4th request should be rate-limited")
	assert.Equal(t, 0, remaining)
}

func TestValkeyLimiter_TenantsAreIndependent(t *testing.T) {
	store := newMemLimitStore()
	limiter := ratelimit.NewTestLimiter(store, 2)
	ctx := context.Background()

	// Exhaust tenant-A
	limiter.Allow(ctx, "tenant-A") //nolint:errcheck
	limiter.Allow(ctx, "tenant-A") //nolint:errcheck
	limited, _, _, _ := limiter.Allow(ctx, "tenant-A")
	assert.False(t, limited)

	// tenant-B still has full quota
	allowed, remaining, _, _ := limiter.Allow(ctx, "tenant-B")
	assert.True(t, allowed)
	assert.Equal(t, 1, remaining)
}

func TestValkeyLimiter_WindowReset(t *testing.T) {
	store := newMemLimitStore()
	limiter := ratelimit.NewTestLimiter(store, 2)
	ctx := context.Background()

	limiter.Allow(ctx, "t1") //nolint:errcheck
	limiter.Allow(ctx, "t1") //nolint:errcheck
	limited, _, _, _ := limiter.Allow(ctx, "t1")
	require.False(t, limited, "should be rate-limited after 2 requests")

	// Simulate window expiry by clearing the store
	store.Reset("paladin:ratelimit:t1")

	allowed, _, _, _ := limiter.Allow(ctx, "t1")
	assert.True(t, allowed, "should be allowed after window reset")
}

// ─── in-memory Store for testing ─────────────────────────────────────────────

type memEntry struct {
	count  int
	expiry time.Time
}

type memLimitStore struct {
	entries map[string]*memEntry
}

func newMemLimitStore() *memLimitStore {
	return &memLimitStore{entries: make(map[string]*memEntry)}
}

func (m *memLimitStore) IncrWithExpire(key string, window time.Duration) (int, error) {
	e, ok := m.entries[key]
	if !ok || time.Now().After(e.expiry) {
		m.entries[key] = &memEntry{count: 1, expiry: time.Now().Add(window)}
		return 1, nil
	}
	e.count++
	return e.count, nil
}

func (m *memLimitStore) Reset(key string) {
	delete(m.entries, key)
}
