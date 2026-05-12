// selfrag.go implements the Self-RAG reflection stage from
// docs/plans/06.rag-hybrid-search-stage6.md §8.
//
// Retrieved candidates are scored by an LLM on three axes:
//   - IsRelevant: chunk addresses the query topic
//   - IsSupported: chunk contains enough information to contribute to an answer
//   - IsUseful: chunk adds marginal value given the other retrieved chunks
//
// Chunks that fail any axis are filtered out before synthesis. On LLM failure
// the reflector fails open (all chunks pass) to preserve retrieval quality.
package qdrant

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ReflectionScore holds the three Self-RAG token verdicts for one candidate.
type ReflectionScore struct {
	PointID     string
	IsRelevant  bool   // IsREL: chunk addresses the query topic
	IsSupported bool   // IsSUP: chunk contains enough info to contribute to an answer
	IsUseful    bool   // IsUSE: chunk adds marginal value over already-included chunks
	Reason      string // one-sentence explanation, for observability
}

// Passes reports whether this chunk cleared all three reflection gates.
func (r ReflectionScore) Passes() bool {
	return r.IsRelevant && r.IsSupported && r.IsUseful
}

// CompletionRequest is the minimum interface needed to call a TierA LLM for reflection.
type CompletionRequest struct {
	Model     string
	MaxTokens int
	Prompt    string
}

// CompletionResponse carries the LLM response text.
type CompletionResponse struct {
	Text string
}

// ReflectionGateway is the narrow LLM interface used by SelfRAGReflector.
// Satisfied by the real paladin-gateway client; easy to fake in tests.
type ReflectionGateway interface {
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
}

// SelfRAGReflector scores retrieved candidates and filters out irrelevant chunks.
type SelfRAGReflector struct {
	gateway ReflectionGateway
	// modelTierA is the LLM model used for reflection (cheap, fast).
	modelTierA string
	// maxTokens caps the reflection response length.
	maxTokens int
}

// NewSelfRAGReflector creates a SelfRAGReflector using the given gateway.
func NewSelfRAGReflector(gateway ReflectionGateway) *SelfRAGReflector {
	return &SelfRAGReflector{
		gateway:    gateway,
		modelTierA: "claude-haiku-4-5",
		maxTokens:  512,
	}
}

// ScoreChunks classifies each candidate on IsREL/IsSUP/IsUSE and returns
// only the candidates that pass all three gates. On gateway failure all
// candidates are returned unchanged (fail-open).
func (r *SelfRAGReflector) ScoreChunks(
	ctx context.Context,
	query string,
	candidates []Candidate,
) ([]Candidate, []ReflectionScore, error) {
	if len(candidates) == 0 {
		return nil, nil, nil
	}

	prompt := buildReflectionPrompt(query, candidates)

	resp, err := r.gateway.Complete(ctx, &CompletionRequest{
		Model:     r.modelTierA,
		MaxTokens: r.maxTokens,
		Prompt:    prompt,
	})
	if err != nil {
		// Fail open: pass all candidates through so retrieval is not broken
		// by a transient LLM outage.
		scores := make([]ReflectionScore, len(candidates))
		for i, c := range candidates {
			scores[i] = ReflectionScore{
				PointID:     c.ID,
				IsRelevant:  true,
				IsSupported: true,
				IsUseful:    true,
				Reason:      "reflection skipped: gateway error",
			}
		}
		return candidates, scores, nil
	}

	scores := parseReflectionScores(resp.Text, candidates)

	var passing []Candidate
	for i, c := range candidates {
		if scores[i].Passes() {
			passing = append(passing, c)
		}
	}

	return passing, scores, nil
}

// buildReflectionPrompt builds the batch reflection prompt for the LLM.
// Format asks the model to rate each chunk on three binary axes.
func buildReflectionPrompt(query string, candidates []Candidate) string {
	var sb strings.Builder
	fmt.Fprintf(&sb,
		"Query: %s\n\n"+
			"For each retrieved chunk below, classify it on three axes:\n"+
			"  IsREL (relevant): does this chunk address the query topic?\n"+
			"  IsSUP (supported): does this chunk contain enough info to contribute to an answer?\n"+
			"  IsUSE (useful): does this chunk add marginal value over the others?\n\n"+
			"Respond ONLY in this exact format, one line per chunk:\n"+
			"  CHUNK_N: IsREL=true/false IsSUP=true/false IsUSE=true/false Reason=<one sentence>\n\n",
		query,
	)
	for i, c := range candidates {
		rawText, _ := c.Payload["raw_text"].(string)
		if rawText == "" {
			rawText = fmt.Sprintf("[chunk %d — no text payload]", i+1)
		}
		if len(rawText) > 600 {
			rawText = rawText[:600] + "..."
		}
		fmt.Fprintf(&sb, "CHUNK_%d:\n%s\n\n", i+1, rawText)
	}
	return sb.String()
}

// chunkLineRe matches "CHUNK_N: IsREL=true/false IsSUP=true/false IsUSE=true/false [Reason=...]"
var chunkLineRe = regexp.MustCompile(
	`(?i)CHUNK_(\d+):\s*IsREL=(true|false)\s+IsSUP=(true|false)\s+IsUSE=(true|false)(?:\s+Reason=(.*))?`,
)

// parseReflectionScores parses the LLM response into one ReflectionScore per candidate.
// Any chunk whose line is missing or unparseable defaults to all-true (fail-open per chunk).
func parseReflectionScores(response string, candidates []Candidate) []ReflectionScore {
	scores := make([]ReflectionScore, len(candidates))
	for i, c := range candidates {
		scores[i] = ReflectionScore{
			PointID:     c.ID,
			IsRelevant:  true,
			IsSupported: true,
			IsUseful:    true,
			Reason:      "parse fallback: line not found",
		}
	}

	for _, line := range strings.Split(response, "\n") {
		m := chunkLineRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		idx, err := strconv.Atoi(m[1])
		if err != nil || idx < 1 || idx > len(candidates) {
			continue
		}
		scores[idx-1] = ReflectionScore{
			PointID:     candidates[idx-1].ID,
			IsRelevant:  strings.EqualFold(m[2], "true"),
			IsSupported: strings.EqualFold(m[3], "true"),
			IsUseful:    strings.EqualFold(m[4], "true"),
			Reason:      strings.TrimSpace(m[5]),
		}
	}

	return scores
}
