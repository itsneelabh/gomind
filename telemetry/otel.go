package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/itsneelabh/gomind/core"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// OTelProvider implements core.Telemetry with OpenTelemetry.
// This is the main integration point between GoMind and OpenTelemetry.
// It manages both tracing and metrics, exporting them via OTLP/HTTP.
//
// Design decisions:
//   - Uses HTTP instead of gRPC for smaller binary size
//   - Batches exports to reduce network overhead
//   - Provides both traces and metrics from a single provider
type OTelProvider struct {
	tracer         trace.Tracer             // For distributed tracing
	meter          metric.Meter             // For metrics
	traceProvider  *sdktrace.TracerProvider // Manages trace export
	metricProvider *sdkmetric.MeterProvider // Manages metric export
	metrics        *MetricInstruments       // Cached metric instruments
}

// NewOTelProvider creates a new OpenTelemetry provider using HTTP exporters.
// This sets up the complete telemetry pipeline:
//  1. Creates HTTP exporters for traces and metrics
//  2. Configures batching for efficient export
//  3. Sets up global providers for SDK access
//
// The endpoint should be an OTLP/HTTP endpoint (typically port 4318).
// For backward compatibility, gRPC ports (4317) are automatically converted.
func NewOTelProvider(serviceName string, endpoint string) (*OTelProvider, error) {
	// Normalize endpoint - support both old gRPC and new HTTP formats
	if endpoint == "" {
		endpoint = "localhost:4318" // Default HTTP endpoint
	}
	// Auto-convert gRPC port to HTTP for backward compatibility
	if endpoint == "localhost:4317" {
		endpoint = "localhost:4318"
	}

	// Create resource with consistent schema
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String("1.0.0"),
	)

	ctx := context.Background()

	// Create HTTP trace exporter (instead of gRPC)
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(), // For development; use TLS in production
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter for endpoint %s: %w", endpoint, err)
	}

	// Create HTTP metric exporter (this was missing!)
	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(endpoint),
		otlpmetrichttp.WithInsecure(), // For development; use TLS in production
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter for endpoint %s: %w", endpoint, err)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	// Create metric provider with periodic reader (exports metrics every 30s)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				metricExporter,
				sdkmetric.WithInterval(30*time.Second),
			),
		),
		sdkmetric.WithResource(res),
	)

	// Set global providers
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	provider := &OTelProvider{
		tracer:         tp.Tracer("gomind-telemetry"),
		meter:          mp.Meter("gomind-telemetry"),
		traceProvider:  tp,
		metricProvider: mp,
		metrics:        NewMetricInstruments("gomind-telemetry"),
	}

	return provider, nil
}

// StartSpan starts a new telemetry span
func (o *OTelProvider) StartSpan(ctx context.Context, name string) (context.Context, core.Span) {
	ctx, span := o.tracer.Start(ctx, name)
	return ctx, &otelSpan{span: span}
}

// RecordMetric records a metric - implements core.Telemetry interface.
// This function intelligently routes metrics to the appropriate instrument type
// based on the metric name pattern. This provides a simple API while maintaining
// semantic correctness for different metric types.
//
// Heuristics used:
//   - Names with "duration", "latency", "time" → Histogram
//   - Names with "count", "total", "errors" → Counter
//   - Names with "gauge", "current", "size" → Gauge/Histogram
func (o *OTelProvider) RecordMetric(name string, value float64, labels map[string]string) {
	ctx := context.Background()

	// Convert label map to OpenTelemetry attributes
	// This allocates but is necessary for the OTel API
	var attrs []attribute.KeyValue
	for k, v := range labels {
		attrs = append(attrs, attribute.String(k, v))
	}

	// Determine metric type based on name patterns
	// This is a simplified approach - in production you'd want explicit metric type registration
	switch {
	case contains(name, "duration", "latency", "time"):
		// Record as histogram for timing metrics
		_ = o.metrics.RecordHistogram(ctx, name, value, metric.WithAttributes(attrs...))
	case contains(name, "count", "total", "errors", "success"):
		// Record as counter for cumulative metrics
		_ = o.metrics.RecordCounter(ctx, name, int64(value), metric.WithAttributes(attrs...))
	case contains(name, "gauge", "current", "size", "queue"):
		// For gauges, we'd need to register a callback - for now record as histogram
		_ = o.metrics.RecordHistogram(ctx, name, value, metric.WithAttributes(attrs...))
	default:
		// Default to histogram for unknown metric types
		_ = o.metrics.RecordHistogram(ctx, name, value, metric.WithAttributes(attrs...))
	}
}

// contains checks if the metric name contains any of the given substrings.
// Used for heuristic metric type detection based on naming patterns.
// Checks both prefix and suffix to handle common naming conventions:
//   - "request_count" (suffix)
//   - "duration_ms" (suffix)
//   - "total_requests" (prefix)
func contains(name string, substrings ...string) bool {
	for _, substr := range substrings {
		if len(name) >= len(substr) &&
			(name[len(name)-len(substr):] == substr || // Check suffix
				name[:len(substr)] == substr) { // Check prefix
			return true
		}
	}
	return false
}

// Shutdown gracefully shuts down the telemetry provider
func (o *OTelProvider) Shutdown(ctx context.Context) error {
	var errs []error

	// Shutdown metrics instruments
	if err := o.metrics.Shutdown(); err != nil {
		errs = append(errs, fmt.Errorf("failed to shutdown metrics: %w", err))
	}

	// Shutdown metric provider (flushes pending metrics)
	if o.metricProvider != nil {
		if err := o.metricProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown metric provider: %w", err))
		}
	}

	// Shutdown trace provider
	if o.traceProvider != nil {
		if err := o.traceProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown trace provider: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// otelSpan wraps an OpenTelemetry span to implement core.Span
type otelSpan struct {
	span trace.Span
}

func (s *otelSpan) End() {
	s.span.End()
}

func (s *otelSpan) SetAttribute(key string, value interface{}) {
	switch v := value.(type) {
	case string:
		s.span.SetAttributes(attribute.String(key, v))
	case int:
		s.span.SetAttributes(attribute.Int(key, v))
	case int64:
		s.span.SetAttributes(attribute.Int64(key, v))
	case float64:
		s.span.SetAttributes(attribute.Float64(key, v))
	case bool:
		s.span.SetAttributes(attribute.Bool(key, v))
	default:
		s.span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", v)))
	}
}

func (s *otelSpan) RecordError(err error) {
	s.span.RecordError(err)
}

// EnableTelemetry adds telemetry to a core BaseAgent
// This is how you upgrade a basic tool with telemetry
func EnableTelemetry(agent *core.BaseAgent, endpoint string) error {
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		if endpoint == "" {
			endpoint = "localhost:4318" // Default HTTP port
		}
	}

	provider, err := NewOTelProvider(agent.GetName(), endpoint)
	if err != nil {
		return fmt.Errorf("failed to create telemetry provider: %w", err)
	}

	agent.Telemetry = provider

	agent.Logger.Info("Telemetry enabled", map[string]interface{}{
		"endpoint": endpoint,
		"agent":    agent.GetName(),
	})

	return nil
}
