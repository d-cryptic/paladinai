package dedup_test

import (
	"context"
	"sync"
	"time"
)

// memStore is a plain in-memory Store stub for unit tests.
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

func (m *memStore) SetNX(_ context.Context, key, value string, expiration time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if e, ok := m.entries[key]; ok && time.Now().Before(e.expiry) {
		return false, nil // key exists and not expired → NX fails
	}

	m.entries[key] = memEntry{value: value, expiry: time.Now().Add(expiration)}
	return true, nil
}

func (m *memStore) Del(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.entries, k)
	}
	return nil
}
