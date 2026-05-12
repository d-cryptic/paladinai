package normalizer

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/sanitizer"
)

// GitHubDeploymentPayload is the payload for deployment and deployment_status events.
type GitHubDeploymentPayload struct {
	Action     string              `json:"action"`
	Deployment GitHubDeployment    `json:"deployment"`
	DeploymentStatus *GitHubDeploymentStatus `json:"deployment_status,omitempty"`
	Repository GitHubRepository    `json:"repository"`
	Sender     GitHubUser          `json:"sender"`
}

// GitHubDeployment is a GitHub Deployments API record.
type GitHubDeployment struct {
	ID          int64             `json:"id"`
	SHA         string            `json:"sha"`
	Ref         string            `json:"ref"`
	Task        string            `json:"task"`
	Environment string            `json:"environment"`
	Description string            `json:"description"`
	Creator     GitHubUser        `json:"creator"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Payload     json.RawMessage   `json:"payload"`
}

// GitHubDeploymentStatus carries the current deployment state.
type GitHubDeploymentStatus struct {
	ID          int64      `json:"id"`
	State       string     `json:"state"` // pending|in_progress|success|failure|error|inactive
	Description string     `json:"description"`
	Environment string     `json:"environment"`
	CreatedAt   time.Time  `json:"created_at"`
}

// GitHubRepository is the repository block in a webhook payload.
type GitHubRepository struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
}

// GitHubUser is a GitHub actor block.
type GitHubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// GitHubCheckRunPayload covers check_run events (CI failure detection).
type GitHubCheckRunPayload struct {
	Action   string          `json:"action"`
	CheckRun GitHubCheckRun  `json:"check_run"`
	Repository GitHubRepository `json:"repository"`
	Sender   GitHubUser      `json:"sender"`
}

// GitHubCheckRun is the check run result.
type GitHubCheckRun struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Status      string         `json:"status"`       // queued|in_progress|completed
	Conclusion  string         `json:"conclusion"`   // success|failure|neutral|cancelled|timed_out|action_required
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	HTMLURL     string         `json:"html_url"`
}

// NormalizeGitHub converts a GitHub webhook event into AlertEnvelopes.
// Deployment failure events produce P2 alerts; CI check failures produce P3.
// Successful deployments are also ingested so the topology store can track
// deployment markers for blast-radius correlation.
func NormalizeGitHub(tenantID, eventType string, raw json.RawMessage, log *zap.Logger) ([]alert.AlertEnvelope, error) {
	if log == nil {
		log = zap.NewNop()
	}

	switch eventType {
	case "deployment", "deployment_status":
		return normalizeGitHubDeployment(tenantID, raw, log)
	case "check_run":
		return normalizeGitHubCheckRun(tenantID, raw, log)
	default:
		// Events like push, pull_request, etc. are not surfaced as alerts.
		return nil, nil
	}
}

func normalizeGitHubDeployment(tenantID string, raw json.RawMessage, log *zap.Logger) ([]alert.AlertEnvelope, error) {
	var payload GitHubDeploymentPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal github deployment payload: %w", err)
	}

	dep := payload.Deployment
	repo := payload.Repository

	state := "pending"
	if payload.DeploymentStatus != nil {
		state = payload.DeploymentStatus.State
	}

	// Only surface failure/error states as alert incidents.
	// Success states are recorded (for topology) but not paged.
	status := alert.StatusFiring
	severity := alert.SeverityP2

	switch strings.ToLower(state) {
	case "success", "inactive":
		// Non-alerting: topology marker only.
		status = alert.StatusResolved
		severity = alert.SeverityP4
	case "failure", "error":
		severity = alert.SeverityP2
	case "pending", "in_progress":
		status = alert.StatusFiring
		severity = alert.SeverityP4
	}

	title := fmt.Sprintf("GitHub Deploy %s — %s@%s failed", state, repo.FullName, dep.Environment)
	if status == alert.StatusResolved {
		title = fmt.Sprintf("GitHub Deploy %s — %s@%s", state, repo.FullName, dep.Environment)
	}

	rawLabels := map[string]string{
		"alertname":   title,
		"source":      "github",
		"repo":        repo.FullName,
		"environment": dep.Environment,
		"ref":         dep.Ref,
		"sha":         dep.SHA,
		"task":        dep.Task,
		"deploy_state": state,
	}
	if payload.Sender.Login != "" {
		rawLabels["triggered_by"] = payload.Sender.Login
	}

	rawAnnotations := map[string]string{
		"description": dep.Description,
		"repo_url":    repo.HTMLURL,
	}

	labelResult := sanitizer.SanitizeMap(rawLabels)
	annotationResult := sanitizer.SanitizeMap(rawAnnotations)
	labels := labelResult.Values
	annotations := annotationResult.Values

	if len(labelResult.ChangedKeys) > 0 || len(labelResult.DroppedKeys) > 0 {
		log.Warn("prompt injection sanitized in github labels",
			zap.String("tenant_id", tenantID),
			zap.Strings("changed_keys", labelResult.ChangedKeys),
			zap.Strings("dropped_keys", labelResult.DroppedKeys),
		)
	}

	startsAt := dep.CreatedAt.UTC()
	if startsAt.IsZero() {
		startsAt = time.Now().UTC()
	}

	fp := alert.ComputeFingerprint(alert.SourceGitHub, labels)

	env := alert.AlertEnvelope{
		ID:          uuid.New().String(),
		Fingerprint: fp,
		TenantID:    tenantID,
		Severity:    severity,
		Status:      status,
		Source:      alert.SourceGitHub,
		Title:       title,
		Description: dep.Description,
		Labels:      labels,
		Annotations: annotations,
		StartsAt:    startsAt,
		ReceivedAt:  time.Now().UTC(),
		AgentState:  "pending",
		Payload:     append(json.RawMessage(nil), raw...),
	}

	if status == alert.StatusResolved {
		t := time.Now().UTC()
		env.EndsAt = &t
	}

	return []alert.AlertEnvelope{env}, nil
}

func normalizeGitHubCheckRun(tenantID string, raw json.RawMessage, log *zap.Logger) ([]alert.AlertEnvelope, error) {
	var payload GitHubCheckRunPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal github check_run payload: %w", err)
	}

	cr := payload.CheckRun
	repo := payload.Repository

	// Only alert on completed+failure conclusions.
	if cr.Status != "completed" {
		return nil, nil
	}

	status := alert.StatusFiring
	severity := alert.SeverityP3

	switch strings.ToLower(cr.Conclusion) {
	case "success", "neutral", "skipped":
		return nil, nil
	case "timed_out", "action_required":
		severity = alert.SeverityP2
	case "failure", "cancelled":
		severity = alert.SeverityP3
	}

	title := fmt.Sprintf("GitHub CI failed: %s — %s", cr.Name, repo.FullName)

	rawLabels := map[string]string{
		"alertname":   title,
		"source":      "github",
		"repo":        repo.FullName,
		"check_name":  cr.Name,
		"conclusion":  cr.Conclusion,
	}

	rawAnnotations := map[string]string{
		"check_url": cr.HTMLURL,
	}

	labelResult := sanitizer.SanitizeMap(rawLabels)
	annotationResult := sanitizer.SanitizeMap(rawAnnotations)
	labels := labelResult.Values
	annotations := annotationResult.Values

	if len(labelResult.ChangedKeys) > 0 || len(labelResult.DroppedKeys) > 0 {
		log.Warn("prompt injection sanitized in github check_run labels",
			zap.String("tenant_id", tenantID),
			zap.Strings("changed_keys", labelResult.ChangedKeys),
			zap.Strings("dropped_keys", labelResult.DroppedKeys),
		)
	}

	startsAt := cr.StartedAt.UTC()
	if startsAt.IsZero() {
		startsAt = time.Now().UTC()
	}

	fp := alert.ComputeFingerprint(alert.SourceGitHub, labels)

	env := alert.AlertEnvelope{
		ID:          uuid.New().String(),
		Fingerprint: fp,
		TenantID:    tenantID,
		Severity:    severity,
		Status:      status,
		Source:      alert.SourceGitHub,
		Title:       title,
		Description: fmt.Sprintf("Check %q concluded with %q in %s", cr.Name, cr.Conclusion, repo.FullName),
		Labels:      labels,
		Annotations: annotations,
		StartsAt:    startsAt,
		ReceivedAt:  time.Now().UTC(),
		AgentState:  "pending",
		Payload:     append(json.RawMessage(nil), raw...),
	}

	return []alert.AlertEnvelope{env}, nil
}
