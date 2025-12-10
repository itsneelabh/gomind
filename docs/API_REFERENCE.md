# API Reference

A comprehensive guide to GoMind's APIs with practical examples and best practices.

## Quick Navigation

**Getting Started:**
- [NewFramework](#newframework) - Bootstrap your component with the framework
- [NewBaseAgent](#newbaseagent) - Create agents that discover and orchestrate
- [NewTool](#newtool) - Build tools that provide specific capabilities
- [NewAIAgent](#newaiagent) - Create AI-powered orchestration agents

**Key Features:**
- [RegisterCapability](#registercapability) - Define capabilities with AI-powered payload generation (3-phase approach)
- [Schema Cache](#schema-cache-phase-3-validation) - Redis-backed schema caching for validation
- [Schema Discovery](#registercapability) - Progressive enhancement: Phase 1 (descriptions) → Phase 2 (field hints) → Phase 3 (validation)

**By Module:**
- [Core](#core-module) - Foundation types and component lifecycle
- [AI](#ai-module) - AI provider integration and intelligent agents
- [Resilience](#resilience-module) - Circuit breakers and retry mechanisms
- [Telemetry](#telemetry-module) - Metrics, tracing, and observability
- [Orchestration](#orchestration-module) - Multi-agent coordination
- [UI](#ui-module) - Chat interfaces and web transports

---

## Core Module

The foundation of GoMind - components, discovery, and lifecycle management.

### Component Interface

Every GoMind component (tools and agents) implements this interface, providing a consistent API for initialization, identification, and capability discovery.

```go
type Component interface {
    Initialize(ctx context.Context) error
    GetID() string
    GetName() string
    GetCapabilities() []Capability
    GetType() ComponentType
}
```

**Why this matters:** This unified interface allows the framework to manage any component type consistently, whether it's a simple tool or complex orchestration agent.

**Example:**
```go
// Any component can be queried uniformly
func inspectComponent(comp core.Component) {
    fmt.Printf("Component: %s (ID: %s)\n", comp.GetName(), comp.GetID())
    fmt.Printf("Type: %s\n", comp.GetType()) // "agent" or "tool"

    caps := comp.GetCapabilities()
    fmt.Printf("Capabilities (%d):\n", len(caps))
    for _, cap := range caps {
        fmt.Printf("  - %s: %s\n", cap.Name, cap.Description)
    }
}
```

### NewBaseAgent

Creates an agent - an active component that can discover and orchestrate other services. Agents are the "brains" of your system, capable of finding tools and other agents to accomplish complex tasks.

```go
func NewBaseAgent(name string) *BaseAgent
func NewBaseAgentWithConfig(config *Config) *BaseAgent
```

**When to use agents vs tools:**
- **Use an Agent when:** You need to discover services, coordinate multiple components, or build orchestrators
- **Use a Tool when:** You're providing a specific capability that others will use

**Example - Simple Agent:**
```go
// Quick start with minimal config
agent := core.NewBaseAgent("data-processor")

// Agent can discover other services (tools can't!)
services, _ := agent.Discover(ctx, core.DiscoveryFilter{
    Type: core.ComponentTypeTool,
    Capabilities: []string{"database"},
})

agent.Initialize(ctx)
agent.Start(ctx, 8080)
```

**Example - Production Agent:**
```go
// Full configuration for production
config := core.NewConfig(
    core.WithName("orchestrator"),
    core.WithPort(8080),
    core.WithRedisURL("redis://redis:6379"),      // Enable discovery
    core.WithLogLevel("info"),
    core.WithEnableMetrics(true),
)

agent := core.NewBaseAgentWithConfig(config)

// Register what this agent can do
agent.RegisterCapability(core.Capability{
    Name:        "orchestrate",
    Description: "Coordinate multiple services to complete tasks",
    Handler: func(w http.ResponseWriter, r *http.Request) {
        // Handler implementation
    },
})

// Initialize connects to Redis, sets up telemetry, etc.
if err := agent.Initialize(ctx); err != nil {
    log.Fatal("Failed to initialize:", err)
}

// Start begins serving HTTP requests
if err := agent.Start(ctx, 8080); err != nil {
    log.Fatal("Failed to start:", err)
}
```

### NewTool

Creates a tool - a passive component that provides specific capabilities. Tools are discovered and used by agents but cannot discover other components themselves.

```go
func NewTool(name string) *BaseTool
func NewToolWithConfig(config *Config) *BaseTool
```

**Tools are perfect for:**
- Microservices that perform specific tasks
- API wrappers (weather, database, external services)
- Stateless functions exposed as services
- Any capability that doesn't need orchestration

**Example - Weather Tool:**
```go
// Create a specialized tool
weatherTool := core.NewTool("weather-service")

// Define what it can do
weatherTool.RegisterCapability(core.Capability{
    Name:        "get_current_weather",
    Description: "Get current weather for any city",
    Endpoint:    "/api/weather/current",
    Handler: func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            City string `json:"city"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        weather := fetchWeatherData(req.City)

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(weather)
    },
})

// Tools still need initialization and startup
weatherTool.Initialize(ctx)
weatherTool.Start(ctx, 8081) // Different port from agents
```

**Key differences between Agents and Tools:**

| Feature | Agent | Tool |
|---------|-------|------|
| Can discover services | ✅ Yes | ❌ No |
| Can be discovered | ✅ Yes | ✅ Yes |
| Has HTTP server | ✅ Yes | ✅ Yes |
| Typical use case | Orchestrator, Router | Microservice, API |
| Complexity | Higher | Lower |

### Discover

**Agent-only capability** for finding components in your system. This is how agents build a dynamic picture of available services.

```go
func (a *BaseAgent) Discover(ctx context.Context, filter DiscoveryFilter) ([]*ServiceInfo, error)
```

**DiscoveryFilter fields:**
- `Type` - Filter by ComponentType (agent/tool)
- `Capabilities` - Required capability names
- `Name` - Exact service name match
- `Metadata` - Custom key-value filters

**Example - Smart Service Discovery:**
```go
// Find all database tools
dbTools, err := agent.Discover(ctx, core.DiscoveryFilter{
    Type: core.ComponentTypeTool,
    Capabilities: []string{"database"},
})

// Find services in specific region (using metadata)
regionalServices, err := agent.Discover(ctx, core.DiscoveryFilter{
    Metadata: map[string]interface{}{
        "region": "us-west",
        "environment": "production",
    },
})

// Find any service that can translate
translators, err := agent.Discover(ctx, core.DiscoveryFilter{
    Capabilities: []string{"translate"},
})

// Load balance across discovered services
service := translators[rand.Intn(len(translators))]
response, err := http.Post(service.Address, "application/json", payload)
```

**Pro tip:** Discovery results are cached for 5 minutes by default to reduce Redis load. Use `WithDiscoveryCacheEnabled(false)` for real-time discovery in development.

### RegisterCapability

Add capabilities to any component. This is how you define what your component can do.

```go
func (c *BaseAgent) RegisterCapability(cap Capability)
func (c *BaseTool) RegisterCapability(cap Capability)
```

**Capability structure:**
```go
type Capability struct {
    Name           string           // Unique identifier
    Description    string           // Human-readable description for AI (Phase 1)
    Endpoint       string           // HTTP endpoint path
    InputTypes     []string         // Accepted content types
    OutputTypes    []string         // Response content types
    Handler        http.HandlerFunc // HTTP handler function

    // Phase 2: Schema discovery fields for AI-powered payload generation
    InputSummary   *SchemaSummary   // Compact field hints for AI (Phase 2)
    OutputSummary  *SchemaSummary   // Output schema hints (optional)
    SchemaEndpoint string           // Full JSON Schema endpoint (Phase 3)
}

// SchemaSummary provides compact field hints for AI payload generation (Phase 2)
type SchemaSummary struct {
    RequiredFields []FieldHint  // Required input fields
    OptionalFields []FieldHint  // Optional input fields
}

// FieldHint describes a single field for AI understanding
type FieldHint struct {
    Name        string  // Field name (exact, used in JSON)
    Type        string  // Field type (string, number, boolean, object, array)
    Example     string  // Example value for AI
    Description string  // Human-readable description
}
```

**Example - Phase 1 (Basic - Description Only):**
```go
// Minimal capability - AI generates payloads from description alone (~85-90% accuracy)
tool.RegisterCapability(core.Capability{
    Name:        "current_weather",
    Description: "Gets current weather for a location. Required: location (city name). Optional: units (metric/imperial).",
    Handler:     handleWeather,
})
```

**Example - Phase 2 (Recommended - With Field Hints):**
```go
// Enhanced capability with field hints for better AI accuracy (~95%)
tool.RegisterCapability(core.Capability{
    Name:        "current_weather",
    Description: "Gets current weather conditions for a location",
    InputTypes:  []string{"json"},
    OutputTypes: []string{"json"},
    Handler:     handleWeather,

    // Phase 2: Add structured field hints for AI payload generation
    InputSummary: &core.SchemaSummary{
        RequiredFields: []core.FieldHint{
            {
                Name:        "location",
                Type:        "string",
                Example:     "London",
                Description: "City name or coordinates (lat,lon)",
            },
        },
        OptionalFields: []core.FieldHint{
            {
                Name:        "units",
                Type:        "string",
                Example:     "metric",
                Description: "Temperature unit: metric or imperial",
            },
        },
    },
})
```

**Example - Phase 3 (Mission-Critical - With Validation):**
```go
// Full capability with schema validation endpoint for maximum reliability (~99%)
tool.RegisterCapability(core.Capability{
    Name:        "process_payment",
    Description: "Process a payment transaction",
    Handler:     handlePayment,

    // Phase 2: Field hints for AI generation
    InputSummary: &core.SchemaSummary{
        RequiredFields: []core.FieldHint{
            {Name: "amount", Type: "number", Example: "99.99", Description: "Payment amount"},
            {Name: "currency", Type: "string", Example: "USD", Description: "Currency code"},
            {Name: "card_number", Type: "string", Example: "4111111111111111", Description: "Credit card number"},
        },
    },

    // Phase 3: Schema endpoint for validation (auto-generated at /api/capabilities/process_payment/schema)
    // Framework automatically serves JSON Schema at this endpoint
})
```

### Schema Cache (Phase 3 Validation)

Redis-backed caching for JSON Schemas used in Phase 3 validation. Schemas are fetched once and cached forever, providing zero-overhead validation after initial fetch.

#### NewSchemaCache

Create a Redis-backed schema cache for agents that perform Phase 3 validation.

```go
func NewSchemaCache(redisClient *redis.Client, opts ...SchemaCacheOption) SchemaCache
```

**SchemaCache interface:**
```go
type SchemaCache interface {
    // Get retrieves a cached schema by tool and capability name
    Get(ctx context.Context, toolName, capabilityName string) (map[string]interface{}, bool)

    // Set stores a schema in the cache
    Set(ctx context.Context, toolName, capabilityName string, schema map[string]interface{}) error

    // Stats returns cache statistics for monitoring
    Stats() map[string]interface{}
}
```

**Example - Basic Schema Cache:**
```go
// In your agent initialization
redisOpt, _ := redis.ParseURL(os.Getenv("REDIS_URL"))
redisClient := redis.NewClient(redisOpt)

// Create schema cache with defaults (24-hour TTL)
agent.SchemaCache = core.NewSchemaCache(redisClient)
```

**Example - Custom Configuration:**
```go
// Production configuration with custom TTL and prefix
agent.SchemaCache = core.NewSchemaCache(redisClient,
    core.WithTTL(1 * time.Hour),          // Shorter TTL for frequently changing schemas
    core.WithPrefix("myapp:schemas:"),    // Custom prefix for multi-tenant deployments
)

// Enable validation via environment variable
// export GOMIND_VALIDATE_PAYLOADS=true

// Agent will now:
// 1. Generate payloads using AI + field hints (Phase 1/2)
// 2. Fetch schema from tool's /schema endpoint (once)
// 3. Cache schema in Redis (shared across all agent replicas)
// 4. Validate all future payloads against cached schema
```

**Cache configuration options:**

```go
// WithTTL sets cache expiration time
WithTTL(ttl time.Duration) SchemaCacheOption

// WithPrefix sets Redis key prefix (for namespacing)
WithPrefix(prefix string) SchemaCacheOption
```

**Monitoring cache performance:**
```go
// Get cache statistics
stats := agent.SchemaCache.Stats()
// Returns: {"hits": 150, "misses": 3, "total_lookups": 153, "hit_rate": 0.98}

// Log statistics periodically
ticker := time.NewTicker(1 * time.Minute)
go func() {
    for range ticker.C {
        stats := agent.SchemaCache.Stats()
        logger.Info("Schema cache stats", stats)
    }
}()
```

**When to use Schema Cache:**

| Scenario | Use Schema Cache? | Reason |
|----------|------------------|--------|
| **Development** | No | Schemas change frequently, overhead not worth it |
| **Production agents** | Yes | Shared cache across replicas, validates critical payloads |
| **Mission-critical APIs** | Yes | ~99% accuracy with validation |
| **Simple tools** | No | Phase 1+2 sufficient for most cases |

**Best practices:**
- Enable only when `GOMIND_VALIDATE_PAYLOADS=true`
- Use shared Redis instance for all agent replicas
- Monitor cache hit rate (should be >95% after warmup)
- Set reasonable TTL (24 hours default, schemas rarely change)
- Use custom prefix for multi-tenant deployments

### Component Lifecycle Management

Every component follows a three-phase lifecycle: Initialize → Start → Stop/Shutdown. Understanding this lifecycle is crucial for building robust services.

```go
// Initialize - Connect to dependencies, load config
func (c *Component) Initialize(ctx context.Context) error

// Start - Begin serving HTTP requests
func (c *Component) Start(ctx context.Context, port int) error

// Stop/Shutdown - Graceful shutdown with timeout
func (c *Component) Stop(ctx context.Context) error     // Agents
func (c *Component) Shutdown(ctx context.Context) error  // Tools
```

**Lifecycle best practices:**

1. **Initialize Phase** - One-time setup
   - Connect to databases
   - Load configuration
   - Set up telemetry
   - Register with discovery

2. **Start Phase** - Begin operations
   - Start HTTP server
   - Begin health checks
   - Accept incoming requests

3. **Stop Phase** - Clean shutdown
   - Stop accepting new requests
   - Wait for in-flight requests
   - Unregister from discovery
   - Close connections

**Example - Production Lifecycle:**
```go
func main() {
    ctx := context.Background()

    // Create and configure
    agent := core.NewBaseAgent("my-service")
    agent.RegisterCapability(/* ... */)

    // Initialize (connects to Redis, etc.)
    if err := agent.Initialize(ctx); err != nil {
        log.Fatal("Initialization failed:", err)
    }

    // Start in goroutine
    errChan := make(chan error, 1)
    go func() {
        if err := agent.Start(ctx, 8080); err != nil {
            errChan <- err
        }
    }()

    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    select {
    case <-sigChan:
        log.Info("Received shutdown signal")
    case err := <-errChan:
        log.Error("Server error:", err)
    }

    // Graceful shutdown with timeout
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := agent.Stop(shutdownCtx); err != nil {
        log.Error("Graceful shutdown failed:", err)
        os.Exit(1)
    }

    log.Info("Shutdown complete")
}
```

### NewFramework

The main entry point for running GoMind components. The framework handles all the complex setup - discovery, telemetry, configuration - so you can focus on your business logic.

```go
func NewFramework(component HTTPComponent, opts ...Option) (*Framework, error)
```

**Why use the framework:**
- Automatic configuration from environment variables
- Built-in health checks and metrics
- Graceful shutdown handling
- Service discovery registration
- Standardized logging

**Example - Simple:**
```go
// Minimal setup - just works!
agent := core.NewBaseAgent("my-agent")
framework, _ := core.NewFramework(agent)
framework.Run(ctx)
```

**Example - Production:**
```go
// Full production setup
agent := createMyAgent()

framework, err := core.NewFramework(agent,
    // Discovery
    core.WithRedisURL("redis://redis:6379"),

    // Observability
    core.WithTelemetry(true, "http://otel:4317"),
    core.WithLogLevel("info"),

    // Networking
    core.WithPort(8080),
    core.WithCORSDefaults(),

    // Features
    core.WithEnableMetrics(true),
    core.WithEnableTracing(true),
)

if err != nil {
    log.Fatal("Framework creation failed:", err)
}

// Run blocks until shutdown
if err := framework.Run(ctx); err != nil {
    log.Fatal("Framework run failed:", err)
}
```

### Configuration

GoMind's configuration system is designed for flexibility - use code, environment variables, or config files.

#### NewConfig

Create configuration programmatically with validation and defaults.

```go
func NewConfig(options ...Option) (*Config, error)
func DefaultConfig() *Config
```

**Configuration priority (highest to lowest):**
1. Explicit options in code
2. Environment variables
3. Config file (if specified)
4. Default values

**Example - Different Configuration Styles:**
```go
// 1. Code-based (explicit, testable)
config := core.NewConfig(
    core.WithName("weather-service"),
    core.WithPort(8080),
    core.WithRedisDiscovery("redis://localhost:6379"),
)

// 2. Environment-based (12-factor app)
// Set: GOMIND_NAME=weather-service
// Set: GOMIND_PORT=8080
// Set: REDIS_URL=redis://localhost:6379
// Set: GOMIND_ORCHESTRATION_TIMEOUT=5m  // For long-running AI workflows
config := core.DefaultConfig() // Reads from env

// 3. File-based (complex configs)
config := core.NewConfig(
    core.WithConfigFile("/etc/gomind/config.yaml"),
)
```

#### Key Configuration Options

**Service Discovery:**
```go
// Enable Redis discovery
WithRedisURL(url string)           // Redis connection
WithRedisDiscovery(url string)     // Shorthand for Redis setup
WithDiscoveryCacheEnabled(bool)    // Cache discovery results
WithMockDiscovery(bool)            // Use mock for testing

// Background retry for Redis connection failures (opt-in)
WithDiscoveryRetry(bool)           // Enable background retry on failure
WithDiscoveryRetryInterval(d)      // Set initial retry interval (default: 30s)
```

**AI Integration:**
```go
WithAI(enabled bool, model string)  // Enable AI with model
WithOpenAIAPIKey(key string)        // Set OpenAI key
WithAIModel(model string)           // Choose model (gpt-4, claude-3, etc.)
WithMockAI(bool)                    // Use mock for testing
```

**Observability:**
```go
WithTelemetry(enabled bool, endpoint string)  // Enable telemetry
WithEnableMetrics(bool)                       // Metrics only
WithEnableTracing(bool)                       // Tracing only
WithLogLevel(level string)                    // error, warn, info, debug
WithLogFormat(format string)                  // json or text
```

**Networking:**
```go
WithPort(port int)                  // HTTP port
WithAddress(addr string)            // Bind address
WithCORS(config CORSConfig)         // CORS settings
WithCORSDefaults()                  // Permissive CORS for dev
```

**Advanced:**
```go
WithMemoryProvider(provider string) // "inmemory" or "redis"
WithCircuitBreaker(config)          // Resilience settings
WithRetry(config)                   // Retry configuration
WithKubernetes(discovery, leader)   // K8s integration
WithDevelopmentMode()               // Debug logging, mock services
```

### Logging

GoMind provides structured logging with automatic context propagation. The framework automatically injects loggers into all components.

**Log Levels (hierarchical):**
- `error` - Critical errors only
- `warn` - Warnings and errors
- `info` - Normal operations (default)
- `debug` - Detailed debugging information

**Using the Logger:**
```go
// Every component gets a logger automatically
func (t *MyTool) ProcessRequest(ctx context.Context, req Request) error {
    // Structured logging with context
    t.Logger.Info("Processing request", map[string]interface{}{
        "request_id": req.ID,
        "user_id":    req.UserID,
        "action":     req.Action,
    })

    result, err := t.performAction(req)
    if err != nil {
        t.Logger.Error("Action failed", map[string]interface{}{
            "error":      err.Error(),
            "request_id": req.ID,
            "duration":   time.Since(start).Milliseconds(),
        })
        return err
    }

    t.Logger.Debug("Action completed", map[string]interface{}{
        "request_id": req.ID,
        "result":     result,
    })

    return nil
}
```

**Configuration via Environment:**
```bash
# Production
export GOMIND_LOG_LEVEL=info
export GOMIND_LOG_FORMAT=json

# Development
export GOMIND_LOG_LEVEL=debug
export GOMIND_LOG_FORMAT=text

# Or use dev mode (sets debug + text automatically)
export GOMIND_DEV_MODE=true
```

### Interfaces

#### Discovery Interface

Service discovery interface for agents. Combines registration with powerful query capabilities.

```go
type Discovery interface {
    Registry  // Embed Registry (Register, UpdateHealth, Unregister)

    // Query methods
    Discover(ctx context.Context, filter DiscoveryFilter) ([]*ServiceInfo, error)
    FindService(ctx context.Context, serviceName string) ([]*ServiceInfo, error)
    FindByCapability(ctx context.Context, capability string) ([]*ServiceInfo, error)
}
```

**Method usage guide:**

| Method | Use When | Example |
|--------|----------|---------|
| `Discover()` | Complex multi-criteria search | Find healthy services in region with capability |
| `FindService()` | Know exact service name | Find all "user-service" instances |
| `FindByCapability()` | Need any service with capability | Find any translator |

**Example - Building a Load Balancer:**
```go
type LoadBalancer struct {
    discovery core.Discovery
    current   uint32
}

func (lb *LoadBalancer) GetService(ctx context.Context, capability string) (*core.ServiceInfo, error) {
    // Find all healthy services
    services, err := lb.discovery.Discover(ctx, core.DiscoveryFilter{
        Capability: capability,
        HealthOnly: true,
    })

    if len(services) == 0 {
        return nil, fmt.Errorf("no services available for %s", capability)
    }

    // Round-robin selection
    index := atomic.AddUint32(&lb.current, 1) % uint32(len(services))
    return services[index], nil
}
```

#### Registry Interface

Service registration interface for tools. Handles registration, health updates, and cleanup.

```go
type Registry interface {
    Register(ctx context.Context, info *ServiceInfo) error
    UpdateHealth(ctx context.Context, id string, status HealthStatus) error
    Unregister(ctx context.Context, id string) error
}
```

**Registration lifecycle:**
```go
// 1. Register on startup
info := &core.ServiceInfo{
    ID:       "weather-1",
    Name:     "weather-service",
    Type:     "tool",
    Address:  "http://localhost:8081",
    Health:   core.HealthHealthy,
}
registry.Register(ctx, info)

// 2. Periodic health updates
ticker := time.NewTicker(30 * time.Second)
go func() {
    for range ticker.C {
        health := checkHealth()
        registry.UpdateHealth(ctx, info.ID, health)
    }
}()

// 3. Unregister on shutdown
defer registry.Unregister(ctx, info.ID)
```

#### Background Redis Retry

GoMind provides an intelligent background retry mechanism for handling Redis connection failures during service startup. This is particularly useful in Kubernetes environments where Redis may not be immediately available.

**Key features:**
- **Opt-in by default** - Backward compatible, must be explicitly enabled
- **Exponential backoff** - Retry intervals double on each failure (30s → 60s → 120s → 240s → 300s cap)
- **Automatic re-registration** - When Redis becomes available, service is automatically registered
- **Thread-safe** - Registry references are updated atomically

**Configuration:**
```go
// Enable via environment variables
// export GOMIND_DISCOVERY_RETRY=true
// export GOMIND_DISCOVERY_RETRY_INTERVAL=30s

// Or via code configuration
config := core.NewConfig(
    core.WithRedisURL("redis://redis:6379"),
    core.WithDiscoveryRetry(true),
    core.WithDiscoveryRetryInterval(30 * time.Second),
)
```

**How it works:**
1. Service attempts initial Redis connection during startup
2. If connection fails and retry is enabled, service starts normally (without discovery)
3. Background goroutine attempts reconnection at configured intervals
4. On successful reconnection, service is registered and heartbeat begins
5. Parent component's registry reference is updated via callback

**Example - Kubernetes Deployment:**
```yaml
env:
  - name: REDIS_URL
    value: "redis://redis:6379"
  - name: GOMIND_DISCOVERY_RETRY
    value: "true"
  - name: GOMIND_DISCOVERY_RETRY_INTERVAL
    value: "30s"
```

**When to use:**
- Kubernetes environments where Redis may start after your service
- Systems with intermittent Redis connectivity
- Services that should remain functional even without discovery

**When NOT to use:**
- Development environments (use mock discovery instead)
- When Redis availability is a hard requirement

#### Memory Interface

Abstract storage for state, sessions, and caching.

```go
type Memory interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key string, value string, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
}
```

**Example - Session Management:**
```go
type SessionManager struct {
    store core.Memory
}

func (sm *SessionManager) CreateSession(userID string) (string, error) {
    sessionID := generateSessionID()
    data := map[string]string{
        "user_id": userID,
        "created": time.Now().Format(time.RFC3339),
    }

    jsonData, _ := json.Marshal(data)

    // Session expires in 24 hours
    err := sm.store.Set(ctx, "session:"+sessionID, string(jsonData), 24*time.Hour)
    return sessionID, err
}

func (sm *SessionManager) ValidateSession(sessionID string) (string, error) {
    data, err := sm.store.Get(ctx, "session:"+sessionID)
    if err != nil {
        return "", errors.New("invalid or expired session")
    }

    var session map[string]string
    json.Unmarshal([]byte(data), &session)
    return session["user_id"], nil
}
```

### Memory Implementations

#### NewInMemoryStore

Fast, local memory storage with automatic expiration. Perfect for development and single-instance deployments.

```go
func NewInMemoryStore() *InMemoryStore
```

**Features:**
- **Automatic TTL expiration** - Items expire based on TTL
- **Background cleanup** - Removes expired items every 10 minutes
- **Thread-safe** - Concurrent access with RWMutex
- **Capacity limited** - Max 1000 items (prevents memory leaks)

**When to use:**
- Development and testing
- Single-instance deployments
- Temporary caching
- Session storage for small apps

**Example - Rate Limiting:**
```go
func RateLimitMiddleware(store core.Memory, limit int) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            clientIP := r.RemoteAddr
            key := "rate:" + clientIP

            // Check current count
            val, err := store.Get(ctx, key)
            if err != nil {
                // First request - allow and start counting
                store.Set(ctx, key, "1", 1*time.Minute)
                next.ServeHTTP(w, r)
                return
            }

            count, _ := strconv.Atoi(val)
            if count >= limit {
                http.Error(w, "Rate limit exceeded", 429)
                return
            }

            // Increment counter
            store.Set(ctx, key, strconv.Itoa(count+1), 1*time.Minute)
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## AI Module

Connect to AI providers and build intelligent agents that leverage LLMs for natural language understanding and generation.

### NewClient

Create an AI client with automatic provider detection. The client intelligently selects the best available AI provider based on environment variables.

```go
func NewClient(opts ...AIOption) (core.AIClient, error)
func MustNewClient(opts ...AIOption) core.AIClient  // Panics on error
```

**Provider auto-detection order:**
1. OpenAI (`OPENAI_API_KEY`)
2. Anthropic (`ANTHROPIC_API_KEY`)
3. Google Gemini (`GEMINI_API_KEY`)
4. Groq (`GROQ_API_KEY`)
5. Together AI (`TOGETHER_API_KEY`) - Priority 75
6. DeepSeek (`DEEPSEEK_API_KEY`) - Priority 80
7. xAI Grok (`XAI_API_KEY`) - Priority 85
8. Qwen (`QWEN_API_KEY`) - Priority 90

**Example - Auto-detect Provider:**
```go
// Automatically uses first available provider
client, err := ai.NewClient()
if err != nil {
    log.Fatal("No AI provider available. Set OPENAI_API_KEY or ANTHROPIC_API_KEY")
}

response, _ := client.GenerateResponse(ctx, "Explain quantum computing", nil)
fmt.Println(response.Content)
```

**Example - Specific Provider:**
```go
// Use specific provider with custom settings
client, err := ai.NewClient(
    ai.WithProvider("anthropic"),
    ai.WithModel("claude-3-opus-20240229"),
    ai.WithTemperature(0.7),
    ai.WithMaxTokens(2000),
)

// Use for complex reasoning tasks
response, _ := client.GenerateResponse(ctx,
    "Analyze this code for potential issues and suggest improvements",
    &core.AIOptions{
        Temperature: 0.3,  // Lower temperature for technical analysis
    },
)
```

### WithProviderAlias

Configure OpenAI-compatible providers with automatic endpoint and model resolution. Provider aliases offer a clean way to use alternative AI providers that implement the OpenAI API specification.

```go
func WithProviderAlias(alias string) AIOption
```

**Supported provider aliases:**
- `"openai"` - Standard OpenAI (default)
- `"openai.deepseek"` - DeepSeek with reasoning models
- `"openai.groq"` - Groq for ultra-fast inference
- `"openai.together"` - Together AI for open models
- `"openai.xai"` - xAI Grok models
- `"openai.qwen"` - Alibaba Qwen models

**Features:**
- **Automatic endpoint configuration** - No need to specify base URLs
- **Model aliases** - Use portable names like "smart", "fast", "code"
- **Environment variable support** - Override endpoints via environment
- **Three-tier configuration** - Explicit → Environment → Defaults

**Example - Using Alternative Providers:**
```go
// Use DeepSeek's reasoning model
client, _ := ai.NewClient(
    ai.WithProviderAlias("openai.deepseek"),
    ai.WithModel("smart"),  // Resolves to "deepseek-reasoner"
)

// Use Groq for fast inference
client, _ := ai.NewClient(
    ai.WithProviderAlias("openai.groq"),
    ai.WithModel("fast"),   // Resolves to "llama-3.3-70b-versatile"
)

// Use Together AI with explicit model
client, _ := ai.NewClient(
    ai.WithProviderAlias("openai.together"),
    ai.WithModel("meta-llama/Llama-3-70b-chat-hf"), // Explicit model name
)
```

**Configuration priority:**
```go
// 1. Explicit configuration (highest priority)
client, _ := ai.NewClient(
    ai.WithProviderAlias("openai.groq"),
    ai.WithAPIKey("explicit-key"),        // Overrides environment
    ai.WithBaseURL("https://custom.url"), // Overrides defaults
)

// 2. Environment variables (medium priority)
// Set: GROQ_API_KEY=your-key
// Set: GROQ_BASE_URL=https://custom.groq.com  // Optional override
client, _ := ai.NewClient(
    ai.WithProviderAlias("openai.groq"),
)

// 3. Hardcoded defaults (lowest priority)
// Uses built-in endpoints like https://api.groq.com/openai/v1
```

### Model Aliases

Use portable model names across different providers. Model aliases allow you to write provider-agnostic code.

**Standard aliases:**
- `"smart"` - Most capable model for complex tasks
- `"fast"` - Quick responses for simple queries
- `"code"` - Optimized for code generation
- `"vision"` - Multimodal/vision capabilities (if available)
- `"default"` - Provider's recommended default

**Model alias resolution examples:**
```go
// "smart" resolves differently per provider
ai.WithProviderAlias("openai")         // → gpt-4
ai.WithProviderAlias("openai.deepseek") // → deepseek-reasoner
ai.WithProviderAlias("openai.groq")     // → mixtral-8x7b-32768

// "fast" for quick responses
ai.WithProviderAlias("openai")         // → gpt-3.5-turbo
ai.WithProviderAlias("openai.deepseek") // → deepseek-chat
ai.WithProviderAlias("openai.groq")     // → llama-3.3-70b-versatile

// Direct model names still work (pass-through)
client, _ := ai.NewClient(
    ai.WithProviderAlias("openai.deepseek"),
    ai.WithModel("deepseek-coder-v2"), // Exact model, not an alias
)
```

### NewChainClient

Create a chain client that automatically fails over between multiple AI providers. Perfect for production systems requiring high availability and resilience.

```go
func NewChainClient(opts ...ChainOption) (*ChainClient, error)
```

**Features:**
- **Automatic failover** - Seamlessly switches to backup providers
- **Graceful degradation** - Works with single provider if needed
- **Smart error handling** - Only fails over on infrastructure errors
- **Configurable chain** - Define your provider priority order

**Example - High Availability Setup:**
```go
// Create a resilient AI client with fallback providers
chain, err := ai.NewChainClient(
    ai.WithProviderChain([]ai.ChainProvider{
        {Provider: "openai", Model: "gpt-4", Priority: 1},
        {Provider: "anthropic", Model: "claude-3-opus", Priority: 2},
        {Provider: "openai.groq", Model: "mixtral-8x7b", Priority: 3},
    }),
)

// Automatically tries providers in order until one succeeds
response, err := chain.GenerateResponse(ctx, "Explain quantum computing", nil)
// Tries: OpenAI → Anthropic → Groq
```

**Example - Using Provider Aliases:**
```go
// Chain with OpenAI-compatible providers
chain, _ := ai.NewChainClient(
    ai.WithProviderChain([]ai.ChainProvider{
        {ProviderAlias: "openai.deepseek", Model: "smart", Priority: 1},
        {ProviderAlias: "openai.groq", Model: "fast", Priority: 2},
        {ProviderAlias: "openai", Model: "gpt-3.5-turbo", Priority: 3},
    }),
)
```

**Graceful degradation:**
```go
// Single provider still works (no chain, direct pass-through)
chain, _ := ai.NewChainClient(
    ai.WithProviderChain([]ai.ChainProvider{
        {Provider: "openai", Model: "gpt-4"},
    }),
)

// Empty chain auto-detects from environment
chain, _ := ai.NewChainClient()  // Uses first available provider
```

### WithProviderChain

Configure the provider chain for automatic failover.

```go
func WithProviderChain(providers []ChainProvider) ChainOption
```

**ChainProvider fields:**
- `Provider` - Provider name ("openai", "anthropic", etc.)
- `ProviderAlias` - Alternative: use provider alias ("openai.groq")
- `Model` - Model to use (can be alias like "smart" or explicit)
- `Priority` - Order in chain (lower = higher priority)
- `APIKey` - Optional: override API key for this provider

**Example - Production Failover Strategy:**
```go
providers := []ai.ChainProvider{
    // Primary: Fast and cheap
    {
        ProviderAlias: "openai",
        Model:         "gpt-3.5-turbo",
        Priority:      1,
    },
    // Backup 1: More capable but slower
    {
        Provider: "anthropic",
        Model:    "claude-3-sonnet",
        Priority: 2,
    },
    // Backup 2: Alternative fast provider
    {
        ProviderAlias: "openai.groq",
        Model:         "llama-3.3-70b-versatile",
        Priority:      3,
    },
    // Emergency: Most capable model
    {
        Provider: "openai",
        Model:    "gpt-4",
        Priority: 4,
        APIKey:   os.Getenv("OPENAI_BACKUP_KEY"), // Different key/account
    },
}

chain, _ := ai.NewChainClient(
    ai.WithProviderChain(providers),
)
```

### GenerateResponse

Generate AI responses with optional parameters for fine-tuning behavior.

```go
func (c *AIClient) GenerateResponse(ctx context.Context, prompt string, options *AIOptions) (*AIResponse, error)
func (c *ChainClient) GenerateResponse(ctx context.Context, prompt string, options *AIOptions) (*AIResponse, error)
```

**AIOptions parameters:**
- `Temperature` (0.0-1.0) - Creativity level (0=deterministic, 1=creative)
- `MaxTokens` - Maximum response length
- `TopP` - Nucleus sampling (alternative to temperature)
- `Model` - Override default model

**Example - Different Use Cases:**
```go
// Technical analysis (low temperature for accuracy)
codeReview, _ := client.GenerateResponse(ctx,
    "Review this Go code for bugs",
    &core.AIOptions{
        Temperature: 0.2,
        MaxTokens:   1500,
    },
)

// Creative writing (high temperature for variety)
story, _ := client.GenerateResponse(ctx,
    "Write a short story about AI",
    &core.AIOptions{
        Temperature: 0.9,
        MaxTokens:   2000,
    },
)

// Structured data extraction (deterministic)
data, _ := client.GenerateResponse(ctx,
    "Extract JSON data from this text",
    &core.AIOptions{
        Temperature: 0,  // Deterministic
        MaxTokens:   500,
    },
)
```

### NewAIAgent

Create an intelligent agent with both AI capabilities and service discovery powers. Perfect for building orchestrators and assistants.

```go
func NewAIAgent(name string, apiKey string) (*AIAgent, error)
```

**AIAgent capabilities:**
- Full agent powers (discovery, orchestration)
- Built-in AI for natural language processing
- Conversation memory management
- Tool use and function calling
- Autonomous decision making

**Example - Intelligent Orchestrator:**
```go
// Create an AI-powered orchestrator
agent, err := ai.NewAIAgent("ai-orchestrator", os.Getenv("OPENAI_API_KEY"))
if err != nil {
    log.Fatal(err)
}

// It can process natural language
response, _ := agent.GenerateResponse(ctx,
    "Find the weather service and get weather for NYC",
    nil,
)

// It remembers conversations
agent.ProcessWithMemory(ctx, "My name is John")
agent.ProcessWithMemory(ctx, "What's my name?") // "Your name is John"

// It can discover and use other services
tools, _ := agent.Discover(ctx, core.DiscoveryFilter{
    Capabilities: []string{"weather", "news"},
})

// Orchestrate multiple tools based on user request
result := agent.ProcessRequest(ctx,
    "Get weather and news for NYC and summarize both",
    tools,
)
```

### NewAITool

Create an AI-powered tool that exposes AI capabilities as a service. Unlike AIAgent, tools are passive and cannot discover other services.

```go
func NewAITool(name string, capability string, apiKey string) (*AITool, error)
```

**When to use AITool vs AIAgent:**

| Feature | AITool | AIAgent |
|---------|--------|---------|
| **Purpose** | Provide AI service | Orchestrate services |
| **Discovery** | Can be discovered | Can discover & be discovered |
| **Memory** | Stateless | Stateful conversations |
| **Use case** | Translation, summarization | Orchestrators, assistants |

**Example - AI Microservices:**
```go
// Create specialized AI tools
translator, _ := ai.NewAITool(
    "translator-service",
    "translate",
    apiKey,
)

summarizer, _ := ai.NewAITool(
    "summarizer-service",
    "summarize",
    apiKey,
)

sentiment, _ := ai.NewAITool(
    "sentiment-service",
    "analyze_sentiment",
    apiKey,
)

// Each runs as independent microservice
go translator.Start(ctx, 8081)
go summarizer.Start(ctx, 8082)
go sentiment.Start(ctx, 8083)

// Agents can discover and use these tools
// POST http://localhost:8081/api/capabilities/translate
// {"text": "Hello", "target_lang": "es"}
```

### AI Provider Support

GoMind supports multiple AI providers with consistent APIs.

| Provider | Models | Best For |
|----------|--------|----------|
| **OpenAI** | gpt-4, gpt-4-turbo, gpt-3.5-turbo | General purpose, code generation |
| **Anthropic** | claude-3-opus, claude-3-sonnet | Complex reasoning, analysis |
| **Google** | gemini-pro, gemini-1.5-pro | Multimodal, long context |
| **Groq** | llama3, mixtral, gemma | Fast inference, open models |

**Example - Multi-Provider Strategy:**
```go
// Use different providers for different tasks
type AIService struct {
    reasoning  core.AIClient  // Claude for complex reasoning
    creative   core.AIClient  // GPT-4 for creative tasks
    fast       core.AIClient  // Groq for quick responses
}

func NewAIService() *AIService {
    return &AIService{
        reasoning: ai.MustNewClient(
            ai.WithProvider("anthropic"),
            ai.WithModel("claude-3-opus-20240229"),
        ),
        creative: ai.MustNewClient(
            ai.WithProvider("openai"),
            ai.WithModel("gpt-4"),
        ),
        fast: ai.MustNewClient(
            ai.WithProvider("groq"),
            ai.WithModel("llama3-70b"),
        ),
    }
}

func (s *AIService) AnalyzeCode(code string) (string, error) {
    // Use Claude for code analysis
    return s.reasoning.GenerateResponse(ctx,
        fmt.Sprintf("Analyze this code:\n%s", code),
        &core.AIOptions{Temperature: 0.3},
    )
}

func (s *AIService) GenerateDocumentation(code string) (string, error) {
    // Use GPT-4 for documentation
    return s.creative.GenerateResponse(ctx,
        fmt.Sprintf("Generate comprehensive docs for:\n%s", code),
        &core.AIOptions{Temperature: 0.7},
    )
}
```

---

## Resilience Module

Build fault-tolerant systems with circuit breakers and intelligent retry mechanisms.

### Circuit Breakers

Protect your application from cascading failures by automatically stopping calls to failing services.

#### NewCircuitBreaker

Create a production-ready circuit breaker with comprehensive configuration.

```go
func NewCircuitBreaker(config *CircuitBreakerConfig) (*CircuitBreaker, error)
func NewCircuitBreakerLegacy(failureThreshold int, recoveryTimeout time.Duration) *CircuitBreaker
```

**How it works:**
- **Closed State**: Normal operation, requests pass through
- **Open State**: Service is down, requests fail immediately
- **Half-Open State**: Testing recovery with limited requests

**Configuration parameters:**
- `ErrorThreshold` - Percentage of failures to open (0.0-1.0)
- `VolumeThreshold` - Minimum requests before evaluation
- `SleepWindow` - How long to wait before testing recovery
- `HalfOpenRequests` - Test requests in half-open state
- `SuccessThreshold` - Success rate to close again

**Example - Production Configuration:**
```go
// Sophisticated circuit breaker for production
breaker, err := resilience.NewCircuitBreaker(&resilience.CircuitBreakerConfig{
    Name:             "payment-api",
    ErrorThreshold:   0.5,              // Open at 50% error rate
    VolumeThreshold:  10,                // Need 10+ requests to evaluate
    SleepWindow:      30 * time.Second, // Wait 30s before recovery test
    HalfOpenRequests: 5,                 // Test with 5 requests
    SuccessThreshold: 0.6,               // Need 60% success to recover

    // Smart error classification
    ErrorClassifier: func(err error) bool {
        // Only infrastructure errors trip the breaker
        if errors.Is(err, context.Canceled) {
            return false  // User cancelled, don't count
        }
        if httpErr, ok := err.(*HTTPError); ok {
            return httpErr.StatusCode >= 500  // Only server errors
        }
        return true  // Network/timeout errors count
    },
})
```

#### Execute Methods

Execute functions with circuit breaker protection.

```go
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error
func (cb *CircuitBreaker) ExecuteWithTimeout(ctx context.Context, timeout time.Duration, fn func() error) error
```

**What Execute does:**
1. Checks circuit state (fails fast if open)
2. Executes your function
3. Records success/failure
4. Updates circuit state
5. Handles panics gracefully

**Example - API Call Protection:**
```go
func CallPaymentAPI(order Order) error {
    return paymentBreaker.Execute(ctx, func() error {
        // This is protected by the circuit breaker
        resp, err := http.Post(paymentURL, "application/json", order)
        if err != nil {
            return err
        }

        if resp.StatusCode >= 500 {
            return fmt.Errorf("server error: %d", resp.StatusCode)
        }

        return nil
    })
}

// Handle circuit breaker states
err := CallPaymentAPI(order)
if errors.Is(err, core.ErrCircuitBreakerOpen) {
    // Service is down, use fallback
    log.Warn("Payment service unavailable, using queue")
    return queuePaymentForLater(order)
}
```

#### Monitoring and Control

Monitor circuit breaker state and metrics for operational visibility.

```go
// Get current state
state := breaker.GetState()  // "closed", "open", or "half-open"

// Get detailed metrics
metrics := breaker.GetMetrics()
// Returns: state, success, failure, error_rate, total_executions, rejected

// Manual control for maintenance
breaker.ForceOpen()    // Block all requests
breaker.ForceClosed()  // Allow all requests
breaker.ClearForce()   // Resume automatic operation

// State change notifications
breaker.AddStateChangeListener(func(name string, from, to CircuitState) {
    if to == resilience.StateOpen {
        alert.Send("Circuit breaker %s opened!", name)
    }
})
```

### Retry Mechanisms

Automatically retry failed operations with configurable backoff strategies.

#### Retry Function

Simple retry with exponential backoff.

```go
func Retry(ctx context.Context, config *RetryConfig, fn func() error) error
```

**RetryConfig parameters:**
- `MaxAttempts` - Maximum retry attempts
- `InitialDelay` - First retry delay
- `BackoffFactor` - Delay multiplier (2.0 = double each time)
- `MaxDelay` - Maximum delay between retries
- `JitterEnabled` - Add randomness to prevent thundering herd

**Example:**
```go
config := &resilience.RetryConfig{
    MaxAttempts:   5,
    InitialDelay:  100 * time.Millisecond,
    BackoffFactor: 2.0,      // 100ms, 200ms, 400ms, 800ms, 1600ms
    MaxDelay:      5 * time.Second,
    JitterEnabled: true,      // Prevent synchronized retries
}

err := resilience.Retry(ctx, config, func() error {
    return callFlakyService()
})
```

#### RetryExecutor

Production-ready retry with logging and telemetry.

```go
func NewRetryExecutor(config *RetryConfig) *RetryExecutor
func (r *RetryExecutor) Execute(ctx context.Context, operation string, fn func() error) error
```

**Why use RetryExecutor:**
- Named operations in logs
- Detailed retry logging
- Telemetry integration
- Success/failure metrics

**Example - Database Operations:**
```go
executor := resilience.NewRetryExecutor(&resilience.RetryConfig{
    MaxAttempts:   3,
    InitialDelay:  50 * time.Millisecond,
    BackoffFactor: 2.0,
    JitterEnabled: true,
})
executor.SetLogger(logger)

// Named operation appears in logs
err := executor.Execute(ctx, "fetch-user-profile", func() error {
    return db.QueryRow("SELECT * FROM users WHERE id = ?", userID).Scan(&user)
})

// Logs show:
// INFO: Starting retry operation [fetch-user-profile]
// DEBUG: Attempt 1 failed, retrying...
// INFO: Operation succeeded on attempt 2
```

#### RetryWithCircuitBreaker

Combine retry and circuit breaker for maximum resilience.

```go
func RetryWithCircuitBreaker(ctx context.Context, config *RetryConfig, cb *CircuitBreaker, fn func() error) error
```

**Best practice pattern:**
```go
type ResilientClient struct {
    breaker *resilience.CircuitBreaker
    retry   *resilience.RetryExecutor
}

func (c *ResilientClient) Call(ctx context.Context, request Request) (Response, error) {
    var response Response

    // Retry handles transient failures
    // Circuit breaker prevents cascading failures
    err := c.breaker.Execute(ctx, func() error {
        return c.retry.Execute(ctx, "api-call", func() error {
            return c.makeHTTPCall(request, &response)
        })
    })

    return response, err
}
```

---

## Telemetry Module

Comprehensive observability with metrics, distributed tracing, and context propagation.

### Basic Metrics

Simple functions for common metrics without boilerplate.

```go
func Counter(name string, labels ...string)
func Histogram(name string, value float64, labels ...string)
func Gauge(name string, value float64, labels ...string)
func Duration(name string, startTime time.Time, labels ...string)
```

**Example - Request Metrics:**
```go
// Count requests
telemetry.Counter("api.requests",
    "method", r.Method,
    "endpoint", r.URL.Path,
    "status", strconv.Itoa(status),
)

// Track latency
start := time.Now()
processRequest()
telemetry.Duration("api.latency", start,
    "endpoint", r.URL.Path,
)

// Monitor concurrent requests
telemetry.Gauge("api.concurrent_requests", float64(active))

// Track response sizes
telemetry.Histogram("api.response_bytes", float64(len(response)),
    "endpoint", r.URL.Path,
)
```

### Context-Aware Telemetry

Advanced telemetry that automatically correlates metrics with distributed traces.

```go
func EmitWithContext(ctx context.Context, name string, value float64, labels ...string)
func GetBaggage(ctx context.Context) map[string]string
func SetBaggage(ctx context.Context, key, value string) context.Context
```

**Why use context-aware telemetry:**
- Automatic trace correlation
- Request metadata propagation
- Cross-service tracking
- Debugging complex flows

**Example - Distributed Request Tracking:**
```go
// API Gateway
func (gw *Gateway) Handle(ctx context.Context, req Request) error {
    // Start trace and add metadata
    ctx, span := telemetry.StartSpan(ctx, "gateway.handle")
    defer span.End()

    ctx = telemetry.SetBaggage(ctx, "request_id", req.ID)
    ctx = telemetry.SetBaggage(ctx, "user_id", req.UserID)

    // Metrics include trace context
    telemetry.EmitWithContext(ctx, "gateway.requests", 1,
        "endpoint", req.Path,
    )

    // Call downstream service (context propagates)
    result, err := gw.authService.Authenticate(ctx, req.UserID)

    return err
}

// Auth Service (automatically gets context)
func (auth *AuthService) Authenticate(ctx context.Context, userID string) error {
    // Access propagated metadata
    baggage := telemetry.GetBaggage(ctx)
    requestID := baggage["request_id"]  // From gateway!

    // Metrics correlated to same trace
    telemetry.EmitWithContext(ctx, "auth.attempts", 1,
        "user_id", userID,
        "request_id", requestID,
    )

    return nil
}
```

### Type-Specific Helpers

Semantic helper functions for common metric types.

```go
func RecordError(name string, errorType string, labels ...string)
func RecordSuccess(name string, labels ...string)
func RecordLatency(name string, milliseconds float64, labels ...string)
func RecordBytes(name string, bytes int64, labels ...string)
```

**Example:**
```go
start := time.Now()
err := processOrder(order)

if err != nil {
    telemetry.RecordError("order.processing", err.Error(),
        "order_id", order.ID,
    )
} else {
    telemetry.RecordSuccess("order.processing",
        "order_id", order.ID,
    )
}

telemetry.RecordLatency("order.processing",
    float64(time.Since(start).Milliseconds()),
    "order_id", order.ID,
)
```

### Unified Metrics API

Cross-module metrics that enable consistent observability across agents and orchestration. These helpers emit standardized metrics with a `module` label, enabling unified Grafana dashboards regardless of which GoMind module you use.

```go
// Module constants
const (
    ModuleAgent         = "agent"
    ModuleOrchestration = "orchestration"
    ModuleCore          = "core"
)

// Request metrics
func RecordRequest(module, operation string, durationMs float64, status string)
func RecordRequestError(module, operation, errorType string)

// Tool/capability call metrics
func RecordToolCall(module, toolName string, durationMs float64, status string)
func RecordToolCallError(module, toolName, errorType string)
func RecordToolCallRetry(module, toolName string)

// AI provider metrics
func RecordAIRequest(module, provider string, durationMs float64, status string)
func RecordAITokens(module, provider, tokenType string, count int64)
```

**Why use unified metrics:**
- Single Grafana dashboard works for both agent and orchestration examples
- Consistent metric names across all GoMind modules
- Easy to compare performance between different module implementations
- Prometheus queries work regardless of which module emits the data

**Example - Agent Request Handler:**
```go
func handleResearchRequest(w http.ResponseWriter, r *http.Request) {
    startTime := time.Now()
    var requestStatus = "success"

    defer func() {
        durationMs := float64(time.Since(startTime).Milliseconds())
        // Unified metric - works in cross-module dashboards
        telemetry.RecordRequest(telemetry.ModuleAgent, "research", durationMs, requestStatus)
    }()

    // Parse request
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        requestStatus = "error"
        telemetry.RecordRequestError(telemetry.ModuleAgent, "research", "validation")
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    // Process...
}
```

**Example - Orchestration with AI:**
```go
func executeWithAI(ctx context.Context, prompt string) (string, error) {
    startTime := time.Now()

    response, err := aiClient.GenerateResponse(ctx, prompt)

    // Record AI metrics
    status := "success"
    if err != nil {
        status = "error"
    }
    telemetry.RecordAIRequest(telemetry.ModuleOrchestration, "openai",
        float64(time.Since(startTime).Milliseconds()), status)

    return response, err
}
```

**Prometheus Queries for Unified Dashboards:**
```promql
# Request rate across all modules
sum(rate(request_total[5m])) by (module, operation)

# P95 latency by module
histogram_quantile(0.95, sum(rate(request_duration_ms_bucket[5m])) by (le, module))

# Error rate comparison: agent vs orchestration
sum(rate(request_errors[5m])) by (module, error_type)

# AI request latency by provider
histogram_quantile(0.95, sum(rate(ai_request_duration_ms_bucket[5m])) by (le, provider))
```

### Distributed Tracing

HTTP instrumentation for automatic trace context propagation across service boundaries.

#### TracingMiddleware

Wrap HTTP handlers to automatically extract and propagate W3C TraceContext headers.

```go
func TracingMiddleware(serviceName string) func(http.Handler) http.Handler
func TracingMiddlewareWithConfig(serviceName string, config *TracingMiddlewareConfig) func(http.Handler) http.Handler
```

**What it does:**
- Extracts `traceparent` and `tracestate` headers from incoming requests
- Creates a span for each HTTP request
- Records HTTP metrics (status codes, latency)
- Propagates trace context to handler code via `context.Context`

**TracingMiddlewareConfig options:**
- `ExcludedPaths` - Paths to skip tracing (e.g., `/health`, `/metrics`)
- `SpanNameFormatter` - Custom function to generate span names

**Example - Basic Usage:**
```go
// Initialize telemetry FIRST
telemetry.Initialize(telemetry.Config{
    ServiceName: "my-service",
    Endpoint:    "http://otel-collector:4318",
})
defer telemetry.Shutdown(context.Background())

// Create handlers
mux := http.NewServeMux()
mux.HandleFunc("/api/users", handleUsers)

// Wrap with tracing middleware
tracedHandler := telemetry.TracingMiddleware("my-service")(mux)
http.ListenAndServe(":8080", tracedHandler)
```

**Example - With Configuration:**
```go
config := &telemetry.TracingMiddlewareConfig{
    // Don't trace health checks
    ExcludedPaths: []string{"/health", "/metrics", "/ready"},

    // Custom span names
    SpanNameFormatter: func(op string, r *http.Request) string {
        return r.Method + " " + r.URL.Path
    },
}

tracedHandler := telemetry.TracingMiddlewareWithConfig("my-service", config)(mux)
```

#### NewTracedHTTPClient

Create an HTTP client that automatically propagates trace context to downstream services.

```go
func NewTracedHTTPClient(baseTransport http.RoundTripper) *http.Client
func NewTracedHTTPClientWithTransport(transport *http.Transport) *http.Client
```

**What it does:**
- Injects `traceparent` and `tracestate` headers into outgoing requests
- Creates child spans for each HTTP call
- Enables distributed tracing across service boundaries

**Example - Basic Usage:**
```go
// Create once, reuse for all requests (connection pooling)
client := telemetry.NewTracedHTTPClient(nil)

func callDownstreamService(ctx context.Context) error {
    // Context carries trace information
    req, _ := http.NewRequestWithContext(ctx, "GET", "http://other-service/api/data", nil)

    // Trace headers automatically injected
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    // Process response...
    return nil
}
```

**Example - With Custom Transport:**
```go
// Production configuration with connection pooling
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
}

client := telemetry.NewTracedHTTPClientWithTransport(transport)
```

**Best practices:**
- Create `TracedHTTPClient` once and reuse (connection pooling)
- Always pass `context.Context` from incoming request to outgoing calls
- Initialize telemetry before creating traced clients

#### End-to-End Tracing Example

Complete example showing trace propagation across services:

```go
// === Service A (API Gateway) ===
func main() {
    telemetry.Initialize(telemetry.Config{ServiceName: "api-gateway"})
    defer telemetry.Shutdown(context.Background())

    // Create traced client for calling Service B
    serviceB := telemetry.NewTracedHTTPClient(nil)

    mux := http.NewServeMux()
    mux.HandleFunc("/api/request", func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()  // Contains trace from middleware

        // Call Service B - trace context propagates automatically
        req, _ := http.NewRequestWithContext(ctx, "GET", "http://service-b/process", nil)
        resp, _ := serviceB.Do(req)
        defer resp.Body.Close()

        // Respond...
    })

    // Wrap with tracing middleware
    traced := telemetry.TracingMiddleware("api-gateway")(mux)
    http.ListenAndServe(":8080", traced)
}

// === Service B ===
func main() {
    telemetry.Initialize(telemetry.Config{ServiceName: "service-b"})
    defer telemetry.Shutdown(context.Background())

    mux := http.NewServeMux()
    mux.HandleFunc("/process", func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()  // Trace context from Service A!

        // Metrics are correlated with the trace
        telemetry.EmitWithContext(ctx, "service-b.processed", 1)

        w.WriteHeader(http.StatusOK)
    })

    traced := telemetry.TracingMiddleware("service-b")(mux)
    http.ListenAndServe(":8081", traced)
}

// Result in Jaeger/Tempo:
// Trace abc123:
// ├── api-gateway: /api/request (100ms)
// │   └── service-b: /process (50ms)
```

---

## Orchestration Module

Intelligently coordinate multiple agents and tools to accomplish complex tasks.

### CreateSimpleOrchestrator

Zero-configuration orchestrator for getting started quickly.

```go
func CreateSimpleOrchestrator(discovery core.Discovery, aiClient core.AIClient) *AIOrchestrator
```

**Example:**
```go
// Just works - no configuration needed!
orchestrator := orchestration.CreateSimpleOrchestrator(discovery, aiClient)

// Process complex natural language requests
response, err := orchestrator.ProcessRequest(ctx,
    "Get weather for NYC and find related news articles, then summarize everything",
    nil,  // Auto-discovers needed services
)

fmt.Println(response.Response)
// Output: "Today in NYC, it's 72°F with partly cloudy skies. Recent news includes..."
```

### CreateOrchestratorWithOptions

Create an orchestrator with flexible configuration options.

```go
func CreateOrchestratorWithOptions(deps OrchestratorDependencies, opts ...OrchestratorOption) (*AIOrchestrator, error)
```

**Available options:**
- `WithCapabilityProvider(type, url)` - Configure capability discovery
- `WithTelemetry(enabled)` - Enable metrics and tracing
- `WithFallback(enabled)` - Graceful degradation
- `WithCache(enabled, ttl)` - Cache discovery results

**Example - Production Orchestrator:**
```go
// Set up dependencies
deps := orchestration.OrchestratorDependencies{
    Discovery: redisDiscovery,
    AIClient:  aiClient,
    Logger:    logger,
}

// Create with options
orchestrator, err := orchestration.CreateOrchestratorWithOptions(deps,
    orchestration.WithCapabilityProvider("service", "http://capability-service:8080"),
    orchestration.WithTelemetry(true),
    orchestration.WithFallback(true),
    orchestration.WithCache(true, 5*time.Minute),
)

// Process requests with automatic service discovery and coordination
response, err := orchestrator.ProcessRequest(ctx,
    "Analyze sales data and generate report",
    nil,
)
```

### ExecutionOptions Configuration

Configure execution behavior for the orchestrator, including retry logic and type safety features.

```go
type ExecutionOptions struct {
    MaxConcurrency           int           // Maximum parallel tool/agent calls (default: 5)
    StepTimeout              time.Duration // Timeout per step (default: 30s)
    TotalTimeout             time.Duration // Overall execution timeout (default: 2m)
    RetryAttempts            int           // Retry failed steps (default: 2)
    RetryDelay               time.Duration // Delay between retries (default: 2s)
    CircuitBreaker           bool          // Enable circuit breaker (default: true)
    FailureThreshold         int           // Circuit breaker threshold (default: 5)
    RecoveryTimeout          time.Duration // Circuit breaker recovery (default: 30s)

    // Type Safety (Layer 3 - Validation Feedback)
    ValidationFeedbackEnabled bool         // Enable LLM-based parameter correction (default: true)
    MaxValidationRetries      int          // Max correction attempts (default: 2)
}
```

**Type Safety Configuration:**

```go
// Default configuration - maximum reliability (~99% success rate)
config := orchestration.DefaultConfig()
// ValidationFeedbackEnabled: true (default)
// MaxValidationRetries: 2 (default)

// Cost-sensitive configuration (~95% success rate)
config.ExecutionOptions.ValidationFeedbackEnabled = false

// Maximum reliability configuration
config.ExecutionOptions.ValidationFeedbackEnabled = true
config.ExecutionOptions.MaxValidationRetries = 3  // More retries for edge cases
```

See [Intelligent Error Handling](./INTELLIGENT_ERROR_HANDLING.md#orchestration-module-multi-layer-type-safety) for details on how type safety layers work together.

### Orchestration Strategies

Different strategies for different scales and use cases:

| Strategy | Use Case | Scale |
|----------|----------|-------|
| **Autonomous** | AI decides which services to use | Small-medium |
| **Directed** | Explicit service specification | Any scale |
| **Hybrid** | Mix of AI and rules | Medium-large |

**Example - Scaling Orchestration:**
```go
// Small scale (< 10 services) - Let AI figure it out
config := &orchestration.OrchestratorConfig{
    RoutingMode:       orchestration.ModeAutonomous,
    SynthesisStrategy: orchestration.StrategyLLM,
}

// Large scale (100+ services) - Use capability service
config := &orchestration.OrchestratorConfig{
    RoutingMode:            orchestration.ModeHybrid,
    CapabilityProviderType: "service",
    CapabilityServiceURL:   "http://capability-registry:8080",
}

// Capability service indexes all available capabilities
// and provides fast, structured search without hitting discovery
```

---

## UI Module

Build interactive chat interfaces and web transports for your GoMind applications.

### Chat Transport

WebSocket-based chat interface for real-time interaction.

```go
func NewChatTransport(agent Agent, aiClient AIClient) *ChatTransport
```

**Features:**
- WebSocket for real-time communication
- Automatic reconnection
- Message history
- Typing indicators
- File upload support

**Example - Chat Application:**
```go
// Create chat-enabled agent
agent := core.NewBaseAgent("assistant")
aiClient := ai.MustNewClient()

transport := ui.NewChatTransport(agent, aiClient)

// Attach to HTTP server
http.Handle("/chat", transport)
http.Handle("/", http.FileServer(http.Dir("./web")))

log.Println("Chat interface available at http://localhost:8080")
http.ListenAndServe(":8080", nil)
```

### REST Transport

RESTful API transport for traditional HTTP interactions.

```go
func NewRESTTransport(agent Agent) *RESTTransport
```

**Automatic endpoints:**
- `GET /api/capabilities` - List available capabilities
- `POST /api/execute/{capability}` - Execute capability
- `GET /api/health` - Health check
- `GET /api/metrics` - Prometheus metrics

**Example:**
```go
transport := ui.NewRESTTransport(agent)

// Adds REST endpoints to your agent
agent.HandleFunc("/api/", transport.Handler())

// Now available:
// curl http://localhost:8080/api/capabilities
// curl -X POST http://localhost:8080/api/execute/translate -d '{"text":"Hello"}'
```

---

## Common Patterns

### Production Service Template

Complete template for production-ready services:

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/resilience"
)

func main() {
    ctx := context.Background()

    // Create component
    agent := core.NewBaseAgent("production-service")

    // Register capabilities
    agent.RegisterCapability(core.Capability{
        Name:        "process",
        Description: "Process data",
        Handler:     handleProcess,
    })

    // Create framework with production settings
    framework, err := core.NewFramework(agent,
        // Discovery
        core.WithRedisURL(os.Getenv("REDIS_URL")),

        // Observability
        core.WithTelemetry(true, os.Getenv("OTEL_ENDPOINT")),
        core.WithLogLevel("info"),

        // Networking
        core.WithPort(8080),
        core.WithCORSDefaults(),

        // Resilience
        core.WithCircuitBreaker(resilience.DefaultConfig()),
    )

    if err != nil {
        log.Fatal("Failed to create framework:", err)
    }

    // Run with graceful shutdown
    errChan := make(chan error, 1)
    go func() {
        errChan <- framework.Run(ctx)
    }()

    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    select {
    case <-sigChan:
        log.Info("Shutdown signal received")
    case err := <-errChan:
        log.Error("Framework error:", err)
    }

    // Graceful shutdown
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := framework.Shutdown(shutdownCtx); err != nil {
        log.Error("Shutdown error:", err)
        os.Exit(1)
    }

    log.Info("Graceful shutdown complete")
}
```

### Multi-Provider AI Strategy

Use different AI providers for different tasks:

```go
type SmartAI struct {
    fast      core.AIClient  // Groq for speed
    accurate  core.AIClient  // Claude for accuracy
    creative  core.AIClient  // GPT-4 for creativity
}

func NewSmartAI() *SmartAI {
    return &SmartAI{
        fast: ai.MustNewClient(
            ai.WithProvider("groq"),
            ai.WithModel("llama3-8b"),
        ),
        accurate: ai.MustNewClient(
            ai.WithProvider("anthropic"),
            ai.WithModel("claude-3-opus-20240229"),
        ),
        creative: ai.MustNewClient(
            ai.WithProvider("openai"),
            ai.WithModel("gpt-4"),
        ),
    }
}

func (s *SmartAI) QuickAnswer(question string) (string, error) {
    // Use fast model for simple queries
    resp, err := s.fast.GenerateResponse(ctx, question, &core.AIOptions{
        MaxTokens:   200,
        Temperature: 0.3,
    })
    return resp.Content, err
}

func (s *SmartAI) AnalyzeCode(code string) (string, error) {
    // Use accurate model for code analysis
    resp, err := s.accurate.GenerateResponse(ctx,
        fmt.Sprintf("Analyze this code for bugs and improvements:\n%s", code),
        &core.AIOptions{
            Temperature: 0.1,  // Very low for technical accuracy
            MaxTokens:   2000,
        },
    )
    return resp.Content, err
}
```

### Service Mesh Pattern

Build a resilient service mesh with discovery and circuit breakers:

```go
type ServiceMesh struct {
    discovery core.Discovery
    breakers  map[string]*resilience.CircuitBreaker
    mu        sync.RWMutex
}

func (sm *ServiceMesh) Call(ctx context.Context, capability string, request interface{}) (interface{}, error) {
    // Discover service
    services, err := sm.discovery.FindByCapability(ctx, capability)
    if err != nil || len(services) == 0 {
        return nil, fmt.Errorf("no service found for %s", capability)
    }

    service := services[0]  // TODO: Add load balancing

    // Get or create circuit breaker for this service
    breaker := sm.getBreaker(service.ID)

    var response interface{}
    err = breaker.Execute(ctx, func() error {
        // Call the service
        return sm.callService(service, request, &response)
    })

    return response, err
}

func (sm *ServiceMesh) getBreaker(serviceID string) *resilience.CircuitBreaker {
    sm.mu.RLock()
    if breaker, exists := sm.breakers[serviceID]; exists {
        sm.mu.RUnlock()
        return breaker
    }
    sm.mu.RUnlock()

    // Create new breaker
    sm.mu.Lock()
    defer sm.mu.Unlock()

    breaker := resilience.NewCircuitBreakerLegacy(5, 30*time.Second)
    sm.breakers[serviceID] = breaker
    return breaker
}
```

---

## Best Practices

### 1. Always Use the Framework

The framework handles complex setup, so you don't have to:

```go
// ❌ Don't do this
agent := core.NewBaseAgent("my-agent")
agent.Initialize(ctx)
agent.Start(ctx, 8080)

// ✅ Do this instead
framework, _ := core.NewFramework(agent, options...)
framework.Run(ctx)
```

### 2. Configure via Environment

Follow 12-factor app principles:

```go
// ✅ Good - configuration from environment
framework, _ := core.NewFramework(agent,
    core.WithRedisURL(os.Getenv("REDIS_URL")),
    core.WithPort(getPortFromEnv()),
)

// ❌ Bad - hardcoded values
framework, _ := core.NewFramework(agent,
    core.WithRedisURL("redis://localhost:6379"),
    core.WithPort(8080),
)
```

### 3. Use Structured Logging

Always log with context:

```go
// ✅ Good - structured with context
logger.Info("Processing request", map[string]interface{}{
    "request_id": req.ID,
    "user_id":    req.UserID,
    "action":     req.Action,
})

// ❌ Bad - unstructured string
log.Printf("Processing request %s for user %s", req.ID, req.UserID)
```

### 4. Handle Circuit Breaker States

Always check for circuit breaker errors:

```go
err := breaker.Execute(ctx, func() error {
    return callService()
})

if errors.Is(err, core.ErrCircuitBreakerOpen) {
    // Service is down, use fallback
    return useFallback()
}

if err != nil {
    // Other error, handle accordingly
    return err
}
```

### 5. Use Context for Cancellation

Always respect context cancellation:

```go
func LongRunningOperation(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()  // Respect cancellation
        default:
            // Do work
        }
    }
}
```

---

## Error Handling

GoMind uses typed errors for better error handling:

```go
// Circuit breaker errors
var ErrCircuitBreakerOpen = errors.New("circuit breaker is open")

// Retry errors
var ErrMaxRetriesExceeded = errors.New("max retries exceeded")

// Discovery errors
var ErrServiceNotFound = errors.New("service not found")

// Configuration errors
var ErrInvalidConfiguration = errors.New("invalid configuration")
```

**Example - Comprehensive Error Handling:**
```go
func HandleRequest(ctx context.Context, req Request) error {
    err := processWithResilience(ctx, req)

    switch {
    case errors.Is(err, core.ErrCircuitBreakerOpen):
        // Service down, use fallback
        return handleWithFallback(ctx, req)

    case errors.Is(err, core.ErrMaxRetriesExceeded):
        // Temporary failure, queue for later
        return queueForRetry(req)

    case errors.Is(err, context.DeadlineExceeded):
        // Timeout, return error to client
        return fmt.Errorf("request timeout: %w", err)

    case err != nil:
        // Unexpected error
        logger.Error("Unexpected error", map[string]interface{}{
            "error": err.Error(),
            "request_id": req.ID,
        })
        return fmt.Errorf("internal error: %w", err)
    }

    return nil
}
```

---

## Environment Variables

GoMind supports configuration through environment variables:

### Core Configuration
- `GOMIND_NAME` - Component name
- `GOMIND_PORT` - HTTP port (default: 8080)
- `REDIS_URL` - Redis connection URL
- `GOMIND_LOG_LEVEL` - Logging level (error/warn/info/debug)
- `GOMIND_LOG_FORMAT` - Log format (json/text)
- `GOMIND_DEV_MODE` - Development mode (true/false)

### Discovery Retry Configuration
- `GOMIND_DISCOVERY_RETRY` - Enable background Redis retry on connection failure (true/false, default: false)
- `GOMIND_DISCOVERY_RETRY_INTERVAL` - Initial retry interval (e.g., "30s", "1m", default: 30s)

### Schema Discovery & Validation
- `GOMIND_VALIDATE_PAYLOADS` - Enable Phase 3 schema validation (true/false, default: false)

### AI Configuration
- `OPENAI_API_KEY` - OpenAI API key
- `ANTHROPIC_API_KEY` - Anthropic API key
- `GEMINI_API_KEY` - Google Gemini API key
- `GROQ_API_KEY` - Groq API key
- `TOGETHER_API_KEY` - Together AI API key
- `DEEPSEEK_API_KEY` - DeepSeek API key
- `XAI_API_KEY` - xAI Grok API key
- `QWEN_API_KEY` - Qwen API key

### OpenAI-Compatible Provider URLs (Optional)
- `GROQ_BASE_URL` - Override Groq endpoint (default: https://api.groq.com/openai/v1)
- `TOGETHER_BASE_URL` - Override Together endpoint (default: https://api.together.xyz/v1)
- `DEEPSEEK_BASE_URL` - Override DeepSeek endpoint (default: https://api.deepseek.com/v1)
- `XAI_BASE_URL` - Override xAI endpoint (default: https://api.x.ai/v1)
- `QWEN_BASE_URL` - Override Qwen endpoint (default: https://dashscope.aliyuncs.com/compatible-mode/v1)

### Kubernetes Configuration
- `GOMIND_K8S_SERVICE_NAME` - Kubernetes service name
- `GOMIND_K8S_SERVICE_PORT` - Kubernetes service port
- `GOMIND_K8S_POD_IP` - Pod IP address
- `HOSTNAME` - Pod hostname

### Telemetry Configuration
- `OTEL_EXPORTER_OTLP_ENDPOINT` - OpenTelemetry endpoint
- `OTEL_SERVICE_NAME` - Service name for telemetry

---

## Migration Guide

### From v0.x to v1.0

The v1.0 release streamlines APIs and improves consistency:

**Component Creation:**
```go
// Old (v0.x)
agent := framework.NewAgent("my-agent")

// New (v1.0)
agent := core.NewBaseAgent("my-agent")
```

**Framework Usage:**
```go
// Old (v0.x)
framework.RunAgent(agent, 8080)

// New (v1.0)
framework, _ := core.NewFramework(agent, core.WithPort(8080))
framework.Run(ctx)
```

**AI Client:**
```go
// Old (v0.x)
client := ai.NewOpenAIClient(apiKey)

// New (v1.0) - Provider agnostic
client, _ := ai.NewClient()  // Auto-detects from environment
```

---

## Troubleshooting

### Common Issues

**"No AI provider available"**
- Set one of: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `GROQ_API_KEY`

**"Failed to connect to Redis"**
- Check `REDIS_URL` is correct
- Ensure Redis is running
- Use `WithMockDiscovery(true)` for testing without Redis

**"Circuit breaker is open"**
- Check if downstream service is healthy
- Review circuit breaker thresholds
- Check logs for failure patterns

**"Port already in use"**
- Another service is using the port
- Change port with `WithPort(different_port)`

### Debug Mode

Enable detailed logging for troubleshooting:

```bash
export GOMIND_LOG_LEVEL=debug
export GOMIND_LOG_FORMAT=text
# or
export GOMIND_DEV_MODE=true
```

---

## Support

- GitHub Issues: [github.com/itsneelabh/gomind/issues](https://github.com/itsneelabh/gomind/issues)
- Documentation: [github.com/itsneelabh/gomind/docs](https://github.com/itsneelabh/gomind/docs)
- Examples: [github.com/itsneelabh/gomind/examples](https://github.com/itsneelabh/gomind/examples)