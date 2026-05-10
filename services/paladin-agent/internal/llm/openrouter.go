// Package llm provides LLM client construction for paladin-agent.
// OpenRouter exposes an OpenAI-compatible API, so we use the Eino OpenAI extension
// pointed at https://openrouter.ai/api/v1 with an OpenRouter API key.
package llm

import (
	"context"
	"fmt"
	"strings"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

// Tier labels the model quality/cost trade-off.
type Tier string

const (
	// TierA — fast, cheap (qwen/qwen3-1.7b). Use for classification, routing.
	TierA Tier = "A"
	// TierB — balanced (qwen/qwen3-8b). Default for triage + RCA.
	TierB Tier = "B"
	// TierC — powerful (deepseek/deepseek-v3). Use for complex reasoning.
	TierC Tier = "C"
)

// Secret wraps a string so it prints as "[REDACTED]" in logs and JSON.
type Secret string

func (s Secret) String() string                   { return "[REDACTED]" }
func (s Secret) MarshalJSON() ([]byte, error)      { return []byte(`"[REDACTED]"`), nil }
func (s Secret) Reveal() string                    { return string(s) }

// Config holds OpenRouter credentials and model names per tier.
type Config struct {
	// BaseURL must be HTTPS. Providing http:// will return an error from New.
	BaseURL    string
	APIKey     Secret
	ModelTierA string
	ModelTierB string
	ModelTierC string
}

// Client wraps per-tier Eino chat models backed by OpenRouter.
type Client struct {
	models map[Tier]model.ToolCallingChatModel
}

// New creates an LLM Client with one model per tier.
// Returns an error if the base URL is not HTTPS or any model fails to initialise.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if !strings.HasPrefix(cfg.BaseURL, "https://") {
		return nil, fmt.Errorf("llm: BaseURL must use HTTPS, got %q", cfg.BaseURL)
	}

	tiers := map[Tier]string{
		TierA: cfg.ModelTierA,
		TierB: cfg.ModelTierB,
		TierC: cfg.ModelTierC,
	}

	models := make(map[Tier]model.ToolCallingChatModel, len(tiers))
	for tier, modelName := range tiers {
		m, err := einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
			BaseURL: cfg.BaseURL,
			APIKey:  cfg.APIKey.Reveal(),
			Model:   modelName,
		})
		if err != nil {
			return nil, fmt.Errorf("llm: init tier %s model %q: %w", tier, modelName, err)
		}
		models[tier] = m
	}

	return &Client{models: models}, nil
}

// Model returns the ToolCallingChatModel for the given tier.
// Returns an error for unknown tiers — callers should Term the message, not crash.
func (c *Client) Model(tier Tier) (model.ToolCallingChatModel, error) {
	m, ok := c.models[tier]
	if !ok {
		return nil, fmt.Errorf("llm: unknown tier %q", tier)
	}
	return m, nil
}
