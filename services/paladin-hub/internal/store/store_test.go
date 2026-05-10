package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/paladinai/paladinai/services/paladin-hub/internal/registry"
	"github.com/paladinai/paladinai/services/paladin-hub/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testServer(tenantID, serverID string) *registry.MCPServer {
	return &registry.MCPServer{
		ID:           serverID,
		TenantID:     tenantID,
		Name:         "Test Server",
		Endpoint:     "https://mcp.test.com",
		Capabilities: []string{"tool_a", "tool_b"},
		Healthy:      true,
		RegisteredAt: time.Now().UTC(),
		LastSeenAt:   time.Now().UTC(),
	}
}

func TestMemStore_UpsertAndGet(t *testing.T) {
	s := store.NewMemStore()
	ctx := context.Background()

	server := testServer("t1", "srv1")
	require.NoError(t, s.Upsert(ctx, server))

	got, err := s.Get(ctx, "t1", "srv1")
	require.NoError(t, err)
	assert.Equal(t, server.ID, got.ID)
	assert.Equal(t, server.TenantID, got.TenantID)
	assert.Equal(t, server.Capabilities, got.Capabilities)
}

func TestMemStore_GetNotFound(t *testing.T) {
	s := store.NewMemStore()
	_, err := s.Get(context.Background(), "t1", "nonexistent")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemStore_List(t *testing.T) {
	s := store.NewMemStore()
	ctx := context.Background()

	require.NoError(t, s.Upsert(ctx, testServer("t1", "srv1")))
	require.NoError(t, s.Upsert(ctx, testServer("t1", "srv2")))
	require.NoError(t, s.Upsert(ctx, testServer("t2", "srv3"))) // different tenant

	t1Servers, err := s.List(ctx, "t1")
	require.NoError(t, err)
	assert.Len(t, t1Servers, 2, "only t1 servers should be returned")
}

func TestMemStore_Delete(t *testing.T) {
	s := store.NewMemStore()
	ctx := context.Background()

	require.NoError(t, s.Upsert(ctx, testServer("t1", "srv1")))
	require.NoError(t, s.Delete(ctx, "t1", "srv1"))

	_, err := s.Get(ctx, "t1", "srv1")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemStore_DeleteNotFound(t *testing.T) {
	s := store.NewMemStore()
	err := s.Delete(context.Background(), "t1", "nonexistent")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemStore_Heartbeat(t *testing.T) {
	s := store.NewMemStore()
	ctx := context.Background()

	require.NoError(t, s.Upsert(ctx, testServer("t1", "srv1")))

	later := time.Now().UTC().Add(time.Minute)
	require.NoError(t, s.Heartbeat(ctx, "t1", "srv1", later))

	got, err := s.Get(ctx, "t1", "srv1")
	require.NoError(t, err)
	assert.Equal(t, later, got.LastSeenAt)
	assert.True(t, got.Healthy)
}

func TestMemStore_UpsertOverwrites(t *testing.T) {
	s := store.NewMemStore()
	ctx := context.Background()

	orig := testServer("t1", "srv1")
	orig.Name = "Original"
	require.NoError(t, s.Upsert(ctx, orig))

	updated := testServer("t1", "srv1")
	updated.Name = "Updated"
	require.NoError(t, s.Upsert(ctx, updated))

	got, err := s.Get(ctx, "t1", "srv1")
	require.NoError(t, err)
	assert.Equal(t, "Updated", got.Name)
}

func TestMemStore_TenantIsolation(t *testing.T) {
	s := store.NewMemStore()
	ctx := context.Background()

	require.NoError(t, s.Upsert(ctx, testServer("tenant-a", "shared-id")))
	require.NoError(t, s.Upsert(ctx, testServer("tenant-b", "shared-id")))

	a, err := s.Get(ctx, "tenant-a", "shared-id")
	require.NoError(t, err)
	assert.Equal(t, "tenant-a", a.TenantID)

	b, err := s.Get(ctx, "tenant-b", "shared-id")
	require.NoError(t, err)
	assert.Equal(t, "tenant-b", b.TenantID)
}
