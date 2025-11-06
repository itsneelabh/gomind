# GoMind Observability Stack

This directory contains a complete observability stack for GoMind framework applications using modern cloud-native patterns.

## 🏗️ Architecture Overview

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   GoMind Apps   │───▶│  OTEL Collector  │───▶│   Prometheus    │
│   (Agents/Tools)│    │   (Port 4318)    │    │   (Port 9090)   │
└─────────────────┘    │                  │    └─────────────────┘
                       │                  │
                       │                  │    ┌─────────────────┐
                       │                  │───▶│     Jaeger      │
                       │                  │    │   (Port 16686)  │
                       └──────────────────┘    └─────────────────┘
```

## 🚀 What Gets Deployed

### Core Infrastructure
- **Redis** - Service discovery backend
- **OTEL Collector** - Telemetry aggregation and routing
- **Prometheus** - Metrics storage and alerting
- **Jaeger** - Distributed tracing
- **Grafana** - Visualization and dashboards

### Modern Telemetry Pipeline
1. **GoMind apps** export metrics/traces via **OTLP** (OpenTelemetry Protocol)
2. **OTEL Collector** receives OTLP data and:
   - Exports metrics in Prometheus format (`/metrics` endpoint)
   - Forwards traces to Jaeger
   - Provides observability for the telemetry pipeline itself
3. **Prometheus** scrapes metrics from collector
4. **Grafana** visualizes data from both Prometheus and Jaeger

## 📊 Why OTLP-First Design?

### Traditional Approach (Problematic)
```
App → /metrics endpoint → Prometheus (metrics only)
App → Jaeger SDK → Jaeger (traces only)
```
**Issues**: Separate instrumentation, no correlation, vendor lock-in

### Modern OTLP Approach (Implemented)
```
App → OTLP → OTEL Collector → Multiple Backends
```
**Benefits**:
- ✅ Single instrumentation SDK
- ✅ Automatic trace-metric correlation
- ✅ Vendor-agnostic (swap backends without code changes)
- ✅ Better resource efficiency (batched exports)
- ✅ Cloud-native standard (CNCF)

## 🛠️ Deployment

### Quick Start
```bash
# Deploy complete observability stack
kubectl apply -k examples/k8-deployment/

# Verify deployment
kubectl get pods -n gomind-examples

# Access services
kubectl port-forward svc/grafana 3000:80 -n gomind-examples      # Grafana UI
kubectl port-forward svc/prometheus 9090:9090 -n gomind-examples # Prometheus UI
kubectl port-forward svc/jaeger-query 16686:80 -n gomind-examples # Jaeger UI
```

### Environment Variables for GoMind Apps

The stack automatically configures these environment variables for your applications:

```yaml
env:
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: "http://otel-collector:4318"    # OTLP export endpoint
- name: REDIS_URL
  value: "redis://redis:6379"           # Service discovery
```

#### Automatic Telemetry Activation

**Important**: Setting `OTEL_EXPORTER_OTLP_ENDPOINT` automatically enables telemetry in the framework.

**How it works** (from [core/config.go:579-581](../../core/config.go#L579-L581)):
```go
if v := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); v != "" {
    c.Telemetry.Endpoint = v
    c.Telemetry.Enabled = true  // Auto-enabled
}
```

**Default telemetry settings** (from [core/config.go:297-304](../../core/config.go#L297-L304)):
```go
TelemetryConfig{
    Enabled:        false,  // Changed to true when endpoint is set
    Provider:       "otel",
    MetricsEnabled: true,   // Metrics collection enabled by default
    TracingEnabled: true,   // Distributed tracing enabled by default
    SamplingRate:   1.0,    // 100% trace sampling
    Insecure:       true,   // No TLS (for dev/local environments)
}
```

**What gets collected automatically:**
- ✅ **Metrics**: Request counts, latencies, error rates, resource usage
- ✅ **Traces**: Distributed request traces with full span context
- ✅ **Context Propagation**: W3C Trace Context headers for correlation

**You do NOT need to set:**
- ❌ `GOMIND_TELEMETRY_ENABLED=true` (auto-enabled by endpoint)
- ❌ `GOMIND_TELEMETRY_METRICS=true` (enabled by default)
- ❌ `GOMIND_TELEMETRY_TRACING=true` (enabled by default)

## 📈 What You Get

### Prometheus Metrics (localhost:9090)
- **GoMind Framework Metrics**: Request counts, latencies, errors
- **AI/LLM Metrics**: Token usage, costs, rate limits
- **Circuit Breaker Metrics**: Success/failure rates, state changes
- **Discovery Metrics**: Service registrations, health checks
- **Infrastructure Metrics**: Redis, OTEL Collector performance

### Jaeger Tracing (localhost:16686)
- **Distributed request traces** across agents and tools
- **AI request tracing** with token usage and latencies
- **Service discovery traces** showing component interactions
- **Automatic correlation** with metrics via trace IDs

### Grafana Dashboards (localhost:3000)
- **GoMind Overview**: System health and performance
- **AI Usage Dashboard**: Token consumption, costs, provider performance
- **Service Discovery**: Component topology and health
- **Infrastructure**: Redis, Kubernetes, OTEL Collector metrics

## 🔍 Monitoring Your Applications

### Application Integration
Your GoMind applications automatically export telemetry when the framework is configured:

```go
// Framework automatically handles this
import "github.com/itsneelabh/gomind/telemetry"

// Initialize telemetry (usually done by framework)
telemetry.Initialize(telemetry.ProfileProduction)

// Metrics are automatically emitted for:
// - HTTP requests (latency, errors, throughput)
// - AI API calls (tokens, costs, latency)
// - Service discovery (registrations, lookups)
// - Circuit breaker events (trips, recoveries)
```

### Custom Metrics
```go
// Add custom business metrics
telemetry.Counter("orders.processed", "status", "success")
telemetry.Histogram("payment.amount", 99.99, "method", "stripe")
telemetry.Gauge("queue.size", 42, "queue", "orders")
```

## 🚨 Alerting Rules

Included Prometheus alerts:
- **GoMindComponentDown**: Component unavailable > 1 minute
- **GoMindHighErrorRate**: Error rate > 10% for 2 minutes
- **GoMindHighLatency**: 95th percentile latency > 1 second for 5 minutes

## 🔧 Configuration

### OTEL Collector Config
The collector is configured to:
- Receive OTLP on ports 4317 (gRPC) and 4318 (HTTP)
- Export Prometheus metrics on port 8888
- Forward traces to Jaeger
- Include resource attribution and batching for efficiency

### Prometheus Discovery
Uses Kubernetes service discovery to automatically find:
- GoMind agents with label `gomind.framework/type: agent`
- GoMind tools with label `gomind.framework/type: tool`
- OTEL Collector with proper annotations

### Resource Requirements
- **OTEL Collector**: 128Mi RAM, 100m CPU
- **Prometheus**: 512Mi RAM, 200m CPU
- **Jaeger**: 512Mi RAM, 200m CPU
- **Grafana**: 256Mi RAM, 100m CPU
- **Redis**: 256Mi RAM, 100m CPU

Total: ~1.6GB RAM, ~800m CPU for complete observability stack

## 📚 Additional Resources

- [OpenTelemetry Documentation](https://opentelemetry.io/)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/)
- [Jaeger Documentation](https://www.jaegertracing.io/)
- [GoMind Telemetry Module](../../telemetry/README.md)