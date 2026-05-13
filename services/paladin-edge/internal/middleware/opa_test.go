package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/auth"
	"github.com/paladinai/paladinai/services/paladin-edge/internal/middleware"
)

// fakeOPAClient implements OPAClient for tests.
type fakeOPAClient struct {
	allow bool
	err   error
}

func (f *fakeOPAClient) Allow(_ context.Context, _ middleware.OPAInput) (bool, error) {
	return f.allow, f.err
}

func applyOPA(client middleware.OPAClient, failOpen bool, h http.Handler) http.Handler {
	return middleware.OPAMiddleware(client, failOpen, zap.NewNop())(h)
}

func ctxWithTenant(tenantID string, roles ...string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	ctx := auth.WithTenantID(req.Context(), tenantID)
	ctx = auth.WithRoles(ctx, roles)
	return req.WithContext(ctx)
}

func TestOPAMiddleware_AllowsWhenOPAReturnsTrue(t *testing.T) {
	t.Parallel()
	client := &fakeOPAClient{allow: true}
	rr := httptest.NewRecorder()
	applyOPA(client, false, http.HandlerFunc(handler200)).ServeHTTP(rr, ctxWithTenant("t1", "viewer"))
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOPAMiddleware_DeniesWhenOPAReturnsFalse(t *testing.T) {
	t.Parallel()
	client := &fakeOPAClient{allow: false}
	rr := httptest.NewRecorder()
	applyOPA(client, false, http.HandlerFunc(handler200)).ServeHTTP(rr, ctxWithTenant("t1", "viewer"))
	assert.Equal(t, http.StatusForbidden, rr.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "access denied", errObj["message"])
}

func TestOPAMiddleware_FailOpen_ContinuesOnError(t *testing.T) {
	t.Parallel()
	client := &fakeOPAClient{err: context.DeadlineExceeded}
	rr := httptest.NewRecorder()
	applyOPA(client, true, http.HandlerFunc(handler200)).ServeHTTP(rr, ctxWithTenant("t1", "viewer"))
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOPAMiddleware_FailClosed_DeniesOnError(t *testing.T) {
	t.Parallel()
	client := &fakeOPAClient{err: context.DeadlineExceeded}
	rr := httptest.NewRecorder()
	applyOPA(client, false, http.HandlerFunc(handler200)).ServeHTTP(rr, ctxWithTenant("t1", "viewer"))
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestNewHTTPOPAClient_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	c := middleware.NewHTTPOPAClient("http://opa:8181")
	assert.NotNil(t, c)
}
