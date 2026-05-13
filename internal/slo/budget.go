// Package slo implements SLO error-budget mathematics from the Stage 3.5
// decision-math specification. It computes burn rates and time-to-exhaustion
// to automatically classify alert severity based on SLO impact.
package slo

import (
	"errors"
	"fmt"
	"time"
)

// WindowPolicy maps a burn-rate threshold to a severity label.
// Based on the Google SRE multi-window alerting model (Chapter 6).
type WindowPolicy struct {
	Window   time.Duration
	BurnRate float64 // minimum burn rate to trigger this window's alert
	Severity string  // P1, P2, P3
}

// DefaultPolicies are the three multi-window alert policies from the spec.
var DefaultPolicies = []WindowPolicy{
	{Window: 1 * time.Hour, BurnRate: 14, Severity: "P1"},
	{Window: 6 * time.Hour, BurnRate: 6, Severity: "P2"},
	{Window: 24 * time.Hour, BurnRate: 3, Severity: "P3"},
}

// Budget holds an SLO definition.
// Availability is the target availability fraction (e.g. 0.999 for 99.9%).
type Budget struct {
	// Availability is the target availability (0 < Availability < 1).
	Availability float64
	// Period is the rolling SLO measurement period (e.g. 30 days).
	Period time.Duration
}

// ErrorBudget returns the total error budget as a Duration.
// error_budget = (1 - availability) × period
func (b Budget) ErrorBudget() (time.Duration, error) {
	if b.Availability <= 0 || b.Availability >= 1 {
		return 0, errors.New("slo: Availability must be in (0, 1)")
	}
	if b.Period <= 0 {
		return 0, errors.New("slo: Period must be > 0")
	}
	return time.Duration(float64(b.Period) * (1 - b.Availability)), nil
}

// AllowedErrorRate returns the maximum tolerated error rate fraction.
// allowed = 1 - availability
func (b Budget) AllowedErrorRate() float64 {
	return 1 - b.Availability
}

// BurnRateResult describes the current error budget consumption.
type BurnRateResult struct {
	// BurnRate is how many times faster than allowed the budget is being consumed.
	// A burn rate of 1 means the budget will be exhausted exactly at period end.
	BurnRate float64

	// Consumed is how much budget has been consumed in the observation window.
	Consumed time.Duration

	// TimeToExhaustion is the estimated time until the budget runs out at the
	// current burn rate. Negative means budget is already exhausted.
	TimeToExhaustion time.Duration

	// RemainingBudget is the budget not yet consumed over the full period.
	RemainingBudget time.Duration

	// Severity is the highest-priority policy triggered at this burn rate,
	// based on the supplied policies. Empty string if no policy fires.
	Severity string
}

// ComputeBurnRate calculates the error budget burn rate for a given observation window.
//
// currentErrorRate is the observed error fraction in the observation window (0–1).
// remainingBudget is how much error budget is left in the current period.
// policies are evaluated in order; first match wins. Pass DefaultPolicies for standard behaviour.
func (b Budget) ComputeBurnRate(currentErrorRate float64, remainingBudget time.Duration, observationWindow time.Duration, policies []WindowPolicy) (BurnRateResult, error) {
	if b.Availability <= 0 || b.Availability >= 1 {
		return BurnRateResult{}, errors.New("slo: Availability must be in (0, 1)")
	}
	if b.Period <= 0 {
		return BurnRateResult{}, errors.New("slo: Period must be > 0")
	}
	if currentErrorRate < 0 || currentErrorRate > 1 {
		return BurnRateResult{}, errors.New("slo: currentErrorRate must be in [0, 1]")
	}

	allowed := b.AllowedErrorRate()

	var burnRate float64
	if allowed == 0 {
		return BurnRateResult{}, errors.New("slo: Availability=1 gives zero allowed error rate")
	}
	burnRate = currentErrorRate / allowed

	// Budget consumed in this observation window at the current error rate.
	// consumed = burnRate × (error_budget × window / period)
	totalBudget, _ := b.ErrorBudget()
	windowBudgetFraction := float64(observationWindow) / float64(b.Period)
	consumed := time.Duration(burnRate * windowBudgetFraction * float64(totalBudget))

	// Time to exhaustion = remainingBudget / rate_of_consumption_per_hour
	var timeToExhaustion time.Duration
	if burnRate > 0 {
		// rate per hour = burnRate × (total_budget / period_hours)
		budgetPerHour := float64(totalBudget) / b.Period.Hours()
		consumptionPerHour := burnRate * budgetPerHour
		if consumptionPerHour > 0 {
			timeToExhaustion = time.Duration(float64(remainingBudget) / consumptionPerHour * float64(time.Hour))
		}
	}

	const eps = 1e-9
	severity := ""
	for _, p := range policies {
		if burnRate >= p.BurnRate-eps {
			severity = p.Severity
			break
		}
	}

	return BurnRateResult{
		BurnRate:         burnRate,
		Consumed:         consumed,
		TimeToExhaustion: timeToExhaustion,
		RemainingBudget:  remainingBudget,
		Severity:         severity,
	}, nil
}

// Summary produces a human-readable triage summary for the burn rate result.
// Format matches the spec example output used in agent triage messages.
func Summary(b Budget, r BurnRateResult) string {
	if r.BurnRate == 0 {
		return "SLO burn rate: 0× — budget healthy"
	}

	totalBudget, _ := b.ErrorBudget()
	_ = totalBudget

	minutes := func(d time.Duration) float64 { return d.Minutes() }

	exhaustHours := r.TimeToExhaustion.Hours()
	sevLabel := r.Severity
	if sevLabel == "" {
		sevLabel = "OK"
	}

	return "SLO burn rate: " +
		formatFloat(r.BurnRate) + "× over observation window. " +
		"Error budget exhausted in ~" + formatFloat(exhaustHours) + "h at current rate. " +
		"Remaining budget: " + formatFloat(minutes(r.RemainingBudget)) + " of " +
		formatFloat(minutes(totalBudget)) + " minutes. " +
		"Classification: " + sevLabel
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.1f", f)
}
