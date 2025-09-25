# Enhanced Research Agent Example

A sophisticated demonstration of **progressive AI integration** into agent capabilities. This example shows how to enhance existing agents by adding AI to specific capabilities while maintaining backward compatibility and demonstrating multiple AI integration patterns.

## 🎯 What This Example Demonstrates

### Progressive AI Enhancement Strategy

This example represents the **evolution path** for teams who:
- Have existing agent-based systems
- Want to **gradually add AI** without breaking existing functionality  
- Need to see **multiple AI integration patterns** in one codebase
- Want to understand **when and how** to use AI in different scenarios

### 4 Distinct AI Integration Patterns

Each capability demonstrates a different approach to AI integration:

#### 1. 🎯 **Tool Discovery + AI Synthesis**
```
Request → Discover Tools → Call Tools → AI Analyzes Results → Synthesized Response
```
- **Use Case**: Enhanced research with intelligent synthesis
- **Pattern**: Traditional tool coordination enhanced with AI analysis
- **When to Use**: When you want to keep existing tool integrations but add intelligent insights

#### 2. 🧠 **AI Categorization + Recommendations** 
```
Request → Discover Services → AI Categorizes → AI Recommends → Enhanced Response
```
- **Use Case**: Service discovery with intelligent recommendations
- **Pattern**: AI adds context and guidance to discovery results
- **When to Use**: When users need help understanding available options

#### 3. 💭 **Pure AI Analysis**
```
Request → Direct AI Processing → Analysis → Insights → Response
```
- **Use Case**: Direct AI-powered data analysis
- **Pattern**: Traditional AI integration for cognitive tasks
- **When to Use**: For tasks requiring intelligence, reasoning, or creative analysis

#### 4. 🎭 **AI Workflow Orchestration**
```
Request → AI Plans Workflow → Execute Steps → AI Synthesizes → Response
```
- **Use Case**: Complex multi-step processes with AI planning
- **Pattern**: AI creates and manages execution plans
- **When to Use**: For complex workflows that benefit from intelligent planning

## 🏗️ Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                Enhanced Research Agent                           │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────┐ │
│  │ Tool Disco  │  │ AI Category │  │ Pure AI     │  │ AI Flow │ │
│  │ + AI Synth  │  │ + Recommend │  │ Analysis    │  │ Orchestr│ │
│  │             │  │             │  │             │  │         │ │
│  │ Pattern 1   │  │ Pattern 2   │  │ Pattern 3   │  │ Pattern4│ │
│  │ Traditional │  │ AI-Enhanced │  │ AI-Native   │  │ AI-Plan │ │
│  │ → Enhanced  │  │ Discovery   │  │ Processing  │  │ Execute │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────┘ │
├─────────────────────────────────────────────────────────────────┤
│                    AI Integration Layer                         │
│   Auto-Detection • Multi-Provider • Graceful Degradation      │
├─────────────────────────────────────────────────────────────────┤
│                   GoMind Core Framework                         │
│   Discovery • Registry • Capabilities • Service Management    │
└─────────────────────────────────────────────────────────────────┘
```

## 🚀 Quick Start

### Prerequisites

- Go 1.25 or later
- At least one AI provider API key:
  - OpenAI API key (`OPENAI_API_KEY`)
  - Groq API key (`GROQ_API_KEY`) 
  - Anthropic API key (`ANTHROPIC_API_KEY`)
- Redis (optional, for service discovery)
- Docker (optional, for containerized deployment)

### 1. Local Development Setup

```bash
# Navigate to the enhanced agent example
cd examples/agent-example-enhanced

# Install dependencies
go mod tidy

# Set up AI provider (choose one or more)
export OPENAI_API_KEY="your-openai-key-here"
export GROQ_API_KEY="your-groq-key-here"        # Optional alternative
export ANTHROPIC_API_KEY="your-claude-key-here" # Optional alternative

# Optional: Configure advanced settings
export AI_PROVIDER="openai"        # openai, groq, anthropic (auto-detected)
export RESEARCH_DEPTH="comprehensive"
export MAX_CONCURRENT_REQUESTS="10"

# Optional: Enable service discovery
export REDIS_URL="redis://localhost:6379"

# Run the enhanced research agent
go run main.go
```

The service will start with automatic AI provider detection:

```
2024/01/15 10:30:00 🚀 Starting Enhanced Research Agent on port 8080
2024/01/15 10:30:00 🧠 AI Provider: OpenAI GPT-4 (auto-detected)
2024/01/15 10:30:00 ⚡ All 4 capabilities are AI-enhanced
2024/01/15 10:30:00 🎯 Pattern demonstrations:
  - Tool Discovery + AI Synthesis
  - AI Categorization + Recommendations  
  - Pure AI Analysis
  - AI Workflow Orchestration

Available endpoints:
  GET  /health - Service health check
  POST /api/capabilities/ai_research_topic - AI-enhanced topic research
  POST /api/capabilities/ai_discover_services - AI service discovery
  POST /api/capabilities/ai_analyze_data - Pure AI data analysis
  POST /api/capabilities/ai_orchestrate_workflow - AI workflow planning
  GET  /api/capabilities - List all capabilities
```

### 2. Docker Deployment

```bash
# Build the Docker image
docker build -t gomind/research-agent-enhanced:latest .

# Run with AI provider configuration
docker run -d \
  --name research-agent-enhanced \
  -p 8080:8080 \
  -p 9090:9090 \
  -e OPENAI_API_KEY="your-key-here" \
  -e RESEARCH_DEPTH="comprehensive" \
  -e REDIS_URL="redis://your-redis:6379" \
  gomind/research-agent-enhanced:latest
```

### 3. Kubernetes Deployment

```bash
# Update secrets in k8-deployment.yaml with your API keys
kubectl apply -f k8-deployment.yaml

# Check deployment status
kubectl get pods -n gomind-enhanced-agent

# Access via port-forward
kubectl port-forward svc/enhanced-agent-service 8080:80 -n gomind-enhanced-agent
```

## 📖 Usage Examples & Patterns

### Pattern 1: Tool Discovery + AI Synthesis

**Use Case**: Research a topic using available tools, then use AI to synthesize intelligent insights.

```bash
curl -X POST http://localhost:8080/api/capabilities/ai_research_topic \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "microservices architecture patterns",
    "depth": "comprehensive",
    "use_ai": true,
    "context": {
      "industry": "fintech",
      "scale": "enterprise"
    }
  }'
```

**What Happens:**
1. 🔍 **Discovery**: Finds available research tools (weather-service, data-analyzers, etc.)
2. 🔄 **Coordination**: Calls relevant tools to gather raw data
3. 🧠 **AI Synthesis**: AI analyzes all results and provides intelligent insights
4. 📋 **Response**: Comprehensive report with AI-generated recommendations

**Response Example:**
```json
{
  "synthesis": "Based on comprehensive analysis of microservices patterns for fintech...",
  "tools_used": ["architecture-analyzer", "pattern-repository"],
  "ai_insights": [
    "Event sourcing is particularly valuable for financial audit trails",
    "Consider CQRS for read-heavy financial reporting systems"
  ],
  "recommendations": ["Implement circuit breakers for payment processing"],
  "confidence_score": 0.92
}
```

### Pattern 2: AI Categorization + Recommendations

**Use Case**: Discover available services and get AI-powered guidance on optimal usage.

```bash
curl -X POST http://localhost:8080/api/capabilities/ai_discover_services \
  -H "Content-Type: application/json" \
  -d '{
    "service_type": "data-processing",
    "requirements": {
      "performance": "high",
      "scalability": "horizontal", 
      "data_type": "time-series"
    }
  }'
```

**What Happens:**
1. 🔍 **Discovery**: Finds all available services in the ecosystem
2. 🧠 **AI Categorization**: AI categorizes services by type, capability, and use case
3. 💡 **AI Recommendations**: AI provides usage guidance based on requirements
4. 📊 **Enhanced Response**: Services with AI-generated context and recommendations

**Response Example:**
```json
{
  "discovered_services": [
    {
      "name": "time-series-processor",
      "category": "data-processing",
      "ai_recommendation": "Optimal for high-frequency trading data",
      "use_cases": ["real-time analytics", "anomaly detection"],
      "compatibility_score": 0.95
    }
  ],
  "ai_insights": {
    "best_match": "time-series-processor",
    "reasoning": "Designed for high-performance time-series with horizontal scaling",
    "integration_tips": ["Use batch processing for historical analysis"]
  }
}
```

### Pattern 3: Pure AI Analysis

**Use Case**: Direct AI processing for tasks requiring intelligence and reasoning.

```bash
curl -X POST http://localhost:8080/api/capabilities/ai_analyze_data \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "customer_feedback": [
        "Love the new features but app crashes sometimes",
        "Great UI improvements, very intuitive now",
        "Performance issues during peak hours"
      ]
    },
    "analysis_type": "sentiment_and_insights"
  }'
```

**What Happens:**
1. 🧠 **Direct AI Processing**: Raw data goes straight to AI for analysis
2. 🎯 **Intelligent Analysis**: AI performs sentiment analysis, pattern recognition, insights
3. 📊 **Structured Output**: AI provides structured results with confidence scores
4. 💡 **Actionable Insights**: AI generates specific recommendations

**Response Example:**
```json
{
  "sentiment_analysis": {
    "overall_sentiment": "mixed_positive",
    "sentiment_score": 0.6,
    "breakdown": {
      "positive": 2,
      "negative": 1,
      "neutral": 0
    }
  },
  "key_insights": [
    "Users appreciate UI improvements (100% positive mention)",
    "Technical issues are primary concern (crashes, performance)",
    "Feature satisfaction is high despite technical problems"
  ],
  "recommendations": [
    "Prioritize stability and performance optimization",
    "Investigate crash patterns during peak usage",
    "Continue UI/UX improvements as they're well-received"
  ],
  "confidence": 0.89
}
```

### Pattern 4: AI Workflow Orchestration

**Use Case**: Complex multi-step workflows where AI plans and manages execution.

```bash
curl -X POST http://localhost:8080/api/capabilities/ai_orchestrate_workflow \
  -H "Content-Type: application/json" \
  -d '{
    "goal": "analyze competitor product features and market positioning",
    "constraints": {
      "time_limit": "30m",
      "data_sources": ["public_web", "api_data"],
      "analysis_depth": "comprehensive"
    },
    "context": {
      "our_product": "project-management-saas",
      "target_competitors": ["asana", "notion", "monday"]
    }
  }'
```

**What Happens:**
1. 🧠 **AI Planning**: AI analyzes the goal and creates an optimal execution plan
2. 🔄 **Dynamic Execution**: AI executes the plan, adapting based on intermediate results
3. 📊 **Progress Monitoring**: AI monitors progress and adjusts strategy if needed
4. 🎯 **Final Synthesis**: AI synthesizes all results into actionable insights

**Response Example:**
```json
{
  "execution_plan": {
    "total_steps": 6,
    "estimated_duration": "25m",
    "strategy": "parallel_analysis_with_synthesis"
  },
  "results": {
    "competitor_analysis": {
      "asana": {
        "key_features": ["timeline view", "custom fields", "proofing"],
        "market_position": "enterprise-focused with strong project tracking"
      }
    },
    "market_gaps": [
      "Limited AI-powered project insights",
      "Weak integration with design tools"
    ],
    "opportunities": [
      "AI-powered project risk prediction", 
      "Enhanced creative workflow integration"
    ]
  },
  "ai_recommendations": [
    "Focus on AI-enhanced project intelligence as differentiator",
    "Develop stronger design tool integrations"
  ]
}
```

## 🎨 AI Integration Patterns Deep Dive

### When to Use Each Pattern

| Pattern | Best For | Complexity | AI Dependency | Backward Compatibility |
|---------|----------|------------|---------------|----------------------|
| **Tool + AI Synthesis** | Enhancing existing workflows | Low | Optional | Perfect |
| **AI Categorization** | User guidance and discovery | Medium | Recommended | High |
| **Pure AI Analysis** | Cognitive tasks | Low | Required | Medium |
| **AI Orchestration** | Complex workflows | High | Required | Low |

### Implementation Strategy

#### Phase 1: Start with Tool + AI Synthesis
```go
// Existing capability with optional AI enhancement
func (r *ResearchAgent) handleResearch(ctx context.Context, req interface{}) (interface{}, error) {
    // Traditional logic
    results := r.gatherDataFromTools(ctx, req)
    
    // Optional AI enhancement
    if r.aiClient != nil && req.UseAI {
        insights := r.aiClient.GenerateResponse(ctx, synthesisPrompt)
        results.AIInsights = insights
    }
    
    return results, nil
}
```

#### Phase 2: Add AI Categorization
```go
// AI adds context to existing functionality
func (r *ResearchAgent) handleDiscovery(ctx context.Context, req interface{}) (interface{}, error) {
    services := r.discoverServices(ctx) // Existing logic
    
    // AI adds categorization and recommendations
    if r.aiClient != nil {
        categories := r.aiClient.CategorizeServices(ctx, services)
        recommendations := r.aiClient.GenerateRecommendations(ctx, services, req)
    }
    
    return enhancedResults, nil
}
```

#### Phase 3: Pure AI Capabilities
```go
// New AI-native capabilities
func (r *ResearchAgent) handleAIAnalysis(ctx context.Context, req interface{}) (interface{}, error) {
    // Directly use AI for cognitive tasks
    analysis := r.aiClient.AnalyzeData(ctx, req.Data, req.AnalysisType)
    insights := r.aiClient.GenerateInsights(ctx, analysis)
    
    return aiResults, nil
}
```

#### Phase 4: AI Orchestration
```go
// AI plans and manages complex workflows
func (r *ResearchAgent) handleOrchestration(ctx context.Context, req interface{}) (interface{}, error) {
    // AI creates execution plan
    plan := r.aiClient.CreateExecutionPlan(ctx, req.Goal)
    
    // AI executes and adapts
    results := r.executeAIPlan(ctx, plan)
    
    return synthesizedResults, nil
}
```

## 🔧 Configuration Options

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `OPENAI_API_KEY` | OpenAI API key | - | Yes (or alternative) |
| `GROQ_API_KEY` | Groq API key | - | Alternative |
| `ANTHROPIC_API_KEY` | Anthropic Claude API key | - | Alternative |
| `AI_PROVIDER` | Preferred AI provider | auto-detect | No |
| `RESEARCH_DEPTH` | Research depth level | `standard` | No |
| `MAX_CONCURRENT_REQUESTS` | Max parallel AI requests | `10` | No |
| `CACHE_ENABLED` | Enable response caching | `true` | No |
| `PORT` | HTTP server port | `8080` | No |

### AI Provider Auto-Detection

The agent automatically detects available AI providers in this order:
1. **OpenAI** - If `OPENAI_API_KEY` is set
2. **Groq** - If `GROQ_API_KEY` is set (faster inference)
3. **Anthropic** - If `ANTHROPIC_API_KEY` is set  
4. **Custom** - If custom provider is configured

### Graceful Degradation

```go
// Agent works even without AI
if r.aiClient == nil {
    r.Logger.Warn("AI client not available, using basic responses")
    return basicResponse, nil
}

// AI-enhanced response
enhancedResponse := r.generateAIResponse(ctx, request)
return enhancedResponse, nil
```

## 📊 Capability Matrix

| Capability | Traditional Logic | AI Enhancement | Fallback Available | Use Case |
|------------|-------------------|----------------|-------------------|----------|
| **ai_research_topic** | Tool coordination | Result synthesis | ✅ Basic results | Research with insights |
| **ai_discover_services** | Service discovery | Categorization & recommendations | ✅ Raw discovery | Guided service selection |
| **ai_analyze_data** | - | Full AI processing | ❌ AI required | Cognitive analysis |
| **ai_orchestrate_workflow** | - | Planning & execution | ❌ AI required | Complex workflows |

## 🧪 Testing the Enhanced Patterns

### Test Pattern 1: Tool + AI Synthesis
```bash
# Test with AI enabled
curl -X POST http://localhost:8080/api/capabilities/ai_research_topic \
  -d '{"topic": "cloud security", "use_ai": true}'

# Test without AI (fallback)  
curl -X POST http://localhost:8080/api/capabilities/ai_research_topic \
  -d '{"topic": "cloud security", "use_ai": false}'
```

### Test Pattern 2: AI Categorization
```bash
curl -X POST http://localhost:8080/api/capabilities/ai_discover_services \
  -d '{"service_type": "data-processing", "get_recommendations": true}'
```

### Test Pattern 3: Pure AI Analysis
```bash
curl -X POST http://localhost:8080/api/capabilities/ai_analyze_data \
  -d '{"data": {"metrics": [1,2,3,4,5]}, "analysis_type": "trend"}'
```

### Test Pattern 4: AI Orchestration
```bash
curl -X POST http://localhost:8080/api/capabilities/ai_orchestrate_workflow \
  -d '{"goal": "competitive analysis", "constraints": {"time": "15m"}}'
```

## 🔍 Monitoring and Observability

### Health Check with AI Status
```bash
curl http://localhost:8080/health
```

**Response:**
```json
{
  "status": "healthy",
  "service": "research-agent-enhanced",
  "ai_provider": "openai",
  "capabilities": 4,
  "ai_enhanced_capabilities": 4,
  "patterns_demonstrated": [
    "tool-discovery-ai-synthesis",
    "ai-categorization-recommendations", 
    "pure-ai-analysis",
    "ai-workflow-orchestration"
  ]
}
```

### Capability List with Enhancement Status
```bash
curl http://localhost:8080/api/capabilities
```

**Response shows which capabilities are AI-enhanced:**
```json
{
  "capabilities": [
    {
      "name": "ai_research_topic",
      "pattern": "tool-discovery-ai-synthesis",
      "ai_required": false,
      "fallback_available": true
    },
    {
      "name": "ai_analyze_data", 
      "pattern": "pure-ai-analysis",
      "ai_required": true,
      "fallback_available": false
    }
  ]
}
```

## 🚀 Migration from Basic Agent

### Step-by-Step Enhancement Guide

#### 1. Add AI Client to Existing Agent
```go
// Existing agent
type ResearchAgent struct {
    *core.BaseAgent
}

// Enhanced agent
type ResearchAgent struct {
    *core.BaseAgent
    aiClient ai.AIClient  // Add AI client
}
```

#### 2. Make AI Optional in Existing Capabilities
```go
func (r *ResearchAgent) handleResearch(ctx context.Context, req interface{}) (interface{}, error) {
    results := r.traditionalResearch(ctx, req)
    
    // Optional AI enhancement
    if r.aiClient != nil && shouldUseAI(req) {
        results.AIInsights = r.generateAIInsights(ctx, results)
    }
    
    return results, nil
}
```

#### 3. Add New AI-Native Capabilities
```go
func (r *ResearchAgent) registerCapabilities() {
    // Existing capabilities (enhanced)
    r.RegisterCapability(core.Capability{
        Name: "research_topic",        // Keep existing name
        Handler: r.handleResearch,    // Enhanced implementation
    })
    
    // New AI-native capabilities
    r.RegisterCapability(core.Capability{
        Name: "ai_analyze_data",       // New AI capability
        Handler: r.handleAIAnalysis,  // Pure AI implementation
    })
}
```

## 🤝 Best Practices

### 1. **Graceful Degradation**
```go
if r.aiClient == nil {
    return r.basicResponse(ctx, request)
}
return r.aiEnhancedResponse(ctx, request)
```

### 2. **Progressive Enhancement**
```go
response := r.coreLogic(ctx, request)
if r.aiClient != nil {
    response.AIInsights = r.addAIInsights(ctx, response)
}
return response
```

### 3. **User Choice**
```go
if request.UseAI && r.aiClient != nil {
    return r.aiEnhancedPath(ctx, request)
}
return r.traditionalPath(ctx, request)
```

### 4. **Monitoring AI Usage**
```go
if r.aiClient != nil {
    r.metrics.IncrementAIRequests()
    defer r.metrics.RecordAILatency(time.Since(start))
}
```

## 📚 Related Examples

- **[Basic Agent Example](../agent-example/)** - Foundation for enhancement
- **[AI-First Agent](../ai-agent-example/)** - Complete AI-native approach
- **[Orchestration Example](../orchestration-example/)** - Multi-agent coordination
- **[Workflow Example](../workflow-example/)** - YAML-based recipes

## 🆘 Support

- **Migration Guide**: See step-by-step enhancement guide above
- **Framework Documentation**: [GoMind Docs](../../docs/)
- **AI Integration**: Best practices for progressive AI adoption
- **Community**: Discuss enhancement strategies and patterns

---

**Built with ❤️ using the GoMind Framework**  
*Demonstrating progressive AI integration and enhancement patterns*