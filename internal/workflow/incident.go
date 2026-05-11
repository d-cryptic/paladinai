// Package workflow: incident.go defines the P1/P2 IncidentWorkflow step graph
// and the human approval gate. These types describe the workflow DAG that is
// registered with Hatchet and triggered by the orchestrator.
//
// Hatchet SDK is not vendored; we model workflows as Go structs that are
// JSON-serialised into the Hatchet REST trigger payload. The step functions
// are registered via the Hatchet worker binary (cmd/paladin-orchestrator).
package workflow

import (
	"context"
	"fmt"
	"time"
)

// ─── Approval gate ────────────────────────────────────────────────────────────

// ApprovalDecision is the outcome of a human approval gate.
type ApprovalDecision string

const (
	ApprovalApproved ApprovalDecision = "approved"
	ApprovalRejected ApprovalDecision = "rejected"
	ApprovalTimeout  ApprovalDecision = "timeout"
)

// ApprovalRequest is published to NATS when a step requires human review.
// The browser UI subscribes to approvals.{tenant}.{incident_id}.{gate_id}.
type ApprovalRequest struct {
	GateID        string    `json:"gate_id"`
	TenantID      string    `json:"tenant_id"`
	IncidentID    string    `json:"incident_id"`
	ActionSummary string    `json:"action_summary"`
	RiskLevel     string    `json:"risk_level"` // "HIGH" | "MEDIUM" | "LOW"
	ExpiresAt     time.Time `json:"expires_at"`
}

// ApprovalResult is the signal received from the UI or timeout.
type ApprovalResult struct {
	GateID   string           `json:"gate_id"`
	Decision ApprovalDecision `json:"decision"`
	Actor    string           `json:"actor,omitempty"` // email of the engineer who decided
	Note     string           `json:"note,omitempty"`
}

// ApprovalStore is the minimal interface for polling approval signals.
// The websocket service writes decisions here; the orchestrator polls.
type ApprovalStore interface {
	// Poll blocks until a decision arrives for gateID or ctx is cancelled.
	Poll(ctx context.Context, tenantID, gateID string) (ApprovalResult, error)
	// Publish stores an approval request for the browser to display.
	Publish(ctx context.Context, req ApprovalRequest) error
}

// DefaultApprovalTimeout is how long to wait for human approval before auto-rejecting.
const DefaultApprovalTimeout = 30 * time.Minute

// ─── Workflow step types ──────────────────────────────────────────────────────

// StepType classifies the kind of action a runbook step takes.
type StepType string

const (
	StepTypeToolCall    StepType = "tool_call"
	StepTypeScript      StepType = "script"
	StepTypeAPICall     StepType = "api_call"
	StepTypeHumanAction StepType = "human_action"
)

// RunbookStep describes one step in a runbook plan.
type RunbookStep struct {
	StepID          string            `json:"step_id"`
	Name            string            `json:"name"`
	Type            StepType          `json:"type"`
	Tool            string            `json:"tool,omitempty"`   // MCP tool name
	Args            map[string]string `json:"args,omitempty"`
	RequiresApproval bool             `json:"requires_approval"`
	VerifyCondition  string           `json:"verify_condition,omitempty"`
	Timeout         time.Duration     `json:"timeout,omitempty"`
}

// RunbookPlan is the ordered sequence of steps produced by SelectRunbook.
type RunbookPlan struct {
	RunbookID string        `json:"runbook_id"`
	Name      string        `json:"name"`
	Steps     []RunbookStep `json:"steps"`
}

// StepResult records the outcome of a single runbook step execution.
type StepResult struct {
	StepID    string        `json:"step_id"`
	Status    string        `json:"status"` // "success" | "failed" | "skipped"
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

// ─── Incident workflow payload types ─────────────────────────────────────────

// P1Payload is the input to the P1IncidentWorkflow registered in Hatchet.
// ExternalID is set to IncidentID so Hatchet deduplicates concurrent triggers
// for the same incident.
type P1Payload struct {
	TenantID   string `json:"tenant_id"`
	IncidentID string `json:"incident_id"`
	Fingerprint string `json:"fingerprint"`
	// ExternalID deduplicates concurrent triggers for the same incident.
	ExternalID string `json:"external_id"` // = IncidentID
}

// P2Payload is the input to the P2IncidentWorkflow.
type P2Payload struct {
	TenantID   string `json:"tenant_id"`
	IncidentID string `json:"incident_id"`
	Fingerprint string `json:"fingerprint"`
	ExternalID string `json:"external_id"` // = IncidentID
}

// ─── Extended trigger client ──────────────────────────────────────────────────

// IncidentTrigger extends Trigger with P1/P2 workflow support.
type IncidentTrigger interface {
	Trigger
	TriggerP1(ctx context.Context, payload P1Payload) (string, error)
	TriggerP2(ctx context.Context, payload P2Payload) (string, error)
}

// TriggerP1 fires the "paladin-p1-incident" Hatchet workflow.
func (c *Client) TriggerP1(ctx context.Context, payload P1Payload) (string, error) {
	return c.triggerWorkflow(ctx, "paladin-p1-incident", payload)
}

// TriggerP2 fires the "paladin-p2-incident" Hatchet workflow.
func (c *Client) TriggerP2(ctx context.Context, payload P2Payload) (string, error) {
	return c.triggerWorkflow(ctx, "paladin-p2-incident", payload)
}

// NoopClient implementations.

// TriggerP1 is a no-op that always succeeds.
func (NoopClient) TriggerP1(_ context.Context, _ P1Payload) (string, error) { return "", nil }

// TriggerP2 is a no-op that always succeeds.
func (NoopClient) TriggerP2(_ context.Context, _ P2Payload) (string, error) { return "", nil }

// ─── Workflow step executor ───────────────────────────────────────────────────

// ToolCaller is the narrow interface for executing a single MCP tool call.
// Implemented by paladin-hub client; faked in tests.
type ToolCaller interface {
	Call(ctx context.Context, tenantID, toolName string, args map[string]string) (string, error)
}

// RunbookExecutor executes a RunbookPlan step-by-step with optional approval gates.
// It is used by both P1 and P2 IncidentWorkflow "ExecuteRunbook" steps.
type RunbookExecutor struct {
	approvals ApprovalStore
	tools     ToolCaller
	timeout   time.Duration
}

// NewRunbookExecutor creates a RunbookExecutor with the given dependencies.
// approvalTimeout configures how long to wait for human approval per step;
// 0 uses DefaultApprovalTimeout.
func NewRunbookExecutor(approvals ApprovalStore, tools ToolCaller, approvalTimeout time.Duration) *RunbookExecutor {
	if approvalTimeout <= 0 {
		approvalTimeout = DefaultApprovalTimeout
	}
	return &RunbookExecutor{approvals: approvals, tools: tools, timeout: approvalTimeout}
}

// Execute runs each step in the plan sequentially, requesting approval where
// required. It returns the per-step results; execution halts on first error.
func (e *RunbookExecutor) Execute(ctx context.Context, tenantID, incidentID string, plan RunbookPlan) ([]StepResult, error) {
	results := make([]StepResult, 0, len(plan.Steps))

	for _, step := range plan.Steps {
		result, err := e.executeStep(ctx, tenantID, incidentID, step)
		results = append(results, result)
		if err != nil {
			return results, fmt.Errorf("runbook %q step %q: %w", plan.RunbookID, step.StepID, err)
		}
		if result.Status == "failed" {
			return results, fmt.Errorf("runbook %q step %q failed: %s", plan.RunbookID, step.StepID, result.Error)
		}
	}
	return results, nil
}

func (e *RunbookExecutor) executeStep(ctx context.Context, tenantID, incidentID string, step RunbookStep) (StepResult, error) {
	start := time.Now()

	// Approval gate: pause until human approves or timeout.
	if step.RequiresApproval {
		gateID := incidentID + ":" + step.StepID
		req := ApprovalRequest{
			GateID:        gateID,
			TenantID:      tenantID,
			IncidentID:    incidentID,
			ActionSummary: fmt.Sprintf("Step %q: %s", step.Name, step.Tool),
			RiskLevel:     "HIGH",
			ExpiresAt:     time.Now().Add(e.timeout),
		}
		if pubErr := e.approvals.Publish(ctx, req); pubErr != nil {
			return StepResult{
				StepID: step.StepID, Status: "failed",
				Error: fmt.Sprintf("publish approval request: %v", pubErr),
			}, pubErr
		}

		approvalCtx, cancel := context.WithTimeout(ctx, e.timeout)
		defer cancel()

		decision, pollErr := e.approvals.Poll(approvalCtx, tenantID, gateID)
		if pollErr != nil || decision.Decision != ApprovalApproved {
			reason := "approval rejected or timed out"
			if pollErr != nil {
				reason = pollErr.Error()
			} else if decision.Decision == ApprovalRejected {
				reason = "rejected by " + decision.Actor
			}
			return StepResult{
				StepID: step.StepID, Status: "skipped",
				Error:     reason,
				Timestamp: time.Now(),
				Duration:  time.Since(start),
			}, nil
		}
	}

	// Execute the tool call.
	var output string
	var execErr error

	stepTimeout := step.Timeout
	if stepTimeout <= 0 {
		stepTimeout = 2 * time.Second
	}

	stepCtx, cancel := context.WithTimeout(ctx, stepTimeout)
	defer cancel()

	switch step.Type {
	case StepTypeToolCall, StepTypeAPICall:
		output, execErr = e.tools.Call(stepCtx, tenantID, step.Tool, step.Args)
	case StepTypeHumanAction:
		// Human actions are gated above; mark as success after approval.
		output = "human action approved"
	default:
		execErr = fmt.Errorf("unsupported step type %q", step.Type)
	}

	status := "success"
	errStr := ""
	if execErr != nil {
		status = "failed"
		errStr = execErr.Error()
	}

	return StepResult{
		StepID:    step.StepID,
		Status:    status,
		Output:    output,
		Error:     errStr,
		Duration:  time.Since(start),
		Timestamp: time.Now(),
	}, execErr
}
