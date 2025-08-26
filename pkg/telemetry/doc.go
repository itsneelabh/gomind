// Package telemetry provides observability and monitoring capabilities for the GoMind Agent Framework
// using OpenTelemetry standards.
//
// This package enables comprehensive monitoring of agent behavior, performance metrics,
// distributed tracing, and operational insights through integration with OpenTelemetry
// and compatible observability platforms.
//
// # Core Components
//
// The telemetry system provides three pillars of observability:
//
// Metrics:
//   - Request/response counters and latencies
//   - Agent invocation statistics
//   - Resource utilization metrics
//   - Business-specific custom metrics
//
// Traces:
//   - Distributed tracing across agent interactions
//   - Automatic span creation for capabilities
//   - Context propagation for multi-agent flows
//   - Performance bottleneck identification
//
// Logs:
//   - Structured logging with trace correlation
//   - Automatic context enrichment
//   - Log-to-trace correlation IDs
//
// # AutoOTEL Interface
//
// The AutoOTEL interface provides automatic instrumentation:
//
//	type AutoOTEL interface {
//	    StartSpan(ctx context.Context, name string) (context.Context, Span)
//	    RecordMetric(name string, value float64, labels map[string]string)
//	    GetTracer() trace.Tracer
//	    GetMeter() metric.Meter
//	    Shutdown(ctx context.Context) error
//	}
//
// # Automatic Instrumentation
//
// The framework automatically instruments:
//   - HTTP requests and responses
//   - Agent capability invocations
//   - Inter-agent communications
//   - AI/LLM interactions
//   - Database operations
//   - Cache hits/misses
//
// # Usage Example
//
// Manual span creation for custom operations:
//
//	ctx, span := telemetry.StartSpan(ctx, "process_order")
//	defer span.End()
//	
//	span.SetAttributes(
//	    attribute.String("order.id", orderID),
//	    attribute.Int("order.items", len(items)),
//	)
//	
//	// Process order...
//	if err != nil {
//	    span.RecordError(err)
//	    span.SetStatus(codes.Error, err.Error())
//	}
//
// Recording custom metrics:
//
//	telemetry.RecordMetric("orders.processed", 1, map[string]string{
//	    "status": "success",
//	    "payment_method": "credit_card",
//	})
//
// # Capability Metadata Integration
//
// Telemetry automatically enriches spans with capability metadata:
//   - Capability name and description
//   - Expected latency and complexity
//   - Business context and impact
//   - Resource requirements
//
// # Configuration
//
// Telemetry can be configured through environment variables:
//   - OTEL_EXPORTER_OTLP_ENDPOINT: OTLP endpoint (e.g., localhost:4317)
//   - OTEL_SERVICE_NAME: Service name for traces
//   - OTEL_RESOURCE_ATTRIBUTES: Additional resource attributes
//   - TELEMETRY_ENABLED: Enable/disable telemetry (true/false)
//
// # Exporters
//
// Supported export destinations:
//   - OTLP (OpenTelemetry Protocol) - Recommended
//   - Jaeger
//   - Prometheus (metrics)
//   - Console (development)
//
// # Context Propagation
//
// The framework handles context propagation automatically:
//   - W3C Trace Context headers for HTTP
//   - Baggage for cross-service metadata
//   - Parent-child span relationships
//
// # Performance Considerations
//
//   - Sampling strategies to reduce overhead
//   - Async export to prevent blocking
//   - Batch processing for efficiency
//   - Configurable queue sizes and timeouts
//
// # Integration with Agents
//
// Telemetry is automatically injected into agents:
//
//	func (a *MyAgent) ProcessData(ctx context.Context, data string) error {
//	    ctx, span := a.Telemetry().StartSpan(ctx, "process_data")
//	    defer span.End()
//	    
//	    // Your processing logic with automatic instrumentation
//	    return nil
//	}
package telemetry