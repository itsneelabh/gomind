# GoMind Telemetry Module

Production-grade observability with zero-friction integration and progressive disclosure.

## 📚 Table of Contents

1. [What Does This Module Do?](#-what-does-this-module-do)
2. [Quick Start](#-quick-start)
3. [**Using Telemetry with GoMind Tools and Agents** ⭐](#-using-telemetry-with-gomind-tools-and-agents)
   - Complete working examples for both Tools and Agents
   - Step-by-step integration guide
   - Where to add telemetry in your code
   - Common patterns for both component types
4. [How It Works](#-how-it-works)
5. [Metric Types Explained](#-metric-types-explained)
6. [Context Propagation](#-context-propagation-distributed-tracing)
7. [Configuration Profiles](#-configuration-profiles)
8. [Production Safety Features](#-production-safety-features)
9. [FAQ for Junior Developers](#-faq-for-junior-developers)
10. [API Reference](#api-reference)

## 🎯 What Does This Module Do?

Think of telemetry as the **dashboard of your car**. Just like how your car's dashboard shows speed, fuel, and engine health, this module shows what's happening inside your distributed system in real-time.

It provides three essential observability pillars:

1. **Metrics** - Numbers that matter (request counts, latencies, error rates)
2. **Traces** - The journey of a request across services (distributed tracing)
3. **Context Propagation** - Carrying important metadata across service boundaries

### Real-World Analogy: The Package Delivery System

Imagine tracking a package from warehouse to doorstep:
- **Metrics** - How many packages processed per hour (like monitoring throughput)
- **Traces** - Following one specific package's journey (like distributed tracing)
- **Context** - The tracking number that stays with the package (like baggage propagation)

The telemetry module ensures you can:
1. Monitor system health at a glance
2. Debug issues by tracing specific requests
3. Understand performance bottlenecks
4. Track business metrics that matter

## 🚀 Quick Start

### Installation

```go
import "github.com/itsneelabh/gomind/telemetry"
```

### Zero to Telemetry in 30 Seconds

```go
// 1. Initialize with smart defaults
telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileDevelopment))
defer telemetry.Shutdown(context.Background())

// 2. Emit metrics with one line
telemetry.Emit("user.login", 1.0, "status", "success", "country", "US")

// 3. That's it! Metrics are now flowing to your backend
```

## 🤖 Using Telemetry with GoMind Tools and Agents

### ✅ Getting Started Checklist

Follow this checklist to add telemetry to your GoMind tools and agents:

1. **[ ] Initialize telemetry in main.go**
   ```go
   telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileDevelopment))
   defer telemetry.Shutdown(context.Background())
   ```

2. **[ ] Start OpenTelemetry Collector**
   ```bash
   docker run -p 4318:4318 otel/opentelemetry-collector:latest
   ```

3. **[ ] Add metrics to your tool/agent methods**
   ```go
   // For Tools (passive components)
   telemetry.Counter("my_tool.operations")
   telemetry.Histogram("my_tool.duration_ms", duration)
   
   // For Agents (active orchestrators)
   telemetry.Counter("my_agent.orchestrations")
   telemetry.Histogram("my_agent.workflow_duration_ms", duration)
   ```

4. **[ ] Check metrics are flowing**
   ```go
   health := telemetry.GetHealth()
   fmt.Printf("Metrics emitted: %d\n", health.MetricsEmitted)
   ```

5. **[ ] Switch to production profile when deploying**
   ```go
   telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileProduction))
   ```

### Complete Examples - From Zero to Production

Here are complete, working examples of how to add telemetry to both GoMind tools and agents:

#### Step 1: Create Your Main Application

```go
// main.go - Your application entry point
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

func main() {
    // STEP 1: Initialize telemetry ONCE in your main function
    // This sets up the global telemetry system for ALL tools and agents
    config := telemetry.UseProfile(telemetry.ProfileDevelopment)
    config.ServiceName = "my-system"  // Name your overall system
    
    err := telemetry.Initialize(config)
    if err != nil {
        log.Fatalf("Failed to initialize telemetry: %v", err)
    }
    
    // IMPORTANT: Always shutdown telemetry when your app exits
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        telemetry.Shutdown(ctx)
        log.Println("Telemetry shutdown complete")
    }()
    
    // STEP 2: Create your components - they'll automatically use telemetry
    // Tools (passive components that respond to requests)
    calculatorTool := NewCalculatorTool()
    weatherTool := NewWeatherTool()
    
    // Agents (active orchestrators that discover and coordinate)
    orchestratorAgent := NewOrchestratorAgent()
    
    // STEP 3: Use your components - metrics flow automatically
    ctx := context.Background()
    
    // Example: Process a request with context
    ctx = telemetry.WithBaggage(ctx, 
        "request_id", "req-123",
        "user_id", "user-456")
    
    // Example: Tool usage
    calcResult, _ := calculatorTool.Add(ctx, 5, 3)
    
    // Example: Agent orchestration
    report, err := orchestratorAgent.GenerateReport(ctx, "AAPL")
    if err != nil {
        log.Printf("Error: %v", err)
    }
    
    log.Printf("Calculator result: %v", calcResult)
    log.Printf("Report: %v", report)
}
```

#### Step 2: Create a Tool with Built-in Telemetry

```go
// calculator_tool.go - A passive tool that performs calculations
package main

import (
    "context"
    "time"
    
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

type CalculatorTool struct {
    *core.BaseTool  // Tools are passive components
}

func NewCalculatorTool() *CalculatorTool {
    // Create base tool
    tool := &CalculatorTool{
        BaseTool: core.NewTool("calculator"),
    }
    
    // Register this tool's metrics (optional but recommended)
    telemetry.DeclareMetrics("calculator", telemetry.ModuleConfig{
        Metrics: []telemetry.MetricDefinition{
            {
                Name:   "calculator.operation.duration_ms",
                Type:   "histogram",
                Help:   "Time to perform calculation in milliseconds",
                Labels: []string{"operation", "status"},
                Unit:   "milliseconds",
            },
            {
                Name:   "calculator.operations.count",
                Type:   "counter",
                Help:   "Number of calculations performed",
                Labels: []string{"operation", "status"},
            },
        },
    })
    
    return tool
}

// Tools are passive - they only respond to requests
func (ct *CalculatorTool) Add(ctx context.Context, a, b float64) (float64, error) {
    // Track the start time for latency measurement
    start := time.Now()
    
    // Track that we're starting a calculation
    telemetry.Counter("calculator.operations.count", 
        "operation", "add",
        "status", "started")
    
    // Perform the calculation (tools just do their job)
    result := a + b
    
    // Track successful completion
    telemetry.Counter("calculator.operations.count",
        "operation", "add",
        "status", "success")
    
    // Track the duration
    duration := time.Since(start).Milliseconds()
    telemetry.Histogram("calculator.operation.duration_ms", float64(duration),
        "operation", "add",
        "status", "success")
    
    return result, nil
}
```

#### Step 3: Create an Agent with Built-in Telemetry

```go
// orchestrator_agent.go - An active agent that orchestrates tools
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

type OrchestratorAgent struct {
    *core.BaseAgent  // Agents are active orchestrators
}

func NewOrchestratorAgent() *OrchestratorAgent {
    // Create base agent
    agent := &OrchestratorAgent{
        BaseAgent: core.NewBaseAgent("orchestrator"),
    }
    
    // Register this agent's metrics
    telemetry.DeclareMetrics("orchestrator", telemetry.ModuleConfig{
        Metrics: []telemetry.MetricDefinition{
            {
                Name:   "orchestrator.workflow.duration_ms",
                Type:   "histogram",
                Help:   "Time to complete workflow in milliseconds",
                Labels: []string{"workflow", "status"},
            },
            {
                Name:   "orchestrator.tools.discovered",
                Type:   "gauge",
                Help:   "Number of tools discovered",
                Labels: []string{"type"},
            },
        },
    })
    
    return agent
}

// Agents can discover and orchestrate tools
func (oa *OrchestratorAgent) GenerateReport(ctx context.Context, symbol string) (interface{}, error) {
    // Track the start time for latency measurement
    start := time.Now()
    
    // Track workflow start
    telemetry.Counter("orchestrator.workflow.count", 
        "workflow", "generate_report",
        "status", "started")
    
    // Add context for distributed tracing
    ctx = telemetry.WithBaggage(ctx,
        "operation", "generate_report",
        "symbol", symbol)
    
    // Discover available tools (agents can discover, tools cannot)
    tools, err := oa.Discover(ctx, core.DiscoveryFilter{
        Type: core.ComponentTypeTool,
    })
    
    if err != nil {
        telemetry.Counter("orchestrator.errors",
            "type", "discovery_failed")
        return nil, err
    }
    
    // Track discovered tools
    telemetry.Gauge("orchestrator.tools.discovered", float64(len(tools)),
        "type", "all")
    
    // Orchestrate multiple tools (example)
    // ... call calculator tool, weather tool, etc.
    
    // Track successful completion
    telemetry.Counter("orchestrator.workflow.count",
        "workflow", "generate_report",
        "status", "success")
    
    // Track the duration
    duration := time.Since(start).Milliseconds()
    telemetry.Histogram("orchestrator.workflow.duration_ms", float64(duration),
        "workflow", "generate_report",
        "status", "success")
    
    return map[string]interface{}{
        "symbol": symbol,
        "tools_used": len(tools),
        "timestamp": time.Now(),
    }, nil
}
```

#### Step 4: Multi-Component Orchestration with Telemetry

```go
// advanced_orchestrator.go - Coordinate multiple tools and agents
package main

import (
    "context"
    "sync"
    "time"
    
    "github.com/itsneelabh/gomind/telemetry"
)

type AdvancedOrchestrator struct {
    // Tools (passive components)
    calculatorTool *CalculatorTool
    weatherTool    *WeatherTool
    
    // Agents (active orchestrators)
    analysisAgent  *AnalysisAgent
}

func (o *AdvancedOrchestrator) GenerateComplexReport(ctx context.Context, symbol string, city string) error {
    // Start tracking the overall operation
    start := time.Now()
    
    // Create a trace for the entire operation
    ctx = telemetry.WithBaggage(ctx,
        "operation", "generate_report",
        "report_id", generateReportID())
    
    telemetry.Counter("report.generation.started")
    
    // Track concurrent operations (mix of tools and agents)
    telemetry.Gauge("concurrent_operations", 3, "type", "mixed_orchestration")
    defer telemetry.Gauge("concurrent_operations", -3, "type", "mixed_orchestration")
    
    // Run tools and agents in parallel
    var wg sync.WaitGroup
    var calcErr, weatherErr, analysisErr error
    var calcResult, weatherResult, analysisResult interface{}
    
    wg.Add(3)
    
    // Calculator tool (passive component)
    go func() {
        defer wg.Done()
        telemetry.Counter("component.invocation", "type", "tool", "name", "calculator", "status", "started")
        calcResult, calcErr = o.calculatorTool.Calculate(ctx, 100, 50)
        
        if calcErr != nil {
            telemetry.Counter("component.invocation", "type", "tool", "name", "calculator", "status", "error")
        } else {
            telemetry.Counter("component.invocation", "type", "tool", "name", "calculator", "status", "success")
        }
    }()
    
    // Weather tool (passive component)
    go func() {
        defer wg.Done()
        telemetry.Counter("component.invocation", "type", "tool", "name", "weather", "status", "started")
        weatherResult, weatherErr = o.weatherTool.GetWeather(ctx, city)
        
        if weatherErr != nil {
            telemetry.Counter("component.invocation", "type", "tool", "name", "weather", "status", "error")
        } else {
            telemetry.Counter("component.invocation", "type", "tool", "name", "weather", "status", "success")
        }
    }()
    
    // Analysis agent (active orchestrator)
    go func() {
        defer wg.Done()
        telemetry.Counter("component.invocation", "type", "agent", "name", "analysis", "status", "started")
        analysisResult, analysisErr = o.analysisAgent.Analyze(ctx, symbol)
        
        if analysisErr != nil {
            telemetry.Counter("component.invocation", "type", "agent", "name", "analysis", "status", "error")
        } else {
            telemetry.Counter("component.invocation", "type", "agent", "name", "analysis", "status", "success")
        }
    }()
    
    // Wait for all tools and agents to complete
    wg.Wait()
    
    // Track completion time
    duration := time.Since(start).Milliseconds()
    telemetry.Histogram("report.generation.duration_ms", float64(duration))
    
    // Check for errors
    if calcErr != nil || weatherErr != nil || analysisErr != nil {
        telemetry.Counter("report.generation.completed", "status", "partial_failure")
        // Handle errors...
    } else {
        telemetry.Counter("report.generation.completed", "status", "success")
    }
    
    // Combine results and generate report
    // ... your business logic here ...
    
    return nil
}
```

### Where to Add Telemetry in Your Component Code

Here's a visual guide showing exactly where to add telemetry calls:

```go
// ┌─────────────────────────────────────┐
// │         INITIALIZATION              │
// └─────────────────────────────────────┘
func main() {
    // ↓ Initialize telemetry FIRST, before creating any components
    telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileDevelopment))
    defer telemetry.Shutdown(context.Background())
    
    // ↓ Then create your tools and agents
    tool := NewMyTool()
    agent := NewMyAgent()
}

// ┌─────────────────────────────────────┐
// │         TOOL CREATION               │
// └─────────────────────────────────────┘
func NewMyTool() *MyTool {
    tool := &MyTool{
        BaseTool: core.NewTool("my-tool"),  // Tools are passive
    }
    
    // ↓ Declare your tool's metrics
    telemetry.DeclareMetrics("my-tool", telemetry.ModuleConfig{
        Metrics: []telemetry.MetricDefinition{
            {
                Name: "my_tool.operations",
                Type: "counter",
                Help: "Number of operations performed",
            },
        },
    })
    
    return tool
}

// ┌─────────────────────────────────────┐
// │         AGENT CREATION              │
// └─────────────────────────────────────┘
func NewMyAgent() *MyAgent {
    agent := &MyAgent{
        BaseAgent: core.NewBaseAgent("my-agent"),  // Agents orchestrate
    }
    
    // ↓ Declare your agent's metrics
    telemetry.DeclareMetrics("my-agent", telemetry.ModuleConfig{
        Metrics: []telemetry.MetricDefinition{
            {
                Name: "my_agent.orchestrations",
                Type: "counter",
                Help: "Number of orchestrations performed",
            },
        },
    })
    
    return agent
}

// ┌─────────────────────────────────────┐
// │         COMPONENT METHODS           │
// └─────────────────────────────────────┘
// Tool method (passive - just responds)
func (t *MyTool) Process(ctx context.Context, input string) (string, error) {
    start := time.Now()
    telemetry.Counter("my_tool.operations", "status", "started")
    
    // Tools just process and return
    result := processInput(input)
    
    telemetry.Counter("my_tool.operations", "status", "success")
    telemetry.Histogram("my_tool.duration_ms", 
        float64(time.Since(start).Milliseconds()))
    
    return result, nil
}

// Agent method (active - orchestrates)
func (a *MyAgent) Orchestrate(ctx context.Context, input string) error {
    // ↓ Track start of orchestration
    start := time.Now()
    telemetry.Counter("my_agent.orchestrations", "status", "started")
    
    // ↓ Add context for tracing
    ctx = telemetry.WithBaggage(ctx, "input_type", "text")
    
    // ↓ Discover and orchestrate tools
    tools, _ := a.Discover(ctx, core.DiscoveryFilter{
        Type: core.ComponentTypeTool,
    })
    telemetry.Gauge("my_agent.tools_discovered", float64(len(tools)))
    
    // ↓ Orchestrate the tools
    result, err := orchestrateTools(tools, input)
    
    if err != nil {
        // ↓ Track errors
        telemetry.Counter("my_agent.errors", "type", err.Error())
        return err
    }
    
    // ↓ Track success and timing
    telemetry.Counter("my_agent.orchestrations", "status", "success")
    telemetry.Histogram("my_agent.duration_ms", 
        float64(time.Since(start).Milliseconds()))
    
    // ↓ Track business metrics
    telemetry.EmitWithContext(ctx, "my_agent.result_size", float64(len(result)))
    
    return nil
}
```

### Common Telemetry Patterns for Tools and Agents

#### Pattern 1: Track Component Lifecycle
```go
// For Tools
func (t *MyTool) Initialize(ctx context.Context) error {
    telemetry.Counter("component.lifecycle", "type", "tool", "name", t.Name, "event", "initialize")
    // ... initialization code ...
    return nil
}

// For Agents
func (a *MyAgent) Initialize(ctx context.Context) error {
    telemetry.Counter("component.lifecycle", "type", "agent", "name", a.Name, "event", "initialize")
    // ... initialization code ...
    return nil
}
```

#### Pattern 2: Track Resource Usage
```go
// Tools track their processing
func (t *MyTool) ProcessBatch(items []Item) {
    telemetry.Histogram("tool.batch_size", float64(len(items)), 
        "tool", t.Name)
    // Process items...
}

// Agents track orchestration complexity
func (a *MyAgent) OrchestrateWorkflow(ctx context.Context) {
    // Track number of components being orchestrated
    tools, _ := a.Discover(ctx, core.DiscoveryFilter{Type: core.ComponentTypeTool})
    telemetry.Histogram("agent.components_orchestrated", float64(len(tools)), 
        "agent", a.Name)
    
    // Orchestrate...
}
```

#### Pattern 3: Track Tool vs Agent Operations
```go
// Tools track simple operations
func (t *MyTool) CallAPI(ctx context.Context, endpoint string) error {
    start := time.Now()
    
    // Track API call from tool
    telemetry.Counter("external_api.calls", 
        "component_type", "tool",
        "component_name", t.Name,
        "endpoint", endpoint)
    
    resp, err := http.Get(endpoint)
    
    // ... make the call ...
    return err
}

// Agents track orchestration
func (a *MyAgent) OrchestrateCalls(ctx context.Context) error {
    start := time.Now()
    
    // Track orchestration
    telemetry.Counter("orchestration.started", 
        "agent", a.Name)
    
    // Discover and use multiple tools
    tools, _ := a.Discover(ctx, core.DiscoveryFilter{Type: core.ComponentTypeTool})
    
    // Track orchestration complexity
    telemetry.Histogram("orchestration.complexity",
        float64(len(tools)),
        "agent", a.Name)
    
    // ... orchestrate tools ...
    
    telemetry.Histogram("orchestration.duration_ms",
        float64(time.Since(start).Milliseconds()),
        "agent", a.Name)
    
    return nil
}
```

### Testing Your Component's Telemetry

```go
// component_test.go - Test that your tools and agents emit metrics correctly
package main

import (
    "context"
    "testing"
    "time"
    
    "github.com/itsneelabh/gomind/telemetry"
)

func TestComponentTelemetry(t *testing.T) {
    // Initialize telemetry for testing
    telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileDevelopment))
    defer telemetry.Shutdown(context.Background())
    
    // Test Tool telemetry
    tool := NewCalculatorTool()
    
    // Test Agent telemetry
    agent := NewOrchestratorAgent()
    
    // Get initial metrics
    healthBefore := telemetry.GetHealth()
    
    // Run your components
    ctx := context.Background()
    
    // Test tool
    _, err := tool.Add(ctx, 5, 3)
    if err != nil {
        t.Fatalf("Tool error: %v", err)
    }
    
    // Test agent
    _, err = agent.GenerateReport(ctx, "AAPL")
    if err != nil {
        t.Fatalf("Agent error: %v", err)
    }
    
    // Check metrics were emitted
    healthAfter := telemetry.GetHealth()
    if healthAfter.MetricsEmitted <= healthBefore.MetricsEmitted {
        t.Error("Expected metrics to be emitted")
    }
    
    // Check for specific metrics
    // In production, you'd use a test exporter to verify exact metrics
}
```

## 🧠 How It Works

### The Three-Layer Architecture

```
┌─────────────────────────────────────────┐
│         Simple API Layer                 │  ← What developers use
│    Emit(), Counter(), Histogram()        │
├─────────────────────────────────────────┤
│        Smart Registry Layer              │  ← Manages lifecycle
│   Thread-safe, Cardinality limits        │
├─────────────────────────────────────────┤
│     OpenTelemetry Provider Layer         │  ← Does the heavy lifting
│    HTTP export to collectors             │
└─────────────────────────────────────────┘
```

### Progressive Disclosure - Start Simple, Go Deep When Needed

#### Level 1: Just Emit Metrics (90% of use cases)
```go
// Super simple - just emit and go
telemetry.Counter("api.requests", "endpoint", "/users")
telemetry.Histogram("api.latency", 45.2, "endpoint", "/users")
telemetry.Gauge("queue.size", 150, "queue", "orders")
```

#### Level 2: Context Propagation (For distributed systems)
```go
// Add context for distributed tracing
ctx := telemetry.WithBaggage(ctx, 
    "request_id", "abc123",
    "user_id", "user456")

// Context automatically flows with the metric
telemetry.EmitWithContext(ctx, "payment.processed", 99.99)
```

#### Level 3: Advanced Features (When you need control)
```go
// Custom configuration
config := telemetry.Config{
    ServiceName:      "payment-service",
    Endpoint:         "localhost:4318",  // HTTP endpoint
    CardinalityLimit: 5000,
    CircuitBreaker: telemetry.CircuitConfig{
        Enabled:      true,
        MaxFailures:  5,
        RecoveryTime: 30 * time.Second,
    },
}
telemetry.Initialize(config)

// Declare metrics upfront for validation
telemetry.DeclareMetrics("payment-service", telemetry.ModuleConfig{
    Metrics: []telemetry.MetricDefinition{
        {
            Name:   "payment.amount",
            Type:   "histogram",
            Help:   "Payment amounts in USD",
            Labels: []string{"method", "currency"},
            Unit:   "dollars",
        },
    },
})
```

## 📊 Metric Types Explained

### Counter - Things that only go up
```go
// Perfect for: request counts, error counts, bytes processed
telemetry.Counter("files.processed", "type", "pdf")
telemetry.Counter("errors.total", "service", "auth")
```

### Histogram - Distribution of values
```go
// Perfect for: latencies, sizes, amounts
telemetry.Histogram("response.time_ms", 123.5, "endpoint", "/api/users")
telemetry.Histogram("file.size_mb", 2.4, "type", "image")
```

### Gauge - Values that go up and down
```go
// Perfect for: active connections, queue sizes, memory usage
telemetry.Gauge("connections.active", 42, "pool", "database")
telemetry.Gauge("memory.heap_mb", 256, "service", "api")
```

## 🔄 Context Propagation (Distributed Tracing)

### Following Requests Across Services

```go
// Service A: Start the journey
ctx := telemetry.WithBaggage(context.Background(),
    "trace_id", "xyz789",
    "user_tier", "premium")

// Make a call to Service B (context flows automatically)
serviceB.Process(ctx)

// Service B: Continue the journey
func (b *ServiceB) Process(ctx context.Context) {
    // Extract context to see the journey
    baggage := telemetry.GetBaggage(ctx)
    
    // Add more context
    ctx = telemetry.WithBaggage(ctx, 
        "payment_method", "credit_card")
    
    // Emit metrics with full context
    telemetry.EmitWithContext(ctx, "payment.processed", 99.99)
    
    // Context flows to Service C
    serviceC.Finalize(ctx)
}
```

### Baggage Limits (Prevent runaway metadata)
```go
// Automatic protection against context explosion
stats := telemetry.GetBaggageStats()
fmt.Printf("Items: %d, Dropped: %d\n", stats.ItemsAdded, stats.ItemsDropped)
```

## 🚦 Configuration Profiles

### Development - Fast feedback, verbose logging
```go
telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileDevelopment))
// ✓ Full sampling (100%)
// ✓ No circuit breaker (fail fast in dev)
// ✓ High cardinality limit (50,000)
// ✓ No PII redaction
```

### Staging - Production-like with safety nets
```go
telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileStaging))
// ✓ 10% sampling rate
// ✓ Circuit breaker enabled (10 failures, 15s recovery)
// ✓ Medium cardinality limit (20,000)
// ✓ PII redaction enabled
```

### Production - Battle-hardened settings
```go
telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileProduction))
// ✓ 0.1% sampling rate (reduce costs)
// ✓ Strict cardinality limits (10,000)
// ✓ Circuit breaker (10 failures, 30s recovery)
// ✓ PII redaction enabled
// ✓ Per-label cardinality limits
```

### Custom Configuration
```go
config := telemetry.Config{
    ServiceName:      "my-service",
    Endpoint:         "otel-collector.prod:4318",  // HTTP endpoint
    CardinalityLimit: 10000,
    SamplingRate:     0.25,  // Sample 25% of traces
    CircuitBreaker: telemetry.CircuitConfig{
        Enabled:      true,
        MaxFailures:  10,
        RecoveryTime: 30 * time.Second,
    },
}
telemetry.Initialize(config)
```

## 🛡️ Production Safety Features

### 1. Circuit Breaker - Protect against backend failures
```go
// Automatically stops sending metrics if backend is down
// Prevents cascading failures
config.CircuitBreaker = telemetry.CircuitConfig{
    Enabled:      true,
    MaxFailures:  5,                // 5 failures trigger open circuit
    RecoveryTime: 30 * time.Second, // Try again after 30s
}
```

### 2. Cardinality Limiter - Prevent metric explosion
```go
// Automatically limits unique label combinations
// Prevents memory/cost explosion
config.CardinalityLimit = 5000  // Max 5000 unique combinations per metric

// Per-label limits for fine control
config.CardinalityLimits = map[string]int{
    "user_id":    100,
    "agent_id":   100,
    "error_type": 50,
}
```

### 3. Thread-Safe Global Registry
```go
// Safe to call from any goroutine
go func() { telemetry.Emit("goroutine.spawned", 1.0) }()
go func() { telemetry.Emit("goroutine.spawned", 1.0) }()
// No race conditions!
```

## 📈 Health Monitoring

### Check telemetry health
```go
health := telemetry.GetHealth()
fmt.Printf("Telemetry Status:\n")
fmt.Printf("  Initialized: %v\n", health.Initialized)
fmt.Printf("  Provider: %s\n", health.Provider)
fmt.Printf("  Metrics Emitted: %d\n", health.MetricsEmitted)
fmt.Printf("  Errors: %d\n", health.Errors)
fmt.Printf("  Circuit Breaker: %s\n", health.CircuitBreakerStatus)
```

### HTTP Health Endpoint
```go
http.HandleFunc("/health/telemetry", telemetry.HealthHandler)
// Returns: {"initialized":true,"provider":"otel","metrics_emitted":1234,...}
```

## 🔌 Backend Integration

### OpenTelemetry Collector Setup
```yaml
# docker-compose.yml
services:
  otel-collector:
    image: otel/opentelemetry-collector:latest
    ports:
      - "4318:4318"  # HTTP endpoint
    volumes:
      - ./otel-config.yaml:/etc/otel-config.yaml
    command: ["--config", "/etc/otel-config.yaml"]
```

### Export to Multiple Backends
```yaml
# otel-config.yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318

exporters:
  prometheus:
    endpoint: "0.0.0.0:9090"
  jaeger:
    endpoint: "jaeger:14250"

service:
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [prometheus]
    traces:
      receivers: [otlp]
      exporters: [jaeger]
```

## 🚀 Common Patterns

### Pattern 1: Request Middleware
```go
func MetricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Track request
        telemetry.Counter("http.requests", 
            "method", r.Method,
            "path", r.URL.Path)
        
        // Call next handler
        next.ServeHTTP(w, r)
        
        // Track latency
        telemetry.Histogram("http.latency_ms", 
            float64(time.Since(start).Milliseconds()),
            "method", r.Method,
            "path", r.URL.Path)
    })
}
```

### Pattern 2: Business Metrics
```go
func ProcessPayment(ctx context.Context, amount float64) error {
    // Track business metric
    telemetry.EmitWithContext(ctx, "payment.amount", amount,
        "currency", "USD",
        "method", "credit_card")
    
    // Process payment...
    
    telemetry.Counter("payment.success", "method", "credit_card")
    return nil
}
```

### Pattern 3: Background Job Monitoring
```go
func RunJob(ctx context.Context) {
    telemetry.Counter("job.started", "type", "data_sync")
    defer telemetry.Counter("job.completed", "type", "data_sync")
    
    // Track active jobs
    telemetry.Gauge("jobs.active", 1, "type", "data_sync")
    defer telemetry.Gauge("jobs.active", -1, "type", "data_sync")
    
    // Job logic...
}
```

## ❓ FAQ for Junior Developers

### Q: Where do I initialize telemetry?
**A:** Always initialize telemetry ONCE in your `main()` function, before creating any agents:

```go
func main() {
    // ✅ Initialize FIRST
    telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileDevelopment))
    defer telemetry.Shutdown(context.Background())
    
    // Then create tools and agents
    tool := NewMyTool()
    agent := NewMyAgent()
}
```

### Q: Do I need to pass telemetry to each tool and agent?
**A:** No! Once initialized, telemetry is globally available. All tools and agents can call `telemetry.Emit()` directly:

```go
// Tools can use it
func (t *MyTool) Process() {
    telemetry.Counter("tool.processed")  // Works automatically!
}

// Agents can use it
func (a *MyAgent) Orchestrate() {
    telemetry.Counter("agent.orchestrated")  // Works automatically!
}
```

### Q: What's the difference between Emit, Counter, Histogram, and Gauge?
**A:** Think of them like this:

```go
// Counter - Counts things (always goes up)
telemetry.Counter("user.logins")  // +1 each time

// Histogram - Measures distributions (like response times)
telemetry.Histogram("api.latency_ms", 125.5)  // Records the value

// Gauge - Current value (can go up or down)
telemetry.Gauge("queue.size", 42)  // Current queue size

// Emit - General purpose (framework decides the type)
telemetry.Emit("custom.metric", 3.14)
```

### Q: How do I track errors in tools vs agents?
**A:** Use counters with component type labels:

```go
// In a Tool
if err != nil {
    telemetry.Counter("errors", 
        "component_type", "tool",
        "component_name", "calculator",
        "error_type", "invalid_input")
    return err
}

// In an Agent
if err != nil {
    telemetry.Counter("errors",
        "component_type", "agent",
        "component_name", "orchestrator",
        "error_type", "discovery_failed")
    return err
}
```

### Q: What labels should I use?
**A:** Keep labels low-cardinality (few unique values):

```go
// ✅ GOOD - Limited values
telemetry.Counter("api.requests", 
    "method", "GET",        // ~5 values (GET, POST, etc.)
    "status", "success")    // ~3 values (success, error, timeout)

// ❌ BAD - Unlimited values
telemetry.Counter("api.requests",
    "user_id", userID,      // Millions of values!
    "timestamp", time.Now()) // Infinite values!
```

### Q: How do I test if my metrics are working?
**A:** Check the telemetry health:

```go
health := telemetry.GetHealth()
fmt.Printf("Metrics emitted: %d\n", health.MetricsEmitted)
fmt.Printf("Errors: %d\n", health.Errors)
```

### Q: What's context propagation and when do I need it?
**A:** Context propagation carries metadata across service calls. Use it for distributed tracing:

```go
// Service A: Start the trace
ctx = telemetry.WithBaggage(ctx, "request_id", "abc123")

// Call Service B - context flows automatically
serviceB.Process(ctx)  

// Service B: Can see the request_id
baggage := telemetry.GetBaggage(ctx)
fmt.Println(baggage["request_id"])  // "abc123"
```

### Q: What if telemetry fails? Will my app crash?
**A:** No! Telemetry failures are silent and non-blocking:

```go
// Even if telemetry backend is down, your app continues
telemetry.Emit("metric", 1.0)  // Won't crash even if backend is down
```

### Q: How often are metrics sent to the backend?
**A:** The OpenTelemetry SDK handles batching and export intervals automatically:
- Metrics are batched and sent periodically (typically every 30 seconds)
- You can't configure the export interval directly in this module
- The SDK optimizes for efficiency and reduces network overhead

### Q: Can I use this without Docker/Kubernetes?
**A:** Yes! For local development, just run the OpenTelemetry Collector:

```bash
# Download and run collector locally
docker run -p 4318:4318 otel/opentelemetry-collector:latest

# Your app will send metrics to localhost:4318
```

## 🎯 Best Practices

### 1. Keep Labels Low-Cardinality
```go
// ❌ Bad: High cardinality
telemetry.Emit("api.request", 1.0, "user_id", userID)  // Millions of values!

// ✅ Good: Low cardinality  
telemetry.Emit("api.request", 1.0, "user_tier", "premium")  // Few values
```

### 2. Use Consistent Naming
```go
// ✅ Good: Consistent patterns
telemetry.Counter("http.requests.total")
telemetry.Histogram("http.request.duration_ms")
telemetry.Gauge("http.connections.active")

// ❌ Bad: Inconsistent
telemetry.Counter("RequestCount")
telemetry.Histogram("http_request_time")
telemetry.Gauge("active-connections")
```

### 3. Initialize Once, Use Everywhere
```go
// main.go
func main() {
    telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileProduction))
    defer telemetry.Shutdown(context.Background())
    
    // Now any package can emit metrics
    server.Start()  // server package can call telemetry.Emit()
}
```

### 4. Handle Shutdown Gracefully
```go
// Always shutdown to flush pending metrics
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    telemetry.Shutdown(ctx)
}()
```

## API Reference

### Core Functions

| Function | Description | Example |
|----------|-------------|------|
| `Initialize(config)` | Initialize telemetry system | `telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileDevelopment))` |
| `Emit(name, value, labels...)` | Emit a metric with labels | `telemetry.Emit("api.requests", 1.0, "method", "GET")` |
| `EmitWithContext(ctx, name, value, labels...)` | Emit metric with context | `telemetry.EmitWithContext(ctx, "payment", 99.99)` |
| `Counter(name, labels...)` | Increment a counter | `telemetry.Counter("errors", "type", "timeout")` |
| `Histogram(name, value, labels...)` | Record a value distribution | `telemetry.Histogram("latency_ms", 123.5)` |
| `Gauge(name, value, labels...)` | Set a gauge value | `telemetry.Gauge("queue.size", 42)` |
| `WithBaggage(ctx, labels...)` | Add context propagation | `telemetry.WithBaggage(ctx, "user_id", "123")` |
| `GetBaggage(ctx)` | Extract baggage from context | `baggage := telemetry.GetBaggage(ctx)` |
| `Shutdown(ctx)` | Gracefully shutdown telemetry | `telemetry.Shutdown(context.Background())` |

## 🔧 Troubleshooting

### Metrics not showing up?
```go
// 1. Check initialization
health := telemetry.GetHealth()
if !health.Initialized {
    log.Fatal("Telemetry not initialized!")
}

// 2. Check for errors
if health.Errors > 0 {
    log.Printf("Telemetry errors: %d", health.Errors)
}

// 3. Check circuit breaker
if health.CircuitBreakerStatus == "open" {
    log.Println("Circuit breaker is open - backend might be down")
}
```

### High memory usage?
```go
// Check metrics emitted
internal := telemetry.GetInternalMetrics()
if internal.Emitted > 1000000 {
    log.Printf("High metric volume: %d emitted, %d dropped\n", 
        internal.Emitted, internal.Dropped)
}

// Solution: Reduce label cardinality or increase limits
config.CardinalityLimit = 20000

// Or use per-label limits
config.CardinalityLimits = map[string]int{
    "user_id": 100,  // Limit unique user_id values
}
```

### Debugging context propagation?
```go
// See what's in the context
ctx = telemetry.WithBaggage(ctx, "debug", "true")
baggage := telemetry.GetBaggage(ctx)
for k, v := range baggage {
    fmt.Printf("Baggage: %s=%s\n", k, v)
}
```

## 📦 What's Included

- ✅ **OpenTelemetry Integration** - Industry standard observability
- ✅ **HTTP/OTLP Export** - Efficient, lightweight protocol
- ✅ **W3C Baggage Propagation** - Standard context propagation
- ✅ **Circuit Breaker** - Protect against backend failures
- ✅ **Cardinality Limiting** - Prevent metric explosion
- ✅ **Thread-Safe Operations** - Safe concurrent access
- ✅ **Zero Dependencies** - Only standard Go + OpenTelemetry
- ✅ **Progressive Disclosure** - Simple API with advanced options
- ✅ **Production Profiles** - Battle-tested configurations

## 🎓 Learn More

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [W3C Baggage Specification](https://www.w3.org/TR/baggage/)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/naming/)
- [Distributed Tracing Guide](https://opentelemetry.io/docs/concepts/observability-primer/#distributed-tracing)

## 💡 Pro Tips

1. **Start with profiles** - Use `ProfileDevelopment` locally, `ProfileProduction` in prod
2. **Emit early, emit often** - Better to have metrics than to wish you had them
3. **Keep labels consistent** - Use the same label names across metrics
4. **Monitor the monitor** - Use health endpoints to monitor telemetry itself
5. **Test with failures** - Verify circuit breaker behavior before production

Remember: Good telemetry is like insurance - you hope you never need it, but when you do, you're glad it's there!

## 🎯 Summary

The GoMind telemetry module provides:
- **Simple API** - Start with one-line metric emission
- **Progressive Disclosure** - Advanced features when you need them
- **Production Safety** - Circuit breakers, cardinality limits, thread-safety
- **Standard Compliance** - OpenTelemetry and W3C baggage standards
- **Multiple Backends** - Export to Prometheus, Jaeger, or any OTLP collector

Get started in seconds, scale to millions of metrics.