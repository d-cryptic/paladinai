// retrieval.go implements the post-retrieval ranking algorithms from
// docs/plans/06.rag-hybrid-search-stage6.md:
//
//   - RRF (Reciprocal Rank Fusion): merges dense and sparse result lists
//   - MMR (Maximal Marginal Relevance): diversity-aware re-ranking
//   - LongContextReorder: edge-first reordering for LLM context window
//   - ShouldUseHyDE: heuristic for activating HyDE query expansion
package qdrant

import (
	"math"
	"sort"
	"strings"
)

// ─── Ranked candidate ─────────────────────────────────────────────────────────

// Candidate is a retrieval result with a relevance score and dense vector.
// The Vector field is required for MMR; it may be nil for results that only
// need RRF fusion (no diversity re-ranking).
type Candidate struct {
	ID      string
	Score   float32
	Vector  []float32      // dense embedding for MMR cosine similarity
	Payload map[string]any // metadata (source, chunk_index, updated_at, …)
}

// ─── RRF — Reciprocal Rank Fusion ─────────────────────────────────────────────

// RRFConfig controls RRF behavior.
type RRFConfig struct {
	// K is the rank-smoothing constant. Typical value: 60.
	K int
}

// DefaultRRFConfig is the spec-recommended RRF configuration.
var DefaultRRFConfig = RRFConfig{K: 60}

// RRF fuses two ranked lists (e.g. dense and sparse) using Reciprocal Rank Fusion.
// Each result receives score = Σ 1/(K + rank_i(d)) across all lists.
// Results are returned sorted by descending fused score.
//
// The two lists may contain overlapping IDs (same document in both dense and
// sparse results). Duplicates are merged by summing their RRF contributions.
func RRF(denseResults, sparseResults []Candidate, cfg RRFConfig) []Candidate {
	if cfg.K <= 0 {
		cfg.K = DefaultRRFConfig.K
	}
	scores := make(map[string]float64)
	payloads := make(map[string]map[string]any)
	vectors := make(map[string][]float32)

	applyList := func(results []Candidate) {
		for rank, c := range results {
			scores[c.ID] += 1.0 / float64(cfg.K+rank+1)
			if _, exists := payloads[c.ID]; !exists {
				payloads[c.ID] = c.Payload
				vectors[c.ID] = c.Vector
			}
		}
	}
	applyList(denseResults)
	applyList(sparseResults)

	fused := make([]Candidate, 0, len(scores))
	for id, score := range scores {
		fused = append(fused, Candidate{
			ID:      id,
			Score:   float32(score),
			Payload: payloads[id],
			Vector:  vectors[id],
		})
	}
	sort.Slice(fused, func(i, j int) bool {
		return fused[i].Score > fused[j].Score
	})
	return fused
}

// ─── MMR — Maximal Marginal Relevance ─────────────────────────────────────────

// MMRConfig controls MMR behavior.
type MMRConfig struct {
	// Lambda trades off relevance vs. diversity. Range [0,1].
	// λ=1.0 → pure relevance (no diversity penalty)
	// λ=0.0 → pure diversity (picks most dissimilar documents)
	// Default: 0.7 (spec recommendation for general queries)
	Lambda float32
	// TopK is the number of results to return.
	TopK int
}

// DefaultMMRConfig is the spec-recommended MMR configuration.
var DefaultMMRConfig = MMRConfig{Lambda: 0.7, TopK: 20}

// MMR performs Maximal Marginal Relevance re-ranking.
// Requires candidates to have non-nil Vector fields.
// QueryVector is the embedding of the original query.
//
// Algorithm: iteratively picks the candidate that maximizes:
//
//	score_mmr(d) = λ × sim(query, d) − (1−λ) × max_sim(d, already_selected)
//
// Returns at most min(len(candidates), TopK) results.
// Candidates without vectors are passed through with score unchanged.
func MMR(candidates []Candidate, queryVector []float32, cfg MMRConfig) []Candidate {
	if cfg.Lambda < 0 || cfg.Lambda > 1 {
		cfg.Lambda = DefaultMMRConfig.Lambda
	}
	if cfg.TopK <= 0 {
		cfg.TopK = DefaultMMRConfig.TopK
	}
	if len(candidates) == 0 {
		return nil
	}

	selected := make([]Candidate, 0, cfg.TopK)
	remaining := make([]Candidate, len(candidates))
	copy(remaining, candidates)

	for len(selected) < cfg.TopK && len(remaining) > 0 {
		bestIdx := -1
		bestScore := float32(math.Inf(-1))

		for i, c := range remaining {
			relevance := cosineSim(queryVector, c.Vector)
			diversity := float32(0)
			for _, sel := range selected {
				sim := cosineSim(c.Vector, sel.Vector)
				if sim > diversity {
					diversity = sim
				}
			}
			mmrScore := cfg.Lambda*relevance - (1-cfg.Lambda)*diversity
			if mmrScore > bestScore {
				bestScore = mmrScore
				bestIdx = i
			}
		}

		if bestIdx < 0 {
			break
		}
		chosen := remaining[bestIdx]
		chosen.Score = bestScore
		selected = append(selected, chosen)
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}
	return selected
}

// cosineSim computes cosine similarity between two vectors.
// Returns 0 if either vector is nil or has zero length.
func cosineSim(a, b []float32) float32 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// ─── Long Context Reorder ─────────────────────────────────────────────────────

// LongContextReorder reorders results so that the most relevant are at the edges
// (position 0 and N-1) and the least relevant are in the middle.
//
// Background: LLMs have "lost-in-the-middle" attention drop — information in the
// middle of a long context is recalled less reliably (Stanford 2023).
// Placing the best result at position 0 and second-best at the last position
// maximises recall for the two strongest signals.
//
// The input must already be sorted by descending relevance. Results are not
// further sorted — only reordered.
func LongContextReorder(results []Candidate) []Candidate {
	if len(results) <= 2 {
		return results
	}
	reordered := make([]Candidate, len(results))
	left, right := 0, len(results)-1
	for i, c := range results {
		if i%2 == 0 {
			reordered[left] = c
			left++
		} else {
			reordered[right] = c
			right--
		}
	}
	return reordered
}

// ─── HyDE query classification ────────────────────────────────────────────────

// questionWords are natural-language question prefixes that suggest HyDE is beneficial.
var questionWords = []string{
	"why", "what caused", "what happened",
	"how did", "when did", "why did",
}

// exactPatterns are technical terms that indicate HyDE should be skipped.
var exactPatterns = []string{
	"oomkilled", "error ", "runbook", "crashloop",
	"steps to", "how to", "exit code",
}

// ShouldUseHyDE returns true if the query is a natural-language question that
// would benefit from HyDE (Hypothetical Document Embeddings) expansion.
// Returns false for exact/technical lookup queries where HyDE may hurt precision.
func ShouldUseHyDE(query string) bool {
	lower := strings.ToLower(query)

	// Check exact patterns first — these override question words.
	for _, p := range exactPatterns {
		if strings.Contains(lower, p) {
			return false
		}
	}

	// Natural language question words → use HyDE.
	for _, w := range questionWords {
		if strings.Contains(lower, w) {
			return true
		}
	}

	return false
}
