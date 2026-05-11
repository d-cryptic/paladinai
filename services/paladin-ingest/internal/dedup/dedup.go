// Package dedup re-exports the shared internal/dedup implementation.
// All dedup logic lives in github.com/paladinai/paladinai/internal/dedup.
package dedup

import (
	"go.uber.org/zap"

	shared "github.com/paladinai/paladinai/internal/dedup"
)

// Store is the minimal interface needed for dedup.
type Store = shared.Store

// Deduplicator uses a Store to track seen alert fingerprints.
type Deduplicator = shared.Deduplicator

// New creates a Deduplicator backed by a redis-compatible Store.
func New(store Store, log *zap.Logger) *Deduplicator { return shared.New(store, log) }
