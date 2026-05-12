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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/services/paladin-auth/internal/handler"
	"github.com/paladinai/paladinai/services/paladin-auth/internal/store"
)

const (
	testSecret      = "test-jwt-secret-32-bytes-padding!"
	testAdminSecret = "test-admin-secret"
)

func newRouter() (*chi.Mux, *store.MemStore) {
	s := store.NewMemStore()
	h := handler.New(s, []byte(testSecret), testAdminSecret, time.Hour, zap.NewNop())
	r := chi.NewRouter()
	h.Register(r)
	return r, s
}

func bodyJSON(v any) *bytes.Reader {
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}

func adminReq(method, path string, body *bytes.Reader) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, body)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("X-Admin-Secret", testAdminSecret)
	return req
}

func TestCreateTenant_Created(t *testing.T) {
	r, _ := newRouter()
	req := adminReq(http.MethodPost, "/tenants", bodyJSON(map[string]string{"slug": "acme-corp", "name": "Acme Corp"}))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	tenant := resp["tenant"].(map[string]any)
	assert.Equal(t, "acme-corp", tenant["slug"])
}

func TestCreateTenant_InvalidSlug(t *testing.T) {
	tests := []struct{ slug string }{
		{"A"}, {"bad slug"}, {"a"}, {"../etc"}, {"has_underscore"},
	}
	r, _ := newRouter()
	for _, tc := range tests {
		req := adminReq(http.MethodPost, "/tenants", bodyJSON(map[string]string{"slug": tc.slug, "name": "Test"}))
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code, "slug=%q should be rejected", tc.slug)
	}
}

func TestCreateTenant_Conflict(t *testing.T) {
	r, _ := newRouter()
	req1 := adminReq(http.MethodPost, "/tenants", bodyJSON(map[string]string{"slug": "dup-co", "name": "Dup"}))
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)
	require.Equal(t, http.StatusCreated, rr1.Code)

	req2 := adminReq(http.MethodPost, "/tenants", bodyJSON(map[string]string{"slug": "dup-co", "name": "Dup2"}))
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusConflict, rr2.Code)
}

func TestCreateTenant_NoAdminSecret(t *testing.T) {
	r, _ := newRouter()
	req := httptest.NewRequest(http.MethodPost, "/tenants",
		bodyJSON(map[string]string{"slug": "no-auth", "name": "No Auth"}))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestListTenants(t *testing.T) {
	r, s := newRouter()
	ctx := context.Background()
	_, err := s.Create(ctx, "t1", "T1")
	require.NoError(t, err)
	_, err = s.Create(ctx, "t2", "T2")
	require.NoError(t, err)

	req := adminReq(http.MethodGet, "/tenants", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	tenants := resp["tenants"].([]any)
	assert.Len(t, tenants, 2)
}

func TestGetTenant_NotFound(t *testing.T) {
	r, _ := newRouter()
	req := adminReq(http.MethodGet, "/tenants/nonexistent", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestIssueToken_Success(t *testing.T) {
	r, s := newRouter()
	tenant, err := s.Create(context.Background(), "tok-test", "Token Test")
	require.NoError(t, err)

	req := adminReq(http.MethodPost, "/tokens", bodyJSON(map[string]any{
		"tenant_id": tenant.ID,
		"user_id":   "usr-1",
		"roles":     []string{"admin"},
	}))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["token"])
	assert.Equal(t, "Bearer", resp["token_type"])
	assert.Equal(t, float64(3600), resp["expires_in"])
}

func TestIssueToken_UnknownRole(t *testing.T) {
	r, s := newRouter()
	tenant, err := s.Create(context.Background(), "role-test", "Role Test")
	require.NoError(t, err)

	req := adminReq(http.MethodPost, "/tokens", bodyJSON(map[string]any{
		"tenant_id": tenant.ID,
		"user_id":   "usr-1",
		"roles":     []string{"superadmin"},
	}))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestIssueToken_SuspendedTenant(t *testing.T) {
	r, s := newRouter()
	ctx := context.Background()
	tenant, err := s.Create(ctx, "susp-co", "Suspended")
	require.NoError(t, err)
	_, err = s.SetState(ctx, tenant.ID, store.TenantStateSuspended)
	require.NoError(t, err)

	req := adminReq(http.MethodPost, "/tokens", bodyJSON(map[string]any{
		"tenant_id": tenant.ID,
		"user_id":   "usr-1",
	}))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestGetTenant_Success(t *testing.T) {
	r, s := newRouter()
	tenant, err := s.Create(context.Background(), "get-co", "Get Co")
	require.NoError(t, err)

	req := adminReq(http.MethodGet, "/tenants/"+tenant.ID, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.NotNil(t, resp["tenant"])
}

func TestSuspendTenant_Success(t *testing.T) {
	r, s := newRouter()
	tenant, err := s.Create(context.Background(), "sus-co", "Sus Co")
	require.NoError(t, err)

	req := adminReq(http.MethodPost, "/tenants/"+tenant.ID+"/suspend", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSuspendTenant_NotFound(t *testing.T) {
	r, _ := newRouter()
	req := adminReq(http.MethodPost, "/tenants/ghost/suspend", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestResumeTenant_Success(t *testing.T) {
	r, s := newRouter()
	ctx := context.Background()
	tenant, err := s.Create(ctx, "res-co", "Res Co")
	require.NoError(t, err)
	_, _ = s.SetState(ctx, tenant.ID, store.TenantStateSuspended)

	req := adminReq(http.MethodPost, "/tenants/"+tenant.ID+"/resume", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestIssueToken_MissingFields(t *testing.T) {
	r, _ := newRouter()

	// Missing user_id
	req := adminReq(http.MethodPost, "/tokens", bodyJSON(map[string]any{
		"tenant_id": "some-tenant",
	}))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestIssueToken_TenantNotFound(t *testing.T) {
	r, _ := newRouter()
	req := adminReq(http.MethodPost, "/tokens", bodyJSON(map[string]any{
		"tenant_id": "nonexistent-id",
		"user_id":   "usr-1",
	}))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestIssueToken_DefaultViewerRole(t *testing.T) {
	r, s := newRouter()
	tenant, err := s.Create(context.Background(), "viewer-co", "Viewer Co")
	require.NoError(t, err)

	// No roles specified — should default to viewer
	req := adminReq(http.MethodPost, "/tokens", bodyJSON(map[string]any{
		"tenant_id": tenant.ID,
		"user_id":   "usr-viewer",
	}))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAdminSecretNotConfigured(t *testing.T) {
	s := store.NewMemStore()
	// Pass empty admin secret
	h := handler.New(s, []byte(testSecret), "", time.Hour, zap.NewNop())
	r := chi.NewRouter()
	h.Register(r)

	req := httptest.NewRequest(http.MethodGet, "/tenants", nil)
	req.Header.Set("X-Admin-Secret", "anything")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestCreateTenant_InvalidBody(t *testing.T) {
	r, _ := newRouter()
	req := httptest.NewRequest(http.MethodPost, "/tenants", bytes.NewBufferString("{bad json}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Secret", testAdminSecret)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateTenant_EmptyName(t *testing.T) {
	r, _ := newRouter()
	req := adminReq(http.MethodPost, "/tenants", bodyJSON(map[string]string{"slug": "ok-slug", "name": ""}))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
