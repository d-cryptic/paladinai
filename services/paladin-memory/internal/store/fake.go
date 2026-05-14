package store

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	memoryv1 "github.com/paladinai/paladinai/gen/go/memory/v1"
)

// FakeEpisodicStore is an in-memory EpisodicStore for unit tests.
type FakeEpisodicStore struct {
	mu       sync.Mutex
	episodes []*Episode
}

func NewFakeEpisodicStore() *FakeEpisodicStore {
	return &FakeEpisodicStore{}
}

func (f *FakeEpisodicStore) Write(_ context.Context, req *memoryv1.WriteEpisodeRequest) (string, error) {
	if req == nil || req.TenantID == "" || req.IncidentID == "" {
		return "", fmt.Errorf("memory: fake episodic write: tenant_id and incident_id are required")
	}
	id := uuid.NewString()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.episodes = append(f.episodes, &Episode{
		ID:          id,
		TenantID:    req.TenantID,
		IncidentID:  req.IncidentID,
		Fingerprint: req.Fingerprint,
		Summary:     req.Summary,
		RootCause:   req.RootCause,
		Resolution:  req.Resolution,
		Severity:    req.Severity,
		Labels:      cloneEpisodeLabels(req.Labels),
		ValidAt:     time.Now().UTC(),
		RecordedAt:  time.Now().UTC(),
	})
	return id, nil
}

func (f *FakeEpisodicStore) Search(_ context.Context, tenantID, query string, topK int) ([]*Episode, error) {
	if topK <= 0 {
		topK = 10
	}
	q := strings.ToLower(query)
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*Episode
	// Iterate from newest to oldest.
	for i := len(f.episodes) - 1; i >= 0; i-- {
		ep := f.episodes[i]
		if ep.TenantID != tenantID {
			continue
		}
		if q != "" &&
			!strings.Contains(strings.ToLower(ep.Summary), q) &&
			!strings.Contains(strings.ToLower(ep.Fingerprint), q) &&
			!strings.Contains(strings.ToLower(ep.Severity), q) {
			continue
		}
		out = append(out, cloneEpisode(ep))
		if len(out) >= topK {
			break
		}
	}
	return out, nil
}

func (f *FakeEpisodicStore) Delete(_ context.Context, tenantID string, incidentIDs []string) (int64, error) {
	if len(incidentIDs) == 0 {
		return 0, nil
	}
	target := make(map[string]struct{}, len(incidentIDs))
	for _, id := range incidentIDs {
		target[id] = struct{}{}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	kept := f.episodes[:0]
	var deleted int64
	for _, ep := range f.episodes {
		if ep.TenantID == tenantID {
			if _, ok := target[ep.IncidentID]; ok {
				deleted++
				continue
			}
		}
		kept = append(kept, ep)
	}
	f.episodes = kept
	return deleted, nil
}

func cloneEpisode(ep *Episode) *Episode {
	if ep == nil {
		return nil
	}
	cp := *ep
	cp.Labels = cloneEpisodeLabels(ep.Labels)
	return &cp
}

func cloneEpisodeLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return map[string]string{}
	}
	cp := make(map[string]string, len(labels))
	for k, v := range labels {
		cp[k] = v
	}
	return cp
}

// FakeWorkingStore is an in-memory WorkingStore for unit tests.
// TTL is intentionally ignored — tests assert behaviour, not expiry semantics.
type FakeWorkingStore struct {
	mu   sync.Mutex
	data map[string]string
}

func NewFakeWorkingStore() *FakeWorkingStore {
	return &FakeWorkingStore{data: map[string]string{}}
}

func (f *FakeWorkingStore) Get(_ context.Context, tenantID, sessionID, key string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[workingKey(tenantID, sessionID, key)]
	return v, ok, nil
}

func (f *FakeWorkingStore) Set(_ context.Context, tenantID, sessionID, key, value string, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[workingKey(tenantID, sessionID, key)] = value
	return nil
}

func (f *FakeWorkingStore) Scan(_ context.Context, tenantID, sessionID, prefix string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 20
	}
	full := workingPrefix(tenantID, sessionID, prefix)
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for k, v := range f.data {
		if strings.HasPrefix(k, full) {
			out = append(out, v)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}
