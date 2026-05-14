package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/workflow"
	"go.uber.org/zap"
)

// RunbookSpecialist executes or prepares a procedural runbook for an alert.
type RunbookSpecialist interface {
	Runbook(ctx context.Context, env *alert.AlertEnvelope) (*RunbookResult, error)
}

// RunbookResult is the supervisor-facing runbook specialist output.
type RunbookResult struct {
	Plan    workflow.RunbookPlan
	Results []workflow.StepResult
}

// RunbookSelector picks the best procedural plan for an alert.
type RunbookSelector interface {
	SelectRunbook(ctx context.Context, env *alert.AlertEnvelope) (workflow.RunbookPlan, error)
}

// RunbookExecutor is the narrow execution dependency used by the supervisor.
type RunbookExecutor interface {
	Execute(ctx context.Context, tenantID, incidentID string, plan workflow.RunbookPlan) ([]workflow.StepResult, error)
}

// PlanningRunbookSpecialist selects a concrete runbook plan and optionally
// executes it when an executor is configured.
type PlanningRunbookSpecialist struct {
	selector RunbookSelector
	executor RunbookExecutor
	log      *zap.Logger
}

// NewPlanningRunbookSpecialist creates a runbook specialist.
func NewPlanningRunbookSpecialist(selector RunbookSelector, executor RunbookExecutor, log *zap.Logger) *PlanningRunbookSpecialist {
	if log == nil {
		log = zap.NewNop()
	}
	return &PlanningRunbookSpecialist{selector: selector, executor: executor, log: log}
}

// Runbook selects the best matching runbook and executes it when possible.
func (r *PlanningRunbookSpecialist) Runbook(ctx context.Context, env *alert.AlertEnvelope) (*RunbookResult, error) {
	if r == nil || r.selector == nil {
		return nil, fmt.Errorf("runbook specialist: selector not configured")
	}
	plan, err := r.selector.SelectRunbook(ctx, env)
	if err != nil {
		return nil, fmt.Errorf("select runbook: %w", err)
	}
	result := &RunbookResult{Plan: plan}
	if r.executor == nil {
		r.log.Info("runbook specialist: selected plan without executor",
			zap.String("runbook_id", plan.RunbookID),
			zap.Int("steps", len(plan.Steps)),
		)
		return result, nil
	}
	results, err := r.executor.Execute(ctx, env.TenantID, incidentID(env), plan)
	if err != nil {
		return nil, fmt.Errorf("execute runbook: %w", err)
	}
	result.Results = results
	return result, nil
}

// StaticRunbookSelector gives production and tests a deterministic procedural
// baseline until semantic runbook retrieval is wired into this path.
type StaticRunbookSelector struct{}

// SelectRunbook maps common procedural alerts to safe, explicit plans.
func (StaticRunbookSelector) SelectRunbook(_ context.Context, env *alert.AlertEnvelope) (workflow.RunbookPlan, error) {
	text := strings.ToLower(strings.Join([]string{
		env.Title,
		env.Description,
		env.Labels["alertname"],
		env.Labels["service"],
		env.Annotations["description"],
	}, " "))

	switch {
	case containsAny(text, "failover", "promote replica", "primary db"):
		return workflow.RunbookPlan{
			RunbookID: "postgres-failover",
			Name:      "Postgres Failover",
			Steps: []workflow.RunbookStep{
				humanRunbookStep("verify_replica_health", "Verify replica health and replication lag"),
				humanRunbookStep("promote_replica", "Promote the healthiest replica"),
				humanRunbookStep("verify_database_recovery", "Verify application database recovery"),
			},
		}, nil
	case containsAny(text, "rollback", "rollout undo", "bad deploy"):
		return workflow.RunbookPlan{
			RunbookID: "deployment-rollback",
			Name:      "Deployment Rollback",
			Steps: []workflow.RunbookStep{
				humanRunbookStep("confirm_bad_release", "Confirm the release causing errors"),
				humanRunbookStep("rollback_release", "Rollback to the last known good version"),
				humanRunbookStep("verify_error_rate", "Verify error rate and latency recovery"),
			},
		}, nil
	case containsAny(text, "flush", "stale cache", "cache clear"):
		return workflow.RunbookPlan{
			RunbookID: "cache-flush",
			Name:      "Cache Flush",
			Steps: []workflow.RunbookStep{
				humanRunbookStep("scope_cache_keys", "Scope impacted cache keys"),
				humanRunbookStep("flush_cache", "Flush the affected cache safely"),
				humanRunbookStep("verify_cache_rebuild", "Verify cache rebuild and request health"),
			},
		}, nil
	case containsAny(text, "scale", "queue depth", "worker fleet"):
		return workflow.RunbookPlan{
			RunbookID: "scale-worker-fleet",
			Name:      "Scale Worker Fleet",
			Steps: []workflow.RunbookStep{
				humanRunbookStep("check_capacity", "Check current queue depth and worker saturation"),
				humanRunbookStep("scale_workers", "Scale workers to the runbook target"),
				humanRunbookStep("verify_drain_rate", "Verify queue drain rate improves"),
			},
		}, nil
	default:
		return workflow.RunbookPlan{}, fmt.Errorf("no static runbook matched alert %q", env.Title)
	}
}

func humanRunbookStep(id, name string) workflow.RunbookStep {
	return workflow.RunbookStep{
		StepID:           id,
		Name:             name,
		Type:             workflow.StepTypeHumanAction,
		RequiresApproval: false,
	}
}

func incidentID(env *alert.AlertEnvelope) string {
	if env.ID != "" {
		return env.ID
	}
	if env.Fingerprint != "" {
		return env.Fingerprint
	}
	return "unknown"
}
