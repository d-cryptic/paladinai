// Package projectconfig handles the project-level paladin.yaml configuration
// file written during `paladin init`. This file is committed to the project
// repository (unlike ~/.paladin/config.yaml which is user-level).
//
// Secrets are never written here — they are stored in the OS keychain or passed
// via PALADIN_{INTEGRATION}_{KEY} environment variables in CI.
package projectconfig

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level structure of a paladin.yaml project file.
type Config struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

// Metadata holds tenant-level settings.
type Metadata struct {
	Tenant string `yaml:"tenant"`
	Tier   string `yaml:"tier"` // pool | bridge | silo
}

// Spec holds runtime configuration.
type Spec struct {
	Region       string        `yaml:"region"`
	Integrations []Integration `yaml:"integrations,omitempty"`
	Routing      Routing       `yaml:"routing,omitempty"`
}

// Integration describes one enabled or disabled integration.
type Integration struct {
	Name    string         `yaml:"name"`
	Version string         `yaml:"version"`
	Enabled bool           `yaml:"enabled"`
	Config  map[string]any `yaml:"config,omitempty"`
}

// Routing maps severity levels to LLM tiers and approval policies.
type Routing struct {
	P1 SeverityRoute `yaml:"p1"`
	P2 SeverityRoute `yaml:"p2"`
	P3 SeverityRoute `yaml:"p3"`
	P4 SeverityRoute `yaml:"p4"`
}

// SeverityRoute is the per-severity routing policy.
type SeverityRoute struct {
	LLMTier        string `yaml:"llm_tier"`
	ApprovalPolicy string `yaml:"approval_policy"`
}

// Default returns a sensible default Config for a new project.
func Default(tenant, tier, region string) Config {
	if tier == "" {
		tier = "pool"
	}
	if region == "" {
		region = "us-west-2"
	}
	return Config{
		APIVersion: "paladin.io/v2",
		Kind:       "Config",
		Metadata:   Metadata{Tenant: tenant, Tier: tier},
		Spec: Spec{
			Region: region,
			Integrations: []Integration{
				{Name: "prometheus", Version: "1.4.0", Enabled: false,
					Config: map[string]any{"url": "http://prometheus.monitoring.svc.cluster.local:9090"}},
				{Name: "grafana", Version: "1.2.0", Enabled: false,
					Config: map[string]any{"url": "http://grafana.monitoring.svc.cluster.local:3000"}},
				{Name: "alertmanager", Version: "1.1.0", Enabled: false,
					Config: map[string]any{"url": "http://alertmanager.monitoring.svc.cluster.local:9093"}},
				{Name: "pagerduty", Version: "2.0.0", Enabled: false,
					Config: map[string]any{"service_region": "us"}},
				{Name: "slack", Version: "1.3.0", Enabled: false,
					Config: map[string]any{"default_channel": "#paladin-alerts"}},
			},
			Routing: Routing{
				P1: SeverityRoute{LLMTier: "C", ApprovalPolicy: "auto"},
				P2: SeverityRoute{LLMTier: "B", ApprovalPolicy: "auto"},
				P3: SeverityRoute{LLMTier: "A", ApprovalPolicy: "auto"},
				P4: SeverityRoute{LLMTier: "A", ApprovalPolicy: "skip"},
			},
		},
	}
}

// WriteFile marshals cfg to YAML and writes it to path.
func WriteFile(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal paladin.yaml: %w", err)
	}
	header := []byte("# paladin.yaml — PaladinAI project configuration\n" +
		"# Commit this file to your git repository.\n" +
		"# Secrets (API keys, tokens) are never stored here — use the OS keychain or\n" +
		"# PALADIN_{INTEGRATION}_{KEY} environment variables in CI.\n\n")
	return os.WriteFile(path, append(header, data...), 0o644)
}

// ReadFile reads and parses a paladin.yaml file.
func ReadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read paladin.yaml: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse paladin.yaml: %w", err)
	}
	return cfg, nil
}
