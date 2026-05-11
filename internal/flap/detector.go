// Package flap detects rapidly oscillating alerts (flapping) — alerts that
// transition between firing and resolved multiple times within a grace window.
// Flapping alerts are suppressed until they stabilise, reducing noise.
//
// Strategy: track per-fingerprint state transitions in a sliding grace window.
// An alert is flapping when it has ≥ flapThreshold transitions within the window.
// While flapping, new firing alerts for that fingerprint are held; resolved always pass.
package flap

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// DefaultGraceWindow is the window over which flip-flop transitions are counted.
const DefaultGraceWindow = 5 * time.Minute

// DefaultThreshold is the minimum firing→resolved or resolved→firing transitions
// within the grace window required to declare an alert flapping.
const DefaultThreshold = 4

// Store is the minimal Valkey/Redis interface needed to track flap counters.
type Store interface {
	// Incr atomically increments the transition counter for the given key.
	Incr(ctx context.Context, key string) (int64, error)
	// ExpireNX sets TTL on key only when the key has no TTL yet (SET … EX … NX semantics).
	ExpireNX(ctx context.Context, key string, ttl time.Duration) (bool, error)
	// Get returns the string value at key and a bool indicating whether it existed.
	Get(ctx context.Context, key string) (string, bool, error)
}

// Detector tracks alert state transitions and identifies flapping alerts.
type Detector struct {
	store     Store
	grace     time.Duration
	threshold int64
	log       *zap.Logger
}

// New creates a Detector backed by the given Store.
func New(store Store, log *zap.Logger) *Detector {
	if log == nil {
		log = zap.NewNop()
	}
	return &Detector{
		store:     store,
		grace:     DefaultGraceWindow,
		threshold: DefaultThreshold,
		log:       log,
	}
}

// WithGrace overrides the grace window and returns a new Detector.
func (d *Detector) WithGrace(window time.Duration) *Detector {
	nd := *d
	nd.grace = window
	return &nd
}

// WithThreshold overrides the flap transition threshold and returns a new Detector.
func (d *Detector) WithThreshold(n int64) *Detector {
	nd := *d
	nd.threshold = n
	return &nd
}

// RecordTransition records a state transition for the given alert fingerprint
// (any firing→resolved or resolved→firing change counts as one transition).
// Returns isFlapping=true when the transition count meets or exceeds the threshold.
//
// Callers should call RecordTransition on every status change and suppress
// or delay new firing alerts when isFlapping is true.
func (d *Detector) RecordTransition(ctx context.Context, tenantID, fingerprint string) (isFlapping bool, count int64, err error) {
	key := d.transitionKey(tenantID, fingerprint)

	count, err = d.store.Incr(ctx, key)
	if err != nil {
		return false, 0, fmt.Errorf("flap.RecordTransition incr: %w", err)
	}

	// Set TTL only on first write so the window resets naturally.
	if count == 1 {
		if _, expErr := d.store.ExpireNX(ctx, key, d.grace); expErr != nil {
			d.log.Warn("flap: failed to set grace window TTL",
				zap.String("tenant", tenantID),
				zap.String("fingerprint", fingerprint),
				zap.Error(expErr),
			)
		}
	}

	isFlapping = count >= d.threshold
	if isFlapping {
		d.log.Warn("flapping alert detected",
			zap.String("tenant", tenantID),
			zap.String("fingerprint", fingerprint),
			zap.Int64("transitions", count),
			zap.Int64("threshold", d.threshold),
			zap.Duration("grace", d.grace),
		)
	}
	return isFlapping, count, nil
}

// IsFlapping reports whether the fingerprint currently has a transition count
// at or above the threshold without recording a new transition.
func (d *Detector) IsFlapping(ctx context.Context, tenantID, fingerprint string) (bool, error) {
	key := d.transitionKey(tenantID, fingerprint)
	val, exists, err := d.store.Get(ctx, key)
	if err != nil {
		return false, fmt.Errorf("flap.IsFlapping get: %w", err)
	}
	if !exists {
		return false, nil
	}

	var count int64
	if _, scanErr := fmt.Sscanf(val, "%d", &count); scanErr != nil {
		return false, fmt.Errorf("flap.IsFlapping parse: %w", scanErr)
	}
	return count >= d.threshold, nil
}

// transitionKey returns the Valkey key for the flap transition counter.
func (d *Detector) transitionKey(tenantID, fingerprint string) string {
	return fmt.Sprintf("paladin:flap:%s:%s", tenantID, fingerprint)
}
