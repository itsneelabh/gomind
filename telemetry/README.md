# GoMind Telemetry Module

The telemetry module provides basic OpenTelemetry integration for the GoMind framework. Currently implements minimal tracing functionality with OTLP export capability.

## Table of Contents
- [Current Features](#current-features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Components](#components)
- [Examples](#examples)
- [API Reference](#api-reference)
- [Roadmap](#roadmap)
- [Contributing](#contributing)

## Current Features

✅ **Implemented:**
- Basic OpenTelemetry tracer setup
- OTLP exporter configuration
- Simple span creation and management
- Attribute setting on spans
- Error recording in spans

⚠️ **Limitations:**
- No metrics collection (stub implementation only)
- No correlation ID management
- No support for multiple exporters
- No Prometheus integration
- No performance monitoring
- No resource monitoring
- Limited configuration options
- No context propagation helpers

## Installation

```bash
go get github.com/itsneelabh/gomind/telemetry
```

## Quick Start

### Basic Tracing Setup

```go
package main

import (
    "context"
    "log"
    "github.com/itsneelabh/gomind/telemetry"
)

func main() {
    // Initialize telemetry provider
    // Uses OTLP exporter to localhost:4317 by default
    provider := telemetry.NewOTELProvider("my-service")
    
    // IMPORTANT: Shutdown provider when done
    defer func() {
        if err := provider.Shutdown(context.Background()); err != nil {
            log.Printf("Error shutting down telemetry: %v", err)
        }
    }()
    
    // Start a span
    ctx, span := provider.StartSpan(context.Background(), "main.operation")
    defer span.End()
    
    // Add attributes
    span.SetAttribute("user.id", "12345")
    span.SetAttribute("operation.type", "process")
    
    // Do your work
    if err := doWork(ctx); err != nil {
        span.RecordError(err)
    }
}
```

### Using with GoMind Agents

```go
package main

import (
    "context"
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

func main() {
    // Create telemetry provider
    telemetryProvider := telemetry.NewOTELProvider("my-agent")
    defer telemetryProvider.Shutdown(context.Background())
    
    // Create agent with telemetry
    agent := core.NewBaseAgent("my-agent")
    agent.Telemetry = telemetryProvider
    
    // Now spans will be created automatically for agent operations
    ctx := context.Background()
    agent.Initialize(ctx) // This will be traced
}
```

## Components

### 1. OTELProvider

The main telemetry provider that wraps OpenTelemetry functionality.

```go
type OTELProvider struct {
    serviceName string
    tracer      trace.Tracer
    provider    *sdktrace.TracerProvider
}
```

**Key Features:**
- Initializes OTLP exporter to `localhost:4317`
- Creates tracer for the service
- Manages provider lifecycle
- Implements the core.Telemetry interface

### 2. OTELSpan

Wrapper around OpenTelemetry spans implementing the core.Span interface.

```go
type OTELSpan struct {
    span trace.Span
}
```

**Methods:**
- `End()`: Completes the span
- `SetAttribute(key, value)`: Add metadata to span
- `RecordError(err)`: Record an error in the span

## Examples

### Example 1: Tracing HTTP Requests

```go
func tracedHandler(provider *telemetry.OTELProvider) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Start span for the request
        ctx, span := provider.StartSpan(r.Context(), "http.request")
        defer span.End()
        
        // Add request metadata
        span.SetAttribute("http.method", r.Method)
        span.SetAttribute("http.path", r.URL.Path)
        span.SetAttribute("http.user_agent", r.UserAgent())
        
        // Process request
        if err := processRequest(ctx, r); err != nil {
            span.RecordError(err)
            span.SetAttribute("http.status_code", 500)
            http.Error(w, "Internal error", 500)
            return
        }
        
        span.SetAttribute("http.status_code", 200)
        w.WriteHeader(200)
        w.Write([]byte("Success"))
    }
}
```

### Example 2: Tracing Database Operations

```go
func queryWithTracing(ctx context.Context, provider *telemetry.OTELProvider, db *sql.DB, query string) (*sql.Rows, error) {
    // Create child span
    ctx, span := provider.StartSpan(ctx, "db.query")
    defer span.End()
    
    // Add query metadata
    span.SetAttribute("db.statement", query)
    span.SetAttribute("db.system", "postgresql")
    
    // Execute query
    rows, err := db.QueryContext(ctx, query)
    if err != nil {
        span.RecordError(err)
        return nil, err
    }
    
    span.SetAttribute("db.rows_affected", rows)
    return rows, nil
}
```

### Example 3: Nested Spans

```go
func processOrder(ctx context.Context, provider *telemetry.OTELProvider, orderID string) error {
    // Parent span
    ctx, span := provider.StartSpan(ctx, "process.order")
    defer span.End()
    span.SetAttribute("order.id", orderID)
    
    // Child span 1: Validate
    if err := validateOrder(ctx, provider, orderID); err != nil {
        span.RecordError(err)
        return err
    }
    
    // Child span 2: Charge payment
    if err := chargePayment(ctx, provider, orderID); err != nil {
        span.RecordError(err)
        return err
    }
    
    // Child span 3: Ship order
    if err := shipOrder(ctx, provider, orderID); err != nil {
        span.RecordError(err)
        return err
    }
    
    return nil
}

func validateOrder(ctx context.Context, provider *telemetry.OTELProvider, orderID string) error {
    ctx, span := provider.StartSpan(ctx, "validate.order")
    defer span.End()
    
    // Validation logic here
    span.SetAttribute("validation.passed", true)
    return nil
}
```

## API Reference

### OTELProvider

| Method | Description |
|--------|-------------|
| `NewOTELProvider(serviceName string)` | Create new telemetry provider |
| `StartSpan(ctx context.Context, name string)` | Start a new span |
| `RecordMetric(name string, value float64, labels map[string]string)` | **NOT IMPLEMENTED** - Placeholder only |
| `Shutdown(ctx context.Context)` | Cleanup provider resources |

### OTELSpan

| Method | Description |
|--------|-------------|
| `End()` | Complete the span |
| `SetAttribute(key string, value interface{})` | Add metadata to span |
| `RecordError(err error)` | Record error in span |

## Configuration

Currently, the telemetry module has limited configuration:

- **Exporter**: Fixed to OTLP at `localhost:4317`
- **Service Name**: Set during provider creation
- **Batching**: Uses default batch span processor

To use with a different OTLP endpoint, you'll need to set the environment variable:
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://your-collector:4317
```

## Roadmap

### Near-term (Planned)
- [ ] Implement metrics collection
- [ ] Add configuration options for exporter
- [ ] Support for Jaeger exporter
- [ ] Support for Zipkin exporter
- [ ] Context propagation helpers

### Medium-term (Under Consideration)
- [ ] Prometheus metrics integration
- [ ] Correlation ID management
- [ ] Sampling configuration
- [ ] Custom span processors
- [ ] Log correlation

### Long-term (Future)
- [ ] Multiple simultaneous exporters
- [ ] Performance monitoring dashboard
- [ ] Resource monitoring
- [ ] Distributed tracing helpers
- [ ] Auto-instrumentation for common libraries

## Testing

```go
func TestSpanCreation(t *testing.T) {
    provider := telemetry.NewOTELProvider("test-service")
    defer provider.Shutdown(context.Background())
    
    ctx, span := provider.StartSpan(context.Background(), "test.operation")
    
    // Add attributes
    span.SetAttribute("test.id", "123")
    span.SetAttribute("test.value", 42)
    
    // Simulate error
    err := errors.New("test error")
    span.RecordError(err)
    
    // End span
    span.End()
    
    // Note: In real tests, you'd want to use a test exporter
    // to verify the span data was recorded correctly
}
```

## Performance Considerations

- **Minimal Overhead**: Basic span creation adds ~1-2μs
- **Batching**: Spans are batched before export to reduce network calls
- **Async Export**: Export happens in background, doesn't block operations
- **Sampling**: Currently exports all spans (no sampling implemented)

## Prerequisites

To use this module, you need an OpenTelemetry collector running:

```bash
# Using Docker
docker run -p 4317:4317 otel/opentelemetry-collector:latest
```

Or configure your application to point to your existing observability infrastructure (Jaeger, Zipkin, Datadog, etc.) that supports OTLP.

## Contributing

We welcome contributions! Current priorities:
1. Implementing metrics collection
2. Adding more exporter options
3. Configuration improvements
4. Context propagation utilities
5. Better testing utilities

Please ensure:
- All code includes tests
- Documentation is updated
- Examples are functional

## License

See the main GoMind repository for license information.