package worker

import (
	"fmt"
	"strings"

	"github.com/paladinai/paladinai/internal/alert"
)

func triagedSubject(tenantID string, source alert.Source) (string, error) {
	return resultSubject("triaged", tenantID, source)
}

func analyzedSubject(tenantID string, source alert.Source) (string, error) {
	return resultSubject("analyzed", tenantID, source)
}

func triageDLQSubject(tenantID string) (string, error) {
	if err := alert.ValidateTenantID(tenantID); err != nil {
		return "", fmt.Errorf("triage dlq tenant: %w", err)
	}
	return fmt.Sprintf("paladin.alerts.triage.dlq.%s", tenantID), nil
}

func resultSubject(kind, tenantID string, source alert.Source) (string, error) {
	if err := alert.ValidateTenantID(tenantID); err != nil {
		return "", fmt.Errorf("%s subject tenant: %w", kind, err)
	}
	sourceToken := string(source)
	if sourceToken == "" {
		return "", fmt.Errorf("%s subject source must not be empty", kind)
	}
	if strings.ContainsAny(sourceToken, ".*>") {
		return "", fmt.Errorf("%s subject source must not contain '.', '*', or '>'", kind)
	}
	return fmt.Sprintf("paladin.alerts.%s.%s.%s", kind, tenantID, sourceToken), nil
}
