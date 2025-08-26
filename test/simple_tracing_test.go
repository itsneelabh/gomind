package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/itsneelabh/gomind/pkg/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

func TestSimpleTracing(t *testing.T) {
	// Initialize stdout exporter for testing
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		t.Fatal(err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("gomind-test"),
			semconv.ServiceVersionKey.String("1.0.0"),
		)),
	)
	defer tp.Shutdown(context.Background())
	otel.SetTracerProvider(tp)

	// Create root span
	tracer := otel.Tracer("gomind.test")
	ctx, rootSpan := tracer.Start(context.Background(), "test-root-operation",
		trace.WithAttributes(
			attribute.String("test.type", "integration"),
			attribute.String("test.name", "simple-tracing"),
		),
	)
	defer rootSpan.End()

	// Add correlation IDs
	ctx = context.WithValue(ctx, telemetry.CorrelationIDKey, "corr-123")
	ctx = context.WithValue(ctx, telemetry.RequestIDKey, "req-456")

	// Create child span
	_, childSpan := tracer.Start(ctx, "test-child-operation",
		trace.WithAttributes(
			attribute.String("correlation.id", telemetry.GetCorrelationID(ctx)),
			attribute.String("request.id", telemetry.GetRequestID(ctx)),
		),
	)
	childSpan.AddEvent("Processing started")
	childSpan.AddEvent("Processing completed")
	childSpan.End()

	// Test log enrichment
	fields := telemetry.EnrichLogFields(ctx, map[string]interface{}{
		"operation": "test",
	})

	fmt.Printf("\nEnriched log fields:\n")
	for key, value := range fields {
		fmt.Printf("  %s: %v\n", key, value)
	}

	rootSpan.AddEvent("Test completed")
}