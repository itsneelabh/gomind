# Semantic Retry Enhancement: Independent Steps Support

## Problem Statement

Layer 4 (Semantic Retry / Contextual Re-Resolution) currently requires step dependencies to function. When a step has no `DependsOn` entries, the semantic retry logic is skipped entirely, even when the error might be fixable with LLM assistance.

### Observed Behavior

**Trace analyzed:** `bd0fbd7cae8c0abbd7ed1f00141684f5`
**Request ID:** `orch-1767829800132101429`

1. `get_country_info` step failed with HTTP 404
2. Layer 3 (Error Analyzer) was invoked and returned:
   ```json
   {
     "reason": "The 404 error indicates that the requested resource for 'France' does not exist",
     "should_retry": false,
     "suggested_changes_count": 0
   }
   ```
3. Layer 4 was NOT invoked despite 404 being in trigger status codes

### Root Cause

In `executor.go` (lines 1652-1656), the `len(sourceData) > 0` condition fails for steps without dependencies, causing Layer 4 to be skipped entirely.

---

## Design Limitation

Layer 4 was designed with the assumption that meaningful corrections require context from dependent steps. However, this breaks for:

1. **First steps in a plan** - These often have no dependencies
2. **Independent parallel steps** - Steps that can run concurrently
3. **Simple lookups** - Steps like `get_country_info` that take user input directly

---

## Code Changes

### executor.go (lines ~1654-1656)

**Before:**
```go
sourceData := e.collectSourceDataFromDependencies(ctx, step.DependsOn)

if len(sourceData) > 0 {
    // Layer 4 logic...
}
```

**After:**
```go
sourceData := e.collectSourceDataFromDependencies(ctx, step.DependsOn)

// Check if semantic retry for independent steps is enabled
if len(sourceData) == 0 && !e.config.SemanticRetry.EnableForIndependentSteps {
    // Skip Layer 4 for independent steps when disabled
} else {
    isIndependentStep := len(sourceData) == 0
    // Layer 4 logic (unchanged, but now runs for independent steps too)
}
```

### executor.go - Telemetry additions

**Add to span event:**
```go
attribute.Bool("independent_step", isIndependentStep),
```

**Add new metric (after successful retry):**
```go
if isIndependentStep {
    telemetry.Counter("orchestration.semantic_retry.independent_step",
        "capability", capability,
        "module", telemetry.ModuleOrchestration,
    )
}
```

**Add to log entries:**
```go
"independent_step": isIndependentStep,
```

### interfaces.go - SemanticRetryConfig

**Add new field:**
```go
type SemanticRetryConfig struct {
    Enabled            bool  `json:"enabled"`
    MaxAttempts        int   `json:"max_attempts"`
    TriggerStatusCodes []int `json:"trigger_status_codes,omitempty"`

    // NEW: Enable semantic retry for steps without dependencies
    // Default: true | Env: GOMIND_SEMANTIC_RETRY_INDEPENDENT_STEPS
    EnableForIndependentSteps bool `json:"enable_for_independent_steps"`
}
```

### config.go - DefaultConfig

**Add initialization:**
```go
config.SemanticRetry = SemanticRetryConfig{
    Enabled:                   true,
    MaxAttempts:               2,
    TriggerStatusCodes:        []int{400, 422},
    EnableForIndependentSteps: getEnvBool("GOMIND_SEMANTIC_RETRY_INDEPENDENT_STEPS", true), // NEW
}
```

---

## Environment Variable

### GOMIND_SEMANTIC_RETRY_INDEPENDENT_STEPS

Controls whether Layer 4 (Semantic Retry) runs for steps that have no dependencies.

| Property | Value |
|----------|-------|
| **Variable Name** | `GOMIND_SEMANTIC_RETRY_INDEPENDENT_STEPS` |
| **Type** | Boolean (`true` / `false`) |
| **Default** | `true` (enabled) |
| **Requires Restart** | Yes |

### Usage

```bash
# Enable semantic retry for independent steps (default behavior)
export GOMIND_SEMANTIC_RETRY_INDEPENDENT_STEPS=true

# Disable - revert to old behavior (skip Layer 4 for steps without dependencies)
export GOMIND_SEMANTIC_RETRY_INDEPENDENT_STEPS=false
```

### Kubernetes Deployment Example

```yaml
env:
  - name: GOMIND_SEMANTIC_RETRY_INDEPENDENT_STEPS
    value: "true"
```

### When to Disable

Consider disabling this feature if:
- You want to minimize LLM API calls and costs
- Your independent steps rarely benefit from parameter corrections
- You're experiencing unexpected retry behavior on first steps

---

## Why This Works

The LLM prompt in `contextual_re_resolver.go` receives sufficient context even without source data:

```
TASK: Re-resolve parameters after execution failure

USER REQUEST:
"Get information about France"

SOURCE DATA FROM PREVIOUS STEPS:
{}

FAILED ATTEMPT:
- Capability: get_country_info
- Parameters sent: {"country": "France"}
- Error received: "404 - resource not found"
- HTTP Status: 404

TARGET CAPABILITY SCHEMA:
{
  "name": "get_country_info",
  "parameters": {
    "country": {"type": "string", "description": "Country name or ISO code"}
  }
}
```

This is sufficient for the LLM to suggest corrections like:
- `{"country": "france"}` (lowercase)
- `{"country": "FR"}` (ISO code)

---

## Expected Behavior After Fix

For the `get_country_info` 404 example:

1. Layer 3 returns `should_retry: false` (unchanged)
2. Layer 4 activates with empty source data (NEW behavior)
3. ContextualReResolver analyzes the error with available context
4. LLM suggests parameter correction
5. Step retries with corrected parameters

---

## Telemetry

### New Metric

| Metric Name | Description |
|-------------|-------------|
| `orchestration.semantic_retry.independent_step` | Counter for semantic retries triggered on steps without dependencies |

### Span Attributes

The `semantic_retry_applied` span event now includes:
- `independent_step` (bool) - Whether the step had no dependencies

---

## Impact Analysis

| Aspect | Impact |
|--------|--------|
| Existing steps with dependencies | **No change** - continues to work as before |
| Independent steps (previously skipped) | **New behavior** - Layer 4 now activates |
| LLM costs | **Minor increase** - additional LLM calls for independent steps |
| Performance | **Negligible** - gated by `maxSemanticRetries` |
| Backward compatibility | **Full** - opt-out via env var or config |

---

## Implementation Checklist

- [ ] Modify `executor.go:1656` - Replace gate check with config-aware logic
- [ ] Add `EnableForIndependentSteps` field to `SemanticRetryConfig` in `interfaces.go`
- [ ] Update `DefaultConfig()` to read `GOMIND_SEMANTIC_RETRY_INDEPENDENT_STEPS` env var
- [ ] Add `getEnvBool` helper if not already present in config.go
- [ ] Add telemetry: `orchestration.semantic_retry.independent_step`
- [ ] Add `independent_step` attribute to span events
- [ ] Add unit test: `TestSemanticRetryForIndependentStep`
- [ ] Add unit test: `TestSemanticRetryDisabledForIndependentSteps`
- [ ] Update `docs/INTELLIGENT_ERROR_HANDLING.md` with new behavior and env var

---

## Test Cases

```go
func TestSemanticRetryForIndependentStep(t *testing.T) {
    // Default config should enable semantic retry for independent steps
    config := DefaultConfig()
    assert.True(t, config.SemanticRetry.EnableForIndependentSteps)

    // Step with no dependencies should trigger Layer 4
    step := RoutingStep{
        StepID:      "step_1",
        AgentName:   "country-info",
        DependsOn:   []string{}, // No dependencies
        Instruction: "Get information about France",
    }

    // Simulate HTTP 404 error
    // Verify Layer 4 is invoked
    // Verify LLM receives UserQuery, ErrorResponse, AttemptedParams
    // Verify SourceData is empty {}
    // Verify telemetry metric "orchestration.semantic_retry.independent_step" is emitted
}

func TestSemanticRetryDisabledForIndependentSteps(t *testing.T) {
    config := DefaultConfig()
    config.SemanticRetry.EnableForIndependentSteps = false

    // Step with no dependencies should NOT trigger Layer 4
    step := RoutingStep{
        StepID:      "step_1",
        AgentName:   "country-info",
        DependsOn:   []string{}, // No dependencies
        Instruction: "Get information about France",
    }

    // Simulate HTTP 404 error
    // Verify Layer 4 is NOT invoked
    // Verify old behavior is preserved
}

func TestSemanticRetryEnvVarOverride(t *testing.T) {
    // Test that env var properly overrides default
    t.Setenv("GOMIND_SEMANTIC_RETRY_INDEPENDENT_STEPS", "false")

    config := DefaultConfig()
    assert.False(t, config.SemanticRetry.EnableForIndependentSteps)
}
```

---

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Increased LLM calls for independent steps | Already gated by `maxSemanticRetries` limit |
| Less context for corrections | LLM still has error message + attempted params + capability schema |
| Unnecessary retries for unrecoverable errors | Layer 3 already filters obvious failures |
| Breaking change concerns | Env var allows opt-out without code changes |

---

## References

- [contextual_re_resolver.go](contextual_re_resolver.go) - Layer 4 implementation
- [executor.go:1647-1714](executor.go#L1647-L1714) - Layer 4 integration point
- [interfaces.go](interfaces.go) - SemanticRetryConfig definition
- [INTELLIGENT_ERROR_HANDLING.md](../docs/INTELLIGENT_ERROR_HANDLING.md) - Layer documentation
- Jaeger trace: `bd0fbd7cae8c0abbd7ed1f00141684f5`
- Request ID: `orch-1767829800132101429`
