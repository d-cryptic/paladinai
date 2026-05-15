// Package config loads paladin-memory configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/paladinai/paladinai/internal/config"
)

// Config holds paladin-memory service settings.
type Config struct {
	config.Base

	// Env is the deployment environment. Development/test may use local fakes.
	Env string
	// GRPCAddr is the gRPC listener address (e.g. ":9010").
	GRPCAddr string
	// HTTPAddr is the health/metrics listener address (e.g. ":9011").
	HTTPAddr string
	// DatabaseURL is the PostgreSQL DSN for episodic memory storage. Required.
	DatabaseURL string
	// ValkeyURL is the Redis/Valkey URL for working memory storage.
	ValkeyURL string
	// WorkingTTL is the default TTL applied to working-memory entries (seconds).
	WorkingTTL int
	// QdrantURL is the base URL of the Qdrant REST API for procedural memory.
	// Optional; when empty, procedural search is disabled.
	QdrantURL string
	// QdrantAPIKey authenticates against Qdrant when set.
	QdrantAPIKey string
	// FalkorDBURL is the Redis-protocol URL for FalkorDB (topology graph). Optional.
	FalkorDBURL string
	// AllowStubEmbedder permits deterministic fake embeddings outside development/test.
	AllowStubEmbedder bool
}

// Load reads Config from environment variables.
func Load() (*Config, error) {
	base, err := config.LoadBase()
	if err != nil {
		return nil, fmt.Errorf("memory: config: base: %w", err)
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("memory: config: DATABASE_URL is required")
	}
	allowStubEmbedder, err := getEnvBool("PALADIN_MEMORY_ALLOW_STUB_EMBEDDER", false)
	if err != nil {
		return nil, fmt.Errorf("memory: config: PALADIN_MEMORY_ALLOW_STUB_EMBEDDER: %w", err)
	}
	c := &Config{
		Base:              base,
		Env:               getEnv("ENV", "development"),
		GRPCAddr:          getEnv("GRPC_ADDR", ":9010"),
		HTTPAddr:          getEnv("MEMORY_HTTP_ADDR", ":9011"),
		DatabaseURL:       dbURL,
		ValkeyURL:         getEnv("VALKEY_URL", "redis://localhost:6379"),
		WorkingTTL:        getEnvInt("WORKING_MEMORY_TTL_SECONDS", 1800),
		QdrantURL:         os.Getenv("QDRANT_URL"),
		QdrantAPIKey:      os.Getenv("QDRANT_API_KEY"),
		FalkorDBURL:       os.Getenv("FALKORDB_URL"),
		AllowStubEmbedder: allowStubEmbedder,
	}
	if c.WorkingTTL <= 0 {
		return nil, fmt.Errorf("memory: config: WORKING_MEMORY_TTL_SECONDS must be > 0")
	}
	return c, nil
}

func getEnvBool(key string, fallback bool) (bool, error) {
	if v := os.Getenv(key); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return false, err
		}
		return parsed, nil
	}
	return fallback, nil
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
