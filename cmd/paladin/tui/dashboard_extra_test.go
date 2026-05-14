package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "paladin-tui-session-*")
	if err != nil {
		panic(err)
	}
	dashboardSessionPath = func() string {
		return filepath.Join(dir, "session.json")
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

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

func TestDashboardModel_SlashCommandTailMode(t *testing.T) {
	m := New("tenant-1")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	wm := result.(Model)
	if !wm.slashMode || wm.commandInput != "/" {
		t.Fatalf("slash mode not started: %+v", wm)
	}
	result, _ = wm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tail")})
	wm = result.(Model)
	result, cmd := wm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("tail command should not quit")
	}
	wm = result.(Model)
	if wm.mode != "tail" {
		t.Fatalf("mode = %q, want tail", wm.mode)
	}
	if len(wm.recent) == 0 || wm.recent[0] != "/tail" {
		t.Fatalf("recent commands = %+v, want latest /tail", wm.recent)
	}
}

func TestDashboardModel_SlashCommandHelpOverlay(t *testing.T) {
	m := New("tenant-1")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("help")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := result.(Model)
	if !wm.helpVisible {
		t.Fatal("help should be visible")
	}
	if !strings.Contains(wm.View(), "/incidents") {
		t.Fatal("help view should contain slash commands")
	}
}

func TestDashboardModel_SlashCommandQuit(t *testing.T) {
	m := New("tenant-1")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("quit")})
	_, cmd := result.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("quit command should return tea.Quit")
	}
}

func TestDashboardModel_UnknownSlashCommandSetsError(t *testing.T) {
	m := New("tenant-1")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("nope")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := result.(Model)
	if wm.err == nil || !strings.Contains(wm.err.Error(), "unknown command") {
		t.Fatalf("expected unknown command error, got %v", wm.err)
	}
}

func TestDashboardSession_SaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".paladin", "session.json")
	want := SessionState{Mode: "tail", Recent: []string{"/tail", "/help"}}
	if err := saveSessionState(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadSessionState(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "tail" || len(got.Recent) != 2 || got.Recent[0] != "/tail" {
		t.Fatalf("state = %+v, want %+v", got, want)
	}
}

func TestDashboardModel_NewRestoresSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".paladin", "session.json")
	if err := saveSessionState(path, SessionState{Mode: "configure", Recent: []string{"/config"}}); err != nil {
		t.Fatal(err)
	}
	oldPath := dashboardSessionPath
	dashboardSessionPath = func() string { return path }
	t.Cleanup(func() { dashboardSessionPath = oldPath })

	m := New("tenant-1")
	if m.mode != "configure" {
		t.Fatalf("mode = %q, want configure", m.mode)
	}
	if len(m.recent) != 1 || m.recent[0] != "/config" {
		t.Fatalf("recent = %+v, want /config", m.recent)
	}
}

func TestDashboardModel_SlashCommandPersistsSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".paladin", "session.json")
	oldPath := dashboardSessionPath
	dashboardSessionPath = func() string { return path }
	t.Cleanup(func() { dashboardSessionPath = oldPath })

	m := New("tenant-1")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("config")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := result.(Model)
	if wm.err != nil {
		t.Fatalf("unexpected error: %v", wm.err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"mode": "configure"`) {
		t.Fatalf("session missing configure mode:\n%s", data)
	}
}

func TestDashboardSession_InvalidModeFallsBackToMonitor(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".paladin", "session.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"mode":"bad","recent":["/bad"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := loadSessionState(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.Mode != "monitor" {
		t.Fatalf("mode = %q, want monitor", state.Mode)
	}
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
