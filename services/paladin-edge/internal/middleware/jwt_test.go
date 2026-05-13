package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/auth"
	"github.com/paladinai/paladinai/services/paladin-edge/internal/middleware"
)

var testSecret = []byte("test-secret-32-bytes-long-enough!")

func newToken(t *testing.T, tenantID, userID string, roles []string, ttl time.Duration) string {
	t.Helper()
	tok, err := auth.IssueToken(testSecret, tenantID, userID, roles, nil, ttl)
	require.NoError(t, err)
	return tok
}

func handler200(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

func applyJWT(next http.HandlerFunc) http.Handler {
	return middleware.JWTMiddleware(testSecret, zap.NewNop())(next)
}

func TestJWTMiddleware_Valid(t *testing.T) {
	tok := newToken(t, "tenant-abc", "usr-1", []string{"admin"}, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	applyJWT(handler200).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestJWTMiddleware_NoToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	applyJWT(handler200).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTMiddleware_Expired(t *testing.T) {
	tok := newToken(t, "tenant-abc", "usr-1", []string{"admin"}, -time.Second)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	applyJWT(handler200).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTMiddleware_WrongSecret(t *testing.T) {
	tok, err := auth.IssueToken([]byte("other-secret-32-bytes-padding!!x"), "t1", "u1", nil, nil, time.Hour)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	applyJWT(handler200).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTMiddleware_TenantIDInjectedInContext(t *testing.T) {
	tok := newToken(t, "tenant-xyz", "usr-2", []string{"viewer"}, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)

	var gotTenant string
	check := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotTenant, _ = auth.TenantIDFromContext(r.Context())
	})
	rr := httptest.NewRecorder()
	middleware.JWTMiddleware(testSecret, zap.NewNop())(check).ServeHTTP(rr, req)
	assert.Equal(t, "tenant-xyz", gotTenant)
}

func TestJWTMiddleware_XTenantMismatch(t *testing.T) {
	tok := newToken(t, "tenant-abc", "usr-1", []string{"admin"}, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Tenant-ID", "different-tenant")
	rr := httptest.NewRecorder()
	applyJWT(handler200).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestJWTMiddleware_BearerCaseInsensitive(t *testing.T) {
	tok := newToken(t, "tenant-abc", "usr-1", []string{"admin"}, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "BEARER "+tok)
	rr := httptest.NewRecorder()
	applyJWT(handler200).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestJWTMiddleware_MalformedHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "notbearer xyz")
	rr := httptest.NewRecorder()
	applyJWT(handler200).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
