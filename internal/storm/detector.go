// Package storm detects alert storms — bursts of alerts that exceed a
// rate threshold within a sliding time window for a given tenant.
// When a storm is detected, subsequent alerts are tagged and callers
// can choose to suppress, batch, or escalate them differently.
package storm

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// DefaultBurst is the alert count that triggers storm detection within one window.
const DefaultBurst = 50

// DefaultWindow is the sliding window duration for burst counting.
const DefaultWindow = time.Minute

// Store is the minimal interface for atomic increment+TTL operations.
// Compatible with Valkey/Redis INCR + EXPIRE semantics.
type Store interface {
	// Incr atomically increments the integer value at key and returns the new value.
	// If the key does not exist it is created with value 1.
	Incr(ctx context.Context, key string) (int64, error)
	// Expire sets the TTL on key only if the key has no TTL already set (NX variant).
	// Returns true if the TTL was set, false if key had a TTL or did not exist.
	ExpireNX(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// Detector counts alerts per tenant per window and signals storms.
type Detector struct {
	store  Store
	burst  int64
	window time.Duration
	log    *zap.Logger
}

// New creates a Detector with the given Store.
func New(store Store, log *zap.Logger) *Detector {
	if log == nil {
		log = zap.NewNop()
	}
	return &Detector{store: store, burst: DefaultBurst, window: DefaultWindow, log: log}
}

// WithBurst overrides the burst threshold and returns a new Detector.
func (d *Detector) WithBurst(burst int64) *Detector {
	nd := *d
	nd.burst = burst
	return &nd
}

// WithWindow overrides the sliding window duration and returns a new Detector.
func (d *Detector) WithWindow(window time.Duration) *Detector {
	nd := *d
	nd.window = window
	return &nd
}

// Record increments the alert count for tenantID and reports whether a storm
// is currently active (count ≥ burst threshold).
// The counter key expires after one window (set on first increment only — NX).
func (d *Detector) Record(ctx context.Context, tenantID string) (storm bool, count int64, err error) {
	key := d.key(tenantID)

	count, err = d.store.Incr(ctx, key)
	if err != nil {
		return false, 0, fmt.Errorf("storm.Record incr: %w", err)
	}

	// Set TTL only on first write (ExpireNX = EXPIRE … NX).
	if count == 1 {
		if _, expErr := d.store.ExpireNX(ctx, key, d.window); expErr != nil {
			d.log.Warn("storm: failed to set window TTL", zap.String("tenant", tenantID), zap.Error(expErr))
		}
	}

	storm = count >= d.burst
	if count == d.burst {
		d.log.Warn("alert storm detected",
			zap.String("tenant", tenantID),
			zap.Int64("count", count),
			zap.Int64("burst", d.burst),
			zap.Duration("window", d.window),
		)
	}
	return storm, count, nil
}

// key constructs the Valkey key for the burst counter.
func (d *Detector) key(tenantID string) string {
	return fmt.Sprintf("paladin:storm:%s", tenantID)
}
