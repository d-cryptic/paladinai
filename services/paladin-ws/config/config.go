// Package config loads paladin-ws configuration from environment variables.
package config

import (
	"fmt"
	"os"

	base "github.com/paladinai/paladinai/internal/config"
)

// Config is the full configuration for paladin-ws.
type Config struct {
	Base         base.Base
	Server       base.Server
	NATSSubject  string
	NATSConsumer string
	JWTSecret    []byte
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	b, err := base.LoadBase()
	if err != nil {
		return Config{}, fmt.Errorf("base config: %w", err)
	}
	srv, err := base.LoadServer("PALADIN_WS_PORT", 9007)
	if err != nil {
		return Config{}, fmt.Errorf("server config: %w", err)
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
	if len(secret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 bytes (got %d)", len(secret))
	}

	return Config{
		Base:         b,
		Server:       srv,
		NATSSubject:  getEnvOr("NATS_ALERTS_SUBJECT", "paladin.alerts.processed"),
		NATSConsumer: getEnvOr("NATS_CONSUMER_NAME", "paladin-ws"),
		JWTSecret:    []byte(secret),
	}, nil
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
