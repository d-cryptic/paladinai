package integrationpkg

import (
	"os"
	"path/filepath"
	"testing"
)

// tempIntegDir creates a temporary directory tree simulating the integrations/ layout.
func tempIntegDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

func writeYAML(t *testing.T, dir, subdir, content string) {
	t.Helper()
	d := filepath.Join(dir, subdir)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(d, "integration.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// ─── loadFile error paths ─────────────────────────────────────────────────────

func TestLoadAll_InvalidYAML_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := tempIntegDir(t)
	writeYAML(t, dir, "bad-yaml", "{{{{ not valid yaml }}}}")

	_, err := LoadAll(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadAll_EmptyName_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := tempIntegDir(t)
	writeYAML(t, dir, "no-name", `
version: "1.0.0"
description: Missing name field
`)

	_, err := LoadAll(dir)
	if err == nil {
		t.Fatal("expected error for integration with empty name, got nil")
	}
}

func TestLoadAll_SkipsNonDirEntries(t *testing.T) {
	t.Parallel()
	dir := tempIntegDir(t)

	// Write a valid integration
	writeYAML(t, dir, "valid", `
name: valid
version: "1.0.0"
description: Valid integration
receiver:
  type: webhook
  path: /valid
auth:
  type: none
`)
	// Create a plain file (not a directory) at the top level — should be skipped.
	if err := os.WriteFile(filepath.Join(dir, "stray-file.yaml"), []byte("name: stray"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	all, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("got %d integrations, want 1 (stray file should be skipped)", len(all))
	}
}

func TestLoadAll_SkipsDirWithNoYAML(t *testing.T) {
	t.Parallel()
	dir := tempIntegDir(t)

	// Subdir with no integration.yaml
	if err := os.MkdirAll(filepath.Join(dir, "no-yaml"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	all, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("got %d integrations, want 0 (dir without yaml should be skipped)", len(all))
	}
}

func TestLoadByName_InvalidYAML_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := tempIntegDir(t)
	writeYAML(t, dir, "bad-yaml", "{{{{ not valid yaml }}}}")

	_, err := LoadByName(dir, "bad-yaml")
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadByName_EmptyName_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := tempIntegDir(t)
	writeYAML(t, dir, "no-name", "description: no name\n")

	_, err := LoadByName(dir, "no-name")
	if err == nil {
		t.Fatal("expected error for integration with empty name, got nil")
	}
}

func TestLoadAll_ValidIntegration_ParsedCorrectly(t *testing.T) {
	t.Parallel()
	dir := tempIntegDir(t)
	writeYAML(t, dir, "myint", `
name: myint
version: "2.0.0"
description: My integration
receiver:
  type: webhook
  path: /myint
  hmac_header: X-Signature
auth:
  type: api_key
  fields:
    - name: api_key
      secret: true
tools:
  - mcp-myint
config_schema:
  region:
    type: string
    required: true
docs_url: https://example.com
`)

	all, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("got %d integrations, want 1", len(all))
	}
	integ := all[0]
	if integ.Name != "myint" {
		t.Errorf("Name = %q, want myint", integ.Name)
	}
	if integ.Receiver.HMACHeader != "X-Signature" {
		t.Errorf("HMACHeader = %q, want X-Signature", integ.Receiver.HMACHeader)
	}
	if integ.DocsURL != "https://example.com" {
		t.Errorf("DocsURL = %q, want https://example.com", integ.DocsURL)
	}
	if len(integ.Tools) != 1 {
		t.Errorf("Tools len = %d, want 1", len(integ.Tools))
	}
	if _, ok := integ.ConfigSchema["region"]; !ok {
		t.Errorf("ConfigSchema missing 'region'")
	}
}
