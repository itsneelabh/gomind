# GoMind Framework Design Principles & Architecture Guidelines

**Version**: 1.0  
**Purpose**: Ensure consistency and maintainability across all framework development  
**Audience**: Core contributors, module developers, LLM-based coding agents

---

## Core Philosophy

### Mission Statement
GoMind enables **autonomous agent networks** in production environments through **compile-time architectural enforcement** and **intelligent defaults**. Unlike orchestrated frameworks, GoMind agents discover and collaborate dynamically without centralized coordination.

### Design Principles

#### 1. **Production-First Architecture**
- **Single Binary Deployment**: All components must compile to standalone executables
- **Minimal Dependencies**: No runtime dependency hell - if it's not in `go.mod`, it doesn't exist
- **Fast Startup**: <1 second initialization for any component
- **Small Footprint**: Target 10-50MB memory per component, <20MB container images
- **Built-in Reliability**: Circuit breakers, retries, and health checks are not afterthoughts

#### 2. **Compile-Time Architectural Enforcement**
- **Interface-Based Separation**: Architecture violations must be caught at compile time
- **Tool/Agent Distinction**: 
  - Tools implement `Registry` interface only (passive, register-only)
  - Agents implement `Discovery` interface (active, can find others)
  - **This is enforced by Go's type system - not convention**
- **No Circular Dependencies**: Module dependency graph must be a proper DAG

#### 3. **Intelligent Configuration Over Convention**
- **Smart Defaults**: Framework should work with minimal configuration
- **Environment-Aware**: Automatically detect and use standard environment variables
- **Auto-Configuration**: When user intent is clear, auto-configure related settings
- **Explicit Override**: Always allow explicit configuration to override defaults

#### 4. **Interface-First Design**
- **Dependency Inversion**: All modules depend on `core` interfaces, not implementations
- **Testability**: All external dependencies must be mockable through interfaces
- **Modularity**: Each module implements well-defined interfaces from `core`
- **Extensibility**: New implementations can be swapped without changing dependent code

---

## Module Architecture

### Core Module (Required Foundation)
**Responsibility**: Define all interfaces and provide base implementations

**Must Provide**:
- All framework interfaces (`Component`, `Registry`, `Discovery`, `AIClient`, `Telemetry`, etc.)
- Base implementations (`BaseTool`, `BaseAgent`)
- Configuration system with intelligent defaults
- Service discovery primitives
- Framework dependency injection

**Must NOT**:
- Import any other framework modules (dependency direction violation)
- Contain business logic beyond framework mechanics
- Make assumptions about specific implementations (Redis, OpenAI, etc.)

### Optional Modules
**Dependency Rule**: Can import `core` and at most one other framework module (`telemetry`)

**Valid Dependencies**:
- `ai` → `core`
- `resilience` → `core` + `telemetry`
- `orchestration` → `core` + `telemetry`
- `ui` → `core`
- `telemetry` → `core` (implements `core.Telemetry` interface)

**Critical Architectural Rule**: The `core` module **NEVER** imports optional modules. This ensures:
1. **Unidirectional dependency flow** - Core is the foundation
2. **True optional modules** - Telemetry remains genuinely optional
3. **Compile-time enforcement** - Architectural violations are caught by the compiler
4. **No circular dependencies** - Impossible by design

---

## Implementation Guidelines

### Configuration System Rules

#### 1. **WithXXX() Option Functions**
```go
// ✅ Good: Smart auto-configuration
func WithDiscovery(enabled bool, provider string) Option {
    // Auto-configure related settings when intent is clear
    if enabled && provider == "redis" {
        // Auto-set Redis URL from environment variables
    }
}

// ❌ Bad: Dumb property setting
func WithDiscovery(enabled bool, provider string) Option {
    // Just set properties without intelligence
}
```

#### 2. **Environment Variable Precedence**
Standard precedence order (highest to lowest):
1. Explicitly set configuration options
2. `REDIS_URL`, `OPENAI_API_KEY`, etc. (standard names)
3. `GOMIND_*` prefixed variables  
4. Sensible defaults (`localhost:6379`, etc.)

#### 3. **Fail-Safe Defaults**
- Components must work with zero configuration in development
- Production deployment should require minimal explicit configuration
- Missing optional dependencies should not break core functionality

### Component Lifecycle Rules

#### 1. **Consistent Behavior Across Components**
```go
// ✅ Both Tools and Agents must behave identically
func (t *BaseTool) Start(ctx context.Context, port int) error {
    return t.server.ListenAndServe() // Blocks until shutdown
}

func (a *BaseAgent) Start(ctx context.Context, port int) error {
    return a.server.ListenAndServe() // Blocks until shutdown  
}
```

#### 2. **Initialization Order**
1. **Configuration**: Apply all options, resolve environment variables
2. **Dependencies**: Auto-inject framework dependencies if needed
3. **Registration**: Register with discovery system if enabled
4. **Heartbeat**: Start keep-alive mechanisms for persistent services

#### 3. **Graceful Shutdown**
- All components must handle context cancellation
- Unregister from discovery systems before shutdown
- Close external connections cleanly

### Discovery System Rules

#### 1. **Automatic Registration**
```go
// ✅ Framework handles registration automatically
func (t *BaseTool) Initialize(ctx context.Context) error {
    if t.Registry != nil && config.Discovery.Enabled {
        // Auto-register and start heartbeat
    }
}

// ❌ User should not need manual registration
tool.Registry.Register(ctx, serviceInfo) // Should be automatic
```

#### 2. **TTL and Heartbeat Management**
- All registrations must have TTL (30 seconds default)
- Components must start heartbeat automatically after registration
- Heartbeat failures should trigger circuit breaker behavior

#### 3. **Capability-Based Discovery**
- Components are discovered by what they can do, not by name
- Capability definitions must be consistent across network
- Support both specific capability matches and pattern matching

### Error Handling Principles

#### 1. **Fail-Fast for Configuration Errors**
```go
// ✅ Configuration problems should fail immediately
func NewConfig(opts ...Option) (*Config, error) {
    if criticalConfigMissing {
        return nil, fmt.Errorf("configuration error: %w", err)
    }
}
```

#### 2. **Resilient Runtime Behavior**
```go
// ✅ Runtime problems should be handled gracefully
func (a *Agent) Process(ctx context.Context) error {
    if a.Telemetry != nil {
        a.Telemetry.Counter("requests.processed")
        // If telemetry fails, continue processing
    }
}
```

#### 3. **Circuit Breaker Integration**
- External API calls must be protected by circuit breakers
- Discovery calls must have circuit breaker protection
- Failed dependencies should not prevent startup (degrade gracefully)

---

## Testing Requirements

### Unit Test Coverage
- **Interfaces**: Mock all external dependencies
- **Configuration**: Test all option combinations and precedence rules
- **Error Paths**: Test failure scenarios and error propagation
- **Edge Cases**: Empty configurations, missing dependencies, network failures

### Integration Test Patterns
```go
// ✅ Good: Test actual framework behavior
func TestFrameworkDependencyInjection(t *testing.T) {
    framework, err := core.NewFramework(agent,
        core.WithDiscovery(true, "redis"), // Should auto-configure
    )
    assert.NoError(t, err)
    // Verify auto-configuration worked
}

// ❌ Bad: Test implementation details
func TestConfigurationInternals(t *testing.T) {
    // Testing internal configuration fields
}
```

### Regression Prevention
- All fixed bugs must have regression tests
- Breaking changes must be caught by compilation failures
- Performance regressions must be caught by benchmarks

---

## Code Quality Standards

### Interface Design
```go
// ✅ Good: Minimal, focused interfaces
type Registry interface {
    Register(ctx context.Context, info *ServiceInfo) error
}

// ❌ Bad: Bloated interfaces
type Registry interface {
    Register(ctx context.Context, info *ServiceInfo) error
    GetMetrics() map[string]int // Mixing concerns
    Configure(config Config) error // Configuration is separate
}
```

### Error Messages
```go
// ✅ Good: Actionable error messages
return fmt.Errorf("failed to connect to Redis at %s: %w (check REDIS_URL environment variable)", url, err)

// ❌ Bad: Vague error messages  
return fmt.Errorf("connection failed: %w", err)
```

### Documentation
- All public interfaces must have clear godoc comments
- Complex configuration options must have usage examples
- Breaking changes must be documented in CHANGELOG.md

---

## Backwards Compatibility

### API Stability
- Public interfaces are stable once released
- Configuration options are stable once released
- Breaking changes require major version bump

### Deprecation Process
1. **Mark as deprecated** with clear migration path
2. **Keep working** for at least one minor version
3. **Remove** in next major version

### Migration Guidelines  
- Provide automated migration tools when possible
- Document migration steps clearly
- Support both old and new patterns during transition

---

## Performance Requirements

### Resource Usage
- **Memory**: Components must not leak memory over time
- **Goroutines**: Must clean up goroutines on shutdown
- **Network**: Must respect connection pooling and limits

### Scalability Targets
- **1000+ concurrent agents** on single machine (through goroutines)
- **Sub-second response times** for discovery operations
- **Minimal CPU overhead** from framework internals

---

## Security Considerations

### Secrets Management
- Never log secrets or API keys
- Support secret rotation without restart
- Use secure defaults (TLS, authentication)

### Network Security
- Default to secure communication protocols
- Support authentication for discovery systems
- Validate all external inputs

---

## Monitoring and Observability

### Telemetry Architecture Pattern

**Design Decision**: Telemetry uses **explicit initialization** at the application level, not framework-level auto-wiring.

**Why This Pattern?**

```go
// ❌ Framework CANNOT do this (violates architectural principles)
import "github.com/itsneelabh/gomind/telemetry"  // Core cannot import modules

// ✅ Applications MUST do this (explicit initialization)
func main() {
    telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileProduction))
    // Now all components can use telemetry
}
```

**Rationale**:
1. **Architectural Purity**: Core module cannot import telemetry module (dependency direction)
2. **True Optionality**: Telemetry remains genuinely optional at compile time
3. **Explicit Control**: Applications have full control over telemetry lifecycle
4. **No Magic**: Clear, predictable initialization order

**Integration Pattern**:

```go
// Step 1: Core defines interface (no implementation knowledge)
type Telemetry interface {
    RecordMetric(name string, value float64, labels map[string]string)
}

// Step 2: Components have Telemetry field (defaults to NoOp)
type BaseTool struct {
    Telemetry Telemetry  // Safe default: &NoOpTelemetry{}
}

// Step 3: Telemetry module provides global singleton
var globalRegistry atomic.Value  // Stores *OTelProvider

// Step 4: Application initializes telemetry
telemetry.Initialize(config)  // Sets up global registry

// Step 5: Application code emits metrics
telemetry.Counter("requests.total")  // Uses global registry
```

**Key Characteristics**:
- **Global Singleton**: `telemetry.Initialize()` sets up a global registry
- **Thread-Safe**: Atomic operations for concurrent metric emission
- **Zero-Cost if Unused**: NoOp implementation when not initialized
- **Standard Environment Variables**: Respects `OTEL_EXPORTER_OTLP_ENDPOINT`

### Built-in Telemetry Requirements

**For Framework Code**:
- Never assume telemetry is initialized
- Always check for nil before using Telemetry interface
- Use NoOp default in constructors
- Never fail operations due to telemetry failures

```go
// ✅ Good: Safe telemetry usage
func (t *BaseTool) processRequest() {
    if t.Telemetry != nil {
        t.Telemetry.RecordMetric("requests.total", 1.0, nil)
    }
    // Continue processing even if telemetry fails
}
```

**For Application Code**:
- Initialize telemetry in `main()` before creating components
- Use `defer telemetry.Shutdown()` for clean shutdown
- Configure via environment variables or explicit config
- Support both development and production profiles

### Health Checks
- All components must provide `/health` endpoints
- Health checks must be fast (<100ms) and reliable
- Support both liveness and readiness probes
- Health status should include telemetry initialization state (optional)

---

## Framework Evolution Guidelines

### Adding New Features
1. **Design interfaces first** in `core` module
2. **Implement in separate module** (avoid core bloat)
3. **Provide intelligent defaults** in configuration system
4. **Add comprehensive tests** including integration scenarios
5. **Update documentation** with examples

### Modifying Existing Features  
1. **Maintain backwards compatibility** in public APIs
2. **Add deprecation warnings** for old patterns
3. **Provide migration path** for breaking changes
4. **Update all related tests** and documentation

### Code Review Checklist
- [ ] Follows interface-first design
- [ ] Maintains Tool/Agent architectural separation
- [ ] Includes intelligent configuration defaults
- [ ] Has comprehensive test coverage
- [ ] Provides clear error messages
- [ ] Updates relevant documentation
- [ ] No backwards compatibility breaks without major version
- [ ] Core module doesn't import optional modules (telemetry, ai, etc.)
- [ ] Telemetry usage is nil-safe (checks before use)
- [ ] Application examples show proper telemetry initialization

---

**Remember**: These principles exist to maintain GoMind's core promise of **autonomous agent networks in production**. When in doubt, favor production reliability over development convenience, and architectural clarity over implementation simplicity.