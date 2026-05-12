// crag.go implements Corrective RAG (CRAG) from
// docs/plans/06.rag-hybrid-search-stage6.md §17.
//
// CRAG asks an LLM to rate the relevance of a retrieved chunk set to the
// original query, then routes downstream behaviour based on the score:
//   - score > 0.7   → CRAGUse        (chunks are good, use as-is)
//   - 0.3 ≤ s ≤ 0.7 → CRAGSupplement (use chunks + supplement with external KB)
//   - score < 0.3   → CRAGReplace    (drop chunks, replace with external KB)
//
// On gateway failure CRAG fails open and returns CRAGUse with score=1.0.
package qdrant

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CRAGDecision is the downstream routing decision emitted by a CRAGEvaluator.
type CRAGDecision int

const (
	// CRAGUse — retrieved chunks are highly relevant; use as-is.
	CRAGUse CRAGDecision = iota
	// CRAGSupplement — partially relevant; combine with external knowledge.
	CRAGSupplement
	// CRAGReplace — not relevant; replace retrieval with external knowledge.
	CRAGReplace
)

// CRAGResult is the structured output of CRAGEvaluator.Evaluate.
type CRAGResult struct {
	Decision  CRAGDecision
	Score     float32 // LLM-reported relevance score 0.0-1.0
	Reasoning string  // one-sentence explanation
}

// CRAGEvaluator scores chunk-set relevance and emits a routing decision.
type CRAGEvaluator struct {
	gateway    ReflectionGateway
	modelTierB string
	maxTokens  int
}

// NewCRAGEvaluator creates a CRAGEvaluator using the given gateway.
func NewCRAGEvaluator(gateway ReflectionGateway) *CRAGEvaluator {
	return &CRAGEvaluator{
		gateway:    gateway,
		modelTierB: "qwen3-8b",
		maxTokens:  128,
	}
}

// Evaluate scores the relevance of retrieved chunks to the query.
// Returns a CRAGResult with the decision and score.
// On LLM failure, returns CRAGUse with score=1.0 (fail-open).
func (c *CRAGEvaluator) Evaluate(ctx context.Context, query string, chunks []Candidate) (CRAGResult, error) {
	prompt := buildCRAGPrompt(query, chunks)

	resp, err := c.gateway.Complete(ctx, &CompletionRequest{
		Model:     c.modelTierB,
		MaxTokens: c.maxTokens,
		Prompt:    prompt,
	})
	if err != nil {
		// Fail open: assume retrieval is good so we do not regress to
		// external-KB lookups on a transient LLM outage.
		return CRAGResult{
			Decision:  CRAGUse,
			Score:     1.0,
			Reasoning: "crag skipped: gateway error",
		}, nil
	}

	score, reasoning := parseCRAGResponse(resp.Text)
	return CRAGResult{
		Decision:  classifyCRAG(score),
		Score:     score,
		Reasoning: reasoning,
	}, nil
}

// buildCRAGPrompt builds the relevance-rating prompt for the LLM.
func buildCRAGPrompt(query string, chunks []Candidate) string {
	var sb strings.Builder
	fmt.Fprintf(&sb,
		"Rate the relevance of these retrieved chunks to the query on a scale of 0.0 to 1.0.\n"+
			"Query: %s\n\nChunks:\n",
		query,
	)
	for i, c := range chunks {
		rawText, _ := c.Payload["raw_text"].(string)
		if rawText == "" {
			rawText = fmt.Sprintf("[chunk %d — no text payload]", i+1)
		}
		if len(rawText) > 400 {
			rawText = rawText[:400]
		}
		fmt.Fprintf(&sb, "CHUNK_%d: %s\n", i+1, rawText)
	}
	sb.WriteString("\nRespond in exactly this format:\nSCORE: <float 0.0-1.0>\nREASONING: <one sentence>\n")
	return sb.String()
}

var (
	cragScoreRe     = regexp.MustCompile(`SCORE:\s*([\d.]+)`)
	cragReasoningRe = regexp.MustCompile(`REASONING:\s*(.+)`)
)

// parseCRAGResponse extracts the (score, reasoning) pair from the LLM response.
// Defaults to (1.0, "parse fallback") if the SCORE line is missing or malformed.
func parseCRAGResponse(text string) (float32, string) {
	score := float32(1.0)
	reasoning := "parse fallback: SCORE line not found"

	if m := cragScoreRe.FindStringSubmatch(text); m != nil {
		if v, err := strconv.ParseFloat(m[1], 32); err == nil {
			score = float32(v)
		}
	}
	if m := cragReasoningRe.FindStringSubmatch(text); m != nil {
		reasoning = strings.TrimSpace(m[1])
	}
	return score, reasoning
}

// classifyCRAG maps a relevance score to the routing decision.
func classifyCRAG(score float32) CRAGDecision {
	switch {
	case score > 0.7:
		return CRAGUse
	case score < 0.3:
		return CRAGReplace
	default:
		return CRAGSupplement
	}
}
