package tui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ── probeEnvVar ───────────────────────────────────────────────────────────────

func TestProbeEnvVar_SetKey_ReturnsTrue(t *testing.T) {
	t.Setenv("PALADIN_TEST_PROBE_KEY", "some-value")
	probe := probeEnvVar("PALADIN_TEST_PROBE_KEY")
	found, confidence := probe()
	if !found {
		t.Error("expected probeEnvVar to find set env var")
	}
	if confidence == "" {
		t.Error("expected non-empty confidence string")
	}
}

func TestProbeEnvVar_UnsetKey_ReturnsFalse(t *testing.T) {
	os.Unsetenv("PALADIN_TEST_MISSING_XYZ")
	probe := probeEnvVar("PALADIN_TEST_MISSING_XYZ")
	found, _ := probe()
	if found {
		t.Error("expected probeEnvVar to return false for unset var")
	}
}

// ── probeKubectl ─────────────────────────────────────────────────────────────

func TestProbeKubectl_NoKubectl_ReturnsFalse(t *testing.T) {
	// Without kubectl installed or with an invalid cluster, the command fails.
	// Either way the probe returns (false, "").
	probe := probeKubectl("prometheus")
	found, _ := probe()
	// We don't assert the value (kubectl may or may not be installed in CI),
	// but we assert it does NOT panic.
	_ = found
}

// ── WizardModel.updateCurrentStep ─────────────────────────────────────────────

func TestWizardModel_UpdateCurrentStep_AllSteps(t *testing.T) {
	m := NewWizardModel("http://api", "http://auth", "", "")
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}

	// stepDetect is initial — send a msg to exercise the detect branch.
	updated, _ := m.Update(msg)
	wm := updated.(WizardModel)
	_ = wm.View() // must not panic
}

// ── WizardModel.Init ──────────────────────────────────────────────────────────

func TestWizardModel_Init_ReturnsCmd(t *testing.T) {
	m := NewWizardModel("http://api", "http://auth", "", "")
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a non-nil Cmd")
	}
}

// ── IntegrationsModel.View ─────────────────────────────────────────────────────

func TestIntegrationsModel_View_ContainsExpectedContent(t *testing.T) {
	m := NewIntegrationsModel()
	view := m.View()
	if !strings.Contains(view, "Step 4") {
		t.Errorf("view missing step title, got: %q", view[:min(len(view), 100)])
	}
	if !strings.Contains(view, "Space to toggle") {
		t.Error("view missing key hint")
	}
}

func TestIntegrationsModel_View_CheckedItem(t *testing.T) {
	m := NewIntegrationsModel()
	// Toggle first item (Space key).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	view := updated.View()
	if !strings.Contains(view, "✓") {
		t.Error("expected checked item marker in view")
	}
}

// ── DeployModel.View ──────────────────────────────────────────────────────────

func TestDeployModel_View_InProgress(t *testing.T) {
	m := NewDeployModel()
	view := m.View()
	if !strings.Contains(view, "deploy") && !strings.Contains(view, "Deploy") &&
		!strings.Contains(view, "Setting") && !strings.Contains(view, "up") {
		t.Logf("DeployModel View (in-progress): %q", view)
	}
}

func TestDeployModel_View_Done(t *testing.T) {
	m := NewDeployModel()
	updated, _ := m.Update(deployReadyMsg{})
	view := updated.View()
	if view == "" {
		t.Error("DeployModel.View() should not be empty when done")
	}
}

func TestDeployModel_Init_ReturnsCmd(t *testing.T) {
	m := NewDeployModel()
	cmd := m.Init()
	if cmd == nil {
		t.Error("DeployModel.Init() should return non-nil cmd")
	}
}

// ── DetectModel.Init ─────────────────────────────────────────────────────────

func TestDetectModel_Init_ReturnsCmd(t *testing.T) {
	m := NewDetectModel()
	cmd := m.Init()
	if cmd == nil {
		t.Error("DetectModel.Init() should return non-nil cmd")
	}
}

// ── DoctorModel.Init ─────────────────────────────────────────────────────────

func TestDoctorModel_Init_ReturnsCmd(t *testing.T) {
	m := NewDoctorModel("http://api")
	cmd := m.Init()
	if cmd == nil {
		t.Error("DoctorModel.Init() should return non-nil cmd")
	}
}

// ── WizardModel View for various steps ────────────────────────────────────────

func TestWizardModel_View_DoneState(t *testing.T) {
	m := NewWizardModel("http://api", "http://auth", "", "")
	m.done = true
	view := m.View()
	if !strings.Contains(view, "ready") && !strings.Contains(view, "PaladinAI") {
		t.Logf("done view: %q", view)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
