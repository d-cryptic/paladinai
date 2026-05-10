package config

import (
	"fmt"
	"os"
	"strings"

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
	// HubURL is the base URL of the paladin-hub MCP registry service.
	// e.g. "http://paladin-hub:8082". When empty, the /api/v1/mcp routes are
	// registered but return 502 immediately (safe for local dev without hub running).
	HubURL string
	// AllowedOrigins is the list of CORS origins for the dashboard SPA.
	// Defaults to localhost dev ports; override via CORS_ALLOWED_ORIGINS (comma-separated).
	AllowedOrigins []string
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

	origins := parseCORSOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))

	return Config{
		Base:           b,
		Server:         srv,
		LLM:            llm,
		RateLimitRPS:   60,
		JWTSecret:      os.Getenv("JWT_SECRET"),
		HubURL:         os.Getenv("HUB_URL"),
		AllowedOrigins: origins,
	}, nil
}

// parseCORSOrigins splits a comma-separated list of origins.
// When raw is empty it returns the safe local-dev defaults.
func parseCORSOrigins(raw string) []string {
	if raw == "" {
		return []string{"http://localhost:3000", "http://localhost:3001"}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return []string{"http://localhost:3000", "http://localhost:3001"}
	}
	return out
}
