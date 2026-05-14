package agent

import (
	"context"
	"strings"

	"github.com/paladinai/paladinai/internal/alert"
)

// IntegrationSpecialist diagnoses alerts involving external providers.
type IntegrationSpecialist interface {
	DiagnoseIntegration(ctx context.Context, env *alert.AlertEnvelope) (*IntegrationResult, error)
}

// IntegrationResult is the supervisor-facing output for integration routes.
type IntegrationResult struct {
	Provider          string   `json:"provider"`
	FailureMode       string   `json:"failure_mode"`
	Checks            []string `json:"checks"`
	RecommendedAction string   `json:"recommended_action"`
	NeedsHuman        bool     `json:"needs_human"`
}

// StaticIntegrationSpecialist is a deterministic provider diagnosis baseline.
type StaticIntegrationSpecialist struct{}

// DiagnoseIntegration derives provider-specific checks without spending an LLM call.
func (StaticIntegrationSpecialist) DiagnoseIntegration(_ context.Context, env *alert.AlertEnvelope) (*IntegrationResult, error) {
	text := strings.ToLower(strings.Join([]string{
		env.Title,
		env.Description,
		env.Labels["alertname"],
		env.Labels["service"],
		env.Labels["job"],
		env.Annotations["description"],
	}, " "))

	provider := integrationProvider(text)
	mode := integrationFailureMode(text)
	checks := integrationChecks(provider, mode)
	return &IntegrationResult{
		Provider:          provider,
		FailureMode:       mode,
		Checks:            checks,
		RecommendedAction: integrationAction(provider, mode),
		NeedsHuman:        env.Severity == alert.SeverityP1 || env.Severity == alert.SeverityP2,
	}, nil
}

func integrationProvider(text string) string {
	switch {
	case containsAny(text, "stripe", "payment", "payments"):
		return "stripe"
	case containsAny(text, "github"):
		return "github"
	case containsAny(text, "slack"):
		return "slack"
	case containsAny(text, "pagerduty", "pager duty"):
		return "pagerduty"
	case containsAny(text, "oauth"):
		return "oauth-provider"
	default:
		return "external-provider"
	}
}

func integrationFailureMode(text string) string {
	switch {
	case containsAny(text, "webhook", "delivery"):
		return "webhook_delivery_failure"
	case containsAny(text, "oauth", "token", "expired", "unauthorized", "401"):
		return "credential_or_oauth_failure"
	case containsAny(text, "timeout", "5xx", "502", "503", "upstream"):
		return "upstream_availability_failure"
	case containsAny(text, "api key", "forbidden", "403"):
		return "api_key_or_permission_failure"
	default:
		return "external_dependency_failure"
	}
}

func integrationChecks(provider, mode string) []string {
	checks := []string{
		"check provider status page",
		"verify outbound network path and DNS",
		"inspect recent integration errors",
	}
	switch mode {
	case "webhook_delivery_failure":
		checks = append(checks, "verify webhook endpoint health", "check signature and retry logs")
	case "credential_or_oauth_failure":
		checks = append(checks, "verify credential expiry", "refresh OAuth token if needed")
	case "upstream_availability_failure":
		checks = append(checks, "check provider 5xx/timeout rate", "enable retry or fallback path")
	case "api_key_or_permission_failure":
		checks = append(checks, "validate API key scope", "rotate compromised or invalid credentials")
	}
	if provider != "external-provider" {
		checks = append(checks, "review "+provider+" integration configuration")
	}
	return checks
}

func integrationAction(provider, mode string) string {
	switch mode {
	case "webhook_delivery_failure":
		return "Stabilize webhook delivery, replay failed events, and confirm provider retries"
	case "credential_or_oauth_failure":
		return "Refresh credentials and verify the integration can authenticate"
	case "upstream_availability_failure":
		return "Confirm provider health and enable fallback or retry controls"
	case "api_key_or_permission_failure":
		return "Validate permissions and rotate or repair the integration secret"
	default:
		return "Triage external dependency health and integration configuration for " + provider
	}
}
