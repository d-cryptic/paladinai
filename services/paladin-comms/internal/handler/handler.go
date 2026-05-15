// Package handler implements the NATS consumer for paladin-comms.
// It reads triage results from NATS and dispatches outbound notifications.
package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/paladinai/paladinai/internal/telemetry"
	"github.com/paladinai/paladinai/services/paladin-comms/internal/notifier"
	"go.uber.org/zap"
)

// severityRank maps severity labels to integers for threshold comparison.
var severityRank = map[string]int{
	"P1": 1,
	"P2": 2,
	"P3": 3,
	"P4": 4,
}

// triageResult is the JSON shape published by paladin-agent after triage/RCA.
// Only fields needed by comms are parsed.
type triageResult struct {
	TenantID      string `json:"tenant_id"`
	IncidentID    string `json:"incident_id"`
	Fingerprint   string `json:"fingerprint"`
	Severity      string `json:"severity"`
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	RunbookURL    string `json:"runbook_url"`
	CorrelationID string `json:"correlation_id"`
}

type agentResult struct {
	Envelope struct {
		TenantID      string `json:"tenant_id"`
		Fingerprint   string `json:"fingerprint"`
		Severity      string `json:"severity"`
		Title         string `json:"title"`
		Runbook       string `json:"runbook"`
		CorrelationID string `json:"correlation_id"`
	} `json:"envelope"`
	Triage struct {
		ConfirmedSeverity string `json:"confirmed_severity"`
		Summary           string `json:"summary"`
		RecommendedAction string `json:"recommended_action"`
	} `json:"triage"`
}

// Handler consumes triage result messages from NATS and dispatches notifications.
type Handler struct {
	notifier    notifier.Notifier
	minSeverity string // notify for this severity and above
	log         *zap.Logger
}

// New creates a Handler. minSeverity is the worst (lowest rank) severity to
// notify on — e.g. "P2" means notify on P1 and P2 but not P3/P4.
func New(n notifier.Notifier, minSeverity string, log *zap.Logger) *Handler {
	if minSeverity == "" {
		minSeverity = "P2"
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{notifier: n, minSeverity: minSeverity, log: log}
}

// ProcessMessage handles one NATS message. It acks on success and naks on error.
func (h *Handler) ProcessMessage(ctx context.Context, msg jetstream.Msg) error {
	ctx = telemetry.ExtractTraceContext(ctx, msg.Headers())
	result, err := parseTriageResult(msg.Data())
	if err != nil {
		_ = msg.Nak()
		return fmt.Errorf("comms: unmarshal triage result: %w", err)
	}

	if !h.shouldNotify(result.Severity) {
		h.log.Debug("comms: skipping notification below threshold",
			zap.String("severity", result.Severity),
			zap.String("min_severity", h.minSeverity),
			zap.String("fingerprint", result.Fingerprint),
		)
		_ = msg.Ack()
		return nil
	}

	n := notifier.Notification{
		TenantID:   result.TenantID,
		IncidentID: result.IncidentID,
		Severity:   result.Severity,
		Title:      result.Title,
		Summary:    result.Summary,
		RunbookURL: result.RunbookURL,
	}

	if err := h.notifier.Notify(ctx, n); err != nil {
		h.log.Warn("comms: notification failed",
			zap.String("tenant", result.TenantID),
			zap.String("incident", result.IncidentID),
			zap.Error(err),
		)
		_ = msg.Nak()
		return fmt.Errorf("comms: notify: %w", err)
	}

	h.log.Info("comms: notification sent",
		zap.String("tenant", result.TenantID),
		zap.String("incident", result.IncidentID),
		zap.String("severity", result.Severity),
	)
	_ = msg.Ack()
	return nil
}

func parseTriageResult(data []byte) (triageResult, error) {
	var result triageResult
	if err := json.Unmarshal(data, &result); err != nil {
		return triageResult{}, err
	}
	if result.TenantID != "" {
		return result, nil
	}

	var wrapped agentResult
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return triageResult{}, err
	}
	result = triageResult{
		TenantID:      wrapped.Envelope.TenantID,
		IncidentID:    firstNonEmpty(wrapped.Envelope.CorrelationID, wrapped.Envelope.Fingerprint),
		Fingerprint:   wrapped.Envelope.Fingerprint,
		Severity:      firstNonEmpty(wrapped.Triage.ConfirmedSeverity, wrapped.Envelope.Severity),
		Title:         wrapped.Envelope.Title,
		Summary:       firstNonEmpty(wrapped.Triage.Summary, wrapped.Triage.RecommendedAction),
		RunbookURL:    wrapped.Envelope.Runbook,
		CorrelationID: wrapped.Envelope.CorrelationID,
	}
	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// shouldNotify returns true when the alert severity is at or above the configured threshold.
func (h *Handler) shouldNotify(severity string) bool {
	rank, ok := severityRank[severity]
	if !ok {
		return false
	}
	minRank, ok := severityRank[h.minSeverity]
	if !ok {
		return true // unknown threshold — notify by default
	}
	return rank <= minRank
}
