// Package llm provides LLM client construction for paladin-agent.
// OpenRouter exposes an OpenAI-compatible API, so we use the Eino OpenAI extension
// pointed at https://openrouter.ai/api/v1 with an OpenRouter API key.
package llm

import (
	"context"

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

// Config holds OpenRouter credentials and model names per tier.
type Config struct {
	BaseURL       string // default: https://openrouter.ai/api/v1
	APIKey        string
	ModelTierA    string
	ModelTierB    string
	ModelTierC    string
}

// Client wraps per-tier Eino chat models backed by OpenRouter.
type Client struct {
	models map[Tier]model.ToolCallingChatModel
}

// New creates an LLM Client with one model per tier.
// Returns an error if any model fails to initialise (bad config, network issue at startup).
func New(ctx context.Context, cfg Config) (*Client, error) {
	tiers := map[Tier]string{
		TierA: cfg.ModelTierA,
		TierB: cfg.ModelTierB,
		TierC: cfg.ModelTierC,
	}

	models := make(map[Tier]model.ToolCallingChatModel, len(tiers))
	for tier, modelName := range tiers {
		m, err := einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
			BaseURL: cfg.BaseURL,
			APIKey:  cfg.APIKey,
			Model:   modelName,
		})
		if err != nil {
			return nil, err
		}
		models[tier] = m
	}

	return &Client{models: models}, nil
}

// Model returns the ToolCallingChatModel for the given tier.
// Panics if tier is unknown — callers must use a defined Tier constant.
func (c *Client) Model(tier Tier) model.ToolCallingChatModel {
	m, ok := c.models[tier]
	if !ok {
		panic("llm: unknown tier " + string(tier))
	}
	return m
}
