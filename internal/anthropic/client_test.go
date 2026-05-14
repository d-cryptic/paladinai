package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/cache"
)

// stubServer returns an httptest.Server that replies with a fixed Anthropic-style
// response. The handler captures the last request body for assertions.
func stubServer(t *testing.T, statusCode int, body string) (*httptest.Server, *[]byte) {
	t.Helper()
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		captured = b
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		//nolint:errcheck
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	c, err := New(Config{
		APIKey:  "test-key",
		Model:   "claude-test",
		BaseURL: baseURL,
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

const okResponse = `{
  "id": "msg_01",
  "content": [{"type": "text", "text": "hello world"}],
  "usage": {
    "input_tokens": 100,
    "output_tokens": 10,
    "cache_creation_input_tokens": 90,
    "cache_read_input_tokens": 0
  },
  "stop_reason": "end_turn"
}`

const cacheHitResponse = `{
  "id": "msg_02",
  "content": [{"type": "text", "text": "cached response"}],
  "usage": {
    "input_tokens": 100,
    "output_tokens": 10,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 90
  },
  "stop_reason": "end_turn"
}`

func TestNew_RequiresAPIKey(t *testing.T) {
	_, err := New(Config{Model: "claude-test"}, zap.NewNop())
	if err == nil || !strings.Contains(err.Error(), "APIKey") {
		t.Errorf("expected APIKey error, got %v", err)
	}
}

func TestNew_RequiresModel(t *testing.T) {
	_, err := New(Config{APIKey: "key"}, zap.NewNop())
	if err == nil || !strings.Contains(err.Error(), "Model") {
		t.Errorf("expected Model error, got %v", err)
	}
}

func TestNew_DefaultsApplied(t *testing.T) {
	c, err := New(Config{APIKey: "k", Model: "m"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.cfg.BaseURL != defaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", c.cfg.BaseURL, defaultBaseURL)
	}
	if c.cfg.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %d, want 1024", c.cfg.MaxTokens)
	}
}

func TestComplete_Success(t *testing.T) {
	srv, captured := stubServer(t, http.StatusOK, okResponse)
	c := newTestClient(t, srv.URL)

	resp, err := c.Complete(context.Background(), Request{
		SystemPrompt: "You are a triage agent.",
		Messages:     cache.PlainChatMessages([][2]string{{"user", "analyse this alert"}}),
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.TextContent() != "hello world" {
		t.Errorf("TextContent = %q, want %q", resp.TextContent(), "hello world")
	}
	if resp.CacheHit() {
		t.Error("expected no cache hit on first request")
	}

	// Verify the request body contains cache_control on the system prompt.
	var wire wireFormat
	if err := json.Unmarshal(*captured, &wire); err != nil {
		t.Fatalf("unmarshal captured body: %v", err)
	}
	if len(wire.System) == 0 {
		t.Fatal("system blocks missing from request")
	}
	if wire.System[0].CacheControl == nil {
		t.Error("system prompt should have cache_control breakpoint")
	}
	if wire.System[0].CacheControl.Type != cache.CacheTypeEphemeral {
		t.Errorf("cache_control.type = %q, want %q", wire.System[0].CacheControl.Type, cache.CacheTypeEphemeral)
	}
}

func TestComplete_MessageCacheBreakpoint(t *testing.T) {
	srv, captured := stubServer(t, http.StatusOK, okResponse)
	c := newTestClient(t, srv.URL)

	msgs := cache.PlainChatMessages([][2]string{
		{"user", "first message"},
		{"assistant", "ok"},
		{"user", "analyse alert X"},
	})
	_, err := c.Complete(context.Background(), Request{SystemPrompt: "sys", Messages: msgs})
	if err != nil {
		t.Fatal(err)
	}

	var wire wireFormat
	if err := json.Unmarshal(*captured, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Last user message (index 2) should have cache_control.
	if len(wire.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(wire.Messages))
	}
	lastUser := wire.Messages[2]
	if lastUser.Content[0].CacheControl == nil {
		t.Error("last user message should have cache_control breakpoint")
	}
	// First user message should NOT have cache_control.
	if wire.Messages[0].Content[0].CacheControl != nil {
		t.Error("first user message must not have cache_control")
	}
}

func TestComplete_ToolCacheBreakpoint(t *testing.T) {
	srv, captured := stubServer(t, http.StatusOK, okResponse)
	c := newTestClient(t, srv.URL)

	tools := []cache.ToolEntry{
		{Name: "query_metrics", Description: "query metrics"},
		{Name: "get_logs", Description: "get logs"},
	}
	_, err := c.Complete(context.Background(), Request{
		SystemPrompt: "sys",
		Tools:        tools,
		Messages:     cache.PlainChatMessages([][2]string{{"user", "go"}}),
	})
	if err != nil {
		t.Fatal(err)
	}

	var wire wireFormat
	if err := json.Unmarshal(*captured, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(wire.Tools))
	}
	if wire.Tools[1].CacheControl == nil {
		t.Error("last tool should have cache_control breakpoint")
	}
	if wire.Tools[0].CacheControl != nil {
		t.Error("first tool must not have cache_control")
	}
}

func TestComplete_NoTools_BodyOmitsToolsField(t *testing.T) {
	srv, captured := stubServer(t, http.StatusOK, okResponse)
	c := newTestClient(t, srv.URL)

	_, err := c.Complete(context.Background(), Request{
		SystemPrompt: "sys",
		Messages:     cache.PlainChatMessages([][2]string{{"user", "hi"}}),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify `tools` key is absent when nil (omitempty).
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(*captured, &raw); err != nil {
		t.Fatal(err)
	}
	if _, exists := raw["tools"]; exists {
		t.Error("tools field should be omitted when nil")
	}
}

func TestComplete_CacheHit(t *testing.T) {
	srv, _ := stubServer(t, http.StatusOK, cacheHitResponse)
	c := newTestClient(t, srv.URL)

	resp, err := c.Complete(context.Background(), Request{
		SystemPrompt: "sys",
		Messages:     cache.PlainChatMessages([][2]string{{"user", "repeat"}}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.CacheHit() {
		t.Error("expected cache hit when CacheReadInputTokens > 0")
	}
	if resp.TextContent() != "cached response" {
		t.Errorf("TextContent = %q, want %q", resp.TextContent(), "cached response")
	}
}

func TestComplete_APIError(t *testing.T) {
	srv, _ := stubServer(t, http.StatusUnauthorized, `{"error":"invalid api key"}`)
	c := newTestClient(t, srv.URL)

	_, err := c.Complete(context.Background(), Request{
		SystemPrompt: "sys",
		Messages:     cache.PlainChatMessages([][2]string{{"user", "hi"}}),
	})
	if err == nil {
		t.Fatal("expected error on non-200 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %q, want 401 mention", err.Error())
	}
}

func TestComplete_ResponseTooLarge(t *testing.T) {
	srv, _ := stubServer(t, http.StatusOK, `{"id":"msg","content":[{"type":"text","text":"`+strings.Repeat("x", maxResponseBytes)+`"}]}`)
	c := newTestClient(t, srv.URL)

	_, err := c.Complete(context.Background(), Request{
		SystemPrompt: "sys",
		Messages:     cache.PlainChatMessages([][2]string{{"user", "hi"}}),
	})
	if err == nil {
		t.Fatal("expected oversized response error")
	}
	if !strings.Contains(err.Error(), "response body exceeds") {
		t.Errorf("error = %q, want response size mention", err.Error())
	}
}

func TestResponse_TextContent_MultipleBlocks(t *testing.T) {
	r := Response{
		Content: []ResponseBlock{
			{Type: "text", Text: "hello "},
			{Type: "thinking", Text: "internal"},
			{Type: "text", Text: "world"},
		},
	}
	got := r.TextContent()
	if got != "hello world" {
		t.Errorf("TextContent = %q, want %q", got, "hello world")
	}
}

func TestComplete_SetsRequiredHeaders(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck
		w.Write([]byte(okResponse))
	}))
	t.Cleanup(srv.Close)
	c := newTestClient(t, srv.URL)

	_, err := c.Complete(context.Background(), Request{
		SystemPrompt: "sys",
		Messages:     cache.PlainChatMessages([][2]string{{"user", "hi"}}),
	})
	if err != nil {
		t.Fatal(err)
	}

	checks := map[string]string{
		"anthropic-version": anthropicVersion,
		"anthropic-beta":    betaPromptCaching,
		"x-api-key":         "test-key",
	}
	for h, want := range checks {
		if got := gotHeaders.Get(h); got != want {
			t.Errorf("header %q = %q, want %q", h, got, want)
		}
	}
}
