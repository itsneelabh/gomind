# HITL Expiry Processor Design

## Table of Contents

- [Problem Statement](#problem-statement)
  - [Observed Behavior](#observed-behavior)
  - [Root Cause](#root-cause)
- [Design Decision: Reject-by-Default on Expiry](#design-decision-reject-by-default-on-expiry)
  - [Rationale](#rationale)
  - [Environment Variable Override](#environment-variable-override)
- [Solution Architecture](#solution-architecture)
  - [Framework vs Application Responsibility](#framework-vs-application-responsibility)
  - [How DefaultAction is Determined](#how-defaultaction-is-determined)
  - [Typical DefaultAction by Scenario](#typical-defaultaction-by-scenario)
- [Key Design Decision: Mode-Aware Expiry](#key-design-decision-mode-aware-expiry)
  - [The Problem: Streaming vs Non-Streaming Have Different User Expectations](#the-problem-streaming-vs-non-streaming-have-different-user-expectations)
  - [The Solution: Request Mode Determines Expiry Behavior](#the-solution-request-mode-determines-expiry-behavior)
  - [Why This Makes Sense](#why-this-makes-sense)
  - [Checkpoint Status Values Explained](#checkpoint-status-values-explained)
  - [User Recovery After Expiry](#user-recovery-after-expiry)
  - [How Request Mode is Tracked](#how-request-mode-is-tracked)
  - [Setting Request Mode in Handlers](#setting-request-mode-in-handlers)
- [Implementation Design](#implementation-design)
  - [New Checkpoint Statuses](#new-checkpoint-statuses)
  - [Expiry Processor Interface](#expiry-processor-interface)
  - [RedisCheckpointStore Implementation](#redischeckpointstore-implementation)
- [Usage Patterns](#usage-patterns)
  - [Streaming Scenario (SSE) - Implicit Deny on Expiry](#streaming-scenario-sse---implicit-deny-on-expiry)
  - [Non-Streaming Scenario (HTTP 202 + Polling) - Policy-Driven Expiry](#non-streaming-scenario-http-202--polling---policy-driven-expiry)
  - [Application-Level Auto-Resume (Non-Streaming Only)](#application-level-auto-resume-non-streaming-only)
- [Configuration](#configuration)
  - [Architectural Decision: Configuration Hierarchy](#architectural-decision-configuration-hierarchy)
  - [Per-Checkpoint Configuration via InterruptDecision](#per-checkpoint-configuration-via-interruptdecision)
  - [Environment Variables](#environment-variables)
  - [Programmatic Configuration](#programmatic-configuration)
- [Developer Workflow](#developer-workflow)
  - [Minimum Required Steps (Both Scenarios)](#minimum-required-steps-both-scenarios)
  - [Scenario 1: Streaming (SSE/WebSocket)](#scenario-1-streaming-ssewebsocket)
  - [Scenario 2: Non-Streaming (HTTP 202 + Polling)](#scenario-2-non-streaming-http-202--polling)
  - [Complete Example: Agent with Both Scenarios](#complete-example-agent-with-both-scenarios)
  - [Summary: Developer Checklist](#summary-developer-checklist)
- [Multi-Agent Considerations](#multi-agent-considerations)
- [Metrics](#metrics)
- [Testing](#testing)
  - [Unit Tests](#unit-tests)
  - [Integration Tests](#integration-tests)
- [Production Considerations](#production-considerations)
  - [Framework Helpers](#framework-helpers)
    - [BuildResumeContext Helper](#1a-buildresumecontext-helper-high)
    - [IsResumableStatus Helper](#1b-isresumablestatus-helper-medium)
    - [Cross-Trace Correlation for Expiry Callbacks](#1c-cross-trace-correlation-for-expiry-callbacks-required)
  - [Distributed Concurrency (Multi-Pod Safety)](#distributed-concurrency-multi-pod-safety)
  - [Callback Error Recovery](#callback-error-recovery)
  - [Graceful Shutdown](#graceful-shutdown)
  - [Configuration Integration](#configuration-integration)
- [Implementation Checklist](#implementation-checklist)
- [Summary: Decision Flow Chart](#summary-decision-flow-chart)
- [Environment Variables Reference](#environment-variables-reference)
  - [Expiry Behavior Variables](#expiry-behavior-variables)
  - [Expiry Processor Variables](#expiry-processor-variables)
  - [Redis Configuration Variables](#redis-configuration-variables)
  - [Configuration Priority](#configuration-priority)
- [Related Documents](#related-documents)

---

## Problem Statement

The HITL (Human-in-the-Loop) system creates checkpoints with `ExpiresAt` timestamps and `DefaultAction` values, but currently lacks a mechanism to automatically process expired checkpoints. When a checkpoint expires:

1. The checkpoint remains in "pending" status indefinitely
2. The pending index retains stale references
3. Streaming clients see "loading" state forever
4. Non-streaming clients polling for status never see resolution

### Observed Behavior

```
Timeline:
22:26:44Z  Checkpoint cp-194ade46-14fe-4b created
           - expires_at: 22:31:44Z (5 min timeout)
           - default_action: approve
           - status: pending

22:31:44Z  Checkpoint EXPIRES
           - Expected: auto-approve and update status
           - Actual: NOTHING HAPPENS

22:34:48Z  User checks status
           - Status still shows "pending"
           - UI shows "loading" indefinitely
```

### Root Cause

From [HUMAN_IN_THE_LOOP_PROPOSAL.md](../HUMAN_IN_THE_LOOP_PROPOSAL.md#L2128-L2133):

> **Limitation**: Timeout only fires if `WaitForCommand` is actively called. There is no background scheduler that auto-executes `DefaultAction` when `ExpiresAt` passes. This is intentional for Phase 3.

The system is non-blocking by design - it returns `ErrInterrupted` immediately rather than blocking. This means no goroutine is waiting on `WaitForCommand` to handle the timeout.

---

## Design Decision: Reject-by-Default on Expiry

> **Decision Date:** 2026-01-24
> **Status:** Implemented

### Rationale

When HITL (Human-in-the-Loop) is enabled with approval required, the explicit intent is that a human
must review and approve actions before they proceed. If a checkpoint expires without human response,
the system should **fail-safe by rejecting** rather than auto-approving.

**Core Principle:** If you've enabled HITL, you want human oversight. Auto-approving on timeout
defeats the purpose of requiring approval in the first place.

**Previous Design (Deprecated):**
| Checkpoint Type | DefaultAction | Rationale (old) |
|-----------------|---------------|-----------------|
| Plan | `approve` | "User-friendly for routine approvals" |
| Step | `reject` | "Fail-safe for sensitive operations" |
| Output validation | `approve` | "Validation usually proceeds" |

**Current Design (Reject-by-Default):**
| Checkpoint Type | DefaultAction | Rationale |
|-----------------|---------------|-----------|
| Plan | `reject` | HITL enabled = require explicit human approval |
| Step | `reject` | HITL enabled = require explicit human approval |
| Output validation | `reject` | HITL enabled = require explicit human approval |
| Error escalation | `abort` | Errors require human attention (unchanged) |

**Why This Change:**
1. **Consistent semantics**: HITL means "human must approve" - expiry without approval = rejection
2. **Security-first**: Sensitive operations should never proceed without explicit approval
3. **Predictable behavior**: All checkpoint types behave the same way on expiry
4. **Explicit opt-in for auto-approve**: Developers who want auto-approve must explicitly configure it

### Environment Variable Override

Developers who need the old auto-approve behavior (e.g., internal tooling, dev environments) can
override using environment variables:

```bash
# Override DefaultAction for all checkpoint types
GOMIND_HITL_DEFAULT_ACTION=approve

# Control what happens on expiry per request mode
GOMIND_HITL_STREAMING_EXPIRY=apply_default    # Apply DefaultAction on streaming expiry
GOMIND_HITL_NON_STREAMING_EXPIRY=apply_default # Apply DefaultAction on non-streaming expiry
```

**Override Behavior Matrix:**

| `GOMIND_HITL_DEFAULT_ACTION` | `*_EXPIRY=apply_default` | Result on Expiry |
|------------------------------|--------------------------|------------------|
| (not set, default=reject) | Yes | Rejected |
| `approve` | Yes | Approved |
| `reject` | Yes | Rejected |
| (any) | `implicit_deny` | Status: `expired`, no action applied |

**Example Configurations:**

```bash
# 1. Strict HITL (default) - all checkpoints reject on expiry
GOMIND_HITL_ENABLED=true
# No override needed - defaults to reject

# 2. Auto-approve for internal tooling
GOMIND_HITL_ENABLED=true
GOMIND_HITL_DEFAULT_ACTION=approve
GOMIND_HITL_STREAMING_EXPIRY=apply_default
GOMIND_HITL_NON_STREAMING_EXPIRY=apply_default

# 3. Strict everywhere - no auto-action, always require manual resume
GOMIND_HITL_ENABLED=true
GOMIND_HITL_STREAMING_EXPIRY=implicit_deny
GOMIND_HITL_NON_STREAMING_EXPIRY=implicit_deny
```

---

## Solution Architecture

### Framework vs Application Responsibility

**Important clarification**: The framework is a library, not a running application. When we say
"framework responsibility", we mean code that the framework **provides** as part of the
`orchestration` package. The application (agent) **imports and runs** this code.

| Responsibility | Owner | What this means |
|----------------|-------|-----------------|
| Provide expiry processor code | **Framework** | `RedisCheckpointStore.StartExpiryProcessor()` method |
| Start the expiry processor | **Application** | Agent calls `checkpointStore.StartExpiryProcessor()` |
| Scan for expired checkpoints | **Framework** | Goroutine provided by `StartExpiryProcessor()` |
| Update checkpoint status | **Framework** | `UpdateCheckpointStatus()` called by processor |
| Provide callback mechanism | **Framework** | `SetExpiryCallback()` method |
| Define what happens on expiry | **Application** | Agent provides callback function |
| Trigger resume execution | **Application** | Callback calls `orchestrator.Resume()` |

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Responsibility Split                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  FRAMEWORK PROVIDES (library code)        APPLICATION RUNS (agent code)     │
│  ─────────────────────────────────        ─────────────────────────────     │
│                                                                              │
│  ✓ StartExpiryProcessor() method          ✓ Calls StartExpiryProcessor()    │
│  ✓ Goroutine that scans pending index     ✓ Provides ExpiryProcessorConfig  │
│  ✓ Mode-aware expiry logic                ✓ Provides ExpiryCallback         │
│  ✓ UpdateCheckpointStatus() calls         ✓ Decides whether to auto-resume  │
│  ✓ SetExpiryCallback() mechanism          ✓ Calls orchestrator.Resume()     │
│                                           ✓ Handles execution result        │
│                                                                              │
│  Framework provides STATE management      Application handles EXECUTION     │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Concrete example of the split:**

```go
// ═══════════════════════════════════════════════════════════════════════════
// FRAMEWORK CODE (in orchestration/hitl_checkpoint_store.go)
// This is library code that the agent imports
// ═══════════════════════════════════════════════════════════════════════════

func (s *RedisCheckpointStore) StartExpiryProcessor(ctx context.Context, config ExpiryProcessorConfig) error {
    // Framework provides: goroutine, scanning, status updates
    go s.expiryProcessorLoop(config)
    return nil
}

func (s *RedisCheckpointStore) SetExpiryCallback(callback ExpiryCallback) error {
    // Framework provides: callback mechanism
    // Returns error if called after processor is started (per FRAMEWORK_DESIGN_PRINCIPLES.md fail-fast)
    if s.expiryStarted {
        return fmt.Errorf("SetExpiryCallback must be called before StartExpiryProcessor")
    }
    s.expiryCallback = callback
    return nil
}

// ═══════════════════════════════════════════════════════════════════════════
// APPLICATION CODE (in agent's main.go or setup.go)
// This is agent-specific code that uses the framework
// ═══════════════════════════════════════════════════════════════════════════

func setupHITL(agent *HITLChatAgent, checkpointStore *orchestration.RedisCheckpointStore) {
    // ┌────────────────────────────────────────────────────────────────────┐
    // │  APPLICATION RESPONSIBILITY: Start the processor                   │
    // │  The framework provides the processor; application starts it.      │
    // └────────────────────────────────────────────────────────────────────┘
    checkpointStore.StartExpiryProcessor(ctx, orchestration.ExpiryProcessorConfig{
        Enabled:      true,
        ScanInterval: 10 * time.Second,
    })

    // ┌────────────────────────────────────────────────────────────────────┐
    // │  APPLICATION RESPONSIBILITY: Define expiry behavior                │
    // │  The framework detects expiry and invokes this callback.           │
    // │  The application decides what to do (resume, notify, log, etc.)    │
    // └────────────────────────────────────────────────────────────────────┘
    checkpointStore.SetExpiryCallback(func(ctx context.Context, cp *orchestration.ExecutionCheckpoint, action orchestration.CommandType) {
        // Application decides: should we auto-resume?
        if !orchestration.IsResumableStatus(cp.Status) || action != orchestration.CommandApprove {
            notifyUser(cp, "Request expired")
            return
        }

        // ┌────────────────────────────────────────────────────────────────┐
        // │  FRAMEWORK HELPER: BuildResumeContext                          │
        // │  Framework prepares the context with all resume state.         │
        // │  Application then uses this context with its own methods.      │
        // └────────────────────────────────────────────────────────────────┘
        resumeCtx, err := orchestration.BuildResumeContext(ctx, cp)
        if err != nil {
            log.Error("Failed to build resume context", "error", err)
            return
        }

        // ┌────────────────────────────────────────────────────────────────┐
        // │  APPLICATION RESPONSIBILITY: Execute the resume                │
        // │  Application calls its own processing method, not framework's. │
        // │  This keeps framework decoupled from application specifics.    │
        // └────────────────────────────────────────────────────────────────┘
        sessionID := cp.UserContext["session_id"].(string)
        agent.ProcessWithStreaming(resumeCtx, sessionID, cp.OriginalRequest, nil)
    })
}
```

### How DefaultAction is Determined

The `DefaultAction` is set by the policy at checkpoint creation time and stored in the checkpoint.

**IMPORTANT:** Per the [Reject-by-Default design decision](#design-decision-reject-by-default-on-expiry),
ALL checkpoint types default to `CommandReject` on expiry. This ensures that:
- HITL enabled = human approval is required
- Expiry without approval = rejection (fail-safe)
- Developers must explicitly opt-in to auto-approve behavior

```go
// From hitl_policy.go - ALL checkpoint types default to reject

// ShouldApprovePlan - plan checkpoints reject on timeout (HITL = require approval)
func (p *RuleBasedPolicy) ShouldApprovePlan(ctx context.Context, plan *RoutingPlan) (*InterruptDecision, error) {
    return &InterruptDecision{
        ShouldInterrupt: true,
        Reason:          ReasonPlanApproval,
        DefaultAction:   CommandReject,  // HITL enabled = require explicit approval
        Timeout:         p.config.DefaultTimeout,
        // ...
    }, nil
}

// ShouldApproveBeforeStep - step checkpoints reject on timeout
func (p *RuleBasedPolicy) ShouldApproveBeforeStep(ctx context.Context, step RoutingStep, plan *RoutingPlan) (*InterruptDecision, error) {
    return &InterruptDecision{
        ShouldInterrupt: true,
        Reason:          ReasonSensitiveOperation,
        DefaultAction:   CommandReject,  // HITL enabled = require explicit approval
        Timeout:         p.config.DefaultTimeout,
        // ...
    }, nil
}

// ShouldApproveAfterStep - output validation rejects on timeout
func (p *RuleBasedPolicy) ShouldApproveAfterStep(ctx context.Context, step RoutingStep, result *StepResult) (*InterruptDecision, error) {
    return &InterruptDecision{
        ShouldInterrupt: true,
        Reason:          ReasonOutputValidation,
        DefaultAction:   CommandReject,  // HITL enabled = require explicit approval
        Timeout:         p.config.DefaultTimeout,
        // ...
    }, nil
}

// From hitl_controller.go - stored in checkpoint
checkpoint := &ExecutionCheckpoint{
    Decision:  decision,  // Contains DefaultAction set by policy
    ExpiresAt: time.Now().Add(decision.Timeout),
    // ...
}
```

**Why explicit is better than fallback:** The checkpoint response shows `default_action` to clients.
If set explicitly, the UI can display "This will auto-reject in 5 min" based on the checkpoint data,
without needing to know the fallback logic.

**Fallback Logic (Safety Net):** If `DefaultAction` is not set by the policy, the expiry processor
uses `CommandReject` as the fallback for all checkpoint types except errors (which use `CommandAbort`).
See `determineExpiryAction()` in `hitl_checkpoint_store.go`.

### Typical DefaultAction by Scenario

| Scenario | InterruptPoint | DefaultAction | Set By | Rationale |
|----------|----------------|---------------|--------|-----------|
| Plan approval | `plan_generated` | `reject` | Policy (`ShouldApprovePlan`) | HITL = require explicit approval |
| Sensitive operation | `before_step` | `reject` | Policy (`ShouldApproveBeforeStep`) | HITL = require explicit approval |
| Output validation | `after_step` | `reject` | Policy (`ShouldApproveAfterStep`) | HITL = require explicit approval |
| Error escalation | `on_error` | `abort` | Policy (`ShouldEscalateError`) | Errors require human attention |

**Note:** The `RuleBasedPolicy` implementation sets `DefaultAction` to `CommandReject` for all
checkpoint types. Developers who need auto-approve behavior must explicitly set
`GOMIND_HITL_DEFAULT_ACTION=approve` (see [Environment Variable Override](#environment-variable-override)).

**Historical Note:** Prior to 2026-01-24, plan and output validation checkpoints defaulted to
`CommandApprove`. This was changed to `CommandReject` to align with HITL semantics: if you've
enabled human oversight, expiry without approval should fail-safe. See
[Design Decision: Reject-by-Default](#design-decision-reject-by-default-on-expiry) for details.

**Override Example:** To restore the old auto-approve behavior for plans:
```bash
GOMIND_HITL_DEFAULT_ACTION=approve
GOMIND_HITL_NON_STREAMING_EXPIRY=apply_default
```

---

## Key Design Decision: Mode-Aware Expiry

### The Problem: Streaming vs Non-Streaming Have Different User Expectations

Consider two scenarios when a checkpoint expires after 5 minutes:

**Scenario A: Streaming (SSE) - User is present**
```
00:00  User: "Transfer $5000 to savings"
00:01  SSE connection established
00:01  Checkpoint created, approval dialog shown on screen
00:02  User gets a phone call, walks away from computer
00:06  Checkpoint expires (5 min timeout)

       Question: What should happen?

       The user SAW the approval dialog but didn't act.
       This could mean:
       - They got distracted (will come back)
       - They're hesitant/unsure
       - They intentionally didn't approve

       Safe assumption: Don't proceed without explicit approval
```

**Scenario B: Non-Streaming (HTTP 202 + Polling) - User may not be watching**
```
00:00  User: "Generate weekly report"
00:01  HTTP 202 returned with task ID
00:01  Checkpoint created (plan approval required)
00:01  User closes laptop, goes to meeting
00:06  Checkpoint expires (5 min timeout)

       Question: What should happen?

       The user submitted an async task and walked away.
       They may EXPECT the system to proceed autonomously.

       Reasonable assumption: Apply configured DefaultAction
```

### The Solution: Request Mode Determines Expiry Behavior (Configurable)

The framework provides **sensible defaults** for each request mode, but developers can override
the streaming behavior via policy configuration:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Mode-Aware Expiry Behavior (Fully Configurable)           │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  REQUEST MODE         DEFAULT BEHAVIOR       CONFIGURABLE VIA                │
│  ────────────         ────────────────       ──────────────                  │
│                                                                              │
│  Streaming (SSE)      Implicit DENY          StreamingExpiryBehavior         │
│                       Status → "expired"     Options:                        │
│                       No action applied.       - "implicit_deny" (default)   │
│                                                - "apply_default" (use policy)│
│                                                                              │
│  Non-Streaming        Apply DefaultAction    NonStreamingExpiryBehavior      │
│  (HTTP 202)           Status → "expired_X"   Options:                        │
│                       (X = action applied)     - "apply_default" (default)   │
│                                                - "implicit_deny" (no action) │
│                                                                              │
│  (not set)            Defaults to            DefaultRequestMode              │
│                       non_streaming          Logs WARN + adds trace event    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Why make BOTH behaviors configurable?**

Different applications have different requirements:

- **Financial apps**: May want implicit deny for BOTH streaming and non-streaming
- **Internal tools**: May want apply_default for BOTH (consistent auto-approve)
- **Batch processing**: May want non-streaming to also require manual intervention
- **Chatbots**: May want consistent behavior regardless of request mode

The framework provides sensible defaults but gives developers full control:

| Use Case | Streaming | Non-Streaming |
|----------|-----------|---------------|
| Safety-critical (banking) | `implicit_deny` | `implicit_deny` |
| Internal automation | `apply_default` | `apply_default` |
| Mixed (current default) | `implicit_deny` | `apply_default` |

**RequestMode not set - Observable Default Behavior**

When `RequestMode` is not set in the context (developer forgot to call `WithRequestMode`),
the framework:

1. **Defaults to `non_streaming`** (or configured `DefaultRequestMode`)
2. **Logs a WARN** with context so it's visible in logs
3. **Adds a span event** so it's visible in distributed tracing (Jaeger)
4. **Documents this** in the user guide for reference

This ensures developers are aware when they're relying on defaults.

### Why This Makes Sense

| Aspect | Streaming | Non-Streaming |
|--------|-----------|---------------|
| **User presence** | Actively waiting (SSE connection open) | Fire-and-forget (polling optional) |
| **Dialog visibility** | User definitely saw it | User may not have seen it |
| **User intent on no response** | Unclear - could be distraction or intentional | Likely trusting the system |
| **Safe default** | Don't proceed (implicit deny) | Follow configured policy |
| **Recovery path** | User clicks "Resume" when they return | Automatic if DefaultAction=approve |

### Checkpoint Status Values Explained

```go
// Human-initiated statuses (explicit user action)
"pending"    // Waiting for human response
"approved"   // Human clicked "Approve"
"rejected"   // Human clicked "Reject"
"edited"     // Human modified the plan and approved
"aborted"    // Human clicked "Abort"
"completed"  // Execution finished after approval

// Expiry statuses - STREAMING requests (implicit deny)
"expired"    // Checkpoint expired, NO action was applied
             // User must manually resume if desired
             // This is the SAFE default for streaming

// Expiry statuses - NON-STREAMING requests (policy-driven)
"expired_approved"  // DefaultAction=approve was auto-applied
"expired_rejected"  // DefaultAction=reject was auto-applied
"expired_aborted"   // DefaultAction=abort was auto-applied
```

### User Recovery After Expiry

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    User Recovery Paths After Expiry                          │
└─────────────────────────────────────────────────────────────────────────────┘

  STREAMING USER (status: "expired")
  ──────────────────────────────────

  User returns to computer, sees:
  ┌────────────────────────────────────────────┐
  │  ⚠️ Request Expired                         │
  │                                            │
  │  Your approval request timed out.          │
  │  The action was NOT performed.             │
  │                                            │
  │  [Resume Request]  [Dismiss]               │
  └────────────────────────────────────────────┘

  Clicking "Resume Request" → Opens new SSE stream → Continues execution


  NON-STREAMING USER (status: "expired_approved")
  ───────────────────────────────────────────────

  User polls for status, sees:
  {
    "checkpoint_id": "cp-xxx",
    "status": "expired_approved",
    "message": "Auto-approved after timeout per policy"
  }

  Execution may have already continued (if callback triggered resume)
```

### How Request Mode is Tracked

```go
// New field in ExecutionCheckpoint
type ExecutionCheckpoint struct {
    // ... existing fields ...

    // RequestMode indicates how the original request was submitted
    // This determines expiry behavior:
    // - "streaming": Implicit deny on expiry (user saw the dialog)
    // - "non_streaming": Apply DefaultAction on expiry
    RequestMode RequestMode `json:"request_mode,omitempty"`
}

type RequestMode string

const (
    // RequestModeStreaming indicates the user is actively connected (SSE/WebSocket)
    // On expiry: Implicit deny - checkpoint marked as "expired", no action applied
    // Rationale: User saw the approval dialog but didn't act
    RequestModeStreaming RequestMode = "streaming"

    // RequestModeNonStreaming indicates async submission (HTTP 202 + polling)
    // On expiry: Apply configured DefaultAction from policy
    // Rationale: User may expect autonomous processing
    RequestModeNonStreaming RequestMode = "non_streaming"
)
```

### Setting Request Mode in Handlers

```go
// SSE Handler - User is actively connected
func handleStreamingRequest(w http.ResponseWriter, r *http.Request) {
    // Mark this as a streaming request
    ctx := WithRequestMode(r.Context(), RequestModeStreaming)

    // Process request - any checkpoints created will have request_mode: "streaming"
    result, err := orchestrator.ProcessRequest(ctx, request, metadata)
    // ...
}

// HTTP 202 Handler - Async submission
func handleAsyncRequest(w http.ResponseWriter, r *http.Request) {
    // Mark this as a non-streaming request
    ctx := WithRequestMode(r.Context(), RequestModeNonStreaming)

    // Process request - any checkpoints created will have request_mode: "non_streaming"
    result, err := orchestrator.ProcessRequest(ctx, request, metadata)
    // ...
}
```

---

## Implementation Design

The framework provides all the machinery for expiry processing as library code. The agent
imports and uses this code, deciding when to start the processor and what to do on expiry.

### New Checkpoint Statuses

Add new statuses to distinguish human actions from automatic expiry:

```go
// In hitl_interfaces.go
const (
    // Human-initiated statuses
    CheckpointStatusPending         CheckpointStatus = "pending"
    CheckpointStatusApproved        CheckpointStatus = "approved"           // Human approved
    CheckpointStatusRejected        CheckpointStatus = "rejected"           // Human rejected
    CheckpointStatusEdited          CheckpointStatus = "edited"             // Human edited
    CheckpointStatusAborted         CheckpointStatus = "aborted"            // Human aborted
    CheckpointStatusCompleted       CheckpointStatus = "completed"          // Execution completed

    // NEW: Expiry status for STREAMING requests (implicit deny)
    CheckpointStatusExpired         CheckpointStatus = "expired"            // No action applied, user must resume

    // NEW: Expiry statuses for NON-STREAMING requests (policy-driven)
    CheckpointStatusExpiredApproved CheckpointStatus = "expired_approved"   // Auto-approved on timeout
    CheckpointStatusExpiredRejected CheckpointStatus = "expired_rejected"   // Auto-rejected on timeout
    CheckpointStatusExpiredAborted  CheckpointStatus = "expired_aborted"    // Auto-aborted on timeout
)

// RequestMode indicates how the original request was submitted
type RequestMode string

const (
    RequestModeStreaming    RequestMode = "streaming"     // SSE/WebSocket - user is waiting
    RequestModeNonStreaming RequestMode = "non_streaming" // HTTP 202 - async submission
)
```

### Updated InterruptDecision with Expiry Behavior Configuration

```go
// In hitl_interfaces.go

// InterruptDecision captures the policy's decision at checkpoint creation.
// All expiry behavior settings are copied from the policy config so that
// the expiry processor knows how to handle this specific checkpoint.
type InterruptDecision struct {
    ShouldInterrupt bool
    Reason          InterruptReason
    Message         string

    // DefaultAction is applied when the checkpoint expires (when behavior is "apply_default")
    DefaultAction CommandType

    // Timeout is how long to wait before expiry
    Timeout time.Duration

    // StreamingExpiryBehavior controls what happens when a STREAMING request expires.
    //   - "implicit_deny" (default): No action applied, user must resume
    //   - "apply_default": Apply DefaultAction
    StreamingExpiryBehavior StreamingExpiryBehavior

    // NonStreamingExpiryBehavior controls what happens when a NON-STREAMING request expires.
    //   - "apply_default" (default): Apply DefaultAction
    //   - "implicit_deny": No action applied, user must resume
    NonStreamingExpiryBehavior NonStreamingExpiryBehavior

    // DefaultRequestMode is used when RequestMode is not set in the context.
    //   - "non_streaming" (default): Treat as async request
    //   - "streaming": Treat as live connection
    // NOTE: When this default is used, a WARN log is emitted and a trace event is added.
    DefaultRequestMode RequestMode
}
```

### Expiry Processor Interface

The framework provides these types and methods. The agent uses them to configure and control
the expiry processor.

```go
// In hitl_interfaces.go

// ExpiryProcessorConfig configures the background expiry processor.
// The agent passes this when calling StartExpiryProcessor().
type ExpiryProcessorConfig struct {
    Enabled      bool          // Whether to run the processor (default: true)
    ScanInterval time.Duration // How often to scan (default: 10s)
    BatchSize    int           // Max checkpoints per scan (default: 100)
}

// NOTE: Circuit breaker is NOT part of config - it's an injected dependency.
// Per ARCHITECTURE.md Section 9: "Circuit breaker is provided by application, not framework"
// Use WithExpiryCircuitBreaker(cb core.CircuitBreaker) to inject a circuit breaker.
// See RedisCheckpointStore struct below for the injection point.

// ExpiryCallback is called when a checkpoint expires.
// The agent provides this callback to define what happens on expiry.
// This is where the agent decides whether to auto-resume, notify users, etc.
type ExpiryCallback func(ctx context.Context, checkpoint *ExecutionCheckpoint, appliedAction CommandType)

// CheckpointStore interface additions
type CheckpointStore interface {
    // Existing methods...
    SaveCheckpoint(ctx context.Context, checkpoint *ExecutionCheckpoint) error
    LoadCheckpoint(ctx context.Context, checkpointID string) (*ExecutionCheckpoint, error)
    UpdateCheckpointStatus(ctx context.Context, checkpointID string, status CheckpointStatus) error
    ListPendingCheckpoints(ctx context.Context, filter CheckpointFilter) ([]*ExecutionCheckpoint, error)
    DeleteCheckpoint(ctx context.Context, checkpointID string) error

    // NEW: Expiry processor methods (agent calls these during setup)
    StartExpiryProcessor(ctx context.Context, config ExpiryProcessorConfig) error
    StopExpiryProcessor(ctx context.Context) error
    SetExpiryCallback(callback ExpiryCallback) error
}
```

### RedisCheckpointStore Implementation

This is the framework library code that provides the expiry processor. The agent imports
`orchestration.RedisCheckpointStore` and calls its methods.

> **Architecture Alignment**: This design follows the patterns from `orchestration/ARCHITECTURE.md`:
> - Three-Layer Resilience Architecture (Section 9)
> - Dependency Injection Pattern (Section 4)
> - Progressive Enhancement (Section 2.3)

```go
// In hitl_checkpoint_store.go (FRAMEWORK CODE)

// RedisCheckpointStore implements CheckpointStore with Redis backend.
// Per ARCHITECTURE.md, optional dependencies are injected, not created internally.
type RedisCheckpointStore struct {
    // Required fields
    client    *redis.Client
    keyPrefix string
    ttl       time.Duration
    redisURL  string

    // Optional dependencies (injected via CheckpointStoreDependencies)
    // Per ARCHITECTURE.md Section 4: "Dependency Injection Pattern"
    circuitBreaker core.CircuitBreaker // Layer 2 resilience - nil means use Layer 1
    logger         core.Logger         // Structured logging - nil safe
    telemetry      core.Telemetry      // For StartSpan/RecordMetric (reserved for future use)
    // NOTE: Counter/AddSpanEvent use global telemetry functions (per hitl_controller.go pattern)

    // Expiry processor configuration and state
    config         ExpiryProcessorConfig
    instanceID     string              // For distributed claim mechanism
    expiryCtx      context.Context
    expiryCancel   context.CancelFunc
    expiryCallback ExpiryCallback
    expiryWg       sync.WaitGroup
}

// CheckpointStoreDependencies holds optional dependencies for RedisCheckpointStore.
// Per ARCHITECTURE.md: "Application layer wires everything together"
type CheckpointStoreDependencies struct {
    // Optional - Layer 2 resilience (if nil, uses built-in Layer 1)
    CircuitBreaker core.CircuitBreaker

    // Optional - structured logging (nil safe)
    Logger core.Logger

    // Optional - telemetry for observability (nil safe)
    Telemetry core.Telemetry
}

// ═══════════════════════════════════════════════════════════════════════════
// Factory Functions - Progressive Enhancement Pattern
// Per ARCHITECTURE.md Section 2.3: "Start simple, add complexity as needed"
//
// NOTE: The existing codebase uses functional options pattern:
//   NewRedisCheckpointStore(opts ...interface{})
// The design below shows a PROPOSED alternative pattern with explicit types.
// Implementation should align with the existing pattern or migrate consistently.
// See "Instance ID Configuration" section for functional options approach.
// ═══════════════════════════════════════════════════════════════════════════

// Level 1: Simple creation (development/testing)
func NewRedisCheckpointStore(redisURL, keyPrefix string) (*RedisCheckpointStore, error) {
    return CreateCheckpointStore(redisURL, keyPrefix, nil)
}

// Level 2: Full production setup with dependencies
func CreateCheckpointStore(redisURL, keyPrefix string, deps *CheckpointStoreDependencies) (*RedisCheckpointStore, error) {
    client := redis.NewClient(&redis.Options{
        Addr: redisURL,
        DB:   core.RedisDBHITL,
    })

    store := &RedisCheckpointStore{
        client:    client,
        keyPrefix: keyPrefix,
        redisURL:  redisURL,
        ttl:       24 * time.Hour,
    }

    // Apply optional dependencies if provided
    if deps != nil {
        store.circuitBreaker = deps.CircuitBreaker
        store.logger = deps.Logger
        store.telemetry = deps.Telemetry
    }

    // Generate default instance ID for distributed claims
    store.instanceID = generateInstanceID()

    return store, nil
}

// generateInstanceID creates a unique identifier for this instance.
// Used for distributed claim mechanism in multi-pod deployments.
func generateInstanceID() string {
    hostname, _ := os.Hostname()
    return fmt.Sprintf("%s-%s", hostname, uuid.New().String()[:8])
}

// StartExpiryProcessor starts the background goroutine that processes expired checkpoints.
// The AGENT calls this method during setup - the framework just provides the implementation.
func (s *RedisCheckpointStore) StartExpiryProcessor(ctx context.Context, config ExpiryProcessorConfig) error {
    if !config.Enabled {
        return nil
    }

    if config.ScanInterval == 0 {
        config.ScanInterval = 10 * time.Second
    }
    if config.BatchSize == 0 {
        config.BatchSize = 100
    }

    s.expiryCtx, s.expiryCancel = context.WithCancel(ctx)

    s.expiryWg.Add(1)
    go s.expiryProcessorLoop(config)

    if s.logger != nil {
        // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 2: operation field required
        s.logger.Info("HITL expiry processor started", map[string]interface{}{
            "operation":     "hitl_expiry_processor_start",
            "scan_interval": config.ScanInterval.String(),
            "batch_size":    config.BatchSize,
            "key_prefix":    s.keyPrefix,
        })
    }

    return nil
}

func (s *RedisCheckpointStore) expiryProcessorLoop(config ExpiryProcessorConfig) {
    defer s.expiryWg.Done()

    ticker := time.NewTicker(config.ScanInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            s.processExpiredCheckpoints(config.BatchSize)
        case <-s.expiryCtx.Done():
            return
        }
    }
}

func (s *RedisCheckpointStore) processExpiredCheckpoints(batchSize int) {
    // Track scan duration for metrics (per hitl_metrics.go helper pattern)
    scanStartTime := time.Now()

    ctx, cancel := context.WithTimeout(s.expiryCtx, 30*time.Second)
    defer cancel()

    // Get all checkpoint IDs from pending index
    pendingKey := fmt.Sprintf("%s:pending", s.keyPrefix)
    checkpointIDs, err := s.client.SMembers(ctx, pendingKey).Result()
    if err != nil {
        // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 4: Record error on span
        telemetry.RecordSpanError(ctx, err)

        if s.logger != nil {
            // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 2: operation field required
            s.logger.WarnWithContext(ctx, "Failed to read pending index", map[string]interface{}{
                "operation": "hitl_expiry_processor",
                "error":     err.Error(),
            })
        }
        RecordExpiryScanSkipped("read_pending_index_failed")
        return
    }

    now := time.Now()
    processed := 0

    for _, cpID := range checkpointIDs {
        if processed >= batchSize {
            break
        }

        checkpoint, err := s.LoadCheckpoint(ctx, cpID)
        if err != nil {
            // Checkpoint may have been deleted (TTL) - remove from index
            s.client.SRem(ctx, pendingKey, cpID)
            continue
        }

        // Check if expired
        if checkpoint.Status != CheckpointStatusPending || checkpoint.ExpiresAt.After(now) {
            continue
        }

        // ┌────────────────────────────────────────────────────────────────┐
        // │  MODE-AWARE EXPIRY BEHAVIOR (FULLY CONFIGURABLE)               │
        // │                                                                │
        // │  Both streaming and non-streaming behaviors are configurable:  │
        // │  - Streaming: StreamingExpiryBehavior (default: implicit_deny) │
        // │  - Non-streaming: NonStreamingExpiryBehavior (default: apply)  │
        // │  - No mode set: Uses DefaultRequestMode + logs WARN            │
        // └────────────────────────────────────────────────────────────────┘

        var newStatus CheckpointStatus
        var appliedAction CommandType

        // Get effective request mode (with observable default behavior)
        effectiveMode := s.getEffectiveRequestMode(ctx, checkpoint)

        // Determine expiry behavior based on mode
        shouldApplyDefault := s.shouldApplyDefaultAction(checkpoint, effectiveMode)

        if !shouldApplyDefault {
            // ────────────────────────────────────────────────────────────
            // IMPLICIT DENY (no action applied)
            // ────────────────────────────────────────────────────────────
            // The configured behavior is to NOT apply any default action.
            // The checkpoint is marked as "expired" and the user must
            // manually resume if they want to proceed.
            //
            // This happens when:
            //   - Streaming request with StreamingExpiryBehavior="implicit_deny"
            //   - Non-streaming request with NonStreamingExpiryBehavior="implicit_deny"
            // ────────────────────────────────────────────────────────────

            newStatus = CheckpointStatusExpired  // No action suffix = no action applied
            appliedAction = ""                   // Explicitly no action

            if s.logger != nil {
                s.logger.InfoWithContext(ctx, "Checkpoint expired (implicit deny)", map[string]interface{}{
                    "operation":      "hitl_expiry_processor",
                    "checkpoint_id":  cpID,
                    "request_id":     checkpoint.RequestID,
                    "request_mode":   string(effectiveMode),
                    "expired_at":     checkpoint.ExpiresAt.Format(time.RFC3339),
                    "note":           "User must manually resume if desired. See HUMAN_IN_THE_LOOP_USER_GUIDE.md for configuration.",
                })
            }

            // Add span event for tracing visibility
            // Uses global telemetry functions (internally nil-safe per telemetry/trace_context.go)
            // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 6: request_id must be first attribute
            telemetry.AddSpanEvent(ctx, "hitl.checkpoint.expired",
                attribute.String("request_id", checkpoint.RequestID),
                attribute.String("checkpoint_id", cpID),
                attribute.String("request_mode", string(effectiveMode)),
                attribute.String("action", "implicit_deny"),
                attribute.String("new_status", string(newStatus)),
            )

            // Record metric using helper function (per hitl_metrics.go pattern)
            RecordCheckpointExpired("implicit_deny", effectiveMode, checkpoint.InterruptPoint)

        } else {
            // ────────────────────────────────────────────────────────────
            // APPLY DEFAULT ACTION
            // ────────────────────────────────────────────────────────────
            // The configured behavior is to apply the policy's DefaultAction.
            //
            // This happens when:
            //   - Streaming request with StreamingExpiryBehavior="apply_default"
            //   - Non-streaming request with NonStreamingExpiryBehavior="apply_default"
            // ────────────────────────────────────────────────────────────
            appliedAction = s.determineExpiryAction(checkpoint)
            newStatus = s.actionToExpiredStatus(appliedAction)

            if s.logger != nil {
                s.logger.InfoWithContext(ctx, "Checkpoint expired, applied default action", map[string]interface{}{
                    "operation":      "hitl_expiry_processor",
                    "checkpoint_id":  cpID,
                    "request_id":     checkpoint.RequestID,
                    "request_mode":   string(effectiveMode),
                    "default_action": string(appliedAction),
                    "new_status":     string(newStatus),
                    "expired_at":     checkpoint.ExpiresAt.Format(time.RFC3339),
                })
            }

            // Add span event for tracing visibility
            // Uses global telemetry functions (internally nil-safe per telemetry/trace_context.go)
            // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 6: request_id must be first attribute
            telemetry.AddSpanEvent(ctx, "hitl.checkpoint.expired",
                attribute.String("request_id", checkpoint.RequestID),
                attribute.String("checkpoint_id", cpID),
                attribute.String("request_mode", string(effectiveMode)),
                attribute.String("action", string(appliedAction)),
                attribute.String("new_status", string(newStatus)),
            )

            // Record metric using helper function (per hitl_metrics.go pattern)
            RecordCheckpointExpired(string(appliedAction), effectiveMode, checkpoint.InterruptPoint)
        }

        // Update checkpoint status (removes from pending index)
        if err := s.UpdateCheckpointStatus(ctx, cpID, newStatus); err != nil {
            // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 4: Record error on span
            telemetry.RecordSpanError(ctx, err)

            if s.logger != nil {
                // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 2: operation field required
                s.logger.WarnWithContext(ctx, "Failed to update expired checkpoint", map[string]interface{}{
                    "operation":     "hitl_expiry_processor",
                    "checkpoint_id": cpID,
                    "request_id":    checkpoint.RequestID,
                    "error":         err.Error(),
                })
            }
            continue
        }

        // Invoke callback if set
        // Note: For streaming requests, callback is still called but with empty action
        // This allows the application to notify the user (e.g., send SSE event)
        if s.expiryCallback != nil {
            checkpoint.Status = newStatus
            s.expiryCallback(ctx, checkpoint, appliedAction)
        }

        processed++
    }

    if processed > 0 && s.logger != nil {
        // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 2: operation field required
        s.logger.DebugWithContext(ctx, "Expiry processor scan complete", map[string]interface{}{
            "operation": "hitl_expiry_processor",
            "processed": processed,
            "total":     len(checkpointIDs),
        })
    }

    // ════════════════════════════════════════════════════════════════════════
    // SCAN METRICS (per hitl_metrics.go helper pattern)
    // These metrics enable monitoring of expiry processor health and throughput
    // ════════════════════════════════════════════════════════════════════════

    // Record scan duration (histogram for latency distribution analysis)
    scanDuration := time.Since(scanStartTime).Seconds()
    RecordExpiryScanDuration(scanDuration)

    // Record batch size (gauge for throughput monitoring)
    RecordExpiryBatchSize(processed)
}

func (s *RedisCheckpointStore) determineExpiryAction(checkpoint *ExecutionCheckpoint) CommandType {
    // Use DefaultAction from the checkpoint's decision
    if checkpoint.Decision != nil && checkpoint.Decision.DefaultAction != "" {
        return checkpoint.Decision.DefaultAction
    }

    // Fallback: HITL enabled = require explicit approval, so reject on expiry
    // Only errors use abort (they need special handling, not just rejection)
    switch checkpoint.InterruptPoint {
    case InterruptPointOnError:
        return CommandAbort // Errors require human attention
    default:
        return CommandReject // All other checkpoints fail-safe to reject
    }
}

func (s *RedisCheckpointStore) actionToExpiredStatus(action CommandType) CheckpointStatus {
    switch action {
    case CommandApprove:
        return CheckpointStatusExpiredApproved
    case CommandReject:
        return CheckpointStatusExpiredRejected
    case CommandAbort:
        return CheckpointStatusExpiredAborted
    default:
        return CheckpointStatusExpiredRejected
    }
}

// getEffectiveRequestMode returns the request mode, using default if not set.
// When the default is used, it logs a WARN and adds a trace event for visibility.
func (s *RedisCheckpointStore) getEffectiveRequestMode(ctx context.Context, checkpoint *ExecutionCheckpoint) RequestMode {
    // If RequestMode is set on the checkpoint, use it
    if checkpoint.RequestMode != "" {
        return checkpoint.RequestMode
    }

    // RequestMode not set - use default and WARN
    defaultMode := s.getDefaultRequestMode(checkpoint)

    // ┌────────────────────────────────────────────────────────────────┐
    // │  OBSERVABLE DEFAULT BEHAVIOR                                   │
    // │                                                                │
    // │  When RequestMode is not set, we:                              │
    // │  1. Use the configured default (or non_streaming)              │
    // │  2. Log a WARN so it's visible in logs                         │
    // │  3. Add a span event so it's visible in Jaeger                 │
    // │                                                                │
    // │  This ensures developers are aware when relying on defaults.   │
    // │  See HUMAN_IN_THE_LOOP_USER_GUIDE.md for configuration details.             │
    // └────────────────────────────────────────────────────────────────┘

    if s.logger != nil {
        s.logger.WarnWithContext(ctx, "RequestMode not set, using default behavior", map[string]interface{}{
            "operation":           "hitl_expiry_processor",
            "checkpoint_id":       checkpoint.CheckpointID,
            "request_id":          checkpoint.RequestID,
            "default_request_mode": string(defaultMode),
            "note":                "Set RequestMode via WithRequestMode(ctx, mode) to avoid this warning. See HUMAN_IN_THE_LOOP_USER_GUIDE.md for details.",
        })
    }

    // Add span event for tracing visibility (Jaeger, etc.)
    // Global telemetry functions are internally nil-safe
    // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 6: request_id must be first attribute
    telemetry.AddSpanEvent(ctx, "hitl.request_mode.default_used",
        attribute.String("request_id", checkpoint.RequestID),
        attribute.String("checkpoint_id", checkpoint.CheckpointID),
        attribute.String("default_request_mode", string(defaultMode)),
        attribute.String("note", "RequestMode not set in context, using default. See HUMAN_IN_THE_LOOP_USER_GUIDE.md."),
    )

    return defaultMode
}

// getDefaultRequestMode returns the configured default request mode.
func (s *RedisCheckpointStore) getDefaultRequestMode(checkpoint *ExecutionCheckpoint) RequestMode {
    // Priority 1: Check checkpoint's decision (captured from policy)
    if checkpoint.Decision != nil && checkpoint.Decision.DefaultRequestMode != "" {
        return checkpoint.Decision.DefaultRequestMode
    }

    // Priority 2: Check environment variable
    if envMode := os.Getenv("GOMIND_HITL_DEFAULT_REQUEST_MODE"); envMode != "" {
        switch envMode {
        case "streaming":
            return RequestModeStreaming
        case "non_streaming":
            return RequestModeNonStreaming
        }
    }

    // Priority 3: Default to non_streaming (async behavior)
    return RequestModeNonStreaming
}

// shouldApplyDefaultAction determines whether to apply DefaultAction or use implicit deny.
// This is the unified decision point for both streaming and non-streaming requests.
func (s *RedisCheckpointStore) shouldApplyDefaultAction(checkpoint *ExecutionCheckpoint, mode RequestMode) bool {
    if mode == RequestModeStreaming {
        behavior := s.getStreamingExpiryBehavior(checkpoint)
        return behavior == StreamingExpiryApplyDefault
    } else {
        behavior := s.getNonStreamingExpiryBehavior(checkpoint)
        return behavior == NonStreamingExpiryApplyDefault
    }
}

// getStreamingExpiryBehavior returns the configured streaming expiry behavior.
func (s *RedisCheckpointStore) getStreamingExpiryBehavior(checkpoint *ExecutionCheckpoint) StreamingExpiryBehavior {
    // Priority 1: Check checkpoint's decision (captured from policy)
    if checkpoint.Decision != nil && checkpoint.Decision.StreamingExpiryBehavior != "" {
        return checkpoint.Decision.StreamingExpiryBehavior
    }

    // Priority 2: Check environment variable
    if envBehavior := os.Getenv("GOMIND_HITL_STREAMING_EXPIRY"); envBehavior != "" {
        switch envBehavior {
        case "apply_default":
            return StreamingExpiryApplyDefault
        case "implicit_deny":
            return StreamingExpiryImplicitDeny
        }
    }

    // Priority 3: Default to implicit_deny (safest for streaming)
    return StreamingExpiryImplicitDeny
}

// getNonStreamingExpiryBehavior returns the configured non-streaming expiry behavior.
func (s *RedisCheckpointStore) getNonStreamingExpiryBehavior(checkpoint *ExecutionCheckpoint) NonStreamingExpiryBehavior {
    // Priority 1: Check checkpoint's decision (captured from policy)
    if checkpoint.Decision != nil && checkpoint.Decision.NonStreamingExpiryBehavior != "" {
        return checkpoint.Decision.NonStreamingExpiryBehavior
    }

    // Priority 2: Check environment variable
    if envBehavior := os.Getenv("GOMIND_HITL_NON_STREAMING_EXPIRY"); envBehavior != "" {
        switch envBehavior {
        case "implicit_deny":
            return NonStreamingExpiryImplicitDeny
        case "apply_default":
            return NonStreamingExpiryApplyDefault
        }
    }

    // Priority 3: Default to apply_default (expected for async requests)
    return NonStreamingExpiryApplyDefault
}

func (s *RedisCheckpointStore) StopExpiryProcessor(ctx context.Context) error {
    if s.expiryCancel != nil {
        s.expiryCancel()

        // Wait with context timeout
        done := make(chan struct{})
        go func() {
            s.expiryWg.Wait()
            close(done)
        }()

        select {
        case <-done:
            return nil
        case <-ctx.Done():
            return fmt.Errorf("expiry processor shutdown cancelled: %w", ctx.Err())
        }
    }
    return nil
}

func (s *RedisCheckpointStore) SetExpiryCallback(callback ExpiryCallback) error {
    // Per FRAMEWORK_DESIGN_PRINCIPLES.md: fail-fast for configuration errors
    if s.expiryStarted {
        return fmt.Errorf("SetExpiryCallback must be called before StartExpiryProcessor")
    }
    s.expiryCallback = callback
    return nil
}
```

---

## Usage Patterns

These examples show **agent code** that uses the framework's expiry processor.

### Streaming Scenario (SSE) - Implicit Deny on Expiry

In streaming scenarios, the user is actively connected and saw the approval dialog.
If they don't respond, we assume they either got distracted or chose not to approve.
The checkpoint is marked as "expired" (no action applied) and the user can manually resume later.

```go
// AGENT CODE: In agent's SSE handler setup

// 1. Mark incoming requests as streaming
func handleStreamingRequest(w http.ResponseWriter, r *http.Request) {
    // IMPORTANT: Mark this as a streaming request
    // This determines expiry behavior: streaming = implicit deny
    ctx := orchestration.WithRequestMode(r.Context(), orchestration.RequestModeStreaming)

    // Set up SSE...
    flusher, _ := w.(http.Flusher)

    // Process request - checkpoints will have request_mode: "streaming"
    result, err := orchestrator.ProcessRequest(ctx, request, metadata)

    // Handle ErrInterrupted...
}

// 2. Set callback to notify user when their checkpoint expires
checkpointStore.SetExpiryCallback(func(ctx context.Context, cp *ExecutionCheckpoint, action CommandType) {
    // For streaming requests, action will be empty (implicit deny)
    // We still want to notify the user so they can resume if desired

    eventType := "checkpoint_expired"
    message := "Request expired. Click 'Resume' to continue."

    if cp.RequestMode == RequestModeStreaming {
        // Streaming: No action was applied, user must manually resume
        message = "Your request timed out. No action was taken. Click 'Resume' if you still want to proceed."
    } else if action == CommandApprove {
        // Non-streaming: Auto-approved
        message = "Request auto-approved after timeout. Execution continuing."
        eventType = "checkpoint_expired_approved"
    }

    // Publish to Redis Pub/Sub for SSE handlers to pick up
    event := map[string]interface{}{
        "type":           eventType,
        "checkpoint_id":  cp.CheckpointID,
        "request_id":     cp.RequestID,
        "applied_action": string(action),  // Empty for streaming
        "new_status":     string(cp.Status),
        "message":        message,
        "can_resume":     true,  // User can always manually resume
    }
    data, _ := json.Marshal(event)
    redisClient.Publish(ctx, fmt.Sprintf("%s:events", keyPrefix), data)
})

// 3. In the SSE handler, listen for expiry events and send to client
go func() {
    pubsub := redisClient.Subscribe(ctx, fmt.Sprintf("%s:events", keyPrefix))
    for msg := range pubsub.Channel() {
        // Send SSE event to client
        fmt.Fprintf(w, "event: checkpoint_update\ndata: %s\n\n", msg.Payload)
        flusher.Flush()
    }
}()
```

**What the user sees when their streaming request expires:**

```
┌──────────────────────────────────────────────────────────────┐
│  ⚠️ Request Timed Out                                        │
│                                                              │
│  Your approval request expired after 5 minutes.              │
│  No action was taken.                                        │
│                                                              │
│  Original request: "Transfer $5000 to savings"               │
│                                                              │
│  [Resume Request]              [Dismiss]                     │
│                                                              │
│  Clicking "Resume" will continue the operation.              │
└──────────────────────────────────────────────────────────────┘
```

### Non-Streaming Scenario (HTTP 202 + Polling) - Policy-Driven Expiry

In non-streaming scenarios, the user submitted an async request and may not be watching.
The system applies the configured `DefaultAction` from the policy.

```go
// AGENT CODE: In agent's HTTP 202 handler

func handleAsyncRequest(w http.ResponseWriter, r *http.Request) {
    // IMPORTANT: Mark this as a non-streaming request
    // This determines expiry behavior: apply DefaultAction from policy
    ctx := orchestration.WithRequestMode(r.Context(), orchestration.RequestModeNonStreaming)

    // Process request - checkpoints will have request_mode: "non_streaming"
    result, err := orchestrator.ProcessRequest(ctx, request, metadata)

    if orchestration.IsInterrupted(err) {
        // Return 202 with checkpoint ID for polling
        w.WriteHeader(http.StatusAccepted)
        json.NewEncoder(w).Encode(map[string]string{
            "status":        "pending_approval",
            "checkpoint_id": orchestration.GetCheckpointID(err),
            "poll_url":      fmt.Sprintf("/hitl/checkpoints/%s", orchestration.GetCheckpointID(err)),
        })
        return
    }
    // ...
}
```

**What the polling client sees after expiry:**

```json
// GET /hitl/checkpoints/cp-xxx returns:

// If DefaultAction was "approve":
{
    "checkpoint_id": "cp-194ade46-14fe-4b",
    "status": "expired_approved",
    "request_mode": "non_streaming",
    "message": "Auto-approved after timeout per policy configuration",
    "expires_at": "2026-01-19T22:31:44Z",
    "decision": {
        "default_action": "approve"
    }
}

// If DefaultAction was "reject":
{
    "checkpoint_id": "cp-194ade46-14fe-4b",
    "status": "expired_rejected",
    "request_mode": "non_streaming",
    "message": "Auto-rejected after timeout - sensitive operation requires explicit approval",
    "expires_at": "2026-01-19T22:31:44Z",
    "decision": {
        "default_action": "reject"
    }
}
```

### Application-Level Auto-Resume (Non-Streaming Only)

For non-streaming requests where the policy auto-approves on expiry,
the application can optionally trigger automatic resume:

```go
// APPLICATION CODE: Auto-resume for non-streaming approved requests
checkpointStore.SetExpiryCallback(func(ctx context.Context, cp *ExecutionCheckpoint, action CommandType) {
    // ┌────────────────────────────────────────────────────────────────────┐
    // │  APPLICATION DECIDES: Should we auto-resume?                       │
    // │  Only auto-resume if:                                              │
    // │  1. It's a non-streaming request (user wasn't waiting)             │
    // │  2. The action was approve (not reject/abort)                      │
    // │  3. The status is resumable (use framework helper)                 │
    // └────────────────────────────────────────────────────────────────────┘
    if cp.RequestMode != RequestModeNonStreaming ||
       action != CommandApprove ||
       !orchestration.IsResumableStatus(cp.Status) {
        // For streaming requests or non-approved, just notify
        notifyUser(cp, "Request expired")
        return
    }

    // ┌────────────────────────────────────────────────────────────────────┐
    // │  FRAMEWORK PROVIDES: BuildResumeContext                            │
    // │  Prepares context with all checkpoint state for resume             │
    // └────────────────────────────────────────────────────────────────────┘
    resumeCtx, err := orchestration.BuildResumeContext(ctx, cp)
    if err != nil {
        logger.Error("Failed to build resume context", map[string]interface{}{
            "checkpoint_id": cp.CheckpointID,
            "error":         err.Error(),
        })
        return
    }

    // ┌────────────────────────────────────────────────────────────────────┐
    // │  APPLICATION EXECUTES: Resume using its own methods                │
    // │  Framework doesn't know how your app processes requests            │
    // └────────────────────────────────────────────────────────────────────┘
    go func() {
        // Application calls its own sync processing method
        _, err := agent.orchestrator.ProcessRequest(resumeCtx, cp.OriginalRequest, nil)
        if err != nil {
            logger.Error("Auto-resume failed", map[string]interface{}{
                "checkpoint_id": cp.CheckpointID,
                "error":         err.Error(),
            })
        }
    }()
})
```

---

## Configuration

### Architectural Decision: Configuration Hierarchy

The expiry behavior configuration uses a **three-tier hierarchy** that balances simplicity
for common cases with flexibility for advanced scenarios:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     Configuration Priority (Highest to Lowest)              │
└─────────────────────────────────────────────────────────────────────────────┘

     Priority 1: InterruptDecision (per-checkpoint)
     ┌─────────────────────────────────────────────────────────────────────┐
     │  Set by HITLPolicy.Evaluate() for each specific checkpoint.        │
     │  Use when different checkpoints need different expiry behavior.    │
     │                                                                     │
     │  Example: High-value transactions require explicit approval,        │
     │           while low-value ones can auto-approve.                    │
     └─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼ (if not set)
     Priority 2: Environment Variables (global override)
     ┌─────────────────────────────────────────────────────────────────────┐
     │  Set once for the entire application via environment.               │
     │  Use for deployment-specific configuration.                         │
     │                                                                     │
     │  Example: Production uses implicit_deny for safety,                 │
     │           staging uses apply_default for faster testing.            │
     └─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼ (if not set)
     Priority 3: Built-in Defaults (hardcoded)
     ┌─────────────────────────────────────────────────────────────────────┐
     │  Framework defaults that work for most applications.                │
     │                                                                     │
     │  • Streaming: implicit_deny (user saw dialog, didn't act)           │
     │  • Non-streaming: apply_default (user expects autonomous processing)│
     │  • Default request mode: non_streaming                              │
     └─────────────────────────────────────────────────────────────────────┘
```

**Why this design instead of a global HITLPolicyConfig?**

| Aspect | Global Config | Hierarchy (Implemented) |
|--------|---------------|------------------------|
| Per-checkpoint control | ❌ Not possible | ✅ Via InterruptDecision |
| Deployment flexibility | ❌ Requires code change | ✅ Via environment variables |
| Simple defaults | ✅ Works | ✅ Works (Priority 3) |
| Common case complexity | Same | Same |

**For 90% of use cases**: Developers just use the defaults. The framework automatically:
- Treats streaming requests with `implicit_deny` behavior
- Treats non-streaming requests with `apply_default` behavior

**For advanced use cases**: Developers can override at any level of the hierarchy.

### Per-Checkpoint Configuration via InterruptDecision

The `InterruptDecision` struct includes expiry behavior fields that are set when the
policy evaluates a checkpoint. These are copied to the checkpoint and used by the
expiry processor:

```go
type InterruptDecision struct {
    ShouldInterrupt bool
    Reason          InterruptReason
    Message         string
    Priority        InterruptPriority
    Timeout         time.Duration
    DefaultAction   CommandType     // Action to take on expiry (approve, reject, abort)

    // Expiry behavior settings (per-checkpoint granularity)
    StreamingExpiryBehavior    StreamingExpiryBehavior    // "implicit_deny" or "apply_default"
    NonStreamingExpiryBehavior NonStreamingExpiryBehavior // "apply_default" or "implicit_deny"
    DefaultRequestMode         RequestMode                 // "streaming" or "non_streaming"
}
```

**Example: Per-checkpoint behavior in HITLPolicy**

```go
func (p *MyPolicy) Evaluate(ctx context.Context, point InterruptPoint, step *RoutingStep, params map[string]interface{}) *InterruptDecision {
    decision := &InterruptDecision{
        ShouldInterrupt: true,
        Reason:          ReasonSensitiveOperation,
        DefaultAction:   CommandApprove,
        Timeout:         5 * time.Minute,
    }

    // High-value transactions: always require explicit approval
    if amount, ok := params["amount"].(float64); ok && amount > 10000 {
        decision.StreamingExpiryBehavior = StreamingExpiryImplicitDeny
        decision.NonStreamingExpiryBehavior = NonStreamingExpiryImplicitDeny
        decision.Message = "High-value transaction requires explicit approval"
    } else {
        // Low-value: use framework defaults (streaming=implicit_deny, non_streaming=apply_default)
        // No need to set anything - defaults will be used
    }

    return decision
}
```

### Environment Variables

Environment variables provide global defaults that apply when `InterruptDecision` doesn't
specify a value. These are read at runtime by the expiry processor.

#### Expiry Behavior Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `GOMIND_HITL_STREAMING_EXPIRY` | `implicit_deny` | What happens when a **streaming** request's checkpoint expires |
| `GOMIND_HITL_NON_STREAMING_EXPIRY` | `apply_default` | What happens when a **non-streaming** request's checkpoint expires |
| `GOMIND_HITL_DEFAULT_REQUEST_MODE` | `non_streaming` | Request mode to assume when not set in context |

**Detailed Variable Documentation:**

---

**`GOMIND_HITL_STREAMING_EXPIRY`**

Controls what happens when a checkpoint from a **streaming** request (SSE/WebSocket) expires
without a human response.

| Value | Behavior |
|-------|----------|
| `implicit_deny` | **(Default)** Checkpoint marked as `expired`. No action applied. User must manually resume if desired. Rationale: User was watching the approval dialog but chose not to respond. |
| `apply_default` | Apply the `DefaultAction` from the policy (e.g., `approve`, `reject`, `abort`). Checkpoint marked as `expired_approved`, `expired_rejected`, or `expired_aborted`. Workflow auto-resumes. |

**Example:**
```bash
# Production: Be conservative, require explicit approval
export GOMIND_HITL_STREAMING_EXPIRY=implicit_deny

# Internal tooling: Auto-approve for faster iteration
export GOMIND_HITL_STREAMING_EXPIRY=apply_default
```

---

**`GOMIND_HITL_NON_STREAMING_EXPIRY`**

Controls what happens when a checkpoint from a **non-streaming** request (HTTP 202 + polling)
expires without a human response.

| Value | Behavior |
|-------|----------|
| `apply_default` | **(Default)** Apply the `DefaultAction` from the policy. Checkpoint marked as `expired_approved`, `expired_rejected`, or `expired_aborted`. Workflow auto-resumes. Rationale: User submitted async and may expect autonomous processing. |
| `implicit_deny` | Checkpoint marked as `expired`. No action applied. User must manually resume. Use for safety-critical applications where ALL actions require explicit approval. |

**Example:**
```bash
# Automation system: Auto-approve expired checkpoints
export GOMIND_HITL_NON_STREAMING_EXPIRY=apply_default

# Banking system: Require manual review for everything
export GOMIND_HITL_NON_STREAMING_EXPIRY=implicit_deny
```

---

**`GOMIND_HITL_DEFAULT_REQUEST_MODE`**

The request mode to assume when `WithRequestMode()` was not called in the HTTP handler.
When this default is used, a **WARN log is emitted** and a **trace event is added** for
observability.

| Value | Behavior |
|-------|----------|
| `non_streaming` | **(Default)** Treat as async submission. Expiry uses `NonStreamingExpiryBehavior`. |
| `streaming` | Treat as live connection. Expiry uses `StreamingExpiryBehavior`. |

**Example:**
```bash
# Application is primarily async (HTTP 202 + polling)
export GOMIND_HITL_DEFAULT_REQUEST_MODE=non_streaming

# Application is primarily real-time (SSE/WebSocket)
export GOMIND_HITL_DEFAULT_REQUEST_MODE=streaming
```

> **Best Practice**: Always call `WithRequestMode()` in your HTTP handlers rather than
> relying on this default. The WARN log helps identify handlers that forgot to set it.

---

#### Expiry Processor Variables

| Variable | Default | Purpose | Valid Values |
|----------|---------|---------|--------------|
| `GOMIND_HITL_EXPIRY_ENABLED` | `true` | Enable/disable the expiry processor | `true`, `false` |
| `GOMIND_HITL_EXPIRY_INTERVAL` | `10s` | How often to scan for expired checkpoints | Duration (e.g., `5s`, `30s`, `1m`) |
| `GOMIND_HITL_EXPIRY_BATCH_SIZE` | `100` | Maximum checkpoints to process per scan | Integer 1-10000 |

> **Note**: Circuit breaker is not configured via environment variables.
> Per ARCHITECTURE.md Section 9, circuit breaker is an optional injected dependency.
> Use `WithExpiryCircuitBreaker(cb core.CircuitBreaker)` to inject.

### Programmatic Configuration

The agent starts the expiry processor during its setup phase:

```go
// AGENT CODE: In agent's main.go or setup.go
config := orchestration.ExpiryProcessorConfig{
    Enabled:      true,
    ScanInterval: 10 * time.Second,
    BatchSize:    100,
}

// Agent calls this to start the processor goroutine
checkpointStore.StartExpiryProcessor(ctx, config)

// Agent provides callback to handle expiry events
checkpointStore.SetExpiryCallback(func(ctx context.Context, cp *orchestration.ExecutionCheckpoint, action orchestration.CommandType) {
    // Agent-specific logic here
})
```

---

## Developer Workflow

This section explains what developers need to do to implement HITL expiry in their agents,
organized by scenario.

### Minimum Required Steps (Both Scenarios)

Every agent using HITL expiry needs these two things:

#### Step 1: Set RequestMode in HTTP Handlers

```go
// In your HTTP handlers - tell the framework how the user is connected

// For SSE/WebSocket handlers (user is watching in real-time)
func (a *Agent) handleSSE(w http.ResponseWriter, r *http.Request) {
    ctx := orchestration.WithRequestMode(r.Context(), orchestration.RequestModeStreaming)
    a.orchestrator.Process(ctx, request, callback)
}

// For async submission handlers (user will poll later)
func (a *Agent) handleAsyncSubmit(w http.ResponseWriter, r *http.Request) {
    ctx := orchestration.WithRequestMode(r.Context(), orchestration.RequestModeNonStreaming)
    go a.orchestrator.Process(ctx, request, callback)
    w.WriteHeader(http.StatusAccepted)
}
```

> **Why is this required?** The framework uses `RequestMode` to determine expiry behavior.
> If not set, a WARN log is emitted and `GOMIND_HITL_DEFAULT_REQUEST_MODE` is used.

#### Step 2: Set Up Expiry Callback and Start Processor

```go
// In your agent's setup/initialization code (one-time setup)

func (a *Agent) setupHITL() {
    // 1. Define what happens when checkpoints expire
    a.checkpointStore.SetExpiryCallback(func(ctx context.Context, cp *orchestration.ExecutionCheckpoint, action orchestration.CommandType) {
        // action is "" for implicit_deny, or "approve"/"reject"/"abort" for apply_default

        if orchestration.IsResumableStatus(cp.Status) && action != "" {
            // Auto-resume the workflow
            resumeCtx, _ := orchestration.BuildResumeContext(ctx, cp)
            a.orchestrator.Process(resumeCtx, cp.OriginalRequest, a.callback)
        } else {
            // Log that checkpoint expired without action
            a.logger.Info("Checkpoint expired", "checkpoint_id", cp.CheckpointID, "status", cp.Status)
        }
    })

    // 2. Start the expiry processor
    a.checkpointStore.StartExpiryProcessor(ctx, orchestration.ExpiryProcessorConfig{
        Enabled:      true,
        ScanInterval: 10 * time.Second,
        BatchSize:    100,
    })
}
```

### Scenario 1: Streaming (SSE/WebSocket)

**User Experience**: User submits request, sees approval dialog in real-time, can approve/reject.

**Default Behavior**: When checkpoint expires → `implicit_deny` → status becomes `expired` → callback receives `action=""`

```
User Request (SSE)
       │
       ▼
┌──────────────────┐
│ ctx = WithRequest│  ◄── Developer adds this line
│ Mode(streaming)  │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Orchestrator     │
│ creates plan     │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ HITL Controller  │  ◄── Framework reads RequestMode from context
│ checkpoint.      │      automatically
│ RequestMode =    │
│ "streaming"      │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ User sees dialog │
│ in browser       │
│ (can approve/    │
│ reject/edit)     │
└────────┬─────────┘
         │
    [Timeout - no response]
         │
         ▼
┌──────────────────┐
│ Expiry Processor │  ◄── Framework checks: streaming + implicit_deny (default)
│ Status: "expired"│      No action applied
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Callback invoked │  ◄── Your callback: action="" (empty string)
│ action = ""      │      Log it, notify user, but don't auto-resume
└──────────────────┘
```

**Callback Example (Streaming)**:

```go
a.checkpointStore.SetExpiryCallback(func(ctx context.Context, cp *orchestration.ExecutionCheckpoint, action orchestration.CommandType) {
    if action == "" {
        // Implicit deny - user saw the dialog but didn't respond
        a.logger.Info("Checkpoint expired (implicit deny)",
            "checkpoint_id", cp.CheckpointID,
            "request_mode", cp.RequestMode,
        )
        // Optionally notify user via SSE that their request expired
        a.sseNotifier.Send(cp.UserContext["session_id"].(string), Event{
            Type:    "checkpoint_expired",
            Message: "Your approval request has expired",
        })
        return
    }
    // ... handle apply_default case if configured ...
})
```

### Scenario 2: Non-Streaming (HTTP 202 + Polling)

**User Experience**: User submits request, gets 202 Accepted, polls for status later.

**Default Behavior**: When checkpoint expires → `apply_default` → DefaultAction applied → status becomes `expired_approved` (or rejected/aborted) → callback receives `action="approve"` (or reject/abort)

```
User Request (HTTP POST)
       │
       ▼
┌──────────────────┐
│ ctx = WithRequest│  ◄── Developer adds this line
│ Mode(non_stream) │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Return 202       │
│ Process in bg    │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ HITL Controller  │  ◄── checkpoint.RequestMode = "non_streaming"
│ creates checkpoint│
└────────┬─────────┘
         │
    [Timeout - user didn't poll/approve]
         │
         ▼
┌──────────────────┐
│ Expiry Processor │  ◄── Framework checks: non_streaming + apply_default
│ Applies default  │      DefaultAction from policy (e.g., "approve")
│ Status:          │      Status becomes "expired_approved"
│ "expired_approved"│
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Callback invoked │  ◄── Your callback: action="approve"
│ action = "approve"│     Build resume context and continue workflow
└──────────────────┘
```

**Callback Example (Non-Streaming)**:

```go
a.checkpointStore.SetExpiryCallback(func(ctx context.Context, cp *orchestration.ExecutionCheckpoint, action orchestration.CommandType) {
    if action == "" {
        // This only happens if NonStreamingExpiryBehavior = implicit_deny (non-default)
        a.logger.Info("Checkpoint expired (implicit deny)", "checkpoint_id", cp.CheckpointID)
        return
    }

    // action is "approve", "reject", or "abort"
    if orchestration.IsResumableStatus(cp.Status) {
        a.logger.Info("Auto-resuming expired checkpoint",
            "checkpoint_id", cp.CheckpointID,
            "action", action,
        )
        // Build resume context and continue workflow
        resumeCtx, err := orchestration.BuildResumeContext(ctx, cp)
        if err != nil {
            a.logger.Error("Failed to build resume context", "error", err)
            return
        }
        // Continue with your normal processing
        a.orchestrator.Process(resumeCtx, cp.OriginalRequest, a.callback)
    }
})
```

### Scenario 2b: Non-Streaming + apply_default (Auto-Reject)

**User Experience**: User submits async request, but organization has configured `DefaultAction=reject` for safety-critical operations. Unapproved requests are automatically rejected.

**Configuration Required**: Set `default_action: reject` in HITL policy (env: `GOMIND_HITL_DEFAULT_ACTION=reject`)

**When to Use**:
- Safety-critical operations (financial transactions, data deletion)
- Compliance requirements where explicit approval is mandatory
- Operations that cannot be rolled back

**Sequence Diagram**:

```
┌─────┐                    ┌───────┐                         ┌───────────┐
│ UI  │                    │ Agent │                         │ Framework │
└──┬──┘                    └───┬───┘                         └─────┬─────┘
   │                           │                                   │
   │  POST /chat (async)       │                                   │
   │──────────────────────────>│                                   │
   │                           │                                   │
   │                           │  WithRequestMode(non_streaming)   │
   │                           │──────────────────────────────────>│
   │                           │                                   │
   │  202 Accepted             │                                   │
   │  {checkpoint_id: "abc"}   │                                   │
   │<──────────────────────────│                                   │
   │                           │                                   │
   │  ┌────────────────────┐   │  orchestrator.Process() (bg)      │
   │  │ User closes browser│   │──────────────────────────────────>│
   │  │ or walks away      │   │                                   │
   │  └────────────────────┘   │                    ┌──────────────┴──────────────┐
   │                           │                    │ Creates checkpoint          │
   │                           │                    │ RequestMode = "non_streaming"│
   │                           │                    │ ExpiresAt = now + timeout   │
   │                           │                    └──────────────┬──────────────┘
   │                           │                                   │
   │         ... timeout elapses (user never approved) ...         │
   │                           │                                   │
   │                           │                    ┌──────────────┴──────────────┐
   │                           │                    │ Expiry Processor scans      │
   │                           │                    │ Finds expired checkpoint    │
   │                           │                    │ Mode=non_streaming → apply_default│
   │                           │                    │ Applies DefaultAction=REJECT│
   │                           │                    │ Status → "expired_rejected" │
   │                           │                    └──────────────┬──────────────┘
   │                           │                                   │
   │                           │  expiryCallback(cp, action="reject")│
   │                           │<──────────────────────────────────│
   │                           │                                   │
   │                           │  ┌────────────────────────────┐   │
   │                           │  │ Agent handles rejection:   │   │
   │                           │  │ - Does NOT resume workflow │   │
   │                           │  │ - Logs rejection           │   │
   │                           │  │ - Cleans up resources      │   │
   │                           │  │ - Stores "rejected" result │   │
   │                           │  └────────────────────────────┘   │
   │                           │                                   │
   │  (User returns later)     │                                   │
   │  GET /status/{id}         │                                   │
   │──────────────────────────>│                                   │
   │                           │                                   │
   │  200 OK                   │                                   │
   │  {status: "rejected",     │                                   │
   │   reason: "timeout"}      │                                   │
   │<──────────────────────────│                                   │
   │                           │                                   │
   │  ┌────────────────────┐   │                                   │
   │  │ User sees:         │   │                                   │
   │  │ "Request rejected  │   │                                   │
   │  │  due to timeout"   │   │                                   │
   │  └────────────────────┘   │                                   │
   │                           │                                   │
```

**Key Difference from Scenario 2**: In Scenario 2, `DefaultAction=approve` causes the workflow to auto-resume. Here, `DefaultAction=reject` causes the workflow to be **terminated** without execution.

**Callback Example (Non-Streaming + Auto-Reject)**:

```go
a.checkpointStore.SetExpiryCallback(func(ctx context.Context, cp *orchestration.ExecutionCheckpoint, action orchestration.CommandType) {
    switch action {
    case orchestration.CommandApprove:
        // Scenario 2: Auto-approve - resume workflow
        if orchestration.IsResumableStatus(cp.Status) {
            resumeCtx, _ := orchestration.BuildResumeContext(ctx, cp)
            go a.orchestrator.Process(resumeCtx, cp.OriginalRequest, a.callback)
        }

    case orchestration.CommandReject:
        // Scenario 2b: Auto-reject - DO NOT resume, just clean up
        a.logger.Info("Checkpoint auto-rejected on timeout",
            "checkpoint_id", cp.CheckpointID,
            "request_id", cp.RequestID,
        )
        // Store rejection result for user to see when they poll
        a.resultStore.Store(cp.RequestID, Result{
            Status:  "rejected",
            Reason:  "Request timed out without approval",
            Message: "Your request was automatically rejected because it was not approved within the timeout period.",
        })
        // Clean up any resources allocated for this request
        a.cleanupRequest(cp.RequestID)

    case "":
        // implicit_deny - only if NonStreamingExpiryBehavior=implicit_deny (non-default)
        a.logger.Info("Checkpoint expired (implicit deny)", "checkpoint_id", cp.CheckpointID)
    }
})
```

**Environment Variables for Auto-Reject**:

```bash
# Set default action to reject (instead of approve)
export GOMIND_HITL_DEFAULT_ACTION=reject

# Non-streaming already uses apply_default by default, so this is optional:
# export GOMIND_HITL_NON_STREAMING_EXPIRY=apply_default
```

---

### Scenario 3: Streaming + apply_default (Auto-Approve via Environment Variable)

**User Experience**: User submits SSE request, sees approval dialog, but organization has configured auto-approve on timeout for operational continuity.

**Configuration Required**: Set `GOMIND_HITL_STREAMING_EXPIRY=apply_default` (overrides default `implicit_deny`)

**When to Use**:
- Low-risk operations where blocking is worse than proceeding
- Operations with downstream rollback capability
- Development/staging environments for faster iteration

```
User Request (SSE)
       │
       ▼
┌──────────────────┐
│ ctx = WithRequest│  ◄── Developer adds this line
│ Mode(streaming)  │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Orchestrator     │
│ creates plan     │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ HITL Controller  │  ◄── checkpoint.RequestMode = "streaming"
│ creates checkpoint│
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ User sees dialog │
│ in browser       │
│ (can approve/    │
│ reject/edit)     │
└────────┬─────────┘
         │
    [Timeout - no response]
         │
         ▼
┌──────────────────┐
│ Expiry Processor │  ◄── Framework checks: streaming + apply_default
│ Applies default  │      (env: GOMIND_HITL_STREAMING_EXPIRY=apply_default)
│ Status:          │      DefaultAction from policy (e.g., "approve")
│ "expired_approved"│      Status becomes "expired_approved"
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Callback invoked │  ◄── Your callback: action="approve"
│ action = "approve"│     Agent auto-resumes execution
└──────────────────┘
```

**Key Difference from Scenario 1**: In Scenario 1 (default), streaming requests use `implicit_deny`,
meaning no action is applied and the checkpoint just becomes `expired`. Here, with explicit
configuration, the `DefaultAction` from the policy is applied automatically.

**Callback Example (Streaming + apply_default)**:

```go
a.checkpointStore.SetExpiryCallback(func(ctx context.Context, cp *orchestration.ExecutionCheckpoint, action orchestration.CommandType) {
    // With GOMIND_HITL_STREAMING_EXPIRY=apply_default, streaming requests
    // will also receive an action (approve/reject/abort) instead of ""

    if action == "" {
        // This only happens if env is implicit_deny (default for streaming)
        a.logger.Info("Checkpoint expired (implicit deny)", "checkpoint_id", cp.CheckpointID)
        return
    }

    // action is "approve", "reject", or "abort" - auto-resume
    if action == orchestration.CommandApprove && orchestration.IsResumableStatus(cp.Status) {
        a.logger.Info("Auto-resuming streaming checkpoint on timeout",
            "checkpoint_id", cp.CheckpointID,
            "request_mode", cp.RequestMode, // "streaming"
            "action", action,
        )

        // Notify user via SSE that their request was auto-approved
        if sessionID, ok := cp.UserContext["session_id"].(string); ok {
            a.sseNotifier.Send(sessionID, Event{
                Type:    "checkpoint_auto_approved",
                Message: "Your request was automatically approved after timeout",
            })
        }

        // Build resume context and continue workflow
        resumeCtx, err := orchestration.BuildResumeContext(ctx, cp)
        if err != nil {
            a.logger.Error("Failed to build resume context", "error", err)
            return
        }
        go a.orchestrator.Process(resumeCtx, cp.OriginalRequest, a.streamCallback)
    }
})
```

**Environment Variable for this Scenario**:

```bash
# In deployment configuration (K8s, Docker, etc.)
export GOMIND_HITL_STREAMING_EXPIRY=apply_default

# With default_action=approve in policy, this means:
# - Streaming request checkpoint expires
# - Status becomes "expired_approved"
# - Callback receives action="approve"
# - Agent auto-resumes execution
```

---

### Complete Example: Agent with All Scenarios

```go
package main

import (
    "context"
    "net/http"
    "time"

    "github.com/anthropics/gomind/orchestration"
)

type Agent struct {
    orchestrator    *orchestration.AIOrchestrator
    checkpointStore orchestration.CheckpointStore
    sseNotifier     *SSENotifier
    logger          Logger
}

func (a *Agent) Setup(ctx context.Context) {
    // Set up expiry callback
    a.checkpointStore.SetExpiryCallback(func(ctx context.Context, cp *orchestration.ExecutionCheckpoint, action orchestration.CommandType) {
        a.handleExpiredCheckpoint(ctx, cp, action)
    })

    // Start expiry processor
    a.checkpointStore.StartExpiryProcessor(ctx, orchestration.ExpiryProcessorConfig{
        Enabled:      true,
        ScanInterval: 10 * time.Second,
        BatchSize:    100,
    })
}

func (a *Agent) handleExpiredCheckpoint(ctx context.Context, cp *orchestration.ExecutionCheckpoint, action orchestration.CommandType) {
    switch action {
    case "": // Implicit deny - no action applied
        // This happens when:
        // - Scenario 1: Streaming + implicit_deny (default for streaming)
        // - Non-streaming + implicit_deny (non-default, requires env var)
        a.logger.Info("Checkpoint expired without action",
            "checkpoint_id", cp.CheckpointID,
            "request_mode", cp.RequestMode,
        )
        if cp.RequestMode == orchestration.RequestModeStreaming {
            // Scenario 1: User saw the dialog but didn't respond
            // Notify them their request has expired
            a.sseNotifier.Send(cp.UserContext["session_id"].(string), Event{
                Type:    "checkpoint_expired",
                Message: "Your approval request has timed out. Please resubmit.",
            })
        }

    case orchestration.CommandApprove, orchestration.CommandReject, orchestration.CommandAbort:
        // This happens when:
        // - Scenario 2: Non-streaming + apply_default (default for non-streaming)
        // - Scenario 3: Streaming + apply_default (requires GOMIND_HITL_STREAMING_EXPIRY=apply_default)
        // The DefaultAction from policy was applied automatically
        a.logger.Info("Checkpoint expired with auto-action",
            "checkpoint_id", cp.CheckpointID,
            "request_mode", cp.RequestMode,
            "action", action,
            "status", cp.Status, // e.g., "expired_approved"
        )
        if orchestration.IsResumableStatus(cp.Status) {
            // Notify streaming users that auto-action was applied
            if cp.RequestMode == orchestration.RequestModeStreaming {
                a.sseNotifier.Send(cp.UserContext["session_id"].(string), Event{
                    Type:    "checkpoint_auto_" + string(action),
                    Message: "Your request was automatically " + string(action) + "d after timeout",
                })
            }
            // Auto-resume the workflow
            resumeCtx, _ := orchestration.BuildResumeContext(ctx, cp)
            go a.orchestrator.Process(resumeCtx, cp.OriginalRequest, a.streamCallback)
        }
    }
}

// HTTP Handlers - the only places developers MUST add WithRequestMode

func (a *Agent) HandleSSE(w http.ResponseWriter, r *http.Request) {
    ctx := orchestration.WithRequestMode(r.Context(), orchestration.RequestModeStreaming)
    // ... SSE setup ...
    a.orchestrator.Process(ctx, request, callback)
}

func (a *Agent) HandleAsyncSubmit(w http.ResponseWriter, r *http.Request) {
    ctx := orchestration.WithRequestMode(r.Context(), orchestration.RequestModeNonStreaming)
    go a.orchestrator.Process(ctx, request, callback)
    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(map[string]string{"request_id": requestID})
}
```

### Sequence Diagrams

The following sequence diagrams show the flow of events between UI, Agent, and Framework for each scenario.

#### Scenario 1: Streaming + implicit_deny (Default)

```
┌─────┐                    ┌───────┐                         ┌───────────┐
│ UI  │                    │ Agent │                         │ Framework │
└──┬──┘                    └───┬───┘                         └─────┬─────┘
   │                           │                                   │
   │  POST /chat/stream (SSE)  │                                   │
   │──────────────────────────>│                                   │
   │                           │                                   │
   │                           │  WithRequestMode(streaming)       │
   │                           │──────────────────────────────────>│
   │                           │                                   │
   │                           │  orchestrator.Process()           │
   │                           │──────────────────────────────────>│
   │                           │                                   │
   │                           │                    ┌──────────────┴──────────────┐
   │                           │                    │ Creates checkpoint          │
   │                           │                    │ RequestMode = "streaming"   │
   │                           │                    │ ExpiresAt = now + timeout   │
   │                           │                    └──────────────┬──────────────┘
   │                           │                                   │
   │                           │  return ErrInterrupted            │
   │                           │<──────────────────────────────────│
   │                           │                                   │
   │  SSE: event=checkpoint    │                                   │
   │<──────────────────────────│                                   │
   │                           │                                   │
   │  ┌────────────────────┐   │                                   │
   │  │ User sees dialog   │   │                                   │
   │  │ but doesn't respond│   │                                   │
   │  └────────────────────┘   │                                   │
   │                           │                                   │
   │         ... timeout elapses ...                               │
   │                           │                                   │
   │                           │                    ┌──────────────┴──────────────┐
   │                           │                    │ Expiry Processor scans      │
   │                           │                    │ Finds expired checkpoint    │
   │                           │                    │ Mode=streaming → implicit_deny│
   │                           │                    │ Status → "expired"          │
   │                           │                    │ (no action applied)         │
   │                           │                    └──────────────┬──────────────┘
   │                           │                                   │
   │                           │  expiryCallback(cp, action="")    │
   │                           │<──────────────────────────────────│
   │                           │                                   │
   │  SSE: event=expired       │                                   │
   │  "Request timed out"      │                                   │
   │<──────────────────────────│                                   │
   │                           │                                   │
   │  ┌────────────────────┐   │                                   │
   │  │ User sees timeout  │   │                                   │
   │  │ Must resubmit      │   │                                   │
   │  └────────────────────┘   │                                   │
   │                           │                                   │
```

#### Scenario 2: Non-Streaming + apply_default (Default)

```
┌─────┐                    ┌───────┐                         ┌───────────┐
│ UI  │                    │ Agent │                         │ Framework │
└──┬──┘                    └───┬───┘                         └─────┬─────┘
   │                           │                                   │
   │  POST /chat (async)       │                                   │
   │──────────────────────────>│                                   │
   │                           │                                   │
   │                           │  WithRequestMode(non_streaming)   │
   │                           │──────────────────────────────────>│
   │                           │                                   │
   │  202 Accepted             │                                   │
   │  {checkpoint_id: "abc"}   │                                   │
   │<──────────────────────────│                                   │
   │                           │                                   │
   │  ┌────────────────────┐   │  orchestrator.Process() (bg)      │
   │  │ User closes browser│   │──────────────────────────────────>│
   │  │ or walks away      │   │                                   │
   │  └────────────────────┘   │                    ┌──────────────┴──────────────┐
   │                           │                    │ Creates checkpoint          │
   │                           │                    │ RequestMode = "non_streaming"│
   │                           │                    │ ExpiresAt = now + timeout   │
   │                           │                    └──────────────┬──────────────┘
   │                           │                                   │
   │         ... timeout elapses (user never polls) ...            │
   │                           │                                   │
   │                           │                    ┌──────────────┴──────────────┐
   │                           │                    │ Expiry Processor scans      │
   │                           │                    │ Finds expired checkpoint    │
   │                           │                    │ Mode=non_streaming → apply_default│
   │                           │                    │ Applies DefaultAction=approve│
   │                           │                    │ Status → "expired_approved" │
   │                           │                    └──────────────┬──────────────┘
   │                           │                                   │
   │                           │  expiryCallback(cp, action="approve")│
   │                           │<──────────────────────────────────│
   │                           │                                   │
   │                           │  BuildResumeContext()             │
   │                           │──────────────────────────────────>│
   │                           │                                   │
   │                           │  orchestrator.Process() (resume)  │
   │                           │──────────────────────────────────>│
   │                           │                                   │
   │                           │                    ┌──────────────┴──────────────┐
   │                           │                    │ Workflow continues          │
   │                           │                    │ Tools executed              │
   │                           │                    │ Result stored               │
   │                           │                    └──────────────┬──────────────┘
   │                           │                                   │
   │  (User returns later)     │                                   │
   │  GET /status/{id}         │                                   │
   │──────────────────────────>│                                   │
   │                           │                                   │
   │  200 OK                   │                                   │
   │  {status: "completed"}    │                                   │
   │<──────────────────────────│                                   │
   │                           │                                   │
```

#### Scenario 2b: Non-Streaming + apply_default (Auto-Reject)

```
┌─────┐                    ┌───────┐                         ┌───────────┐
│ UI  │                    │ Agent │                         │ Framework │
└──┬──┘                    └───┬───┘                         └─────┬─────┘
   │                           │                                   │
   │                           │          ┌────────────────────────┴────────────────────────┐
   │                           │          │ Environment: GOMIND_HITL_DEFAULT_ACTION=reject  │
   │                           │          └────────────────────────┬────────────────────────┘
   │                           │                                   │
   │  POST /chat (async)       │                                   │
   │──────────────────────────>│                                   │
   │                           │                                   │
   │                           │  WithRequestMode(non_streaming)   │
   │                           │──────────────────────────────────>│
   │                           │                                   │
   │  202 Accepted             │                                   │
   │  {checkpoint_id: "abc"}   │                                   │
   │<──────────────────────────│                                   │
   │                           │                                   │
   │  ┌────────────────────┐   │  orchestrator.Process() (bg)      │
   │  │ User closes browser│   │──────────────────────────────────>│
   │  │ or walks away      │   │                                   │
   │  └────────────────────┘   │                    ┌──────────────┴──────────────┐
   │                           │                    │ Creates checkpoint          │
   │                           │                    │ RequestMode = "non_streaming"│
   │                           │                    │ ExpiresAt = now + timeout   │
   │                           │                    └──────────────┬──────────────┘
   │                           │                                   │
   │         ... timeout elapses (user never approved) ...         │
   │                           │                                   │
   │                           │                    ┌──────────────┴──────────────┐
   │                           │                    │ Expiry Processor scans      │
   │                           │                    │ Finds expired checkpoint    │
   │                           │                    │ Mode=non_streaming → apply_default│
   │                           │                    │ Applies DefaultAction=REJECT│
   │                           │                    │ Status → "expired_rejected" │
   │                           │                    └──────────────┬──────────────┘
   │                           │                                   │
   │                           │  expiryCallback(cp, action="reject")│
   │                           │<──────────────────────────────────│
   │                           │                                   │
   │                           │  ┌────────────────────────────┐   │
   │                           │  │ Agent: NO resume!          │   │
   │                           │  │ - Log rejection            │   │
   │                           │  │ - Store "rejected" result  │   │
   │                           │  │ - Clean up resources       │   │
   │                           │  └────────────────────────────┘   │
   │                           │                                   │
   │  (User returns later)     │                                   │
   │  GET /status/{id}         │                                   │
   │──────────────────────────>│                                   │
   │                           │                                   │
   │  200 OK                   │                                   │
   │  {status: "rejected",     │                                   │
   │   reason: "timeout"}      │                                   │
   │<──────────────────────────│                                   │
   │                           │                                   │
```

#### Scenario 3: Streaming + apply_default (Override via Env Var)

```
┌─────┐                    ┌───────┐                         ┌───────────┐
│ UI  │                    │ Agent │                         │ Framework │
└──┬──┘                    └───┬───┘                         └─────┬─────┘
   │                           │                                   │
   │                           │          ┌────────────────────────┴────────────────────────┐
   │                           │          │ Environment: GOMIND_HITL_STREAMING_EXPIRY=apply_default │
   │                           │          └────────────────────────┬────────────────────────┘
   │                           │                                   │
   │  POST /chat/stream (SSE)  │                                   │
   │──────────────────────────>│                                   │
   │                           │                                   │
   │                           │  WithRequestMode(streaming)       │
   │                           │──────────────────────────────────>│
   │                           │                                   │
   │                           │  orchestrator.Process()           │
   │                           │──────────────────────────────────>│
   │                           │                                   │
   │                           │                    ┌──────────────┴──────────────┐
   │                           │                    │ Creates checkpoint          │
   │                           │                    │ RequestMode = "streaming"   │
   │                           │                    │ ExpiresAt = now + timeout   │
   │                           │                    └──────────────┬──────────────┘
   │                           │                                   │
   │                           │  return ErrInterrupted            │
   │                           │<──────────────────────────────────│
   │                           │                                   │
   │  SSE: event=checkpoint    │                                   │
   │<──────────────────────────│                                   │
   │                           │                                   │
   │  ┌────────────────────┐   │                                   │
   │  │ User sees dialog   │   │                                   │
   │  │ but doesn't respond│   │                                   │
   │  └────────────────────┘   │                                   │
   │                           │                                   │
   │         ... timeout elapses ...                               │
   │                           │                                   │
   │                           │                    ┌──────────────┴──────────────┐
   │                           │                    │ Expiry Processor scans      │
   │                           │                    │ Finds expired checkpoint    │
   │                           │                    │ Mode=streaming BUT env=apply_default│
   │                           │                    │ Applies DefaultAction=approve│
   │                           │                    │ Status → "expired_approved" │
   │                           │                    └──────────────┬──────────────┘
   │                           │                                   │
   │                           │  expiryCallback(cp, action="approve")│
   │                           │<──────────────────────────────────│
   │                           │                                   │
   │  SSE: event=auto_approved │                                   │
   │  "Request auto-approved"  │                                   │
   │<──────────────────────────│                                   │
   │                           │                                   │
   │                           │  BuildResumeContext()             │
   │                           │──────────────────────────────────>│
   │                           │                                   │
   │                           │  orchestrator.Process() (resume)  │
   │                           │──────────────────────────────────>│
   │                           │                                   │
   │                           │                    ┌──────────────┴──────────────┐
   │                           │                    │ Workflow continues          │
   │                           │                    │ Tools executed              │
   │                           │                    └──────────────┬──────────────┘
   │                           │                                   │
   │  SSE: event=step          │                                   │
   │  SSE: event=chunk         │                                   │
   │  SSE: event=done          │                                   │
   │<──────────────────────────│<──────────────────────────────────│
   │                           │                                   │
   │  ┌────────────────────┐   │                                   │
   │  │ User sees result   │   │                                   │
   │  │ (auto-completed)   │   │                                   │
   │  └────────────────────┘   │                                   │
   │                           │                                   │
```

### Scenario Quick Reference

| Scenario | Request Mode | Expiry Behavior | DefaultAction | Status After Expiry | Callback `action` | Workflow Result |
|----------|--------------|-----------------|---------------|---------------------|-------------------|-----------------|
| **1** (Default Streaming) | `streaming` | `implicit_deny` | N/A | `expired` | `""` (empty) | Stopped, user must resubmit |
| **2a** (Non-Streaming Auto-Approve) | `non_streaming` | `apply_default` | `approve` | `expired_approved` | `approve` | **Continues** automatically |
| **2b** (Non-Streaming Auto-Reject) | `non_streaming` | `apply_default` | `reject` | `expired_rejected` | `reject` | **Terminated**, not executed |
| **3** (Streaming Auto-Approve) | `streaming` | `apply_default` | `approve` | `expired_approved` | `approve` | **Continues** automatically |

**Environment Variables by Scenario**:

| Scenario | `GOMIND_HITL_STREAMING_EXPIRY` | `GOMIND_HITL_NON_STREAMING_EXPIRY` | `GOMIND_HITL_DEFAULT_ACTION` |
|----------|-------------------------------|-----------------------------------|------------------------------|
| **1** | `implicit_deny` (default) | - | - |
| **2a** | - | `apply_default` (default) | `approve` (default) |
| **2b** | - | `apply_default` (default) | `reject` |
| **3** | `apply_default` | - | `approve` (default) |

**Choosing Between Scenarios**:
- **Scenario 1**: User is watching the screen (SSE). If they don't respond, respect their implicit denial.
- **Scenario 2a**: User submitted async, low-risk operation. Auto-approve for operational continuity.
- **Scenario 2b**: User submitted async, high-risk operation. Auto-reject for safety (explicit approval required).
- **Scenario 3**: SSE but auto-approve is acceptable (low-risk ops, dev environment, or rollback capability).

### Summary: Developer Checklist

| Task | Where | Required? |
|------|-------|-----------|
| Call `WithRequestMode()` | HTTP handlers | **Yes** |
| Call `SetExpiryCallback()` | Agent setup | **Yes** |
| Call `StartExpiryProcessor()` | Agent setup | **Yes** |
| Use `BuildResumeContext()` | In callback | When auto-resuming |
| Use `IsResumableStatus()` | In callback | Recommended |
| Set env vars | Deployment | Only if overriding defaults |
| Set `InterruptDecision` fields | HITLPolicy | Only for per-checkpoint control |

---

## Multi-Agent Considerations

Each agent runs its own expiry processor that only processes checkpoints under its key prefix:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     Multi-Agent Expiry Processing                            │
└─────────────────────────────────────────────────────────────────────────────┘

  Agent: payment-service                    Agent: trading-bot
  ┌─────────────────────────┐              ┌─────────────────────────┐
  │  ExpiryProcessor        │              │  ExpiryProcessor        │
  │  keyPrefix: gomind:hitl │              │  keyPrefix: gomind:hitl │
  │            :payment-    │              │            :trading-bot │
  │            service      │              │                         │
  └───────────┬─────────────┘              └───────────┬─────────────┘
              │                                        │
              │ scans only                             │ scans only
              ▼                                        ▼
  gomind:hitl:payment-service:pending      gomind:hitl:trading-bot:pending
```

---

## Metrics

### Metric Definitions

All expiry processor metrics follow the established pattern in `hitl_metrics.go` and include the
`module=orchestration` label per telemetry/ARCHITECTURE.md guidelines.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `orchestration.hitl.checkpoint_expired_total` | Counter | `action`, `request_mode`, `interrupt_point`, `module` | Checkpoints auto-processed on expiry |
| `orchestration.hitl.expiry_scan_duration_seconds` | Histogram | `module` | Time taken for each expiry scan |
| `orchestration.hitl.expiry_batch_size` | Gauge | `module` | Number of checkpoints processed per scan |
| `orchestration.hitl.expiry_scan_skipped_total` | Counter | `reason`, `module` | Scans skipped due to errors |
| `orchestration.hitl.callback_panic_total` | Counter | `module` | Callback panics caught and recovered |

### Helper Functions (Add to hitl_metrics.go)

Per the established pattern in `hitl_metrics.go`, add these helper functions for expiry metrics:

```go
// In orchestration/hitl_metrics.go

// =============================================================================
// Expiry Processor Metrics
// =============================================================================

const (
    // Expiry processor counters
    MetricCheckpointExpired  = "orchestration.hitl.checkpoint_expired_total"
    MetricExpiryScanSkipped  = "orchestration.hitl.expiry_scan_skipped_total"
    MetricCallbackPanic      = "orchestration.hitl.callback_panic_total"

    // Expiry processor histograms/gauges
    MetricExpiryScanDuration = "orchestration.hitl.expiry_scan_duration_seconds"
    MetricExpiryBatchSize    = "orchestration.hitl.expiry_batch_size"
)

// RecordCheckpointExpired records when a checkpoint is auto-processed on expiry.
// Labels: action (approve, reject, abort, or empty string for implicit_deny), request_mode, interrupt_point, module
func RecordCheckpointExpired(action string, requestMode RequestMode, interruptPoint InterruptPoint) {
    telemetry.Counter(MetricCheckpointExpired,
        "action", action,
        "request_mode", string(requestMode),
        "interrupt_point", string(interruptPoint),
        "module", telemetry.ModuleOrchestration,
    )
}

// RecordExpiryScanDuration records time taken for each expiry scan.
// Labels: module
func RecordExpiryScanDuration(durationSeconds float64) {
    telemetry.Histogram(MetricExpiryScanDuration, durationSeconds,
        "module", telemetry.ModuleOrchestration,
    )
}

// RecordExpiryBatchSize records number of checkpoints processed per scan.
// Labels: module
func RecordExpiryBatchSize(count int) {
    telemetry.Gauge(MetricExpiryBatchSize, float64(count),
        "module", telemetry.ModuleOrchestration,
    )
}

// RecordExpiryScanSkipped records when an expiry scan is skipped due to errors.
// Labels: reason, module
func RecordExpiryScanSkipped(reason string) {
    telemetry.Counter(MetricExpiryScanSkipped,
        "reason", reason,
        "module", telemetry.ModuleOrchestration,
    )
}

// RecordCallbackPanic records when an expiry callback panics.
// Labels: module
// NOTE: checkpoint_id is NOT a label (high-cardinality). It's logged separately via logger.
func RecordCallbackPanic() {
    telemetry.Counter(MetricCallbackPanic,
        "module", telemetry.ModuleOrchestration,
    )
}
```

---

## Testing

### Unit Tests

```go
func TestExpiryProcessor_ProcessesExpiredCheckpoints(t *testing.T) {
    store := NewRedisCheckpointStore(...)

    // Create a checkpoint that's already expired
    cp := &ExecutionCheckpoint{
        CheckpointID: "cp-test-001",
        Status:       CheckpointStatusPending,
        ExpiresAt:    time.Now().Add(-1 * time.Minute), // Already expired
        Decision: &InterruptDecision{
            DefaultAction: CommandApprove,
        },
    }
    store.SaveCheckpoint(ctx, cp)

    // Run one scan
    store.processExpiredCheckpoints(100)

    // Verify status updated
    loaded, _ := store.LoadCheckpoint(ctx, "cp-test-001")
    assert.Equal(t, CheckpointStatusExpiredApproved, loaded.Status)
}
```

### Integration Tests

```go
func TestExpiryProcessor_CallsCallback(t *testing.T) {
    store := NewRedisCheckpointStore(...)

    callbackCalled := make(chan *ExecutionCheckpoint, 1)
    store.SetExpiryCallback(func(ctx context.Context, cp *ExecutionCheckpoint, action CommandType) {
        callbackCalled <- cp
    })

    // Create expired checkpoint
    // Start processor
    // Wait for callback

    select {
    case cp := <-callbackCalled:
        assert.Equal(t, CheckpointStatusExpiredApproved, cp.Status)
    case <-time.After(15 * time.Second):
        t.Fatal("Callback not called")
    }
}
```

### InMemoryCheckpointStore (For Testing)

> **⚠️ REMOVED FROM IMPLEMENTATION (2026-01-21)**
>
> The InMemoryCheckpointStore was removed from the implementation. Redis is now required
> for all HITL functionality. Developers should run their own Redis instance for testing.
> The code below is preserved for historical reference only.

~~An in-memory implementation of CheckpointStore is required for unit testing without Redis dependency.
This implementation should be in `hitl_checkpoint_store.go` alongside the Redis implementation.~~

```go
// In hitl_checkpoint_store.go (FRAMEWORK CODE)

// InMemoryCheckpointStore is an in-memory implementation of CheckpointStore.
// Used for testing and development. NOT for production use.
type InMemoryCheckpointStore struct {
    mu            sync.RWMutex
    checkpoints   map[string]*ExecutionCheckpoint
    expiryCallback ExpiryCallback
    expiryStarted  bool
    expiryCancel   context.CancelFunc
    expiryWg       sync.WaitGroup
    config         ExpiryProcessorConfig
    logger         core.Logger
}

// NewInMemoryCheckpointStore creates an in-memory checkpoint store.
func NewInMemoryCheckpointStore(opts ...InMemoryCheckpointStoreOption) *InMemoryCheckpointStore {
    store := &InMemoryCheckpointStore{
        checkpoints: make(map[string]*ExecutionCheckpoint),
    }
    for _, opt := range opts {
        opt(store)
    }
    return store
}

// InMemoryCheckpointStoreOption configures the in-memory store.
type InMemoryCheckpointStoreOption func(*InMemoryCheckpointStore)

// WithInMemoryLogger sets the logger.
func WithInMemoryLogger(logger core.Logger) InMemoryCheckpointStoreOption {
    return func(s *InMemoryCheckpointStore) {
        s.logger = logger
    }
}

// SaveCheckpoint saves a checkpoint to memory.
func (s *InMemoryCheckpointStore) SaveCheckpoint(ctx context.Context, checkpoint *ExecutionCheckpoint) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.checkpoints[checkpoint.CheckpointID] = checkpoint
    return nil
}

// LoadCheckpoint loads a checkpoint from memory.
func (s *InMemoryCheckpointStore) LoadCheckpoint(ctx context.Context, checkpointID string) (*ExecutionCheckpoint, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    cp, ok := s.checkpoints[checkpointID]
    if !ok {
        return nil, fmt.Errorf("checkpoint not found: %s", checkpointID)
    }
    return cp, nil
}

// UpdateCheckpointStatus updates checkpoint status.
func (s *InMemoryCheckpointStore) UpdateCheckpointStatus(ctx context.Context, checkpointID string, status CheckpointStatus) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    cp, ok := s.checkpoints[checkpointID]
    if !ok {
        return fmt.Errorf("checkpoint not found: %s", checkpointID)
    }
    cp.Status = status
    return nil
}

// ListPendingCheckpoints lists checkpoints matching the filter.
func (s *InMemoryCheckpointStore) ListPendingCheckpoints(ctx context.Context, filter CheckpointFilter) ([]*ExecutionCheckpoint, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    var results []*ExecutionCheckpoint
    for _, cp := range s.checkpoints {
        if filter.Status != "" && cp.Status != filter.Status {
            continue
        }
        if filter.ExpiredBefore != nil && !cp.ExpiresAt.Before(*filter.ExpiredBefore) {
            continue
        }
        results = append(results, cp)
    }
    return results, nil
}

// DeleteCheckpoint deletes a checkpoint.
func (s *InMemoryCheckpointStore) DeleteCheckpoint(ctx context.Context, checkpointID string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    delete(s.checkpoints, checkpointID)
    return nil
}

// StartExpiryProcessor starts the background expiry processor.
func (s *InMemoryCheckpointStore) StartExpiryProcessor(ctx context.Context, config ExpiryProcessorConfig) error {
    if err := validateExpiryConfig(config); err != nil {
        return fmt.Errorf("invalid expiry processor configuration: %w", err)
    }
    if !config.Enabled {
        return nil
    }

    s.mu.Lock()
    s.expiryStarted = true
    s.config = config
    ctx, s.expiryCancel = context.WithCancel(ctx)
    s.mu.Unlock()

    s.expiryWg.Add(1)
    go s.expiryProcessorLoop(ctx)
    return nil
}

// StopExpiryProcessor stops the expiry processor gracefully.
func (s *InMemoryCheckpointStore) StopExpiryProcessor(ctx context.Context) error {
    s.mu.Lock()
    cancel := s.expiryCancel
    s.mu.Unlock()

    if cancel == nil {
        return nil
    }

    cancel()

    done := make(chan struct{})
    go func() {
        s.expiryWg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return fmt.Errorf("expiry processor shutdown cancelled: %w", ctx.Err())
    }
}

// SetExpiryCallback sets the callback for expired checkpoints.
func (s *InMemoryCheckpointStore) SetExpiryCallback(callback ExpiryCallback) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.expiryStarted {
        return fmt.Errorf("SetExpiryCallback must be called before StartExpiryProcessor")
    }
    s.expiryCallback = callback
    return nil
}

// expiryProcessorLoop runs the background expiry processor.
func (s *InMemoryCheckpointStore) expiryProcessorLoop(ctx context.Context) {
    defer s.expiryWg.Done()

    interval := s.config.ScanInterval
    if interval == 0 {
        interval = 10 * time.Second
    }

    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.processExpiredCheckpoints(ctx)
        }
    }
}

// processExpiredCheckpoints processes all expired checkpoints.
func (s *InMemoryCheckpointStore) processExpiredCheckpoints(ctx context.Context) {
    // Track scan duration for metrics (per hitl_metrics.go helper pattern)
    scanStartTime := time.Now()

    now := time.Now()
    filter := CheckpointFilter{
        Status:        CheckpointStatusPending,
        ExpiredBefore: &now,
    }

    checkpoints, err := s.ListPendingCheckpoints(ctx, filter)
    if err != nil {
        RecordExpiryScanSkipped("list_pending_failed")
        return
    }

    batchSize := s.config.BatchSize
    if batchSize == 0 {
        batchSize = 100
    }

    processed := 0
    for i, cp := range checkpoints {
        if i >= batchSize {
            break
        }
        s.processExpiredCheckpoint(ctx, cp)
        processed++
    }

    // Record scan metrics using helper functions (per hitl_metrics.go pattern)
    RecordExpiryScanDuration(time.Since(scanStartTime).Seconds())
    RecordExpiryBatchSize(processed)
}

// processExpiredCheckpoint processes a single expired checkpoint.
func (s *InMemoryCheckpointStore) processExpiredCheckpoint(ctx context.Context, cp *ExecutionCheckpoint) {
    // Determine new status and action based on request mode
    var newStatus CheckpointStatus
    var action CommandType
    var actionLabel string

    if cp.RequestMode == RequestModeStreaming {
        // Streaming: implicit deny (no action applied)
        newStatus = CheckpointStatusExpired
        action = "" // No action
        actionLabel = "implicit_deny"
    } else {
        // Non-streaming: apply default action
        if cp.Decision != nil {
            action = cp.Decision.DefaultAction
        }
        if action == CommandApprove {
            newStatus = CheckpointStatusExpiredApproved
            actionLabel = string(action)
        } else {
            newStatus = CheckpointStatusExpiredRejected
            actionLabel = string(action)
        }
    }

    // Record metric using helper function (per hitl_metrics.go pattern)
    RecordCheckpointExpired(actionLabel, cp.RequestMode, cp.InterruptPoint)

    // Update status
    _ = s.UpdateCheckpointStatus(ctx, cp.CheckpointID, newStatus)
    cp.Status = newStatus

    // Invoke callback if set
    s.mu.RLock()
    callback := s.expiryCallback
    s.mu.RUnlock()

    if callback != nil {
        callback(ctx, cp, action)
    }
}

// Ensure InMemoryCheckpointStore implements CheckpointStore
var _ CheckpointStore = (*InMemoryCheckpointStore)(nil)
```

---

## Production Considerations

This section covers production-grade patterns required for running the HITL expiry processor
in a distributed, multi-pod environment. Per `FRAMEWORK_DESIGN_PRINCIPLES.md`, the framework
provides built-in reliability rather than documenting limitations.

### Summary of Production Items

| # | Item | Priority | Rationale |
|---|------|----------|-----------|
| 1 | BuildResumeContext Helper | **HIGH** | Encapsulates resume context setup; keeps framework decoupled from application |
| 2 | IsResumableStatus Helper | **MEDIUM** | Prevents status check bugs; ensures consistent resume logic |
| 3 | Distributed Concurrency | **MEDIUM** | Required for multi-pod deployments; prevents duplicate processing |
| 4 | Callback Error Recovery | **MEDIUM** | Panicking callback would crash entire processor |
| 5 | Graceful Shutdown | **MEDIUM** | Required by FRAMEWORK_DESIGN_PRINCIPLES.md Component Lifecycle Rules |
| 6 | Configuration Integration | **MEDIUM** | Aligns with existing HITLConfig pattern; reduces configuration fragmentation |
| 7 | Input Validation & Errors | **MEDIUM** | Required by FRAMEWORK_DESIGN_PRINCIPLES.md Security + Error Message guidelines |

---

### Item 1: Framework Helpers [HIGH]

The framework provides helper functions that encapsulate common patterns, reducing boilerplate
and ensuring consistent behavior across applications.

#### 1a. BuildResumeContext Helper [HIGH]

##### Why BuildResumeContext Instead of ResumeCheckpoint?

The framework provides `BuildResumeContext` instead of a `ResumeCheckpoint` helper that would
execute the resume directly. This design follows the framework's **Interface-First Design**
principle and maintains proper separation between framework and application responsibilities.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    WHY NOT ResumeCheckpoint?                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  A ResumeCheckpoint(ctx, orchestrator, checkpoint) helper would:            │
│                                                                              │
│  ❌ Require framework to call application's processing methods              │
│  ❌ Couple framework to specific method signatures (ProcessWithStreaming)   │
│  ❌ Violate Interface-First Design (depends on concrete Orchestrator)       │
│  ❌ Make testing harder (need to mock orchestrator in framework tests)      │
│  ❌ Limit flexibility (what if app needs custom session handling?)          │
│                                                                              │
│  BuildResumeContext instead:                                                 │
│                                                                              │
│  ✅ Framework only prepares the context (its responsibility)                │
│  ✅ Application controls execution (its responsibility)                     │
│  ✅ No coupling to application method signatures                            │
│  ✅ Easy to test (pure function, returns context)                           │
│  ✅ Maximum flexibility for application-specific needs                      │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

##### Framework vs Application Responsibility

| Responsibility | Owner | Rationale |
|----------------|-------|-----------|
| Prepare resume context with checkpoint state | **Framework** | Same pattern for all apps, encapsulates complexity |
| Validate checkpoint is resumable | **Framework** | Canonical definition of resumable statuses |
| Inject WithResumeMode, WithPlanOverride, etc. | **Framework** | Knows which context helpers are needed |
| Decide whether to resume | **Application** | Business logic varies per app |
| Call processing method | **Application** | App controls its own orchestrator/methods |
| Handle session/user context | **Application** | App knows its session management |
| Handle errors from resume | **Application** | App decides error handling strategy |

##### Implementation

```go
// In orchestration/hitl_helpers.go (FRAMEWORK CODE)

// BuildResumeContext prepares a context for HITL resume execution.
//
// This helper encapsulates the context setup pattern from agent-with-human-approval/handlers.go:
//   - WithResumeMode(ctx, checkpoint.CheckpointID)
//   - WithPlanOverride(ctx, checkpoint.Plan)
//   - WithCompletedSteps(ctx, checkpoint.StepResults)
//   - WithPreResolvedParams(ctx, checkpoint.ResolvedParameters, stepID)
//
// The framework prepares the context; the application uses it with its own processing method.
// This keeps the framework decoupled from application-specific execution patterns.
//
// Usage in expiry callback:
//
//     checkpointStore.SetExpiryCallback(func(ctx context.Context, cp *ExecutionCheckpoint, action CommandType) {
//         // Application decides: should we resume?
//         if !orchestration.IsResumableStatus(cp.Status) || action != CommandApprove {
//             return
//         }
//
//         // Framework prepares the context
//         resumeCtx, err := orchestration.BuildResumeContext(ctx, cp)
//         if err != nil {
//             log.Error("Failed to build resume context", "error", err)
//             return
//         }
//
//         // Application executes the resume using its own method
//         sessionID := cp.UserContext["session_id"].(string)
//         agent.ProcessWithStreaming(resumeCtx, sessionID, cp.OriginalRequest, callback)
//     })
func BuildResumeContext(ctx context.Context, checkpoint *ExecutionCheckpoint) (context.Context, error) {
    if checkpoint == nil {
        return nil, fmt.Errorf("checkpoint cannot be nil")
    }

    // Validate checkpoint is resumable
    if !IsResumableStatus(checkpoint.Status) {
        return nil, fmt.Errorf("checkpoint %s has non-resumable status %q "+
            "(only approved, edited, or expired_approved checkpoints can be resumed)",
            checkpoint.CheckpointID, checkpoint.Status)
    }

    // Build resume context with all required helpers
    // Each helper injects state needed for the orchestrator/executor to resume correctly
    resumeCtx := WithResumeMode(ctx, checkpoint.CheckpointID)

    if checkpoint.Plan != nil {
        // Inject the approved plan so orchestrator skips LLM planning
        resumeCtx = WithPlanOverride(resumeCtx, checkpoint.Plan)
    }

    if len(checkpoint.StepResults) > 0 {
        // Inject completed steps so executor skips already-executed steps
        resumeCtx = WithCompletedSteps(resumeCtx, checkpoint.StepResults)
    }

    if checkpoint.ResolvedParameters != nil && checkpoint.CurrentStep != nil {
        // Inject pre-resolved params so executor uses approved values
        resumeCtx = WithPreResolvedParams(
            resumeCtx,
            checkpoint.ResolvedParameters,
            checkpoint.CurrentStep.StepID,
        )
    }

    return resumeCtx, nil
}
```

##### Complete Usage Example

```go
// ═══════════════════════════════════════════════════════════════════════════
// APPLICATION CODE: Setting up the expiry callback
// ═══════════════════════════════════════════════════════════════════════════

func (agent *HITLChatAgent) setupExpiryHandler(checkpointStore *orchestration.RedisCheckpointStore) {
    checkpointStore.SetExpiryCallback(func(ctx context.Context, cp *orchestration.ExecutionCheckpoint, action orchestration.CommandType) {
        // ┌────────────────────────────────────────────────────────────────┐
        // │  STEP 1: Application decides whether to resume                 │
        // │  This is business logic - framework doesn't know your rules    │
        // └────────────────────────────────────────────────────────────────┘
        if !orchestration.IsResumableStatus(cp.Status) {
            agent.Logger.Info("Checkpoint not resumable", "status", cp.Status)
            agent.notifyUser(cp, "Your request expired")
            return
        }

        if action != orchestration.CommandApprove {
            agent.Logger.Info("Checkpoint expired with non-approve action", "action", action)
            agent.notifyUser(cp, "Your request was not approved")
            return
        }

        // ┌────────────────────────────────────────────────────────────────┐
        // │  STEP 2: Framework builds the resume context                   │
        // │  Framework knows which context helpers are needed              │
        // └────────────────────────────────────────────────────────────────┘
        resumeCtx, err := orchestration.BuildResumeContext(ctx, cp)
        if err != nil {
            agent.Logger.Error("Failed to build resume context", "error", err)
            agent.notifyUser(cp, "Failed to resume: "+err.Error())
            return
        }

        // ┌────────────────────────────────────────────────────────────────┐
        // │  STEP 3: Application executes the resume                       │
        // │  Application calls its own method with its own session mgmt    │
        // └────────────────────────────────────────────────────────────────┘
        sessionID, _ := cp.UserContext["session_id"].(string)
        callback := agent.getSSECallbackForSession(sessionID)

        if err := agent.ProcessWithStreaming(resumeCtx, sessionID, cp.OriginalRequest, callback); err != nil {
            agent.Logger.Error("Resume execution failed", "error", err)
            agent.notifyUser(cp, "Execution failed: "+err.Error())
            return
        }

        agent.Logger.Info("Resume completed successfully", "checkpoint_id", cp.CheckpointID)
    })
}
```

#### 1c. Cross-Trace Correlation for Expiry Callbacks [REQUIRED]

When an expiry callback triggers auto-resume, the resulting trace should be linked back to the
original request trace for end-to-end observability in Jaeger. This follows the same patterns
used in `agent-with-human-approval/handlers.go` for manual HITL resume.

**Why this matters:**
- Without trace linking, expiry-triggered resumes appear as isolated traces in Jaeger
- Operators cannot follow the complete request journey across HITL boundaries
- Searching by `original_request_id` should find ALL traces in the HITL conversation

**Key patterns from `agent-with-human-approval/handlers.go`:**

1. **`telemetry.StartLinkedSpan`** - Creates a new trace with a "link" to the original trace
2. **`telemetry.WithBaggage`** - Propagates `original_request_id` through downstream spans
3. **Checkpoint `UserContext` fields** - `original_trace_id` and `original_span_id` are stored by the framework

```go
// ═══════════════════════════════════════════════════════════════════════════
// Cross-Trace Correlation in Expiry Callback
// Per agent-with-human-approval/handlers.go patterns
// ═══════════════════════════════════════════════════════════════════════════

func (agent *HITLChatAgent) setupExpiryHandlerWithTracing(checkpointStore *orchestration.RedisCheckpointStore) {
    checkpointStore.SetExpiryCallback(func(ctx context.Context, cp *orchestration.ExecutionCheckpoint, action orchestration.CommandType) {
        // Step 1: Application decides whether to resume (same as before)
        if !orchestration.IsResumableStatus(cp.Status) || action != orchestration.CommandApprove {
            agent.notifyUser(cp, "Your request expired")
            return
        }

        // ┌────────────────────────────────────────────────────────────────────┐
        // │  STEP 2: Extract trace context from checkpoint                     │
        // │  The framework stores these in UserContext during checkpoint save  │
        // └────────────────────────────────────────────────────────────────────┘
        var originalTraceID, originalSpanID string
        if cp.UserContext != nil {
            if tid, ok := cp.UserContext["original_trace_id"].(string); ok {
                originalTraceID = tid
            }
            if sid, ok := cp.UserContext["original_span_id"].(string); ok {
                originalSpanID = sid
            }
        }

        // Determine original_request_id for trace correlation across HITL conversation
        originalRequestID := cp.RequestID // This checkpoint's request_id

        // ┌────────────────────────────────────────────────────────────────────┐
        // │  STEP 3: Create linked span for Jaeger cross-trace correlation     │
        // │  This creates a NEW trace with a "link" to the original trace      │
        // │  The link allows Jaeger to show both traces are related            │
        // └────────────────────────────────────────────────────────────────────┘
        ctx, endLinkedSpan := telemetry.StartLinkedSpan(
            ctx,
            "hitl.expiry_resume",
            originalTraceID,
            originalSpanID,
            map[string]string{
                "checkpoint_id":       cp.CheckpointID,
                "request_id":          cp.RequestID,
                "original_request_id": originalRequestID,
                "link.type":           "hitl_expiry_resume",
                "trigger":             "expiry_processor",
            },
        )
        defer endLinkedSpan()

        // ┌────────────────────────────────────────────────────────────────────┐
        // │  STEP 4: Set original_request_id in baggage for downstream spans   │
        // │  This ensures all child spans can be found by searching Jaeger     │
        // │  for original_request_id                                           │
        // └────────────────────────────────────────────────────────────────────┘
        ctx = telemetry.WithBaggage(ctx, "original_request_id", originalRequestID)

        // Add span event for Jaeger visibility
        telemetry.AddSpanEvent(ctx, "hitl.expiry_resume.started",
            attribute.String("request_id", cp.RequestID),
            attribute.String("checkpoint_id", cp.CheckpointID),
            attribute.String("original_trace_id", originalTraceID),
            attribute.String("trigger", "expiry_processor"),
        )

        // Step 5: Framework builds the resume context (same as before)
        resumeCtx, err := orchestration.BuildResumeContext(ctx, cp)
        if err != nil {
            telemetry.RecordSpanError(ctx, err)
            agent.Logger.ErrorWithContext(ctx, "Failed to build resume context", map[string]interface{}{
                "operation":     "hitl_expiry_resume",
                "checkpoint_id": cp.CheckpointID,
                "error":         err.Error(),
            })
            return
        }

        // Step 6: Application executes the resume
        sessionID, _ := cp.UserContext["session_id"].(string)
        if err := agent.ProcessWithStreaming(resumeCtx, sessionID, cp.OriginalRequest, nil); err != nil {
            telemetry.RecordSpanError(ctx, err)
            agent.Logger.ErrorWithContext(ctx, "Expiry resume execution failed", map[string]interface{}{
                "operation":     "hitl_expiry_resume",
                "checkpoint_id": cp.CheckpointID,
                "error":         err.Error(),
            })
            return
        }

        telemetry.AddSpanEvent(ctx, "hitl.expiry_resume.completed",
            attribute.String("request_id", cp.RequestID),
            attribute.String("checkpoint_id", cp.CheckpointID),
        )

        agent.Logger.InfoWithContext(ctx, "Expiry resume completed successfully", map[string]interface{}{
            "operation":     "hitl_expiry_resume",
            "checkpoint_id": cp.CheckpointID,
            "session_id":    sessionID,
        })
    })
}
```

**How trace IDs are stored in checkpoint:**

The framework automatically stores trace context in `checkpoint.UserContext` when creating
checkpoints. This is done in `hitl_controller.go`:

```go
// In hitl_controller.go - CreateCheckpoint or similar
// The framework extracts current trace context and stores it for later resume
tc := telemetry.GetTraceContext(ctx)
if tc.TraceID != "" {
    checkpoint.UserContext["original_trace_id"] = tc.TraceID
    checkpoint.UserContext["original_span_id"] = tc.SpanID
}
```

**Trace visualization in Jaeger:**

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    End-to-End HITL Trace in Jaeger                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  TRACE 1: Original Request (trace_id: abc123)                               │
│  ───────────────────────────────────────────────                            │
│  orchestration.process_request                                              │
│    ├── llm.plan_generation                                                  │
│    ├── hitl.plan_approval_required                                          │
│    └── hitl.checkpoint.created (checkpoint_id: cp-xyz)                      │
│        └── [LINK] ────────────────────────────────────────┐                 │
│                                                            │                 │
│  TRACE 2: Expiry Resume (trace_id: def456)                │                 │
│  ─────────────────────────────────────────  <─────────────┘                 │
│  hitl.expiry_resume                                                         │
│    ├── [attributes: original_trace_id=abc123, link.type=hitl_expiry_resume] │
│    ├── orchestration.execute_plan                                           │
│    │   ├── step.weather-tool                                                │
│    │   └── step.currency-tool                                               │
│    └── hitl.expiry_resume.completed                                         │
│                                                                              │
│  Searching Jaeger for original_request_id=req-001 shows BOTH traces         │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### 1b. IsResumableStatus Helper [MEDIUM]

The `IsResumableStatus` helper provides consistent status checking across applications.

```go
// In orchestration/hitl_helpers.go (FRAMEWORK CODE)

// IsResumableStatus returns true if the checkpoint status allows resumption.
// This is the canonical definition of which statuses can be resumed.
//
// Resumable statuses:
//   - expired_approved: Auto-approved on timeout, can be resumed
//   - approved: Human approved, can be resumed
//   - edited: Human edited and approved, can be resumed
//
// Non-resumable statuses:
//   - pending: Still waiting for decision
//   - expired: Implicit deny, user must explicitly request resume
//   - expired_rejected: Auto-rejected, cannot resume
//   - expired_aborted: Auto-aborted, cannot resume
//   - rejected: Human rejected, cannot resume
//   - aborted: Human aborted, cannot resume
//   - completed: Already completed, nothing to resume
func IsResumableStatus(status CheckpointStatus) bool {
    switch status {
    case CheckpointStatusExpiredApproved,
         CheckpointStatusApproved,
         CheckpointStatusEdited:
        return true
    default:
        return false
    }
}

// Checkpoint status constants for clarity
const (
    // Human-initiated statuses
    CheckpointStatusPending   CheckpointStatus = "pending"
    CheckpointStatusApproved  CheckpointStatus = "approved"
    CheckpointStatusRejected  CheckpointStatus = "rejected"
    CheckpointStatusEdited    CheckpointStatus = "edited"
    CheckpointStatusAborted   CheckpointStatus = "aborted"
    CheckpointStatusCompleted CheckpointStatus = "completed"

    // Expiry statuses - implicit deny (no action applied)
    CheckpointStatusExpired CheckpointStatus = "expired"

    // Expiry statuses - policy-driven action applied
    CheckpointStatusExpiredApproved CheckpointStatus = "expired_approved"
    CheckpointStatusExpiredRejected CheckpointStatus = "expired_rejected"
    CheckpointStatusExpiredAborted  CheckpointStatus = "expired_aborted"
)
```

### Item 3: Distributed Concurrency (Multi-Pod Safety) [MEDIUM]

In production, multiple pods may run the same agent. The expiry processor must ensure that
only ONE instance processes each expired checkpoint. Per `FRAMEWORK_DESIGN_PRINCIPLES.md`
"Built-in Reliability" principle, this is handled by the framework automatically.

#### 3a. Three-Layer Resilience Architecture [REQUIRED]

Per `orchestration/ARCHITECTURE.md` Section 9 "Resilience & Fault Tolerance", the expiry
processor follows the **Three-Layer Resilience Architecture**:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    THREE-LAYER RESILIENCE FOR EXPIRY PROCESSOR              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Layer 1: Simple Built-in Resilience (ALWAYS ACTIVE)                        │
│  ─────────────────────────────────────────────────────                       │
│  • 3 retries with exponential backoff (50ms, 100ms, 200ms)                  │
│  • Simple failure tracking (5 failures → 30s cooldown)                      │
│  • Timeout protection (30s default)                                         │
│  • No external dependencies - works out of the box                          │
│                                                                              │
│  Layer 2: Circuit Breaker (OPTIONAL, INJECTED)                              │
│  ─────────────────────────────────────────────────                          │
│  • Full circuit breaker pattern (closed/open/half-open states)              │
│  • Sliding window metrics                                                    │
│  • Provided by APPLICATION, not framework                                   │
│  • Injected via CheckpointStoreDependencies                                 │
│                                                                              │
│  Layer 3: Graceful Degradation (CONFIGURABLE)                               │
│  ─────────────────────────────────────────────────                          │
│  • Skip scan on failure, retry next interval                                │
│  • Log warning with actionable remediation                                  │
│  • Continue processing other operations                                     │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

##### Implementation with Three-Layer Resilience

```go
// In hitl_checkpoint_store.go (FRAMEWORK CODE)

// processExpiredCheckpoints implements Three-Layer Resilience.
// Per ARCHITECTURE.md: "Layer 2 is optional, injected; Layer 1 is always active"
func (s *RedisCheckpointStore) processExpiredCheckpoints(batchSize int) {
    var err error

    // Layer 2: Use injected circuit breaker if provided
    if s.circuitBreaker != nil {
        err = s.circuitBreaker.Execute(s.expiryCtx, func() error {
            return s.doProcessExpiredCheckpoints(batchSize)
        })
    } else {
        // Layer 1: Use built-in simple resilience
        err = s.doProcessExpiredCheckpointsWithRetry(batchSize)
    }

    // Layer 3: Graceful degradation on failure
    if err != nil {
        if s.logger != nil {
            s.logger.WarnWithContext(s.expiryCtx, "Expiry scan skipped due to error", map[string]interface{}{
                "operation": "hitl_expiry_processor",
                "error":     err.Error(),
                "note":      "Will retry on next interval. Check Redis connectivity.",
            })
        }
        // Record metric using helper function (per hitl_metrics.go pattern)
        RecordExpiryScanSkipped("error")
        // Don't crash - just skip this interval and try again next time
    }
}

// doProcessExpiredCheckpointsWithRetry implements Layer 1 built-in resilience.
// This is used when no circuit breaker is injected.
func (s *RedisCheckpointStore) doProcessExpiredCheckpointsWithRetry(batchSize int) error {
    var lastErr error
    delays := []time.Duration{50 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond}

    for attempt, delay := range delays {
        err := s.doProcessExpiredCheckpoints(batchSize)
        if err == nil {
            return nil
        }
        lastErr = err

        if attempt < len(delays)-1 {
            select {
            case <-time.After(delay):
                // Continue to retry
            case <-s.expiryCtx.Done():
                return s.expiryCtx.Err()
            }
        }
    }

    return fmt.Errorf("expiry scan failed after %d retries: %w", len(delays), lastErr)
}
```

##### Application Usage - Injecting Circuit Breaker

```go
// APPLICATION CODE: Production setup with circuit breaker

// Create circuit breaker using resilience module
cb, _ := resilience.NewCircuitBreaker(&resilience.CircuitBreakerConfig{
    Name:            "hitl-checkpoint-store",
    ErrorThreshold:  0.5,
    VolumeThreshold: 10,
    SleepWindow:     30 * time.Second,
})

// Inject via dependencies
deps := &orchestration.CheckpointStoreDependencies{
    CircuitBreaker: cb,
    Logger:         logger,
    Telemetry:      telemetry,
}

store, _ := orchestration.CreateCheckpointStore(redisURL, "hitl:", deps)
```

##### Why This Design?

Per `ARCHITECTURE.md` Section "Why Not Import Resilience Directly?":
- **Framework Rule**: Orchestration can only import `core` + `telemetry`
- **Separation of Concerns**: Framework provides capability, apps choose implementation
- **Flexibility**: Apps might use different circuit breaker libraries
- **Testability**: Can test orchestration without resilience module

#### 3b. Claim Mechanism with Redis SETNX

The framework uses Redis SETNX (SET if Not eXists) with instance ID for distributed locking:

```go
// In hitl_checkpoint_store.go (FRAMEWORK CODE)

// claimExpiredCheckpoint attempts to claim exclusive processing rights for a checkpoint.
// Returns true if this instance successfully claimed it, false if another instance claimed it.
//
// Uses Redis SETNX with TTL for distributed locking:
//   - Key: {keyPrefix}:expiry:claim:{checkpointID}
//   - Value: instanceID (unique per pod)
//   - TTL: 30 seconds (prevents orphaned claims if pod crashes)
//
// This ensures that in a multi-pod deployment, only ONE pod processes each expired checkpoint.
func (s *RedisCheckpointStore) claimExpiredCheckpoint(ctx context.Context, checkpointID string) (bool, error) {
    claimKey := fmt.Sprintf("%s:expiry:claim:%s", s.keyPrefix, checkpointID)
    claimTTL := 30 * time.Second

    // SETNX with TTL - only succeeds if key doesn't exist
    success, err := s.client.SetNX(ctx, claimKey, s.instanceID, claimTTL).Result()
    if err != nil {
        return false, fmt.Errorf("failed to claim checkpoint %s via Redis SETNX at key %s: %w (check Redis connectivity and permissions)",
            checkpointID, claimKey, err)
    }

    if success && s.logger != nil {
        s.logger.DebugWithContext(ctx, "Claimed expired checkpoint", map[string]interface{}{
            "operation":     "hitl_expiry_claim",
            "checkpoint_id": checkpointID,
            "instance_id":   s.instanceID,
        })
    }

    return success, nil
}

// releaseExpiredCheckpointClaim releases the claim after processing.
// Only releases if this instance holds the claim (check-and-delete).
func (s *RedisCheckpointStore) releaseExpiredCheckpointClaim(ctx context.Context, checkpointID string) error {
    claimKey := fmt.Sprintf("%s:expiry:claim:%s", s.keyPrefix, checkpointID)

    // Use Lua script for atomic check-and-delete
    script := `
        if redis.call("GET", KEYS[1]) == ARGV[1] then
            return redis.call("DEL", KEYS[1])
        end
        return 0
    `
    _, err := s.client.Eval(ctx, script, []string{claimKey}, s.instanceID).Result()
    return err
}
```

#### Updated processExpiredCheckpoints with Claim Mechanism

```go
func (s *RedisCheckpointStore) processExpiredCheckpoints(batchSize int) {
    // Track scan duration for metrics (per hitl_metrics.go helper pattern)
    scanStartTime := time.Now()

    ctx, cancel := context.WithTimeout(s.expiryCtx, 30*time.Second)
    defer cancel()

    // Get all checkpoint IDs from pending index
    pendingKey := fmt.Sprintf("%s:pending", s.keyPrefix)
    checkpointIDs, err := s.client.SMembers(ctx, pendingKey).Result()
    if err != nil {
        // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 4: Record error on span
        telemetry.RecordSpanError(ctx, err)

        if s.logger != nil {
            // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 2: operation field required
            s.logger.WarnWithContext(ctx, "Failed to read pending index", map[string]interface{}{
                "operation": "hitl_expiry_processor",
                "error":     err.Error(),
            })
        }
        RecordExpiryScanSkipped("read_pending_index_failed")
        return
    }

    now := time.Now()
    processed := 0

    for _, cpID := range checkpointIDs {
        if processed >= batchSize {
            break
        }

        checkpoint, err := s.LoadCheckpoint(ctx, cpID)
        if err != nil {
            // Checkpoint may have been deleted (TTL) - remove from index
            s.client.SRem(ctx, pendingKey, cpID)
            continue
        }

        // Check if expired
        if checkpoint.Status != CheckpointStatusPending || checkpoint.ExpiresAt.After(now) {
            continue
        }

        // ┌────────────────────────────────────────────────────────────────┐
        // │  DISTRIBUTED CLAIM MECHANISM                                   │
        // │                                                                │
        // │  In multi-pod deployments, multiple expiry processors may be   │
        // │  running. We use Redis SETNX to ensure only ONE pod processes  │
        // │  each checkpoint. Per FRAMEWORK_DESIGN_PRINCIPLES.md, this is  │
        // │  built-in reliability, not a documented limitation.            │
        // └────────────────────────────────────────────────────────────────┘

        claimed, err := s.claimExpiredCheckpoint(ctx, cpID)
        if err != nil {
            // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 4: Record error on span
            telemetry.RecordSpanError(ctx, err)

            if s.logger != nil {
                // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 2: operation field required
                s.logger.WarnWithContext(ctx, "Failed to claim checkpoint", map[string]interface{}{
                    "operation":     "hitl_expiry_processor",
                    "checkpoint_id": cpID,
                    "error":         err.Error(),
                })
            }
            continue
        }

        if !claimed {
            // Another instance is processing this checkpoint
            if s.logger != nil {
                s.logger.DebugWithContext(ctx, "Checkpoint claimed by another instance", map[string]interface{}{
                    "operation":     "hitl_expiry_skip",
                    "checkpoint_id": cpID,
                })
            }
            continue
        }

        // Process the checkpoint (existing logic)...
        // [Mode-aware expiry behavior code here]

        // Release claim after successful processing
        defer s.releaseExpiredCheckpointClaim(ctx, cpID)

        processed++
    }

    // Record scan metrics using helper functions (per hitl_metrics.go pattern)
    RecordExpiryScanDuration(time.Since(scanStartTime).Seconds())
    RecordExpiryBatchSize(processed)
}
```

#### Instance ID Configuration

```go
// In NewRedisCheckpointStore (FRAMEWORK CODE)

type RedisCheckpointStore struct {
    client     *redis.Client
    keyPrefix  string
    ttl        time.Duration
    logger     core.Logger

    // Instance ID for distributed claim mechanism
    // Unique per pod - defaults to hostname + random suffix
    instanceID string

    // Expiry processor fields...
}

// WithInstanceID sets a custom instance ID for distributed claim mechanism.
// If not set, defaults to hostname + random suffix.
func WithInstanceID(id string) CheckpointStoreOption {
    return func(s *RedisCheckpointStore) {
        s.instanceID = id
    }
}

// WithExpiryCircuitBreaker sets an optional circuit breaker for expiry processor Redis operations.
// Per ARCHITECTURE.md Section 9: "Circuit breaker is provided by application, not framework"
//
// If provided, the circuit breaker wraps Redis scan operations in the expiry processor.
// If not provided, Layer 1 built-in resilience (simple retry with backoff) is used.
//
// Example:
//
//     cb, _ := resilience.NewCircuitBreaker(&resilience.CircuitBreakerConfig{
//         Name:            "hitl-expiry-redis",
//         ErrorThreshold:  0.5,
//         VolumeThreshold: 10,
//         SleepWindow:     30 * time.Second,
//     })
//     store := NewRedisCheckpointStore(
//         WithCheckpointRedisURL(redisURL),
//         WithExpiryCircuitBreaker(cb),
//     )
func WithExpiryCircuitBreaker(cb core.CircuitBreaker) CheckpointStoreOption {
    return func(s *RedisCheckpointStore) {
        s.circuitBreaker = cb
    }
}

// In NewRedisCheckpointStore
func NewRedisCheckpointStore(opts ...CheckpointStoreOption) (*RedisCheckpointStore, error) {
    s := &RedisCheckpointStore{
        // ... defaults ...
    }

    // Apply options
    for _, opt := range opts {
        opt(s)
    }

    // Generate default instance ID if not provided
    if s.instanceID == "" {
        hostname, _ := os.Hostname()
        s.instanceID = fmt.Sprintf("%s-%s", hostname, generateRandomSuffix())
    }

    return s, nil
}
```

### Item 4: Callback Error Recovery [MEDIUM]

Per `FRAMEWORK_DESIGN_PRINCIPLES.md`, the framework should handle callback errors gracefully.
A panicking callback should not crash the expiry processor or affect other checkpoints.

#### 4a. Panic Recovery [MEDIUM]

```go
// In hitl_checkpoint_store.go (FRAMEWORK CODE)

// invokeExpiryCallback safely invokes the callback with panic recovery.
// Returns error if callback panicked or returned an error.
func (s *RedisCheckpointStore) invokeExpiryCallback(
    ctx context.Context,
    checkpoint *ExecutionCheckpoint,
    action CommandType,
) (err error) {
    if s.expiryCallback == nil {
        return nil
    }

    // Panic recovery - callback should never crash the processor
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("expiry callback panicked: %v", r)

            // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 4: Record error on span FIRST
            telemetry.RecordSpanError(ctx, err)

            if s.logger != nil {
                // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 2: operation field required
                s.logger.ErrorWithContext(ctx, "Expiry callback panicked", map[string]interface{}{
                    "operation":     "hitl_expiry_callback_panic",
                    "checkpoint_id": checkpoint.CheckpointID,
                    "request_id":    checkpoint.RequestID,
                    "panic":         fmt.Sprintf("%v", r),
                    "stack":         string(debug.Stack()),
                })
            }

            // Record metric using helper function (per hitl_metrics.go pattern)
            // NOTE: checkpoint_id is logged above but NOT in the metric (high-cardinality)
            RecordCallbackPanic()
        }
    }()

    s.expiryCallback(ctx, checkpoint, action)
    return nil
}
```

#### 4b. Configurable Delivery Semantics [MEDIUM]

Different applications have different requirements for callback delivery:

- **At-most-once**: Status updated BEFORE callback. If callback fails, checkpoint won't be reprocessed.
  Best for: Idempotent operations, notifications.

- **At-least-once**: Callback invoked BEFORE status update. If callback fails, checkpoint may be
  reprocessed. Best for: Critical operations that must complete.

```go
// In hitl_interfaces.go (FRAMEWORK CODE)

// DeliverySemantics controls callback invocation timing relative to status update.
type DeliverySemantics string

const (
    // DeliveryAtMostOnce updates status BEFORE callback.
    // If callback fails, checkpoint is already marked processed - no retry.
    // Use for: Notifications, idempotent operations, fire-and-forget.
    DeliveryAtMostOnce DeliverySemantics = "at_most_once"

    // DeliveryAtLeastOnce invokes callback BEFORE status update.
    // If callback fails, status remains "pending" and will be retried.
    // Use for: Critical operations that must complete (with idempotent callback).
    DeliveryAtLeastOnce DeliverySemantics = "at_least_once"
)

// ExpiryProcessorConfig with delivery semantics
type ExpiryProcessorConfig struct {
    Enabled           bool
    ScanInterval      time.Duration
    BatchSize         int
    DeliverySemantics DeliverySemantics // Default: DeliveryAtMostOnce
}

// NOTE: Circuit breaker is injected via WithExpiryCircuitBreaker(), not configured here.
// Per ARCHITECTURE.md: "Circuit breaker is provided by application, not framework"
```

#### Implementation with Configurable Delivery

```go
func (s *RedisCheckpointStore) processExpiredCheckpoint(
    ctx context.Context,
    checkpoint *ExecutionCheckpoint,
    newStatus CheckpointStatus,
    appliedAction CommandType,
) error {
    switch s.config.DeliverySemantics {
    case DeliveryAtLeastOnce:
        // Callback FIRST, then update status
        // If callback fails, checkpoint remains pending for retry
        if err := s.invokeExpiryCallback(ctx, checkpoint, appliedAction); err != nil {
            if s.logger != nil {
                s.logger.WarnWithContext(ctx, "Callback failed, will retry", map[string]interface{}{
                    "operation":     "hitl_expiry_callback_retry",
                    "checkpoint_id": checkpoint.CheckpointID,
                    "error":         err.Error(),
                })
            }
            return err // Don't update status - will be retried
        }
        // Callback succeeded, now update status
        return s.UpdateCheckpointStatus(ctx, checkpoint.CheckpointID, newStatus)

    case DeliveryAtMostOnce:
        fallthrough
    default:
        // Update status FIRST, then callback
        // If callback fails, checkpoint is already marked - no retry
        if err := s.UpdateCheckpointStatus(ctx, checkpoint.CheckpointID, newStatus); err != nil {
            return err
        }
        checkpoint.Status = newStatus
        _ = s.invokeExpiryCallback(ctx, checkpoint, appliedAction) // Ignore callback error
        return nil
    }
}
```

### Item 5: Graceful Shutdown [MEDIUM]

The expiry processor should complete in-progress work before shutting down.
This is especially important for `DeliveryAtLeastOnce` semantics.

> **Priority Upgrade Note**: Originally LOW, upgraded to MEDIUM because
> `FRAMEWORK_DESIGN_PRINCIPLES.md` Section "Component Lifecycle Rules → Graceful Shutdown"
> explicitly requires: "All components must handle context cancellation" and
> "Close external connections cleanly". This is a core framework requirement, not optional.

```go
// In hitl_checkpoint_store.go (FRAMEWORK CODE)

// StopExpiryProcessor stops the processor gracefully.
// Accepts context for timeout control - caller decides the timeout.
// Per FRAMEWORK_DESIGN_PRINCIPLES.md: context propagation for lifecycle operations.
func (s *RedisCheckpointStore) StopExpiryProcessor(ctx context.Context) error {
    if s.expiryCancel == nil {
        return nil
    }

    // Signal shutdown
    s.expiryCancel()

    // Wait for graceful completion with context timeout
    done := make(chan struct{})
    go func() {
        s.expiryWg.Wait()
        close(done)
    }()

    select {
    case <-done:
        if s.logger != nil {
            // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 2: operation field required
            s.logger.Info("Expiry processor stopped gracefully", map[string]interface{}{
                "operation": "hitl_expiry_processor_stop",
            })
        }
        return nil
    case <-ctx.Done():
        if s.logger != nil {
            // Per DISTRIBUTED_TRACING_GUIDE.md Pattern 2: operation field required
            s.logger.Warn("Expiry processor shutdown cancelled", map[string]interface{}{
                "operation": "hitl_expiry_processor_stop",
                "error":     ctx.Err().Error(),
                "note":      "Some checkpoints may need reprocessing",
            })
        }
        return fmt.Errorf("expiry processor shutdown cancelled: %w", ctx.Err())
    }
}

// Usage in agent's shutdown handler:
//
//     sigChan := make(chan os.Signal, 1)
//     signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
//
//     go func() {
//         <-sigChan
//         log.Println("Shutting down...")
//
//         // Stop accepting new requests
//         server.Shutdown(ctx)
//
//         // Stop expiry processor (waits for in-progress work, with 30s timeout)
//         shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//         defer cancel()
//         checkpointStore.StopExpiryProcessor(shutdownCtx)
//
//         log.Println("Shutdown complete")
//     }()
```

### Item 6: Configuration Integration [MEDIUM]

The expiry processor configuration integrates with the existing `HITLConfig` pattern
used throughout the orchestration package.

#### Extended HITLConfig

```go
// In hitl_interfaces.go

// HITLConfig is the unified configuration for HITL features.
// Per FRAMEWORK_DESIGN_PRINCIPLES.md, uses functional options pattern.
type HITLConfig struct {
    // Existing fields...
    Enabled        bool
    DefaultTimeout time.Duration
    WebhookURL     string

    // Expiry processor configuration
    ExpiryProcessor ExpiryProcessorConfig
}

// ExpiryProcessorConfig is the expiry processor specific config
type ExpiryProcessorConfig struct {
    Enabled           bool              // Enable expiry processing (default: true)
    ScanInterval      time.Duration     // Scan interval (default: 10s)
    BatchSize         int               // Max per scan (default: 100)
    DeliverySemantics DeliverySemantics // Callback timing (default: at_most_once)
}

// NOTE: Circuit breaker is injected via WithExpiryCircuitBreaker(), not configured here.
// Per ARCHITECTURE.md Section 9: "Circuit breaker is provided by application, not framework"

// WithExpiryProcessor configures the expiry processor with smart defaults.
// Per FRAMEWORK_DESIGN_PRINCIPLES.md "Intelligent Configuration Over Convention".
func WithExpiryProcessor(config ExpiryProcessorConfig) HITLOption {
    return func(h *HITLConfig) {
        // Apply smart defaults when intent is clear
        if config.Enabled && config.ScanInterval == 0 {
            config.ScanInterval = 10 * time.Second
        }
        if config.Enabled && config.BatchSize == 0 {
            config.BatchSize = 100
        }
        if config.DeliverySemantics == "" {
            config.DeliverySemantics = DeliveryAtMostOnce
        }

        h.ExpiryProcessor = config
    }
}
```

#### 6a. Configuration Validation [REQUIRED]

Per `FRAMEWORK_DESIGN_PRINCIPLES.md` Section "Fail-Fast for Configuration Errors":
> "Configuration problems should fail immediately"

`StartExpiryProcessor()` MUST validate configuration and return an error for invalid values:

```go
// In hitl_checkpoint_store.go (FRAMEWORK CODE)

// validateExpiryConfig validates the expiry processor configuration.
// Returns actionable error message if validation fails.
// Per FRAMEWORK_DESIGN_PRINCIPLES.md: fail-fast for configuration errors.
func validateExpiryConfig(config ExpiryProcessorConfig) error {
    if !config.Enabled {
        return nil // Disabled config is always valid
    }

    if config.ScanInterval > 0 && config.ScanInterval < 1*time.Second {
        return fmt.Errorf("ExpiryProcessorConfig.ScanInterval must be at least 1s, got %v "+
            "(use 0 for default of 10s, or set explicit value >= 1s)", config.ScanInterval)
    }

    if config.BatchSize < 0 {
        return fmt.Errorf("ExpiryProcessorConfig.BatchSize cannot be negative, got %d "+
            "(use 0 for default of 100, or set explicit positive value)", config.BatchSize)
    }

    if config.BatchSize > 10000 {
        return fmt.Errorf("ExpiryProcessorConfig.BatchSize exceeds maximum of 10000, got %d "+
            "(large batch sizes can cause memory issues and Redis timeouts)", config.BatchSize)
    }

    switch config.DeliverySemantics {
    case "", DeliveryAtMostOnce, DeliveryAtLeastOnce:
        // Valid
    default:
        return fmt.Errorf("ExpiryProcessorConfig.DeliverySemantics has invalid value %q "+
            "(valid values: %q, %q)", config.DeliverySemantics, DeliveryAtMostOnce, DeliveryAtLeastOnce)
    }

    // NOTE: Circuit breaker is validated at injection time (via WithExpiryCircuitBreaker),
    // not here. Per ARCHITECTURE.md: circuit breaker is an optional injected dependency.

    return nil
}

// StartExpiryProcessor validates config before starting.
func (s *RedisCheckpointStore) StartExpiryProcessor(ctx context.Context, config ExpiryProcessorConfig) error {
    // Fail-fast for configuration errors
    if err := validateExpiryConfig(config); err != nil {
        return fmt.Errorf("invalid expiry processor configuration: %w", err)
    }

    // ... rest of implementation
}
```

#### Environment Variable Mapping

| Variable | Config Field | Default |
|----------|--------------|---------|
| `GOMIND_HITL_EXPIRY_ENABLED` | `ExpiryProcessor.Enabled` | `true` |
| `GOMIND_HITL_EXPIRY_INTERVAL` | `ExpiryProcessor.ScanInterval` | `10s` |
| `GOMIND_HITL_EXPIRY_BATCH_SIZE` | `ExpiryProcessor.BatchSize` | `100` |
| `GOMIND_HITL_EXPIRY_DELIVERY` | `ExpiryProcessor.DeliverySemantics` | `at_most_once` |

> **Note**: Circuit breaker is not configured via environment variables.
> Per ARCHITECTURE.md, circuit breaker is injected by the application using `WithExpiryCircuitBreaker(cb)`.

### Item 7: Input Validation and Actionable Errors [MEDIUM]

Per `FRAMEWORK_DESIGN_PRINCIPLES.md` Section "Security Considerations":
> "Validate all external inputs"

And Section "Error Messages":
> "Error messages should be actionable"

#### 7a. Checkpoint Data Validation [REQUIRED]

Checkpoint data loaded from Redis is external input and MUST be validated before processing:

```go
// In hitl_checkpoint_store.go (FRAMEWORK CODE)

// validateCheckpointForExpiry validates checkpoint data before expiry processing.
// Returns error with actionable message if validation fails.
func validateCheckpointForExpiry(checkpoint *ExecutionCheckpoint) error {
    if checkpoint == nil {
        return fmt.Errorf("checkpoint is nil (data corruption or race condition)")
    }

    if checkpoint.CheckpointID == "" {
        return fmt.Errorf("checkpoint has empty ID (data corruption in Redis key %s)",
            checkpoint.CheckpointID)
    }

    if checkpoint.ExpiresAt.IsZero() {
        return fmt.Errorf("checkpoint %s has zero ExpiresAt timestamp "+
            "(checkpoint was created without expiry - check HITLPolicy.DefaultTimeout)",
            checkpoint.CheckpointID)
    }

    // Validate status is a known value
    switch checkpoint.Status {
    case CheckpointStatusPending, CheckpointStatusApproved, CheckpointStatusRejected,
         CheckpointStatusEdited, CheckpointStatusAborted, CheckpointStatusCompleted,
         CheckpointStatusExpired, CheckpointStatusExpiredApproved,
         CheckpointStatusExpiredRejected, CheckpointStatusExpiredAborted:
        // Valid status
    default:
        return fmt.Errorf("checkpoint %s has unknown status %q "+
            "(possible version mismatch or data corruption)",
            checkpoint.CheckpointID, checkpoint.Status)
    }

    return nil
}

// Usage in processExpiredCheckpoints:
func (s *RedisCheckpointStore) doProcessExpiredCheckpoints(batchSize int) error {
    // ... load checkpoint ...

    if err := validateCheckpointForExpiry(checkpoint); err != nil {
        if s.logger != nil {
            s.logger.ErrorWithContext(ctx, "Invalid checkpoint data, skipping", map[string]interface{}{
                "operation":     "hitl_expiry_validation",
                "checkpoint_id": cpID,
                "error":         err.Error(),
            })
        }
        // Remove from pending index to prevent repeated validation failures
        s.client.SRem(ctx, pendingKey, cpID)
        continue
    }

    // ... continue processing ...
}
```

#### 7b. Actionable Error Message Pattern [REQUIRED]

All error messages in the expiry processor MUST follow this pattern:

```go
// ✅ GOOD: Actionable error message
return fmt.Errorf("failed to load checkpoint %s from Redis: %w "+
    "(check Redis connectivity and key TTL settings)", checkpointID, err)

// ❌ BAD: Vague error message
return fmt.Errorf("failed to load checkpoint: %w", err)
```

**Error Message Template:**
```
<what failed> <context/identifiers>: <underlying error> (<remediation hint>)
```

**Examples:**
| Operation | Error Message |
|-----------|---------------|
| Load checkpoint | `"failed to load checkpoint %s from Redis: %w (check Redis connectivity and key TTL settings)"` |
| Claim checkpoint | `"failed to claim checkpoint %s via Redis SETNX at key %s: %w (check Redis connectivity and permissions)"` |
| Update status | `"failed to update checkpoint %s status to %s: %w (check Redis write permissions)"` |
| Invoke callback | `"expiry callback panicked for checkpoint %s: %v (check callback implementation for nil pointer or bounds errors)"` |

---

## Implementation Checklist

> **Last Updated:** 2026-01-21

### Phase 1: Core Types and Interfaces

> **Architectural Clarification:** The original design proposed adding expiry configuration
> fields to `HITLPolicyConfig`. The implementation instead uses a three-tier configuration
> hierarchy: `InterruptDecision` (per-checkpoint) → Environment Variables (global defaults)
> → Built-in Defaults. This provides better per-checkpoint granularity while maintaining
> simple global defaults. See **Section 4.1: Architectural Decision: Configuration Hierarchy**
> for detailed rationale.

- [x] Add `RequestMode` type and constants to `hitl_interfaces.go`
- [x] Add `RequestMode` field to `ExecutionCheckpoint` struct
- [x] Add `StreamingExpiryBehavior` type and constants to `hitl_interfaces.go`
- [x] Add `NonStreamingExpiryBehavior` type and constants to `hitl_interfaces.go`
- [x] Add all three expiry behavior fields to `InterruptDecision` struct (per-checkpoint config)
- [x] Add new `expired` status (for implicit deny)
- [x] Add new `expired_*` status constants (for apply_default)
- [x] Add `ExpiryProcessorConfig` type to `hitl_interfaces.go`
- [x] Add `ExpiryCallback` type to `hitl_interfaces.go`
- [x] Add expiry processor methods to `CheckpointStore` interface
- [x] Add `instanceID` field to `RedisCheckpointStore` struct (auto-generated)
- [N/A] ~~Add expiry fields to `HITLPolicyConfig`~~ → Replaced by `InterruptDecision` + env vars (see architectural clarification above)
- [x] Add `WithInstanceID` option for `RedisCheckpointStore` (for testing/override)
- [x] Add `DeliverySemantics` type and constants (Phase 6 - callback error recovery)

### Phase 2: Context Helpers
- [x] Add `WithRequestMode(ctx, mode)` context helper (in `hitl_helpers.go`)
- [x] Add `GetRequestMode(ctx)` context helper
- [x] Update `createCheckpoint()` in `hitl_controller.go` to read request mode from context

### Phase 3: Framework Helpers (Production-Grade)
- [x] Create `hitl_helpers.go` file
- [x] Implement `BuildResumeContext()` helper function (returns context, not execution)
- [x] Implement `IsResumableStatus()` helper function
- [x] Implement `IsTerminalStatus()` helper function
- [x] Implement `IsPendingStatus()` helper function
- [x] Add unit tests for `BuildResumeContext()` helper
- [x] Add unit tests for `IsResumableStatus()` helper

### Phase 4: Expiry Processor Implementation
- [x] Implement `StartExpiryProcessor()` in `RedisCheckpointStore`
- [x] Implement `validateExpiryConfig()` with fail-fast validation
- [x] Implement `StopExpiryProcessor(ctx context.Context) error` with graceful shutdown
- [x] Implement `SetExpiryCallback(callback ExpiryCallback) error` in `RedisCheckpointStore`
- [x] Implement `getEffectiveRequestMode()` with WARN logging and trace event
- [x] Implement `getDefaultRequestMode()` helper method
- [x] Implement `shouldApplyDefaultAction()` unified decision method
- [x] Implement `getStreamingExpiryBehavior()` helper method
- [x] Implement `getNonStreamingExpiryBehavior()` helper method
- [x] Implement configurable mode-aware expiry logic
- [x] Add metrics for expiry processing (with request_mode labels)
- [x] Add span events for tracing visibility (Jaeger)
- [x] Add environment variable support (`GOMIND_HITL_DEFAULT_REQUEST_MODE`, `GOMIND_HITL_STREAMING_EXPIRY`, `GOMIND_HITL_NON_STREAMING_EXPIRY`)
- [REMOVED] ~~Implement `InMemoryCheckpointStore` (for unit testing without Redis)~~ → Redis is required; developers should run their own Redis instance
- [ ] Wrap expiry scan with circuit breaker protection (NOTE: Per ARCHITECTURE.md, circuit breaker is application-provided, not framework)

### Phase 5: Distributed Concurrency (Multi-Pod Safety)
- [x] Implement `claimExpiredCheckpoint()` with Redis SETNX
- [x] Implement `releaseExpiredCheckpointClaim()` with Lua check-and-delete
- [N/A] ~~Implement `validateCheckpointForExpiry()` for input validation~~ → Validation is done inline in processExpiredCheckpoints loop
- [x] Generate default instance ID (hostname + random suffix)
- [x] Add `WithInstanceID()` option for custom instance ID (testing/override)
- [x] Update `processExpiredCheckpoints()` to use claim mechanism
- [x] Add metrics for claim success/skip counts (`MetricClaimSuccess`, `MetricClaimSkipped`)
- [x] Add unit tests for claim mechanism
- [DEFERRED] Add integration tests for multi-instance scenarios → Requires Redis + multi-pod setup

### Phase 6: Callback Error Recovery
- [x] Implement `invokeCallbackSafely()` with panic recovery (renamed from `invokeExpiryCallback`)
- [x] Implement configurable `DeliverySemantics` (at-most-once vs at-least-once)
- [x] Add metrics for callback panics (`MetricCallbackPanic`)
- [x] Add `RecordCallbackPanic()` helper function with unit tests
- [x] Add unit tests for both delivery semantics

### Phase 7: Configuration Integration
- [x] Extend `HITLConfig` with `ExpiryProcessor` sub-config
- [x] Add `HITLOption` type for functional options pattern
- [x] Add `WithExpiryProcessor()` functional option with smart defaults
- [x] Add `NewHITLConfig()` and `ApplyHITLOptions()` helper functions
- [x] Add environment variable support via `ExpiryProcessorConfigFromEnv()`
  - `GOMIND_HITL_EXPIRY_ENABLED` - Enable/disable (default: true)
  - `GOMIND_HITL_EXPIRY_INTERVAL` - Scan interval (default: 10s)
  - `GOMIND_HITL_EXPIRY_BATCH_SIZE` - Max per scan (default: 100)
  - `GOMIND_HITL_EXPIRY_DELIVERY` - Delivery semantics (default: at_most_once)
- [x] Add unit tests for new configuration functions

### Phase 7.5: Unit Test Infrastructure (miniredis)

> **Architectural Decision (2026-01-21):** For unit testing Redis-dependent code, we use
> `miniredis` - an in-memory Redis server implementation for Go. This is consistent with
> the established pattern in `core/schema_cache_test.go` and other framework components.

- [x] Add unit tests for checkpoint store using `miniredis` (in-memory Redis)
- [x] Create `setupCheckpointTestRedis()` helper following `core/schema_cache_test.go` pattern
- [x] Create `newCheckpointTestStore()` helper for test setup with miniredis client
- [x] Add comprehensive unit tests for checkpoint store methods in `hitl_checkpoint_store_test.go`
- [x] Add tests for: SaveCheckpoint, LoadCheckpoint, UpdateCheckpointStatus, ListPendingCheckpoints, DeleteCheckpoint
- [x] Add tests for: claimExpiredCheckpoint, releaseExpiredCheckpointClaim (distributed claim mechanism)
- [x] Add tests for: Close, StopExpiryProcessor, SetExpiryCallback
- [x] Add integration-style tests for multi-operation workflows

**Files Added:**
- `hitl_checkpoint_store_test.go` (NEW) - Unit tests with miniredis

**Design Rationale:**
- Follows established framework pattern (consistent with `core/schema_cache_test.go`)
- Tests real Redis semantics (miniredis implements actual Redis behavior)
- No interface abstraction needed - simpler production code
- `miniredis` is already a framework dependency

### Phase 8: Agent Integration

> **Design Decision (2026-01-21):** We chose the **polling approach** over real-time SSE push for
> expiry notifications. This is simpler to implement and sufficient for the use case. The UI polls
> checkpoint status periodically and detects expiry on the next poll. Real-time SSE push would
> require keeping SSE connections open or implementing Redis Pub/Sub subscription in SSE handlers.
> See `HUMAN_IN_THE_LOOP_PROPOSAL.md` Section 9.5 for details on command delivery patterns.

> **Design Decision (2026-01-21):** For Scenario 1 (Streaming + implicit_deny), expired checkpoints
> do NOT offer a Resume option. The user simply sees "Request Timed Out, please resubmit." This
> simplifies the implementation - no need to handle resume of expired checkpoints. Auto-resume only
> applies to Scenarios 2a/3 where `apply_default` behavior is configured.

#### Phase 8.1: Scenario 1 Implementation (Streaming + implicit_deny)

This is the default streaming behavior. No environment variables needed.

**Agent Changes (`agent-with-human-approval`):**
- [x] Update SSE handler (`sse_handler.go`) to set `RequestModeStreaming` in context ✅
- [x] Start expiry processor in `hitl_setup.go` using `StartExpiryProcessor()` ✅
- [x] Add expiry callback in `hitl_setup.go` that logs when `action=""` (implicit deny) ✅

**UI Changes (`chat-ui/hitl.html`):**
- [x] Add polling mechanism to check checkpoint status every 5 seconds while approval dialog is shown ✅
- [x] Detect `status="expired"` → show "Request Timed Out" message (NO Resume button) ✅
- [x] Clear polling interval when user approves/rejects or dialog closes ✅

**Flow (Scenario 1):**
```
1. User sends request via SSE
2. Agent sets ctx = WithRequestMode(ctx, RequestModeStreaming)
3. Checkpoint created with RequestMode="streaming"
4. SSE handler returns ErrInterrupted, UI shows approval dialog
5. UI starts polling GET /hitl/checkpoints/{id} every 5-10s
6. [Timeout - e.g., 5 minutes, user doesn't respond]
7. Expiry processor: streaming + implicit_deny → status="expired", action=""
8. Callback logs: "Checkpoint expired (implicit deny)"
9. UI's next poll detects status="expired" → shows "Request Timed Out"
10. User must resubmit request (no Resume option)
```

**Code Changes Summary:**

| File | Change | Lines |
|------|--------|-------|
| `sse_handler.go` | Add `WithRequestMode(streaming)` before `Process()` | ~1 |
| `hitl_setup.go` | Call `StartExpiryProcessor()` | ~8 |
| `hitl_setup.go` | Add `SetExpiryCallback()` for logging | ~15 |
| `hitl.html` | Add polling + timeout detection | ~25 |
| **Total** | | **~49 lines** |

#### Phase 8.2: Scenario 2a/2b/3 Implementation (apply_default) - DEFERRED

For scenarios with auto-resume or auto-reject, additional work is needed:

**Deferred Agent Changes:**
- [ ] Update sync handler (`handlers.go`) to set `RequestModeNonStreaming` in context
- [ ] Extend callback to handle `action="approve"` → auto-resume workflow
- [ ] Extend callback to handle `action="reject"` → store rejection result
- [ ] Pass orchestrator reference to callback for auto-resume capability

**Deferred UI Changes:**
- [ ] Handle `status="expired_approved"` → show "Auto-approved" notification
- [ ] Handle `status="expired_rejected"` → show "Request rejected due to timeout"

**Callback Pattern for All Scenarios (Future):**
```go
// In main.go AFTER orchestrator is created (not in hitl_setup.go)
hitl.CheckpointStore.SetExpiryCallback(func(ctx context.Context, cp *orchestration.ExecutionCheckpoint, action orchestration.CommandType) {
    switch action {
    case "": // Scenario 1: implicit_deny
        logger.Info("Checkpoint expired (implicit deny)", "checkpoint_id", cp.CheckpointID)
        // No action - UI handles via polling

    case orchestration.CommandApprove: // Scenario 2a/3: auto-approve
        logger.Info("Checkpoint auto-approved", "checkpoint_id", cp.CheckpointID)
        // Auto-resume workflow
        resumeCtx, _ := orchestration.BuildResumeContext(ctx, cp)
        go orchestrator.Process(resumeCtx, cp.OriginalRequest, callback)

    case orchestration.CommandReject: // Scenario 2b: auto-reject
        logger.Info("Checkpoint auto-rejected", "checkpoint_id", cp.CheckpointID)
        // Store rejection result for user to see when they poll
        resultStore.Store(cp.RequestID, Result{Status: "rejected", Reason: "timeout"})
    }
})
```

**NOT implementing (deferred to future WebSocket support):**
- Real-time SSE push of `checkpoint_expired` events
- Redis Pub/Sub subscription in SSE handlers
- Keeping SSE connection open during approval wait

### Phase 9: Testing and Documentation
- [x] Write unit tests for mode-aware expiry logic - ✅ `hitl_expiry_processor_test.go`
- [x] Write unit tests for `StreamingExpiryBehavior` configuration - ✅ `TestGetStreamingExpiryBehavior_*`
- [x] Write unit tests for `NonStreamingExpiryBehavior` configuration - ✅ `TestGetNonStreamingExpiryBehavior_*`
- [x] Write unit tests for `DefaultRequestMode` with WARN logging verification - ✅ `TestGetEffectiveRequestMode_LogsWarnWhenUsingDefault`
- [DEFERRED] Write integration tests for streaming expiry with `implicit_deny` (default) → Requires Redis + full system
- [DEFERRED] Write integration tests for streaming expiry with `apply_default` → Requires Redis + full system
- [DEFERRED] Write integration tests for non-streaming expiry with `apply_default` (default) → Requires Redis + full system
- [DEFERRED] Write integration tests for non-streaming expiry with `implicit_deny` → Requires Redis + full system
- [x] Write tests for all environment variable overrides - ✅ `TestConfigHierarchy_*`
- [x] Verify WARN logs appear when RequestMode not set - ✅ `TestGetEffectiveRequestMode_LogsWarnWhenUsingDefault`
- [DEFERRED] Verify trace events appear in Jaeger when using defaults → Requires Jaeger + full system
- [DEFERRED] Verify cross-trace correlation links expiry resume traces to original request traces → Requires Jaeger + full system
- [DEFERRED] Verify `original_request_id` baggage propagates to downstream spans → Requires Jaeger + full system
- [x] Write tests for `BuildResumeContext()` helper
- [x] Write tests for `IsResumableStatus()` helper
- [DEFERRED] Write tests for distributed claim mechanism (multi-instance) → Requires Redis + multi-pod setup
- [x] Write tests for callback panic recovery - ✅ `TestInvokeCallbackSafely_*`
- [x] Write tests for `DeliveryAtMostOnce` semantics - ✅ `TestDeliverySemantics_*`, `TestExpiryProcessorConfig_DeliverySemantics`
- [x] Write tests for `DeliveryAtLeastOnce` semantics - ✅ `TestDeliverySemantics_*`, `TestExpiryProcessorConfig_DeliverySemantics`
- [x] Write tests for graceful shutdown with timeout - ✅ `TestStopExpiryProcessor_*`
- [x] Write tests for `validateExpiryConfig()` fail-fast validation
- [x] Write tests for `validateExpiryConfig()` edge cases - ✅ `TestValidateExpiryConfig_EdgeCases`
- [x] Verify all error messages include actionable remediation hints - ✅ `TestValidateExpiryConfig_ErrorMessages`

#### Design Document Updates (This Document)
- [x] Document architectural decision: Configuration Hierarchy (InterruptDecision → env vars → defaults)
- [x] Document per-checkpoint configuration via InterruptDecision with examples
- [x] Document all environment variables with purpose, defaults, valid values, and examples
- [x] Add Developer Workflow section with step-by-step guide for both scenarios
- [x] Add complete agent code example showing streaming and non-streaming handlers
- [x] Add Developer Checklist summary table

#### External Documentation
- [ ] Update HUMAN_IN_THE_LOOP_USER_GUIDE.md with:
  - [ ] All expiry behavior configuration options
  - [ ] Default behaviors and how to override them
  - [ ] Observable defaults (WARN logging, trace events)
  - [ ] Example configurations for common use cases
  - [ ] Production deployment considerations (multi-pod, graceful shutdown)
  - [ ] `BuildResumeContext()` and `IsResumableStatus()` helper usage
  - [ ] Cross-trace correlation patterns for expiry callbacks
  - [ ] `telemetry.StartLinkedSpan` usage for HITL resume tracing
  - [ ] `telemetry.WithBaggage` usage for `original_request_id` propagation
  - [ ] Framework vs Application responsibility matrix
  - [ ] Delivery semantics options and when to use each
- [ ] Update HUMAN_IN_THE_LOOP_DESIGN.md with mode-aware expiry section
- [ ] Document all new environment variables in ENVIRONMENT_VARIABLES_GUIDE.md

### Phase 10: RuleBasedPolicy DefaultAction Fix

> **Issue Identified (2026-01-23):** The `RuleBasedPolicy` currently uses `p.config.DefaultAction`
> for ALL checkpoint types, which means both plan and step checkpoints use the same default action.
> This is incorrect - step checkpoints should auto-reject on timeout for fail-safe behavior.
> See "How DefaultAction is Determined" section above for the correct design.

**Current Code (Needs Fix) - Exact Locations:**

**1. `ShouldApprovePlan()` - Line 97 (sensitive ops branch):**
```go
// hitl_policy.go:91-104
return &InterruptDecision{
    ShouldInterrupt: true,
    Reason:          ReasonSensitiveOperation,
    Message:         fmt.Sprintf("Plan contains sensitive operations requiring approval: %v", sensitiveDetails),
    Priority:        PriorityHigh,
    Timeout:         p.config.DefaultTimeout,
    DefaultAction:   p.config.DefaultAction,  // ← BUG: Should be CommandApprove
    Metadata: map[string]interface{}{...},
}, nil
```

**2. `ShouldApprovePlan()` - Line 122 (RequirePlanApproval branch):**
```go
// hitl_policy.go:116-129
return &InterruptDecision{
    ShouldInterrupt: true,
    Reason:          ReasonPlanApproval,
    Message:         fmt.Sprintf("Plan approval required for request: %s", truncateString(plan.OriginalRequest, 100)),
    Priority:        PriorityNormal,
    Timeout:         p.config.DefaultTimeout,
    DefaultAction:   p.config.DefaultAction,  // ← BUG: Should be CommandApprove
    Metadata: map[string]interface{}{...},
}, nil
```

**3. `ShouldApproveBeforeStep()` - Line 165 (sensitive agent branch):**
```go
// hitl_policy.go:159-171
return &InterruptDecision{
    ShouldInterrupt: true,
    Reason:          ReasonSensitiveOperation,
    Message:         fmt.Sprintf("Step approval required for agent: %s", step.AgentName),
    Priority:        PriorityHigh,
    Timeout:         p.config.DefaultTimeout,
    DefaultAction:   p.config.DefaultAction,  // ← BUG: Should be CommandReject
    Metadata: map[string]interface{}{...},
}, nil
```

**4. `ShouldApproveBeforeStep()` - Line 197 (sensitive capability branch):**
```go
// hitl_policy.go:191-204
return &InterruptDecision{
    ShouldInterrupt: true,
    Reason:          ReasonSensitiveOperation,
    Message:         fmt.Sprintf("Step approval required for operation: %s.%s", step.AgentName, capability),
    Priority:        PriorityHigh,
    Timeout:         p.config.DefaultTimeout,
    DefaultAction:   p.config.DefaultAction,  // ← BUG: Should be CommandReject
    Metadata: map[string]interface{}{...},
}, nil
```

**5. `ShouldApproveAfterStep()` - Line 222 (output validation branch):**
```go
// hitl_policy.go:216-229
return &InterruptDecision{
    ShouldInterrupt: true,
    Reason:          ReasonOutputValidation,
    Message:         fmt.Sprintf("Output validation required for: %s.%s", step.AgentName, capability),
    Priority:        PriorityNormal,
    Timeout:         p.config.DefaultTimeout,
    DefaultAction:   p.config.DefaultAction,  // ← BUG: Should be CommandApprove
    Metadata: map[string]interface{}{...},
}, nil
```

**6. `ShouldEscalateError()` - Line 250 (already correct):**
```go
// hitl_policy.go:244-257
return &InterruptDecision{
    ShouldInterrupt: true,
    Reason:          ReasonEscalation,
    Message:         fmt.Sprintf("Escalation after %d failed attempts: %s", attempts, err.Error()),
    Priority:        PriorityHigh,
    Timeout:         p.config.DefaultTimeout,
    DefaultAction:   CommandAbort,  // ✓ Already correct
    Metadata: map[string]interface{}{...},
}, nil
```

**Required Changes Summary:**

| # | Method | Line | Current | Change To |
|---|--------|------|---------|-----------|
| 1 | `ShouldApprovePlan()` | 97 | `p.config.DefaultAction` | `CommandApprove` |
| 2 | `ShouldApprovePlan()` | 122 | `p.config.DefaultAction` | `CommandApprove` |
| 3 | `ShouldApproveBeforeStep()` | 165 | `p.config.DefaultAction` | `CommandReject` |
| 4 | `ShouldApproveBeforeStep()` | 197 | `p.config.DefaultAction` | `CommandReject` |
| 5 | `ShouldApproveAfterStep()` | 222 | `p.config.DefaultAction` | `CommandApprove` |
| 6 | `ShouldEscalateError()` | 250 | `CommandAbort` | ✓ No change needed |

**Implementation Tasks:** ✅ **COMPLETED (2026-01-23)**
- [x] Line 97: Change `p.config.DefaultAction` → `CommandApprove`
- [x] Line 122: Change `p.config.DefaultAction` → `CommandApprove`
- [x] Line 165: Change `p.config.DefaultAction` → `CommandReject`
- [x] Line 197: Change `p.config.DefaultAction` → `CommandReject`
- [x] Line 222: Change `p.config.DefaultAction` → `CommandApprove`
- [x] Line 250: Already uses `CommandAbort` - no change needed
- [x] Add unit tests to verify each method returns the correct `DefaultAction`
- [ ] (Optional) Deprecate `HITLConfig.DefaultAction` field or document it as "fallback only"

**Test Cases:**
```go
// hitl_policy_test.go
func TestRuleBasedPolicy_DefaultActionByCheckpointType(t *testing.T) {
    ctx := context.Background()
    policy := NewRuleBasedPolicy(HITLConfig{
        RequirePlanApproval:   true,
        SensitiveCapabilities: []string{"transfer_funds"},
        EscalateAfterRetries:  3,
    })

    plan := &RoutingPlan{PlanID: "plan-123", Steps: []RoutingStep{{StepID: "step-1", AgentName: "agent"}}}
    step := RoutingStep{StepID: "step-1", AgentName: "agent", Metadata: map[string]interface{}{"capability": "transfer_funds"}}
    result := &StepResult{Response: "done"}
    err := errors.New("test error")

    // Plan checkpoints should auto-approve
    decision, _ := policy.ShouldApprovePlan(ctx, plan)
    assert.True(t, decision.ShouldInterrupt)
    assert.Equal(t, CommandApprove, decision.DefaultAction)

    // Step checkpoints should fail-safe (reject)
    decision, _ = policy.ShouldApproveBeforeStep(ctx, step, plan)
    assert.True(t, decision.ShouldInterrupt)
    assert.Equal(t, CommandReject, decision.DefaultAction)

    // After-step validation (no plan param in interface)
    decision, _ = policy.ShouldApproveAfterStep(ctx, step, result)
    // By default, requiresOutputValidation returns false, so no interrupt
    // When interrupt IS triggered, DefaultAction should be CommandApprove

    // Error escalation should abort (needs step, err, attempts)
    decision, _ = policy.ShouldEscalateError(ctx, step, err, 3)
    assert.True(t, decision.ShouldInterrupt)
    assert.Equal(t, CommandAbort, decision.DefaultAction)
}
```

**Files to Modify:**
- `orchestration/hitl_policy.go` - Update 4 methods
- `orchestration/hitl_policy_test.go` - Add test cases

### Expected Behavior Matrix (After Phase 10 Fix)

This matrix shows the expected expiry behavior for all configuration combinations after the fix is applied.

**Configuration Variables:**
| Variable | Controls |
|----------|----------|
| `GOMIND_HITL_REQUIRE_PLAN_APPROVAL` | Whether plan checkpoints are created |
| `GOMIND_HITL_SENSITIVE_CAPABILITIES` | Which capabilities trigger step checkpoints |
| Request Mode | `streaming` (SSE) vs `non_streaming` (HTTP 202) |

**Expiry Behavior Matrix:**

| # | Plan Approval | Sensitive Caps | Request Mode | Checkpoint Type | DefaultAction | Expiry Behavior | Final Status | Can Resume? |
|---|---------------|----------------|--------------|-----------------|---------------|-----------------|--------------|-------------|
| 1 | `true` | any | `streaming` | Plan | `approve` | `implicit_deny` | `expired` | No (resubmit) |
| 2 | `true` | any | `non_streaming` | Plan | `approve` | `apply_default` | `expired_approved` | Yes |
| 3 | `false` | `stock_quote` | `streaming` | Step | `reject` | `implicit_deny` | `expired` | No (resubmit) |
| 4 | `false` | `stock_quote` | `non_streaming` | Step | `reject` | `apply_default` | `expired_rejected` | No (fail-safe) |
| 5 | `true` | `stock_quote` | `streaming` | Plan→Step | varies | `implicit_deny` | `expired` | No (resubmit) |
| 6 | `true` | `stock_quote` | `non_streaming` | Plan→Step | varies | `apply_default` | See below | Depends |

**Row 6 Detail (multi-checkpoint scenario):**
- First checkpoint (Plan): `DefaultAction=approve` → `expired_approved` → Can resume to next checkpoint
- Second checkpoint (Step): `DefaultAction=reject` → `expired_rejected` → Cannot resume (fail-safe)

**Key Security Improvement:**

| Scenario | Current (Bug) | After Fix |
|----------|---------------|-----------|
| Step checkpoint expires (non-streaming) | `expired_approved` - sensitive op runs without approval | `expired_rejected` - blocked, user must explicitly approve |

**Summary of DefaultAction by Checkpoint Type:**

| Checkpoint Type | Policy Method | DefaultAction | Rationale |
|-----------------|---------------|---------------|-----------|
| `plan_generated` | `ShouldApprovePlan()` | `approve` | Plans are routine, auto-proceed is user-friendly |
| `before_step` | `ShouldApproveBeforeStep()` | `reject` | Sensitive ops need explicit approval (fail-safe) |
| `after_step` | `ShouldApproveAfterStep()` | `approve` | Output validation usually proceeds |
| `on_error` | `ShouldEscalateError()` | `abort` | Don't retry errors without human decision |

---

## Summary: Decision Flow Chart

```
                           Checkpoint Expires
                                  │
                                  ▼
                   ┌───────────────────────────┐
                   │   Is RequestMode set?     │
                   └───────────────────────────┘
                          │             │
                    yes   │             │  no
                          │             ▼
                          │   ┌─────────────────────────┐
                          │   │  Use DefaultRequestMode │
                          │   │  Log WARN + trace event │
                          │   └─────────────────────────┘
                          │             │
                          └──────┬──────┘
                                 ▼
                   ┌───────────────────────────┐
                   │   What is RequestMode?    │
                   └───────────────────────────┘
                          │             │
               streaming  │             │  non_streaming
                          ▼             ▼
         ┌────────────────────────────┐  ┌────────────────────────────┐
         │ StreamingExpiryBehavior?   │  │ NonStreamingExpiryBehavior?│
         └────────────────────────────┘  └────────────────────────────┘
              │                   │           │                   │
   implicit_deny          apply_default  apply_default      implicit_deny
   (default)                              (default)
              │                   │           │                   │
              └─────────┬─────────┘           └─────────┬─────────┘
                        │                               │
    ┌───────────────────┴───────────────────────────────┴────────────────┐
    │                                                                    │
    ▼                                                                    ▼
┌─────────────────┐                                        ┌─────────────────────────┐
│  IMPLICIT DENY  │                                        │  APPLY DEFAULT ACTION   │
│                 │                                        │                         │
│  Status →       │                                        │  DefaultAction=approve  │
│  "expired"      │                                        │    → "expired_approved" │
│                 │                                        │                         │
│  No action      │                                        │  DefaultAction=reject   │
│  applied.       │                                        │    → "expired_rejected" │
│                 │                                        │                         │
│  User must      │                                        │  DefaultAction=abort    │
│  resume.        │                                        │    → "expired_aborted"  │
└─────────────────┘                                        └─────────────────────────┘
```

**Key Points:**
- **RequestMode not set** → Uses `DefaultRequestMode`, logs WARN, adds trace event
- **Streaming requests** → Check `StreamingExpiryBehavior` (default: `implicit_deny`)
- **Non-streaming requests** → Check `NonStreamingExpiryBehavior` (default: `apply_default`)
- **All behaviors are configurable** via policy config or environment variables
- **Observable defaults** → WARN logs + trace events ensure visibility when using defaults

---

## Environment Variables Reference

This section consolidates **all** environment variables introduced by the HITL Expiry Processor.
For deployment-specific configuration, set these in your environment (e.g., Kubernetes ConfigMap,
Docker Compose, or shell export).

### Expiry Behavior Variables

These control what happens when checkpoints expire without a human response.

| Variable | Default | Valid Values | Purpose |
|----------|---------|--------------|---------|
| `GOMIND_HITL_DEFAULT_ACTION` | `reject` | `approve`, `reject`, `abort` | **Override** the DefaultAction for all checkpoint types. By default, all checkpoints reject on expiry (see [Reject-by-Default](#design-decision-reject-by-default-on-expiry)). Set to `approve` to restore auto-approve behavior. |
| `GOMIND_HITL_STREAMING_EXPIRY` | `implicit_deny` | `implicit_deny`, `apply_default` | Action when **streaming** (SSE/WebSocket) request's checkpoint expires. `implicit_deny` = mark as expired, no action; `apply_default` = apply the policy's DefaultAction. |
| `GOMIND_HITL_NON_STREAMING_EXPIRY` | `apply_default` | `implicit_deny`, `apply_default` | Action when **non-streaming** (HTTP 202) request's checkpoint expires. `apply_default` = apply the policy's DefaultAction; `implicit_deny` = mark as expired, no action. |
| `GOMIND_HITL_DEFAULT_REQUEST_MODE` | `non_streaming` | `streaming`, `non_streaming` | Request mode to assume when `WithRequestMode()` was not called. A WARN log is emitted when this default is used. |

**Usage Examples:**

```bash
# Production (default): Strict HITL - all checkpoints reject on expiry
# No configuration needed - this is the default behavior
# Streaming: implicit_deny (no action)
# Non-streaming: apply_default → reject (since DEFAULT_ACTION=reject)

# Internal tooling: Auto-approve everything for faster iteration
export GOMIND_HITL_DEFAULT_ACTION=approve
export GOMIND_HITL_STREAMING_EXPIRY=apply_default
export GOMIND_HITL_NON_STREAMING_EXPIRY=apply_default

# Mixed: Auto-approve non-streaming, strict for streaming
export GOMIND_HITL_DEFAULT_ACTION=approve
export GOMIND_HITL_STREAMING_EXPIRY=implicit_deny
export GOMIND_HITL_NON_STREAMING_EXPIRY=apply_default

# Ultra-strict: Require manual resume for everything (no auto-action)
export GOMIND_HITL_STREAMING_EXPIRY=implicit_deny
export GOMIND_HITL_NON_STREAMING_EXPIRY=implicit_deny
```

### Expiry Processor Variables

These control the background expiry processor that scans for and processes expired checkpoints.

| Variable | Default | Valid Values | Purpose |
|----------|---------|--------------|---------|
| `GOMIND_HITL_EXPIRY_ENABLED` | `true` | `true`, `false`, `1`, `0`, `yes`, `no`, `on`, `off` | Enable/disable the expiry processor. Set to `false` to disable automatic expiry processing. |
| `GOMIND_HITL_EXPIRY_INTERVAL` | `10s` | Go duration (e.g., `5s`, `30s`, `1m`) | How often to scan for expired checkpoints. Minimum: 1s. |
| `GOMIND_HITL_EXPIRY_BATCH_SIZE` | `100` | Integer 1-10000 | Maximum checkpoints to process per scan. Larger values process more checkpoints but use more memory. |
| `GOMIND_HITL_EXPIRY_DELIVERY` | `at_most_once` | `at_most_once`, `at_least_once` | Callback delivery semantics. `at_most_once` = update status first, then callback (no retry on failure); `at_least_once` = callback first, then update status (may retry on failure). |

**Usage Examples:**

```bash
# Default configuration (recommended for most applications)
# No need to set anything - defaults are sensible

# High-throughput system: Process more checkpoints per scan, scan less frequently
export GOMIND_HITL_EXPIRY_INTERVAL=30s
export GOMIND_HITL_EXPIRY_BATCH_SIZE=500

# Critical operations: Use at-least-once delivery for guaranteed processing
export GOMIND_HITL_EXPIRY_DELIVERY=at_least_once

# Disable expiry processor (e.g., for testing or manual control)
export GOMIND_HITL_EXPIRY_ENABLED=false
```

### Redis Configuration Variables

These control the Redis connection for checkpoint storage.

| Variable | Default | Purpose |
|----------|---------|---------|
| `REDIS_URL` | `redis://localhost:6379` | Redis connection URL for checkpoint storage. |
| `GOMIND_HITL_REDIS_DB` | `6` | Redis database number for HITL checkpoints (isolates from other data). |
| `GOMIND_HITL_KEY_PREFIX` | `gomind:hitl` | Key prefix for all HITL Redis keys. |
| `GOMIND_AGENT_NAME` | (empty) | If set, appends to key prefix for multi-agent isolation: `{prefix}:{agent_name}`. |

**Usage Examples:**

```bash
# Production with dedicated Redis
export REDIS_URL=redis://redis.production.svc:6379
export GOMIND_HITL_REDIS_DB=6

# Multi-agent deployment with key isolation
export GOMIND_AGENT_NAME=travel-agent
# Keys will be: gomind:hitl:travel-agent:checkpoint:...
```

### Configuration Priority

Per FRAMEWORK_DESIGN_PRINCIPLES.md, configuration follows this precedence (highest to lowest):

1. **Explicitly set configuration options** (e.g., `WithExpiryProcessor(config)`)
2. **Environment variables** (this section)
3. **Sensible defaults** (hardcoded in framework)

**Example: Combining programmatic and environment configuration:**

```go
// Load from environment variables as a base
config := ExpiryProcessorConfigFromEnv()

// Override specific values programmatically
config.BatchSize = 200  // Override env GOMIND_HITL_EXPIRY_BATCH_SIZE

checkpointStore.StartExpiryProcessor(ctx, config)
```

> **Note**: Circuit breaker is **not** configured via environment variables.
> Per ARCHITECTURE.md Section 9, circuit breaker is an optional dependency injected by the application
> using `WithExpiryCircuitBreaker(cb)`.

---

## Related Documents

- [HUMAN_IN_THE_LOOP_PROPOSAL.md](../HUMAN_IN_THE_LOOP_PROPOSAL.md) - Original design (mentions Phase 3 limitation)
- [HUMAN_IN_THE_LOOP_DESIGN.md](./HUMAN_IN_THE_LOOP_DESIGN.md) - Current implementation details
- [HUMAN_IN_THE_LOOP_USER_GUIDE.md](../../docs/HUMAN_IN_THE_LOOP_USER_GUIDE.md) - User-facing documentation
- [DISTRIBUTED_TRACING_GUIDE.md](../../docs/DISTRIBUTED_TRACING_GUIDE.md) - Distributed tracing patterns
- [agent-with-human-approval/handlers.go](../../examples/agent-with-human-approval/handlers.go) - Reference implementation for cross-trace correlation in HITL resume flows
