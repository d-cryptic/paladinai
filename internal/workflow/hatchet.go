// Package workflow triggers durable workflow runs on an external workflow engine.
//
// The current implementation targets Hatchet's REST API: a thin HTTP client posts
// a JSON payload to /api/v1/workflows/{name}/trigger. The package also ships a
// NoopClient for environments where Hatchet is not configured, so the orchestrator
// can degrade gracefully without conditionals at every call site.
package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

const maxHatchetResponseBytes = 64 * 1024

// TriagePayload is the input passed to the paladin-triage durable workflow.
//
// Field names are JSON-stable: they form the public contract with Hatchet workers
// and must not be renamed without a coordinated migration.
type TriagePayload struct {
	TenantID      string            `json:"tenant_id"`
	IncidentID    string            `json:"incident_id"`
	Fingerprint   string            `json:"fingerprint"`
	Severity      string            `json:"severity"`
	CorrelationID string            `json:"correlation_id"`
	Labels        map[string]string `json:"labels"`
}

// Trigger is the narrow interface the orchestrator depends on. It is satisfied
// by both Client (real HTTP trigger) and NoopClient (disabled mode).
type Trigger interface {
	TriggerTriage(ctx context.Context, payload TriagePayload) (string, error)
}

// Client triggers Hatchet workflows over the REST API. It is safe for concurrent
// use; the underlying http.Client is shared across goroutines.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	log        *zap.Logger
}

// NewClient creates a Hatchet workflow client. A nil logger is replaced with
// zap.NewNop() so callers can leave logging optional.
func NewClient(baseURL, apiKey string, log *zap.Logger) *Client {
	if log == nil {
		log = zap.NewNop()
	}
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		log:        log,
	}
}

// TriggerTriage fires the "paladin-triage" workflow for an incident and returns
// the workflow run ID on success. The run ID is best-effort: if Hatchet does not
// return a parseable body the call still succeeds with an empty run ID.
func (c *Client) TriggerTriage(ctx context.Context, payload TriagePayload) (string, error) {
	return c.triggerWorkflow(ctx, "paladin-triage", payload)
}

// triggerWorkflow fires an arbitrary named Hatchet workflow with the given payload.
// It is shared by TriggerTriage, TriggerP1, and TriggerP2.
func (c *Client) triggerWorkflow(ctx context.Context, workflowName string, payload any) (string, error) {
	body, err := json.Marshal(map[string]any{"input": payload})
	if err != nil {
		return "", fmt.Errorf("workflow: marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/workflows/%s/trigger", c.baseURL, workflowName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("workflow: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("workflow: hatchet request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("workflow: hatchet returned %d for %s", resp.StatusCode, workflowName)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxHatchetResponseBytes+1))
	if err != nil {
		return "", fmt.Errorf("workflow: read hatchet response: %w", err)
	}
	if len(data) > maxHatchetResponseBytes {
		c.log.Warn("workflow: response too large to parse run ID",
			zap.Int("bytes", len(data)),
			zap.Int("max_bytes", maxHatchetResponseBytes),
		)
		return "", nil
	}

	var result struct {
		WorkflowRunID string `json:"workflow_run_id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		c.log.Warn("workflow: could not parse run ID from response", zap.Error(err))
		return "", nil
	}
	return result.WorkflowRunID, nil
}

// NoopClient is a Trigger implementation that does nothing. It is used when
// Hatchet is not configured so the orchestrator main loop can call the trigger
// unconditionally.
type NoopClient struct{}

// TriggerTriage returns an empty run ID with no error.
func (NoopClient) TriggerTriage(_ context.Context, _ TriagePayload) (string, error) {
	return "", nil
}
