package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	memoryv1 "github.com/paladinai/paladinai/gen/go/memory/v1"
	"github.com/paladinai/paladinai/services/paladin-memory/internal/store"
)

// newUnreachablePool returns a pgx pool aimed at a port nothing listens on so
// every Query/Exec returns a connection error within the context deadline.
func newUnreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig("postgres://nobody:nobody@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	require.NoError(t, err)
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func TestPostgresEpisodicStore_Write_ConnectionFailure(t *testing.T) {
	s := store.NewPostgresEpisodicStore(newUnreachablePool(t))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := s.Write(ctx, &memoryv1.WriteEpisodeRequest{
		TenantID:   "t1",
		IncidentID: uuid.NewString(),
		Summary:    "boom",
		RootCause:  "rc",
		Resolution: "res",
		Severity:   "P1",
		Labels:     map[string]string{"env": "prod"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "memory: episodic write")
}

func TestPostgresEpisodicStore_Write_NilLabelsDefaulted(t *testing.T) {
	s := store.NewPostgresEpisodicStore(newUnreachablePool(t))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Exercises the `labels == nil` branch (defaulting to empty map) before
	// the pool call fails.
	_, err := s.Write(ctx, &memoryv1.WriteEpisodeRequest{
		TenantID:   "t1",
		IncidentID: uuid.NewString(),
		Summary:    "boom",
		Severity:   "P1",
	})
	require.Error(t, err)
}

func TestPostgresEpisodicStore_Search_ConnectionFailure(t *testing.T) {
	s := store.NewPostgresEpisodicStore(newUnreachablePool(t))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := s.Search(ctx, "t1", "query", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "memory: episodic search")
}

func TestPostgresEpisodicStore_Search_DefaultsTopKWhenZero(t *testing.T) {
	s := store.NewPostgresEpisodicStore(newUnreachablePool(t))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// topK=0 hits the `topK = 10` default branch before query fails.
	_, err := s.Search(ctx, "t1", "", 0)
	require.Error(t, err)
}

func TestPostgresEpisodicStore_Delete_ConnectionFailure(t *testing.T) {
	s := store.NewPostgresEpisodicStore(newUnreachablePool(t))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := s.Delete(ctx, "t1", []string{uuid.NewString()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "memory: episodic delete")
}
