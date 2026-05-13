// Package topology models the service dependency graph and provides
// query operations for blast radius analysis, causal chain discovery,
// and deployment correlation.
//
// The Store interface is designed for FalkorDB (graph database) as the
// backend, but is decoupled to allow in-memory fakes in tests.
//
// See docs/plans/05.memory-stage5.md §4 for the graph schema specification.
package topology

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ─── Node types ───────────────────────────────────────────────────────────────

// Service is a node in the topology graph representing a microservice.
type Service struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	TenantID string `json:"tenant_id"`
	Tier     string `json:"tier"` // "critical" | "standard" | "experimental"
	Lang     string `json:"lang"`
}

// Team owns one or more services and has an on-call rotation.
type Team struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	TenantID        string `json:"tenant_id"`
	OncallPagerDuty string `json:"oncall_pagerduty"`
}

// SLO is a service level objective attached to a service.
type SLO struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Target   float64 `json:"target"` // e.g. 0.999
	Window   string  `json:"window"` // e.g. "30d"
	TenantID string  `json:"tenant_id"`
}

// Deployment tracks a code deployment to a service (used for incident correlation).
type Deployment struct {
	ID         string    `json:"id"`
	ServiceID  string    `json:"service_id"`
	Version    string    `json:"version"`
	SHA        string    `json:"sha"`
	DeployedBy string    `json:"deployed_by"`
	DeployedAt time.Time `json:"deployed_at"`
	TenantID   string    `json:"tenant_id"`
}

// DependsOnEdge is the structural dependency edge between two services.
type DependsOnEdge struct {
	TenantID      string  `json:"tenant_id"`
	FromServiceID string  `json:"from_service_id"`
	ToServiceID   string  `json:"to_service_id"`
	Protocol      string  `json:"protocol"` // "http" | "grpc" | "kafka" | "db"
	Weight        float64 `json:"weight"`   // [0,1] traffic weight
}

// ─── Store interface ──────────────────────────────────────────────────────────

// Store is the graph storage interface for topology operations.
// Concrete implementations: FalkorDB (production), InMemoryStore (tests).
type Store interface {
	// UpsertService creates or updates a Service node.
	UpsertService(ctx context.Context, svc Service) error

	// UpsertDependency creates or updates a DEPENDS_ON edge between services.
	UpsertDependency(ctx context.Context, edge DependsOnEdge) error

	// UpsertDeployment creates or updates a Deployment node and DEPLOYED_TO edge.
	UpsertDeployment(ctx context.Context, dep Deployment) error

	// BlastRadius returns names of all downstream services reachable from serviceName
	// up to maxDepth hops via DEPENDS_ON edges, for the given tenant.
	BlastRadius(ctx context.Context, tenantID, serviceName string, maxDepth int) ([]string, error)

	// RecentDeployments returns deployments to a service in the window [since, until].
	// Used for incident correlation: find deploys that preceded an incident.
	RecentDeployments(ctx context.Context, tenantID, serviceName string, since, until time.Time) ([]Deployment, error)

	// ServicesByTenant returns all services for a tenant.
	ServicesByTenant(ctx context.Context, tenantID string) ([]Service, error)
}

// ─── Errors ───────────────────────────────────────────────────────────────────

// ErrServiceNotFound is returned when a service does not exist in the graph.
var ErrServiceNotFound = errors.New("topology: service not found")

// ErrInvalidDepth is returned when maxDepth is < 1.
var ErrInvalidDepth = errors.New("topology: maxDepth must be >= 1")

// ─── InMemoryStore ────────────────────────────────────────────────────────────

// InMemoryStore is a test-only in-memory implementation of Store.
// It uses adjacency lists for graph traversal (BFS for BlastRadius).
// All methods are safe for concurrent use.
type InMemoryStore struct {
	mu          sync.RWMutex
	services    map[string]Service // key: tenantID+":"+serviceID
	edges       []DependsOnEdge
	deployments []Deployment
}

// NewInMemoryStore creates an empty InMemoryStore.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		services: make(map[string]Service),
	}
}

func (s *InMemoryStore) UpsertService(_ context.Context, svc Service) error {
	if svc.TenantID == "" || svc.ID == "" {
		return fmt.Errorf("topology: service requires tenant_id and id")
	}
	s.mu.Lock()
	s.services[svc.TenantID+":"+svc.ID] = svc
	s.mu.Unlock()
	return nil
}

func (s *InMemoryStore) UpsertDependency(_ context.Context, edge DependsOnEdge) error {
	if edge.TenantID == "" {
		return fmt.Errorf("topology: dependency edge requires tenant_id")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.edges {
		if e.TenantID == edge.TenantID && e.FromServiceID == edge.FromServiceID && e.ToServiceID == edge.ToServiceID {
			s.edges[i] = edge
			return nil
		}
	}
	s.edges = append(s.edges, edge)
	return nil
}

func (s *InMemoryStore) UpsertDeployment(_ context.Context, dep Deployment) error {
	if dep.TenantID == "" || dep.ServiceID == "" {
		return fmt.Errorf("topology: deployment requires tenant_id and service_id")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, d := range s.deployments {
		if d.ID == dep.ID {
			s.deployments[i] = dep
			return nil
		}
	}
	s.deployments = append(s.deployments, dep)
	return nil
}

// BlastRadius performs a BFS from the named service, returning all reachable downstream
// service names up to maxDepth hops (excluding the source itself).
func (s *InMemoryStore) BlastRadius(_ context.Context, tenantID, serviceName string, maxDepth int) ([]string, error) {
	if maxDepth < 1 {
		return nil, ErrInvalidDepth
	}

	s.mu.RLock()
	// Build a name→ID index for this tenant.
	nameToID := make(map[string]string)
	idToName := make(map[string]string)
	for _, svc := range s.services {
		if svc.TenantID != tenantID {
			continue
		}
		nameToID[svc.Name] = svc.ID
		idToName[svc.ID] = svc.Name
	}
	edges := make([]DependsOnEdge, len(s.edges))
	copy(edges, s.edges)
	s.mu.RUnlock()

	sourceID, ok := nameToID[serviceName]
	if !ok {
		return nil, fmt.Errorf("%w: %q in tenant %q", ErrServiceNotFound, serviceName, tenantID)
	}

	// BFS.
	visited := map[string]bool{sourceID: true}
	queue := []string{sourceID}
	depth := 0
	var result []string

	for len(queue) > 0 && depth < maxDepth {
		next := queue
		queue = nil
		depth++
		for _, nodeID := range next {
			for _, edge := range edges {
				if edge.TenantID != tenantID || edge.FromServiceID != nodeID {
					continue
				}
				if !visited[edge.ToServiceID] {
					visited[edge.ToServiceID] = true
					queue = append(queue, edge.ToServiceID)
					if name, exists := idToName[edge.ToServiceID]; exists {
						result = append(result, name)
					}
				}
			}
		}
	}
	return result, nil
}

// RecentDeployments returns deployments to a service in [since, until].
func (s *InMemoryStore) RecentDeployments(_ context.Context, tenantID, serviceName string, since, until time.Time) ([]Deployment, error) {
	s.mu.RLock()
	// Build name→ID map.
	var serviceID string
	for _, svc := range s.services {
		if svc.TenantID == tenantID && svc.Name == serviceName {
			serviceID = svc.ID
			break
		}
	}
	deps := make([]Deployment, len(s.deployments))
	copy(deps, s.deployments)
	s.mu.RUnlock()

	if serviceID == "" {
		return nil, fmt.Errorf("%w: %q in tenant %q", ErrServiceNotFound, serviceName, tenantID)
	}

	var result []Deployment
	for _, dep := range deps {
		if dep.TenantID == tenantID &&
			dep.ServiceID == serviceID &&
			!dep.DeployedAt.Before(since) &&
			!dep.DeployedAt.After(until) {
			result = append(result, dep)
		}
	}
	return result, nil
}

// ServicesByTenant returns all services for a tenant.
func (s *InMemoryStore) ServicesByTenant(_ context.Context, tenantID string) ([]Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Service
	for _, svc := range s.services {
		if svc.TenantID == tenantID {
			result = append(result, svc)
		}
	}
	return result, nil
}

// compile-time interface check.
var _ Store = (*InMemoryStore)(nil)
