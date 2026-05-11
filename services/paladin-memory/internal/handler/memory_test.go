package handler_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	memoryv1 "github.com/paladinai/paladinai/gen/go/memory/v1"
	"github.com/paladinai/paladinai/services/paladin-memory/internal/handler"
	"github.com/paladinai/paladinai/services/paladin-memory/internal/store"
)

func newHandler() (*handler.MemoryHandler, *store.FakeWorkingStore, *store.FakeEpisodicStore) {
	ws := store.NewFakeWorkingStore()
	es := store.NewFakeEpisodicStore()
	h := handler.New(ws, es, 30*time.Minute, nil)
	return h, ws, es
}

func TestSetThenGetWorkingMemory(t *testing.T) {
	t.Parallel()
	h, _, _ := newHandler()
	ctx := context.Background()

	_, err := h.SetWorkingMemory(ctx, &memoryv1.SetWorkingMemoryRequest{
		TenantID:  "t1",
		SessionID: "s1",
		Key:       "step",
		Value:     "first",
	})
	require.NoError(t, err)

	resp, err := h.GetWorkingMemory(ctx, &memoryv1.GetWorkingMemoryRequest{
		TenantID:  "t1",
		SessionID: "s1",
		Key:       "step",
	})
	require.NoError(t, err)
	assert.True(t, resp.Found)
	assert.Equal(t, "first", resp.Value)
}

func TestGetWorkingMemory_MissingReturnsFoundFalse(t *testing.T) {
	t.Parallel()
	h, _, _ := newHandler()

	resp, err := h.GetWorkingMemory(context.Background(), &memoryv1.GetWorkingMemoryRequest{
		TenantID:  "t1",
		SessionID: "s1",
		Key:       "absent",
	})
	require.NoError(t, err)
	assert.False(t, resp.Found)
}

func TestSetWorkingMemory_RejectsMissingFields(t *testing.T) {
	t.Parallel()
	h, _, _ := newHandler()

	_, err := h.SetWorkingMemory(context.Background(), &memoryv1.SetWorkingMemoryRequest{TenantID: "t1"})
	require.Error(t, err)
}

func TestWriteEpisode_ReturnsNonEmptyID(t *testing.T) {
	t.Parallel()
	h, _, _ := newHandler()

	resp, err := h.WriteEpisode(context.Background(), &memoryv1.WriteEpisodeRequest{
		TenantID:    "t1",
		IncidentID:  uuid.NewString(),
		Fingerprint: "fp",
		Summary:     "kafka broker down",
		Severity:    "P1",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.EpisodeID)
}

func TestSearchMemory_DefaultsToEpisodic(t *testing.T) {
	t.Parallel()
	h, _, _ := newHandler()
	ctx := context.Background()

	_, err := h.WriteEpisode(ctx, &memoryv1.WriteEpisodeRequest{
		TenantID:   "t1",
		IncidentID: uuid.NewString(),
		Summary:    "kafka broker down",
		Severity:   "P1",
	})
	require.NoError(t, err)

	resp, err := h.SearchMemory(ctx, &memoryv1.SearchMemoryRequest{
		TenantID: "t1",
		Query:    "kafka",
		TopK:     5,
	})
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, memoryv1.MemoryTypeEpisodic, resp.Results[0].Type)
	assert.Equal(t, "episodic", resp.Results[0].Source)
}

func TestSearchMemory_RoutesByMemoryType(t *testing.T) {
	t.Parallel()
	h, ws, _ := newHandler()
	ctx := context.Background()

	// SearchMemory scans working memory with a "*" session placeholder, so
	// seed under the same placeholder for the test.
	require.NoError(t, ws.Set(ctx, "t1", "*", "note:1", "first", time.Minute))

	resp, err := h.SearchMemory(ctx, &memoryv1.SearchMemoryRequest{
		TenantID:    "t1",
		Query:       "note:",
		MemoryTypes: []memoryv1.MemoryType{memoryv1.MemoryTypeWorking},
		TopK:        5,
	})
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, memoryv1.MemoryTypeWorking, resp.Results[0].Type)
	assert.Equal(t, "working", resp.Results[0].Source)
	assert.Equal(t, "first", resp.Results[0].Content)
}

func TestSearchMemory_RejectsMissingTenant(t *testing.T) {
	t.Parallel()
	h, _, _ := newHandler()
	_, err := h.SearchMemory(context.Background(), &memoryv1.SearchMemoryRequest{Query: "x"})
	require.Error(t, err)
}

func TestDeleteEpisodes_RemovesRows(t *testing.T) {
	t.Parallel()
	h, _, _ := newHandler()
	ctx := context.Background()

	inc := uuid.NewString()
	_, err := h.WriteEpisode(ctx, &memoryv1.WriteEpisodeRequest{
		TenantID:   "t1",
		IncidentID: inc,
		Summary:    "x",
		Severity:   "P3",
	})
	require.NoError(t, err)

	resp, err := h.DeleteEpisodes(ctx, &memoryv1.DeleteEpisodesRequest{
		TenantID:    "t1",
		IncidentIDs: []string{inc},
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, resp.Deleted)
}
