// Package agent: supervisor.go implements the Stage 3 explicit pipeline that
// classifies an alert and routes it to the appropriate specialist agent.
package agent

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// SupervisorPipeline implements the explicit classify→route→triage/rca pipeline.
// This is the Stage 3 replacement for ad-hoc routing in the worker.
// Each step is deterministic: the classifier decides once, then the appropriate
// specialist runs. No ReAct loop, no LLM deciding "next step".
type SupervisorPipeline struct {
	classifier *ClassifierAgent
	triager    Triager
	rca        RCAAnalyzer // optional; if nil, alerts routed to "rca" fall back to triage
	log        *zap.Logger
}

// NewSupervisorPipeline creates a SupervisorPipeline.
// rcaAnalyzer may be nil — in that case alerts routed to "rca" fall back to the triage path.
func NewSupervisorPipeline(classifier *ClassifierAgent, triager Triager, rcaAnalyzer RCAAnalyzer, log *zap.Logger) *SupervisorPipeline {
	if log == nil {
		log = zap.NewNop()
	}
	return &SupervisorPipeline{
		classifier: classifier,
		triager:    triager,
		rca:        rcaAnalyzer,
		log:        log,
	}
}

// Process runs the full classify→route→specialist pipeline for one alert.
// It returns the populated IncidentState.
func (s *SupervisorPipeline) Process(ctx context.Context, state *IncidentState) (*IncidentState, error) {
	// Stage 1: classify
	cls, err := s.classifier.Classify(ctx, &state.Alert)
	if err != nil {
		return state, fmt.Errorf("supervisor: classify: %w", err)
	}

	state.Intent = cls.Intent
	state.AgentType = cls.AgentType
	state.Severity = cls.Severity
	state.RoutingConfidence = cls.Confidence

	s.log.Info("supervisor: classified alert",
		zap.String("tenant", state.TenantID),
		zap.String("fingerprint", state.Alert.Fingerprint),
		zap.String("intent", state.Intent),
		zap.String("agent_type", state.AgentType),
		zap.String("severity", state.Severity),
		zap.Float32("confidence", state.RoutingConfidence),
	)

	// Stage 2: route to specialist
	switch state.AgentType {
	case "rca":
		if s.rca != nil {
			// First triage (RCA needs triage context), then RCA.
			tr, err := s.triager.Triage(ctx, &state.Alert)
			if err != nil {
				return state, fmt.Errorf("supervisor: triage before rca: %w", err)
			}
			state.TriageResult = tr
			state.NeedsHuman = tr.NeedsHuman

			rr, err := s.rca.Analyze(ctx, &state.Alert, tr)
			if err != nil {
				s.log.Warn("supervisor: rca failed, continuing with triage only", zap.Error(err))
			} else {
				state.RCAResult = rr
			}
		} else {
			// No RCA configured — fall through to triage.
			s.log.Debug("supervisor: rca not configured, falling back to triage")
			tr, err := s.triager.Triage(ctx, &state.Alert)
			if err != nil {
				return state, fmt.Errorf("supervisor: triage fallback: %w", err)
			}
			state.TriageResult = tr
			state.NeedsHuman = tr.NeedsHuman
		}

	default: // "triage"
		tr, err := s.triager.Triage(ctx, &state.Alert)
		if err != nil {
			return state, fmt.Errorf("supervisor: triage: %w", err)
		}
		state.TriageResult = tr
		state.NeedsHuman = tr.NeedsHuman
	}

	return state, nil
}
