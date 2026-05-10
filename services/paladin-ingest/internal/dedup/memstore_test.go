package dedup_test

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// memStore is an in-memory redis.SetNX/Del stub for unit tests.
// No networking — safe to run in sandbox environments.
type memStore struct {
	mu      sync.Mutex
	entries map[string]memEntry
}

type memEntry struct {
	value  string
	expiry time.Time
}

func newMemStore() *memStore {
	return &memStore{entries: make(map[string]memEntry)}
}

func (m *memStore) SetNX(_ context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := redis.NewBoolCmd(context.Background())

	if e, ok := m.entries[key]; ok && time.Now().Before(e.expiry) {
		cmd.SetVal(false) // key exists and not expired → NX fails
		return cmd
	}

	val := ""
	if s, ok := value.(string); ok {
		val = s
	}
	m.entries[key] = memEntry{value: val, expiry: time.Now().Add(expiration)}
	cmd.SetVal(true)
	return cmd
}

func (m *memStore) Del(_ context.Context, keys ...string) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := redis.NewIntCmd(context.Background())
	var deleted int64
	for _, k := range keys {
		if _, ok := m.entries[k]; ok {
			delete(m.entries, k)
			deleted++
		}
	}
	cmd.SetVal(deleted)
	return cmd
}
