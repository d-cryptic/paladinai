package agent

import (
	"strings"

	"github.com/paladinai/paladinai/internal/alert"
)

// TriageContextFromAlert builds the minimal triage context needed by RCA when
// the supervisor already has classifier output. It avoids an extra LLM call on
// RCA routes while preserving the RCA input contract.
func TriageContextFromAlert(env *alert.AlertEnvelope, severity, intent string) *TriageResult {
	if severity == "" {
		severity = string(env.Severity)
	}
	severity = strings.ToUpper(strings.TrimSpace(severity))
	if !validSeverities[severity] {
		severity = "P3"
	}

	service := strings.TrimSpace(env.Labels["service"])
	if service == "" {
		service = strings.TrimSpace(env.Labels["job"])
	}
	if service == "" {
		service = "unknown"
	}

	cause := firstNonEmptyString(
		env.Annotations["description"],
		env.Description,
		env.Title,
		"root cause requires RCA",
	)
	return &TriageResult{
		ConfirmedSeverity: severity,
		Summary:           truncate(firstNonEmptyString(env.Title, env.Description, "alert requires RCA"), maxSummaryLen),
		LikelyCause:       truncate(cause, maxCauseLen),
		AffectedServices:  []string{truncate(service, maxServiceLen)},
		RecommendedAction: truncate(recommendedActionForIntent(intent), maxActionLen),
		NeedsHuman:        severity == "P1" || severity == "P2",
	}
}

func recommendedActionForIntent(intent string) string {
	switch intent {
	case "service_down":
		return "Investigate outage indicators and restore service health"
	case "oom":
		return "Inspect memory pressure, recent changes, and restart policy"
	case "security_alert":
		return "Escalate to security owner and preserve evidence"
	case "network_issue":
		return "Check network path, DNS, and upstream connectivity"
	case "capacity_warning":
		return "Inspect saturation metrics and add capacity if needed"
	default:
		return "Run RCA using alert labels, annotations, and recent changes"
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
