package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/pkg/communication"
	"github.com/itsneelabh/gomind/pkg/logger"
	"github.com/itsneelabh/gomind/pkg/routing"
)

// PlanExecutor implements the Executor interface
type PlanExecutor struct {
	communicator   communication.AgentCommunicator
	logger         logger.Logger
	options        *ExecutionOptions
	maxConcurrency int
	semaphore      chan struct{}
	
	// Metrics
	totalAgentCalls   int64
	failedAgentCalls  int64
	metricsMutex      sync.Mutex
}

// NewPlanExecutor creates a new plan executor
func NewPlanExecutor(
	communicator communication.AgentCommunicator,
	logger logger.Logger,
	options *ExecutionOptions,
) *PlanExecutor {
	maxConcurrency := options.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 5
	}
	
	return &PlanExecutor{
		communicator:   communicator,
		logger:         logger,
		options:        options,
		maxConcurrency: maxConcurrency,
		semaphore:      make(chan struct{}, maxConcurrency),
	}
}

// Execute runs a routing plan and collects agent responses
func (e *PlanExecutor) Execute(ctx context.Context, plan *routing.RoutingPlan) (*ExecutionResult, error) {
	startTime := time.Now()
	
	e.logger.Info("Executing routing plan", map[string]interface{}{
		"plan_id":     plan.ID,
		"steps_count": len(plan.Steps),
		"mode":        plan.Mode,
	})
	
	// Group steps by execution order
	stepGroups := e.groupStepsByOrder(plan.Steps)
	
	// Execute step groups in order
	results := make([]StepResult, 0, len(plan.Steps))
	stepResults := make(map[string]StepResult)
	resultsMutex := sync.Mutex{}
	
	for order, group := range stepGroups {
		e.logger.Debug("Executing step group", map[string]interface{}{
			"order":       order,
			"group_size":  len(group),
		})
		
		// Check if dependencies are satisfied
		if !e.checkDependencies(group, stepResults) {
			e.logger.Error("Dependencies not satisfied for step group", map[string]interface{}{
				"order": order,
			})
			continue
		}
		
		// Execute steps in parallel if they are marked as parallel
		if e.canExecuteInParallel(group) {
			groupResults := e.executeParallel(ctx, group)
			resultsMutex.Lock()
			for _, result := range groupResults {
				stepResults[result.StepID] = result
				results = append(results, result)
			}
			resultsMutex.Unlock()
		} else {
			// Execute sequentially
			for _, step := range group {
				result, err := e.ExecuteStep(ctx, step)
				if err != nil && step.Required {
					e.logger.Error("Required step failed", map[string]interface{}{
						"step_id":    step.StepID,
						"agent_name": step.AgentName,
						"error":      err.Error(),
					})
					// Stop execution if a required step fails
					return &ExecutionResult{
						PlanID:        plan.ID,
						Steps:         results,
						Success:       false,
						TotalDuration: time.Since(startTime),
					}, err
				}
				resultsMutex.Lock()
				stepResults[result.StepID] = *result
				results = append(results, *result)
				resultsMutex.Unlock()
			}
		}
	}
	
	// Determine overall success
	success := true
	for _, result := range results {
		if !result.Success {
			// Check if this was a required step
			for _, step := range plan.Steps {
				if step.StepID == result.StepID && step.Required {
					success = false
					break
				}
			}
		}
	}
	
	executionResult := &ExecutionResult{
		PlanID:        plan.ID,
		Steps:         results,
		Success:       success,
		TotalDuration: time.Since(startTime),
		Metadata: map[string]interface{}{
			"total_agent_calls":  e.totalAgentCalls,
			"failed_agent_calls": e.failedAgentCalls,
		},
	}
	
	e.logger.Info("Completed plan execution", map[string]interface{}{
		"plan_id":       plan.ID,
		"success":       success,
		"duration":      time.Since(startTime),
		"steps_executed": len(results),
	})
	
	return executionResult, nil
}

// ExecuteStep executes a single routing step
func (e *PlanExecutor) ExecuteStep(ctx context.Context, step routing.RoutingStep) (*StepResult, error) {
	startTime := time.Now()
	
	// Apply step timeout if configured
	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	} else if e.options.StepTimeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, e.options.StepTimeout)
		defer cancel()
	}
	
	e.logger.Debug("Executing step", map[string]interface{}{
		"step_id":     step.StepID,
		"agent_name":  step.AgentName,
		"namespace":   step.Namespace,
		"instruction": step.Instruction,
	})
	
	// Build agent identifier
	agentIdentifier := step.AgentName
	if step.Namespace != "" {
		agentIdentifier = fmt.Sprintf("%s.%s", step.AgentName, step.Namespace)
	}
	
	// Execute with retry logic
	var response string
	var lastErr error
	attempts := 0
	maxAttempts := 1
	
	if step.RetryPolicy != nil {
		maxAttempts = step.RetryPolicy.MaxAttempts
	} else if e.options.RetryAttempts > 0 {
		maxAttempts = e.options.RetryAttempts
	}
	
	for attempts < maxAttempts {
		attempts++
		
		e.incrementAgentCalls()
		
		// Call the agent
		response, lastErr = e.communicator.CallAgent(stepCtx, agentIdentifier, step.Instruction)
		
		if lastErr == nil {
			// Success
			e.logger.Debug("Step executed successfully", map[string]interface{}{
				"step_id":  step.StepID,
				"attempts": attempts,
			})
			
			return &StepResult{
				StepID:      step.StepID,
				AgentName:   step.AgentName,
				Namespace:   step.Namespace,
				Instruction: step.Instruction,
				Response:    response,
				Success:     true,
				Duration:    time.Since(startTime),
				Attempts:    attempts,
				StartTime:   startTime,
				EndTime:     time.Now(),
			}, nil
		}
		
		e.incrementFailedAgentCalls()
		
		// Check if we should retry
		if attempts < maxAttempts {
			retryDelay := e.options.RetryDelay
			if step.RetryPolicy != nil && step.RetryPolicy.Delay > 0 {
				retryDelay = step.RetryPolicy.Delay
			}
			
			if step.RetryPolicy != nil && step.RetryPolicy.BackoffType == "exponential" {
				retryDelay = retryDelay * time.Duration(attempts)
			}
			
			e.logger.Debug("Retrying step", map[string]interface{}{
				"step_id":     step.StepID,
				"attempt":     attempts + 1,
				"retry_delay": retryDelay,
			})
			
			select {
			case <-time.After(retryDelay):
				// Continue to next attempt
			case <-stepCtx.Done():
				// Context cancelled
				lastErr = stepCtx.Err()
				break
			}
		}
	}
	
	// All attempts failed
	e.logger.Error("Step failed after retries", map[string]interface{}{
		"step_id":  step.StepID,
		"attempts": attempts,
		"error":    lastErr.Error(),
	})
	
	return &StepResult{
		StepID:      step.StepID,
		AgentName:   step.AgentName,
		Namespace:   step.Namespace,
		Instruction: step.Instruction,
		Response:    "",
		Success:     false,
		Error:       lastErr.Error(),
		Duration:    time.Since(startTime),
		Attempts:    attempts,
		StartTime:   startTime,
		EndTime:     time.Now(),
	}, &ExecutionError{
		StepID:  step.StepID,
		Agent:   agentIdentifier,
		Message: lastErr.Error(),
		Code:    ErrMaxRetriesReached,
		Retries: attempts,
	}
}

// SetMaxConcurrency sets the maximum number of parallel executions
func (e *PlanExecutor) SetMaxConcurrency(max int) {
	if max <= 0 {
		max = 1
	}
	e.maxConcurrency = max
	e.semaphore = make(chan struct{}, max)
}

// groupStepsByOrder groups steps by their execution order
func (e *PlanExecutor) groupStepsByOrder(steps []routing.RoutingStep) map[int][]routing.RoutingStep {
	groups := make(map[int][]routing.RoutingStep)
	
	for _, step := range steps {
		groups[step.Order] = append(groups[step.Order], step)
	}
	
	return groups
}

// checkDependencies verifies that all dependencies for a step group are satisfied
func (e *PlanExecutor) checkDependencies(steps []routing.RoutingStep, completed map[string]StepResult) bool {
	for _, step := range steps {
		for range step.DependsOn {
			// Check if any step with the required order has completed successfully
			// This is a simplified check - in production, we'd match by order
			found := false
			for _, result := range completed {
				if result.Success {
					found = true
					break
				}
			}
			if !found && len(completed) > 0 {
				return false
			}
		}
	}
	return true
}

// canExecuteInParallel checks if steps in a group can be executed in parallel
func (e *PlanExecutor) canExecuteInParallel(steps []routing.RoutingStep) bool {
	if len(steps) <= 1 {
		return false
	}
	
	for _, step := range steps {
		if !step.Parallel {
			return false
		}
	}
	
	return true
}

// executeParallel executes multiple steps in parallel
func (e *PlanExecutor) executeParallel(ctx context.Context, steps []routing.RoutingStep) []StepResult {
	results := make([]StepResult, 0, len(steps))
	resultsChan := make(chan StepResult, len(steps))
	var wg sync.WaitGroup
	
	for _, step := range steps {
		wg.Add(1)
		
		// Copy step for goroutine
		stepCopy := step
		
		go func() {
			defer wg.Done()
			
			// Acquire semaphore
			e.semaphore <- struct{}{}
			defer func() { <-e.semaphore }()
			
			result, err := e.ExecuteStep(ctx, stepCopy)
			if err != nil {
				e.logger.Error("Parallel step execution failed", map[string]interface{}{
					"step_id": stepCopy.StepID,
					"error":   err.Error(),
				})
			}
			resultsChan <- *result
		}()
	}
	
	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()
	
	// Collect results
	for result := range resultsChan {
		results = append(results, result)
	}
	
	return results
}

// Metrics helpers
func (e *PlanExecutor) incrementAgentCalls() {
	e.metricsMutex.Lock()
	defer e.metricsMutex.Unlock()
	e.totalAgentCalls++
}

func (e *PlanExecutor) incrementFailedAgentCalls() {
	e.metricsMutex.Lock()
	defer e.metricsMutex.Unlock()
	e.failedAgentCalls++
}

func (e *PlanExecutor) GetMetrics() map[string]int64 {
	e.metricsMutex.Lock()
	defer e.metricsMutex.Unlock()
	
	return map[string]int64{
		"total_agent_calls":  e.totalAgentCalls,
		"failed_agent_calls": e.failedAgentCalls,
	}
}