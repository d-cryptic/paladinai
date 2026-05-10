// Package cache — L2 semantic LLM cache.
//
// L2 sits between L1 (exact SHA-256 match) and the LLM API. It uses HNSW
// vector similarity to serve cached responses for semantically equivalent but
// textually distinct triage queries.
//
// Design constraints from Stage 9:
//   - Cosine similarity threshold: >= 0.92 (distance <= 0.08)
//   - Cross-tenant hits are impossible by construction (one index per tenant)
//   - L2 does NOT store response bodies; it stores (vector → L1 key) mappings
//   - If the L1 key referenced by an L2 entry has expired, it is a stale miss
//   - TTL: 6 hours per L2 entry
package cache

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	l2TTL       = 6 * time.Hour
	l2Threshold = float64(0.92) // minimum cosine similarity for a hit
	l2Dim       = 1024          // BGE-M3 embedding dimension

	// l2MaxDistance is the HNSW KNN distance threshold: 1 - cosine threshold.
	// Valkey-search uses cosine DISTANCE (lower = more similar), not similarity.
	l2MaxDistance = 1.0 - l2Threshold // 0.08
)

// Embedder converts a text string into a fixed-dimension float32 vector.
// The production implementation calls BGE-M3 via Triton; tests use MemEmbedder.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dim() int
}

// L2Cache is the interface for the semantic LLM response cache.
type L2Cache interface {
	// Lookup embeds query and searches the tenant's HNSW index.
	// Returns the L1 key if a semantically similar entry exists; otherwise "".
	Lookup(ctx context.Context, tenantID, queryText string) (l1Key string, err error)
	// Store indexes (embedding of queryText → l1Key) for future lookups.
	Store(ctx context.Context, tenantID, queryText, l1Key string) error
}

// --- ValkeyL2 (production) ---------------------------------------------------

// ValkeyL2 uses Valkey Search (Redis-compatible HNSW index) for semantic lookup.
// One index per tenant: "llm:l2:{tenantID}".
// Entries are stored as HASH keys under prefix "llm:l2:{tenantID}:{sha256hex}".
type ValkeyL2 struct {
	rdb       *redis.Client
	embedder  Embedder
	mu        sync.Mutex
	indexSeen map[string]struct{} // tracks which tenant indexes have been created
}

// NewValkeyL2 creates a new production L2 cache backed by Valkey Search.
func NewValkeyL2(rdb *redis.Client, embedder Embedder) *ValkeyL2 {
	return &ValkeyL2{
		rdb:       rdb,
		embedder:  embedder,
		indexSeen: make(map[string]struct{}),
	}
}

// Lookup embeds query, runs a KNN search in the tenant's HNSW index, and
// returns the L1 key of the nearest match if cosine similarity >= 0.92.
func (c *ValkeyL2) Lookup(ctx context.Context, tenantID, queryText string) (string, error) {
	if err := c.ensureIndex(ctx, tenantID); err != nil {
		return "", fmt.Errorf("l2 ensure index: %w", err)
	}

	vec, err := c.embedder.Embed(ctx, queryText)
	if err != nil {
		return "", fmt.Errorf("l2 embed: %w", err)
	}

	// KNN query: "*=>[KNN 1 @embedding $vec AS __score]"
	query := "*=>[KNN 1 @embedding $vec AS __score]"
	result, err := c.rdb.FTSearchWithArgs(ctx, l2IndexName(tenantID), query, &redis.FTSearchOptions{
		Params: map[string]any{"vec": float32SliceToBytes(vec)},
		SortBy: []redis.FTSearchSortBy{{FieldName: "__score", Asc: true}},
		Limit:  1,
		Return: []redis.FTSearchReturn{
			{FieldName: "l1_key"},
			{FieldName: "__score"},
		},
		DialectVersion: 2,
	}).Result()
	if err != nil {
		if isIndexMissingErr(err) {
			return "", nil // index not yet populated
		}
		return "", fmt.Errorf("l2 search: %w", err)
	}

	if result.Total == 0 || len(result.Docs) == 0 {
		return "", nil
	}

	doc := result.Docs[0]
	scoreStr, ok := doc.Fields["__score"]
	if !ok {
		return "", nil
	}
	distance, err := parseFloat(scoreStr)
	if err != nil || distance > l2MaxDistance {
		return "", nil // below threshold
	}

	l1Key, _ := doc.Fields["l1_key"]
	if l1Key == "" {
		return "", nil
	}
	return l1Key, nil
}

// Store adds an (embedding → l1Key) entry to the tenant's HNSW index.
func (c *ValkeyL2) Store(ctx context.Context, tenantID, queryText, l1Key string) error {
	if err := c.ensureIndex(ctx, tenantID); err != nil {
		return fmt.Errorf("l2 ensure index: %w", err)
	}

	vec, err := c.embedder.Embed(ctx, queryText)
	if err != nil {
		return fmt.Errorf("l2 embed: %w", err)
	}

	hashKey := l2EntryKey(tenantID, l1Key)
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, hashKey,
		"embedding", float32SliceToBytes(vec),
		"l1_key", l1Key,
		"created_at", time.Now().Unix(),
	)
	pipe.Expire(ctx, hashKey, l2TTL)
	_, err = pipe.Exec(ctx)
	return err
}

// ensureIndex creates the HNSW index for tenantID if it doesn't exist yet.
func (c *ValkeyL2) ensureIndex(ctx context.Context, tenantID string) error {
	c.mu.Lock()
	if _, seen := c.indexSeen[tenantID]; seen {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	indexName := l2IndexName(tenantID)
	prefix := l2EntryPrefix(tenantID)

	err := c.rdb.FTCreate(ctx, indexName, &redis.FTCreateOptions{
		OnHash: true,
		Prefix: []any{prefix},
	}, &redis.FieldSchema{
		FieldName: "embedding",
		FieldType: redis.SearchFieldTypeVector,
		VectorArgs: &redis.FTVectorArgs{
			HNSWOptions: &redis.FTHNSWOptions{
				Type:           "FLOAT32",
				Dim:            c.embedder.Dim(),
				DistanceMetric: "COSINE",
			},
		},
	}, &redis.FieldSchema{
		FieldName: "l1_key",
		FieldType: redis.SearchFieldTypeTag,
	}, &redis.FieldSchema{
		FieldName: "created_at",
		FieldType: redis.SearchFieldTypeNumeric,
	}).Err()

	if err != nil && !isIndexExistsErr(err) {
		return fmt.Errorf("ft.create %s: %w", indexName, err)
	}

	c.mu.Lock()
	c.indexSeen[tenantID] = struct{}{}
	c.mu.Unlock()
	return nil
}

func l2IndexName(tenantID string) string {
	return "llm:l2:" + tenantID
}

func l2EntryPrefix(tenantID string) string {
	return "llm:l2:" + tenantID + ":"
}

func l2EntryKey(tenantID, l1Key string) string {
	// Stable entry key: prefix + l1Key suffix (already a SHA-256 hex, so short and unique)
	if len(l1Key) > 16 {
		return l2EntryPrefix(tenantID) + l1Key[len(l1Key)-16:]
	}
	return l2EntryPrefix(tenantID) + l1Key
}

// float32SliceToBytes serialises a float32 slice to little-endian bytes
// as required by Valkey-search vector fields.
func float32SliceToBytes(v []float32) []byte {
	b := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

func isIndexMissingErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, redis.Nil) ||
		contains(err.Error(), "no such index") ||
		contains(err.Error(), "Unknown Index name")
}

func isIndexExistsErr(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "Index already exists")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- MemL2 (in-memory test fake) --------------------------------------------

// MemL2 is a thread-safe in-memory L2 cache that computes exact cosine
// similarity instead of using HNSW. Used only in unit tests.
type MemL2 struct {
	mu       sync.RWMutex
	entries  map[string][]memL2Entry // tenantID → entries
	embedder Embedder
}

type memL2Entry struct {
	vec       []float32
	l1Key     string
	expiresAt time.Time
}

// NewMemL2 creates a fresh in-memory L2 cache.
func NewMemL2(embedder Embedder) *MemL2 {
	return &MemL2{
		entries:  make(map[string][]memL2Entry),
		embedder: embedder,
	}
}

// Lookup embeds query and finds the most similar stored vector above threshold.
func (m *MemL2) Lookup(ctx context.Context, tenantID, queryText string) (string, error) {
	vec, err := m.embedder.Embed(ctx, queryText)
	if err != nil {
		return "", fmt.Errorf("mem l2 embed: %w", err)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	var bestKey string
	bestSim := float64(-1)

	for _, e := range m.entries[tenantID] {
		if now.After(e.expiresAt) {
			continue
		}
		sim := cosineSimilarity(vec, e.vec)
		if sim > bestSim {
			bestSim = sim
			bestKey = e.l1Key
		}
	}

	if bestSim < l2Threshold {
		return "", nil
	}
	return bestKey, nil
}

// Store adds an (embedding → l1Key) entry for the tenant.
func (m *MemL2) Store(ctx context.Context, tenantID, queryText, l1Key string) error {
	vec, err := m.embedder.Embed(ctx, queryText)
	if err != nil {
		return fmt.Errorf("mem l2 embed: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.entries[tenantID] = append(m.entries[tenantID], memL2Entry{
		vec:       vec,
		l1Key:     l1Key,
		expiresAt: time.Now().Add(l2TTL),
	})
	return nil
}

// cosineSimilarity returns the cosine similarity in [−1, 1].
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		fa, fb := float64(a[i]), float64(b[i])
		dot += fa * fb
		normA += fa * fa
		normB += fb * fb
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// --- MemEmbedder (test helper) -----------------------------------------------

// MemEmbedder returns deterministic embeddings for test input.
// The same text always returns the same vector; different texts return orthogonal vectors.
type MemEmbedder struct {
	mu   sync.Mutex
	seen map[string][]float32
	dim  int
	next int // counter for creating new orthogonal basis vectors
}

// NewMemEmbedder creates a test embedder with the given dimension.
func NewMemEmbedder(dim int) *MemEmbedder {
	return &MemEmbedder{
		seen: make(map[string][]float32),
		dim:  dim,
	}
}

// Embed returns a deterministic, unit-length vector for text.
// Same text → same vector; different text → orthogonal vector (if dim allows).
func (e *MemEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if v, ok := e.seen[text]; ok {
		return v, nil
	}
	// Create a basis vector with a single 1.0 at position next%dim.
	v := make([]float32, e.dim)
	v[e.next%e.dim] = 1.0
	e.next++
	e.seen[text] = v
	return v, nil
}

// SimilarTo registers text2 to return the same vector as text1 (simulating semantic similarity).
func (e *MemEmbedder) SimilarTo(text1, text2 string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if v, ok := e.seen[text1]; ok {
		e.seen[text2] = v
	}
}

func (e *MemEmbedder) Dim() int { return e.dim }
