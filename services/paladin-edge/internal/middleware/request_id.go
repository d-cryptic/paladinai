// Package middleware provides HTTP middleware for paladin-edge.
package middleware

import (
	"context"
	"net/http"
	"regexp"

	"github.com/google/uuid"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// safeRequestID matches only characters safe for structured logging (alphanumeric + hyphen + underscore).
// This prevents log injection via a crafted X-Request-ID header.
var safeRequestID = regexp.MustCompile(`^[a-zA-Z0-9\-_]{1,128}$`)

// RequestID attaches a unique request ID to every request and response.
// Uses the X-Request-ID header if present and safe, otherwise generates a UUID v4.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || !safeRequestID.MatchString(id) {
			id = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID retrieves the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}
