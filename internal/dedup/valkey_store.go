package dedup

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// ValkeyStore adapts go-redis Client to the Store interface.
type ValkeyStore struct {
	rdb *redis.Client
}

// NewValkeyStore wraps a go-redis Client.
func NewValkeyStore(rdb *redis.Client) *ValkeyStore {
	return &ValkeyStore{rdb: rdb}
}

func (s *ValkeyStore) SetNX(ctx context.Context, key, value string, expiration time.Duration) (bool, error) {
	return s.rdb.SetNX(ctx, key, value, expiration).Result()
}

func (s *ValkeyStore) Del(ctx context.Context, keys ...string) error {
	return s.rdb.Del(ctx, keys...).Err()
}
