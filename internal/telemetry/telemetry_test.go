// White-box tests: package telemetry gives access to noopSpanExporter.
// Tests must NOT call t.Parallel() — Init mutates global OTel state
// (otel.SetTracerProvider, otel.SetTextMapPropagator).
package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
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

func TestProvider_ShutdownWithTimeout(t *testing.T) {
	p, err := Init(context.Background(), "test-svc", "0.0.1", "", zap.NewNop())
	require.NoError(t, err)
	assert.NoError(t, p.ShutdownWithTimeout(time.Second))
}

func TestProvider_ShutdownWithTimeoutUsesDefaultForInvalidTimeout(t *testing.T) {
	p, err := Init(context.Background(), "test-svc", "0.0.1", "", zap.NewNop())
	require.NoError(t, err)
	assert.NoError(t, p.ShutdownWithTimeout(0))
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
	n := &noopSpanExporter{}
	assert.NoError(t, n.ExportSpans(context.Background(), nil))
}

func TestNoopExporter_Shutdown_NoError(t *testing.T) {
	n := &noopSpanExporter{}
	assert.NoError(t, n.Shutdown(context.Background()))
}

// TestInit_WithEndpoint exercises the gRPC exporter path.
// grpc.NewClient is lazy — it does not block on a live connection.
// otlptracegrpc.New may fail or succeed depending on the environment;
// we accept either outcome and just verify the function does not panic.
func TestInit_WithEndpoint_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		p, err := Init(context.Background(), "test-svc", "0.0.1", "localhost:4317", zap.NewNop())
		if err == nil && p != nil {
			_ = p.Shutdown(context.Background())
		}
	})
}

func TestNormalizeGRPCEndpoint(t *testing.T) {
	tests := map[string]string{
		"":                            "",
		" localhost:4317 ":            "localhost:4317",
		"http://otel-collector:4317":  "otel-collector:4317",
		"https://otel-collector:4317": "otel-collector:4317",
		"http://otel-collector:4317/": "otel-collector:4317",
	}

	for input, want := range tests {
		assert.Equal(t, want, normalizeGRPCEndpoint(input))
	}
}

func TestTraceFields_EmptyContext(t *testing.T) {
	assert.Empty(t, TraceFields(context.Background()))
}

func TestTraceFields_ActiveSpanContext(t *testing.T) {
	tid, err := trace.TraceIDFromHex("00112233445566778899aabbccddeeff")
	require.NoError(t, err)
	sid, err := trace.SpanIDFromHex("0011223344556677")
	require.NoError(t, err)
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
	})

	fields := TraceFields(trace.ContextWithSpanContext(context.Background(), sc))
	require.Len(t, fields, 3)
	assert.Equal(t, "trace_id", fields[0].Key)
	assert.Equal(t, "span_id", fields[1].Key)
	assert.Equal(t, "trace_sampled", fields[2].Key)
}

func TestCommonAttributes_OnlyNonEmptyValues(t *testing.T) {
	attrs := CommonAttributes("tenant-a", "", "P1")
	require.Len(t, attrs, 2)
	assert.Equal(t, "paladin.tenant_id", string(attrs[0].Key))
	assert.Equal(t, "paladin.severity", string(attrs[1].Key))
}
