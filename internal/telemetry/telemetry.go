// Package telemetry initialises OpenTelemetry tracing and metrics.
package telemetry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DefaultShutdownTimeout bounds OTel exporter flush during service shutdown.
const DefaultShutdownTimeout = 5 * time.Second

// Provider wraps the OTel TracerProvider for lifecycle management.
type Provider struct {
	tp *sdktrace.TracerProvider
	mp *sdkmetric.MeterProvider
}

// Init sets up OTel tracing. Returns a no-op provider if endpoint is empty (dev).
func Init(ctx context.Context, serviceName, serviceVersion, endpoint string, log *zap.Logger) (*Provider, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	var spanExporter sdktrace.SpanExporter
	var metricReader sdkmetric.Reader
	target := normalizeGRPCEndpoint(endpoint)
	if target == "" {
		log.Info("otel endpoint not set; using noop exporter")
		spanExporter = &noopSpanExporter{}
		metricReader = sdkmetric.NewManualReader()
	} else {
		conn, err := grpc.NewClient(target,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return nil, fmt.Errorf("otel grpc conn: %w", err)
		}
		spanExporter, err = otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
		if err != nil {
			return nil, fmt.Errorf("otlp exporter: %w", err)
		}
		metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
		if err != nil {
			return nil, fmt.Errorf("otlp metric exporter: %w", err)
		}
		metricReader = sdkmetric.NewPeriodicReader(metricExporter)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(spanExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricReader),
		sdkmetric.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Info("otel tracing initialised",
		zap.String("service", serviceName),
		zap.String("endpoint", endpoint),
		zap.String("grpc_target", target),
	)
	return &Provider{tp: tp, mp: mp}, nil
}

// Shutdown flushes pending spans and metrics.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	if err := p.tp.Shutdown(ctx); err != nil {
		return err
	}
	return p.mp.Shutdown(ctx)
}

// ShutdownWithTimeout flushes pending spans with a bounded background context.
func (p *Provider) ShutdownWithTimeout(timeout time.Duration) error {
	if timeout <= 0 {
		timeout = DefaultShutdownTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return p.Shutdown(ctx)
}

// TraceFields returns zap fields that correlate logs with the active span.
func TraceFields(ctx context.Context) []zap.Field {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}
	fields := []zap.Field{
		zap.String("trace_id", sc.TraceID().String()),
		zap.String("span_id", sc.SpanID().String()),
	}
	if sc.TraceFlags().IsSampled() {
		fields = append(fields, zap.Bool("trace_sampled", true))
	}
	return fields
}

// CommonAttributes returns stable Paladin dimensions for spans and metrics.
func CommonAttributes(tenantID, incidentID, severity string) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 3)
	if tenantID != "" {
		attrs = append(attrs, attribute.String("paladin.tenant_id", tenantID))
	}
	if incidentID != "" {
		attrs = append(attrs, attribute.String("paladin.incident_id", incidentID))
	}
	if severity != "" {
		attrs = append(attrs, attribute.String("paladin.severity", severity))
	}
	return attrs
}

// Attrs converts attribute key-values to metric options.
func Attrs(attrs ...attribute.KeyValue) metric.MeasurementOption {
	return metric.WithAttributes(attrs...)
}

func normalizeGRPCEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	return strings.TrimSuffix(endpoint, "/")
}

// noopSpanExporter discards all spans when no OTel endpoint is configured.
type noopSpanExporter struct{}

func (n *noopSpanExporter) ExportSpans(_ context.Context, _ []sdktrace.ReadOnlySpan) error {
	return nil
}
func (n *noopSpanExporter) Shutdown(_ context.Context) error { return nil }
