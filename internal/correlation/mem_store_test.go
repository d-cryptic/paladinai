package correlation_test

import (
	"context"
	"sync"
	"time"
)

// memStore is an in-memory GetOrSet store for unit tests.
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

func (m *memStore) GetOrSet(_ context.Context, key, defaultValue string, ttl time.Duration) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if e, ok := m.entries[key]; ok && time.Now().Before(e.expiry) {
		return e.value, false, nil
	}

	m.entries[key] = memEntry{value: defaultValue, expiry: time.Now().Add(ttl)}
	return defaultValue, true, nil
}
