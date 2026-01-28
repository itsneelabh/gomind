# HITL Resume Flow Fix Plan

**Version**: 1.0
**Date**: January 2025
**Status**: Proposed
**Priority**: High - Critical bug in chained checkpoint flow

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Bug Description](#bug-description)
3. [Root Cause Analysis](#root-cause-analysis)
4. [Solution: Use Stored Plan on Resume](#solution-use-stored-plan-on-resume)
5. [Implementation Phases](#implementation-phases)
6. [Code Changes Required](#code-changes-required)
7. [Testing Strategy](#testing-strategy)
8. [Phase 5: Trace Continuity Fix](#phase-5-trace-continuity-fix)
   - [Implementation Bug Fix](#implementation-bug-fix-january-2025)
   - [Framework Enhancement](#framework-enhancement-set-original_request_id-on-all-traces-january-2025)
   - [CORS Fix](#cors-fix-for-x-gomind-original-request-id-header-january-2025)
   - [UI Enhancement](#ui-enhancement-display-original_request_id-for-easy-copying-january-2025)
   - [Summary of All Changes](#summary-of-all-changes-for-phase-5)
   - [Verification Results](#verification-results)

---

## Executive Summary

The HITL (Human-in-the-Loop) feature has a critical bug in chained checkpoint scenarios: when a user approves a plan-level checkpoint and resumes execution, any step-level checkpoints that fire will ask for approval MULTIPLE times for the same sensitive capability.

**Observed Behavior**:
- User sends: "Get stock price for AAPL"
- Plan-level HITL triggers (1 approval dialog) - User approves
- Step-level HITL for stock capability triggers (1 approval dialog) - User approves
- Step-level HITL for stock capability triggers AGAIN (duplicate approval!)
- Step-level HITL for stock capability triggers YET AGAIN (3rd duplicate!)

**Expected Behavior**:
- 1 plan approval + 1 step approval = 2 total dialogs

---

## Bug Description

### Trace Analysis (Request ID: awhl-1768623520494280336)

The agent logs show three consecutive step-approval checkpoints for the same stock capability:

```
1. cp-907fecc1 - Plan approval (plan_generated)
2. cp-c22f0aa5 - Step approval for stock (before_step, step-4)
3. cp-e7a86ac8 - Step approval for stock (before_step, step-3)
```

Notice the step IDs are DIFFERENT (`step-4` vs `step-3`), even though it's the same capability.

### Visual Timeline

```
Request: "Get stock price for AAPL"
    │
    ▼
┌─────────────────────────────────────────────────────────┐
│  PHASE 1: Initial Request Processing                    │
│                                                         │
│  1. Orchestrator generates plan via LLM                 │
│     Plan: step-1: geocode, step-2: weather,             │
│           step-3: stock (sensitive)                     │
│                                                         │
│  2. HITL policy detects sensitive capability (stock)    │
│     Creates checkpoint cp-907fecc1 (plan_generated)     │
│                                                         │
│  3. UI shows "Plan Approval Required" dialog            │
│     User clicks APPROVE                                 │
└─────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────┐
│  PHASE 2: Resume from Plan Approval (BUGGY)             │
│                                                         │
│  1. handlers.go:handleResumeSSE called                  │
│     - Loads checkpoint cp-907fecc1                      │
│     - Sets WithResumeMode(ctx, "cp-907fecc1")           │
│     - Calls ProcessWithStreaming(ctx, originalRequest)  │
│                                                         │
│  2. Orchestrator REGENERATES plan via LLM!              │
│     NEW Plan: step-1: geocode, step-2: weather,         │
│               step-4: stock  <-- NEW STEP ID!           │
│                                                         │
│  3. CheckPlanApproval skips (resume mode)               │
│                                                         │
│  4. Executor runs step-1, step-2 successfully           │
│                                                         │
│  5. CheckBeforeStep for step-4 (stock)                  │
│     - Policy returns ShouldInterrupt=true               │
│     - Creates checkpoint cp-c22f0aa5 (before_step)      │
│                                                         │
│  6. UI shows SECOND "Step Approval" dialog              │
│     User clicks APPROVE                                 │
└─────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────┐
│  PHASE 3: Resume from Step Approval (STILL BUGGY)       │
│                                                         │
│  1. handlers.go:handleResumeSSE called AGAIN            │
│     - Loads checkpoint cp-c22f0aa5                      │
│     - Sets WithResumeMode(ctx, "cp-c22f0aa5")           │
│     - Sets WithPreResolvedParams(ctx, params, "step-4") │
│     - Calls ProcessWithStreaming(ctx, originalRequest)  │
│                                                         │
│  2. Orchestrator REGENERATES plan AGAIN!                │
│     NEW Plan: step-1: geocode, step-2: weather,         │
│               step-3: stock  <-- DIFFERENT STEP ID!     │
│                                                         │
│  3. Executor runs step-1, step-2 AGAIN (wasted work)    │
│                                                         │
│  4. CheckBeforeStep for step-3 (stock)                  │
│     - Pre-resolved params key is "step-4" but           │
│       current step is "step-3" -> MISMATCH!             │
│     - Policy returns ShouldInterrupt=true               │
│     - Creates checkpoint cp-e7a86ac8 (before_step)      │
│                                                         │
│  5. UI shows THIRD "Step Approval" dialog               │
│     User clicks APPROVE... cycle continues              │
└─────────────────────────────────────────────────────────┘
```

---

## Root Cause Analysis

### Primary Issue: Resume Regenerates Plan

The resume flow in `handlers.go` does NOT use the stored plan from the checkpoint:

```go
// handlers.go:handleResumeSSE (line ~405)
ctx = orchestration.WithResumeMode(ctx, checkpointID)
if err := t.ProcessWithStreaming(ctx, sessionID, checkpoint.OriginalRequest, callback); err != nil {
    // ...
}
```

This calls `ProcessWithStreaming` with `checkpoint.OriginalRequest`, which causes the orchestrator to:
1. Generate a NEW plan via LLM
2. Assign NEW step IDs (not guaranteed to match the original)
3. Re-execute ALL steps from scratch

### Secondary Issue: Step ID Mismatch

Pre-resolved parameters are keyed by step ID:

```go
// handlers.go:handleResumeSSE (line ~411)
if checkpoint.ResolvedParameters != nil && len(checkpoint.ResolvedParameters) > 0 && checkpoint.CurrentStep != nil {
    ctx = orchestration.WithPreResolvedParams(ctx, checkpoint.ResolvedParameters, checkpoint.CurrentStep.StepID)
}
```

But since the plan is regenerated, the step IDs change:
- Checkpoint saved with `step-4` for stock
- Regenerated plan has `step-3` for stock
- Pre-resolved params lookup fails (keyed to wrong step ID)
- HITL triggers again for the "new" step

### Tertiary Issue: Completed Steps Not Skipped

The checkpoint stores `CompletedSteps` and `StepResults`, but these are never used:

```go
// hitl_interfaces.go - ExecutionCheckpoint struct
type ExecutionCheckpoint struct {
    // ...
    Plan              *RoutingPlan           `json:"plan"`            // <-- UNUSED on resume
    CompletedSteps    []StepResult           `json:"completed_steps"` // <-- UNUSED on resume
    CurrentStep       *RoutingStep           `json:"current_step"`    // <-- UNUSED on resume
    StepResults       map[string]*StepResult `json:"step_results"`    // <-- UNUSED on resume
    // ...
}
```

---

## Solution: Use Stored Plan on Resume

### Design Principle

The resume flow should **use the stored execution state** from the checkpoint rather than regenerating everything:

```
Current Flow (Buggy):
  Resume -> Regenerate Plan -> Re-execute All Steps -> HITL fires again

Fixed Flow:
  Resume -> Load Plan from Checkpoint -> Skip Completed Steps -> Resume from CurrentStep
```

### Key Changes

1. **Use Stored Plan**: Don't call LLM to regenerate; use `checkpoint.Plan`
2. **Skip Completed Steps**: Mark steps in `checkpoint.CompletedSteps` as done
3. **Resume from CurrentStep**: Start execution from `checkpoint.CurrentStep`
4. **Use Stored Parameters**: Apply `checkpoint.ResolvedParameters` directly

### Proposed Framework API

```go
// New context helpers in orchestrator.go

// WithPlanOverride injects a pre-approved plan, bypassing LLM planning.
func WithPlanOverride(ctx context.Context, plan *RoutingPlan) context.Context

// GetPlanOverride retrieves the injected plan from context.
func GetPlanOverride(ctx context.Context) *RoutingPlan

// WithCompletedSteps injects already-completed step results.
// The executor will skip these steps and use the provided results.
func WithCompletedSteps(ctx context.Context, results map[string]*StepResult) context.Context

// GetCompletedSteps retrieves completed steps from context.
func GetCompletedSteps(ctx context.Context) map[string]*StepResult
```

### Updated Resume Handler

**File**: `examples/agent-with-human-approval/handlers.go`
**Location**: `handleResumeSSE()` function, around line 405

```go
// EXISTING CODE (line 402-405):
callback.SendStatus("resuming", "Resuming execution after approval...")

// Mark context as resume mode to bypass HITL check
ctx = orchestration.WithResumeMode(ctx, checkpointID)

// NEW CODE - Insert after line 405:
// Validate checkpoint has required data
if checkpoint.Plan == nil {
    t.Logger.ErrorWithContext(ctx, "Invalid checkpoint - no plan stored", map[string]interface{}{
        "checkpoint_id": checkpointID,
    })
    callback.SendError("invalid_checkpoint", "Checkpoint has no plan - cannot resume", false)
    return
}

// ALWAYS use stored plan instead of regenerating via LLM
ctx = orchestration.WithPlanOverride(ctx, checkpoint.Plan)

// For step-level checkpoints, inject completed steps to skip re-execution
if checkpoint.InterruptPoint == orchestration.InterruptPointBeforeStep {
    if len(checkpoint.StepResults) > 0 {
        ctx = orchestration.WithCompletedSteps(ctx, checkpoint.StepResults)
        t.Logger.InfoWithContext(ctx, "Injecting completed steps from checkpoint", map[string]interface{}{
            "checkpoint_id":   checkpointID,
            "completed_count": len(checkpoint.StepResults),
        })
    }
}

// EXISTING CODE (line 407-419) - pre-resolved params injection (unchanged):
if checkpoint.ResolvedParameters != nil && len(checkpoint.ResolvedParameters) > 0 && checkpoint.CurrentStep != nil {
    ctx = orchestration.WithPreResolvedParams(ctx, checkpoint.ResolvedParameters, checkpoint.CurrentStep.StepID)
    // ...
}

// EXISTING CODE (line 422) - process request (unchanged):
if err := t.ProcessWithStreaming(ctx, sessionID, checkpoint.OriginalRequest, callback); err != nil {
    // ...
}
```

**Note**: Apply the same changes to `handleResumeSyncJSON()` (around line 836).

### Updated Orchestrator

**Location**: `ProcessRequestStreaming()` around line 1027, BEFORE calling `generateExecutionPlan`.

```go
// EXISTING CODE (around line 1026):
// ... earlier setup code ...

// NEW CODE - Insert before line 1027:
// Check for plan override (HITL resume)
plan := GetPlanOverride(ctx)
if plan != nil {
    // Resume flow: use stored plan from checkpoint
    if o.logger != nil {
        o.logger.InfoWithContext(ctx, "Using plan override from checkpoint", map[string]interface{}{
            "plan_id":    plan.PlanID,
            "step_count": len(plan.Steps),
            "operation":  "hitl_resume_plan_override",
        })
    }
} else {
    // EXISTING CODE (line 1027-1038) - now inside else block:
    // Normal flow: generate plan via LLM
    var err error
    plan, err = o.generateExecutionPlan(ctx, request, requestID)
    if err != nil {
        if o.logger != nil {
            o.logger.ErrorWithContext(ctx, "Plan generation failed", map[string]interface{}{
                "operation":  "streaming_plan_error",
                "request_id": requestID,
                "error":      err.Error(),
            })
        }
        return nil, fmt.Errorf("failed to generate execution plan: %w", err)
    }
}

// EXISTING CODE (line 1040+) - HITL plan approval check (unchanged)
// This already skips when IsResumeMode(ctx) is true
if o.config.HITL.Enabled && o.interruptController != nil {
    // ...
}
```

**Note**: Apply the same change to `ProcessRequest()` (around line 720).

### Updated Executor

GoMind uses **dynamic DAG-based execution** (not pre-computed batches). The executor has:
- `executed map[string]bool` - tracks which steps have run
- `stepResults map[string]*StepResult` - stores results for dependency resolution
- `findReadySteps()` - dynamically finds steps whose dependencies are satisfied

**The fix is simple**: Pre-populate these maps with checkpoint data at the start of `Execute()`.

```go
// executor.go - Execute method (around line 365)
func (e *SmartExecutor) Execute(ctx context.Context, plan *RoutingPlan) (*ExecutionResult, error) {
    // ... existing initialization ...

    // Create a map to store step results for dependency resolution
    stepResults := make(map[string]*StepResult)
    var resultsMutex sync.Mutex

    // Execute steps respecting dependencies
    executed := make(map[string]bool)

    // NEW: Pre-populate with completed steps from HITL checkpoint
    completedSteps := GetCompletedSteps(ctx)
    if completedSteps != nil {
        for stepID, cachedResult := range completedSteps {
            // Mark as executed so findReadySteps() won't return it
            executed[stepID] = true
            // Store result so dependent steps can access it
            stepResults[stepID] = cachedResult
            // Add to result.Steps so it appears in final output
            result.Steps = append(result.Steps, *cachedResult)

            if e.logger != nil {
                e.logger.DebugWithContext(ctx, "Pre-populated completed step from checkpoint", map[string]interface{}{
                    "step_id":   stepID,
                    "operation": "hitl_resume_prepopulate",
                })
            }
        }
    }

    // EXISTING: Main execution loop (unchanged)
    for len(executed) < len(plan.Steps) {
        readySteps := e.findReadySteps(plan, executed, stepResults)
        // ... rest of execution logic unchanged ...
    }
}
```

This approach is elegant because:
1. `findReadySteps()` automatically skips steps already in `executed`
2. Dependent steps can access cached results from `stepResults`
3. No changes needed to the main execution loop

---

## Implementation Phases

### Phase 1: Context Helpers (Framework)
**Files**: `orchestration/orchestrator.go`

1. Add `WithPlanOverride` / `GetPlanOverride` context helpers (used by orchestrator)
2. Add `WithCompletedSteps` / `GetCompletedSteps` context helpers (used by executor)

> **Note**: All context helpers are defined in `orchestrator.go` but exported so `executor.go`
> can call `GetCompletedSteps(ctx)`. This follows the existing pattern where `GetResolvedParams`
> and other context helpers are defined in orchestrator.go but used across the package.

### Phase 2: Orchestrator Changes (Framework)
**Files**: `orchestration/orchestrator.go`

1. Modify `ProcessRequest` to check for plan override before calling `generateRoutingPlan`
2. Modify `ProcessRequestStreaming` to check for plan override before calling `generateRoutingPlan`
3. If plan override exists, use it instead of calling LLM

> **Note**: The HITL plan check (`CheckPlanApproval`) already skips when `IsResumeMode(ctx)` is true.
> No additional changes needed there - it's handled by existing logic in `hitl_controller.go:100-112`.

### Phase 3: Executor Changes (Framework)
**Files**: `orchestration/executor.go`

1. Modify batch execution loop to check `CompletedSteps`
2. Use cached results for completed steps
3. Only execute steps NOT in `CompletedSteps`

### Phase 4: Handler Changes (Agent)
**Files**: `examples/agent-with-human-approval/handlers.go`

1. Update `handleResumeSSE` to differentiate plan-level vs step-level
2. Update `handleResumeSyncJSON` similarly
3. Inject `WithPlanOverride` and `WithCompletedSteps`
4. Add validation for nil plan in checkpoint

---

## Code Changes Required

### File: orchestration/orchestrator.go

```go
// Add after existing context helpers

// planOverrideContextKey holds a pre-approved plan for resume flows.
const planOverrideContextKey orchestratorContextKey = "orchestrator_plan_override"

// WithPlanOverride injects a pre-approved plan into context.
// When set, ProcessRequest/ProcessRequestStreaming will use this plan
// instead of generating a new one via LLM.
func WithPlanOverride(ctx context.Context, plan *RoutingPlan) context.Context {
    if plan == nil {
        return ctx
    }
    return context.WithValue(ctx, planOverrideContextKey, plan)
}

// GetPlanOverride retrieves the injected plan from context.
func GetPlanOverride(ctx context.Context) *RoutingPlan {
    if ctx == nil {
        return nil
    }
    if v := ctx.Value(planOverrideContextKey); v != nil {
        if p, ok := v.(*RoutingPlan); ok {
            return p
        }
    }
    return nil
}

// completedStepsContextKey holds step results from a checkpoint.
const completedStepsContextKey orchestratorContextKey = "orchestrator_completed_steps"

// WithCompletedSteps injects already-completed step results into context.
// The executor will skip these steps and use the cached results.
func WithCompletedSteps(ctx context.Context, results map[string]*StepResult) context.Context {
    if results == nil {
        return ctx
    }
    return context.WithValue(ctx, completedStepsContextKey, results)
}

// GetCompletedSteps retrieves completed step results from context.
func GetCompletedSteps(ctx context.Context) map[string]*StepResult {
    if ctx == nil {
        return nil
    }
    if v := ctx.Value(completedStepsContextKey); v != nil {
        if r, ok := v.(map[string]*StepResult); ok {
            return r
        }
    }
    return nil
}
```

### File: orchestration/executor.go

**Location**: Inside `Execute()` method, around line 369, AFTER the `executed` map is created.

```go
// EXISTING CODE (line 364-369):
// Create a map to store step results for dependency resolution
stepResults := make(map[string]*StepResult)
var resultsMutex sync.Mutex

// Execute steps respecting dependencies
executed := make(map[string]bool)

// NEW CODE - Insert here (after line 369, before line 371):
// Pre-populate with completed steps from HITL checkpoint
completedSteps := GetCompletedSteps(ctx)
if completedSteps != nil {
    for stepID, cachedResult := range completedSteps {
        executed[stepID] = true
        stepResults[stepID] = cachedResult
        result.Steps = append(result.Steps, *cachedResult)
        if e.logger != nil {
            e.logger.DebugWithContext(ctx, "Pre-populated completed step from checkpoint", map[string]interface{}{
                "step_id":   stepID,
                "operation": "hitl_resume_prepopulate",
            })
        }
    }
}

// EXISTING CODE (line 371) - unchanged:
for len(executed) < len(plan.Steps) {
    readySteps := e.findReadySteps(plan, executed, stepResults)
    // ...
}
```

**Why this works**: The `findReadySteps()` function checks `executed[stepID]` to skip already-executed steps.
By pre-populating this map, completed steps are automatically excluded from future execution.

---

## Why This Works (Existing Controller Logic)

The existing `hitl_controller.go` already has the correct skip logic. The bug was that this logic
couldn't work because step IDs kept changing. With the fix (using stored plan), the logic works:

**`CheckPlanApproval` (line 100-112):**
```go
if checkpointID, ok := IsResumeMode(ctx); ok {
    // Skip plan HITL - already approved
    return nil, nil
}
```

**`CheckBeforeStep` (line 229-276):**
```go
if checkpointID, ok := IsResumeMode(ctx); ok {
    checkpoint, _ := c.store.LoadCheckpoint(ctx, checkpointID)

    isStepCheckpoint := checkpoint.InterruptPoint == InterruptPointBeforeStep
    isThisStep := checkpoint.CurrentStep.StepID == step.StepID  // NOW MATCHES!

    if isStepCheckpoint && isThisStep {
        // Skip step HITL - this exact step was already approved
        return nil, nil
    }
}
```

**Before the fix**: `step.StepID` was "step-3" but `checkpoint.CurrentStep.StepID` was "step-4" (different plans)
**After the fix**: Both are "step-3" (same plan) → skip logic works!

---

## Testing Strategy

### Unit Tests

1. **Context helpers tests**
   - `TestWithPlanOverride` / `TestGetPlanOverride`
   - `TestWithCompletedSteps` / `TestGetCompletedSteps`

2. **Executor skip logic tests**
   - `TestExecutor_SkipsCompletedSteps` - Steps in CompletedSteps are skipped
   - `TestExecutor_UsesCachedResults` - Results from CompletedSteps added to output
   - `TestExecutor_PartialBatchCompletion` - Some steps in batch completed, others not
   - `TestExecutor_NoBatchCompleted` - No steps skipped (normal execution)

3. **Orchestrator plan override tests**
   - `TestOrchestrator_UsesPlanOverride` - Uses injected plan
   - `TestOrchestrator_SkipsLLMWhenPlanProvided` - No LLM call when plan injected

### Integration Tests

1. **Chained checkpoint flow (Plan -> Step)**
   - Send request that triggers plan + step HITL
   - Approve plan checkpoint
   - Verify stored plan used (no LLM call on resume)
   - Verify only ONE step checkpoint fires
   - Approve step checkpoint
   - Verify execution completes with no more checkpoints

2. **Multiple step checkpoints (Sequential)**
   - Send request with 2+ sensitive steps in different batches
   - Approve plan
   - Approve first sensitive step
   - Verify ONLY second sensitive step fires (not first again)
   - Approve second step
   - Verify completion

### E2E Tests (Manual)

1. Deploy agent-with-human-approval to k8s
2. Open hitl.html in browser
3. Send: "Get stock price for AAPL"
4. Count approval dialogs (should be 2: plan + stock)
5. Verify no duplicate dialogs

---

## Phase 5: Trace Continuity Fix (Client/Server Correlation)

### Problem Statement

When searching for a HITL request in Jaeger by `request_id`, only the resume trace is returned. The initial request trace (containing plan generation and checkpoint creation) is not found.

**Observed Behavior**:
- User searches Jaeger: `request_id=awhl-1768669718899878208`
- Jaeger returns: 1 trace (the final resume trace)
- Missing: Initial trace with plan generation, intermediate resume traces

**Expected Behavior**:
- ALL traces in a HITL conversation should be searchable by a single identifier

### Root Cause Analysis

HITL creates **multiple HTTP requests** → **multiple traces** with **different request_ids**:

```
┌─────────────────────────────────────────────────────────────────┐
│  INITIAL REQUEST (Trace A)                                      │
│                                                                 │
│  POST /chat/stream                                              │
│  └── orchestrator.process_request_streaming                     │
│      ├── Generates request_id = awhl-1768669654690679054       │
│      ├── plan_generation (LLM call)                            │
│      └── hitl.checkpoint.created (stores request_id)           │
│                                                                 │
│  Response: request_id = awhl-1768669654690679054               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  RESUME REQUEST 1 (Trace B) - Plan Approval                     │
│                                                                 │
│  POST /hitl/resume/{checkpoint_id}                              │
│  └── hitl.resume span (request_id from checkpoint)             │
│      └── orchestrator.process_request_streaming                 │
│          ├── Generates NEW request_id = awhl-..713..  ← NEW!   │
│          ├── executor runs steps                                │
│          └── hitl.checkpoint.created (stores NEW request_id)   │
│                                                                 │
│  Response: request_id = awhl-1768669713312047303 (NEW)         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  RESUME REQUEST 2 (Trace C) - Step Approval                     │
│                                                                 │
│  POST /hitl/resume/{checkpoint_id}                              │
│  └── hitl.resume span (request_id from checkpoint)             │
│      └── orchestrator.process_request_streaming                 │
│          ├── Generates NEW request_id = awhl-..718..  ← NEW!   │
│          └── synthesizer                                        │
│                                                                 │
│  Response: request_id = awhl-1768669718899878208 (NEW)         │
└─────────────────────────────────────────────────────────────────┘
```

**Key Issue**: The orchestrator generates a NEW `request_id` on every call. Each resume creates a new ID, and the user only sees the FINAL one in the response.

### Design Decision: Client/Server Correlation

This is a **client/server correlation problem**, not a framework problem. The framework correctly generates unique request_ids per orchestrator call. The solution is to propagate a **conversation-level correlation ID** via HTTP headers.

**Why not change the framework?**
- The orchestrator's request_id is designed to be unique per call (for metrics, debugging)
- Changing this would break existing behavior and metrics
- Client/server correlation is a standard HTTP pattern (similar to `X-Request-ID` in load balancers)

### Solution: X-Gomind-Original-Request-ID Header

Use a custom header following GoMind naming convention to propagate the original request_id:

```
Header: X-Gomind-Original-Request-ID
```

**Flow:**

1. **Initial Request** → Agent returns `request_id` in response
2. **UI stores** the `request_id` as `originalRequestId`
3. **Resume Request** → UI sends `X-Gomind-Original-Request-ID: {originalRequestId}` header
4. **Agent extracts header** → Sets `original_request_id` in baggage (separate from orchestrator's `request_id`)
5. **All spans** have `original_request_id` attribute for search

### Code Changes

#### File: `examples/chat-ui/hitl.html`

**Location**: JavaScript section, store original request_id from first response

```javascript
// Store original request_id when first checkpoint is received
let originalRequestId = null;

// In SSE message handler for initial request:
if (data.request_id && !originalRequestId) {
    originalRequestId = data.request_id;
    console.log('Stored original request_id:', originalRequestId);
}

// In resume request:
async function resumeCheckpoint(checkpointId, decision) {
    const headers = {
        'Content-Type': 'application/json',
    };

    // Include original request_id for trace correlation
    if (originalRequestId) {
        headers['X-Gomind-Original-Request-ID'] = originalRequestId;
    }

    const response = await fetch(`/hitl/resume/${checkpointId}`, {
        method: 'POST',
        headers: headers,
        body: JSON.stringify({ decision: decision })
    });
    // ...
}
```

#### File: `examples/agent-with-human-approval/handlers.go`

**Location**: `handleResumeSSE()` function, extract header and set baggage

```go
// Extract original request_id from header for trace correlation
// This allows searching Jaeger by original_request_id to find ALL traces in the conversation
originalRequestID := r.Header.Get("X-Gomind-Original-Request-ID")
if originalRequestID == "" {
    // Fallback to checkpoint's request_id if header not provided
    originalRequestID = checkpoint.RequestID
}

// Create linked span with both request IDs
ctx, endLinkedSpan := telemetry.StartLinkedSpan(
    ctx,
    "hitl.resume",
    originalTraceID,
    originalSpanID,
    map[string]string{
        "checkpoint_id":        checkpointID,
        "request_id":           checkpoint.RequestID,      // Checkpoint's request_id
        "original_request_id":  originalRequestID,         // Conversation's original ID
        "link.type":            "hitl_resume",
    },
)
defer endLinkedSpan()

// Set original_request_id in baggage for downstream span propagation
// This key is separate from "request_id" which the orchestrator will overwrite
ctx = telemetry.WithBaggage(ctx, "original_request_id", originalRequestID)
```

**Location**: `handleResumeSyncJSON()` function - apply same changes

### Why This Works

1. **Separate key avoids overwrite**: Using `original_request_id` instead of `request_id` means the orchestrator's baggage write doesn't overwrite it

2. **Baggage propagates to all spans**: All downstream spans (orchestrator, executor, tool calls) will have `original_request_id` attribute

3. **Single search finds all traces**: Searching Jaeger by `original_request_id=X` returns:
   - Initial trace (has `original_request_id=X` via baggage inheritance)
   - All resume traces (have `original_request_id=X` via header → baggage)

4. **No framework changes**: This is entirely handled at the agent/UI level

### GoMind Header Convention

All GoMind-specific HTTP headers should follow the naming pattern:

```
X-Gomind-{Purpose}
```

Examples:
- `X-Gomind-Original-Request-ID` - Trace correlation across HITL resumes
- `X-Gomind-Session-ID` - Session tracking (if needed)
- `X-Gomind-Checkpoint-ID` - Already used in webhook handler

### Verification

After the fix:

1. Send initial request: "Get stock price for AAPL"
2. Approve plan checkpoint
3. Approve step checkpoint
4. Note the `request_id` from final response

Search Jaeger by `original_request_id={value}`:
- Should return 3 traces (initial + 2 resumes)
- All traces linked via FOLLOWS_FROM references

### Backward Compatibility

If the UI doesn't send `X-Gomind-Original-Request-ID`:
- Agent falls back to `checkpoint.RequestID`
- Existing behavior preserved
- No breaking changes

### Implementation Bug Fix (January 2025)

**Issue Discovered**: The checkpoint SSE event was NOT including `request_id` in its data payload.

**Root Cause**: In `sse_handler.go`, the `SendCheckpoint()` function created a data map with `checkpoint_id`, `interrupt_point`, `expires_at`, and `status`, but did NOT include `request_id`:

```go
// BEFORE (bug):
data := map[string]interface{}{
    "checkpoint_id":   checkpoint.CheckpointID,
    "interrupt_point": checkpoint.InterruptPoint,
    "expires_at":      checkpoint.ExpiresAt,
    "status":          checkpoint.Status,
}
```

**Effect**: The JavaScript code `if (data.request_id && !originalRequestId)` never triggered because `data.request_id` was `undefined`. This meant `originalRequestId` stayed `null`, and the `X-Gomind-Original-Request-ID` header was never sent on resume requests.

**Fix**: Added `request_id` to the checkpoint event data in `SendCheckpoint()`:

```go
// AFTER (fixed):
data := map[string]interface{}{
    "checkpoint_id":   checkpoint.CheckpointID,
    "request_id":      checkpoint.RequestID,  // Added for trace correlation
    "interrupt_point": checkpoint.InterruptPoint,
    "expires_at":      checkpoint.ExpiresAt,
    "status":          checkpoint.Status,
}
```

**File Changed**: `examples/agent-with-human-approval/sse_handler.go` (line 143-148)

### Framework Enhancement: Set original_request_id on ALL Traces (January 2025)

**Problem**: The initial trace did NOT have `original_request_id` attribute, only resume traces had it. This meant searching by `original_request_id` missed the initial trace.

**Solution**: Modified the orchestrator to set `original_request_id` in baggage for ALL requests:
- On **initial requests**: `original_request_id = request_id` (same value)
- On **resume requests**: `original_request_id` is already set via header, so it's preserved

#### File: `orchestration/orchestrator.go`

**Location 1**: `ProcessRequest()` (around line 736)

```go
// Add request_id to context baggage so downstream components (AI client, etc.)
// can access it via telemetry.GetBaggage() and include it in their logs
ctx = telemetry.WithBaggage(ctx, "request_id", requestID)

// Set original_request_id for trace correlation across HITL resumes.
// On initial requests: original_request_id = request_id (same value)
// On resume requests: original_request_id is already set via header, don't overwrite
if bag := telemetry.GetBaggage(ctx); bag == nil || bag["original_request_id"] == "" {
    ctx = telemetry.WithBaggage(ctx, "original_request_id", requestID)
}
```

**Location 2**: `ProcessRequestStreaming()` (around line 1034)

Same change applied to the streaming variant.

**Location 3**: Span attribute (around line 1056)

```go
span.SetAttribute("request_id", requestID)
span.SetAttribute("streaming", true)
// Set original_request_id for trace correlation - will be same as request_id on initial,
// or the original value on resumes (preserved from baggage set by handler)
if bag := telemetry.GetBaggage(ctx); bag != nil && bag["original_request_id"] != "" {
    span.SetAttribute("original_request_id", bag["original_request_id"])
}
```

**Why This Works**:
1. Initial trace now has `original_request_id` attribute (same as `request_id`)
2. Resume traces have `original_request_id` from header (preserved, not overwritten)
3. Single Jaeger search by `original_request_id` finds ALL traces in the conversation

### CORS Fix for X-Gomind-Original-Request-ID Header (January 2025)

**Problem**: Browser blocked the `X-Gomind-Original-Request-ID` header on resume requests due to CORS.

**Root Cause**: The CORS middleware was configured with `core.WithCORS([]string{"*"}, true)` which only allowed `Content-Type` and `Authorization` headers.

**Fix**: Changed to `core.WithCORSDefaults()` which allows all headers.

**File Changed**: `examples/agent-with-human-approval/main.go` (line 94)

```go
// BEFORE:
core.WithCORS([]string{"*"}, true),

// AFTER:
core.WithCORSDefaults(), // Allows all headers including X-Gomind-Original-Request-ID
```

### UI Enhancement: Display original_request_id for Easy Copying (January 2025)

**Problem**: The UI displayed the final `request_id` (from the last resume), but users need the `original_request_id` to search Jaeger for all traces.

**Solution**: Updated `showCompletion()` to display `original_request_id` instead.

**File Changed**: `examples/chat-ui/hitl.html` (showCompletion function)

```javascript
// Use original_request_id for Jaeger search (finds ALL traces in conversation)
// Fall back to requestId if originalRequestId not set
const traceId = originalRequestId || requestId;
const shortTraceId = traceId ? traceId.substring(0, 12) : 'n/a';

footer.innerHTML = `
    <span class="footer-item">🛠️ ${toolCount} tool${toolCount !== 1 ? 's' : ''}</span>
    <span class="footer-item">⏱️ ${durationSec}s</span>
    <span class="footer-item request-id"
          title="Trace ID: ${traceId} - Search Jaeger with original_request_id=${traceId} (click to copy)"
          onclick="navigator.clipboard.writeText('${traceId}')...">
        🔍 ${shortTraceId}
    </span>
`;
```

**User Experience**:
1. After completing a HITL flow, click the 🔍 trace ID in the message footer
2. ID is copied to clipboard
3. In Jaeger, search: `original_request_id=<copied value>`
4. All 3 traces (initial + 2 resumes) are returned

### Summary of All Changes for Phase 5

| Layer | File | Change |
|-------|------|--------|
| **Framework** | `orchestration/orchestrator.go` | Set `original_request_id` in baggage for all requests |
| **Framework** | `orchestration/orchestrator.go` | Add `original_request_id` span attribute |
| **Agent** | `sse_handler.go` | Include `request_id` in checkpoint SSE event |
| **Agent** | `handlers.go` | Extract header, set baggage on resume |
| **Agent** | `main.go` | CORS fix to allow custom header |
| **UI** | `hitl.html` | Store `originalRequestId` from first checkpoint |
| **UI** | `hitl.html` | Send `X-Gomind-Original-Request-ID` header on resume |
| **UI** | `hitl.html` | Display `original_request_id` for easy copying |

### Verification Results

After all fixes, searching Jaeger by `original_request_id=awhl-1768678764913133805` returns:

| Trace | Type | request_id | original_request_id |
|-------|------|------------|---------------------|
| `c8da8be3...` | Initial `/chat/stream` | `awhl-...805` | `awhl-...805` ✅ |
| `cb193b2a...` | 1st Resume (plan) | `awhl-...422` | `awhl-...805` ✅ |
| `0222156b...` | 2nd Resume (step) | `awhl-...299` | `awhl-...805` ✅ |

All traces now have the same `original_request_id`, enabling single-search trace correlation.
