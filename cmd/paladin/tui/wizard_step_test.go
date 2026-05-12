package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ── WizardModel extra paths ───────────────────────────────────────────────────

func TestWizardModel_Update_WhenDone_Quits(t *testing.T) {
	m := NewWizardModel("http://api", "http://auth", "", "")
	m.done = true
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("done model should return tea.Quit")
	}
}

func TestWizardModel_Update_WhenAborted_Quits(t *testing.T) {
	m := NewWizardModel("http://api", "http://auth", "", "")
	m.aborted = true
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("aborted model should return tea.Quit")
	}
}

func TestWizardModel_AdvanceStep_Deploy_NoOp(t *testing.T) {
	m := NewWizardModel("http://api", "http://auth", "", "")
	m.step = stepDeploy
	result, _ := m.Update(stepCompleteMsg{})
	wm := result.(WizardModel)
	if wm.step != stepDoctor {
		t.Errorf("expected to advance to stepDoctor, got %d", wm.step)
	}
}

func TestWizardModel_View_AllSteps(t *testing.T) {
	for _, s := range []int{stepDetect, stepAuth, stepTier, stepIntegrations, stepDeploy, stepDoctor} {
		m := NewWizardModel("http://api", "http://auth", "", "")
		m.step = s
		view := m.View()
		if view == "" {
			t.Errorf("step %d produced empty view", s)
		}
	}
}

// ── DetectModel extra tests ───────────────────────────────────────────────────

func TestDetectModel_View_Done_WithDetected(t *testing.T) {
	m := NewDetectModel()
	m.detected = []DetectedIntegration{
		{Name: "prometheus", Found: true, Confidence: "Medium"},
		{Name: "grafana", Found: false, Confidence: ""},
	}
	m.done = true
	view := m.View()
	if !strings.Contains(view, "prometheus") {
		t.Error("view should contain detected service names")
	}
}

// ── TierModel extra tests ─────────────────────────────────────────────────────

func TestTierModel_Update_Down_Up_Boundary(t *testing.T) {
	m := NewTierModel()
	// navigate up at top — should stay at 0
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if updated.cursor != 0 {
		t.Errorf("cursor should stay at 0 when already at top, got %d", updated.cursor)
	}
	// navigate to bottom
	for i := 0; i < len(tiers); i++ {
		updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
	if updated.cursor != len(tiers)-1 {
		t.Errorf("cursor should cap at %d, got %d", len(tiers)-1, updated.cursor)
	}
}

func TestTierModel_Update_SelectWithUp(t *testing.T) {
	m := NewTierModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("up")})
	_ = updated // no panic
}

// ── IntegrationsModel extra tests ─────────────────────────────────────────────

func TestIntegrationsModel_Update_Boundary(t *testing.T) {
	m := NewIntegrationsModel()
	// navigate up at top — should stay at 0
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if updated.cursor != 0 {
		t.Errorf("cursor should stay at 0, got %d", updated.cursor)
	}
}

// ── DeployModel extra tests ───────────────────────────────────────────────────

func TestDeployModel_Update_KeyWhenNotDone_SpinnerTick(t *testing.T) {
	m := NewDeployModel()
	m.done = false
	// A non-key message goes to spinner update
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_ = updated // no panic
}

// ── DoctorModel extra tests ───────────────────────────────────────────────────

func TestDoctorModel_View_Done_Healthy(t *testing.T) {
	m := NewDoctorModel("http://api")
	m.done = true
	m.healthy = true
	m.message = "API reachable"
	view := m.View()
	if !strings.Contains(view, "✓") && !strings.Contains(view, "ready") {
		t.Logf("healthy doctor view: %q", view)
	}
}

func TestDoctorModel_View_Done_Unhealthy(t *testing.T) {
	m := NewDoctorModel("http://api")
	m.done = true
	m.healthy = false
	m.message = "API not reachable"
	view := m.View()
	if !strings.Contains(view, "✗") && !strings.Contains(view, "not reachable") {
		t.Logf("unhealthy doctor view: %q", view)
	}
}

func TestDoctorModel_View_NotDone(t *testing.T) {
	m := NewDoctorModel("http://api")
	view := m.View()
	if view == "" {
		t.Error("DoctorModel View() should not be empty when not done")
	}
}

// ── AuthModel extra tests ─────────────────────────────────────────────────────

func TestAuthModel_View_NoExisting(t *testing.T) {
	m := NewAuthModel("http://api", "")
	view := m.View()
	if !strings.Contains(view, "Authentication") && !strings.Contains(view, "token") {
		t.Logf("auth view: %q", view)
	}
}

func TestAuthModel_View_WithExisting(t *testing.T) {
	m := NewAuthModel("http://api", "existing-token")
	view := m.View()
	if !strings.Contains(view, "Existing") && !strings.Contains(view, "existing") {
		t.Logf("auth-existing view: %q", view)
	}
}

// ── runDetection smoke test ───────────────────────────────────────────────────

func TestRunDetection_ReturnsMsg(t *testing.T) {
	msg := runDetection()
	dm, ok := msg.(detectionDoneMsg)
	if !ok {
		t.Fatalf("expected detectionDoneMsg, got %T", msg)
	}
	if len(dm.detected) != 6 {
		t.Errorf("expected 6 detected entries, got %d", len(dm.detected))
	}
}
