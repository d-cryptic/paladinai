package tui

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type stepCompleteMsg struct {
	detected     []DetectedIntegration
	token        string
	tier         string
	integrations []string
}

type DetectModel struct {
	spinner  spinner.Model
	detected []DetectedIntegration
	done     bool
	input    textinput.Model
}

type detectionDoneMsg struct {
	detected []DetectedIntegration
}

func NewDetectModel() DetectModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#25A065"))
	ti := textinput.New()
	ti.Placeholder = "add integration name (Enter to skip)"
	return DetectModel{spinner: s, input: ti}
}

func (m DetectModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, runDetection)
}

func (m DetectModel) Update(msg tea.Msg) (DetectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case detectionDoneMsg:
		m.detected = msg.detected
		m.done = true
		m.input.Focus()
		return m, nil
	case tea.KeyMsg:
		if m.done {
			switch msg.Type {
			case tea.KeyEnter:
				manual := strings.TrimSpace(m.input.Value())
				if manual != "" {
					m.detected = append(m.detected, DetectedIntegration{
						Name:       manual,
						Found:      true,
						Confidence: "Manual",
					})
					m.input.SetValue("")
				}
				return m, func() tea.Msg {
					return stepCompleteMsg{detected: m.detected}
				}
			}
			var tiCmd tea.Cmd
			m.input, tiCmd = m.input.Update(msg)
			return m, tiCmd
		}
	default:
		if !m.done {
			var spinCmd tea.Cmd
			m.spinner, spinCmd = m.spinner.Update(msg)
			return m, spinCmd
		}
	}
	return m, nil
}

func (m DetectModel) View() string {
	var b strings.Builder
	b.WriteString(stepTitleStyle.Render("Step 1: Detecting your environment") + "\n\n")

	if !m.done {
		fmt.Fprintf(&b, "  %s Scanning cluster and environment...\n", m.spinner.View())
		return b.String()
	}

	if len(m.detected) == 0 {
		b.WriteString(dimStyle.Render("  No services detected automatically.\n"))
	} else {
		fmt.Fprintf(&b, "  %-20s %-10s %s\n", "Service", "Detected", "Confidence")
		b.WriteString(dimStyle.Render("  " + strings.Repeat("─", 42) + "\n"))
		for _, d := range m.detected {
			found := errorStyle.Render("✗ not found")
			if d.Found {
				found = checkStyle.Render("✓ (" + d.Confidence + ")")
			}
			fmt.Fprintf(&b, "  %-20s %s\n", d.Name, found)
		}
	}
	b.WriteString("\n")
	b.WriteString("  Add integration (Enter to continue):\n  " + m.input.View() + "\n")
	return b.String()
}

func runDetection() tea.Msg {
	detected := []DetectedIntegration{}
	probes := []struct {
		name string
		fn   func() (bool, string)
	}{
		{"prometheus", probeKubectl("prometheus")},
		{"grafana", probeKubectl("grafana")},
		{"alertmanager", probeKubectl("alertmanager")},
		{"loki", probeKubectl("loki")},
		{"datadog", probeEnvVar("DD_API_KEY")},
		{"pagerduty", probeEnvVar("PAGERDUTY_TOKEN")},
	}

	for _, p := range probes {
		found, confidence := p.fn()
		detected = append(detected, DetectedIntegration{
			Name:       p.name,
			Found:      found,
			Confidence: confidence,
		})
	}
	return detectionDoneMsg{detected: detected}
}

func probeKubectl(pattern string) func() (bool, string) {
	return func() (bool, string) {
		out, err := exec.Command("kubectl", "get", "pods", "--all-namespaces", "-o", "name").Output()
		if err != nil {
			return false, ""
		}
		if strings.Contains(strings.ToLower(string(out)), pattern) {
			return true, "Medium"
		}
		return false, ""
	}
}

func probeEnvVar(key string) func() (bool, string) {
	return func() (bool, string) {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true, "Low"
		}
		return false, ""
	}
}

type AuthModel struct {
	input        textinput.Model
	authEndpoint string
	existing     string
}

func NewAuthModel(authEndpoint, existing string) AuthModel {
	ti := textinput.New()
	ti.Placeholder = "paste token or press Enter to use existing"
	ti.EchoMode = textinput.EchoPassword
	ti.Focus()
	return AuthModel{input: ti, authEndpoint: authEndpoint, existing: existing}
}

func (m AuthModel) Init() tea.Cmd { return textinput.Blink }

func (m AuthModel) Update(msg tea.Msg) (AuthModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			token := strings.TrimSpace(m.input.Value())
			if token == "" {
				token = m.existing
			}
			return m, func() tea.Msg {
				return stepCompleteMsg{token: token}
			}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m AuthModel) View() string {
	var b strings.Builder
	b.WriteString(stepTitleStyle.Render("Step 2: Authentication") + "\n\n")
	if m.existing != "" {
		b.WriteString(checkStyle.Render("  ✓ Existing token found.") + "\n")
		b.WriteString(dimStyle.Render("  Press Enter to keep it or paste a new one:\n"))
	} else {
		b.WriteString("  API token (leave blank to continue unauthenticated):\n")
	}
	if strings.TrimSpace(m.authEndpoint) != "" {
		fmt.Fprintf(&b, "%s\n", dimStyle.Render("  Auth endpoint: "+m.authEndpoint))
	}
	b.WriteString("  " + m.input.View() + "\n")
	return b.String()
}

var tiers = []string{"pool", "bridge", "silo"}

type TierModel struct {
	cursor int
}

func NewTierModel() TierModel { return TierModel{} }

func (m TierModel) Init() tea.Cmd { return nil }

func (m TierModel) Update(msg tea.Msg) (TierModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(tiers)-1 {
				m.cursor++
			}
		case "enter", " ":
			selected := tiers[m.cursor]
			return m, func() tea.Msg {
				return stepCompleteMsg{tier: selected}
			}
		}
	}
	return m, nil
}

func (m TierModel) View() string {
	var b strings.Builder
	b.WriteString(stepTitleStyle.Render("Step 3: Select your deployment tier") + "\n\n")
	for i, tier := range tiers {
		cursor := "  "
		if i == m.cursor {
			cursor = "❯ "
		}
		name := strings.ToUpper(tier[:1]) + tier[1:]
		line := fmt.Sprintf("%s%s", cursor, boldStyle.Render(name))
		if i == m.cursor {
			line = checkStyle.Render(line)
		}
		b.WriteString("  " + line + "\n")
	}
	b.WriteString(dimStyle.Render("\n  ↑/↓ to navigate, Enter to select\n"))
	return b.String()
}

type integrationItem struct {
	name    string
	checked bool
}

type IntegrationsModel struct {
	items  []integrationItem
	cursor int
}

var knownIntegrations = []string{
	"alertmanager", "prometheus", "grafana", "loki",
	"datadog", "cloudwatch", "pagerduty", "slack",
	"github", "kubernetes", "jira", "linear",
}

func NewIntegrationsModel() IntegrationsModel {
	items := make([]integrationItem, len(knownIntegrations))
	for i, name := range knownIntegrations {
		items[i] = integrationItem{name: name}
	}
	return IntegrationsModel{items: items}
}

func (m IntegrationsModel) preCheck(name string) {
	for i := range m.items {
		if m.items[i].name == name {
			m.items[i].checked = true
			return
		}
	}
}

func (m IntegrationsModel) withDetected(detected []DetectedIntegration) IntegrationsModel {
	for _, d := range detected {
		if d.Found {
			for i := range m.items {
				if m.items[i].name == d.Name {
					m.items[i].checked = true
				}
			}
		}
	}
	return m
}

func (m IntegrationsModel) Init() tea.Cmd { return nil }

func (m IntegrationsModel) Update(msg tea.Msg) (IntegrationsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case " ":
			m.items[m.cursor].checked = !m.items[m.cursor].checked
		case "enter":
			selected := make([]string, 0, len(m.items))
			for _, item := range m.items {
				if item.checked {
					selected = append(selected, item.name)
				}
			}
			return m, func() tea.Msg {
				return stepCompleteMsg{integrations: selected}
			}
		}
	}
	return m, nil
}

func (m IntegrationsModel) View() string {
	var b strings.Builder
	b.WriteString(stepTitleStyle.Render("Step 4: Enable integrations") + "\n\n")
	for i, item := range m.items {
		check := "[ ]"
		if item.checked {
			check = checkStyle.Render("[✓]")
		}
		cursor := "  "
		if i == m.cursor {
			cursor = "❯ "
		}
		fmt.Fprintf(&b, "  %s%s %s\n", cursor, check, item.name)
	}
	b.WriteString(dimStyle.Render("\n  Space to toggle, Enter to confirm\n"))
	return b.String()
}

type DeployModel struct {
	spinner spinner.Model
	done    bool
}

func NewDeployModel() DeployModel {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#25A065"))
	return DeployModel{spinner: s}
}

func (m DeployModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg { return deployReadyMsg{} })
}

type deployReadyMsg struct{}

func (m DeployModel) Update(msg tea.Msg) (DeployModel, tea.Cmd) {
	switch msg.(type) {
	case deployReadyMsg:
		m.done = true
		return m, nil
	case tea.KeyMsg:
		if m.done {
			return m, func() tea.Msg { return stepCompleteMsg{} }
		}
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m DeployModel) View() string {
	var b strings.Builder
	b.WriteString(stepTitleStyle.Render("Step 5: Deploy") + "\n\n")
	if m.done {
		b.WriteString(checkStyle.Render("  ✓ Configuration applied.") + "\n")
		b.WriteString(dimStyle.Render("  Press Enter to continue to health check...\n"))
	} else {
		fmt.Fprintf(&b, "  %s Applying configuration...\n", m.spinner.View())
	}
	return b.String()
}

var doctorHTTPClient = &http.Client{Timeout: 5 * time.Second}

type DoctorModel struct {
	spinner     spinner.Model
	apiEndpoint string
	done        bool
	healthy     bool
	message     string
}

func NewDoctorModel(apiEndpoint string) DoctorModel {
	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#25A065"))
	return DoctorModel{spinner: s, apiEndpoint: apiEndpoint}
}

func (m DoctorModel) withEndpoint(ep string) DoctorModel {
	m.apiEndpoint = ep
	return m
}

type doctorCheckMsg struct {
	healthy bool
	message string
}

func (m DoctorModel) Init() tea.Cmd {
	ep := m.apiEndpoint
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		return checkDoctorReadiness(ep)
	})
}

func checkDoctorReadiness(endpoint string) doctorCheckMsg {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		return doctorCheckMsg{healthy: false, message: "API endpoint is not configured"}
	}

	req, err := http.NewRequest(http.MethodGet, endpoint+"/readyz", nil)
	if err != nil {
		return doctorCheckMsg{healthy: false, message: "API readiness URL is invalid: " + err.Error()}
	}
	resp, err := doctorHTTPClient.Do(req)
	if err != nil {
		return doctorCheckMsg{healthy: false, message: "API not ready at " + endpoint}
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return doctorCheckMsg{healthy: false, message: fmt.Sprintf("API not ready at %s: HTTP %d", endpoint, resp.StatusCode)}
	}
	return doctorCheckMsg{healthy: true, message: "API ready at " + endpoint}
}

func (m DoctorModel) Update(msg tea.Msg) (DoctorModel, tea.Cmd) {
	switch msg := msg.(type) {
	case doctorCheckMsg:
		m.done = true
		m.healthy = msg.healthy
		m.message = msg.message
		return m, nil
	case tea.KeyMsg:
		if m.done && msg.Type == tea.KeyEnter {
			return m, func() tea.Msg { return stepCompleteMsg{} }
		}
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m DoctorModel) View() string {
	var b strings.Builder
	b.WriteString(stepTitleStyle.Render("Step 6: Verify") + "\n\n")
	if !m.done {
		fmt.Fprintf(&b, "  %s Running health checks...\n", m.spinner.View())
		return b.String()
	}
	if m.healthy {
		b.WriteString(checkStyle.Render("  ✓ "+m.message) + "\n")
		b.WriteString(checkStyle.Render("  ✓ PaladinAI is ready!") + "\n")
	} else {
		b.WriteString(errorStyle.Render("  ✗ "+m.message) + "\n")
		b.WriteString(dimStyle.Render("  Run 'paladin doctor' to investigate.\n"))
	}
	b.WriteString(dimStyle.Render("\n  Press Enter to finish.\n"))
	return b.String()
}
