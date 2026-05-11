// Package confidence provides composite confidence scoring for PaladinAI triage
// decisions, implementing the Stage 3.5 decision-math specification.
package confidence

import "math"

// Weight constants for the composite score. Must sum to 1.0.
const (
	weightLLM      = 0.40
	weightEvidence = 0.25
	weightHistory  = 0.25
	weightCoverage = 0.10
)

// Score carries the four input signals for composite scoring.
// Each field is expected in [0, 1]; values outside that range are clamped.
type Score struct {
	// LLMRaw is the raw confidence returned by the LLM classifier (0–1).
	LLMRaw float64

	// EvidenceCompleteness measures how many expected evidence items are present.
	EvidenceCompleteness float64

	// HistoricalAccuracy is the fraction of similar past alerts resolved correctly.
	HistoricalAccuracy float64

	// ToolCoverage is the fraction of recommended tools that were successfully run.
	ToolCoverage float64
}

// Composite returns a weighted composite confidence in [0, 1].
// The formula: 0.4·LLM + 0.25·evidence + 0.25·history + 0.10·coverage.
func Composite(s Score) float64 {
	raw := weightLLM*s.LLMRaw +
		weightEvidence*s.EvidenceCompleteness +
		weightHistory*s.HistoricalAccuracy +
		weightCoverage*s.ToolCoverage
	return clamp01(raw)
}

// TemperatureScale applies temperature scaling to a raw probability score.
// Temperature > 1 softens confidence (flattens toward 0.5).
// Temperature < 1 sharpens confidence (pushes toward 0 or 1).
// Temperature == 1 is the identity.
// The returned value is always in (0, 1).
func TemperatureScale(raw, temperature float64) float64 {
	if temperature <= 0 {
		temperature = 1
	}
	return sigmoid(logit(clamp01(raw)) / temperature)
}

// Gate returns true when score meets or exceeds threshold.
func Gate(score, threshold float64) bool {
	return score >= threshold
}

// GateLabel maps a score into a human-readable tier label.
//
//	>= 0.85 → HIGH
//	>= 0.65 → MEDIUM
//	>= 0.40 → LOW
//	<  0.40 → INSUFFICIENT
func GateLabel(score float64) string {
	switch {
	case score >= 0.85:
		return "HIGH"
	case score >= 0.65:
		return "MEDIUM"
	case score >= 0.40:
		return "LOW"
	default:
		return "INSUFFICIENT"
	}
}

// clamp01 restricts v to [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// logit is the log-odds transform: ln(p/(1-p)).
// Input is clamped to (ε, 1-ε) to avoid ±Inf.
func logit(p float64) float64 {
	const eps = 1e-7
	p = clamp01(p)
	if p < eps {
		p = eps
	}
	if p > 1-eps {
		p = 1 - eps
	}
	return math.Log(p / (1 - p))
}

// sigmoid is the inverse logit: 1/(1+e^(-x)).
func sigmoid(x float64) float64 {
	return 1 / (1 + math.Exp(-x))
}
