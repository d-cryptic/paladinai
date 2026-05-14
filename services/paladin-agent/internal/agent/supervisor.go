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
	runbook       RunbookSpecialist
	integration   IntegrationSpecialist
	memory        MemorySpecialist
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

// WithRunbookSpecialist configures the runbook execution path.
func (s *SupervisorPipeline) WithRunbookSpecialist(runbook RunbookSpecialist) *SupervisorPipeline {
	s.runbook = runbook
	return s
}

// WithIntegrationSpecialist configures the integration diagnosis path.
func (s *SupervisorPipeline) WithIntegrationSpecialist(integration IntegrationSpecialist) *SupervisorPipeline {
	s.integration = integration
	return s
}

// WithMemorySpecialist configures the memory recall path.
func (s *SupervisorPipeline) WithMemorySpecialist(memory MemorySpecialist) *SupervisorPipeline {
	s.memory = memory
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
			tr := TriageContextFromAlert(&state.Alert, state.Severity, state.Intent)
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

	case "runbook":
		if s.runbook != nil {
			runbookCtx, runbookCancel := contextWithOptionalTimeout(ctx, s.triageTimeout)
			rr, err := s.runbook.Runbook(runbookCtx, &state.Alert)
			runbookCancel()
			if err != nil {
				return state, fmt.Errorf("supervisor: runbook: %w", err)
			}
			state.RunbookPlan = &rr.Plan
			state.RunbookResults = rr.Results
			state.NeedsHuman = runbookNeedsHuman(rr)
			return state, nil
		}
		s.log.Info("supervisor: runbook not configured, falling back to triage")
		triageCtx, triageCancel := contextWithOptionalTimeout(ctx, s.triageTimeout)
		tr, err := s.triager.Triage(triageCtx, &state.Alert)
		triageCancel()
		if err != nil {
			return state, fmt.Errorf("supervisor: runbook fallback triage: %w", err)
		}
		state.TriageResult = tr
		state.NeedsHuman = tr.NeedsHuman

	case "integration":
		if s.integration != nil {
			integrationCtx, integrationCancel := contextWithOptionalTimeout(ctx, s.triageTimeout)
			result, err := s.integration.DiagnoseIntegration(integrationCtx, &state.Alert)
			integrationCancel()
			if err != nil {
				return state, fmt.Errorf("supervisor: integration: %w", err)
			}
			state.IntegrationResult = result
			state.NeedsHuman = result.NeedsHuman
			return state, nil
		}
		s.log.Info("supervisor: integration not configured, falling back to triage")
		triageCtx, triageCancel := contextWithOptionalTimeout(ctx, s.triageTimeout)
		tr, err := s.triager.Triage(triageCtx, &state.Alert)
		triageCancel()
		if err != nil {
			return state, fmt.Errorf("supervisor: integration fallback triage: %w", err)
		}
		state.TriageResult = tr
		state.NeedsHuman = tr.NeedsHuman

	case "memory":
		if s.memory != nil {
			memoryCtx, memoryCancel := contextWithOptionalTimeout(ctx, s.triageTimeout)
			result, err := s.memory.RecallMemory(memoryCtx, &state.Alert)
			memoryCancel()
			if err != nil {
				return state, fmt.Errorf("supervisor: memory: %w", err)
			}
			state.MemoryResult = result
			state.NeedsHuman = result.NeedsHuman
			return state, nil
		}
		s.log.Info("supervisor: memory not configured, falling back to triage")
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

func runbookNeedsHuman(result *RunbookResult) bool {
	if result == nil {
		return false
	}
	for _, step := range result.Plan.Steps {
		if step.RequiresApproval || step.Type == "human_action" {
			return true
		}
	}
	return false
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
