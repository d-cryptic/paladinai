package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/paladinai/paladinai/services/paladin-ingest/internal/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubChecker satisfies handler.Checker.
type stubChecker struct {
	name string
	err  error
}

func (s *stubChecker) Name() string                      { return s.name }
func (s *stubChecker) Check(ctx context.Context) error   { return s.err }

func TestHealthHandler_LivenessAlwaysOK(t *testing.T) {
	h := handler.NewHealthHandler("paladin-ingest")
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	h.Liveness(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}

func TestHealthHandler_ReadinessNoCheckers(t *testing.T) {
	h := handler.NewHealthHandler("paladin-ingest")
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	h.Readiness(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
	assert.NotEmpty(t, body["uptime"])
	assert.Equal(t, "paladin-ingest", body["service"])
}

func TestHealthHandler_ReadinessAllHealthy(t *testing.T) {
	h := handler.NewHealthHandler("paladin-ingest",
		&stubChecker{name: "nats", err: nil},
		&stubChecker{name: "valkey", err: nil},
	)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	h.Readiness(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])

	checks, ok := body["checks"].(map[string]any)
	require.True(t, ok, "checks field should be a map")
	assert.Equal(t, "ok", checks["nats"])
	assert.Equal(t, "ok", checks["valkey"])
}

func TestHealthHandler_ReadinessNATSUnhealthy(t *testing.T) {
	h := handler.NewHealthHandler("paladin-ingest",
		&stubChecker{name: "nats", err: errors.New("connection refused")},
		&stubChecker{name: "valkey", err: nil},
	)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	h.Readiness(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "degraded", body["status"])

	checks, ok := body["checks"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "connection refused", checks["nats"])
	assert.Equal(t, "ok", checks["valkey"])
}

func TestHealthHandler_ReadinessValkeyUnhealthy(t *testing.T) {
	h := handler.NewHealthHandler("paladin-ingest",
		&stubChecker{name: "nats", err: nil},
		&stubChecker{name: "valkey", err: errors.New("dial tcp: timeout")},
	)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	h.Readiness(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "degraded", body["status"])

	checks := body["checks"].(map[string]any)
	assert.Equal(t, "ok", checks["nats"])
	assert.Equal(t, "dial tcp: timeout", checks["valkey"])
}

func TestHealthHandler_ReadinessBothUnhealthy(t *testing.T) {
	h := handler.NewHealthHandler("paladin-ingest",
		&stubChecker{name: "nats", err: errors.New("nats down")},
		&stubChecker{name: "valkey", err: errors.New("valkey down")},
	)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	h.Readiness(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "degraded", body["status"])

	checks := body["checks"].(map[string]any)
	assert.Equal(t, "nats down", checks["nats"])
	assert.Equal(t, "valkey down", checks["valkey"])
}
