// Package incident provides an in-memory incident store and HTTP API for the
// paladin-agent service. It records every processed alert group as an incident
// and exposes REST endpoints for list, show, and replay.
package incident

import (
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
	StatusOpen     Status = "open"
	StatusResolved Status = "resolved"
	StatusReplaying Status = "replaying"
)

// Incident is a record of a correlated alert group processed by the agent.
type Incident struct {
	ID           string              `json:"id"`
	TenantID     string              `json:"tenant_id"`
	Status       Status              `json:"status"`
	Severity     string              `json:"severity"`
	Title        string              `json:"title"`
	AlertCount   int                 `json:"alert_count"`
	TriageResult json.RawMessage     `json:"triage_result,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
	ReplayOf     string              `json:"replay_of,omitempty"`
	Labels       map[string]string   `json:"labels,omitempty"`
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
func (s *Store) RecordFromEnvelope(env *alert.AlertEnvelope, severity string, result json.RawMessage) *Incident {
	title := env.Labels["alertname"]
	if title == "" {
		title = "Unnamed Incident"
	}
	return s.Record(env.TenantID, severity, title, 1, env.Labels, result)
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

// Handler exposes incident CRUD over HTTP.
type Handler struct {
	store *Store
	log   *zap.Logger
}

// NewHandler creates an incident HTTP handler.
func NewHandler(store *Store, log *zap.Logger) *Handler {
	return &Handler{store: store, log: log}
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

	// Fire-and-forget: in production this would submit to Hatchet. Here we
	// immediately mark it resolved as a synchronous stub so the CLI can poll.
	go func() {
		// Simulate async replay completing after 2s.
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		<-timer.C
		h.store.MarkResolved(replay.ID)
		h.log.Info("replay completed", zap.String("replay_id", replay.ID))
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
