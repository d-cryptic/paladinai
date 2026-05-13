package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/alert"
	anthropicpkg "github.com/paladinai/paladinai/internal/anthropic"
	"github.com/paladinai/paladinai/internal/cache"
)

// PromptProvider returns the active system prompt for a named agent.
type PromptProvider interface {
	Get(agentName, modelID string) string
}

// AnthropicTriager is a Triager backed by Anthropic's native Messages API.
// It injects prompt cache breakpoints on every request (Stage 9), resulting
// in ~90% cost reduction on the cached prefix (system prompt + tool catalog)
// after the first request per session.
//
// Use NewAnthropicTriager when ANTHROPIC_API_KEY is configured; otherwise
// fall back to NewTriageAgent (Eino/OpenRouter path).
type AnthropicTriager struct {
	client *anthropicpkg.Client
	log    *zap.Logger
	ps     PromptProvider     // optional; nil = use built-in constant
	rag    *RAGContextBuilder // optional RAG context injection
}

// WithPromptStore attaches a hot-reloadable prompt store.
func (a *AnthropicTriager) WithPromptStore(p PromptProvider) *AnthropicTriager {
	a.ps = p
	return a
}

// WithRAG attaches a runbook retrieval pipeline for context injection.
func (a *AnthropicTriager) WithRAG(r *RAGContextBuilder) *AnthropicTriager {
	a.rag = r
	return a
}

// NewAnthropicTriager creates an AnthropicTriager backed by the given client.
func NewAnthropicTriager(client *anthropicpkg.Client, log *zap.Logger) *AnthropicTriager {
	if log == nil {
		log = zap.NewNop()
	}
	return &AnthropicTriager{client: client, log: log}
}

// Triage classifies the alert using Anthropic's native API with automatic
// prompt cache injection. Falls back to a degraded result on non-JSON output.
func (a *AnthropicTriager) Triage(ctx context.Context, env *alert.AlertEnvelope) (*TriageResult, error) {
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
		return nil, fmt.Errorf("anthropic triage: marshal alert: %w", err)
	}

	sysPrompt := triageSystemPrompt
	if a.ps != nil {
		if p := a.ps.Get("triage", a.client.Model()); p != "" {
			sysPrompt = p
		}
	}
	if a.rag != nil {
		sysPrompt += a.rag.Build(ctx, env)
	}
	req := anthropicpkg.Request{
		SystemPrompt: sysPrompt,
		Messages: cache.PlainChatMessages([][2]string{
			{cache.RoleUser, string(alertJSON)},
		}),
	}

	resp, err := a.client.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("anthropic triage: complete: %w", err)
	}

	if resp.CacheHit() {
		a.log.Debug("anthropic prompt cache hit",
			zap.String("fingerprint", env.Fingerprint),
			zap.Int("cached_tokens", resp.Usage.CacheReadInputTokens),
			zap.Int("saved_tokens", cache.TokenSavingsEstimate(resp.Usage.CacheReadInputTokens)),
		)
	}

	content := resp.TextContent()

	var result TriageResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		a.log.Warn("anthropic triage: model returned non-JSON, degrading",
			zap.String("content_prefix", truncate(content, 200)),
			zap.Error(err),
		)
		result = TriageResult{
			ConfirmedSeverity: string(env.Severity),
			Summary:           truncate(content, maxSummaryLen),
			NeedsHuman:        env.Severity == alert.SeverityP1 || env.Severity == alert.SeverityP2,
			Degraded:          true,
		}
		if !validSeverities[result.ConfirmedSeverity] {
			result.ConfirmedSeverity = "P3"
		}
		return &result, nil
	}

	if err := result.validate(); err != nil {
		a.log.Warn("anthropic triage: invalid result, degrading",
			zap.Error(err),
		)
		result.ConfirmedSeverity = string(env.Severity)
		if !validSeverities[result.ConfirmedSeverity] {
			result.ConfirmedSeverity = "P3"
		}
		result.Degraded = true
	}

	a.log.Info("anthropic triage complete",
		zap.String("fingerprint", env.Fingerprint),
		zap.String("severity", result.ConfirmedSeverity),
		zap.Bool("needs_human", result.NeedsHuman),
		zap.Bool("cache_hit", resp.CacheHit()),
	)
	return &result, nil
}
