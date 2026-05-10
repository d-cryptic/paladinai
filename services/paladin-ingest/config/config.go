package config

import (
	"fmt"
	"os"

	base "github.com/paladinai/paladinai/internal/config"
)

// Config is the full configuration for paladin-ingest.
type Config struct {
	Base   base.Base
	Server base.Server
	// HMAC signing secret for webhook signature verification.
	// Each integration tenant gets a per-tenant secret stored in Vault.
	// This is the fallback dev secret.
	WebhookSecret string
}

func Load() (Config, error) {
	b, err := base.LoadBase()
	if err != nil {
		return Config{}, fmt.Errorf("base config: %w", err)
	}

	srv, err := base.LoadServer("PALADIN_INGEST_PORT", 9001)
	if err != nil {
		return Config{}, fmt.Errorf("server config: %w", err)
	}

	return Config{
		Base:          b,
		Server:        srv,
		WebhookSecret: os.Getenv("WEBHOOK_SECRET"),
	}, nil
}
