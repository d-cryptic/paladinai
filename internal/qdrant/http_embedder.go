package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxEmbedderResponseBytes = 4 << 20

// HTTPEmbedder calls an OpenAI-compatible embeddings endpoint (e.g. Triton,
// Ollama, or OpenRouter) to produce dense embeddings.
type HTTPEmbedder struct {
	baseURL    string
	apiKey     string
	model      string
	dim        int
	httpClient *http.Client
}

// NewHTTPEmbedder constructs an HTTPEmbedder.
//   - baseURL: base of the embeddings API (e.g. "https://openrouter.ai/api/v1")
//   - apiKey: bearer token (empty = no auth header)
//   - model: model name (e.g. "BAAI/bge-m3")
//   - dim: expected output dimension (0 = auto-detect from first response)
func NewHTTPEmbedder(baseURL, apiKey, model string, dim int) *HTTPEmbedder {
	return &HTTPEmbedder{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		dim:        dim,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Embed calls the embeddings endpoint and returns the first embedding vector.
func (e *HTTPEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]any{
		"input": text,
		"model": e.model,
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("embedder: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("embedder: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedder: do: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, fmt.Errorf("embedder: status %d: %s", resp.StatusCode, string(b))
	}

	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := decodeLimitedJSON(resp.Body, maxEmbedderResponseBytes, &out); err != nil {
		return nil, fmt.Errorf("embedder: decode: %w", err)
	}
	if len(out.Data) == 0 || len(out.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embedder: empty embedding in response")
	}
	if e.dim > 0 && len(out.Data[0].Embedding) != e.dim {
		return nil, fmt.Errorf("embedder: dimension mismatch: expected %d, got %d", e.dim, len(out.Data[0].Embedding))
	}
	return out.Data[0].Embedding, nil
}

// Dim returns the configured embedding dimension.
func (e *HTTPEmbedder) Dim() int { return e.dim }
