package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fakePublisher struct {
	published []alert.AlertEnvelope
}

func (f *fakePublisher) PublishAlert(_ context.Context, env alert.AlertEnvelope) error {
	f.published = append(f.published, env)
	return nil
}

type fakeDedup struct {
	seen map[string]bool
}

func newFakeDedup() *fakeDedup { return &fakeDedup{seen: make(map[string]bool)} }

func (f *fakeDedup) IsDuplicate(_ context.Context, env *alert.AlertEnvelope) (bool, error) {
	if env.Status == alert.StatusResolved {
		return false, nil
	}
	key := env.TenantID + ":" + env.Fingerprint
	if f.seen[key] {
		return true, nil
	}
	f.seen[key] = true
	return false, nil
}

func (f *fakeDedup) Reset(_ context.Context, tenantID, fingerprint string) error {
	delete(f.seen, tenantID+":"+fingerprint)
	return nil
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestWebhookHandler_Alertmanager_Success(t *testing.T) {
	pub := &fakePublisher{}
	wh := handler.NewWebhookHandler(pub, newFakeDedup(), zap.NewNop())

	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())

	payload := alertmanagerBody(t, "firing", "HighCPU", "critical")
	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager/tenant-123", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
	require.Len(t, pub.published, 1)
	assert.Equal(t, "tenant-123", pub.published[0].TenantID)
	assert.Equal(t, alert.SeverityP1, pub.published[0].Severity)
}

func TestWebhookHandler_Alertmanager_DeduplicatesSameFiring(t *testing.T) {
	pub := &fakePublisher{}
	wh := handler.NewWebhookHandler(pub, newFakeDedup(), zap.NewNop())

	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())

	payload := alertmanagerBody(t, "firing", "HighCPU", "critical")

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager/t1", bytes.NewReader(payload))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req1)

	// Second identical request — should be deduped
	req2 := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager/t1", bytes.NewReader(payload))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	assert.Equal(t, http.StatusAccepted, rr2.Code)
	assert.Len(t, pub.published, 1, "second identical alert should be suppressed by dedup")
}

func TestWebhookHandler_Alertmanager_MissingTenantID(t *testing.T) {
	wh := handler.NewWebhookHandler(&fakePublisher{}, newFakeDedup(), zap.NewNop())

	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())

	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager/", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// chi returns 301 or 404 for missing trailing segment — either is fine
	assert.NotEqual(t, http.StatusAccepted, rr.Code)
}

func TestWebhookHandler_Alertmanager_InvalidPayload(t *testing.T) {
	wh := handler.NewWebhookHandler(&fakePublisher{}, newFakeDedup(), zap.NewNop())

	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())

	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager/t1", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestWebhookHandler_Alertmanager_ResolvedClearsDedup(t *testing.T) {
	pub := &fakePublisher{}
	dedup := newFakeDedup()
	wh := handler.NewWebhookHandler(pub, dedup, zap.NewNop())

	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())

	// Fire alert
	firing := alertmanagerBody(t, "firing", "HighCPU", "critical")
	req1 := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager/t1", bytes.NewReader(firing))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req1)

	// Resolve alert — should clear dedup entry
	resolved := alertmanagerBody(t, "resolved", "HighCPU", "critical")
	req2 := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager/t1", bytes.NewReader(resolved))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req2)

	assert.Len(t, pub.published, 2, "both firing and resolved should be published")
}

// ── helpers ───────────────────────────────────────────────────────────────────

func alertmanagerBody(t *testing.T, status, alertname, severity string) []byte {
	t.Helper()
	type amAlert struct {
		Status      string            `json:"status"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		StartsAt    time.Time         `json:"startsAt"`
		EndsAt      time.Time         `json:"endsAt"`
	}
	payload := map[string]interface{}{
		"version":  "4",
		"status":   status,
		"receiver": "paladin",
		"alerts": []amAlert{{
			Status:   status,
			Labels:   map[string]string{"alertname": alertname, "severity": severity, "namespace": "prod"},
			StartsAt: time.Now(),
		}},
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	return b
}
