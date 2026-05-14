package tui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ── WizardModel ───────────────────────────────────────────────────────────────

func TestNewWizardModel_Defaults(t *testing.T) {
	m := NewWizardModel("http://api", "http://auth", "tenant-1", "tok")
	if m.step != stepDetect {
		t.Errorf("initial step = %d, want %d", m.step, stepDetect)
	}
	res := m.Result()
	if res.APIEndpoint != "http://api" {
		t.Errorf("APIEndpoint = %q, want %q", res.APIEndpoint, "http://api")
	}
	if res.Tier != "pool" {
		t.Errorf("default Tier = %q, want %q", res.Tier, "pool")
	}
	if m.Aborted() {
		t.Error("fresh wizard should not be aborted")
	}
}

func TestWizardModel_CtrlC_Aborts(t *testing.T) {
	m := NewWizardModel("http://api", "http://auth", "", "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	wm := updated.(WizardModel)
	if !wm.Aborted() {
		t.Error("Ctrl+C should set aborted=true")
	}
}

func TestWizardModel_AbortedView(t *testing.T) {
	m := NewWizardModel("http://api", "http://auth", "", "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	view := updated.(WizardModel).View()
	if view == "" {
		t.Error("aborted view should not be empty")
	}
}

func TestWizardModel_RenderProgress(t *testing.T) {
	m := NewWizardModel("", "", "", "")
	m.step = 2
	out := m.renderProgress()
	if out == "" {
		t.Error("renderProgress should return non-empty string")
	}
}

func TestWizardModel_AdvanceStep_Detect(t *testing.T) {
	m := NewWizardModel("http://api", "http://auth", "t1", "")
	detected := []DetectedIntegration{
		{Name: "prometheus", Found: true, Confidence: "Medium"},
	}
	msg := stepCompleteMsg{detected: detected}
	updated, _ := m.advanceStep(msg)
	wm := updated.(WizardModel)
	if wm.step != stepAuth {
		t.Errorf("after detect step, step = %d, want %d", wm.step, stepAuth)
	}
	if len(wm.result.Detected) != 1 {
		t.Errorf("detected len = %d, want 1", len(wm.result.Detected))
	}
}

func TestWizardModel_AdvanceStep_DetectSkipsAuthWithExistingToken(t *testing.T) {
	m := NewWizardModel("http://api", "http://auth", "t1", "tok")

	updated, _ := m.advanceStep(stepCompleteMsg{detected: []DetectedIntegration{
		{Name: "prometheus", Found: true, Confidence: "Medium"},
	}})
	wm := updated.(WizardModel)

	if wm.step != stepTier {
		t.Errorf("after detect with existing token, step = %d, want %d", wm.step, stepTier)
	}
	if wm.result.Token != "tok" {
		t.Errorf("token = %q, want %q", wm.result.Token, "tok")
	}
}

func TestNewWizardModel_AuthModelUsesAuthEndpoint(t *testing.T) {
	m := NewWizardModel("http://api", "http://auth", "t1", "")
	if m.authModel.authEndpoint != "http://auth" {
		t.Errorf("auth endpoint = %q, want %q", m.authModel.authEndpoint, "http://auth")
	}
}

func TestWizardModel_AdvanceStep_Auth(t *testing.T) {
	m := NewWizardModel("http://api", "http://auth", "t1", "")
	m.step = stepAuth
	msg := stepCompleteMsg{token: "new-token"}
	updated, _ := m.advanceStep(msg)
	wm := updated.(WizardModel)
	if wm.result.Token != "new-token" {
		t.Errorf("token = %q, want %q", wm.result.Token, "new-token")
	}
	if wm.step != stepTier {
		t.Errorf("after auth step, step = %d, want %d", wm.step, stepTier)
	}
}

func TestWizardModel_AdvanceStep_Tier(t *testing.T) {
	m := NewWizardModel("", "", "", "")
	m.step = stepTier
	msg := stepCompleteMsg{tier: "silo"}
	updated, _ := m.advanceStep(msg)
	wm := updated.(WizardModel)
	if wm.result.Tier != "silo" {
		t.Errorf("tier = %q, want %q", wm.result.Tier, "silo")
	}
	if wm.step != stepIntegrations {
		t.Errorf("after tier step, step = %d, want %d", wm.step, stepIntegrations)
	}
}

func TestWizardModel_AdvanceStep_Integrations(t *testing.T) {
	m := NewWizardModel("", "", "", "")
	m.step = stepIntegrations
	msg := stepCompleteMsg{integrations: []string{"slack", "github"}}
	updated, _ := m.advanceStep(msg)
	wm := updated.(WizardModel)
	if len(wm.result.Integrations) != 2 {
		t.Errorf("integrations len = %d, want 2", len(wm.result.Integrations))
	}
	if wm.step != stepDeploy {
		t.Errorf("after integrations step, step = %d, want %d", wm.step, stepDeploy)
	}
}

func TestWizardModel_AdvanceStep_Doctor_Quits(t *testing.T) {
	m := NewWizardModel("", "", "", "")
	m.step = stepDoctor
	updated, cmd := m.advanceStep(stepCompleteMsg{})
	wm := updated.(WizardModel)
	if !wm.done {
		t.Error("after doctor step, wizard should be done")
	}
	if cmd == nil {
		t.Error("should return tea.Quit cmd")
	}
}

// ── DetectModel ───────────────────────────────────────────────────────────────

func TestDetectModel_DoneOnDetectionMsg(t *testing.T) {
	m := NewDetectModel()
	detected := []DetectedIntegration{{Name: "grafana", Found: true, Confidence: "High"}}
	updated, _ := m.Update(detectionDoneMsg{detected: detected})
	if !updated.done {
		t.Error("DetectModel should be done after detectionDoneMsg")
	}
	if len(updated.detected) != 1 {
		t.Errorf("detected len = %d, want 1", len(updated.detected))
	}
}

func TestDetectModel_View_BeforeDone(t *testing.T) {
	m := NewDetectModel()
	view := m.View()
	if view == "" {
		t.Error("DetectModel.View() should not be empty before done")
	}
}

func TestDetectModel_View_AfterDone_Empty(t *testing.T) {
	m := NewDetectModel()
	m.done = true
	m.detected = nil
	view := m.View()
	if view == "" {
		t.Error("DetectModel.View() should not be empty after done with no detections")
	}
}

// ── TierModel ─────────────────────────────────────────────────────────────────

func TestTierModel_Navigation(t *testing.T) {
	m := NewTierModel()
	if m.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", m.cursor)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if updated.cursor != 1 {
		t.Errorf("after 'j', cursor = %d, want 1", updated.cursor)
	}
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if updated.cursor != 0 {
		t.Errorf("after 'k', cursor = %d, want 0", updated.cursor)
	}
}

func TestTierModel_SelectEmitsMsg(t *testing.T) {
	m := NewTierModel()
	m.cursor = 2 // "silo"
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should emit a command")
	}
	msg := cmd()
	scm, ok := msg.(stepCompleteMsg)
	if !ok {
		t.Fatalf("expected stepCompleteMsg, got %T", msg)
	}
	if scm.tier != "silo" {
		t.Errorf("tier = %q, want %q", scm.tier, "silo")
	}
}

func TestTierModel_View(t *testing.T) {
	m := NewTierModel()
	view := m.View()
	if view == "" {
		t.Error("TierModel.View() should not be empty")
	}
}

// ── IntegrationsModel ─────────────────────────────────────────────────────────

func TestIntegrationsModel_Toggle(t *testing.T) {
	m := NewIntegrationsModel()
	// toggle first item
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if !updated.items[0].checked {
		t.Error("space should toggle first item to checked")
	}
	// toggle again
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if updated.items[0].checked {
		t.Error("second space should uncheck first item")
	}
}

func TestIntegrationsModel_WithDetected_PreChecks(t *testing.T) {
	m := NewIntegrationsModel()
	detected := []DetectedIntegration{
		{Name: "prometheus", Found: true, Confidence: "Medium"},
		{Name: "datadog", Found: false, Confidence: ""},
	}
	m = m.withDetected(detected)
	for _, it := range m.items {
		if it.name == "prometheus" && !it.checked {
			t.Error("prometheus should be pre-checked (Found=true)")
		}
		if it.name == "datadog" && it.checked {
			t.Error("datadog should NOT be pre-checked (Found=false)")
		}
	}
}

func TestIntegrationsModel_EnterEmitsIntegrations(t *testing.T) {
	m := NewIntegrationsModel()
	// check prometheus (index depends on knownIntegrations order — find it)
	for i, it := range m.items {
		if it.name == "prometheus" {
			m.cursor = i
			break
		}
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should emit a command")
	}
	msg := cmd()
	scm, ok := msg.(stepCompleteMsg)
	if !ok {
		t.Fatalf("expected stepCompleteMsg, got %T", msg)
	}
	if len(scm.integrations) != 1 || scm.integrations[0] != "prometheus" {
		t.Errorf("integrations = %v, want [prometheus]", scm.integrations)
	}
}

// ── AuthModel ─────────────────────────────────────────────────────────────────

func TestAuthModel_EnterWithEmptyUsesExisting(t *testing.T) {
	m := NewAuthModel("http://api", "existing-token")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should emit a command")
	}
	msg := cmd()
	scm, ok := msg.(stepCompleteMsg)
	if !ok {
		t.Fatalf("expected stepCompleteMsg, got %T", msg)
	}
	if scm.token != "existing-token" {
		t.Errorf("token = %q, want %q", scm.token, "existing-token")
	}
}

func TestAuthModel_View(t *testing.T) {
	m := NewAuthModel("http://api", "tok")
	view := m.View()
	if view == "" {
		t.Error("AuthModel.View() should not be empty")
	}
}

// ── DeployModel ───────────────────────────────────────────────────────────────

func TestDeployModel_ReadyThenEnter(t *testing.T) {
	m := NewDeployModel()
	// Simulate deployReadyMsg
	updated, _ := m.Update(deployReadyMsg{})
	if !updated.done {
		t.Error("DeployModel should be done after deployReadyMsg")
	}
	_, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter after ready should emit a command")
	}
	msg := cmd()
	if _, ok := msg.(stepCompleteMsg); !ok {
		t.Errorf("expected stepCompleteMsg, got %T", msg)
	}
}

func TestCheckDoctorReadiness_UsesReadyz(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	msg := checkDoctorReadiness(srv.URL + "/")
	if !msg.healthy {
		t.Fatalf("healthy = false, message = %q", msg.message)
	}
	if gotPath != "/readyz" {
		t.Fatalf("path = %q, want /readyz", gotPath)
	}
	if !strings.Contains(msg.message, "API ready") {
		t.Fatalf("message = %q, want ready message", msg.message)
	}
}

func TestCheckDoctorReadiness_NonOKFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	msg := checkDoctorReadiness(srv.URL)
	if msg.healthy {
		t.Fatal("healthy = true, want false")
	}
	if !strings.Contains(msg.message, "HTTP 503") {
		t.Fatalf("message = %q, want status detail", msg.message)
	}
}

// ── DoctorModel ───────────────────────────────────────────────────────────────

func TestDoctorModel_HealthyMsg(t *testing.T) {
	m := NewDoctorModel("http://api")
	updated, _ := m.Update(doctorCheckMsg{healthy: true, message: "ok"})
	if !updated.done {
		t.Error("DoctorModel should be done after doctorCheckMsg")
	}
	if !updated.healthy {
		t.Error("DoctorModel should be healthy=true")
	}
}

func TestDoctorModel_UnhealthyMsg(t *testing.T) {
	m := NewDoctorModel("http://api")
	updated, _ := m.Update(doctorCheckMsg{healthy: false, message: "timeout"})
	if updated.healthy {
		t.Error("DoctorModel should be healthy=false after unhealthy msg")
	}
	view := updated.View()
	if view == "" {
		t.Error("View() should not be empty after unhealthy check")
	}
}

func TestDoctorModel_WithEndpoint(t *testing.T) {
	m := NewDoctorModel("")
	m = m.withEndpoint("http://new-api")
	if m.apiEndpoint != "http://new-api" {
		t.Errorf("apiEndpoint = %q, want %q", m.apiEndpoint, "http://new-api")
	}
}

func TestDoctorModel_EnterAfterDoneAdvances(t *testing.T) {
	m := NewDoctorModel("http://api")
	m, _ = m.Update(doctorCheckMsg{healthy: true, message: "ok"})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter after done should emit a command")
	}
	msg := cmd()
	if _, ok := msg.(stepCompleteMsg); !ok {
		t.Errorf("expected stepCompleteMsg, got %T", msg)
	}
}
