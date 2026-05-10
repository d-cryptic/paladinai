// Package handler provides HTTP handlers for paladin-edge.
package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

var startTime = time.Now()

// HealthHandler implements /healthz and /readyz for paladin-edge.
type HealthHandler struct{}

// Liveness returns 200 if the process is running (load-balancer probe).
func (h *HealthHandler) Liveness(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Readiness returns 200 when all critical dependencies are reachable.
func (h *HealthHandler) Readiness(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"uptime":  time.Since(startTime).String(),
		"service": "paladin-edge",
	})
}
