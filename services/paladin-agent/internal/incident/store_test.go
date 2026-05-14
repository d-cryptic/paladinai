package incident_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// ─── ReplayPublisher tests ────────────────────────────────────────────────────

type fakeReplayPublisher struct {
	mu       sync.Mutex
	subjects []string
	err      error
}

func (f *fakeReplayPublisher) Publish(_ context.Context, subject string, _ []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subjects = append(f.subjects, subject)
	return f.err
}

func (f *fakeReplayPublisher) Subjects() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.subjects...)
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
		Labels:      map[string]string{"alertname": "DB Down"},
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

	subjects := pub.Subjects()
	if len(subjects) != 1 {
		t.Fatalf("want 1 NATS publish, got %d", len(subjects))
	}
	if !strings.Contains(subjects[0], "paladin.alerts.raw") {
		t.Errorf("expected raw ingest subject, got %q", subjects[0])
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
	if subjects := pub.Subjects(); len(subjects) != 0 {
		t.Errorf("expected no NATS publish for legacy incident, got %d", len(subjects))
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

func TestReplay_InvalidFingerprintDoesNotPublish(t *testing.T) {
	s := incident.NewStore()
	pub := &fakeReplayPublisher{}
	h := incident.NewHandler(s, zap.NewNop()).WithReplayPublisher(pub)
	srv := mount(h)

	env := &alert.AlertEnvelope{
		TenantID:    "tenant-a",
		Fingerprint: "fp.bad",
	}
	inc := s.RecordFromEnvelope(env, "P2", nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/"+inc.ID+"/replay", bytes.NewReader(nil))
	req.Header.Set("X-Tenant-ID", "tenant-a")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", rec.Code)
	}

	time.Sleep(50 * time.Millisecond)

	if subjects := pub.Subjects(); len(subjects) != 0 {
		t.Fatalf("expected no NATS publish for invalid fingerprint, got %d", len(subjects))
	}

	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode replay response: %v", err)
	}
	replayID, _ := result["replay_id"].(string)
	replayInc := s.Get(replayID)
	if replayInc == nil {
		t.Fatal("replay incident not found")
	}
	if replayInc.Status == incident.StatusResolved {
		t.Errorf("want status != resolved when replay subject is invalid, got %s", replayInc.Status)
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

func TestStore_IsolatesMutableIncidentFields(t *testing.T) {
	s := incident.NewStore()
	labels := map[string]string{"service": "api"}
	result := json.RawMessage(`{"summary":"ok"}`)

	inc := s.Record("tenant-a", "P2", "Latency", 1, labels, result)
	labels["service"] = "mutated"
	result[12] = 'x'
	inc.Labels["service"] = "returned-mutated"
	inc.TriageResult[12] = 'y'

	got := s.Get(inc.ID)
	if got.Labels["service"] != "api" {
		t.Fatalf("stored labels mutated through caller reference: %q", got.Labels["service"])
	}
	if string(got.TriageResult) != `{"summary":"ok"}` {
		t.Fatalf("stored triage result mutated through caller reference: %s", got.TriageResult)
	}

	got.Labels["service"] = "get-mutated"
	got.TriageResult[12] = 'z'
	again := s.Get(inc.ID)
	if again.Labels["service"] != "api" {
		t.Fatalf("stored labels mutated through Get result: %q", again.Labels["service"])
	}
	if string(again.TriageResult) != `{"summary":"ok"}` {
		t.Fatalf("stored triage result mutated through Get result: %s", again.TriageResult)
	}
}

func TestStartReplay_IsolatesLabels(t *testing.T) {
	s := incident.NewStore()
	inc := s.Record("tenant-a", "P2", "Latency", 1, map[string]string{"service": "api"}, nil)

	replay, err := s.StartReplay(inc.ID, "tenant-a")
	if err != nil {
		t.Fatalf("start replay: %v", err)
	}
	replay.Labels["service"] = "mutated"

	source := s.Get(inc.ID)
	if source.Labels["service"] != "api" {
		t.Fatalf("source labels mutated through replay result: %q", source.Labels["service"])
	}
	gotReplay := s.Get(replay.ID)
	if gotReplay.Labels["service"] != "api" {
		t.Fatalf("stored replay labels mutated through replay result: %q", gotReplay.Labels["service"])
	}
}
