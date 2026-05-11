// Package agent: state.go defines the IncidentState struct that carries data
// through the Stage 3 supervisor pipeline (classify → route → triage/rca).
package agent

import "github.com/paladinai/paladinai/internal/alert"

// IncidentState carries all data through the supervisor pipeline.
// All fields are set by pipeline stages; callers treat unset fields as zero values.
type IncidentState struct {
	// Immutable input
	TenantID string
	Alert    alert.AlertEnvelope

	// Supervisor output (classify stage)
	Intent            string // "log_analysis" | "metric_spike" | "service_down" | "oom"
	AgentType         string // "triage" | "rca"
	Severity          string // "P1" | "P2" | "P3" | "P4"
	RoutingConfidence float32

	// Triage output
	TriageResult *TriageResult

	// RCA output (only for P1/P2)
	RCAResult *RCAResult

	// Control
	NeedsHuman   bool
	ErrorMessage string

	// Observability
	TraceID   string
	SessionID string
}
