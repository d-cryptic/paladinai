// Package qdrant provides a minimal HTTP client for the Qdrant vector database
// along with chunking, embedding, and indexing helpers used by the Paladin RAG
// pipeline (Stage 6).
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

	"go.uber.org/zap"
)

const (
	// RunbookCollection is the default Qdrant collection name for runbook chunks.
	RunbookCollection = "paladin_runbooks"
	// SemanticCollection is the Qdrant collection name for semantic fact chunks.
	SemanticCollection = "paladin_semantic_facts"
	defaultTimeout     = 10 * time.Second
)

// Point represents a Qdrant vector point.
type Point struct {
	ID      string         `json:"id"`
	Vector  []float32      `json:"vector"`
	Payload map[string]any `json:"payload"`
}

// SearchResult is one result from a similarity search.
type SearchResult struct {
	ID      string
	Score   float32
	Payload map[string]any
}

// Client is a minimal Qdrant HTTP client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	log        *zap.Logger
}

// NewClient is an alias for New for callers that prefer the longer form.
func NewClient(baseURL, apiKey string, log *zap.Logger) *Client {
	return New(baseURL, apiKey, log)
}

// New creates a Qdrant client. baseURL should not contain a trailing slash.
func New(baseURL, apiKey string, log *zap.Logger) *Client {
	if log == nil {
		log = zap.NewNop()
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: defaultTimeout},
		log:        log,
	}
}

// do executes an HTTP request with optional JSON body and decodes the response
// into out (if non-nil).
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("qdrant: marshal body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("qdrant: new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("api-key", c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant: do %s %s: %w", method, path, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return fmt.Errorf("qdrant: %s %s: status %d: %s", method, path, resp.StatusCode, string(b))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("qdrant: decode response: %w", err)
		}
	}
	return nil
}

// EnsureCollection creates the collection if it doesn't already exist.
// vectorSize is the dense embedding dimension (1024 for BGE-M3).
func (c *Client) EnsureCollection(ctx context.Context, name string, vectorSize int) error {
	body := map[string]any{
		"vectors": map[string]any{
			"size":     vectorSize,
			"distance": "Cosine",
		},
	}
	if err := c.do(ctx, http.MethodPut, "/collections/"+name, body, nil); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return err
	}
	return nil
}

// Upsert inserts or updates points in a collection.
func (c *Client) Upsert(ctx context.Context, collection string, points []Point) error {
	if len(points) == 0 {
		return nil
	}
	body := map[string]any{"points": points}
	return c.do(ctx, http.MethodPut, "/collections/"+collection+"/points", body, nil)
}

// Search performs a vector similarity search and returns up to topK results
// ordered by score (highest first). filter, when non-nil, is included as the
// Qdrant filter clause (e.g. a must-match on tenant_id).
func (c *Client) Search(ctx context.Context, collection string, vector []float32, topK int, filter map[string]any) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 10
	}
	body := map[string]any{
		"vector":       vector,
		"limit":        topK,
		"with_payload": true,
	}
	if filter != nil {
		body["filter"] = filter
	}
	var resp struct {
		Result []struct {
			ID      any            `json:"id"`
			Score   float32        `json:"score"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	if err := c.do(ctx, http.MethodPost, "/collections/"+collection+"/points/search", body, &resp); err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(resp.Result))
	for _, r := range resp.Result {
		out = append(out, SearchResult{
			ID:      fmt.Sprintf("%v", r.ID),
			Score:   r.Score,
			Payload: r.Payload,
		})
	}
	return out, nil
}

// Delete removes points by ID from a collection.
func (c *Client) Delete(ctx context.Context, collection string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	body := map[string]any{"points": ids}
	return c.do(ctx, http.MethodPost, "/collections/"+collection+"/points/delete", body, nil)
}
