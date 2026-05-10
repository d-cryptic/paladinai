// Package agent implements the PaladinAI alert-response agent graph.
// Phase 4 skeleton: Triage agent classifies severity and extracts structured
// context from the AlertEnvelope. Downstream agents (RCA, Runbook, Comms)
// are stubs wired in later phases.
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/paladinai/paladinai/internal/alert"
	"go.uber.org/zap"
)

const triageSystemPrompt = `You are PaladinAI's triage agent. Your job is to analyse an incoming alert and produce a structured JSON assessment.

Given an alert, you MUST respond with ONLY valid JSON matching this schema:
{
  "confirmed_severity": "<P1|P2|P3|P4>",
  "summary": "<one-sentence summary of what is wrong>",
  "likely_cause": "<brief hypothesis of root cause>",
  "affected_services": ["<service name>", ...],
  "recommended_action": "<immediate next step>",
  "needs_human": <true if P1/P2 or escalation needed, else false>
}

Rules:
- P1: production outage, revenue impact, data loss risk
- P2: significant degradation, multiple users affected
- P3: minor degradation, single service issue
- P4: informational, no user impact
- Use the alert labels and annotations to fill the fields
- Do NOT include any text outside the JSON object`

// TriageResult is the structured output produced by the triage agent.
type TriageResult struct {
	ConfirmedSeverity  string   `json:"confirmed_severity"`
	Summary            string   `json:"summary"`
	LikelyCause        string   `json:"likely_cause"`
	AffectedServices   []string `json:"affected_services"`
	RecommendedAction  string   `json:"recommended_action"`
	NeedsHuman         bool     `json:"needs_human"`
}

// TriageAgent wraps an Eino ReAct agent for alert triage.
type TriageAgent struct {
	agent *react.Agent
	log   *zap.Logger
}

// NewTriageAgent builds a TriageAgent backed by the given ToolCallingChatModel.
// Tools may be empty for the Phase 4 skeleton; they are added in later phases.
func NewTriageAgent(ctx context.Context, m model.ToolCallingChatModel, log *zap.Logger) (*TriageAgent, error) {
	a, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: m,
		MessageModifier:  react.NewPersonaModifier(triageSystemPrompt),
		MaxStep:          10,
		GraphName:        "PaladinTriageAgent",
	})
	if err != nil {
		return nil, fmt.Errorf("triage agent: %w", err)
	}
	return &TriageAgent{agent: a, log: log}, nil
}

// Triage classifies the given AlertEnvelope and returns a TriageResult.
// The alert is serialised to JSON and sent as the user message.
func (t *TriageAgent) Triage(ctx context.Context, env *alert.AlertEnvelope) (*TriageResult, error) {
	alertJSON, err := json.MarshalIndent(map[string]any{
		"title":       env.Title,
		"severity":    string(env.Severity),
		"status":      string(env.Status),
		"labels":      env.Labels,
		"annotations": env.Annotations,
		"description": env.Description,
		"starts_at":   env.StartsAt,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("triage: marshal alert: %w", err)
	}

	messages := []*schema.Message{
		schema.UserMessage(string(alertJSON)),
	}

	resp, err := t.agent.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("triage: generate: %w", err)
	}

	var result TriageResult
	content := resp.Content
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		t.log.Warn("triage: failed to parse JSON response, using raw content",
			zap.String("content", content),
			zap.Error(err),
		)
		// Graceful degradation: return a minimal result with the raw summary
		return &TriageResult{
			ConfirmedSeverity: string(env.Severity),
			Summary:           content,
			NeedsHuman:        env.Severity == alert.SeverityP1 || env.Severity == alert.SeverityP2,
		}, nil
	}

	t.log.Info("triage complete",
		zap.String("fingerprint", env.Fingerprint),
		zap.String("severity", result.ConfirmedSeverity),
		zap.Bool("needs_human", result.NeedsHuman),
	)
	return &result, nil
}
