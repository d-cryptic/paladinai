package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newSlackGitHubRouter(t *testing.T) (chi.Router, *fakePublisher) {
	t.Helper()
	pub := &fakePublisher{}
	wh := handler.NewWebhookHandler(pub, newFakeDedup(), zap.NewNop())
	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())
	return r, pub
}

// ── Slack ─────────────────────────────────────────────────────────────────────

func TestWebhookHandler_Slack_CriticalMessage(t *testing.T) {
	r, pub := newSlackGitHubRouter(t)

	body := `{
		"type":"event_callback",
		"team_id":"T01",
		"event":{
			"type":"message",
			"text":"CRITICAL: payment service is down",
			"user":"U01",
			"channel":"C01",
			"ts":"1700000000.000000"
		},
		"event_time":1700000000
	}`

	req := httptest.NewRequest(http.MethodPost, "/webhook/slack/tenant-a", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Len(t, pub.published, 1)
	assert.Equal(t, "slack", string(pub.published[0].Source))
}

func TestWebhookHandler_Slack_URLVerificationNoPublish(t *testing.T) {
	r, pub := newSlackGitHubRouter(t)

	body := `{"type":"url_verification","challenge":"abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/slack/tenant-a", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Len(t, pub.published, 0, "url_verification should not publish")
}

func TestWebhookHandler_Slack_MissingTenant(t *testing.T) {
	pub := &fakePublisher{}
	wh := handler.NewWebhookHandler(pub, newFakeDedup(), zap.NewNop())
	r := chi.NewRouter()
	r.Mount("/webhook", wh.Routes())

	req := httptest.NewRequest(http.MethodPost, "/webhook/slack/", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// ── GitHub ────────────────────────────────────────────────────────────────────

func TestWebhookHandler_GitHub_DeploymentFailure(t *testing.T) {
	r, pub := newSlackGitHubRouter(t)

	body := `{
		"action": "created",
		"deployment": {
			"id": 1, "sha": "abc", "ref": "main", "task": "deploy",
			"environment": "production", "description": "Deploy v1",
			"creator": {"id":1,"login":"ci"},
			"created_at": "2026-05-12T10:00:00Z",
			"updated_at": "2026-05-12T10:00:00Z"
		},
		"deployment_status": {
			"id": 2, "state": "failure",
			"description": "exit code 1",
			"environment": "production",
			"created_at": "2026-05-12T10:05:00Z"
		},
		"repository": {"id":10,"name":"api","full_name":"acme/api","html_url":"https://github.com/acme/api"},
		"sender": {"id":1,"login":"ci"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/webhook/github/tenant-a", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "deployment_status")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Len(t, pub.published, 1)
	assert.Equal(t, "github", string(pub.published[0].Source))
	assert.Equal(t, "P2", string(pub.published[0].Severity))
}

func TestWebhookHandler_GitHub_CheckRunFailure(t *testing.T) {
	r, pub := newSlackGitHubRouter(t)

	body := `{
		"action": "completed",
		"check_run": {
			"id": 555, "name": "tests",
			"status": "completed", "conclusion": "failure",
			"started_at": "2026-05-12T09:00:00Z",
			"html_url": "https://github.com/acme/api/runs/555"
		},
		"repository": {"id":789,"name":"api","full_name":"acme/api","html_url":""},
		"sender": {"id":1,"login":"ci"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/webhook/github/tenant-a", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "check_run")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Len(t, pub.published, 1)
}

func TestWebhookHandler_GitHub_PushEventIgnored(t *testing.T) {
	r, pub := newSlackGitHubRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/webhook/github/tenant-a", strings.NewReader(`{"ref":"refs/heads/main"}`))
	req.Header.Set("X-GitHub-Event", "push")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Len(t, pub.published, 0, "push event should not produce alerts")
}
