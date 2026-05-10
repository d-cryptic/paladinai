package config

import (
	"fmt"
	"os"
	"time"

	"github.com/paladinai/paladinai/internal/config"
)

type Config struct {
	Base config.Base

	// JWTSecret is the HS256 signing key — must be ≥32 bytes.
	JWTSecret string

	// AdminSecret is the pre-shared secret that gates tenant management
	// and token issuance endpoints (X-Admin-Secret header).
	AdminSecret string

	// TokenTTL is how long issued access tokens live.
	TokenTTL time.Duration

	Server struct {
		Port            int
		ReadTimeout     time.Duration
		WriteTimeout    time.Duration
		IdleTimeout     time.Duration
		ShutdownTimeout time.Duration
	}
}

func Load() (*Config, error) {
	base, err := config.LoadBase()
	if err != nil {
		return nil, fmt.Errorf("base config: %w", err)
	}

	c := &Config{Base: base}

	c.JWTSecret = os.Getenv("JWT_SECRET")
	if c.JWTSecret == "" {
		// In production (ENV=production) a real secret is mandatory.
		if base.Env == "production" {
			return nil, fmt.Errorf("JWT_SECRET is required in production")
		}
		c.JWTSecret = "dev-secret-change-in-production!!" // ≥32 bytes
	}
	if len(c.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 bytes (got %d)", len(c.JWTSecret))
	}

	c.AdminSecret = os.Getenv("ADMIN_SECRET")

	if ttl := os.Getenv("TOKEN_TTL"); ttl != "" {
		d, err := time.ParseDuration(ttl)
		if err != nil {
			return nil, fmt.Errorf("TOKEN_TTL: %w", err)
		}
		c.TokenTTL = d
	} else {
		c.TokenTTL = 24 * time.Hour
	}

	c.Server.Port = 9003
	c.Server.ReadTimeout = 5 * time.Second
	c.Server.WriteTimeout = 10 * time.Second
	c.Server.IdleTimeout = 60 * time.Second
	c.Server.ShutdownTimeout = 10 * time.Second

	return c, nil
}
