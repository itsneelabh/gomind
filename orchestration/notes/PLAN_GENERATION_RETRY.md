# Plan Generation JSON Parse Error Handling

## Overview

This document describes handling JSON parse errors during LLM plan generation. **This is not a standalone solution** - it extends the existing LLM response handling infrastructure already present in the orchestration module.

**Existing Infrastructure (Already Handles):**
- Markdown formatting (`**bold**`, `*italic*`) → [INTELLIGENT_PARAMETER_BINDING.md § Markdown Stripping](../INTELLIGENT_PARAMETER_BINDING.md#markdown-stripping-from-llm-responses)
- Code block extraction (` ```json ... ``` `) → `cleanLLMResponse()` in [orchestrator.go:1042-1069](../orchestrator.go#L1042-L1069)
- JSON boundary detection → `cleanLLMResponse()` finds `{` and `}`

**This Document Adds:**
- Arithmetic expression prevention (prompt-level) - **IMPLEMENTED**
- Retry mechanism with error feedback - **IMPLEMENTED**

LLM responses are probabilistic - the same query may produce valid JSON one time and invalid JSON the next.

---

## Problem Statement

During plan generation, the LLM may produce invalid JSON due to:

1. **Arithmetic expressions**: `"amount": 100 * "{{step-1.response.price}}"` — ❌ **Cannot be cleaned** (this doc addresses)
2. **Markdown formatting**: `"city": "**Paris**"` or `"tool": "*weather*"` — ✅ **Already handled** by `stripMarkdownFromJSON()`
3. **Code block wrapping**: ` ```json { ... } ``` ` — ✅ **Already handled** by `cleanLLMResponse()`
4. **Malformed templates**: Missing quotes, trailing commas, etc. — ❌ **Cannot be cleaned**

These errors result in parse failures:

```json
{
    "error": "Orchestration failed",
    "details": "failed to generate execution plan: failed to parse plan JSON: invalid character '*' after object key:value pair",
    "status": 500
}
```

---

## Solution Architecture

The orchestration module uses a **multi-layer defense** strategy for LLM response handling, with an **error feedback retry loop** for unrecoverable parse failures.

### Complete Flow: From User Request to Execution Plan

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                         PLAN GENERATION FLOW                                      │
│                                                                                   │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │  USER REQUEST                                                                │ │
│  │  "What's the weather in Tokyo and convert 100 USD to JPY"                   │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│       │                                                                           │
│       ▼                                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │  STEP 1: BUILD INITIAL PROMPT                                                │ │
│  │  buildPlanningPrompt(ctx, request)                                           │ │
│  │    • Fetch available capabilities from discovery                             │ │
│  │    • Include JSON format instructions (no markdown, no arithmetic)           │ │
│  │    • Include capability schemas for tool parameters                          │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│       │                                                                           │
│       ▼                                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │  STEP 2: CALL LLM (Attempt 1)                                                │ │
│  │  aiClient.GenerateResponse(ctx, prompt, {Temperature: 0.3})                  │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│       │                                                                           │
│       ▼                                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │  STEP 3: CLEANUP LLM RESPONSE                                                │ │
│  │    • cleanLLMResponse() - extract JSON from ```json ... ``` blocks           │ │
│  │    • stripMarkdownFromJSON() - remove **bold** and *italic* markers          │ │
│  │    • Find JSON boundaries { ... }                                            │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│       │                                                                           │
│       ▼                                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │  STEP 4: PARSE JSON                                                          │ │
│  │  json.Unmarshal(cleaned, &RoutingPlan{})                                     │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│       │                                                                           │
│       ├───────────────────── SUCCESS ──────────────────────────────────────────► │
│       │                      Return *RoutingPlan                         EXIT    │
│       │                                                                           │
│       ▼ PARSE ERROR (e.g., "invalid character '*' after object key:value pair")  │
│                                                                                   │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │  STEP 5: CHECK RETRY ELIGIBILITY                                             │ │
│  │    • Is PlanParseRetryEnabled? (default: true)                               │ │
│  │    • Is attempt < maxAttempts? (default maxAttempts = 3)                     │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│       │                                                                           │
│       ├───────────────────── NO (retry disabled or exhausted) ─────────────────► │
│       │                      Return parse error                          EXIT    │
│       │                                                                           │
│       ▼ YES (retries remaining)                                                   │
│                                                                                   │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │  STEP 6: BUILD ERROR FEEDBACK PROMPT                                         │ │
│  │  buildPlanningPromptWithParseError(ctx, request, parseError)                 │ │
│  │                                                                               │ │
│  │  ┌─────────────────────────────────────────────────────────────────────────┐ │ │
│  │  │  IMPORTANT: Your previous response could not be parsed as valid JSON.   │ │ │
│  │  │                                                                          │ │ │
│  │  │  Parse Error: invalid character '*' after object key:value pair         │ │ │
│  │  │                                                                          │ │ │
│  │  │  Common JSON mistakes to avoid:                                         │ │ │
│  │  │  - NO arithmetic expressions: "amount": 100 * price is INVALID          │ │ │
│  │  │  - NO markdown formatting: **bold** and *italic* are INVALID            │ │ │
│  │  │  - NO trailing commas: {"a": 1,} is INVALID                             │ │ │
│  │  │  - NO comments: // comments are INVALID in JSON                         │ │ │
│  │  │                                                                          │ │ │
│  │  │  Please regenerate a VALID JSON execution plan.                         │ │ │
│  │  └─────────────────────────────────────────────────────────────────────────┘ │ │
│  │                                                                               │ │
│  │  + Original prompt with capabilities                                         │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│       │                                                                           │
│       ▼                                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │  STEP 7: CALL LLM (Attempt 2)                                                │ │
│  │  aiClient.GenerateResponse(ctx, errorFeedbackPrompt, {Temperature: 0.3})     │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│       │                                                                           │
│       ▼                                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │  STEP 8: CLEANUP & PARSE (same as Steps 3-4)                                 │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│       │                                                                           │
│       ├───────────────────── SUCCESS ──────────────────────────────────────────► │
│       │                      Return *RoutingPlan                         EXIT    │
│       │                                                                           │
│       ▼ PARSE ERROR (still failing)                                               │
│                                                                                   │
│       │ Loop back to STEP 5 (Check Retry Eligibility)                            │
│       │ Continue until success or maxAttempts (3) exhausted                      │
│       ▼                                                                           │
│                                                                                   │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │  FINAL: ALL RETRIES EXHAUSTED                                                │ │
│  │  Return last parse error to caller                                           │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                   │
└──────────────────────────────────────────────────────────────────────────────────┘
```

### Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `GOMIND_PLAN_RETRY_ENABLED` | `true` | Enable retry on JSON parse failures |
| `GOMIND_PLAN_RETRY_MAX` | `2` | Maximum retry attempts (total attempts = 1 initial + 2 retries = 3) |

### Layer Summary

| Layer | Component | When | What It Does |
|-------|-----------|------|--------------|
| **Layer 1** | Prompt Instructions | Before LLM call | Tells LLM: no markdown, no code blocks, no arithmetic |
| **Layer 2** | `cleanLLMResponse()` | After LLM response | Extracts JSON from code fences, finds `{...}` boundaries |
| **Layer 2** | `stripMarkdownFromJSON()` | After cleanup | Removes `**bold**` and `*italic*` markers from strings |
| **Layer 3** | `json.Unmarshal()` | After cleanup | Parses JSON string into `*RoutingPlan` struct |
| **Layer 4** | Retry Loop | On parse failure | Sends error details back to LLM for correction |

**Key Insight:** Layers 1-3 handle predictable issues (markdown, code blocks) programmatically. Layer 4 (retry) handles unpredictable issues (arithmetic expressions, syntax errors) by giving the LLM explicit error feedback and a second chance to generate valid JSON.

---

## What Can Be Cleaned vs What Cannot

| Issue | Can Be Cleaned? | Solution | Status |
|-------|-----------------|----------|--------|
| `**bold**` in strings | ✅ Yes | `stripMarkdownFromJSON()` removes `**` | **Existing** |
| `*italic*` in strings | ✅ Yes | `stripMarkdownFromJSON()` removes `*` | **Existing** |
| ` ```json ... ``` ` wrapper | ✅ Yes | `cleanLLMResponse()` extracts JSON | **Existing** |
| Intro text before `{` | ✅ Yes | `cleanLLMResponse()` finds `{` | **Existing** |
| `"amount": 100 * price` | ❌ No | Prompt-level prevention + retry | **Implemented** |
| Trailing commas | ❌ No | Retry with error feedback | **Implemented** |
| Missing quotes | ❌ No | Retry with error feedback | **Implemented** |

**Key Insight:** The first four issues (markdown-related) are **already handled** by the existing cleanup infrastructure in `orchestrator.go`. Arithmetic expressions like `100 * price` cannot be fixed programmatically because they represent **computation**, not malformed text. The `*` is not markdown - it's a multiplication operator that JSON doesn't support.

---

## Implemented: Arithmetic Expression Prevention

Added to the prompt instructions to prevent arithmetic expressions before they occur.

**Location:** [default_prompt_builder.go:273-274](../default_prompt_builder.go#L273-L274)

```go
CRITICAL FORMAT RULES (applies to all LLM providers):
- You are a JSON API. Your ONLY output is a raw JSON object.
- Output ONLY valid JSON - no markdown, no code blocks, no backticks
- Do NOT use any text formatting: no ** (bold), no * (italic), no __ (underline)
- Do NOT wrap JSON in code fences (no triple backticks)
- Do NOT include any explanatory text before or after the JSON
- String values must be plain text without any markdown formatting
- Do NOT use arithmetic expressions in JSON values (e.g., "amount": 100 * price is INVALID)  ← NEW
- If calculations are needed, create separate steps or use literal values only                 ← NEW
- Start your response with { and end with } - nothing else
```

**Why prompt-level prevention:**
- Arithmetic expressions cannot be cleaned up post-hoc
- JSON fundamentally doesn't support expressions
- LLMs understand "don't do X" better when given explicit examples

---

## Existing Cleanup Infrastructure (Production-Ready)

The orchestration module already includes robust LLM response cleaning. **This infrastructure is fully documented in [INTELLIGENT_PARAMETER_BINDING.md § Markdown Stripping from LLM Responses](../INTELLIGENT_PARAMETER_BINDING.md#markdown-stripping-from-llm-responses)** (lines 554-777).

### cleanLLMResponse() - [orchestrator.go:1042-1069](../orchestrator.go#L1042-L1069)

Handles structural cleanup (already implemented):
- Extracts JSON from markdown code blocks (` ```json ... ``` `)
- Finds JSON object boundaries (`{` ... `}`)
- Strips intro/outro text ("Here's your plan:" etc.)

### stripMarkdownFromJSON() - [orchestrator.go:1076-1113](../orchestrator.go#L1076-L1113)

Handles formatting cleanup (already implemented):
- Strips bold markers: `**text**` → `text`
- Strips italic markers: `*text*` → `text`
- Conservative matching to avoid breaking valid JSON

**Note:** The markdown stripping algorithm is detailed in INTELLIGENT_PARAMETER_BINDING.md, including regex patterns, safety guarantees, and debug logging instructions.

### LLM Provider Behaviors

| Provider | JSON Mode | Markdown Tendency | Arithmetic Tendency |
|----------|-----------|-------------------|---------------------|
| **OpenAI GPT-4** | `response_format: json_object` | Low | Low |
| **Anthropic Claude** | No native JSON mode | Low | Medium |
| **Google Gemini** | `responseMimeType: application/json` | High | Medium |
| **Groq** | Follows OpenAI format | Medium | Low |

---

## Implemented: Retry Mechanism

For cases where prevention fails, a retry mechanism with error feedback provides additional resilience.

### Design

The `generateExecutionPlan()` function retries on JSON parse failures, following the existing `regeneratePlan()` pattern used for validation errors.

### Configuration

| Config Field | Default | Environment Variable | Description |
|--------------|---------|---------------------|-------------|
| `PlanParseRetryEnabled` | `true` | `GOMIND_PLAN_RETRY_ENABLED` | Enable retry on JSON parse failures |
| `PlanParseMaxRetries` | `2` | `GOMIND_PLAN_RETRY_MAX` | Maximum retry attempts after initial failure |

---

## Implementation Details

### 1. Configuration Fields ([interfaces.go](../interfaces.go))

Added to `OrchestratorConfig` struct:

```go
// Plan Parse Retry configuration
// When enabled, retries LLM plan generation if JSON parsing fails.
// This handles cases where the LLM produces invalid JSON (arithmetic expressions,
// malformed syntax) that cannot be fixed by the cleanup functions.
PlanParseRetryEnabled bool `json:"plan_parse_retry_enabled"`
PlanParseMaxRetries   int  `json:"plan_parse_max_retries"` // Default: 2
```

### 2. Default Configuration ([interfaces.go](../interfaces.go))

Updated `DefaultConfig()` with defaults and environment variable loading:

```go
// In DefaultConfig() struct initialization:
// Plan Parse Retry defaults
PlanParseRetryEnabled: true, // Enable by default for production reliability
PlanParseMaxRetries:   2,    // Up to 2 retry attempts after initial failure

// Environment variable loading:
// Plan Parse Retry configuration from environment
if retryEnabled := os.Getenv("GOMIND_PLAN_RETRY_ENABLED"); retryEnabled != "" {
    config.PlanParseRetryEnabled = strings.ToLower(retryEnabled) == "true"
}
if maxRetries := os.Getenv("GOMIND_PLAN_RETRY_MAX"); maxRetries != "" {
    if val, err := strconv.Atoi(maxRetries); err == nil && val >= 0 {
        config.PlanParseMaxRetries = val
    }
}
```

### 3. Functional Option ([factory.go](../factory.go))

Added `WithPlanParseRetry()` functional option:

```go
// WithPlanParseRetry creates an option for configuring plan parse retry behavior.
// When enabled, the orchestrator will retry LLM plan generation if JSON parsing fails
// due to invalid syntax (e.g., arithmetic expressions, malformed JSON).
//
// Parameters:
//   - enabled: whether to retry on JSON parse failures
//   - maxRetries: maximum number of retry attempts (0 = no retries, default: 2)
func WithPlanParseRetry(enabled bool, maxRetries int) OrchestratorOption {
    return func(c *OrchestratorConfig) {
        c.PlanParseRetryEnabled = enabled
        if maxRetries >= 0 {
            c.PlanParseMaxRetries = maxRetries
        }
    }
}
```

### 4. Retry Loop in generateExecutionPlan() ([orchestrator.go:522-731](../orchestrator.go#L522-L731))

The core retry logic:

```go
func (o *AIOrchestrator) generateExecutionPlan(ctx context.Context, request string, requestID string) (*RoutingPlan, error) {
    planGenStart := time.Now()

    // Build initial prompt with capability information
    prompt, err := o.buildPlanningPrompt(ctx, request)
    if err != nil {
        return nil, err
    }

    // Determine max attempts: 1 initial + retries (if enabled)
    maxAttempts := 1
    if o.config != nil && o.config.PlanParseRetryEnabled {
        maxAttempts = 1 + o.config.PlanParseMaxRetries
    }

    var lastParseErr error
    var totalTokensUsed int

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        // Telemetry: Record LLM prompt for visibility in Jaeger
        telemetry.AddSpanEvent(ctx, "llm.plan_generation.request",
            attribute.String("request_id", requestID),
            attribute.String("prompt", truncateString(prompt, 2000)),
            attribute.Int("attempt", attempt),
        )

        // Call LLM
        aiResponse, err := o.aiClient.GenerateResponse(ctx, prompt, &core.AIOptions{
            Temperature:  0.3,
            MaxTokens:    2000,
            SystemPrompt: "You are an intelligent orchestrator that creates execution plans for multi-agent systems.",
        })
        if err != nil {
            return nil, err
        }

        totalTokensUsed += aiResponse.Usage.TotalTokens

        // Parse the LLM response into a plan
        plan, parseErr := o.parsePlan(aiResponse.Content)
        if parseErr == nil {
            // Success!
            return plan, nil
        }

        // Parse failed - check if we should retry
        lastParseErr = parseErr

        // Telemetry: Record parse failure
        telemetry.AddSpanEvent(ctx, "llm.plan_generation.parse_error",
            attribute.String("request_id", requestID),
            attribute.String("error", parseErr.Error()),
            attribute.Int("attempt", attempt),
            attribute.Bool("will_retry", attempt < maxAttempts),
        )

        // If we have retries left, build a new prompt with error feedback
        if attempt < maxAttempts {
            prompt, err = o.buildPlanningPromptWithParseError(ctx, request, parseErr)
            if err != nil {
                return nil, lastParseErr
            }
        }
    }

    // All attempts exhausted
    return nil, lastParseErr
}
```

### 5. Error Feedback Prompt ([orchestrator.go:849-878](../orchestrator.go#L849-L878))

The `buildPlanningPromptWithParseError()` function provides error context to the LLM:

```go
// buildPlanningPromptWithParseError constructs a retry prompt that includes the parse error
// context to help the LLM generate valid JSON on subsequent attempts.
func (o *AIOrchestrator) buildPlanningPromptWithParseError(ctx context.Context, request string, parseErr error) (string, error) {
    // Get the base prompt first
    basePrompt, err := o.buildPlanningPrompt(ctx, request)
    if err != nil {
        return "", err
    }

    // Construct error feedback section
    errorFeedback := fmt.Sprintf(`
IMPORTANT: Your previous response could not be parsed as valid JSON.

Parse Error: %s

Common JSON mistakes to avoid:
- NO arithmetic expressions: "amount": 100 * price is INVALID JSON
- NO markdown formatting: **bold** and *italic* are INVALID in JSON strings
- NO code blocks: Do not wrap JSON in triple backticks
- NO trailing commas: {"a": 1,} is INVALID (remove trailing comma)
- NO comments: // comments are INVALID in JSON
- ALL string values must be in double quotes
- ALL keys must be in double quotes

Please regenerate a VALID JSON execution plan. Start with { and end with }.`,
        parseErr.Error())

    // Insert error feedback before the base prompt's final instruction
    return errorFeedback + "\n\n" + basePrompt, nil
}
```

### 6. Telemetry (Span Events + Unified Metrics)

The plan generation retry mechanism implements the **dual telemetry approach** per [telemetry/ARCHITECTURE.md](../../telemetry/ARCHITECTURE.md): span events for Jaeger trace visibility AND metrics for Prometheus dashboards.

#### Span Events (Jaeger Trace Visibility)

All span events use the parent context (`ctx`), ensuring they appear within the same trace in Jaeger:

| Event | Attributes | Description |
|-------|------------|-------------|
| `llm.plan_generation.request` | request_id, prompt, prompt_length, temperature, max_tokens, attempt | Recorded before each LLM call |
| `llm.plan_generation.response` | request_id, response, response_length, prompt_tokens, completion_tokens, total_tokens, duration_ms, attempt | Recorded after successful LLM response |
| `llm.plan_generation.parse_error` | request_id, error, attempt, will_retry | Recorded when JSON parsing fails |
| `llm.plan_generation.error` | request_id, error, duration_ms, attempt | Recorded on LLM call failure |

#### OpenTelemetry Metrics

The following metrics are recorded for plan generation. Cross-module metrics use the [Unified Metrics API](../../telemetry/unified_metrics.go), while orchestration-specific metrics use direct `telemetry.Counter()` and `telemetry.Histogram()` calls.

**Cross-Module Metrics (via Unified API):**

| Metric | Type | Attributes | Description |
|--------|------|------------|-------------|
| `ai.request.duration_ms` | Histogram | module, provider, status | Per-LLM-call duration |
| `ai.request.total` | Counter | module, provider, status | LLM call count (success/error) |
| `ai.tokens.used` | Counter | module, provider, type | Token usage per call (input/output) |

**Orchestration-Local Metrics (via direct calls):**

| Metric | Type | Attributes | Description |
|--------|------|------------|-------------|
| `plan_generation.duration_ms` | Histogram | module, status | Total plan generation duration (including retries) |
| `plan_generation.total` | Counter | module, status | Plan generation count (success/error) |
| `plan_generation.parse_errors` | Counter | module, will_retry | Parse error count |
| `plan_generation.retries` | Counter | module | Retry attempt count |

**Attribute Values:**
- `module`: `"orchestration"`
- `provider`: `"plan_generation"` (for AI metrics)
- `status`: `"success"` or `"error"`
- `type`: `"input"` or `"output"` (for token metrics)
- `will_retry`: `"true"` or `"false"`

#### Metric Recording Locations

| Metric Call | Location in Code | When Called |
|-------------|------------------|-------------|
| `telemetry.RecordAIRequest()` | After `GenerateResponse()` | Every LLM call (success or error) |
| `telemetry.RecordAITokens()` | After successful LLM response | For both input and output tokens |
| `telemetry.Histogram("plan_generation.duration_ms", ...)` | On success or final failure | Once per plan generation request |
| `telemetry.Counter("plan_generation.total", ...)` | On success or final failure | Once per plan generation request |
| `telemetry.Counter("plan_generation.parse_errors", ...)` | On parse failure | Each time JSON parsing fails |
| `telemetry.Counter("plan_generation.retries", ...)` | Before retry attempt | Each time a retry is about to be made |

**Architecture Note:** Orchestration-specific metrics use direct `telemetry.Counter()` and `telemetry.Histogram()` calls rather than helper functions in `unified_metrics.go`. This keeps the telemetry module focused on cross-module contracts while allowing orchestration to define its own metrics.

---

## Implementation Status

| Component | Status | Location |
|-----------|--------|----------|
| Prompt: markdown prevention | **Existing** | [default_prompt_builder.go:266-272](../default_prompt_builder.go#L266-L272) |
| Prompt: arithmetic prevention | **Implemented** | [default_prompt_builder.go:273-274](../default_prompt_builder.go#L273-L274) |
| Cleanup: code block extraction | **Existing** | [orchestrator.go:1042-1069](../orchestrator.go#L1042-L1069) |
| Cleanup: markdown stripping | **Existing** | [orchestrator.go:1076-1113](../orchestrator.go#L1076-L1113) |
| Retry mechanism | **Implemented** | [orchestrator.go](../orchestrator.go), [interfaces.go](../interfaces.go) |
| Span events (Jaeger) | **Implemented** | [orchestrator.go](../orchestrator.go) - `telemetry.AddSpanEvent()` calls |
| Cross-module metrics (AI) | **Implemented** | [orchestrator.go](../orchestrator.go) - `RecordAIRequest()`, `RecordAITokens()` |
| Orchestration-local metrics | **Implemented** | [orchestrator.go](../orchestrator.go) - direct `Counter()`, `Histogram()` calls |

---

## Implementation Checklist

- [x] Add `PlanParseRetryEnabled` and `PlanParseMaxRetries` to `OrchestratorConfig` - [interfaces.go](../interfaces.go)
- [x] Update `DefaultConfig()` with env var loading - [interfaces.go](../interfaces.go)
- [x] Add `WithPlanParseRetry()` functional option - [factory.go](../factory.go)
- [x] Modify `generateExecutionPlan()` with retry loop - [orchestrator.go](../orchestrator.go)
- [x] Add `buildPlanningPromptWithParseError()` function - [orchestrator.go](../orchestrator.go)
- [x] Add span events for Jaeger trace visibility - [orchestrator.go](../orchestrator.go)
- [x] Add cross-module metrics (RecordAIRequest, RecordAITokens) - [orchestrator.go](../orchestrator.go)
- [x] Add orchestration-local metrics (Counter/Histogram for plan_generation.*) - [orchestrator.go](../orchestrator.go)
- [x] Write unit tests - [factory_test.go](../factory_test.go), [orchestrator_test.go](../orchestrator_test.go)
- [x] Update ENVIRONMENT_VARIABLES_GUIDE.md - [ENVIRONMENT_VARIABLES_GUIDE.md](../../docs/ENVIRONMENT_VARIABLES_GUIDE.md)

---

## Unit Tests

### Configuration Tests ([factory_test.go](../factory_test.go))

```go
func TestDefaultConfig_PlanParseRetry_EnvironmentConfiguration(t *testing.T) {
    tests := []struct {
        name           string
        envVars        map[string]string
        expectedEnabled bool
        expectedMax    int
    }{
        {
            name:            "disable_retry_via_env",
            envVars:         map[string]string{"GOMIND_PLAN_RETRY_ENABLED": "false"},
            expectedEnabled: false,
            expectedMax:     2,
        },
        {
            name:            "set_max_retries_via_env",
            envVars:         map[string]string{"GOMIND_PLAN_RETRY_MAX": "5"},
            expectedEnabled: true,
            expectedMax:     5,
        },
        // ... more test cases
    }
    // Test implementation...
}

func TestWithPlanParseRetry(t *testing.T) {
    tests := []struct {
        name            string
        enabled         bool
        maxRetries      int
        expectedEnabled bool
        expectedMax     int
    }{
        {"enable_with_max_retries", true, 3, true, 3},
        {"disable_retry", false, 0, false, 0},
        {"negative_max_retries_ignored", true, -1, true, 2}, // Uses default
    }
    // Test implementation...
}
```

### Retry Logic Tests ([orchestrator_test.go](../orchestrator_test.go))

```go
func TestAIOrchestrator_GenerateExecutionPlan_RetryDisabled(t *testing.T) {
    // Verifies that when retry is disabled, parse errors fail immediately
}

func TestAIOrchestrator_GenerateExecutionPlan_RetrySuccess(t *testing.T) {
    // Verifies retry succeeds when LLM returns valid JSON on second attempt
}

func TestAIOrchestrator_GenerateExecutionPlan_AllRetriesExhausted(t *testing.T) {
    // Verifies proper error handling when all retries are exhausted
}

func TestAIOrchestrator_BuildPlanningPromptWithParseError(t *testing.T) {
    // Verifies error feedback prompt contains the parse error
}
```

---

## Usage Examples

### Programmatic Configuration

```go
// Using functional option
orchestrator, err := orchestration.CreateOrchestratorWithOptions(
    deps,
    orchestration.WithPlanParseRetry(true, 3), // Enable with 3 retries
)

// Using config directly
config := orchestration.DefaultConfig()
config.PlanParseRetryEnabled = true
config.PlanParseMaxRetries = 3
orchestrator, err := orchestration.CreateOrchestrator(config, deps)
```

### Environment Variable Configuration

```bash
# Disable retry (fail fast on parse errors)
export GOMIND_PLAN_RETRY_ENABLED=false

# Increase retry attempts (default: 2)
export GOMIND_PLAN_RETRY_MAX=3

# Disable by setting max retries to 0
export GOMIND_PLAN_RETRY_MAX=0
```

---

## Related Documentation

| Document | Relevance |
|----------|-----------|
| [INTELLIGENT_PARAMETER_BINDING.md](../INTELLIGENT_PARAMETER_BINDING.md) | **Primary reference** - Contains full details on markdown stripping (§ Markdown Stripping from LLM Responses, lines 554-777), LLM provider behaviors, and regex patterns |
| [README.md](../README.md) | Module overview and architecture |
| [ARCHITECTURE.md](../ARCHITECTURE.md) | Module design principles |
| [ENVIRONMENT_VARIABLES_GUIDE.md](../../docs/ENVIRONMENT_VARIABLES_GUIDE.md) | Configuration options |

**Architecture Note:** This document and INTELLIGENT_PARAMETER_BINDING.md together describe the complete LLM response handling strategy for the orchestration module. The cleanup functions (`cleanLLMResponse()`, `stripMarkdownFromJSON()`) are shared infrastructure used by both plan generation and parameter binding.
