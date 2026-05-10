package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &PaladinConfig{
		APIEndpoint:   "http://localhost:8080",
		AuthEndpoint:  "http://localhost:9003",
		DefaultTenant: "test-tenant",
		OutputFormat:  "table",
		Token:         "tok123",
	}

	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	fi, err := os.Stat(filepath.Join(dir, ".paladin", "config.yaml"))
	if err != nil {
		t.Fatalf("config file missing: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("config file permissions = %o, want 0600", fi.Mode().Perm())
	}

	loaded, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if loaded.APIEndpoint != cfg.APIEndpoint {
		t.Errorf("APIEndpoint = %q, want %q", loaded.APIEndpoint, cfg.APIEndpoint)
	}
	if loaded.Token != cfg.Token {
		t.Errorf("Token = %q, want %q", loaded.Token, cfg.Token)
	}
	if loaded.DefaultTenant != cfg.DefaultTenant {
		t.Errorf("DefaultTenant = %q, want %q", loaded.DefaultTenant, cfg.DefaultTenant)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig on missing file: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config for missing file")
	}
	if cfg.OutputFormat != "table" {
		t.Errorf("default OutputFormat = %q, want %q", cfg.OutputFormat, "table")
	}
}

func TestLoadConfig_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if err := os.MkdirAll(filepath.Join(dir, ".paladin"), 0o700); err != nil {
		t.Fatal(err)
	}
	// yaml.v3 errors when a sequence cannot unmarshal into a struct.
	if err := os.WriteFile(filepath.Join(dir, ".paladin", "config.yaml"), []byte("- a\n- b\n- c\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error for corrupt config file")
	}
}

func TestConfigPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	path := configPath()
	if !filepath.IsAbs(path) {
		t.Errorf("configPath() = %q, want absolute path", path)
	}
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("configPath() base = %q, want config.yaml", filepath.Base(path))
	}
}

func TestSaveConfig_EmptyTokenIsOmitted(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Write config without a token (simulates env-sourced token not being written)
	cfg := &PaladinConfig{
		APIEndpoint:  "http://localhost:8080",
		OutputFormat: "table",
		Token:        "", // empty — must not appear in file
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(configPath())
	if err != nil {
		t.Fatal(err)
	}
	if contains(string(data), "token:") {
		t.Errorf("empty token should be omitted from config file, got:\n%s", data)
	}

	loaded, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if loaded.Token != "" {
		t.Errorf("loaded Token = %q, want empty", loaded.Token)
	}
}

func TestSaveConfig_RoundTrip_NoToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &PaladinConfig{
		APIEndpoint:   "https://api.example.com",
		AuthEndpoint:  "https://auth.example.com",
		DefaultTenant: "acme-corp",
		OutputFormat:  "json",
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.APIEndpoint != cfg.APIEndpoint || loaded.AuthEndpoint != cfg.AuthEndpoint ||
		loaded.DefaultTenant != cfg.DefaultTenant || loaded.OutputFormat != cfg.OutputFormat {
		t.Errorf("round-trip mismatch: got %+v, want %+v", loaded, cfg)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
