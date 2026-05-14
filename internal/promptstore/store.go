// Package promptstore provides versioned, hot-reloadable system prompts for
// PaladinAI agents. Prompts are stored in the prompt_versions table; one row
// per (agent_name, model_id) is marked is_active = true. The Store keeps an
// in-memory cache and refreshes it on a configurable interval, falling back
// to built-in defaults when the DB is unreachable or has no entry.
package promptstore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PromptVersion represents one row in the prompt_versions table.
type PromptVersion struct {
	ID        string
	AgentName string
	ModelID   string
	Version   int
	Content   string
	IsActive  bool
	CreatedAt time.Time
	Notes     string
}

// Querier is the minimal DB interface the Store needs. It is satisfied by
// pgxpool.Pool, *pgx.Conn, and hand-written fakes in tests.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
}

// Rows abstracts a pgx-like row iterator.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// WildcardModel is used when a prompt applies to any model.
const WildcardModel = "*"

// DefaultRefreshInterval is the default poll interval for hot-reload.
const DefaultRefreshInterval = 60 * time.Second

// Store retrieves active prompts from the database with a local cache.
// Hot-reload: a background goroutine refreshes every RefreshInterval.
type Store struct {
	db              Querier
	log             *zap.Logger
	mu              sync.RWMutex
	refreshMu       sync.Mutex
	refreshCancel   context.CancelFunc
	cache           map[string]string // key: "agentName:modelID" → content
	RefreshInterval time.Duration
}

// New creates a Store and loads initial prompts.
// If db is nil or the initial load fails, the Store falls back to built-in
// defaults (graceful degradation). The error is logged but not returned so
// callers can treat New as best-effort.
func New(ctx context.Context, db Querier, log *zap.Logger) (*Store, error) {
	if log == nil {
		log = zap.NewNop()
	}
	s := &Store{
		db:              db,
		log:             log,
		cache:           make(map[string]string),
		RefreshInterval: DefaultRefreshInterval,
	}
	if db == nil {
		log.Warn("promptstore: no DB provided, using built-in defaults only")
		return s, nil
	}
	if err := s.refresh(ctx); err != nil {
		log.Warn("promptstore: initial refresh failed, using built-in defaults", zap.Error(err))
		// Do not fail construction — graceful degradation per stage 4.5 spec.
	}
	return s, nil
}

// Get returns the active prompt for the given agent and model.
//
// Lookup order:
//  1. cache[agentName:modelID]            — exact match
//  2. cache[agentName:*]                  — wildcard model fallback
//  3. defaults[agentName:modelID]         — built-in exact
//  4. defaults[agentName:*]               — built-in wildcard
//  5. ""                                  — unknown agent (caller must handle)
func (s *Store) Get(agentName, modelID string) string {
	s.mu.RLock()
	if v, ok := s.cache[cacheKey(agentName, modelID)]; ok {
		s.mu.RUnlock()
		return v
	}
	if v, ok := s.cache[cacheKey(agentName, WildcardModel)]; ok {
		s.mu.RUnlock()
		return v
	}
	s.mu.RUnlock()

	if v, ok := defaults[cacheKey(agentName, modelID)]; ok {
		return v
	}
	if v, ok := defaults[cacheKey(agentName, WildcardModel)]; ok {
		return v
	}
	return ""
}

// StartRefresh starts a background goroutine that refreshes prompts every
// s.RefreshInterval. Cancel ctx or call StopRefresh to stop. Repeated calls
// replace the previous loop so callers cannot accidentally leak refreshers.
func (s *Store) StartRefresh(ctx context.Context) {
	if s.db == nil {
		return
	}
	interval := s.RefreshInterval
	if interval <= 0 {
		s.log.Warn("promptstore: non-positive refresh interval, using default",
			zap.Duration("interval", interval),
			zap.Duration("default", DefaultRefreshInterval),
		)
		interval = DefaultRefreshInterval
	}

	s.refreshMu.Lock()
	if s.refreshCancel != nil {
		s.refreshCancel()
	}
	refreshCtx, cancel := context.WithCancel(ctx)
	s.refreshCancel = cancel
	s.refreshMu.Unlock()

	go s.refreshLoop(refreshCtx, interval)
}

// StopRefresh stops the active background refresh loop, if one is running.
func (s *Store) StopRefresh() {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	if s.refreshCancel != nil {
		s.refreshCancel()
		s.refreshCancel = nil
	}
}

func (s *Store) refreshLoop(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.refresh(ctx); err != nil {
				s.log.Warn("promptstore: refresh failed", zap.Error(err))
			}
		}
	}
}

// refresh reloads all active prompts from DB into the cache.
func (s *Store) refresh(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	const q = `SELECT agent_name, model_id, content
		FROM prompt_versions
		WHERE is_active = true`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return fmt.Errorf("promptstore: query: %w", err)
	}
	defer rows.Close()

	next := make(map[string]string, len(s.cache))
	for rows.Next() {
		var agent, model, content string
		if err := rows.Scan(&agent, &model, &content); err != nil {
			return fmt.Errorf("promptstore: scan: %w", err)
		}
		next[cacheKey(agent, model)] = content
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("promptstore: rows: %w", err)
	}

	s.mu.Lock()
	s.cache = next
	s.mu.Unlock()
	return nil
}

// setCache is used by tests to inject prompt content without a DB.
func (s *Store) setCache(agent, model, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[cacheKey(agent, model)] = content
}

func cacheKey(agent, model string) string { return agent + ":" + model }
