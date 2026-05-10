package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/paladinai/paladinai/cmd/paladin/tui"
	"github.com/stretchr/testify/assert"
)

func sampleAlerts() []tui.Alert {
	return []tui.Alert{
		{Fingerprint: "fp1", Severity: "P1", Status: "firing", Title: "DB is down", CorrelationID: "corr-aabbccdd", Tenant: "t1"},
		{Fingerprint: "fp2", Severity: "P3", Status: "firing", Title: "High latency", CorrelationID: "corr-11223344", Tenant: "t1"},
	}
}

func TestModel_InitDoesNotPanic(t *testing.T) {
	m := tui.New("test-tenant")
	cmd := m.Init()
	assert.Nil(t, cmd, "Init should return nil cmd for base model")
}

func TestModel_SetAlerts(t *testing.T) {
	m := tui.New("test-tenant")
	m = m.SetAlerts(sampleAlerts())

	view := m.View()
	assert.Contains(t, view, "test-tenant")
	assert.Contains(t, view, "P1")
	assert.Contains(t, view, "DB is down")
}

func TestModel_QuitKey(t *testing.T) {
	m := tui.New("test-tenant")
	m = m.SetAlerts(sampleAlerts())

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	_ = updated
	assert.NotNil(t, cmd, "q should return a quit command")
}

func TestModel_ViewContainsTenant(t *testing.T) {
	m := tui.New("acme-corp")
	view := m.View()
	assert.Contains(t, view, "acme-corp")
}

func TestModel_AlertsLoadedMsg(t *testing.T) {
	m := tui.New("t1")
	updated, _ := m.Update(tui.AlertsLoadedMsg(sampleAlerts()))
	view := updated.(tui.Model).View()
	assert.Contains(t, view, "P1")
}

func TestModel_ErrorDisplayed(t *testing.T) {
	m := tui.New("t1")
	updated, _ := m.Update(tui.ErrMsg{Err: assert.AnError})
	view := updated.(tui.Model).View()
	assert.True(t, strings.Contains(view, "Error") || strings.Contains(view, "error"),
		"error message should appear in view")
}

func TestModel_EmptyAlerts(t *testing.T) {
	m := tui.New("empty-tenant")
	m = m.SetAlerts(nil)
	view := m.View()
	assert.Contains(t, view, "empty-tenant", "tenant header should still show with no alerts")
}
