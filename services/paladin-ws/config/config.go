// Package config loads paladin-ws configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	base "github.com/paladinai/paladinai/internal/config"
	inats "github.com/paladinai/paladinai/internal/nats"
)

// Config is the full configuration for paladin-ws.
type Config struct {
	Base         base.Base
	Server       base.Server
	AdminPort    int
	NATSSubjects []string
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
	adminPort, err := loadAdminPort()
	if err != nil {
		return Config{}, err
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
		AdminPort:    adminPort,
		NATSSubjects: loadNATSSubjects(),
		NATSConsumer: getEnvOr("NATS_CONSUMER_NAME", "paladin-ws"),
		JWTSecret:    []byte(secret),
	}, nil
}

func loadNATSSubjects() []string {
	if value := os.Getenv("NATS_ALERTS_SUBJECTS"); value != "" {
		return splitSubjects(value)
	}
	if value := os.Getenv("NATS_ALERTS_SUBJECT"); value != "" {
		return []string{value}
	}
	return []string{inats.SubjectAlertsTriaged, inats.SubjectAlertsAnalyzed}
}

func splitSubjects(value string) []string {
	parts := strings.Split(value, ",")
	subjects := make([]string, 0, len(parts))
	for _, part := range parts {
		subject := strings.TrimSpace(part)
		if subject != "" {
			subjects = append(subjects, subject)
		}
	}
	return subjects
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadAdminPort() (int, error) {
	value := os.Getenv("PALADIN_WS_ADMIN_PORT")
	if value == "" {
		return 0, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("PALADIN_WS_ADMIN_PORT must be an integer: %w", err)
	}
	if port < 0 || port > 65535 {
		return 0, fmt.Errorf("PALADIN_WS_ADMIN_PORT must be between 0 and 65535")
	}
	return port, nil
}
