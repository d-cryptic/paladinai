package normalizer

import (
	"encoding/json"
	"testing"

	"github.com/paladinai/paladinai/internal/alert"
	"go.uber.org/zap"
)

func TestNormalizeGitHub_UnknownEventType(t *testing.T) {
	raw := json.RawMessage(`{"action":"opened"}`)
	envs, err := NormalizeGitHub("tenant-a", "pull_request", raw, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 0 {
		t.Errorf("expected no envelopes for pull_request event, got %d", len(envs))
	}
}

func TestNormalizeGitHub_DeploymentFailure(t *testing.T) {
	raw := json.RawMessage(`{
		"action": "created",
		"deployment": {
			"id": 123,
			"sha": "abc123",
			"ref": "main",
			"task": "deploy",
			"environment": "production",
			"description": "Deploy v1.2.3 to production",
			"creator": {"id": 1, "login": "ci-bot"},
			"created_at": "2026-05-12T10:00:00Z",
			"updated_at": "2026-05-12T10:00:00Z"
		},
		"deployment_status": {
			"id": 456,
			"state": "failure",
			"description": "Deploy failed: exit code 1",
			"environment": "production",
			"created_at": "2026-05-12T10:05:00Z"
		},
		"repository": {
			"id": 789,
			"name": "api",
			"full_name": "acme/api",
			"html_url": "https://github.com/acme/api"
		},
		"sender": {"id": 1, "login": "ci-bot"}
	}`)

	envs, err := NormalizeGitHub("tenant-a", "deployment_status", raw, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 1 {
		t.Fatalf("expected 1 envelope, got %d", len(envs))
	}
	env := envs[0]
	if env.TenantID != "tenant-a" {
		t.Errorf("want tenant-a, got %s", env.TenantID)
	}
	if env.Source != alert.SourceGitHub {
		t.Errorf("want source github, got %s", env.Source)
	}
	if env.Severity != alert.SeverityP2 {
		t.Errorf("want P2 for deployment failure, got %s", env.Severity)
	}
	if env.Status != alert.StatusFiring {
		t.Errorf("want firing status, got %s", env.Status)
	}
	if env.Labels["repo"] != "acme/api" {
		t.Errorf("want repo=acme/api, got %q", env.Labels["repo"])
	}
	if env.Labels["environment"] != "production" {
		t.Errorf("want environment=production, got %q", env.Labels["environment"])
	}
	if env.Fingerprint == "" {
		t.Error("fingerprint should not be empty")
	}
}

func TestNormalizeGitHub_DeploymentSuccess(t *testing.T) {
	raw := json.RawMessage(`{
		"action": "created",
		"deployment": {
			"id": 1,
			"sha": "def456",
			"ref": "v2.0.0",
			"task": "deploy",
			"environment": "staging",
			"description": "Release v2.0.0",
			"creator": {"id": 2, "login": "deployer"},
			"created_at": "2026-05-12T11:00:00Z",
			"updated_at": "2026-05-12T11:01:00Z"
		},
		"deployment_status": {
			"id": 2,
			"state": "success",
			"description": "Deployed successfully",
			"environment": "staging",
			"created_at": "2026-05-12T11:01:00Z"
		},
		"repository": {
			"id": 10,
			"name": "worker",
			"full_name": "acme/worker",
			"html_url": "https://github.com/acme/worker"
		},
		"sender": {"id": 2, "login": "deployer"}
	}`)

	envs, err := NormalizeGitHub("tenant-a", "deployment_status", raw, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 1 {
		t.Fatalf("expected 1 envelope (topology marker), got %d", len(envs))
	}
	env := envs[0]
	if env.Status != alert.StatusResolved {
		t.Errorf("want resolved for successful deploy, got %s", env.Status)
	}
}

func TestNormalizeGitHub_CheckRunFailure(t *testing.T) {
	raw := json.RawMessage(`{
		"action": "completed",
		"check_run": {
			"id": 555,
			"name": "test-suite",
			"status": "completed",
			"conclusion": "failure",
			"started_at": "2026-05-12T09:00:00Z",
			"completed_at": "2026-05-12T09:10:00Z",
			"html_url": "https://github.com/acme/api/actions/runs/555"
		},
		"repository": {
			"id": 789,
			"name": "api",
			"full_name": "acme/api",
			"html_url": "https://github.com/acme/api"
		},
		"sender": {"id": 3, "login": "developer"}
	}`)

	envs, err := NormalizeGitHub("tenant-a", "check_run", raw, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 1 {
		t.Fatalf("expected 1 envelope, got %d", len(envs))
	}
	env := envs[0]
	if env.Severity != alert.SeverityP3 {
		t.Errorf("want P3 for check failure, got %s", env.Severity)
	}
	if env.Status != alert.StatusFiring {
		t.Errorf("want firing, got %s", env.Status)
	}
	if env.Labels["check_name"] != "test-suite" {
		t.Errorf("want check_name=test-suite, got %q", env.Labels["check_name"])
	}
}

func TestNormalizeGitHub_CheckRunSuccess(t *testing.T) {
	raw := json.RawMessage(`{
		"action": "completed",
		"check_run": {
			"id": 600,
			"name": "lint",
			"status": "completed",
			"conclusion": "success",
			"started_at": "2026-05-12T09:00:00Z",
			"html_url": "https://github.com/acme/api/actions/runs/600"
		},
		"repository": {"id": 789, "name": "api", "full_name": "acme/api", "html_url": ""},
		"sender": {"id": 1, "login": "ci"}
	}`)
	envs, err := NormalizeGitHub("tenant-a", "check_run", raw, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 0 {
		t.Errorf("expected no envelopes for successful check_run, got %d", len(envs))
	}
}

func TestNormalizeGitHub_CheckRunTimedOut(t *testing.T) {
	raw := json.RawMessage(`{
		"action": "completed",
		"check_run": {
			"id": 700,
			"name": "integration-tests",
			"status": "completed",
			"conclusion": "timed_out",
			"started_at": "2026-05-12T08:00:00Z",
			"html_url": "https://github.com/acme/api/actions/runs/700"
		},
		"repository": {"id": 789, "name": "api", "full_name": "acme/api", "html_url": ""},
		"sender": {"id": 1, "login": "ci"}
	}`)
	envs, err := NormalizeGitHub("tenant-a", "check_run", raw, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 1 {
		t.Fatalf("expected 1 envelope for timed_out check, got %d", len(envs))
	}
	if envs[0].Severity != alert.SeverityP2 {
		t.Errorf("want P2 for timed_out, got %s", envs[0].Severity)
	}
}

func TestNormalizeGitHub_InvalidJSON(t *testing.T) {
	_, err := NormalizeGitHub("tenant-a", "deployment_status", json.RawMessage(`not-json`), zap.NewNop())
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
