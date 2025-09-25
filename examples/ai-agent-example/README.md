# AI-First Agent Example

A complete **AI-first architecture** demonstration where **artificial intelligence drives every decision** from request understanding to execution planning to result synthesis. This represents the most advanced pattern in the GoMind framework for building truly intelligent agents.

## 🎯 What This Example Demonstrates

### AI-First Architecture Philosophy

Unlike traditional agents that use AI as a tool, this example demonstrates an **AI-centric approach** where:

- **🧠 AI is the brain**: Every request goes through AI for understanding and planning
- **🎯 AI plans everything**: AI creates execution strategies before any action
- **🔄 AI adapts dynamically**: AI monitors execution and adjusts strategies in real-time
- **💡 AI synthesizes intelligently**: AI creates comprehensive responses with full context

### 4 Core AI-First Capabilities

Each capability demonstrates pure AI-driven processing:

#### 1. 🧠 **Intelligent Query Processor**
```
Query → AI Analysis → AI Planning → AI-Guided Execution → AI Synthesis
```
- **Use Case**: Natural language query understanding and intelligent response generation
- **Pattern**: AI processes any type of query with contextual understanding
- **AI Role**: Query intent analysis, execution planning, adaptive execution, response synthesis

#### 2. 🎯 **Goal-Oriented Task Executor** 
```
Goal → AI Decomposition → AI Resource Mapping → AI Execution Strategy → AI Success Evaluation
```
- **Use Case**: Complex goal achievement through intelligent planning and execution
- **Pattern**: AI breaks down goals into actionable sub-tasks and executes them
- **AI Role**: Goal decomposition, resource allocation, strategy planning, progress monitoring

#### 3. 🔍 **Intelligent Service Orchestrator**
```
Need → AI Service Analysis → AI Strategy Creation → AI-Optimized Execution → AI Quality Assessment
```
- **Use Case**: Intelligent service selection and coordination based on requirements
- **Pattern**: AI analyzes needs and orchestrates optimal service combinations
- **AI Role**: Service analysis, orchestration strategy, execution optimization, quality assessment

#### 4. 🚀 **Adaptive Problem Solver**
```
Problem → AI Analysis → AI Solution Generation → AI Adaptive Execution → AI Learning
```
- **Use Case**: Complex problem solving with learning and adaptation
- **Pattern**: AI tries multiple approaches, learns from failures, and adapts strategies
- **AI Role**: Problem analysis, solution generation, adaptive execution, failure learning

## 🏗️ Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                       AI-First Agent                           │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────┐ │
│  │ Intelligent │  │ Goal-Orient │  │ Service     │  │ Adaptive│ │
│  │ Query       │  │ Task        │  │ Orchestrat  │  │ Problem │ │
│  │ Processor   │  │ Executor    │  │ or          │  │ Solver  │ │
│  │             │  │             │  │             │  │         │ │
│  │ AI Analyzes │  │ AI Plans    │  │ AI Selects  │  │ AI Learns│ │
│  │ → Plans     │  │ → Executes  │  │ → Optimizes │  │ → Adapts│ │
│  │ → Executes  │  │ → Evaluates │  │ → Assesses  │  │ → Solves│ │
│  │ → Synthesizes│  │ → Learns    │  │ → Improves  │  │ → Evolves│ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────┘ │
├─────────────────────────────────────────────────────────────────┤
│                    AI Decision Engine                           │
│   Intent Analysis • Planning • Execution • Adaptation • Learning│
├─────────────────────────────────────────────────────────────────┤
│                Universal AI Provider Layer                      │
│   Auto-Detection • Multi-Provider • Graceful Fallbacks        │
├─────────────────────────────────────────────────────────────────┤
│                   GoMind Core Framework                         │
│   Discovery • Registry • Capabilities • Service Management    │
└─────────────────────────────────────────────────────────────────┘
```

## 🚀 Quick Start

### Prerequisites

- Go 1.25 or later
- **AI Provider API Key (REQUIRED)** - Agent cannot function without AI:
  - OpenAI API key (`OPENAI_API_KEY`) - **Recommended**
  - Groq API key (`GROQ_API_KEY`) - Faster inference
  - Anthropic API key (`ANTHROPIC_API_KEY`) - Claude models
  - Gemini API key (`GEMINI_API_KEY`) - Google AI
- Redis (optional, for service discovery)
- Docker (optional, for containerized deployment)

### 1. Local Development Setup

```bash
# Navigate to the AI-first agent example
cd examples/ai-agent-example

# Install dependencies
go mod tidy

# Set up AI provider (REQUIRED - choose one or more)
export OPENAI_API_KEY="your-openai-key-here"           # Primary option
export GROQ_API_KEY="your-groq-key-here"               # Fast alternative
export ANTHROPIC_API_KEY="your-claude-key-here"        # Claude alternative

# Optional: Configure AI behavior
export AI_PROVIDER="openai"                             # openai, groq, anthropic (auto-detected)
export AI_MODEL="gpt-4"                                 # AI model to use
export AI_TEMPERATURE="0.3"                            # Creativity level (0.0-1.0)
export AI_MAX_TOKENS="1500"                            # Max response length

# Optional: Configure AI-first agent behavior
export PLANNING_ENABLED="true"                         # Enable AI planning
export ADAPTIVE_EXECUTION="true"                       # Enable adaptive execution
export CONTEXT_WINDOW="8000"                          # AI context window size
export GOAL_DECOMPOSITION_DEPTH="5"                   # Max goal breakdown depth

# Optional: Enable service discovery
export REDIS_URL="redis://localhost:6379"
export SERVICE_NAME="ai-first-agent"

# Run the AI-first agent
go run main.go
```

The agent will start with AI-driven capabilities:

```
🧠 AI-First Agent Starting...
🎯 AI Provider: OpenAI GPT-4 (auto-detected)
⚡ All 4 capabilities are AI-powered
🤖 AI drives every decision from understanding to execution

Available endpoints:
  GET  /health - Service health with AI status
  POST /api/capabilities/process_intelligent_query - AI query processing
  POST /api/capabilities/execute_goal_oriented_task - AI goal execution  
  POST /api/capabilities/orchestrate_intelligent_services - AI orchestration
  POST /solve-problem - AI adaptive problem solving
  GET  /api/capabilities - List all AI-first capabilities
```

### 2. Docker Deployment

```bash
# Build the Docker image
docker build -t gomind/ai-first-agent:latest .

# Run with AI provider configuration
docker run -d \
  --name ai-first-agent \
  -p 8092:8080 \
  -p 9090:9090 \
  -e OPENAI_API_KEY="your-key-here" \
  -e AI_MODEL="gpt-4" \
  -e PLANNING_ENABLED="true" \
  -e REDIS_URL="redis://your-redis:6379" \
  gomind/ai-first-agent:latest
```

### 3. Kind Cluster Deployment (Complete Self-Contained)

This AI-First Agent is **completely self-sufficient** for Kind cluster deployment, including dedicated Redis for AI coordination.

#### Prerequisites
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) installed
- [Docker](https://docs.docker.com/get-docker/) running
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed

#### Step-by-Step Kind Deployment

```bash
# 1. Create Kind cluster
kind create cluster --name gomind-ai-first

# 2. Build Docker image
docker build -t ai-first-agent:latest .

# 3. Load image into Kind cluster
kind load docker-image ai-first-agent:latest --name gomind-ai-first

# 4. Create secrets with your AI API keys (REQUIRED)
kubectl create secret generic ai-first-agent-secrets \
  --from-literal=OPENAI_API_KEY="sk-your-openai-key" \
  --from-literal=GROQ_API_KEY="gsk-your-groq-key" \
  --from-literal=ANTHROPIC_API_KEY="sk-ant-your-claude-key" \
  --dry-run=client -o yaml | kubectl apply -f -

# 5. Deploy everything (includes Redis + AI-First Agent)
kubectl apply -f k8-deployment.yaml

# 6. Wait for all pods to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=ai-first-agent -n gomind-ai-first --timeout=300s

# 7. Verify AI coordination is working
kubectl exec -it deployment/redis -n gomind-ai-first -- redis-cli ping

# 8. Check deployment status
kubectl get pods,svc -n gomind-ai-first
```

#### Access the AI-First Agent

```bash
# Port forward to access the AI agent
kubectl port-forward svc/ai-first-agent-service 8092:80 -n gomind-ai-first

# Test AI-first intelligent query processing
curl -X POST http://localhost:8092/api/capabilities/process_intelligent_query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "Find the best approach to analyze customer sentiment",
    "context": "E-commerce customer reviews",
    "preferences": {"accuracy": "high", "speed": "moderate"}
  }'

# Test AI-driven service orchestration
curl -X POST http://localhost:8092/api/capabilities/orchestrate_ai_workflow \
  -H "Content-Type: application/json" \
  -d '{
    "objective": "Process customer feedback for insights",
    "constraints": ["privacy-compliant", "automated"],
    "ai_driven": true
  }'
```

#### Troubleshooting AI-First Deployment

**AI processing issues:**
```bash
# Check AI agent logs for reasoning traces
kubectl logs -f deployment/ai-first-agent -n gomind-ai-first

# Verify AI provider connectivity
kubectl exec -it deployment/ai-first-agent -n gomind-ai-first -- sh
# Inside pod: env | grep API_KEY
```

**Service discovery problems:**
```bash
# Check Redis connectivity for AI coordination
kubectl exec -it deployment/ai-first-agent -n gomind-ai-first -- sh
# Inside pod: redis-cli -h redis-service ping

# Check AI service registry
kubectl exec -it deployment/redis -n gomind-ai-first -- redis-cli KEYS "gomind:ai:*"
```

#### Clean Up

```bash
# Delete the Kind cluster (removes everything)
kind delete cluster --name gomind-ai-first
```

## 📖 Usage Examples & AI-First Patterns

### Pattern 1: Intelligent Query Processing

**Use Case**: Natural language queries that AI understands, plans, and executes intelligently.

```bash
curl -X POST http://localhost:8092/api/capabilities/process_intelligent_query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "Find services that can process financial data and recommend the best one for real-time fraud detection",
    "context": "Building a fintech fraud detection system",
    "preferences": {
      "performance": "real-time",
      "accuracy": "high"
    },
    "constraints": ["must support streaming", "low latency required"]
  }'
```

**AI-First Process:**
1. 🧠 **AI Intent Analysis**: AI understands the real need (fraud detection, not just data processing)
2. 🎯 **AI Planning**: AI creates execution plan (discover services → analyze capabilities → recommend best)
3. 🔄 **AI Execution**: AI discovers services and analyzes them against fraud detection requirements
4. 💡 **AI Synthesis**: AI provides intelligent recommendation with reasoning

**Response Example:**
```json
{
  "query": "Find services that can process financial data...",
  "ai_analysis": {
    "intent": "fraud_detection_service_selection",
    "entities": ["financial_data", "real_time", "fraud_detection"],
    "confidence": 0.94,
    "category": "service_discovery",
    "complexity": "moderate",
    "approach": "discovery_analysis_recommendation"
  },
  "execution_plan": {
    "steps": [
      {"id": "discover", "action": "discover_services", "rationale": "Find all data processing services"},
      {"id": "analyze", "action": "analyze_capabilities", "rationale": "Evaluate fraud detection suitability"},
      {"id": "recommend", "action": "intelligent_recommendation", "rationale": "Recommend best match"}
    ]
  },
  "ai_response": {
    "recommendation": "time-series-processor with fraud-detection-module",
    "reasoning": "Combines real-time processing with specialized fraud algorithms",
    "confidence": 0.91,
    "integration_guidance": "Use streaming API for real-time detection"
  }
}
```

### Pattern 2: Goal-Oriented Task Execution

**Use Case**: Complex goals that AI breaks down and executes systematically.

```bash
curl -X POST http://localhost:8092/api/capabilities/execute_goal_oriented_task \
  -H "Content-Type: application/json" \
  -d '{
    "goal": "Create a comprehensive competitive analysis of project management tools",
    "success_criteria": [
      "Data collected from at least 5 competitors",
      "Feature comparison matrix created",
      "Market positioning analysis completed",
      "Actionable recommendations provided"
    ],
    "available_resources": {
      "data_sources": "web, api, services",
      "analysis_tools": "available",
      "time_budget": "30 minutes"
    },
    "timeline": "complete within 30 minutes",
    "priority": "high"
  }'
```

**AI-First Process:**
1. 🧠 **AI Goal Decomposition**: AI breaks goal into specific sub-goals (data collection, analysis, insights)
2. 🎯 **AI Resource Mapping**: AI matches available services to each sub-goal
3. 🔄 **AI Execution Strategy**: AI plans optimal sequence and parallel execution
4. 📊 **AI Success Evaluation**: AI evaluates results against success criteria

**Response Example:**
```json
{
  "goal": "Create a comprehensive competitive analysis...",
  "goal_decomposition": {
    "sub_goals": [
      "Identify top 5 competitors in PM space",
      "Collect feature data for each competitor",
      "Analyze pricing and positioning",
      "Generate insights and recommendations"
    ],
    "dependencies": ["competitor_identification → feature_collection → analysis → insights"],
    "priority_order": [1, 2, 3, 4]
  },
  "execution_results": {
    "competitors_analyzed": ["Asana", "Notion", "Monday", "Trello", "ClickUp"],
    "feature_matrix": "comprehensive comparison of 50+ features",
    "market_insights": ["AI integration is key differentiator", "Mobile-first trend emerging"]
  },
  "success_evaluation": {
    "achieved": true,
    "confidence": 0.88,
    "criteria_met": "All 4 success criteria satisfied"
  }
}
```

### Pattern 3: Intelligent Service Orchestration

**Use Case**: AI selects and coordinates multiple services to fulfill complex needs.

```bash
curl -X POST http://localhost:8092/api/capabilities/orchestrate_intelligent_services \
  -H "Content-Type: application/json" \
  -d '{
    "need": "Process customer feedback data to extract sentiments and generate improvement recommendations",
    "quality": "comprehensive",
    "budget": "medium",
    "context": "E-commerce customer satisfaction improvement",
    "preferences": {
      "accuracy_over_speed": "true",
      "detailed_insights": "true"
    }
  }'
```

**AI-First Process:**
1. 🔍 **AI Service Discovery**: AI finds all available services and analyzes their capabilities
2. 🧠 **AI Strategy Creation**: AI creates orchestration strategy for the specific need
3. 🎯 **AI Execution Optimization**: AI coordinates services for optimal results
4. 📊 **AI Quality Assessment**: AI evaluates orchestration quality against requirements

### Pattern 4: Adaptive Problem Solving

**Use Case**: Complex problems that require multiple attempts and learning from failures.

```bash
curl -X POST http://localhost:8092/solve-problem \
  -H "Content-Type: application/json" \
  -d '{
    "problem": "Our microservices are experiencing intermittent communication failures",
    "constraints": [
      "Cannot modify existing service code",
      "Must maintain current performance",
      "Solution must be cost-effective"
    ],
    "attempts": 3,
    "learn": true
  }'
```

**AI-First Process:**
1. 🧠 **AI Problem Analysis**: AI analyzes problem nature, constraints, and potential approaches
2. 🚀 **AI Solution Generation**: AI generates multiple solution strategies
3. 🔄 **AI Adaptive Execution**: AI tries solutions, learns from results, adapts approach
4. 🎯 **AI Solution Evaluation**: AI evaluates final solution effectiveness

## 🔧 Configuration Options

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `OPENAI_API_KEY` | OpenAI API key | - | **Yes** (or alternative) |
| `GROQ_API_KEY` | Groq API key (faster inference) | - | Alternative |
| `ANTHROPIC_API_KEY` | Anthropic Claude API key | - | Alternative |
| `GEMINI_API_KEY` | Google Gemini API key | - | Alternative |
| `AI_PROVIDER` | Preferred AI provider | auto-detect | No |
| `AI_MODEL` | AI model to use | gpt-4 | No |
| `AI_TEMPERATURE` | AI creativity level (0.0-1.0) | 0.3 | No |
| `AI_MAX_TOKENS` | Maximum AI response length | 1500 | No |
| `PLANNING_ENABLED` | Enable AI planning | true | No |
| `ADAPTIVE_EXECUTION` | Enable adaptive execution | true | No |
| `CONTEXT_WINDOW` | AI context window size | 8000 | No |
| `GOAL_DECOMPOSITION_DEPTH` | Max goal breakdown depth | 5 | No |
| `MAX_CONCURRENT_AI_REQUESTS` | Max parallel AI requests | 5 | No |
| `AI_REQUEST_TIMEOUT` | AI request timeout | 30s | No |
| `CACHE_AI_RESPONSES` | Enable AI response caching | true | No |
| `PORT` | HTTP server port | 8080 | No |

### AI Provider Auto-Detection

The agent automatically detects available AI providers in this order:
1. **OpenAI** - If `OPENAI_API_KEY` is set (most capable)
2. **Groq** - If `GROQ_API_KEY` is set (fastest inference)
3. **Anthropic** - If `ANTHROPIC_API_KEY` is set (Claude models)
4. **Gemini** - If `GEMINI_API_KEY` is set (Google AI)

### Hard Dependency on AI

**Important**: This agent **requires** an AI provider to function. Without AI:
```go
// Agent creation fails if no AI provider is available
if err != nil {
    log.Fatalf("AI-First Agent requires an AI provider. Please set OPENAI_API_KEY, GROQ_API_KEY, or another supported provider")
}
```

## 📊 AI-First Capability Matrix

| Capability | AI Analysis | AI Planning | AI Execution | AI Adaptation | Use Case |
|------------|-------------|-------------|--------------|---------------|----------|
| **process_intelligent_query** | ✅ Intent & entities | ✅ Execution plan | ✅ Guided execution | ✅ Real-time adaptation | Any natural language query |
| **execute_goal_oriented_task** | ✅ Goal decomposition | ✅ Resource mapping | ✅ Strategy execution | ✅ Success evaluation | Complex goal achievement |
| **orchestrate_intelligent_services** | ✅ Service analysis | ✅ Orchestration strategy | ✅ Optimized coordination | ✅ Quality assessment | Multi-service coordination |
| **solve_adaptive_problem** | ✅ Problem analysis | ✅ Solution generation | ✅ Multiple attempts | ✅ Failure learning | Complex problem solving |

## 🧪 Testing the AI-First Patterns

### Test Intelligent Query Processing
```bash
# Test natural language understanding
curl -X POST http://localhost:8092/api/capabilities/process_intelligent_query \
  -d '{"query": "Help me understand which AI models work best for code generation", "context": "developer productivity"}'
```

### Test Goal-Oriented Execution
```bash
# Test complex goal decomposition
curl -X POST http://localhost:8092/api/capabilities/execute_goal_oriented_task \
  -d '{"goal": "Analyze our API performance and suggest optimizations", "success_criteria": ["bottlenecks identified", "solutions proposed"]}'
```

### Test Intelligent Orchestration
```bash
# Test AI service coordination
curl -X POST http://localhost:8092/api/capabilities/orchestrate_intelligent_services \
  -d '{"need": "Real-time data processing with anomaly detection", "quality": "high", "budget": "medium"}'
```

### Test Adaptive Problem Solving
```bash
# Test AI learning and adaptation
curl -X POST http://localhost:8092/solve-problem \
  -d '{"problem": "Database queries are timing out during peak hours", "constraints": ["no downtime allowed"], "attempts": 3, "learn": true}'
```

## 🔍 Monitoring and AI Observability

### Health Check with AI Status
```bash
curl http://localhost:8092/health
```

**Response shows AI capabilities:**
```json
{
  "status": "healthy",
  "service": "ai-first-agent",
  "ai_provider": "OpenAI GPT-4",
  "ai_required": true,
  "capabilities": 4,
  "ai_first_capabilities": 4,
  "ai_features": {
    "intent_analysis": true,
    "planning": true,
    "adaptive_execution": true,
    "learning": true
  }
}
```

### AI Capability Details
```bash
curl http://localhost:8092/api/capabilities
```

**Response shows AI-first nature:**
```json
{
  "capabilities": [
    {
      "name": "process_intelligent_query",
      "ai_driven": true,
      "ai_phases": ["analysis", "planning", "execution", "synthesis"]
    },
    {
      "name": "execute_goal_oriented_task",
      "ai_driven": true,
      "ai_phases": ["decomposition", "mapping", "strategy", "evaluation"]
    }
  ]
}
```

## 🆚 Comparison with Traditional Agents

| Aspect | Traditional Agent | AI-First Agent |
|--------|------------------|----------------|
| **Decision Making** | Hardcoded logic | AI-driven analysis |
| **Request Handling** | Pattern matching | Intent understanding |
| **Execution** | Fixed workflows | AI-planned strategies |
| **Adaptation** | Manual updates | Real-time AI adaptation |
| **Problem Solving** | Predefined solutions | AI-generated solutions |
| **Learning** | None | Continuous AI learning |
| **Complexity Handling** | Limited | Unlimited AI reasoning |
| **Natural Language** | Basic parsing | Full NL understanding |

## 🚀 Migration from Traditional Agents

### Step 1: Add AI Requirement
```go
// Traditional agent
func NewAgent() *Agent {
    return &Agent{...}
}

// AI-first agent
func NewAIFirstAgent() (*AIFirstAgent, error) {
    // AI is REQUIRED - agent won't work without it
    aiClient, err := ai.NewClient()
    if err != nil {
        return nil, fmt.Errorf("AI-First Agent requires AI provider: %w", err)
    }
    return &AIFirstAgent{aiClient: aiClient}, nil
}
```

### Step 2: Replace Logic with AI Analysis
```go
// Traditional: hardcoded logic
func (a *Agent) handleRequest(req Request) Response {
    if strings.Contains(req.Text, "weather") {
        return a.getWeather()
    }
    // ... more hardcoded patterns
}

// AI-first: AI understanding
func (a *AIFirstAgent) handleRequest(req Request) Response {
    analysis, _ := a.aiClient.AnalyzeIntent(req.Text)
    plan, _ := a.aiClient.CreateExecutionPlan(analysis)
    return a.executeAIPlan(plan)
}
```

### Step 3: Enable AI Learning and Adaptation
```go
// AI continuously improves based on results
func (a *AIFirstAgent) executeAndLearn(plan ExecutionPlan) Response {
    result := a.execute(plan)
    if !result.Success {
        // AI learns from failure and adapts
        adaptation, _ := a.aiClient.AdaptStrategy(plan, result.Error)
        return a.execute(adaptation)
    }
    return result
}
```

## 🤝 Best Practices for AI-First Development

### 1. **AI Dependency Management**
```go
// Always check AI availability
if a.aiClient == nil {
    return errors.New("AI-first agent cannot function without AI")
}
```

### 2. **AI Response Validation**
```go
// Validate AI responses before acting
response, err := a.aiClient.GenerateResponse(ctx, prompt)
if err != nil || !a.validateAIResponse(response) {
    return a.fallbackStrategy()
}
```

### 3. **Context Management**
```go
// Maintain context across AI interactions
context := &AIContext{
    PreviousResults: results,
    CurrentGoal:     goal,
    Constraints:     constraints,
}
response := a.aiClient.GenerateWithContext(ctx, prompt, context)
```

### 4. **AI Performance Monitoring**
```go
// Monitor AI performance and costs
metrics := &AIMetrics{
    RequestLatency: time.Since(start),
    TokensUsed:     response.TokenCount,
    Confidence:     response.Confidence,
}
a.recordAIMetrics(metrics)
```

## 📚 Related Examples

- **[Enhanced Agent Example](../agent-example-enhanced/)** - Progressive AI integration approach
- **[Basic Agent Example](../agent-example/)** - Traditional agent foundation
- **[Orchestration Example](../orchestration-example/)** - Multi-agent coordination
- **[Workflow Example](../workflow-example/)** - Structured task execution

## 🆘 Support

- **AI-First Development**: Best practices for AI-centric architecture
- **Framework Documentation**: [GoMind Docs](../../docs/)
- **AI Integration**: Complete AI provider integration guide
- **Community**: Discuss AI-first patterns and implementations

---

**Built with ❤️ using the GoMind Framework**  
*Demonstrating AI-first architecture where artificial intelligence drives every decision and process*