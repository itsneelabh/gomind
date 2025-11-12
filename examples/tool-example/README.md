# Weather Tool Example

A comprehensive example demonstrating how to build a **Tool** (passive component) using the GoMind framework. This tool provides weather-related capabilities and showcases the framework's auto-discovery, capability registration, and production-ready patterns.

## 🎯 What This Example Demonstrates

- **Tool Pattern**: Passive component that registers capabilities but cannot discover other components
- **Framework Integration**: Auto-dependency injection, Redis discovery, health checks
- **Multiple Capabilities**: Different endpoint patterns and handler types
- **Production Patterns**: Error handling, logging, caching, metrics
- **Kubernetes Deployment**: Complete K8s configuration with monitoring
- **Clean Architecture**: Decomposed into 4 focused files for maintainability (119-158 lines each)

## 🏗️ Architecture

```
┌─────────────────────────────────────┐
│         Weather Tool                │
│         (Passive Component)         │
├─────────────────────────────────────┤
│ Capabilities:                       │
│ • current_weather                   │
│ • forecast                          │
│ • alerts                           │
│ • historical_analysis              │
├─────────────────────────────────────┤
│ Framework Features:                 │
│ • Auto-endpoint generation          │
│ • Redis registry                    │
│ • Health checks                     │
│ • CORS support                      │
│ • Memory caching                    │
└─────────────────────────────────────┘
```

**Key Principle**: Tools are **passive** - they register themselves with the framework but cannot discover or call other components.

## 🚀 Quick Start - Local Kind Cluster (2 minutes)

### Prerequisites
- Docker Desktop or Docker Engine
- [kind](https://kind.sigs.k8s.io/) - Kubernetes in Docker
- [kubectl](https://kubernetes.io/docs/tasks/tools/) - Kubernetes CLI
- Optional: Weather API key from [weatherapi.com](https://www.weatherapi.com/)

### Option 1: Complete Setup (Easiest)

```bash
# 1. Navigate to the example
cd examples/tool-example

# 2. (Optional) Configure Weather API key for real data
cp .env.example .env
# Edit .env and add your WEATHER_API_KEY (tool works with mock data without key)

# 3. Deploy everything with one command
make all

# That's it! The tool is now running in your local Kind cluster.
```

### Option 2: Step-by-Step Setup

```bash
# 1. Create Kind cluster and install dependencies
make setup

# 2. Deploy the weather tool
make deploy

# 3. Test the deployment
make test

# 4. View logs
make logs
```

### Test the Deployed Tool

```bash
# Port forward to access the tool
kubectl port-forward -n gomind-examples svc/weather-service 8080:80

# In another terminal:
# Health check
curl http://localhost:8080/health

# List all capabilities
curl http://localhost:8080/api/capabilities

# Test current weather capability
curl -X POST http://localhost:8080/api/capabilities/current_weather \
  -H "Content-Type: application/json" \
  -d '{"location":"New York","units":"metric"}'

# Test Phase 3 schema endpoint (v0.6.4 feature)
curl http://localhost:8080/api/capabilities/current_weather/schema
```

### Expected Response

```json
{
  "location": "New York",
  "temperature": 22.5,
  "humidity": 65,
  "condition": "partly cloudy",
  "wind_speed": 10.5,
  "timestamp": "2024-01-15T10:30:00Z",
  "source": "weather-service-v1.0"
}
```

### Makefile Commands Reference

```bash
make help         # Show all available commands
make setup        # Create Kind cluster and install dependencies
make deploy       # Build, load, and deploy the tool
make test         # Run automated tests against deployed tool
make logs         # View tool logs (follows)
make status       # Check deployment status
make port-forward # Start port forwarding (Ctrl+C to stop)
make debug        # Show debug information
make clean        # Delete Kind cluster and clean up
```

## 🆕 What's New in v0.6.4

This example showcases the latest GoMind v0.6.4 features:

### 3-Phase AI-Powered Schema Discovery

- **Phase 1: Description-Based** (Always Active)
  - AI generates payloads from capability descriptions
  - ~85-90% accuracy baseline

- **Phase 2: Field-Hint-Based** (Implemented ✓)
  - All capabilities include structured field hints with `InputSummary`
  - AI uses exact field names, types, and examples
  - ~95% accuracy for tool calls
  - See [main.go:93-238](main.go#L93-L238) for implementation

- **Phase 3: Schema Validation** (Available)
  - Full JSON Schema v7 validation via `/api/capabilities/{name}/schema`
  - Cached in Redis for 0ms overhead after first fetch
  - Enable validation with `GOMIND_VALIDATE_PAYLOADS=true`

### Environment-Based Configuration

All configuration now comes from environment variables (no hardcoded values):

```bash
# Core configuration
PORT=8080
NAMESPACE=gomind-examples
DEV_MODE=false

# Discovery (required)
REDIS_URL=redis://redis.default.svc.cluster.local:6379
```

See [.env.example](.env.example) for complete configuration.

### Production-Ready Features

- **Configuration Validation**: `validateConfig()` function catches errors at startup
- **Graceful Shutdown**: 30-second timeout with proper signal handling (SIGINT/SIGTERM)
- **Makefile Automation**: Complete deployment automation for Kind clusters
- **Local Development**: Includes kind-config.yaml, redis-deployment.yaml, setup.sh

### Testing with agent-example

This tool is designed to work alongside [agent-example](../agent-example/README.md) in the same `gomind-demo` cluster:

```bash
# Deploy both examples in the same cluster
cd examples/tool-example
make setup deploy

cd ../agent-example
make deploy  # Reuses existing cluster and Redis

# Both tools now share service discovery via Redis
```

## 📊 Understanding the Code

### Project Structure

The tool-example is organized into 4 focused files following Go best practices:

```
tool-example/
├── main.go              (119 lines)  - Application entry point & lifecycle
│   ├── main() function
│   ├── validateConfig()
│   ├── Framework initialization
│   └── Graceful shutdown handling
│
├── weather_tool.go      (158 lines)  - Component definition
│   ├── WeatherTool struct
│   ├── Request/Response types
│   ├── NewWeatherTool() constructor
│   └── registerCapabilities() - All capability definitions
│
├── handlers.go          (126 lines)  - HTTP request/response handling
│   ├── handleCurrentWeather()
│   ├── handleForecast()
│   └── handleAlerts()
│
└── weather_data.go      (34 lines)   - Business logic & data simulation
    └── simulateWeatherData()
```

**Benefits of this structure:**
- **Separation of Concerns**: Each file has a single, clear responsibility
- **Easy Maintenance**: Know exactly where to make changes
- **Better Readability**: Files are 34-158 lines (easy to navigate)
- **Testable**: Can test handlers and business logic independently
- **Scalable**: Easy to add new capabilities without bloating files

### Core Framework Pattern

```go
// 1. Create a tool (passive component) - weather_tool.go
tool := core.NewTool("weather-service")

// 2. Register capabilities - weather_tool.go
tool.RegisterCapability(core.Capability{
    Name:        "current_weather",
    Description: "Gets current weather conditions",
    Handler:     handleCurrentWeather, // Defined in handlers.go
    // Endpoint auto-generates as: /api/capabilities/current_weather
})

// 3. Framework initialization - main.go
framework, _ := core.NewFramework(tool,
    core.WithRedisURL("redis://localhost:6379"),
    core.WithDiscovery(true), // Tools can register but not discover
    core.WithCORS([]string{"*"}, true),
)

// 4. Run (framework auto-injects Registry, Logger, Memory)
framework.Run(context.Background())
```

### Capability Patterns

The example demonstrates three capability patterns:

#### 1. Auto-Generated Endpoint (Most Common)
```go
tool.RegisterCapability(core.Capability{
    Name:        "current_weather",
    Description: "Gets current weather",
    Handler:     w.handleCurrentWeather,
    // No Endpoint specified = auto-generates /api/capabilities/current_weather
})
```

#### 2. Custom Endpoint
```go
tool.RegisterCapability(core.Capability{
    Name:        "alerts",
    Description: "Weather alerts",
    Endpoint:    "/weather/alerts", // Custom endpoint
    Handler:     w.handleAlerts,
})
```

#### 3. Generic Handler (Prototyping)
```go
tool.RegisterCapability(core.Capability{
    Name:        "historical_analysis",
    Description: "Historical weather analysis",
    // No Handler = framework provides generic response
})
```

### Framework Auto-Injection

The framework automatically injects dependencies:

```go
func (w *WeatherTool) handleCurrentWeather(rw http.ResponseWriter, r *http.Request) {
    // Logger auto-injected
    w.Logger.Info("Processing request", map[string]interface{}{
        "method": r.Method,
    })
    
    // Memory auto-injected for caching
    w.Memory.Set(r.Context(), "cache-key", data, 5*time.Minute)
    
    // Registry auto-injected (for tools: registration only)
    // w.Registry.Register() - tools can register themselves
    // w.Discovery.Discover() - NOT AVAILABLE (tools are passive)
}
```

## 🔧 Configuration Options

### Environment Variables (v0.6.4)

All configuration is now environment-based. See [.env.example](.env.example) for the complete reference.

#### Core Configuration (Required)

```bash
# Server port (default: 8080)
PORT=8080

# Kubernetes namespace for service discovery
NAMESPACE=gomind-examples

# Discovery backend URL (REQUIRED)
REDIS_URL=redis://redis.default.svc.cluster.local:6379

# Development mode (enables verbose logging)
DEV_MODE=false
```

#### Application Configuration (Optional)

```bash
# Weather API key for real data (optional - uses mock data if not set)
WEATHER_API_KEY=your-key-from-weatherapi.com
```

#### Logging Configuration (Optional)

```bash
# Log level: debug, info, warn, error (default: info)
GOMIND_LOG_LEVEL=info

# Log format: json, text (default: json)
GOMIND_LOG_FORMAT=json
```

#### Kubernetes-Specific (Optional)

```bash
# Service name for K8s service discovery
GOMIND_K8S_SERVICE_NAME=weather-service

# Pod IP (auto-detected in K8s)
GOMIND_POD_IP=
```

#### Telemetry Configuration (Optional)

```bash
# OpenTelemetry collector endpoint
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector.observability.svc.cluster.local:4317

# Service name for traces
OTEL_SERVICE_NAME=weather-service

# Trace sampling (1.0 = 100%)
OTEL_TRACE_SAMPLING=1.0
```

#### Phase 3 Schema Validation (Optional)

```bash
# Enable JSON Schema validation before tool calls
GOMIND_VALIDATE_PAYLOADS=true

# Schema cache TTL in seconds (default: 300)
GOMIND_SCHEMA_CACHE_TTL=300
```

### Configuration Validation

The tool validates all required configuration at startup:

```go
func validateConfig() error {
    // REDIS_URL is required
    redisURL := os.Getenv("REDIS_URL")
    if redisURL == "" {
        return fmt.Errorf("REDIS_URL environment variable required")
    }

    // Validate Redis URL format
    if !strings.HasPrefix(redisURL, "redis://") && !strings.HasPrefix(redisURL, "rediss://") {
        return fmt.Errorf("invalid REDIS_URL format (must start with redis:// or rediss://)")
    }

    // Validate port if set
    if portStr := os.Getenv("PORT"); portStr != "" {
        if _, err := strconv.Atoi(portStr); err != nil {
            return fmt.Errorf("invalid PORT value: %v", err)
        }
    }

    return nil
}
```

### Framework Options (Code-Based)

```go
framework, _ := core.NewFramework(tool,
    // Basic configuration from environment
    core.WithName("weather-service"),
    core.WithPort(port),  // From PORT env var
    core.WithNamespace(os.Getenv("NAMESPACE")),

    // Discovery (tools register themselves)
    core.WithRedisURL(os.Getenv("REDIS_URL")),
    core.WithDiscovery(true, "redis"),

    // CORS for web access
    core.WithCORS([]string{"*"}, true),

    // Development features
    core.WithDevelopmentMode(os.Getenv("DEV_MODE") == "true"),
)
```

## 🐳 Docker Usage

### Build Image (Automated via Makefile)

The Makefile handles Docker builds automatically:

```bash
# Build is triggered automatically by 'make deploy'
make build  # Standalone build - fetches v0.6.4 from GitHub
```

Manual build (if needed):

```bash
# Standalone build from tool-example directory
cd examples/tool-example
docker build -t weather-tool:latest .

# This fetches gomind v0.6.4 from GitHub - no workspace needed
```

### Run Container Locally

```bash
docker run -p 8080:8080 \
  -e PORT=8080 \
  -e REDIS_URL=redis://host.docker.internal:6379 \
  -e NAMESPACE=local \
  -e DEV_MODE=true \
  -e WEATHER_API_KEY=your-key \
  weather-tool:latest
```

Note: The Dockerfile uses multi-stage builds and supports dynamic PORT configuration via environment variable.

## ☸️ Kubernetes Deployment

### Automated Deployment (Recommended)

Use the Makefile for complete automation:

```bash
# Full setup: cluster + Redis + secrets + deployment
make all

# Or step by step:
make setup    # Create cluster, Redis, secrets
make deploy   # Build, load, deploy tool
make test     # Run automated tests
make status   # Check deployment status
```

### Manual Deployment

If you prefer manual control:

```bash
# 1. Create Kind cluster
kind create cluster --name gomind-demo --config kind-config.yaml

# 2. Create namespace
kubectl create namespace gomind-examples

# 3. Deploy Redis
kubectl apply -f redis-deployment.yaml -n default

# 4. Create secrets (optional for real API key)
kubectl create secret generic weather-tool-secrets \
  --from-literal=WEATHER_API_KEY=your-key \
  -n gomind-examples

# 5. Build and load image (standalone - fetches v0.6.4 from GitHub)
cd examples/tool-example
docker build -t weather-tool:latest .
kind load docker-image weather-tool:latest --name gomind-demo

# 6. Deploy the tool
kubectl apply -f k8-deployment.yaml

# 7. Wait for deployment
kubectl wait --for=condition=available --timeout=120s \
  deployment/weather-tool -n gomind-examples
```

### Verify Deployment

```bash
# Check pod status
kubectl get pods -n gomind-examples -l app=weather-tool

# View logs
kubectl logs -n gomind-examples -l app=weather-tool --tail=50 -f

# Test the service via port forward
kubectl port-forward -n gomind-examples svc/weather-service 8080:80 &
curl http://localhost:8080/health

# Check service discovery (in Redis)
kubectl exec -it deployment/redis -n default -- redis-cli KEYS "gomind:services:*"
kubectl exec -it deployment/redis -n default -- redis-cli GET "gomind:services:weather-service"
```

### Cleanup

```bash
# Complete cleanup (deletes cluster)
make clean

# Or selective cleanup
kubectl delete -f k8-deployment.yaml
kubectl delete namespace gomind-examples
kind delete cluster --name gomind-demo
```

## 🧪 Testing & Verification

### Automated Testing (Recommended)

The Makefile includes comprehensive automated tests:

```bash
# Run all automated tests against deployed tool
make test

# This tests:
# 1. Health endpoint
# 2. Capabilities listing endpoint
# 3. Current weather capability (with payload)
# 4. Phase 3 schema endpoint (v0.6.4 feature)
```

Output example:
```
Testing weather tool endpoints...

1. Testing health endpoint...
{"status":"healthy","uptime":"5m","framework_version":"0.6.4"}

2. Testing capabilities endpoint...
[
  {
    "name": "current_weather",
    "description": "Gets current weather conditions for a location",
    ...
  }
]

3. Testing current weather endpoint...
{
  "location": "London",
  "temperature": 15.5,
  "condition": "partly cloudy"
}

4. Testing schema endpoint (Phase 3)...
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {...}
}

✓ Tests complete
```

### Manual Testing

```bash
# Port forward to access the tool
kubectl port-forward -n gomind-examples svc/weather-service 8080:80

# In another terminal:

# 1. Health check
curl -f http://localhost:8080/health

# 2. Capability discovery
curl http://localhost:8080/api/capabilities | jq '.'

# 3. Test current weather capability
curl -X POST http://localhost:8080/api/capabilities/current_weather \
  -H "Content-Type: application/json" \
  -d '{"location":"London","units":"metric"}' | jq '.'

# 4. Test forecast capability
curl -X POST http://localhost:8080/api/capabilities/forecast \
  -H "Content-Type: application/json" \
  -d '{"location":"San Francisco","days":5}' | jq '.'

# 5. Test Phase 3 schema endpoint (v0.6.4)
curl http://localhost:8080/api/capabilities/current_weather/schema | jq '.'
```

### Discovery Verification (requires Redis)

```bash
# Check Redis for service registration
kubectl exec -it deployment/redis -n default -- redis-cli KEYS "gomind:services:*"
kubectl exec -it deployment/redis -n default -- redis-cli GET "gomind:services:weather-service"

# Expected output shows registered capabilities and metadata
```

### Local Development Testing

```bash
# Test build without deployment
make dev-test

# This verifies the tool builds successfully without full deployment
```

### Load Testing

```bash
# Simple load test (with port-forward active)
for i in {1..100}; do
  curl -s -X POST http://localhost:8080/api/capabilities/current_weather \
    -H "Content-Type: application/json" \
    -d "{\"location\":\"City$i\"}" &
done
wait
echo "Load test complete"
```

### Debugging Failed Tests

```bash
# View recent logs
make logs

# Check deployment status
make status

# Full debug information
make debug

# This shows:
# - Pod status and events
# - Recent error logs
# - Resource status
```

## 📊 Monitoring & Observability

### Metrics (Prometheus)
- Component health: `up{job="gomind-tools"}`
- Request rate: `rate(http_requests_total[5m])`
- Error rate: `rate(http_requests_total{status=~"5.."}[5m])`

### Logs (Structured JSON)
```json
{
  "level": "info",
  "msg": "Processing current weather request",
  "method": "POST",
  "path": "/api/capabilities/current_weather",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Tracing (Jaeger)
- Service discovery registration
- HTTP request/response cycles
- Redis cache operations

## 🎨 Customization Guide

### File Organization Guide

Understanding where to make changes:

| Task | File to Modify | Example |
|------|---------------|---------|
| Add new capability | [weather_tool.go](weather_tool.go) | Add to `registerCapabilities()` |
| Implement capability handler | [handlers.go](handlers.go) | Add new `handleXXX()` function |
| Change data source/logic | [weather_data.go](weather_data.go) | Modify `simulateWeatherData()` |
| Add external API integration | [weather_data.go](weather_data.go) | Add `fetchRealWeatherData()` |
| Change port/config | [main.go](main.go) | Modify `main()` or `validateConfig()` |
| Add startup logging | [main.go](main.go) | Add to `main()` before `framework.Run()` |
| Modify shutdown behavior | [main.go](main.go) | Update graceful shutdown goroutine |

### Adding New Capabilities

**Step 1**: Register capability in [weather_tool.go](weather_tool.go):

```go
// Add to registerCapabilities() method in weather_tool.go
func (w *WeatherTool) registerCapabilities() {
    // ... existing capabilities ...

    // New capability
    w.RegisterCapability(core.Capability{
        Name:        "weather_map",
        Description: "Generates weather visualization map",
        InputTypes:  []string{"json"},
        OutputTypes: []string{"image/png", "application/json"},
        Handler:     w.handleWeatherMap, // Handler defined in handlers.go

        // Phase 2: Add field hints for AI accuracy
        InputSummary: &core.SchemaSummary{
            RequiredFields: []core.FieldHint{
                {Name: "location", Type: "string", Example: "London"},
            },
        },
    })
}
```

**Step 2**: Implement handler in [handlers.go](handlers.go):

```go
// Add to handlers.go
func (w *WeatherTool) handleWeatherMap(rw http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // 1. Parse request
    var req WeatherRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(rw, "Invalid request", http.StatusBadRequest)
        return
    }

    // 2. Call business logic (from weather_data.go)
    mapData := w.generateWeatherMap(req.Location)

    // 3. Return response
    rw.Header().Set("Content-Type", "application/json")
    json.NewEncoder(rw).Encode(mapData)
}
```

**Step 3**: (Optional) Add business logic to [weather_data.go](weather_data.go):

```go
// Add to weather_data.go if you need custom data processing
func (w *WeatherTool) generateWeatherMap(location string) map[string]interface{} {
    // Your map generation logic here
    return map[string]interface{}{
        "location": location,
        "map_url": "https://example.com/map.png",
    }
}
```

### AI Coding Assistant Prompts

The decomposed structure makes it easier to work with AI assistants. Ask clear, file-specific questions:

```
"Add a 'severe_weather_warnings' capability to weather_tool.go"

"Implement the handler for severe_weather_warnings in handlers.go"

"Update weather_data.go to fetch real weather data from weatherapi.com"

"Add authentication middleware in main.go before framework.Run()"

"Help me add OpenTelemetry tracing to the handlers in handlers.go"

"Refactor weather_data.go to support multiple weather API providers"
```

**Pro Tip**: When working with AI assistants, specify the file name in your request for more accurate and focused responses.

### Integration with Existing Systems

```go
// Add custom middleware
func customAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Your auth logic
        next.ServeHTTP(w, r)
    })
}

// Apply to framework
framework.AddMiddleware(customAuth)
```

## 🚨 Common Issues & Solutions

### Issue: Configuration error at startup

**Symptom**: Tool fails with "Configuration error: REDIS_URL environment variable required"

**Solution**:
```bash
# Check if .env file exists
ls -la .env

# If missing, copy from example
cp .env.example .env

# Ensure REDIS_URL is set correctly
grep REDIS_URL .env

# For Kind cluster, use:
REDIS_URL=redis://redis.default.svc.cluster.local:6379
```

### Issue: Tool not appearing in discovery

**Symptom**: Agent can't find weather-service

**Solution**:
```bash
# 1. Check Redis is running
kubectl get pods -n default | grep redis

# 2. Check tool logs for connection errors
make logs
# or
kubectl logs -n gomind-examples -l app=weather-tool --tail=50

# 3. Verify Redis connection from tool pod
kubectl exec -n gomind-examples deployment/weather-tool -- \
  sh -c 'redis-cli -u $REDIS_URL ping'

# 4. Check service registration in Redis
kubectl exec -it deployment/redis -n default -- \
  redis-cli KEYS "gomind:services:*"

# 5. Check REDIS_URL configuration
kubectl get deployment weather-tool -n gomind-examples -o yaml | grep REDIS_URL
```

### Issue: Deployment fails or pods crash

**Symptom**: `make deploy` fails or pods show CrashLoopBackOff

**Solution**:
```bash
# 1. Check full debug information
make debug

# 2. Check pod events
kubectl describe pod -n gomind-examples -l app=weather-tool

# 3. Check logs for startup errors
kubectl logs -n gomind-examples -l app=weather-tool --tail=100

# 4. Common causes:
# - Invalid REDIS_URL format (must start with redis:// or rediss://)
# - Redis not running (check: kubectl get pods -n default | grep redis)
# - Port conflicts (check k8-deployment.yaml)
# - Image not loaded to Kind (rerun: make load-image)
```

### Issue: CORS errors when calling from browser

**Symptom**: Browser console shows CORS policy errors

**Solution**:
```go
// Framework enables CORS by default
// Check main.go:
core.WithCORS([]string{"*"}, true),  // Allows all origins

// For production, restrict origins:
core.WithCORS([]string{"https://your-app.com"}, false),
```

### Issue: Port conflicts or wrong port

**Symptom**: Service not accessible on expected port

**Solution**:
```bash
# 1. Check PORT environment variable
kubectl get deployment weather-tool -n gomind-examples -o yaml | grep PORT

# 2. Change port in .env file
echo "PORT=8081" >> .env

# 3. Redeploy
make deploy

# 4. Update port-forward command
kubectl port-forward -n gomind-examples svc/weather-service 8081:80
```

### Issue: Weather API returns mock data

**Symptom**: All weather responses show mock data

**Solution**:
```bash
# 1. This is expected if WEATHER_API_KEY is not set
# The tool works with mock data by default

# 2. To use real weather data:
# - Get API key from https://www.weatherapi.com/
# - Add to .env file:
echo "WEATHER_API_KEY=your-actual-key-here" >> .env

# 3. Update secret in cluster
source .env
make create-secrets

# 4. Restart deployment
kubectl rollout restart deployment/weather-tool -n gomind-examples
kubectl rollout status deployment/weather-tool -n gomind-examples
```

### Issue: Build fails with module errors

**Symptom**: `go build` or Docker build fails with "module not found"

**Solution**:
```bash
# 1. Build standalone from tool-example directory
cd examples/tool-example
docker build -t weather-tool:latest .
# This fetches gomind v0.6.4 from GitHub - no workspace needed

# 2. Or use Makefile which handles this automatically
make build

# 3. Check go.mod uses correct version
grep "github.com/itsneelabh/gomind/core" go.mod
# Should show: v0.6.4
```

### Issue: Schema endpoint returns 404

**Symptom**: Phase 3 schema endpoint not found

**Solution**:
```bash
# 1. Schema endpoint requires v0.6.4+
# Check framework version in logs:
make logs | grep "Framework version"

# 2. Update to v0.6.4 if needed
grep "github.com/itsneelabh/gomind/core" go.mod
# Should show: v0.6.4

# 3. Correct endpoint format:
curl http://localhost:8080/api/capabilities/current_weather/schema
#                                            ^^^^^^^^^^^^^^^^ (capability name)
```

## 📋 Redis Service Registry Example

When the weather-tool is deployed and running, it automatically registers itself in Redis. Here's what the service entry looks like:

```json
{
  "name": "weather-service",
  "type": "tool",
  "address": "weather-tool-service.gomind-examples.svc.cluster.local",
  "port": 80,
  "namespace": "gomind-examples",
  "capabilities": [
    {
      "name": "current_weather",
      "description": "Gets current weather conditions for a specified location with temperature, humidity, wind speed, and conditions",
      "endpoint": "/api/capabilities/current_weather",
      "input_types": ["application/json"],
      "output_types": ["application/json"],
      "input_summary": {
        "required_fields": [
          {
            "name": "location",
            "type": "string",
            "example": "New York",
            "description": "City name or coordinates"
          }
        ],
        "optional_fields": [
          {
            "name": "units",
            "type": "string",
            "example": "metric",
            "description": "Temperature units (metric/imperial)"
          }
        ]
      }
    },
    {
      "name": "forecast",
      "description": "Provides multi-day weather forecast with daily predictions",
      "endpoint": "/api/capabilities/forecast",
      "input_types": ["application/json"],
      "output_types": ["application/json"],
      "input_summary": {
        "required_fields": [
          {
            "name": "location",
            "type": "string",
            "example": "London",
            "description": "City name or coordinates"
          }
        ],
        "optional_fields": [
          {
            "name": "days",
            "type": "integer",
            "example": 5,
            "description": "Number of forecast days (1-7)"
          }
        ]
      }
    },
    {
      "name": "alerts",
      "description": "Gets active weather alerts and warnings for a location",
      "endpoint": "/weather/alerts",
      "input_types": ["application/json"],
      "output_types": ["application/json"],
      "input_summary": {
        "required_fields": [
          {
            "name": "location",
            "type": "string",
            "example": "Miami",
            "description": "City name or coordinates"
          }
        ]
      }
    },
    {
      "name": "historical_analysis",
      "description": "Analyzes historical weather patterns and trends",
      "endpoint": "/api/capabilities/historical_analysis",
      "input_types": ["application/json"],
      "output_types": ["application/json"]
    }
  ],
  "metadata": {
    "version": "v1.0.0",
    "framework_version": "0.6.4",
    "discovery_enabled": true,
    "last_heartbeat": "2025-11-10T05:15:23Z"
  },
  "health_endpoint": "/health",
  "registered_at": "2025-11-10T04:45:15Z",
  "last_seen": "2025-11-10T05:15:23Z",
  "ttl": 30
}
```

### Understanding the Registry Data

**Key Fields:**
- **name**: Service identifier used for discovery (`weather-service`)
- **type**: Component type - `tool` (passive) vs `agent` (active)
- **address**: Kubernetes service DNS name for cross-namespace communication
- **port**: Service port (Kubernetes Service port 80, maps to container port 8080)
- **capabilities**: Array of all registered capabilities with Phase 2 field hints
- **input_summary**: Phase 2 enhancement - provides field hints for 95% AI accuracy
- **ttl**: Time-to-live in seconds (30s) - refreshed every 15s by heartbeat
- **metadata**: Framework version, discovery status, last heartbeat timestamp

**Service Discovery Pattern:**
- Both pod replicas send heartbeats to the same Redis key: `gomind:services:weather-service`
- Kubernetes Service (`weather-tool-service`) load-balances traffic across pods
- Agents discover one service entry, Kubernetes handles pod-level routing
- Heartbeat keeps TTL fresh - service auto-expires if pods stop

**Redis Index Structure:**
```
gomind:services:weather-service          → Full service data (30s TTL)
gomind:types:tool                        → Set of all tools (60s TTL)
gomind:names:weather-service             → Name index (60s TTL)
gomind:capabilities:current_weather      → Capability index (60s TTL)
gomind:capabilities:forecast             → Capability index (60s TTL)
gomind:capabilities:alerts               → Capability index (60s TTL)
gomind:capabilities:historical_analysis  → Capability index (60s TTL)
```

You can inspect this data in your cluster:
```bash
# Get the full service entry
kubectl exec -it deployment/redis -n default -- \
  redis-cli GET "gomind:services:weather-service"

# List all service keys
kubectl exec -it deployment/redis -n default -- \
  redis-cli KEYS "gomind:services:*"

# See all tools
kubectl exec -it deployment/redis -n default -- \
  redis-cli SMEMBERS "gomind:types:tool"
```

## 📚 Next Steps

### 1. Test with Agent Example

Deploy both examples together to see end-to-end orchestration:

```bash
# Tool is already deployed from this example
cd ../agent-example

# Deploy agent (reuses existing cluster and Redis)
make deploy

# Agent will automatically discover and use weather-tool
make test
```

See [Agent Example](../agent-example/README.md) for details on how agents discover and orchestrate this tool.

### 2. Implement Phase 3 Validation

Enable JSON Schema validation for production:

```bash
# Add to .env
echo "GOMIND_VALIDATE_PAYLOADS=true" >> .env

# Redeploy
make deploy

# All tool calls will now be validated against schemas
# Invalid payloads will be rejected with clear error messages
```

### 3. Add Real Weather API

Replace mock data with real weather information:

```bash
# 1. Get API key from https://www.weatherapi.com/
# 2. Add to .env
echo "WEATHER_API_KEY=your-actual-key" >> .env

# 3. Update secret and restart
source .env && make create-secrets
kubectl rollout restart deployment/weather-tool -n gomind-examples
```

### 4. Customize for Your Domain

Adapt this example for your own tools by following the same 4-file structure:

**Step 1**: Update [weather_tool.go](weather_tool.go) → `your_domain_tool.go`:
```go
// Rename WeatherTool to YourDomainTool
type YourDomainTool struct {
    *core.BaseTool
    // Your fields
}

// Update constructor
func NewYourDomainTool() *YourDomainTool {
    tool := &YourDomainTool{
        BaseTool: core.NewTool("your-service-name"),
    }
    tool.registerCapabilities()
    return tool
}

// Register your capabilities
func (t *YourDomainTool) registerCapabilities() {
    t.RegisterCapability(core.Capability{
        Name:        "your_capability",
        Description: "What your capability does",
        Handler:     t.handleYourCapability,
        // Include Phase 2 field hints for 95% AI accuracy
        InputSummary: &core.SchemaSummary{
            RequiredFields: []core.FieldHint{...},
        },
    })
}
```

**Step 2**: Update [handlers.go](handlers.go):
```go
// Implement your handlers
func (t *YourDomainTool) handleYourCapability(rw http.ResponseWriter, r *http.Request) {
    // Your handler implementation
}
```

**Step 3**: Update [weather_data.go](weather_data.go) → `your_domain_data.go`:
```go
// Your business logic and data operations
func (t *YourDomainTool) processYourData(input string) YourResponse {
    // Your logic here
}
```

**Step 4**: Update [main.go](main.go):
```go
func main() {
    // Change tool initialization
    tool := NewYourDomainTool()

    framework, _ := core.NewFramework(tool,
        core.WithName("your-service-name"),
        // ... rest stays the same
    )
}
```

The decomposed structure makes it easy to understand and modify each aspect independently!

### 5. Production Deployment

For production use:

- Review [k8-deployment.yaml](k8-deployment.yaml) for resource limits
- Set up proper secrets management (e.g., Sealed Secrets, Vault)
- Enable TLS/mTLS for service communication
- Configure production Redis with persistence
- Set up monitoring with Prometheus/Grafana (see [k8-deployment/OBSERVABILITY.md](../k8-deployment/OBSERVABILITY.md))
- Use `DEV_MODE=false` for JSON-formatted logs

## 🎓 Key Learnings

### Architecture Patterns

- **Tools are Passive**: They register capabilities but cannot discover others
- **Framework Handles Everything**: Auto-injection, discovery, health checks, endpoints
- **Environment-Based Config**: All configuration from env vars (v0.6.4)
- **3-Phase Discovery**: Description → Field Hints → Schema validation
- **Decomposed Structure**: Organized into 4 files for better maintainability

### File Organization Benefits

The 4-file structure provides clear benefits:

1. **[main.go](main.go)**: Entry point only
   - Easy to understand application startup
   - Configuration validation in one place
   - Graceful shutdown logic isolated

2. **[weather_tool.go](weather_tool.go)**: Component definition
   - All capabilities registered in one file
   - Type definitions co-located with usage
   - Clear API surface for the tool

3. **[handlers.go](handlers.go)**: HTTP layer
   - Request/response handling separated
   - Easy to add middleware or validation
   - Testable HTTP handlers

4. **[weather_data.go](weather_data.go)**: Business logic
   - Data generation/API calls isolated
   - Easy to swap implementations
   - No HTTP concerns mixed in

### v0.6.4 Features

- **Phase 2 Field Hints**: 95% AI accuracy for payload generation
- **Phase 3 Schema Validation**: 99% accuracy with Redis caching
- **Configuration Validation**: Fail-fast on startup with clear errors
- **Graceful Shutdown**: Proper signal handling for K8s
- **Makefile Automation**: Complete local development workflow

### Production Patterns

- **Structured Logging**: Framework provides context-aware JSON logs for debugging
- **Health Checks**: Built-in readiness/liveness endpoints
- **Discovery Integration**: Automatic Redis registration and heartbeat
- **CORS Support**: Browser-friendly API access
- **Multi-Stage Builds**: Optimized Docker images (~10MB)

### Development Workflow

- **Local Kind Cluster**: Full K8s environment in minutes with `make all`
- **Automated Testing**: Comprehensive endpoint tests with `make test`
- **Live Debugging**: Real-time logs with `make logs` and `make debug`
- **Quick Iteration**: Rebuild and redeploy with single `make deploy`

This tool showcases GoMind v0.6.4 best practices and can be discovered and orchestrated by agents in your system!

---

**Ready to see it in action?** Deploy the [Agent Example](../agent-example/README.md) to watch intelligent agents discover and orchestrate this tool automatically.