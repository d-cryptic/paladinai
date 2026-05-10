// Package correlation groups related alerts into incident correlation windows.
// Strategy: same-tenant alerts with matching label sets (namespace + job + cluster)
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
	log    *zap.Logger
}

// New creates a Correlator with the given Store backend.
func New(store Store, log *zap.Logger) *Correlator {
	return &Correlator{store: store, window: CorrelationWindow, log: log}
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

// groupKey builds a stable hash key from the correlation label values.
// Alerts with the same group key are in the same incident group.
func (c *Correlator) groupKey(env *alert.AlertEnvelope) string {
	var parts []string
	parts = append(parts, "tenant:"+env.TenantID)

	for _, k := range correlationKeys {
		if v, ok := env.Labels[k]; ok && v != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	sort.Strings(parts[1:]) // sort label parts, keep tenant first

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
