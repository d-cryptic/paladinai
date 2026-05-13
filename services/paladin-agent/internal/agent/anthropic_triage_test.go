package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/alert"
	anthropicpkg "github.com/paladinai/paladinai/internal/anthropic"
	"github.com/paladinai/paladinai/internal/qdrant"
)

func makeAnthropicStub(t *testing.T, statusCode int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		//nolint:errcheck
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func makeAnthropicTriager(t *testing.T, baseURL string) *AnthropicTriager {
	t.Helper()
	c, err := anthropicpkg.New(anthropicpkg.Config{
		APIKey:  "test-key",
		Model:   "claude-test",
		BaseURL: baseURL,
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("anthropicpkg.New: %v", err)
	}
	return NewAnthropicTriager(c, zap.NewNop())
}

func makeTriageResponse(severity, summary, cause, action string, needsHuman bool) string {
	b, _ := json.Marshal(map[string]any{
		"confirmed_severity":  severity,
		"summary":             summary,
		"likely_cause":        cause,
		"affected_services":   []string{"api-server"},
		"recommended_action":  action,
		"needs_human":         needsHuman,
	})
	return anthropicResponseBody(string(b), false)
}

func anthropicResponseBody(text string, cacheHit bool) string {
	cacheRead := 0
	if cacheHit {
		cacheRead = 500
	}
	b, _ := json.Marshal(map[string]any{
		"id": "msg_test",
		"content": []map[string]string{
			{"type": "text", "text": text},
		},
		"usage": map[string]int{
			"input_tokens":                100,
			"output_tokens":               50,
			"cache_creation_input_tokens": 0,
			"cache_read_input_tokens":     cacheRead,
		},
		"stop_reason": "end_turn",
	})
	return string(b)
}

func testEnvelope() *alert.AlertEnvelope {
	return &alert.AlertEnvelope{
		TenantID:    "tenant-1",
		Fingerprint: "fp-abc123",
		Title:       "High CPU usage on api-server",
		Severity:    alert.SeverityP2,
		Status:      alert.StatusFiring,
		Labels:      map[string]string{"service": "api-server", "namespace": "prod"},
		Annotations: map[string]string{"runbook": "https://wiki/cpu"},
		StartsAt:    time.Now(),
	}
}

func TestAnthropicTriager_SuccessfulTriage(t *testing.T) {
	body := makeTriageResponse("P2", "High CPU on api-server", "Memory leak in handler", "Restart pods", true)
	srv := makeAnthropicStub(t, http.StatusOK, body)
	triager := makeAnthropicTriager(t, srv.URL)

	result, err := triager.Triage(context.Background(), testEnvelope())
	if err != nil {
		t.Fatalf("Triage: %v", err)
	}
	if result.ConfirmedSeverity != "P2" {
		t.Errorf("severity = %q, want P2", result.ConfirmedSeverity)
	}
	if !result.NeedsHuman {
		t.Error("P2 alert should require human")
	}
	if result.Degraded {
		t.Error("result should not be degraded on valid JSON")
	}
}

func TestAnthropicTriager_CacheHitLogged(t *testing.T) {
	body := anthropicResponseBody(`{"confirmed_severity":"P3","summary":"minor","likely_cause":"noise","affected_services":[],"recommended_action":"monitor","needs_human":false}`, true)
	srv := makeAnthropicStub(t, http.StatusOK, body)
	triager := makeAnthropicTriager(t, srv.URL)

	result, err := triager.Triage(context.Background(), testEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	if result.ConfirmedSeverity != "P3" {
		t.Errorf("severity = %q, want P3", result.ConfirmedSeverity)
	}
}

func TestAnthropicTriager_NonJSONDegrades(t *testing.T) {
	body := anthropicResponseBody("Sorry, I cannot analyse this alert.", false)
	srv := makeAnthropicStub(t, http.StatusOK, body)
	triager := makeAnthropicTriager(t, srv.URL)

	env := testEnvelope()
	env.Severity = alert.SeverityP1

	result, err := triager.Triage(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Degraded {
		t.Error("non-JSON response should set Degraded=true")
	}
	if !result.NeedsHuman {
		t.Error("P1 degraded result should set NeedsHuman=true")
	}
}

func TestAnthropicTriager_APIError(t *testing.T) {
	srv := makeAnthropicStub(t, http.StatusUnauthorized, `{"error":"bad key"}`)
	triager := makeAnthropicTriager(t, srv.URL)

	_, err := triager.Triage(context.Background(), testEnvelope())
	if err == nil {
		t.Fatal("expected error on 401")
	}
}

func TestAnthropicTriager_InvalidSeverityDegrades(t *testing.T) {
	body := anthropicResponseBody(`{"confirmed_severity":"CRITICAL","summary":"bad","likely_cause":"x","affected_services":[],"recommended_action":"y","needs_human":false}`, false)
	srv := makeAnthropicStub(t, http.StatusOK, body)
	triager := makeAnthropicTriager(t, srv.URL)

	result, err := triager.Triage(context.Background(), testEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Degraded {
		t.Error("invalid severity should set Degraded=true")
	}
}

func TestAnthropicTriager_ImplementsTriager(t *testing.T) {
	// Compile-time interface check.
	srv := makeAnthropicStub(t, http.StatusOK, makeTriageResponse("P3", "s", "c", "a", false))
	var _ Triager = makeAnthropicTriager(t, srv.URL)
}

// stubPromptProvider returns a fixed prompt regardless of agent/model.
type stubPromptProvider struct{ prompt string }

func (s *stubPromptProvider) Get(_, _ string) string { return s.prompt }

// emptyRAGRetriever returns no chunks and no error.
type emptyRAGRetriever struct{}

func (emptyRAGRetriever) Search(_ context.Context, _, _ string, _ int) ([]qdrant.RunbookChunk, error) {
	return nil, nil
}

func TestAnthropicTriager_NilLogUsesNop(t *testing.T) {
	// NewAnthropicTriager with nil log should not panic and should use zap.NewNop().
	c, err := anthropicpkg.New(anthropicpkg.Config{
		APIKey: "test-key",
		Model:  "claude-test",
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("anthropicpkg.New: %v", err)
	}
	tr := NewAnthropicTriager(c, nil)
	if tr == nil {
		t.Fatal("expected non-nil triager")
	}
	if tr.log == nil {
		t.Fatal("expected log to be replaced with zap.NewNop()")
	}
}

func TestAnthropicTriager_WithPromptStore_AttachesProvider(t *testing.T) {
	srv := makeAnthropicStub(t, http.StatusOK, makeTriageResponse("P2", "s", "c", "a", false))
	tr := makeAnthropicTriager(t, srv.URL)

	ps := &stubPromptProvider{prompt: "custom system prompt"}
	tr2 := tr.WithPromptStore(ps)
	if tr2 != tr {
		t.Fatal("WithPromptStore should return the same triager (builder pattern)")
	}
	if tr.ps == nil {
		t.Fatal("prompt provider was not attached")
	}

	// Exercise the prompt provider branch in Triage.
	if _, err := tr.Triage(context.Background(), testEnvelope()); err != nil {
		t.Fatalf("Triage with prompt store: %v", err)
	}
}

func TestAnthropicTriager_WithPromptStore_EmptyPromptFallsBack(t *testing.T) {
	srv := makeAnthropicStub(t, http.StatusOK, makeTriageResponse("P2", "s", "c", "a", false))
	tr := makeAnthropicTriager(t, srv.URL)

	// Empty prompt should leave sysPrompt untouched (covers the empty-string branch).
	tr.WithPromptStore(&stubPromptProvider{prompt: ""})
	if _, err := tr.Triage(context.Background(), testEnvelope()); err != nil {
		t.Fatalf("Triage with empty prompt: %v", err)
	}
}

func TestAnthropicTriager_WithRAG_AttachesBuilder(t *testing.T) {
	srv := makeAnthropicStub(t, http.StatusOK, makeTriageResponse("P2", "s", "c", "a", false))
	tr := makeAnthropicTriager(t, srv.URL)

	// Use a real RAG builder with a retriever returning no results — exercises
	// the rag != nil branch in Triage without needing a real Qdrant.
	rb := NewRAGContextBuilder(emptyRAGRetriever{}, zap.NewNop())
	tr2 := tr.WithRAG(rb)
	if tr2 != tr {
		t.Fatal("WithRAG should return the same triager (builder pattern)")
	}
	if tr.rag == nil {
		t.Fatal("rag builder was not attached")
	}

	if _, err := tr.Triage(context.Background(), testEnvelope()); err != nil {
		t.Fatalf("Triage with rag: %v", err)
	}
}
