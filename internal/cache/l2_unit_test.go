package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCtx() context.Context { return context.Background() }

// ─── ValkeyL2 constructor ─────────────────────────────────────────────────────

func TestNewValkeyL2_ReturnsNonNil(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer rdb.Close() //nolint:errcheck
	emb := NewMemEmbedder(4)
	l2 := NewValkeyL2(rdb, emb)
	require.NotNil(t, l2)
}

// ─── ValkeyL2 early-validation paths ─────────────────────────────────────────

func TestValkeyL2_Lookup_EmptyTenantID_ReturnsError(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer rdb.Close() //nolint:errcheck
	l2 := NewValkeyL2(rdb, NewMemEmbedder(4))

	_, err := l2.Lookup(newCtx(), "", "some query")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenantID must not be empty")
}

func TestValkeyL2_Store_EmptyTenantID_ReturnsError(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer rdb.Close() //nolint:errcheck
	l2 := NewValkeyL2(rdb, NewMemEmbedder(4))

	err := l2.Store(newCtx(), "", "some query", "some-l1-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenantID must not be empty")
}

// ─── isIndexMissingErr / isIndexExistsErr ────────────────────────────────────

func TestIsIndexMissingErr_NilError_ReturnsFalse(t *testing.T) {
	assert.False(t, isIndexMissingErr(nil))
}

func TestIsIndexMissingErr_NoSuchIndex(t *testing.T) {
	assert.True(t, isIndexMissingErr(errors.New("no such index")))
}

func TestIsIndexMissingErr_UnknownIndexName(t *testing.T) {
	assert.True(t, isIndexMissingErr(errors.New("Unknown Index name")))
}

func TestIsIndexMissingErr_OtherError_ReturnsFalse(t *testing.T) {
	assert.False(t, isIndexMissingErr(errors.New("some other redis error")))
}

func TestIsIndexExistsErr_NilError_ReturnsFalse(t *testing.T) {
	assert.False(t, isIndexExistsErr(nil))
}

func TestIsIndexExistsErr_IndexAlreadyExists(t *testing.T) {
	assert.True(t, isIndexExistsErr(errors.New("Index already exists")))
}

func TestIsIndexExistsErr_OtherError_ReturnsFalse(t *testing.T) {
	assert.False(t, isIndexExistsErr(errors.New("some other error")))
}

// ─── l2 key helpers ───────────────────────────────────────────────────────────

func TestL2IndexName(t *testing.T) {
	assert.Equal(t, "llm:l2:tenant-1", l2IndexName("tenant-1"))
}

func TestL2EntryPrefix(t *testing.T) {
	assert.Equal(t, "llm:l2:tenant-1:", l2EntryPrefix("tenant-1"))
}

func TestL2EntryKey(t *testing.T) {
	key := l2EntryKey("tenant-1", "abc123")
	assert.Equal(t, "llm:l2:tenant-1:abc123", key)
}

// ─── MemEmbedder dim ─────────────────────────────────────────────────────────

func TestMemEmbedder_Dim(t *testing.T) {
	emb := NewMemEmbedder(16)
	assert.Equal(t, 16, emb.Dim())
}
