package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/auth"
	"github.com/paladinai/paladinai/services/paladin-auth/internal/store"
)

// validSlug enforces lowercase alphanumeric with hyphens, 2-64 chars.
// Must start and end with an alphanumeric character.
var validSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$`)

// allowedRoles is the whitelist of roles callers may request.
var allowedRoles = map[string]bool{"admin": true, "viewer": true}

const maxBodyBytes = 64 * 1024 // 64 KB

// Handler provides the paladin-auth HTTP endpoints.
type Handler struct {
	store       store.Store
	jwtSecret   []byte
	adminSecret string // secret required for management + token-issue endpoints
	tokenTTL    time.Duration
	log         *zap.Logger
}

func New(s store.Store, jwtSecret []byte, adminSecret string, tokenTTL time.Duration, log *zap.Logger) *Handler {
	return &Handler{
		store:       s,
		jwtSecret:   jwtSecret,
		adminSecret: adminSecret,
		tokenTTL:    tokenTTL,
		log:         log,
	}
}

// Register mounts all routes onto the provided router.
// All routes are protected by the admin-secret middleware.
func (h *Handler) Register(r chi.Router) {
	// All admin management routes require X-Admin-Secret header.
	r.Group(func(r chi.Router) {
		r.Use(h.requireAdminSecret)
		r.Post("/tenants", h.createTenant)
		r.Get("/tenants", h.listTenants)
		r.Get("/tenants/{id}", h.getTenant)
		r.Post("/tenants/{id}/suspend", h.suspendTenant)
		r.Post("/tenants/{id}/resume", h.resumeTenant)
		r.Post("/tokens", h.issueToken)
	})
}

// requireAdminSecret gates endpoints to callers presenting the correct secret.
func (h *Handler) requireAdminSecret(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.adminSecret == "" {
			// Admin secret not configured — reject all requests.
			writeError(w, http.StatusServiceUnavailable, "admin secret not configured")
			return
		}
		provided := r.Header.Get("X-Admin-Secret")
		if provided != h.adminSecret {
			writeError(w, http.StatusUnauthorized, "invalid admin secret")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// POST /tenants
func (h *Handler) createTenant(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if !validSlug.MatchString(body.Slug) {
		writeError(w, http.StatusBadRequest, "slug must be 2-64 lowercase alphanumeric chars or hyphens")
		return
	}
	if body.Name == "" || len(body.Name) > 128 {
		writeError(w, http.StatusBadRequest, "name is required and must be ≤128 chars")
		return
	}

	t, err := h.store.Create(r.Context(), body.Slug, body.Name)
	if errors.Is(err, store.ErrAlreadyExists) {
		writeError(w, http.StatusConflict, "tenant slug already exists")
		return
	}
	if err != nil {
		h.log.Error("create tenant", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"tenant": t})
}

// GET /tenants
func (h *Handler) listTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := h.store.List(r.Context())
	if err != nil {
		h.log.Error("list tenants", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenants": tenants})
}

// GET /tenants/{id}
func (h *Handler) getTenant(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := h.store.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}
	if err != nil {
		h.log.Error("get tenant", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenant": t})
}

// POST /tenants/{id}/suspend
func (h *Handler) suspendTenant(w http.ResponseWriter, r *http.Request) {
	h.setState(w, r, store.TenantStateSuspended)
}

// POST /tenants/{id}/resume
func (h *Handler) resumeTenant(w http.ResponseWriter, r *http.Request) {
	h.setState(w, r, store.TenantStateActive)
}

func (h *Handler) setState(w http.ResponseWriter, r *http.Request, state store.TenantState) {
	id := chi.URLParam(r, "id")
	t, err := h.store.SetState(r.Context(), id, state)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}
	if err != nil {
		h.log.Error("set tenant state", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenant": t})
}

// POST /tokens — issues a JWT for a tenant (requires X-Admin-Secret).
// Roles are validated against the server-defined allowlist; unknown roles
// are rejected to prevent privilege escalation.
func (h *Handler) issueToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TenantID string   `json:"tenant_id"`
		UserID   string   `json:"user_id"`
		Roles    []string `json:"roles"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if body.TenantID == "" || body.UserID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id and user_id are required")
		return
	}

	// Default to viewer role when none specified.
	if len(body.Roles) == 0 {
		body.Roles = []string{"viewer"}
	}
	// Validate roles against server-side whitelist.
	for _, role := range body.Roles {
		if !allowedRoles[role] {
			writeError(w, http.StatusBadRequest, "unknown role: "+role)
			return
		}
	}

	// Verify tenant exists and is active.
	t, err := h.store.Get(r.Context(), body.TenantID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if t.State == store.TenantStateSuspended {
		writeError(w, http.StatusForbidden, "tenant is suspended")
		return
	}

	token, err := auth.IssueToken(h.jwtSecret, body.TenantID, body.UserID, body.Roles, h.tokenTTL)
	if err != nil {
		h.log.Error("issue token", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to sign token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_in": int(h.tokenTTL.Seconds()),
		"token_type": "Bearer",
	})
}

// decodeBody reads and JSON-decodes the request body with size and field restrictions.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	//nolint:errcheck
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{
		"error": map[string]string{
			"code":    http.StatusText(code),
			"message": msg,
		},
	})
}
