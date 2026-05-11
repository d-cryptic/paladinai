package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	memoryv1 "github.com/paladinai/paladinai/gen/go/memory/v1"
	"github.com/paladinai/paladinai/services/paladin-memory/internal/store"
)

func writeEpisode(t *testing.T, s store.EpisodicStore, tenant, incident, summary, severity string) string {
	t.Helper()
	id, err := s.Write(context.Background(), &memoryv1.WriteEpisodeRequest{
		TenantID:    tenant,
		IncidentID:  incident,
		Fingerprint: "fp-" + incident,
		Summary:     summary,
		Severity:    severity,
		Labels:      map[string]string{"env": "prod"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, id)
	return id
}

func TestFakeEpisodicStore_WriteReturnsID(t *testing.T) {
	t.Parallel()
	s := store.NewFakeEpisodicStore()
	id := writeEpisode(t, s, "t1", uuid.NewString(), "kafka down", "P1")
	assert.NotEmpty(t, id)
}

func TestFakeEpisodicStore_WriteRejectsMissingFields(t *testing.T) {
	t.Parallel()
	s := store.NewFakeEpisodicStore()
	_, err := s.Write(context.Background(), &memoryv1.WriteEpisodeRequest{Summary: "x"})
	require.Error(t, err)
}

func TestFakeEpisodicStore_SearchFiltersByTenant(t *testing.T) {
	t.Parallel()
	s := store.NewFakeEpisodicStore()
	writeEpisode(t, s, "t1", uuid.NewString(), "kafka down", "P1")
	writeEpisode(t, s, "t2", uuid.NewString(), "kafka down", "P1")

	results, err := s.Search(context.Background(), "t1", "kafka", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "t1", results[0].TenantID)
}

func TestFakeEpisodicStore_SearchEmptyQueryReturnsAll(t *testing.T) {
	t.Parallel()
	s := store.NewFakeEpisodicStore()
	writeEpisode(t, s, "t1", uuid.NewString(), "a", "P1")
	writeEpisode(t, s, "t1", uuid.NewString(), "b", "P2")

	results, err := s.Search(context.Background(), "t1", "", 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestFakeEpisodicStore_SearchRespectsTopK(t *testing.T) {
	t.Parallel()
	s := store.NewFakeEpisodicStore()
	for i := 0; i < 5; i++ {
		writeEpisode(t, s, "t1", uuid.NewString(), "x", "P1")
	}

	results, err := s.Search(context.Background(), "t1", "", 3)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestFakeEpisodicStore_DeleteByIncidentIDs(t *testing.T) {
	t.Parallel()
	s := store.NewFakeEpisodicStore()
	incA := uuid.NewString()
	incB := uuid.NewString()
	writeEpisode(t, s, "t1", incA, "a", "P1")
	writeEpisode(t, s, "t1", incB, "b", "P2")

	n, err := s.Delete(context.Background(), "t1", []string{incA})
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)

	results, err := s.Search(context.Background(), "t1", "", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, incB, results[0].IncidentID)
}

func TestFakeEpisodicStore_DeleteNoOpForEmptyList(t *testing.T) {
	t.Parallel()
	s := store.NewFakeEpisodicStore()
	n, err := s.Delete(context.Background(), "t1", nil)
	require.NoError(t, err)
	assert.EqualValues(t, 0, n)
}
