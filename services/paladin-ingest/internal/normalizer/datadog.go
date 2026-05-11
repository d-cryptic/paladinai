// Package normalizer converts integration-specific payloads to AlertEnvelope.
package normalizer

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/sanitizer"
)

// DatadogWebhook is the shape of a Datadog alert webhook payload.
// Datadog tags arrive as a comma-separated string of "key:value" pairs.
type DatadogWebhook struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Text           string `json:"text"`
	DateHappened   int64  `json:"date_happened"`
	Priority       string `json:"priority"`
	Host           string `json:"host"`
	AlertMetric    string `json:"alert_metric"`
	AlertStatus    string `json:"alert_status"`
	AlertType      string `json:"alert_type"`
	Tags           string `json:"tags"`
	AggregationKey string `json:"aggregation_key"`
	EventType      string `json:"event_type"`
}

// NormalizeDatadog converts a Datadog webhook payload into AlertEnvelopes.
// A single webhook represents one alert, so the returned slice always has
// length one on success.
func NormalizeDatadog(tenantID string, raw json.RawMessage, log *zap.Logger) ([]alert.AlertEnvelope, error) {
	if log == nil {
		log = zap.NewNop()
	}
	var payload DatadogWebhook
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal datadog payload: %w", err)
	}

	rawLabels := parseDatadogTags(payload.Tags)
	if payload.Host != "" {
		rawLabels["host"] = payload.Host
	}
	if payload.AlertType != "" {
		rawLabels["alert_type"] = payload.AlertType
	}
	if payload.AlertMetric != "" {
		rawLabels["alert_metric"] = payload.AlertMetric
	}
	if payload.AggregationKey != "" {
		rawLabels["aggregation_key"] = payload.AggregationKey
	}
	if payload.Title != "" {
		rawLabels["alertname"] = payload.Title
	}

	rawAnnotations := map[string]string{}
	if payload.Text != "" {
		rawAnnotations["description"] = payload.Text
	}

	labelResult := sanitizer.SanitizeMap(rawLabels)
	annotationResult := sanitizer.SanitizeMap(rawAnnotations)
	labels := labelResult.Values
	annotations := annotationResult.Values

	if len(labelResult.ChangedKeys) > 0 || len(labelResult.DroppedKeys) > 0 {
		log.Warn("prompt injection sanitized in alert labels",
			zap.String("tenant_id", tenantID),
			zap.Strings("changed_keys", labelResult.ChangedKeys),
			zap.Strings("dropped_keys", labelResult.DroppedKeys),
		)
	}
	if len(annotationResult.ChangedKeys) > 0 || len(annotationResult.DroppedKeys) > 0 {
		log.Warn("prompt injection sanitized in alert annotations",
			zap.String("tenant_id", tenantID),
			zap.Strings("changed_keys", annotationResult.ChangedKeys),
			zap.Strings("dropped_keys", annotationResult.DroppedKeys),
		)
	}

	status := mapDatadogStatus(payload.AlertStatus)
	severity := mapDatadogSeverity(payload.AlertStatus, payload.Priority)
	fp := alert.ComputeFingerprint(alert.SourceDatadog, labels)

	startsAt := time.Unix(payload.DateHappened, 0).UTC()
	if payload.DateHappened == 0 {
		startsAt = time.Now().UTC()
	}

	env := alert.AlertEnvelope{
		ID:          uuid.New().String(),
		Fingerprint: fp,
		TenantID:    tenantID,
		Severity:    severity,
		Status:      status,
		Source:      alert.SourceDatadog,
		Title:       payload.Title,
		Description: annotations["description"],
		Labels:      labels,
		Annotations: annotations,
		StartsAt:    startsAt,
		ReceivedAt:  time.Now().UTC(),
		AgentState:  "pending",
		Payload:     append(json.RawMessage(nil), raw...),
	}

	if status == alert.StatusResolved {
		t := time.Now().UTC()
		env.EndsAt = &t
	}

	return []alert.AlertEnvelope{env}, nil
}

// parseDatadogTags converts "key:value,key2:value2" into a label map.
func parseDatadogTags(tags string) map[string]string {
	out := map[string]string{}
	if tags == "" {
		return out
	}
	for _, raw := range strings.Split(tags, ",") {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		if idx := strings.Index(t, ":"); idx > 0 {
			out[t[:idx]] = t[idx+1:]
		} else {
			out[t] = ""
		}
	}
	return out
}

func mapDatadogStatus(s string) alert.Status {
	switch strings.ToLower(s) {
	case "triggered", "warn", "warning":
		return alert.StatusFiring
	case "recovered", "resolved", "ok":
		return alert.StatusResolved
	default:
		return alert.StatusFiring
	}
}

func mapDatadogSeverity(status, priority string) alert.Severity {
	st := strings.ToLower(status)
	pri := strings.ToLower(priority)
	if st == "recovered" || st == "resolved" {
		return alert.SeverityP4
	}
	if st == "triggered" {
		switch pri {
		case "urgent", "high":
			return alert.SeverityP1
		case "normal":
			return alert.SeverityP2
		}
	}
	return alert.SeverityP3
}
