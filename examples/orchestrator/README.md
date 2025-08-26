# Orchestrator Agent Example

This example demonstrates how to build an orchestrator agent that coordinates multi-agent workflows using the GoMind Framework's orchestration capabilities.

## Features

- **Multi-Agent Coordination**: Orchestrates complex workflows across multiple agents
- **Flexible Routing**: Supports autonomous (LLM-based), workflow (YAML-based), and hybrid routing modes
- **Parallel Execution**: Executes independent agent tasks in parallel for better performance
- **Response Synthesis**: Combines responses from multiple agents into coherent results
- **Error Handling**: Includes circuit breakers, retries, and graceful degradation
- **Metrics & Monitoring**: Tracks execution metrics and maintains history

## Architecture

```
┌─────────────┐
│    User     │
└──────┬──────┘
       │
       ▼
┌─────────────────────┐
│  Orchestrator Agent │
├─────────────────────┤
│ • Router            │
│ • Executor          │
│ • Synthesizer       │
└──────┬──────────────┘
       │
       ├─────────────┬─────────────┬─────────────┐
       ▼             ▼             ▼             ▼
┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
│ Agent A  │  │ Agent B  │  │ Agent C  │  │ Agent D  │
└──────────┘  └──────────┘  └──────────┘  └──────────┘
```

## Configuration

### Environment Variables

```bash
# Required
export OPENAI_API_KEY="your-api-key"          # For LLM-based routing and synthesis

# Optional
export REDIS_URL="redis://localhost:6379"     # Redis for service discovery
export ROUTING_MODE="hybrid"                  # autonomous, workflow, or hybrid
export WORKFLOW_PATH="./workflows"            # Path to workflow YAML files
export MAX_CONCURRENCY="5"                    # Max parallel agent executions
export SYNTHESIS_STRATEGY="llm"              # llm, template, or simple
export LOG_LEVEL="INFO"                       # DEBUG, INFO, WARN, ERROR
```

## Routing Modes

### 1. Autonomous Routing
Uses LLM to intelligently decide which agents to call based on the request:
```bash
export ROUTING_MODE="autonomous"
```

### 2. Workflow Routing
Uses predefined YAML workflows for common patterns:
```bash
export ROUTING_MODE="workflow"
```

### 3. Hybrid Routing (Default)
Tries workflow patterns first, falls back to LLM if no match:
```bash
export ROUTING_MODE="hybrid"
```

## Synthesis Strategies

### 1. LLM Synthesis (Default)
Uses AI to create coherent responses from agent results:
```bash
export SYNTHESIS_STRATEGY="llm"
```

### 2. Template Synthesis
Uses predefined templates to format responses:
```bash
export SYNTHESIS_STRATEGY="template"
```

### 3. Simple Synthesis
Concatenates agent responses with basic formatting:
```bash
export SYNTHESIS_STRATEGY="simple"
```

## Running the Example

### Local Development

1. Start Redis:
```bash
docker run -d -p 6379:6379 redis:latest
```

2. Start supporting agents (optional):
```bash
# In separate terminals
go run examples/calculator/main.go
go run examples/weather/main.go
go run examples/database/main.go
```

3. Start the orchestrator:
```bash
go run examples/orchestrator/main.go
```

4. Test the orchestrator:
```bash
# Process a request
curl -X POST http://localhost:8080/process \
  -H "Content-Type: text/plain" \
  -d "Calculate the weather impact on energy consumption"

# Or use the orchestrate endpoint with metadata
curl -X POST http://localhost:8080/orchestrate \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Analyze portfolio risk for tech stocks",
    "metadata": {
      "user_id": "user123",
      "priority": "high"
    }
  }'

# Get metrics
curl http://localhost:8080/orchestrator/metrics

# Get execution history
curl http://localhost:8080/orchestrator/history
```

### Kubernetes Deployment

1. Build and push the Docker image:
```bash
docker build -t your-registry/orchestrator:latest .
docker push your-registry/orchestrator:latest
```

2. Deploy to Kubernetes:
```bash
kubectl apply -f k8s/
```

3. Access the orchestrator:
```bash
kubectl port-forward svc/orchestrator 8080:8080
```

## Example Workflows

### Data Analysis Workflow
```yaml
name: data-analysis
triggers:
  keywords: ["analyze", "data"]
steps:
  - name: fetch-data
    agent: database-agent
    instruction: "Fetch relevant data"
  - name: process-data
    agent: analytics-agent
    instruction: "Process and analyze data"
    depends_on: [fetch-data]
  - name: generate-report
    agent: report-agent
    instruction: "Generate analysis report"
    depends_on: [process-data]
```

### Parallel Search Workflow
```yaml
name: comprehensive-search
triggers:
  patterns: [".*search.*everywhere.*"]
steps:
  - name: search-db
    agent: database-agent
    instruction: "Search database"
    parallel: true
  - name: search-cache
    agent: cache-agent
    instruction: "Search cache"
    parallel: true
  - name: search-api
    agent: api-agent
    instruction: "Search external API"
    parallel: true
  - name: aggregate
    agent: aggregator-agent
    instruction: "Combine all results"
    depends_on: [search-db, search-cache, search-api]
```

## API Endpoints

### Core Endpoints

- `POST /process` - Process natural language request (plain text)
- `POST /orchestrate` - Process request with metadata (JSON)
- `GET /health` - Health check
- `GET /ready` - Readiness check

### Orchestrator-Specific Endpoints

- `GET /orchestrator/metrics` - Get orchestration metrics
- `GET /orchestrator/history` - Get execution history

## Metrics

The orchestrator tracks the following metrics:

- `total_requests` - Total orchestration requests
- `successful_requests` - Successfully completed orchestrations
- `failed_requests` - Failed orchestrations
- `average_latency` - Average orchestration latency
- `median_latency` - Median orchestration latency
- `p99_latency` - 99th percentile latency
- `agent_calls_total` - Total agent invocations
- `agent_calls_failed` - Failed agent invocations
- `synthesis_count` - Total response syntheses
- `synthesis_errors` - Failed syntheses

## Error Handling

### Circuit Breaker
Prevents cascading failures when agents are consistently failing:
```go
config.ExecutionOptions.CircuitBreaker = true
config.ExecutionOptions.FailureThreshold = 5
config.ExecutionOptions.RecoveryTimeout = 30 * time.Second
```

### Retry Logic
Automatically retries failed agent calls:
```go
config.ExecutionOptions.RetryAttempts = 3
config.ExecutionOptions.RetryDelay = 2 * time.Second
```

### Graceful Degradation
Continues with partial results when non-required steps fail:
```yaml
steps:
  - name: optional-enhancement
    agent: enhancement-agent
    required: false  # Failure won't block the workflow
```

## Performance Optimization

### Caching
Caches routing decisions and synthesized responses:
```go
config.CacheEnabled = true
config.CacheTTL = 5 * time.Minute
```

### Parallel Execution
Controls concurrent agent executions:
```go
config.ExecutionOptions.MaxConcurrency = 10
```

### Timeouts
Prevents hanging on slow agents:
```go
config.ExecutionOptions.StepTimeout = 30 * time.Second
config.ExecutionOptions.TotalTimeout = 2 * time.Minute
```

## Troubleshooting

### Debug Logging
Enable debug logs for detailed execution traces:
```bash
export LOG_LEVEL="DEBUG"
```

### Common Issues

1. **No agents found**: Ensure agents are registered in Redis
2. **Routing failures**: Check agent catalog format and availability
3. **Synthesis errors**: Verify OpenAI API key and quota
4. **Timeout errors**: Adjust timeout configurations
5. **Circuit breaker open**: Check agent health and reset if needed

## Advanced Usage

### Custom Synthesis Function
```go
synthesizer.SetCustomSynthesisFunc(func(ctx context.Context, request string, results *ExecutionResult) (string, error) {
    // Custom synthesis logic
    return customSynthesis(results), nil
})
```

### Dynamic Workflow Loading
```go
workflowRouter.AddWorkflow(&routing.WorkflowDefinition{
    Name: "custom-workflow",
    Triggers: routing.WorkflowTriggers{
        Keywords: []string{"custom"},
    },
    Steps: []routing.WorkflowStep{
        // Define steps
    },
})
```

### Monitoring Integration
```go
// Export metrics to Prometheus
http.Handle("/metrics", promhttp.Handler())
```

## Contributing

Contributions are welcome! Areas for improvement:

1. Additional synthesis strategies
2. More sophisticated routing algorithms
3. Enhanced error recovery mechanisms
4. Performance optimizations
5. Additional workflow templates