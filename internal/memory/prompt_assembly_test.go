package memory

import (
	"strings"
	"testing"
)

func TestAssemblePrompt_AllSections(t *testing.T) {
	parts := PromptParts{
		ProceduralChunks: []string{"step 1: check logs", "step 2: restart pod"},
		SemanticChunks:   []string{"payments depends on db"},
		EpisodicChunks:   []string{"2024-01-10: OOM kill in payments"},
		ToolResults:      []string{"prometheus: cpu=95%"},
		CurrentQuery:     "why is payments down?",
	}
	got := AssemblePrompt(parts)

	// Primacy: procedural first
	runbookPos := strings.Index(got, TagRunbookStart)
	topologyPos := strings.Index(got, TagTopologyStart)
	episodicPos := strings.Index(got, TagPastIncidentStart)
	toolPos := strings.Index(got, TagToolResultsStart)
	queryPos := strings.Index(got, "QUERY:")

	if runbookPos < 0 {
		t.Error("missing RUNBOOK section")
	}
	if topologyPos < 0 {
		t.Error("missing TOPOLOGY section")
	}
	if episodicPos < 0 {
		t.Error("missing PAST_INCIDENTS section")
	}
	if toolPos < 0 {
		t.Error("missing TOOL_RESULTS section")
	}
	if queryPos < 0 {
		t.Error("missing QUERY line")
	}

	// Ordering: procedural < topology < episodic < tool_results < query
	if runbookPos >= topologyPos || topologyPos >= episodicPos || episodicPos >= toolPos || toolPos >= queryPos { //nolint:staticcheck
		t.Errorf("section ordering wrong: runbook=%d topology=%d episodic=%d tool=%d query=%d",
			runbookPos, topologyPos, episodicPos, toolPos, queryPos)
	}
}

func TestAssemblePrompt_EmptyParts_ReturnsEmpty(t *testing.T) {
	got := AssemblePrompt(PromptParts{})
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestAssemblePrompt_OnlyProcedural(t *testing.T) {
	parts := PromptParts{ProceduralChunks: []string{"restart nginx"}}
	got := AssemblePrompt(parts)
	if !strings.Contains(got, TagRunbookStart) {
		t.Error("missing RUNBOOK_START tag")
	}
	if !strings.Contains(got, TagRunbookEnd) {
		t.Error("missing RUNBOOK_END tag")
	}
	if !strings.Contains(got, "restart nginx") {
		t.Error("missing procedural content")
	}
	if strings.Contains(got, TagTopologyStart) {
		t.Error("unexpected TOPOLOGY section")
	}
}

func TestAssemblePrompt_OnlyToolResults(t *testing.T) {
	parts := PromptParts{ToolResults: []string{"cpu=80%"}}
	got := AssemblePrompt(parts)
	if !strings.Contains(got, TagToolResultsStart) {
		t.Error("missing TOOL_RESULTS_START")
	}
	if !strings.Contains(got, "cpu=80%") {
		t.Error("missing tool result content")
	}
}

func TestAssemblePrompt_QueryAppended(t *testing.T) {
	parts := PromptParts{CurrentQuery: "what is the blast radius?"}
	got := AssemblePrompt(parts)
	if !strings.Contains(got, "QUERY: what is the blast radius?") {
		t.Errorf("query not appended correctly, got: %q", got)
	}
}

func TestAssemblePrompt_MultipleChunks(t *testing.T) {
	parts := PromptParts{
		ProceduralChunks: []string{"chunk-a", "chunk-b", "chunk-c"},
	}
	got := AssemblePrompt(parts)
	for _, chunk := range parts.ProceduralChunks {
		if !strings.Contains(got, chunk) {
			t.Errorf("missing chunk %q in output", chunk)
		}
	}
}

func TestAssemblePrompt_TagsAreBalanced(t *testing.T) {
	parts := PromptParts{
		ProceduralChunks: []string{"x"},
		SemanticChunks:   []string{"y"},
		EpisodicChunks:   []string{"z"},
		ToolResults:      []string{"w"},
	}
	got := AssemblePrompt(parts)
	pairs := [][2]string{
		{TagRunbookStart, TagRunbookEnd},
		{TagTopologyStart, TagTopologyEnd},
		{TagPastIncidentStart, TagPastIncidentEnd},
		{TagToolResultsStart, TagToolResultsEnd},
	}
	for _, pair := range pairs {
		startPos := strings.Index(got, pair[0])
		endPos := strings.Index(got, pair[1])
		if startPos < 0 || endPos < 0 {
			t.Errorf("missing tag pair: %s / %s", pair[0], pair[1])
		}
		if startPos >= endPos {
			t.Errorf("end tag before start tag: %s / %s", pair[0], pair[1])
		}
	}
}

func TestPartitionResults_ByType(t *testing.T) {
	results := []MemoryResult{
		{ID: "1", Content: "runbook step", Type: MemoryTypeProcedural},
		{ID: "2", Content: "topology fact", Type: MemoryTypeSemantic},
		{ID: "3", Content: "graph node", Type: MemoryTypeSemanticGraph},
		{ID: "4", Content: "past incident", Type: MemoryTypeEpisodic},
		{ID: "5", Content: "working memory", Type: MemoryTypeWorking},
	}
	parts := PartitionResults(results)

	if len(parts.ProceduralChunks) != 1 || parts.ProceduralChunks[0] != "runbook step" {
		t.Errorf("procedural chunks wrong: %v", parts.ProceduralChunks)
	}
	if len(parts.SemanticChunks) != 2 {
		t.Errorf("expected 2 semantic chunks (semantic + semantic_graph), got %d", len(parts.SemanticChunks))
	}
	if len(parts.EpisodicChunks) != 1 || parts.EpisodicChunks[0] != "past incident" {
		t.Errorf("episodic chunks wrong: %v", parts.EpisodicChunks)
	}
}

func TestPartitionResults_WorkingMemory_NotPartitioned(t *testing.T) {
	results := []MemoryResult{
		{ID: "1", Content: "session cache", Type: MemoryTypeWorking},
	}
	parts := PartitionResults(results)
	if len(parts.ProceduralChunks)+len(parts.SemanticChunks)+len(parts.EpisodicChunks) != 0 {
		t.Error("working memory should not be partitioned into any chunk bucket")
	}
}

func TestPartitionResults_Empty(t *testing.T) {
	parts := PartitionResults(nil)
	if len(parts.ProceduralChunks) != 0 || len(parts.SemanticChunks) != 0 || len(parts.EpisodicChunks) != 0 {
		t.Error("empty input should produce empty PromptParts")
	}
}

func TestAssemblePrompt_RecencyBias_ToolResultsLast(t *testing.T) {
	parts := PromptParts{
		ProceduralChunks: []string{"proc"},
		ToolResults:      []string{"live signal"},
		CurrentQuery:     "query",
	}
	got := AssemblePrompt(parts)
	toolPos := strings.Index(got, TagToolResultsStart)
	runbookPos := strings.Index(got, TagRunbookStart)
	queryPos := strings.Index(got, "QUERY:")

	if toolPos < runbookPos {
		t.Error("tool results should come after runbook (recency bias)")
	}
	if queryPos < toolPos {
		t.Error("query should come after tool results")
	}
}
