// Package proxy provides a reverse-proxy handler that forwards requests to
// backend services, injecting tenant context from JWT claims.
package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/auth"
)

// Handler is a thin reverse-proxy that forwards requests to a fixed backend URL.
// It strips the Authorization header, overwrites X-Tenant-ID and X-User-ID from
// validated JWT claims in context, and optionally strips a path prefix before forwarding.
type Handler struct {
	rp  *httputil.ReverseProxy
	log *zap.Logger
}

// New creates a Handler that proxies to target.
// target must have scheme "http" or "https" and a non-empty host.
// prefix is a URL path segment to remove before forwarding (e.g. "/api/v1/mcp" →
// "/api/v1/mcp/servers" becomes "/servers" on the backend). Only the exact prefix
// followed by "/" or end-of-path is stripped; partial matches are rejected.
// Pass an empty string to forward the path unchanged.
func New(target *url.URL, prefix string, log *zap.Logger) *Handler {
	rp := httputil.NewSingleHostReverseProxy(target)

	director := rp.Director
	rp.Director = func(req *http.Request) {
		director(req)

		// Strip the mount prefix so /api/v1/mcp/servers becomes /servers
		// on the backend. Guard against partial segment matches (/api/v1/mcpevil).
		if prefix != "" {
			p := req.URL.Path
			if p == prefix {
				req.URL.Path = "/"
			} else if strings.HasPrefix(p, prefix+"/") {
				req.URL.Path = p[len(prefix):]
			}
			req.URL.RawPath = ""
		}

		// Do not forward credentials; the backend trusts the edge's injected headers.
		req.Header.Del("Authorization")

		// Always delete first to prevent client-supplied header injection when the
		// JWT middleware does not populate context (e.g. future unauthenticated paths
		// wired through this proxy).
		req.Header.Del("X-Tenant-ID")
		req.Header.Del("X-User-ID")
		if tenantID, ok := auth.TenantIDFromContext(req.Context()); ok {
			req.Header.Set("X-Tenant-ID", tenantID)
		}
		if userID, ok := auth.UserIDFromContext(req.Context()); ok {
			req.Header.Set("X-User-ID", userID)
		}
	}

	rp.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		log.Error("proxy upstream error", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		//nolint:errcheck
		fmt.Fprint(w, `{"error":{"code":"UPSTREAM_UNAVAILABLE","message":"upstream service unavailable"}}`)
	}

	return &Handler{rp: rp, log: log}
}

// Validate checks that target is usable as a backend URL.
func Validate(target *url.URL) error {
	if target.Scheme != "http" && target.Scheme != "https" {
		return fmt.Errorf("proxy target scheme must be http or https, got %q", target.Scheme)
	}
	if target.Host == "" {
		return fmt.Errorf("proxy target must include a host")
	}
	return nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.rp.ServeHTTP(w, r)
}
