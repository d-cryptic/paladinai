package qdrant

import (
	"context"
	"fmt"
)

// PointStore is the subset of Client methods the Indexer depends on. Defined
// here so tests can substitute an in-memory fake.
type PointStore interface {
	Upsert(ctx context.Context, collection string, points []Point) error
	Search(ctx context.Context, collection string, vector []float32, topK int) ([]SearchResult, error)
}

// Indexer ingests runbook documents into Qdrant and serves semantic search.
type Indexer struct {
	client   PointStore
	embedder Embedder
}

// NewIndexer constructs an Indexer.
func NewIndexer(client PointStore, embedder Embedder) *Indexer {
	return &Indexer{client: client, embedder: embedder}
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
	if err := idx.client.Upsert(ctx, RunbookCollection, points); err != nil {
		return 0, fmt.Errorf("indexer: upsert: %w", err)
	}
	return len(points), nil
}

// Search finds runbook chunks semantically similar to the query, filtering by
// tenant ID via the payload.
func (idx *Indexer) Search(ctx context.Context, query, tenantID string, topK int) ([]RunbookChunk, error) {
	vec, err := idx.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("indexer: embed query: %w", err)
	}
	results, err := idx.client.Search(ctx, RunbookCollection, vec, topK)
	if err != nil {
		return nil, fmt.Errorf("indexer: search: %w", err)
	}
	chunks := make([]RunbookChunk, 0, len(results))
	for _, r := range results {
		if tid, _ := r.Payload["tenant_id"].(string); tid != tenantID {
			continue
		}
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
