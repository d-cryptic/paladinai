package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/paladinai/paladinai/internal/qdrant"
	"github.com/paladinai/paladinai/services/paladin-hub/internal/handler"
	"go.uber.org/zap"
)

// stubPointStore satisfies qdrant.PointStore with in-memory storage.
type stubPointStore struct {
	mu     sync.RWMutex
	points []qdrant.Point
}

func (s *stubPointStore) Upsert(_ context.Context, _ string, points []qdrant.Point) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.points = append(s.points, points...)
	return nil
}

func (s *stubPointStore) Search(_ context.Context, _ string, _ []float32, topK int) ([]qdrant.SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]qdrant.SearchResult, 0, len(s.points))
	for _, p := range s.points {
		results = append(results, qdrant.SearchResult{
			ID:      p.ID,
			Score:   0.9,
			Payload: p.Payload,
		})
		if len(results) >= topK {
			break
		}
	}
	return results, nil
}

func (s *stubPointStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.points)
}

func newTestRunbookHandler() (*handler.RunbookHandler, *stubPointStore) {
	store := &stubPointStore{}
	indexer := qdrant.NewIndexer(store, &qdrant.StubEmbedder{})
	return handler.NewRunbookHandler(indexer, zap.NewNop()), store
}

func mountRunbookRoutes(h *handler.RunbookHandler) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		h.RunbookRoutes(r)
	})
	return r
}

func TestRunbookImport_MissingTenant(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	body, _ := json.Marshal(map[string]any{"source": "github", "repo": "owner/repo"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestRunbookImport_MissingSource(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", rec.Code)
	}
}

func TestRunbookImport_GitHubSource(t *testing.T) {
	h, store := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write([]byte("# Redis OOM Runbook\n\nCheck pod memory and restart safely.\n"))
	}))
	defer source.Close()

	body, _ := json.Marshal(map[string]any{
		"source": "github",
		"repo":   source.URL,
		"path":   "docs/runbooks/redis.md",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["imported"] == nil {
		t.Error("want imported field in response")
	}
	if store.Count() == 0 {
		t.Error("want at least one chunk indexed in store")
	}
}

func TestRunbookImport_GitHubSourceFailsOnMissingRepo(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	body, _ := json.Marshal(map[string]any{
		"source": "github",
		"path":   "docs/runbooks",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRunbookImport_RemoteRunbookTooLarge(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("a"), (2<<20)+2))
	}))
	defer source.Close()

	body, _ := json.Marshal(map[string]any{
		"source": "github",
		"repo":   source.URL,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRunbookImport_FileSource(t *testing.T) {
	// Write a temp runbook file.
	dir := t.TempDir()
	p := filepath.Join(dir, "runbook.md")
	content := "# DB Failover Runbook\n\nStep 1: Check replication lag.\nStep 2: Promote replica.\n"
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	body, _ := json.Marshal(map[string]any{"source": "file", "path": p})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRunbookList(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	// Import one first.
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("# API Runbook\n\nInvestigate failed deploys."))
	}))
	defer source.Close()

	body, _ := json.Marshal(map[string]any{"source": "github", "repo": source.URL})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/runbooks", nil)
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	data, ok := resp["data"].([]any)
	if !ok || len(data) == 0 {
		t.Error("want at least one runbook in list")
	}
}

func TestRunbookList_IsTenantScoped(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("# Tenant Runbook\n\nOnly visible to owning tenant."))
	}))
	defer source.Close()

	for _, tenantID := range []string{"tenant-alpha", "tenant-beta"} {
		body, _ := json.Marshal(map[string]any{"source": "github", "repo": source.URL})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", tenantID)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("import %s: want 201, got %d: %s", tenantID, rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runbooks", nil)
	req.Header.Set("X-Tenant-ID", "tenant-alpha")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data []handler.RunbookRecord `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("tenant-alpha should see exactly one runbook, got %+v", resp.Data)
	}
	if resp.Data[0].TenantID != "tenant-alpha" {
		t.Fatalf("tenant leak: got record for %q", resp.Data[0].TenantID)
	}
}

func TestRunbookSearch(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	// Import runbook first so search has something to find.
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("# Failover Runbook\n\nSteps for database failover."))
	}))
	defer source.Close()

	importBody, _ := json.Marshal(map[string]any{"source": "github", "repo": source.URL})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", bytes.NewReader(importBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	searchBody, _ := json.Marshal(map[string]any{"query": "failover runbook", "top_k": 3})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/search", bytes.NewReader(searchBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRunbookSearch_MissingQuery(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	body, _ := json.Marshal(map[string]any{"query": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", rec.Code)
	}
}
