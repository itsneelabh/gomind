# Request ID Logging Issue Analysis

**Date**: 2026-01-08
**Status**: ✅ Fixed
**Affected Components**: Multiple modules across the framework

## Summary

Logs from framework components were missing `trace.request_id`, which prevents end-to-end request tracing. The issue was that multiple files used non-context logging methods and manually added trace fields instead of leveraging the framework's automatic baggage extraction.

## Implementation Summary

The fix was applied across all framework modules by converting logger calls to use `WithContext` methods in methods that have access to `context.Context`. This enables automatic extraction of `trace.request_id`, `trace.trace_id`, and `trace.span_id` from context baggage.

### Files Updated

| Module | File | Changes |
|--------|------|---------|
| orchestration | [executor.go](executor.go) | ~40 logs converted to WithContext, removed manual trace_id/span_id |
| orchestration | [template_prompt_builder.go](template_prompt_builder.go) | 2 logs converted to WithContext |
| orchestration | [default_prompt_builder.go](default_prompt_builder.go) | 1 log converted to WithContext |
| orchestration | [catalog.go](catalog.go) | Multiple logs in Refresh() and fetchAgentInfo() converted |
| orchestration | [workflow_engine.go](workflow_engine.go) | Multiple logs converted, removed trace_id/span_id fields |
| core | [middleware.go](../core/middleware.go) | 4 logs in LoggingMiddleware converted |
| core | [agent.go](../core/agent.go) | 1 log in RecoveryMiddleware converted |
| core | [redis_registry.go](../core/redis_registry.go) | ~40+ logs across Register, UpdateHealth, Unregister, etc. |
| core | [redis_discovery.go](../core/redis_discovery.go) | ~20+ logs in Discover() method |
| core | [memory_store.go](../core/memory_store.go) | 10 logs in Get, Set, Delete, Exists methods |
| core | [redis_client.go](../core/redis_client.go) | 3 logs in HealthCheck() method |
| resilience | [retry.go](../resilience/retry.go) | 9 logs in Execute() method |
| ai | [ai_tool.go](../ai/ai_tool.go) | 4 logs in ProcessWithAI() and RegisterAICapability() handler |

### Pattern Applied

```go
// Before:
tc := telemetry.GetTraceContext(ctx)
e.logger.Debug("Message", map[string]interface{}{
    "field":    value,
    "trace_id": tc.TraceID,
    "span_id":  tc.SpanID,
})

// After:
e.logger.DebugWithContext(ctx, "Message", map[string]interface{}{
    "field": value,
})
// trace.request_id, trace.trace_id, trace.span_id added automatically
```

### Intentionally Unchanged

The following types of logs were intentionally NOT converted:
- Setup methods without ctx parameter (SetLogger, SetTelemetry, NewAITool, etc.)
- Panic handlers in defer blocks (ctx may be invalid)
- Helper methods without ctx parameter (findReadySteps, interpolateParameters, parsePlan, validatePlan)
- Periodic summary logs without ctx (logHeartbeatSummary, checkAndLogPeriodicSummary)
- Background processes without request context (runBackgroundRefresh, rotateBuckets)
- Configuration loading methods (LoadFromEnvironment, LoadFromFile)
- Factory/initialization functions (NewRetryExecutorWithDependencies)
- Telemetry module's internal TelemetryLogger (uses different logging implementation)

---

## Original Analysis (For Reference)

The following sections document the original problem analysis.

## Problem Statement

Example log output (current behavior):

```json
{
  "agent_name": "stock-service",
  "attempt": 1,
  "component": "framework/orchestration",
  "level": "DEBUG",
  "message": "Agent HTTP call successful",
  "operation": "agent_http_response",
  "response_length": 190,
  "service": "travel-chat-agent",
  "span_id": "cbcddc7704dd9405",
  "step_id": "step-1",
  "timestamp": "2026-01-08T17:25:29Z",
  "trace_id": "b7ea3d75093603d3cf1a0e31bb669d26"
}
```

**Missing**: `trace.request_id` field that should connect this log to the orchestration request.

**Also incorrect**: `trace_id` and `span_id` should have `trace.` prefix per documentation.

## How Request ID Should Flow

### Step 1: Orchestrator Generates request_id

At [orchestrator.go:373](orchestrator.go#L373):

```go
requestID := generateRequestID()
```

### Step 2: Orchestrator Adds request_id to Context Baggage

At [orchestrator.go:375-377](orchestrator.go#L375-L377):

```go
// Add request_id to context baggage so downstream components (AI client, etc.)
// can access it via telemetry.GetBaggage() and include it in their logs
ctx = telemetry.WithBaggage(ctx, "request_id", requestID)
```

### Step 3: ProductionLogger Extracts Baggage with WithContext Methods

At [core/config.go:1670-1676](../core/config.go#L1670-L1676):

```go
// LAYER 3: Add trace context when available
if ctx != nil && p.metricsEnabled {
    if baggage := getContextBaggage(ctx); len(baggage) > 0 {
        for k, v := range baggage {
            logEntry["trace."+k] = v  // Adds trace.request_id, trace.trace_id, etc.
        }
    }
}
```

### Step 4: FrameworkMetricsRegistry.GetBaggage() Includes Trace Context

At [telemetry/framework_integration.go:67-83](../telemetry/framework_integration.go#L67-L83):

```go
func (f *FrameworkMetricsRegistry) GetBaggage(ctx context.Context) map[string]string {
    // Start with W3C Baggage (custom key-value pairs like request_id)
    result := GetBaggage(ctx)
    if result == nil {
        result = make(map[string]string)
    }

    // Add OpenTelemetry trace context for log correlation
    tc := GetTraceContext(ctx)
    if tc.TraceID != "" {
        result["trace_id"] = tc.TraceID
        result["span_id"] = tc.SpanID
    }

    return result
}
```

## Root Cause

The executor.go breaks this flow by violating the framework's logging guidelines.

### 1. Using Non-Context Logging Methods

This is **"Mistake 1"** from [LOGGING_IMPLEMENTATION_GUIDE.md](../docs/LOGGING_IMPLEMENTATION_GUIDE.md#mistake-1-using-basic-methods-in-handlers):

> **The golden rule**: If you have access to `context.Context` from an HTTP request, use the `WithContext` methods. They enable trace-log correlation, which is essential for debugging in production.

```go
// Current (wrong) - executor.go:1511
e.logger.Debug("Agent HTTP call successful", map[string]interface{}{...})

// Should be (per documentation):
e.logger.DebugWithContext(ctx, "Agent HTTP call successful", map[string]interface{}{...})
```

When `Debug()` is used instead of `DebugWithContext()`, the context is not passed to `logEvent()`, so baggage extraction never happens.

The executor **does have access to `ctx`** - it's passed into `Execute(ctx, plan)` and `executeStep(ctx, step)`. This means it SHOULD use `WithContext` methods per the golden rule.

### 2. Manually Adding Trace Fields

```go
// Current (wrong) - executor.go:1517-1518
"trace_id": tc.TraceID,
"span_id":  tc.SpanID,
```

The executor manually extracts trace context at [executor.go:303](executor.go#L303) and [executor.go:1195](executor.go#L1195):

```go
tc := telemetry.GetTraceContext(ctx)
```

Then manually adds these fields to logs, but:
- Does NOT extract `request_id` from baggage
- Does NOT use the `trace.` prefix that the framework adds automatically

## Comparison: Orchestrator vs Executor Logging

### Orchestrator (Correct Pattern)

Uses `WithContext` methods AND explicitly includes request_id:

```go
// orchestrator.go:380-388
o.logger.InfoWithContext(ctx, "Starting request processing", map[string]interface{}{
    "operation":    "process_request",
    "request_id":   requestID,
    "request_length": len(request),
    ...
})
```

Note: The orchestrator has direct access to `requestID` variable, so it includes it explicitly in addition to using `WithContext`.

### Executor (Incorrect Pattern)

Uses non-context methods and manually adds only trace_id/span_id:

```go
// executor.go:1511-1519
e.logger.Debug("Agent HTTP call successful", map[string]interface{}{
    "operation":       "agent_http_response",
    "step_id":         step.StepID,
    "agent_name":      step.AgentName,
    "attempt":         attempt,
    "response_length": len(response),
    "trace_id":        tc.TraceID,
    "span_id":         tc.SpanID,
})
```

## Affected Log Statements

The executor.go has **~60+ log statements** that follow this incorrect pattern. Key locations:

| Line | Method | Message |
|------|--------|---------|
| 312 | Debug | "Starting plan execution" |
| 399 | Debug | "Executing steps in parallel" |
| 549 | Info | "Plan execution finished" |
| 1204 | Debug | "Starting step execution" |
| 1485 | Debug | "Making HTTP call to agent" |
| 1511 | Debug | "Agent HTTP call successful" |
| 1940 | Error | "Agent HTTP call failed after all retries" |
| ... | ... | ... |

Full list can be obtained with:
```bash
grep -n "e\.logger\." orchestration/executor.go
```

## Expected Log Format (After Fix)

Per [LOGGING_IMPLEMENTATION_GUIDE.md](../docs/LOGGING_IMPLEMENTATION_GUIDE.md#L686-L691) and [DISTRIBUTED_TRACING_GUIDE.md](../docs/DISTRIBUTED_TRACING_GUIDE.md#L251-L258):

```json
{
  "timestamp": "2026-01-08T17:25:29Z",
  "level": "DEBUG",
  "service": "travel-chat-agent",
  "component": "framework/orchestration",
  "message": "Agent HTTP call successful",
  "operation": "agent_http_response",
  "step_id": "step-1",
  "agent_name": "stock-service",
  "attempt": 1,
  "response_length": 190,
  "trace.request_id": "travel-research-1765636433370038463",
  "trace.trace_id": "b7ea3d75093603d3cf1a0e31bb669d26",
  "trace.span_id": "cbcddc7704dd9405"
}
```

## Recommended Fix

### Option A: Use WithContext Methods (Recommended - Per Documentation)

This is the **documented correct approach** per [LOGGING_IMPLEMENTATION_GUIDE.md](../docs/LOGGING_IMPLEMENTATION_GUIDE.md#where-to-use-each-logger-method):

| Location | Method to Use | Why |
|----------|---------------|-----|
| HTTP handler | `InfoWithContext()` | Request context available for tracing |

Change all logging calls to use `WithContext` methods and remove manual trace field addition:

```go
// Before:
tc := telemetry.GetTraceContext(ctx)
e.logger.Debug("Agent HTTP call successful", map[string]interface{}{
    "operation":       "agent_http_response",
    "step_id":         step.StepID,
    "agent_name":      step.AgentName,
    "attempt":         attempt,
    "response_length": len(response),
    "trace_id":        tc.TraceID,
    "span_id":         tc.SpanID,
})

// After:
e.logger.DebugWithContext(ctx, "Agent HTTP call successful", map[string]interface{}{
    "operation":       "agent_http_response",
    "step_id":         step.StepID,
    "agent_name":      step.AgentName,
    "attempt":         attempt,
    "response_length": len(response),
})
// trace.request_id, trace.trace_id, trace.span_id added automatically
```

**Pros**:
- Consistent with documented format
- Automatic baggage extraction (request_id, trace_id, span_id)
- Follows LOGGING_IMPLEMENTATION_GUIDE.md recommendations
- Future-proof: any new baggage items automatically included

**Cons**:
- Field naming changes (`trace_id` -> `trace.trace_id`) - Note: This is actually **fixing** to the documented standard format, not breaking it. Any existing log queries using `trace_id` would need updating, but they're currently non-standard.
- Requires updating ~60+ log statements

### Option B: Explicitly Extract request_id from Baggage (NOT Recommended)

This option keeps the current anti-pattern but adds request_id. **Not recommended** because it perpetuates "Mistake 1" from the logging guide:

```go
tc := telemetry.GetTraceContext(ctx)
baggage := telemetry.GetBaggage(ctx)
requestID := baggage["request_id"]

e.logger.Debug("Agent HTTP call successful", map[string]interface{}{
    "operation":       "agent_http_response",
    "step_id":         step.StepID,
    "request_id":      requestID,  // Added
    "trace_id":        tc.TraceID,
    "span_id":         tc.SpanID,
    ...
})
```

**Pros**:
- Minimal change to existing pattern
- No breaking change in field naming

**Cons**:
- Doesn't follow documented format (missing `trace.` prefix)
- Manual baggage extraction in multiple places
- Must remember to add each new baggage item manually

## Implementation Notes

1. **Keep `tc := telemetry.GetTraceContext(ctx)` for span events**: The calls at lines 303 and 1195 are used for `telemetry.AddSpanEvent()` calls, not just logging. These should be kept for span event attributes but removed from log field maps.

2. **Non-context logs are OK for initialization**: Some log statements during component setup (e.g., `SetLogger`, `SetTelemetry`) don't have request context. These should continue using non-context methods per [LOGGING_IMPLEMENTATION_GUIDE.md](../docs/LOGGING_IMPLEMENTATION_GUIDE.md#specific-locations).

3. **Orchestrator pattern**: The orchestrator uses both `WithContext` methods AND explicit `request_id` in fields because it has direct access to the `requestID` variable. The executor doesn't have this direct access, so it must rely solely on baggage extraction via `WithContext`.

4. **Scope of change**: Focus on log statements within request-handling code paths (Execute, executeStep, callTool, etc.). Log statements in SetLogger/SetTelemetry/constructor methods should remain non-context.

## Related Documentation

- [LOGGING_IMPLEMENTATION_GUIDE.md](../docs/LOGGING_IMPLEMENTATION_GUIDE.md) - Logging best practices
- [DISTRIBUTED_TRACING_GUIDE.md](../docs/DISTRIBUTED_TRACING_GUIDE.md) - Trace correlation setup
- [core/COMPONENT_LOGGING_DESIGN.md](../core/COMPONENT_LOGGING_DESIGN.md) - Component-aware logging architecture

## Testing

After implementing the fix, verify with:

```bash
# Deploy to cluster
kubectl apply -f examples/k8-deployment/

# Make a test request
curl -X POST http://localhost:8092/orchestrate/natural \
  -H "Content-Type: application/json" \
  -d '{"request":"What is the weather in Tokyo?"}'

# Check logs for trace.request_id
kubectl logs -n gomind-examples -l app=travel-chat-agent --since=60s | \
  grep -o '"trace\.request_id":"[^"]*"' | head -5
```

Expected output:
```
"trace.request_id":"travel-research-1765636433370038463"
"trace.request_id":"travel-research-1765636433370038463"
...
```
