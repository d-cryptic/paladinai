// Package config loads paladin-hub configuration from environment variables.
package config

import (
	"fmt"
	"os"

	"github.com/paladinai/paladinai/internal/config"
)

// Hub holds paladin-hub-specific config.
type Hub struct {
	config.Base
	Server config.Server
	// DatabaseURL is the PostgreSQL DSN (e.g. postgres://user:pass@host/db).
	// Empty string means use MemStore (development / test mode).
	DatabaseURL string
}

// Load reads Hub config from environment variables.
func Load() (*Hub, error) {
	base, err := config.LoadBase()
	if err != nil {
		return nil, fmt.Errorf("hub config load base: %w", err)
	}

	srv, err := config.LoadServer("HUB_PORT", 8082)
	if err != nil {
		return nil, fmt.Errorf("hub config load server: %w", err)
	}

	return &Hub{Base: base, Server: srv, DatabaseURL: os.Getenv("DATABASE_URL")}, nil
}
