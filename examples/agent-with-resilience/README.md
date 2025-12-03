# Agent with Resilience

This example demonstrates how to add **fault tolerance** to a GoMind agent using the `resilience` module. It showcases circuit breakers, automatic retries with exponential backoff, timeout management, and graceful degradation.

## What This Example Adds

Building on the foundation of `agent-example`, this example adds:

| Feature | Description |
|---------|-------------|
| **Circuit Breakers** | Per-tool circuit breakers that protect against cascading failures |
| **Automatic Retries** | Exponential backoff with jitter using `resilience.RetryWithCircuitBreaker` |
| **Timeout Management** | Per-call timeouts using `cb.ExecuteWithTimeout` |
| **Graceful Degradation** | Returns partial results when some tools fail |
| **Health Monitoring** | Circuit breaker states exposed via `/health` endpoint |

## Quick Start

### Recommended: Run Everything Locally

```bash
./setup.sh run-all
```

This command:
1. Detects and **reuses** existing infrastructure (Redis, services)
2. Only starts what's missing
3. Builds and runs all components:
   - `grocery-store-api` (port 8081) - Mock API with error injection
   - `grocery-tool` (port 8083) - GoMind tool wrapper
   - `research-agent-resilience` (port 8093) - The resilient agent

### All Setup Commands

| Command | Description |
|---------|-------------|
| `./setup.sh run-all` | **Recommended** - Build and run everything locally |
| `./setup.sh run` | Setup and run agent only (assumes dependencies running) |
| `./setup.sh deploy` | Full Kubernetes deployment |
| `./setup.sh forward` | Port-forward K8s services to localhost |
| `./setup.sh test` | Run resilience test scenario |
| `./setup.sh build-all` | Build all components without running |
| `./setup.sh cleanup` | Remove all K8s resources |

### Smart Infrastructure Detection

The `run-all` command intelligently detects existing services:

```bash
# If Redis is already running (local, Docker, or K8s port-forward):
[SUCCESS] Redis available (Docker: gomind-redis)

# If grocery-store-api is already running:
[SUCCESS] grocery-store-api already available on port 8081
[INFO] Using existing grocery-store-api on port 8081
```

This means you can:
- Run multiple examples sharing the same Redis
- Use `./setup.sh forward` first, then `run-all` reuses the K8s services
- Mix local and K8s deployments seamlessly

### Manual Setup

```bash
# 1. Start Redis (if not running)
docker run -d --name gomind-redis -p 6379:6379 redis:7-alpine

# 2. Set environment variables
export REDIS_URL="redis://localhost:6379"
export OPENAI_API_KEY="sk-..."  # Optional for AI features

# 3. Build and run
go build -o research-agent-resilience .
./research-agent-resilience
```

## Framework APIs Used

This example demonstrates proper usage of the GoMind `resilience` module:

```go
// 1. Create circuit breakers using the factory (auto-detects telemetry)
cb, err := resilience.CreateCircuitBreaker("tool-name", resilience.ResilienceDependencies{
    Logger: agent.Logger,
})

// 2. Use combined retry + circuit breaker pattern
err := resilience.RetryWithCircuitBreaker(ctx, resilience.DefaultRetryConfig(), cb, func() error {
    return makeToolCall()
})

// 3. Use timeout with circuit breaker
err := cb.ExecuteWithTimeout(ctx, 10*time.Second, func() error {
    return makeToolCall()
})

// 4. Monitor circuit breaker state
state := cb.GetState()     // "closed", "open", or "half-open"
metrics := cb.GetMetrics() // Comprehensive metrics map

// 5. Listen to state changes
cb.AddStateChangeListener(func(name string, from, to resilience.CircuitState) {
    log.Printf("Circuit %s: %s -> %s", name, from, to)
})
```

## Circuit Breaker Behavior

```
   CLOSED ─────────────────────────────────► OPEN
     │     (Error rate > 50% with 10+ requests)   │
     │                                             │
     │                                      (Wait 30s)
     │                                             │
     │                                             ▼
     │                                        HALF-OPEN
     │                                             │
     │     ◄───────────────────────────────────────┤
     │     (60% success in test requests)          │
     │                                             │
     └─────────────────────────────────────────────┘
                    (Success rate < 60%)
```

**Default Configuration** (from `resilience.DefaultConfig()`):

| Setting | Value | Description |
|---------|-------|-------------|
| ErrorThreshold | 0.5 | Open circuit at 50% error rate |
| VolumeThreshold | 10 | Minimum requests before evaluation |
| SleepWindow | 30s | Wait before entering half-open |
| HalfOpenRequests | 5 | Test requests in half-open state |
| SuccessThreshold | 0.6 | 60% success to close circuit |
| WindowSize | 60s | Sliding window for metrics |

## API Endpoints

### POST /api/capabilities/research_topic

Resilient research with automatic circuit breaker protection.

```bash
curl -X POST http://localhost:8093/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "weather in New York",
    "use_ai": true
  }'
```

**Response with Partial Success:**
```json
{
  "topic": "weather in New York",
  "summary": "Found 2 results (1 successful, 1 failed)",
  "tools_used": ["weather-service"],
  "results": [...],
  "partial": true,
  "failed_tools": ["stock-service"],
  "success_rate": 0.5,
  "processing_time": "1.234s"
}
```

### GET /health

Health check with circuit breaker states.

```bash
curl http://localhost:8093/health
```

**Response:**
```json
{
  "status": "healthy",
  "timestamp": 1700000000,
  "redis": "healthy",
  "ai_provider": "connected",
  "circuit_breakers": {
    "weather-service": {
      "name": "weather-service",
      "state": "closed",
      "error_rate": 0.1,
      "total": 50,
      "success": 45,
      "failure": 5
    }
  },
  "resilience": {
    "enabled": true,
    "circuit_breakers": 2,
    "retry_config": {
      "max_attempts": 3,
      "initial_delay": "100ms",
      "max_delay": "5s",
      "backoff_factor": 2,
      "jitter_enabled": true
    }
  }
}
```

### GET /api/capabilities/discover_tools

Discover tools with circuit breaker status.

```bash
curl http://localhost:8093/api/capabilities/discover_tools
```

## Changes from agent-example

### research_agent.go

```diff
type ResearchAgent struct {
    *core.BaseAgent
    aiClient        core.AIClient
    httpClient      *http.Client
+   circuitBreakers map[string]*resilience.CircuitBreaker
+   retryConfig     *resilience.RetryConfig
+   cbMutex         sync.RWMutex
}

func NewResearchAgent() (*ResearchAgent, error) {
    // ... existing code ...
+   circuitBreakers: make(map[string]*resilience.CircuitBreaker),
+   retryConfig:     resilience.DefaultRetryConfig(),
}

+ // Use framework factory for CB creation
+ func (r *ResearchAgent) getOrCreateCircuitBreaker(toolName string) *resilience.CircuitBreaker {
+     cb, err := resilience.CreateCircuitBreaker(toolName, resilience.ResilienceDependencies{
+         Logger: r.Logger,
+     })
+     // ...
+ }
```

### orchestration.go

```diff
- func (r *ResearchAgent) callToolWithIntelligentRetry(...) *ToolResult {
-     // Custom retry logic with manual backoff
- }

+ func (r *ResearchAgent) callToolWithResilience(...) *ToolResult {
+     cb := r.getOrCreateCircuitBreaker(tool.Name)
+
+     // Use framework's combined retry + circuit breaker
+     err := resilience.RetryWithCircuitBreaker(ctx, r.retryConfig, cb, func() error {
+         result, callErr = r.callToolDirect(ctx, tool, capability, topic)
+         return callErr
+     })
+ }
```

### handlers.go

```diff
func (r *ResearchAgent) handleHealth(w http.ResponseWriter, req *http.Request) {
    health := map[string]interface{}{
        "status": "healthy",
+       "circuit_breakers": r.getCircuitBreakerStates(),
+       "resilience": map[string]interface{}{
+           "enabled": true,
+           "retry_config": r.retryConfig,
+       },
    }

+   // Check if any circuit is open
+   for name, cb := range r.circuitBreakers {
+       if cb.GetState() == "open" {
+           health["status"] = "degraded"
+           health["degraded_reason"] = fmt.Sprintf("Circuit '%s' is open", name)
+       }
+   }
}
```

## Graceful Degradation

When some tools fail, the agent continues with partial results:

```go
for _, tool := range tools {
    result := r.callToolWithResilience(ctx, tool, capability, topic)
    if result.Success {
        successfulResults = append(successfulResults, *result)
    } else {
        failedTools = append(failedTools, tool.Name)
        // Continue processing - don't fail the entire request
    }
}

// Return partial results with metadata
response := ResearchResponse{
    Results:     successfulResults,
    Partial:     len(failedTools) > 0,
    FailedTools: failedTools,
    SuccessRate: float64(len(successfulResults)) / float64(totalTools),
}
```

## Kubernetes Deployment

```bash
# Deploy to Kubernetes
kubectl apply -f k8-deployment.yaml

# Check status
kubectl get pods -n gomind-examples -l app=research-agent-resilience

# View logs
kubectl logs -n gomind-examples -l app=research-agent-resilience -f

# Port forward for local access
kubectl port-forward -n gomind-examples svc/research-agent-resilience 8093:8093
```

## Testing Resilience

This section demonstrates how to test the resilience features using the `grocery-store-api` with error injection capabilities.

### AI-Driven vs Direct Testing

The agent supports two modes for calling tools:

#### Production Mode (AI-Driven)
```bash
# AI analyzes the query and decides which tools to use
curl -X POST http://localhost:8093/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{"topic":"What grocery products are available?","use_ai":true}'
```

The AI will:
1. Discover available tools (weather-service, stock-service, grocery-service)
2. Analyze the query to determine which tools are relevant
3. Call the appropriate tools with resilience protection
4. Generate an intelligent summary with recommendations

**Example Response:**
```json
{
  "topic": "What grocery products are available?",
  "tools_used": ["grocery-service"],
  "ai_analysis": "**Analysis:** 20 products available across 5 categories. Recommendations: Consider diversifying manufacturers...",
  "metadata": {
    "ai_enabled": true,
    "tools_discovered": 3,
    "tools_called": 1,
    "tools_succeeded": 1
  }
}
```

#### Testing Mode (Direct)
```bash
# Explicitly target specific tools - bypasses AI decision-making
curl -X POST http://localhost:8093/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{"topic":"grocery","sources":["grocery-service"],"use_ai":false}'
```

This mode is used in the test scenarios below to:
- Isolate resilience testing from AI variability
- Ensure consistent, reproducible test results
- Directly test circuit breakers and retry logic

### Prerequisites

This example is **fully self-contained**. All required components are included in the GoMind repository:

| Component | Location | Description |
|-----------|----------|-------------|
| `grocery-store-api` | [`../mock-services/grocery-store-api/`](../mock-services/grocery-store-api/) | Mock API with error injection |
| `grocery-tool` | [`../grocery-tool/`](../grocery-tool/) | GoMind tool that proxies to the API |
| `research-agent-resilience` | This directory | The resilient research agent |

**Deploy to Kubernetes:**
```bash
# Option 1: Use setup.sh (builds and deploys everything)
./setup.sh deploy

# Option 2: Manual deployment
# Build Docker images
docker build -t grocery-store-api:latest ../mock-services/grocery-store-api/
docker build -t grocery-tool:latest ../grocery-tool/
docker build -t research-agent-resilience:latest .

# Load to Kind (if using Kind)
kind load docker-image grocery-store-api:latest grocery-tool:latest research-agent-resilience:latest

# Deploy manifests
kubectl apply -f ../mock-services/grocery-store-api/k8-deployment.yaml
kubectl apply -f ../grocery-tool/k8-deployment.yaml
kubectl apply -f k8-deployment.yaml
```

### Error Injection Modes

The `grocery-store-api` supports three error injection modes:

| Mode | Description | Use Case |
|------|-------------|----------|
| `normal` | All requests succeed | Baseline testing |
| `rate_limit` | Returns 429 after N requests | Test rate limit handling & retries |
| `server_error` | Returns 500 with probability | Test circuit breaker opening |

### Admin Endpoints (grocery-store-api)

```bash
# Set error injection mode
curl -X POST http://localhost:8081/admin/inject-error \
  -H "Content-Type: application/json" \
  -d '{"mode":"rate_limit","rate_limit_after":2,"retry_after_secs":5}'

# Check current status
curl http://localhost:8081/admin/status

# Reset to normal mode
curl -X POST http://localhost:8081/admin/reset
```

### Test Scenarios

#### Scenario 1: Normal Operation
```bash
# Ensure normal mode
curl -X POST http://localhost:8081/admin/reset

# Make request through agent
curl -X POST http://localhost:8093/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{"topic":"grocery","sources":["grocery-service"],"use_ai":false}'
```

**Expected Response:**
```json
{
  "success_rate": 1,
  "metadata": {
    "tools_succeeded": 1,
    "resilience_enabled": true
  }
}
```

#### Scenario 2: Rate Limiting & Retries
```bash
# Enable rate limiting (429 after 1 request)
curl -X POST http://localhost:8081/admin/inject-error \
  -H "Content-Type: application/json" \
  -d '{"mode":"rate_limit","rate_limit_after":1}'

# Make multiple requests - observe retries and failures
for i in 1 2 3 4; do
  echo "Request $i:"
  curl -s -X POST http://localhost:8093/api/capabilities/research_topic \
    -H "Content-Type: application/json" \
    -d '{"topic":"grocery","sources":["grocery-service"],"use_ai":false}' | jq '{success_rate, partial, metadata}'
done
```

**Expected Behavior:**
- Request 1: Succeeds (`tools_succeeded: 1`)
- Requests 2-4: Fail after 3 retries (`partial: true`, `tools_succeeded: 0`)

**Agent Logs Show:**
```json
{"error":"max retry attempts (3) exceeded for HTTP 429: RATE_LIMIT_EXCEEDED",
 "message":"Tool call failed after resilience attempts",
 "tool":"grocery-service"}
```

#### Scenario 3: Circuit Breaker Opens
After enough failures, the circuit breaker opens:

```bash
# Check circuit breaker status
curl http://localhost:8093/health | jq '.circuit_breakers["grocery-service"]'
```

**Expected Response:**
```json
{
  "state": "half-open",
  "error_rate": 0.9,
  "success": 1,
  "failure": 9,
  "total": 10
}
```

#### Scenario 4: Recovery
```bash
# Reset to normal mode
curl -X POST http://localhost:8081/admin/reset

# Make recovery request
curl -X POST http://localhost:8093/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{"topic":"grocery","sources":["grocery-service"],"use_ai":false}'
```

**Expected Response:**
```json
{
  "success_rate": 1,
  "metadata": {
    "tools_succeeded": 1
  }
}
```

**Agent Logs Show:**
```json
{"message":"Resilient tool call succeeded",
 "circuit_state":"half-open",
 "tool":"grocery-service"}
```

### Step-by-Step Walkthrough: What Happens Internally

This section explains exactly what happens during a resilience test, helping developers understand the internal mechanics.

#### Step 1: Normal Operation (Circuit Closed)

```bash
# Reset API to normal mode
curl -X POST http://localhost:8081/admin/reset

# Make a request
curl -X POST http://localhost:8093/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{"topic":"What groceries are available?","use_ai":true}'
```

**What happens internally:**
1. Agent receives request → AI analyzes topic → selects `grocery-service`
2. Agent calls `getOrCreateCircuitBreaker("grocery-service")` → returns circuit in `closed` state
3. `RetryWithCircuitBreaker()` wraps the HTTP call
4. HTTP call to grocery-tool succeeds → grocery-tool proxies to grocery-store-api → returns products
5. Circuit breaker records: `success++`, `total++`
6. Response returns with `success_rate: 1`

**Agent log:**
```json
{"message":"Resilient tool call succeeded","tool":"grocery-service","circuit_state":"closed"}
```

#### Step 2: Rate Limiting Triggers Retries

```bash
# Enable rate limiting (429 after 1 request)
curl -X POST http://localhost:8081/admin/inject-error \
  -H "Content-Type: application/json" \
  -d '{"mode":"rate_limit","rate_limit_after":1}'

# Make second request
curl -X POST http://localhost:8093/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{"topic":"List grocery items","use_ai":true}'
```

**What happens internally:**
1. Agent receives request → AI selects `grocery-service`
2. Circuit is `closed` → allows the call
3. **Attempt 1**: HTTP call → grocery-store-api returns `429 Too Many Requests`
4. `RetryWithCircuitBreaker` catches error → waits `100ms` (initial delay)
5. **Attempt 2**: Retry → still `429` → waits `200ms` (backoff × 2)
6. **Attempt 3**: Final retry → still `429` → waits `400ms` (backoff × 2)
7. All 3 attempts exhausted → records failure in circuit breaker
8. Response returns with `partial: true`, `failed_tools: ["grocery-service"]`

**Agent log:**
```json
{"error":"max retry attempts (3) exceeded for HTTP 429: RATE_LIMIT_EXCEEDED",
 "message":"Tool call failed after resilience attempts",
 "tool":"grocery-service",
 "attempts":3,
 "total_duration":"700ms"}
```

#### Step 3: Failures Accumulate, Circuit Opens

After ~10 requests with >50% failure rate:

```bash
# Check circuit breaker status
curl http://localhost:8093/health | jq '.circuit_breakers["grocery-service"]'
```

**What happens internally:**
1. Circuit breaker evaluates: `total >= VolumeThreshold (10)` ✓
2. Calculates error rate: `failures / total = 9/10 = 90%`
3. Error rate `90% > ErrorThreshold (50%)` → **Circuit OPENS**
4. Next requests are immediately rejected without attempting HTTP call
5. Circuit starts `SleepWindow` timer (30 seconds)

**Circuit state:**
```json
{
  "state": "open",
  "error_rate": 0.9,
  "success": 1,
  "failure": 9,
  "total": 10
}
```

#### Step 4: Half-Open State (Recovery Window)

After 30 seconds, circuit enters `half-open`:

**What happens internally:**
1. `SleepWindow` expires → circuit transitions to `half-open`
2. Circuit allows limited test requests (`HalfOpenRequests: 5`)
3. If test requests succeed → circuit closes
4. If test requests fail → circuit re-opens

**Agent log (state change):**
```json
{"message":"Circuit state changed","circuit":"grocery-service","from":"open","to":"half-open"}
```

#### Step 5: Recovery (Circuit Closes)

```bash
# Reset API to normal
curl -X POST http://localhost:8081/admin/reset

# Make recovery request
curl -X POST http://localhost:8093/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{"topic":"What groceries can I buy?","use_ai":true}'
```

**What happens internally:**
1. Circuit is `half-open` → allows the test request
2. HTTP call succeeds → grocery-store-api returns products
3. Circuit evaluates: success rate in half-open = 100%
4. `100% >= SuccessThreshold (60%)` → **Circuit CLOSES**
5. Normal operation resumes

**Agent log:**
```json
{"message":"Circuit state changed","circuit":"grocery-service","from":"half-open","to":"closed"}
{"message":"Resilient tool call succeeded","tool":"grocery-service","circuit_state":"closed"}
```

#### Timeline Summary

| Time | Event | Circuit State | Error Rate |
|------|-------|---------------|------------|
| 0:00 | First request succeeds | closed | 0% |
| 0:01 | Rate limiting enabled | closed | 0% |
| 0:02 | Request 2 fails (3 retries) | closed | 50% |
| 0:03 | Request 3 fails (3 retries) | closed | 67% |
| 0:04 | Request 4 fails (3 retries) | **open** | 75% |
| 0:05 | Requests rejected immediately | open | - |
| 0:35 | Sleep window expires | **half-open** | - |
| 0:36 | Rate limiting disabled | half-open | - |
| 0:37 | Recovery request succeeds | **closed** | 0% |

### Circuit Breaker Lifecycle Proof

The complete lifecycle demonstrates:

| Phase | Circuit State | Error Rate | Behavior |
|-------|---------------|------------|----------|
| Initial | closed | 0% | Normal operation |
| Under Load | closed | <50% | Requests processed, failures recorded |
| Threshold Exceeded | open | >50% | Requests rejected immediately |
| Recovery Window | half-open | - | Limited test requests allowed |
| Recovery Success | closed | reset | Normal operation resumes |

### Viewing Resilience Logs

```bash
# Filter for resilience-related logs
kubectl logs -n gomind-examples deployment/research-agent-resilience --since=5m \
  | grep -E "(circuit|retry|error|fail|succeed)"
```

**Key Log Messages:**
- `"Resilient tool call succeeded"` - Successful call through circuit breaker
- `"Tool call failed after resilience attempts"` - All retries exhausted
- `"max retry attempts (3) exceeded"` - Retry limit reached
- `"circuit_state":"open"` - Circuit breaker is protecting the system

### Testing Configuration

The circuit breaker uses these defaults (from `resilience.DefaultConfig()`):

| Parameter | Value | Effect on Testing |
|-----------|-------|-------------------|
| `VolumeThreshold` | 10 | Need 10+ requests before circuit evaluates |
| `ErrorThreshold` | 0.5 | Circuit opens at 50% error rate |
| `SleepWindow` | 30s | Wait 30s before recovery attempt |
| `MaxRetries` | 3 | Each request retries 3 times internally |

**Tip:** With `rate_limit_after: 1` and 3 retries per request, you need ~4 requests to accumulate 10+ attempts and trigger the circuit breaker.

## Project Structure

```
agent-with-resilience/
├── main.go              # Entry point, framework initialization
├── research_agent.go    # Agent with circuit breaker map
├── handlers.go          # HTTP handlers with resilience
├── orchestration.go     # Tool calls using RetryWithCircuitBreaker
├── go.mod               # Dependencies (includes resilience module)
├── .env.example         # Environment configuration template
├── Dockerfile           # Multi-stage Docker build
├── Makefile             # Build automation
├── k8-deployment.yaml   # Kubernetes manifests
├── setup.sh             # One-click setup script
└── README.md            # This file
```

## What You'll Learn

1. **How to use `resilience.CreateCircuitBreaker()`** for proper dependency injection
2. **How to use `resilience.RetryWithCircuitBreaker()`** for combined retry + CB patterns
3. **How to use `cb.ExecuteWithTimeout()`** for timeout management
4. **How to implement graceful degradation** with partial results
5. **How to monitor circuit states** via `cb.GetMetrics()` and `cb.GetState()`
6. **How to react to state changes** via `cb.AddStateChangeListener()`

## Next Steps

- **Add Telemetry**: Check out `agent-with-telemetry` for observability
- **Combine Both**: Create a production-grade agent with resilience + telemetry
- **Customize Configuration**: Tune circuit breaker thresholds for your use case

## Troubleshooting

### Circuit breaker keeps opening

- Check the error rate threshold (default 50%)
- Increase `VolumeThreshold` if you have low traffic
- Review logs for the actual errors causing failures

### Retries not working

- Ensure the error is retryable (not 401/403)
- Check if circuit breaker is open (will reject immediately)
- Verify timeout isn't too short

### Health endpoint shows "degraded"

- One or more circuit breakers are in "open" state
- Check which service is failing and why
- Wait for `SleepWindow` (30s) for half-open recovery attempt

## License

MIT License - See [LICENSE](../../LICENSE) for details.
