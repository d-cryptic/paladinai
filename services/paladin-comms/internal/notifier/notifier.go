// Package notifier implements Slack and PagerDuty notification dispatch.
// All implementations are safe for concurrent use.
package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Notification carries the fields needed to build an outbound alert message.
type Notification struct {
	TenantID   string
	IncidentID string
	Severity   string // P1|P2|P3|P4
	Title      string
	Summary    string
	RunbookURL string
}

// Notifier sends an outbound notification for an incident.
type Notifier interface {
	Notify(ctx context.Context, n Notification) error
}

// severityColor maps severity to Slack attachment color codes.
func severityColor(sev string) string {
	switch sev {
	case "P1":
		return "danger"
	case "P2":
		return "warning"
	case "P3":
		return "good"
	default:
		return "#aaaaaa"
	}
}

// pdSeverity maps PaladinAI severity to PagerDuty Events API severity.
func pdSeverity(sev string) string {
	switch sev {
	case "P1":
		return "critical"
	case "P2":
		return "error"
	case "P3":
		return "warning"
	default:
		return "info"
	}
}

// ── SlackNotifier ────────────────────────────────────────────────────────────

// SlackNotifier sends notifications to a Slack incoming webhook.
type SlackNotifier struct {
	webhookURL string
	client     *http.Client
}

// NewSlackNotifier creates a SlackNotifier targeting the given Slack webhook URL.
func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify sends the notification to Slack.
func (s *SlackNotifier) Notify(ctx context.Context, n Notification) error {
	text := fmt.Sprintf("[%s] %s — %s", n.Severity, n.TenantID, n.Title)
	fields := []map[string]string{
		{"title": "Incident", "value": n.IncidentID, "short": "true"},
		{"title": "Severity", "value": n.Severity, "short": "true"},
	}
	if n.Summary != "" {
		fields = append(fields, map[string]string{"title": "Summary", "value": n.Summary, "short": "false"})
	}
	if n.RunbookURL != "" {
		fields = append(fields, map[string]string{"title": "Runbook", "value": n.RunbookURL, "short": "false"})
	}

	payload := map[string]any{
		"text": text,
		"attachments": []map[string]any{
			{
				"color":  severityColor(n.Severity),
				"fields": fields,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("slack: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: send: %w", err)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("slack: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// ── PagerDutyNotifier ────────────────────────────────────────────────────────

const pdEventsURL = "https://events.pagerduty.com/v2/enqueue"

// PagerDutyNotifier sends notifications to PagerDuty Events API v2.
type PagerDutyNotifier struct {
	routingKey string
	eventsURL  string // override for testing
	client     *http.Client
}

// NewPagerDutyNotifier creates a PagerDutyNotifier using the given routing key.
func NewPagerDutyNotifier(routingKey string) *PagerDutyNotifier {
	return &PagerDutyNotifier{
		routingKey: routingKey,
		eventsURL:  pdEventsURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify sends a trigger event to PagerDuty.
func (p *PagerDutyNotifier) Notify(ctx context.Context, n Notification) error {
	payload := map[string]any{
		"routing_key":  p.routingKey,
		"event_action": "trigger",
		"dedup_key":    n.IncidentID,
		"payload": map[string]any{
			"summary":  fmt.Sprintf("[%s] %s", n.Severity, n.Title),
			"severity": pdSeverity(n.Severity),
			"source":   n.TenantID,
			"custom_details": map[string]string{
				"incident_id": n.IncidentID,
				"summary":     n.Summary,
				"runbook_url": n.RunbookURL,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("pagerduty: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.eventsURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("pagerduty: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("pagerduty: send: %w", err)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()

	// PD Events API v2 returns 202 Accepted on success.
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("pagerduty: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// ── MultiNotifier ────────────────────────────────────────────────────────────

// MultiNotifier fans out to multiple Notifier implementations concurrently.
// All notifiers are called even if some fail; all errors are collected.
type MultiNotifier struct {
	notifiers []Notifier
}

// NewMultiNotifier creates a MultiNotifier from one or more notifiers.
func NewMultiNotifier(notifiers ...Notifier) *MultiNotifier {
	return &MultiNotifier{notifiers: notifiers}
}

// Notify calls all registered notifiers concurrently and returns a combined error if any fail.
func (m *MultiNotifier) Notify(ctx context.Context, n Notification) error {
	if len(m.notifiers) == 0 {
		return nil
	}
	errs := make([]error, len(m.notifiers))
	var wg sync.WaitGroup
	for i, notifier := range m.notifiers {
		wg.Add(1)
		go func(idx int, nt Notifier) {
			defer wg.Done()
			errs[idx] = nt.Notify(ctx, n)
		}(i, notifier)
	}
	wg.Wait()

	var failed []error
	for _, err := range errs {
		if err != nil {
			failed = append(failed, err)
		}
	}
	if len(failed) == 0 {
		return nil
	}
	return fmt.Errorf("multi-notifier: %d/%d failed: %w", len(failed), len(m.notifiers), errors.Join(failed...))
}
