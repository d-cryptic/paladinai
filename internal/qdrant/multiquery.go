// multiquery.go implements Multi-Query Expansion from §13 of
// docs/plans/06.rag-hybrid-search-stage6.md.
//
// MultiQueryExpander asks a TierB LLM to rephrase the user's query in
// several alternative ways so that the retriever has a better chance of
// matching documents that use different terminology. On gateway failure
// the expander returns only the original query (fail-open).
package qdrant

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// MultiQueryExpander generates query variants to improve recall.
type MultiQueryExpander struct {
	gateway     ReflectionGateway
	modelTierB  string
	maxTokens   int
	numVariants int // default 3
}

// NewMultiQueryExpander creates a MultiQueryExpander wired to the given gateway.
func NewMultiQueryExpander(gateway ReflectionGateway) *MultiQueryExpander {
	return &MultiQueryExpander{
		gateway:     gateway,
		modelTierB:  "qwen3-8b",
		maxTokens:   256,
		numVariants: 3,
	}
}

// variantLineRe matches "VARIANT_N: <text>".
var variantLineRe = regexp.MustCompile(`(?i)VARIANT_\d+:\s*(.+)`)

// Expand generates numVariants alternative phrasings of the query.
// Returns a slice of [original query] + [variants].
// On LLM failure, returns just [original query] (fail-open: still useful, no extra recall).
func (m *MultiQueryExpander) Expand(ctx context.Context, query string) ([]string, error) {
	prompt := fmt.Sprintf(
		"Generate %d alternative search queries for the following alert/incident query.\n"+
			"Each variant should use different terminology but seek the same information.\n"+
			"Original: %s\n\n"+
			"Respond in exactly this format:\n"+
			"VARIANT_1: <query variant>\n"+
			"VARIANT_2: <query variant>\n"+
			"VARIANT_3: <query variant>",
		m.numVariants, query,
	)

	resp, err := m.gateway.Complete(ctx, &CompletionRequest{
		Model:     m.modelTierB,
		MaxTokens: m.maxTokens,
		Prompt:    prompt,
	})
	if err != nil {
		// Fail-open: original query is always useful on its own.
		return []string{query}, nil
	}

	out := []string{query}
	for _, line := range strings.Split(resp.Text, "\n") {
		match := variantLineRe.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}
		variant := strings.TrimRight(match[1], " \t\r\n")
		if variant == "" {
			continue
		}
		out = append(out, variant)
	}
	return out, nil
}

// ShouldExpandQuery returns true if the query is likely to benefit from multi-query expansion.
// Long natural-language queries with > 5 words and no exact technical identifiers benefit most.
func ShouldExpandQuery(query string) bool {
	fields := strings.Fields(query)
	if len(fields) <= 5 {
		return false
	}
	lowered := strings.ToLower(query)
	for _, marker := range []string{"-api", "-service", "-db"} {
		if strings.Contains(lowered, marker) {
			return false
		}
	}
	return true
}
