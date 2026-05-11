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

// PagerDutyWebhook is the PagerDuty v3 webhook envelope.
type PagerDutyWebhook struct {
	Event PagerDutyEvent `json:"event"`
}

// PagerDutyEvent is the inner event payload.
type PagerDutyEvent struct {
	ID           string             `json:"id"`
	EventType    string             `json:"event_type"`
	ResourceType string             `json:"resource_type"`
	OccurredAt   time.Time          `json:"occurred_at"`
	Data         PagerDutyEventData `json:"data"`
}

// PagerDutyEventData carries the incident fields.
type PagerDutyEventData struct {
	ID             string            `json:"id"`
	Summary        string            `json:"summary"`
	Status         string            `json:"status"`
	Urgency        string            `json:"urgency"`
	IncidentNumber int               `json:"incident_number"`
	Title          string            `json:"title"`
	Service        PagerDutyService  `json:"service"`
	Teams          []PagerDutyTeam   `json:"teams"`
	Priority       PagerDutyPriority `json:"priority"`
}

// PagerDutyService is the affected service block in the event.
type PagerDutyService struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

// PagerDutyTeam is a team assignment block.
type PagerDutyTeam struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PagerDutyPriority is the priority block.
type PagerDutyPriority struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NormalizePagerDuty converts a PagerDuty v3 webhook into AlertEnvelopes.
func NormalizePagerDuty(tenantID string, raw json.RawMessage, log *zap.Logger) ([]alert.AlertEnvelope, error) {
	if log == nil {
		log = zap.NewNop()
	}
	var payload PagerDutyWebhook
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal pagerduty payload: %w", err)
	}

	ev := payload.Event
	data := ev.Data

	title := data.Title
	if title == "" {
		title = data.Summary
	}

	rawLabels := map[string]string{}
	if data.Service.Name != "" {
		rawLabels["service"] = data.Service.Name
	}
	if len(data.Teams) > 0 && data.Teams[0].Name != "" {
		rawLabels["team"] = data.Teams[0].Name
	}
	if data.Urgency != "" {
		rawLabels["urgency"] = data.Urgency
	}
	if data.Priority.Name != "" {
		rawLabels["priority"] = data.Priority.Name
	}
	if title != "" {
		rawLabels["alertname"] = title
	}
	if data.ID != "" {
		rawLabels["incident_id"] = data.ID
	}

	rawAnnotations := map[string]string{}
	if data.Summary != "" {
		rawAnnotations["description"] = data.Summary
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

	status := mapPagerDutyStatus(ev.EventType)
	severity := mapPagerDutySeverity(data.Urgency, data.Status)
	fp := alert.ComputeFingerprint(alert.SourcePagerDuty, labels)

	startsAt := ev.OccurredAt.UTC()
	if startsAt.IsZero() {
		startsAt = time.Now().UTC()
	}

	env := alert.AlertEnvelope{
		ID:          uuid.New().String(),
		Fingerprint: fp,
		TenantID:    tenantID,
		Severity:    severity,
		Status:      status,
		Source:      alert.SourcePagerDuty,
		Title:       title,
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

func mapPagerDutyStatus(eventType string) alert.Status {
	switch strings.ToLower(eventType) {
	case "incident.resolved":
		return alert.StatusResolved
	case "incident.triggered", "incident.acknowledged", "incident.escalated", "incident.reassigned":
		return alert.StatusFiring
	default:
		return alert.StatusFiring
	}
}

func mapPagerDutySeverity(urgency, status string) alert.Severity {
	u := strings.ToLower(urgency)
	st := strings.ToLower(status)
	switch u {
	case "high":
		if st == "triggered" {
			return alert.SeverityP1
		}
		return alert.SeverityP2
	case "low":
		return alert.SeverityP3
	default:
		return alert.SeverityP2
	}
}
