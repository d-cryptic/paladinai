package cmd

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	integrationpkg "github.com/paladinai/paladinai/internal/integrationpkg"
	"github.com/spf13/cobra"
)

// ── tenantStateAction ─────────────────────────────────────────────────────────

func TestTenantStateAction_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api/v1/tenants/acme-corp/suspend") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"acme-corp","state":"suspended"}`))
	}))
	defer srv.Close()

	cmd := makeTestCmdWithAuthURL(srv.URL)
	err := tenantStateAction(cmd, "acme-corp", "suspend")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTenantStateAction_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	cmd := makeTestCmdWithAuthURL(srv.URL)
	err := tenantStateAction(cmd, "acme-corp", "suspend")
	if err == nil {
		t.Error("expected error on 403 response")
	}
}

func TestTenantStateAction_InvalidURL(t *testing.T) {
	cmd := makeTestCmdWithAuthURL("://bad-url")
	err := tenantStateAction(cmd, "acme-corp", "suspend")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func makeTestCmdWithAuthURL(u string) *cobra.Command {
	c := &cobra.Command{Use: "test"}
	c.Flags().String("auth-url", u, "")
	c.Flags().String("admin-secret", "secret", "")
	c.SetContext(context.Background())
	return c
}

// ── saveConfig ────────────────────────────────────────────────────────────────

func TestSaveConfig_WritesToFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &PaladinConfig{
		APIEndpoint:   "http://localhost:9001",
		DefaultTenant: "test-tenant",
		OutputFormat:  "json",
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig error: %v", err)
	}
	path := filepath.Join(dir, ".paladin", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	if !strings.Contains(string(data), "test-tenant") {
		t.Errorf("config file missing tenant, got:\n%s", data)
	}
}

// ── filterServices ────────────────────────────────────────────────────────────

func TestFilterServices_EmptyAllowlist_ReturnsAll(t *testing.T) {
	all := []serviceSpec{{name: "ingest"}, {name: "agent"}, {name: "edge"}}
	got := filterServices(all, nil)
	if len(got) != 3 {
		t.Errorf("expected 3 services, got %d", len(got))
	}
}

func TestFilterServices_WithAllowlist_FiltersCorrectly(t *testing.T) {
	all := []serviceSpec{{name: "ingest"}, {name: "agent"}, {name: "edge"}}
	got := filterServices(all, []string{"ingest", "edge"})
	if len(got) != 2 {
		t.Errorf("expected 2 services, got %d", len(got))
	}
	names := make(map[string]bool)
	for _, s := range got {
		names[s.name] = true
	}
	if !names["ingest"] || !names["edge"] {
		t.Error("expected ingest and edge in result")
	}
}

func TestFilterServices_NoMatch_ReturnsEmpty(t *testing.T) {
	all := []serviceSpec{{name: "ingest"}, {name: "agent"}}
	got := filterServices(all, []string{"unknown"})
	if len(got) != 0 {
		t.Errorf("expected empty result, got %+v", got)
	}
}

// ── linePrefixWriter ──────────────────────────────────────────────────────────

func TestLinePrefixWriter_Write_SingleLine(t *testing.T) {
	r, w, _ := os.Pipe()
	pw := prefixWriter(w, "ingest")
	_, _ = pw.Write([]byte("hello world\n"))
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "[ingest") {
		t.Errorf("expected prefix in output, got: %q", out)
	}
	if !strings.Contains(out, "hello world") {
		t.Errorf("expected content in output, got: %q", out)
	}
}

func TestLinePrefixWriter_Write_MultiLine(t *testing.T) {
	r, w, _ := os.Pipe()
	pw := prefixWriter(w, "agent")
	_, _ = pw.Write([]byte("line1\nline2\n"))
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()
	if strings.Count(out, "[agent") < 2 {
		t.Errorf("expected 2 prefixed lines, got: %q", out)
	}
}

func TestLinePrefixWriter_Write_PartialLine(t *testing.T) {
	r, w, _ := os.Pipe()
	pw := prefixWriter(w, "edge")
	_, _ = pw.Write([]byte("partial"))
	_, _ = pw.Write([]byte(" line\n"))
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "partial line") {
		t.Errorf("expected partial+line combined, got: %q", out)
	}
}

// ── resolveBinary ─────────────────────────────────────────────────────────────

func TestResolveBinary_FindsInBinDir(t *testing.T) {
	dir := t.TempDir()
	// Create a fake binary file
	fakeBin := filepath.Join(dir, "mybin")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := resolveBinary(dir, "mybin")
	if got != fakeBin {
		t.Errorf("expected %q, got %q", fakeBin, got)
	}
}

func TestResolveBinary_FallsBackToPath(t *testing.T) {
	// "sh" is always on $PATH
	got := resolveBinary("/nonexistent/dir", "sh")
	if got == "" {
		t.Error("expected to find 'sh' in PATH")
	}
}

func TestResolveBinary_NotFound_ReturnsEmpty(t *testing.T) {
	got := resolveBinary("/nonexistent/dir", "definitely-not-a-real-binary-xyz")
	if got != "" {
		t.Errorf("expected empty string for missing binary, got %q", got)
	}
}

// ── printIntegrationDefinition with tools and configSchema ────────────────────

func TestPrintIntegrationDefinition_WithToolsAndSchema(t *testing.T) {
	integ := &integrationpkg.Integration{
		Name:    "datadog",
		Version: "2.0",
		Tools:   []string{"query_metrics", "list_monitors", "create_event"},
		ConfigSchema: map[string]integrationpkg.Field{
			"api_key": {Type: "string", Required: true, Description: "Datadog API key"},
			"site":    {Type: "string", Required: false, Description: "Datadog site"},
		},
		Auth: integrationpkg.AuthConfig{
			Type: "apikey",
			Fields: []integrationpkg.AuthField{
				{Name: "api_key", Secret: true, Description: "API key"},
			},
		},
		Receiver: integrationpkg.ReceiverConfig{
			Type:       "webhook",
			HMACHeader: "X-DD-Signature",
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

	for _, want := range []string{"datadog", "query_metrics", "api_key", "X-DD-Signature"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
}
