// Package dedup implements fingerprint-based alert deduplication.
// Dedup window: 5 minutes (configurable). Resolved alerts always pass through.
package dedup

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/paladinai/paladinai/internal/alert"
	"go.uber.org/zap"
)

const defaultWindow = 5 * time.Minute
const defaultKeyPrefix = "paladin:dedup"

// Store is the minimal interface needed for dedup.
// Using plain return types keeps it backend-agnostic (Valkey, Dragonfly, in-memory).
type Store interface {
	SetNX(ctx context.Context, key, value string, expiration time.Duration) (bool, error)
	Del(ctx context.Context, keys ...string) error
}

// Deduplicator uses a Store to track seen alert fingerprints.
type Deduplicator struct {
	store     Store
	window    time.Duration
	keyPrefix string
	log       *zap.Logger
}

// New creates a Deduplicator backed by a redis-compatible Store.
func New(store Store, log *zap.Logger) *Deduplicator {
	return NewWithKeyPrefix(store, defaultKeyPrefix, log)
}

// NewWithKeyPrefix creates a Deduplicator using an explicit backend key prefix.
func NewWithKeyPrefix(store Store, keyPrefix string, log *zap.Logger) *Deduplicator {
	if log == nil {
		log = zap.NewNop()
	}
	if keyPrefix == "" {
		keyPrefix = defaultKeyPrefix
	}
	return &Deduplicator{store: store, window: defaultWindow, keyPrefix: keyPrefix, log: log}
}

// IsDuplicate returns true if the alert's fingerprint was already seen within the dedup window.
// Resolved alerts bypass dedup — they must always propagate.
// Reset is NOT called on resolved alerts here; callers (e.g. pipeline.Process) call Reset
// explicitly so that the alert can re-fire after resolution.
func (d *Deduplicator) IsDuplicate(ctx context.Context, env *alert.AlertEnvelope) (bool, error) {
	if env == nil {
		return false, errors.New("dedup: nil envelope")
	}
	if env.Status == alert.StatusResolved {
		return false, nil
	}

	key := d.key(env.TenantID, env.Fingerprint)

	// SET NX with TTL — only succeeds on first sight
	ok, err := d.store.SetNX(ctx, key, env.ID, d.window)
	if err != nil {
		return false, fmt.Errorf("dedup check: %w", err)
	}

	if ok {
		d.log.Debug("dedup: new alert",
			zap.String("fingerprint", env.Fingerprint),
			zap.String("tenant", env.TenantID),
		)
		return false, nil
	}

	d.log.Debug("dedup: suppressed duplicate",
		zap.String("fingerprint", env.Fingerprint),
		zap.String("tenant", env.TenantID),
	)
	return true, nil
}

// Reset clears the dedup entry for an alert (call when alert resolves so it can re-fire).
func (d *Deduplicator) Reset(ctx context.Context, tenantID, fingerprint string) error {
	key := d.key(tenantID, fingerprint)
	d.log.Debug("dedup: resetting key", zap.String("fingerprint", fingerprint), zap.String("tenant", tenantID))
	return d.store.Del(ctx, key)
}

func (d *Deduplicator) key(tenantID, fingerprint string) string {
	return fmt.Sprintf("%s:%s:%s", d.keyPrefix, tenantID, fingerprint)
}
