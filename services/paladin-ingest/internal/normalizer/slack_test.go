package normalizer

import (
	"encoding/json"
	"testing"

	"github.com/paladinai/paladinai/internal/alert"
	"go.uber.org/zap"
)

func TestNormalizeSlack_URLVerification(t *testing.T) {
	raw := json.RawMessage(`{"type":"url_verification","challenge":"abc123"}`)
	envs, err := NormalizeSlack("tenant-a", raw, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 0 {
		t.Errorf("expected no envelopes for url_verification, got %d", len(envs))
	}
}

func TestNormalizeSlack_NonMessageEvent(t *testing.T) {
	raw := json.RawMessage(`{"type":"event_callback","event":{"type":"reaction_added"},"event_time":1700000000}`)
	envs, err := NormalizeSlack("tenant-a", raw, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 0 {
		t.Errorf("expected no envelopes for non-message event, got %d", len(envs))
	}
}

func TestNormalizeSlack_PaladinBotSkipped(t *testing.T) {
	raw := json.RawMessage(`{
		"type":"event_callback",
		"event":{"type":"message","text":"P1 outage","username":"PaladinAI Bot","channel":"C01","ts":"1700000000.000000"},
		"event_time":1700000000
	}`)
	envs, err := NormalizeSlack("tenant-a", raw, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 0 {
		t.Errorf("expected no envelopes for paladin bot message, got %d", len(envs))
	}
}

func TestNormalizeSlack_MessageProducesEnvelope(t *testing.T) {
	raw := json.RawMessage(`{
		"type":"event_callback",
		"team_id":"T01ABC",
		"event":{
			"type":"message",
			"text":"CRITICAL: PostgreSQL primary is down, all writes failing",
			"user":"U01ABC",
			"channel":"C01DEF",
			"ts":"1700000000.000000"
		},
		"event_time":1700000000
	}`)
	envs, err := NormalizeSlack("tenant-a", raw, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 1 {
		t.Fatalf("expected 1 envelope, got %d", len(envs))
	}
	env := envs[0]
	if env.TenantID != "tenant-a" {
		t.Errorf("want tenant-a, got %s", env.TenantID)
	}
	if env.Source != alert.SourceSlack {
		t.Errorf("want source slack, got %s", env.Source)
	}
	if env.Severity != alert.SeverityP1 {
		t.Errorf("want P1 for CRITICAL message, got %s", env.Severity)
	}
	if env.Fingerprint == "" {
		t.Error("fingerprint should not be empty")
	}
	if env.Labels["channel"] != "C01DEF" {
		t.Errorf("want channel C01DEF in labels, got %q", env.Labels["channel"])
	}
}

func TestNormalizeSlack_SeverityInference(t *testing.T) {
	tests := []struct {
		text     string
		wantSev  alert.Severity
	}{
		{"p0 database is down for all tenants", alert.SeverityP1},
		{"[P1] API gateway outage", alert.SeverityP1},
		{"sev2 latency spike detected", alert.SeverityP2},
		{"[P2] High error rate on worker service", alert.SeverityP2},
		{"warning: memory usage elevated on node-3", alert.SeverityP3},
		{"deployment finished successfully", alert.SeverityP4},
	}
	for _, tc := range tests {
		got := inferSlackSeverity(tc.text)
		if got != tc.wantSev {
			t.Errorf("inferSlackSeverity(%q) = %s, want %s", tc.text, got, tc.wantSev)
		}
	}
}

func TestNormalizeSlack_EmptyText(t *testing.T) {
	raw := json.RawMessage(`{
		"type":"event_callback",
		"event":{"type":"message","text":"   ","channel":"C01","ts":"1700000000.000000"},
		"event_time":1700000000
	}`)
	envs, err := NormalizeSlack("tenant-a", raw, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 0 {
		t.Errorf("expected no envelopes for empty text, got %d", len(envs))
	}
}

func TestNormalizeSlack_InvalidJSON(t *testing.T) {
	_, err := NormalizeSlack("tenant-a", json.RawMessage(`not-json`), zap.NewNop())
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestTruncate(t *testing.T) {
	long := string(make([]byte, 200))
	got := truncate(long, 120)
	if len([]rune(got)) > 120 {
		t.Errorf("truncated string too long: %d", len(got))
	}
}
