// Package anthropic provides a minimal Anthropic Messages API client that
// unconditionally injects prompt cache breakpoints on every request.
//
// Unlike the OpenRouter/Eino path (OpenAI-compatible), this client targets
// Anthropic's native API (api.anthropic.com/v1/messages). Using the native
// API is required for prompt caching — the OpenAI-compatible endpoint does
// not expose cache_control parameters.
//
// Cache injection strategy (per Stage 9 spec):
//
//   Breakpoint 1: end of system prompt       (BuildSystemBlocks)
//   Breakpoint 2: last tool in catalog       (AppendToolCacheControl)
//   Breakpoint 3: last user message content  (InjectMessageCacheBreakpoints)
//
// All three breakpoints are set on every request regardless of model tier.
// Unused breakpoints (e.g. no tools) are silently skipped by the helpers.
package anthropic

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

	"github.com/paladinai/paladinai/internal/cache"
)

const (
	defaultBaseURL    = "https://api.anthropic.com"
	anthropicVersion  = "2023-06-01"
	betaPromptCaching = "prompt-caching-2024-07-31"
	defaultTimeout    = 60 * time.Second
)

// Config holds credentials and model selection for the Anthropic client.
type Config struct {
	// APIKey is the Anthropic API key (sk-ant-…). Required.
	APIKey string
	// Model is the Anthropic model ID (e.g. "claude-3-5-sonnet-20241022"). Required.
	Model string
	// BaseURL overrides the default endpoint. Useful for testing with a stub server.
	BaseURL string
	// MaxTokens caps the response length. Defaults to 1024.
	MaxTokens int
	// Timeout overrides the per-request HTTP timeout.
	Timeout time.Duration
}

// Request is a structured Anthropic Messages API request with prompt cache breakpoints.
// The System and Tools fields are populated automatically by the client using the
// cache injection helpers.
type Request struct {
	// SystemPrompt is the plain-text system prompt. The client wraps it with
	// BuildSystemBlocks, adding a cache_control breakpoint at the end.
	SystemPrompt string
	// Tools is the tool catalog to include. The client applies AppendToolCacheControl,
	// adding a cache_control breakpoint on the last tool. May be nil.
	Tools []cache.ToolEntry
	// Messages is the conversation history. The client applies InjectMessageCacheBreakpoints,
	// adding a cache_control breakpoint on the last user message.
	Messages []cache.ChatMessage
}

// Response wraps the Anthropic Messages API response.
type Response struct {
	// ID is the Anthropic request ID.
	ID string `json:"id"`
	// Content holds the model's output blocks.
	Content []ResponseBlock `json:"content"`
	// Usage reports token consumption including prompt cache statistics.
	Usage Usage `json:"usage"`
	// StopReason indicates why the model stopped ("end_turn", "max_tokens", etc.).
	StopReason string `json:"stop_reason"`
}

// ResponseBlock is one content block in the response.
type ResponseBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Usage holds token counts from the Anthropic API response.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// TextContent returns the concatenated text from all text blocks in the response.
func (r *Response) TextContent() string {
	var sb strings.Builder
	for _, b := range r.Content {
		if b.Type == "text" {
			sb.WriteString(b.Text)
		}
	}
	return sb.String()
}

// CacheHit returns true if the response came (wholly or partly) from Anthropic's
// prompt cache — i.e. CacheReadInputTokens > 0.
func (r *Response) CacheHit() bool { return r.Usage.CacheReadInputTokens > 0 }

// Model returns the configured Anthropic model ID.
func (c *Client) Model() string { return c.cfg.Model }

// Client sends requests to the Anthropic Messages API with automatic prompt
// cache injection on every request.
type Client struct {
	cfg    Config
	http   *http.Client
	log    *zap.Logger
}

// New creates a Client. Returns an error if APIKey or Model is empty.
func New(cfg Config, log *zap.Logger) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("anthropic: APIKey is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("anthropic: Model is required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 1024
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: timeout},
		log:  log,
	}, nil
}

// Complete sends a Messages API request with all three prompt cache breakpoints
// injected automatically. The caller provides the plain-text system prompt,
// optional tool catalog, and conversation messages. The client builds the
// structured Anthropic request body, adds the cache_control markers, and
// parses the response.
func (c *Client) Complete(ctx context.Context, req Request) (*Response, error) {
	body, err := c.buildBody(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: build request body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	httpReq.Header.Set("anthropic-beta", betaPromptCaching)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: HTTP request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic: API error %d: %s", resp.StatusCode, trimLog(string(respBody), 400))
	}

	var result Response
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("anthropic: unmarshal response: %w", err)
	}

	c.log.Debug("anthropic: request complete",
		zap.String("id", result.ID),
		zap.Int("input_tokens", result.Usage.InputTokens),
		zap.Int("output_tokens", result.Usage.OutputTokens),
		zap.Int("cache_read_tokens", result.Usage.CacheReadInputTokens),
		zap.Int("cache_creation_tokens", result.Usage.CacheCreationInputTokens),
		zap.Bool("cache_hit", result.CacheHit()),
	)

	return &result, nil
}

// wireFormat is the JSON-serialisable form of an Anthropic Messages API request.
// It mirrors the Anthropic API spec, not the cache.* types.
type wireFormat struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    []cache.ContentBlock `json:"system"`
	Tools     []cache.ToolEntry  `json:"tools,omitempty"`
	Messages  []cache.ChatMessage `json:"messages"`
}

func (c *Client) buildBody(req Request) ([]byte, error) {
	wire := wireFormat{
		Model:     c.cfg.Model,
		MaxTokens: c.cfg.MaxTokens,
		System:    cache.BuildSystemBlocks(req.SystemPrompt),
		Tools:     cache.AppendToolCacheControl(req.Tools),
		Messages:  cache.InjectMessageCacheBreakpoints(req.Messages),
	}
	return json.Marshal(wire)
}

func trimLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
