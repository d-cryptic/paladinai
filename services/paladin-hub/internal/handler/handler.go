// Package handler implements the HTTP API for the MCP server registry.
// All endpoints are tenant-scoped; tenant ID comes from the X-Tenant-ID header
// set by the auth layer upstream (paladin-edge).
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-hub/internal/registry"
	"github.com/paladinai/paladinai/services/paladin-hub/internal/store"
	"go.uber.org/zap"
)

// Handler wires the registry store to HTTP routes.
type Handler struct {
	store store.Store
	log   *zap.Logger
}

// New creates a Handler with the given store.
func New(s store.Store, log *zap.Logger) *Handler {
	return &Handler{store: s, log: log}
}

// Routes mounts all MCP registry endpoints under the given router.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/mcp/servers", h.register)
	r.Get("/mcp/servers", h.list)
	r.Get("/mcp/servers/{serverID}", h.get)
	r.Delete("/mcp/servers/{serverID}", h.deregister)
	r.Post("/mcp/servers/{serverID}/heartbeat", h.heartbeat)
}

// register handles POST /mcp/servers.
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if err := alert.ValidateTenantID(tenantID); err != nil {
		jsonErr(w, "INVALID_TENANT", "missing or invalid X-Tenant-ID header", http.StatusBadRequest)
		return
	}

	var req registry.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "INVALID_BODY", "request body must be valid JSON", http.StatusBadRequest)
		return
	}
	if err := req.Validate(); err != nil {
		jsonErr(w, "VALIDATION_FAILED", err.Error(), http.StatusUnprocessableEntity)
		return
	}

	server := req.ToServer(tenantID, time.Now().UTC())
	if err := h.store.Upsert(r.Context(), server); err != nil {
		h.log.Error("register: upsert failed", zap.Error(err))
		jsonErr(w, "INTERNAL_ERROR", "failed to register server", http.StatusInternalServerError)
		return
	}

	h.log.Info("mcp server registered",
		zap.String("id", server.ID),
		zap.String("tenant", tenantID),
		zap.String("endpoint", server.Endpoint),
	)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": server})
}

// list handles GET /mcp/servers.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if err := alert.ValidateTenantID(tenantID); err != nil {
		jsonErr(w, "INVALID_TENANT", "missing or invalid X-Tenant-ID header", http.StatusBadRequest)
		return
	}

	servers, err := h.store.List(r.Context(), tenantID)
	if err != nil {
		h.log.Error("list: store error", zap.Error(err))
		jsonErr(w, "INTERNAL_ERROR", "failed to list servers", http.StatusInternalServerError)
		return
	}
	if servers == nil {
		servers = []*registry.MCPServer{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": servers,
		"meta": map[string]int{"total": len(servers)},
	})
}

// get handles GET /mcp/servers/{serverID}.
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if err := alert.ValidateTenantID(tenantID); err != nil {
		jsonErr(w, "INVALID_TENANT", "missing or invalid X-Tenant-ID header", http.StatusBadRequest)
		return
	}

	serverID := chi.URLParam(r, "serverID")
	server, err := h.store.Get(r.Context(), tenantID, serverID)
	if errors.Is(err, store.ErrNotFound) {
		jsonErr(w, "NOT_FOUND", "server not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.log.Error("get: store error", zap.Error(err))
		jsonErr(w, "INTERNAL_ERROR", "failed to retrieve server", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": server})
}

// deregister handles DELETE /mcp/servers/{serverID}.
func (h *Handler) deregister(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if err := alert.ValidateTenantID(tenantID); err != nil {
		jsonErr(w, "INVALID_TENANT", "missing or invalid X-Tenant-ID header", http.StatusBadRequest)
		return
	}

	serverID := chi.URLParam(r, "serverID")
	if err := h.store.Delete(r.Context(), tenantID, serverID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			jsonErr(w, "NOT_FOUND", "server not found", http.StatusNotFound)
			return
		}
		h.log.Error("deregister: store error", zap.Error(err))
		jsonErr(w, "INTERNAL_ERROR", "failed to deregister server", http.StatusInternalServerError)
		return
	}

	h.log.Info("mcp server deregistered",
		zap.String("id", serverID),
		zap.String("tenant", tenantID),
	)
	w.WriteHeader(http.StatusNoContent)
}

// heartbeat handles POST /mcp/servers/{serverID}/heartbeat.
func (h *Handler) heartbeat(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if err := alert.ValidateTenantID(tenantID); err != nil {
		jsonErr(w, "INVALID_TENANT", "missing or invalid X-Tenant-ID header", http.StatusBadRequest)
		return
	}

	serverID := chi.URLParam(r, "serverID")
	if err := h.store.Heartbeat(r.Context(), tenantID, serverID, time.Now().UTC()); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			jsonErr(w, "NOT_FOUND", "server not found", http.StatusNotFound)
			return
		}
		h.log.Error("heartbeat: store error", zap.Error(err))
		jsonErr(w, "INTERNAL_ERROR", "heartbeat failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func jsonErr(w http.ResponseWriter, code, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": msg},
	})
}
