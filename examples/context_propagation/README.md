# Context Propagation Example

A comprehensive demonstration of context propagation and telemetry patterns using the GoMind framework's telemetry system with OpenTelemetry baggage.

## 🎯 What This Example Demonstrates

- **Context Propagation**: Automatic propagation of request metadata through deep call stacks
- **Telemetry Integration**: Using `EmitWithContext()` for consistent metric labeling
- **Baggage Management**: Adding and managing context baggage at different layers
- **Request Lifecycle Tracking**: Complete request flow monitoring from edge to business logic
- **Multi-Layer Observability**: Telemetry across authentication, database, cache, and business logic layers
- **Production Patterns**: Realistic simulation of multi-tenant, multi-user system behavior

## 🏗️ Architecture

```
Request → handleRequest() [adds baggage] → authenticateUser()
                                       └→ fetchFromDatabase() → checkCache()
                                       └→ cacheData()
                                       └→ processBusinessLogic()
```

Each function automatically inherits all context labels and can add layer-specific metadata.

## 📊 Telemetry Labels Propagated

### Edge Context (Added Once)
- `request_id` - Unique request identifier
- `user_id` - User identifier
- `tenant_id` - Tenant identifier for multi-tenancy
- `path` - API endpoint path
- `method` - HTTP method

### Layer-Specific Context (Added by Each Layer)
- Database: `db.operation`, `db.table`
- Cache: `cache.operation`
- Business Logic: `processor`, `algorithm`

## 📝 Code Structure

```
context-propagation-example/
├── main.go          # Complete HTTP request simulation
├── go.mod          # Module configuration with telemetry dependencies
└── README.md       # This documentation
```

## 🚀 Running the Example

### Prerequisites
- Go 1.25 or later
- GoMind framework (automatically resolved via local replace directive)

### Quick Start

```bash
# Navigate to the example directory
cd examples/context_propagation

# Run the demonstration
env GOWORK=off go run .
```

### Expected Output
The example processes multiple requests with full telemetry:

```
=== Context Propagation Example ===
Processing request req-001

Processing request req-002

Processing request req-003

=== Telemetry Summary ===
Metrics Emitted: 47
Errors: 0
```

Note: The shutdown error about connecting to localhost:4318 is expected when no OTLP collector is running locally.

## 📈 Metrics Generated

### Request-Level Metrics
- `http.request.started` - Request initiation
- `http.request.duration_ms` - Request duration
- `http.request.completed` - Request completion

### Authentication Metrics
- `auth.attempt` - Authentication attempts
- `auth.success` / `auth.failure` - Auth outcomes

### Database Metrics
- `db.query.started` - Database query initiation
- `db.query.duration_ms` - Query execution time
- `db.query.success` / `db.query.failure` - Query outcomes
- `db.cache.hit` / `db.cache.miss` - Cache statistics

### Cache Metrics
- `cache.lookup` - Cache lookup attempts
- `cache.write` - Cache write operations
- `cache.bytes` - Cached data size

### Business Logic Metrics
- `business_logic.started` / `business_logic.completed` - Processing lifecycle
- `business_logic.duration_ms` - Processing time
- `items.processed` - Business-specific metrics

## 🔍 Key Learning Points

### 1. Context Creation at Edge
```go
// Create context ONCE at system boundary
ctx := telemetry.WithBaggage(context.Background(),
    "request_id", req.ID,
    "user_id", req.UserID,
    "tenant_id", req.TenantID,
)
```

### 2. Automatic Label Inheritance
```go
// All metrics automatically include edge context
telemetry.EmitWithContext(ctx, "auth.attempt", 1)
// Result: metric has request_id, user_id, tenant_id labels
```

### 3. Layer-Specific Context Addition
```go
// Add layer context without losing parent context
ctx = telemetry.WithBaggage(ctx,
    "db.operation", "select",
    "db.table", "products")
```

### 4. Consistent Metric Emission
```go
// All functions use EmitWithContext for consistent labeling
telemetry.EmitWithContext(ctx, "db.query.duration_ms", float64(duration))
```

## 🛠️ Customization

### Add Custom Context
```go
// Add your own domain-specific context
ctx = telemetry.WithBaggage(ctx,
    "service_version", "v2.1",
    "feature_flag", "new_algorithm")
```

### Extend Metrics
```go
// Add custom metrics with full context propagation
telemetry.EmitWithContext(ctx, "custom.metric", value)
```

### Integration with Real Services
Replace the simulation functions with real service calls:
- HTTP handlers
- Database clients
- Cache clients
- Message queue consumers

## 🎯 Production Use Cases

### Request Tracing
```bash
# Find all metrics for a specific request
request_id="req-001"
```

### User Behavior Analysis
```bash
# Analyze specific user patterns
user_id="user-123"
```

### Multi-Tenant Monitoring
```bash
# Monitor tenant-specific performance
tenant_id="tenant-abc"
```

### Performance Debugging
```bash
# Drill down through layers
request_id="req-001" AND db.operation="select"
```

## 🔧 Local Testing with OTLP Collector

To see full telemetry output, run a local OTLP collector:

```bash
# Using Docker
docker run -p 4317:4317 -p 4318:4318 \
  otel/opentelemetry-collector:latest
```

## 📚 Related Examples

- [Error Handling Example](../error_handling/) - Complementary error handling patterns
- [Agent Example](../agent-example/) - Real-world service with context propagation
- [Telemetry Module](../../telemetry/) - Complete telemetry system documentation

## 🔗 Framework Documentation

- [Telemetry API](../../telemetry/api.go) - Core telemetry functions
- [Context Management](../../telemetry/context.go) - Baggage and context utilities
- [OpenTelemetry Integration](../../telemetry/README.md) - Full telemetry system overview