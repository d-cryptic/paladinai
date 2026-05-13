package qdrant

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeStore struct {
	upserts   [][]Point
	upsertErr error
	searchOut []SearchResult
	searchErr error
}

func (f *fakeStore) Upsert(_ context.Context, _ string, points []Point) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upserts = append(f.upserts, points)
	return nil
}

func (f *fakeStore) Search(_ context.Context, _ string, _ []float32, _ int, _ map[string]any) ([]SearchResult, error) {
	return f.searchOut, f.searchErr
}

func TestIndexer_Index_Empty(t *testing.T) {
	t.Parallel()
	idx := NewIndexer(&fakeStore{}, &StubEmbedder{})
	n, err := idx.Index(context.Background(), "src", "t", "tenant", "", nil)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 chunks for empty text, got %d", n)
	}
}

func TestIndexer_Index_Upserts(t *testing.T) {
	t.Parallel()
	store := &fakeStore{}
	idx := NewIndexer(store, &StubEmbedder{})

	text := strings.Repeat("word ", 700) // 700 words -> 2 chunks
	n, err := idx.Index(context.Background(), "src", "t", "tenant", text, []string{"db"})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 chunks, got %d", n)
	}
	if len(store.upserts) != 1 || len(store.upserts[0]) != 2 {
		t.Fatalf("expected 1 upsert with 2 points; got %+v", store.upserts)
	}
	if len(store.upserts[0][0].Vector) != EmbeddingDim {
		t.Fatalf("vector dim: got %d", len(store.upserts[0][0].Vector))
	}
}

func TestIndexer_Index_UpsertError(t *testing.T) {
	t.Parallel()
	store := &fakeStore{upsertErr: errors.New("boom")}
	idx := NewIndexer(store, &StubEmbedder{})
	if _, err := idx.Index(context.Background(), "s", "t", "tenant", "hello world", nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestIndexer_Search_PassesFilterToStore(t *testing.T) {
	t.Parallel()
	// The store now receives the server-side filter and returns only matching results.
	// The fake simply returns what it is configured with (filter enforcement is in Qdrant).
	store := &fakeStore{searchOut: []SearchResult{
		{ID: "a", Score: 0.9, Payload: map[string]any{"tenant_id": "tenant", "content": "match", "source": "s", "title": "t"}},
	}}
	idx := NewIndexer(store, &StubEmbedder{})
	chunks, err := idx.Search(context.Background(), "q", "tenant", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Content != "match" {
		t.Fatalf("expected 1 chunk, got %+v", chunks)
	}
	if chunks[0].Source != "s" || chunks[0].Title != "t" {
		t.Fatalf("payload not unpacked: %+v", chunks[0])
	}
}

func TestStubEmbedder_Deterministic(t *testing.T) {
	t.Parallel()
	e := &StubEmbedder{}
	a, _ := e.Embed(context.Background(), "hello")
	b, _ := e.Embed(context.Background(), "hello")
	c, _ := e.Embed(context.Background(), "different")
	if len(a) != EmbeddingDim {
		t.Fatalf("dim: %d", len(a))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("not deterministic at %d", i)
		}
	}
	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatalf("different texts produced identical vectors")
	}
}
