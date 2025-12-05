# GoMind Telemetry Module

Welcome to the observability powerhouse of GoMind! Think of this guide as your friendly companion who'll walk you through every aspect of telemetry, from the simplest metric to sophisticated production monitoring. Grab a coffee and let's dive in! ☕

## 📚 Table of Contents

- [🎯 What Is Telemetry and Why Should You Care?](#-what-is-telemetry-and-why-should-you-care)
- [🚀 The Simplest Thing That Works](#-the-simplest-thing-that-works)
- [📊 The Three Types of Metrics](#-the-three-types-of-metrics-and-when-to-use-each)
- [🎨 Adding Context with Labels](#-adding-context-with-labels)
- [🔍 Progressive Disclosure: From Simple to Advanced](#-progressive-disclosure-from-simple-to-advanced)
- [🏗️ Production-Ready Configuration](#️-production-ready-configuration)
- [🐳 Deploying with Docker](#-deploying-with-docker)
- [☸️ Deploying on Kubernetes](#️-deploying-on-kubernetes)
- [🔧 Adding Telemetry to Tools and Agents](#-adding-telemetry-to-tools-and-agents)
- [🏭 The Architecture Under the Hood](#-the-architecture-under-the-hood)
- [🛡️ Production Safety Features](#️-production-safety-features)
- [🧪 Testing Your Telemetry](#-testing-your-telemetry)
- [🔍 Debugging Telemetry Issues](#-debugging-telemetry-issues)
- [📈 Advanced Patterns](#-advanced-patterns)
- [🎯 Best Practices Summary](#-best-practices-summary)
- [🏁 Quick Reference](#-quick-reference)
- [🌐 Distributed Tracing](#-distributed-tracing)
  - [📖 Comprehensive Guide](../docs/DISTRIBUTED_TRACING_GUIDE.md)
- [🎉 Summary](#-summary)

## 🎯 What Is Telemetry and Why Should You Care?

Let me explain this with a story that everyone can relate to.

### The Dashboard Analogy

Imagine you're driving a car. Your dashboard tells you:
- **Speed** - How fast you're going
- **Fuel** - How much gas you have left
- **Temperature** - If your engine is overheating
- **Warning Lights** - If something needs attention

Without this dashboard, you'd be driving blind. You wouldn't know if you're about to run out of gas or if your engine is about to overheat.

**That's exactly what telemetry does for your software!** It gives you a dashboard to see what's happening inside your running application:
- **Metrics** tell you the numbers (requests per second, error rates)
- **Traces** show you the journey (how a request flows through your system)
- **Logs** capture the details (what happened and when)
- **Health checks** warn you of problems (circuit breakers, failures)

### Why Every Application Needs Telemetry

Think about these scenarios:
1. **Your app is slow** - But which part? Database? Network? Your code?
2. **Users report errors** - But you can't reproduce them locally
3. **Memory keeps growing** - But you don't know what's leaking
4. **Your service crashed at 3 AM** - But you were asleep

Without telemetry, you're debugging in the dark. With telemetry, you have X-ray vision into your application.

## 🚀 The Simplest Thing That Works

Let's start with the absolute basics. Here's how to add telemetry to your application in 30 seconds:

```go
package main

import (
    "context"
    "time"

    "github.com/itsneelabh/gomind/telemetry"
)

func main() {
    // Step 1: Initialize telemetry (one line!)
    telemetry.Initialize(telemetry.Config{
        ServiceName: "my-app",
        Enabled:     true,
    })

    // Step 2: Always clean up when your app exits
    defer telemetry.Shutdown(context.Background())

    // Step 3: Emit metrics anywhere in your code
    telemetry.Counter("app.started")

    // That's it! You're now tracking metrics
    processRequest()
}

func processRequest() {
    // Track how long something takes
    start := time.Now()
    defer telemetry.Duration("request.duration_ms", start)

    // Count events
    telemetry.Counter("request.received")

    // Do your actual work here
    time.Sleep(100 * time.Millisecond)

    // Track success
    telemetry.Counter("request.success")
}
```

**That's literally all you need to start!** No complex setup, no configuration files, no external dependencies to install. The telemetry module handles everything internally.

## 📊 The Three Types of Metrics (And When to Use Each)

Just like there are different tools in a toolbox, there are different types of metrics for different jobs:

### 1. Counters - Things That Only Go Up
Think of counters like the odometer in your car - they only increase, never decrease.

```go
// Perfect for counting events
telemetry.Counter("user.login")
telemetry.Counter("api.request", "endpoint", "/users")
telemetry.Counter("error.occurred", "type", "database")
```

**Use counters when you want to know "how many times did this happen?"**

### 2. Histograms - Distributions of Values
Think of histograms like a speed chart - they show you the range and frequency of values.

```go
// Perfect for measuring durations, sizes, or amounts
telemetry.Histogram("response.time_ms", 125.5)
telemetry.Histogram("payload.size_bytes", 2048)
telemetry.Histogram("batch.size", 50)
```

**Use histograms when you want to know "what's the typical value, and what's the range?"**

The beauty of histograms is they automatically calculate:
- Average (mean)
- Median (50th percentile)
- 95th and 99th percentiles
- Min and max values

### 3. Gauges - Values That Go Up and Down
Think of gauges like the fuel gauge in your car - they can increase or decrease.

```go
// Perfect for current state metrics
telemetry.Gauge("memory.used_mb", 512)
telemetry.Gauge("queue.size", 1500)
telemetry.Gauge("active.connections", 42)
```

**Use gauges when you want to know "what's the current value right now?"**

## 🎨 Adding Context with Labels

Labels are like tags on your metrics - they add context and allow you to filter and group your data.

### The Restaurant Menu Analogy
Imagine you run a restaurant and track "orders". That's good, but wouldn't it be better to know:
- **What** was ordered (pizza, pasta, salad)
- **When** it was ordered (lunch, dinner)
- **How** it was ordered (dine-in, takeout, delivery)

That's exactly what labels do for your metrics:

```go
// Without labels - not very useful
telemetry.Counter("orders")  // Total orders... but what kind?

// With labels - now we're talking!
telemetry.Counter("orders",
    "item", "pizza",
    "time", "dinner",
    "type", "delivery")

// You can now answer questions like:
// - How many pizzas were ordered at dinner?
// - What's our most popular delivery item?
// - Is lunch or dinner busier?
```

### Label Best Practices

```go
// ✅ GOOD: Low cardinality labels (limited set of values)
telemetry.Counter("api.request",
    "method", "GET",        // Only ~5 values (GET, POST, PUT, DELETE, PATCH)
    "status", "200",        // Only ~10 values (200, 201, 400, 404, 500, etc.)
    "endpoint", "/users")   // Only ~20-50 endpoints in your API

// ❌ BAD: High cardinality labels (unlimited unique values)
telemetry.Counter("api.request",
    "user_id", "12345",     // Could be millions of unique values!
    "timestamp", "1234567", // Every request has a unique timestamp!
    "request_id", "abc123") // Every request has a unique ID!
```

**Why does cardinality matter?** Each unique combination of labels creates a new metric series. Too many series = memory explosion!

## 🔍 Progressive Disclosure: From Simple to Advanced

The telemetry module follows the principle of progressive disclosure - start simple, add complexity only when needed.

### Level 1: Dead Simple (90% of Your Needs)
```go
// Just emit metrics - that's it!
telemetry.Counter("events.processed")
telemetry.Histogram("processing.time_ms", 45.2)
telemetry.Gauge("queue.depth", 100)
```

### Level 2: With Context (For Distributed Systems)
```go
// Add tracing context for distributed systems
ctx := telemetry.WithBaggage(ctx,
    "request_id", "req-123",
    "user_id", "user-456")

// Metrics now include trace context
telemetry.EmitWithContext(ctx, "payment.processed", 99.99)
```

### Level 3: Full Control (When You Need It)
```go
// Declare metrics upfront for validation
telemetry.DeclareMetrics("payment", telemetry.ModuleConfig{
    Metrics: []telemetry.MetricDefinition{
        {
            Name:    "payment.amount",
            Type:    "histogram",
            Help:    "Payment amounts in USD",
            Labels:  []string{"currency", "method"},
            Unit:    "dollars",
            Buckets: []float64{1, 10, 100, 1000, 10000},
        },
    },
})

// Use advanced emission options
telemetry.EmitWithOptions(ctx, "payment.amount", 99.99,
    telemetry.WithUnit(telemetry.UnitDollars),
    telemetry.WithSampleRate(0.1),  // Sample 10% for high-volume metrics
    telemetry.WithTimestamp(eventTime),
)
```

## 🏗️ Production-Ready Configuration

When you're ready to deploy to production, you need more sophisticated configuration. Let me show you how to set up telemetry that adapts to different environments.

### The Smart Configuration Pattern

Think of this like your phone's battery saver mode:
- **Development**: Full brightness, all features on (capture everything for debugging)
- **Staging**: Balanced mode (good visibility, moderate resource usage)
- **Production**: Power saving mode (minimal overhead, only essential metrics)

Here's how to implement environment-aware configuration:

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    "github.com/itsneelabh/gomind/telemetry"
)

// initTelemetry sets up telemetry based on your environment
func initTelemetry(serviceName string) {
    // Detect environment from APP_ENV variable
    env := os.Getenv("APP_ENV")
    if env == "" {
        env = "development" // Safe default
    }

    // Select the appropriate profile
    var profile telemetry.Profile
    switch env {
    case "production", "prod":
        profile = telemetry.ProfileProduction
    case "staging", "stage", "qa":
        profile = telemetry.ProfileStaging
    default:
        profile = telemetry.ProfileDevelopment
    }

    // Use the profile to get base configuration
    config := telemetry.UseProfile(profile)

    // Override with your service name
    config.ServiceName = serviceName

    // Allow environment variables to override specific settings
    if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
        config.Endpoint = endpoint
    }

    // Initialize telemetry
    if err := telemetry.Initialize(config); err != nil {
        // IMPORTANT: Don't let telemetry failures crash your app!
        log.Printf("WARNING: Telemetry initialization failed: %v", err)
        log.Printf("Application will continue without telemetry")
        return
    }

    log.Printf("✅ Telemetry initialized successfully")
    log.Printf("   Environment: %s", env)
    log.Printf("   Profile: %s", profile)
    log.Printf("   Service: %s", serviceName)
    log.Printf("   Endpoint: %s", config.Endpoint)
}

func main() {
    // Initialize telemetry with environment detection
    initTelemetry("my-service")

    // Always clean up
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := telemetry.Shutdown(ctx); err != nil {
            log.Printf("Warning: Telemetry shutdown error: %v", err)
        }
    }()

    // Your application code here
    runApplication()
}

func runApplication() {
    // Your main application logic here
    // This is where your actual service runs
}
```

### Understanding Telemetry Profiles

The module comes with three pre-configured profiles that represent common deployment scenarios:

#### Development Profile (Maximum Visibility)
```go
// ProfileDevelopment - Capture everything for debugging
// - 100% sampling (see every request)
// - No circuit breaker (don't hide problems)
// - High cardinality limits (track everything)
// - Local endpoint (localhost:4318)
config := telemetry.UseProfile(telemetry.ProfileDevelopment)
```

#### Staging Profile (Balanced Approach)
```go
// ProfileStaging - Good visibility with reasonable overhead
// - 10% sampling (see enough to understand patterns)
// - Circuit breaker enabled (protect the telemetry backend)
// - Moderate cardinality limits
// - Staging collector endpoint
config := telemetry.UseProfile(telemetry.ProfileStaging)
```

#### Production Profile (Optimized for Scale)
```go
// ProfileProduction - Minimal overhead, maximum reliability
// - 0.1% sampling (tiny overhead for high-volume services)
// - Aggressive circuit breaker (fail fast if backend is down)
// - Strict cardinality limits (prevent memory explosion)
// - Production collector endpoint
config := telemetry.UseProfile(telemetry.ProfileProduction)
```

### The Three-Tier Configuration System

Configuration follows a clear priority order (like CSS cascading):

```go
// Priority 1: Explicit configuration (highest priority)
config := telemetry.Config{
    ServiceName: "my-service",
    Endpoint:    "my-collector:4318",  // This wins
}

// Priority 2: Environment variables (medium priority)
// export OTEL_EXPORTER_OTLP_ENDPOINT=env-collector:4318
// If no explicit endpoint, this is used

// Priority 3: Profile defaults (lowest priority)
// If nothing else is set, profile defaults are used
```

Here's a complete example:

```go
func configureTelemetry() telemetry.Config {
    // Start with a profile
    config := telemetry.UseProfile(telemetry.ProfileProduction)

    // Override with explicit values
    config.ServiceName = "payment-service"

    // Environment variables can override
    if endpoint := os.Getenv("TELEMETRY_ENDPOINT"); endpoint != "" {
        config.Endpoint = endpoint
    }

    // Feature flags can control behavior
    if os.Getenv("TELEMETRY_DEBUG") == "true" {
        config.SamplingRate = 1.0  // Temporary 100% sampling for debugging
    }

    return config
}
```

## 🐳 Deploying with Docker

Here's how to configure telemetry for containerized applications:

```dockerfile
# Dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN go build -o myapp .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Copy the binary
COPY --from=builder /app/myapp .

# Set default environment
ENV APP_ENV=production
ENV OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4318

CMD ["./myapp"]
```

```yaml
# docker-compose.yml
version: '3.8'

services:
  myapp:
    build: .
    environment:
      - APP_ENV=staging
      - OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4318
      - SERVICE_NAME=my-service
    depends_on:
      - otel-collector

  otel-collector:
    image: otel/opentelemetry-collector-contrib:latest
    ports:
      - "4318:4318"  # HTTP
      - "4317:4317"  # gRPC
    volumes:
      - ./otel-config.yaml:/etc/otel/config.yaml
    command: ["--config", "/etc/otel/config.yaml"]
```

## ☸️ Deploying on Kubernetes

For Kubernetes deployments, use ConfigMaps and environment variables:

```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  APP_ENV: "production"
  OTEL_EXPORTER_OTLP_ENDPOINT: "otel-collector.monitoring:4318"

---
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-service
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: app
        image: my-service:latest
        envFrom:
        - configMapRef:
            name: app-config
        env:
        - name: SERVICE_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
```

## 🔧 Adding Telemetry to Tools and Agents

Now let's see how telemetry integrates with GoMind's core components - Tools and Agents.

### Adding Telemetry to a Tool

Remember: Tools are passive components that do one thing well. Here's how to add comprehensive telemetry:

```go
package main

import (
    "context"
    "time"

    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

// WeatherTool fetches weather data (passive component)
type WeatherTool struct {
    *core.BaseTool
}

func NewWeatherTool() *WeatherTool {
    tool := &WeatherTool{
        BaseTool: core.NewTool("weather"),
    }

    // Declare the metrics this tool will emit
    telemetry.DeclareMetrics("weather", telemetry.ModuleConfig{
        Metrics: []telemetry.MetricDefinition{
            {
                Name:   "weather.api.calls",
                Type:   "counter",
                Help:   "Number of weather API calls",
                Labels: []string{"city", "status"},
            },
            {
                Name:    "weather.api.latency_ms",
                Type:    "histogram",
                Help:    "Weather API response time",
                Labels:  []string{"city"},
                Unit:    "milliseconds",
                Buckets: []float64{50, 100, 250, 500, 1000, 2000},
            },
            {
                Name:   "weather.cache.hits",
                Type:   "counter",
                Help:   "Weather data cache hits",
            },
            {
                Name:   "weather.temperature",
                Type:   "gauge",
                Help:   "Current temperature reading",
                Labels: []string{"city", "unit"},
            },
        },
    })

    return tool
}

// Weather represents weather data (example type for demonstration)
type Weather struct {
    Temperature float64
    City        string
    Conditions  string
}

// GetWeather demonstrates comprehensive telemetry in a tool
func (w *WeatherTool) GetWeather(ctx context.Context, city string) (*Weather, error) {
    // Track the overall operation
    start := time.Now()
    defer func() {
        telemetry.Histogram("weather.api.latency_ms",
            float64(time.Since(start).Milliseconds()),
            "city", city)
    }()

    // Check cache first
    if cached := w.checkCache(city); cached != nil {
        telemetry.Counter("weather.cache.hits")
        return cached, nil
    }

    // Track API call
    telemetry.Counter("weather.api.calls",
        "city", city,
        "status", "started")

    // Make the actual API call
    weather, err := w.callWeatherAPI(city)

    if err != nil {
        // Track failures
        telemetry.Counter("weather.api.calls",
            "city", city,
            "status", "error")
        telemetry.RecordError("weather.api.error", err.Error())
        return nil, err
    }

    // Track success
    telemetry.Counter("weather.api.calls",
        "city", city,
        "status", "success")

    // Track the actual temperature (business metric)
    telemetry.Gauge("weather.temperature",
        weather.Temperature,
        "city", city,
        "unit", "fahrenheit")

    // Cache the result
    w.updateCache(city, weather)

    return weather, nil
}

// Helper methods (example implementations)
func (w *WeatherTool) checkCache(city string) *Weather {
    // Your cache implementation here
    return nil
}

func (w *WeatherTool) callWeatherAPI(city string) (*Weather, error) {
    // Your API call implementation here
    return &Weather{Temperature: 72, City: city, Conditions: "Sunny"}, nil
}

func (w *WeatherTool) updateCache(city string, weather *Weather) {
    // Your cache update implementation here
}
```

### Adding Telemetry to an Agent

Agents are active orchestrators that coordinate multiple components. They need different telemetry patterns:

```go
package main

import (
    "context"
    "sync"
    "time"

    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

// TravelAgent orchestrates travel planning (active component)
type TravelAgent struct {
    *core.BaseAgent
}

// TripPlan represents a travel plan (example type for demonstration)
type TripPlan struct {
    Destination string
    Weather     *Weather
    Flights     []Flight
    Hotels      []Hotel
}

// Flight and Hotel are example types for demonstration
type Flight struct {
    Number string
    Price  float64
}

type Hotel struct {
    Name  string
    Price float64
}

func NewTravelAgent() *TravelAgent {
    agent := &TravelAgent{
        BaseAgent: core.NewBaseAgent("travel-agent"),
    }

    // Declare metrics for orchestration patterns
    telemetry.DeclareMetrics("travel-agent", telemetry.ModuleConfig{
        Metrics: []telemetry.MetricDefinition{
            {
                Name:   "agent.orchestrations",
                Type:   "counter",
                Help:   "Number of orchestrations performed",
                Labels: []string{"workflow", "status"},
            },
            {
                Name:   "agent.tools.discovered",
                Type:   "gauge",
                Help:   "Number of tools discovered",
                Labels: []string{"type"},
            },
            {
                Name:   "agent.workflow.steps",
                Type:   "counter",
                Help:   "Workflow steps executed",
                Labels: []string{"workflow", "step", "status"},
            },
            {
                Name:    "agent.workflow.duration_ms",
                Type:    "histogram",
                Help:    "Total workflow execution time",
                Labels:  []string{"workflow"},
                Buckets: []float64{100, 500, 1000, 5000, 10000, 30000},
            },
        },
    })

    return agent
}

// PlanTrip demonstrates agent orchestration with telemetry
func (a *TravelAgent) PlanTrip(ctx context.Context, destination string) (*TripPlan, error) {
    // Track the entire orchestration
    start := time.Now()
    defer func() {
        telemetry.Histogram("agent.workflow.duration_ms",
            float64(time.Since(start).Milliseconds()),
            "workflow", "plan_trip")
    }()

    // Add context for distributed tracing
    ctx = telemetry.WithBaggage(ctx,
        "workflow", "plan_trip",
        "destination", destination,
        "agent_id", a.GetID())

    telemetry.Counter("agent.orchestrations",
        "workflow", "plan_trip",
        "status", "started")

    // Step 1: Discover available tools
    telemetry.Counter("agent.workflow.steps",
        "workflow", "plan_trip",
        "step", "discovery",
        "status", "started")

    tools, err := a.Discover(ctx, core.DiscoveryFilter{
        Type: core.ComponentTypeTool,
    })

    if err != nil {
        telemetry.Counter("agent.workflow.steps",
            "workflow", "plan_trip",
            "step", "discovery",
            "status", "error")
        return nil, err
    }

    telemetry.Gauge("agent.tools.discovered",
        float64(len(tools)),
        "type", "all")

    // Step 2: Orchestrate tools in parallel
    telemetry.Counter("agent.workflow.steps",
        "workflow", "plan_trip",
        "step", "orchestration",
        "status", "started")

    var wg sync.WaitGroup
    results := &TripPlan{}

    // Track concurrent operations
    telemetry.Gauge("agent.concurrent_operations", 3)
    defer telemetry.Gauge("agent.concurrent_operations", -3)

    // Get weather (tool invocation)
    wg.Add(1)
    go func() {
        defer wg.Done()
        weatherTool := a.findTool(tools, "weather")
        if weatherTool != nil {
            telemetry.Counter("agent.tool.invocation",
                "tool", "weather",
                "status", "started")
            // Invoke tool...
        }
    }()

    // Get flights (tool invocation)
    wg.Add(1)
    go func() {
        defer wg.Done()
        flightTool := a.findTool(tools, "flights")
        if flightTool != nil {
            telemetry.Counter("agent.tool.invocation",
                "tool", "flights",
                "status", "started")
            // Invoke tool...
        }
    }()

    // Get hotels (tool invocation)
    wg.Add(1)
    go func() {
        defer wg.Done()
        hotelTool := a.findTool(tools, "hotels")
        if hotelTool != nil {
            telemetry.Counter("agent.tool.invocation",
                "tool", "hotels",
                "status", "started")
            // Invoke tool...
        }
    }()

    wg.Wait()

    telemetry.Counter("agent.orchestrations",
        "workflow", "plan_trip",
        "status", "success")

    return results, nil
}

// Helper method to find a tool by name (example implementation)
func (a *TravelAgent) findTool(tools []*core.ServiceInfo, name string) *core.ServiceInfo {
    for _, tool := range tools {
        if tool.Name == name {
            return tool
        }
    }
    return nil
}
```

## 🏭 The Architecture Under the Hood

Let me explain how telemetry works internally, using an analogy everyone understands.

### The Post Office Analogy

Think of the telemetry system like a post office:

```
Your Code (Sender) → Envelope (Metric) → Post Office (Registry) → Delivery Truck (Exporter) → Destination (Collector)
```

Here's what happens when you emit a metric:

```go
telemetry.Counter("request.count")  // You drop a letter in the mailbox
```

1. **The Registry (Post Office)**: Receives your metric and checks if it's valid
2. **The Circuit Breaker (Safety System)**: Ensures the post office isn't overwhelmed
3. **The Cardinality Limiter (Size Checker)**: Makes sure you're not sending a package that's too big
4. **The Exporter (Delivery Truck)**: Batches metrics and sends them to the collector
5. **The Collector (Destination)**: Receives and stores your metrics

### The Three-Layer Architecture

```
┌─────────────────────────────────────────┐
│         Simple API Layer                 │  ← What you use (Counter, Histogram, Gauge)
│    Emit(), Counter(), Histogram()        │
├─────────────────────────────────────────┤
│        Smart Registry Layer              │  ← Manages everything
│   Thread-safe, Cardinality limits        │     (You never see this)
├─────────────────────────────────────────┤
│     OpenTelemetry Provider Layer         │  ← Does the actual work
│    HTTP export to collectors             │     (Handles the complexity)
└─────────────────────────────────────────┘
```

## 🛡️ Production Safety Features

The telemetry module includes several safety features to protect your application in production:

### 1. The Circuit Breaker (Automatic Failure Protection)

Just like a circuit breaker in your house prevents electrical overload, the telemetry circuit breaker prevents a failing metrics backend from affecting your application:

```go
// The circuit breaker has three states:
// CLOSED: Normal operation, metrics flow through
// OPEN: Backend is down, metrics are dropped (fail fast)
// HALF-OPEN: Testing if backend recovered

// You don't need to configure this manually, but here's how it works:
config := telemetry.Config{
    CircuitBreaker: telemetry.CircuitConfig{
        Enabled:      true,
        MaxFailures:  5,              // Open after 5 failures
        RecoveryTime: 30 * time.Second, // Try again after 30 seconds
    },
}

// In your code, you just emit metrics normally
telemetry.Counter("my.metric")  // If circuit is open, this returns immediately
```

### 2. Cardinality Limits (Memory Protection)

High cardinality can cause memory explosions. The module protects you automatically:

```go
// BAD: User ID as a label creates millions of metric series
for _, userID := range users {
    telemetry.Counter("user.action", "user_id", userID)  // DON'T DO THIS!
}

// The cardinality limiter will automatically:
// 1. Detect high cardinality
// 2. Start dropping new label combinations
// 3. Log warnings so you know what's happening

// GOOD: Use bounded labels instead
telemetry.Counter("user.action",
    "user_type", "premium",  // Only a few types
    "action", "login")       // Only a few actions
```

### 3. Graceful Degradation

The module is designed to never crash your application:

```go
func main() {
    // Even if telemetry fails to initialize, your app keeps running
    if err := telemetry.Initialize(config); err != nil {
        log.Printf("Telemetry failed to initialize: %v", err)
        // Your app continues without telemetry
    }

    // Even if the metrics backend dies, your app keeps running
    telemetry.Counter("my.metric")  // Never panics, never blocks

    // Even if shutdown fails, your app exits cleanly
    defer func() {
        if err := telemetry.Shutdown(context.Background()); err != nil {
            log.Printf("Telemetry shutdown failed: %v", err)
            // Your app still exits normally
        }
    }()
}
```

## 🧪 Testing Your Telemetry

Here's how to test that your components emit metrics correctly:

```go
package main

import (
    "context"
    "testing"

    "github.com/itsneelabh/gomind/telemetry"
)

// MyComponent is an example component for testing
type MyComponent struct {
    name string
}

func NewMyComponent() *MyComponent {
    return &MyComponent{name: "test-component"}
}

func (c *MyComponent) DoSomething(ctx context.Context) error {
    // Emit some metrics
    telemetry.Counter("component.operation", "name", c.name)
    return nil
}

func TestMyComponentTelemetry(t *testing.T) {
    // In tests, use development profile for predictable behavior
    config := telemetry.UseProfile(telemetry.ProfileDevelopment)
    config.ServiceName = "test"

    // Initialize telemetry for the test
    if err := telemetry.Initialize(config); err != nil {
        t.Fatalf("Failed to initialize telemetry: %v", err)
    }
    defer telemetry.Shutdown(context.Background())

    // Get health before operation
    healthBefore := telemetry.GetHealth()

    // Run your component
    component := NewMyComponent()
    err := component.DoSomething(context.Background())

    if err != nil {
        t.Fatalf("Component failed: %v", err)
    }

    // Get health after operation
    healthAfter := telemetry.GetHealth()

    // Verify metrics were emitted
    if healthAfter.MetricsEmitted <= healthBefore.MetricsEmitted {
        t.Error("Expected metrics to be emitted")
    }

    // For more detailed testing, you can:
    // 1. Use a test exporter to capture exact metrics
    // 2. Mock the telemetry system
    // 3. Use the metrics registry to query specific metrics
}

// Test with different profiles
func TestTelemetryProfiles(t *testing.T) {
    profiles := []telemetry.Profile{
        telemetry.ProfileDevelopment,
        telemetry.ProfileStaging,
        telemetry.ProfileProduction,
    }

    for _, profile := range profiles {
        t.Run(string(profile), func(t *testing.T) {
            config := telemetry.UseProfile(profile)

            // Verify profile-specific settings
            switch profile {
            case telemetry.ProfileDevelopment:
                if config.SamplingRate != 1.0 {
                    t.Error("Development should have 100% sampling")
                }
            case telemetry.ProfileProduction:
                if config.SamplingRate >= 0.1 {
                    t.Error("Production should have low sampling rate")
                }
            }
        })
    }
}
```

## 🔍 Debugging Telemetry Issues

When telemetry isn't working as expected, here's how to debug:

```go
import (
    "fmt"
    "github.com/itsneelabh/gomind/telemetry"
)

func debugTelemetry() {
    // 1. Check if telemetry is initialized
    health := telemetry.GetHealth()
    fmt.Printf("Telemetry Health Check:\n")
    fmt.Printf("  Initialized: %v\n", health.Initialized)
    fmt.Printf("  Metrics Emitted: %d\n", health.MetricsEmitted)
    fmt.Printf("  Circuit State: %s\n", health.CircuitState)
    fmt.Printf("  Last Error: %s\n", health.LastError)

    // 2. Enable debug logging
    config := telemetry.Config{
        ServiceName: "debug-test",
        Enabled:     true,
        // In development, you might want to see everything
    }

    // 3. Test with a simple metric
    telemetry.Counter("debug.test")

    // 4. Check health again
    healthAfter := telemetry.GetHealth()
    if healthAfter.MetricsEmitted == health.MetricsEmitted {
        fmt.Println("WARNING: Metric was not emitted!")
        fmt.Printf("Possible reasons:\n")
        fmt.Printf("- Telemetry not initialized\n")
        fmt.Printf("- Circuit breaker is open\n")
        fmt.Printf("- Sampling rate is 0\n")
    }
}
```

## 📈 Advanced Patterns

### Pattern 1: Request Tracing
```go
import (
    "net/http"
    "github.com/itsneelabh/gomind/telemetry"
)

func handleRequest(w http.ResponseWriter, r *http.Request) {
    // Create request context with correlation ID
    ctx := telemetry.WithBaggage(r.Context(),
        "request_id", r.Header.Get("X-Request-ID"),
        "user_id", getUserID(r),
        "endpoint", r.URL.Path)

    // All metrics in this request will include this context
    defer telemetry.TimeOperation("request.duration",
        "endpoint", r.URL.Path,
        "method", r.Method)()

    // Your handler logic...
    processHTTPRequest(w, r)
}

// Helper function (example)
func getUserID(r *http.Request) string {
    // Your user ID extraction logic here
    return "user-123"
}

func processHTTPRequest(w http.ResponseWriter, r *http.Request) {
    // Your request processing logic here
    w.WriteHeader(http.StatusOK)
}
```

### Pattern 2: Batch Operations
```go
// Item represents a work item (example type)
type Item struct {
    ID   string
    Data interface{}
}

func processBatch(items []Item) {
    // Track batch metrics
    telemetry.Histogram("batch.size", float64(len(items)))

    start := time.Now()
    successful := 0
    failed := 0

    for _, item := range items {
        if err := processItem(item); err != nil {
            failed++
            telemetry.Counter("item.processing.failed",
                "error", err.Error())
        } else {
            successful++
            telemetry.Counter("item.processing.success")
        }
    }

    // Summary metrics
    telemetry.Histogram("batch.duration_ms",
        float64(time.Since(start).Milliseconds()))
    telemetry.Gauge("batch.success_rate",
        float64(successful)/float64(len(items))*100)
}

// Helper function (example)
func processItem(item Item) error {
    // Your item processing logic here
    return nil
}
```

### Pattern 3: Resource Monitoring
```go
import (
    "runtime"
    "time"
    "github.com/itsneelabh/gomind/telemetry"
)

func monitorResources() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)

        // Memory metrics
        telemetry.Gauge("memory.alloc_mb", float64(m.Alloc/1024/1024))
        telemetry.Gauge("memory.total_alloc_mb", float64(m.TotalAlloc/1024/1024))
        telemetry.Gauge("memory.sys_mb", float64(m.Sys/1024/1024))
        telemetry.Gauge("memory.num_gc", float64(m.NumGC))

        // Goroutine metrics
        telemetry.Gauge("goroutines.count", float64(runtime.NumGoroutine()))
    }
}
```

## 🎯 Best Practices Summary

### DO ✅
- **Initialize early**: Set up telemetry at the start of main()
- **Use profiles**: Leverage pre-configured profiles for different environments
- **Add context**: Use labels to make metrics meaningful
- **Handle failures gracefully**: Don't let telemetry crash your app
- **Test your metrics**: Verify components emit expected metrics
- **Monitor cardinality**: Use bounded label values

### DON'T ❌
- **Don't use high-cardinality labels**: No user IDs, timestamps, or UUIDs
- **Don't block on telemetry**: Always use timeouts for shutdown
- **Don't emit sensitive data**: No passwords, tokens, or PII in metrics
- **Don't over-instrument**: Start simple, add more as needed
- **Don't ignore errors**: Log telemetry failures for debugging

## 🏁 Quick Reference

### Initialization
```go
// Simplest
telemetry.Initialize(telemetry.Config{ServiceName: "my-app"})

// With environment detection
config := telemetry.UseProfile(profile)
telemetry.Initialize(config)

// Always shutdown
defer telemetry.Shutdown(context.Background())
```

### Basic Metrics
```go
telemetry.Counter("metric.name", "label", "value")
telemetry.Histogram("metric.name", 123.45, "label", "value")
telemetry.Gauge("metric.name", 67.89, "label", "value")
telemetry.Duration("metric.name", startTime, "label", "value")
```

### With Context
```go
ctx = telemetry.WithBaggage(ctx, "key", "value")
telemetry.EmitWithContext(ctx, "metric.name", 123.45)
```

### Health Check
```go
health := telemetry.GetHealth()
if !health.Initialized {
    // Telemetry not working
}
```

## 🌐 Distributed Tracing

Distributed tracing allows you to follow a request as it flows through multiple services. The telemetry module provides HTTP instrumentation that automatically propagates trace context using W3C TraceContext headers.

### The Journey of a Request

Imagine a user request that touches three services:

```
User → API Gateway → Weather Service → Database
```

Without distributed tracing, when something goes wrong, you have three separate log files with no way to connect them. With distributed tracing, you can see the entire journey as a single trace:

```
Trace ID: abc123
├── API Gateway (100ms)
│   ├── Weather Service (80ms)
│   │   └── Database Query (20ms)
│   └── Response formatting (5ms)
└── Total: 105ms
```

### Server-Side: Tracing Middleware

Wrap your HTTP handlers with `TracingMiddleware` to automatically:
- Extract trace context from incoming requests
- Create spans for each request
- Record HTTP metrics (status codes, latency)
- Propagate context to your handler code

```go
package main

import (
    "net/http"
    "github.com/itsneelabh/gomind/telemetry"
)

func main() {
    // Initialize telemetry FIRST
    telemetry.Initialize(telemetry.Config{
        ServiceName: "weather-service",
        Endpoint:    "http://otel-collector:4318",
    })
    defer telemetry.Shutdown(context.Background())

    // Create your handlers
    mux := http.NewServeMux()
    mux.HandleFunc("/api/weather", handleWeather)
    mux.HandleFunc("/health", handleHealth)

    // Wrap with tracing middleware
    tracedHandler := telemetry.TracingMiddleware("weather-service")(mux)

    http.ListenAndServe(":8080", tracedHandler)
}
```

### Excluding Paths from Tracing

Health checks and metrics endpoints shouldn't create traces (they're noisy!):

```go
config := &telemetry.TracingMiddlewareConfig{
    ExcludedPaths: []string{"/health", "/metrics", "/ready", "/live"},
}

tracedHandler := telemetry.TracingMiddlewareWithConfig("my-service", config)(mux)
```

### Custom Span Names

By default, spans are named `HTTP GET /api/weather`. Customize this:

```go
config := &telemetry.TracingMiddlewareConfig{
    SpanNameFormatter: func(operation string, r *http.Request) string {
        // Create semantic span names
        return r.Method + " " + getRoutePattern(r)  // "GET /api/users/:id"
    },
}
```

### Client-Side: Traced HTTP Client

When calling other services, use `NewTracedHTTPClient` to automatically propagate trace context:

```go
// Create once, reuse for all requests
client := telemetry.NewTracedHTTPClient(nil)

func callWeatherService(ctx context.Context, city string) (*Weather, error) {
    // Context carries trace information
    req, _ := http.NewRequestWithContext(ctx, "GET",
        "http://weather-service/api/weather?city="+city, nil)

    // Trace headers (traceparent, tracestate) are automatically injected
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    // Parse response...
}
```

### With Custom Transport Settings

For production, configure connection pooling:

```go
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
}

client := telemetry.NewTracedHTTPClientWithTransport(transport)
```

### Complete Example: Multi-Service Tracing

Here's how tracing flows across services:

```go
// === API Gateway (service 1) ===
func handleUserRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()  // Contains incoming trace context

    // Call downstream services - context propagates automatically
    weather, _ := weatherClient.GetWeather(ctx, "NYC")
    news, _ := newsClient.GetNews(ctx, "NYC")

    // Combine and respond
    json.NewEncoder(w).Encode(map[string]interface{}{
        "weather": weather,
        "news":    news,
    })
}

// === Weather Service (service 2) ===
func handleWeather(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()  // Trace context from gateway!

    // This span is a child of the gateway's span
    telemetry.Counter("weather.requests", "city", r.URL.Query().Get("city"))

    // Fetch data and respond...
}

// === In Jaeger/Grafana Tempo ===
// You'll see a single trace spanning both services:
//
// api-gateway: handleUserRequest (150ms)
// ├── weather-service: handleWeather (50ms)
// └── news-service: handleNews (80ms)
```

### Environment Configuration

Configure tracing via environment variables:

```bash
# Required for trace collection
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318

# Service identification in traces
OTEL_SERVICE_NAME=weather-service

# Sampling (production should use lower rates)
OTEL_TRACES_SAMPLER=traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1  # Sample 10% of traces
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: weather-service
spec:
  template:
    spec:
      containers:
      - name: app
        image: weather-service:latest
        env:
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          value: "http://otel-collector:4318"
        - name: OTEL_SERVICE_NAME
          value: "weather-service"
```

### Tracing Best Practices

**DO:**
- Initialize telemetry before creating traced middleware/clients
- Exclude health/metrics endpoints from tracing
- Use semantic span names that match your routing patterns
- Reuse `TracedHTTPClient` instances (connection pooling)
- Always pass `context.Context` through your call chain

**DON'T:**
- Create new traced clients for each request
- Trace every single internal operation (too noisy)
- Forget to call `telemetry.Shutdown()` (traces may be lost)
- Use tracing without an OTEL Collector (nowhere for traces to go!)

### 📖 Comprehensive Distributed Tracing Guide

For a complete deep-dive into distributed tracing with GoMind, including:
- **Trace-Log Correlation** - Connecting traces to logs for easier debugging
- **Complete Multi-Service Examples** - Based on actual working examples in `examples/agent-with-telemetry/`
- **Infrastructure Setup** - OTEL Collector, Jaeger, and Grafana configuration
- **Troubleshooting Guide** - Common problems and solutions
- **Best Practices** - Production-ready patterns

See the **[Distributed Tracing and Log Correlation Guide](../docs/DISTRIBUTED_TRACING_GUIDE.md)**.

## 🎉 Summary

The telemetry module is your application's dashboard, giving you visibility into what's happening in production. It's designed to be:

- **Simple to start with** - One line to initialize, one line to emit metrics
- **Safe in production** - Circuit breakers, cardinality limits, graceful degradation
- **Flexible when needed** - Profiles, contexts, advanced options

Remember: Good telemetry is like good insurance - you hope you never need it, but when you do, you're incredibly glad it's there.

Start simple, add complexity as needed, and always prioritize your application's stability over perfect metrics. Happy monitoring! 📊