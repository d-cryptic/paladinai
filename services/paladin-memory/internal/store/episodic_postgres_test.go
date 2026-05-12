package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	memoryv1 "github.com/paladinai/paladinai/gen/go/memory/v1"
	"github.com/paladinai/paladinai/services/paladin-memory/internal/store"
)

// newPostgresStore creates a PostgresEpisodicStore with a nil pool.
// The nil pool is safe for tests that only exercise parameter validation
// (the pool is never accessed before the validation returns an error).
func newPostgresStore() *store.PostgresEpisodicStore {
	return store.NewPostgresEpisodicStore(nil)
}

func TestPostgresEpisodicStore_Write_NilRequest(t *testing.T) {
	s := newPostgresStore()
	_, err := s.Write(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil request")
}

func TestPostgresEpisodicStore_Write_MissingTenantID(t *testing.T) {
	s := newPostgresStore()
	_, err := s.Write(context.Background(), &memoryv1.WriteEpisodeRequest{
		IncidentID: "inc-1",
		Summary:    "test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant_id")
}

func TestPostgresEpisodicStore_Write_MissingIncidentID(t *testing.T) {
	s := newPostgresStore()
	_, err := s.Write(context.Background(), &memoryv1.WriteEpisodeRequest{
		TenantID: "t1",
		Summary:  "test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incident_id")
}

func TestPostgresEpisodicStore_Search_EmptyTenantID(t *testing.T) {
	s := newPostgresStore()
	_, err := s.Search(context.Background(), "", "query", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant_id")
}

func TestPostgresEpisodicStore_Delete_EmptyTenantID(t *testing.T) {
	s := newPostgresStore()
	_, err := s.Delete(context.Background(), "", []string{"inc-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant_id")
}

func TestPostgresEpisodicStore_Delete_EmptyIncidentIDs_ReturnsZero(t *testing.T) {
	s := newPostgresStore()
	n, err := s.Delete(context.Background(), "t1", nil)
	require.NoError(t, err)
	assert.EqualValues(t, 0, n)

	n, err = s.Delete(context.Background(), "t1", []string{})
	require.NoError(t, err)
	assert.EqualValues(t, 0, n)
}

func TestPostgresEpisodicStore_Delete_InvalidUUID_ReturnsError(t *testing.T) {
	s := newPostgresStore()
	_, err := s.Delete(context.Background(), "t1", []string{"not-a-uuid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid incident_id")
}
