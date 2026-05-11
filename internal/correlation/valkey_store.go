package correlation

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient is the minimal interface over go-redis used by ValkeyStore.
// Satisfied by *redis.Client; extracted so unit tests can inject fakes without
// a real Redis/Valkey server.
type RedisClient interface {
	SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd
	Get(ctx context.Context, key string) *redis.StringCmd
}

// ValkeyStore adapts go-redis to the correlation Store interface.
type ValkeyStore struct {
	rdb RedisClient
}

// NewValkeyStore wraps a go-redis Client for use as a correlation Store.
func NewValkeyStore(rdb *redis.Client) *ValkeyStore {
	return &ValkeyStore{rdb: rdb}
}

// NewValkeyStoreFromClient wraps any RedisClient implementation.
// Used in unit tests to inject in-memory fakes without a real server.
func NewValkeyStoreFromClient(rdb RedisClient) *ValkeyStore {
	return &ValkeyStore{rdb: rdb}
}

// GetOrSet atomically returns the stored value for key, or sets it to defaultValue with ttl.
// Uses SetNX + Get. A narrow race window (key expires between SetNX and Get) is handled
// by a single retry; if still lost, the actual winner's value is returned.
func (s *ValkeyStore) GetOrSet(ctx context.Context, key, defaultValue string, ttl time.Duration) (string, bool, error) {
	ok, err := s.rdb.SetNX(ctx, key, defaultValue, ttl).Result()
	if err != nil {
		return "", false, err
	}
	if ok {
		return defaultValue, true, nil
	}

	// Key existed — return the authoritative stored value.
	existing, err := s.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		// Race: key expired between SetNX and Get — retry once.
		ok2, err2 := s.rdb.SetNX(ctx, key, defaultValue, ttl).Result()
		if err2 != nil {
			return "", false, err2
		}
		if ok2 {
			return defaultValue, true, nil
		}
		// Another writer won the second race — fetch what they stored.
		actual, err3 := s.rdb.Get(ctx, key).Result()
		if err3 != nil {
			return "", false, err3
		}
		return actual, false, nil
	}
	if err != nil {
		return "", false, err
	}
	return existing, false, nil
}
