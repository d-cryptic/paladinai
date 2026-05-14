package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/paladinai/paladinai/internal/qdrant"
	memcfg "github.com/paladinai/paladinai/services/paladin-memory/config"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
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

func TestMemoryEmbedder_UsesHTTPEmbedderWhenAPIKeySet(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "or-key")
	t.Setenv("LLM_GATEWAY_URL", "http://llm.local/v1")
	t.Setenv("EMBED_MODEL", "bge-test")

	embedder, err := memoryEmbedder(&memcfg.Config{Env: "production"}, zap.NewNop())

	assert.NoError(t, err)
	_, ok := embedder.(*qdrant.HTTPEmbedder)
	assert.True(t, ok, "expected HTTP embedder")
}

func TestMemoryEmbedder_AllowsStubInDevelopment(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")

	embedder, err := memoryEmbedder(&memcfg.Config{Env: "development"}, zap.NewNop())

	assert.NoError(t, err)
	_, ok := embedder.(*qdrant.StubEmbedder)
	assert.True(t, ok, "expected stub embedder")
}

func TestMemoryEmbedder_RejectsStubInProduction(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")

	embedder, err := memoryEmbedder(&memcfg.Config{Env: "production"}, zap.NewNop())

	assert.Nil(t, embedder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OPENROUTER_API_KEY")
}

func TestMemoryEmbedder_ExplicitlyAllowsProductionStub(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")

	embedder, err := memoryEmbedder(&memcfg.Config{
		Env:               "production",
		AllowStubEmbedder: true,
	}, zap.NewNop())

	assert.NoError(t, err)
	_, ok := embedder.(*qdrant.StubEmbedder)
	assert.True(t, ok, "expected stub embedder")
}
