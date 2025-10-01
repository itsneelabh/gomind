# API Reference

Complete reference for all GoMind functions and types.

## Quick Navigation

**Most Used:**
- [NewFramework](#newframework) - Start your component
- [NewBaseAgent](#newbaseagent) - Create an agent
- [NewTool](#newtool) - Create a tool
- [NewAIAgent](#newaiagent) - Create an AI-powered agent
- [RegisterCapability](#registercapability) - Add capabilities

**By Module:**
- [Core](#core-module) - Foundation types and interfaces
- [AI](#ai-module) - AI providers and clients  
- [Resilience](#resilience-module) - Circuit breakers and retries
- [Telemetry](#telemetry-module) - Metrics and observability
- [Orchestration](#orchestration-module) - Multi-agent coordination
- [UI](#ui-module) - Chat interfaces and transports

---

## Core Module

Foundation types that every component uses.

### Component Interface

Every tool and agent implements this interface.

```go
type Component interface {
    Initialize(ctx context.Context) error
    GetID() string
    GetName() string
    GetCapabilities() []Capability
    GetType() ComponentType
}
```

**Example:**
```go
// Every component has these methods
component.Initialize(ctx)           // Start it up
id := component.GetID()             // "agent-abc123"
name := component.GetName()         // "calculator"
caps := component.GetCapabilities() // [Capability{Name: "add", ...}]
typ := component.GetType()          // ComponentTypeTool or ComponentTypeAgent
```

### NewBaseAgent

Creates an agent that can discover and coordinate other components.

```go
func NewBaseAgent(name string) *BaseAgent
func NewBaseAgentWithConfig(config *Config) *BaseAgent
```

Agents are active - they find tools and agents and make them work together.

**Example:**
```go
// Simple agent
agent := core.NewBaseAgent("orchestrator")

// Agent with configuration  
config := core.NewConfig(
    core.WithName("orchestrator"),
    core.WithPort(8080),
    core.WithRedisURL("redis://localhost:6379"),
)
agent := core.NewBaseAgentWithConfig(config)

// Initialize and start
ctx := context.Background()
err := agent.Initialize(ctx)
err = agent.Start(ctx, 8080)
```

### NewTool

Creates a tool that provides capabilities but cannot discover other components.

```go
func NewTool(name string) *BaseTool
func NewToolWithConfig(config *Config) *BaseTool
```

Tools are passive - they do work but can't discover other components.

**Example:**
```go
// Create a tool
calculator := core.NewTool("calculator")

// Add what it can do
calculator.RegisterCapability(core.Capability{
    Name:        "add",
    Description: "Add two numbers",
    Handler: func(w http.ResponseWriter, r *http.Request) {
        var params struct {
            A float64 `json:"a"`
            B float64 `json:"b"`
        }
        json.NewDecoder(r.Body).Decode(&params)
        
        result := params.A + params.B
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]float64{
            "result": result,
        })
    },
})

// Initialize and start
calculator.Initialize(ctx)
calculator.Start(ctx, 8081)
```

### Discover

Find components in the system. Only agents can discover - tools cannot.

```go
func (a *BaseAgent) Discover(ctx context.Context, filter DiscoveryFilter) ([]*ServiceInfo, error)
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `ctx` | `context.Context` | Request context |
| `filter` | `DiscoveryFilter` | What to look for |

**Example:**
```go
// Find all calculator tools
services, err := agent.Discover(ctx, core.DiscoveryFilter{
    Type:         core.ComponentTypeTool,
    Capabilities: []string{"calculate"},
})

// Find tools in a specific region
services, err := agent.Discover(ctx, core.DiscoveryFilter{
    Metadata: map[string]interface{}{"region": "us-east"},
})

// Use discovered services
for _, service := range services {
    fmt.Printf("Found: %s at %s\n", service.Name, service.Address)
}
```

### RegisterCapability

Add a capability to any component.

```go
func (c *BaseAgent) RegisterCapability(cap Capability)
func (c *BaseTool) RegisterCapability(cap Capability)
```

**Example:**
```go
tool.RegisterCapability(core.Capability{
    Name:        "send_email",
    Description: "Send an email message",
    Endpoint:    "/api/email",
    InputTypes:  []string{"application/json"},
    OutputTypes: []string{"application/json"},
    Handler: func(w http.ResponseWriter, r *http.Request) {
        // Parse email request, send email, return response
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "status": "sent",
        })
    },
})
```

### Types

#### ServiceInfo

Describes a component in the system.

```go
type ServiceInfo struct {
    ID           string                 `json:"id"`
    Name         string                 `json:"name"`
    Type         ComponentType          `json:"type"`
    Description  string                 `json:"description"`
    Address      string                 `json:"address"`
    Port         int                    `json:"port"`
    Capabilities []Capability           `json:"capabilities"`
    Metadata     map[string]interface{} `json:"metadata"`
    Health       HealthStatus           `json:"health"`
    LastSeen     time.Time              `json:"last_seen"`
}
```

#### DiscoveryFilter

Filter for finding components.

```go
type DiscoveryFilter struct {
    Type         ComponentType          `json:"type,omitempty"`
    Capabilities []string               `json:"capabilities,omitempty"`
    Name         string                 `json:"name,omitempty"`
    Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
```

#### Capability

Represents something a component can do.

```go
type Capability struct {
    Name        string           `json:"name"`
    Description string           `json:"description"`
    Endpoint    string           `json:"endpoint"`
    InputTypes  []string         `json:"input_types"`
    OutputTypes []string         `json:"output_types"`
    Handler     http.HandlerFunc `json:"-"`
}
```

### NewFramework

Start your component with the framework.

```go
func NewFramework(component HTTPComponent, opts ...Option) (*Framework, error)
```

This is the main entry point. Create a framework, add options, and run. Accepts any HTTPComponent (both Tools and Agents implement this interface).

**Configuration Options:**

| Option | Description | Example |
|--------|-------------|---------| 
| `WithName(name)` | Set component name | `"calculator"` |
| `WithPort(port)` | Set HTTP port | `8080` |
| `WithAddress(addr)` | Set bind address | `"0.0.0.0"` |
| `WithRedisURL(url)` | Enable Redis discovery | `"redis://localhost:6379"` |
| `WithDiscovery(discovery)` | Set custom discovery | `NewMockDiscovery()` |
| `WithAI(client)` | Add AI capabilities | `aiClient` |
| `WithTelemetry(telemetry)` | Add telemetry | `telemetryProvider` |
| `WithLogLevel(level)` | Set log level | `"info"` |
| `WithCORSDefaults()` | Enable CORS | - |

**Example:**
```go
// Simple - just run it
framework, _ := core.NewFramework(agent)
framework.Run(ctx)

// Production - with all features
framework, _ := core.NewFramework(agent,
    core.WithRedisURL("redis://localhost:6379"),
    core.WithPort(8080),
    core.WithLogLevel("info"),
    core.WithCORSDefaults(),
)
framework.Run(ctx)
```

### Configuration

#### NewConfig

Create configuration for components.

```go
func NewConfig(options ...Option) (*Config, error)
func DefaultConfig() *Config
```

**Example:**
```go
config := core.NewConfig(
    core.WithName("my-agent"),
    core.WithPort(8080),
    core.WithRedisURL("redis://localhost:6379"),
    core.WithDevelopmentMode(),
)
```

### Logging Configuration

Control framework logging behavior through configuration options or environment variables.

#### Configuration Options

```go
// Via code configuration
framework, _ := core.NewFramework(component,
    core.WithLogLevel("debug"),      // Set log level
    core.WithLogFormat("json"),      // Set output format
)
```

#### Environment Variables (Recommended for Containers)

The framework automatically reads these environment variables at startup, making them ideal for container deployments:

| Variable | Values | Default | Description |
|----------|--------|---------|-------------|
| `GOMIND_LOG_LEVEL` | `error`, `warn`, `info`, `debug` | `info` | Minimum logging level (filters output) |
| `GOMIND_LOG_FORMAT` | `json`, `text` | `json` | Output format |
| `GOMIND_DEV_MODE` | `true`, `false` | `false` | Overrides logging to debug + text format |

**Important Notes:**
- `GOMIND_DEV_MODE=true` overrides both level and format to `debug` and `text`
- Log levels properly filter output - `error` only shows errors, `warn` shows warn+error, etc.
- Default `info` level is recommended for production

**Local Development:**
```bash
# Enable debug logging to see framework internals
export GOMIND_LOG_LEVEL=debug
export GOMIND_LOG_FORMAT=json
go run main.go
```

**Kubernetes Deployment:**
```yaml
env:
- name: GOMIND_LOG_LEVEL
  value: "info"     # Change to "debug" for troubleshooting
- name: GOMIND_LOG_FORMAT
  value: "json"
```

**Docker Compose:**
```yaml
services:
  my-service:
    image: my-service:latest
    environment:
      - GOMIND_LOG_LEVEL=debug
      - GOMIND_LOG_FORMAT=json
```

**Changing Log Level Without Restart (Kubernetes):**
```bash
# Enable debug mode
kubectl set env deployment/my-service GOMIND_LOG_LEVEL=debug -n my-namespace

# Watch the logs
kubectl logs -f deployment/my-service -n my-namespace

# Set back to info
kubectl set env deployment/my-service GOMIND_LOG_LEVEL=info -n my-namespace
```

#### Log Levels Explained

The framework implements proper log level filtering with a standard hierarchy:

| Level | What Gets Logged | Use Case |
|-------|------------------|----------|
| `error` | Only Error messages | Production - minimal logging, only critical issues |
| `warn` | Warn + Error messages | Production - standard monitoring |
| `info` | Info + Warn + Error messages | Production default - normal operations |
| `debug` | Debug + Info + Warn + Error | Development/troubleshooting - verbose |

**Log Level Hierarchy:**
```
ERROR (highest severity)
  ↑
WARN
  ↑
INFO (default)
  ↑
DEBUG (most verbose)
```

**How It Works:**
- Setting a level includes all higher severity levels
- `GOMIND_LOG_LEVEL=error` → Only critical errors logged
- `GOMIND_LOG_LEVEL=warn` → Warnings and errors logged
- `GOMIND_LOG_LEVEL=info` → Info, warnings, and errors (default)
- `GOMIND_LOG_LEVEL=debug` → Everything logged

**Performance Note:**
- Debug level is significantly more verbose
- Early return prevents formatting costs for filtered messages
- Use `debug` only for troubleshooting, not in production

#### Output Formats

**JSON (recommended for production):**

Best for log aggregation systems (Elasticsearch, Splunk, CloudWatch, etc.).

```json
{"timestamp":"2025-01-29T10:30:45Z", "level":"INFO", "service":"weather-service", "component":"framework", "message":"HTTP server started", "port":8080}
{"timestamp":"2025-01-29T10:30:46Z", "level":"DEBUG", "service":"weather-service", "component":"framework", "message":"Registering capability", "capability":"current_weather"}
```

**Text (human-readable for development):**

Easier to read during local development.

```
2025-01-29 10:30:45 INFO [weather-service] HTTP server started port=8080
2025-01-29 10:30:46 DEBUG [weather-service] Registering capability name=current_weather
```

#### What the Framework Logs

The framework automatically logs important events at appropriate levels:

**DEBUG level logs:**
- Capability registration details
- Discovery query results
- Request routing decisions
- Configuration loading steps
- Heartbeat operations
- Circuit breaker state changes
- Detailed internal operations

**INFO level logs:**
- Service startup and shutdown
- HTTP server start/stop
- Redis connection established
- Service registration success
- Health check results
- Normal operational events

**WARN level logs:**
- Configuration issues (non-fatal)
- Retry attempts
- Degraded performance
- Resource constraints
- Deprecated feature usage

**ERROR level logs:**
- Failed service connections
- HTTP server errors
- Registration failures
- Discovery errors
- Critical operational issues

#### Using the Logger in Your Code

Every component (Tool or Agent) automatically gets a logger injected. Use it for structured logging:

```go
// Debug - verbose details for troubleshooting
tool.Logger.Debug("Cache lookup", map[string]interface{}{
    "key":   "weather:nyc",
    "found": true,
})

// Info - normal operational events
tool.Logger.Info("Processing request", map[string]interface{}{
    "method":   "POST",
    "path":     "/api/weather",
    "location": "New York",
})

// Warn - potential issues that aren't errors
tool.Logger.Warn("Cache miss, fetching from API", map[string]interface{}{
    "cache_key": "weather:nyc",
    "fallback":  "api",
})

// Error - critical issues
tool.Logger.Error("API call failed", map[string]interface{}{
    "error":    err.Error(),
    "provider": "openweathermap",
    "retry":    3,
})
```

For detailed examples of using the Logger interface, see [core/README.md - Logging Interface](../core/README.md#-logging-interface-know-whats-happening).

### Interfaces

#### Discovery Interface

Service discovery for agents (registration + discovery).

```go
type Discovery interface {
    Registry  // Embeds Registry interface
    Discover(ctx context.Context, filter DiscoveryFilter) ([]*ServiceInfo, error)
    FindService(ctx context.Context, serviceName string) ([]*ServiceInfo, error)
    FindByCapability(ctx context.Context, capability string) ([]*ServiceInfo, error)
}
```

#### Registry Interface  

Service registration for tools (registration only).

```go
type Registry interface {
    Register(ctx context.Context, info *ServiceInfo) error
    UpdateHealth(ctx context.Context, id string, status HealthStatus) error
    Unregister(ctx context.Context, id string) error
}
```

#### Memory Interface

State storage abstraction.

```go
type Memory interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key string, value string, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
}
```

### Discovery Implementations

#### NewRedisDiscovery

Redis-based service discovery.

```go
func NewRedisDiscovery(redisURL string) (*RedisDiscovery, error)
```

#### NewMockDiscovery  

Mock discovery for testing.

```go
func NewMockDiscovery() *MockDiscovery
```

### Memory Implementations

#### NewInMemoryStore

In-memory storage implementation.

```go
func NewInMemoryStore() *InMemoryStore
```

---

## AI Module

Connect to AI providers like OpenAI, Anthropic, Google, and more.

### NewClient

Create an AI client with auto-detection.

```go
func NewClient(opts ...AIOption) (core.AIClient, error)
func MustNewClient(opts ...AIOption) core.AIClient
```

Auto-detects your AI provider from environment variables.

**AI Options:**

| Option | Description | Example |
|--------|-------------|---------|
| `WithProvider(provider)` | Choose provider | `"openai"`, `"anthropic"` |
| `WithAPIKey(key)` | Override API key | `"sk-..."` |
| `WithModel(model)` | Choose model | `"gpt-4"`, `"claude-3-opus"` |
| `WithTemperature(temp)` | Set creativity (0-1) | `0.7` |
| `WithMaxTokens(tokens)` | Response length | `1000` |
| `WithTimeout(duration)` | Request timeout | `30*time.Second` |

**Example:**
```go
// Auto-detect from environment variables
client, _ := ai.NewClient()

// Specific provider
client, _ := ai.NewClient(
    ai.WithProvider("openai"),
    ai.WithModel("gpt-4"),
    ai.WithTemperature(0.7),
)

// Ask a question
response, _ := client.GenerateResponse(ctx, "What is 2+2?", nil)
fmt.Println(response.Content)  // "4"
```

### GenerateResponse

Ask the AI a question.

```go
func (c *AIClient) GenerateResponse(ctx context.Context, prompt string, options *AIOptions) (*AIResponse, error)
```

**Example:**
```go
// Simple question
response, _ := client.GenerateResponse(ctx, "Hello!", nil)

// With custom options
response, _ := client.GenerateResponse(ctx, "Write a poem", &core.AIOptions{
    Temperature: 0.9,  // Note: float32 type
    MaxTokens:   200,
})
```

### NewAIAgent

Create an AI-powered agent.

```go
func NewAIAgent(name string, apiKey string) (*AIAgent, error)
func NewIntelligentAgent(id string) *IntelligentAgent  // For testing
```

**Example:**
```go
// Create an AI assistant
agent, err := ai.NewAIAgent("assistant", os.Getenv("OPENAI_API_KEY"))
if err != nil {
    log.Fatal(err)
}

// It can answer questions
response, _ := agent.GenerateResponse(ctx, "What's the weather like?", nil)

// It can process with memory
result, _ := agent.ProcessWithMemory(ctx, "Remember my name is John")
```

### AI Providers

GoMind supports these AI providers (auto-detected from environment):

| Provider | Environment Variable | Models |
|----------|---------------------|---------|
| OpenAI | `OPENAI_API_KEY` | gpt-4, gpt-4-turbo, gpt-3.5-turbo |
| Anthropic | `ANTHROPIC_API_KEY` | claude-3-opus, claude-3-sonnet |
| Google | `GEMINI_API_KEY` | gemini-pro |
| Groq | `GROQ_API_KEY` | llama3, mixtral |

**Provider Detection:**
```go
// Provider auto-detection is handled internally by NewClient()
// The client automatically detects and uses the best available provider
// based on environment variables (OPENAI_API_KEY, ANTHROPIC_API_KEY, etc.)
client, err := ai.NewClient()  // Auto-detects provider
```

---

## Resilience Module

Handle failures gracefully with circuit breakers and retry logic.

### NewCircuitBreaker

Stop calling services that are failing.

```go
func NewCircuitBreaker(config *CircuitBreakerConfig) (*CircuitBreaker, error)
func NewCircuitBreakerLegacy(failureThreshold int, recoveryTimeout time.Duration) *CircuitBreaker
func DefaultConfig() *CircuitBreakerConfig
```

**Example:**
```go
// Create circuit breaker with config
config := &resilience.CircuitBreakerConfig{
    Name:             "external-api",
    ErrorThreshold:   0.5,  // 50% error rate triggers opening
    VolumeThreshold:  10,   // Need 10 requests minimum
    SleepWindow:      30 * time.Second,
    HalfOpenRequests: 3,
}
breaker, err := resilience.NewCircuitBreaker(config)

// Or use legacy simple version
breaker := resilience.NewCircuitBreakerLegacy(5, 30*time.Second)

// Use it to protect calls
err := breaker.Execute(ctx, func() error {
    return callExternalService()
})

if errors.Is(err, core.ErrCircuitBreakerOpen) {
    return fallbackResponse()
}
```

### Retry

Automatically retry failed operations.

```go
func Retry(ctx context.Context, config *RetryConfig, fn func() error) error
func DefaultRetryConfig() *RetryConfig
```

**Example:**
```go
// Use default retry config
err := resilience.Retry(ctx, resilience.DefaultRetryConfig(), func() error {
    return unreliableOperation()
})

// Custom retry config
config := &resilience.RetryConfig{
    MaxAttempts:   5,
    InitialDelay:  100 * time.Millisecond,
    BackoffFactor: 2.0,
    MaxDelay:      5 * time.Second,
    JitterEnabled: true,
}
err := resilience.Retry(ctx, config, func() error {
    return flakeyAPICall()
})
```

### RetryWithCircuitBreaker

Combine retry logic with circuit breaker protection.

```go
func RetryWithCircuitBreaker(ctx context.Context, config *RetryConfig, cb *CircuitBreaker, fn func() error) error
```

**Example:**
```go
retryConfig := resilience.DefaultRetryConfig()
breaker := resilience.NewCircuitBreakerLegacy(3, 10*time.Second)

err := resilience.RetryWithCircuitBreaker(ctx, retryConfig, breaker, func() error {
    return callUnreliableService()
})
```

---

## Telemetry Module

Monitor and observe your system with metrics and distributed tracing.

### Metrics

Track what's happening in your system.

```go
// Simple API for common metrics
func Counter(name string, labels ...string)
func Histogram(name string, value float64, labels ...string)
func Gauge(name string, value float64, labels ...string)  
func Duration(name string, startTime time.Time, labels ...string)
```

**Example:**
```go
// Count events
telemetry.Counter("requests.total", "method", "GET", "status", "200")
telemetry.Counter("errors.count", "type", "timeout")

// Track distributions (latency, sizes)
telemetry.Histogram("request.duration_ms", 125.3, "endpoint", "/api/users")
telemetry.Histogram("request.size_bytes", 1024, "method", "POST")

// Current values (memory, connections) 
telemetry.Gauge("memory.used_bytes", 1024*1024*100)
telemetry.Gauge("connections.active", 42, "pool", "database")

// Time operations
start := time.Now()
processRequest()
telemetry.Duration("operation.duration_ms", start, "op", "process")
```

### Type-specific Helpers

```go
func RecordError(name string, errorType string, labels ...string)
func RecordSuccess(name string, labels ...string)
func RecordLatency(name string, milliseconds float64, labels ...string)
func RecordBytes(name string, bytes int64, labels ...string)
```

**Example:**
```go
// Record operations
telemetry.RecordSuccess("api.calls", "endpoint", "/users")
telemetry.RecordError("api.calls", "timeout", "endpoint", "/users")

// Record performance metrics
telemetry.RecordLatency("api.latency", 150.0, "endpoint", "/users")
telemetry.RecordBytes("response.size", 2048, "endpoint", "/users")
```

### Advanced Configuration

```go
// Configure telemetry with OpenTelemetry
func NewOTelProvider(serviceName string, endpoint string) (*OTelProvider, error)
```

**Example:**
```go
provider, err := telemetry.NewOTelProvider("my-service", "http://jaeger:14268/api/traces")
defer provider.Shutdown(ctx)
```

---

## Orchestration Module

Coordinate multiple agents and tools to work together, with smart capability discovery for scaling to hundreds of agents.

### CreateSimpleOrchestrator

Create an orchestrator with zero configuration - perfect for getting started.

```go
func CreateSimpleOrchestrator(discovery core.Discovery, aiClient core.AIClient) *AIOrchestrator
```

**Example:**
```go
// Zero configuration - just works!
orchestrator := orchestration.CreateSimpleOrchestrator(discovery, aiClient)

// Process natural language requests
response, err := orchestrator.ProcessRequest(ctx,
    "Get weather for NYC, find news about it, and write a summary",
    nil,
)

fmt.Println(response.Response)
fmt.Printf("Used agents: %v\n", response.AgentsInvolved)
```

### CreateOrchestrator

Create an orchestrator with configuration and dependency injection for production use.

```go
func CreateOrchestrator(config *OrchestratorConfig, deps OrchestratorDependencies) (*AIOrchestrator, error)
```

**Example:**
```go
// Create dependencies with optional components
deps := orchestration.OrchestratorDependencies{
    Discovery:      discovery,                    // Required
    AIClient:       aiClient,                     // Required
    CircuitBreaker: circuitBreaker,              // Optional: for resilience
    Logger:         logger,                       // Optional: for logging
    Telemetry:      telemetry,                   // Optional: for observability
}

// Configure orchestrator
config := &orchestration.OrchestratorConfig{
    RoutingMode:            orchestration.ModeAutonomous,
    SynthesisStrategy:      orchestration.StrategyLLM,
    CapabilityProviderType: "default",           // "default" or "service" for large scale
    EnableFallback:         true,                 // Graceful degradation
    CacheEnabled:           true,
    CacheTTL:              5 * time.Minute,
}

orchestrator, err := orchestration.CreateOrchestrator(config, deps)

// Process natural language requests
response, err := orchestrator.ProcessRequest(ctx,
    "Get weather for NYC, find news about it, and write a summary",
    nil,
)
```

### Scaling to Hundreds of Agents

When you have 100s-1000s of agents, use the service-based capability provider:

```go
// Configure for large scale with external RAG service
config := orchestration.DefaultConfig()
config.CapabilityProviderType = "service"
config.CapabilityService = orchestration.ServiceCapabilityConfig{
    Endpoint:  "http://capability-service:8080",
    TopK:      20,        // Return top 20 relevant agents
    Threshold: 0.7,       // Minimum relevance score
    Timeout:   10 * time.Second,
}

deps := orchestration.OrchestratorDependencies{
    Discovery:      discovery,
    AIClient:       aiClient,
    CircuitBreaker: cb,  // Recommended for production
}

orchestrator, _ := orchestration.CreateOrchestrator(config, deps)
```

### Workflow Engine

Define and execute structured multi-step workflows.

```go
func NewWorkflowEngine(discovery core.Discovery, stateStore StateStore, logger core.Logger) *WorkflowEngine
```

**Example:**
```go
stateStore := orchestration.NewRedisStateStore(discovery)
engine := orchestration.NewWorkflowEngine(discovery, stateStore, logger)

// Define workflow as YAML
yamlDef := `
name: user-onboarding
version: "1.0"
inputs:
  user_email:
    type: string
    required: true
steps:
  - name: validate_email
    tool: validator          # Tool: passive validator
    action: check_email
    inputs:
      email: ${inputs.user_email}
  - name: create_account  
    tool: user_service       # Tool: service that creates users
    action: create_user
    inputs:
      email: ${inputs.user_email}
    depends_on: [validate_email]
  - name: send_welcome
    tool: email_service      # Tool: service that sends emails
    action: send_welcome
    inputs:
      user_id: ${steps.create_account.output.user_id}
    depends_on: [create_account]
outputs:
  user_id: ${steps.create_account.output.user_id}
`

// Parse and execute
workflow, _ := engine.ParseWorkflowYAML([]byte(yamlDef))
execution, err := engine.ExecuteWorkflow(ctx, workflow, map[string]interface{}{
    "user_email": "john@example.com",
})

fmt.Printf("Status: %s\n", execution.Status)
fmt.Printf("User ID: %v\n", execution.Outputs["user_id"])
```

---

## UI Module

Create chat interfaces and user interactions.

### NewChatAgent

Create a chat agent with pluggable transports.

```go
func NewChatAgent(config ChatAgentConfig, aiClient core.AIClient, sessions SessionManager) *DefaultChatAgent
func NewChatAgentWithOptions(name string, aiClient core.AIClient, opts ...ChatAgentOption) *DefaultChatAgent
```

**Example:**
```go
// Create chat agent
config := ui.DefaultChatAgentConfig("assistant")

// Production: Use Redis session manager
sessions, _ := ui.NewRedisSessionManager("redis://localhost:6379", ui.DefaultSessionConfig())
// Development: Use mock session manager  
// sessions := ui.NewMockSessionManager(ui.DefaultSessionConfig())

aiClient, _ := ai.NewClient()
chatAgent := ui.NewChatAgent(config, aiClient, sessions)

// Handle chat messages
response, err := chatAgent.HandleMessage(ctx, ui.ChatMessage{
    SessionID: "session-123",
    Content:   "Hello, how can you help me?",
    UserID:    "user-456",
})

fmt.Println(response.Content) // AI-generated response
```

### Transports

Connect your chat agent to different protocols.

#### WebSocket Transport
```go
transport := websocket.New(websocket.Config{
    Address: ":8080",
    Path:    "/ws",
})
```

#### HTTP/REST Transport
```go  
transport := http.New(http.Config{
    Address:  ":8080", 
    BasePath: "/api/chat",
})
```

#### Server-Sent Events (SSE)
```go
transport := sse.New(sse.Config{
    Address: ":8080",
    Path:    "/events", 
})
```

### Session Management

Create and manage chat sessions.

#### NewRedisSessionManager

Redis-based distributed session management.

```go
func NewRedisSessionManager(redisURL string, config SessionConfig) (*RedisSessionManager, error)
```

#### NewMockSessionManager (Development)

Mock session manager for testing.

```go
func NewMockSessionManager(config SessionConfig) *MockSessionManager
```

### Security Features

The UI module includes comprehensive security:

```go
// Rate limiting (in ui/security package)
rateLimiter := security.NewRedisRateLimiter(config, redisClient)
transport = security.NewRateLimitTransport(transport, config, rateLimiter, logger, telemetry)
```

---

## HTTP API Endpoints

When components are started with the framework, they automatically expose HTTP endpoints.

### Health Check

```
GET /health
```

**Response:**
```json
{
    "status": "healthy",
    "component": "component-name",
    "id": "component-id"
}
```

### Capabilities

```
GET /api/capabilities
```

Lists all capabilities provided by the component.

**Response:**
```json
[
    {
        "name": "calculate",
        "description": "Performs calculations",
        "endpoint": "/api/capabilities/calculate",
        "input_types": ["application/json"],
        "output_types": ["application/json"]
    }
]
```

### Invoke Capability

```
POST /api/capabilities/{capability}
```

Invokes a specific capability.

**Request:**
```json
{
    "input": "capability-specific input data"
}
```

**Response:**
```json
{
    "capability": "capability-name",
    "status": "success", 
    "result": "capability-specific result"
}
```

---

## Error Handling

### Common Errors

| Error | Package | Description |
|-------|---------|-------------|
| `ErrCircuitBreakerOpen` | core | Circuit breaker is open |
| `ErrMaxRetriesExceeded` | core | Retry attempts exhausted |
| `ErrTimeout` | core | Operation timed out |
| `ErrNotFound` | core | Resource not found |
| `ErrConfigurationError` | core | Invalid configuration |
| `ErrStateError` | core | Invalid state |
| `ErrContextCanceled` | core | Context was canceled |

### Error Checking

```go
// Check specific errors
if errors.Is(err, core.ErrCircuitBreakerOpen) {
    return fallbackResponse()
}

// Check error categories
if core.IsTimeout(err) {
    // Handle timeout
}

if core.IsNotFound(err) {
    // Handle not found
}
```

---

## Environment Variables

Configure GoMind with these environment variables:

### AI Providers
```bash
OPENAI_API_KEY=sk-...           # OpenAI
ANTHROPIC_API_KEY=sk-ant-...    # Claude  
GEMINI_API_KEY=...              # Google Gemini
GROQ_API_KEY=...                # Groq
```

### Service Discovery
```bash
REDIS_URL=redis://localhost:6379
```

### Logging and Telemetry
```bash
GOMIND_LOG_LEVEL=info          # debug, info, warn, error
GOMIND_LOG_FORMAT=json         # json, text
OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:14268/api/traces
```

### Framework
```bash
GOMIND_PORT=8080               # Default HTTP port
GOMIND_ENV=production          # development, production
```

---

## Import Paths

```go
// Main framework - re-exports all core types
import "github.com/itsneelabh/gomind"

// Individual modules  
import "github.com/itsneelabh/gomind/core"
import "github.com/itsneelabh/gomind/ai" 
import "github.com/itsneelabh/gomind/resilience"
import "github.com/itsneelabh/gomind/telemetry"
import "github.com/itsneelabh/gomind/orchestration"
import "github.com/itsneelabh/gomind/ui"
```

---

## Quick Tips

### Always Check Errors
```go
// ✅ GOOD
response, err := agent.ProcessRequest(ctx, request)
if err != nil {
    return fmt.Errorf("processing failed: %w", err)
}

// ❌ BAD  
response, _ := agent.ProcessRequest(ctx, request)
```

### Use Context Timeouts
```go
// ✅ GOOD
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

// ❌ BAD
ctx := context.Background()
```

### Component Separation
```go
// ✅ GOOD - Tools do specific tasks
calculator := core.NewTool("calculator") 
emailer := core.NewTool("emailer")

// Agents orchestrate tools
orchestrator := core.NewBaseAgent("orchestrator")

// ❌ BAD - One component doing everything  
everything := core.NewTool("do-all")
```

---

## Migration Examples

**From LangChain:**
```python
# Before (Python)
chain = LLMChain(llm=OpenAI())
result = chain.run("Hello")
```

```go
// After (Go)
agent, _ := ai.NewAIAgent("assistant", os.Getenv("OPENAI_API_KEY"))
result, _ := agent.GenerateResponse(ctx, "Hello", nil)
```

**From AutoGen:**
```python
# Before (Python)
assistant = AssistantAgent("bot") 
reply = assistant.generate_reply(msg)
```

```go
// After (Go)  
config := ui.DefaultChatAgentConfig("bot")
agent := ui.NewChatAgent(config, aiClient, sessions)
reply, _ := agent.HandleMessage(ctx, msg)
```

---

## Need Help?

- **Quick Start**: [QUICK_START.md](./QUICK_START.md)
- **Examples**: [github.com/itsneelabh/gomind/examples](https://github.com/itsneelabh/gomind/tree/main/examples)
- **Issues**: [github.com/itsneelabh/gomind/issues](https://github.com/itsneelabh/gomind/issues)
- **Documentation**: [docs/](https://github.com/itsneelabh/gomind/tree/main/docs)