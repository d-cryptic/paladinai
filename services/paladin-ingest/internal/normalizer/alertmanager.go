// Package normalizer converts integration-specific payloads to AlertEnvelope.
package normalizer

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/sanitizer"
)

// AlertmanagerWebhook is the Alertmanager v2 webhook payload shape.
type AlertmanagerWebhook struct {
	Version           string                 `json:"version"`
	GroupKey          string                 `json:"groupKey"`
	Status            string                 `json:"status"` // "firing" | "resolved"
	Receiver          string                 `json:"receiver"`
	GroupLabels       map[string]string      `json:"groupLabels"`
	CommonLabels      map[string]string      `json:"commonLabels"`
	CommonAnnotations map[string]string      `json:"commonAnnotations"`
	ExternalURL       string                 `json:"externalURL"`
	Alerts            []AlertmanagerAlert    `json:"alerts"`
}

type AlertmanagerAlert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
	GeneratorURL string           `json:"generatorURL"`
	Fingerprint string            `json:"fingerprint"`
}

// NormalizeAlertmanager converts an Alertmanager webhook payload into AlertEnvelopes.
// One webhook can carry multiple alerts; each becomes a separate envelope.
func NormalizeAlertmanager(tenantID string, raw json.RawMessage) ([]alert.AlertEnvelope, error) {
	var webhook AlertmanagerWebhook
	if err := json.Unmarshal(raw, &webhook); err != nil {
		return nil, fmt.Errorf("unmarshal alertmanager payload: %w", err)
	}

	now := time.Now().UTC()
	envelopes := make([]alert.AlertEnvelope, 0, len(webhook.Alerts))

	for _, a := range webhook.Alerts {
		// Merge labels: alert-level overrides group-level, then sanitize.
		rawLabels := mergeLabels(webhook.CommonLabels, a.Labels)
		rawAnnotations := mergeLabels(webhook.CommonAnnotations, a.Annotations)
		labels, _ := sanitizer.SanitizeMap(rawLabels)
		annotations, _ := sanitizer.SanitizeMap(rawAnnotations)

		fp := alert.ComputeFingerprint(alert.SourceAlertmanager, labels)

		env := alert.AlertEnvelope{
			ID:          uuid.New().String(),
			Fingerprint: fp,
			TenantID:    tenantID,
			Severity:    mapSeverity(labels),
			Status:      mapStatus(a.Status),
			Source:      alert.SourceAlertmanager,
			Title:       labels["alertname"],
			Description: annotations["description"],
			Labels:      labels,
			Annotations: annotations,
			StartsAt:    a.StartsAt.UTC(),
			ReceivedAt:  now,
			Runbook:     annotations["runbook_url"],
			Dashboard:   annotations["dashboard_url"],
			AgentState:  "pending",
		}

		if a.Status == "resolved" && !a.EndsAt.IsZero() {
			t := a.EndsAt.UTC()
			env.EndsAt = &t
		}

		// Preserve per-alert payload for the audit trail.
		// We marshal only the individual alert, not the full batch, to avoid
		// storing N copies of an 8MB payload in NATS.
		if alertPayload, err := json.Marshal(a); err == nil {
			env.Payload = alertPayload
		}

		envelopes = append(envelopes, env)
	}

	return envelopes, nil
}

func mapSeverity(labels map[string]string) alert.Severity {
	switch labels["severity"] {
	case "critical":
		return alert.SeverityP1
	case "high":
		return alert.SeverityP2
	case "warning":
		return alert.SeverityP3
	case "info":
		return alert.SeverityP4
	default:
		return alert.SeverityUnknown
	}
}

func mapStatus(s string) alert.Status {
	switch s {
	case "firing":
		return alert.StatusFiring
	case "resolved":
		return alert.StatusResolved
	default:
		// Return firing as a conservative default but callers may inspect
		// the raw payload to detect the actual status.
		return alert.StatusFiring
	}
}

func mergeLabels(base, override map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}
