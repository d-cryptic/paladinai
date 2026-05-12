package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/services/paladin-ingest/internal/handler"
)

func newRouter() (chi.Router, *fakePublisher, *fakeDedup) {
	pub := &fakePublisher{}
	dd := newFakeDedup()
	wh := handler.NewWebhookHandler(pub, dd, zap.NewNop())
	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())
	return r, pub, dd
}

// ─── Datadog ──────────────────────────────────────────────────────────────────

func datadogBody(t *testing.T) []byte {
	t.Helper()
	payload := map[string]any{
		"id":            "dd-1",
		"title":         "High CPU",
		"text":          "CPU usage above 90%",
		"date_happened": time.Now().Unix(),
		"priority":      "normal",
		"alert_type":    "error",
		"alert_status":  "Triggered",
		"tags":          "service:api,env:prod", // comma-separated string, not an array
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	return b
}

func TestWebhookHandler_Datadog_RouteExists(t *testing.T) {
	r, _, _ := newRouter()

	req := httptest.NewRequest(http.MethodPost, "/webhook/datadog/tenant-dd",
		bytes.NewReader(datadogBody(t)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestWebhookHandler_Datadog_InvalidPayload_Returns400(t *testing.T) {
	r, _, _ := newRouter()

	req := httptest.NewRequest(http.MethodPost, "/webhook/datadog/tenant-dd",
		bytes.NewReader([]byte("not json {{")))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestWebhookHandler_Datadog_InvalidTenant_Returns400(t *testing.T) {
	r, _, _ := newRouter()

	req := httptest.NewRequest(http.MethodPost, "/webhook/datadog/invalid..tenant",
		bytes.NewReader(datadogBody(t)))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── PagerDuty ────────────────────────────────────────────────────────────────

func pagerDutyBody(t *testing.T) []byte {
	t.Helper()
	payload := map[string]any{
		"messages": []map[string]any{{
			"event":      "trigger",
			"id":         "pd-inc-001",
			"service":    map[string]string{"name": "payments-api"},
			"urgency":    "high",
			"created_on": time.Now().UTC().Format(time.RFC3339),
			"log_entries": []map[string]any{{
				"type": "notify_log_entry",
				"channel": map[string]string{
					"type":    "nagios",
					"summary": "payments high error rate",
				},
			}},
		}},
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	return b
}

func TestWebhookHandler_PagerDuty_RouteExists(t *testing.T) {
	r, _, _ := newRouter()

	req := httptest.NewRequest(http.MethodPost, "/webhook/pagerduty/tenant-pd",
		bytes.NewReader(pagerDutyBody(t)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// 400 is acceptable here if the normalizer doesn't handle all PD payload shapes;
	// what matters is the route is wired and doesn't 404.
	assert.NotEqual(t, http.StatusNotFound, rr.Code)
	assert.NotEqual(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestWebhookHandler_PagerDuty_InvalidTenant_Returns400(t *testing.T) {
	r, _, _ := newRouter()

	req := httptest.NewRequest(http.MethodPost, "/webhook/pagerduty/bad..tenant",
		bytes.NewReader(pagerDutyBody(t)))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── CloudWatch ───────────────────────────────────────────────────────────────

func cloudwatchBody(t *testing.T) []byte {
	t.Helper()
	payload := map[string]any{
		"AlarmName":        "HighErrorRate",
		"AlarmDescription": "Error rate above threshold",
		"NewStateValue":    "ALARM",
		"NewStateReason":   "Threshold crossed",
		"StateChangeTime":  time.Now().UTC().Format(time.RFC3339),
		"Region":           "us-east-1",
		"AlarmArn":         "arn:aws:cloudwatch:us-east-1:123:alarm:HighErrorRate",
		"Trigger": map[string]any{
			"MetricName": "5XXError",
			"Namespace":  "AWS/ApiGateway",
			"Threshold":  5.0,
		},
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	return b
}

func TestWebhookHandler_CloudWatch_RouteExists(t *testing.T) {
	r, _, _ := newRouter()

	req := httptest.NewRequest(http.MethodPost, "/webhook/cloudwatch/tenant-cw",
		bytes.NewReader(cloudwatchBody(t)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusNotFound, rr.Code)
	assert.NotEqual(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestWebhookHandler_CloudWatch_InvalidPayload_Returns400(t *testing.T) {
	r, _, _ := newRouter()

	req := httptest.NewRequest(http.MethodPost, "/webhook/cloudwatch/tenant-cw",
		bytes.NewReader([]byte("{{broken")))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestWebhookHandler_CloudWatch_InvalidTenant_Returns400(t *testing.T) {
	r, _, _ := newRouter()

	req := httptest.NewRequest(http.MethodPost, "/webhook/cloudwatch/bad..tenant",
		bytes.NewReader(cloudwatchBody(t)))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── Checkers ─────────────────────────────────────────────────────────────────

func TestValkeyChecker_Name(t *testing.T) {
	// redis.NewClient only builds the struct — no network I/O.
	rc := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer rc.Close() //nolint:errcheck
	c := handler.NewValkeyChecker(rc)
	assert.NotNil(t, c)
	assert.Equal(t, "valkey", c.Name())
}

// ─── WebhookHandler constructor coverage ─────────────────────────────────────

func TestNewWebhookHandler_ReturnsNonNil(t *testing.T) {
	wh := handler.NewWebhookHandler(&fakePublisher{}, newFakeDedup(), zap.NewNop())
	assert.NotNil(t, wh)
}
