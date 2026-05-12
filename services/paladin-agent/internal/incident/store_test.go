package incident_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/incident"
	"go.uber.org/zap"
)

func newHandler() (*incident.Handler, *incident.Store) {
	s := incident.NewStore()
	h := incident.NewHandler(s, zap.NewNop())
	return h, s
}

func mount(h *incident.Handler) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		h.Routes(r)
	})
	return r
}

func TestList_Empty(t *testing.T) {
	h, _ := newHandler()
	srv := mount(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	req.Header.Set("X-Tenant-ID", "tenant-a")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].([]any)
	if len(data) != 0 {
		t.Errorf("want empty list, got %d items", len(data))
	}
}

func TestList_MissingTenant(t *testing.T) {
	h, _ := newHandler()
	srv := mount(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestRecord_And_List(t *testing.T) {
	h, s := newHandler()
	srv := mount(h)

	s.Record("tenant-a", "critical", "DB Down", 3, map[string]string{"service": "postgres"}, nil)
	s.Record("tenant-b", "warning", "High CPU", 1, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	req.Header.Set("X-Tenant-ID", "tenant-a")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("want 1 incident for tenant-a, got %d", len(data))
	}
}

func TestGet_Found(t *testing.T) {
	h, s := newHandler()
	srv := mount(h)

	inc := s.Record("tenant-a", "critical", "DB Down", 1, nil, json.RawMessage(`{"intent":"service_down"}`))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/"+inc.ID, nil)
	req.Header.Set("X-Tenant-ID", "tenant-a")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]any)
	if data["id"] != inc.ID {
		t.Errorf("want id %s, got %v", inc.ID, data["id"])
	}
}

func TestGet_CrossTenantBlocked(t *testing.T) {
	h, s := newHandler()
	srv := mount(h)

	inc := s.Record("tenant-a", "critical", "DB Down", 1, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/"+inc.ID, nil)
	req.Header.Set("X-Tenant-ID", "tenant-b")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 for cross-tenant, got %d", rec.Code)
	}
}

func TestReplay_Success(t *testing.T) {
	h, s := newHandler()
	srv := mount(h)

	inc := s.Record("tenant-a", "critical", "DB Down", 1, nil, nil)

	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/"+inc.ID+"/replay", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "tenant-a")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)
	if result["replay_id"] == nil {
		t.Error("want replay_id in response")
	}
	if result["source_id"] != inc.ID {
		t.Errorf("want source_id %s, got %v", inc.ID, result["source_id"])
	}
}

func TestReplay_NotFound(t *testing.T) {
	h, _ := newHandler()
	srv := mount(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/nonexistent/replay", nil)
	req.Header.Set("X-Tenant-ID", "tenant-a")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}

func TestStore_MarkResolved(t *testing.T) {
	s := incident.NewStore()
	inc := s.Record("tenant-a", "critical", "Test", 1, nil, nil)
	s.MarkResolved(inc.ID)
	updated := s.Get(inc.ID)
	if updated.Status != incident.StatusResolved {
		t.Errorf("want resolved, got %s", updated.Status)
	}
}

func TestStore_StartReplay_CrossTenant(t *testing.T) {
	s := incident.NewStore()
	inc := s.Record("tenant-a", "critical", "Test", 1, nil, nil)
	_, err := s.StartReplay(inc.ID, "tenant-b")
	if err == nil {
		t.Error("want error for cross-tenant replay")
	}
}
