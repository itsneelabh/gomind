# AI Module Logging and Telemetry Audit

This document provides a comprehensive audit of the `ai` module's compliance with GoMind's logging standards and practical recommendations for improved observability.

> **Status: ✅ IMPLEMENTATION COMPLETE** (December 2024)
>
> All items in this audit have been implemented. See [Implementation Status](#implementation-status) for details.

## Table of Contents

- [Executive Summary](#executive-summary)
- [Guiding Principles](#guiding-principles)
- [File-by-File Analysis](#file-by-file-analysis) *(Original analysis preserved for reference)*
- [Implementation Recommendations](#implementation-recommendations) *(Original recommendations preserved)*
- [Telemetry Strategy](#telemetry-strategy)
- [Implementation Priority](#implementation-priority)
- [Integration Guide: Enabling AI Telemetry in Agents and Tools](#integration-guide-enabling-ai-telemetry-in-agents-and-tools)
- [CRITICAL BUG: AI Module Logger Not Propagated from Framework](#critical-bug-ai-module-logger-not-propagated-from-framework)
- [Implementation Plan: Framework-Driven Logger Propagation](#implementation-plan-framework-driven-logger-propagation)

---

## Executive Summary

This audit identified that the AI module had inconsistent logging practices, with several providers having **broken** logging (nil logger bugs in Gemini and Bedrock factories), **missing** logging (ai_tool.go had zero logging), and **partial** compliance (missing trace correlation and error path logging). The root cause was discovered to be a critical initialization order bug: the Framework's logger was not being propagated to AI clients that were created during agent construction.

**Key Finding**: The Framework's `applyConfigToComponent()` function updated the agent's logger but never propagated it to the AI client, which held a stale reference to `NoOpLogger`. This caused all AI operations to be invisible in production logs and traces.

**Solution Implemented**: Added `SetLogger()` method to `BaseClient` and updated `applyConfigToComponent()` to detect and call this method via interface check, ensuring the real logger is propagated to the AI client after Framework initialization.

### Current Compliance Status (Post-Implementation)

| File | Status | What Was Implemented |
|------|--------|---------------------|
| `client.go` | ✅ Compliant | Already compliant - no changes needed |
| `registry.go` | ✅ Compliant | Already compliant - no changes needed |
| `providers/base.go` | ✅ Compliant | Added SetLogger(), Telemetry field, StartSpan helper, WithContext logging |
| `providers/openai/factory.go` | ✅ Compliant | Added component wrapping, telemetry injection via SetTelemetry |
| `providers/openai/client.go` | ✅ Compliant | Added spans with token attributes, error recording, error path logging |
| `providers/anthropic/factory.go` | ✅ Compliant | Added component wrapping, telemetry injection via SetTelemetry |
| `providers/anthropic/client.go` | ✅ Compliant | Added spans with token attributes, error recording, error path logging |
| `providers/gemini/factory.go` | ✅ Compliant | **Fixed nil logger bug**, added component wrapping, telemetry injection |
| `providers/gemini/client.go` | ✅ Compliant | Added spans with token attributes, error recording, error path logging |
| `providers/bedrock/factory.go` | ✅ Compliant | **Fixed nil logger bug**, added component wrapping, telemetry injection |
| `providers/bedrock/client.go` | ✅ Compliant | Added spans for GenerateResponse, **StreamResponse**, InvokeModel, GetEmbeddings |
| `chain_client.go` | ✅ Compliant | Added component wrapping, failover metrics, WithContext logging |
| `ai_agent.go` | ✅ Compliant | Added WithContext trace correlation throughout |
| `ai_tool.go` | ✅ Compliant | Added logging throughout (was zero logging) |
| `core/agent.go` | ✅ Compliant | **Framework now propagates logger to AI client** via SetLogger() |

### Original Compliance Status (Pre-Implementation)

*Preserved for historical reference - this was the state before implementation:*

| File | Status | Production Impact |
|------|--------|-------------------|
| `client.go` | ✅ Compliant | Good observability |
| `registry.go` | ✅ Compliant | Good observability |
| `providers/base.go` | ⚠️ Partial | Missing trace correlation |
| `providers/openai/factory.go` | ⚠️ Partial | Component mismatch in logs |
| `providers/openai/client.go` | ⚠️ Partial | Error paths have no logging |
| `providers/anthropic/factory.go` | ⚠️ Partial | Component mismatch in logs |
| `providers/anthropic/client.go` | ⚠️ Partial | Error paths have no logging |
| `providers/gemini/factory.go` | ❌ Broken | **Silent failures** |
| `providers/gemini/client.go` | ⚠️ Partial | Error paths have no logging |
| `providers/bedrock/factory.go` | ❌ Broken | **Silent failures** |
| `providers/bedrock/client.go` | ⚠️ Partial | Error paths + **StreamResponse has no logging** |
| `chain_client.go` | ⚠️ Partial | Failover debugging difficult |
| `ai_agent.go` | ⚠️ Partial | Missing trace correlation |
| `ai_tool.go` | ❌ Missing | No visibility at all |

---

## Guiding Principles

### What We Log (High Value)

| Category | Why It Matters | Example |
|----------|----------------|---------|
| **Request boundaries** | Know when operations start/end | "AI request started", "AI request completed" |
| **Failures with context** | Debug root cause quickly | Error + provider + model + attempt count |
| **Performance indicators** | Identify slow operations | Duration, token counts |
| **State transitions** | Understand retry/failover behavior | "Retry attempt 2/3", "Failover to provider B" |

### What We DON'T Log (Low Value / Noise)

| Category | Why to Avoid |
|----------|--------------|
| Every loop iteration | Creates log explosion |
| Successful intermediate steps | Only log boundaries |
| Full prompt/response content at INFO | Security risk + noise (use DEBUG only) |
| Duplicate information | If span captures it, don't also log it |

### Telemetry vs Logging Decision Matrix

| Scenario | Use Logging | Use Telemetry Span | Use Metric |
|----------|-------------|-------------------|------------|
| Operation start/end | ✅ | ✅ | - |
| Error with details | ✅ | ✅ (RecordError) | ✅ (error counter) |
| Token usage | - | ✅ (attribute) | ✅ (for cost tracking) |
| Latency | ✅ (duration_ms) | ✅ (automatic) | ✅ (histogram) |
| Retry attempts | ✅ | ✅ (span event) | ✅ (counter) |
| Failover events | ✅ | ✅ (span event) | ✅ (counter) |

---

## File-by-File Analysis

### 1. client.go ✅ COMPLIANT

**Current State**: Properly implements component-aware logging.

```go
// EXISTING - Lines 27-31 (correct implementation)
if config.Logger != nil {
    if cal, ok := config.Logger.(core.ComponentAwareLogger); ok {
        config.Logger = cal.WithComponent("framework/ai")
    }
    config.Logger.Info("Starting AI client creation", map[string]interface{}{
        "operation":        "ai_client_creation",
        "provider_setting": config.Provider,
        "auto_detect":      config.Provider == string(ProviderAuto),
    })
}
```

**Production Value**: ✅ Already provides good visibility into client creation and provider selection.

**No Changes Needed**

---

### 2. registry.go ✅ COMPLIANT

**Current State**: Proper logging for provider detection.

```go
// EXISTING - Lines 130-135 (correct implementation)
if logger != nil {
    logger.Info("Starting AI provider environment detection", map[string]interface{}{
        "operation":             "ai_provider_detection",
        "registered_providers":  len(registry.providers),
    })
}
```

**Production Value**: ✅ Helps debug "why was provider X selected?" questions.

**No Changes Needed**

---

### 3. providers/base.go ⚠️ PARTIAL COMPLIANCE

**Issues**:
1. No `WithContext` usage - loses trace correlation
2. No component wrapping in constructor

**Current Code**:
```go
// EXISTING - Lines 33-48
func NewBaseClient(timeout time.Duration, logger core.Logger) *BaseClient {
    if logger == nil {
        logger = &core.NoOpLogger{}
    }
    // Missing: Component wrapping
    // Missing: Context parameter for trace correlation

    return &BaseClient{
        HTTPClient: &http.Client{Timeout: timeout},
        Logger:     logger,
        // ...
    }
}
```

**Recommended Changes**:

```go
// NEW - Add SetLogger method for component-aware logging
func (b *BaseClient) SetLogger(logger core.Logger) {
    if logger == nil {
        b.Logger = &core.NoOpLogger{}
    } else if cal, ok := logger.(core.ComponentAwareLogger); ok {
        b.Logger = cal.WithComponent("framework/ai")
    } else {
        b.Logger = logger
    }
}

// MODIFIED - ExecuteWithRetry: Add context to logging (Lines 51-151)
func (b *BaseClient) ExecuteWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
    var lastErr error

    for attempt := 0; attempt <= b.MaxRetries; attempt++ {
        if attempt > 0 && b.Logger != nil && lastErr != nil {
            // CHANGED: Info -> WarnWithContext for trace correlation
            b.Logger.WarnWithContext(ctx, "AI request retry attempt", map[string]interface{}{
                "operation":   "ai_request_retry",
                "attempt":     attempt,
                "max_retries": b.MaxRetries,
                "last_error":  lastErr.Error(),
            })
        }
        // ... existing retry logic ...
    }

    // CHANGED: Error -> ErrorWithContext
    b.Logger.ErrorWithContext(ctx, "AI request failed after all retries", map[string]interface{}{
        "operation":      "ai_request_final_failure",
        "total_attempts": b.MaxRetries + 1,
        "final_error":    lastErr.Error(),
        "error_type":     fmt.Sprintf("%T", lastErr),
    })

    return nil, fmt.Errorf("request failed after %d retries: %w", b.MaxRetries, lastErr)
}

// MODIFIED - LogRequest: Add context parameter (Lines 229-246)
func (b *BaseClient) LogRequestWithContext(ctx context.Context, provider, model, prompt string) {
    // NEW: Use WithContext for trace correlation
    b.Logger.InfoWithContext(ctx, "AI request initiated", map[string]interface{}{
        "operation":     "ai_request",
        "provider":      provider,
        "model":         model,
        "prompt_length": len(prompt),
    })

    // Keep DEBUG level for full content - only visible when debugging
    b.Logger.DebugWithContext(ctx, "AI request prompt content", map[string]interface{}{
        "operation": "ai_request_content",
        "provider":  provider,
        "prompt":    prompt,
    })
}

// MODIFIED - LogResponse: Add context parameter (Lines 249-261)
func (b *BaseClient) LogResponseWithContext(ctx context.Context, provider, model string, tokens core.TokenUsage, duration time.Duration) {
    // NEW: Use WithContext for trace correlation
    b.Logger.InfoWithContext(ctx, "AI response received", map[string]interface{}{
        "operation":         "ai_response",
        "provider":          provider,
        "model":             model,
        "prompt_tokens":     tokens.PromptTokens,
        "completion_tokens": tokens.CompletionTokens,
        "total_tokens":      tokens.TotalTokens,
        "duration_ms":       duration.Milliseconds(),
        "status":            "success",
    })
}
```

**Production Value**:
- **Trace correlation**: When a request is slow, ops can find ALL related logs across services using trace_id
- **Retry visibility**: See exactly which requests are retrying and why
- **No extra noise**: Same log count, just with context

---

### 4. providers/openai/factory.go ⚠️ PARTIAL COMPLIANCE

**Issue**: Logger not wrapped with component.

**Current Code**:
```go
// EXISTING - Lines 26-29
logger := config.Logger
if logger == nil {
    logger = &core.NoOpLogger{}
}
// Missing: WithComponent wrapping
```

**Recommended Change**:

```go
// MODIFIED - Lines 26-34
logger := config.Logger
if logger == nil {
    logger = &core.NoOpLogger{}
} else if cal, ok := logger.(core.ComponentAwareLogger); ok {
    // NEW: Wrap with component for proper log segregation
    logger = cal.WithComponent("framework/ai")
}

// EXISTING - Lines 37-46 (good logging, no changes needed)
logger.Info("OpenAI provider initialized", map[string]interface{}{
    "operation":      "ai_provider_init",
    "provider":       "openai",
    "provider_alias": config.ProviderAlias,
    "base_url":       baseURL,
    "has_api_key":    apiKey != "",
})
```

**Production Value**: Logs will show `"component": "framework/ai"` enabling filtering like:
```bash
kubectl logs ... | jq 'select(.component == "framework/ai")'
```

---

### 4b. providers/openai/client.go ⚠️ PARTIAL COMPLIANCE

**Issue**: Error paths in `GenerateResponse` have no logging - failures are silent until they bubble up.

**Current Code (missing error logging)**:
```go
// EXISTING - Lines 41-43 (no logging on error)
if c.apiKey == "" {
    return nil, fmt.Errorf("OpenAI API key not configured")
}

// EXISTING - Lines 75-78 (no logging on error)
jsonData, err := json.Marshal(reqBody)
if err != nil {
    return nil, fmt.Errorf("failed to marshal request: %w", err)
}

// EXISTING - Lines 81-84 (no logging on error)
req, err := http.NewRequestWithContext(ctx, "POST", ...)
if err != nil {
    return nil, fmt.Errorf("failed to create request: %w", err)
}
```

**Recommended Changes**:

```go
// MODIFIED - Add logging to error paths
if c.apiKey == "" {
    if c.Logger != nil {
        c.Logger.ErrorWithContext(ctx, "OpenAI request failed - API key not configured", map[string]interface{}{
            "operation": "ai_request_error",
            "provider":  "openai",
            "error":     "api_key_missing",
        })
    }
    return nil, fmt.Errorf("OpenAI API key not configured")
}

jsonData, err := json.Marshal(reqBody)
if err != nil {
    if c.Logger != nil {
        c.Logger.ErrorWithContext(ctx, "OpenAI request failed - marshal error", map[string]interface{}{
            "operation": "ai_request_error",
            "provider":  "openai",
            "error":     err.Error(),
            "phase":     "request_preparation",
        })
    }
    return nil, fmt.Errorf("failed to marshal request: %w", err)
}
```

**Production Value**: When requests fail before reaching the API, you'll know exactly why instead of debugging blind.

---

### 5. providers/anthropic/factory.go ⚠️ PARTIAL COMPLIANCE

**Issue**: Same as OpenAI - missing component wrapping.

**Recommended Change**:

```go
// MODIFIED - Lines 49-57
logger := config.Logger
if logger == nil {
    logger = &core.NoOpLogger{}
} else if cal, ok := logger.(core.ComponentAwareLogger); ok {
    // NEW: Wrap with component
    logger = cal.WithComponent("framework/ai")
}

// EXISTING - Lines 54-62 (good, no changes)
logger.Info("Anthropic provider initialized", map[string]interface{}{...})
```

---

### 5b. providers/anthropic/client.go ⚠️ PARTIAL COMPLIANCE

**Issue**: Same pattern as OpenAI - error paths in `GenerateResponse` have no logging.

**Current Code (missing error logging)**:
```go
// EXISTING - Lines 49-51 (no logging on error)
if c.apiKey == "" {
    return nil, fmt.Errorf("anthropic API key not configured")
}

// EXISTING - Lines 81-84, 87-90, 99-101 (no logging on errors)
// - json.Marshal error
// - http.NewRequestWithContext error
// - ExecuteWithRetry error
```

**Recommended Changes**: Same pattern as OpenAI client - add `ErrorWithContext` logging before each error return with provider-specific context.

---

### 6. providers/gemini/factory.go ❌ BROKEN

**Critical Issue**: Logger is nil - ALL Gemini operations are silent!

**Current Code (BROKEN)**:
```go
// EXISTING - Lines 53-57 (BUG!)
// Create logger (nil will use NoOpLogger)
var logger core.Logger  // <-- This is nil!

// Create the client with full configuration
client := NewClient(apiKey, baseURL, logger)  // Passing nil!
```

**Recommended Fix**:

```go
// NEW - Replace lines 53-65
// Use config logger with proper component wrapping
logger := config.Logger
if logger == nil {
    logger = &core.NoOpLogger{}
} else if cal, ok := logger.(core.ComponentAwareLogger); ok {
    logger = cal.WithComponent("framework/ai")
}

// NEW: Add initialization logging (matches OpenAI/Anthropic pattern)
logger.Info("Gemini provider initialized", map[string]interface{}{
    "operation":   "ai_provider_init",
    "provider":    "gemini",
    "base_url":    baseURL,
    "has_api_key": apiKey != "",
    "model":       config.Model,
})

client := NewClient(apiKey, baseURL, logger)
```

**Production Value**:
- **Before**: Gemini failures are completely invisible - ops has no idea why requests fail
- **After**: Same visibility as OpenAI/Anthropic providers

---

### 6b. providers/gemini/client.go ⚠️ PARTIAL COMPLIANCE

**Issue**: Same pattern as OpenAI/Anthropic - error paths in `GenerateResponse` have no logging.

**Current Code (missing error logging)**:
```go
// EXISTING - Lines 47-49 (no logging on error)
if c.apiKey == "" {
    return nil, fmt.Errorf("gemini API key not configured")
}

// EXISTING - Lines 86-89, 94-97, 103-106 (no logging on errors)
// - json.Marshal error
// - http.NewRequestWithContext error
// - ExecuteWithRetry error
```

**Recommended Changes**: Same pattern as OpenAI client - add `ErrorWithContext` logging before each error return with provider-specific context (`"provider": "gemini"`).

---

### 7. providers/bedrock/factory.go ❌ BROKEN

**Critical Issue**: Same as Gemini - logger is nil.

**Recommended Fix**:

```go
// NEW - Replace lines 81-85
// Use config logger with proper component wrapping
logger := config.Logger
if logger == nil {
    logger = &core.NoOpLogger{}
} else if cal, ok := logger.(core.ComponentAwareLogger); ok {
    logger = cal.WithComponent("framework/ai")
}

// NEW: Add initialization logging
logger.Info("Bedrock provider initialized", map[string]interface{}{
    "operation":   "ai_provider_init",
    "provider":    "bedrock",
    "region":      region,
    "model":       config.Model,
})

client := NewClient(awsCfg, region.(string), logger)
```

---

### 7b. providers/bedrock/client.go ⚠️ PARTIAL COMPLIANCE

**Issues**:
1. Error paths in `GenerateResponse` have no logging (same as other providers)
2. **`StreamResponse` method (Lines 154-237) has ZERO logging** - complete blind spot for streaming operations

**Current Code - StreamResponse (NO LOGGING)**:
```go
// EXISTING - Lines 154-237 (no logging at all!)
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, stream chan<- string) error {
    defer close(stream)

    // Apply defaults
    options = c.ApplyDefaults(options)

    // ... entire streaming operation with no visibility ...

    // Start the stream
    output, err := c.bedrockClient.ConverseStream(ctx, input)
    if err != nil {
        return fmt.Errorf("bedrock stream error: %w", err)  // No logging!
    }

    // ... process stream events with no logging ...

    return nil
}
```

**Recommended Changes for StreamResponse**:

```go
// MODIFIED - StreamResponse with logging
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, stream chan<- string) error {
    defer close(stream)
    startTime := time.Now()  // NEW

    options = c.ApplyDefaults(options)

    // NEW: Log stream start
    c.Logger.InfoWithContext(ctx, "Bedrock stream started", map[string]interface{}{
        "operation":     "ai_stream_request",
        "provider":      "bedrock",
        "model":         options.Model,
        "prompt_length": len(prompt),
    })

    // ... build messages ...

    output, err := c.bedrockClient.ConverseStream(ctx, input)
    if err != nil {
        // NEW: Log stream error
        c.Logger.ErrorWithContext(ctx, "Bedrock stream failed to start", map[string]interface{}{
            "operation": "ai_stream_error",
            "provider":  "bedrock",
            "model":     options.Model,
            "error":     err.Error(),
            "phase":     "stream_init",
        })
        return fmt.Errorf("bedrock stream error: %w", err)
    }

    // Process the stream
    eventStream := output.GetStream()
    defer eventStream.Close()

    tokenCount := 0  // NEW: Track streamed tokens
    for {
        event, ok := <-eventStream.Events()
        if !ok {
            break
        }

        switch v := event.(type) {
        case *types.ConverseStreamOutputMemberContentBlockDelta:
            if v.Value.Delta != nil {
                switch d := v.Value.Delta.(type) {
                case *types.ContentBlockDeltaMemberText:
                    tokenCount++  // NEW: Approximate token count
                    select {
                    case stream <- d.Value:
                    case <-ctx.Done():
                        // NEW: Log cancellation
                        c.Logger.WarnWithContext(ctx, "Bedrock stream cancelled", map[string]interface{}{
                            "operation":      "ai_stream_cancelled",
                            "provider":       "bedrock",
                            "tokens_streamed": tokenCount,
                            "duration_ms":    time.Since(startTime).Milliseconds(),
                        })
                        return ctx.Err()
                    }
                }
            }
        case *types.ConverseStreamOutputMemberMessageStop:
            // NEW: Log successful completion
            c.Logger.InfoWithContext(ctx, "Bedrock stream completed", map[string]interface{}{
                "operation":       "ai_stream_complete",
                "provider":        "bedrock",
                "model":           options.Model,
                "tokens_streamed": tokenCount,
                "duration_ms":     time.Since(startTime).Milliseconds(),
                "status":          "success",
            })
            return nil
        }
    }

    if err := eventStream.Err(); err != nil {
        // NEW: Log stream error
        c.Logger.ErrorWithContext(ctx, "Bedrock stream error during processing", map[string]interface{}{
            "operation":       "ai_stream_error",
            "provider":        "bedrock",
            "tokens_streamed": tokenCount,
            "duration_ms":     time.Since(startTime).Milliseconds(),
            "error":           err.Error(),
        })
        return fmt.Errorf("bedrock stream error: %w", err)
    }

    return nil
}
```

**Production Value**:
- **Before**: Streaming operations are completely invisible - no way to debug stream failures or track usage
- **After**: Full visibility into stream lifecycle, token counts, cancellations, and errors

---

### 8. chain_client.go ⚠️ PARTIAL COMPLIANCE

**Issues**:
1. Logger not wrapped with component
2. Failover events need better context for debugging

**Current Code**:
```go
// EXISTING - Lines 56-59
client := &ChainClient{
    providers: make([]core.AIClient, 0, len(config.ProviderAliases)),
    logger:    config.Logger,  // Not wrapped
}
```

**Recommended Changes**:

```go
// MODIFIED - Lines 56-65
// NEW: Wrap logger with component
logger := config.Logger
if logger != nil {
    if cal, ok := logger.(core.ComponentAwareLogger); ok {
        logger = cal.WithComponent("framework/ai")
    }
}

client := &ChainClient{
    providers: make([]core.AIClient, 0, len(config.ProviderAliases)),
    logger:    logger,
}

// MODIFIED - GenerateResponse method (Lines 109-152)
func (c *ChainClient) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
    var lastErr error
    startTime := time.Now()  // NEW: Track total duration

    for i, provider := range c.providers {
        // EXISTING (keep as-is for DEBUG level)
        if c.logger != nil {
            c.logger.DebugWithContext(ctx, "Trying provider", map[string]interface{}{
                "index":    i,
                "provider": fmt.Sprintf("%T", provider),
            })
        }

        resp, err := provider.GenerateResponse(ctx, prompt, options)
        if err == nil {
            // MODIFIED: Add more context for successful failover
            if c.logger != nil && i > 0 {
                c.logger.InfoWithContext(ctx, "Chain failover succeeded", map[string]interface{}{
                    "operation":           "ai_chain_failover",
                    "failed_providers":    i,
                    "successful_index":    i,
                    "total_duration_ms":   time.Since(startTime).Milliseconds(),
                    "status":              "recovered",
                })
            }
            return resp, nil
        }

        lastErr = err

        // MODIFIED: Better failover logging
        if c.logger != nil {
            c.logger.WarnWithContext(ctx, "Provider failed in chain", map[string]interface{}{
                "operation":         "ai_chain_provider_failed",
                "provider_index":    i,
                "remaining":         len(c.providers) - i - 1,
                "error":             err.Error(),
                "is_client_error":   isClientError(err),  // NEW: Help debug if retryable
            })
        }

        if isClientError(err) {
            // NEW: Log why we're not trying other providers
            if c.logger != nil {
                c.logger.ErrorWithContext(ctx, "Chain aborted - client error not retryable", map[string]interface{}{
                    "operation":       "ai_chain_abort",
                    "provider_index":  i,
                    "error":           err.Error(),
                    "duration_ms":     time.Since(startTime).Milliseconds(),
                })
            }
            return nil, fmt.Errorf("client error (not retrying): %w", err)
        }
    }

    // NEW: Final failure log with complete context
    if c.logger != nil {
        c.logger.ErrorWithContext(ctx, "All chain providers failed", map[string]interface{}{
            "operation":        "ai_chain_exhausted",
            "providers_tried":  len(c.providers),
            "final_error":      lastErr.Error(),
            "duration_ms":      time.Since(startTime).Milliseconds(),
        })
    }

    return nil, fmt.Errorf("all %d providers failed, last error: %w", len(c.providers), lastErr)
}
```

**Production Value**:
- **Failover visibility**: Ops can see "Provider 0 failed, 1 failed, 2 succeeded" in one trace
- **Duration tracking**: Know if failovers are adding latency
- **Client error distinction**: Immediately see if error is retryable or not

---

### 9. ai_agent.go ⚠️ PARTIAL COMPLIANCE

**Issue**: Has ctx but uses `Info()` instead of `InfoWithContext()`.

**Current Code**:
```go
// EXISTING - Lines 133-139
if a.Logger != nil {
    a.Logger.Info("Starting ThinkAndAct process", map[string]interface{}{
        "operation":   "ai_think_and_act_start",
        "agent_id":    a.ID,
        "task":        truncateString(task, 150),
        "task_length": len(task),
    })
}
```

**Recommended Change** (apply to ALL methods with ctx):

```go
// MODIFIED - Lines 133-139
if a.Logger != nil {
    // CHANGED: Info -> InfoWithContext for trace correlation
    a.Logger.InfoWithContext(ctx, "Starting ThinkAndAct process", map[string]interface{}{
        "operation":   "ai_think_and_act_start",
        "agent_id":    a.ID,
        "task":        truncateString(task, 150),
        "task_length": len(task),
    })
}
```

**Methods to Update** (same pattern - just add `ctx` as first parameter):
- `ThinkAndAct` - Lines 133, 148, 162, 172, 184, 197, 206, 224
- `ProcessWithAI` - Lines 246, 256, 269, 277, 294, 306, 314, 333, 343, 351, 363, 382
- `DiscoverAndOrchestrate` - Lines 405, 427, 438, 456, 466, 479, 489, 500, 519, 532, 551, 563, 581, 591, 612

**Production Value**: When orchestration is slow, ops can trace the exact sequence across all AI calls using a single trace_id.

---

### 10. ai_tool.go ❌ NO LOGGING

**Issue**: Zero logging - complete blind spot in production.

**Current Code**:
```go
// EXISTING - Lines 20-38 (no logging at all)
func NewAITool(name string, apiKey string) (*AITool, error) {
    tool := core.NewTool(name)

    aiClient, err := NewClient(
        WithProvider("openai"),
        WithAPIKey(apiKey),
        // Missing: WithLogger(tool.Logger)
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create AI client: %w", err)
    }
    // ... no success logging
}
```

**Recommended Changes**:

```go
// MODIFIED - NewAITool (Lines 20-50)
func NewAITool(name string, apiKey string) (*AITool, error) {
    tool := core.NewTool(name)

    // NEW: Pass logger to AI client for visibility
    aiClient, err := NewClient(
        WithProvider("openai"),
        WithAPIKey(apiKey),
        WithLogger(tool.Logger),  // NEW: Enable AI client logging
    )
    if err != nil {
        // NEW: Log creation failure
        if tool.Logger != nil {
            tool.Logger.Error("Failed to create AI client for tool", map[string]interface{}{
                "operation":  "ai_tool_creation",
                "tool_name":  name,
                "error":      err.Error(),
            })
        }
        return nil, fmt.Errorf("failed to create AI client: %w", err)
    }

    // NEW: Log successful creation
    if tool.Logger != nil {
        tool.Logger.Info("AI tool created", map[string]interface{}{
            "operation": "ai_tool_creation",
            "tool_name": name,
            "status":    "success",
        })
    }

    tool.AI = aiClient
    return &AITool{BaseTool: tool, aiClient: aiClient}, nil
}

// MODIFIED - ProcessWithAI (Lines 41-52)
func (t *AITool) ProcessWithAI(ctx context.Context, input string) (string, error) {
    startTime := time.Now()  // NEW

    // NEW: Log request start (only at DEBUG to avoid noise for high-volume tools)
    if t.Logger != nil {
        t.Logger.DebugWithContext(ctx, "AI tool processing request", map[string]interface{}{
            "operation":    "ai_tool_process",
            "input_length": len(input),
        })
    }

    response, err := t.aiClient.GenerateResponse(ctx, input, &core.AIOptions{
        Model:       "gpt-3.5-turbo",
        Temperature: 0.7,
        MaxTokens:   500,
    })
    if err != nil {
        // NEW: Log failure with context
        if t.Logger != nil {
            t.Logger.ErrorWithContext(ctx, "AI tool processing failed", map[string]interface{}{
                "operation":   "ai_tool_process",
                "error":       err.Error(),
                "duration_ms": time.Since(startTime).Milliseconds(),
            })
        }
        return "", fmt.Errorf("AI processing failed: %w", err)
    }

    // NEW: Log success with metrics
    if t.Logger != nil {
        t.Logger.InfoWithContext(ctx, "AI tool processing completed", map[string]interface{}{
            "operation":         "ai_tool_process",
            "duration_ms":       time.Since(startTime).Milliseconds(),
            "prompt_tokens":     response.Usage.PromptTokens,
            "completion_tokens": response.Usage.CompletionTokens,
            "status":            "success",
        })
    }

    return response.Content, nil
}

// MODIFIED - RegisterAICapability handler (Lines 55-83)
func (t *AITool) RegisterAICapability(name, description, prompt string) {
    capability := core.Capability{
        Name:        name,
        Description: description,
        Endpoint:    fmt.Sprintf("/ai/%s", name),
        Handler: func(w http.ResponseWriter, r *http.Request) {
            ctx := r.Context()  // NEW: Get context for tracing
            startTime := time.Now()  // NEW

            body, err := io.ReadAll(r.Body)
            if err != nil {
                // NEW: Log read failure
                if t.Logger != nil {
                    t.Logger.ErrorWithContext(ctx, "Failed to read request body", map[string]interface{}{
                        "operation":  "ai_capability_request",
                        "capability": name,
                        "error":      err.Error(),
                    })
                }
                http.Error(w, "Failed to read request", http.StatusBadRequest)
                return
            }

            fullPrompt := fmt.Sprintf("%s\n\nInput: %s", prompt, string(body))
            response, err := t.ProcessWithAI(ctx, fullPrompt)  // MODIFIED: Pass ctx
            if err != nil {
                // Logging already handled in ProcessWithAI
                http.Error(w, "AI processing failed", http.StatusInternalServerError)
                return
            }

            // NEW: Log successful capability execution
            if t.Logger != nil {
                t.Logger.InfoWithContext(ctx, "AI capability executed", map[string]interface{}{
                    "operation":   "ai_capability_complete",
                    "capability":  name,
                    "duration_ms": time.Since(startTime).Milliseconds(),
                    "status":      "success",
                })
            }

            w.Header().Set("Content-Type", "text/plain")
            w.Write([]byte(response))
        },
    }

    t.RegisterCapability(capability)
}
```

**Production Value**:
- **Before**: AI tool failures invisible - "why did summarization fail?"
- **After**: Clear error + duration + token usage for debugging
- **Note**: Uses DEBUG for request start to avoid log explosion for high-volume tools

---

## Telemetry Strategy

### Architecture Note

**The AI module can import telemetry**: Per [FRAMEWORK_DESIGN_PRINCIPLES.md](../FRAMEWORK_DESIGN_PRINCIPLES.md), the valid dependency is `ai → core + telemetry`. This allows the AI module to emit metrics and create spans for production observability.

Use the `telemetry.ModuleAI` constant (defined in `telemetry/unified_metrics.go`) for all metrics to enable consistent filtering and dashboard queries.

### Principle: Telemetry for Aggregation, Logs for Details

| Data Type | Telemetry (Spans/Metrics) | Logs |
|-----------|---------------------------|------|
| Request count | ✅ Counter | - |
| Error rate | ✅ Counter by error_type | ✅ With full error message |
| Latency distribution | ✅ Histogram | ✅ Individual duration_ms |
| Token usage | ✅ Counter for cost tracking | ✅ Per-request for debugging |
| Retry count | ✅ Counter | ✅ With attempt details |
| Failover events | ✅ Counter | ✅ With provider sequence |

### Recommended Telemetry Additions

#### 1. providers/base.go - Request Metrics

```go
import "github.com/itsneelabh/gomind/telemetry"

// NEW - Add to LogResponseWithContext method
func (b *BaseClient) LogResponseWithContext(ctx context.Context, provider, model string, tokens core.TokenUsage, duration time.Duration) {
    // EXISTING logging...

    // NEW: Emit metrics using unified helpers with ModuleAI
    telemetry.RecordAIRequest(telemetry.ModuleAI, provider,
        float64(duration.Milliseconds()), "success")

    // Token usage for cost tracking (prompt + completion separately)
    telemetry.RecordAITokens(telemetry.ModuleAI, provider, "prompt", int64(tokens.PromptTokens))
    telemetry.RecordAITokens(telemetry.ModuleAI, provider, "completion", int64(tokens.CompletionTokens))
}

// NEW - Add to ExecuteWithRetry for retry tracking
func (b *BaseClient) ExecuteWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
    // ... existing logic ...

    for attempt := 0; attempt <= b.MaxRetries; attempt++ {
        if attempt > 0 {
            // NEW: Track retry attempts with ModuleAI label
            telemetry.Counter("ai.retries.total",
                "module", telemetry.ModuleAI,
                "provider", provider,
                "attempt", fmt.Sprintf("%d", attempt),
            )
        }
        // ... rest of retry logic ...
    }
}
```

**Dashboard Value**:
- Token usage trends over time (cost monitoring)
- P50/P95/P99 latency by provider
- Retry rate spikes indicate provider issues

#### 2. chain_client.go - Failover Metrics

```go
import "github.com/itsneelabh/gomind/telemetry"

// NEW - Add to GenerateResponse method
func (c *ChainClient) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
    // ... on successful failover (i > 0) ...
    telemetry.Counter("ai.chain.failover",
        "module", telemetry.ModuleAI,
        "failed_count", fmt.Sprintf("%d", i),
        "successful_index", fmt.Sprintf("%d", i),
    )

    // ... on final failure ...
    telemetry.Counter("ai.chain.exhausted",
        "module", telemetry.ModuleAI,
        "providers_tried", fmt.Sprintf("%d", len(c.providers)),
    )
}
```

**Dashboard Value**:
- Failover frequency (is primary provider unstable?)
- Which backup providers are saving requests

#### 3. Provider Clients - Span Creation

Add spans ONLY for the main GenerateResponse method (not every helper):

```go
import "github.com/itsneelabh/gomind/telemetry"

// NEW - Example for providers/openai/client.go GenerateResponse
func (c *Client) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
    // NEW: Create span for the AI request
    ctx, span := telemetry.StartSpan(ctx, "ai.generate_response")
    defer span.End()

    // Set span attributes for debugging and filtering
    span.SetAttributes(
        attribute.String("module", telemetry.ModuleAI),
        attribute.String("ai.provider", "openai"),
        attribute.String("ai.model", options.Model),
        attribute.Int("ai.prompt_length", len(prompt)),
    )

    // ... existing logic ...

    // On success, add result attributes
    span.SetAttributes(
        attribute.Int("ai.prompt_tokens", result.Usage.PromptTokens),
        attribute.Int("ai.completion_tokens", result.Usage.CompletionTokens),
    )

    // On error
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }

    return result, nil
}
```

**Trace Value**: See AI calls as spans in Jaeger, with duration and token counts visible. Filter by `module=ai` to see all AI-related spans.

### What NOT to Add (Avoiding Excess)

| Don't Add | Why | Alternative (see below) |
|-----------|-----|-------------------------|
| ~~Span for each retry attempt~~ | ~~Span events are sufficient~~ | **✅ IMPLEMENTED** - Now in `ai.http_attempt` spans |
| Span for JSON marshaling | Too granular, adds overhead | Parent span + error recording |
| Metric for prompt length | Low value, high cardinality risk | Span attribute or log query |
| Full prompt in span attributes | Security risk + size limits | DEBUG logs only |
| Span for configuration loading | One-time operation, use logs | Startup logs |

---

## Alternative Data Extraction Mechanisms

For items intentionally not added as spans/metrics, here's how developers can access the data when debugging:

### 1. Retry Attempts ✅ IMPLEMENTED

**Now available as spans!** Each retry creates an `ai.http_attempt` child span visible in Jaeger:

```
ai.generate_response (parent span)
├── ai.http_attempt (attempt=1, status=server_error, duration=1200ms)
├── ai.http_attempt (attempt=2, status=server_error, duration=1100ms)
└── ai.http_attempt (attempt=3, status=success, duration=850ms)
```

**Span Attributes:**
- `ai.attempt` - Attempt number (1, 2, 3...)
- `ai.max_retries` - Maximum retry count configured
- `ai.is_retry` - Boolean, true if this is a retry (not first attempt)
- `ai.attempt_status` - "success", "server_error", "network_error", "client_error"
- `ai.attempt_duration_ms` - Duration of this specific attempt
- `ai.previous_error` - Error from previous attempt (on retries)
- `ai.retryable` - Whether the error was retryable
- `http.status_code` - HTTP status code (when available)

**Jaeger Query:**
```
service=my-service operation=ai.http_attempt ai.is_retry=true
```

### 2. JSON Marshaling Timing

**Why not a span:** Marshaling AI payloads typically takes <1ms. Adding spans would be pure noise.

**How to debug if needed:**
```go
// Add temporary timing in your application code:
start := time.Now()
jsonData, err := json.Marshal(reqBody)
log.Printf("Marshal took %v", time.Since(start))
```

**Alternative:** If marshaling fails, the error is captured in the parent span via `span.RecordError()`. The parent span's total duration includes marshaling time.

### 3. Prompt Length Analysis

**Why not a metric:** Each unique prompt length would create a new time series, causing cardinality explosion in Prometheus.

**How to access:**

1. **From Spans (Jaeger):**
   ```
   # Query spans with large prompts
   service=my-service ai.prompt_length>10000
   ```

2. **From Logs:**
   ```bash
   # Find requests with large prompts
   kubectl logs ... | jq 'select(.operation == "ai_request" and .prompt_length > 10000)'
   ```

3. **Custom Bucketed Metric (if truly needed):**
   ```go
   // Add in your application layer if aggregate analysis is required
   bucket := "small"
   switch {
   case len(prompt) > 10000: bucket = "xlarge"
   case len(prompt) > 5000:  bucket = "large"
   case len(prompt) > 1000:  bucket = "medium"
   }
   telemetry.Counter("app.ai.prompt_size", "bucket", bucket)
   ```

### 4. Full Prompt Content

**Why not in spans:** Security risk - prompts may contain PII, API keys, or sensitive business data. Span attributes are often stored long-term and broadly accessible.

**How to access:**

1. **DEBUG Logs (Recommended):**
   ```bash
   # Enable debug logging for AI module
   LOG_LEVEL=debug go run ./...

   # Filter for prompt content
   kubectl logs ... | jq 'select(.operation == "ai_request_content")'
   ```

2. **Temporary Code (Development Only):**
   ```go
   // NEVER commit this to production
   log.Printf("Prompt: %s", prompt)
   ```

3. **Request Logging Middleware:**
   ```go
   // Add at application boundary with proper access controls
   if os.Getenv("ENABLE_PROMPT_LOGGING") == "true" {
       secureLog.Info("AI prompt", "prompt", prompt, "user_id", userID)
   }
   ```

### 5. Configuration Loading Timing

**Why not a span:** Happens once at startup, not per-request. Would create orphan spans with no parent context.

**How to access:**

1. **Startup Logs:**
   ```bash
   # Provider initialization is logged at INFO level
   kubectl logs ... | jq 'select(.operation == "ai_provider_init")'
   ```

2. **Startup Timing (if needed):**
   ```go
   // Add in your main.go if startup time is a concern
   start := time.Now()
   client, err := ai.NewClient(...)
   log.Printf("AI client creation took %v", time.Since(start))
   ```

---

## Implementation Status

### ✅ COMPLETED

All priority items have been implemented:

| Priority | Category | Status |
|----------|----------|--------|
| 1 | Critical (nil logger fixes) | ✅ Complete |
| 2 | High (error path logging, component wrapping) | ✅ Complete |
| 3 | Medium (WithContext trace correlation) | ✅ Complete |
| 4 | Enhancement (spans, metrics, dashboards) | ✅ Complete |

### Implementation Details

#### Provider Spans (All Providers)
- `ai.generate_response` - Main request span with token usage attributes
- `ai.stream_response` - Streaming span (Bedrock) with streaming-specific attributes
- `ai.invoke_model` - Direct model invocation (Bedrock)
- `ai.get_embeddings` - Embedding generation (Bedrock)
- `ai.http_attempt` - Individual retry attempt spans

#### Span Attributes (Consistent Across Providers)
| Attribute | Description |
|-----------|-------------|
| `ai.provider` | Provider name (openai, anthropic, gemini, bedrock) |
| `ai.model` | Model identifier |
| `ai.prompt_length` | Input prompt character count |
| `ai.prompt_tokens` | Tokens consumed by prompt |
| `ai.completion_tokens` | Tokens in response |
| `ai.total_tokens` | Total token usage |
| `ai.response_length` | Response content length |
| `ai.region` | AWS region (Bedrock only) |
| `ai.streaming` | Boolean for streaming requests |
| `ai.stream_completed` | Boolean when stream completes normally |
| `ai.stream_cancelled` | Boolean when stream is cancelled |

#### Metrics
| Metric | Type | Labels |
|--------|------|--------|
| `ai.request.duration_ms` | Histogram | module, provider, status |
| `ai.tokens.total` | Counter | module, provider, type (input/output) |
| `ai.request.retries` | Counter | module |
| `ai.request.failures` | Counter | module, reason |
| `ai.chain.failover` | Counter | module, failed_count |
| `ai.chain.exhausted` | Counter | module, providers_tried |

---

## Implementation Priority (Reference)

### Priority 1: Critical (Blocks Debugging)

| Task | File | Effort | Impact |
|------|------|--------|--------|
| Fix nil logger | `providers/gemini/factory.go` | 10 min | Gemini visibility |
| Fix nil logger | `providers/bedrock/factory.go` | 10 min | Bedrock visibility |

### Priority 2: High (Production Debugging)

| Task | File | Effort | Impact |
|------|------|--------|--------|
| Add component wrapping | `providers/openai/factory.go` | 5 min | Log filtering |
| Add component wrapping | `providers/anthropic/factory.go` | 5 min | Log filtering |
| Add component wrapping | `chain_client.go` | 5 min | Log filtering |
| Add logging | `ai_tool.go` | 30 min | Tool visibility |
| Add error path logging | `providers/openai/client.go` | 15 min | Pre-API error visibility |
| Add error path logging | `providers/anthropic/client.go` | 15 min | Pre-API error visibility |
| Add error path logging | `providers/gemini/client.go` | 15 min | Pre-API error visibility |
| Add error path logging | `providers/bedrock/client.go` | 15 min | Pre-API error visibility |
| Add StreamResponse logging | `providers/bedrock/client.go` | 20 min | **Streaming visibility** |

### Priority 3: Medium (Trace Correlation)

| Task | File | Effort | Impact |
|------|------|--------|--------|
| Add WithContext | `providers/base.go` | 20 min | Trace-log correlation |
| Add WithContext | `ai_agent.go` | 30 min | Trace-log correlation |

### Priority 4: Enhancement (Dashboards)

| Task | File | Effort | Impact |
|------|------|--------|--------|
| Add token metrics | `providers/base.go` | 15 min | Cost dashboards |
| Add failover metrics | `chain_client.go` | 15 min | Reliability dashboards |
| Add provider spans | All provider clients | 1 hour | Trace visualization |

---

## Verification Commands

After implementing changes:

```bash
# 1. Verify component field appears
go run ./examples/ai-agent-example/ 2>&1 | grep '"component":"framework/ai"'

# 2. Verify Gemini/Bedrock now log
GEMINI_API_KEY=test go test -v ./ai/providers/gemini/... 2>&1 | grep "ai_provider_init"

# 3. Check trace correlation (look for trace.trace_id in logs)
curl -H "traceparent: 00-abc123def456-789012-01" http://localhost:8080/api/generate
kubectl logs ... | jq 'select(.["trace.trace_id"] == "abc123def456")'

# 4. Verify metrics (after telemetry additions)
curl -s http://localhost:9090/api/v1/query?query=ai_tokens_total | jq '.data.result'
```

---

## Summary

### Implementation Complete ✅

| Category | Before | After |
|----------|--------|-------|
| Silent failures | Gemini, Bedrock factories | None - all providers log |
| Error path logging | Missing in all provider clients | Full visibility with span recording |
| Streaming visibility | None (Bedrock StreamResponse) | Full lifecycle logging + spans |
| Trace correlation | 2 files | All files with ctx |
| Component filtering | Inconsistent | All "framework/ai" |
| Failover debugging | Minimal | Full sequence visible + metrics |
| Metrics | None | Token, latency, retry, failover |
| **Distributed Tracing** | **None** | **Full spans for all providers + retries** |

### What's Now Visible in Jaeger

```
my-service
└── ai.generate_response (provider=openai, model=gpt-4, total_tokens=1523)
    ├── ai.http_attempt (attempt=1, status=server_error, duration=1200ms)
    ├── ai.http_attempt (attempt=2, status=server_error, duration=1100ms)
    └── ai.http_attempt (attempt=3, status=success, duration=850ms)
```

### Key Files Modified

#### Production Files (16 files)

| File | Changes |
|------|---------|
| `ai/providers/base.go` | Added Telemetry field, StartSpan helper, retry attempt spans, WithContext logging |
| `ai/providers/openai/client.go` | Added spans with token attributes, error recording, error path logging |
| `ai/providers/openai/factory.go` | Added component wrapping, telemetry injection via SetTelemetry |
| `ai/providers/anthropic/client.go` | Added spans with token attributes, error recording, error path logging |
| `ai/providers/anthropic/factory.go` | Added component wrapping, telemetry injection via SetTelemetry |
| `ai/providers/gemini/client.go` | Added spans with token attributes, error recording, error path logging |
| `ai/providers/gemini/factory.go` | Fixed nil logger bug, added component wrapping, telemetry injection |
| `ai/providers/bedrock/client.go` | Added spans for GenerateResponse, StreamResponse, InvokeModel, GetEmbeddings |
| `ai/providers/bedrock/factory.go` | Fixed nil logger bug, added component wrapping, telemetry injection |
| `ai/provider.go` | Added Telemetry field to AIConfig, WithTelemetry option |
| `ai/client.go` | Component-aware logging (already compliant) |
| `ai/registry.go` | Provider detection logging (already compliant) |
| `ai/chain_client.go` | Added component wrapping, failover metrics, WithContext logging |
| `ai/ai_agent.go` | Added WithContext trace correlation |
| `ai/ai_tool.go` | Added logging throughout (was zero logging) |
| `telemetry/unified_metrics.go` | Added `ModuleAI` constant for AI module metrics |

#### Test Files (3 files)

| File | Changes |
|------|---------|
| `ai/ai_tool_capability_test.go` | Added tests for RegisterAICapability, WithAIToolLogger |
| `ai/provider_test.go` | Added tests for WithTelemetry option, provider constants |
| `ai/providers/base_test.go` | Added tests for StartSpan, ExecuteWithRetry, telemetry integration |

#### Documentation Files (2 files)

| File | Changes |
|------|---------|
| `ai/ARCHITECTURE.md` | Added Logging and Telemetry section |
| `telemetry/ARCHITECTURE.md` | Added module import permissions table, UI telemetry planned note |

**Total files**: 21 (16 production, 3 test, 2 documentation)
**Total span types**: 5 (generate_response, stream_response, invoke_model, get_embeddings, http_attempt)
**Total new metrics**: 6 (request duration, tokens, retries, failures, failover, exhausted)
**Overhead**: Minimal - all telemetry is conditional on being initialized

---

## Integration Guide: Enabling AI Telemetry in Agents and Tools

This section documents the **critical initialization order** required for agents and tools that use the AI module to correctly emit distributed traces.

### The Problem

When an agent or tool creates an AI client using `ai.NewClient(ai.WithTelemetry(...))`, the telemetry provider must already be initialized. If telemetry is initialized **after** the AI client is created, the AI client receives `nil` for its telemetry provider and no spans will be created.

### ❌ WRONG: Telemetry Initialized After AI Client

```go
func main() {
    // 1. Create agent (which creates AI client internally)
    agent, err := NewMyAgent()  // AI client created here with nil telemetry!

    // 2. Initialize telemetry AFTER - TOO LATE!
    telemetry.Initialize(config)

    // Result: AI operations emit NO spans to Jaeger
}

func NewMyAgent() (*MyAgent, error) {
    // This call happens BEFORE telemetry.Initialize()
    // telemetry.GetTelemetryProvider() returns nil here!
    aiClient, err := ai.NewClient(
        ai.WithTelemetry(telemetry.GetTelemetryProvider()), // Returns nil!
    )
    // aiClient.Telemetry is nil - no spans will be created
}
```

### ✅ CORRECT: Telemetry Initialized Before AI Client

```go
func main() {
    // 1. Validate configuration first (fail fast)
    if err := validateConfig(); err != nil {
        log.Fatalf("Configuration error: %v", err)
    }

    // 2. Set component type FIRST for telemetry auto-inference
    // This must happen before telemetry initialization so service_type is set correctly
    core.SetCurrentComponentType(core.ComponentTypeAgent)

    // 3. Initialize telemetry BEFORE agent creation
    // This ensures telemetry.GetTelemetryProvider() returns a valid provider
    initTelemetry("my-service-name")
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        telemetry.Shutdown(ctx)
    }()

    // 4. Create agent AFTER telemetry is initialized
    // The AI client will now receive the telemetry provider for distributed tracing
    agent, err := NewMyAgent()
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }

    // Result: AI operations emit spans to Jaeger ✅
}
```

### Required Steps

| Step | What | Why |
|------|------|-----|
| 1 | `core.SetCurrentComponentType(core.ComponentTypeAgent)` | Sets component type for telemetry service_type auto-inference |
| 2 | `telemetry.Initialize(config)` | Initializes the global telemetry provider |
| 3 | `ai.NewClient(ai.WithTelemetry(...))` | AI client receives the initialized telemetry provider |

### In the Agent/Tool Constructor

When creating the AI client in your agent or tool constructor, always pass telemetry:

```go
func NewMyAgent() (*MyAgent, error) {
    agent := core.NewBaseAgent("my-agent")

    // Pass telemetry to enable distributed tracing for AI operations
    aiClient, err := ai.NewClient(
        ai.WithTelemetry(telemetry.GetTelemetryProvider()),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create AI client: %w", err)
    }

    // Store AI client in agent
    agent.AI = aiClient

    return &MyAgent{BaseAgent: agent, aiClient: aiClient}, nil
}
```

### Verification

After implementing the correct initialization order, verify AI traces appear in Jaeger:

```bash
# 1. Send an AI request
curl -X POST http://localhost:8094/orchestrate/natural \
  -H "Content-Type: application/json" \
  -d '{"request":"What is the weather in Tokyo?","use_ai":true}'

# 2. Check Jaeger for AI operations
curl -s "http://localhost:16686/api/operations?service=my-service" | \
  jq '.data[] | select(.name | contains("ai."))'

# Expected output:
# { "name": "ai.generate_response", "spanKind": "internal" }
# { "name": "ai.http_attempt", "spanKind": "internal" }
```

### Example Implementations

See these examples for correct implementation:

| Example | File | Key Lines |
|---------|------|-----------|
| `agent-with-orchestration` | `main.go` | Lines 64-85 |
| `agent-with-telemetry` | `main.go` | Lines 65-86 |

### Common Mistakes

| Mistake | Symptom | Fix |
|---------|---------|-----|
| Telemetry initialized after agent creation | No `ai.generate_response` spans in Jaeger | Move `telemetry.Initialize()` before agent constructor |
| Missing `ai.WithTelemetry()` option | No AI spans despite telemetry being initialized | Add `ai.WithTelemetry(telemetry.GetTelemetryProvider())` to `ai.NewClient()` |
| Component type not set before telemetry | service_type not auto-inferred | Add `core.SetCurrentComponentType()` before `telemetry.Initialize()` |

### What You Get

When correctly configured, AI operations create the following span hierarchy in Jaeger:

```
my-service
└── orchestrator.process_request (8000ms)
    ├── ai.generate_response (3500ms)           ← AI planning
    │   └── ai.http_attempt (3400ms)            ← HTTP call to OpenAI
    ├── HTTP POST geocoding-tool (1500ms)       ← Tool call
    ├── HTTP POST weather-tool (600ms)          ← Tool call
    └── ai.generate_response (2000ms)           ← AI synthesis
        └── ai.http_attempt (1900ms)            ← HTTP call to OpenAI
```

Each AI span includes attributes:
- `ai.provider` - Provider name (openai, anthropic, gemini, bedrock)
- `ai.model` - Model identifier
- `ai.prompt_tokens` - Tokens consumed by prompt
- `ai.completion_tokens` - Tokens in response
- `ai.total_tokens` - Total token usage

---

## CRITICAL BUG: AI Module Logger Not Propagated from Framework

**Status:** ✅ IMPLEMENTED
**Priority:** P0 - Critical
**Affects:** All AI module logging (INFO, DEBUG, ERROR levels)
**Symptom:** AI module logs are completely silent even when orchestration module logs work correctly

### Root Cause Analysis

The AI client is created **before** the Framework sets the real logger on the agent. When `core.NewBaseAgent()` is called, it initializes `Logger` to `NoOpLogger`. The AI client captures this `NoOpLogger` and stores its own copy. Later, when `core.NewFramework()` calls `applyConfigToComponent()`, it updates `agent.Logger` to the real logger, but the AI client's internal logger reference is never updated.

### Timeline of Events (Before Fix)

*This shows the original bug behavior - preserved for understanding the root cause:*

```
Step 1: NewTravelResearchAgent() called
        │
        ├── core.NewBaseAgent("travel-research-orchestration")
        │   └── agent.Logger = &NoOpLogger{}  ← INITIALIZED TO NoOpLogger
        │
        ├── ai.NewClient(ai.WithLogger(agent.Logger))
        │   └── AI client stores NoOpLogger internally  ← BUG: CAPTURES NoOpLogger
        │
        └── return agent

Step 2: core.NewFramework(agent, core.WithLogger(logger))
        │
        └── applyConfigToComponent(agent, config)
            └── base.Logger = createComponentLogger(...)  ← UPDATES agent.Logger
                                                            BUT AI CLIENT STILL HAS NoOpLogger!

Step 3: agent.InitializeOrchestrator(discovery)  ← CALLED AFTER FRAMEWORK
        │
        └── orchestration.CreateOrchestrator(config, deps)
            └── deps.Logger = t.Logger  ← NOW HAS REAL LOGGER (from Step 2)
                Orchestrator gets real logger ✅
```

### Why Orchestration Logs Work But AI Logs Don't

| Component | When Created | Logger at Creation | Current Logger |
|-----------|--------------|-------------------|----------------|
| AI Client | Step 1 (before Framework) | `NoOpLogger` ❌ | `NoOpLogger` ❌ |
| Orchestrator | Step 3 (after Framework) | Real Logger ✅ | Real Logger ✅ |

The orchestrator is created in `InitializeOrchestrator()` which is called **after** `NewFramework()`. By that point, `agent.Logger` has been updated to the real logger. The AI client, however, was created in Step 1 and permanently stores `NoOpLogger`.

### Code Evidence

**1. NoOpLogger assigned at creation** - [core/agent.go:141](../core/agent.go#L141):
```go
return &BaseAgent{
    ...
    Logger: &NoOpLogger{},  // Will be initialized based on config
    ...
}
```

**2. AI Client created with NoOpLogger** - [examples/agent-with-orchestration/research_agent.go:103-106](../examples/agent-with-orchestration/research_agent.go#L103):
```go
aiClient, err := ai.NewClient(
    ai.WithTelemetry(telemetry.GetTelemetryProvider()),
    ai.WithLogger(agent.Logger),  // agent.Logger is STILL NoOpLogger here!
)
```

**3. AI Client stores its own copy** - [ai/providers/base.go:45](providers/base.go#L45):
```go
return &BaseClient{
    ...
    Logger: logger,  // This is NoOpLogger and NEVER gets updated
    ...
}
```

**4. Framework updates agent.Logger LATER** - [core/agent.go:964](../core/agent.go#L964):
```go
base.Logger = createComponentLogger(config.logger, "agent/"+base.ID)
// But AI client already has NoOpLogger stored!
```

---

## Implementation Plan: Framework-Driven Logger Propagation

> **Status: ✅ IMPLEMENTED** - All steps below have been completed. Code is now in production.

### Design Philosophy

Following the "logging and telemetry should just work" principle, the Framework should automatically propagate the logger to all sub-components including the AI client. Developers should not need to worry about initialization order.

### Implementation Steps (All Completed ✅)

#### Step 1: Implement `SetLogger` in BaseClient ✅

**File:** `ai/providers/base.go`
**Location:** After line 57 (after `SetTelemetry` method)
**Change:** Add `SetLogger` method

```go
// SetLogger updates the logger after client creation.
// This is called by Framework.applyConfigToComponent() to propagate
// the real logger to the AI client after framework initialization.
//
// This follows the same pattern as SetTelemetry and orchestrator.SetLogger.
func (b *BaseClient) SetLogger(logger core.Logger) {
    if logger == nil {
        b.Logger = &core.NoOpLogger{}
    } else if cal, ok := logger.(core.ComponentAwareLogger); ok {
        b.Logger = cal.WithComponent("framework/ai")
    } else {
        b.Logger = logger
    }
}
```

#### Step 2: Framework Propagates Logger to AI Client ✅

**File:** `core/agent.go`
**Function:** `applyConfigToComponent()` (starts at line 946)
**Change:** Insert AI logger propagation in 4 locations, each BEFORE the `return` statement

**Location 1: Direct *BaseAgent case** (insert between lines 964-965, before `return`)

Current code:
```go
// line 964
base.Logger = createComponentLogger(config.logger, "agent/"+base.ID)
// line 965
return
```

Change to:
```go
base.Logger = createComponentLogger(config.logger, "agent/"+base.ID)
// NEW: Propagate logger to AI client if it exists
if base.AI != nil {
    if loggable, ok := base.AI.(interface{ SetLogger(Logger) }); ok {
        loggable.SetLogger(base.Logger)
    }
}
return
```

**Location 2: Direct *BaseTool case** (insert between lines 980-981, before `return`)

Current code:
```go
// line 980
base.Logger = createComponentLogger(config.logger, "tool/"+base.ID)
// line 981
return
```

Change to:
```go
base.Logger = createComponentLogger(config.logger, "tool/"+base.ID)
// NEW: Propagate logger to AI client if it exists
if base.AI != nil {
    if loggable, ok := base.AI.(interface{ SetLogger(Logger) }); ok {
        loggable.SetLogger(base.Logger)
    }
}
return
```

**Location 3: Embedded *BaseAgent case** (insert between lines 1015-1016, before `return`)

Current code:
```go
// line 1015
base.Logger = createComponentLogger(config.logger, "agent/"+base.ID)
// line 1016
return
```

Change to:
```go
base.Logger = createComponentLogger(config.logger, "agent/"+base.ID)
// NEW: Propagate logger to AI client if it exists
if base.AI != nil {
    if loggable, ok := base.AI.(interface{ SetLogger(Logger) }); ok {
        loggable.SetLogger(base.Logger)
    }
}
return
```

**Location 4: Embedded *BaseTool case** (insert between lines 1035-1036, before `return`)

Current code:
```go
// line 1035
base.Logger = createComponentLogger(config.logger, "tool/"+base.ID)
// line 1036
return
```

Change to:
```go
base.Logger = createComponentLogger(config.logger, "tool/"+base.ID)
// NEW: Propagate logger to AI client if it exists
if base.AI != nil {
    if loggable, ok := base.AI.(interface{ SetLogger(Logger) }); ok {
        loggable.SetLogger(base.Logger)
    }
}
return
```

### Design Notes

**Why inline interface check instead of importing from ai module?**

Using `interface{ SetLogger(Logger) }` avoids a circular dependency:
- `ai` imports `core` (for Logger, AIClient interfaces)
- `core` cannot import `ai` (would create circular dependency)

The inline interface check allows `core/agent.go` to call SetLogger without importing the ai module.

**Why SetLogger wraps with "framework/ai"?**

The AI module should log with component `"framework/ai"` for filtering, not inherit the agent's component name. SetLogger applies this wrapping to ensure consistent log filtering regardless of which component created the AI client.

### Files to Modify Summary

| File | Change | Lines |
|------|--------|-------|
| `ai/providers/base.go` | Add `SetLogger()` method | After line 57 |
| `core/agent.go` | Propagate logger to `base.AI` (4 locations) | Between lines 964-965, 980-981, 1015-1016, 1035-1036 |

### Test Plan

#### Unit Tests

**File:** `ai/providers/base_test.go`
**Add test:**
```go
func TestBaseClient_SetLogger(t *testing.T) {
    // Test 1: SetLogger with nil
    client := NewBaseClient(time.Second, nil)
    client.SetLogger(nil)
    assert.IsType(t, &core.NoOpLogger{}, client.Logger)

    // Test 2: SetLogger with regular logger
    mockLogger := &MockLogger{}
    client.SetLogger(mockLogger)
    assert.Equal(t, mockLogger, client.Logger)

    // Test 3: SetLogger with ComponentAwareLogger
    calLogger := &MockComponentAwareLogger{}
    client.SetLogger(calLogger)
    // Verify WithComponent was called with "framework/ai"
}
```

**File:** `core/agent_test.go`
**Add test:**
```go
func TestFramework_PropagatesLoggerToAIClient(t *testing.T) {
    // Create agent with AI client before framework
    agent := NewBaseAgent("test-agent")
    mockAI := &MockAIClient{}
    agent.AI = mockAI

    // Create framework with logger
    testLogger := &TestLogger{}
    framework, err := NewFramework(agent, WithLogger(testLogger))
    require.NoError(t, err)

    // Verify AI client received the logger
    assert.True(t, mockAI.SetLoggerCalled)
    assert.NotNil(t, mockAI.ReceivedLogger)
}
```

#### Integration Test

**File:** `examples/agent-with-orchestration/integration_test.go` (new file)
```go
func TestAILogsAppearWithTraceID(t *testing.T) {
    // 1. Initialize telemetry
    // 2. Create agent (AI client created with NoOpLogger)
    // 3. Create framework (should propagate real logger to AI)
    // 4. Make AI request
    // 5. Verify logs contain trace_id and component="framework/ai"
}
```

### Verification Commands

After implementation:

```bash
# 1. Build and run
go build ./examples/agent-with-orchestration/...
./agent-with-orchestration 2>&1 | head -50

# 2. Verify AI logs appear with component field
./agent-with-orchestration 2>&1 | grep '"component":"framework/ai"'

# 3. Verify AI logs have trace correlation
curl -X POST http://localhost:8094/orchestrate/natural \
  -H "Content-Type: application/json" \
  -d '{"request":"What is the weather in Tokyo?","use_ai":true}'

# Check logs for trace_id in AI operations
kubectl logs -l app=travel-research-agent | \
  jq 'select(.component == "framework/ai" and .["trace.trace_id"] != null)'

# 4. Verify spans still appear in Jaeger (unchanged)
curl -s "http://localhost:16686/api/operations?service=travel-research-orchestration" | \
  jq '.data[] | select(.name | contains("ai."))'
```

### Expected Outcome

After implementation:

| Before | After |
|--------|-------|
| AI module logs completely silent | AI logs appear with `component: "framework/ai"` |
| No trace correlation in AI logs | Logs include `trace.trace_id` for correlation |
| Developers must know initialization order | Framework handles propagation automatically |
| Orchestration logs work, AI logs don't | Both work consistently |

### Example Log Output (After Fix)

```json
{
  "level": "info",
  "ts": "2024-12-14T10:30:00.000Z",
  "msg": "AI request initiated",
  "component": "framework/ai",
  "operation": "ai_request",
  "provider": "openai",
  "model": "gpt-4.1-mini",
  "prompt_length": 2500,
  "trace.trace_id": "abc123def456",
  "trace.span_id": "789012345678"
}

{
  "level": "debug",
  "ts": "2024-12-14T10:30:01.500Z",
  "msg": "AI HTTP request completed",
  "component": "framework/ai",
  "operation": "ai_http_success",
  "status_code": 200,
  "duration_ms": 1450,
  "trace.trace_id": "abc123def456",
  "trace.span_id": "789012345678"
}
```

### Backward Compatibility

This change is fully backward compatible:

1. **Existing code without AI client** - No change in behavior
2. **AI client created after Framework** - SetLogger called but logger is same
3. **AI client created before Framework** - SetLogger propagates real logger (the fix)
4. **AI client without SetLogger method** - Interface check fails silently, existing behavior preserved
