package agent_test

import (
	"context"
	"testing"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/workflow"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeRunbookExecutor struct {
	calls  int
	tenant string
	planID string
}

func (f *fakeRunbookExecutor) Execute(_ context.Context, tenantID, _ string, plan workflow.RunbookPlan) ([]workflow.StepResult, error) {
	f.calls++
	f.tenant = tenantID
	f.planID = plan.RunbookID
	return []workflow.StepResult{{StepID: "verify", Status: "success"}}, nil
}

func TestStaticRunbookSelector_PostgresFailover(t *testing.T) {
	env := &alert.AlertEnvelope{
		TenantID:    "tenant-1",
		Title:       "Known Postgres Failover Procedure",
		Description: "Primary DB down, standard failover: promote replica",
		Labels:      map[string]string{"service": "database"},
		Annotations: map[string]string{"description": "promote replica"},
	}

	plan, err := agent.StaticRunbookSelector{}.SelectRunbook(context.Background(), env)
	require.NoError(t, err)
	assert.Equal(t, "postgres-failover", plan.RunbookID)
	assert.NotEmpty(t, plan.Steps)
}

func TestPlanningRunbookSpecialist_ExecutesSelectedPlan(t *testing.T) {
	env := &alert.AlertEnvelope{
		ID:          "inc-1",
		TenantID:    "tenant-1",
		Title:       "Rollback Deployment",
		Description: "Bad deploy causing errors, rollback available",
		Labels:      map[string]string{"service": "api"},
		Annotations: map[string]string{},
	}
	exec := &fakeRunbookExecutor{}
	specialist := agent.NewPlanningRunbookSpecialist(agent.StaticRunbookSelector{}, exec, zap.NewNop())

	result, err := specialist.Runbook(context.Background(), env)
	require.NoError(t, err)

	assert.Equal(t, "deployment-rollback", result.Plan.RunbookID)
	assert.Equal(t, 1, exec.calls)
	assert.Equal(t, "tenant-1", exec.tenant)
	assert.Equal(t, "deployment-rollback", exec.planID)
	assert.Len(t, result.Results, 1)
}
