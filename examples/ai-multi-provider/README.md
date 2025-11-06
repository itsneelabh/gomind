# Multi-Provider AI with Fallback Example

A comprehensive demonstration of **GoMind's Universal AI Provider Architecture** with **automatic fallback patterns** for both tools and agents. This example showcases how GoMind's AI module enables seamless switching between 20+ AI providers while maintaining code simplicity and reliability.

## 🎯 What This Example Demonstrates

### GoMind's Universal Provider Strategy

This example leverages **GoMind's revolutionary approach** to AI provider management:

- **Universal OpenAI-Compatible Provider**: One implementation works with OpenAI, Groq, DeepSeek, xAI, Ollama, and 15+ other services
- **Native Provider Support**: Dedicated implementations for Anthropic Claude, Google Gemini, and AWS Bedrock
- **Automatic Detection**: The AI module automatically discovers available providers from environment variables
- **Zero Configuration**: Works out-of-the-box with simple environment variable setup
- **Provider Priority System**: Intelligently selects the best available provider based on configured priority

### Multi-Provider Architecture Patterns

| Pattern | Description | Benefits | Implementation |
|---------|-------------|----------|--------------|
| **🔄 Automatic Fallback** | Primary → Fallback → Secondary provider chain | High availability, fault tolerance | Tool & Agent modes |
| **⚖️ Provider Comparison** | Execute same task on multiple providers simultaneously | Quality comparison, best result selection | Tool mode |
| **🎯 Intelligent Routing** | Route requests to first available provider | Fast response, automatic failover | Agent mode |
| **🎼 Provider Orchestration** | Use different providers for different aspects of complex tasks | Optimize for each provider's strengths | Agent mode |
| **📊 Health Monitoring** | Real-time provider availability and performance testing | Proactive failure detection | Tool mode |

### Two Implementation Examples

This example includes **both tool and agent implementations**:

#### 🔧 **Multi-Provider Tool** (Port 8085)
- **Passive component** with multi-provider capabilities
- **2 core capabilities**: provider comparison and health monitoring
- **Parallel processing** for provider comparison
- **Real-time health checks** with performance metrics

#### 🤖 **Multi-Provider Agent** (Port 8086)
- **Active orchestration** with intelligent routing
- **2 core capabilities**: intelligent routing and provider orchestration
- **Automatic failover** with provider priority
- **Complex task distribution** across multiple providers

## 🏗️ Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│              GoMind Multi-Provider AI Architecture                  │
├─────────────────────────────────────────────────────────────────────┤
│  🔧 Tool (Port 8085)              🤖 Agent (Port 8086)             │
│  ┌──────────────────────────┐    ┌─────────────────────────────────┐ │
│  │ • compare_providers      │    │ • intelligent_routing           │ │
│  │ • provider_health        │    │ • provider_orchestration        │ │
│  └──────────────────────────┘    └─────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────────────┤
│                       GoMind AI Module (Universal Layer)            │
│         ┌──────────────────┬──────────────────┬─────────────────┐   │
│         │  Universal       │    Native        │    Native       │   │
│         │  OpenAI Provider │  Anthropic       │   Gemini        │   │
│         │                  │  Provider        │   Provider      │   │
│         │ • OpenAI         │ • Claude-3       │ • Gemini-Pro    │   │
│         │ • Groq           │ • Claude-3.5     │ • Gemini-1.5    │   │
│         │ • DeepSeek       │                  │                 │   │
│         │ • xAI Grok       │                  │                 │   │
│         │ • Ollama         │                  │                 │   │
│         │ • 15+ Others     │                  │                 │   │
│         └──────────────────┴──────────────────┴─────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

### Multi-Provider Configuration Logic

```
Environment Scanning:
┌─────────────────────┐
│ OPENAI_API_KEY?     │ ──► Primary Provider (OpenAI)
├─────────────────────┤
│ ANTHROPIC_API_KEY?  │ ──► Fallback Provider (Claude)
├─────────────────────┤
│ GEMINI_API_KEY?     │ ──► Secondary Provider (Gemini)
└─────────────────────┘
         │
         ▼
Auto-Detection: ai.NewClient() picks best available provider
```

### Provider Fallback Flow

```
Tool Mode: compare_providers (parallel execution)
Primary ┐
        ├─► Aggregate Results ──► Best Response
Fallback┘
Secondary

Agent Mode: intelligent_routing (sequential fallback)
Primary → (fail) → Fallback → (fail) → Secondary → (fail) → Error
```

## 🚀 Quick Start

### Prerequisites

**Minimum Requirements:**
- Go 1.25 or later
- **At least 1 AI provider API key** (for basic functionality)
- **At least 2 AI provider API keys** (for multi-provider fallback demonstration)

**This Example's Manual Provider Configuration:**
Unlike the AI module's automatic provider detection, this example demonstrates **manual multi-provider setup** by explicitly configuring three specific providers:

```bash
# Manual configuration - this example explicitly sets up these providers:
export OPENAI_API_KEY="sk-your-openai-key"        # Primary → OpenAI GPT-4
export ANTHROPIC_API_KEY="sk-ant-your-claude-key" # Fallback → Claude-3.5 Sonnet
export GEMINI_API_KEY="your-google-gemini-key"    # Secondary → Gemini-1.5-Pro
```

**Key Difference from AI Module Auto-Detection:**
- **AI Module Auto-Detection**: Uses priority system (OpenAI→Groq→DeepSeek→xAI→Qwen→Anthropic→Gemini)
- **This Example**: Uses manual fallback chain (OpenAI→Anthropic→Gemini) regardless of priority

**Minimum Configuration** (at least one provider required):
```bash
# At minimum, set one of these:
export OPENAI_API_KEY="sk-your-key"           # Will be primary provider
export ANTHROPIC_API_KEY="sk-ant-your-key"    # Will be fallback provider
export GEMINI_API_KEY="your-google-key"       # Will be secondary provider
```

**Auto-Detection Fallback:** If no explicit providers are configured, the example falls back to `ai.NewClient()` auto-detection.

### 📋 Step-by-Step Setup & Validation

#### Step 1: Validate Your Provider Configuration

**Check which providers are detected:**
```bash
# Navigate to the example
cd examples/ai-multi-provider

# Install dependencies
go mod tidy

# Test GoMind's provider auto-detection
go run -c 'import "github.com/itsneelabh/gomind/ai"; providers := ai.GetProviderInfo(); for _, p := range providers { fmt.Printf("%s: available=%v, priority=%d\n", p.Name, p.Available, p.Priority) }'
```

**Verify your specific API keys:**
```bash
# Check what providers this example will use
echo "Checking provider configuration..."
echo "OPENAI_API_KEY: ${OPENAI_API_KEY:+SET}"
echo "ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:+SET}"
echo "GEMINI_API_KEY: ${GEMINI_API_KEY:+SET}"
echo "GROQ_API_KEY: ${GROQ_API_KEY:+SET}"
echo "DEEPSEEK_API_KEY: ${DEEPSEEK_API_KEY:+SET}"
```

#### Step 2: Start the Multi-Provider Services

**Option A: Tool Mode Only** (Port 8085)
```bash
export DEPLOYMENT_MODE="tool"
export TOOL_PORT="8085"
go run main.go
```

**Expected Output:**
```
✅ Primary AI provider (OpenAI) configured
✅ Fallback AI provider (Anthropic) configured
✅ Secondary AI provider (Gemini) configured
🔧 Starting in Tool mode...
🚀 Multi-Provider AI Tool starting on port 8085
```

**Option B: Agent Mode Only** (Port 8086)
```bash
export DEPLOYMENT_MODE="agent"
export AGENT_PORT="8086"
go run main.go
```

**Option C: Both Modes Simultaneously**
```bash
export DEPLOYMENT_MODE="both"
export TOOL_PORT="8085"
export AGENT_PORT="8086"
go run main.go
```

## 🧪 Comprehensive Testing Strategy

This section provides step-by-step testing to validate all multi-provider functionality and ensure the example works correctly with your provider configuration.

### Phase 1: Provider Health Validation

#### Test 1: Verify Provider Detection and Health

**Start the tool service** (if not already running):
```bash
export DEPLOYMENT_MODE="tool"
export TOOL_PORT="8085"
go run main.go &
sleep 5  # Wait for startup
```

**Check all providers' health:**
```bash
curl -X POST http://localhost:8085/api/capabilities/provider_health \
  -H "Content-Type: application/json" \
  -d '{}'
```

**Expected Response Structure:**
```json
{
  "results": {
    "primary": {
      "available": true,
      "response_time": "1.234s",
      "error": null
    },
    "fallback": {
      "available": true,
      "response_time": "2.567s",
      "error": null
    },
    "secondary": {
      "available": false,
      "response_time": "0s",
      "error": "API key not provided"
    }
  },
  "success": true,
  "timestamp": "2025-01-XX..."
}
```

**What this tells you:**
- `available: true` = Provider is working correctly
- `available: false` + `error` = Provider configuration issue
- `response_time` = Performance comparison between providers

#### Test 2: Single Provider Failover Testing

**Test what happens when providers fail:**
```bash
# Temporarily rename your primary API key to simulate failure
export OPENAI_API_KEY_BACKUP="$OPENAI_API_KEY"
export OPENAI_API_KEY=""

# Restart the service to pick up the change
pkill -f "go run main.go" || true
sleep 2
go run main.go &
sleep 5

# Test health again - should show primary as unavailable
curl -X POST http://localhost:8085/api/capabilities/provider_health -d '{}'

# Restore the API key
export OPENAI_API_KEY="$OPENAI_API_KEY_BACKUP"
```

### Phase 2: Multi-Provider Tool Testing

#### Test 3: Provider Comparison (Core Tool Capability)

**Restart with all providers** (restore your configuration):
```bash
pkill -f "go run main.go" || true
sleep 2
export DEPLOYMENT_MODE="tool"
go run main.go &
sleep 5
```

**Test parallel provider comparison:**
```bash
curl -X POST http://localhost:8085/api/capabilities/compare_providers \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Write a haiku about programming"
  }'
```

**Expected Response Structure:**
```json
{
  "results": {
    "primary": {
      "content": "Code flows like a stream\nLogic branches through the mind\nBugs hide, then are found",
      "model": "gpt-4",
      "usage": {"total_tokens": 45}
    },
    "fallback": {
      "content": "Variables dance free\nIn loops and functions they play\nSoftware comes to life",
      "model": "claude-3-5-sonnet-20241022",
      "usage": {"total_tokens": 42}
    },
    "secondary": {
      "content": "Syntax guides the way\nLogic unfolds line by line\nMachine understands",
      "model": "gemini-1.5-pro",
      "usage": {"total_tokens": 38}
    }
  },
  "providers": ["primary", "fallback", "secondary"],
  "success": true,
  "timestamp": "..."
}
```

**Validation Questions:**
- ✅ Did all configured providers respond?
- ✅ Are the response formats different but valid?
- ✅ Do you see different `model` names for each provider?

#### Test 4: Performance and Response Time Comparison

**Test with a more complex prompt:**
```bash
curl -X POST http://localhost:8085/api/capabilities/compare_providers \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Explain the differences between microservices and monolithic architecture, including pros and cons of each approach."
  }'
```

**Analysis:** Check the response times and content quality differences between providers. Look for:
- Which provider responds fastest?
- Which provides the most comprehensive answer?
- Are there clear differences in writing style?

**Expected Response Structure:**
```json
{
  "results": {
    "primary": {
      "content": "Microservices architecture breaks applications into...",
      "model": "gpt-4",
      "usage": {"total_tokens": 245}
    },
    "fallback": {
      "content": "Monolithic vs microservices represents a fundamental...",
      "model": "claude-3-5-sonnet-20241022",
      "usage": {"total_tokens": 267}
    },
    "secondary": {
      "content": "The choice between microservices and monolithic...",
      "model": "gemini-1.5-pro",
      "usage": {"total_tokens": 198}
    }
  },
  "providers": ["primary", "fallback", "secondary"],
  "success": true,
  "timestamp": "2025-01-XX..."
}
```

### Phase 3: Multi-Provider Agent Testing

#### Test 5: Intelligent Routing (Core Agent Capability)

**Switch to agent mode:**
```bash
pkill -f "go run main.go" || true
sleep 2
export DEPLOYMENT_MODE="agent"
export AGENT_PORT="8086"
go run main.go &
sleep 5
```

**Test intelligent routing with automatic failover:**
```bash
curl -X POST http://localhost:8086/api/capabilities/intelligent_routing \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "What are the key principles of clean code?"
  }'
```

**Expected Response Structure:**
```json
{
  "results": {
    "content": "Clean code principles include: 1. Meaningful names...",
    "model": "gpt-4",
    "usage": {"total_tokens": 234}
  },
  "provider": "primary",
  "success": true,
  "timestamp": "..."
}
```

**Key Validation:**
- ✅ Did it route to the first available provider?
- ✅ Does the `provider` field show which one was used?
- ✅ Is the response coherent and complete?

#### Test 6: Provider Orchestration

**Test complex task distribution across multiple providers:**
```bash
curl -X POST http://localhost:8086/api/capabilities/provider_orchestration \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Design a scalable web application architecture"
  }'
```

**Expected Response Structure:**
```json
{
  "results": {
    "analysis": "Technical analysis from primary provider...",
    "creative": "Creative perspective from fallback provider...",
    "summary": "Summary from secondary provider..."
  },
  "providers": ["primary", "fallback", "secondary"],
  "success": true,
  "timestamp": "..."
}
```

**What this demonstrates:**
- Each provider handles a different aspect of the complex task
- Primary: Technical analysis (lower temperature = more focused)
- Fallback: Creative perspective (higher temperature = more creative)
- Secondary: Summarization (balanced approach)

### Phase 4: Edge Case and Failure Testing

#### Test 7: Sequential Provider Failure

**Simulate primary provider failure:**
```bash
# Temporarily disable primary provider
export OPENAI_API_KEY_BACKUP="$OPENAI_API_KEY"
export OPENAI_API_KEY=""

# Restart agent to pick up change
pkill -f "go run main.go" || true
sleep 2
export DEPLOYMENT_MODE="agent"
go run main.go &
sleep 5

# Test intelligent routing - should automatically use fallback
curl -X POST http://localhost:8086/api/capabilities/intelligent_routing \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "This should use the fallback provider"
  }'
```

**Expected Response:**
```json
{
  "results": {
    "content": "I can respond using the fallback provider...",
    "model": "claude-3-5-sonnet-20241022",
    "usage": {"total_tokens": 87}
  },
  "provider": "fallback",
  "success": true,
  "timestamp": "..."
}
```

**Expected behavior:**
- Should automatically route to fallback provider (Anthropic)
- Response should show `"provider": "fallback"`
- No error messages for the user

#### Test 8: All Providers Failure Scenario

**Simulate all providers failing:**
```bash
# Temporarily disable all providers
export ANTHROPIC_API_KEY_BACKUP="$ANTHROPIC_API_KEY"
export GEMINI_API_KEY_BACKUP="$GEMINI_API_KEY"
export OPENAI_API_KEY=""
export ANTHROPIC_API_KEY=""
export GEMINI_API_KEY=""

# Restart service
pkill -f "go run main.go" || true
sleep 2
go run main.go &
sleep 5

# This should fail gracefully
curl -X POST http://localhost:8086/api/capabilities/intelligent_routing \
  -d '{"prompt": "This should fail gracefully"}'
```

**Expected Response:**
```json
{
  "success": false,
  "error": "All AI providers unavailable",
  "timestamp": "..."
}
```

**Restore providers:**
```bash
export OPENAI_API_KEY="$OPENAI_API_KEY_BACKUP"
export ANTHROPIC_API_KEY="$ANTHROPIC_API_KEY_BACKUP"
export GEMINI_API_KEY="$GEMINI_API_KEY_BACKUP"
```

#### Test 9: Both Modes Simultaneously

**Test running both tool and agent together:**
```bash
pkill -f "go run main.go" || true
sleep 2
export DEPLOYMENT_MODE="both"
export TOOL_PORT="8085"
export AGENT_PORT="8086"
go run main.go &
sleep 10  # Extra time for both services to start

# Test tool service
curl -X POST http://localhost:8085/api/capabilities/provider_health -d '{}'

# Test agent service
curl -X POST http://localhost:8086/api/capabilities/intelligent_routing \
  -d '{"prompt": "Both services should work simultaneously"}'
```

**Expected:** Both services should respond successfully on their respective ports.

### Phase 5: Testing Summary and Validation Checklist

#### ✅ Complete Testing Checklist

After running all tests, validate these key behaviors:

**Basic Functionality:**
- [ ] Provider health check shows all configured providers
- [ ] Provider comparison returns responses from all available providers
- [ ] Intelligent routing successfully routes to first available provider
- [ ] Provider orchestration distributes tasks across multiple providers

**Failure Handling:**
- [ ] Service continues working when primary provider fails
- [ ] Graceful error handling when all providers fail
- [ ] Automatic failover without user-visible errors
- [ ] Services restart properly after provider configuration changes

**Performance & Reliability:**
- [ ] Response times are reasonable for your providers
- [ ] Both tool and agent modes can run simultaneously
- [ ] Provider comparison shows clear differences in responses
- [ ] Health checks accurately reflect provider availability

#### 🛠️ Automated Testing Script

**Create a comprehensive test script:**
```bash
#!/bin/bash
# Save as: test-multi-provider.sh

echo "🧪 Starting Multi-Provider AI Testing Suite"
echo "============================================"

# Check prerequisites
echo "📋 Checking prerequisites..."
echo "OPENAI_API_KEY: ${OPENAI_API_KEY:+SET}"
echo "ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:+SET}"
echo "GEMINI_API_KEY: ${GEMINI_API_KEY:+SET}"

# Start tool service
echo "🔧 Starting tool service..."
export DEPLOYMENT_MODE="tool"
export TOOL_PORT="8085"
go run main.go &
TOOL_PID=$!
sleep 8

# Test 1: Provider health
echo "🏥 Testing provider health..."
curl -s -X POST http://localhost:8085/api/capabilities/provider_health -d '{}' | jq '.results | keys'

# Test 2: Provider comparison
echo "⚖️ Testing provider comparison..."
curl -s -X POST http://localhost:8085/api/capabilities/compare_providers \
  -H "Content-Type: application/json" \
  -d '{"prompt":"Hello AI"}' | jq '.providers'

# Switch to agent mode
echo "🤖 Switching to agent mode..."
kill $TOOL_PID 2>/dev/null
sleep 3
export DEPLOYMENT_MODE="agent"
export AGENT_PORT="8086"
go run main.go &
AGENT_PID=$!
sleep 8

# Test 3: Intelligent routing
echo "🎯 Testing intelligent routing..."
curl -s -X POST http://localhost:8086/api/capabilities/intelligent_routing \
  -H "Content-Type: application/json" \
  -d '{"prompt":"Route this request"}' | jq '.provider'

# Test 4: Provider orchestration
echo "🎼 Testing provider orchestration..."
curl -s -X POST http://localhost:8086/api/capabilities/provider_orchestration \
  -H "Content-Type: application/json" \
  -d '{"prompt":"Orchestrate this task"}' | jq '.results | keys'

# Cleanup
echo "🧹 Cleaning up..."
kill $AGENT_PID 2>/dev/null
kill $TOOL_PID 2>/dev/null

echo "✅ Multi-Provider AI testing complete!"
```

**Run the automated test:**
```bash
chmod +x test-multi-provider.sh
./test-multi-provider.sh
```

## 🚨 Troubleshooting Guide

### Common Issues and Solutions

#### ❌ Issue: "No AI providers available"
**Cause:** No valid API keys found
```bash
# Check your environment variables
env | grep -E "(OPENAI|ANTHROPIC|GEMINI|GROQ|DEEPSEEK)_API_KEY"
```
**Solution:** Set at least one valid API key and restart the service

#### ❌ Issue: Provider health shows "available: false"
**Possible causes and solutions:**
```bash
# 1. Invalid API key format
echo "Check API key format:"
echo "OpenAI: sk-proj-... or sk-..."
echo "Anthropic: sk-ant-..."
echo "Gemini: Any alphanumeric string"

# 2. API quota exceeded
curl -s -X POST http://localhost:8085/api/capabilities/provider_health -d '{}' | jq '.results'

# 3. Network connectivity
curl -I https://api.openai.com/v1/models
curl -I https://api.anthropic.com/v1/messages
```

#### ❌ Issue: Only one provider responding in comparison tests
**Diagnosis:**
```bash
# Check which providers are actually configured
curl -s -X POST http://localhost:8085/api/capabilities/provider_health -d '{}' | \
  jq '.results | to_entries[] | select(.value.available == true) | .key'
```
**Solution:** Configure additional providers or verify existing API keys

#### ❌ Issue: "Connection refused" errors
```bash
# Check if service is running
ps aux | grep "go run main.go"

# Check port availability
lsof -ti:8085
lsof -ti:8086

# Restart with logging
go run main.go 2>&1 | tee multi-provider.log
```

#### ❌ Issue: Inconsistent responses between test runs
**Expected behavior:** Different providers will give different responses to the same prompt
**Validation:** Check that the `provider` field in responses shows different values

### Performance Optimization

#### Slow Response Times
```bash
# Check individual provider performance
time curl -s -X POST http://localhost:8085/api/capabilities/provider_health -d '{}'

# Monitor which providers are fastest
curl -s -X POST http://localhost:8085/api/capabilities/compare_providers \
  -H "Content-Type: application/json" \
  -d '{"prompt":"Quick test"}' | jq '.results | to_entries[] | {provider: .key, content_length: (.value.content | length)}'
```

#### Memory Usage
```bash
# Monitor memory usage during testing
ps aux | grep "go run main.go" | awk '{print $4 " " $6 " " $11}'

# For production deployment, build binary instead of using go run
go build -o multi-provider-ai main.go
./multi-provider-ai
```

## 🔧 Configuration Reference

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `DEPLOYMENT_MODE` | Service mode: `tool`, `agent`, or `both` | `tool` | No |
| `TOOL_PORT` | Tool service port | 8085 | No |
| `AGENT_PORT` | Agent service port | 8086 | No |
| `OPENAI_API_KEY` | OpenAI API key (Primary provider) | - | Recommended |
| `ANTHROPIC_API_KEY` | Anthropic Claude API key (Fallback) | - | Recommended |
| `GEMINI_API_KEY` | Google Gemini API key (Secondary) | - | Optional |

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
docker build -t multi-provider-ai:latest .

# 3. Load image into Kind cluster
kind load docker-image multi-provider-ai:latest --name gomind-multi-provider

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
# Test tool provider comparison
curl -X POST http://localhost:8085/api/capabilities/compare_providers \
  -H "Content-Type: application/json" \
  -d '{"prompt":"Test message for comparison"}'

# Test agent intelligent routing
curl -X POST http://localhost:8086/api/capabilities/intelligent_routing \
  -H "Content-Type: application/json" \
  -d '{"prompt":"Test message for routing"}'

# Test provider health
curl -X POST http://localhost:8085/api/capabilities/provider_health \
  -H "Content-Type: application/json" \
  -d '{}'
```

### Load Testing

```bash
# Concurrent requests to test provider comparison under load
for i in {1..50}; do
  curl -X POST http://localhost:8085/api/capabilities/compare_providers \
    -H "Content-Type: application/json" \
    -d "{\"prompt\":\"Load test message $i\"}" &
done
wait

# Test agent routing under load
for i in {1..20}; do
  curl -X POST http://localhost:8086/api/capabilities/intelligent_routing \
    -H "Content-Type: application/json" \
    -d "{\"prompt\":\"Routing test $i\"}" &
done
wait
```

### Failover Testing

```bash
# Temporarily disable primary provider to test agent routing fallback
export OPENAI_API_KEY_TEMP="$OPENAI_API_KEY"
export OPENAI_API_KEY=""

# Restart agent service to pick up the change
pkill -f "go run main.go" || true
export DEPLOYMENT_MODE="agent"
go run main.go &
sleep 5

# Test intelligent routing - should use fallback provider
curl -X POST http://localhost:8086/api/capabilities/intelligent_routing \
  -H "Content-Type: application/json" \
  -d '{"prompt":"Failover test - should use fallback provider"}'

# Restore API key
export OPENAI_API_KEY="$OPENAI_API_KEY_TEMP"
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
# Use provider comparison to identify fastest provider
curl -X POST http://localhost:8085/api/capabilities/compare_providers \
  -H "Content-Type: application/json" \
  -d '{"prompt":"Quick response test"}'

# Use intelligent routing for automatic provider selection
curl -X POST http://localhost:8086/api/capabilities/intelligent_routing \
  -H "Content-Type: application/json" \
  -d '{"prompt":"Route to best available provider"}'
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