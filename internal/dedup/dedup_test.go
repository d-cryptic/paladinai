package dedup_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/dedup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ─── In-memory Store stub ─────────────────────────────────────────────────────

type memStore struct {
	mu      sync.Mutex
	entries map[string]memEntry
}

type memEntry struct {
	value  string
	expiry time.Time
}

func newMemStore() *memStore {
	return &memStore{entries: make(map[string]memEntry)}
}

func (m *memStore) SetNX(_ context.Context, key, value string, expiration time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.entries[key]; ok && time.Now().Before(e.expiry) {
		return false, nil
	}
	m.entries[key] = memEntry{value: value, expiry: time.Now().Add(expiration)}
	return true, nil
}

func (m *memStore) Del(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.entries, k)
	}
	return nil
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestDeduplicator_FirstAlertPassesThrough(t *testing.T) {
	d := dedup.New(newMemStore(), zap.NewNop())
	env := &alert.AlertEnvelope{ID: "a1", TenantID: "t1", Fingerprint: "fp1", Status: alert.StatusFiring}

	isDup, err := d.IsDuplicate(context.Background(), env)
	require.NoError(t, err)
	assert.False(t, isDup)
}

func TestDeduplicator_SecondAlertIsDuplicate(t *testing.T) {
	d := dedup.New(newMemStore(), zap.NewNop())
	env := &alert.AlertEnvelope{ID: "a1", TenantID: "t1", Fingerprint: "fp1", Status: alert.StatusFiring}

	_, _ = d.IsDuplicate(context.Background(), env)

	isDup, err := d.IsDuplicate(context.Background(), env)
	require.NoError(t, err)
	assert.True(t, isDup)
}

func TestDeduplicator_ResolvedAlwaysPassesThrough(t *testing.T) {
	d := dedup.New(newMemStore(), zap.NewNop())
	env := &alert.AlertEnvelope{ID: "a1", TenantID: "t1", Fingerprint: "fp1", Status: alert.StatusResolved}

	for i := 0; i < 3; i++ {
		isDup, err := d.IsDuplicate(context.Background(), env)
		require.NoError(t, err)
		assert.False(t, isDup, "resolved alerts always pass through (iteration %d)", i)
	}
}

func TestDeduplicator_DifferentTenantsAreIndependent(t *testing.T) {
	d := dedup.New(newMemStore(), zap.NewNop())
	env1 := &alert.AlertEnvelope{ID: "a1", TenantID: "tenant-1", Fingerprint: "fp", Status: alert.StatusFiring}
	env2 := &alert.AlertEnvelope{ID: "a2", TenantID: "tenant-2", Fingerprint: "fp", Status: alert.StatusFiring}

	isDup1, _ := d.IsDuplicate(context.Background(), env1)
	isDup2, _ := d.IsDuplicate(context.Background(), env2)
	assert.False(t, isDup1)
	assert.False(t, isDup2, "same fingerprint in different tenants must not share dedup state")
}

func TestDeduplicator_Reset_AllowsRefiring(t *testing.T) {
	d := dedup.New(newMemStore(), zap.NewNop())
	env := &alert.AlertEnvelope{ID: "a1", TenantID: "t1", Fingerprint: "fp1", Status: alert.StatusFiring}

	_, _ = d.IsDuplicate(context.Background(), env)
	isDup, _ := d.IsDuplicate(context.Background(), env)
	require.True(t, isDup)

	require.NoError(t, d.Reset(context.Background(), "t1", "fp1"))

	isDup, err := d.IsDuplicate(context.Background(), env)
	require.NoError(t, err)
	assert.False(t, isDup, "after Reset, alert must be treated as new")
}

func TestDeduplicator_NilEnvelope_ReturnsError(t *testing.T) {
	d := dedup.New(newMemStore(), zap.NewNop())
	isDup, err := d.IsDuplicate(context.Background(), nil)
	require.Error(t, err)
	assert.False(t, isDup)
}

func TestDeduplicator_NilLogger_UsesNop(t *testing.T) {
	d := dedup.New(newMemStore(), nil)
	env := &alert.AlertEnvelope{ID: "a1", TenantID: "t1", Fingerprint: "fp1", Status: alert.StatusFiring}
	isDup, err := d.IsDuplicate(context.Background(), env)
	require.NoError(t, err)
	assert.False(t, isDup)
}

func TestDeduplicator_DifferentFingerprints_Independent(t *testing.T) {
	d := dedup.New(newMemStore(), zap.NewNop())
	env1 := &alert.AlertEnvelope{ID: "a1", TenantID: "t1", Fingerprint: "fp-A", Status: alert.StatusFiring}
	env2 := &alert.AlertEnvelope{ID: "a2", TenantID: "t1", Fingerprint: "fp-B", Status: alert.StatusFiring}

	_, _ = d.IsDuplicate(context.Background(), env1)

	isDup, err := d.IsDuplicate(context.Background(), env2)
	require.NoError(t, err)
	assert.False(t, isDup, "different fingerprints in same tenant must not interfere")
}
