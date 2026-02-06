# Bug Report: LLM Hallucinated Non-Existent Tool

**Date:** 2026-02-02
**Request ID:** `orch-1769993253809626971`
**Agent:** travel-chat-agent
**Severity:** High
**Status:** ✅ Fixed & Verified (2026-02-05)
**Verification Request ID:** `orch-1770268232220395625`

---

## Summary

The plan generation LLM (gpt-4o-mini) hallucinated a non-existent tool `time-tool-v1` with capability `get_current_time`, despite being explicitly told that only `weather-tool-v2` was available in the filtered tool list.

## User Request

```
what is the time right in CST
```

## Expected Behavior

The orchestrator should either:
1. Return an error indicating no tool can fulfill this request
2. Use `research-agent-telemetry/research_topic` to answer from AI knowledge
3. Return empty plan and explain that time queries are not supported

## Actual Behavior

1. Tiered selection returned `["weather-tool-v2/get_current_weather"]` (incorrect but expected given no time tool exists)
2. Plan generation was given ONLY `weather-tool-v2` in the prompt
3. LLM **hallucinated** `time-tool-v1` agent with `get_current_time` capability
4. Plan validation should have caught this but didn't (or was bypassed)
5. Execution failed with: `"agent time-tool-v1 not found in catalog"`

---

## Timeline of Events

| Step | Component | What Happened | Issue |
|------|-----------|---------------|-------|
| 1 | Tiered Selection (396ms) | Selected `weather-tool-v2` | Wrong tool - no time tool exists |
| 2 | Plan Generation (2560ms) | Prompt showed ONLY `weather-tool-v2` | Correct filtering |
| 3 | LLM Response | Output `time-tool-v1` | **LLM hallucination** |
| 4 | Validation | Should have caught invalid agent | Either failed or bypassed |
| 5 | Execution (0.2ms) | Failed: "agent not found" | Correctly rejected |
| 6 | Synthesis (2466ms) | Generated fallback response | Graceful degradation |

---

## Root Cause Analysis

### Primary Cause: LLM Hallucination

The plan generation prompt (from `llm_interactions[1]`) clearly stated:

```
Available Agents and Capabilities:

Agent: weather-tool-v2 (ID: weather-tool-v2)
  Address: http://weather-tool-v2-service.gomind-examples.svc.cluster.local:80
  - Capability: get_current_weather
    Description: Gets current weather conditions for a location...
```

Yet the LLM responded with:

```json
{
  "agent_name": "time-tool-v1",
  "metadata": {
    "capability": "get_current_time",
    "parameters": {
      "timezone": "CST"
    }
  }
}
```

The model "knew" what a time tool should look like from its training data and invented one, ignoring the explicit constraint that only `weather-tool-v2` was available.

### Contributing Factor 1: Tiered Selection Mismatch

When asked about time, the tiered selector returned `weather-tool-v2` instead of returning an empty array (indicating no tool can fulfill the request). The selector prompt should recognize when NO available tool matches the request.

### Contributing Factor 2: Existing Validation is Registry-Based, Not Prompt-Based

The existing `validatePlan()` at line 2044 checks if agents exist in the **registry** via discovery:

```go
// orchestrator.go:2060
agents, err := o.discovery.FindService(context.Background(), step.AgentName)
if err != nil || len(agents) == 0 {
    return fmt.Errorf("agent %s not found", step.AgentName)
}
```

**Key distinction:**
- `validatePlan()`: Checks if agent exists in the **full registry**
- Fix 1's validation: Checks if agent was in the **filtered list provided to LLM**

The error `"agent time-tool-v1 not found in catalog"` came from the **executor**, not validation. This means either:
1. `validatePlan()` was bypassed in some code path
2. Discovery returned an unexpected result
3. Validation failed but `regeneratePlan()` also produced an invalid plan

**Why Fix 1 is still needed:** Even if `validatePlan()` works correctly, Fix 1 adds defense-in-depth by validating against the **exact agent list** shown to the LLM, not just registry existence. This catches hallucinations earlier and provides better error messages.

---

## Evidence from Debug Payload

### LLM Interaction 1: Tiered Selection
```json
{
  "type": "tiered_selection",
  "prompt": "... list of 40+ tools ...",
  "response": "[\"weather-tool-v2/get_current_weather\"]"
}
```

### LLM Interaction 2: Plan Generation
```json
{
  "type": "plan_generation",
  "prompt": "Available Agents: weather-tool-v2 only",
  "response": "{ agent_name: 'time-tool-v1' }"  // HALLUCINATED
}
```

### Execution Result
```json
{
  "success": false,
  "error": "agent time-tool-v1 not found in catalog"
}
```

---

## Recommended Fixes

### Fix 1: Post-Parse Plan Validation Against Filtered Tools (P0 - Critical)

Add immediate validation after parsing the LLM response to check agent names against the filtered capability list that was provided to the LLM. When validation fails, **trigger plan regeneration with explicit error feedback** to give the LLM a chance to self-correct.

#### Implementation Challenge

The current code flow doesn't track which agents were in the filtered capability info:

```
buildPlanningPrompt(ctx, request)
         │
         ├── capabilityProvider.GetCapabilities(ctx, request, nil)
         │         │
         │         └── Returns: string (formatted capability info)
         │                      ↑ Agent names embedded in string, not structured
         │
         └── Returns: string (full prompt)
```

**Solution: Modify `buildPlanningPrompt()` to also return allowed agents**

#### Step 1: Update buildPlanningPrompt Signature

```go
// orchestrator.go - Change return type to include allowed agents
type PlanningPromptResult struct {
    Prompt        string
    AllowedAgents map[string]bool  // Agent names that were included in the prompt
}

func (o *AIOrchestrator) buildPlanningPrompt(ctx context.Context, request string) (*PlanningPromptResult, error) {
    // ... existing capability provider call ...
    capabilityInfo, err := o.capabilityProvider.GetCapabilities(ctx, request, nil)

    // Extract agent names from capability info (parse "Agent: xxx (ID: yyy)" lines)
    allowedAgents := extractAgentNamesFromCapabilityInfo(capabilityInfo)

    // ... build prompt ...

    return &PlanningPromptResult{
        Prompt:        prompt,
        AllowedAgents: allowedAgents,
    }, nil
}

// Helper to parse agent names from formatted capability info
func extractAgentNamesFromCapabilityInfo(capabilityInfo string) map[string]bool {
    agents := make(map[string]bool)
    // Parse lines like "Agent: weather-tool-v2 (ID: weather-tool-v2)"
    re := regexp.MustCompile(`Agent:\s+(\S+)\s+\(ID:`)
    matches := re.FindAllStringSubmatch(capabilityInfo, -1)
    for _, match := range matches {
        if len(match) > 1 {
            agents[match[1]] = true
        }
    }
    return agents
}
```

#### Step 2: Validation Function

```go
// orchestrator.go - New validation function
func (o *AIOrchestrator) validatePlanAgainstAllowedAgents(plan *RoutingPlan, allowedAgents map[string]bool) (string, error) {
    for _, step := range plan.Steps {
        if !allowedAgents[step.AgentName] {
            return step.AgentName, fmt.Errorf("LLM hallucinated agent '%s' not in allowed list", step.AgentName)
        }
    }
    return "", nil
}
```

#### Step 3: Add Configuration (similar to PlanParseRetry)

```go
// interfaces.go - Add to OrchestratorConfig struct (around line 313)

// Hallucination Retry configuration
// When enabled, retries LLM plan generation if hallucinated agents are detected.
// This handles cases where the LLM invents agent names not in the allowed list.
HallucinationRetryEnabled bool `json:"hallucination_retry_enabled"` // Default: true
HallucinationMaxRetries   int  `json:"hallucination_max_retries"`   // Default: 1
```

#### Step 4: Integrate into generateExecutionPlan

This code follows all required patterns from:
- **DISTRIBUTED_TRACING_GUIDE.md Section 11** (6 tracing patterns)
- **LOGGING_IMPLEMENTATION_GUIDE.md Section 11** (3 logging patterns)

```go
// In generateExecutionPlan() - around line 1765 after parsePlan succeeds
plan, parseErr := o.parsePlan(aiResponse.Content)
if parseErr == nil {
    // NEW: Validate against allowed agents before returning
    hallStartTime := time.Now()  // Track duration for logging
    hallucinatedAgent, hallErr := o.validatePlanAgainstAllowedAgents(plan, promptResult.AllowedAgents)
    if hallErr != nil {
        // Determine max hallucination retries
        maxHallRetries := 1 // default
        if o.config != nil && o.config.HallucinationRetryEnabled {
            maxHallRetries = o.config.HallucinationMaxRetries
        }

        allowedList := make([]string, 0, len(promptResult.AllowedAgents))
        for name := range promptResult.AllowedAgents {
            allowedList = append(allowedList, name)
        }

        // Retry loop for hallucination
        for hallRetry := 0; hallRetry < maxHallRetries; hallRetry++ {
            retryStartTime := time.Now()  // Track per-retry duration

            // Pattern 4 (Tracing): Record error on span FIRST (visible in Jaeger)
            telemetry.RecordSpanError(ctx, hallErr)

            // Pattern 6 (Tracing): Span event with request_id as FIRST attribute
            telemetry.AddSpanEvent(ctx, "llm.hallucination_detected",
                attribute.String("request_id", requestID),  // FIRST attribute per Pattern 6
                attribute.String("hallucinated_agent", hallucinatedAgent),
                attribute.Int("allowed_agent_count", len(allowedList)),
                attribute.Int("hall_retry", hallRetry+1),
                attribute.Int("max_hall_retries", maxHallRetries),
                attribute.Int("attempt", attempt),
            )

            // Pattern 5 (Tracing): Counter with module label
            telemetry.Counter("plan_generation.hallucinations",
                "module", telemetry.ModuleOrchestration,  // REQUIRED per Pattern 5
                "agent", hallucinatedAgent,
            )

            // Logging Patterns 1, 2, 3: nil check + operation + request_id
            if o.logger != nil {
                o.logger.WarnWithContext(ctx, "LLM hallucinated non-existent agent", map[string]interface{}{
                    "operation":          "hallucination_detection",  // REQUIRED: Pattern 2
                    "request_id":         requestID,                  // REQUIRED: Pattern 3
                    "hallucinated_agent": hallucinatedAgent,
                    "allowed_agents":     allowedList,
                    "attempt":            attempt,
                    "hall_retry":         hallRetry + 1,
                    "max_hall_retries":   maxHallRetries,
                    "error":              hallErr.Error(),            // REQUIRED: error field for warn/error logs
                })
            }

            // LLM Debug Store: Record hallucination for production debugging (per ARCHITECTURE.md §9.9)
            if o.debugStore != nil {
                o.debugStore.RecordAsync(ctx, &LLMInteraction{
                    RequestID:   requestID,
                    Type:        "hallucination_detection",
                    Request:     promptResult.Prompt,
                    Response:    aiResponse.Content,
                    Error:       hallErr.Error(),
                    Metadata: map[string]interface{}{
                        "hallucinated_agent": hallucinatedAgent,
                        "allowed_agents":     allowedList,
                        "hall_retry":         hallRetry + 1,
                    },
                })
            }

            // Retry with explicit feedback
            plan, err = o.regeneratePlan(ctx, request, requestID,
                fmt.Errorf("INVALID PLAN: You used agent '%s' which does NOT exist. "+
                    "You MUST ONLY use agents from: %v",
                    hallucinatedAgent, allowedList))
            if err != nil {
                // Pattern 4 (Tracing): Record regeneration error on span
                telemetry.RecordSpanError(ctx, err)
                telemetry.AddSpanEvent(ctx, "llm.hallucination_regeneration_failed",
                    attribute.String("request_id", requestID),
                    attribute.String("error", err.Error()),
                    attribute.Int("hall_retry", hallRetry+1),
                )

                // Logging: Log regeneration failure with error field
                if o.logger != nil {
                    o.logger.ErrorWithContext(ctx, "Plan regeneration failed during hallucination retry", map[string]interface{}{
                        "operation":   "hallucination_detection",
                        "request_id":  requestID,
                        "hall_retry":  hallRetry + 1,
                        "error":       err.Error(),  // REQUIRED: error field
                        "duration_ms": time.Since(retryStartTime).Milliseconds(),
                    })
                }
                return nil, fmt.Errorf("plan regeneration failed: %w", err)
            }

            // Record successful regeneration attempt
            telemetry.AddSpanEvent(ctx, "llm.hallucination_regeneration_complete",
                attribute.String("request_id", requestID),
                attribute.Int("hall_retry", hallRetry+1),
            )

            // Validate the regenerated plan
            hallucinatedAgent, hallErr = o.validatePlanAgainstAllowedAgents(plan, promptResult.AllowedAgents)
            if hallErr == nil {
                // Pattern 5 (Tracing): Counter for successful recovery
                telemetry.Counter("plan_generation.hallucination_recovered",
                    "module", telemetry.ModuleOrchestration,
                    "retries_used", strconv.Itoa(hallRetry+1),
                )
                telemetry.AddSpanEvent(ctx, "llm.hallucination_recovered",
                    attribute.String("request_id", requestID),
                    attribute.Int("retries_used", hallRetry+1),
                )

                // Logging: Log successful recovery
                if o.logger != nil {
                    o.logger.InfoWithContext(ctx, "LLM hallucination recovered after retry", map[string]interface{}{
                        "operation":    "hallucination_detection",
                        "request_id":   requestID,
                        "retries_used": hallRetry + 1,
                        "duration_ms":  time.Since(hallStartTime).Milliseconds(),
                        "status":       "recovered",
                    })
                }
                break // Success!
            }
        }

        // Check if we exhausted retries
        if hallErr != nil {
            // Actionable error message per FRAMEWORK_DESIGN_PRINCIPLES.md
            finalErr := fmt.Errorf("LLM hallucinated agent '%s' after %d retries: %w "+
                "(verify the agent is registered in the discovery system and included in tiered selection)",
                hallucinatedAgent, maxHallRetries, hallErr)

            // Pattern 4 (Tracing): Record final error on span
            telemetry.RecordSpanError(ctx, finalErr)
            telemetry.AddSpanEvent(ctx, "llm.hallucination_unrecoverable",
                attribute.String("request_id", requestID),
                attribute.String("hallucinated_agent", hallucinatedAgent),
                attribute.Int("retries_exhausted", maxHallRetries),
            )

            // Pattern 5 (Tracing): Counter for unrecoverable hallucinations
            telemetry.Counter("plan_generation.hallucination_unrecoverable",
                "module", telemetry.ModuleOrchestration,
                "agent", hallucinatedAgent,
            )

            // Logging Patterns 1, 2, 3 + error field + duration_ms
            if o.logger != nil {
                o.logger.ErrorWithContext(ctx, "LLM hallucination unrecoverable after retries", map[string]interface{}{
                    "operation":          "hallucination_detection",  // REQUIRED: Pattern 2
                    "request_id":         requestID,                  // REQUIRED: Pattern 3
                    "hallucinated_agent": hallucinatedAgent,
                    "retries_exhausted":  maxHallRetries,
                    "error":              hallErr.Error(),            // REQUIRED: error field
                    "duration_ms":        time.Since(hallStartTime).Milliseconds(),
                    "status":             "unrecoverable",
                })
            }

            return nil, finalErr
        }
    }

    // Success - return the validated plan
    return plan, nil
}
```

#### Why This Works

1. **Catches ALL hallucinations**: Validates against the exact list of agents the LLM was told about
2. **Consistent logging**: Uses existing `o.logger.WarnWithContext()` pattern
3. **Retry with feedback**: Uses existing `regeneratePlan()` which appends error context to prompt
4. **Metrics**: Tracks hallucination rate for monitoring
5. **Fail-safe**: If retry also hallucinates, returns clear error

#### Distributed Tracing Compliance (DISTRIBUTED_TRACING_GUIDE.md Section 11)

| Pattern | Requirement | Implementation |
|---------|-------------|----------------|
| **Pattern 1** | Logger nil check | `if o.logger != nil { ... }` |
| **Pattern 2** | `operation` field in every log | `"operation": "hallucination_detection"` |
| **Pattern 3** | request_id propagation | `requestID` already in context via `generateRequestID()` |
| **Pattern 4** | `telemetry.RecordSpanError()` for errors | Called before logging, visible in Jaeger |
| **Pattern 5** | Counter with `module` label | `"module", telemetry.ModuleOrchestration` |
| **Pattern 6** | `request_id` as first span attribute | First attr in all `AddSpanEvent()` calls |

#### Logging Compliance (LOGGING_IMPLEMENTATION_GUIDE.md Section 11)

| Pattern | Requirement | Implementation |
|---------|-------------|----------------|
| **Pattern 1** | Logger nil check | `if o.logger != nil { ... }` before every log call |
| **Pattern 2** | `operation` field in every log | `"operation": "hallucination_detection"` |
| **Pattern 3** | `request_id` in request-scoped logs | `"request_id": requestID` in all log entries |

**Additional Required Fields (Section 10 & 15):**

| Field | Requirement | Implementation |
|-------|-------------|----------------|
| `error` | Error message in warn/error logs | `"error": hallErr.Error()` or `"error": err.Error()` |
| `duration_ms` | Track operation duration | `"duration_ms": time.Since(hallStartTime).Milliseconds()` |
| `status` | Result status for completions | `"status": "recovered"` or `"status": "unrecoverable"` |
| `WithContext` | Use context methods in handlers | `WarnWithContext()`, `ErrorWithContext()`, `InfoWithContext()` |

#### Log Messages Emitted

| Level | Message | When Logged |
|-------|---------|-------------|
| `WARN` | "LLM hallucinated non-existent agent" | On each hallucination detection before retry |
| `ERROR` | "Plan regeneration failed during hallucination retry" | When `regeneratePlan()` fails |
| `INFO` | "LLM hallucination recovered after retry" | When retry succeeds |
| `ERROR` | "LLM hallucination unrecoverable after retries" | When all retries exhausted |

#### Span Events (visible in Jaeger)

| Event Name | When Emitted |
|------------|--------------|
| `llm.hallucination_detected` | LLM used an agent not in allowed list |
| `llm.hallucination_regeneration_complete` | Retry LLM call completed |
| `llm.hallucination_regeneration_failed` | Retry LLM call failed |
| `llm.hallucination_recovered` | Retry succeeded, plan now valid |
| `llm.hallucination_unrecoverable` | All retries exhausted, still hallucinating |

#### Metrics Emitted

| Metric Name | Labels | Description |
|-------------|--------|-------------|
| `plan_generation.hallucinations` | `module`, `agent` | Count of hallucination detections |
| `plan_generation.hallucination_recovered` | `module`, `retries_used` | Count of successful recoveries |
| `plan_generation.hallucination_unrecoverable` | `module`, `agent` | Count of unrecoverable failures |

#### LLM Debug Store Integration (ARCHITECTURE.md §9.9)

When enabled, hallucination events are recorded to the LLM Debug Store for production debugging:

| Recording Site | Type | When Recorded |
|----------------|------|---------------|
| `generateExecutionPlan()` | `hallucination_detection` | When LLM hallucinates an agent |

**Metadata captured:**
- `hallucinated_agent`: The invented agent name
- `allowed_agents`: The list that was provided to the LLM
- `hall_retry`: Current retry attempt

#### Files to Modify

| File | Changes |
|------|---------|
| `interfaces.go` | Add `HallucinationRetryEnabled` and `HallucinationMaxRetries` to `OrchestratorConfig` |
| `orchestrator.go` | Add `PlanningPromptResult` struct, modify `buildPlanningPrompt()`, add `validatePlanAgainstAllowedAgents()`, update `generateExecutionPlan()` |
| `orchestrator.go` | Add `extractAgentNamesFromCapabilityInfo()` helper |
| `orchestrator.go` | Add LLM Debug Store recording for hallucination events (if `o.debugStore != nil`) |
| `factory.go` | Add default values for hallucination retry config in `DefaultConfig()` |

#### Orchestration Architecture Compliance (orchestration/ARCHITECTURE.md)

| Principle | Section | Status | Implementation |
|-----------|---------|--------|----------------|
| **Interface-Based DI** | §1 | ✅ | Uses `core.Logger`, `telemetry.*` - no direct ai/resilience imports |
| **Explicit Configuration** | §2 | ✅ | `HallucinationRetryEnabled`, `HallucinationMaxRetries` in `OrchestratorConfig` |
| **Progressive Enhancement** | §3 | ✅ | Defaults work out-of-box, explicit config for customization |
| **Fail-Safe Defaults** | §4 | ✅ | Default `enabled: true`, `retries: 1` |
| **Three-Layer Resilience** | §9 | ✅ | Layer 1: Built-in retry, Layer 3: Clear error on exhaustion |
| **Tiered Provider Pattern** | §8 | ✅ | Aligns with existing "Hallucination filtering" pattern |
| **LLM Debug Store** | §9.9 | ✅ | Records hallucination events via `o.debugStore.RecordAsync()` |
| **Module Dependencies** | §3 | ✅ | Only imports `core` + `telemetry` (per valid dependency list) |

#### Telemetry Architecture Compliance (telemetry/ARCHITECTURE.md)

| Principle | Section | Status | Implementation |
|-----------|---------|--------|----------------|
| **Global Singleton** | §3 | ✅ | Uses `telemetry.Counter()`, `telemetry.AddSpanEvent()`, `telemetry.RecordSpanError()` |
| **Progressive Disclosure** | §2.3 | ✅ | Uses Level 1 simple API (90% of use cases) |
| **Module Boundaries** | §2.4 | ✅ | Metrics defined in orchestrator.go, NOT unified_metrics.go |
| **Thread Safety** | §3 | ✅ | All telemetry functions are thread-safe via atomic operations |
| **Safe NoOp Fallback** | §3 | ✅ | Telemetry functions have built-in nil checks |
| **Context Propagation** | §5 | ✅ | Uses context for span events, request_id via baggage |
| **Cardinality Protection** | §8 | ⚠️ | `"agent", hallucinatedAgent` label - protected by 1000 limit |

**Cardinality Note**: The `"agent", hallucinatedAgent` label has variable cardinality based on LLM-invented names. This is acceptable because:
1. Hallucinated names follow predictable patterns (e.g., "time-tool-v1")
2. Built-in cardinality limiter (1000 combinations) protects against explosion
3. Critical for debugging production hallucination issues

#### Framework Design Principles Compliance (FRAMEWORK_DESIGN_PRINCIPLES.md)

| Principle | Status | Implementation |
|-----------|--------|----------------|
| **Production-First** | ✅ | Built-in retry mechanism with configurable limits, fail-safe when exhausted |
| **Intelligent Configuration** | ✅ | Smart defaults (`enabled: true`, `retries: 1`), explicit override via `OrchestratorConfig` |
| **Interface-First Design** | ✅ | Uses `core.Logger` interface, separate testable `validatePlanAgainstAllowedAgents()` function |
| **Resilient Runtime** | ✅ | Retries before failing, never fails silently |
| **Telemetry Nil-Safety** | ✅ | Global `telemetry.*` functions have built-in nil checks (no explicit guards needed) |
| **Logger Nil Check** | ✅ | All logger calls guarded with `if o.logger != nil` |
| **Actionable Errors** | ✅ | Error includes guidance: "verify the agent is registered in the discovery system" |
| **Module Architecture** | ✅ | Changes in `orchestration` module (valid: depends on `core` + `telemetry`) |
| **Backwards Compatibility** | ✅ | New config fields with defaults, no breaking changes to existing behavior |

### Fix 2: Strengthen Plan Generation Prompt (P1 - High)

Add explicit anti-hallucination constraint to the planning prompt in `buildPlanningPrompt()`.

#### Current Prompt (line ~1942-1946)

```go
Important:
1. Only use agents and capabilities that exist in the catalog
2. Ensure parameter names AND TYPES match exactly what the capability expects
...
```

#### Enhanced Prompt

```go
// In buildPlanningPrompt() - modify the default prompt around line 1942
Important:
1. CRITICAL: You MUST ONLY use agent names listed in "Available Agents" above
2. DO NOT invent or create agent names - if you use an agent not listed above, your plan will be REJECTED
3. If no available agent can fulfill the request, return an EMPTY plan:
   {"plan_id": "no-matching-tool", "steps": [], "original_request": "...", "mode": "autonomous"}
4. Ensure parameter names AND TYPES match exactly what the capability expects
5. Order steps based on dependencies
6. Be specific in instructions
```

#### Implementation

```go
// orchestrator.go - Around line 1942 in the default prompt
return fmt.Sprintf(`You are an AI orchestrator managing a multi-agent system.

%s

User Request: %s

Create an execution plan in JSON format with the following structure:
{
  "plan_id": "unique-id",
  ...
}

CRITICAL - Agent Name Rules:
- You MUST ONLY use agent_name values that appear in "Available Agents" above
- DO NOT invent, guess, or hallucinate agent names based on what you think should exist
- If you use ANY agent_name not explicitly listed above, your plan will be REJECTED
- If no available agent can fulfill the request, return a plan with ZERO steps

Important:
1. Only use agents and capabilities from the list above
...
`, capabilityInfo, request), nil
```

**Implementation location:** `orchestration/orchestrator.go` in `buildPlanningPrompt()` lines 1904-1955

---

## Fix 3: Enhanced Hallucination Retry Strategy (P0 - Critical)

> **Status**: ✅ IMPLEMENTED & VERIFIED (2026-02-05)
>
> **Verification Request ID:** `orch-1770268232220395625`
>
> **Implementation**: Generic hallucination retry with enhanced tiered selection is now the
> default behavior in `orchestrator.go`. Uses existing config:
> - `HallucinationRetryEnabled: true` (default)
> - `HallucinationMaxRetries: 1` (default)
>
> **Issue Solved**: The original retry logic used the same tool list. Now we extract context
> from the hallucinated step and re-run tiered selection with capability hints, allowing
> the LLM to find semantically similar tools.
>
> **Verified Behavior**: Query "What is 100 times the current tesla stock price, multiplied
> by the current temperature of Chicago?" triggered hallucination of `calculation-agent`,
> which was detected, retried with enhanced tiered selection, and recovered successfully.

### The Problem with Simple Retry

```
Original Flow (Broken):
┌─────────────────────────────────────────────────────────────────────────────┐
│ 1. User Request: "100 × Tesla stock price × Chicago temperature"            │
│                                                                             │
│ 2. Tiered Selection → [stock-service, weather-tool-v2]  (no math tool!)    │
│                                                                             │
│ 3. Plan Generation → LLM hallucinates "calculator" (doesn't exist)         │
│                                                                             │
│ 4. Hallucination Detected → Call regeneratePlan()                          │
│         │                                                                   │
│         └──► buildPlanningPrompt() → Tiered Selection AGAIN                │
│                      │                                                      │
│                      └──► Same request → Same tools [stock, weather]       │
│                                                                             │
│ 5. Retry Plan Generation → LLM STILL hallucinates "calculator"             │
│                                                                             │
│ 6. FAILURE: "LLM hallucinated agent 'calculator' after 1 retries"          │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Root Cause**: Tiered selection doesn't know that "calculator" capability is needed.
The user request "100 × Tesla stock price × Chicago temperature" doesn't contain
keywords that would trigger selection of `research-agent-telemetry/math_analysis`.

### Solution: Enhanced Tiered Selection with Hallucination Hint

On hallucination retry, we should:
1. **Extract** the hallucinated agent/capability info from the failed plan
2. **Enhance** the request with a hint about the needed capability
3. **Re-run** tiered selection with the enhanced request
4. **Prepend** critical feedback about the hallucination to the new prompt
5. **Call** the LLM with the (hopefully better) tool list

### Complete Retry Workflow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                        ENHANCED HALLUCINATION RETRY WORKFLOW                             │
│                              (Domain-Agnostic Design)                                    │
└─────────────────────────────────────────────────────────────────────────────────────────┘

                              ┌──────────────────────┐
                              │    User Request      │
                              │  "100 × TSLA price   │
                              │   × Chicago temp"    │
                              └──────────┬───────────┘
                                         │
                                         ▼
                    ┌────────────────────────────────────────────┐
                    │         1. TIERED SELECTION (Original)      │
                    │    ─────────────────────────────────────    │
                    │    Input: User request                      │
                    │    Output: [stock-service, weather-service] │
                    │    Note: No math tool selected              │
                    └────────────────────┬───────────────────────┘
                                         │
                                         ▼
                    ┌────────────────────────────────────────────┐
                    │         2. PLAN GENERATION (Original)       │
                    │    ─────────────────────────────────────    │
                    │    Prompt: Available agents list            │
                    │    LLM Response: Uses "calculation-agent"   │
                    │    ⚠️  HALLUCINATION DETECTED!               │
                    └────────────────────┬───────────────────────┘
                                         │
                                         ▼
                    ┌────────────────────────────────────────────┐
                    │      3. HALLUCINATION VALIDATION            │
                    │    ─────────────────────────────────────    │
                    │    Check: "calculation-agent" in allowed?   │
                    │    Result: ❌ NOT FOUND                      │
                    │    Action: Trigger enhanced retry           │
                    └────────────────────┬───────────────────────┘
                                         │
           ┌─────────────────────────────┴─────────────────────────────┐
           │                                                           │
           ▼                                                           │
┌──────────────────────────────────┐                                   │
│  4. EXTRACT HALLUCINATION CONTEXT │                                   │
│  ────────────────────────────────  │                                   │
│  From failed plan step:           │                                   │
│  • AgentName: "calculation-agent" │                                   │
│  • Instruction: "Calculate 100    │                                   │
│    times the Tesla stock price    │                                   │
│    multiplied by temperature"     │                                   │
│  • Capability: "" (not in meta)   │                                   │
└───────────────┬──────────────────┘                                   │
                │                                                       │
                ▼                                                       │
┌──────────────────────────────────┐                                   │
│  5. BUILD ENHANCED REQUEST        │                                   │
│  ────────────────────────────────  │                                   │
│  Original + CAPABILITY_HINT:      │                                   │
│                                   │                                   │
│  "100 × TSLA price × Chicago temp │                                   │
│                                   │                                   │
│  [CAPABILITY_HINT: The request    │                                   │
│  requires a tool that can         │                                   │
│  perform: Calculate 100 times     │                                   │
│  the Tesla stock price...;        │                                   │
│  agent type: calculation-agent]"  │                                   │
└───────────────┬──────────────────┘                                   │
                │                                                       │
                ▼                                                       │
┌──────────────────────────────────┐                                   │
│  6. TIERED SELECTION (Re-run)     │                                   │
│  ────────────────────────────────  │                                   │
│  Input: Enhanced request with     │                                   │
│         CAPABILITY_HINT           │                                   │
│  LLM: Semantic match attempt      │                                   │
│  Output: [stock-service,          │                                   │
│           weather-service]        │                                   │
│  Note: Same tools (no math tool   │                                   │
│        exists in registry)        │                                   │
└───────────────┬──────────────────┘                                   │
                │                                                       │
                ▼                                                       │
┌──────────────────────────────────┐                                   │
│  7. BUILD RETRY PROMPT            │                                   │
│  ────────────────────────────────  │                                   │
│  CRITICAL ERROR - YOUR PREVIOUS   │                                   │
│  PLAN WAS REJECTED:               │                                   │
│  You used agent 'calculation-     │                                   │
│  agent' which does NOT exist.     │                                   │
│                                   │                                   │
│  STRICT RULES FOR THIS RETRY:     │                                   │
│  1. ONLY use agents from list     │                                   │
│  2. DO NOT hallucinate            │                                   │
│  3. Return ZERO steps if needed   │                                   │
│  4. Check list carefully!         │                                   │
│                                   │                                   │
│  [Full planning prompt with       │                                   │
│   stock-service, weather-service] │                                   │
└───────────────┬──────────────────┘                                   │
                │                                                       │
                ▼                                                       │
┌──────────────────────────────────┐                                   │
│  8. PLAN GENERATION (Retry)       │                                   │
│  ────────────────────────────────  │                                   │
│  LLM sees: CRITICAL ERROR +       │                                   │
│            Available tools list   │                                   │
│  LLM generates: Valid 2-step plan │                                   │
│    Step 1: stock-service/quote    │                                   │
│    Step 2: weather-service/       │                                   │
│            current_weather        │                                   │
│  ✅ NO HALLUCINATION               │                                   │
└───────────────┬──────────────────┘                                   │
                │                                                       │
                ▼                                                       │
┌──────────────────────────────────┐                                   │
│  9. VALIDATION (New Plan)         │                                   │
│  ────────────────────────────────  │                                   │
│  Check all agents in allowed list │                                   │
│  • stock-service: ✅               │                                   │
│  • weather-service: ✅             │                                   │
│  Result: VALID PLAN               │                                   │
└───────────────┬──────────────────┘                                   │
                │                                                       │
                └─────────────────────────────┬─────────────────────────┘
                                              │
                                              ▼
                    ┌────────────────────────────────────────────┐
                    │              10. DAG EXECUTION              │
                    │    ─────────────────────────────────────    │
                    │    Execute valid plan:                      │
                    │    • Step 1: Get TSLA stock price           │
                    │    • Step 2: Get Chicago temperature        │
                    │    Both steps succeed                       │
                    └────────────────────┬───────────────────────┘
                                         │
                                         ▼
                    ┌────────────────────────────────────────────┐
                    │              11. SYNTHESIS                  │
                    │    ─────────────────────────────────────    │
                    │    LLM combines results:                    │
                    │    • TSLA price: $X                         │
                    │    • Chicago temp: Y°F                      │
                    │    • Calculates: 100 × X × Y = Z            │
                    │    Returns natural language response        │
                    └────────────────────┬───────────────────────┘
                                         │
                                         ▼
                              ┌──────────────────────┐
                              │   ✅ SUCCESS!         │
                              │   User gets answer   │
                              │   with calculation   │
                              └──────────────────────┘
```

### Sequence Diagram (Simplified)

```
User          Orchestrator       Tiered Selection      Plan LLM        Executor       Synthesis
 │                 │                    │                  │               │              │
 │──── Request ───▶│                    │                  │               │              │
 │                 │                    │                  │               │              │
 │                 │──── Select ───────▶│                  │               │              │
 │                 │◀─── [stock,weather]│                  │               │              │
 │                 │                    │                  │               │              │
 │                 │─────────────── Generate Plan ────────▶│               │              │
 │                 │◀──────────── Plan w/"calculation-agent"               │              │
 │                 │                    │                  │               │              │
 │                 │ ⚠️ HALLUCINATION    │                  │               │              │
 │                 │   DETECTED!        │                  │               │              │
 │                 │                    │                  │               │              │
 │                 │─ Enhanced Request ▶│                  │               │              │
 │                 │  + CAPABILITY_HINT │                  │               │              │
 │                 │◀─── [stock,weather]│  (same tools)    │               │              │
 │                 │                    │                  │               │              │
 │                 │────── CRITICAL ERROR + Prompt ───────▶│               │              │
 │                 │◀──────────── Valid Plan (no halluc.)  │               │              │
 │                 │                    │                  │               │              │
 │                 │ ✅ VALIDATION PASS  │                  │               │              │
 │                 │                    │                  │               │              │
 │                 │─────────────────────────── Execute ──▶│               │              │
 │                 │◀────────────────────────── Results ───│               │              │
 │                 │                    │                  │               │              │
 │                 │────────────────────────────────────── Synthesize ────▶│              │
 │                 │◀───────────────────────────────────── Response ───────│              │
 │                 │                    │                  │               │              │
 │◀─── Response ───│                    │                  │               │              │
 │                 │                    │                  │               │              │
```

```
Enhanced Retry Flow:
┌─────────────────────────────────────────────────────────────────────────────┐
│ 1. Hallucination Detected: LLM used "calculator" (doesn't exist)           │
│                                                                             │
│ 2. Extract hallucinated info:                                               │
│    - agent_name: "calculator"                                               │
│    - capability: "calculate" (from plan metadata)                           │
│                                                                             │
│ 3. Enhance request for tiered selection:                                    │
│    Original: "100 × Tesla stock price × Chicago temperature"               │
│    Enhanced: "100 × Tesla stock price × Chicago temperature                │
│              [CAPABILITY_HINT: calculation, math, arithmetic]"              │
│                                                                             │
│ 4. Re-run Tiered Selection with enhanced request                            │
│    → Now selects: [stock-service, weather-tool-v2, research-agent/math]    │
│                                                                             │
│ 5. Build new prompt with:                                                   │
│    a) CRITICAL FEEDBACK (first!) about hallucination                        │
│    b) New tool list (includes math_analysis)                                │
│    c) Original request                                                      │
│                                                                             │
│ 6. LLM generates valid plan using research-agent-telemetry/math_analysis   │
│                                                                             │
│ 7. SUCCESS!                                                                 │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Implementation Design

> **Design Principle**: This implementation is **domain-agnostic** and works for any application
> built on the framework. It does NOT use hard-coded keyword mappings, instead relying on the
> LLM's semantic understanding to match hallucinated capabilities to available tools.

#### Step 1: Extract Hallucination Context (Generic)

```go
// HallucinationContext captures info about the hallucinated agent for retry hints.
// This is a GENERIC structure - no domain-specific knowledge required.
type HallucinationContext struct {
    AgentName   string // The hallucinated agent name (e.g., "calculator")
    Capability  string // The capability from plan metadata (e.g., "calculate")
    Instruction string // The step's instruction (e.g., "Multiply 100 by stock price")
}

// extractHallucinationContext extracts context from a failed plan for retry hints.
// This function is GENERIC - it extracts whatever the LLM was trying to do without
// any domain-specific interpretation.
func extractHallucinationContext(plan *RoutingPlan, hallucinatedAgent string) *HallucinationContext {
    ctx := &HallucinationContext{
        AgentName: hallucinatedAgent,
    }

    if plan == nil {
        return ctx
    }

    // Find the step with the hallucinated agent
    for _, step := range plan.Steps {
        if step.AgentName == hallucinatedAgent {
            ctx.Instruction = step.Instruction
            if step.Metadata != nil {
                // Extract capability from metadata
                if cap, ok := step.Metadata["capability"].(string); ok {
                    ctx.Capability = cap
                }
            }
            break
        }
    }

    return ctx
}
```

#### Step 2: Enhanced Request for Tiered Selection (Generic)

```go
// buildEnhancedRequestForRetry creates an enhanced request for tiered selection.
//
// DESIGN: This is GENERIC and domain-agnostic. Instead of mapping "calculator" to
// ["math", "calculation"] (which would require domain knowledge), we pass the
// hallucinated agent/capability/instruction directly to the tiered selection LLM,
// which can semantically match them to available tools.
//
// Example output:
//   "100 × Tesla stock price × Chicago temperature
//
//    [CAPABILITY_HINT: The request requires a tool that can perform: Multiply 100 by
//    the stock price and temperature values; agent type: calculator; capability: calculate.
//    The planning LLM attempted to use a non-existent tool. Please ensure any tools
//    with similar capabilities are included in the selection.]"
//
func buildEnhancedRequestForRetry(originalRequest string, hallCtx *HallucinationContext) string {
    if hallCtx == nil {
        return originalRequest
    }

    // Build descriptive hint from actual hallucination context
    // NO hard-coded domain knowledge - just describe what the LLM was trying to do
    var hintParts []string

    if hallCtx.Instruction != "" {
        hintParts = append(hintParts, fmt.Sprintf("perform: %s", hallCtx.Instruction))
    }
    if hallCtx.AgentName != "" {
        hintParts = append(hintParts, fmt.Sprintf("agent type: %s", hallCtx.AgentName))
    }
    if hallCtx.Capability != "" {
        hintParts = append(hintParts, fmt.Sprintf("capability: %s", hallCtx.Capability))
    }

    if len(hintParts) == 0 {
        return originalRequest
    }

    return fmt.Sprintf(`%s

[CAPABILITY_HINT: The request requires a tool that can %s.
The planning LLM attempted to use a non-existent tool. Please ensure any tools
with similar capabilities are included in the selection.]`,
        originalRequest,
        strings.Join(hintParts, "; "))
}
```

#### Step 3: Updated Hallucination Retry Logic

```go
// In generateExecutionPlan() - hallucination retry section

// Extract context about what the LLM was trying to do (GENERIC extraction)
hallCtx := extractHallucinationContext(plan, hallucinatedAgent)

// Build enhanced request with context from the hallucination
// This lets tiered selection's LLM semantically match to available tools
enhancedRequest := buildEnhancedRequestForRetry(request, hallCtx)

// Re-run tiered selection with enhanced request (may select different tools!)
retryPromptResult, retryPromptErr := o.buildPlanningPrompt(ctx, enhancedRequest)
if retryPromptErr != nil {
    // Log error and fall back to original prompt result
    if o.logger != nil {
        o.logger.ErrorWithContext(ctx, "Failed to build retry prompt with enhanced request", ...)
    }
    // Fall back to original prompt result if enhanced retry fails
    retryPromptResult = promptResult
}

// Update allowed agents list from the NEW prompt (may have different tools)
newAllowedList := make([]string, 0, len(retryPromptResult.AllowedAgents))
for name := range retryPromptResult.AllowedAgents {
    newAllowedList = append(newAllowedList, name)
}

// Build retry prompt with CRITICAL FEEDBACK FIRST
// Use capability if available, otherwise fall back to agent name
capabilityHint := hallCtx.Capability
if capabilityHint == "" {
    capabilityHint = hallCtx.AgentName
}
hallucinationFeedback := fmt.Sprintf(`CRITICAL ERROR - YOUR PREVIOUS PLAN WAS REJECTED:
You used agent '%s' which does NOT exist in the available agents list.

STRICT RULES FOR THIS RETRY:
1. You MUST ONLY use agents from the "Available Agents" section below
2. DO NOT invent, guess, or hallucinate any agent names
3. If you cannot fulfill the request with available agents, return a plan with ZERO steps
4. The capability you were trying to use ('%s') may be available under a DIFFERENT agent name - check the list carefully!

%s`, hallucinatedAgent, capabilityHint, retryPromptResult.Prompt)

// Call LLM with retry prompt
retryResponse, retryErr := o.aiClient.GenerateResponse(ctx, hallucinationFeedback, &core.AIOptions{
    Temperature: 0.2, // Lower temperature for more deterministic output
    MaxTokens:   2000,
})

// Validate the regenerated plan against the NEW allowed agents
// (retryPromptResult may have different tools from enhanced tiered selection)
hallucinatedAgent, hallErr = o.validatePlanAgainstAllowedAgents(plan, retryPromptResult.AllowedAgents)
```

### Why Generic Design Works

The key insight is that **tiered selection uses an LLM** for semantic matching. Instead of
hard-coding `"calculator" → ["math", "calculation"]`, we pass the full context:

```
[CAPABILITY_HINT: The request requires a tool that can perform: Multiply 100 by
the stock price and temperature values; agent type: calculator; capability: calculate.]
```

The tiered selection LLM then matches this semantically to:
- `research-agent-telemetry/math_analysis: "Advanced mathematical analysis and problem-solving..."`

This works for **any domain** because:
1. We describe WHAT the LLM was trying to do (instruction)
2. We describe WHAT it tried to use (agent type, capability)
3. The tiered selection LLM does semantic matching (no hard-coded patterns needed)

### Example: Calculator Hallucination

**Hallucinated step from plan:**
```json
{
  "agent_name": "calculator",
  "instruction": "Multiply 100 by the stock price and temperature values",
  "metadata": { "capability": "calculate" }
}
```

**Generated enhanced request:**
```
100 × Tesla stock price × Chicago temperature

[CAPABILITY_HINT: The request requires a tool that can perform: Multiply 100 by
the stock price and temperature values; agent type: calculator; capability: calculate.
The planning LLM attempted to use a non-existent tool. Please ensure any tools
with similar capabilities are included in the selection.]
```

**Tiered selection sees:**
- Available tool: `research-agent-telemetry/math_analysis: "Advanced mathematical analysis..."`
- Hint mentions: "Multiply", "calculate"
- **Semantic match!** → Includes `math_analysis` in selection

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **No hard-coded keyword mappings** | Framework must be domain-agnostic |
| **Use instruction from failed step** | Most descriptive of what LLM was trying to do |
| **Pass hallucinated names directly** | LLM can semantically match "calculator" to "math_analysis" |
| **Feedback FIRST in retry prompt** | LLM sees error before the long tool list |
| **Lower temperature (0.2) on retry** | More deterministic output after error |
| **Update allowedAgents from new prompt** | Validation uses the NEW tool list |
| **Zero-steps option explicit** | Gives LLM a valid fallback path |

### Configuration

```go
// interfaces.go - Add to OrchestratorConfig

// Enhanced Hallucination Retry configuration
// When enabled, extracts context from hallucinated agents and re-runs tiered
// selection with enhanced request to find semantically similar tools.
// This is GENERIC and works for any domain without hard-coded mappings.
HallucinationEnhancedRetryEnabled bool `json:"hallucination_enhanced_retry_enabled"` // Default: true
```

### Expected Outcomes

| Scenario | Before (Simple Retry) | After (Enhanced Retry) |
|----------|----------------------|------------------------|
| Any domain: hallucinated tool | ❌ Same tools selected, same hallucination | ✅ LLM semantic match finds alternatives |
| "100 × stock × temp" | ❌ Hallucinates "calculator" twice | ✅ Finds `math_analysis`, succeeds |
| "what time is it" | ❌ Hallucinates "time-tool" twice | ⚠️ Returns zero-steps (no time tool) |
| "email the report" | ❌ Hallucinates "email-service" | ⚠️ Returns zero-steps (no email tool) |

**Note**: For requests where NO tool can help, the enhanced retry will correctly return
a zero-step plan, which synthesis can handle gracefully.

### Files to Modify

| File | Changes |
|------|---------|
| `orchestrator.go` | Add `HallucinationContext`, `extractHallucinationContext()`, `buildEnhancedRequestForRetry()`, update retry logic |
| `interfaces.go` | Add `HallucinationEnhancedRetryEnabled` config |
| `factory.go` | Add default value for enhanced retry config |

---

## Implementation Status

| Priority | Fix | Status | Impact | Files |
|----------|-----|--------|--------|-------|
| **P0** | Fix 1 - Post-parse validation + retry | ✅ Implemented & Verified | Catches ALL hallucinations, allows self-correction | `orchestrator.go` |
| **P0** | Fix 3 - Enhanced retry with tiered hints | ✅ Implemented & Verified | Actually recovers from hallucinations | `orchestrator.go` |
| **P1** | Fix 2 - Strengthen prompt | ✅ Implemented | Reduces hallucination rate | `orchestrator.go` |

**Implementation Details:**
- Config fields added: `HallucinationRetryEnabled`, `HallucinationMaxRetries` in `OrchestratorConfig`
- New struct: `PlanningPromptResult` with `Prompt` and `AllowedAgents` fields
- New functions: `extractHallucinationContext()`, `buildEnhancedRequestForRetry()`, `validatePlanAgainstAllowedAgents()`
- Helper: `extractAgentNamesFromCapabilityInfo()` with regex parsing
- Telemetry: Span events, counters, and context-aware logging per DISTRIBUTED_TRACING_GUIDE.md
- Unit tests: 17 test cases in `hallucination_detection_test.go`

---

## Verification Steps

### Completed Verification (2026-02-05)

1. **✅ Test hallucination detection & recovery:**
   ```bash
   # Query that triggers hallucination (calculation-agent)
   curl -X POST http://localhost:8356/chat/stream \
     -H "Content-Type: application/json" \
     -d '{"message": "What is 100 times the current tesla stock price, multiplied by the current temperature of Chicago?"}'
   ```
   Result: Hallucination detected, retry succeeded, valid response returned

2. **✅ Check logs for hallucination workflow:**
   ```bash
   kubectl logs -n gomind-examples -l app=travel-chat-agent | grep -i "hallucin"
   ```
   Result: Logs show detection → retry → recovery sequence

3. **✅ Verify in Jaeger:**
   - Trace ID: `21bf54bbdf00cbb187279eb2bc2b1e7d`
   - Span events: `llm.hallucination_detected`, `llm.hallucination_recovered`

### Additional Test Cases

| Test | Query | Expected | Status |
|------|-------|----------|--------|
| Simple weather | "What's the weather in Tokyo?" | Valid plan, no hallucination | ✅ |
| Stock price | "What's Tesla stock price?" | Valid plan, no hallucination | ✅ |
| Math trigger | "100 × TSLA × Chicago temp" | Hallucinate → Recover | ✅ |
| No tool match | "What time is it?" | Hallucinate → Zero-step plan or synthesis | ⚠️ Depends on LLM |

---

## Related Files

- `orchestration/interfaces.go` - Config structs
  - `OrchestratorConfig`: lines 275-360 (add hallucination retry config here)
- `orchestration/factory.go` - Default config and factory functions
- `orchestration/orchestrator.go` - Plan generation, validation, and prompt building
  - `generateExecutionPlan()`: lines 1605-1852
  - `buildPlanningPrompt()`: lines 1858-1956
  - `validatePlan()`: lines 2044-2130
  - `regeneratePlan()`: lines 2133-2163
- `orchestration/tiered_capability_provider.go` - Tiered tool selection
  - `GetCapabilities()`: lines 217-280
  - `selectRelevantTools()`: lines 287-450
- `orchestration/catalog.go` - Agent catalog and formatting
  - `FormatToolsForLLM()`: lines 602-660
- `orchestration/executor.go` - Plan execution
- `orchestration/bugs/non-existant-tool.json` - Full debug payload

---

## Verification Results (2026-02-05)

### Test Query

```
"What is 100 times the current tesla stock price, multiplied by the current temperature of Chicago?"
```

### Request ID

`orch-1770268232220395625`

### Log Evidence

**1. Hallucination Detected:**
```json
{
  "level": "WARN",
  "message": "LLM hallucinated non-existent agent",
  "operation": "hallucination_detection",
  "request_id": "orch-1770268232220395625",
  "hallucinated_agent": "calculation-agent",
  "allowed_agents": ["stock-service", "weather-service"],
  "hall_retry": 1,
  "max_hall_retries": 1
}
```

**2. Enhanced Tiered Selection Re-run:**
```json
{
  "level": "DEBUG",
  "message": "Retrying plan generation with enhanced tiered selection",
  "operation": "hallucination_retry",
  "request_id": "orch-1770268232220395625",
  "hallucinated_agent": "calculation-agent",
  "hallucinated_instr": "Calculate 100 times the Tesla stock price multiplied by the current temperature in Chicago.",
  "original_tool_count": 2,
  "new_tool_count": 2
}
```

**3. Retry Prompt Sent to LLM:**
```
CRITICAL ERROR - YOUR PREVIOUS PLAN WAS REJECTED:
You used agent 'calculation-agent' which does NOT exist in the available agents list.

STRICT RULES FOR THIS RETRY:
1. You MUST ONLY use agents from the "Available Agents" section below
2. DO NOT invent, guess, or hallucinate any agent names
3. If you cannot fulfill the request with available agents, return a plan with ZERO steps
4. The capability you were trying to use ('calculation-agent') may be available under a DIFFERENT agent name - check the list carefully!

[Full planning prompt with Available Agents: weather-service, stock-service]

[CAPABILITY_HINT: The request requires a tool that can perform: Calculate 100 times
the Tesla stock price multiplied by the current temperature in Chicago.;
agent type: calculation-agent.]
```

**4. Recovery Success:**
```json
{
  "level": "INFO",
  "message": "LLM hallucination recovered after retry",
  "operation": "hallucination_detection",
  "request_id": "orch-1770268232220395625",
  "retries_used": 1,
  "duration_ms": 9353,
  "status": "recovered"
}
```

### Verification Summary

| Step | Expected | Actual | Status |
|------|----------|--------|--------|
| Hallucination Detection | Detect `calculation-agent` not in allowed list | Detected `calculation-agent` | ✅ |
| Context Extraction | Extract instruction from hallucinated step | Extracted: "Calculate 100 times..." | ✅ |
| Enhanced Request | Append CAPABILITY_HINT to request | Appended capability hint | ✅ |
| Tiered Selection Re-run | Re-run with enhanced request | Re-ran (same tools - no math tool exists) | ✅ |
| CRITICAL ERROR Feedback | Prepend error feedback to prompt | Prepended CRITICAL ERROR | ✅ |
| Plan Generation Retry | LLM generates valid plan | Generated valid 2-step plan | ✅ |
| Validation Pass | New plan uses only allowed agents | Plan uses stock-service, weather-service | ✅ |
| DAG Execution | Execute the valid plan | Executed successfully | ✅ |
| Synthesis | Combine results with calculation | LLM performed calculation in synthesis | ✅ |

### Key Observation

Even though the enhanced tiered selection returned the same tools (no math tool exists in the registry),
the fix still worked because:

1. **The CRITICAL ERROR feedback** informed the LLM of its mistake
2. **The LLM self-corrected** by generating a valid 2-step plan
3. **Synthesis handled the math** - when no dedicated tool exists, the LLM performs calculations in the final response

This demonstrates the robustness of the solution - it works even when tiered selection can't find
alternative tools, relying on the feedback loop to prevent re-hallucination.

---

## References

- [LLM Debug Payload](non-existant-tool.json) - Complete interaction trace
- [Tiered Capability Resolution](../notes/TIERED_CAPABILITY_RESOLUTION.md) - Design doc
- [Validation Code](../orchestrator.go#L2043) - validatePlan function
- [Orchestration Architecture](../ARCHITECTURE.md) - Module architecture and resilience patterns
- [Telemetry Architecture](../../telemetry/ARCHITECTURE.md) - Global singleton, module boundaries, cardinality
- [Framework Design Principles](../../FRAMEWORK_DESIGN_PRINCIPLES.md) - Architecture guidelines
- [Distributed Tracing Guide](../../docs/DISTRIBUTED_TRACING_GUIDE.md) - Tracing patterns
- [Logging Implementation Guide](../../docs/LOGGING_IMPLEMENTATION_GUIDE.md) - Logging patterns
