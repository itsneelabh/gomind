# Research Assistant Agent Example

A comprehensive example demonstrating how to build an **Agent** (active orchestrator) using the GoMind framework. This agent can discover other components, orchestrate complex workflows, and use AI to intelligently coordinate multiple tools.

## 🎯 What This Example Demonstrates

- **Agent Pattern**: Active component that can discover and orchestrate other components
- **AI Integration**: Auto-detecting AI providers (OpenAI, Anthropic, Groq, etc.)
- **Service Discovery**: Finding and calling available tools dynamically
- **Intelligent Orchestration**: Using AI to plan and execute multi-step workflows
- **3-Phase Schema Discovery**: AI-powered payload generation with progressive enhancement
- **Production Patterns**: Resilient HTTP calls, error handling, caching
- **Kubernetes Deployment**: Complete K8s configuration with AI secrets

## 🤖 3-Phase AI-Powered Payload Generation

This example implements the complete 3-phase approach for AI-powered tool payload generation:

### Phase 1: Description-Based (Always Active)
- AI generates payloads from natural language capability descriptions
- ~85-90% accuracy baseline
- Works for all capabilities automatically

### Phase 2: Field-Hint-Based (Implemented)
- All capabilities include structured field hints (`InputSummary`)
- AI uses exact field names, types, and examples
- ~95% accuracy for tool calls
- See [research_agent.go:93-238](research_agent.go#L93-L238) for implementation

### Phase 3: Schema Validation (Optional)
- Full JSON Schema v7 validation before sending payloads
- Cached in Redis (0ms overhead after first fetch)
- Enable with `GOMIND_VALIDATE_PAYLOADS=true`
- See [orchestration.go:93-121](orchestration.go#L93-L121) for validation logic

**Example Capability with Phase 2:**
```go
r.RegisterCapability(core.Capability{
    Name: "research_topic",
    Description: "Researches a topic by discovering and coordinating relevant tools",
    // Phase 2: Field hints guide AI generation
    InputSummary: &core.SchemaSummary{
        RequiredFields: []core.FieldHint{
            {
                Name: "topic",
                Type: "string",
                Example: "latest developments in renewable energy",
                Description: "The research topic or question to investigate",
            },
        },
        // ... optional fields
    },
    Handler: r.handleResearchTopic,
})
```

**Learn More**: See [Tool Schema Discovery Guide](../../docs/TOOL_SCHEMA_DISCOVERY_GUIDE.md) for complete documentation.

## 🧠 Architecture

```
┌─────────────────────────────────────────────────┐
│            Research Assistant Agent             │
│            (Active Orchestrator)                │
├─────────────────────────────────────────────────┤
│ AI Capabilities:                                │
│ • Auto-detect providers (OpenAI/Anthropic/etc) │
│ • Intelligent analysis and synthesis           │
│ • Workflow planning                            │
├─────────────────────────────────────────────────┤
│ Orchestration Capabilities:                     │
│ • research_topic (AI + tool coordination)      │
│ • discover_tools (service discovery)           │
│ • analyze_data (AI-powered analysis)           │
│ • orchestrate_workflow (multi-step coordination)│
├─────────────────────────────────────────────────┤
│ Framework Features:                             │
│ • Full Discovery powers                         │
│ • Redis registry + discovery                   │
│ • AI client auto-injection                     │
│ • Context propagation                          │
└─────────────────────────────────────────────────┘
```

**Key Principle**: Agents are **active** - they can discover other components and orchestrate complex workflows.

## 🚀 Quick Start - Local Kind Cluster (2 minutes)

### Prerequisites
- Docker Desktop or Docker Engine
- [kind](https://kind.sigs.k8s.io/) - Kubernetes in Docker
- [kubectl](https://kubernetes.io/docs/tasks/tools/) - Kubernetes CLI
- Optional: AI API key (OpenAI, Anthropic, Groq, etc.)

### Option 1: Complete Setup (Easiest)

```bash
# 1. Navigate to the example
cd examples/agent-example

# 2. (Optional) Configure AI provider
cp .env.example .env
# Edit .env and add your API key:
#   OPENAI_API_KEY=sk-...

# 3. Deploy everything with one command
make all
```

That's it! The agent is now running in your local Kind cluster.

### Option 2: Step-by-Step

```bash
# 1. Set up Kind cluster and Redis
make setup

# 2. Build and deploy the agent
make deploy

# 3. Test the deployment
make test
```

### Accessing the Agent

```bash
# Port forward to access locally
kubectl port-forward -n gomind-examples svc/research-agent-service 8090:80

# In another terminal, test it
curl http://localhost:8090/health
curl http://localhost:8090/api/capabilities
```

### Configuring AI Provider

The agent works without AI but has limited functionality. To enable full AI capabilities:

**Method 1: Using .env file (Recommended)**
```bash
# Copy the example file
cp .env.example .env

# Then load it and recreate secrets
source .env
make create-secrets

# Restart the agent to use new keys
kubectl rollout restart deployment/research-agent -n gomind-examples
```

**Method 2: Environment Variables**
```bash
# Set directly in your shell
export OPENAI_API_KEY="sk-..."

# Then create secrets
make create-secrets
```

### Available Make Commands

```bash
make setup      # Create Kind cluster, install Redis
make deploy     # Build and deploy the agent
make test       # Run automated tests
make logs       # View agent logs
make status     # Check deployment status
make clean      # Delete everything
make help       # Show all commands
```

## 🧪 Testing the Agent

### Understanding the `use_ai` Parameter

The research agent supports **hybrid operation** - combining tool orchestration with AI intelligence:

| Scenario | `use_ai` Setting | Behavior | Use Case |
|----------|------------------|----------|----------|
| **Tools Available** | `false` | Tools only, basic text summary | Fast, deterministic results |
| **Tools Available** | `true` | Tools + AI synthesis | Intelligent analysis of tool results |
| **No Tools Available** | `false` | Empty results | N/A - need relevant tools |
| **No Tools Available** | `true` | **AI answers directly** | General knowledge questions |

**Key Discovery**: When `use_ai: true` and no relevant tools are found, the agent automatically falls back to direct AI responses. This enables the agent to answer general questions:

```bash
# Example: Product recommendation (no tools available)
curl -X POST http://localhost:8090/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "Recommend top 3 wifi routers supporting 2 Gbps for home use",
    "use_ai": true,
    "max_results": 1
  }'
# ✓ Works - AI provides recommendations directly

# Same query without AI
curl -X POST http://localhost:8090/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "Recommend top 3 wifi routers supporting 2 Gbps for home use",
    "use_ai": false
  }'
# ✗ Returns empty results - no relevant tools
```

### Basic Testing

```bash
# Start port forwarding
kubectl port-forward -n gomind-examples svc/research-agent-service 8090:80

# Test health endpoint
curl http://localhost:8090/health

# Discover available tools and agents
curl -X POST http://localhost:8090/api/capabilities/discover_tools \
  -H "Content-Type: application/json" -d '{}'

# Research a topic (orchestrates multiple tools)
curl -X POST http://localhost:8090/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "weather in New York",
    "use_ai": true,
    "max_results": 5
  }'

# AI-powered data analysis (if AI provider configured)
curl -X POST http://localhost:8090/api/capabilities/analyze_data \
  -H "Content-Type: application/json" \
  -d '{
    "data": "Temperature: 22°C, Humidity: 65%, Wind: 10 km/h, Condition: Partly cloudy"
  }'
```

### 4. Expected Response (Research Topic)

```json
{
  "topic": "weather in New York",
  "summary": "Research completed successfully. The current weather in New York shows partly cloudy conditions with moderate temperature and humidity levels.",
  "tools_used": ["weather-service"],
  "results": [
    {
      "tool_name": "weather-service",
      "capability": "current_weather",
      "data": {
        "location": "New York",
        "temperature": 22.5,
        "humidity": 65,
        "condition": "partly cloudy"
      },
      "success": true,
      "duration": "120ms"
    }
  ],
  "ai_analysis": "Based on the weather data, New York is experiencing pleasant conditions with partly cloudy skies and comfortable temperatures around 22°C...",
  "confidence": 1.0,
  "processing_time": "342ms",
  "metadata": {
    "tools_discovered": 2,
    "tools_used": 1,
    "ai_enabled": true
  }
}
```

## 📊 Understanding the Code

### Agent vs Tool Differences

| Aspect | Tools (Passive) | Agents (Active) |
|--------|----------------|----------------|
| Discovery | ❌ Can only register | ✅ Full discovery powers |
| Orchestration | ❌ Cannot call others | ✅ Can coordinate workflows |
| AI Integration | ✅ Can use AI internally | ✅ Can use AI for orchestration |
| Framework Injection | Registry, Logger, Memory | Discovery, Logger, Memory, AI |

### Core Agent Pattern

```go
// 1. Create agent (active orchestrator)
agent := core.NewBaseAgent("research-assistant")

// 2. Add AI capabilities (optional but powerful)
aiClient, _ := ai.NewClient() // Auto-detects best provider
agent.AI = aiClient

// 3. Register orchestration capabilities
agent.RegisterCapability(core.Capability{
    Name:        "research_topic",
    Description: "Orchestrates research across multiple tools",
    Handler:     r.handleResearchTopic,
})

// 4. Framework auto-injects Discovery (unlike tools)
framework, _ := core.NewFramework(agent,
    core.WithRedisURL("redis://localhost:6379"),
    core.WithDiscovery(true), // Agents get full discovery
)

// 5. Use Discovery in handlers
func (r *ResearchAgent) handleResearchTopic(w http.ResponseWriter, req *http.Request) {
    // Discover available tools
    tools, _ := r.Discovery.Discover(ctx, core.DiscoveryFilter{
        Type: core.ComponentTypeTool,
    })
    
    // Orchestrate calls to multiple tools
    for _, tool := range tools {
        result := r.callTool(ctx, tool, query)
        // Process results...
    }
    
    // Use AI to synthesize results
    if r.aiClient != nil {
        analysis := r.generateAIAnalysis(ctx, topic, results)
    }
}
```

### AI Provider Auto-Detection

The agent automatically detects and configures the best available AI provider:

```go
// Auto-detection priority (in order):
// 1. OpenAI (OPENAI_API_KEY)
// 2. Groq (GROQ_API_KEY) - Fast inference, free tier  
// 3. DeepSeek (DEEPSEEK_API_KEY) - Advanced reasoning
// 4. Anthropic (ANTHROPIC_API_KEY)
// 5. Gemini (GEMINI_API_KEY)
// 6. Custom OpenAI-compatible (OPENAI_BASE_URL)

aiClient, err := ai.NewClient() // Picks best available
if err != nil {
    // Agent works without AI, just less intelligent
    log.Printf("No AI provider available: %v", err)
}
```

### Dynamic Tool Discovery

```go
func (r *ResearchAgent) handleResearchTopic(w http.ResponseWriter, req *http.Request) {
    // Step 1: Discover all available tools
    tools, err := r.Discovery.Discover(ctx, core.DiscoveryFilter{
        Type: core.ComponentTypeTool, // Only tools
    })
    
    // Step 2: Filter relevant tools for the topic
    var relevantTools []*core.ServiceInfo
    for _, tool := range tools {
        if r.isToolRelevant(tool, request.Topic) {
            relevantTools = append(relevantTools, tool)
        }
    }
    
    // Step 3: Call each relevant tool
    var results []ToolResult
    for _, tool := range relevantTools {
        result := r.callTool(ctx, tool, request.Topic)
        results = append(results, result)
    }
    
    // Step 4: Use AI to synthesize (if available)
    if r.aiClient != nil && request.UseAI {
        analysis := r.generateAIAnalysis(ctx, request.Topic, results)
    }
}
```

### Resilient Tool Calling

```go
func (r *ResearchAgent) callTool(ctx context.Context, tool *core.ServiceInfo, query string) *ToolResult {
    // Build endpoint URL
    capability := tool.Capabilities[0] // Use first capability
    endpoint := capability.Endpoint
    if endpoint == "" {
        endpoint = fmt.Sprintf("/api/capabilities/%s", capability.Name)
    }
    url := fmt.Sprintf("http://%s:%d%s", tool.Address, tool.Port, endpoint)
    
    // Create HTTP request with timeout
    httpCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    
    req, _ := http.NewRequestWithContext(httpCtx, "POST", url, body)
    req.Header.Set("Content-Type", "application/json")
    
    // Make resilient HTTP call
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return &ToolResult{Success: false, Error: err.Error()}
    }
    defer resp.Body.Close()
    
    // Handle response
    body, _ := io.ReadAll(resp.Body)
    var data interface{}
    json.Unmarshal(body, &data)
    
    return &ToolResult{
        ToolName:   tool.Name,
        Data:       data,
        Success:    resp.StatusCode < 400,
        Duration:   time.Since(startTime).String(),
    }
}
```

## 🤖 AI Integration Deep Dive

### Supported Providers

The agent automatically works with 20+ AI providers:

```bash
# Native providers (optimized implementations)
export OPENAI_API_KEY="sk-..."      # OpenAI GPT models
export ANTHROPIC_API_KEY="sk-ant-..." # Claude models
export GEMINI_API_KEY="..."         # Google Gemini

# OpenAI-compatible providers (one implementation, many services)
export GROQ_API_KEY="gsk-..."       # Ultra-fast inference
export DEEPSEEK_API_KEY="..."       # Advanced reasoning
export XAI_API_KEY="..."            # Elon's xAI Grok

# Custom OpenAI-compatible endpoints
export OPENAI_BASE_URL="https://your-llm.company.com/v1"
export OPENAI_API_KEY="your-key"

# Local models
export OPENAI_BASE_URL="http://localhost:11434/v1" # Ollama
export OPENAI_BASE_URL="http://localhost:8000/v1"  # vLLM
```

### AI Usage Patterns

#### 1. Intelligent Topic Analysis
```go
func (r *ResearchAgent) generateAIAnalysis(ctx context.Context, topic string, results []ToolResult) string {
    prompt := fmt.Sprintf(`Analyze research results for: "%s"
    
Results from tools:
%s

Provide a comprehensive summary with key insights.`, topic, formatResults(results))

    response, err := r.aiClient.GenerateResponse(ctx, prompt, &core.AIOptions{
        Temperature: 0.4, // More focused analysis
        MaxTokens:   800,
    })
    return response.Content
}
```

#### 2. Dynamic Workflow Planning
```go
// Future enhancement: AI plans which tools to call
prompt := fmt.Sprintf(`I need to research "%s". 
Available tools: %s
Plan the optimal sequence of tool calls.`, topic, availableTools)

plan, _ := r.aiClient.GenerateResponse(ctx, prompt, options)
// Execute the AI-generated plan
```

#### 3. Data Synthesis
```go
func (r *ResearchAgent) handleAnalyzeData(w http.ResponseWriter, req *http.Request) {
    if r.aiClient == nil {
        http.Error(w, "AI not available", http.StatusServiceUnavailable)
        return
    }
    
    prompt := fmt.Sprintf(`Analyze this data and provide insights:
%s

Provide:
1. Key findings  
2. Patterns/trends
3. Recommendations
4. Confidence level`, requestData["data"])

    analysis, _ := r.aiClient.GenerateResponse(req.Context(), prompt, &core.AIOptions{
        Temperature: 0.3, // Analytical mode
        MaxTokens:   1000,
    })
    
    response := map[string]interface{}{
        "analysis":    analysis.Content,
        "model":       analysis.Model,
        "tokens_used": analysis.Usage.TotalTokens,
    }
    
    json.NewEncoder(w).Encode(response)
}
```

## 🔧 Configuration Options

### Environment Variables (Production Best Practice)

```bash
# Core Agent Configuration (v0.6.4+ pattern - no hardcoded values)
export PORT="8090"                         # Service port
export NAMESPACE="examples"                # Service namespace
export DEV_MODE="false"                    # Production mode

# Legacy support (for backward compatibility)
export GOMIND_PORT="8090"
export GOMIND_NAMESPACE="examples"
export GOMIND_DEV_MODE="false"

# Discovery Configuration (REQUIRED - no defaults)
export REDIS_URL="redis://localhost:6379"  # Must be set explicitly
export GOMIND_REDIS_URL="redis://localhost:6379"  # Legacy support

# AI Provider Configuration (pick one or multiple)
export OPENAI_API_KEY="sk-..."
export GROQ_API_KEY="gsk-..."
export ANTHROPIC_API_KEY="sk-ant-..."

# Production Configuration
export GOMIND_LOG_LEVEL="info"            # info, debug, error
export GOMIND_LOG_FORMAT="json"           # json for structured logging
export OTEL_EXPORTER_OTLP_ENDPOINT="http://otel-collector:4318"
```

### Framework Options (v0.6.4+ Pattern)

```go
// Production pattern: ALL configuration from environment
framework, _ := core.NewFramework(agent,
    // Read from environment - no hardcoded values
    core.WithRedisURL(os.Getenv("REDIS_URL")),     // Required
    core.WithNamespace(os.Getenv("NAMESPACE")),    // Optional

    // Port from environment with proper parsing
    core.WithPort(getPortFromEnv()),

    // Development mode from environment
    core.WithDevelopmentMode(os.Getenv("DEV_MODE") == "true"),

    // Discovery enabled for agents
    core.WithDiscovery(true, "redis"),

    // CORS configuration (can be environment-based too)
    core.WithCORS(getCORSFromEnv(), true),
)

// Helper functions for production
func getPortFromEnv() int {
    portStr := os.Getenv("PORT")
    if portStr == "" {
        return 8090 // Default only if not set
    }
    port, err := strconv.Atoi(portStr)
    if err != nil {
        log.Fatalf("Invalid PORT: %v", err)
    }
    return port
}

func getCORSFromEnv() []string {
    origins := os.Getenv("CORS_ORIGINS")
    if origins == "" {
        return []string{"*"} // Default for development
    }
    return strings.Split(origins, ",")
}
```

### AI Configuration

```go
// Auto-detection (recommended)
aiClient, err := ai.NewClient()

// Explicit provider
aiClient, err := ai.NewClient(
    ai.WithProvider("anthropic"),
    ai.WithAPIKey("sk-ant-..."),
    ai.WithModel("claude-3-sonnet-20240229"),
)

// Multiple providers with fallback
primary, _ := ai.NewClient(ai.WithProvider("openai"))
fallback, _ := ai.NewClient(ai.WithProvider("groq"))
```

## 🏭 Production Best Practices

### 1. Environment-Based Configuration
**NEVER hardcode configuration values** in production code:

```go
// ❌ BAD - Hardcoded values
core.WithRedisURL("redis://localhost:6379")
core.WithPort(8090)

// ✅ GOOD - Environment-based
core.WithRedisURL(os.Getenv("REDIS_URL"))
core.WithPort(getPortFromEnv())
```

### 2. Health Checks & Readiness Probes
Always implement comprehensive health checks:

```go
// Liveness probe - is the service running?
func (r *ResearchAgent) handleHealth(w http.ResponseWriter, req *http.Request) {
    health := map[string]interface{}{
        "status": "healthy",
        "timestamp": time.Now().Unix(),
    }

    // Check critical dependencies
    if err := r.checkRedis(); err != nil {
        health["status"] = "unhealthy"
        health["redis"] = err.Error()
        w.WriteHeader(http.StatusServiceUnavailable)
    }

    json.NewEncoder(w).Encode(health)
}

// Readiness probe - is the service ready to handle requests?
func (r *ResearchAgent) handleReady(w http.ResponseWriter, req *http.Request) {
    // Check if discovery is initialized
    if r.Discovery == nil {
        http.Error(w, "Discovery not ready", http.StatusServiceUnavailable)
        return
    }

    // Check if we can discover services
    ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
    defer cancel()

    _, err := r.Discovery.Discover(ctx, core.DiscoveryFilter{})
    if err != nil {
        http.Error(w, "Discovery check failed", http.StatusServiceUnavailable)
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}
```

### 3. Graceful Shutdown
Implement proper shutdown handling:

```go
func main() {
    // ... framework setup ...

    // Graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-sigChan
        log.Println("Shutting down gracefully...")

        // Give ongoing requests time to complete
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        if err := framework.Shutdown(ctx); err != nil {
            log.Printf("Shutdown error: %v", err)
        }
        os.Exit(0)
    }()

    framework.Run()
}
```

### 4. Error Handling & Retries
Implement resilient error handling with retries:

```go
func (r *ResearchAgent) callToolWithRetry(ctx context.Context, tool *core.ServiceInfo, query string) (*ToolResult, error) {
    var lastErr error

    for attempt := 0; attempt < 3; attempt++ {
        if attempt > 0 {
            // Exponential backoff
            time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)
        }

        result, err := r.callTool(ctx, tool, query)
        if err == nil {
            return result, nil
        }

        lastErr = err
        log.Printf("Tool call attempt %d failed: %v", attempt+1, err)

        // Don't retry on context cancellation
        if errors.Is(err, context.Canceled) {
            return nil, err
        }
    }

    return nil, fmt.Errorf("all retry attempts failed: %w", lastErr)
}
```

> **See Also**: For advanced error handling patterns including AI-powered error correction and intelligent retry strategies, see the [Intelligent Error Handling Guide](https://github.com/itsneelabh/gomind/blob/main/docs/INTELLIGENT_ERROR_HANDLING.md).

### 5. Structured Logging
Use structured logging for better observability:

```go
import "github.com/sirupsen/logrus"

func setupLogging() {
    // Configure based on environment
    if os.Getenv("GOMIND_LOG_FORMAT") == "json" {
        logrus.SetFormatter(&logrus.JSONFormatter{})
    }

    level, err := logrus.ParseLevel(os.Getenv("GOMIND_LOG_LEVEL"))
    if err == nil {
        logrus.SetLevel(level)
    }

    // Add context to all logs
    logrus.WithFields(logrus.Fields{
        "service": "research-agent",
        "version": "v0.6.4",
        "namespace": os.Getenv("NAMESPACE"),
    }).Info("Agent starting")
}
```

### 6. Resource Limits & Monitoring
Set appropriate resource limits and monitor usage:

```yaml
# In k8-deployment.yaml
resources:
  requests:
    memory: "256Mi"  # Baseline memory
    cpu: "200m"      # Baseline CPU
  limits:
    memory: "512Mi"  # Prevent memory leaks from affecting cluster
    cpu: "1000m"     # Prevent CPU spikes
```

### 7. Security Best Practices

```go
// Input validation
func (r *ResearchAgent) validateRequest(req *ResearchRequest) error {
    if len(req.Topic) > 500 {
        return fmt.Errorf("topic too long")
    }

    // Sanitize input to prevent injection
    req.Topic = strings.TrimSpace(req.Topic)

    // Validate rate limits
    if !r.checkRateLimit(req.ClientID) {
        return fmt.Errorf("rate limit exceeded")
    }

    return nil
}

// Secure AI prompts
func (r *ResearchAgent) sanitizePrompt(userInput string) string {
    // Remove potential injection patterns
    sanitized := strings.ReplaceAll(userInput, "```", "")
    sanitized = strings.ReplaceAll(sanitized, "system:", "")
    return sanitized
}
```

### 8. Circuit Breaker Pattern
Prevent cascade failures:

```go
import "github.com/sony/gobreaker"

func setupCircuitBreaker() *gobreaker.CircuitBreaker {
    settings := gobreaker.Settings{
        Name:        "ToolCall",
        MaxRequests: 3,
        Interval:    10 * time.Second,
        Timeout:     30 * time.Second,
        OnStateChange: func(name string, from, to gobreaker.State) {
            log.Printf("Circuit breaker %s: %s -> %s", name, from, to)
        },
    }
    return gobreaker.NewCircuitBreaker(settings)
}
```

### 9. Distributed Tracing
Implement proper tracing for debugging:

```go
import "go.opentelemetry.io/otel"

func (r *ResearchAgent) handleWithTracing(w http.ResponseWriter, req *http.Request) {
    ctx, span := otel.Tracer("research-agent").Start(req.Context(), "research_topic")
    defer span.End()

    // Add attributes
    span.SetAttributes(
        attribute.String("topic", request.Topic),
        attribute.Bool("ai_enabled", request.UseAI),
    )

    // Pass context through call chain
    result, err := r.orchestrateResearch(ctx, request)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }
}
```

### 10. Configuration Validation
Validate all configuration at startup:

```go
func validateConfig() error {
    required := []string{"REDIS_URL", "PORT"}

    for _, env := range required {
        if os.Getenv(env) == "" {
            return fmt.Errorf("required environment variable %s not set", env)
        }
    }

    // Validate Redis URL format
    redisURL := os.Getenv("REDIS_URL")
    if !strings.HasPrefix(redisURL, "redis://") {
        return fmt.Errorf("invalid REDIS_URL format")
    }

    // Validate port is numeric
    if _, err := strconv.Atoi(os.Getenv("PORT")); err != nil {
        return fmt.Errorf("invalid PORT: %v", err)
    }

    return nil
}

func main() {
    if err := validateConfig(); err != nil {
        log.Fatalf("Configuration error: %v", err)
    }
    // ... rest of setup ...
}
```

## 🐳 Docker Usage

### Build and Run

```bash
# Build
docker build -t research-agent:latest .

# Run with AI (requires API key)
docker run -p 8090:8090 \
  -e REDIS_URL=redis://host.docker.internal:6379 \
  -e OPENAI_API_KEY=$OPENAI_API_KEY \
  research-agent:latest

# Run complete stack
cd examples/
docker-compose up
```

## ☸️ Kubernetes Deployment  

### Quick Deploy

```bash
# 1. Deploy infrastructure
kubectl apply -f k8-deployment/namespace.yaml
kubectl apply -f k8-deployment/redis.yaml

# 2. Create AI secrets
kubectl create secret generic ai-keys \
  --from-literal=openai-api-key=$OPENAI_API_KEY \
  --from-literal=groq-api-key=$GROQ_API_KEY \
  -n gomind-examples

# 3. Deploy agent
kubectl apply -f agent-example/k8-deployment.yaml

# 4. Check status
kubectl get pods -n gomind-examples -l app=research-agent
kubectl logs -f deployment/research-agent -n gomind-examples
```

### Complete Stack with Monitoring

```bash
# Deploy everything including monitoring
kubectl apply -k k8-deployment/

# Access services
kubectl port-forward svc/research-agent-service 8090:80 -n gomind-examples
kubectl port-forward svc/grafana 3000:80 -n gomind-examples
kubectl port-forward svc/jaeger-query 16686:80 -n gomind-examples
```

### Scaling Configuration

```bash
# Agents typically run as singletons for coordination
# But can be scaled if stateless

kubectl scale deployment research-agent --replicas=2 -n gomind-examples

# Configure load balancing for stateless operations
kubectl annotate service research-agent-service \
  service.kubernetes.io/load-balancer-class=nlb -n gomind-examples
```

## 🧪 Testing & Verification

### 1. Agent Health & Discovery

```bash
# Health check
curl http://localhost:8090/health

# Check AI provider status
curl http://localhost:8090/api/capabilities
# Look for AI-related capabilities

# Test service discovery
curl -X POST http://localhost:8090/api/capabilities/discover_tools \
  -H "Content-Type: application/json" -d '{}'
```

### 2. Orchestration Testing

```bash
# Test with weather tool running on port 8080
curl -X POST http://localhost:8090/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "weather in San Francisco",
    "use_ai": true,
    "max_results": 3
  }'
```

### 3. AI Analysis Testing

```bash
# Test AI-powered analysis
curl -X POST http://localhost:8090/api/capabilities/analyze_data \
  -H "Content-Type: application/json" \
  -d '{
    "data": "Sales increased 25% in Q4. Customer satisfaction: 94%. New product launches: 3. Market share: 15%."
  }'
```

### 4. Workflow Orchestration

```bash  
# Test complex workflow
curl -X POST http://localhost:8090/orchestrate \
  -H "Content-Type: application/json" \
  -d '{
    "workflow_type": "weather_analysis", 
    "parameters": {
      "location": "New York",
      "include_forecast": true
    }
  }'
```

## 📊 Monitoring & Observability

### Metrics (Prometheus)

Key metrics to monitor:
- `up{job="gomind-agents"}` - Agent availability
- `discovery_requests_total` - Service discovery calls
- `orchestration_requests_total` - Workflow orchestrations
- `ai_requests_total` - AI API calls
- `tool_calls_total` - Tool invocations

### Distributed Tracing (Jaeger)

The agent creates spans for:
- Service discovery operations
- Tool calls with latency
- AI API requests
- Workflow orchestration steps

### Structured Logs

```json
{
  "level": "info",
  "msg": "Starting research topic orchestration",
  "topic": "weather in New York",
  "tools_discovered": 2,
  "ai_enabled": true,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## 🎨 Advanced Usage & Customization

### Multi-Provider AI Strategy

```go
type ResilientAIAgent struct {
    primary   core.AIClient
    fallback  core.AIClient  
    local     core.AIClient
}

func (a *ResilientAIAgent) ProcessWithAI(ctx context.Context, prompt string, sensitive bool) (*core.AIResponse, error) {
    if sensitive {
        return a.local.GenerateResponse(ctx, prompt, options)
    }
    
    // Try primary first
    response, err := a.primary.GenerateResponse(ctx, prompt, options)
    if err != nil {
        // Fallback on error
        return a.fallback.GenerateResponse(ctx, prompt, options)
    }
    return response, nil
}
```

### Custom Orchestration Logic

```go  
func (r *ResearchAgent) orchestrateWeatherAnalysis(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    location := params["location"].(string)
    
    // Step 1: Get current weather
    currentWeather := r.callWeatherTool(ctx, "current_weather", location)
    
    // Step 2: Get forecast if requested
    var forecast interface{}
    if params["include_forecast"].(bool) {
        forecast = r.callWeatherTool(ctx, "forecast", location)
    }
    
    // Step 3: Use AI to analyze and correlate
    if r.aiClient != nil {
        analysis := r.generateWeatherInsights(ctx, currentWeather, forecast)
        return map[string]interface{}{
            "current": currentWeather,
            "forecast": forecast, 
            "insights": analysis,
        }, nil
    }
    
    return map[string]interface{}{
        "current": currentWeather,
        "forecast": forecast,
    }, nil
}
```

### AI Coding Assistant Prompts

Ask your AI assistant:

```
"Help me add a new capability that discovers and orchestrates financial data tools"

"Convert this research agent to specialize in scientific literature analysis"

"Add memory/conversation context to this agent for multi-turn interactions"

"Help me implement a circuit breaker pattern for tool calls in this agent"

"Add streaming responses for real-time orchestration updates"
```

## 🚨 Common Issues & Solutions

### Issue: Agent can't discover any tools
```bash
# Check Redis connection
redis-cli -u $REDIS_URL ping

# Check if tools are registered
redis-cli KEYS "gomind:services:*"

# Verify agent has Discovery permissions
kubectl logs deployment/research-agent -n gomind-examples | grep -i discovery
```

### Issue: AI requests failing
```bash
# Check API key configuration
kubectl get secret ai-keys -o yaml -n gomind-examples

# Test API key manually
curl -H "Authorization: Bearer $OPENAI_API_KEY" \
  https://api.openai.com/v1/models

# Check agent logs for AI errors
kubectl logs deployment/research-agent -n gomind-examples | grep -i "ai\|openai\|anthropic"
```

### Issue: Tool calls timing out
```go
// Increase timeout in tool calls
httpCtx, cancel := context.WithTimeout(ctx, 30*time.Second) // Increased from 10s

// Add retry logic
for attempts := 0; attempts < 3; attempts++ {
    result, err := r.callTool(ctx, tool, query)
    if err == nil {
        return result
    }
    time.Sleep(time.Duration(attempts) * time.Second)
}
```

### Issue: High resource usage
```yaml
# In k8-deployment.yaml, adjust resource limits
resources:
  requests:
    memory: "256Mi"  # Increased from 128Mi
    cpu: "300m"      # Increased from 200m  
  limits:
    memory: "512Mi"  # Increased from 256Mi
    cpu: "1000m"     # Increased from 500m
```

## 📋 Redis Service Registry Example

When the research-agent is deployed and running, it automatically registers itself in Redis. Here's what the agent's service entry looks like:

```json
{
  "name": "research-assistant",
  "type": "agent",
  "address": "research-agent-service.gomind-examples.svc.cluster.local",
  "port": 80,
  "namespace": "gomind-examples",
  "capabilities": [
    {
      "name": "research_topic",
      "description": "Researches a topic by discovering and coordinating relevant tools, optionally using AI for intelligent analysis",
      "endpoint": "/api/capabilities/research_topic",
      "input_types": ["application/json"],
      "output_types": ["application/json"],
      "input_summary": {
        "required_fields": [
          {
            "name": "topic",
            "type": "string",
            "example": "weather in New York",
            "description": "Research topic or question to investigate"
          }
        ],
        "optional_fields": [
          {
            "name": "use_ai",
            "type": "boolean",
            "example": true,
            "description": "Enable AI-powered analysis and synthesis"
          },
          {
            "name": "max_results",
            "type": "integer",
            "example": 5,
            "description": "Maximum number of tool results to collect"
          }
        ]
      }
    },
    {
      "name": "discover_tools",
      "description": "Discovers available tools and services in the system",
      "endpoint": "/api/capabilities/discover_tools",
      "input_types": ["application/json"],
      "output_types": ["application/json"],
      "input_summary": {
        "optional_fields": [
          {
            "name": "type",
            "type": "string",
            "example": "tool",
            "description": "Filter by component type (tool/agent)"
          },
          {
            "name": "capability",
            "type": "string",
            "example": "current_weather",
            "description": "Filter by capability name"
          }
        ]
      }
    },
    {
      "name": "analyze_data",
      "description": "Uses AI to analyze and provide insights on provided data",
      "endpoint": "/api/capabilities/analyze_data",
      "input_types": ["application/json"],
      "output_types": ["application/json"],
      "input_summary": {
        "required_fields": [
          {
            "name": "data",
            "type": "string",
            "example": "Temperature: 22°C, Humidity: 65%",
            "description": "Data to analyze"
          }
        ],
        "optional_fields": [
          {
            "name": "analysis_type",
            "type": "string",
            "example": "summary",
            "description": "Type of analysis (summary/detailed/trends)"
          }
        ]
      }
    },
    {
      "name": "orchestrate_workflow",
      "description": "Orchestrates complex multi-step workflows across discovered tools",
      "endpoint": "/api/capabilities/orchestrate_workflow",
      "input_types": ["application/json"],
      "output_types": ["application/json"],
      "input_summary": {
        "required_fields": [
          {
            "name": "workflow_type",
            "type": "string",
            "example": "weather_analysis",
            "description": "Type of workflow to execute"
          }
        ],
        "optional_fields": [
          {
            "name": "parameters",
            "type": "object",
            "example": {"location": "New York"},
            "description": "Workflow-specific parameters"
          }
        ]
      }
    }
  ],
  "metadata": {
    "version": "v1.0.0",
    "framework_version": "0.6.4",
    "discovery_enabled": true,
    "ai_enabled": true,
    "ai_provider": "openai",
    "last_heartbeat": "2025-11-10T05:15:25Z"
  },
  "health_endpoint": "/health",
  "registered_at": "2025-11-10T04:45:17Z",
  "last_seen": "2025-11-10T05:15:25Z",
  "ttl": 30
}
```

### Understanding the Agent Registry Data

**Key Differences from Tools:**
- **type**: Set to `agent` (active orchestrator with discovery powers)
- **ai_enabled**: Indicates AI integration is available
- **ai_provider**: Shows which AI provider is configured (openai, anthropic, groq, etc.)
- **capabilities**: Orchestration-focused capabilities that coordinate multiple tools

**Agent-Specific Features:**
- Agents can **discover** other services using the Discovery API
- Agents can **orchestrate** complex workflows across multiple tools
- Agents can use **AI** for intelligent analysis and decision-making
- Agents typically run as singletons or small replicas for coordination

**Service Discovery Pattern:**
- Both pod replicas send heartbeats to the same Redis key: `gomind:services:research-assistant`
- Kubernetes Service (`research-agent-service`) load-balances traffic across pods
- Other agents/tools discover one service entry, Kubernetes handles routing
- Heartbeat keeps TTL fresh - service auto-expires if pods stop

**Redis Index Structure:**
```
gomind:services:research-assistant       → Full service data (30s TTL)
gomind:types:agent                       → Set of all agents (60s TTL)
gomind:names:research-assistant          → Name index (60s TTL)
gomind:capabilities:research_topic       → Capability index (60s TTL)
gomind:capabilities:discover_tools       → Capability index (60s TTL)
gomind:capabilities:analyze_data         → Capability index (60s TTL)
gomind:capabilities:orchestrate_workflow → Capability index (60s TTL)
```

**Phase 2 Input Summary:**
All capabilities include `input_summary` with field hints:
- **required_fields**: Fields that must be provided
- **optional_fields**: Fields that enhance functionality
- Each field includes: name, type, example, description
- AI uses these hints for 95% accurate payload generation

You can inspect this data in your cluster:
```bash
# Get the full agent entry
kubectl exec -it deployment/redis -n default -- \
  redis-cli GET "gomind:services:research-assistant"

# List all registered services
kubectl exec -it deployment/redis -n default -- \
  redis-cli KEYS "gomind:services:*"

# See all agents
kubectl exec -it deployment/redis -n default -- \
  redis-cli SMEMBERS "gomind:types:agent"

# Check which services have specific capabilities
kubectl exec -it deployment/redis -n default -- \
  redis-cli SMEMBERS "gomind:capabilities:research_topic"
```

**How Agents Use Discovery:**
```go
// Agent discovers tools at runtime
tools, err := agent.Discovery.Discover(ctx, core.DiscoveryFilter{
    Type: core.ComponentTypeTool,
    Capability: "current_weather",
})

// Agent calls discovered tool
for _, tool := range tools {
    result := agent.callTool(ctx, tool, payload)
    // Process results...
}

// Agent uses AI to synthesize
if agent.AI != nil {
    analysis := agent.AI.GenerateResponse(ctx, prompt, options)
}
```

## 📚 Next Steps & Extensions

### 1. Add More AI Capabilities
- **Conversation Memory**: Multi-turn interactions
- **Streaming Responses**: Real-time updates  
- **Vision Analysis**: Image processing capabilities
- **Code Generation**: Dynamic tool creation

### 2. Advanced Orchestration
- **Workflow Engine**: Complex multi-step processes
- **Event-Driven**: React to tool/agent events
- **Parallel Execution**: Concurrent tool calls
- **Circuit Breakers**: Resilience patterns

### 3. Production Enhancements  
- **Authentication**: Secure agent APIs
- **Rate Limiting**: Protect downstream tools  
- **Caching**: Intelligent response caching
- **Load Balancing**: Scale across regions

### 4. Domain Specialization
- **Financial Agent**: Market analysis and trading
- **Healthcare Agent**: Medical data orchestration
- **DevOps Agent**: Infrastructure coordination
- **Scientific Agent**: Research paper analysis

## 🎓 Key Learnings

- **Agents are Active**: They can discover and coordinate other components
- **AI Amplifies Intelligence**: Auto-detection makes AI integration seamless
- **Discovery Powers Orchestration**: Dynamic tool calling enables flexible workflows
- **Resilience is Key**: Handle failures gracefully in distributed systems
- **Context Propagation**: Maintain request context across tool calls
- **Framework Does the Heavy Lifting**: Focus on business logic, not infrastructure

This agent can now discover your tools and create intelligent workflows combining multiple services!

---

**Next**: Try running both examples together to see the full tool + agent orchestration in action.