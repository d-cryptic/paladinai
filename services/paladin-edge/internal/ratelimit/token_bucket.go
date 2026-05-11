// Package ratelimit implements a per-tenant token-bucket rate limiter.
// Backed by Valkey (Redis-compatible) using fixed-window INCR+EXPIRE.
// Fails open on Valkey errors to avoid blocking real traffic.
package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Store is the minimal interface for the rate-limit counter.
// Decoupled from go-redis for testability.
type Store interface {
	IncrWithExpire(key string, window time.Duration) (count int, err error)
}

// Limiter is the rate-limiting interface consumed by HTTP middleware.
type Limiter interface {
	Allow(ctx context.Context, tenantID string) (allowed bool, remaining int, resetAt time.Time, err error)
	// Limit returns the configured maximum requests per window for header reporting.
	Limit() int
}

// ValkeyLimiter is a fixed-window rate limiter backed by Valkey.
type ValkeyLimiter struct {
	store  Store
	limit  int // max requests per window
	window time.Duration
	log    *zap.Logger
}

// NewValkeyLimiter creates a limiter with the given default limit.
func NewValkeyLimiter(rdb *redis.Client, limit int, log *zap.Logger) *ValkeyLimiter {
	return &ValkeyLimiter{
		store:  &valkeyStore{rdb: rdb},
		limit:  limit,
		window: time.Minute,
		log:    log,
	}
}

// NewTestLimiter creates a ValkeyLimiter using the provided Store (for tests).
func NewTestLimiter(store Store, limit int) *ValkeyLimiter {
	return &ValkeyLimiter{store: store, limit: limit, window: time.Minute, log: zap.NewNop()}
}

// Limit returns the configured max requests per window (used for X-RateLimit-Limit header).
func (l *ValkeyLimiter) Limit() int { return l.limit }

func (l *ValkeyLimiter) Allow(ctx context.Context, tenantID string) (bool, int, time.Time, error) {
	key := fmt.Sprintf("paladin:ratelimit:%s", tenantID)
	now := time.Now()
	resetAt := now.Truncate(l.window).Add(l.window)

	count, err := l.store.IncrWithExpire(key, l.window)
	if err != nil {
		// Fail open — don't block real traffic on store errors.
		l.log.Error("rate limit store error, failing open",
			zap.String("tenant", tenantID),
			zap.Error(err),
		)
		return true, l.limit, resetAt, nil
	}

	remaining := l.limit - count
	if remaining < 0 {
		remaining = 0
	}
	return count <= l.limit, remaining, resetAt, nil
}

// valkeyStore wraps go-redis for rate-limit counting.
type valkeyStore struct {
	rdb *redis.Client
}

func (s *valkeyStore) IncrWithExpire(key string, window time.Duration) (int, error) {
	ctx := context.Background()
	pipe := s.rdb.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return int(incrCmd.Val()), nil
}

// Middleware returns an HTTP middleware that enforces rate limits per tenant.
// The tenant ID is read from the X-Tenant-ID header (set by the auth layer upstream).
func Middleware(limiter Limiter, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := r.Header.Get("X-Tenant-ID")
			if tenantID == "" {
				next.ServeHTTP(w, r)
				return
			}

			allowed, remaining, resetAt, err := limiter.Allow(r.Context(), tenantID)
			if err != nil {
				log.Error("rate limiter error", zap.Error(err))
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limiter.Limit()))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))

			if !allowed {
				w.Header().Set("Retry-After", strconv.FormatInt(int64(time.Until(resetAt).Seconds()), 10))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprintf(w, `{"error":{"code":"RATE_LIMIT_EXCEEDED","message":"rate limit exceeded, retry after %s"}}`,
					resetAt.UTC().Format(time.RFC3339))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
