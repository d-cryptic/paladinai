package flap_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/flap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	mu     sync.Mutex
	counts map[string]int64
	hasTTL map[string]bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		counts: make(map[string]int64),
		hasTTL: make(map[string]bool),
	}
}

func (f *fakeStore) Incr(_ context.Context, key string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counts[key]++
	return f.counts[key], nil
}

func (f *fakeStore) ExpireNX(_ context.Context, key string, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.hasTTL[key] {
		return false, nil
	}
	f.hasTTL[key] = true
	return true, nil
}

func (f *fakeStore) Get(_ context.Context, key string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.counts[key]
	if !ok {
		return "", false, nil
	}
	return fmt.Sprintf("%d", v), true, nil
}

func TestDetector_NotFlappingBeforeThreshold(t *testing.T) {
	d := flap.New(newFakeStore(), nil).WithThreshold(4).WithGrace(time.Minute)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		isFlap, _, err := d.RecordTransition(ctx, "tenant-1", "fp-abc")
		require.NoError(t, err)
		assert.False(t, isFlap, "should not be flapping before threshold at transition %d", i+1)
	}
}

func TestDetector_FlappingAtThreshold(t *testing.T) {
	d := flap.New(newFakeStore(), nil).WithThreshold(4).WithGrace(time.Minute)
	ctx := context.Background()

	var isFlap bool
	var err error
	for i := 0; i < 4; i++ {
		isFlap, _, err = d.RecordTransition(ctx, "tenant-1", "fp-abc")
		require.NoError(t, err)
	}
	assert.True(t, isFlap, "should be flapping exactly at threshold")
}

func TestDetector_IsFlapping_MissingKey(t *testing.T) {
	d := flap.New(newFakeStore(), nil)
	flapping, err := d.IsFlapping(context.Background(), "tenant-1", "nonexistent")
	require.NoError(t, err)
	assert.False(t, flapping)
}

func TestDetector_IsFlapping_BelowThreshold(t *testing.T) {
	d := flap.New(newFakeStore(), nil).WithThreshold(4)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		_, _, err := d.RecordTransition(ctx, "tenant-1", "fp-xyz")
		require.NoError(t, err)
	}

	flapping, err := d.IsFlapping(ctx, "tenant-1", "fp-xyz")
	require.NoError(t, err)
	assert.False(t, flapping)
}

func TestDetector_IsFlapping_AtOrAboveThreshold(t *testing.T) {
	d := flap.New(newFakeStore(), nil).WithThreshold(4)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, _, err := d.RecordTransition(ctx, "tenant-1", "fp-xyz")
		require.NoError(t, err)
	}

	flapping, err := d.IsFlapping(ctx, "tenant-1", "fp-xyz")
	require.NoError(t, err)
	assert.True(t, flapping)
}

func TestDetector_IsolatedAcrossTenants(t *testing.T) {
	d := flap.New(newFakeStore(), nil).WithThreshold(3)
	ctx := context.Background()

	// Push tenant-a to flapping
	for i := 0; i < 3; i++ {
		_, _, err := d.RecordTransition(ctx, "tenant-a", "fp-shared")
		require.NoError(t, err)
	}

	// tenant-b with same fingerprint should still be 1
	isFlap, count, err := d.RecordTransition(ctx, "tenant-b", "fp-shared")
	require.NoError(t, err)
	assert.False(t, isFlap)
	assert.Equal(t, int64(1), count)
}

func TestDetector_IsolatedAcrossFingerprints(t *testing.T) {
	d := flap.New(newFakeStore(), nil).WithThreshold(3)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, _, err := d.RecordTransition(ctx, "tenant-1", "fp-A")
		require.NoError(t, err)
	}

	// fp-B should be independent
	isFlap, count, err := d.RecordTransition(ctx, "tenant-1", "fp-B")
	require.NoError(t, err)
	assert.False(t, isFlap)
	assert.Equal(t, int64(1), count)
}

func TestDetector_TTLSetOnlyOnce(t *testing.T) {
	fs := newFakeStore()
	d := flap.New(fs, nil).WithThreshold(10)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, _, err := d.RecordTransition(ctx, "tenant-1", "fp-ttl")
		require.NoError(t, err)
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()
	for _, hasTTL := range fs.hasTTL {
		assert.True(t, hasTTL)
	}
}

func TestDetector_DefaultThreshold(t *testing.T) {
	d := flap.New(newFakeStore(), nil)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		isFlap, _, err := d.RecordTransition(ctx, "tenant-1", "fp-def")
		require.NoError(t, err)
		assert.False(t, isFlap, "should not flap before DefaultThreshold=4")
	}

	isFlap, _, err := d.RecordTransition(ctx, "tenant-1", "fp-def")
	require.NoError(t, err)
	assert.True(t, isFlap, "should flap at DefaultThreshold=4")
}
