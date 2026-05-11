package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/paladinai/paladinai/services/paladin-hub/internal/handler"
	"github.com/paladinai/paladinai/services/paladin-hub/internal/registry"
	"github.com/paladinai/paladinai/services/paladin-hub/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestRouter() (*chi.Mux, *store.MemStore) {
	s := store.NewMemStore()
	h := handler.New(s, zap.NewNop())
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		h.Routes(r)
	})
	return r, s
}

func withTenant(req *http.Request, tenantID string) *http.Request {
	req.Header.Set("X-Tenant-ID", tenantID)
	return req
}

func registerServer(t *testing.T, r http.Handler, tenantID string, body registry.RegisterRequest) *httptest.ResponseRecorder {
	t.Helper()
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req = withTenant(req, tenantID)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func validRegisterReq() registry.RegisterRequest {
	return registry.RegisterRequest{
		ID:           "test-server",
		Name:         "Test MCP Server",
		Endpoint:     "https://mcp.test.com",
		Capabilities: []string{"read_logs", "exec"},
	}
}

func TestHandler_Register_Created(t *testing.T) {
	r, _ := newTestRouter()
	rr := registerServer(t, r, "tenant-1", validRegisterReq())

	assert.Equal(t, http.StatusCreated, rr.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	data := body["data"].(map[string]any)
	assert.Equal(t, "test-server", data["id"])
	assert.Equal(t, "tenant-1", data["tenant_id"])
}

func TestHandler_Register_MissingTenant(t *testing.T) {
	r, _ := newTestRouter()
	data, _ := json.Marshal(validRegisterReq())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_Register_InvalidBody(t *testing.T) {
	r, _ := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers", bytes.NewBufferString("not json"))
	req = withTenant(req, "t1")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_Register_ValidationFails(t *testing.T) {
	r, _ := newTestRouter()
	bad := validRegisterReq()
	bad.Capabilities = nil
	rr := registerServer(t, r, "t1", bad)
	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
}

func TestHandler_List(t *testing.T) {
	r, _ := newTestRouter()

	registerServer(t, r, "tenant-1", validRegisterReq())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers", nil)
	req = withTenant(req, "tenant-1")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	data := body["data"].([]any)
	assert.Len(t, data, 1)
}

func TestHandler_List_EmptyForNewTenant(t *testing.T) {
	r, _ := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers", nil)
	req = withTenant(req, "new-tenant")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	data := body["data"].([]any)
	assert.Len(t, data, 0)
}

func TestHandler_Get(t *testing.T) {
	r, _ := newTestRouter()
	registerServer(t, r, "t1", validRegisterReq())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers/test-server", nil)
	req = withTenant(req, "t1")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandler_Get_NotFound(t *testing.T) {
	r, _ := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers/nonexistent", nil)
	req = withTenant(req, "t1")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandler_Deregister(t *testing.T) {
	r, _ := newTestRouter()
	registerServer(t, r, "t1", validRegisterReq())

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/mcp/servers/test-server", nil)
	req = withTenant(req, "t1")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Confirm it's gone
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers/test-server", nil)
	req2 = withTenant(req2, "t1")
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusNotFound, rr2.Code)
}

func TestHandler_Heartbeat(t *testing.T) {
	r, _ := newTestRouter()
	registerServer(t, r, "t1", validRegisterReq())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/test-server/heartbeat", nil)
	req = withTenant(req, "t1")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandler_TenantIsolation(t *testing.T) {
	r, _ := newTestRouter()
	// Register under tenant-A
	registerServer(t, r, "tenant-a", validRegisterReq())

	// tenant-B should not see it
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers/test-server", nil)
	req = withTenant(req, "tenant-b")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code, "tenant-B must not access tenant-A servers")
}

func TestHandler_List_MissingTenant(t *testing.T) {
	r, _ := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_Get_MissingTenant(t *testing.T) {
	r, _ := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers/test-server", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_Deregister_MissingTenant(t *testing.T) {
	r, _ := newTestRouter()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/mcp/servers/test-server", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_Deregister_NotFound(t *testing.T) {
	r, _ := newTestRouter()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/mcp/servers/ghost", nil)
	req = withTenant(req, "t1")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandler_Heartbeat_MissingTenant(t *testing.T) {
	r, _ := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/test-server/heartbeat", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_Heartbeat_NotFound(t *testing.T) {
	r, _ := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/ghost/heartbeat", nil)
	req = withTenant(req, "t1")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandler_Register_ResponseContainsMeta(t *testing.T) {
	r, _ := newTestRouter()
	registerServer(t, r, "tenant-1", validRegisterReq())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers", nil)
	req = withTenant(req, "tenant-1")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	meta := body["meta"].(map[string]any)
	assert.Equal(t, float64(1), meta["total"])
}
