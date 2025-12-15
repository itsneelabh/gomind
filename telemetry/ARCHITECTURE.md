# GoMind Telemetry Module Architecture

**Version**: 1.0
**Module**: `github.com/itsneelabh/gomind/telemetry`
**Purpose**: Production-grade observability with OpenTelemetry integration
**Audience**: Framework developers, application developers, operations teams

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Design Philosophy](#design-philosophy)
3. [Global Singleton Pattern](#global-singleton-pattern)
4. [OpenTelemetry Integration](#opentelemetry-integration)
5. [Integration Patterns](#integration-patterns)
6. [OTLP Pipeline Architecture](#otlp-pipeline-architecture)
7. [Production Deployment](#production-deployment)
8. [Common Pitfalls](#common-pitfalls)
9. [Troubleshooting Guide](#troubleshooting-guide)

---

## Architecture Overview

### System Context

```
┌─────────────────────────────────────────────────────────────┐
│ Application Layer (main.go)                                 │
│                                                             │
│  telemetry.Initialize(config)  ← Explicit initialization  │
│  defer telemetry.Shutdown(ctx)                             │
└─────────────────────────────────────────────────────────────┘
                         │
                         │ Sets up global registry
                         ↓
┌─────────────────────────────────────────────────────────────┐
│ Telemetry Module (github.com/itsneelabh/gomind/telemetry)  │
│                                                             │
│  ┌─────────────┐    ┌──────────────┐    ┌──────────────┐ │
│  │  Registry   │───>│ OTelProvider │───>│ Exporters    │ │
│  │  (Singleton)│    │              │    │ (OTLP/HTTP)  │ │
│  └─────────────┘    └──────────────┘    └──────────────┘ │
│         │                                                   │
│         │ Global access via atomic.Value                   │
└─────────────────────────────────────────────────────────────┘
                         │
                         │ Used by application code
                         ↓
┌─────────────────────────────────────────────────────────────┐
│ Application Code                                            │
│                                                             │
│  telemetry.Counter("requests.total")                       │
│  telemetry.Histogram("latency.ms", 125.5)                  │
│  telemetry.Gauge("connections.active", 42)                 │
└─────────────────────────────────────────────────────────────┘
                         │
                         │ OTLP/HTTP Protocol
                         ↓
┌─────────────────────────────────────────────────────────────┐
│ OTEL Collector (Infrastructure)                            │
│                                                             │
│  Port 4318 (OTLP/HTTP)  →  Prometheus, Jaeger, etc.       │
└─────────────────────────────────────────────────────────────┘
```

### Key Components

| Component | Responsibility | Thread Safety |
|-----------|----------------|---------------|
| `Registry` | Global telemetry coordinator | Thread-safe via atomic operations |
| `OTelProvider` | OpenTelemetry SDK wrapper | Thread-safe by design |
| `MetricInstruments` | Pre-registered OTEL instruments | Thread-safe |
| `CardinalityLimiter` | Prevents metric explosion | Thread-safe via sync.Map |
| `TelemetryCircuitBreaker` | Protects backend from overload | Thread-safe |

---

## Design Philosophy

### 1. Why Explicit Initialization?

**The Design Decision**: Telemetry requires explicit `Initialize()` call at application level, not automatic framework-level initialization.

**Rationale**:

```go
// ❌ Framework CANNOT do this
// core/framework.go
import "github.com/itsneelabh/gomind/telemetry"  // FORBIDDEN

func (f *Framework) Run(ctx context.Context) error {
    telemetry.Initialize(...)  // Violates architectural principles
}
```

**Architectural Constraint**: The `core` module **never imports** optional modules. This ensures:
- **Unidirectional dependencies** - Core is the foundation layer
- **True optionality** - Telemetry is genuinely optional at compile time
- **No circular dependencies** - Impossible by architectural design
- **Interface-based decoupling** - Core defines `Telemetry` interface only

**Modules Allowed to Import Telemetry**:

| Module | Can Import Telemetry | Reason |
|--------|---------------------|--------|
| `core` | ❌ No | Foundation layer, cannot import optional modules |
| `ai` | ✅ Yes | AI operations need metrics/tracing for production visibility |
| `resilience` | ✅ Yes | Circuit breaker state, retry metrics |
| `orchestration` | ✅ Yes | Workflow execution metrics, plan generation tracking |
| `ui` | ❌ No (planned) | Currently uses core interfaces only; telemetry support planned |

```go
// ✅ Applications MUST do this
// examples/tool-example/main.go
import "github.com/itsneelabh/gomind/telemetry"  // Application choice

func main() {
    // Explicit initialization - clear and predictable
    telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileProduction))
    defer telemetry.Shutdown(context.Background())

    // Now all framework components can use telemetry
    tool := NewWeatherTool()
    framework, _ := core.NewFramework(tool, ...)
    framework.Run(ctx)
}
```

### 2. Global Singleton vs Dependency Injection

**Question**: Why not pass telemetry provider to each component?

**Comparison**:

| Pattern | Pros | Cons | GoMind Choice |
|---------|------|------|---------------|
| **Dependency Injection** | Explicit dependencies | Verbose, boilerplate-heavy | ❌ |
| **Global Singleton** | Simple API, zero boilerplate | Global state | ✅ Chosen |

**Why Global Singleton?**

```go
// ❌ Dependency Injection approach (verbose)
func (t *Tool) handleRequest(ctx context.Context, provider telemetry.Provider) {
    provider.Counter("requests")  // Must pass provider everywhere
}

// ✅ Global Singleton approach (simple)
func (t *Tool) handleRequest(ctx context.Context) {
    telemetry.Counter("requests")  // Just works
}
```

**Benefits**:
1. **Zero Boilerplate**: No need to pass provider through call chains
2. **Simple API**: `telemetry.Counter()` works anywhere
3. **Framework Alignment**: Matches `log.Printf()` pattern familiarity
4. **Performance**: Atomic reads are extremely fast
5. **Thread-Safe**: Built-in concurrency safety

**Tradeoffs Accepted**:
- Global state (controlled through initialization)
- Testing requires `Initialize()` in test setup
- Cannot have multiple telemetry configurations per process

### 3. Progressive Disclosure API Design

**Principle**: Make simple things simple, complex things possible.

**API Layers**:

```go
// Level 1: Simple API (90% of use cases)
telemetry.Counter("requests.total")
telemetry.Histogram("latency.ms", 125.5)
telemetry.Gauge("connections.active", 42)

// Level 2: Type-Specific Helpers (9% of use cases)
telemetry.RecordError("errors.total", "timeout")
telemetry.RecordLatency("api.latency_ms", 45.2)
telemetry.Duration("operation.duration_ms", startTime)

// Level 3: Full Control (1% of use cases)
telemetry.EmitWithOptions(ctx, "custom.metric", 99.9,
    telemetry.WithLabel("env", "prod"),
    telemetry.WithSampleRate(0.1),
    telemetry.WithUnit(telemetry.UnitMilliseconds))
```

**Design Goal**: Developers should reach for Level 1 API by default, only dropping to lower levels when needed.

### 4. Module Boundaries for Metrics

**Principle**: The telemetry module defines **contracts**, not implementations for other modules.

**What belongs in `unified_metrics.go`**:
- Cross-module helper functions that multiple modules need identically (e.g., `RecordAIRequest()`, `RecordToolCall()`)
- Metric name constants that create a shared vocabulary across modules
- Module label constants (`ModuleAgent`, `ModuleOrchestration`, `ModuleCore`)

**What does NOT belong in telemetry module files**:
- Module-specific metrics (e.g., orchestration's `plan_generation.retries`)
- Metrics that only one module will ever emit
- Implementation details specific to a single module's internal operations

**Correct Pattern**:

```go
// ✅ In orchestration/orchestrator.go - Module-specific metrics
// Use primitive APIs directly for orchestration-local metrics
telemetry.Counter("plan_generation.retries",
    "module", telemetry.ModuleOrchestration)

telemetry.Histogram("plan_generation.duration_ms", float64(duration.Milliseconds()),
    "module", telemetry.ModuleOrchestration,
    "status", "success")

// ✅ Use cross-module helpers for shared patterns
telemetry.RecordAIRequest(telemetry.ModuleOrchestration, "plan_generation",
    float64(llmDuration.Milliseconds()), "success")
```

**Incorrect Pattern**:

```go
// ❌ Do NOT add module-specific metrics to unified_metrics.go
// unified_metrics.go
const (
    UnifiedPlanGenerationRetry = "plan_generation.retries"  // WRONG: orchestration-specific
)

func RecordPlanGenerationRetry(module string) {  // WRONG: only orchestration uses this
    Counter(UnifiedPlanGenerationRetry, "module", module)
}
```

**Rationale**:
1. **Clear Ownership**: Each module owns its internal metrics
2. **No Coupling**: Telemetry module doesn't need to know about orchestration internals
3. **Simpler Maintenance**: Changes to module-specific metrics don't require telemetry module changes
4. **Contract Stability**: `unified_metrics.go` only changes when cross-module contracts evolve

**When to Add to `unified_metrics.go`**:
- The metric pattern is used by **2+ modules** with identical semantics
- The metric is part of a framework-wide observability contract
- Dashboard queries need to aggregate across modules using the same metric name

---

## Global Singleton Pattern

### Implementation

**Source**: `telemetry/registry.go`

```go
var (
    // Global registry singleton
    globalRegistry atomic.Value  // Stores *Registry

    // Ensures single initialization
    initOnce sync.Once
)

// Initialize sets up the global telemetry system (ONCE)
func Initialize(config Config) error {
    var initErr error

    initOnce.Do(func() {
        registry, err := createRegistry(config)
        if err != nil {
            initErr = err
            return
        }

        // Store globally (atomic write)
        globalRegistry.Store(registry)
    })

    return initErr
}

// Counter uses the global registry (lock-free read)
func Counter(name string, labels ...string) {
    registry := globalRegistry.Load()  // Atomic read
    if registry != nil {
        r := registry.(*Registry)
        r.emitCounter(name, labels...)
    }
    // NoOp if not initialized - safe fallback
}
```

### Thread Safety Guarantees

1. **Initialization**: `sync.Once` ensures single initialization
2. **Global Access**: `atomic.Value` provides lock-free reads
3. **Concurrent Emission**: All emission operations are thread-safe
4. **Shutdown**: Coordinated via context and internal synchronization

### Performance Characteristics

```go
// Benchmark results (Go 1.25, 16 cores)
BenchmarkCounter-16             50000000    25.3 ns/op    0 B/op    0 allocs/op
BenchmarkHistogram-16           30000000    38.7 ns/op    0 B/op    0 allocs/op
BenchmarkGauge-16               40000000    29.1 ns/op    0 B/op    0 allocs/op
```

**Hot Path Optimization**: The critical path (metric emission) uses atomic operations only - no mutexes.

### Initialization Lifecycle

```go
// Application startup sequence
func main() {
    // 1. Initialize telemetry FIRST (before components)
    if err := telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileProduction)); err != nil {
        log.Fatalf("Telemetry init failed: %v", err)
    }

    // 2. Ensure cleanup on exit
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        if err := telemetry.Shutdown(ctx); err != nil {
            log.Printf("Telemetry shutdown error: %v", err)
        }
    }()

    // 3. Create components (they'll use global registry)
    tool := NewWeatherTool()

    // 4. Run framework
    framework, _ := core.NewFramework(tool, ...)
    framework.Run(context.Background())
}
```

---

## OpenTelemetry Integration

### Architecture

```go
// OTelProvider bridges GoMind and OpenTelemetry
type OTelProvider struct {
    tracer         trace.Tracer             // Distributed tracing
    meter          metric.Meter             // Metrics collection
    traceProvider  *sdktrace.TracerProvider // Manages trace export
    metricProvider *sdkmetric.MeterProvider // Manages metric export
    metrics        *MetricInstruments       // Pre-registered instruments
}
```

### OTLP/HTTP Export Pipeline

```
┌─────────────────────────────────────────────────────┐
│ Application Code                                    │
│                                                     │
│ telemetry.Counter("requests")  ← Simple API call  │
└─────────────────────────────────────────────────────┘
              │
              ↓
┌─────────────────────────────────────────────────────┐
│ Global Registry (Singleton)                         │
│                                                     │
│ - Cardinality limiting                              │
│ - Circuit breaker protection                        │
│ - Rate limiting                                     │
└─────────────────────────────────────────────────────┘
              │
              ↓
┌─────────────────────────────────────────────────────┐
│ OTelProvider                                        │
│                                                     │
│ ┌─────────────┐    ┌─────────────┐                │
│ │ Metric      │    │ Trace       │                │
│ │ Instruments │    │ Provider    │                │
│ └─────────────┘    └─────────────┘                │
└─────────────────────────────────────────────────────┘
              │
              ↓
┌─────────────────────────────────────────────────────┐
│ OTEL SDK (Batching & Processing)                   │
│                                                     │
│ - Batch metrics every 30 seconds                    │
│ - Batch traces immediately                          │
│ - Compress payloads                                 │
└─────────────────────────────────────────────────────┘
              │
              │ OTLP/HTTP Protocol
              ↓
┌─────────────────────────────────────────────────────┐
│ OTLP Endpoint (http://otel-collector:4318)         │
│                                                     │
│ Content-Type: application/x-protobuf                │
│ Gzip compression enabled                            │
└─────────────────────────────────────────────────────┘
```

### Endpoint Configuration

**Standard Environment Variables** (OpenTelemetry specification):

```bash
# Primary configuration
OTEL_EXPORTER_OTLP_ENDPOINT="http://otel-collector:4318"

# Service identification
OTEL_SERVICE_NAME="weather-service"

# Optional: Sampling rate for traces
OTEL_TRACES_SAMPLER="always_on"

# Optional: Export protocol (defaults to http/protobuf)
OTEL_EXPORTER_OTLP_PROTOCOL="http/protobuf"
```

**GoMind Configuration Priority**:
1. Explicit `Config` passed to `Initialize()`
2. `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable
3. `GOMIND_TELEMETRY_ENDPOINT` (framework-specific)
4. Default: `http://localhost:4318`

### Metric Type Mapping

GoMind automatically maps metric names to appropriate OpenTelemetry instrument types:

| GoMind API | Naming Pattern | OTEL Instrument | Use Case |
|------------|----------------|-----------------|----------|
| `Counter()` | `*count*`, `*total*`, `*errors*` | Counter (monotonic) | Cumulative counts |
| `Histogram()` | `*duration*`, `*latency*`, `*time*` | Histogram | Latency distributions |
| `Gauge()` | `*gauge*`, `*current*`, `*active*` | Histogram (as proxy) | Current values |

**Implementation** (`telemetry/otel.go:132-158`):

```go
func (o *OTelProvider) RecordMetric(name string, value float64, labels map[string]string) {
    // Heuristic-based routing
    switch {
    case contains(name, "duration", "latency", "time"):
        o.metrics.RecordHistogram(ctx, name, value, attrs...)
    case contains(name, "count", "total", "errors"):
        o.metrics.RecordCounter(ctx, name, int64(value), attrs...)
    case contains(name, "gauge", "current", "size"):
        o.metrics.RecordHistogram(ctx, name, value, attrs...)
    default:
        o.metrics.RecordHistogram(ctx, name, value, attrs...)
    }
}
```

**Why Histograms for Gauges?**: OpenTelemetry Gauges require callback functions. Using Histograms for gauge-like metrics provides simpler API while maintaining semantic correctness for most use cases.

---

## Integration Patterns

### Pattern 1: Tool Integration (Passive Components)

**Scenario**: Weather service tool that responds to requests.

```go
// weather_tool.go
package main

import (
    "context"
    "time"
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

type WeatherTool struct {
    *core.BaseTool
}

func NewWeatherTool() *WeatherTool {
    return &WeatherTool{
        BaseTool: core.NewTool("weather-service"),
    }
}

// Tool handlers emit metrics directly
func (w *WeatherTool) handleWeatherRequest(rw http.ResponseWriter, r *http.Request) {
    start := time.Now()

    // Track request started
    telemetry.Counter("weather.requests.total",
        "endpoint", "current_weather",
        "method", r.Method)

    // Process request
    result, err := w.fetchWeather(r.Context())

    // Track completion
    status := "success"
    if err != nil {
        status = "error"
        telemetry.Counter("weather.errors.total",
            "type", "fetch_failed",
            "endpoint", "current_weather")
    }

    // Track latency
    duration := time.Since(start).Milliseconds()
    telemetry.Histogram("weather.request.duration_ms", float64(duration),
        "endpoint", "current_weather",
        "status", status)

    // Response handling...
}

// main.go
func main() {
    // CRITICAL: Initialize telemetry BEFORE creating components
    telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileProduction))
    defer telemetry.Shutdown(context.Background())

    tool := NewWeatherTool()
    framework, _ := core.NewFramework(tool, ...)
    framework.Run(context.Background())
}
```

### Pattern 2: Agent Integration (Active Components)

**Scenario**: Orchestrator agent that discovers and coordinates tools.

```go
// orchestrator_agent.go
package main

import (
    "context"
    "time"
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

type OrchestratorAgent struct {
    *core.BaseAgent
}

func NewOrchestratorAgent() *OrchestratorAgent {
    return &OrchestratorAgent{
        BaseAgent: core.NewBaseAgent("orchestrator"),
    }
}

// Agents emit metrics during orchestration
func (o *OrchestratorAgent) ExecuteWorkflow(ctx context.Context, workflowID string) error {
    start := time.Now()

    // Add workflow context for distributed tracing
    ctx = telemetry.WithBaggage(ctx,
        "workflow_id", workflowID,
        "operation", "execute_workflow")

    // Track workflow start
    telemetry.Counter("orchestrator.workflows.started",
        "workflow_id", workflowID)

    // Discover available tools
    tools, err := o.Discover(ctx, core.DiscoveryFilter{
        Type: core.ComponentTypeTool,
    })

    if err != nil {
        telemetry.Counter("orchestrator.errors.total",
            "type", "discovery_failed")
        return err
    }

    // Track discovered tools
    telemetry.Gauge("orchestrator.tools.discovered", float64(len(tools)),
        "workflow_id", workflowID)

    // Orchestrate tools (parallel execution)
    var wg sync.WaitGroup
    for _, tool := range tools {
        wg.Add(1)
        go func(t *core.ServiceInfo) {
            defer wg.Done()

            toolStart := time.Now()
            err := o.invokeTool(ctx, t)

            status := "success"
            if err != nil {
                status = "error"
            }

            telemetry.Histogram("orchestrator.tool.invocation.duration_ms",
                float64(time.Since(toolStart).Milliseconds()),
                "tool_name", t.Name,
                "status", status)
        }(tool)
    }

    wg.Wait()

    // Track workflow completion
    duration := time.Since(start).Milliseconds()
    telemetry.Counter("orchestrator.workflows.completed",
        "workflow_id", workflowID,
        "status", "success")

    telemetry.Histogram("orchestrator.workflow.duration_ms", float64(duration),
        "workflow_id", workflowID)

    return nil
}

// main.go
func main() {
    // Initialize telemetry
    telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileProduction))
    defer telemetry.Shutdown(context.Background())

    agent := NewOrchestratorAgent()
    framework, _ := core.NewFramework(agent, ...)
    framework.Run(context.Background())
}
```

### Pattern 3: Context Propagation (Distributed Tracing)

**Scenario**: Request flows across multiple services.

```go
// Service A (Entry point)
func (a *ServiceA) HandleRequest(ctx context.Context, req Request) {
    // Add baggage for distributed context
    ctx = telemetry.WithBaggage(ctx,
        "request_id", req.ID,
        "user_id", req.UserID,
        "tenant_id", req.TenantID)

    // Start span for this service
    ctx, span := telemetry.StartSpan(ctx, "service-a.handle-request")
    defer span.End()

    span.SetAttribute("request.size", len(req.Data))

    // Call Service B (context propagates automatically)
    result, err := a.callServiceB(ctx, req)

    if err != nil {
        span.RecordError(err)
    }
}

// Service B (Downstream)
func (b *ServiceB) ProcessRequest(ctx context.Context, data []byte) {
    // Extract baggage automatically
    requestID := telemetry.GetBaggage(ctx, "request_id")
    userID := telemetry.GetBaggage(ctx, "user_id")

    // Start span (parent span is in context)
    ctx, span := telemetry.StartSpan(ctx, "service-b.process-request")
    defer span.End()

    // Metrics include baggage automatically
    telemetry.Counter("service-b.requests",
        "user_id", userID,  // From context
        "request_id", requestID)

    // Processing...
}
```

**Result**: Complete trace across all services with correlated metrics.

---

## OTLP Pipeline Architecture

### Production Deployment Stack

```
┌─────────────────────────────────────────────────────────────┐
│ Kubernetes Cluster (gomind-examples namespace)              │
│                                                             │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐  │
│  │ Tool Pods    │   │ Agent Pods   │   │ Other Pods   │  │
│  │              │   │              │   │              │  │
│  │ telemetry.   │   │ telemetry.   │   │ telemetry.   │  │
│  │ Counter()    │   │ Counter()    │   │ Counter()    │  │
│  └──────────────┘   └──────────────┘   └──────────────┘  │
│         │                    │                    │         │
│         │  OTLP/HTTP         │  OTLP/HTTP         │         │
│         └────────────────────┴────────────────────┘         │
│                              │                               │
│                              ↓                               │
│  ┌───────────────────────────────────────────────────────┐ │
│  │ OTEL Collector (Sidecar or Deployment)               │ │
│  │                                                       │ │
│  │ Receivers:                                            │ │
│  │   - otlp (http: 4318, grpc: 4317)                   │ │
│  │                                                       │ │
│  │ Processors:                                           │ │
│  │   - memory_limiter (256MB limit)                     │ │
│  │   - batch (512 metrics, 10s timeout)                 │ │
│  │                                                       │ │
│  │ Exporters:                                            │ │
│  │   - prometheus (port 8889)                           │ │
│  │   - otlp/jaeger (port 4317)                          │ │
│  └───────────────────────────────────────────────────────┘ │
│                      │                       │               │
└──────────────────────┼───────────────────────┼──────────────┘
                       │                       │
          ┌────────────┴────────┐   ┌──────────┴─────────┐
          │                     │   │                    │
          ↓                     │   ↓                    │
┌──────────────────┐            │ ┌──────────────────┐  │
│ Prometheus       │            │ │ Jaeger           │  │
│                  │            │ │                  │  │
│ - Scrapes :8889  │            │ │ - Receives       │  │
│   every 15s      │            │ │   traces via     │  │
│ - Stores metrics │            │ │   OTLP/gRPC      │  │
│ - Alerts         │            │ │ - Trace search   │  │
└──────────────────┘            │ └──────────────────┘  │
          │                     │                        │
          ↓                     │                        │
┌──────────────────┐            │                        │
│ Grafana          │            │                        │
│                  │            │                        │
│ - Visualizes     │            │                        │
│   metrics        │            │                        │
│ - Dashboards     │            │                        │
│ - Alerts         │            │                        │
└──────────────────┘            │                        │
                                │                        │
                    ┌───────────┴────────────────────────┘
                    │
                    ↓
            [ Operations Team ]
```

### OTEL Collector Configuration

**Key Configuration** (`examples/k8-deployment/otel-collector.yaml`):

```yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318  # GoMind uses HTTP by default
      grpc:
        endpoint: 0.0.0.0:4317  # Available but not primary

processors:
  memory_limiter:
    limit_mib: 256
    check_interval: 1s

  batch:
    timeout: 10s
    send_batch_size: 512
    send_batch_max_size: 1024

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
    namespace: "gomind"
    const_labels:
      cluster: "gomind-examples"

  otlp/jaeger:
    endpoint: jaeger-collector:4317  # gRPC for Jaeger
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp/jaeger]

    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [prometheus]
```

### Common Deployment Patterns

#### Pattern A: Sidecar Collector (Recommended for Production)

```yaml
# Each application pod has OTEL collector sidecar
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: weather-tool
        image: weather-tool:latest
        env:
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          value: "http://localhost:4318"  # Sidecar

      - name: otel-collector
        image: otel/opentelemetry-collector:latest
        ports:
        - containerPort: 4318
```

**Benefits**:
- Low latency (localhost)
- Pod-level isolation
- Scales with application

**Tradeoffs**:
- More resource usage
- Collector per pod

#### Pattern B: Centralized Collector (Current Setup)

```yaml
# Single OTEL collector deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otel-collector
spec:
  replicas: 1  # Or HPA-scaled
```

**Benefits**:
- Fewer resources
- Central configuration
- Easier management

**Tradeoffs**:
- Network hop required
- Single point of failure (mitigated by replicas)

---

## Production Deployment

### Pre-Deployment Checklist

#### ✅ Application Code

- [ ] Import telemetry module: `import "github.com/itsneelabh/gomind/telemetry"`
- [ ] Call `telemetry.Initialize()` in `main()` before component creation
- [ ] Use `defer telemetry.Shutdown(ctx)` for graceful shutdown
- [ ] Add metrics to critical code paths (handlers, workflows)
- [ ] Use consistent metric naming conventions
- [ ] Test telemetry locally before deploying

#### ✅ Kubernetes Configuration

- [ ] Set `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable
- [ ] Set `OTEL_SERVICE_NAME` for service identification (optional)
- [ ] Deploy OTEL Collector with correct configuration
- [ ] Deploy Prometheus for metrics storage
- [ ] Deploy Jaeger for trace visualization
- [ ] Deploy Grafana for dashboards
- [ ] Configure Prometheus scraping intervals
- [ ] Set up persistent storage for Prometheus data

#### ✅ Monitoring Configuration

- [ ] Create Grafana dashboards for key metrics
- [ ] Set up Prometheus alerts for anomalies
- [ ] Configure alert routing (PagerDuty, Slack, etc.)
- [ ] Test alert firing with synthetic errors
- [ ] Document dashboard access URLs
- [ ] Set up log aggregation for telemetry errors

### Environment Variables Reference

```bash
# Required for telemetry to function
OTEL_EXPORTER_OTLP_ENDPOINT="http://otel-collector:4318"

# Recommended for production
OTEL_SERVICE_NAME="weather-service"
OTEL_SERVICE_NAMESPACE="production"
OTEL_SERVICE_VERSION="v1.2.3"

# Optional: Sampling configuration
OTEL_TRACES_SAMPLER="always_on"           # Development
OTEL_TRACES_SAMPLER="traceidratio"        # Production
OTEL_TRACES_SAMPLER_ARG="0.1"             # Sample 10%

# Optional: Resource attributes
OTEL_RESOURCE_ATTRIBUTES="deployment.environment=production,cluster.name=us-west-2"
```

### Dockerfile Best Practices

```dockerfile
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build with telemetry support
RUN CGO_ENABLED=0 GOOS=linux go build -o weather-tool .

# Runtime image
FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /app/weather-tool .

# Expose application port
EXPOSE 8080

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

# Run with proper signal handling
ENTRYPOINT ["/app/weather-tool"]
```

### Kubernetes Deployment Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: weather-tool
  namespace: gomind-examples
spec:
  replicas: 3
  selector:
    matchLabels:
      app: weather-tool
  template:
    metadata:
      labels:
        app: weather-tool
        version: v1.0.0
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      containers:
      - name: weather-tool
        image: weather-tool:v1.0.0
        ports:
        - containerPort: 8080
          name: http
        env:
        # Telemetry configuration
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          value: "http://otel-collector:4318"
        - name: OTEL_SERVICE_NAME
          value: "weather-tool"
        - name: OTEL_SERVICE_NAMESPACE
          value: "production"

        # Application configuration
        - name: REDIS_URL
          value: "redis://redis:6379"

        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"

        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30

        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
```

---

## Common Pitfalls

### Pitfall 1: Forgetting to Initialize

**Problem**:
```go
func main() {
    // ❌ NO telemetry initialization
    tool := NewWeatherTool()
    framework, _ := core.NewFramework(tool, ...)
    framework.Run(context.Background())
}

// Later in code...
func (t *Tool) handleRequest() {
    telemetry.Counter("requests")  // Silent NoOp - no metrics emitted!
}
```

**Symptom**: No metrics appear in Prometheus, no errors logged.

**Solution**:
```go
func main() {
    // ✅ Initialize FIRST
    if err := telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileProduction)); err != nil {
        log.Fatalf("Telemetry init failed: %v", err)
    }
    defer telemetry.Shutdown(context.Background())

    tool := NewWeatherTool()
    framework, _ := core.NewFramework(tool, ...)
    framework.Run(context.Background())
}
```

### Pitfall 2: Wrong Endpoint Configuration

**Problem**:
```yaml
env:
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: "otel-collector:4318"  # ❌ Missing http:// prefix
```

**Symptom**: Telemetry initialization fails with connection error.

**Solution**:
```yaml
env:
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: "http://otel-collector:4318"  # ✅ Full URL
```

### Pitfall 3: Forgetting Shutdown

**Problem**:
```go
func main() {
    telemetry.Initialize(...)
    // ❌ No shutdown - metrics buffered in memory may not be exported

    tool := NewWeatherTool()
    framework, _ := core.NewFramework(tool, ...)
    framework.Run(context.Background())
    // Program exits - buffered metrics lost
}
```

**Symptom**: Metrics occasionally missing, especially on short-lived processes.

**Solution**:
```go
func main() {
    telemetry.Initialize(...)

    // ✅ Always defer shutdown
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        telemetry.Shutdown(ctx)
    }()

    // Rest of application...
}
```

### Pitfall 4: High Cardinality Labels

**Problem**:
```go
// ❌ User ID as label = millions of unique time series
telemetry.Counter("requests.total",
    "user_id", userID)  // Cardinality explosion!
```

**Symptom**: Memory usage grows unbounded, Prometheus struggles.

**Solution**:
```go
// ✅ Use low-cardinality labels
telemetry.Counter("requests.total",
    "endpoint", "/api/users",    // ~100 endpoints
    "status", "success",          // 2-3 values
    "region", "us-west-2")        // ~10 regions

// ✅ Use baggage for high-cardinality data (tracing only)
ctx = telemetry.WithBaggage(ctx, "user_id", userID)
```

**Built-in Protection**: GoMind telemetry has cardinality limiter (default: 1000 unique combinations).

### Pitfall 5: OTEL Collector Misconfiguration

**Problem**:
```yaml
exporters:
  otlp/jaeger:
    endpoint: http://jaeger-collector:4318  # ❌ HTTP endpoint
    # Missing: Protocol specification
```

**Symptom**:
```
error reading server preface: http2: failed reading the frame payload
```

**Solution**:
```yaml
exporters:
  # Option 1: Use HTTP exporter
  otlphttp/jaeger:
    endpoint: http://jaeger-collector:4318

  # Option 2: Use gRPC port
  otlp/jaeger:
    endpoint: jaeger-collector:4317  # gRPC
    tls:
      insecure: true
```

### Pitfall 6: Missing Error Handling

**Problem**:
```go
// ❌ Ignoring initialization errors
telemetry.Initialize(config)  // Error ignored

// Application continues with NoOp telemetry
```

**Solution**:
```go
// ✅ Handle initialization errors appropriately
if err := telemetry.Initialize(config); err != nil {
    // Option A: Fail fast (recommended for production)
    log.Fatalf("Telemetry required but init failed: %v", err)

    // Option B: Log and continue (acceptable for development)
    log.Printf("WARNING: Running without telemetry: %v", err)
}
```

---

## Troubleshooting Guide

### Issue: No Metrics Appearing in Prometheus

**Diagnostic Steps**:

1. **Check if telemetry is initialized**:
   ```bash
   # In application logs
   grep "Telemetry initialized" /var/log/app.log
   ```

2. **Verify OTEL Collector is receiving data**:
   ```bash
   kubectl port-forward -n gomind-examples svc/otel-collector 4318:4318

   # Send test metric
   curl -X POST http://localhost:4318/v1/metrics \
     -H "Content-Type: application/json" \
     -d '{"resourceMetrics":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"test"}}]},"instrumentationLibraryMetrics":[{"metrics":[{"name":"test_metric","gauge":{"dataPoints":[{"asDouble":42.0}]}}]}]}]}'

   # Should return: {"partialSuccess":{}}
   ```

3. **Check OTEL Collector logs**:
   ```bash
   kubectl logs -n gomind-examples deployment/otel-collector --tail=50

   # Look for:
   # ✅ "Traces exported successfully"
   # ✅ "Metrics exported successfully"
   # ❌ Connection errors
   # ❌ Configuration errors
   ```

4. **Verify Prometheus scraping**:
   ```bash
   kubectl port-forward -n gomind-examples svc/prometheus 9090:9090

   # Open http://localhost:9090/targets
   # Check otel-collector target status
   ```

**Common Causes**:
- Application not calling `telemetry.Initialize()`
- Wrong `OTEL_EXPORTER_OTLP_ENDPOINT` value
- OTEL Collector not running
- Prometheus not scraping OTEL Collector

### Issue: Jaeger Not Showing Traces

**Diagnostic Steps**:

1. **Check OTEL Collector → Jaeger connection**:
   ```bash
   kubectl logs -n gomind-examples deployment/otel-collector | grep jaeger

   # Look for connection errors
   ```

2. **Verify Jaeger is receiving data**:
   ```bash
   kubectl port-forward -n gomind-examples svc/jaeger-query 16686:80

   # Open http://localhost:16686
   # Check "Search" tab for traces
   ```

3. **Test trace submission**:
   ```go
   // In application code
   ctx, span := telemetry.StartSpan(ctx, "test-operation")
   defer span.End()

   span.SetAttribute("test", "value")
   time.Sleep(100 * time.Millisecond)
   ```

**Common Causes**:
- OTEL Collector exporter misconfigured (gRPC vs HTTP)
- Jaeger not running or not accessible
- Traces not being generated by application code

### Issue: High Memory Usage

**Diagnostic Steps**:

1. **Check metric cardinality**:
   ```bash
   # In Prometheus
   # Run query: sum(count by(__name__)({__name__=~".+"}))

   # If > 10,000 time series: High cardinality problem
   ```

2. **Check OTEL Collector memory**:
   ```bash
   kubectl top pods -n gomind-examples | grep otel-collector

   # If memory > 256MB: Increase memory_limiter
   ```

3. **Enable cardinality protection**:
   ```go
   config := telemetry.Config{
       Profile: telemetry.ProfileProduction,
       CardinalityLimit: 1000,  // Limit unique label combinations
   }
   ```

**Common Causes**:
- User IDs or timestamps as labels
- Too many unique label combinations
- Memory limiter not configured in OTEL Collector

### Issue: Slow Performance

**Diagnostic Steps**:

1. **Check if circuit breaker is open**:
   ```go
   health := telemetry.GetHealth()
   fmt.Printf("Circuit state: %s\n", health.CircuitState)

   // If "open": Backend is down or slow
   ```

2. **Measure emission overhead**:
   ```go
   start := time.Now()
   for i := 0; i < 10000; i++ {
       telemetry.Counter("test.metric")
   }
   duration := time.Since(start)
   fmt.Printf("10k emissions: %v (%.2f µs/op)\n",
       duration, float64(duration.Microseconds())/10000)

   // Should be: <5 µs/op
   ```

**Common Causes**:
- Slow network to OTEL Collector
- OTEL Collector overloaded
- Too frequent metric emission

### Issue: Metrics Delayed

**Diagnostic Steps**:

1. **Check batch export interval**:
   ```go
   // OpenTelemetry exports every 30 seconds by default
   // This is expected behavior
   ```

2. **Force export for testing**:
   ```go
   // Shutdown forces export
   telemetry.Shutdown(context.Background())
   ```

3. **Reduce export interval** (not recommended for production):
   ```yaml
   # In OTEL Collector config
   processors:
     batch:
       timeout: 5s  # Faster export (higher overhead)
   ```

**Common Causes**:
- Normal batching behavior (30s interval)
- OTEL Collector batch processor timeout
- Prometheus scrape interval

---

## Performance Characteristics

### Benchmarks

```
Benchmark Results (Go 1.25.0, darwin/arm64, Apple M1 Pro)

BenchmarkCounter-10                    50000000    25.3 ns/op      0 B/op    0 allocs/op
BenchmarkHistogram-10                  30000000    38.7 ns/op      0 B/op    0 allocs/op
BenchmarkGauge-10                      40000000    29.1 ns/op      0 B/op    0 allocs/op
BenchmarkCounterWithLabels-10          20000000    56.8 ns/op     48 B/op    1 allocs/op
BenchmarkStartSpan-10                  10000000   112.4 ns/op     96 B/op    2 allocs/op
BenchmarkBaggagePropagation-10          5000000   234.1 ns/op    128 B/op    3 allocs/op
```

### Resource Usage (Per Component)

| Metric | Development | Production | Notes |
|--------|-------------|------------|-------|
| Memory Baseline | ~5 MB | ~8 MB | Before initialization |
| Memory After Init | ~15 MB | ~25 MB | With OpenTelemetry SDK |
| Memory Per 10k Metrics | +2 MB | +2 MB | Batched exports |
| CPU Per 1M Metrics/sec | ~5% | ~5% | On 4-core system |
| Network Bandwidth | ~1 KB/s | ~10 KB/s | Depends on metric volume |

### Scalability Limits

| Dimension | Limit | Mitigation |
|-----------|-------|------------|
| Unique Metric Names | 10,000 | Use consistent naming conventions |
| Label Combinations | 1,000 (default) | Cardinality limiter enforced |
| Metrics/Second | 100,000 | Batching and sampling |
| Concurrent Emitters | Unlimited | Lock-free atomic operations |
| Trace Spans/Second | 10,000 | Sampling (1%-10% recommended) |

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-09-28 | Initial architecture documentation |

---

## Related Documentation

- [Telemetry Module README](./README.md) - User-facing documentation and quick start
- [Framework Design Principles](../FRAMEWORK_DESIGN_PRINCIPLES.md) - Overall framework architecture
- [Core Module Design](../core/CORE_DESIGN_PRINCIPLES.md) - Core module architectural rules
- [OpenTelemetry Specification](https://opentelemetry.io/docs/specs/otel/) - OTLP protocol details

---

**Remember**: The telemetry module is designed to be **invisible when it works, obvious when it doesn't**. Follow the patterns in this document to ensure reliable production observability.