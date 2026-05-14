package worker

import (
	"fmt"

	"github.com/paladinai/paladinai/internal/alert"
)

func triageDLQSubject(tenantID string) (string, error) {
	if err := alert.ValidateTenantID(tenantID); err != nil {
		return "", fmt.Errorf("triage dlq tenant: %w", err)
	}
	return fmt.Sprintf("paladin.alerts.triage.dlq.%s", tenantID), nil
}
