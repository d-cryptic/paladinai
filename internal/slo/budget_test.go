package slo_test

import (
	"strings"
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/slo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var thirtyDays = 30 * 24 * time.Hour

func budget999() slo.Budget {
	return slo.Budget{Availability: 0.999, Period: thirtyDays}
}

// ─── ErrorBudget ─────────────────────────────────────────────────────────────

func TestErrorBudget_99_9(t *testing.T) {
	b := budget999()
	eb, err := b.ErrorBudget()
	require.NoError(t, err)
	// 0.001 × 30d = 43.2 minutes
	assert.InDelta(t, 43.2, eb.Minutes(), 0.1)
}

func TestErrorBudget_InvalidAvailability(t *testing.T) {
	_, err := slo.Budget{Availability: 0, Period: thirtyDays}.ErrorBudget()
	assert.Error(t, err)

	_, err = slo.Budget{Availability: 1, Period: thirtyDays}.ErrorBudget()
	assert.Error(t, err)
}

func TestErrorBudget_ZeroPeriod(t *testing.T) {
	_, err := slo.Budget{Availability: 0.99}.ErrorBudget()
	assert.Error(t, err)
}

// ─── AllowedErrorRate ─────────────────────────────────────────────────────────

func TestAllowedErrorRate(t *testing.T) {
	b := budget999()
	assert.InDelta(t, 0.001, b.AllowedErrorRate(), 1e-10)
}

// ─── ComputeBurnRate ──────────────────────────────────────────────────────────

func TestComputeBurnRate_SpecExample(t *testing.T) {
	// Spec example: SLO 99.9%, current error rate 2.3%, allowed 0.1%
	// burn rate = 2.3% / 0.1% = 23×
	b := budget999()
	remaining := 30 * time.Minute

	r, err := b.ComputeBurnRate(0.023, remaining, time.Hour, slo.DefaultPolicies)
	require.NoError(t, err)

	assert.InDelta(t, 23.0, r.BurnRate, 0.01, "burn rate should be 23×")
	assert.Equal(t, "P1", r.Severity, "23× > 14× burn rate should be P1")
	assert.Greater(t, r.TimeToExhaustion, time.Duration(0))
	// 30min remaining / 1.38 min-per-hour consumption = ~21.7h
	assert.InDelta(t, 21.7, r.TimeToExhaustion.Hours(), 0.5)
}

func TestComputeBurnRate_ZeroErrorRate(t *testing.T) {
	b := budget999()
	r, err := b.ComputeBurnRate(0, thirtyDays, time.Hour, slo.DefaultPolicies)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, r.BurnRate, 1e-10)
	assert.Equal(t, "", r.Severity, "zero burn rate should have no severity")
}

func TestComputeBurnRate_BurnRate6x_P2(t *testing.T) {
	b := budget999()
	// error rate = 6 × 0.001 = 0.006
	r, err := b.ComputeBurnRate(0.006, 20*time.Minute, 6*time.Hour, slo.DefaultPolicies)
	require.NoError(t, err)
	assert.InDelta(t, 6.0, r.BurnRate, 0.01)
	assert.Equal(t, "P2", r.Severity)
}

func TestComputeBurnRate_BurnRate3x_P3(t *testing.T) {
	b := budget999()
	r, err := b.ComputeBurnRate(0.003, 30*time.Minute, 24*time.Hour, slo.DefaultPolicies)
	require.NoError(t, err)
	assert.InDelta(t, 3.0, r.BurnRate, 0.01)
	assert.Equal(t, "P3", r.Severity)
}

func TestComputeBurnRate_BurnRate2x_NoPolicy(t *testing.T) {
	b := budget999()
	r, err := b.ComputeBurnRate(0.002, 30*time.Minute, 24*time.Hour, slo.DefaultPolicies)
	require.NoError(t, err)
	assert.InDelta(t, 2.0, r.BurnRate, 0.01)
	assert.Equal(t, "", r.Severity, "2× < 3× minimum threshold should not trigger any policy")
}

func TestComputeBurnRate_InvalidErrorRate(t *testing.T) {
	b := budget999()
	_, err := b.ComputeBurnRate(1.5, 0, time.Hour, nil)
	assert.Error(t, err)
}

func TestComputeBurnRate_InvalidAvailability(t *testing.T) {
	b := slo.Budget{Availability: 0, Period: thirtyDays}
	_, err := b.ComputeBurnRate(0.01, 0, time.Hour, nil)
	assert.Error(t, err)
}

func TestComputeBurnRate_FirstPolicyWins(t *testing.T) {
	// 14× should match P1 (first policy), not P2
	b := budget999()
	r, err := b.ComputeBurnRate(0.014, 5*time.Minute, time.Hour, slo.DefaultPolicies)
	require.NoError(t, err)
	assert.Equal(t, "P1", r.Severity)
}

func TestComputeBurnRate_ExactlyAtBoundary(t *testing.T) {
	b := budget999()
	// exactly 14× — should be P1
	r, err := b.ComputeBurnRate(0.014, 5*time.Minute, time.Hour, slo.DefaultPolicies)
	require.NoError(t, err)
	assert.Equal(t, "P1", r.Severity)
}

// ─── Summary ──────────────────────────────────────────────────────────────────

func TestSummary_ContainsKeyInfo(t *testing.T) {
	b := budget999()
	r, err := b.ComputeBurnRate(0.023, 30*time.Minute, time.Hour, slo.DefaultPolicies)
	require.NoError(t, err)

	s := slo.Summary(b, r)
	assert.Contains(t, s, "23")
	assert.Contains(t, s, "P1")
	assert.Contains(t, strings.ToLower(s), "burn rate")
}

func TestSummary_ZeroBurnRate(t *testing.T) {
	b := budget999()
	r, _ := b.ComputeBurnRate(0, thirtyDays, time.Hour, slo.DefaultPolicies)
	s := slo.Summary(b, r)
	assert.Contains(t, s, "0")
	assert.Contains(t, strings.ToLower(s), "healthy")
}
