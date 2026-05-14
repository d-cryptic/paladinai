// Package agent implements the PaladinAI alert-response agent graph.
// rca.go: Root Cause Analysis agent — runs after triage and produces a
// structured root cause hypothesis with evidence and remediation keywords.
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
	"github.com/paladinai/paladinai/internal/tenantguard"
	"go.uber.org/zap"
)

const rcaSystemPrompt = `You are PaladinAI's root cause analysis agent. Your job is to determine the root cause of an infrastructure or application alert.

You will receive a JSON object containing:
- "alert": the original alert envelope (title, labels, annotations, severity, description)
- "triage": the output of the triage agent (confirmed_severity, summary, likely_cause, affected_services)

You MUST respond with ONLY valid JSON matching this schema:
{
  "root_cause_hypothesis": "<detailed hypothesis of the root cause, max 500 chars>",
  "evidence": ["<specific evidence item>"],
  "confidence": "<HIGH|MEDIUM|LOW>",
  "recommended_fix": "<concrete remediation steps, max 200 chars>",
  "escalation_path": "<team or person to escalate to if unresolved, omit if none>",
  "runbook_keywords": ["<keyword for searching relevant runbooks>"]
}

Rules:
- HIGH confidence: clear evidence points to a single cause
- MEDIUM confidence: evidence is consistent but not conclusive
- LOW confidence: limited data, speculative
- runbook_keywords: 3-10 short keywords to search runbook database (not full sentences)
- evidence: list specific, observable facts from labels/annotations/timing — not opinions
- Do NOT include any text outside the JSON object`

var validConfidences = map[string]bool{"HIGH": true, "MEDIUM": true, "LOW": true}

const (
	maxHypothesisLen  = 500
	maxEvidenceLen    = 200
	maxFixLen         = 200
	maxEscalationLen  = 128
	maxRunbookKWLen   = 64
	maxRunbookKWCount = 20
	maxEvidenceCount  = 20
)

// RCAResult is the structured output produced by the RCA agent.
type RCAResult struct {
	RootCauseHypothesis string   `json:"root_cause_hypothesis"`
	Evidence            []string `json:"evidence"`
	Confidence          string   `json:"confidence"`
	RecommendedFix      string   `json:"recommended_fix"`
	EscalationPath      string   `json:"escalation_path,omitempty"`
	RunbookKeywords     []string `json:"runbook_keywords"`
	// Degraded is true when the result was synthesised from a non-JSON model response.
	Degraded bool `json:"degraded,omitempty"`
}

// validate normalises field values and clamps lengths.
// Unknown confidence values are normalised to MEDIUM rather than returning an error.
func (r *RCAResult) validate() {
	r.Confidence = strings.ToUpper(strings.TrimSpace(r.Confidence))
	if !validConfidences[r.Confidence] {
		r.Confidence = "MEDIUM"
	}

	r.RootCauseHypothesis = truncate(r.RootCauseHypothesis, maxHypothesisLen)
	r.RecommendedFix = truncate(r.RecommendedFix, maxFixLen)
	r.EscalationPath = truncate(r.EscalationPath, maxEscalationLen)

	if r.Evidence == nil {
		r.Evidence = []string{}
	}
	if len(r.Evidence) > maxEvidenceCount {
		r.Evidence = r.Evidence[:maxEvidenceCount]
	}
	for i, ev := range r.Evidence {
		r.Evidence[i] = truncate(ev, maxEvidenceLen)
	}

	if r.RunbookKeywords == nil {
		r.RunbookKeywords = []string{}
	}
	if len(r.RunbookKeywords) > maxRunbookKWCount {
		r.RunbookKeywords = r.RunbookKeywords[:maxRunbookKWCount]
	}
	for i, kw := range r.RunbookKeywords {
		r.RunbookKeywords[i] = truncate(kw, maxRunbookKWLen)
	}
}

// RCAAnalyzer is the narrow interface satisfied by RCAAgent (and test fakes).
type RCAAnalyzer interface {
	Analyze(ctx context.Context, env *alert.AlertEnvelope, triage *TriageResult) (*RCAResult, error)
}

// RCAAgent wraps an Eino ReAct agent for root cause analysis.
type RCAAgent struct {
	agent *react.Agent
	log   *zap.Logger
}

// NewRCAAgent builds an RCAAgent backed by the given ToolCallingChatModel.
func NewRCAAgent(ctx context.Context, m model.ToolCallingChatModel, log *zap.Logger) (*RCAAgent, error) {
	a, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: m,
		MessageModifier:  react.NewPersonaModifier(tenantguard.TrustedBoundarySystemPrompt + "\n\n" + rcaSystemPrompt), //nolint:staticcheck
		MaxStep:          10,
		GraphName:        "PaladinRCAAgent",
	})
	if err != nil {
		return nil, fmt.Errorf("rca agent: %w", err)
	}
	return &RCAAgent{agent: a, log: log}, nil
}

// Analyze runs the RCA agent on the given alert + triage context.
// On non-JSON model output it degrades gracefully, flagging result.Degraded=true.
func (r *RCAAgent) Analyze(ctx context.Context, env *alert.AlertEnvelope, triage *TriageResult) (*RCAResult, error) {
	input, err := json.Marshal(map[string]any{
		"alert": map[string]any{
			"title":       env.Title,
			"severity":    string(env.Severity),
			"labels":      env.Labels,
			"annotations": env.Annotations,
			"description": tenantguard.WrapAlertContent(env.Description),
			"starts_at":   env.StartsAt,
		},
		"triage": map[string]any{
			"confirmed_severity": triage.ConfirmedSeverity,
			"summary":            triage.Summary,
			"likely_cause":       triage.LikelyCause,
			"affected_services":  triage.AffectedServices,
			"needs_human":        triage.NeedsHuman,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("rca: marshal input: %w", err)
	}

	resp, err := r.agent.Generate(ctx, []*schema.Message{schema.UserMessage(string(input))})
	if err != nil {
		return nil, fmt.Errorf("rca: generate: %w", err)
	}

	var result RCAResult
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		r.log.Warn("rca: model returned non-JSON, degrading",
			zap.String("fingerprint", env.Fingerprint),
			zap.String("content_prefix", truncate(resp.Content, 200)),
			zap.Error(err),
		)
		// Degrade: populate from triage context as best-effort.
		hypothesis := triage.LikelyCause
		if hypothesis == "" {
			hypothesis = resp.Content
		}
		result = RCAResult{
			RootCauseHypothesis: hypothesis,
			Evidence:            []string{},
			Confidence:          "LOW",
			RecommendedFix:      triage.RecommendedAction,
			RunbookKeywords:     []string{},
			Degraded:            true,
		}
		result.validate() // clamp lengths even in degraded path
		return &result, nil
	}

	result.validate()

	r.log.Info("rca complete",
		zap.String("fingerprint", env.Fingerprint),
		zap.String("confidence", result.Confidence),
		zap.Bool("degraded", result.Degraded),
	)
	return &result, nil
}
