# LLM Debug Payload Store - Production Design

## Table of Contents

- [Key Changes Summary](#key-changes-summary)
- [Implementation Status](#implementation-status)
- [Problem Statement](#problem-statement)
- [Design Goals](#design-goals)
- [Framework Principles Compliance](#framework-principles-compliance)
- [Feature Highlights](#feature-highlights)
- [Architecture](#architecture)
  - [Storage Layer](#storage-layer--implemented)
  - [Data Model](#data-model--implemented)
  - [Size Management](#size-management--implemented)
  - [TTL Policy](#ttl-policy--implemented)
- [Implementation](#implementation)
  - [1. LLM Debug Store Interface](#1-llm-debug-store-interface--implemented)
  - [2. Storage Implementations](#2-storage-implementations--implemented)
  - [3. Redis Implementation (Default)](#3-redis-implementation-default--implemented)
  - [4. Integration Points in Orchestrator](#4-integration-points-in-orchestrator--implemented)
    - [4.1 Orchestrator Integration](#41-orchestrator-integration--implemented)
    - [4.2 Synthesizer Integration](#42-synthesizer-integration--implemented)
    - [4.3 Micro Resolver Integration](#43-micro-resolver-integration--implemented)
    - [4.4 Contextual Re-Resolver Integration](#44-contextual-re-resolver-integration--implemented)
    - [4.5 Factory Wiring](#45-factory-wiring--implemented)
    - [4.6 Lifecycle Management](#46-lifecycle-management--implemented)
    - [4.7 Provider Field Implementation (Phase 1f)](#47-provider-field-implementation-phase-1f--implemented)
    - [4.8 Streaming Synthesis Recording (Phase 1g)](#48-streaming-synthesis-recording-phase-1g--implemented)
  - [5. Debug API Endpoint](#5-debug-api-endpoint-application-responsibility--not-implemented-phase-2)
  - [6. Configuration](#6-configuration-following-framework-principles--implemented)
- [Operator Configuration](#operator-configuration)
- [Usage](#usage)
- [Alternative: Enhanced Jaeger](#alternative-enhanced-jaeger-not-recommended)
- [Recommended Implementation Order](#recommended-implementation-order)
- [File Changes Summary](#file-changes-summary)
- [Estimated Effort](#estimated-effort)

---

## Key Changes Summary

> **Quick Reference** - For detailed implementation, see sections below.

### Overview

The LLM Debug Payload Store captures complete LLM request/response payloads during orchestration, enabling production debugging without truncation. This is an **orchestration-specific** feature (not available for agents using only `core` and `ai` modules).

### Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `GOMIND_LLM_DEBUG_ENABLED` | `false` | Enable/disable debug capture (disabled by default) |
| `GOMIND_LLM_DEBUG_TTL` | `24h` | TTL for successful records |
| `GOMIND_LLM_DEBUG_ERROR_TTL` | `168h` (7 days) | TTL for error records |
| `GOMIND_LLM_DEBUG_REDIS_DB` | `7` | Redis database index (`core.RedisDBLLMDebug`) |

### Recording Sites (6 Total)

| Site | Component | Description |
|------|-----------|-------------|
| `plan_generation` | Orchestrator | Initial plan generation from LLM |
| `correction` | Orchestrator | Plan correction after tool failures |
| `synthesis` | Synthesizer | Final response synthesis (non-streaming) |
| `synthesis_streaming` | Orchestrator | Final response synthesis (streaming) |
| `micro_resolution` | MicroResolver | Micro-step resolution |
| `semantic_retry` | ContextualReResolver | Semantic retry with full context |

### Data Model

```go
type LLMDebugRecord struct {
    RequestID    string              // Orchestration request identifier
    TraceID      string              // Links to distributed tracing (Jaeger)
    CreatedAt    time.Time           // When the first interaction was recorded
    UpdatedAt    time.Time           // When the record was last modified
    Interactions []LLMInteraction    // All LLM calls for this request
    Metadata     map[string]string   // Additional key-value pairs for investigation
}

type LLMInteraction struct {
    Type             string    // Recording site (e.g., "plan_generation")
    Timestamp        time.Time // When the call occurred
    DurationMs       int64     // Call duration
    Prompt           string    // Complete prompt sent to LLM
    SystemPrompt     string    // System prompt if used
    Temperature      float64   // Temperature setting
    MaxTokens        int       // Max tokens setting
    Model            string    // Model identifier (e.g., "gpt-4o")
    Provider         string    // Provider identifier (e.g., "openai", "anthropic")
    Response         string    // Complete response from LLM
    PromptTokens     int       // Token usage
    CompletionTokens int
    TotalTokens      int
    Success          bool      // Whether the call succeeded
    Error            string    // Error details if failed
    Attempt          int       // Attempt number (for retries)
}
```

### Key Files

| File | Purpose |
|------|---------|
| `orchestration/llm_debug_store.go` | `LLMDebugStore` interface, `LLMInteraction`, `LLMDebugRecord` types |
| `orchestration/redis_llm_debug_store.go` | Redis implementation with compression and resilience |
| `orchestration/noop_llm_debug_store.go` | Safe default when disabled |
| `orchestration/memory_llm_debug_store.go` | In-memory implementation for testing |
| `orchestration/factory.go` | Factory options: `WithLLMDebug()`, `WithLLMDebugStore()`, `WithLLMDebugTTL()`, `WithLLMDebugErrorTTL()` |
| `core/redis_client.go` | `RedisDBLLMDebug` constant, `IsReservedDB()` |

### Three-Layer Resilience

1. **Layer 1**: Built-in retry (3 attempts, 50ms backoff) for transient Redis failures
2. **Layer 2**: Optional circuit breaker via dependency injection
3. **Layer 3**: Automatic fallback to `NoOpLLMDebugStore` if Redis unavailable

### Provider Support

All AI clients set the `Provider` field in `core.AIResponse`:
- `openai` - Standard OpenAI API
- `openai.groq` - Groq via OpenAI-compatible API
- `openai.deepseek` - DeepSeek via OpenAI-compatible API
- `anthropic` - Anthropic Claude
- `gemini` - Google Gemini
- `bedrock` - AWS Bedrock

---

## Implementation Status

> **Last Updated**: 2026-01-12
> **Test Coverage**: ~90% for new LLM Debug code

| Phase | Component | Status | Notes |
|-------|-----------|--------|-------|
| **1a** | Core changes (`redis_client.go`) | ✅ Implemented | `RedisDBLLMDebug`, `IsReservedDB()`, warning |
| **1a** | Interface + types (`llm_debug_store.go`) | ✅ Implemented | `LLMDebugStore`, `LLMInteraction`, `LLMDebugRecord` |
| **1a** | Redis store (`redis_llm_debug_store.go`) | ✅ Implemented | Compression, TTL, Layer 1/2 resilience |
| **1a** | NoOp store (`noop_llm_debug_store.go`) | ✅ Implemented | Safe default |
| **1a** | Memory store (`memory_llm_debug_store.go`) | ✅ Implemented | For testing |
| **1a** | Factory options | ✅ Implemented | `WithLLMDebug()`, `WithLLMDebugStore()`, etc. |
| **1b** | Orchestrator base integration | ✅ Implemented | `debugStore` field, `SetLLMDebugStore()`, `GetLLMDebugStore()`, `recordDebugInteraction()` |
| **1b** | Plan generation call site | ✅ Implemented | Records at `plan_generation` LLM calls, includes `Model` field |
| **1b** | Correction call site | ✅ Implemented | Records at `correction` LLM calls, includes `Model` field |
| **1b** | Synthesizer integration | ✅ Implemented | `debugStore`, `debugWg`, `debugSeqID`, `logger` fields, setter, recording, includes `Model` field |
| **1b** | Micro resolver integration | ✅ Implemented | `debugStore`, `debugWg`, `debugSeqID` fields, setter, recording, includes `Model` field |
| **1b** | Contextual re-resolver integration | ✅ Implemented | `debugStore`, `debugWg`, `debugSeqID` fields, setter, recording, includes `Model` field |
| **1b** | Factory wiring for sub-components | ✅ Implemented | Propagate store via `SetLLMDebugStore()` |
| **1c** | Lifecycle management (WaitGroup) | ✅ Implemented | `debugWg sync.WaitGroup` in all components |
| **1c** | Shutdown method | ✅ Implemented | `Shutdown(ctx)` in orchestrator, synthesizer, micro_resolver, contextual_re_resolver |
| **1c** | Telemetry fallback (`debugSeqID`) | ✅ Implemented | `atomic.Uint64` counter for empty TraceID |
| **1d** | Unit tests (`llm_debug_store_test.go`) | ✅ Implemented | 22 tests covering stores, propagation, shutdown, factory options |
| **1e** | Model field population | ✅ Implemented | All successful LLM call sites now capture `Model` from `aiResponse.Model` |
| **1f** | Provider field population | ✅ Implemented | `Provider` field added to `core.AIResponse`, all AI clients (OpenAI, Anthropic, Gemini, Bedrock) set provider, all recording sites capture `Provider` |
| **1g** | Streaming synthesis recording | ✅ Implemented | `ProcessRequestStreaming` now records `synthesis_streaming` LLM interactions with full prompt/response |
| **2** | Debug API endpoint | ❌ Not Started | Application layer (travel-chat-agent) |
| **3** | Registry viewer UI | ✅ Implemented | UI tab with collapsible interactions in registry-viewer-app |

---

## Problem Statement

When running the orchestration module (e.g., `travel-chat-agent`), operators need to view:
1. The complete prompt sent to LLM for plan generation
2. The complete response received from LLM
3. Timing, token usage, and model information

Currently, Jaeger spans only show **truncated payloads** (1500-2000 chars), which is insufficient for production debugging.

## Design Goals

1. **Complete Payload Visibility**: Store full prompts and responses without truncation
2. **Request Correlation**: Query by `request_id` (generated per orchestration request)
3. **Low Overhead**: Minimal impact on orchestration latency
4. **Production-Ready**: TTL-based cleanup, size limits, compression for large payloads
5. **Integration**: Works with existing Redis infrastructure and telemetry

## Framework Principles Compliance

This design follows `FRAMEWORK_DESIGN_PRINCIPLES.md`:

| Principle | How This Design Complies |
|-----------|-------------------------|
| **Interface-First Design** | `LLMDebugStore` interface allows swappable backends |
| **WithXXX() Option Functions** | `WithLLMDebug()`, `WithLLMDebugStore()`, `WithDebugRedisURL()`, etc. |
| **Environment Variable Precedence** | `REDIS_URL` > `GOMIND_REDIS_URL` > defaults |
| **Intelligent Defaults** | Auto-uses Redis from discovery when available |
| **Safe Defaults** | `NoOpLLMDebugStore` when storage unavailable |
| **Disabled by Default** | Feature OFF; enable via `GOMIND_LLM_DEBUG_ENABLED=true` |
| **Operator Controlled** | Enable/disable via env var, code, or K8s ConfigMap |
| **Resilient Runtime** | Async recording, errors logged not propagated |
| **Three-Layer Resilience** | Layer 1: Built-in retry. Layer 2: Circuit breaker via `core.CircuitBreaker` interface (injected). Layer 3: NoOp fallback |
| **Module Dependency Compliance** | Only imports `core` + `telemetry`; does NOT import `resilience` module |
| **Actionable Error Messages** | Include env var hints in connection errors |
| **Handlers in Applications** | HTTP endpoints defined by apps, not framework |
| **Testability** | `MemoryLLMDebugStore` for unit tests |

## Feature Highlights

- **Disabled by default** - Enable via `GOMIND_LLM_DEBUG_ENABLED=true` or `WithLLMDebug(true)`
- **Interface-based design** - Supports Redis, PostgreSQL, S3, or custom backends via `LLMDebugStore` interface
- **Async recording** - No latency impact on orchestration; recording runs in goroutines
- **Three-layer resilience** - Layer 1: Built-in retry with backoff. Layer 2: Optional circuit breaker via dependency injection. Layer 3: NoOp fallback
- **Architecture compliant** - Only imports `core` + `telemetry`; does NOT import `resilience` module (circuit breaker injected via `core.CircuitBreaker` interface)
- **Compression** - Gzip for payloads over 100KB to reduce storage costs
- **TTL management** - 24h for successful records, 7 days for errors (configurable)
- **Complete payloads** - No truncation like Jaeger traces; full prompt and response stored
- **Trace correlation** - Links to Jaeger traces via `trace_id` for cross-referencing

## Architecture

### Storage Layer ✅ IMPLEMENTED

Use **Redis DB 7** via `core.RedisDBReserved7` (currently reserved for extensions).

When implementing, update `core/redis_client.go`:

#### 1. Add new constant
```go
// RedisDBLLMDebug is for LLM debug payload storage
RedisDBLLMDebug = 7
```

#### 2. Update `GetRedisDBName()` to return `"LLM Debug"` for DB 7

#### 3. Add reserved DB range constants and warning

```go
const (
    // RedisDBReservedStart marks the beginning of framework-reserved databases
    RedisDBReservedStart = 7

    // RedisDBReservedEnd marks the end of framework-reserved databases
    // Note: Redis default is 0-15 (16 DBs). Configure `databases` in redis.conf for more.
    RedisDBReservedEnd = 15
)

// IsReservedDB returns true if the DB number is reserved for framework extensions
func IsReservedDB(db int) bool {
    return db >= RedisDBReservedStart && db <= RedisDBReservedEnd
}
```

#### 4. Add warning in `NewRedisClient()` for reserved DB usage

```go
func NewRedisClient(opts RedisClientOptions) (*RedisClient, error) {
    // Warn if application is using a framework-reserved DB
    if IsReservedDB(opts.DB) {
        if opts.Logger != nil {
            opts.Logger.Warn("Using framework-reserved Redis DB", map[string]interface{}{
                "db":         opts.DB,
                "db_name":    GetRedisDBName(opts.DB),
                "reserved":   fmt.Sprintf("%d-%d", RedisDBReservedStart, RedisDBReservedEnd),
                "hint":       "DBs 7-15 are reserved for framework extensions. Use DBs 0-6 for application data.",
            })
        }
    }
    // ... continue with client creation
}
```

This warns developers but respects the **explicit override** principle - they can still use reserved DBs if needed.

```
Key Pattern: gomind:llm:debug:{request_id}
Value: JSON document with all LLM interactions
TTL: 24 hours (configurable, longer for errors)
```

### Data Model ✅ IMPLEMENTED

```go
// LLMDebugRecord stores all LLM interactions for a single orchestration request
type LLMDebugRecord struct {
    RequestID    string              `json:"request_id"`
    TraceID      string              `json:"trace_id"`
    CreatedAt    time.Time           `json:"created_at"`
    UpdatedAt    time.Time           `json:"updated_at"`
    Interactions []LLMInteraction    `json:"interactions"`
    Metadata     map[string]string   `json:"metadata,omitempty"`
}

// LLMInteraction captures a single LLM call
type LLMInteraction struct {
    Type          string            `json:"type"`           // "plan_generation", "synthesis", "micro_resolution", "correction", "semantic_retry"
    Timestamp     time.Time         `json:"timestamp"`
    DurationMs    int64             `json:"duration_ms"`

    // Request
    Prompt        string            `json:"prompt"`
    SystemPrompt  string            `json:"system_prompt,omitempty"`
    Temperature   float64           `json:"temperature"`
    MaxTokens     int               `json:"max_tokens"`
    Model         string            `json:"model,omitempty"`      // ✅ Populated from aiResponse.Model on success
    Provider      string            `json:"provider,omitempty"`   // ✅ Populated from aiResponse.Provider

    // Response
    Response      string            `json:"response"`
    PromptTokens  int               `json:"prompt_tokens"`
    CompletionTokens int            `json:"completion_tokens"`
    TotalTokens   int               `json:"total_tokens"`

    // Status
    Success       bool              `json:"success"`
    Error         string            `json:"error,omitempty"`
    Attempt       int               `json:"attempt"`        // For retries
}
```

### Size Management ✅ IMPLEMENTED

| Payload Size | Strategy |
|-------------|----------|
| < 100KB | Store as-is |
| 100KB - 1MB | Compress with gzip |
| > 1MB | Truncate middle, keep first/last 400KB each |

### TTL Policy ✅ IMPLEMENTED

| Outcome | TTL |
|---------|-----|
| Success | 24 hours |
| Error/Failure | 7 days |
| Manual flag for investigation | 30 days |

## Implementation

### 1. LLM Debug Store Interface ✅ IMPLEMENTED

```go
// File: orchestration/llm_debug_store.go

package orchestration

import (
    "context"
    "time"
)

// LLMDebugStore stores LLM interaction payloads for debugging
type LLMDebugStore interface {
    // RecordInteraction appends an LLM interaction to the debug record
    RecordInteraction(ctx context.Context, requestID string, interaction LLMInteraction) error

    // GetRecord retrieves the complete debug record for a request
    GetRecord(ctx context.Context, requestID string) (*LLMDebugRecord, error)

    // SetMetadata adds metadata to an existing record
    SetMetadata(ctx context.Context, requestID string, key, value string) error

    // ExtendTTL extends retention for investigation
    ExtendTTL(ctx context.Context, requestID string, duration time.Duration) error

    // ListRecent returns recent records (for UI listing)
    ListRecent(ctx context.Context, limit int) ([]LLMDebugRecordSummary, error)
}

// LLMDebugRecordSummary is a lightweight version for listing
type LLMDebugRecordSummary struct {
    RequestID      string    `json:"request_id"`
    TraceID        string    `json:"trace_id"`
    CreatedAt      time.Time `json:"created_at"`
    InteractionCount int     `json:"interaction_count"`
    TotalTokens    int       `json:"total_tokens"`
    HasErrors      bool      `json:"has_errors"`
}
```

### 2. Storage Implementations ✅ IMPLEMENTED

The framework provides a **Redis implementation** as the default, but teams can implement
the interface with any backend that suits their needs.

#### Supported Backends

| Backend | Implementation | Use Case |
|---------|---------------|----------|
| Redis | `RedisLLMDebugStore` (provided) | Default, fast lookups, already in infra |
| PostgreSQL | `PostgresLLMDebugStore` | Long-term retention, SQL queries, joins with other data |
| S3/GCS | `S3LLMDebugStore` | Cheap archival, compliance, large payloads |
| Elasticsearch | `ElasticLLMDebugStore` | Full-text search across prompts/responses |
| MongoDB | `MongoLLMDebugStore` | Document storage, flexible schema |
| In-memory | `MemoryLLMDebugStore` | Testing, development, short-lived debugging |
| File-based | `FileLLMDebugStore` | Simple setups, no external dependencies |

#### Example: PostgreSQL Implementation

```go
// File: your-app/postgres_llm_debug_store.go

package main

import (
    "context"
    "database/sql"
    "encoding/json"
    "time"

    "github.com/anthropics/gomind/orchestration"
)

type PostgresLLMDebugStore struct {
    db *sql.DB
}

func NewPostgresLLMDebugStore(db *sql.DB) *PostgresLLMDebugStore {
    return &PostgresLLMDebugStore{db: db}
}

func (s *PostgresLLMDebugStore) RecordInteraction(ctx context.Context, requestID string, interaction orchestration.LLMInteraction) error {
    interactionJSON, _ := json.Marshal(interaction)

    _, err := s.db.ExecContext(ctx, `
        INSERT INTO llm_debug_records (request_id, trace_id, created_at, updated_at, interactions)
        VALUES ($1, $2, NOW(), NOW(), $3::jsonb)
        ON CONFLICT (request_id) DO UPDATE SET
            updated_at = NOW(),
            interactions = llm_debug_records.interactions || $3::jsonb
    `, requestID, getTraceID(ctx), interactionJSON)

    return err
}

func (s *PostgresLLMDebugStore) GetRecord(ctx context.Context, requestID string) (*orchestration.LLMDebugRecord, error) {
    var record orchestration.LLMDebugRecord
    var interactionsJSON []byte

    err := s.db.QueryRowContext(ctx, `
        SELECT request_id, trace_id, created_at, updated_at, interactions
        FROM llm_debug_records WHERE request_id = $1
    `, requestID).Scan(&record.RequestID, &record.TraceID, &record.CreatedAt, &record.UpdatedAt, &interactionsJSON)

    if err != nil {
        return nil, err
    }
    json.Unmarshal(interactionsJSON, &record.Interactions)
    return &record, nil
}

// ... implement other interface methods
```

#### Example: In-Memory Implementation (for testing)

```go
// File: orchestration/memory_llm_debug_store.go (provided by framework for testing)

package orchestration

import (
    "context"
    "fmt"
    "sync"
    "time"
)

type MemoryLLMDebugStore struct {
    mu      sync.RWMutex
    records map[string]*LLMDebugRecord
}

func NewMemoryLLMDebugStore() *MemoryLLMDebugStore {
    return &MemoryLLMDebugStore{
        records: make(map[string]*LLMDebugRecord),
    }
}

func (s *MemoryLLMDebugStore) RecordInteraction(ctx context.Context, requestID string, interaction LLMInteraction) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    record, exists := s.records[requestID]
    if !exists {
        record = &LLMDebugRecord{
            RequestID: requestID,
            CreatedAt: time.Now(),
            Interactions: []LLMInteraction{},
        }
        s.records[requestID] = record
    }

    record.Interactions = append(record.Interactions, interaction)
    record.UpdatedAt = time.Now()
    return nil
}

func (s *MemoryLLMDebugStore) GetRecord(ctx context.Context, requestID string) (*LLMDebugRecord, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    record, exists := s.records[requestID]
    if !exists {
        return nil, fmt.Errorf("record not found: %s", requestID)
    }
    return record, nil
}

// ... implement other interface methods
```

#### Wiring Up Your Custom Implementation

```go
// In your application's main.go

func main() {
    // Option 1: Use provided Redis implementation
    debugStore, _ := orchestration.NewRedisLLMDebugStore(redisAddr, logger)

    // Option 2: Use your own PostgreSQL implementation
    db, _ := sql.Open("postgres", postgresConnStr)
    debugStore := NewPostgresLLMDebugStore(db)

    // Option 3: Use in-memory for testing
    debugStore := orchestration.NewMemoryLLMDebugStore()

    // Pass to orchestrator
    config := orchestration.DefaultConfig()
    config.LLMDebugStore = debugStore
    orch, _ := orchestration.CreateOrchestrator(config, deps)
}
```

---

### 3. Redis Implementation (Default) ✅ IMPLEMENTED

> **Architecture Compliance**: This implementation follows `ARCHITECTURE.md` strictly:
> - Does NOT import `resilience` module (only `core` + `telemetry` allowed)
> - Uses `core.CircuitBreaker` interface for optional injection
> - Provides built-in Layer 1 resilience (simple retry with backoff)
> - Circuit breaker is injected by application, not created internally

#### Three-Layer Resilience Architecture

| Layer | Description | Status |
|-------|-------------|--------|
| **Layer 1** | Built-in simple retry with exponential backoff | Always active |
| **Layer 2** | Circuit breaker (injected via `WithDebugCircuitBreaker`) | Optional |
| **Layer 3** | Fallback to `NoOpLLMDebugStore` on persistent failures | Handled by factory |

```go
// File: orchestration/redis_llm_debug_store.go

package orchestration

import (
    "bytes"
    "compress/gzip"
    "context"
    "encoding/json"
    "fmt"
    "os"
    "sync"
    "time"

    "github.com/go-redis/redis/v8"
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
    // NOTE: NO resilience import - per ARCHITECTURE.md
)

const (
    llmDebugKeyPrefix     = "gomind:llm:debug:"
    llmDebugIndexKey      = "gomind:llm:debug:index"
    defaultDebugTTL       = 24 * time.Hour
    errorDebugTTL         = 7 * 24 * time.Hour
    compressionThreshold  = 100 * 1024  // 100KB
    maxPayloadSize        = 1024 * 1024 // 1MB

    // Layer 1 Resilience Constants
    layer1MaxRetries     = 3
    layer1InitialBackoff = 100 * time.Millisecond
    layer1MaxBackoff     = 2 * time.Second
    layer1FailureWindow  = 30 * time.Second
    layer1MaxFailures    = 5
)

// RedisLLMDebugStoreOption configures the Redis debug store
type RedisLLMDebugStoreOption func(*redisDebugStoreConfig)

type redisDebugStoreConfig struct {
    redisURL       string
    redisDB        int
    logger         core.Logger
    circuitBreaker core.CircuitBreaker // Interface - injected by application (optional)
    ttl            time.Duration
    errorTTL       time.Duration
}

// WithDebugRedisURL sets the Redis connection URL
func WithDebugRedisURL(url string) RedisLLMDebugStoreOption {
    return func(c *redisDebugStoreConfig) {
        c.redisURL = url
    }
}

// WithDebugRedisDB sets the Redis database number (default: 7)
func WithDebugRedisDB(db int) RedisLLMDebugStoreOption {
    return func(c *redisDebugStoreConfig) {
        c.redisDB = db
    }
}

// WithDebugLogger sets the logger for debug store operations
func WithDebugLogger(logger core.Logger) RedisLLMDebugStoreOption {
    return func(c *redisDebugStoreConfig) {
        c.logger = logger
    }
}

// WithDebugCircuitBreaker sets a circuit breaker for Redis operations.
// The circuit breaker must implement core.CircuitBreaker interface.
// If not provided, built-in Layer 1 resilience (simple retry with backoff) is used.
// This follows ARCHITECTURE.md: circuit breaker is injected by application, not created internally.
func WithDebugCircuitBreaker(cb core.CircuitBreaker) RedisLLMDebugStoreOption {
    return func(c *redisDebugStoreConfig) {
        c.circuitBreaker = cb
    }
}

// WithDebugTTL sets custom TTL for successful debug records
func WithDebugTTL(ttl time.Duration) RedisLLMDebugStoreOption {
    return func(c *redisDebugStoreConfig) {
        c.ttl = ttl
    }
}

// WithDebugErrorTTL sets custom TTL for error debug records
func WithDebugErrorTTL(ttl time.Duration) RedisLLMDebugStoreOption {
    return func(c *redisDebugStoreConfig) {
        c.errorTTL = ttl
    }
}

// RedisLLMDebugStore is a Redis-backed implementation of LLMDebugStore.
// Resilience follows the Three-Layer Architecture from ARCHITECTURE.md:
// - Layer 1: Built-in simple retry with exponential backoff (always active)
// - Layer 2: Optional circuit breaker (injected via WithDebugCircuitBreaker)
// - Layer 3: Fallback to NoOp on persistent failures (handled by factory)
type RedisLLMDebugStore struct {
    client         *redis.Client
    logger         core.Logger
    circuitBreaker core.CircuitBreaker // Optional - injected by application
    ttl            time.Duration
    errorTTL       time.Duration

    // Layer 1 resilience state (simple failure tracking)
    failureCount int
    failureMu    sync.Mutex
    lastFailure  time.Time
}

// NewRedisLLMDebugStore creates a Redis-backed debug store with intelligent defaults.
// Environment variable precedence: explicit options > REDIS_URL > GOMIND_REDIS_URL > localhost:6379
func NewRedisLLMDebugStore(opts ...RedisLLMDebugStoreOption) (*RedisLLMDebugStore, error) {
    // Apply intelligent defaults
    cfg := &redisDebugStoreConfig{
        redisURL: getRedisURLWithFallback(),
        redisDB:  getEnvInt("GOMIND_LLM_DEBUG_REDIS_DB", core.RedisDBLLMDebug),
        logger:   &core.NoOpLogger{},
        ttl:      getEnvDuration("GOMIND_LLM_DEBUG_TTL", defaultDebugTTL),
        errorTTL: getEnvDuration("GOMIND_LLM_DEBUG_ERROR_TTL", errorDebugTTL),
    }

    // Apply explicit options (override defaults)
    for _, opt := range opts {
        opt(cfg)
    }

    // Parse Redis URL and create client
    redisOpt, err := redis.ParseURL(cfg.redisURL)
    if err != nil {
        // Try treating it as a simple address if URL parsing fails
        redisOpt = &redis.Options{
            Addr: cfg.redisURL,
        }
    }
    redisOpt.DB = cfg.redisDB

    client := redis.NewClient(redisOpt)

    // Verify connection with actionable error message
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("redis connection failed at %s (DB %d): %w\n"+
            "Hint: Check REDIS_URL or GOMIND_REDIS_URL environment variables, "+
            "or use WithDebugRedisURL() option", cfg.redisURL, cfg.redisDB, err)
    }

    // Note: Circuit breaker is optional and injected by application (per ARCHITECTURE.md)
    // If not provided, built-in Layer 1 resilience (simple retry) is used

    cfg.logger.Info("Redis LLM debug store initialized", map[string]interface{}{
        "redis_addr":      redisOpt.Addr,
        "redis_db":        cfg.redisDB,
        "ttl":             cfg.ttl.String(),
        "error_ttl":       cfg.errorTTL.String(),
        "circuit_breaker": cfg.circuitBreaker != nil,
        "resilience":      "layer1_builtin",
    })

    return &RedisLLMDebugStore{
        client:         client,
        logger:         cfg.logger,
        circuitBreaker: cfg.circuitBreaker,
        ttl:            cfg.ttl,
        errorTTL:       cfg.errorTTL,
    }, nil
}

// executeWithRetry implements Layer 1 built-in resilience with simple retry and exponential backoff.
// This is always available, even without an injected circuit breaker.
// Per ARCHITECTURE.md Layer 1: "3 retries with exponential backoff, simple failure tracking"
func (s *RedisLLMDebugStore) executeWithRetry(ctx context.Context, operation func() error) error {
    // Check if we're in cooldown due to too many failures
    s.failureMu.Lock()
    if s.failureCount >= layer1MaxFailures && time.Since(s.lastFailure) < layer1FailureWindow {
        s.failureMu.Unlock()
        return fmt.Errorf("debug store in cooldown after %d failures", s.failureCount)
    }
    s.failureMu.Unlock()

    var lastErr error
    backoff := layer1InitialBackoff

    for attempt := 1; attempt <= layer1MaxRetries; attempt++ {
        if err := operation(); err == nil {
            // Success - reset failure count
            s.failureMu.Lock()
            s.failureCount = 0
            s.failureMu.Unlock()
            return nil
        } else {
            lastErr = err
        }

        // Exponential backoff (except on last attempt)
        if attempt < layer1MaxRetries {
            time.Sleep(backoff)
            backoff *= 2
            if backoff > layer1MaxBackoff {
                backoff = layer1MaxBackoff
            }
        }
    }

    // All retries failed - track failure
    s.failureMu.Lock()
    s.failureCount++
    s.lastFailure = time.Now()
    s.failureMu.Unlock()

    return fmt.Errorf("operation failed after %d attempts: %w", layer1MaxRetries, lastErr)
}

// RecordInteraction uses Layer 2 circuit breaker if injected, otherwise Layer 1 retry
func (s *RedisLLMDebugStore) RecordInteraction(ctx context.Context, requestID string, interaction LLMInteraction) error {
    operation := func() error {
        // ... Redis operations ...
        return nil
    }

    // Layer 2: Use injected circuit breaker if available
    if s.circuitBreaker != nil {
        return s.circuitBreaker.Execute(ctx, operation)
    }

    // Layer 1: Built-in simple retry with exponential backoff
    return s.executeWithRetry(ctx, operation)
}

// getRedisURLWithFallback returns Redis URL with environment variable precedence
func getRedisURLWithFallback() string {
    if url := os.Getenv("REDIS_URL"); url != "" {
        return url
    }
    if url := os.Getenv("GOMIND_REDIS_URL"); url != "" {
        return url
    }
    return "localhost:6379"
}

// getEnvInt parses an integer from environment variable with fallback
func getEnvInt(key string, defaultVal int) int {
    if val := os.Getenv(key); val != "" {
        if result, err := strconv.Atoi(val); err == nil {
            return result
        }
    }
    return defaultVal
}

// getEnvDuration parses a duration from environment variable with fallback
func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
    if val := os.Getenv(key); val != "" {
        if result, err := time.ParseDuration(val); err == nil {
            return result
        }
    }
    return defaultVal
}

func (s *RedisLLMDebugStore) RecordInteraction(ctx context.Context, requestID string, interaction LLMInteraction) error {
    // Use circuit breaker to protect against Redis failures
    return s.circuitBreaker.Execute(ctx, func() error {
        key := llmDebugKeyPrefix + requestID

        // Get or create record
        record, err := s.getOrCreateRecord(ctx, key, requestID)
        if err != nil {
            s.logger.Warn("Failed to get debug record, creating new", map[string]interface{}{
                "request_id": requestID,
                "error":      err.Error(),
            })
            // Create fresh record on error (don't fail the whole operation)
            record = &LLMDebugRecord{
                RequestID:    requestID,
                CreatedAt:    time.Now(),
                Interactions: []LLMInteraction{},
            }
        }

        // Append interaction
        record.Interactions = append(record.Interactions, interaction)
        record.UpdatedAt = time.Now()

        // Serialize with optional compression
        data, err := s.serialize(record)
        if err != nil {
            return fmt.Errorf("serialization failed: %w", err)
        }

        // Determine TTL based on success/error
        ttl := defaultDebugTTL
        if !interaction.Success {
            ttl = errorDebugTTL
        }

        // Store
        if err := s.client.Set(ctx, key, data, ttl).Err(); err != nil {
            return fmt.Errorf("redis set failed at %s: %w", s.client.Options().Addr, err)
        }

        // Update index for listing (sorted set by timestamp) - best effort
        if err := s.client.ZAdd(ctx, llmDebugIndexKey, redis.Z{
            Score:  float64(record.CreatedAt.Unix()),
            Member: requestID,
        }).Err(); err != nil {
            s.logger.Warn("Failed to update debug index", map[string]interface{}{
                "request_id": requestID,
                "error":      err.Error(),
            })
            // Don't fail - index is for convenience, not critical
        }

        return nil
    })
}

func (s *RedisLLMDebugStore) GetRecord(ctx context.Context, requestID string) (*LLMDebugRecord, error) {
    key := llmDebugKeyPrefix + requestID

    data, err := s.client.Get(ctx, key).Bytes()
    if err == redis.Nil {
        return nil, fmt.Errorf("record not found: %s", requestID)
    }
    if err != nil {
        return nil, fmt.Errorf("redis get failed: %w", err)
    }

    return s.deserialize(data)
}

// SetMetadata adds metadata to an existing record.
func (s *RedisLLMDebugStore) SetMetadata(ctx context.Context, requestID string, key, value string) error {
    return s.circuitBreaker.Execute(ctx, func() error {
        redisKey := llmDebugKeyPrefix + requestID

        record, err := s.GetRecord(ctx, requestID)
        if err != nil {
            return err
        }

        if record.Metadata == nil {
            record.Metadata = make(map[string]string)
        }
        record.Metadata[key] = value
        record.UpdatedAt = time.Now()

        data, err := s.serialize(record)
        if err != nil {
            return err
        }

        // Get current TTL to preserve it
        ttl, err := s.client.TTL(ctx, redisKey).Result()
        if err != nil || ttl < 0 {
            ttl = s.ttl
        }

        return s.client.Set(ctx, redisKey, data, ttl).Err()
    })
}

// ExtendTTL extends retention for investigation.
func (s *RedisLLMDebugStore) ExtendTTL(ctx context.Context, requestID string, duration time.Duration) error {
    key := llmDebugKeyPrefix + requestID
    return s.client.Expire(ctx, key, duration).Err()
}

func (s *RedisLLMDebugStore) ListRecent(ctx context.Context, limit int) ([]LLMDebugRecordSummary, error) {
    // Get recent request IDs from sorted set (newest first)
    ids, err := s.client.ZRevRange(ctx, llmDebugIndexKey, 0, int64(limit-1)).Result()
    if err != nil {
        return nil, err
    }

    summaries := make([]LLMDebugRecordSummary, 0, len(ids))
    for _, id := range ids {
        record, err := s.GetRecord(ctx, id)
        if err != nil {
            continue // Skip missing records (TTL expired)
        }

        totalTokens := 0
        hasErrors := false
        for _, interaction := range record.Interactions {
            totalTokens += interaction.TotalTokens
            if !interaction.Success {
                hasErrors = true
            }
        }

        summaries = append(summaries, LLMDebugRecordSummary{
            RequestID:        record.RequestID,
            TraceID:          record.TraceID,
            CreatedAt:        record.CreatedAt,
            InteractionCount: len(record.Interactions),
            TotalTokens:      totalTokens,
            HasErrors:        hasErrors,
        })
    }

    return summaries, nil
}

// Close closes the Redis connection.
func (s *RedisLLMDebugStore) Close() error {
    return s.client.Close()
}

// serialize with optional gzip compression
func (s *RedisLLMDebugStore) serialize(record *LLMDebugRecord) ([]byte, error) {
    data, err := json.Marshal(record)
    if err != nil {
        return nil, err
    }

    // Compress if over threshold
    if len(data) > compressionThreshold {
        var buf bytes.Buffer
        buf.WriteByte(1) // Compression flag
        gz := gzip.NewWriter(&buf)
        if _, err := gz.Write(data); err != nil {
            return nil, err
        }
        gz.Close()
        return buf.Bytes(), nil
    }

    // Prepend 0 byte to indicate no compression
    return append([]byte{0}, data...), nil
}

// deserialize with optional gzip decompression
func (s *RedisLLMDebugStore) deserialize(data []byte) (*LLMDebugRecord, error) {
    if len(data) == 0 {
        return nil, fmt.Errorf("empty data")
    }

    var jsonData []byte
    if data[0] == 1 { // Compressed
        gz, err := gzip.NewReader(bytes.NewReader(data[1:]))
        if err != nil {
            return nil, err
        }
        defer gz.Close()

        var buf bytes.Buffer
        if _, err := buf.ReadFrom(gz); err != nil {
            return nil, err
        }
        jsonData = buf.Bytes()
    } else {
        jsonData = data[1:]
    }

    var record LLMDebugRecord
    if err := json.Unmarshal(jsonData, &record); err != nil {
        return nil, err
    }
    return &record, nil
}

func (s *RedisLLMDebugStore) getOrCreateRecord(ctx context.Context, key, requestID string) (*LLMDebugRecord, error) {
    data, err := s.client.Get(ctx, key).Bytes()
    if err == redis.Nil {
        // Create new record
        tc := GetTraceContext(ctx)
        return &LLMDebugRecord{
            RequestID:    requestID,
            TraceID:      tc.TraceID,
            CreatedAt:    time.Now(),
            UpdatedAt:    time.Now(),
            Interactions: []LLMInteraction{},
        }, nil
    }
    if err != nil {
        return nil, err
    }
    return s.deserialize(data)
}
```

### 4. Integration Points in Orchestrator ✅ IMPLEMENTED

Per `FRAMEWORK_DESIGN_PRINCIPLES.md` - **Resilient Runtime Behavior**:
> "Runtime problems should be handled gracefully... If telemetry fails, continue processing"

The same principle applies to debug storage. Debug failures must **never** block orchestration.

#### Framework LLM Call Sites

The orchestration module makes LLM calls at 5 distinct points:

| Call Site | File | Type | Purpose |
|-----------|------|------|---------|
| Plan Generation | `orchestrator.go` | `plan_generation` | Generate execution plan from user request |
| Parameter Correction | `orchestrator.go` | `correction` | Fix type errors in tool parameters (Layer 3) |
| Synthesis | `synthesizer.go` | `synthesis` | Combine agent responses into final answer |
| Micro Resolution | `micro_resolver.go` | `micro_resolution` | Resolve missing parameters via LLM |
| Contextual Re-Resolution | `contextual_re_resolver.go` | `semantic_retry` | Re-resolve parameters after validation errors (Layer 4) |

#### 4.1 Orchestrator Integration ✅ IMPLEMENTED

Modify `orchestrator.go` to use the debug store:

```go
// Helper function for resilient debug recording
func (o *Orchestrator) recordDebugInteraction(ctx context.Context, requestID string, interaction LLMInteraction) {
    if o.debugStore == nil {
        return
    }

    // Run async to avoid blocking orchestration
    go func() {
        if err := o.debugStore.RecordInteraction(ctx, requestID, interaction); err != nil {
            // Log but don't fail - debug is observability, not critical path
            o.logger.Warn("Failed to record LLM debug interaction", map[string]interface{}{
                "request_id": requestID,
                "type":       interaction.Type,
                "error":      err.Error(),
            })
        }
    }()
}
```

##### Plan Generation (generateExecutionPlan)

```go
// After LLM response received (success):
o.recordDebugInteraction(ctx, requestID, LLMInteraction{
    Type:             "plan_generation",
    Timestamp:        llmStartTime,
    DurationMs:       llmDuration.Milliseconds(),
    Prompt:           prompt,
    SystemPrompt:     systemPrompt,
    Temperature:      0.3,
    MaxTokens:        2000,
    Model:            aiResponse.Model,  // ✅ Capture model from AI response
    Response:         aiResponse.Content,
    PromptTokens:     aiResponse.Usage.PromptTokens,
    CompletionTokens: aiResponse.Usage.CompletionTokens,
    TotalTokens:      aiResponse.Usage.TotalTokens,
    Success:          true,
    Attempt:          attempt,
})

// On LLM error:
o.recordDebugInteraction(ctx, requestID, LLMInteraction{
    Type:       "plan_generation",
    Timestamp:  llmStartTime,
    DurationMs: llmDuration.Milliseconds(),
    Prompt:     prompt,  // Include prompt for debugging failed requests
    Success:    false,
    Error:      err.Error(),
    Attempt:    attempt,
})
```

##### Parameter Correction (requestParameterCorrection)

```go
// In requestParameterCorrection() - after LLM call (success):
o.recordDebugInteraction(ctx, requestID, LLMInteraction{
    Type:             "correction",
    Timestamp:        llmStartTime,
    DurationMs:       llmDuration.Milliseconds(),
    Prompt:           correctionPrompt,
    Temperature:      0.2,
    MaxTokens:        500,
    Model:            response.Model,  // ✅ Capture model from AI response
    Response:         response.Content,
    PromptTokens:     response.Usage.PromptTokens,
    CompletionTokens: response.Usage.CompletionTokens,
    TotalTokens:      response.Usage.TotalTokens,
    Success:          true,
    Attempt:          1,
})
```

#### 4.2 Synthesizer Integration ✅ IMPLEMENTED

The `AISynthesizer` needs access to the debug store via dependency injection.

```go
// File: orchestration/synthesizer.go

// CURRENT struct in codebase:
// type AISynthesizer struct {
//     aiClient core.AIClient
//     strategy SynthesisStrategy
// }

// MODIFIED struct (add debugStore and logger fields):
type AISynthesizer struct {
    aiClient   core.AIClient
    strategy   SynthesisStrategy
    debugStore LLMDebugStore  // NEW: for debug recording
    logger     core.Logger    // NEW: for logging errors
}

// Add setter methods:
func (s *AISynthesizer) SetLLMDebugStore(store LLMDebugStore) {
    s.debugStore = store
}

func (s *AISynthesizer) SetLogger(logger core.Logger) {
    s.logger = logger
}

// Add helper method:
func (s *AISynthesizer) recordDebugInteraction(ctx context.Context, interaction LLMInteraction) {
    if s.debugStore == nil {
        return
    }
    requestID := telemetry.GetTraceContext(ctx).TraceID // Use trace ID as request correlation
    go func() {
        if err := s.debugStore.RecordInteraction(ctx, requestID, interaction); err != nil {
            if s.logger != nil {
                s.logger.Warn("Failed to record synthesis debug", map[string]interface{}{
                    "error": err.Error(),
                })
            }
        }
    }()
}

// In synthesizeWithLLM() - after LLM call (success), add:
s.recordDebugInteraction(ctx, LLMInteraction{
    Type:             "synthesis",
    Timestamp:        llmStartTime,
    DurationMs:       llmDuration.Milliseconds(),
    Prompt:           prompt,
    SystemPrompt:     "You are an AI that synthesizes multiple agent responses...",
    Temperature:      0.5,
    MaxTokens:        1500,
    Model:            aiResponse.Model,  // ✅ Capture model from AI response
    Response:         aiResponse.Content,
    PromptTokens:     aiResponse.Usage.PromptTokens,
    CompletionTokens: aiResponse.Usage.CompletionTokens,
    TotalTokens:      aiResponse.Usage.TotalTokens,
    Success:          true,
    Attempt:          1,
})
```

#### 4.3 Micro Resolver Integration ✅ IMPLEMENTED

The `MicroResolver` needs access to the debug store via dependency injection.

```go
// File: orchestration/micro_resolver.go

// CURRENT struct in codebase:
// type MicroResolver struct {
//     aiClient       core.AIClient
//     functionClient FunctionCallingClient  // Note: field is "functionClient" not "fcClient"
//     logger         core.Logger
// }

// MODIFIED struct (add debugStore field):
type MicroResolver struct {
    aiClient       core.AIClient
    functionClient FunctionCallingClient
    logger         core.Logger
    debugStore     LLMDebugStore  // NEW: for debug recording
}

// Add setter method:
func (m *MicroResolver) SetLLMDebugStore(store LLMDebugStore) {
    m.debugStore = store
}

// Add helper method:
func (m *MicroResolver) recordDebugInteraction(ctx context.Context, interaction LLMInteraction) {
    if m.debugStore == nil {
        return
    }
    requestID := telemetry.GetTraceContext(ctx).TraceID
    go func() {
        if err := m.debugStore.RecordInteraction(ctx, requestID, interaction); err != nil {
            if m.logger != nil {
                m.logger.Warn("Failed to record micro_resolution debug", map[string]interface{}{
                    "error": err.Error(),
                })
            }
        }
    }()
}

// In resolveWithText() - after LLM call (success), add:
m.recordDebugInteraction(ctx, LLMInteraction{
    Type:             "micro_resolution",
    Timestamp:        llmStartTime,
    DurationMs:       llmDuration.Milliseconds(),
    Prompt:           prompt,
    Temperature:      0.0,
    MaxTokens:        500,
    Model:            resp.Model,  // ✅ Capture model from AI response
    Response:         resp.Content,
    PromptTokens:     resp.Usage.PromptTokens,
    CompletionTokens: resp.Usage.CompletionTokens,
    TotalTokens:      resp.Usage.TotalTokens,
    Success:          true,
    Attempt:          1,
})
```

#### 4.4 Contextual Re-Resolver Integration ✅ IMPLEMENTED

The `ContextualReResolver` needs access to the debug store via dependency injection.

```go
// File: orchestration/contextual_re_resolver.go

// CURRENT struct in codebase:
// type ContextualReResolver struct {
//     aiClient core.AIClient
//     logger   core.Logger
// }

// MODIFIED struct (add debugStore field):
type ContextualReResolver struct {
    aiClient   core.AIClient
    logger     core.Logger
    debugStore LLMDebugStore  // NEW: for debug recording
}

// Add setter method:
func (r *ContextualReResolver) SetLLMDebugStore(store LLMDebugStore) {
    r.debugStore = store
}

// Add helper method:
func (r *ContextualReResolver) recordDebugInteraction(ctx context.Context, interaction LLMInteraction) {
    if r.debugStore == nil {
        return
    }
    requestID := telemetry.GetTraceContext(ctx).TraceID
    go func() {
        if err := r.debugStore.RecordInteraction(ctx, requestID, interaction); err != nil {
            if r.logger != nil {
                r.logger.Warn("Failed to record semantic_retry debug", map[string]interface{}{
                    "error": err.Error(),
                })
            }
        }
    }()
}

// In ReResolve() - after LLM call (success), add:
r.recordDebugInteraction(ctx, LLMInteraction{
    Type:             "semantic_retry",
    Timestamp:        startTime,
    DurationMs:       duration.Milliseconds(),
    Prompt:           prompt,
    Temperature:      0.0,
    MaxTokens:        1000,
    Model:            response.Model,  // ✅ Capture model from AI response
    Response:         response.Content,
    PromptTokens:     response.Usage.PromptTokens,
    CompletionTokens: response.Usage.CompletionTokens,
    TotalTokens:      response.Usage.TotalTokens,
    Success:          true,
    Attempt:          execCtx.RetryCount + 1,
})
```

#### 4.5 Factory Wiring ✅ IMPLEMENTED

Update `factory.go` to propagate the debug store to all components.

**Current component access paths** (based on actual codebase structure):
- `orchestrator.synthesizer` → `*AISynthesizer`
- `orchestrator.executor` → `*SmartExecutor`
- `orchestrator.executor.hybridResolver` → `*HybridResolver`
- `orchestrator.executor.hybridResolver.microResolver` → `*MicroResolver`
- `orchestrator.executor.contextualReResolver` → `*ContextualReResolver`

```go
// In CreateOrchestrator() - after setting up debug store:
if config.LLMDebugStore != nil {
    // Orchestrator (already done)
    orchestrator.SetLLMDebugStore(config.LLMDebugStore)

    // Synthesizer
    if orchestrator.synthesizer != nil {
        orchestrator.synthesizer.SetLLMDebugStore(config.LLMDebugStore)
        orchestrator.synthesizer.SetLogger(deps.Logger)
    }

    // Executor sub-components
    if orchestrator.executor != nil {
        // HybridResolver -> MicroResolver
        if orchestrator.executor.hybridResolver != nil {
            if orchestrator.executor.hybridResolver.microResolver != nil {
                orchestrator.executor.hybridResolver.microResolver.SetLLMDebugStore(config.LLMDebugStore)
            }
        }

        // ContextualReResolver
        if orchestrator.executor.contextualReResolver != nil {
            orchestrator.executor.contextualReResolver.SetLLMDebugStore(config.LLMDebugStore)
        }
    }
}
```

#### 4.6 Lifecycle Management ✅ IMPLEMENTED

Per `FRAMEWORK_DESIGN_PRINCIPLES.md` - **Component Lifecycle Rules**:
> "All components must handle context cancellation... Close external connections cleanly"

##### Goroutine Lifecycle with WaitGroup

Async recording goroutines must complete before shutdown to prevent data loss.
Add a `sync.WaitGroup` to track in-flight recordings:

```go
// File: orchestration/orchestrator.go

import (
    "sync/atomic"
)

type AIOrchestrator struct {
    // ... existing fields ...
    debugStore    LLMDebugStore
    debugWg       sync.WaitGroup  // NEW: tracks in-flight debug recordings
    debugSeqID    atomic.Uint64   // NEW: for generating unique fallback IDs
}

// recordDebugInteraction stores an LLM interaction for debugging.
// Uses WaitGroup to ensure graceful shutdown waits for pending recordings.
func (o *AIOrchestrator) recordDebugInteraction(ctx context.Context, requestID string, interaction LLMInteraction) {
    if o.debugStore == nil {
        return
    }

    o.debugWg.Add(1)  // Track this goroutine
    go func() {
        defer o.debugWg.Done()  // Signal completion

        // Use background context to allow completion even if request ctx is cancelled
        recordCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        if err := o.debugStore.RecordInteraction(recordCtx, requestID, interaction); err != nil {
            if o.logger != nil {
                o.logger.Warn("Failed to record LLM debug interaction", map[string]interface{}{
                    "request_id": requestID,
                    "type":       interaction.Type,
                    "error":      err.Error(),
                })
            }
        }
    }()
}
```

##### Graceful Shutdown

The orchestrator must wait for pending recordings, then close the debug store:

```go
// Shutdown waits for pending debug recordings before closing resources.
// The ctx parameter controls the maximum wait time for pending recordings.
func (o *AIOrchestrator) Shutdown(ctx context.Context) error {
    var errs []error

    // Wait for in-flight debug recordings (with timeout from ctx)
    done := make(chan struct{})
    go func() {
        o.debugWg.Wait()
        close(done)
    }()

    select {
    case <-done:
        // All recordings completed successfully
    case <-ctx.Done():
        if o.logger != nil {
            o.logger.Warn("Shutdown timeout: some debug recordings may be lost", nil)
        }
        errs = append(errs, fmt.Errorf("debug recording timeout: %w", ctx.Err()))
    }

    // Close debug store if it implements io.Closer
    if o.debugStore != nil {
        if closer, ok := o.debugStore.(io.Closer); ok {
            if err := closer.Close(); err != nil {
                errs = append(errs, fmt.Errorf("debug store close failed: %w", err))
            }
        }
    }

    // ... close other components (synthesizer, executor, etc.) ...

    if len(errs) > 0 {
        return fmt.Errorf("shutdown errors: %v", errs)
    }
    return nil
}
```

**Same pattern applies to sub-components** (Synthesizer, MicroResolver, ContextualReResolver):

```go
// Each component that records debug interactions should have:
type AISynthesizer struct {
    // ... existing fields ...
    debugStore LLMDebugStore
    debugWg    sync.WaitGroup  // Track in-flight recordings
    debugSeqID atomic.Uint64   // For fallback ID generation
    logger     core.Logger
}

// Shutdown waits for pending recordings with a timeout.
// Should be called by orchestrator's Shutdown() method.
func (s *AISynthesizer) Shutdown(ctx context.Context) error {
    done := make(chan struct{})
    go func() {
        s.debugWg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return fmt.Errorf("synthesizer shutdown timeout: %w", ctx.Err())
    }
}
```

##### Telemetry Nil-Safety

Per `FRAMEWORK_DESIGN_PRINCIPLES.md` - **Telemetry Architecture Pattern**:
> "Never assume telemetry is initialized"

The `telemetry.GetTraceContext()` function is **already nil-safe** - it returns a zero-value
`TraceContext{}` when ctx is nil or no span is active. However, `TraceID` will be an empty
string in these cases.

**Handle empty TraceID gracefully using atomic counter (avoids collisions):**

```go
func (s *AISynthesizer) recordDebugInteraction(ctx context.Context, interaction LLMInteraction) {
    if s.debugStore == nil {
        return
    }

    // GetTraceContext is nil-safe, returns empty TraceID if no span
    tc := telemetry.GetTraceContext(ctx)
    requestID := tc.TraceID
    if requestID == "" {
        // Generate unique fallback ID using atomic counter (collision-safe)
        seq := s.debugSeqID.Add(1)
        requestID = fmt.Sprintf("no-trace-%d-%d", time.Now().Unix(), seq)
    }

    s.debugWg.Add(1)
    go func() {
        defer s.debugWg.Done()

        recordCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        if err := s.debugStore.RecordInteraction(recordCtx, requestID, interaction); err != nil {
            if s.logger != nil {
                s.logger.Warn("Failed to record debug interaction", map[string]interface{}{
                    "request_id": requestID,
                    "type":       interaction.Type,
                    "error":      err.Error(),
                })
            }
        }
    }()
}
```

**Key Design Decisions:**
1. **Async recording** - Debug storage runs in a goroutine to avoid latency impact
2. **Fail silently** - Errors are logged, not propagated
3. **Always safe** - nil check on debugStore happens first
4. **Error capture** - Failed LLM calls include the prompt for debugging
5. **Dependency injection** - Debug store passed to sub-components via SetLLMDebugStore()
6. **Trace correlation** - Components without requestID use TraceID for correlation (with fallback)
7. **Accurate field names** - Uses actual field names from codebase (e.g., `functionClient`, `hybridResolver`, `contextualReResolver`)
8. **Graceful shutdown** - WaitGroup ensures pending recordings complete before exit
9. **Telemetry nil-safe** - Handles empty TraceID when telemetry not initialized

### 4.7 Provider Field Implementation (Phase 1f) ✅ IMPLEMENTED

The `LLMInteraction.Provider` field captures provider information (OpenAI, Anthropic, Gemini, Bedrock) at the framework level.

> **Note on OpenAI-Compatible Providers**: Groq, DeepSeek, and other OpenAI-compatible providers are NOT separate AI clients. They use the OpenAI client with a `providerAlias` field (e.g., `"openai.groq"`, `"openai.deepseek"`). The Provider field reflects the actual provider alias, enabling filtering by the real provider.

#### Implementation Summary

**Changes Made:**
1. Added `Provider string` field to `core.AIResponse` struct in `core/interfaces.go`
2. Updated all AI clients to set `Provider` in their responses:
   - **OpenAI**: Uses `c.getProviderName()` helper (returns `providerAlias` or `"openai"` fallback)
   - **Anthropic**: Sets `Provider: "anthropic"`
   - **Gemini**: Sets `Provider: "gemini"`
   - **Bedrock**: Sets `Provider: "bedrock"`
3. All LLM debug recording sites now capture `Provider: aiResponse.Provider`
4. Unit tests added in `ai/providers/openai/client_test.go` for `getProviderName()`

#### Original Problem (Solved)

The `Provider` field in `LLMInteraction` struct was defined but always empty:
```go
type LLMInteraction struct {
    // ...
    Provider      string            `json:"provider,omitempty"`   // ❌ Not yet populated
    // ...
}
```

This makes it difficult to:
1. Filter debug records by AI provider
2. Compare performance across providers
3. Audit which provider handled specific requests

#### Solution: Add Provider to core.AIResponse

The provider information should be captured at the source - in the `core.AIResponse` struct returned by each AI client.

---

##### Step 1: Modify core/interfaces.go (Line 76-80)

**File**: `core/interfaces.go`
**Current code** (lines 76-80):
```go
// AIResponse from AI client
type AIResponse struct {
	Content string
	Model   string
	Usage   TokenUsage
}
```

**Change to**:
```go
// AIResponse from AI client
type AIResponse struct {
	Content  string
	Model    string
	Provider string     // NEW: "openai", "openai.groq", "openai.deepseek", "anthropic", "gemini", "bedrock"
	Usage    TokenUsage
}
```

---

##### Step 2: Update AI Client Implementations

Each AI client must set the `Provider` field when returning responses.

---

###### 2a. OpenAI Client (and OpenAI-Compatible Providers)

**File**: `ai/providers/openai/client.go`

The OpenAI client has a `providerAlias` field (line 23) that identifies the actual provider:
- `"openai"` for direct OpenAI API
- `"openai.groq"` for Groq
- `"openai.deepseek"` for DeepSeek
- etc.

> **Safety Note**: We add a helper method `getProviderName()` to handle the edge case where `providerAlias` might be empty. This follows the same pattern used in `models.go:174-175` for `ResolveModel`.

**First, add helper method** (after line 44, after `NewClient`):
```go
// getProviderName returns the provider name for AIResponse.
// Falls back to "openai" if providerAlias is not set.
func (c *Client) getProviderName() string {
    if c.providerAlias == "" {
        return "openai"  // Safe fallback for direct instantiation
    }
    return c.providerAlias
}
```

**GenerateResponse - Success path** (lines 214-222):
```go
// CURRENT (line 214-222):
result := &core.AIResponse{
    Content: openAIResp.Choices[0].Message.Content,
    Model:   openAIResp.Model,
    Usage: core.TokenUsage{
        PromptTokens:     openAIResp.Usage.PromptTokens,
        CompletionTokens: openAIResp.Usage.CompletionTokens,
        TotalTokens:      openAIResp.Usage.TotalTokens,
    },
}

// CHANGE TO:
result := &core.AIResponse{
    Content:  openAIResp.Choices[0].Message.Content,
    Model:    openAIResp.Model,
    Provider: c.getProviderName(),  // NEW: Uses helper for safe fallback
    Usage: core.TokenUsage{
        PromptTokens:     openAIResp.Usage.PromptTokens,
        CompletionTokens: openAIResp.Usage.CompletionTokens,
        TotalTokens:      openAIResp.Usage.TotalTokens,
    },
}
```

**StreamResponse - All return sites** (lines 382, 400, 473, 500):
Add `Provider: c.getProviderName()` to each `&core.AIResponse{}` return.

This ensures:
- Direct OpenAI calls → `Provider: "openai"`
- Groq calls → `Provider: "openai.groq"`
- DeepSeek calls → `Provider: "openai.deepseek"`
- Edge case (empty alias) → `Provider: "openai"` (safe fallback)

---

###### 2b. Anthropic Client

**File**: `ai/providers/anthropic/client.go`

**GenerateResponse - Success path** (lines 228-236):
```go
// CURRENT (line 228-236):
result := &core.AIResponse{
    Content: content,
    Model:   anthropicResp.Model,
    Usage: core.TokenUsage{
        PromptTokens:     anthropicResp.Usage.InputTokens,
        CompletionTokens: anthropicResp.Usage.OutputTokens,
        TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
    },
}

// CHANGE TO:
result := &core.AIResponse{
    Content:  content,
    Model:    anthropicResp.Model,
    Provider: "anthropic",  // NEW
    Usage: core.TokenUsage{
        PromptTokens:     anthropicResp.Usage.InputTokens,
        CompletionTokens: anthropicResp.Usage.OutputTokens,
        TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
    },
}
```

**StreamResponse - All return sites** (lines 379, 400, 469, 511):
Add `Provider: "anthropic"` to each `&core.AIResponse{}` return.

---

###### 2c. Gemini Client

**File**: `ai/providers/gemini/client.go`

**GenerateResponse - Success path** (lines 243-251):
```go
// CURRENT (line 243-251):
result := &core.AIResponse{
    Content: content,
    Model:   options.Model,
    Usage: core.TokenUsage{
        PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
        CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
        TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
    },
}

// CHANGE TO:
result := &core.AIResponse{
    Content:  content,
    Model:    options.Model,
    Provider: "gemini",  // NEW
    Usage: core.TokenUsage{
        PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
        CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
        TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
    },
}
```

**StreamResponse - All return sites** (lines 405, 422, 475, 512):
Add `Provider: "gemini"` to each `&core.AIResponse{}` return.

---

###### 2d. Bedrock Client

**File**: `ai/providers/bedrock/client.go`

**GenerateResponse - Success path** (lines 178-190):
```go
// CURRENT (line 178-181):
result := &core.AIResponse{
    Content: content,
    Model:   options.Model,
}

// CHANGE TO:
result := &core.AIResponse{
    Content:  content,
    Model:    options.Model,
    Provider: "bedrock",  // NEW
}
```

**StreamResponse - All return sites** (lines 325, 362, 387, 409):
Add `Provider: "bedrock"` to each `&core.AIResponse{}` return.

---

##### Step 3: Capture Provider at LLM Debug Recording Sites

Update all 4 orchestration components to include the Provider field from AIResponse.

---

###### 3a. Orchestrator - Correction Site

**File**: `orchestration/orchestrator.go`

**Line 445-457** (successful correction recording):
```go
// CURRENT (line 445-457):
o.recordDebugInteraction(ctx, requestID, LLMInteraction{
    Type:             "correction",
    Timestamp:        llmStartTime,
    DurationMs:       llmDuration.Milliseconds(),
    Prompt:           correctionPrompt,
    Response:         response.Content,
    Model:            response.Model,
    PromptTokens:     response.Usage.PromptTokens,
    CompletionTokens: response.Usage.CompletionTokens,
    TotalTokens:      response.Usage.TotalTokens,
    Success:          true,
    Attempt:          1,
})

// ADD after Model field:
    Provider:         response.Provider,  // NEW
```

**Line 1076** (plan_generation site): Add `Provider: aiResponse.Provider` similarly.

---

###### 3b. Synthesizer

**File**: `orchestration/synthesizer.go`

**Line 119-134** (successful synthesis recording):
```go
// CURRENT (line 119-134):
s.recordDebugInteraction(ctx, requestID, LLMInteraction{
    Type:             "synthesis",
    Timestamp:        llmStartTime,
    DurationMs:       llmDuration.Milliseconds(),
    Prompt:           prompt,
    SystemPrompt:     systemPrompt,
    Temperature:      0.5,
    MaxTokens:        1500,
    Model:            aiResponse.Model,
    Response:         aiResponse.Content,
    PromptTokens:     aiResponse.Usage.PromptTokens,
    CompletionTokens: aiResponse.Usage.CompletionTokens,
    TotalTokens:      aiResponse.Usage.TotalTokens,
    Success:          true,
    Attempt:          1,
})

// ADD after Model field:
    Provider:         aiResponse.Provider,  // NEW
```

---

###### 3c. MicroResolver

**File**: `orchestration/micro_resolver.go`

**Line 263-277** (successful micro_resolution recording):
```go
// CURRENT (line 263-277):
m.recordDebugInteraction(ctx, requestID, LLMInteraction{
    Type:             "micro_resolution",
    Timestamp:        llmStartTime,
    DurationMs:       llmDuration.Milliseconds(),
    Prompt:           prompt,
    Temperature:      0.0,
    MaxTokens:        500,
    Model:            resp.Model,
    Response:         resp.Content,
    PromptTokens:     resp.Usage.PromptTokens,
    CompletionTokens: resp.Usage.CompletionTokens,
    TotalTokens:      resp.Usage.TotalTokens,
    Success:          true,
    Attempt:          1,
})

// ADD after Model field:
    Provider:         resp.Provider,  // NEW
```

---

###### 3d. ContextualReResolver

**File**: `orchestration/contextual_re_resolver.go`

**Line 179** (successful semantic_retry recording):
```go
// ADD after Model field in the LLMInteraction struct:
    Provider:         response.Provider,  // NEW
```

---

#### Files to Modify (Phase 1f) - CORRECTED

| Module | File | Lines | Change |
|--------|------|-------|--------|
| `core` | `core/interfaces.go` | 76-80 | Add `Provider string` to `AIResponse` struct |
| `ai` | `ai/providers/openai/client.go` | 214-222, 382, 400, 473, 500 | Set `Provider: c.providerAlias` in all `AIResponse` returns (supports "openai", "openai.groq", "openai.deepseek", etc.) |
| `ai` | `ai/providers/anthropic/client.go` | 228-236, 379, 400, 469, 511 | Set `Provider: "anthropic"` in all `AIResponse` returns |
| `ai` | `ai/providers/gemini/client.go` | 243-251, 405, 422, 475, 512 | Set `Provider: "gemini"` in all `AIResponse` returns |
| `ai` | `ai/providers/bedrock/client.go` | 178-181, 325, 362, 387, 409 | Set `Provider: "bedrock"` in all `AIResponse` returns |
| `orchestration` | `orchestration/orchestrator.go` | 451, 1076 | Add `Provider: response.Provider` at correction and plan_generation sites |
| `orchestration` | `orchestration/synthesizer.go` | 127 | Add `Provider: aiResponse.Provider` at synthesis site |
| `orchestration` | `orchestration/micro_resolver.go` | 270 | Add `Provider: resp.Provider` at micro_resolution site |
| `orchestration` | `orchestration/contextual_re_resolver.go` | 179 | Add `Provider: response.Provider` at semantic_retry site |

> **Note**: No separate client files exist for Groq, DeepSeek, or other OpenAI-compatible providers. They use the OpenAI client via the `providerAlias` field (e.g., `"openai.groq"`, `"openai.deepseek"`). The `getProviderName()` helper method ensures the actual provider is captured with a safe fallback to `"openai"` if the alias is empty.

#### Estimated Effort

~40 lines of code changes across 9 files (1 core + 4 ai providers + 4 orchestration).

#### Benefits

1. **Debugging**: Quickly identify which provider handled a request
2. **Analytics**: Compare token usage, latency, and costs across providers
3. **Compliance**: Audit trail of which AI systems processed user data
4. **UI Enhancement**: Registry viewer can display provider badges in LLM Debug view

---

### 4.8 Streaming Synthesis Recording (Phase 1g) ✅ IMPLEMENTED

The `ProcessRequestStreaming` method in `orchestrator.go` bypasses the LLM debug recording for the synthesis step. When streaming is enabled, the synthesis LLM call is made directly without recording.

#### Problem

When a request is processed via `ProcessRequestStreaming`:
1. `plan_generation` is recorded correctly ✅
2. Tool calls execute (HTTP calls, not LLM) - N/A
3. **Streaming synthesis LLM call is NOT recorded** ❌

This means operators only see `plan_generation` in the LLM debug store, missing the synthesis call that generates the final user response.

#### Root Cause

In `orchestrator.go`, the `ProcessRequestStreaming` method at line ~871 calls `streamingClient.StreamResponse()` directly:

```go
// Line ~871 in orchestrator.go - ProcessRequestStreaming
aiResponse, err := streamingClient.StreamResponse(ctx, synthesisPrompt, &core.AIOptions{
    Temperature:  0.7,
    MaxTokens:    2000,
    SystemPrompt: "You are a helpful assistant synthesizing responses from multiple agents.",
}, streamCallback)
```

This bypasses the recording that exists in `synthesizer.go:128` because `ProcessRequestStreaming` handles synthesis inline rather than delegating to the `AISynthesizer`.

#### Solution

Add LLM debug recording after the streaming synthesis completes (after line ~900).

##### Implementation

**File**: `orchestration/orchestrator.go`
**Function**: `ProcessRequestStreaming`
**Location**: After streaming completes (around line ~900)

```go
// Before the StreamResponse call, capture start time
synthesisStart := time.Now()

// Existing streaming call
aiResponse, err := streamingClient.StreamResponse(ctx, synthesisPrompt, &core.AIOptions{
    Temperature:  0.7,
    MaxTokens:    2000,
    SystemPrompt: "You are a helpful assistant synthesizing responses from multiple agents.",
}, streamCallback)

// After streaming completes (around line ~900), add recording:
if err == nil && aiResponse != nil {
    // LLM Debug: Record streaming synthesis
    o.recordDebugInteraction(ctx, requestID, LLMInteraction{
        Type:             "synthesis_streaming",
        Timestamp:        synthesisStart,
        DurationMs:       time.Since(synthesisStart).Milliseconds(),
        Prompt:           synthesisPrompt,
        SystemPrompt:     "You are a helpful assistant synthesizing responses from multiple agents.",
        Temperature:      0.7,
        MaxTokens:        2000,
        Model:            aiResponse.Model,
        Provider:         aiResponse.Provider,
        Response:         aiResponse.Content,
        PromptTokens:     aiResponse.Usage.PromptTokens,
        CompletionTokens: aiResponse.Usage.CompletionTokens,
        TotalTokens:      aiResponse.Usage.TotalTokens,
        Success:          true,
        Attempt:          1,
    })
} else if err != nil {
    // Record failed streaming synthesis
    o.recordDebugInteraction(ctx, requestID, LLMInteraction{
        Type:       "synthesis_streaming",
        Timestamp:  synthesisStart,
        DurationMs: time.Since(synthesisStart).Milliseconds(),
        Prompt:     synthesisPrompt,
        Success:    false,
        Error:      err.Error(),
        Attempt:    1,
    })
}
```

##### Type Value

Use `"synthesis_streaming"` to distinguish from non-streaming `"synthesis"` calls in `synthesizer.go`. This allows operators to:
- Filter by streaming vs non-streaming synthesis
- Identify which code path handled synthesis
- Track streaming-specific metrics

##### Performance Impact

**None** - Recording is asynchronous (goroutine) and happens after the response is already streamed to the user:
- User sees: `[tokens streaming...] → [done]`
- Redis write: Happens in background goroutine after "done"
- Recording uses same proven pattern as all other recording sites

#### Files to Modify

| File | Change |
|------|--------|
| `orchestration/orchestrator.go` | Add `synthesisStart` capture before `StreamResponse`, add `recordDebugInteraction` call after |

#### Estimated Effort

~20 lines of code in one file.

#### Implementation Summary

**Completed**: 2026-01-12

Changes made to `orchestration/orchestrator.go` in `ProcessRequestStreaming`:

1. **Added time capture before StreamResponse** (line ~857):
   ```go
   synthesisStart := time.Now()
   systemPrompt := "You are a helpful assistant synthesizing responses from multiple agents."
   ```

2. **Updated StreamResponse call** to use `systemPrompt` variable (line ~878)

3. **Added partial completion case recording** (line ~883-902):
   ```go
   // LLM Debug: Record partial streaming synthesis (interrupted but has content)
   o.recordDebugInteraction(ctx, requestID, LLMInteraction{
       Type:             "synthesis_streaming",
       Timestamp:        synthesisStart,
       DurationMs:       time.Since(synthesisStart).Milliseconds(),
       Prompt:           synthesisPrompt,
       SystemPrompt:     systemPrompt,
       Temperature:      0.7,
       MaxTokens:        2000,
       Model:            aiResponse.Model,
       Provider:         aiResponse.Provider,
       Response:         aiResponse.Content,
       PromptTokens:     aiResponse.Usage.PromptTokens,     // May be partial
       CompletionTokens: aiResponse.Usage.CompletionTokens, // May be partial
       TotalTokens:      aiResponse.Usage.TotalTokens,      // May be partial
       Success:          true,                              // Partial success - we have content
       Error:            "stream partially completed",
       Attempt:          1,
   })
   ```

4. **Added full failure case recording** (line ~920-932):
   ```go
   // LLM Debug: Record failed streaming synthesis
   o.recordDebugInteraction(ctx, requestID, LLMInteraction{
       Type:         "synthesis_streaming",
       Timestamp:    synthesisStart,
       DurationMs:   time.Since(synthesisStart).Milliseconds(),
       Prompt:       synthesisPrompt,
       SystemPrompt: systemPrompt,
       Temperature:  0.7,
       MaxTokens:    2000,
       Success:      false,
       Error:        err.Error(),
       Attempt:      1,
   })
   ```

5. **Added success case recording** (line ~966-983):
   ```go
   // LLM Debug: Record successful streaming synthesis
   o.recordDebugInteraction(ctx, requestID, LLMInteraction{
       Type:             "synthesis_streaming",
       Timestamp:        synthesisStart,
       DurationMs:       time.Since(synthesisStart).Milliseconds(),
       Prompt:           synthesisPrompt,
       SystemPrompt:     systemPrompt,
       Temperature:      0.7,
       MaxTokens:        2000,
       Model:            aiResponse.Model,
       Provider:         aiResponse.Provider,
       Response:         aiResponse.Content,
       PromptTokens:     aiResponse.Usage.PromptTokens,
       CompletionTokens: aiResponse.Usage.CompletionTokens,
       TotalTokens:      aiResponse.Usage.TotalTokens,
       Success:          true,
       Attempt:          1,
   })
   ```

**Recording Sites Summary** (now 7 total):
- `plan_generation` - Orchestrator plan generation
- `correction` - Orchestrator parameter correction
- `synthesis` - Non-streaming synthesis (via AISynthesizer)
- `synthesis_streaming` - **NEW**: Streaming synthesis (inline in ProcessRequestStreaming)
- `micro_resolution` - Micro resolver
- `semantic_retry` - Contextual re-resolver

---

### 5. Debug API Endpoint (Application Responsibility) ❌ NOT IMPLEMENTED (Phase 2)

> **Note**: HTTP handlers should NOT live in the framework. The framework provides the
> store interface and implementation. Applications wire up their own handlers using
> their preferred router (chi, gin, gorilla/mux, standard library, etc.).

**Example implementation in travel-chat-agent:**

```go
// File: examples/travel-chat-agent/debug_handler.go

package main

import (
    "encoding/json"
    "net/http"

    "github.com/anthropics/gomind/orchestration"
)

type DebugHandler struct {
    store orchestration.LLMDebugStore
}

func NewDebugHandler(store orchestration.LLMDebugStore) *DebugHandler {
    return &DebugHandler{store: store}
}

// RegisterRoutes registers debug endpoints with your router
// Adapt this to your router of choice (chi, gin, gorilla/mux, etc.)
func (h *DebugHandler) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("GET /api/v1/debug/llm/{request_id}", h.GetLLMDebugRecord)
    mux.HandleFunc("GET /api/v1/debug/llm", h.ListRecentRecords)
}

// GET /api/v1/debug/llm/{request_id}
func (h *DebugHandler) GetLLMDebugRecord(w http.ResponseWriter, r *http.Request) {
    requestID := r.PathValue("request_id") // Go 1.22+ standard library

    record, err := h.store.GetRecord(r.Context(), requestID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(record)
}

// GET /api/v1/debug/llm?limit=50
func (h *DebugHandler) ListRecentRecords(w http.ResponseWriter, r *http.Request) {
    limit := 50 // default
    if l := r.URL.Query().Get("limit"); l != "" {
        fmt.Sscanf(l, "%d", &limit)
    }

    records, err := h.store.ListRecent(r.Context(), limit)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(records)
}
```

**Why handlers stay in the application:**
- Framework remains router-agnostic
- Applications can add auth, rate limiting, custom middleware
- Consistent with gomind's design (registry-viewer-app is separate)
- Applications control their HTTP layer

### 6. Configuration (Following Framework Principles) ✅ IMPLEMENTED

Per `FRAMEWORK_DESIGN_PRINCIPLES.md`, configuration must use:
- **WithXXX() option functions** for smart auto-configuration
- **Environment variable precedence** (explicit > standard env vars > GOMIND_* > defaults)
- **Intelligent defaults** that work with zero configuration

#### Option Functions

```go
// File: orchestration/options.go

// WithLLMDebug enables LLM debug payload storage with intelligent defaults.
// When enabled without explicit store, auto-uses Redis from discovery if available.
func WithLLMDebug(enabled bool) Option {
    return func(c *OrchestratorConfig) {
        c.EnableLLMDebug = enabled

        if enabled && c.LLMDebugStore == nil {
            // Auto-configure from environment or existing Redis connection
            // Precedence: explicit config > REDIS_URL > GOMIND_REDIS_URL > discovery Redis
            if redisURL := getRedisURL(); redisURL != "" {
                store, err := NewRedisLLMDebugStore(
                    WithDebugRedisURL(redisURL),
                    WithDebugRedisDB(getEnvInt("GOMIND_LLM_DEBUG_REDIS_DB", core.RedisDBLLMDebug)),
                )
                if err == nil {
                    c.LLMDebugStore = store
                }
                // If Redis fails, fall back to NoOp (don't break orchestration)
            }
        }

        // If still nil, use NoOp to ensure safe operation
        if enabled && c.LLMDebugStore == nil {
            c.LLMDebugStore = &NoOpLLMDebugStore{}
        }
    }
}

// WithLLMDebugStore explicitly sets the debug store implementation.
// Use this when you want a custom backend (PostgreSQL, S3, etc.)
func WithLLMDebugStore(store LLMDebugStore) Option {
    return func(c *OrchestratorConfig) {
        c.EnableLLMDebug = true
        c.LLMDebugStore = store
    }
}

// WithLLMDebugTTL sets custom TTL for successful debug records.
func WithLLMDebugTTL(ttl time.Duration) Option {
    return func(c *OrchestratorConfig) {
        c.LLMDebugTTL = ttl
    }
}
```

#### Environment Variable Support

| Variable | Purpose | Default |
|----------|---------|---------|
| `GOMIND_LLM_DEBUG_ENABLED` | Enable/disable debug payload storage | `false` |
| `GOMIND_LLM_DEBUG_REDIS_DB` | Redis database number | `7` (`core.RedisDBLLMDebug`) |
| `GOMIND_LLM_DEBUG_TTL` | TTL for success records | `24h` |
| `GOMIND_LLM_DEBUG_ERROR_TTL` | TTL for error records | `168h` (7d) |
| `REDIS_URL` | Redis connection (shared, highest precedence) | - |
| `GOMIND_REDIS_URL` | Redis connection (fallback) | `localhost:6379` |

#### NoOp Implementation (Safe Default) ✅ IMPLEMENTED

```go
// File: orchestration/noop_llm_debug_store.go

// NoOpLLMDebugStore is a safe default that does nothing.
// Used when debug storage is disabled or unavailable.
type NoOpLLMDebugStore struct{}

func (s *NoOpLLMDebugStore) RecordInteraction(ctx context.Context, requestID string, interaction LLMInteraction) error {
    return nil // Silent no-op
}

func (s *NoOpLLMDebugStore) GetRecord(ctx context.Context, requestID string) (*LLMDebugRecord, error) {
    return nil, fmt.Errorf("debug storage not configured")
}

func (s *NoOpLLMDebugStore) SetMetadata(ctx context.Context, requestID string, key, value string) error {
    return nil
}

func (s *NoOpLLMDebugStore) ExtendTTL(ctx context.Context, requestID string, duration time.Duration) error {
    return nil
}

func (s *NoOpLLMDebugStore) ListRecent(ctx context.Context, limit int) ([]LLMDebugRecordSummary, error) {
    return []LLMDebugRecordSummary{}, nil
}
```

#### Config Struct (internal, not exposed directly)

```go
type OrchestratorConfig struct {
    // ... existing fields ...

    // Debug payload storage (configured via WithLLMDebug options)
    EnableLLMDebug   bool
    LLMDebugStore    LLMDebugStore  // Defaults to NoOpLLMDebugStore
    LLMDebugTTL      time.Duration  // Default: 24h
    LLMDebugErrorTTL time.Duration  // Default: 7d
}
```

## Operator Configuration

This feature is **enabled by default** for better production observability. Operators can disable it if needed.

### Enabling/Disabling

#### Option 1: Environment Variable (Recommended for Production)

```bash
# Enabled by default - no action needed

# Disable LLM debug payload storage
export GOMIND_LLM_DEBUG_ENABLED=false
```

#### Option 2: Code Configuration

```go
// Enable with auto-configured Redis
config := orchestration.DefaultConfig()
orchestration.WithLLMDebug(true)(&config)

// Disable explicitly
orchestration.WithLLMDebug(false)(&config)

// Enable with custom store
orchestration.WithLLMDebugStore(myCustomStore)(&config)
```

#### Option 3: Kubernetes ConfigMap/Secret

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gomind-config
data:
  GOMIND_LLM_DEBUG_ENABLED: "true"
  GOMIND_LLM_DEBUG_TTL: "48h"
  GOMIND_LLM_DEBUG_ERROR_TTL: "168h"
```

### Runtime Behavior When Disabled

When `GOMIND_LLM_DEBUG_ENABLED=false`:
- `NoOpLLMDebugStore` is used
- Zero overhead on orchestration
- No Redis connections for debug storage
- Debug API endpoints return "not configured" errors

### Runtime Behavior When Enabled

When `GOMIND_LLM_DEBUG_ENABLED=true`:
- Auto-configures Redis using `REDIS_URL` > `GOMIND_REDIS_URL` > `localhost:6379`
- Falls back to `NoOpLLMDebugStore` if Redis unavailable (no failure)
- Full payload storage with compression for payloads > 100KB
- Debug API endpoints fully functional

### Runtime Toggle (Without Restart)

For runtime enable/disable without restart, operators can:

1. **Use feature flags** - Integrate with your feature flag system
2. **Hot-reload config** - If your app supports config hot-reload

```go
// Example: Check feature flag before recording
func (o *Orchestrator) recordDebugInteraction(ctx context.Context, requestID string, interaction LLMInteraction) {
    // Check runtime flag (e.g., from feature flag service)
    if !o.config.EnableLLMDebug || !featureFlags.IsEnabled("llm_debug") {
        return
    }
    // ... rest of implementation
}
```

### Production Checklist

| Item | Check |
|------|-------|
| Redis has sufficient memory for debug payloads | ☐ |
| TTL configured appropriately for retention needs | ☐ |
| Debug API endpoint is secured (auth/network) | ☐ |
| Monitoring alerts for Redis DB 7 memory usage | ☐ |
| Decided whether to keep enabled (default) or disable | ☐ |

## Usage

### Querying via API

```bash
# Get specific request's LLM interactions
curl http://localhost:8080/api/v1/debug/llm/req-abc123

# List recent requests
curl http://localhost:8080/api/v1/debug/llm?limit=20
```

### Response Example

> **Note**: The `model` field is populated from `core.AIResponse.Model` on successful LLM calls.
> The `provider` field is not yet populated (would require adding `Provider` to `core.AIResponse`).

```json
{
  "request_id": "req-abc123",
  "trace_id": "369fecb4e3156c34e0950c61f1f99d62",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:02Z",
  "interactions": [
    {
      "type": "plan_generation",
      "timestamp": "2024-01-15T10:30:00Z",
      "duration_ms": 1523,
      "prompt": "You are an intelligent orchestrator. Given the user request and available capabilities...[FULL PROMPT]",
      "system_prompt": "You must respond with valid JSON only.",
      "temperature": 0.3,
      "max_tokens": 2000,
      "model": "gpt-4o-mini",
      "response": "{\"routing_plan\":{\"steps\":[...FULL RESPONSE...]}}",
      "prompt_tokens": 1247,
      "completion_tokens": 423,
      "total_tokens": 1670,
      "success": true,
      "attempt": 1
    },
    {
      "type": "synthesis",
      "timestamp": "2024-01-15T10:30:02Z",
      "duration_ms": 892,
      "prompt": "Synthesize the following tool results into a user-friendly response...",
      "model": "gpt-4o-mini",
      "response": "Based on the weather data, Tokyo will have...",
      "prompt_tokens": 856,
      "completion_tokens": 234,
      "total_tokens": 1090,
      "success": true,
      "attempt": 1
    }
  ]
}
```

### Linking from Jaeger

In Jaeger, the span will include:

```
Span: orchestrator.process_request
Attributes:
  - request_id: req-abc123
  - llm_debug.available: true
  - llm_debug.endpoint: /api/v1/debug/llm/req-abc123
```

## Alternative: Enhanced Jaeger (Not Recommended)

For reference, here's why storing full payloads in Jaeger is problematic:

| Concern | Impact |
|---------|--------|
| Span size limits | ~128KB soft limit per span, varies by backend |
| Query performance | Large spans slow down trace queries |
| Storage costs | 10x more storage for large payloads |
| Cardinality explosion | Full prompts create high-cardinality attributes |

If you must use Jaeger, consider span **events** (not attributes) with chunking:

```go
// NOT RECOMMENDED - but possible
chunks := chunkString(prompt, 30000)
for i, chunk := range chunks {
    telemetry.AddSpanEvent(ctx, fmt.Sprintf("llm.prompt.chunk.%d", i),
        attribute.String("content", chunk),
    )
}
```

This approach makes traces hard to query and analyze.

## Recommended Implementation Order

1. **Phase 1a**: ✅ Implement `RedisLLMDebugStore`, NoOp, Memory stores + core changes
2. **Phase 1b**: ✅ Complete integration at all LLM call sites (synthesizer, micro_resolver, contextual_re_resolver, correction) + factory wiring
3. **Phase 1c**: ✅ Add lifecycle management (WaitGroup, Shutdown, telemetry fallback)
4. **Phase 1d**: ✅ Add unit tests (22 tests with ~90% coverage)
5. **Phase 1e**: ✅ Add Model field population from `aiResponse.Model` at all successful LLM call sites
6. **Phase 1f**: ❌ **NEXT STEP** - Add Provider field to `core.AIResponse`, update AI clients (openai, anthropic, gemini, bedrock), capture at all LLM debug recording sites
7. **Phase 2**: ❌ Add debug API endpoint to travel-chat-agent (application layer)
8. **Phase 3**: ✅ Add simple UI tab to registry-viewer-app for payload inspection (collapsible interactions, expand/collapse all, auto-refresh disabled for LLM Debug view)

## File Changes Summary

### Module: `core`

| File | Change Type | Status | Purpose |
|------|-------------|--------|---------|
| `core/redis_client.go` | Modify | ✅ Done | Add `RedisDBLLMDebug`, `RedisDBReservedStart/End`, `IsReservedDB()`, `GetRedisDBName()` update, warning in `NewRedisClient()` |
| `core/interfaces.go` | Modify | ❌ TODO (1f) | Add `Provider string` field to `AIResponse` struct (lines 76-80) |

### Module: `ai`

| File | Change Type | Status | Purpose |
|------|-------------|--------|---------|
| `ai/providers/openai/client.go` | Modify | ❌ TODO (1f) | Add `getProviderName()` helper method (safe fallback), set `Provider: c.getProviderName()` in all `AIResponse` returns - supports OpenAI + compatible providers (lines 44+, 214-222, 382, 400, 473, 500) |
| `ai/providers/anthropic/client.go` | Modify | ❌ TODO (1f) | Set `Provider: "anthropic"` in all `AIResponse` returns (lines 228-236, 379, 400, 469, 511) |
| `ai/providers/gemini/client.go` | Modify | ❌ TODO (1f) | Set `Provider: "gemini"` in all `AIResponse` returns (lines 243-251, 405, 422, 475, 512) |
| `ai/providers/bedrock/client.go` | Modify | ❌ TODO (1f) | Set `Provider: "bedrock"` in all `AIResponse` returns (lines 178-181, 325, 362, 387, 409) |

> **Note**: No separate client files exist for Groq, DeepSeek, or other OpenAI-compatible providers. They use the OpenAI client via the `providerAlias` field (e.g., `"openai.groq"`, `"openai.deepseek"`). The `getProviderName()` helper method ensures the actual provider is captured with a safe fallback to `"openai"` if the alias is empty.

### Module: `orchestration`

| File | Change Type | Status | Purpose |
|------|-------------|--------|---------|
| `orchestration/llm_debug_store.go` | **New** | ✅ Done | Interface (`LLMDebugStore`) + data types (`LLMDebugRecord`, `LLMInteraction`, `LLMDebugConfig`) |
| `orchestration/redis_llm_debug_store.go` | **New** | ✅ Done | Redis implementation with circuit breaker, compression, TTL |
| `orchestration/noop_llm_debug_store.go` | **New** | ✅ Done | NoOp safe default |
| `orchestration/memory_llm_debug_store.go` | **New** | ✅ Done | In-memory for testing |
| `orchestration/llm_debug_store_test.go` | **New** | ✅ Done | 22 unit tests: MemoryLLMDebugStore, NoOpLLMDebugStore, propagation, shutdown, factory options |
| `orchestration/factory.go` | Modify | ✅ Done | `WithLLMDebug()`, `WithLLMDebugStore()`, auto-init, sub-component wiring via `SetLLMDebugStore()` propagation |
| `orchestration/interfaces.go` | Modify | ✅ Done | Add `LLMDebug` and `LLMDebugStore` to `OrchestratorConfig`, env var parsing in `DefaultConfig()` |
| `orchestration/orchestrator.go` | Modify | ✅ Done (⚠️ 1f pending) | `debugStore`, `debugWg`, `debugSeqID` fields, `SetLLMDebugStore()`, `GetLLMDebugStore()` with propagation, `recordDebugInteraction()`, `Shutdown(ctx)`, `generateFallbackRequestID()`, correction call site integration. **1f**: Add `Provider: aiResponse.Provider` |
| `orchestration/synthesizer.go` | Modify | ✅ Done (⚠️ 1f pending) | `debugStore`, `debugWg`, `debugSeqID`, `logger` fields, `SetLLMDebugStore()`, `SetLogger()`, `recordDebugInteraction()`, `Shutdown(ctx)`, synthesis call site integration. **1f**: Add `Provider: aiResponse.Provider` |
| `orchestration/micro_resolver.go` | Modify | ✅ Done (⚠️ 1f pending) | `debugStore`, `debugWg`, `debugSeqID` fields, `SetLLMDebugStore()`, `recordDebugInteraction()`, `Shutdown(ctx)`, micro_resolution call site integration. **1f**: Add `Provider: resp.Provider` |
| `orchestration/contextual_re_resolver.go` | Modify | ✅ Done (⚠️ 1f pending) | `debugStore`, `debugWg`, `debugSeqID` fields, `SetLLMDebugStore()`, `recordDebugInteraction()`, `Shutdown(ctx)`, semantic_retry call site integration. **1f**: Add `Provider: response.Provider` |

### Module: `examples` (Application Layer)

| File | Change Type | Status | Purpose |
|------|-------------|--------|---------|
| `examples/registry-viewer-app/static/index.html` | Modify | ✅ Done | LLM Debug UI tab with collapsible interactions, expand/collapse all buttons |
| `examples/travel-chat-agent/debug_handler.go` | **New** | ❌ TODO | HTTP handlers for debug API |
| `examples/travel-chat-agent/main.go` | Modify | ❌ TODO | Wire up debug store and routes |

### Summary (Phase 1 - Framework Layer)

```
Status Legend: ✅ Done | ⚠️ Partial | ❌ TODO

core/                          1 modified  ✅ (redis_client.go)
                               1 modified  ❌ TODO (1f: interfaces.go - Provider field in AIResponse)
ai/providers/                  4 modified  ❌ TODO (1f: openai, anthropic, gemini, bedrock clients)
                               Note: No groq_client.go - Groq uses OpenAI alias system
orchestration/                 5 new       ✅ (llm_debug_store, redis_llm_debug_store, noop, memory, test)
                               6 modified  ⚠️ (factory✅, interfaces✅, orchestrator⚠️, synthesizer⚠️, micro_resolver⚠️, contextual_re_resolver⚠️)
                                           ⚠️ = Phase 1f Provider field pending
───────────────────────────────────────────
Completed (Phase 1a-1e):       12 files (5 new + 7 modified)
Pending (Phase 1f):            9 files (1 core + 4 ai providers + 4 orchestration)
```

### Summary (Phase 2 - Application Layer - ⚠️ Partial)

```
examples/registry-viewer-app/  1 modified  ✅ (UI tab implemented)
examples/travel-chat-agent/    1 new, 1 modified  ❌ TODO (debug API handlers)
```

## Estimated Effort

| Phase | Description | LOC | Status |
|-------|-------------|-----|--------|
| **1a** | Store implementations, core changes | ~250 | ✅ Done |
| **1b** | Integration at all 5 LLM call sites (synthesizer, micro_resolver, contextual_re_resolver, correction, factory wiring) | ~100 | ✅ Done |
| **1c** | Lifecycle management (WaitGroup, Shutdown, telemetry fallback) | ~50 | ✅ Done |
| **1d** | Unit tests (22 tests covering stores, propagation, shutdown, factory) | ~800 | ✅ Done |
| **1e** | Model field population at all successful LLM call sites | ~10 | ✅ Done |
| **1f** | **NEXT STEP**: Provider field - add to `core/interfaces.go:AIResponse`, add `getProviderName()` helper to OpenAI client (safe fallback), update 4 AI clients (`ai/providers/{openai,anthropic,gemini,bedrock}/client.go`), capture at 4 orchestration recording sites | ~45 | ❌ TODO |
| **2** | Debug API endpoint (application layer) | ~50 | ❌ TODO |
| **3** | Registry viewer UI tab (collapsible interactions, expand/collapse all, auto-refresh control) | ~200 | ✅ Done |
| **4** | Compression and TTL refinements | ~50 | ✅ Done (included in 1a) |

**Total**: ~1550 lines of new code across core, ai, orchestration, and examples.
**Completed**: ~1460 lines (Phase 1a + 1b + 1c + 1d + 1e + Phase 3)
**Remaining**: ~95 lines (Phase 1f: Provider field ~45 across 9 files including helper method, Phase 2: debug API handlers ~50)
