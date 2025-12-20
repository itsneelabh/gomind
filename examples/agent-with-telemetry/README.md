# Research Agent with Telemetry

A production-ready intelligent research agent with comprehensive observability through OpenTelemetry integration. This example demonstrates the complete telemetry capabilities of the GoMind framework, including metrics and distributed tracing.

## Overview

This example extends the basic [agent-example](../agent-example) with:

- **Comprehensive Metrics**: 10+ metric types tracking research operations, tool calls, AI synthesis, and discovery
- **Distributed Tracing**: End-to-end request tracing across the agent and tool ecosystem
- **Multi-Environment Profiles**: Development (100% sampling), Staging (10%), Production (0.1%)
- **Production-Ready Configuration**: Environment-based telemetry with graceful degradation

> **Scope**: This example focuses on **telemetry and observability**. For intelligent error handling with AI-powered retry and parameter correction, see [agent-with-orchestration](../agent-with-orchestration/) which uses the orchestration module.

## What You'll Learn

- How to integrate the GoMind telemetry module into your agents
- Best practices for declaring and emitting metrics
- Configuring environment-specific telemetry profiles
- Debugging performance issues with distributed tracing
- Production deployment with Kubernetes

## Architecture

```
┌─────────────────────┐
│  Research Agent     │
│  (Port 8092)        │
│  ┌───────────────┐  │     ┌────────────────────┐
│  │ Telemetry     │──┼────▶│ OTEL Collector     │
│  │ Module        │  │     │ (Kubernetes)       │
│  └───────────────┘  │     └─────────┬──────────┘
└─────────────────────┘               │
                                      │
                         ┌────────────┼──────────────┐
                         │            │              │
                         ▼            ▼              ▼
                  ┌──────────┐ ┌──────────┐  ┌──────────┐
                  │Prometheus│ │  Jaeger  │  │ Logging  │
                  │          │ │          │  │          │
                  └────┬─────┘ └────┬─────┘  └──────────┘
                       │            │
                       └─────┬──────┘
                             ▼
                      ┌──────────┐
                      │ Grafana  │
                      │          │
                      └──────────┘
```

## Metrics Collected

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `agent.research.duration_ms` | Histogram | Research operation duration | topic, status |
| `agent.research.requests` | Counter | Total research requests | status |
| `agent.research.tools_called` | Counter | Tools called during research | tool_name |
| `agent.tool_call.duration_ms` | Histogram | Individual tool call duration | tool |
| `agent.tool_call.success` | Counter | Successful tool calls | tool |
| `agent.tool_call.errors` | Counter | Tool call errors | tool, error_type |
| `agent.discovery.duration_ms` | Histogram | Tool discovery duration | - |
| `agent.tools.discovered` | Gauge | Number of tools discovered | - |
| `agent.ai_synthesis.duration_ms` | Histogram | AI synthesis duration | - |
| `agent.ai.requests` | Counter | AI API requests | provider, operation |
| `agent.ai.tokens.prompt` | Counter | Prompt tokens used | provider |
| `agent.ai.tokens.completion` | Counter | Completion tokens used | provider |

## Quick Start

### One-Click Local Setup (Easiest!)

For local development with Kind, use the automated setup script:

```bash
cd examples/agent-with-telemetry
./setup.sh
```

This script will:
- ✅ Create a Kind cluster
- ✅ Deploy complete monitoring infrastructure (Redis, OTEL, Prometheus, Jaeger, Grafana)
- ✅ Build and deploy the agent
- ✅ Set up port forwarding
- ✅ Test the deployment

**Access after setup:**
- Agent: http://localhost:8092
- Grafana: http://localhost:3000 (admin/admin)
- Prometheus: http://localhost:9090
- Jaeger: http://localhost:16686

### Manual Setup

#### Prerequisites

- Go 1.25+
- Kubernetes cluster with GoMind monitoring stack
  - **Setup:** Run `cd examples/k8-deployment && ./setup-infrastructure.sh`
  - See [k8-deployment](../k8-deployment) for details
- Redis (deployed by infrastructure script)
- OpenAI API key (or compatible provider)

#### Local Development

1. **Clone and navigate to the example**:
   ```bash
   cd examples/agent-with-telemetry
   ```

2. **Copy and configure environment**:
   ```bash
   cp .env.example .env
   # Edit .env and add your OPENAI_API_KEY
   ```

3. **Run the agent locally**:
   ```bash
   # For local dev, telemetry will be disabled unless OTEL_EXPORTER_OTLP_ENDPOINT is set
   export APP_ENV=development
   go run .
   ```

4. **Test the agent**:
   ```bash
   curl -X POST http://localhost:8092/api/capabilities/research_topic \
     -H "Content-Type: application/json" \
     -d '{"topic": "latest AI developments", "ai_synthesis": true}'
   ```

### Kubernetes Deployment

1. **Deploy infrastructure first** (if not already running):
   ```bash
   cd examples/k8-deployment
   ./setup-infrastructure.sh

   # The script will:
   # - Check if infrastructure components already exist
   # - Skip deployment if they're healthy
   # - Deploy only what's missing
   # - Never delete existing resources
   ```

2. **Build and push the Docker image**:
   ```bash
   cd examples/agent-with-telemetry
   make docker-build
   docker tag research-agent-telemetry:latest your-registry/research-agent-telemetry:v1
   docker push your-registry/research-agent-telemetry:v1
   ```

3. **Deploy to Kubernetes**:
   ```bash
   # Update image in k8-deployment.yaml to match your registry
   kubectl apply -f k8-deployment.yaml
   ```

4. **Verify deployment**:
   ```bash
   kubectl get pods -n gomind-examples
   kubectl logs -f deployment/research-agent-telemetry -n gomind-examples
   ```

5. **Access monitoring**:
   - Metrics will be sent to the OTEL Collector configured in your cluster
   - View in Grafana dashboards (see your cluster's monitoring setup)
   - View traces in Jaeger

## AI Module Distributed Tracing

This example includes distributed tracing for AI operations. When you view traces in Jaeger, you'll see `ai.generate_response` and `ai.http_attempt` spans with full details including:

- **Token usage**: `ai.prompt_tokens`, `ai.completion_tokens`, `ai.total_tokens`
- **Model info**: `ai.provider`, `ai.model`
- **HTTP details**: `http.status_code`, `ai.attempt_duration_ms`
- **Retry tracking**: `ai.attempt`, `ai.max_retries`, `ai.is_retry`

### Critical: Initialization Order

**The telemetry module MUST be initialized BEFORE creating the AI client.** This example demonstrates the correct order in `main.go`:

```go
func main() {
    // 1. Set component type for service_type labeling
    core.SetCurrentComponentType(core.ComponentTypeAgent)

    // 2. Initialize telemetry BEFORE creating agent
    initTelemetry(serviceName)
    defer telemetry.Shutdown(context.Background())

    // 3. Create agent AFTER telemetry - AI client gets the provider
    agent, err := NewResearchAgent()
    // ...
}
```

If you reverse this order (creating the agent before telemetry), `telemetry.GetTelemetryProvider()` returns `nil` and no AI spans will appear in your traces.

## Migration Guide

If you have an existing agent based on [agent-example](../agent-example), follow these steps to add telemetry:

### Step 1: Add Telemetry Dependency

**go.mod**:
```go
require (
    github.com/itsneelabh/gomind/core v0.6.5
    github.com/itsneelabh/gomind/ai v0.6.5
    github.com/itsneelabh/gomind/telemetry v0.6.5  // Add this
)
```

Run: `go mod tidy`

### Step 2: Initialize Telemetry in main.go

**Before**:
```go
func main() {
    agent := NewResearchAgent(aiClient)
    agent.Start()
}
```

**After**:
```go
import "github.com/itsneelabh/gomind/telemetry"

func main() {
    // Initialize telemetry BEFORE creating agent
    initTelemetry("research-assistant-telemetry")
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := telemetry.Shutdown(ctx); err != nil {
            log.Printf("⚠️  Telemetry shutdown error: %v", err)
        }
    }()

    agent := NewResearchAgent(aiClient)
    agent.Start()
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
        log.Printf("⚠️  Telemetry initialization failed: %v", err)
        log.Printf("   Application will continue without telemetry")
    }
}
```

### Step 3: Declare Metrics in NewAgent

**Add to your agent constructor**:
```go
func NewResearchAgent(aiClient core.AIClient) *ResearchAgent {
    agent := &ResearchAgent{
        BaseAgent:  core.NewBaseAgent("research-assistant-telemetry"),
        aiClient:   aiClient,
        httpClient: &http.Client{Timeout: 30 * time.Second},
    }

    // NEW: Declare metrics this agent will emit
    telemetry.DeclareMetrics("research-agent", telemetry.ModuleConfig{
        Metrics: []telemetry.MetricDefinition{
            {
                Name:    "agent.research.duration_ms",
                Type:    "histogram",
                Help:    "Research operation duration in milliseconds",
                Labels:  []string{"topic", "status"},
                Unit:    "milliseconds",
                Buckets: []float64{100, 500, 1000, 5000, 10000, 30000},
            },
            {
                Name:   "agent.research.requests",
                Type:   "counter",
                Help:   "Total research requests",
                Labels: []string{"status"},
            },
            // Add more metrics as needed
        },
    })

    agent.RegisterCapability(/* ... */)
    return agent
}
```

### Step 4: Emit Metrics in Handlers

**Before**:
```go
func (r *ResearchAgent) handleResearchTopic(rw http.ResponseWriter, req *http.Request) {
    // ... process request ...

    results := r.orchestrateResearch(ctx, request)

    // ... return response ...
}
```

**After**:
```go
import "github.com/itsneelabh/gomind/telemetry"

func (r *ResearchAgent) handleResearchTopic(rw http.ResponseWriter, req *http.Request) {
    startTime := time.Now()

    // Track overall operation duration
    defer func() {
        telemetry.Histogram("agent.research.duration_ms",
            float64(time.Since(startTime).Milliseconds()),
            "topic", request.Topic,
            "status", "completed")
    }()

    // Track request count
    telemetry.Counter("agent.research.requests", "status", "started")

    // ... process request ...

    results := r.orchestrateResearch(ctx, request)

    // ... return response ...
}
```

### Step 5: Add Tool Call Telemetry

Track individual tool invocations with timing and success/failure metrics:

```go
func (r *ResearchAgent) callToolWithCapability(ctx context.Context, tool *core.ServiceInfo, capability *core.Capability, topic string) *ToolResult {
    startTime := time.Now()

    // Track individual tool call duration
    defer func() {
        telemetry.Histogram("agent.tool_call.duration_ms",
            float64(time.Since(startTime).Milliseconds()),
            "tool", tool.Name)
    }()

    // ... make tool call ...

    if err != nil {
        telemetry.Counter("agent.tool_call.errors",
            "tool", tool.Name,
            "error_type", classifyError(err))
        return &ToolResult{Success: false, Error: err.Error()}
    }

    telemetry.Counter("agent.tool_call.success", "tool", tool.Name)
    return result
}
```

### Step 6: Add Environment Configuration

**Add to .env**:
```bash
# Telemetry Configuration
APP_ENV=development                           # development|staging|production
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

**Add to Kubernetes deployment**:
```yaml
env:
  - name: APP_ENV
    value: "production"
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "http://otel-collector.monitoring:4318"
```

## Environment Profiles

The telemetry module supports three built-in profiles:

| Profile | Trace Sampling | Metric Interval | Use Case |
|---------|----------------|-----------------|----------|
| Development | 100% | 1s | Local development, debugging |
| Staging | 10% | 5s | Pre-production testing |
| Production | 0.1% | 15s | Production workloads |

Set via `APP_ENV` environment variable:
```bash
APP_ENV=development  # 100% sampling
APP_ENV=staging      # 10% sampling
APP_ENV=production   # 0.1% sampling
```

## Configuration Reference

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8092 | HTTP server port |
| `REDIS_URL` | redis://localhost:6379 | Redis connection URL |
| `OPENAI_API_KEY` | - | OpenAI API key (required) |
| `OPENAI_BASE_URL` | - | Custom OpenAI endpoint (optional) |
| `APP_ENV` | development | Telemetry profile (development/staging/production) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | http://localhost:4318 | OTEL Collector endpoint |

### Makefile Targets

```bash
make build           # Build the binary
make run             # Run locally
make test            # Run tests
make docker-build    # Build Docker image
make k8s-deploy      # Deploy to Kubernetes
make clean           # Clean build artifacts
```

## Troubleshooting

### No metrics appearing in monitoring system

1. **Check OTEL Collector endpoint**:
   ```bash
   echo $OTEL_EXPORTER_OTLP_ENDPOINT  # Should be configured
   ```

2. **Verify environment profile**:
   ```bash
   # Check logs for telemetry initialization
   kubectl logs deployment/research-agent-telemetry -n gomind-examples | grep -i telemetry
   ```

3. **Test with development profile locally**:
   ```bash
   export APP_ENV=development
   export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
   go run .
   ```

### Traces not appearing in Jaeger

1. **Verify sampling rate**:
   - Check `APP_ENV` is set to "development" for 100% sampling
   - Production uses 0.1% sampling by design

2. **Check OTEL Collector configuration**:
   - Ensure trace pipeline is configured in your cluster
   - Verify Jaeger endpoint is reachable

### High memory usage

For production deployments:

1. **Use Production profile**:
   ```bash
   export APP_ENV=production  # 0.1% sampling
   ```

2. **Monitor cardinality**:
   - Check telemetry health: `telemetry.GetHealth()`
   - Avoid high-cardinality labels (user IDs, timestamps)

## Telemetry Best Practices

### DO ✅

- **Initialize early**: Set up telemetry at the start of main()
- **Use profiles**: Leverage pre-configured profiles for different environments
- **Add context**: Use labels to make metrics meaningful
- **Handle failures gracefully**: Don't let telemetry crash your app
- **Use bounded labels**: Only use labels with limited value sets

### DON'T ❌

- **Don't use high-cardinality labels**: No user IDs, timestamps, or UUIDs
- **Don't emit sensitive data**: No passwords, tokens, or PII in metrics
- **Don't over-instrument**: Start simple, add more as needed
- **Don't block on telemetry**: Use appropriate timeouts

## Key Implementation Patterns

### 1. Metric Declaration (Before Use)

Declare all metrics upfront in your constructor:

```go
telemetry.DeclareMetrics("component-name", telemetry.ModuleConfig{
    Metrics: []telemetry.MetricDefinition{
        {
            Name:    "metric.name",
            Type:    "histogram",  // or "counter", "gauge"
            Help:    "What this metric measures",
            Labels:  []string{"label1", "label2"},
            Buckets: []float64{...},  // For histograms
        },
    },
})
```

### 2. Duration Tracking Pattern

Use defer for automatic duration tracking:

```go
func operation() {
    startTime := time.Now()
    defer func() {
        telemetry.Histogram("operation.duration_ms",
            float64(time.Since(startTime).Milliseconds()),
            "status", "completed")
    }()

    // Your operation code here
}
```

### 3. Error Classification Pattern

Classify errors for better metrics grouping:

```go
func classifyError(err error) string {
    errStr := err.Error()
    switch {
    case strings.Contains(errStr, "timeout"):
        return "timeout"
    case strings.Contains(errStr, "connection refused"):
        return "connection_refused"
    case strings.Contains(errStr, "context canceled"):
        return "canceled"
    default:
        return "unknown"
    }
}
```

> **See Also**: For advanced error handling patterns including AI-powered error correction and intelligent retry strategies, see the [Intelligent Error Handling Guide](https://github.com/itsneelabh/gomind/blob/main/docs/INTELLIGENT_ERROR_HANDLING.md).

## Next Steps

1. **Add Custom Metrics**: Extend `DeclareMetrics()` with agent-specific metrics
2. **Create Alerts**: Define Prometheus alert rules for critical conditions
3. **Custom Dashboards**: Build Grafana dashboards for your specific use case
4. **Distributed Tracing**: See the [Distributed Tracing Guide](../../docs/DISTRIBUTED_TRACING_GUIDE.md) for complete tracing patterns, log correlation, and multi-service examples

## Related Examples

- [agent-example](../agent-example) - Basic agent without telemetry
- [tool-example](../tool-example) - Tool with telemetry integration
- [orchestration-example](../orchestration-example) - Advanced orchestration patterns

## Learn More

- [GoMind Telemetry Module](../../telemetry/README.md) - Complete telemetry documentation
- [Distributed Tracing Guide](../../docs/DISTRIBUTED_TRACING_GUIDE.md) - **End-to-end request tracing, log correlation, and multi-service examples**
- [OpenTelemetry Go Documentation](https://opentelemetry.io/docs/languages/go/)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/naming/)

## License

This example is part of the GoMind framework and is licensed under the same terms.
