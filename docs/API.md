# API Documentation

## Framework Core API

### Creating a Framework Instance

```go
func New(options ...Option) (*Framework, error)
```

Creates a new framework instance with the specified options.

**Parameters:**
- `options ...Option` - Configuration options for the framework

**Returns:**
- `*Framework` - The framework instance
- `error` - Error if initialization fails

**Example:**
```go
fw, err := framework.New(
    framework.WithPort(8080),
    framework.WithRedisURL("redis://localhost:6379"),
)
```

### Framework Options

#### WithPort
```go
func WithPort(port int) Option
```
Sets the HTTP server port.

#### WithRedisURL
```go
func WithRedisURL(url string) Option
```
Configures Redis connection for discovery and memory.

#### WithAI
```go
func WithAI(client ai.Client) Option
```
Sets the AI client for the framework.

#### WithLogLevel
```go
func WithLogLevel(level string) Option
```
Sets the logging level (debug, info, warn, error).

#### WithTelemetry
```go
func WithTelemetry(enabled bool) Option
```
Enables or disables telemetry collection.

### Framework Methods

#### Start
```go
func (f *Framework) Start(ctx context.Context) error
```
Starts the framework server and all components.

#### Stop
```go
func (f *Framework) Stop() error
```
Gracefully stops the framework and all components.

#### RegisterAgent
```go
func (f *Framework) RegisterAgent(id string, agent interface{}) error
```
Registers an agent with the framework.

#### GetAgent
```go
func (f *Framework) GetAgent(id string) (interface{}, error)
```
Retrieves a registered agent by ID.

#### Memory
```go
func (f *Framework) Memory() memory.Memory
```
Returns the memory interface for data persistence.

#### Discovery
```go
func (f *Framework) Discovery() discovery.Discovery
```
Returns the discovery interface for agent discovery.

#### Logger
```go
func (f *Framework) Logger() logger.Logger
```
Returns the logger interface.

---

## Package APIs

### pkg/agent

#### Agent Interface
```go
type Agent interface {
    ID() string
    Capabilities() []Capability
}
```

#### Capability
```go
type Capability struct {
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Inputs      []ParameterDef    `json:"inputs"`
    Outputs     []ParameterDef    `json:"outputs"`
}
```

#### Manager Interface
```go
type Manager interface {
    Register(id string, agent Agent) error
    Get(id string) (Agent, error)
    List() []Agent
    Invoke(agentID, capability string, input interface{}) (interface{}, error)
}
```

### pkg/ai

#### Client Interface
```go
type Client interface {
    Chat(ctx context.Context, request ChatRequest) (*ChatResponse, error)
    Stream(ctx context.Context, request ChatRequest) (<-chan StreamResponse, error)
}
```

#### ChatRequest
```go
type ChatRequest struct {
    Messages []Message `json:"messages"`
    Model    string    `json:"model,omitempty"`
    Stream   bool      `json:"stream,omitempty"`
}
```

#### Message
```go
type Message struct {
    Role    string `json:"role"`    // "user", "assistant", "system"
    Content string `json:"content"`
}
```

#### ChatResponse
```go
type ChatResponse struct {
    ID      string    `json:"id"`
    Choices []Choice  `json:"choices"`
    Usage   Usage     `json:"usage"`
}
```

#### OpenAI Client
```go
func NewOpenAIClient(apiKey string, options ...ClientOption) (*OpenAIClient, error)
```

### pkg/discovery

#### Discovery Interface
```go
type Discovery interface {
    Register(agent Agent) error
    Discover() ([]Agent, error)
    GetCapabilities(agentID string) ([]Capability, error)
    Health() error
}
```

#### Redis Discovery
```go
func NewRedisDiscovery(redisURL string) (*RedisDiscovery, error)
```

### pkg/memory

#### Memory Interface
```go
type Memory interface {
    Store(key string, value interface{}) error
    Retrieve(key string) (interface{}, error)
    Delete(key string) error
    List(pattern string) ([]string, error)
}
```

#### Redis Memory
```go
func NewRedisMemory(redisURL string) (*RedisMemory, error)
```

#### In-Memory Storage
```go
func NewInMemoryStorage() *InMemoryStorage
```

### pkg/logger

#### Logger Interface
```go
type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, fields ...Field)
    With(fields ...Field) Logger
}
```

#### Field
```go
type Field struct {
    Key   string
    Value interface{}
}
```

#### Simple Logger
```go
func NewSimpleLogger(level LogLevel) *SimpleLogger
```

### pkg/telemetry

#### Telemetry Interface
```go
type Telemetry interface {
    Counter(name string) Counter
    Histogram(name string) Histogram
    Gauge(name string) Gauge
    StartSpan(ctx context.Context, name string) (context.Context, Span)
}
```

#### OpenTelemetry Provider
```go
func NewOpenTelemetryProvider(serviceName string) (*OpenTelemetryProvider, error)
```

---

## HTTP API Endpoints

### Chat Endpoints

#### POST /ai/chat
Send a message to the AI assistant.

**Request:**
```json
{
    "message": "Hello, how can you help me?",
    "conversation_id": "conv_123",
    "model": "gpt-4"
}
```

**Response:**
```json
{
    "response": "I'm an AI assistant. I can help you with various tasks...",
    "conversation_id": "conv_123",
    "message_id": "msg_456"
}
```

#### GET /ai/stream
Stream AI responses using Server-Sent Events.

**Query Parameters:**
- `message` - The message to send
- `conversation_id` - Optional conversation ID
- `model` - Optional model name

**Response:**
Server-Sent Events stream with JSON chunks:
```
data: {"type": "start", "conversation_id": "conv_123"}
data: {"type": "chunk", "content": "I'm"}
data: {"type": "chunk", "content": " an"}
data: {"type": "chunk", "content": " AI"}
data: {"type": "end"}
```

### Agent Endpoints

#### GET /agents
List all registered agents.

**Response:**
```json
{
    "agents": [
        {
            "id": "market_agent",
            "capabilities": ["market_analysis", "trend_prediction"]
        }
    ]
}
```

#### GET /agents/{id}/capabilities
Get capabilities for a specific agent.

**Response:**
```json
{
    "agent_id": "market_agent",
    "capabilities": [
        {
            "name": "market_analysis",
            "description": "Analyzes market trends",
            "inputs": [
                {
                    "name": "market_data",
                    "type": "string",
                    "description": "Historical market data"
                }
            ],
            "outputs": [
                {
                    "name": "analysis",
                    "type": "object",
                    "description": "Market analysis results"
                }
            ]
        }
    ]
}
```

#### POST /agents/{id}/invoke
Invoke an agent capability.

**Request:**
```json
{
    "capability": "market_analysis",
    "input": {
        "market_data": "..."
    }
}
```

**Response:**
```json
{
    "result": {
        "analysis": "Market shows upward trend..."
    },
    "execution_time": "150ms"
}
```

### System Endpoints

#### GET /health
Health check endpoint.

**Response:**
```json
{
    "status": "healthy",
    "components": {
        "redis": "connected",
        "ai": "available",
        "memory": "operational"
    },
    "timestamp": "2025-08-14T23:00:00Z"
}
```

#### GET /metrics
Prometheus metrics endpoint.

**Response:**
```
# HELP framework_requests_total Total number of requests
# TYPE framework_requests_total counter
framework_requests_total{method="GET",endpoint="/health"} 42

# HELP framework_request_duration_seconds Request duration in seconds
# TYPE framework_request_duration_seconds histogram
framework_request_duration_seconds_bucket{le="0.1"} 10
```

---

## Error Handling

All API endpoints return errors in a consistent format:

```json
{
    "error": {
        "code": "INVALID_REQUEST",
        "message": "The provided request is invalid",
        "details": {
            "field": "message",
            "reason": "cannot be empty"
        }
    },
    "request_id": "req_789"
}
```

### Common Error Codes

- `INVALID_REQUEST` - Request validation failed
- `AGENT_NOT_FOUND` - Specified agent does not exist
- `CAPABILITY_NOT_FOUND` - Specified capability does not exist
- `AI_SERVICE_ERROR` - AI service is unavailable
- `INTERNAL_ERROR` - Internal server error
- `RATE_LIMITED` - Too many requests

---

## Authentication

The framework supports multiple authentication methods:

### API Key Authentication
```bash
curl -H "Authorization: Bearer your-api-key" \
     -X POST http://localhost:8080/ai/chat
```

### JWT Token Authentication
```bash
curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
     -X POST http://localhost:8080/ai/chat
```

### Custom Authentication
Implement the `Authenticator` interface for custom authentication:

```go
type Authenticator interface {
    Authenticate(ctx context.Context, token string) (*User, error)
}
```
