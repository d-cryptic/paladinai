// Package store implements memory backends for the paladin-memory service.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// WorkingStore is the contract for session-scoped, ephemeral working memory.
// Implementations may be backed by Valkey/Redis (prod) or in-memory (tests).
type WorkingStore interface {
	Get(ctx context.Context, tenantID, sessionID, key string) (string, bool, error)
	Set(ctx context.Context, tenantID, sessionID, key, value string, ttl time.Duration) error
	// Scan returns all values whose key starts with the given prefix for the
	// (tenantID, sessionID). Used to support SearchMemory over working memory.
	Scan(ctx context.Context, tenantID, sessionID, prefix string, limit int) ([]string, error)
}

// RedisWorkingStore is a Valkey/Redis-backed WorkingStore.
type RedisWorkingStore struct {
	client redis.UniversalClient
}

// NewRedisWorkingStore wraps an existing redis client.
func NewRedisWorkingStore(client redis.UniversalClient) *RedisWorkingStore {
	return &RedisWorkingStore{client: client}
}

// Key format: wm:{tenant_id}:{session_id}:{key}
func workingKey(tenantID, sessionID, key string) string {
	return fmt.Sprintf("wm:%s:%s:%s", tenantID, sessionID, key)
}

func workingPrefix(tenantID, sessionID, prefix string) string {
	return fmt.Sprintf("wm:%s:%s:%s", tenantID, sessionID, prefix)
}

// Get returns the value at the given key. The bool is false when the key is absent.
func (s *RedisWorkingStore) Get(ctx context.Context, tenantID, sessionID, key string) (string, bool, error) {
	v, err := s.client.Get(ctx, workingKey(tenantID, sessionID, key)).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("memory: working get: %w", err)
	}
	return v, true, nil
}

// Set writes the value with the given TTL.
func (s *RedisWorkingStore) Set(ctx context.Context, tenantID, sessionID, key, value string, ttl time.Duration) error {
	if err := s.client.Set(ctx, workingKey(tenantID, sessionID, key), value, ttl).Err(); err != nil {
		return fmt.Errorf("memory: working set: %w", err)
	}
	return nil
}

// Scan returns up to `limit` values whose key begins with the given prefix.
// Uses SCAN to avoid blocking on large keyspaces.
func (s *RedisWorkingStore) Scan(ctx context.Context, tenantID, sessionID, prefix string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 20
	}
	pattern := workingPrefix(tenantID, sessionID, prefix) + "*"
	var (
		cursor uint64
		out    []string
	)
	for {
		keys, next, err := s.client.Scan(ctx, cursor, pattern, int64(limit)).Result()
		if err != nil {
			return nil, fmt.Errorf("memory: working scan: %w", err)
		}
		for _, k := range keys {
			v, err := s.client.Get(ctx, k).Result()
			if errors.Is(err, redis.Nil) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("memory: working scan get: %w", err)
			}
			out = append(out, v)
			if len(out) >= limit {
				return out, nil
			}
		}
		if next == 0 {
			break
		}
		cursor = next
	}
	return out, nil
}
