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
)

// SmartExecutor handles intelligent execution of routing plans
type SmartExecutor struct {
	catalog        *AgentCatalog
	httpClient     *http.Client
	maxConcurrency int
	semaphore      chan struct{}
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

// Execute runs a routing plan and collects agent responses.
// This method orchestrates the execution of all steps in the plan,
// respecting dependencies and running steps in parallel where possible.
// It includes panic recovery for each step to ensure resilience.
func (e *SmartExecutor) Execute(ctx context.Context, plan *RoutingPlan) (*ExecutionResult, error) {
	startTime := time.Now()

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

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	result.TotalDuration = time.Since(startTime)
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
		result.Success = false
		result.Error = fmt.Sprintf("agent %s not found in catalog", step.AgentName)
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime)
		return result
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

		// Make the HTTP request
		response, err := e.callAgent(ctx, url, parameters)
		if err == nil {
			result.Success = true
			result.Response = response
			break
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
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	result.EndTime = time.Now()
	result.Duration = time.Since(startTime)

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
