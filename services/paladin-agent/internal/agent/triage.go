// Package agent implements the PaladinAI alert-response agent graph.
// Phase 4 skeleton: Triage agent classifies severity and extracts structured
// context from the AlertEnvelope. Downstream agents (RCA, Runbook, Comms)
// are stubs wired in later phases.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
  "summary": "<one-sentence summary of what is wrong, max 200 chars>",
  "likely_cause": "<brief hypothesis of root cause, max 400 chars>",
  "affected_services": ["<service name>"],
  "recommended_action": "<immediate next step, max 200 chars>",
  "needs_human": <true if P1/P2 or escalation needed, else false>
}

Rules:
- P1: production outage, revenue impact, data loss risk
- P2: significant degradation, multiple users affected
- P3: minor degradation, single service issue
- P4: informational, no user impact
- Use the alert labels and annotations to fill the fields
- Do NOT include any text outside the JSON object`

// validSeverities is the set of accepted values for ConfirmedSeverity.
var validSeverities = map[string]bool{"P1": true, "P2": true, "P3": true, "P4": true}

const (
	maxSummaryLen   = 200
	maxCauseLen     = 400
	maxActionLen    = 200
	maxServiceLen   = 64
	maxServiceCount = 20
)

// TriageResult is the structured output produced by the triage agent.
type TriageResult struct {
	ConfirmedSeverity string   `json:"confirmed_severity"`
	Summary           string   `json:"summary"`
	LikelyCause       string   `json:"likely_cause"`
	AffectedServices  []string `json:"affected_services"`
	RecommendedAction string   `json:"recommended_action"`
	NeedsHuman        bool     `json:"needs_human"`
	// Degraded is true when the result was synthesised from a non-JSON model response.
	Degraded bool `json:"degraded,omitempty"`
}

// validate clamps field lengths and normalises ConfirmedSeverity to the platform enum.
// Returns an error if the severity is not a known value (after normalisation).
func (r *TriageResult) validate() error {
	r.ConfirmedSeverity = strings.ToUpper(strings.TrimSpace(r.ConfirmedSeverity))
	if !validSeverities[r.ConfirmedSeverity] {
		return fmt.Errorf("unknown severity %q", r.ConfirmedSeverity)
	}

	r.Summary = truncate(r.Summary, maxSummaryLen)
	r.LikelyCause = truncate(r.LikelyCause, maxCauseLen)
	r.RecommendedAction = truncate(r.RecommendedAction, maxActionLen)

	if len(r.AffectedServices) > maxServiceCount {
		r.AffectedServices = r.AffectedServices[:maxServiceCount]
	}
	for i, s := range r.AffectedServices {
		r.AffectedServices[i] = truncate(s, maxServiceLen)
	}
	return nil
}

// truncate clips s to max runes — safe for multibyte UTF-8.
func truncate(s string, max int) string {
	i := 0
	for j := range s {
		if i >= max {
			return s[:j]
		}
		i++
	}
	return s
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
		MessageModifier:  react.NewPersonaModifier(triageSystemPrompt), //nolint:staticcheck
		MaxStep:          10,
		GraphName:        "PaladinTriageAgent",
	})
	if err != nil {
		return nil, fmt.Errorf("triage agent: %w", err)
	}
	return &TriageAgent{agent: a, log: log}, nil
}

// Triage classifies the given AlertEnvelope and returns a validated TriageResult.
// On non-JSON model output it degrades gracefully, flagging result.Degraded=true.
func (t *TriageAgent) Triage(ctx context.Context, env *alert.AlertEnvelope) (*TriageResult, error) {
	alertJSON, err := json.Marshal(map[string]any{
		"title":       env.Title,
		"severity":    string(env.Severity),
		"status":      string(env.Status),
		"labels":      env.Labels,
		"annotations": env.Annotations,
		"description": env.Description,
		"starts_at":   env.StartsAt,
	})
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
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		// Graceful degradation — log, mark degraded, derive from envelope.
		t.log.Warn("triage: model returned non-JSON, degrading",
			zap.String("content_prefix", truncate(resp.Content, 200)),
			zap.Error(err),
		)
		result = TriageResult{
			ConfirmedSeverity: string(env.Severity),
			Summary:           truncate(resp.Content, maxSummaryLen),
			NeedsHuman:        env.Severity == alert.SeverityP1 || env.Severity == alert.SeverityP2,
			Degraded:          true,
		}
		// Ensure degraded severity is still valid
		if !validSeverities[result.ConfirmedSeverity] {
			result.ConfirmedSeverity = "P3"
		}
		return &result, nil
	}

	if err := result.validate(); err != nil {
		t.log.Warn("triage: invalid result from model, degrading",
			zap.Error(err),
			zap.String("raw_severity", result.ConfirmedSeverity),
		)
		result.ConfirmedSeverity = string(env.Severity)
		if !validSeverities[result.ConfirmedSeverity] {
			result.ConfirmedSeverity = "P3"
		}
		result.Degraded = true
	}

	t.log.Info("triage complete",
		zap.String("fingerprint", env.Fingerprint),
		zap.String("severity", result.ConfirmedSeverity),
		zap.Bool("needs_human", result.NeedsHuman),
		zap.Bool("degraded", result.Degraded),
	)
	return &result, nil
}
