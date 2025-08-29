# GoMind Core Module

The core module provides the fundamental building blocks for the GoMind framework. It contains essential interfaces, base agent implementation, configuration management, and basic discovery capabilities. This module is designed to be lightweight (8MB) and serves as the foundation for all other GoMind modules.

## Table of Contents
- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Components](#components)
- [Configuration](#configuration)
- [Examples](#examples)
- [API Reference](#api-reference)

## Features

- **Lightweight Foundation**: Minimal dependencies, ~8MB footprint
- **Base Agent Implementation**: Ready-to-use agent with HTTP server, discovery, and state management
- **Service Discovery**: Built-in support for Redis-based and mock discovery
- **Configuration Management**: Flexible configuration with builder pattern
- **CORS Support**: Built-in CORS middleware for web applications
- **Memory Storage**: In-memory and Redis-backed state storage
- **Extensible Interfaces**: Clean interfaces for logging, telemetry, AI, and discovery

## Installation

```bash
go get github.com/itsneelabh/gomind/core
```

## Quick Start

### Creating a Simple Agent

```go
package main

import (
    "context"
    "log"
    "github.com/itsneelabh/gomind/core"
)

func main() {
    // Create a simple agent
    agent := core.NewBaseAgent("my-agent")
    
    // Add capabilities
    agent.AddCapability(core.Capability{
        Name:        "greet",
        Description: "Greets the user",
        Endpoint:    "/api/greet",
    })
    
    // Create framework and run
    framework, err := core.NewFramework(agent, core.WithPort(8080))
    if err != nil {
        log.Fatal(err)
    }
    
    if err := framework.Run(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

### Using Configuration Options

```go
// Create agent with custom configuration
config := core.NewConfig(
    core.WithName("stock-analyzer"),
    core.WithPort(8081),
    core.WithNamespace("financial"),
    core.WithRedisURL("redis://localhost:6379"),
    core.WithCORSDefaults(),
)

agent := core.NewBaseAgentWithConfig(config)
```

## Components

### 1. Agent Interface

The core `Agent` interface that all agents must implement:

```go
type Agent interface {
    Initialize(ctx context.Context) error
    GetID() string
    GetName() string
    GetCapabilities() []Capability
}
```

### 2. BaseAgent

The `BaseAgent` provides a complete implementation with:
- HTTP server with health checks
- Service discovery registration
- State management
- Capability management
- Extensible HTTP routing

### 3. Capability

Represents a capability that an agent provides:

```go
type Capability struct {
    Name        string   // Unique capability name
    Description string   // Human-readable description
    Endpoint    string   // HTTP endpoint path
    InputTypes  []string // Expected input types
    OutputTypes []string // Output types produced
}
```

### 4. Discovery Interface

Service discovery abstraction:

```go
type Discovery interface {
    Register(ctx context.Context, registration *ServiceRegistration) error
    Unregister(ctx context.Context, serviceID string) error
    FindService(ctx context.Context, serviceName string) ([]*ServiceRegistration, error)
    FindByCapability(ctx context.Context, capability string) ([]*ServiceRegistration, error)
    UpdateHealth(ctx context.Context, serviceID string, status HealthStatus) error
}
```

### 5. Memory Interface

State storage abstraction:

```go
type Memory interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key string, value string, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
}
```

## Configuration

The module uses a flexible configuration system with builder pattern:

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithName(name)` | Set agent name | "gomind-agent" |
| `WithPort(port)` | Set HTTP server port | 8080 |
| `WithAddress(addr)` | Set bind address | "0.0.0.0" |
| `WithNamespace(ns)` | Set namespace | "default" |
| `WithRedisURL(url)` | Configure Redis discovery | "" |
| `WithCORS(config)` | Configure CORS | disabled |
| `WithCORSDefaults()` | Enable CORS with defaults | - |
| `WithDevelopmentMode()` | Enable development mode | false |
| `WithMockDiscovery()` | Use mock discovery | false |

### Configuration File

You can also load configuration from a YAML file:

```yaml
# config.yaml
name: my-agent
port: 8080
namespace: production
discovery:
  enabled: true
  provider: redis
  redis_url: redis://localhost:6379
cors:
  enabled: true
  allowed_origins: ["*"]
  allowed_methods: ["GET", "POST", "PUT", "DELETE"]
logging:
  level: info
  format: json
```

```go
config, err := core.LoadConfig("config.yaml")
agent := core.NewBaseAgentWithConfig(config)
```

## Examples

### 1. Agent with Redis Discovery

```go
package main

import (
    "context"
    "github.com/itsneelabh/gomind/core"
)

func main() {
    // Create discovery service
    discovery, _ := core.NewRedisDiscovery("redis://localhost:6379")
    
    // Create agent with discovery
    agent := core.NewBaseAgent("analytics-agent")
    agent.Discovery = discovery
    
    // Add capabilities
    agent.AddCapability(core.Capability{
        Name:        "analyze_data",
        Description: "Analyzes data patterns",
        Endpoint:    "/api/analyze",
    })
    
    // Run with framework
    framework, _ := core.NewFramework(agent, 
        core.WithPort(8080),
        core.WithDiscovery(discovery),
    )
    
    framework.Run(context.Background())
}
```

### 2. Agent with State Management

```go
// Using memory for state management
agent := core.NewBaseAgent("stateful-agent")

// Store state
ctx := context.Background()
agent.Memory.Set(ctx, "user:123", `{"name":"John","score":100}`, 1*time.Hour)

// Retrieve state
data, _ := agent.Memory.Get(ctx, "user:123")

// Check existence
exists, _ := agent.Memory.Exists(ctx, "user:123")
```

### 3. Custom HTTP Handlers

```go
agent := core.NewBaseAgent("api-agent")

// Add custom HTTP handler
agent.HandleFunc("/api/custom", func(w http.ResponseWriter, r *http.Request) {
    // Your custom logic here
    json.NewEncoder(w).Encode(map[string]string{
        "status": "success",
        "message": "Custom endpoint",
    })
})

// Add capability for discovery
agent.AddCapability(core.Capability{
    Name:     "custom_endpoint",
    Endpoint: "/api/custom",
})
```

### 4. Agent with Mock Discovery (Development)

```go
// Perfect for development and testing
agent := core.NewBaseAgent("dev-agent")
discovery := core.NewMockDiscovery()

// Register some mock services
discovery.Register(context.Background(), &core.ServiceRegistration{
    ID:           "mock-1",
    Name:         "mock-service",
    Capabilities: []string{"test"},
})

agent.Discovery = discovery
```

## API Reference

### BaseAgent Methods

| Method | Description |
|--------|-------------|
| `Initialize(ctx)` | Initialize the agent and its components |
| `GetID()` | Get the unique agent ID |
| `GetName()` | Get the agent name |
| `GetCapabilities()` | Get list of capabilities |
| `AddCapability(cap)` | Add a new capability |
| `HandleFunc(path, handler)` | Register HTTP handler |
| `Start(ctx)` | Start the HTTP server |
| `Stop(ctx)` | Stop the agent gracefully |

### Framework Methods

| Method | Description |
|--------|-------------|
| `NewFramework(agent, opts...)` | Create new framework instance |
| `Run(ctx)` | Run the agent with all components |
| `Shutdown(ctx)` | Graceful shutdown |

### Discovery Methods

| Method | Description |
|--------|-------------|
| `Register(ctx, registration)` | Register service |
| `Unregister(ctx, serviceID)` | Unregister service |
| `FindService(ctx, name)` | Find services by name |
| `FindByCapability(ctx, cap)` | Find services by capability |
| `UpdateHealth(ctx, id, status)` | Update service health |

### Memory Methods

| Method | Description |
|--------|-------------|
| `Get(ctx, key)` | Get value by key |
| `Set(ctx, key, value, ttl)` | Set value with TTL |
| `Delete(ctx, key)` | Delete key |
| `Exists(ctx, key)` | Check if key exists |

## Best Practices

1. **Use Configuration Options**: Leverage the builder pattern for clean configuration
2. **Implement Graceful Shutdown**: Always handle context cancellation
3. **Register Capabilities**: Make your agent discoverable by registering capabilities
4. **Use Namespaces**: Organize agents in different namespaces for better isolation
5. **Enable Health Checks**: The built-in `/health` endpoint helps with monitoring
6. **Handle Errors**: Always check and handle errors appropriately

## Testing

The module includes comprehensive test utilities:

```go
// Use mock discovery for testing
discovery := core.NewMockDiscovery()

// Use in-memory store for testing
memory := core.NewInMemoryStore()

// Create test agent
agent := core.NewBaseAgent("test-agent")
agent.Discovery = discovery
agent.Memory = memory

// Test your agent logic
```

## Performance Considerations

- **Lightweight**: Core module adds only ~8MB to your binary
- **Efficient Discovery**: Redis discovery uses caching to minimize network calls
- **Connection Pooling**: HTTP client reuses connections
- **Graceful Shutdown**: Proper cleanup of resources

## Migration Guide

If you're migrating from the monolithic framework:

```go
// Old way
import "github.com/itsneelabh/gomind"

// New way
import "github.com/itsneelabh/gomind/core"

// The API remains largely the same
agent := core.NewBaseAgent("my-agent")  // Previously gomind.NewBaseAgent
```

## Contributing

Contributions are welcome! The core module is designed to be:
- Minimal and focused
- Well-tested
- Backward compatible
- Clear and documented

## License

See the main GoMind repository for license information.