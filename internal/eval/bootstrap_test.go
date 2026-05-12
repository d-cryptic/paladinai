package eval

import (
	"math"
	"testing"
)

func TestBootstrapCI_AllOnes(t *testing.T) {
	scores := make([]float64, 100)
	for i := range scores {
		scores[i] = 1.0
	}
	lo, hi := BootstrapCI(scores, 1000, 0.95)
	// All-ones input → both bounds must be exactly 1.0.
	if math.Abs(lo-1.0) > 1e-9 || math.Abs(hi-1.0) > 1e-9 {
		t.Errorf("all-ones CI = (%.4f, %.4f), want (1.0, 1.0)", lo, hi)
	}
}

func TestBootstrapCI_AllZeros(t *testing.T) {
	scores := make([]float64, 100)
	lo, hi := BootstrapCI(scores, 1000, 0.95)
	if math.Abs(lo) > 1e-9 || math.Abs(hi) > 1e-9 {
		t.Errorf("all-zeros CI = (%.4f, %.4f), want (0.0, 0.0)", lo, hi)
	}
}

func TestBootstrapCI_EmptyInput(t *testing.T) {
	lo, hi := BootstrapCI(nil, 1000, 0.95)
	if lo != 0 || hi != 0 {
		t.Errorf("empty CI = (%.4f, %.4f), want (0, 0)", lo, hi)
	}
}

func TestBootstrapCI_BoundsContainMean(t *testing.T) {
	// Mix of 0s and 1s with known mean 0.7.
	scores := make([]float64, 100)
	for i := range scores {
		if i < 70 {
			scores[i] = 1.0
		}
	}
	lo, hi := BootstrapCI(scores, 2000, 0.95)
	if lo >= hi {
		t.Errorf("lower (%v) ≥ upper (%v)", lo, hi)
	}
	mean := 0.7
	if lo > mean || hi < mean {
		t.Errorf("CI (%.4f, %.4f) does not contain mean %.4f", lo, hi, mean)
	}
}

func TestBootstrapCI_DefaultsOnZeroIterations(t *testing.T) {
	scores := []float64{0.5, 0.5, 0.5}
	// Should not panic with zero iterations.
	lo, hi := BootstrapCI(scores, 0, 0.95)
	if lo > hi {
		t.Errorf("lower (%v) > upper (%v) with default iterations", lo, hi)
	}
}

func TestBootstrapCI_DefaultsOnInvalidConfidence(t *testing.T) {
	scores := []float64{0.8, 0.9, 0.7}
	lo, hi := BootstrapCI(scores, 100, -1.0) // invalid confidence → use 0.95
	if lo > hi {
		t.Errorf("lower (%v) > upper (%v)", lo, hi)
	}
}

func TestRegressionDetected_NonOverlapping(t *testing.T) {
	baseline := MetricCI{Mean: 0.923, Lower: 0.911, Upper: 0.935}
	current := MetricCI{Mean: 0.890, Lower: 0.875, Upper: 0.905}
	if !RegressionDetected(baseline, current) {
		t.Error("expected regression detected when CIs don't overlap")
	}
}

func TestRegressionDetected_Overlapping_NoRegression(t *testing.T) {
	baseline := MetricCI{Mean: 0.923, Lower: 0.911, Upper: 0.935}
	current := MetricCI{Mean: 0.918, Lower: 0.905, Upper: 0.930}
	if RegressionDetected(baseline, current) {
		t.Error("expected no regression when CIs overlap")
	}
}

func TestNewMetricCI_ContainsMean(t *testing.T) {
	scores := []float64{0.8, 0.9, 0.85, 0.88, 0.92, 0.87, 0.83, 0.91}
	ci := NewMetricCI(scores)
	if ci.Lower > ci.Mean || ci.Upper < ci.Mean {
		t.Errorf("CI (%.4f, %.4f) does not contain mean %.4f", ci.Lower, ci.Upper, ci.Mean)
	}
}

func TestScoresToFloat64(t *testing.T) {
	in := []Score{
		{Score: 0.9},
		{Score: 0.5},
		{Score: 1.0},
	}
	out := ScoresToFloat64(in)
	if len(out) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(out))
	}
	if out[0] != 0.9 || out[1] != 0.5 || out[2] != 1.0 {
		t.Errorf("ScoresToFloat64 = %v, unexpected values", out)
	}
}
