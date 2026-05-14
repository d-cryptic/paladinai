package tui

import (
	"errors"
	"fmt"
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
	counter := 0
	dashboardSessionPath = func() string {
		counter++
		return filepath.Join(dir, fmt.Sprintf("session-%d.json", counter))
	}
	if err := os.Setenv("PALADIN_TUI_SESSION_PATH", filepath.Join(dir, "external-session.json")); err != nil {
		panic(err)
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

func TestDashboardModel_SlashCommandRunbooks(t *testing.T) {
	m := New("tenant-1")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("runbooks")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := result.(Model)
	if wm.mode != "runbooks" {
		t.Fatalf("mode = %q, want runbooks", wm.mode)
	}
}

func TestDashboardModel_SlashCommandRunbooksSearch(t *testing.T) {
	m := New("tenant-1")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("runbooks search latency")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := result.(Model)
	if wm.mode != "runbook-search" {
		t.Fatalf("mode = %q, want runbook-search", wm.mode)
	}
}

func TestDashboardModel_SlashCommandDoctor(t *testing.T) {
	m := New("tenant-1")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("doctor prometheus")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := result.(Model)
	if wm.mode != "doctor" {
		t.Fatalf("mode = %q, want doctor", wm.mode)
	}
}

func TestDashboardModel_SlashCommandInvestigate(t *testing.T) {
	m := New("tenant-1")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("investigate inc-123")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := result.(Model)
	if wm.mode != "investigate" {
		t.Fatalf("mode = %q, want investigate", wm.mode)
	}
	if wm.focusedID != "inc-123" {
		t.Fatalf("focusedID = %q, want inc-123", wm.focusedID)
	}
}

func TestDashboardModel_EnterInvestigatesSelectedAlert(t *testing.T) {
	m := New("tenant-1")
	m = m.SetAlerts([]Alert{
		{Fingerprint: "fp1", Severity: "P1", Status: "firing", Title: "DB Down", CorrelationID: "corr-1", Tenant: "tenant-1"},
	})

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := result.(Model)
	if wm.mode != "investigate" {
		t.Fatalf("mode = %q, want investigate", wm.mode)
	}
	if wm.focusedID != "corr-1" {
		t.Fatalf("focusedID = %q, want corr-1", wm.focusedID)
	}
}

func TestDashboardModel_SlashCommandStage11Coverage(t *testing.T) {
	cases := []struct {
		command string
		mode    string
	}{
		{command: "integrations", mode: "integrations"},
		{command: "integrations enable github", mode: "integration-enable"},
		{command: "audit severity=p1", mode: "audit"},
		{command: "memory query payments", mode: "memory-query"},
		{command: "pinned", mode: "pinned"},
	}

	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			m := New("tenant-1")
			result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
			result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.command)})
			result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
			wm := result.(Model)
			if wm.mode != tc.mode {
				t.Fatalf("mode = %q, want %q", wm.mode, tc.mode)
			}
		})
	}
}

func TestDashboardModel_SlashCommandConfigValidate(t *testing.T) {
	m := New("tenant-1")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("config validate")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := result.(Model)
	if wm.mode != "config-validate" {
		t.Fatalf("mode = %q, want config-validate", wm.mode)
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

func TestDashboardModel_ViewShowsSummaryAndDetailPane(t *testing.T) {
	m := New("tenant-1")
	m = m.SetAlerts([]Alert{
		{Fingerprint: "fp1", Severity: "P1", Status: "firing", Title: "DB Down", CorrelationID: "corr-1", Tenant: "tenant-1"},
		{Fingerprint: "fp2", Severity: "P2", Status: "triaging", Title: "API Latency", CorrelationID: "corr-2", Tenant: "tenant-1"},
	})

	view := m.View()
	for _, want := range []string{"active=2", "P1=1", "P2=1", "firing=1", "triaging=1", "Incident detail", "DB Down"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestDashboardModel_SetSnapshotShowsLiveCollections(t *testing.T) {
	m := New("tenant-1")
	m = m.SetSnapshot(NewSnapshot(
		[]Alert{{Fingerprint: "fp1", Severity: "P1", Status: "firing", Title: "DB Down", CorrelationID: "corr-1", Tenant: "tenant-1"}},
		[]Runbook{{ID: "rb-1", Title: "Database pool recovery", Source: "github", Embedded: true, UpdatedAt: "2026-05-14T00:00:00Z"}},
		[]Integration{{ID: "prom", Name: "Prometheus", Endpoint: "http://prometheus:9090", Healthy: true, Capabilities: []string{"query_metrics"}}},
	))

	view := m.View()
	for _, want := range []string{"open=1", "critical=1", "runbooks=1", "integrations=1/1"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing metric %q:\n%s", want, view)
		}
	}
}

func TestDashboardModel_RunbooksModeRendersRunbookTableAndDetail(t *testing.T) {
	m := New("tenant-1").SetSnapshot(NewSnapshot(
		nil,
		[]Runbook{{ID: "rb-1", Title: "Redis OOM recovery", Source: "notion", Embedded: true, UpdatedAt: "2026-05-14T00:00:00Z"}},
		nil,
	))

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("runbooks")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := result.(Model)
	view := wm.View()
	for _, want := range []string{"Runbook detail", "Redis OOM recovery", "embedded", "notion"} {
		if !strings.Contains(view, want) {
			t.Fatalf("runbooks view missing %q:\n%s", want, view)
		}
	}
}

func TestDashboardModel_IntegrationsModeRendersIntegrationTableAndDetail(t *testing.T) {
	m := New("tenant-1").SetSnapshot(NewSnapshot(
		nil,
		nil,
		[]Integration{{ID: "grafana", Name: "Grafana MCP", Endpoint: "http://grafana:3000", Healthy: false, Capabilities: []string{"query_dashboards", "list_alerts"}}},
	))

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("integrations")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := result.(Model)
	view := wm.View()
	for _, want := range []string{"Integration detail", "Grafana MCP", "degraded", "query_dashboards"} {
		if !strings.Contains(view, want) {
			t.Fatalf("integrations view missing %q:\n%s", want, view)
		}
	}
}

func TestDashboardModel_EvalsAndAgentsModes(t *testing.T) {
	m := New("tenant-1").SetSnapshot(NewSnapshot(
		[]Alert{{Fingerprint: "fp1", Severity: "P1", Status: "firing", Title: "DB Down", CorrelationID: "corr-1", Tenant: "tenant-1"}},
		[]Runbook{{ID: "rb-1", Title: "Database pool recovery", Source: "github", Embedded: true}},
		[]Integration{{ID: "prom", Name: "Prometheus", Healthy: true}},
	))

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("evals")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := result.(Model)
	if wm.mode != "evals" || !strings.Contains(wm.View(), "golden-regression") {
		t.Fatalf("evals mode did not render eval rows:\n%s", wm.View())
	}

	result, _ = wm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("agents")})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm = result.(Model)
	if wm.mode != "agents" || !strings.Contains(wm.View(), "triage-reactor") {
		t.Fatalf("agents mode did not render agent rows:\n%s", wm.View())
	}
}

func TestDashboardModel_TabCyclesPrimarySurfaces(t *testing.T) {
	m := New("tenant-1").SetSnapshot(NewSnapshot(nil, nil, nil))

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	wm := result.(Model)
	if wm.mode != "investigate" {
		t.Fatalf("mode = %q, want investigate", wm.mode)
	}
	result, _ = wm.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	wm = result.(Model)
	if wm.mode != "monitor" {
		t.Fatalf("mode = %q, want monitor", wm.mode)
	}
}

func TestDashboardModel_ViewEmptyState(t *testing.T) {
	m := New("tenant-1")

	view := m.View()
	if !strings.Contains(view, "No active alerts") {
		t.Fatalf("view missing empty state:\n%s", view)
	}
}

func TestDashboardModel_WindowSizeResizesTable(t *testing.T) {
	m := New("tenant-1")

	result, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 32})
	wm := result.(Model)
	if wm.width != 140 || wm.height != 32 {
		t.Fatalf("window size not stored: width=%d height=%d", wm.width, wm.height)
	}
	if wm.table.Width() <= 84 {
		t.Fatalf("table width did not expand: %d", wm.table.Width())
	}
}

func TestPrependRecentCommandDeduplicatesAndCaps(t *testing.T) {
	got := []string{}
	for _, command := range []string{"/a", "/b", "/c", "/d", "/e", "/f", "/g", "/h", "/i", "/c"} {
		got = prependRecentCommand(got, command)
	}
	if len(got) != 8 {
		t.Fatalf("recent length = %d, want 8: %+v", len(got), got)
	}
	if got[0] != "/c" {
		t.Fatalf("latest command = %q, want /c: %+v", got[0], got)
	}
	for i := 1; i < len(got); i++ {
		if got[i] == "/c" {
			t.Fatalf("duplicate command retained: %+v", got)
		}
	}
}
