package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paladinai/paladinai/internal/projectconfig"
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

func setupInitTest(t *testing.T) (string, string) {
	t.Helper()
	homeDir := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Chdir(projectDir)
	return homeDir, projectDir
}

func TestRunInitPrompt_AcceptsDefaults(t *testing.T) {
	_, projectDir := setupInitTest(t)
	t.Setenv("PALADIN_TOKEN", "")
	t.Setenv("PALADIN_TENANT", "")

	// Fake API server that serves /readyz.
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
	projectCfg, err := projectconfig.ReadFile(filepath.Join(projectDir, "paladin.yaml"))
	if err != nil {
		t.Fatalf("ReadFile paladin.yaml: %v", err)
	}
	if projectCfg.Metadata.Tenant != "default-tenant" {
		t.Errorf("project tenant = %q, want %q", projectCfg.Metadata.Tenant, "default-tenant")
	}
	if projectCfg.Metadata.Tier != "pool" {
		t.Errorf("project tier = %q, want %q", projectCfg.Metadata.Tier, "pool")
	}
}

func TestRunInitPrompt_CustomAPIURL(t *testing.T) {
	_, projectDir := setupInitTest(t)
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
		srv.URL,     // API URL
		"",          // auth URL (default)
		"my-token",  // token
		"acme-corp", // tenant
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
	if loaded.Token != "" {
		t.Errorf("Token = %q, want empty keychain-only config token", loaded.Token)
	}
	stored, err := loadStoredToken(loaded, "http://localhost:9003")
	if err != nil {
		t.Fatal(err)
	}
	if stored != "my-token" {
		t.Errorf("stored token = %q, want %q", stored, "my-token")
	}
	projectCfg, err := projectconfig.ReadFile(filepath.Join(projectDir, "paladin.yaml"))
	if err != nil {
		t.Fatalf("ReadFile paladin.yaml: %v", err)
	}
	if projectCfg.Metadata.Tenant != "acme-corp" {
		t.Errorf("project tenant = %q, want %q", projectCfg.Metadata.Tenant, "acme-corp")
	}
}

func TestRunInitPrompt_InvalidAPIURL_Returns_Error(t *testing.T) {
	setupInitTest(t)
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

func TestRunInitDryRun_DoesNotWriteProjectOrToken(t *testing.T) {
	homeDir, projectDir := setupInitTest(t)
	t.Setenv("PALADIN_TOKEN", "")
	t.Setenv("PALADIN_TENANT", "")

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &PaladinConfig{
		APIEndpoint:   srv.URL,
		AuthEndpoint:  "http://localhost:9003",
		DefaultTenant: "dry-run-tenant",
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	var runErr error
	stdout := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"init", "--dry-run"})
		runErr = rootCmd.Execute()
	})
	if runErr != nil {
		t.Fatalf("dry-run init: %v", runErr)
	}
	if gotPath != "/readyz" {
		t.Fatalf("readiness path = %q, want /readyz", gotPath)
	}
	if !strings.Contains(stdout, "Writes: disabled") {
		t.Fatalf("dry-run output missing write-disabled marker:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "paladin.yaml")); !os.IsNotExist(err) {
		t.Fatalf("dry-run must not write paladin.yaml, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".paladin", "tokens")); !os.IsNotExist(err) {
		t.Fatalf("dry-run must not write token store, stat err=%v", err)
	}
}

func TestRunInitDryRun_InvalidAuthURLReturnsError(t *testing.T) {
	setupInitTest(t)
	t.Setenv("PALADIN_TOKEN", "")
	t.Setenv("PALADIN_TENANT", "")

	cfg := &PaladinConfig{
		APIEndpoint:   "http://localhost:9002",
		AuthEndpoint:  "not-a-url",
		DefaultTenant: "tenant-a",
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	var runErr error
	captureStdout(t, func() {
		rootCmd.SetArgs([]string{"init", "--dry-run"})
		runErr = rootCmd.Execute()
	})
	if runErr == nil {
		t.Fatal("expected invalid auth URL error")
	}
	if !strings.Contains(runErr.Error(), "invalid auth URL") {
		t.Fatalf("error = %v, want invalid auth URL", runErr)
	}
}

// ── applyAndSave ──────────────────────────────────────────────────────────────

func TestApplyAndSave_WritesConfig(t *testing.T) {
	_, projectDir := setupInitTest(t)
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
	if loaded.Token != "" {
		t.Errorf("Token = %q, want empty keychain-only config token", loaded.Token)
	}
	stored, err := loadStoredToken(loaded, "http://localhost:9003")
	if err != nil {
		t.Fatal(err)
	}
	if stored != "tok-xyz" {
		t.Errorf("stored token = %q, want %q", stored, "tok-xyz")
	}
	projectCfg, err := projectconfig.ReadFile(filepath.Join(projectDir, "paladin.yaml"))
	if err != nil {
		t.Fatalf("ReadFile paladin.yaml: %v", err)
	}
	if projectCfg.Metadata.Tenant != "test-tenant" {
		t.Errorf("project tenant = %q, want %q", projectCfg.Metadata.Tenant, "test-tenant")
	}
}

func TestApplyAndSave_VerifiesReadinessEndpoint(t *testing.T) {
	setupInitTest(t)
	t.Setenv("PALADIN_TOKEN", "")

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &PaladinConfig{}
	_ = applyAndSave(initCmd, cfg, srv.URL, "http://localhost:9003", "test-tenant", "tok-xyz", false)

	if gotPath != "/readyz" {
		t.Fatalf("verification path = %q, want /readyz", gotPath)
	}
}

func TestApplyAndSave_TokenFromEnv_NotWrittenToDisk(t *testing.T) {
	homeDir, _ := setupInitTest(t)
	t.Setenv("PALADIN_TOKEN", "env-token")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &PaladinConfig{}
	_ = applyAndSave(initCmd, cfg, srv.URL, "http://localhost:9003", "tenant-x", "disk-token", true)

	data, err := os.ReadFile(filepath.Join(homeDir, ".paladin", "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "token:") {
		t.Errorf("token should not be written to disk when tokenFromEnv=true, got:\n%s", data)
	}
}

func TestApplyAndSave_UnreachableServer_ContinuesWithWarning(t *testing.T) {
	setupInitTest(t)
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

func TestApplyAndSaveProject_WritesSelectedIntegrations(t *testing.T) {
	_, projectDir := setupInitTest(t)
	t.Setenv("PALADIN_TOKEN", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := applyAndSaveProject(
		initCmd,
		&PaladinConfig{},
		srv.URL,
		"http://localhost:9003",
		"tenant-bridge",
		"tok-xyz",
		false,
		"bridge",
		[]string{"slack", "custom-webhook", "prometheus"},
	)
	if err != nil {
		t.Fatalf("applyAndSaveProject: %v", err)
	}

	projectCfg, err := projectconfig.ReadFile(filepath.Join(projectDir, "paladin.yaml"))
	if err != nil {
		t.Fatalf("ReadFile paladin.yaml: %v", err)
	}
	if projectCfg.Metadata.Tenant != "tenant-bridge" {
		t.Errorf("project tenant = %q, want tenant-bridge", projectCfg.Metadata.Tenant)
	}
	if projectCfg.Metadata.Tier != "bridge" {
		t.Errorf("project tier = %q, want bridge", projectCfg.Metadata.Tier)
	}

	enabled := make(map[string]bool)
	for _, integration := range projectCfg.Spec.Integrations {
		enabled[integration.Name] = integration.Enabled
	}
	for _, name := range []string{"prometheus", "slack", "custom-webhook"} {
		if !enabled[name] {
			t.Errorf("integration %q should be enabled", name)
		}
	}
	if enabled["grafana"] {
		t.Error("grafana should remain disabled when it was not selected")
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
