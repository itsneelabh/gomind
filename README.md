# GoMind - Kubernetes-Native AI Agent Framework

[![Go Version](https://img.shields.io/badge/go-1.23+-blue.svg)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

GoMind is a lightweight framework for building AI agents that run efficiently on Kubernetes. With an 8MB core and native K8s integration, it's designed for enterprises that need to deploy AI at scale using their existing infrastructure.

## Why GoMind?

**The Problem**: You have Kubernetes. You need AI agents. Most frameworks require 500MB+ containers, complex service meshes, and dedicated infrastructure.

**The Solution**: GoMind gives you 8MB agents that run as regular pods, use standard K8s services, and scale with HPA. No special operators, no CRDs, no complexity.

## Key Features

### 🎯 Kubernetes-Native from Day One
- **8MB containers** vs 500MB+ for Python frameworks
- Works with standard K8s services, ConfigMaps, and Secrets
- Built-in health checks and graceful shutdown
- Scales with HorizontalPodAutoscaler
- No custom operators or CRDs required

### 🚀 Production-Ready Architecture
```go
// Service discovery built-in
agent := framework.NewBaseAgent("pricing-service")
framework.RunAgent(agent, 8080)  // Automatically registers with Redis
```

- Redis-based service discovery (works with ElastiCache/MemoryStore)
- HTTP/JSON communication (no gRPC complexity)
- Distributed tracing with OpenTelemetry
- Circuit breakers and retry logic included

### 📦 Modular Design
Start small, grow as needed:
- **Core (8MB)**: Service discovery, HTTP server, basic orchestration
- **AI Module (+2MB)**: OpenAI/Anthropic integration, prompt management
- **Telemetry (+10MB)**: Full OpenTelemetry with Jaeger/Datadog support

## Quick Start

### 1. Create an Agent
```go
package main

import (
    "context"
    framework "github.com/itsneelabh/gomind"
)

type AnalyticsAgent struct {
    framework.BaseAgent
}

// Auto-discovered capability
// @capability: analyze_metrics
func (a *AnalyticsAgent) AnalyzeMetrics(ctx context.Context, data []float64) (string, error) {
    // Your logic here
    return "Analysis complete", nil
}

func main() {
    agent := &AnalyticsAgent{}
    framework.RunAgent(agent, 8080)
}
```

### 2. Deploy to Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: analytics-agent
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: agent
        image: mycompany/analytics-agent:latest
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"
        env:
        - name: REDIS_URL
          value: "redis://redis:6379"
```

### 3. Scale with HPA
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: analytics-agent-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: analytics-agent
  minReplicas: 2
  maxReplicas: 100
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

## Real-World Example: Multi-Agent System

```go
// Market data service (deterministic, 8MB)
type MarketDataService struct {
    framework.BaseAgent
}

// Risk analyzer (AI-powered, 10MB)  
type RiskAnalyzer struct {
    framework.BaseAgent
    ai framework.AIClient
}

// Orchestrator coordinates both
orchestrator := framework.NewOrchestrator()
result := orchestrator.Execute(ctx, 
    "Get market data for AAPL and analyze risk")
```

## Production Patterns

### Service Discovery
```go
// Agents find each other automatically
discovery := framework.NewRedisDiscovery(redisURL)
agents := discovery.FindByCapability("analyze_risk")
```

### Circuit Breakers
```go
// Prevent cascade failures
response, err := framework.CallWithCircuitBreaker(
    func() (interface{}, error) {
        return agent.CallRemoteAgent(ctx, "expensive-operation")
    },
)
```

### Distributed Tracing
```go
// Traces flow across all agents automatically
ctx, span := tracer.Start(ctx, "ProcessOrder")
defer span.End()
// All downstream agent calls are traced
```

## Performance on Kubernetes

| Metric | GoMind | Traditional Frameworks |
|--------|--------|----------------------|
| Container Size | 8-20MB | 500MB+ |
| Memory Usage | 10-30MB | 200MB+ |
| Cold Start | <100ms | 2-10s |
| Pods per Node* | 100-200 | 10-20 |
| HPA Scale Time | <10s | 30-60s |

*Based on 2GB node memory

## Enterprise Integration

### Works with Your Stack
- **Databases**: PostgreSQL, MongoDB, DynamoDB
- **Message Queues**: Kafka, RabbitMQ, SQS
- **Cache**: Redis, Memcached
- **Observability**: Prometheus, Grafana, Datadog
- **AI Providers**: OpenAI, Anthropic, Bedrock, Vertex AI

### Security & Compliance
- No external dependencies in core module
- Supports private endpoints and VPC peering
- Works with K8s NetworkPolicies
- Compatible with service meshes (Istio, Linkerd)
- Audit logging built-in

## Use Cases

### Financial Services
- **Trading Bots**: Sub-millisecond latency with Go's performance
- **Risk Analysis**: Orchestrate multiple specialized agents
- **Fraud Detection**: Scale to handle transaction spikes

### Healthcare
- **Patient Routing**: HIPAA-compliant agent communication
- **Diagnostic Assistance**: Coordinate specialist AI models
- **Resource Optimization**: Efficient scheduling agents

### E-Commerce
- **Dynamic Pricing**: Real-time price adjustments
- **Inventory Management**: Distributed decision making
- **Customer Service**: Scalable chat agents

## Getting Started

```bash
# Install the framework
go get github.com/itsneelabh/gomind/core

# Run locally
go run main.go

# Build container (multi-stage for tiny images)
docker build -t myagent:latest .

# Deploy to Kubernetes
kubectl apply -f deployment.yaml
```

## Documentation

- [Quick Start Guide](docs/GETTING_STARTED.md)
- [Framework Capabilities](docs/framework_capabilities_guide.md)
- [Kubernetes Deployment](docs/k8s-service-fronted-discovery.md)
- [API Reference](https://pkg.go.dev/github.com/itsneelabh/gomind)

## Architecture

```
┌─────────────────────────────────────────────┐
│            Kubernetes Cluster                │
├─────────────────────────────────────────────┤
│                                             │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐   │
│  │ Agent A │  │ Agent B │  │ Agent C │   │
│  │  (8MB)  │  │  (10MB) │  │  (8MB)  │   │
│  └────┬────┘  └────┬────┘  └────┬────┘   │
│       │            │            │          │
│       └────────────┼────────────┘          │
│                    │                       │
│            ┌───────▼────────┐              │
│            │     Redis      │              │
│            │Service Registry│              │
│            └────────────────┘              │
│                                             │
│  ┌──────────────────────────────────────┐  │
│  │        Horizontal Pod Autoscaler      │  │
│  └──────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
```

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/itsneelabh/gomind/issues)
- **Discussions**: [GitHub Discussions](https://github.com/itsneelabh/gomind/discussions)
- **Security**: [SECURITY.md](SECURITY.md)

---

Built for developers who need production-ready AI agents on Kubernetes. No hype, just solid engineering.