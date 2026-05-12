package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/paladinai/paladinai/services/paladin-comms/internal/notifier"
	"go.uber.org/zap"
)

// ── fake jetstream.Msg ────────────────────────────────────────────────────────

type fakeMsg struct {
	data  []byte
	acked bool
	naked bool
}

func (f *fakeMsg) Metadata() (*jetstream.MsgMetadata, error) { return nil, nil }
func (f *fakeMsg) Data() []byte                              { return f.data }
func (f *fakeMsg) Headers() nats.Header                      { return nil }
func (f *fakeMsg) Subject() string                           { return "test.subject" }
func (f *fakeMsg) Reply() string                             { return "" }
func (f *fakeMsg) Ack() error                                { f.acked = true; return nil }
func (f *fakeMsg) DoubleAck(_ context.Context) error         { f.acked = true; return nil }
func (f *fakeMsg) Nak() error                                { f.naked = true; return nil }
func (f *fakeMsg) NakWithDelay(_ time.Duration) error        { f.naked = true; return nil }
func (f *fakeMsg) InProgress() error                         { return nil }
func (f *fakeMsg) Term() error                               { return nil }
func (f *fakeMsg) TermWithReason(_ string) error             { return nil }

// ── fake Notifier ─────────────────────────────────────────────────────────────

type fakeNotifier struct {
	called    bool
	lastNotif notifier.Notification
	err       error
}

func (f *fakeNotifier) Notify(_ context.Context, n notifier.Notification) error {
	f.called = true
	f.lastNotif = n
	return f.err
}

// ── helpers ───────────────────────────────────────────────────────────────────

func validJSON(severity string) []byte {
	return []byte(`{
		"tenant_id": "acme",
		"incident_id": "inc-001",
		"fingerprint": "abc123",
		"severity": "` + severity + `",
		"title": "DB Down",
		"summary": "Connection pool exhausted",
		"runbook_url": "https://runbooks.acme.com/db",
		"correlation_id": "corr-xyz"
	}`)
}

// ── New ───────────────────────────────────────────────────────────────────────

func TestNew_DefaultMinSeverity(t *testing.T) {
	h := New(&fakeNotifier{}, "", nil)
	if h.minSeverity != "P2" {
		t.Errorf("default minSeverity = %q, want P2", h.minSeverity)
	}
}

func TestNew_NilLogger(t *testing.T) {
	h := New(&fakeNotifier{}, "P1", nil)
	if h.log == nil {
		t.Error("log should not be nil after New")
	}
}

func TestNew_WithLogger(t *testing.T) {
	h := New(&fakeNotifier{}, "P3", zap.NewNop())
	if h.minSeverity != "P3" {
		t.Errorf("minSeverity = %q, want P3", h.minSeverity)
	}
}

// ── shouldNotify ──────────────────────────────────────────────────────────────

func TestShouldNotify_P1_AboveP2Threshold(t *testing.T) {
	h := New(&fakeNotifier{}, "P2", zap.NewNop())
	if !h.shouldNotify("P1") {
		t.Error("P1 should notify when threshold is P2")
	}
}

func TestShouldNotify_P2_AtThreshold(t *testing.T) {
	h := New(&fakeNotifier{}, "P2", zap.NewNop())
	if !h.shouldNotify("P2") {
		t.Error("P2 should notify when threshold is P2")
	}
}

func TestShouldNotify_P3_BelowP2Threshold(t *testing.T) {
	h := New(&fakeNotifier{}, "P2", zap.NewNop())
	if h.shouldNotify("P3") {
		t.Error("P3 should NOT notify when threshold is P2")
	}
}

func TestShouldNotify_P4_BelowThreshold(t *testing.T) {
	h := New(&fakeNotifier{}, "P2", zap.NewNop())
	if h.shouldNotify("P4") {
		t.Error("P4 should NOT notify when threshold is P2")
	}
}

func TestShouldNotify_UnknownSeverity_ReturnsFalse(t *testing.T) {
	h := New(&fakeNotifier{}, "P2", zap.NewNop())
	if h.shouldNotify("CRITICAL") {
		t.Error("unknown severity should NOT notify")
	}
}

func TestShouldNotify_UnknownThreshold_ReturnsTrue(t *testing.T) {
	h := New(&fakeNotifier{}, "UNKNOWN", zap.NewNop())
	if !h.shouldNotify("P1") {
		t.Error("unknown threshold should notify by default")
	}
}

func TestShouldNotify_P1_AtP1Threshold(t *testing.T) {
	h := New(&fakeNotifier{}, "P1", zap.NewNop())
	if !h.shouldNotify("P1") {
		t.Error("P1 should notify when threshold is P1")
	}
}

func TestShouldNotify_P2_BelowP1Threshold(t *testing.T) {
	h := New(&fakeNotifier{}, "P1", zap.NewNop())
	if h.shouldNotify("P2") {
		t.Error("P2 should NOT notify when threshold is P1")
	}
}

// ── ProcessMessage ────────────────────────────────────────────────────────────

func TestProcessMessage_ValidP1_Notifies(t *testing.T) {
	n := &fakeNotifier{}
	h := New(n, "P2", zap.NewNop())
	msg := &fakeMsg{data: validJSON("P1")}

	err := h.ProcessMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !n.called {
		t.Error("expected notifier to be called for P1")
	}
	if n.lastNotif.TenantID != "acme" {
		t.Errorf("TenantID = %q, want acme", n.lastNotif.TenantID)
	}
	if n.lastNotif.Severity != "P1" {
		t.Errorf("Severity = %q, want P1", n.lastNotif.Severity)
	}
	if n.lastNotif.Title != "DB Down" {
		t.Errorf("Title = %q, want DB Down", n.lastNotif.Title)
	}
	if !msg.acked {
		t.Error("message should be acked on success")
	}
}

func TestProcessMessage_P2_AtThreshold_Notifies(t *testing.T) {
	n := &fakeNotifier{}
	h := New(n, "P2", zap.NewNop())
	msg := &fakeMsg{data: validJSON("P2")}

	if err := h.ProcessMessage(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !n.called {
		t.Error("P2 at threshold should notify")
	}
	if !msg.acked {
		t.Error("message should be acked")
	}
}

func TestProcessMessage_P3_BelowThreshold_Skips(t *testing.T) {
	n := &fakeNotifier{}
	h := New(n, "P2", zap.NewNop())
	msg := &fakeMsg{data: validJSON("P3")}

	if err := h.ProcessMessage(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.called {
		t.Error("P3 below threshold should NOT notify")
	}
	if !msg.acked {
		t.Error("skipped message should still be acked")
	}
}

func TestProcessMessage_MalformedJSON_Naks(t *testing.T) {
	n := &fakeNotifier{}
	h := New(n, "P2", zap.NewNop())
	msg := &fakeMsg{data: []byte(`{invalid json`)}

	err := h.ProcessMessage(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if n.called {
		t.Error("notifier should not be called for malformed input")
	}
	if !msg.naked {
		t.Error("malformed message should be nacked")
	}
}

func TestProcessMessage_NotifyFails_Naks(t *testing.T) {
	n := &fakeNotifier{err: errors.New("slack down")}
	h := New(n, "P2", zap.NewNop())
	msg := &fakeMsg{data: validJSON("P1")}

	err := h.ProcessMessage(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error when notifier fails")
	}
	if !msg.naked {
		t.Error("failed notification should nak the message")
	}
	if msg.acked {
		t.Error("failed notification should not ack the message")
	}
}

func TestProcessMessage_NotifyFields_Mapped(t *testing.T) {
	n := &fakeNotifier{}
	h := New(n, "P1", zap.NewNop())
	msg := &fakeMsg{data: validJSON("P1")}

	if err := h.ProcessMessage(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	notif := n.lastNotif
	if notif.IncidentID != "inc-001" {
		t.Errorf("IncidentID = %q, want inc-001", notif.IncidentID)
	}
	if notif.Summary != "Connection pool exhausted" {
		t.Errorf("Summary = %q", notif.Summary)
	}
	if notif.RunbookURL != "https://runbooks.acme.com/db" {
		t.Errorf("RunbookURL = %q", notif.RunbookURL)
	}
}
