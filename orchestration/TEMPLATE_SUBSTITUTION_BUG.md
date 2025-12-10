# Template Substitution Bug: Response Wrapper Mismatch

**Status:** ✅ FIXED
**Severity:** High
**Discovered:** December 2025
**Fixed:** December 2025
**Module:** orchestration/executor.go
**Related:** STEP_REFERENCE_TEMPLATE_BUG.md (discovered during verification)

## Resolution

Fixed by wrapping step results in a `response` key in `buildStepContext`:

```go
// Before (buggy):
deps[depID] = parsed

// After (fixed):
deps[depID] = map[string]interface{}{"response": parsed}
```

### Files Modified

| File | Change | Lines |
|------|--------|-------|
| `executor.go` | Wrap parsed response in "response" key | 444-446 |
| `executor.go` | Update comments to reflect new structure | 424-426 |
| `executor_test.go` | Add 7 new tests for response wrapper | 1508-1840 |

### Verification

Tested with multi-step orchestration (geocoding → weather):
- weather-tool logs show: `{"lat":35.6768601,"lon":139.7638947,...}`
- Template `{{step-1.response.latitude}}` correctly resolved to `35.6768601`

## Summary

The executor's template substitution fails because of a contract mismatch between:
1. The prompt instructions (which tell LLM to use `{{stepId.response.field}}`)
2. The executor's `buildStepContext` (which stores results WITHOUT a `response` wrapper)

## Symptoms

Templates are sent as literal strings to tools instead of being resolved:

```json
// Step-3 sends this to country-info-tool:
{"country": "{{step-2.response.data.country}}"}

// Tool returns error:
{"error": "Country '{{step-2.response.data.country}}' not found"}
```

## Root Cause Analysis

### Template Format (from prompt)

The prompt tells the LLM to use this format:
```
{{<step_id>.response.<field_path>}}

Example: {{step-1.response.data.id}}
```

### Executor Storage (buildStepContext)

```go
// executor.go lines 422-449
func (e *SmartExecutor) buildStepContext(...) context.Context {
    deps := make(map[string]map[string]interface{})
    for _, depID := range step.DependsOn {
        if result, ok := results[depID]; ok && result.Response != "" {
            var parsed map[string]interface{}
            json.Unmarshal([]byte(result.Response), &parsed)
            deps[depID] = parsed  // <-- Stores WITHOUT "response" wrapper
        }
    }
    return context.WithValue(ctx, dependencyResultsKey, deps)
}
```

### The Mismatch

If step-2 returns `{"data": {"country": "France"}}`:

| Expected by Prompt | Actual in Executor |
|--------------------|-------------------|
| `deps["step-2"]["response"]["data"]["country"]` | `deps["step-2"]["data"]["country"]` |

### extractFieldValue Trace

```go
// Template: {{step-2.response.data.country}}
// stepID = "step-2"
// fieldPath = "response.data.country"

stepData = deps["step-2"]  // {"data": {"country": "France"}}
extractFieldValue(stepData, "response.data.country")
  // splits fieldPath: ["response", "data", "country"]
  // looks for stepData["response"] --> NOT FOUND
  // returns nil

// Since value is nil, template is left unchanged
```

## Proposed Fix

### Option A: Wrap Results in `response` Key (Recommended)

Update `buildStepContext` to wrap the parsed response:

```go
// executor.go, line 443
// BEFORE:
deps[depID] = parsed

// AFTER:
deps[depID] = map[string]interface{}{"response": parsed}
```

**Why this is better:**
1. Maintains consistency with the documented template syntax
2. Makes it explicit that we're accessing the step's "response"
3. No changes needed to the prompt
4. Clear semantic meaning: `{{stepId.response.field}}`

### Option B: Change Prompt to Not Include `response.`

Update prompt instructions to use `{{stepId.field}}` instead of `{{stepId.response.field}}`.

**Why NOT recommended:**
1. Breaking change for any existing templates
2. Less explicit - `{{step-1.data.id}}` could be confused with step metadata
3. Would require changes to the just-fixed prompt

## Implementation Plan

### Phase 1: Fix Executor (Immediate)

1. Update `buildStepContext` to wrap parsed results
2. Add/update unit tests
3. Verify template resolution works

### Phase 2: Add Observability

1. Log successful template resolutions (DEBUG level)
2. Add metrics for template resolution success/failure
3. Include resolved values in span attributes for debugging

## Testing Plan

### Unit Tests

```go
func TestTemplateSubstitution_WithResponseWrapper(t *testing.T) {
    executor := NewSmartExecutor(nil)

    // Simulate step-1 returning {"data": {"id": 123}}
    depResults := map[string]map[string]interface{}{
        "step-1": {
            "response": map[string]interface{}{
                "data": map[string]interface{}{
                    "id": 123,
                },
            },
        },
    }

    // Template should resolve correctly
    result := executor.substituteTemplates("{{step-1.response.data.id}}", depResults)
    assert.Equal(t, 123, result)
}

func TestBuildStepContext_WrapsResponseCorrectly(t *testing.T) {
    executor := NewSmartExecutor(nil)

    // Simulate completed step with JSON response
    results := map[string]*StepResult{
        "step-1": {
            Response: `{"data": {"country": "France"}}`,
        },
    }

    step := RoutingStep{
        StepID:    "step-2",
        DependsOn: []string{"step-1"},
    }

    ctx := executor.buildStepContext(context.Background(), step, results)
    deps := ctx.Value(dependencyResultsKey).(map[string]map[string]interface{})

    // Verify response is wrapped
    assert.NotNil(t, deps["step-1"]["response"])

    // Verify nested access works
    response := deps["step-1"]["response"].(map[string]interface{})
    data := response["data"].(map[string]interface{})
    assert.Equal(t, "France", data["country"])
}
```

### Integration Test

1. Send request that triggers multi-step orchestration
2. Verify step-3's parameters contain resolved values (not template strings)
3. Verify no "not found" errors caused by literal template strings

## Code Locations

| File | Line | Issue |
|------|------|-------|
| `executor.go` | 443 | `deps[depID] = parsed` missing `response` wrapper |
| `executor.go` | 487-590 | `substituteTemplates` expects `response.` in path |
| `default_prompt_builder.go` | 15-50 | Prompt instructs `{{stepId.response.field}}` format |

## Impact

### Affected
- All multi-step orchestrations with cross-step references
- Any template using `{{stepId.response.field}}` syntax

### Not Affected
- Single-step orchestrations
- Steps with no dependencies
- Hardcoded parameters

## References

- [STEP_REFERENCE_TEMPLATE_BUG.md](./STEP_REFERENCE_TEMPLATE_BUG.md) - Fixed prompt syntax issue
- [executor.go](./executor.go) - Template substitution logic
- [default_prompt_builder.go](./default_prompt_builder.go) - Prompt with template instructions
