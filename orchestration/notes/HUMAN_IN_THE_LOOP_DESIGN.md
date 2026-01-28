# Human-in-the-Loop (HITL) Implementation Reference

**Version**: 1.1
**Date**: January 2025
**Last Updated**: 2025-01-20
**Status**: Implemented

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Design Overview](#2-design-overview)
3. [HITL Scenarios](#3-hitl-scenarios)
   - [Scenario 1: Plan Approval](#scenario-1-plan-approval---complete)
   - [Scenario 2: Tool Call Approval](#scenario-2-tool-call-approval---complete-january-2025)
   - [Scenario 3: Output Validation](#scenario-3-output-validation---not-configured)
   - [Scenario 4: Error Escalation](#scenario-4-error-escalation---uses-scenario-2-flow)
4. [Files Created](#4-files-created)
5. [Files Modified](#5-files-modified)
6. [Key Code Snippets](#6-key-code-snippets)
7. [HITL Resume Fix](#7-hitl-resume-fix)
8. [Trace Continuity Enhancement](#8-trace-continuity-enhancement)
9. [Configuration Reference](#9-configuration-reference)
10. [API Reference](#10-api-reference)
11. [Testing](#11-testing)
12. [Quick Reference by Scenario](#12-quick-reference-by-scenario)

---

## 1. Executive Summary

This document is a consolidated reference for the Human-in-the-Loop (HITL) system implemented in GoMind's orchestration module. HITL enables human oversight at critical decision points while maintaining GoMind's core principles.

### Core Patterns Implemented

| Pattern | Description |
|---------|-------------|
| **Interrupt & Resume** | Pause execution at checkpoints, resume with human decisions |
| **Policy-Based Approval** | Declarative rules (via `RuleBasedPolicy`) for when human approval is required |
| **Checkpoint Persistence** | State management via Redis (DB 6) for long-running workflows |

### Four HITL Scenarios

| Scenario | Trigger Point | Implementation Status |
|----------|---------------|----------------------|
| 1. Plan Approval | After LLM generates plan | **Complete** |
| 2. Tool Call Approval | Before executing sensitive step | **Complete** |
| 3. Output Validation | After step execution | Policy returns false (not configured) |
| 4. Error Escalation | After N retry failures | Uses same flow as Scenario 2 |

---

## 2. Design Overview

### Architecture

```
+------------------------------------------------------------------+
|                        AIOrchestrator                             |
|  +------------------------------------------------------------+  |
|  |                   InterruptController                       |  |
|  |  +--------------+  +--------------+  +------------------+  |  |
|  |  | RuleBasedPolicy|  | Checkpoint |  | CheckpointStore  |  |  |
|  |  |              |  | Handler      |  | (Redis DB 6)     |  |  |
|  |  +--------------+  +--------------+  +------------------+  |  |
|  +------------------------------------------------------------+  |
|                              |                                    |
|  +------------------------------------------------------------+  |
|  |                     SmartExecutor                           |  |
|  |  Pre-Step Hook --> Execute Step --> Post-Step Hook          |  |
|  |       |                                  |                  |  |
|  |  [CheckBeforeStep]              [CheckAfterStep/OnError]    |  |
|  +------------------------------------------------------------+  |
+------------------------------------------------------------------+
```

### Data Flow

```
ProcessRequest()
    |
    v
generateExecutionPlan()
    |
    v
[CheckPlanApproval?] --Yes--> SaveCheckpoint() --> Return ErrInterrupted
    | No                                                  |
    v                                               Client receives
Execute()                                           checkpoint, shows UI
    |
For each step:                                      User approves
  [CheckBeforeStep?] --Yes--> SaveCheckpoint() --> Return ErrInterrupted
    | No
  executeStep()
    |
  Continue
```

---

## 3. HITL Scenarios

### Scenario 1: Plan Approval - COMPLETE

- **Trigger**: `GOMIND_HITL_REQUIRE_PLAN_APPROVAL=true` or plan contains sensitive capabilities
- **Location**: `orchestrator.go:790` (non-streaming), `orchestrator.go:1045` (streaming)
- **Behavior**: Returns `&ErrInterrupted{}` when checkpoint created
- **Agent handling**: `chat_agent.go:334` catches via `orchestration.IsInterrupted(err)`

### Scenario 2: Tool Call Approval - COMPLETE (January 2025)

- **Trigger**: Step capability matches `GOMIND_HITL_SENSITIVE_CAPABILITIES` or `GOMIND_HITL_STEP_SENSITIVE_CAPABILITIES`
- **Location**: `executor.go:1253` calls `CheckBeforeStep(ctx, step, plan)`
- **Key change**: Executor stores checkpoint in `result.Metadata["hitl_checkpoint"]`
- **Propagation flow**:
  1. Executor detects checkpoint in metadata after batch (lines 550-594)
  2. Executor calls `UpdateCheckpointProgress()` to save completed steps
  3. Executor returns `ErrInterrupted` with full checkpoint
  4. Orchestrator propagates at lines 800-814 (non-streaming) / 1055-1069 (streaming)

### Scenario 3: Output Validation - NOT CONFIGURED

- **Location**: `executor.go:2024` calls `CheckAfterStep(ctx, step, &result)`
- **Policy**: `hitl_policy.go:243-246` - `requiresOutputValidation()` always returns false
- **Gap**: No `GOMIND_HITL_VALIDATE_OUTPUT_CAPABILITIES` environment variable implemented

### Scenario 4: Error Escalation - USES SCENARIO 2 FLOW

- **Location**: `executor.go:2054` calls `CheckOnError(ctx, step, err, attempts)`
- **Trigger**: `GOMIND_HITL_ESCALATE_AFTER_RETRIES` exceeded
- **Status**: Uses same propagation mechanism as Scenario 2

---

## 4. Files Created

### Framework Files (`orchestration/`)

| File | Lines | Purpose |
|------|-------|---------|
| `hitl_interfaces.go` | 504 | Core interfaces: `InterruptController`, `InterruptPolicy`, `CheckpointStore`, `ExecutionCheckpoint` struct |
| `hitl_controller.go` | 844 | `DefaultInterruptController` - orchestrates HITL flow with `CheckPlanApproval`, `CheckBeforeStep`, `CheckAfterStep`, `CheckOnError` |
| `hitl_api.go` | 575 | HTTP handlers for HITL endpoints (`/hitl/command`, `/hitl/resume`, `/hitl/checkpoints`) |
| `hitl_api_test.go` | 853 | Comprehensive tests for HITL API handlers |
| `hitl_checkpoint_store.go` | 582 | Redis-backed checkpoint persistence (DB 6) |
| `hitl_webhook_handler.go` | 310 | `WebhookInterruptHandler` for external notifications |
| `hitl_webhook_handler_test.go` | 516 | Webhook handler tests |
| `hitl_policy.go` | 356 | `RuleBasedPolicy` implementation - evaluates when to interrupt |
| `hitl_command_store.go` | 323 | Redis Pub/Sub for command distribution |
| `hitl_metrics.go` | 121 | Prometheus metrics for HITL operations |
| `hitl_metrics_test.go` | 327 | Metrics tests |
| `hitl_errors.go` | 172 | `ErrInterrupted` type and helper functions (`IsInterrupted`, `GetCheckpoint`) |
| **Total** | **5,483** | |

### Example Agent Files (`examples/agent-with-human-approval/`)

| File | Lines | Purpose |
|------|-------|---------|
| `main.go` | ~500 | Entry point with HITL always enabled, CORS setup |
| `chat_agent.go` | ~790 | `HITLChatAgent` with `SetInterruptController()` integration |
| `handlers.go` | ~1,200 | HTTP handlers: `handleResumeSSE`, `handleResumeSyncJSON`, `WithPlanOverride` injection |
| `sse_handler.go` | ~430 | SSE callback with `SendCheckpoint()` method |
| `hitl_setup.go` | ~100 | HITL infrastructure setup (stores, controller, policy) |
| `session.go` | ~230 | Session management |

### UI Files (`examples/chat-ui/`)

| File | Lines | Purpose |
|------|-------|---------|
| `hitl.html` | 1,876 | Mode-aware HITL chat UI with approval dialog, checkpoint handling, resume flow |
| `welcome.html` | ~150 | Landing page for scenario selection (Chat Agent / Plan HITL / Step HITL) |

### Design Documents

| File | Purpose |
|------|---------|
| `orchestration/HUMAN_IN_THE_LOOP_PROPOSAL.md` | Original design proposal (v1.6) with architecture and interfaces |
| `orchestration/notes/HUMAN_IN_THE_LOOP_DESIGN.md` | This consolidated reference document |
| `orchestration/notes/HITL_RESUME_FIX_PLAN.md` | Resume flow bug fix plan (step ID mismatch, trace continuity) |
| `orchestration/notes/HITL_EXPIRY_PROCESSOR_DESIGN.md` | Checkpoint expiry processor design |
| `examples/agent-with-human-approval/HITL_CHAT_ASSISTANT_PLAN.md` | Implementation plan (v3.0) for example agent |
| `examples/agent-with-human-approval/HITL_SCENARIOS_IMPLEMENTATION.md` | Detailed scenario implementation guide |

---

## 5. Files Modified

### Summary of HITL Changes in Existing Files

| File | Key HITL Additions |
|------|-------------------|
| `orchestration/orchestrator.go` | `SetInterruptController()`, `WithResumeMode()`, `WithPlanOverride()`, `WithCompletedSteps()`, `CheckPlanApproval` integration, `original_request_id` trace correlation |
| `orchestration/executor.go` | `SetInterruptController()`, step-level HITL hooks, `WithResolvedParams()`, `WithPreResolvedParams()`, `GetCompletedSteps()` for resume, `ErrInterrupted` propagation |
| `orchestration/interfaces.go` | `StepResult.Metadata` field, `HITLConfig` struct in `OrchestratorConfig`, env var parsing for `GOMIND_HITL_*` |
| `orchestration/executor_test.go` | Tests for HITL step execution and checkpoint handling |

### `orchestration/orchestrator.go`

**Changes**: Added HITL plan approval check to streaming mode

```go
// Lines 1037-1074: HITL Plan Approval Check (streaming mode)
if o.config.HITL.Enabled && o.interruptController != nil {
    checkpoint, err := o.interruptController.CheckPlanApproval(ctx, plan)
    if checkpoint != nil {
        return nil, &ErrInterrupted{
            CheckpointID: checkpoint.CheckpointID,
            Checkpoint:   checkpoint,
        }
    }
}
```

**Changes**: Added context helpers for resume flow

```go
// WithPlanOverride injects a pre-approved plan for resume flows
func WithPlanOverride(ctx context.Context, plan *RoutingPlan) context.Context

// WithCompletedSteps injects already-completed step results
func WithCompletedSteps(ctx context.Context, results map[string]*StepResult) context.Context
```

### `orchestration/executor.go`

**Changes**: Added HITL checkpoint propagation for step-level interrupts

```go
// Lines 550-594: After batch completion, detect HITL checkpoint
for _, result := range stepResults {
    if checkpoint, ok := result.Metadata["hitl_checkpoint"].(*ExecutionCheckpoint); ok {
        // Update checkpoint with completed steps
        e.interruptController.UpdateCheckpointProgress(ctx, checkpoint, completedSteps)
        return nil, &ErrInterrupted{
            CheckpointID: checkpoint.CheckpointID,
            Checkpoint:   checkpoint,
        }
    }
}
```

**Changes**: Store checkpoint in result metadata when HITL triggers

```go
// Lines 1282-1287: Store checkpoint in result metadata
result.Metadata = map[string]interface{}{
    "hitl_checkpoint_id":   checkpoint.CheckpointID,
    "hitl_interrupt_point": string(checkpoint.InterruptPoint),
    "hitl_checkpoint":      checkpoint,
}
```

### `orchestration/interfaces.go`

**Changes**: Added `Metadata` field to `StepResult`

```go
type StepResult struct {
    // ... existing fields ...
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}
```

---

## 6. Key Code Snippets

### ErrInterrupted Error Type

```go
// hitl_errors.go
type ErrInterrupted struct {
    CheckpointID string
    Checkpoint   *ExecutionCheckpoint
}

func (e *ErrInterrupted) Error() string {
    return fmt.Sprintf("execution interrupted at checkpoint %s", e.CheckpointID)
}

// Helper function to check for interruption
func IsInterrupted(err error) bool {
    var interrupted *ErrInterrupted
    return errors.As(err, &interrupted)
}
```

### RuleBasedPolicy Decision Logic

```go
// hitl_policy.go
func (p *RuleBasedPolicy) ShouldApproveBeforeStep(ctx context.Context, step RoutingStep, plan *RoutingPlan) (*InterruptDecision, error) {
    // Check if capability is sensitive
    capability := step.Metadata["capability"].(string)
    for _, sensitive := range p.config.SensitiveCapabilities {
        if capability == sensitive {
            return &InterruptDecision{
                ShouldInterrupt: true,
                Reason:          ReasonSensitiveOperation,
                Message:         fmt.Sprintf("Approval required for sensitive operation: %s", capability),
                Priority:        PriorityHigh,
            }, nil
        }
    }
    return &InterruptDecision{ShouldInterrupt: false}, nil
}
```

### Agent Catching ErrInterrupted

```go
// chat_agent.go:334
result, err := t.orchestrator.ProcessRequestStreaming(ctx, request, callback)
if err != nil {
    if orchestration.IsInterrupted(err) {
        var interrupted *orchestration.ErrInterrupted
        errors.As(err, &interrupted)
        callback.SendCheckpoint(interrupted.Checkpoint)
        return nil
    }
    return fmt.Errorf("orchestration failed: %w", err)
}
```

### SSE Checkpoint Event

```go
// sse_handler.go
func (c *StreamingCallback) SendCheckpoint(checkpoint *orchestration.ExecutionCheckpoint) error {
    data := map[string]interface{}{
        "checkpoint_id":   checkpoint.CheckpointID,
        "request_id":      checkpoint.RequestID,
        "interrupt_point": checkpoint.InterruptPoint,
        "expires_at":      checkpoint.ExpiresAt,
        "status":          checkpoint.Status,
    }
    // Include plan for plan-level checkpoints
    if checkpoint.Plan != nil {
        data["plan"] = checkpoint.Plan
    }
    return c.sendEvent("checkpoint", data)
}
```

---

## 7. HITL Resume Fix

### Bug Description

In chained checkpoint scenarios (plan approval followed by step approval), the resume flow was causing duplicate approval dialogs for the same capability.

**Root Cause**: Resume regenerates plan instead of using stored plan
- Step IDs change between regenerations
- Pre-resolved parameters keyed to wrong step ID
- Completed steps are re-executed

### Solution: Use Stored Plan on Resume

**Context helpers added to `orchestrator.go`**:

```go
// WithPlanOverride injects a pre-approved plan, bypassing LLM planning
func WithPlanOverride(ctx context.Context, plan *RoutingPlan) context.Context

// WithCompletedSteps injects already-completed step results
func WithCompletedSteps(ctx context.Context, results map[string]*StepResult) context.Context
```

**Updated resume handler in `handlers.go`**:

```go
func handleResumeSSE(checkpointID string) {
    checkpoint := loadCheckpoint(checkpointID)

    ctx = orchestration.WithResumeMode(ctx, checkpointID)

    // ALWAYS use stored plan instead of regenerating
    ctx = orchestration.WithPlanOverride(ctx, checkpoint.Plan)

    // For step-level checkpoints, inject completed steps
    if checkpoint.InterruptPoint == orchestration.InterruptPointBeforeStep {
        ctx = orchestration.WithCompletedSteps(ctx, checkpoint.StepResults)
    }

    t.ProcessWithStreaming(ctx, checkpoint.OriginalRequest, callback)
}
```

**Updated executor in `executor.go`**:

```go
func (e *SmartExecutor) Execute(ctx context.Context, plan *RoutingPlan) (*ExecutionResult, error) {
    stepResults := make(map[string]*StepResult)
    executed := make(map[string]bool)

    // Pre-populate with completed steps from checkpoint
    completedSteps := GetCompletedSteps(ctx)
    if completedSteps != nil {
        for stepID, cachedResult := range completedSteps {
            executed[stepID] = true
            stepResults[stepID] = cachedResult
            result.Steps = append(result.Steps, *cachedResult)
        }
    }

    // Main execution loop - automatically skips pre-populated steps
    for len(executed) < len(plan.Steps) {
        readySteps := e.findReadySteps(plan, executed, stepResults)
        // ...
    }
}
```

---

## 8. Trace Continuity Enhancement

### Problem

HITL creates multiple HTTP requests with different `request_id` values. Searching Jaeger by the final `request_id` misses the initial trace.

### Solution: `original_request_id` Propagation

**Header**: `X-Gomind-Original-Request-ID`

**Flow**:
1. Initial request returns `request_id` in checkpoint event
2. UI stores as `originalRequestId`
3. Resume requests include `X-Gomind-Original-Request-ID` header
4. Agent sets `original_request_id` in telemetry baggage
5. All spans have `original_request_id` attribute

**Files changed**:

| File | Change |
|------|--------|
| `orchestration/orchestrator.go` | Set `original_request_id` in baggage for all requests |
| `sse_handler.go` | Include `request_id` in checkpoint SSE event |
| `handlers.go` | Extract header, set baggage on resume |
| `main.go` | CORS fix for custom header (`WithCORSDefaults()`) |
| `hitl.html` | Store and send `originalRequestId`, display for copying |

**Verification**: Search Jaeger by `original_request_id=X` returns all traces:
- Initial `/chat/stream` trace
- All resume traces

---

## 9. Configuration Reference

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GOMIND_HITL_ENABLED` | `false` | Enable HITL system |
| `GOMIND_HITL_REQUIRE_PLAN_APPROVAL` | `false` | Require approval for all plans |
| `GOMIND_HITL_SENSITIVE_CAPABILITIES` | `""` | Comma-separated capabilities requiring approval |
| `GOMIND_HITL_SENSITIVE_AGENTS` | `""` | Comma-separated agents requiring approval |
| `GOMIND_HITL_STEP_SENSITIVE_CAPABILITIES` | `""` | Capabilities requiring step-only approval |
| `GOMIND_HITL_STEP_SENSITIVE_AGENTS` | `""` | Agents requiring step-only approval |
| `GOMIND_HITL_ESCALATE_AFTER_RETRIES` | `3` | Retries before error escalation |
| `GOMIND_HITL_DEFAULT_TIMEOUT` | `5m` | Checkpoint expiration |
| `GOMIND_HITL_REDIS_DB` | `6` | Redis database for checkpoints |
| `GOMIND_AGENT_NAME` | `""` | Agent name for multi-agent key prefix isolation |

### Multi-Agent Key Prefix Isolation

In production deployments with multiple HITL-enabled agents, all agents share the same Redis database (DB 6) but are isolated via key prefixes based on `GOMIND_AGENT_NAME`.

**Key Construction** (from `hitl_checkpoint_store.go:105-108`):
```go
agentName := getEnvOrDefault("GOMIND_AGENT_NAME", "")
keyPrefix := basePrefix  // "gomind:hitl"
if agentName != "" {
    keyPrefix = fmt.Sprintf("%s:%s", basePrefix, agentName)
}
```

**Resulting Key Patterns**:
| Agent Name | Checkpoint Key | Pending Index |
|------------|----------------|---------------|
| `payment-service` | `gomind:hitl:payment-service:checkpoint:cp-xxx` | `gomind:hitl:payment-service:pending` |
| `trading-bot` | `gomind:hitl:trading-bot:checkpoint:cp-xxx` | `gomind:hitl:trading-bot:pending` |
| (not set) | `gomind:hitl:checkpoint:cp-xxx` | `gomind:hitl:pending` |

**Benefits**:
- Each agent operates independently within the same Redis instance
- No cross-talk between agents' checkpoints or Pub/Sub channels
- Simplified infrastructure (single Redis for all HITL agents)

### Example Configuration

**Scenario 1 (Plan Approval)**:
```bash
GOMIND_HITL_ENABLED=true
GOMIND_HITL_REQUIRE_PLAN_APPROVAL=true
```

**Scenario 2 (Step-Only Approval)**:
```bash
GOMIND_HITL_ENABLED=true
GOMIND_HITL_REQUIRE_PLAN_APPROVAL=false
GOMIND_HITL_STEP_SENSITIVE_CAPABILITIES=transfer_funds,delete_account
```

---

## 10. API Reference

### POST /hitl/command

Submit a human decision for a checkpoint.

**Request**:
```json
{
    "checkpoint_id": "cp-xxx",
    "type": "approve"
}
```

**Response**:
```json
{
    "should_resume": true
}
```

**Command Types**: `approve`, `reject`, `edit`, `skip`, `abort`, `retry`

### POST /hitl/resume/{checkpoint_id}

Resume execution after approval (SSE stream).

**Response**: SSE events (`step`, `chunk`, `checkpoint`, `done`, `error`)

### POST /hitl/resume-sync/{checkpoint_id}

Resume execution (synchronous JSON response).

**Response**:
```json
{
    "response": "...",
    "request_id": "...",
    "tool_results": [...]
}
```

---

## 11. Testing

### Manual Testing

```bash
# Step 1: Send request
curl -X POST http://localhost:8352/chat \
  -H "Content-Type: application/json" \
  -d '{"request": "What is the stock price of AAPL?"}'

# Response: interrupted=true, checkpoint_id="cp-xxx"

# Step 2: Approve
curl -X POST http://localhost:8352/hitl/command \
  -H "Content-Type: application/json" \
  -d '{"checkpoint_id": "cp-xxx", "type": "approve"}'

# Step 3: Resume
curl -X POST http://localhost:8352/hitl/resume-sync/cp-xxx
```

### UI Testing

1. Start `agent-with-human-approval` and tools
2. Open `http://localhost:8080/welcome.html`
3. Select "Plan HITL" or "Step HITL" mode
4. Send a request and observe approval dialog
5. Approve and verify execution completes

---

## 12. Quick Reference by Scenario

| Scenario | Key Files and Line References |
|----------|-------------------------------|
| **1. Plan Approval** | `orchestrator.go:790` (non-streaming), `orchestrator.go:1045` (streaming), `hitl_controller.go:226` (`CheckPlanApproval`), `hitl_policy.go:58` (`ShouldApprovePlan`) |
| **2. Step Approval** | `executor.go:1253` (`CheckBeforeStep`), `executor.go:550-594` (propagation), `hitl_controller.go:317` (`CheckBeforeStep`), `hitl_policy.go:124` (`ShouldApproveBeforeStep`) |
| **3. Output Validation** | `executor.go:2024` (`CheckAfterStep`), `hitl_policy.go:166-188` (`ShouldApproveAfterStep`), `hitl_policy.go:243-246` (`requiresOutputValidation` - ⚠️ always returns false) |
| **4. Error Escalation** | `executor.go:2054` (`CheckOnError`), `hitl_policy.go:195-215` (`ShouldEscalateError`) |

---

## Related Documents

- [HUMAN_IN_THE_LOOP_PROPOSAL.md](../HUMAN_IN_THE_LOOP_PROPOSAL.md) - Original design proposal (v1.6)
- [HITL_RESUME_FIX_PLAN.md](./HITL_RESUME_FIX_PLAN.md) - Resume bug fix details (trace continuity, step ID mismatch)
- [HITL_EXPIRY_PROCESSOR_DESIGN.md](./HITL_EXPIRY_PROCESSOR_DESIGN.md) - Checkpoint expiry processor design
- [HITL_SCENARIOS_IMPLEMENTATION.md](../../examples/agent-with-human-approval/HITL_SCENARIOS_IMPLEMENTATION.md) - Detailed scenario documentation
- [HITL_CHAT_ASSISTANT_PLAN.md](../../examples/agent-with-human-approval/HITL_CHAT_ASSISTANT_PLAN.md) - Example agent implementation plan (v3.0)
