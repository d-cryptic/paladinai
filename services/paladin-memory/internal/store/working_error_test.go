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

// closedClientStore returns a working store whose redis client is closed,
// so every operation returns an error.
func closedClientStore(t *testing.T) *store.RedisWorkingStore {
	t.Helper()
	skipIfNoNetwork(t)
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	require.NoError(t, c.Close())
	mr.Close()
	return store.NewRedisWorkingStore(c)
}

func TestRedisWorkingStore_Get_ErrorWrapped(t *testing.T) {
	ws := closedClientStore(t)
	_, _, err := ws.Get(context.Background(), "t", "s", "k")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "memory: working get")
}

func TestRedisWorkingStore_Set_ErrorWrapped(t *testing.T) {
	ws := closedClientStore(t)
	err := ws.Set(context.Background(), "t", "s", "k", "v", time.Minute)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "memory: working set")
}

func TestRedisWorkingStore_Scan_ErrorWrapped(t *testing.T) {
	ws := closedClientStore(t)
	_, err := ws.Scan(context.Background(), "t", "s", "p", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "memory: working scan")
}

func TestRedisWorkingStore_Scan_DefaultLimitWhenZero(t *testing.T) {
	t.Parallel()
	skipIfNoNetwork(t)
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	ws := store.NewRedisWorkingStore(c)
	ctx := context.Background()

	require.NoError(t, ws.Set(ctx, "t", "s", "k1", "v1", time.Minute))
	require.NoError(t, ws.Set(ctx, "t", "s", "k2", "v2", time.Minute))

	// limit=0 should default to 20 internally.
	vals, err := ws.Scan(ctx, "t", "s", "k", 0)
	require.NoError(t, err)
	assert.Len(t, vals, 2)
}
