// Package anomaly implements the six anomaly detection methods from the
// Stage 3.5 decision-math specification. Each method is stateless — callers
// supply the time series and baseline parameters.
//
// Methods:
//  1. ZScore          — point anomaly vs Gaussian baseline
//  2. EWMA            — control chart for sustained shifts
//  3. STLResidual     — seasonal-trend decomposition before scoring
//  4. CUSUM           — changepoint detection via cumulative sum
//  5. GrangerLead     — test whether metric A predicts metric B
//  6. Mahalanobis     — multivariate distance accounting for correlations
package anomaly

import (
	"errors"
	"math"
)

// Result describes whether an anomaly was detected and how severe it is.
type Result struct {
	// Anomaly is true when the method's threshold was exceeded.
	Anomaly bool
	// Score is the raw detection statistic (z-score, D², CUSUM S⁺, etc.).
	Score float64
	// Threshold is the decision boundary for this method.
	Threshold float64
}

// ─── Method 1: Z-Score ───────────────────────────────────────────────────────

// ZScoreThreshold is the default threshold (3σ) for declaring a point anomaly.
const ZScoreThreshold = 3.0

// ZScore computes the standard-score anomaly for a single observation against
// a Gaussian baseline. Returns ErrInvalidInput when stddev ≤ 0.
// Threshold defaults to ZScoreThreshold when ≤ 0.
func ZScore(value, mean, stddev, threshold float64) (Result, error) {
	if stddev <= 0 {
		return Result{}, errors.New("anomaly.ZScore: stddev must be > 0")
	}
	if threshold <= 0 {
		threshold = ZScoreThreshold
	}
	z := math.Abs(value-mean) / stddev
	return Result{Anomaly: z >= threshold, Score: z, Threshold: threshold}, nil
}

// ─── Method 2: EWMA Control Chart ────────────────────────────────────────────

// EWMAParams holds the EWMA control chart parameters.
// Lambda is the smoothing factor (0 < λ ≤ 1); 0.2 is recommended.
// Mu0 and Sigma are the in-control mean and standard deviation.
// NSigma is the width of the control limits (default 3).
type EWMAParams struct {
	Lambda float64 // smoothing weight; recommended 0.2
	Mu0    float64 // baseline mean
	Sigma  float64 // baseline standard deviation
	NSigma float64 // control limit width (default 3)
}

// EWMAResult extends Result with the updated EWMA statistic.
type EWMAResult struct {
	Result
	// EWMA is the new exponentially weighted value after incorporating x.
	EWMA float64
}

// EWMAUpdate computes one EWMA step given the previous EWMA value and a new
// observation x, then checks whether the updated value exceeds the control limits.
//
// prevEWMA should be initialised to Mu0 at startup.
// Returns ErrInvalidInput when Sigma ≤ 0 or Lambda is out of (0, 1].
func EWMAUpdate(prevEWMA, x float64, p EWMAParams) (EWMAResult, error) {
	if p.Sigma <= 0 {
		return EWMAResult{}, errors.New("anomaly.EWMAUpdate: Sigma must be > 0")
	}
	if p.Lambda <= 0 || p.Lambda > 1 {
		return EWMAResult{}, errors.New("anomaly.EWMAUpdate: Lambda must be in (0, 1]")
	}
	if p.NSigma <= 0 {
		p.NSigma = 3
	}

	ewma := p.Lambda*x + (1-p.Lambda)*prevEWMA

	// Control limit: UCL/LCL = μ₀ ± NSigma × σ × √(λ/(2-λ))
	spread := p.NSigma * p.Sigma * math.Sqrt(p.Lambda/(2-p.Lambda))
	ucl := p.Mu0 + spread
	lcl := p.Mu0 - spread

	anomaly := ewma > ucl || ewma < lcl
	// Score: how many spreads away from the nearest limit
	var score float64
	if ewma > p.Mu0 {
		score = (ewma - ucl) / spread
	} else {
		score = (lcl - ewma) / spread
	}

	return EWMAResult{
		Result: Result{Anomaly: anomaly, Score: score, Threshold: 0},
		EWMA:   ewma,
	}, nil
}

// ─── Method 3: STL Residual ──────────────────────────────────────────────────

// STLResidual returns the residual component after subtracting trend and seasonal
// components from raw, then calls ZScore on the residual.
//
// trend and seasonal must have the same length as raw.
// Returns ErrInvalidInput on length mismatch or when stddev ≤ 0.
func STLResidual(raw, trend, seasonal []float64, residualMean, residualStddev, threshold float64) ([]Result, error) {
	n := len(raw)
	if len(trend) != n || len(seasonal) != n {
		return nil, errors.New("anomaly.STLResidual: raw, trend, and seasonal must have equal length")
	}
	results := make([]Result, n)
	for i := range raw {
		residual := raw[i] - trend[i] - seasonal[i]
		r, err := ZScore(residual, residualMean, residualStddev, threshold)
		if err != nil {
			return nil, err
		}
		results[i] = r
	}
	return results, nil
}

// ─── Method 4: CUSUM ─────────────────────────────────────────────────────────

// CUSUMState holds the running CUSUM statistics across calls.
// Zero value is valid: S+ and S- start at 0.
type CUSUMState struct {
	SPos float64 // S⁺(t): cumulative upward deviation
	SNeg float64 // S⁻(t): cumulative downward deviation
}

// CUSUMResult extends Result with the full CUSUM state.
type CUSUMResult struct {
	Result
	State CUSUMState
	// Direction is "up", "down", or "" when not anomalous.
	Direction string
}

// CUSUMUpdate performs one CUSUM step for observation x against baseline mean mu0.
// k is the allowable slack (typically 0.5 × shift-to-detect in σ units).
// h is the decision threshold (typically 4–5 in σ units).
func CUSUMUpdate(state CUSUMState, x, mu0, k, h float64) (CUSUMResult, error) {
	if h <= 0 {
		return CUSUMResult{}, errors.New("anomaly.CUSUMUpdate: h must be > 0")
	}

	sPos := math.Max(0, state.SPos+(x-mu0-k))
	sNeg := math.Max(0, state.SNeg-(x-mu0-k))

	newState := CUSUMState{SPos: sPos, SNeg: sNeg}
	score := math.Max(sPos, sNeg)
	anomaly := score >= h

	direction := ""
	if anomaly {
		if sPos >= sNeg {
			direction = "up"
		} else {
			direction = "down"
		}
	}

	return CUSUMResult{
		Result:    Result{Anomaly: anomaly, Score: score, Threshold: h},
		State:     newState,
		Direction: direction,
	}, nil
}

// ─── Method 5: Granger Causality ─────────────────────────────────────────────

// GrangerResult reports whether series A Granger-causes series B.
type GrangerResult struct {
	// Causes is true when the F-test p-value is below Alpha.
	Causes bool
	// FStatistic is the computed F-statistic.
	FStatistic float64
	// PValue is an approximate p-value from the F distribution (requires >30 samples).
	PValue float64
	// Lag is the number of lags used in the VAR model.
	Lag int
}

// GrangerTest runs a simplified Granger causality test at the given lag.
// It fits a restricted VAR (only lags of B) and an unrestricted VAR
// (lags of both A and B) using OLS, then computes the F-statistic.
//
// Minimum series length: 2×lag + 1. Returns ErrInvalidInput for short series.
// alpha is the significance level; 0.05 is the conventional choice.
func GrangerTest(a, b []float64, lag int, alpha float64) (GrangerResult, error) {
	n := len(a)
	if len(b) != n {
		return GrangerResult{}, errors.New("anomaly.GrangerTest: a and b must have equal length")
	}
	if n < 2*lag+1 {
		return GrangerResult{}, errors.New("anomaly.GrangerTest: series too short for given lag")
	}
	if lag < 1 {
		return GrangerResult{}, errors.New("anomaly.GrangerTest: lag must be ≥ 1")
	}
	if alpha <= 0 || alpha >= 1 {
		alpha = 0.05
	}

	// Build the response vector y and design matrices for OLS.
	obs := n - lag
	y := b[lag:]

	// Restricted model: y ~ B_{t-1..lag}
	rssR := olsRSS(buildMatrix([][]float64{b[:n-lag]}, lag, obs), y)

	// Unrestricted model: y ~ B_{t-1..lag} + A_{t-1..lag}
	rssU := olsRSS(buildMatrix([][]float64{b[:n-lag], a[:n-lag]}, lag, obs), y)

	// F-statistic: ((RSSR - RSSU)/q) / (RSSU/(n-k))
	q := float64(lag)       // restrictions
	k := float64(2*lag + 1) // unrestricted params
	df2 := float64(obs) - k
	if df2 <= 0 || rssU == 0 {
		return GrangerResult{}, errors.New("anomaly.GrangerTest: insufficient degrees of freedom")
	}

	fStat := ((rssR - rssU) / q) / (rssU / df2)
	// Approximate p-value using chi-squared approximation (p ≈ P(χ²(q) > q×F))
	pValue := approxFPValue(fStat, q, df2)

	return GrangerResult{
		Causes:     pValue < alpha,
		FStatistic: fStat,
		PValue:     pValue,
		Lag:        lag,
	}, nil
}

// ─── Method 6: Mahalanobis Distance ──────────────────────────────────────────

// MahalanobisResult extends Result with the squared distance D².
type MahalanobisResult struct {
	Result
	// DSquared is the Mahalanobis D² value.
	DSquared float64
}

// MahalanobisDistance computes D² = (x-μ)ᵀ Σ⁻¹ (x-μ) for a single observation.
// covInv must be the inverse of the covariance matrix (precomputed by caller).
// threshold is typically the chi-squared critical value at df=len(x) and α=0.001.
//
// Returns ErrInvalidInput on dimension mismatch.
func MahalanobisDistance(x, mean []float64, covInv [][]float64, threshold float64) (MahalanobisResult, error) {
	d := len(x)
	if len(mean) != d {
		return MahalanobisResult{}, errors.New("anomaly.MahalanobisDistance: x and mean must have equal length")
	}
	if len(covInv) != d {
		return MahalanobisResult{}, errors.New("anomaly.MahalanobisDistance: covInv row count must equal len(x)")
	}
	for _, row := range covInv {
		if len(row) != d {
			return MahalanobisResult{}, errors.New("anomaly.MahalanobisDistance: covInv must be square")
		}
	}

	// diff = x - μ
	diff := make([]float64, d)
	for i := range diff {
		diff[i] = x[i] - mean[i]
	}

	// tmp = Σ⁻¹ × diff
	tmp := make([]float64, d)
	for i := range tmp {
		for j := range diff {
			tmp[i] += covInv[i][j] * diff[j]
		}
	}

	// D² = diff · tmp
	var dSq float64
	for i := range diff {
		dSq += diff[i] * tmp[i]
	}

	return MahalanobisResult{
		Result:   Result{Anomaly: dSq >= threshold, Score: dSq, Threshold: threshold},
		DSquared: dSq,
	}, nil
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

// buildMatrix constructs a design matrix (with intercept column) from lagged series.
// Each series in sources is expanded into lag columns.
func buildMatrix(sources [][]float64, lag, obs int) [][]float64 {
	// columns: intercept + lag columns per source
	cols := 1 + len(sources)*lag
	X := make([][]float64, obs)
	for i := range X {
		row := make([]float64, cols)
		row[0] = 1 // intercept
		col := 1
		for _, src := range sources {
			for l := 1; l <= lag; l++ {
				row[col] = src[len(src)-obs+i-l+lag]
				col++
			}
		}
		X[i] = row
	}
	return X
}

// olsRSS fits OLS via normal equations and returns the residual sum of squares.
// X is obs×cols; y is obs.
func olsRSS(X [][]float64, y []float64) float64 {
	obs := len(y)
	cols := len(X[0])

	// XtX = Xᵀ X
	XtX := make([][]float64, cols)
	for i := range XtX {
		XtX[i] = make([]float64, cols)
		for j := range XtX[i] {
			for k := range X {
				XtX[i][j] += X[k][i] * X[k][j]
			}
		}
	}

	// Xty = Xᵀ y
	Xty := make([]float64, cols)
	for i := range Xty {
		for k := range X {
			Xty[i] += X[k][i] * y[k]
		}
	}

	// β = (XtX)⁻¹ Xty via Gaussian elimination
	beta := solveLinear(XtX, Xty)
	if beta == nil {
		return 0
	}

	// RSS = Σ (y_i - X_i·β)²
	var rss float64
	for i := 0; i < obs; i++ {
		var pred float64
		for j := range beta {
			pred += X[i][j] * beta[j]
		}
		r := y[i] - pred
		rss += r * r
	}
	return rss
}

// solveLinear solves Ax = b using Gaussian elimination with partial pivoting.
// Returns nil if the system is singular.
func solveLinear(A [][]float64, b []float64) []float64 {
	n := len(b)
	// Augmented matrix
	aug := make([][]float64, n)
	for i := range aug {
		aug[i] = make([]float64, n+1)
		copy(aug[i], A[i])
		aug[i][n] = b[i]
	}

	for col := 0; col < n; col++ {
		// Partial pivot
		maxRow := col
		for row := col + 1; row < n; row++ {
			if math.Abs(aug[row][col]) > math.Abs(aug[maxRow][col]) {
				maxRow = row
			}
		}
		aug[col], aug[maxRow] = aug[maxRow], aug[col]

		pivot := aug[col][col]
		if math.Abs(pivot) < 1e-12 {
			return nil // singular
		}
		for j := col; j <= n; j++ {
			aug[col][j] /= pivot
		}
		for row := 0; row < n; row++ {
			if row == col {
				continue
			}
			factor := aug[row][col]
			for j := col; j <= n; j++ {
				aug[row][j] -= factor * aug[col][j]
			}
		}
	}

	x := make([]float64, n)
	for i := range x {
		x[i] = aug[i][n]
	}
	return x
}

// approxFPValue returns an approximate p-value for F(df1, df2) > f using the
// Wilson-Hilferty cube-root normal approximation.
// This is accurate for df2 > 30; use a proper F-distribution for small samples.
func approxFPValue(f, df1, df2 float64) float64 {
	if f <= 0 {
		return 1.0
	}
	// Wilson-Hilferty approximation: transform F to approximately N(0,1)
	x := (math.Pow(f*df1/df2, 1.0/3) - (1 - 2/(9*df1))) / math.Sqrt(2/(9*df1))
	// P(Z > x) where Z ~ N(0,1)
	return 1 - stdNormalCDF(x)
}

// stdNormalCDF approximates Φ(x) using Horner's method (Abramowitz & Stegun 26.2.17).
func stdNormalCDF(x float64) float64 {
	if x < -8 {
		return 0
	}
	if x > 8 {
		return 1
	}
	// Use symmetry
	neg := x < 0
	if neg {
		x = -x
	}

	t := 1 / (1 + 0.2316419*x)
	poly := t * (0.319381530 +
		t*(-0.356563782+
			t*(1.781477937+
				t*(-1.821255978+
					t*1.330274429))))
	cdf := 1 - math.Exp(-0.5*x*x)/math.Sqrt(2*math.Pi)*poly
	if neg {
		return 1 - cdf
	}
	return cdf
}
