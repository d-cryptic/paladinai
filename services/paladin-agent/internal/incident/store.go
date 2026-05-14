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
	"strings"
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

// Store is a thread-safe in-memory incident store.
type Store struct {
	mu        sync.RWMutex
	incidents map[string]*Incident
}

// NewStore creates an empty incident store.
func NewStore() *Store {
	return &Store{incidents: make(map[string]*Incident)}
}

// Record stores a new incident record from a processed alert envelope.
// Returns the created incident.
func (s *Store) Record(tenantID, severity, title string, alertCount int, labels map[string]string, result json.RawMessage) *Incident {
	s.mu.Lock()
	defer s.mu.Unlock()
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
func (s *Store) RecordFromEnvelope(env *alert.AlertEnvelope, severity string, result json.RawMessage) *Incident {
	title := env.Labels["alertname"]
	if title == "" {
		title = "Unnamed Incident"
	}
	inc := s.Record(env.TenantID, severity, title, 1, env.Labels, result)
	// Store the raw envelope so replays can re-publish it to NATS unchanged.
	if raw, err := json.Marshal(env); err == nil {
		s.mu.Lock()
		if i, ok := s.incidents[inc.ID]; ok {
			i.RawEnvelope = raw
		}
		s.mu.Unlock()
	}
	return inc
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
		Labels:     src.Labels,
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
func replaySubject(tenantID, fingerprint string) (string, error) {
	if err := alert.ValidateTenantID(tenantID); err != nil {
		return "", fmt.Errorf("replay subject tenant: %w", err)
	}
	if fingerprint == "" {
		return "", fmt.Errorf("replay subject fingerprint must not be empty")
	}
	if strings.ContainsAny(fingerprint, ".*>") {
		return "", fmt.Errorf("replay subject fingerprint must not contain '.', '*', or '>'")
	}
	return fmt.Sprintf("paladin.alerts.raw.%s.%s", tenantID, fingerprint), nil
}

// Handler exposes incident CRUD over HTTP.
type Handler struct {
	store     *Store
	log       *zap.Logger
	replayPub ReplayPublisher
}

// NewHandler creates an incident HTTP handler.
// replayPub may be nil; NoopReplayPublisher is used in that case.
func NewHandler(store *Store, log *zap.Logger) *Handler {
	return &Handler{store: store, log: log, replayPub: NoopReplayPublisher{}}
}

// WithReplayPublisher sets the NATS publisher used to re-ingest replayed alerts.
func (h *Handler) WithReplayPublisher(pub ReplayPublisher) *Handler {
	cp := *h
	cp.replayPub = pub
	return &cp
}

// Routes mounts incident routes on the given chi router.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/incidents", h.list)
	r.Get("/incidents/{incidentID}", h.get)
	r.Post("/incidents/{incidentID}/replay", h.replay)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		jsonErrAgent(w, "INVALID_TENANT", "missing X-Tenant-ID header", http.StatusBadRequest)
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
	go func() {
		src := h.store.Get(incidentID)
		if src == nil || len(src.RawEnvelope) == 0 {
			// No stored envelope: legacy incident or test. Resolve immediately.
			h.store.MarkResolved(replay.ID)
			h.log.Warn("replay: no envelope stored, marking resolved without re-ingest",
				zap.String("replay_id", replay.ID),
			)
			return
		}
		subject, err := replaySubject(tenantID, src.Labels["fingerprint"])
		if err != nil {
			h.log.Error("replay: invalid NATS subject",
				zap.String("replay_id", replay.ID),
				zap.Error(err),
			)
			return
		}
		pubCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

// RunContextKey is used to pass the replay context via chi middleware.
type RunContextKey struct{}
