package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/auth"
	"github.com/paladinai/paladinai/services/paladin-edge/internal/middleware"
)

// ─── RequestID middleware ─────────────────────────────────────────────────────

func TestRequestID_GeneratesIDWhenAbsent(t *testing.T) {
	var capturedID string
	h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = middleware.GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.NotEmpty(t, capturedID, "a UUID should be generated when no X-Request-ID is present")
	assert.Equal(t, capturedID, rr.Header().Get("X-Request-ID"), "response header must match context value")
}

func TestRequestID_PropagatesExistingHeader(t *testing.T) {
	const fixedID = "my-request-id-123"

	var capturedID string
	h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = middleware.GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", fixedID)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, fixedID, capturedID, "existing X-Request-ID must be preserved in context")
	assert.Equal(t, fixedID, rr.Header().Get("X-Request-ID"), "existing X-Request-ID must be reflected in response")
}

func TestGetRequestID_MissingContext_ReturnsEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.Empty(t, middleware.GetRequestID(req.Context()))
}

// ─── JWT middleware edge cases ────────────────────────────────────────────────

func TestJWTMiddleware_EmptyBearerToken_Returns401(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer   ") // whitespace only
	rr := httptest.NewRecorder()
	applyJWT(handler200).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTMiddleware_XTenantMatchesClaim_Passes(t *testing.T) {
	tok := newToken(t, "tenant-abc", "usr-1", []string{"admin"}, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Tenant-ID", "tenant-abc") // matches claim
	rr := httptest.NewRecorder()
	applyJWT(handler200).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestJWTMiddleware_RolesInjectedInContext(t *testing.T) {
	tok := newToken(t, "tenant-abc", "usr-1", []string{"admin", "viewer"}, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)

	var gotRoles []string
	check := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotRoles = auth.RolesFromContext(r.Context())
	})
	rr := httptest.NewRecorder()
	middleware.JWTMiddleware(testSecret, zap.NewNop())(check).ServeHTTP(rr, req)
	assert.ElementsMatch(t, []string{"admin", "viewer"}, gotRoles)
}

func TestJWTMiddleware_UserIDInjectedInContext(t *testing.T) {
	tok := newToken(t, "tenant-abc", "usr-99", []string{"admin"}, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)

	var gotUserID string
	check := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotUserID, _ = auth.UserIDFromContext(r.Context())
	})
	rr := httptest.NewRecorder()
	middleware.JWTMiddleware(testSecret, zap.NewNop())(check).ServeHTTP(rr, req)
	assert.Equal(t, "usr-99", gotUserID)
}

func TestTenantIDFromContext_ReturnsInjectedValue(t *testing.T) {
	tok := newToken(t, "tenant-mw", "usr-1", nil, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)

	var tid string
	var ok bool
	check := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		tid, ok = middleware.TenantIDFromContext(r)
	})
	rr := httptest.NewRecorder()
	middleware.JWTMiddleware(testSecret, zap.NewNop())(check).ServeHTTP(rr, req)
	assert.True(t, ok)
	assert.Equal(t, "tenant-mw", tid)
}
