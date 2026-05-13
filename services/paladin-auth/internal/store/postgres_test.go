//go:build integration

// Run with: go test -tags integration -run TestPostgresStore ./services/paladin-auth/internal/store/...
// Requires: DATABASE_URL pointing to a running Postgres (default: postgres://paladin:paladin@localhost:5432/paladin?sslmode=disable)
package store

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://paladin:paladin@localhost:5432/paladin?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Skipf("skipping postgres integration test: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("skipping postgres integration test: ping failed: %v", err)
	}
	require.NoError(t, NewPostgresStore(pool).MigrateUp(context.Background()))
	t.Cleanup(pool.Close)
	return pool
}

func TestPostgresStore_CreateAndGet(t *testing.T) {
	pool := newTestPool(t)
	s := NewPostgresStore(pool)
	ctx := context.Background()

	slug := "integ-test-tenant-" + t.Name()

	// Create
	got, err := s.Create(ctx, slug, "Integration Test Tenant")
	require.NoError(t, err)
	assert.Equal(t, slug, got.Slug)
	assert.Equal(t, TenantStateActive, got.State)
	assert.NotEmpty(t, got.ID)

	// Get by ID
	fetched, err := s.Get(ctx, got.ID)
	require.NoError(t, err)
	assert.Equal(t, got.ID, fetched.ID)
	assert.Equal(t, got.Slug, fetched.Slug)

	// Get by slug
	bySlug, err := s.GetBySlug(ctx, slug)
	require.NoError(t, err)
	assert.Equal(t, got.ID, bySlug.ID)

	// Duplicate slug → ErrAlreadyExists
	_, err = s.Create(ctx, slug, "Duplicate")
	assert.ErrorIs(t, err, ErrAlreadyExists)

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", got.ID)
}

func TestPostgresStore_SetState(t *testing.T) {
	pool := newTestPool(t)
	s := NewPostgresStore(pool)
	ctx := context.Background()

	slug := "integ-state-" + t.Name()
	tenant, err := s.Create(ctx, slug, "State Test")
	require.NoError(t, err)

	updated, err := s.SetState(ctx, tenant.ID, TenantStateSuspended)
	require.NoError(t, err)
	assert.Equal(t, TenantStateSuspended, updated.State)

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenant.ID)
}

func TestPostgresStore_Get_NotFound(t *testing.T) {
	pool := newTestPool(t)
	s := NewPostgresStore(pool)

	_, err := s.Get(context.Background(), "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresStore_GetBySlug_NotFound(t *testing.T) {
	pool := newTestPool(t)
	s := NewPostgresStore(pool)

	_, err := s.GetBySlug(context.Background(), "definitely-does-not-exist")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresStore_List(t *testing.T) {
	pool := newTestPool(t)
	s := NewPostgresStore(pool)
	ctx := context.Background()

	slug := "integ-list-" + t.Name()
	tenant, err := s.Create(ctx, slug, "List Test")
	require.NoError(t, err)

	tenants, err := s.List(ctx)
	require.NoError(t, err)
	found := false
	for _, tt := range tenants {
		if tt.ID == tenant.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "created tenant should appear in List")

	_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenant.ID)
}
