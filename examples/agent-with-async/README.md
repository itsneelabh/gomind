# Async AI-Driven Agent

This example demonstrates the **GoMind async task system** combined with **AI orchestration** for autonomous multi-tool coordination. The agent accepts natural language queries and dynamically decides which tools to call.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Async AI Agent                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  HTTP API (Task Submission)                                      │
│  POST /api/v1/tasks        → Submit natural language query       │
│  GET  /api/v1/tasks/:id    → Poll task status/result             │
│  POST /api/v1/tasks/:id/cancel → Cancel running task             │
│                                                                  │
│  AI Orchestration Flow                                           │
│  1. Parse natural language query                                 │
│  2. AI generates execution plan based on available tools        │
│  3. DAG executor runs steps (parallel where possible)           │
│  4. OnStepComplete callback reports per-tool progress           │
│  5. AI synthesizes final response                               │
│                                                                  │
│  Available Tools (auto-discovered via Redis):                    │
│  • weather-tool-v2   → Weather forecasts for any location        │
│  • geocoding-tool    → Convert location names to coordinates     │
│  • currency-tool     → Currency conversion and exchange rates    │
│  • news-tool         → Search news articles by topic             │
│  • stock-market-tool → Stock prices and market news              │
│  • country-info-tool → Country information and facts             │
│                                                                  │
│  Adding new tools = Just deploy them (no agent code changes!)   │
│  See "Deploy Tool Examples" section below.                       │
└─────────────────────────────────────────────────────────────────┘
```

## Features

- **AI-Driven Orchestration**: LLM dynamically decides which tools to call based on query intent
- **Natural Language Input**: No structured input required - just describe what you need
- **DAG-Based Parallel Execution**: Independent tools run concurrently for faster results
- **Per-Tool Progress Reporting**: Real-time updates as each tool completes via `OnStepComplete` callback
- **4-Layer Intelligent Error Recovery**: Semantic retry with LLM-based error analysis
- **Async Task Execution**: HTTP 202 + polling pattern for long-running operations
- **Zero Code Changes for New Tools**: Just deploy new tools - the agent discovers them automatically

## Quick Start

### Prerequisites

- Go 1.25+
- Docker (for building container images)
- [Kind](https://kind.sigs.k8s.io/) (for local Kubernetes cluster)
- kubectl
- At least ONE AI provider API key (see below)

> **Note**: The `setup.sh` script handles Kind cluster creation automatically. Run `./setup.sh cluster` to create the cluster, or `./setup.sh full` for complete setup including monitoring infrastructure.

### AI Provider Setup

The agent uses **Chain Client** for automatic failover between AI providers. You only need **one** provider configured, but multiple providers give you higher availability.

```bash
# Copy the example configuration
cp .env.example .env

# Edit .env and add at least ONE API key:
```

| Provider | Environment Variable | Get API Key | Notes |
|----------|---------------------|-------------|-------|
| **OpenAI** (recommended) | `OPENAI_API_KEY` | [platform.openai.com](https://platform.openai.com/api-keys) | Best quality, most features |
| **Anthropic** | `ANTHROPIC_API_KEY` | [console.anthropic.com](https://console.anthropic.com/) | Excellent reasoning |
| **Groq** | `GROQ_API_KEY` | [console.groq.com](https://console.groq.com/keys) | Ultra-fast, free tier available |
| **Google Gemini** | `GEMINI_API_KEY` | [aistudio.google.com](https://aistudio.google.com/apikey) | Good multimodal |
| **DeepSeek** | `DEEPSEEK_API_KEY` | [platform.deepseek.com](https://platform.deepseek.com/) | Advanced reasoning |

**Failover Order**: OpenAI → Anthropic → Groq (configurable)

**Model Aliases**: Use `GOMIND_{PROVIDER}_MODEL_DEFAULT` to override models without code changes:
```bash
# Use cheaper models in development
GOMIND_OPENAI_MODEL_DEFAULT=gpt-4o-mini
GOMIND_ANTHROPIC_MODEL_DEFAULT=claude-3-haiku-20240307
```

> See [AI Module Documentation](../../ai/README.md) for full provider configuration details.

### Deploy Tool Examples (Required)

The agent **dynamically discovers tools** via Redis service discovery. Without tools deployed, the agent has nothing to call!

**Available Tool Examples:**

| Tool | Capabilities | Deploy Command |
|------|-------------|----------------|
| `weather-tool-v2` | Get weather forecasts | `./examples/weather-tool-v2/setup.sh deploy` |
| `geocoding-tool` | Convert locations to coordinates | `./examples/geocoding-tool/setup.sh deploy` |
| `currency-tool` | Currency conversion rates | `./examples/currency-tool/setup.sh deploy` |
| `news-tool` | Search news articles | `./examples/news-tool/setup.sh deploy` |
| `stock-market-tool` | Stock prices and market news | `./examples/stock-market-tool/setup.sh deploy` |
| `country-info-tool` | Country information | `./examples/country-info-tool/setup.sh deploy` |

**Deploy all tools at once:**
```bash
# From the gomind root directory
for tool in weather-tool-v2 geocoding-tool currency-tool news-tool stock-market-tool country-info-tool; do
  ./examples/$tool/setup.sh deploy
done
```

**How it works:**
1. Tools register their capabilities in Redis when they start
2. The agent queries Redis to discover available tools
3. AI orchestrator selects tools based on user query intent
4. **Adding new tools** = Just deploy them - the agent discovers them automatically!

### Run Locally

```bash
# Start all services (Redis, tools, and agent)
./setup.sh run-all

# Or run components separately:
./setup.sh setup  # Setup environment only
./setup.sh run    # Setup and run agent
./setup.sh build  # Build only
```

### Submit a Natural Language Query (Recommended)

```bash
# Submit a natural language query
curl -X POST http://localhost:8098/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "query",
    "input": {
      "query": "What is the weather in Paris?"
    }
  }'
```

**Response (Task Queued):**
```json
{
  "task_id": "b4bdca9e-85e6-4a26-be68-7d86432b0c62",
  "status": "queued",
  "status_url": "/api/v1/tasks/b4bdca9e-85e6-4a26-be68-7d86432b0c62"
}
```

### Poll for Status

```bash
# Check task status
curl http://localhost:8098/api/v1/tasks/b4bdca9e-85e6-4a26-be68-7d86432b0c62
```

**Response (In Progress):**
```json
{
  "task_id": "b4bdca9e-85e6-4a26-be68-7d86432b0c62",
  "type": "query",
  "status": "running",
  "progress": {
    "current_step": 2,
    "total_steps": 3,
    "step_name": "completed: weather-tool-v2",
    "percentage": 75,
    "message": "Tool 1/1 completed"
  }
}
```

**Response (Completed):**
```json
{
  "task_id": "b4bdca9e-85e6-4a26-be68-7d86432b0c62",
  "type": "query",
  "status": "completed",
  "progress": {
    "current_step": 3,
    "total_steps": 3,
    "step_name": "Complete",
    "percentage": 100,
    "message": "Executed 1 tools successfully"
  },
  "result": {
    "query": "What is the weather in Paris?",
    "response": "The current weather in Paris is characterized by slight snowfall, with a temperature of -0.6°C. The humidity level is quite high at 93%, and there is a light wind blowing at a speed of 5.1 km/h. Given the cold temperatures and snow, it's advisable to dress warmly if you're planning to go outside.",
    "tools_used": ["weather-tool-v2"],
    "step_results": [
      {
        "tool_name": "weather-tool-v2",
        "success": true,
        "duration": "609.030792ms"
      }
    ],
    "execution_time": "9.259515754s",
    "confidence": 0.95,
    "metadata": {
      "duration_ms": 9259,
      "mode": "ai_orchestrated",
      "request_id": "1767631206964317921-964318004"
    }
  },
  "created_at": "2026-01-05T16:40:06.956579004Z",
  "started_at": "2026-01-05T16:40:06.958070462Z",
  "completed_at": "2026-01-05T16:40:16.2178858Z"
}
```

### Multi-Tool Query Example

The AI orchestrator automatically selects multiple tools based on query complexity:

```bash
curl -X POST http://localhost:8098/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "query",
    "input": {
      "query": "I am planning to visit Tokyo next week. What is the weather forecast and current exchange rate from USD to JPY?"
    }
  }'
```

**Response (Completed with Multiple Tools):**
```json
{
  "task_id": "abc123-multi-tool",
  "type": "query",
  "status": "completed",
  "progress": {
    "current_step": 5,
    "total_steps": 5,
    "step_name": "Complete",
    "percentage": 100,
    "message": "Executed 3 tools successfully"
  },
  "result": {
    "query": "I am planning to visit Tokyo next week...",
    "response": "Based on my research for your Tokyo trip next week: The weather forecast shows temperatures around 8-12°C with partly cloudy skies. The current exchange rate is 1 USD = 149.50 JPY. I recommend bringing layers for the variable weather.",
    "tools_used": ["geocoding-tool", "weather-tool-v2", "currency-tool"],
    "step_results": [
      {"tool_name": "geocoding-tool", "success": true, "duration": "120.5ms"},
      {"tool_name": "weather-tool-v2", "success": true, "duration": "650.2ms"},
      {"tool_name": "currency-tool", "success": true, "duration": "180.8ms"}
    ],
    "execution_time": "12.5s",
    "confidence": 0.95,
    "metadata": {
      "mode": "ai_orchestrated"
    }
  }
}
```

### Cancel a Task

```bash
curl -X POST http://localhost:8098/api/v1/tasks/b4bdca9e-85e6-4a26-be68-7d86432b0c62/cancel
```

**Response:**
```json
{
  "task_id": "b4bdca9e-85e6-4a26-be68-7d86432b0c62",
  "status": "cancelled",
  "message": "Task cancelled successfully"
}
```

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `REDIS_URL` | Required | Redis connection URL |
| `PORT` | 8098 | HTTP server port |
| `WORKER_COUNT` | 3 | Number of background workers |
| `NAMESPACE` | - | Kubernetes namespace for discovery |
| `OPENAI_API_KEY` | - | OpenAI API key for AI orchestration |
| `ANTHROPIC_API_KEY` | - | Anthropic API key for AI orchestration |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | - | OpenTelemetry collector endpoint |
| `APP_ENV` | development | Environment (development/staging/production) |
| `DEV_MODE` | false | Enable development mode |

## Task Input Schema

### Natural Language Query

```json
{
  "type": "query",
  "input": {
    "query": "string (required) - Natural language description of what you need"
  }
}
```

## Deployment Modes

The agent supports three deployment modes via the `GOMIND_MODE` environment variable:

| Mode | `GOMIND_MODE` | Description | Use Case |
|------|---------------|-------------|----------|
| **Embedded** | `""` (unset) | API + Workers in same process | Local development |
| **API** | `api` | HTTP server only, enqueues tasks | Production (scale by request load) |
| **Worker** | `worker` | Task processing only, minimal /health | Production (scale by queue depth) |

### Local Development (Embedded Mode)

```bash
# Run with both API and workers in same process
./setup.sh run

# Or explicitly
GOMIND_MODE= go run .
```

### Production Architecture

For production, deploy API and Worker separately using the same Docker image:

```
┌─────────────────────────────┐     ┌─────────────────────────────┐
│ async-travel-agent-api      │     │ async-travel-agent-worker   │
│ (GOMIND_MODE=api)           │     │ (GOMIND_MODE=worker)        │
├─────────────────────────────┤     ├─────────────────────────────┤
│ • POST /api/v1/tasks        │     │ • GET /health               │
│ • GET /api/v1/tasks/:id     │     │ • BRPOP from Redis queue    │
│ • Scale: HTTP request rate  │     │ • Scale: Redis queue depth  │
└──────────────┬──────────────┘     └──────────────┬──────────────┘
               │         ┌─────────────────┐       │
               └────────>│     Redis       │<──────┘
                         │  Task Queue     │
                         └─────────────────┘
```

**Benefits of Split Deployment:**
- Scale API and Workers independently
- API pods are lightweight (100m CPU, 128Mi memory)
- Worker pods are compute-heavy (500m-1000m CPU, 256Mi-1Gi memory)
- Isolate task processing logs from HTTP logs

## Kubernetes Deployment

### Development/Testing (Embedded Mode)

```bash
# Deploy embedded mode (API + workers in same pod)
./setup.sh deploy

# Port forward to access locally
kubectl port-forward -n gomind-examples svc/async-travel-agent-service 8098:80
```

### Production (Split Mode)

```bash
# Deploy separate API and Worker deployments
./setup.sh deploy-prod

# This deploys:
#   - async-travel-agent-api (2 replicas, GOMIND_MODE=api)
#   - async-travel-agent-worker (2 replicas, GOMIND_MODE=worker)
```

### K8s Deployment Files

| File | Mode | Description |
|------|------|-------------|
| `k8-deployment.yaml` | Embedded | Single deployment with API + Workers |
| `k8-deployment-api.yaml` | API | HTTP server only (production) |
| `k8-deployment-worker.yaml` | Worker | Task processing only (production) |

### Prerequisites for K8s

Ensure these are deployed first:
1. Redis (for task queue and service discovery)
2. OpenTelemetry Collector (for telemetry)
3. Jaeger (for distributed tracing)
4. Any tools you want the agent to use (auto-discovered via Redis)

## Observability

The agent integrates with OpenTelemetry for comprehensive observability:

- **Traces**: Full request journey including async task processing and per-tool calls
- **Metrics**: Task counts, durations, tool call counts, tools per query
- **Logs**: Structured JSON logging with correlation IDs

### Accessing Observability Stack

```bash
# Jaeger (Distributed Tracing)
kubectl port-forward -n gomind-examples svc/jaeger-query 16686:80
# Open http://localhost:16686

# Prometheus (Metrics)
kubectl port-forward -n gomind-examples svc/prometheus 9090:9090
# Open http://localhost:9090

# Grafana (Dashboards)
kubectl port-forward -n gomind-examples svc/grafana 3000:3000
# Open http://localhost:3000
```

### Distributed Tracing with Jaeger

The async task system uses **linked spans** (OpenTelemetry standard) to connect API and Worker traces across the async boundary.

#### Understanding the Trace Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    DISTRIBUTED TRACE FLOW                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Client Request                                                          │
│       │                                                                  │
│       ▼                                                                  │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │ API Trace (async-travel-agent-api)                               │    │
│  │ TraceID: abcd1234...                                             │    │
│  │                                                                   │    │
│  │   HTTP POST /api/v1/tasks [2ms]                                  │    │
│  │       └── Store trace context in task                            │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                            │                                             │
│                            │ Task stored in Redis with                  │
│                            │ trace_id + parent_span_id                  │
│                            ▼                                             │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │ Worker Trace (async-travel-agent-worker)                         │    │
│  │ TraceID: xyz789... (NEW trace)                                   │    │
│  │                                                                   │    │
│  │   task.process [6.6s]                                            │    │
│  │   │  └── FOLLOWS_FROM: abcd1234... (link to API trace)          │    │
│  │   │                                                               │    │
│  │   ├── orchestrator.process_request [6.5s]                        │    │
│  │   │   ├── orchestrator.build_prompt [1ms]                        │    │
│  │   │   ├── ai.chain.generate_response [3.7s]                      │    │
│  │   │   │   └── ai.http_attempt (OpenAI API call)                  │    │
│  │   │   │                                                           │    │
│  │   │   ├── HTTP POST /api/capabilities/get_current_weather [620ms]│    │
│  │   │   │   └── HTTP GET api.open-meteo.com [618ms]                │    │
│  │   │   │                                                           │    │
│  │   │   └── ai.chain.generate_response [2.9s] (synthesis)          │    │
│  │   │                                                               │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

#### How to Navigate Traces in Jaeger

**Step 1: View Worker Traces**
1. Open Jaeger UI: http://localhost:16686
2. Select Service: `async-travel-agent-worker`
3. Click "Find Traces"
4. Click on a trace to expand

**Step 2: Find the Linked API Trace**
1. Expand the `task.process` span
2. Look at the **"References"** section
3. You'll see: `FOLLOWS_FROM → [original trace ID]`
4. Click the trace ID to jump to the API trace

**Step 3: Understand Span Hierarchy**

| Span | Description | Typical Duration |
|------|-------------|------------------|
| `task.process` | Root worker span, contains FOLLOWS_FROM link | 5-30s |
| `orchestrator.process_request` | AI orchestration | 5-30s |
| `orchestrator.build_prompt` | Building LLM prompt | 1-5ms |
| `ai.chain.generate_response` | LLM API call | 2-10s |
| `ai.http_attempt` | Actual HTTP to OpenAI/Anthropic | 2-10s |
| `HTTP POST /api/capabilities/*` | Tool call to another service | 100ms-2s |

#### Why Separate Traces?

The API and Worker use **linked spans** instead of child spans because:

1. **Async Boundary**: The worker may process the task seconds/minutes later
2. **Independent Lifecycles**: API trace completes immediately (HTTP 202), worker trace can be long-running
3. **OpenTelemetry Standard**: `FOLLOWS_FROM` is the standard for async/queued operations

#### Viewing Tool Call Details

Tool calls appear as HTTP spans with logs showing key data:

```
Span: HTTP POST /api/capabilities/get_current_weather
  Tags:
    http.status_code: 200
    http.response_content_length: 267
  Logs:
    event: request_received
    event: calling_external_api (api: open-meteo, lat: 48.8566, lon: 2.3522)
    event: weather_retrieved (condition: "Slight snow fall", temperature: -0.6)
```

> **Note**: Full response bodies are not captured in traces (for performance/security). Key data points are logged instead.

### Metrics

The agent emits the following Prometheus metrics:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `gomind_async_orchestration_tasks` | Counter | `status` | Task completions (completed/failed) |
| `gomind_async_orchestration_tool_calls` | Counter | `tool`, `status` | Per-tool call counts |
| `gomind_async_orchestration_duration_ms` | Histogram | - | Task execution duration |
| `gomind_async_orchestration_tools_per_query` | Histogram | - | Number of tools called per query |

#### Example Prometheus Queries

```promql
# Task success rate
sum(rate(gomind_async_orchestration_tasks{status="completed"}[5m])) /
sum(rate(gomind_async_orchestration_tasks[5m]))

# Average task duration
rate(gomind_async_orchestration_duration_ms_sum[5m]) /
rate(gomind_async_orchestration_duration_ms_count[5m])

# Tool call failure rate by tool
sum(rate(gomind_async_orchestration_tool_calls{status="failed"}[5m])) by (tool)
```

### Submitting Tasks with Trace Context

To correlate client-side traces with the async task, include the W3C `traceparent` header:

```bash
# Submit task with trace context (for end-to-end tracing)
curl -X POST http://localhost:8098/api/v1/tasks \
  -H "Content-Type: application/json" \
  -H "traceparent: 00-abcd1234567890abcdef1234567890ab-1234567890abcdef-01" \
  -d '{"type":"query","input":{"query":"What is the weather in Paris?"}}'
```

The trace ID (`abcd1234...`) will be stored with the task and linked in the worker trace.

### Worker Logs

Worker logs are structured JSON for easy parsing. Example log output during task processing:

```json
// Telemetry initialization
{
  "component": "telemetry",
  "endpoint": "otel-collector.gomind-examples:4318",
  "level": "INFO",
  "message": "OpenTelemetry provider created successfully",
  "service": "async-travel-agent-worker",
  "service_name": "orchestrator",
  "timestamp": "2026-01-05T16:40:06Z"
}

// Orchestration startup
2026/01/05 16:40:06 [GOMIND-ORCH-V2] NewAIOrchestrator starting - EnableHybridResolution=true
[ORCHESTRATOR] Hybrid resolution enabled: hybridResolver=true, useHybridResolution=true

// Step execution
2026/01/05 16:40:12 [GOMIND-EXEC-V2] Step step-1: depends=0, useHybrid=true, resolverSet=true
```

**View Worker Logs:**
```bash
# Tail worker logs
kubectl logs -f -n gomind-examples -l app=async-travel-agent-worker

# Filter for specific task
kubectl logs -n gomind-examples -l app=async-travel-agent-worker | grep "task_id"

# Filter for orchestration events
kubectl logs -n gomind-examples -l app=async-travel-agent-worker | grep "GOMIND-ORCH"
```

## How It Works

1. **Task Submission**: Client submits a natural language query via POST /api/v1/tasks
2. **Queue**: Task is added to Redis queue with status "queued"
3. **Worker Pickup**: Background worker dequeues the task
4. **AI Planning**: LLM analyzes the query and determines which tools to call
5. **DAG Execution**: Tools are executed (parallel where dependencies allow)
6. **Progress Reporting**: `OnStepComplete` callback reports each tool completion
7. **AI Synthesis**: LLM combines tool outputs into a coherent response
8. **Completion**: Results stored, status set to "completed"
9. **Polling**: Client polls GET /api/v1/tasks/:id for status

## Files

| File | Description |
|------|-------------|
| `main.go` | Application entry point with GOMIND_MODE support (api/worker/embedded) |
| `travel_research_agent.go` | Agent definition, types, AI orchestrator initialization |
| `handlers.go` | Task handlers (HandleQuery, HandleLegacyTravelResearch) with OnStepComplete callback |
| `go.mod` | Go module dependencies |
| `.env.example` | Environment configuration template (copy to `.env`) |
| `Dockerfile` | Container build configuration |
| `Dockerfile.workspace` | Build using go workspace (for local development) |
| `k8-deployment.yaml` | Kubernetes manifests (embedded mode - development) |
| `k8-deployment-api.yaml` | Kubernetes manifests (API mode - production) |
| `k8-deployment-worker.yaml` | Kubernetes manifests (Worker mode - production) |
| `setup.sh` | Local development and deployment script |

## Key Benefits vs. Hardcoded Workflows

| Aspect | Old (Hardcoded) | New (AI-Driven) |
|--------|-----------------|-----------------|
| Input | Structured fields | Natural language |
| Workflow | Fixed in Go code | LLM-generated plan |
| Tool selection | Predetermined sequence | AI decides dynamically |
| Parallelization | Manual/sequential | Automatic DAG analysis |
| Error recovery | Basic logging | 4-layer intelligent retry |
| Adding features | Code change + redeploy | Just deploy new tools |
