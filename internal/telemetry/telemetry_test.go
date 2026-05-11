// White-box tests: package telemetry gives access to noopExporter.
// Tests must NOT call t.Parallel() — Init mutates global OTel state
// (otel.SetTracerProvider, otel.SetTextMapPropagator).
package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestInit_EmptyEndpoint_NoError(t *testing.T) {
	p, err := Init(context.Background(), "test-svc", "0.0.1", "", zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, p)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })
}

func TestInit_EmptyEndpoint_ShutdownOK(t *testing.T) {
	p, err := Init(context.Background(), "test-svc", "0.0.1", "", zap.NewNop())
	require.NoError(t, err)
	assert.NoError(t, p.Shutdown(context.Background()))
}

func TestInit_EmptyServiceName_NoError(t *testing.T) {
	// Service name is a semantic-convention attribute — empty is valid at the API level.
	p, err := Init(context.Background(), "", "dev", "", zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })
}

// TestInit_ProviderShutdownDoesNotPanic asserts that calling Shutdown more than
// once does not panic (OTel SDK guarantees idempotency, but we verify it doesn't
// bleed into our Provider wrapper).
func TestInit_ProviderShutdownDoesNotPanic(t *testing.T) {
	p, err := Init(context.Background(), "test-svc", "0.0.1", "", zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, p.Shutdown(context.Background()))
	assert.NotPanics(t, func() {
		_ = p.Shutdown(context.Background())
	})
}

// TestNoopExporter_ExportSpans and TestNoopExporter_Shutdown verify that the
// dev-mode exporter is truly a no-op and never returns an error.
func TestNoopExporter_ExportSpans_NoError(t *testing.T) {
	n := &noopExporter{}
	assert.NoError(t, n.ExportSpans(context.Background(), nil))
}

func TestNoopExporter_Shutdown_NoError(t *testing.T) {
	n := &noopExporter{}
	assert.NoError(t, n.Shutdown(context.Background()))
}
