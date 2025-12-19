# Cross-Provider Model Alias Support

**Status**: ✅ IMPLEMENTED (Critical Bugs Fixed - December 2025)
**Author**: Claude (with user guidance)
**Date**: 2025-12-15
**Last Updated**: 2025-12-16 (OpenAI Factory Auto-Detection Bug FIXED - all 6 critical bugs resolved)
**Related**: [ARCHITECTURE.md](./ARCHITECTURE.md), [Chain Client](./chain_client.go)

---

## Table of Contents

- [Implementation Summary (December 2025)](#implementation-summary-december-2025) **← COMPLETED**
- [Problem Statement](#problem-statement)
- [CRITICAL BUG: Options Mutation During Failover](#critical-bug-options-mutation-during-failover) **← FIXED**
- [CRITICAL BUG: Provider DefaultModel Inheritance](#critical-bug-provider-defaultmodel-inheritance) **← FIXED**
- [CRITICAL BUG: Anthropic Model Name Outdated](#critical-bug-anthropic-model-name-outdated) **← FIXED**
- [CRITICAL BUG: ChainClient Logger Not Propagated](#critical-bug-chainclient-logger-not-propagated) **← FIXED**
- [BUG: OpenAI Factory Auto-Detection Conflicts with Explicit Chains](#bug-openai-factory-auto-detection-conflicts-with-explicit-chains) **← FIXED**
- [Current Architecture](#current-architecture)
- [Recommended Solution: Provider-Local Aliases](#recommended-solution-provider-local-aliases)
- [Implementation Checklist](#implementation-checklist)
- [Testing](#testing)
- [Backward Compatibility](#backward-compatibility)

---

## Implementation Summary (December 2025)

> **Status**: ✅ ALL CRITICAL BUGS FIXED AND VERIFIED
> **Test**: Double failover (OpenAI → Anthropic → Groq) working correctly

### Changes Made

| # | File | Change | Status |
|---|------|--------|--------|
| 1 | `ai/chain_client.go` | Options cloning + original model preservation | ✅ Implemented |
| 2 | `ai/providers/openai/factory.go` | Provider-specific DefaultModel from ModelAliases | ✅ Implemented |
| 3 | `ai/providers/anthropic/client.go` | Updated default model to `claude-sonnet-4-5-20250929` | ✅ Implemented |
| 4 | `ai/chain_client.go` | Added `SetLogger()` for logger propagation | ✅ Implemented |
| 5 | `ai/providers/openai/factory.go` + `client.go` | Use "default" alias for ALL calls to enable env var override | ✅ Implemented |
| 6 | `ai/providers/openai/factory.go` | Added explicit `case "openai":` to prevent auto-detection | ✅ Implemented & Verified |

### Verification Test Results

**Test Setup:**
- OPENAI_API_KEY: `sk-invalid-test-key` (intentionally invalid)
- ANTHROPIC_API_KEY: `sk-invalid-anthropic-key` (intentionally invalid)
- GROQ_API_KEY: Valid key

**Test Request:**
```bash
curl -s -X POST http://localhost:8094/orchestrate/natural \
  -H "Content-Type: application/json" \
  -d '{"request": "What is the weather in London?", "use_ai": true}'
```

**Result:** ✅ SUCCESS
- OpenAI failed (invalid key) → failover triggered
- Anthropic failed (invalid key) → failover triggered
- Groq succeeded with `llama-3.3-70b-versatile` model
- Response: Weather data for London (10.6°C, overcast, 85% humidity)
- Confidence: 95%
- Execution time: 4.3s

### Verification Test 2: Groq-Only Configuration (December 2025)

**Test Setup:**
- OPENAI_API_KEY: ❌ Not set (commented out)
- ANTHROPIC_API_KEY: ❌ Not set (commented out)
- GROQ_API_KEY: ✅ Valid key
- GOMIND_GROQ_MODEL_DEFAULT: `llama-3.3-70b-versatile`
- GOMIND_GROQ_MODEL_SMART: `llama-3.3-70b-versatile`

**Test Request:**
```bash
curl -s -X POST http://localhost:8094/orchestrate/natural \
  -H "Content-Type: application/json" \
  -d '{"request": "What is the weather in London?", "use_ai": true}'
```

**Result:** ✅ SUCCESS

**Log Output (Verified):**
```json
// Step 1: OpenAI skipped - no API key (FIX WORKING!)
{"provider_alias":"openai","provider_index":0,"error":"api_key_missing"}
{"message":"Provider failed in chain, trying next","provider":"openai"}

// Step 2: Anthropic skipped - no API key
{"provider_alias":"anthropic","provider_index":1,"error":"api_key_missing"}
{"message":"Provider failed in chain, trying next","provider":"anthropic"}

// Step 3: Groq succeeds with correct model from env var override
{"provider_alias":"openai.groq","provider_index":2}
{"model":"llama-3.3-70b-versatile","operation":"ai_request"}
{"completion_tokens":220,"prompt_tokens":888,"status":"success"}
{"message":"Chain failover succeeded","successful_provider":"openai.groq"}
```

**Key Verification Points:**
| Check | Before Fix | After Fix |
|-------|-----------|-----------|
| OpenAI with no API key | Auto-detected Groq → `gpt-4o-mini` → 404 error | Returns `api_key_missing` → clean skip ✅ |
| Model used by Groq | N/A (never reached correctly) | `llama-3.3-70b-versatile` (from env var) ✅ |
| Failover chain | Broken (credential-model mismatch) | Clean: openai → anthropic → groq ✅ |

---

## Problem Statement

The AI module's **Chain Client** enables automatic failover between providers, but **model aliases only work for OpenAI-compatible providers**. When using a chain like `["openai", "anthropic"]`:

1. If you pass `Model: "smart"`:
   - OpenAI resolves it to `gpt-4` ✅
   - Anthropic receives `"smart"` literally → falls back to default ❌

2. If you pass `Model: "gpt-4"`:
   - OpenAI uses it correctly ✅
   - Anthropic tries to use `"gpt-4"` → API error ❌

3. If you pass no model:
   - Each provider uses its default (works, but you lose control)

---

## CRITICAL BUG: Options Mutation During Failover

> **Status**: ✅ IMPLEMENTED (December 2025)
> **Priority**: P0 - Critical
> **Affects**: All ChainClient failover scenarios where no model is specified
> **Symptom**: Failover to secondary providers fails with "model not found" errors

### Root Cause

When the primary provider fails and failover occurs, secondary providers receive the **first provider's concrete model name** instead of being able to use their own defaults or resolve aliases.

**Timeline of Events:**

```
Step 1: Orchestrator calls ChainClient.GenerateResponse()
        │
        ├── options = &AIOptions{Temperature: 0.3, MaxTokens: 2000}
        │   └── options.Model = "" (empty)
        │
        └── ChainClient tries Provider 1 (OpenAI)

Step 2: OpenAI.GenerateResponse(ctx, prompt, options)
        │
        ├── ApplyDefaults(options)
        │   └── options.Model = "gpt-4.1-mini-2025-04-14" (OpenAI's default)
        │       ^^^ OPTIONS OBJECT IS MUTATED!
        │
        └── OpenAI API call FAILS (e.g., invalid API key)

Step 3: ChainClient tries Provider 2 (Anthropic) with SAME options
        │
        ├── options.Model = "gpt-4.1-mini-2025-04-14" (from Step 2!)
        │
        ├── ApplyDefaults(options)
        │   └── Model is NOT empty, so no default applied
        │
        ├── resolveModel("gpt-4.1-mini-2025-04-14")
        │   └── Not an alias → passes through unchanged
        │
        └── Anthropic API call: "model gpt-4.1-mini-2025-04-14 not found" ❌
```

### Code Evidence

**1. ChainClient passes same options to all providers** - [chain_client.go:130](chain_client.go#L130):
```go
resp, err := provider.GenerateResponse(ctx, prompt, options)  // Same options for ALL providers!
```

**2. ApplyDefaults mutates the options object** - [providers/base.go:256](providers/base.go#L256):
```go
if options.Model == "" && b.DefaultModel != "" {
    options.Model = b.DefaultModel  // Mutates the shared options!
}
```

**3. Provider resolveModel only handles known aliases** - [providers/anthropic/models.go:80](providers/anthropic/models.go#L80):
```go
if actual, exists := modelAliases[model]; exists {
    return actual
}
return model  // "gpt-4.1-mini-*" passes through unchanged!
```

### Solution: Options Cloning with Original Model Preservation

**Fix Location**: [chain_client.go](chain_client.go)

**Approach**: Clone options for each provider and reset model to original value:

```go
func (c *ChainClient) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
    // Preserve original model setting (empty or alias like "smart")
    originalModel := ""
    if options != nil {
        originalModel = options.Model
    }

    for i, provider := range c.providers {
        // Clone options for each provider to avoid mutation bleeding across providers
        providerOpts := cloneAIOptions(options)

        // Reset model to original so each provider can apply its own defaults/resolution
        if providerOpts != nil {
            providerOpts.Model = originalModel
        }

        resp, err := provider.GenerateResponse(ctx, prompt, providerOpts)
        // ... rest of method unchanged
    }
}

// cloneAIOptions creates a shallow copy of AIOptions
func cloneAIOptions(opts *core.AIOptions) *core.AIOptions {
    if opts == nil {
        return nil
    }
    clone := *opts
    return &clone
}
```

### Why This Works

| Scenario | Before Fix | After Fix |
|----------|-----------|-----------|
| Empty model → OpenAI fails → Anthropic | Anthropic gets `gpt-4.1-mini-*` → **ERROR** | Anthropic gets empty → uses its default ✅ |
| `Model: "smart"` → OpenAI fails → Anthropic | Anthropic gets `gpt-4` → **ERROR** | Anthropic gets `"smart"` → resolves to Claude ✅ |
| Explicit model → OpenAI fails → Anthropic | Anthropic gets explicit model → **ERROR** | Same behavior (expected - explicit models are provider-specific) |

### Telemetry & Logging Requirements

Per [ai/notes/LOGGING_TELEMETRY_AUDIT.md](notes/LOGGING_TELEMETRY_AUDIT.md), failover events should be fully observable:

**Metrics:**
| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ai.chain.attempt` | Counter | provider, status, attempt | Each provider attempt |
| `ai.chain.failover` | Counter | from_provider, to_provider, reason | Successful failover |
| `ai.chain.exhausted` | Counter | providers_tried | All providers failed |

**Spans:**
| Span | Attributes | Description |
|------|-----------|-------------|
| `ai.chain.generate_response` | original_model, providers_count | Parent span for entire chain |
| `ai.chain.provider_attempt` | provider_index, provider_type, model, attempt_status | Each provider attempt |

**Logs (WithContext for trace correlation):**
- `INFO`: Chain request started with original model setting
- `DEBUG`: Each provider attempt with resolved model
- `WARN`: Provider failed, failover to next
- `ERROR`: All providers exhausted with failure details

---

## CRITICAL BUG: Provider DefaultModel Inheritance

> **Status**: ✅ IMPLEMENTED (December 2025)
> **Priority**: P0 - Critical
> **Affects**: All OpenAI-compatible providers (Groq, DeepSeek, xAI, Qwen, Together, Ollama)
> **Symptom**: Even with options cloning fix, failover still fails with "model not found" errors

### Root Cause

All OpenAI-compatible providers inherit the **same DefaultModel** (`gpt-4.1-mini-2025-04-14`) from the base OpenAI client, because `factory.go` didn't set provider-specific defaults.

**Timeline of Events (After Options Cloning Fix):**

```
Step 1: ChainClient tries Provider 1 (OpenAI)
        │
        ├── options.Model = "" (empty, preserved by cloning)
        │
        └── OpenAI.ApplyDefaults(options)
            └── options.Model = "gpt-4.1-mini-2025-04-14" (OpenAI's default)
            └── OpenAI API call FAILS (e.g., invalid API key)

Step 2: ChainClient tries Provider 2 (Groq) with CLONED options
        │
        ├── providerOpts.Model = "" (reset to original ✅)
        │
        └── Groq.ApplyDefaults(providerOpts)
            └── providerOpts.Model = "gpt-4.1-mini-2025-04-14" ← BUG!
                ^^^ Groq inherited OpenAI's default because factory didn't set Groq's default!
            │
            └── Groq API call: "model gpt-4.1-mini-2025-04-14 not found" ❌
```

### Code Evidence

**factory.go (BEFORE fix)** - Line 48:
```go
// NewClient creates client with OpenAI's hardcoded default
client := NewClient(apiKey, baseURL, config.ProviderAlias, logger)
// client.DefaultModel is "gpt-4.1-mini-2025-04-14" from client.go:31

// Only explicit config.Model was applied, not provider-specific defaults
if config.Model != "" {
    client.DefaultModel = config.Model
}
// ^^^ Groq with no explicit model gets OpenAI's default!
```

**client.go** - Line 31:
```go
base.DefaultModel = "gpt-4.1-mini-2025-04-14" // All providers inherit this!
```

### Solution: Provider-Specific DefaultModel from ModelAliases

**Fix Location**: [ai/providers/openai/factory.go:65-84](providers/openai/factory.go#L65-L84)

**Implementation:**
```go
// Apply model defaults
// CRITICAL FIX: Set provider-specific DefaultModel to enable proper failover
// Without this, all OpenAI-compatible providers (Groq, DeepSeek, etc.) inherit
// OpenAI's default model (gpt-4.1-mini-2025-04-14), causing "model not found" errors
// during chain failover when the model name bleeds across providers.
//
// Priority:
// 1. Explicit config.Model (highest)
// 2. Provider-specific default from ModelAliases (e.g., "llama-3.3-70b-versatile" for Groq)
// 3. Client's hardcoded default (lowest) - only for vanilla OpenAI
if config.Model != "" {
    client.DefaultModel = config.Model
} else if config.ProviderAlias != "" && config.ProviderAlias != "openai" {
    // For OpenAI-compatible providers, use their specific default model
    if aliases, exists := ModelAliases[config.ProviderAlias]; exists {
        if defaultModel, hasDefault := aliases["default"]; hasDefault {
            client.DefaultModel = defaultModel
        }
    }
}
```

### ModelAliases with Default Keys

**File**: [ai/providers/openai/models.go](providers/openai/models.go)

Each provider in `ModelAliases` has a `"default"` key:
```go
var ModelAliases = map[string]map[string]string{
    "openai": {
        "default": "gpt-5.2-chat-latest",
        // ... other aliases
    },
    "openai.groq": {
        "default": "llama-3.3-70b-versatile",  // ← Used when no model specified
        "fast":    "llama-3.1-8b-instant",
        "smart":   "llama-3.3-70b-versatile",
        // ...
    },
    "openai.deepseek": {
        "default": "deepseek-chat",
        // ...
    },
    // ... other providers
}
```

### Why This Works

| Provider | Before Fix (DefaultModel) | After Fix (DefaultModel) |
|----------|---------------------------|--------------------------|
| openai | gpt-4.1-mini-2025-04-14 | gpt-4.1-mini-2025-04-14 (unchanged) |
| openai.groq | gpt-4.1-mini-2025-04-14 ❌ | llama-3.3-70b-versatile ✅ |
| openai.deepseek | gpt-4.1-mini-2025-04-14 ❌ | deepseek-chat ✅ |
| openai.xai | gpt-4.1-mini-2025-04-14 ❌ | grok-4 ✅ |

---

## CRITICAL BUG: Anthropic Model Name Outdated

> **Status**: ✅ IMPLEMENTED (December 2025)
> **Priority**: P1 - High
> **Affects**: Anthropic provider in chain failover
> **Symptom**: Anthropic API returns 404 "model: claude-3-5-sonnet-20240620" not found

### Root Cause

The default model in `anthropic/client.go` was outdated (`claude-3-5-sonnet-20240620`), which no longer exists in Anthropic's API.

**Error observed during failover:**
```json
{
  "type": "error",
  "error": {
    "type": "not_found_error",
    "message": "model: claude-3-5-sonnet-20240620"
  }
}
```

### Solution: Update to Current Model

**Fix Location**: [ai/providers/anthropic/client.go:37](providers/anthropic/client.go#L37)

**Before:**
```go
base.DefaultModel = "claude-3-5-sonnet-20240620" // OUTDATED - no longer exists!
```

**After:**
```go
base.DefaultModel = "claude-sonnet-4-5-20250929" // Claude Sonnet 4.5 - recommended per docs.anthropic.com
```

### Source Verification

Model name verified from official Anthropic documentation:
- **URL**: https://docs.anthropic.com/en/docs/about-claude/models
- **Current model**: `claude-sonnet-4-5-20250929` (Claude Sonnet 4.5)
- **Previous model**: `claude-3-5-sonnet-20240620` (deprecated, returns 404)

---

## CRITICAL BUG: ChainClient Logger Not Propagated

> **Status**: ✅ IMPLEMENTED (December 2025)
> **Priority**: P1 - High
> **Affects**: All ChainClient AI operations - logs were silent
> **Symptom**: AI module logs not appearing despite logging fixes in other areas

### Root Cause

The `ChainClient` was missing a `SetLogger()` method. The Framework's logger propagation mechanism uses interface detection to call `SetLogger()` on AI clients:

```go
// core/agent.go - applyConfigToComponent()
if base.AI != nil {
    if loggable, ok := base.AI.(interface{ SetLogger(Logger) }); ok {
        loggable.SetLogger(base.Logger)  // Only called if SetLogger exists!
    }
}
```

When an agent uses `ChainClient` (for provider failover), the interface check failed silently and the `ChainClient` kept using `NoOpLogger`.

**Symptoms:**
- AI module logs were missing (`"component": "framework/ai"`)
- No trace correlation for AI operations
- Debugging failover behavior was impossible

### Solution: Add SetLogger() to ChainClient

**Fix Location**: [ai/chain_client.go:122-137](chain_client.go#L122-L137)

```go
// SetLogger updates the logger after client creation.
// This is called by Framework.applyConfigToComponent() to propagate
// the real logger to the ChainClient after framework initialization.
//
// This fixes the critical bug where ChainClient captures NoOpLogger during
// agent construction (before Framework sets the real logger).
//
// See: ai/notes/LOGGING_TELEMETRY_AUDIT.md - "CRITICAL BUG: AI Module Logger Not Propagated"
func (c *ChainClient) SetLogger(logger core.Logger) {
    if logger == nil {
        c.logger = &core.NoOpLogger{}
    } else if cal, ok := logger.(core.ComponentAwareLogger); ok {
        c.logger = cal.WithComponent("framework/ai")
    } else {
        c.logger = logger
    }

    // Propagate to all underlying providers
    for _, provider := range c.providers {
        if loggable, ok := provider.(interface{ SetLogger(core.Logger) }); ok {
            loggable.SetLogger(logger)
        }
    }
}
```

### Why This Works

The fix follows the documented pattern from [docs/LOGGING_IMPLEMENTATION_GUIDE.md](../docs/LOGGING_IMPLEMENTATION_GUIDE.md) and [core/COMPONENT_LOGGING_DESIGN.md](../core/COMPONENT_LOGGING_DESIGN.md):

1. **Implements the interface contract** - Framework's `applyConfigToComponent()` can now propagate the logger
2. **Wraps with component tag** - Uses `WithComponent("framework/ai")` for proper log attribution
3. **Propagates to providers** - Each underlying `BaseClient` also receives the logger

### Verification

After the fix, AI module logs appear with proper trace correlation:

```json
{
  "component": "framework/ai",
  "level": "INFO",
  "message": "Chain request succeeded on primary provider",
  "operation": "ai_chain_success",
  "provider": "openai",
  "trace.trace_id": "1f78eda9fe6085c98fe7f2ed7a942bbd",
  "trace.span_id": "acfe6052df921bd2"
}
```

---

## BUG: OpenAI Factory Auto-Detection Conflicts with Explicit Chains

> **Status**: ✅ FIXED AND VERIFIED (December 2025)
> **Priority**: P1 - High
> **Affects**: ChainClient with explicit "openai" provider when OPENAI_API_KEY is not set
> **Symptom**: Request sent to wrong API with wrong model name, causing 404 errors before failover
> **Solution**: Solution B - Added explicit `case "openai":` in switch ([factory.go:179-190](providers/openai/factory.go#L179-L190))
> **Verification**: Tested with Groq-only configuration - clean failover, correct model used

### Root Cause

The OpenAI factory's auto-detection feature was designed for **single-client scenarios** (when user sets `Provider: "auto"`), but it also activates when "openai" is explicitly requested in a ChainClient. This creates a **credential-model mismatch**:

1. User explicitly configures chain: `["openai", "anthropic", "openai.groq"]`
2. No `OPENAI_API_KEY` set, but `GROQ_API_KEY` is available
3. OpenAI factory's `resolveCredentials()` auto-detects Groq → uses **Groq's API URL and key**
4. But `providerAlias` remains "openai" → model resolution uses **OpenAI's aliases**
5. Request: Groq API + `gpt-4o-mini` model → **404 error** (Groq doesn't have this model)

### Code Evidence

**1. ChainClient explicitly requests "openai"** - [examples/agent-with-orchestration/research_agent.go:103-104](../examples/agent-with-orchestration/research_agent.go#L103-L104):
```go
aiClient, err := ai.NewChainClient(
    ai.WithProviderChain("openai", "anthropic", "openai.groq"),  // "openai" explicitly requested
)
```

**2. OpenAI factory auto-detects Groq when OPENAI_API_KEY is missing** - [providers/openai/factory.go:186-206](providers/openai/factory.go#L186-L206):
```go
// resolveCredentials() default case for ProviderAlias == "openai":
if os.Getenv("OPENAI_API_KEY") != "" {
    // ... use OpenAI
    return apiKey, baseURL
}

// Priority 95: Groq (ultra-fast inference)
if os.Getenv("GROQ_API_KEY") != "" {
    apiKey = firstNonEmpty(config.APIKey, os.Getenv("GROQ_API_KEY"))
    baseURL = "https://api.groq.com/openai/v1"  // ← Uses Groq's URL
    return apiKey, baseURL
}
```

**3. But model resolution still uses OpenAI's aliases** - [providers/openai/client.go:64](providers/openai/client.go#L64):
```go
options.Model = ResolveModel(c.providerAlias, options.Model)
// c.providerAlias is "openai" (not "openai.groq")
// So "smart" → "gpt-4o-mini" (OpenAI's model, not Groq's!)
```

### Observed Behavior

**Logs from Kubernetes deployment:**
```json
// Step 1: "openai" provider tries with Groq's credentials but OpenAI's model
{"provider_alias":"openai","provider_index":0,"model":"gpt-4o-mini"}

// Step 2: Groq API returns 404 - it doesn't have gpt-4o-mini
{"error":"OpenAI API error (status 404): model gpt-4o-mini does not exist"}

// Step 3: Failover to Anthropic succeeds
{"provider":"anthropic","model":"claude-3-haiku-20240307","status":"success"}
```

**Jaeger trace shows:**
- Span 1: `gpt-4o-mini` call (failed - this is the "openai" provider using Groq backend)
- Span 2: `claude-3-haiku-20240307` call (succeeded - Anthropic failover)

### Impact

1. **Wasted API call**: Request goes to Groq with wrong model, guaranteed to fail
2. **Misleading traces**: Jaeger shows `gpt-4o-mini` even though OpenAI was never actually called
3. **Confusing logs**: Logs say "openai" but traffic goes to Groq
4. **Latency penalty**: Extra round-trip to Groq before failover

### Proposed Solutions

#### Option A: If-Statement Inside Default Case

Add an if-statement inside the `default:` case to check for "openai" explicitly.

**Drawback**: Mixes explicit handling with auto-detection logic in the same case, harder to read.

#### Option B: Add Explicit `case "openai":` in Switch ✅ CHOSEN

Add a dedicated switch case for "openai" parallel to other provider aliases.

**Fix Location**: [ai/providers/openai/factory.go:120-271](providers/openai/factory.go#L120-L271)

```go
func (f *Factory) resolveCredentials(config *ai.AIConfig) (apiKey, baseURL string) {
    switch config.ProviderAlias {
    case "openai.deepseek":
        // ... existing (unchanged)

    case "openai.groq":
        // ... existing (unchanged)

    case "openai.xai":
        // ... existing (unchanged)

    case "openai.qwen":
        // ... existing (unchanged)

    case "openai.together":
        // ... existing (unchanged)

    case "openai.ollama":
        // ... existing (unchanged)

    case "openai":  // NEW: Explicit case for vanilla OpenAI
        // User explicitly requested "openai" - use ONLY OpenAI credentials
        // No auto-detection, no fallback to other providers
        apiKey = firstNonEmpty(config.APIKey, os.Getenv("OPENAI_API_KEY"))
        baseURL = firstNonEmpty(
            config.BaseURL,
            os.Getenv("OPENAI_BASE_URL"),
            "https://api.openai.com/v1",
        )
        return apiKey, baseURL

    default:
        // Empty ProviderAlias ("" or unset) → true auto-detection
        // This is the ONLY path where auto-detection should run
        // ... existing auto-detection logic (unchanged)
    }
}
```

#### Option C: ChainClient Validates API Keys Before Adding Provider

ChainClient checks if the provider's required API key exists before adding to chain.

**Drawback**: Requires ChainClient to know provider-specific requirements, violates separation of concerns.

### Why Solution B is Best

1. **Follows Existing Pattern**: Other providers (deepseek, groq, xai, qwen, together, ollama) already have explicit cases
2. **Clear Separation**: Explicit "openai" request vs auto-detection are separate code paths
3. **Follows Fail-Fast Principle** ([FRAMEWORK_DESIGN_PRINCIPLES.md:164-171](../FRAMEWORK_DESIGN_PRINCIPLES.md#L164-L171)): If "openai" is explicitly requested but OPENAI_API_KEY is not set, the provider returns empty credentials → ChainClient skips it → failover to next provider
4. **Explicit Override Principle** ([FRAMEWORK_DESIGN_PRINCIPLES.md:36](../FRAMEWORK_DESIGN_PRINCIPLES.md#L36)): Explicit configuration should override defaults
5. **Minimal Code Change**: Add ~10 lines, no changes to existing logic
6. **No Breaking Change**: Auto-detection still works when ProviderAlias is empty

### Implementation Details for Solution B

**File**: `ai/providers/openai/factory.go`

**What to do**:
1. Add `case "openai":` before the `default:` case (around line 179)
2. Move the OpenAI-specific credential resolution from the `default` case's first check into this new case
3. The `default:` case then ONLY handles empty `ProviderAlias` for true auto-detection

**Before** (current code in `default:` case):
```go
default:
    // "openai" or empty - vanilla OpenAI or auto-detection fallback

    // Priority 100: OpenAI (vanilla)
    if os.Getenv("OPENAI_API_KEY") != "" {
        apiKey = firstNonEmpty(config.APIKey, os.Getenv("OPENAI_API_KEY"))
        baseURL = firstNonEmpty(config.BaseURL, os.Getenv("OPENAI_BASE_URL"), "https://api.openai.com/v1")
        return apiKey, baseURL
    }

    // Priority 95: Groq (ultra-fast inference)
    if os.Getenv("GROQ_API_KEY") != "" {
        // ... auto-detects Groq even when "openai" was requested! BUG!
    }
    // ... more auto-detection
```

**After** (with Solution B):
```go
case "openai":
    // Explicit "openai" request - use ONLY OpenAI credentials, no auto-detection
    apiKey = firstNonEmpty(config.APIKey, os.Getenv("OPENAI_API_KEY"))
    baseURL = firstNonEmpty(
        config.BaseURL,
        os.Getenv("OPENAI_BASE_URL"),
        "https://api.openai.com/v1",
    )
    return apiKey, baseURL

default:
    // Empty ProviderAlias → true auto-detection
    // This path ONLY runs when user wants automatic provider selection

    // Priority 100: OpenAI (vanilla)
    if os.Getenv("OPENAI_API_KEY") != "" {
        // ... same as before
    }

    // Priority 95: Groq (ultra-fast inference)
    if os.Getenv("GROQ_API_KEY") != "" {
        // ... auto-detection is now correct - only runs when ProviderAlias is empty
    }
    // ... rest of auto-detection unchanged
```

### Expected Behavior After Fix

| Scenario | ProviderAlias | OPENAI_API_KEY | GROQ_API_KEY | Result |
|----------|---------------|----------------|--------------|--------|
| Explicit "openai" in chain | "openai" | ❌ Not set | ✅ Set | Returns empty credentials → ChainClient skips → failover |
| Explicit "openai" in chain | "openai" | ✅ Set | ✅ Set | Uses OpenAI credentials ✅ |
| Auto-detection | "" (empty) | ❌ Not set | ✅ Set | Auto-detects Groq ✅ |
| Auto-detection | "" (empty) | ✅ Set | ✅ Set | Uses OpenAI (highest priority) ✅ |

### Workaround (No Longer Needed - Bug Fixed)

~~Until fixed, users can avoid this issue by:~~

**This workaround is no longer needed.** The bug has been fixed by adding an explicit `case "openai":` in the switch statement. Now when "openai" is explicitly requested in a ChainClient but OPENAI_API_KEY is not set, the provider returns empty credentials and is skipped, allowing proper failover to the next provider.

**Previous workarounds (no longer necessary):**

1. ~~Not including "openai" in chain when OPENAI_API_KEY is not set~~
2. ~~Setting OPENAI_API_KEY even if primary provider is Anthropic~~

---

## Current Architecture

### Where Aliases Live Today

```
ai/providers/openai/models.go
├── ModelAliases map[string]map[string]string
└── ResolveModel(providerAlias, model) string
```

Resolution happens in `openai/factory.go:35-37` at **client creation time**.

### Chain Client Behavior

Looking at [chain_client.go:129](chain_client.go#L129):
```go
resp, err := provider.GenerateResponse(ctx, prompt, options)
```

The **same `options`** (including `options.Model`) goes to **all providers**.

---

## Recommended Solution: Provider-Local Aliases

**Principle**: Each provider handles its own alias resolution. No central registry.

This follows:
- **FRAMEWORK_DESIGN_PRINCIPLES.md**: "Minimal Dependencies", "Interface-First Design"
- **ai/ARCHITECTURE.md**: Providers are self-contained with embedded `BaseClient`
- **Existing pattern**: OpenAI already does this in `models.go`

### Implementation Plan (Accurate)

This section provides exact file locations and required changes.

---

#### Change 1: Extend `ai/providers/anthropic/models.go`

**File**: `ai/providers/anthropic/models.go`
**Module**: `github.com/itsneelabh/gomind/ai/providers/anthropic`
**Current content**: Lines 1-52 contain only struct definitions (AnthropicRequest, Message, etc.)
**Action**: ADD the following after line 52 (after ErrorResponse struct):

```go
import (
    "os"
    "strings"
)

// modelAliases maps portable names to Anthropic model IDs
// These aliases enable portable model names across providers when using Chain Client.
var modelAliases = map[string]string{
    "fast":   "claude-3-haiku-20240307",
    "smart":  "claude-3-5-sonnet-20241022",
    "code":   "claude-3-5-sonnet-20241022",
    "vision": "claude-3-5-sonnet-20241022",
}

// resolveModel returns the actual model name for an alias.
// Priority: 1) Env var override, 2) Hardcoded alias, 3) Pass-through
func resolveModel(model string) string {
    // Check for environment variable override: GOMIND_ANTHROPIC_MODEL_{ALIAS}
    envKey := "GOMIND_ANTHROPIC_MODEL_" + strings.ToUpper(model)
    if override := os.Getenv(envKey); override != "" {
        return override
    }

    // Check hardcoded aliases
    if actual, exists := modelAliases[model]; exists {
        return actual
    }

    return model
}
```

---

#### Change 2: Extend `ai/providers/gemini/models.go`

**File**: `ai/providers/gemini/models.go`
**Module**: `github.com/itsneelabh/gomind/ai/providers/gemini`
**Current content**: Lines 1-78 contain only struct definitions (GeminiRequest, Content, etc.)
**Action**: ADD the following after line 78 (after ErrorResponse struct):

```go
import (
    "os"
    "strings"
)

// modelAliases maps portable names to Gemini model IDs
// These aliases enable portable model names across providers when using Chain Client.
var modelAliases = map[string]string{
    "fast":   "gemini-1.5-flash",
    "smart":  "gemini-1.5-pro",
    "code":   "gemini-1.5-pro",
    "vision": "gemini-1.5-pro",
}

// resolveModel returns the actual model name for an alias.
// Priority: 1) Env var override, 2) Hardcoded alias, 3) Pass-through
func resolveModel(model string) string {
    // Check for environment variable override: GOMIND_GEMINI_MODEL_{ALIAS}
    envKey := "GOMIND_GEMINI_MODEL_" + strings.ToUpper(model)
    if override := os.Getenv(envKey); override != "" {
        return override
    }

    // Check hardcoded aliases
    if actual, exists := modelAliases[model]; exists {
        return actual
    }

    return model
}
```

---

#### Change 3: Add alias resolution in `ai/providers/anthropic/client.go`

**File**: `ai/providers/anthropic/client.go`
**Module**: `github.com/itsneelabh/gomind/ai/providers/anthropic`
**Location**: Line 70 (after `options = c.ApplyDefaults(options)`)
**Action**: ADD one line after line 70:

```go
// Line 69-70 (existing):
    // Apply defaults
    options = c.ApplyDefaults(options)

// ADD after line 70:
    options.Model = resolveModel(options.Model)

// Line 72-73 (existing, unchanged):
    // Add model to span attributes after defaults are applied
    span.SetAttribute("ai.model", options.Model)
```

---

#### Change 4: Add alias resolution in `ai/providers/gemini/client.go`

**File**: `ai/providers/gemini/client.go`
**Module**: `github.com/itsneelabh/gomind/ai/providers/gemini`
**Location**: Line 68 (after `options = c.ApplyDefaults(options)`)
**Action**: ADD one line after line 68:

```go
// Line 67-68 (existing):
    // Apply defaults
    options = c.ApplyDefaults(options)

// ADD after line 68:
    options.Model = resolveModel(options.Model)

// Line 70-71 (existing, unchanged):
    // Add model to span attributes after defaults are applied
    span.SetAttribute("ai.model", options.Model)
```

---

#### Change 5: Add `providerAlias` field to OpenAI Client struct

**File**: `ai/providers/openai/client.go`
**Module**: `github.com/itsneelabh/gomind/ai/providers/openai`
**Location**: Lines 17-21 (Client struct definition)
**Action**: ADD `providerAlias` field:

```go
// Current (lines 17-21):
type Client struct {
    *providers.BaseClient
    apiKey  string
    baseURL string
}

// CHANGE TO:
type Client struct {
    *providers.BaseClient
    apiKey        string
    baseURL       string
    providerAlias string  // NEW: For request-time alias resolution
}
```

---

#### Change 6: Update `NewClient` to accept providerAlias

**File**: `ai/providers/openai/client.go`
**Module**: `github.com/itsneelabh/gomind/ai/providers/openai`
**Location**: Lines 24-37 (NewClient function)
**Action**: ADD `providerAlias` parameter and set it in returned struct:

```go
// Current signature (line 24):
func NewClient(apiKey, baseURL string, logger core.Logger) *Client {

// CHANGE TO:
func NewClient(apiKey, baseURL, providerAlias string, logger core.Logger) *Client {

// Current return (lines 32-36):
    return &Client{
        BaseClient: base,
        apiKey:     apiKey,
        baseURL:    baseURL,
    }

// CHANGE TO:
    return &Client{
        BaseClient:    base,
        apiKey:        apiKey,
        baseURL:       baseURL,
        providerAlias: providerAlias,
    }
```

---

#### Change 7: Add request-time resolution in OpenAI GenerateResponse

**File**: `ai/providers/openai/client.go`
**Module**: `github.com/itsneelabh/gomind/ai/providers/openai`
**Location**: Line 62 (after `options = c.ApplyDefaults(options)`)
**Action**: ADD one line after line 62:

```go
// Line 61-62 (existing):
    // Apply defaults
    options = c.ApplyDefaults(options)

// ADD after line 62:
    options.Model = ResolveModel(c.providerAlias, options.Model)

// Line 64-65 (existing, unchanged):
    // Add model to span attributes after defaults are applied
    span.SetAttribute("ai.model", options.Model)
```

---

#### Change 8: Update factory to pass providerAlias

**File**: `ai/providers/openai/factory.go`
**Module**: `github.com/itsneelabh/gomind/ai/providers/openai`
**Location**: Line 51 (NewClient call) and lines 35-37 (remove creation-time resolution)
**Action**:

**REMOVE** lines 35-37:
```go
// DELETE these lines:
    if config.Model != "" {
        config.Model = ResolveModel(config.ProviderAlias, config.Model)
    }
```

**UPDATE** line 51:
```go
// Current (line 51):
    client := NewClient(apiKey, baseURL, logger)

// CHANGE TO:
    client := NewClient(apiKey, baseURL, config.ProviderAlias, logger)
```

---

#### Change 9: Update OpenAI `ResolveModel` to support env var overrides

**File**: `ai/providers/openai/models.go`
**Module**: `github.com/itsneelabh/gomind/ai/providers/openai`
**Location**: Lines 109-124 (existing ResolveModel function)
**Action**: UPDATE to check env vars first. Add import for "strings" if not present.

```go
// Current (lines 109-124):
func ResolveModel(providerAlias string, model string) string {
    if providerAlias == "" {
        providerAlias = "openai"
    }
    if aliases, exists := ModelAliases[providerAlias]; exists {
        if actualModel, exists := aliases[model]; exists {
            return actualModel
        }
    }
    return model
}

// CHANGE TO:
func ResolveModel(providerAlias string, model string) string {
    if providerAlias == "" {
        providerAlias = "openai"
    }

    // Check for environment variable override: GOMIND_{PROVIDER}_MODEL_{ALIAS}
    // Normalize provider alias: "openai.deepseek" -> "DEEPSEEK", "openai" -> "OPENAI"
    envProvider := providerAlias
    if strings.HasPrefix(providerAlias, "openai.") {
        envProvider = strings.TrimPrefix(providerAlias, "openai.")
    }
    envKey := "GOMIND_" + strings.ToUpper(envProvider) + "_MODEL_" + strings.ToUpper(model)
    if override := os.Getenv(envKey); override != "" {
        return override
    }

    // Check hardcoded aliases
    if aliases, exists := ModelAliases[providerAlias]; exists {
        if actualModel, exists := aliases[model]; exists {
            return actualModel
        }
    }

    return model
}
```

**Note**: Add `"strings"` to the import block at the top of models.go if not already present.

---

## Environment Variable Overrides

Each provider supports runtime model alias overrides via environment variables.

### Pattern

```
GOMIND_{PROVIDER}_MODEL_{ALIAS}={actual_model_name}
```

### Examples

```bash
# Override ALL OpenAI calls (calls without explicit Model specified)
# This is the "default" alias - most important for cost control
export GOMIND_OPENAI_MODEL_DEFAULT=gpt-4o-mini

# Override Anthropic's "smart" alias to use Opus instead of Sonnet
export GOMIND_ANTHROPIC_MODEL_SMART=claude-3-opus-20240229

# Override Gemini's "fast" alias to use the newer 2.0 model
export GOMIND_GEMINI_MODEL_FAST=gemini-2.0-flash

# Override OpenAI's "code" alias
export GOMIND_OPENAI_MODEL_CODE=gpt-4-turbo

# Override OpenAI's "smart" alias (for explicit Model: "smart" calls)
export GOMIND_OPENAI_MODEL_SMART=gpt-4o-mini

# Override for OpenAI-compatible providers (alias prefix is stripped)
export GOMIND_DEEPSEEK_MODEL_DEFAULT=deepseek-chat   # for openai.deepseek
export GOMIND_DEEPSEEK_MODEL_SMART=deepseek-coder    # for openai.deepseek
export GOMIND_GROQ_MODEL_DEFAULT=llama-3.3-70b-versatile  # for openai.groq
export GOMIND_GROQ_MODEL_FAST=llama-3.2-90b-vision   # for openai.groq
export GOMIND_XAI_MODEL_DEFAULT=grok-4               # for openai.xai
export GOMIND_XAI_MODEL_SMART=grok-3                 # for openai.xai
```

### Priority Order

1. **Environment variable** (highest) - `GOMIND_ANTHROPIC_MODEL_SMART`
2. **Hardcoded alias** - `modelAliases["smart"]`
3. **Pass-through** (lowest) - Use model name as-is

This enables:
- Runtime configuration without code changes
- Per-environment model selection (dev vs prod)
- Easy testing with different models
- Kubernetes ConfigMap/Secret integration

### Per-Environment Model Selection

Your code uses portable aliases - the actual model depends on environment variables:

```go
// Same code in all environments
resp, _ := client.GenerateResponse(ctx, prompt, &core.AIOptions{
    Model: "smart",  // Resolved at runtime based on env vars
})
```

**Development** (cheaper/faster models):
```bash
export GOMIND_ANTHROPIC_MODEL_SMART=claude-3-haiku-20240307
export GOMIND_OPENAI_MODEL_SMART=gpt-3.5-turbo
```

**Staging** (mid-tier for testing):
```bash
export GOMIND_ANTHROPIC_MODEL_SMART=claude-3-5-sonnet-20241022
export GOMIND_OPENAI_MODEL_SMART=gpt-4
```

**Production** (best models):
```bash
export GOMIND_ANTHROPIC_MODEL_SMART=claude-3-opus-20240229
export GOMIND_OPENAI_MODEL_SMART=gpt-4-turbo
```

### Kubernetes ConfigMap Example

```yaml
# dev/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ai-model-config
  namespace: dev
data:
  GOMIND_ANTHROPIC_MODEL_SMART: "claude-3-haiku-20240307"
  GOMIND_OPENAI_MODEL_SMART: "gpt-3.5-turbo"

---
# prod/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ai-model-config
  namespace: prod
data:
  GOMIND_ANTHROPIC_MODEL_SMART: "claude-3-opus-20240229"
  GOMIND_OPENAI_MODEL_SMART: "gpt-4-turbo"
```

Reference in deployment:
```yaml
envFrom:
  - configMapRef:
      name: ai-model-config
```

**Key benefit**: Same code artifact, different behavior per environment - controlled entirely through configuration.

---

## What This Solution Does NOT Include

Intentionally excluded for simplicity (following FRAMEWORK_DESIGN_PRINCIPLES.md):

| Feature | Why Excluded |
|---------|--------------|
| Central model alias registry | Adds global state, couples providers |
| `RegisterModelAliases()` API | Adds API surface, rarely needed |
| Query functions | `GetModelAliases()`, etc. add complexity |
| `ProviderName` in BaseClient | Adds coupling between providers |

**If you need a specific model**: Use the explicit model name directly.

```go
// Explicit model names always work and bypass alias resolution
resp, _ := client.GenerateResponse(ctx, prompt, &core.AIOptions{
    Model: "claude-3-opus-20240229",  // Explicit - always works
})
```

---

## Standard Aliases

All providers should support these portable aliases:

| Alias | Intent | OpenAI | Anthropic | Gemini |
|-------|--------|--------|-----------|--------|
| `fast` | Speed/cost optimized | gpt-3.5-turbo | claude-3-haiku | gemini-1.5-flash |
| `smart` | Quality optimized | gpt-4 | claude-3-5-sonnet | gemini-1.5-pro |
| `code` | Code generation | gpt-4 | claude-3-5-sonnet | gemini-1.5-pro |
| `vision` | Multimodal | gpt-4-vision | claude-3-5-sonnet | gemini-1.5-pro |

---

## Implementation Checklist

### Critical Bug Fixes (December 2025) - ✅ COMPLETED

| # | File | Change | Status |
|---|------|--------|--------|
| 1 | `ai/chain_client.go` | Options cloning + original model preservation (L137-177, L338-349) | ✅ Implemented |
| 2 | `ai/providers/openai/factory.go` | Provider-specific DefaultModel from ModelAliases (L65-84) | ✅ Implemented |
| 3 | `ai/providers/anthropic/client.go` | Updated default model to `claude-sonnet-4-5-20250929` (L37) | ✅ Implemented |
| 4 | `ai/chain_client.go` | Added `SetLogger()` for logger propagation (L122-137) | ✅ Implemented |
| 5 | `ai/providers/openai/factory.go` + `client.go` | Use "default" alias for env var override support | ✅ Implemented & Verified |
| 6 | `ai/providers/openai/factory.go` | Added `case "openai":` to prevent auto-detection (L179-190) | ✅ Implemented & Verified |

### Code Details

**1. chain_client.go - Options Cloning (L137-177)**
```go
// Preserve original model setting
originalModel := ""
if options != nil {
    originalModel = options.Model
}

for i, provider := range c.providers {
    // Clone options for each provider
    providerOpts := cloneAIOptions(options)

    // Reset model to original
    if providerOpts != nil {
        providerOpts.Model = originalModel
    }
    // ...
}
```

**2. factory.go - Provider-Specific DefaultModel (L65-84)**
```go
if config.Model != "" {
    client.DefaultModel = config.Model
} else if config.ProviderAlias != "" && config.ProviderAlias != "openai" {
    if aliases, exists := ModelAliases[config.ProviderAlias]; exists {
        if defaultModel, hasDefault := aliases["default"]; hasDefault {
            client.DefaultModel = defaultModel
        }
    }
}
```

**3. anthropic/client.go - Updated Default Model (L37)**
```go
base.DefaultModel = "claude-sonnet-4-5-20250929" // Claude Sonnet 4.5
```

**5. factory.go + client.go - Use "default" Alias for ALL Calls**

This change enables `GOMIND_OPENAI_MODEL_DEFAULT` to override ALL AI calls, not just those with explicit `Model: "smart"`.

**client.go (L30-34):**
```go
base := providers.NewBaseClient(30*time.Second, logger)
// Use "default" alias so ResolveModel() is always called, enabling env var overrides
// The actual model is resolved at request-time via ModelAliases["openai"]["default"]
// or GOMIND_OPENAI_MODEL_DEFAULT env var
base.DefaultModel = "default"
```

**factory.go (L65-87):**
```go
// Apply model defaults
// CRITICAL: Use the "default" ALIAS (not resolved model name) to enable env var overrides
//
// How it works:
// 1. DefaultModel is set to "default" (the alias)
// 2. When GenerateResponse() calls ApplyDefaults(), options.Model = "default"
// 3. ResolveModel() resolves "default" by checking:
//    a. Env var GOMIND_{PROVIDER}_MODEL_DEFAULT (highest priority)
//    b. ModelAliases[provider]["default"] (fallback)
//
// This enables runtime model override for ALL calls via:
//   GOMIND_OPENAI_MODEL_DEFAULT=gpt-4o-mini
//   GOMIND_GROQ_MODEL_DEFAULT=llama-3.2-90b-vision
//
// Priority:
// 1. Explicit config.Model (highest) - use as-is, may be alias or concrete name
// 2. "default" alias (enables env var override for all unspecified models)
if config.Model != "" {
    client.DefaultModel = config.Model
} else {
    // Use "default" alias so ALL calls go through ResolveModel() and respect env vars
    client.DefaultModel = "default"
}
```

**6. factory.go - Explicit `case "openai":` to Prevent Auto-Detection (L179-190)**
```go
case "openai":
    // Explicit "openai" request - use ONLY OpenAI credentials, no auto-detection
    // This fixes the bug where ChainClient with ["openai", "anthropic", "openai.groq"]
    // would auto-detect Groq when OPENAI_API_KEY is not set, causing credential-model mismatch.
    // See: ai/MODEL_ALIAS_CROSS_PROVIDER_PROPOSAL.md - "BUG: OpenAI Factory Auto-Detection"
    apiKey = firstNonEmpty(config.APIKey, os.Getenv("OPENAI_API_KEY"))
    baseURL = firstNonEmpty(
        config.BaseURL,
        os.Getenv("OPENAI_BASE_URL"),
        "https://api.openai.com/v1",
    )
    return apiKey, baseURL
```

### Future Enhancements (Not Yet Implemented)

| # | File | Change | Lines | Status |
|---|------|--------|-------|--------|
| 1 | `ai/providers/anthropic/models.go` | ADD alias map + resolveModel() with env var support | After L52 | ⏳ Planned |
| 2 | `ai/providers/gemini/models.go` | ADD alias map + resolveModel() with env var support | After L78 | ⏳ Planned |
| 3 | `ai/providers/gemini/client.go` | ADD `options.Model = resolveModel(options.Model)` | After L68 | ⏳ Planned |
| 4 | Tests | ADD unit tests for alias resolution + env var overrides | New files | ⏳ Planned |

**Files touched for critical fixes**: 4 files (`chain_client.go`, `factory.go`, `client.go`, `anthropic/client.go`)
**Lines added**: ~60 lines
**Lines modified**: ~10 lines

### ✅ VERIFIED: Environment Variable Alias Override (December 2025)

The `agent-with-orchestration` example was used to verify env var alias override functionality.

**Configuration:**
- K8s deployment: `GOMIND_OPENAI_MODEL_DEFAULT=gpt-4o-mini` + `GOMIND_OPENAI_MODEL_SMART=gpt-4o-mini` ([k8-deployment.yaml:146-150](../examples/agent-with-orchestration/k8-deployment.yaml#L146-L150))
- Local .env: `GOMIND_OPENAI_MODEL_DEFAULT=gpt-4o-mini` + `GOMIND_OPENAI_MODEL_SMART=gpt-4o-mini` ([.env:14-17](../examples/agent-with-orchestration/.env#L14-L17))

**Key Insight:** Using only `GOMIND_OPENAI_MODEL_SMART` was insufficient because:
- The orchestration module's plan generation doesn't specify `Model: "smart"` - it uses the provider's default
- Only explicit `Model: "smart"` calls (like `synthesizeWorkflowResults()`) were being overridden
- ALL other calls used the hardcoded default model (`gpt-4.1-mini-2025-04-14`)

**Solution:** Use "default" alias instead of concrete model name in `factory.go` and `client.go`:
- Changed `client.DefaultModel = "default"` (was hardcoded to `gpt-4.1-mini-2025-04-14`)
- This ensures ALL calls go through `ResolveModel()` which checks `GOMIND_OPENAI_MODEL_DEFAULT` env var

**Verification Results:**

```bash
# Test endpoint used
curl -s -X POST http://localhost:8094/orchestrate/travel-research \
  -H "Content-Type: application/json" \
  -d '{"destination":"Tokyo","use_ai":true}'

# Log output confirmed env var override working for ALL calls
kubectl logs -n gomind-examples -l app=travel-research-agent --tail=100 | grep '"model"'
# Output: "model":"gpt-4o-mini-2024-07-18"  ✅ (env override)
# NOT: "model":"gpt-4.1-mini-2025-04-14"   (old hardcoded default)
```

**Verification Status:** ✅ COMPLETED
- [x] Verify `gpt-4o-mini` appears in logs for ALL AI calls (env override working)
- [x] Verify AI module logs show `"component": "framework/ai"` (SetLogger fix working)
- [x] Both `GOMIND_OPENAI_MODEL_DEFAULT` and `GOMIND_OPENAI_MODEL_SMART` env vars working

### TODO: Documentation Update

After implementing this feature, update `ai/README.md` to include:

1. **Custom Model Aliases via Environment Variables** - Document how developers can create custom aliases without code changes:
   ```bash
   # Pattern: GOMIND_{PROVIDER}_MODEL_{ALIAS}=actual-model-name

   # Create custom aliases for any provider
   export GOMIND_ANTHROPIC_MODEL_REASONING=claude-opus-4-5-20251101
   export GOMIND_OPENAI_MODEL_CHEAP=gpt-4o-mini
   export GOMIND_GEMINI_MODEL_EXPERIMENTAL=gemini-2.0-pro-exp

   # For OpenAI-compatible providers (strip "openai." prefix)
   export GOMIND_DEEPSEEK_MODEL_EXPERIMENTAL=deepseek-v3-beta
   export GOMIND_GROQ_MODEL_TURBO=llama-3.2-90b-vision
   ```
   Then use in code:
   ```go
   resp, _ := client.GenerateResponse(ctx, prompt, &core.AIOptions{
       Model: "reasoning",  // Resolves via env var
   })
   ```

2. **Priority Order** - Document the resolution priority:
   - Environment variable (highest) - `GOMIND_ANTHROPIC_MODEL_REASONING`
   - Hardcoded alias - `modelAliases["reasoning"]`
   - Pass-through (lowest) - Use model name as-is

3. **Per-Environment Configuration** - Show how to use different models per environment via ConfigMaps/Secrets

4. **Custom Provider Implementation** - How custom providers (those not in `ai/providers/`) can implement their own model alias resolution following the same pattern as built-in providers

This ensures developers have clear guidance on creating custom aliases and supporting portable model names.

---

## Testing

### Test Files to Create

**File**: `ai/providers/anthropic/models_test.go`
```go
package anthropic

import (
    "os"
    "testing"
)

func TestResolveModel(t *testing.T) {
    tests := []struct {
        input    string
        expected string
    }{
        {"smart", "claude-3-5-sonnet-20241022"},
        {"fast", "claude-3-haiku-20240307"},
        {"code", "claude-3-5-sonnet-20241022"},
        {"vision", "claude-3-5-sonnet-20241022"},
        {"claude-3-opus-20240229", "claude-3-opus-20240229"}, // Pass-through
        {"unknown-alias", "unknown-alias"},                   // Pass-through
    }

    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            result := resolveModel(tt.input)
            if result != tt.expected {
                t.Errorf("resolveModel(%q) = %q, want %q", tt.input, result, tt.expected)
            }
        })
    }
}

func TestResolveModelEnvOverride(t *testing.T) {
    // Set env var override
    os.Setenv("GOMIND_ANTHROPIC_MODEL_SMART", "claude-3-opus-20240229")
    defer os.Unsetenv("GOMIND_ANTHROPIC_MODEL_SMART")

    result := resolveModel("smart")
    expected := "claude-3-opus-20240229"
    if result != expected {
        t.Errorf("resolveModel with env override: got %q, want %q", result, expected)
    }
}
```

**File**: `ai/providers/gemini/models_test.go`
```go
package gemini

import (
    "os"
    "testing"
)

func TestResolveModel(t *testing.T) {
    tests := []struct {
        input    string
        expected string
    }{
        {"smart", "gemini-1.5-pro"},
        {"fast", "gemini-1.5-flash"},
        {"code", "gemini-1.5-pro"},
        {"vision", "gemini-1.5-pro"},
        {"gemini-2.0-flash", "gemini-2.0-flash"}, // Pass-through
        {"unknown", "unknown"},                    // Pass-through
    }

    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            result := resolveModel(tt.input)
            if result != tt.expected {
                t.Errorf("resolveModel(%q) = %q, want %q", tt.input, result, tt.expected)
            }
        })
    }
}

func TestResolveModelEnvOverride(t *testing.T) {
    // Set env var override
    os.Setenv("GOMIND_GEMINI_MODEL_FAST", "gemini-2.0-flash")
    defer os.Unsetenv("GOMIND_GEMINI_MODEL_FAST")

    result := resolveModel("fast")
    expected := "gemini-2.0-flash"
    if result != expected {
        t.Errorf("resolveModel with env override: got %q, want %q", result, expected)
    }
}
```

**File**: `ai/providers/openai/models_test.go` (add to existing tests)
```go
func TestResolveModelEnvOverride(t *testing.T) {
    tests := []struct {
        name          string
        providerAlias string
        model         string
        envKey        string
        envValue      string
        expected      string
    }{
        {
            name:          "OpenAI env override",
            providerAlias: "openai",
            model:         "smart",
            envKey:        "GOMIND_OPENAI_MODEL_SMART",
            envValue:      "gpt-4-turbo",
            expected:      "gpt-4-turbo",
        },
        {
            name:          "DeepSeek env override",
            providerAlias: "openai.deepseek",
            model:         "code",
            envKey:        "GOMIND_DEEPSEEK_MODEL_CODE",
            envValue:      "deepseek-coder-v2",
            expected:      "deepseek-coder-v2",
        },
        {
            name:          "Groq env override",
            providerAlias: "openai.groq",
            model:         "fast",
            envKey:        "GOMIND_GROQ_MODEL_FAST",
            envValue:      "llama-3.2-90b",
            expected:      "llama-3.2-90b",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            os.Setenv(tt.envKey, tt.envValue)
            defer os.Unsetenv(tt.envKey)

            result := ResolveModel(tt.providerAlias, tt.model)
            if result != tt.expected {
                t.Errorf("ResolveModel(%q, %q) with env = %q, want %q",
                    tt.providerAlias, tt.model, result, tt.expected)
            }
        })
    }
}
```

### Run Tests
```bash
go test ./ai/providers/anthropic -run TestResolveModel -v
go test ./ai/providers/gemini -run TestResolveModel -v
go test ./ai/providers/openai -run TestResolveModel -v
```

---

## Backward Compatibility

- ✅ Explicit model names continue to work
- ✅ OpenAI aliases continue to work (moved, not changed)
- ✅ No API changes
- ✅ No new dependencies

---

## Conclusion

### ✅ Critical Fixes Implemented (December 2025)

Six critical bugs/features were identified and fixed to enable proper ChainClient failover and env var override:

1. **Options Mutation Bug** - Fixed in `chain_client.go` by cloning options and preserving original model
2. **Provider DefaultModel Inheritance Bug** - Fixed in `factory.go` by setting provider-specific defaults from ModelAliases
3. **Anthropic Model Name Bug** - Fixed in `anthropic/client.go` by updating to current model name
4. **ChainClient Logger Bug** - Fixed in `chain_client.go` by adding `SetLogger()` method
5. **"Default" Alias for ALL Calls** - Fixed in `factory.go` + `client.go` by using "default" alias instead of concrete model name, enabling `GOMIND_OPENAI_MODEL_DEFAULT` env var to override ALL AI calls
6. **OpenAI Factory Auto-Detection Bug** - Fixed in `factory.go` by adding explicit `case "openai":` to prevent auto-detection when "openai" is explicitly requested in a ChainClient

### Verified Working

- **Single failover**: OpenAI (fail) → Anthropic (success) ✅
- **Double failover**: OpenAI (fail) → Anthropic (fail) → Groq (success) ✅
- **Model isolation**: Each provider uses its own default model during failover ✅
- **Env var override (ALL calls)**: `GOMIND_OPENAI_MODEL_DEFAULT=gpt-4o-mini` overrides ALL AI calls ✅
- **Env var override (explicit alias)**: `GOMIND_OPENAI_MODEL_SMART=gpt-4o-mini` overrides explicit `Model: "smart"` calls ✅
- **Groq-only configuration**: OpenAI/Anthropic skipped cleanly with `api_key_missing`, Groq succeeds with env var model ✅

### Architecture Benefits

This simplified approach:

1. **Follows existing patterns** - Same as OpenAI already does
2. **Keeps providers independent** - No central registry or coupling
3. **Minimal changes** - ~50 lines added across 3 files
4. **Easy to maintain** - Each provider owns its aliases
5. **No new API surface** - Internal implementation only
6. **Runtime configurable** - Env vars enable per-environment model selection
7. **Backward compatible** - Existing code continues to work
