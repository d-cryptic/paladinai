//go:build integration

package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/paladinai/paladinai/services/paladin-hub/internal/registry"
	"github.com/paladinai/paladinai/services/paladin-hub/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping Postgres integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func makeTestServer(tenantID, serverID string) *registry.MCPServer {
	return &registry.MCPServer{
		ID:           serverID,
		TenantID:     tenantID,
		Name:         "Test Server " + serverID,
		Description:  "Integration test server",
		Endpoint:     "https://test.example.com/" + serverID,
		Capabilities: []string{"search", "read"},
		Healthy:      true,
		RegisteredAt: time.Now().UTC().Truncate(time.Microsecond),
		LastSeenAt:   time.Now().UTC().Truncate(time.Microsecond),
	}
}

func cleanupTenant(t *testing.T, pool *pgxpool.Pool, tenantID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`DELETE FROM mcp_servers WHERE tenant_id = $1`, tenantID)
	require.NoError(t, err)
}

func TestPostgresStore_MigrateUp(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewPostgresStore(pool)

	require.NoError(t, s.MigrateUp(context.Background()), "migrate should be idempotent")
	require.NoError(t, s.MigrateUp(context.Background()), "second migrate should be a no-op")
}

func TestPostgresStore_UpsertAndGet(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewPostgresStore(pool)
	require.NoError(t, s.MigrateUp(context.Background()))

	server := makeTestServer("tenant-pg-1", "srv-1")
	t.Cleanup(func() { cleanupTenant(t, pool, "tenant-pg-1") })

	require.NoError(t, s.Upsert(context.Background(), server))

	got, err := s.Get(context.Background(), "tenant-pg-1", "srv-1")
	require.NoError(t, err)
	assert.Equal(t, server.Name, got.Name)
	assert.Equal(t, server.Endpoint, got.Endpoint)
	assert.Equal(t, server.Capabilities, got.Capabilities)
}

func TestPostgresStore_UpsertReplaces(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewPostgresStore(pool)
	require.NoError(t, s.MigrateUp(context.Background()))
	t.Cleanup(func() { cleanupTenant(t, pool, "tenant-pg-upsert") })

	server := makeTestServer("tenant-pg-upsert", "srv-upsert")
	require.NoError(t, s.Upsert(context.Background(), server))

	server.Name = "Updated Name"
	server.Capabilities = []string{"search", "read", "write"}
	require.NoError(t, s.Upsert(context.Background(), server))

	got, err := s.Get(context.Background(), "tenant-pg-upsert", "srv-upsert")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", got.Name)
	assert.Len(t, got.Capabilities, 3)
}

func TestPostgresStore_GetNotFound(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewPostgresStore(pool)
	require.NoError(t, s.MigrateUp(context.Background()))

	_, err := s.Get(context.Background(), "tenant-pg-nf", "does-not-exist")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestPostgresStore_List(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewPostgresStore(pool)
	require.NoError(t, s.MigrateUp(context.Background()))
	t.Cleanup(func() { cleanupTenant(t, pool, "tenant-pg-list") })

	for _, id := range []string{"srv-a", "srv-b", "srv-c"} {
		require.NoError(t, s.Upsert(context.Background(), makeTestServer("tenant-pg-list", id)))
	}

	list, err := s.List(context.Background(), "tenant-pg-list")
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

func TestPostgresStore_ListEmptyTenant(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewPostgresStore(pool)
	require.NoError(t, s.MigrateUp(context.Background()))

	list, err := s.List(context.Background(), "tenant-pg-empty")
	require.NoError(t, err)
	assert.NotNil(t, list, "nil slice would break JSON encoding")
	assert.Len(t, list, 0)
}

func TestPostgresStore_Delete(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewPostgresStore(pool)
	require.NoError(t, s.MigrateUp(context.Background()))
	t.Cleanup(func() { cleanupTenant(t, pool, "tenant-pg-del") })

	server := makeTestServer("tenant-pg-del", "srv-del")
	require.NoError(t, s.Upsert(context.Background(), server))
	require.NoError(t, s.Delete(context.Background(), "tenant-pg-del", "srv-del"))

	_, err := s.Get(context.Background(), "tenant-pg-del", "srv-del")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestPostgresStore_DeleteNotFound(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewPostgresStore(pool)
	require.NoError(t, s.MigrateUp(context.Background()))

	err := s.Delete(context.Background(), "tenant-pg-dnf", "does-not-exist")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestPostgresStore_Heartbeat(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewPostgresStore(pool)
	require.NoError(t, s.MigrateUp(context.Background()))
	t.Cleanup(func() { cleanupTenant(t, pool, "tenant-pg-hb") })

	server := makeTestServer("tenant-pg-hb", "srv-hb")
	require.NoError(t, s.Upsert(context.Background(), server))

	at := time.Now().UTC().Add(5 * time.Minute).Truncate(time.Microsecond)
	require.NoError(t, s.Heartbeat(context.Background(), "tenant-pg-hb", "srv-hb", at))

	got, err := s.Get(context.Background(), "tenant-pg-hb", "srv-hb")
	require.NoError(t, err)
	assert.Equal(t, at, got.LastSeenAt)
	assert.True(t, got.Healthy)
}

func TestPostgresStore_HeartbeatNotFound(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewPostgresStore(pool)
	require.NoError(t, s.MigrateUp(context.Background()))

	err := s.Heartbeat(context.Background(), "tenant-pg-hbnf", "does-not-exist", time.Now())
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestPostgresStore_TenantIsolation(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewPostgresStore(pool)
	require.NoError(t, s.MigrateUp(context.Background()))
	t.Cleanup(func() {
		cleanupTenant(t, pool, "tenant-iso-1")
		cleanupTenant(t, pool, "tenant-iso-2")
	})

	require.NoError(t, s.Upsert(context.Background(), makeTestServer("tenant-iso-1", "shared-id")))
	require.NoError(t, s.Upsert(context.Background(), makeTestServer("tenant-iso-2", "shared-id")))

	list1, err := s.List(context.Background(), "tenant-iso-1")
	require.NoError(t, err)
	list2, err := s.List(context.Background(), "tenant-iso-2")
	require.NoError(t, err)

	assert.Len(t, list1, 1)
	assert.Len(t, list2, 1)
	assert.Equal(t, "tenant-iso-1", list1[0].TenantID)
	assert.Equal(t, "tenant-iso-2", list2[0].TenantID)
}
