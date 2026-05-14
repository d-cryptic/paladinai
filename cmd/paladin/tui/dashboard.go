// Package tui implements the Bubble Tea interactive dashboard for paladin.
// The dashboard shows active alerts in real-time and allows keyboard navigation.
package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Alert is a simplified view model for the TUI (decoupled from alert.AlertEnvelope).
type Alert struct {
	Fingerprint   string
	Severity      string
	Status        string
	Title         string
	CorrelationID string
	Tenant        string
}

type SessionState struct {
	Mode   string   `json:"mode"`
	Recent []string `json:"recent"`
}

// severityColor maps severity to a lipgloss color.
func severityColor(sev string) lipgloss.Color {
	switch strings.ToUpper(sev) {
	case "P1":
		return lipgloss.Color("#FF0000")
	case "P2":
		return lipgloss.Color("#FF8C00")
	case "P3":
		return lipgloss.Color("#FFD700")
	default:
		return lipgloss.Color("#6C757D")
	}
}

var (
	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#444444"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))
)

var dashboardSessionPath = defaultDashboardSessionPath

// Model is the Bubble Tea model for the alert dashboard.
type Model struct {
	table        table.Model
	alerts       []Alert
	tenant       string
	err          error
	mode         string
	commandInput string
	slashMode    bool
	helpVisible  bool
	recent       []string
	width        int
	height       int
}

// AlertsLoadedMsg is sent when alerts are fetched from the API.
type AlertsLoadedMsg []Alert

// ErrMsg wraps fetch errors.
type ErrMsg struct{ Err error }

// New creates a dashboard Model for the given tenant.
func New(tenant string) Model {
	cols := []table.Column{
		{Title: "SEV", Width: 4},
		{Title: "STATUS", Width: 9},
		{Title: "TITLE", Width: 32},
		{Title: "CORRELATION", Width: 18},
		{Title: "FINGERPRINT", Width: 16},
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#444")).BorderBottom(true).Bold(true)
	s.Selected = s.Selected.Foreground(lipgloss.Color("#FFFDF5")).Background(lipgloss.Color("#25A065")).Bold(false)
	t.SetStyles(s)

	m := Model{table: t, tenant: tenant, mode: "monitor", width: 96, height: 28}
	state, err := loadSessionState(dashboardSessionPath())
	if err != nil {
		return m
	}
	m.applySessionState(state)
	return m
}

// SetAlerts updates the model with a fresh batch of alerts.
func (m Model) SetAlerts(alerts []Alert) Model {
	m.alerts = alerts
	rows := make([]table.Row, len(alerts))
	for i, a := range alerts {
		rows[i] = table.Row{
			colourSeverity(a.Severity),
			a.Status,
			truncateStr(a.Title, 32),
			truncateStr(a.CorrelationID, 18),
			truncateStr(a.Fingerprint, 16),
		}
	}
	m.table.SetRows(rows)
	return m
}

func colourSeverity(sev string) string {
	return lipgloss.NewStyle().Foreground(severityColor(sev)).Render(sev)
}

func truncateStr(s string, max int) string {
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}

// Init is called once when the program starts.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and key events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(tableWidth(msg.Width))
		m.table.SetHeight(tableHeight(msg.Height))
		return m, nil
	case tea.KeyMsg:
		if m.slashMode {
			next, cmd := m.updateCommandInput(msg)
			return next, cmd
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			return m, nil // caller can wire a refresh command
		case "/":
			m.slashMode = true
			m.commandInput = "/"
			return m, nil
		case "?":
			m.helpVisible = !m.helpVisible
			return m, nil
		}
	case AlertsLoadedMsg:
		m = m.SetAlerts([]Alert(msg))
	case ErrMsg:
		m.err = msg.Err
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) updateCommandInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.slashMode = false
		m.commandInput = ""
		return m, nil
	case tea.KeyBackspace:
		if len(m.commandInput) > 0 {
			m.commandInput = m.commandInput[:len(m.commandInput)-1]
		}
		if m.commandInput == "" {
			m.slashMode = false
		}
		return m, nil
	case tea.KeyEnter:
		command := strings.TrimSpace(m.commandInput)
		m.slashMode = false
		m.commandInput = ""
		if command != "" {
			m.recent = prependRecentCommand(m.recent, command)
		}
		return m.applySlashCommand(command)
	case tea.KeyRunes:
		m.commandInput += string(msg.Runes)
		return m, nil
	}
	return m, nil
}

func (m Model) applySlashCommand(command string) (tea.Model, tea.Cmd) {
	name := strings.Fields(strings.TrimPrefix(command, "/"))
	if len(name) == 0 {
		return m, nil
	}
	switch name[0] {
	case "q", "quit":
		return m, tea.Quit
	case "help":
		m.helpVisible = true
	case "tail":
		m.mode = "tail"
	case "config":
		if len(name) > 1 && name[1] == "validate" {
			m.mode = "config-validate"
		} else {
			m.mode = "configure"
		}
	case "incidents":
		m.mode = "monitor"
	case "runbooks":
		if len(name) > 1 && name[1] == "search" {
			m.mode = "runbook-search"
		} else {
			m.mode = "runbooks"
		}
	case "doctor":
		m.mode = "doctor"
	default:
		m.err = fmt.Errorf("unknown command: /%s", name[0])
	}
	if err := saveSessionState(dashboardSessionPath(), m.sessionState()); err != nil {
		m.err = fmt.Errorf("save session: %w", err)
	}
	return m, nil
}

func (m *Model) applySessionState(state SessionState) {
	if isDashboardMode(state.Mode) {
		m.mode = state.Mode
	}
	m.recent = append([]string(nil), state.Recent...)
}

func (m Model) sessionState() SessionState {
	return SessionState{
		Mode:   m.mode,
		Recent: append([]string(nil), m.recent...),
	}
}

func prependRecentCommand(recent []string, command string) []string {
	next := make([]string, 0, minInt(len(recent)+1, 8))
	next = append(next, command)
	for _, item := range recent {
		if item == command {
			continue
		}
		next = append(next, item)
		if len(next) == 8 {
			break
		}
	}
	return next
}

func isDashboardMode(mode string) bool {
	switch mode {
	case "monitor", "tail", "configure", "config-validate", "runbooks", "runbook-search", "doctor":
		return true
	default:
		return false
	}
}

func defaultDashboardSessionPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".paladin", "session.json")
}

func loadSessionState(path string) (SessionState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SessionState{}, err
	}
	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return SessionState{}, fmt.Errorf("parse session: %w", err)
	}
	if !isDashboardMode(state.Mode) {
		state.Mode = "monitor"
	}
	return state, nil
}

func saveSessionState(path string, state SessionState) error {
	if state.Mode == "" {
		state.Mode = "monitor"
	}
	if len(state.Recent) > 8 {
		state.Recent = append([]string(nil), state.Recent[:8]...)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	return nil
}

// View renders the dashboard.
func (m Model) View() string {
	header := titleStyle.Width(contentWidth(m.width)).Render(
		fmt.Sprintf(" PaladinAI Dashboard — tenant: %s • mode: %s ", m.tenant, m.mode),
	)
	help := helpStyle.Render("↑/↓ navigate  •  enter inspect  •  / commands  •  ? help  •  r refresh  •  q quit")

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		baseStyle.Render(m.table.View()),
		" ",
		baseStyle.Width(detailWidth(m.width)).Render(m.detailPane()),
	)

	errLine := ""
	if m.err != nil {
		errLine = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(
			fmt.Sprintf("Error: %v", m.err))
	}

	parts := []string{header, summaryStrip(m.alerts), body}
	if errLine != "" {
		parts = append(parts, errLine)
	}
	if m.helpVisible {
		parts = append(parts, helpOverlay())
	}
	parts = append(parts, commandBar(m))
	parts = append(parts, help)
	return strings.Join(parts, "\n")
}

func commandBar(m Model) string {
	input := m.commandInput
	if input == "" {
		input = "_"
	}
	recent := ""
	if len(m.recent) > 0 {
		recent = "  recent: " + strings.Join(m.recent[:minInt(len(m.recent), 3)], "  ")
	}
	return helpStyle.Render("/incidents  /tail  /runbooks  /doctor  /config  /help  /quit"+recent) + "\n> " + input
}

func helpOverlay() string {
	return baseStyle.Render(strings.Join([]string{
		"Keyboard",
		"  /      command mode",
		"  ?      toggle help",
		"  q      quit",
		"Commands",
		"  /incidents  monitor mode",
		"  /tail       live tail mode",
		"  /runbooks   runbook explorer",
		"  /doctor     health checks",
		"  /config     configure mode",
		"  /quit       exit",
	}, "\n"))
}

func (m Model) detailPane() string {
	if len(m.alerts) == 0 {
		return strings.Join([]string{
			"Incident detail",
			"",
			"No active alerts.",
			"Run `paladin tail` for the live stream or press r to refresh.",
		}, "\n")
	}
	alert := m.selectedAlert()
	return strings.Join([]string{
		"Incident detail",
		"",
		"Title:       " + nonEmpty(alert.Title, "untitled"),
		"Severity:    " + strings.ToUpper(nonEmpty(alert.Severity, "unknown")),
		"Status:      " + nonEmpty(alert.Status, "unknown"),
		"Correlation: " + nonEmpty(alert.CorrelationID, "none"),
		"Fingerprint: " + nonEmpty(alert.Fingerprint, "none"),
		"Tenant:      " + nonEmpty(alert.Tenant, m.tenant),
		"",
		modeHint(m.mode),
	}, "\n")
}

func (m Model) selectedAlert() Alert {
	if len(m.alerts) == 0 {
		return Alert{}
	}
	cursor := m.table.Cursor()
	if cursor < 0 || cursor >= len(m.alerts) {
		return m.alerts[0]
	}
	return m.alerts[cursor]
}

func summaryStrip(alerts []Alert) string {
	counts := severityCounts(alerts)
	statuses := statusCounts(alerts)
	parts := []string{
		fmt.Sprintf("active=%d", len(alerts)),
		fmt.Sprintf("P1=%d", counts["P1"]),
		fmt.Sprintf("P2=%d", counts["P2"]),
		fmt.Sprintf("P3=%d", counts["P3"]),
		fmt.Sprintf("P4=%d", counts["P4"]),
	}
	for _, status := range sortedKeys(statuses) {
		parts = append(parts, fmt.Sprintf("%s=%d", status, statuses[status]))
	}
	return helpStyle.Render(strings.Join(parts, "  "))
}

func severityCounts(alerts []Alert) map[string]int {
	counts := map[string]int{"P1": 0, "P2": 0, "P3": 0, "P4": 0}
	for _, alert := range alerts {
		severity := strings.ToUpper(alert.Severity)
		if _, ok := counts[severity]; !ok {
			severity = "P4"
		}
		counts[severity]++
	}
	return counts
}

func statusCounts(alerts []Alert) map[string]int {
	counts := make(map[string]int)
	for _, alert := range alerts {
		status := strings.ToLower(strings.TrimSpace(alert.Status))
		if status == "" {
			status = "unknown"
		}
		counts[status]++
	}
	return counts
}

func sortedKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func modeHint(mode string) string {
	switch mode {
	case "tail":
		return "Tail mode: watch alert flow and correlation changes."
	case "runbooks", "runbook-search":
		return "Runbooks mode: search response procedures before approving actions."
	case "doctor":
		return "Doctor mode: verify API, auth, integration, and stream health."
	case "configure", "config-validate":
		return "Config mode: inspect local settings and validate paladin.yaml."
	default:
		return "Monitor mode: review active alerts and select incidents for detail."
	}
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func contentWidth(width int) int {
	if width < 60 {
		return 60
	}
	return width
}

func tableWidth(width int) int {
	if width < 100 {
		return 84
	}
	return width - detailWidth(width) - 6
}

func tableHeight(height int) int {
	if height < 16 {
		return 10
	}
	return minInt(height-12, 18)
}

func detailWidth(width int) int {
	if width < 100 {
		return 34
	}
	return minInt(42, maxInt(34, width/3))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
