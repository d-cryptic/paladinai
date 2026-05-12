package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	integrationpkg "github.com/paladinai/paladinai/internal/integrationpkg"
)

// ── printTenantTable ──────────────────────────────────────────────────────────

func TestPrintTenantTable_HeaderOnly(t *testing.T) {
	body := []byte(`{"data":[]}`)
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	if err := printTenantTable(body); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if !bytes.Contains(buf.Bytes(), []byte("ID")) {
		t.Error("expected header row with ID")
	}
}

func TestPrintTenantTable_WithData(t *testing.T) {
	body := []byte(`{"data":[{"id":"t1","slug":"acme","name":"Acme Corp","state":"active","created_at":"2026-01-01"}]}`)
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	_ = printTenantTable(body)
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if !bytes.Contains(buf.Bytes(), []byte("acme")) {
		t.Error("expected slug 'acme' in output")
	}
}

func TestPrintTenantTable_InvalidJSON(t *testing.T) {
	body := []byte("not json")
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	_ = printTenantTable(body)
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if !bytes.Contains(buf.Bytes(), []byte("not json")) {
		t.Error("expected raw body for invalid JSON")
	}
}

// ── authURL / adminSecret ─────────────────────────────────────────────────────

func TestAuthURL_NoFlag_ReturnsDefault(t *testing.T) {
	got := authURL(rootCmd)
	if got == "" {
		t.Error("authURL should return a non-empty default")
	}
}

func TestAdminSecret_NoFlag_ReturnsEmpty(t *testing.T) {
	got := adminSecret(rootCmd)
	if got != "" {
		t.Errorf("adminSecret without flag = %q, want empty", got)
	}
}

// ── printIncidentDetail ───────────────────────────────────────────────────────

func TestPrintIncidentDetail_AllFields(t *testing.T) {
	inc := map[string]any{
		"id":             "inc-001",
		"title":          "Payments Down",
		"severity":       "P1",
		"status":         "open",
		"correlation_id": "corr-xyz",
		"created_at":     "2026-05-10T12:00:00Z",
		"resolved_at":    nil,
	}
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	printIncidentDetail(inc)
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()
	for _, want := range []string{"inc-001", "Payments Down", "P1", "open"} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestPrintIncidentDetail_MissingFields(t *testing.T) {
	inc := map[string]any{"id": "only-id"}
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	printIncidentDetail(inc)
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if !bytes.Contains(buf.Bytes(), []byte("only-id")) {
		t.Error("expected id in output")
	}
}

// ── integrationsDir ───────────────────────────────────────────────────────────

func TestIntegrationsDir_ExistingPath(t *testing.T) {
	dir := t.TempDir()
	cmd := integrationsEnableCmd
	_ = cmd.Flags().Lookup("integrations-dir")
	// Use cobra flag mechanism if available; otherwise test the fallback.
	// The fallback checks if ./integrations exists.
	result := integrationsDir(rootCmd)
	// Just verify it returns a non-empty string without panicking.
	if result == "" {
		t.Error("integrationsDir should return a non-empty string")
	}
	_ = dir
}

// ── printIntegrationDefinition ────────────────────────────────────────────────

func TestPrintIntegrationDefinition_OutputsFields(t *testing.T) {
	integ := &integrationpkg.Integration{
		Name:        "slack",
		Version:     "1.0",
		Description: "Slack integration",
		DocsURL:     "https://docs.example.com/slack",
		Receiver: integrationpkg.ReceiverConfig{
			Type: "webhook",
			Path: "/slack/{tenantID}",
		},
		Auth: integrationpkg.AuthConfig{
			Type: "oauth",
			Fields: []integrationpkg.AuthField{
				{Name: "bot_token", Secret: true},
				{Name: "signing_secret", Secret: true},
			},
		},
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	printIntegrationDefinition(integ)
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()
	for _, want := range []string{"slack", "1.0", "webhook", "oauth", "bot_token"} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
}

// ── resolveBinDir ─────────────────────────────────────────────────────────────

func TestResolveBinDir_ReturnsPath(t *testing.T) {
	// resolveBinDir takes no args — just verify it does not panic.
	result := resolveBinDir()
	_ = result
}

// ── JSON output helpers ───────────────────────────────────────────────────────

func TestPrintIncidentTable_JSONBody_ValidatesFields(t *testing.T) {
	data := map[string]any{
		"data": []map[string]any{
			{
				"id":       "inc-999",
				"severity": "P2",
				"title":    "Test incident",
				"status":   "open",
			},
		},
	}
	body, _ := json.Marshal(data)
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	_ = printIncidentTable(body)
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if !bytes.Contains(buf.Bytes(), []byte("inc-999")) {
		t.Error("expected incident ID in table output")
	}
}

// ── configDir edge case ───────────────────────────────────────────────────────

func TestConfigDir_UnderTempHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	got := configDir()
	want := filepath.Join(dir, ".paladin")
	if got != want {
		t.Errorf("configDir() = %q, want %q", got, want)
	}
}
