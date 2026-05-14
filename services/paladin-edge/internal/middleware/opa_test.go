package middleware_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/auth"
	"github.com/paladinai/paladinai/services/paladin-edge/internal/middleware"
)

// capturingOPAClient records the OPAInput it was called with, allowing
// tests to assert on the action derived from the HTTP method.
type capturingOPAClient struct {
	allow bool
	err   error
	last  middleware.OPAInput
}

func (c *capturingOPAClient) Allow(_ context.Context, input middleware.OPAInput) (bool, error) {
	c.last = input
	return c.allow, c.err
}

func opaRequestWithMethod(method string) *http.Request {
	req := httptest.NewRequest(method, "/api/v1/incidents", nil)
	ctx := auth.WithTenantID(req.Context(), "t1")
	ctx = auth.WithRoles(ctx, []string{"operator"})
	return req.WithContext(ctx)
}

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

func TestOPAMiddleware_HTTPMethodToAction(t *testing.T) {
	t.Parallel()
	cases := []struct {
		method string
		want   string
	}{
		{http.MethodGet, "incidents:read"},
		{http.MethodHead, "incidents:read"},
		{http.MethodPost, "incidents:update"},
		{http.MethodPut, "incidents:update"},
		{http.MethodPatch, "incidents:update"},
		{http.MethodDelete, "incidents:delete"},
		{http.MethodOptions, "incidents:read"}, // default branch
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.method, func(t *testing.T) {
			t.Parallel()
			client := &capturingOPAClient{allow: true}
			rr := httptest.NewRecorder()
			applyOPA(client, false, http.HandlerFunc(handler200)).ServeHTTP(rr, opaRequestWithMethod(tc.method))
			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, tc.want, client.last.Action)
		})
	}
}

func TestHTTPOPAClient_Allow_ReturnsTrue(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/data/paladin/authz/allow", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":true}`)
	}))
	defer srv.Close()

	client := middleware.NewHTTPOPAClient(srv.URL)
	allowed, err := client.Allow(context.Background(), middleware.OPAInput{
		Token:    middleware.OPAToken{TenantID: "t1", Roles: []string{"viewer"}},
		Action:   "incidents:read",
		Resource: middleware.OPAResource{TenantID: "t1"},
	})
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestHTTPOPAClient_Allow_ReturnsFalse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":false}`)
	}))
	defer srv.Close()

	client := middleware.NewHTTPOPAClient(srv.URL)
	allowed, err := client.Allow(context.Background(), middleware.OPAInput{
		Token:    middleware.OPAToken{TenantID: "t1"},
		Action:   "incidents:delete",
		Resource: middleware.OPAResource{TenantID: "t2"},
	})
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestHTTPOPAClient_Allow_NonOKStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := middleware.NewHTTPOPAClient(srv.URL)
	_, err := client.Allow(context.Background(), middleware.OPAInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestHTTPOPAClient_Allow_MalformedJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":`) // truncated
	}))
	defer srv.Close()

	client := middleware.NewHTTPOPAClient(srv.URL)
	_, err := client.Allow(context.Background(), middleware.OPAInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

func TestHTTPOPAClient_Allow_RejectsOversizedResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, strings.Repeat("x", 64*1024+1))
	}))
	defer srv.Close()

	client := middleware.NewHTTPOPAClient(srv.URL)
	_, err := client.Allow(context.Background(), middleware.OPAInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "response body exceeds")
}

func TestHTTPOPAClient_Allow_UnreachableServer(t *testing.T) {
	t.Parallel()
	// Build a client against an unroutable address; expect a network error.
	client := middleware.NewHTTPOPAClient("http://127.0.0.1:1")
	_, err := client.Allow(context.Background(), middleware.OPAInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "opa: query")
}
