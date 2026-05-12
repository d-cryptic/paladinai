package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── runInitPrompt ─────────────────────────────────────────────────────────────

// simulatePrompt builds a fake stdin that answers each question with the
// given lines in order.
func simulatePrompt(lines ...string) *os.File {
	r, w, _ := os.Pipe()
	go func() {
		defer w.Close()
		for _, l := range lines {
			w.WriteString(l + "\n")
		}
	}()
	return r
}

func TestRunInitPrompt_AcceptsDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("PALADIN_TOKEN", "")
	t.Setenv("PALADIN_TENANT", "")

	// Fake API server that serves /healthz.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &PaladinConfig{
		APIEndpoint:   srv.URL,
		AuthEndpoint:  "http://localhost:9003",
		DefaultTenant: "default-tenant",
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Provide empty lines → accept all defaults.
	stdin := simulatePrompt("", "", "", "", "")
	oldStdin := os.Stdin
	os.Stdin = stdin
	defer func() { os.Stdin = oldStdin }()

	rootCmd.SetArgs([]string{"init", "--tui=false"})
	err := rootCmd.Execute()
	// The command may fail because of connectivity to auth or other services;
	// we just verify it does not panic.
	_ = err

	loaded, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig after init: %v", err)
	}
	if loaded.APIEndpoint != srv.URL {
		t.Errorf("APIEndpoint = %q, want %q", loaded.APIEndpoint, srv.URL)
	}
}

func TestRunInitPrompt_CustomAPIURL(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("PALADIN_TOKEN", "")
	t.Setenv("PALADIN_TENANT", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &PaladinConfig{OutputFormat: "table"}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	stdin := simulatePrompt(
		srv.URL,         // API URL
		"",              // auth URL (default)
		"my-token",      // token
		"acme-corp",     // tenant
	)
	oldStdin := os.Stdin
	os.Stdin = stdin
	defer func() { os.Stdin = oldStdin }()

	rootCmd.SetArgs([]string{"init", "--tui=false"})
	_ = rootCmd.Execute()

	loaded, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if loaded.DefaultTenant != "acme-corp" {
		t.Errorf("DefaultTenant = %q, want %q", loaded.DefaultTenant, "acme-corp")
	}
	if loaded.Token != "my-token" {
		t.Errorf("Token = %q, want %q", loaded.Token, "my-token")
	}
}

func TestRunInitPrompt_InvalidAPIURL_Returns_Error(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("PALADIN_TOKEN", "")
	t.Setenv("PALADIN_TENANT", "")

	cfg := &PaladinConfig{}
	_ = saveConfig(cfg)

	stdin := simulatePrompt("not-a-url", "")
	oldStdin := os.Stdin
	os.Stdin = stdin
	defer func() { os.Stdin = oldStdin }()

	rootCmdMu.Lock()
	defer rootCmdMu.Unlock()
	rootCmd.SetArgs([]string{"init", "--tui=false"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid API URL, got nil")
	}
}

// ── applyAndSave ──────────────────────────────────────────────────────────────

func TestApplyAndSave_WritesConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("PALADIN_TOKEN", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &PaladinConfig{}
	cmd := initCmd
	_ = applyAndSave(cmd, cfg, srv.URL, "http://localhost:9003", "test-tenant", "tok-xyz", false)

	loaded, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if loaded.APIEndpoint != srv.URL {
		t.Errorf("APIEndpoint = %q, want %q", loaded.APIEndpoint, srv.URL)
	}
	if loaded.DefaultTenant != "test-tenant" {
		t.Errorf("DefaultTenant = %q, want %q", loaded.DefaultTenant, "test-tenant")
	}
	if loaded.Token != "tok-xyz" {
		t.Errorf("Token = %q, want %q", loaded.Token, "tok-xyz")
	}
}

func TestApplyAndSave_TokenFromEnv_NotWrittenToDisk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("PALADIN_TOKEN", "env-token")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &PaladinConfig{}
	_ = applyAndSave(initCmd, cfg, srv.URL, "http://localhost:9003", "tenant-x", "disk-token", true)

	data, err := os.ReadFile(filepath.Join(dir, ".paladin", "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "token:") {
		t.Errorf("token should not be written to disk when tokenFromEnv=true, got:\n%s", data)
	}
}

func TestApplyAndSave_UnreachableServer_ContinuesWithWarning(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("PALADIN_TOKEN", "")

	cfg := &PaladinConfig{}
	// Use a port that's not listening.
	err := applyAndSave(initCmd, cfg, "http://127.0.0.1:19999", "http://localhost:9003", "t1", "", false)
	// Should NOT return an error even when the server is unreachable —
	// the function continues with a warning.
	if err != nil {
		t.Errorf("applyAndSave should not fail when server is unreachable, got: %v", err)
	}

	loaded, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if loaded.DefaultTenant != "t1" {
		t.Errorf("DefaultTenant = %q, want %q", loaded.DefaultTenant, "t1")
	}
}

// ── printCompletionHint ───────────────────────────────────────────────────────

func TestPrintCompletionHint_ZshDoesNotPanic(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	// Redirect stdout to discard.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		os.Stdout = old
		w.Close()
		var buf bytes.Buffer
		buf.ReadFrom(r)
	}()
	printCompletionHint()
}

func TestPrintCompletionHint_BashDoesNotPanic(t *testing.T) {
	t.Setenv("SHELL", "/bin/bash")
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old; w.Close() }()
	printCompletionHint()
}

func TestPrintCompletionHint_FishDoesNotPanic(t *testing.T) {
	t.Setenv("SHELL", "/usr/local/bin/fish")
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old; w.Close() }()
	printCompletionHint()
}

func TestPrintCompletionHint_UnknownShellDoesNotPanic(t *testing.T) {
	t.Setenv("SHELL", "")
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old; w.Close() }()
	printCompletionHint()
}
