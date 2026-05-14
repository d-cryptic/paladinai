package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/services/paladin-auth/internal/store"
)

func TestMemStore_CreateAndGet(t *testing.T) {
	s := store.NewMemStore()
	ctx := context.Background()

	tenant, err := s.Create(ctx, "acme", "Acme Corp")
	require.NoError(t, err)
	assert.NotEmpty(t, tenant.ID)
	assert.Equal(t, "acme", tenant.Slug)
	assert.Equal(t, store.TenantStateActive, tenant.State)

	got, err := s.Get(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, got.ID)
}

func TestMemStore_GetBySlug(t *testing.T) {
	s := store.NewMemStore()
	ctx := context.Background()

	orig, err := s.Create(ctx, "beta", "Beta Inc")
	require.NoError(t, err)

	got, err := s.GetBySlug(ctx, "beta")
	require.NoError(t, err)
	assert.Equal(t, orig.ID, got.ID)
}

func TestMemStore_DuplicateSlug(t *testing.T) {
	s := store.NewMemStore()
	ctx := context.Background()

	_, err := s.Create(ctx, "dup", "First")
	require.NoError(t, err)

	_, err = s.Create(ctx, "dup", "Second")
	assert.ErrorIs(t, err, store.ErrAlreadyExists)
}

func TestMemStore_NotFound(t *testing.T) {
	s := store.NewMemStore()
	ctx := context.Background()

	_, err := s.Get(ctx, "nonexistent-id")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemStore_GetBySlug_NotFound(t *testing.T) {
	s := store.NewMemStore()

	_, err := s.GetBySlug(context.Background(), "missing")

	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemStore_SetState(t *testing.T) {
	s := store.NewMemStore()
	ctx := context.Background()

	tenant, err := s.Create(ctx, "gamma", "Gamma LLC")
	require.NoError(t, err)

	updated, err := s.SetState(ctx, tenant.ID, store.TenantStateSuspended)
	require.NoError(t, err)
	assert.Equal(t, store.TenantStateSuspended, updated.State)

	// Verify persistence.
	got, err := s.Get(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, store.TenantStateSuspended, got.State)
}

func TestMemStore_SetState_NotFound(t *testing.T) {
	s := store.NewMemStore()

	_, err := s.SetState(context.Background(), "missing", store.TenantStateSuspended)

	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemStore_List(t *testing.T) {
	s := store.NewMemStore()
	ctx := context.Background()

	_, _ = s.Create(ctx, "t1", "T1")
	_, _ = s.Create(ctx, "t2", "T2")

	list, err := s.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
