# Async Task Orchestration Guide

Welcome to the complete guide on async tasks in GoMind! Think of this as your friendly mentor sitting next to you, explaining how to build agents that handle long-running operations without blocking. Grab a coffee, and let's dive in!

## Table of Contents

- [What Are Async Tasks and Why Should You Care?](#what-are-async-tasks-and-why-should-you-care)
- [The Solution: HTTP 202 + Polling Pattern](#the-solution-http-202--polling-pattern)
- [Understanding the Architecture](#understanding-the-architecture)
- [Quick Start: Your First Async Agent](#quick-start-your-first-async-agent)
  - [Understanding the Request Format](#understanding-the-request-format)
  - [Framework vs Developer Responsibilities](#framework-vs-developer-responsibilities)
- [Deployment Modes](#deployment-modes)
- [Writing Task Handlers](#writing-task-handlers)
- [Progress Reporting](#progress-reporting)
- [Distributed Tracing Across Async Boundaries](#distributed-tracing-across-async-boundaries)
- [Monitoring and Metrics](#monitoring-and-metrics)
- [Configuration Reference](#configuration-reference)
  - [Implementing Custom Backends](#implementing-custom-backends)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)
- [Related Documentation](#related-documentation)

---

## What Are Async Tasks and Why Should You Care?

### The Problem: AI Tasks Take Minutes to Hours

AI agent workflows aren't like typical web requests that complete in milliseconds. They involve complex operations that can take **minutes to hours**:

| Factor | Typical Duration | Example |
|--------|------------------|---------|
| **LLM Latency** | 5-60s per call | Complex reasoning chains with multiple LLM invocations |
| **External APIs** | 10-120s | Rate-limited APIs, slow third-party services |
| **Data Processing** | 1-30 min | Large document analysis, embeddings generation |
| **Human-in-the-Loop** | Minutes to hours | Waiting for approvals or input |
| **Multi-Agent Coordination** | Variable | Sequential agent handoffs, consensus building |

**Example: A research agent workflow**
```
Step 1: Search 5 sources        → 30s each = 2.5 min
Step 2: Analyze results         → 60s
Step 3: Synthesize with LLM     → 60s
Step 4: Generate report         → 30s
────────────────────────────────────────────
Total: ~4+ minutes
```

This is a fundamental mismatch with synchronous HTTP request-response patterns.

### The Restaurant Analogy

Think of it like a busy restaurant:

**Synchronous (without async tasks):**
1. You order a complex dish (AI-orchestrated research)
2. The waiter stands frozen at your table until it's ready
3. Other customers can't get service
4. The restaurant grinds to a halt

**Asynchronous:**
1. You order a complex dish
2. Waiter takes your order number: "Your order #123 is in the kitchen"
3. Waiter serves other customers
4. You check the display board: "Order #123: 50% complete"
5. Order arrives when ready

**That order number is exactly what a Task ID does for long-running operations!**

### Why Synchronous Doesn't Work

```
┌─────────────────────────────────────────────────────────────────┐
│                    Current Synchronous Flow                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Client ──HTTP POST──> Server ──Process──> Server ──Response──>  │
│                                                                  │
│  [────────────── Connection held open ──────────────]            │
│                                                                  │
│  Problems:                                                       │
│  • HTTP timeouts (server, load balancer, browser)                │
│  • Connection drops on network issues                            │
│  • No progress visibility                                        │
│  • Server resources tied up                                      │
│  • Client retries cause duplicate work                           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

Without async:
- HTTP requests timeout before complex AI workflows complete
- Users have no idea what's happening during processing
- Server threads blocked waiting, limiting concurrency
- No way to cancel mid-flight operations
- Client timeout retries cause duplicate processing

With async:
- Client gets immediate response (Task ID) in milliseconds
- Worker processes in background with no time pressure
- Client polls for progress updates
- Scalable to thousands of concurrent tasks
- Tasks can be cancelled at any time

---

## The Solution: HTTP 202 + Polling Pattern

The solution is elegantly simple: **return immediately with a task ID, process in background, let clients poll for status**.

### How It Works

```
┌─────────────────────────────────────────────────────────────────────┐
│                         CLIENT REQUEST                               │
│                    POST /api/v1/tasks                                │
│                    {"type": "query", "input": {...}}                 │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         API SERVER                                   │
│                                                                      │
│  1. Validate request                                                 │
│  2. Create Task with UUID                                            │
│  3. Enqueue to Redis                                                 │
│  4. Return HTTP 202 Accepted                                         │
│                                                                      │
│     Response: {"task_id": "abc123", "status": "queued"}              │
└───────────────────────────┬─────────────────────────────────────────┘
                            │ (immediate, ~10ms)
                            │
┌───────────────────────────┴─────────────────────────────────────────┐
│                         REDIS QUEUE                                  │
│                                                                      │
│  Queue: [task-abc123, task-def456, ...]                             │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      BACKGROUND WORKER                               │
│                                                                      │
│  1. BRPOP from queue (blocks waiting for tasks)                     │
│  2. Load task from Redis                                             │
│  3. Execute handler (AI orchestration)                               │
│  4. Report progress via ProgressReporter                             │
│  5. Save result to Redis                                             │
│  6. Mark task complete                                               │
└─────────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      CLIENT POLLS                                    │
│                                                                      │
│  GET /api/v1/tasks/abc123                                            │
│                                                                      │
│  Poll 1: {"status": "running", "progress": {"percentage": 25}}       │
│  Poll 2: {"status": "running", "progress": {"percentage": 75}}       │
│  Poll 3: {"status": "completed", "result": {...}}                    │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Understanding the Architecture

Before diving into code, let's understand the components.

> **Note: Pluggable Backend Design**
>
> GoMind's async task system uses an **interface-first design**. The `TaskQueue` and `TaskStore` interfaces are defined in the `core` module, while **Redis implementations are provided as defaults**. You can implement these interfaces for other backends (PostgreSQL, in-memory for testing, etc.) if needed.
>
> This guide uses the Redis implementations throughout. See [Configuration Reference](#configuration-reference) for Redis configuration details, or [Implementing Custom Backends](#implementing-custom-backends) for guidance on creating your own implementations.

### Component Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                    ASYNC TASK SYSTEM                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ core.TaskQueue interface → orchestration.RedisTaskQueue     │    │
│  │ ─────────────────────────────────────────────────────────── │    │
│  │ • Enqueue(task) - Add task to processing queue              │    │
│  │ • Dequeue()     - Blocking pop from queue                   │    │
│  │ • Default impl uses Redis LIST for reliable queuing         │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ core.TaskStore interface → orchestration.RedisTaskStore     │    │
│  │ ─────────────────────────────────────────────────────────── │    │
│  │ • Create(task)  - Persist new task                          │    │
│  │ • Get(id)       - Retrieve task by ID                       │    │
│  │ • Update(task)  - Update progress/result                    │    │
│  │ • Cancel(id)    - Mark task as cancelled                    │    │
│  │ • Default impl uses Redis STRING for JSON storage           │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ TaskWorkerPool (orchestration.TaskWorkerPool)               │    │
│  │ ─────────────────────────────────────────────────────────── │    │
│  │ • RegisterHandler(type, fn) - Map task types to handlers    │    │
│  │ • Start(ctx)                - Start N worker goroutines     │    │
│  │ • Graceful shutdown with in-flight task completion          │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ TaskAPIHandler (orchestration.TaskAPIHandler)               │    │
│  │ ─────────────────────────────────────────────────────────── │    │
│  │ • HandleSubmit()   - POST /api/v1/tasks                     │    │
│  │ • HandleGetTask()  - GET /api/v1/tasks/:id                  │    │
│  │ • HandleCancel()   - POST /api/v1/tasks/:id/cancel          │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ ProgressReporter (core.ProgressReporter)                    │    │
│  │ ─────────────────────────────────────────────────────────── │    │
│  │ • Report(progress) - Update task progress in real-time      │    │
│  │ • Enables per-step visibility during execution              │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Data Flow

1. **Submit**: Client → API Server → TaskStore.Create() → TaskQueue.Enqueue()
2. **Process**: Worker → TaskQueue.Dequeue() → Handler() → ProgressReporter.Report()
3. **Poll**: Client → API Server → TaskStore.Get() → Return status/progress/result

---

## Quick Start: Your First Async Agent

Let's build an async agent step by step.

> **Working Example**: See [examples/agent-with-async/](../examples/agent-with-async/) for a complete implementation.

### Step 1: Create Project Structure

```
my-async-agent/
├── main.go           # Entry point with deployment mode logic
├── handlers.go       # Task handler implementations
├── agent.go          # Agent struct and initialization
├── go.mod
├── Dockerfile
├── k8-deployment.yaml
└── setup.sh          # Deployment helper script
```

### Step 2: Initialize Redis-Backed Infrastructure

```go
// main.go
package main

import (
    "context"
    "log"
    "os"

    "github.com/go-redis/redis/v8"
    "github.com/itsneelabh/gomind/orchestration"
)

func main() {
    // Connect to Redis
    redisURL := os.Getenv("REDIS_URL")
    redisOpt, err := redis.ParseURL(redisURL)
    if err != nil {
        log.Fatalf("Failed to parse REDIS_URL: %v", err)
    }
    redisClient := redis.NewClient(redisOpt)

    // Verify connection
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    if err := redisClient.Ping(ctx).Err(); err != nil {
        cancel()
        log.Fatalf("Failed to connect to Redis: %v", err)
    }
    cancel()
    log.Println("Connected to Redis")

    // Create async task infrastructure
    taskQueue := orchestration.NewRedisTaskQueue(redisClient, nil)
    taskStore := orchestration.NewRedisTaskStore(redisClient, nil)

    // ... continue with worker pool and API setup
}
```

> 📁 **Full Example**: See [examples/agent-with-async/main.go](../examples/agent-with-async/main.go) lines 114-132 for complete Redis connection and task infrastructure setup.

### Step 3: Create Worker Pool and Register Handlers

```go
// main.go (continued)
func main() {
    // ... Redis setup from Step 2 ...

    // Create worker pool configuration
    workerConfig := &orchestration.TaskWorkerConfig{
        WorkerCount:        3,                    // Number of concurrent workers
        DequeueTimeout:     30 * time.Second,     // Timeout for Redis BRPOP
        ShutdownTimeout:    60 * time.Second,     // Grace period for shutdown
        DefaultTaskTimeout: 10 * time.Minute,     // Max time per task
    }

    workerPool := orchestration.NewTaskWorkerPool(taskQueue, taskStore, workerConfig)

    // Create your agent
    agent, err := NewMyAgent(redisClient)
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }

    // Register task handlers (maps task type to handler function)
    workerPool.RegisterHandler("query", agent.HandleQuery)
    workerPool.RegisterHandler("research", agent.HandleResearch)
    workerPool.SetLogger(agent.Logger)
}
```

> 📁 **Full Example**: See [examples/agent-with-async/main.go](../examples/agent-with-async/main.go) lines 241-259 for worker pool configuration and handler registration.

### Step 4: Set Up HTTP API

```go
// main.go (continued)
func main() {
    // ... previous setup ...

    // Create Task API handler
    taskAPI := orchestration.NewTaskAPIHandler(taskQueue, taskStore, agent.Logger)

    // Register HTTP endpoints
    agent.HandleFunc("/api/v1/tasks", taskAPI.HandleSubmit)
    agent.HandleFunc("/api/v1/tasks/", func(w http.ResponseWriter, r *http.Request) {
        if strings.HasSuffix(r.URL.Path, "/cancel") && r.Method == "POST" {
            taskAPI.HandleCancel(w, r)
        } else if r.Method == "GET" {
            taskAPI.HandleGetTask(w, r)
        }
    })

    // Create and run framework
    framework, err := core.NewFramework(agent.BaseAgent,
        core.WithName("my-async-agent"),
        core.WithPort(8098),
        core.WithRedisURL(redisURL),
        core.WithDiscovery(true, "redis"),
    )
    if err != nil {
        log.Fatalf("Failed to create framework: %v", err)
    }

    // Start worker pool in background
    workerCtx, workerCancel := context.WithCancel(context.Background())
    go workerPool.Start(workerCtx)

    // Run framework (HTTP server)
    framework.Run(context.Background())
}
```

> 📁 **Full Example**: See [examples/agent-with-async/main.go](../examples/agent-with-async/main.go) lines 164-226 for API mode setup, or lines 344-486 for embedded mode with both API and workers.

### Step 5: Implement Task Handler

> 📁 **Full Example**: See [examples/agent-with-async/travel_research_agent.go](../examples/agent-with-async/travel_research_agent.go) for the complete `AsyncTravelAgent` struct (lines 51-57), `QueryResult` type (lines 67-75), and `InitializeOrchestrator` method (lines 132-174).

```go
// handlers.go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/itsneelabh/gomind/core"
)

// HandleQuery processes a natural language query task
func (a *MyAgent) HandleQuery(
    ctx context.Context,
    task *core.Task,
    reporter core.ProgressReporter,
) error {
    startTime := time.Now()

    // Parse input
    query, ok := task.Input["query"].(string)
    if !ok || query == "" {
        return fmt.Errorf("query field is required")
    }

    // Report initial progress
    reporter.Report(&core.TaskProgress{
        CurrentStep: 1,
        TotalSteps:  3,
        StepName:    "Planning",
        Percentage:  10,
        Message:     "Analyzing request...",
    })

    // Do your work here (AI orchestration, tool calls, etc.)
    result, err := a.processQuery(ctx, query, reporter)
    if err != nil {
        return err
    }

    // Set final result
    task.Result = result

    // Report completion
    reporter.Report(&core.TaskProgress{
        CurrentStep: 3,
        TotalSteps:  3,
        StepName:    "Complete",
        Percentage:  100,
        Message:     "Task completed successfully",
    })

    return nil
}
```

### Step 6: Test Your Agent

```bash
# Submit a task
curl -X POST http://localhost:8098/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{"type": "query", "input": {"query": "What is the weather in Tokyo?"}}'

# Response (HTTP 202 Accepted):
# {
#   "task_id": "abc123-def456-...",
#   "status": "queued",
#   "status_url": "/api/v1/tasks/abc123-def456-..."
# }

# Poll for status
curl http://localhost:8098/api/v1/tasks/abc123-def456-...

# Response (in progress):
# {
#   "task_id": "abc123-def456-...",
#   "status": "running",
#   "progress": {
#     "current_step": 2,
#     "total_steps": 3,
#     "step_name": "Executing tools",
#     "percentage": 60,
#     "message": "Tool 2/3 completed"
#   }
# }

# Response (completed):
# {
#   "task_id": "abc123-def456-...",
#   "status": "completed",
#   "result": {
#     "query": "What is the weather in Tokyo?",
#     "response": "The current weather in Tokyo is 15°C with partly cloudy skies...",
#     "tools_used": ["geocoding-tool", "weather-tool-v2"],
#     "execution_time": "4.2s"
#   }
# }
```

### Understanding the Request Format

The task submission request uses a deliberate structure that separates **framework concerns** from **handler concerns**:

```json
{
  "type": "query",                              // ← Framework field
  "input": {                                    // ← Handler field (opaque to framework)
    "query": "What is the weather in Tokyo?"
  },
  "timeout": "10m"                              // ← Framework field (optional)
}
```

#### Why This Structure?

| Layer | Fields | Responsibility |
|-------|--------|----------------|
| **Framework** | `type`, `timeout` | Task routing, lifecycle management, timeout enforcement |
| **Handler** | Everything inside `input` | Business logic, domain-specific validation |

This separation provides several benefits:

1. **Handler Independence**: Different task types can have completely different input schemas without changing the framework.

2. **No Field Collisions**: Your handler's `input` can contain any field names (even `type` or `timeout`) without conflicting with framework fields.

3. **Extensibility**: Add new task types by registering handlers - no framework changes needed.

4. **Clear Boundaries**: Framework code never looks inside `input`; it just passes it to the appropriate handler.

#### Examples of Different Task Types

```bash
# Simple query task
curl -X POST http://localhost:8098/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "query",
    "input": {
      "query": "What is the weather in Tokyo?"
    }
  }'

# Research task with more parameters
curl -X POST http://localhost:8098/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "research",
    "input": {
      "topic": "AI trends in 2025",
      "depth": "detailed",
      "max_sources": 10,
      "include_citations": true
    },
    "timeout": "30m"
  }'

# Travel planning task with complex nested input
curl -X POST http://localhost:8098/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "travel_research",
    "input": {
      "destination": "Paris",
      "travel_dates": {
        "start": "2025-06-15",
        "end": "2025-06-22"
      },
      "budget_usd": 5000,
      "interests": ["museums", "food", "architecture"]
    },
    "timeout": "15m"
  }'
```

Each task type (`query`, `research`, `travel_research`) has a different handler, and each handler defines its own `input` schema. The framework doesn't need to understand these schemas - it just routes based on `type` and passes `input` to the handler.

#### Parsing Input in Your Handler

In your handler code, you receive `task.Input` as `map[string]interface{}` and parse it according to YOUR schema:

```go
func (a *Agent) HandleTravelResearch(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
    // Option 1: Direct type assertions for simple inputs
    destination, ok := task.Input["destination"].(string)
    if !ok || destination == "" {
        return fmt.Errorf("destination is required")
    }

    // Option 2: Unmarshal into a struct for complex inputs
    type TravelInput struct {
        Destination  string   `json:"destination"`
        TravelDates  struct {
            Start string `json:"start"`
            End   string `json:"end"`
        } `json:"travel_dates"`
        BudgetUSD    int      `json:"budget_usd"`
        Interests    []string `json:"interests"`
    }

    var input TravelInput
    inputBytes, _ := json.Marshal(task.Input)
    if err := json.Unmarshal(inputBytes, &input); err != nil {
        return fmt.Errorf("invalid input format: %w", err)
    }

    // Now use input.Destination, input.TravelDates, etc.
    // ...
}
```

> **Design Pattern**: This "envelope + payload" pattern is common in message queue systems (SQS, RabbitMQ, Kafka). The envelope (`type`, `timeout`, `task_id`) is understood by the infrastructure; the payload (`input`) is understood only by the consumer (handler).

### Framework vs Developer Responsibilities

Understanding what the framework handles automatically versus what you must implement is crucial for building effective async agents. This section provides a comprehensive breakdown.

#### Overview: Who Does What?

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    FRAMEWORK RESPONSIBILITIES                                │
│                 (TaskWorkerPool + TaskAPIHandler)                           │
├─────────────────────────────────────────────────────────────────────────────┤
│ ✓ Task lifecycle (status transitions)                                       │
│ ✓ Queue management (enqueue, dequeue, acknowledge)                         │
│ ✓ Trace context restoration (StartLinkedSpan)                              │
│ ✓ Timeout enforcement (context cancellation)                               │
│ ✓ Panic recovery (catches handler panics)                                  │
│ ✓ Metric emission (gomind.tasks.* metrics)                                 │
│ ✓ Timestamp management (CreatedAt, StartedAt, CompletedAt)                 │
│ ✓ HTTP API (submit, poll, cancel endpoints)                                │
│ ✓ Worker pool coordination (goroutines, shutdown)                          │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ calls your handler with (ctx, task, reporter)
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                    DEVELOPER RESPONSIBILITIES                                │
│                      (Your Handler Function)                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│ ✓ Input validation (parse and validate task.Input)                         │
│ ✓ Business logic (what the task actually does)                             │
│ ✓ Progress messages (when and what to report)                              │
│ ✓ Result structure (what data to return)                                   │
│ ✓ Error messages (meaningful error returns)                                │
│ ✓ Context checking (respect ctx.Done() for cancellation)                   │
│ ✓ Custom spans (additional trace detail, if needed)                        │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Detailed Breakdown by Area

##### 1. Task Lifecycle Management

| Aspect | Framework Handles | Developer Handles |
|--------|-------------------|-------------------|
| **Status: queued** | Set when task is submitted | - |
| **Status: running** | Set when worker starts processing | - |
| **Status: completed** | Set when handler returns `nil` | Return `nil` on success |
| **Status: failed** | Set when handler returns `error` | Return meaningful error |
| **Status: cancelled** | Set when Cancel API called | Check `ctx.Done()` to exit early |
| **Timestamps** | CreatedAt, StartedAt, CompletedAt auto-set | - |

**What this means**: You never call `task.Status = completed` yourself. Just return `nil` from your handler, and the framework sets the status.

```go
// Your handler - just focus on the work
func (a *Agent) HandleQuery(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
    // Do work...

    // Return nil = framework sets status to "completed"
    return nil

    // Return error = framework sets status to "failed"
    // return fmt.Errorf("something went wrong")
}
```

> 📁 **Full Example**: See [examples/agent-with-async/handlers.go](../examples/agent-with-async/handlers.go) lines 25-244 for the complete `HandleQuery` implementation.

##### 2. Progress Reporting

| Aspect | Framework Handles | Developer Handles |
|--------|-------------------|-------------------|
| **Persistence** | Saves progress to TaskStore | - |
| **Telemetry** | Emits `task.progress` span event | - |
| **Content** | - | What to put in each field |
| **Timing** | - | When to call `reporter.Report()` |
| **Step structure** | - | Total steps, step names, percentages |

**What this means**: The framework provides the `reporter` interface and handles saving progress. You decide **what** to report and **when**.

```go
// Framework gives you reporter - you decide what to report
func (a *Agent) HandleQuery(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
    // YOU decide: report at start
    reporter.Report(&core.TaskProgress{
        CurrentStep: 1,
        TotalSteps:  3,
        StepName:    "Analyzing",      // YOU define the step name
        Percentage:  10,               // YOU calculate percentage
        Message:     "Planning...",    // YOU write the message
    })

    // Do step 1...

    // YOU decide: report after each major step
    reporter.Report(&core.TaskProgress{
        CurrentStep: 2,
        TotalSteps:  3,
        StepName:    "Executing",
        Percentage:  50,
        Message:     "Running tools...",
    })

    // Do step 2...

    return nil
}
```

> 📁 **Full Example**: See [examples/agent-with-async/handlers.go](../examples/agent-with-async/handlers.go) lines 44-51, 125-131, and 206-212 for real progress reporting with `OnStepComplete` callback integration.

##### 3. Result Handling

| Aspect | Framework Handles | Developer Handles |
|--------|-------------------|-------------------|
| **Persistence** | Saves `task.Result` to store after handler returns | - |
| **Serialization** | JSON marshals `interface{}` | - |
| **Structure** | - | Define result schema |
| **Content** | - | Populate result data |
| **Validation** | - | Ensure result is meaningful |

**What this means**: The framework stores whatever you put in `task.Result`. You define the structure.

```go
// Framework stores task.Result - you define what goes in it
func (a *Agent) HandleQuery(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
    // Do work, get response...

    // YOU define the result structure
    task.Result = &QueryResult{
        Query:         query,
        Response:      response,
        ToolsUsed:     []string{"weather-tool", "currency-tool"},
        ExecutionTime: time.Since(startTime).String(),
        Confidence:    0.95,
        // Add whatever fields make sense for your use case
    }

    return nil  // Framework persists task.Result
}

// YOUR result type - framework doesn't care about structure
type QueryResult struct {
    Query         string   `json:"query"`
    Response      string   `json:"response"`
    ToolsUsed     []string `json:"tools_used"`
    ExecutionTime string   `json:"execution_time"`
    Confidence    float64  `json:"confidence"`
}
```

> 📁 **Full Example**: See [examples/agent-with-async/travel_research_agent.go](../examples/agent-with-async/travel_research_agent.go) lines 67-83 for the `QueryResult` struct, and [examples/agent-with-async/handlers.go](../examples/agent-with-async/handlers.go) lines 216-228 for how it's populated.

##### 4. Error Handling

| Aspect | Framework Handles | Developer Handles |
|--------|-------------------|-------------------|
| **Panic recovery** | Catches panics, sets status=failed | - |
| **Timeout** | Cancels context, sets error code TASK_TIMEOUT | - |
| **Error storage** | Saves task.Error from returned error | - |
| **Error code** | Sets HANDLER_ERROR for returned errors | - |
| **Error message** | - | Return meaningful error messages |
| **Retryable errors** | - | Return errors that indicate retryability |

**What this means**: Framework catches panics and enforces timeouts. You return meaningful errors.

```go
func (a *Agent) HandleQuery(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
    // Framework will catch this panic and set status=failed with HANDLER_PANIC
    // panic("oops") // Don't panic intentionally, but framework handles it

    // YOU return meaningful errors
    result, err := a.callExternalAPI(ctx)
    if err != nil {
        // This error message is stored in task.Error.Message
        return fmt.Errorf("API call failed: %w", err)
    }

    // Framework enforces timeout - just check ctx.Done()
    select {
    case <-ctx.Done():
        return ctx.Err()  // Framework sets TASK_TIMEOUT or TASK_CANCELLED
    default:
    }

    return nil
}
```

> 📁 **Full Example**: See [examples/agent-with-async/handlers.go](../examples/agent-with-async/handlers.go) lines 34-37 for input validation, and lines 54-70 for graceful fallback when orchestrator is unavailable.

##### 5. Observability (Traces & Metrics)

| Aspect | Framework Handles | Developer Handles |
|--------|-------------------|-------------------|
| **Task spans** | Creates `task.process` span with linked context | - |
| **Lifecycle events** | Emits task.started, task.completed, task.failed | - |
| **Lifecycle metrics** | gomind.tasks.submitted, started, completed, duration | - |
| **Worker metrics** | gomind.tasks.workers.active, worker.started/stopped | - |
| **Queue metrics** | gomind.tasks.queue_depth, queue_wait_ms | - |
| **Custom spans** | - | Add spans for your business operations |
| **Custom attributes** | - | Add attributes to existing spans |
| **Business metrics** | - | Emit domain-specific metrics |

**What this means**: Framework gives you full trace context. Add your own spans for business operations.

```go
func (a *Agent) HandleQuery(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
    // ctx already has trace context restored by framework
    // Any spans you create will be children of task.process span

    // OPTIONAL: Add custom span for detailed visibility
    ctx, endSpan := telemetry.StartSpan(ctx, "business.analyze_query")
    result := a.analyzeQuery(ctx, query)
    endSpan()

    // OPTIONAL: Add custom attributes to current span
    telemetry.SetSpanAttribute(ctx, "query.length", len(query))
    telemetry.SetSpanAttribute(ctx, "query.type", "weather")

    // OPTIONAL: Emit custom business metric
    telemetry.Counter("my_agent.queries_processed", "type", "weather")

    return nil
}
```

> 📁 **Full Example**: See [examples/agent-with-async/handlers.go](../examples/agent-with-async/handlers.go) lines 119-121 and 230-233 for custom metric emission, and [examples/agent-with-async/travel_research_agent.go](../examples/agent-with-async/travel_research_agent.go) lines 117-125 for metric declarations.

#### Quick Reference: "Where Does This Happen?"

| Question | Answer |
|----------|--------|
| Where is `task.Status` set to `running`? | `TaskWorkerPool.processTask()` - before calling handler |
| Where is `task.Status` set to `completed`? | `TaskWorkerPool.processTask()` - after handler returns nil |
| Where is trace context restored? | `TaskWorkerPool.processTask()` - calls `telemetry.StartLinkedSpan()` |
| Where is timeout enforced? | `TaskWorkerPool.processTask()` - wraps handler call with `context.WithTimeout()` |
| Where are metrics emitted? | `orchestration/task_telemetry.go` - called by TaskWorkerPool |
| Where is panic caught? | `TaskWorkerPool.executeHandler()` - defer/recover block |
| Where is progress persisted? | `progressReporter.Report()` - calls `store.Update()` |
| Where is `task.Input` parsed? | **Your handler** - framework passes raw `map[string]interface{}` |
| Where is `task.Result` defined? | **Your handler** - set before returning nil |
| Where is progress content decided? | **Your handler** - you call `reporter.Report()` with content |

#### Code Locations Reference

| Component | File | Key Functions |
|-----------|------|---------------|
| Task struct & interfaces | `core/async_task.go` | `Task`, `TaskHandler`, `ProgressReporter` |
| Worker pool | `orchestration/task_worker.go` | `processTask()`, `executeHandler()`, `failTask()` |
| Telemetry helpers | `orchestration/task_telemetry.go` | `EmitTaskStarted()`, `EmitTaskCompleted()`, etc. |
| API handler | `orchestration/task_api.go` | `HandleSubmit()`, `HandleGetTask()`, `HandleCancel()` |
| Redis queue | `orchestration/redis_task_queue.go` | `Enqueue()`, `Dequeue()`, `Acknowledge()` |
| Redis store | `orchestration/redis_task_store.go` | `Create()`, `Get()`, `Update()`, `Cancel()` |
| Linked spans | `telemetry/async_span.go` | `StartLinkedSpan()` |

> 📁 **Complete Working Example**: The [examples/agent-with-async/](../examples/agent-with-async/) directory contains a production-ready implementation demonstrating all these patterns:
> - [main.go](../examples/agent-with-async/main.go) - Entry point with 3 deployment modes (api/worker/embedded)
> - [handlers.go](../examples/agent-with-async/handlers.go) - Handler implementation with AI orchestration and progress reporting
> - [travel_research_agent.go](../examples/agent-with-async/travel_research_agent.go) - Agent struct, types, and orchestrator initialization

---

## Deployment Modes

GoMind async agents support three deployment modes, controlled by the `GOMIND_MODE` environment variable.

### Mode 1: Embedded (Default, Local Development)

**When to use**: Local development, testing, simple deployments.

```
GOMIND_MODE= (unset or empty)

┌─────────────────────────────────────────────────────────────────┐
│                    Single Process                                │
├─────────────────────────────────────────────────────────────────┤
│  HTTP API Server  +  Background Workers (5 by default)          │
│                                                                  │
│  POST /api/v1/tasks → Task Queue → Worker 1                     │
│  GET /api/v1/tasks/:id           → Worker 2                     │
│                                  → Worker 3                     │
└─────────────────────────────────────────────────────────────────┘
```

**Pros**:
- Simple deployment (single container)
- Good for development and testing
- No coordination overhead

**Cons**:
- Can't scale API and workers independently
- Limited horizontal scaling

### Mode 2: API + Worker (Production)

**When to use**: Production deployments requiring independent scaling.

```
GOMIND_MODE=api (for API pods)
GOMIND_MODE=worker (for worker pods)

┌─────────────────────────────┐     ┌─────────────────────────────┐
│ API Pod (GOMIND_MODE=api)   │     │ Worker Pod (GOMIND_MODE=worker)│
├─────────────────────────────┤     ├─────────────────────────────┤
│ • POST /api/v1/tasks        │     │ • GET /health (minimal)     │
│ • GET /api/v1/tasks/:id     │     │ • BRPOP from Redis queue    │
│ • Scale: HTTP request rate  │     │ • Scale: Redis queue depth  │
└──────────────┬──────────────┘     └──────────────┬──────────────┘
               │         ┌─────────────────┐       │
               └────────>│     Redis       │<──────┘
                         │  Task Queue     │
                         │  Task Store     │
                         └─────────────────┘
```

**Kubernetes Deployment Example**:

```yaml
# API Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-agent-api
spec:
  replicas: 2  # Scale based on HTTP load
  template:
    spec:
      containers:
      - name: api
        image: my-agent:latest
        env:
        - name: GOMIND_MODE
          value: "api"
        - name: REDIS_URL
          value: "redis://redis:6379"
---
# Worker Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-agent-worker
spec:
  replicas: 5  # Scale based on queue depth
  template:
    spec:
      containers:
      - name: worker
        image: my-agent:latest
        env:
        - name: GOMIND_MODE
          value: "worker"
        - name: WORKER_COUNT
          value: "3"  # Workers per pod
        - name: REDIS_URL
          value: "redis://redis:6379"
```

**Scaling Guidelines**:

| Metric | Scale | Component |
|--------|-------|-----------|
| HTTP request rate | Horizontal | API pods |
| Redis queue depth | Horizontal | Worker pods |
| Task latency | Increase `WORKER_COUNT` | Per worker pod |
| Memory per task | Vertical | Worker pods |

---

## Writing Task Handlers

Task handlers are the core of your async agent. They process tasks from the queue.

### Handler Signature

```go
type TaskHandler func(
    ctx context.Context,           // Cancellation, deadlines, trace context
    task *core.Task,               // Task ID, input, metadata
    reporter core.ProgressReporter,// Progress reporting interface
) error
```

### The Task Struct

```go
type Task struct {
    ID           string                 // Unique task identifier (UUID)
    Type         string                 // Handler type (e.g., "query", "research")
    Status       TaskStatus             // queued, running, completed, failed, cancelled
    Input        map[string]interface{} // User-provided input
    Result       interface{}            // Set by handler on completion
    Error        *TaskError             // Error info (if failed)
    Progress     *TaskProgress          // Current progress (if running)
    Options      TaskOptions            // Execution options (timeout, etc.)
    CreatedAt    time.Time              // When task was submitted
    StartedAt    *time.Time             // When processing began (nil if queued)
    CompletedAt  *time.Time             // When task finished (nil if not complete)
    CancelledAt  *time.Time             // When cancelled (nil if not cancelled)
    TraceID      string                 // W3C trace ID for distributed tracing
    ParentSpanID string                 // Parent span ID for trace linking
}
```

### Task Error Codes

When a task fails, the `Error.Code` field contains one of these standard codes:

| Error Code | Constant | Description |
|------------|----------|-------------|
| `TASK_TIMEOUT` | `core.TaskErrorCodeTimeout` | Task exceeded its timeout duration |
| `TASK_CANCELLED` | `core.TaskErrorCodeCancelled` | Task was cancelled by request |
| `HANDLER_ERROR` | `core.TaskErrorCodeHandlerError` | Handler returned an error |
| `HANDLER_PANIC` | `core.TaskErrorCodePanic` | Handler panicked (caught by worker) |
| `INVALID_INPUT` | `core.TaskErrorCodeInvalidInput` | Task input validation failed |

### Task Helper Functions

The `core` package provides helper functions for creating tasks:

```go
// Create a new task with defaults
task := core.NewTask(taskID, "query", map[string]interface{}{
    "query": "weather in Tokyo",
})
// Status is automatically set to TaskStatusQueued
// CreatedAt is automatically set to time.Now()

// Create a task with a custom timeout
task := core.NewTaskWithTimeout(taskID, "research", input, 30*time.Minute)

// Set trace context for distributed tracing
tc := telemetry.GetTraceContext(ctx)
task.SetTraceContext(tc.TraceID, tc.SpanID)
```

### Best Practices for Handlers

#### 1. Validate Input Early

```go
func (a *Agent) HandleQuery(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
    // Validate input immediately
    query, ok := task.Input["query"].(string)
    if !ok || query == "" {
        return fmt.Errorf("query field is required")
    }

    // Type assertions for optional fields
    maxResults := 10 // default
    if mr, ok := task.Input["max_results"].(float64); ok {
        maxResults = int(mr)
    }

    // Continue with processing...
}
```

#### 2. Check Context for Cancellation

```go
func (a *Agent) HandleQuery(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
    // Check context before each significant operation
    for i, tool := range tools {
        select {
        case <-ctx.Done():
            return ctx.Err() // Task was cancelled
        default:
        }

        result, err := a.callTool(ctx, tool)
        if err != nil {
            return err
        }
    }
    return nil
}
```

#### 3. Report Progress Regularly

```go
func (a *Agent) HandleQuery(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
    // Report at meaningful milestones
    reporter.Report(&core.TaskProgress{
        CurrentStep: 1,
        TotalSteps:  4,
        StepName:    "Planning",
        Percentage:  5,
        Message:     "AI is analyzing request...",
    })

    // After each significant step
    for i, tool := range tools {
        result := callTool(ctx, tool)

        reporter.Report(&core.TaskProgress{
            CurrentStep: i + 2,
            TotalSteps:  len(tools) + 2, // +planning +synthesis
            StepName:    fmt.Sprintf("Tool: %s", tool.Name),
            Percentage:  float64(10 + i*80/len(tools)),
            Message:     fmt.Sprintf("Completed %d/%d tools", i+1, len(tools)),
        })
    }
}
```

#### 4. Set Result Before Returning

```go
func (a *Agent) HandleQuery(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
    // ... processing ...

    // Always set result before returning nil
    task.Result = &QueryResult{
        Query:         query,
        Response:      response,
        ToolsUsed:     toolNames,
        ExecutionTime: time.Since(startTime).String(),
    }

    return nil // Result is saved by worker pool
}
```

---

## Progress Reporting

The `ProgressReporter` interface enables real-time visibility into task execution.

### TaskProgress Structure

```go
type TaskProgress struct {
    CurrentStep int     // Current step number (1-based)
    TotalSteps  int     // Total number of steps
    StepName    string  // Human-readable step name
    Percentage  float64 // Completion percentage (0-100)
    Message     string  // Detailed status message
}
```

### Integration with OnStepComplete Callback

For AI-orchestrated tasks, the `OnStepComplete` callback provides automatic per-tool progress:

```go
// From examples/agent-with-async/handlers.go
config.ExecutionOptions.OnStepComplete = func(
    stepIndex, totalSteps int,
    step orchestration.RoutingStep,
    result orchestration.StepResult,
) {
    status := "completed"
    if !result.Success {
        status = "failed"
    }

    // Report per-tool progress
    percentage := 10 + int(float64(stepIndex+1)/float64(totalSteps)*85)
    reporter.Report(&core.TaskProgress{
        CurrentStep: stepIndex + 2,  // +1 for planning, +1 for 1-based
        TotalSteps:  totalSteps + 2, // +planning +synthesis
        StepName:    fmt.Sprintf("%s: %s", status, step.AgentName),
        Percentage:  float64(percentage),
        Message:     fmt.Sprintf("Tool %d/%d %s", stepIndex+1, totalSteps, status),
    })
}
```

> 📁 **Full Example**: See [examples/agent-with-async/handlers.go](../examples/agent-with-async/handlers.go) lines 88-140 for a complete `OnStepComplete` callback implementation that tracks planning, tool execution, and synthesis phases.

### Client-Side Polling

```javascript
// JavaScript polling example
async function pollTask(taskId) {
    const pollInterval = 2000; // 2 seconds

    while (true) {
        const response = await fetch(`/api/v1/tasks/${taskId}`);
        const task = await response.json();

        switch (task.status) {
            case 'queued':
                console.log('Task queued, waiting...');
                break;
            case 'running':
                console.log(`Progress: ${task.progress.percentage}% - ${task.progress.message}`);
                // Update UI progress bar
                updateProgressBar(task.progress.percentage);
                break;
            case 'completed':
                console.log('Task completed!', task.result);
                return task.result;
            case 'failed':
                throw new Error(task.error);
            case 'cancelled':
                throw new Error('Task was cancelled');
        }

        await new Promise(r => setTimeout(r, pollInterval));
    }
}
```

---

## Distributed Tracing Across Async Boundaries

One of the key challenges with async tasks is maintaining trace context when the request (API) and processing (worker) happen in different processes.

### The Challenge

```
Request Thread (API Pod)        Worker Thread (Worker Pod)
       │                               │
    trace-123                          │ ← How does worker get trace-123?
       │                               │
    Enqueue to Redis ─────────────────>│
       │                               │
    Return 202                         │ Process task
       │                               │
       ▼                               ▼
                                   New trace-456?  ← BAD: Disconnected traces!
```

### The Solution: StartLinkedSpan

GoMind provides `telemetry.StartLinkedSpan()` to create linked traces across async boundaries:

```go
// When enqueuing (API side): trace context is stored in Task fields
// The TaskAPIHandler does this automatically:
tc := telemetry.GetTraceContext(ctx)
task := &core.Task{
    ID:           uuid.New().String(),
    Type:         "query",
    Input:        input,
    TraceID:      tc.TraceID,      // Stored directly on Task
    ParentSpanID: tc.SpanID,       // Stored directly on Task
}
```

```go
// When processing (Worker side): create linked span
// The TaskWorkerPool does this automatically:
func processTask(task *core.Task) {
    // Create linked span using Task's trace fields
    ctx, endSpan := telemetry.StartLinkedSpan(
        context.Background(),
        "task.process",
        task.TraceID,        // W3C trace ID (32 hex chars)
        task.ParentSpanID,   // Parent span ID (16 hex chars)
        map[string]string{   // Attributes to attach to span
            "task.id":   task.ID,
            "task.type": task.Type,
        },
    )
    defer endSpan()

    // Process with linked context
    handleTask(ctx, task)
}
```

### What You See in Jaeger

With linked spans, Jaeger shows the relationship:

```
Trace 1 (API Request):
└── POST /api/v1/tasks (15ms)
    └── enqueue_task (5ms)
        └── [link to Trace 2]

Trace 2 (Worker Processing):
└── async-task-execution (8.5s)
    ├── [linked from Trace 1]
    ├── ai_planning (2.1s)
    ├── tool: weather-tool-v2 (600ms)
    ├── tool: currency-tool (400ms)
    └── ai_synthesis (1.8s)
```

### Enabling Async Tracing

The framework handles this automatically when you:

1. Initialize telemetry before creating the agent
2. Use the Task API handler (stores trace context)
3. Use the Task Worker Pool (creates linked spans)

```go
// main.go - Telemetry must be initialized first
func main() {
    // 1. Initialize telemetry BEFORE creating agent
    initTelemetry("my-async-agent")
    defer telemetry.Shutdown(context.Background())

    // 2. Create agent (inherits telemetry)
    agent, _ := NewMyAgent(redisClient)

    // 3. Use framework with tracing middleware
    framework, _ := core.NewFramework(agent.BaseAgent,
        core.WithMiddleware(telemetry.TracingMiddleware("my-async-agent")),
    )
}
```

---

## Monitoring and Metrics

Async agents expose metrics for monitoring queue depth, task latency, and worker health.

### Built-in Metrics

The async task system emits the following metrics via the `orchestration/task_telemetry.go` module:

#### Task Lifecycle Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `gomind.tasks.submitted` | Counter | `task_type` | Tasks submitted to the queue |
| `gomind.tasks.started` | Counter | `task_type` | Tasks that began processing |
| `gomind.tasks.completed` | Counter | `task_type`, `status`, `error_code` | Tasks that finished (any terminal state) |
| `gomind.tasks.duration_ms` | Histogram | `task_type`, `status` | Task execution duration in milliseconds |

#### Queue Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `gomind.tasks.queue_depth` | Gauge | `queue` | Current number of tasks in queue |
| `gomind.tasks.queue_wait_ms` | Histogram | `task_type` | Time tasks spend waiting in queue |

#### Worker Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `gomind.tasks.workers.active` | Gauge | - | Number of active worker goroutines |
| `gomind.tasks.worker.started` | Counter | `worker_id` | Worker start events |
| `gomind.tasks.worker.stopped` | Counter | `worker_id` | Worker stop events |
| `gomind.tasks.worker.panic` | Counter | `worker_id` | Handler panics caught by workers |

### Key Metrics to Monitor

| Metric | Type | Description | Alert Threshold |
|--------|------|-------------|-----------------|
| `gomind_tasks_completed_total{status="completed"}` | Counter | Successful task completions | - |
| `gomind_tasks_completed_total{status="failed"}` | Counter | Failed tasks | > 5% of total |
| `gomind_tasks_completed_total{status="timeout"}` | Counter | Timed out tasks | > 1% of total |
| `gomind_tasks_duration_ms` | Histogram | Task execution time | p95 > 30s |
| `gomind_tasks_queue_depth` | Gauge | Queue backlog | > 100 |
| `gomind_tasks_workers_active` | Gauge | Active workers | = 0 (no workers) |

### Prometheus Queries

```promql
# Task success rate
sum(rate(gomind_tasks_completed_total{status="completed"}[5m])) /
sum(rate(gomind_tasks_completed_total[5m]))

# Task p95 duration
histogram_quantile(0.95, sum(rate(gomind_tasks_duration_ms_bucket[5m])) by (le))

# Task throughput (tasks/second)
sum(rate(gomind_tasks_completed_total[5m]))

# Queue wait time p95
histogram_quantile(0.95, sum(rate(gomind_tasks_queue_wait_ms_bucket[5m])) by (le))

# Worker utilization (approximate)
sum(rate(gomind_tasks_started_total[5m])) / gomind_tasks_workers_active

# Failure rate by task type
sum(rate(gomind_tasks_completed_total{status="failed"}[5m])) by (task_type) /
sum(rate(gomind_tasks_completed_total[5m])) by (task_type)
```

### Grafana Dashboard

See [examples/k8-deployment/grafana.yaml](../examples/k8-deployment/grafana.yaml) for a pre-built dashboard.

---

## Configuration Reference

### Environment Variables

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `GOMIND_MODE` | Deployment mode: `api`, `worker`, or empty | `` (embedded) | `worker` |
| `REDIS_URL` | Redis connection URL | Required | `redis://localhost:6379` |
| `PORT` | HTTP server port | `8098` | `8080` |
| `WORKER_COUNT` | Workers per pod | `5` | `3` |
| `NAMESPACE` | K8s namespace for discovery | `` | `gomind-examples` |
| `DEV_MODE` | Enable development mode (verbose logging) | `false` | `true` |
| `APP_ENV` | Telemetry profile: `development`, `staging`, `production` | `development` | `production` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint | - | `http://otel-collector:4318` |
| `OPENAI_API_KEY` | OpenAI API key (enables AI orchestration) | - | `sk-...` |
| `ANTHROPIC_API_KEY` | Anthropic API key (fallback provider) | - | `sk-ant-...` |
| `GROQ_API_KEY` | Groq API key (alternative provider) | - | `gsk-...` |

> 📖 **AI Provider Configuration**: For comprehensive information on configuring AI providers, model aliases, provider chains with failover, and environment variable overrides for models, see the [AI Providers Setup Guide](./AI_PROVIDERS_SETUP_GUIDE.md). It covers:
> - All supported providers (OpenAI, Anthropic, Groq, DeepSeek, Gemini, Ollama, etc.)
> - Model aliases (`smart`, `fast`, `default`) and how to override them
> - Chain Client for production-grade failover between providers
> - Cost-optimized and privacy-first deployment scenarios

### TaskWorkerConfig

```go
type TaskWorkerConfig struct {
    WorkerCount        int           // Number of worker goroutines (default: 5)
    DequeueTimeout     time.Duration // Timeout for Redis BRPOP (default: 30s)
    ShutdownTimeout    time.Duration // Graceful shutdown timeout (default: 30s)
    DefaultTaskTimeout time.Duration // Max time per task (default: 30m)
}
```

### RedisTaskQueueConfig

```go
type RedisTaskQueueConfig struct {
    QueueKey       string         // Redis key for queue list (default: "gomind:tasks:queue")
    ProcessingKey  string         // Redis key for processing list (default: "gomind:tasks:processing")
    RetryAttempts  int            // Retry count for Redis operations (default: 3)
    RetryDelay     time.Duration  // Delay between retries (default: 100ms)
    CircuitBreaker CircuitBreaker // Optional circuit breaker for Redis operations
    Logger         Logger         // Optional logger
}
```

### RedisTaskStoreConfig

```go
type RedisTaskStoreConfig struct {
    KeyPrefix     string        // Prefix for task keys (default: "gomind:tasks")
    TTL           time.Duration // Task data expiration (default: 24h)
    RetryAttempts int           // Retry count for Redis operations (default: 3)
    RetryDelay    time.Duration // Delay between retries (default: 100ms)
    Logger        Logger        // Optional logger
}
```

### Utility Methods (Beyond the Interface)

The Redis implementations provide additional methods useful for monitoring and administration:

#### RedisTaskQueue Extra Methods

```go
// QueueLength returns the current number of tasks in the queue.
// Useful for monitoring queue depth and triggering scaling.
length, err := taskQueue.QueueLength(ctx)
if err != nil {
    log.Printf("Failed to get queue length: %v", err)
}
fmt.Printf("Queue depth: %d\n", length)
```

#### RedisTaskStore Extra Methods

```go
// ListByStatus returns all tasks with the given status.
// Useful for monitoring and admin operations.
// Note: This scans all keys with the prefix, so use sparingly in production.
runningTasks, err := taskStore.ListByStatus(ctx, core.TaskStatusRunning)
if err != nil {
    log.Printf("Failed to list running tasks: %v", err)
}
for _, task := range runningTasks {
    fmt.Printf("Running: %s (%s)\n", task.ID, task.Type)
}
```

### Implementing Custom Backends

GoMind's async task system is designed with pluggable backends. While Redis implementations are provided as defaults, you can implement `core.TaskQueue` and `core.TaskStore` interfaces for other storage systems.

#### TaskQueue Interface

```go
// TaskQueue handles async task submission and retrieval.
type TaskQueue interface {
    // Enqueue adds a task to the queue.
    // The task's Status should be TaskStatusQueued.
    Enqueue(ctx context.Context, task *Task) error

    // Dequeue retrieves the next task from the queue.
    // Blocks until a task is available or timeout expires.
    // Returns nil, nil if timeout expires with no task.
    Dequeue(ctx context.Context, timeout time.Duration) (*Task, error)

    // Acknowledge marks a task as successfully processed.
    // Called after the worker completes task processing.
    Acknowledge(ctx context.Context, taskID string) error

    // Reject returns a task to the queue for retry.
    // Called when processing fails but should be retried.
    Reject(ctx context.Context, taskID string, reason string) error
}
```

#### TaskStore Interface

```go
// TaskStore persists task state and results.
type TaskStore interface {
    // Create persists a new task.
    // Returns error if task with same ID already exists.
    Create(ctx context.Context, task *Task) error

    // Get retrieves a task by ID.
    // Returns core.ErrTaskNotFound if task doesn't exist.
    Get(ctx context.Context, taskID string) (*Task, error)

    // Update persists task changes (status, progress, result).
    // Returns core.ErrTaskNotFound if task doesn't exist.
    Update(ctx context.Context, task *Task) error

    // Delete removes a task.
    // Used for cleanup of old tasks.
    Delete(ctx context.Context, taskID string) error

    // Cancel marks a task as cancelled.
    // Returns core.ErrTaskNotFound if task doesn't exist.
    // Returns core.ErrTaskNotCancellable if task is already in a terminal state.
    Cancel(ctx context.Context, taskID string) error
}
```

#### Example: AWS SQS Implementation (for AWS-Native Deployments)

For teams running on AWS, here's an SQS-backed TaskQueue implementation:

```go
import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/sqs"
    "github.com/aws/aws-sdk-go-v2/service/sqs/types"
    "github.com/itsneelabh/gomind/core"
)

// SQSTaskQueue implements core.TaskQueue using AWS SQS.
type SQSTaskQueue struct {
    client         *sqs.Client
    queueURL       string
    logger         core.Logger
    receiptHandles map[string]string // taskID -> receiptHandle
    mu             sync.RWMutex
}

// SQSTaskQueueConfig configures the SQS task queue.
type SQSTaskQueueConfig struct {
    QueueURL string      // SQS queue URL (required)
    Region   string      // AWS region (default: from environment)
    Logger   core.Logger // Optional logger
}

// NewSQSTaskQueue creates a new SQS-backed task queue.
func NewSQSTaskQueue(ctx context.Context, cfg *SQSTaskQueueConfig) (*SQSTaskQueue, error) {
    // Build config options - only set region if explicitly provided
    var optFns []func(*config.LoadOptions) error
    if cfg.Region != "" {
        optFns = append(optFns, config.WithRegion(cfg.Region))
    }

    // Load AWS config (falls back to AWS_REGION env var if region not specified)
    awsCfg, err := config.LoadDefaultConfig(ctx, optFns...)
    if err != nil {
        return nil, fmt.Errorf("failed to load AWS config: %w", err)
    }

    return &SQSTaskQueue{
        client:         sqs.NewFromConfig(awsCfg),
        queueURL:       cfg.QueueURL,
        logger:         cfg.Logger,
        receiptHandles: make(map[string]string),
    }, nil
}

// Enqueue adds a task to the SQS queue.
func (q *SQSTaskQueue) Enqueue(ctx context.Context, task *core.Task) error {
    // Serialize task to JSON
    data, err := json.Marshal(task)
    if err != nil {
        return fmt.Errorf("failed to serialize task: %w", err)
    }

    // Send message to SQS
    _, err = q.client.SendMessage(ctx, &sqs.SendMessageInput{
        QueueUrl:    aws.String(q.queueURL),
        MessageBody: aws.String(string(data)),
        MessageAttributes: map[string]types.MessageAttributeValue{
            "TaskID": {
                DataType:    aws.String("String"),
                StringValue: aws.String(task.ID),
            },
            "TaskType": {
                DataType:    aws.String("String"),
                StringValue: aws.String(task.Type),
            },
        },
    })
    if err != nil {
        return fmt.Errorf("failed to send message to SQS: %w", err)
    }

    if q.logger != nil {
        q.logger.Info("Task enqueued to SQS", map[string]interface{}{
            "task_id":   task.ID,
            "task_type": task.Type,
            "queue_url": q.queueURL,
        })
    }
    return nil
}

// Dequeue retrieves the next task from SQS.
// Uses long polling for efficient waiting.
func (q *SQSTaskQueue) Dequeue(ctx context.Context, timeout time.Duration) (*core.Task, error) {
    // SQS long polling max is 20 seconds
    waitTimeSeconds := int32(timeout.Seconds())
    if waitTimeSeconds > 20 {
        waitTimeSeconds = 20
    }

    result, err := q.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
        QueueUrl:            aws.String(q.queueURL),
        MaxNumberOfMessages: 1,
        WaitTimeSeconds:     waitTimeSeconds,
        MessageAttributeNames: []string{"All"},
    })
    if err != nil {
        return nil, fmt.Errorf("failed to receive message from SQS: %w", err)
    }

    // No messages available
    if len(result.Messages) == 0 {
        return nil, nil
    }

    msg := result.Messages[0]

    // Deserialize task
    var task core.Task
    if err := json.Unmarshal([]byte(*msg.Body), &task); err != nil {
        // Delete malformed message to prevent poison pill
        q.deleteMessage(ctx, msg.ReceiptHandle)
        return nil, fmt.Errorf("failed to deserialize task: %w", err)
    }

    // Store receipt handle for later acknowledgment
    q.mu.Lock()
    q.receiptHandles[task.ID] = *msg.ReceiptHandle
    q.mu.Unlock()

    if q.logger != nil {
        q.logger.Info("Task dequeued from SQS", map[string]interface{}{
            "task_id":   task.ID,
            "task_type": task.Type,
        })
    }
    return &task, nil
}

// Acknowledge deletes the message from SQS (task completed successfully).
func (q *SQSTaskQueue) Acknowledge(ctx context.Context, taskID string) error {
    // Look up the receipt handle
    q.mu.RLock()
    receiptHandle, exists := q.receiptHandles[taskID]
    q.mu.RUnlock()

    if !exists {
        if q.logger != nil {
            q.logger.Warn("No receipt handle found for task", map[string]interface{}{
                "task_id": taskID,
            })
        }
        return nil // Not an error - message may have already been deleted
    }

    // Delete the message from SQS
    _, err := q.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
        QueueUrl:      aws.String(q.queueURL),
        ReceiptHandle: aws.String(receiptHandle),
    })
    if err != nil {
        return fmt.Errorf("failed to delete message from SQS: %w", err)
    }

    // Clean up the receipt handle
    q.mu.Lock()
    delete(q.receiptHandles, taskID)
    q.mu.Unlock()

    if q.logger != nil {
        q.logger.Debug("Task acknowledged, message deleted from SQS", map[string]interface{}{
            "task_id": taskID,
        })
    }
    return nil
}

// Reject returns the message to the queue for retry.
// Sets visibility timeout to 0 for immediate retry.
func (q *SQSTaskQueue) Reject(ctx context.Context, taskID string, reason string) error {
    // Look up the receipt handle
    q.mu.RLock()
    receiptHandle, exists := q.receiptHandles[taskID]
    q.mu.RUnlock()

    if !exists {
        if q.logger != nil {
            q.logger.Warn("No receipt handle found for rejected task", map[string]interface{}{
                "task_id": taskID,
                "reason":  reason,
            })
        }
        return nil
    }

    // Set visibility timeout to 0 for immediate retry
    _, err := q.client.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
        QueueUrl:          aws.String(q.queueURL),
        ReceiptHandle:     aws.String(receiptHandle),
        VisibilityTimeout: 0, // Make message immediately visible for retry
    })
    if err != nil {
        // Log but don't fail - message will become visible after timeout anyway
        if q.logger != nil {
            q.logger.Warn("Failed to change message visibility", map[string]interface{}{
                "task_id": taskID,
                "error":   err.Error(),
            })
        }
    }

    // Clean up the receipt handle
    q.mu.Lock()
    delete(q.receiptHandles, taskID)
    q.mu.Unlock()

    if q.logger != nil {
        q.logger.Warn("Task rejected, message returned to queue", map[string]interface{}{
            "task_id": taskID,
            "reason":  reason,
        })
    }
    return nil
}

// deleteMessage removes a message from the queue.
func (q *SQSTaskQueue) deleteMessage(ctx context.Context, receiptHandle *string) error {
    _, err := q.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
        QueueUrl:      aws.String(q.queueURL),
        ReceiptHandle: receiptHandle,
    })
    return err
}
```

**SQS Queue Configuration Tips:**

```bash
# Create an SQS queue with recommended settings for async tasks
aws sqs create-queue \
  --queue-name gomind-tasks \
  --attributes '{
    "VisibilityTimeout": "300",
    "MessageRetentionPeriod": "86400",
    "ReceiveMessageWaitTimeSeconds": "20"
  }'

# For dead-letter queue (failed tasks after max retries)
aws sqs create-queue --queue-name gomind-tasks-dlq

# Configure redrive policy
aws sqs set-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/123456789/gomind-tasks \
  --attributes '{
    "RedrivePolicy": "{\"deadLetterTargetArn\":\"arn:aws:sqs:us-east-1:123456789:gomind-tasks-dlq\",\"maxReceiveCount\":\"3\"}"
  }'
```

**Using SQS with Redis Store (Hybrid Setup):**

You can mix backends based on your infrastructure. For example, use SQS for the queue (leveraging AWS managed infrastructure) while keeping Redis for task state storage:

```go
// Create hybrid task infrastructure
ctx := context.Background()

// SQS for queue (AWS-managed, auto-scaling)
sqsQueue, err := NewSQSTaskQueue(ctx, &SQSTaskQueueConfig{
    QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789/gomind-tasks",
    Region:   "us-east-1",
    Logger:   logger,
})
if err != nil {
    log.Fatalf("Failed to create SQS queue: %v", err)
}

// Redis for store (existing infrastructure)
redisClient := redis.NewClient(&redis.Options{
    Addr: "redis:6379",
})
redisStore := orchestration.NewRedisTaskStore(redisClient, nil)

// Create worker pool with hybrid backends
workerPool := orchestration.NewTaskWorkerPool(sqsQueue, redisStore, workerConfig)
```

#### Example: AWS DynamoDB Implementation (for AWS-Native Deployments)

For a fully AWS-native setup, here's a DynamoDB-backed TaskStore implementation to pair with the SQS TaskQueue above:

```go
import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    "github.com/itsneelabh/gomind/core"
)

// DynamoDBTaskStore implements core.TaskStore using AWS DynamoDB.
type DynamoDBTaskStore struct {
    client    *dynamodb.Client
    tableName string
    logger    core.Logger
}

// DynamoDBTaskStoreConfig configures the DynamoDB task store.
type DynamoDBTaskStoreConfig struct {
    TableName string      // DynamoDB table name (required)
    Region    string      // AWS region (default: from environment)
    Logger    core.Logger // Optional logger
}

// NewDynamoDBTaskStore creates a new DynamoDB-backed task store.
func NewDynamoDBTaskStore(ctx context.Context, cfg *DynamoDBTaskStoreConfig) (*DynamoDBTaskStore, error) {
    var optFns []func(*config.LoadOptions) error
    if cfg.Region != "" {
        optFns = append(optFns, config.WithRegion(cfg.Region))
    }

    awsCfg, err := config.LoadDefaultConfig(ctx, optFns...)
    if err != nil {
        return nil, fmt.Errorf("failed to load AWS config: %w", err)
    }

    return &DynamoDBTaskStore{
        client:    dynamodb.NewFromConfig(awsCfg),
        tableName: cfg.TableName,
        logger:    cfg.Logger,
    }, nil
}

// Create persists a new task to DynamoDB.
func (s *DynamoDBTaskStore) Create(ctx context.Context, task *core.Task) error {
    item, err := attributevalue.MarshalMap(task)
    if err != nil {
        return fmt.Errorf("failed to marshal task: %w", err)
    }

    // Use condition to prevent overwriting existing task
    _, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
        TableName:           aws.String(s.tableName),
        Item:                item,
        ConditionExpression: aws.String("attribute_not_exists(ID)"),
    })
    if err != nil {
        // Check if it's a condition check failure (task already exists)
        var ccf *types.ConditionalCheckFailedException
        if ok := errors.As(err, &ccf); ok {
            return fmt.Errorf("task already exists: %s", task.ID)
        }
        return fmt.Errorf("failed to create task: %w", err)
    }

    if s.logger != nil {
        s.logger.Info("Task created in DynamoDB", map[string]interface{}{
            "task_id":   task.ID,
            "task_type": task.Type,
        })
    }
    return nil
}

// Get retrieves a task by ID from DynamoDB.
func (s *DynamoDBTaskStore) Get(ctx context.Context, taskID string) (*core.Task, error) {
    result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
        TableName: aws.String(s.tableName),
        Key: map[string]types.AttributeValue{
            "ID": &types.AttributeValueMemberS{Value: taskID},
        },
    })
    if err != nil {
        return nil, fmt.Errorf("failed to get task: %w", err)
    }

    if result.Item == nil {
        return nil, core.ErrTaskNotFound
    }

    var task core.Task
    if err := attributevalue.UnmarshalMap(result.Item, &task); err != nil {
        return nil, fmt.Errorf("failed to unmarshal task: %w", err)
    }

    return &task, nil
}

// Update persists task changes to DynamoDB.
func (s *DynamoDBTaskStore) Update(ctx context.Context, task *core.Task) error {
    item, err := attributevalue.MarshalMap(task)
    if err != nil {
        return fmt.Errorf("failed to marshal task: %w", err)
    }

    // Use condition to ensure task exists
    _, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
        TableName:           aws.String(s.tableName),
        Item:                item,
        ConditionExpression: aws.String("attribute_exists(ID)"),
    })
    if err != nil {
        var ccf *types.ConditionalCheckFailedException
        if ok := errors.As(err, &ccf); ok {
            return core.ErrTaskNotFound
        }
        return fmt.Errorf("failed to update task: %w", err)
    }

    if s.logger != nil {
        s.logger.Debug("Task updated in DynamoDB", map[string]interface{}{
            "task_id": task.ID,
            "status":  task.Status,
        })
    }
    return nil
}

// Delete removes a task from DynamoDB.
func (s *DynamoDBTaskStore) Delete(ctx context.Context, taskID string) error {
    _, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
        TableName: aws.String(s.tableName),
        Key: map[string]types.AttributeValue{
            "ID": &types.AttributeValueMemberS{Value: taskID},
        },
    })
    if err != nil {
        return fmt.Errorf("failed to delete task: %w", err)
    }

    if s.logger != nil {
        s.logger.Info("Task deleted from DynamoDB", map[string]interface{}{
            "task_id": taskID,
        })
    }
    return nil
}

// Cancel marks a task as cancelled in DynamoDB.
func (s *DynamoDBTaskStore) Cancel(ctx context.Context, taskID string) error {
    // First, get the current task to check status
    task, err := s.Get(ctx, taskID)
    if err != nil {
        return err
    }

    if task.Status.IsTerminal() {
        return core.ErrTaskNotCancellable
    }

    // Update with cancellation
    now := time.Now()
    _, err = s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
        TableName: aws.String(s.tableName),
        Key: map[string]types.AttributeValue{
            "ID": &types.AttributeValueMemberS{Value: taskID},
        },
        UpdateExpression: aws.String("SET #status = :status, CancelledAt = :cancelledAt, #error = :error"),
        ExpressionAttributeNames: map[string]string{
            "#status": "Status",
            "#error":  "Error",
        },
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":status":      &types.AttributeValueMemberS{Value: string(core.TaskStatusCancelled)},
            ":cancelledAt": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
            ":error": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
                "Code":    &types.AttributeValueMemberS{Value: string(core.TaskErrorCodeCancelled)},
                "Message": &types.AttributeValueMemberS{Value: "Task was cancelled by request"},
            }},
        },
        // Only update if task exists and is not already terminal
        ConditionExpression: aws.String("attribute_exists(ID)"),
    })
    if err != nil {
        var ccf *types.ConditionalCheckFailedException
        if ok := errors.As(err, &ccf); ok {
            return core.ErrTaskNotFound
        }
        return fmt.Errorf("failed to cancel task: %w", err)
    }

    if s.logger != nil {
        s.logger.Info("Task cancelled in DynamoDB", map[string]interface{}{
            "task_id": taskID,
        })
    }
    return nil
}
```

**DynamoDB Table Setup:**

```bash
# Create a DynamoDB table for tasks
aws dynamodb create-table \
  --table-name gomind-tasks \
  --attribute-definitions AttributeName=ID,AttributeType=S \
  --key-schema AttributeName=ID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

# Optional: Add TTL for automatic cleanup of old tasks
aws dynamodb update-time-to-live \
  --table-name gomind-tasks \
  --time-to-live-specification "Enabled=true,AttributeName=TTL"
```

**Using SQS + DynamoDB (Complete AWS-Native Setup):**

```go
// Create fully AWS-native task infrastructure
ctx := context.Background()

// SQS for queue
sqsQueue, err := NewSQSTaskQueue(ctx, &SQSTaskQueueConfig{
    QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789/gomind-tasks",
    Region:   "us-east-1",
    Logger:   logger,
})
if err != nil {
    log.Fatalf("Failed to create SQS queue: %v", err)
}

// DynamoDB for store
dynamoStore, err := NewDynamoDBTaskStore(ctx, &DynamoDBTaskStoreConfig{
    TableName: "gomind-tasks",
    Region:    "us-east-1",
    Logger:    logger,
})
if err != nil {
    log.Fatalf("Failed to create DynamoDB store: %v", err)
}

// Create worker pool with AWS backends
workerPool := orchestration.NewTaskWorkerPool(sqsQueue, dynamoStore, workerConfig)
```

#### Using Custom Implementations

```go
// Use your custom implementations instead of the defaults
taskQueue := NewMyCustomTaskQueue()  // Your custom queue implementation
taskStore := NewMyCustomTaskStore()  // Your custom store implementation

// Create worker pool with custom backends
workerPool := orchestration.NewTaskWorkerPool(taskQueue, taskStore, workerConfig)

// Create API handler with custom backends
apiHandler := orchestration.NewTaskAPIHandler(taskQueue, taskStore, logger)
```

#### When to Use Each Backend

| Use Case | Recommended Backend |
|----------|---------------------|
| **Production** | Redis (default) - battle-tested, horizontally scalable |
| **Unit tests** | Mock the interfaces or use Redis with testcontainers |
| **Integration tests** | Redis (testcontainers) |
| **AWS-heavy infrastructure** | SQS queue + Redis store (hybrid example above) |
| **Fully AWS-native** | SQS queue + DynamoDB store (examples above) |
| **PostgreSQL shop** | Custom PostgreSQL implementation for both interfaces |

---

## Best Practices

### DO

1. **Set appropriate timeouts**:
   ```go
   workerConfig := &orchestration.TaskWorkerConfig{
       DefaultTaskTimeout: 10 * time.Minute, // Match your longest expected task
   }
   ```

2. **Report progress frequently**:
   ```go
   // At minimum: start, each major step, completion
   reporter.Report(&core.TaskProgress{...})
   ```

3. **Handle cancellation**:
   ```go
   select {
   case <-ctx.Done():
       return ctx.Err()
   default:
       // Continue processing
   }
   ```

4. **Use structured results**:
   ```go
   task.Result = &QueryResult{
       Query:         query,
       Response:      response,
       ExecutionTime: duration.String(),
       Metadata:      map[string]interface{}{...},
   }
   ```

5. **Initialize telemetry early**:
   ```go
   func main() {
       initTelemetry("my-agent") // BEFORE creating agent
       defer telemetry.Shutdown(context.Background())
       // ...
   }
   ```

### DON'T

1. **Don't block forever**:
   ```go
   // BAD
   <-someChannel // Could block forever

   // GOOD
   select {
   case result := <-someChannel:
       // handle
   case <-ctx.Done():
       return ctx.Err()
   }
   ```

2. **Don't ignore errors**:
   ```go
   // BAD
   result, _ := a.callTool(ctx, tool)

   // GOOD
   result, err := a.callTool(ctx, tool)
   if err != nil {
       return fmt.Errorf("tool %s failed: %w", tool.Name, err)
   }
   ```

3. **Don't forget to set result**:
   ```go
   // BAD - result is nil on success
   return nil

   // GOOD
   task.Result = &MyResult{...}
   return nil
   ```

4. **Don't log secrets**:
   ```go
   // BAD
   a.Logger.Info("Calling AI", map[string]interface{}{
       "api_key": os.Getenv("OPENAI_API_KEY"), // NEVER!
   })

   // GOOD
   a.Logger.Info("Calling AI", map[string]interface{}{
       "provider": "openai",
       "model":    "gpt-4o-mini",
   })
   ```

---

## Troubleshooting

### Problem: Tasks Stuck in "queued" Status

**Symptoms**: Tasks submitted but never transition to "running".

**Causes**:
1. **Workers not running**: Check if worker pods are up
2. **Redis connection failed**: Workers can't connect to queue
3. **Wrong queue key**: API and workers using different queue keys

**Diagnosis**:
```bash
# Check worker pods
kubectl get pods -l app=my-agent-worker -n gomind-examples

# Check worker logs
kubectl logs -l app=my-agent-worker -n gomind-examples --tail=50

# Check Redis queue depth
kubectl exec -n gomind-examples deploy/redis -- redis-cli LLEN gomind:tasks:queue
```

**Solution**:
```bash
# Verify REDIS_URL matches between API and workers
kubectl get deployment my-agent-api -o jsonpath='{.spec.template.spec.containers[0].env}'
kubectl get deployment my-agent-worker -o jsonpath='{.spec.template.spec.containers[0].env}'
```

### Problem: Tasks Fail Immediately

**Symptoms**: Tasks transition to "failed" within seconds.

**Causes**:
1. **Handler not registered**: No handler for task type
2. **Input validation failed**: Invalid input format
3. **Missing dependencies**: AI client not configured

**Diagnosis**:
```bash
# Check task error
curl http://localhost:8098/api/v1/tasks/{task_id} | jq '.error'

# Check worker logs for handler registration
kubectl logs -l app=my-agent-worker | grep "Registered handler"
```

**Solution**:
```go
// Ensure handler is registered for the task type
workerPool.RegisterHandler("query", agent.HandleQuery)  // Must match task type

// Verify task type in submit request
// {"type": "query", ...}  // Must match registered handler
```

### Problem: Progress Not Updating

**Symptoms**: Task shows 0% progress, then jumps to 100%.

**Causes**:
1. **Not calling reporter.Report()**: Progress not being sent
2. **Reporter not connected**: Store not saving updates
3. **Polling too slow**: Updates happening between polls

**Solution**:
```go
// Ensure you're reporting progress at each step
func (a *Agent) HandleQuery(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
    // Report BEFORE starting work
    reporter.Report(&core.TaskProgress{Percentage: 5, Message: "Starting..."})

    // Report DURING work
    for i, step := range steps {
        reporter.Report(&core.TaskProgress{
            Percentage: float64(10 + i*80/len(steps)),
            Message:    fmt.Sprintf("Step %d/%d", i+1, len(steps)),
        })
        doStep(step)
    }

    // Report AFTER completion
    reporter.Report(&core.TaskProgress{Percentage: 100, Message: "Done"})
    return nil
}
```

### Problem: Traces Not Linked

**Symptoms**: API and worker traces appear as separate, unconnected traces.

**Causes**:
1. **Telemetry not initialized**: Worker doesn't have telemetry
2. **Metadata not passed**: Trace context not stored in task

**Solution**:
```go
// Initialize telemetry in BOTH API and worker modes
func main() {
    mode := os.Getenv("GOMIND_MODE")
    serviceName := "my-agent"
    if mode != "" {
        serviceName = fmt.Sprintf("my-agent-%s", mode)
    }
    initTelemetry(serviceName)  // Initialize in all modes
}
```

### Problem: Workers Using Too Much Memory

**Symptoms**: Worker pods OOMKilled.

**Causes**:
1. **Too many concurrent workers**: Each worker holds task in memory
2. **Large AI responses**: Results consuming memory
3. **Memory leaks**: Resources not being freed

**Solution**:
```yaml
# Reduce worker count or increase memory
containers:
- name: worker
  env:
  - name: WORKER_COUNT
    value: "2"  # Reduce from 3
  resources:
    limits:
      memory: "512Mi"  # Increase from 256Mi
```

---

## Related Documentation

- [Distributed Tracing Guide](./DISTRIBUTED_TRACING_GUIDE.md) - Complete tracing setup
- [AI-Powered Payload Generation Guide](./TOOL_SCHEMA_DISCOVERY_GUIDE.md) - Tool schema discovery
- [Orchestration Architecture](../orchestration/notes/ASYNC_TASK_DESIGN.md) - Technical architecture details
- [Example: agent-with-async](../examples/agent-with-async/) - Complete working example

---

Happy building async agents!
