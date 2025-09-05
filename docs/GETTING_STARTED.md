# Getting Started with GoMind

Build your first AI agent in 5 minutes, then scale to production.

## Prerequisites

```bash
# Required: Go 1.21+
go version

# Required: Docker (for Redis)
docker --version
```

## Installation

```bash
# Create your project
mkdir my-agent && cd my-agent
go mod init my-agent

# Install GoMind
go get github.com/itsneelabh/gomind/core@latest

# Start Redis (required for agent discovery registry)
docker run -d --name redis -p 6379:6379 redis:7-alpine
```

## Your First Agent (2 minutes)

Create `main.go`:

```go
package main

import (
    "log"
    "github.com/itsneelabh/gomind/core"
)

func main() {
    // 1. Create an agent
    agent := core.NewBaseAgent("hello-agent")
    
    // 2. Tell it what it can do
    agent.RegisterCapability(core.Capability{
        Name:        "greet",
        Description: "Says hello",
    })
    
    // 3. Run it
    log.Printf("Starting agent on :8080")
    if err := agent.Start(8080); err != nil {
        log.Fatal(err)
    }
}
```

Run it:

```bash
go run main.go
# Output: Starting agent on :8080
```

Test it:

```bash
# Check health
curl http://localhost:8080/health
# Output: {"status":"healthy","agent":"hello-agent"}

# See capabilities
curl http://localhost:8080/api/capabilities
# Output: [{"name":"greet","description":"Says hello"}]
```

🎉 **Congratulations!** You have a running agent. Let's make it do something useful.

## Building a Real Agent (5 minutes)

Let's create a calculator agent that actually works.

Create `calculator.go`:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strings"
    
    "github.com/itsneelabh/gomind/core"
)

type CalculatorAgent struct {
    *core.BaseAgent
}

func main() {
    // Create calculator agent
    agent := core.NewBaseAgent("calculator")
    
    // Connect to agent discovery registry (optional but recommended)
    discovery, err := core.NewRedisDiscovery("redis://localhost:6379")
    if err != nil {
        log.Printf("Warning: No discovery registry, agent running standalone")
    } else {
        agent.Discovery = discovery
        log.Printf("Agent registered in discovery registry")
    }
    
    // Register this agent's capabilities
    agent.RegisterCapability(core.Capability{
        Name:        "add",
        Description: "Addition capability",
    })
    
    agent.RegisterCapability(core.Capability{
        Name:        "multiply",
        Description: "Multiplication capability",
    })
    
    // Add custom HTTP handler for calculations
    // Note: We should ideally register this with the agent's handler
    agent.RegisterCapability(core.Capability{
        Name:        "calculate",
        Description: "Performs calculations",
        Endpoint:    "/calculate",
        Handler:     handleCalculate,
    })
    
    // Start agent
    log.Printf("Calculator agent starting on :8080")
    if err := agent.Start(8080); err != nil {
        log.Fatal(err)
    }
}

func handleCalculate(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req struct {
        Operation string  `json:"operation"`
        A         float64 `json:"a"`
        B         float64 `json:"b"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    var result float64
    switch strings.ToLower(req.Operation) {
    case "add":
        result = req.A + req.B
    case "multiply":
        result = req.A * req.B
    default:
        http.Error(w, "Unknown operation", http.StatusBadRequest)
        return
    }
    
    json.NewEncoder(w).Encode(map[string]float64{
        "result": result,
    })
}
```

Test it:

```bash
# Run the calculator
go run calculator.go

# Calculate something
curl -X POST http://localhost:8080/calculate \
  -H "Content-Type: application/json" \
  -d '{"operation":"add","a":10,"b":5}'
# Output: {"result":15}
```

## Multi-Agent Coordination (5 minutes)

Now let's create an assistant agent that discovers and coordinates with the calculator agent.

Create `assistant.go`:

```go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    
    "github.com/itsneelabh/gomind/core"
)

type Assistant struct {
    *core.BaseAgent
    discovery core.Discovery
}

func main() {
    // Create assistant agent on different port
    agent := core.NewBaseAgent("assistant")
    agent.Config.Port = 8081
    
    // Setup discovery to find other agents
    discovery, err := core.NewRedisDiscovery("redis://localhost:6379")
    if err != nil {
        log.Fatal("Redis required for multi-agent: ", err)
    }
    agent.Discovery = discovery
    
    assistant := &Assistant{
        BaseAgent: agent,
        discovery: discovery,
    }
    
    // Register this assistant agent's capability
    agent.RegisterCapability(core.Capability{
        Name:        "solve_math",
        Description: "Coordinates with calculator agent to solve problems",
    })
    
    // Register the solve capability with custom handler
    agent.RegisterCapability(core.Capability{
        Name:        "solve",
        Description: "Solves problems using other agents",
        Endpoint:    "/solve",
        Handler:     assistant.handleSolve,
    })
    
    // Start on configured port
    log.Printf("Assistant starting on :%d", config.Port)
    if err := agent.Start(config.Port); err != nil {
        log.Fatal(err)
    }
}

func (a *Assistant) handleSolve(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Problem string `json:"problem"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Find calculator agent
    ctx := context.Background()
    agents, err := a.discovery.FindByCapability(ctx, "add")
    if err != nil || len(agents) == 0 {
        http.Error(w, "Calculator not available", http.StatusServiceUnavailable)
        return
    }
    
    // Connect to the first available calculator agent
    calculatorAgent := agents[0]
    calcURL := fmt.Sprintf("http://%s:%d/calculate", calculatorAgent.Address, calculatorAgent.Port)
    
    // For demo, we'll just add 10 + 20
    payload := map[string]interface{}{
        "operation": "add",
        "a":         10,
        "b":         20,
    }
    
    body, _ := json.Marshal(payload)
    resp, err := http.Post(calcURL, "application/json", bytes.NewBuffer(body))
    if err != nil {
        http.Error(w, "Failed to call calculator", http.StatusInternalServerError)
        return
    }
    defer resp.Body.Close()
    
    var result map[string]float64
    json.NewDecoder(resp.Body).Decode(&result)
    
    response := map[string]interface{}{
        "problem": req.Problem,
        "answer":  result["result"],
        "solved_by": calculatorAgent.ID,  // Which agent helped
    }
    
    json.NewEncoder(w).Encode(response)
}
```

Test multi-agent coordination:

```bash
# Terminal 1: Run calculator
go run calculator.go

# Terminal 2: Run assistant
go run assistant.go

# Terminal 3: Test
curl -X POST http://localhost:8081/solve \
  -H "Content-Type: application/json" \
  -d '{"problem":"What is 10 + 20?"}'
# Output: {"answer":30,"problem":"What is 10 + 20?","solved_by":"calculator-agent-abc123"}
```

## Adding Production Features to Your Agents

### 1. Resilience (Circuit Breakers)

```go
import "github.com/itsneelabh/gomind/resilience"

// Wrap external calls with circuit breaker
cb := resilience.NewCircuitBreaker(resilience.DefaultConfig())

err := cb.Execute(ctx, func() error {
    // Your risky operation
    return callExternalAPI()
})
```

### 2. Observability (Metrics)

```go
import "github.com/itsneelabh/gomind/telemetry"

// Initialize once
telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileProduction))

// Use anywhere
telemetry.Counter("requests.processed", "agent", "calculator")
telemetry.Histogram("response.time", 234.5, "endpoint", "/calculate")
```

### 3. AI Integration

```go
import "github.com/itsneelabh/gomind/ai"

// Create intelligent agent (has tool discovery capabilities)
agent := ai.NewIntelligentAgent("ai-assistant", os.Getenv("OPENAI_API_KEY"))

// Use the intelligent agent's tool discovery
response, err := agent.DiscoverAndUseTools(ctx, "Explain quantum computing")
```

### 4. Orchestration

```go
import "github.com/itsneelabh/gomind/orchestration"

// Natural language orchestration
aiClient := ai.NewOpenAIClient(os.Getenv("OPENAI_API_KEY"), nil)
orchestrator := orchestration.NewAIOrchestrator(
    orchestration.DefaultConfig(),
    discovery,
    aiClient,
)

response, err := orchestrator.ProcessRequest(ctx, 
    "Get weather for NYC and analyze if good for outdoor events",
    nil,
)
```

## Deployment

### Docker (Single Binary ~16MB)

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o agent .

# Runtime stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/agent .
EXPOSE 8080
CMD ["./agent"]
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-agent
  labels:
    app: my-agent
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-agent
  template:
    metadata:
      labels:
        app: my-agent
    spec:
      containers:
      - name: agent
        image: my-agent:latest
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: REDIS_URL
          value: "redis://redis:6379"
        - name: GOMIND_AGENT_NAME
          value: "my-agent"
        - name: GOMIND_PORT
          value: "8080"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "10Mi"    # Go agents typically use 10-20MB
            cpu: "50m"       # 0.05 CPU cores
          limits:
            memory: "50Mi"    # Limit to 50MB
            cpu: "200m"      # Allow burst to 0.2 CPU cores
---
apiVersion: v1
kind: Service
metadata:
  name: my-agent
spec:
  selector:
    app: my-agent
  ports:
  - port: 80
    targetPort: 8080
    protocol: TCP
  type: ClusterIP
```

## Common Agent Patterns

### Pattern 1: Agent Discovery

```go
// Register your agent
discovery, _ := core.NewRedisDiscovery("redis://localhost:6379")
agent.Discovery = discovery

// Find other agents
services, _ := discovery.FindByCapability(ctx, "calculate")
for _, service := range services {
    fmt.Printf("Found: %s at %s:%d\n", service.Name, service.Address, service.Port)
}
```

### Pattern 2: Agent Lifecycle Management

```go
import (
    "os"
    "os/signal"
    "syscall"
)

func main() {
    agent := core.NewBaseAgent("my-agent")
    
    // Handle agent shutdown gracefully
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        log.Println("Agent shutting down gracefully...")
        agent.Stop(context.Background())
        os.Exit(0)
    }()
    
    agent.Start(8080)
}
```

### Pattern 3: Inter-Agent Error Handling

```go
// Always check agent discovery registry
discovery, err := core.NewRedisDiscovery(redisURL)
if err != nil {
    log.Printf("Agent discovery not available: %v", err)
    // Agent can run standalone or use mock discovery
    discovery = core.NewMockDiscovery()
}

// Check if required agents are available
agents, err := discovery.FindByCapability(ctx, "capability")
if err != nil {
    return fmt.Errorf("agent discovery failed: %w", err)
}
if len(agents) == 0 {
    return fmt.Errorf("no agents with required capability found")
}
```

## Quick Reference

### Essential Commands

```bash
# Install GoMind for agents
go get github.com/itsneelabh/gomind/core@latest

# Run Redis for agent discovery
docker run -d --name redis -p 6379:6379 redis:7-alpine

# Build your agent
CGO_ENABLED=0 go build -o agent

# Test agent health
curl http://localhost:8080/health

# Dockerize your agent
docker build -t my-agent .
docker run -p 8080:8080 my-agent
```

### Environment Variables

```bash
# Core configuration
GOMIND_AGENT_NAME=my-agent
GOMIND_PORT=8080
REDIS_URL=redis://localhost:6379

# Optional
OPENAI_API_KEY=sk-...
LOG_LEVEL=debug
GOMIND_NAMESPACE=default
```

### Module Imports

```go
// Core (always needed for agents)
import "github.com/itsneelabh/gomind/core"

// Optional modules for agent capabilities
import "github.com/itsneelabh/gomind/ai"          // AI-powered agents
import "github.com/itsneelabh/gomind/resilience"  // Agent resilience
import "github.com/itsneelabh/gomind/telemetry"   // Agent metrics & tracing
import "github.com/itsneelabh/gomind/orchestration" // Multi-agent orchestration
```

## Common Gotchas

### Health endpoint returns 404
**Problem**: `/health` endpoint not working
**Solution**: Health endpoint should work by default with `NewBaseAgent()` after recent fixes

### Custom handlers not working
**Problem**: HTTP handlers registered with `http.HandleFunc` aren't accessible
**Solution**: Use `agent.RegisterCapability()` with `Handler` field:
```go
agent.RegisterCapability(core.Capability{
    Name:     "custom",
    Endpoint: "/custom",
    Handler:  yourHandlerFunc,
})
```

### Can't access AI methods
**Problem**: `agent.GenerateResponse` doesn't exist on IntelligentAgent
**Solution**: Use `agent.DiscoverAndUseTools()` or access via `EnableAI()`:
```go
// For tool discovery
agent := ai.NewIntelligentAgent(name, apiKey)
result, _ := agent.DiscoverAndUseTools(ctx, query)

// For direct AI calls
baseAgent := core.NewBaseAgent("my-agent")
ai.EnableAI(baseAgent, apiKey)
response, _ := baseAgent.AI.GenerateResponse(ctx, prompt, nil)
```

## Troubleshooting

### Agent won't start
```bash
# Check port availability
lsof -i :8080

# Check Redis
redis-cli ping
```

### Can't find other agents
```bash
# Check Redis registrations
redis-cli KEYS "agents:*"

# Enable debug logging
LOG_LEVEL=debug go run main.go
```

### High memory usage
```go
// Add profiling
import _ "net/http/pprof"
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
// Profile: go tool pprof http://localhost:6060/debug/pprof/heap
```

## API Quick Reference

### Creating Agents
```go
// Simple pattern
agent := core.NewBaseAgent("agent-name")

// Or with custom config
config := core.DefaultConfig()
config.Name = "agent-name"
config.Port = 8080
agent := core.NewBaseAgentWithConfig(config)
```

### Registering Capabilities
```go
agent.RegisterCapability(core.Capability{
    Name:        "capability-name",
    Description: "What it does",
    Endpoint:    "/api/endpoint",  // Optional, auto-generated if empty
    Handler:     customHandler,     // Optional custom handler
})
```

### Agent Discovery Registry
```go
// Register this agent in the discovery registry
discovery, _ := core.NewRedisDiscovery("redis://localhost:6379")
agent.Discovery = discovery

// Discover other agents by their capabilities
agents, _ := discovery.FindByCapability(ctx, "calculate")
for _, agent := range agents {
    fmt.Printf("Found %s agent at %s:%d\n", agent.Name, agent.Address, agent.Port)
}
```

### Starting Your Agent
```go
// Initialize agent (registers with discovery)
agent.Initialize(ctx)

// Start the agent's HTTP server
agent.Start(port)  // Blocks until shutdown
```

## Next Steps

**You've learned the basics!** Now explore:

1. **[AI Module](../ai/README.md)** - Build intelligent AI-powered agents
2. **[Orchestration Module](../orchestration/README.md)** - Coordinate multi-agent systems
3. **[Resilience Module](../resilience/README.md)** - Make agents fault-tolerant
4. **[Telemetry Module](../telemetry/README.md)** - Monitor agent behavior and performance
5. **[API Reference](API.md)** - Complete agent framework API

## Need Help?

- 📖 Read module-specific READMEs for deep dives
- 🐛 Report issues on [GitHub](https://github.com/itsneelabh/gomind/issues)
- 💡 Check [examples/](../examples/) for working code

---

**Happy Building!** 🚀 Start simple, add modules as needed, scale to production.