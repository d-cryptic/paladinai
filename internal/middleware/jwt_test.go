package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/auth"
	"github.com/paladinai/paladinai/internal/middleware"
)

const testSecret = "test-secret-32-bytes-for-hs256!!"

func issueToken(t *testing.T, tenantID, userID string, roles []string, ttl time.Duration) string {
	t.Helper()
	tok, err := auth.IssueToken([]byte(testSecret), tenantID, userID, roles, ttl)
	require.NoError(t, err)
	return tok
}

func newMiddleware() func(http.Handler) http.Handler {
	return middleware.JWTMiddleware([]byte(testSecret), zap.NewNop())
}

func serve(mw func(http.Handler) http.Handler, req *http.Request) *httptest.ResponseRecorder {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)
	return rr
}

func TestJWTMiddleware_ValidToken_Passes(t *testing.T) {
	tok := issueToken(t, "acme-corp", "user-1", []string{"admin"}, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)

	rr := serve(newMiddleware(), req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestJWTMiddleware_ValidToken_InjectsContext(t *testing.T) {
	tok := issueToken(t, "acme-corp", "user-1", []string{"viewer"}, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)

	var gotTenant, gotUser string
	var gotRoles []string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant, _ = auth.TenantIDFromContext(r.Context())
		gotUser, _ = auth.UserIDFromContext(r.Context())
		gotRoles = auth.RolesFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	newMiddleware()(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "acme-corp", gotTenant)
	assert.Equal(t, "user-1", gotUser)
	assert.Equal(t, []string{"viewer"}, gotRoles)
}

func TestJWTMiddleware_MissingAuthHeader_Returns401(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := serve(newMiddleware(), req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTMiddleware_MalformedAuthHeader_Returns401(t *testing.T) {
	for _, hdr := range []string{"Basic dXNlcjpwYXNz", "Bearer", " ", "token-without-bearer"} {
		hdr := hdr
		t.Run(hdr, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", hdr)
			rr := serve(newMiddleware(), req)
			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}
}

func TestJWTMiddleware_ExpiredToken_Returns401(t *testing.T) {
	tok := issueToken(t, "t1", "u1", nil, -time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := serve(newMiddleware(), req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTMiddleware_WrongSecret_Returns401(t *testing.T) {
	other := strings.Repeat("z", 32)
	tok, err := auth.IssueToken([]byte(other), "t1", "u1", nil, time.Hour)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := serve(newMiddleware(), req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTMiddleware_TenantIDHeaderMatches_Passes(t *testing.T) {
	tok := issueToken(t, "acme-corp", "u1", nil, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Tenant-ID", "acme-corp") // matches token claim

	rr := serve(newMiddleware(), req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestJWTMiddleware_TenantIDHeaderMismatch_Returns403(t *testing.T) {
	tok := issueToken(t, "acme-corp", "u1", nil, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Tenant-ID", "evil-corp") // does not match token claim

	rr := serve(newMiddleware(), req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
	// Response must not leak the real tenant to the attacker.
	assert.NotContains(t, rr.Body.String(), "acme-corp")
}

func TestJWTMiddleware_CaseInsensitiveBearer(t *testing.T) {
	tok := issueToken(t, "t1", "u1", nil, time.Hour)

	for _, prefix := range []string{"Bearer", "bearer", "BEARER", "BeArEr"} {
		prefix := prefix
		t.Run("valid_"+prefix, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", prefix+" "+tok)
			rr := serve(newMiddleware(), req)
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}

	// Negative: scheme variants that must NOT be accepted.
	for _, hdr := range []string{
		"Bearer" + tok, // no space between scheme and token
		"Bear " + tok,  // wrong scheme name
	} {
		hdr := hdr
		t.Run("invalid_"+hdr[:5], func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", hdr)
			rr := serve(newMiddleware(), req)
			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}
}
