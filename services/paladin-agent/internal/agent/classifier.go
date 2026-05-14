// Package agent: classifier.go implements the Stage 3 classifier agent.
// The classifier is a lightweight, single-shot LLM call that determines the
// intent, severity, and target specialist (triage vs rca) for an alert.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/tenantguard"
	"go.uber.org/zap"
)

const classifierSystemPrompt = `You are PaladinAI's routing agent. Classify this alert and determine which specialist to route to.

OUTPUT: JSON only, no prose outside the JSON:
{"intent":"log_analysis|metric_spike|service_down|oom|network_issue|capacity_warning|config_drift|security_alert","agent_type":"triage|rca","severity":"P1|P2|P3|P4","confidence":0.9}

Rules:
- service_down, oom, security_alert, or P1 → rca
- network_issue or capacity_warning at P1/P2 → rca; otherwise triage
- metric_spike, log_analysis, config_drift → triage unless severity is P1
- 5xx, 500, unavailable, healthcheck failing, connection refused, primary down, timeout cascade → service_down
- CPU, latency, p99, saturation, lag, disk IO, replication delay, error-rate-but-not-outage → metric_spike
- certificate expiry, TLS expiry, log rate, audit/event-only warnings → log_analysis
- configmap, ingress drift, source-of-truth mismatch → config_drift
- disk full soon, pool exhausted, node pressure, bandwidth saturated, queue/log backlog → capacity_warning
- packet loss, DNS, upstream connection errors, egress network symptoms → network_issue
- confidence: how certain you are (0.0-1.0)`

// ClassificationResult is the structured output from the classifier.
type ClassificationResult struct {
	Intent     string  `json:"intent"`
	AgentType  string  `json:"agent_type"` // "triage" or "rca"
	Severity   string  `json:"severity"`   // "P1" | "P2" | "P3" | "P4"
	Confidence float32 `json:"confidence"`
}

var validIntents = map[string]bool{
	"log_analysis":     true,
	"metric_spike":     true,
	"service_down":     true,
	"oom":              true,
	"network_issue":    true,
	"capacity_warning": true,
	"config_drift":     true,
	"security_alert":   true,
}

// ClassifierAgent classifies an alert and determines routing.
type ClassifierAgent struct {
	m   model.BaseChatModel
	log *zap.Logger
}

// NewClassifierAgent creates a ClassifierAgent.
// Accepts BaseChatModel — the classifier only calls Generate, no tool binding needed.
// Both model.ChatModel and model.ToolCallingChatModel satisfy this interface.
func NewClassifierAgent(m model.BaseChatModel, log *zap.Logger) *ClassifierAgent {
	if log == nil {
		log = zap.NewNop()
	}
	return &ClassifierAgent{m: m, log: log}
}

// Classify classifies the alert envelope and returns routing instructions.
// On invalid JSON or unknown values it degrades gracefully using severity-based heuristics.
func (c *ClassifierAgent) Classify(ctx context.Context, env *alert.AlertEnvelope) (*ClassificationResult, error) {
	alertJSON, err := json.Marshal(map[string]any{
		"title":       env.Title,
		"severity":    string(env.Severity),
		"labels":      env.Labels,
		"description": tenantguard.WrapAlertContent(env.Description),
	})
	if err != nil {
		return nil, fmt.Errorf("classifier: marshal alert: %w", err)
	}

	resp, err := c.m.Generate(ctx, []*schema.Message{
		schema.SystemMessage(tenantguard.TrustedBoundarySystemPrompt + "\n\n" + classifierSystemPrompt),
		schema.UserMessage(string(alertJSON)),
	})
	if err != nil {
		return nil, fmt.Errorf("classifier: generate: %w", err)
	}

	content := strings.TrimSpace(resp.Content)
	// Strip markdown code fences if present (```json ... ``` or ``` ... ```).
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result ClassificationResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		c.log.Warn("classifier: non-JSON response, using heuristic fallback",
			zap.String("content_prefix", truncate(content, 100)),
		)
		return c.heuristicFallback(env), nil
	}

	result.Severity = strings.ToUpper(strings.TrimSpace(result.Severity))
	if !validSeverities[result.Severity] {
		result.Severity = strings.ToUpper(string(env.Severity))
		if !validSeverities[result.Severity] {
			result.Severity = "P3"
		}
	}
	if !validIntents[result.Intent] {
		result.Intent = "metric_spike"
	}
	if result.AgentType != "triage" && result.AgentType != "rca" {
		result.AgentType = c.routeByHeuristic(result.Severity, result.Intent)
	}

	return &result, nil
}

func (c *ClassifierAgent) heuristicFallback(env *alert.AlertEnvelope) *ClassificationResult {
	sev := strings.ToUpper(string(env.Severity))
	if !validSeverities[sev] {
		sev = "P3"
	}
	agentType := c.routeByHeuristic(sev, "metric_spike")
	return &ClassificationResult{
		Intent:     "metric_spike",
		AgentType:  agentType,
		Severity:   sev,
		Confidence: 0.5,
	}
}

func (c *ClassifierAgent) routeByHeuristic(severity, intent string) string {
	if severity == "P1" || intent == "service_down" || intent == "oom" || intent == "security_alert" {
		return "rca"
	}
	if severity == "P2" && (intent == "network_issue" || intent == "capacity_warning") {
		return "rca"
	}
	return "triage"
}
