// Package hypothesis implements Bayesian hypothesis ranking for RCA agents.
// Given a set of competing hypotheses with prior probabilities and evidence
// likelihoods, it computes normalized posterior probabilities and supports
// online prior updates after each resolved incident.
//
// See docs/plans/03.5.decision-math-stage3.5.md §7 for the specification.
package hypothesis

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// Hypothesis is a single competing explanation for an incident.
type Hypothesis struct {
	// ID uniquely identifies the hypothesis (e.g. "db_conn_pool_exhausted").
	ID string
	// Label is a human-readable description.
	Label string
	// Prior is P(H) — the prior probability before observing evidence.
	// Must be > 0. All priors are normalized internally.
	Prior float64
	// Likelihood is P(evidence | H) — how well the current evidence supports H.
	// Must be in [0, 1].
	Likelihood float64
}

// RankedHypothesis is a Hypothesis annotated with its posterior probability.
type RankedHypothesis struct {
	Hypothesis
	// Posterior is P(H | evidence), normalized across all hypotheses.
	Posterior float64
}

// ErrNoHypotheses is returned when the input slice is empty.
var ErrNoHypotheses = errors.New("hypothesis: at least one hypothesis is required")

// ErrNegativePrior is returned when a hypothesis has a non-positive prior.
var ErrNegativePrior = errors.New("hypothesis: priors must be positive")

// ErrLikelihoodOutOfRange is returned when a likelihood is not in [0, 1].
var ErrLikelihoodOutOfRange = errors.New("hypothesis: likelihood must be in [0, 1]")

// Rank computes posterior probabilities for each hypothesis using Bayes' theorem
// and returns them sorted by descending posterior (highest confidence first).
//
// The unnormalized score for each hypothesis is: prior × likelihood.
// All scores are then divided by their sum to produce a valid probability distribution.
//
// If all likelihoods are zero (no evidence supports any hypothesis), posteriors
// are set equal to the normalized priors.
func Rank(hypotheses []Hypothesis) ([]RankedHypothesis, error) {
	if len(hypotheses) == 0 {
		return nil, ErrNoHypotheses
	}
	for _, h := range hypotheses {
		if h.Prior <= 0 {
			return nil, fmt.Errorf("%w: hypothesis %q has prior %g", ErrNegativePrior, h.ID, h.Prior)
		}
		if h.Likelihood < 0 || h.Likelihood > 1 {
			return nil, fmt.Errorf("%w: hypothesis %q has likelihood %g", ErrLikelihoodOutOfRange, h.ID, h.Likelihood)
		}
	}

	// Normalize priors so they sum to 1 regardless of input scale.
	priorSum := 0.0
	for _, h := range hypotheses {
		priorSum += h.Prior
	}

	// Compute unnormalized posteriors: normalized_prior × likelihood.
	unnorm := make([]float64, len(hypotheses))
	unnormSum := 0.0
	for i, h := range hypotheses {
		unnorm[i] = (h.Prior / priorSum) * h.Likelihood
		unnormSum += unnorm[i]
	}

	results := make([]RankedHypothesis, len(hypotheses))
	for i, h := range hypotheses {
		posterior := 0.0
		if unnormSum > 0 {
			posterior = unnorm[i] / unnormSum
		} else {
			// No evidence supports any hypothesis — fall back to normalized priors.
			posterior = h.Prior / priorSum
		}
		results[i] = RankedHypothesis{Hypothesis: h, Posterior: posterior}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Posterior > results[j].Posterior
	})

	return results, nil
}

// UpdatePrior computes an updated prior for a hypothesis using Laplace smoothing.
// After each resolved incident:
//   - correctCount = number of times this hypothesis was the confirmed root cause
//   - totalIncidents = total incidents for this alert fingerprint
//
// Formula: P(H) = (alpha + correctCount) / (alpha + beta + totalIncidents)
// where alpha=1, beta=1 (Laplace smoothing prevents zero probabilities).
//
// Returns an error if totalIncidents < correctCount or either is negative.
func UpdatePrior(correctCount, totalIncidents int) (float64, error) {
	if correctCount < 0 || totalIncidents < 0 {
		return 0, fmt.Errorf("hypothesis: counts must be non-negative")
	}
	if correctCount > totalIncidents {
		return 0, fmt.Errorf("hypothesis: correctCount %d > totalIncidents %d", correctCount, totalIncidents)
	}
	const alpha, beta = 1.0, 1.0
	return (alpha + float64(correctCount)) / (alpha + beta + float64(totalIncidents)), nil
}

// Top returns the highest-ranked hypothesis from a ranked list.
// Panics if the list is empty (callers should check Rank errors first).
func Top(ranked []RankedHypothesis) RankedHypothesis {
	return ranked[0]
}

// ConfidenceThresholdMet returns true if the top hypothesis posterior exceeds threshold.
// The Stage 3.5 spec uses 0.85 to skip Phase 3.
func ConfidenceThresholdMet(ranked []RankedHypothesis, threshold float64) bool {
	if len(ranked) == 0 {
		return false
	}
	return ranked[0].Posterior >= threshold
}

// Entropy computes the Shannon entropy of a ranked hypothesis distribution.
// High entropy indicates uncertainty (multiple plausible hypotheses).
// Low entropy indicates a clear winner. Returns 0 for a single hypothesis.
func Entropy(ranked []RankedHypothesis) float64 {
	h := 0.0
	for _, r := range ranked {
		if r.Posterior > 0 {
			h -= r.Posterior * math.Log2(r.Posterior)
		}
	}
	return h
}
