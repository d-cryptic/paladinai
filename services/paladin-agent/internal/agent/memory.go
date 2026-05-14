package agent

import (
	"context"
	"strings"

	"github.com/paladinai/paladinai/internal/alert"
)

// MemorySpecialist recalls similar incidents and recurrence hints.
type MemorySpecialist interface {
	RecallMemory(ctx context.Context, env *alert.AlertEnvelope) (*MemoryResult, error)
}

// MemoryResult is the supervisor-facing memory route output.
type MemoryResult struct {
	Query          string   `json:"query"`
	MemoryTypes    []string `json:"memory_types"`
	Signals        []string `json:"signals"`
	SimilarPattern string   `json:"similar_pattern"`
	NeedsHuman     bool     `json:"needs_human"`
}

// StaticMemorySpecialist is a deterministic baseline for recurrence routing.
type StaticMemorySpecialist struct{}

// RecallMemory derives memory lookup intent from the alert without an LLM call.
func (StaticMemorySpecialist) RecallMemory(_ context.Context, env *alert.AlertEnvelope) (*MemoryResult, error) {
	text := strings.ToLower(strings.Join([]string{
		env.Title,
		env.Description,
		env.Labels["alertname"],
		env.Labels["service"],
		env.Labels["job"],
		env.Annotations["description"],
	}, " "))

	signals := memorySignals(text)
	return &MemoryResult{
		Query:          memoryQuery(env),
		MemoryTypes:    memoryTypesForSignals(signals),
		Signals:        signals,
		SimilarPattern: similarPattern(signals),
		NeedsHuman:     env.Severity == alert.SeverityP1 || env.Severity == alert.SeverityP2,
	}, nil
}

func memorySignals(text string) []string {
	signals := make([]string, 0, 4)
	if containsAny(text, "kafka", "consumer lag", "rebalance") {
		signals = append(signals, "kafka_rebalance_or_lag")
	}
	if containsAny(text, "recurring", "similar prior", "happened before", "repeat") {
		signals = append(signals, "recurring_incident")
	}
	if containsAny(text, "cache", "session", "stale") {
		signals = append(signals, "cache_or_session_pattern")
	}
	if containsAny(text, "deployment", "rollback", "regression") {
		signals = append(signals, "deployment_regression_pattern")
	}
	if len(signals) == 0 {
		signals = append(signals, "similar_incident_lookup")
	}
	return signals
}

func memoryTypesForSignals(signals []string) []string {
	types := []string{"episodic"}
	for _, signal := range signals {
		switch signal {
		case "cache_or_session_pattern":
			types = append(types, "working")
		case "deployment_regression_pattern":
			types = append(types, "semantic")
		case "kafka_rebalance_or_lag", "recurring_incident":
			types = append(types, "procedural")
		}
	}
	return dedupeStrings(types)
}

func similarPattern(signals []string) string {
	if len(signals) == 0 {
		return "similar_incident_lookup"
	}
	return strings.Join(signals, "+")
}

func memoryQuery(env *alert.AlertEnvelope) string {
	parts := []string{
		env.Labels["service"],
		env.Labels["job"],
		env.Labels["alertname"],
		env.Title,
		env.Annotations["description"],
	}
	var nonEmpty []string
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			nonEmpty = append(nonEmpty, strings.TrimSpace(part))
		}
	}
	if len(nonEmpty) == 0 {
		return "similar incidents"
	}
	return strings.Join(nonEmpty, " ")
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
