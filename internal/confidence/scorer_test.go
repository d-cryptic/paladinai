package confidence_test

import (
	"math"
	"testing"

	"github.com/paladinai/paladinai/internal/confidence"
	"github.com/stretchr/testify/assert"
)

func TestComposite_AllSignals(t *testing.T) {
	s := confidence.Score{LLMRaw: 0.9, EvidenceCompleteness: 0.8, HistoricalAccuracy: 0.7, ToolCoverage: 1.0}
	got := confidence.Composite(s)
	// 0.4*0.9 + 0.25*0.8 + 0.25*0.7 + 0.10*1.0 = 0.36+0.20+0.175+0.10 = 0.835
	assert.InDelta(t, 0.835, got, 1e-9)
}

func TestComposite_ZeroSignals(t *testing.T) {
	assert.Equal(t, 0.0, confidence.Composite(confidence.Score{}))
}

func TestComposite_PerfectSignals(t *testing.T) {
	s := confidence.Score{LLMRaw: 1, EvidenceCompleteness: 1, HistoricalAccuracy: 1, ToolCoverage: 1}
	assert.Equal(t, 1.0, confidence.Composite(s))
}

func TestComposite_ClampsAboveOne(t *testing.T) {
	s := confidence.Score{LLMRaw: 2, EvidenceCompleteness: 2, HistoricalAccuracy: 2, ToolCoverage: 2}
	assert.Equal(t, 1.0, confidence.Composite(s))
}

func TestComposite_ClampsBelowZero(t *testing.T) {
	s := confidence.Score{LLMRaw: -1, EvidenceCompleteness: -1, HistoricalAccuracy: -1, ToolCoverage: -1}
	assert.Equal(t, 0.0, confidence.Composite(s))
}

func TestComposite_UniformativePrior(t *testing.T) {
	// All signals at 0.5 → composite = 0.5
	s := confidence.Score{LLMRaw: 0.5, EvidenceCompleteness: 0.5, HistoricalAccuracy: 0.5, ToolCoverage: 0.5}
	assert.InDelta(t, 0.5, confidence.Composite(s), 1e-9)
}

func TestTemperatureScale_Identity(t *testing.T) {
	// T=1 is the identity
	for _, raw := range []float64{0.1, 0.3, 0.5, 0.7, 0.9} {
		got := confidence.TemperatureScale(raw, 1.0)
		assert.InDelta(t, raw, got, 1e-6, "raw=%v", raw)
	}
}

func TestTemperatureScale_HighTempSoftens(t *testing.T) {
	// T>1 pulls values toward 0.5
	high := confidence.TemperatureScale(0.9, 2.0)
	assert.Less(t, high, 0.9)
	assert.Greater(t, high, 0.5)
}

func TestTemperatureScale_LowTempSharpens(t *testing.T) {
	// T<1 pushes values away from 0.5
	sharp := confidence.TemperatureScale(0.9, 0.5)
	assert.Greater(t, sharp, 0.9)
}

func TestTemperatureScale_ZeroTemperatureFallsBack(t *testing.T) {
	// T<=0 treated as 1 (identity)
	got := confidence.TemperatureScale(0.7, 0)
	assert.InDelta(t, 0.7, got, 1e-6)
}

func TestTemperatureScale_BoundaryInputs(t *testing.T) {
	// Should not produce NaN/Inf for edge-case inputs
	for _, raw := range []float64{0, 1, 1e-8, 1 - 1e-8} {
		got := confidence.TemperatureScale(raw, 1.0)
		assert.False(t, math.IsNaN(got), "NaN for raw=%v", raw)
		assert.False(t, math.IsInf(got, 0), "Inf for raw=%v", raw)
	}
}

func TestGate_PassAndBlock(t *testing.T) {
	assert.True(t, confidence.Gate(0.7, 0.65))
	assert.True(t, confidence.Gate(0.65, 0.65)) // exact boundary passes
	assert.False(t, confidence.Gate(0.64, 0.65))
	assert.False(t, confidence.Gate(0.0, 0.01))
}

func TestGateLabel(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{0.95, "HIGH"},
		{0.85, "HIGH"},
		{0.84, "MEDIUM"},
		{0.65, "MEDIUM"},
		{0.64, "LOW"},
		{0.40, "LOW"},
		{0.39, "INSUFFICIENT"},
		{0.0, "INSUFFICIENT"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, confidence.GateLabel(tc.score), "score=%v", tc.score)
	}
}

func TestComposite_Monotonic(t *testing.T) {
	// Increasing any signal should not decrease the composite
	base := confidence.Score{LLMRaw: 0.5, EvidenceCompleteness: 0.5, HistoricalAccuracy: 0.5, ToolCoverage: 0.5}
	baseScore := confidence.Composite(base)

	higher := base
	higher.LLMRaw = 0.9
	assert.GreaterOrEqual(t, confidence.Composite(higher), baseScore)
}

func TestComposite_AlwaysInRange(t *testing.T) {
	// Fuzz-style: random combinations should always return [0,1]
	inputs := []float64{0, 0.25, 0.5, 0.75, 1}
	for _, llm := range inputs {
		for _, ev := range inputs {
			got := confidence.Composite(confidence.Score{LLMRaw: llm, EvidenceCompleteness: ev, HistoricalAccuracy: 0.5, ToolCoverage: 0.5})
			assert.GreaterOrEqual(t, got, 0.0)
			assert.LessOrEqual(t, got, 1.0)
		}
	}
}
