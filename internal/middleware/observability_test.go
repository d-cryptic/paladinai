package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestObservabilityPreservesStatusCode(t *testing.T) {
	r := chi.NewRouter()
	r.Use(Observability("test-service", zap.NewNop()))
	r.Get("/boom", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/boom", nil))

	assert.Equal(t, http.StatusTeapot, rr.Code)
}

func TestObservabilityDefaultsImplicitStatusToOK(t *testing.T) {
	r := chi.NewRouter()
	r.Use(Observability("test-service", zap.NewNop()))
	r.Get("/ok", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/ok", nil))

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestStatusRecorderUnwrapsResponseWriter(t *testing.T) {
	rr := httptest.NewRecorder()
	rec := &statusRecorder{ResponseWriter: rr, status: http.StatusOK}

	assert.Same(t, rr, rec.Unwrap())
}
