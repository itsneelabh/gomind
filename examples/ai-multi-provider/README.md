# Multi-Provider AI with Fallback Example

A comprehensive demonstration of **multi-provider AI configuration** with **automatic fallback patterns** for both tools and agents. This example shows how to build resilient AI-powered systems that can gracefully handle provider failures and optimize performance by routing tasks to the most suitable AI provider.

## 🎯 What This Example Demonstrates

### Multi-Provider Architecture Patterns

| Pattern | Description | Benefits |
|---------|-------------|----------|
| **🔄 Automatic Fallback** | Primary → Fallback → Secondary provider chain | High availability, fault tolerance |
| **⚖️ Provider Comparison** | Execute same task on multiple providers | Quality comparison, best result selection |
| **🎯 Task-Aware Routing** | Route tasks based on provider strengths | Optimal performance, cost efficiency |
| **📊 Health Monitoring** | Real-time provider health checking | Proactive failure detection |

### Two Implementation Examples

This example includes **both tool and agent implementations**:

#### 🔧 **Multi-Provider Tool** (Port 8085)
- **Passive component** with AI fallback capabilities
- **4 capabilities** demonstrating different fallback patterns
- **Provider health monitoring** and comparison features
- **Automatic failover** on provider errors

#### 🤖 **Multi-Provider Agent** (Port 8086)  
- **Active orchestration** with discovery and AI fallback
- **4 capabilities** showing agent-specific patterns
- **Intelligent task routing** based on provider strengths
- **Service discovery** with AI-powered planning

## 🏗️ Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                Multi-Provider AI System                 │
├─────────────────────────────────────────────────────────┤
│  🔧 Tool (Port 8085)        🤖 Agent (Port 8086)       │
│  ┌─────────────────────┐    ┌─────────────────────────┐ │
│  │ • Fallback Processing│    │ • Discovery + Planning │ │
│  │ • Provider Comparison│    │ • Multi-Provider Orch  │ │
│  │ • Best Response     │    │ • Adaptive Problem Solv│ │
│  │ • Health Monitoring │    │ • Task-Aware Routing   │ │
│  └─────────────────────┘    └─────────────────────────┘ │
├─────────────────────────────────────────────────────────┤
│                    Fallback Chain                       │
│   Primary Provider → Fallback Provider → Secondary      │
├─────────────────────────────────────────────────────────┤
│                  Supported Providers                    │
│  OpenAI • Groq • Anthropic • DeepSeek • Gemini         │
└─────────────────────────────────────────────────────────┘
```

### Fallback Logic Flow

```
Request → Primary Provider
            ↓ (on failure)
          Fallback Provider  
            ↓ (on failure)
          Secondary Provider
            ↓ (on failure)
            Error Response
```

## 🚀 Quick Start

### Prerequisites

**Minimum Requirements:**
- Go 1.25 or later
- **At least 2 AI provider API keys** (for fallback functionality)

**Recommended Setup** (3+ providers for full functionality):
```bash
export OPENAI_API_KEY="sk-your-openai-key"        # Primary (most capable)
export GROQ_API_KEY="gsk-your-groq-key"           # Fallback (ultra-fast)
export ANTHROPIC_API_KEY="sk-ant-your-claude-key" # Secondary (analysis)
```

**Optional Additional Providers:**
```bash
export DEEPSEEK_API_KEY="sk-your-deepseek-key"    # Advanced reasoning  
export GEMINI_API_KEY="your-google-gemini-key"    # Google AI
```

### 1. Run Multi-Provider Tool

```bash
# Navigate to the example
cd examples/ai-multi-provider

# Install dependencies
go mod tidy

# Deploy the tool with fallback capabilities
export DEPLOYMENT_MODE="tool"
export TOOL_PORT="8085"
go run main.go
```

**Expected Output:**
```
🎯 Multi-provider configuration:
   Primary: OpenAI GPT (openai)
   Fallback: Groq (openai)
   Secondary: Anthropic Claude (anthropic)
✅ Multi-provider AI tool created successfully
🔧 Multi-provider AI Tool starting on port 8085...
```

### 2. Run Multi-Provider Agent  

```bash
# Deploy the agent with discovery and AI fallback
export DEPLOYMENT_MODE="agent"  
export AGENT_PORT="8086"
go run main.go
```

### 3. Run Both Simultaneously

```bash
# Deploy both tool and agent together
export DEPLOYMENT_MODE="both"
export TOOL_PORT="8085"
export AGENT_PORT="8086"
go run main.go
```

## 📖 Usage Examples

### Tool Examples (Port 8085)

#### 1. **Automatic Fallback Processing**

Demonstrates automatic failover when primary provider fails:

```bash
curl -X POST http://localhost:8085/api/capabilities/process_with_fallback \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Explain the concept of machine learning in simple terms",
    "task": "analyze",
    "max_retry": 3
  }'
```

**Response:**
```json
{
  "text": "Explain the concept of machine learning...",
  "task": "analyze",
  "result": "Machine learning is a way for computers to learn patterns...",
  "provider_used": "OpenAI GPT",
  "attempt_number": 1,
  "fallback_applied": false,
  "processing_time": "2.3s",
  "model": "gpt-4",
  "multi_provider": true
}
```

**Fallback Response** (when primary fails):
```json
{
  "provider_used": "Groq (fallback)",
  "attempt_number": 2,
  "fallback_applied": true,
  "processing_time": "0.8s"
}
```

#### 2. **Provider Comparison**

Execute the same task on all available providers:

```bash
curl -X POST http://localhost:8085/api/capabilities/compare_providers \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Write a professional email declining a job offer",
    "task": "creative",
    "parallel": true
  }'
```

**Response:**
```json
{
  "text": "Write a professional email declining a job offer",
  "comparison": [
    {
      "provider_used": "OpenAI GPT",
      "content": "Dear [Hiring Manager], Thank you for offering me the position...",
      "quality_score": 0.92,
      "processing_time": "2.1s"
    },
    {
      "provider_used": "Groq",  
      "content": "Hi there, Thanks for the job offer...",
      "quality_score": 0.78,
      "processing_time": "0.6s"
    },
    {
      "provider_used": "Anthropic Claude",
      "content": "Dear Hiring Team, I am writing to express my gratitude...",
      "quality_score": 0.95,
      "processing_time": "3.2s"
    }
  ],
  "providers": 3,
  "best_result": {
    "provider_used": "Anthropic Claude",
    "quality_score": 0.95
  }
}
```

#### 3. **Best Response Selection**

Try all providers and automatically select the highest quality response:

```bash
curl -X POST http://localhost:8085/api/capabilities/best_response \
  -H "Content-Type: application/json" \
  -d '{
    "text": "function calculateTax(income) { return income * 0.25; }",
    "task": "code_review",
    "quality_criteria": "accuracy"
  }'
```

#### 4. **Provider Health Check**

Monitor the health and availability of all configured providers:

```bash
curl -X POST http://localhost:8085/api/capabilities/provider_health
```

**Response:**
```json
{
  "provider_health": {
    "primary": {
      "provider": "OpenAI GPT",
      "healthy": true,
      "response_time": "1.2s",
      "response": "OK",
      "model": "gpt-4"
    },
    "fallback": {
      "provider": "Groq",
      "healthy": true,
      "response_time": "0.3s",
      "response": "OK"
    },
    "secondary": {
      "provider": "Anthropic Claude", 
      "healthy": false,
      "error": "API quota exceeded"
    }
  }
}
```

### Agent Examples (Port 8086)

#### 1. **Discovery and Planning with Fallback**

Agent discovers services and creates execution plans using AI with fallback:

```bash
curl -X POST http://localhost:8086/api/capabilities/discover_and_plan \
  -H "Content-Type: application/json" \
  -d '{
    "goal": "Analyze customer feedback and generate improvement recommendations",
    "constraints": ["real-time processing", "privacy compliance"],
    "max_services": 5
  }'
```

**Response:**
```json
{
  "goal": "Analyze customer feedback and generate improvement recommendations",
  "services_discovered": 3,
  "execution_plan": "1. Use sentiment analysis service for emotion detection...",
  "planner_provider": "OpenAI GPT",
  "planning_attempt": 1,
  "available_services": [...],
  "multi_provider": true
}
```

#### 2. **Multi-Provider Orchestration**

Uses different providers for different orchestration tasks:

```bash
curl -X POST http://localhost:8086/api/capabilities/orchestrate_multi_provider \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Create a comprehensive marketing campaign strategy",
    "complexity": "complex",
    "priority": "high"
  }'
```

#### 3. **Adaptive Problem Solving**

Solves problems using multiple providers with learning:

```bash
curl -X POST http://localhost:8086/api/capabilities/solve_with_fallback \
  -H "Content-Type: application/json" \
  -d '{
    "problem": "Our API response times have increased by 200% in the last week",
    "context": "E-commerce platform with microservices architecture",
    "constraints": ["no downtime allowed", "limited budget"],
    "max_attempts": 3
  }'
```

**Response:**
```json
{
  "problem": "Our API response times have increased by 200%...",
  "solution_found": true,
  "final_solution": "Based on the symptoms, this appears to be a database bottleneck...",
  "successful_provider": "Anthropic Claude",
  "total_attempts": 2,
  "all_attempts": [
    {
      "attempt": 1,
      "provider": "OpenAI GPT",
      "success": false,
      "error": "API timeout"
    },
    {
      "attempt": 2,
      "provider": "Anthropic Claude",
      "success": true,
      "selected": true,
      "solution": "Based on the symptoms..."
    }
  ],
  "fallback_applied": true
}
```

#### 4. **Task-Aware Provider Routing**

Routes tasks to the most suitable provider based on task type and provider strengths:

```bash
curl -X POST http://localhost:8086/api/capabilities/route_by_provider_strength \
  -H "Content-Type: application/json" \
  -d '{
    "task_type": "creative",
    "content": "Write a compelling product description for a new smartwatch",
    "preferences": {
      "creativity": true,
      "speed": false
    }
  }'
```

## 🔧 Configuration & Deployment

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `DEPLOYMENT_MODE` | Deployment mode: `tool`, `agent`, or `both` | `tool` | No |
| `TOOL_PORT` | Tool service port | 8085 | No |
| `AGENT_PORT` | Agent service port | 8086 | No |
| `REDIS_URL` | Redis connection for service discovery | redis://localhost:6379 | No |

### AI Provider Configuration

The system automatically detects available providers in priority order:

1. **OpenAI GPT** (`OPENAI_API_KEY`) - Most capable, primary choice
2. **Groq** (`GROQ_API_KEY`) - Ultra-fast inference, excellent fallback
3. **Anthropic Claude** (`ANTHROPIC_API_KEY`) - Great for analysis and reasoning
4. **DeepSeek** (`DEEPSEEK_API_KEY`) - Advanced reasoning capabilities
5. **Google Gemini** (`GEMINI_API_KEY`) - Google AI models

### Provider-Specific Configuration

```bash
# OpenAI (standard configuration)
export OPENAI_API_KEY="sk-your-key"

# Groq (OpenAI-compatible with custom endpoint)  
export GROQ_API_KEY="gsk-your-key"
# Automatically configured: https://api.groq.com/openai/v1

# Anthropic (native API)
export ANTHROPIC_API_KEY="sk-ant-your-key"

# DeepSeek (OpenAI-compatible with custom endpoint)
export DEEPSEEK_API_KEY="sk-your-key" 
# Automatically configured: https://api.deepseek.com/v1

# Google Gemini (native API)
export GEMINI_API_KEY="your-key"
```

### Docker Deployment

#### Tool Only
```bash
docker run -p 8085:8080 \
  -e DEPLOYMENT_MODE="tool" \
  -e OPENAI_API_KEY="your-key" \
  -e GROQ_API_KEY="your-key" \
  -e ANTHROPIC_API_KEY="your-key" \
  multi-provider-ai:latest
```

#### Agent Only
```bash
docker run -p 8086:8080 \
  -e DEPLOYMENT_MODE="agent" \
  -e OPENAI_API_KEY="your-key" \
  -e GROQ_API_KEY="your-key" \
  -e REDIS_URL="redis://your-redis:6379" \
  multi-provider-ai:latest
```

#### Both Services  
```bash
docker run -p 8085:8085 -p 8086:8086 \
  -e DEPLOYMENT_MODE="both" \
  -e TOOL_PORT="8085" \
  -e AGENT_PORT="8086" \
  -e OPENAI_API_KEY="your-key" \
  -e GROQ_API_KEY="your-key" \
  -e ANTHROPIC_API_KEY="your-key" \
  multi-provider-ai:latest
```

### Kind Cluster Deployment (Complete Self-Contained)

This example is **completely self-sufficient** for Kind cluster deployment, including Redis for multi-provider coordination.

#### Prerequisites
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) installed
- [Docker](https://docs.docker.com/get-docker/) running
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed

#### Step-by-Step Kind Deployment

```bash
# 1. Create Kind cluster
kind create cluster --name gomind-multi-provider

# 2. Build Docker image
docker build -t ai-multi-provider:latest .

# 3. Load image into Kind cluster
kind load docker-image ai-multi-provider:latest --name gomind-multi-provider

# 4. Create secrets with your AI API keys (REQUIRED - at least 2 for fallback)
kubectl create secret generic multi-provider-secrets \
  --from-literal=OPENAI_API_KEY="sk-your-openai-key" \
  --from-literal=GROQ_API_KEY="gsk-your-groq-key" \
  --from-literal=ANTHROPIC_API_KEY="sk-ant-your-claude-key" \
  --from-literal=DEEPSEEK_API_KEY="sk-your-deepseek-key" \
  --dry-run=client -o yaml | kubectl apply -f -

# 5. Deploy everything (includes Redis + Multi-Provider AI)
kubectl apply -f k8-deployment.yaml

# 6. Wait for all pods to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=multi-provider-ai -n gomind-multi-provider --timeout=300s

# 7. Verify Redis coordination is working
kubectl exec -it deployment/redis -n gomind-multi-provider -- redis-cli ping

# 8. Check deployment status (should see both tool and agent services)
kubectl get pods,svc -n gomind-multi-provider
```

#### Access the Multi-Provider Services

**Tool Service (Port 8085):**
```bash
# Port forward to tool service
kubectl port-forward svc/multi-provider-service 8085:8085 -n gomind-multi-provider

# Test fallback patterns
curl -X POST http://localhost:8085/api/capabilities/process_with_fallback \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Explain quantum computing",
    "task": "analyze",
    "max_retry": 3
  }'

# Test provider comparison
curl -X POST http://localhost:8085/api/capabilities/compare_providers \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Write a haiku about programming",
    "task": "creative",
    "parallel": true
  }'
```

**Agent Service (Port 8086):**
```bash
# Port forward to agent service
kubectl port-forward svc/multi-provider-service 8086:8086 -n gomind-multi-provider

# Test intelligent service discovery and planning
curl -X POST http://localhost:8086/api/capabilities/discover_and_plan \
  -H "Content-Type: application/json" \
  -d '{
    "goal": "Create a comprehensive analysis of user data",
    "constraints": ["privacy-compliant", "fast-execution"],
    "max_services": 5
  }'

# Test multi-provider orchestration
curl -X POST http://localhost:8086/api/capabilities/orchestrate_multi_provider \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Process customer feedback for insights",
    "complexity": "moderate",
    "priority": "high"
  }'
```

#### Troubleshooting Multi-Provider Deployment

**Provider fallback not working:**
```bash
# Check provider configuration
kubectl logs -f deployment/multi-provider-ai -n gomind-multi-provider

# Verify API keys are set
kubectl get secret multi-provider-secrets -n gomind-multi-provider -o yaml

# Test individual providers
kubectl exec -it deployment/multi-provider-ai -n gomind-multi-provider -- sh
# Inside pod: env | grep API_KEY
```

**Service discovery issues:**
```bash
# Check Redis connectivity from multi-provider service
kubectl exec -it deployment/multi-provider-ai -n gomind-multi-provider -- sh
# Inside pod: redis-cli -h redis-service ping

# Check registered services
kubectl exec -it deployment/redis -n gomind-multi-provider -- redis-cli KEYS "gomind:*"
```

**Performance optimization:**
```bash
# Check which providers are fastest
curl -X POST http://localhost:8085/api/capabilities/provider_health \
  -H "Content-Type: application/json" \
  -d '{}'

# Monitor fallback patterns in logs
kubectl logs -f deployment/multi-provider-ai -n gomind-multi-provider | grep -E "fallback|provider"
```

#### Deployment Modes

The multi-provider example supports different deployment configurations:

```bash
# Deploy only tool service
kubectl set env deployment/multi-provider-ai DEPLOYMENT_MODE=tool -n gomind-multi-provider

# Deploy only agent service  
kubectl set env deployment/multi-provider-ai DEPLOYMENT_MODE=agent -n gomind-multi-provider

# Deploy both services (default)
kubectl set env deployment/multi-provider-ai DEPLOYMENT_MODE=both -n gomind-multi-provider
```

#### Clean Up

```bash
# Delete the Kind cluster (removes everything)
kind delete cluster --name gomind-multi-provider
```

## 📊 Advanced Features

### Provider Strength Matrix

Different providers excel at different tasks:

| Provider | Creative Writing | Code Analysis | Reasoning | Speed | Cost |
|----------|------------------|---------------|-----------|-------|------|
| **OpenAI GPT-4** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐ |
| **Groq** | ⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **Claude** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐ |
| **DeepSeek** | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Gemini** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |

### Automatic Task Routing

The system automatically routes tasks to optimal providers:

```go
Creative Tasks    → OpenAI GPT (creativity strength)
Code Analysis     → Claude or DeepSeek (technical analysis)  
Fast Processing   → Groq (ultra-fast inference)
Complex Reasoning → Claude (reasoning excellence)
Cost-Sensitive   → Groq or DeepSeek (cost-effective)
```

### Fallback Strategies

#### 1. **Linear Fallback** (Default)
```
Primary → Fallback → Secondary → Error
```

#### 2. **Task-Specific Fallback**
```
Code Task → DeepSeek → Claude → OpenAI
Creative Task → OpenAI → Claude → Gemini
Speed Task → Groq → OpenAI → Others
```

#### 3. **Quality-Based Fallback**
```
Try All Providers → Select Best Quality Response
```

### Health Monitoring

Real-time provider health monitoring with automatic failover:

```bash
# Monitor provider health
while true; do
  curl -s http://localhost:8085/api/capabilities/provider_health | jq '.provider_health'
  sleep 30
done
```

## 🧪 Testing & Validation

### Basic Functionality Tests

```bash
# Test tool fallback
curl -X POST http://localhost:8085/api/capabilities/process_with_fallback \
  -d '{"text":"Test message","task":"analyze"}'

# Test agent discovery  
curl -X POST http://localhost:8086/api/capabilities/discover_and_plan \
  -d '{"goal":"Test goal"}'

# Test provider health
curl -X POST http://localhost:8085/api/capabilities/provider_health
```

### Load Testing

```bash
# Concurrent requests to test fallback under load
for i in {1..50}; do
  curl -X POST http://localhost:8085/api/capabilities/process_with_fallback \
    -d "{\"text\":\"Load test $i\",\"task\":\"analyze\"}" &
done
wait
```

### Failover Testing

```bash
# Temporarily disable primary provider (revoke API key) to test fallback
export OPENAI_API_KEY=""
curl -X POST http://localhost:8085/api/capabilities/process_with_fallback \
  -d '{"text":"Failover test","task":"analyze"}'
# Should automatically use fallback provider
```

## 🎯 Best Practices

### 1. **Provider Configuration**

```bash
# Recommended configuration for production
export OPENAI_API_KEY="primary"    # Most capable, higher cost
export GROQ_API_KEY="fallback"     # Fast, cost-effective fallback
export ANTHROPIC_API_KEY="analysis" # Best for reasoning tasks
```

### 2. **Error Handling**

```json
{
  "error_handling": {
    "retry_attempts": 3,
    "timeout": "30s",
    "fallback_enabled": true,
    "health_check_interval": "60s"
  }
}
```

### 3. **Performance Optimization**

```bash
# Use parallel processing for provider comparison
curl -X POST http://localhost:8085/api/capabilities/compare_providers \
  -d '{"text":"content","parallel":true}'

# Route speed-sensitive tasks to Groq
curl -X POST http://localhost:8086/api/capabilities/route_by_provider_strength \
  -d '{"task_type":"fast","preferences":{"speed":true}}'
```

### 4. **Cost Management**

```bash
# Monitor token usage across providers
curl -X POST http://localhost:8085/api/capabilities/provider_health | \
  jq '.provider_health[].token_usage'
```

## 🚨 Troubleshooting

### Common Issues

#### 1. **Insufficient Providers**
```
Error: multi-provider setup requires at least 2 AI providers
```
**Solution**: Set at least 2 provider API keys

#### 2. **All Providers Failed**
```json
{"error": "Processing failed on all available providers"}
```
**Solutions**:
- Check API keys and quotas
- Verify network connectivity
- Check provider status pages

#### 3. **Slow Fallback Response**
```
High response times when fallback is triggered
```
**Solutions**:
- Use faster fallback provider (Groq)
- Reduce timeout values
- Implement provider caching

#### 4. **Agent Discovery Issues**
```json
{"services_discovered": 0}
```
**Solutions**:
- Verify Redis connectivity
- Check service registration
- Ensure same namespace

### Debug Commands

```bash
# Check provider configuration
curl -s http://localhost:8085/health | jq '.ai_providers'

# Monitor real-time health
watch -n 5 'curl -s http://localhost:8085/api/capabilities/provider_health'

# Test each provider individually
for provider in openai groq anthropic; do
  echo "Testing $provider..."
  curl -X POST http://localhost:8085/api/capabilities/compare_providers \
    -d "{\"text\":\"test\",\"provider\":\"$provider\"}"
done
```

## 📊 Monitoring & Metrics

### Key Metrics to Track

- **Fallback Rate**: Percentage of requests using fallback providers
- **Provider Health**: Real-time availability status
- **Response Times**: Latency per provider
- **Cost Per Request**: Token usage and associated costs
- **Quality Scores**: Response quality metrics

### Prometheus Metrics

```
ai_requests_total{provider="openai",status="success"}
ai_fallback_total{primary="openai",fallback="groq"} 
ai_response_duration_seconds{provider="groq"}
ai_provider_health{provider="anthropic",status="healthy"}
```

## 📚 Related Examples

- **[AI Tools Showcase](../ai-tools-showcase/)** - Built-in AI tools with single provider
- **[AI Agent Example](../ai-agent-example/)** - AI-first agent architecture
- **[Agent Example](../agent-example/)** - Basic agent patterns
- **[Tool Example](../tool-example/)** - Basic tool patterns

## 🎉 Key Takeaways

### Multi-Provider Benefits

- **🔄 High Availability**: Automatic failover prevents service interruptions
- **⚡ Performance Optimization**: Route tasks to optimal providers
- **💰 Cost Efficiency**: Use cost-effective providers as fallbacks
- **📈 Quality Improvement**: Compare providers for best results
- **🛡️ Risk Mitigation**: Reduce dependency on single provider

### Implementation Patterns

- **Tool Pattern**: Passive components with AI fallback capabilities
- **Agent Pattern**: Active orchestration with discovery and AI routing
- **Hybrid Deployments**: Run both patterns simultaneously
- **Health Monitoring**: Proactive failure detection and recovery

---

**Built with ❤️ using the GoMind Framework**  
*Demonstrating resilient multi-provider AI architecture with automatic fallback patterns for both tools and agents*