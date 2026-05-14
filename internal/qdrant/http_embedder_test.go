package qdrant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPEmbedder_Embed_Success(t *testing.T) {
	wantVec := []float32{0.1, 0.2, 0.3, 0.4}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/embeddings", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		resp := map[string]any{
			"data": []map[string]any{
				{"embedding": wantVec},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer srv.Close()

	emb := NewHTTPEmbedder(srv.URL, "test-key", "test-model", 4)
	got, err := emb.Embed(context.Background(), "hello world")
	require.NoError(t, err)
	require.Len(t, got, 4)
	assert.InDelta(t, 0.1, got[0], 0.001)
	assert.Equal(t, 4, emb.Dim())
}

func TestHTTPEmbedder_Embed_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "model overloaded", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	emb := NewHTTPEmbedder(srv.URL, "", "test-model", 4)
	_, err := emb.Embed(context.Background(), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestHTTPEmbedder_Embed_ServerErrorTruncatesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(strings.Repeat("x", maxErrorBodyBytes+1024)))
	}))
	defer srv.Close()

	emb := NewHTTPEmbedder(srv.URL, "", "test-model", 4)
	_, err := emb.Embed(context.Background(), "test")
	require.Error(t, err)
	if strings.Count(err.Error(), "x") > maxErrorBodyBytes {
		t.Fatalf("error body was not capped: %d x chars", strings.Count(err.Error(), "x"))
	}
}

func TestHTTPEmbedder_Embed_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{"data": []any{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer srv.Close()

	emb := NewHTTPEmbedder(srv.URL, "", "m", 4)
	_, err := emb.Embed(context.Background(), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty embedding")
}

func TestHTTPEmbedder_Embed_DimensionMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"embedding": []float32{0.1, 0.2}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer srv.Close()

	emb := NewHTTPEmbedder(srv.URL, "", "m", 4)
	_, err := emb.Embed(context.Background(), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dimension mismatch")
	assert.Contains(t, err.Error(), "expected 4, got 2")
}
