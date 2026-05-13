// Package tui provides Bubble Tea models for interactive CLI workflows.
// wizard.go implements the paladin init 6-step setup wizard.
package tui

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Styles ─────────────────────────────────────────────────────────────────────

var (
	progressStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#25A065"))

	stepTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	checkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#25A065"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF4444"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	boldStyle = lipgloss.NewStyle().Bold(true)
)

// ── Step results ───────────────────────────────────────────────────────────────

// WizardResult holds the values collected across all wizard steps.
type WizardResult struct {
	APIEndpoint  string
	AuthEndpoint string
	Tenant       string
	Token        string
	Tier         string                // "pool" | "bridge" | "silo"
	Integrations []string              // names selected in step 4
	Detected     []DetectedIntegration // from environment detection
}

// DetectedIntegration is a service found during environment auto-detection.
type DetectedIntegration struct {
	Name       string
	Found      bool
	Confidence string // "High" | "Medium" | "Low" | ""
}

// ── Step indexes ───────────────────────────────────────────────────────────────

const (
	stepDetect       = 0
	stepAuth         = 1
	stepTier         = 2
	stepIntegrations = 3
	stepDeploy       = 4
	stepDoctor       = 5
	totalSteps       = 6
)

var stepNames = [totalSteps]string{
	"Detecting environment",
	"Authentication",
	"Select tier",
	"Enable integrations",
	"Deploy",
	"Verify",
}

// ── Main wizard model ──────────────────────────────────────────────────────────

// WizardModel is the top-level Bubble Tea model for the 6-step init wizard.
type WizardModel struct {
	step    int
	result  WizardResult
	done    bool
	aborted bool

	// Per-step sub-models.
	detectModel       DetectModel
	authModel         AuthModel
	tierModel         TierModel
	integrationsModel IntegrationsModel
	deployModel       DeployModel
	doctorModel       DoctorModel
}

// NewWizardModel creates the wizard with defaults pre-populated from cfg.
func NewWizardModel(apiEndpoint, authEndpoint, defaultTenant, existingToken string) WizardModel {
	m := WizardModel{
		step: stepDetect,
	}
	m.result = WizardResult{
		APIEndpoint:  apiEndpoint,
		AuthEndpoint: authEndpoint,
		Tenant:       defaultTenant,
		Token:        existingToken,
		Tier:         "pool",
	}
	m.detectModel = NewDetectModel()
	m.authModel = NewAuthModel(apiEndpoint, existingToken)
	m.tierModel = NewTierModel()
	m.integrationsModel = NewIntegrationsModel()
	m.deployModel = NewDeployModel()
	m.doctorModel = NewDoctorModel(apiEndpoint)
	return m
}

func (m WizardModel) Init() tea.Cmd {
	return m.detectModel.Init()
}

func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done || m.aborted {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.aborted = true
			return m, tea.Quit
		}
	case stepCompleteMsg:
		return m.advanceStep(msg)
	case tea.WindowSizeMsg:
		// propagate to all sub-models
	}

	return m.updateCurrentStep(msg)
}

// advanceStep carries step results forward and advances to the next step.
func (m WizardModel) advanceStep(msg stepCompleteMsg) (tea.Model, tea.Cmd) {
	switch m.step {
	case stepDetect:
		m.result.Detected = msg.detected
		// Pre-populate integrations from detected services.
		for _, d := range msg.detected {
			if d.Found {
				m.integrationsModel.preCheck(d.Name)
			}
		}
		m.integrationsModel = m.integrationsModel.withDetected(msg.detected)
	case stepAuth:
		if msg.token != "" {
			m.result.Token = msg.token
		}
	case stepTier:
		m.result.Tier = msg.tier
	case stepIntegrations:
		m.result.Integrations = msg.integrations
	case stepDeploy:
		// nothing extra to carry
	case stepDoctor:
		m.done = true
		return m, tea.Quit
	}

	m.step++
	if m.step >= totalSteps {
		m.done = true
		return m, tea.Quit
	}
	return m, m.initCurrentStep()
}

func (m WizardModel) initCurrentStep() tea.Cmd {
	switch m.step {
	case stepDetect:
		return m.detectModel.Init()
	case stepAuth:
		return m.authModel.Init()
	case stepTier:
		return m.tierModel.Init()
	case stepIntegrations:
		return m.integrationsModel.Init()
	case stepDeploy:
		return m.deployModel.Init()
	case stepDoctor:
		m.doctorModel = m.doctorModel.withEndpoint(m.result.APIEndpoint)
		return m.doctorModel.Init()
	}
	return nil
}

func (m WizardModel) updateCurrentStep(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.step {
	case stepDetect:
		m.detectModel, cmd = m.detectModel.Update(msg)
	case stepAuth:
		m.authModel, cmd = m.authModel.Update(msg)
	case stepTier:
		m.tierModel, cmd = m.tierModel.Update(msg)
	case stepIntegrations:
		m.integrationsModel, cmd = m.integrationsModel.Update(msg)
	case stepDeploy:
		m.deployModel, cmd = m.deployModel.Update(msg)
	case stepDoctor:
		m.doctorModel, cmd = m.doctorModel.Update(msg)
	}
	return m, cmd
}

func (m WizardModel) View() string {
	if m.aborted {
		return errorStyle.Render("\nSetup aborted. Run 'paladin init' to retry.\n")
	}
	if m.done {
		return checkStyle.Render("\nPaladinAI is ready! Run 'paladin doctor' to verify.\n")
	}

	var b strings.Builder
	b.WriteString(m.renderProgress())
	b.WriteString("\n")

	switch m.step {
	case stepDetect:
		b.WriteString(m.detectModel.View())
	case stepAuth:
		b.WriteString(m.authModel.View())
	case stepTier:
		b.WriteString(m.tierModel.View())
	case stepIntegrations:
		b.WriteString(m.integrationsModel.View())
	case stepDeploy:
		b.WriteString(m.deployModel.View())
	case stepDoctor:
		b.WriteString(m.doctorModel.View())
	}

	b.WriteString(dimStyle.Render("\n  Ctrl+C to abort\n"))
	return b.String()
}

func (m WizardModel) renderProgress() string {
	name := ""
	if m.step < totalSteps {
		name = stepNames[m.step]
	}
	return progressStyle.Render(fmt.Sprintf("[%d/%d] %s", m.step+1, totalSteps, name))
}

// Result returns the collected wizard result. Call after the program exits.
func (m WizardModel) Result() WizardResult { return m.result }

// Aborted returns true if the user quit early.
func (m WizardModel) Aborted() bool { return m.aborted }

// ── Step completion message ────────────────────────────────────────────────────

// stepCompleteMsg is sent by each step sub-model when the user advances.
type stepCompleteMsg struct {
	// Fields for each step; zero-values are ignored.
	detected     []DetectedIntegration
	token        string
	tier         string
	integrations []string
}

// ── Step 1: Environment detection ─────────────────────────────────────────────

// DetectModel runs environment auto-detection (kubectl, helm, env vars).
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

// runDetection probes the environment for known observability services.
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
		// We use exec to avoid importing os at package level just for getenv.
		out, err := exec.Command("sh", "-c", "echo $"+key).Output()
		if err != nil {
			return false, ""
		}
		if strings.TrimSpace(string(out)) != "" {
			return true, "Low"
		}
		return false, ""
	}
}

// ── Step 2: Authentication ─────────────────────────────────────────────────────

// AuthModel handles token input (simplified; full PKCE OAuth in Stage 11 proper).
type AuthModel struct {
	input       textinput.Model
	apiEndpoint string
	existing    string
}

func NewAuthModel(apiEndpoint, existing string) AuthModel {
	ti := textinput.New()
	ti.Placeholder = "paste token or press Enter to use existing"
	ti.EchoMode = textinput.EchoPassword
	ti.Focus()
	return AuthModel{input: ti, apiEndpoint: apiEndpoint, existing: existing}
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
	b.WriteString("  " + m.input.View() + "\n")
	return b.String()
}

// ── Step 3: Tier selection ─────────────────────────────────────────────────────

var tiers = []string{"pool", "bridge", "silo"}

// TierModel lets the user choose a deployment tier.
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
	for i, t := range tiers {
		cursor := "  "
		if i == m.cursor {
			cursor = "❯ "
		}
		name := strings.ToUpper(t[:1]) + t[1:]
		line := fmt.Sprintf("%s%s", cursor, boldStyle.Render(name))
		if i == m.cursor {
			line = checkStyle.Render(line)
		}
		b.WriteString("  " + line + "\n")
	}
	b.WriteString(dimStyle.Render("\n  ↑/↓ to navigate, Enter to select\n"))
	return b.String()
}

// ── Step 4: Integration enablement ────────────────────────────────────────────

type integrationItem struct {
	name    string
	checked bool
}

// IntegrationsModel shows detected + available integrations for selection.
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
	for i, n := range knownIntegrations {
		items[i] = integrationItem{name: n}
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
			var selected []string
			for _, it := range m.items {
				if it.checked {
					selected = append(selected, it.name)
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
	for i, it := range m.items {
		check := "[ ]"
		if it.checked {
			check = checkStyle.Render("[✓]")
		}
		cursor := "  "
		if i == m.cursor {
			cursor = "❯ "
		}
		fmt.Fprintf(&b, "  %s%s %s\n", cursor, check, it.name)
	}
	b.WriteString(dimStyle.Render("\n  Space to toggle, Enter to confirm\n"))
	return b.String()
}

// ── Step 5: Deploy ─────────────────────────────────────────────────────────────

// DeployModel shows deployment progress (simplified: just advances on Enter).
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

// ── Step 6: Doctor (inline health check) ──────────────────────────────────────

var doctorHTTPClient = &http.Client{Timeout: 5 * time.Second}

// DoctorModel runs a quick inline health check at the end of the wizard.
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
