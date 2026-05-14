// Package zep provides a REST client for the Zep episodic memory service.
// Zep stores incident episodes as bi-temporal session messages and supports
// semantic search with MMR (Maximal Marginal Relevance) ranking.
//
// See docs/plans/05.memory-stage5.md §3 for the specification.
package zep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// Client is a Zep REST API client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Zep client.
// baseURL should be the Zep instance URL (e.g. "https://zep.internal/api/v2").
// apiKey is the Zep API key for authentication.
func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{baseURL: baseURL, apiKey: apiKey, httpClient: httpClient}
}

// ─── Episode types ────────────────────────────────────────────────────────────

// Episode is a single incident memory entry to store in Zep.
type Episode struct {
	ID         string         // UUID for idempotent writes
	TenantID   string         // owner tenant
	IncidentID string         // source incident
	Content    string         // summary text for semantic search
	ValidAt    time.Time      // bi-temporal: when the incident occurred (event time)
	RecordedAt time.Time      // bi-temporal: when we recorded it (system time)
	Domain     string         // "infra" | "traffic" | "unknown" — drives decay lambda
	Metadata   map[string]any // any extra fields
}

// SearchResult is one result from a Zep semantic search.
type SearchResult struct {
	UUID       string         `json:"uuid"`
	Content    string         `json:"content"`
	CreatedAt  time.Time      `json:"created_at"`
	Metadata   map[string]any `json:"metadata"`
	Score      float32        // raw relevance score from Zep
	DecayScore float32        // score after time-decay applied
}

// ─── Wire types (Zep API v2) ──────────────────────────────────────────────────

type zepAddMessagesReq struct {
	Messages []zepMessage `json:"messages"`
}

type zepMessage struct {
	UUID      string         `json:"uuid"`
	Content   string         `json:"content"`
	CreatedAt time.Time      `json:"created_at"`
	Metadata  map[string]any `json:"metadata"`
	RoleType  string         `json:"role_type"`
}

type zepSearchReq struct {
	Text       string `json:"text"`
	SearchType string `json:"search_type"`
	Limit      int    `json:"limit"`
}

type zepSearchResp struct {
	Results []zepSearchResult `json:"results"`
}

type zepSearchResult struct {
	Message zepMessage `json:"message"`
	Score   float32    `json:"score"`
}

// ─── Session ID convention ────────────────────────────────────────────────────

// SessionID returns the Zep session ID for a tenant's episodic memory.
// Convention: "paladin-{tenantID}" (one session per tenant).
func SessionID(tenantID string) string {
	return "paladin-" + tenantID
}

// ─── CRUD operations ─────────────────────────────────────────────────────────

// AddEpisode writes one incident episode to Zep as a session message.
func (c *Client) AddEpisode(ctx context.Context, ep Episode) error {
	if ep.TenantID == "" || ep.ID == "" {
		return fmt.Errorf("zep: AddEpisode: tenant_id and id are required")
	}
	meta := map[string]any{
		"recorded_at": ep.RecordedAt.Format(time.RFC3339),
		"tenant_id":   ep.TenantID,
		"incident_id": ep.IncidentID,
		"domain":      ep.Domain,
	}
	for k, v := range ep.Metadata {
		meta[k] = v
	}
	body := zepAddMessagesReq{
		Messages: []zepMessage{{
			UUID:      ep.ID,
			Content:   ep.Content,
			CreatedAt: ep.ValidAt,
			Metadata:  meta,
			RoleType:  "system",
		}},
	}
	url := fmt.Sprintf("%s/sessions/%s/messages", c.baseURL, SessionID(ep.TenantID))
	return c.post(ctx, url, body)
}

// SearchEpisodes performs a semantic search over a tenant's episodic memory.
// Results are sorted by Zep's score and then annotated with time-decay scores.
func (c *Client) SearchEpisodes(ctx context.Context, tenantID, query string, topK int) ([]SearchResult, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("zep: SearchEpisodes: tenant_id is required")
	}
	url := fmt.Sprintf("%s/sessions/%s/search", c.baseURL, SessionID(tenantID))
	req := zepSearchReq{Text: query, SearchType: "mmr", Limit: topK}

	var resp zepSearchResp
	if err := c.postDecode(ctx, url, req, &resp); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(resp.Results))
	for _, r := range resp.Results {
		domain := ""
		if d, ok := r.Message.Metadata["domain"].(string); ok {
			domain = d
		}
		decay := ApplyDecay(r.Score, r.Message.CreatedAt, domain)
		results = append(results, SearchResult{
			UUID:       r.Message.UUID,
			Content:    r.Message.Content,
			CreatedAt:  r.Message.CreatedAt,
			Metadata:   r.Message.Metadata,
			Score:      r.Score,
			DecayScore: decay,
		})
	}
	return results, nil
}

// DeleteSession deletes all episodes for a tenant (GDPR erasure).
func (c *Client) DeleteSession(ctx context.Context, tenantID string) error {
	if tenantID == "" {
		return fmt.Errorf("zep: DeleteSession: tenant_id is required")
	}
	url := fmt.Sprintf("%s/sessions/%s", c.baseURL, SessionID(tenantID))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("zep: build DELETE request: %w", err)
	}
	c.setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("zep: DELETE session: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return fmt.Errorf("zep: DELETE session returned %d: %s", resp.StatusCode, body)
	}
	return nil
}

// ─── Time-decay scoring ───────────────────────────────────────────────────────

// decayLambda maps domain to exponential decay rate λ.
// Higher λ = faster decay.
var decayLambda = map[string]float64{
	"infra":   0.05, // slow decay — runbooks stay relevant for weeks
	"traffic": 0.15, // moderate — traffic patterns shift in days
}

// defaultDecayLambda is used for unknown or empty domain.
const defaultDecayLambda = 0.30

const (
	maxErrorBodyBytes = 64 * 1024
	maxJSONBodyBytes  = 4 << 20
)

// ApplyDecay computes time-decayed relevance score.
// relevance(t) = rawScore × exp(-λ × t_days)
// where t_days = (now - validAt).Hours() / 24
func ApplyDecay(rawScore float32, validAt time.Time, domain string) float32 {
	λ, ok := decayLambda[domain]
	if !ok {
		λ = defaultDecayLambda
	}
	tDays := time.Since(validAt).Hours() / 24
	if tDays < 0 {
		tDays = 0
	}
	return rawScore * float32(math.Exp(-λ*tDays))
}

// ─── HTTP helpers ─────────────────────────────────────────────────────────────

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Api-Key "+c.apiKey)
	}
}

func (c *Client) post(ctx context.Context, url string, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("zep: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("zep: build request: %w", err)
	}
	c.setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("zep: POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return fmt.Errorf("zep: POST %s returned %d: %s", url, resp.StatusCode, body)
	}
	return nil
}

func (c *Client) postDecode(ctx context.Context, url string, reqBody, out any) error {
	b, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("zep: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("zep: build request: %w", err)
	}
	c.setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("zep: POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return fmt.Errorf("zep: POST %s returned %d: %s", url, resp.StatusCode, body)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxJSONBodyBytes+1)).Decode(out); err != nil {
		return fmt.Errorf("zep: decode response from %s: %w", url, err)
	}
	return nil
}
