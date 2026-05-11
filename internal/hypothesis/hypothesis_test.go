package hypothesis_test

import (
	"math"
	"testing"

	"github.com/paladinai/paladinai/internal/hypothesis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Rank ─────────────────────────────────────────────────────────────────────

func TestRank_SpecExample(t *testing.T) {
	// Exactly the spec example from docs/plans/03.5.decision-math-stage3.5.md §7.
	// P(H1|e) ≈ 0.79, P(H2|e) ≈ 0.17, P(H3|e) ≈ 0.03
	hypotheses := []hypothesis.Hypothesis{
		{ID: "conn_pool", Label: "DB connection pool exhausted", Prior: 0.34, Likelihood: 0.91},
		{ID: "mem_leak", Label: "Memory leak in payments-service", Prior: 0.22, Likelihood: 0.31},
		{ID: "net_partition", Label: "Network partition between pods", Prior: 0.08, Likelihood: 0.15},
	}
	ranked, err := hypothesis.Rank(hypotheses)
	require.NoError(t, err)
	require.Len(t, ranked, 3)

	// Highest posterior should be conn_pool
	assert.Equal(t, "conn_pool", ranked[0].ID)
	assert.InDelta(t, 0.79, ranked[0].Posterior, 0.02)

	// Second is mem_leak
	assert.Equal(t, "mem_leak", ranked[1].ID)
	assert.InDelta(t, 0.17, ranked[1].Posterior, 0.02)

	// Posteriors sum to ~1.0
	sum := ranked[0].Posterior + ranked[1].Posterior + ranked[2].Posterior
	assert.InDelta(t, 1.0, sum, 1e-9)
}

func TestRank_SortedDescending(t *testing.T) {
	hypotheses := []hypothesis.Hypothesis{
		{ID: "h1", Prior: 0.5, Likelihood: 0.1},
		{ID: "h2", Prior: 0.3, Likelihood: 0.9},
		{ID: "h3", Prior: 0.2, Likelihood: 0.5},
	}
	ranked, err := hypothesis.Rank(hypotheses)
	require.NoError(t, err)
	for i := 1; i < len(ranked); i++ {
		assert.GreaterOrEqual(t, ranked[i-1].Posterior, ranked[i].Posterior)
	}
}

func TestRank_AllZeroLikelihoods_FallsBackToPriors(t *testing.T) {
	hypotheses := []hypothesis.Hypothesis{
		{ID: "h1", Prior: 0.6, Likelihood: 0},
		{ID: "h2", Prior: 0.4, Likelihood: 0},
	}
	ranked, err := hypothesis.Rank(hypotheses)
	require.NoError(t, err)
	// Should fall back to normalized priors
	assert.InDelta(t, 0.6, ranked[0].Posterior, 1e-9)
	assert.InDelta(t, 0.4, ranked[1].Posterior, 1e-9)
}

func TestRank_SingleHypothesis_Posterior1(t *testing.T) {
	hypotheses := []hypothesis.Hypothesis{
		{ID: "only", Prior: 1.0, Likelihood: 0.7},
	}
	ranked, err := hypothesis.Rank(hypotheses)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, ranked[0].Posterior, 1e-9)
}

func TestRank_EmptyInput_Error(t *testing.T) {
	_, err := hypothesis.Rank(nil)
	assert.ErrorIs(t, err, hypothesis.ErrNoHypotheses)
}

func TestRank_NegativePrior_Error(t *testing.T) {
	hypotheses := []hypothesis.Hypothesis{
		{ID: "h1", Prior: -0.1, Likelihood: 0.5},
	}
	_, err := hypothesis.Rank(hypotheses)
	assert.ErrorIs(t, err, hypothesis.ErrNegativePrior)
}

func TestRank_ZeroPrior_Error(t *testing.T) {
	hypotheses := []hypothesis.Hypothesis{
		{ID: "h1", Prior: 0, Likelihood: 0.5},
	}
	_, err := hypothesis.Rank(hypotheses)
	assert.ErrorIs(t, err, hypothesis.ErrNegativePrior)
}

func TestRank_LikelihoodOutOfRange_Error(t *testing.T) {
	hypotheses := []hypothesis.Hypothesis{
		{ID: "h1", Prior: 0.5, Likelihood: 1.5},
	}
	_, err := hypothesis.Rank(hypotheses)
	assert.ErrorIs(t, err, hypothesis.ErrLikelihoodOutOfRange)
}

func TestRank_NegativeLikelihood_Error(t *testing.T) {
	hypotheses := []hypothesis.Hypothesis{
		{ID: "h1", Prior: 0.5, Likelihood: -0.1},
	}
	_, err := hypothesis.Rank(hypotheses)
	assert.ErrorIs(t, err, hypothesis.ErrLikelihoodOutOfRange)
}

func TestRank_PriorsNotNormalized_StillWorks(t *testing.T) {
	// Priors don't need to sum to 1 — they are normalized internally.
	hypotheses := []hypothesis.Hypothesis{
		{ID: "h1", Prior: 34, Likelihood: 0.91},
		{ID: "h2", Prior: 22, Likelihood: 0.31},
		{ID: "h3", Prior: 8, Likelihood: 0.15},
	}
	ranked, err := hypothesis.Rank(hypotheses)
	require.NoError(t, err)
	assert.Equal(t, "h1", ranked[0].ID)
	sum := 0.0
	for _, r := range ranked {
		sum += r.Posterior
	}
	assert.InDelta(t, 1.0, sum, 1e-9)
}

// ─── UpdatePrior ─────────────────────────────────────────────────────────────

func TestUpdatePrior_ZeroIncidents_LaplaceSmoothingApplied(t *testing.T) {
	// (1 + 0) / (1 + 1 + 0) = 0.5
	p, err := hypothesis.UpdatePrior(0, 0)
	require.NoError(t, err)
	assert.InDelta(t, 0.5, p, 1e-9)
}

func TestUpdatePrior_AllCorrect_HighPrior(t *testing.T) {
	// 50 incidents, 50 correct: (1+50)/(1+1+50) = 51/52 ≈ 0.98
	p, err := hypothesis.UpdatePrior(50, 50)
	require.NoError(t, err)
	assert.InDelta(t, 51.0/52.0, p, 1e-9)
}

func TestUpdatePrior_NoneCorrect_LowPrior(t *testing.T) {
	// 50 incidents, 0 correct: (1+0)/(1+1+50) = 1/52 ≈ 0.019
	p, err := hypothesis.UpdatePrior(0, 50)
	require.NoError(t, err)
	assert.InDelta(t, 1.0/52.0, p, 1e-9)
}

func TestUpdatePrior_NegativeCount_Error(t *testing.T) {
	_, err := hypothesis.UpdatePrior(-1, 10)
	assert.Error(t, err)
}

func TestUpdatePrior_CorrectExceedsTotal_Error(t *testing.T) {
	_, err := hypothesis.UpdatePrior(10, 5)
	assert.Error(t, err)
}

func TestUpdatePrior_PriorInZeroOneRange(t *testing.T) {
	for correct := 0; correct <= 100; correct += 10 {
		p, err := hypothesis.UpdatePrior(correct, 100)
		require.NoError(t, err)
		assert.Greater(t, p, 0.0)
		assert.Less(t, p, 1.0)
	}
}

// ─── ConfidenceThresholdMet ───────────────────────────────────────────────────

func TestConfidenceThresholdMet_AboveThreshold(t *testing.T) {
	ranked := []hypothesis.RankedHypothesis{
		{Hypothesis: hypothesis.Hypothesis{ID: "h1"}, Posterior: 0.97},
		{Hypothesis: hypothesis.Hypothesis{ID: "h2"}, Posterior: 0.03},
	}
	assert.True(t, hypothesis.ConfidenceThresholdMet(ranked, 0.85))
}

func TestConfidenceThresholdMet_BelowThreshold(t *testing.T) {
	ranked := []hypothesis.RankedHypothesis{
		{Hypothesis: hypothesis.Hypothesis{ID: "h1"}, Posterior: 0.60},
	}
	assert.False(t, hypothesis.ConfidenceThresholdMet(ranked, 0.85))
}

func TestConfidenceThresholdMet_ExactThreshold(t *testing.T) {
	ranked := []hypothesis.RankedHypothesis{
		{Hypothesis: hypothesis.Hypothesis{ID: "h1"}, Posterior: 0.85},
	}
	assert.True(t, hypothesis.ConfidenceThresholdMet(ranked, 0.85))
}

func TestConfidenceThresholdMet_EmptySlice_False(t *testing.T) {
	assert.False(t, hypothesis.ConfidenceThresholdMet(nil, 0.85))
}

// ─── Entropy ──────────────────────────────────────────────────────────────────

func TestEntropy_UniformDistribution_MaxEntropy(t *testing.T) {
	// 4 equal hypotheses: entropy = log2(4) = 2 bits
	ranked := []hypothesis.RankedHypothesis{
		{Posterior: 0.25},
		{Posterior: 0.25},
		{Posterior: 0.25},
		{Posterior: 0.25},
	}
	h := hypothesis.Entropy(ranked)
	assert.InDelta(t, math.Log2(4), h, 1e-9)
}

func TestEntropy_SingleHypothesis_Zero(t *testing.T) {
	ranked := []hypothesis.RankedHypothesis{{Posterior: 1.0}}
	assert.InDelta(t, 0.0, hypothesis.Entropy(ranked), 1e-9)
}

func TestEntropy_TwoEqualHypotheses_OneBit(t *testing.T) {
	ranked := []hypothesis.RankedHypothesis{
		{Posterior: 0.5},
		{Posterior: 0.5},
	}
	assert.InDelta(t, 1.0, hypothesis.Entropy(ranked), 1e-9)
}

func TestEntropy_HighConfidence_LowEntropy(t *testing.T) {
	ranked := []hypothesis.RankedHypothesis{
		{Posterior: 0.97},
		{Posterior: 0.02},
		{Posterior: 0.01},
	}
	h := hypothesis.Entropy(ranked)
	assert.Less(t, h, 0.3, "high-confidence ranking should have low entropy")
}
