package telemetry

import (
	"context"
	"fmt"
	"os"

	"github.com/itsneelabh/gomind/core"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// OTelProvider implements core.Telemetry with OpenTelemetry
type OTelProvider struct {
	tracer        trace.Tracer
	meter         metric.Meter
	traceProvider *sdktrace.TracerProvider
}

// NewOTelProvider creates a new OpenTelemetry provider
func NewOTelProvider(serviceName string, endpoint string) (*OTelProvider, error) {
	// Create resource
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}
	
	// Create OTLP exporter
	ctx := context.Background()
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}
	
	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	
	// Set global providers
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	
	return &OTelProvider{
		tracer:        tp.Tracer("gomind-telemetry"),
		meter:         otel.Meter("gomind-telemetry"),
		traceProvider: tp,
	}, nil
}

// StartSpan starts a new telemetry span
func (o *OTelProvider) StartSpan(ctx context.Context, name string) (context.Context, core.Span) {
	ctx, span := o.tracer.Start(ctx, name)
	return ctx, &otelSpan{span: span}
}

// RecordMetric records a metric
func (o *OTelProvider) RecordMetric(name string, value float64, labels map[string]string) {
	// Convert labels to attributes
	var attrs []attribute.KeyValue
	for k, v := range labels {
		attrs = append(attrs, attribute.String(k, v))
	}
	
	// Record metric (simplified - in real implementation would cache counters/gauges)
	// This is a placeholder - actual implementation would use proper metric instruments
	_ = attrs // Will be used when actual metric recording is implemented
}

// Shutdown gracefully shuts down the telemetry provider
func (o *OTelProvider) Shutdown(ctx context.Context) error {
	return o.traceProvider.Shutdown(ctx)
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
			endpoint = "localhost:4317" // Default
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