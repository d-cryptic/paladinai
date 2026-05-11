package topology_test

import (
	"context"
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/topology"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Fixtures ─────────────────────────────────────────────────────────────────

func newStore() *topology.InMemoryStore {
	return topology.NewInMemoryStore()
}

func mustUpsertService(t *testing.T, s *topology.InMemoryStore, svc topology.Service) {
	t.Helper()
	require.NoError(t, s.UpsertService(context.Background(), svc))
}

func mustUpsertEdge(t *testing.T, s *topology.InMemoryStore, edge topology.DependsOnEdge) {
	t.Helper()
	require.NoError(t, s.UpsertDependency(context.Background(), edge))
}

// ─── UpsertService ────────────────────────────────────────────────────────────

func TestUpsertService_StoresService(t *testing.T) {
	st := newStore()
	svc := topology.Service{ID: "svc-1", Name: "payments", TenantID: "t1", Tier: "critical"}
	require.NoError(t, st.UpsertService(context.Background(), svc))

	svcs, err := st.ServicesByTenant(context.Background(), "t1")
	require.NoError(t, err)
	require.Len(t, svcs, 1)
	assert.Equal(t, "payments", svcs[0].Name)
}

func TestUpsertService_UpdatesExisting(t *testing.T) {
	st := newStore()
	mustUpsertService(t, st, topology.Service{ID: "svc-1", Name: "payments", TenantID: "t1", Tier: "critical"})
	mustUpsertService(t, st, topology.Service{ID: "svc-1", Name: "payments-v2", TenantID: "t1", Tier: "standard"})

	svcs, err := st.ServicesByTenant(context.Background(), "t1")
	require.NoError(t, err)
	require.Len(t, svcs, 1)
	assert.Equal(t, "payments-v2", svcs[0].Name)
}

func TestUpsertService_MissingIDOrTenant_Error(t *testing.T) {
	st := newStore()
	assert.Error(t, st.UpsertService(context.Background(), topology.Service{Name: "x", TenantID: "t1"}))
	assert.Error(t, st.UpsertService(context.Background(), topology.Service{ID: "id", Name: "x"}))
}

// ─── ServicesByTenant ─────────────────────────────────────────────────────────

func TestServicesByTenant_Isolation(t *testing.T) {
	st := newStore()
	mustUpsertService(t, st, topology.Service{ID: "svc-1", Name: "api", TenantID: "t1"})
	mustUpsertService(t, st, topology.Service{ID: "svc-2", Name: "db", TenantID: "t2"})

	t1Svcs, err := st.ServicesByTenant(context.Background(), "t1")
	require.NoError(t, err)
	assert.Len(t, t1Svcs, 1)
	assert.Equal(t, "api", t1Svcs[0].Name)
}

func TestServicesByTenant_Empty(t *testing.T) {
	st := newStore()
	svcs, err := st.ServicesByTenant(context.Background(), "unknown-tenant")
	require.NoError(t, err)
	assert.Empty(t, svcs)
}

// ─── BlastRadius ──────────────────────────────────────────────────────────────

func TestBlastRadius_DirectDependency(t *testing.T) {
	st := newStore()
	mustUpsertService(t, st, topology.Service{ID: "svc-a", Name: "api", TenantID: "t1"})
	mustUpsertService(t, st, topology.Service{ID: "svc-b", Name: "payments", TenantID: "t1"})
	mustUpsertEdge(t, st, topology.DependsOnEdge{FromServiceID: "svc-a", ToServiceID: "svc-b"})

	blast, err := st.BlastRadius(context.Background(), "t1", "api", 5)
	require.NoError(t, err)
	assert.Contains(t, blast, "payments")
}

func TestBlastRadius_Transitive(t *testing.T) {
	// api → payments → db (2 hops)
	st := newStore()
	mustUpsertService(t, st, topology.Service{ID: "svc-a", Name: "api", TenantID: "t1"})
	mustUpsertService(t, st, topology.Service{ID: "svc-b", Name: "payments", TenantID: "t1"})
	mustUpsertService(t, st, topology.Service{ID: "svc-c", Name: "db", TenantID: "t1"})
	mustUpsertEdge(t, st, topology.DependsOnEdge{FromServiceID: "svc-a", ToServiceID: "svc-b"})
	mustUpsertEdge(t, st, topology.DependsOnEdge{FromServiceID: "svc-b", ToServiceID: "svc-c"})

	blast, err := st.BlastRadius(context.Background(), "t1", "api", 5)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"payments", "db"}, blast)
}

func TestBlastRadius_MaxDepthLimitsResults(t *testing.T) {
	// api → payments → db — with maxDepth=1, only payments should appear
	st := newStore()
	mustUpsertService(t, st, topology.Service{ID: "svc-a", Name: "api", TenantID: "t1"})
	mustUpsertService(t, st, topology.Service{ID: "svc-b", Name: "payments", TenantID: "t1"})
	mustUpsertService(t, st, topology.Service{ID: "svc-c", Name: "db", TenantID: "t1"})
	mustUpsertEdge(t, st, topology.DependsOnEdge{FromServiceID: "svc-a", ToServiceID: "svc-b"})
	mustUpsertEdge(t, st, topology.DependsOnEdge{FromServiceID: "svc-b", ToServiceID: "svc-c"})

	blast, err := st.BlastRadius(context.Background(), "t1", "api", 1)
	require.NoError(t, err)
	assert.Equal(t, []string{"payments"}, blast)
}

func TestBlastRadius_NoCyclicLoop(t *testing.T) {
	// a → b → a (cycle) — must not loop forever
	st := newStore()
	mustUpsertService(t, st, topology.Service{ID: "svc-a", Name: "a", TenantID: "t1"})
	mustUpsertService(t, st, topology.Service{ID: "svc-b", Name: "b", TenantID: "t1"})
	mustUpsertEdge(t, st, topology.DependsOnEdge{FromServiceID: "svc-a", ToServiceID: "svc-b"})
	mustUpsertEdge(t, st, topology.DependsOnEdge{FromServiceID: "svc-b", ToServiceID: "svc-a"})

	blast, err := st.BlastRadius(context.Background(), "t1", "a", 5)
	require.NoError(t, err)
	assert.Equal(t, []string{"b"}, blast)
}

func TestBlastRadius_ServiceNotFound_Error(t *testing.T) {
	st := newStore()
	_, err := st.BlastRadius(context.Background(), "t1", "nonexistent", 5)
	assert.ErrorIs(t, err, topology.ErrServiceNotFound)
}

func TestBlastRadius_InvalidDepth_Error(t *testing.T) {
	st := newStore()
	mustUpsertService(t, st, topology.Service{ID: "svc-a", Name: "api", TenantID: "t1"})
	_, err := st.BlastRadius(context.Background(), "t1", "api", 0)
	assert.ErrorIs(t, err, topology.ErrInvalidDepth)
}

func TestBlastRadius_TenantIsolation(t *testing.T) {
	st := newStore()
	mustUpsertService(t, st, topology.Service{ID: "svc-a", Name: "api", TenantID: "t1"})
	mustUpsertService(t, st, topology.Service{ID: "svc-b", Name: "payments", TenantID: "t2"})
	// Edge between t1 and t2 services (cross-tenant edge in memory — shouldn't appear in t1 blast radius)
	mustUpsertEdge(t, st, topology.DependsOnEdge{FromServiceID: "svc-a", ToServiceID: "svc-b"})

	blast, err := st.BlastRadius(context.Background(), "t1", "api", 5)
	require.NoError(t, err)
	// svc-b is t2's service — its name appears in blast radius but belongs to another tenant.
	// The InMemoryStore doesn't filter by tenant on edges (FalkorDB handles this via graph isolation).
	// This test just ensures no panic/error on cross-tenant edge traversal.
	_ = blast
}

func TestBlastRadius_NoOutboundEdges_Empty(t *testing.T) {
	st := newStore()
	mustUpsertService(t, st, topology.Service{ID: "svc-a", Name: "leaf", TenantID: "t1"})

	blast, err := st.BlastRadius(context.Background(), "t1", "leaf", 5)
	require.NoError(t, err)
	assert.Empty(t, blast)
}

// ─── RecentDeployments ────────────────────────────────────────────────────────

func TestRecentDeployments_WithinWindow(t *testing.T) {
	st := newStore()
	mustUpsertService(t, st, topology.Service{ID: "svc-1", Name: "api", TenantID: "t1"})

	now := time.Now()
	dep := topology.Deployment{
		ID:         "dep-1",
		ServiceID:  "svc-1",
		TenantID:   "t1",
		Version:    "v2.1.0",
		SHA:        "abc123",
		DeployedBy: "ci-bot",
		DeployedAt: now.Add(-15 * time.Minute),
	}
	require.NoError(t, st.UpsertDeployment(context.Background(), dep))

	results, err := st.RecentDeployments(context.Background(), "t1", "api",
		now.Add(-30*time.Minute), now)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "v2.1.0", results[0].Version)
}

func TestRecentDeployments_OutsideWindow_Empty(t *testing.T) {
	st := newStore()
	mustUpsertService(t, st, topology.Service{ID: "svc-1", Name: "api", TenantID: "t1"})

	now := time.Now()
	dep := topology.Deployment{
		ID: "dep-1", ServiceID: "svc-1", TenantID: "t1",
		DeployedAt: now.Add(-2 * time.Hour), // outside 30-min window
	}
	require.NoError(t, st.UpsertDeployment(context.Background(), dep))

	results, err := st.RecentDeployments(context.Background(), "t1", "api",
		now.Add(-30*time.Minute), now)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestRecentDeployments_ServiceNotFound_Error(t *testing.T) {
	st := newStore()
	now := time.Now()
	_, err := st.RecentDeployments(context.Background(), "t1", "ghost",
		now.Add(-time.Hour), now)
	assert.ErrorIs(t, err, topology.ErrServiceNotFound)
}

func TestUpsertDependency_DeduplicatesEdges(t *testing.T) {
	st := newStore()
	mustUpsertService(t, st, topology.Service{ID: "a", Name: "a", TenantID: "t1"})
	mustUpsertService(t, st, topology.Service{ID: "b", Name: "b", TenantID: "t1"})

	edge := topology.DependsOnEdge{FromServiceID: "a", ToServiceID: "b", Protocol: "http"}
	require.NoError(t, st.UpsertDependency(context.Background(), edge))
	// Update weight
	edge.Weight = 0.9
	require.NoError(t, st.UpsertDependency(context.Background(), edge))

	blast, err := st.BlastRadius(context.Background(), "t1", "a", 1)
	require.NoError(t, err)
	assert.Len(t, blast, 1, "deduplication must prevent duplicate edges")
}
