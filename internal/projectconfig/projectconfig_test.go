package projectconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault_FieldsPopulated(t *testing.T) {
	cfg := Default("acme-corp", "bridge", "eu-west-1")
	if cfg.APIVersion != "paladin.io/v2" {
		t.Errorf("APIVersion = %q, want paladin.io/v2", cfg.APIVersion)
	}
	if cfg.Kind != "Config" {
		t.Errorf("Kind = %q, want Config", cfg.Kind)
	}
	if cfg.Metadata.Tenant != "acme-corp" {
		t.Errorf("Tenant = %q, want acme-corp", cfg.Metadata.Tenant)
	}
	if cfg.Metadata.Tier != "bridge" {
		t.Errorf("Tier = %q, want bridge", cfg.Metadata.Tier)
	}
	if cfg.Spec.Region != "eu-west-1" {
		t.Errorf("Region = %q, want eu-west-1", cfg.Spec.Region)
	}
}

func TestDefault_EmptyTierAndRegion_UseFallbacks(t *testing.T) {
	cfg := Default("tenant-x", "", "")
	if cfg.Metadata.Tier != "pool" {
		t.Errorf("Tier = %q, want pool (default)", cfg.Metadata.Tier)
	}
	if cfg.Spec.Region != "us-west-2" {
		t.Errorf("Region = %q, want us-west-2 (default)", cfg.Spec.Region)
	}
}

func TestDefault_HasExpectedIntegrations(t *testing.T) {
	cfg := Default("t", "", "")
	if len(cfg.Spec.Integrations) == 0 {
		t.Fatal("expected at least one integration in default config")
	}
	names := make(map[string]bool)
	for _, i := range cfg.Spec.Integrations {
		names[i.Name] = true
	}
	for _, required := range []string{"prometheus", "grafana", "alertmanager"} {
		if !names[required] {
			t.Errorf("missing required integration %q in default config", required)
		}
	}
}

func TestDefault_RoutingAllSeverities(t *testing.T) {
	cfg := Default("t", "", "")
	if cfg.Spec.Routing.P1.LLMTier != "C" {
		t.Errorf("P1 LLMTier = %q, want C", cfg.Spec.Routing.P1.LLMTier)
	}
	if cfg.Spec.Routing.P4.ApprovalPolicy != "skip" {
		t.Errorf("P4 ApprovalPolicy = %q, want skip", cfg.Spec.Routing.P4.ApprovalPolicy)
	}
}

func TestWriteFileAndReadFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paladin.yaml")

	cfg := Default("round-trip-tenant", "silo", "ap-southeast-1")
	if err := WriteFile(path, cfg); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	read, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if read.Metadata.Tenant != cfg.Metadata.Tenant {
		t.Errorf("Tenant round-trip = %q, want %q", read.Metadata.Tenant, cfg.Metadata.Tenant)
	}
	if read.Metadata.Tier != cfg.Metadata.Tier {
		t.Errorf("Tier round-trip = %q, want %q", read.Metadata.Tier, cfg.Metadata.Tier)
	}
	if read.Spec.Region != cfg.Spec.Region {
		t.Errorf("Region round-trip = %q, want %q", read.Spec.Region, cfg.Spec.Region)
	}
}

func TestWriteFile_ContainsHeaderComment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paladin.yaml")
	if err := WriteFile(path, Default("t", "", "")); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if len(data) == 0 || data[0] != '#' {
		t.Error("expected file to start with a comment header")
	}
}

func TestReadFile_NonExistent_ReturnsError(t *testing.T) {
	_, err := ReadFile("/nonexistent/paladin.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestReadFile_InvalidYAML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	_ = os.WriteFile(path, []byte(": invalid: yaml: {"), 0o644)
	_, err := ReadFile(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
