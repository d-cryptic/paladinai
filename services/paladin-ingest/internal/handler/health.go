package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const checkTimeout = 2 * time.Second

// Checker tests a single dependency's health.
// Name must be unique across checkers registered with one HealthHandler.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// HealthHandler exposes liveness and readiness probes.
type HealthHandler struct {
	service   string
	checkers  []Checker
	startTime time.Time
}

// NewHealthHandler creates a HealthHandler for the given service name.
// Checkers are run concurrently on /readyz; passing none gives a trivial "ok" readiness.
// Each Checker's Name() should be unique; duplicate names silently overwrite earlier results.
func NewHealthHandler(service string, checkers ...Checker) *HealthHandler {
	return &HealthHandler{
		service:   service,
		checkers:  checkers,
		startTime: time.Now(),
	}
}

// Liveness returns 200 if the process is running.
func (h *HealthHandler) Liveness(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Readiness runs all registered checkers concurrently with a 2-second timeout.
// Returns 200 when all pass, 503 (status: "degraded") when any fail.
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), checkTimeout)
	defer cancel()

	type result struct {
		name string
		err  error
	}

	results := make([]result, len(h.checkers))
	var wg sync.WaitGroup
	for i, c := range h.checkers {
		wg.Add(1)
		go func(i int, c Checker) {
			defer wg.Done()
			results[i] = result{name: c.Name(), err: c.Check(ctx)}
		}(i, c)
	}
	wg.Wait()

	checks := make(map[string]any, len(h.checkers))
	degraded := false
	for _, r := range results {
		if r.err != nil {
			checks[r.name] = r.err.Error()
			degraded = true
		} else {
			checks[r.name] = "ok"
		}
	}

	status := "ok"
	code := http.StatusOK
	if degraded {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  status,
		"uptime":  time.Since(h.startTime).String(),
		"service": h.service,
		"checks":  checks,
	})
}
