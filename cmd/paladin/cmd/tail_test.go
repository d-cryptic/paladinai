package cmd

import (
	"io"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// newTestTailCmd creates an isolated tailCmd with the same flag set.
// This avoids package-level flag state leaking between tests.
func newTestTailCmd() *cobra.Command {
	c := &cobra.Command{Use: "tail"}
	c.Flags().String("severity", "", "")
	c.Flags().String("service", "", "")
	c.Flags().String("since", "", "")
	return c
}

func TestAlertWSURL_HTTPtoWS(t *testing.T) {
	c := newTestTailCmd()
	_ = c.ParseFlags([]string{})

	wsURL, err := alertWSURL("http://localhost:8080", c)
	if err != nil {
		t.Fatalf("alertWSURL: %v", err)
	}
	if !strings.HasPrefix(wsURL, "ws://") {
		t.Errorf("expected ws:// prefix, got %q", wsURL)
	}
	if !strings.Contains(wsURL, "/v2/ws/alerts") {
		t.Errorf("expected /v2/ws/alerts path, got %q", wsURL)
	}
}

func TestAlertWSURL_HTTPStoWSS(t *testing.T) {
	c := newTestTailCmd()
	_ = c.ParseFlags([]string{})

	wsURL, err := alertWSURL("https://api.example.com", c)
	if err != nil {
		t.Fatalf("alertWSURL: %v", err)
	}
	if !strings.HasPrefix(wsURL, "wss://") {
		t.Errorf("expected wss:// prefix, got %q", wsURL)
	}
}

func TestAlertWSURL_UnsupportedScheme(t *testing.T) {
	c := newTestTailCmd()
	_ = c.ParseFlags([]string{})

	_, err := alertWSURL("ftp://example.com", c)
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
	if !strings.Contains(err.Error(), "unsupported scheme") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAlertWSURL_WithFilters(t *testing.T) {
	c := newTestTailCmd()
	_ = c.ParseFlags([]string{"--severity", "p1", "--service", "payments-api"})

	wsURL, err := alertWSURL("http://localhost:8080", c)
	if err != nil {
		t.Fatalf("alertWSURL: %v", err)
	}
	if !strings.Contains(wsURL, "severity=p1") {
		t.Errorf("expected severity query param, got %q", wsURL)
	}
	if !strings.Contains(wsURL, "service=payments-api") {
		t.Errorf("expected service query param, got %q", wsURL)
	}
}

func TestAlertWSURL_NoFilters_NoQueryParams(t *testing.T) {
	c := newTestTailCmd()
	_ = c.ParseFlags([]string{})

	wsURL, err := alertWSURL("http://localhost:8080", c)
	if err != nil {
		t.Fatalf("alertWSURL: %v", err)
	}
	if strings.Contains(wsURL, "?") {
		t.Errorf("no filters should produce no query string, got %q", wsURL)
	}
}

func TestPrintEvent_TruncatesLongFields(t *testing.T) {
	ev := AlertEvent{
		Severity: "p1",
		Status:   "firing",
		Service:  strings.Repeat("x", 50), // longer than 24 chars
		Title:    strings.Repeat("y", 50), // longer than 36 chars
		StartsAt: time.Time{},             // zero — falls back to now
	}

	var sb strings.Builder
	w := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	printEvent(w, ev)
	out := sb.String()

	// Service must be truncated
	if strings.Contains(out, strings.Repeat("x", 50)) {
		t.Errorf("service name should be truncated in output")
	}
	// Title must be truncated
	if strings.Contains(out, strings.Repeat("y", 50)) {
		t.Errorf("title should be truncated in output")
	}
	// Output must still contain the severity and status
	if !strings.Contains(out, "P1") {
		t.Errorf("output should contain severity P1, got %q", out)
	}
}

func TestPrintEvent_ZeroTime_UsesNow(t *testing.T) {
	ev := AlertEvent{
		Severity: "p2",
		Status:   "resolved",
		Service:  "api",
		Title:    "test",
		StartsAt: time.Time{}, // zero → use current time
	}
	before := time.Now()
	var sb strings.Builder
	printEvent(tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0), ev)
	after := time.Now()

	out := sb.String()
	// The timestamp in output should be current time (cannot assert exact value,
	// but verify it's present and not empty).
	if !strings.Contains(out, before.Local().Format("15:04")) {
		// Allow for second boundary between before and after
		if !strings.Contains(out, after.Local().Format("15:04")) {
			t.Errorf("zero StartsAt should print current time, got %q", out)
		}
	}
}

func TestPrintEvent_NoSideEffectsOnDiscard(t *testing.T) {
	// Verify printEvent writes to the writer and does not panic.
	ev := AlertEvent{
		Severity: "p3",
		Status:   "firing",
		Service:  "svc",
		Title:    "alert",
		StartsAt: time.Now(),
	}
	w := tabwriter.NewWriter(io.Discard, 0, 0, 2, ' ', 0)
	printEvent(w, ev)
}
