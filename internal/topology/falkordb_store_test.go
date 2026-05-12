package topology

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// fakeGraphRunner captures Cypher queries and returns pre-canned responses.
type fakeGraphRunner struct {
	// responses is a FIFO queue; each Do() call pops the front entry.
	responses []fakeResponse
	// captured holds every cypher string sent.
	captured []string
}

type fakeResponse struct {
	result interface{}
	err    error
}

func (f *fakeGraphRunner) Do(_ context.Context, args ...interface{}) *redis.Cmd {
	// args: "GRAPH.QUERY", graphName, cypher, "--compact"
	if len(args) >= 3 {
		if cypher, ok := args[2].(string); ok {
			f.captured = append(f.captured, cypher)
		}
	}
	cmd := redis.NewCmd(context.Background(), args...)
	if len(f.responses) == 0 {
		// Default: successful empty write response (header=[], rows=[], stats=[]).
		cmd.SetVal([]interface{}{
			[]interface{}{}, // header
			[]interface{}{}, // rows
			[]interface{}{}, // stats
		})
		return cmd
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	if resp.err != nil {
		cmd.SetErr(resp.err)
	} else {
		cmd.SetVal(resp.result)
	}
	return cmd
}

// emptyOK is a standard successful write response with no data rows.
func emptyOK() fakeResponse {
	return fakeResponse{result: []interface{}{
		[]interface{}{},
		[]interface{}{},
		[]interface{}{},
	}}
}

// rowsOK wraps data rows in the FalkorDB compact envelope.
func rowsOK(rows []interface{}) fakeResponse {
	return fakeResponse{result: []interface{}{
		[]interface{}{},
		rows,
		[]interface{}{},
	}}
}

func newFakeStore(responses ...fakeResponse) (*FalkorDBStore, *fakeGraphRunner) {
	f := &fakeGraphRunner{responses: responses}
	s := &FalkorDBStore{runner: f, graph: "test_graph"}
	return s, f
}

// ─── UpsertService ────────────────────────────────────────────────────────────

func TestFalkorDB_UpsertService_SendsCypher(t *testing.T) {
	s, f := newFakeStore(emptyOK())
	err := s.UpsertService(context.Background(), Service{
		ID: "svc-1", TenantID: "t1", Name: "api-gateway", Tier: "critical", Lang: "go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.captured) != 1 {
		t.Fatalf("want 1 query, got %d", len(f.captured))
	}
}

func TestFalkorDB_UpsertService_ValidationError(t *testing.T) {
	s, _ := newFakeStore()
	err := s.UpsertService(context.Background(), Service{ID: "x"}) // missing TenantID
	if err == nil {
		t.Fatal("want error for missing tenant_id")
	}
}

func TestFalkorDB_UpsertService_PropagatesRedisError(t *testing.T) {
	s, _ := newFakeStore(fakeResponse{err: errors.New("connection refused")})
	err := s.UpsertService(context.Background(), Service{ID: "x", TenantID: "t1"})
	if err == nil {
		t.Fatal("want error from redis")
	}
}

// ─── UpsertDependency ─────────────────────────────────────────────────────────

func TestFalkorDB_UpsertDependency_SendsCypher(t *testing.T) {
	s, f := newFakeStore(emptyOK())
	err := s.UpsertDependency(context.Background(), DependsOnEdge{
		FromServiceID: "svc-1", ToServiceID: "svc-2", Protocol: "http", Weight: 0.9,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.captured) != 1 {
		t.Fatalf("want 1 query, got %d", len(f.captured))
	}
}

// ─── UpsertDeployment ────────────────────────────────────────────────────────

func TestFalkorDB_UpsertDeployment_SendsCypher(t *testing.T) {
	s, f := newFakeStore(emptyOK())
	err := s.UpsertDeployment(context.Background(), Deployment{
		ID:         "dep-1",
		ServiceID:  "svc-1",
		TenantID:   "t1",
		Version:    "v2.0.0",
		SHA:        "abc123",
		DeployedBy: "ci-bot",
		DeployedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.captured) != 1 {
		t.Fatalf("want 1 query, got %d", len(f.captured))
	}
}

func TestFalkorDB_UpsertDeployment_ValidationError(t *testing.T) {
	s, _ := newFakeStore()
	err := s.UpsertDeployment(context.Background(), Deployment{ID: "dep-1"}) // missing ServiceID + TenantID
	if err == nil {
		t.Fatal("want validation error")
	}
}

// ─── BlastRadius ─────────────────────────────────────────────────────────────

func TestFalkorDB_BlastRadius_ReturnsNames(t *testing.T) {
	rows := []interface{}{
		[]interface{}{"svc-b"},
		[]interface{}{"svc-c"},
	}
	s, _ := newFakeStore(rowsOK(rows))
	names, err := s.BlastRadius(context.Background(), "t1", "api-gateway", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("want 2 names, got %d: %v", len(names), names)
	}
	if names[0] != "svc-b" || names[1] != "svc-c" {
		t.Errorf("unexpected names: %v", names)
	}
}

func TestFalkorDB_BlastRadius_EmptyResult(t *testing.T) {
	s, _ := newFakeStore(rowsOK([]interface{}{}))
	names, err := s.BlastRadius(context.Background(), "t1", "leaf-service", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("want empty slice, got %v", names)
	}
}

func TestFalkorDB_BlastRadius_InvalidDepth(t *testing.T) {
	s, _ := newFakeStore()
	_, err := s.BlastRadius(context.Background(), "t1", "svc", 0)
	if !errors.Is(err, ErrInvalidDepth) {
		t.Fatalf("want ErrInvalidDepth, got %v", err)
	}
}

// ─── RecentDeployments ───────────────────────────────────────────────────────

func TestFalkorDB_RecentDeployments_ReturnsParsed(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	rows := []interface{}{
		[]interface{}{"dep-1", "svc-1", "v1.2.3", "deadbeef", "ci-bot", now.Unix(), "t1"},
	}
	s, _ := newFakeStore(rowsOK(rows))
	deps, err := s.RecentDeployments(context.Background(), "t1", "api-gw", now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("want 1 deployment, got %d", len(deps))
	}
	d := deps[0]
	if d.ID != "dep-1" || d.Version != "v1.2.3" || d.SHA != "deadbeef" {
		t.Errorf("unexpected deployment: %+v", d)
	}
	if !d.DeployedAt.Equal(now) {
		t.Errorf("want deployed_at %v, got %v", now, d.DeployedAt)
	}
}

func TestFalkorDB_RecentDeployments_Empty(t *testing.T) {
	s, _ := newFakeStore(rowsOK([]interface{}{}))
	deps, err := s.RecentDeployments(context.Background(), "t1", "svc", time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("want empty, got %d", len(deps))
	}
}

// ─── ServicesByTenant ────────────────────────────────────────────────────────

func TestFalkorDB_ServicesByTenant_ReturnsParsed(t *testing.T) {
	rows := []interface{}{
		[]interface{}{"svc-1", "api-gw", "critical", "go"},
		[]interface{}{"svc-2", "worker", "standard", "go"},
	}
	s, _ := newFakeStore(rowsOK(rows))
	svcs, err := s.ServicesByTenant(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svcs) != 2 {
		t.Fatalf("want 2 services, got %d", len(svcs))
	}
	if svcs[0].Name != "api-gw" || svcs[1].Name != "worker" {
		t.Errorf("unexpected services: %+v", svcs)
	}
	for _, svc := range svcs {
		if svc.TenantID != "t1" {
			t.Errorf("want tenant_id t1, got %q", svc.TenantID)
		}
	}
}

func TestFalkorDB_ServicesByTenant_Empty(t *testing.T) {
	s, _ := newFakeStore(rowsOK([]interface{}{}))
	svcs, err := s.ServicesByTenant(context.Background(), "unknown-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svcs) != 0 {
		t.Errorf("want empty, got %d", len(svcs))
	}
}

// ─── response parsing edge cases ─────────────────────────────────────────────

func TestFalkorDB_MalformedResponse_ReturnsError(t *testing.T) {
	// Return a non-array response to trigger the "unexpected type" path.
	s, _ := newFakeStore(fakeResponse{result: "not-an-array"})
	_, err := s.ServicesByTenant(context.Background(), "t1")
	if err == nil {
		t.Fatal("want error for malformed response")
	}
}
