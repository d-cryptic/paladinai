// Package agent: supervisor.go implements the Stage 3 explicit pipeline that
// classifies an alert and routes it to the appropriate specialist agent.
package agent

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// SupervisorPipeline implements the explicit classify→route→specialist pipeline.
// This is the Stage 3 replacement for ad-hoc routing in the worker.
// Each step is deterministic: the classifier decides once, then the appropriate
// specialist runs. No ReAct loop, no LLM deciding "next step".
type SupervisorPipeline struct {
	classifier    *ClassifierAgent
	triager       Triager
	rca           RCAAnalyzer // optional; if nil, alerts routed to "rca" fall back to triage
	log           *zap.Logger
	triageTimeout time.Duration
	rcaTimeout    time.Duration
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

// WithTimeouts bounds each specialist step independently.
func (s *SupervisorPipeline) WithTimeouts(triageTimeout, rcaTimeout time.Duration) *SupervisorPipeline {
	s.triageTimeout = triageTimeout
	s.rcaTimeout = rcaTimeout
	return s
}

// Process runs the full classify→route→specialist pipeline for one alert.
// It returns the populated IncidentState.
func (s *SupervisorPipeline) Process(ctx context.Context, state *IncidentState) (*IncidentState, error) {
	// Stage 1: classify
	classifyCtx, classifyCancel := contextWithOptionalTimeout(ctx, s.triageTimeout)
	cls, err := s.classifier.Classify(classifyCtx, &state.Alert)
	classifyCancel()
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
			triageCtx, triageCancel := contextWithOptionalTimeout(ctx, s.triageTimeout)
			tr, err := s.triager.Triage(triageCtx, &state.Alert)
			triageCancel()
			if err != nil {
				return state, fmt.Errorf("supervisor: triage before rca: %w", err)
			}
			state.TriageResult = tr
			state.NeedsHuman = tr.NeedsHuman

			rcaCtx, rcaCancel := contextWithOptionalTimeout(ctx, s.effectiveRCATimeout())
			rr, err := s.rca.Analyze(rcaCtx, &state.Alert, tr)
			rcaCancel()
			if err != nil {
				s.log.Warn("supervisor: rca failed, continuing with triage only", zap.Error(err))
			} else {
				state.RCAResult = rr
			}
		} else {
			// No RCA configured — fall through to triage.
			s.log.Debug("supervisor: rca not configured, falling back to triage")
			triageCtx, triageCancel := contextWithOptionalTimeout(ctx, s.triageTimeout)
			tr, err := s.triager.Triage(triageCtx, &state.Alert)
			triageCancel()
			if err != nil {
				return state, fmt.Errorf("supervisor: triage fallback: %w", err)
			}
			state.TriageResult = tr
			state.NeedsHuman = tr.NeedsHuman
		}

	case "runbook", "memory", "integration":
		// Specialist workers are not yet separate runtime dependencies in this
		// process. Preserve the classified route for observability/evals, then
		// execute triage as the safe baseline path until the specialist is wired.
		s.log.Info("supervisor: specialist route falling back to triage",
			zap.String("agent_type", state.AgentType),
		)
		triageCtx, triageCancel := contextWithOptionalTimeout(ctx, s.triageTimeout)
		tr, err := s.triager.Triage(triageCtx, &state.Alert)
		triageCancel()
		if err != nil {
			return state, fmt.Errorf("supervisor: %s fallback triage: %w", state.AgentType, err)
		}
		state.TriageResult = tr
		state.NeedsHuman = tr.NeedsHuman

	default: // "triage"
		triageCtx, triageCancel := contextWithOptionalTimeout(ctx, s.triageTimeout)
		tr, err := s.triager.Triage(triageCtx, &state.Alert)
		triageCancel()
		if err != nil {
			return state, fmt.Errorf("supervisor: triage: %w", err)
		}
		state.TriageResult = tr
		state.NeedsHuman = tr.NeedsHuman
	}

	return state, nil
}

func (s *SupervisorPipeline) effectiveRCATimeout() time.Duration {
	if s.rcaTimeout > 0 {
		return s.rcaTimeout
	}
	return s.triageTimeout
}

func contextWithOptionalTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}
