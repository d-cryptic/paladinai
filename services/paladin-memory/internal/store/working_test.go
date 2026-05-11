package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/services/paladin-memory/internal/store"
)

func newMiniredisStore(t *testing.T) (*store.RedisWorkingStore, *miniredis.Miniredis) {
	t.Helper()
	s := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	return store.NewRedisWorkingStore(c), s
}

func TestRedisWorkingStore_SetThenGet(t *testing.T) {
	t.Parallel()
	ws, _ := newMiniredisStore(t)
	ctx := context.Background()

	require.NoError(t, ws.Set(ctx, "tenant-1", "session-A", "step", "1", time.Minute))
	v, found, err := ws.Get(ctx, "tenant-1", "session-A", "step")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "1", v)
}

func TestRedisWorkingStore_GetMissingReturnsFoundFalse(t *testing.T) {
	t.Parallel()
	ws, _ := newMiniredisStore(t)

	_, found, err := ws.Get(context.Background(), "tenant-1", "session-A", "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestRedisWorkingStore_TTLExpires(t *testing.T) {
	t.Parallel()
	ws, mr := newMiniredisStore(t)
	ctx := context.Background()

	require.NoError(t, ws.Set(ctx, "tenant-1", "session-A", "k", "v", time.Second))
	mr.FastForward(2 * time.Second)

	_, found, err := ws.Get(ctx, "tenant-1", "session-A", "k")
	require.NoError(t, err)
	assert.False(t, found, "value should have expired")
}

func TestRedisWorkingStore_ScanByPrefix(t *testing.T) {
	t.Parallel()
	ws, _ := newMiniredisStore(t)
	ctx := context.Background()

	require.NoError(t, ws.Set(ctx, "t", "s", "step:1", "first", time.Minute))
	require.NoError(t, ws.Set(ctx, "t", "s", "step:2", "second", time.Minute))
	require.NoError(t, ws.Set(ctx, "t", "s", "other", "ignored", time.Minute))

	got, err := ws.Scan(ctx, "t", "s", "step:", 10)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"first", "second"}, got)
}

func TestRedisWorkingStore_IsolatedAcrossTenants(t *testing.T) {
	t.Parallel()
	ws, _ := newMiniredisStore(t)
	ctx := context.Background()

	require.NoError(t, ws.Set(ctx, "tenant-a", "s", "k", "a-value", time.Minute))
	require.NoError(t, ws.Set(ctx, "tenant-b", "s", "k", "b-value", time.Minute))

	a, _, err := ws.Get(ctx, "tenant-a", "s", "k")
	require.NoError(t, err)
	b, _, err := ws.Get(ctx, "tenant-b", "s", "k")
	require.NoError(t, err)
	assert.Equal(t, "a-value", a)
	assert.Equal(t, "b-value", b)
}
