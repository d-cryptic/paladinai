package incident_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/paladinai/paladinai/internal/alert"
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

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
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

// ─── ReplayPublisher tests ────────────────────────────────────────────────────

type fakeReplayPublisher struct {
	subjects []string
	err      error
}

func (f *fakeReplayPublisher) Publish(_ context.Context, subject string, _ []byte) error {
	f.subjects = append(f.subjects, subject)
	return f.err
}

func TestReplay_PublishesToNATS(t *testing.T) {
	s := incident.NewStore()
	pub := &fakeReplayPublisher{}
	h := incident.NewHandler(s, zap.NewNop()).WithReplayPublisher(pub)
	srv := mount(h)

	// RecordFromEnvelope stores the raw envelope.
	env := &alert.AlertEnvelope{
		TenantID:    "tenant-a",
		Fingerprint: "fp-abc",
		Labels:      map[string]string{"alertname": "DB Down", "fingerprint": "fp-abc"},
		Severity:    "P1",
	}
	inc := s.RecordFromEnvelope(env, "critical", nil)

	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/"+inc.ID+"/replay", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "tenant-a")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Give the goroutine time to publish.
	time.Sleep(50 * time.Millisecond)

	if len(pub.subjects) != 1 {
		t.Fatalf("want 1 NATS publish, got %d", len(pub.subjects))
	}
	if !strings.Contains(pub.subjects[0], "paladin.alerts.raw") {
		t.Errorf("expected raw ingest subject, got %q", pub.subjects[0])
	}
}

func TestReplay_NoEnvelope_MarksResolvedWithoutPublish(t *testing.T) {
	s := incident.NewStore()
	pub := &fakeReplayPublisher{}
	h := incident.NewHandler(s, zap.NewNop()).WithReplayPublisher(pub)
	srv := mount(h)

	// Record without envelope (legacy path).
	inc := s.Record("tenant-a", "critical", "Old Incident", 1, nil, nil)

	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/"+inc.ID+"/replay", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "tenant-a")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", rec.Code)
	}

	time.Sleep(50 * time.Millisecond)

	// No publish should happen.
	if len(pub.subjects) != 0 {
		t.Errorf("expected no NATS publish for legacy incident, got %d", len(pub.subjects))
	}
}

func TestReplay_NATSPublishFailure_DoesNotMarkResolved(t *testing.T) {
	s := incident.NewStore()
	pub := &fakeReplayPublisher{err: errors.New("nats: connection lost")}
	h := incident.NewHandler(s, zap.NewNop()).WithReplayPublisher(pub)
	srv := mount(h)

	env := &alert.AlertEnvelope{
		TenantID:    "tenant-a",
		Fingerprint: "fp-fail",
		Labels:      map[string]string{"fingerprint": "fp-fail"},
	}
	inc := s.RecordFromEnvelope(env, "P2", nil)

	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/"+inc.ID+"/replay", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "tenant-a")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", rec.Code)
	}

	time.Sleep(50 * time.Millisecond)

	// The replay incident should still be "replaying" (not resolved) since publish failed.
	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)
	replayID, _ := result["replay_id"].(string)
	replayInc := s.Get(replayID)
	if replayInc == nil {
		t.Fatal("replay incident not found")
	}
	if replayInc.Status == incident.StatusResolved {
		t.Errorf("want status != resolved when publish fails, got %s", replayInc.Status)
	}
}

func TestRecordFromEnvelope_StoresRawEnvelope(t *testing.T) {
	s := incident.NewStore()
	env := &alert.AlertEnvelope{
		TenantID:    "tenant-a",
		Fingerprint: "fp-raw",
		Labels:      map[string]string{"alertname": "Test"},
	}
	inc := s.RecordFromEnvelope(env, "P2", nil)
	got := s.Get(inc.ID)
	if got == nil {
		t.Fatal("incident not found")
	}
	if len(got.RawEnvelope) == 0 {
		t.Error("want RawEnvelope to be set")
	}
}
