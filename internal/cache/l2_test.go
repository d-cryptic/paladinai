package cache

import (
	"context"
	"encoding/binary"
	"math"
	"sync"
	"testing"
	"time"
)

// TestMemL2_MissOnEmpty verifies a fresh cache returns no hit.
func TestMemL2_MissOnEmpty(t *testing.T) {
	l2 := NewMemL2(NewMemEmbedder(4))
	key, err := l2.Lookup(context.Background(), "t1", "high memory usage on redis-main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "" {
		t.Fatalf("expected miss, got l1 key %q", key)
	}
}

// TestMemL2_ExactHit verifies the same text returns the stored L1 key.
func TestMemL2_ExactHit(t *testing.T) {
	emb := NewMemEmbedder(4)
	l2 := NewMemL2(emb)
	ctx := context.Background()

	const query = "pod redis-main-2 OOMKilled"
	const l1Key = "llm:l1:abc123"

	if err := l2.Store(ctx, "t1", query, l1Key); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := l2.Lookup(ctx, "t1", query)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got != l1Key {
		t.Errorf("expected %q, got %q", l1Key, got)
	}
}

// TestMemL2_SemanticHit verifies a semantically similar (same embedding) query hits.
func TestMemL2_SemanticHit(t *testing.T) {
	emb := NewMemEmbedder(4)
	l2 := NewMemL2(emb)
	ctx := context.Background()

	original := "high memory usage on pod redis-main-2"
	similar := "redis-main-2 pod memory at 94 percent"
	const l1Key = "llm:l1:def456"

	// Register similar text to return the same vector.
	if err := l2.Store(ctx, "t1", original, l1Key); err != nil {
		t.Fatalf("Store: %v", err)
	}
	emb.SimilarTo(original, similar)

	got, err := l2.Lookup(ctx, "t1", similar)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got != l1Key {
		t.Errorf("similar query should hit: expected %q, got %q", l1Key, got)
	}
}

// TestMemL2_OrthogonalMiss verifies dissimilar queries do not hit.
func TestMemL2_OrthogonalMiss(t *testing.T) {
	emb := NewMemEmbedder(4)
	l2 := NewMemL2(emb)
	ctx := context.Background()

	if err := l2.Store(ctx, "t1", "query A", "llm:l1:aaa"); err != nil {
		t.Fatalf("Store: %v", err)
	}
	// "query B" gets a different (orthogonal) embedding from MemEmbedder.
	got, err := l2.Lookup(ctx, "t1", "query B")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got != "" {
		t.Errorf("orthogonal query should miss, got %q", got)
	}
}

// TestMemL2_TenantIsolation verifies tenant A's entries are invisible to tenant B.
func TestMemL2_TenantIsolation(t *testing.T) {
	emb := NewMemEmbedder(4)
	l2 := NewMemL2(emb)
	ctx := context.Background()

	query := "disk pressure on node worker-1"
	if err := l2.Store(ctx, "tenant-a", query, "llm:l1:t-a-key"); err != nil {
		t.Fatalf("Store: %v", err)
	}
	// tenant-b uses the same query text (same vector) but must not get a hit.
	got, err := l2.Lookup(ctx, "tenant-b", query)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got != "" {
		t.Errorf("tenant-b should not see tenant-a entry, got %q", got)
	}
}

// TestMemL2_Expiry verifies entries are not returned after TTL.
func TestMemL2_Expiry(t *testing.T) {
	emb := NewMemEmbedder(4)
	l2 := NewMemL2(emb)
	ctx := context.Background()

	query := "network timeout on ingress-nginx"
	if err := l2.Store(ctx, "t1", query, "llm:l1:exp-key"); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Manually expire the entry.
	l2.mu.Lock()
	for i := range l2.entries["t1"] {
		l2.entries["t1"][i].expiresAt = time.Now().Add(-time.Second)
	}
	l2.mu.Unlock()

	got, err := l2.Lookup(ctx, "t1", query)
	if err != nil {
		t.Fatalf("Lookup after expiry: %v", err)
	}
	if got != "" {
		t.Errorf("expired entry should be a miss, got %q", got)
	}
}

// TestMemL2_MultipleEntriesPicksBest verifies the entry with the highest cosine similarity wins.
func TestMemL2_MultipleEntriesPicksBest(t *testing.T) {
	emb := NewMemEmbedder(4)
	l2 := NewMemL2(emb)
	ctx := context.Background()

	// Store two unrelated queries with different vectors.
	if err := l2.Store(ctx, "t1", "query X", "llm:l1:x"); err != nil {
		t.Fatalf("Store X: %v", err)
	}
	if err := l2.Store(ctx, "t1", "query Y", "llm:l1:y"); err != nil {
		t.Fatalf("Store Y: %v", err)
	}

	// Make a new query similar to X but not Y.
	const lookupQuery = "lookup for X-like"
	emb.SimilarTo("query X", lookupQuery)

	got, err := l2.Lookup(ctx, "t1", lookupQuery)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got != "llm:l1:x" {
		t.Errorf("should pick X (most similar), got %q", got)
	}
}

// TestMemEmbedder_SameTextSameVector verifies idempotent embedding.
func TestMemEmbedder_SameTextSameVector(t *testing.T) {
	emb := NewMemEmbedder(8)
	ctx := context.Background()

	v1, err := emb.Embed(ctx, "hello world")
	if err != nil {
		t.Fatal(err)
	}
	v2, err := emb.Embed(ctx, "hello world")
	if err != nil {
		t.Fatal(err)
	}
	for i := range v1 {
		if v1[i] != v2[i] {
			t.Errorf("same text should produce same vector, diff at index %d", i)
		}
	}
}

// TestCosineSimilarity verifies the similarity helper with known values.
func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b []float32
		want float64
	}{
		{
			name: "identical vectors",
			a:    []float32{1, 0, 0},
			b:    []float32{1, 0, 0},
			want: 1.0,
		},
		{
			name: "orthogonal",
			a:    []float32{1, 0, 0},
			b:    []float32{0, 1, 0},
			want: 0.0,
		},
		{
			name: "opposite",
			a:    []float32{1, 0, 0},
			b:    []float32{-1, 0, 0},
			want: -1.0,
		},
		{
			name: "45 degrees",
			a:    []float32{1, 0},
			b:    []float32{1, 1},
			want: 0.7071, // 1/sqrt(2)
		},
		{
			name: "zero vector",
			a:    []float32{0, 0},
			b:    []float32{1, 0},
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			diff := got - tt.want
			if diff < -0.001 || diff > 0.001 {
				t.Errorf("cosineSimilarity = %.4f, want %.4f", got, tt.want)
			}
		})
	}
}

// TestFloat32SliceToBytes verifies serialisation round-trip with bit-level accuracy.
func TestFloat32SliceToBytes(t *testing.T) {
	v := []float32{1.0, 0.5, -0.25, 0.0}
	b := float32SliceToBytes(v)
	if len(b) != len(v)*4 {
		t.Fatalf("expected %d bytes, got %d", len(v)*4, len(b))
	}
	// Verify round-trip: decode little-endian bytes back to float32.
	for i, expected := range v {
		bits := binary.LittleEndian.Uint32(b[i*4:])
		got := math.Float32frombits(bits)
		if got != expected {
			t.Errorf("round-trip mismatch at index %d: got %v, want %v", i, got, expected)
		}
	}
}

// TestMemL2_EmptyTenantID verifies that empty tenantID returns an error.
func TestMemL2_EmptyTenantID(t *testing.T) {
	l2 := NewMemL2(NewMemEmbedder(4))
	_, err := l2.Lookup(context.Background(), "", "some query")
	if err == nil {
		t.Fatal("expected error for empty tenantID in Lookup")
	}
	if err := l2.Store(context.Background(), "", "some query", "k"); err == nil {
		t.Fatal("expected error for empty tenantID in Store")
	}
}

// TestMemL2_ConcurrentStoreAndLookup verifies race freedom.
func TestMemL2_ConcurrentStoreAndLookup(t *testing.T) {
	emb := NewMemEmbedder(8)
	l2 := NewMemL2(emb)
	ctx := context.Background()

	// Pre-embed "query C" so SimilarTo can be called.
	if err := l2.Store(ctx, "t1", "query C", "llm:l1:c"); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			l2.Store(ctx, "t1", "concurrent store", "llm:l1:conc") //nolint:errcheck
		}()
		go func() {
			defer wg.Done()
			l2.Lookup(ctx, "t1", "concurrent lookup") //nolint:errcheck
		}()
	}
	wg.Wait()
}

// TestMemL2_IdempotentStore verifies re-storing the same l1Key works correctly.
func TestMemL2_IdempotentStore(t *testing.T) {
	emb := NewMemEmbedder(4)
	l2 := NewMemL2(emb)
	ctx := context.Background()

	const query = "same query stored twice"
	const l1Key = "llm:l1:idem"

	for i := 0; i < 2; i++ {
		if err := l2.Store(ctx, "t1", query, l1Key); err != nil {
			t.Fatalf("Store #%d: %v", i, err)
		}
	}
	got, err := l2.Lookup(ctx, "t1", query)
	if err != nil {
		t.Fatal(err)
	}
	if got != l1Key {
		t.Errorf("idempotent store: expected %q, got %q", l1Key, got)
	}
}
