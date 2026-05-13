package qdrant

import (
	"context"
	"fmt"
)

// PointStore is the subset of Client methods the Indexer depends on. Defined
// here so tests can substitute an in-memory fake.
type PointStore interface {
	Upsert(ctx context.Context, collection string, points []Point) error
	// Search performs a vector similarity search. filter, when non-nil, is passed
	// to the backing store as a server-side predicate (e.g. Qdrant must-match).
	Search(ctx context.Context, collection string, vector []float32, topK int, filter map[string]any) ([]SearchResult, error)
}

// NoopClient is a PointStore implementation that discards all writes and returns
// empty results. Used when QDRANT_URL is not configured.
type NoopClient struct{}

func (NoopClient) Upsert(_ context.Context, _ string, _ []Point) error { return nil }
func (NoopClient) Search(_ context.Context, _ string, _ []float32, _ int, _ map[string]any) ([]SearchResult, error) {
	return nil, nil
}

// Indexer ingests documents into Qdrant and serves semantic search.
type Indexer struct {
	client     PointStore
	embedder   Embedder
	collection string
}

// NewIndexer constructs an Indexer targeting RunbookCollection by default.
func NewIndexer(client PointStore, embedder Embedder) *Indexer {
	return &Indexer{client: client, embedder: embedder, collection: RunbookCollection}
}

// WithCollection returns a new Indexer targeting the given collection name.
func (idx *Indexer) WithCollection(name string) *Indexer {
	c := *idx
	c.collection = name
	return &c
}

// Index chunks the document, embeds each chunk, and upserts them into the
// runbook collection. Returns the number of chunks indexed.
func (idx *Indexer) Index(ctx context.Context, source, title, tenantID, text string, tags []string) (int, error) {
	chunks := ChunkText(source, title, tenantID, text, tags)
	if len(chunks) == 0 {
		return 0, nil
	}
	points := make([]Point, 0, len(chunks))
	for _, chunk := range chunks {
		vec, err := idx.embedder.Embed(ctx, chunk.Content)
		if err != nil {
			return 0, fmt.Errorf("indexer: embed chunk %d: %w", chunk.ChunkIndex, err)
		}
		points = append(points, chunk.ToPoint(vec))
	}
	if err := idx.client.Upsert(ctx, idx.collection, points); err != nil {
		return 0, fmt.Errorf("indexer: upsert: %w", err)
	}
	return len(points), nil
}

// Search finds runbook chunks semantically similar to the query, filtered by
// tenant ID server-side via a Qdrant must-match predicate.
func (idx *Indexer) Search(ctx context.Context, query, tenantID string, topK int) ([]RunbookChunk, error) {
	vec, err := idx.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("indexer: embed query: %w", err)
	}
	filter := map[string]any{
		"must": []map[string]any{
			{"key": "tenant_id", "match": map[string]any{"value": tenantID}},
		},
	}
	results, err := idx.client.Search(ctx, idx.collection, vec, topK, filter)
	if err != nil {
		return nil, fmt.Errorf("indexer: search: %w", err)
	}
	chunks := make([]RunbookChunk, 0, len(results))
	for _, r := range results {
		chunks = append(chunks, RunbookChunk{
			ID:       r.ID,
			Source:   getString(r.Payload, "source"),
			Title:    getString(r.Payload, "title"),
			Content:  getString(r.Payload, "content"),
			TenantID: tenantID,
		})
	}
	return chunks, nil
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
