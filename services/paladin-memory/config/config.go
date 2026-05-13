// Package config loads paladin-memory configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds paladin-memory service settings.
type Config struct {
	// GRPCAddr is the gRPC listener address (e.g. ":9010").
	GRPCAddr string
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
}

// Load reads Config from environment variables.
func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("memory: config: DATABASE_URL is required")
	}
	c := &Config{
		GRPCAddr:     getEnv("GRPC_ADDR", ":9010"),
		DatabaseURL:  dbURL,
		ValkeyURL:    getEnv("VALKEY_URL", "redis://localhost:6379"),
		WorkingTTL:   getEnvInt("WORKING_MEMORY_TTL_SECONDS", 1800),
		QdrantURL:    os.Getenv("QDRANT_URL"),
		QdrantAPIKey: os.Getenv("QDRANT_API_KEY"),
		FalkorDBURL:  os.Getenv("FALKORDB_URL"),
	}
	if c.WorkingTTL <= 0 {
		return nil, fmt.Errorf("memory: config: WORKING_MEMORY_TTL_SECONDS must be > 0")
	}
	return c, nil
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
