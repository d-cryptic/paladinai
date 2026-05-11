// Package handler contains the HTTP handlers for paladin-ingest.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/normalizer"
	"go.uber.org/zap"
)

// Publisher abstracts the NATS publishing dependency.
type Publisher interface {
	PublishAlert(ctx context.Context, env alert.AlertEnvelope) error
}

// Deduplicator abstracts the dedup dependency.
type Deduplicator interface {
	IsDuplicate(ctx context.Context, env *alert.AlertEnvelope) (bool, error)
	Reset(ctx context.Context, tenantID, fingerprint string) error
}

// WebhookHandler handles inbound webhooks from monitoring integrations.
type WebhookHandler struct {
	pub   Publisher
	dedup Deduplicator
	log   *zap.Logger
}

// NewWebhookHandler creates a configured WebhookHandler.
func NewWebhookHandler(pub Publisher, dedup Deduplicator, log *zap.Logger) *WebhookHandler {
	return &WebhookHandler{pub: pub, dedup: dedup, log: log}
}

// Routes returns a chi Router with all webhook routes.
// Path pattern: /webhook/{integration}/{tenantID}
func (h *WebhookHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/alertmanager/{tenantID}", h.alertmanager)
	return r
}

// alertmanager handles POST /webhook/alertmanager/{tenantID}
func (h *WebhookHandler) alertmanager(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id required")
		return
	}

	// Validate tenant ID to prevent NATS subject injection
	if err := alert.ValidateTenantID(tenantID); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid tenant_id: %s", err))
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 8*1024*1024)) // 8MB max
	if err != nil {
		h.log.Error("read webhook body", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "read body failed")
		return
	}

	envelopes, err := normalizer.NormalizeAlertmanager(tenantID, json.RawMessage(body), h.log)
	if err != nil {
		h.log.Warn("normalise alertmanager payload",
			zap.String("tenant", tenantID),
			zap.Error(err),
		)
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid payload: %s", err))
		return
	}

	// Process all envelopes, collecting failures.
	// We never return 500 mid-batch because Alertmanager will retry the full
	// batch, causing duplicates for already-published alerts.
	published, suppressed, failed := 0, 0, 0

	for i := range envelopes {
		env := envelopes[i]

		isDup, dupErr := h.dedup.IsDuplicate(r.Context(), &env)
		if dupErr != nil {
			// Dedup failure is non-fatal — let the alert through to avoid
			// suppressing real incidents. Observable via the failed counter.
			h.log.Error("dedup check failed",
				zap.String("fingerprint", env.Fingerprint),
				zap.Error(dupErr),
			)
			failed++
		} else if isDup {
			h.log.Debug("suppressed duplicate",
				zap.String("fingerprint", env.Fingerprint),
				zap.String("tenant", tenantID),
			)
			suppressed++
			continue
		}

		// Clear dedup key for resolved alerts so the same alert can re-fire.
		if env.Status == alert.StatusResolved {
			_ = h.dedup.Reset(r.Context(), env.TenantID, env.Fingerprint)
		}

		if pubErr := h.pub.PublishAlert(r.Context(), env); pubErr != nil {
			h.log.Error("publish alert",
				zap.String("tenant", tenantID),
				zap.String("fingerprint", env.Fingerprint),
				zap.Error(pubErr),
			)
			failed++
			continue
		}
		published++
	}

	h.log.Info("webhook processed",
		zap.String("tenant", tenantID),
		zap.String("integration", "alertmanager"),
		zap.Int("total", len(envelopes)),
		zap.Int("published", published),
		zap.Int("suppressed", suppressed),
		zap.Int("failed", failed),
	)

	// Return 202 always — even partial failure is reported in the body.
	// Alertmanager expects 2xx and will retry on non-2xx; partial publish
	// info is surfaced here for observability.
	code := http.StatusAccepted
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "accepted",
		"published":  published,
		"suppressed": suppressed,
		"failed":     failed,
		"total":      len(envelopes),
	})
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
