// Package tui implements the Bubble Tea interactive dashboard for paladin.
// The dashboard shows active alerts in real-time and allows keyboard navigation.
package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

	m := Model{table: t, tenant: tenant, mode: "monitor"}
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
			m.recent = append([]string{command}, m.recent...)
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
		m.mode = "configure"
	case "incidents":
		m.mode = "monitor"
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

func isDashboardMode(mode string) bool {
	switch mode {
	case "monitor", "tail", "configure":
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
	header := titleStyle.Render(fmt.Sprintf(" PaladinAI Dashboard — tenant: %s • mode: %s ", m.tenant, m.mode))
	help := helpStyle.Render("↑/↓ navigate  •  / commands  •  ? help  •  r refresh  •  q quit")

	body := baseStyle.Render(m.table.View())

	errLine := ""
	if m.err != nil {
		errLine = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(
			fmt.Sprintf("Error: %v", m.err))
	}

	parts := []string{header, body}
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
	return helpStyle.Render("/incidents  /tail  /config  /help  /quit") + "\n> " + input
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
		"  /config     configure mode",
		"  /quit       exit",
	}, "\n"))
}
