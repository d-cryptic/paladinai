package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// errorDedup always returns an error from IsDuplicate.
type errorDedup struct{}

func (e *errorDedup) IsDuplicate(_ context.Context, _ *alert.AlertEnvelope) (bool, error) {
	return false, fmt.Errorf("valkey unavailable")
}
func (e *errorDedup) Reset(_ context.Context, _, _ string) error { return nil }

// errorPublisher always returns an error from PublishAlert.
type errorPublisher struct{}

func (e *errorPublisher) PublishAlert(_ context.Context, _ alert.AlertEnvelope) error {
	return fmt.Errorf("nats: publish failed")
}

// countingPublisher succeeds for the first failAfter calls, then always fails.
type countingPublisher struct {
	calls     int
	failAfter int
}

func (c *countingPublisher) PublishAlert(_ context.Context, _ alert.AlertEnvelope) error {
	c.calls++
	if c.calls > c.failAfter {
		return fmt.Errorf("nats: publish failed (call %d)", c.calls)
	}
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

func TestWebhookHandler_Alertmanager_InvalidTenantID_NATSInjection(t *testing.T) {
	wh := handler.NewWebhookHandler(&fakePublisher{}, newFakeDedup(), zap.NewNop())

	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())

	// Tenant IDs with NATS wildcard characters must be rejected
	badIDs := []string{"foo.bar", "foo>bar", "foo*bar", "foo..bar"}
	payload := alertmanagerBody(t, "firing", "Test", "warning")

	for _, id := range badIDs {
		req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager/"+id, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code, "expected rejection for tenant_id=%q", id)
	}
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

func TestWebhookHandler_Alertmanager_OversizedPayload(t *testing.T) {
	wh := handler.NewWebhookHandler(&fakePublisher{}, newFakeDedup(), zap.NewNop())

	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())

	// Build a JSON body just over 8 MB.
	bigLabel := make([]byte, 8*1024*1024+1)
	for i := range bigLabel {
		bigLabel[i] = 'x'
	}
	type amAlert struct {
		Status string            `json:"status"`
		Labels map[string]string `json:"labels"`
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"version":  "4",
		"status":   "firing",
		"receiver": "paladin",
		"alerts": []amAlert{{
			Status: "firing",
			Labels: map[string]string{"alertname": "BigAlert", "severity": "critical", "padding": string(bigLabel)},
		}},
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager/tenant-big", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// The LimitReader silently truncates at 8 MiB; the truncated JSON is invalid.
	assert.Equal(t, http.StatusBadRequest, rr.Code, "oversized body should yield 400 after truncated JSON parse failure")
}

func TestWebhookHandler_Alertmanager_DedupErrorIsNonFatal(t *testing.T) {
	pub := &fakePublisher{}
	dedup := &errorDedup{}
	wh := handler.NewWebhookHandler(pub, dedup, zap.NewNop())

	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())

	payload := alertmanagerBody(t, "firing", "HighCPU", "critical")
	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager/tenant-dd", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// Dedup error is non-fatal: alert is still published and 202 is returned.
	// `failed` counts only publish failures, not dedup check errors, so it should be 0.
	assert.Equal(t, http.StatusAccepted, rr.Code)
	require.Len(t, pub.published, 1, "alert should be published even when dedup check errors")

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, float64(1), body["published"], "alert should be counted as published despite dedup error")
	assert.Equal(t, float64(0), body["failed"], "dedup error should not count as a publish failure")
}

func TestWebhookHandler_Alertmanager_PublishFailureCountedAsFailed(t *testing.T) {
	pub := &errorPublisher{}
	wh := handler.NewWebhookHandler(pub, newFakeDedup(), zap.NewNop())

	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())

	payload := alertmanagerBody(t, "firing", "HighCPU", "critical")
	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager/tenant-pub", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// Even on publish failure we return 202 (Alertmanager retries on non-2xx,
	// causing duplicates for already-published alerts in a batch).
	assert.Equal(t, http.StatusAccepted, rr.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, float64(0), body["published"])
	assert.Equal(t, float64(1), body["failed"])
}

func TestWebhookHandler_Alertmanager_BatchPartialFailure(t *testing.T) {
	pub := &countingPublisher{failAfter: 1}
	wh := handler.NewWebhookHandler(pub, newFakeDedup(), zap.NewNop())

	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())

	// Send 3 alerts: first succeeds, remaining fail.
	payload := alertmanagerBatch(t, 3, "firing")
	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager/tenant-batch", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code, "202 even on partial failure")

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, float64(3), body["total"])
	assert.Equal(t, float64(1), body["published"])
	assert.Equal(t, float64(2), body["failed"])
}

// fakeStormDetector implements StormDetector for tests.
type fakeStormDetector struct {
	storm bool
	err   error
	calls int
}

func (f *fakeStormDetector) Record(_ context.Context, _ string) (bool, int64, error) {
	f.calls++
	return f.storm, int64(f.calls), f.err
}

func TestWebhookHandler_StormDetector_TagsAlertsOnStorm(t *testing.T) {
	pub := &fakePublisher{}
	det := &fakeStormDetector{storm: true}
	wh := handler.NewWebhookHandler(pub, newFakeDedup(), zap.NewNop()).WithStormDetector(det)

	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())

	payload := alertmanagerBody(t, "firing", "HighCPU", "critical")
	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager/tenant-storm", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
	require.Len(t, pub.published, 1)
	assert.Equal(t, "true", pub.published[0].Labels["storm"], "storm label must be set when detector fires")
}

func TestWebhookHandler_StormDetector_NoTagWhenNoStorm(t *testing.T) {
	pub := &fakePublisher{}
	det := &fakeStormDetector{storm: false}
	wh := handler.NewWebhookHandler(pub, newFakeDedup(), zap.NewNop()).WithStormDetector(det)

	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())

	payload := alertmanagerBody(t, "firing", "HighCPU", "warning")
	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager/tenant-ok", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
	require.Len(t, pub.published, 1)
	assert.Empty(t, pub.published[0].Labels["storm"], "storm label must not be set when detector is calm")
}

func TestWebhookHandler_StormDetector_ErrorIsNonFatal(t *testing.T) {
	pub := &fakePublisher{}
	det := &fakeStormDetector{storm: false, err: fmt.Errorf("valkey down")}
	wh := handler.NewWebhookHandler(pub, newFakeDedup(), zap.NewNop()).WithStormDetector(det)

	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())

	payload := alertmanagerBody(t, "firing", "HighCPU", "critical")
	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager/tenant-sd-err", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// Storm detector error must not suppress alert publication.
	assert.Equal(t, http.StatusAccepted, rr.Code)
	require.Len(t, pub.published, 1, "alert should be published even when storm detector errors")
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

// alertmanagerBatch builds a payload with n distinct alerts (unique alertnames).
func alertmanagerBatch(t *testing.T, n int, status string) []byte {
	t.Helper()
	type amAlert struct {
		Status   string            `json:"status"`
		Labels   map[string]string `json:"labels"`
		StartsAt time.Time         `json:"startsAt"`
	}
	alerts := make([]amAlert, n)
	for i := range alerts {
		alerts[i] = amAlert{
			Status:   status,
			Labels:   map[string]string{"alertname": fmt.Sprintf("Alert%d", i), "severity": "critical"},
			StartsAt: time.Now(),
		}
	}
	b, err := json.Marshal(map[string]interface{}{
		"version":  "4",
		"status":   status,
		"receiver": "paladin",
		"alerts":   alerts,
	})
	require.NoError(t, err)
	return b
}
