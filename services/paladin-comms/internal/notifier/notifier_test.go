package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── SlackNotifier ─────────────────────────────────────────────────────────────

func TestSlackNotifier_Notify_SendsCorrectJSON(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewSlackNotifier(srv.URL)
	err := n.Notify(context.Background(), Notification{
		TenantID:   "acme",
		IncidentID: "inc-001",
		Severity:   "P1",
		Title:      "DB Down",
		Summary:    "Connection pool exhausted",
		RunbookURL: "https://runbooks.acme.com/db",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("invalid JSON sent to Slack: %v", err)
	}
	text, _ := payload["text"].(string)
	if !strings.Contains(text, "P1") || !strings.Contains(text, "DB Down") {
		t.Errorf("slack text missing severity/title: %q", text)
	}
	attachments, _ := payload["attachments"].([]any)
	if len(attachments) == 0 {
		t.Error("expected at least one attachment")
	}
}

func TestSlackNotifier_Notify_Non200_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := NewSlackNotifier(srv.URL)
	err := n.Notify(context.Background(), Notification{Severity: "P1", Title: "test"})
	if err == nil {
		t.Error("expected error for non-200 response")
	}
}

func TestSlackNotifier_Notify_WithoutSummaryOrRunbook(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewSlackNotifier(srv.URL)
	err := n.Notify(context.Background(), Notification{
		TenantID: "tenant-1",
		Severity: "P2",
		Title:    "High CPU",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSeverityColor_AllCases(t *testing.T) {
	cases := map[string]string{
		"P1":      "danger",
		"P2":      "warning",
		"P3":      "good",
		"P4":      "#aaaaaa",
		"unknown": "#aaaaaa",
	}
	for sev, want := range cases {
		got := severityColor(sev)
		if got != want {
			t.Errorf("severityColor(%q) = %q, want %q", sev, got, want)
		}
	}
}

// ── PagerDutyNotifier ─────────────────────────────────────────────────────────

func TestPagerDutyNotifier_Notify_SendsCorrectJSON(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	n := &PagerDutyNotifier{
		routingKey: "test-key",
		eventsURL:  srv.URL,
		client:     &http.Client{},
	}
	err := n.Notify(context.Background(), Notification{
		TenantID:   "acme",
		IncidentID: "inc-002",
		Severity:   "P2",
		Title:      "High Memory",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("invalid JSON sent to PagerDuty: %v", err)
	}
	if payload["routing_key"] != "test-key" {
		t.Errorf("routing_key mismatch: %v", payload["routing_key"])
	}
	if payload["event_action"] != "trigger" {
		t.Errorf("event_action mismatch: %v", payload["event_action"])
	}
	if payload["dedup_key"] != "inc-002" {
		t.Errorf("dedup_key mismatch: %v", payload["dedup_key"])
	}
}

func TestPagerDutyNotifier_Notify_Non202_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	n := &PagerDutyNotifier{routingKey: "key", eventsURL: srv.URL, client: &http.Client{}}
	err := n.Notify(context.Background(), Notification{Severity: "P1"})
	if err == nil {
		t.Error("expected error for non-202 response")
	}
}

func TestPDSeverity_AllCases(t *testing.T) {
	cases := map[string]string{
		"P1": "critical",
		"P2": "error",
		"P3": "warning",
		"P4": "info",
		"":   "info",
	}
	for sev, want := range cases {
		got := pdSeverity(sev)
		if got != want {
			t.Errorf("pdSeverity(%q) = %q, want %q", sev, got, want)
		}
	}
}

// ── MultiNotifier ─────────────────────────────────────────────────────────────

type fakeNotifier struct {
	called bool
	err    error
}

func (f *fakeNotifier) Notify(_ context.Context, _ Notification) error {
	f.called = true
	return f.err
}

func TestMultiNotifier_Notify_CallsAll(t *testing.T) {
	a, b := &fakeNotifier{}, &fakeNotifier{}
	m := NewMultiNotifier(a, b)
	if err := m.Notify(context.Background(), Notification{Severity: "P1"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !a.called || !b.called {
		t.Error("expected both notifiers to be called")
	}
}

func TestMultiNotifier_Notify_CollectsErrors(t *testing.T) {
	bad := &fakeNotifier{err: errTest("notify failed")}
	good := &fakeNotifier{}
	m := NewMultiNotifier(bad, good)
	err := m.Notify(context.Background(), Notification{Severity: "P1"})
	if err == nil {
		t.Error("expected error from failed notifier")
	}
	if !good.called {
		t.Error("good notifier should still be called after bad one fails")
	}
}

func TestMultiNotifier_Notify_AllFail(t *testing.T) {
	bad1 := &fakeNotifier{err: errTest("first")}
	bad2 := &fakeNotifier{err: errTest("second")}
	m := NewMultiNotifier(bad1, bad2)
	err := m.Notify(context.Background(), Notification{})
	if err == nil {
		t.Error("expected error when all notifiers fail")
	}
}

func TestMultiNotifier_Empty_ReturnsNil(t *testing.T) {
	m := NewMultiNotifier()
	if err := m.Notify(context.Background(), Notification{}); err != nil {
		t.Errorf("empty multi-notifier should return nil, got: %v", err)
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }
