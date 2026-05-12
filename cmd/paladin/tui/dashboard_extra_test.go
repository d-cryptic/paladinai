package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSeverityColor_AllCases(t *testing.T) {
	cases := []string{"P1", "P2", "P3", "P4", "unknown", ""}
	for _, sev := range cases {
		c := severityColor(sev)
		_ = c // must not panic
	}
}

func TestTruncateStr_LongString(t *testing.T) {
	s := strings.Repeat("a", 50)
	got := truncateStr(s, 10)
	if len([]rune(got)) > 10 {
		t.Errorf("truncated string too long: %d chars", len(got))
	}
	if !strings.Contains(got, "…") {
		t.Error("expected ellipsis in truncated string")
	}
}

func TestTruncateStr_ShortString(t *testing.T) {
	got := truncateStr("hi", 10)
	if got != "hi" {
		t.Errorf("expected unchanged string, got %q", got)
	}
}

func TestDashboardModel_Update_Quit(t *testing.T) {
	m := New("tenant-1")
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	_ = result
	if cmd == nil {
		t.Error("quit key should return a tea.Quit cmd")
	}
}

func TestDashboardModel_Update_CtrlC(t *testing.T) {
	m := New("tenant-1")
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	_ = result
	if cmd == nil {
		t.Error("ctrl+c should return tea.Quit")
	}
}

func TestDashboardModel_Update_Refresh(t *testing.T) {
	m := New("tenant-1")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	_ = result
	// should not panic
}

func TestDashboardModel_Update_AlertsLoaded(t *testing.T) {
	m := New("tenant-1")
	alerts := AlertsLoadedMsg{
		{Fingerprint: "abc123", Severity: "P1", Status: "firing", Title: "DB Down", CorrelationID: "corr-1", Tenant: "tenant-1"},
		{Fingerprint: "def456", Severity: "P2", Status: "firing", Title: "High CPU", CorrelationID: "corr-2", Tenant: "tenant-1"},
	}
	result, _ := m.Update(alerts)
	wm := result.(Model)
	if len(wm.alerts) != 2 {
		t.Errorf("expected 2 alerts, got %d", len(wm.alerts))
	}
}

func TestDashboardModel_Update_ErrMsg(t *testing.T) {
	m := New("tenant-1")
	err := errors.New("connection refused")
	result, _ := m.Update(ErrMsg{Err: err})
	wm := result.(Model)
	if wm.err == nil {
		t.Error("expected error to be stored in model")
	}
}

func TestDashboardModel_View_WithError(t *testing.T) {
	m := New("tenant-1")
	m.err = errors.New("API unavailable")
	view := m.View()
	if !strings.Contains(view, "Error") {
		t.Error("view should contain error message")
	}
}

func TestDashboardModel_View_WithAlerts(t *testing.T) {
	m := New("tenant-1")
	m = m.SetAlerts([]Alert{
		{Fingerprint: "fp1", Severity: "P1", Status: "firing", Title: "Test Alert"},
	})
	view := m.View()
	if !strings.Contains(view, "PaladinAI") {
		t.Error("view should contain dashboard header")
	}
}

func TestDashboardModel_View_NoError(t *testing.T) {
	m := New("tenant-1")
	view := m.View()
	if !strings.Contains(view, "tenant-1") {
		t.Error("view should contain tenant name")
	}
	if strings.Contains(view, "Error") {
		t.Error("view should not contain error when none set")
	}
}

func TestDashboardModel_SetAlerts_LongStrings(t *testing.T) {
	m := New("tenant-1")
	alerts := []Alert{
		{
			Fingerprint:   strings.Repeat("x", 40),
			Severity:      "P1",
			Status:        "firing",
			Title:         strings.Repeat("Long Title ", 10),
			CorrelationID: strings.Repeat("c", 25),
			Tenant:        "tenant-1",
		},
	}
	m = m.SetAlerts(alerts)
	if len(m.alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(m.alerts))
	}
}
