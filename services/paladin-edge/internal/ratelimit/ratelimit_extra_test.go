package ratelimit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/services/paladin-edge/internal/ratelimit"
)

func TestValkeyLimiter_Limit_Method(t *testing.T) {
	store := newMemLimitStore()
	limiter := ratelimit.NewTestLimiter(store, 42)
	assert.Equal(t, 42, limiter.Limit())
}

func TestValkeyLimiter_RemainingClampedAtZero(t *testing.T) {
	store := newMemLimitStore()
	limiter := ratelimit.NewTestLimiter(store, 1)
	ctx := context.Background()

	// Exhaust and exceed the limit.
	for i := 0; i < 3; i++ {
		_, _, _, err := limiter.Allow(ctx, "clamp-test")
		require.NoError(t, err)
	}

	_, remaining, _, err := limiter.Allow(ctx, "clamp-test")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, remaining, 0, "remaining must never be negative")
}

func TestValkeyLimiter_ResetAtIsInFuture(t *testing.T) {
	store := newMemLimitStore()
	limiter := ratelimit.NewTestLimiter(store, 10)

	_, _, resetAt, err := limiter.Allow(context.Background(), "reset-test")
	require.NoError(t, err)
	assert.True(t, resetAt.After(time.Now()), "resetAt must be in the future")
}

// TestMiddleware_LimiterErrorAndDenied exercises the branch where Allow returns
// err != nil AND allowed=false (e.g. a custom Limiter implementation that lets
// errors propagate as denials).
func TestMiddleware_LimiterErrorAndDenied_Returns429(t *testing.T) {
	limiter := &fakeLimiter{
		limit:     60,
		allowed:   false,
		remaining: 0,
		resetAt:   time.Now().Add(time.Minute),
		err:       assert.AnError,
	}
	mw := ratelimit.Middleware(limiter, zap.NewNop())

	rr := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rr, newRequest("tenant-deny-err"))

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestValkeyLimiter_MultipleTenantsCountSeparately(t *testing.T) {
	store := newMemLimitStore()
	limiter := ratelimit.NewTestLimiter(store, 5)
	ctx := context.Background()

	// 3 requests for tenant-X
	for i := 0; i < 3; i++ {
		_, _, _, err := limiter.Allow(ctx, "tx")
		require.NoError(t, err)
	}

	// tenant-Y should still have full quota
	allowed, remaining, _, err := limiter.Allow(ctx, "ty")
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 4, remaining)
}
