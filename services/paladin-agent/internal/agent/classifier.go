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
{"intent":"log_analysis|metric_spike|service_down|oom","agent_type":"triage|rca","severity":"P1|P2|P3|P4","confidence":0.9}

Rules:
- P1 or service_down → rca (always needs deep root cause analysis)
- oom → rca (memory issues need deep investigation)
- P2/P3/P4 + metric_spike → triage
- P4 or log_analysis → triage
- confidence: how certain you are (0.0-1.0)`

// ClassificationResult is the structured output from the classifier.
type ClassificationResult struct {
	Intent     string  `json:"intent"`
	AgentType  string  `json:"agent_type"` // "triage" or "rca"
	Severity   string  `json:"severity"`   // "P1" | "P2" | "P3" | "P4"
	Confidence float32 `json:"confidence"`
}

var validIntents = map[string]bool{
	"log_analysis": true,
	"metric_spike": true,
	"service_down": true,
	"oom":          true,
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
	if severity == "P1" || intent == "service_down" || intent == "oom" {
		return "rca"
	}
	return "triage"
}
