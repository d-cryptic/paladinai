// bootstrap.go implements Stage 10 §12: bootstrap confidence intervals for
// eval pass-rate estimates.
//
// A single eval run gives a point estimate subject to LLM non-determinism.
// BootstrapCI resamples the observed scores with replacement to produce a
// 95% confidence interval so regressions can be distinguished from noise.
package eval

import (
	"math/rand"
	"sort"
)

// BootstrapCI computes a bootstrap confidence interval for a slice of
// per-case scores (each in [0, 1]).
//
//   - iterations is the number of bootstrap samples (1000 is the standard).
//   - confidence is the coverage probability, e.g. 0.95 for a 95% CI.
//
// Returns (lower, upper) bounds on the mean. Returns (0, 0) for empty input.
func BootstrapCI(scores []float64, iterations int, confidence float64) (lower, upper float64) {
	n := len(scores)
	if n == 0 {
		return 0, 0
	}
	if iterations <= 0 {
		iterations = 1000
	}
	if confidence <= 0 || confidence >= 1 {
		confidence = 0.95
	}

	means := make([]float64, iterations)
	sample := make([]float64, n)
	for i := range means {
		for j := range sample {
			sample[j] = scores[rand.Intn(n)] //nolint:gosec // non-crypto OK for statistical sampling
		}
		means[i] = mean(sample)
	}
	sort.Float64s(means)

	tail := (1 - confidence) / 2
	lo := int(float64(iterations) * tail)
	hi := int(float64(iterations) * (1 - tail))
	if hi >= iterations {
		hi = iterations - 1
	}
	return means[lo], means[hi]
}

// ScoresToFloat64 extracts the Score field from a slice of Score structs.
func ScoresToFloat64(scores []Score) []float64 {
	out := make([]float64, len(scores))
	for i, s := range scores {
		out[i] = s.Score
	}
	return out
}

// MetricCI holds a metric estimate with its bootstrap confidence interval.
type MetricCI struct {
	Mean  float64
	Lower float64 // 95% CI lower bound
	Upper float64 // 95% CI upper bound
}

// NewMetricCI computes a MetricCI from a slice of scores.
func NewMetricCI(scores []float64) MetricCI {
	lo, hi := BootstrapCI(scores, 1000, 0.95)
	return MetricCI{Mean: mean(scores), Lower: lo, Upper: hi}
}

// RegressionDetected returns true only when the current CI is entirely below
// the baseline CI (non-overlapping). This prevents flaky blocks caused by
// LLM non-determinism within the noise floor.
func RegressionDetected(baseline, current MetricCI) bool {
	return current.Upper < baseline.Lower
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	return sum / float64(len(v))
}
