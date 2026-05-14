// Package middleware provides shared HTTP middleware for all PaladinAI services.
package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/auth"
)

// JWTMiddleware validates Bearer tokens and injects tenant context.
// Requests without a valid token receive 401. Expired tokens receive 401.
// If X-Tenant-ID header is present it must match the token's tenant_id claim.
func JWTMiddleware(secret []byte, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, err := extractBearer(r)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "missing or malformed Authorization header")
				return
			}

			claims, err := auth.ParseClaims(raw, secret)
			if err != nil {
				log.Debug("jwt validation failed", zap.Error(err))
				writeJSONError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			if claims.TenantID == "" {
				writeJSONError(w, http.StatusUnauthorized, "token missing tenant_id claim")
				return
			}

			// Optional header must agree with the claim — prevents header spoofing.
			if hdr := r.Header.Get("X-Tenant-ID"); hdr != "" && hdr != claims.TenantID {
				log.Warn("X-Tenant-ID header mismatch",
					zap.String("header", hdr),
					zap.String("claim", claims.TenantID))
				writeJSONError(w, http.StatusForbidden, "X-Tenant-ID does not match token")
				return
			}

			ctx := r.Context()
			ctx = auth.WithTenantID(ctx, claims.TenantID)
			ctx = auth.WithUserID(ctx, claims.Subject)
			ctx = auth.WithRoles(ctx, claims.Roles)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

var errMissingAuth = errors.New("missing authorization header")

func extractBearer(r *http.Request) (string, error) {
	hdr := r.Header.Get("Authorization")
	if hdr != "" {
		parts := strings.SplitN(hdr, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			return "", errMissingAuth
		}
		tok := strings.TrimSpace(parts[1])
		if tok == "" {
			return "", errMissingAuth
		}
		return tok, nil
	}

	for _, protocol := range strings.Split(r.Header.Get("Sec-WebSocket-Protocol"), ",") {
		protocol = strings.TrimSpace(protocol)
		if token, ok := strings.CutPrefix(protocol, "paladinai.jwt."); ok && token != "" {
			return token, nil
		}
	}

	return "", errMissingAuth
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	//nolint:errcheck
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": http.StatusText(code), "message": msg},
	})
}
