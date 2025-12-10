# Internal Design: Multi-Layer Type Safety for AI Orchestrator

> **Note:** This is an internal design document for GoMind maintainers. For user-facing documentation, see:
> - [Orchestration README - Multi-Layer Type Safety](./README.md#multi-layer-type-safety)
> - [Intelligent Error Handling Guide](../docs/INTELLIGENT_ERROR_HANDLING.md#orchestration-module-multi-layer-type-safety)
> - [API Reference - ExecutionOptions](../docs/API_REFERENCE.md#executionoptions-configuration)

## Problem Statement

When using the AI orchestrator's natural language mode, LLM-generated execution plans contain parameters with incorrect types. The LLM often outputs numeric values as strings (e.g., `"lat": "35.6897"` instead of `"lat": 35.6897`), causing tool endpoints to fail with type mismatch errors.

### Observed Errors

```
# Weather Tool
json: cannot unmarshal string into Go struct field WeatherRequest.lat of type float64

# Currency Tool
currency not found (due to "Euro" instead of "EUR")
```

### Impact

- **Workflow mode**: Works perfectly (parameters are typed via workflow variables)
- **Natural language mode**: ~30% of tool calls fail due to type mismatches

---

## Root Cause Analysis

### The Flow Where Types Get Lost

```
User Request: "What's the weather in Paris?"
         ↓
[AI Orchestrator - generateExecutionPlan]
         ↓
[LLM generates JSON plan]
    {
      "steps": [{
        "metadata": {
          "capability": "get_weather",
          "parameters": {
            "lat": "48.8566",    ← STRING (should be number)
            "lon": "2.3522"      ← STRING (should be number)
          }
        }
      }]
    }
         ↓
[json.Unmarshal into RoutingStep]
    Metadata: map[string]interface{}{
        "parameters": map[string]interface{}{
            "lat": "48.8566",   ← Stays as string
            "lon": "2.3522",    ← Stays as string
        }
    }
         ↓
[SmartExecutor.executeStep]
         ↓
[callAgent - json.Marshal(parameters)]
    Sends: {"lat":"48.8566","lon":"2.3522"}
         ↓
[Tool endpoint receives wrong types]
    Expects: {"lat":48.8566,"lon":2.3522}
    Got:     {"lat":"48.8566","lon":"2.3522"}
         ↓
[ERROR: cannot unmarshal string into float64]
```

### Key Files Involved

| File | Lines | Role |
|------|-------|------|
| `interfaces.go` | 20-28 | `RoutingStep.Metadata` is `map[string]interface{}` (untyped) |
| `orchestrator.go` | 498-545 | LLM prompt generation (includes type rules) |
| `orchestrator.go` | 547-571 | Plan parsing with `json.Unmarshal` |
| `executor.go` | 647-673 | Parameter extraction from metadata |
| `executor.go` | 414-438 | Parameter interpolation |
| `executor.go` | 847-921 | HTTP call to tool with `json.Marshal` |
| `catalog.go` | 52-59 | `Parameter` struct with `Type` field (schema exists!) |

### Why Workflow Mode Works

Workflow mode uses typed `InputDef` with explicit type declarations:

```go
// workflow_engine.go
type InputDef struct {
    Type     string      `yaml:"type" json:"type"`  // Explicit type!
    Required bool        `yaml:"required"`
    Default  interface{} `yaml:"default,omitempty"`
}
```

Parameters flow through typed workflow variables, not LLM-generated JSON.

---

## Industry Research

### Key Findings

| Finding | Source | Implication |
|---------|--------|-------------|
| LLMs make ~30% type mistakes on function calling | Agentica Framework | Type errors are expected, not edge cases |
| Validation feedback increases success 70% → 95% | Agentica Framework | Retry with error context helps |
| Schema-based validation is essential | OpenAI Structured Outputs | Post-process against known schema |
| Go `json.Unmarshal` preserves string types | Go issue #22463 | Go won't auto-coerce strings to numbers |
| Pydantic validators catch type discrepancies | Multiple sources | Schema validation before execution |
| OpenAI Structured Outputs guarantees 100% schema compliance | OpenAI (Aug 2024) | When using `strict: true` mode |

### How Other Frameworks Handle This

**Python (Pydantic/LangChain)**:
- Use Pydantic models with type annotations
- Automatic coercion: `"35.6"` → `35.6` for `float` fields
- Validation errors trigger retry with feedback

**Agentica Framework**:
- Uses typia runtime validator
- Feeds validation errors back to LLM
- 70% → 95% → ~100% success over 3 retries

**OpenAI Structured Outputs (Aug 2024)**:
- `strict: true` mode guarantees schema compliance
- Only works with OpenAI API directly

### Sources

- [Enforcing JSON Outputs in Commercial LLMs](https://datachain.ai/blog/enforcing-json-outputs-in-commercial-llms)
- [Agentica Function Calling Concepts](https://wrtnlabs.io/agentica/docs/concepts/function-calling/)
- [Agentica LLM Vendors - Validation Feedback](https://wrtnlabs.io/agentica/docs/core/vendor/)
- [LangChain Tool Calling](https://blog.langchain.com/tool-calling-with-langchain/)
- [Go JSON Number Handling Issue](https://github.com/golang/go/issues/22463)
- [OpenAI Agents SDK Function Schema](https://openai.github.io/openai-agents-python/ref/function_schema/)
- [Medium: Enhancing JSON Output with LLMs](https://medium.com/@dinber19/enhancing-json-output-with-large-language-models-a-comprehensive-guide-f1935aa724fb)

---

## Recommended Solution: Multi-Layer Defense

```
┌─────────────────────────────────────────────────────────────────┐
│                    MULTI-LAYER TYPE SAFETY                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Layer 1: PROMPT IMPROVEMENT (Preventive)                       │
│  ├── Include concrete JSON examples with correct types           │
│  ├── Use few-shot prompting showing number vs "number"          │
│  └── Effectiveness: ~70-80% (LLMs don't always follow)          │
│                                                                  │
│  Layer 2: SCHEMA COERCION (Corrective) ⭐ PRIMARY FIX           │
│  ├── Location: executor.go, BEFORE calling tool                 │
│  ├── Use capability.Parameters[].Type to know expected type     │
│  ├── Coerce: "35.6" → 35.6, "true" → true, "123" → 123         │
│  └── Effectiveness: ~95%+ (deterministic)                       │
│                                                                  │
│  Layer 3: VALIDATION FEEDBACK (Recovery) - Future Enhancement   │
│  ├── If tool returns 400 with type error                        │
│  ├── Feed error back to LLM for plan correction                 │
│  ├── Retry with corrected parameters                            │
│  └── Effectiveness: ~99%+ (requires additional LLM call)        │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Implementation Plan

This section provides detailed implementation plans for all three layers of defense.

---

## Layer 1: Prompt Improvement (Preventive)

### Overview

Improve the LLM prompt to reduce type errors at the source. While not 100% effective, this reduces the burden on downstream layers.

**Effectiveness**: ~70-80% (LLMs don't always follow instructions)
**Location**: `orchestrator.go`, `buildPlanningPrompt()` method (lines 436-545)

### Two Implementation Options

**Option A: Modify Default Prompt** (simpler)
- Edit the hardcoded default prompt at lines 498-544
- Quick to implement but requires code changes

**Option B: Use PromptBuilder Interface** (more flexible)
- Leverage existing `PromptBuilder` interface (lines 474-496)
- Create a custom `DefaultPromptBuilder` with enhanced type rules
- Set via `orchestrator.SetPromptBuilder(builder)`
- No changes to core orchestrator code

The examples below show Option A. For Option B, implement a custom `PromptBuilder` and call `SetPromptBuilder()`.

### Current Prompt Issues

The current prompt (lines 498-545) includes type rules but lacks concrete examples:

```go
// Current prompt snippet
CRITICAL - Parameter Type Rules:
- Parameters with type "number" or "float64" MUST be JSON numbers (e.g., 35.6897), NOT strings
...
```

### Proposed Enhancement

Add few-shot examples showing correct vs incorrect JSON:

```go
// orchestrator.go - Enhanced prompt in buildPlanningPrompt()

const enhancedTypeRulesPrompt = `
CRITICAL - JSON Type Rules (MUST FOLLOW):

CORRECT examples:
{
  "parameters": {
    "lat": 35.6897,        // ✓ Number without quotes
    "lon": 139.6917,       // ✓ Number without quotes
    "count": 10,           // ✓ Integer without quotes
    "enabled": true,       // ✓ Boolean without quotes
    "city": "Tokyo"        // ✓ String with quotes
  }
}

INCORRECT examples (DO NOT DO THIS):
{
  "parameters": {
    "lat": "35.6897",      // ✗ WRONG - number in quotes becomes string
    "lon": "139.6917",     // ✗ WRONG - number in quotes becomes string
    "count": "10",         // ✗ WRONG - integer in quotes becomes string
    "enabled": "true",     // ✗ WRONG - boolean in quotes becomes string
  }
}

Rules:
1. Numbers (float64, number, integer): NO quotes → 35.6897, 10
2. Booleans (bool, boolean): NO quotes → true, false
3. Strings: WITH quotes → "Tokyo", "USD"
4. Check the parameter type in the capability schema and match it exactly
`
```

### Implementation Code

```go
// orchestrator.go - Add to buildPlanningPrompt() around line 498

func (o *AIOrchestrator) buildPlanningPrompt(ctx context.Context, request string) (string, error) {
    capabilityInfo, err := o.capabilityProvider.FormatCapabilities(ctx, request)
    if err != nil {
        return "", fmt.Errorf("failed to get capabilities: %w", err)
    }

    // Enhanced prompt with concrete type examples
    typeExamples := `
## JSON Type Rules - CRITICAL

When specifying parameters, use the CORRECT JSON types:

✓ CORRECT:
  "lat": 35.6897      (number - no quotes)
  "count": 10         (integer - no quotes)
  "enabled": true     (boolean - no quotes)
  "city": "Tokyo"     (string - with quotes)

✗ WRONG:
  "lat": "35.6897"    (string - will cause type error)
  "count": "10"       (string - will cause type error)
  "enabled": "true"   (string - will cause type error)

Match the parameter type from the capability schema exactly.
`

    return fmt.Sprintf(`You are an AI orchestrator managing a multi-agent system.

%s

%s

User Request: %s

Create an execution plan in JSON format...`, capabilityInfo, typeExamples, request), nil
}
```

### Testing Layer 1

```bash
# Before enhancement - observe type errors in generated plans
# After enhancement - fewer type errors (but not zero)

# Monitor with logging:
grep "type_coercion" /var/log/orchestrator.log | wc -l
# Should decrease after prompt improvement
```

---

## Layer 2: Schema-Based Type Coercion (Corrective) - PRIMARY FIX

**Location**: `executor.go`, after parameter interpolation, before `callAgent()`

**Current Code** (lines 647-708 in `executeStep` method):
```go
// Lines 647-656: Extract capability and parameters from metadata
capability := ""
var parameters map[string]interface{}

if cap, ok := step.Metadata["capability"].(string); ok {
    capability = cap
}
if params, ok := step.Metadata["parameters"].(map[string]interface{}); ok {
    parameters = params
}

// Lines 658-673: Interpolate template parameters with dependency results
if depResults, ok := ctx.Value(dependencyResultsKey).(map[string]map[string]interface{}); ok && len(depResults) > 0 {
    interpolated := e.interpolateParameters(parameters, depResults)
    if interpolated != nil {
        parameters = interpolated
    }
}

// Lines 675-708: Find the capability endpoint and build URL
endpoint := e.findCapabilityEndpoint(agentInfo, capability)
url := fmt.Sprintf("http://%s:%d%s", ...)

// Lines 710-788: Execute with retry logic
response, err := e.callAgent(ctx, url, parameters)
```

**Proposed Change**:
```go
// Extract capability and parameters from metadata
capability := ""
var parameters map[string]interface{}

if cap, ok := step.Metadata["capability"].(string); ok {
    capability = cap
}
if params, ok := step.Metadata["parameters"].(map[string]interface{}); ok {
    parameters = params
}

// Interpolate template parameters...
parameters = interpolated

// NEW: Coerce parameters to match capability schema
capabilitySchema := e.findCapabilitySchema(agentInfo, capability)
if capabilitySchema != nil && len(capabilitySchema.Parameters) > 0 {
    coerced, coercionLog := coerceParameterTypes(parameters, capabilitySchema.Parameters)
    if len(coercionLog) > 0 && e.logger != nil {
        e.logger.Debug("Parameter types coerced to match schema", map[string]interface{}{
            "operation":   "type_coercion",
            "step_id":     step.StepID,
            "capability":  capability,
            "coercions":   coercionLog,
        })
    }
    parameters = coerced
}

// Find the capability endpoint
endpoint := e.findCapabilityEndpoint(agentInfo, capability)

// Build URL and call agent
url := fmt.Sprintf("http://%s:%d%s", ...)
response, err := e.callAgent(ctx, url, parameters)
```

### New Functions to Add

```go
// findCapabilitySchema returns the capability schema for type coercion
func (e *SmartExecutor) findCapabilitySchema(agentInfo *AgentInfo, capabilityName string) *EnhancedCapability {
    if agentInfo == nil {
        return nil
    }
    for i := range agentInfo.Capabilities {
        if agentInfo.Capabilities[i].Name == capabilityName {
            return &agentInfo.Capabilities[i]
        }
    }
    return nil
}

// coerceParameterTypes converts string values to their expected types based on schema.
// Returns the coerced parameters and a log of coercions performed.
func coerceParameterTypes(params map[string]interface{}, schema []Parameter) (map[string]interface{}, []string) {
    if params == nil || len(schema) == 0 {
        return params, nil
    }

    // Build schema lookup: parameter name -> expected type
    schemaMap := make(map[string]string)
    for _, p := range schema {
        schemaMap[p.Name] = strings.ToLower(p.Type)
    }

    result := make(map[string]interface{})
    var coercionLog []string

    for key, value := range params {
        expectedType, hasSchema := schemaMap[key]
        if !hasSchema {
            result[key] = value
            continue
        }

        coerced, wasCoerced := coerceValue(value, expectedType)
        result[key] = coerced

        if wasCoerced {
            coercionLog = append(coercionLog, fmt.Sprintf("%s: %T(%v) -> %T(%v)",
                key, value, value, coerced, coerced))
        }
    }

    return result, coercionLog
}

// coerceValue attempts to convert a value to the expected type.
// Returns the coerced value and whether coercion was performed.
func coerceValue(value interface{}, expectedType string) (interface{}, bool) {
    // If value is already a non-string, return as-is
    strVal, isString := value.(string)
    if !isString {
        return value, false
    }

    // Attempt coercion based on expected type
    switch expectedType {
    case "number", "float64", "float", "double":
        if f, err := strconv.ParseFloat(strVal, 64); err == nil {
            return f, true
        }

    case "integer", "int", "int64", "int32":
        if i, err := strconv.ParseInt(strVal, 10, 64); err == nil {
            return i, true
        }

    case "boolean", "bool":
        if b, err := strconv.ParseBool(strVal); err == nil {
            return b, true
        }

    case "string":
        // Already a string, no coercion needed
        return value, false
    }

    // Coercion failed or not applicable, return original
    return value, false
}
```

### Required Import

```go
import (
    "strconv"
    "strings"
)
```

### Important Note for Layer 2 Only Implementation

If implementing **only Layer 2** (without Layer 3), the existing `callAgent` function is sufficient. The coercion should be added **before** the existing retry loop at lines 710-788. No changes to the retry loop itself are needed.

```go
// Layer 2 only: Add coercion, then use existing callAgent
parameters = coerced  // After coercion
// ... existing retry loop with callAgent remains unchanged ...
response, err := e.callAgent(ctx, url, parameters)
```

Only when implementing Layer 3 do you need `callAgentWithBody` and the modified retry logic.

---

## Layer 3: Validation Feedback Retry (Recovery)

### Overview

When a tool call fails with a type-related error, feed the error back to the LLM and request a corrected plan. This catches edge cases that slip through Layers 1 and 2.

**Effectiveness**: ~99%+ (with up to 3 retries)
**Location**: `executor.go` and `orchestrator.go`
**Trade-off**: Additional LLM calls increase latency and cost

### Architecture

```
Step Execution Flow with Validation Feedback:

[Execute Step]
      ↓
[Call Tool] ──────────────────────────────────────┐
      ↓                                            │
[Success?] ─── Yes ──→ [Return Result]            │
      │                                            │
      No (400 error)                               │
      ↓                                            │
[Is Type Error?] ─── No ──→ [Return Error]        │
      │                                            │
      Yes                                          │
      ↓                                            │
[Retry Count < 3?] ─── No ──→ [Return Error]      │
      │                                            │
      Yes                                          │
      ↓                                            │
[Request LLM Correction] ◄─────────────────────────┘
      ↓                     (feed error context)
[LLM Returns Corrected Parameters]
      ↓
[Retry Call Tool] ─────────────────────────────────┘
```

### Implementation Code

#### Step 1: Add Error Classification

```go
// executor.go - Add type error detection

// isTypeRelatedError checks if an error is due to JSON type mismatch
func isTypeRelatedError(err error, responseBody string) bool {
    typeErrorPatterns := []string{
        "cannot unmarshal string into",
        "cannot unmarshal number into",
        "cannot unmarshal bool into",
        "json: cannot unmarshal",
        "type mismatch",
        "invalid type",
        "expected number",
        "expected string",
        "expected boolean",
    }

    errStr := strings.ToLower(err.Error() + " " + responseBody)
    for _, pattern := range typeErrorPatterns {
        if strings.Contains(errStr, strings.ToLower(pattern)) {
            return true
        }
    }
    return false
}
```

#### Step 2: Add Correction Request to AI Client

```go
// orchestrator.go - Add method for parameter correction

// requestParameterCorrection asks the LLM to fix parameters based on error feedback
func (o *AIOrchestrator) requestParameterCorrection(
    ctx context.Context,
    step RoutingStep,
    originalParams map[string]interface{},
    errorMessage string,
    capabilitySchema *EnhancedCapability,
) (map[string]interface{}, error) {

    // Build correction prompt
    schemaJSON, _ := json.MarshalIndent(capabilitySchema.Parameters, "", "  ")
    paramsJSON, _ := json.MarshalIndent(originalParams, "", "  ")

    correctionPrompt := fmt.Sprintf(`The following tool call failed with a type error.

Tool: %s
Capability: %s
Error: %s

Original Parameters (INCORRECT):
%s

Expected Parameter Schema:
%s

Please provide CORRECTED parameters as a JSON object.
Remember:
- Numbers must NOT be in quotes: "lat": 35.6897 (correct) vs "lat": "35.6897" (wrong)
- Booleans must NOT be in quotes: "enabled": true (correct) vs "enabled": "true" (wrong)
- Only strings should be in quotes

Respond with ONLY the corrected JSON parameters object, no explanation.`,
        step.AgentName,
        step.Metadata["capability"],
        errorMessage,
        string(paramsJSON),
        string(schemaJSON),
    )

    // Call LLM for correction
    response, err := o.aiClient.GenerateResponse(ctx, correctionPrompt, nil)
    if err != nil {
        return nil, fmt.Errorf("LLM correction request failed: %w", err)
    }

    // Parse corrected parameters
    var correctedParams map[string]interface{}
    if err := json.Unmarshal([]byte(response.Content), &correctedParams); err != nil {
        return nil, fmt.Errorf("failed to parse corrected parameters: %w", err)
    }

    return correctedParams, nil
}
```

#### Step 2.5: Add callAgentWithBody Function (REQUIRED)

The current `callAgent` function doesn't return the error response body, which is needed for type error detection.

```go
// executor.go - Add new function after callAgent (around line 921)

// callAgentWithBody makes an HTTP call and returns both response and error body
// This is needed for Layer 3 validation feedback to detect type errors from response
func (e *SmartExecutor) callAgentWithBody(ctx context.Context, url string, parameters map[string]interface{}) (string, string, error) {
    // Prepare request body
    body, err := json.Marshal(parameters)
    if err != nil {
        return "", "", fmt.Errorf("failed to marshal parameters: %w", err)
    }

    // Create request
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    if err != nil {
        return "", "", fmt.Errorf("failed to create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    // Make the request
    resp, err := e.httpClient.Do(req)
    if err != nil {
        return "", "", fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    // Read response body (always, even on error)
    respBody, _ := io.ReadAll(resp.Body)
    respBodyStr := string(respBody)

    // Check status code
    if resp.StatusCode != http.StatusOK {
        return "", respBodyStr, fmt.Errorf("agent returned status %d: %s", resp.StatusCode, respBodyStr)
    }

    // Parse successful response
    var result map[string]interface{}
    if err := json.Unmarshal(respBody, &result); err != nil {
        return "", respBodyStr, fmt.Errorf("failed to decode response: %w", err)
    }

    responseBytes, _ := json.Marshal(result)
    return string(responseBytes), respBodyStr, nil
}
```

**Required import**: Add `"io"` to the imports in executor.go

#### Step 3: Integrate into Executor

```go
// executor.go - Modify executeStep to include validation feedback

func (e *SmartExecutor) executeStep(ctx context.Context, step RoutingStep) *StepResult {
    // ... existing code ...

    // Execute with retry logic including validation feedback
    maxAttempts := 3
    validationRetries := 0
    maxValidationRetries := e.maxValidationRetries // Default: 2

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        result.Attempts = attempt

        // Make the HTTP request (using new callAgentWithBody)
        response, responseBody, err := e.callAgentWithBody(ctx, url, parameters)

        if err == nil {
            result.Success = true
            result.Response = response
            break
        }

        // Check if this is a type-related error that could be fixed by LLM
        // Only attempt if validation feedback is enabled
        if e.validationFeedbackEnabled && isTypeRelatedError(err, responseBody) && validationRetries < e.maxValidationRetries {
            validationRetries++

            if e.logger != nil {
                e.logger.Info("Type error detected, requesting LLM correction", map[string]interface{}{
                    "operation":          "validation_feedback",
                    "step_id":            step.StepID,
                    "validation_retry":   validationRetries,
                    "error":              err.Error(),
                })
            }

            // Get capability schema for correction context
            capabilitySchema := e.findCapabilitySchema(agentInfo, capability)

            // Request correction from LLM (via orchestrator callback)
            if e.correctionCallback != nil && capabilitySchema != nil {
                correctedParams, corrErr := e.correctionCallback(ctx, step, parameters, err.Error(), capabilitySchema)
                if corrErr == nil {
                    parameters = correctedParams

                    if e.logger != nil {
                        e.logger.Debug("Parameters corrected by LLM", map[string]interface{}{
                            "operation":     "validation_feedback_success",
                            "step_id":       step.StepID,
                            "new_params":    correctedParams,
                        })
                    }

                    // Don't count this as a regular retry, continue the loop
                    attempt-- // Retry with corrected parameters
                    continue
                }
            }
        }

        // Regular retry logic for non-type errors
        result.Error = err.Error()
        if attempt < maxAttempts {
            time.Sleep(time.Duration(attempt) * time.Second)
        }
    }

    return result
}
```

#### Step 4: Add Correction Callback and Config to Executor

```go
// executor.go - Add callback type and new fields to SmartExecutor struct

// CorrectionCallback is called when validation feedback is needed
type CorrectionCallback func(
    ctx context.Context,
    step RoutingStep,
    originalParams map[string]interface{},
    errorMessage string,
    schema *EnhancedCapability,
) (map[string]interface{}, error)

// Current SmartExecutor struct (lines 39-48):
type SmartExecutor struct {
    catalog        *AgentCatalog
    httpClient     *http.Client
    maxConcurrency int
    semaphore      chan struct{}
    logger         core.Logger

    // NEW: Add these fields for Layer 3
    correctionCallback      CorrectionCallback
    validationFeedbackEnabled bool  // Default: true
    maxValidationRetries    int     // Default: 2
}

// Update NewSmartExecutor to set defaults (around line 51):
func NewSmartExecutor(catalog *AgentCatalog) *SmartExecutor {
    // ... existing code ...
    return &SmartExecutor{
        catalog:                   catalog,
        maxConcurrency:            maxConcurrency,
        semaphore:                 make(chan struct{}, maxConcurrency),
        httpClient:                tracedClient,
        // NEW: Layer 3 defaults
        validationFeedbackEnabled: true,  // Enable by default
        maxValidationRetries:      2,     // Up to 2 correction attempts
    }
}

// SetCorrectionCallback sets the callback for validation feedback
func (e *SmartExecutor) SetCorrectionCallback(cb CorrectionCallback) {
    e.correctionCallback = cb
}

// SetValidationFeedback configures Layer 3 validation feedback
func (e *SmartExecutor) SetValidationFeedback(enabled bool, maxRetries int) {
    e.validationFeedbackEnabled = enabled
    if maxRetries > 0 {
        e.maxValidationRetries = maxRetries
    }
}
```

#### Step 5: Wire Up in Orchestrator

```go
// orchestrator.go - Connect executor to correction method

func NewAIOrchestrator(config *OrchestratorConfig, discovery core.Discovery, aiClient core.AIClient) *AIOrchestrator {
    o := &AIOrchestrator{
        // ... existing initialization (lines 44-66) ...
    }

    // Wire up Layer 3 validation feedback (add after line 66)
    if config.ExecutionOptions.ValidationFeedbackEnabled {
        o.executor.SetCorrectionCallback(o.requestParameterCorrection)
        o.executor.SetValidationFeedback(true, config.ExecutionOptions.MaxValidationRetries)
    }

    return o
}
```

**Note**: The `requestParameterCorrection` method needs access to `o.aiClient`, which the orchestrator has. The callback pattern allows the executor to trigger LLM correction without directly depending on the AI client.

### Configuration

```go
// interfaces.go - Add to ExecutionOptions

type ExecutionOptions struct {
    // ... existing fields ...

    // ValidationFeedback enables LLM-based parameter correction on type errors
    ValidationFeedbackEnabled bool          `json:"validation_feedback_enabled"`
    MaxValidationRetries      int           `json:"max_validation_retries"` // Default: 2
}

// DefaultConfig update
func DefaultConfig() *OrchestratorConfig {
    return &OrchestratorConfig{
        ExecutionOptions: ExecutionOptions{
            // ... existing ...
            ValidationFeedbackEnabled: true,  // Enable by default
            MaxValidationRetries:      2,     // Up to 2 correction attempts
        },
    }
}
```

### Metrics for Layer 3

```go
// Track validation feedback events
telemetry.Counter("orchestrator.validation_feedback.attempts",
    "step_id", step.StepID,
    "agent", step.AgentName)

telemetry.Counter("orchestrator.validation_feedback.success",
    "step_id", step.StepID,
    "retry_number", strconv.Itoa(validationRetries))

telemetry.Counter("orchestrator.validation_feedback.failed",
    "step_id", step.StepID,
    "reason", "max_retries_exceeded")
```

### Testing Layer 3

```go
func TestValidationFeedbackRetry(t *testing.T) {
    // Mock tool that fails with type error on first call, succeeds on second
    callCount := 0
    mockTool := func(params map[string]interface{}) error {
        callCount++
        if callCount == 1 {
            // First call: params have wrong types
            if _, ok := params["lat"].(string); ok {
                return fmt.Errorf("json: cannot unmarshal string into Go struct field .lat of type float64")
            }
        }
        // Second call: params should be corrected
        return nil
    }

    // Mock LLM correction
    mockCorrection := func(ctx context.Context, step RoutingStep, params map[string]interface{}, err string, schema *EnhancedCapability) (map[string]interface{}, error) {
        return map[string]interface{}{
            "lat": 35.6897, // Corrected type
            "lon": 139.69,
        }, nil
    }

    executor := NewSmartExecutor(...)
    executor.SetCorrectionCallback(mockCorrection)

    result := executor.executeStep(ctx, step)

    assert.True(t, result.Success)
    assert.Equal(t, 2, callCount) // Should have retried once
}
```

### Cost Considerations

| Scenario | Additional LLM Calls | Latency Impact |
|----------|---------------------|----------------|
| No type errors | 0 | None |
| Type error, 1 correction | 1 | +1-3 seconds |
| Type error, 2 corrections | 2 | +2-6 seconds |
| Max retries exceeded | 2 | +2-6 seconds (still fails) |

**Recommendation**: Enable by default but allow configuration to disable for cost-sensitive deployments.

---

## Alternative Approaches Considered

### Option 1: Improve LLM Prompt Only

**Pros**: Simple, no code changes
**Cons**: LLMs don't reliably follow instructions (~70-80% success)
**Verdict**: Helpful but insufficient as sole solution

### Option 2: Post-Process Parsed Plan (in orchestrator.go)

**Pros**: Catches issues early
**Cons**: Requires catalog access at parse time, more complex
**Verdict**: Viable but executor is cleaner location

### Option 3: Typed RoutingStep.Metadata Struct

**Pros**: Compile-time safety, most robust
**Cons**: Breaking change to interfaces, migration required
**Verdict**: Best for v2 API, not suitable for immediate fix

### Option 4: Validation Feedback Retry

**Pros**: Highest success rate (~99%)
**Cons**: Additional LLM calls, increased latency and cost
**Verdict**: Good future enhancement, not primary fix

---

## Testing Plan

### Unit Tests

```go
func TestCoerceParameterTypes(t *testing.T) {
    schema := []Parameter{
        {Name: "lat", Type: "float64"},
        {Name: "lon", Type: "float64"},
        {Name: "count", Type: "integer"},
        {Name: "enabled", Type: "boolean"},
        {Name: "name", Type: "string"},
    }

    tests := []struct {
        name     string
        input    map[string]interface{}
        expected map[string]interface{}
    }{
        {
            name: "string numbers to float64",
            input: map[string]interface{}{
                "lat": "35.6897",
                "lon": "139.6917",
            },
            expected: map[string]interface{}{
                "lat": 35.6897,
                "lon": 139.6917,
            },
        },
        {
            name: "string to integer",
            input: map[string]interface{}{
                "count": "42",
            },
            expected: map[string]interface{}{
                "count": int64(42),
            },
        },
        {
            name: "string to boolean",
            input: map[string]interface{}{
                "enabled": "true",
            },
            expected: map[string]interface{}{
                "enabled": true,
            },
        },
        {
            name: "already correct types unchanged",
            input: map[string]interface{}{
                "lat":     48.8566,
                "count":   int64(10),
                "enabled": false,
            },
            expected: map[string]interface{}{
                "lat":     48.8566,
                "count":   int64(10),
                "enabled": false,
            },
        },
        {
            name: "invalid coercion returns original",
            input: map[string]interface{}{
                "lat": "not-a-number",
            },
            expected: map[string]interface{}{
                "lat": "not-a-number",
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, _ := coerceParameterTypes(tt.input, schema)
            // Assert result matches expected
        })
    }
}
```

### Integration Test

```bash
# After deploying fix, test natural language mode
curl -X POST http://localhost:8094/orchestrate/natural \
  -H "Content-Type: application/json" \
  -d '{
    "request": "What is the weather in Berlin, Germany?",
    "use_ai": true
  }'

# Expected: All tools succeed, no type errors in logs
```

---

## Rollout Plan

### Phase 1: Layer 2 - Schema Coercion (Week 1)

| Step | Task | Files Modified |
|------|------|----------------|
| 1.1 | Add `coerceParameterTypes()` function | `executor.go` |
| 1.2 | Add `findCapabilitySchema()` helper | `executor.go` |
| 1.3 | Integrate coercion into `executeStep()` | `executor.go` |
| 1.4 | Add debug logging for coercion events | `executor.go` |
| 1.5 | Write unit tests | `executor_test.go` |
| 1.6 | Deploy to test environment | - |
| 1.7 | Verify with travel-research-agent | - |

**Expected Outcome**: ~95% success rate for natural language mode

### Phase 2: Layer 1 - Prompt Improvement (Week 2)

| Step | Task | Files Modified |
|------|------|----------------|
| 2.1 | Add type examples to planning prompt | `orchestrator.go` |
| 2.2 | Add few-shot examples | `orchestrator.go` |
| 2.3 | Test with various LLM providers | - |
| 2.4 | Monitor coercion frequency reduction | - |

**Expected Outcome**: Reduced coercion events (fewer type errors at source)

### Phase 3: Layer 3 - Validation Feedback (Week 3-4)

| Step | Task | Files Modified |
|------|------|----------------|
| 3.1 | Add `isTypeRelatedError()` detection | `executor.go` |
| 3.2 | Add `CorrectionCallback` type | `executor.go` |
| 3.3 | Implement `requestParameterCorrection()` | `orchestrator.go` |
| 3.4 | Wire callback in orchestrator init | `orchestrator.go` |
| 3.5 | Add configuration options | `interfaces.go` |
| 3.6 | Add telemetry metrics | `executor.go` |
| 3.7 | Write integration tests | `orchestrator_test.go` |
| 3.8 | Deploy with feature flag | - |

**Expected Outcome**: ~99% success rate for natural language mode

### Phase 4: Monitoring & Tuning (Ongoing)

| Metric | Threshold | Action |
|--------|-----------|--------|
| Coercion rate | >20% | Review prompt, may need enhancement |
| Validation feedback rate | >5% | Layer 2 may need improvement |
| LLM correction success | <80% | Review correction prompt |
| Total success rate | <95% | Investigate edge cases |

---

## Success Metrics

| Metric | Before | After Layer 2 | After All Layers |
|--------|--------|---------------|------------------|
| Natural language tool success rate | ~70% | >95% | >99% |
| Type-related 400 errors | Common | Rare | Near zero |
| Schema coercion events | N/A | Tracked | Decreasing |
| Validation feedback events | N/A | N/A | <5% of requests |
| Average latency (no errors) | Baseline | Same | Same |
| Average latency (with correction) | N/A | N/A | +1-3 seconds |

---

## Decision Summary

### Recommended Implementation Order

| Priority | Layer | Effectiveness | Effort | Deploy |
|----------|-------|---------------|--------|--------|
| **1st** | Layer 2: Schema Coercion | ~95% | Medium | Week 1 |
| **2nd** | Layer 1: Prompt Improvement | +5-10% | Low | Week 2 |
| **3rd** | Layer 3: Validation Feedback | ~99% | High | Week 3-4 |

### Rationale for Multi-Layer Approach

1. **Defense in Depth**: No single layer is 100% effective; combining layers provides robust coverage

2. **Graceful Degradation**: Each layer catches what the previous missed:
   - Layer 1 reduces errors at source (prevention)
   - Layer 2 fixes remaining errors deterministically (correction)
   - Layer 3 handles edge cases with LLM assistance (recovery)

3. **Cost Optimization**: Most requests are handled by cheap layers (1 & 2); expensive Layer 3 only triggers for edge cases

4. **Observability**: Each layer provides metrics to identify where improvements are needed

### Configuration Recommendations

```go
// Production configuration
config := orchestration.DefaultConfig()
config.ExecutionOptions.ValidationFeedbackEnabled = true  // Enable Layer 3
config.ExecutionOptions.MaxValidationRetries = 2          // Limit LLM calls

// Cost-sensitive configuration
config.ExecutionOptions.ValidationFeedbackEnabled = false // Disable Layer 3
// Relies on Layers 1 & 2 only (~95% success)

// Maximum reliability configuration
config.ExecutionOptions.ValidationFeedbackEnabled = true
config.ExecutionOptions.MaxValidationRetries = 3  // More retries
// ~99%+ success, higher cost
```

---

## Files to Modify

| File | Changes | Layer |
|------|---------|-------|
| `executor.go` | Add coercion functions, callback, error detection | 2, 3 |
| `orchestrator.go` | Enhance prompt, add correction method | 1, 3 |
| `interfaces.go` | Add configuration options | 3 |
| `executor_test.go` | Unit tests for coercion | 2 |
| `orchestrator_test.go` | Integration tests for feedback | 3 |

---

## Required Imports Summary

When implementing all layers, ensure these imports are present:

**executor.go**:
```go
import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"         // NEW: For Layer 3 callAgentWithBody
    "strconv"    // NEW: For Layer 2 coercion
    "strings"    // NEW: For Layer 2 coercion
    // ... existing imports ...
)
```

**orchestrator.go** (no new imports needed for Layers 1 & 3)

**interfaces.go** (no new imports needed)

---

## Compatibility with TOOL_SCHEMA_DISCOVERY_GUIDE.md

This fix is designed to work with tools that follow the [TOOL_SCHEMA_DISCOVERY_GUIDE.md](../docs/TOOL_SCHEMA_DISCOVERY_GUIDE.md):

- **Phase 2 Field Hints** (`core.FieldHint.Type`) flow through to `orchestration.Parameter.Type`
- Layer 2 coercion uses this type information automatically
- No changes needed to existing tools - just ensure they define `InputSummary` with proper `FieldHint.Type`

The bridge is in `catalog.go:325-353` (`convertBasicCapabilities`) which converts `core.FieldHint` → `orchestration.Parameter`.

---

*Document created: December 2024*
*Status: Layer 2 & Layer 3 IMPLEMENTED with Production Telemetry*
*Last updated: December 9, 2024 - Layer 3 (Validation Feedback Retry) implemented with telemetry*

---

## Implementation Status

### Layer 2: Schema-Based Type Coercion ✅ COMPLETED

**Implemented in**: `executor.go`

**Functions added**:
- `findCapabilitySchema()` - Finds capability schema for an agent (line 865-875)
- `coerceParameterTypes()` - Converts string parameters to expected types (line 877-912)
- `coerceValue()` - Coerces individual values based on expected type (line 914-951)

**Integration point**: Added after parameter interpolation in `executeStep()` (lines 676-712)

**Telemetry instrumentation** (added per telemetry/ARCHITECTURE.md):
- **Span Events**: `telemetry.AddSpanEvent(ctx, "type_coercion_applied", ...)` - Adds visible events to distributed traces in Jaeger/Grafana for coercion visibility
- **Metrics**: `telemetry.Counter("orchestration.type_coercion.applied", ...)` - Enables monitoring of coercion frequency across the system
- **Attributes**: `coercions_count`, `capability`, `step_id` for detailed observability

**Telemetry code snippet** (lines 683-697):
```go
if len(coercionLog) > 0 {
    // Span event for distributed tracing
    telemetry.AddSpanEvent(ctx, "type_coercion_applied",
        attribute.Int("coercions_count", len(coercionLog)),
        attribute.String("capability", capability),
        attribute.String("step_id", step.StepID),
    )

    // Counter metric for monitoring dashboards
    telemetry.Counter("orchestration.type_coercion.applied",
        "capability", capability,
        "module", telemetry.ModuleOrchestration,
    )
}
```

**Tests added** in `executor_test.go`:
- `TestCoerceValue` - 18 test cases covering float64, integer, boolean, and edge cases
- `TestCoerceParameterTypes` - 8 test cases covering various coercion scenarios
- `TestFindCapabilitySchema` - 4 test cases for schema lookup
- `TestSmartExecutor_TypeCoercionIntegration` - End-to-end test verifying string→float64 coercion

**Prometheus Query Examples**:
```promql
# Count of type coercions by capability
sum(rate(orchestration_type_coercion_applied_total[5m])) by (capability)

# Total coercions across the orchestration module
sum(rate(orchestration_type_coercion_applied_total[5m]))
```

### Layer 1: Prompt Improvement ⏳ PENDING
- Optional enhancement to reduce LLM type errors at source
- Can use existing `PromptBuilder` interface for customization

### Layer 3: Validation Feedback Retry ✅ COMPLETED

**Implemented in**: `executor.go`, `orchestrator.go`, `interfaces.go`

**Functions added in executor.go**:
- `CorrectionCallback` type - Callback function type for LLM-based parameter correction (lines 41-49)
- `isTypeRelatedError()` - Detects type-related errors from error messages and response bodies (lines 1009-1035)
- `callAgentWithBody()` - HTTP call that returns response body even on error for type error detection (lines 1116-1184)
- `SetCorrectionCallback()` - Sets the callback for validation feedback (line 101)
- `SetValidationFeedback()` - Configures Layer 3 validation feedback (lines 103-108)

**Functions added in orchestrator.go**:
- `requestParameterCorrection()` - LLM-based parameter correction with type rules (lines 167-250)
- `extractJSON()` - Helper to extract JSON from markdown-wrapped LLM responses (lines 252-271)

**Struct updates**:
- `SmartExecutor` - Added `correctionCallback`, `validationFeedbackEnabled`, `maxValidationRetries` fields (lines 61-64)
- `ExecutionOptions` - Added `ValidationFeedbackEnabled`, `MaxValidationRetries` config (interfaces.go lines 169-172)

**Integration point**: Added to `executeStep()` retry loop (lines 785-958)

**Telemetry instrumentation** (aligned with telemetry/ARCHITECTURE.md):
- **Span Events**:
  - `validation_feedback_started` - When type error detected and correction initiated
  - `validation_feedback_success` - When LLM correction succeeds
  - `validation_feedback_failed` - When correction fails (callback error or max retries)
- **Counter Metrics**:
  - `orchestration.validation_feedback.attempts` - Tracks correction attempts
  - `orchestration.validation_feedback.success` - Tracks successful corrections
  - `orchestration.validation_feedback.failed` - Tracks failed corrections

**Configuration defaults** (interfaces.go DefaultConfig):
```go
ValidationFeedbackEnabled: true,  // Enable by default for production reliability
MaxValidationRetries:      2,     // Up to 2 correction attempts
```

**Tests added** in `executor_test.go`:
- `TestIsTypeRelatedError` - 14 test cases covering type error detection patterns
- `TestSmartExecutor_ValidationFeedback` - Integration test for validation feedback flow
- `TestSmartExecutor_ValidationFeedbackDisabled` - Verifies feature can be disabled

**Type error patterns detected**:
```go
typeErrorPatterns := []string{
    "cannot unmarshal string into",
    "cannot unmarshal number into",
    "json: cannot unmarshal",
    "type mismatch",
    "invalid type",
    "expected number",
    "expected string",
    "expected boolean",
    "invalid value",
}
```

**Prometheus Query Examples**:
```promql
# Validation feedback attempts rate
sum(rate(orchestration_validation_feedback_attempts_total[5m])) by (agent)

# Validation feedback success rate
sum(rate(orchestration_validation_feedback_success_total[5m])) /
sum(rate(orchestration_validation_feedback_attempts_total[5m]))

# Failed corrections by reason
sum(rate(orchestration_validation_feedback_failed_total[5m])) by (reason)
```

**Jaeger Trace Query**:
- Search for spans with event name `validation_feedback_started`
- Filter by `orchestration.type_error=true` attribute
