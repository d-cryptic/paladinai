package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

var startTime = time.Now()

// HealthHandler returns health and readiness check handlers.
type HealthHandler struct{}

// Liveness returns 200 if the process is running.
func (h *HealthHandler) Liveness(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Readiness returns 200 when all dependencies are healthy.
// TODO: add NATS + Valkey ping checks.
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"uptime":  time.Since(startTime).String(),
		"service": "paladin-ingest",
	})
}
