package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ── listRunbooks coverage ────────────────────────────────────────────────────

func TestRunbookList_MissingTenant(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runbooks", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for missing tenant, got %d", rec.Code)
	}
}

func TestRunbookList_WithLimitParam(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	// Import a runbook first.
	importBody, _ := json.Marshal(map[string]any{"source": "github", "repo": "owner/repo"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", bytes.NewReader(importBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/runbooks?limit=1", nil)
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestRunbookList_WithSourceFilter(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	// Import a github runbook.
	importBody, _ := json.Marshal(map[string]any{"source": "github", "repo": "owner/repo"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", bytes.NewReader(importBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	// Filter by source=file (should return empty list).
	req = httptest.NewRequest(http.MethodGet, "/api/v1/runbooks?source=file", nil)
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].([]any)
	if len(data) != 0 {
		t.Errorf("want 0 runbooks with source=file filter, got %d", len(data))
	}
}

func TestRunbookList_InvalidLimitIsIgnored(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runbooks?limit=notanumber", nil)
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 with invalid limit (uses default), got %d", rec.Code)
	}
}

func TestRunbookRecords_ConcurrentImportAndList(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "runbook.md")
	if err := os.WriteFile(p, []byte("# Runbook\n\nStep 1: check logs.\n"), 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{"source": "file", "path": p})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Tenant-ID", fmt.Sprintf("tenant-%d", i%3))
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusCreated {
				t.Errorf("import status = %d", rec.Code)
			}
		}(i)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/runbooks?limit=10", nil)
			req.Header.Set("X-Tenant-ID", fmt.Sprintf("tenant-%d", i%3))
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("list status = %d", rec.Code)
			}
		}(i)
	}
	wg.Wait()
}

func TestRunbookImport_BodyTooLarge(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", strings.NewReader(`{"source":"file","path":"`+strings.Repeat("x", 70*1024)+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRunbookSearch_BodyTooLarge(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/search", strings.NewReader(`{"query":"`+strings.Repeat("x", 70*1024)+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── searchRunbooks coverage ───────────────────────────────────────────────────

func TestRunbookSearch_MissingTenant(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	body, _ := json.Marshal(map[string]any{"query": "failover"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for missing tenant, got %d", rec.Code)
	}
}

func TestRunbookSearch_InvalidBody(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/search", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for invalid body, got %d", rec.Code)
	}
}

// ── fetchContent: file source edge cases ─────────────────────────────────────

func TestRunbookImport_File_NotExist(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	body, _ := json.Marshal(map[string]any{
		"source": "file",
		"path":   "/nonexistent/path/to/runbook.md",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502 for nonexistent file (fetch error), got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRunbookImport_File_SingleFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "runbook.md")
	_ = os.WriteFile(p, []byte("# Runbook\n\nStep 1: check logs.\n"), 0600)

	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	body, _ := json.Marshal(map[string]any{"source": "file", "path": p})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201 for file import, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRunbookImport_UnknownSource(t *testing.T) {
	h, _ := newTestRunbookHandler()
	srv := mountRunbookRoutes(h)

	body, _ := json.Marshal(map[string]any{"source": "s3", "path": "s3://bucket/key"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502 for unknown source (fetch error), got %d", rec.Code)
	}
}
