# Semantic Retry Architecture Design

> **Status**: Implemented ✅
> **Author**: Neelabh Tripathi (@itsneelabh)
> **Created**: December 2025
> **Implementation**: December 2025
> **Related**: [error_analyzer.go](../error_analyzer.go), [micro_resolver.go](../micro_resolver.go), [executor.go](../executor.go), [contextual_re_resolver.go](../contextual_re_resolver.go)
> **Companion Doc**: [INTELLIGENT_PARAMETER_BINDING.md](../INTELLIGENT_PARAMETER_BINDING.md) (Layers 1-3)

---

## Document Relationship

This document describes **Layer 4** of the gomind parameter resolution system. It complements [INTELLIGENT_PARAMETER_BINDING.md](./INTELLIGENT_PARAMETER_BINDING.md), which documents Layers 1-3:

| Document | Scope | When |
|----------|-------|------|
| **[INTELLIGENT_PARAMETER_BINDING.md](./INTELLIGENT_PARAMETER_BINDING.md)** | Layers 1-3: Auto-wiring, Micro-Resolution, Error Analysis | **Before** execution |
| **This document** | Layer 4: Contextual Re-Resolution | **After** execution fails |

The key insight: Layers 1-3 handle parameter binding before we know if they work. Layer 4 handles the case where they didn't work, but we now have both the error context AND the original source data to compute a fix.

---

## Executive Summary

This document proposes a **Contextual Semantic Retry** mechanism that addresses a critical gap in the current orchestration module: the inability to correct parameter extraction errors that require computation or full context awareness.

**The Problem**: When a step fails due to incorrect parameters (e.g., `amount: 0` instead of `amount: 46828.5`), the ErrorAnalyzer can diagnose the issue but cannot prescribe a fix because it lacks access to the source data needed to compute the correct value.

**The Solution**: A new `ContextualReResolver` component that receives full execution context (user query, source data, error details) and uses LLM capabilities to compute corrected parameters.

---

## Table of Contents

1. [End-to-End Flow Walkthrough](#end-to-end-flow-walkthrough) ← **Start here for context**
2. [Current Architecture Analysis](#current-architecture-analysis)
3. [The Gap: A Concrete Example](#the-gap-a-concrete-example)
4. [Research Foundation](#research-foundation)
5. [Proposed Solution](#proposed-solution)
6. [Implementation Design](#implementation-design)
7. [Retry Loop Mechanics](#retry-loop-mechanics)
8. [Integration Points](#integration-points) ← **Complete before/after code**
9. [Configuration](#configuration)
10. [Telemetry & Observability](#telemetry--observability)
11. [Testing Strategy](#testing-strategy)
12. [Migration Path](#migration-path)

---

## Quick Reference: Implementation Checklist

| # | File | Action | Section |
|---|------|--------|---------|
| 1 | `orchestration/contextual_re_resolver.go` | **CREATE** | [Section 6](#6-new-file-contextual_re_resolvergo) |
| 2 | `orchestration/contextual_re_resolver_test.go` | **CREATE** | [Testing Strategy](#testing-strategy) |
| 3 | `orchestration/executor.go:52-76` | **MODIFY** - Add struct fields | [Section 1](#1-executorgo---smartexecutor-struct) |
| 4 | `orchestration/executor.go:180` | **ADD** - New setter methods | [Section 2](#2-executorgo---new-setter-methods) |
| 5 | `orchestration/executor.go:1328` | **ADD** - `previousErrors` variable | [Section 3](#3-executorgo---integration-in-executestep) |
| 6 | `orchestration/executor.go:1463-1487` | **MODIFY** - Integration code | [Section 3](#3-executorgo---integration-in-executestep) |
| 7 | `orchestration/interfaces.go:193-226` | **MODIFY** - Add SemanticRetryConfig | [Section 4](#4-interfacesgo---orchestratorconfig-extension) |
| 8 | `orchestration/interfaces.go:229-281` | **MODIFY** - Update DefaultConfig() | [Section 5](#5-interfacesgo---defaultconfig-update) |

**Key Dependencies**:
- Reuses `findJSONEndSimple` from [error_analyzer.go:349-388](./error_analyzer.go#L349-L388)
- Reuses `truncateString` from [orchestrator.go](./orchestrator.go)
- Reuses `getMapKeys` from existing code or inline
- Uses `collectDependencyResults` from [executor.go:514](./executor.go#L514)

---

## End-to-End Flow Walkthrough

This section provides a complete walkthrough from agent code to orchestration module internals, showing exactly where Layer 4 (Semantic Retry) fits in the execution flow.

### 1. Agent Creates Orchestrator

Your agent creates the orchestrator in its main function:

```go
// examples/agent-with-orchestration/main.go (typical agent pattern)

func main() {
    // Create discovery (Redis or K8s)
    discovery := core.NewRedisDiscovery(redisURL)

    // Create AI client
    aiClient := openai.NewClient(apiKey)

    // Create orchestrator with intelligent defaults
    config := orchestration.DefaultConfig()
    orch := orchestration.NewAIOrchestrator(config, discovery, aiClient)

    // Set optional observability
    orch.SetLogger(myLogger)
    orch.SetTelemetry(myTelemetry)

    // Start catalog refresh loop
    orch.Start(ctx)
}
```

### 2. Inside NewAIOrchestrator

**Location**: [orchestrator.go:49-110](./orchestrator.go#L49-L110)

```go
func NewAIOrchestrator(config *OrchestratorConfig, ...) *AIOrchestrator {
    catalog := NewAgentCatalog(discovery)

    o := &AIOrchestrator{
        config:      config,
        executor:    NewSmartExecutor(catalog),   // ← Creates Executor
        synthesizer: NewAISynthesizer(aiClient),
        // ...
    }

    // Wire Layer 3 Validation Feedback
    if config.ExecutionOptions.ValidationFeedbackEnabled {
        o.executor.SetCorrectionCallback(o.requestParameterCorrection)
    }

    // Wire Hybrid Resolution (Layer 2)
    if config.EnableHybridResolution {
        hybridResolver := NewHybridResolver(aiClient, nil)
        o.executor.SetHybridResolver(hybridResolver)
    }

    // [PROPOSED] Wire Layer 4 Semantic Retry
    // if config.SemanticRetry.Enabled {
    //     reResolver := NewContextualReResolver(aiClient, nil)
    //     o.executor.SetContextualReResolver(reResolver)
    //     o.executor.SetMaxSemanticRetries(config.SemanticRetry.MaxAttempts)
    // }

    return o
}
```

### 3. Agent Calls ProcessRequest

**Location**: [orchestrator.go:343-441](./orchestrator.go#L343-L441)

```go
func (o *AIOrchestrator) ProcessRequest(ctx context.Context, request string, ...) {
    // Step 1: LLM generates execution plan
    plan, err := o.generateExecutionPlan(ctx, request, requestID)

    // Step 2: Validate the plan
    if err := o.validatePlan(plan); err != nil {
        plan, err = o.regeneratePlan(ctx, request, requestID, err)
    }

    // Step 3: Execute the plan via SmartExecutor ← KEY CALL
    result, err := o.executor.Execute(ctx, plan)

    // Step 4: Synthesize final response
    response, err := o.synthesizer.Synthesize(ctx, request, result)

    return response, nil
}
```

### 4. Inside Execute

**Location**: [executor.go:182-269](./executor.go#L182-L269)

```go
func (e *SmartExecutor) Execute(ctx context.Context, plan *RoutingPlan) (*ExecutionResult, error) {
    stepResults := make(map[string]*StepResult)
    executed := make(map[string]bool)

    // Process steps in dependency order
    for len(executed) < len(plan.Steps) {
        readySteps := e.findReadySteps(plan, executed, stepResults)

        for _, step := range readySteps {
            // Execute single step (with retry logic)
            stepResult, _ := e.ExecuteStep(ctx, step)  // ← All layers happen here
            stepResults[step.StepID] = stepResult
        }

        for _, step := range readySteps {
            executed[step.StepID] = true
        }
    }

    return result, nil
}
```

### 5. Inside ExecuteStep - The Heart of the Flow

**Location**: [executor.go:1100-1550](./executor.go#L1100-L1550)

This is where all layers come together:

```go
func (e *SmartExecutor) ExecuteStep(ctx context.Context, step RoutingStep) (*StepResult, error) {
    // PHASE 1: Discover the target agent/tool
    agentInfo, _ := e.catalog.GetAgent(step.AgentName)

    // PHASE 2: LAYER 2 - Hybrid Parameter Resolution
    if len(step.DependsOn) > 0 && e.useHybridResolution && e.hybridResolver != nil {
        stepResultsMap := e.collectDependencyResults(ctx, step.DependsOn)
        resolved, _ := e.hybridResolver.ResolveParameters(ctx, stepResultsMap, schema)
        // Merge resolved into parameters
    }

    // PHASE 3: Call the tool/agent (retry loop)
    for attempt := 0; attempt < retryAttempts; attempt++ {
        response, responseBody, err := e.callTool(ctx, url, parameters)

        if err == nil {
            // SUCCESS!
            break
        }

        // PHASE 4: LAYER 3 - LLM Error Analysis
        if e.errorAnalyzer != nil {
            analysisResult, _ := e.errorAnalyzer.AnalyzeError(ctx, errCtx)

            if analysisResult.ShouldRetry {
                // Apply suggested changes and retry
                continue
            }

            if !analysisResult.ShouldRetry {
                // ★ THE GAP: ErrorAnalyzer lacks source data

                // [PROPOSED] PHASE 5: LAYER 4 - Contextual Re-Resolution
                // if e.contextualReResolver != nil {
                //     sourceData := e.collectSourceDataFromDependencies(ctx, step.DependsOn)
                //     execCtx := &ExecutionContext{
                //         UserQuery:       step.Instruction,
                //         SourceData:      sourceData,  // ← HAS the data!
                //         AttemptedParams: parameters,
                //         ErrorResponse:   responseBody,
                //         ...
                //     }
                //     reResult, _ := e.contextualReResolver.ReResolve(ctx, execCtx)
                //     if reResult.ShouldRetry {
                //         parameters = reResult.CorrectedParameters
                //         attempt--
                //         continue  // ← Retry with computed fix!
                //     }
                // }

                break  // Current behavior: give up
            }
        }
    }

    return result, nil
}
```

### Visual Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              AGENT CODE                                          │
│  response, err := orchestrator.ProcessRequest(ctx, "weather in Paris", nil)     │
└──────────────────────────────────┬──────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         ProcessRequest (orchestrator.go:343)                     │
│  1. generateExecutionPlan() → LLM creates multi-step plan                       │
│  2. validatePlan()                                                               │
│  3. executor.Execute(plan) ────────────────────────────────────────────┐        │
│  4. synthesizer.Synthesize(results)                                     │        │
└─────────────────────────────────────────────────────────────────────────┼────────┘
                                                                          │
                                   ┌──────────────────────────────────────┘
                                   ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           Execute (executor.go:182)                              │
│  FOR each step in dependency order:                                              │
│    - findReadySteps() → steps with all dependencies satisfied                   │
│    - ExecuteStep(ctx, step) ───────────────────────────────────────────┐        │
└─────────────────────────────────────────────────────────────────────────┼────────┘
                                                                          │
                                   ┌──────────────────────────────────────┘
                                   ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         ExecuteStep (executor.go:1100)                           │
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────────┐│
│  │ LAYER 2: Hybrid Parameter Resolution (Lines 1138-1206)                      ││
│  │   hybridResolver.ResolveParameters(ctx, dependencyResults, schema)          ││
│  │     → AutoWirer: exact/case-insensitive name match                          ││
│  │     → MicroResolver: LLM extraction for semantic mappings                   ││
│  └─────────────────────────────────────────────────────────────────────────────┘│
│                                    │                                             │
│                                    ▼                                             │
│  ┌─────────────────────────────────────────────────────────────────────────────┐│
│  │ CALL TOOL: callTool(ctx, url, parameters) (Line 1356)                       ││
│  └─────────────────────────────────────────────────────────────────────────────┘│
│                                    │                                             │
│                          ┌─────────┴─────────┐                                   │
│                          ▼                   ▼                                   │
│                      SUCCESS              ERROR                                  │
│                         │                   │                                    │
│                         │    ┌──────────────┴──────────────┐                     │
│                         │    ▼                             │                     │
│  ┌──────────────────────│────────────────────────────────┐ │                     │
│  │ LAYER 3: ErrorAnalyzer (Lines 1382-1487)              │ │                     │
│  │   errorAnalyzer.AnalyzeError(ctx, errContext)         │ │                     │
│  │   → NO SourceData available!                          │ │                     │
│  │   → Can diagnose but cannot compute fixes             │ │                     │
│  └───────────────────────────────────────────────────────┘ │                     │
│                              │                              │                    │
│               ┌──────────────┴──────────────┐               │                    │
│               ▼                             ▼               │                    │
│         ShouldRetry=true              ShouldRetry=false     │                    │
│         (has SuggestedChanges)        (THE GAP!)            │                    │
│               │                             │               │                    │
│               ▼                             ▼               │                    │
│           RETRY                    ┌────────────────────────┘                    │
│       (with suggestions)           │                                             │
│                                    ▼                                             │
│  ┌─────────────────────────────────────────────────────────────────────────────┐│
│  │ [PROPOSED] LAYER 4: ContextualReResolver                                    ││
│  │   → HAS SourceData (from dependencies)                                       ││
│  │   → CAN compute fixes (e.g., 100 × 468.285 = 46828.5)                        ││
│  │   → parameters = reResult.CorrectedParameters                                ││
│  │   → attempt--; continue  // Semantic retry!                                  ││
│  └─────────────────────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────────────────────┘
```

### The Gap Illustrated with Real Data

**What Layer 3 (ErrorAnalyzer) receives** - MISSING SOURCE DATA:

```go
ErrorAnalysisContext{
    HTTPStatus:      422,
    ErrorResponse:   "amount must be greater than 0",
    OriginalRequest: {"from": "EUR", "to": "JPY", "amount": -46828.5},
    UserQuery:       "How much is 100 EUR in JPY?",
    CapabilityName:  "convert_currency",
    // ❌ NO SourceData - doesn't know price=468.285
}
// LLM thinks: "User wanted 100, but amount is negative... I don't know the exchange rate"
// Returns: {ShouldRetry: false, Reason: "Cannot determine correct amount"}
```

**What Layer 4 (ContextualReResolver) receives** - HAS SOURCE DATA:

```go
ExecutionContext{
    UserQuery:       "How much is 100 EUR in JPY?",
    SourceData:      {"price": 468.285, "currency": "EUR", ...},  // ← KEY!
    AttemptedParams: {"from": "EUR", "to": "JPY", "amount": -46828.5},
    ErrorResponse:   "amount must be greater than 0",
    HTTPStatus:      422,
    Capability:      {Name: "convert_currency", Parameters: [...]},
}
// LLM understands: "User wants 100 × 468.285 = 46828.5 (positive!)"
// Returns: {ShouldRetry: true, CorrectedParameters: {"amount": 46828.5}}
```

This is the architectural gap that Layer 4 fills.

---

## Current Architecture Analysis

### Existing Components

```
┌─────────────────────────────────────────────────────────────────────┐
│                    CURRENT ORCHESTRATION FLOW                        │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  1. PLAN GENERATION (AI Planner)                                    │
│     └─ Creates steps with parameter templates                       │
│        e.g., {amount: "{{step-1.response.price}}"}                  │
│                                                                      │
│  2. PARAMETER RESOLUTION (Before Execution)                         │
│     ├─ HybridResolver (Auto-wiring + MicroResolution)               │
│     │   └─ LLM extracts values from source data                     │
│     └─ Template interpolation: {{step-1.price}} → 468.29            │
│                                                                      │
│  3. STEP EXECUTION (HTTP Call)                                      │
│     └─ Tool returns error (e.g., HTTP 400)                          │
│                                                                      │
│  4. ERROR ANALYSIS (After Failure)                                  │
│     └─ ErrorAnalyzer determines: retry or fail                      │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | When | Input | Output | Limitation |
|-----------|------|-------|--------|------------|
| **MicroResolver** | Before execution | Source data, target schema | Extracted parameters | Cannot see errors |
| **ErrorAnalyzer** | After failure | Error response, failed params | Retry decision | Cannot see source data |
| **CorrectionCallback** | After type error | Error message, schema | Type-corrected params | Limited to type fixes |

### The Architectural Gap

The `ErrorAnalysisContext` struct ([error_analyzer.go:63-81](./error_analyzer.go#L63-L81)) receives:
- ✅ HTTP status code (`HTTPStatus`)
- ✅ Error response body (`ErrorResponse`)
- ✅ Original request parameters (`OriginalRequest`)
- ✅ User query (`UserQuery`)
- ✅ Capability description (`CapabilityDescription`)

But it's **MISSING**:
- ❌ Source data from dependent steps (no `SourceData` field)
- ❌ Computational context (e.g., "100 shares × price")
- ❌ Ability to perform arithmetic

**Evidence from executor.go** ([lines 1386-1396](./executor.go#L1386-L1396)):
```go
errCtx := &ErrorAnalysisContext{
    HTTPStatus:            httpStatus,
    ErrorResponse:         responseBody,
    OriginalRequest:       parameters,
    UserQuery:             step.Instruction,
    CapabilityName:        capability,
    CapabilityDescription: "",
    // ❌ NO SourceData - dependency data is NOT passed here
}
```

---

## The Gap: A Concrete Example

### Scenario: Currency Conversion After Stock Lookup

**User Query**: "I am planning to sell 100 Tesla shares to fund my travel to Seoul"

**Execution Flow**:

```
Step 1: get_stock_quote("TSLA")
  → Response: {symbol: "TSLA", current_price: 468.285}

Step 2: geocode_location("Seoul")
  → Response: {lat: 37.5665, lon: 126.978, country: "South Korea"}

Step 5: convert_currency(from, to, amount)  [depends on step-1, step-2]
  → MicroResolver extracts: {from: "USD", to: "KRW", amount: 0}
  → HTTP 400: "amount must be greater than 0"
```

**Why MicroResolver Failed**:
- It saw `current_price: 468.285` in source data
- It saw user mentioned "100 shares"
- But it didn't **compute** `100 × 468.285 = 46828.5`
- Instead, it defaulted to `amount: 0`

**Why ErrorAnalyzer Can't Fix It**:
```go
// ErrorAnalyzer receives:
errCtx := &ErrorAnalysisContext{
    HTTPStatus:      400,
    ErrorResponse:   `{"error": "amount must be greater than 0"}`,
    OriginalRequest: map[string]interface{}{"from": "USD", "to": "KRW", "amount": 0},
    UserQuery:       "I am planning to sell 100 Tesla shares...",
    // MISSING: source_data with {current_price: 468.285}
}

// Result:
{
    "should_retry": false,
    "reason": "cannot be fixed by modifying request parameters
               without changing the amount to a positive value"
}
```

The ErrorAnalyzer correctly identifies the problem but **cannot compute the fix** because it doesn't have access to the stock price.

---

## Research Foundation

### Key Patterns from 2025 LLM Agent Research

#### 1. Reflexion Pattern
> "Given a sparse reward signal (success/fail), the current trajectory, and its persistent memory, the self-reflection model generates nuanced and specific feedback."
> — [Shinn et al., Reflexion: Language Agents with Verbal Reinforcement Learning](https://arxiv.org/pdf/2303.11366)

**Insight**: Store the entire execution trajectory, not just the error.

#### 2. Retrials Without Feedback
> "Simpler methods such as chain-of-thoughts often outperform more sophisticated reasoning frameworks when given the opportunity to retry."
> — [Are Retrials All You Need?](https://arxiv.org/html/2504.12951)

**Insight**: Sometimes re-running resolution with error context is sufficient.

#### 3. LLM Self-Correction
> "Pass tool call error messages back to the LLM so it can correct itself."

**Insight**: The LLM that made the mistake should be the one to fix it.

#### 4. Process Calling for State
> "For customer-facing AI, function calling is the wrong abstraction. You need something robust and stateful."
> — [Rasa: Process Calling](https://rasa.com/blog/process-calling-agentic-tools-need-state)

**Insight**: Track state across the entire execution, not just per-step.

#### 5. Anthropic's Agent Design Principles
> "The most successful implementations use simple, composable patterns rather than complex frameworks."
> — [Anthropic: Building Effective Agents](https://www.anthropic.com/research/building-effective-agents)

**Insight**: Keep the retry mechanism simple and focused.

---

## Proposed Solution

### Design Principles

1. **LLM-Guided Everything**: The LLM made the parameter extraction decision; it should also fix it
2. **Full Context on Retry**: The re-resolver needs ALL information MicroResolver had, plus the error
3. **Domain-Agnostic**: No hardcoded domain knowledge - LLM infers what's needed from context
4. **Memory Across Attempts**: Each retry should benefit from previous failure context
5. **Single Responsibility**: Separate component from ErrorAnalyzer (analysis ≠ resolution)

### Three-Phase Semantic Retry Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                    PROPOSED SEMANTIC RETRY FLOW                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  PHASE 1: INITIAL RESOLUTION (existing)                             │
│  ├─ MicroResolver extracts: {amount: 0} ← LLM mistake               │
│  └─ Execution fails: HTTP 400                                       │
│                                                                      │
│  PHASE 2: CONTEXTUAL RE-RESOLUTION (NEW)                            │
│  ├─ Triggered by: HTTP 4xx with validation error                    │
│  ├─ Input to LLM:                                                   │
│  │   • Original user query: "sell 100 Tesla shares..."              │
│  │   • Source data: {price: 468.29, symbol: "TSLA"}                 │
│  │   • Failed parameters: {from: "USD", to: "KRW", amount: 0}       │
│  │   • Error message: "amount must be greater than 0"               │
│  │   • Target schema: {amount: number, from: string, to: string}    │
│  │                                                                   │
│  ├─ Explicit instruction:                                           │
│  │   "The previous attempt failed. Analyze the error and compute    │
│  │    the correct values. If calculation is needed (e.g., shares    │
│  │    × price), perform it and return the numeric result."          │
│  │                                                                   │
│  └─ Output: {amount: 46828.5} ← LLM computes 100 * 468.29           │
│                                                                      │
│  PHASE 3: RETRY WITH CORRECTED PARAMETERS                           │
│  └─ Execution succeeds: HTTP 200                                    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Implementation Design

### Overview

The complete implementation is provided in **[Section 6: NEW FILE: contextual_re_resolver.go](#6-new-file-contextual_re_resolvergo)** of the Integration Points section. Here we summarize the key design decisions.

### New Types

| Type | Purpose | Location |
|------|---------|----------|
| `ExecutionContext` | Captures full trajectory for re-resolution | `contextual_re_resolver.go` |
| `ReResolutionResult` | LLM response with corrected parameters | `contextual_re_resolver.go` |
| `ContextualReResolver` | Main component that performs re-resolution | `contextual_re_resolver.go` |
| `SemanticRetryConfig` | Configuration for Layer 4 | `interfaces.go` |

### Key Design Decisions

**1. ExecutionContext includes ALL information MicroResolver had, plus error:**
```go
type ExecutionContext struct {
    UserQuery       string                  // Original intent
    SourceData      map[string]interface{}  // Data from dependencies (key!)
    StepID          string
    Capability      *EnhancedCapability     // Target schema
    AttemptedParams map[string]interface{}  // What failed
    ErrorResponse   string                  // Error message
    HTTPStatus      int
    RetryCount      int                     // Memory across attempts
    PreviousErrors  []string
}
```

**2. Domain-agnostic prompt** - The framework provides context, LLM infers computation:
- User query → LLM understands intent ("sell 100 shares")
- Source data → LLM sees available values (`{price: 468.29}`)
- Error → LLM knows the constraint (`amount must be > 0`)
- Schema → LLM knows the target type (`amount: number`)

**3. Reuses existing helpers:**
- `findJSONEndSimple` from [error_analyzer.go:349-388](./error_analyzer.go#L349-L388)
- `truncateString` from [orchestrator.go](./orchestrator.go)
- `getMapKeys` from [utils.go](./utils.go) or inline

**4. Follows existing patterns:**
- Logger setup matches [error_analyzer.go:392-402](./error_analyzer.go#L392-L402)
- Telemetry follows [error_analyzer.go:188-259](./error_analyzer.go#L188-L259)
- Response parsing follows [error_analyzer.go:317-346](./error_analyzer.go#L317-L346)

---

## Retry Loop Mechanics

When the LLM returns corrected parameters, here's exactly what happens in the executor:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    RETRY LOOP MECHANICS                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  for attempt := 1; attempt <= maxAttempts; attempt++ {              │
│                                                                      │
│      1. EXECUTE HTTP CALL                                           │
│         response, err = httpClient.Do(req, parameters)              │
│                                                                      │
│      2. IF SUCCESS → break (done!)                                  │
│                                                                      │
│      3. IF ERROR → Run ErrorAnalyzer                                │
│         analysisResult = errorAnalyzer.Analyze(err, params)         │
│                                                                      │
│      4. IF ErrorAnalyzer says "should_retry: false"                 │
│         AND semanticRetries < maxSemanticRetries:                   │
│                                                                      │
│         4a. Build ExecutionContext with ALL data                    │
│             execCtx = {                                             │
│                 UserQuery:       step.Instruction,                  │
│                 SourceData:      collectFromDependencies(),         │
│                 AttemptedParams: parameters,                        │
│                 ErrorResponse:   responseBody,                      │
│                 ...                                                 │
│             }                                                       │
│                                                                      │
│         4b. Call ContextualReResolver                               │
│             reResult = reResolver.ReResolve(ctx, execCtx)           │
│                                                                      │
│         4c. IF reResult.ShouldRetry:                                │
│             - parameters = reResult.CorrectedParameters  ← REPLACE  │
│             - semanticRetries++                                     │
│             - attempt--  ← DON'T COUNT AS REGULAR RETRY             │
│             - continue   ← GO BACK TO STEP 1 WITH NEW PARAMS        │
│                                                                      │
│      5. Regular retry logic (same params, just retry)               │
│         - Sleep with backoff                                        │
│         - continue                                                  │
│  }                                                                  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Key Mechanics

1. **`parameters` is replaced** - The corrected parameters completely replace the failed ones
2. **`attempt--`** - Semantic retries don't count against the regular retry budget
3. **`continue`** - Loop restarts, making a fresh HTTP call with new parameters
4. **Separate counters** - `semanticRetries` tracks LLM-corrected attempts, `attempt` tracks infrastructure retries

### Concrete Example

```go
// Initial state
parameters = {"from": "USD", "to": "KRW", "amount": 0}
attempt = 1
semanticRetries = 0

// Attempt 1: HTTP 400 "amount must be > 0"
// ErrorAnalyzer: should_retry = false
// ContextualReResolver: should_retry = true, corrected = {amount: 46828.5}

parameters = {"from": "USD", "to": "KRW", "amount": 46828.5}  // REPLACED
semanticRetries = 1
attempt = 0  // decremented
// continue → attempt becomes 1 again

// Attempt 1 (retry): HTTP 200 OK ✓
// break → success!
```

The corrected parameters flow directly into the same HTTP call logic - no special handling needed. The tool receives a valid request and responds normally.

### Transient Error Handling

A critical edge case arises with **transient infrastructure errors** (503 timeouts, service unavailable). For these errors:

1. **LLM may return `should_retry=true` with empty `suggested_changes`** - meaning "retry with same parameters"
2. **Tool responses may contain `retryable: false`** - but LLM's assessment should take precedence for transient issues

#### The `IsTransientError` Field

The `ErrorAnalysisResult` includes an `IsTransientError` field that LLM sets for infrastructure issues:

```go
type ErrorAnalysisResult struct {
    ShouldRetry      bool                   `json:"should_retry"`
    Reason           string                 `json:"reason"`
    SuggestedChanges map[string]interface{} `json:"suggested_changes,omitempty"`
    // IsTransientError signals temporary infrastructure issues
    // (timeouts, service unavailable, connection refused)
    IsTransientError bool `json:"is_transient_error,omitempty"`
}
```

#### The `isTransientErrorDetected` Flag

In `executor.go`, a local flag tracks when LLM identifies a transient error:

```go
for attempt := 1; attempt <= maxAttempts; attempt++ {
    isTransientErrorDetected := false  // Reset each attempt

    // ... HTTP call and error analysis ...

    if analysisResult.IsTransientError {
        isTransientErrorDetected = true
        // Continue with resilience retry (same payload + backoff)
    }

    // IMPORTANT: Skip isNonRetryableToolError check for transient errors
    // LLM's assessment takes precedence over tool's retryable flag
    if !isTransientErrorDetected && isNonRetryableToolError(responseBody) {
        break  // Tool says don't retry, respect it
    }
}
```

This ensures that when LLM detects a transient error (e.g., 503 timeout), the executor doesn't break out early due to `retryable: false` in the tool response.

#### The `should_retry=true` with Empty Changes Scenario

When LLM determines an error is retryable but doesn't have specific parameter changes:

```go
// Changed from:
if analysisResult.ShouldRetry && len(analysisResult.SuggestedChanges) > 0 {

// To:
if analysisResult.ShouldRetry {
    // Handles both:
    // 1. Parameter changes needed (SuggestedChanges non-empty)
    // 2. Transient errors where retry with same params may work (SuggestedChanges empty)

    // Merge changes (no-op if empty)
    for key, val := range analysisResult.SuggestedChanges {
        parameters[key] = val
    }

    attempt--
    continue
}
```

This ensures transient errors that slip through HTTP status routing (like 503 errors with structured tool responses) are handled correctly.

#### When Each Path Activates

| Scenario | `ShouldRetry` | `SuggestedChanges` | `IsTransientError` | Action |
|----------|---------------|--------------------|--------------------|--------|
| Typo fix (e.g., "Tokio" → "Tokyo") | `true` | `{location: "Tokyo"}` | `false` | Retry with new params |
| Transient timeout | `true` | `{}` | `true` | Retry with same params |
| Permanent error (auth fail) | `false` | `{}` | `false` | Don't retry |
| Transient but not fixable | `false` | `{}` | `true` | Resilience retry |

---

## Integration Points

### File Changes Required

This section provides exact before/after code for each file that needs modification.

---

### 1. executor.go - SmartExecutor Struct

**Location**: [executor.go:52-76](./executor.go#L52-L76)

**BEFORE** (current struct):
```go
type SmartExecutor struct {
    catalog        *AgentCatalog
    httpClient     *http.Client
    maxConcurrency int
    semaphore      chan struct{}
    logger core.Logger
    correctionCallback        CorrectionCallback
    validationFeedbackEnabled bool
    maxValidationRetries      int
    hybridResolver       *HybridResolver
    useHybridResolution  bool
    errorAnalyzer *ErrorAnalyzer
}
```

**AFTER** (add new fields at end):
```go
type SmartExecutor struct {
    catalog        *AgentCatalog
    httpClient     *http.Client
    maxConcurrency int
    semaphore      chan struct{}
    logger core.Logger
    correctionCallback        CorrectionCallback
    validationFeedbackEnabled bool
    maxValidationRetries      int
    hybridResolver       *HybridResolver
    useHybridResolution  bool
    errorAnalyzer *ErrorAnalyzer

    // Layer 4: Contextual Re-Resolution for Semantic Retry
    // When ErrorAnalyzer says "cannot fix" but source data exists,
    // this component can compute derived values using full context.
    contextualReResolver *ContextualReResolver
    maxSemanticRetries   int  // Default: 2
}
```

---

### 2. executor.go - New Setter Methods

**Location**: Add after `SetLogger` method (around [executor.go:180](./executor.go#L180), after the logger propagation to errorAnalyzer)

**ADD** (new methods):
```go
// SetContextualReResolver configures Layer 4 semantic retry.
// When set, the executor can re-resolve parameters after failures using full context.
// This complements ErrorAnalyzer by providing source data for computation.
func (e *SmartExecutor) SetContextualReResolver(resolver *ContextualReResolver) {
    e.contextualReResolver = resolver
    if e.logger != nil && resolver != nil {
        e.logger.Info("Contextual re-resolver enabled for semantic retries", map[string]interface{}{
            "operation": "contextual_re_resolver_configured",
        })
    }
}

// SetMaxSemanticRetries sets the maximum number of semantic retry attempts.
// Default is 2. These retries are separate from regular retry attempts.
func (e *SmartExecutor) SetMaxSemanticRetries(max int) {
    if max < 0 {
        max = 0
    }
    e.maxSemanticRetries = max
}

// collectSourceDataFromDependencies converts step results to a flat source data map.
// This is the same logic used by HybridResolver.collectSourceData.
// Returns map[string]interface{} suitable for LLM context.
func (e *SmartExecutor) collectSourceDataFromDependencies(ctx context.Context, dependsOn []string) map[string]interface{} {
    sourceData := make(map[string]interface{})

    // Get step results from context (stored by buildStepContext)
    stepResults := e.collectDependencyResults(ctx, dependsOn)

    // Convert each step result to flat key-value pairs
    for _, result := range stepResults {
        if result == nil || result.Response == "" || !result.Success {
            continue
        }

        var parsed map[string]interface{}
        if err := json.Unmarshal([]byte(result.Response), &parsed); err != nil {
            if e.logger != nil {
                e.logger.Warn("Failed to parse step response for source data", map[string]interface{}{
                    "step_id": result.StepID,
                    "error":   err.Error(),
                })
            }
            continue
        }

        // Merge into sourceData (later steps may override earlier for same keys)
        for k, v := range parsed {
            sourceData[k] = v
        }
    }

    return sourceData
}
```

---

### 3. executor.go - Integration in executeStep

**Location**: Inside the `executeStep` retry loop, after the ErrorAnalyzer `!ShouldRetry` block ([executor.go:1463-1487](./executor.go#L1463-L1487))

**PREREQUISITE**: Add `previousErrors` initialization alongside existing `validationRetries` at line 1328:
```go
validationRetries := 0
previousErrors := []string{}  // ADD THIS LINE - tracks error history for semantic retry
```

**BEFORE** (current code at line ~1470):
```go
            } else if !analysisResult.ShouldRetry {
                // LLM determined this error is not fixable
                if e.logger != nil {
                    e.logger.Info("LLM error analysis: not retryable", map[string]interface{}{
                        "operation":   "error_analysis_no_retry",
                        "step_id":     step.StepID,
                        "capability":  capability,
                        "http_status": httpStatus,
                        "reason":      analysisResult.Reason,
                    })
                }

                telemetry.AddSpanEvent(ctx, "llm_error_analysis_no_retry",
                    attribute.String("step_id", step.StepID),
                    attribute.String("reason", analysisResult.Reason),
                )
                telemetry.Counter("orchestration.error_analysis.no_retry",
                    "capability", capability,
                    "http_status", fmt.Sprintf("%d", httpStatus),
                    "module", telemetry.ModuleOrchestration,
                )

                // Break out of retry loop - LLM says don't retry
                break
            }
```

**AFTER** (insert Layer 4 before the `break`):
```go
            } else if !analysisResult.ShouldRetry {
                // LLM determined this error is not fixable via simple retry
                if e.logger != nil {
                    e.logger.Info("LLM error analysis: not retryable", map[string]interface{}{
                        "operation":   "error_analysis_no_retry",
                        "step_id":     step.StepID,
                        "capability":  capability,
                        "http_status": httpStatus,
                        "reason":      analysisResult.Reason,
                    })
                }

                telemetry.AddSpanEvent(ctx, "llm_error_analysis_no_retry",
                    attribute.String("step_id", step.StepID),
                    attribute.String("reason", analysisResult.Reason),
                )
                telemetry.Counter("orchestration.error_analysis.no_retry",
                    "capability", capability,
                    "http_status", fmt.Sprintf("%d", httpStatus),
                    "module", telemetry.ModuleOrchestration,
                )

                // ============================================================
                // LAYER 4: Contextual Re-Resolution (Semantic Retry)
                // ErrorAnalyzer said "cannot fix" but it lacks source data.
                // Try ContextualReResolver which has BOTH error AND source data.
                // ============================================================
                if e.contextualReResolver != nil && validationRetries < e.maxSemanticRetries {
                    // Collect source data from dependencies (same data MicroResolver had)
                    sourceData := e.collectSourceDataFromDependencies(ctx, step.DependsOn)

                    if len(sourceData) > 0 {
                        // Get capability schema for parameter descriptions
                        // (capabilitySchema is already available from line 1237)
                        execCtx := &ExecutionContext{
                            UserQuery:       step.Instruction,
                            SourceData:      sourceData,
                            StepID:          step.StepID,
                            Capability:      capabilitySchema,  // Already retrieved at line 1237
                            AttemptedParams: parameters,
                            ErrorResponse:   responseBody,
                            HTTPStatus:      httpStatus,
                            RetryCount:      validationRetries,
                            PreviousErrors:  previousErrors,
                        }

                        reResult, reErr := e.contextualReResolver.ReResolve(ctx, execCtx)
                        if reErr == nil && reResult.ShouldRetry && len(reResult.CorrectedParameters) > 0 {
                            validationRetries++
                            previousErrors = append(previousErrors, responseBody)

                            // Telemetry: Record semantic retry
                            correctedJSON, _ := json.Marshal(reResult.CorrectedParameters)
                            telemetry.AddSpanEvent(ctx, "semantic_retry_applied",
                                attribute.String("step_id", step.StepID),
                                attribute.String("capability", capability),
                                attribute.String("analysis", reResult.Analysis),
                                attribute.String("corrected_params", string(correctedJSON)),
                            )
                            telemetry.Counter("orchestration.semantic_retry.applied",
                                "capability", capability,
                                "module", telemetry.ModuleOrchestration,
                            )

                            if e.logger != nil {
                                e.logger.Info("SEMANTIC RETRY: Applying corrected parameters", map[string]interface{}{
                                    "operation":             "semantic_retry_applied",
                                    "step_id":               step.StepID,
                                    "capability":            capability,
                                    "analysis":              reResult.Analysis,
                                    "corrected_parameters":  reResult.CorrectedParameters,
                                    "semantic_retry_count":  validationRetries,
                                })
                            }

                            // Replace parameters and retry
                            parameters = reResult.CorrectedParameters
                            attempt-- // Don't count as regular retry
                            continue  // Go back to HTTP call with new params
                        }

                        // Re-resolution failed or said don't retry
                        if reErr != nil && e.logger != nil {
                            e.logger.Warn("Semantic retry failed", map[string]interface{}{
                                "operation": "semantic_retry_failed",
                                "step_id":   step.StepID,
                                "error":     reErr.Error(),
                            })
                        }
                    }
                }
                // END LAYER 4

                // Break out of retry loop - neither ErrorAnalyzer nor ContextualReResolver could fix
                break
            }
```

---

### 4. interfaces.go - OrchestratorConfig Extension

**Location**: [interfaces.go:193-226](./interfaces.go#L193-L226)

**BEFORE** (end of OrchestratorConfig struct):
```go
    // Plan Parse Retry configuration
    PlanParseRetryEnabled bool `json:"plan_parse_retry_enabled"`
    PlanParseMaxRetries   int  `json:"plan_parse_max_retries"` // Default: 2
}
```

**AFTER** (add SemanticRetry before closing brace):
```go
    // Plan Parse Retry configuration
    PlanParseRetryEnabled bool `json:"plan_parse_retry_enabled"`
    PlanParseMaxRetries   int  `json:"plan_parse_max_retries"` // Default: 2

    // Layer 4: Semantic Retry Configuration
    // When enabled, uses ContextualReResolver to fix errors that require computation
    SemanticRetry SemanticRetryConfig `json:"semantic_retry,omitempty"`
}

// SemanticRetryConfig configures Layer 4 contextual re-resolution
type SemanticRetryConfig struct {
    // Enable contextual re-resolution on validation errors (default: true)
    Enabled bool `json:"enabled"`

    // Maximum semantic retry attempts per step (default: 2)
    MaxAttempts int `json:"max_attempts"`

    // HTTP status codes that trigger semantic retry in addition to ErrorAnalyzer
    // Default: [400, 422] - validation errors that might be fixable with different params
    TriggerStatusCodes []int `json:"trigger_status_codes,omitempty"`
}
```

---

### 5. interfaces.go - DefaultConfig Update

**Location**: Inside `DefaultConfig()` function ([interfaces.go:229-281](./interfaces.go#L229-L281))

**ADD** (before the closing `return config`):
```go
    // Layer 4: Semantic Retry defaults
    config.SemanticRetry = SemanticRetryConfig{
        Enabled:            true,
        MaxAttempts:        2,
        TriggerStatusCodes: []int{400, 422},
    }

    // Semantic Retry configuration from environment
    if enabled := os.Getenv("GOMIND_SEMANTIC_RETRY_ENABLED"); enabled != "" {
        config.SemanticRetry.Enabled = strings.ToLower(enabled) == "true"
    }
    if maxAttempts := os.Getenv("GOMIND_SEMANTIC_RETRY_MAX_ATTEMPTS"); maxAttempts != "" {
        if val, err := strconv.Atoi(maxAttempts); err == nil && val >= 0 {
            config.SemanticRetry.MaxAttempts = val
        }
    }
```

### 6. NEW FILE: contextual_re_resolver.go

**Location**: `orchestration/contextual_re_resolver.go` (new file)

**Complete Implementation**:

```go
// Package orchestration provides intelligent parameter binding for multi-step workflows.
//
// This file implements Layer 4: Contextual Re-Resolution for semantic retry.
// When ErrorAnalyzer says "cannot fix" but source data exists, this component
// can compute derived values using the full execution context.
package orchestration

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
    "go.opentelemetry.io/otel/attribute"
)

// ExecutionContext captures all information needed for semantic retry.
// This is the "full trajectory" that enables intelligent re-resolution.
type ExecutionContext struct {
    // Original user intent (critical for understanding what to compute)
    UserQuery string

    // All source data from dependent steps (what MicroResolver had)
    SourceData map[string]interface{}

    // Step being executed
    StepID     string
    Capability *EnhancedCapability

    // What we tried (failed parameters)
    AttemptedParams map[string]interface{}

    // What went wrong
    ErrorResponse string
    HTTPStatus    int

    // Retry state (memory across attempts)
    RetryCount     int
    PreviousErrors []string
}

// ReResolutionResult is returned by ContextualReResolver
type ReResolutionResult struct {
    // Should we retry with corrected parameters?
    ShouldRetry bool `json:"should_retry"`

    // Corrected parameters to use for retry
    CorrectedParameters map[string]interface{} `json:"corrected_parameters"`

    // Explanation of what was fixed (for logging/debugging)
    Analysis string `json:"analysis"`
}

// ContextualReResolver combines error analysis with parameter re-resolution.
// Unlike ErrorAnalyzer (which only analyzes), this component can PRESCRIBE fixes
// because it has access to the full execution context including source data.
type ContextualReResolver struct {
    aiClient core.AIClient
    logger   core.Logger
}

// NewContextualReResolver creates a new contextual re-resolver
func NewContextualReResolver(aiClient core.AIClient, logger core.Logger) *ContextualReResolver {
    r := &ContextualReResolver{
        aiClient: aiClient,
        logger:   logger,
    }
    return r
}

// ReResolve attempts to resolve parameters after an execution failure.
// It uses the full execution context to compute corrected parameters.
func (r *ContextualReResolver) ReResolve(
    ctx context.Context,
    execCtx *ExecutionContext,
) (*ReResolutionResult, error) {
    if execCtx == nil {
        return nil, fmt.Errorf("execution context is required")
    }

    if r.aiClient == nil {
        return &ReResolutionResult{
            ShouldRetry: false,
            Analysis:    "AI client not configured for semantic retry",
        }, nil
    }

    // Check context before expensive LLM operation
    if ctx.Err() != nil {
        return nil, ctx.Err()
    }

    // Build comprehensive prompt with ALL context
    prompt := r.buildReResolutionPrompt(execCtx)

    // Telemetry: Track re-resolution attempt
    telemetry.AddSpanEvent(ctx, "contextual_re_resolution.start",
        attribute.String("step_id", execCtx.StepID),
        attribute.String("capability", execCtx.Capability.Name),
        attribute.Int("retry_count", execCtx.RetryCount),
        attribute.Int("http_status", execCtx.HTTPStatus),
        attribute.Int("source_data_keys", len(execCtx.SourceData)),
    )

    r.logInfo("Starting contextual re-resolution", map[string]interface{}{
        "step_id":          execCtx.StepID,
        "capability":       execCtx.Capability.Name,
        "http_status":      execCtx.HTTPStatus,
        "source_data_keys": getMapKeys(execCtx.SourceData),
    })

    startTime := time.Now()

    // LLM generates corrected parameters with reasoning
    response, err := r.aiClient.GenerateResponse(ctx, prompt, &core.AIOptions{
        Temperature: 0.0,  // Deterministic for parameter extraction
        MaxTokens:   1000, // Allow space for reasoning and computation
    })

    duration := time.Since(startTime)

    // Record LLM latency
    telemetry.Histogram("orchestration.semantic_retry.llm_latency_ms",
        float64(duration.Milliseconds()),
        "capability", execCtx.Capability.Name,
        "module", telemetry.ModuleOrchestration,
    )

    if err != nil {
        telemetry.AddSpanEvent(ctx, "contextual_re_resolution.error",
            attribute.String("error", err.Error()),
            attribute.Int64("duration_ms", duration.Milliseconds()),
        )
        telemetry.Counter("orchestration.semantic_retry.llm_errors",
            "capability", execCtx.Capability.Name,
            "module", telemetry.ModuleOrchestration,
        )
        r.logWarn("Re-resolution LLM call failed", map[string]interface{}{
            "error":       err.Error(),
            "duration_ms": duration.Milliseconds(),
        })
        return nil, fmt.Errorf("re-resolution LLM call failed: %w", err)
    }

    // Parse structured response
    result, parseErr := r.parseReResolutionResponse(response.Content)
    if parseErr != nil {
        telemetry.AddSpanEvent(ctx, "contextual_re_resolution.parse_error",
            attribute.String("error", parseErr.Error()),
            attribute.String("response", truncateString(response.Content, 200)),
        )
        r.logWarn("Failed to parse re-resolution response", map[string]interface{}{
            "error":    parseErr.Error(),
            "response": truncateString(response.Content, 200),
        })
        return nil, fmt.Errorf("failed to parse re-resolution response: %w", parseErr)
    }

    // Telemetry: Track result
    telemetry.AddSpanEvent(ctx, "contextual_re_resolution.complete",
        attribute.Bool("should_retry", result.ShouldRetry),
        attribute.String("analysis", truncateString(result.Analysis, 200)),
        attribute.Int("corrected_params_count", len(result.CorrectedParameters)),
        attribute.Int64("duration_ms", duration.Milliseconds()),
    )

    if result.ShouldRetry {
        telemetry.Counter("orchestration.semantic_retry.success",
            "capability", execCtx.Capability.Name,
            "module", telemetry.ModuleOrchestration,
        )
    } else {
        telemetry.Counter("orchestration.semantic_retry.cannot_fix",
            "capability", execCtx.Capability.Name,
            "module", telemetry.ModuleOrchestration,
        )
    }

    r.logInfo("Contextual re-resolution completed", map[string]interface{}{
        "step_id":              execCtx.StepID,
        "capability":           execCtx.Capability.Name,
        "should_retry":         result.ShouldRetry,
        "analysis":             result.Analysis,
        "corrected_params":     result.CorrectedParameters,
        "duration_ms":          duration.Milliseconds(),
    })

    return result, nil
}

// buildReResolutionPrompt creates the domain-agnostic prompt for re-resolution.
// The framework provides ALL available context and lets the LLM figure out
// what computation (if any) is needed.
func (r *ContextualReResolver) buildReResolutionPrompt(execCtx *ExecutionContext) string {
    sourceJSON, _ := json.MarshalIndent(execCtx.SourceData, "", "  ")
    failedJSON, _ := json.MarshalIndent(execCtx.AttemptedParams, "", "  ")

    // Build parameter schema description
    var paramDescs []string
    for _, p := range execCtx.Capability.Parameters {
        required := ""
        if p.Required {
            required = " (required)"
        }
        paramDescs = append(paramDescs, fmt.Sprintf("- %s (%s%s): %s",
            p.Name, p.Type, required, p.Description))
    }

    // Include previous errors if this is a retry of a retry
    previousContext := ""
    if len(execCtx.PreviousErrors) > 0 {
        previousContext = fmt.Sprintf("\nPREVIOUS FAILED ATTEMPTS:\n%s\n",
            strings.Join(execCtx.PreviousErrors, "\n---\n"))
    }

    return fmt.Sprintf(`TASK: Re-resolve parameters after execution failure

USER REQUEST:
"%s"

SOURCE DATA FROM PREVIOUS STEPS:
%s

FAILED ATTEMPT:
- Capability: %s
- Parameters sent: %s
- Error received: "%s"
- HTTP Status: %d
%s
TARGET CAPABILITY SCHEMA:
%s

INSTRUCTIONS:
1. Analyze the error message to understand what went wrong
2. Look at the USER REQUEST to understand the intent
3. Look at the SOURCE DATA to find values that can fix the error
4. If the fix requires deriving a value (calculation, combination, transformation),
   perform it based on your understanding of the user's intent
5. Return corrected parameters that satisfy the capability schema

RESPONSE FORMAT (JSON only):
{
  "should_retry": true,
  "analysis": "what was wrong and how you determined the fix",
  "corrected_parameters": {
    "param1": value1,
    "param2": value2
  }
}

If the error cannot be fixed (e.g., missing required data in source), respond:
{
  "should_retry": false,
  "analysis": "explanation of why it cannot be fixed",
  "corrected_parameters": {}
}`,
        execCtx.UserQuery,
        string(sourceJSON),
        execCtx.Capability.Name,
        string(failedJSON),
        execCtx.ErrorResponse,
        execCtx.HTTPStatus,
        previousContext,
        strings.Join(paramDescs, "\n"),
    )
}

// parseReResolutionResponse parses the LLM's JSON response.
// Uses the same pattern as ErrorAnalyzer.parseAnalysisResponse.
func (r *ContextualReResolver) parseReResolutionResponse(content string) (*ReResolutionResult, error) {
    // Clean up the response (handle markdown, extra text, etc.)
    content = strings.TrimSpace(content)

    // Remove markdown code blocks if present
    content = strings.TrimPrefix(content, "```json")
    content = strings.TrimPrefix(content, "```")
    content = strings.TrimSuffix(content, "```")
    content = strings.TrimSpace(content)

    // Find JSON object (same logic as error_analyzer.go:328-337)
    jsonStart := strings.Index(content, "{")
    if jsonStart == -1 {
        return nil, fmt.Errorf("no JSON object found in response")
    }

    jsonEnd := findJSONEndSimple(content, jsonStart) // Reuse from error_analyzer.go
    if jsonEnd == -1 {
        return nil, fmt.Errorf("invalid JSON structure in response")
    }

    jsonStr := content[jsonStart:jsonEnd]

    var result ReResolutionResult
    if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
        return nil, fmt.Errorf("failed to parse JSON: %w", err)
    }

    // Initialize empty map if nil
    if result.CorrectedParameters == nil {
        result.CorrectedParameters = make(map[string]interface{})
    }

    return &result, nil
}

// SetLogger sets the logger for the contextual re-resolver.
// The component is always set to "framework/orchestration" for proper attribution.
func (r *ContextualReResolver) SetLogger(logger core.Logger) {
    if logger == nil {
        r.logger = nil
    } else {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            r.logger = cal.WithComponent("framework/orchestration")
        } else {
            r.logger = logger
        }
    }
}

// Logging helpers
func (r *ContextualReResolver) logInfo(msg string, fields map[string]interface{}) {
    if r.logger != nil {
        r.logger.Info(msg, fields)
    }
}

func (r *ContextualReResolver) logWarn(msg string, fields map[string]interface{}) {
    if r.logger != nil {
        r.logger.Warn(msg, fields)
    }
}
```

---

## Configuration

### Environment Variables

```bash
# Enable/disable semantic retry (default: true)
GOMIND_SEMANTIC_RETRY_ENABLED=true

# Maximum semantic retry attempts (default: 2)
GOMIND_SEMANTIC_RETRY_MAX_ATTEMPTS=2
```

### OrchestratorConfig (already defined in Section 4-5 above)

The `SemanticRetryConfig` struct and `DefaultConfig()` updates are shown in Section 4 and 5 of Integration Points.

---

## Telemetry & Observability

This section documents the comprehensive telemetry emitted by Layer 4 (Semantic Retry). The implementation follows the same patterns established in:
- [DISTRIBUTED_TRACING_GUIDE.md](../docs/DISTRIBUTED_TRACING_GUIDE.md) - Span events and trace structure
- [LOGGING_IMPLEMENTATION_GUIDE.md](../docs/LOGGING_IMPLEMENTATION_GUIDE.md) - Context-aware logging
- [error_analyzer.go](./error_analyzer.go) - LLM telemetry patterns

### Span Events

The `ContextualReResolver` emits span events that appear in the Jaeger **Logs** tab:

| Event Name | Description | Key Attributes |
|------------|-------------|----------------|
| `contextual_re_resolution.start` | Semantic retry begins | `step_id`, `capability`, `retry_count`, `http_status`, `source_data_keys` |
| `contextual_re_resolution.error` | LLM call failed | `error`, `duration_ms` |
| `contextual_re_resolution.parse_error` | Failed to parse LLM response | `error`, `response` (truncated) |
| `contextual_re_resolution.complete` | Semantic retry finished | `should_retry`, `analysis`, `corrected_params_count`, `duration_ms` |
| `semantic_retry_applied` | Executor applies corrected parameters | `step_id`, `capability`, `analysis`, `corrected_params` |

### Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `orchestration.semantic_retry.llm_latency_ms` | Histogram | `capability`, `module` | LLM call duration for re-resolution |
| `orchestration.semantic_retry.success` | Counter | `capability`, `module` | Re-resolution determined fixable |
| `orchestration.semantic_retry.cannot_fix` | Counter | `capability`, `module` | Re-resolution determined not fixable |
| `orchestration.semantic_retry.llm_errors` | Counter | `capability`, `module` | LLM call failures |
| `orchestration.semantic_retry.applied` | Counter | `capability`, `module` | Corrected parameters applied |

### What You'll See in Jaeger

When you click on an orchestration span and expand the **Logs** tab, semantic retry interactions appear with full context:

**Scenario: Stock Sale → Currency Conversion (amount calculation needed)**

```
▼ error_analyzer.llm_call_start
  capability: "convert_currency"
  http_status: 400

▼ error_analyzer.analysis_complete
  should_retry: false
  reason: "Cannot be fixed by modifying request parameters..."
  suggested_changes_count: 0
  total_duration_ms: 342

▼ contextual_re_resolution.start                           ← LAYER 4 BEGINS
  step_id: "step-5-convert_currency"
  capability: "convert_currency"
  retry_count: 0
  http_status: 400
  source_data_keys: 5

▼ contextual_re_resolution.complete                        ← LAYER 4 RESULT
  should_retry: true
  analysis: "The user wants to sell 100 Tesla shares at $468.285 per share.
             The amount should be 100 × 468.285 = 46828.5 USD"
  corrected_params_count: 3
  duration_ms: 1247

▼ semantic_retry_applied                                   ← EXECUTOR ACTS
  step_id: "step-5-convert_currency"
  capability: "convert_currency"
  analysis: "The user wants to sell 100 Tesla shares..."
  corrected_params: {"from":"USD","to":"KRW","amount":46828.5}
```

**Failure Scenario: Cannot fix (missing source data)**

```
▼ contextual_re_resolution.start
  step_id: "step-2-get_stock_price"
  capability: "get_stock_price"
  retry_count: 0
  http_status: 400
  source_data_keys: 0                                      ← NO SOURCE DATA

▼ contextual_re_resolution.complete
  should_retry: false                                      ← CANNOT FIX
  analysis: "Cannot compute stock symbol from user query alone..."
  corrected_params_count: 0
  duration_ms: 523
```

### Visual Trace Structure

The complete trace shows the semantic retry flow in context:

```
travel-research-agent                                                    [3.2s]
└─ POST /api/capabilities/research_topic                                 [3.1s]
   ├─ orchestration.process_request                                      [3.0s]
   │  ├─ llm.plan_generation.request
   │  ├─ llm.plan_generation.response                         [412ms]
   │  │
   │  ├─ orchestration.step_execution (step-1: get_stock_price)
   │  │  └─ http.client.request → stock-tool                  [200 OK]
   │  │
   │  ├─ orchestration.step_execution (step-5: convert_currency)
   │  │  ├─ http.client.request → currency-tool               [400 Bad Request]
   │  │  │  └─ error: "amount must be greater than 0"
   │  │  │
   │  │  ├─ error_analyzer.llm_call_start
   │  │  ├─ error_analyzer.analysis_complete                  [342ms]
   │  │  │  └─ should_retry: false  ← Layer 3 says "cannot fix"
   │  │  │
   │  │  ├─ contextual_re_resolution.start                    ← LAYER 4
   │  │  ├─ contextual_re_resolution.complete                 [1247ms]
   │  │  │  └─ should_retry: true   ← Layer 4 CAN fix it!
   │  │  │
   │  │  ├─ semantic_retry_applied
   │  │  │  └─ corrected_params: {amount: 46828.5}
   │  │  │
   │  │  └─ http.client.request (retry) → currency-tool       [200 OK] ✓
   │  │
   │  └─ llm.synthesis.response                               [523ms]
   └─ HTTP 200 OK
```

### Context-Aware Logging

The `ContextualReResolver` uses context-aware logging for trace correlation:

```go
// All log methods support trace correlation via context
r.logInfo("Starting contextual re-resolution", map[string]interface{}{
    "step_id":          execCtx.StepID,
    "capability":       execCtx.Capability.Name,
    "http_status":      execCtx.HTTPStatus,
    "source_data_keys": getMapKeys(execCtx.SourceData),
})

r.logWarn("Re-resolution LLM call failed", map[string]interface{}{
    "error":       err.Error(),
    "duration_ms": duration.Milliseconds(),
})

r.logInfo("Contextual re-resolution completed", map[string]interface{}{
    "step_id":          execCtx.StepID,
    "capability":       execCtx.Capability.Name,
    "should_retry":     result.ShouldRetry,
    "analysis":         result.Analysis,
    "corrected_params": result.CorrectedParameters,
    "duration_ms":      duration.Milliseconds(),
})
```

The logger is configured with component prefix `framework/orchestration` (matching [error_analyzer.go:401](./error_analyzer.go#L401)) for consistent log filtering:

```bash
# Filter logs to see semantic retry activity
kubectl logs deploy/travel-research-agent | grep "framework/orchestration"
```

### Debugging with Telemetry

| What You Want to Know | Where to Look |
|----------------------|---------------|
| Did semantic retry trigger? | `contextual_re_resolution.start` event in Jaeger |
| What source data was available? | `source_data_keys` attribute (count), logs show actual keys |
| What did the LLM compute? | `analysis` attribute in `contextual_re_resolution.complete` |
| What parameters were corrected? | `corrected_params` in `semantic_retry_applied` event |
| How long did re-resolution take? | `duration_ms` in completion event, or histogram metric |
| Why didn't it fix the error? | `should_retry: false` with `analysis` explaining why |

### Prometheus Queries

```promql
# Semantic retry success rate
sum(rate(orchestration_semantic_retry_success_total[5m]))
/ sum(rate(orchestration_semantic_retry_success_total[5m]) + rate(orchestration_semantic_retry_cannot_fix_total[5m]))

# P99 LLM latency for semantic retry
histogram_quantile(0.99, rate(orchestration_semantic_retry_llm_latency_ms_bucket[5m]))

# Semantic retry error rate
rate(orchestration_semantic_retry_llm_errors_total[5m])
```

---

## Testing Strategy

### Test File Location

`orchestration/contextual_re_resolver_test.go`

### Unit Tests

```go
package orchestration

import (
    "context"
    "testing"

    "github.com/stretchr/testify/require"
)

// TestContextualReResolver_NilContext returns error for nil context
func TestContextualReResolver_NilContext(t *testing.T) {
    resolver := NewContextualReResolver(nil, nil)
    result, err := resolver.ReResolve(context.Background(), nil)

    require.Error(t, err)
    require.Nil(t, result)
    require.Contains(t, err.Error(), "execution context is required")
}

// TestContextualReResolver_NilAIClient returns non-retry result
func TestContextualReResolver_NilAIClient(t *testing.T) {
    resolver := NewContextualReResolver(nil, nil) // No AI client

    execCtx := &ExecutionContext{
        UserQuery:       "test query",
        SourceData:      map[string]interface{}{"key": "value"},
        StepID:          "step-1",
        Capability:      &EnhancedCapability{Name: "test_cap"},
        AttemptedParams: map[string]interface{}{},
        ErrorResponse:   "error",
        HTTPStatus:      400,
    }

    result, err := resolver.ReResolve(context.Background(), execCtx)

    require.NoError(t, err)
    require.False(t, result.ShouldRetry)
    require.Contains(t, result.Analysis, "AI client not configured")
}

// TestContextualReResolver_ComputationRequired tests LLM computing derived values
func TestContextualReResolver_ComputationRequired(t *testing.T) {
    // Uses mock AI client that returns expected computation
    mockAI := &mockAIClient{
        response: `{
            "should_retry": true,
            "analysis": "User wants 100 shares at $468.285, so amount = 100 × 468.285 = 46828.5",
            "corrected_parameters": {"from": "USD", "to": "KRW", "amount": 46828.5}
        }`,
    }

    resolver := NewContextualReResolver(mockAI, nil)

    execCtx := &ExecutionContext{
        UserQuery: "sell 100 Tesla shares",
        SourceData: map[string]interface{}{
            "symbol":        "TSLA",
            "current_price": 468.285,
        },
        StepID: "step-5",
        Capability: &EnhancedCapability{
            Name: "convert_currency",
            Parameters: []Parameter{
                {Name: "from", Type: "string", Required: true},
                {Name: "to", Type: "string", Required: true},
                {Name: "amount", Type: "number", Required: true},
            },
        },
        AttemptedParams: map[string]interface{}{
            "from":   "USD",
            "to":     "KRW",
            "amount": 0,
        },
        ErrorResponse: `{"error": "amount must be greater than 0"}`,
        HTTPStatus:    400,
    }

    result, err := resolver.ReResolve(context.Background(), execCtx)

    require.NoError(t, err)
    require.True(t, result.ShouldRetry)
    require.InDelta(t, 46828.5, result.CorrectedParameters["amount"], 0.01)
    require.Contains(t, result.Analysis, "46828")
}

// TestContextualReResolver_TypeCoercion tests fixing type mismatches
func TestContextualReResolver_TypeCoercion(t *testing.T) {
    mockAI := &mockAIClient{
        response: `{
            "should_retry": true,
            "analysis": "lat was sent as string, converting to number",
            "corrected_parameters": {"lat": 35.6762, "lon": 139.6503}
        }`,
    }

    resolver := NewContextualReResolver(mockAI, nil)

    execCtx := &ExecutionContext{
        UserQuery: "get weather in Tokyo",
        SourceData: map[string]interface{}{
            "latitude":  35.6762,
            "longitude": 139.6503,
        },
        StepID: "step-2",
        Capability: &EnhancedCapability{
            Name: "get_weather",
            Parameters: []Parameter{
                {Name: "lat", Type: "number", Required: true},
                {Name: "lon", Type: "number", Required: true},
            },
        },
        AttemptedParams: map[string]interface{}{
            "lat": "35.6762", // String instead of number
            "lon": "139.6503",
        },
        ErrorResponse: "lat must be a number",
        HTTPStatus:    400,
    }

    result, err := resolver.ReResolve(context.Background(), execCtx)

    require.NoError(t, err)
    require.True(t, result.ShouldRetry)
    require.Equal(t, 35.6762, result.CorrectedParameters["lat"])
}

// TestContextualReResolver_CannotFix tests unfixable errors
func TestContextualReResolver_CannotFix(t *testing.T) {
    mockAI := &mockAIClient{
        response: `{
            "should_retry": false,
            "analysis": "No stock symbol available in source data",
            "corrected_parameters": {}
        }`,
    }

    resolver := NewContextualReResolver(mockAI, nil)

    execCtx := &ExecutionContext{
        UserQuery:  "get stock price",
        SourceData: map[string]interface{}{}, // Empty - no symbol available
        StepID:     "step-1",
        Capability: &EnhancedCapability{
            Name: "get_stock_quote",
            Parameters: []Parameter{
                {Name: "symbol", Type: "string", Required: true},
            },
        },
        AttemptedParams: map[string]interface{}{"symbol": ""},
        ErrorResponse:   "symbol is required",
        HTTPStatus:      400,
    }

    result, err := resolver.ReResolve(context.Background(), execCtx)

    require.NoError(t, err)
    require.False(t, result.ShouldRetry)
    require.Contains(t, result.Analysis, "No stock symbol")
}

// mockAIClient for testing (same pattern as error_analyzer_test.go)
type mockAIClient struct {
    response string
    err      error
}

func (m *mockAIClient) GenerateResponse(ctx context.Context, prompt string, opts *core.AIOptions) (*core.AIResponse, error) {
    if m.err != nil {
        return nil, m.err
    }
    return &core.AIResponse{Content: m.response}, nil
}
```

### Integration Tests

```go
// TestSemanticRetry_EndToEnd tests full orchestration with Layer 4
func TestSemanticRetry_EndToEnd(t *testing.T) {
    // Setup orchestrator with all layers enabled
    config := DefaultConfig()
    config.SemanticRetry.Enabled = true
    config.SemanticRetry.MaxAttempts = 2

    orchestrator := setupTestOrchestrator(t, config)

    // Request that will trigger semantic retry
    request := "I want to sell 100 shares of TSLA and convert to Korean Won"

    response, err := orchestrator.ProcessRequest(context.Background(), request, nil)

    require.NoError(t, err)
    require.True(t, response.Success)

    // Verify semantic retry telemetry was recorded
    // (Check Jaeger traces or metrics depending on test infrastructure)
}
```

---

## Migration Path

### Phase 1: Add ContextualReResolver (Non-Breaking)

**Files to create:**
| File | Description |
|------|-------------|
| `orchestration/contextual_re_resolver.go` | See Section 6 of Integration Points |
| `orchestration/contextual_re_resolver_test.go` | Unit tests (see Testing Strategy) |

**Files to modify:**
| File | Changes | Reference |
|------|---------|-----------|
| `orchestration/executor.go` | Add struct fields, setter methods, integration code | Sections 1-3 of Integration Points |
| `orchestration/interfaces.go` | Add `SemanticRetryConfig`, update `DefaultConfig()` | Sections 4-5 of Integration Points |

**No existing behavior changes** - Layer 4 is triggered only when:
1. ErrorAnalyzer is enabled AND returns `ShouldRetry: false`
2. ContextualReResolver is configured
3. Source data exists from dependencies

### Phase 2: Enable by Default

After validation in production:

1. `SemanticRetryConfig.Enabled` defaults to `true` in `DefaultConfig()` (already set in Section 5)
2. Update `INTELLIGENT_PARAMETER_BINDING.md` to mark Layer 4 as active
3. Update `ENVIRONMENT_VARIABLES_GUIDE.md` with new env vars

### Phase 3: Deprecate Redundant Code (Future)

Once semantic retry proves reliable:

1. **CorrectionCallback** ([executor.go:42-50](./executor.go#L42-L50)): Consider deprecation since ContextualReResolver handles type errors too
2. **ErrorAnalyzer retry suggestions**: Simplify to analysis-only (remove `SuggestedChanges` field)

---

## Appendix: Domain-Agnostic Examples

The semantic retry mechanism works across any domain:

| Domain | User Query | Source Data | Failed Param | Computed Fix |
|--------|-----------|-------------|--------------|--------------|
| **Finance** | "sell 100 shares" | `{price: 468.29}` | `amount: 0` | `100 × 468.29 = 46829` |
| **Shipping** | "ship all items" | `{items: [{wt: 2.5}, {wt: 1.2}]}` | `weight: 0` | `2.5 + 1.2 = 3.7` |
| **Scheduling** | "meeting 9am-2pm" | `{start: "09:00", end: "14:00"}` | `duration: 0` | `14 - 9 = 5 hours` |
| **E-commerce** | "buy 3 at $19.99" | `{unit_price: 19.99, qty: 3}` | `total: 0` | `3 × 19.99 = 59.97` |
| **Travel** | "2 adults, 1 child" | `{adult_price: 100, child_price: 50}` | `cost: 0` | `2×100 + 1×50 = 250` |

The framework handles all these cases with the same mechanism because the LLM understands the semantic relationship between user intent, source data, and required parameters.

---

## References

1. [Reflexion: Language Agents with Verbal Reinforcement Learning](https://arxiv.org/pdf/2303.11366) - Shinn et al., 2023
2. [Self-Reflection in LLM Agents](https://arxiv.org/pdf/2405.06682) - 2024
3. [Are Retrials All You Need?](https://arxiv.org/html/2504.12951) - 2025
4. [Anthropic: Building Effective Agents](https://www.anthropic.com/research/building-effective-agents)
5. [Rasa: Process Calling](https://rasa.com/blog/process-calling-agentic-tools-need-state)
6. [Error Recovery in AI Agent Development](https://www.gocodeo.com/post/error-recovery-and-fallback-strategies-in-ai-agent-development)
