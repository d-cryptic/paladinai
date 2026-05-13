package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemoryHealthHandler_HealthzAlwaysOK(t *testing.T) {
	h := memoryHealthHandler(
		func(context.Context) error { return errors.New("postgres down") },
		func(context.Context) error { return errors.New("valkey down") },
	)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMemoryHealthHandler_ReadyzOK(t *testing.T) {
	h := memoryHealthHandler(
		func(context.Context) error { return nil },
		func(context.Context) error { return nil },
	)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMemoryHealthHandler_ReadyzFailsWhenPostgresUnavailable(t *testing.T) {
	h := memoryHealthHandler(
		func(context.Context) error { return errors.New("postgres down") },
		func(context.Context) error { return nil },
	)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Contains(t, rr.Body.String(), "postgres unavailable")
}

func TestMemoryHealthHandler_ReadyzFailsWhenValkeyUnavailable(t *testing.T) {
	h := memoryHealthHandler(
		func(context.Context) error { return nil },
		func(context.Context) error { return errors.New("valkey down") },
	)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Contains(t, rr.Body.String(), "valkey unavailable")
}
