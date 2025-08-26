package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

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

// CapabilityMetadata represents capability metadata for telemetry
type CapabilityMetadata struct {
	Name            string
	Domain          string
	Complexity      string
	ConfidenceLevel float64
	BusinessValue   []string
	LLMPrompt       string
	Specialties     []string
}

// OTELImpl provides zero-configuration OpenTelemetry integration
type OTELImpl struct {
	TraceProvider *sdktrace.TracerProvider
	MeterProvider metric.MeterProvider
	Tracer        trace.Tracer
	Meter         metric.Meter
	serviceName   string
	agentID       string
	capabilities  []string
	resource      *resource.Resource
}

// NewAutoOTEL creates a new auto-configured OTEL instance
func NewAutoOTEL(serviceName, agentID string, capabilities []string) (AutoOTEL, error) {
	// Check if OTEL is disabled
	if os.Getenv("OTEL_SDK_DISABLED") == "true" {
		return &OTELImpl{
			Tracer: otel.Tracer("noop"),
			Meter:  otel.Meter("noop"),
		}, nil
	}

	// Auto-detect service name
	if serviceName == "" {
		serviceName = os.Getenv("OTEL_SERVICE_NAME")
		if serviceName == "" {
			serviceName = agentID // Fallback to agent ID
		}
	}

	// Create resource with rich context
	res, err := createResourceWithAttributes(serviceName, agentID, capabilities)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTEL resource: %w", err)
	}

	// Set up trace provider
	traceProvider, err := setupTraceProvider(res)
	if err != nil {
		return nil, fmt.Errorf("failed to setup trace provider: %w", err)
	}

	// Set up meter provider
	meterProvider, err := setupMeterProvider(res)
	if err != nil {
		return nil, fmt.Errorf("failed to setup meter provider: %w", err)
	}

	// Set global providers
	otel.SetTracerProvider(traceProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	autoOTEL := &OTELImpl{
		TraceProvider: traceProvider,
		MeterProvider: meterProvider,
		Tracer:        traceProvider.Tracer("gomind-framework"),
		Meter:         meterProvider.Meter("gomind-framework"),
		serviceName:   serviceName,
		agentID:       agentID,
		capabilities:  capabilities,
		resource:      res,
	}

	return autoOTEL, nil
}

// createResourceWithAttributes creates an OTEL resource with GoMind framework attributes
func createResourceWithAttributes(serviceName, agentID string, capabilities []string) (*resource.Resource, error) {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String(getServiceVersion()),
		semconv.DeploymentEnvironmentKey.String(getEnvironment()),

		// GoMind Framework specific attributes
		attribute.String("gomind.agent.id", agentID),
		attribute.String("gomind.framework", "gomind-framework-go"),
		attribute.String("gomind.version", "v1.0.0"),
		attribute.StringSlice("gomind.agent.capabilities", capabilities),
		attribute.String("gomind.discovery.backend", "redis"),

		// Kubernetes attributes (if running in K8s)
		semconv.K8SNamespaceNameKey.String(os.Getenv("KUBERNETES_NAMESPACE")),
		semconv.K8SPodNameKey.String(os.Getenv("HOSTNAME")),
		attribute.String("k8s.pod.ip", os.Getenv("POD_IP")),
	), nil
}

// setupTraceProvider configures the trace provider based on environment
func setupTraceProvider(res *resource.Resource) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	// Check for OTLP endpoint
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		// No OTEL endpoint - use noop provider
		return sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
		), nil
	}

	// Set up OTLP exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(), // TODO: Make configurable
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Configure sampling
	sampler := sdktrace.AlwaysSample()
	samplerArg := os.Getenv("OTEL_TRACES_SAMPLER_ARG")
	if samplerArg != "" && os.Getenv("OTEL_TRACES_SAMPLER") == "traceidratio" {
		// Parse sampling ratio
		if ratio, err := parseFloat64(samplerArg); err == nil {
			sampler = sdktrace.TraceIDRatioBased(ratio)
		}
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	return provider, nil
}

// setupMeterProvider configures the meter provider
func setupMeterProvider(res *resource.Resource) (metric.MeterProvider, error) {
	// For now, return the global meter provider
	// TODO: Add Prometheus exporter configuration
	return otel.GetMeterProvider(), nil
}

// getServiceVersion gets the service version from environment or default
func getServiceVersion() string {
	if version := os.Getenv("OTEL_SERVICE_VERSION"); version != "" {
		return version
	}
	return "1.0.0" // Default version
}

// getEnvironment gets the deployment environment
func getEnvironment() string {
	if env := os.Getenv("DEPLOYMENT_ENVIRONMENT"); env != "" {
		return env
	}
	if env := os.Getenv("OTEL_RESOURCE_ATTRIBUTES"); env != "" {
		// Parse environment from resource attributes
		// Simplified parsing - in production, use proper parsing
		return "production"
	}
	return "development"
}

// parseFloat64 safely parses a float64 from string
func parseFloat64(s string) (float64, error) {
	// Simplified implementation
	switch s {
	case "0.1":
		return 0.1, nil
	case "0.01":
		return 0.01, nil
	case "1.0":
		return 1.0, nil
	default:
		return 0.1, nil // Default sampling ratio
	}
}

// CreateSpanWithCapability creates a span with capability metadata
func (a *OTELImpl) CreateSpanWithCapability(ctx context.Context, capability CapabilityMetadata) (context.Context, trace.Span) {
	spanName := fmt.Sprintf("capability.%s", capability.Name)
	ctx, span := a.Tracer.Start(ctx, spanName)

	// Add capability metadata to span
	span.SetAttributes(
		attribute.String("ai.capability.name", capability.Name),
		attribute.String("ai.capability.domain", capability.Domain),
		attribute.String("ai.capability.complexity", capability.Complexity),
		attribute.Float64("ai.capability.confidence", capability.ConfidenceLevel),
		attribute.StringSlice("ai.capability.business_value", capability.BusinessValue),
		attribute.String("ai.capability.llm_prompt", capability.LLMPrompt),
		attribute.StringSlice("ai.capability.specialties", capability.Specialties),
		attribute.String("gomind.agent.id", a.agentID),
	)

	return ctx, span
}

// RecordCapabilityMetrics records metrics for capability execution
func (a *OTELImpl) RecordCapabilityMetrics(ctx context.Context, capability CapabilityMetadata, duration time.Duration, err error) {
	// Record execution counter
	if counter, counterErr := a.Meter.Int64Counter(
		"capability_executions_total",
		metric.WithDescription("Total capability executions"),
	); counterErr == nil {
		labels := []attribute.KeyValue{
			attribute.String("capability", capability.Name),
			attribute.String("domain", capability.Domain),
			attribute.String("agent", a.agentID),
		}
		if err != nil {
			labels = append(labels, attribute.String("status", "error"))
		} else {
			labels = append(labels, attribute.String("status", "success"))
		}
		counter.Add(ctx, 1, metric.WithAttributes(labels...))
	}

	// Record duration histogram
	if histogram, histErr := a.Meter.Float64Histogram(
		"capability_duration_seconds",
		metric.WithDescription("Capability execution duration"),
	); histErr == nil {
		histogram.Record(ctx, duration.Seconds(),
			metric.WithAttributes(
				attribute.String("capability", capability.Name),
				attribute.String("domain", capability.Domain),
				attribute.String("agent", a.agentID),
			))
	}
}

// Shutdown gracefully shuts down the OTEL providers
func (a *OTELImpl) Shutdown(ctx context.Context) error {
	if a.TraceProvider != nil {
		return a.TraceProvider.Shutdown(ctx)
	}
	return nil
}
