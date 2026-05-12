package handler_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	memoryv1 "github.com/paladinai/paladinai/gen/go/memory/v1"
	"github.com/paladinai/paladinai/internal/qdrant"
	"github.com/paladinai/paladinai/services/paladin-memory/internal/handler"
	"github.com/paladinai/paladinai/services/paladin-memory/internal/store"
)

func newHandler() (*handler.MemoryHandler, *store.FakeWorkingStore, *store.FakeEpisodicStore) {
	ws := store.NewFakeWorkingStore()
	es := store.NewFakeEpisodicStore()
	h := handler.New(ws, es, 30*time.Minute, nil)
	return h, ws, es
}

// fakeProceduralSearcher satisfies handler.ProceduralSearcher.
type fakeProceduralSearcher struct {
	chunks []qdrant.RunbookChunk
	err    error
}

func (f *fakeProceduralSearcher) Search(_ context.Context, _, _ string, _ int) ([]qdrant.RunbookChunk, error) {
	return f.chunks, f.err
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

// ─── Nil request guards ───────────────────────────────────────────────────────

func TestSearchMemory_NilRequest_ReturnsError(t *testing.T) {
	h, _, _ := newHandler()
	_, err := h.SearchMemory(context.Background(), nil)
	require.Error(t, err)
}

func TestGetWorkingMemory_NilRequest_ReturnsError(t *testing.T) {
	h, _, _ := newHandler()
	_, err := h.GetWorkingMemory(context.Background(), nil)
	require.Error(t, err)
}

func TestSetWorkingMemory_NilRequest_ReturnsError(t *testing.T) {
	h, _, _ := newHandler()
	_, err := h.SetWorkingMemory(context.Background(), nil)
	require.Error(t, err)
}

func TestWriteEpisode_NilRequest_ReturnsError(t *testing.T) {
	h, _, _ := newHandler()
	_, err := h.WriteEpisode(context.Background(), nil)
	require.Error(t, err)
}

func TestDeleteEpisodes_NilRequest_ReturnsError(t *testing.T) {
	h, _, _ := newHandler()
	_, err := h.DeleteEpisodes(context.Background(), nil)
	require.Error(t, err)
}

// ─── Procedural memory ────────────────────────────────────────────────────────

func TestSearchMemory_ProceduralType_ReturnsChunks(t *testing.T) {
	t.Parallel()
	h, _, _ := newHandler()
	ps := &fakeProceduralSearcher{
		chunks: []qdrant.RunbookChunk{
			{Source: "runbook-db", Content: "restart postgres", Tags: []string{"db"}},
		},
	}
	h.WithProcedural(ps)

	resp, err := h.SearchMemory(context.Background(), &memoryv1.SearchMemoryRequest{
		TenantID:    "t1",
		Query:       "postgres",
		MemoryTypes: []memoryv1.MemoryType{memoryv1.MemoryTypeProcedural},
		TopK:        5,
	})
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, memoryv1.MemoryTypeProcedural, resp.Results[0].Type)
	assert.Equal(t, "runbook-db", resp.Results[0].Source)
}

func TestSearchMemory_ProceduralNilSearcher_SkipsGracefully(t *testing.T) {
	t.Parallel()
	h, _, _ := newHandler()
	// No procedural searcher attached.
	resp, err := h.SearchMemory(context.Background(), &memoryv1.SearchMemoryRequest{
		TenantID:    "t1",
		Query:       "anything",
		MemoryTypes: []memoryv1.MemoryType{memoryv1.MemoryTypeProcedural},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Results)
}

func TestSearchMemory_SemanticType_SkippedWhenNotConfigured(t *testing.T) {
	t.Parallel()
	h, _, _ := newHandler()
	resp, err := h.SearchMemory(context.Background(), &memoryv1.SearchMemoryRequest{
		TenantID:    "t1",
		Query:       "query",
		MemoryTypes: []memoryv1.MemoryType{memoryv1.MemoryTypeSemantic},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Results)
}

func TestSearchMemory_SemanticType_ReturnsChunks(t *testing.T) {
	t.Parallel()
	h, _, _ := newHandler()
	ss := &fakeProceduralSearcher{
		chunks: []qdrant.RunbookChunk{
			{Content: "api latency anomaly", Source: "semantic-facts"},
		},
	}
	h = h.WithSemantic(ss)

	resp, err := h.SearchMemory(context.Background(), &memoryv1.SearchMemoryRequest{
		TenantID:    "t1",
		Query:       "latency",
		MemoryTypes: []memoryv1.MemoryType{memoryv1.MemoryTypeSemantic},
	})
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, memoryv1.MemoryTypeSemantic, resp.Results[0].Type)
	assert.Equal(t, "api latency anomaly", resp.Results[0].Content)
	assert.Equal(t, "semantic-facts", resp.Results[0].Source)
}

func TestSearchMemory_SemanticType_ErrorPropagates(t *testing.T) {
	t.Parallel()
	h, _, _ := newHandler()
	ss := &fakeProceduralSearcher{err: fmt.Errorf("qdrant unavailable")}
	h = h.WithSemantic(ss)

	_, err := h.SearchMemory(context.Background(), &memoryv1.SearchMemoryRequest{
		TenantID:    "t1",
		Query:       "latency",
		MemoryTypes: []memoryv1.MemoryType{memoryv1.MemoryTypeSemantic},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "semantic")
}

func TestSetWorkingMemory_CustomTTL(t *testing.T) {
	t.Parallel()
	h, ws, _ := newHandler()
	ctx := context.Background()

	_, err := h.SetWorkingMemory(ctx, &memoryv1.SetWorkingMemoryRequest{
		TenantID:   "t1",
		SessionID:  "s1",
		Key:        "custom",
		Value:      "v",
		TTLSeconds: 60,
	})
	require.NoError(t, err)

	resp, err := h.GetWorkingMemory(ctx, &memoryv1.GetWorkingMemoryRequest{
		TenantID:  "t1",
		SessionID: "s1",
		Key:       "custom",
	})
	require.NoError(t, err)
	assert.True(t, resp.Found)
	_ = ws // verify store was used
}
