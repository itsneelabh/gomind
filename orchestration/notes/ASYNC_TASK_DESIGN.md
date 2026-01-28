# Async Task Architecture for Long-Running Operations

> **Status**: v1 Implemented
> **Author**: GoMind Team
> **Created**: 2025-01-15
> **Last Updated**: 2026-01-04

## Table of Contents

- [Executive Summary](#executive-summary)
- [Architecture Compliance](#architecture-compliance)
- [Problem Statement](#problem-statement)
- [Responsibility Matrix](#responsibility-matrix)
- [Current State Analysis](#current-state-analysis)
- [Industry Best Practices](#industry-best-practices)
- [Proposed Architecture](#proposed-architecture)
- [API Design](#api-design)
- [Core Interfaces (v1)](#core-interfaces-v1)
- [Pluggable Infrastructure Design](#pluggable-infrastructure-design)
- [Integration with SmartExecutor](#integration-with-smartexecutor)
- [Telemetry Integration](#telemetry-integration-v1---built-in)
- [Worker Deployment Architecture](#worker-deployment-architecture)
- [Implementation Roadmap](#implementation-roadmap)
  - [Detailed Implementation Specifications](#detailed-implementation-specifications)
- [Future Enhancements (v2+)](#future-enhancements-v2)
- [v2 Breaking Changes](#v2-breaking-changes)
- [Developer Impact](#developer-impact)
- [References](#references)

---

## Executive Summary

GoMind's current HTTP-based architecture has **timeout constraints** that make it unsuitable for AI agent tasks that can take minutes or hours to complete. The HTTP server's WriteTimeout (30s) and the orchestration HTTP client timeout (60s, configurable via `GOMIND_ORCHESTRATION_TIMEOUT`) limit how long requests can run. This document proposes an **asynchronous task architecture** that enables long-running operations while maintaining the framework's Kubernetes-native, production-ready design principles.

### Phased Approach

| Phase | Scope | Goal |
|-------|-------|------|
| **v1 (MVP)** | ~2,400 lines | Solve the timeout problem |
| **v2 (Future)** | ~350 lines | Add enhancements based on user feedback |

### v1 Key Features (MVP)

1. **HTTP 202 + Polling** - Submit task, get ID, poll for status/result
2. **Background worker execution** - Decouple HTTP from processing
3. **Task status tracking** - queued → running → completed/failed/cancelled
4. **Task cancellation** - Cancel queued or running tasks
5. **Redis backend** - Production-ready, leverages existing infrastructure
6. **Progress reporting interface** - Enable (but don't require) progress updates
7. **Distributed tracing** - Trace context propagation across async boundaries
8. **Prometheus metrics** - Queue depth, task duration, completion counts

### v2 Future Enhancements (Based on Feedback)

- SSE streaming for real-time progress
- Checkpointing for crash recovery
- Retry with exponential backoff
- SmartExecutor auto-progress (automatic progress reporting per execution step)

> **Note**: v2 features will require changes to v1 structs. See [v2 Breaking Changes](#v2-breaking-changes) section.

---

## Architecture Compliance

This design follows the principles defined in:
- [FRAMEWORK_DESIGN_PRINCIPLES.md](../FRAMEWORK_DESIGN_PRINCIPLES.md)
- [core/CORE_DESIGN_PRINCIPLES.md](../core/CORE_DESIGN_PRINCIPLES.md)
- [orchestration/ARCHITECTURE.md](./ARCHITECTURE.md)

### Module Dependencies

```
┌─────────────────────────────────────────────────────────────┐
│                     Async Task System                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  core/async_task.go (Interfaces)                            │
│  ├── TaskQueue interface                                     │
│  ├── TaskStore interface                                     │
│  ├── TaskWorker interface                                    │
│  └── Task struct (with TraceID, ParentSpanID)               │
│                          ▲                                   │
│                          │ implements                        │
│                          │                                   │
│  orchestration/ (Implementations)                           │
│  ├── redis_task_queue.go    → imports core                  │
│  ├── redis_task_store.go    → imports core                  │
│  ├── task_worker.go         → imports core + telemetry      │
│  ├── task_api.go            → imports core                  │
│  └── task_telemetry.go      → imports telemetry             │
│                                                              │
│  ❌ NO imports of: ai, resilience, ui                        │
│  ✅ Allowed imports: core, telemetry                         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Interface-First Design

Per [CORE_DESIGN_PRINCIPLES.md](../core/CORE_DESIGN_PRINCIPLES.md):
> Every external dependency and framework concept must be defined as an interface in core.

All async task interfaces (`TaskQueue`, `TaskStore`, `TaskWorker`, `ProgressReporter`) are defined in `core/async_task.go`, with implementations in the orchestration module.

### Dependency Injection Pattern

Per [orchestration/ARCHITECTURE.md](./ARCHITECTURE.md#dependency-injection-pattern):
> The orchestration module uses interface-based dependencies for optional modules.

```go
// TaskWorker accepts optional dependencies via injection
type TaskWorkerConfig struct {
    Telemetry core.Telemetry   `json:"-"` // Optional, injected
    Logger    core.Logger      `json:"-"` // Optional, injected
}

// Application wires dependencies
worker := orchestration.NewTaskWorker(queue, store, &TaskWorkerConfig{
    Telemetry: telemetry.GetTelemetryProvider(),
    Logger:    logger,
})
```

### Resilience Pattern (Three-Layer)

Per [orchestration/ARCHITECTURE.md](./ARCHITECTURE.md#three-layer-resilience-architecture), Redis operations should follow the three-layer pattern:

| Layer | Description | Implementation |
|-------|-------------|----------------|
| **Layer 1** | Built-in simple resilience | Retries with backoff in `RedisTaskQueue` |
| **Layer 2** | Circuit breaker (optional) | Injected via `TaskQueueConfig.CircuitBreaker` |
| **Layer 3** | Fallback | Graceful degradation when Redis unavailable |

### Telemetry Module Prerequisite

This design requires a new function in the `telemetry` module:
- `telemetry.StartLinkedSpan()` - For creating spans with linked parent trace context

This is documented in the [Telemetry Integration](#telemetry-integration-v1---built-in) section.

### Compliance Status Report

| Principle | Status | Notes |
|-----------|--------|-------|
| Module dependencies (core + telemetry only) | ✅ | No imports of `ai`, `resilience`, `ui` |
| Interfaces in core | ✅ | `TaskQueue`, `TaskStore`, `TaskWorker`, `ProgressReporter` in `core/async_task.go` |
| Implementations in orchestration | ✅ | `redis_task_queue.go`, `redis_task_store.go`, `task_worker.go`, `task_api.go` |
| Dependency injection for optional features | ✅ | Telemetry, Logger, CircuitBreaker injected via config |
| Three-layer resilience | ✅ | Built-in retries → Optional circuit breaker → Fallback |
| Intelligent configuration | ✅ | `GOMIND_MODE` env var with smart defaults |
| Production-first (single binary) | ✅ | Same Docker image, different `GOMIND_MODE` value |
| Distributed tracing continuity | ✅ | `TraceID`, `ParentSpanID` in Task struct |

### Prerequisites for Implementation

Before implementing, the following must be in place:

| Prerequisite | Module | Status | Specification |
|--------------|--------|--------|---------------|
| `StartLinkedSpan()` function | `telemetry` | ✅ IMPLEMENTED | [telemetry/async_span.go](../telemetry/async_span.go) |
| `CircuitBreaker` interface | `core` | ✅ EXISTS - Already defined | [core/interfaces.go](../core/interfaces.go) |
| `Logger` interface | `core` | ✅ EXISTS - Already defined | [core/interfaces.go:10-23](../core/interfaces.go) |
| `Telemetry` interface | `core` | ✅ EXISTS - Already defined | [core/interfaces.go:49-53](../core/interfaces.go) |

---

## Problem Statement

### Why Long-Running Tasks Are Critical for AI Agents

AI agent workflows can take **minutes to hours** due to:

| Factor | Typical Duration | Example |
|--------|------------------|---------|
| LLM Latency | 5-60s per call | Complex reasoning chains with multiple LLM invocations |
| External APIs | 10-120s | Rate-limited APIs, slow third-party services |
| Data Processing | 1-30 min | Large document analysis, embeddings generation |
| Human-in-the-Loop | Minutes to hours | Waiting for approvals or input |
| Multi-Agent Coordination | Variable | Sequential agent handoffs, consensus building |

### Example Scenario

A research agent workflow:
```
Step 1: Search 5 sources        → 30s each = 2.5 min
Step 2: Analyze results         → 60s
Step 3: Synthesize with LLM     → 60s
Step 4: Generate report         → 30s
────────────────────────────────────────────
Total: ~4+ minutes
```

**Current system has timeout constraints** - this workflow may fail due to HTTP client timeout (60s per step) or server WriteTimeout (30s).

### The HTTP Timeout Problem

```
┌─────────────────────────────────────────────────────────────────┐
│                    Current Synchronous Flow                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Client ──HTTP POST──> Server ──Process──> Server ──Response──> │
│                                                                  │
│  [────────────── Connection held open ──────────────]           │
│                                                                  │
│  Problems:                                                       │
│  • HTTP WriteTimeout: 30s max                                   │
│  • Load balancer timeout: 60s typical                           │
│  • Browser timeout: 2-5 min                                     │
│  • Connection drops on network issues                           │
│  • No progress visibility                                       │
│  • Server resources tied up                                     │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Responsibility Matrix

This section defines clear boundaries between what the **framework provides** vs what **developers implement**.

### Framework Provides (v1 MVP)

| Component | Description | Why Framework? |
|-----------|-------------|----------------|
| **Task submission endpoint** | `POST /api/v1/tasks` → returns task ID | Core solution to timeout |
| **Task status endpoint** | `GET /api/v1/tasks/:id/status` | Required for polling |
| **Task result endpoint** | `GET /api/v1/tasks/:id/result` | Required to get output |
| **Task cancel endpoint** | `POST /api/v1/tasks/:id/cancel` | Abort running tasks |
| **TaskQueue interface** | `Enqueue()`, `Dequeue()`, `Ack()` | Standardizes queue abstraction |
| **TaskStore interface** | `Create()`, `Get()`, `Update()`, `Cancel()` | Standardizes state persistence |
| **RedisTaskQueue** | Redis-backed queue implementation | Default production backend |
| **RedisTaskStore** | Redis-backed store implementation | Default production backend |
| **TaskWorker** | Background worker that processes tasks | Executes queued tasks |
| **ProgressReporter interface** | `Report(progress)` | Enables progress updates |

### Framework Provides (v2 Future)

| Component | Description | Add When? |
|-----------|-------------|-----------|
| SSE streaming | Real-time progress to browsers | User requests real-time updates |
| Checkpointing | Save/restore workflow state | Users have workflows > 10 min |
| Retry with backoff | Auto-retry failed tasks | Users report transient failures |
| Task metrics | Prometheus metrics for tasks | Users need observability |

### Developer Provides (Application Code)

| Component | Description | Why Developer? |
|-----------|-------------|----------------|
| **Task result schema** | What data your task returns | Application-specific |
| **Progress messages** | "Step 3: Analyzing data..." | Business logic |
| **Custom backends** | SQS, RabbitMQ, NATS, Kafka | Niche; implement interface |
| **Webhook receivers** | Endpoints to receive callbacks | Client infrastructure |
| **Human-in-the-loop logic** | Approval workflows | Complex business logic |
| **Task scheduling** | Cron-like task submission | Use K8s CronJob |
| **Authorization/tenancy** | Who can submit/view tasks | Application security |
| **Result retention policy** | How long to keep results | Compliance/storage needs |
| **Task dependencies** | Task A depends on Task B | Workflow engine territory |

### Visual Summary

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        RESPONSIBILITY BOUNDARIES                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │ FRAMEWORK v1 (MVP) - ~2,400 lines                                       ││
│  │                                                                         ││
│  │  • HTTP 202 + Task ID         • Task status tracking                   ││
│  │  • Background worker          • TaskQueue/TaskStore interfaces         ││
│  │  • Redis backend              • ProgressReporter interface             ││
│  │  • Task cancellation          •                                        ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │ FRAMEWORK v2 (Future) - ~350 lines - Based on user feedback            ││
│  │                                                                         ││
│  │  • SSE streaming              • Checkpointing                          ││
│  │  • Retry with backoff         • Task metrics                           ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │ DEVELOPER PROVIDES                                                      ││
│  │                                                                         ││
│  │  • Task result schema         • Progress message content               ││
│  │  • Custom backends (SQS...)   • Webhook receivers                      ││
│  │  • Human-in-the-loop logic    • Authorization/tenancy                  ││
│  │  • Task scheduling            • Result retention policies              ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Current State Analysis

### HTTP Architecture in GoMind

Based on analysis of the codebase (`core/config.go`, `orchestration/executor.go`, `orchestration/interfaces.go`):

#### Timeout Configuration

```go
// HTTP Server Timeouts (core/config.go) - ENFORCED
HTTPConfig{
    ReadTimeout:       30 * time.Second,
    ReadHeaderTimeout: 10 * time.Second,
    WriteTimeout:      30 * time.Second,  // ← Enforced by net/http
    IdleTimeout:       120 * time.Second,
    ShutdownTimeout:   10 * time.Second,
}

// Orchestration HTTP Client (orchestration/executor.go) - ENFORCED
// Set via GOMIND_ORCHESTRATION_TIMEOUT env var, default 60s
tracedClient.Timeout = 60 * time.Second  // ← Actual limit for tool calls

// ExecutionOptions (orchestration/interfaces.go) - CONFIGURED BUT NOT ENFORCED
ExecutionOptions{
    StepTimeout:      30 * time.Second,   // ⚠️ Defined but NOT used
    TotalTimeout:     2 * time.Minute,    // ⚠️ Defined but NOT used
    RetryAttempts:    2,                  // ⚠️ Defined but NOT used
    RetryDelay:       2 * time.Second,    // ⚠️ Defined but NOT used
    RecoveryTimeout:  30 * time.Second,   // ⚠️ Defined but NOT used
}
```

> **Important**: `StepTimeout` and `TotalTimeout` are defined in `ExecutionOptions` but are **not enforced** in `SmartExecutor.Execute()`. The actual timeout for tool calls comes from the HTTP client's global timeout (60s default).

#### Current Request Flow

```
Client HTTP Request
    │
    ▼
┌─────────────────────────────┐
│  AIOrchestrator             │
│  ProcessRequest()           │
└─────────────┬───────────────┘
              │
              ▼
┌─────────────────────────────┐
│  Router                     │
│  Creates RoutingPlan        │
└─────────────┬───────────────┘
              │
              ▼
┌─────────────────────────────┐
│  SmartExecutor.Execute()    │
│  • Semaphore: 5 concurrent  │
│  • HTTP client: 60s timeout │
│  • No total timeout         │
├─────────────────────────────┤
│  Step 1 ──HTTP POST──> Tool │
│  Step 2 ──HTTP POST──> Tool │
│  Step N ──HTTP POST──> Tool │
└─────────────┬───────────────┘
              │
              ▼
┌─────────────────────────────┐
│  Response Aggregation       │
└─────────────┬───────────────┘
              │
              ▼
HTTP Response (must complete within WriteTimeout)
```

#### Critical Bottlenecks

| Bottleneck | Current State | Impact |
|------------|---------------|--------|
| HTTP WriteTimeout | 30s (enforced) | Server response must complete within 30s |
| HTTP Client Timeout | 60s (enforced) | Any single tool call max 60s |
| Total Execution Timeout | None (not enforced) | No limit on total workflow duration |
| StepTimeout/TotalTimeout | Configured but unused | False sense of control |
| Synchronous Pattern | Blocking | Client connection held open |
| No Async Primitives | None | No job queue, callbacks, or SSE |
| Fixed Concurrency | 5 | Only 5 steps can execute in parallel |

#### What's Missing

- No asynchronous task submission
- No job queue for background processing
- No progress reporting mechanism
- No task status tracking
- No checkpointing for recovery
- No webhook/callback support
- No SSE/WebSocket for streaming progress
- **ExecutionOptions timeout enforcement** (StepTimeout, TotalTimeout are defined but not wired to execution)

---

## Industry Best Practices

### Pattern 1: Asynchronous Request-Reply (HTTP 202 Accepted)

**The gold standard for long-running HTTP operations.**

> Source: [Microsoft Azure Architecture - Async Request-Reply](https://learn.microsoft.com/en-us/azure/architecture/patterns/async-request-reply)

```
┌─────────┐          ┌─────────────┐          ┌────────────┐
│  Client │          │  API Server │          │   Worker   │
└────┬────┘          └──────┬──────┘          └─────┬──────┘
     │                      │                       │
     │ POST /orchestrate    │                       │
     │─────────────────────>│                       │
     │                      │                       │
     │                      │  Queue Job            │
     │                      │──────────────────────>│
     │                      │                       │
     │ 202 Accepted         │                       │
     │ Location: /jobs/123  │                       │
     │<─────────────────────│                       │
     │                      │                       │
     │                      │       Process...      │
     │ GET /jobs/123/status │       (minutes)       │
     │─────────────────────>│                       │
     │                      │                       │
     │ 200 {status:running} │                       │
     │<─────────────────────│                       │
     │                      │                       │
     │ GET /jobs/123/status │       Complete!       │
     │─────────────────────>│<──────────────────────│
     │                      │                       │
     │ 303 See Other        │                       │
     │ Location: /result    │                       │
     │<─────────────────────│                       │
```

**Key HTTP Status Codes:**
- `202 Accepted` - Task queued, processing will happen asynchronously
- `200 OK` - Status check successful, task still running
- `303 See Other` - Task complete, redirect to result
- `200 OK` - Result retrieval successful

**Polling Strategies:**
- **Fixed interval**: Poll every N seconds
- **Exponential backoff**: Start at 1s, double each time (1s, 2s, 4s, 8s...)
- **Adaptive**: Server provides `Retry-After` header

### Pattern 2: Webhook Callbacks

> Source: [AWS Architecture Blog - Managing Async Workflows](https://aws.amazon.com/blogs/architecture/managing-asynchronous-workflows-with-a-rest-api/)

```
Client provides callback URL when submitting task:

POST /orchestrate
{
    "task": "research AI trends",
    "callback_url": "https://client.com/webhook/complete",
    "callback_events": ["progress", "complete", "error"]
}

Server calls back when events occur:

POST https://client.com/webhook/complete
{
    "task_id": "task-123",
    "event": "complete",
    "status": "success",
    "result": { ... }
}
```

**Advantages:**
- No polling overhead
- Real-time notifications
- Client doesn't need to maintain connection

**Disadvantages:**
- Client must expose HTTP endpoint
- Webhook delivery can fail
- Not suitable for browser clients

**Best for:** Service-to-service communication, CI/CD pipelines, automated workflows.

### Pattern 3: Server-Sent Events (SSE) for Progress Streaming

```
Client                              Server
  │                                   │
  │ GET /tasks/123/stream             │
  │ Accept: text/event-stream         │
  │──────────────────────────────────>│
  │                                   │
  │ HTTP/1.1 200 OK                   │
  │ Content-Type: text/event-stream   │
  │<──────────────────────────────────│
  │                                   │
  │ event: progress                   │
  │ data: {"step":1,"pct":20}         │
  │<──────────────────────────────────│
  │                                   │
  │ event: progress                   │
  │ data: {"step":2,"pct":50}         │
  │<──────────────────────────────────│
  │                                   │
  │ event: complete                   │
  │ data: {"result_url":"/result"}    │
  │<──────────────────────────────────│
  │                                   │
  │ Connection closed                 │
```

**Advantages:**
- Real-time progress updates
- Works in browsers natively
- Auto-reconnection built into EventSource API
- Unidirectional (server to client) - simpler than WebSocket

**Disadvantages:**
- Connection held open
- Limited browser connections per domain (6)
- Not bidirectional

**Best for:** Browser clients needing real-time progress.

### Pattern 4: Queue-Based Architecture

> Source: [CodeOpinion - Avoiding Long-Running HTTP Requests](https://codeopinion.com/avoiding-long-running-http-api-requests/)

```
┌────────────┐     ┌─────────────┐     ┌─────────────┐     ┌──────────┐
│   Client   │────>│  HTTP API   │────>│    Queue    │────>│  Worker  │
└────────────┘     └─────────────┘     │  (Redis)    │     │  Pool    │
                          │            └─────────────┘     └────┬─────┘
                          │                                     │
                   ┌──────▼──────┐                              │
                   │  Job Store  │<─────────────────────────────┘
                   │  (Redis)    │     Update status/results
                   └─────────────┘
```

**Components:**
1. **API Server**: Accepts requests, enqueues tasks, returns 202
2. **Queue**: Redis list/stream for pending tasks
3. **Worker Pool**: Processes tasks from queue
4. **Job Store**: Persists task state, progress, results

**Go Libraries for Redis Job Queues:**

| Library | GitHub Stars | Features |
|---------|--------------|----------|
| [Asynq](https://github.com/hibiken/asynq) | 9k+ | Simple, reliable, Redis-backed, recommended |
| [Machinery](https://github.com/RichardKnop/machinery) | 7k+ | Workflows, multiple brokers |
| [Taskq](https://github.com/vmihailenco/taskq) | 1k+ | Multi-backend (Redis, SQS) |

### Pattern 5: Checkpointing for Recovery

> Source: [Kinde - Orchestrating Multi-Step Agents](https://kinde.com/learn/ai-for-software-engineering/ai-devops/orchestrating-multi-step-agents-patterns-for-long-running-work/)

For truly long-running workflows (hours), save state at each step:

```go
type WorkflowCheckpoint struct {
    TaskID          string                 `json:"task_id"`
    CurrentStep     int                    `json:"current_step"`
    CompletedSteps  []StepResult           `json:"completed_steps"`
    State           map[string]interface{} `json:"state"`
    LastUpdated     time.Time              `json:"last_updated"`
}
```

**Recovery Flow:**
1. Worker crashes mid-workflow
2. Task times out in queue (visibility timeout)
3. Another worker picks up task
4. Worker loads checkpoint from store
5. Resumes from last completed step

**Critical for:**
- Workflows > 5 minutes
- Expensive operations (don't repeat completed work)
- Stateful multi-step processes

### Pattern 6: AI Agent Specific Patterns

> Source: [Microsoft - AI Agent Design Patterns](https://learn.microsoft.com/en-us/azure/architecture/ai-ml/guide/ai-agent-design-patterns)

**Asynchronous Handoffs:**
> "For tasks that require waiting—for an API call to complete, for a scheduled time to pass, or for a human to click a button—the orchestrator should not block resources. It should pause the workflow efficiently and resume only when the external event occurs."

**Human-in-the-Loop:**
> "Many automated processes require human judgment for approval or review. The orchestrator must be able to pause the workflow and wait for external human input."

**Compensating Actions (Saga Pattern):**
When a step fails mid-workflow, undo previous steps:
```
Step 1: Reserve inventory ✓
Step 2: Charge payment ✓
Step 3: Ship order ✗ (failed)
        ↓
Compensate: Refund payment
Compensate: Release inventory
```

---

## Proposed Architecture

### High-Level Design

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         GoMind Async Layer                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌──────────────────┐     ┌──────────────────┐     ┌────────────────┐  │
│  │    HTTP API      │     │   Task Queue     │     │  Worker Pool   │  │
│  │                  │     │    (Redis)       │     │                │  │
│  │ POST /tasks      │────>│                  │────>│ ┌────────────┐ │  │
│  │ GET /tasks/:id   │     │  Priority queues │     │ │  Worker 1  │ │  │
│  │ GET /tasks/:id/  │     │  • high          │     │ │            │ │  │
│  │     status       │     │  • normal        │     │ │ SmartExec  │ │  │
│  │ GET /tasks/:id/  │     │  • low           │     │ └────────────┘ │  │
│  │     stream (SSE) │     │                  │     │ ┌────────────┐ │  │
│  │ POST /tasks/:id/ │     │  BRPOP for       │     │ │  Worker 2  │ │  │
│  │     cancel       │     │  blocking dequeue│     │ └────────────┘ │  │
│  └────────┬─────────┘     └──────────────────┘     │ ┌────────────┐ │  │
│           │                                        │ │  Worker N  │ │  │
│           │                                        │ └────────────┘ │  │
│           │                                        └───────┬────────┘  │
│           │                                                │           │
│           │         ┌──────────────────────────┐           │           │
│           └────────>│       Task Store         │<──────────┘           │
│                     │        (Redis)           │                       │
│                     │                          │                       │
│                     │ • Task metadata          │                       │
│                     │ • Status tracking        │                       │
│                     │ • Progress updates       │                       │
│                     │ • Results storage        │                       │
│                     │ • Checkpoints            │                       │
│                     │ • SSE pub/sub            │                       │
│                     └──────────────────────────┘                       │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| **HTTP API** | Accept task submissions, return 202, serve status/stream endpoints |
| **Task Queue** | Priority-based FIFO queue, blocking dequeue, at-least-once delivery |
| **Task Store** | Persist task state, enable status queries, pub/sub for SSE |
| **Worker Pool** | Process tasks, report progress, handle failures, checkpoint |
| **SmartExecutor** | Execute orchestration steps (existing component, reused) |

### Design Principles

1. **Leverage Existing Infrastructure**: Use Redis (already used for discovery)
2. **Minimal New Dependencies**: Avoid external workflow engines (Temporal, etc.)
3. **Kubernetes-Native**: Workers scale independently via HPA
4. **Interface-Based**: Define interfaces in `core/`, implementations in `orchestration/`
5. **Backwards Compatible**: Existing sync endpoints continue to work

---

## API Design

### Task Lifecycle

```
┌─────────┐     ┌─────────┐     ┌───────────┐     ┌───────────┐
│ queued  │────>│ running │────>│ completed │     │  failed   │
└─────────┘     └────┬────┘     └───────────┘     └───────────┘
                     │                                   ▲
                     │          ┌───────────┐            │
                     └─────────>│ cancelled │────────────┘
                                └───────────┘
```

### HTTP Endpoints

#### 1. Submit Task

```http
POST /api/v1/tasks
Content-Type: application/json

{
    "type": "orchestration",
    "input": {
        "goal": "Research AI trends and write a comprehensive report",
        "context": {
            "depth": "detailed",
            "sources": ["academic", "news", "blogs"]
        }
    },
    "options": {
        "timeout": "30m",
        "priority": "normal",
        "callback_url": "https://example.com/webhook",
        "callback_events": ["complete", "error"]
    }
}
```

**Response: 202 Accepted**
```json
{
    "task_id": "task-abc123def456",
    "status": "queued",
    "created_at": "2025-01-15T10:00:00Z",
    "links": {
        "self": "/api/v1/tasks/task-abc123def456",
        "status": "/api/v1/tasks/task-abc123def456/status",
        "stream": "/api/v1/tasks/task-abc123def456/stream",
        "cancel": "/api/v1/tasks/task-abc123def456/cancel",
        "result": "/api/v1/tasks/task-abc123def456/result"
    }
}
```

#### 2. Get Task Status (Polling)

```http
GET /api/v1/tasks/task-abc123def456/status
```

**Response: 200 OK (Running)**
```json
{
    "task_id": "task-abc123def456",
    "status": "running",
    "progress": {
        "current_step": 3,
        "total_steps": 5,
        "step_name": "Synthesizing research findings",
        "percentage": 60,
        "message": "Processing 15 sources..."
    },
    "created_at": "2025-01-15T10:00:00Z",
    "started_at": "2025-01-15T10:00:05Z",
    "estimated_completion": "2025-01-15T10:05:00Z",
    "retry_after": 5
}
```

**Response: 303 See Other (Completed)**
```http
HTTP/1.1 303 See Other
Location: /api/v1/tasks/task-abc123def456/result
```

#### 3. Stream Progress (SSE)

```http
GET /api/v1/tasks/task-abc123def456/stream
Accept: text/event-stream
```

**Response: 200 OK**
```
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive

event: status
data: {"status":"running","started_at":"2025-01-15T10:00:05Z"}

event: progress
data: {"current_step":1,"total_steps":5,"step_name":"Searching sources","percentage":20}

event: progress
data: {"current_step":2,"total_steps":5,"step_name":"Analyzing results","percentage":40}

event: progress
data: {"current_step":3,"total_steps":5,"step_name":"Synthesizing findings","percentage":60}

event: progress
data: {"current_step":4,"total_steps":5,"step_name":"Generating report","percentage":80}

event: progress
data: {"current_step":5,"total_steps":5,"step_name":"Finalizing","percentage":100}

event: complete
data: {"task_id":"task-abc123def456","result_url":"/api/v1/tasks/task-abc123def456/result"}
```

#### 4. Get Result

```http
GET /api/v1/tasks/task-abc123def456/result
```

**Response: 200 OK**
```json
{
    "task_id": "task-abc123def456",
    "status": "completed",
    "result": {
        "report": "# AI Trends Report\n\n## Executive Summary...",
        "sources_used": 15,
        "confidence_score": 0.87
    },
    "metadata": {
        "duration_ms": 180000,
        "steps_executed": 5,
        "tokens_used": 15000,
        "tools_invoked": ["search", "analyze", "synthesize"]
    },
    "created_at": "2025-01-15T10:00:00Z",
    "started_at": "2025-01-15T10:00:05Z",
    "completed_at": "2025-01-15T10:03:05Z"
}
```

#### 5. Cancel Task

```http
POST /api/v1/tasks/task-abc123def456/cancel
```

**Response: 200 OK**
```json
{
    "task_id": "task-abc123def456",
    "status": "cancelled",
    "cancelled_at": "2025-01-15T10:01:30Z",
    "partial_result": {
        "completed_steps": 2,
        "partial_output": "..."
    }
}
```

#### 6. List Tasks

```http
GET /api/v1/tasks?status=running&limit=10&offset=0
```

**Response: 200 OK**
```json
{
    "tasks": [
        {
            "task_id": "task-abc123",
            "type": "orchestration",
            "status": "running",
            "created_at": "2025-01-15T10:00:00Z",
            "progress": {"percentage": 60}
        }
    ],
    "total": 42,
    "limit": 10,
    "offset": 0
}
```

### Webhook Callback Format

When `callback_url` is provided, the server sends POST requests:

```http
POST https://example.com/webhook
Content-Type: application/json
X-GoMind-Signature: sha256=...
X-GoMind-Event: complete

{
    "event": "complete",
    "task_id": "task-abc123def456",
    "timestamp": "2025-01-15T10:03:05Z",
    "data": {
        "status": "completed",
        "result_url": "https://gomind.example.com/api/v1/tasks/task-abc123def456/result"
    }
}
```

**Event Types:**
- `queued` - Task accepted and queued
- `started` - Task processing began
- `progress` - Progress update (if requested)
- `complete` - Task completed successfully
- `error` - Task failed
- `cancelled` - Task was cancelled

---

## Core Interfaces (v1)

> **Scope**: v1 MVP - minimal interfaces to solve the timeout problem

### Location: `core/async_task.go`

```go
package core

import (
    "context"
    "errors"
    "time"
)

// ═══════════════════════════════════════════════════════════════════════════
// Errors
// ═══════════════════════════════════════════════════════════════════════════

// ErrTaskNotFound is returned when a task cannot be found
var ErrTaskNotFound = errors.New("task not found")

// ErrTaskNotCancellable is returned when a task cannot be cancelled (already completed/failed/cancelled)
var ErrTaskNotCancellable = errors.New("task not cancellable")

// ═══════════════════════════════════════════════════════════════════════════
// Types
// ═══════════════════════════════════════════════════════════════════════════

// TaskStatus represents the state of a long-running task
type TaskStatus string

const (
    TaskStatusQueued    TaskStatus = "queued"
    TaskStatusRunning   TaskStatus = "running"
    TaskStatusCompleted TaskStatus = "completed"
    TaskStatusFailed    TaskStatus = "failed"
    TaskStatusCancelled TaskStatus = "cancelled"
)

// Task represents a long-running async task
type Task struct {
    ID          string                 `json:"id"`
    Type        string                 `json:"type"`
    Status      TaskStatus             `json:"status"`
    Input       map[string]interface{} `json:"input"`
    Result      interface{}            `json:"result,omitempty"`
    Error       *TaskError             `json:"error,omitempty"`
    Progress    *TaskProgress          `json:"progress,omitempty"`
    Options     TaskOptions            `json:"options"`
    CreatedAt   time.Time              `json:"created_at"`
    StartedAt   *time.Time             `json:"started_at,omitempty"`
    CompletedAt *time.Time             `json:"completed_at,omitempty"`
    CancelledAt *time.Time             `json:"cancelled_at,omitempty"` // v1: When task was cancelled

    // Trace context for distributed tracing continuity
    // These fields preserve the trace chain across async boundaries
    TraceID      string `json:"trace_id,omitempty"`       // W3C trace ID from original request
    ParentSpanID string `json:"parent_span_id,omitempty"` // Span ID of the submitting request
}

// TaskProgress tracks execution progress
type TaskProgress struct {
    CurrentStep int     `json:"current_step"`
    TotalSteps  int     `json:"total_steps"`
    StepName    string  `json:"step_name"`
    Percentage  float64 `json:"percentage"`
    Message     string  `json:"message,omitempty"`
}

// TaskOptions configures task execution
type TaskOptions struct {
    Timeout time.Duration `json:"timeout"`
}

// TaskError contains error information
type TaskError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════
// Interfaces (v1 MVP)
// ═══════════════════════════════════════════════════════════════════════════

// TaskQueue handles async task submission and retrieval
type TaskQueue interface {
    // Enqueue adds a task to the queue
    Enqueue(ctx context.Context, task *Task) error

    // Dequeue retrieves the next task (blocking with timeout)
    // Returns nil, nil if timeout expires with no task
    Dequeue(ctx context.Context, timeout time.Duration) (*Task, error)

    // Acknowledge marks a task as successfully processed
    Acknowledge(ctx context.Context, taskID string) error

    // Reject returns a task to the queue (for retry)
    Reject(ctx context.Context, taskID string, reason string) error
}

// TaskStore persists task state and results
type TaskStore interface {
    // Create persists a new task
    Create(ctx context.Context, task *Task) error

    // Get retrieves task by ID
    Get(ctx context.Context, taskID string) (*Task, error)

    // Update persists task changes (status, progress, result)
    Update(ctx context.Context, task *Task) error

    // Delete removes a task (for cleanup)
    Delete(ctx context.Context, taskID string) error

    // Cancel marks a task as cancelled
    // Returns ErrTaskNotFound if task doesn't exist
    // Returns ErrTaskNotCancellable if task is already completed/failed/cancelled
    Cancel(ctx context.Context, taskID string) error
}

// TaskWorker processes tasks from the queue
type TaskWorker interface {
    // Start begins processing tasks (blocking)
    Start(ctx context.Context) error

    // Stop gracefully stops the worker
    Stop(ctx context.Context) error

    // RegisterHandler registers a handler for a task type
    RegisterHandler(taskType string, handler TaskHandler) error
}

// TaskHandler processes a specific task type
type TaskHandler func(ctx context.Context, task *Task, reporter ProgressReporter) error

// ProgressReporter allows handlers to report progress
type ProgressReporter interface {
    // Report updates task progress (stored in TaskStore)
    Report(progress *TaskProgress) error
}

// ═══════════════════════════════════════════════════════════════════════════
// Configuration
// ═══════════════════════════════════════════════════════════════════════════

// AsyncTaskConfig configures the async task system
type AsyncTaskConfig struct {
    // Queue settings
    QueuePrefix string `json:"queue_prefix"`

    // Worker settings
    WorkerCount     int           `json:"worker_count"`
    DequeueTimeout  time.Duration `json:"dequeue_timeout"`
    ShutdownTimeout time.Duration `json:"shutdown_timeout"`

    // Task defaults
    DefaultTimeout time.Duration `json:"default_timeout"`

    // Cleanup settings
    ResultTTL time.Duration `json:"result_ttl"`
}

// DefaultAsyncTaskConfig returns sensible defaults
func DefaultAsyncTaskConfig() AsyncTaskConfig {
    return AsyncTaskConfig{
        QueuePrefix:     "gomind:tasks",
        WorkerCount:     5,
        DequeueTimeout:  30 * time.Second,
        ShutdownTimeout: 30 * time.Second,
        DefaultTimeout:  30 * time.Minute,
        ResultTTL:       24 * time.Hour,
    }
}
```

---

## Pluggable Infrastructure Design

This section describes how the async task infrastructure supports **extensibility without bloat**, allowing enterprise developers to implement their own backends while GoMind ships only what's needed for most users.

### Design Philosophy: Lean by Default, Extensible When Needed

> **Principle**: Ship only Redis. Define clean interfaces. Let the community contribute the rest.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Application Layer                                    │
│                    (Your Agents & Orchestration)                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                     INTERFACES (core/)                                  │ │
│  │                                                                        │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────────┐ │ │
│  │  │  TaskQueue   │  │  TaskStore   │  │  TaskEventPublisher          │ │ │
│  │  │              │  │              │  │                              │ │ │
│  │  │ • Enqueue()  │  │ • Create()   │  │ • Publish()                  │ │ │
│  │  │ • Dequeue()  │  │ • Get()      │  │ • Subscribe()                │ │ │
│  │  │ • Ack()      │  │ • Update()   │  │                              │ │ │
│  │  └──────────────┘  └──────────────┘  └──────────────────────────────┘ │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
├──────────────────────────────────────────────────────────────────────────────┤
│                      SHIPPED IMPLEMENTATION (orchestration/)                 │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │   Redis (production)                                                 │    │
│  │                                                                      │    │
│  │   • RedisTaskQueue    - LPUSH/BRPOP for queue operations            │    │
│  │   • RedisTaskStore    - Hash for task state persistence             │    │
│  │   • RedisEventPub     - Pub/Sub for real-time events                │    │
│  │                                                                      │    │
│  │   Why Redis? Already used by GoMind for discovery, battle-tested    │    │
│  │   for job queues, simple operations, widely available.              │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
├──────────────────────────────────────────────────────────────────────────────┤
│                      CUSTOM IMPLEMENTATIONS (your code)                      │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Implement TaskQueue/TaskStore/TaskEventPublisher interfaces to use:        │
│                                                                              │
│  • AWS SQS + DynamoDB    • RabbitMQ    • NATS    • Kafka                    │
│  • PostgreSQL            • Your enterprise system                           │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Why This Approach?

| Aspect | Over-Engineered (Rejected) | Lean (Adopted) |
|--------|---------------------------|----------------|
| **Lines of Code** | ~10,000+ | ~2,400 |
| **Shipped Backends** | 7+ (Redis, SQS, RabbitMQ, NATS, Kafka, PostgreSQL, etc.) | 1 (Redis) |
| **Dependencies** | AWS SDK, AMQP, NATS, Kafka clients | Only go-redis |
| **Abstraction Layers** | 2 (Portable Type + Driver) | 1 (Interface only) |
| **Time to Implement** | Long | Short |
| **Maintenance Burden** | High | Low |

**Bottom line**: Most users need Redis, and GoMind already uses Redis for service discovery. The few who need SQS/RabbitMQ/NATS can implement the interface themselves or wait for community contributions.

---

### Shipped Implementation

#### Redis (Production Default)

Redis is the default and recommended backend because:
- **Already used** by GoMind for service discovery
- **Battle-tested** for job queues (Sidekiq, Resque, Asynq)
- **Simple operations** (LPUSH/BRPOP for queue, Hash for store, Pub/Sub for events)
- **Widely available** (ElastiCache, Redis Cloud, self-hosted)

```go
// Location: orchestration/redis_task_queue.go

package orchestration

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
    "github.com/itsneelabh/gomind/core"
)

// RedisTaskQueue implements core.TaskQueue using Redis
type RedisTaskQueue struct {
    client *redis.Client
    config RedisQueueConfig
}

// RedisQueueConfig configures the Redis queue
type RedisQueueConfig struct {
    Prefix string // Key prefix, default: "gomind:tasks"
}

// NewRedisTaskQueue creates a new Redis-backed task queue
func NewRedisTaskQueue(client *redis.Client, config RedisQueueConfig) *RedisTaskQueue {
    if config.Prefix == "" {
        config.Prefix = "gomind:tasks"
    }
    return &RedisTaskQueue{client: client, config: config}
}

// Redis Key Schema (v1 - single queue, no priority)
// Queue: gomind:tasks:queue - List (LPUSH/BRPOP)
// Task:  gomind:tasks:data:{task_id} - Hash

func (q *RedisTaskQueue) Enqueue(ctx context.Context, task *core.Task) error {
    // Serialize task
    data, err := json.Marshal(task)
    if err != nil {
        return err
    }

    // Push to queue (v1: single queue, no priority)
    key := fmt.Sprintf("%s:queue", q.config.Prefix)
    return q.client.LPush(ctx, key, data).Err()
}

func (q *RedisTaskQueue) Dequeue(ctx context.Context, timeout time.Duration) (*core.Task, error) {
    // BRPOP from queue (v1: single queue)
    key := fmt.Sprintf("%s:queue", q.config.Prefix)

    result, err := q.client.BRPop(ctx, timeout, key).Result()
    if err == redis.Nil {
        return nil, nil // Timeout, no task
    }
    if err != nil {
        return nil, err
    }

    var task core.Task
    if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
        return nil, err
    }

    return &task, nil
}

func (q *RedisTaskQueue) Acknowledge(ctx context.Context, taskID string) error {
    // With BRPOP, task is already removed from queue
    // Nothing to do here
    return nil
}

func (q *RedisTaskQueue) Reject(ctx context.Context, taskID string, reason string) error {
    // v1: Simple re-enqueue (no delay, no retry tracking)
    // v2 will add retry delay and attempt tracking
    return nil
}

// Size returns the current queue length (not part of interface, for monitoring)
func (q *RedisTaskQueue) Size(ctx context.Context) (int64, error) {
    key := fmt.Sprintf("%s:queue", q.config.Prefix)
    return q.client.LLen(ctx, key).Result()
}
```

---

### Configuration

#### Environment Variables

```bash
# Redis URL (standard format)
export GOMIND_REDIS_URL="redis://localhost:6379/0"

# Or with password
export GOMIND_REDIS_URL="redis://:password@localhost:6379/0"

# Or with TLS (AWS ElastiCache, etc.)
export GOMIND_REDIS_URL="rediss://redis.example.com:6380/0"
```

#### Programmatic Configuration

```go
// Simple: Use existing Redis client (recommended)
redisClient := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: os.Getenv("REDIS_PASSWORD"),
    DB:       0,
})

taskQueue := orchestration.NewRedisTaskQueue(redisClient, orchestration.RedisQueueConfig{
    Prefix: "gomind:tasks",
})

taskStore := orchestration.NewRedisTaskStore(redisClient, orchestration.RedisStoreConfig{
    Prefix: "gomind:tasks",
})

eventPublisher := orchestration.NewRedisEventPublisher(redisClient, orchestration.RedisEventConfig{
    Prefix: "gomind:tasks",
})
```

---

### Implementing Custom Backends

If you need SQS, RabbitMQ, NATS, Kafka, or another backend, implement the interfaces defined in `core/async_task.go`:

```go
// Your custom implementation (e.g., in your project, not GoMind)
package mycompany

import (
    "context"
    "time"

    "github.com/aws/aws-sdk-go-v2/service/sqs"
    "github.com/itsneelabh/gomind/core"
)

// SQSTaskQueue implements core.TaskQueue using AWS SQS
type SQSTaskQueue struct {
    client   *sqs.Client
    queueURL string
}

func NewSQSTaskQueue(client *sqs.Client, queueURL string) *SQSTaskQueue {
    return &SQSTaskQueue{client: client, queueURL: queueURL}
}

func (q *SQSTaskQueue) Enqueue(ctx context.Context, task *core.Task) error {
    data, _ := json.Marshal(task)

    _, err := q.client.SendMessage(ctx, &sqs.SendMessageInput{
        QueueUrl:    &q.queueURL,
        MessageBody: aws.String(string(data)),
    })
    return err
}

func (q *SQSTaskQueue) Dequeue(ctx context.Context, timeout time.Duration) (*core.Task, error) {
    result, err := q.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
        QueueUrl:            &q.queueURL,
        MaxNumberOfMessages: 1,
        WaitTimeSeconds:     int32(timeout.Seconds()),
    })
    if err != nil {
        return nil, err
    }

    if len(result.Messages) == 0 {
        return nil, nil
    }

    var task core.Task
    json.Unmarshal([]byte(*result.Messages[0].Body), &task)
    return &task, nil
}

func (q *SQSTaskQueue) Acknowledge(ctx context.Context, taskID string) error {
    // Delete message from SQS using receipt handle
    // (requires storing receipt handle during Dequeue)
    return nil
}

func (q *SQSTaskQueue) Reject(ctx context.Context, taskID string, reason string) error {
    // Change visibility timeout for retry
    return nil
}
```

**Usage:**

```go
// In your application
sqsClient := sqs.NewFromConfig(awsConfig)
taskQueue := mycompany.NewSQSTaskQueue(sqsClient, "https://sqs.us-east-1.amazonaws.com/123456789/my-queue")

// Use with GoMind worker
worker := orchestration.NewTaskWorker(taskQueue, taskStore, workerConfig)
```

---

### Backend Selection Guide

| Use Case | Recommended Backend | Why |
|----------|-------------------|-----|
| **Default / Getting Started** | Redis | Simple, battle-tested, already used by GoMind |
| **AWS Native** | SQS + DynamoDB | Managed, auto-scaling, pay-per-use (implement interface) |
| **High Throughput** | NATS JetStream | ~1M msgs/sec, lightweight (implement interface) |
| **Complex Routing** | RabbitMQ | Exchanges, routing keys, dead letters (implement interface) |
| **Event Sourcing** | Kafka | Persistent log, replay capability (implement interface) |
| **Single Database** | PostgreSQL | SKIP LOCKED + LISTEN/NOTIFY (implement interface) |

---

### Summary

| Question | Answer |
|----------|--------|
| **What backends ship with GoMind?** | Redis (leverages existing discovery infrastructure) |
| **Can I use SQS/RabbitMQ/NATS?** | Yes, implement the `TaskQueue`/`TaskStore`/`TaskEventPublisher` interfaces |
| **Is Redis required?** | Yes for async tasks (already required for service discovery) |
| **Where are the interfaces?** | `core/async_task.go` |
| **Will GoMind add more backends?** | Community contributions welcome via separate packages |

**Design Principle**: Keep the framework lean. Redis is already a dependency for GoMind's service discovery, so we leverage it for async tasks too.

---

## Integration with SmartExecutor

### Wrapper Handler

The existing `SmartExecutor` is reused inside a `TaskHandler`. For v1, we keep it simple without checkpointing:

```go
// Location: orchestration/async_handler.go

package orchestration

import (
    "context"
    "fmt"

    "github.com/itsneelabh/gomind/core"
)

// NewOrchestrationTaskHandler creates a TaskHandler that wraps AIOrchestrator.ProcessRequest()
// Note: For v1, we use ProcessRequest() which handles both planning and execution.
// For v2, we may expose GenerateExecutionPlan() to separate planning from execution for progress reporting.
func NewOrchestrationTaskHandler(
    orchestrator *AIOrchestrator,
) core.TaskHandler {
    return func(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
        // Extract goal from task input
        goal, ok := task.Input["goal"].(string)
        if !ok {
            return fmt.Errorf("task input must contain 'goal' string")
        }

        // Create execution context with task timeout
        execCtx, cancel := context.WithTimeout(ctx, task.Options.Timeout)
        defer cancel()

        // Use ProcessRequest which handles both planning and execution
        // See orchestration/orchestrator.go:353 for implementation
        response, err := orchestrator.ProcessRequest(execCtx, goal, nil)
        if err != nil {
            return fmt.Errorf("orchestration failed: %w", err)
        }

        // Store result in task
        task.Result = response
        return nil
    }
}

// v1 Progress Reporting: Manual via ProgressReporter
// Handlers can optionally report progress during long operations:
//
//   reporter.Report(&core.TaskProgress{
//       CurrentStep: 2,
//       TotalSteps:  5,
//       StepName:    "Analyzing results",
//       Percentage:  40,
//   })
```

### Future: Modified SmartExecutor (v2)

In v2, we'll add progress callback support directly to SmartExecutor:

```go
// Location: orchestration/executor.go (v2 modifications)

// ProgressCallback is called after each step completes (v2)
type ProgressCallback func(stepIndex, totalSteps int, stepName string, result interface{})

// ExecuteWithProgress executes with progress reporting (v2)
func (e *SmartExecutor) ExecuteWithProgress(
    ctx context.Context,
    plan *RoutingPlan,
    progress ProgressCallback,
) (*ExecutionResult, error) {
    // ... existing execution logic ...

    // After each step completes:
    if progress != nil {
        progress(stepIndex, len(plan.Steps), step.Name, stepResult)
    }

    // ... rest of execution ...
}
```

> **Note**: For v1, the `SmartExecutor.Execute()` method is used as-is. Progress reporting is optional and done manually by the handler. v2 will add built-in progress callbacks.

---

## Telemetry Integration (v1 - Built-In)

> **Production Primitive**: Observability is not an afterthought. Distributed tracing and metrics are built into the async task system from v1 to ensure full visibility across async boundaries.

### The Problem: Broken Trace Chains

Without proper telemetry integration, async tasks break the distributed trace chain:

```
❌ Without Trace Context Propagation:

User Request ─────────────────────────────────────────────────────────
    │
    ▼
Agent (trace: abc123, span: 001)
    │
    ├── Submit task to Redis ────────── [TRACE CONTEXT LOST HERE]
    │
    └── Return 202 Accepted
                                        ┌─────────────────────────────
                                        │ Worker picks up task
                                        │ (NEW trace: xyz789) ← Disconnected!
                                        │     │
                                        │     ├── Call weather-tool (trace: xyz789)
                                        │     └── Call stock-tool (trace: xyz789)
                                        │
                                        │ User cannot connect original
                                        │ request to task execution!
                                        └─────────────────────────────
```

### The Solution: Trace Context in Task Struct

The `Task` struct includes trace context fields that preserve the trace chain:

```go
type Task struct {
    // ... other fields ...

    // Trace context for distributed tracing continuity
    TraceID      string `json:"trace_id,omitempty"`       // W3C trace ID
    ParentSpanID string `json:"parent_span_id,omitempty"` // Span ID of submitter
}
```

```
✅ With Trace Context Propagation:

User Request ─────────────────────────────────────────────────────────
    │
    ▼
Agent (trace: abc123, span: 001)
    │
    ├── Submit task with TraceID=abc123, ParentSpanID=001
    │
    └── Return 202 Accepted (includes task_id)
                                        ┌─────────────────────────────
                                        │ Worker picks up task
                                        │ Restores trace context:
                                        │ (trace: abc123, parent: 001)
                                        │     │
                                        │     ├── Call weather-tool (trace: abc123)
                                        │     └── Call stock-tool (trace: abc123)
                                        │
                                        │ Full trace visibility!
                                        │ User sees entire journey in Jaeger
                                        └─────────────────────────────
```

### Implementation: Submitting Tasks with Trace Context

When submitting an async task, extract and store trace context:

```go
// In your async task handler
func (h *AsyncTaskHandler) handleAsyncOrchestrate(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Extract trace context from incoming request
    tc := telemetry.GetTraceContext(ctx)

    // Create task with trace context
    task := &core.Task{
        ID:           generateTaskID(),
        Type:         "orchestration",
        Status:       core.TaskStatusQueued,
        Input:        request.Input,
        Options:      core.TaskOptions{Timeout: 10 * time.Minute},
        CreatedAt:    time.Now(),

        // Preserve trace context for worker
        TraceID:      tc.TraceID,
        ParentSpanID: tc.SpanID,
    }

    // Emit span event for task submission
    telemetry.AddSpanEvent(ctx, "task.submitted",
        attribute.String("task_id", task.ID),
        attribute.String("task_type", task.Type),
    )

    // Enqueue task
    if err := h.taskQueue.Enqueue(ctx, task); err != nil {
        telemetry.RecordSpanError(ctx, err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Log with trace correlation
    log.Printf("Task submitted task_id=%s trace_id=%s", task.ID, tc.TraceID)

    // Return 202 Accepted
    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(TaskSubmitResponse{
        TaskID:    task.ID,
        Status:    string(task.Status),
        StatusURL: fmt.Sprintf("/api/tasks/%s", task.ID),
    })
}
```

### Implementation: Worker with Trace Context Restoration

The worker restores trace context when processing tasks.

> **Architecture Note**: Per [FRAMEWORK_DESIGN_PRINCIPLES.md](../FRAMEWORK_DESIGN_PRINCIPLES.md), the orchestration module can import `core` and `telemetry` only. The code below uses the telemetry module's public API. Direct OpenTelemetry usage is acceptable since `telemetry` already depends on OTel as a transitive dependency.

```go
import (
    "context"

    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
    "go.opentelemetry.io/otel/attribute"  // For span attributes
)

// TaskWorker processes tasks with trace context restoration
type TaskWorker struct {
    taskQueue  core.TaskQueue
    taskStore  core.TaskStore
    handler    TaskHandler
    telemetry  core.Telemetry  // Injected, optional
    config     WorkerConfig
}

func (w *TaskWorker) processTask(task *core.Task) error {
    // Restore trace context and create processing span
    // Uses telemetry module's helper for async task tracing
    ctx, endSpan := telemetry.StartLinkedSpan(
        context.Background(),
        "task.process",
        task.TraceID,
        task.ParentSpanID,
        map[string]string{
            "task.id":   task.ID,
            "task.type": task.Type,
        },
    )
    defer endSpan()

    // Emit task started event using telemetry module API
    // Note: AddSpanEvent uses attribute.KeyValue from go.opentelemetry.io/otel/attribute
    telemetry.AddSpanEvent(ctx, "task.started",
        attribute.String("task_id", task.ID),
    )

    // Process the task (orchestration, etc.)
    result, err := w.handler.Handle(ctx, task)

    if err != nil {
        // Emit task failed event
        telemetry.AddSpanEvent(ctx, "task.failed",
            attribute.String("task_id", task.ID),
            attribute.String("error", err.Error()),
        )
        telemetry.RecordSpanError(ctx, err)
        return err
    }

    // Emit task completed event
    telemetry.AddSpanEvent(ctx, "task.completed",
        attribute.String("task_id", task.ID),
        attribute.Int("result_size", len(result)),
    )

    return nil
}
```

### Telemetry Module Prerequisite

The async task system requires a new function in the `telemetry` module for creating linked spans:

```go
// telemetry/async_span.go - NEW file required for v1

// StartLinkedSpan creates a span linked to a parent trace context.
// Used for async task workers where trace context is restored from storage.
//
// Parameters:
//   - ctx: Base context (usually context.Background() for workers)
//   - name: Span name (e.g., "task.process")
//   - traceID: W3C trace ID from stored task
//   - parentSpanID: Parent span ID from stored task
//   - attributes: Key-value pairs to attach to the span
//
// Returns:
//   - ctx: Context with the new span
//   - endSpan: Function to call when span is complete (defer endSpan())
func StartLinkedSpan(
    ctx context.Context,
    name string,
    traceID string,
    parentSpanID string,
    attributes map[string]string,
) (context.Context, func()) {
    // Implementation uses OpenTelemetry internally
    // Creates span with trace.WithLinks to parent
    // ...
}
```

This follows the pattern established in [DISTRIBUTED_TRACING_GUIDE.md](../docs/DISTRIBUTED_TRACING_GUIDE.md) where the telemetry module provides high-level APIs that wrap OpenTelemetry internals.

### Task Lifecycle Span Events

The async task system emits standardized span events for full observability:

| Event Name | When Emitted | Key Attributes |
|------------|--------------|----------------|
| `task.submitted` | Task enqueued to Redis | `task_id`, `task_type`, `trace_id` |
| `task.started` | Worker begins processing | `task_id`, `worker_id` |
| `task.progress` | Progress update | `task_id`, `step`, `percentage`, `message` |
| `task.completed` | Task finished successfully | `task_id`, `duration_ms`, `result_size` |
| `task.failed` | Task failed | `task_id`, `error`, `error_code` |
| `task.cancelled` | Task was cancelled | `task_id`, `cancelled_by` |
| `task.timeout` | Task exceeded timeout | `task_id`, `timeout_ms` |

### Viewing Async Tasks in Jaeger

With proper trace context propagation, you'll see the complete async flow in Jaeger:

```
research-agent: POST /api/tasks/submit (trace: abc123)
├── task.submitted (event: task_id=task-001)
└── HTTP 202 Accepted (15ms)

... time passes ...

research-agent-worker: task.process (trace: abc123, linked to parent)
├── task.started (event)
├── orchestrator.process_request
│   ├── ai.generate_response (1.2s)
│   ├── HTTP POST → weather-tool (200ms)
│   ├── HTTP POST → stock-tool (180ms)
│   └── ai.generate_response (800ms)
├── task.progress (event: 100%, "Synthesis complete")
└── task.completed (event: duration_ms=2450)
```

### Metrics for Async Tasks (v1)

The async task system emits Prometheus metrics for monitoring:

```go
// Task queue metrics
telemetry.Gauge("gomind.tasks.queue_depth", queueDepth,
    "queue", queueName)

// Task processing metrics
telemetry.Counter("gomind.tasks.submitted",
    "task_type", task.Type,
    "service", serviceName)

telemetry.Counter("gomind.tasks.completed",
    "task_type", task.Type,
    "status", string(task.Status)) // completed, failed, cancelled

telemetry.Histogram("gomind.tasks.duration_ms", duration.Milliseconds(),
    "task_type", task.Type,
    "status", string(task.Status))

telemetry.Histogram("gomind.tasks.queue_wait_ms", waitTime.Milliseconds(),
    "task_type", task.Type)
```

### Grafana Dashboard Queries

Example PromQL queries for async task monitoring:

```promql
# Task submission rate
rate(gomind_tasks_submitted_total[5m])

# Task completion rate by status
rate(gomind_tasks_completed_total[5m])

# Average task duration (p95)
histogram_quantile(0.95, rate(gomind_tasks_duration_ms_bucket[5m]))

# Queue depth (tasks waiting)
gomind_tasks_queue_depth

# Task queue wait time (p95)
histogram_quantile(0.95, rate(gomind_tasks_queue_wait_ms_bucket[5m]))

# Task failure rate
rate(gomind_tasks_completed_total{status="failed"}[5m])
  / rate(gomind_tasks_completed_total[5m])
```

### Log Correlation

All async task logs include trace context for correlation:

```go
// In task handler
func (h *Handler) Handle(ctx context.Context, task *core.Task) error {
    tc := telemetry.GetTraceContext(ctx)

    // All logs include trace_id and task_id
    log.Printf("Processing task task_id=%s trace_id=%s span_id=%s type=%s",
        task.ID, tc.TraceID, tc.SpanID, task.Type)

    // ... process task ...

    log.Printf("Task completed task_id=%s trace_id=%s duration_ms=%d",
        task.ID, tc.TraceID, duration.Milliseconds())

    return nil
}
```

**Log search by trace_id:**
```bash
# Find all logs for an async task across API and worker
kubectl logs -n gomind-examples -l app=research-agent --all-containers | grep "abc123"
```

### Troubleshooting Async Tasks

| What You Have | How to Debug |
|---------------|--------------|
| `task_id` from 202 response | Search Jaeger: tag `task.id=<value>` |
| `trace_id` from logs | Direct URL: `http://jaeger:16686/trace/<trace_id>` |
| Slow tasks | Check `gomind_tasks_duration_ms` histogram in Grafana |
| Queue buildup | Alert on `gomind_tasks_queue_depth > 100` |
| High failure rate | Check `task.failed` events in Jaeger for error details |

### Summary: Telemetry in v1

| Feature | Status | Description |
|---------|--------|-------------|
| **Trace Context in Task** | ✅ v1 | `TraceID`, `ParentSpanID` fields |
| **Trace Restoration in Worker** | ✅ v1 | Worker links to original trace |
| **Task Lifecycle Events** | ✅ v1 | submitted, started, progress, completed, failed |
| **Prometheus Metrics** | ✅ v1 | Queue depth, duration, completion counts |
| **Log Correlation** | ✅ v1 | All logs include `trace_id`, `task_id` |
| **SSE with Trace Context** | ⏳ v2 | Real-time events with trace propagation |

**Observability is a first-class citizen, not an afterthought.**

---

## Worker Deployment Architecture

> **Production Primitive**: GoMind is built on production primitives. The worker deployment model follows industry best practices for Kubernetes-native, long-term maintainable systems.

### Design Decision: Separate API and Worker Deployments

After analyzing cognitive load for developers and ops engineers, we recommend **separate deployments** using the same binary with a mode flag. This aligns with:

- [Celery's worker architecture](https://docs.celeryq.dev/en/stable/userguide/workers.html) - separate worker processes
- [Asynq's server/client model](https://github.com/hibiken/asynq) - dedicated worker servers
- [Kubernetes best practices](https://kubernetes.io/docs/concepts/workloads/) - single-responsibility pods

### Why Separate Deployments?

| Factor | Embedded (Rejected) | Separate (Adopted) |
|--------|--------------------|--------------------|
| **Scaling** | Coupled (API + workers scale together) | Independent (scale each based on its metrics) |
| **Resource Efficiency** | Poor (mixed workload profiles) | Excellent (right-sized pods) |
| **Fault Isolation** | Poor (worker OOM kills API) | Excellent (isolated failure domains) |
| **Log Analysis** | Hard (mixed logs) | Easy (per-component streams) |
| **Incident Response** | 25+ min MTTR | 5-10 min MTTR |
| **Alerting** | Complex thresholds | Clear per-workload thresholds |
| **Cognitive Load** | High (mixed concerns) | Low (clear responsibilities) |

### Single Binary, Two Modes

The same agent binary runs in different modes based on `GOMIND_MODE` environment variable:

```go
// main.go - Single binary, mode-based execution
func main() {
    mode := os.Getenv("GOMIND_MODE") // "api", "worker", or "" (embedded)

    // Create agent with async task support
    agent := NewResearchAgent()
    orchestrator := orchestration.NewAIOrchestrator(config)

    // Create task infrastructure
    redisClient := redis.NewClient(&redis.Options{Addr: redisURL})
    taskQueue := orchestration.NewRedisTaskQueue(redisClient, queueConfig)
    taskStore := orchestration.NewRedisTaskStore(redisClient, storeConfig)

    // Create worker with orchestration handler
    worker := orchestration.NewTaskWorker(taskQueue, taskStore, workerConfig)
    worker.RegisterHandler("orchestration",
        orchestration.NewOrchestrationTaskHandler(orchestrator))

    switch mode {
    case "api":
        // API mode: HTTP server only, enqueues tasks
        // Workers run in separate pods
        framework := core.NewFramework(agent,
            core.WithName("research-agent-api"),
            core.WithPort(8090),
            core.WithTaskQueue(taskQueue),  // For enqueuing
            core.WithTaskStore(taskStore),  // For status queries
        )
        framework.Run(ctx)

    case "worker":
        // Worker mode: Process tasks only, minimal HTTP (just /health)
        // API runs in separate pods
        log.Println("Starting worker mode...")
        worker.Start(ctx)  // Blocks, processing tasks

    default:
        // Embedded mode: Both API and workers (for local development)
        go worker.Start(ctx)
        framework := core.NewFramework(agent,
            core.WithName("research-agent"),
            core.WithPort(8090),
            core.WithTaskQueue(taskQueue),
            core.WithTaskStore(taskStore),
        )
        framework.Run(ctx)
    }
}
```

### Deployment Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        Production Deployment                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────┐     ┌─────────────────────────────┐        │
│  │ research-agent-api          │     │ research-agent-worker       │        │
│  │ (Deployment)                │     │ (Deployment)                │        │
│  ├─────────────────────────────┤     ├─────────────────────────────┤        │
│  │ GOMIND_MODE=api             │     │ GOMIND_MODE=worker          │        │
│  │ replicas: 2-10 (HPA)        │     │ replicas: 1-20 (HPA)        │        │
│  │ cpu: 100m-500m              │     │ cpu: 500m-2000m             │        │
│  │ memory: 128Mi-512Mi         │     │ memory: 512Mi-4Gi           │        │
│  ├─────────────────────────────┤     ├─────────────────────────────┤        │
│  │ Endpoints:                  │     │ Endpoints:                  │        │
│  │ • POST /api/v1/tasks        │     │ • GET /health               │        │
│  │ • GET /api/v1/tasks/:id/*   │     │                             │        │
│  │ • POST /api/capabilities/*  │     │ Processing:                 │        │
│  │ • GET /health               │     │ • BRPOP from Redis queue    │        │
│  │                             │     │ • Execute orchestration     │        │
│  │ Scaling trigger:            │     │ • Update task store         │        │
│  │ • HTTP request rate         │     │                             │        │
│  │ • CPU utilization           │     │ Scaling trigger:            │        │
│  │                             │     │ • Redis queue depth         │        │
│  └──────────────┬──────────────┘     └──────────────┬──────────────┘        │
│                 │                                   │                        │
│                 │         ┌─────────────────┐       │                        │
│                 └────────>│  Redis          │<──────┘                        │
│                           │  • Task Queue   │                                │
│                           │  • Task Store   │                                │
│                           │  • Discovery    │                                │
│                           └─────────────────┘                                │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Local Development: Embedded Mode

For local development, run without `GOMIND_MODE` to get embedded behavior:

```bash
# Local development - embedded mode (simple)
go run main.go

# Or explicitly
GOMIND_MODE= go run main.go
```

This starts both HTTP server and workers in the same process - simple for development, not recommended for production.

### Cognitive Load Benefits

**For Developers:**
| Task | Embedded (Complex) | Separate (Simple) |
|------|-------------------|-------------------|
| "Where are task logs?" | Mixed with HTTP logs | `kubectl logs deploy/research-agent-worker` |
| "Why is my task slow?" | Could be HTTP, could be worker | Check worker metrics/logs only |
| "Test just the worker" | Can't isolate easily | `GOMIND_MODE=worker go test` |

**For Ops Engineers:**
| Task | Embedded (Complex) | Separate (Simple) |
|------|-------------------|-------------------|
| "Scale for more tasks" | Scales HTTP too (waste) | Scale workers only |
| "Worker OOM" | Takes down API (outage) | API unaffected |
| "Set CPU alert" | What threshold? Mixed workload | Clear: API=30%, Worker=80% |
| "3 AM incident" | 25+ min to diagnose | 5 min - check which deployment |

### Summary

| Question | Answer |
|----------|--------|
| **How do I run locally?** | `go run main.go` (embedded mode, simple) |
| **How do I deploy to production?** | Two deployments: `GOMIND_MODE=api` and `GOMIND_MODE=worker` |
| **Same binary?** | Yes, same image, different env var |
| **Can I use embedded in production?** | Possible but not recommended (coupling issues) |
| **How do workers access orchestrator?** | Embedded in worker binary (same as API mode) |
| **Independent scaling?** | Yes, each deployment has its own HPA |

---

## Implementation Roadmap

### v1 MVP (~2,400 lines)

> **Goal**: Solve the timeout problem with minimal implementation

**Deliverables:**
- [x] Define interfaces in `core/async_task.go`
- [x] Implement `RedisTaskQueue` in `orchestration/`
- [x] Implement `RedisTaskStore` in `orchestration/` (with Cancel support)
- [x] Basic worker pool implementation (with cancellation context propagation)
- [x] Task HTTP endpoints (submit, status, result, cancel)
- [x] Progress reporting interface (optional for handlers to use)
- [x] Trace context propagation (submit → store → worker restoration)
- [x] Task lifecycle span events and Prometheus metrics
- [x] **Step completion callback** (`OnStepComplete` in `ExecutionOptions`) for per-tool progress in async orchestration workflows
- [ ] Unit tests (using Redis in Kind cluster)

### Detailed Implementation Specifications

This section provides unambiguous specifications for what to change, where to change it, and what code to add.

---

#### Phase 0: Prerequisite - Telemetry Module Enhancement ✅ IMPLEMENTED

**File:** `telemetry/async_span.go`

**Status:** ✅ Implemented - `StartLinkedSpan()` function exists and is used by task worker.

**Purpose:** Add `StartLinkedSpan()` function for creating spans linked to stored trace context.

**Code to Add:**

```go
// telemetry/async_span.go
package telemetry

import (
    "context"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

// StartLinkedSpan creates a span linked to a stored trace context.
// Used for async workers restoring trace continuity from persistent storage.
//
// Parameters:
//   - ctx: Base context (typically context.Background() for workers)
//   - name: Span name (e.g., "task.process")
//   - traceID: W3C trace ID (32 hex chars) from stored task
//   - parentSpanID: Span ID (16 hex chars) from stored task
//   - attributes: Key-value pairs to attach to span
//
// Returns:
//   - context.Context with span attached
//   - func() to call when span completes (defer endSpan())
//
// Usage:
//
//	ctx, endSpan := telemetry.StartLinkedSpan(
//	    context.Background(),
//	    "task.process",
//	    task.TraceID,
//	    task.ParentSpanID,
//	    map[string]string{"task.id": task.ID},
//	)
//	defer endSpan()
func StartLinkedSpan(
    ctx context.Context,
    name string,
    traceID string,
    parentSpanID string,
    attributes map[string]string,
) (context.Context, func()) {
    tracer := otel.Tracer("gomind-telemetry")

    // Build span options
    opts := []trace.SpanStartOption{}

    // Add link to parent if trace context is valid
    if traceID != "" && parentSpanID != "" {
        tid, tidErr := trace.TraceIDFromHex(traceID)
        sid, sidErr := trace.SpanIDFromHex(parentSpanID)

        if tidErr == nil && sidErr == nil {
            parentSC := trace.NewSpanContext(trace.SpanContextConfig{
                TraceID: tid,
                SpanID:  sid,
                Remote:  true,
            })
            opts = append(opts, trace.WithLinks(trace.Link{
                SpanContext: parentSC,
                Attributes: []attribute.KeyValue{
                    attribute.String("link.type", "async_task"),
                },
            }))
        }
    }

    // Start span
    ctx, span := tracer.Start(ctx, name, opts...)

    // Add attributes
    for k, v := range attributes {
        span.SetAttributes(attribute.String(k, v))
    }

    return ctx, func() { span.End() }
}
```

**Verification:** Run `go build ./telemetry/...` after adding.

---

#### Phase 1: Core Interfaces ✅ IMPLEMENTED

**File:** `core/async_task.go`

**Purpose:** Define all async task interfaces and types.

**Code:** See [Core Interfaces (v1)](#core-interfaces-v1) section for complete code (~180 lines).

**Key Components:**

| Type | Description |
|------|-------------|
| `TaskStatus` | Enum: `queued`, `running`, `completed`, `failed`, `cancelled` |
| `Task` | Main struct with `TraceID`, `ParentSpanID` for trace continuity |
| `TaskProgress` | Progress tracking struct |
| `TaskQueue` | Interface: `Enqueue()`, `Dequeue()`, `Acknowledge()`, `Reject()` |
| `TaskStore` | Interface: `Create()`, `Get()`, `Update()`, `Delete()`, `Cancel()` |
| `TaskWorker` | Interface: `Start()`, `Stop()`, `RegisterHandler()` |
| `ProgressReporter` | Interface: `Report()` |
| `AsyncTaskConfig` | Configuration struct with defaults |

**Verification:** Run `go build ./core/...` after adding.

---

#### Phase 2: Redis Implementations ✅ IMPLEMENTED

**File:** `orchestration/redis_task_queue.go`

**Purpose:** Redis-backed task queue using LPUSH/BRPOP.

**Key Functions:**

```go
// RedisTaskQueue implements core.TaskQueue using Redis lists
type RedisTaskQueue struct {
    client *redis.Client
    config RedisTaskQueueConfig
    logger core.Logger
}

// RedisTaskQueueConfig with optional dependencies
type RedisTaskQueueConfig struct {
    QueueKey       string              `json:"queue_key"`
    CircuitBreaker core.CircuitBreaker `json:"-"` // Optional Layer 2 resilience
    Logger         core.Logger         `json:"-"`
}

func NewRedisTaskQueue(client *redis.Client, config *RedisTaskQueueConfig) *RedisTaskQueue
func (q *RedisTaskQueue) Enqueue(ctx context.Context, task *core.Task) error      // LPUSH
func (q *RedisTaskQueue) Dequeue(ctx context.Context, timeout time.Duration) (*core.Task, error)  // BRPOP
func (q *RedisTaskQueue) Acknowledge(ctx context.Context, taskID string) error
func (q *RedisTaskQueue) Reject(ctx context.Context, taskID string, reason string) error
```

---

**File:** `orchestration/redis_task_store.go`

**Purpose:** Redis-backed task state persistence using Redis hashes.

**Key Functions:**

```go
// RedisTaskStore implements core.TaskStore using Redis hashes
type RedisTaskStore struct {
    client *redis.Client
    config RedisTaskStoreConfig
    logger core.Logger
}

type RedisTaskStoreConfig struct {
    KeyPrefix string        `json:"key_prefix"`
    TTL       time.Duration `json:"ttl"`
    Logger    core.Logger   `json:"-"`
}

func NewRedisTaskStore(client *redis.Client, config *RedisTaskStoreConfig) *RedisTaskStore
func (s *RedisTaskStore) Create(ctx context.Context, task *core.Task) error
func (s *RedisTaskStore) Get(ctx context.Context, taskID string) (*core.Task, error)
func (s *RedisTaskStore) Update(ctx context.Context, task *core.Task) error
func (s *RedisTaskStore) Delete(ctx context.Context, taskID string) error
func (s *RedisTaskStore) Cancel(ctx context.Context, taskID string) error
```

**Redis Key Pattern:** `{prefix}:task:{task_id}`

---

#### Phase 3: Worker Implementation ✅ IMPLEMENTED

**File:** `orchestration/task_worker.go`

**Purpose:** Background worker pool that processes tasks with trace context restoration.

**Key Functions:**

```go
type TaskWorkerPool struct {
    queue     core.TaskQueue
    store     core.TaskStore
    handlers  map[string]core.TaskHandler
    config    TaskWorkerConfig
    telemetry core.Telemetry
    logger    core.Logger
    cancel    context.CancelFunc
    wg        sync.WaitGroup
}

type TaskWorkerConfig struct {
    WorkerCount     int           `json:"worker_count"`
    DequeueTimeout  time.Duration `json:"dequeue_timeout"`
    ShutdownTimeout time.Duration `json:"shutdown_timeout"`
    Telemetry       core.Telemetry `json:"-"`
    Logger          core.Logger    `json:"-"`
}

func NewTaskWorkerPool(queue core.TaskQueue, store core.TaskStore, config *TaskWorkerConfig) *TaskWorkerPool
func (p *TaskWorkerPool) Start(ctx context.Context) error
func (p *TaskWorkerPool) Stop(ctx context.Context) error
func (p *TaskWorkerPool) RegisterHandler(taskType string, handler core.TaskHandler) error
func (p *TaskWorkerPool) processTask(task *core.Task) error  // Uses telemetry.StartLinkedSpan()
```

**Imports Required:**

```go
import (
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
    "go.opentelemetry.io/otel/attribute"
)
```

---

#### Phase 4: HTTP API ✅ IMPLEMENTED

**File:** `orchestration/task_api.go`

**Purpose:** HTTP endpoints for task submission, status, result, and cancellation.

**Key Functions:**

```go
type TaskAPIHandler struct {
    queue  core.TaskQueue
    store  core.TaskStore
    logger core.Logger
}

func NewTaskAPIHandler(queue core.TaskQueue, store core.TaskStore, logger core.Logger) *TaskAPIHandler
func (h *TaskAPIHandler) HandleSubmit(w http.ResponseWriter, r *http.Request)   // POST /api/v1/tasks
func (h *TaskAPIHandler) HandleStatus(w http.ResponseWriter, r *http.Request)   // GET /api/v1/tasks/:id/status
func (h *TaskAPIHandler) HandleResult(w http.ResponseWriter, r *http.Request)   // GET /api/v1/tasks/:id/result
func (h *TaskAPIHandler) HandleCancel(w http.ResponseWriter, r *http.Request)   // POST /api/v1/tasks/:id/cancel
func (h *TaskAPIHandler) RegisterRoutes(mux *http.ServeMux)
```

**Telemetry:** `HandleSubmit` extracts trace context using `telemetry.GetTraceContext(ctx)` and stores in task.

---

#### Phase 5: Telemetry Helpers ✅ IMPLEMENTED

**File:** `orchestration/task_telemetry.go`

**Purpose:** Centralized task metrics and span event helpers.

**Key Functions:**

```go
// EmitTaskSubmitted emits task.submitted metric and span event
func EmitTaskSubmitted(ctx context.Context, task *core.Task) {
    telemetry.Counter("gomind.tasks.submitted",
        "task_type", task.Type,
    )
    telemetry.AddSpanEvent(ctx, "task.submitted",
        attribute.String("task_id", task.ID),
        attribute.String("task_type", task.Type),
    )
}

// EmitTaskCompleted emits task.completed metric and span event
func EmitTaskCompleted(ctx context.Context, task *core.Task, duration time.Duration) {
    telemetry.Counter("gomind.tasks.completed",
        "task_type", task.Type,
        "status", "completed",
    )
    telemetry.Histogram("gomind.tasks.duration_ms", float64(duration.Milliseconds()),
        "task_type", task.Type,
    )
    telemetry.AddSpanEvent(ctx, "task.completed",
        attribute.String("task_id", task.ID),
        attribute.Int64("duration_ms", duration.Milliseconds()),
    )
}

// EmitTaskFailed, EmitTaskCancelled, EmitQueueDepth, etc.
```

---

#### Phase 6: Step Completion Callback for Async Orchestration ✅ IMPLEMENTED

**Status:** ✅ Implemented - `OnStepComplete` callback added to `ExecutionOptions` and invoked by `SmartExecutor`.

**Problem:** The v1 async task system provides `ProgressReporter` for manual progress updates, but there's no built-in way to get **automatic per-step progress** from the `SmartExecutor`. When an async task uses AI orchestration with multiple tool calls:

1. **Current State**: Handler must manually call `reporter.Report()` - but it has no visibility into executor's step-by-step progress
2. **Gap**: `SmartExecutor.Execute()` runs multiple steps but provides no callback mechanism to notify the handler as each step completes
3. **Result**: Async tasks using orchestration can only report "started" and "completed" - no per-tool granularity

**Solution:** Add `OnStepComplete` callback to `ExecutionOptions` so the executor notifies callers after each step completes. This bridges the gap between the orchestration module and the async task system.

---

**File:** `orchestration/interfaces.go` (MODIFY)

**Purpose:** Add callback type and field to `ExecutionOptions`.

**Code to Add:**

```go
// StepCompleteCallback is called after each step in a routing plan completes.
// This enables real-time progress reporting for async workflows that use
// AI orchestration with multiple tool calls.
//
// The callback is invoked from within the executor goroutine after each step
// completes (success or failure). It should be lightweight or delegate to a
// channel for async processing to avoid blocking execution.
//
// Parameters:
//   - stepIndex: 0-based index of the completed step
//   - totalSteps: total number of steps in the plan
//   - step: the step that completed (contains AgentName, StepID, etc.)
//   - result: the step execution result (contains Success, Duration, Response, etc.)
//
// Usage with async tasks:
//
//	config.ExecutionOptions.OnStepComplete = func(stepIndex, totalSteps int, step RoutingStep, result StepResult) {
//	    reporter.Report(&core.TaskProgress{
//	        CurrentStep: stepIndex + 1,
//	        TotalSteps:  totalSteps,
//	        StepName:    step.AgentName,
//	        Percentage:  float64(stepIndex+1) / float64(totalSteps) * 100,
//	        Message:     fmt.Sprintf("Completed %s", step.AgentName),
//	    })
//	}
type StepCompleteCallback func(stepIndex, totalSteps int, step RoutingStep, result StepResult)

// In ExecutionOptions struct, add:
//
//     // Step completion callback for progress reporting (v1 addition)
//     // Called after each step completes (success or failure).
//     // Used by async task handlers to report per-tool progress.
//     OnStepComplete StepCompleteCallback `json:"-"` // Not serializable
```

---

**File:** `orchestration/executor.go` (MODIFY)

**Purpose:** Add callback field to `SmartExecutor` and invoke after step completion.

**Code to Add:**

```go
// In SmartExecutor struct, add field:
type SmartExecutor struct {
    // ... existing fields ...

    // Step completion callback for async progress reporting (v1 addition)
    onStepComplete StepCompleteCallback
}

// Add setter method:

// SetOnStepComplete sets the callback for step completion notifications.
// Used by async task handlers to receive per-step progress updates.
func (e *SmartExecutor) SetOnStepComplete(cb StepCompleteCallback) {
    e.onStepComplete = cb
}

// In Execute() method, after storing step result (around line 418-425):
// Add callback invocation AFTER releasing the mutex:

// Store result
resultsMutex.Lock()
stepResults[s.StepID] = &stepResult
result.Steps = append(result.Steps, stepResult)
executed[s.StepID] = true
stepIndex := len(result.Steps) - 1 // Capture index while holding lock

if !stepResult.Success {
    result.Success = false
}
resultsMutex.Unlock()

// Invoke step completion callback (outside lock to avoid blocking)
if e.onStepComplete != nil {
    e.onStepComplete(stepIndex, len(plan.Steps), s, stepResult)
}
```

---

**Integration with Async Tasks:**

When an async task handler uses AI orchestration, it can now set the callback to bridge to `ProgressReporter`:

```go
// In examples/agent-with-async/handlers.go
func (a *AsyncAgent) HandleQuery(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
    query := task.Input["query"].(string)

    // Report planning phase
    reporter.Report(&core.TaskProgress{
        CurrentStep: 1,
        TotalSteps:  2,
        StepName:    "Planning",
        Percentage:  5,
        Message:     "AI is analyzing request...",
    })

    // Configure orchestrator with step callback
    config := orchestration.DefaultConfig()
    config.ExecutionOptions.OnStepComplete = func(stepIndex, totalSteps int, step orchestration.RoutingStep, result orchestration.StepResult) {
        status := "completed"
        if !result.Success {
            status = "failed"
        }
        reporter.Report(&core.TaskProgress{
            CurrentStep: stepIndex + 2, // +1 for planning, +1 for 1-based
            TotalSteps:  totalSteps + 2, // +2 for planning and synthesis
            StepName:    fmt.Sprintf("%s: %s", status, step.AgentName),
            Percentage:  10 + float64(stepIndex+1)/float64(totalSteps)*85,
            Message:     fmt.Sprintf("Tool %d/%d %s", stepIndex+1, totalSteps, status),
        })
    }

    // Execute with callback-enabled config
    response, err := a.orchestrator.ProcessRequestWithConfig(ctx, query, config, nil)
    // ...
}
```

**Result in Task Status API:**

```json
{
  "task_id": "abc-123",
  "status": "running",
  "progress": {
    "current_step": 3,
    "total_steps": 6,
    "step_name": "completed: weather-tool-v2",
    "percentage": 50,
    "message": "Tool 2/4 completed"
  }
}
```

---

**Verification:**
- Run `go build ./orchestration/...` after adding
- Existing tests should pass (callback is optional/nil by default)

---

### Implementation Order

| Step | File | Depends On | Verification | Status |
|------|------|------------|--------------|--------|
| 0 | `telemetry/async_span.go` | None | `go build ./telemetry/...` | ✅ Done |
| 1 | `core/async_task.go` | None | `go build ./core/...` | ✅ Done |
| 2 | `orchestration/redis_task_queue.go` | Step 1 | `go build ./orchestration/...` | ✅ Done |
| 3 | `orchestration/redis_task_store.go` | Step 1 | `go build ./orchestration/...` | ✅ Done |
| 4 | `orchestration/task_telemetry.go` | Step 0 | `go build ./orchestration/...` | ✅ Done |
| 5 | `orchestration/task_worker.go` | Steps 0,1,2,3,4 | `go build ./orchestration/...` | ✅ Done |
| 6 | `orchestration/task_api.go` | Steps 1,2,3 | `go build ./orchestration/...` | ✅ Done |
| 6.1 | `orchestration/interfaces.go` | None | `go build ./orchestration/...` | ✅ Done |
| 6.2 | `orchestration/executor.go` | Step 6.1 | `go build ./orchestration/...` | ✅ Done |
| 7 | `orchestration/*_test.go` | All above | `go test ./orchestration/...` | Pending |

---

### Files Summary

```
telemetry/
├── async_span.go          # StartLinkedSpan() function (199 lines)
├── async_span_test.go     # Tests (223 lines)

core/
├── async_task.go          # Interfaces and types (372 lines)
├── async_task_test.go     # Tests (321 lines)

orchestration/
├── redis_task_queue.go    # Redis queue implementation (291 lines)
├── redis_task_store.go    # Redis store implementation (388 lines)
├── task_worker.go         # Worker pool with trace restoration (513 lines)
├── task_api.go            # HTTP endpoints (412 lines)
├── task_telemetry.go      # Metrics and span event helpers (220 lines)
├── task_worker_test.go    # Worker tests (568 lines)
├── task_api_test.go       # API tests (511 lines)
├── task_telemetry_test.go # Telemetry tests (199 lines)
```

**Total Implementation:** ~2,400 lines | **Total Tests:** ~1,800 lines

---

**What's NOT in v1:**
- ❌ SSE streaming (use polling instead)
- ❌ Checkpointing (see [v2+ Future Enhancements](#future-enhancements-v2))
- ❌ Retry with backoff (use existing resilience module)
- ❌ Webhooks (use polling)
- ❌ Priority queues (all tasks equal priority)
- ❌ SmartExecutor Auto-Progress (see [v2+ Future Enhancements](#future-enhancements-v2))

---

## Future Enhancements (v2+)

> **Add based on user feedback** - don't build until needed

### SSE Streaming (~100 lines)
**Add when**: Users request real-time progress in browsers

```
orchestration/
├── task_sse.go            # NEW: SSE handler
```

### Retry with Backoff (~80 lines)
**Add when**: Users report transient failures in task execution

```
orchestration/
├── task_retry.go          # NEW: Retry policies
```

### ~~Task Metrics~~ (Moved to v1)
> **Note**: Task metrics are now part of v1. See [Telemetry Integration](#telemetry-integration-v1---built-in).

### Checkpointing (~250 lines)
**Add when**: Users have long-running tasks (10+ minutes) where failure recovery matters

#### Problem
Without checkpointing, a task that fails at minute 25 of a 30-minute workflow must restart from scratch—wasting LLM tokens ($$$), time, and user patience.

#### Solution Summary
- Save task state after each significant step to Redis
- On retry, restore state and resume from last checkpoint
- Auto-cleanup checkpoint on successful completion

#### Key Components
| Component | Purpose |
|-----------|---------|
| `Checkpoint` struct | Stores `TaskID`, `LastCompletedStep`, `State` (intermediate data) |
| `CheckpointStore` interface | `Save()`, `Get()`, `Delete()` |
| `CheckpointableProgressReporter` | Extends `ProgressReporter` with `GetCheckpoint()`, `SaveCheckpoint()` |
| `RedisCheckpointStore` | Redis implementation with TTL-based cleanup |

#### Handler Pattern
```go
func handler(ctx, task, reporter) error {
    checkpointReporter := reporter.(CheckpointableProgressReporter)

    // Resume from checkpoint if exists
    if cp := checkpointReporter.GetCheckpoint(); cp != nil {
        startStep = cp.LastCompletedStep + 1
        state = cp.State
    }

    for step := startStep; step <= total; step++ {
        result := executeStep(step)
        state = merge(state, result)

        // Save after each expensive step
        checkpointReporter.SaveCheckpoint(&Checkpoint{
            LastCompletedStep: step,
            State: state,
        })
    }
    return nil
}
```

#### Files
```
core/async_task.go              # Add Checkpoint, CheckpointStore, CheckpointableProgressReporter
orchestration/redis_checkpoint_store.go  # NEW: Redis implementation
orchestration/task_worker.go    # Integrate checkpointable reporter
```

### SmartExecutor Auto-Progress (~190 lines)
**Add when**: Users want automatic progress reporting without manual `reporter.Report()` calls

#### Problem
Manual progress reporting is tedious—developers must add `reporter.Report()` calls after each step. This leads to either:
- No progress reporting (poor UX)
- Boilerplate code in every handler

#### Solution Summary
- Hook into `SmartExecutor.Execute()` to auto-report after each plan step
- Calculate percentage from `completedSteps / totalSteps`
- Works for both sequential and parallel execution
- Zero code changes in existing handlers

#### Key Components
| Component | Purpose |
|-----------|---------|
| `SmartExecutor.progressReporter` | Optional field to hold reporter reference |
| `SetProgressReporter(ProgressReporter)` | Method to inject reporter |
| `ProcessRequestWithProgress()` | Wrapper that auto-reports after each step |
| `atomic.Int32` counter | Thread-safe step tracking for parallel execution |

#### Usage Pattern
```go
// In task worker handler
func orchestrationHandler(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
    executor := orchestration.NewSmartExecutor(...)
    executor.SetProgressReporter(reporter)  // Enable auto-progress

    // Auto-reports: "Step 1/5: get_weather (20%)", "Step 2/5: search_news (40%)", etc.
    result, err := executor.ProcessRequestWithProgress(ctx, query)
    return err
}
```

#### What Users See
```json
{"task_id":"abc123","status":"running","progress":{"current_step":1,"total_steps":5,"step_name":"get_weather","percentage":20}}
{"task_id":"abc123","status":"running","progress":{"current_step":2,"total_steps":5,"step_name":"search_news","percentage":40}}
```

#### Files
```
orchestration/smart_executor.go  # Add progressReporter field, SetProgressReporter(), ProcessRequestWithProgress()
```

> **Developer choice**: For custom progress messages, use manual `reporter.Report()` instead

---

## v2 Breaking Changes

> **Important**: v2 features will require modifications to v1 structs. This section documents the expected changes to help with planning.

### Task Struct Changes

v2 features require additional fields in the `Task` struct:

```go
// Task struct - v2 additions (will modify v1 struct)
type Task struct {
    // ... existing v1 fields ...

    // v2: Checkpointing
    Checkpoint map[string]interface{} `json:"checkpoint,omitempty"`

    // v2: Retry
    RetryCount int `json:"retry_count,omitempty"`
    MaxRetries int `json:"max_retries,omitempty"`
}
```

### TaskOptions Changes

v2 retry feature requires additional configuration:

```go
// TaskOptions - v2 additions
type TaskOptions struct {
    Timeout time.Duration `json:"timeout"` // v1

    // v2: Retry configuration
    RetryPolicy *RetryPolicy `json:"retry_policy,omitempty"`
}

// RetryPolicy - NEW in v2
type RetryPolicy struct {
    MaxRetries     int           `json:"max_retries"`
    InitialBackoff time.Duration `json:"initial_backoff"`
    MaxBackoff     time.Duration `json:"max_backoff"`
    BackoffFactor  float64       `json:"backoff_factor"`
}
```

### New Interfaces in v2

```go
// TaskEventPublisher - NEW in v2 for SSE streaming
type TaskEventPublisher interface {
    // Publish sends a task event to subscribers
    Publish(ctx context.Context, taskID string, event TaskEvent) error

    // Subscribe returns a channel of events for a task
    Subscribe(ctx context.Context, taskID string) (<-chan TaskEvent, error)

    // Unsubscribe removes a subscription
    Unsubscribe(ctx context.Context, taskID string, ch <-chan TaskEvent) error
}

// TaskEvent - NEW in v2
type TaskEvent struct {
    Type      string      `json:"type"` // "progress", "completed", "failed", "cancelled"
    Timestamp time.Time   `json:"timestamp"`
    Data      interface{} `json:"data,omitempty"`
}
```

### Migration Path

1. **v1 → v2 Upgrade**: Add new fields with `omitempty` tags - existing v1 data remains valid
2. **No data migration required**: New fields are optional and nil/zero by default
3. **Interface additions**: New interfaces (like `TaskEventPublisher`) are separate - existing implementations remain compatible

### Summary of v2 Changes

| Component | Change Type | Impact |
|-----------|-------------|--------|
| `Task` struct | Add fields | Low - `omitempty` tags |
| `TaskOptions` struct | Add fields | Low - `omitempty` tags |
| `TaskEventPublisher` | New interface | None - separate interface |
| `TaskEvent` | New type | None - new type |
| `RetryPolicy` | New type | None - new type |

**Bottom line**: v2 changes are additive. Existing v1 code will continue to work, but consumers will need to update their struct definitions when upgrading to v2.

---

## Comparison Summary

| Approach | Pros | Cons | Best For |
|----------|------|------|----------|
| **Sync (Current)** | Simple, immediate response | Timeouts, blocked connections | Quick tasks (<60s) |
| **Polling (202)** | Simple client, stateless, universal | Wasted requests, latency | Most use cases |
| **SSE Streaming** | Real-time progress, browser native | Connection held open | Browser clients |
| **Webhooks** | No polling, real-time | Client needs endpoint | Service-to-service |
| **Queue + Workers** | Scalable, reliable, recoverable | More infrastructure | Production systems |

### Recommended Combination

```
┌─────────────────────────────────────────────────────────────┐
│                  Recommended Architecture                    │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Primary:     HTTP 202 + Status Polling                     │
│               (Universal, works everywhere)                  │
│                                                              │
│  Enhancement: SSE Streaming                                  │
│               (Real-time progress for browsers)              │
│                                                              │
│  Backend:     Redis Queue + Worker Pool                      │
│               (Scalable, reliable, K8s-native)              │
│                                                              │
│  Recovery:    Checkpointing                                  │
│               (Resume long workflows after failure)          │
│                                                              │
│  Optional:    Webhooks                                       │
│               (Service-to-service notifications)             │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Developer Impact

This section clarifies how the async task architecture affects developers writing tools and agents in GoMind.

### Key Principle: Additive, Not Breaking

The async task architecture is an **additive layer** on top of the existing system. It does **not** replace or break existing patterns:

```
┌─────────────────────────────────────────┐
│         NEW: Async Task Layer           │  ← Optional for long tasks
│  (Submit → Queue → Worker → Poll/SSE)   │
├─────────────────────────────────────────┤
│      UNCHANGED: Orchestration Layer     │  ← SmartExecutor, Router
│    (AI planning, step execution)        │
├─────────────────────────────────────────┤
│      UNCHANGED: Tool/Agent Layer        │  ← Your tools and agents
│    (Capabilities, HTTP handlers)        │
├─────────────────────────────────────────┤
│      UNCHANGED: Core Layer              │  ← Discovery, interfaces
│    (Registry, BaseAgent, BaseTool)      │
└─────────────────────────────────────────┘
```

### Impact by Component

| Component | Changes Required | Impact Level |
|-----------|------------------|--------------|
| **Existing Tools** | None | Zero impact |
| **Existing Agents** | None | Zero impact |
| **Existing Sync Endpoints** | None | Continue to work |
| **Long-Running Workflows** | Use new Task API | New capability |
| **Client Code** | Choose sync or async | Minor adaptation |

---

### For Tool Developers: No Changes Required

Tools remain exactly the same. They are passive components that:
- Register capabilities
- Handle individual HTTP requests
- Return responses synchronously

```go
// UNCHANGED - Tools work exactly as before
type WeatherTool struct {
    *core.BaseTool
}

func NewWeatherTool() *WeatherTool {
    tool := &WeatherTool{
        BaseTool: core.NewTool("weather"),
    }

    tool.RegisterCapability(core.Capability{
        Name:        "get_weather",
        Description: "Get weather for a city",
        Handler:     tool.handleGetWeather,
    })

    return tool
}

func (t *WeatherTool) handleGetWeather(w http.ResponseWriter, r *http.Request) {
    // This still has a 30s timeout - that's fine for a single tool call
    city := r.URL.Query().Get("city")
    weather, err := t.fetchWeatherAPI(city)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(weather)
}
```

**Why no changes?** The async layer sits ABOVE tools. Tools are called by the orchestrator, and each tool call is still a quick HTTP request. The async system handles the orchestration of MULTIPLE tool calls over time.

---

### For Agent Developers: Two Modes

Agents have **two modes** after this change:

#### Mode 1: Synchronous (Existing - Unchanged)

For quick orchestrations (< 2 minutes), nothing changes:

```go
// UNCHANGED - Agent with sync capabilities
type SimpleAgent struct {
    *core.BaseAgent
}

func NewSimpleAgent() *SimpleAgent {
    agent := &SimpleAgent{
        BaseAgent: core.NewBaseAgent("simple-agent"),
    }

    // Register sync capability (existing pattern)
    agent.RegisterCapability(core.Capability{
        Name:        "quick_search",
        Description: "Quick search (< 30s)",
        Handler:     agent.handleQuickSearch,
    })

    return agent
}

// Client calls sync endpoint - works exactly as before
// POST /api/capabilities/quick_search
// Response returned immediately (within timeout)
```

#### Mode 2: Asynchronous (New - Opt-in)

For long-running orchestrations, agents can opt-in to async task support:

```go
// NEW - Agent that supports async tasks
type ResearchAgent struct {
    *core.BaseAgent
    taskStore  core.TaskStore   // NEW: Optional dependency
    taskQueue  core.TaskQueue   // NEW: Optional dependency
}

func NewResearchAgent(taskStore core.TaskStore, taskQueue core.TaskQueue) *ResearchAgent {
    agent := &ResearchAgent{
        BaseAgent:  core.NewBaseAgent("research-agent"),
        taskStore:  taskStore,
        taskQueue:  taskQueue,
    }

    // Register BOTH sync and async capabilities

    // Sync endpoint (existing pattern - for quick tasks)
    agent.RegisterCapability(core.Capability{
        Name:        "quick_lookup",
        Description: "Quick lookup (< 30s)",
        Handler:     agent.handleQuickLookup,
    })

    // Async task endpoints are registered automatically
    // when taskStore and taskQueue are provided:
    // - POST /api/v1/tasks
    // - GET  /api/v1/tasks/:id/status
    // - GET  /api/v1/tasks/:id/stream
    // - GET  /api/v1/tasks/:id/result
    // - POST /api/v1/tasks/:id/cancel

    return agent
}
```

---

### Client Code Comparison

#### Before (Synchronous Only)

```go
// Client code - must wait for response
resp, err := http.Post(
    "http://agent:8080/api/capabilities/research",
    "application/json",
    bytes.NewReader(requestBody),
)
// ⚠️ This blocks and may timeout after 30s (WriteTimeout) or 60s (HTTP client)

if err != nil {
    // Handle timeout or connection error
    return err
}

var result ResearchResult
json.NewDecoder(resp.Body).Decode(&result)
```

#### After (Async Option Available)

```go
// Client code - submit and poll (non-blocking)

// Step 1: Submit task (returns immediately ~10ms)
resp, _ := http.Post(
    "http://agent:8080/api/v1/tasks",
    "application/json",
    strings.NewReader(`{
        "type": "orchestration",
        "input": {"goal": "Research AI trends"},
        "options": {"timeout": "30m"}
    }`),
)

var task TaskResponse
json.NewDecoder(resp.Body).Decode(&task)
// task.TaskID = "task-abc123"
// task.Status = "queued"

// Step 2: Poll for completion (or use SSE for real-time)
for {
    resp, _ := http.Get(
        "http://agent:8080/api/v1/tasks/" + task.TaskID + "/status",
    )

    var status StatusResponse
    json.NewDecoder(resp.Body).Decode(&status)

    fmt.Printf("Progress: %s (%.0f%%)\n",
        status.Progress.StepName,
        status.Progress.Percentage)

    if status.Status == "completed" {
        // Get final result
        resp, _ := http.Get(
            "http://agent:8080/api/v1/tasks/" + task.TaskID + "/result",
        )
        var result TaskResult
        json.NewDecoder(resp.Body).Decode(&result)
        return result, nil
    }

    if status.Status == "failed" {
        return nil, errors.New(status.Error.Message)
    }

    // Use server-provided retry interval
    time.Sleep(time.Duration(status.RetryAfter) * time.Second)
}
```

#### Alternative: SSE Streaming (Real-time Progress)

```go
// Client code - stream progress in real-time
req, _ := http.NewRequest("GET",
    "http://agent:8080/api/v1/tasks/"+taskID+"/stream", nil)
req.Header.Set("Accept", "text/event-stream")

resp, _ := http.DefaultClient.Do(req)
defer resp.Body.Close()

reader := bufio.NewReader(resp.Body)
for {
    line, err := reader.ReadString('\n')
    if err == io.EOF {
        break
    }

    if strings.HasPrefix(line, "data: ") {
        data := strings.TrimPrefix(line, "data: ")
        var event TaskEvent
        json.Unmarshal([]byte(data), &event)

        switch event.Type {
        case "progress":
            fmt.Printf("Step %d/%d: %s\n",
                event.Progress.CurrentStep,
                event.Progress.TotalSteps,
                event.Progress.StepName)
        case "complete":
            fmt.Println("Task completed!")
            return
        case "error":
            fmt.Printf("Task failed: %s\n", event.Error)
            return
        }
    }
}
```

---

### Decision Matrix: When to Use What

| Scenario | Use Sync | Use Async | Why |
|----------|----------|-----------|-----|
| Single tool call | ✅ | ❌ | Quick, no overhead needed |
| 2-3 tool calls (< 1 min) | ✅ | ❌ | Still within timeout |
| Multi-step workflow (> 1 min) | ❌ | ✅ | May exceed sync timeout |
| Need progress updates | ❌ | ✅ | Sync has no progress |
| Human-in-the-loop | ❌ | ✅ | Unpredictable wait time |
| Browser client (simple) | ✅ | ❌ | Simpler implementation |
| Browser client (long task) | ❌ | ✅ | Use SSE for progress |
| Service-to-service | Either | ✅ | Webhook callbacks |
| Batch processing | ❌ | ✅ | Queue multiple tasks |
| Need task cancellation | ❌ | ✅ | Sync can't be cancelled |

---

### Migration Guide

#### Existing Projects: No Action Required

If your tools and agents work today, they will continue to work unchanged:

```go
// This code continues to work exactly as before
tool := weather.NewWeatherTool()
tool.Initialize(ctx, discovery, config)
tool.Start(ctx, 8080)

// Clients can still call:
// POST /api/capabilities/get_weather
```

#### Adding Async Support: Opt-In

To enable async tasks for an existing agent:

```go
// Step 1: Add task infrastructure (once, in main.go)
redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
taskQueue := orchestration.NewRedisTaskQueue(redisClient, config)
taskStore := orchestration.NewRedisTaskStore(redisClient, config)

// Step 2: Inject into agent
agent := NewResearchAgent(taskStore, taskQueue)

// Step 3: Register orchestration handler
worker := orchestration.NewTaskWorker(taskQueue, taskStore, config)
worker.RegisterHandler("orchestration",
    orchestration.NewOrchestrationTaskHandler(orchestrator))

// Step 4: Start worker pool (separate process or goroutine)
go worker.Start(ctx)
```

#### New Projects: Choose Your Pattern

```go
// Simple agent (sync only) - minimal setup
agent := simple.NewAgent()
agent.Start(ctx, 8080)

// Full-featured agent (sync + async) - with task support
agent := research.NewAgent(taskStore, taskQueue)
worker := orchestration.NewTaskWorker(...)
go worker.Start(ctx)
agent.Start(ctx, 8080)
```

#### Production Deployment: Use GOMIND_MODE

For production, use the [dual deployment pattern](#worker-deployment-architecture) with separate API and Worker deployments:

```go
// main.go - Single binary, mode-based execution
func main() {
    mode := os.Getenv("GOMIND_MODE") // "api", "worker", or "" (embedded)

    // Common setup
    agent := NewResearchAgent()
    taskQueue := orchestration.NewRedisTaskQueue(redisClient, config)
    taskStore := orchestration.NewRedisTaskStore(redisClient, config)
    worker := orchestration.NewTaskWorker(taskQueue, taskStore, config)

    switch mode {
    case "api":
        // Production: API-only mode
        framework := core.NewFramework(agent,
            core.WithTaskQueue(taskQueue),
            core.WithTaskStore(taskStore),
        )
        framework.Run(ctx)

    case "worker":
        // Production: Worker-only mode
        worker.Start(ctx)

    default:
        // Local development: Embedded mode
        go worker.Start(ctx)
        framework := core.NewFramework(agent,
            core.WithTaskQueue(taskQueue),
            core.WithTaskStore(taskStore),
        )
        framework.Run(ctx)
    }
}
```

**Kubernetes deployments**: Set `GOMIND_MODE=api` for API pods, `GOMIND_MODE=worker` for worker pods. See [Appendix B](#appendix-b-kubernetes-deployment) for complete YAML examples.

---

### Summary

| Question | Answer |
|----------|--------|
| Do I need to change my existing tools? | **No** |
| Do I need to change my existing agents? | **No** |
| Will my existing code break? | **No** |
| Is async mandatory? | **No**, it's opt-in |
| When should I use async? | Tasks > 1 minute, need progress, human-in-loop |
| How do I add async to existing agent? | Inject TaskStore + TaskQueue, register handler |

**Bottom line**: Write tools and agents exactly as you do today. The async layer is there when you need it for long-running workflows, but it doesn't force any changes to existing code.

---

## References

### Industry Patterns

1. [Asynchronous Request-Reply Pattern - Microsoft Azure Architecture](https://learn.microsoft.com/en-us/azure/architecture/patterns/async-request-reply)
2. [Managing Asynchronous Workflows with REST API - AWS Architecture Blog](https://aws.amazon.com/blogs/architecture/managing-asynchronous-workflows-with-a-rest-api/)
3. [REST API Design for Long-Running Tasks - RESTful API](https://restfulapi.net/rest-api-design-for-long-running-tasks/)
4. [Avoiding Long-Running HTTP API Requests - CodeOpinion](https://codeopinion.com/avoiding-long-running-http-api-requests/)
5. [Cloud Design Patterns for Long-Running Tasks - Telstra Purple](https://purple.telstra.com/blog/design-patterns-for-handling-long-running-tasks)

### AI Agent Orchestration

6. [AI Agent Design Patterns - Microsoft Azure](https://learn.microsoft.com/en-us/azure/architecture/ai-ml/guide/ai-agent-design-patterns)
7. [Orchestrating Multi-Step Agents - Kinde](https://kinde.com/learn/ai-for-software-engineering/ai-devops/orchestrating-multi-step-agents-patterns-for-long-running-work/)
8. [Workflow Orchestration Agents - AWS Prescriptive Guidance](https://docs.aws.amazon.com/prescriptive-guidance/latest/agentic-ai-patterns/workflow-orchestration-agents.html)
9. [Agentic AI with Temporal Orchestration - IntuitionLabs](https://intuitionlabs.ai/articles/agentic-ai-temporal-orchestration)

### Go Libraries

10. [Asynq - Distributed Task Queue in Go](https://github.com/hibiken/asynq)
11. [Machinery - Async Task Queue](https://github.com/RichardKnop/machinery)
12. [Taskq - Multi-backend Task Queue](https://github.com/vmihailenco/taskq)

### Timeout Strategies

13. [Timeout Strategies in Microservices - GeeksforGeeks](https://www.geeksforgeeks.org/timeout-strategies-in-microservices-architecture/)
14. [Timeout Pattern - Resilient Microservice Design](https://vinsguru.medium.com/resilient-microservice-design-with-spring-boot-timeout-pattern-72b5f5174d2a)

---

## Appendix A: Quick Start Example

Once implemented, using async tasks will be straightforward:

```go
// Submit a long-running task
resp, _ := http.Post("http://agent:8080/api/v1/tasks", "application/json",
    strings.NewReader(`{
        "type": "orchestration",
        "input": {"goal": "Research quantum computing trends"},
        "options": {"timeout": "30m"}
    }`))

var task TaskResponse
json.NewDecoder(resp.Body).Decode(&task)
// task.TaskID = "task-abc123"

// Poll for completion
for {
    resp, _ := http.Get("http://agent:8080/api/v1/tasks/" + task.TaskID + "/status")
    var status StatusResponse
    json.NewDecoder(resp.Body).Decode(&status)

    if status.Status == "completed" {
        // Get result
        resp, _ := http.Get("http://agent:8080/api/v1/tasks/" + task.TaskID + "/result")
        // Process result...
        break
    }

    time.Sleep(time.Duration(status.RetryAfter) * time.Second)
}
```

---

## Appendix B: Kubernetes Deployment

This section demonstrates the **dual deployment pattern** recommended in the [Worker Deployment Architecture](#worker-deployment-architecture) section. Both deployments use the same Docker image but run in different modes via the `GOMIND_MODE` environment variable.

### Complete Example: Research Agent with Async Tasks

```yaml
# =============================================================================
# API DEPLOYMENT - Handles HTTP requests, enqueues async tasks
# =============================================================================
apiVersion: apps/v1
kind: Deployment
metadata:
  name: research-agent-api
  labels:
    app: research-agent
    component: api
spec:
  replicas: 2
  selector:
    matchLabels:
      app: research-agent
      component: api
  template:
    metadata:
      labels:
        app: research-agent
        component: api
    spec:
      containers:
      - name: api
        image: research-agent:latest  # Same image as worker
        env:
        - name: GOMIND_MODE
          value: "api"                # API mode - HTTP server only
        - name: REDIS_URL
          value: "redis://redis:6379"
        - name: PORT
          value: "8090"
        ports:
        - containerPort: 8090
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
          limits:
            cpu: "500m"
            memory: "256Mi"
        readinessProbe:
          httpGet:
            path: /health
            port: 8090
          initialDelaySeconds: 5
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /health
            port: 8090
          initialDelaySeconds: 15
          periodSeconds: 20
---
# API Service - Exposes HTTP endpoints
apiVersion: v1
kind: Service
metadata:
  name: research-agent-service
spec:
  selector:
    app: research-agent
    component: api
  ports:
  - port: 80
    targetPort: 8090
---
# API HPA - Scales based on HTTP request rate / CPU
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: research-agent-api-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: research-agent-api
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
---
# =============================================================================
# WORKER DEPLOYMENT - Processes async tasks from queue
# =============================================================================
apiVersion: apps/v1
kind: Deployment
metadata:
  name: research-agent-worker
  labels:
    app: research-agent
    component: worker
spec:
  replicas: 3
  selector:
    matchLabels:
      app: research-agent
      component: worker
  template:
    metadata:
      labels:
        app: research-agent
        component: worker
    spec:
      containers:
      - name: worker
        image: research-agent:latest  # Same image as API
        env:
        - name: GOMIND_MODE
          value: "worker"             # Worker mode - process tasks only
        - name: REDIS_URL
          value: "redis://redis:6379"
        - name: WORKER_CONCURRENCY
          value: "5"
        ports:
        - containerPort: 8091         # Health check only
        resources:
          requests:
            cpu: "500m"               # Higher CPU for task processing
            memory: "512Mi"
          limits:
            cpu: "2000m"
            memory: "1Gi"
        readinessProbe:
          httpGet:
            path: /health
            port: 8091
          initialDelaySeconds: 5
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /health
            port: 8091
          initialDelaySeconds: 15
          periodSeconds: 20
---
# Worker HPA - Scales based on queue depth (external metric)
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: research-agent-worker-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: research-agent-worker
  minReplicas: 2
  maxReplicas: 20
  metrics:
  - type: External
    external:
      metric:
        name: redis_list_length
        selector:
          matchLabels:
            queue: gomind-tasks-research-agent
      target:
        type: AverageValue
        averageValue: "10"  # Scale up when > 10 tasks per worker
```

### Key Differences: API vs Worker

| Aspect | API Deployment | Worker Deployment |
|--------|----------------|-------------------|
| **GOMIND_MODE** | `api` | `worker` |
| **Purpose** | Handle HTTP requests | Process async tasks |
| **CPU Request** | 100m (lightweight) | 500m (compute-heavy) |
| **Memory Request** | 128Mi | 512Mi |
| **Scaling Metric** | CPU utilization | Queue depth |
| **External Traffic** | Yes (via Service) | No (internal only) |
| **Health Port** | 8090 (main port) | 8091 (dedicated health) |

### Prometheus ServiceMonitor (for queue-based HPA)

```yaml
# Required for queue-depth based autoscaling
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: redis-exporter
spec:
  selector:
    matchLabels:
      app: redis-exporter
  endpoints:
  - port: metrics
    interval: 15s
---
# PrometheusRule for queue length metric
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: gomind-task-queue-rules
spec:
  groups:
  - name: gomind.tasks
    rules:
    - record: gomind:task_queue:length
      expr: redis_list_length{key=~"gomind:tasks:.*"}
```
