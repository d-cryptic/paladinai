// Package telemetry initialises OpenTelemetry tracing and metrics.
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Provider wraps the OTel TracerProvider for lifecycle management.
type Provider struct {
	tp  *sdktrace.TracerProvider
	log *zap.Logger
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

	var exporter sdktrace.SpanExporter
	if endpoint == "" {
		log.Info("otel endpoint not set — using noop exporter")
		exporter = &noopExporter{}
	} else {
		conn, err := grpc.NewClient(endpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return nil, fmt.Errorf("otel grpc conn: %w", err)
		}
		exporter, err = otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
		if err != nil {
			return nil, fmt.Errorf("otlp exporter: %w", err)
		}
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Info("otel tracing initialised",
		zap.String("service", serviceName),
		zap.String("endpoint", endpoint),
	)
	return &Provider{tp: tp, log: log}, nil
}

// Shutdown flushes pending spans.
func (p *Provider) Shutdown(ctx context.Context) error {
	return p.tp.Shutdown(ctx)
}

// noopExporter discards all spans (used when no OTel endpoint configured).
type noopExporter struct{}

func (n *noopExporter) ExportSpans(_ context.Context, _ []sdktrace.ReadOnlySpan) error { return nil }
func (n *noopExporter) Shutdown(_ context.Context) error                               { return nil }
