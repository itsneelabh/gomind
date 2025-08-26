# Getting Started with GoMind

This guide will take you from zero to a running multi-agent system in 15 minutes.

## Prerequisites Check

Before you start, ensure you have:

```bash
# Check Go version (need 1.22+)
go version

# Check Docker (for Redis)
docker --version

# Optional: Check Kubernetes
kubectl version --client
```

If you're missing any of these:
- **Go**: Download from [golang.org](https://golang.org/dl/)
- **Docker**: Get [Docker Desktop](https://www.docker.com/products/docker-desktop)
- **Kubernetes**: Use [kind](https://kind.sigs.k8s.io/) for local testing

## Understanding the Core Concepts

Before diving into code, understand these 4 key concepts:

1. **Agents**: Autonomous Go services that do one thing well
2. **Capabilities**: Functions that agents expose (auto-discovered via comments)
3. **Discovery**: How agents find each other (via Redis)
4. **Communication**: Agents talk in plain English, not APIs

## Step 1: Set Up Your Development Environment

### Create a new project
```bash
mkdir my-agent-system
cd my-agent-system
go mod init my-agent-system
```

### Install GoMind
```bash
go get github.com/itsneelabh/gomind
```

### Start Redis (required for agent discovery)
```bash
# Using Docker
docker run -d --name redis -p 6379:6379 redis:7-alpine

# Verify Redis is running
docker exec -it redis redis-cli ping
# Should return: PONG
```

## Step 2: Build Your First Agent

Create `calculator/main.go`:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "strconv"
    "strings"
    
    framework "github.com/itsneelabh/gomind"
)

// CalculatorAgent performs mathematical operations
type CalculatorAgent struct {
    framework.BaseAgent
}

// @capability: calculate
// @description: Performs mathematical calculations
// @input: expression string "Mathematical expression like '2 + 2' or '10 * 5'"
// @output: result string "The calculated result"
func (c *CalculatorAgent) Calculate(ctx context.Context, expression string) (string, error) {
    // Simple parser for demonstration
    parts := strings.Fields(expression)
    if len(parts) != 3 {
        return "", fmt.Errorf("invalid expression format")
    }
    
    a, _ := strconv.ParseFloat(parts[0], 64)
    b, _ := strconv.ParseFloat(parts[2], 64)
    
    var result float64
    switch parts[1] {
    case "+":
        result = a + b
    case "-":
        result = a - b
    case "*":
        result = a * b
    case "/":
        if b == 0 {
            return "", fmt.Errorf("division by zero")
        }
        result = a / b
    default:
        return "", fmt.Errorf("unsupported operation: %s", parts[1])
    }
    
    return fmt.Sprintf("%.2f", result), nil
}

// ProcessRequest handles natural language requests
func (c *CalculatorAgent) ProcessRequest(ctx context.Context, request string) (string, error) {
    // Extract math expression from natural language
    // "What is 5 plus 3?" -> "5 + 3"
    processed := strings.ReplaceAll(request, "plus", "+")
    processed = strings.ReplaceAll(processed, "minus", "-")
    processed = strings.ReplaceAll(processed, "times", "*")
    processed = strings.ReplaceAll(processed, "divided by", "/")
    
    // Find the expression
    if strings.Contains(processed, "What is") {
        processed = strings.TrimPrefix(processed, "What is ")
        processed = strings.TrimSuffix(processed, "?")
        processed = strings.TrimSpace(processed)
    }
    
    return c.Calculate(ctx, processed)
}

func main() {
    // Create the agent
    agent := &CalculatorAgent{}
    
    // Run with framework - this handles EVERYTHING:
    // - Service discovery registration
    // - HTTP server setup
    // - Health checks
    // - Graceful shutdown
    // - Observability
    if err := framework.RunAgent(agent,
        framework.WithAgentName("calculator"),
        framework.WithPort(8080),
        framework.WithRedisURL("redis://localhost:6379"),
    ); err != nil {
        log.Fatal(err)
    }
}
```

### Run your agent
```bash
go run calculator/main.go
```

You should see:
```
[INFO] Initializing agent: calculator
[INFO] Connected to Redis for service discovery
[INFO] Agent registered with capabilities: [calculate]
[INFO] HTTP server starting on :8080
[INFO] Agent ready to receive requests
```

### Test your agent
```bash
# Check if agent is registered
curl http://localhost:8080/health

# See discovered capabilities
curl http://localhost:8080/capabilities

# Test the calculation capability
curl -X POST http://localhost:8080/process \
  -H "Content-Type: text/plain" \
  -d "What is 10 plus 5?"
  
# Response: 15.00
```

## Step 3: Create a Second Agent That Uses the First

Create `assistant/main.go`:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "strings"
    
    framework "github.com/itsneelabh/gomind"
)

type AssistantAgent struct {
    framework.BaseAgent
}

// ProcessRequest handles user queries and delegates to other agents
func (a *AssistantAgent) ProcessRequest(ctx context.Context, request string) (string, error) {
    request = strings.ToLower(request)
    
    // Determine what the user needs
    if strings.Contains(request, "calculate") || 
       strings.Contains(request, "what is") ||
       strings.Contains(request, "plus") ||
       strings.Contains(request, "minus") {
        
        // Discover and call the calculator agent
        response := a.AskAgent("calculator", request)
        return fmt.Sprintf("The answer is: %s", response), nil
    }
    
    return "I can help you with calculations. Try asking 'What is 5 plus 3?'", nil
}

func main() {
    agent := &AssistantAgent{}
    
    if err := framework.RunAgent(agent,
        framework.WithAgentName("assistant"),
        framework.WithPort(8081), // Different port
        framework.WithRedisURL("redis://localhost:6379"),
    ); err != nil {
        log.Fatal(err)
    }
}
```

### Run the assistant (in a new terminal)
```bash
go run assistant/main.go
```

### Test multi-agent communication
```bash
# The assistant will automatically discover and use the calculator
curl -X POST http://localhost:8081/process \
  -H "Content-Type: text/plain" \
  -d "Can you calculate 100 divided by 4 for me?"

# Response: The answer is: 25.00
```

## Step 4: Add Observability

### Enable OpenTelemetry tracing

Update your agent initialization:

```go
framework.RunAgent(agent,
    framework.WithAgentName("calculator"),
    framework.WithPort(8080),
    framework.WithRedisURL("redis://localhost:6379"),
    framework.WithOTELEndpoint("http://localhost:4317"), // Add this
    framework.WithMetricsEnabled(true),
)
```

### Run Jaeger for trace visualization
```bash
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  jaegertracing/all-in-one:latest
```

### View traces
Open http://localhost:16686 and you'll see:
- Request flow across agents
- Latency for each operation
- Errors and retries
- Correlation IDs linking everything

## Step 5: Deploy to Kubernetes (Production)

### Create agent container

Create `calculator/Dockerfile`:
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o agent calculator/main.go

FROM alpine:3.18
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/agent .
CMD ["./agent"]
```

### Build and push
```bash
docker build -t myregistry/calculator-agent:v1 -f calculator/Dockerfile .
docker push myregistry/calculator-agent:v1
```

### Deploy to Kubernetes

Create `k8s/calculator-agent.yaml`:
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: agents
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: calculator-agent
  namespace: agents
spec:
  replicas: 3
  selector:
    matchLabels:
      app: calculator-agent
  template:
    metadata:
      labels:
        app: calculator-agent
    spec:
      containers:
      - name: agent
        image: myregistry/calculator-agent:v1
        ports:
        - containerPort: 8080
        env:
        - name: REDIS_URL
          value: "redis://redis.redis.svc.cluster.local:6379"
        - name: OTEL_ENDPOINT
          value: "http://otel-collector.monitoring:4317"
        - name: AGENT_NAME
          value: "calculator"
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 3
          periodSeconds: 5
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "500m"
---
apiVersion: v1
kind: Service
metadata:
  name: calculator-agent
  namespace: agents
spec:
  selector:
    app: calculator-agent
  ports:
  - port: 80
    targetPort: 8080
```

### Deploy
```bash
kubectl apply -f k8s/calculator-agent.yaml

# Check deployment
kubectl get pods -n agents
kubectl logs -n agents -l app=calculator-agent
```

## Common Patterns

### Pattern 1: Capability Chaining
```go
func (a *AnalysisAgent) AnalyzePortfolio(ctx context.Context, portfolio string) (string, error) {
    // Chain multiple agent capabilities
    marketData := a.AskAgent("market-data", fmt.Sprintf("Get prices for %s", portfolio))
    riskScore := a.AskAgent("risk-calculator", fmt.Sprintf("Calculate risk for: %s", marketData))
    report := a.AskAgent("report-generator", fmt.Sprintf("Generate report: Risk=%s", riskScore))
    
    return report, nil
}
```

### Pattern 2: Parallel Agent Calls
```go
func (o *OrchestratorAgent) ProcessComplex(ctx context.Context, data string) (string, error) {
    // Call multiple agents in parallel
    results := make(chan string, 3)
    errors := make(chan error, 3)
    
    go func() {
        res := o.AskAgent("analyzer", data)
        results <- res
    }()
    
    go func() {
        res := o.AskAgent("validator", data)
        results <- res
    }()
    
    go func() {
        res := o.AskAgent("enricher", data)
        results <- res
    }()
    
    // Collect results
    var responses []string
    for i := 0; i < 3; i++ {
        select {
        case res := <-results:
            responses = append(responses, res)
        case err := <-errors:
            return "", err
        case <-time.After(5 * time.Second):
            return "", fmt.Errorf("timeout waiting for agents")
        }
    }
    
    return strings.Join(responses, "\n"), nil
}
```

### Pattern 3: Fallback Handling
```go
func (a *Agent) SafeCalculate(ctx context.Context, expr string) string {
    // Try primary calculator
    result := a.AskAgent("calculator", expr)
    if result == "" || strings.Contains(result, "error") {
        // Fallback to backup calculator
        result = a.AskAgent("backup-calculator", expr)
        if result == "" {
            // Final fallback
            return "Calculation service temporarily unavailable"
        }
    }
    return result
}
```

## Debugging Tips

### 1. Check Agent Registration
```bash
# See all registered agents
redis-cli KEYS "agents:*"

# Get details of specific agent
redis-cli HGETALL "agents:calculator"
```

### 2. Enable Debug Logging
```go
framework.RunAgent(agent,
    framework.WithLogLevel("DEBUG"),
)
```

### 3. Monitor Redis Commands
```bash
redis-cli MONITOR
```

### 4. Test Inter-Agent Communication Manually
```bash
# Simulate agent communication
curl -X POST http://localhost:8080/api/message \
  -H "Content-Type: application/json" \
  -d '{"from": "test", "message": "What is 2+2?"}'
```

## Production Checklist

Before going to production, ensure:

- [ ] **Resource Limits**: Set appropriate CPU/memory limits
- [ ] **Health Checks**: Implement proper liveness/readiness probes  
- [ ] **Observability**: Configure tracing and metrics
- [ ] **Error Handling**: Add circuit breakers for external calls
- [ ] **Security**: Use TLS for Redis in production
- [ ] **Scaling**: Configure HPA based on metrics
- [ ] **Persistence**: Use Redis persistence or cluster mode
- [ ] **Monitoring**: Set up alerts for agent failures
- [ ] **Documentation**: Document each agent's capabilities
- [ ] **Testing**: Load test inter-agent communication

## Troubleshooting

### Agent not discovering others
```bash
# Check Redis connectivity
redis-cli ping

# Check agent registration
redis-cli HGETALL "agents:registry"

# Check network policies (k8s)
kubectl describe networkpolicy -n agents
```

### High latency between agents
```bash
# Check pod placement
kubectl get pods -o wide -n agents

# Check service endpoints
kubectl get endpoints -n agents

# Enable trace sampling
export OTEL_TRACES_SAMPLER_ARG=1.0
```

### Memory issues
```go
// Add memory profiling
import _ "net/http/pprof"

// Then profile at http://localhost:6060/debug/pprof/
```

## Next Steps

1. **Explore Examples**: Check out `/examples` for more complex patterns
2. **Read Architecture Guide**: Understand the [system design](framework_capabilities_guide.md)
3. **Try Orchestration**: Build complex workflows with the orchestrator pattern
4. **Add AI Integration**: Connect your agents to OpenAI or Anthropic
5. **Build Your System**: Start replacing microservices with intelligent agents

## Getting Help

- Check the [API Reference](API.md) for detailed documentation
- Look at [example implementations](../examples/) for common patterns
- Open an [issue](https://github.com/itsneelabh/gomind/issues) for bugs
- Read the [architecture guide](framework_capabilities_guide.md) for deep-dives

---

**Remember**: GoMind handles the distributed systems complexity. Focus on your agent logic.