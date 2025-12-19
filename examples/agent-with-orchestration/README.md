# Travel Research Agent with Orchestration

This example demonstrates the GoMind **orchestration module** through an intelligent Travel Research Agent that dynamically coordinates multiple travel-related tools.

## What This Example Teaches

1. **AI-Powered Orchestration** - How to use `orchestration.AIOrchestrator` for dynamic request routing
2. **DAG-Based Workflows** - Predefined workflows with parallel/sequential dependencies
3. **Dynamic Tool Discovery** - How the orchestrator discovers and invokes tools at runtime
4. **Natural Language Processing** - Convert user requests into execution plans using LLM
5. **AI Synthesis** - Combine multi-tool results into coherent responses
6. **Distributed Tracing** - Track requests across tool boundaries

## Architecture

```
                    ┌─────────────────────────────────────────────────────────┐
                    │           Travel Research Agent (Port 8094)              │
                    │                                                         │
                    │  ┌─────────────────────────────────────────────────┐   │
                    │  │              AI Orchestrator                      │   │
                    │  │  ┌─────────┐  ┌─────────┐  ┌──────────────┐    │   │
                    │  │  │ Catalog │  │ Executor│  │  Synthesizer │    │   │
                    │  │  └─────────┘  └─────────┘  └──────────────┘    │   │
                    │  └─────────────────────────────────────────────────┘   │
                    │                                                         │
                    │  ┌─────────────────────────────────────────────────┐   │
                    │  │           Predefined Workflows                    │   │
                    │  │  • travel-research (5 steps)                     │   │
                    │  │  • quick-weather (2 steps)                       │   │
                    │  │  • currency-check (1 step)                       │   │
                    │  └─────────────────────────────────────────────────┘   │
                    └─────────────────────────────────────────────────────────┘
                                              │
                    ┌─────────────────────────┴─────────────────────────┐
                    │                   Redis Discovery                   │
                    └─────────────────────────────────────────────────────┘
                                              │
        ┌─────────────┬─────────────┬─────────────┬─────────────┬─────────────┐
        ▼             ▼             ▼             ▼             ▼
   ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐
   │Geocoding│  │Weather  │  │Currency │  │Country  │  │  News   │
   │  Tool   │  │Tool v2  │  │  Tool   │  │  Info   │  │  Tool   │
   │ (8085)  │  │ (8086)  │  │ (8087)  │  │ (8088)  │  │ (8089)  │
   └─────────┘  └─────────┘  └─────────┘  └─────────┘  └─────────┘
```

## Two Orchestration Modes

### 1. Workflow Mode (Predefined DAGs)

Execute predefined workflows with explicit step dependencies:

```bash
curl -X POST http://localhost:8094/orchestrate/travel-research \
  -H "Content-Type: application/json" \
  -d '{
    "destination": "Tokyo, Japan",
    "country": "Japan",
    "base_currency": "USD",
    "amount": 1000
  }'
```

#### Travel Research Workflow Parameters

| Parameter | Required | Type | Description | Example |
|-----------|----------|------|-------------|---------|
| `destination` | Yes | string | City/location for geocoding and news | `"Tokyo, Japan"` |
| `country` | Yes | string | Country name for country-info lookup | `"Japan"` |
| `base_currency` | No | string | Your home currency (default: USD) | `"USD"` |
| `amount` | No | number | Amount to convert (default: 100) | `1000` |

The `travel-research` workflow executes:
1. **geocode** → Get coordinates for `{{destination}}`
2. **weather** (depends on geocode) → Get weather using `{{geocode.data.lat}}`, `{{geocode.data.lon}}`
3. **country-info** (parallel) → Get country information for `{{country}}`
4. **currency** (depends on country-info) → Convert `{{amount}}` from `{{base_currency}}` to `{{country-info.data.currency.code}}`
5. **news** (parallel) → Get travel news for `{{destination}}`

### 2. Dynamic Mode (AI-Powered)

Let the AI generate execution plans from natural language:

```bash
curl -X POST http://localhost:8094/orchestrate/natural \
  -H "Content-Type: application/json" \
  -d '{
    "request": "I am planning a trip to Paris next week. What is the weather like and what currency do they use?",
    "ai_synthesis": true
  }'
```

#### Natural Language Parameters

| Parameter | Required | Type | Description | Example |
|-----------|----------|------|-------------|---------|
| `request` | Yes | string | Natural language travel research request | `"What's the weather in Tokyo?"` |
| `ai_synthesis` | No | boolean | Enable AI synthesis (default: true) | `true` |
| `metadata` | No | object | Additional context and preferences | `{"user_preferences": {...}}` |

The orchestrator:
1. Discovers available tools from Redis
2. Uses LLM to generate an execution plan
3. Validates the plan against available capabilities
4. Executes steps (parallel when possible)
5. Synthesizes results into a coherent response

### Example Response (natural language)

```json
{
  "request_id": "natural-1733660421234567890",
  "original_request": "I am planning a trip to Paris next week. What is the weather like and what currency do they use?",
  "execution_plan": {
    "steps": [
      {"step_id": "geocode", "tool_name": "geocoding-tool", "capability": "geocode"},
      {"step_id": "weather", "tool_name": "weather-tool-v2", "capability": "get_weather", "depends_on": ["geocode"]},
      {"step_id": "country-info", "tool_name": "country-info-tool", "capability": "get_country_info"}
    ]
  },
  "step_results": [
    {"step_id": "geocode", "tool_name": "geocoding-tool", "success": true, "data": {"lat": 48.8566, "lon": 2.3522}},
    {"step_id": "weather", "tool_name": "weather-tool-v2", "success": true, "data": {"temp": 8.5, "condition": "Cloudy"}},
    {"step_id": "country-info", "tool_name": "country-info-tool", "success": true, "data": {"currency": {"code": "EUR", "name": "Euro"}}}
  ],
  "synthesized_response": "For your trip to Paris next week: The current weather shows 8.5°C with cloudy conditions. France uses the Euro (EUR) as its currency.",
  "execution_time": "2.341s",
  "tools_used": ["geocoding-tool", "weather-tool-v2", "country-info-tool"]
}
```

**Key Differences from Workflow Mode:**
- `execution_plan`: Shows the AI-generated plan with inferred dependencies
- `synthesized_response`: Natural language summary combining all tool results
- The AI determines which tools to call based on the request

### Complex Query Examples

These examples demonstrate the orchestrator's ability to handle sophisticated natural language requests that invoke all 5 tools.

#### Example 1: Single Destination (All 5 Tools)

```bash
curl -X POST http://localhost:8094/orchestrate/natural \
  -H "Content-Type: application/json" \
  -d '{
    "request": "I am planning a 2-week vacation to Tokyo, Japan starting next month. Can you help me with: 1) What is the current weather like in Tokyo? 2) What currency do they use in Japan and how much would 5000 USD convert to? 3) Tell me about Japan - population, languages spoken, and capital city. 4) Are there any recent travel news or advisories about Tokyo I should know about?",
    "ai_synthesis": true
  }'
```

**Expected Results:**

| Tool | Data Returned |
|------|---------------|
| geocoding-tool | Tokyo coordinates (lat/lon) |
| weather-tool-v2 | Temperature, conditions, humidity, wind |
| country-info-tool | Population: 123M, Language: Japanese, Capital: Tokyo |
| currency-tool | 5000 USD → ~781,749 JPY |
| news-tool | Current travel advisories |

**Metrics:** ~13 seconds execution, 0.95 confidence

#### Example 2: Multi-Destination Comparison (All 5 Tools × 2 Cities)

```bash
curl -X POST http://localhost:8094/orchestrate/natural \
  -H "Content-Type: application/json" \
  -d '{
    "request": "I want to compare traveling to Paris, France versus Berlin, Germany for a winter holiday. For each city, I need: the current weather conditions, currency information and how much 2000 USD would convert to their local currency, population and what languages are spoken, and any travel news about these cities.",
    "ai_synthesis": true
  }'
```

**Expected Results:**

| Aspect | Paris, France | Berlin, Germany |
|--------|--------------|-----------------|
| Weather | ~13°C, overcast | ~10°C, overcast |
| Currency | Euro (EUR) | Euro (EUR) |
| 2000 USD | ~1,718 EUR | ~1,718 EUR |
| Population | 66.35 million | 83.49 million |
| Language | French | German |
| Travel News | Current events/advisories | Current events/advisories |

**Metrics:** ~24 seconds execution, 0.95 confidence

#### Why These Queries Work

The orchestrator's **schema-based type coercion** (Layer 2) automatically converts LLM-generated string parameters to the correct types expected by each tool:

- `"35.6897"` (string) → `35.6897` (float64) for coordinates
- `"5000"` (string) → `5000` (number) for currency amounts
- `"true"` (string) → `true` (boolean) for flags

This eliminates type mismatch errors that would otherwise cause tool invocations to fail.

## Quick Start

### Prerequisites

- Go 1.23+
- Redis (local, Docker, or K8s)
- AI API key (OpenAI, Anthropic, Groq, etc.)
- Travel tools deployed (geocoding, weather-v2, currency, country-info, news)

### Local Development

```bash
# 1. Setup and run
./setup.sh run-all

# 2. Test the endpoints
curl http://localhost:8094/orchestrate/workflows
curl http://localhost:8094/health

# 3. Test travel-research workflow
curl -X POST http://localhost:8094/orchestrate/travel-research \
  -H "Content-Type: application/json" \
  -d '{"destination":"Paris, France","country":"France","base_currency":"USD","amount":1000}'
```

### Kubernetes Deployment

```bash
# 1. Build and deploy
./setup.sh deploy

# 2. Port forward
./setup.sh forward

# 3. Test
./setup.sh test
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/orchestrate/natural` | POST | Natural language orchestration |
| `/orchestrate/travel-research` | POST | Execute travel research workflow |
| `/orchestrate/custom` | POST | Execute custom workflow |
| `/orchestrate/workflows` | GET | List available workflows |
| `/orchestrate/history` | GET | Get execution history |
| `/api/capabilities` | GET | List agent capabilities with schemas |
| `/discover` | GET | Discover available tools |
| `/health` | GET | Health check with metrics |

### Example Response (travel-research)

```json
{
  "request_id": "travel-research-1234567890",
  "workflow_used": "travel-research",
  "execution_time": "1.885s",
  "confidence": 1,
  "step_results": [
    {"step_id": "geocode", "tool_name": "geocoding-tool", "success": true},
    {"step_id": "weather", "tool_name": "weather-tool-v2", "success": true},
    {"step_id": "country-info", "tool_name": "country-info-tool", "success": true},
    {"step_id": "currency", "tool_name": "currency-tool", "success": true},
    {"step_id": "news", "tool_name": "news-tool", "success": true}
  ]
}
```

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `REDIS_URL` | Yes | - | Redis connection URL |
| `PORT` | No | 8094 | HTTP server port |
| `OPENAI_API_KEY` | No* | - | OpenAI API key for AI features |
| `DEV_MODE` | No | false | Enable development mode |
| `GOMIND_ORCHESTRATOR_MODE` | No | autonomous | Orchestration mode |

*AI key required for natural language orchestration and synthesis

## How It Works

### Dynamic Request Flow

```
User Request: "What's the weather in Tokyo?"
                        │
                        ▼
              ┌─────────────────┐
              │ 1. ProcessRequest│
              └────────┬────────┘
                       │
                       ▼
              ┌─────────────────┐
              │ 2. Get Capabilities│ ← AgentCatalog.FormatForLLM()
              └────────┬────────┘
                       │
                       ▼
              ┌─────────────────┐
              │ 3. Generate Plan │ ← LLM creates execution plan
              └────────┬────────┘
                       │
                       ▼
              ┌─────────────────┐
              │ 4. Validate Plan │ ← Check tools exist
              └────────┬────────┘
                       │
                       ▼
              ┌─────────────────┐
              │ 5. Execute Plan  │ ← SmartExecutor runs steps
              └────────┬────────┘
                       │
                       ▼
              ┌─────────────────┐
              │ 6. Synthesize    │ ← AISynthesizer combines results
              └────────┬────────┘
                       │
                       ▼
              Response: Weather summary
```

### Workflow DAG Example

The `travel-research` workflow defines dependencies:

```
geocode ──┬──► weather
          │
          │    country-info ──► currency
          │
          └──► news (parallel)
```

Steps without dependencies run in parallel for faster execution.

## Files

| File | Description |
|------|-------------|
| `main.go` | Entry point with framework setup |
| `research_agent.go` | TravelResearchAgent with workflows |
| `handlers.go` | HTTP handlers for API endpoints |
| `go.mod` | Go module dependencies |
| `.env.example` | Environment variable template |
| `Dockerfile` | Container build configuration |
| `k8-deployment.yaml` | Kubernetes deployment manifest |
| `setup.sh` | One-click setup script |

## Observability

The agent integrates with GoMind telemetry:

- **Traces**: Follow requests across tool boundaries
- **Metrics**: Track orchestrator performance
- **Logs**: Structured logging with correlation IDs

View in Grafana:
```bash
kubectl port-forward -n gomind-examples svc/grafana 3000:80
# Open http://localhost:3000
```

### AI Module Distributed Tracing

This example demonstrates full AI telemetry integration. When you view traces in Jaeger, you'll see:

- **`ai.generate_response`** spans for each AI call with token usage and model info
- **`ai.http_attempt`** spans showing HTTP-level details and retry behavior

**Critical: Initialization Order**

The telemetry module MUST be initialized BEFORE creating the AI client. This example follows the correct order in `main.go`:

```go
func main() {
    // 1. Set component type
    core.SetCurrentComponentType(core.ComponentTypeAgent)

    // 2. Initialize telemetry BEFORE agent creation
    initTelemetry("travel-research-orchestration")
    defer telemetry.Shutdown(context.Background())

    // 3. Create agent AFTER telemetry
    agent, err := NewTravelResearchAgent()
}
```

### Viewing AI Traces

1. Port-forward Jaeger: `kubectl port-forward -n gomind-examples svc/jaeger-query 16686:80`
2. Open: `http://localhost:16686`
3. Select service: `travel-research-orchestration`
4. Find a trace and expand it to see `ai.generate_response` and `ai.http_attempt` spans

## Troubleshooting

### "Orchestrator not initialized"
Ensure Redis is running and tools are discovered:
```bash
redis-cli keys "gomind:*"
```

### "AI client not configured"
Set an AI provider API key:
```bash
export OPENAI_API_KEY=sk-your-key
```

### "Tool not found in catalog"
Deploy the required travel tools first:
```bash
kubectl apply -f ../geocoding-tool/k8-deployment.yaml
kubectl apply -f ../weather-tool-v2/k8-deployment.yaml
# etc.
```

## Related Examples

- **agent-example** - Basic agent with tool discovery
- **agent-with-resilience** - Resilience patterns (circuit breakers, retries)
- **agent-with-telemetry** - Distributed tracing and metrics
- **workflow-example** - Pure workflow execution without orchestration

## TODO

- [ ] **YAML Workflow Support** - Replace hardcoded Go workflow definitions with YAML files using the framework's `WorkflowEngine.ParseWorkflowYAML()`. This would demonstrate the declarative workflow feature documented in the orchestration module.
