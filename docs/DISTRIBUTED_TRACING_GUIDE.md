# Distributed Tracing and Log Correlation Guide

Welcome to the complete guide on distributed tracing in GoMind! Think of this as your friendly mentor sitting next to you, explaining how to follow a request as it travels through your entire system. Grab a coffee, and let's dive in!

## Table of Contents

- [What Is Distributed Tracing and Why Should You Care?](#-what-is-distributed-tracing-and-why-should-you-care)
- [The Problem Without Tracing](#-the-problem-without-tracing)
- [The Solution: Context Propagation](#-the-solution-context-propagation)
- [Understanding Trace IDs, Span IDs, and Parent Spans](#-understanding-trace-ids-span-ids-and-parent-spans)
- [Trace-Log Correlation: The Magic Glue](#-trace-log-correlation-the-magic-glue)
- [Implementation: Server-Side (TracingMiddleware)](#-implementation-server-side-tracingmiddleware)
- [Implementation: Client-Side (TracedHTTPClient)](#-implementation-client-side-tracedhttpclient)
- [Complete Example: Multi-Service Tracing](#-complete-example-multi-service-tracing)
- [Infrastructure Setup (Kubernetes)](#-infrastructure-setup-kubernetes)
- [Viewing Your Traces](#-viewing-your-traces)
- [Best Practices](#-best-practices)
- [Troubleshooting](#-troubleshooting)
- [Quick Reference](#-quick-reference)

---

## What Is Distributed Tracing and Why Should You Care?

Let me explain this with a story everyone can relate to.

### The Package Delivery Analogy

Imagine you order a gift online that needs to be assembled from parts made by different factories:

1. **Factory A** makes the electronics
2. **Factory B** makes the casing
3. **Factory C** does the assembly
4. **Shipping Center** packs and ships it

Now imagine your package never arrives. You call customer service, and they say:
- "Factory A says they sent their part on time"
- "Factory B has no record of your order"
- "Factory C says they never got anything"
- "Shipping says they have 10,000 packages and can't find yours"

**Nightmare, right?**

Now imagine if every package had a **tracking number** that followed it through every step:
- Factory A: "Package #12345 - electronics completed at 10:00 AM"
- Factory B: "Package #12345 - casing completed at 11:00 AM"
- Factory C: "Package #12345 - waiting for casing (still at Factory B!)"

**That tracking number is exactly what a Trace ID does for your requests!**

### Why This Matters for Your Applications

In a microservices architecture (like GoMind's tools and agents), a single user request might touch:
- 1 Agent (orchestrator)
- 5 Tools (weather, currency, geocoding, etc.)
- 2 Databases
- 1 External API

Without distributed tracing, when something goes wrong, you have:
- **6+ separate log files** with no way to connect them
- **No visibility** into which service caused the delay
- **Debugging nightmares** at 3 AM during an outage

With distributed tracing, you get:
- **One trace ID** connecting all logs across all services
- **Visual timeline** showing exactly where time was spent
- **Instant root cause analysis** - "Oh, the currency service took 5 seconds!"

---

## The Problem Without Tracing

Let me show you what debugging looks like without distributed tracing.

### Scenario: User Request Is Slow

A user complains: "Getting weather and stock data takes forever!"

**Without tracing, your logs look like this:**

```
# research-agent logs
2024-01-01 10:00:00 INFO  Processing research request
2024-01-01 10:00:05 INFO  Research completed

# weather-tool logs
2024-01-01 10:00:01 INFO  Weather request received
2024-01-01 10:00:01 INFO  Weather response sent

# stock-tool logs
2024-01-01 10:00:02 INFO  Stock request received
2024-01-01 10:00:04 INFO  Stock response sent
```

**Questions you can't answer:**
- Which request in the agent logs corresponds to which tool calls?
- Was the 5-second delay in the agent or the tools?
- If there were 100 concurrent requests, which logs go together?

### The Correlation Challenge

The fundamental problem is: **logs from different services have no common identifier**.

```
Service A: "Started processing"      ← Which request?
Service B: "Database query slow"     ← Same request? Different request?
Service C: "Returned response"       ← No idea!
```

---

## The Solution: Context Propagation

The solution is elegantly simple: **pass a unique identifier with every request**.

### How It Works

```
┌─────────────────────────────────────────────────────────────────────┐
│                         USER REQUEST                                 │
│                    (no trace ID yet)                                │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      RESEARCH AGENT                                  │
│                                                                      │
│  TracingMiddleware extracts OR generates:                           │
│  trace_id: abc123                                                   │
│  span_id: span-001                                                  │
│                                                                      │
│  Every log now includes: {"trace.trace_id": "abc123", ...}          │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
          ┌─────────────────┼─────────────────┐
          │                 │                 │
          ▼                 ▼                 ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│  WEATHER TOOL   │ │  STOCK TOOL     │ │  CURRENCY TOOL  │
│                 │ │                 │ │                 │
│ HTTP Headers:   │ │ HTTP Headers:   │ │ HTTP Headers:   │
│ traceparent:    │ │ traceparent:    │ │ traceparent:    │
│ 00-abc123-...   │ │ 00-abc123-...   │ │ 00-abc123-...   │
│                 │ │                 │ │                 │
│ Logs include:   │ │ Logs include:   │ │ Logs include:   │
│ trace_id:abc123 │ │ trace_id:abc123 │ │ trace_id:abc123 │
└─────────────────┘ └─────────────────┘ └─────────────────┘
```

### The W3C TraceContext Standard

GoMind uses the **W3C TraceContext** standard, which is supported by all major tracing systems (Jaeger, Zipkin, Datadog, etc.).

The magic happens through HTTP headers:

```http
# Outgoing request from agent to tool
POST /api/capabilities/get_weather HTTP/1.1
Host: weather-tool:8080
traceparent: 00-abc123def456789-span001-01
tracestate: gomind=research-agent
```

**The `traceparent` header contains:**
- `00` - Version (always 00)
- `abc123def456789` - **Trace ID** (same for ALL services in this request)
- `span001` - **Span ID** (unique to this specific operation)
- `01` - Flags (01 = sampled)

---

## Understanding Trace IDs, Span IDs, and Parent Spans

Before we dive into implementation, let's understand the core concepts.

### The Family Tree Analogy

Think of a trace like a family tree:
- **Trace ID** = The family name (shared by everyone in the family)
- **Span ID** = Each person's unique ID
- **Parent Span ID** = Who is your parent

```
Trace ID: "Smith Family" (abc123)
│
├── Grandparent (span: A, parent: none)
│   ├── Parent (span: B, parent: A)
│   │   ├── Child 1 (span: C, parent: B)
│   │   └── Child 2 (span: D, parent: B)
│   └── Uncle (span: E, parent: A)
│       └── Cousin (span: F, parent: E)
```

### In Practice: A Research Request

```
Trace ID: fee30b72efcbefd21fddf9cd56d2c8c9
│
├── research-agent: HTTP POST /api/research (span: 1134)
│   ├── research-agent: call_weather_tool (span: 2245, parent: 1134)
│   │   └── weather-tool: HTTP POST /api/get_weather (span: 3356, parent: 2245)
│   │       └── weather-tool: fetch_api_data (span: 4467, parent: 3356)
│   │
│   ├── research-agent: call_stock_tool (span: 5578, parent: 1134)
│   │   └── stock-tool: HTTP POST /api/stock_quote (span: 6689, parent: 5578)
│   │
│   └── research-agent: aggregate_results (span: 7790, parent: 1134)
```

### What This Gives You

In Jaeger or Grafana, you see a beautiful timeline:

```
research-agent: HTTP POST /api/research ─────────────────────────────▶ 350ms
├─ call_weather_tool ─────────────────▶ 150ms
│  └─ weather-tool: HTTP POST ────────▶ 145ms
│     └─ fetch_api_data ─────▶ 100ms
│
├─ call_stock_tool ──────────────────▶ 180ms
│  └─ stock-tool: HTTP POST ──────────▶ 175ms
│
└─ aggregate_results ▶ 10ms
```

**Now you can instantly see:** The stock tool is the bottleneck (180ms vs 150ms for weather).

---

## Trace-Log Correlation: The Magic Glue

Distributed tracing shows you the *timeline*. But logs show you the *details*. **Trace-log correlation connects them.**

### The Problem: Searching Through Logs

Even with Jaeger showing you a slow span, you need to find the actual logs:

```bash
# Without correlation - good luck finding the right log!
grep "error" /var/log/stock-tool.log | head -100
# Returns 100 lines... which one is YOUR request?
```

### The Solution: Trace IDs in Every Log

With trace-log correlation, every log entry includes the trace ID:

```json
{
  "timestamp": "2024-01-01T10:00:02Z",
  "level": "info",
  "message": "Processing stock quote request",
  "trace.trace_id": "fee30b72efcbefd21fddf9cd56d2c8c9",
  "trace.span_id": "6689abcd1234",
  "symbol": "AAPL"
}
```

**Now you can search:**

```bash
# Find ALL logs for this specific request across ALL services
grep "fee30b72efcbefd21fddf9cd56d2c8c9" /var/log/*.log
```

### How GoMind Implements This

When using the `TracingMiddleware`, you can extract trace information from the context and include it in your logs:

```go
// In your handler, extract trace context from the request
func (t *MyTool) handleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Extract trace context for logging
    tc := telemetry.GetTraceContext(ctx)

    // Include in your logs
    log.Printf("Processing request trace_id=%s span_id=%s symbol=%s",
        tc.TraceID, tc.SpanID, req.Symbol)
}
```

For structured JSON logging, you can create a helper:

```go
// Helper to add trace context to log fields
func logWithTrace(ctx context.Context, msg string, fields map[string]interface{}) {
    tc := telemetry.GetTraceContext(ctx)
    fields["trace.trace_id"] = tc.TraceID
    fields["trace.span_id"] = tc.SpanID
    // Use your preferred JSON logger (zerolog, zap, etc.)
    jsonLog(msg, fields)
}
```

**The trace context is automatically propagated** via HTTP headers - you just need to include it in your logs for correlation!

---

## Implementation: Server-Side (TracingMiddleware)

Now let's get practical. Here's how to add distributed tracing to your GoMind tools and agents.

### What TracingMiddleware Does

The `TracingMiddleware` wraps your HTTP handlers and automatically:
1. **Extracts** trace context from incoming `traceparent` headers
2. **Creates** a new span for this request
3. **Records** HTTP metrics (status codes, latency)
4. **Propagates** context to your handler code via `r.Context()`

### Basic Usage (Recommended)

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

func main() {
    // 1. Initialize telemetry FIRST
    initTelemetry("weather-service")
    defer telemetry.Shutdown(context.Background())

    // 2. Create your tool
    tool := NewWeatherTool()

    // 3. Create framework WITH tracing middleware
    framework, err := core.NewFramework(tool,
        core.WithName("weather-service"),
        core.WithPort(8080),
        core.WithRedisURL(os.Getenv("REDIS_URL")),
        core.WithDiscovery(true, "redis"),

        // THIS IS THE KEY LINE - adds tracing middleware
        core.WithMiddleware(telemetry.TracingMiddleware("weather-service")),
    )
    if err != nil {
        log.Fatalf("Failed to create framework: %v", err)
    }

    // 4. Run
    ctx := context.Background()
    framework.Run(ctx)
}

func initTelemetry(serviceName string) {
    config := telemetry.UseProfile(telemetry.ProfileDevelopment)
    config.ServiceName = serviceName

    // Point to your OTEL Collector
    if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
        config.Endpoint = endpoint
    }

    if err := telemetry.Initialize(config); err != nil {
        log.Printf("Warning: Telemetry init failed: %v", err)
    }
}
```

### What Happens Under the Hood

When a request arrives:

```
Incoming Request
    │
    ▼
┌─────────────────────────────────────────────────────────────────┐
│ TracingMiddleware                                                │
│                                                                  │
│ 1. Check for traceparent header                                 │
│    - If present: Extract trace_id and parent_span_id            │
│    - If absent: Generate new trace_id                           │
│                                                                  │
│ 2. Create a new span for this request                           │
│    - Name: "HTTP POST /api/capabilities/get_weather"            │
│    - Parent: The extracted parent_span_id (if any)              │
│                                                                  │
│ 3. Add span to context                                          │
│    ctx = context.WithValue(ctx, spanKey, span)                  │
│                                                                  │
│ 4. Call your handler with enriched context                      │
│    next.ServeHTTP(w, r.WithContext(ctx))                        │
│                                                                  │
│ 5. When handler returns, end the span and record metrics        │
└─────────────────────────────────────────────────────────────────┘
    │
    ▼
Your Handler (receives ctx with trace info)
```

### Excluding Health Checks (Best Practice)

Health check endpoints are called frequently by Kubernetes. You don't want to trace them:

```go
// Use TracingMiddlewareWithConfig for more control
config := &telemetry.TracingMiddlewareConfig{
    ExcludedPaths: []string{"/health", "/metrics", "/ready", "/live"},
}

framework, _ := core.NewFramework(tool,
    core.WithMiddleware(
        telemetry.TracingMiddlewareWithConfig("weather-service", config),
    ),
)
```

### Custom Span Names

By default, span names are `HTTP GET /api/capabilities/get_weather`. You can customize:

```go
config := &telemetry.TracingMiddlewareConfig{
    SpanNameFormatter: func(operation string, r *http.Request) string {
        // Create more semantic names
        return fmt.Sprintf("%s %s", r.Method, getRoutePattern(r))
    },
}
```

---

## Implementation: Client-Side (TracedHTTPClient)

Server-side tracing is only half the story. When your **agent calls a tool**, you need to **propagate the trace context** in the outgoing request.

### What TracedHTTPClient Does

The `NewTracedHTTPClient()` creates an HTTP client that automatically:
1. **Injects** `traceparent` header into all outgoing requests
2. **Creates** client-side spans for each HTTP call
3. **Records** request/response metrics
4. **Propagates** the trace context to downstream services

### Basic Usage

This example is from `examples/agent-with-telemetry/research_agent.go`:

```go
package main

import (
    "context"
    "net/http"
    "time"

    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

type ResearchAgent struct {
    *core.BaseAgent
    httpClient *http.Client  // Traced HTTP client
}

func NewResearchAgent() (*ResearchAgent, error) {
    agent := core.NewBaseAgent("research-assistant-telemetry")

    // Create traced HTTP client with custom transport for production use
    // This is the ACTUAL pattern from agent-with-telemetry example
    tracedClient := telemetry.NewTracedHTTPClientWithTransport(&http.Transport{
        MaxIdleConns:        100,              // Connection pool size
        MaxIdleConnsPerHost: 10,               // Per-host connection limit
        IdleConnTimeout:     90 * time.Second, // Keep-alive timeout
        DisableKeepAlives:   false,            // Enable connection reuse
        ForceAttemptHTTP2:   true,             // Use HTTP/2 when available
    })
    tracedClient.Timeout = 30 * time.Second

    return &ResearchAgent{
        BaseAgent:  agent,
        httpClient: tracedClient,
    }, nil
}

func (a *ResearchAgent) callWeatherTool(ctx context.Context, city string) (*Weather, error) {
    // Create request WITH CONTEXT - this is crucial!
    req, err := http.NewRequestWithContext(ctx, "POST",
        "http://weather-tool:8080/api/capabilities/get_weather",
        strings.NewReader(`{"location": "`+city+`"}`))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")

    // The traced client automatically adds traceparent header!
    resp, err := a.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    // Parse response...
    var weather Weather
    json.NewDecoder(resp.Body).Decode(&weather)
    return &weather, nil
}
```

### What Happens Under the Hood

```
Agent's httpClient.Do(req)
    │
    ▼
┌─────────────────────────────────────────────────────────────────┐
│ otelhttp.Transport (inside TracedHTTPClient)                    │
│                                                                  │
│ 1. Extract trace context from ctx                               │
│    trace_id: abc123, span_id: span-001                          │
│                                                                  │
│ 2. Create child span for this HTTP call                         │
│    Name: "HTTP POST weather-tool:8080"                          │
│    Parent: span-001                                              │
│    New span_id: span-002                                         │
│                                                                  │
│ 3. Inject traceparent header into request                       │
│    traceparent: 00-abc123-span002-01                            │
│                                                                  │
│ 4. Make the actual HTTP request                                 │
│                                                                  │
│ 5. When response returns, end span with status                  │
└─────────────────────────────────────────────────────────────────┘
    │
    ▼
Weather Tool receives request with traceparent header
```

### Important: Always Pass Context!

The trace propagation only works if you pass the context:

```go
// CORRECT - trace context propagates
req, _ := http.NewRequestWithContext(ctx, "POST", url, body)
resp, _ := client.Do(req)

// WRONG - trace context is lost!
req, _ := http.NewRequest("POST", url, body)  // No context!
resp, _ := client.Do(req)  // traceparent header won't be added
```

### Simple vs Production Client

For quick development, you can use the simpler form:

```go
// Simple form - uses default transport settings
client := telemetry.NewTracedHTTPClient(nil)
```

For production, use `NewTracedHTTPClientWithTransport` with custom settings (as shown in the agent-with-telemetry example above) to control connection pooling and timeouts.

---

## Complete Example: Multi-Service Tracing

The best way to understand distributed tracing is to look at the **actual working examples** in the GoMind repository.

> **Working Examples:**
> - Agent: `examples/agent-with-telemetry/` - Full agent with tracing
> - Tool: `examples/tool-example/` - Weather tool with tracing

### The Architecture

```
User Request
    │
    ▼
┌─────────────────────────────────────────────────────────────────┐
│ Research Agent (Port 8092)                                       │
│ See: examples/agent-with-telemetry/                             │
│                                                                  │
│ - TracingMiddleware (extracts/creates trace)                    │
│ - TracedHTTPClient (propagates trace to tools)                  │
└───────────────────────────────┬─────────────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        │                       │                       │
        ▼                       ▼                       ▼
┌───────────────┐       ┌───────────────┐       ┌───────────────┐
│ Weather Tool  │       │ Stock Tool    │       │ Currency Tool │
│ (Port 8080)   │       │ (Port 8082)   │       │ (Port 8094)   │
│               │       │               │       │               │
│ TracingMW     │       │ TracingMW     │       │ TracingMW     │
│               │       │               │       │               │
│ tool-example/ │       │ stock-market- │       │ currency-tool/│
│               │       │ tool/         │       │               │
└───────────────┘       └───────────────┘       └───────────────┘
```

### Agent Code (from examples/agent-with-telemetry/research_agent.go)

This is a **simplified version** of the actual code. See the full implementation for additional features like metric declarations, AI integration, and more.

```go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "log"
    "net/http"
    "time"

    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

type ResearchAgent struct {
    *core.BaseAgent
    httpClient *http.Client
}

func NewResearchAgent() (*ResearchAgent, error) {
    agent := core.NewBaseAgent("research-assistant-telemetry")

    // Create traced HTTP client with production settings
    // This is the ACTUAL pattern from the example
    tracedClient := telemetry.NewTracedHTTPClientWithTransport(&http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        DisableKeepAlives:   false,
        ForceAttemptHTTP2:   true,
    })
    tracedClient.Timeout = 30 * time.Second

    researchAgent := &ResearchAgent{
        BaseAgent:  agent,
        httpClient: tracedClient,
    }

    // Register capabilities
    researchAgent.registerCapabilities()
    return researchAgent, nil
}

func (r *ResearchAgent) registerCapabilities() {
    r.RegisterCapability(core.Capability{
        Name:        "research_topic",
        Description: "Researches a topic using multiple tools",
        Handler:     r.handleResearchTopic,
    })
}

func (r *ResearchAgent) handleResearchTopic(w http.ResponseWriter, req *http.Request) {
    ctx := req.Context()  // Contains trace context from TracingMiddleware

    var request struct {
        Topic string `json:"topic"`
    }
    json.NewDecoder(req.Body).Decode(&request)

    log.Printf("Starting research for topic: %s", request.Topic)

    // Call tools - trace context propagates via TracedHTTPClient
    weather, _ := r.callTool(ctx, "http://weather-tool:8080/api/capabilities/get_weather",
        map[string]string{"location": "London"})

    stock, _ := r.callTool(ctx, "http://stock-tool:8082/api/capabilities/stock_quote",
        map[string]string{"symbol": "AAPL"})

    log.Printf("Research completed, called 2 tools")

    // Return combined results
    json.NewEncoder(w).Encode(map[string]interface{}{
        "weather": weather,
        "stock":   stock,
    })
}

func (r *ResearchAgent) callTool(ctx context.Context, url string, params interface{}) (interface{}, error) {
    body, _ := json.Marshal(params)

    // CRITICAL: Use NewRequestWithContext to propagate trace!
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")

    // TracedHTTPClient adds traceparent header automatically
    resp, err := r.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    return result, nil
}
```

### Agent Main (from examples/agent-with-telemetry/main.go)

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

func main() {
    // Initialize telemetry BEFORE creating agent
    initTelemetry("research-assistant-telemetry")
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        telemetry.Shutdown(ctx)
    }()

    // Create agent
    agent, err := NewResearchAgent()
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }

    // Create framework with tracing middleware
    framework, _ := core.NewFramework(agent,
        core.WithName("research-assistant-telemetry"),
        core.WithPort(8092),
        core.WithRedisURL(os.Getenv("REDIS_URL")),
        core.WithDiscovery(true, "redis"),

        // Add tracing middleware for incoming requests
        core.WithMiddleware(telemetry.TracingMiddleware("research-assistant-telemetry")),
    )

    ctx := context.Background()
    log.Println("Research Agent starting on port 8092...")
    framework.Run(ctx)
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
        log.Printf("Warning: Telemetry init failed: %v", err)
    }
}
```

### Tool Code (from examples/tool-example/main.go)

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

func main() {
    // Initialize telemetry
    initTelemetry("weather-service")
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        telemetry.Shutdown(ctx)
    }()

    // Create tool
    tool := NewWeatherTool()

    // Create framework with tracing middleware
    framework, _ := core.NewFramework(tool,
        core.WithName("weather-service"),
        core.WithPort(8080),
        core.WithRedisURL(os.Getenv("REDIS_URL")),
        core.WithDiscovery(true, "redis"),

        // Add tracing middleware - extracts trace from incoming requests
        core.WithMiddleware(telemetry.TracingMiddleware("weather-service")),
    )

    ctx := context.Background()
    log.Println("Weather Tool starting on port 8080...")
    framework.Run(ctx)
}

// initTelemetry follows the same pattern as the agent
func initTelemetry(serviceName string) {
    // ... same as agent example above
}
```

### The Result: Connected Traces

When you make a request to the agent:

```bash
curl -X POST http://localhost:8092/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{"topic": "weather and stocks"}'
```

**In Jaeger, you'll see ONE trace spanning all services:**

```
research-agent: HTTP POST /api/capabilities/research_topic ─────────────────▶ 450ms
├── research-agent: callTool(weather) ─────────────────────▶ 200ms
│   └── weather-service: HTTP POST /api/capabilities/get_weather ──▶ 195ms
│
└── research-agent: callTool(stock) ────────────────────────▶ 220ms
    └── stock-service: HTTP POST /api/capabilities/stock_quote ────▶ 215ms
```

**In your logs, every entry has the same trace ID:**

```json
// research-agent log
{"level":"info","message":"Starting research","trace.trace_id":"abc123","trace.span_id":"1111"}

// weather-service log
{"level":"info","message":"Fetching weather","trace.trace_id":"abc123","trace.span_id":"2222"}

// stock-service log
{"level":"info","message":"Getting stock quote","trace.trace_id":"abc123","trace.span_id":"3333"}
```

---

## Infrastructure Setup (Kubernetes)

For distributed tracing to work, you need a place to **collect** and **visualize** traces. Here's the recommended setup.

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Your Services                                 │
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │   Agent      │  │ Weather Tool │  │ Stock Tool   │              │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘              │
│         │                 │                 │                        │
│         │ OTLP/HTTP       │ OTLP/HTTP       │ OTLP/HTTP             │
│         │ :4318           │ :4318           │ :4318                 │
│         └────────────────┬┴─────────────────┘                        │
│                          │                                           │
│                          ▼                                           │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │              OTEL Collector (otel-collector:4318)              │  │
│  │                                                                │  │
│  │  Receives traces → Batches them → Exports to backends         │  │
│  └─────────────────────────────┬─────────────────────────────────┘  │
│                                │                                     │
│              ┌─────────────────┴─────────────────┐                  │
│              │                                   │                  │
│              ▼                                   ▼                  │
│  ┌───────────────────┐               ┌───────────────────┐         │
│  │      Jaeger       │               │    Prometheus     │         │
│  │  (Trace Storage)  │               │  (Metric Storage) │         │
│  │   Port 16686 UI   │               │   Port 9090 UI    │         │
│  └───────────────────┘               └───────────────────┘         │
│              │                                   │                  │
│              └─────────────────┬─────────────────┘                  │
│                                │                                     │
│                                ▼                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                    Grafana (:3000)                             │  │
│  │                                                                │  │
│  │   - Trace visualization (via Jaeger datasource)               │  │
│  │   - Metrics dashboards (via Prometheus datasource)            │  │
│  │   - Correlated views (trace + metrics together)               │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

### Environment Variables for Services

Every service that sends traces needs to know where the collector is:

```yaml
# In your Kubernetes deployment
env:
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "http://otel-collector:4318"
  - name: APP_ENV
    value: "production"  # or "development" for 100% sampling
```

### OTEL Collector Configuration

The collector receives traces from your services and forwards them to Jaeger:

```yaml
# otel-collector.yaml (ConfigMap)
receivers:
  otlp:
    protocols:
      http:
        endpoint: "0.0.0.0:4318"
      grpc:
        endpoint: "0.0.0.0:4317"

processors:
  batch:
    timeout: 1s
    send_batch_size: 1024

exporters:
  otlp/jaeger:
    endpoint: "jaeger-collector:4317"
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/jaeger]
```

### Quick Start: Deploy Infrastructure

If you're using the GoMind examples, the infrastructure is already defined:

```bash
# Apply the infrastructure
kubectl apply -f examples/k8-deployment/otel-collector.yaml
kubectl apply -f examples/k8-deployment/jaeger.yaml
kubectl apply -f examples/k8-deployment/prometheus.yaml
kubectl apply -f examples/k8-deployment/grafana.yaml

# Verify everything is running
kubectl get pods -n gomind-examples
```

---

## Viewing Your Traces

Once your infrastructure is set up, here's how to view and analyze traces.

### Accessing Jaeger UI

```bash
# Port-forward to access Jaeger locally
kubectl port-forward -n gomind-examples svc/jaeger-query 16686:80

# Open in browser
open http://localhost:16686
```

### Finding a Trace

1. **Select your service** from the "Service" dropdown
2. **Click "Find Traces"**
3. **Click on a trace** to see the full timeline

### What to Look For

**Healthy Trace:**
```
research-agent: POST /research ─────────────────────────▶ 150ms
├── weather-tool: POST /weather ─────────▶ 50ms
└── stock-tool: POST /stock ─────────▶ 45ms
```

**Problem: Sequential calls that should be parallel:**
```
research-agent: POST /research ─────────────────────────────────────▶ 300ms
├── weather-tool: POST /weather ─────────▶ 50ms
│                                        (100ms gap - why?)
└── stock-tool: POST /stock                         ─────────▶ 45ms
```

**Problem: One slow service:**
```
research-agent: POST /research ─────────────────────────────────────▶ 5200ms
├── weather-tool: POST /weather ─▶ 50ms
└── stock-tool: POST /stock ──────────────────────────────────────▶ 5100ms
    └── database query ────────────────────────────────────────────▶ 5050ms
```

### Searching by Trace ID

If you have a trace ID from your logs:

```bash
# In Jaeger, paste the trace ID directly in the search box
# Or construct the URL:
http://localhost:16686/trace/fee30b72efcbefd21fddf9cd56d2c8c9
```

### Correlating with Logs

1. Find the problematic span in Jaeger
2. Note the `trace_id`
3. Search your logs:
   ```bash
   # Kubernetes logs
   kubectl logs -n gomind-examples deployment/stock-tool | grep "fee30b72efcbefd21fddf9cd56d2c8c9"

   # Or in Grafana Loki
   {app="stock-tool"} |= "fee30b72efcbefd21fddf9cd56d2c8c9"
   ```

---

## Best Practices

### DO

1. **Always pass context through your code:**
   ```go
   // GOOD
   result, err := processData(ctx, input)

   // BAD
   result, err := processData(input)  // Lost trace context!
   ```

2. **Include trace IDs in logs for correlation:**
   ```go
   // Extract trace context and include in logs
   tc := telemetry.GetTraceContext(ctx)
   log.Printf("Processing request trace_id=%s span_id=%s", tc.TraceID, tc.SpanID)
   ```

3. **Initialize telemetry early:**
   ```go
   func main() {
       initTelemetry("my-service")
       defer telemetry.Shutdown(context.Background())
       // ... rest of main
   }
   ```

4. **Exclude noisy endpoints:**
   ```go
   config := &telemetry.TracingMiddlewareConfig{
       ExcludedPaths: []string{"/health", "/metrics", "/ready"},
   }
   ```

5. **Reuse HTTP clients:**
   ```go
   // GOOD - create once
   client := telemetry.NewTracedHTTPClient(nil)

   // BAD - creates connection pool per request
   for _, url := range urls {
       client := telemetry.NewTracedHTTPClient(nil)  // Don't do this!
       client.Get(url)
   }
   ```

### DON'T

1. **Don't forget `context.Background()` for background tasks:**
   ```go
   // If you're starting a background goroutine, use a fresh context
   // Note: For custom spans, use OpenTelemetry's tracer directly:
   //   tracer := otel.Tracer("my-service")
   //   ctx, span := tracer.Start(context.Background(), "background-task")
   go func() {
       ctx := context.Background()
       // ... work with ctx
   }()
   ```

2. **Don't trace every internal operation:**
   ```go
   // For custom spans, use OpenTelemetry's tracer:
   tracer := otel.Tracer("my-service")

   // BAD - too noisy
   for i := 0; i < 1000; i++ {
       _, span := tracer.Start(ctx, "loop-iteration")
       doTinyThing()
       span.End()
   }

   // GOOD - trace meaningful operations
   ctx, span := tracer.Start(ctx, "process-batch")
   for i := 0; i < 1000; i++ {
       doTinyThing()
   }
   span.End()
   ```

3. **Don't forget to call `Shutdown()`:**
   ```go
   // BAD - traces may be lost
   func main() {
       telemetry.Initialize(config)
       runApp()
       // Exit without flushing!
   }

   // GOOD
   func main() {
       telemetry.Initialize(config)
       defer telemetry.Shutdown(context.Background())  // Flushes pending traces
       runApp()
   }
   ```

---

## Troubleshooting

### Problem: Traces Not Appearing in Jaeger

**Symptoms:** Services are running, but no traces in Jaeger UI.

**Check 1: OTEL Collector connectivity**
```bash
# Check if collector is running
kubectl get pods -n gomind-examples | grep otel-collector

# Check collector logs
kubectl logs -n gomind-examples deployment/otel-collector
```

**Check 2: Service environment variables**
```bash
# Verify OTEL endpoint is set
kubectl exec -n gomind-examples deployment/weather-tool -- env | grep OTEL
# Should show: OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
```

**Check 3: Telemetry initialization**
```bash
# Check service logs for telemetry initialization
kubectl logs -n gomind-examples deployment/weather-tool | grep -i telemetry
# Should show: "Telemetry initialized for weather-service"
```

### Problem: Traces Are Disconnected

**Symptoms:** Traces appear, but agent and tools have separate traces (not connected).

**Cause:** Context not propagating between services.

**Fix 1: Use `NewRequestWithContext`**
```go
// WRONG
req, _ := http.NewRequest("POST", url, body)

// RIGHT
req, _ := http.NewRequestWithContext(ctx, "POST", url, body)
```

**Fix 2: Use `TracedHTTPClient`**
```go
// WRONG - regular http.Client doesn't inject headers
client := &http.Client{}

// RIGHT - traced client injects traceparent header
client := telemetry.NewTracedHTTPClient(nil)
```

### Problem: Logs Don't Have Trace IDs

**Symptoms:** Traces work, but logs don't show `trace.trace_id`.

**Cause:** Not extracting trace context from the request context.

**Fix:**
```go
// WRONG - no trace context in log
log.Printf("Processing request")

// RIGHT - extract and include trace ID
tc := telemetry.GetTraceContext(ctx)
log.Printf("Processing request trace_id=%s span_id=%s", tc.TraceID, tc.SpanID)
```

### Problem: Too Many Traces (Noisy)

**Symptoms:** Millions of traces, hard to find important ones.

**Fix 1: Reduce sampling rate for production**
```go
// In telemetry initialization
config := telemetry.UseProfile(telemetry.ProfileProduction)  // 0.1% sampling
```

**Fix 2: Exclude health endpoints**
```go
config := &telemetry.TracingMiddlewareConfig{
    ExcludedPaths: []string{"/health", "/metrics", "/ready", "/live"},
}
```

### Problem: High Latency from Tracing

**Symptoms:** Service is slower after adding tracing.

**Check 1: Ensure collector is batching**
```yaml
# In otel-collector config
processors:
  batch:
    timeout: 1s
    send_batch_size: 1024  # Don't send one trace at a time!
```

**Check 2: Use async export (default)**
The telemetry module uses asynchronous export by default. If you've customized it, ensure you're not using synchronous export.

---

## Quick Reference

### Adding Tracing to a New Service

```go
// 1. Initialize telemetry
telemetry.Initialize(telemetry.Config{
    ServiceName: "my-service",
    Endpoint:    os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
})
defer telemetry.Shutdown(context.Background())

// 2. Add TracingMiddleware to framework
framework, _ := core.NewFramework(component,
    core.WithMiddleware(telemetry.TracingMiddleware("my-service")),
)

// 3. Use TracedHTTPClient for outgoing calls
client := telemetry.NewTracedHTTPClient(nil)

// 4. Always pass context
req, _ := http.NewRequestWithContext(ctx, "POST", url, body)
resp, _ := client.Do(req)

// 5. Include trace ID in logs for correlation
tc := telemetry.GetTraceContext(ctx)
log.Printf("Message trace_id=%s span_id=%s", tc.TraceID, tc.SpanID)
```

### Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTEL Collector endpoint | `http://otel-collector:4318` |
| `APP_ENV` | Environment (affects sampling) | `production`, `development` |

### Key Types and Functions

| Type/Function | Purpose |
|---------------|---------|
| `telemetry.TracingMiddleware()` | Extracts trace from incoming requests |
| `telemetry.NewTracedHTTPClient()` | Injects trace into outgoing requests (simple form) |
| `telemetry.NewTracedHTTPClientWithTransport()` | Injects trace with custom transport (production) |
| `telemetry.GetTraceContext(ctx)` | Returns `TraceContext` struct with `.TraceID` and `.SpanID` |
| `telemetry.HasTraceContext(ctx)` | Returns true if context has valid trace info |
| `telemetry.AddSpanEvent(ctx, name, attrs...)` | Add named events to the current span |
| `telemetry.RecordSpanError(ctx, err)` | Record an error on the current span |
| `telemetry.SetSpanAttributes(ctx, attrs...)` | Add attributes to the current span |
| `telemetry.TracingMiddlewareConfig` | Configure path exclusions, span names |

### Telemetry Profiles

| Profile | Sampling | Use Case |
|---------|----------|----------|
| `ProfileDevelopment` | 100% | Local development, see everything |
| `ProfileStaging` | 10% | Testing environments |
| `ProfileProduction` | 0.1% | High-traffic production |

---

## Summary

Distributed tracing transforms debugging from guesswork into science. Here's what you've learned:

1. **The Problem:** Without tracing, logs from different services have no common identifier
2. **The Solution:** Trace IDs propagate through HTTP headers (W3C TraceContext)
3. **Server-Side:** `TracingMiddleware` extracts/creates traces for incoming requests
4. **Client-Side:** `TracedHTTPClient` propagates traces to downstream services
5. **Log Correlation:** Extract trace IDs from context to include in your logs
6. **Infrastructure:** OTEL Collector + Jaeger + Grafana for collection and visualization

**Remember:** Tracing is like having GPS for your requests. You always know where they are, where they've been, and why they're stuck in traffic!

---

## Related Documentation

- [Telemetry Module README](../telemetry/README.md) - Metrics and configuration
- [Core Module README](../core/README.md) - Framework fundamentals
- [API Reference - Tracing Section](./API_REFERENCE.md#distributed-tracing) - API details
- [Examples](../examples/) - Working code samples

Happy tracing!
