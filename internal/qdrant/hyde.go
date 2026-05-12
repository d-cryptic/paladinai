// hyde.go implements the HyDE (Hypothetical Document Embeddings) retriever from
// docs/plans/06.rag-hybrid-search-stage6.md §7.
//
// HyDE generates a hypothetical answer to the user's query and embeds that text
// instead of the raw query. The hypothetical text is closer in embedding space
// to real answer documents than the original question, which boosts recall for
// natural-language queries against postmortem / runbook corpora.
package qdrant

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// HyDERetriever generates a hypothetical answer document for a query, used as
// an embedding seed in place of (or alongside) the raw query.
type HyDERetriever struct {
	gateway    ReflectionGateway
	modelTierA string
	maxTokens  int
}

// NewHyDERetriever creates a HyDERetriever using the given gateway.
func NewHyDERetriever(gateway ReflectionGateway) *HyDERetriever {
	return &HyDERetriever{
		gateway:    gateway,
		modelTierA: "claude-haiku-4-5",
		maxTokens:  256,
	}
}

// GenerateHypothetical generates a hypothetical document for the query.
// Returns (hypotheticalText, hydeUsed, error).
// hydeUsed is false when the gateway fails and we fall back to the raw query.
func (h *HyDERetriever) GenerateHypothetical(ctx context.Context, query string) (string, bool, error) {
	prompt := fmt.Sprintf(
		"Write a brief factual answer to this question as if from a postmortem/runbook. Be specific.\n\nQuestion: %s\n\nAnswer:",
		query,
	)

	resp, err := h.gateway.Complete(ctx, &CompletionRequest{
		Model:     h.modelTierA,
		MaxTokens: h.maxTokens,
		Prompt:    prompt,
	})
	if err != nil {
		// Fail open: fall back to the raw query so retrieval is not blocked
		// by a transient LLM outage.
		return query, false, nil
	}

	text := strings.TrimSpace(resp.Text)
	if text == "" {
		return query, false, nil
	}
	return text, true, nil
}

// ConditionalHyDE returns true if HyDE should be used based on initial
// retrieval score + query heuristics. See plan §22.
//
// initialTopScore is the score of the top-1 dense result (0 if no results).
func ConditionalHyDE(query string, initialTopScore float32) bool {
	if initialTopScore < 0.55 {
		return true
	}
	hasQuestion := containsAny(query, []string{"why", "what caused", "how did", "what is", "explain"})
	hasExact := hasExactTerms(query)
	return hasQuestion && !hasExact
}

// containsAny reports whether the lowercased query contains any of the
// provided needles (which should be lowercase).
func containsAny(query string, needles []string) bool {
	lower := strings.ToLower(query)
	for _, n := range needles {
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
}

// errorCodeRe matches mixed letter+digit tokens like OOMKill, HTTP5xx, E1234.
var errorCodeRe = regexp.MustCompile(`[A-Za-z]+\d+[A-Za-z0-9]*|[A-Z]{2,}\s?\d+[A-Za-z]*`)

// ipRe matches dotted-quad IPv4 addresses.
var ipRe = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)

// serviceSuffixes are common service-name suffixes used in infra incident text.
var serviceSuffixes = []string{"-api", "-service", "-db"}

// hasExactTerms reports whether the query contains tokens that look like
// concrete technical identifiers — error codes, service names, or IPs —
// which usually retrieve well without HyDE expansion.
func hasExactTerms(query string) bool {
	lower := strings.ToLower(query)
	for _, s := range serviceSuffixes {
		if strings.Contains(lower, s) {
			return true
		}
	}
	if errorCodeRe.MatchString(query) {
		return true
	}
	if ipRe.MatchString(query) {
		return true
	}
	return false
}
