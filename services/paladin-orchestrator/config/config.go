// Package config loads paladin-orchestrator configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/paladinai/paladinai/internal/config"
)

// Orchestrator holds paladin-orchestrator-specific config.
type Orchestrator struct {
	config.Base

	// ConsumerName is the durable JetStream consumer name for the raw-alert stream.
	ConsumerName string

	// Workers is the number of concurrent alert processing goroutines.
	Workers int
}

// Load reads Orchestrator config from environment variables.
func Load() (*Orchestrator, error) {
	base, err := config.LoadBase()
	if err != nil {
		return nil, fmt.Errorf("orchestrator config: %w", err)
	}

	return &Orchestrator{
		Base:         base,
		ConsumerName: envStr("ORCHESTRATOR_CONSUMER_NAME", "paladin-orchestrator"),
		Workers:      envInt("ORCHESTRATOR_WORKERS", 8),
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
