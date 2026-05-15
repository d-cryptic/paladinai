// Package tui provides Bubble Tea models for interactive CLI workflows.
// wizard.go implements the paladin init 6-step setup wizard.
package tui

import (
	"fmt"
	"strings"

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
	m.authModel = NewAuthModel(authEndpoint, existingToken)
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
		if m.result.Token != "" {
			m.step = stepTier
			return m, m.initCurrentStep()
		}
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
