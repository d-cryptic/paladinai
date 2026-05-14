package qdrant_test

import (
	"context"
	"math"
	"testing"

	"github.com/paladinai/paladinai/internal/qdrant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStubEmbedder_Dim(t *testing.T) {
	e := &qdrant.StubEmbedder{}
	v, err := e.Embed(context.Background(), "hello world")
	require.NoError(t, err)
	assert.Len(t, v, qdrant.EmbeddingDim)
}

func TestStubEmbedder_Deterministic(t *testing.T) {
	e := &qdrant.StubEmbedder{}
	ctx := context.Background()
	v1, err := e.Embed(ctx, "high CPU usage")
	require.NoError(t, err)
	v2, err := e.Embed(ctx, "high CPU usage")
	require.NoError(t, err)
	assert.Equal(t, v1, v2, "same text must produce same embedding")
}

func TestStubEmbedder_DifferentInputs(t *testing.T) {
	e := &qdrant.StubEmbedder{}
	ctx := context.Background()
	v1, err := e.Embed(ctx, "postgres down")
	require.NoError(t, err)
	v2, err := e.Embed(ctx, "redis timeout")
	require.NoError(t, err)
	// Different inputs should produce different vectors (not guaranteed but extremely likely).
	equal := true
	for i := range v1 {
		if v1[i] != v2[i] {
			equal = false
			break
		}
	}
	assert.False(t, equal, "different inputs should produce different embeddings")
}

func TestStubEmbedder_VectorNormRange(t *testing.T) {
	e := &qdrant.StubEmbedder{}
	v, err := e.Embed(context.Background(), "test alert")
	require.NoError(t, err)

	// Each element should be in [-1, 1].
	for i, x := range v {
		assert.True(t, x >= -1 && x <= 1, "element %d=%v out of range [-1, 1]", i, x)
	}
}

func TestStubEmbedder_EmptyString(t *testing.T) {
	e := &qdrant.StubEmbedder{}
	v, err := e.Embed(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, v, qdrant.EmbeddingDim)
}

func TestStubEmbedder_L2NormNonZero(t *testing.T) {
	e := &qdrant.StubEmbedder{}
	v, err := e.Embed(context.Background(), "non-zero test")
	require.NoError(t, err)

	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	assert.True(t, math.Sqrt(norm) > 0, "embedding vector must not be all-zero")
}

func TestIsMockGatewayURL(t *testing.T) {
	cases := map[string]bool{
		"http://mock-llm:1080/api/v1":         true,
		"http://paladin-mock-llm:1080/api/v1": true,
		"http://localhost:1080/api/v1":        true,
		"http://127.0.0.1:1080/api/v1":        true,
		"http://localhost:8080/api/v1":        false,
		"https://openrouter.ai/api/v1":        false,
		"https://mock-llm.example.com/api/v1": false,
		"://bad-url":                          false,
		"":                                    false,
	}

	for rawURL, want := range cases {
		assert.Equal(t, want, qdrant.IsMockGatewayURL(rawURL), rawURL)
	}
}
