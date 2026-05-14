// Package memory implements the Stage 5 Agentic RAG Router: routes queries
// across multiple memory backends (FalkorDB, Zep, Qdrant, Valkey) based on
// query intent, and fuses the results via Reciprocal Rank Fusion.
package memory

import (
	"sort"
	"strings"
)

// MemoryType classifies the kind of memory being accessed.
type MemoryType string

const (
	MemoryTypeWorking       MemoryType = "working"        // in-session Valkey
	MemoryTypeEpisodic      MemoryType = "episodic"       // Zep bi-temporal
	MemoryTypeSemantic      MemoryType = "semantic"       // Qdrant dense facts
	MemoryTypeProcedural    MemoryType = "procedural"     // Qdrant runbooks/steps
	MemoryTypeSemanticGraph MemoryType = "semantic_graph" // FalkorDB topology
)

// MemoryPath describes one backend query path for a given memory operation.
type MemoryPath struct {
	Backend    string // "falkordb" | "zep" | "qdrant-semantic" | "qdrant-proc" | "valkey"
	MemoryType MemoryType
	Params     map[string]any
}

// Query is the input to the RAG router.
type Query struct {
	Text              string
	AgentType         string // "classifier" | "triage" | "rca" | "supervisor"
	Domain            string // e.g. "payments", "infrastructure"
	TemporalAnchor    string // ISO-8601 timestamp for bi-temporal queries
	RequiresTopology  bool
	RequiresProcedure bool
	RequiresTemporal  bool
	RequiresFacts     bool
	Trivial           bool // true for simple in-session lookups
}

// MemoryResult is a single result returned from a memory backend.
type MemoryResult struct {
	ID      string
	Content string
	Score   float64
	Backend string
	Type    MemoryType
}

// RouteQuery determines which memory backends to query for the given Query.
// Multiple paths may be returned; the caller should query each in parallel.
func RouteQuery(q Query) []MemoryPath {
	var paths []MemoryPath

	// Multi-hop topology reasoning → FalkorDB
	if q.RequiresTopology || containsAny(q.Text, "depends on", "upstream", "blast radius", "critical path") {
		paths = append(paths, MemoryPath{
			Backend:    "falkordb",
			MemoryType: MemoryTypeSemanticGraph,
			Params:     map[string]any{"max_hops": 5},
		})
	}

	// Runbook / procedure lookup → Qdrant procedural
	if q.RequiresProcedure || containsAny(q.Text, "runbook", "how to", "steps to", "playbook") {
		paths = append(paths, MemoryPath{
			Backend:    "qdrant-proc",
			MemoryType: MemoryTypeProcedural,
			Params:     map[string]any{"use_colbert": true, "self_rag": true},
		})
	}

	// Temporal / what changed → Zep bi-temporal
	if q.RequiresTemporal || containsAny(q.Text, "what changed", "before the incident", "at the time", "last week") {
		paths = append(paths, MemoryPath{
			Backend:    "zep",
			MemoryType: MemoryTypeEpisodic,
			Params:     map[string]any{"valid_at": q.TemporalAnchor, "decay_domain": q.Domain},
		})
	}

	// Semantic fact retrieval → Qdrant semantic
	if q.RequiresFacts {
		paths = append(paths, MemoryPath{
			Backend:    "qdrant-semantic",
			MemoryType: MemoryTypeSemantic,
		})
	}

	// Trivial or no specific intent → working memory only
	if len(paths) == 0 || q.Trivial {
		return []MemoryPath{{Backend: "valkey", MemoryType: MemoryTypeWorking}}
	}

	// RCA agent always gets episodic memory added (past incidents = essential context).
	if q.AgentType == "rca" {
		if !hasType(paths, MemoryTypeEpisodic) {
			paths = append(paths, MemoryPath{
				Backend:    "zep",
				MemoryType: MemoryTypeEpisodic,
				Params:     map[string]any{"decay_domain": q.Domain, "include_most_recent": true},
			})
		}
	}

	return paths
}

// FuseResults merges multiple ranked result lists into a single ranked list
// using Reciprocal Rank Fusion (k=60). topK caps the output.
func FuseResults(results [][]MemoryResult, topK int) []MemoryResult {
	const k = 60
	scores := make(map[string]float64)
	byID := make(map[string]MemoryResult)

	for _, list := range results {
		for rank, r := range list {
			scores[r.ID] += 1.0 / float64(k+rank+1)
			if _, exists := byID[r.ID]; !exists {
				byID[r.ID] = r
			}
		}
	}

	type scored struct {
		id    string
		score float64
	}
	all := make([]scored, 0, len(scores))
	for id, s := range scores {
		all = append(all, scored{id, s})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].score > all[j].score })

	if topK > len(all) {
		topK = len(all)
	}
	out := make([]MemoryResult, topK)
	for i, s := range all[:topK] {
		r := byID[s.id]
		r.Score = s.score
		out[i] = r
	}
	return out
}

func containsAny(text string, terms ...string) bool {
	lower := strings.ToLower(text)
	for _, t := range terms {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

func hasType(paths []MemoryPath, mt MemoryType) bool {
	for _, p := range paths {
		if p.MemoryType == mt {
			return true
		}
	}
	return false
}
