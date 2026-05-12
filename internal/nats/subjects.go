package nats

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidToken is returned when a subject token contains a dot or wildcard,
// which would corrupt the NATS subject hierarchy.
var ErrInvalidToken = errors.New("nats: subject token must not contain '.', '*', or '>'")

// validateToken rejects strings that would split or glob NATS subject segments.
func validateToken(v string) error {
	if v == "" {
		return fmt.Errorf("%w: token is empty", ErrInvalidToken)
	}
	if strings.ContainsAny(v, ".*>") {
		return fmt.Errorf("%w: %q", ErrInvalidToken, v)
	}
	return nil
}

// AlertSubject returns a tenant-scoped alert subject.
//
//	alerts.{tenant_id}.{severity}.{source}
//
// severity: "critical" | "warning" | "info"
// source:   "alertmanager" | "datadog" | "pagerduty" | ...
func AlertSubject(tenantID, severity, source string) (string, error) {
	for _, tok := range []string{tenantID, severity, source} {
		if err := validateToken(tok); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("alerts.%s.%s.%s", tenantID, severity, source), nil
}

// AlertSubscribePattern returns a NATS wildcard that matches all alerts for a
// tenant: `alerts.{tenant_id}.>`.
func AlertSubscribePattern(tenantID string) (string, error) {
	if err := validateToken(tenantID); err != nil {
		return "", err
	}
	return fmt.Sprintf("alerts.%s.>", tenantID), nil
}

// IncidentSubject returns a tenant-scoped incident subject.
//
//	incidents.{tenant_id}.{incident_id}
func IncidentSubject(tenantID, incidentID string) (string, error) {
	for _, tok := range []string{tenantID, incidentID} {
		if err := validateToken(tok); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("incidents.%s.%s", tenantID, incidentID), nil
}

// RunbookStepSubject returns a tenant-scoped runbook step subject.
//
//	runbooks.{tenant_id}.{incident_id}.{step_id}
func RunbookStepSubject(tenantID, incidentID, stepID string) (string, error) {
	for _, tok := range []string{tenantID, incidentID, stepID} {
		if err := validateToken(tok); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("runbooks.%s.%s.%s", tenantID, incidentID, stepID), nil
}

// WorkingMemoryKey returns the Valkey key for a working-memory entry.
//
//	wm:{tenant_id}:{session_id}:{key}
func WorkingMemoryKey(tenantID, sessionID, key string) (string, error) {
	for _, tok := range []string{tenantID, sessionID, key} {
		if tok == "" {
			return "", fmt.Errorf("nats: WorkingMemoryKey token must not be empty")
		}
	}
	return fmt.Sprintf("wm:%s:%s:%s", tenantID, sessionID, key), nil
}

// ToolCacheKey returns the Valkey key for a tenant-scoped tool cache entry (L3).
//
//	tool:l3:{tenant_id}:{hash}
func ToolCacheKey(tenantID, hash string) (string, error) {
	for _, tok := range []string{tenantID, hash} {
		if tok == "" {
			return "", fmt.Errorf("nats: ToolCacheKey token must not be empty")
		}
	}
	return fmt.Sprintf("tool:l3:%s:%s", tenantID, hash), nil
}

// TenantConfigKeyPattern returns the Valkey key prefix for all config keys
// belonging to a tenant.
//
//	config:{tenant_id}:*
func TenantConfigKeyPattern(tenantID string) (string, error) {
	if tenantID == "" {
		return "", fmt.Errorf("nats: tenant ID must not be empty")
	}
	return fmt.Sprintf("config:%s:*", tenantID), nil
}
