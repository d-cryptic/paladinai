package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/internal/cache"
)

func TestMemL1_SetAndGet(t *testing.T) {
	c := cache.NewMemL1()
	ctx := context.Background()
	key := cache.CacheKey([]byte("test-payload"))

	// Miss before write.
	v, err := c.Get(ctx, key)
	require.NoError(t, err)
	assert.Nil(t, v)

	// Set then hit.
	require.NoError(t, c.Set(ctx, key, []byte(`{"ok":true}`), time.Minute))
	v, err = c.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, []byte(`{"ok":true}`), v)
}

func TestMemL1_Expiry(t *testing.T) {
	c := cache.NewMemL1()
	ctx := context.Background()
	key := cache.CacheKey([]byte("expiry-test"))

	require.NoError(t, c.Set(ctx, key, []byte("val"), -time.Second))
	v, err := c.Get(ctx, key)
	require.NoError(t, err)
	assert.Nil(t, v, "expired entry should be a miss")
}

func TestMemL1_CopiesValuesOnSetAndGet(t *testing.T) {
	c := cache.NewMemL1()
	ctx := context.Background()
	key := cache.CacheKey([]byte("copy-test"))
	value := []byte("original")

	require.NoError(t, c.Set(ctx, key, value, time.Minute))
	value[0] = 'X'

	got, err := c.Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, []byte("original"), got)

	got[0] = 'Y'
	gotAgain, err := c.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, []byte("original"), gotAgain)
}

func TestCacheKey_Deterministic(t *testing.T) {
	k1 := cache.CacheKey([]byte("same"))
	k2 := cache.CacheKey([]byte("same"))
	assert.Equal(t, k1, k2)
}

func TestCacheKey_Different(t *testing.T) {
	k1 := cache.CacheKey([]byte("payload-a"))
	k2 := cache.CacheKey([]byte("payload-b"))
	assert.NotEqual(t, k1, k2)
}

func TestTriageRequest_MarshalKey(t *testing.T) {
	r := cache.TriageRequest{
		Fingerprint:   "fp1",
		CorrelationID: "corr1",
		Title:         "CPU spike",
		Severity:      "P2",
		ModelID:       "qwen/qwen3-8b",
	}
	k1, err := r.MarshalKey()
	require.NoError(t, err)
	k2, err := r.MarshalKey()
	require.NoError(t, err)
	assert.Equal(t, k1, k2, "same struct should produce same key")
	assert.Contains(t, k1, "llm:l1:")
}
