package agent

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/cache"
)

const (
	defaultL1TTL      = 30 * time.Minute
	cacheWriteTimeout = 2 * time.Second
)

// Triager is the narrow interface for anything that triages an alert.
// Satisfied by TriageAgent and CachedTriager (and test fakes in worker).
type Triager interface {
	Triage(ctx context.Context, env *alert.AlertEnvelope) (*TriageResult, error)
}

// CachedTriager wraps a Triager with a write-through L1 exact cache.
// Identical alerts (same fingerprint + correlation_id + title + severity +
// labels + annotations + model) return cached results without an LLM call.
type CachedTriager struct {
	inner   Triager
	l1      cache.L1Cache
	modelID string
	ttl     time.Duration
	log     *zap.Logger
}

// NewCachedTriager creates a Triager that checks the L1 cache before
// delegating to the inner Triager.
func NewCachedTriager(inner Triager, l1 cache.L1Cache, modelID string, log *zap.Logger) *CachedTriager {
	return &CachedTriager{
		inner:   inner,
		l1:      l1,
		modelID: modelID,
		ttl:     defaultL1TTL,
		log:     log,
	}
}

// Triage checks L1 cache first; on miss calls inner.Triage and caches result.
func (c *CachedTriager) Triage(ctx context.Context, env *alert.AlertEnvelope) (*TriageResult, error) {
	req := cache.TriageRequest{
		Fingerprint:   env.Fingerprint,
		CorrelationID: env.CorrelationID,
		Title:         env.Title,
		Severity:      string(env.Severity),
		Labels:        env.Labels,
		Annotations:   env.Annotations,
		Description:   env.Description,
		ModelID:       c.modelID,
	}

	key, err := req.MarshalKey()
	if err != nil {
		c.log.Warn("l1 cache: key derivation failed, bypassing cache", zap.Error(err))
		return c.inner.Triage(ctx, env)
	}

	// L1 lookup.
	cached, getErr := c.l1.Get(ctx, key)
	if getErr != nil {
		// Log cache tier errors so on-call can detect Valkey degradation.
		c.log.Warn("l1 cache: get error, proceeding without cache",
			zap.String("fingerprint", env.Fingerprint),
			zap.Error(getErr),
		)
	} else if cached != nil {
		var result TriageResult
		if err := json.Unmarshal(cached, &result); err == nil {
			c.log.Debug("l1 cache hit",
				zap.String("fingerprint", env.Fingerprint),
				zap.String("key_prefix", key[len("llm:l1:"):len("llm:l1:")+16]),
			)
			return &result, nil
		}
		// Corrupted or stale format — log and fall through to LLM.
		c.log.Warn("l1 cache: unmarshal failed for cached entry, bypassing",
			zap.String("fingerprint", env.Fingerprint),
			zap.Error(err),
		)
	}

	// Cache miss — call the real agent.
	result, err := c.inner.Triage(ctx, env)
	if err != nil {
		return nil, err
	}

	// Write back to L1 using a detached context so a cancelled request ctx
	// does not abort the cache write after a successful (expensive) LLM call.
	b, merr := json.Marshal(result)
	if merr != nil {
		c.log.Warn("l1 cache: marshal failed, skipping write", zap.Error(merr))
		return result, nil
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cacheWriteTimeout)
	defer cancel()
	if serr := c.l1.Set(writeCtx, key, b, c.ttl); serr != nil {
		c.log.Warn("l1 cache: set failed", zap.Error(serr))
	}

	return result, nil
}
