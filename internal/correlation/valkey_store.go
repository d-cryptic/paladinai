package correlation

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// ValkeyStore adapts go-redis to the correlation Store interface.
type ValkeyStore struct {
	rdb *redis.Client
}

// NewValkeyStore wraps a go-redis Client for use as a correlation Store.
func NewValkeyStore(rdb *redis.Client) *ValkeyStore {
	return &ValkeyStore{rdb: rdb}
}

// GetOrSet returns the stored value for key, or atomically sets it to defaultValue with the given TTL.
// Returns (value, wasNew, err). Uses SET NX + GET to emulate GetOrSet atomically.
func (s *ValkeyStore) GetOrSet(ctx context.Context, key, defaultValue string, ttl time.Duration) (string, bool, error) {
	// Try to set the key only if it doesn't exist
	ok, err := s.rdb.SetNX(ctx, key, defaultValue, ttl).Result()
	if err != nil {
		return "", false, err
	}
	if ok {
		return defaultValue, true, nil
	}

	// Key existed — return the current value
	existing, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		// Raced: key expired between SetNX and Get — retry once as new
		ok2, err2 := s.rdb.SetNX(ctx, key, defaultValue, ttl).Result()
		if err2 != nil {
			return "", false, err2
		}
		return defaultValue, ok2, nil
	}
	if err != nil {
		return "", false, err
	}
	return existing, false, nil
}
