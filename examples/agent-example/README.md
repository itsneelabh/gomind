# Research Assistant Agent Example

A comprehensive example demonstrating how to build an **Agent** (active orchestrator) using the GoMind framework. This agent can discover other components, orchestrate complex workflows, and use AI to intelligently coordinate multiple tools.

## 🎯 What This Example Demonstrates

- **Agent Pattern**: Active component that can discover and orchestrate other components
- **AI Integration**: Auto-detecting AI providers (OpenAI, Anthropic, Groq, etc.)
- **Service Discovery**: Finding and calling available tools dynamically
- **Intelligent Orchestration**: Using AI to plan and execute multi-step workflows
- **Production Patterns**: Resilient HTTP calls, error handling, caching
- **Kubernetes Deployment**: Complete K8s configuration with AI secrets

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

## 🚀 Quick Start (5 minutes)

### Prerequisites
- Go 1.25+
- Redis (or use Docker Compose)
- Optional: AI API key (OpenAI, Anthropic, Groq, etc.)

### 1. Set Up AI Provider (Optional)

The agent works without AI but is much more powerful with it:

```bash
# Option A: OpenAI
export OPENAI_API_KEY="sk-your-openai-key-here"

# Option B: Groq (fast and free tier available)
export GROQ_API_KEY="gsk-your-groq-key-here"

# Option C: Anthropic Claude
export ANTHROPIC_API_KEY="sk-ant-your-anthropic-key-here"

# Option D: Multiple providers (agent picks the best available)
export OPENAI_API_KEY="sk-..."
export GROQ_API_KEY="gsk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
```

### 2. Run the Complete Stack

```bash
# From examples/ directory - runs agent + tool + Redis
docker-compose up

# Or run just the agent (requires Redis and optionally other tools)
cd agent-example
go mod tidy
go run main.go
```

### 3. Test Agent Capabilities

```bash
# Health check
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

### Environment Variables

```bash
# Core Agent Configuration  
export GOMIND_AGENT_NAME="research-assistant"
export GOMIND_PORT=8090
export GOMIND_NAMESPACE="examples"

# Discovery Configuration
export REDIS_URL="redis://localhost:6379"
export GOMIND_REDIS_URL="redis://localhost:6379"

# AI Provider Configuration (pick one or multiple)
export OPENAI_API_KEY="sk-..."
export GROQ_API_KEY="gsk-..."
export ANTHROPIC_API_KEY="sk-ant-..."

# Development
export GOMIND_DEV_MODE=true
```

### Framework Options

```go
framework, _ := core.NewFramework(agent,
    // Core configuration
    core.WithName("research-assistant"),
    core.WithPort(8090),
    core.WithNamespace("examples"),
    
    // Discovery (agents get full powers)
    core.WithRedisURL("redis://localhost:6379"),
    core.WithDiscovery(true, "redis"), // Full discovery enabled
    
    // CORS for web access
    core.WithCORS([]string{"*"}, true),
    
    // Development features
    core.WithDevelopmentMode(true),
)
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