# GoMind Workflow Engine

## 🎯 Simple Explanation: What is the Workflow Engine?

Think of the workflow engine as a **recipe executor** for multi-agent tasks. Just like a recipe tells you what ingredients to use and in what order, a workflow tells the system which agents to call and how to coordinate them.

### The Coffee Shop Analogy

Imagine you're running a coffee shop with different workers:
- **Barista** - Makes coffee
- **Cashier** - Takes orders
- **Baker** - Makes pastries

A workflow is like the process for fulfilling a customer order:
1. Cashier takes the order
2. Barista makes the coffee AND Baker prepares pastry (in parallel)
3. Cashier assembles everything and serves the customer

The workflow engine makes sure everything happens in the right order!

## 🔧 How It Works

### 1. Discovery Integration - Finding Your Workers

The workflow engine doesn't need to know WHERE your agents are - it uses Discovery to find them:

```yaml
# Your workflow just uses names
steps:
  - name: get-price
    agent: stock-price-tool  # Discovery finds where this lives!
    action: fetch_price
```

**What happens behind the scenes:**
1. Workflow: "I need stock-price-tool"
2. Discovery: "It's at http://192.168.1.10:8080"
3. Workflow calls the agent and gets the result

### 2. Capability-Based Discovery - "I need someone who can..."

Even cooler - you don't even need to know the agent's name:

```yaml
steps:
  - name: analyze-news
    capability: news_analysis  # Find ANY agent that can analyze news
    action: analyze
```

The workflow asks Discovery: "Who can do news_analysis?" and uses whoever is available!

### 3. DAG Execution - Smart Dependency Management

DAG = Directed Acyclic Graph. Sounds complex? It's actually simple:

```yaml
steps:
  - name: A              # Runs first
  - name: B
    depends_on: [A]      # Runs after A
  - name: C
    depends_on: [A]      # Also runs after A (parallel with B!)
  - name: D
    depends_on: [B, C]   # Waits for both B and C
```

This creates an execution plan:
```
    A
   / \
  B   C
   \ /
    D
```

The engine automatically figures out that B and C can run in parallel!

## 📝 Writing Your First Workflow

### Basic Example: Analyze Stock

```yaml
name: analyze-stock
version: "1.0"

inputs:
  symbol:
    type: string
    required: true

steps:
  # Step 1: Get stock price
  - name: get-price
    agent: stock-price-tool
    action: get_current_price
    inputs:
      symbol: ${inputs.symbol}
    
  # Step 2: Analyze the price
  - name: analyze
    agent: technical-analyst
    action: analyze
    inputs:
      price_data: ${steps.get-price.output}  # Use output from step 1
    depends_on: [get-price]
    
outputs:
  recommendation: ${steps.analyze.output.recommendation}
```

## 🚀 Using the Workflow Engine

### In Go Code

```go
// 1. Create the engine
engine := orchestration.NewWorkflowEngine(discovery)

// 2. Load your workflow
yamlData, _ := os.ReadFile("workflow.yaml")
workflow, _ := engine.ParseWorkflowYAML(yamlData)

// 3. Execute with inputs
inputs := map[string]interface{}{
    "symbol": "TSLA",
}

execution, err := engine.ExecuteWorkflow(ctx, workflow, inputs)

// 4. Get results
fmt.Printf("Result: %v\n", execution.Outputs["recommendation"])
```

## 🎯 Key Features

### 1. Parallel Execution
```yaml
steps:
  # These run at the same time!
  - name: task1
  - name: task2
  - name: task3
    
  # This waits for all three
  - name: combine
    depends_on: [task1, task2, task3]
```

### 2. Error Handling & Retry
```yaml
steps:
  - name: flaky-service
    retry:
      max_attempts: 3
      backoff: exponential
      initial_wait: 1s
    on_error: continue  # Don't fail the whole workflow
```

### 3. Dynamic Service Discovery
```yaml
steps:
  - name: process-data
    capability: data_processor  # Finds any available processor
    prefer_local: true          # Prefers services in same cluster
```

### 4. State Persistence
- Workflows save their state to Redis
- Can resume after failures
- Track execution history

## 🔍 Real-World Example: Multi-Agent Analysis

```yaml
name: investment-analysis
steps:
  # Gather data in parallel
  - name: market-data
    capability: market_data_provider
    action: get_data
    
  - name: news
    capability: news_aggregator
    action: get_news
    
  - name: social-sentiment
    capability: social_media_analyzer
    action: analyze_sentiment
    
  # Analyze everything
  - name: ai-analysis
    agent: ai-advisor
    action: analyze
    inputs:
      market: ${steps.market-data.output}
      news: ${steps.news.output}
      sentiment: ${steps.social-sentiment.output}
    depends_on: [market-data, news, social-sentiment]
    
  # Generate report
  - name: report
    agent: report-generator
    action: create_report
    inputs:
      analysis: ${steps.ai-analysis.output}
    depends_on: [ai-analysis]
```

## 🎨 Advanced Features

### Conditional Execution
```yaml
steps:
  - name: check-market
    agent: market-checker
    
  - name: buy-stock
    condition:
      if: ${steps.check-market.output.is_open}
      then: execute
      else: skip
```

### Scatter-Gather Pattern
```yaml
steps:
  - name: get-opinions
    scatter_gather:
      capability: market_analyst
      max_instances: 5  # Ask up to 5 different analysts
    aggregate: average_scores
```

### Timeouts and Deadlines
```yaml
timeout: 2m  # Overall workflow timeout

steps:
  - name: quick-task
    timeout: 10s  # Step-specific timeout
```

## 📊 Monitoring & Metrics

The workflow engine tracks:
- Execution times
- Success/failure rates
- Step-level metrics
- Parallelism efficiency

```go
metrics := engine.GetMetrics()
fmt.Printf("Success rate: %.2f%%\n", metrics.SuccessRate * 100)
fmt.Printf("Average time: %v\n", metrics.AverageTime)
```

## 🔄 Workflow Lifecycle

1. **Parse** - Load and validate workflow
2. **Build DAG** - Create execution graph
3. **Discover Services** - Find agents/tools
4. **Execute** - Run steps in order/parallel
5. **Persist State** - Save progress
6. **Handle Errors** - Retry/fail as configured
7. **Collect Results** - Gather outputs
8. **Complete** - Return final result

## 💡 Best Practices

1. **Keep workflows simple** - Each workflow should do one thing well
2. **Use capabilities over names** - More flexible and resilient
3. **Handle errors gracefully** - Use retries and fallbacks
4. **Set reasonable timeouts** - Prevent hanging workflows
5. **Monitor execution** - Track metrics and failures
6. **Version your workflows** - Maintain compatibility

## 🚦 Common Patterns

### Sequential Processing
```yaml
A → B → C → D
```

### Parallel Processing
```yaml
    A
   /|\
  B C D
   \|/
    E
```

### Pipeline
```yaml
A → B → C
    ↓   ↓
    D → E
```

### Fan-out/Fan-in
```yaml
A → [B1, B2, B3, ...Bn] → C
```

## 🛠️ Troubleshooting

**Q: My workflow is stuck**
- Check for circular dependencies
- Verify all agents are healthy
- Look for timeout issues

**Q: Steps aren't running in parallel**
- Ensure no hidden dependencies
- Check worker pool size
- Verify services are available

**Q: Can't find my agent**
- Check Discovery registration
- Verify agent name/capability
- Ensure agent is healthy

## 📚 Summary

The workflow engine makes it easy to:
1. **Coordinate multiple agents** without knowing where they are
2. **Run tasks in parallel** when possible
3. **Handle failures gracefully** with retries
4. **Track execution state** for monitoring
5. **Build complex multi-agent systems** with simple YAML

Think of it as the conductor of an orchestra - it doesn't play the instruments (agents), but it makes sure everyone plays at the right time to create beautiful music (complete the task)!