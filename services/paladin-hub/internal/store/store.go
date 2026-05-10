// Package store provides the persistence interface and in-memory implementation
// for the MCP server registry. The interface is backend-agnostic; production
// will use Postgres via pgx.
package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/paladinai/paladinai/services/paladin-hub/internal/registry"
)

// ErrNotFound is returned when a server is not in the store.
var ErrNotFound = fmt.Errorf("mcp server not found")

// Store persists and retrieves MCPServer records.
type Store interface {
	// Upsert inserts or replaces a server record.
	Upsert(ctx context.Context, s *registry.MCPServer) error

	// Get returns a server by tenantID + serverID.
	Get(ctx context.Context, tenantID, serverID string) (*registry.MCPServer, error)

	// List returns all servers for a tenant.
	List(ctx context.Context, tenantID string) ([]*registry.MCPServer, error)

	// Delete removes a server; returns ErrNotFound if absent.
	Delete(ctx context.Context, tenantID, serverID string) error

	// Heartbeat updates LastSeenAt and Healthy for a server.
	Heartbeat(ctx context.Context, tenantID, serverID string, at time.Time) error
}

// MemStore is a thread-safe in-memory Store. Used in unit tests and dev mode.
type MemStore struct {
	mu      sync.RWMutex
	servers map[string]*registry.MCPServer // key: tenantID+":"+serverID
}

// NewMemStore returns an empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{servers: make(map[string]*registry.MCPServer)}
}

func (m *MemStore) key(tenantID, serverID string) string {
	return tenantID + ":" + serverID
}

func (m *MemStore) Upsert(_ context.Context, s *registry.MCPServer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *s
	cp.Capabilities = make([]string, len(s.Capabilities))
	copy(cp.Capabilities, s.Capabilities)
	m.servers[m.key(s.TenantID, s.ID)] = &cp
	return nil
}

func (m *MemStore) Get(_ context.Context, tenantID, serverID string) (*registry.MCPServer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.servers[m.key(tenantID, serverID)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *s
	cp.Capabilities = append([]string{}, s.Capabilities...)
	return &cp, nil
}

func (m *MemStore) List(_ context.Context, tenantID string) ([]*registry.MCPServer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*registry.MCPServer
	prefix := tenantID + ":"
	for k, s := range m.servers {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			cp := *s
			cp.Capabilities = append([]string{}, s.Capabilities...)
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (m *MemStore) Delete(_ context.Context, tenantID, serverID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := m.key(tenantID, serverID)
	if _, ok := m.servers[k]; !ok {
		return ErrNotFound
	}
	delete(m.servers, k)
	return nil
}

func (m *MemStore) Heartbeat(_ context.Context, tenantID, serverID string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.servers[m.key(tenantID, serverID)]
	if !ok {
		return ErrNotFound
	}
	s.LastSeenAt = at
	s.Healthy = true
	return nil
}
