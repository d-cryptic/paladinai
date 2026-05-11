package qdrant

import (
	"context"
	"math/rand"
)

// EmbeddingDim is the dense vector size used by BGE-M3.
const EmbeddingDim = 1024

// Embedder computes dense embeddings for text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// StubEmbedder produces deterministic pseudo-random embeddings for testing.
// Replace with a real Triton/OpenAI embedder in production.
type StubEmbedder struct{}

// Embed returns a deterministic pseudo-random vector seeded by the text hash.
func (s *StubEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	h := 0
	for _, b := range []byte(text) {
		h = h*31 + int(b)
	}
	r := rand.New(rand.NewSource(int64(h))) //nolint:gosec // deterministic stub, not crypto
	v := make([]float32, EmbeddingDim)
	for i := range v {
		v[i] = r.Float32()*2 - 1
	}
	return v, nil
}
