# Automatic Service Type Inference

GoMind automatically infers whether your service is a **Tool** or an **Agent** for Grafana dashboard segregation. This requires **zero configuration** - just rebuild and redeploy!

## Problem Statement

The Grafana dashboard's "Active Agents" panel was showing 10 when only 4 agents were deployed. Investigation revealed the metric query was counting ALL services (tools + agents) because there was no way to distinguish between them.

**Before:**
```promql
# Counted everything - tools AND agents
count(count by (exported_job) (gomind_request_total))
```

**After:**
```promql
# Count only tools
count(count by (exported_job) (gomind_request_total{service_type="tool"}))

# Count only agents
count(count by (exported_job) (gomind_request_total{service_type="agent"}))
```

## Solution Design

### Key Insight

The distinction between Tool and Agent is **fundamental and binding** in GoMind's architecture:

- **Tools** are created with `core.NewTool()` - passive, cannot discover other components
- **Agents** are created with `core.NewBaseAgent()` - active, can discover and orchestrate

This means the service type can be **automatically inferred** at component creation time.

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        User Code                                 │
├─────────────────────────────────────────────────────────────────┤
│  agent := core.NewBaseAgent("my-agent")                         │
│           ↓                                                      │
│  SetCurrentComponentType(ComponentTypeAgent)  ← AUTO-TRACKED    │
│           ↓                                                      │
│  telemetry.Initialize(config)                                   │
│           ↓                                                      │
│  config.ServiceType = core.GetCurrentComponentType() ← AUTO-READ│
│           ↓                                                      │
│  OTel Resource: service.type = "agent"                          │
└─────────────────────────────────────────────────────────────────┘
```

### Priority Chain for ServiceType

1. **Explicit config** (highest priority): `config.ServiceType = "tool"`
2. **Auto-inferred**: From `core.GetCurrentComponentType()`
3. **Environment variable** (fallback): `GOMIND_SERVICE_TYPE=tool`

## Files Changed

| File | Change |
|------|--------|
| `core/component.go` | Added package-level tracking with `SetCurrentComponentType()` and `GetCurrentComponentType()` |
| `core/tool.go` | `NewToolWithConfig()` calls `SetCurrentComponentType(ComponentTypeTool)` |
| `core/agent.go` | `NewBaseAgentWithConfig()` calls `SetCurrentComponentType(ComponentTypeAgent)` |
| `telemetry/config.go` | Added `ServiceType string` field to `Config` struct |
| `telemetry/otel.go` | Updated `NewOTelProvider()` to accept `serviceType` parameter; adds `service.type` as OTel resource attribute |
| `telemetry/registry.go` | `newRegistry()` reads from `core.GetCurrentComponentType()` if `ServiceType` is empty; added `InitializeForComponent()` helper |
| `examples/k8-deployment/grafana.yaml` | Split panel into "Active Tools" (blue) and "Active Agents" (purple) |

## Detailed Changes

### 1. core/component.go

Added thread-safe package-level tracking:

```go
var (
    currentComponentType ComponentType
    componentTypeMu      sync.RWMutex
)

// SetCurrentComponentType sets the current component type (called by NewTool/NewBaseAgent)
func SetCurrentComponentType(t ComponentType) {
    componentTypeMu.Lock()
    defer componentTypeMu.Unlock()
    currentComponentType = t
}

// GetCurrentComponentType returns the current component type for telemetry inference
func GetCurrentComponentType() ComponentType {
    componentTypeMu.RLock()
    defer componentTypeMu.RUnlock()
    return currentComponentType
}
```

### 2. core/tool.go

```go
func NewToolWithConfig(config *Config) *BaseTool {
    // ... existing code ...

    // Track component type for automatic telemetry inference
    SetCurrentComponentType(ComponentTypeTool)

    return &BaseTool{...}
}
```

### 3. core/agent.go

```go
func NewBaseAgentWithConfig(config *Config) *BaseAgent {
    // ... existing code ...

    // Track component type for automatic telemetry inference
    SetCurrentComponentType(ComponentTypeAgent)

    return &BaseAgent{...}
}
```

### 4. telemetry/config.go

```go
type Config struct {
    Enabled     bool
    ServiceName string
    ServiceType string // "tool" or "agent" - automatically inferred from component type
    Endpoint    string
    // ... rest of fields ...
}
```

### 5. telemetry/otel.go

Updated function signature and resource creation:

```go
func NewOTelProvider(serviceName, serviceType, endpoint string) (*OTelProvider, error) {
    // Fallback to env var if not provided
    if serviceType == "" {
        serviceType = os.Getenv("GOMIND_SERVICE_TYPE")
    }

    // Build resource attributes
    attrs := []attribute.KeyValue{
        semconv.ServiceNameKey.String(serviceName),
        semconv.ServiceVersionKey.String("1.0.0"),
    }
    // Add service.type if provided
    if serviceType != "" {
        attrs = append(attrs, attribute.String("service.type", serviceType))
    }

    res := resource.NewWithAttributes(semconv.SchemaURL, attrs...)
    // ...
}
```

### 6. telemetry/registry.go

Auto-inference in `newRegistry()`:

```go
func newRegistry(config Config) (*Registry, error) {
    // ... existing defaults ...

    // Auto-infer ServiceType from the most recently created component
    if config.ServiceType == "" {
        config.ServiceType = string(core.GetCurrentComponentType())
    }

    provider, err := NewOTelProvider(config.ServiceName, config.ServiceType, config.Endpoint)
    // ...
}
```

New helper function:

```go
// InitializeForComponent initializes telemetry with automatic service type inference
func InitializeForComponent(component interface{ GetType() core.ComponentType }, config Config) error {
    config.ServiceType = string(component.GetType())
    return Initialize(config)
}
```

### 7. examples/k8-deployment/grafana.yaml

Split into two panels:

```json
// Active Tools (blue, width: 2)
{
    "title": "Active Tools",
    "expr": "count(count by (exported_job) (gomind_request_total{service_type=\"tool\"}))"
}

// Active Agents (purple, width: 2)
{
    "title": "Active Agents",
    "expr": "count(count by (exported_job) (gomind_request_total{service_type=\"agent\"}))"
}
```

## Usage

### Automatic (Zero Configuration)

**IMPORTANT:** Create the component (agent/tool) BEFORE initializing telemetry. This ensures `core.GetCurrentComponentType()` returns the correct type.

```go
// 1. Create your component FIRST - this sets the component type
agent := core.NewBaseAgent("my-agent")

// 2. THEN initialize telemetry - type is auto-inferred!
config := telemetry.UseProfile(telemetry.ProfileProduction)
config.ServiceName = "my-agent"
telemetry.Initialize(config)  // ServiceType automatically set to "agent"
```

**Wrong order (will result in empty service_type):**
```go
// DON'T DO THIS - telemetry initialized before agent creation
telemetry.Initialize(config)  // ServiceType will be empty!
agent := core.NewBaseAgent("my-agent")  // Too late
```

### Manual Options

```go
// Option 1: Explicit in config
config.ServiceType = "tool"
telemetry.Initialize(config)

// Option 2: Use helper function
tool := core.NewTool("my-tool")
telemetry.InitializeForComponent(tool, config)

// Option 3: Environment variable (no code change)
// Set GOMIND_SERVICE_TYPE=tool in k8-deployment.yaml
```

## Deployment

After making these changes:

1. **Update OTel Collector config** to convert resource attributes to labels:
   ```yaml
   exporters:
     prometheus:
       endpoint: "0.0.0.0:8889"
       namespace: "gomind"
       resource_to_telemetry_conversion:
         enabled: true  # Required for service.type to appear as label
   ```

2. **Rebuild** all tools and agents with the updated code
3. **Redeploy** to Kubernetes
4. **Redeploy Grafana** with updated dashboard config:
   ```bash
   kubectl delete configmap grafana-dashboards -n gomind-examples
   kubectl apply -f examples/k8-deployment/grafana.yaml
   kubectl rollout restart deployment grafana -n gomind-examples
   ```

The dashboard will show `0` for tools/agents until services emit metrics with the new `service.type` attribute.

## Thread Safety

The component type tracking uses `sync.RWMutex` for thread-safe access:
- Write lock in `SetCurrentComponentType()` (called during component creation)
- Read lock in `GetCurrentComponentType()` (called during telemetry init)

This is safe because each microservice typically creates ONE component before initializing telemetry.

## Backward Compatibility

- **Existing code**: Works without changes - if no ServiceType is set and no component was created, the field remains empty (no `service.type` attribute added)
- **Existing metrics**: Continue to work - the filter `{service_type="tool"}` simply won't match old metrics
- **Mixed deployments**: Old services show in "Active Tools and Agents" (if you keep that panel), new services show in segregated panels

## Test Coverage

Tests are located in `core/component_test.go`:

| Test | Description |
|------|-------------|
| `TestComponentTypeTracking` | Basic set/get operations for component type |
| `TestNewToolSetsComponentType` | Verifies `NewTool()` sets `ComponentTypeTool` |
| `TestNewBaseAgentSetsComponentType` | Verifies `NewBaseAgent()` sets `ComponentTypeAgent` |
| `TestComponentTypeOverwrite` | Verifies last-created component wins (expected behavior) |
| `TestComponentTypeThreadSafety` | Concurrent access safety with 100 goroutines |
| `BenchmarkComponentTypeTracking` | Performance benchmarks for Set/Get operations |

Run tests:
```bash
go test ./core -run TestComponentType -v
```

## Code References

Exact line numbers for key implementations (as of December 2024):

| File | Lines | Description |
|------|-------|-------------|
| `core/component.go` | 17-37 | `currentComponentType`, `componentTypeMu`, `SetCurrentComponentType()`, `GetCurrentComponentType()` |
| `core/tool.go` | ~80 | `SetCurrentComponentType(ComponentTypeTool)` call in `NewToolWithConfig()` |
| `core/agent.go` | ~134 | `SetCurrentComponentType(ComponentTypeAgent)` call in `NewBaseAgentWithConfig()` |
| `telemetry/config.go` | ~25 | `ServiceType string` field in `Config` struct |
| `telemetry/registry.go` | 206-208 | Auto-inference: `config.ServiceType = string(core.GetCurrentComponentType())` |
| `telemetry/otel.go` | 103-110 | `service.type` resource attribute creation |

## Related Documentation

- **User-facing docs**: `telemetry/README.md` - Section "🏷️ Service Type Labeling"
- **Example pattern**: See any tool/agent in `examples/` for correct initialization order

## Troubleshooting

### service_type label not appearing in Prometheus

1. **Check initialization order**: Component must be created BEFORE `telemetry.Initialize()`
2. **Verify OTel Collector config**: `resource_to_telemetry_conversion.enabled: true`
3. **Check metrics endpoint**: `curl http://localhost:8889/metrics | grep service_type`

### Dashboard shows 0 for Active Tools/Agents

1. Services need to emit at least one request metric after redeployment
2. Run a test request: `curl http://service:port/health`
3. Verify in Prometheus: `gomind_request_total{service_type="tool"}`
