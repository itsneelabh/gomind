package orchestration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// SmartExecutor handles intelligent execution of routing plans
type SmartExecutor struct {
	catalog        *AgentCatalog
	httpClient     *http.Client
	maxConcurrency int
	semaphore      chan struct{}

	// Observability (follows framework design principles)
	logger core.Logger // For structured logging
}

// NewSmartExecutor creates a new smart executor
func NewSmartExecutor(catalog *AgentCatalog) *SmartExecutor {
	maxConcurrency := 5
	return &SmartExecutor{
		catalog:        catalog,
		maxConcurrency: maxConcurrency,
		semaphore:      make(chan struct{}, maxConcurrency),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
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

	if e.logger != nil {
		e.logger.Debug("Starting plan execution", map[string]interface{}{
			"operation":       "execute_plan",
			"plan_id":         plan.PlanID,
			"step_count":      len(plan.Steps),
			"max_concurrency": e.maxConcurrency,
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

						// Structured logging placeholder - enable for debugging
						// When proper logging is integrated, replace this with logger calls
						if false { // Disabled in production, enable for debugging
							fmt.Printf("PANIC|step=%s|agent=%s|error=%v|stack=%s\n",
								s.StepID, s.AgentName, r, stackTrace)
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
	if e.logger != nil {
		e.logger.Info("Plan execution finished", map[string]interface{}{
			"operation":      "execute_plan_complete",
			"plan_id":        plan.PlanID,
			"success":        result.Success,
			"failed_steps":   failedSteps,
			"total_steps":    len(plan.Steps),
			"duration_ms":    result.TotalDuration.Milliseconds(),
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
		for _, dep := range step.DependsOn {
			if !executed[dep] {
				allDepsReady = false
				break
			}
			// Check if dependency was successful
			if result, ok := results[dep]; ok && !result.Success {
				// Skip steps whose dependencies failed
				allDepsReady = false
				break
			}
		}

		if allDepsReady {
			ready = append(ready, step)
		}
	}

	return ready
}

// buildStepContext creates context with dependency results
func (e *SmartExecutor) buildStepContext(ctx context.Context, step RoutingStep, results map[string]*StepResult) context.Context {
	// Add dependency results to context for reference
	type contextKey string
	const dependencyKey contextKey = "dependencies"

	deps := make(map[string]interface{})
	for _, depID := range step.DependsOn {
		if result, ok := results[depID]; ok {
			deps[depID] = result.Response
		}
	}

	return context.WithValue(ctx, dependencyKey, deps)
}

// executeStep executes a single routing step
func (e *SmartExecutor) executeStep(ctx context.Context, step RoutingStep) StepResult {
	startTime := time.Now()

	if e.logger != nil {
		e.logger.Debug("Starting step execution", map[string]interface{}{
			"operation":  "step_execution_start",
			"step_id":    step.StepID,
			"agent_name": step.AgentName,
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
		if e.logger != nil {
			e.logger.Error("Agent not found in catalog", map[string]interface{}{
				"operation":  "agent_discovery",
				"step_id":    step.StepID,
				"agent_name": step.AgentName,
			})
		}
		result.Success = false
		result.Error = fmt.Sprintf("agent %s not found in catalog", step.AgentName)
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

	// Find the capability endpoint
	endpoint := e.findCapabilityEndpoint(agentInfo, capability)
	if endpoint == "" {
		if e.logger != nil {
			e.logger.Error("Capability endpoint not found", map[string]interface{}{
				"operation":   "capability_resolution",
				"step_id":     step.StepID,
				"agent_name":  step.AgentName,
				"capability":  capability,
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
		result.Error = fmt.Sprintf("capability %s not found for agent %s", capability, step.AgentName)
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime)
		return result
	}

	// Build the request URL
	url := fmt.Sprintf("http://%s:%d%s",
		agentInfo.Registration.Address,
		agentInfo.Registration.Port,
		endpoint)

	// Execute with retry logic
	maxAttempts := 3
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
			})
		}

		// Make the HTTP request
		response, err := e.callAgent(ctx, url, parameters)
		if err == nil {
			if e.logger != nil {
				e.logger.Debug("Agent HTTP call successful", map[string]interface{}{
					"operation":      "agent_http_response",
					"step_id":        step.StepID,
					"agent_name":     step.AgentName,
					"attempt":        attempt,
					"response_length": len(response),
				})
			}
			result.Success = true
			result.Response = response
			break
		}

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
					"operation":    "retry_delay",
					"step_id":      step.StepID,
					"agent_name":   step.AgentName,
					"attempt":      attempt,
					"delay_seconds": retryDelay.Seconds(),
				})
			}
			time.Sleep(retryDelay)
		}
	}

	result.EndTime = time.Now()
	result.Duration = time.Since(startTime)

	if e.logger != nil {
		e.logger.Debug("Step execution completed", map[string]interface{}{
			"operation":     "step_execution_complete",
			"step_id":       step.StepID,
			"agent_name":    step.AgentName,
			"success":       result.Success,
			"duration_ms":   result.Duration.Milliseconds(),
			"error":         result.Error,
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

// callAgent makes an HTTP call to an agent
// This method sends a POST request with JSON parameters to the specified agent endpoint
// and returns the JSON response as a string
func (e *SmartExecutor) callAgent(ctx context.Context, url string, parameters map[string]interface{}) (string, error) {
	// Prepare request body
	body, err := json.Marshal(parameters)
	if err != nil {
		return "", fmt.Errorf("failed to marshal parameters: %w", err)
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
			fmt.Printf("Error closing response body: %v\n", closeErr)
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

	return string(responseBytes), nil
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
	return &SimpleExecutor{
		SmartExecutor: &SmartExecutor{
			maxConcurrency: 5,
			semaphore:      make(chan struct{}, 5),
			httpClient: &http.Client{
				Timeout: 30 * time.Second,
			},
		},
	}
}
