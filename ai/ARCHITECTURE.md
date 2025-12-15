# GoMind AI Module Architecture

**Version**: 1.0
**Module**: `github.com/itsneelabh/gomind/ai`
**Purpose**: Production-grade AI provider abstraction with multi-provider support
**Audience**: Framework developers, application developers, operations teams

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Design Philosophy](#design-philosophy)
3. [Module Dependencies](#module-dependencies)
4. [Provider Registry System](#provider-registry-system)
5. [Provider Implementation Pattern](#provider-implementation-pattern)
6. [Chain Client (Failover)](#chain-client-failover)
7. [AI Agent and AI Tool](#ai-agent-and-ai-tool)
8. [Logging and Telemetry](#logging-and-telemetry)
9. [Configuration System](#configuration-system)
10. [Integration Patterns](#integration-patterns)
11. [Common Pitfalls](#common-pitfalls)
12. [Troubleshooting Guide](#troubleshooting-guide)

---

## Architecture Overview

### System Context

```
┌─────────────────────────────────────────────────────────────┐
│ Application Layer                                            │
│                                                             │
│  client, _ := ai.NewClient(ai.WithProvider("openai"))      │
│  response, _ := client.GenerateResponse(ctx, prompt, opts)  │
└─────────────────────────────────────────────────────────────┘
                         │
                         │ Uses factory pattern
                         ↓
┌─────────────────────────────────────────────────────────────┐
│ AI Module (github.com/itsneelabh/gomind/ai)                 │
│                                                             │
│  ┌─────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │  Registry   │───>│ ProviderFactory│───>│ AIClient    │  │
│  │  (Global)   │    │ (Per Provider)│    │ (Per Call)  │  │
│  └─────────────┘    └──────────────┘    └──────────────┘  │
│         │                                                   │
│         │ Import-time registration via init()              │
└─────────────────────────────────────────────────────────────┘
                         │
                         │ Implements core.AIClient
                         ↓
┌─────────────────────────────────────────────────────────────┐
│ Provider Clients                                            │
│                                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │ OpenAI   │  │Anthropic │  │  Gemini  │  │ Bedrock  │  │
│  │ Client   │  │  Client  │  │  Client  │  │  Client  │  │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘  │
│         │            │            │            │           │
│         └────────────┴────────────┴────────────┘           │
│                              │                             │
│                    All embed BaseClient                    │
└─────────────────────────────────────────────────────────────┘
                         │
                         │ HTTPS API calls
                         ↓
┌─────────────────────────────────────────────────────────────┐
│ External AI APIs                                            │
│                                                             │
│  OpenAI, Anthropic, Google Gemini, AWS Bedrock, etc.       │
└─────────────────────────────────────────────────────────────┘
```

### Key Components

| Component | Responsibility | Location |
|-----------|----------------|----------|
| `ProviderRegistry` | Global registry of provider factories | `registry.go` |
| `ProviderFactory` | Interface for provider creation | `registry.go` |
| `AIConfig` | Configuration options for clients | `provider.go` |
| `BaseClient` | Shared functionality (retry, logging) | `providers/base.go` |
| `ChainClient` | Multi-provider failover | `chain_client.go` |
| `AIAgent` | Agent with AI + discovery capabilities | `ai_agent.go` |
| `AITool` | Tool with AI capabilities (no discovery) | `ai_tool.go` |

---

## Design Philosophy

### 1. Import-Driven Provider Registration

**The Design Decision**: Providers self-register via `init()` functions when their package is imported.

```go
// Application chooses which providers to include at compile time
import (
    "github.com/itsneelabh/gomind/ai"
    _ "github.com/itsneelabh/gomind/ai/providers/openai"     // Registers OpenAI
    _ "github.com/itsneelabh/gomind/ai/providers/anthropic"  // Registers Anthropic
    // Don't import bedrock → not compiled in, smaller binary
)
```

**Benefits**:
- **Compile-time selection**: Only imported providers are included in binary
- **No runtime configuration**: Provider availability is determined at build time
- **Smaller binaries**: Unused providers don't bloat the executable
- **Clear dependencies**: `go.mod` shows exactly which providers are used

### 2. Factory Pattern for Client Creation

**The Design Decision**: Each provider implements `ProviderFactory` interface.

```go
type ProviderFactory interface {
    Create(config *AIConfig) core.AIClient
    DetectEnvironment() (priority int, available bool)
    Name() string
    Description() string
}
```

**Why Factory Pattern?**

| Pattern | Pros | Cons | GoMind Choice |
|---------|------|------|---------------|
| Direct instantiation | Simple | Tight coupling | ❌ |
| Factory method | Flexible, testable | Slightly more code | ✅ Chosen |
| Dependency injection | Maximum flexibility | Complex setup | ❌ |

**Benefits**:
1. **Environment detection**: Auto-detect available providers
2. **Priority-based selection**: Choose best provider automatically
3. **Configuration injection**: Pass config at creation time
4. **Testability**: Easy to mock provider factories

### 3. Shared Base Client for Cross-Cutting Concerns

**The Design Decision**: All provider clients embed `BaseClient`.

```go
// providers/openai/client.go
type Client struct {
    *providers.BaseClient  // Embedded - provides retry, logging, defaults
    apiKey  string
    baseURL string
}
```

**What BaseClient Provides**:
- HTTP client with configurable timeout
- Exponential backoff retry logic
- Request/response logging
- Default value management
- Error handling utilities

### 4. Tool/Agent Separation with AI

**The Design Decision**: Maintain the framework's Tool/Agent distinction.

```
┌─────────────────────────────────────────────────────────────┐
│ AITool (Passive)                                            │
│ - Uses AI for capabilities                                  │
│ - NO discovery (cannot find other components)               │
│ - Example: Translation tool, Summarization tool             │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ AIAgent (Active)                                            │
│ - Uses AI for orchestration                                 │
│ - HAS discovery (can find and coordinate components)        │
│ - Example: Research agent, Planning agent                   │
└─────────────────────────────────────────────────────────────┘
```

This enforced distinction prevents architectural violations where tools accidentally become orchestrators.

---

## Module Dependencies

### Dependency Decision

```
Valid Dependencies:
┌─────────────────────────────────────────────────────────────┐
│  ai  →  core  +  telemetry                                  │
└─────────────────────────────────────────────────────────────┘
```

**Rationale**: The AI module is expanded to include telemetry for production visibility:

| Dependency | Purpose | Justification |
|------------|---------|---------------|
| `core` | Interfaces (AIClient, Logger) | Required foundation |
| `telemetry` | Metrics emission | Production observability |

**Why `ai` needs `telemetry`**:
1. **External API calls**: AI providers make external HTTP calls that need latency/error tracking
2. **Cost visibility**: Token usage directly translates to costs
3. **Failover tracking**: Chain client failovers need metrics
4. **Consistency**: Matches `resilience` and `orchestration` modules

### Import Structure

```go
// ai/client.go
import (
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"  // Allowed
)

// ai/providers/openai/client.go
import (
    "github.com/itsneelabh/gomind/ai"
    "github.com/itsneelabh/gomind/ai/providers"
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"  // Allowed
)
```

---

## Provider Registry System

### Global Registry Architecture

```go
// registry.go
var registry = &ProviderRegistry{
    providers: make(map[string]ProviderFactory),
}

// Thread-safe registration
func Register(factory ProviderFactory) error {
    registry.mu.Lock()
    defer registry.mu.Unlock()

    if _, exists := registry.providers[factory.Name()]; exists {
        return fmt.Errorf("provider '%s' already registered", factory.Name())
    }
    registry.providers[factory.Name()] = factory
    return nil
}
```

### Provider Registration Flow

```
┌──────────────────────────────────────────────────────────────┐
│ Application Startup                                           │
│                                                              │
│ 1. Go runtime processes imports                              │
│ 2. Each provider's init() runs                               │
│ 3. init() calls ai.Register(&Factory{})                      │
│ 4. Factory stored in global registry                         │
└──────────────────────────────────────────────────────────────┘
          │
          ↓
┌──────────────────────────────────────────────────────────────┐
│ Provider Registration (e.g., openai/factory.go)              │
│                                                              │
│ func init() {                                                │
│     if err := ai.Register(&Factory{}); err != nil {          │
│         panic(err)  // Fail fast on registration error       │
│     }                                                        │
│ }                                                            │
└──────────────────────────────────────────────────────────────┘
          │
          ↓
┌──────────────────────────────────────────────────────────────┐
│ Client Creation                                              │
│                                                              │
│ client, _ := ai.NewClient(                                   │
│     ai.WithProvider("openai"),  // Lookup in registry        │
│ )                                                            │
│                                                              │
│ // Registry finds "openai" factory, calls factory.Create()   │
└──────────────────────────────────────────────────────────────┘
```

### Auto-Detection Logic

```go
func detectBestProvider(logger core.Logger) (string, error) {
    var candidates []candidate

    // Check each registered provider
    for name, factory := range registry.providers {
        priority, available := factory.DetectEnvironment()
        if available {
            candidates = append(candidates, candidate{name, priority})
        }
    }

    // Sort by priority (highest first)
    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].priority > candidates[j].priority
    })

    if len(candidates) == 0 {
        return "", fmt.Errorf("no provider detected")
    }

    return candidates[0].name, nil
}
```

**Provider Priorities** (default):
| Provider | Priority | Detection Method |
|----------|----------|------------------|
| OpenAI | 100 | `OPENAI_API_KEY` exists |
| Anthropic | 90 | `ANTHROPIC_API_KEY` exists |
| Gemini | 80 | `GEMINI_API_KEY` or `GOOGLE_API_KEY` exists |
| Bedrock | 70 | AWS credentials available |
| Mock | 1 | Never auto-detected |

---

## Provider Implementation Pattern

### Factory Implementation

Each provider implements this pattern:

```go
// providers/openai/factory.go
package openai

func init() {
    if err := ai.Register(&Factory{}); err != nil {
        panic(fmt.Sprintf("failed to register openai provider: %v", err))
    }
}

type Factory struct{}

func (f *Factory) Name() string        { return "openai" }
func (f *Factory) Description() string { return "OpenAI GPT models" }

func (f *Factory) DetectEnvironment() (priority int, available bool) {
    if os.Getenv("OPENAI_API_KEY") != "" {
        return 100, true  // High priority when key exists
    }
    return 0, false
}

func (f *Factory) Create(config *ai.AIConfig) core.AIClient {
    // Extract configuration
    apiKey := firstNonEmpty(config.APIKey, os.Getenv("OPENAI_API_KEY"))
    baseURL := firstNonEmpty(config.BaseURL, os.Getenv("OPENAI_BASE_URL"), DefaultBaseURL)

    // CRITICAL: Get logger from config, wrap with component
    logger := config.Logger
    if logger != nil {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            logger = cal.WithComponent("framework/ai")
        }
    }

    return NewClient(apiKey, baseURL, logger)
}
```

### Client Implementation

```go
// providers/openai/client.go
package openai

type Client struct {
    *providers.BaseClient  // Embedded for retry, logging
    apiKey  string
    baseURL string
}

func NewClient(apiKey, baseURL string, logger core.Logger) *Client {
    base := providers.NewBaseClient(30*time.Second, logger)
    base.DefaultModel = "gpt-3.5-turbo"
    base.DefaultMaxTokens = 1000

    return &Client{
        BaseClient: base,
        apiKey:     apiKey,
        baseURL:    baseURL,
    }
}

func (c *Client) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
    // Apply defaults
    options = c.ApplyDefaults(options)

    // Log request
    c.LogRequest("openai", options.Model, prompt)
    startTime := time.Now()

    // Build and execute request...

    // Log response
    c.LogResponse("openai", result.Model, result.Usage, time.Since(startTime))

    return result, nil
}
```

### OpenAI-Compatible Providers (Provider Aliases)

The `ai` module supports OpenAI-compatible providers through aliases:

```go
// Usage
client, _ := ai.NewClient(
    ai.WithProviderAlias("openai.deepseek"),  // Uses DeepSeek API
    ai.WithModel("smart"),                     // Resolves to "deepseek-reasoner"
)
```

**Supported Aliases**:
| Alias | Base URL | API Key Env |
|-------|----------|-------------|
| `openai.deepseek` | `api.deepseek.com` | `DEEPSEEK_API_KEY` |
| `openai.groq` | `api.groq.com/openai/v1` | `GROQ_API_KEY` |
| `openai.xai` | `api.x.ai/v1` | `XAI_API_KEY` |
| `openai.together` | `api.together.xyz/v1` | `TOGETHER_API_KEY` |
| `openai.qwen` | `dashscope-intl.aliyuncs.com/...` | `QWEN_API_KEY` |
| `openai.ollama` | `localhost:11434/v1` | (none required) |

---

## Chain Client (Failover)

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ ChainClient                                                  │
│                                                             │
│  GenerateResponse(ctx, prompt, opts)                        │
│         │                                                   │
│         ├──→ Provider 1 (OpenAI) ────→ Success? Return     │
│         │         │                                        │
│         │         ↓ Failure (5xx)                          │
│         │                                                   │
│         ├──→ Provider 2 (Anthropic) ──→ Success? Return    │
│         │         │                                        │
│         │         ↓ Failure (5xx)                          │
│         │                                                   │
│         └──→ Provider 3 (Gemini) ────→ Success? Return     │
│                   │                                        │
│                   ↓ All failed                             │
│                                                             │
│              Return aggregated error                        │
└─────────────────────────────────────────────────────────────┘
```

### Usage

```go
chain, err := ai.NewChainClient(
    ai.WithProviderChain("openai", "openai.deepseek", "anthropic"),
    ai.WithChainLogger(logger),
)

// Uses OpenAI first, falls back to DeepSeek, then Anthropic
response, err := chain.GenerateResponse(ctx, prompt, opts)
```

### Failover Behavior

| Error Type | Behavior | Rationale |
|------------|----------|-----------|
| **Client errors (4xx)** | Fail fast, no retry | Request is invalid for all providers |
| **Server errors (5xx)** | Try next provider | Provider-specific issue |
| **Rate limits (429)** | Try next provider | Provider at capacity |
| **Network errors** | Try next provider | Transient connectivity |

### Metrics for Failover

When telemetry is initialized, the chain client should emit:

```go
// On failover
telemetry.Counter("ai.chain.failover",
    "from_provider", "openai",
    "to_provider", "anthropic",
    "reason", "server_error")

// On complete failure
telemetry.Counter("ai.chain.exhausted",
    "providers_tried", "3",
    "final_error", "rate_limit")
```

---

## AI Agent and AI Tool

### AIAgent (Active Orchestrator)

```go
type AIAgent struct {
    *core.BaseAgent               // Has discovery capability
    AI              core.AIClient // AI client for processing
}

// Can discover and coordinate other components
func (a *AIAgent) DiscoverAndOrchestrate(ctx context.Context, query string) (string, error) {
    // 1. Use AI to understand intent
    // 2. Discover available components via a.Discover()
    // 3. Use AI to plan component usage
    // 4. Execute plan
    // 5. Synthesize response
}
```

### AITool (Passive Service)

```go
type AITool struct {
    *core.BaseTool    // NO discovery capability
    aiClient core.AIClient
}

// Can only process requests, cannot discover
func (t *AITool) ProcessWithAI(ctx context.Context, input string) (string, error) {
    return t.aiClient.GenerateResponse(ctx, input, opts)
}
```

### When to Use Each

| Component | Use Case | Example |
|-----------|----------|---------|
| **AIAgent** | Orchestration requiring discovery | Research agent coordinating multiple tools |
| **AITool** | Single-purpose AI capability | Translation tool, Summarization tool |
| **Raw AIClient** | Direct API access | Custom integration |

---

## Logging and Telemetry

### Logging Guidelines

**Where to Log** (all logging uses `core.Logger`):

| Location | Log Level | What to Log |
|----------|-----------|-------------|
| `client.go` | INFO | Client creation, provider selection |
| `registry.go` | INFO/DEBUG | Provider detection, auto-selection |
| `providers/*/factory.go` | INFO | Provider initialization |
| `providers/*/client.go` | INFO/DEBUG | Request/response, errors |
| `providers/base.go` | INFO/WARN | Retries, failures |
| `chain_client.go` | INFO/WARN | Failover events |
| `ai_agent.go` | INFO | Orchestration phases |

**Structured Log Fields**:

```go
// Standard fields for AI operations
logger.Info("AI request initiated", map[string]interface{}{
    "operation":     "ai_request",        // Operation type
    "provider":      "openai",            // Provider name
    "model":         "gpt-4",             // Model used
    "prompt_length": len(prompt),         // Input size
    "max_tokens":    1000,                // Token limit
})

logger.Info("AI response received", map[string]interface{}{
    "operation":         "ai_response",
    "provider":          "openai",
    "model":             "gpt-4",
    "prompt_tokens":     usage.PromptTokens,
    "completion_tokens": usage.CompletionTokens,
    "duration_ms":       duration.Milliseconds(),
    "status":            "success",
})
```

### Telemetry Guidelines

**Metrics to Emit** (using `telemetry` module):

| Metric | Type | Labels | Purpose |
|--------|------|--------|---------|
| `ai.request.duration_ms` | Histogram | provider, model, status | Latency tracking |
| `ai.request.tokens` | Counter | provider, model, token_type | Cost tracking |
| `ai.request.errors` | Counter | provider, error_type | Error rates |
| `ai.chain.failover` | Counter | from_provider, to_provider | Failover frequency |
| `ai.provider.available` | Gauge | provider | Health monitoring |

**Implementation Pattern**:

```go
// In providers/*/client.go
func (c *Client) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
    startTime := time.Now()

    // ... execute request ...

    duration := time.Since(startTime).Milliseconds()

    // Emit metrics (if telemetry is initialized)
    telemetry.RecordAIRequest(telemetry.ModuleAI, "openai", float64(duration), "success")
    telemetry.RecordAITokens(telemetry.ModuleAI, "openai", "prompt", int64(usage.PromptTokens))
    telemetry.RecordAITokens(telemetry.ModuleAI, "openai", "completion", int64(usage.CompletionTokens))

    return result, nil
}
```

### Component Filtering

All AI module logs should use the `framework/ai` component:

```go
// In factory.go
if config.Logger != nil {
    if cal, ok := config.Logger.(core.ComponentAwareLogger); ok {
        config.Logger = cal.WithComponent("framework/ai")
    }
}
```

This enables filtering:
```bash
kubectl logs ... | jq 'select(.component == "framework/ai")'
```

---

## Configuration System

### Configuration Hierarchy

Priority order (highest to lowest):

```
1. Explicit options     → ai.WithAPIKey("sk-...")
2. Provider-specific    → OPENAI_API_KEY, ANTHROPIC_API_KEY
3. GOMIND prefixed      → GOMIND_AI_PROVIDER
4. Defaults             → "auto" detection
```

### Configuration Options

```go
client, err := ai.NewClient(
    // Provider selection
    ai.WithProvider("openai"),           // Explicit provider
    ai.WithProviderAlias("openai.groq"), // OpenAI-compatible service

    // Credentials
    ai.WithAPIKey("sk-..."),             // API key
    ai.WithBaseURL("https://..."),       // Custom endpoint

    // Model configuration
    ai.WithModel("gpt-4"),               // Model selection
    ai.WithTemperature(0.7),             // Generation temperature
    ai.WithMaxTokens(1000),              // Token limit

    // Connection settings
    ai.WithTimeout(30 * time.Second),    // Request timeout
    ai.WithMaxRetries(3),                // Retry count

    // Observability
    ai.WithLogger(logger),               // Logger instance
)
```

### Environment Variable Reference

| Variable | Provider | Description |
|----------|----------|-------------|
| `OPENAI_API_KEY` | OpenAI | API key |
| `OPENAI_BASE_URL` | OpenAI | Custom endpoint |
| `ANTHROPIC_API_KEY` | Anthropic | API key |
| `GEMINI_API_KEY` | Gemini | API key |
| `GOOGLE_API_KEY` | Gemini | Alternative key |
| `AWS_REGION` | Bedrock | AWS region |
| `DEEPSEEK_API_KEY` | OpenAI.DeepSeek | API key |
| `GROQ_API_KEY` | OpenAI.Groq | API key |
| `XAI_API_KEY` | OpenAI.xAI | API key |
| `TOGETHER_API_KEY` | OpenAI.Together | API key |
| `QWEN_API_KEY` | OpenAI.Qwen | API key |

---

## Integration Patterns

### Pattern 1: Direct Client Usage

```go
import (
    "github.com/itsneelabh/gomind/ai"
    _ "github.com/itsneelabh/gomind/ai/providers/openai"
)

func main() {
    client, err := ai.NewClient(
        ai.WithProvider("openai"),
        ai.WithModel("gpt-4"),
    )
    if err != nil {
        log.Fatal(err)
    }

    response, err := client.GenerateResponse(ctx, "Hello!", nil)
}
```

### Pattern 2: With Framework Integration

```go
import (
    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/ai"
    "github.com/itsneelabh/gomind/telemetry"
    _ "github.com/itsneelabh/gomind/ai/providers/openai"
)

func main() {
    // Initialize telemetry
    telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileProduction))
    defer telemetry.Shutdown(context.Background())

    // Create agent with AI
    agent, err := ai.NewAIAgent("research-agent", os.Getenv("OPENAI_API_KEY"))
    if err != nil {
        log.Fatal(err)
    }

    // Create framework
    framework, err := core.NewFramework(agent,
        core.WithLogger(logger),
        core.WithDiscovery(true, "redis"),
    )

    // Run
    framework.Run(context.Background())
}
```

### Pattern 3: Multi-Provider with Failover

```go
import (
    "github.com/itsneelabh/gomind/ai"
    _ "github.com/itsneelabh/gomind/ai/providers/openai"
    _ "github.com/itsneelabh/gomind/ai/providers/anthropic"
)

func main() {
    chain, err := ai.NewChainClient(
        ai.WithProviderChain("openai", "anthropic"),
        ai.WithChainLogger(logger),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Automatically fails over if OpenAI is unavailable
    response, err := chain.GenerateResponse(ctx, prompt, nil)
}
```

---

## Common Pitfalls

### Pitfall 1: Forgetting Provider Import

**Problem**:
```go
import "github.com/itsneelabh/gomind/ai"
// Missing: _ "github.com/itsneelabh/gomind/ai/providers/openai"

client, err := ai.NewClient(ai.WithProvider("openai"))
// Error: provider 'openai' not registered
```

**Solution**:
```go
import (
    "github.com/itsneelabh/gomind/ai"
    _ "github.com/itsneelabh/gomind/ai/providers/openai"  // Add blank import
)
```

### Pitfall 2: Nil Logger in Factory

**Problem**:
```go
// In factory.go (BROKEN)
func (f *Factory) Create(config *ai.AIConfig) core.AIClient {
    var logger core.Logger  // Nil!
    return NewClient(apiKey, baseURL, logger)
}
```

**Symptom**: Silent failures, no logging from provider.

**Solution**:
```go
func (f *Factory) Create(config *ai.AIConfig) core.AIClient {
    logger := config.Logger
    if logger != nil {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            logger = cal.WithComponent("framework/ai")
        }
    }
    return NewClient(apiKey, baseURL, logger)
}
```

### Pitfall 3: Missing Error Handling

**Problem**:
```go
client, _ := ai.NewClient()  // Ignoring error
response, _ := client.GenerateResponse(ctx, prompt, nil)  // Panic if client is nil
```

**Solution**:
```go
client, err := ai.NewClient()
if err != nil {
    log.Fatalf("Failed to create AI client: %v", err)
}

response, err := client.GenerateResponse(ctx, prompt, nil)
if err != nil {
    // Handle error appropriately
}
```

### Pitfall 4: Using Auto-Detection in Production

**Problem**:
```go
// Dangerous in production - provider could change unexpectedly
client, _ := ai.NewClient()  // Uses auto-detection
```

**Solution**:
```go
// Explicit provider selection in production
client, _ := ai.NewClient(
    ai.WithProvider("openai"),  // Explicit
    ai.WithModel("gpt-4"),      // Explicit
)
```

---

## Troubleshooting Guide

### Issue: "provider not registered"

**Diagnostic**:
```go
// Check registered providers
providers := ai.ListProviders()
fmt.Printf("Registered providers: %v\n", providers)
```

**Common Causes**:
1. Missing blank import for provider package
2. Build tags excluding provider (e.g., `//go:build bedrock`)

**Solution**: Add blank import:
```go
import _ "github.com/itsneelabh/gomind/ai/providers/openai"
```

### Issue: "no provider detected in environment"

**Diagnostic**:
```go
info := ai.GetProviderInfo()
for _, p := range info {
    fmt.Printf("%s: available=%v, priority=%d\n", p.Name, p.Available, p.Priority)
}
```

**Common Causes**:
1. API key environment variables not set
2. Using auto-detection with no configured providers

**Solution**: Set required environment variables or use explicit configuration.

### Issue: Silent Failures (No Logs)

**Diagnostic**:
```go
// Check if logger is being passed
client, _ := ai.NewClient(
    ai.WithLogger(myLogger),  // Ensure logger is passed
)
```

**Common Causes**:
1. Logger not passed to `NewClient`
2. Factory not propagating logger to client
3. Nil logger in factory (Gemini, Bedrock bug)

**Solution**: Ensure logger propagation through factory chain.

### Issue: Chain Client Exhausts All Providers

**Diagnostic**: Check logs for failover sequence.

**Common Causes**:
1. All providers have same underlying issue (invalid API key)
2. Request is malformed (4xx errors don't trigger failover)

**Solution**:
- Verify API keys for all providers in chain
- Check if error is client error (4xx) vs server error (5xx)

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-12-14 | Initial architecture documentation |

---

## Related Documentation

- [AI Module Logging/Telemetry Audit](./LOGGING_TELEMETRY_AUDIT.md) - Implementation recommendations
- [Framework Design Principles](../FRAMEWORK_DESIGN_PRINCIPLES.md) - Overall framework architecture
- [Core Module Design](../core/CORE_DESIGN_PRINCIPLES.md) - Core module rules
- [Telemetry Architecture](../telemetry/ARCHITECTURE.md) - Telemetry patterns

---

**Remember**: The AI module abstracts away provider complexity while maintaining the framework's architectural principles. When in doubt, favor explicit configuration over auto-detection, and always propagate loggers through the factory chain for production visibility.
