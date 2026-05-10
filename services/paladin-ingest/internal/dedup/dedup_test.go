package dedup_test

import (
	"context"
	"testing"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/dedup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestDeduplicator_FirstAlertPassesThrough(t *testing.T) {
	d := dedup.New(newMemStore(), zap.NewNop())
	ctx := context.Background()

	env := &alert.AlertEnvelope{
		ID:          "alert-1",
		TenantID:    "tenant-abc",
		Fingerprint: "abc123",
		Status:      alert.StatusFiring,
	}

	isDup, err := d.IsDuplicate(ctx, env)
	require.NoError(t, err)
	assert.False(t, isDup, "first alert should not be a duplicate")
}

func TestDeduplicator_SecondAlertIsDuplicate(t *testing.T) {
	d := dedup.New(newMemStore(), zap.NewNop())
	ctx := context.Background()

	env := &alert.AlertEnvelope{
		ID:          "alert-1",
		TenantID:    "tenant-abc",
		Fingerprint: "abc123",
		Status:      alert.StatusFiring,
	}

	_, err := d.IsDuplicate(ctx, env)
	require.NoError(t, err)

	isDup, err := d.IsDuplicate(ctx, env)
	require.NoError(t, err)
	assert.True(t, isDup, "second identical alert should be a duplicate")
}

func TestDeduplicator_ResolvedAlwaysPassesThrough(t *testing.T) {
	d := dedup.New(newMemStore(), zap.NewNop())
	ctx := context.Background()

	env := &alert.AlertEnvelope{
		ID:          "alert-1",
		TenantID:    "tenant-abc",
		Fingerprint: "abc123",
		Status:      alert.StatusResolved,
	}

	isDup, err := d.IsDuplicate(ctx, env)
	require.NoError(t, err)
	assert.False(t, isDup, "resolved alerts always pass through dedup")

	isDup, err = d.IsDuplicate(ctx, env)
	require.NoError(t, err)
	assert.False(t, isDup)
}

func TestDeduplicator_DifferentTenantsAreIndependent(t *testing.T) {
	d := dedup.New(newMemStore(), zap.NewNop())
	ctx := context.Background()

	fp := "same-fingerprint"
	env1 := &alert.AlertEnvelope{ID: "a1", TenantID: "tenant-1", Fingerprint: fp, Status: alert.StatusFiring}
	env2 := &alert.AlertEnvelope{ID: "a2", TenantID: "tenant-2", Fingerprint: fp, Status: alert.StatusFiring}

	isDup1, _ := d.IsDuplicate(ctx, env1)
	isDup2, _ := d.IsDuplicate(ctx, env2)

	assert.False(t, isDup1)
	assert.False(t, isDup2, "same fingerprint in different tenants should not dedup each other")
}

func TestDeduplicator_Reset(t *testing.T) {
	d := dedup.New(newMemStore(), zap.NewNop())
	ctx := context.Background()

	env := &alert.AlertEnvelope{
		ID:          "alert-1",
		TenantID:    "t1",
		Fingerprint: "fp1",
		Status:      alert.StatusFiring,
	}

	_, _ = d.IsDuplicate(ctx, env)

	isDup, _ := d.IsDuplicate(ctx, env)
	require.True(t, isDup)

	err := d.Reset(ctx, "t1", "fp1")
	require.NoError(t, err)

	isDup, err = d.IsDuplicate(ctx, env)
	require.NoError(t, err)
	assert.False(t, isDup, "after reset, alert should pass through again")
}
