# DAG Visualization Feature Proposal

A new view for the Registry Viewer App to display LLM-based plan generation and execution as an interactive DAG (Directed Acyclic Graph).

**Version:** 3.0 (Interface-First Design)
**Last Updated:** 2026-01-25
**Status:** Ready for Implementation

## Table of Contents

1. [Overview](#overview)
   1. [Purpose](#purpose)
   2. [Scope](#scope)
   3. [Design Principles Alignment](#design-principles-alignment)
2. [User Requirements](#user-requirements)
3. [Data Analysis](#data-analysis)
   1. [Available Data Structures](#available-data-structures)
      - [RoutingPlan (Plan Structure)](#1-routingplan-plan-structure)
      - [ExecutionResult (Step Execution Data)](#2-executionresult-step-execution-data)
      - [LLMDebugRecord (LLM Interactions)](#3-llmdebugrecord-llm-interactions)
      - [Existing Store Pattern (Reference)](#4-existing-store-pattern-reference)
   2. [Data Correlation](#data-correlation)
4. [Gap Analysis](#gap-analysis)
   1. [Current State](#current-state)
   2. [Gap: Plan + Execution Not Persisted](#gap-plan--execution-not-persisted)
   3. [Why Store Plan + Result Together?](#why-store-plan--result-together)
5. [Proposed Solution](#proposed-solution)
   1. [New Data Type: StoredExecution](#new-data-type-storedexecution)
   2. [New Interface: ExecutionStore](#new-interface-executionstore)
   3. [New Interface: StorageProvider](#new-interface-storageprovider)
   4. [Configuration: ExecutionStoreConfig](#configuration-executionstoreconfig)
   5. [Storage Key Patterns](#storage-key-patterns-configurable-via-keyprefix)
6. [Framework Changes](#framework-changes)
   1. [Files to Create in Framework](#files-to-create-in-framework-orchestration)
   2. [Core Module Update](#core-module-update)
   3. [Application-Level Files (Optional)](#application-level-files-optional)
   4. [Files to Modify](#files-to-modify)
      - [orchestration/interfaces.go](#1-orchestrationinterfacesgo)
      - [orchestration/orchestrator.go](#2-orchestrationorchestratorgo)
      - [orchestration/factory.go](#3-orchestrationfactorygo--implemented)
      - [orchestration/interfaces.go - DefaultConfig()](#4-orchestrationinterfacesgo---defaultconfig)
   5. [Environment Variables (Framework)](#environment-variables-framework)
   6. [Features (Same as LLM Debug Store)](#features-same-as-llm-debug-store)
7. [API Design](#api-design)
   1. [Registry Viewer Endpoints](#registry-viewer-endpoints)
      - [List Executions](#list-executions)
      - [Get Execution Details](#get-execution-details)
      - [Get DAG Structure (Computed)](#get-dag-structure-computed)
      - [Search](#search)
8. [UI Design](#ui-design)
   1. [Glassmorphism Theme Integration](#glassmorphism-theme-integration)
      - [CSS Variables](#css-variables-already-defined)
   2. [Layout](#layout)
   3. [Node Status Colors](#node-status-colors-theme-aligned)
   4. [Glassmorphism Component Styles](#glassmorphism-component-styles)
      - [DAG Container](#dag-container)
      - [Execution List Panel](#execution-list-panel)
      - [Search Box](#search-box)
      - [Step Details Panel](#step-details-panel)
      - [Status Badges](#status-badges-follow-existing-pattern)
   5. [Cytoscape.js Node Styling (Glassmorphism)](#cytoscapejs-node-styling-glassmorphism)
   6. [Interactive Features](#interactive-features)
   7. [Recommended Library: Cytoscape.js](#recommended-library-cytoscapejs)
9. [Implementation Phases](#implementation-phases)
   1. [Phase 1: Framework - Execution Debug Store](#phase-1-framework---execution-debug-store-orchestration-module--complete) ✅
   2. [Phase 2: Auto-Configuration](#phase-2-auto-configuration--implemented-no-application-code-needed) ✅
   3. [Phase 3: Registry Viewer API](#phase-3-registry-viewer-api)
   4. [Phase 4: DAG Visualization UI](#phase-4-dag-visualization-ui)
10. [Technical Considerations](#technical-considerations)
    1. [Performance](#performance)
    2. [Data Consistency](#data-consistency)
    3. [Resilience](#resilience)
    4. [Backwards Compatibility](#backwards-compatibility)
11. [Framework Compliance Checklist](#framework-compliance-checklist)
12. [Open Questions (Resolved)](#open-questions-resolved)
13. [Phase 5: Enhanced Full Execution Flow Visualization](#phase-5-enhanced-full-execution-flow-visualization) ✅
    1. [Motivation](#motivation)
    2. [Current State vs Proposed](#current-state-vs-proposed)
    3. [Node Types & Visual Design](#node-types--visual-design)
    4. [Data Gap Analysis](#data-gap-analysis)
    5. [Backend Changes Required](#backend-changes-required)
       - [Change 1: Add Name to OrchestratorConfig](#change-1-add-name-to-orchestratorconfig)
       - [Change 2: Add AgentName to StoredExecution](#change-2-add-agentname-to-storedexecution)
       - [Change 3: Update Orchestrator Storage Call](#change-3-update-orchestrator-storage-call)
       - [Change 4: Add Helper Method for Agent Name](#change-4-add-helper-method-for-agent-name)
       - [Change 5: Update DefaultConfig() for Name](#change-5-update-defaultconfig-for-name)
       - [Change 6: Add StepID to LLMInteraction](#change-6-add-stepid-to-llminteraction-for-step-specific-llm-calls)
    6. [Summary of Framework Changes](#summary-of-framework-changes)
    7. [What Will NOT Break](#what-will-not-break)
    8. [How Agents Can Set Their Name](#how-agents-can-set-their-name-optional)
    9. [Deferred Changes (Phase 5f+)](#deferred-changes-phase-5f)
    10. [Registry Viewer API Changes](#registry-viewer-api-changes)
    11. [UI Implementation](#ui-implementation)
        - [Cytoscape Node Types](#cytoscape-node-types)
        - [View Toggle](#view-toggle)
    12. [Implementation Phases (5a-5c)](#implementation-phases-1)
    13. [Data Flow Diagram](#data-flow-diagram)
14. [Next Steps](#next-steps)

---

## Overview

### Purpose

Provide developers with a graphical visualization of LLM-based plan execution, enabling:
- Visual understanding of step dependencies and execution flow
- Quick identification of failed steps and bottlenecks
- Deep inspection of LLM interactions at each step
- Efficient debugging via request_id or trace_id search

### Scope

- **In Scope:** LLM-based plan generation and execution (autonomous mode)
- **Out of Scope (Initial):** Workflow engine DAG visualization (future enhancement)
- **Applies To:** Both regular and HITL (Human-in-the-Loop) executions

### Design Principles Alignment

This proposal follows [FRAMEWORK_DESIGN_PRINCIPLES.md](../../FRAMEWORK_DESIGN_PRINCIPLES.md):
- **Interface-First Design:** New `ExecutionStore` interface for swappable backends
- **Safe Defaults:** Disabled by default, NoOp implementation when unavailable
- **Environment-Aware:** Configuration via `GOMIND_*` environment variables
- **Dependency Injection:** Follows established `LLMDebugStore` pattern

---

## User Requirements

| Requirement | Description |
|-------------|-------------|
| DAG Visualization | Display plan steps as a graph with dependency edges |
| Clickable Nodes | Click any step to view detailed information |
| LLM Interactions | Show request/response for LLM calls associated with each step |
| Search | Find executions by request_id or trace_id |
| Status Indicators | Visual distinction for success/failure/pending/running steps |
| Timing Information | Display duration and timeline for each step |
| HITL Support | Works for both regular and HITL executions |

---

## Data Analysis

### Available Data Structures

#### 1. RoutingPlan (Plan Structure)

Location: `orchestration/interfaces.go:23-39`

```go
type RoutingStep struct {
    StepID      string                 `json:"step_id"`
    AgentName   string                 `json:"agent_name"`
    Namespace   string                 `json:"namespace"`
    Instruction string                 `json:"instruction"`
    DependsOn   []string               `json:"depends_on,omitempty"`  // DAG edges
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type RoutingPlan struct {
    PlanID          string        `json:"plan_id"`
    OriginalRequest string        `json:"original_request"`
    Mode            RouterMode    `json:"mode"`
    Steps           []RoutingStep `json:"steps"`
    CreatedAt       time.Time     `json:"created_at"`
}
```

**Key Insight:** `DependsOn` field provides explicit DAG structure needed for visualization.

#### 2. ExecutionResult (Step Execution Data)

Location: `orchestration/interfaces.go:98-121`

```go
type ExecutionResult struct {
    PlanID        string                 `json:"plan_id"`
    Steps         []StepResult           `json:"steps"`
    Success       bool                   `json:"success"`
    TotalDuration time.Duration          `json:"total_duration"`
    Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type StepResult struct {
    StepID      string                 `json:"step_id"`
    AgentName   string                 `json:"agent_name"`
    Namespace   string                 `json:"namespace"`
    Instruction string                 `json:"instruction"`
    Response    string                 `json:"response"`
    Success     bool                   `json:"success"`
    Error       string                 `json:"error,omitempty"`
    Duration    time.Duration          `json:"duration"`
    Attempts    int                    `json:"attempts"`
    StartTime   time.Time              `json:"start_time"`
    EndTime     time.Time              `json:"end_time"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
```

**Key Insight:** Complete per-step timing and status information available.

**Important:** `ExecutionResult` only contains `PlanID`, NOT the full `RoutingPlan`. We need to store them together for DAG visualization.

#### 3. LLMDebugRecord (LLM Interactions)

Location: `orchestration/llm_debug_store.go:48-72`

```go
type LLMDebugRecord struct {
    RequestID         string            `json:"request_id"`
    OriginalRequestID string            `json:"original_request_id,omitempty"`
    TraceID           string            `json:"trace_id"`
    CreatedAt         time.Time         `json:"created_at"`
    UpdatedAt         time.Time         `json:"updated_at"`
    Interactions      []LLMInteraction  `json:"interactions"`
    Metadata          map[string]string `json:"metadata,omitempty"`
}
```

**Recording Sites (LLMInteraction.Type):**

| Type | Description | Source File |
|------|-------------|-------------|
| `plan_generation` | Initial plan creation from user request | `orchestrator.go` |
| `correction` | Plan correction after validation failure | `orchestrator.go` |
| `synthesis` | Final response synthesis (non-streaming) | `synthesizer.go` |
| `synthesis_streaming` | Final response synthesis (streaming) | `orchestrator.go` |
| `micro_resolution` | Parameter binding between steps | `micro_resolver.go` |
| `semantic_retry` | Full-context error recovery | `contextual_re_resolver.go` |

#### 4. Existing Store Pattern (Reference)

Location: `orchestration/llm_debug_store.go:23-44`

```go
type LLMDebugStore interface {
    RecordInteraction(ctx context.Context, requestID string, interaction LLMInteraction) error
    GetRecord(ctx context.Context, requestID string) (*LLMDebugRecord, error)
    SetMetadata(ctx context.Context, requestID string, key, value string) error
    ExtendTTL(ctx context.Context, requestID string, duration time.Duration) error
    ListRecent(ctx context.Context, limit int) ([]LLMDebugRecordSummary, error)
}
```

**This is the pattern we must follow for ExecutionStore.**

### Data Correlation

```
request_id (generated in orchestrator.go:725 or orchestrator.go:1029)
    ├── RoutingPlan.PlanID = request_id
    ├── ExecutionResult.PlanID = request_id
    ├── LLMDebugRecord.RequestID = request_id
    ├── LLMDebugRecord.TraceID (for distributed tracing)
    │
    └── For HITL Resume:
        ├── New request_id (for resumed execution)
        └── original_request_id (links to original conversation via baggage)
```

---

## Gap Analysis

### Current State

| Data | Captured | Persisted to Redis | Accessible via API |
|------|----------|-------------------|-------------------|
| LLMDebugRecord | ✅ | ✅ (DB 7, key: `gomind:llm:debug:*`) | ✅ (registry-viewer) |
| RoutingPlan | ✅ (in memory) | ❌ | ❌ |
| ExecutionResult | ✅ (returned) | ❌ | ❌ |
| StepResult | ✅ (in ExecutionResult) | ❌ | ❌ |

### Gap: Plan + Execution Not Persisted

**Problem:** After `executor.Execute()` returns in `orchestrator.go:893`, we have both `plan` and `result` available, but neither is persisted. They are only returned to the caller.

**Impact:** Cannot visualize execution history or build DAG after the request completes.

**Solution:** Add execution store that persists **both** `RoutingPlan` AND `ExecutionResult` together.

### Why Store Plan + Result Together?

1. **DAG Edges:** `RoutingStep.DependsOn` is in `RoutingPlan`, not `ExecutionResult`
2. **Self-Contained:** Each stored record has everything needed for visualization
3. **Correlation:** `LLMDebugRecord` can be fetched separately via same `request_id`

---

## Proposed Solution

### New Data Type: StoredExecution

Combines plan and result for complete DAG visualization data:

```go
// StoredExecution contains everything needed for DAG visualization.
// This is stored as a single record to ensure atomicity and self-containment.
type StoredExecution struct {
    // Correlation identifiers
    RequestID         string `json:"request_id"`
    OriginalRequestID string `json:"original_request_id,omitempty"` // For HITL resume correlation
    TraceID           string `json:"trace_id"`                      // For distributed tracing

    // Original user request (for search and display)
    OriginalRequest string `json:"original_request"`

    // The plan with step dependencies (contains DependsOn for DAG edges)
    Plan *RoutingPlan `json:"plan"`

    // The execution results (contains StepResult with timing/status)
    Result *ExecutionResult `json:"result"`

    // Timestamps
    CreatedAt time.Time `json:"created_at"`

    // Optional metadata for investigation notes
    Metadata map[string]string `json:"metadata,omitempty"`
}

// ExecutionSummary is a lightweight version for listing.
// Used by ListRecent to avoid loading full payloads.
// Note: Named ExecutionSummary (not ExecutionResultSummary) to avoid
// collision with existing ExecutionRecord type in interfaces.go:150
type ExecutionSummary struct {
    RequestID         string        `json:"request_id"`
    OriginalRequestID string        `json:"original_request_id,omitempty"`
    TraceID           string        `json:"trace_id"`
    OriginalRequest   string        `json:"original_request"`
    Success           bool          `json:"success"`
    StepCount         int           `json:"step_count"`
    FailedSteps       int           `json:"failed_steps"`
    TotalDuration     time.Duration `json:"total_duration"`
    CreatedAt         time.Time     `json:"created_at"`
}
```

### New Interface: ExecutionStore

Follows the established `LLMDebugStore` pattern from `orchestration/llm_debug_store.go`:

```go
// ExecutionStore stores execution records (plan + result) for debugging and visualization.
// Implementations must be safe for concurrent use.
//
// Design follows FRAMEWORK_DESIGN_PRINCIPLES.md:
// - Interface-first design for swappable backends
// - Safe defaults (NoOp when unavailable)
// - Disabled by default (enable via GOMIND_EXECUTION_DEBUG_STORE_ENABLED=true)
type ExecutionStore interface {
    // Store saves a complete execution record (plan + result).
    // This is called asynchronously from the orchestrator to avoid latency impact.
    // Errors should be logged but not propagated to avoid blocking orchestration.
    Store(ctx context.Context, execution *StoredExecution) error

    // Get retrieves the complete execution record by request ID.
    // Returns an error if the record is not found or has expired.
    Get(ctx context.Context, requestID string) (*StoredExecution, error)

    // GetByTraceID retrieves an execution by distributed trace ID.
    // Useful for correlating with Jaeger traces.
    GetByTraceID(ctx context.Context, traceID string) (*StoredExecution, error)

    // SetMetadata adds metadata to an existing record.
    // Useful for adding investigation notes or flags.
    SetMetadata(ctx context.Context, requestID string, key, value string) error

    // ExtendTTL extends retention for investigation.
    // Allows keeping important records longer than the default TTL.
    ExtendTTL(ctx context.Context, requestID string, duration time.Duration) error

    // ListRecent returns recent records for UI listing.
    // Results are ordered by creation time, newest first.
    ListRecent(ctx context.Context, limit int) ([]ExecutionSummary, error)
}
```

### New Interface: StorageProvider

**Per FRAMEWORK_DESIGN_PRINCIPLES.md:** "Dependency Inversion: All modules depend on `core` interfaces, not implementations"

The framework does NOT directly depend on Redis. Instead, it defines a `StorageProvider` interface that abstracts the underlying storage backend:

```go
// StorageProvider abstracts the underlying storage backend.
// Implementations can be Redis, PostgreSQL, S3, etc.
//
// This follows FRAMEWORK_DESIGN_PRINCIPLES.md:
// - "All modules depend on core interfaces, not implementations"
// - "Core module should NOT make assumptions about specific implementations (Redis, OpenAI, etc.)"
//
// The application is responsible for providing a concrete implementation.
//
// NOTE: Method names are intentionally storage-agnostic (not Redis-specific).
// The sorted index operations can be implemented by:
// - Redis: ZADD, ZREVRANGEBYSCORE, ZREM
// - PostgreSQL: INSERT with score column, SELECT ORDER BY score DESC, DELETE
// - DynamoDB: GSI with sort key
type StorageProvider interface {
    // Get retrieves a value by key. Returns empty string and nil error if not found.
    Get(ctx context.Context, key string) (string, error)

    // Set stores a value with TTL. Use 0 for no expiration.
    Set(ctx context.Context, key string, value string, ttl time.Duration) error

    // Del deletes one or more keys.
    Del(ctx context.Context, keys ...string) error

    // Exists checks if a key exists.
    Exists(ctx context.Context, key string) (bool, error)

    // AddToIndex adds a member with score to a sorted index.
    // Used for time-based listing (score = timestamp).
    // Redis implementation: ZADD
    AddToIndex(ctx context.Context, key string, score float64, member string) error

    // ListByScoreDesc returns members from sorted index (highest score first) with pagination.
    // Used for listing recent executions.
    // Redis implementation: ZREVRANGEBYSCORE
    ListByScoreDesc(ctx context.Context, key string, min, max string, offset, count int64) ([]string, error)

    // RemoveFromIndex removes members from a sorted index.
    // Used for cleaning up stale index entries.
    // Redis implementation: ZREM
    RemoveFromIndex(ctx context.Context, key string, members ...string) error
}

// NewExecutionStoreWithProvider creates an ExecutionStore backed by the given StorageProvider.
// This is the recommended way to create an ExecutionStore - the application provides
// the storage backend implementation (Redis, PostgreSQL, etc.).
func NewExecutionStoreWithProvider(provider StorageProvider, config ExecutionStoreConfig, logger core.Logger) ExecutionStore {
    return &executionStoreImpl{
        provider: provider,
        config:   config,
        logger:   logger,
    }
}
```

### Configuration: ExecutionStoreConfig

Framework configuration is minimal - storage-specific settings are managed by the application.

```go
// ExecutionStoreConfig holds configuration for execution storage.
// This is embedded in OrchestratorConfig.
//
// Note: Storage-specific settings (Redis DB, connection URL, etc.) are NOT here.
// Per FRAMEWORK_DESIGN_PRINCIPLES.md, the framework doesn't assume specific backends.
// The application provides storage configuration when creating the StorageProvider.
type ExecutionStoreConfig struct {
    // Enabled controls whether execution storage is active.
    // Default: false (disabled). Enable via GOMIND_EXECUTION_DEBUG_STORE_ENABLED=true
    Enabled bool `json:"enabled"`

    // TTL is the retention period for successful records.
    // Default: 24h. Override via GOMIND_EXECUTION_DEBUG_TTL
    // This is passed to the StorageProvider implementation.
    TTL time.Duration `json:"ttl"`

    // ErrorTTL is the retention period for records with errors.
    // Default: 168h (7 days). Override via GOMIND_EXECUTION_DEBUG_ERROR_TTL
    ErrorTTL time.Duration `json:"error_ttl"`

    // KeyPrefix is the prefix for all storage keys.
    // Default: "gomind:execution:debug". Override via GOMIND_EXECUTION_DEBUG_KEY_PREFIX
    // This allows multi-tenant deployments or custom namespacing.
    // Per FRAMEWORK_DESIGN_PRINCIPLES.md: "Explicit Override: Always allow explicit configuration"
    KeyPrefix string `json:"key_prefix"`
}

// DefaultExecutionStoreConfig returns the default configuration.
// Feature is disabled by default per FRAMEWORK_DESIGN_PRINCIPLES.md.
func DefaultExecutionStoreConfig() ExecutionStoreConfig {
    return ExecutionStoreConfig{
        Enabled:   false,              // Disabled by default
        TTL:       24 * time.Hour,     // 24 hours for success
        ErrorTTL:  7 * 24 * time.Hour, // 7 days for errors
        KeyPrefix: "gomind:execution:debug", // Default prefix (follows LLM Debug Store pattern)
    }
}
```

### Storage Key Patterns (Configurable via KeyPrefix)

Keys are built using the configurable `KeyPrefix` (default: `gomind:execution:debug`):

| Key Pattern | Purpose | Type |
|-------------|---------|------|
| `{KeyPrefix}:{request_id}` | Full StoredExecution JSON | String |
| `{KeyPrefix}:index` | Recent executions list | Sorted Set (score=timestamp) |
| `{KeyPrefix}:trace:{trace_id}` | Trace ID → Request ID mapping | String |

**Example with default prefix:**
- `gomind:execution:debug:orch-123` - Execution record
- `gomind:execution:debug:index` - Index sorted set
- `gomind:execution:debug:trace:abc123` - Trace mapping

**Example with custom prefix (`GOMIND_EXECUTION_DEBUG_KEY_PREFIX=myapp:dag:`):**
- `myapp:dag:orch-123` - Execution record
- `myapp:dag:index` - Index sorted set
- `myapp:dag:trace:abc123` - Trace mapping

**Recommendation:** Use Redis DB 8 (`core.RedisDBExecutionDebug`) to keep separate from LLM Debug Store (DB 7).

---

## Framework Changes

### Files to Create in Framework (`orchestration/`)

| File | Purpose | Pattern Source | Status |
|------|---------|----------------|--------|
| `orchestration/execution_store.go` | Interface (`ExecutionStore`, `StorageProvider`), types, config, `executionStoreImpl` | `llm_debug_store.go` | ✅ Done |
| `orchestration/noop_execution_store.go` | NoOp implementation (disabled/fallback) | `noop_llm_debug_store.go` | ✅ Done |
| `orchestration/execution_store_test.go` | Unit tests (using mock StorageProvider) | `llm_debug_store_test.go` | ✅ Done |
| `orchestration/redis_execution_store.go` | Auto-configuring Redis-backed ExecutionStore (same pattern as `redis_llm_debug_store.go`) | `redis_llm_debug_store.go` | ✅ Done |

### Core Module Update

| File | Change | Status |
|------|--------|--------|
| `core/redis_client.go` | Added `RedisDBExecutionDebug = 8` constant for dedicated Redis DB | ✅ Done |

### Application-Level Files (Optional)

Since the framework now includes `NewRedisExecutionDebugStore()` with auto-configuration, application-level files are **optional**. Only create custom implementations if you need a non-Redis backend.

| File | Purpose | When Needed |
|------|---------|-------------|
| `redis_storage_provider.go` | Custom Redis implementation using `*core.RedisClient` | Only if you need custom Redis behavior |
| `postgres_storage_provider.go` | PostgreSQL implementation of `StorageProvider` | For PostgreSQL backend |
| `dynamodb_storage_provider.go` | DynamoDB implementation of `StorageProvider` | For DynamoDB backend |

### Files to Modify

#### 1. `orchestration/interfaces.go`

Add to `OrchestratorConfig` (after line 322, after `HITL` field):

```go
// ExecutionStore configuration for DAG visualization
// When enabled, stores plan + execution results for debugging.
// Disabled by default for backward compatibility.
// Enable via GOMIND_EXECUTION_DEBUG_STORE_ENABLED=true or config.ExecutionStore.Enabled=true.
ExecutionStore ExecutionStoreConfig `json:"execution_store,omitempty"`

// ExecutionStoreBackend is the storage backend for execution records.
// If nil and ExecutionStore.Enabled is true, uses NoOp store (logs warning).
// Use WithExecutionStore() to inject a StorageProvider-backed implementation.
ExecutionStoreBackend ExecutionStore `json:"-"` // Not serializable
```

#### 2. `orchestration/orchestrator.go`

Add fields to `AIOrchestrator` struct (after line 199, after `debugSeqID` field):

```go
// Execution Store for DAG visualization
// When enabled, stores plan + execution results for debugging
executionStore ExecutionStore
// executionWg tracks in-flight execution storage goroutines for graceful shutdown
executionWg sync.WaitGroup
```

Add setter method (after `SetLLMDebugStore` around line 398):

```go
// SetExecutionStore sets the execution store for DAG visualization.
// Per FRAMEWORK_DESIGN_PRINCIPLES.md, nil values are safely ignored.
func (o *AIOrchestrator) SetExecutionStore(store ExecutionStore) {
    if store == nil {
        return // Safe default: ignore nil
    }
    o.executionStore = store
}

// GetExecutionStore returns the configured execution store (for API handlers).
func (o *AIOrchestrator) GetExecutionStore() ExecutionStore {
    return o.executionStore
}
```

Add storage call in `ProcessRequest` (after line 911, after `executor.Execute` returns):

```go
result, err := o.executor.Execute(ctx, plan)

// Store execution for DAG visualization (async, non-blocking)
// Per FRAMEWORK_DESIGN_PRINCIPLES.md: errors logged but not propagated
// Per FRAMEWORK_DESIGN_PRINCIPLES.md lines 132-135: track goroutines for graceful shutdown
if o.executionStore != nil {
    // Track this goroutine for graceful shutdown (same pattern as debugWg)
    o.executionWg.Add(1)

    go func(ctx context.Context, plan *RoutingPlan, result *ExecutionResult) {
        defer o.executionWg.Done()

        // Use original context with timeout to preserve baggage (original_request_id)
        storeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
        defer cancel()

        // Extract trace correlation from baggage
        bag := telemetry.GetBaggage(ctx)
        traceID := ""
        originalRequestID := requestID
        if bag != nil {
            traceID = bag["trace_id"]
            if origID := bag["original_request_id"]; origID != "" {
                originalRequestID = origID
            }
        }

        stored := &StoredExecution{
            RequestID:         requestID,
            OriginalRequestID: originalRequestID,
            TraceID:           traceID,
            OriginalRequest:   request,
            Plan:              plan,
            Result:            result,
            CreatedAt:         time.Now(),
        }

        if err := o.executionStore.Store(storeCtx, stored); err != nil {
            if o.logger != nil {
                o.logger.Warn("Failed to store execution for DAG visualization", map[string]interface{}{
                    "operation":  "execution_store",
                    "request_id": requestID,
                    "error":      err.Error(),
                })
            }
        }
    }(ctx, plan, result)
}

if err != nil {
    // ... existing error handling
}
```

Add similar storage call in `ProcessRequestStreaming` (after line 1239, after `executor.Execute` returns).

Modify `Shutdown` method (around line 498) to wait for execution store goroutines:

```go
func (o *AIOrchestrator) Shutdown(ctx context.Context) error {
    // Stop background operations first
    o.cancel()

    // Wait for pending debug recordings AND execution storage with timeout
    done := make(chan struct{})
    go func() {
        o.debugWg.Wait()
        o.executionWg.Wait()  // ADD: Wait for execution store goroutines
        close(done)
    }()

    select {
    case <-done:
        // ... existing success logging
    case <-ctx.Done():
        // ... existing timeout handling
    }
}
```

#### 3. `orchestration/factory.go` ✅ IMPLEMENTED

Store setup with auto-configuration (after LLMDebug setup block, around line 240):

```go
// Set up execution store if enabled
// Auto-configures from environment (same pattern as LLM Debug Store).
// Enable via GOMIND_EXECUTION_DEBUG_STORE_ENABLED=true
if config.ExecutionStore.Enabled {
    if config.ExecutionStoreBackend != nil {
        // Custom backend provided - use it
        orchestrator.SetExecutionStore(config.ExecutionStoreBackend)
        factoryLogger.Info("ExecutionStore configured with custom backend", map[string]interface{}{
            "operation": "execution_store_initialization",
            "ttl":       config.ExecutionStore.TTL.String(),
            "error_ttl": config.ExecutionStore.ErrorTTL.String(),
        })
    } else {
        // Auto-configure Redis store from environment (same pattern as LLM Debug Store)
        store, err := NewRedisExecutionDebugStore(
            WithExecutionDebugRedisDB(core.RedisDBExecutionDebug),
            WithExecutionDebugLogger(deps.Logger),
            WithExecutionDebugTTL(config.ExecutionStore.TTL),
            WithExecutionDebugErrorTTL(config.ExecutionStore.ErrorTTL),
            WithExecutionDebugKeyPrefix(config.ExecutionStore.KeyPrefix),
        )
        if err != nil {
            // Resilient behavior - use NoOp store if Redis unavailable
            factoryLogger.Warn("Failed to initialize Redis execution debug store, using NoOp", map[string]interface{}{
                "operation": "execution_debug_store_initialization",
                "error":     err.Error(),
                "hint":      "Set REDIS_URL or GOMIND_REDIS_URL, or disable via GOMIND_EXECUTION_DEBUG_STORE_ENABLED=false",
            })
            orchestrator.SetExecutionStore(NewNoOpExecutionStore())
        } else {
            orchestrator.SetExecutionStore(store)
            factoryLogger.Info("Redis execution debug store initialized", map[string]interface{}{
                "operation":  "execution_debug_store_initialization",
                "redis_db":   core.RedisDBExecutionDebug,
                "key_prefix": config.ExecutionStore.KeyPrefix,
                "ttl":        config.ExecutionStore.TTL.String(),
                "error_ttl":  config.ExecutionStore.ErrorTTL.String(),
            })
        }
    }
}
```

Add option functions:

```go
// WithExecutionStore explicitly sets the execution store implementation.
// Use this to inject a StorageProvider-backed store.
// Setting a store automatically enables execution storage (same pattern as WithLLMDebugStore).
//
// Example:
//   provider := NewRedisStorageProvider(redisClient) // Application code
//   store := orchestration.NewExecutionStoreWithProvider(provider, config, logger)
//   orchestrator, _ := orchestration.NewOrchestrator(deps, orchestration.WithExecutionStore(store))
func WithExecutionStore(store ExecutionStore) OrchestratorOption {
    return func(c *OrchestratorConfig) {
        c.ExecutionStore.Enabled = true  // Auto-enable when store is provided
        c.ExecutionStoreBackend = store
    }
}

// WithExecutionStoreProvider is a convenience function that creates an ExecutionStore
// from a StorageProvider. The application provides the StorageProvider implementation.
func WithExecutionStoreProvider(provider StorageProvider, logger core.Logger) OrchestratorOption {
    return func(c *OrchestratorConfig) {
        c.ExecutionStore.Enabled = true  // Auto-enable when provider is provided
        c.ExecutionStoreBackend = NewExecutionStoreWithProvider(provider, c.ExecutionStore, logger)
    }
}
```

#### 4. `orchestration/interfaces.go` - DefaultConfig()

Add environment variable parsing in `DefaultConfig()` (before line 523, after HITL config parsing at line 521, before `return config`):

```go
// Execution store configuration from environment
// Note: Storage-specific settings (Redis URL, DB, etc.) are NOT here.
// The application configures those when creating the StorageProvider.
config.ExecutionStore = DefaultExecutionStoreConfig()
if enabled := os.Getenv("GOMIND_EXECUTION_DEBUG_STORE_ENABLED"); enabled != "" {
    config.ExecutionStore.Enabled = strings.ToLower(enabled) == "true"
}
if ttl := os.Getenv("GOMIND_EXECUTION_DEBUG_TTL"); ttl != "" {
    if duration, err := time.ParseDuration(ttl); err == nil {
        config.ExecutionStore.TTL = duration
    }
}
if errorTTL := os.Getenv("GOMIND_EXECUTION_DEBUG_ERROR_TTL"); errorTTL != "" {
    if duration, err := time.ParseDuration(errorTTL); err == nil {
        config.ExecutionStore.ErrorTTL = duration
    }
}
```

### Environment Variables (Framework)

| Variable | Default | Description |
|----------|---------|-------------|
| `GOMIND_EXECUTION_DEBUG_STORE_ENABLED` | `false` | Enable execution debug storage |
| `GOMIND_EXECUTION_DEBUG_TTL` | `24h` | Retention for successful executions |
| `GOMIND_EXECUTION_DEBUG_ERROR_TTL` | `168h` | Retention for failed executions |
| `GOMIND_EXECUTION_DEBUG_KEY_PREFIX` | `gomind:execution:debug:` | Prefix for all storage keys (multi-tenant support) |
| `GOMIND_EXECUTION_DEBUG_REDIS_DB` | `8` | Redis database number (uses `core.RedisDBExecutionDebug`) |
| `REDIS_URL` or `GOMIND_REDIS_URL` | `localhost:6379` | Redis connection URL (shared with LLM Debug Store) |

**Note:** The framework now auto-configures Redis when `GOMIND_EXECUTION_DEBUG_STORE_ENABLED=true`. No application code needed!

### Features (Same as LLM Debug Store)

The execution debug store has full feature parity with the LLM Debug Store:
- **Gzip compression**: Large payloads (>100KB) are automatically compressed
- **Layer 1 resilience**: Built-in retry with exponential backoff (3 retries, 100ms-2s backoff)
- **Layer 2 resilience**: Optional circuit breaker injection via `WithExecutionDebugCircuitBreaker()`
- **Layer 3 resilience**: NoOp fallback when Redis is unavailable

---

## API Design

### Registry Viewer Endpoints

#### List Executions

```
GET /api/executions?limit=50&offset=0
```

**Response:**
```json
{
  "executions": [
    {
      "request_id": "orch-1705312800123456789",
      "original_request_id": "orch-1705312800123456789",
      "trace_id": "abc123def456",
      "original_request": "What's the weather in Tokyo and convert to Celsius?",
      "success": true,
      "step_count": 2,
      "failed_steps": 0,
      "total_duration_ms": 2450,
      "created_at": "2024-01-15T10:00:00Z"
    }
  ],
  "total": 150,
  "has_more": true
}
```

#### Get Execution Details

```
GET /api/executions/{request_id}
```

**Response:**
```json
{
  "request_id": "orch-1705312800123456789",
  "original_request_id": "orch-1705312800123456789",
  "trace_id": "abc123def456",
  "original_request": "What's the weather in Tokyo and convert to Celsius?",
  "plan": {
    "plan_id": "orch-1705312800123456789",
    "original_request": "What's the weather in Tokyo and convert to Celsius?",
    "mode": "autonomous",
    "steps": [
      {
        "step_id": "step-1",
        "agent_name": "weather-tool",
        "instruction": "Get current weather for Tokyo",
        "depends_on": []
      },
      {
        "step_id": "step-2",
        "agent_name": "unit-converter",
        "instruction": "Convert temperature to Celsius",
        "depends_on": ["step-1"]
      }
    ],
    "created_at": "2024-01-15T10:00:00Z"
  },
  "result": {
    "plan_id": "orch-1705312800123456789",
    "success": true,
    "total_duration": 2450000000,
    "steps": [
      {
        "step_id": "step-1",
        "agent_name": "weather-tool",
        "success": true,
        "response": "{\"temp\": 72, \"unit\": \"F\"}",
        "duration": 1200000000,
        "start_time": "2024-01-15T10:00:00.100Z",
        "end_time": "2024-01-15T10:00:01.300Z",
        "attempts": 1
      },
      {
        "step_id": "step-2",
        "agent_name": "unit-converter",
        "success": true,
        "response": "{\"temp\": 22.2, \"unit\": \"C\"}",
        "duration": 800000000,
        "start_time": "2024-01-15T10:00:01.350Z",
        "end_time": "2024-01-15T10:00:02.150Z",
        "attempts": 1
      }
    ]
  },
  "created_at": "2024-01-15T10:00:00Z"
}
```

#### Get DAG Structure (Computed)

```
GET /api/executions/{request_id}/dag
```

**Response:**
```json
{
  "nodes": [
    {
      "id": "step-1",
      "label": "weather-tool",
      "instruction": "Get current weather for Tokyo",
      "status": "completed",
      "duration_ms": 1200,
      "level": 0
    },
    {
      "id": "step-2",
      "label": "unit-converter",
      "instruction": "Convert temperature to Celsius",
      "status": "completed",
      "duration_ms": 800,
      "level": 1
    }
  ],
  "edges": [
    {
      "source": "step-1",
      "target": "step-2"
    }
  ],
  "levels": [
    ["step-1"],
    ["step-2"]
  ],
  "statistics": {
    "total_nodes": 2,
    "completed_nodes": 2,
    "failed_nodes": 0,
    "skipped_nodes": 0,
    "max_parallelism": 1,
    "depth": 2
  }
}
```

#### Search

```
GET /api/executions/search?q={request_id_or_trace_id}
```

---

## UI Design

### Glassmorphism Theme Integration

The DAG visualization follows the existing Registry Viewer glassmorphism theme from `static/index.html`.

#### CSS Variables (Already Defined)

```css
:root {
    --glass-bg: rgba(10, 10, 15, 0.85);
    --glass-bg-light: rgba(20, 20, 30, 0.75);
    --glass-border: rgba(255, 255, 255, 0.08);
    --glass-border-light: rgba(255, 255, 255, 0.12);
    --glass-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
    --glass-blur: 24px;
    --text-primary: #ffffff;
    --text-secondary: rgba(255, 255, 255, 0.75);
    --text-muted: rgba(255, 255, 255, 0.5);
    --accent-blue: #0a84ff;
    --accent-purple: #da8fff;
    --accent-green: #32d74b;
    --accent-red: #ff6b6b;
    --accent-orange: #ffb340;
    --accent-teal: #64d2ff;
    --accent-pink: #ff6eb4;
    --border-subtle: rgba(255, 255, 255, 0.08);
}
```

### Layout

```
┌─────────────────────────────────────────────────────────────────────┐
│  Services  │  LLM Debug  │  HITL  │  Execution DAG  │              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  🔍 Search by Request ID or Trace ID          [Search]      │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌──────────────────────┐  ┌────────────────────────────────────┐  │
│  │  Recent Executions   │  │          DAG Visualization         │  │
│  │                      │  │                                    │  │
│  │  ○ orch-170531...    │  │       ┌─────────┐                  │  │
│  │    ✓ 2 steps, 2.4s   │  │       │ step-1  │                  │  │
│  │                      │  │       │ weather │                  │  │
│  │  ○ orch-170530...    │  │       └────┬────┘                  │  │
│  │    ✗ 3 steps, 1.8s   │  │            │                       │  │
│  │                      │  │       ┌────▼────┐                  │  │
│  │  ○ orch-170529...    │  │       │ step-2  │                  │  │
│  │    ✓ 5 steps, 4.2s   │  │       │ convert │                  │  │
│  │                      │  │       └─────────┘                  │  │
│  └──────────────────────┘  └────────────────────────────────────┘  │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  Step Details: step-1 (weather-tool)                        │   │
│  │  ─────────────────────────────────────────────────────────  │   │
│  │  Status: ✓ Success    Duration: 1.2s    Attempts: 1         │   │
│  │  Instruction: Get current weather for Tokyo                 │   │
│  │  Response: {"temp": 72, "unit": "F"}                        │   │
│  │                                                              │   │
│  │  ▼ LLM Interactions (1)                                     │   │
│  │    ┌─────────────────────────────────────────────────────┐  │   │
│  │    │ plan_generation | gpt-4 | 450ms | 1250 tokens       │  │   │
│  │    │ [View Prompt] [View Response]                       │  │   │
│  │    └─────────────────────────────────────────────────────┘  │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Node Status Colors (Theme-Aligned)

| Status | Color | CSS Variable | Cytoscape Background |
|--------|-------|--------------|---------------------|
| Pending | Muted | `--text-muted` | `rgba(255, 255, 255, 0.15)` |
| Running | Blue | `--accent-blue` | `rgba(10, 132, 255, 0.25)` |
| Completed | Green | `--accent-green` | `rgba(50, 215, 75, 0.25)` |
| Failed | Red | `--accent-red` | `rgba(255, 107, 107, 0.25)` |
| Skipped | Orange | `--accent-orange` | `rgba(255, 179, 64, 0.25)` |

### Glassmorphism Component Styles

#### DAG Container

```css
.dag-container {
    flex: 1;
    background: var(--glass-bg);
    backdrop-filter: blur(var(--glass-blur)) saturate(180%);
    -webkit-backdrop-filter: blur(var(--glass-blur)) saturate(180%);
    border-radius: 18px;
    border: 1px solid var(--glass-border-light);
    box-shadow: var(--glass-shadow), inset 0 1px 0 rgba(255,255,255,0.05);
    overflow: hidden;
}
```

#### Execution List Panel

```css
.execution-list {
    width: 280px;
    background: var(--glass-bg-light);
    backdrop-filter: blur(var(--glass-blur)) saturate(180%);
    border-radius: 18px;
    border: 1px solid var(--glass-border);
    overflow-y: auto;
}

.execution-item {
    padding: 14px 16px;
    border-bottom: 1px solid var(--border-subtle);
    cursor: pointer;
    transition: all 0.15s;
}

.execution-item:hover {
    background: rgba(255, 255, 255, 0.04);
}

.execution-item.selected {
    background: rgba(10, 132, 255, 0.15);
    border-left: 3px solid var(--accent-blue);
}
```

#### Search Box

```css
.dag-search-box {
    display: flex;
    align-items: center;
    gap: 10px;
    background: rgba(255, 255, 255, 0.06);
    border-radius: 12px;
    padding: 0 14px;
    border: 1px solid transparent;
    transition: all 0.2s;
}

.dag-search-box:focus-within {
    background: rgba(255, 255, 255, 0.1);
    border-color: var(--accent-blue);
    box-shadow: 0 0 0 3px rgba(10, 132, 255, 0.2);
}

.dag-search-box input {
    flex: 1;
    background: none;
    border: none;
    color: var(--text-primary);
    font-size: 14px;
    padding: 12px 0;
    outline: none;
}

.dag-search-box input::placeholder {
    color: var(--text-muted);
}
```

#### Step Details Panel

```css
.step-details-panel {
    background: var(--glass-bg);
    backdrop-filter: blur(var(--glass-blur)) saturate(180%);
    border-radius: 18px;
    border: 1px solid var(--glass-border-light);
    box-shadow: var(--glass-shadow);
    padding: 20px;
}

.step-details-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
    padding-bottom: 12px;
    border-bottom: 1px solid var(--border-subtle);
}

.step-details-title {
    color: var(--text-primary);
    font-size: 16px;
    font-weight: 600;
}
```

#### Status Badges (Follow Existing Pattern)

```css
.status-badge {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 5px 12px;
    border-radius: 20px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
}

.status-badge.completed {
    background: rgba(50, 215, 75, 0.2);
    color: var(--accent-green);
    border: 1px solid rgba(50, 215, 75, 0.3);
}

.status-badge.failed {
    background: rgba(255, 107, 107, 0.2);
    color: var(--accent-red);
    border: 1px solid rgba(255, 107, 107, 0.3);
}

.status-badge.running {
    background: rgba(10, 132, 255, 0.2);
    color: var(--accent-blue);
    border: 1px solid rgba(10, 132, 255, 0.3);
    animation: pulse-running 1.5s ease-in-out infinite;
}

@keyframes pulse-running {
    0%, 100% { box-shadow: 0 0 0 0 rgba(10, 132, 255, 0.4); }
    50% { box-shadow: 0 0 8px 4px rgba(10, 132, 255, 0.2); }
}

.status-badge.pending {
    background: rgba(255, 255, 255, 0.08);
    color: var(--text-secondary);
    border: 1px solid rgba(255, 255, 255, 0.1);
}

.status-badge.skipped {
    background: rgba(255, 179, 64, 0.2);
    color: var(--accent-orange);
    border: 1px solid rgba(255, 179, 64, 0.3);
}
```

### Cytoscape.js Node Styling (Glassmorphism)

```javascript
const cytoscapeStyles = [
    // Base node style - glassy appearance
    {
        selector: 'node',
        style: {
            'label': 'data(label)',
            'text-valign': 'center',
            'text-halign': 'center',
            'color': '#ffffff',
            'font-size': '12px',
            'font-weight': '600',
            'text-outline-color': 'rgba(0, 0, 0, 0.5)',
            'text-outline-width': 2,
            'width': 120,
            'height': 50,
            'shape': 'roundrectangle',
            'border-width': 1,
            'border-opacity': 0.3,
        }
    },
    // Status-based colors
    {
        selector: 'node[status="completed"]',
        style: {
            'background-color': 'rgba(50, 215, 75, 0.25)',
            'border-color': 'rgba(50, 215, 75, 0.5)',
        }
    },
    {
        selector: 'node[status="failed"]',
        style: {
            'background-color': 'rgba(255, 107, 107, 0.25)',
            'border-color': 'rgba(255, 107, 107, 0.5)',
        }
    },
    {
        selector: 'node[status="running"]',
        style: {
            'background-color': 'rgba(10, 132, 255, 0.25)',
            'border-color': 'rgba(10, 132, 255, 0.5)',
        }
    },
    {
        selector: 'node[status="pending"]',
        style: {
            'background-color': 'rgba(255, 255, 255, 0.1)',
            'border-color': 'rgba(255, 255, 255, 0.2)',
        }
    },
    {
        selector: 'node[status="skipped"]',
        style: {
            'background-color': 'rgba(255, 179, 64, 0.25)',
            'border-color': 'rgba(255, 179, 64, 0.5)',
        }
    },
    // Selected node - glowing effect
    {
        selector: 'node:selected',
        style: {
            'border-width': 2,
            'border-color': '#0a84ff',
            'box-shadow': '0 0 20px rgba(10, 132, 255, 0.5)',
        }
    },
    // Edge styles
    {
        selector: 'edge',
        style: {
            'width': 2,
            'line-color': 'rgba(255, 255, 255, 0.2)',
            'target-arrow-color': 'rgba(255, 255, 255, 0.4)',
            'target-arrow-shape': 'triangle',
            'curve-style': 'bezier',
            'arrow-scale': 1.2,
        }
    },
    // Highlighted edge (dependency chain)
    {
        selector: 'edge.highlighted',
        style: {
            'line-color': 'rgba(10, 132, 255, 0.6)',
            'target-arrow-color': '#0a84ff',
            'width': 3,
        }
    }
];
```

### Interactive Features

1. **Click Node:** Opens step details panel with glassmorphism styling
2. **Hover Node:** Shows tooltip with quick stats (glassy tooltip)
3. **Click Edge:** Highlights dependency chain with blue glow
4. **Zoom/Pan:** Navigate large DAGs (controls in glassy toolbar)
5. **Timeline Toggle:** Switch between DAG and timeline view

### Recommended Library: Cytoscape.js

```html
<!-- CDN - no build step required -->
<script src="https://cdnjs.cloudflare.com/ajax/libs/cytoscape/3.28.1/cytoscape.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/dagre/0.8.5/dagre.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/cytoscape-dagre/2.5.0/cytoscape-dagre.min.js"></script>
```

---

## Implementation Phases

### Phase 1: Framework - Execution Debug Store (orchestration module) ✅ COMPLETE

**Files Created:**
- ✅ `orchestration/execution_store.go` - Interfaces (`ExecutionStore`, `StorageProvider`), types, config, `executionStoreImpl`
- ✅ `orchestration/noop_execution_store.go` - NoOp implementation for disabled state
- ✅ `orchestration/execution_store_test.go` - Unit tests using mock StorageProvider
- ✅ `orchestration/redis_execution_store.go` - Auto-configuring Redis backend with full feature parity (gzip, Layer 1 retry, optional circuit breaker)

**Files Modified:**
- ✅ `orchestration/interfaces.go` - Added `ExecutionStoreConfig` to `OrchestratorConfig`
- ✅ `orchestration/orchestrator.go` - Added store field, setter, storage calls
- ✅ `orchestration/factory.go` - Added auto-configuration when enabled
- ✅ `core/redis_client.go` - Added `RedisDBExecutionDebug = 8` constant

### Phase 2: Auto-Configuration ✅ IMPLEMENTED (No Application Code Needed!)

**The framework now auto-configures Redis when enabled.** Developers just set environment variables:

```bash
# Enable execution debug storage - that's it!
export GOMIND_EXECUTION_DEBUG_STORE_ENABLED=true
export REDIS_URL=redis://localhost:6379
```

Or in Kubernetes:

```yaml
env:
  - name: GOMIND_EXECUTION_DEBUG_STORE_ENABLED
    value: "true"
  - name: REDIS_URL
    value: "redis://redis.gomind.svc.cluster.local:6379"
```

**Zero code changes required in agents!** The framework's `CreateOrchestrator()` automatically:
1. Detects `GOMIND_EXECUTION_DEBUG_STORE_ENABLED=true`
2. Creates `NewRedisExecutionDebugStore()` with auto-configured settings
3. Falls back to NoOp if Redis is unavailable (resilient behavior)

#### Optional: Custom StorageProvider

Only if you need a non-Redis backend (PostgreSQL, DynamoDB, etc.), implement `StorageProvider`:

```go
// Example: Custom PostgreSQL StorageProvider
type PostgresStorageProvider struct {
    db *sql.DB
}

func (p *PostgresStorageProvider) Get(ctx context.Context, key string) (string, error) {
    // SELECT value FROM executions WHERE key = $1
}

func (p *PostgresStorageProvider) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
    // INSERT INTO executions (key, value, expires_at) VALUES ($1, $2, $3)
}

// ... implement remaining StorageProvider methods

// Usage with custom provider:
orchestrator, _ := orchestration.CreateOrchestrator(config, deps,
    orchestration.WithExecutionStoreProvider(customProvider, logger),
)
```

### Phase 3: Registry Viewer API

**File to Modify:** `examples/registry-viewer-app/main.go`

**New Endpoints:**
- `GET /api/executions` - List recent executions
- `GET /api/executions/{request_id}` - Get full execution
- `GET /api/executions/{request_id}/dag` - Get DAG structure
- `GET /api/executions/search` - Search by request_id or trace_id

### Phase 4: DAG Visualization UI

**File to Modify:** `examples/registry-viewer-app/static/index.html`

**Components:**
- New "Execution DAG" tab
- Execution list sidebar
- Cytoscape.js DAG visualization
- Step details panel
- Search bar

---

## Technical Considerations

### Performance

1. **Async Storage:** Store execution asynchronously (goroutine) to not block response
2. **Timeout:** 5-second timeout for storage operation
3. **Compression:** Use gzip for large execution records (same as LLMDebugStore)
4. **Pagination:** Limit list results, use offset for pagination

### Data Consistency

1. **Atomic Storage:** Store plan + result together in single Redis SET
2. **Index Updates:** Use Redis transactions for index consistency
3. **TTL Alignment:** Default TTL matches LLMDebugStore (24h success, 7d error)

### Resilience

Following the three-layer resilience pattern from [ARCHITECTURE.md](../../orchestration/ARCHITECTURE.md#three-layer-resilience-architecture):

1. **Layer 1:** Built-in retry (3 attempts, exponential backoff)
2. **Layer 2:** Optional circuit breaker via dependency injection
3. **Layer 3:** NoOp fallback on failure (never blocks orchestration)

### Backwards Compatibility

1. **Disabled by Default:** No impact on existing deployments
2. **Optional Feature:** Can be enabled incrementally
3. **Graceful Degradation:** UI shows "data unavailable" for missing records

---

## Framework Compliance Checklist

Per [FRAMEWORK_DESIGN_PRINCIPLES.md](../../FRAMEWORK_DESIGN_PRINCIPLES.md) and [orchestration/ARCHITECTURE.md](../../orchestration/ARCHITECTURE.md):

| Principle | Status | Implementation |
|-----------|--------|----------------|
| Interface-First Design | ✅ | `ExecutionStore` + `StorageProvider` interfaces for custom backends |
| Dependency Inversion | ✅ | Framework depends on interfaces; Redis is opt-in convenience |
| Zero-Config Experience | ✅ | Auto-configures Redis from env vars (same pattern as LLM Debug Store) |
| Custom Backend Support | ✅ | Application can inject custom `StorageProvider` (PostgreSQL, DynamoDB, etc.) |
| Safe Defaults | ✅ | Disabled by default, NoOp when Redis unavailable |
| Environment-Aware | ✅ | `GOMIND_*` prefixed variables + `REDIS_URL` |
| Module Dependencies | ✅ | Uses existing `go-redis/v8` dependency (shared with LLM Debug Store) |
| Fail-Safe Runtime | ✅ | Async storage, errors logged not propagated, NoOp fallback |
| Three-Layer Resilience | ✅ | Built-in retry, optional CB, NoOp fallback |
| Telemetry Nil-Safe | ✅ | Check before use, continue on failure |
| No Backwards Breakage | ✅ | Optional feature, existing code unchanged |

---

## Open Questions (Resolved)

1. ~~Should we store the full plan in ExecutionResult?~~
   - **Resolved:** Store full `RoutingPlan` in `StoredExecution` for self-contained visualization

2. ~~Which Redis DB to use?~~
   - **Resolved:** Uses `core.RedisDBExecutionDebug = 8` (dedicated DB for execution debug store).
   - LLM Debug Store uses DB 7, Execution Debug Store uses DB 8 (clean separation).

3. ~~What type name to use for summary?~~
   - **Resolved:** `ExecutionSummary` (not `ExecutionResultSummary`) to avoid collision

4. ~~Should framework have direct Redis dependency?~~
   - **Resolved:** Yes, for developer experience. Framework already depends on `go-redis/v8` for `RedisLLMDebugStore`.
   - `NewRedisExecutionDebugStore()` provides zero-configuration auto-setup (same pattern as `NewRedisLLMDebugStore()`).
   - Custom backends are still supported via `StorageProvider` interface for PostgreSQL, DynamoDB, etc.

5. ~~Should we have in-memory implementation for testing?~~
   - **Resolved:** No. Tests use mock `StorageProvider`. Application can create its own if needed.

6. ~~Should execution store have same features as LLM Debug Store?~~
   - **Resolved:** Yes, full feature parity. `RedisExecutionDebugStore` includes:
     - Gzip compression for large payloads (>100KB)
     - Layer 1 resilience with retry and exponential backoff
     - Optional Layer 2 circuit breaker injection
     - Same environment variable patterns (`GOMIND_EXECUTION_DEBUG_*`)

---

## Phase 5: Enhanced Full Execution Flow Visualization

**Status:** Proposed (January 2026)

### Motivation

The current DAG visualization shows only the tool/service steps. Users want to see the **complete execution flow** including:
- The orchestrator agent as the root node
- LLM calls during planning and synthesis phases
- HITL checkpoints when human approval is required
- The relationship between all components

This provides a complete picture of what happened during an orchestration request.

### Current State vs Proposed

#### Current DAG (Steps Only)
```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ stock-service│     │geocoding-tool│     │  news-tool   │
└──────┬───────┘     └──────┬───────┘     └──────────────┘
       │                    │
       │             ┌──────▼───────┐
       │             │currency-tool │
       │             └──────┬───────┘
       │                    │
       │             ┌──────▼───────┐
       └────────────►│weather-service│
                     └──────────────┘
```

#### Proposed: Full Execution Flow
```
                              ┌─────────────────────────┐
                              │     🤖 ORCHESTRATOR     │  ◄── Root node (agent_name)
                              │    (travel-agent)       │
                              └───────────┬─────────────┘
                                          │
                              ┌───────────▼─────────────┐
                              │    💭 LLM: Planning     │  ◄── LLM call (type=plan_generation)
                              │   model: gpt-4o         │
                              │   duration: 1.2s        │
                              └───────────┬─────────────┘
                                          │
                    ┌─────────────────────┴─────────────────────┐  (if HITL enabled)
                    │                                           │
          ┌─────────▼─────────┐                                 │
          │  ⏸️ CHECKPOINT    │  ◄── HITL: Plan Approval        │
          │  status: approved │      (from ExecutionCheckpoint) │
          │  waited: 45s      │                                 │
          └─────────┬─────────┘                                 │
                    │ ✅ Approved                                │
                    └─────────────────────┬─────────────────────┘
                                          │
       ╔══════════════════════════════════╧══════════════════════════════════╗
       ║                        EXECUTION PHASE                              ║
       ╚══════════════════════════════════╤══════════════════════════════════╝
                                          │
         ┌────────────────┬───────────────┼───────────────┬────────────────┐
         │                │               │               │                │
  ┌──────▼──────┐  ┌──────▼──────┐ ┌──────▼──────┐       │                │
  │🔧 stock-    │  │🔧 geocoding-│ │🔧 news-tool │       │                │
  │  service    │  │    tool     │ │             │       │                │
  │  (step-1)   │  │  (step-2)   │ │  (step-3)   │       │                │
  └──────┬──────┘  └──────┬──────┘ └─────────────┘       │                │
         │                │                              │                │
         │         ┌──────▼──────┐                       │                │
         │         │🔧 currency- │◄──────────────────────┘                │
         │         │    tool     │ depends_on: [step-2]                   │
         │         │  (step-4)   │                                        │
         │         └──────┬──────┘                                        │
         │                │                                               │
         │         ┌──────▼──────┐                                        │
         └────────►│🔧 weather-  │◄───────────────────────────────────────┘
                   │   service   │ depends_on: [step-1, step-4]
                   │  (step-5)   │
                   └──────┬──────┘
                          │
       ╔══════════════════╧═══════════════════════════════════════════════╝
       ║                        SYNTHESIS PHASE
       ╚══════════════════════════════════╤══════════════════════════════════╗
                                          │                                  ║
                              ┌───────────▼─────────────┐                    ║
                              │    💭 LLM: Synthesis    │  ◄── LLM call      ║
                              │   model: gpt-4o         │      (type=synthesis)
                              │   duration: 0.8s        │                    ║
                              └───────────┬─────────────┘                    ║
                                          │                                  ║
                              ┌───────────▼─────────────┐                    ║
                              │     📤 RESPONSE         │  ◄── Final output ║
                              │   success: true         │                    ║
                              │   total: 19.1s          │                    ║
                              └─────────────────────────┘                    ║
       ╚═════════════════════════════════════════════════════════════════════╝
```

### Node Types & Visual Design

| Node Type | Icon | Color Scheme | Shape | Data Source |
|-----------|------|--------------|-------|-------------|
| Orchestrator | 🤖 | Purple (`--accent-purple`) | Hexagon | `agent_name` from StoredExecution |
| LLM Call | 💭 | Blue (`--accent-blue`) | Rounded pill | `llm_interactions[]` from LLMDebugRecord |
| HITL Checkpoint | ⏸️ | Orange (`--accent-orange`) | Diamond | `ExecutionCheckpoint` (separate Redis key) |
| Tool/Service Step | 🔧 | Green/Red by status | Rectangle | `plan.steps` + `step_results` |
| Response | 📤 | Teal (`--accent-teal`) | Rounded rectangle | `result.success` + `total_duration` |

### Data Gap Analysis

#### Currently Available Data

| Data | Store | Key Pattern | Available |
|------|-------|-------------|-----------|
| Plan + Step Results | ExecutionStore | `gomind:execution:debug:{request_id}` | ✅ |
| LLM Interactions | LLMDebugStore | `gomind:llm:debug:{request_id}` | ✅ |
| HITL Checkpoints | CheckpointStore | `gomind:hitl:{checkpoint_id}` | ✅ |

#### Data Gaps to Fill

| Gap | Current State | Required Change | Priority |
|-----|---------------|-----------------|----------|
| `agent_name` in StoredExecution | Not stored | Add to `StoredExecution` struct | High |
| `step_id` in LLMInteraction | Not stored | Add optional field for per-step LLM tracking | Medium |
| Checkpoint linkage | Separate Redis key | Join by `request_id` in registry-viewer | High |
| LLM call ordering | Stored with timestamp | Already available via `timestamp` field | ✅ Done |

### Backend Changes Required

> **Code Review Complete (January 2026)**
> After reviewing the framework code, the following exact changes are documented.

---

#### Change 1: Add `Name` to OrchestratorConfig

**File:** `orchestration/interfaces.go` (line ~262)

**Current State:**
```go
type OrchestratorConfig struct {
    RoutingMode       RouterMode        `json:"routing_mode"`
    ExecutionOptions  ExecutionOptions  `json:"execution_options"`
    // ... (no Name field exists)
    RequestIDPrefix string `json:"request_id_prefix,omitempty"` // exists at line 338
}
```

**After Change:**
```go
type OrchestratorConfig struct {
    // Name identifies this orchestrator agent for debugging and visualization.
    // Examples: "travel-agent", "support-bot", "order-processor"
    // Default: Falls back to RequestIDPrefix if set, otherwise "orchestrator"
    // Env: GOMIND_AGENT_NAME (shared with HITL for agent identity)
    Name string `json:"name,omitempty"`

    RoutingMode       RouterMode        `json:"routing_mode"`
    // ... existing fields unchanged ...
}
```

**Why this location?** The `Name` field should be first in the struct as it's the primary identifier. Adding it at the top makes it prominent in config dumps and documentation.

**Backward Compatibility:** ✅ Optional field with fallback. Existing agents work unchanged.

---

#### Change 2: Add `AgentName` to StoredExecution

**File:** `orchestration/execution_store.go` (line 60-80)

**Current State:**
```go
type StoredExecution struct {
    RequestID         string `json:"request_id"`
    OriginalRequestID string `json:"original_request_id,omitempty"`
    TraceID           string `json:"trace_id"`
    OriginalRequest   string `json:"original_request"`
    Plan              *RoutingPlan     `json:"plan"`
    Result            *ExecutionResult `json:"result"`
    CreatedAt         time.Time        `json:"created_at"`
    Metadata          map[string]string `json:"metadata,omitempty"`
}
```

**After Change:**
```go
type StoredExecution struct {
    RequestID         string `json:"request_id"`
    OriginalRequestID string `json:"original_request_id,omitempty"`
    TraceID           string `json:"trace_id"`

    // AgentName identifies the orchestrator that created this execution.
    // Populated from OrchestratorConfig.Name or RequestIDPrefix or "orchestrator".
    AgentName string `json:"agent_name,omitempty"`

    OriginalRequest   string `json:"original_request"`
    Plan              *RoutingPlan     `json:"plan"`
    Result            *ExecutionResult `json:"result"`
    CreatedAt         time.Time        `json:"created_at"`
    Metadata          map[string]string `json:"metadata,omitempty"`
}
```

**Backward Compatibility:** ✅ Old stored records will have empty `agent_name`. UI shows "orchestrator" fallback.

---

#### Change 3: Update Orchestrator Storage Call

**File:** `orchestration/orchestrator.go` (lines 966-974 and 1343-1351)

There are **two locations** where `StoredExecution` is created:
1. `ProcessRequest()` - line 966
2. `ProcessRequestStreaming()` - line 1343

**Current State (both locations):**
```go
stored := &StoredExecution{
    RequestID:         reqID,
    OriginalRequestID: originalRequestID,
    TraceID:           traceID,
    OriginalRequest:   request,
    Plan:              plan,
    Result:            result,
    CreatedAt:         time.Now(),
}
```

**After Change (both locations):**
```go
stored := &StoredExecution{
    RequestID:         reqID,
    OriginalRequestID: originalRequestID,
    TraceID:           traceID,
    AgentName:         o.getAgentName(), // NEW
    OriginalRequest:   request,
    Plan:              plan,
    Result:            result,
    CreatedAt:         time.Now(),
}
```

---

#### Change 4: Add Helper Method for Agent Name

**File:** `orchestration/orchestrator.go` (new method, add after line ~400)

```go
// getAgentName returns the agent name for debug visualization.
// Priority: config.Name > config.RequestIDPrefix > "orchestrator"
func (o *AIOrchestrator) getAgentName() string {
    if o.config.Name != "" {
        return o.config.Name
    }
    if o.config.RequestIDPrefix != "" {
        return o.config.RequestIDPrefix
    }
    return "orchestrator"
}
```

**Rationale:** Uses existing `RequestIDPrefix` as fallback since some agents already customize this (e.g., "awhl" for agent-with-human-approval). This means existing agents get a reasonable name without code changes.

---

#### Change 5: Update DefaultConfig() for Name

**File:** `orchestration/interfaces.go` (in `DefaultConfig()` function, line ~373)

**Add environment variable parsing:**
```go
func DefaultConfig() *OrchestratorConfig {
    config := &OrchestratorConfig{
        Name:              os.Getenv("GOMIND_AGENT_NAME"), // Reuses existing env var for agent identity
        RoutingMode:       ModeAutonomous,
        // ... existing fields ...
    }
    // ... existing parsing ...
    return config
}
```

---

#### Change 6: Add `StepID` to LLMInteraction for Step-Specific LLM Calls

**Status:** ✅ COMPLETE (January 2026) - StepID added to LLMInteraction for DAG visualization

**Problem Statement:**
Currently, `micro_resolution` and `semantic_retry` LLM calls are recorded without their associated step context. In the DAG visualization, these appear as disconnected nodes in the main flow chain, when they should be associated with specific execution steps.

**Which LLM Types Need StepID:**

| LLM Type | Step-Specific? | Rationale |
|----------|----------------|-----------|
| `micro_resolution` | ✅ **Yes** | Called during step execution to resolve parameters |
| `semantic_retry` | ✅ **Yes** | Called when a step fails and needs error-aware retry |
| `plan_generation` | ❌ No | Happens before any steps exist |
| `correction` | ❌ No | Plan-level correction, not step-specific |
| `synthesis` | ❌ No | Happens after all steps complete |
| `tiered_selection` | ❌ No | Capability discovery, not step execution |

**Current Call Chain (Missing StepID):**

```
executor.go:1534        → hybridResolver.ResolveParameters(ctx, stepResultsMap, capability)
                                ↓ (step.StepID available here but NOT passed)
hybrid_resolver.go:92   → ResolveParameters(ctx, dependencyResults, targetCapability)
                                ↓ (no stepID parameter)
micro_resolver.go:88    → ResolveParameters(ctx, sourceData, targetCapability, hint)
                                ↓ (no stepID to record)
micro_resolver.go:239   → recordDebugInteraction(ctx, requestID, LLMInteraction{...})
                                ↓ (LLMInteraction has no StepID field)
```

**Files to Modify:**

##### 6.1 Add `StepID` Field to LLMInteraction

**File:** `orchestration/llm_debug_store.go` (line 76-105)

```go
type LLMInteraction struct {
    Type        string    `json:"type"`
    Timestamp   time.Time `json:"timestamp"`
    Model       string    `json:"model"`
    Provider    string    `json:"provider,omitempty"`
    DurationMs  int64     `json:"duration_ms"`
    TokensIn    int       `json:"tokens_in,omitempty"`
    TokensOut   int       `json:"tokens_out,omitempty"`
    Success     bool      `json:"success"`
    Error       string    `json:"error,omitempty"`
    Prompt      string    `json:"prompt,omitempty"`
    Response    string    `json:"response,omitempty"`

    // StepID associates this LLM call with a specific execution step.
    // Populated for: micro_resolution, semantic_retry
    // Empty for: plan_generation, correction, synthesis, tiered_selection
    StepID string `json:"step_id,omitempty"`
}
```

##### 6.2 Update HybridResolver Signature

**File:** `orchestration/hybrid_resolver.go` (line 92)

```go
// Before:
func (h *HybridResolver) ResolveParameters(
    ctx context.Context,
    dependencyResults map[string]*StepResult,
    targetCapability *EnhancedCapability,
) (map[string]interface{}, error)

// After:
func (h *HybridResolver) ResolveParameters(
    ctx context.Context,
    dependencyResults map[string]*StepResult,
    targetCapability *EnhancedCapability,
    stepID string,  // NEW: For LLM debug correlation
) (map[string]interface{}, error)
```

##### 6.3 Update MicroResolver Signature

**File:** `orchestration/micro_resolver.go` (line 88)

```go
// Before:
func (m *MicroResolver) ResolveParameters(
    ctx context.Context,
    sourceData map[string]interface{},
    targetCapability *EnhancedCapability,
    hint string,
) (map[string]interface{}, error)

// After:
func (m *MicroResolver) ResolveParameters(
    ctx context.Context,
    sourceData map[string]interface{},
    targetCapability *EnhancedCapability,
    hint string,
    stepID string,  // NEW: For LLM debug correlation
) (map[string]interface{}, error)
```

##### 6.4 Update MicroResolver recordDebugInteraction Calls

**File:** `orchestration/micro_resolver.go` (lines 239, 263)

```go
// Before (line 239):
m.recordDebugInteraction(ctx, requestID, LLMInteraction{
    Type:       "micro_resolution",
    // ... other fields
})

// After:
m.recordDebugInteraction(ctx, requestID, LLMInteraction{
    Type:       "micro_resolution",
    StepID:     stepID,  // NEW
    // ... other fields
})
```

##### 6.5 Update Executor Call Site

**File:** `orchestration/executor.go` (line 1534)

```go
// Before:
resolved, err := e.hybridResolver.ResolveParameters(ctx, stepResultsMap, capabilityForResolution)

// After:
resolved, err := e.hybridResolver.ResolveParameters(ctx, stepResultsMap, capabilityForResolution, step.StepID)
```

##### 6.6 Update ContextualReResolver (for semantic_retry)

**File:** `orchestration/contextual_re_resolver.go`

Similar pattern - add `stepID` parameter to `ReResolve()` method and include in `LLMInteraction`:

```go
// recordDebugInteraction calls (lines 159, 179):
r.recordDebugInteraction(ctx, requestID, LLMInteraction{
    Type:   "semantic_retry",
    StepID: stepID,  // NEW
    // ... other fields
})
```

**Note:** The caller of `ContextualReResolver.ReResolve()` needs to be identified and updated to pass `stepID`.

##### 6.7 Files That Do NOT Need StepID Changes

These files record LLM interactions that are not step-specific:

| File | LLM Types | Why No StepID |
|------|-----------|---------------|
| `orchestrator.go` | plan_generation, correction | Plan-level, before/during planning |
| `synthesizer.go` | synthesis, synthesis_streaming | Post-execution, all steps complete |
| `tiered_capability_provider.go` | tiered_selection | Capability discovery phase |

**Summary of Changes for StepID:**

| File | Change | Lines Affected | Risk |
|------|--------|----------------|------|
| `orchestration/llm_debug_store.go` | Add `StepID` field to `LLMInteraction` | +3 lines | Low |
| `orchestration/hybrid_resolver.go` | Add `stepID` parameter to `ResolveParameters()` | +1 line (signature) | Low (internal) |
| `orchestration/micro_resolver.go` | Add `stepID` parameter, include in `LLMInteraction` | +4 lines | Low (internal) |
| `orchestration/executor.go` | Pass `step.StepID` to hybrid resolver | +1 line | Low |
| `orchestration/contextual_re_resolver.go` | Add `stepID` to semantic_retry calls | +4 lines | Low (internal) |
| Caller of `ReResolve()` | Pass step context | +1 line | Low |

**Total: ~14 lines added, 2 method signatures modified (internal APIs only)**

**API Visibility Assessment:**

| Type | Exported? | Used Outside Module? | Verdict |
|------|-----------|---------------------|---------|
| `HybridResolver` | Yes (uppercase) | No - only in `orchestration/*.go` | **Internal API** |
| `MicroResolver` | Yes (uppercase) | No - only in `orchestration/*.go` | **Internal API** |

Both types are implementation details of the orchestration module. Changing their signatures does not break external consumers per **FRAMEWORK_DESIGN_PRINCIPLES.md §6 (Backwards Compatibility)**.

**Risk Assessment:**
- **Low risk** - signature changes affect only internal module code
- All callers are within `orchestration/` directory
- No external applications use these types directly

**Test Requirements:**

Per **FRAMEWORK_DESIGN_PRINCIPLES.md §5 (Testing Requirements)**, the following tests must be updated:

| Test File | Changes Required |
|-----------|-----------------|
| `orchestration/hybrid_resolver_test.go` | Update `ResolveParameters()` calls to include `stepID` parameter |
| `orchestration/llm_debug_store_test.go` | Add test for `StepID` field in `LLMInteraction` |
| `orchestration/contextual_re_resolver_test.go` | Update tests for semantic_retry `StepID` |

**New Test Cases to Add:**

```go
// Test: StepID is populated for micro_resolution
func TestMicroResolver_RecordsStepID(t *testing.T) {
    // Verify LLMInteraction.StepID == "step_1" for micro_resolution calls
}

// Test: StepID is empty for plan_generation (not step-specific)
func TestOrchestrator_PlanGeneration_NoStepID(t *testing.T) {
    // Verify LLMInteraction.StepID == "" for plan_generation
}

// Test: StepID is empty for synthesis (not step-specific)
func TestSynthesizer_Synthesis_NoStepID(t *testing.T) {
    // Verify LLMInteraction.StepID == "" for synthesis
}
```

---

### Impact Analysis & Execution Plan

#### Production Code Call Sites

**`HybridResolver.ResolveParameters()` - 3 callers (all internal):**

| File | Line | Current Call |
|------|------|--------------|
| `executor.go` | 1534 | `e.hybridResolver.ResolveParameters(ctx, stepResultsMap, capabilityForResolution)` |
| `hybrid_resolver.go` | 162 | Internal call to `microResolver.ResolveParameters()` |
| `hybrid_resolver.go` | 281 | Internal call to `microResolver.ResolveParameters()` |

**`MicroResolver.ResolveParameters()` - 2 callers (all internal):**

| File | Line | Called From |
|------|------|-------------|
| `hybrid_resolver.go` | 162 | `h.microResolver.ResolveParameters(ctx, sourceData, targetCapability, hint)` |
| `hybrid_resolver.go` | 281 | `h.microResolver.ResolveParameters(ctx, sourceData, targetCap, paramHint)` |

**`LLMInteraction{}` Creation Sites - 17 sites:**

| File | Lines | Types | StepID Needed? |
|------|-------|-------|----------------|
| `micro_resolver.go` | 239, 263 | `micro_resolution` | ✅ **Yes** |
| `contextual_re_resolver.go` | 159, 179 | `semantic_retry` | ✅ **Yes** |
| `orchestrator.go` | 680, 693, 1480, 1519, 1565, 1693, 1733 | `plan_generation`, `correction` | ❌ No |
| `synthesizer.go` | 92, 119 | `synthesis`, `synthesis_streaming` | ❌ No |
| `tiered_capability_provider.go` | 351, 379 | `tiered_selection` | ❌ No |

#### Test Code Impact

**Tests requiring signature updates:**

| Test File | Call Count | Action |
|-----------|------------|--------|
| `hybrid_resolver_test.go` | **14 calls** | Add `""` as `stepID` parameter |
| `contextual_re_resolver_test.go` | **15 calls** | Update if `ReResolve()` signature changes |
| `llm_debug_store_test.go` | N/A | Add assertions for `StepID` field |

#### Backwards Compatibility

| Scenario | Impact | Risk |
|----------|--------|------|
| Old Redis data → New code | `StepID` will be empty string | ✅ **None** |
| New Redis data → Old code | `step_id` field ignored | ✅ **None** |
| Registry Viewer API | Additive field | ✅ **None** |
| Frontend (index.html) | Handles unknown fields | ✅ **None** |

#### Regression Prevention

**Run before and after changes:**
```bash
go test ./orchestration/... -v -run "TestHybridResolver"    # 14 tests
go test ./orchestration/... -v -run "TestMicroResolver"     # Related tests
go test ./orchestration/... -v -run "TestContextual"        # 15 tests
go test ./orchestration/... -v -run "TestLLMDebug"          # Debug store tests
```

#### Step-by-Step Execution Order

| Step | Action | Why This Order |
|------|--------|----------------|
| 1 | Add `StepID` field to `LLMInteraction` | No breaking changes, additive only |
| 2 | Update `hybrid_resolver_test.go` (14 calls) | Add `""` as stepID param |
| 3 | Update `MicroResolver.ResolveParameters()` signature | Tests now pass |
| 4 | Update `HybridResolver.ResolveParameters()` signature | Depends on step 3 |
| 5 | Update `executor.go` call site | Pass `step.StepID` |
| 6 | Update `micro_resolver.go` to include `StepID` | Actual functionality |
| 7 | Update `contextual_re_resolver.go` for semantic_retry | Similar pattern |
| 8 | Run full test suite | Verify no regressions |
| 9 | Add new StepID verification tests | Prevent future regressions |

#### Rollback Strategy

| Step | Action |
|------|--------|
| 1 | Revert code changes (signatures + struct field) |
| 2 | Existing Redis data remains valid |
| 3 | No data migration needed |

---

### Summary of Framework Changes

#### Phase 5a: Agent Name (Implemented ✅)

| File | Change | Lines Affected | Risk |
|------|--------|----------------|------|
| `orchestration/interfaces.go` | Add `Name` field to `OrchestratorConfig` | +5 lines | Low |
| `orchestration/interfaces.go` | Add env var parsing in `DefaultConfig()` | +4 lines | Low |
| `orchestration/execution_store.go` | Add `AgentName` field to `StoredExecution` | +3 lines | Low |
| `orchestration/orchestrator.go` | Add `getAgentName()` helper method | +10 lines | Low |
| `orchestration/orchestrator.go` | Update storage call in `ProcessRequest()` | +1 line | Low |
| `orchestration/orchestrator.go` | Update storage call in `ProcessRequestStreaming()` | +1 line | Low |

**Phase 5a Total: ~24 lines added**

#### Phase 5b: StepID for LLM Interactions (Pending)

| File | Change | Lines Affected | Risk |
|------|--------|----------------|------|
| `orchestration/llm_debug_store.go` | Add `StepID` field to `LLMInteraction` | +3 lines | Low |
| `orchestration/hybrid_resolver.go` | Add `stepID` parameter to `ResolveParameters()` | +1 line | Low (internal API) |
| `orchestration/micro_resolver.go` | Add `stepID` parameter, include in `LLMInteraction` | +4 lines | Low (internal API) |
| `orchestration/executor.go` | Pass `step.StepID` to hybrid resolver | +1 line | Low |
| `orchestration/contextual_re_resolver.go` | Add `stepID` to semantic_retry `LLMInteraction` | +4 lines | Low (internal API) |
| Caller of `ReResolve()` | Pass step context | +1 line | Low |

**Phase 5b Total: ~14 lines added, 2 method signatures modified (internal APIs only)**

**Overall Total: ~38 lines added, 2 method signatures modified**

---

### What Will NOT Break

| Scenario | Reason |
|----------|--------|
| Existing agents without `Name` config | Falls back to `RequestIDPrefix` or "orchestrator" |
| Existing stored data in Redis | New field is optional, old data works fine |
| API consumers reading execution data | JSON is additive, new fields are ignored by old clients |
| Tests comparing structs | New field has zero value by default |

---

### How Agents Can Set Their Name (Optional)

**Option 1: Environment Variable (Zero Code Change)**
```bash
export GOMIND_AGENT_NAME="travel-agent"
```

**Option 2: Config (Explicit)**
```go
config := orchestration.DefaultConfig()
config.Name = "travel-agent"
orch, _ := orchestration.CreateOrchestrator(config, deps)
```

**Option 3: RequestIDPrefix (Already Works)**
```go
config := orchestration.DefaultConfig()
config.RequestIDPrefix = "travel"  // Will use "travel" as agent name
```

---

### Deferred Changes (Phase 5f+)

| Change | Reason for Deferral |
|--------|---------------------|
| Add `CheckpointID` to `StoredExecution` | Can be derived from HITL store via `request_id` join |
| Add `ParentStepID` to `LLMInteraction` | For nested orchestration (agent calls agent) - rare use case |

---

### Registry Viewer API Changes

#### New Unified Endpoint

```
GET /api/executions/{request_id}/full
```

**Response:** Combines all three data sources into a unified view.

```json
{
  "request_id": "orch-1705312800123456789",
  "agent_name": "travel-agent",
  "original_request": "What's the weather in Tokyo?",
  "trace_id": "abc123def456",

  "phases": [
    {
      "type": "orchestrator",
      "agent_name": "travel-agent",
      "started_at": "2024-01-15T10:00:00.000Z"
    },
    {
      "type": "llm_call",
      "llm_type": "plan_generation",
      "model": "gpt-4o",
      "provider": "openai",
      "duration_ms": 1200,
      "tokens_in": 450,
      "tokens_out": 320,
      "success": true,
      "started_at": "2024-01-15T10:00:00.050Z"
    },
    {
      "type": "checkpoint",
      "checkpoint_id": "cp-abc123",
      "interrupt_point": "plan_approval",
      "status": "approved",
      "wait_duration_ms": 45000,
      "created_at": "2024-01-15T10:00:01.250Z",
      "resolved_at": "2024-01-15T10:00:46.250Z"
    },
    {
      "type": "execution",
      "steps": [
        {
          "step_id": "step-1",
          "agent_name": "weather-tool",
          "status": "completed",
          "duration_ms": 1200,
          "depends_on": []
        }
      ]
    },
    {
      "type": "llm_call",
      "llm_type": "synthesis",
      "model": "gpt-4o",
      "duration_ms": 800,
      "success": true,
      "started_at": "2024-01-15T10:00:48.000Z"
    },
    {
      "type": "response",
      "success": true,
      "total_duration_ms": 49200
    }
  ],

  "dag": {
    "nodes": [...],
    "edges": [...]
  }
}
```

### UI Implementation

#### Cytoscape Node Types

```javascript
const nodeTypeStyles = {
    // Orchestrator root node
    'orchestrator': {
        'shape': 'hexagon',
        'background-color': 'rgba(218, 143, 255, 0.25)',
        'border-color': 'rgba(218, 143, 255, 0.6)',
        'border-width': 2,
        'width': 140,
        'height': 60
    },
    // LLM call nodes
    'llm_call': {
        'shape': 'round-rectangle',
        'background-color': 'rgba(10, 132, 255, 0.2)',
        'border-color': 'rgba(10, 132, 255, 0.5)',
        'border-width': 1,
        'width': 130,
        'height': 50
    },
    // HITL checkpoint nodes
    'checkpoint': {
        'shape': 'diamond',
        'background-color': 'rgba(255, 179, 64, 0.25)',
        'border-color': 'rgba(255, 179, 64, 0.6)',
        'border-width': 2,
        'width': 100,
        'height': 100
    },
    // Tool/service step nodes (existing)
    'step': {
        'shape': 'round-rectangle',
        'width': 150,
        'height': 46
    },
    // Final response node
    'response': {
        'shape': 'round-rectangle',
        'background-color': 'rgba(100, 210, 255, 0.2)',
        'border-color': 'rgba(100, 210, 255, 0.5)',
        'border-width': 2,
        'width': 140,
        'height': 50
    }
};
```

#### View Toggle

Add toggle to switch between:
- **Steps Only** (current view) - Shows just the tool/service DAG
- **Full Flow** (new view) - Shows complete execution including LLM calls and checkpoints

```html
<div class="dag-view-toggle">
    <button class="toggle-btn active" data-view="steps">Steps Only</button>
    <button class="toggle-btn" data-view="full">Full Flow</button>
</div>
```

### Implementation Phases

#### Phase 5a: Agent Name & Basic Data Enhancement ✅ COMPLETE
- [x] Add `Name` field to `OrchestratorConfig` (`interfaces.go`)
- [x] Add env var parsing for `GOMIND_AGENT_NAME` in `DefaultConfig()`
- [x] Add `AgentName` field to `StoredExecution`
- [x] Update orchestrator to populate `AgentName` when storing

#### Phase 5b: StepID for LLM Interactions (Pending)
- [ ] Add `StepID` field to `LLMInteraction` (`llm_debug_store.go`)
- [ ] Update `HybridResolver.ResolveParameters()` signature to accept `stepID`
- [ ] Update `MicroResolver.ResolveParameters()` signature to accept `stepID`
- [ ] Update `executor.go` to pass `step.StepID` to hybrid resolver
- [ ] Update `micro_resolver.go` to include `StepID` in `LLMInteraction`
- [ ] Update `contextual_re_resolver.go` for semantic_retry `StepID`

**Files to modify:**
| File | Change |
|------|--------|
| `orchestration/llm_debug_store.go` | Add `StepID` field |
| `orchestration/hybrid_resolver.go` | Add `stepID` parameter |
| `orchestration/micro_resolver.go` | Add `stepID` parameter + include in LLMInteraction |
| `orchestration/executor.go` | Pass `step.StepID` to resolver |
| `orchestration/contextual_re_resolver.go` | Include `StepID` in semantic_retry |

#### Phase 5c: HITL Checkpoint Step Association (UI Only)

**Problem:** Step-level HITL checkpoints (`before_step`, `after_step`, `on_error`) are not visualized in the DAG. Only plan approval checkpoints are shown.

**Data Already Available:**
```go
// ExecutionCheckpoint already has step association (hitl_interfaces.go:320-324)
type ExecutionCheckpoint struct {
    InterruptPoint    InterruptPoint  `json:"interrupt_point"`     // plan_generated, before_step, after_step, on_error
    CurrentStep       *RoutingStep    `json:"current_step"`        // Has StepID for step-level checkpoints
    CurrentStepResult *StepResult     `json:"current_step_result"` // For after_step/on_error
}
```

**Current Gap in index.html:3632:**
```javascript
// Only plan checkpoints are shown - step-level checkpoints are filtered out!
const planCheckpoints = checkpoints.filter(c => c.interrupt_point === 'before_plan_execution');
```

**Tasks:**
- [x] Add step-level checkpoint nodes (`before_step`, `after_step`, `on_error`) to DAG
- [x] Connect checkpoint nodes to their parent steps using `checkpoint.current_step.step_id`
- [x] Add visual distinction: ⏸️ (before_step), ✓ (after_step), ⚠️ (on_error)
- [x] Update legend to show step-level checkpoint types

**Implementation:**
```javascript
// Add after step nodes are created (around line 3698)
const stepCheckpoints = checkpoints.filter(c =>
    ['before_step', 'after_step', 'on_error'].includes(c.interrupt_point)
);

stepCheckpoints.forEach((cp, idx) => {
    const nodeId = `checkpoint_step_${idx}`;
    const parentStepId = cp.current_step?.step_id;

    if (parentStepId) {
        const icons = { 'before_step': '⏸️', 'after_step': '✓', 'on_error': '⚠️' };
        nodes.push({
            data: {
                id: nodeId,
                label: icons[cp.interrupt_point] || '⬡',
                nodeType: 'checkpoint',
                checkpointType: cp.interrupt_point,
                parentStep: parentStepId,
                status: cp.status
            }
        });

        // Connect based on interrupt point
        if (cp.interrupt_point === 'before_step') {
            // Insert checkpoint before step execution
            // Find edges going INTO parentStepId, redirect through checkpoint
            edges.push({ data: { source: nodeId, target: parentStepId } });
        } else {
            // Connect checkpoint after step (after_step, on_error)
            edges.push({ data: { source: parentStepId, target: nodeId } });
        }
    }
});
```

**Risk:** Low - UI only, no backend changes

#### Phase 5d: Registry Viewer API Enhancement ✅ COMPLETE (January 2026)
- [x] Update `/api/executions/{request_id}/unified` to include `step_id` from LLM interactions
- [x] Update UI to associate Resolution nodes with their parent steps

#### Phase 5e: UI Enhancement ✅ COMPLETE (January 2026)
- [x] Add view toggle (Steps Only / Full Flow)
- [x] Add new Cytoscape node types for orchestrator, LLM, checkpoint, response
- [x] Add edge styling for different connection types
- [x] Update layout algorithm for hierarchical flow
- [x] Add click handlers for new node types
- [x] Associate Resolution LLM nodes with parent step nodes using `step_id`

#### Deferred (Phase 5g+)
- [ ] Add `CheckpointID` to `StoredExecution` (can join by request_id)
- [ ] Add `ParentStepID` to `LLMInteraction` for nested orchestration

### Data Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Redis Storage                                   │
├───────────────────────┬───────────────────────┬─────────────────────────────┤
│  DB 8: Execution      │  DB 7: LLM Debug      │  DB 6: HITL                 │
│  (StoredExecution)    │  (LLMDebugRecord)     │  (ExecutionCheckpoint)      │
│                       │                       │                             │
│  request_id ─────────►│◄───── request_id ────►│◄───── request_id            │
│  agent_name           │  interactions[]       │  checkpoint_id              │
│  plan                 │    .type              │  interrupt_point ◄──────────┤
│  result               │    .model             │  current_step.step_id ◄─────┤ Step Association
│                       │    .duration_ms       │  current_step_result        │
│                       │    .step_id (Phase5b) │  status                     │
└───────────┬───────────┴───────────┬───────────┴─────────────┬───────────────┘
            │                       │                         │
            └───────────────────────┼─────────────────────────┘
                                    │
                    ┌───────────────▼───────────────┐
                    │    Registry Viewer API        │
                    │  /api/executions/{id}/unified │
                    │                               │
                    │  Joins all three data sources │
                    │  by request_id                │
                    │                               │
                    │  Step associations:           │
                    │  - LLM.step_id → Step (5b)    │
                    │  - HITL.current_step → Step   │
                    └───────────────┬───────────────┘
                                    │
                    ┌───────────────▼───────────────┐
                    │         UI (Cytoscape)        │
                    │                               │
                    │  Renders unified flow graph   │
                    │  with step associations:      │
                    │  - Resolution → parent step   │
                    │  - Checkpoint → parent step   │
                    └───────────────────────────────┘
```

### Step Association Summary

| Data Source | Field | Associates With | Phase |
|-------------|-------|-----------------|-------|
| LLMInteraction | `step_id` | Parent execution step | Phase 5b |
| ExecutionCheckpoint | `current_step.step_id` | Triggering step | ✅ Already available |
| ExecutionCheckpoint | `interrupt_point` | Checkpoint type | ✅ Already available |

---

## Next Steps

1. [x] Review and approve proposal
2. [x] Implement Phase 1: Execution Debug Store in orchestration module
   - [x] `execution_store.go` - Interfaces and implementation
   - [x] `noop_execution_store.go` - NoOp fallback
   - [x] `redis_execution_store.go` - Auto-configuring Redis backend with full feature parity (gzip, retry, circuit breaker)
   - [x] `execution_store_test.go` - Unit tests
   - [x] `core/redis_client.go` - Added `RedisDBExecutionDebug = 8`
   - [x] `factory.go` - Auto-configuration when enabled
3. [x] Phase 2 is now automatic (no application code needed!)
4. [x] Implement Phase 3: Registry Viewer API endpoints
5. [x] Implement Phase 4: DAG Visualization UI (basic steps view)
6. [x] **Phase 5a: Backend Data Enhancement** ✅ COMPLETE (January 2026)
   - [x] Add `Name` to `OrchestratorConfig` (interfaces.go)
   - [x] Add `AgentName` to `StoredExecution` and `ExecutionSummary` (execution_store.go)
   - [x] Add `getAgentName()` helper method (orchestrator.go)
   - [x] Update orchestrator storage calls in `ProcessRequest()` and `ProcessRequestStreaming()`
   - [x] Add env var parsing for `GOMIND_AGENT_NAME` (reuses existing env var)
   - [~] Add `CheckpointID` to `StoredExecution` - DEFERRED (can join by request_id)
7. [x] **Phase 5b: StepID for LLM Interactions** ✅ COMPLETE (January 2026)
   - [x] Add `StepID` field to `LLMInteraction` struct
   - [x] Update resolver call chain: executor → hybrid_resolver → micro_resolver
   - [x] Update contextual_re_resolver for semantic_retry StepID
   - See [Change 6](#change-6-add-stepid-to-llminteraction-for-step-specific-llm-calls) for full implementation details
8. [x] **Phase 5c: HITL Checkpoint Step Association** ✅ COMPLETE (January 2026) (UI Only)
   - [x] Add step-level checkpoint nodes (`before_step`, `after_step`, `on_error`) to DAG
   - [x] Connect checkpoint nodes to parent steps using `checkpoint.current_step.step_id`
   - [x] Add visual distinction: ⏸️ (before_step), ✓ (after_step), ⚠️ (on_error)
   - [x] Update legend to show step-level checkpoint types
   - **Data already available:** `ExecutionCheckpoint.current_step.step_id` and `interrupt_point`
9. [x] **Phase 5d: Registry Viewer API for Full Flow** ✅ COMPLETE (January 2026)
   - [x] `/api/executions/{request_id}/unified` endpoint already exists (reused)
   - [x] AgentName added to UnifiedExecutionView, StoredExecution, ExecutionSummary
   - [x] Checkpoint lookup by request_id already implemented
   - [x] LLM interactions already merged in buildUnifiedView()
10. [x] **Phase 5e: UI Full Flow Visualization** ✅ COMPLETE (January 2026)
    - [x] Add view toggle (Steps Only / Full Flow)
    - [x] Add new Cytoscape node styles: orchestrator (purple), llm_call (blue), checkpoint (orange/diamond), response (teal)
    - [x] Update legend to show different items based on view mode
    - [x] Update initCytoscape() with dagViewMode branching
    - [x] Add showFullFlowNodePopup() for new node type popups
    - [x] Full flow graph: Orchestrator → Planning LLM → [Checkpoint] → Steps → Synthesis LLM → Response
11. [ ] **Phase 5f: Associate Resolution LLM with Steps** (After Phase 5b)
    - [ ] Update UI to show Resolution nodes connected to their parent steps
    - [ ] Use `step_id` from LLMInteraction to create parent-child edges
12. [ ] Testing and documentation
13. [ ] Deploy and gather feedback
