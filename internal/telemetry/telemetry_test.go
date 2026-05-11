// White-box tests: package telemetry gives access to noopExporter.
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
}

func TestInit_EmptyEndpoint_ShutdownOK(t *testing.T) {
	p, err := Init(context.Background(), "test-svc", "0.0.1", "", zap.NewNop())
	require.NoError(t, err)
	assert.NoError(t, p.Shutdown(context.Background()))
}

func TestInit_EmptyServiceName_NoError(t *testing.T) {
	// Service name is a semantic-convention attribute — empty is valid at the API level.
	_, err := Init(context.Background(), "", "dev", "", zap.NewNop())
	require.NoError(t, err)
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

func TestInit_ProviderCanShutdownTwice(t *testing.T) {
	p, err := Init(context.Background(), "test-svc", "0.0.1", "", zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, p.Shutdown(context.Background()))
	// Second shutdown should not panic — OTel SDK is defined to be idempotent.
	assert.NoError(t, p.Shutdown(context.Background()))
}
