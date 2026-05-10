package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/paladinai/paladinai/services/paladin-edge/internal/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_Liveness(t *testing.T) {
	h := &handler.HealthHandler{}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	h.Liveness(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}

func TestHealthHandler_Readiness(t *testing.T) {
	h := &handler.HealthHandler{}
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	h.Readiness(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, "paladin-edge", body["service"])
	assert.NotEmpty(t, body["uptime"])
}
