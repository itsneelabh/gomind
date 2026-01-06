# GoMind Orchestration Module

Multi-agent coordination with AI-driven orchestration and declarative workflows.

## 📚 Table of Contents

- [🎯 What Does This Module Do?](#-what-does-this-module-do)
- [🚀 Quick Start](#-quick-start)
- [🧠 How It Works](#-how-it-works)
- [🤖 AI Orchestration in Detail](#-ai-orchestration-in-detail)
  - [LLM-Generated Execution Plan Structure](#llm-execution-plan)
- [🔧 Workflow Engine in Detail](#-workflow-engine-in-detail)
- [🎭 When to Use Each Mode](#-when-to-use-each-mode)
- [⏱️ Async Tasks for Long-Running Operations](#️-async-tasks-for-long-running-operations)
- [🏗️ Architecture & Design Decisions](#️-architecture--design-decisions)
- [🏗️ How Everything Fits Together](#️-how-everything-fits-together)
- [📊 Performance & Caching](#-performance--caching-explained)
- [🔍 Monitoring & Metrics](#-monitoring--metrics---know-whats-happening)
- [🛠️ Configuration](#️-configuration)
- [📝 Usage Patterns](#-usage-patterns)
- [🚦 Requirements](#-requirements)
- [🚀 Scaling to Hundreds of Agents](#-scaling-to-hundreds-of-agents---capability-provider-architecture)
- [⚡ Performance Considerations](#-performance-considerations)
- [🔮 Potential Enhancements](#-potential-enhancements)
- [📖 API Reference](#-api-reference)
- [💡 Best Practices & Tips](#-best-practices--tips)
- [🆕 Production-Ready Enhancements](#-production-ready-enhancements)
- [🎓 Summary](#-summary---what-youve-learned)

## 🎯 What Does This Module Do?

Think of this module as the **conductor of an orchestra**. Just like a conductor coordinates musicians to create beautiful music, this module coordinates multiple agents to accomplish complex tasks.

It provides two powerful ways to orchestrate agents and tools:

1. **AI Orchestration** - Tell it what you want in natural language, and AI figures out which tools and agents to call
2. **Workflow Engine** - Define step-by-step "recipes" that execute reliably every time

### Real-World Analogy: The Coffee Shop

Imagine running a coffee shop with different workers:
- **Barista** - Makes coffee (like a data processing tool)
- **Cashier** - Takes orders (like an API gateway tool)
- **Baker** - Makes pastries (like a report generator tool)

The orchestration module ensures:
1. The cashier takes the order
2. The barista and baker work **in parallel** (no waiting!)
3. Everything comes together for the customer

That's exactly how it coordinates your tools and agents!

## 🚀 Quick Start

### Installation

```go
import "github.com/itsneelabh/gomind/orchestration"
```

### Two Ways to Orchestrate

#### Option 1: AI-Driven (Natural Language)
```go
// Just describe what you want in plain English
response, _ := orchestrator.ProcessRequest(ctx,
    "Analyze Tesla stock and summarize recent news",
    nil,
)

// Behind the scenes:
// 1. AI reads your request
// 2. Looks at available tools and agents (stock-tool, news-tool, analyzer-agent)
// 3. Creates an execution plan
// 4. Calls components in the right order
// 5. Combines results into a coherent response
```

#### Option 2: Workflow (Predictable Recipes)
```yaml
# Define exact steps like a recipe
name: analyze-stock
steps:
  - name: get-price          # Step 1: Get the data
    tool: stock-tool         # Using a tool (passive component)
    action: fetch_price
    
  - name: get-news           # Step 2: Get news (parallel with step 1!)
    tool: news-tool          # Another tool
    action: fetch_latest
    
  - name: analyze            # Step 3: Analyze everything
    agent: ai-analyzer       # Using an agent (active orchestrator)
    action: analyze
    inputs:
      price: ${steps.get-price.output}  # Use output from step 1
      news: ${steps.get-news.output}    # Use output from step 2
    depends_on: [get-price, get-news]   # Wait for both
```

## 🧠 How It Works

### The Two Orchestration Modes Explained

#### 1. AI Orchestration - The Smart Assistant
**How it works:** Like having a smart assistant who understands your request and figures out what to do.

```
Your Request: "Analyze Apple stock"
     ↓
1. AI understands: "User wants stock analysis"
     ↓
2. AI checks available components: "I have stock-price tool, news tool, and analyzer agent"
     ↓
3. AI creates plan: "First get price and news from tools, then analyze with agent"
     ↓
4. Executes plan: Calls tools and agents in parallel where possible
     ↓
5. AI synthesizes: Combines all responses into one answer
     ↓
Your Response: "Apple stock is up 3% today. Based on news about..."
```

#### 2. Workflow Engine - The Recipe Book
**How it works:** Like following a recipe - same steps every time, predictable results.

```
Workflow: Daily Report
     ↓
1. Read recipe: "Get sales, inventory, and customers"
     ↓
2. Execute in parallel: All three can run at once!
     ↓
3. Wait for dependencies: Report generator waits for all data
     ↓
4. Variable substitution: ${steps.sales.output} becomes actual data
     ↓
5. Return outputs: Structured result every time
```

### 🔧 Core Components Explained

| Component | What It Does | Real-World Analogy |
|-----------|--------------|-------------------|
| **Component Catalog** | Keeps track of all available tools and agents and what they can do | Like a phone book of workers and their skills |
| **Smart Executor** | Runs multiple tool/agent calls in parallel when possible | Like a project manager coordinating team members |
| **AI Synthesizer** | Combines responses from multiple tools and agents into one answer | Like an editor combining reporter stories into one article |
| **Workflow Engine** | Executes predefined step-by-step processes | Like a factory assembly line |
| **DAG Scheduler** | Figures out what can run in parallel | Like a smart scheduler who knows task dependencies |
| **Routing Cache** | Remembers recent decisions to speed things up | Like remembering phone numbers instead of looking them up |

## 🤖 AI Orchestration in Detail

### Step-by-Step: How AI Processes Your Request

#### Example: "Get me a comprehensive analysis of Tesla"

**Step 1: Understanding (Natural Language → Intent)**
```javascript
You say: "Get me a comprehensive analysis of Tesla"
          ↓
AI understands: {
  "intent": "analyze_company",
  "target": "Tesla",
  "scope": "comprehensive"
}
```

**Step 2: Discovery (Finding the Right Workers)**
```javascript
AI checks catalog:
✓ financial-tool     -> can get financials
✓ news-tool         -> can get news
✓ sentiment-agent    -> can analyze sentiment
✓ technical-agent    -> can do technical analysis

AI decides: "I'll use both tools and agents!"
```

**Step 3: Smart Planning (Creating Execution Order)**
```javascript
AI creates plan:
1. [Parallel Group 1]
   - financial-tool: get_financials("TSLA")
   - news-tool: get_recent_news("Tesla")
   
2. [Parallel Group 2] 
   - sentiment-agent: analyze(news_data)
   - technical-agent: analyze(financial_data)
   
3. [Final Step]
   - Synthesize all results into coherent analysis
```

**Step 4: Synthesis (Making Sense of Everything)**
```javascript
Tool and agent responses:
- Financial Tool: "Revenue $96B, up 35% YoY..."
- News Tool: "Tesla announces new factory..."
- Sentiment Agent: "72% positive sentiment..."
- Technical Agent: "RSI 65, bullish trend..."
          ↓
AI combines into:
"Tesla shows strong growth with $96B revenue (+35% YoY).
Recent factory announcement drives positive sentiment (72%).
Technical indicators suggest continued bullish trend.
Recommendation: Strong Buy"
```

### LLM-Generated Execution Plan Structure {#llm-execution-plan}

When the AI orchestrator processes a natural language request, the LLM generates a **DAG-based execution plan** in JSON format. This plan defines which tools/agents to call, their parameters, and dependencies between steps.

#### JSON Plan Structure

```json
{
  "plan_id": "travel-plan-1766115892559988547",
  "steps": [
    {
      "step_id": "step-1",
      "agent_name": "stock-service",
      "capability": "stock_quote",
      "description": "Get TSLA stock quote to calculate funds",
      "parameters": {"symbol": "TSLA"},
      "depends_on": []
    },
    {
      "step_id": "step-2",
      "agent_name": "country-info-tool",
      "capability": "get_country_info",
      "description": "Get Switzerland currency info",
      "parameters": {"country": "Switzerland"},
      "depends_on": []
    },
    {
      "step_id": "step-3",
      "agent_name": "currency-tool",
      "capability": "convert_currency",
      "description": "Convert USD to CHF using step-1 & step-2 data",
      "parameters": {
        "from": "USD",
        "to": "{{step-2.response.data.currency.code}}",
        "amount": "{{step-1.response.data.price}}"
      },
      "depends_on": ["step-1", "step-2"]
    },
    {
      "step_id": "step-4",
      "agent_name": "geocoding-tool",
      "capability": "geocode_location",
      "description": "Get Zurich coordinates for weather lookup",
      "parameters": {"location": "Zurich"},
      "depends_on": []
    },
    {
      "step_id": "step-5",
      "agent_name": "weather-tool-v2",
      "capability": "get_current_weather",
      "description": "Get Zurich weather using coordinates from step-4",
      "parameters": {
        "lat": "{{step-4.response.data.lat}}",
        "lon": "{{step-4.response.data.lon}}"
      },
      "depends_on": ["step-4"]
    },
    {
      "step_id": "step-6",
      "agent_name": "news-tool",
      "capability": "search_news",
      "description": "Search news about Zurich",
      "parameters": {"query": "Zurich", "max_results": 5},
      "depends_on": []
    }
  ]
}
```

#### Key Fields

| Field | Description |
|-------|-------------|
| `plan_id` | Unique identifier for the execution plan |
| `step_id` | Unique identifier for each step (used in `depends_on` references) |
| `agent_name` | The tool/agent to call (discovered via Redis) |
| `capability` | The specific capability/action to invoke |
| `parameters` | Input parameters, may include template references |
| `depends_on` | Array of step IDs that must complete before this step runs |

#### Template References

Parameters can reference outputs from previous steps using the template syntax:
- `{{step-N.response.data.field}}` - Access nested field from step N's response
- At execution time, templates are resolved with actual values from completed steps

#### DAG Visualization: Parallel Execution Groups

The executor analyzes `depends_on` to determine which steps can run in parallel:

```
┌─────────────────────────────────────────────────────────────────────┐
│  PARALLEL GROUP 1 (4 independent steps - no dependencies)          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────┐  ┌─────────────────┐  ┌──────────────┐  ┌───────┐ │
│  │   step-1    │  │     step-2      │  │    step-4    │  │step-6 │ │
│  │ stock_quote │  │ get_country_info│  │geocode_location│ │ news │ │
│  │  (TSLA)     │  │  (Switzerland)  │  │   (Zurich)   │  │       │ │
│  └──────┬──────┘  └────────┬────────┘  └──────┬───────┘  └───────┘ │
│         │                  │                  │                     │
└─────────┼──────────────────┼──────────────────┼─────────────────────┘
          │                  │                  │
          ▼                  ▼                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│  PARALLEL GROUP 2 (2 dependent steps - wait for dependencies)      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌───────────────────────────────┐    ┌──────────────────────────┐ │
│  │           step-3              │    │         step-5           │ │
│  │     convert_currency          │    │   get_current_weather    │ │
│  │  depends_on: [step-1, step-2] │    │   depends_on: [step-4]   │ │
│  │                               │    │                          │ │
│  │  amount: {{step-1...price}}   │    │  lat: {{step-4...lat}}   │ │
│  │  to: {{step-2...currency}}    │    │  lon: {{step-4...lon}}   │ │
│  └───────────────────────────────┘    └──────────────────────────┘ │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │    RESPONSE     │
                    │  (all 6 steps   │
                    │   aggregated)   │
                    └─────────────────┘
```

**Execution Flow:**
1. **Group 1**: Steps 1, 2, 4, 6 execute concurrently (no dependencies)
2. **Group 2**: Steps 3, 5 execute concurrently after their dependencies complete
3. **Response**: All results are aggregated and optionally synthesized by AI

#### Intelligent Error Recovery

When a step fails (e.g., wrong parameters, validation errors), the executor uses a **4-layer error recovery system**:

| Layer | Component | Trigger | Capability |
|-------|-----------|---------|------------|
| **Layer 1** | Auto-Wirer | Before execution | Exact name matching, type coercion |
| **Layer 2** | Micro-Resolver | Before execution | LLM extracts values from source data |
| **Layer 3** | Error Analyzer | After 4xx error | LLM suggests parameter corrections |
| **Layer 4** | Semantic Retry | After Layer 3 fails | LLM computes derived values with full context |

**Example: Layer 4 Semantic Retry in Action**

```
User: "Sell 100 Tesla shares and convert to EUR"

Step 1: stock_quote("TSLA") → {price: 468.285}

Step 3: convert_currency({amount: 0}) → HTTP 400 "amount must be > 0"
        ↓
Layer 3: ErrorAnalyzer says "cannot fix - don't know the amount"
        ↓
Layer 4: ContextualReResolver sees:
         - User query: "sell 100 shares"
         - Source data: {price: 468.285}
         - Computes: 100 × 468.285 = 46,828.50
        ↓
Retry:  convert_currency({amount: 46828.5}) → HTTP 200 ✓
```

> 📖 **Full Details**: See [LLM-First Hybrid Parameter Resolution](#-production-ready-enhancements) and [SEMANTIC_RETRY_DESIGN.md](notes/SEMANTIC_RETRY_DESIGN.md) for the complete 4-layer architecture.

#### Observing Plans in Jaeger

The execution plan and its execution are fully traced in Jaeger:

1. **Find the trace**: Use the `request_id` from your API response to search in Jaeger
2. **Plan generation span**: Look for `orchestrator.plan` span with attributes:
   - `plan_steps`: Number of steps in the plan
   - `ai.tokens_used`: Tokens consumed by LLM for planning
   - `ai.model`: Model used (e.g., `gpt-4o-mini`)
3. **Step execution spans**: Each `executor.step.*` span shows:
   - `step_id`: Which step executed
   - `agent_name`: Tool/agent called
   - `capability`: Action invoked
   - `duration`: Execution time
   - `status`: Success/failure
4. **Parallel execution**: Steps in the same group will have overlapping timestamps

**Example Jaeger URL:**
```
http://localhost:16686/trace/<trace_id>
```

**Key Metrics from Trace:**
| Metric | Where to Find |
|--------|---------------|
| Plan generation time | `orchestrator.plan` span duration |
| Total execution time | Root span duration |
| Per-step latency | Individual `executor.step.*` spans |
| LLM token usage | `ai.tokens_used` attribute |
| Parallelization efficiency | Compare overlapping step timestamps |

### Setting Up AI Orchestration

```go
// Step 1: Set up discovery (the registry for tools and agents)
// Set environment: export REDIS_URL="redis://localhost:6379"
discovery := core.NewRedisDiscovery(os.Getenv("REDIS_URL"))  // e.g., "redis://localhost:6379"

// Step 2: Set up AI (the brain)
aiClient, _ := ai.NewClient(
    ai.WithProvider("openai"),
    ai.WithAPIKey(apiKey),
)

// Step 3: Create orchestrator with dependencies
deps := orchestration.OrchestratorDependencies{
    Discovery: discovery,  // Required: Agent/tool discovery
    AIClient:  aiClient,   // Required: LLM for routing decisions
    // Optional dependencies (can be nil):
    // CircuitBreaker: cb,  // For sophisticated resilience
    // Logger: logger,      // For structured logging
    // Telemetry: telemetry,// For observability
}

config := orchestration.DefaultConfig()
config.CacheEnabled = true  // Remember recent decisions
config.ExecutionOptions.MaxConcurrency = 10  // Run up to 10 tools/agents at once

orchestrator, err := orchestration.CreateOrchestrator(config, deps)
if err != nil {
    log.Fatal(err)
}

// Step 4: Start the orchestrator
orchestrator.Start(ctx)

// Step 5: Just ask questions!
response, _ := orchestrator.ProcessRequest(ctx,
    "What's the weather and traffic like in NYC?",
    nil,
)
fmt.Println(response.Response)
// Output: "Current NYC weather is 72°F and sunny. 
//          Traffic is moderate with 25 min delays on I-95..."
```

#### Quick Start with Simple Orchestrator

```go
// For rapid prototyping - zero configuration required!
orchestrator := orchestration.CreateSimpleOrchestrator(discovery, aiClient)

// That's it! Start using immediately
response, _ := orchestrator.ProcessRequest(ctx,
    "Analyze Apple stock performance",
    nil,
)
```

## 🔧 Workflow Engine in Detail

### How Workflows Work - The Smart Recipe Executor

#### Understanding DAG Execution (It's Simpler Than It Sounds!)

**DAG = Directed Acyclic Graph** - Fancy words for "tasks with dependencies"

Think of it like cooking dinner:
```yaml
steps:
  - name: boil-water        # Can start immediately
  - name: chop-vegetables   # Can also start immediately (parallel!)
  - name: cook-pasta
    depends_on: [boil-water]  # Must wait for water
  - name: make-sauce
    depends_on: [chop-vegetables]  # Must wait for veggies
  - name: combine-dish
    depends_on: [cook-pasta, make-sauce]  # Waits for both
```

The workflow engine automatically figures out:
```
[boil-water]  [chop-vegetables]  <- These run in parallel!
      ↓              ↓
[cook-pasta]   [make-sauce]       <- These also run in parallel!
      \            /
       [combine-dish]             <- This waits for both
```

### Three Powerful Discovery Methods

#### 1. Direct Component Discovery
```yaml
steps:
  - name: get-price
    tool: stock-price-tool  # "I want THIS specific tool"
    action: fetch_price
```

#### 2. Capability-Based Discovery  
```yaml
steps:
  - name: analyze-text
    capability: sentiment_analysis  # "I need ANY component that can do this"
    action: analyze
    # Engine finds available tools/agents: sentiment-tool-v1, sentiment-agent-v2, etc.
    # Picks the best one (healthy, lowest load)
```

#### 3. Dynamic Component Discovery
```yaml
# No hardcoded URLs needed!
# Workflow says: "I need financial-advisor-agent" or "I need stock-price-tool"
# Discovery returns: "It's at http://10.0.0.5:8080" 
# But if it moves to http://10.0.0.9:9090, workflow still works!
```

### Complete Workflow Example: Investment Analysis

```yaml
name: investment-analysis
version: "1.0"
description: Analyze a stock for investment decisions

inputs:
  symbol:
    type: string
    required: true
    description: Stock symbol (e.g., AAPL, TSLA)

steps:
  # Phase 1: Gather Data (all run in parallel!)
  - name: get-price
    tool: market-data-tool     # Tool for data fetching
    action: fetch_price
    inputs:
      symbol: ${inputs.symbol}
    timeout: 5s
    
  - name: get-news
    capability: news_aggregation  # Find ANY news tool or agent
    action: fetch_recent
    inputs:
      query: ${inputs.symbol}
      limit: 10
    
  - name: get-sentiment
    tool: social-sentiment-tool  # Tool for sentiment data
    action: analyze
    inputs:
      symbol: ${inputs.symbol}
    retry:  # Handle flaky services
      max_attempts: 3
      backoff: exponential
      initial_wait: 1s
    
  # Phase 2: Analysis (waits for data, then parallel)
  - name: technical-analysis
    agent: technical-analyzer    # Agent for complex analysis
    action: analyze_technicals
    inputs:
      price_data: ${steps.get-price.output}
    depends_on: [get-price]
    
  - name: news-analysis
    agent: ai-news-analyzer      # Agent for intelligent analysis
    action: analyze_impact
    inputs:
      articles: ${steps.get-news.output}
      sentiment: ${steps.get-sentiment.output}
    depends_on: [get-news, get-sentiment]
    
  # Phase 3: Generate Report (waits for all analysis)
  - name: final-report
    agent: ai-advisor           # Agent for orchestration and synthesis
    action: generate_recommendation
    inputs:
      price: ${steps.get-price.output}
      technical: ${steps.technical-analysis.output}
      news_impact: ${steps.news-analysis.output}
    depends_on: [technical-analysis, news-analysis]

outputs:
  recommendation: ${steps.final-report.output.action}  # BUY/SELL/HOLD
  confidence: ${steps.final-report.output.confidence}   # 0-100%
  report: ${steps.final-report.output.summary}

on_error:
  strategy: continue  # Keep going even if one service fails
```

### Using Workflows in Code

```go
// Step 1: Create the workflow engine
stateStore := orchestration.NewRedisStateStore(discovery)
engine := orchestration.NewWorkflowEngine(discovery, stateStore, logger)

// Step 2: Load your workflow (from file or string)
yamlData, _ := os.ReadFile("investment-analysis.yaml")
workflow, _ := engine.ParseWorkflowYAML(yamlData)

// Step 3: Execute with inputs
inputs := map[string]interface{}{
    "symbol": "AAPL",
}

execution, err := engine.ExecuteWorkflow(ctx, workflow, inputs)
if err != nil {
    log.Printf("Workflow failed: %v", err)
    return
}

// Step 4: Use the results!
fmt.Printf("Recommendation: %s\n", execution.Outputs["recommendation"])
fmt.Printf("Confidence: %.0f%%\n", execution.Outputs["confidence"])
fmt.Printf("Analysis: %s\n", execution.Outputs["report"])

// Step 5: Check what happened (optional)
for stepName, step := range execution.Steps {
    fmt.Printf("Step %s: %s (took %v)\n", 
        stepName, step.Status, step.EndTime.Sub(*step.StartTime))
}
```

### How Variables Work - Data Flow Between Steps

```yaml
# Variables let steps share data, like passing ingredients in cooking!

steps:
  - name: step-one
    tool: data-fetcher          # Tool fetches data
    action: get_data
    # This step produces output
    
  - name: step-two
    agent: processor            # Agent processes data
    action: process
    inputs:
      data: ${steps.step-one.output}  # Uses output from step-one!
      # At runtime, this becomes the actual data
    depends_on: [step-one]

# You can also use:
# ${inputs.fieldName}           - Input parameters
# ${steps.stepName.output}      - Full output object
# ${steps.stepName.output.field} - Specific field from output
```

## 🎯 When to Use the Orchestration Module

Use this module when you need capabilities beyond basic tool/agent coordination:

| Capability | Without Orchestration | With Orchestration |
|------------|----------------------|-------------------|
| Tool discovery | Manual HTTP calls | Automatic via registry |
| Error handling | Fail or basic retry | **AI analyzes error, corrects parameters** |
| Multi-tool workflows | Custom coordination code | Declarative YAML or AI-planned |
| Parameter generation | Manual payload construction | **LLM-generated from natural language** |
| Retry logic | Simple exponential backoff | **Semantic retry with parameter correction** |

### Key Features You Get

1. **Intelligent Error Recovery (Layer 4 Semantic Retry)**
   - LLM analyzes why a tool call failed
   - Automatically corrects parameters (e.g., "Flower Mound, TX" → "Flower Mound, US")
   - Retries with corrected payload

2. **AI-Powered Orchestration**
   - Natural language request processing
   - Dynamic tool/agent selection
   - Intelligent parameter binding

3. **Workflow Engine**
   - Declarative YAML workflows
   - Parallel and sequential execution
   - Template variable interpolation

> **Note**: If your agent only needs basic tool calls without intelligent retry, you can use `core` directly. For smarter error recovery, use this module.

## 🎭 When to Use Each Mode

### Use AI Orchestration When:
- Processing natural language requests
- Tool/agent selection needs to be dynamic
- Tasks require intelligent routing decisions
- Exploring new tool and agent combinations

### Use Workflows When:
- Processes are well-defined and repeatable
- You need guaranteed execution order
- Predictable performance is important
- Avoiding LLM costs for routine tasks

## ⏱️ Async Tasks for Long-Running Operations

### The Problem: When AI Takes Too Long

Imagine you walk into a restaurant and order a slow-cooked brisket. The waiter doesn't make you stand at the counter for 6 hours waiting—they give you a ticket number and tell you they'll let you know when it's ready.

The same principle applies to AI orchestration. When your workflow involves:
- Multiple tool calls executed in sequence or parallel
- AI reasoning that takes 30+ seconds
- External API calls with unpredictable latency
- Complex research tasks spanning several services

...the HTTP connection might timeout before you get results. Your client is left wondering: "Did it fail? Is it still running?"

### The Solution: HTTP 202 + Polling Pattern

GoMind provides an async task system that works like that restaurant ticket:

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Async Task Flow                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│   1. Client submits task    →  HTTP 202 + Task ID (instant)        │
│   2. Worker processes task  →  Background (takes as long as needed) │
│   3. Client polls status    →  GET /tasks/{id} (progress updates)   │
│   4. Task completes         →  Results available in response        │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

The framework provides two components:
- **TaskAPIHandler**: Accepts task submissions, returns HTTP 202 immediately
- **TaskWorkerPool**: Processes tasks in background with configurable concurrency

### When to Use Async Tasks

| Scenario | Sync (Regular HTTP) | Async (Task System) |
|----------|---------------------|---------------------|
| Single tool call (< 5s) | ✅ | Overkill |
| Multi-tool orchestration (< 30s) | ✅ | Optional |
| Complex AI research (30s - 5min) | ⚠️ Risky | ✅ Recommended |
| Batch processing or pipelines | ❌ Timeout likely | ✅ Required |

### Quick Example

```go
// Register an async-capable handler
workerPool.RegisterHandler("research", func(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
    // Report progress as you go
    reporter.Report(&core.TaskProgress{
        CurrentStep: 1,
        TotalSteps:  3,
        Message:     "Calling weather service...",
    })

    // Use orchestrator for AI-driven tool selection
    response, err := orchestrator.ProcessRequest(ctx, task.Input["query"].(string), nil)
    if err != nil {
        return err
    }

    task.Result = response
    return nil
})
```

📖 **For complete implementation details, deployment patterns, and production configuration, see the [Async Orchestration Guide](../docs/ASYNC_ORCHESTRATION_GUIDE.md).**

## 🏗️ Architecture & Design Decisions

### Why Orchestration Doesn't Import the AI Module

**Critical Design Decision**: The orchestration module uses `core.AIClient` interface instead of importing the `ai` module directly. This is intentional and follows the framework's "Zero Framework Dependencies" principle.

#### The Dependency Injection Pattern

```go
// ❌ NEVER DO THIS - Violates architectural principles
// orchestration/orchestrator.go
import "github.com/itsneelabh/gomind/ai"  // FORBIDDEN: Module importing module

// ✅ THIS IS CORRECT - Interface-based dependency injection
import "github.com/itsneelabh/gomind/core"  // Only import core

type AIOrchestrator struct {
    aiClient core.AIClient  // Uses interface from core, NOT ai module
}

func NewAIOrchestrator(config *OrchestratorConfig, discovery core.Discovery, aiClient core.AIClient) *AIOrchestrator {
    // AIClient is INJECTED as parameter, not created internally
    return &AIOrchestrator{
        aiClient: aiClient,  // Dependency injection
    }
}
```

#### How Applications Wire Everything Together

The application layer is responsible for creating both the AI client and orchestrator:

```go
// main.go - Application wires components together
import (
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/ai"            // App imports ai
    "github.com/itsneelabh/gomind/orchestration" // App imports orchestration
)

func main() {
    // Step 1: Create AI client (from ai module)
    aiClient, _ := ai.NewClient(
        ai.WithProvider("openai"),
        ai.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
    )

    // Step 2: Pass AI client to orchestrator (dependency injection)
    orchestrator := orchestration.CreateSimpleOrchestrator(discovery, aiClient)

    // The orchestrator now has AI capabilities WITHOUT importing ai module!
}
```

#### Benefits of This Design

1. **True Modularity**: Orchestration can work with ANY implementation of `core.AIClient`:
   ```go
   // Use ai module's implementation
   aiClient := ai.NewClient(...)

   // OR use a custom implementation
   aiClient := mycompany.NewCustomAIClient(...)

   // OR use a mock for testing
   aiClient := &MockAIClient{}

   // Orchestration doesn't care - just uses the interface
   orchestrator := orchestration.NewAIOrchestrator(config, discovery, aiClient)
   ```

2. **No Circular Dependencies**: Modules only import core, never each other:
   ```
   orchestration → core ← ai
   telemetry    → core ← resilience
   ui           → core

   (No direct connections between optional modules)
   ```

3. **Testing Isolation**: Test orchestration without the ai module:
   ```go
   // orchestration/factory_test.go
   func TestOrchestrator(t *testing.T) {
       aiClient := NewMockAIClient()  // Mock implementation
       orchestrator := CreateSimpleOrchestrator(discovery, aiClient)
       // Test orchestration logic without real AI calls
   }
   ```

4. **Provider Flexibility**: Switch AI providers without touching orchestration code:
   ```go
   // Today: Using OpenAI
   aiClient := ai.NewClient(ai.WithProvider("openai"))

   // Tomorrow: Switch to Anthropic
   aiClient := ai.NewClient(ai.WithProvider("anthropic"))

   // Next week: Use your private LLM
   aiClient := mycompany.PrivateLLMClient()

   // Orchestration code remains unchanged!
   ```

#### The Dependency Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│     core     │     │     core     │     │     core     │
│              │     │              │     │              │
│ Defines:     │     │ Defines:     │     │ Defines:     │
│ - AIClient   │     │ - Discovery  │     │ - Telemetry  │
│   interface  │     │   interface  │     │   interface  │
└──────▲───────┘     └──────▲───────┘     └──────▲───────┘
       │                    │                    │
       ├────────────────────┼────────────────────┤
       │                    │                    │
┌──────┴───────┐     ┌──────┴───────┐     ┌──────┴───────┐
│      ai      │     │orchestration │     │  telemetry   │
│              │     │              │     │              │
│ Implements:  │     │    Uses:     │     │ Implements:  │
│ - AIClient   │     │ - AIClient   │     │ - Telemetry  │
└──────────────┘     │ - Discovery  │     └──────────────┘
                     └──────────────┘

Note: orchestration NEVER imports ai or telemetry directly!
```

#### Comparison with Telemetry Pattern

This follows the exact same pattern as telemetry integration:

| Aspect | Telemetry Pattern | AI Pattern |
|--------|------------------|------------|
| **Interface** | `core.Telemetry` | `core.AIClient` |
| **Implementation** | `telemetry` module | `ai` module |
| **Usage** | Components have `Telemetry` field | Orchestrator has `AIClient` field |
| **Initialization** | App calls `telemetry.Initialize()` | App creates `ai.NewClient()` |
| **Injection** | Set via `SetTelemetry()` | Pass via constructor |

#### Summary

This design is a textbook example of the **Dependency Inversion Principle**:
- High-level modules (orchestration) depend on abstractions (`core.AIClient`)
- Not on concrete implementations (`ai` module)
- This maintains architectural purity and enables true modularity

## 🏗️ How Everything Fits Together

### The Orchestra Metaphor - Complete Picture

The orchestration module offers **two modes** - the application chooses which to use:

```
                    ┌─────────────────────────────────────────────┐
                    │          🎭 User Request                     │
                    │     "Analyze Tesla and recommend action"     │
                    └──────────────────┬──────────────────────────┘
                                       │
                         ┌─────────────┴─────────────┐
                         │   Application chooses     │
                         │   which mode to use       │
                         └─────────────┬─────────────┘
                                       │
          ┌────────────────────────────┴────────────────────────────┐
          │                                                         │
          ▼                                                         ▼
┌──────────────────────┐                              ┌──────────────────────┐
│  🤖 AI Orchestrator  │                              │  📋 Workflow Engine  │
│  ProcessRequest()    │                              │  ExecuteWorkflow()   │
│                      │                              │                      │
│  "I'll figure out    │                              │  "I'll follow the    │
│   what to call"      │                              │   predefined recipe" │
└──────────┬───────────┘                              └──────────┬───────────┘
           │                                                     │
           ▼                                                     │
┌──────────────────────┐                                         │
│  🧠 LLM Planner      │                                         │
│                      │                                         │
│  Generates DAG-based │                                         │
│  execution plan      │                                         │
└──────────┬───────────┘                                         │
           │                                                     │
           └────────────────────┬────────────────────────────────┘
                                │
                      ┌─────────▼──────────┐
                      │   🎯 Executor      │ (The Stage Manager)
                      │                    │
                      │ Calls agents in    │
                      │ parallel when      │
                      │ possible           │
                      └─────────┬──────────┘
                                │
               ┌────────────────┼────────────────┐
               │                │                │
          ┌────▼───┐       ┌────▼───┐       ┌────▼───┐
          │  Tool  │       │  Tool  │       │ Agent  │ (The Musicians)
          │   A    │       │   B    │       │   C    │
          └────┬───┘       └────┬───┘       └────┬───┘
               │                │                │
               └────────────────┼────────────────┘
                                │
                      ┌─────────▼──────────┐
                      │  🎨 Synthesizer    │ (The Editor)
                      │                    │
                      │ Combines all       │
                      │ responses into     │
                      │ one answer         │
                      └─────────┬──────────┘
                                │
                      ┌─────────▼──────────┐
                      │   📜 Response      │
                      │                    │
                      │ "Tesla: BUY        │
                      │  Confidence: 85%   │
                      │  Reasons: ..."     │
                      └────────────────────┘
```

**Key difference:**
- **AI Orchestrator**: LLM dynamically generates the execution plan based on discovered capabilities
- **Workflow Engine**: Uses a predefined workflow definition (no LLM planning step)

### How Components Work Together

| Component | Role | Real-World Analogy |
|-----------|------|-------------------|
| **Discovery Service** | Finds where tools and agents live | Like DNS for your components |
| **Component Catalog** | Knows what each tool/agent can do | Like LinkedIn profiles for components |
| **Routing Cache** | Remembers recent decisions | Like muscle memory |
| **Executor** | Runs tools and agents efficiently | Like a project manager |
| **State Store** | Tracks workflow progress | Like a progress tracker |

## 📊 Performance & Caching Explained

### Why Caching Matters - The Restaurant Analogy

Imagine a restaurant where:
- **Without cache**: Every order requires calling suppliers to check prices (slow!)
- **With cache**: The menu has today's prices ready (fast!)

### Two Smart Caching Strategies

#### 1. Time-Based Cache (SimpleCache)
**How it works**: Like milk with an expiration date
```go
// "Remember this for 5 minutes"
cache.Set("tesla-analysis", result, 5*time.Minute)

// Later...
if cached := cache.Get("tesla-analysis"); cached != nil {
    return cached  // Instant response!
}
```

#### 2. LRU Cache (Least Recently Used)
**How it works**: Like a small notebook - when full, erase the oldest unused notes
```go
// Cache holds 100 most recent items
lruCache := NewLRUCache(100)

// Automatically removes least-used items when full
lruCache.Set("apple-data", data)  // Might remove "old-company-data"
```

### Configuring Cache for Your Needs

```go
config := orchestration.DefaultConfig()

// For frequently changing data (stock prices)
config.CacheEnabled = true
config.CacheTTL = 1 * time.Minute  // Short cache

// For stable data (company profiles)
config.CacheEnabled = true  
config.CacheTTL = 1 * time.Hour  // Long cache

// For real-time critical systems
config.CacheEnabled = false  // No cache, always fresh
```

## 🔍 Monitoring & Metrics - Know What's Happening

### Understanding Your System's Health

Think of metrics like your car's dashboard - they tell you if everything's running smoothly!

#### Key Metrics Explained

| Metric | What It Tells You | Why You Care |
|--------|------------------|--------------|
| **Total Requests** | How busy is your system? | Capacity planning |
| **Success Rate** | Are things working? | System health |
| **Average Latency** | How fast are responses? | User experience |
| **Component Failures** | Which tools/agents are struggling? | Troubleshooting |
| **Cache Hit Rate** | Is caching helping? | Performance tuning |

### Using Metrics in Practice

```go
// Get current metrics
metrics := orchestrator.GetMetrics()

// Check system health
successRate := float64(metrics.SuccessfulRequests) / float64(metrics.TotalRequests) * 100
if successRate < 95 {
    alert("Success rate below 95%!")
}

// Monitor performance
if metrics.AverageLatency > 5*time.Second {
    alert("System is slow!")
}

// Track specific workflows
fmt.Printf("Investment Analysis Workflow:\n")
fmt.Printf("  Executions: %d\n", metrics.WorkflowExecutions["investment-analysis"])
fmt.Printf("  Avg Duration: %v\n", metrics.WorkflowAvgDuration["investment-analysis"])
```

### Debugging with Metrics

```go
// When things go wrong, metrics help you find the problem:
if metrics.ComponentCallsFailed > 10 {
    // Check which tools/agents are failing
    for component, failures := range metrics.ComponentFailures {
        if failures > 5 {
            fmt.Printf("Component %s is having issues (%d failures)\n", component, failures)
        }
    }
}
```

## 🛠️ Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GOMIND_ORCHESTRATION_TIMEOUT` | `60s` | HTTP client timeout for tool/agent calls. For long-running AI workflows, set higher values (e.g., `5m`, `10m`). Uses Go duration format. |

```bash
# Example: Allow 5 minutes for AI-heavy workflows
export GOMIND_ORCHESTRATION_TIMEOUT=5m
```

### Programmatic Configuration

```go
config := &orchestration.OrchestratorConfig{
    // Routing mode
    RoutingMode: orchestration.ModeAutonomous,  // Options: ModeAutonomous, ModeWorkflow
    
    // Synthesis strategy  
    SynthesisStrategy: orchestration.StrategyLLM, // Options: StrategyLLM, StrategyTemplate, StrategySimple
    
    // Capability Provider (for scaling)
    CapabilityProviderType: "default",  // Options: "default" or "service"
    EnableFallback: true,                // Fallback to default provider on service failure
    
    // Execution configuration
    ExecutionOptions: orchestration.ExecutionOptions{
        MaxConcurrency:   5,                // Maximum parallel tool/agent calls
        StepTimeout:      30 * time.Second, // Timeout per step
        TotalTimeout:     2 * time.Minute,  // Overall execution timeout
        RetryAttempts:    2,                // Retry failed steps
        RetryDelay:       2 * time.Second,  // Delay between retries
        CircuitBreaker:   true,             // Enable circuit breaker
        FailureThreshold: 5,                // Circuit breaker threshold
        RecoveryTimeout:  30 * time.Second, // Circuit breaker recovery
    },
    
    // History and caching
    HistorySize:  100,              // Execution history buffer size
    CacheEnabled: true,              // Enable routing cache
    CacheTTL:     5 * time.Minute,  // Cache expiration time
}
```

## 📝 Usage Patterns

The orchestration module supports various usage patterns as demonstrated in the documentation above. Refer to the code examples in this README for implementation guidance.

## 🚦 Requirements

- **Redis** - For tool/agent discovery and state storage
- **OpenAI API Key** - For AI orchestration (or compatible LLM)
- **Running Components** - Tools and agents registered with discovery

## 🚀 Scaling to Hundreds of Agents - Capability Provider Architecture

### The Problem: Token Overflow at Scale

When you have hundreds or thousands of agents and tools, sending ALL their capabilities to the LLM causes:
- **Token limit overflow** (even with 1M+ token models)  
- **Increased costs** (more tokens = more money)
- **Slower responses** (processing huge contexts)

### The Solution: Smart Capability Discovery

The orchestration module provides two strategies:

#### 1. Default Provider (Small Scale: < 200 agents)
```go
// Sends ALL capabilities to LLM (original behavior)
config := orchestration.DefaultConfig()
config.CapabilityProviderType = "default"  // This is the default

// Simple, no external dependencies, perfect for getting started
```

#### 2. Service Provider (Large Scale: 100s-1000s of agents)

**Kubernetes (Recommended):** Use environment variable for the endpoint:
```bash
export GOMIND_CAPABILITY_SERVICE_URL="http://capability-service:8080"
```

```go
// Uses external RAG service for semantic search
// Endpoint is read from GOMIND_CAPABILITY_SERVICE_URL environment variable
config := orchestration.DefaultConfig()
config.CapabilityProviderType = "service"
config.CapabilityService = orchestration.ServiceCapabilityConfig{
    // Endpoint: automatically loaded from GOMIND_CAPABILITY_SERVICE_URL
    TopK:      20,       // Return top 20 most relevant agents
    Threshold: 0.7,      // Minimum relevance score
    Timeout:   10 * time.Second,
}
config.EnableFallback = true  // Fall back to default if service fails

deps := orchestration.OrchestratorDependencies{
    Discovery: discovery,
    AIClient:  aiClient,
}

orchestrator, _ := orchestration.CreateOrchestrator(config, deps)
```

> **Note:** If `GOMIND_CAPABILITY_SERVICE_URL` is not set and `CapabilityService.Endpoint` is empty, `CreateOrchestrator` returns an error. See [factory.go:79](factory.go#L79).

### How Service Provider Works

```
User Request: "Analyze customer sentiment"
         ↓
1. Query RAG Service with semantic search
         ↓
2. Service returns ONLY relevant agents:
   - sentiment-analyzer (score: 0.95)
   - text-processor (score: 0.88)
   - emotion-detector (score: 0.85)
         ↓
3. Send only these 3 to LLM (not all 1000!)
         ↓
4. LLM makes decision with focused context
```

### Production Configuration with Resilience

```go
// For production: Add circuit breaker and monitoring
import "github.com/itsneelabh/gomind/resilience"

// Create circuit breaker for the external service
cb, _ := resilience.NewCircuitBreaker(&resilience.CircuitBreakerConfig{
    Name:           "capability-service",
    ErrorThreshold: 0.5,
    VolumeThreshold: 10,
    SleepWindow:    30 * time.Second,
})

// Create logger for observability
logger := myapp.NewLogger()

// Configure with full resilience
config := orchestration.DefaultConfig()
config.CapabilityProviderType = "service"
config.CapabilityService = orchestration.ServiceCapabilityConfig{
    Endpoint:  "http://capability-service:8080",
    TopK:      50,  // More results for production
    Threshold: 0.8, // Higher quality threshold
}
config.EnableFallback = true  // Graceful degradation

deps := orchestration.OrchestratorDependencies{
    Discovery:      discovery,
    AIClient:       aiClient,
    CircuitBreaker: cb,      // Optional: Sophisticated resilience
    Logger:         logger,  // Optional: Structured logging
}

orchestrator, _ := orchestration.CreateOrchestrator(config, deps)
```

### Environment-Based Configuration

```bash
# Configure via environment variables
export GOMIND_CAPABILITY_SERVICE_URL="http://capability-service:8080"
export GOMIND_CAPABILITY_TOP_K="30"
export GOMIND_CAPABILITY_THRESHOLD="0.75"

# The orchestrator auto-configures when these are set
```

### Three Layers of Resilience

The service provider includes built-in resilience:

1. **Circuit Breaker** (if injected) - Prevents cascading failures
2. **Retry Logic** (built-in) - 3 retries with exponential backoff
3. **Fallback Provider** (configurable) - Falls back to default provider

### When to Use Each Provider

| Scenario | Provider | Why |
|----------|----------|-----|
| **Development/Testing** | Default | Simple, no dependencies |
| **< 200 agents** | Default | Token usage acceptable |
| **100s-1000s agents** | Service | Semantic search scales better |
| **Production critical** | Service + Circuit Breaker | Maximum resilience |

## ⚡ Performance Considerations

1. **Workflow Execution** - DAG-based execution with automatic parallelization
2. **Caching** - Use routing cache to reduce redundant LLM calls
3. **Discovery** - Component catalog refreshes every 10 seconds by default
4. **Concurrency** - Default 5 parallel tool/agent calls, configurable via `MaxConcurrency`
5. **Timeouts** - Configure appropriate timeouts for your use case
6. **Capability Provider** - Use service provider for 100s+ agents to avoid token overflow

## 🔮 Potential Enhancements

These features are not yet implemented but could be added:
- Visual workflow designer UI
- Distributed workflow execution across nodes
- Streaming response support
- WebSocket for real-time updates
- Workflow versioning and migration tools
- Custom capability provider implementations (e.g., GraphQL-based)

## 📖 API Reference

### Core Types
- `Orchestrator` - Main orchestration interface
- `WorkflowEngine` - Workflow execution engine
- `OrchestratorConfig` - Configuration structure
- `OrchestratorDependencies` - Dependency injection container
- `CapabilityProvider` - Interface for capability discovery
- `ServiceCapabilityConfig` - Configuration for service-based provider
- `WorkflowDefinition` - YAML workflow structure
- `ExecutionResult` - Execution results

### Key Functions
- `CreateOrchestrator(config, deps)` - Create orchestrator with dependencies
- `CreateSimpleOrchestrator(discovery, aiClient)` - Quick start orchestrator
- `CreateOrchestratorWithOptions(deps, opts...)` - Create with option functions
- `NewAIOrchestrator(config, discovery, aiClient)` - Low-level orchestrator creation
- `NewWorkflowEngine(discovery, stateStore, logger)` - Create workflow engine
- `ProcessRequest(ctx, request, metadata)` - Process natural language request
- `ExecuteWorkflow(ctx, workflow, inputs)` - Execute defined workflow
- `ParseWorkflowYAML(data)` - Parse workflow from YAML

### Configuration Options
- `WithCapabilityProvider(type, url)` - Configure capability provider type and URL
- `WithTelemetry(enabled)` - Enable/disable telemetry
- `WithFallback(enabled)` - Enable/disable fallback provider

## 💡 Best Practices & Tips

### The Journey from Prototype to Production

#### Phase 1: Exploration (Use AI Orchestration)
```go
// Start with natural language - let AI figure it out
response := orchestrator.ProcessRequest(ctx, 
    "analyze this company and tell me if I should invest", nil)
```

#### Phase 2: Pattern Recognition
```
// After a few runs, you notice the pattern:
// 1. Always fetches financials
// 2. Always checks news
// 3. Always runs sentiment analysis
// 4. Always generates report
```

#### Phase 3: Production (Create Workflow)
```yaml
# Now codify the pattern into a reliable workflow
name: investment-analysis
steps:
  - name: get-financials
  - name: check-news  
  - name: sentiment-analysis
  - name: generate-report
# Faster, cheaper, predictable!
```

### Golden Rules

1. **🎯 Start Simple**: Use AI mode to explore, then optimize with workflows
2. **🔍 Use Discovery**: Never hardcode agent URLs - let discovery find them
3. **⚡ Cache Smartly**: Cache stable data long, volatile data short
4. **📊 Monitor Everything**: If you can't measure it, you can't improve it
5. **🔄 Handle Failures**: Always configure retries and timeouts
6. **🚀 Think Parallel**: Design workflows to maximize parallelism

## 🆕 Production-Ready Enhancements

### LLM-First Hybrid Parameter Resolution

In multi-step workflows, parameters must flow between tools automatically. The orchestration module uses a **four-layer resolution system** where LLM handles all semantic understanding:

| Layer | Strategy | When Used | Cost |
|-------|----------|-----------|------|
| **Layer 1: Auto-Wiring** | Exact name match, case-insensitive match, type coercion | Always (first) | Free |
| **Layer 2: Micro-Resolution** | LLM extracts parameters via function calling | When Layer 1 leaves required params unmapped | 1 LLM call |
| **Layer 3: Error Analysis** | LLM analyzes tool errors and suggests corrections | When tool returns 400/404/409/422 | 1 LLM call |
| **Layer 4: Semantic Retry** | LLM computes parameters from full execution context | When Layer 3 says "cannot fix" but source data exists | 1 LLM call |

**How it works:**

```
Step 1 completes → Output: {"latitude": "48.85", "country": "France"}

┌─────────────────────────────────────────────────────────────┐
│ Layer 1: Auto-Wiring (instant, free)                        │
│   • Exact match: lat ← lat ✓                                │
│   • Type coercion: "48.85" → 48.85                          │
│   • Nested extraction: {code: "EUR"} → "EUR"                │
│                                                             │
│   NOTE: No semantic aliases - framework is domain-agnostic  │
└─────────────────────────────────────────────────────────────┘
         │
         ▼ (if required params still missing)
┌─────────────────────────────────────────────────────────────┐
│ Layer 2: Micro-Resolution (LLM call)                        │
│   • LLM understands "latitude" means "lat"                  │
│   • LLM infers "France" uses "EUR" currency                 │
│   • Guaranteed type safety via JSON schema                  │
└─────────────────────────────────────────────────────────────┘
         │
         ▼ (if tool call fails with correctable error)
┌─────────────────────────────────────────────────────────────┐
│ Layer 3: Error Analysis (LLM call)                          │
│   • Analyzes error: "City 'Tokio' not found"                │
│   • Suggests fix: {"city": "Tokyo"}                         │
│   • Retries with corrected parameters                       │
└─────────────────────────────────────────────────────────────┘
```

**Key Design Principle:** The framework contains **no domain-specific knowledge** (weather, currency, etc.). All semantic understanding is delegated to the LLM.

**Disabling LLM Layers:**
```go
// Disable Layer 2: Micro-resolution (auto-wiring only)
resolver := NewHybridResolver(aiClient, logger,
    WithMicroResolution(false))

// Disable Layer 3: Error Analysis (no LLM-based error recovery)
analyzer := NewErrorAnalyzer(aiClient, logger,
    WithErrorAnalysisEnabled(false))

// Runtime toggle for Layer 3
analyzer.Enable(false)  // Disable
analyzer.Enable(true)   // Re-enable
```

**Error Handling by HTTP Status:**

| Status | Handler | Action |
|--------|---------|--------|
| 400, 404, 409, 422 | LLM Error Analyzer → Semantic Retry | Analyze → correct → retry |
| 408, 429, 5xx | Resilience Module | Same payload + backoff |
| 401, 403, 405 | Neither | Fail immediately |

**Observability:**
- Span events: `llm.micro_resolution.*`, `error_analyzer.*`
- All LLM calls are traced in Jaeger with prompts, responses, and token usage

For detailed implementation information, see [INTELLIGENT_PARAMETER_BINDING.md](./INTELLIGENT_PARAMETER_BINDING.md).

### Layer 4: Semantic Retry (Contextual Re-Resolution)

**The Problem Solved:** When Layer 3 (Error Analysis) determines "this error cannot be fixed with different parameters" but the source data to compute the correct value actually exists, standard retry gives up. Semantic Retry uses the full execution trajectory to compute the correct parameters.

**Real-World Example:**
```
User: "Sell 100 Tesla shares and convert proceeds to EUR"

Step 1 (stock-tool): Returns {symbol: "TSLA", price: 468.285}
Step 2 (currency-tool): Called with {amount: 0} ← Layer 1/2 couldn't compute this!
        ↓
Tool returns 400: "amount must be greater than 0"
        ↓
Layer 3 (Error Analysis): "Cannot fix - don't know what amount should be"
        ↓
🆕 Layer 4 (Semantic Retry): Has access to:
   • User query: "Sell 100 Tesla shares..."
   • Source data: {price: 468.285}
   • Failed params: {amount: 0}

   LLM computes: 100 × 468.285 = 46828.5
        ↓
Retries with: {amount: 46828.5} ✅ SUCCESS!
```

**How It Works:**
```
┌─────────────────────────────────────────────────────────────┐
│ Layer 4: Semantic Retry (enabled by default)                 │
│                                                              │
│   Triggers when:                                             │
│   • Tool returns 4xx error (400, 404, 409, 422)             │
│   • Layer 3 says "cannot fix"                                │
│   • Source data exists from dependent steps                  │
│                                                              │
│   The LLM receives:                                          │
│   • User's original query (intent)                           │
│   • All source data from previous steps                      │
│   • Failed parameters and error message                      │
│   • Target capability schema                                 │
│                                                              │
│   Returns:                                                   │
│   • should_retry: true/false                                 │
│   • corrected_parameters: computed values                    │
│   • analysis: explanation of the fix                         │
└─────────────────────────────────────────────────────────────┘
```

**Configuration:**
```go
config := orchestration.DefaultConfig()

// Semantic retry is enabled by default
config.SemanticRetry.Enabled = true        // Default: true
config.SemanticRetry.MaxAttempts = 2       // Default: 2

// Disable for cost-sensitive deployments
config.SemanticRetry.Enabled = false
```

**Environment Variables:**
```bash
# Enable/disable semantic retry (default: true)
export GOMIND_SEMANTIC_RETRY_ENABLED=true

# Maximum retry attempts (default: 2)
export GOMIND_SEMANTIC_RETRY_MAX_ATTEMPTS=2
```

**When Semantic Retry Activates:**

| Condition | Layer 4 Activates? |
|-----------|-------------------|
| Tool returns 400 + Layer 3 says "cannot fix" | ✅ Yes |
| Tool returns 500 (server error) | ❌ No (handled by resilience) |
| Tool returns 401 (auth error) | ❌ No (not retryable) |
| Layer 3 successfully corrects | ❌ No (already fixed) |
| No source data from dependencies | ❌ No (nothing to compute from) |

**Observability:**
- Span events: `contextual_re_resolution.start`, `contextual_re_resolution.complete`
- Metrics: `orchestration.semantic_retry.success`, `orchestration.semantic_retry.cannot_fix`
- Full visibility in Jaeger traces

**Key Insight:** Semantic Retry succeeds where static rules fail because it has access to:
1. **User intent** - understands what computation is needed
2. **Source data** - has the values to compute from
3. **Error context** - knows exactly what went wrong

This is the **same reasoning a human developer would apply** when debugging a failed API call, now automated by the framework.

### Comprehensive Logging System
The orchestration module now includes production-grade logging for all operations:

- **Workflow Execution Logging**: Track every step of workflow execution with structured logs
- **AI Decision Logging**: Capture AI orchestrator decisions and reasoning
- **Component Interaction Logging**: Log all tool and agent interactions
- **Error Context Logging**: Detailed error information with full context

#### Logging Examples:
```go
// Workflow step execution
logger.InfoWithContext(ctx, "Executing workflow step", map[string]interface{}{
    "workflow": "data-analysis",
    "step": "fetch-data",
    "attempt": 1,
})

// AI orchestration decision
logger.DebugWithContext(ctx, "AI selected components", map[string]interface{}{
    "query": "analyze Tesla stock",
    "selected_tools": []string{"stock-fetcher", "analyzer"},
    "confidence": 0.95,
})

// Error with context
logger.ErrorWithContext(ctx, "Workflow step failed", map[string]interface{}{
    "workflow": "report-generation",
    "step": "pdf-export",
    "error": err.Error(),
    "retry_count": 3,
})
```

### Enhanced Service Discovery
- **Improved Registry Performance**: Optimized component lookup and caching
- **Better Error Recovery**: Automatic retry on transient discovery failures
- **Detailed Metrics**: Track discovery latency and success rates

### Internal Capability Flag

Capabilities can be marked as "internal" to exclude them from the LLM planning catalog while keeping them HTTP-callable. This prevents self-referential orchestration bugs where an orchestrator agent might recursively call itself.

**Problem Solved:**
When an orchestrator agent registers its `orchestrate_natural` capability, the LLM might include it in execution plans, causing recursive self-calls with 400 errors. The `Internal` flag prevents this by filtering internal capabilities from the catalog sent to the LLM.

**Usage:**
```go
// Mark orchestration capabilities as internal to prevent LLM from calling them
agent.RegisterCapability(core.Capability{
    Name:        "orchestrate_natural",
    Description: "Process natural language requests with AI orchestration",
    Endpoint:    "/orchestrate/natural",
    Internal:    true,  // Exclude from LLM catalog
    Handler:     handleOrchestration,
})
```

**Key Behaviors:**
| Behavior | Internal: true | Internal: false (default) |
|----------|----------------|---------------------------|
| HTTP callable | ✅ Yes | ✅ Yes |
| In LLM catalog | ❌ No | ✅ Yes |
| In `FormatForLLM()` output | ❌ No | ✅ Yes |

**When to Use `Internal: true`:**
- Orchestration endpoints (prevent recursive planning)
- Admin/maintenance endpoints
- Deprecated capabilities (still accessible but hidden from AI)
- Health check or metrics endpoints

**Backward Compatibility:**
- `Internal` defaults to `false` (Go zero value)
- Existing capabilities without the field remain public
- No changes required for existing tool/agent code

### Workflow Engine Improvements
- **Parallel Step Execution**: Execute independent steps concurrently
- **Step Timeout Configuration**: Set timeouts per workflow step
- **Conditional Branching**: Support for if/else logic in workflows
- **Error Handling Strategies**: Configure retry, skip, or fail strategies per step

## 🎓 Summary - What You've Learned

### This Module Gives You Two Superpowers:

#### 1. **AI Orchestration** - The Smart Assistant
- Understands natural language
- Figures out which tools and agents to call
- Adapts to available components
- Perfect for exploration and dynamic tasks

#### 2. **Workflow Engine** - The Reliable Machine
- Follows exact recipes
- Maximizes parallelism automatically
- Handles failures gracefully
- Perfect for production and repeated tasks

### Remember the Coffee Shop

Just like a coffee shop needs someone to:
- Take orders (orchestrator)
- Coordinate workers (executor)
- Ensure quality (synthesizer)
- Serve customers (response)

This module does the same for your tools and agents!

### Quick Decision Guide

**Choose AI Orchestration when:**
- You're exploring what's possible
- Requirements change frequently
- You want natural language interface
- Flexibility is more important than speed

**Choose Workflows when:**
- You know exactly what needs to happen
- You need predictable performance
- You want to minimize costs (no LLM calls)
- Reliability is critical

### The Power of Both

The real magic happens when you use both:
1. **Explore** with AI orchestration
2. **Discover** patterns that work
3. **Codify** into workflows
4. **Deploy** with confidence

---

**🎉 Congratulations!** You now understand how to conduct your component orchestra. Whether you choose AI's flexibility or workflows' reliability (or both!), you have the tools to build powerful multi-agent systems with both passive tools and active agents.