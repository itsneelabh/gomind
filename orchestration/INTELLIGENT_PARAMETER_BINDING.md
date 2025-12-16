# Intelligent Parameter Binding

**A comprehensive guide to gomind's LLM-powered parameter resolution system**

**Status:** Production-Ready (All Layers Complete)
**Last Updated:** December 2025

---

## Overview

This document explains how gomind's orchestration module resolves parameters between workflow steps. When one step depends on another (e.g., weather tool needs coordinates from geocoding), the system automatically extracts and converts the required data.

**Key Design Principle:** The framework is **domain-agnostic**. It contains no hardcoded knowledge about weather, currency, coordinates, or any specific domain. All semantic understanding is delegated to the LLM.

---

## The Problem

In multi-step workflows, the LLM plans which tools to use and their dependencies, but it doesn't know the exact response structure of each tool. This leads to:

```
LLM generates:  {{step-1.response.lat}}
Actual data:    {"latitude": "48.85", "longitude": "2.35"}

Result: Parameter binding fails
```

**Three failure modes:**
1. **Path mismatch** - Template path doesn't match actual JSON structure
2. **Type mismatch** - String `"48.85"` instead of number `48.85`
3. **Semantic gap** - Template expects `country.currency` but data has `country: "France"`

---

## The Solution: LLM-First Hybrid Resolution

The orchestrator uses a **three-layer resolution strategy** where **LLM handles all semantic understanding**:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Parameter Resolution Flow                     │
│                                                                  │
│  Step 1 Completes → Output: {"lat": "48.85", "country": "FR"}   │
│       │                                                          │
│       ▼                                                          │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Layer 1: Auto-Wiring (instant, free)                       │ │
│  │   ONLY handles trivial cases:                              │ │
│  │   • Exact name match: lat → lat                            │ │
│  │   • Case-insensitive: LAT → lat                            │ │
│  │   • Nested extraction: {code: "EUR"} → "EUR"               │ │
│  │   • Type coercion: "48.85" → 48.85                         │ │
│  │                                                             │ │
│  │   NO semantic aliases (framework is domain-agnostic)       │ │
│  └────────────────────────────────────────────────────────────┘ │
│       │                                                          │
│       ▼ (if required parameters still missing)                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Layer 2: Micro-Resolution (LLM call)                       │ │
│  │   • Small focused LLM call with function calling           │ │
│  │   • Handles ALL semantic understanding:                    │ │
│  │     - "latitude" → "lat" (different names, same meaning)   │ │
│  │     - "France" → "EUR" (domain inference)                  │ │
│  │   • Guaranteed type safety via JSON schema                 │ │
│  └────────────────────────────────────────────────────────────┘ │
│       │                                                          │
│       ▼ (if templates still unresolved)                         │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Layer 3: LLM Error Analysis (smart retry decision)         │ │
│  │   • LLM analyzes error message and context                 │ │
│  │   • Decides if error is fixable with different parameters  │ │
│  │   • No "Retryable" flag needed from tool developers        │ │
│  │   • 503 errors: LLM analyzes to detect semantic errors     │ │
│  │   • Resilience module handles 408/429/500/502/504          │ │
│  └────────────────────────────────────────────────────────────┘ │
│       │                                                          │
│       ▼                                                          │
│  Step 2 Executes with correctly typed parameters                │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## What Changed (December 2025)

### Files Modified

| File | Module | Change | Status |
|------|--------|--------|--------|
| [auto_wire.go](./auto_wire.go) | `orchestration` | Emptied `SemanticAliases` map | ✅ Done |
| [hybrid_resolver.go](./hybrid_resolver.go) | `orchestration` | LLM-first design documentation | ✅ Done |
| [hybrid_resolver_test.go](./hybrid_resolver_test.go) | `orchestration` | Unit tests for HybridResolver | ✅ Done |
| [auto_wire_test.go](./auto_wire_test.go) | `orchestration` | Updated tests for empty SemanticAliases | ✅ Done |
| [executor.go](./executor.go) | `orchestration` | Update `shouldAttemptAICorrection()` for LLM error analysis | ✅ Done |
| [error_analyzer.go](./error_analyzer.go) | `orchestration` | New file: LLM-based error analysis with telemetry | ✅ Done |
| [error_analyzer_test.go](./error_analyzer_test.go) | `orchestration` | Unit tests for ErrorAnalyzer | ✅ Done |
| [micro_resolver.go](./micro_resolver.go) | `orchestration` | LLM micro-resolution with telemetry | ✅ Done |
| [synthesizer.go](./synthesizer.go) | `orchestration` | LLM synthesis with telemetry | ✅ Done |
| [orchestrator.go](./orchestrator.go) | `orchestration` | LLM plan generation with telemetry | ✅ Done |

### Layer 3 Implementation (Complete)

#### File: `orchestration/error_analyzer.go`

```go
// ErrorAnalyzer uses LLM to determine if an error is fixable with different parameters.
// This removes the burden from tool developers to set Retryable flags.
type ErrorAnalyzer struct {
    aiClient  core.AIClient
    logger    core.Logger
}

// ErrorAnalysisResult contains the LLM's decision about error retryability
type ErrorAnalysisResult struct {
    ShouldRetry      bool                   `json:"should_retry"`
    Reason           string                 `json:"reason"`
    SuggestedChanges map[string]interface{} `json:"suggested_changes,omitempty"`
}

// AnalyzeError asks the LLM if this error can be fixed with different parameters
func (e *ErrorAnalyzer) AnalyzeError(
    ctx context.Context,
    httpStatus int,
    errorResponse string,
    originalRequest map[string]interface{},
    userContext string,
) (*ErrorAnalysisResult, error) {
    // Layer 1: HTTP status heuristics (no LLM call needed)
    switch httpStatus {
    case 401, 403, 405:
        return &ErrorAnalysisResult{ShouldRetry: false, Reason: "Auth/permission error"}, nil
    case 408, 429, 500, 502, 503, 504:
        return nil, nil  // Delegate to resilience module (same payload + backoff)
    }

    // Layer 2: LLM analyzes 400, 404, 409, 422 errors
    return e.analyzeWithLLM(ctx, errorResponse, originalRequest, userContext)
}
```

#### Modified: `orchestration/executor.go`

```go
// Update shouldAttemptAICorrection to use ErrorAnalyzer
func (e *SmartExecutor) shouldAttemptAICorrection(
    ctx context.Context,
    httpStatus int,
    err error,
    responseBody string,
    originalRequest map[string]interface{},
) bool {
    // Skip if no error analyzer configured
    if e.errorAnalyzer == nil {
        return e.legacyShouldAttemptAICorrection(err, responseBody)
    }

    // Use LLM-first error analysis
    result, analysisErr := e.errorAnalyzer.AnalyzeError(
        ctx, httpStatus, responseBody, originalRequest, e.userContext)

    if analysisErr != nil || result == nil {
        return false  // Delegate to resilience module or fail
    }

    return result.ShouldRetry
}
```

#### Integration with Resilience Module

The orchestrator already wraps HTTP calls with the resilience module. The error analyzer integrates as follows:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Request Execution Flow                        │
│                                                                  │
│  Executor calls tool via HTTP                                    │
│       │                                                          │
│       ▼                                                          │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Resilience Module (wraps HTTP call)                        │ │
│  │   • Circuit breaker protection                             │ │
│  │   • Handles 408, 429, 5xx with exponential backoff         │ │
│  │   • Same payload retried after delay                       │ │
│  └────────────────────────────────────────────────────────────┘ │
│       │                                                          │
│       ▼ (if error not handled by resilience)                    │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Error Analyzer (orchestration module)                      │ │
│  │   • Handles 400, 404, 409, 422                             │ │
│  │   • LLM analyzes error and suggests corrections            │ │
│  │   • Different payload retried                              │ │
│  └────────────────────────────────────────────────────────────┘ │
│       │                                                          │
│       ▼                                                          │
│  Success OR clear failure message to user                       │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Change 1: Removed Domain-Specific Aliases

**Before:** The framework had 34+ hardcoded aliases:
```go
// OLD - Domain-specific hacks
var SemanticAliases = map[string][]string{
    "lat": {"latitude", "lat", "y", "lat_coord"},
    "lon": {"longitude", "lng", "lon", "x", "long"},
    "to_currency": {"to", "to_currency", "target_currency"},
    // ... 30+ more domain-specific mappings
}
```

**After:** Empty map, LLM handles all semantic understanding:
```go
// NEW - Framework agnostic
var SemanticAliases = map[string][]string{}
```

**Why this matters:**
- Framework works with ANY domain (medical, legal, IoT, etc.)
- No maintenance burden for adding new domains
- LLM naturally understands semantic relationships

### Change 2: LLM-Powered Error Analysis

**Before:** Required tool developers to set `Retryable: true`:
```go
// OLD - Developer burden
if toolError.Retryable {
    useLLMCorrection()  // Only if tool explicitly said so
}
```

**After:** LLM analyzes ALL errors to determine if they're fixable:
```go
// NEW - LLM decides retryability (no developer action needed)
if shouldAttemptAICorrection(err, responseBody, httpStatus) {
    // Layer 1: HTTP status heuristics (no LLM call)
    //   - 401/403 → Never retry (auth errors)
    //   - 429/503 → Resilience module handles (same payload + backoff)
    // Layer 2: LLM error analysis (smart decision)
    //   - Analyzes error message and context
    //   - Decides if different parameters could fix it
    useLLMCorrection()
}
```

**Why this matters:**
- **No developer burden** - Tool authors don't need to set `Retryable` flags
- **Scales automatically** - Works with any tool, any error format
- **Smarter decisions** - LLM understands error semantics

**Separation of Concerns - HTTP Status Code Routing:**

| HTTP Status | Category | Handler | Action | Rationale |
|-------------|----------|---------|--------|-----------|
| **400** | Bad Request | LLM Error Analyzer | Analyze → correct → retry | Malformed params, validation failure - LLM can fix |
| **401** | Unauthorized | Neither | Fail immediately | Auth credentials issue - not fixable by LLM |
| **403** | Forbidden | Neither | Fail immediately | Permissions issue - not fixable by LLM |
| **404** | Not Found | LLM Error Analyzer | Analyze → might fix | "City 'Tokio' not found" → might be typo → "Tokyo" |
| **405** | Method Not Allowed | Neither | Fail immediately | Framework/tool misconfiguration - not input issue |
| **408** | Request Timeout | Resilience module | Same payload + backoff | Transient - request took too long |
| **409** | Conflict | LLM Error Analyzer | Analyze → might fix | Business logic conflict - depends on error details |
| **422** | Unprocessable Entity | LLM Error Analyzer | Analyze → correct → retry | "Amount must be positive" - LLM can fix |
| **429** | Rate Limit | Resilience module | Same payload + backoff | Transient - just need to wait |
| **5xx** | Server Errors | Resilience module | Same payload + backoff | Transient - server issue, same payload will work |

**Summary:**
- **LLM Error Analyzer triggers for:** 400, 404, 409, 422 (input/validation errors that might be fixable)
- **Resilience module handles:** 408, 429, 5xx (transient errors - same payload + exponential backoff)
- **Fail immediately:** 401, 403, 405 (auth/permissions/method issues - not fixable)

---

## How Each Layer Works (Simple Explanation)

### Layer 1: Auto-Wiring

**What it does:** Matches parameters by exact name, no intelligence required.

```
Source data: {"lat": 48.85, "lon": 2.35}
Target needs: lat (number), lon (number)
Result: Direct match! No LLM needed.
```

**What it doesn't do anymore:** No semantic guessing. If source has `"latitude"` and target needs `"lat"`, auto-wiring won't match them. That's Layer 2's job.

### Layer 2: Micro-Resolution

**What it does:** Makes a small LLM call to extract parameters intelligently.

```
Source data: {"latitude": "48.85", "country": "France"}
Target needs: lat (number), to_currency (string)

LLM prompt: "Extract 'lat' and 'to_currency' from this data..."
LLM response: {lat: 48.85, to_currency: "EUR"}
```

The LLM understands that:
- `latitude` means `lat`
- `"48.85"` (string) should be `48.85` (number)
- `France` uses `EUR` currency

### Layer 3: LLM Error Analysis

**What it does:** When a tool call fails, LLM analyzes the error and decides if correction is possible.

```
Tool call: convert_currency({from: "USD", to: "USD", amount: 100})
Tool response: {
    success: false,
    error: {
        code: "INVALID_CURRENCY",
        message: "Source and target currency cannot be the same"
        // No "retryable" field needed - LLM will analyze!
    }
}

Framework asks LLM: "Can this error be fixed with different parameters?"
LLM analysis: {
    should_retry: true,
    reason: "Error indicates same currency used for source and target",
    suggestion: "User wants EUR for Paris trip, change 'to' parameter"
}

Retry: convert_currency({from: "USD", to: "EUR", amount: 100})
```

**Key principle:** The LLM decides retryability, not the tool developer. This scales across any tool without requiring explicit flags.

---

## Implementation Files

| File | Purpose |
|------|---------|
| [auto_wire.go](./auto_wire.go) | Trivial matching: exact names, case-insensitive, type coercion |
| [micro_resolver.go](./micro_resolver.go) | LLM-based parameter extraction via function calling |
| [hybrid_resolver.go](./hybrid_resolver.go) | Combines all layers, orchestrates the resolution flow |
| [error_analyzer.go](./error_analyzer.go) | LLM-based error analysis for smart retry decisions |
| [executor.go](./executor.go) | Step execution, coordinates with resilience module and error analyzer |
| [catalog.go](./catalog.go) | Capability discovery with schema enrichment |

### Error Handling Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Error Handling Flow                          │
│                                                                  │
│  Tool returns HTTP error                                         │
│       │                                                          │
│       ▼                                                          │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ HTTP Status Router (no LLM call)                           │ │
│  │                                                             │ │
│  │   401/403/405 → Fail immediately (auth/permission/method)  │ │
│  │   408/429/5xx → Resilience module (same payload + backoff) │ │
│  │   400/404/409/422 → LLM Error Analyzer (analyze + correct) │ │
│  └────────────────────────────────────────────────────────────┘ │
│       │                                                          │
│       ▼ (400, 404, 409, 422 only)                               │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ LLM Error Analyzer (orchestration/error_analyzer.go)       │ │
│  │                                                             │ │
│  │   Input:                                                    │ │
│  │   • HTTP status code                                        │ │
│  │   • Error response body                                     │ │
│  │   • Original request parameters                             │ │
│  │   • User's original query (context)                         │ │
│  │                                                             │ │
│  │   LLM prompt:                                                │ │
│  │   "Given this error, can the request succeed with           │ │
│  │    different parameters? If yes, what changes?"             │ │
│  │                                                             │ │
│  │   Output: {should_retry, reason, suggested_changes}         │ │
│  └────────────────────────────────────────────────────────────┘ │
│       │                                                          │
│       ▼                                                          │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ If should_retry=true:                                      │ │
│  │   Apply suggested_changes to parameters                    │ │
│  │   Retry with corrected payload                             │ │
│  │                                                             │ │
│  │ If should_retry=false:                                     │ │
│  │   Return clear error with LLM's reason                     │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Resilience Module Integration

The orchestrator coordinates with the resilience module (`resilience/`) for transient errors:

```go
// Resilience module handles (same payload, exponential backoff):
//   408 Request Timeout
//   429 Too Many Requests
//   500 Internal Server Error
//   502 Bad Gateway
//   503 Service Unavailable
//   504 Gateway Timeout
//   Network timeouts, connection refused

// LLM Error Analyzer handles (different payload needed):
//   400 Bad Request      - validation failure, malformed params
//   404 Not Found        - typo in location/symbol name
//   409 Conflict         - business logic conflict
//   422 Unprocessable    - semantic validation error

// Neither handles (fail immediately):
//   401 Unauthorized     - invalid credentials
//   403 Forbidden        - permission denied
//   405 Method Not Allowed - framework misconfiguration
```

This separation ensures:
1. **No wasted LLM calls** for rate limits or service outages
2. **Smart correction** for errors that need different parameters
3. **Proper backoff** handled by battle-tested resilience module
4. **Fast failure** for errors that can't be fixed programmatically

---

## Configuration

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `OPENAI_API_KEY` | Required for micro-resolution and error correction |
| `GOMIND_LOG_LEVEL` | Set to `debug` to see resolution details |

### Disabling LLM Features

Each LLM-powered layer can be independently disabled:

```go
// Disable Layer 2: Micro-resolution (auto-wiring only, no LLM for parameter binding)
resolver := NewHybridResolver(aiClient, logger,
    WithMicroResolution(false))

// Disable Layer 3: Error Analysis (no LLM-based error recovery)
// Option 1: At creation time
analyzer := NewErrorAnalyzer(aiClient, logger,
    WithErrorAnalysisEnabled(false))

// Option 2: At runtime (dynamic toggle)
analyzer.Enable(false)  // Disable
analyzer.Enable(true)   // Re-enable

// Option 3: Check current status
if analyzer.IsEnabled() {
    // LLM error analysis is active
}
```

| Layer | Disable Method | Effect |
|-------|----------------|--------|
| Layer 2 | `WithMicroResolution(false)` | Only exact name matching works; semantic binding disabled |
| Layer 3 | `WithErrorAnalysisEnabled(false)` | Tool errors fail immediately; no smart retry with corrected params |

---

## Debugging

### Log Messages

| Log Message | Meaning |
|-------------|---------|
| `Auto-wiring result` | Shows which params were matched by name |
| `All parameters auto-wired successfully` | Layer 1 succeeded, no LLM needed |
| `Using micro-resolution for unmapped parameters` | Falling back to Layer 2 |
| `SEMANTIC FALLBACK: Attempting LLM resolution` | Detected unresolved template |
| `Attempting AI-powered correction` | Layer 3: LLM analyzing tool error |
| `TYPE COERCION APPLIED` | String converted to number/bool |

### Verification Commands

```bash
# Check if a tool exposes proper schema
curl -s http://localhost:8086/api/capabilities | jq '.capabilities[0].input_summary'

# Test orchestration with logging
GOMIND_LOG_LEVEL=debug curl -X POST http://localhost:8094/orchestrate/natural \
  -H "Content-Type: application/json" \
  -d '{"request":"What is the weather in Tokyo?"}'

# Check resolution logs
kubectl logs -n gomind-examples -l app=travel-research-agent --since=60s | \
  grep -E "Auto-wiring|micro-resolution|SEMANTIC FALLBACK|AI-powered correction"
```

---

## Design Philosophy

### Why LLM-First?

1. **Domain Agnosticism** - Framework has no weather/currency/stock knowledge
2. **Flexibility** - Works with any tool, any domain, any schema
3. **Maintainability** - No aliases to maintain when adding new tools
4. **Future-Proof** - LLMs get better; hardcoded rules don't

### Why Keep Auto-Wiring?

Auto-wiring still handles **trivial cases** to avoid wasting LLM calls:
- `lat` → `lat` (exact match - obvious, no LLM needed)
- `"48.85"` → `48.85` (type coercion - deterministic)

### Cost vs Reliability Tradeoff

| Approach | Cost | Reliability |
|----------|------|-------------|
| Auto-wiring only | Free | Low (breaks on any name mismatch) |
| LLM only | High | High (understands semantics) |
| **Hybrid (current)** | **Optimized** | **High** (LLM when needed) |

---

## Example Flow

**User Query:** "What's the weather in Paris and what currency do they use?"

```
1. LLM plans workflow:
   - Step 1: geocoding_tool.geocode_location("Paris")
   - Step 2: weather_tool.get_weather(lat, lon) [depends on Step 1]
   - Step 3: country_info.get_info("France") [depends on Step 1]
   - Step 4: currency_tool.convert(from, to, amount) [depends on Step 3]

2. Step 1 executes:
   Output: {"lat": 48.85, "lon": 2.35, "country": "France"}

3. Step 2 resolution (weather needs lat/lon):
   - Auto-wiring: lat=48.85, lon=2.35 ✓ (exact match)
   - No LLM needed

4. Step 3 executes:
   Output: {"currency": {"code": "EUR", "name": "Euro"}}

5. Step 4 resolution (currency needs "to" parameter):
   - Auto-wiring: "to" not found in source (source has "currency.code")
   - Micro-resolution: LLM extracts to="EUR" from nested object
   - Success!
```

---

## Markdown Stripping from LLM Responses

### The Problem

When the orchestrator asks an LLM to create an execution plan, the LLM should return pure JSON. However, LLMs often add markdown formatting to their responses, even when explicitly told not to. This causes JSON parsing failures.

**What users see:**
```json
{
  "instruction": "Get weather for **Paris**",
  "agent_name": "*weather-tool*"
}
```

The `*` and `**` characters are markdown formatting (italic and bold) that the LLM added. When this JSON is parsed and passed to tools, the formatting characters remain in the strings, causing confusion or errors.

**Why LLMs do this:**
- LLMs are trained on text that heavily uses markdown
- Emphasizing important words (like city names) with `**bold**` feels natural to them
- Different LLM providers have different tendencies:
  - **Gemini**: Most likely to add markdown, even in JSON mode
  - **GPT-4**: Usually follows "no markdown" instructions well
  - **Claude**: Generally respects format instructions but may occasionally add emphasis

### The Solution: Two-Layer Defense

We use both **prevention** (prompt instructions) and **cleanup** (programmatic stripping).

#### Layer 1: Prompt Instructions (Prevention)

Located in [default_prompt_builder.go:256-264](./default_prompt_builder.go#L256-L264):

```go
CRITICAL FORMAT RULES (applies to all LLM providers):
- You are a JSON API. Your ONLY output is a raw JSON object.
- Output ONLY valid JSON - no markdown, no code blocks, no backticks
- Do NOT use any text formatting: no ** (bold), no * (italic), no __ (underline)
- Do NOT wrap JSON in code fences (no triple backticks)
- Do NOT include any explanatory text before or after the JSON
- String values must be plain text without any markdown formatting
- Start your response with { and end with } - nothing else
```

**Why these specific instructions:**
- "You are a JSON API" - Frames the LLM's role as purely data-focused
- "no ** (bold), no * (italic)" - Explicitly names the characters we want avoided
- "Start with { and end with }" - Removes any intro text like "Here's the plan:"

This works well for GPT-4 and Claude, but Gemini often ignores these instructions.

#### Layer 2: Programmatic Stripping (Cleanup)

Located in [orchestrator.go:1071-1113](./orchestrator.go#L1071-L1113):

```go
// Strip bold: **text** → text
s = markdownBoldRegex.ReplaceAllString(s, "$1")
```

Even if the LLM adds formatting, we programmatically remove it before parsing the JSON.

### How the Stripping Works (Step by Step)

The `cleanLLMResponse()` function (lines 1042-1069) processes LLM output in three steps:

**Step 1: Extract from Code Blocks**
```go
// If LLM wrapped JSON in ```json ... ```, extract the content
if matches := markdownCodeBlockRegex.FindStringSubmatch(s); len(matches) > 1 {
    s = strings.TrimSpace(matches[1])
}
```

This handles responses like:
```
Here's your plan:
```json
{"step_id": "step-1", ...}
```
```

**Step 2: Find JSON Object**
```go
// Find the first { and its matching }
jsonStart := strings.Index(s, "{")
jsonEnd := findJSONEndStringSafe(s, jsonStart)
s = strings.TrimSpace(s[jsonStart:jsonEnd])
```

This handles responses like:
```
I'll create a plan for you.
{"step_id": "step-1", ...}
Let me know if you need changes!
```

**Step 3: Strip Markdown from String Values**
```go
s = stripMarkdownFromJSON(s)
```

This is where `**bold**` becomes `bold` and `*italic*` becomes `italic`.

### The Markdown Stripping Algorithm

The `stripMarkdownFromJSON()` function (lines 1076-1113) handles two patterns:

**Pattern 1: Bold (`**text**`)**
```go
var markdownBoldRegex = regexp.MustCompile(`\*\*([^*]+)\*\*`)
s = markdownBoldRegex.ReplaceAllString(s, "$1")
```

This regex:
- `\*\*` - Matches two asterisks (the opening bold marker)
- `([^*]+)` - Captures one or more characters that aren't asterisks (the text)
- `\*\*` - Matches two asterisks (the closing bold marker)
- `$1` - Replaces the whole match with just the captured text

**Example:** `"Get weather for **Paris**"` → `"Get weather for Paris"`

**Pattern 2: Italic (`*text*`)**

Italic is trickier because we must avoid:
- Matching `**` (bold markers)
- Breaking valid JSON content
- False positives on arithmetic or glob patterns

```go
// Character-by-character analysis
if s[i] == '*' && i+1 < len(s) && s[i+1] != '*' {
    // Look for closing * that isn't part of **
    endIdx := strings.Index(s[i+1:], "*")
    if endIdx > 0 && endIdx < 100 {
        // Verify it's not a double asterisk
        if fullEndIdx+1 >= len(s) || s[fullEndIdx+1] != '*' {
            // Ensure content doesn't have JSON special chars
            content := s[i+1 : fullEndIdx]
            if !strings.ContainsAny(content, "\n\t{}[]\"") {
                // Safe to strip - this is markdown italic
                result.WriteString(content)
            }
        }
    }
}
```

The algorithm is conservative:
- Only strips `*text*` if `text` doesn't contain JSON special characters
- Never strips if the `*` is part of `**`
- Limits match length to 100 characters (prevents runaway matching)

**Example:** `"*weather-tool*"` → `"weather-tool"`
**Preserved:** `"path/*/file"` stays as `"path/*/file"` (not valid markdown)

### Research Sources

This implementation is based on community research and best practices:

1. **OpenAI Community Discussion** (2024)
   - URL: https://community.openai.com/t/how-to-prevent-gpt-from-outputting-responses-in-markdown-format/961314
   - Key insight: "Even with explicit instructions, programmatic post-processing is recommended as a fallback"

2. **DataChain Blog: Enforcing JSON Outputs in Commercial LLMs** (2024)
   - URL: https://datachain.ai/blog/enforcing-json-outputs-in-commercial-llms
   - Key insight: "Gemini is particularly prone to adding markdown despite JSON mode"

### LLM Provider Behaviors

| Provider | JSON Mode | Markdown Tendency | Recommendation |
|----------|-----------|-------------------|----------------|
| **OpenAI GPT-4** | `response_format: json_object` | Low | Prompt instructions usually sufficient |
| **Anthropic Claude** | No native JSON mode | Low | Prompt instructions usually sufficient |
| **Google Gemini** | `responseMimeType: application/json` | High | Always use programmatic stripping |
| **Groq** | Follows OpenAI format | Medium | Use both layers |
| **DeepSeek** | OpenAI-compatible | Medium | Use both layers |

### Regex Patterns Reference

Defined in [orchestrator.go:1020-1029](./orchestrator.go#L1020-L1029):

```go
// Matches ```json ... ``` or ``` ... ```
var markdownCodeBlockRegex = regexp.MustCompile("(?s)```(?:json)?\\s*([\\s\\S]*?)\\s*```")

// Matches **bold text**
var markdownBoldRegex = regexp.MustCompile(`\*\*([^*]+)\*\*`)

// Reference pattern for italic (actual implementation uses character iteration)
var markdownItalicRegex = regexp.MustCompile(`(?:^|[^*])\*([^*]+)\*(?:[^*]|$)`)
```

### What Gets Cleaned (Examples)

| Before (LLM Response) | After (Cleaned) |
|-----------------------|-----------------|
| `"**Paris**"` | `"Paris"` |
| `"*weather-tool*"` | `"weather-tool"` |
| `"Get **current** weather"` | `"Get current weather"` |
| `"Tool: *geocoding*"` | `"Tool: geocoding"` |
| `"Value: 123"` | `"Value: 123"` (unchanged) |
| `"path/*/glob"` | `"path/*/glob"` (unchanged - not markdown) |

### Debug Logging

When markdown is stripped, you can see it in the logs:

```bash
# Enable debug logging
GOMIND_LOG_LEVEL=debug kubectl logs -l app=travel-research-agent

# Look for cleaning messages
grep -E "cleanLLMResponse|stripMarkdown" logs.txt
```

### Safety Guarantees

The stripping is designed to be safe for JSON:

1. **No false positives on JSON structure** - `{}`, `[]`, `""` are never modified
2. **No false positives on arithmetic** - `"5 * 3"` stays as `"5 * 3"`
3. **No multi-line matching** - Prevents runaway matches across the entire document
4. **Conservative italic handling** - Only strips when confident it's markdown

---

## Related Documentation

- [INTELLIGENT_ERROR_HANDLING.md](../docs/INTELLIGENT_ERROR_HANDLING.md) - Retry and correction patterns
- [ENVIRONMENT_VARIABLES_GUIDE.md](../docs/ENVIRONMENT_VARIABLES_GUIDE.md) - All configuration options
- [examples/agent-with-orchestration/](../examples/agent-with-orchestration/) - Working example

---

*Document version: 1.0 — Intelligent Parameter Binding Reference*
