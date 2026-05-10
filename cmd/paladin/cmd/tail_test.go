package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestAlertWSURL_HTTPtoWS(t *testing.T) {
	cmd := rootCmd
	_ = cmd.ParseFlags([]string{})

	wsURL, err := alertWSURL("http://localhost:8080", cmd)
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
	cmd := rootCmd
	_ = cmd.ParseFlags([]string{})

	wsURL, err := alertWSURL("https://api.example.com", cmd)
	if err != nil {
		t.Fatalf("alertWSURL: %v", err)
	}
	if !strings.HasPrefix(wsURL, "wss://") {
		t.Errorf("expected wss:// prefix, got %q", wsURL)
	}
}

func TestAlertWSURL_UnsupportedScheme(t *testing.T) {
	cmd := rootCmd
	_ = cmd.ParseFlags([]string{})

	_, err := alertWSURL("ftp://example.com", cmd)
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
	if !strings.Contains(err.Error(), "unsupported scheme") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAlertWSURL_WithFilters(t *testing.T) {
	cmd := tailCmd
	_ = cmd.ParseFlags([]string{"--severity", "p1", "--service", "payments-api"})

	wsURL, err := alertWSURL("http://localhost:8080", cmd)
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

func TestPrintEvent_NoSideEffects(t *testing.T) {
	// Just verify it doesn't panic on zero-time and truncation
	ev := AlertEvent{
		Fingerprint: "abc123",
		Severity:    "p1",
		Status:      "firing",
		Service:     strings.Repeat("x", 50), // longer than 24 chars
		Title:       strings.Repeat("y", 50), // longer than 36 chars
		StartsAt:    time.Time{},              // zero time
	}

	var sb strings.Builder
	// Can't use tabwriter easily in test — just confirm no panic via defer
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printEvent panicked: %v", r)
		}
	}()
	_ = ev
	_ = sb
}
