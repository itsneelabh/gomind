# GoMind Framework Examples

Complete, production-ready examples demonstrating AI-enhanced distributed systems with the GoMind framework. Optimized for local development with Kind and cloud deployment on any Kubernetes platform.

## 📦 Available Examples

### 🎯 Quick Reference - Start Here

| Example | Pattern | Complexity | Best For | Time |
|---------|---------|------------|----------|------|
| **[tool-example](tool-example/)** | Tool (Passive) | ⭐ Beginner | Learning tool patterns, external APIs | 15 min |
| **[agent-example](agent-example/)** | Agent (Active) | ⭐⭐ Intermediate | Service discovery, coordination | 20 min |
| **[ai-tools-showcase](ai-tools-showcase/)** | Ready AI Tools | ⭐⭐ Intermediate | Adding AI to existing systems | 15 min |
| **[agent-example-enhanced](agent-example-enhanced/)** | AI Agent | ⭐⭐⭐ Advanced | AI-powered capabilities | 30 min |
| **[ai-agent-example](ai-agent-example/)** | AI-Native | ⭐⭐⭐⭐ Expert | AI-driven architecture | 45 min |
| **[ai-multi-provider](ai-multi-provider/)** | Resilient AI | ⭐⭐⭐⭐ Expert | Mission-critical AI systems | 60 min |
| **[orchestration-example](orchestration-example/)** | Orchestrator | ⭐⭐⭐⭐ Expert | Complex workflows | 45 min |
| **[workflow-example](workflow-example/)** | YAML Engine | ⭐⭐⭐ Advanced | Declarative workflows | 30 min |

### 🚀 Framework Patterns (Optional)

| Example | Focus | Best For | Time |
|---------|-------|----------|------|
| **[telemetry](telemetry/)** | Monitoring | Production observability | 20 min |
| **[context_propagation](context_propagation/)** | Tracing | Distributed system debugging | 15 min |
| **[error_handling](error_handling/)** | Error Patterns | Framework consistency | 10 min |

### 💡 Where Should I Start?

**👋 New to GoMind?** → `tool-example` then `agent-example`

**🤖 Want AI features?** → `ai-tools-showcase` (4 ready tools) or `agent-example-enhanced`

**🏢 Enterprise workflows?** → `orchestration-example` or `workflow-example`

**⚡ Production AI?** → `ai-multi-provider` (reliability) + `telemetry` (monitoring)

**🔧 Framework dev?** → Start with core examples, then framework patterns

### Infrastructure
| Component | Purpose | Features |
|-----------|---------|----------|
| **[k8-deployment/](k8-deployment/)** | Kubernetes deployment configs | Redis, Prometheus, Grafana, Jaeger, OTEL |

## 🚀 Quick Start

### Prerequisites

**Required:**
- [Docker](https://docs.docker.com/get-docker/) (20.10+)
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) (0.17+)
- [kubectl](https://kubernetes.io/docs/tasks/tools/) (1.25+)
- [Go](https://golang.org/dl/) (1.21+) - for building examples

**Verification:**
```bash
docker --version    # Docker 20.10+
kind --version      # kind 0.17+
kubectl version     # Client 1.25+
go version          # go1.21+
```

### One-Command Demo Setup

```bash
# Clone and setup
git clone https://github.com/itsneelabh/gomind.git
cd gomind/examples

# Setup API keys (interactive - optional but recommended)
./setup-api-keys.sh

# Create complete Kind cluster with monitoring stack
./setup-kind-demo.sh setup

# 🎉 Access services:
# Grafana:      http://localhost:3000 (admin/admin)
# Prometheus:   http://localhost:9090
# Jaeger:       http://localhost:16686
# Weather Tool: http://localhost:8080/health
# Agent:        http://localhost:8090/health
```

## 🧪 Test the System

Once the demo is running, test the complete tool → agent orchestration:

```bash
# 1. Test weather tool directly
curl -X POST http://localhost:8080/api/capabilities/current_weather \
  -H "Content-Type: application/json" \
  -d '{"location":"New York","units":"metric"}'

# 2. Test agent service discovery
curl http://localhost:8090/api/capabilities/discover_tools

# 3. Test intelligent orchestration (agent + AI + tools)
curl -X POST http://localhost:8090/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "current weather conditions in San Francisco",
    "use_ai": true,
    "max_results": 5
  }'
```

**Expected Flow:**
1. Agent receives research request
2. Agent discovers available tools (weather tool)
3. Agent extracts location and calls weather tool
4. Agent uses AI to analyze and synthesize results
5. Agent returns intelligent summary

## 🔧 Individual Example Usage

### Running Examples Locally

Each example can run independently:

```bash
# Terminal 1: Start Redis (required for service discovery)
docker run -p 6379:6379 redis:7-alpine

# Terminal 2: Run any tool
cd tool-example
go run main.go
# Tool starts on http://localhost:8080

# Terminal 3: Run any agent
cd agent-example
go run main.go
# Agent starts on http://localhost:8090
```

### Building Examples

Each example builds independently:

```bash
# Build any example
cd <example-directory>
go mod tidy
go build -o example-binary .
./example-binary

# Or with Docker
docker build -t <example-name>:latest .
```

## 🔑 API Key Configuration

### Automated Setup (Recommended)

```bash
# Interactive setup for local and Kubernetes
./setup-api-keys.sh
```

### Manual Setup

Create `.env` file in `examples/` directory:

```bash
# AI Providers (at least one recommended)
OPENAI_API_KEY=sk-your-openai-key
GROQ_API_KEY=gsk-your-groq-key        # Free tier available
ANTHROPIC_API_KEY=sk-ant-your-key
DEEPSEEK_API_KEY=your-deepseek-key
GOOGLE_AI_API_KEY=your-gemini-key

# External APIs (optional)
WEATHER_API_KEY=your-weather-api-key
```

The framework auto-detects available providers and uses the best one available.

## ☸️ Kubernetes Deployment

### Local Development (Kind)

```bash
# Complete automated setup
./setup-kind-demo.sh setup

# Manual access to services
kubectl port-forward -n gomind-examples svc/grafana 3000:80
kubectl port-forward -n gomind-examples svc/prometheus 9090:9090
kubectl port-forward -n gomind-examples svc/jaeger-query 16686:16686
```

### Cloud Deployment

The deployments are cloud-agnostic by default. For cloud-specific features:

1. **Read the deployment guide**: [CLOUD_DEPLOYMENT_GUIDE.md](CLOUD_DEPLOYMENT_GUIDE.md)
2. **Choose your platform**: EKS, GKE, AKS, or Kind
3. **Enable Ingress resources**: Uncomment cloud-specific Ingress configurations
4. **Configure storage classes**: Uncomment appropriate storage classes in PVCs

```bash
# Deploy to any Kubernetes cluster
kubectl apply -f k8-deployment/

# Setup API keys
kubectl create secret generic ai-provider-keys \
  --from-literal=OPENAI_API_KEY="$OPENAI_API_KEY" \
  --from-literal=GROQ_API_KEY="$GROQ_API_KEY" \
  -n gomind-examples

# Deploy examples
kubectl apply -f agent-example/k8-deployment.yaml
kubectl apply -f tool-example/k8-deployment.yaml
```

## 📋 Detailed Example Features

### 🔧 Core Learning Examples

#### [tool-example](tool-example/) - Passive Tool Pattern
**What it demonstrates:**
- Multiple capability registration (`current_weather`, `forecast`, `analysis`)
- Auto-generated REST endpoints (`/api/capabilities/current_weather`)
- External API integration (Weather API) with error handling
- Caching patterns and request/response transformation
- Tool registration without discovery capabilities (passive pattern)

**Perfect for:** Understanding how to build focused, discoverable services that other agents can find and use

---

#### [agent-example](agent-example/) - Active Agent Pattern
**What it demonstrates:**
- Service discovery capabilities (finds and catalogs available tools)
- Tool orchestration and coordination logic
- AI provider auto-detection (OpenAI/Groq/Anthropic/Gemini)
- Basic AI-enhanced responses and intelligent routing
- Agent-to-tool communication patterns and data flow

**Perfect for:** Understanding coordination patterns and building service mesh-like architectures

---

### 🤖 AI-Enhanced Examples

#### [agent-example-enhanced](agent-example-enhanced/) - AI-Everything Agent
**What it demonstrates:**
- 4 distinct capabilities ALL enhanced with AI intelligence
- Multiple AI provider support with automatic fallback mechanisms
- Enhanced data synthesis and contextual analysis
- Smart context awareness across different operations
- Production-ready AI error handling and graceful degradation

**Perfect for:** Building agents where every capability benefits from AI enhancement

---

#### [ai-agent-example](ai-agent-example/) - AI-First Architecture
**What it demonstrates:**
- AI drives EVERY decision from initial request to final response
- Intent recognition and dynamic execution planning
- AI-guided execution flow with no hardcoded business logic
- Continuous AI oversight and real-time adaptation
- Hard dependency on AI (demonstrates true AI-native design)

**Perfect for:** Building systems where AI IS the primary brain, not just a helper tool

---

#### [ai-multi-provider](ai-multi-provider/) - Production AI Resilience
**What it demonstrates:**
- Primary/fallback/secondary AI provider configuration
- Automatic provider failover and real-time health checking
- Hybrid deployment (can run as Tool, Agent, or both simultaneously)
- Provider-specific optimization (speed vs accuracy vs cost)
- Production-grade AI reliability patterns

**Perfect for:** Mission-critical systems that cannot afford AI downtime

---

#### [ai-tools-showcase](ai-tools-showcase/) - Ready-to-Deploy AI Tools
**What it demonstrates:**
- 4 production-ready AI tools you can use immediately:
  - **Translation Tool**: Professional multi-language translation
  - **Summarization Tool**: Intelligent document and content summarization
  - **Sentiment Analysis Tool**: Emotion, tone, and intent detection
  - **Code Review Tool**: AI-powered code quality and security analysis
- Composite deployment pattern (all tools hosted in single service)
- Individual tool deployment and scaling strategies

**Perfect for:** Adding professional AI capabilities to existing systems without building from scratch

---

### 🏗️ Advanced Orchestration

#### [orchestration-example](orchestration-example/) - Multi-Modal Orchestration
**What it demonstrates:**
- **Autonomous Mode**: AI analyzes incoming requests and dynamically determines routing
- **Workflow Mode**: Recipe-based execution with explicit dependencies and error handling
- **Hybrid Mode**: Intelligently combines AI decision-making with predefined workflows
- Multi-agent coordination patterns and complex scenario handling
- Advanced routing strategies and workflow adaptation

**Perfect for:** Enterprise systems requiring sophisticated workflow coordination

---

#### [workflow-example](workflow-example/) - Declarative YAML Workflows
**What it demonstrates:**
- YAML workflow definitions loaded dynamically from Kubernetes ConfigMaps
- Declarative step dependencies, parallel execution, and error handling
- Runtime workflow modification without service redeployment
- Built-in workflow templates and common patterns
- Optional AI enhancement for workflow optimization and adaptation

**Perfect for:** Business process automation and user-configurable workflows

---

### 📊 Framework Patterns

#### [telemetry](telemetry/) - Production Monitoring
- Comprehensive metrics emission (counters, histograms, gauges)
- Circuit breaker integration with telemetry
- Error tracking and success rate monitoring
- Development vs production telemetry profiles

#### [context_propagation](context_propagation/) - Distributed Tracing
- Request correlation across service boundaries
- OpenTelemetry integration and trace visualization
- User and tenant context tracking
- Performance monitoring across microservices

#### [error_handling](error_handling/) - Framework Error Consistency
- Structured error types and sentinel error patterns
- Retryable error detection and automatic retry logic
- Configuration error handling and validation
- Framework-wide error consistency and debugging

## 🏗️ System Architecture

```
┌─────────────────┐    Service      ┌─────────────────┐
│     Agents      │    Discovery    │      Redis      │
│  (Active Logic) │◄────────────────┤   (Registry)    │
└─────────┬───────┘                 └─────────────────┘
          │                                   ▲
          │ Orchestrate                       │ Register
          │                                   │
          ▼                                   │
┌─────────────────┐                 ┌─────────┴───────┐
│     Tools       │                 │   Monitoring    │
│ (Capabilities)  │                 │ Prometheus/Otel │
└─────────┬───────┘                 └─────────────────┘
          │
          ▼
┌─────────────────┐
│   AI Providers  │
│ OpenAI/Groq/etc │
└─────────────────┘
```

**Component Roles:**
- **Tools**: Provide focused capabilities (weather, APIs, data processing)
- **Agents**: Coordinate workflows and orchestrate multiple tools
- **Redis**: Service discovery registry and caching layer
- **AI Providers**: External intelligence for analysis and decision making
- **Monitoring**: Full observability with metrics, logs, and traces

## 📊 Monitoring & Observability

### Accessing Dashboards

With Kind demo running:
- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090
- **Jaeger**: http://localhost:16686
- **Redis**: Direct access via `kubectl port-forward`

### Key Metrics

```promql
# Service health
up{job=~"gomind.*"}

# Request rates
rate(http_requests_total{gomind_framework_type=~"tool|agent"}[5m])

# Error rates
rate(http_requests_total{status=~"5.."}[5m])

# Service discovery
rate(gomind_discovery_requests_total[5m])
```

### Service Health Monitoring

#### Understanding Heartbeat Logs

When running examples, you'll see periodic heartbeat logs that indicate service health:

```bash
# Example output from a healthy service
INFO Started heartbeat for tool registration tool_id=weather-tool-7f8d9 tool_name=weather-tool interval_sec=15 ttl_sec=30
INFO Tool initialization completed id=weather-tool-7f8d9 name=weather-tool discovery_enabled=true
# ... after 5 minutes ...
INFO Heartbeat health summary service_id=weather-tool-7f8d9 service_name=weather-tool success_count=20 failure_count=0 success_rate=100.00% uptime_minutes=5
```

#### Monitoring Heartbeat Health

```bash
# View all heartbeat-related logs
kubectl logs deployment/<service-name> -n gomind-examples | grep -E "(heartbeat|Heartbeat)"

# Watch health summaries in real-time
kubectl logs -f deployment/<service-name> -n gomind-examples | grep "Heartbeat health summary"

# Check for heartbeat failures
kubectl logs deployment/<service-name> -n gomind-examples | grep -E "(Failed to send heartbeat|failure_count)"
```

## 🚨 Troubleshooting

### Common Issues

**Services not discovering each other:**
```bash
# Check if heartbeats are running
kubectl logs deployment/<service-name> -n gomind-examples --tail=100 | grep -E "(Started heartbeat|Heartbeat health)"

# Check Redis connectivity
kubectl port-forward -n gomind-examples svc/redis 6379:6379
redis-cli ping

# Check service registrations
redis-cli KEYS "gomind:services:*"

# Common issues and solutions:
# 1. "Failed to send heartbeat" with "connection refused"
#    -> Redis is not accessible. Check Redis deployment and service.

# 2. No "Started heartbeat" log
#    -> Service discovery might be disabled. Check REDIS_URL environment variable.

# 3. High failure_count in health summary
#    -> Intermittent network issues. Check pod networking and Redis stability.
```

**AI requests failing:**
```bash
# Verify API keys are set
kubectl get secret ai-provider-keys -n gomind-examples -o yaml

# Check logs for AI errors
kubectl logs -f deployment/research-agent -n gomind-examples
```

**Pods not starting:**
```bash
# Check pod status
kubectl get pods -n gomind-examples

# Get detailed events
kubectl describe pod <pod-name> -n gomind-examples
```

### Debug Commands

```bash
# Check cluster status
./setup-kind-demo.sh status

# View logs
kubectl logs -f -l app.kubernetes.io/part-of=gomind-framework -n gomind-examples

# Clean restart
./setup-kind-demo.sh cleanup
./setup-kind-demo.sh setup
```

### 🐛 Debugging & Troubleshooting

#### Enable Debug Logging

When things aren't working as expected, enable debug logs to see framework internals:

```bash
# For normal operations (recommended)
kubectl set env deployment/weather-tool GOMIND_LOG_LEVEL=info -n gomind-examples

# For debugging heartbeat issues
kubectl set env deployment/weather-tool GOMIND_LOG_LEVEL=debug -n gomind-examples

# Watch the detailed logs
kubectl logs -f deployment/weather-tool -n gomind-examples

# View logs with timestamps
kubectl logs deployment/weather-tool -n gomind-examples --timestamps=true
```

#### Supported Log Levels

| Level | Use Case | What Gets Logged |
|-------|----------|------------------|
| `debug` | Troubleshooting | Everything - Debug + Info + Warn + Error |
| `info` | Production (default) | Info + Warn + Error messages |
| `warn` | Production (minimal) | Warn + Error messages only |
| `error` | Production (critical only) | Error messages only |

**How Filtering Works:**
- Each level includes all higher severity levels
- Setting `error` minimizes log volume to only critical issues
- Setting `debug` shows maximum detail for troubleshooting

#### Quick Troubleshooting Guide

**"Service not registering in Redis"**
```bash
# Check Redis connectivity
kubectl exec -it deployment/weather-tool -n gomind-examples -- sh -c 'nc -zv redis 6379'

# Enable debug to see registration attempts
kubectl set env deployment/weather-tool GOMIND_LOG_LEVEL=debug -n gomind-examples
kubectl logs -f deployment/weather-tool -n gomind-examples | grep -i redis
```

**"Agent can't discover tools"**
```bash
# Check what's registered in Redis
kubectl exec -it deployment/redis -n gomind-examples -- redis-cli KEYS "gomind:services:*"

# Enable debug on agent to see discovery attempts
kubectl set env deployment/research-agent GOMIND_LOG_LEVEL=debug -n gomind-examples
kubectl logs -f deployment/research-agent -n gomind-examples | grep -i discover
```

**"AI provider errors"**
```bash
# Check if API keys are set correctly
kubectl get secret external-api-keys -n gomind-examples -o yaml

# Enable debug to see AI API calls
kubectl set env deployment/research-agent GOMIND_LOG_LEVEL=debug -n gomind-examples
kubectl logs -f deployment/research-agent -n gomind-examples | grep -i "ai\|openai\|anthropic"
```

For comprehensive logging configuration, see [Logging Configuration](../docs/API_REFERENCE.md#logging-configuration) in the API Reference.

## 🎨 Development Workflow

### 1. Pattern-Specific Development

#### 🔧 Building Tools (Passive Components)
```bash
# Start with tool-example
cd tool-example && go run main.go

# Key patterns to implement:
# 1. Register capabilities with core.Capability{}
# 2. Implement handler functions
# 3. Use core.NewTool() for base functionality
# 4. Test auto-generated endpoints: /api/capabilities/<name>

# Create your own tool:
cp -r tool-example my-data-tool
# → Modify capabilities: data_analysis, data_transform, etc.
# → Update handlers for your domain
# → Tools are discovered automatically by agents
```

#### 🤖 Building Agents (Active Coordinators)
```bash
# Start with agent-example
cd agent-example && go run main.go

# Key patterns to implement:
# 1. Use core.NewBaseAgent() for discovery powers
# 2. Implement service discovery with agent.DiscoverServices()
# 3. Coordinate tool calls based on requests
# 4. Add AI integration with ai.NewClient()

# Create your own agent:
cp -r agent-example my-workflow-agent
# → Modify coordination logic
# → Add domain-specific orchestration
# → Agents discover and coordinate tools automatically
```

#### 🧠 AI-Enhanced Development
```bash
# For basic AI enhancement:
cd agent-example-enhanced

# For AI-native architecture:
cd ai-agent-example

# For production AI reliability:
cd ai-multi-provider

# Key AI patterns:
# - ai.NewClient() for auto-detection
# - Multiple provider support
# - Fallback mechanisms
# - AI-driven decision making
```

### 2. Integration Testing
```bash
# Full system testing with Kind
./setup-kind-demo.sh setup

# Test specific patterns:
# Tools: curl -X POST http://localhost:8080/api/capabilities/<name>
# Agents: curl http://localhost:8090/api/capabilities/discover_tools
# AI: curl -X POST http://localhost:8090/api/capabilities/research_topic

# Debug specific components:
kubectl logs -f -l app.kubernetes.io/name=<component> -n gomind-examples
```

### 3. Extending Examples

#### 📝 Adding New Capabilities
```bash
# In any tool:
tool.RegisterCapability(core.Capability{
    Name:        "your_capability",
    Description: "What it does",
    InputTypes:  []string{"json"},
    OutputTypes: []string{"json"},
    Handler:     yourHandlerFunction,
})
# → Automatically creates /api/capabilities/your_capability endpoint
```

#### 🔄 Adding Orchestration Logic
```bash
# In any agent:
func (a *YourAgent) handleComplexWorkflow(w http.ResponseWriter, r *http.Request) {
    // 1. Discover available tools
    tools, _ := a.DiscoverServices()

    // 2. Use AI to plan execution (if available)
    if a.aiClient != nil {
        plan, _ := a.aiClient.CreateCompletion(context.Background(), &ai.CompletionRequest{
            Messages: []ai.Message{{Role: "user", Content: "How should I process this request?"}},
        })
    }

    // 3. Coordinate tool calls
    // 4. Synthesize results
}
```

### 4. Production Deployment

**Choose Your Deployment Pattern:**
- **Simple**: Use existing k8-deployment YAML files
- **Cloud-Specific**: Follow [CLOUD_DEPLOYMENT_GUIDE.md](CLOUD_DEPLOYMENT_GUIDE.md)
- **Custom**: Modify deployment configs for your infrastructure

**Key Production Considerations:**
```bash
# Resource limits based on example type:
# Tools: 200m CPU, 256Mi memory (lightweight)
# Agents: 500m CPU, 512Mi memory (coordination overhead)
# AI-Enhanced: 1000m CPU, 1Gi memory (AI processing)

# Scaling patterns:
# Tools: Scale horizontally based on request load
# Agents: Typically 1-3 replicas (coordination complexity)
# Multi-Provider: Scale based on AI API rate limits
```

## 🧹 Cleanup

```bash
# Stop port forwarding
./setup-kind-demo.sh cleanup

# Delete Kind cluster
./setup-kind-demo.sh delete

# Or manually
kind delete cluster --name gomind-demo
```

## 📚 Learning Progression

### 🚀 Beginner Path (30 minutes)
1. **Start Simple**: `./setup-kind-demo.sh setup` → See everything working
2. **Core Concepts**: Study `tool-example` → Understand passive tools
3. **Coordination**: Study `agent-example` → Understand active agents
4. **Test Together**: Run tool + agent → See service discovery in action

### 🤖 AI Integration Path (1 hour)
1. **Enhanced AI**: Study `agent-example-enhanced` → AI in every capability
2. **AI-Native**: Study `ai-agent-example` → AI-driven architecture
3. **Production AI**: Study `ai-multi-provider` → Provider resilience
4. **Ready Tools**: Study `ai-tools-showcase` → Use built-in AI capabilities

### 🏗️ Advanced Architecture Path (2 hours)
1. **Complex Flows**: Study `orchestration-example` → Multi-modal coordination
2. **Declarative**: Study `workflow-example` → YAML-driven workflows
3. **Observability**: Study `telemetry` + `context_propagation` → Production monitoring
4. **Reliability**: Study `error_handling` → Framework consistency

### 🎯 Use Case Focused Learning

**"I want to add AI to my existing service"**
→ Start with `ai-tools-showcase` → See 4 ready-to-use AI tools

**"I want to build intelligent workflows"**
→ `agent-example` → `orchestration-example` → `workflow-example`

**"I want production-grade AI reliability"**
→ `ai-multi-provider` → `telemetry` → `error_handling`

**"I want to understand the framework patterns"**
→ `tool-example` → `agent-example` → `context_propagation`

## 📚 Next Steps

1. **🏃 Run Quick Start** - See the full system (5 min)
2. **🔍 Pick Your Path** - Choose beginner/AI/advanced based on your needs (15-120 min)
3. **🧪 Test Interactions** - Agent + Tool orchestration (10 min)
4. **🎨 Customize** - Copy and modify examples for your use case (30 min)
5. **☸️ Deploy** - Production Kubernetes deployment (1 hour)
6. **🤖 Build** - Create your intelligent distributed system

## 📖 Documentation

- **[Setup Script](setup-kind-demo.sh)** - Automated demo environment
- **[API Keys Guide](setup-api-keys.sh)** - Automated API key configuration
- **[Cloud Deployment](CLOUD_DEPLOYMENT_GUIDE.md)** - Production deployment guide
- **[Individual Examples](.)** - Each example has its own README
- **[Framework Core](../core/)** - Core framework documentation
- **[AI Integration](../ai/)** - AI provider configuration

---

**Ready to build intelligent distributed systems?** 🚀

Start with `./setup-kind-demo.sh setup` and explore the examples. Each demonstrates different patterns for building AI-enhanced, discoverable, and orchestrable systems.

The framework handles service discovery, monitoring, and AI integration while you focus on building amazing capabilities!