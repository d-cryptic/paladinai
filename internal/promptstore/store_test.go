package promptstore

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

// ── Fake Querier ─────────────────────────────────────────────────────────────

type fakeRow struct{ agent, model, content string }

type fakeRows struct {
	data   []fakeRow
	idx    int
	closed bool
	err    error
}

func (r *fakeRows) Next() bool {
	if r.err != nil {
		return false
	}
	if r.idx >= len(r.data) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if len(dest) != 3 {
		return errors.New("expected 3 columns")
	}
	row := r.data[r.idx-1]
	*(dest[0].(*string)) = row.agent
	*(dest[1].(*string)) = row.model
	*(dest[2].(*string)) = row.content
	return nil
}

func (r *fakeRows) Err() error { return r.err }
func (r *fakeRows) Close()     { r.closed = true }

type fakeDB struct {
	rows    []fakeRow
	queries atomic.Int64
	failErr error
}

func (f *fakeDB) Query(_ context.Context, _ string, _ ...any) (Rows, error) {
	f.queries.Add(1)
	if f.failErr != nil {
		return nil, f.failErr
	}
	return &fakeRows{data: f.rows}, nil
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestNew_NilDB_UsesDefaults(t *testing.T) {
	s, err := New(context.Background(), nil, zap.NewNop())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got := s.Get("triage", "claude-sonnet-4-6")
	if !strings.Contains(got, "triage agent") {
		t.Fatalf("expected default triage prompt, got %q", got)
	}
}

func TestNew_DBError_UsesDefaults(t *testing.T) {
	db := &fakeDB{failErr: errors.New("connection refused")}
	s, err := New(context.Background(), db, zap.NewNop())
	if err != nil {
		t.Fatalf("New must not return error on graceful degradation: %v", err)
	}
	if got := s.Get("supervisor", "*"); !strings.Contains(got, "supervisor agent") {
		t.Fatalf("expected default supervisor prompt, got %q", got)
	}
}

func TestGet_ExactMatchOverridesWildcard(t *testing.T) {
	db := &fakeDB{rows: []fakeRow{
		{"triage", "*", "wildcard-content"},
		{"triage", "claude-sonnet-4-6", "exact-content"},
	}}
	s, err := New(context.Background(), db, zap.NewNop())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := s.Get("triage", "claude-sonnet-4-6"); got != "exact-content" {
		t.Fatalf("exact match: got %q", got)
	}
}

func TestGet_WildcardFallback(t *testing.T) {
	db := &fakeDB{rows: []fakeRow{
		{"rca", "*", "wildcard-rca"},
	}}
	s, err := New(context.Background(), db, zap.NewNop())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := s.Get("rca", "qwen3-72b"); got != "wildcard-rca" {
		t.Fatalf("wildcard fallback: got %q", got)
	}
}

func TestGet_UnknownAgentReturnsEmpty(t *testing.T) {
	s, err := New(context.Background(), nil, zap.NewNop())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := s.Get("nonexistent", "*"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestRefresh_LoadsCache(t *testing.T) {
	db := &fakeDB{rows: []fakeRow{
		{"triage", "*", "v2-triage"},
	}}
	s, err := New(context.Background(), db, zap.NewNop())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := s.Get("triage", "anything"); got != "v2-triage" {
		t.Fatalf("after refresh: got %q", got)
	}
	if got := db.queries.Load(); got != 1 {
		t.Fatalf("expected 1 query, got %d", got)
	}
}

func TestRefresh_ReplacesCacheEntirely(t *testing.T) {
	db := &fakeDB{rows: []fakeRow{{"triage", "*", "v1"}}}
	s, err := New(context.Background(), db, zap.NewNop())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Simulate DB change: remove triage, add comms.
	db.rows = []fakeRow{{"comms", "*", "comms-v1"}}
	if err := s.refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	// triage should fall back to defaults (not stale cache).
	if got := s.Get("triage", "*"); !strings.Contains(got, "triage agent") {
		t.Fatalf("expected fallback to defaults after triage removed, got %q", got)
	}
	if got := s.Get("comms", "*"); got != "comms-v1" {
		t.Fatalf("expected comms-v1, got %q", got)
	}
}

func TestSetCache_TestHelper(t *testing.T) {
	s, _ := New(context.Background(), nil, zap.NewNop())
	s.setCache("triage", "claude-sonnet-4-6", "injected")
	if got := s.Get("triage", "claude-sonnet-4-6"); got != "injected" {
		t.Fatalf("setCache: got %q", got)
	}
}

func TestStartRefresh_RespectsContextCancel(t *testing.T) {
	db := &fakeDB{rows: []fakeRow{{"triage", "*", "x"}}}
	s, _ := New(context.Background(), db, zap.NewNop())
	s.RefreshInterval = 10 * time.Millisecond
	t.Cleanup(s.StopRefresh)

	ctx, cancel := context.WithCancel(context.Background())
	s.StartRefresh(ctx)
	time.Sleep(35 * time.Millisecond)
	cancel()
	time.Sleep(20 * time.Millisecond)

	// Refresh should have run at least twice during the 35ms window
	// (1 initial from New + at least 1 from the ticker).
	if got := db.queries.Load(); got < 2 {
		t.Fatalf("expected >=2 queries from refresh loop, got %d", got)
	}
}

func TestStartRefresh_NoOpWhenNilDB(t *testing.T) {
	s, _ := New(context.Background(), nil, zap.NewNop())
	// Should not panic; goroutine should exit immediately.
	s.StartRefresh(context.Background())
}

func TestStartRefresh_NonPositiveIntervalDoesNotPanic(t *testing.T) {
	db := &fakeDB{rows: []fakeRow{{"triage", "*", "x"}}}
	s, _ := New(context.Background(), db, zap.NewNop())
	s.RefreshInterval = 0

	ctx, cancel := context.WithCancel(context.Background())
	s.StartRefresh(ctx)
	cancel()
	s.StopRefresh()
}

func TestStartRefresh_CanBeCalledRepeatedly(t *testing.T) {
	db := &fakeDB{rows: []fakeRow{{"triage", "*", "x"}}}
	s, _ := New(context.Background(), db, zap.NewNop())
	s.RefreshInterval = time.Hour
	t.Cleanup(s.StopRefresh)

	ctx := context.Background()
	s.StartRefresh(ctx)
	firstCancel := s.refreshCancel
	if firstCancel == nil {
		t.Fatal("expected first refresh cancel func")
	}

	s.StartRefresh(ctx)
	if s.refreshCancel == nil {
		t.Fatal("expected replacement refresh cancel func")
	}
}

func TestRefresh_QueryError(t *testing.T) {
	db := &fakeDB{rows: []fakeRow{{"triage", "*", "v1"}}}
	s, _ := New(context.Background(), db, zap.NewNop())
	// Inject a query-level error on the next refresh.
	db.failErr = errors.New("connection lost")
	if err := s.refresh(context.Background()); err == nil {
		t.Fatal("expected refresh error")
	}
}

func TestDefaults_AllFiveAgentsPresent(t *testing.T) {
	want := []string{"supervisor", "triage", "rca", "runbook", "comms"}
	for _, a := range want {
		if _, ok := defaults[cacheKey(a, WildcardModel)]; !ok {
			t.Errorf("missing default for agent %q", a)
		}
	}
}
