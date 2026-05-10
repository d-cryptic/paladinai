package config

import (
	"fmt"

	base "github.com/paladinai/paladinai/internal/config"
)

// Config is the full configuration for paladin-edge.
type Config struct {
	Base   base.Base
	Server base.Server
	LLM    base.LLM
	// RateLimitRPS is the default requests-per-second per tenant for the token-bucket limiter.
	// Per-tenant overrides are stored in Valkey.
	RateLimitRPS int
	// JWTSecret is the HS256 signing key for verifying agent JWTs (dev only).
	// In production, validation is delegated to paladin-auth via gRPC.
	JWTSecret string
}

func Load() (Config, error) {
	b, err := base.LoadBase()
	if err != nil {
		return Config{}, fmt.Errorf("base config: %w", err)
	}

	srv, err := base.LoadServer("PALADIN_EDGE_PORT", 9002)
	if err != nil {
		return Config{}, fmt.Errorf("server config: %w", err)
	}

	llm, err := base.LoadLLM()
	if err != nil {
		// LLM config is optional at startup — paladin-edge proxies requests but
		// does not call LLMs directly in this version.
		llm = base.LLM{}
	}

	return Config{
		Base:         b,
		Server:       srv,
		LLM:          llm,
		RateLimitRPS: 60,
	}, nil
}
