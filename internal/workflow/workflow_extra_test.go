package workflow_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/internal/workflow"
)

// ─── NoopClient ──────────────────────────────────────────────────────────────

func TestNoopClient_TriggerP1(t *testing.T) {
	id, err := workflow.NoopClient{}.TriggerP1(context.Background(), workflow.P1Payload{
		TenantID: "t1", IncidentID: "inc-1", Fingerprint: "fp", ExternalID: "inc-1",
	})
	require.NoError(t, err)
	assert.Empty(t, id)
}

func TestNoopClient_TriggerP2(t *testing.T) {
	id, err := workflow.NoopClient{}.TriggerP2(context.Background(), workflow.P2Payload{
		TenantID: "t1", IncidentID: "inc-2", Fingerprint: "fp", ExternalID: "inc-2",
	})
	require.NoError(t, err)
	assert.Empty(t, id)
}

// ─── RunbookExecutor — unsupported step type ──────────────────────────────────

func TestRunbookExecutor_UnsupportedStepType_ReturnsError(t *testing.T) {
	store := &fakeApprovalStore{}
	tools := &fakeToolCaller{}
	exec := workflow.NewRunbookExecutor(store, tools, time.Minute)

	plan := workflow.RunbookPlan{
		RunbookID: "rb-unsupported",
		Steps: []workflow.RunbookStep{
			{StepID: "s1", Name: "mystery", Type: workflow.StepType("unknown_type")},
		},
	}

	results, err := exec.Execute(context.Background(), "t1", "inc-1", plan)
	require.Error(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "failed", results[0].Status)
	assert.Contains(t, results[0].Error, "unsupported step type")
}

// ─── RunbookExecutor — poll error ─────────────────────────────────────────────

func TestRunbookExecutor_PollError_SkipsStep(t *testing.T) {
	store := &fakeApprovalStore{
		pollErr:  assert.AnError,
		decision: workflow.ApprovalResult{},
	}
	tools := &fakeToolCaller{}
	exec := workflow.NewRunbookExecutor(store, tools, time.Minute)

	plan := workflow.RunbookPlan{
		RunbookID: "rb-poll-err",
		Steps: []workflow.RunbookStep{
			{StepID: "s1", Type: workflow.StepTypeToolCall, RequiresApproval: true},
		},
	}

	results, err := exec.Execute(context.Background(), "t1", "inc-1", plan)
	require.NoError(t, err, "poll error leads to step skip, not execution error")
	require.Len(t, results, 1)
	assert.Equal(t, "skipped", results[0].Status)
	assert.Equal(t, 0, tools.calls)
}

// ─── ApprovalDecision constants ───────────────────────────────────────────────

func TestApprovalDecisionConstants(t *testing.T) {
	assert.Equal(t, workflow.ApprovalDecision("approved"), workflow.ApprovalApproved)
	assert.Equal(t, workflow.ApprovalDecision("rejected"), workflow.ApprovalRejected)
	assert.Equal(t, workflow.ApprovalDecision("timeout"), workflow.ApprovalTimeout)
}

// ─── RunbookPlan with zero steps ──────────────────────────────────────────────

func TestRunbookExecutor_EmptyPlan(t *testing.T) {
	store := &fakeApprovalStore{}
	tools := &fakeToolCaller{}
	exec := workflow.NewRunbookExecutor(store, tools, time.Minute)

	results, err := exec.Execute(context.Background(), "t1", "inc-1", workflow.RunbookPlan{RunbookID: "empty"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ─── APICall type ─────────────────────────────────────────────────────────────

func TestRunbookExecutor_APICallType_RoutesLikeToolCall(t *testing.T) {
	store := &fakeApprovalStore{}
	tools := &fakeToolCaller{output: "api response"}
	exec := workflow.NewRunbookExecutor(store, tools, time.Minute)

	plan := workflow.RunbookPlan{
		Steps: []workflow.RunbookStep{
			{StepID: "s1", Type: workflow.StepTypeAPICall, Tool: "mcp-webhook"},
		},
	}

	results, err := exec.Execute(context.Background(), "t1", "inc-1", plan)
	require.NoError(t, err)
	assert.Equal(t, "success", results[0].Status)
	assert.Equal(t, 1, tools.calls)
}
