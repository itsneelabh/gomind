# Getting Started with GoMind

Welcome to GoMind! This guide will walk you through building AI-powered agent systems step by step, from installation to production deployment.

## Table of Contents
- [Prerequisites](#prerequisites)
- [Core Concepts](#core-concepts)
- [Installation](#installation)
- [Quick Start - Your First Agent](#quick-start---your-first-agent)
- [Building Multi-Agent Systems](#building-multi-agent-systems)
- [Adding AI Capabilities](#adding-ai-capabilities)
- [Creating Workflows](#creating-workflows)
- [Production Patterns](#production-patterns)
- [Deployment](#deployment)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)

## Prerequisites

Before you begin, ensure you have:

```bash
# Check Go version (requires 1.21+)
go version

# Check Docker (for Redis and optional services)
docker --version

# Optional: Check for OpenAI API key (for AI features)
echo $OPENAI_API_KEY
```

**Required installations:**
- **Go 1.21+**: Download from [golang.org](https://golang.org/dl/)
- **Docker**: Get [Docker Desktop](https://www.docker.com/products/docker-desktop)
- **Redis**: We'll run it via Docker

**Optional:**
- **OpenAI API Key**: For AI features, get one at [platform.openai.com](https://platform.openai.com)
- **Kubernetes**: Use [kind](https://kind.sigs.k8s.io/) or [minikube](https://minikube.sigs.k8s.io/) for local testing

## Core Concepts

GoMind is built around simple, powerful concepts:

| Concept | Description | Example |
|---------|-------------|---------|
| **Agents** | Autonomous services that perform specific tasks | Calculator agent, Weather agent |
| **Capabilities** | Functions that agents expose to others | "calculate", "get_weather" |
| **Discovery** | How agents find and communicate with each other | Redis-based service registry |
| **Orchestration** | Coordinating multiple agents to achieve complex goals | AI-driven or workflow-based |

## Installation

### Step 1: Create a new Go project

```bash
mkdir my-gomind-project
cd my-gomind-project
go mod init my-gomind-project
```

### Step 2: Install GoMind

```bash
# Install the latest stable version (recommended)
go get github.com/itsneelabh/gomind@v0.3.1

# Or install from main branch (latest features)
# go get github.com/itsneelabh/gomind@main
```

### Step 3: Start Redis (required for discovery)

```bash
# Start Redis using Docker
docker run -d --name redis -p 6379:6379 redis:7-alpine

# Verify Redis is running
docker exec -it redis redis-cli ping
# Should output: PONG
```

## Quick Start - Your First Agent

Let's build a simple calculator agent that can perform mathematical operations.

### Create `main.go`:

```go
package main

import (
    "context"
    "log"
    
    "github.com/itsneelabh/gomind/core"
)

func main() {
    // Create a simple agent with configuration
    config, err := core.NewConfig(
        core.WithName("calculator"),
        core.WithPort(8080),
        core.WithRedisURL("redis://localhost:6379"),
        core.WithCORSDefaults(), // Enable CORS for web clients
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // Create the base agent
    agent := core.NewBaseAgentWithConfig(config)
    
    // Register a capability (what this agent can do)
    agent.RegisterCapability(core.Capability{
        Name:        "add",
        Description: "Adds two numbers",
        Endpoint:    "/api/capabilities/add",
        InputTypes:  []string{"number", "number"},
        OutputTypes: []string{"number"},
    })
    
    // Initialize the agent
    ctx := context.Background()
    if err := agent.Initialize(ctx); err != nil {
        log.Fatal(err)
    }
    
    // Start the agent
    log.Printf("Starting calculator agent on port %d...", config.Port)
    if err := agent.Start(config.Port); err != nil {
        log.Fatal(err)
    }
}
```

### Run your agent:

```bash
go run main.go
```

You should see:
```
2025/01/20 10:00:00 Starting calculator agent on port 8080...
```

### Test your agent:

```bash
# Check if it's healthy
curl http://localhost:8080/health
# Output: {"status":"healthy","agent":"calculator","id":"calculator-abc123"}

# See its capabilities
curl http://localhost:8080/api/capabilities
# Output: [{"name":"add","description":"Adds two numbers",...}]
```

Great! You've created your first agent. But it doesn't actually add numbers yet. Let's fix that.

## Building a Functional Calculator Agent

Now let's create a calculator that actually works. We'll use a more structured approach:

### Create `calculator/main.go`:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "strconv"
    "strings"
    "syscall"
    
    "github.com/itsneelabh/gomind/core"
)

// CalculatorAgent performs mathematical operations
type CalculatorAgent struct {
    *core.BaseAgent
    server *http.Server
}

// NewCalculatorAgent creates a new calculator agent
func NewCalculatorAgent() (*CalculatorAgent, error) {
    // Create configuration
    config, err := core.NewConfig(
        core.WithName("calculator"),
        core.WithPort(8080),
        core.WithRedisURL("redis://localhost:6379"),
        core.WithCORSDefaults(),
        core.WithDevelopmentMode(), // Enable development features
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create config: %w", err)
    }
    
    // Create base agent
    baseAgent := core.NewBaseAgentWithConfig(config)
    
    // Register capabilities
    baseAgent.RegisterCapability(core.Capability{
        Name:        "calculate",
        Description: "Evaluates mathematical expressions",
        Endpoint:    "/api/calculate",
        InputTypes:  []string{"expression"},
        OutputTypes: []string{"result"},
    })
    
    return &CalculatorAgent{
        BaseAgent: baseAgent,
    }, nil
}

// Calculate evaluates a mathematical expression
func (c *CalculatorAgent) Calculate(expression string) (float64, error) {
    // Simple parser for basic operations (production would use a proper parser)
    expression = strings.TrimSpace(expression)
    
    // Try to parse as simple "a op b" format
    var a, b float64
    var op string
    
    // Try different operators
    operators := []string{"+", "-", "*", "/", "plus", "minus", "times", "divided by"}
    found := false
    
    for _, operator := range operators {
        if strings.Contains(expression, operator) {
            parts := strings.Split(expression, operator)
            if len(parts) == 2 {
                var err error
                a, err = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
                if err != nil {
                    continue
                }
                b, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
                if err != nil {
                    continue
                }
                op = operator
                found = true
                break
            }
        }
    }
    
    if !found {
        return 0, fmt.Errorf("invalid expression format")
    }
    
    // Normalize operator
    switch op {
    case "+", "plus":
        return a + b, nil
    case "-", "minus":
        return a - b, nil
    case "*", "times":
        return a * b, nil
    case "/", "divided by":
        if b == 0 {
            return 0, fmt.Errorf("division by zero")
        }
        return a / b, nil
    default:
        return 0, fmt.Errorf("unknown operator: %s", op)
    }
}

// Start initializes and starts the agent with custom HTTP handlers
func (c *CalculatorAgent) Start(ctx context.Context) error {
    // Initialize the base agent
    if err := c.BaseAgent.Initialize(ctx); err != nil {
        return fmt.Errorf("failed to initialize: %w", err)
    }
    
    // Create HTTP mux for custom endpoints
    mux := http.NewServeMux()
    
    // Add health check
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "status": "healthy",
            "agent":  c.BaseAgent.GetName(),
            "id":     c.BaseAgent.GetID(),
        })
    })
    
    // Add capabilities endpoint
    mux.HandleFunc("/api/capabilities", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(c.BaseAgent.GetCapabilities())
    })
    
    // Add calculate endpoint
    mux.HandleFunc("/api/calculate", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }
        
        var request struct {
            Expression string `json:"expression"`
        }
        
        if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
            http.Error(w, "Invalid request", http.StatusBadRequest)
            return
        }
        
        result, err := c.Calculate(request.Expression)
        if err != nil {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusBadRequest)
            json.NewEncoder(w).Encode(map[string]string{
                "error": err.Error(),
            })
            return
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "result":     result,
            "expression": request.Expression,
        })
    })
    
    // Create and start server
    c.server = &http.Server{
        Addr:    fmt.Sprintf(":%d", c.BaseAgent.Config.Port),
        Handler: mux,
    }
    
    log.Printf("Calculator agent starting on port %d", c.BaseAgent.Config.Port)
    return c.server.ListenAndServe()
}

// Stop gracefully shuts down the agent
func (c *CalculatorAgent) Stop(ctx context.Context) error {
    if c.server != nil {
        return c.server.Shutdown(ctx)
    }
    return nil
}

func main() {
    // Create agent
    agent, err := NewCalculatorAgent()
    if err != nil {
        log.Fatal(err)
    }
    
    // Setup graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        log.Println("Shutting down agent...")
        if err := agent.Stop(ctx); err != nil {
            log.Printf("Error during shutdown: %v", err)
        }
        cancel()
    }()
    
    // Start agent
    if err := agent.Start(ctx); err != nil && err != http.ErrServerClosed {
        log.Fatal(err)
    }
}
```

### Test the calculator:

```bash
# Run the agent
go run calculator/main.go

# In another terminal, test it:
curl -X POST http://localhost:8080/api/calculate \
  -H "Content-Type: application/json" \
  -d '{"expression":"10 + 5"}'
# Output: {"expression":"10 + 5","result":15}

curl -X POST http://localhost:8080/api/calculate \
  -H "Content-Type: application/json" \
  -d '{"expression":"100 / 4"}'
# Output: {"expression":"100 / 4","result":25}
```

## Building Multi-Agent Systems

Now let's create an assistant agent that discovers and uses the calculator:

### Create `assistant/main.go`:

```go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "os/signal"
    "strings"
    "syscall"
    
    "github.com/itsneelabh/gomind/core"
)

type AssistantAgent struct {
    *core.BaseAgent
    discovery core.Discovery
    server    *http.Server
}

func NewAssistantAgent() (*AssistantAgent, error) {
    config, err := core.NewConfig(
        core.WithName("assistant"),
        core.WithPort(8081), // Different port from calculator
        core.WithRedisURL("redis://localhost:6379"),
        core.WithCORSDefaults(),
    )
    if err != nil {
        return nil, err
    }
    
    baseAgent := core.NewBaseAgentWithConfig(config)
    
    // Setup discovery
    discovery, err := core.NewRedisDiscovery("redis://localhost:6379")
    if err != nil {
        log.Printf("Warning: Could not connect to Redis, using mock discovery")
        discovery = core.NewMockDiscovery()
    }
    
    // Register capabilities
    baseAgent.RegisterCapability(core.Capability{
        Name:        "assist",
        Description: "Natural language assistance using other agents",
        Endpoint:    "/api/assist",
        InputTypes:  []string{"query"},
        OutputTypes: []string{"response"},
    })
    
    return &AssistantAgent{
        BaseAgent: baseAgent,
        discovery: discovery,
    }, nil
}

func (a *AssistantAgent) CallCalculator(expression string) (float64, error) {
    ctx := context.Background()
    
    // Find calculator service
    services, err := a.discovery.FindByCapability(ctx, "calculate")
    if err != nil {
        return 0, fmt.Errorf("discovery error: %w", err)
    }
    
    if len(services) == 0 {
        return 0, fmt.Errorf("calculator service not available")
    }
    
    // Use first available service
    service := services[0]
    url := fmt.Sprintf("http://%s:%d/api/calculate", service.Address, service.Port)
    
    // Prepare request
    reqBody, _ := json.Marshal(map[string]string{
        "expression": expression,
    })
    
    // Make HTTP call
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
    if err != nil {
        return 0, fmt.Errorf("failed to call calculator: %w", err)
    }
    defer resp.Body.Close()
    
    // Parse response
    body, _ := io.ReadAll(resp.Body)
    var result struct {
        Result float64 `json:"result"`
        Error  string  `json:"error"`
    }
    
    if err := json.Unmarshal(body, &result); err != nil {
        return 0, fmt.Errorf("failed to parse response: %w", err)
    }
    
    if result.Error != "" {
        return 0, fmt.Errorf("calculator error: %s", result.Error)
    }
    
    return result.Result, nil
}

func (a *AssistantAgent) ProcessQuery(query string) (string, error) {
    query = strings.ToLower(query)
    
    // Check if this is a math question
    mathKeywords := []string{"calculate", "compute", "what is", "how much", "solve"}
    isMath := false
    for _, keyword := range mathKeywords {
        if strings.Contains(query, keyword) {
            isMath = true
            break
        }
    }
    
    if isMath {
        // Extract expression from query
        expression := query
        for _, keyword := range mathKeywords {
            expression = strings.ReplaceAll(expression, keyword, "")
        }
        expression = strings.TrimSpace(expression)
        expression = strings.Trim(expression, "?")
        
        // Try to calculate
        result, err := a.CallCalculator(expression)
        if err != nil {
            return fmt.Sprintf("I couldn't calculate that: %v", err), nil
        }
        
        return fmt.Sprintf("The answer is: %.2f", result), nil
    }
    
    // Default response
    return "I can help you with calculations. Try asking 'What is 10 + 5?'", nil
}

func (a *AssistantAgent) Start(ctx context.Context) error {
    // Initialize base agent
    if err := a.BaseAgent.Initialize(ctx); err != nil {
        return err
    }
    
    // Setup HTTP handlers
    mux := http.NewServeMux()
    
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "status": "healthy",
            "agent":  a.BaseAgent.GetName(),
        })
    })
    
    mux.HandleFunc("/api/assist", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }
        
        var request struct {
            Query string `json:"query"`
        }
        
        if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
            http.Error(w, "Invalid request", http.StatusBadRequest)
            return
        }
        
        response, err := a.ProcessQuery(request.Query)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "response": response,
        })
    })
    
    // Start server
    a.server = &http.Server{
        Addr:    fmt.Sprintf(":%d", a.BaseAgent.Config.Port),
        Handler: mux,
    }
    
    log.Printf("Assistant agent starting on port %d", a.BaseAgent.Config.Port)
    return a.server.ListenAndServe()
}

func (a *AssistantAgent) Stop(ctx context.Context) error {
    if a.server != nil {
        return a.server.Shutdown(ctx)
    }
    return nil
}

func main() {
    agent, err := NewAssistantAgent()
    if err != nil {
        log.Fatal(err)
    }
    
    ctx := context.Background()
    
    // Graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        log.Println("Shutting down...")
        agent.Stop(ctx)
        os.Exit(0)
    }()
    
    if err := agent.Start(ctx); err != nil {
        log.Fatal(err)
    }
}
```

### Test multi-agent communication:

```bash
# Terminal 1: Run calculator
go run calculator/main.go

# Terminal 2: Run assistant
go run assistant/main.go

# Terminal 3: Test the assistant
curl -X POST http://localhost:8081/api/assist \
  -H "Content-Type: application/json" \
  -d '{"query":"What is 25 + 75?"}'
# Output: {"response":"The answer is: 100.00"}
```

## Adding AI Capabilities

Let's enhance our assistant with OpenAI integration:

### Create `ai-assistant/main.go`:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    
    "github.com/itsneelabh/gomind/ai"
    "github.com/itsneelabh/gomind/core"
)

type AIAssistant struct {
    *ai.IntelligentAgent
    server *http.Server
}

func NewAIAssistant() (*AIAssistant, error) {
    // Get API key from environment
    apiKey := os.Getenv("OPENAI_API_KEY")
    if apiKey == "" {
        return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set")
    }
    
    // Create intelligent agent with AI capabilities
    intelligentAgent := ai.NewIntelligentAgent("ai-assistant", apiKey)
    
    // Configure the agent
    config, _ := core.NewConfig(
        core.WithName("ai-assistant"),
        core.WithPort(8082),
        core.WithRedisURL("redis://localhost:6379"),
    )
    intelligentAgent.BaseAgent.Config = config
    
    // Register capability
    intelligentAgent.RegisterCapability(core.Capability{
        Name:        "ai_assist",
        Description: "AI-powered assistance using GPT-4",
        Endpoint:    "/api/ai-assist",
        InputTypes:  []string{"prompt"},
        OutputTypes: []string{"response"},
    })
    
    return &AIAssistant{
        IntelligentAgent: intelligentAgent,
    }, nil
}

func (a *AIAssistant) ProcessWithAI(prompt string) (string, error) {
    ctx := context.Background()
    
    // First, try to discover and use relevant tools
    toolResponse, err := a.DiscoverAndUseTools(ctx, prompt)
    if err == nil && toolResponse != "" {
        return toolResponse, nil
    }
    
    // Fallback to direct AI response
    aiOptions := &core.AIOptions{
        Model:        "gpt-4",
        Temperature:  0.7,
        MaxTokens:    500,
        SystemPrompt: "You are a helpful assistant in a multi-agent system.",
    }
    
    response, err := a.AI.GenerateResponse(ctx, prompt, aiOptions)
    if err != nil {
        return "", fmt.Errorf("AI error: %w", err)
    }
    
    return response.Content, nil
}

func (a *AIAssistant) Start(ctx context.Context) error {
    // Initialize agent
    if err := a.Initialize(ctx); err != nil {
        return err
    }
    
    // Setup HTTP handlers
    mux := http.NewServeMux()
    
    mux.HandleFunc("/api/ai-assist", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }
        
        var request struct {
            Prompt string `json:"prompt"`
        }
        
        if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
            http.Error(w, "Invalid request", http.StatusBadRequest)
            return
        }
        
        response, err := a.ProcessWithAI(request.Prompt)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "response": response,
        })
    })
    
    // Start server
    a.server = &http.Server{
        Addr:    fmt.Sprintf(":%d", a.BaseAgent.Config.Port),
        Handler: mux,
    }
    
    log.Printf("AI Assistant starting on port %d", a.BaseAgent.Config.Port)
    return a.server.ListenAndServe()
}

func main() {
    agent, err := NewAIAssistant()
    if err != nil {
        log.Fatal(err)
    }
    
    ctx := context.Background()
    if err := agent.Start(ctx); err != nil {
        log.Fatal(err)
    }
}
```

### Test AI capabilities:

```bash
# Set your OpenAI API key
export OPENAI_API_KEY="your-api-key-here"

# Run the AI assistant
go run ai-assistant/main.go

# Test it
curl -X POST http://localhost:8082/api/ai-assist \
  -H "Content-Type: application/json" \
  -d '{"prompt":"Explain quantum computing in simple terms"}'
```

## Creating Workflows

For complex multi-step processes, use the workflow engine:

### Create a workflow YAML file `workflow.yaml`:

```yaml
name: data-processing
version: "1.0"
description: Process data through multiple agents

inputs:
  data:
    type: string
    required: true
    description: Data to process

steps:
  - name: validate
    agent: validator
    action: validate_data
    inputs:
      data: ${inputs.data}
    timeout: 10s
    
  - name: transform
    agent: transformer  
    action: transform_data
    inputs:
      data: ${steps.validate.output}
    depends_on: [validate]
    
  - name: analyze
    agent: analyzer
    action: analyze_data
    inputs:
      data: ${steps.transform.output}
    depends_on: [transform]
    
  - name: store
    agent: storage
    action: store_result
    inputs:
      result: ${steps.analyze.output}
    depends_on: [analyze]
    retry:
      max_attempts: 3
      backoff: exponential

outputs:
  result: ${steps.store.output}
  status: "completed"

on_error:
  strategy: continue
```

### Execute workflow in code:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/orchestration"
)

func main() {
    // Setup discovery
    discovery, err := core.NewRedisDiscovery("redis://localhost:6379")
    if err != nil {
        log.Fatal(err)
    }
    
    // Create workflow engine
    engine := orchestration.NewWorkflowEngine(discovery)
    
    // Load workflow from file
    yamlData, err := os.ReadFile("workflow.yaml")
    if err != nil {
        log.Fatal(err)
    }
    
    // Parse workflow
    workflow, err := engine.ParseWorkflowYAML(yamlData)
    if err != nil {
        log.Fatal(err)
    }
    
    // Execute workflow
    inputs := map[string]interface{}{
        "data": "sample data to process",
    }
    
    ctx := context.Background()
    execution, err := engine.ExecuteWorkflow(ctx, workflow, inputs)
    if err != nil {
        log.Fatal(err)
    }
    
    // Check results
    fmt.Printf("Workflow completed: %s\n", execution.Status)
    fmt.Printf("Result: %v\n", execution.Outputs["result"])
}
```

## Production Patterns

### 1. Resilience Pattern

Add circuit breakers and retries for production reliability:

```go
import (
    "github.com/itsneelabh/gomind/resilience"
)

func CallServiceWithResilience(url string, data []byte) ([]byte, error) {
    // Create circuit breaker
    cb := resilience.NewCircuitBreaker(5, 30*time.Second)
    
    // Configure retry
    retryConfig := resilience.DefaultRetryConfig()
    retryConfig.MaxAttempts = 3
    retryConfig.InitialDelay = 100 * time.Millisecond
    
    var response []byte
    err := resilience.RetryWithCircuitBreaker(
        context.Background(),
        retryConfig,
        cb,
        func() error {
            resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
            if err != nil {
                return err
            }
            defer resp.Body.Close()
            
            if resp.StatusCode >= 500 {
                return fmt.Errorf("server error: %d", resp.StatusCode)
            }
            
            response, err = io.ReadAll(resp.Body)
            return err
        },
    )
    
    return response, err
}
```

### 2. Observability Pattern

Add distributed tracing for monitoring:

```go
import (
    "github.com/itsneelabh/gomind/telemetry"
)

func SetupTelemetry(serviceName string) (*telemetry.OTelProvider, error) {
    // Create telemetry provider
    provider, err := telemetry.NewOTelProvider(
        serviceName,
        "localhost:4317", // OTLP collector endpoint
    )
    if err != nil {
        return nil, err
    }
    
    return provider, nil
}

// Use in your agent
func (a *MyAgent) ProcessWithTracing(ctx context.Context, data string) error {
    // Start a span
    ctx, span := a.telemetry.StartSpan(ctx, "process_data")
    defer span.End()
    
    // Add attributes
    span.SetAttribute("data.length", len(data))
    span.SetAttribute("agent.name", a.GetName())
    
    // Do work
    result, err := a.process(data)
    if err != nil {
        span.RecordError(err)
        return err
    }
    
    span.SetAttribute("result.size", len(result))
    return nil
}
```

### 3. Configuration Pattern

Use environment variables for production config:

```go
func LoadConfigFromEnv() (*core.Config, error) {
    port := 8080
    if p := os.Getenv("GOMIND_PORT"); p != "" {
        port, _ = strconv.Atoi(p)
    }
    
    redisURL := os.Getenv("REDIS_URL")
    if redisURL == "" {
        redisURL = "redis://localhost:6379"
    }
    
    logLevel := os.Getenv("LOG_LEVEL")
    if logLevel == "" {
        logLevel = "info"
    }
    
    return core.NewConfig(
        core.WithName(os.Getenv("AGENT_NAME")),
        core.WithPort(port),
        core.WithRedisURL(redisURL),
        core.WithLogLevel(logLevel),
        core.WithNamespace(os.Getenv("NAMESPACE")),
    )
}
```

## Deployment

### Docker Deployment

Create a `Dockerfile`:

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o agent .

# Runtime stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/agent .
CMD ["./agent"]
```

Build and run:

```bash
# Build image
docker build -t my-agent:v1 .

# Run container
docker run -d \
  --name my-agent \
  -p 8080:8080 \
  -e REDIS_URL=redis://host.docker.internal:6379 \
  -e OPENAI_API_KEY=$OPENAI_API_KEY \
  my-agent:v1
```

### Kubernetes Deployment

Create `k8s-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: calculator-agent
  namespace: gomind
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
        image: my-registry/calculator-agent:v1
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: AGENT_NAME
          value: "calculator"
        - name: REDIS_URL
          value: "redis://redis.gomind.svc.cluster.local:6379"
        - name: LOG_LEVEL
          value: "info"
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
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
---
apiVersion: v1
kind: Service
metadata:
  name: calculator-agent
  namespace: gomind
spec:
  selector:
    app: calculator-agent
  ports:
  - port: 80
    targetPort: 8080
    name: http
  type: ClusterIP
```

Deploy to Kubernetes:

```bash
# Create namespace
kubectl create namespace gomind

# Deploy Redis first
kubectl apply -n gomind -f redis-deployment.yaml

# Deploy your agent
kubectl apply -f k8s-deployment.yaml

# Check status
kubectl get pods -n gomind
kubectl logs -n gomind -l app=calculator-agent
```

## Troubleshooting

### Common Issues and Solutions

#### 1. Agent not registering with Redis

**Problem:** Agent starts but doesn't appear in discovery.

**Solution:**
```bash
# Check Redis connectivity
redis-cli ping

# Check registrations
redis-cli KEYS "agents:*"

# Debug mode
export LOG_LEVEL=debug
go run main.go
```

#### 2. Cannot find other agents

**Problem:** Agents can't discover each other.

**Solution:**
```go
// Add debug logging
services, err := discovery.FindByCapability(ctx, "calculate")
log.Printf("Found %d services with capability 'calculate'", len(services))
for _, svc := range services {
    log.Printf("Service: %s at %s:%d", svc.Name, svc.Address, svc.Port)
}
```

#### 3. High latency between agents

**Problem:** Slow response times in multi-agent systems.

**Solutions:**
- Enable connection pooling
- Add caching for discovery results
- Use circuit breakers to fail fast
- Monitor with distributed tracing

#### 4. Memory leaks

**Problem:** Agent memory usage grows over time.

**Solution:**
```go
// Add pprof for profiling
import _ "net/http/pprof"

func main() {
    // Enable profiling endpoint
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
    
    // Your agent code...
}

// Profile memory
// go tool pprof http://localhost:6060/debug/pprof/heap
```

### Debug Commands

```bash
# Check agent health
curl http://localhost:8080/health

# List capabilities
curl http://localhost:8080/api/capabilities

# Check Redis
redis-cli
> KEYS agents:*
> HGETALL agents:registry

# View logs
docker logs my-agent

# Kubernetes debugging
kubectl describe pod <pod-name> -n gomind
kubectl logs <pod-name> -n gomind
kubectl exec -it <pod-name> -n gomind -- sh
```

## Next Steps

Now that you've built your first agents, explore these advanced topics:

### 1. Build Complex AI Orchestrations
Learn to coordinate multiple AI agents for complex tasks. See the [Orchestration Module README](../orchestration/README.md).

### 2. Add Production Monitoring
Implement comprehensive observability with OpenTelemetry. See the [Telemetry Module README](../telemetry/README.md).

### 3. Implement Advanced Resilience
Add sophisticated fault tolerance patterns. See the [Resilience Module README](../resilience/README.md).

### 4. Explore the Full API
Deep dive into all available features. See the [API Reference](API.md).

### 5. Join the Community
- Report issues: [GitHub Issues](https://github.com/itsneelabh/gomind/issues)
- Contribute: Fork and submit PRs
- Discuss: Join discussions in issues

## Quick Reference

### Essential Commands

```bash
# Install GoMind
go get github.com/itsneelabh/gomind@v0.3.1

# Start Redis
docker run -d --name redis -p 6379:6379 redis:7-alpine

# Run agent
go run main.go

# Test endpoints
curl http://localhost:8080/health
curl http://localhost:8080/api/capabilities

# Build for production
CGO_ENABLED=0 GOOS=linux go build -o agent

# Docker operations
docker build -t my-agent:v1 .
docker run -d -p 8080:8080 my-agent:v1

# Kubernetes operations
kubectl apply -f deployment.yaml
kubectl get pods
kubectl logs -f <pod-name>
```

### Configuration Options

```go
// All available options
config, err := core.NewConfig(
    core.WithName("my-agent"),
    core.WithPort(8080),
    core.WithAddress("0.0.0.0"),
    core.WithNamespace("production"),
    core.WithRedisURL("redis://localhost:6379"),
    core.WithCORS(corsConfig),
    core.WithCORSDefaults(),
    core.WithLogLevel("info"),
    core.WithLogFormat("json"),
    core.WithDevelopmentMode(),
    core.WithMockDiscovery(),
    core.WithOpenAIAPIKey("sk-..."),
    core.WithOTELEndpoint("localhost:4317"),
)
```

### Environment Variables

```bash
# Core configuration
export AGENT_NAME="my-agent"
export GOMIND_PORT="8080"
export REDIS_URL="redis://localhost:6379"
export LOG_LEVEL="info"
export NAMESPACE="default"

# AI configuration
export OPENAI_API_KEY="sk-..."

# Telemetry
export OTEL_EXPORTER_OTLP_ENDPOINT="localhost:4317"
```

---

**Congratulations!** 🎉 You're now ready to build production-grade multi-agent systems with GoMind. Start simple, iterate, and scale as needed. Happy building!