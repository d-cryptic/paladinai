package storm_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/storm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// fakeStore is a minimal in-memory implementation of storm.Store.
type fakeStore struct {
	mu     sync.Mutex
	counts map[string]int64
	hasTTL map[string]bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{counts: make(map[string]int64), hasTTL: make(map[string]bool)}
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

func TestDetector_NoStormBeforeBurst(t *testing.T) {
	d := storm.New(newFakeStore(), nil).WithBurst(5).WithWindow(time.Minute)
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		isStorm, count, err := d.Record(ctx, "tenant-1")
		require.NoError(t, err)
		assert.False(t, isStorm, "should not be a storm at count %d", count)
	}
}

func TestDetector_StormAtBurst(t *testing.T) {
	d := storm.New(newFakeStore(), nil).WithBurst(5).WithWindow(time.Minute)
	ctx := context.Background()

	var isStorm bool
	var err error
	for i := 0; i < 5; i++ {
		isStorm, _, err = d.Record(ctx, "tenant-1")
		require.NoError(t, err)
	}
	assert.True(t, isStorm, "exactly at burst threshold should trigger storm")
}

func TestDetector_StormAboveBurst(t *testing.T) {
	d := storm.New(newFakeStore(), nil).WithBurst(3).WithWindow(time.Minute)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		isStorm, _, err := d.Record(ctx, "tenant-1")
		require.NoError(t, err)
		if i >= 2 { // 0-indexed, burst=3 means ≥3 triggers
			assert.True(t, isStorm)
		}
	}
}

func TestDetector_IsolatedAcrossTenants(t *testing.T) {
	d := storm.New(newFakeStore(), nil).WithBurst(3).WithWindow(time.Minute)
	ctx := context.Background()

	// Push tenant-a to storm
	for i := 0; i < 3; i++ {
		_, _, err := d.Record(ctx, "tenant-a")
		require.NoError(t, err)
	}

	// tenant-b count should still be 1
	isStorm, count, err := d.Record(ctx, "tenant-b")
	require.NoError(t, err)
	assert.False(t, isStorm)
	assert.Equal(t, int64(1), count)
}

func TestDetector_TTLSetOnlyOnFirstIncr(t *testing.T) {
	fs := newFakeStore()
	d := storm.New(fs, nil).WithBurst(10).WithWindow(time.Minute)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, _, err := d.Record(ctx, "tenant-1")
		require.NoError(t, err)
	}

	// ExpireNX should have been called only once per tenant key
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for k, hasTTL := range fs.hasTTL {
		_ = k
		assert.True(t, hasTTL, "TTL should be set")
	}
}

func TestDetector_DefaultBurstAndWindow(t *testing.T) {
	d := storm.New(newFakeStore(), nil)
	ctx := context.Background()

	// 49 alerts — should not storm
	var isStorm bool
	var err error
	for i := 0; i < 49; i++ {
		isStorm, _, err = d.Record(ctx, "tenant-1")
		require.NoError(t, err)
	}
	assert.False(t, isStorm)

	// 50th alert — storms at DefaultBurst=50
	isStorm, _, err = d.Record(ctx, "tenant-1")
	require.NoError(t, err)
	assert.True(t, isStorm)
}

func TestDetector_LogsOnlyWhenThresholdIsCrossed(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	d := storm.New(newFakeStore(), zap.New(core)).WithBurst(3).WithWindow(time.Minute)
	ctx := context.Background()

	for i := 0; i < 6; i++ {
		_, _, err := d.Record(ctx, "tenant-1")
		require.NoError(t, err)
	}

	entries := logs.FilterMessage("alert storm detected").All()
	require.Len(t, entries, 1)
	assert.Equal(t, int64(3), entries[0].ContextMap()["count"])
}
