package qdrant

import (
	"strings"
	"testing"
)

func TestChunkText_Empty(t *testing.T) {
	t.Parallel()
	if got := ChunkText("src", "t", "tenant", "   ", nil); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
}

func TestChunkText_Short(t *testing.T) {
	t.Parallel()
	chunks := ChunkText("src", "title", "tenant-1", "one two three", []string{"a"})
	if len(chunks) != 1 {
		t.Fatalf("want 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Content != "one two three" {
		t.Fatalf("content mismatch: %q", chunks[0].Content)
	}
	if chunks[0].TenantID != "tenant-1" {
		t.Fatalf("tenant mismatch: %q", chunks[0].TenantID)
	}
	if chunks[0].ID == "" {
		t.Fatalf("missing ID")
	}
}

func TestChunkText_Overlap(t *testing.T) {
	t.Parallel()
	words := make([]string, 600)
	for i := range words {
		words[i] = "w"
	}
	text := strings.Join(words, " ")
	chunks := ChunkText("src", "title", "tenant", text, nil)
	if len(chunks) != 2 {
		t.Fatalf("want 2 chunks, got %d", len(chunks))
	}
	if got := len(strings.Fields(chunks[0].Content)); got != 500 {
		t.Fatalf("chunk0 words: got %d want 500", got)
	}
	if got := len(strings.Fields(chunks[1].Content)); got != 150 {
		t.Fatalf("chunk1 words: got %d want 150", got)
	}
	if chunks[0].ID == chunks[1].ID {
		t.Fatalf("chunk IDs should be distinct")
	}
}

func TestChunkID_Stable(t *testing.T) {
	t.Parallel()
	a := ChunkID("src", 3)
	b := ChunkID("src", 3)
	if a != b {
		t.Fatalf("ChunkID not stable: %s vs %s", a, b)
	}
	if a == ChunkID("src", 4) {
		t.Fatalf("ChunkID collided across indexes")
	}
}

func TestToPoint(t *testing.T) {
	t.Parallel()
	c := RunbookChunk{ID: "id1", Source: "s", Title: "t", Content: "c", ChunkIndex: 2, TenantID: "tenant", Tags: []string{"x"}}
	p := c.ToPoint([]float32{1, 2, 3})
	if p.ID != "id1" || len(p.Vector) != 3 {
		t.Fatalf("unexpected point: %+v", p)
	}
	if p.Payload["tenant_id"] != "tenant" {
		t.Fatalf("payload tenant_id mismatch: %v", p.Payload["tenant_id"])
	}
}
