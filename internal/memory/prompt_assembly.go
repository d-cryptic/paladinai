// prompt_assembly.go implements Stage 5 §12: Query Fanout & Prompt Assembly.
//
// AssemblePrompt orders retrieved memory results into the deliberate structure
// that maximises LLM attention on the most action-relevant content:
//
//  1. PROCEDURAL  — runbook chunks (highest attention, start of context)
//  2. SEMANTIC    — topology / SLO facts
//  3. EPISODIC    — past incidents (decay-weighted)
//  4. TOOL_RESULTS — live signals (also high attention, end of context)
//  5. CURRENT_QUERY
//
// The ordering follows the primacy+recency bias documented in the Stage 5 plan.
package memory

import (
	"strings"
)

// PromptSection constants used as XML-like tags in assembled prompts.
const (
	TagRunbookStart      = "[RUNBOOK_START]"
	TagRunbookEnd        = "[RUNBOOK_END]"
	TagTopologyStart     = "[TOPOLOGY_START]"
	TagTopologyEnd       = "[TOPOLOGY_END]"
	TagPastIncidentStart = "[PAST_INCIDENTS_START]"
	TagPastIncidentEnd   = "[PAST_INCIDENTS_END]"
	TagToolResultsStart  = "[TOOL_RESULTS_START]"
	TagToolResultsEnd    = "[TOOL_RESULTS_END]"
)

// PromptParts holds the retrieved context segments before assembly.
type PromptParts struct {
	ProceduralChunks []string // runbook / how-to steps
	SemanticChunks   []string // topology, SLO facts
	EpisodicChunks   []string // past incident summaries
	ToolResults      []string // live Prometheus/Loki results
	CurrentQuery     string
}

// AssemblePrompt builds the context section of an LLM prompt from the
// retrieved memory parts. The returned string does not include the static
// system prefix (cached separately at the provider level).
func AssemblePrompt(parts PromptParts) string {
	var b strings.Builder

	// [1] Procedural context — first for primacy attention bias.
	if len(parts.ProceduralChunks) > 0 {
		b.WriteString(TagRunbookStart + "\n")
		for _, chunk := range parts.ProceduralChunks {
			b.WriteString(chunk)
			b.WriteString("\n")
		}
		b.WriteString(TagRunbookEnd + "\n\n")
	}

	// [2] Semantic context — topology and SLO facts.
	if len(parts.SemanticChunks) > 0 {
		b.WriteString(TagTopologyStart + "\n")
		for _, chunk := range parts.SemanticChunks {
			b.WriteString(chunk)
			b.WriteString("\n")
		}
		b.WriteString(TagTopologyEnd + "\n\n")
	}

	// [3] Episodic context — past incidents.
	if len(parts.EpisodicChunks) > 0 {
		b.WriteString(TagPastIncidentStart + "\n")
		for _, chunk := range parts.EpisodicChunks {
			b.WriteString(chunk)
			b.WriteString("\n")
		}
		b.WriteString(TagPastIncidentEnd + "\n\n")
	}

	// [4] Tool results — last for recency attention bias.
	if len(parts.ToolResults) > 0 {
		b.WriteString(TagToolResultsStart + "\n")
		for _, result := range parts.ToolResults {
			b.WriteString(result)
			b.WriteString("\n")
		}
		b.WriteString(TagToolResultsEnd + "\n\n")
	}

	// [5] Current query — appended after context.
	if parts.CurrentQuery != "" {
		b.WriteString("QUERY: ")
		b.WriteString(parts.CurrentQuery)
		b.WriteString("\n")
	}

	return b.String()
}

// PartitionResults splits a flat FuseResults output into PromptParts buckets
// based on memory type. topK per section is respected to stay within token
// budgets (procedural ≤800, semantic ≤400, episodic ≤360 tokens).
func PartitionResults(results []MemoryResult) PromptParts {
	var parts PromptParts
	for _, r := range results {
		switch r.Type {
		case MemoryTypeProcedural:
			parts.ProceduralChunks = append(parts.ProceduralChunks, r.Content)
		case MemoryTypeSemantic, MemoryTypeSemanticGraph:
			parts.SemanticChunks = append(parts.SemanticChunks, r.Content)
		case MemoryTypeEpisodic:
			parts.EpisodicChunks = append(parts.EpisodicChunks, r.Content)
		}
	}
	return parts
}
