// Package incident provides an in-memory incident store and HTTP API for the
// paladin-agent service. It records every processed alert group as an incident
// and exposes REST endpoints for list, show, and replay.
package incident

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/paladinai/paladinai/internal/alert"
	"go.uber.org/zap"
)

// Status represents the lifecycle of an incident.
type Status string

const (
	StatusOpen      Status = "open"
	StatusResolved  Status = "resolved"
	StatusReplaying Status = "replaying"
)

// Incident is a record of a correlated alert group processed by the agent.
type Incident struct {
	ID           string          `json:"id"`
	TenantID     string          `json:"tenant_id"`
	Status       Status          `json:"status"`
	Severity     string          `json:"severity"`
	Title        string          `json:"title"`
	AlertCount   int             `json:"alert_count"`
	Fingerprint  string          `json:"fingerprint,omitempty"`
	TriageResult json.RawMessage `json:"triage_result,omitempty"`
	// RawEnvelope stores the original AlertEnvelope JSON so replays can
	// re-publish to NATS without synthesizing a fake envelope from labels.
	RawEnvelope json.RawMessage   `json:"raw_envelope,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	ReplayOf    string            `json:"replay_of,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// ReplayResult is returned when a replay is triggered.
type ReplayResult struct {
	ReplayID  string    `json:"replay_id"`
	SourceID  string    `json:"source_id"`
	TenantID  string    `json:"tenant_id"`
	StartedAt time.Time `json:"started_at"`
}

// maxStoreSize is the maximum number of incidents kept in memory.
// Old entries are evicted on insertion to prevent OOM in long-running processes.
const maxStoreSize = 10_000

// incidentTTL is the maximum age of an incident kept in memory.
const incidentTTL = 7 * 24 * time.Hour

// Store is a thread-safe in-memory incident store.
type Store struct {
	mu        sync.RWMutex
	incidents map[string]*Incident
}

// NewStore creates an empty incident store.
func NewStore() *Store {
	return &Store{incidents: make(map[string]*Incident)}
}

// evictLocked removes expired entries and, if still over capacity, removes the
// oldest resolved incidents. Must be called with s.mu held for writing.
func (s *Store) evictLocked() {
	cutoff := time.Now().UTC().Add(-incidentTTL)
	for id, inc := range s.incidents {
		if inc.UpdatedAt.Before(cutoff) {
			delete(s.incidents, id)
		}
	}
	// If still over capacity, drop resolved entries arbitrarily.
	for len(s.incidents) >= maxStoreSize {
		for id, inc := range s.incidents {
			if inc.Status == StatusResolved {
				delete(s.incidents, id)
				break
			}
		}
		// Safety: avoid infinite loop if all entries are open.
		if len(s.incidents) >= maxStoreSize {
			for id := range s.incidents {
				delete(s.incidents, id)
				break
			}
		}
	}
}

// Record stores a new incident record from a processed alert envelope.
// Returns the created incident.
func (s *Store) Record(tenantID, severity, title string, alertCount int, labels map[string]string, result json.RawMessage) *Incident {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()
	now := time.Now().UTC()
	inc := &Incident{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		Status:       StatusOpen,
		Severity:     severity,
		Title:        title,
		AlertCount:   alertCount,
		TriageResult: result,
		CreatedAt:    now,
		UpdatedAt:    now,
		Labels:       labels,
	}
	s.incidents[inc.ID] = inc
	return inc
}

// RecordFromEnvelope creates an incident record from an AlertEnvelope.
// The raw envelope JSON is stored for future replay publishing.
// The entire operation (create + set RawEnvelope) runs under a single lock
// to prevent a reader from seeing the incident before the envelope is attached.
func (s *Store) RecordFromEnvelope(env *alert.AlertEnvelope, severity string, result json.RawMessage) *Incident {
	title := env.Labels["alertname"]
	if title == "" {
		title = "Unnamed Incident"
	}

	// Marshal outside the lock — json.Marshal does not touch shared state.
	raw, marshalErr := json.Marshal(env)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()
	now := time.Now().UTC()
	inc := &Incident{
		ID:           uuid.New().String(),
		TenantID:     env.TenantID,
		Status:       StatusOpen,
		Severity:     severity,
		Title:        title,
		AlertCount:   1,
		Fingerprint:  env.Fingerprint,
		TriageResult: result,
		CreatedAt:    now,
		UpdatedAt:    now,
		Labels:       copyLabels(env.Labels),
	}
	if marshalErr == nil {
		inc.RawEnvelope = raw
	}
	s.incidents[inc.ID] = inc
	cp := *inc
	cp.Labels = copyLabels(inc.Labels)
	return &cp
}

// Get returns the incident with the given ID or nil.
func (s *Store) Get(id string) *Incident {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inc := s.incidents[id]
	if inc == nil {
		return nil
	}
	cp := *inc
	cp.Labels = copyLabels(inc.Labels)
	return &cp
}

// List returns incidents for the given tenant, sorted by createdAt descending.
func (s *Store) List(tenantID, status string, limit int) []*Incident {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Incident, 0, len(s.incidents))
	for _, inc := range s.incidents {
		if inc.TenantID != tenantID {
			continue
		}
		if status != "" && string(inc.Status) != status {
			continue
		}
		cp := *inc
		cp.Labels = copyLabels(inc.Labels)
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// StartReplay creates a new incident record cloned from sourceID marked as
// StatusReplaying. The caller is responsible for running the actual replay job.
func (s *Store) StartReplay(sourceID, tenantID string) (*Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src, ok := s.incidents[sourceID]
	if !ok {
		return nil, fmt.Errorf("incident %q not found", sourceID)
	}
	if src.TenantID != tenantID {
		return nil, fmt.Errorf("incident %q not found", sourceID)
	}
	now := time.Now().UTC()
	replay := &Incident{
		ID:         uuid.New().String(),
		TenantID:   src.TenantID,
		Status:     StatusReplaying,
		Severity:   src.Severity,
		Title:      "[REPLAY] " + src.Title,
		AlertCount: src.AlertCount,
		CreatedAt:  now,
		UpdatedAt:  now,
		ReplayOf:   sourceID,
		Labels:     copyLabels(src.Labels),
	}
	s.incidents[replay.ID] = replay
	return replay, nil
}

// MarkResolved sets the incident status to resolved.
func (s *Store) MarkResolved(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if inc, ok := s.incidents[id]; ok {
		inc.Status = StatusResolved
		inc.UpdatedAt = time.Now().UTC()
	}
}

// ReplayPublisher is the narrow interface for re-publishing an alert envelope
// to NATS so the orchestrator pipeline can re-process it. Satisfied by
// internalnats.Client.Publish; NoopReplayPublisher is used when NATS is absent.
type ReplayPublisher interface {
	// Publish sends data to subject and returns the NATS sequence number.
	Publish(ctx context.Context, subject string, data []byte) error
}

// NoopReplayPublisher is a ReplayPublisher that does nothing. Used when NATS
// is not configured (dev/test).
type NoopReplayPublisher struct{}

func (NoopReplayPublisher) Publish(_ context.Context, _ string, _ []byte) error { return nil }

// replaySubject returns the NATS subject for replaying an envelope.
// Republish to the raw ingest topic so dedup+correlate runs again.
func replaySubject(tenantID, fingerprint string) string {
	return fmt.Sprintf("paladin.alerts.raw.%s.%s", tenantID, fingerprint)
}

// Handler exposes incident CRUD over HTTP.
type Handler struct {
	store     *Store
	log       *zap.Logger
	replayPub ReplayPublisher
	wg        sync.WaitGroup // tracks in-flight replay goroutines
}

// NewHandler creates an incident HTTP handler.
// replayPub may be nil; NoopReplayPublisher is used in that case.
func NewHandler(store *Store, log *zap.Logger) *Handler {
	return &Handler{store: store, log: log, replayPub: NoopReplayPublisher{}}
}

// WithReplayPublisher sets the NATS publisher used to re-ingest replayed alerts.
func (h *Handler) WithReplayPublisher(pub ReplayPublisher) *Handler {
	h.replayPub = pub
	return h
}

// Shutdown waits for all in-flight replay goroutines to finish.
// Call this during graceful service shutdown.
func (h *Handler) Shutdown() { h.wg.Wait() }

// Routes mounts incident routes on the given chi router.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/incidents", h.list)
	r.Get("/incidents/{incidentID}", h.get)
	r.Post("/incidents/{incidentID}/replay", h.replay)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID") // read from trusted header set by edge proxy
	if tenantID == "" {
		jsonErrAgent(w, "INVALID_TENANT", "missing tenant context", http.StatusUnauthorized)
		return
	}
	status := r.URL.Query().Get("status")
	limit := 50

	incidents := h.store.List(tenantID, status, limit)
	jsonOK(w, map[string]any{"data": incidents, "total": len(incidents)})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	incidentID, _ := url.PathUnescape(chi.URLParam(r, "incidentID"))

	inc := h.store.Get(incidentID)
	if inc == nil || inc.TenantID != tenantID {
		jsonErrAgent(w, "NOT_FOUND", "incident not found", http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]any{"data": inc})
}

func (h *Handler) replay(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	incidentID, _ := url.PathUnescape(chi.URLParam(r, "incidentID"))

	replay, err := h.store.StartReplay(incidentID, tenantID)
	if err != nil {
		jsonErrAgent(w, "NOT_FOUND", "incident not found", http.StatusNotFound)
		return
	}

	h.log.Info("replay started",
		zap.String("source_id", incidentID),
		zap.String("replay_id", replay.ID),
		zap.String("tenant", tenantID),
	)

	// Re-publish the original alert envelope to the raw ingest NATS topic so
	// the orchestrator pipeline (dedup → correlate → agent) re-processes it.
	// The replay incident is marked resolved after a successful publish.
	// When the envelope is missing (legacy incidents), the replay is a no-op.
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		src := h.store.Get(incidentID)
		if src == nil || len(src.RawEnvelope) == 0 {
			// No stored envelope: legacy incident or test. Resolve immediately.
			h.store.MarkResolved(replay.ID)
			h.log.Warn("replay: no envelope stored, marking resolved without re-ingest",
				zap.String("replay_id", replay.ID),
			)
			return
		}
		subject := replaySubject(tenantID, src.Fingerprint)
		// Detach from the request context so the goroutine is not canceled when the
		// HTTP handler returns, but still bound by a wall-clock timeout.
		pubCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 10*time.Second)
		defer cancel()
		if err := h.replayPub.Publish(pubCtx, subject, src.RawEnvelope); err != nil {
			h.log.Error("replay: NATS publish failed",
				zap.String("replay_id", replay.ID),
				zap.String("subject", subject),
				zap.Error(err),
			)
			// Do not mark resolved: leave as "replaying" so the caller knows it failed.
			return
		}
		h.store.MarkResolved(replay.ID)
		h.log.Info("replay: envelope re-published to NATS",
			zap.String("replay_id", replay.ID),
			zap.String("subject", subject),
		)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(ReplayResult{
		ReplayID:  replay.ID,
		SourceID:  incidentID,
		TenantID:  tenantID,
		StartedAt: replay.CreatedAt,
	})
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonErrAgent(w http.ResponseWriter, code, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": msg},
	})
}

// copyLabels returns a deep copy of a labels map to prevent callers from
// mutating the stored incident's label set.
func copyLabels(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// RunContextKey is used to pass the replay context via chi middleware.
type RunContextKey struct{}
