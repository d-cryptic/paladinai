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

// SlackEventWrapper is the outer Slack Events API envelope.
type SlackEventWrapper struct {
	Type      string     `json:"type"`
	EventID   string     `json:"event_id"`
	TeamID    string     `json:"team_id"`
	Event     SlackEvent `json:"event"`
	EventTime int64      `json:"event_time"`
	APIAppID  string     `json:"api_app_id"`
}

// SlackEvent is the inner event.
type SlackEvent struct {
	Type        string `json:"type"`
	SubType     string `json:"subtype,omitempty"`
	Text        string `json:"text"`
	User        string `json:"user"`
	Channel     string `json:"channel"`
	Timestamp   string `json:"ts"`
	BotID       string `json:"bot_id,omitempty"`
	BotUsername string `json:"username,omitempty"`
}

// NormalizeSlack converts a Slack Events API payload into AlertEnvelopes.
// Only message events that contain alert keywords are surfaced as incidents.
// Slack is primarily an outbound notification channel; inbound events are
// used for bot-forwarded alerts from other tools (e.g. Datadog → Slack bot).
func NormalizeSlack(tenantID string, raw json.RawMessage, log *zap.Logger) ([]alert.AlertEnvelope, error) {
	if log == nil {
		log = zap.NewNop()
	}

	var wrapper SlackEventWrapper
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("unmarshal slack payload: %w", err)
	}

	// URL verification challenge — Slack sends this during integration setup.
	// The webhook handler is expected to respond with the challenge value;
	// the normalizer returns an empty slice so nothing is published to NATS.
	if wrapper.Type == "url_verification" {
		log.Info("slack url_verification handshake received", zap.String("tenant_id", tenantID))
		return nil, nil
	}

	ev := wrapper.Event
	if ev.Type != "message" {
		// Non-message events (e.g. reaction_added) are not normalized as alerts.
		return nil, nil
	}

	// Skip bot messages that originated from PaladinAI itself to prevent loops.
	if ev.BotUsername != "" && strings.Contains(strings.ToLower(ev.BotUsername), "paladin") {
		return nil, nil
	}

	text := strings.TrimSpace(ev.Text)
	if text == "" {
		return nil, nil
	}

	severity := inferSlackSeverity(text)
	title := truncate(text, 120)

	rawLabels := map[string]string{
		"alertname": title,
		"source":    "slack",
		"channel":   ev.Channel,
	}
	if ev.User != "" {
		rawLabels["slack_user"] = ev.User
	}
	if wrapper.TeamID != "" {
		rawLabels["slack_team"] = wrapper.TeamID
	}

	rawAnnotations := map[string]string{
		"description": text,
	}

	labelResult := sanitizer.SanitizeMap(rawLabels)
	annotationResult := sanitizer.SanitizeMap(rawAnnotations)
	labels := labelResult.Values
	annotations := annotationResult.Values

	if len(labelResult.ChangedKeys) > 0 || len(labelResult.DroppedKeys) > 0 {
		log.Warn("prompt injection sanitized in slack labels",
			zap.String("tenant_id", tenantID),
			zap.Strings("changed_keys", labelResult.ChangedKeys),
			zap.Strings("dropped_keys", labelResult.DroppedKeys),
		)
	}

	startsAt := time.Now().UTC()
	if wrapper.EventTime > 0 {
		startsAt = time.Unix(wrapper.EventTime, 0).UTC()
	}

	fp := alert.ComputeFingerprint(alert.SourceSlack, labels)

	env := alert.AlertEnvelope{
		ID:          uuid.New().String(),
		Fingerprint: fp,
		TenantID:    tenantID,
		Severity:    severity,
		Status:      alert.StatusFiring,
		Source:      alert.SourceSlack,
		Title:       title,
		Description: text,
		Labels:      labels,
		Annotations: annotations,
		StartsAt:    startsAt,
		ReceivedAt:  time.Now().UTC(),
		AgentState:  "pending",
		Payload:     append(json.RawMessage(nil), raw...),
	}

	return []alert.AlertEnvelope{env}, nil
}

// inferSlackSeverity guesses severity from common alerting keywords in the text.
func inferSlackSeverity(text string) alert.Severity {
	lower := strings.ToLower(text)
	switch {
	case containsAny(lower, "p0", "critical", "down", "outage", "severity: 1", "[p1]", "sev1"):
		return alert.SeverityP1
	case containsAny(lower, "p2", "high", "degraded", "latency spike", "sev2", "[p2]"):
		return alert.SeverityP2
	case containsAny(lower, "p3", "warning", "elevated", "sev3", "[p3]"):
		return alert.SeverityP3
	default:
		return alert.SeverityP4
	}
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
