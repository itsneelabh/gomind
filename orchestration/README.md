# GoMind Orchestration Module

Multi-agent coordination with AI-driven orchestration and declarative workflows.

## 🎯 What Does This Module Do?

Think of this module as the **conductor of an orchestra**. Just like a conductor coordinates musicians to create beautiful music, this module coordinates multiple agents to accomplish complex tasks.

It provides two powerful ways to orchestrate agents:

1. **AI Orchestration** - Tell it what you want in natural language, and AI figures out which agents to call
2. **Workflow Engine** - Define step-by-step "recipes" that execute reliably every time

### Real-World Analogy: The Coffee Shop

Imagine running a coffee shop with different workers:
- **Barista** - Makes coffee (like a data processing agent)
- **Cashier** - Takes orders (like an API gateway agent)
- **Baker** - Makes pastries (like a report generator agent)

The orchestration module ensures:
1. The cashier takes the order
2. The barista and baker work **in parallel** (no waiting!)
3. Everything comes together for the customer

That's exactly how it coordinates your agents!

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
// 2. Looks at available agents (stock-service, news-service, analyzer)
// 3. Creates an execution plan
// 4. Calls agents in the right order
// 5. Combines results into a coherent response
```

#### Option 2: Workflow (Predictable Recipes)
```yaml
# Define exact steps like a recipe
name: analyze-stock
steps:
  - name: get-price          # Step 1: Get the data
    agent: stock-service
    action: fetch_price
    
  - name: get-news           # Step 2: Get news (parallel with step 1!)
    agent: news-service
    action: fetch_latest
    
  - name: analyze            # Step 3: Analyze everything
    agent: ai-analyzer
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
2. AI checks available agents: "I have stock-price, news, and analyzer agents"
     ↓
3. AI creates plan: "First get price and news, then analyze both"
     ↓
4. Executes plan: Calls agents in parallel where possible
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
| **Agent Catalog** | Keeps track of all available agents and what they can do | Like a phone book of workers and their skills |
| **Smart Executor** | Runs multiple agent calls in parallel when possible | Like a project manager coordinating team members |
| **AI Synthesizer** | Combines responses from multiple agents into one answer | Like an editor combining reporter stories into one article |
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
✓ financial-agent     -> can get financials
✓ news-agent         -> can get news
✓ sentiment-agent    -> can analyze sentiment
✓ technical-agent    -> can do technical analysis

AI decides: "I'll use all four agents!"
```

**Step 3: Smart Planning (Creating Execution Order)**
```javascript
AI creates plan:
1. [Parallel Group 1]
   - financial-agent: get_financials("TSLA")
   - news-agent: get_recent_news("Tesla")
   
2. [Parallel Group 2] 
   - sentiment-agent: analyze(news_data)
   - technical-agent: analyze(financial_data)
   
3. [Final Step]
   - Synthesize all results into coherent analysis
```

**Step 4: Synthesis (Making Sense of Everything)**
```javascript
Agent responses:
- Financial: "Revenue $96B, up 35% YoY..."
- News: "Tesla announces new factory..."
- Sentiment: "72% positive sentiment..."
- Technical: "RSI 65, bullish trend..."
          ↓
AI combines into:
"Tesla shows strong growth with $96B revenue (+35% YoY).
Recent factory announcement drives positive sentiment (72%).
Technical indicators suggest continued bullish trend.
Recommendation: Strong Buy"
```

### Setting Up AI Orchestration

```go
// Step 1: Set up discovery (the phone book for agents)
discovery := core.NewRedisDiscovery("redis://localhost:6379")

// Step 2: Set up AI (the brain)
aiClient := ai.NewOpenAIClient(apiKey)

// Step 3: Create orchestrator (the conductor)
config := orchestration.DefaultConfig()
config.CacheEnabled = true  // Remember recent decisions
config.ExecutionOptions.MaxConcurrency = 10  // Run up to 10 agents at once

orchestrator := orchestration.NewAIOrchestrator(
    config,
    discovery,
    aiClient,
)

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

#### 1. Direct Agent Discovery
```yaml
steps:
  - name: get-price
    agent: stock-price-service  # "I want THIS specific agent"
    action: fetch_price
```

#### 2. Capability-Based Discovery  
```yaml
steps:
  - name: analyze-text
    capability: sentiment_analysis  # "I need ANY agent that can do this"
    action: analyze
    # Engine finds: sentiment-analyzer-v1, sentiment-analyzer-v2, etc.
    # Picks the best one (healthy, lowest load)
```

#### 3. Dynamic Service Discovery
```yaml
# No hardcoded URLs needed!
# Workflow says: "I need financial-advisor"
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
    agent: market-data-service
    action: fetch_price
    inputs:
      symbol: ${inputs.symbol}
    timeout: 5s
    
  - name: get-news
    capability: news_aggregation  # Find ANY news agent
    action: fetch_recent
    inputs:
      query: ${inputs.symbol}
      limit: 10
    
  - name: get-sentiment
    agent: social-sentiment
    action: analyze
    inputs:
      symbol: ${inputs.symbol}
    retry:  # Handle flaky services
      max_attempts: 3
      backoff: exponential
      initial_wait: 1s
    
  # Phase 2: Analysis (waits for data, then parallel)
  - name: technical-analysis
    agent: technical-analyzer
    action: analyze_technicals
    inputs:
      price_data: ${steps.get-price.output}
    depends_on: [get-price]
    
  - name: news-analysis
    agent: ai-news-analyzer
    action: analyze_impact
    inputs:
      articles: ${steps.get-news.output}
      sentiment: ${steps.get-sentiment.output}
    depends_on: [get-news, get-sentiment]
    
  # Phase 3: Generate Report (waits for all analysis)
  - name: final-report
    agent: ai-advisor
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
engine := orchestration.NewWorkflowEngine(discovery)

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
    agent: data-fetcher
    action: get_data
    # This step produces output
    
  - name: step-two
    agent: processor
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

## 🎭 When to Use Each Mode

### Use AI Orchestration When:
- Processing natural language requests
- Agent selection needs to be dynamic
- Tasks require intelligent routing decisions
- Exploring new agent combinations

### Use Workflows When:
- Processes are well-defined and repeatable
- You need guaranteed execution order
- Predictable performance is important
- Avoiding LLM costs for routine tasks

## 🏗️ How Everything Fits Together

### The Orchestra Metaphor - Complete Picture

```
┌─────────────────────────────────────────────┐
│          🎭 User Request                     │
│     "Analyze Tesla and recommend action"     │
└──────────────────┬──────────────────────────┘
                   │
        ┌──────────▼──────────┐
        │   🎼 Orchestrator   │ (The Conductor)
        │  Decides: AI or     │
        │  Workflow approach? │
        └──────────┬──────────┘
                   │
      ┌────────────┴────────────┐
      │                         │
┌─────▼─────┐           ┌───────▼────────┐
│ 🤖 AI     │           │ 📋 Workflow    │
│ Router    │           │ Engine         │
│           │           │                │
│ "Let me   │           │ "I'll follow   │
│ figure    │           │ the recipe"    │
│ this out" │           │                │
└─────┬─────┘           └───────┬────────┘
      │                         │
      └───────────┬─────────────┘
                  │
        ┌─────────▼──────────┐
        │   🎯 Executor      │ (The Stage Manager)
        │                    │
        │ Calls agents in    │
        │ parallel when      │
        │ possible           │
        └─────────┬──────────┘
                  │
     ┌────────────┼────────────┐
     │            │            │
┌────▼───┐  ┌────▼───┐  ┌────▼───┐
│ Agent  │  │ Agent  │  │ Agent  │ (The Musicians)
│   A    │  │   B    │  │   C    │
└────┬───┘  └────┬───┘  └────┬───┘
     │            │            │
     └────────────┼────────────┘
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

### How Components Work Together

| Component | Role | Real-World Analogy |
|-----------|------|-------------------|
| **Discovery Service** | Finds where agents live | Like DNS for your agents |
| **Agent Catalog** | Knows what each agent can do | Like LinkedIn profiles for agents |
| **Routing Cache** | Remembers recent decisions | Like muscle memory |
| **Executor** | Runs agents efficiently | Like a project manager |
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
| **Agent Failures** | Which agents are struggling? | Troubleshooting |
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
if metrics.AgentCallsFailed > 10 {
    // Check which agents are failing
    for agent, failures := range metrics.AgentFailures {
        if failures > 5 {
            fmt.Printf("Agent %s is having issues (%d failures)\n", agent, failures)
        }
    }
}
```

## 🛠️ Configuration

```go
config := &orchestration.OrchestratorConfig{
    // Routing mode
    RoutingMode: orchestration.ModeHybrid,  // Options: ModeAutonomous, ModeWorkflow, ModeHybrid
    
    // Synthesis strategy  
    SynthesisStrategy: orchestration.StrategyLLM, // Options: StrategyLLM, StrategyTemplate, StrategySimple
    
    // Execution configuration
    ExecutionOptions: orchestration.ExecutionOptions{
        MaxConcurrency:   5,                // Maximum parallel agent calls
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

- **Redis** - For agent discovery and state storage
- **OpenAI API Key** - For AI orchestration (or compatible LLM)
- **Running Agents** - Services registered with discovery

## ⚡ Performance Considerations

1. **Workflow Execution** - DAG-based execution with automatic parallelization
2. **Caching** - Use routing cache to reduce redundant LLM calls
3. **Discovery** - Agent catalog refreshes every 10 seconds by default
4. **Concurrency** - Default 5 parallel agent calls, configurable via `MaxConcurrency`
5. **Timeouts** - Configure appropriate timeouts for your use case

## 🔮 Potential Enhancements

These features are not yet implemented but could be added:
- Visual workflow designer UI
- Distributed workflow execution across nodes
- Semantic similarity-based agent discovery
- Streaming response support
- WebSocket for real-time updates
- Workflow versioning and migration tools

## 📖 API Reference

### Core Types
- `Orchestrator` - Main orchestration interface
- `WorkflowEngine` - Workflow execution engine
- `OrchestratorConfig` - Configuration structure
- `WorkflowDefinition` - YAML workflow structure
- `ExecutionResult` - Execution results

### Key Functions
- `NewAIOrchestrator()` - Create AI-powered orchestrator
- `NewWorkflowEngine()` - Create workflow engine
- `ProcessRequest()` - Process natural language request
- `ExecuteWorkflow()` - Execute defined workflow
- `ParseWorkflowYAML()` - Parse workflow from YAML

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

## 🎓 Summary - What You've Learned

### This Module Gives You Two Superpowers:

#### 1. **AI Orchestration** - The Smart Assistant
- Understands natural language
- Figures out which agents to call
- Adapts to available agents
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

This module does the same for your agents!

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

**🎉 Congratulations!** You now understand how to conduct your agent orchestra. Whether you choose AI's flexibility or workflows' reliability (or both!), you have the tools to build powerful multi-agent systems.