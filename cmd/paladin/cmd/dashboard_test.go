package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// cmdWithCtx wraps newTestCmd and sets a non-nil Context (required for
// client.Get which calls http.NewRequestWithContext).
func cmdWithCtx(tenant, token, apiURL string) *cobra.Command {
	c := newTestCmd(tenant, token, apiURL)
	c.SetContext(context.Background())
	return c
}

func TestFetchAlerts_Success(t *testing.T) {
	payload := map[string]any{
		"data": []map[string]any{
			{
				"fingerprint":    "abc123",
				"severity":       "P1",
				"status":         "firing",
				"title":          "Payments API down",
				"correlation_id": "corr-001",
			},
			{
				"fingerprint":    "def456",
				"severity":       "P2",
				"status":         "firing",
				"title":          "High latency",
				"correlation_id": "corr-002",
			},
		},
	}
	body, _ := json.Marshal(payload)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cmd := cmdWithCtx("tenant-x", "", srv.URL)

	alerts, err := fetchAlerts(cmd, srv.URL, "tenant-x")
	if err != nil {
		t.Fatalf("fetchAlerts error: %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(alerts))
	}
	if alerts[0].Fingerprint != "abc123" {
		t.Errorf("unexpected fingerprint: %q", alerts[0].Fingerprint)
	}
	if alerts[1].Severity != "P2" {
		t.Errorf("unexpected severity: %q", alerts[1].Severity)
	}
}

func TestFetchAlerts_InvalidURL(t *testing.T) {
	cmd := newTestCmd("tenant-x", "", "://bad")
	_, err := fetchAlerts(cmd, "://bad-url", "tenant-x")
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}

func TestFetchAlerts_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cmd := cmdWithCtx("tenant-x", "", srv.URL)
	_, err := fetchAlerts(cmd, srv.URL, "tenant-x")
	if err == nil {
		t.Error("expected error on server 500, got nil")
	}
}

func TestFetchAlerts_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	cmd := cmdWithCtx("tenant-x", "", srv.URL)
	_, err := fetchAlerts(cmd, srv.URL, "tenant-x")
	if err == nil {
		t.Error("expected JSON parse error, got nil")
	}
}

func TestFetchAlerts_EmptyData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	cmd := cmdWithCtx("tenant-a", "", srv.URL)
	alerts, err := fetchAlerts(cmd, srv.URL, "tenant-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestDashboardCommand_CIModeWritesJSONWithoutTUI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"fingerprint":"abc123","severity":"P1","status":"firing","title":"Payments API down","correlation_id":"corr-001"}]}`))
	}))
	defer srv.Close()

	cmd := newDashboardCommandForTest("tenant-x", srv.URL)
	_ = cmd.Flags().Set("ci", "true")

	var runErr error
	out := captureStdout(t, func() {
		runErr = dashboardCmd.RunE(cmd, nil)
	})
	if runErr != nil {
		t.Fatalf("dashboard RunE error: %v", runErr)
	}
	if !strings.Contains(out, `"tenant":"tenant-x"`) || !strings.Contains(out, `"fingerprint":"abc123"`) {
		t.Fatalf("dashboard JSON output missing expected data:\n%s", out)
	}
}

func TestDashboardCommand_SimpleModeWritesTableWithoutTUI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"fingerprint":"abc123","severity":"P1","status":"firing","title":"Payments API down","correlation_id":"corr-001"}]}`))
	}))
	defer srv.Close()

	cmd := newDashboardCommandForTest("tenant-x", srv.URL)
	_ = cmd.Flags().Set("simple", "true")

	var runErr error
	out := captureStdout(t, func() {
		runErr = dashboardCmd.RunE(cmd, nil)
	})
	if runErr != nil {
		t.Fatalf("dashboard RunE error: %v", runErr)
	}
	if !strings.Contains(out, "TENANT") || !strings.Contains(out, "Payments API down") {
		t.Fatalf("dashboard table output missing expected data:\n%s", out)
	}
}

func newDashboardCommandForTest(tenant, apiURL string) *cobra.Command {
	cmd := &cobra.Command{Use: "dashboard"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("tenant", tenant, "")
	cmd.Flags().String("token", "", "")
	cmd.Flags().String("api-url", apiURL, "")
	cmd.Flags().StringP("output", "o", "table", "")
	cmd.Flags().Bool("ci", false, "")
	cmd.Flags().Bool("simple", false, "")
	return cmd
}
