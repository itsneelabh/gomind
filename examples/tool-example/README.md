# Weather Tool Example

A comprehensive example demonstrating how to build a **Tool** (passive component) using the GoMind framework. This tool provides weather-related capabilities and showcases the framework's auto-discovery, capability registration, and production-ready patterns.

## 🎯 What This Example Demonstrates

- **Tool Pattern**: Passive component that registers capabilities but cannot discover other components
- **Framework Integration**: Auto-dependency injection, Redis discovery, health checks
- **Multiple Capabilities**: Different endpoint patterns and handler types
- **Production Patterns**: Error handling, logging, caching, metrics
- **Kubernetes Deployment**: Complete K8s configuration with monitoring

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

## 🚀 Quick Start (5 minutes)

### Prerequisites
- Go 1.25+
- Docker & Docker Compose
- Redis (or use Docker Compose)

### 1. Clone and Run Locally

```bash
# Navigate to the tool example
cd examples/tool-example

# Set up environment (optional)
export WEATHER_API_KEY="your-api-key-here"

# Option A: Run with Docker Compose (includes Redis)
cd ../
docker-compose up weather-tool redis

# Option B: Run directly (requires Redis running)
cd tool-example
go mod tidy
go run main.go
```

### 2. Test the Tool

```bash
# Health check
curl http://localhost:8080/health

# List all capabilities
curl http://localhost:8080/api/capabilities

# Test current weather capability
curl -X POST http://localhost:8080/api/capabilities/current_weather \
  -H "Content-Type: application/json" \
  -d '{"location":"New York","units":"metric"}'

# Test forecast capability
curl -X POST http://localhost:8080/api/capabilities/forecast \
  -H "Content-Type: application/json" \
  -d '{"location":"San Francisco","days":5}'

# Test custom endpoint
curl "http://localhost:8080/weather/alerts?location=Miami"
```

### 3. Expected Response

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

## 📊 Understanding the Code

### Core Framework Pattern

```go
// 1. Create a tool (passive component)
tool := core.NewTool("weather-service")

// 2. Register capabilities
tool.RegisterCapability(core.Capability{
    Name:        "current_weather",
    Description: "Gets current weather conditions",
    Handler:     handleCurrentWeather,
    // Endpoint auto-generates as: /api/capabilities/current_weather
})

// 3. Framework handles everything else
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

### Environment Variables

```bash
# Core Configuration
export GOMIND_AGENT_NAME="weather-service"
export GOMIND_PORT=8080
export GOMIND_NAMESPACE="examples"

# Discovery Configuration
export REDIS_URL="redis://localhost:6379"
export GOMIND_REDIS_URL="redis://localhost:6379"

# Application Configuration
export WEATHER_API_KEY="your-api-key"

# Development
export GOMIND_DEV_MODE=true
```

### Framework Options

```go
framework, _ := core.NewFramework(tool,
    // Basic configuration
    core.WithName("weather-service"),
    core.WithPort(8080),
    core.WithNamespace("examples"),
    
    // Discovery (tools register themselves)
    core.WithRedisURL("redis://localhost:6379"),
    core.WithDiscovery(true, "redis"),
    
    // CORS for web access
    core.WithCORS([]string{"*"}, true),
    
    // Development features
    core.WithDevelopmentMode(true),
)
```

## 🐳 Docker Usage

### Build Image
```bash
docker build -t weather-tool:latest .
```

### Run Container
```bash
docker run -p 8080:8080 \
  -e REDIS_URL=redis://host.docker.internal:6379 \
  -e WEATHER_API_KEY=your-key \
  weather-tool:latest
```

### With Docker Compose
```bash
# From examples/ directory
docker-compose up weather-tool redis
```

## ☸️ Kubernetes Deployment

### Quick Deploy

```bash
# From examples/ directory

# 1. Deploy infrastructure
kubectl apply -f k8-deployment/namespace.yaml
kubectl apply -f k8-deployment/redis.yaml

# 2. Create API key secret (optional)
kubectl create secret generic api-keys \
  --from-literal=weather-api-key=your-api-key \
  -n gomind-examples

# 3. Deploy the tool
kubectl apply -f tool-example/k8-deployment.yaml

# 4. Check status
kubectl get pods -n gomind-examples
kubectl logs -f deployment/weather-tool -n gomind-examples
```

### Complete Stack (with monitoring)

```bash
# Deploy everything with Kustomize
kubectl apply -k k8-deployment/

# Port forward to access services
kubectl port-forward svc/weather-tool-service 8080:80 -n gomind-examples
```

### Verify Deployment

```bash
# Check pod status
kubectl get pods -n gomind-examples -l app=weather-tool

# Test the service
kubectl port-forward svc/weather-tool-service 8080:80 -n gomind-examples
curl http://localhost:8080/health

# Check service discovery (in Redis)
kubectl exec -it deployment/redis -n gomind-examples -- redis-cli KEYS "gomind:*"
```

## 🧪 Testing & Verification

### Basic Functionality

```bash
# 1. Health check
curl -f http://localhost:8080/health
# Expected: {"status":"healthy",...}

# 2. Capability discovery
curl http://localhost:8080/api/capabilities
# Expected: Array of capability definitions

# 3. Test each capability
curl -X POST http://localhost:8080/api/capabilities/current_weather \
  -H "Content-Type: application/json" \
  -d '{"location":"London","units":"metric"}'
```

### Discovery Verification (requires Redis)

```bash
# Check Redis for service registration
redis-cli KEYS "gomind:services:*"
redis-cli GET "gomind:services:weather-service-*"
```

### Load Testing

```bash
# Simple load test
for i in {1..100}; do
  curl -X POST http://localhost:8080/api/capabilities/current_weather \
    -H "Content-Type: application/json" \
    -d "{\"location\":\"City$i\"}" &
done
wait
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

### Adding New Capabilities

```go
// Add to registerCapabilities() method
tool.RegisterCapability(core.Capability{
    Name:        "weather_map",
    Description: "Generates weather visualization map",
    InputTypes:  []string{"json"},
    OutputTypes: []string{"image/png", "application/json"},
    Handler:     w.handleWeatherMap,
})

// Implement handler
func (w *WeatherTool) handleWeatherMap(rw http.ResponseWriter, r *http.Request) {
    // Your implementation here
}
```

### AI Coding Assistant Prompts

Ask your AI assistant:

```
"Help me add a new 'severe_weather_warnings' capability to this weather tool"

"Convert this weather tool to handle financial data instead"

"Add authentication middleware to this GoMind tool"

"Help me add OpenTelemetry tracing to this tool's handlers"
```

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

### Issue: Tool not appearing in discovery
```bash
# Check Redis connection
redis-cli -u $REDIS_URL ping

# Check tool logs
kubectl logs deployment/weather-tool -n gomind-examples

# Verify registration
redis-cli KEYS "gomind:services:*"
```

### Issue: CORS errors
```go
// Enable CORS in framework
core.WithCORS([]string{"*"}, true),

// Or configure specific origins
core.WithCORS([]string{"https://your-app.com"}, false),
```

### Issue: Port conflicts
```bash
# Check what's using the port
lsof -i :8080

# Use different port
export GOMIND_PORT=8081
```

## 📚 Next Steps

1. **Try the Agent Example**: See how agents discover and orchestrate this tool
2. **Add AI Capabilities**: Enhance with AI-powered analysis
3. **Production Deployment**: Use the K8s configs for production
4. **Custom Integration**: Integrate with your existing weather APIs
5. **Monitoring**: Set up the full observability stack

## 🎓 Key Learnings

- **Tools are Passive**: They register capabilities but cannot discover others
- **Framework Handles Everything**: Auto-injection, discovery, health checks
- **Multiple Patterns**: Auto-endpoints, custom endpoints, generic handlers  
- **Production Ready**: Built-in logging, caching, metrics, K8s support
- **AI Assistant Friendly**: Clear structure for easy modification

This tool can now be discovered and used by agents in your GoMind system!

---

**Next**: Check out the [Agent Example](../agent-example/README.md) to see how intelligent agents discover and orchestrate this tool.