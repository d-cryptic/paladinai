// stepback.go implements Step-Back Prompting from §14 of
// docs/plans/06.rag-hybrid-search-stage6.md.
//
// StepBackPrompter rewrites a concrete symptom question into a more abstract
// query that surfaces architectural context and known patterns. On gateway
// failure the original query is returned with used=false (fail-open).
package qdrant

import (
	"context"
	"fmt"
	"strings"
)

// StepBackPrompter generates an abstract "step-back" version of a symptom query.
// The step-back query retrieves architectural context + known patterns.
type StepBackPrompter struct {
	gateway    ReflectionGateway
	modelTierB string
	maxTokens  int
}

// NewStepBackPrompter creates a StepBackPrompter wired to the given gateway.
func NewStepBackPrompter(gateway ReflectionGateway) *StepBackPrompter {
	return &StepBackPrompter{
		gateway:    gateway,
		modelTierB: "qwen3-8b",
		maxTokens:  256,
	}
}

// GenerateStepBack produces an abstract version of a concrete symptom query.
// Returns (stepBackQuery, used, error).
// used=false means the gateway failed and we return just the original query.
func (s *StepBackPrompter) GenerateStepBack(ctx context.Context, symptomQuery string) (string, bool, error) {
	prompt := fmt.Sprintf(
		"Convert this specific symptom question into a more general, abstract query\n"+
			"that would help find architectural context and known patterns.\n"+
			"Symptom: %s\n\n"+
			"Respond with only the abstract query, nothing else.",
		symptomQuery,
	)

	resp, err := s.gateway.Complete(ctx, &CompletionRequest{
		Model:     s.modelTierB,
		MaxTokens: s.maxTokens,
		Prompt:    prompt,
	})
	if err != nil {
		return symptomQuery, false, nil
	}

	text := strings.TrimSpace(resp.Text)
	if text == "" {
		return symptomQuery, false, nil
	}
	return text, true, nil
}

// IsSymptomQuery returns true if the query is a specific symptom (benefits from step-back).
// Heuristics: contains "why is", "why does", "why did", "what caused", "how do I fix",
// or ends with "?" — indicating a concrete question rather than a lookup.
func IsSymptomQuery(query string) bool {
	trimmed := strings.TrimSpace(query)
	if strings.HasSuffix(trimmed, "?") {
		return true
	}
	lowered := strings.ToLower(trimmed)
	for _, marker := range []string{
		"why is",
		"why does",
		"why did",
		"what caused",
		"how do i fix",
	} {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}
