// Package config loads paladin-agent configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/paladinai/paladinai/internal/config"
)

// Agent holds paladin-agent-specific config layered on top of Base.
type Agent struct {
	config.Base
	LLM config.LLM

	// AgentWorkers is the number of concurrent alert processing workers.
	AgentWorkers int

	// TriageTimeout is the max time allowed for a single triage operation (Tier B model).
	TriageTimeout time.Duration

	// RCATimeout is the max time allowed for a single RCA operation (Tier C model — slower).
	// Defaults to 2× TriageTimeout.
	RCATimeout time.Duration

	// NATSConsumerName is the durable consumer name for this agent instance.
	NATSConsumerName string
}

// Load reads Agent config from environment, applying sane defaults.
func Load() (*Agent, error) {
	base, err := config.LoadBase()
	if err != nil {
		return nil, fmt.Errorf("agent config load base: %w", err)
	}

	llm, err := config.LoadLLM()
	if err != nil {
		return nil, fmt.Errorf("agent config load llm: %w", err)
	}

	triageTimeout := 30 * time.Second
	return &Agent{
		Base:             base,
		LLM:              llm,
		AgentWorkers:     envInt("AGENT_WORKERS", 4),
		TriageTimeout:    triageTimeout,
		RCATimeout:       2 * triageTimeout,
		NATSConsumerName: envStr("AGENT_CONSUMER_NAME", "paladin-agent-runtime"),
	}, nil
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
