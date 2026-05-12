package topology

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// fakeGraphRunner captures Cypher queries and returns pre-canned responses.
// Not goroutine-safe; use from a single goroutine only.
type fakeGraphRunner struct {
	responses []fakeResponse
	captured  []string // every Cypher string sent
}

type fakeResponse struct {
	result any
	err    error
}

func (f *fakeGraphRunner) Do(_ context.Context, args ...any) *redis.Cmd {
	if len(args) >= 3 {
		if cypher, ok := args[2].(string); ok {
			f.captured = append(f.captured, cypher)
		}
	}
	cmd := redis.NewCmd(context.Background(), args...)
	if len(f.responses) == 0 {
		cmd.SetVal(emptyOKResult())
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

// emptyOKResult is the standard successful write response (no data rows).
func emptyOKResult() any {
	return []any{[]any{}, []any{}, []any{}}
}

func emptyOK() fakeResponse { return fakeResponse{result: emptyOKResult()} }

// rowsOK wraps data rows in the FalkorDB plain-format envelope.
func rowsOK(rows []any) fakeResponse {
	return fakeResponse{result: []any{[]any{}, rows, []any{}}}
}

func newFakeStore(responses ...fakeResponse) (*FalkorDBStore, *fakeGraphRunner) {
	f := &fakeGraphRunner{responses: responses}
	return &FalkorDBStore{runner: f, graph: "test_graph"}, f
}

// ─── cypherStr ────────────────────────────────────────────────────────────────

func TestCypherStr_EscapesBackslash(t *testing.T) {
	got := cypherStr(`C:\Users\admin`)
	want := `'C:\\Users\\admin'`
	if got != want {
		t.Errorf("want %s, got %s", want, got)
	}
}

func TestCypherStr_EscapesSingleQuote(t *testing.T) {
	got := cypherStr("O'Brien")
	want := `'O\'Brien'`
	if got != want {
		t.Errorf("want %s, got %s", want, got)
	}
}

func TestCypherStr_InjectionAttempt(t *testing.T) {
	// A naive injection: attempt to close the string and inject Cypher.
	input := "x'}) MATCH (n) DETACH DELETE n //"
	got := cypherStr(input)
	// The inner single-quote must be escaped (\'). If it were bare, the attacker
	// would break out of the Cypher string literal.
	if strings.Contains(got, "x')") {
		t.Errorf("single-quote in injection was not escaped: %s", got)
	}
	if !strings.HasPrefix(got, "'") || !strings.HasSuffix(got, "'") {
		t.Errorf("result not properly single-quoted: %s", got)
	}
	// The escaped form is 'x\'}) MATCH ...'; it is safe because it is
	// treated as the literal string value — not executable Cypher.
	if !strings.Contains(got, `x\'`) {
		t.Errorf("expected escaped quote in output: %s", got)
	}
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
	if !strings.Contains(f.captured[0], "MERGE") {
		t.Errorf("expected MERGE in cypher: %s", f.captured[0])
	}
	if !strings.Contains(f.captured[0], "'t1'") {
		t.Errorf("expected tenant_id in cypher: %s", f.captured[0])
	}
}

func TestFalkorDB_UpsertService_MissingID(t *testing.T) {
	s, _ := newFakeStore()
	err := s.UpsertService(context.Background(), Service{TenantID: "t1"})
	if err == nil {
		t.Fatal("want error for missing id")
	}
}

func TestFalkorDB_UpsertService_MissingTenantID(t *testing.T) {
	s, _ := newFakeStore()
	err := s.UpsertService(context.Background(), Service{ID: "x"})
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
	if !strings.Contains(err.Error(), "GRAPH.QUERY") {
		t.Errorf("error should mention GRAPH.QUERY: %v", err)
	}
}

// ─── UpsertDependency ─────────────────────────────────────────────────────────

func TestFalkorDB_UpsertDependency_SendsCypherWithTenantScope(t *testing.T) {
	s, f := newFakeStore(emptyOK())
	err := s.UpsertDependency(context.Background(), DependsOnEdge{
		TenantID: "t1", FromServiceID: "svc-1", ToServiceID: "svc-2", Protocol: "http", Weight: 0.9,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.captured) != 1 {
		t.Fatalf("want 1 query, got %d", len(f.captured))
	}
	// Both MATCH arms must include the tenant_id to prevent cross-tenant edges.
	count := strings.Count(f.captured[0], "'t1'")
	if count < 2 {
		t.Errorf("want tenant_id scoped on both sides, got %d occurrence(s): %s", count, f.captured[0])
	}
}

func TestFalkorDB_UpsertDependency_MissingTenantID(t *testing.T) {
	s, _ := newFakeStore()
	err := s.UpsertDependency(context.Background(), DependsOnEdge{
		FromServiceID: "svc-1", ToServiceID: "svc-2",
	})
	if err == nil {
		t.Fatal("want error for missing tenant_id")
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
	// MERGE on Deployment must include tenant_id to prevent cross-tenant collisions.
	if !strings.Contains(f.captured[0], "tenant_id") {
		t.Errorf("MERGE on Deployment must include tenant_id: %s", f.captured[0])
	}
}

func TestFalkorDB_UpsertDeployment_MissingRequired(t *testing.T) {
	s, _ := newFakeStore()
	err := s.UpsertDeployment(context.Background(), Deployment{ID: "dep-1"}) // no ServiceID / TenantID
	if err == nil {
		t.Fatal("want validation error")
	}
}

// ─── BlastRadius ─────────────────────────────────────────────────────────────

func TestFalkorDB_BlastRadius_ReturnsNames(t *testing.T) {
	rows := []any{
		[]any{"svc-b"},
		[]any{"svc-c"},
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

func TestFalkorDB_BlastRadius_InvalidDepth(t *testing.T) {
	s, _ := newFakeStore()
	_, err := s.BlastRadius(context.Background(), "t1", "svc", 0)
	if !errors.Is(err, ErrInvalidDepth) {
		t.Fatalf("want ErrInvalidDepth, got %v", err)
	}
}

func TestFalkorDB_BlastRadius_CapsMaxDepth(t *testing.T) {
	s, f := newFakeStore(rowsOK([]any{}))
	_, err := s.BlastRadius(context.Background(), "t1", "svc", 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The Cypher should use maxBlastRadiusDepth (10), not 999.
	if !strings.Contains(f.captured[0], "1..10") {
		t.Errorf("expected depth capped to 10 in: %s", f.captured[0])
	}
}

func TestFalkorDB_BlastRadius_EmptyResult(t *testing.T) {
	s, _ := newFakeStore(rowsOK([]any{}))
	names, err := s.BlastRadius(context.Background(), "t1", "leaf-service", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("want empty slice, got %v", names)
	}
}

// ─── RecentDeployments ───────────────────────────────────────────────────────

func TestFalkorDB_RecentDeployments_ReturnsParsed(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	rows := []any{
		[]any{"dep-1", "svc-1", "v1.2.3", "deadbeef", "ci-bot", now.Unix(), "t1"},
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

func TestFalkorDB_RecentDeployments_BadTimestampSkipsRow(t *testing.T) {
	rows := []any{
		[]any{"dep-1", "svc-1", "v1", "sha1", "bot", "not-a-number", "t1"},
	}
	s, _ := newFakeStore(rowsOK(rows))
	deps, err := s.RecentDeployments(context.Background(), "t1", "svc", time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Row with bad timestamp is silently skipped (not returned, not fatal).
	if len(deps) != 0 {
		t.Errorf("want 0 (bad row skipped), got %d: %+v", len(deps), deps)
	}
}

func TestFalkorDB_RecentDeployments_MissingArgs(t *testing.T) {
	s, _ := newFakeStore()
	_, err := s.RecentDeployments(context.Background(), "", "svc", time.Now(), time.Now())
	if err == nil {
		t.Fatal("want error for empty tenantID")
	}
}

func TestFalkorDB_RecentDeployments_Empty(t *testing.T) {
	s, _ := newFakeStore(rowsOK([]any{}))
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
	rows := []any{
		[]any{"svc-1", "api-gw", "critical", "go"},
		[]any{"svc-2", "worker", "standard", "go"},
	}
	s, _ := newFakeStore(rowsOK(rows))
	svcs, err := s.ServicesByTenant(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svcs) != 2 {
		t.Fatalf("want 2 services, got %d", len(svcs))
	}
	for _, svc := range svcs {
		if svc.TenantID != "t1" {
			t.Errorf("want tenant_id t1, got %q", svc.TenantID)
		}
	}
}

func TestFalkorDB_ServicesByTenant_Empty(t *testing.T) {
	s, _ := newFakeStore(rowsOK([]any{}))
	svcs, err := s.ServicesByTenant(context.Background(), "unknown-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svcs) != 0 {
		t.Errorf("want empty, got %d", len(svcs))
	}
}

func TestFalkorDB_ServicesByTenant_MissingTenantID(t *testing.T) {
	s, _ := newFakeStore()
	_, err := s.ServicesByTenant(context.Background(), "")
	if err == nil {
		t.Fatal("want error for empty tenantID")
	}
}

// ─── response parsing edge cases ─────────────────────────────────────────────

func TestFalkorDB_MalformedResponse_NotArray(t *testing.T) {
	s, _ := newFakeStore(fakeResponse{result: "not-an-array"})
	_, err := s.ServicesByTenant(context.Background(), "t1")
	if err == nil {
		t.Fatal("want error for malformed (non-array) response")
	}
}

func TestFalkorDB_MalformedResponse_RowsNotArray(t *testing.T) {
	// parts[1] is not []any
	s, _ := newFakeStore(fakeResponse{result: []any{[]any{}, "bad-rows", []any{}}})
	_, err := s.ServicesByTenant(context.Background(), "t1")
	if err == nil {
		t.Fatal("want error when rows field is not []any")
	}
}

func TestFalkorDB_SkipsMalformedRows(t *testing.T) {
	// One malformed row (wrong arity), one good row.
	rows := []any{
		"not-a-row",
		[]any{"svc-1", "api-gw", "critical", "go"},
	}
	s, _ := newFakeStore(rowsOK(rows))
	svcs, err := s.ServicesByTenant(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Bad row is skipped; good row is returned.
	if len(svcs) != 1 || svcs[0].ID != "svc-1" {
		t.Errorf("want 1 valid service, got %+v", svcs)
	}
}
