// Package config loads paladin-comms configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/paladinai/paladinai/internal/config"
)

// Comms holds paladin-comms-specific configuration layered on top of Base.
type Comms struct {
	config.Base

	// SlackWebhookURL is the destination for Slack notifications.
	// Empty disables the Slack notifier.
	SlackWebhookURL string

	// PagerDutyKey is the PagerDuty Events API v2 routing key.
	// Empty disables the PagerDuty notifier.
	PagerDutyKey string

	// NATSConsumerName is the durable consumer name for the comms agent.
	NATSConsumerName string

	// Workers is the number of concurrent notification dispatch workers.
	Workers int

	// MinSeverity is the minimum severity level to dispatch notifications.
	// Alerts below this level are silently acked. Default: P2.
	MinSeverity string
}

// Load reads Comms config from environment, applying sane defaults.
func Load() (*Comms, error) {
	base, err := config.LoadBase()
	if err != nil {
		return nil, fmt.Errorf("comms config load base: %w", err)
	}

	return &Comms{
		Base:             base,
		SlackWebhookURL:  os.Getenv("SLACK_WEBHOOK_URL"),
		PagerDutyKey:     os.Getenv("PAGERDUTY_ROUTING_KEY"),
		NATSConsumerName: envStr("COMMS_CONSUMER_NAME", "paladin-comms"),
		Workers:          envInt("COMMS_WORKERS", 4),
		MinSeverity:      envStr("COMMS_MIN_SEVERITY", "P2"),
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
