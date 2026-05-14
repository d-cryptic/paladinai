// Package alert defines the canonical AlertEnvelope and related types.
// All integrations normalize to AlertEnvelope before publishing to NATS.
package alert

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// tenantIDPattern restricts tenant IDs to safe characters for NATS subjects.
// NATS subjects use '.' as delimiter and '*'/'>' as wildcards — none are allowed.
var tenantIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Severity maps to P1–P4 incident classification.
type Severity string

const (
	SeverityP1      Severity = "P1"
	SeverityP2      Severity = "P2"
	SeverityP3      Severity = "P3"
	SeverityP4      Severity = "P4"
	SeverityUnknown Severity = "UNKNOWN"
)

// Status represents the lifecycle state of an alert.
type Status string

const (
	StatusFiring   Status = "firing"
	StatusResolved Status = "resolved"
	StatusSilenced Status = "silenced"
)

// Source identifies which integration produced the alert.
type Source string

const (
	SourceAlertmanager Source = "alertmanager"
	SourceDatadog      Source = "datadog"
	SourcePagerDuty    Source = "pagerduty"
	SourceCloudWatch   Source = "cloudwatch"
	SourceSlack        Source = "slack"
	SourceGitHub       Source = "github"
	SourceOTLP         Source = "otlp"
	SourceCustom       Source = "custom"
)

// AlertEnvelope is the canonical internal representation of an alert.
// Every integration normalises to this shape before NATS publish.
// Fields are deliberately flat for fast serialisation and NATS filtering.
type AlertEnvelope struct {
	// Identity
	ID            string `json:"id"`                       // UUID v7 assigned by paladin-ingest
	Fingerprint   string `json:"fingerprint"`              // deterministic hash for dedup
	TenantID      string `json:"tenant_id"`                // set by paladin-auth, never from LLM context
	CorrelationID string `json:"correlation_id,omitempty"` // set by correlator

	// Classification
	Severity Severity `json:"severity"`
	Status   Status   `json:"status"`
	Category string   `json:"category,omitempty"` // infrastructure, application, security ...
	Source   Source   `json:"source"`

	// Content
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Payload     json.RawMessage   `json:"payload,omitempty"` // original source payload

	// Timing
	StartsAt   time.Time  `json:"starts_at"`
	EndsAt     *time.Time `json:"ends_at,omitempty"`
	ReceivedAt time.Time  `json:"received_at"`

	// Routing
	Runbook   string `json:"runbook,omitempty"`
	Dashboard string `json:"dashboard,omitempty"`

	// Agent processing (populated as pipeline progresses)
	AgentState string `json:"agent_state,omitempty"` // triage, rca, runbook, comms, done
	Summary    string `json:"summary,omitempty"`
	RootCause  string `json:"root_cause,omitempty"`
}

// Fingerprint computes a deterministic fingerprint from stable alert attributes.
// Matches the same fingerprinting logic used in Alertmanager.
// Labels used: alertname + namespace + job + instance (sorted for stability).
func ComputeFingerprint(source Source, labels map[string]string) string {
	// Stable label subset for fingerprinting (matches stage 2 spec)
	fingerprintKeys := []string{"alertname", "namespace", "job", "instance", "cluster", "service"}

	parts := []string{string(source)}
	for _, k := range fingerprintKeys {
		if v, ok := labels[k]; ok {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Include any label starting with "paladin_" for custom tenant routing
	var extra []string
	for k, v := range labels {
		if strings.HasPrefix(k, "paladin_") {
			extra = append(extra, fmt.Sprintf("%s=%s", k, v))
		}
	}
	sort.Strings(extra)
	parts = append(parts, extra...)

	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:16]) // 32 hex chars, compact
}

// ValidateTenantID returns an error if the tenant ID contains characters that
// would allow NATS subject injection (wildcards, dots, or empty string).
func ValidateTenantID(tenantID string) error {
	if tenantID == "" {
		return fmt.Errorf("tenant_id must not be empty")
	}
	if !tenantIDPattern.MatchString(tenantID) {
		return fmt.Errorf("tenant_id %q contains invalid characters (only [a-zA-Z0-9_-] allowed)", tenantID)
	}
	return nil
}

// NATSSubjectE returns the NATS subject for publishing this alert.
// Format: paladin.alerts.raw.<tenant_id>.<source>
func (e *AlertEnvelope) NATSSubjectE() (string, error) {
	if err := ValidateTenantID(e.TenantID); err != nil {
		return "", err
	}
	source := string(e.Source)
	if source == "" {
		return "", fmt.Errorf("source must not be empty")
	}
	if !tenantIDPattern.MatchString(source) {
		return "", fmt.Errorf("source %q contains invalid characters (only [a-zA-Z0-9_-] allowed)", source)
	}
	return fmt.Sprintf("paladin.alerts.raw.%s.%s", e.TenantID, source), nil
}

// NATSSubject returns the NATS subject for publishing this alert.
// Panics if the tenant or source would create an invalid NATS subject.
func (e *AlertEnvelope) NATSSubject() string {
	subject, err := e.NATSSubjectE()
	if err != nil {
		panic(fmt.Sprintf("AlertEnvelope.NATSSubject: %s", err))
	}
	return subject
}

// Clone returns a deep copy safe for concurrent modification.
func (e AlertEnvelope) Clone() AlertEnvelope {
	clone := e
	if e.Labels != nil {
		clone.Labels = make(map[string]string, len(e.Labels))
		for k, v := range e.Labels {
			clone.Labels[k] = v
		}
	}
	if e.Annotations != nil {
		clone.Annotations = make(map[string]string, len(e.Annotations))
		for k, v := range e.Annotations {
			clone.Annotations[k] = v
		}
	}
	// json.RawMessage is []byte — clone the slice to avoid sharing the underlying array.
	if e.Payload != nil {
		clone.Payload = append(json.RawMessage(nil), e.Payload...)
	}
	return clone
}
