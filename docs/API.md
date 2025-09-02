# GoMind Framework API Reference

## Table of Contents

- [Core Module API](#core-module-api)
  - [Interfaces](#core-interfaces)
  - [Types and Structures](#core-types)
  - [Methods](#core-methods)
  - [Configuration](#configuration-api)
- [AI Module API](#ai-module-api)
  - [OpenAI Client](#openai-client)
  - [Intelligent Agent](#intelligent-agent)
- [Orchestration Module API](#orchestration-module-api)
  - [AI Orchestrator](#ai-orchestrator)
  - [Workflow Engine](#workflow-engine)
- [Resilience Module API](#resilience-module-api)
  - [Circuit Breaker](#circuit-breaker)
  - [Retry](#retry)
- [Telemetry Module API](#telemetry-module-api)
- [HTTP API Endpoints](#http-api-endpoints)
- [Error Types](#error-types)
- [Constants](#constants)

---

## Core Module API

### Core Interfaces

#### Agent Interface
```go
type Agent interface {
    Initialize(ctx context.Context) error
    GetID() string
    GetName() string
    GetCapabilities() []Capability
}
```

The fundamental interface that all agents must implement.

#### Logger Interface
```go
type Logger interface {
    Info(msg string, fields map[string]interface{})
    Error(msg string, fields map[string]interface{})
    Warn(msg string, fields map[string]interface{})
    Debug(msg string, fields map[string]interface{})
}
```

Minimal logging interface for framework components.

#### Discovery Interface
```go
type Discovery interface {
    Register(ctx context.Context, registration *ServiceRegistration) error
    Unregister(ctx context.Context, serviceID string) error
    FindService(ctx context.Context, serviceName string) ([]*ServiceRegistration, error)
    FindByCapability(ctx context.Context, capability string) ([]*ServiceRegistration, error)
    UpdateHealth(ctx context.Context, serviceID string, status HealthStatus) error
}
```

Service discovery abstraction for agent registration and lookup.

#### Memory Interface
```go
type Memory interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key string, value string, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
}
```

State storage abstraction for agents.

#### Telemetry Interface
```go
type Telemetry interface {
    StartSpan(ctx context.Context, name string) (context.Context, Span)
    RecordMetric(name string, value float64, labels map[string]string)
}
```

Optional telemetry support for distributed tracing and metrics.

#### Span Interface
```go
type Span interface {
    End()
    SetAttribute(key string, value interface{})
    RecordError(err error)
}
```

Represents a telemetry span for tracing.

#### AIClient Interface
```go
type AIClient interface {
    GenerateResponse(ctx context.Context, prompt string, options *AIOptions) (*AIResponse, error)
}
```

Optional AI client support for intelligent agents.

### Core Types

#### Capability
```go
type Capability struct {
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Endpoint    string   `json:"endpoint"`
    InputTypes  []string `json:"input_types"`
    OutputTypes []string `json:"output_types"`
}
```

Represents a capability that an agent provides.

#### ServiceRegistration
```go
type ServiceRegistration struct {
    ID           string
    Name         string
    Address      string
    Port         int
    Capabilities []string
    Metadata     map[string]string
    Health       HealthStatus
    LastSeen     time.Time
}
```

Service registration information for discovery.

#### AIOptions
```go
type AIOptions struct {
    Model        string
    Temperature  float32
    MaxTokens    int
    SystemPrompt string
}
```

Options for AI generation requests.

#### AIResponse
```go
type AIResponse struct {
    Content string
    Model   string
    Usage   TokenUsage
}
```

Response from AI client.

#### TokenUsage
```go
type TokenUsage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}
```

Token consumption details for AI responses.

#### HealthStatus
```go
type HealthStatus string

const (
    HealthHealthy   HealthStatus = "healthy"
    HealthUnhealthy HealthStatus = "unhealthy"
    HealthUnknown   HealthStatus = "unknown"
)
```

Service health status enumeration.

### No-Op Implementations

The core module provides no-op implementations for testing and default behavior:

#### NoOpLogger
```go
type NoOpLogger struct{}
```
A logger that discards all log messages. Used as default when no logger is configured.

#### NoOpTelemetry
```go
type NoOpTelemetry struct{}
```
A telemetry provider that does nothing. Used as default when telemetry is not configured.

#### NoOpSpan
```go
type NoOpSpan struct{}
```
A span that does nothing. Returned by NoOpTelemetry.

### Core Methods

#### BaseAgent Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewBaseAgent` | `func NewBaseAgent(name string) *BaseAgent` | Creates a new base agent with minimal dependencies |
| `NewBaseAgentWithConfig` | `func NewBaseAgentWithConfig(config *Config) *BaseAgent` | Creates a new base agent with configuration |
| `Initialize` | `func (b *BaseAgent) Initialize(ctx context.Context) error` | Initializes the agent and its components |
| `GetID` | `func (b *BaseAgent) GetID() string` | Returns the unique agent ID |
| `GetName` | `func (b *BaseAgent) GetName() string` | Returns the agent name |
| `GetCapabilities` | `func (b *BaseAgent) GetCapabilities() []Capability` | Returns list of agent capabilities |
| `RegisterCapability` | `func (b *BaseAgent) RegisterCapability(cap Capability)` | Registers a new capability to the agent |
| `Start` | `func (b *BaseAgent) Start(port int) error` | Starts the HTTP server |
| `Stop` | `func (b *BaseAgent) Stop(ctx context.Context) error` | Stops the agent gracefully |

#### Discovery Methods

##### RedisDiscovery
| Method | Signature | Description |
|--------|-----------|-------------|
| `NewRedisDiscovery` | `func NewRedisDiscovery(redisURL string) (*RedisDiscovery, error)` | Creates Redis-based discovery |
| `Register` | `func (r *RedisDiscovery) Register(ctx context.Context, registration *ServiceRegistration) error` | Registers a service |
| `Unregister` | `func (r *RedisDiscovery) Unregister(ctx context.Context, serviceID string) error` | Unregisters a service |
| `FindService` | `func (r *RedisDiscovery) FindService(ctx context.Context, serviceName string) ([]*ServiceRegistration, error)` | Finds services by name |
| `FindByCapability` | `func (r *RedisDiscovery) FindByCapability(ctx context.Context, capability string) ([]*ServiceRegistration, error)` | Finds services by capability |
| `UpdateHealth` | `func (r *RedisDiscovery) UpdateHealth(ctx context.Context, serviceID string, status HealthStatus) error` | Updates service health status |

##### MockDiscovery
| Method | Signature | Description |
|--------|-----------|-------------|
| `NewMockDiscovery` | `func NewMockDiscovery() *MockDiscovery` | Creates mock discovery for testing |

#### Memory Methods

##### InMemoryStore
| Method | Signature | Description |
|--------|-----------|-------------|
| `NewInMemoryStore` | `func NewInMemoryStore() *InMemoryStore` | Creates in-memory storage |
| `Get` | `func (m *InMemoryStore) Get(ctx context.Context, key string) (string, error)` | Gets value by key |
| `Set` | `func (m *InMemoryStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error` | Sets value with TTL |
| `Delete` | `func (m *InMemoryStore) Delete(ctx context.Context, key string) error` | Deletes key |
| `Exists` | `func (m *InMemoryStore) Exists(ctx context.Context, key string) (bool, error)` | Checks if key exists |

### Configuration API

#### Config Structure
```go
type Config struct {
    // Basic configuration
    ID        string
    Name      string
    Port      int
    Address   string
    Namespace string
    
    // Component configuration
    Discovery  DiscoveryConfig
    HTTP       HTTPConfig
    Logging    LogConfig
    Telemetry  TelemetryConfig
    
    // Development configuration
    Development DevelopmentConfig
}
```

Main configuration structure for agents.

#### Configuration Options

| Option | Function | Description | Default |
|--------|----------|-------------|---------|
| `WithName` | `func WithName(name string) Option` | Set agent name | "gomind-agent" |
| `WithPort` | `func WithPort(port int) Option` | Set HTTP server port | 8080 |
| `WithAddress` | `func WithAddress(addr string) Option` | Set bind address | "0.0.0.0" |
| `WithNamespace` | `func WithNamespace(ns string) Option` | Set namespace | "default" |
| `WithRedisURL` | `func WithRedisURL(url string) Option` | Configure Redis discovery | "" |
| `WithCORS` | `func WithCORS(config CORSConfig) Option` | Configure CORS | disabled |
| `WithCORSDefaults` | `func WithCORSDefaults() Option` | Enable CORS with defaults | - |
| `WithDevelopmentMode` | `func WithDevelopmentMode() Option` | Enable development mode | false |
| `WithMockDiscovery` | `func WithMockDiscovery() Option` | Use mock discovery | false |

#### Configuration Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewConfig` | `func NewConfig(options ...Option) (*Config, error)` | Creates new configuration with options |
| `DefaultConfig` | `func DefaultConfig() *Config` | Returns default configuration |
| `LoadConfig` | `func LoadConfig(filename string) (*Config, error)` | Loads configuration from YAML file |
| `Validate` | `func (c *Config) Validate() error` | Validates configuration |

### Framework Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewFramework` | `func NewFramework(agent Agent, opts ...Option) (*Framework, error)` | Create new framework instance |
| `Run` | `func (f *Framework) Run(ctx context.Context) error` | Run the agent with all components |

---

## AI Module API

### OpenAI Client

#### Creation
```go
func NewOpenAIClient(apiKey string, logger ...core.Logger) *OpenAIClient
```

Creates a new OpenAI client. If apiKey is empty, uses `OPENAI_API_KEY` environment variable.

#### Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `GenerateResponse` | `func (c *OpenAIClient) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error)` | Generates AI response |

#### Supported Models
- `gpt-4`
- `gpt-4-turbo-preview`
- `gpt-3.5-turbo`
- `gpt-3.5-turbo-16k`

#### Default Configuration
- **Timeout**: 30 seconds
- **Base URL**: https://api.openai.com/v1
- **Default Model**: gpt-4
- **Default Temperature**: 0.7
- **Default MaxTokens**: 1000

### Intelligent Agent

#### Creation
```go
func NewIntelligentAgent(name string, apiKey string) *IntelligentAgent
```

Creates an agent with AI capabilities.

#### Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `EnableAI` | `func EnableAI(agent *core.BaseAgent, apiKey string)` | Adds AI to existing BaseAgent |
| `DiscoverAndUseTools` | `func (ia *IntelligentAgent) DiscoverAndUseTools(ctx context.Context, query string) (string, error)` | AI-powered tool discovery and orchestration |

---

## Orchestration Module API

### AI Orchestrator

#### OrchestratorConfig
```go
type OrchestratorConfig struct {
    RoutingMode       RouterMode
    SynthesisStrategy SynthesisStrategy
    ExecutionOptions  ExecutionOptions
    HistorySize       int
    CacheEnabled      bool
    CacheTTL          time.Duration
}
```

Configuration for AI orchestrator.

#### ExecutionOptions
```go
type ExecutionOptions struct {
    MaxConcurrency   int
    StepTimeout      time.Duration
    TotalTimeout     time.Duration
    RetryAttempts    int
    RetryDelay       time.Duration
    CircuitBreaker   bool
    FailureThreshold int
    RecoveryTimeout  time.Duration
}
```

Execution configuration options.

#### Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewAIOrchestrator` | `func NewAIOrchestrator(config *OrchestratorConfig, discovery core.Discovery, aiClient core.AIClient) *AIOrchestrator` | Creates AI-powered orchestrator |
| `ProcessRequest` | `func (o *AIOrchestrator) ProcessRequest(ctx context.Context, request string, metadata map[string]interface{}) (*OrchestratorResponse, error)` | Processes natural language request |
| `ExecutePlan` | `func (o *AIOrchestrator) ExecutePlan(ctx context.Context, plan *RoutingPlan) (*ExecutionResult, error)` | Executes routing plan |
| `GetExecutionHistory` | `func (o *AIOrchestrator) GetExecutionHistory() []ExecutionRecord` | Returns execution history |
| `GetMetrics` | `func (o *AIOrchestrator) GetMetrics() OrchestratorMetrics` | Returns orchestrator metrics |
| `Start` | `func (o *AIOrchestrator) Start(ctx context.Context) error` | Starts background processes |
| `Stop` | `func (o *AIOrchestrator) Stop() error` | Stops orchestrator |

### Workflow Engine

#### WorkflowDefinition
```go
type WorkflowDefinition struct {
    Name        string                          `yaml:"name"`
    Version     string                          `yaml:"version"`
    Description string                          `yaml:"description"`
    Inputs      map[string]InputDef             `yaml:"inputs"`
    Steps       []WorkflowStepDefinition        `yaml:"steps"`
    Outputs     map[string]string               `yaml:"outputs"`
    OnError     ErrorHandler                    `yaml:"on_error"`
    Timeout     time.Duration                   `yaml:"timeout"`
}
```

Workflow definition structure.

#### WorkflowStepDefinition
```go
type WorkflowStepDefinition struct {
    Name         string                 `yaml:"name"`
    Agent        string                 `yaml:"agent"`
    Capability   string                 `yaml:"capability"`
    Action       string                 `yaml:"action"`
    Inputs       map[string]interface{} `yaml:"inputs"`
    DependsOn    []string               `yaml:"depends_on"`
    Condition    *StepCondition         `yaml:"condition"`
    Retry        *RetryConfig           `yaml:"retry"`
    Timeout      time.Duration          `yaml:"timeout"`
    OnError      string                 `yaml:"on_error"`
    Transform    string                 `yaml:"transform"`
}
```

Individual workflow step definition.

#### Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewWorkflowEngine` | `func NewWorkflowEngine(discovery core.Discovery) *WorkflowEngine` | Creates workflow engine |
| `ParseWorkflowYAML` | `func (e *WorkflowEngine) ParseWorkflowYAML(yamlContent []byte) (*WorkflowDefinition, error)` | Parses workflow from YAML |
| `ExecuteWorkflow` | `func (e *WorkflowEngine) ExecuteWorkflow(ctx context.Context, workflow *WorkflowDefinition, inputs map[string]interface{}) (*WorkflowExecution, error)` | Executes workflow |
| `GetWorkflowStatus` | `func (e *WorkflowEngine) GetWorkflowStatus(executionID string) (*WorkflowExecution, error)` | Gets workflow status |
| `CancelWorkflow` | `func (e *WorkflowEngine) CancelWorkflow(executionID string) error` | Cancels running workflow |
| `GetMetrics` | `func (e *WorkflowEngine) GetMetrics() *WorkflowMetrics` | Returns workflow metrics |

### Types and Enums

#### RouterMode
```go
type RouterMode string

const (
    ModeAutonomous RouterMode = "autonomous"  // AI decides routing
    ModeWorkflow   RouterMode = "workflow"    // Use predefined workflow
    ModeHybrid     RouterMode = "hybrid"       // Combine both approaches
)
```

#### SynthesisStrategy
```go
type SynthesisStrategy string

const (
    StrategyLLM      SynthesisStrategy = "llm"       // Use LLM to synthesize
    StrategyTemplate SynthesisStrategy = "template"  // Use templates
    StrategySimple   SynthesisStrategy = "simple"    // Concatenate responses
    StrategyCustom   SynthesisStrategy = "custom"    // Custom function
)
```

---

## Resilience Module API

### Circuit Breaker

#### Creation
```go
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker
```

Creates a circuit breaker with failure threshold and timeout.

#### Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `CanExecute` | `func (cb *CircuitBreaker) CanExecute() bool` | Checks if the circuit breaker allows execution |
| `RecordSuccess` | `func (cb *CircuitBreaker) RecordSuccess()` | Records a successful operation |
| `RecordFailure` | `func (cb *CircuitBreaker) RecordFailure()` | Records a failed operation |
| `GetState` | `func (cb *CircuitBreaker) GetState() string` | Returns current state (Open/Closed) |
| `Reset` | `func (cb *CircuitBreaker) Reset()` | Manually resets the circuit breaker |

#### States
```go
type State int

const (
    StateClosed State = iota  // Normal operation
    StateOpen                 // Circuit is open, requests fail immediately
)
```

### Retry

#### Configuration
```go
type RetryConfig struct {
    MaxAttempts     int
    InitialDelay    time.Duration
    MaxDelay        time.Duration
    BackoffFactor   float64
    JitterEnabled   bool
}

func DefaultRetryConfig() *RetryConfig
```

Returns default retry configuration with sensible defaults.

#### Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `Retry` | `func Retry(ctx context.Context, config *RetryConfig, fn func() error) error` | Executes function with retry logic |
| `DefaultRetryConfig` | `func DefaultRetryConfig() *RetryConfig` | Returns default configuration |

#### Default Configuration
- MaxAttempts: 3
- InitialDelay: 100ms
- MaxDelay: 5s
- BackoffFactor: 2.0
- JitterEnabled: true

### Helper Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `RetryWithCircuitBreaker` | `func RetryWithCircuitBreaker(ctx context.Context, config *RetryConfig, cb *CircuitBreaker, fn func() error) error` | Combines retry and circuit breaker patterns |

---

## Telemetry Module API

### OTELProvider

#### Creation
```go
func NewOTelProvider(serviceName string, endpoint string) (*OTelProvider, error)
```

Creates OpenTelemetry provider with OTLP exporter to the specified endpoint. If endpoint is empty, uses localhost:4317 as default.

#### Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `StartSpan` | `func (p *OTELProvider) StartSpan(ctx context.Context, name string) (context.Context, core.Span)` | Starts a new span |
| `RecordMetric` | `func (p *OTELProvider) RecordMetric(name string, value float64, labels map[string]string)` | Records a metric (stub) |
| `Shutdown` | `func (p *OTELProvider) Shutdown(ctx context.Context) error` | Shuts down provider |

### OTELSpan

#### Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `End` | `func (s *OTELSpan) End()` | Ends the span |
| `SetAttribute` | `func (s *OTELSpan) SetAttribute(key string, value interface{})` | Sets span attribute |
| `RecordError` | `func (s *OTELSpan) RecordError(err error)` | Records error in span |

---

## HTTP API Endpoints

### Agent Endpoints

#### GET /health
Health check endpoint.

**Response:**
```json
{
    "status": "healthy",
    "agent": "agent-name",
    "id": "agent-id"
}
```

**Status Codes:**
- `200 OK`: Agent is healthy
- `503 Service Unavailable`: Agent is unhealthy

#### GET /api/capabilities
Lists agent capabilities.

**Response:**
```json
[
    {
        "name": "calculate",
        "description": "Performs calculations",
        "endpoint": "/api/calculate",
        "input_types": ["string"],
        "output_types": ["string"]
    }
]
```

**Status Codes:**
- `200 OK`: Success

#### POST /api/capabilities/{capability}
Invokes a specific capability (endpoint varies by capability).

**Request:**
```json
{
    "input": "capability-specific input"
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

**Status Codes:**
- `200 OK`: Success
- `400 Bad Request`: Invalid input
- `404 Not Found`: Capability not found
- `500 Internal Server Error`: Processing error

### CORS Support

All endpoints support CORS when enabled via configuration. Preflight requests are handled automatically.

**CORS Headers:**
- `Access-Control-Allow-Origin`
- `Access-Control-Allow-Methods`
- `Access-Control-Allow-Headers`
- `Access-Control-Allow-Credentials`
- `Access-Control-Max-Age`

---

## Error Types

### Common Errors

| Error | Description | Module |
|-------|-------------|--------|
| `ErrCircuitOpen` | Circuit breaker is open | Resilience |
| `ErrMaxRetriesExceeded` | Maximum retry attempts reached | Resilience |
| `ErrTimeout` | Operation timed out | Core |
| `ErrNotFound` | Resource not found | Core |
| `ErrInvalidConfig` | Invalid configuration | Core |
| `ErrDiscoveryUnavailable` | Discovery service unavailable | Core |
| `ErrWorkflowFailed` | Workflow execution failed | Orchestration |
| `ErrStepFailed` | Workflow step failed | Orchestration |
| `ErrInvalidWorkflow` | Invalid workflow definition | Orchestration |

---

## Default Configuration Values

These are the default values used by the framework (from struct tags and DefaultConfig functions):

### HTTP Defaults

| Setting | Default Value | Description |
|---------|---------------|-------------|
| Port | 8080 | HTTP server port |
| Health Check Path | "/health" | Health check endpoint |
| Read Timeout | 30s | HTTP read timeout |
| Write Timeout | 30s | HTTP write timeout |
| Idle Timeout | 120s | HTTP idle timeout |
| Max Header Bytes | 1048576 (1MB) | Maximum header size |
| Health Check Enabled | true | Health checks enabled by default |

### Retry Defaults (from DefaultRetryConfig)

| Setting | Default Value | Description |
|---------|---------------|-------------|
| Max Attempts | 3 | Maximum retry attempts |
| Initial Delay | 100ms | Initial retry delay |
| Max Delay | 5s | Maximum retry delay |
| Backoff Factor | 2.0 | Exponential backoff multiplier |
| Jitter Enabled | true | Random jitter enabled |

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENAI_API_KEY` | OpenAI API key for AI module | - |
| `REDIS_URL` | Redis connection URL | "redis://localhost:6379" |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OpenTelemetry endpoint | "localhost:4317" |
| `LOG_LEVEL` | Logging level (debug/info/warn/error) | "info" |
| `GOMIND_ENV` | Environment (development/production) | "production" |

### Import Paths

| Package | Import Path | Description |
|---------|------------|-------------|
| Framework (Main) | `github.com/itsneelabh/gomind` | Main framework package (re-exports core types) |
| Core | `github.com/itsneelabh/gomind/core` | Core agent framework |
| AI | `github.com/itsneelabh/gomind/ai` | AI capabilities and OpenAI client |
| Orchestration | `github.com/itsneelabh/gomind/orchestration` | Multi-agent orchestration |
| Telemetry | `github.com/itsneelabh/gomind/telemetry` | OpenTelemetry integration |
| Resilience | `github.com/itsneelabh/gomind/resilience` | Circuit breakers and retry patterns |

---

## Usage Examples

### Creating a Simple Agent

```go
package main

import (
    "context"
    "log"
    "github.com/itsneelabh/gomind/core"
)

func main() {
    // Create agent with configuration
    config := core.NewConfig(
        core.WithName("calculator"),
        core.WithPort(8080),
        core.WithRedisURL("redis://localhost:6379"),
        core.WithCORSDefaults(),
    )
    
    agent := core.NewBaseAgentWithConfig(config)
    
    // Add capability
    agent.RegisterCapability(core.Capability{
        Name:        "calculate",
        Description: "Performs mathematical calculations",
        Endpoint:    "/api/calculate",
        InputTypes:  []string{"expression"},
        OutputTypes: []string{"result"},
    })
    
    // Initialize and start
    ctx := context.Background()
    if err := agent.Initialize(ctx); err != nil {
        log.Fatal(err)
    }
    
    if err := agent.Start(8080); err != nil {
        log.Fatal(err)
    }
}
```

### Using AI Orchestration

```go
package main

import (
    "context"
    "fmt"
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/ai"
    "github.com/itsneelabh/gomind/orchestration"
)

func main() {
    // Set up components
    discovery, _ := core.NewRedisDiscovery("redis://localhost:6379")
    aiClient := ai.NewOpenAIClient(os.Getenv("OPENAI_API_KEY"))
    
    // Configure orchestrator
    config := orchestration.DefaultConfig()
    config.RoutingMode = orchestration.ModeAutonomous
    config.CacheEnabled = true
    
    // Create orchestrator
    orch := orchestration.NewAIOrchestrator(config, discovery, aiClient)
    
    // Start orchestrator
    ctx := context.Background()
    orch.Start(ctx)
    defer orch.Stop()
    
    // Process natural language request
    response, err := orch.ProcessRequest(ctx,
        "Analyze the stock market and give me recommendations",
        nil,
    )
    
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Response: %s\n", response.Response)
    fmt.Printf("Agents used: %v\n", response.AgentsInvolved)
    fmt.Printf("Execution time: %v\n", response.ExecutionTime)
}
```

### Implementing a Workflow

```go
package main

import (
    "context"
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/orchestration"
)

func main() {
    // Create workflow engine
    discovery, _ := core.NewRedisDiscovery("redis://localhost:6379")
    engine := orchestration.NewWorkflowEngine(discovery)
    
    // Define workflow in YAML
    yamlDef := `
name: data-processing
version: "1.0"
inputs:
  data:
    type: string
    required: true
steps:
  - name: validate
    agent: validator
    action: validate_data
    inputs:
      data: ${inputs.data}
  - name: process
    agent: processor
    action: process_data
    inputs:
      data: ${steps.validate.output}
    depends_on: [validate]
  - name: store
    agent: storage
    action: store_result
    inputs:
      result: ${steps.process.output}
    depends_on: [process]
outputs:
  result: ${steps.store.output}
`
    
    // Parse workflow
    workflow, _ := engine.ParseWorkflowYAML([]byte(yamlDef))
    
    // Execute workflow
    inputs := map[string]interface{}{
        "data": "sample data",
    }
    
    execution, err := engine.ExecuteWorkflow(context.Background(), workflow, inputs)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Workflow completed: %s\n", execution.Status)
    fmt.Printf("Result: %v\n", execution.Outputs["result"])
}
```

### Adding Resilience

```go
package main

import (
    "context"
    "time"
    "github.com/itsneelabh/gomind/resilience"
)

func main() {
    // Create circuit breaker
    cb := resilience.NewCircuitBreaker(5, 30*time.Second)
    
    // Create retry configuration
    retryConfig := resilience.DefaultRetryConfig()
    retryConfig.MaxAttempts = 3
    
    // Use combined pattern
    err := resilience.RetryWithCircuitBreaker(
        context.Background(),
        retryConfig,
        cb,
        func() error {
            // Your potentially failing operation
            _, err := callExternalAPI()
            return err
        },
    )
    
    if err != nil {
        log.Printf("Operation failed: %v", err)
        return
    }
    
    fmt.Println("Operation succeeded")
}
```

---

## Thread Safety

All components in the GoMind framework are designed to be thread-safe:

- **BaseAgent**: Safe for concurrent use
- **Discovery**: Thread-safe operations with Redis
- **Memory**: Thread-safe storage operations
- **CircuitBreaker**: Safe for concurrent Execute calls
- **Retry**: Safe for concurrent Execute calls
- **Orchestrator**: Safe for concurrent request processing
- **WorkflowEngine**: Safe for concurrent workflow execution

---

## Performance Considerations

### Resource Usage

| Component | Memory Footprint | CPU Usage | Network |
|-----------|-----------------|-----------|---------|
| Core Module | ~8MB | Low | Minimal |
| AI Module | ~2MB + model cache | Medium (API calls) | High (API) |
| Orchestration | ~1MB + cache | Medium | Medium |
| Resilience | <1MB | Minimal | None |
| Telemetry | ~10MB | Low | Low |

### Optimization Tips

1. **Discovery Caching**: Enable caching to reduce Redis queries
2. **Connection Pooling**: Reuse HTTP connections for agent communication
3. **Concurrent Execution**: Use orchestrator's MaxConcurrency setting
4. **Circuit Breakers**: Protect against cascading failures
5. **Workflow DAG**: Maximize parallelism in workflow definitions

---

## Migration Guide

### Installation and Import

```go
// Import the main framework (includes all re-exported core types)
import "github.com/itsneelabh/gomind"

// OR import specific subpackages directly
import "github.com/itsneelabh/gomind/core"
import "github.com/itsneelabh/gomind/ai"
import "github.com/itsneelabh/gomind/orchestration"
import "github.com/itsneelabh/gomind/telemetry"
import "github.com/itsneelabh/gomind/resilience"

// Create an agent (using main package or core package)
agent := gomind.NewBaseAgent("my-agent")  // Via main package
// OR
agent := core.NewBaseAgent("my-agent")    // Via core package
```

### Installation
```bash
# Install from main branch
go get github.com/itsneelabh/gomind@main
```

### Key Features
1. **Monolithic package**: All modules included in a single package
2. **Configuration**: Builder pattern for configuration
3. **Discovery**: Redis-based service discovery
4. **AI Integration**: Built-in OpenAI support

---

## Best Practices

1. **Always handle context cancellation** in long-running operations
2. **Use configuration files** for production deployments
3. **Enable telemetry** for production monitoring
4. **Implement graceful shutdown** for all agents
5. **Use circuit breakers** for external service calls
6. **Cache discovery results** to reduce latency
7. **Validate workflows** before deployment
8. **Monitor metrics** for performance optimization
9. **Use structured logging** for debugging
10. **Test with mock discovery** during development

---

## Support

For additional help and examples:
- [Getting Started Guide](GETTING_STARTED.md)
- [Framework Capabilities](framework_capabilities_guide.md)
- [GitHub Issues](https://github.com/itsneelabh/gomind/issues)
- [Example Code](https://github.com/itsneelabh/gomind/tree/main/examples)

---

*This API reference is for the GoMind framework. For specific versions, please refer to the tagged releases.*