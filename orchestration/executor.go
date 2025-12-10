package orchestration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// contextKey is a custom type for context keys to avoid collisions
type executorContextKey string

// Context keys for step execution
const (
	// dependencyResultsKey stores results from dependent steps for template interpolation
	dependencyResultsKey executorContextKey = "dependency_results"
)

// Pre-compiled regex patterns for template substitution (performance optimization)
// Compiling once at package level avoids repeated compilation overhead
var (
	// stepOutputTemplatePattern matches {{stepId.fieldPath}} for step output references
	// Examples: {{geocode.latitude}}, {{weather.data.temp}}, {{country-info.data.currency.code}}
	// Note: Step IDs can contain hyphens (e.g., country-info), so we use [\w-]+ for the step ID
	stepOutputTemplatePattern = regexp.MustCompile(`\{\{([\w-]+)\.([\w-]+(?:\.[\w-]+)*)\}\}`)
)

// CorrectionCallback is called when validation feedback is needed (Layer 3).
// It requests the LLM to correct parameters based on type error feedback.
type CorrectionCallback func(
	ctx context.Context,
	step RoutingStep,
	originalParams map[string]interface{},
	errorMessage string,
	schema *EnhancedCapability,
) (map[string]interface{}, error)

// SmartExecutor handles intelligent execution of routing plans
type SmartExecutor struct {
	catalog        *AgentCatalog
	httpClient     *http.Client
	maxConcurrency int
	semaphore      chan struct{}

	// Observability (follows framework design principles)
	logger core.Logger // For structured logging

	// Layer 3: Validation Feedback configuration
	correctionCallback        CorrectionCallback // Callback to request LLM parameter correction
	validationFeedbackEnabled bool               // Enable/disable validation feedback (default: true)
	maxValidationRetries      int                // Maximum validation correction attempts (default: 2)
}

// NewSmartExecutor creates a new smart executor
func NewSmartExecutor(catalog *AgentCatalog) *SmartExecutor {
	maxConcurrency := 5

	// Create a traced HTTP client that automatically propagates trace context
	// to downstream services via W3C TraceContext headers.
	// This enables distributed tracing across the orchestration workflow.
	// Per FRAMEWORK_DESIGN_PRINCIPLES.md, orchestration is allowed to import telemetry.
	tracedClient := telemetry.NewTracedHTTPClient(nil)

	// Configurable timeout: GOMIND_ORCHESTRATION_TIMEOUT (default: 60s)
	// For long-running AI workflows, set to higher values (e.g., "5m", "10m")
	timeout := 60 * time.Second
	if envTimeout := os.Getenv("GOMIND_ORCHESTRATION_TIMEOUT"); envTimeout != "" {
		if parsed, err := time.ParseDuration(envTimeout); err == nil {
			timeout = parsed
		}
	}
	tracedClient.Timeout = timeout

	return &SmartExecutor{
		catalog:        catalog,
		maxConcurrency: maxConcurrency,
		semaphore:      make(chan struct{}, maxConcurrency),
		httpClient:     tracedClient,
		// Layer 3: Validation Feedback defaults
		validationFeedbackEnabled: true, // Enable by default for production reliability
		maxValidationRetries:      2,    // Up to 2 correction attempts
	}
}

// SetCorrectionCallback sets the callback for Layer 3 validation feedback.
// The callback is called when a type-related error is detected, requesting the LLM
// to correct the parameters based on error feedback.
func (e *SmartExecutor) SetCorrectionCallback(cb CorrectionCallback) {
	e.correctionCallback = cb
}

// SetValidationFeedback configures Layer 3 validation feedback behavior.
// enabled: Whether to attempt LLM correction on type errors
// maxRetries: Maximum number of correction attempts (default: 2)
func (e *SmartExecutor) SetValidationFeedback(enabled bool, maxRetries int) {
	e.validationFeedbackEnabled = enabled
	if maxRetries > 0 {
		e.maxValidationRetries = maxRetries
	}
}

// SetLogger sets the logger provider (follows framework design principles)
func (e *SmartExecutor) SetLogger(logger core.Logger) {
	if logger == nil {
		e.logger = &core.NoOpLogger{}
	} else {
		e.logger = logger
	}
}

// Execute runs a routing plan and collects agent responses.
// This method orchestrates the execution of all steps in the plan,
// respecting dependencies and running steps in parallel where possible.
// It includes panic recovery for each step to ensure resilience.
func (e *SmartExecutor) Execute(ctx context.Context, plan *RoutingPlan) (*ExecutionResult, error) {
	startTime := time.Now()

	// Get trace context for log correlation
	tc := telemetry.GetTraceContext(ctx)

	// Add span event for plan execution start
	telemetry.AddSpanEvent(ctx, "plan_execution_started",
		attribute.String("plan_id", plan.PlanID),
		attribute.Int("step_count", len(plan.Steps)),
	)

	if e.logger != nil {
		e.logger.Debug("Starting plan execution", map[string]interface{}{
			"operation":       "execute_plan",
			"plan_id":         plan.PlanID,
			"step_count":      len(plan.Steps),
			"max_concurrency": e.maxConcurrency,
			"trace_id":        tc.TraceID,
			"span_id":         tc.SpanID,
		})
	}

	result := &ExecutionResult{
		PlanID:        plan.PlanID,
		Steps:         make([]StepResult, 0, len(plan.Steps)),
		Success:       true,
		TotalDuration: 0,
		Metadata:      make(map[string]interface{}),
	}

	// Create a map to store step results for dependency resolution
	stepResults := make(map[string]*StepResult)
	var resultsMutex sync.Mutex

	// Execute steps respecting dependencies
	executed := make(map[string]bool)

	for len(executed) < len(plan.Steps) {
		// Find steps that can be executed (all dependencies met)
		readySteps := e.findReadySteps(plan, executed, stepResults)

		if len(readySteps) == 0 {
			// No steps ready - check if it's due to failed dependencies or circular dependency
			// Mark remaining steps as skipped if their dependencies failed
			hasSkipped := false
			for _, step := range plan.Steps {
				if executed[step.StepID] {
					continue
				}
				// Check if this step is blocked by failed dependencies
				blockedByFailure := false
				for _, dep := range step.DependsOn {
					if result, ok := stepResults[dep]; ok && !result.Success {
						blockedByFailure = true
						break
					}
				}
				if blockedByFailure {
					// Mark this step as skipped due to failed dependency
					skippedResult := StepResult{
						StepID:    step.StepID,
						AgentName: step.AgentName,
						Namespace: step.Namespace,
						Success:   false,
						Error:     "skipped due to failed dependency",
						StartTime: time.Now(),
						Duration:  0,
					}
					stepResults[step.StepID] = &skippedResult
					result.Steps = append(result.Steps, skippedResult)
					executed[step.StepID] = true
					result.Success = false
					hasSkipped = true
				}
			}

			if hasSkipped {
				// We skipped some steps, continue to check if more can be executed
				continue
			}

			// No steps were skipped, this is likely a circular dependency
			return nil, fmt.Errorf("no executable steps found - check for circular dependencies")
		}

		if e.logger != nil {
			stepIDs := make([]string, len(readySteps))
			for i, step := range readySteps {
				stepIDs[i] = step.StepID
			}
			e.logger.Debug("Executing steps in parallel", map[string]interface{}{
				"operation":      "parallel_execution",
				"plan_id":        plan.PlanID,
				"ready_steps":    stepIDs,
				"parallel_count": len(readySteps),
			})
		}

		// Execute ready steps in parallel
		var wg sync.WaitGroup
		for _, step := range readySteps {
			wg.Add(1)
			go func(s RoutingStep) {
				// Track when this step started for accurate timing
				stepStartTime := time.Now()

				// Acquire semaphore for concurrency control BEFORE setting up defer
				// This ensures the semaphore is always released even if panic occurs
				e.semaphore <- struct{}{}

				defer func() {
					// Always release semaphore first
					<-e.semaphore

					if r := recover(); r != nil {
						// Panic recovery mechanism for step execution.
						// Captures any panic that occurs during step execution and converts it
						// to a failed step result rather than crashing the entire workflow.
						// This ensures that one failing step doesn't break the entire execution.
						stackTrace := string(debug.Stack())
						errorMsg := fmt.Sprintf("step %s execution panic: %v", s.StepID, r)

						// Log panic with structured logging
						if e.logger != nil {
							e.logger.Error("Step execution panicked", map[string]interface{}{
								"step_id":    s.StepID,
								"agent_name": s.AgentName,
								"panic":      fmt.Sprintf("%v", r),
								"stack":      string(stackTrace),
							})
						}

						// Store panic as a failed step result in the execution results.
						// Uses direct Lock/Unlock instead of defer to avoid potential deadlock
						// in nested defer statements during panic recovery.
						resultsMutex.Lock()

						panicResult := StepResult{
							StepID:    s.StepID,
							AgentName: s.AgentName,
							Namespace: s.Namespace,
							Success:   false,
							Error:     errorMsg,
							StartTime: stepStartTime, // Use the actual start time
							Duration:  time.Since(stepStartTime),
						}

						stepResults[s.StepID] = &panicResult
						result.Steps = append(result.Steps, panicResult)
						executed[s.StepID] = true
						result.Success = false

						resultsMutex.Unlock() // Unlock immediately, no defer
					}
					wg.Done()
				}()

				// Build context for step execution
				stepCtx := e.buildStepContext(ctx, s, stepResults)

				// Execute the step
				stepResult := e.executeStep(stepCtx, s)

				// Store result
				resultsMutex.Lock()
				stepResults[s.StepID] = &stepResult
				result.Steps = append(result.Steps, stepResult)
				executed[s.StepID] = true

				if !stepResult.Success {
					result.Success = false
				}
				resultsMutex.Unlock()
			}(step)
		}

		// Wait for this batch to complete
		wg.Wait()

		completedSteps := len(executed)
		totalSteps := len(plan.Steps)
		if e.logger != nil {
			e.logger.Debug("Execution batch completed", map[string]interface{}{
				"operation":        "batch_complete",
				"plan_id":          plan.PlanID,
				"completed_steps":  completedSteps,
				"total_steps":      totalSteps,
				"progress_percent": float64(completedSteps) / float64(totalSteps) * 100,
			})
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	result.TotalDuration = time.Since(startTime)

	failedSteps := 0
	for _, step := range result.Steps {
		if !step.Success {
			failedSteps++
		}
	}

	// Add span event for plan execution completion
	telemetry.AddSpanEvent(ctx, "plan_execution_completed",
		attribute.String("plan_id", plan.PlanID),
		attribute.Bool("success", result.Success),
		attribute.Int("failed_steps", failedSteps),
		attribute.Int("total_steps", len(plan.Steps)),
		attribute.Int64("duration_ms", result.TotalDuration.Milliseconds()),
	)

	if e.logger != nil {
		e.logger.Info("Plan execution finished", map[string]interface{}{
			"operation":    "execute_plan_complete",
			"plan_id":      plan.PlanID,
			"success":      result.Success,
			"failed_steps": failedSteps,
			"total_steps":  len(plan.Steps),
			"duration_ms":  result.TotalDuration.Milliseconds(),
			"trace_id":     tc.TraceID,
			"span_id":      tc.SpanID,
		})
	}

	return result, nil
}

// findReadySteps identifies steps that can be executed.
// A step is ready when all its dependencies have been successfully executed.
// This enables parallel execution of independent steps.
func (e *SmartExecutor) findReadySteps(plan *RoutingPlan, executed map[string]bool, results map[string]*StepResult) []RoutingStep {
	var ready []RoutingStep

	for _, step := range plan.Steps {
		if executed[step.StepID] {
			continue
		}

		// Check if all dependencies are satisfied
		allDepsReady := true
		blockedBy := ""
		blockReason := ""

		for _, dep := range step.DependsOn {
			if !executed[dep] {
				allDepsReady = false
				blockedBy = dep
				blockReason = "not_executed"
				break
			}
			// Check if dependency was successful
			if result, ok := results[dep]; ok && !result.Success {
				// Skip steps whose dependencies failed
				allDepsReady = false
				blockedBy = dep
				blockReason = "failed"
				break
			}
		}

		if allDepsReady {
			ready = append(ready, step)
			if e.logger != nil {
				e.logger.Debug("Step ready for execution", map[string]interface{}{
					"operation":    "dependency_resolution",
					"step_id":      step.StepID,
					"agent_name":   step.AgentName,
					"dependencies": step.DependsOn,
					"status":       "ready",
				})
			}
		} else if e.logger != nil {
			e.logger.Debug("Step blocked by dependency", map[string]interface{}{
				"operation":    "dependency_resolution",
				"step_id":      step.StepID,
				"agent_name":   step.AgentName,
				"blocked_by":   blockedBy,
				"block_reason": blockReason,
				"dependencies": step.DependsOn,
				"status":       "blocked",
			})
		}
	}

	return ready
}

// buildStepContext creates context with dependency results for template interpolation
func (e *SmartExecutor) buildStepContext(ctx context.Context, step RoutingStep, results map[string]*StepResult) context.Context {
	// Add dependency results to context for template interpolation
	// The results are stored as parsed JSON so templates like {{stepId.field}} can be resolved
	deps := make(map[string]map[string]interface{})
	for _, depID := range step.DependsOn {
		if result, ok := results[depID]; ok && result.Response != "" {
			// Parse the JSON response to enable field access in templates
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result.Response), &parsed); err != nil {
				// Log parsing failure - this could indicate a non-JSON response from a tool
				if e.logger != nil {
					e.logger.Warn("Failed to parse dependency response as JSON for template interpolation", map[string]interface{}{
						"step_id":      step.StepID,
						"dependency":   depID,
						"error":        err.Error(),
						"response_len": len(result.Response),
					})
				}
				// Continue without this dependency - templates referencing it will be unresolved
			} else {
				deps[depID] = parsed
			}
		}
	}

	return context.WithValue(ctx, dependencyResultsKey, deps)
}

// interpolateParameters substitutes template placeholders in parameters with values from dependency results.
// Templates use the format {{stepId.fieldPath}} where:
//   - stepId: The ID of a dependent step whose result should be used
//   - fieldPath: Dot-separated path to a field in the step's JSON response (e.g., "latitude" or "data.temp")
//
// Example: {"lat": "{{geocode.latitude}}"} becomes {"lat": 35.6762} after geocode step completes
func (e *SmartExecutor) interpolateParameters(
	params map[string]interface{},
	depResults map[string]map[string]interface{},
) map[string]interface{} {
	if params == nil {
		return nil
	}

	interpolated := make(map[string]interface{})
	for key, value := range params {
		if strVal, ok := value.(string); ok {
			interpolated[key] = e.substituteTemplates(strVal, depResults)
		} else {
			interpolated[key] = value
		}
	}

	return interpolated
}

// substituteTemplates replaces template placeholders with actual values from step results.
// Supports patterns like {{stepId.field}} and {{stepId.nested.field}}.
// If the entire string is a single template that resolves to a non-string value (e.g., number),
// the actual type is preserved rather than converting to string.
//
// Template Resolution Rules:
//   - Templates referencing non-existent steps are left unchanged (logged as warning)
//   - Templates referencing non-existent fields are left unchanged (logged as warning)
//   - Numeric values (float64, int) are preserved when template is the entire string
//   - Complex types (maps, slices) are converted to string representation
func (e *SmartExecutor) substituteTemplates(
	template string,
	depResults map[string]map[string]interface{},
) interface{} {
	// Use pre-compiled regex for performance (avoids re-compilation on each call)
	matches := stepOutputTemplatePattern.FindAllStringSubmatch(template, -1)
	if len(matches) == 0 {
		return template // No templates found, return as-is
	}

	// Special case: if the entire string is a single template, preserve the value's type
	// This allows numeric values to remain as numbers instead of being converted to strings
	// Critical for parameters like latitude/longitude that must be numbers, not strings
	if len(matches) == 1 && matches[0][0] == template {
		stepID := matches[0][1]
		fieldPath := matches[0][2]

		stepData, stepExists := depResults[stepID]
		if !stepExists {
			// Step not found in dependencies - this is likely a configuration error
			if e.logger != nil {
				e.logger.Warn("Template references non-existent step dependency", map[string]interface{}{
					"template":        template,
					"referenced_step": stepID,
					"available_deps":  getDepResultKeys(depResults),
				})
			}
			return template
		}

		value := extractFieldValue(stepData, fieldPath)
		if value == nil {
			// Field not found in step response
			if e.logger != nil {
				e.logger.Warn("Template references non-existent field in step response", map[string]interface{}{
					"template":         template,
					"step_id":          stepID,
					"field_path":       fieldPath,
					"available_fields": getFieldKeys(stepData),
				})
			}
			return template
		}

		// Validate that the resolved value is a primitive type suitable for HTTP parameters
		switch v := value.(type) {
		case float64, int, int64, string, bool:
			return value // Safe primitive types
		case map[string]interface{}, []interface{}:
			// Complex types - convert to JSON string for safety
			if e.logger != nil {
				e.logger.Debug("Template resolved to complex type, converting to JSON", map[string]interface{}{
					"template":   template,
					"value_type": fmt.Sprintf("%T", v),
				})
			}
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return template // Fall back to original on marshal error
			}
			return string(jsonBytes)
		default:
			return value // Other types pass through
		}
	}

	// Multiple templates or template is part of a larger string - substitute as strings
	result := template
	for _, match := range matches {
		fullMatch := match[0]
		stepID := match[1]
		fieldPath := match[2]

		stepData, stepExists := depResults[stepID]
		if !stepExists {
			if e.logger != nil {
				e.logger.Warn("Template references non-existent step dependency", map[string]interface{}{
					"template":        fullMatch,
					"referenced_step": stepID,
				})
			}
			continue // Leave template unresolved
		}

		value := extractFieldValue(stepData, fieldPath)
		if value != nil {
			result = strings.Replace(result, fullMatch, fmt.Sprintf("%v", value), 1)
		} else if e.logger != nil {
			e.logger.Warn("Template references non-existent field", map[string]interface{}{
				"template":   fullMatch,
				"step_id":    stepID,
				"field_path": fieldPath,
			})
		}
	}

	return result
}

// getDepResultKeys returns the step IDs available in dependency results (for logging)
func getDepResultKeys(m map[string]map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// getFieldKeys returns the field names available in a step result (for logging)
func getFieldKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// extractFieldValue extracts a value from a nested map using a dot-separated path.
// For example, extractFieldValue(data, "location.lat") returns data["location"]["lat"]
func extractFieldValue(data map[string]interface{}, fieldPath string) interface{} {
	parts := strings.Split(fieldPath, ".")
	current := interface{}(data)

	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return nil // Path not found
		}
	}

	return current
}

// executeStep executes a single routing step
func (e *SmartExecutor) executeStep(ctx context.Context, step RoutingStep) StepResult {
	startTime := time.Now()

	// Get trace context for log correlation
	tc := telemetry.GetTraceContext(ctx)

	// Add span event for step execution start
	telemetry.AddSpanEvent(ctx, "step_execution_started",
		attribute.String("step_id", step.StepID),
		attribute.String("agent_name", step.AgentName),
	)

	if e.logger != nil {
		e.logger.Debug("Starting step execution", map[string]interface{}{
			"operation":  "step_execution_start",
			"step_id":    step.StepID,
			"agent_name": step.AgentName,
			"trace_id":   tc.TraceID,
			"span_id":    tc.SpanID,
		})
	}

	result := StepResult{
		StepID:      step.StepID,
		AgentName:   step.AgentName,
		Namespace:   step.Namespace,
		Instruction: step.Instruction,
		StartTime:   startTime,
		Attempts:    0,
	}

	// Get agent info from catalog
	agentInfo := e.findAgentByName(step.AgentName)
	if agentInfo == nil {
		err := fmt.Errorf("agent %s not found in catalog", step.AgentName)
		telemetry.RecordSpanError(ctx, err)
		if e.logger != nil {
			e.logger.Error("Agent not found in catalog", map[string]interface{}{
				"operation":  "agent_discovery",
				"step_id":    step.StepID,
				"agent_name": step.AgentName,
				"trace_id":   tc.TraceID,
				"span_id":    tc.SpanID,
			})
		}
		result.Success = false
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime)
		return result
	}

	if e.logger != nil {
		e.logger.Debug("Agent discovered successfully", map[string]interface{}{
			"operation":     "agent_discovery",
			"step_id":       step.StepID,
			"agent_name":    step.AgentName,
			"agent_id":      agentInfo.Registration.ID,
			"agent_address": agentInfo.Registration.Address,
		})
	}

	// Extract capability and parameters from metadata
	capability := ""
	var parameters map[string]interface{}

	if cap, ok := step.Metadata["capability"].(string); ok {
		capability = cap
	}
	if params, ok := step.Metadata["parameters"].(map[string]interface{}); ok {
		parameters = params
	}

	// Interpolate template parameters with dependency results
	// This enables templates like {{geocode.latitude}} to be replaced with actual values
	if depResults, ok := ctx.Value(dependencyResultsKey).(map[string]map[string]interface{}); ok && len(depResults) > 0 {
		interpolated := e.interpolateParameters(parameters, depResults)
		if interpolated != nil {
			if e.logger != nil {
				e.logger.Debug("Template parameters interpolated", map[string]interface{}{
					"operation":    "parameter_interpolation",
					"step_id":      step.StepID,
					"original":     parameters,
					"interpolated": interpolated,
				})
			}
			parameters = interpolated
		}
	}

	// Layer 2: Schema-Based Type Coercion
	// Coerce parameters to match capability schema types.
	// This fixes LLM-generated parameters that have incorrect types (e.g., "35.6" instead of 35.6).
	// The schema is obtained from the tool's InputSummary via the catalog.
	capabilitySchema := e.findCapabilitySchema(agentInfo, capability)
	if capabilitySchema != nil && len(capabilitySchema.Parameters) > 0 {
		coerced, coercionLog := coerceParameterTypes(parameters, capabilitySchema.Parameters)
		if len(coercionLog) > 0 {
			// Telemetry: Add span event for distributed tracing visibility
			// This allows operators to see coercion events in Jaeger/Grafana traces
			telemetry.AddSpanEvent(ctx, "type_coercion_applied",
				attribute.Int("coercions_count", len(coercionLog)),
				attribute.String("capability", capability),
				attribute.String("step_id", step.StepID),
			)

			// Telemetry: Record metric for monitoring dashboards
			// Enables tracking coercion frequency across the system
			telemetry.Counter("orchestration.type_coercion.applied",
				"capability", capability,
				"module", telemetry.ModuleOrchestration,
			)

			// Structured logging for debugging
			if e.logger != nil {
				e.logger.Debug("Parameter types coerced to match schema", map[string]interface{}{
					"operation":  "type_coercion",
					"step_id":    step.StepID,
					"capability": capability,
					"coercions":  coercionLog,
					"trace_id":   tc.TraceID,
					"span_id":    tc.SpanID,
				})
			}
		}
		parameters = coerced
	}

	// Find the capability endpoint
	endpoint := e.findCapabilityEndpoint(agentInfo, capability)
	if endpoint == "" {
		err := fmt.Errorf("capability %s not found for agent %s", capability, step.AgentName)
		telemetry.RecordSpanError(ctx, err)
		if e.logger != nil {
			e.logger.Error("Capability endpoint not found", map[string]interface{}{
				"operation":  "capability_resolution",
				"step_id":    step.StepID,
				"agent_name": step.AgentName,
				"capability": capability,
				"trace_id":   tc.TraceID,
				"span_id":    tc.SpanID,
				"available_capabilities": func() []string {
					caps := make([]string, len(agentInfo.Capabilities))
					for i, cap := range agentInfo.Capabilities {
						caps[i] = cap.Name
					}
					return caps
				}(),
			})
		}
		result.Success = false
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime)
		return result
	}

	// Build the request URL
	url := fmt.Sprintf("http://%s:%d%s",
		agentInfo.Registration.Address,
		agentInfo.Registration.Port,
		endpoint)

	// Execute with retry logic including Layer 3 validation feedback
	maxAttempts := 3
	validationRetries := 0

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result.Attempts = attempt

		if e.logger != nil {
			e.logger.Debug("Making HTTP call to agent", map[string]interface{}{
				"operation":    "agent_http_call",
				"step_id":      step.StepID,
				"agent_name":   step.AgentName,
				"attempt":      attempt,
				"max_attempts": maxAttempts,
				"url":          url,
				"capability":   capability,
				"trace_id":     tc.TraceID,
				"span_id":      tc.SpanID,
			})
		}

		// Make the HTTP request (using callAgentWithBody for Layer 3 error detection)
		response, responseBody, err := e.callAgentWithBody(ctx, url, parameters)
		if err == nil {
			if e.logger != nil {
				e.logger.Debug("Agent HTTP call successful", map[string]interface{}{
					"operation":       "agent_http_response",
					"step_id":         step.StepID,
					"agent_name":      step.AgentName,
					"attempt":         attempt,
					"response_length": len(response),
					"trace_id":        tc.TraceID,
					"span_id":         tc.SpanID,
				})
			}
			result.Success = true
			result.Response = response
			break
		}

		// Layer 3: Check if this is a type-related error that could be fixed by LLM
		if e.validationFeedbackEnabled && e.correctionCallback != nil &&
			validationRetries < e.maxValidationRetries && isTypeRelatedError(err, responseBody) {

			validationRetries++

			// Telemetry: Add span event for validation feedback attempt
			telemetry.AddSpanEvent(ctx, "validation_feedback_started",
				attribute.String("step_id", step.StepID),
				attribute.String("capability", capability),
				attribute.Int("validation_retry", validationRetries),
				attribute.String("error_type", "type_mismatch"),
			)

			// Telemetry: Record metric for monitoring dashboards
			telemetry.Counter("orchestration.validation_feedback.attempts",
				"capability", capability,
				"module", telemetry.ModuleOrchestration,
			)

			if e.logger != nil {
				e.logger.Info("Type error detected, requesting LLM correction", map[string]interface{}{
					"operation":         "validation_feedback",
					"step_id":           step.StepID,
					"capability":        capability,
					"validation_retry":  validationRetries,
					"max_retries":       e.maxValidationRetries,
					"error":             err.Error(),
					"trace_id":          tc.TraceID,
					"span_id":           tc.SpanID,
				})
			}

			// Request correction from LLM via callback
			correctedParams, corrErr := e.correctionCallback(ctx, step, parameters, err.Error(), capabilitySchema)
			if corrErr == nil && correctedParams != nil {
				// Telemetry: Record successful correction
				telemetry.AddSpanEvent(ctx, "validation_feedback_success",
					attribute.String("step_id", step.StepID),
					attribute.Int("retries_used", validationRetries),
				)
				telemetry.Counter("orchestration.validation_feedback.success",
					"capability", capability,
					"module", telemetry.ModuleOrchestration,
				)

				if e.logger != nil {
					e.logger.Debug("Parameters corrected by LLM", map[string]interface{}{
						"operation":   "validation_feedback_success",
						"step_id":     step.StepID,
						"capability":  capability,
						"new_params":  correctedParams,
						"trace_id":    tc.TraceID,
						"span_id":     tc.SpanID,
					})
				}

				// Update parameters and retry without incrementing attempt count
				parameters = correctedParams
				attempt-- // Retry with corrected parameters (don't count as regular retry)
				continue
			}

			// Correction failed
			telemetry.AddSpanEvent(ctx, "validation_feedback_failed",
				attribute.String("step_id", step.StepID),
				attribute.String("reason", "llm_correction_failed"),
			)
			telemetry.Counter("orchestration.validation_feedback.failed",
				"capability", capability,
				"reason", "llm_correction_failed",
				"module", telemetry.ModuleOrchestration,
			)

			if e.logger != nil {
				e.logger.Warn("LLM parameter correction failed", map[string]interface{}{
					"operation":  "validation_feedback_failed",
					"step_id":    step.StepID,
					"capability": capability,
					"error":      corrErr.Error(),
					"trace_id":   tc.TraceID,
					"span_id":    tc.SpanID,
				})
			}
		}

		// Regular error handling
		logLevel := "Debug" // Use DEBUG for retry attempts
		if attempt == maxAttempts {
			logLevel = "Error" // Use ERROR for final failure
		}
		if e.logger != nil {
			logData := map[string]interface{}{
				"operation":    "agent_http_call_failed",
				"step_id":      step.StepID,
				"agent_name":   step.AgentName,
				"attempt":      attempt,
				"max_attempts": maxAttempts,
				"error":        err.Error(),
				"will_retry":   attempt < maxAttempts,
				"trace_id":     tc.TraceID,
				"span_id":      tc.SpanID,
			}
			if logLevel == "Error" {
				e.logger.Error("Agent HTTP call failed after all retries", logData)
			} else {
				e.logger.Debug("Agent HTTP call failed, retrying", logData)
			}
		}

		result.Error = err.Error()

		// Don't retry if context is cancelled
		select {
		case <-ctx.Done():
			return result // Exit immediately on context cancellation
		default:
		}

		// Wait before retry
		if attempt < maxAttempts {
			retryDelay := time.Duration(attempt) * time.Second
			if e.logger != nil {
				e.logger.Debug("Waiting before retry", map[string]interface{}{
					"operation":     "retry_delay",
					"step_id":       step.StepID,
					"agent_name":    step.AgentName,
					"attempt":       attempt,
					"delay_seconds": retryDelay.Seconds(),
				})
			}
			time.Sleep(retryDelay)
		}
	}

	result.EndTime = time.Now()
	result.Duration = time.Since(startTime)

	// Add span event for step completion
	telemetry.AddSpanEvent(ctx, "step_execution_completed",
		attribute.String("step_id", step.StepID),
		attribute.String("agent_name", step.AgentName),
		attribute.Bool("success", result.Success),
		attribute.Int64("duration_ms", result.Duration.Milliseconds()),
	)

	// Record error on span if step failed
	if !result.Success && result.Error != "" {
		telemetry.RecordSpanError(ctx, fmt.Errorf("step %s failed: %s", step.StepID, result.Error))
	}

	if e.logger != nil {
		e.logger.Debug("Step execution completed", map[string]interface{}{
			"operation":   "step_execution_complete",
			"step_id":     step.StepID,
			"agent_name":  step.AgentName,
			"success":     result.Success,
			"duration_ms": result.Duration.Milliseconds(),
			"error":       result.Error,
			"trace_id":    tc.TraceID,
			"span_id":     tc.SpanID,
		})
	}

	return result
}

// findAgentByName finds agent info by name
func (e *SmartExecutor) findAgentByName(name string) *AgentInfo {
	agents := e.catalog.GetAgents()
	for _, agent := range agents {
		if agent.Registration.Name == name {
			return agent
		}
	}
	return nil
}

// findCapabilityEndpoint finds the endpoint for a capability
func (e *SmartExecutor) findCapabilityEndpoint(agent *AgentInfo, capabilityName string) string {
	for _, cap := range agent.Capabilities {
		if cap.Name == capabilityName {
			return cap.Endpoint
		}
	}
	// Default endpoint if not specified
	return fmt.Sprintf("/api/%s", capabilityName)
}

// findCapabilitySchema returns the capability schema for type coercion.
// This is used by Layer 2 (Schema-Based Type Coercion) to get parameter type information.
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
// This is the core of Layer 2 (Schema-Based Type Coercion) which fixes LLM-generated
// parameters that have incorrect types (e.g., "35.6" instead of 35.6).
// Returns the coerced parameters and a log of coercions performed for debugging.
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
// Supported type conversions:
//   - string -> float64/number: "35.6" -> 35.6
//   - string -> int64/integer: "42" -> 42
//   - string -> bool/boolean: "true" -> true
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

// isTypeRelatedError detects errors that can be corrected by requesting the LLM to fix parameters.
// This is used by Layer 3 (Validation Feedback) to detect:
// - Type errors (e.g., string instead of number)
// - Validation errors (e.g., "must be greater than 0", "is required")
// - Value constraint errors (e.g., "out of range", "invalid format")
// When these errors are detected, the orchestrator will ask the AI to analyze
// the error and generate corrected parameters.
func isTypeRelatedError(err error, responseBody string) bool {
	// Type-related error patterns (original Layer 3)
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
		"expected integer",
		"expected float",
		"invalid value",
	}

	// Validation error patterns (enhanced - catches business logic validation)
	validationErrorPatterns := []string{
		"must be greater than",
		"must be less than",
		"must be positive",
		"must be non-negative",
		"must be at least",
		"must be at most",
		"is required",
		"missing required",
		"cannot be empty",
		"cannot be zero",
		"cannot be null",
		"cannot be negative",
		"invalid format",
		"out of range",
		"invalid_request", // Common API error code
		"bad request",
		"validation failed",
		"constraint violation",
	}

	errStr := strings.ToLower(err.Error() + " " + responseBody)

	// Check type error patterns
	for _, pattern := range typeErrorPatterns {
		if strings.Contains(errStr, strings.ToLower(pattern)) {
			return true
		}
	}

	// Check validation error patterns
	for _, pattern := range validationErrorPatterns {
		if strings.Contains(errStr, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// callAgent makes an HTTP call to an agent
// This method sends a POST request with JSON parameters to the specified agent endpoint
// and returns the JSON response as a string
func (e *SmartExecutor) callAgent(ctx context.Context, url string, parameters map[string]interface{}) (string, error) {
	// Prepare request body
	body, err := json.Marshal(parameters)
	if err != nil {
		return "", fmt.Errorf("failed to marshal parameters: %w", err)
	}

	// Log request details at DEBUG level
	if e.logger != nil {
		e.logger.Debug("HTTP request to agent", map[string]interface{}{
			"operation":   "agent_http_request",
			"url":         url,
			"method":      "POST",
			"body_length": len(body),
			"parameters":  parameters,
		})
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't fail the operation
			if e.logger != nil {
				e.logger.Warn("Error closing response body", map[string]interface{}{
					"url":   url,
					"error": closeErr.Error(),
				})
			}
		}
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("agent returned status %d", resp.StatusCode)
	}

	// Read response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert back to string for storage
	responseBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	responseStr := string(responseBytes)

	// Log response content at DEBUG level
	if e.logger != nil {
		e.logger.Debug("HTTP response from agent", map[string]interface{}{
			"operation":       "agent_http_response",
			"url":             url,
			"status_code":     resp.StatusCode,
			"response_length": len(responseStr),
			"response":        result, // Log parsed response for readability
		})
	}

	return responseStr, nil
}

// callAgentWithBody makes an HTTP call and returns both response and error body.
// This is needed for Layer 3 validation feedback to detect type errors from the
// response body when the tool returns a 4xx error with details.
// Returns: (successResponse, errorResponseBody, error)
func (e *SmartExecutor) callAgentWithBody(ctx context.Context, url string, parameters map[string]interface{}) (string, string, error) {
	// Prepare request body
	body, err := json.Marshal(parameters)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal parameters: %w", err)
	}

	// Log request details at DEBUG level
	if e.logger != nil {
		e.logger.Debug("HTTP request to agent (with body tracking)", map[string]interface{}{
			"operation":   "agent_http_request_with_body",
			"url":         url,
			"method":      "POST",
			"body_length": len(body),
		})
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
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			if e.logger != nil {
				e.logger.Warn("Error closing response body", map[string]interface{}{
					"url":   url,
					"error": closeErr.Error(),
				})
			}
		}
	}()

	// Read response body (always, even on error - needed for type error detection)
	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", "", fmt.Errorf("failed to read response body: %w", readErr)
	}
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

	responseBytes, err := json.Marshal(result)
	if err != nil {
		return "", respBodyStr, fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(responseBytes), respBodyStr, nil
}

// ExecuteStep executes a single routing step (interface method)
func (e *SmartExecutor) ExecuteStep(ctx context.Context, step RoutingStep) (*StepResult, error) {
	result := e.executeStep(ctx, step)
	if !result.Success {
		return nil, fmt.Errorf("%s", result.Error)
	}
	return &result, nil
}

// SetMaxConcurrency sets the maximum number of parallel executions
func (e *SmartExecutor) SetMaxConcurrency(max int) {
	e.maxConcurrency = max
	// Recreate semaphore with new size
	e.semaphore = make(chan struct{}, max)
}

// SimpleExecutor is kept for backward compatibility
type SimpleExecutor struct {
	*SmartExecutor
}

// NewExecutor creates a new executor (backward compatibility)
func NewExecutor() *SimpleExecutor {
	// Create a traced HTTP client for distributed tracing support
	tracedClient := telemetry.NewTracedHTTPClient(nil)
	tracedClient.Timeout = 30 * time.Second

	return &SimpleExecutor{
		SmartExecutor: &SmartExecutor{
			maxConcurrency: 5,
			semaphore:      make(chan struct{}, 5),
			httpClient:     tracedClient,
		},
	}
}
