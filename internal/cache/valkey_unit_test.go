package cache

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfNoNetwork skips the test when the sandbox blocks TCP listen.
func skipIfNoNetwork(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("network unavailable, skipping miniredis-based test: %v", err)
	}
	ln.Close()
}

func newMiniredis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	skipIfNoNetwork(t)
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

// ─── ValkeyL1 ─────────────────────────────────────────────────────────────────

func TestValkeyL1_SetAndGet_Hit(t *testing.T) {
	_, rdb := newMiniredis(t)
	c := NewValkeyL1(rdb)
	ctx := context.Background()

	key := CacheKey([]byte("test-payload"))
	require.NoError(t, c.Set(ctx, key, []byte(`{"ok":true}`), time.Minute))

	val, err := c.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, []byte(`{"ok":true}`), val)
}

func TestValkeyL1_Get_Miss(t *testing.T) {
	_, rdb := newMiniredis(t)
	c := NewValkeyL1(rdb)

	val, err := c.Get(context.Background(), "nonexistent-key")
	require.NoError(t, err)
	assert.Nil(t, val)
}

func TestValkeyL1_Set_Expiry(t *testing.T) {
	mr, rdb := newMiniredis(t)
	c := NewValkeyL1(rdb)
	ctx := context.Background()

	key := CacheKey([]byte("expiry-payload"))
	require.NoError(t, c.Set(ctx, key, []byte("val"), time.Second))

	// Fast-forward time past expiry.
	mr.FastForward(2 * time.Second)

	val, err := c.Get(ctx, key)
	require.NoError(t, err)
	assert.Nil(t, val, "expired entry should be a miss")
}

func TestValkeyL1_Set_ZeroTTL(t *testing.T) {
	_, rdb := newMiniredis(t)
	c := NewValkeyL1(rdb)
	ctx := context.Background()

	key := CacheKey([]byte("zero-ttl"))
	// Negative TTL should store with no expiry (miniredis behavior mirrors Redis).
	require.NoError(t, c.Set(ctx, key, []byte("persistent"), 0))
}

// ─── ValkeyL3 ─────────────────────────────────────────────────────────────────

func TestValkeyL3_SetAndGet_Hit(t *testing.T) {
	_, rdb := newMiniredis(t)
	c := NewValkeyL3(rdb)
	ctx := context.Background()

	args := map[string]string{"pod": "api-server"}
	require.NoError(t, c.Set(ctx, "tenant-a", "mcp-k8s", args, []byte(`{"cpu":"50m"}`)))

	val, err := c.Get(ctx, "tenant-a", "mcp-k8s", args)
	require.NoError(t, err)
	assert.Equal(t, []byte(`{"cpu":"50m"}`), val)
}

func TestValkeyL3_Get_Miss(t *testing.T) {
	_, rdb := newMiniredis(t)
	c := NewValkeyL3(rdb)

	val, err := c.Get(context.Background(), "tenant-a", "mcp-k8s", nil)
	require.NoError(t, err)
	assert.Nil(t, val)
}

func TestValkeyL3_UncacheableTool_NotStored(t *testing.T) {
	_, rdb := newMiniredis(t)
	c := NewValkeyL3(rdb)
	ctx := context.Background()

	args := map[string]string{"channel": "#ops"}
	require.NoError(t, c.Set(ctx, "t1", "mcp-slack", args, []byte(`{"ok":true}`)))

	val, err := c.Get(ctx, "t1", "mcp-slack", args)
	require.NoError(t, err)
	assert.Nil(t, val, "mcp-slack (TTL=0) must never be cached")
}

func TestValkeyL3_SecretResult_NotStored(t *testing.T) {
	_, rdb := newMiniredis(t)
	c := NewValkeyL3(rdb)
	ctx := context.Background()

	secretData := []byte(`{"token": "sk-abcdefghijklmnopqrstuvwxyz12345678901234"}`)
	args := map[string]string{"path": ".env"}
	require.NoError(t, c.Set(ctx, "t1", "mcp-github", args, secretData))

	val, err := c.Get(ctx, "t1", "mcp-github", args)
	require.NoError(t, err)
	assert.Nil(t, val, "secret result must not be stored in L3")
}

func TestValkeyL3_TenantIsolation(t *testing.T) {
	_, rdb := newMiniredis(t)
	c := NewValkeyL3(rdb)
	ctx := context.Background()

	args := map[string]string{"pod": "api"}
	require.NoError(t, c.Set(ctx, "tenant-a", "mcp-k8s", args, []byte(`{"tenantA":true}`)))

	val, err := c.Get(ctx, "tenant-b", "mcp-k8s", args)
	require.NoError(t, err)
	assert.Nil(t, val, "tenant-B must not see tenant-A cache entries")
}

func TestValkeyL3_BadArgs_ErrorOnSet(t *testing.T) {
	_, rdb := newMiniredis(t)
	c := NewValkeyL3(rdb)
	ctx := context.Background()

	err := c.Set(ctx, "t1", "mcp-k8s", make(chan int), []byte("data"))
	require.Error(t, err)
}

func TestValkeyL3_BadArgs_ErrorOnGet(t *testing.T) {
	_, rdb := newMiniredis(t)
	c := NewValkeyL3(rdb)
	ctx := context.Background()

	_, err := c.Get(ctx, "t1", "mcp-k8s", make(chan int))
	require.Error(t, err)
}
