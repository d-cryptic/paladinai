package ratelimit_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/auth"
	"github.com/paladinai/paladinai/services/paladin-edge/internal/ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ─── Fake Limiter ─────────────────────────────────────────────────────────────

type fakeLimiter struct {
	limit        int
	allowed      bool
	remaining    int
	resetAt      time.Time
	err          error
	lastTenantID string
}

func (f *fakeLimiter) Limit() int { return f.limit }

func (f *fakeLimiter) Allow(_ context.Context, tenantID string) (bool, int, time.Time, error) {
	f.lastTenantID = tenantID
	return f.allowed, f.remaining, f.resetAt, f.err
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func newRequest(tenantID string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	if tenantID != "" {
		r.Header.Set("X-Tenant-ID", tenantID)
	}
	return r
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestMiddleware_NoTenantHeader_PassesThrough(t *testing.T) {
	limiter := &fakeLimiter{limit: 60, allowed: false} // even a denying limiter must not block without header
	mw := ratelimit.Middleware(limiter, zap.NewNop())

	rr := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rr, newRequest(""))

	assert.Equal(t, http.StatusOK, rr.Code, "missing X-Tenant-ID must bypass rate limiting")
	assert.Empty(t, rr.Header().Get("X-RateLimit-Remaining"))
}

func TestMiddleware_UnderLimit_PassesThrough(t *testing.T) {
	resetAt := time.Now().Add(30 * time.Second)
	limiter := &fakeLimiter{limit: 60, allowed: true, remaining: 55, resetAt: resetAt}
	mw := ratelimit.Middleware(limiter, zap.NewNop())

	rr := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rr, newRequest("tenant-1"))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "60", rr.Header().Get("X-RateLimit-Limit"), "limit header must reflect configured limit")
	assert.Equal(t, "55", rr.Header().Get("X-RateLimit-Remaining"))
	assert.Equal(t, strconv.FormatInt(resetAt.Unix(), 10), rr.Header().Get("X-RateLimit-Reset"))
}

func TestMiddleware_ContextTenantOverridesSpoofedHeader(t *testing.T) {
	resetAt := time.Now().Add(30 * time.Second)
	limiter := &fakeLimiter{limit: 60, allowed: true, remaining: 55, resetAt: resetAt}
	mw := ratelimit.Middleware(limiter, zap.NewNop())

	req := newRequest("evil-tenant")
	req = req.WithContext(auth.WithTenantID(req.Context(), "token-tenant"))

	rr := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "token-tenant", limiter.lastTenantID)
}

func TestMiddleware_OverLimit_Returns429(t *testing.T) {
	resetAt := time.Now().Add(45 * time.Second)
	limiter := &fakeLimiter{limit: 60, allowed: false, remaining: 0, resetAt: resetAt}
	mw := ratelimit.Middleware(limiter, zap.NewNop())

	rr := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rr, newRequest("tenant-2"))

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "0", rr.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, rr.Header().Get("Retry-After"), "Retry-After header must be set on 429")
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "RATE_LIMIT_EXCEEDED", errObj["code"])
}

func TestMiddleware_OverLimit_NextHandlerNotCalled(t *testing.T) {
	limiter := &fakeLimiter{limit: 60, allowed: false, remaining: 0, resetAt: time.Now().Add(time.Minute)}
	mw := ratelimit.Middleware(limiter, zap.NewNop())

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, newRequest("tenant-3"))

	assert.False(t, nextCalled, "next handler must not be called when rate-limited")
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestMiddleware_LimiterError_LogsAndPassesThrough(t *testing.T) {
	// ValkeyLimiter.Allow swallows store errors and returns (true, limit, resetAt, nil).
	// Middleware trusts whatever Allow returns. This test simulates that fail-open
	// behaviour: err is non-nil but allowed=true, so the request must pass through.
	limiter := &fakeLimiter{limit: 60, err: errors.New("valkey down"), allowed: true, remaining: 60, resetAt: time.Now().Add(time.Minute)}
	mw := ratelimit.Middleware(limiter, zap.NewNop())

	rr := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rr, newRequest("tenant-4"))

	assert.Equal(t, http.StatusOK, rr.Code, "fail-open: request must pass through when limiter returns allowed=true")
}

func TestMiddleware_RetryAfterIsPositive(t *testing.T) {
	resetAt := time.Now().Add(90 * time.Second)
	limiter := &fakeLimiter{limit: 60, allowed: false, remaining: 0, resetAt: resetAt}
	mw := ratelimit.Middleware(limiter, zap.NewNop())

	rr := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rr, newRequest("tenant-5"))

	retryAfterStr := rr.Header().Get("Retry-After")
	require.NotEmpty(t, retryAfterStr)
	retryAfter, err := strconv.ParseInt(retryAfterStr, 10, 64)
	require.NoError(t, err)
	assert.Positive(t, retryAfter, "Retry-After must be a positive number of seconds")
}

func TestMiddleware_HeadersAlwaysSetBeforeDecision(t *testing.T) {
	resetAt := time.Now().Add(30 * time.Second)
	limiter := &fakeLimiter{limit: 60, allowed: false, remaining: 0, resetAt: resetAt}
	mw := ratelimit.Middleware(limiter, zap.NewNop())

	rr := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rr, newRequest("tenant-6"))

	// Rate limit headers must be present even on 429 (needed by API consumers).
	assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Limit"))
	assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Reset"))
}

func TestMiddleware_LimitHeaderReflectsConfiguredLimit(t *testing.T) {
	// Use a non-default limit to prove the header isn't hardcoded.
	limiter := &fakeLimiter{limit: 600, allowed: true, remaining: 598, resetAt: time.Now().Add(time.Minute)}
	mw := ratelimit.Middleware(limiter, zap.NewNop())

	rr := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rr, newRequest("tenant-premium"))

	assert.Equal(t, "600", rr.Header().Get("X-RateLimit-Limit"), "header must reflect actual configured limit, not hardcoded 60")
}

// ─── Store error path on Allow ─────────────────────────────────────────────

type errLimitStore struct{ err error }

func (e *errLimitStore) IncrWithExpire(_ context.Context, _ string, _ time.Duration) (int, error) {
	return 0, e.err
}

func TestValkeyLimiter_StoreError_FailsOpen(t *testing.T) {
	store := &errLimitStore{err: errors.New("valkey: timeout")}
	limiter := ratelimit.NewTestLimiter(store, 10)

	allowed, remaining, _, err := limiter.Allow(context.Background(), "tenant-err")
	require.NoError(t, err, "fail-open means error must be swallowed and nil returned")
	assert.True(t, allowed, "fail-open must allow the request")
	assert.Equal(t, 10, remaining, "remaining should be limit when failing open")
}

func TestValkeyLimiter_ZeroLimit_AlwaysBlocked(t *testing.T) {
	store := newMemLimitStore()
	limiter := ratelimit.NewTestLimiter(store, 0)

	allowed, remaining, _, err := limiter.Allow(context.Background(), "tenant-zero")
	require.NoError(t, err)
	assert.False(t, allowed, "zero limit should block all requests")
	assert.Equal(t, 0, remaining)
}
