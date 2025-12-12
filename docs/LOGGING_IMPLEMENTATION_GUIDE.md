# Logging Implementation Guide

Welcome to the GoMind logging guide! This document explains how to implement consistent, production-ready logging across your tools and agents. Think of this as your complete reference for doing logging the right way.

## Table of Contents

- [Why This Guide Exists](#why-this-guide-exists)
- [The Logger Interface](#the-logger-interface)
- [Log Levels Explained](#log-levels-explained)
- [Environment Configuration](#environment-configuration)
- [Where to Use Each Logger Method](#where-to-use-each-logger-method)
- [Agent Logging: Complete Example](#agent-logging-complete-example)
- [Tool Logging: Complete Example](#tool-logging-complete-example)
- [Handler Logging with Trace Correlation](#handler-logging-with-trace-correlation)
- [Structured Logging: Field Naming Standards](#structured-logging-field-naming-standards)
- [The Mixed Logging Problem](#the-mixed-logging-problem)
- [Telemetry Integration](#telemetry-integration)
- [Component-Aware Logging for Framework Modules](#component-aware-logging-for-framework-modules)
- [Common Mistakes and How to Avoid Them](#common-mistakes-and-how-to-avoid-them)
- [Quick Reference](#quick-reference)

---

## Why This Guide Exists

In a distributed system with multiple agents and tools, logs are your primary debugging tool. Without consistent logging:

- You can't correlate requests across services
- You can't filter logs effectively in production
- You waste hours debugging issues that should take minutes

This guide ensures every GoMind component logs in a consistent, useful way.

---

## The Logger Interface

GoMind uses a custom `Logger` interface defined in [`core/interfaces.go:11-23`](../core/interfaces.go#L11-L23). This design:

- **Avoids vendor lock-in** (not tied to zap, logrus, zerolog, etc.)
- **Is minimal and composable** (easy to test and mock)
- **Supports trace correlation** (via context-aware methods)

### The Interface Definition

```go
// From core/interfaces.go
type Logger interface {
    // Basic logging methods (no trace correlation)
    Info(msg string, fields map[string]interface{})
    Error(msg string, fields map[string]interface{})
    Warn(msg string, fields map[string]interface{})
    Debug(msg string, fields map[string]interface{})

    // Context-aware methods (with trace correlation)
    InfoWithContext(ctx context.Context, msg string, fields map[string]interface{})
    ErrorWithContext(ctx context.Context, msg string, fields map[string]interface{})
    WarnWithContext(ctx context.Context, msg string, fields map[string]interface{})
    DebugWithContext(ctx context.Context, msg string, fields map[string]interface{})
}
```

### Why Two Sets of Methods?

| Method Type | When to Use | Example Location |
|-------------|-------------|------------------|
| Basic (no context) | Startup, shutdown, background tasks | `main()`, init functions |
| WithContext | HTTP handlers, any request processing | Handler functions |

**The golden rule**: If you have access to `context.Context` from an HTTP request, use the `WithContext` methods. They enable trace-log correlation, which is essential for debugging in production.

### Default Logger Behavior

When you create a component with `core.NewBaseAgent()` or `core.NewTool()`, the Logger is initially set to `NoOpLogger` (a silent logger defined in [`core/interfaces.go:110-121`](../core/interfaces.go#L110-L121)). The framework replaces this with a `ProductionLogger` when you call `core.NewFramework()`.

---

## Log Levels Explained

GoMind uses four standard log levels, from most to least verbose:

| Level | When to Use | Example |
|-------|-------------|---------|
| **DEBUG** | Detailed flow information for troubleshooting | "Executing step 3 of workflow" |
| **INFO** | Significant events, lifecycle changes | "Request completed successfully" |
| **WARN** | Unexpected but recoverable situations | "Retrying request (attempt 2/3)" |
| **ERROR** | Failures that need attention | "Failed to connect to database" |

### Level Hierarchy

```
DEBUG (0) → INFO (1) → WARN (2) → ERROR (3)
```

> **Source**: [`core/config.go:1500-1512`](../core/config.go#L1500-L1512) (LogLevel constants)

When you set `GOMIND_LOG_LEVEL=INFO`, you see INFO, WARN, and ERROR logs. DEBUG logs are hidden.

### Production Recommendations

| Environment | Recommended Level |
|-------------|-------------------|
| Development | DEBUG |
| Staging | INFO |
| Production | INFO (or WARN for high-volume services) |

---

## Environment Configuration

GoMind logging is configured through environment variables:

### Core Environment Variables

| Variable | Values | Default | Description |
|----------|--------|---------|-------------|
| `GOMIND_LOG_LEVEL` | debug, info, warn, error | info | Minimum level to log |
| `GOMIND_LOG_FORMAT` | json, text | json | Output format |
| `GOMIND_DEBUG` | true, false | false | Enable debug mode |

> **Source**: [`core/config.go:213-218`](../core/config.go#L213-L218) (LoggingConfig struct)

### Format Behavior

The framework's `ProductionLogger` uses the format from configuration (defaults to JSON).

The telemetry module's `TelemetryLogger` has additional auto-detection logic:

```go
// From telemetry/logger.go:76-79
if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
    format = "json" // Use JSON in K8s for log aggregation
}
```

**Recommendation**: For consistency, explicitly set `GOMIND_LOG_FORMAT`:
- **Production/Kubernetes**: `json` (for log aggregation tools like Loki, ELK)
- **Local development**: `text` (human-readable)

### Output Format Examples

**Text format (local development):**
```
2024-01-15T10:30:45Z [INFO] [weather-service] Processing weather request lat=35.67 lon=139.65
```

**JSON format (production/K8s):**
```json
{
  "timestamp": "2024-01-15T10:30:45Z",
  "level": "INFO",
  "service": "weather-service",
  "component": "framework",
  "message": "Processing weather request",
  "lat": 35.67,
  "lon": 139.65
}
```

---

## Where to Use Each Logger Method

This is the most important section. Understanding when to use each method prevents the inconsistencies that make debugging difficult.

### Decision Tree

```
Are you in a function that received context.Context from an HTTP request?
├── YES → Use WithContext methods
│         t.Logger.InfoWithContext(ctx, "message", fields)
│
└── NO → Use basic methods
         t.Logger.Info("message", fields)
```

### Specific Locations

| Location | Method to Use | Why |
|----------|---------------|-----|
| `main()` startup | `Info()` / `Error()` | No request context exists yet |
| `initTelemetry()` | `Info()` / `Error()` | Background initialization |
| HTTP handler | `InfoWithContext()` | Request context available for tracing |
| Background goroutine | `Info()` / `Error()` | No request context |
| Graceful shutdown | `Info()` | No request context |

---

## Agent Logging: Complete Example

Here's a complete, correctly-implemented agent with proper logging at every level.

### main.go (Startup Logging)

```go
package main

import (
    "context"
    "errors"
    "log"  // ONLY for fatal startup errors before framework is ready
    "os"
    "os/signal"
    "strconv"
    "syscall"
    "time"

    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

func main() {
    // 1. Validate configuration first (fail fast)
    // Use standard log here because framework isn't created yet
    if err := validateConfig(); err != nil {
        log.Fatalf("Configuration error: %v", err)
    }

    // 2. Initialize telemetry BEFORE creating agent
    initTelemetry("my-research-agent")
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := telemetry.Shutdown(ctx); err != nil {
            // Use standard log because we're shutting down
            log.Printf("Warning: Telemetry shutdown error: %v", err)
        }
    }()

    // 3. Create agent
    agent, err := NewResearchAgent()
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }

    // 4. Get port configuration
    port := 8080
    if portStr := os.Getenv("PORT"); portStr != "" {
        if p, err := strconv.Atoi(portStr); err == nil {
            port = p
        }
    }

    // 5. Create framework
    framework, err := core.NewFramework(agent,
        core.WithName("my-research-agent"),
        core.WithPort(port),
        core.WithNamespace(os.Getenv("NAMESPACE")),
        core.WithRedisURL(os.Getenv("REDIS_URL")),
        core.WithDiscovery(true, "redis"),
        core.WithCORS([]string{"*"}, true),
        core.WithDevelopmentMode(os.Getenv("DEV_MODE") == "true"),
        core.WithMiddleware(telemetry.TracingMiddleware("my-research-agent")),
    )
    if err != nil {
        log.Fatalf("Failed to create framework: %v", err)
    }

    // 6. Log startup information using the agent's Logger
    // At this point, framework has configured the agent's Logger
    agent.Logger.Info("Agent starting", map[string]interface{}{
        "id":           agent.GetID(),
        "name":         agent.GetName(),
        "port":         port,
        "capabilities": len(agent.Capabilities),
    })

    // 7. Set up graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-sigChan
        agent.Logger.Info("Shutting down gracefully", nil)
        cancel()
    }()

    // 8. Run the framework
    if err := framework.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
        agent.Logger.Error("Framework error", map[string]interface{}{
            "error": err.Error(),
        })
        os.Exit(1)
    }

    agent.Logger.Info("Shutdown completed", nil)
}
```

### research_agent.go (Agent Definition)

```go
package main

import (
    "github.com/itsneelabh/gomind/ai"
    "github.com/itsneelabh/gomind/core"
)

type ResearchAgent struct {
    *core.BaseAgent
}

func NewResearchAgent() (*ResearchAgent, error) {
    agent := core.NewBaseAgent("my-research-agent")

    // Auto-configure AI client
    aiClient, err := ai.NewClient()
    if err != nil {
        // Log warning but continue - AI is optional
        agent.Logger.Warn("AI client creation failed, some features limited", map[string]interface{}{
            "error": err.Error(),
        })
    } else {
        agent.AI = aiClient
        agent.Logger.Info("AI client configured", map[string]interface{}{
            "provider": "auto-detected",
        })
    }

    researchAgent := &ResearchAgent{
        BaseAgent: agent,
    }

    // Register capabilities
    researchAgent.registerCapabilities()

    agent.Logger.Info("Research agent created", map[string]interface{}{
        "capabilities": len(agent.Capabilities),
    })

    return researchAgent, nil
}

func (r *ResearchAgent) registerCapabilities() {
    r.RegisterCapability(core.Capability{
        Name:        "research_topic",
        Description: "Research a topic using available tools",
        Endpoint:    "/api/capabilities/research_topic",
        InputTypes:  []string{"json"},
        OutputTypes: []string{"json"},
        Handler:     r.handleResearchTopic,
    })

    r.Logger.Debug("Registered capability", map[string]interface{}{
        "name":     "research_topic",
        "endpoint": "/api/capabilities/research_topic",
    })
}
```

### handlers.go (Request Handlers - WITH Context)

```go
package main

import (
    "context"
    "encoding/json"
    "net/http"
    "time"
)

type ResearchRequest struct {
    Topic string `json:"topic"`
}

type ResearchResponse struct {
    Topic     string      `json:"topic"`
    Results   interface{} `json:"results"`
    Duration  string      `json:"duration"`
    RequestID string      `json:"request_id,omitempty"`
}

// handleResearchTopic processes research requests
// IMPORTANT: Always use WithContext methods in handlers!
func (r *ResearchAgent) handleResearchTopic(w http.ResponseWriter, req *http.Request) {
    startTime := time.Now()
    ctx := req.Context()  // Get context from request

    // Log request start WITH CONTEXT (enables trace correlation)
    r.Logger.InfoWithContext(ctx, "Processing research request", map[string]interface{}{
        "method": req.Method,
        "path":   req.URL.Path,
    })

    // Parse request
    var request ResearchRequest
    if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
        r.Logger.ErrorWithContext(ctx, "Failed to decode request", map[string]interface{}{
            "error": err.Error(),
        })
        http.Error(w, "Invalid request format", http.StatusBadRequest)
        return
    }

    // Validate request
    if request.Topic == "" {
        r.Logger.WarnWithContext(ctx, "Empty topic in request", nil)
        http.Error(w, "Topic is required", http.StatusBadRequest)
        return
    }

    r.Logger.DebugWithContext(ctx, "Request validated", map[string]interface{}{
        "topic": request.Topic,
    })

    // Perform research (your business logic here)
    results, err := r.performResearch(ctx, request.Topic)
    if err != nil {
        r.Logger.ErrorWithContext(ctx, "Research failed", map[string]interface{}{
            "topic": request.Topic,
            "error": err.Error(),
        })
        http.Error(w, "Research failed", http.StatusInternalServerError)
        return
    }

    // Build response
    response := ResearchResponse{
        Topic:    request.Topic,
        Results:  results,
        Duration: time.Since(startTime).String(),
    }

    // Log successful completion WITH CONTEXT
    r.Logger.InfoWithContext(ctx, "Research completed", map[string]interface{}{
        "topic":       request.Topic,
        "duration_ms": time.Since(startTime).Milliseconds(),
        "status":      "success",
    })

    // Send response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func (r *ResearchAgent) performResearch(ctx context.Context, topic string) (interface{}, error) {
    // Log internal operations with context for tracing
    r.Logger.DebugWithContext(ctx, "Starting tool discovery", map[string]interface{}{
        "topic": topic,
    })

    // ... your research logic here ...

    return nil, nil
}
```

---

## Tool Logging: Complete Example

Tools follow the same patterns as agents. Here's a weather tool example:

### main.go

```go
package main

import (
    "context"
    "errors"
    "log"
    "os"
    "os/signal"
    "strconv"
    "syscall"
    "time"

    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

func main() {
    if err := validateConfig(); err != nil {
        log.Fatalf("Configuration error: %v", err)
    }

    initTelemetry("weather-tool")
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        telemetry.Shutdown(ctx)
    }()

    tool := NewWeatherTool()

    port := 8080
    if portStr := os.Getenv("PORT"); portStr != "" {
        if p, err := strconv.Atoi(portStr); err == nil {
            port = p
        }
    }

    framework, err := core.NewFramework(tool,
        core.WithName("weather-tool"),
        core.WithPort(port),
        core.WithNamespace(os.Getenv("NAMESPACE")),
        core.WithRedisURL(os.Getenv("REDIS_URL")),
        core.WithDiscovery(true, "redis"),
        core.WithCORS([]string{"*"}, true),
        core.WithMiddleware(telemetry.TracingMiddleware("weather-tool")),
    )
    if err != nil {
        log.Fatalf("Failed to create framework: %v", err)
    }

    // Use tool's Logger after framework is created
    tool.Logger.Info("Weather tool starting", map[string]interface{}{
        "port":         port,
        "capabilities": len(tool.Capabilities),
    })

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-sigChan
        tool.Logger.Info("Shutting down", nil)
        cancel()
    }()

    if err := framework.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
        tool.Logger.Error("Framework error", map[string]interface{}{
            "error": err.Error(),
        })
    }
}
```

### weather_tool.go

```go
package main

import (
    "github.com/itsneelabh/gomind/core"
)

type WeatherTool struct {
    *core.BaseTool
}

func NewWeatherTool() *WeatherTool {
    tool := core.NewTool("weather-tool")

    weatherTool := &WeatherTool{
        BaseTool: tool,
    }

    weatherTool.registerCapabilities()

    return weatherTool
}

func (w *WeatherTool) registerCapabilities() {
    w.RegisterCapability(core.Capability{
        Name:        "get_weather",
        Description: "Get current weather for coordinates",
        Endpoint:    "/api/capabilities/get_weather",
        InputTypes:  []string{"json"},
        OutputTypes: []string{"json"},
        Handler:     w.handleGetWeather,
        InputSummary: &core.SchemaSummary{
            RequiredFields: []core.FieldHint{
                {Name: "lat", Type: "number", Example: "35.6762", Description: "Latitude"},
                {Name: "lon", Type: "number", Example: "139.6503", Description: "Longitude"},
            },
        },
    })
}
```

### handlers.go

```go
package main

import (
    "encoding/json"
    "net/http"
    "time"
)

type WeatherRequest struct {
    Lat float64 `json:"lat"`
    Lon float64 `json:"lon"`
}

type WeatherResponse struct {
    Temperature float64 `json:"temperature"`
    Condition   string  `json:"condition"`
    Location    string  `json:"location"`
}

// handleGetWeather processes weather requests
func (w *WeatherTool) handleGetWeather(rw http.ResponseWriter, req *http.Request) {
    startTime := time.Now()
    ctx := req.Context()

    // Always use WithContext in handlers
    w.Logger.InfoWithContext(ctx, "Processing weather request", map[string]interface{}{
        "method": req.Method,
        "path":   req.URL.Path,
    })

    var request WeatherRequest
    if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
        w.Logger.ErrorWithContext(ctx, "Failed to decode request", map[string]interface{}{
            "error": err.Error(),
        })
        http.Error(rw, "Invalid request", http.StatusBadRequest)
        return
    }

    w.Logger.DebugWithContext(ctx, "Fetching weather data", map[string]interface{}{
        "lat": request.Lat,
        "lon": request.Lon,
    })

    // Fetch weather data (your implementation)
    response := WeatherResponse{
        Temperature: 22.5,
        Condition:   "Sunny",
        Location:    "Tokyo, Japan",
    }

    w.Logger.InfoWithContext(ctx, "Weather request completed", map[string]interface{}{
        "lat":         request.Lat,
        "lon":         request.Lon,
        "duration_ms": time.Since(startTime).Milliseconds(),
    })

    rw.Header().Set("Content-Type", "application/json")
    json.NewEncoder(rw).Encode(response)
}
```

---

## Handler Logging with Trace Correlation

The `WithContext` methods enable trace-log correlation. Here's how it works:

### How Trace Correlation Works

1. **TracingMiddleware** extracts/creates trace context from incoming requests
2. **Context** carries the trace ID and span ID through your code
3. **WithContext methods** extract and include these IDs in log output

> **Source**: Trace context extraction is handled by [`telemetry/trace_context.go`](../telemetry/trace_context.go) and [`telemetry/framework_integration.go:67-83`](../telemetry/framework_integration.go#L67-L83)

### What Your Logs Look Like

**Without trace correlation (bad):**
```
10:00:00 [INFO] [weather-service] Processing weather request
10:00:00 [INFO] [weather-service] Processing weather request  <- Which is which?
10:00:01 [ERROR] [weather-service] Request failed             <- Which request?
```

**With trace correlation (good):**
```
10:00:00 [INFO] [weather-service] [req=abc123] Processing weather request
10:00:00 [INFO] [weather-service] [req=def456] Processing weather request
10:00:01 [ERROR] [weather-service] [req=abc123] Request failed  <- Clear!
```

### JSON Output with Trace Context

When using JSON format (production), trace context appears as fields:

```json
{
  "timestamp": "2024-01-15T10:00:00Z",
  "level": "INFO",
  "service": "weather-tool",
  "message": "Processing weather request",
  "trace.trace_id": "abc123def456789",
  "trace.span_id": "1234567890abcdef",
  "lat": 35.67,
  "lon": 139.65
}
```

---

## Structured Logging: Field Naming Standards

Consistent field names make log searching and filtering much easier.

### Standard Field Names

Use these field names across all your services:

| Field Name | Type | Description | Example |
|------------|------|-------------|---------|
| `operation` | string | The operation being performed | "research_topic", "get_weather" |
| `status` | string | Result status | "success", "error", "retry" |
| `error` | string | Error message | "connection refused" |
| `error_type` | string | Error classification | "timeout", "validation", "network" |
| `duration_ms` | number | Operation duration in milliseconds | 125 |
| `method` | string | HTTP method | "GET", "POST" |
| `path` | string | Request path | "/api/capabilities/get_weather" |
| `topic` | string | Research topic | "Tokyo weather" |
| `tool_name` | string | Tool being called | "weather-tool" |
| `capability` | string | Capability being invoked | "get_weather" |

### Good vs Bad Field Names

```go
// BAD - inconsistent naming
logger.Info("Request", map[string]interface{}{
    "time_taken": duration,     // Should be duration_ms
    "err": err.Error(),         // Should be error
    "api": "weather",           // Vague
})

// GOOD - consistent naming
logger.Info("Request completed", map[string]interface{}{
    "duration_ms": duration.Milliseconds(),
    "error":       err.Error(),
    "tool_name":   "weather-tool",
    "capability":  "get_weather",
})
```

---

## The Mixed Logging Problem

A common mistake is mixing Go's standard `log` package with the framework's Logger.

### The Problem

```go
func main() {
    // Creates standard log output - not integrated with framework
    log.Println("Starting agent...")

    agent := NewAgent()

    // Creates framework log output - different format, no correlation
    agent.Logger.Info("Agent created", nil)
}
```

This creates inconsistent output:

```
2024/01/15 10:00:00 Starting agent...                          <- Standard log format
2024-01-15T10:00:01Z [INFO] [my-agent] Agent created          <- Framework format
```

### The Solution

Use `log.Fatalf` only for unrecoverable startup errors. Once the framework is created, use the component's Logger exclusively:

```go
func main() {
    // BEFORE framework - standard log is acceptable for fatal errors
    if err := validateConfig(); err != nil {
        log.Fatalf("Configuration error: %v", err)
    }

    agent := NewAgent()
    framework, err := core.NewFramework(agent, ...)
    if err != nil {
        log.Fatalf("Framework creation failed: %v", err)
    }

    // AFTER framework - use component Logger exclusively
    agent.Logger.Info("Starting agent", map[string]interface{}{
        "port": port,
    })
}
```

---

## Telemetry Integration

Logging integrates with GoMind's telemetry system for metrics and tracing.

### Three-Layer Observability

GoMind's `ProductionLogger` ([`core/config.go:1532-1702`](../core/config.go#L1532-L1702)) implements three layers:

1. **Layer 1 - Console Output**: Always works, immediate visibility ([line 1626-1652](../core/config.go#L1626-L1652))
2. **Layer 2 - Metrics Emission**: When telemetry is initialized ([line 1674-1676](../core/config.go#L1674-L1676))
3. **Layer 3 - Trace Context**: When using `WithContext` methods ([line 1636-1643](../core/config.go#L1636-L1643))

### Enabling Telemetry

Initialize telemetry before creating your component:

```go
func main() {
    // Initialize telemetry FIRST
    initTelemetry("my-service")
    defer telemetry.Shutdown(context.Background())

    // Create component - Logger will auto-integrate with telemetry
    agent := NewAgent()
    framework, _ := core.NewFramework(agent, ...)
}

func initTelemetry(serviceName string) {
    env := os.Getenv("APP_ENV")
    if env == "" {
        env = "development"
    }

    var profile telemetry.Profile
    switch env {
    case "production", "prod":
        profile = telemetry.ProfileProduction
    case "staging", "stage":
        profile = telemetry.ProfileStaging
    default:
        profile = telemetry.ProfileDevelopment
    }

    config := telemetry.UseProfile(profile)
    config.ServiceName = serviceName

    if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
        config.Endpoint = endpoint
    }

    if err := telemetry.Initialize(config); err != nil {
        log.Printf("Warning: Telemetry initialization failed: %v", err)
        // Continue without telemetry - graceful degradation
    }
}
```

### Metric Emission from Logs

When telemetry is enabled, logs automatically emit metrics. Only specific low-cardinality fields are used as metric labels to prevent metric explosion:

**Allowed label fields** (from [`core/config.go:1689-1694`](../core/config.go#L1689-L1694)):
- `operation`
- `status`
- `error_type`
- `service_type`
- `provider`

```go
// This log statement...
agent.Logger.Error("Request failed", map[string]interface{}{
    "operation": "get_weather",
    "error_type": "timeout",
})

// ...also emits this metric (automatically):
// gomind.framework.operations{level="ERROR", service="my-agent", operation="get_weather", error_type="timeout"}
```

---

## Component-Aware Logging for Framework Modules

GoMind uses a component-based logging architecture that separates framework-level logs from agent/tool-level logs. This section explains how this segregation works and how to use it effectively.

### Understanding Log Segregation

Every log message in GoMind includes a `component` field that identifies the source of the log. Components are organized into categories:

| Category | Format | Examples |
|----------|--------|----------|
| Framework modules | `framework/<module>` | `framework/core`, `framework/orchestration`, `framework/resilience`, `framework/ai`, `framework/ui` |
| Agents | `agent/<name>` | `agent/travel-research-agent`, `agent/research-agent-telemetry` |
| Tools | `tool/<name>` | `tool/weather-service`, `tool/currency-service` |

This separation makes it easy to filter and analyze logs by origin.

### Real-World Example Logs

Here are actual logs from a deployed `research-agent-telemetry` agent in a Kubernetes cluster, showing how components are segregated:

**Framework Core Logs** (service discovery operations):
```json
{
  "component": "framework/core",
  "level": "INFO",
  "message": "Starting service discovery",
  "service": "research-agent-telemetry",
  "timestamp": "2025-12-12T20:24:41Z"
}

{
  "component": "framework/core",
  "level": "INFO",
  "message": "Service discovery completed",
  "service": "research-agent-telemetry",
  "services_checked": 11,
  "services_found": 11,
  "timestamp": "2025-12-12T20:24:41Z"
}
```

**Agent Handler Logs** (your application code):
```json
{
  "component": "agent/research-agent-telemetry",
  "level": "INFO",
  "message": "AI-powered tool+capability selection (1 call, 50% cost savings)",
  "capability": "current_weather",
  "tool": "weather-service",
  "topic": "weather in Tokyo",
  "timestamp": "2025-12-12T20:25:06Z"
}

{
  "component": "agent/research-agent-telemetry",
  "level": "INFO",
  "message": "Tool call completed",
  "capability": "current_weather",
  "tool": "weather-service",
  "success": true,
  "timestamp": "2025-12-12T20:25:07Z"
}

{
  "component": "agent/research-agent-telemetry",
  "level": "INFO",
  "message": "Research topic completed",
  "processing_time": "3.04614971s",
  "tools_used": 1,
  "topic": "weather in Tokyo",
  "timestamp": "2025-12-12T20:25:07Z"
}
```

Notice how:
- Framework infrastructure logs show `"component": "framework/core"`
- Application-level logs show `"component": "agent/research-agent-telemetry"`
- Both share the same `service` field for correlation

### How Logging Works in Agents and Tools

When you create an agent or tool and pass a logger to framework modules, each module automatically identifies itself in log output. Here's how it flows:

```go
// Your agent passes its logger to the orchestrator
orchestrator := orchestration.NewAIOrchestrator(aiClient, catalogConfig, logger)

// Inside the orchestrator, the logger is wrapped with the framework component
// Logs will show "component": "framework/orchestration" instead of your agent's name
```

**Example: What your logs look like**

When your `travel-research-agent` calls the orchestration module:

```json
// Agent-level log (your code)
{
  "message": "Starting travel research request",
  "component": "agent/travel-research-agent",
  "topic": "Paris trip"
}

// Framework-level log (orchestration module)
{
  "message": "Auto-wiring parameters for step",
  "component": "framework/orchestration",
  "step": "get_weather",
  "params_resolved": 2
}

// Framework-level log (resilience module)
{
  "message": "Circuit breaker state change",
  "component": "framework/resilience",
  "state": "closed"
}
```

### Using Logging in Your Agents

When building agents, use the `Logger` from `BaseAgent` for all your application logs:

```go
type MyAgent struct {
    *core.BaseAgent
}

func (a *MyAgent) handleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Your agent logs use your agent's component name automatically
    a.Logger.InfoWithContext(ctx, "Processing request", map[string]interface{}{
        "path": r.URL.Path,
    })

    // When you use framework modules (orchestration, resilience, etc.),
    // their logs will show "framework/<module>" as the component
    result, err := a.orchestrator.ExecuteWorkflow(ctx, workflow)

    // Your completion log uses your agent's component name
    a.Logger.InfoWithContext(ctx, "Request completed", map[string]interface{}{
        "status": "success",
    })
}
```

### Using Logging in Your Tools

Tools work the same way as agents:

```go
type WeatherTool struct {
    *core.BaseTool
}

func (t *WeatherTool) handleGetWeather(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Tool logs show "tool/weather-tool" as the component
    t.Logger.InfoWithContext(ctx, "Fetching weather data", map[string]interface{}{
        "lat": lat,
        "lon": lon,
    })
}
```

### Filtering Logs by Component

In production, you can filter logs to focus on specific components. These commands work with the deployed examples in Kubernetes.

#### Quick Component Count

Get a summary of which components are logging:

```bash
# Count logs by component in the last 60 seconds
kubectl logs -n gomind-examples -l app=research-agent-telemetry --since=60s 2>&1 | \
  grep -oP '"component":"[^"]*"' | sort | uniq -c

# Example output:
#   5 "component":"agent/research-agent-telemetry"
#  15 "component":"framework/core"
```

#### JSON Format Filtering (with jq)

**Show only framework logs:**
```bash
kubectl logs -n gomind-examples -l app=my-agent | jq 'select(.component | startswith("framework/"))'
```

**Show only your agent/tool logs (hide framework noise):**
```bash
kubectl logs -n gomind-examples -l app=research-agent-telemetry | \
  jq 'select(.component | startswith("agent/") or startswith("tool/"))'
```

**Show specific framework module logs:**
```bash
# Core module (discovery, registry)
kubectl logs -n gomind-examples -l app=my-agent | jq 'select(.component == "framework/core")'

# Orchestration module
kubectl logs -n gomind-examples -l app=my-agent | jq 'select(.component == "framework/orchestration")'

# Resilience module (retries, circuit breakers)
kubectl logs -n gomind-examples -l app=my-agent | jq 'select(.component == "framework/resilience")'

# AI module
kubectl logs -n gomind-examples -l app=my-agent | jq 'select(.component == "framework/ai")'
```

**Filter by component AND log level:**
```bash
# Only errors from orchestration
kubectl logs -n gomind-examples -l app=my-agent | \
  jq 'select(.component == "framework/orchestration" and .level == "ERROR")'

# Warnings from any framework module
kubectl logs -n gomind-examples -l app=my-agent | \
  jq 'select(.component | startswith("framework/") and .level == "WARN")'
```

**Extract specific fields for analysis:**
```bash
# Show timestamp, component, and message only
kubectl logs -n gomind-examples -l app=research-agent-telemetry | \
  jq '{timestamp, component, message}'
```

#### Using grep for Text-Format Logs

When JSON parsing isn't available, use grep:

```bash
# Framework logs
kubectl logs -n gomind-examples -l app=my-agent | grep '"component":"framework/'

# Agent logs
kubectl logs -n gomind-examples -l app=my-agent | grep '"component":"agent/'

# Tool logs
kubectl logs -n gomind-examples -l app=my-agent | grep '"component":"tool/'

# Specific component
kubectl logs -n gomind-examples -l app=my-agent | grep '"component":"framework/orchestration"'
```

#### Grafana Loki (LogQL)

If using Grafana Loki for log aggregation:

```logql
# Agent handler logs only
{namespace="gomind-examples"} | json | component =~ "agent/.*"

# Framework orchestration with errors
{namespace="gomind-examples"} | json | component="framework/orchestration" | level="ERROR"

# All tool logs with slow responses (>1 second)
{namespace="gomind-examples"} | json | component =~ "tool/.*" | duration_ms > 1000

# Trace a request across all components using trace_id
{namespace="gomind-examples"} | json | trace_id="abc123def456"
```

### Identifying Log Origins

When debugging, the `component` field tells you exactly where the log came from:

| Component | Origin | Example Log Messages |
|-----------|--------|----------------------|
| `agent/<name>` | Your agent's code (handlers, business logic) | "Processing research request", "Research topic completed" |
| `tool/<name>` | Your tool's code (capability handlers) | "Fetching weather data", "Tool call completed" |
| `framework/core` | Core infrastructure (discovery, registry, config) | "Service discovery completed", "Starting service discovery" |
| `framework/orchestration` | Orchestration (auto-wiring, execution, planning) | "Building execution plan", "Workflow execution complete" |
| `framework/resilience` | Resilience patterns (retries, circuit breakers) | "Retry attempt 2/3", "Circuit breaker opened" |
| `framework/ai` | AI module (LLM calls, prompts) | "AI request completed", "Token usage logged" |
| `framework/ui` | UI module (sessions, registry, dashboard) | "Session created", "Registry updated" |

### Sample Log Output Analysis

Here's a complete request flow from a deployed `research-agent-telemetry` showing how components are segregated:

```
20:24:41 [INFO] [framework/core] Starting service discovery
20:24:41 [INFO] [framework/core] Service discovery completed (11 services found)
20:25:06 [INFO] [agent/research-agent-telemetry] AI-powered tool+capability selection (1 call, 50% cost savings)
20:25:06 [INFO] [agent/research-agent-telemetry] Calling AI-selected tool+capability
20:25:07 [INFO] [agent/research-agent-telemetry] AI-generated payload successfully
20:25:07 [INFO] [agent/research-agent-telemetry] Calling tool with intelligent retry enabled
20:25:07 [INFO] [agent/research-agent-telemetry] Tool call completed (success)
20:25:07 [INFO] [agent/research-agent-telemetry] Research topic completed (3.04s)
```

From this log:
- Lines with `[framework/core]` are infrastructure operations (discovery, registry)
- Lines with `[agent/research-agent-telemetry]` are your application's business logic
- The request took 3.04 seconds total, with AI selection and tool execution

### Testing Component Logging

To verify component-aware logging is working in your deployment:

```bash
# 1. Port forward to your agent
kubectl port-forward -n gomind-examples svc/research-agent-telemetry 8092:8092 &

# 2. Make a test request
curl -s -X POST http://localhost:8092/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{"topic":"weather in Tokyo","use_ai":false}'

# 3. Check logs for component field
kubectl logs -n gomind-examples -l app=research-agent-telemetry --since=60s | \
  grep -oP '"component":"[^"]*"' | sort | uniq -c

# Expected output should show both framework and agent components:
#   5 "component":"agent/research-agent-telemetry"
#  15 "component":"framework/core"
```

### The ComponentAwareLogger Interface

The component segregation is powered by the `ComponentAwareLogger` interface defined in [`core/interfaces.go`](../core/interfaces.go):

```go
// ComponentAwareLogger extends Logger with component context support
type ComponentAwareLogger interface {
    Logger
    // WithComponent returns a new logger with the specified component
    WithComponent(component string) Logger
}
```

The framework's `ProductionLogger` implements this interface, so component segregation works automatically when you use GoMind's standard logging setup.

### For Framework Module Developers

If you're developing new framework modules, see [`core/COMPONENT_LOGGING_DESIGN.md`](../core/COMPONENT_LOGGING_DESIGN.md) for the complete implementation guide including:

- The `ComponentAwareLogger` interface design
- Standard `SetLogger` pattern for all modules
- Component naming conventions (`framework/<module>`)
- Implementation examples for each module (core, ai, orchestration, resilience, ui)
- Summary table of all implemented files

**Key Pattern**: Every framework module's `SetLogger` method should wrap the logger with `WithComponent("framework/<module>")`:

```go
func (x *MyComponent) SetLogger(logger core.Logger) {
    if logger == nil {
        x.logger = &core.NoOpLogger{}
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            x.logger = cal.WithComponent("framework/mymodule")
        } else {
            x.logger = logger
        }
    }
}
```

---

## Common Mistakes and How to Avoid Them

### Mistake 1: Using Basic Methods in Handlers

```go
// BAD - loses trace correlation
func (r *Agent) handleRequest(w http.ResponseWriter, req *http.Request) {
    r.Logger.Info("Processing request", nil)  // No context!
}

// GOOD - enables trace correlation
func (r *Agent) handleRequest(w http.ResponseWriter, req *http.Request) {
    ctx := req.Context()
    r.Logger.InfoWithContext(ctx, "Processing request", nil)
}
```

### Mistake 2: Logging Sensitive Data

```go
// BAD - exposes secrets
logger.Info("API call", map[string]interface{}{
    "api_key": apiKey,  // NEVER log secrets!
    "password": pwd,
})

// GOOD - safe logging
logger.Info("API call", map[string]interface{}{
    "provider": "openai",
    "has_key": apiKey != "",  // Boolean is safe
})
```

### Mistake 3: High-Cardinality Fields

```go
// BAD - creates too many unique metric labels
logger.Info("Request", map[string]interface{}{
    "user_id": "user-12345",     // High cardinality
    "request_id": uuid.New(),    // Unique every time
    "timestamp": time.Now(),     // Always different
})

// GOOD - low cardinality fields as labels
logger.Info("Request", map[string]interface{}{
    "operation": "get_weather",  // Fixed set of values
    "status": "success",         // Fixed set of values
    "duration_ms": 125,          // Not a label, just a value
})
```

### Mistake 4: Not Logging Errors Properly

```go
// BAD - loses error context
if err != nil {
    logger.Error("Failed", nil)
    return err
}

// GOOD - includes error details
if err != nil {
    logger.ErrorWithContext(ctx, "Operation failed", map[string]interface{}{
        "operation": "fetch_data",
        "error": err.Error(),
        "error_type": fmt.Sprintf("%T", err),
    })
    return err
}
```

### Mistake 5: Forgetting to Log Success

```go
// BAD - only logs failures
func (r *Agent) handleRequest(w http.ResponseWriter, req *http.Request) {
    ctx := req.Context()

    result, err := doWork()
    if err != nil {
        r.Logger.ErrorWithContext(ctx, "Failed", map[string]interface{}{"error": err.Error()})
        return
    }

    // Where's the success log?
    json.NewEncoder(w).Encode(result)
}

// GOOD - logs both success and failure
func (r *Agent) handleRequest(w http.ResponseWriter, req *http.Request) {
    ctx := req.Context()
    startTime := time.Now()

    r.Logger.InfoWithContext(ctx, "Processing request", nil)

    result, err := doWork()
    if err != nil {
        r.Logger.ErrorWithContext(ctx, "Request failed", map[string]interface{}{
            "error": err.Error(),
            "duration_ms": time.Since(startTime).Milliseconds(),
        })
        return
    }

    r.Logger.InfoWithContext(ctx, "Request completed", map[string]interface{}{
        "duration_ms": time.Since(startTime).Milliseconds(),
        "status": "success",
    })

    json.NewEncoder(w).Encode(result)
}
```

---

## Quick Reference

### Environment Variables

| Variable | Values | Default |
|----------|--------|---------|
| `GOMIND_LOG_LEVEL` | debug, info, warn, error | info |
| `GOMIND_LOG_FORMAT` | json, text | json |
| `GOMIND_DEBUG` | true, false | false |

### Method Selection

| Situation | Method |
|-----------|--------|
| HTTP handler | `InfoWithContext(ctx, ...)` |
| Startup/shutdown | `Info(...)` |
| Background task | `Info(...)` |
| Any error | `ErrorWithContext(ctx, ...)` or `Error(...)` |

### Standard Fields

| Field | Use For |
|-------|---------|
| `operation` | What action is being performed |
| `status` | success, error, retry |
| `error` | Error message |
| `duration_ms` | How long it took |
| `method` | HTTP method |
| `path` | Request path |

### Logging Checklist

- [ ] Using `WithContext` methods in all HTTP handlers
- [ ] Including `duration_ms` for operations
- [ ] Using consistent field names
- [ ] Not logging sensitive data
- [ ] Logging both success and failure paths
- [ ] Using appropriate log levels
- [ ] Initializing telemetry before creating components

---

## Manual Trace ID Extraction

For advanced use cases where you need direct access to trace IDs (e.g., including them in API responses or external logging systems), use `telemetry.GetTraceContext()`:

```go
import "github.com/itsneelabh/gomind/telemetry"

func (a *MyAgent) handleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Extract trace context for manual use
    tc := telemetry.GetTraceContext(ctx)

    // Include in response headers for client correlation
    if tc.TraceID != "" {
        w.Header().Set("X-Trace-ID", tc.TraceID)
    }

    // Or include in structured logs manually
    a.Logger.InfoWithContext(ctx, "Processing", map[string]interface{}{
        "trace_id": tc.TraceID,  // Usually automatic via WithContext
        "span_id":  tc.SpanID,
    })
}
```

> **Note**: The `WithContext` methods automatically include trace correlation. Manual extraction is only needed for special cases like response headers or external system integration.

For complete distributed tracing setup including infrastructure (Jaeger, OTEL Collector), client-side propagation, and trace visualization, see **[DISTRIBUTED_TRACING_GUIDE.md](./DISTRIBUTED_TRACING_GUIDE.md)**.

---

## Summary

1. **Use the framework's Logger**, not Go's standard `log` package (except for fatal startup errors)
2. **Always use `WithContext` methods** in HTTP handlers for trace correlation
3. **Be consistent with field names** across all services
4. **Log both success and failure** with duration metrics
5. **Initialize telemetry first** to enable all three observability layers

Following these guidelines ensures your logs are useful in production, easy to search, and properly correlated across your distributed system.

---

## See Also

- **[DISTRIBUTED_TRACING_GUIDE.md](./DISTRIBUTED_TRACING_GUIDE.md)** - Complete guide for distributed tracing setup, including TracingMiddleware, TracedHTTPClient, Jaeger/OTEL infrastructure, and trace visualization
- **[core/COMPONENT_LOGGING_DESIGN.md](../core/COMPONENT_LOGGING_DESIGN.md)** - Technical design document for component-aware logging architecture, including implementation details for all framework modules
- **[telemetry/trace_context.go](../telemetry/trace_context.go)** - Source for `GetTraceContext()`, `AddSpanEvent()`, `RecordSpanError()`
- **[core/config.go](../core/config.go)** - ProductionLogger implementation and `WithComponent()` method
- **[core/interfaces.go](../core/interfaces.go)** - Logger interface and `ComponentAwareLogger` interface definitions
