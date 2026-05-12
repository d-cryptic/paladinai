package workflow_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Fakes ───────────────────────────────────────────────────────────────────

type fakeApprovalStore struct {
	decision  workflow.ApprovalResult
	pollErr   error
	pubErr    error
	published []workflow.ApprovalRequest
}

func (f *fakeApprovalStore) Poll(_ context.Context, _, _ string) (workflow.ApprovalResult, error) {
	return f.decision, f.pollErr
}
func (f *fakeApprovalStore) Publish(_ context.Context, req workflow.ApprovalRequest) error {
	f.published = append(f.published, req)
	return f.pubErr
}

type fakeToolCaller struct {
	output string
	err    error
	calls  int
}

func (f *fakeToolCaller) Call(_ context.Context, _, toolName string, _ map[string]string) (string, error) {
	f.calls++
	return f.output, f.err
}

// ─── RunbookExecutor tests ────────────────────────────────────────────────────

func TestRunbookExecutor_SingleStepSuccess(t *testing.T) {
	store := &fakeApprovalStore{}
	tools := &fakeToolCaller{output: "ok"}
	exec := workflow.NewRunbookExecutor(store, tools, time.Minute)

	plan := workflow.RunbookPlan{
		RunbookID: "rb-1",
		Steps: []workflow.RunbookStep{
			{StepID: "s1", Name: "restart", Type: workflow.StepTypeToolCall, Tool: "mcp-k8s"},
		},
	}

	results, err := exec.Execute(context.Background(), "t1", "inc-1", plan)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "success", results[0].Status)
	assert.Equal(t, 1, tools.calls)
}

func TestRunbookExecutor_ToolCallError(t *testing.T) {
	store := &fakeApprovalStore{}
	tools := &fakeToolCaller{err: errors.New("k8s unreachable")}
	exec := workflow.NewRunbookExecutor(store, tools, time.Minute)

	plan := workflow.RunbookPlan{
		RunbookID: "rb-err",
		Steps: []workflow.RunbookStep{
			{StepID: "s1", Name: "restart", Type: workflow.StepTypeToolCall, Tool: "mcp-k8s"},
		},
	}

	results, err := exec.Execute(context.Background(), "t1", "inc-1", plan)
	require.Error(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "failed", results[0].Status)
}

func TestRunbookExecutor_ApprovalGranted(t *testing.T) {
	store := &fakeApprovalStore{decision: workflow.ApprovalResult{
		Decision: workflow.ApprovalApproved, Actor: "alice@example.com",
	}}
	tools := &fakeToolCaller{output: "restarted"}
	exec := workflow.NewRunbookExecutor(store, tools, time.Minute)

	plan := workflow.RunbookPlan{
		RunbookID: "rb-approved",
		Steps: []workflow.RunbookStep{
			{
				StepID:           "s1",
				Name:             "delete pod",
				Type:             workflow.StepTypeToolCall,
				Tool:             "mcp-k8s",
				RequiresApproval: true,
			},
		},
	}

	results, err := exec.Execute(context.Background(), "t1", "inc-1", plan)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "success", results[0].Status)
	assert.Equal(t, 1, tools.calls)

	// Approval request should have been published
	require.Len(t, store.published, 1)
	assert.Equal(t, "t1", store.published[0].TenantID)
}

func TestRunbookExecutor_ApprovalRejected(t *testing.T) {
	store := &fakeApprovalStore{decision: workflow.ApprovalResult{
		Decision: workflow.ApprovalRejected, Actor: "bob@example.com",
	}}
	tools := &fakeToolCaller{}
	exec := workflow.NewRunbookExecutor(store, tools, time.Minute)

	plan := workflow.RunbookPlan{
		RunbookID: "rb-rejected",
		Steps: []workflow.RunbookStep{
			{StepID: "s1", RequiresApproval: true, Type: workflow.StepTypeToolCall, Tool: "mcp-k8s"},
		},
	}

	results, err := exec.Execute(context.Background(), "t1", "inc-1", plan)
	require.NoError(t, err, "rejected approval is not an error — step is skipped")
	require.Len(t, results, 1)
	assert.Equal(t, "skipped", results[0].Status)
	assert.Equal(t, 0, tools.calls, "tool should not be called if approval rejected")
}

func TestRunbookExecutor_PublishApprovalError(t *testing.T) {
	store := &fakeApprovalStore{pubErr: errors.New("nats down")}
	tools := &fakeToolCaller{}
	exec := workflow.NewRunbookExecutor(store, tools, time.Minute)

	plan := workflow.RunbookPlan{
		RunbookID: "rb-pub-err",
		Steps: []workflow.RunbookStep{
			{StepID: "s1", RequiresApproval: true, Type: workflow.StepTypeToolCall},
		},
	}

	_, err := exec.Execute(context.Background(), "t1", "inc-1", plan)
	assert.Error(t, err, "publish error should fail the step and halt execution")
}

func TestRunbookExecutor_MultipleStepsHaltOnError(t *testing.T) {
	callCount := 0
	store := &fakeApprovalStore{}
	tools := &fakeToolCaller{}
	tools.err = errors.New("first step fails")

	exec := workflow.NewRunbookExecutor(store, tools, time.Minute)

	plan := workflow.RunbookPlan{
		RunbookID: "rb-multi",
		Steps: []workflow.RunbookStep{
			{StepID: "s1", Type: workflow.StepTypeToolCall, Tool: "mcp-k8s"},
			{StepID: "s2", Type: workflow.StepTypeToolCall, Tool: "mcp-k8s"}, // should not run
		},
	}

	results, err := exec.Execute(context.Background(), "t1", "inc-1", plan)
	require.Error(t, err)
	assert.Equal(t, 1, callCount+tools.calls, "only the first step should execute")
	assert.Len(t, results, 1, "only result for the failed step")
}

func TestRunbookExecutor_HumanActionStep(t *testing.T) {
	store := &fakeApprovalStore{decision: workflow.ApprovalResult{Decision: workflow.ApprovalApproved}}
	tools := &fakeToolCaller{}
	exec := workflow.NewRunbookExecutor(store, tools, time.Minute)

	plan := workflow.RunbookPlan{
		RunbookID: "rb-human",
		Steps: []workflow.RunbookStep{
			{StepID: "s1", Name: "manual rollback", Type: workflow.StepTypeHumanAction, RequiresApproval: true},
		},
	}

	results, err := exec.Execute(context.Background(), "t1", "inc-1", plan)
	require.NoError(t, err)
	assert.Equal(t, "success", results[0].Status)
	assert.Equal(t, 0, tools.calls, "human action does not call a tool")
}

func TestRunbookExecutor_DefaultTimeout(t *testing.T) {
	// 0 duration → uses DefaultApprovalTimeout without panicking
	store := &fakeApprovalStore{decision: workflow.ApprovalResult{Decision: workflow.ApprovalApproved}}
	tools := &fakeToolCaller{output: "ok"}
	exec := workflow.NewRunbookExecutor(store, tools, 0)

	plan := workflow.RunbookPlan{
		Steps: []workflow.RunbookStep{
			{StepID: "s1", Type: workflow.StepTypeToolCall, Tool: "mcp-k8s"},
		},
	}

	results, err := exec.Execute(context.Background(), "t1", "inc-1", plan)
	require.NoError(t, err)
	assert.Equal(t, "success", results[0].Status)
}
