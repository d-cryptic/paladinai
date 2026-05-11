package qdrant_test

import (
	"math"
	"testing"

	"github.com/paladinai/paladinai/internal/qdrant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

func mkCandidate(id string, score float32, vec []float32) qdrant.Candidate {
	return qdrant.Candidate{ID: id, Score: score, Vector: vec}
}

func normalize(v []float32) []float32 {
	var norm float32
	for _, x := range v {
		norm += x * x
	}
	norm = float32(math.Sqrt(float64(norm)))
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x / norm
	}
	return out
}

// ─── RRF ─────────────────────────────────────────────────────────────────────

func TestRRF_MergesDistinctLists(t *testing.T) {
	dense := []qdrant.Candidate{
		{ID: "a", Score: 0.9},
		{ID: "b", Score: 0.8},
		{ID: "c", Score: 0.7},
	}
	sparse := []qdrant.Candidate{
		{ID: "d", Score: 0.85},
		{ID: "b", Score: 0.75},
		{ID: "e", Score: 0.6},
	}
	fused := qdrant.RRF(dense, sparse, qdrant.DefaultRRFConfig)
	require.NotEmpty(t, fused)

	// "b" appears in both lists — it should accumulate more RRF score than "a" or "d"
	idxMap := make(map[string]int)
	for i, r := range fused {
		idxMap[r.ID] = i
	}
	// b has two rank contributions, a and d only one each — b should rank higher
	assert.Less(t, idxMap["b"], idxMap["a"])
	assert.Less(t, idxMap["b"], idxMap["d"])
}

func TestRRF_SortedDescending(t *testing.T) {
	dense := []qdrant.Candidate{{ID: "a", Score: 0.9}, {ID: "b", Score: 0.8}}
	sparse := []qdrant.Candidate{{ID: "c", Score: 0.7}, {ID: "d", Score: 0.6}}
	fused := qdrant.RRF(dense, sparse, qdrant.DefaultRRFConfig)
	for i := 1; i < len(fused); i++ {
		assert.GreaterOrEqual(t, fused[i-1].Score, fused[i].Score)
	}
}

func TestRRF_EmptyLists_ReturnsEmpty(t *testing.T) {
	assert.Empty(t, qdrant.RRF(nil, nil, qdrant.DefaultRRFConfig))
}

func TestRRF_OnlyDense_StillWorks(t *testing.T) {
	dense := []qdrant.Candidate{{ID: "a", Score: 0.9}, {ID: "b", Score: 0.8}}
	fused := qdrant.RRF(dense, nil, qdrant.DefaultRRFConfig)
	assert.Len(t, fused, 2)
}

func TestRRF_DefaultK60_ScoreFormula(t *testing.T) {
	// With K=60, rank-0 score = 1/(60+1) ≈ 0.01639
	dense := []qdrant.Candidate{{ID: "a", Score: 1.0}}
	fused := qdrant.RRF(dense, nil, qdrant.DefaultRRFConfig)
	require.Len(t, fused, 1)
	assert.InDelta(t, 1.0/61.0, float64(fused[0].Score), 1e-5)
}

func TestRRF_ZeroK_UsesDefault(t *testing.T) {
	dense := []qdrant.Candidate{{ID: "a", Score: 1.0}}
	fused := qdrant.RRF(dense, nil, qdrant.RRFConfig{K: 0})
	require.Len(t, fused, 1)
	assert.InDelta(t, 1.0/61.0, float64(fused[0].Score), 1e-5)
}

// ─── MMR ─────────────────────────────────────────────────────────────────────

func TestMMR_SelectsTopK(t *testing.T) {
	query := normalize([]float32{1, 0, 0})
	candidates := []qdrant.Candidate{
		mkCandidate("a", 0.9, normalize([]float32{1, 0, 0})),    // identical to query
		mkCandidate("b", 0.8, normalize([]float32{0.9, 0.1, 0})), // similar to query
		mkCandidate("c", 0.7, normalize([]float32{0, 1, 0})),    // orthogonal
		mkCandidate("d", 0.6, normalize([]float32{0, 0, 1})),    // orthogonal
		mkCandidate("e", 0.5, normalize([]float32{-1, 0, 0})),   // opposite
	}
	result := qdrant.MMR(candidates, query, qdrant.MMRConfig{Lambda: 0.7, TopK: 3})
	require.Len(t, result, 3)
}

func TestMMR_FirstSelectedIsHighestRelevance(t *testing.T) {
	// With empty selected set, first pick should be the most query-relevant.
	query := normalize([]float32{1, 0})
	candidates := []qdrant.Candidate{
		mkCandidate("best", 0.9, normalize([]float32{1, 0})),
		mkCandidate("ok", 0.6, normalize([]float32{0.5, 0.5})),
	}
	result := qdrant.MMR(candidates, query, qdrant.MMRConfig{Lambda: 0.7, TopK: 2})
	require.Len(t, result, 2)
	assert.Equal(t, "best", result[0].ID)
}

func TestMMR_DiversityPenalizesRedundant(t *testing.T) {
	// Two near-identical candidates + one with partial relevance.
	// After picking dup1, the diversity penalty on dup2 (cos sim = 1.0 to dup1) is:
	//   MMR(dup2) = λ×1.0 − (1−λ)×1.0 = λ − (1−λ)
	// The partial candidate has cos sim ≈ 0.6 to query and 0.6 to dup1:
	//   MMR(partial) = λ×0.6 − (1−λ)×0.6 = 0.6(λ − (1−λ))
	// With λ=0.3: MMR(dup2) = 0.3−0.7 = −0.4; MMR(partial) = 0.6×(0.3−0.7) = −0.24
	// partial wins because −0.24 > −0.4.
	query := normalize([]float32{1, 0, 0})
	dup1 := normalize([]float32{1, 0, 0})
	dup2 := normalize([]float32{1, 0, 0})     // identical to dup1 (heavily penalized)
	partial := normalize([]float32{0.6, 0.8, 0}) // some relevance, lower sim to dup1

	candidates := []qdrant.Candidate{
		mkCandidate("dup1", 0.9, dup1),
		mkCandidate("dup2", 0.85, dup2),
		mkCandidate("partial", 0.7, partial),
	}
	result := qdrant.MMR(candidates, query, qdrant.MMRConfig{Lambda: 0.3, TopK: 2})
	require.Len(t, result, 2)
	ids := []string{result[0].ID, result[1].ID}
	assert.Contains(t, ids, "dup1")
	assert.Contains(t, ids, "partial", "partial candidate should beat fully redundant dup2")
}

func TestMMR_EmptyCandidates_ReturnsNil(t *testing.T) {
	result := qdrant.MMR(nil, []float32{1, 0}, qdrant.DefaultMMRConfig)
	assert.Nil(t, result)
}

func TestMMR_TopKLargerThanCandidates_ReturnsAll(t *testing.T) {
	query := normalize([]float32{1, 0})
	candidates := []qdrant.Candidate{
		mkCandidate("a", 0.9, normalize([]float32{1, 0})),
		mkCandidate("b", 0.7, normalize([]float32{0, 1})),
	}
	result := qdrant.MMR(candidates, query, qdrant.MMRConfig{Lambda: 0.7, TopK: 100})
	assert.Len(t, result, 2)
}

// ─── LongContextReorder ───────────────────────────────────────────────────────

func TestLongContextReorder_TwoOrFewer_Unchanged(t *testing.T) {
	input := []qdrant.Candidate{{ID: "a"}, {ID: "b"}}
	output := qdrant.LongContextReorder(input)
	assert.Equal(t, input, output)
}

func TestLongContextReorder_FiveItems_BestAtEdges(t *testing.T) {
	// Input sorted best→worst: [a, b, c, d, e]
	// Expected edge-first reorder: [a, c, e, d, b]
	input := []qdrant.Candidate{
		{ID: "a", Score: 0.9},
		{ID: "b", Score: 0.8},
		{ID: "c", Score: 0.7},
		{ID: "d", Score: 0.6},
		{ID: "e", Score: 0.5},
	}
	result := qdrant.LongContextReorder(input)
	require.Len(t, result, 5)
	// Best (a) at position 0, second-best (b) at last position
	assert.Equal(t, "a", result[0].ID)
	assert.Equal(t, "b", result[len(result)-1].ID)
}

func TestLongContextReorder_PreservesAllItems(t *testing.T) {
	input := []qdrant.Candidate{
		{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "e"}, {ID: "f"},
	}
	result := qdrant.LongContextReorder(input)
	require.Len(t, result, 6)
	ids := make(map[string]bool)
	for _, c := range result {
		ids[c.ID] = true
	}
	assert.Len(t, ids, 6, "all items must be present after reorder")
}

func TestLongContextReorder_SingleItem(t *testing.T) {
	input := []qdrant.Candidate{{ID: "solo"}}
	result := qdrant.LongContextReorder(input)
	assert.Equal(t, input, result)
}

// ─── ShouldUseHyDE ────────────────────────────────────────────────────────────

func TestShouldUseHyDE_NaturalLanguageQuestion_True(t *testing.T) {
	queries := []string{
		"why did our p99 latency spike last Tuesday?",
		"what caused the December outage?",
		"what happened to payments-api last night?",
		"how did the cert expiry propagate?",
		"when did the alert first fire?",
		"why did the deployment fail?",
	}
	for _, q := range queries {
		assert.True(t, qdrant.ShouldUseHyDE(q), "expected HyDE for: %q", q)
	}
}

func TestShouldUseHyDE_TechnicalLookup_False(t *testing.T) {
	queries := []string{
		"OOMKilled error 137 runbook",
		"payments-api connection pool exhaustion steps",
		"kubernetes pod crashloopbackoff",
		"how to restart a pod",
		"steps to rollback deployment",
		"exit code 137 handler",
	}
	for _, q := range queries {
		assert.False(t, qdrant.ShouldUseHyDE(q), "expected no HyDE for: %q", q)
	}
}

func TestShouldUseHyDE_AmbiguousQuery_False(t *testing.T) {
	// Default is false for queries that don't match either pattern.
	assert.False(t, qdrant.ShouldUseHyDE("prometheus metrics dashboard"))
	assert.False(t, qdrant.ShouldUseHyDE(""))
}

func TestShouldUseHyDE_CaseInsensitive(t *testing.T) {
	assert.True(t, qdrant.ShouldUseHyDE("WHY did this fail?"))
	assert.False(t, qdrant.ShouldUseHyDE("RUNBOOK for oomkilled"))
}

func TestShouldUseHyDE_ExactPatternOverridesQuestion(t *testing.T) {
	// "why" is a question word but "runbook" is an exact pattern — exact wins.
	assert.False(t, qdrant.ShouldUseHyDE("why is there no runbook for this error?"))
}
