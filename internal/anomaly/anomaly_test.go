package anomaly_test

import (
	"math"
	"testing"

	"github.com/paladinai/paladinai/internal/anomaly"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── ZScore ──────────────────────────────────────────────────────────────────

func TestZScore_NormalPoint(t *testing.T) {
	r, err := anomaly.ZScore(48, 45, 8, 3)
	require.NoError(t, err)
	assert.False(t, r.Anomaly, "value 1σ above mean should not be anomalous")
	assert.InDelta(t, 0.375, r.Score, 1e-6)
}

func TestZScore_AnomalousPoint(t *testing.T) {
	r, err := anomaly.ZScore(90, 45, 8, 3)
	require.NoError(t, err)
	assert.True(t, r.Anomaly, "value 5.625σ above mean should be anomalous")
	assert.InDelta(t, 5.625, r.Score, 1e-6)
}

func TestZScore_ExactThreshold(t *testing.T) {
	// value exactly at 3σ above mean → anomaly=true (score ≥ threshold)
	r, err := anomaly.ZScore(69, 45, 8, 3)
	require.NoError(t, err)
	assert.True(t, r.Anomaly)
}

func TestZScore_NegativeSide(t *testing.T) {
	r, err := anomaly.ZScore(0, 45, 8, 3)
	require.NoError(t, err)
	assert.True(t, r.Anomaly)
}

func TestZScore_DefaultThreshold(t *testing.T) {
	r, err := anomaly.ZScore(90, 45, 8, 0) // 0 → use ZScoreThreshold=3
	require.NoError(t, err)
	assert.Equal(t, anomaly.ZScoreThreshold, r.Threshold)
}

func TestZScore_InvalidStddev(t *testing.T) {
	_, err := anomaly.ZScore(1, 0, 0, 3)
	assert.Error(t, err)
}

// ─── EWMA ────────────────────────────────────────────────────────────────────

func TestEWMAUpdate_NormalValue(t *testing.T) {
	p := anomaly.EWMAParams{Lambda: 0.2, Mu0: 45, Sigma: 8, NSigma: 3}
	r, err := anomaly.EWMAUpdate(45, 46, p) // small deviation
	require.NoError(t, err)
	assert.False(t, r.Anomaly)
	assert.InDelta(t, 45.2, r.EWMA, 0.01)
}

func TestEWMAUpdate_ControlLimitBreached(t *testing.T) {
	p := anomaly.EWMAParams{Lambda: 0.2, Mu0: 45, Sigma: 8, NSigma: 3}
	// Simulate EWMA already near the UCL, then add a large value
	// spread = 3 × 8 × sqrt(0.2/1.8) ≈ 8.0
	// UCL ≈ 53
	r, err := anomaly.EWMAUpdate(70, 200, p)
	require.NoError(t, err)
	assert.True(t, r.Anomaly, "EWMA should breach UCL with 200 as new value from 70")
}

func TestEWMAUpdate_InvalidSigma(t *testing.T) {
	_, err := anomaly.EWMAUpdate(45, 45, anomaly.EWMAParams{Lambda: 0.2, Mu0: 45, Sigma: 0})
	assert.Error(t, err)
}

func TestEWMAUpdate_InvalidLambda(t *testing.T) {
	_, err := anomaly.EWMAUpdate(45, 45, anomaly.EWMAParams{Lambda: 0, Mu0: 45, Sigma: 8})
	assert.Error(t, err)
}

func TestEWMAUpdate_DefaultNSigma(t *testing.T) {
	p := anomaly.EWMAParams{Lambda: 0.2, Mu0: 45, Sigma: 8, NSigma: 0} // 0 → default 3
	_, err := anomaly.EWMAUpdate(45, 45, p)
	assert.NoError(t, err)
}

// ─── STL Residual ────────────────────────────────────────────────────────────

func TestSTLResidual_Normal(t *testing.T) {
	raw := []float64{50, 55, 52}
	trend := []float64{48, 50, 52}
	seasonal := []float64{2, 4, 1}
	// residuals: 0, 1, -1 → all within 3σ for σ=2
	results, err := anomaly.STLResidual(raw, trend, seasonal, 0, 2, 3)
	require.NoError(t, err)
	require.Len(t, results, 3)
	for _, r := range results {
		assert.False(t, r.Anomaly)
	}
}

func TestSTLResidual_AnomalousResidual(t *testing.T) {
	raw := []float64{50, 55, 100} // last point has large residual
	trend := []float64{48, 50, 52}
	seasonal := []float64{2, 4, 1}
	// residuals: 0, 1, 47 → last is way above 3σ for σ=2
	results, err := anomaly.STLResidual(raw, trend, seasonal, 0, 2, 3)
	require.NoError(t, err)
	assert.False(t, results[0].Anomaly)
	assert.True(t, results[2].Anomaly, "residual 47 should be anomalous with σ=2, threshold=3")
}

func TestSTLResidual_LengthMismatch(t *testing.T) {
	_, err := anomaly.STLResidual([]float64{1, 2}, []float64{1}, []float64{0, 0}, 0, 1, 3)
	assert.Error(t, err)
}

// ─── CUSUM ───────────────────────────────────────────────────────────────────

func TestCUSUM_NoAnomaly(t *testing.T) {
	state := anomaly.CUSUMState{}
	r, err := anomaly.CUSUMUpdate(state, 46, 45, 4, 40)
	require.NoError(t, err)
	assert.False(t, r.Anomaly)
	assert.Equal(t, "", r.Direction) // no anomaly → no direction
}

func TestCUSUM_UpwardAnomaly(t *testing.T) {
	// Accumulate upward shift over many steps
	state := anomaly.CUSUMState{}
	var r anomaly.CUSUMResult
	var err error
	// x=55, mu0=45, k=4 → S⁺ grows by 6 each step
	for i := 0; i < 8; i++ {
		r, err = anomaly.CUSUMUpdate(state, 55, 45, 4, 40)
		require.NoError(t, err)
		state = r.State
	}
	assert.True(t, r.Anomaly)
	assert.Equal(t, "up", r.Direction)
}

func TestCUSUM_ResetOnNormal(t *testing.T) {
	// After a spike, normal values should push S⁺ back toward 0
	state := anomaly.CUSUMState{SPos: 10}
	r, err := anomaly.CUSUMUpdate(state, 40, 45, 4, 40)
	require.NoError(t, err)
	// x=40, mu0=45, k=4 → x - mu0 - k = -9 → S⁺ = max(0, 10-9) = 1
	assert.InDelta(t, 1.0, r.State.SPos, 1e-9)
}

func TestCUSUM_InvalidH(t *testing.T) {
	_, err := anomaly.CUSUMUpdate(anomaly.CUSUMState{}, 50, 45, 4, 0)
	assert.Error(t, err)
}

// ─── Granger ─────────────────────────────────────────────────────────────────

func TestGrangerTest_SuffficientData(t *testing.T) {
	// synthetic correlated series
	n := 60
	a := make([]float64, n)
	b := make([]float64, n)
	for i := range a {
		a[i] = float64(i%10) + 1
	}
	// b lags a by 1
	b[0] = 0
	for i := 1; i < n; i++ {
		b[i] = a[i-1] + 0.1
	}

	r, err := anomaly.GrangerTest(a, b, 1, 0.05)
	require.NoError(t, err)
	// With strong correlation we expect Causes=true
	assert.True(t, r.Causes, "a should Granger-cause b with lag=1")
	assert.Equal(t, 1, r.Lag)
}

func TestGrangerTest_ShortSeries(t *testing.T) {
	_, err := anomaly.GrangerTest([]float64{1, 2}, []float64{1, 2}, 5, 0.05)
	assert.Error(t, err)
}

func TestGrangerTest_LengthMismatch(t *testing.T) {
	_, err := anomaly.GrangerTest([]float64{1, 2, 3}, []float64{1, 2}, 1, 0.05)
	assert.Error(t, err)
}

func TestGrangerTest_InvalidLag(t *testing.T) {
	_, err := anomaly.GrangerTest(make([]float64, 20), make([]float64, 20), 0, 0.05)
	assert.Error(t, err)
}

// ─── Mahalanobis ─────────────────────────────────────────────────────────────

func TestMahalanobis_IdentityCovariance(t *testing.T) {
	// With identity covariance, D² = Σ (x_i - μ_i)²
	x := []float64{1, 0, 0}
	mu := []float64{0, 0, 0}
	covInv := [][]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}

	r, err := anomaly.MahalanobisDistance(x, mu, covInv, 16.27)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, r.DSquared, 1e-9)
	assert.False(t, r.Anomaly)
}

func TestMahalanobis_AnomalousPoint(t *testing.T) {
	// x is 5 units away on each dimension → D²=75
	x := []float64{5, 5, 5}
	mu := []float64{0, 0, 0}
	covInv := [][]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}

	r, err := anomaly.MahalanobisDistance(x, mu, covInv, 16.27)
	require.NoError(t, err)
	assert.InDelta(t, 75.0, r.DSquared, 1e-9)
	assert.True(t, r.Anomaly)
}

func TestMahalanobis_LengthMismatch(t *testing.T) {
	_, err := anomaly.MahalanobisDistance(
		[]float64{1, 2},
		[]float64{0},
		[][]float64{{1}},
		10,
	)
	assert.Error(t, err)
}

func TestMahalanobis_NonSquareCovInv(t *testing.T) {
	_, err := anomaly.MahalanobisDistance(
		[]float64{1, 2},
		[]float64{0, 0},
		[][]float64{{1}, {0, 1}},
		10,
	)
	assert.Error(t, err)
}

func TestMahalanobis_ZeroDistance(t *testing.T) {
	x := []float64{3, 4}
	mu := []float64{3, 4}
	covInv := [][]float64{{1, 0}, {0, 1}}

	r, err := anomaly.MahalanobisDistance(x, mu, covInv, 5.99)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, r.DSquared, 1e-9)
	assert.False(t, r.Anomaly)
}

// ─── NaN / Inf safety ────────────────────────────────────────────────────────

func TestZScore_NoNaNOrInf(t *testing.T) {
	r, err := anomaly.ZScore(45, 45, 0.001, 3)
	require.NoError(t, err)
	assert.False(t, math.IsNaN(r.Score))
	assert.False(t, math.IsInf(r.Score, 0))
}
