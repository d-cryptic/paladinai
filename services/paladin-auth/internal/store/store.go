package store

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a tenant does not exist.
var ErrNotFound = errors.New("tenant not found")

// ErrAlreadyExists is returned when a tenant slug is already taken.
var ErrAlreadyExists = errors.New("tenant already exists")

// TenantState tracks lifecycle.
type TenantState string

const (
	TenantStateActive    TenantState = "active"
	TenantStateSuspended TenantState = "suspended"
)

// Tenant is the canonical tenant record.
type Tenant struct {
	ID        string      `json:"id"`
	Slug      string      `json:"slug"`
	Name      string      `json:"name"`
	State     TenantState `json:"state"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// Store is the narrow interface that paladin-auth uses for tenant persistence.
// Implementations: MemStore (tests + dev), PostgresStore (production).
type Store interface {
	Create(ctx context.Context, slug, name string) (*Tenant, error)
	Get(ctx context.Context, id string) (*Tenant, error)
	GetBySlug(ctx context.Context, slug string) (*Tenant, error)
	List(ctx context.Context) ([]*Tenant, error)
	SetState(ctx context.Context, id string, state TenantState) (*Tenant, error)
}

// MemStore is a thread-safe in-memory Store used for tests and local dev.
type MemStore struct {
	mu      sync.RWMutex
	byID    map[string]*Tenant
	bySlug  map[string]*Tenant
}

func NewMemStore() *MemStore {
	return &MemStore{
		byID:   make(map[string]*Tenant),
		bySlug: make(map[string]*Tenant),
	}
}

func (s *MemStore) Create(_ context.Context, slug, name string) (*Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.bySlug[slug]; ok {
		return nil, ErrAlreadyExists
	}

	now := time.Now().UTC()
	t := &Tenant{
		ID:        uuid.New().String(),
		Slug:      slug,
		Name:      name,
		State:     TenantStateActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.byID[t.ID] = t
	s.bySlug[t.Slug] = t
	return t, nil
}

func (s *MemStore) Get(_ context.Context, id string) (*Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *t
	return &cp, nil
}

func (s *MemStore) GetBySlug(_ context.Context, slug string) (*Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.bySlug[slug]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *t
	return &cp, nil
}

func (s *MemStore) List(_ context.Context) ([]*Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Tenant, 0, len(s.byID))
	for _, t := range s.byID {
		cp := *t
		out = append(out, &cp)
	}
	return out, nil
}

func (s *MemStore) SetState(_ context.Context, id string, state TenantState) (*Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	t.State = state
	t.UpdatedAt = time.Now().UTC()
	cp := *t
	return &cp, nil
}
