# Component-Aware Logging Design

## Problem Statement

The `ProductionLogger` currently hardcodes `"component": "framework"` for ALL logs (line 1632 in config.go), making it impossible to distinguish:

- Framework internal logs (core, ai, orchestration, resilience, telemetry)
- User-developed agent logs (e.g., TravelResearchAgent, ResearchAgent)
- User-developed tool logs (e.g., WeatherTool, GeocodingTool)

This creates difficulty when debugging production systems where you need to filter logs by component type.

## Current Code Analysis

### ProductionLogger Struct (config.go:1532-1539)

```go
type ProductionLogger struct {
    level          LogLevel  // Numeric level for efficient comparison
    serviceName    string
    format         string
    output         io.Writer
    metricsEnabled bool      // Metrics layer (enabled when telemetry available)
}
```

**Missing:** A `component` field to identify the log source.

### Logger Interface (interfaces.go:11-23)

```go
type Logger interface {
    // Basic logging methods
    Info(msg string, fields map[string]interface{})
    Error(msg string, fields map[string]interface{})
    Warn(msg string, fields map[string]interface{})
    Debug(msg string, fields map[string]interface{})

    // Context-aware methods for distributed tracing
    InfoWithContext(ctx context.Context, msg string, fields map[string]interface{})
    ErrorWithContext(ctx context.Context, msg string, fields map[string]interface{})
    WarnWithContext(ctx context.Context, msg string, fields map[string]interface{})
    DebugWithContext(ctx context.Context, msg string, fields map[string]interface{})
}
```

### applyConfigToComponent (agent.go:944-1030)

This function handles both BaseAgent and BaseTool:

```go
func applyConfigToComponent(component HTTPComponent, config *Config) {
    switch base := component.(type) {
    case *BaseAgent:
        base.Logger = config.logger  // Sets logger for agents
        return
    case *BaseTool:
        base.Logger = config.logger  // Sets logger for tools
        return
    }
    // Reflection fallback for embedded types...
}
```

### Example Patterns in Codebase

**Tools** (e.g., examples/tool-example/weather_tool.go):
```go
type WeatherTool struct {
    *core.BaseTool  // Embedded BaseTool
    apiKey string
}
```

**Agents** (e.g., examples/agent-with-orchestration/research_agent.go):
```go
type TravelResearchAgent struct {
    *core.BaseAgent  // Embedded BaseAgent
    orchestrator  *orchestration.AIOrchestrator
    workflows     map[string]*TravelWorkflow
    // ...
}
```

## Logging vs Telemetry Architecture

**IMPORTANT:** This design targets the **Logging layer** (`ProductionLogger`), which is **separate from** the **Telemetry layer**.

### Dual-Layer Observability Pattern

All examples in the codebase use two distinct observability mechanisms:

| Layer | Purpose | Source | Used In |
|-------|---------|--------|---------|
| **Logging** | Structured event logs | `r.Logger.Info()`, `t.Logger.InfoWithContext()` | ALL examples |
| **Telemetry** | Metrics, spans, traces | `telemetry.Counter()`, `telemetry.AddSpanEvent()` | Examples with telemetry |

### Example Usage Patterns

**agent-example (no telemetry module):**
```go
// Only uses Logger - comes from BaseAgent.Logger (ProductionLogger)
r.Logger.Info("Starting research topic orchestration", map[string]interface{}{
    "method": req.Method,
    "path":   req.URL.Path,
})
```

**agent-with-orchestration (with telemetry module):**
```go
// Uses BOTH Logger AND Telemetry (separate concerns)
telemetry.AddSpanEvent(ctx, "request_received",  // Telemetry layer
    attribute.String("method", r.Method),
)
t.Logger.InfoWithContext(ctx, "Processing natural language request", map[string]interface{}{  // Logging layer
    "method": r.Method,
    "path":   r.URL.Path,
})
```

### Key Insight

The `r.Logger` / `t.Logger` used in handlers **comes from the same source** regardless of whether telemetry is enabled:

```
Framework → Config.logger (ProductionLogger) → applyConfigToComponent() → BaseAgent.Logger / BaseTool.Logger
```

**This means the component-aware logging design applies universally to ALL examples**, whether they use the telemetry module or not.

## Proposed Solution: Hierarchical Component Naming

### Component Naming Scheme

```
framework/core          - Core framework (discovery, registry, config)
framework/orchestration - Orchestration module
framework/ai            - AI module
framework/resilience    - Resilience patterns
framework/telemetry     - Telemetry integration
agent/<name>            - User agents (e.g., agent/travel-research-orchestration)
tool/<name>             - User tools (e.g., tool/weather-service)
```

## Implementation Changes

### Phase 1: Add Component Field to ProductionLogger

**File: core/config.go**

```go
// ProductionLogger provides layered observability for framework operations
type ProductionLogger struct {
    level          LogLevel
    serviceName    string
    component      string    // NEW: Component identifier
    format         string
    output         io.Writer
    metricsEnabled bool
}

// NewProductionLogger creates a logger from LoggingConfig
func NewProductionLogger(logging LoggingConfig, dev DevelopmentConfig, serviceName string) Logger {
    // ... existing code ...
    return &ProductionLogger{
        level:          level,
        serviceName:    serviceName,
        component:      "framework/core",  // Default component
        format:         logging.Format,
        output:         output,
        metricsEnabled: false,
    }
}
```

### Phase 2: Add WithComponent Method

**File: core/config.go**

```go
// ComponentAwareLogger extends Logger with component context support
type ComponentAwareLogger interface {
    Logger
    WithComponent(component string) Logger
}

// WithComponent creates a child logger with specific component context.
// This allows different parts of the application to have their own
// component identifier while sharing the same base configuration.
func (p *ProductionLogger) WithComponent(component string) Logger {
    return &ProductionLogger{
        level:          p.level,
        serviceName:    p.serviceName,
        component:      component,
        format:         p.format,
        output:         p.output,
        metricsEnabled: p.metricsEnabled,
    }
}
```

### Phase 3: Update logEvent to Use Component Field

**File: core/config.go**

```go
func (p *ProductionLogger) logEvent(level, msg string, fields map[string]interface{}, ctx context.Context) {
    timestamp := time.Now().Format(time.RFC3339)

    if p.format == "json" {
        logEntry := map[string]interface{}{
            "timestamp": timestamp,
            "level":     level,
            "service":   p.serviceName,
            "component": p.component,  // USE FIELD instead of hardcoded "framework"
            "message":   msg,
        }
        // ... rest unchanged
    }
    // ...
}
```

### Phase 4: Update applyConfigToComponent

**File: core/agent.go**

```go
func applyConfigToComponent(component HTTPComponent, config *Config) {
    // Determine component type for logging
    componentName := "framework/core"  // default

    switch base := component.(type) {
    case *BaseAgent:
        componentName = "agent/" + base.ID
        base.Config = config
        base.Name = config.Name
        // ... existing ID logic ...

        // Create component-specific logger
        if cal, ok := config.logger.(ComponentAwareLogger); ok {
            base.Logger = cal.WithComponent(componentName)
        } else {
            base.Logger = config.logger
        }
        return

    case *BaseTool:
        componentName = "tool/" + base.ID
        base.Config = config
        base.Name = config.Name
        // ... existing ID logic ...

        // Create component-specific logger
        if cal, ok := config.logger.(ComponentAwareLogger); ok {
            base.Logger = cal.WithComponent(componentName)
        } else {
            base.Logger = config.logger
        }
        return
    }

    // Reflection fallback for embedded types
    v := reflect.ValueOf(component)
    if v.Kind() != reflect.Ptr {
        return
    }
    v = v.Elem()
    if v.Kind() != reflect.Struct {
        return
    }

    for i := 0; i < v.NumField(); i++ {
        field := v.Field(i)
        fieldType := field.Type()

        if fieldType == reflect.TypeOf((*BaseAgent)(nil)) && field.CanInterface() {
            if base, ok := field.Interface().(*BaseAgent); ok && base != nil {
                componentName = "agent/" + base.ID
                base.Config = config
                base.Name = config.Name
                // ... existing ID logic ...

                if cal, ok := config.logger.(ComponentAwareLogger); ok {
                    base.Logger = cal.WithComponent(componentName)
                } else {
                    base.Logger = config.logger
                }
                return
            }
        }

        if fieldType == reflect.TypeOf((*BaseTool)(nil)) && field.CanInterface() {
            if base, ok := field.Interface().(*BaseTool); ok && base != nil {
                componentName = "tool/" + base.ID
                base.Config = config
                base.Name = config.Name
                // ... existing ID logic ...

                if cal, ok := config.logger.(ComponentAwareLogger); ok {
                    base.Logger = cal.WithComponent(componentName)
                } else {
                    base.Logger = config.logger
                }
                return
            }
        }
    }
}
```

### Phase 5: Framework Sub-Module Integration

Each framework module can set its own component when creating internal loggers:

**File: orchestration/auto_wire.go**

```go
func NewAIOrchestrator(discovery core.Discovery, opts ...OrchestratorOption) (*AIOrchestrator, error) {
    o := &AIOrchestrator{
        // ...
    }

    // Apply options
    for _, opt := range opts {
        opt(o)
    }

    // Set orchestration-specific component
    if cal, ok := o.logger.(core.ComponentAwareLogger); ok {
        o.logger = cal.WithComponent("framework/orchestration")
    }

    return o, nil
}
```

**File: ai/client.go**

```go
func NewClient(opts ...ClientOption) (*Client, error) {
    c := &Client{
        // ...
    }

    // Set AI-specific component
    if cal, ok := c.logger.(core.ComponentAwareLogger); ok {
        c.logger = cal.WithComponent("framework/ai")
    }

    return c, nil
}
```

## Example Log Output After Implementation

### Framework Core Log
```json
{
  "timestamp": "2025-01-15T10:30:00Z",
  "level": "INFO",
  "service": "travel-research-orchestration",
  "component": "framework/core",
  "message": "Discovery initialized",
  "tool_count": 5
}
```

### Orchestration Module Log
```json
{
  "timestamp": "2025-01-15T10:30:01Z",
  "level": "INFO",
  "service": "travel-research-orchestration",
  "component": "framework/orchestration",
  "message": "AI orchestrator planning workflow",
  "step_count": 3,
  "trace.trace_id": "abc123"
}
```

### Agent Handler Log (TravelResearchAgent)
```json
{
  "timestamp": "2025-01-15T10:30:02Z",
  "level": "INFO",
  "service": "travel-research-orchestration",
  "component": "agent/travel-research-orchestration",
  "message": "Processing natural language orchestration request",
  "method": "POST",
  "path": "/orchestrate/natural",
  "trace.trace_id": "abc123"
}
```

### Tool Log (WeatherTool)
```json
{
  "timestamp": "2025-01-15T10:30:03Z",
  "level": "INFO",
  "service": "weather-tool-v2",
  "component": "tool/weather-service",
  "message": "Weather data fetched",
  "location": "Tokyo",
  "duration_ms": 150
}
```

## Log Filtering Examples

### Using kubectl and jq

```bash
# All agent logs
kubectl logs -l app=travel-research-agent | jq 'select(.component | startswith("agent/"))'

# All framework orchestration logs
kubectl logs -l app=travel-research-agent | jq 'select(.component == "framework/orchestration")'

# All tool logs cluster-wide
kubectl logs -l gomind-type=tool | jq 'select(.component | startswith("tool/"))'

# All framework logs (any sub-module)
kubectl logs -l app=travel-research-agent | jq 'select(.component | startswith("framework/"))'

# Specific framework module
kubectl logs -l app=travel-research-agent | jq 'select(.component == "framework/resilience")'
```

### Using Grafana Loki

```logql
# Agent handler logs only
{namespace="gomind-examples"} | json | component =~ "agent/.*"

# Framework orchestration with errors
{namespace="gomind-examples"} | json | component="framework/orchestration" | level="ERROR"

# All tool logs with slow responses
{namespace="gomind-examples"} | json | component =~ "tool/.*" | duration_ms > 1000

# Trace a request across components
{namespace="gomind-examples"} | json | trace_id="abc123"
```

## Backward Compatibility

This change is backward compatible:

1. Default component remains `"framework/core"` if not explicitly set
2. Existing `Logger` interface unchanged - `ComponentAwareLogger` extends it
3. Type assertion used to check for component support
4. Existing log queries filtering on `component="framework"` can be updated to `component =~ "framework/.*"`

## Migration Path

| Phase | Changes | Impact |
|-------|---------|--------|
| 1 | Add `component` field to `ProductionLogger` | Internal only |
| 2 | Add `WithComponent()` method and `ComponentAwareLogger` interface | No breaking changes |
| 3 | Update `logEvent()` to use `p.component` | Logs change from `"framework"` to `"framework/core"` |
| 4 | Update `applyConfigToComponent()` | Agent/tool logs get proper component names |
| 5 | Update framework modules (orchestration, ai, resilience) | Framework sub-modules get proper component names |

## Benefits

1. **Granular Filtering**: Filter logs by exact component or component type prefix
2. **Debugging**: Quickly identify which module generated a log entry
3. **Monitoring**: Create component-specific dashboards and alerts in Grafana
4. **Performance Analysis**: Track latency and errors per component
5. **Audit**: Trace request flow across components using trace_id + component
6. **Production Troubleshooting**: Distinguish user code logs from framework logs

## Files to Modify

| File | Changes |
|------|---------|
| `core/config.go` | Add `component` field, `WithComponent()` method |
| `core/interfaces.go` | Add `ComponentAwareLogger` interface |
| `core/agent.go` | Update `applyConfigToComponent()` |
| `orchestration/auto_wire.go` | Set `framework/orchestration` component |
| `ai/client.go` | Set `framework/ai` component |
| `resilience/*.go` | Set `framework/resilience` component |

---

## Completed Implementation Changes

This section documents the actual code changes implemented in each framework module to support component-aware logging.

### Standard SetLogger Pattern

Every framework module's `SetLogger` method follows this pattern:

```go
// SetLogger sets the logger for the component.
// The component is always set to "framework/<module>" to ensure proper log attribution
// regardless of which agent or tool is using this module.
func (x *Type) SetLogger(logger core.Logger) {
    if logger == nil {
        x.logger = &core.NoOpLogger{}  // or nil for lightweight components
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            x.logger = cal.WithComponent("framework/<module>")
        } else {
            x.logger = logger
        }
    }
    // Propagate original logger to sub-components (they apply their own WithComponent)
    if x.subComponent != nil {
        x.subComponent.SetLogger(logger)
    }
}
```

### core Module (Infrastructure - 4 files)

The core module provides the foundational infrastructure that enables component-aware logging across all other modules.

#### core/interfaces.go - `ComponentAwareLogger` Interface

```go
// ComponentAwareLogger extends Logger with component context support.
// Loggers implementing this interface can create child loggers with
// specific component identifiers, enabling log segregation by source.
type ComponentAwareLogger interface {
    Logger
    // WithComponent returns a new logger with the specified component.
    // The component string identifies the log source (e.g., "framework/orchestration",
    // "agent/my-agent", "tool/weather-tool").
    WithComponent(component string) Logger
}
```

#### core/config.go - `ProductionLogger.WithComponent`

```go
// ProductionLogger provides layered observability for framework operations
type ProductionLogger struct {
    level          LogLevel
    serviceName    string
    component      string    // Component identifier (e.g., "framework/core", "agent/my-agent")
    format         string
    output         io.Writer
    metricsEnabled bool
}

// WithComponent creates a child logger with specific component context.
// This allows different parts of the application to have their own
// component identifier while sharing the same base configuration.
func (p *ProductionLogger) WithComponent(component string) Logger {
    return &ProductionLogger{
        level:          p.level,
        serviceName:    p.serviceName,
        component:      component,
        format:         p.format,
        output:         p.output,
        metricsEnabled: p.metricsEnabled,
    }
}
```

#### core/agent.go - `createComponentLogger` Helper

```go
// createComponentLogger creates a logger with the appropriate component name.
// If the logger implements ComponentAwareLogger, it creates a child logger
// with the specified component. Otherwise, it returns the original logger.
func createComponentLogger(logger Logger, componentName string) Logger {
    if cal, ok := logger.(ComponentAwareLogger); ok {
        return cal.WithComponent(componentName)
    }
    return logger
}
```

This helper is used in `applyConfigToComponent()` to assign component-specific loggers to agents and tools:

```go
func applyConfigToComponent(component HTTPComponent, config *Config) {
    switch base := component.(type) {
    case *BaseAgent:
        componentName := "agent/" + base.ID
        base.Logger = createComponentLogger(config.logger, componentName)
        // ...
    case *BaseTool:
        componentName := "tool/" + base.ID
        base.Logger = createComponentLogger(config.logger, componentName)
        // ...
    }
}
```

#### core/component.go - Component Type Tracking

```go
// ComponentType represents the type of component for telemetry inference
type ComponentType int

const (
    ComponentTypeUnknown ComponentType = iota
    ComponentTypeAgent
    ComponentTypeTool
)

// SetCurrentComponentType sets the current component type (used during initialization)
func SetCurrentComponentType(t ComponentType) {
    currentComponentType = t
}

// GetCurrentComponentType returns the current component type
func GetCurrentComponentType() ComponentType {
    return currentComponentType
}
```

### ai Module (1 file)

#### ai/client.go - Component-Aware Logging in AI Client

```go
// NewClient creates a new AI client with the provided options.
// If a logger is provided and implements ComponentAwareLogger,
// it is wrapped with the "framework/ai" component.
func NewClient(opts ...ClientOption) (*Client, error) {
    // ... client initialization ...

    // Apply component-specific logging for AI module
    if config.Logger != nil {
        if cal, ok := config.Logger.(core.ComponentAwareLogger); ok {
            config.Logger = cal.WithComponent("framework/ai")
        }
        // ... propagate to sub-components ...
    }

    return client, nil
}
```

### orchestration Module (8 files)

#### orchestration/orchestrator.go - `AIOrchestrator.SetLogger`

```go
// SetLogger sets the logger provider (follows framework design principles)
// The component is always set to "framework/orchestration" to ensure proper log attribution
// regardless of which agent or tool is using the orchestration module.
func (o *AIOrchestrator) SetLogger(logger core.Logger) {
    if logger == nil {
        o.logger = &core.NoOpLogger{}
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            o.logger = cal.WithComponent("framework/orchestration")
        } else {
            o.logger = logger
        }
    }

    // Propagate logger to sub-components (they will apply their own WithComponent)
    if o.executor != nil {
        o.executor.SetLogger(logger)
    }
    if o.catalog != nil {
        o.catalog.SetLogger(logger)
    }
}
```

#### orchestration/executor.go - `SmartExecutor.SetLogger`

```go
// SetLogger sets the logger provider (follows framework design principles)
// The component is always set to "framework/orchestration" to ensure proper log attribution
// regardless of which agent or tool is using the orchestration module.
func (e *SmartExecutor) SetLogger(logger core.Logger) {
    if logger == nil {
        e.logger = &core.NoOpLogger{}
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            e.logger = cal.WithComponent("framework/orchestration")
        } else {
            e.logger = logger
        }
    }
    // Propagate logger to hybrid resolver if configured (it will apply its own WithComponent)
    if e.hybridResolver != nil {
        e.hybridResolver.SetLogger(logger)
    }
}
```

#### orchestration/catalog.go - `AgentCatalog.SetLogger`

```go
// SetLogger sets the logger provider (follows framework design principles)
// The component is always set to "framework/orchestration" to ensure proper log attribution
// regardless of which agent or tool is using the orchestration module.
func (c *AgentCatalog) SetLogger(logger core.Logger) {
    if logger == nil {
        c.logger = &core.NoOpLogger{}
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            c.logger = cal.WithComponent("framework/orchestration")
        } else {
            c.logger = logger
        }
    }
}
```

#### orchestration/hybrid_resolver.go - `HybridResolver.SetLogger`

```go
// SetLogger sets the logger for the hybrid resolver and its sub-components
// The component is always set to "framework/orchestration" to ensure proper log attribution
// regardless of which agent or tool is using the orchestration module.
func (h *HybridResolver) SetLogger(logger core.Logger) {
    if logger == nil {
        h.logger = nil
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            h.logger = cal.WithComponent("framework/orchestration")
        } else {
            h.logger = logger
        }
    }
    // Propagate to sub-components (they will apply their own WithComponent)
    if h.autoWirer != nil {
        h.autoWirer.SetLogger(logger)
    }
    if h.microResolver != nil {
        h.microResolver.SetLogger(logger)
    }
}
```

#### orchestration/micro_resolver.go - `MicroResolver.SetLogger`

```go
// SetLogger sets the logger for the micro-resolver
// The component is always set to "framework/orchestration" to ensure proper log attribution
// regardless of which agent or tool is using the orchestration module.
func (m *MicroResolver) SetLogger(logger core.Logger) {
    if logger == nil {
        m.logger = nil
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            m.logger = cal.WithComponent("framework/orchestration")
        } else {
            m.logger = logger
        }
    }
}
```

#### orchestration/auto_wire.go - `AutoWirer.SetLogger`

```go
// SetLogger sets the logger for the auto-wirer
// The component is always set to "framework/orchestration" to ensure proper log attribution
// regardless of which agent or tool is using the orchestration module.
func (w *AutoWirer) SetLogger(logger core.Logger) {
    if logger == nil {
        w.logger = nil
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            w.logger = cal.WithComponent("framework/orchestration")
        } else {
            w.logger = logger
        }
    }
}
```

#### orchestration/default_prompt_builder.go - `DefaultPromptBuilder.SetLogger`

```go
// SetLogger sets the logger for debug output (follows framework design principles)
// The component is always set to "framework/orchestration" to ensure proper log attribution
// regardless of which agent or tool is using the orchestration module.
func (d *DefaultPromptBuilder) SetLogger(logger core.Logger) {
    if logger == nil {
        d.logger = nil
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            d.logger = cal.WithComponent("framework/orchestration")
        } else {
            d.logger = logger
        }
    }
}
```

#### orchestration/template_prompt_builder.go - `TemplatePromptBuilder.SetLogger`

```go
// SetLogger sets the logger for debug output (follows framework design principles)
// The component is always set to "framework/orchestration" to ensure proper log attribution
// regardless of which agent or tool is using the orchestration module.
func (t *TemplatePromptBuilder) SetLogger(logger core.Logger) {
    if logger == nil {
        t.logger = nil
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            t.logger = cal.WithComponent("framework/orchestration")
        } else {
            t.logger = logger
        }
    }
    // Propagate to fallback (it will apply its own WithComponent)
    if t.fallback != nil {
        t.fallback.SetLogger(logger)
    }
}
```

### resilience Module (2 files)

#### resilience/retry.go - `RetryHandler.SetLogger`

```go
// SetLogger sets the logger for the retry handler
// The component is always set to "framework/resilience" to ensure proper log attribution
// regardless of which agent or tool is using the resilience module.
func (r *RetryHandler) SetLogger(logger core.Logger) {
    if logger == nil {
        r.logger = &core.NoOpLogger{}
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            r.logger = cal.WithComponent("framework/resilience")
        } else {
            r.logger = logger
        }
    }
}
```

#### resilience/circuit_breaker.go - `CircuitBreaker.SetLogger`

```go
// SetLogger sets the logger for the circuit breaker
// The component is always set to "framework/resilience" to ensure proper log attribution
// regardless of which agent or tool is using the resilience module.
func (cb *CircuitBreaker) SetLogger(logger core.Logger) {
    if logger == nil {
        cb.logger = &core.NoOpLogger{}
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            cb.logger = cal.WithComponent("framework/resilience")
        } else {
            cb.logger = logger
        }
    }
}
```

### ui Module (3 files)

#### ui/session_redis.go - `RedisSessionStore.SetLogger`

```go
// SetLogger sets the logger for the Redis session store
// The component is always set to "framework/ui" to ensure proper log attribution
// regardless of which agent or tool is using the UI module.
func (s *RedisSessionStore) SetLogger(logger core.Logger) {
    if logger == nil {
        s.logger = &core.NoOpLogger{}
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            s.logger = cal.WithComponent("framework/ui")
        } else {
            s.logger = logger
        }
    }
}
```

#### ui/registry.go - `Registry.SetLogger`

```go
// SetLogger sets the logger for the registry
// The component is always set to "framework/ui" to ensure proper log attribution
// regardless of which agent or tool is using the UI module.
func (r *Registry) SetLogger(logger core.Logger) {
    if logger == nil {
        r.logger = &core.NoOpLogger{}
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            r.logger = cal.WithComponent("framework/ui")
        } else {
            r.logger = logger
        }
    }
}
```

#### ui/circuit_breaker.go - `CircuitBreaker.SetLogger`

```go
// SetLogger sets the logger for the circuit breaker
// The component is always set to "framework/ui" to ensure proper log attribution
// regardless of which agent or tool is using the UI module.
func (cb *CircuitBreaker) SetLogger(logger core.Logger) {
    if logger == nil {
        cb.logger = &core.NoOpLogger{}
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            cb.logger = cal.WithComponent("framework/ui")
        } else {
            cb.logger = logger
        }
    }
}
```

### Summary Table

| Module | File | Type | Component Name |
|--------|------|------|----------------|
| **core** | interfaces.go | `ComponentAwareLogger` | *(interface definition)* |
| **core** | config.go | `ProductionLogger` | `framework/core` (default) |
| **core** | agent.go | `createComponentLogger` | `agent/<id>`, `tool/<id>` |
| **core** | component.go | `ComponentType` | *(type tracking)* |
| **ai** | client.go | `Client` | `framework/ai` |
| orchestration | orchestrator.go | `AIOrchestrator` | `framework/orchestration` |
| orchestration | executor.go | `SmartExecutor` | `framework/orchestration` |
| orchestration | catalog.go | `AgentCatalog` | `framework/orchestration` |
| orchestration | hybrid_resolver.go | `HybridResolver` | `framework/orchestration` |
| orchestration | micro_resolver.go | `MicroResolver` | `framework/orchestration` |
| orchestration | auto_wire.go | `AutoWirer` | `framework/orchestration` |
| orchestration | default_prompt_builder.go | `DefaultPromptBuilder` | `framework/orchestration` |
| orchestration | template_prompt_builder.go | `TemplatePromptBuilder` | `framework/orchestration` |
| resilience | retry.go | `RetryHandler` | `framework/resilience` |
| resilience | circuit_breaker.go | `CircuitBreaker` | `framework/resilience` |
| ui | session_redis.go | `RedisSessionStore` | `framework/ui` |
| ui | registry.go | `Registry` | `framework/ui` |
| ui | circuit_breaker.go | `CircuitBreaker` | `framework/ui` |

### Key Design Decisions

1. **nil logger handling**: Some components use `&core.NoOpLogger{}` (when they have a non-nil logger expectation), others use `nil` (for lightweight components that check `if logger != nil` before logging).

2. **Sub-component propagation**: Parent components pass the **original** logger (not the wrapped one) to sub-components, allowing each to apply its own `WithComponent`.

3. **Component granularity**: All files within a module use the same component name (e.g., all orchestration files use `framework/orchestration`).

4. **Documentation**: Each `SetLogger` method includes a comment explaining the component-aware pattern.

---

### Phase 6: Update Examples to Use Component-Aware Logging

All examples currently use a mix of `log.Printf` (standard Go log) and `r.Logger.Info()` / `t.Logger.Info()` (component-aware ProductionLogger). To see component values in logs, examples must use the component-aware logger (`r.Logger` for agents, `t.Logger` for tools) instead of `log.Printf`.

**Important Notes:**
- Examples excluded from this phase: `ai-multi-provider`, `mock-services`
- `agent-example` does NOT use the telemetry module and should remain that way
- The change is optional for startup/shutdown messages where component identification is less critical

#### Example Migration Summary

| Example | Type | Has Telemetry | log.Printf Count | Logger Count | Priority |
|---------|------|---------------|------------------|--------------|----------|
| `agent-example` | Agent | No | 15 | 80 | Medium |
| `agent-with-orchestration` | Agent | Yes | 15 | 37 | High |
| `agent-with-resilience` | Agent | Yes | 18 | 43 | High |
| `agent-with-telemetry` | Agent | Yes | 28 | 84 | High |
| `ai-agent-example` | Agent | No | 25 | 17 | Medium |
| `country-info-tool` | Tool | Yes | 4 | 3 | Low |
| `currency-tool` | Tool | Yes | 11 | 8 | Medium |
| `geocoding-tool` | Tool | Yes | 12 | 10 | Medium |
| `grocery-tool` | Tool | Yes | 25 | 18 | High |
| `news-tool` | Tool | Yes | 6 | 3 | Low |
| `stock-market-tool` | Tool | Yes | 13 | 24 | Medium |
| `tool-example` | Tool | Yes | 12 | 17 | Medium |
| `weather-tool-v2` | Tool | Yes | 12 | 10 | Medium |

#### Change Pattern for Each Example Type

##### For Agents (with telemetry)

Examples: `agent-with-orchestration`, `agent-with-resilience`, `agent-with-telemetry`

**Before (log.Printf - no component):**
```go
func (t *TravelResearchAgent) handleNaturalOrchestration(w http.ResponseWriter, r *http.Request) {
    log.Printf("Processing natural language orchestration request")
    // ...
}
```

**After (Logger.InfoWithContext - with component):**
```go
func (t *TravelResearchAgent) handleNaturalOrchestration(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    t.Logger.InfoWithContext(ctx, "Processing natural language orchestration request", map[string]interface{}{
        "method": r.Method,
        "path":   r.URL.Path,
    })
    // ...
}
```

##### For Agents (without telemetry)

Examples: `agent-example`, `ai-agent-example`

**Note:** These examples do NOT import the telemetry module and should remain that way.

**Before (log.Printf - no component):**
```go
func (r *ResearchAgent) handleResearch(w http.ResponseWriter, req *http.Request) {
    log.Printf("Starting research topic orchestration")
    // ...
}
```

**After (Logger.Info - with component, no context):**
```go
func (r *ResearchAgent) handleResearch(w http.ResponseWriter, req *http.Request) {
    r.Logger.Info("Starting research topic orchestration", map[string]interface{}{
        "method": req.Method,
        "path":   req.URL.Path,
    })
    // ...
}
```

##### For Tools

Examples: `tool-example`, `weather-tool-v2`, `geocoding-tool`, `currency-tool`, `country-info-tool`, `news-tool`, `stock-market-tool`, `grocery-tool`

**Before (log.Printf - no component):**
```go
func (t *WeatherTool) handleCurrentWeather(w http.ResponseWriter, r *http.Request) {
    log.Printf("Processing weather request for location: %s", location)
    // ...
}
```

**After (Logger.InfoWithContext - with component):**
```go
func (t *WeatherTool) handleCurrentWeather(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    t.Logger.InfoWithContext(ctx, "Processing weather request", map[string]interface{}{
        "location": location,
    })
    // ...
}
```

#### Detailed Changes Per Example

##### 1. agent-example (Agent, No Telemetry)

**Files to modify:** `research_agent.go`, `orchestration.go`

**Change type:** Replace `log.Printf` with `r.Logger.Info()` in handler functions

**Special considerations:**
- Do NOT add telemetry import - this example demonstrates basic agent without telemetry
- Use `r.Logger.Info()` (not `InfoWithContext`) since telemetry context is not available
- Expected component in logs: `"component":"agent/research-agent"`

##### 2. agent-with-orchestration (Agent, With Telemetry)

**Files to modify:** `research_agent.go`, `orchestration.go`

**Change type:** Replace `log.Printf` with `t.Logger.InfoWithContext(ctx, ...)`

**Special considerations:**
- Use `InfoWithContext` to correlate with distributed traces
- Expected component in logs: `"component":"agent/travel-research-agent"`

##### 3. agent-with-resilience (Agent, With Telemetry)

**Files to modify:** `research_agent.go`, `orchestration.go`, `handlers.go`

**Change type:** Replace `log.Printf` with `r.Logger.InfoWithContext(ctx, ...)`

**Special considerations:**
- Already uses resilience patterns which have `"component":"framework/resilience"`
- Agent handler logs should show `"component":"agent/research-assistant-resilience"`

##### 4. agent-with-telemetry (Agent, With Telemetry)

**Files to modify:** `research_agent.go`, `main.go`

**Change type:** Replace `log.Printf` with `r.Logger.InfoWithContext(ctx, ...)`

**Special considerations:**
- This is the primary example for testing component-aware logging
- Expected component in logs: `"component":"agent/research-agent-telemetry"`

##### 5. ai-agent-example (Agent, No Telemetry)

**Files to modify:** `main.go`

**Change type:** Replace `log.Printf` with `agent.Logger.Info()`

**Special considerations:**
- Do NOT add telemetry import
- Uses AI module for text generation
- Expected component in logs: `"component":"agent/ai-agent-example"`

##### 6. tool-example (Tool, With Telemetry)

**Files to modify:** `weather_tool.go`, `handlers.go`

**Change type:** Replace `log.Printf` with `t.Logger.InfoWithContext(ctx, ...)`

**Special considerations:**
- Reference example for tool development
- Expected component in logs: `"component":"tool/weather-service"`

##### 7. weather-tool-v2 (Tool, With Telemetry)

**Files to modify:** `weather_tool.go`, `handlers.go`

**Change type:** Replace `log.Printf` with `t.Logger.InfoWithContext(ctx, ...)`

**Special considerations:**
- Enhanced weather tool with multiple data sources
- Expected component in logs: `"component":"tool/weather-service"`

##### 8. geocoding-tool (Tool, With Telemetry)

**Files to modify:** `geocoding_tool.go`, `handlers.go`

**Change type:** Replace `log.Printf` with `t.Logger.InfoWithContext(ctx, ...)`

**Special considerations:**
- Expected component in logs: `"component":"tool/geocoding-service"`

##### 9. currency-tool (Tool, With Telemetry)

**Files to modify:** `currency_tool.go`, `handlers.go`

**Change type:** Replace `log.Printf` with `t.Logger.InfoWithContext(ctx, ...)`

**Special considerations:**
- Expected component in logs: `"component":"tool/currency-service"`

##### 10. country-info-tool (Tool, With Telemetry)

**Files to modify:** `country_info_tool.go`, `handlers.go`

**Change type:** Replace `log.Printf` with `t.Logger.InfoWithContext(ctx, ...)`

**Special considerations:**
- Low priority - only 4 log.Printf calls
- Expected component in logs: `"component":"tool/country-info-service"`

##### 11. news-tool (Tool, With Telemetry)

**Files to modify:** `news_tool.go`, `handlers.go`

**Change type:** Replace `log.Printf` with `t.Logger.InfoWithContext(ctx, ...)`

**Special considerations:**
- Low priority - only 6 log.Printf calls
- Expected component in logs: `"component":"tool/news-service"`

##### 12. stock-market-tool (Tool, With Telemetry)

**Files to modify:** `stock_tool.go`, `handlers.go`

**Change type:** Replace `log.Printf` with `t.Logger.InfoWithContext(ctx, ...)`

**Special considerations:**
- Expected component in logs: `"component":"tool/stock-market-service"`

##### 13. grocery-tool (Tool, With Telemetry)

**Files to modify:** `grocery_tool.go`, `handlers.go`

**Change type:** Replace `log.Printf` with `t.Logger.InfoWithContext(ctx, ...)`

**Special considerations:**
- High priority - 25 log.Printf calls
- Expected component in logs: `"component":"tool/grocery-service"`

#### Expected Log Output After Phase 6

##### Agent Log (agent-with-telemetry)
```json
{
  "timestamp": "2025-01-15T10:30:02Z",
  "level": "INFO",
  "service": "research-agent-telemetry",
  "component": "agent/research-agent-telemetry",
  "message": "Processing research topic",
  "topic": "weather in Tokyo",
  "method": "POST",
  "trace.trace_id": "abc123"
}
```

##### Tool Log (weather-tool-v2)
```json
{
  "timestamp": "2025-01-15T10:30:03Z",
  "level": "INFO",
  "service": "weather-tool-v2",
  "component": "tool/weather-service",
  "message": "Fetching weather data",
  "location": "Tokyo",
  "provider": "openweathermap",
  "trace.trace_id": "abc123"
}
```

#### Verification After Phase 6

After updating an example, verify component-aware logging is working:

```bash
# Deploy the example
cd examples/<example-name>
./setup.sh deploy

# Port forward and make a request
./setup.sh forward
curl -X POST http://localhost:<port>/api/capabilities/<capability> \
  -H "Content-Type: application/json" \
  -d '{"param":"value"}'

# Check logs for component field
kubectl logs -n gomind-examples -l app=<app-name> --since=60s | \
  grep -o '"component":"[^"]*"' | sort | uniq -c
```

Expected output should show both framework and agent/tool components:
```
  5 "component":"agent/research-agent-telemetry"
 15 "component":"framework/core"
  3 "component":"framework/resilience"
```
