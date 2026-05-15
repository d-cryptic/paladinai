package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/paladinai/paladinai/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

var (
	httpTracer        = otel.Tracer("github.com/paladinai/paladinai/internal/middleware")
	httpMeter         = otel.Meter("github.com/paladinai/paladinai/internal/middleware")
	httpRequestCount  metric.Int64Counter
	httpRequestTime   metric.Float64Histogram
	httpInstrumentErr error
)

func init() {
	httpRequestCount, httpInstrumentErr = httpMeter.Int64Counter(
		"paladin_http_server_requests_total",
		metric.WithDescription("Total inbound HTTP requests"),
	)
	if httpInstrumentErr != nil {
		return
	}
	httpRequestTime, httpInstrumentErr = httpMeter.Float64Histogram(
		"paladin_http_server_duration_seconds",
		metric.WithDescription("Inbound HTTP request duration"),
		metric.WithUnit("s"),
	)
}

// Observability records inbound HTTP spans, metrics, and trace-correlated error logs.
func Observability(service string, log *zap.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			spanName := r.Method + " " + r.URL.Path
			ctx, span := httpTracer.Start(r.Context(), spanName)
			defer span.End()

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(rec, r.WithContext(ctx))

			route := routePattern(r)
			status := rec.status
			attrs := []attribute.KeyValue{
				attribute.String("service.name", service),
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", route),
				attribute.Int("http.response.status_code", status),
			}
			span.SetAttributes(attrs...)
			if status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(status))
				log.Warn("http request failed",
					append(telemetry.TraceFields(ctx),
						zap.String("service", service),
						zap.String("method", r.Method),
						zap.String("route", route),
						zap.Int("status", status),
					)...,
				)
			}
			if httpTelemetryReady() {
				metricAttrs := append(attrs, attribute.String("status_code", strconv.Itoa(status)))
				httpRequestCount.Add(ctx, 1, telemetry.Attrs(metricAttrs...))
				httpRequestTime.Record(ctx, time.Since(start).Seconds(), telemetry.Attrs(metricAttrs...))
			}
		})
	}
}

func routePattern(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if pattern := rctx.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return r.URL.Path
}

func httpTelemetryReady() bool {
	return httpInstrumentErr == nil
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
