# Agent with Orchestration - Implementation Plan

## Overview

This document outlines the implementation plan for `agent-with-orchestration`, which demonstrates the GoMind `orchestration` module through a **Smart Travel Research Assistant** use case.

**Port:** 8094
**Status:** Planning
**Base:** Builds on agent-example (NOT on previous examples)

---

## Use Case: Smart Travel Research Assistant

A real-world travel planning assistant that coordinates multiple tools to research a destination. When a user asks *"Plan a trip to Tokyo in December"*, the agent will:

1. **Geocode** the destination to get coordinates
2. **Fetch weather** for the location (depends on geocoding)
3. **Get currency exchange** rates (parallel with geocoding)
4. **Fetch country info** (parallel)
5. **Get relevant news** about the destination (parallel)
6. **AI Synthesize** all results into a cohesive travel report

### Why This Use Case?

This demonstrates all key orchestration features:

| Feature | How It's Demonstrated |
|---------|----------------------|
| **Parallel execution** | Currency, country-info, news run simultaneously |
| **Sequential dependencies** | Weather depends on geocoding for coordinates |
| **DAG-based workflows** | Explicit dependency graph with validation |
| **Multi-agent coordination** | 5 specialized tools working together |
| **AI synthesis** | LLM combines results into cohesive report |
| **Graceful degradation** | Partial results if some tools fail |

---

## Free APIs Selected

All APIs are free, require no credit card, and provide real data:

| Tool | API Provider | Free Tier | Auth Required | Documentation |
|------|--------------|-----------|---------------|---------------|
| **Geocoding** | [Nominatim/OpenStreetMap](https://nominatim.org/release-docs/latest/api/Overview/) | Unlimited (1 req/sec) | No | [Docs](https://nominatim.org/release-docs/latest/api/Search/) |
| **Weather** | [Open-Meteo](https://open-meteo.com/) | Unlimited | No | [Docs](https://open-meteo.com/en/docs) |
| **Currency** | [Frankfurter](https://frankfurter.dev/) | Unlimited | No | [Docs](https://frankfurter.dev/docs) |
| **Country Info** | [RestCountries](https://restcountries.com/) | Unlimited | No | [Docs](https://restcountries.com/#endpoints-all) |
| **News** | [GNews.io](https://gnews.io/) | 100 req/day (dev) | Free API key | [Docs](https://gnews.io/docs/v4) |

### API Selection Rationale

- **No credit card required** - All free tiers work without payment info
- **No complex auth** - Simple API keys or no auth at all
- **Real data** - No mock data, actual live information
- **Reliable** - Well-documented, stable services
- **Good rate limits** - Sufficient for development and demos

---

## Workflow DAG Visualization

```
                    ┌─────────────────┐
                    │   User Request  │
                    │ "Trip to Tokyo" │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┬──────────────┐
              │              │              │              │
              ▼              ▼              ▼              ▼
    ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
    │  Geocoding  │  │  Currency   │  │ Country Info│  │    News     │
    │   (step-1)  │  │  (step-2)   │  │  (step-3)   │  │  (step-4)   │
    │ [Nominatim] │  │[Frankfurter]│  │[RestCountries] │[GNews.io]  │
    └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘
           │                │                │                │
           ▼                │                │                │
    ┌─────────────┐         │                │                │
    │   Weather   │         │                │                │
    │  (step-5)   │         │                │                │
    │ [Open-Meteo]│         │                │                │
    │ depends on: │         │                │                │
    │   step-1    │         │                │                │
    └──────┬──────┘         │                │                │
           │                │                │                │
           └────────────────┴────────────────┴────────────────┘
                                    │
                                    ▼
                    ┌─────────────────────────────────────────┐
                    │              AI Synthesizer             │
                    │     Combines all results into report    │
                    └─────────────────────────────────────────┘
```

**Execution Levels (from DAG):**
- **Level 1**: geocoding, currency, country-info, news (4 parallel tasks)
- **Level 2**: weather (depends on geocoding for lat/lon)
- **Level 3**: AI synthesis (combines all results)

---

## Project Structure

```
examples/
├── agent-with-orchestration/          # The orchestration agent (Port 8094)
│   ├── main.go                        # Entry point with orchestrator setup
│   ├── research_agent.go              # Agent with orchestration capabilities
│   ├── handlers.go                    # HTTP handlers
│   ├── workflows.go                   # Predefined workflow definitions
│   ├── go.mod / go.sum
│   ├── .env.example
│   ├── Dockerfile
│   ├── k8-deployment.yaml
│   ├── setup.sh
│   └── README.md
│
├── geocoding-tool/                    # Tool 1: Location lookup (Port 8095)
│   ├── main.go
│   ├── geocoding_tool.go
│   ├── handlers.go
│   ├── go.mod
│   ├── Dockerfile
│   ├── k8-deployment.yaml
│   └── README.md
│
├── weather-tool-v2/                   # Tool 2: Weather forecast (Port 8096)
│   ├── main.go                        # Uses Open-Meteo API
│   ├── weather_tool.go
│   ├── handlers.go
│   ├── go.mod
│   ├── Dockerfile
│   ├── k8-deployment.yaml
│   └── README.md
│
├── currency-tool/                     # Tool 3: Currency exchange (Port 8097)
│   ├── main.go
│   ├── currency_tool.go
│   ├── handlers.go
│   ├── go.mod
│   ├── Dockerfile
│   ├── k8-deployment.yaml
│   └── README.md
│
├── country-info-tool/                 # Tool 4: Country information (Port 8098)
│   ├── main.go
│   ├── country_tool.go
│   ├── handlers.go
│   ├── go.mod
│   ├── Dockerfile
│   ├── k8-deployment.yaml
│   └── README.md
│
└── news-tool/                         # Tool 5: News headlines (Port 8099)
    ├── main.go
    ├── news_tool.go
    ├── handlers.go
    ├── go.mod
    ├── Dockerfile
    ├── k8-deployment.yaml
    └── README.md
```

---

## Tool Specifications

### Tool 1: Geocoding Tool

**API**: Nominatim (OpenStreetMap)
**Endpoint**: `https://nominatim.openstreetmap.org/search`
**Rate Limit**: 1 request/second (must respect this)

```go
// Capability: geocode_location
// Input
type GeocodeRequest struct {
    Location string `json:"location"` // e.g., "Tokyo, Japan"
}

// Output
type GeocodeResponse struct {
    Latitude    float64 `json:"lat"`
    Longitude   float64 `json:"lon"`
    DisplayName string  `json:"display_name"`
    CountryCode string  `json:"country_code"`
    Country     string  `json:"country"`
}
```

**Example API Call:**
```bash
curl "https://nominatim.openstreetmap.org/search?q=Tokyo&format=json&limit=1"
```

### Tool 2: Weather Tool v2

**API**: Open-Meteo
**Endpoint**: `https://api.open-meteo.com/v1/forecast`
**Rate Limit**: Unlimited (non-commercial)

```go
// Capability: get_weather_forecast
// Input
type WeatherRequest struct {
    Latitude  float64 `json:"lat"`
    Longitude float64 `json:"lon"`
    Days      int     `json:"days"` // 1-16
}

// Output
type WeatherResponse struct {
    Location    string          `json:"location"`
    Temperature float64         `json:"temperature_avg"`
    TempMin     float64         `json:"temperature_min"`
    TempMax     float64         `json:"temperature_max"`
    Condition   string          `json:"condition"`
    Humidity    int             `json:"humidity"`
    Forecast    []DailyForecast `json:"forecast"`
}
```

**Example API Call:**
```bash
curl "https://api.open-meteo.com/v1/forecast?latitude=35.6762&longitude=139.6503&daily=temperature_2m_max,temperature_2m_min,weathercode&timezone=auto"
```

### Tool 3: Currency Tool

**API**: Frankfurter
**Endpoint**: `https://api.frankfurter.dev/latest`
**Rate Limit**: Unlimited

```go
// Capability: convert_currency
// Input
type CurrencyRequest struct {
    From   string  `json:"from"`   // e.g., "USD"
    To     string  `json:"to"`     // e.g., "JPY"
    Amount float64 `json:"amount"` // e.g., 1000
}

// Output
type CurrencyResponse struct {
    From      string  `json:"from"`
    To        string  `json:"to"`
    Amount    float64 `json:"amount"`
    Result    float64 `json:"result"`
    Rate      float64 `json:"rate"`
    Date      string  `json:"date"`
}
```

**Example API Call:**
```bash
curl "https://api.frankfurter.dev/latest?from=USD&to=JPY&amount=1000"
```

### Tool 4: Country Info Tool

**API**: RestCountries
**Endpoint**: `https://restcountries.com/v3.1/name/{country}`
**Rate Limit**: Unlimited

```go
// Capability: get_country_info
// Input
type CountryRequest struct {
    Country string `json:"country"` // e.g., "Japan"
}

// Output
type CountryResponse struct {
    Name       string   `json:"name"`
    Capital    string   `json:"capital"`
    Region     string   `json:"region"`
    Population int64    `json:"population"`
    Languages  []string `json:"languages"`
    Timezones  []string `json:"timezones"`
    Currency   struct {
        Code   string `json:"code"`
        Name   string `json:"name"`
        Symbol string `json:"symbol"`
    } `json:"currency"`
    Flag       string `json:"flag"` // Emoji flag
}
```

**Example API Call:**
```bash
curl "https://restcountries.com/v3.1/name/japan?fields=name,capital,region,population,languages,timezones,currencies,flag"
```

### Tool 5: News Tool

**API**: GNews.io
**Endpoint**: `https://gnews.io/api/v4/search`
**Rate Limit**: 100 requests/day (free tier)
**Auth**: API key required (free signup)

```go
// Capability: search_news
// Input
type NewsRequest struct {
    Query      string `json:"query"`       // e.g., "Tokyo travel"
    MaxResults int    `json:"max_results"` // 1-10
    Language   string `json:"language"`    // e.g., "en"
}

// Output
type NewsResponse struct {
    TotalArticles int       `json:"total_articles"`
    Articles      []Article `json:"articles"`
}

type Article struct {
    Title       string `json:"title"`
    Description string `json:"description"`
    URL         string `json:"url"`
    Source      string `json:"source"`
    PublishedAt string `json:"published_at"`
}
```

**Example API Call:**
```bash
curl "https://gnews.io/api/v4/search?q=Tokyo+travel&lang=en&max=5&apikey=YOUR_API_KEY"
```

---

## Framework APIs Demonstrated

This example showcases the `orchestration` module's key features:

| API | Purpose | File |
|-----|---------|------|
| `orchestration.NewAIOrchestrator()` | Create AI-powered orchestrator | `main.go` |
| `orchestration.DefaultConfig()` | Production-ready defaults | `main.go` |
| `orchestrator.ProcessRequest()` | Handle natural language requests | `handlers.go` |
| `orchestrator.ExecutePlan()` | Execute predefined workflows | `handlers.go` |
| `orchestration.NewWorkflowDAG()` | Build execution graphs | `workflows.go` |
| `dag.AddNode()` | Add nodes with dependencies | `workflows.go` |
| `dag.Validate()` | Check for cycles | `workflows.go` |
| `dag.GetExecutionLevels()` | Get parallel execution groups | `workflows.go` |
| `dag.GetReadyNodes()` | Get nodes ready to execute | `workflows.go` |
| `orchestrator.SetTelemetry()` | Enable observability | `main.go` |
| `orchestrator.GetMetrics()` | Get orchestration metrics | `handlers.go` |
| `orchestrator.GetExecutionHistory()` | Get execution history | `handlers.go` |

### Distributed Tracing APIs

This example also demonstrates the `telemetry` module's distributed tracing features:

| API | Purpose | File |
|-----|---------|------|
| `telemetry.Initialize()` | Set up OpenTelemetry with propagators | `main.go` |
| `telemetry.TracingMiddleware()` | Server-side trace context extraction | `main.go` |
| `telemetry.TracingMiddlewareWithConfig()` | Middleware with excluded paths | `main.go` |
| `telemetry.NewTracedHTTPClient()` | Client with trace propagation | All tools |

**Why Distributed Tracing Matters for Orchestration:**

When the orchestrator calls multiple tools in parallel, distributed tracing:
- Creates a parent span for the entire workflow
- Links child spans for each tool invocation
- Shows parallel execution timing in trace visualizations (e.g., Jaeger)
- Propagates trace context via W3C TraceContext headers (`traceparent`, `tracestate`)

---

## API Endpoints

### POST /api/orchestrate/travel-research

Main orchestrated travel research endpoint.

```bash
curl -X POST http://localhost:8094/api/orchestrate/travel-research \
  -H "Content-Type: application/json" \
  -d '{
    "destination": "Tokyo",
    "travel_month": "December",
    "home_currency": "USD",
    "budget": 5000
  }'
```

**Response:**
```json
{
  "request_id": "req-1234567890",
  "destination": "Tokyo",
  "execution_time": "2.5s",
  "workflow_stats": {
    "total_steps": 5,
    "completed": 5,
    "failed": 0,
    "parallel_levels": 2,
    "max_parallelism": 4
  },
  "results": {
    "location": {
      "lat": 35.6762,
      "lon": 139.6503,
      "country": "Japan",
      "country_code": "jp"
    },
    "weather": {
      "temperature_avg": 10.5,
      "temperature_min": 5,
      "temperature_max": 15,
      "condition": "Partly Cloudy",
      "humidity": 55
    },
    "currency": {
      "from": "USD",
      "to": "JPY",
      "rate": 149.5,
      "budget_converted": 747500
    },
    "country": {
      "capital": "Tokyo",
      "population": 125836021,
      "languages": ["Japanese"],
      "timezone": "UTC+09:00"
    },
    "news": [
      {
        "title": "Best Time to Visit Tokyo",
        "source": "Travel Weekly",
        "url": "https://..."
      }
    ]
  },
  "ai_summary": "## Tokyo Travel Research\n\n**Weather in December**: Expect cool, dry weather with temperatures between 5-15°C (41-59°F). Pack layers and a warm jacket.\n\n**Budget**: Your $5,000 USD converts to approximately ¥747,500 JPY...",
  "agents_involved": [
    "geocoding-tool",
    "weather-tool-v2",
    "currency-tool",
    "country-info-tool",
    "news-tool"
  ]
}
```

### POST /api/orchestrate/natural

Process natural language requests with AI-powered routing.

```bash
curl -X POST http://localhost:8094/api/orchestrate/natural \
  -H "Content-Type: application/json" \
  -d '{
    "request": "I want to visit Paris next summer. What should I know about the weather, currency from GBP, and any recent news?"
  }'
```

### POST /api/orchestrate/custom

Execute custom workflows with explicit DAG definition.

```bash
curl -X POST http://localhost:8094/api/orchestrate/custom \
  -H "Content-Type: application/json" \
  -d '{
    "request": "Compare weather in Tokyo and New York",
    "workflow": {
      "steps": [
        {"id": "geo-tokyo", "tool": "geocoding-tool", "capability": "geocode_location", "params": {"location": "Tokyo"}},
        {"id": "geo-nyc", "tool": "geocoding-tool", "capability": "geocode_location", "params": {"location": "New York"}},
        {"id": "weather-tokyo", "tool": "weather-tool-v2", "capability": "get_weather_forecast", "depends_on": ["geo-tokyo"]},
        {"id": "weather-nyc", "tool": "weather-tool-v2", "capability": "get_weather_forecast", "depends_on": ["geo-nyc"]}
      ]
    }
  }'
```

### GET /api/orchestrate/workflows

List available predefined workflows.

```bash
curl http://localhost:8094/api/orchestrate/workflows
```

### GET /api/orchestrate/history

Get execution history with DAG statistics.

```bash
curl http://localhost:8094/api/orchestrate/history?limit=10
```

### GET /health

Health check with orchestrator metrics.

```bash
curl http://localhost:8094/health
```

**Response:**
```json
{
  "status": "healthy",
  "orchestrator": {
    "total_requests": 150,
    "successful_requests": 145,
    "failed_requests": 5,
    "average_latency_ms": 2500,
    "agents_available": 5
  },
  "tools": {
    "geocoding-tool": "healthy",
    "weather-tool-v2": "healthy",
    "currency-tool": "healthy",
    "country-info-tool": "healthy",
    "news-tool": "healthy"
  }
}
```

---

## Key Code Patterns

### 1. Orchestrator Initialization with Distributed Tracing

```go
// main.go
import (
    "github.com/itsneelabh/gomind/orchestration"
    "github.com/itsneelabh/gomind/ai"
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

func main() {
    ctx := context.Background()

    // Initialize telemetry (MUST be first - sets up trace propagators)
    shutdown, err := telemetry.Initialize(ctx, telemetry.Config{
        ServiceName:    "travel-research-agent",
        ServiceVersion: "1.0.0",
        OTLPEndpoint:   os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"), // e.g., "otel-collector:4317"
    })
    if err != nil {
        log.Printf("Warning: telemetry init failed: %v", err)
    } else {
        defer shutdown(ctx)
    }

    // Create discovery client
    discovery, _ := core.NewRedisDiscovery(os.Getenv("REDIS_URL"))

    // Create AI client for synthesis
    aiClient, _ := ai.NewClient()

    // Configure orchestrator
    config := orchestration.DefaultConfig()
    config.ExecutionOptions.MaxConcurrency = 5      // Run up to 5 tools in parallel
    config.ExecutionOptions.StepTimeout = 10 * time.Second
    config.SynthesisStrategy = orchestration.StrategyLLM

    // Create orchestrator
    orch := orchestration.NewAIOrchestrator(config, discovery, aiClient)

    // Start background processes (catalog refresh, etc.)
    if err := orch.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer orch.Stop()

    // Create agent with traced HTTP client for tool calls
    httpClient := telemetry.NewTracedHTTPClient(nil)
    agent := NewTravelResearchAgent(orch, httpClient)

    // Set up HTTP server with tracing middleware
    mux := http.NewServeMux()
    mux.HandleFunc("/api/orchestrate/travel-research", agent.handleTravelResearch)
    mux.HandleFunc("/api/orchestrate/natural", agent.handleNaturalRequest)
    mux.HandleFunc("/health", agent.handleHealth)

    // Apply tracing middleware (excludes health/metrics from traces)
    tracingConfig := &telemetry.TracingMiddlewareConfig{
        ExcludedPaths: []string{"/health", "/metrics"},
    }
    traced := telemetry.TracingMiddlewareWithConfig("travel-research-agent", tracingConfig)(mux)

    log.Println("Starting Travel Research Agent on :8094")
    http.ListenAndServe(":8094", traced)
}
```

### 1b. Tool Initialization with Trace Propagation

Each tool follows the same pattern for receiving and propagating trace context:

```go
// Example: geocoding-tool/main.go
func main() {
    ctx := context.Background()

    // Initialize telemetry
    shutdown, err := telemetry.Initialize(ctx, telemetry.Config{
        ServiceName:    "geocoding-tool",
        ServiceVersion: "1.0.0",
        OTLPEndpoint:   os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
    })
    if err != nil {
        log.Printf("Warning: telemetry init failed: %v", err)
    } else {
        defer shutdown(ctx)
    }

    // Create traced HTTP client for external API calls (Nominatim)
    httpClient := telemetry.NewTracedHTTPClient(nil)
    tool := NewGeocodingTool(httpClient)

    // Set up HTTP server
    mux := http.NewServeMux()
    mux.HandleFunc("/api/capabilities/geocode_location", tool.handleGeocode)
    mux.HandleFunc("/health", tool.handleHealth)

    // Apply tracing middleware - extracts trace context from incoming requests
    traced := telemetry.TracingMiddleware("geocoding-tool")(mux)

    log.Println("Starting Geocoding Tool on :8095")
    http.ListenAndServe(":8095", traced)
}
```

**Trace Flow:**
```
orchestrator → geocoding-tool → Nominatim API
     │                │               │
     └── traceparent ─┴── traceparent ┴── (if supported)
```

When a request flows through:
1. **Orchestrator** creates a parent span for the workflow
2. **TracingMiddleware** on each tool extracts the trace context from headers
3. **NewTracedHTTPClient** injects trace context into outbound requests
4. All spans appear connected in Jaeger/visualization tools

### 2. Building Workflows with DAG

```go
// workflows.go
func BuildTravelResearchWorkflow(destination, currency string) (*orchestration.RoutingPlan, error) {
    dag := orchestration.NewWorkflowDAG()

    // Level 1: Parallel tasks (no dependencies)
    dag.AddNode("geocoding", []string{})      // No dependencies
    dag.AddNode("currency", []string{})       // No dependencies
    dag.AddNode("country-info", []string{})   // No dependencies
    dag.AddNode("news", []string{})           // No dependencies

    // Level 2: Depends on geocoding for lat/lon
    dag.AddNode("weather", []string{"geocoding"})

    // Validate DAG (check for cycles, missing dependencies)
    if err := dag.Validate(); err != nil {
        return nil, fmt.Errorf("invalid workflow: %w", err)
    }

    // Get execution levels for visualization
    levels := dag.GetExecutionLevels()
    // levels = [["geocoding", "currency", "country-info", "news"], ["weather"]]

    // Get DAG statistics
    stats := dag.GetStatistics()
    log.Printf("Workflow: %d nodes, %d levels, max parallelism: %d",
        stats.TotalNodes, stats.Depth, stats.MaxParallelism)

    // Build routing plan from DAG
    return &orchestration.RoutingPlan{
        PlanID:          fmt.Sprintf("travel-%s-%d", destination, time.Now().Unix()),
        OriginalRequest: fmt.Sprintf("Research trip to %s", destination),
        Mode:            orchestration.ModeWorkflow,
        Steps:           buildStepsFromDAG(dag, destination, currency),
        CreatedAt:       time.Now(),
    }, nil
}

func buildStepsFromDAG(dag *orchestration.WorkflowDAG, destination, currency string) []orchestration.RoutingStep {
    return []orchestration.RoutingStep{
        {
            StepID:      "geocoding",
            AgentName:   "geocoding-tool",
            Instruction: fmt.Sprintf("Get coordinates for %s", destination),
            DependsOn:   []string{},
            Metadata: map[string]interface{}{
                "capability": "geocode_location",
                "parameters": map[string]interface{}{
                    "location": destination,
                },
            },
        },
        {
            StepID:      "currency",
            AgentName:   "currency-tool",
            Instruction: fmt.Sprintf("Convert 1000 %s to local currency", currency),
            DependsOn:   []string{},
            Metadata: map[string]interface{}{
                "capability": "convert_currency",
                "parameters": map[string]interface{}{
                    "from":   currency,
                    "to":     "JPY", // Will be determined from geocoding in practice
                    "amount": 1000,
                },
            },
        },
        // ... more steps
        {
            StepID:      "weather",
            AgentName:   "weather-tool-v2",
            Instruction: "Get weather forecast using coordinates from geocoding",
            DependsOn:   []string{"geocoding"}, // KEY: Depends on geocoding
            Metadata: map[string]interface{}{
                "capability": "get_weather_forecast",
                "parameters": map[string]interface{}{
                    "days": 7,
                    // lat/lon will be injected from geocoding result
                },
            },
        },
    }
}
```

### 3. Executing Workflows

```go
// handlers.go
func (a *TravelResearchAgent) handleTravelResearch(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    var req TravelResearchRequest
    json.NewDecoder(r.Body).Decode(&req)

    // Build workflow DAG
    plan, err := BuildTravelResearchWorkflow(req.Destination, req.HomeCurrency)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Execute the plan (orchestrator handles parallelism)
    result, err := a.orchestrator.ExecutePlan(ctx, plan)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Synthesize results using AI
    summary, err := a.synthesizeResults(ctx, req, result)
    if err != nil {
        // Continue with partial results
        summary = "Unable to generate AI summary"
    }

    // Build response
    response := TravelResearchResponse{
        RequestID:      plan.PlanID,
        Destination:    req.Destination,
        ExecutionTime:  result.TotalDuration.String(),
        WorkflowStats:  extractWorkflowStats(result),
        Results:        extractResults(result),
        AISummary:      summary,
        AgentsInvolved: extractAgents(plan),
    }

    json.NewEncoder(w).Encode(response)
}
```

### 4. Natural Language Processing

```go
// handlers.go
func (a *TravelResearchAgent) handleNaturalRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    var req struct {
        Request string `json:"request"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    // Use AI orchestrator to process natural language
    // It will:
    // 1. Analyze the request
    // 2. Determine which tools to use
    // 3. Build execution plan with dependencies
    // 4. Execute in parallel where possible
    // 5. Synthesize results
    response, err := a.orchestrator.ProcessRequest(ctx, req.Request, nil)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(response)
}
```

### 5. DAG Monitoring

```go
// handlers.go
func (a *TravelResearchAgent) handleWorkflowStatus(w http.ResponseWriter, r *http.Request) {
    // Get execution history
    history := a.orchestrator.GetExecutionHistory()

    // Get orchestrator metrics
    metrics := a.orchestrator.GetMetrics()

    response := map[string]interface{}{
        "metrics": map[string]interface{}{
            "total_requests":      metrics.TotalRequests,
            "successful_requests": metrics.SuccessfulRequests,
            "failed_requests":     metrics.FailedRequests,
            "average_latency_ms":  metrics.AverageLatency.Milliseconds(),
            "p99_latency_ms":      metrics.P99Latency.Milliseconds(),
        },
        "recent_executions": history[:min(10, len(history))],
    }

    json.NewEncoder(w).Encode(response)
}
```

---

## Dynamic Request Handling (AI-Powered Orchestration)

This section explains how the orchestrator handles **natural language requests without predefined workflows**. This is the core intelligence that makes `ProcessRequest()` work.

### High-Level Flow

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                        ProcessRequest(request)                                │
└───────────────────────────────────┬──────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│  Step 1: GET CAPABILITIES (AgentCatalog)                                      │
│  ───────────────────────────────────────                                      │
│  • AgentCatalog refreshes from Redis Discovery every 10s                      │
│  • Discovers all registered tools (geocoding, weather, currency, etc.)        │
│  • Fetches /api/capabilities from each tool                                   │
│  • Builds capability index with parameters, descriptions, return types        │
│  • FormatForLLM() produces human-readable capability listing                  │
└───────────────────────────────────┬──────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│  Step 2: LLM GENERATES EXECUTION PLAN                                         │
│  ────────────────────────────────────                                         │
│  • buildPlanningPrompt() constructs prompt with all capabilities              │
│  • aiClient.GenerateResponse() asks LLM to create JSON execution plan         │
│  • LLM analyzes request and decides which tools to use                        │
│  • LLM determines dependencies between steps                                  │
│  • parsePlan() extracts JSON from LLM response                                │
└───────────────────────────────────┬──────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│  Step 3: VALIDATE PLAN                                                        │
│  ─────────────────────                                                        │
│  • validatePlan() checks each agent exists in discovery                       │
│  • Verifies each capability exists for that agent                             │
│  • Validates dependencies reference valid step_ids                            │
│  • If validation fails → regeneratePlan() with error feedback to LLM          │
└───────────────────────────────────┬──────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│  Step 4: EXECUTE PLAN (SmartExecutor)                                         │
│  ────────────────────────────────────                                         │
│  • executor.Execute(plan) runs the plan                                       │
│  • Executes steps by dependency levels (parallel where possible)              │
│  • Level 1: [geocoding, currency, country-info, news] → PARALLEL              │
│  • Level 2: [weather] → waits for geocoding lat/lon                           │
│  • Each step: HTTP POST to tool's capability endpoint                         │
│  • Injects outputs from dependent steps into parameters                       │
└───────────────────────────────────┬──────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│  Step 5: AI SYNTHESIS                                                         │
│  ────────────────────                                                         │
│  • synthesizer.Synthesize(request, results) combines all tool outputs         │
│  • LLM generates cohesive response from multiple tool results                 │
│  • Returns final OrchestratorResponse to caller                               │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Key Components

#### 1. AgentCatalog (`orchestration/catalog.go`)

The catalog maintains a local cache of all agents and their capabilities:

```go
type AgentCatalog struct {
    agents          map[string]*AgentInfo      // Agent ID → full info
    capabilityIndex map[string][]string        // Capability name → agent IDs
    discovery       core.Discovery             // Redis discovery client
}

// Refresh() - Called every 10 seconds in background
// 1. Queries Redis discovery for all registered services
// 2. Fetches /api/capabilities from each service
// 3. Builds capability index for fast lookup

// FormatForLLM() - Formats catalog for LLM consumption
// Returns human-readable text like:
// "Agent: geocoding-tool
//    - Capability: geocode_location
//      Parameters: location (string, required)
//      Returns: lat, lon, country_code"
```

#### 2. CapabilityProvider (`orchestration/capability_provider.go`)

Two implementations for different scales:

| Provider | Use Case | How It Works |
|----------|----------|--------------|
| `DefaultCapabilityProvider` | Quick start, small deployments | Sends ALL capabilities to LLM via `catalog.FormatForLLM()` |
| `ServiceCapabilityProvider` | Large-scale, 100+ tools | Queries external service for semantic search, returns top-K relevant capabilities |

#### 3. Plan Generation (`orchestration/orchestrator.go:322`)

The LLM receives a prompt like:

```
You are an AI orchestrator managing a multi-agent system.

Available Agents and Capabilities:
Agent: geocoding-tool (ID: geocoding-tool)
  - Capability: geocode_location
    Description: Get coordinates for a location
    Parameters: location (string, required)
    Returns: lat, lon, country_code

Agent: weather-tool-v2 (ID: weather-tool-v2)
  - Capability: get_weather_forecast
    Parameters: lat (float64, required), lon (float64, required), days (int)
    Returns: temperature, condition, humidity

[... more agents ...]

User Request: "What's the weather like in Tokyo and how much would $1000 be in yen?"

Create an execution plan in JSON format...
```

The LLM responds with a plan:

```json
{
  "plan_id": "request-12345",
  "steps": [
    {
      "step_id": "geo",
      "agent_name": "geocoding-tool",
      "metadata": {
        "capability": "geocode_location",
        "parameters": {"location": "Tokyo"}
      }
    },
    {
      "step_id": "weather",
      "agent_name": "weather-tool-v2",
      "depends_on": ["geo"],
      "metadata": {
        "capability": "get_weather_forecast",
        "parameters": {"days": 7}
      }
    },
    {
      "step_id": "currency",
      "agent_name": "currency-tool",
      "metadata": {
        "capability": "convert_currency",
        "parameters": {"from": "USD", "to": "JPY", "amount": 1000}
      }
    }
  ]
}
```

**Key Insight**: The LLM intelligently chose NOT to use `country-info-tool` or `news-tool` because they weren't relevant to this specific request!

#### 4. Self-Correcting Plans (`orchestration/orchestrator.go:553`)

If plan validation fails (e.g., LLM hallucinated a non-existent tool), the orchestrator asks the LLM to fix it:

```go
func (o *AIOrchestrator) regeneratePlan(ctx, request, requestID, validationErr) {
    prompt := basePrompt + fmt.Sprintf(
        "The previous plan failed validation with error: %s\n" +
        "Please generate a corrected plan that addresses this error.",
        validationErr.Error())

    // LLM sees the error and generates a corrected plan
    aiResponse := o.aiClient.GenerateResponse(ctx, prompt, opts)
    return o.parsePlan(aiResponse.Content)
}
```

### Example: Dynamic vs Predefined Workflows

| Aspect | Predefined Workflow (`ExecutePlan`) | Dynamic Request (`ProcessRequest`) |
|--------|-------------------------------------|-------------------------------------|
| Workflow definition | Hardcoded in `workflows.go` | LLM generates from request |
| Tool selection | Fixed set of tools | LLM chooses relevant tools |
| Dependencies | Explicit in code | LLM infers from context |
| Use case | Known, repeatable workflows | Arbitrary natural language |
| Example | "Travel Research" template | "What's the weather in Tokyo?" |

### Configuration Options

```go
config := orchestration.DefaultConfig()

// Routing mode (for metrics/logging)
config.RoutingMode = orchestration.ModeAutonomous  // AI-driven

// Execution settings
config.ExecutionOptions.MaxConcurrency = 5         // Max parallel tool calls
config.ExecutionOptions.StepTimeout = 30 * time.Second
config.ExecutionOptions.RetryAttempts = 2

// Synthesis strategy
config.SynthesisStrategy = orchestration.StrategyLLM  // Use LLM to combine results

// Capability provider (for large deployments)
config.CapabilityProviderType = "service"  // Use semantic search service
config.CapabilityService.Endpoint = "http://capability-service:8080"
config.CapabilityService.TopK = 20         // Return top 20 relevant capabilities
config.CapabilityService.Threshold = 0.7   // Minimum similarity score
```

---

## What Developers Learn

After completing this example, developers will understand:

1. **How to create AI-powered orchestrators** using `orchestration.NewAIOrchestrator()`
2. **How to build workflow DAGs** with dependencies using `WorkflowDAG`
3. **How to define execution dependencies** between steps
4. **How to execute parallel workflows** with automatic dependency resolution
5. **How to handle partial failures** in multi-step workflows
6. **How to synthesize results** from multiple agents using AI
7. **How to monitor orchestration** via metrics and execution history
8. **Production patterns** for multi-agent coordination
9. **How to implement distributed tracing** using `TracingMiddleware` and `NewTracedHTTPClient`
10. **How to visualize parallel workflows** in Jaeger with connected spans
11. **How dynamic request handling works** - LLM-based plan generation from natural language
12. **How the AgentCatalog discovers tools** - automatic capability discovery from Redis
13. **How plan validation and self-correction works** - LLM regenerates plans on validation errors
14. **How to scale with ServiceCapabilityProvider** - semantic search for large tool ecosystems

---

## Setup Commands

```bash
# One-click: Run everything locally (recommended)
./setup.sh run-all

# This will:
# 1. Start Redis (if not running)
# 2. Build and start all 5 tools
# 3. Build and start the orchestration agent
# 4. Register all tools with Redis discovery

# Individual commands
./setup.sh build-all      # Build all components
./setup.sh run            # Run agent only (tools must be running)
./setup.sh deploy         # Full Kubernetes deployment
./setup.sh forward        # Port-forward K8s services
./setup.sh demo           # Run demo workflow
./setup.sh test           # Run integration tests
./setup.sh cleanup        # Remove all resources
```

---

## Environment Variables

```bash
# Required
REDIS_URL=redis://localhost:6379
OPENAI_API_KEY=sk-...              # For AI synthesis

# Optional (for news tool - free signup at gnews.io)
GNEWS_API_KEY=...

# Distributed Tracing (optional but recommended)
OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317  # or localhost:4317 for local dev

# Service ports (defaults)
GEOCODING_TOOL_PORT=8095
WEATHER_TOOL_PORT=8096
CURRENCY_TOOL_PORT=8097
COUNTRY_INFO_TOOL_PORT=8098
NEWS_TOOL_PORT=8099
ORCHESTRATION_AGENT_PORT=8094
```

---

## Comparison: agent-example vs agent-with-orchestration

| Feature | agent-example | agent-with-orchestration |
|---------|---------------|--------------------------|
| Tool discovery | Yes | Yes |
| Sequential tool calls | Yes | Yes |
| **Parallel execution** | No | Yes |
| **DAG-based workflows** | No | Yes |
| **Dependency management** | No | Yes |
| **Cycle detection** | No | Yes |
| **Execution levels** | No | Yes |
| **AI synthesis** | Basic | Advanced (multi-source) |
| **Execution history** | No | Yes |
| **Workflow metrics** | No | Yes |
| **Predefined workflows** | No | Yes |
| **Natural language routing** | Basic | Advanced |
| **Distributed tracing** | No | Yes (TracingMiddleware + TracedHTTPClient) |

---

## Implementation Phases

### Phase 1: Core Tools (5 tools)
- [ ] Create `geocoding-tool` with Nominatim API
- [ ] Create `weather-tool-v2` with Open-Meteo API
- [ ] Create `currency-tool` with Frankfurter API
- [ ] Create `country-info-tool` with RestCountries API
- [ ] Create `news-tool` with GNews.io API

### Phase 2: Orchestration Agent
- [ ] Create `agent-with-orchestration` directory structure
- [ ] Implement `research_agent.go` with orchestrator setup
- [ ] Implement `workflows.go` with DAG building
- [ ] Implement `handlers.go` with API endpoints
- [ ] Create setup.sh with run-all support

### Phase 3: Documentation & Testing
- [ ] Write comprehensive README.md
- [ ] Create demo script
- [ ] Add integration tests
- [ ] Create K8s deployment manifests

---

## Risk Considerations

| Risk | Mitigation |
|------|------------|
| API rate limits | Respect rate limits (esp. Nominatim 1/sec), implement caching |
| GNews API key required | Provide fallback without news, document free signup |
| Network failures | Use resilience patterns from agent-with-resilience |
| Long execution times | Set appropriate timeouts, show progress |

---

## References

- [Nominatim API Documentation](https://nominatim.org/release-docs/latest/api/Overview/)
- [Open-Meteo API Documentation](https://open-meteo.com/en/docs)
- [Frankfurter API Documentation](https://frankfurter.dev/docs)
- [RestCountries API Documentation](https://restcountries.com/)
- [GNews API Documentation](https://gnews.io/docs/v4)
- [Microsoft AI Agent Design Patterns](https://learn.microsoft.com/en-us/azure/architecture/ai-ml/guide/ai-agent-design-patterns)
- [Public APIs GitHub Repository](https://github.com/public-apis/public-apis)

---

*Document Version: 1.1*
*Created: 2025-12-02*
*Updated: 2025-12-05*
*Status: Awaiting Approval*

### Change Log
- **v1.1** (2025-12-05): Added "Dynamic Request Handling" section explaining AI-powered orchestration internals
