// Package tui implements the Bubble Tea interactive dashboard for paladin.
// The dashboard shows active alerts in real-time and allows keyboard navigation.
package tui

import (
	"fmt"
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

// Model is the Bubble Tea model for the alert dashboard.
type Model struct {
	table  table.Model
	alerts []Alert
	tenant string
	err    error
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

	return Model{table: t, tenant: tenant}
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
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			return m, nil // caller can wire a refresh command
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

// View renders the dashboard.
func (m Model) View() string {
	header := titleStyle.Render(fmt.Sprintf(" PaladinAI Dashboard — tenant: %s ", m.tenant))
	help := helpStyle.Render("↑/↓ navigate  •  r refresh  •  q quit")

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
	parts = append(parts, help)
	return strings.Join(parts, "\n")
}
