package memory

import (
	"testing"
)

func TestRouteQuery_TopologyQuery_UsesFalkorDB(t *testing.T) {
	q := Query{Text: "what depends on the payments service upstream"}
	paths := RouteQuery(q)
	if !hasBackend(paths, "falkordb") {
		t.Error("topology query should route to falkordb")
	}
}

func TestRouteQuery_RunbookQuery_UsesQdrantProc(t *testing.T) {
	q := Query{Text: "show me the runbook for database connection pool exhaustion"}
	paths := RouteQuery(q)
	if !hasBackend(paths, "qdrant-proc") {
		t.Error("runbook query should route to qdrant-proc")
	}
}

func TestRouteQuery_TemporalQuery_UsesZep(t *testing.T) {
	q := Query{Text: "what changed before the incident started last week"}
	paths := RouteQuery(q)
	if !hasBackend(paths, "zep") {
		t.Error("temporal query should route to zep")
	}
}

func TestRouteQuery_FactsRequired_UsesQdrantSemantic(t *testing.T) {
	q := Query{RequiresFacts: true}
	paths := RouteQuery(q)
	if !hasBackend(paths, "qdrant-semantic") {
		t.Error("facts query should route to qdrant-semantic")
	}
}

func TestRouteQuery_TrivialQuery_UsesValkey(t *testing.T) {
	q := Query{Trivial: true, Text: "something important", RequiresFacts: true}
	paths := RouteQuery(q)
	if len(paths) != 1 || paths[0].Backend != "valkey" {
		t.Errorf("trivial query should use only valkey, got %+v", paths)
	}
}

func TestRouteQuery_EmptyQuery_FallsBackToValkey(t *testing.T) {
	q := Query{Text: "hello"}
	paths := RouteQuery(q)
	if len(paths) != 1 || paths[0].Backend != "valkey" {
		t.Errorf("empty query should fall back to valkey, got %+v", paths)
	}
}

func TestRouteQuery_RCAAgent_AlwaysGetsEpisodic(t *testing.T) {
	q := Query{
		AgentType:    "rca",
		RequiresFacts: true,
		Text:         "what caused the OOM kill in payments",
	}
	paths := RouteQuery(q)
	if !hasBackend(paths, "zep") {
		t.Error("RCA agent should always get episodic (zep) path")
	}
}

func TestRouteQuery_RCAAgent_NoEpisodicDuplicate(t *testing.T) {
	// If query already routes to zep, don't add a duplicate path.
	q := Query{
		AgentType:        "rca",
		RequiresTemporal: true,
		Text:             "what changed before the incident",
	}
	paths := RouteQuery(q)
	zepCount := 0
	for _, p := range paths {
		if p.Backend == "zep" {
			zepCount++
		}
	}
	if zepCount > 1 {
		t.Errorf("expected at most 1 zep path, got %d", zepCount)
	}
}

func TestRouteQuery_MultipleIntents_MultiBackend(t *testing.T) {
	q := Query{
		Text:             "show me the runbook and what changed before the incident",
		RequiresProcedure: true,
		RequiresTemporal:  true,
	}
	paths := RouteQuery(q)
	if !hasBackend(paths, "qdrant-proc") || !hasBackend(paths, "zep") {
		t.Errorf("multi-intent query should use both qdrant-proc and zep, got %+v", paths)
	}
}

func TestFuseResults_SingleList_PreservesOrder(t *testing.T) {
	results := [][]MemoryResult{
		{
			{ID: "a", Content: "A", Backend: "zep"},
			{ID: "b", Content: "B", Backend: "zep"},
			{ID: "c", Content: "C", Backend: "zep"},
		},
	}
	fused := FuseResults(results, 3)
	if len(fused) != 3 {
		t.Fatalf("expected 3 results, got %d", len(fused))
	}
	// First result should have highest RRF score.
	if fused[0].ID != "a" {
		t.Errorf("expected 'a' first, got %q", fused[0].ID)
	}
}

func TestFuseResults_MultiList_MergesAndDeduplicates(t *testing.T) {
	list1 := []MemoryResult{
		{ID: "a", Content: "A"},
		{ID: "b", Content: "B"},
	}
	list2 := []MemoryResult{
		{ID: "b", Content: "B"}, // duplicate — should be merged
		{ID: "c", Content: "C"},
	}
	fused := FuseResults([][]MemoryResult{list1, list2}, 10)
	if len(fused) != 3 {
		t.Errorf("expected 3 unique results, got %d", len(fused))
	}
	// "b" appears in both lists → should be ranked higher (higher RRF score).
	if fused[0].ID != "b" {
		t.Errorf("expected 'b' (appears in both lists) to be ranked first, got %q", fused[0].ID)
	}
}

func TestFuseResults_TopKClamps(t *testing.T) {
	results := [][]MemoryResult{
		{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}},
	}
	fused := FuseResults(results, 2)
	if len(fused) != 2 {
		t.Errorf("expected 2 results (topK=2), got %d", len(fused))
	}
}

func TestFuseResults_Empty(t *testing.T) {
	fused := FuseResults(nil, 10)
	if len(fused) != 0 {
		t.Errorf("expected 0 results for empty input, got %d", len(fused))
	}
}

func TestFuseResults_ScoresDecreasing(t *testing.T) {
	results := [][]MemoryResult{
		{{ID: "a"}, {ID: "b"}, {ID: "c"}},
	}
	fused := FuseResults(results, 3)
	for i := 1; i < len(fused); i++ {
		if fused[i].Score > fused[i-1].Score {
			t.Errorf("scores not monotone decreasing: index %d (%.6f) > index %d (%.6f)", i, fused[i].Score, i-1, fused[i-1].Score)
		}
	}
}

func hasBackend(paths []MemoryPath, backend string) bool {
	for _, p := range paths {
		if p.Backend == backend {
			return true
		}
	}
	return false
}
