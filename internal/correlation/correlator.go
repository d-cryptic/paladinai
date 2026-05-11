// Package correlation groups related alerts into incident correlation windows.
// Strategy: same-tenant alerts with matching label sets (namespace + job + cluster + service)
// within a 5-minute sliding window are assigned the same CorrelationID.
//
// This implements the Stage 2 spec: label-based correlation with 5-min window.
package correlation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/paladinai/paladinai/internal/alert"
	"go.uber.org/zap"
)

// CorrelationWindow is how long after the first alert in a group we keep the window open.
const CorrelationWindow = 5 * time.Minute

// DefaultGate is the minimum label-coverage fraction required for an alert to join
// a correlation group. Alerts providing fewer correlation keys than this fraction
// are isolated to their own fingerprint-based group to prevent false merges.
// Derived from Stage 3.5 decision-math thresholds.
const DefaultGate = 0.65

// correlationKeys are the label keys used to group related alerts.
// Alerts with the same values for all of these keys form a correlation group.
var correlationKeys = []string{"namespace", "job", "cluster", "service"}

// Store is the minimal interface for storing/retrieving correlation state.
// Must support atomic SetNX and Get operations.
type Store interface {
	// GetOrSet returns the existing correlation ID for the group key, or stores
	// and returns the provided default if not present.
	GetOrSet(ctx context.Context, key, defaultValue string, ttl time.Duration) (value string, wasNew bool, err error)
}

// Correlator assigns CorrelationIDs to alerts based on shared label groups.
type Correlator struct {
	store  Store
	window time.Duration
	gate   float64 // minimum label-coverage fraction to join a group [0, 1]
	log    *zap.Logger
}

// New creates a Correlator with the given Store backend.
func New(store Store, log *zap.Logger) *Correlator {
	if log == nil {
		log = zap.NewNop()
	}
	return &Correlator{store: store, window: CorrelationWindow, gate: DefaultGate, log: log}
}

// WithGate sets the label-coverage gate threshold and returns a new Correlator.
// Alerts whose fraction of populated correlationKeys is below gate are isolated
// to their own group (fingerprint-based key) rather than merged into a label group.
// Values outside [0, 1] are clamped. A gate of 0 means all alerts are eligible
// for grouping; a gate of 1 requires all correlationKeys to be present.
//
// WithGate must be called before any concurrent use of Correlate.
// Correlator is safe for concurrent use after construction.
func (c *Correlator) WithGate(gate float64) *Correlator {
	if gate < 0 {
		gate = 0
	}
	if gate > 1 {
		gate = 1
	}
	nc := *c
	nc.gate = gate
	return &nc
}

// Correlate assigns a CorrelationID to the alert.
// If another alert with the same group key was seen in the last 5 minutes,
// the same CorrelationID is returned (same incident group).
// Otherwise a new CorrelationID is minted and stored.
func (c *Correlator) Correlate(ctx context.Context, env *alert.AlertEnvelope) error {
	groupKey := c.groupKey(env)
	correlationID := newCorrelationID(env.TenantID, groupKey)

	existing, wasNew, err := c.store.GetOrSet(ctx, groupKey, correlationID, c.window)
	if err != nil {
		return fmt.Errorf("correlate: %w", err)
	}

	env.CorrelationID = existing

	if wasNew {
		c.log.Info("new correlation group",
			zap.String("correlation_id", existing),
			zap.String("tenant", env.TenantID),
			zap.String("fingerprint", env.Fingerprint),
		)
	} else {
		c.log.Debug("alert correlated to existing group",
			zap.String("correlation_id", existing),
			zap.String("tenant", env.TenantID),
			zap.String("fingerprint", env.Fingerprint),
		)
	}

	return nil
}

// labelCoverage returns the fraction of correlationKeys present with non-empty, non-whitespace values.
// Returns a value in [0, 1]: 0 = none present, 1 = all present.
func labelCoverage(labels map[string]string) float64 {
	if len(correlationKeys) == 0 {
		return 0
	}
	count := 0
	for _, k := range correlationKeys {
		if v, ok := labels[k]; ok && strings.TrimSpace(v) != "" {
			count++
		}
	}
	return float64(count) / float64(len(correlationKeys))
}

// groupKey builds a stable hash key from the correlation label values.
// Alerts with the same group key are in the same incident group.
//
// Gate enforcement: if the alert's label coverage is below c.gate, it is
// isolated to its own fingerprint-based group to avoid false merges with
// poorly-labelled alerts. Falls back to fingerprint if no correlation labels
// are present regardless of gate.
func (c *Correlator) groupKey(env *alert.AlertEnvelope) string {
	coverage := labelCoverage(env.Labels)
	if coverage < c.gate {
		// Below gate — isolate to fingerprint so this alert doesn't merge
		// into a label group it doesn't have enough context to belong to.
		c.log.Debug("correlation gate: isolating alert below coverage threshold",
			zap.Float64("coverage", coverage),
			zap.Float64("gate", c.gate),
			zap.String("fingerprint", env.Fingerprint),
			zap.String("tenant", env.TenantID),
		)
		parts := []string{"tenant:" + env.TenantID, "fp=" + env.Fingerprint}
		h := sha256.Sum256([]byte(strings.Join(parts, "|")))
		return "paladin:corr:" + hex.EncodeToString(h[:16])
	}

	labelParts := make([]string, 0, len(correlationKeys))
	for _, k := range correlationKeys {
		if v, ok := env.Labels[k]; ok && strings.TrimSpace(v) != "" {
			labelParts = append(labelParts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	sort.Strings(labelParts)

	// Fallback: if no labels survived (e.g. gate=0 and alert has no correlation labels),
	// isolate to fingerprint to prevent all label-less alerts collapsing into one group.
	if len(labelParts) == 0 {
		labelParts = append(labelParts, "fp="+env.Fingerprint)
	}

	parts := append([]string{"tenant:" + env.TenantID}, labelParts...)
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return "paladin:corr:" + hex.EncodeToString(h[:16])
}

// newCorrelationID generates a human-readable correlation ID.
// Format: corr-<8 hex chars of tenant+group hash>
func newCorrelationID(tenantID, groupKey string) string {
	h := sha256.Sum256([]byte(tenantID + groupKey))
	return "corr-" + hex.EncodeToString(h[:8])
}

// CorrelatedSubject returns the NATS subject for a processed (correlated) alert.
// Format: paladin.alerts.correlated.{tenantID}.{source}
func CorrelatedSubject(tenantID, source string) string {
	return fmt.Sprintf("paladin.alerts.correlated.%s.%s", tenantID, source)
}
