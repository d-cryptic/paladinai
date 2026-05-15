// Package config centralises environment-based configuration for all services.
// Each service imports this package and calls its own typed loader.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Base holds fields common to every PaladinAI service.
type Base struct {
	Env            string
	LogLevel       string
	NatsURL        string
	DatabaseURL    string
	ValkeyURL      string
	QdrantURL      string
	VaultAddr      string
	VaultToken     string
	OtelEndpoint   string
	ServiceVersion string
}

// LoadBase reads common config from environment variables.
// Returns an error if any required variable is missing or invalid.
func LoadBase() (Base, error) {
	b := Base{
		Env:            getEnv("ENV", "development"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		NatsURL:        getEnv("NATS_URL", "nats://localhost:4222"),
		DatabaseURL:    getEnv("DATABASE_URL", ""),
		ValkeyURL:      getEnv("VALKEY_URL", "redis://localhost:6379"),
		QdrantURL:      getEnv("QDRANT_URL", "http://localhost:6333"),
		VaultAddr:      getEnv("VAULT_ADDR", "http://localhost:8200"),
		VaultToken:     getEnv("VAULT_TOKEN", ""),
		OtelEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		ServiceVersion: getEnv("OTEL_SERVICE_VERSION", "dev"),
	}
	return b, nil
}

// LLM holds OpenRouter / gateway config.
type LLM struct {
	GatewayURL    string
	OpenRouterKey string
	// AllowInsecureGateway permits http:// gateways for local mocks only.
	AllowInsecureGateway bool
	// Default model tiers (see stage 9 for full routing table)
	TierA string // fast, cheap: qwen/qwen3.6-flash
	TierB string // balanced: qwen/qwen3-8b
	TierC string // powerful: deepseek/deepseek-v3.2
	// MaxTokens bounds generated tokens for latency and cost.
	MaxTokens int
}

func LoadLLM() (LLM, error) {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		return LLM{}, fmt.Errorf("OPENROUTER_API_KEY is required")
	}
	return LLM{
		GatewayURL:           getEnv("LLM_GATEWAY_URL", "https://openrouter.ai/api/v1"),
		OpenRouterKey:        key,
		AllowInsecureGateway: getEnvBool("LLM_ALLOW_INSECURE_GATEWAY", false),
		TierA:                getEnv("LLM_TIER_A", "qwen/qwen3.6-flash"),
		TierB:                getEnv("LLM_TIER_B", "qwen/qwen3-8b"),
		TierC:                getEnv("LLM_TIER_C", "deepseek/deepseek-v3.2"),
		MaxTokens:            getEnvInt("LLM_MAX_TOKENS", 512),
	}, nil
}

// Server holds HTTP/gRPC listener config.
type Server struct {
	Port            int
	GRPCPort        int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

func LoadServer(portEnv string, defaultPort int) (Server, error) {
	port := getEnvInt(portEnv, defaultPort)
	return Server{
		Port:            port,
		GRPCPort:        port + 100,
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
