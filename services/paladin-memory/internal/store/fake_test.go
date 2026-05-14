package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	memoryv1 "github.com/paladinai/paladinai/gen/go/memory/v1"
	"github.com/paladinai/paladinai/services/paladin-memory/internal/store"
)

// ─── FakeWorkingStore ─────────────────────────────────────────────────────────

func TestFakeWorkingStore_SetAndGet_Hit(t *testing.T) {
	t.Parallel()
	ws := store.NewFakeWorkingStore()

	require.NoError(t, ws.Set(context.Background(), "t1", "s1", "key", "val", time.Minute))
	v, found, err := ws.Get(context.Background(), "t1", "s1", "key")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "val", v)
}

func TestFakeWorkingStore_Get_MissingReturnsFalse(t *testing.T) {
	t.Parallel()
	ws := store.NewFakeWorkingStore()
	_, found, err := ws.Get(context.Background(), "t1", "s1", "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestFakeWorkingStore_SetOverwritesExisting(t *testing.T) {
	t.Parallel()
	ws := store.NewFakeWorkingStore()
	ctx := context.Background()

	require.NoError(t, ws.Set(ctx, "t1", "s1", "k", "old", time.Minute))
	require.NoError(t, ws.Set(ctx, "t1", "s1", "k", "new", time.Minute))

	v, found, err := ws.Get(ctx, "t1", "s1", "k")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "new", v)
}

func TestFakeWorkingStore_TenantIsolation(t *testing.T) {
	t.Parallel()
	ws := store.NewFakeWorkingStore()
	ctx := context.Background()

	require.NoError(t, ws.Set(ctx, "tenant-a", "s", "k", "a-val", time.Minute))
	require.NoError(t, ws.Set(ctx, "tenant-b", "s", "k", "b-val", time.Minute))

	a, foundA, err := ws.Get(ctx, "tenant-a", "s", "k")
	require.NoError(t, err)
	assert.True(t, foundA)
	assert.Equal(t, "a-val", a)

	b, foundB, err := ws.Get(ctx, "tenant-b", "s", "k")
	require.NoError(t, err)
	assert.True(t, foundB)
	assert.Equal(t, "b-val", b)
}

func TestFakeWorkingStore_SessionIsolation(t *testing.T) {
	t.Parallel()
	ws := store.NewFakeWorkingStore()
	ctx := context.Background()

	require.NoError(t, ws.Set(ctx, "t1", "session-a", "k", "for-a", time.Minute))
	require.NoError(t, ws.Set(ctx, "t1", "session-b", "k", "for-b", time.Minute))

	a, _, err := ws.Get(ctx, "t1", "session-a", "k")
	require.NoError(t, err)
	b, _, err := ws.Get(ctx, "t1", "session-b", "k")
	require.NoError(t, err)
	assert.Equal(t, "for-a", a)
	assert.Equal(t, "for-b", b)
}

func TestFakeWorkingStore_Scan_MatchesPrefix(t *testing.T) {
	t.Parallel()
	ws := store.NewFakeWorkingStore()
	ctx := context.Background()

	require.NoError(t, ws.Set(ctx, "t1", "s1", "step:1", "one", time.Minute))
	require.NoError(t, ws.Set(ctx, "t1", "s1", "step:2", "two", time.Minute))
	require.NoError(t, ws.Set(ctx, "t1", "s1", "other", "skip", time.Minute))

	vals, err := ws.Scan(ctx, "t1", "s1", "step:", 10)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"one", "two"}, vals)
}

func TestFakeWorkingStore_Scan_RespectsLimit(t *testing.T) {
	t.Parallel()
	ws := store.NewFakeWorkingStore()
	ctx := context.Background()

	for i := range 5 {
		require.NoError(t, ws.Set(ctx, "t1", "s1", "k:"+string(rune('a'+i)), "v", time.Minute))
	}

	vals, err := ws.Scan(ctx, "t1", "s1", "k:", 2)
	require.NoError(t, err)
	assert.Len(t, vals, 2)
}

func TestFakeWorkingStore_Scan_ZeroLimitDefaultsTwenty(t *testing.T) {
	t.Parallel()
	ws := store.NewFakeWorkingStore()
	ctx := context.Background()

	require.NoError(t, ws.Set(ctx, "t1", "s1", "x", "v", time.Minute))
	vals, err := ws.Scan(ctx, "t1", "s1", "", 0) // limit=0 → default 20
	require.NoError(t, err)
	assert.Len(t, vals, 1)
}

func TestFakeWorkingStore_Scan_EmptyWhenNoMatch(t *testing.T) {
	t.Parallel()
	ws := store.NewFakeWorkingStore()
	ctx := context.Background()

	require.NoError(t, ws.Set(ctx, "t1", "s1", "k", "v", time.Minute))
	vals, err := ws.Scan(ctx, "t1", "s1", "zzz", 10)
	require.NoError(t, err)
	assert.Empty(t, vals)
}

// ─── FakeEpisodicStore extra paths ───────────────────────────────────────────

func TestFakeEpisodicStore_Search_ZeroTopKDefaultsTen(t *testing.T) {
	t.Parallel()
	s := store.NewFakeEpisodicStore()
	ctx := context.Background()
	for i := 0; i < 15; i++ {
		writeEpisode(t, s, "t1", "inc-"+string(rune('a'+i)), "summary", "P1")
	}
	results, err := s.Search(ctx, "t1", "", 0) // 0 → default 10
	require.NoError(t, err)
	assert.Len(t, results, 10)
}

func TestFakeEpisodicStore_Delete_CrossTenantNotDeleted(t *testing.T) {
	t.Parallel()
	s := store.NewFakeEpisodicStore()
	ctx := context.Background()
	incA := "00000000-0000-0000-0000-000000000001"
	writeEpisode(t, s, "t1", incA, "episode for t1", "P1")
	writeEpisode(t, s, "t2", incA, "episode for t2", "P2")

	n, err := s.Delete(ctx, "t1", []string{incA})
	require.NoError(t, err)
	assert.EqualValues(t, 1, n, "only t1's episode should be deleted")

	// t2's episode should still exist
	results, err := s.Search(ctx, "t2", "", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1, "t2's episode must survive t1's delete")
}

func TestFakeEpisodicStore_IsolatesLabels(t *testing.T) {
	t.Parallel()
	s := store.NewFakeEpisodicStore()
	labels := map[string]string{"env": "prod"}
	_, err := s.Write(context.Background(), &memoryv1.WriteEpisodeRequest{
		TenantID:   "t1",
		IncidentID: "inc-labels",
		Summary:    "summary",
		Severity:   "P2",
		Labels:     labels,
	})
	require.NoError(t, err)
	labels["env"] = "mutated"

	results, err := s.Search(context.Background(), "t1", "", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "prod", results[0].Labels["env"])

	results[0].Labels["env"] = "search-mutated"
	again, err := s.Search(context.Background(), "t1", "", 10)
	require.NoError(t, err)
	require.Len(t, again, 1)
	assert.Equal(t, "prod", again[0].Labels["env"])
}

func TestFakeEpisodicStore_Write_NilRequest_ReturnsError(t *testing.T) {
	t.Parallel()
	s := store.NewFakeEpisodicStore()
	_, err := s.Write(context.Background(), nil)
	require.Error(t, err)
}

func TestFakeEpisodicStore_Write_NilLabels_DefaultsToEmpty(t *testing.T) {
	t.Parallel()
	s := store.NewFakeEpisodicStore()
	id, err := s.Write(context.Background(), &memoryv1.WriteEpisodeRequest{
		TenantID:   "t1",
		IncidentID: "inc-nil-labels",
		Summary:    "test",
		Severity:   "P2",
		// Labels intentionally nil
	})
	require.NoError(t, err)
	assert.NotEmpty(t, id)
}
