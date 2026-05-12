package workflow

import (
	"context"
	"strings"
)

// SeverityRouter dispatches incident workflow triggers based on alert severity.
// P0 and P1 (critical) alerts fire TriggerP1; P2 and below fire TriggerP2;
// unknown severities fall through to TriggerTriage.
type SeverityRouter struct {
	client IncidentTrigger
}

// NewSeverityRouter wraps an IncidentTrigger with severity-based dispatch.
func NewSeverityRouter(client IncidentTrigger) *SeverityRouter {
	return &SeverityRouter{client: client}
}

// Route selects the appropriate workflow for the given severity and fires it.
// It returns the workflow run ID or an empty string on noop triggers.
func (r *SeverityRouter) Route(ctx context.Context, severity string, payload TriagePayload) (string, error) {
	switch normalizeSeverity(severity) {
	case "p0", "p1", "critical":
		return r.client.TriggerP1(ctx, P1Payload{
			TenantID:    payload.TenantID,
			IncidentID:  payload.IncidentID,
			Fingerprint: payload.Fingerprint,
			ExternalID:  payload.IncidentID,
		})
	case "p2", "warning", "high":
		return r.client.TriggerP2(ctx, P2Payload{
			TenantID:    payload.TenantID,
			IncidentID:  payload.IncidentID,
			Fingerprint: payload.Fingerprint,
			ExternalID:  payload.IncidentID,
		})
	default:
		return r.client.TriggerTriage(ctx, payload)
	}
}

// normalizeSeverity lowercases and strips whitespace for comparison.
func normalizeSeverity(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
