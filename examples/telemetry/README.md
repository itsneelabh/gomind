# Telemetry Example

A comprehensive example demonstrating all telemetry features in the GoMind framework. This example shows how to emit metrics, track operations, integrate with circuit breakers, and monitor application health.

## 🎯 What This Example Demonstrates

- **Simple Metrics**: Counter, Histogram, Gauge emission
- **Operation Timing**: Automatic duration tracking
- **Error/Success Tracking**: Recording operation outcomes
- **Circuit Breaker Integration**: Metrics with resilience patterns
- **Retry Integration**: Telemetry-aware retry mechanisms
- **Advanced Options**: Labels, units, sampling, batch emission
- **Health Monitoring**: Telemetry system health and internal metrics

## 🧠 Features Covered

### 1. Simple Metrics Emission
```go
telemetry.Counter("app.requests", "endpoint", "/api/users", "method", "GET")
telemetry.Histogram("app.response_time", 125.5, "endpoint", "/api/users")
telemetry.Gauge("app.active_connections", 42, "server", "api-1")
```

### 2. Operation Timing
```go
done := telemetry.TimeOperation("database.query", "table", "users")
// ... perform operation ...
done() // Automatically records duration
```

### 3. Error & Success Tracking
```go
if err != nil {
    telemetry.RecordError("app.errors", "timeout", "operation", "fetch_data")
} else {
    telemetry.RecordSuccess("app.operations", "operation", "fetch_data")
}
```

### 4. Circuit Breaker with Telemetry
```go
cb, _ := resilience.NewCircuitBreakerWithTelemetry("api-circuit")
resilience.ExecuteWithTelemetry(cb, ctx, func() error {
    // Your operation here
    return callAPI()
})
```

### 5. Retry with Telemetry
```go
resilience.RetryWithTelemetry(ctx, "data_fetch", config, func() error {
    // Your operation here
    return fetchData()
})
```

### 6. Advanced Emission Options
```go
telemetry.EmitWithOptions(ctx, "app.custom_metric", 99.9,
    telemetry.WithLabel("env", "production"),
    telemetry.WithLabel("region", "us-west-2"),
    telemetry.WithUnit(telemetry.UnitMilliseconds),
    telemetry.WithSampleRate(0.1), // Sample 10%
)
```

### 7. Batch Metrics for Efficiency
```go
metrics := []struct {
    Name   string
    Value  float64
    Labels []string
}{
    {"batch.metric1", 10, []string{"type", "a"}},
    {"batch.metric2", 20, []string{"type", "b"}},
}
telemetry.BatchEmit(metrics)
```

### 8. Health & Monitoring
```go
health := telemetry.GetHealth()
fmt.Printf("Metrics Emitted: %d\n", health.MetricsEmitted)
fmt.Printf("Circuit State: %s\n", health.CircuitState)
fmt.Printf("Uptime: %s\n", health.Uptime)

internal := telemetry.GetInternalMetrics()
fmt.Printf("Emitted: %d, Dropped: %d, Errors: %d\n",
    internal.Emitted, internal.Dropped, internal.Errors)
```

## 🚀 Quick Start

### Prerequisites
- Go 1.25+

### 1. Run the Example

```bash
cd examples/telemetry
go mod tidy
go run main.go
```

### 2. Expected Output

The example will demonstrate all 7 features sequentially:
1. Simple metrics emission
2. Operation timing
3. Error tracking
4. Circuit breaker integration
5. Retry with telemetry
6. Advanced emission options
7. Batch metrics

Finally, it will display:
- Telemetry health status
- Internal metrics (emitted, dropped, errors)

## 📊 Telemetry Profiles

The example uses **Development** profile for console output:
```go
telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileDevelopment))
```

For production, use:
```go
telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileProduction))
```

**Development Profile:**
- Console output (human-readable)
- No OTLP export
- All metrics emitted
- Verbose logging

**Production Profile:**
- OTLP export to collector
- Sampling enabled
- Efficient batching
- Minimal overhead

## 🏗️ Production Deployment

### Docker Build

```bash
# From project root
docker build -f examples/telemetry/Dockerfile -t telemetry-example .
```

### Kubernetes Deployment

```bash
# Deploy to Kubernetes
kubectl apply -f examples/telemetry/k8-deployment.yaml
```

The deployment includes:
- OTLP collector integration
- Prometheus metrics endpoint
- Health checks
- Resource limits
- Non-root security

## 🔍 Integration Patterns

### With OpenTelemetry Collector

The example automatically exports metrics to OTLP collector when configured:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="http://otel-collector:4318"
```

### With Prometheus

Metrics are exposed at `/metrics` endpoint:
```bash
curl http://localhost:8080/metrics
```

## 📈 Monitoring Dashboard

When deployed to Kubernetes with the included configuration:

1. **Prometheus** scrapes metrics from `/metrics`
2. **Grafana** visualizes the data
3. **OTLP Collector** aggregates and exports

Example metrics you'll see:
- `app_requests_total{endpoint="/api/users",method="GET"}`
- `app_response_time_bucket{endpoint="/api/users"}`
- `app_active_connections{server="api-1"}`
- `circuit_breaker_state{name="api-circuit"}`
- `retry_attempts_total{operation="data_fetch"}`

## 🎓 Learning Path

1. **Start Here**: Run the example to see all features in action
2. **Experiment**: Modify metric names, labels, values
3. **Integrate**: Add telemetry to your own applications
4. **Production**: Deploy with OTLP collector and Prometheus

## 📚 Related Examples

- `resilience` - Circuit breaker patterns
- `monitoring` - Full observability stack
- `agent-example` - Using telemetry in agents

## 🔗 Documentation

- [Telemetry Module API](../../telemetry/README.md)
- [Resilience Integration](../../resilience/README.md)
- [OpenTelemetry Setup](../../docs/telemetry.md)
