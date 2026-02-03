package orchestration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	// planContextKey stores the current routing plan for HITL checks
	planContextKey executorContextKey = "routing_plan"
	// resolvedParamsKey stores resolved parameters for HITL checkpoint visibility
	resolvedParamsKey executorContextKey = "resolved_params"
	// preResolvedParamsKey stores parameters from checkpoint during resume
	// These parameters were approved by the user and should be used directly
	preResolvedParamsKey executorContextKey = "pre_resolved_params"
	// preResolvedStepIDKey stores the step ID that pre-resolved params are for
	// Only the step with matching ID should use the pre-resolved params
	preResolvedStepIDKey executorContextKey = "pre_resolved_step_id"
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

	// Hybrid Parameter Resolution (Phase 4: Auto-wiring + Micro-resolution)
	// When enabled, uses intelligent parameter binding instead of brittle template substitution
	hybridResolver      *HybridResolver // Hybrid resolver for parameter binding
	useHybridResolution bool            // Enable hybrid resolution (default: true when resolver is set)

	// Layer 3: LLM-based Error Analysis
	// When enabled, uses LLM to analyze errors and determine if they can be fixed
	// with different parameters (replaces the need for tools to set Retryable flags)
	errorAnalyzer *ErrorAnalyzer // LLM-based error analyzer

	// Layer 4: Contextual Re-Resolution for Semantic Retry
	// When ErrorAnalyzer says "cannot fix" but source data exists,
	// this component can compute derived values using full context.
	contextualReResolver             *ContextualReResolver
	maxSemanticRetries               int  // Default: 2
	semanticRetryForIndependentSteps bool // Default: true - enable Layer 4 for steps without dependencies

	// Step completion callback for async progress reporting (v1 addition)
	// Called after each step completes to enable per-tool progress reporting.
	// See notes/ASYNC_TASK_DESIGN.md Phase 6 for details.
	onStepComplete StepCompleteCallback

	// Retry configuration
	maxAttempts int // Maximum number of retry attempts (default: 2)

	// HITL (Human-in-the-Loop) support
	// When set, enables human oversight before/after step execution.
	//
	// Design note: Executor checks controller != nil, not config.HITL.Enabled.
	// The presence of a controller indicates HITL is active. The orchestrator
	// is responsible for only setting the controller when HITL is enabled in config.
	// This avoids coupling executor to OrchestratorConfig.
	interruptController InterruptController
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
		// Retry defaults
		maxAttempts: 2, // Up to 2 retry attempts (default)
	}
}

// SetMaxAttempts configures the maximum number of retry attempts for step execution.
// Set to 1 to disable retries (useful for tests). Default is 2.
func (e *SmartExecutor) SetMaxAttempts(max int) {
	if max < 1 {
		max = 1
	}
	e.maxAttempts = max
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

// SetHybridResolver configures the hybrid parameter resolver.
// When set, the executor uses intelligent auto-wiring and optional LLM micro-resolution
// instead of brittle template substitution for parameter binding between steps.
// This significantly improves reliability of multi-step workflows.
func (e *SmartExecutor) SetHybridResolver(resolver *HybridResolver) {
	e.hybridResolver = resolver
	e.useHybridResolution = resolver != nil
	if e.logger != nil && resolver != nil {
		e.logger.Info("Hybrid parameter resolution enabled", map[string]interface{}{
			"operation": "hybrid_resolver_configured",
		})
	}
}

// EnableHybridResolution enables or disables hybrid parameter resolution.
// Use this to temporarily disable hybrid resolution without removing the resolver.
func (e *SmartExecutor) EnableHybridResolution(enabled bool) {
	e.useHybridResolution = enabled && e.hybridResolver != nil
}

// SetErrorAnalyzer configures the LLM-based error analyzer.
// When set, the executor uses LLM to analyze errors and determine if they can be
// fixed with different parameters. This replaces the need for tools to set Retryable flags.
// See PARAMETER_BINDING_FIX.md for the complete design rationale.
func (e *SmartExecutor) SetErrorAnalyzer(analyzer *ErrorAnalyzer) {
	e.errorAnalyzer = analyzer
	if e.logger != nil && analyzer != nil {
		e.logger.Info("LLM error analyzer enabled", map[string]interface{}{
			"operation": "error_analyzer_configured",
		})
	}
}

// SetLogger sets the logger provider (follows framework design principles)
// The component is always set to "framework/orchestration" to ensure proper log attribution
// regardless of which agent or tool is using the orchestration module.
func (e *SmartExecutor) SetLogger(logger core.Logger) {
	if logger == nil {
		e.logger = &core.NoOpLogger{}
	} else {
		if cal, ok := logger.(core.ComponentAwareLogger); ok {
			e.logger = cal.WithComponent("framework/orchestration")
		} else {
			e.logger = logger
		}
	}
	// Propagate logger to hybrid resolver if configured (it will apply its own WithComponent)
	if e.hybridResolver != nil {
		e.hybridResolver.SetLogger(logger)
	}
	// Propagate logger to error analyzer if configured
	if e.errorAnalyzer != nil {
		e.errorAnalyzer.SetLogger(logger)
	}
	// Propagate logger to contextual re-resolver if configured
	if e.contextualReResolver != nil {
		e.contextualReResolver.SetLogger(logger)
	}
}

// SetContextualReResolver configures Layer 4 semantic retry.
// When set, the executor can re-resolve parameters after failures using full context.
// This complements ErrorAnalyzer by providing source data for computation.
func (e *SmartExecutor) SetContextualReResolver(resolver *ContextualReResolver) {
	e.contextualReResolver = resolver
	if e.logger != nil && resolver != nil {
		e.logger.Info("Contextual re-resolver enabled for semantic retries", map[string]interface{}{
			"operation": "contextual_re_resolver_configured",
		})
	}
}

// SetMaxSemanticRetries sets the maximum number of semantic retry attempts.
// Default is 2. These retries are separate from regular retry attempts.
func (e *SmartExecutor) SetMaxSemanticRetries(max int) {
	if max < 0 {
		max = 0
	}
	e.maxSemanticRetries = max
}

// SetSemanticRetryForIndependentSteps enables/disables semantic retry for steps
// without dependencies. When true (default), Layer 4 runs even when source data
// from previous steps is empty. Controlled by GOMIND_SEMANTIC_RETRY_INDEPENDENT_STEPS.
func (e *SmartExecutor) SetSemanticRetryForIndependentSteps(enabled bool) {
	e.semanticRetryForIndependentSteps = enabled
}

// SetOnStepComplete sets the callback for step completion notifications.
// Used by async task handlers to receive per-step progress updates.
// The callback is invoked after each step completes (success or failure).
// See notes/ASYNC_TASK_DESIGN.md Phase 6 for usage details.
func (e *SmartExecutor) SetOnStepComplete(cb StepCompleteCallback) {
	e.onStepComplete = cb
}

// SetInterruptController sets the HITL interrupt controller.
// When set, enables human oversight before/after step execution.
func (e *SmartExecutor) SetInterruptController(controller InterruptController) {
	e.interruptController = controller
}

// safeInvokeStepCallback invokes a step callback with panic protection.
// If the callback panics, the panic is recovered and logged, preventing
// user callback errors from crashing the executor goroutine.
func (e *SmartExecutor) safeInvokeStepCallback(cb StepCompleteCallback, stepIndex, totalSteps int, step RoutingStep, result StepResult) {
	if cb == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			if e.logger != nil {
				e.logger.Error("Step callback panicked", map[string]interface{}{
					"operation":   "step_callback",
					"step_id":     step.StepID,
					"step_index":  stepIndex,
					"total_steps": totalSteps,
					"panic":       fmt.Sprintf("%v", r),
				})
			}
			// Continue execution - don't let callback panic affect the executor
		}
	}()

	cb(stepIndex, totalSteps, step, result)
}

// collectSourceDataFromDependencies converts step results to a flat source data map.
// This is the same logic used by HybridResolver.collectSourceData.
// Returns map[string]interface{} suitable for LLM context.
func (e *SmartExecutor) collectSourceDataFromDependencies(ctx context.Context, dependsOn []string) map[string]interface{} {
	sourceData := make(map[string]interface{})

	// Get step results from context (stored by buildStepContext)
	stepResults := e.collectDependencyResults(ctx, dependsOn)

	// Convert each step result to flat key-value pairs
	for _, result := range stepResults {
		if result == nil || result.Response == "" || !result.Success {
			continue
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result.Response), &parsed); err != nil {
			if e.logger != nil {
				e.logger.WarnWithContext(ctx, "Failed to parse step response for source data", map[string]interface{}{
					"step_id": result.StepID,
					"error":   err.Error(),
				})
			}
			continue
		}

		// Merge into sourceData (later steps may override earlier for same keys)
		for k, v := range parsed {
			sourceData[k] = v
		}
	}

	return sourceData
}

// Execute runs a routing plan and collects agent responses.
// This method orchestrates the execution of all steps in the plan,
// respecting dependencies and running steps in parallel where possible.
// It includes panic recovery for each step to ensure resilience.
func (e *SmartExecutor) Execute(ctx context.Context, plan *RoutingPlan) (*ExecutionResult, error) {
	startTime := time.Now()

	// Add span event for plan execution start
	telemetry.AddSpanEvent(ctx, "plan_execution_started",
		attribute.String("plan_id", plan.PlanID),
		attribute.Int("step_count", len(plan.Steps)),
	)

	if e.logger != nil {
		e.logger.DebugWithContext(ctx, "Starting plan execution", map[string]interface{}{
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

	// Pre-populate with completed steps from HITL checkpoint (resume flow)
	// This allows the executor to skip steps that were already completed
	// before the HITL checkpoint was created.
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
		if e.logger != nil {
			e.logger.InfoWithContext(ctx, "Executor resuming with completed steps from checkpoint", map[string]interface{}{
				"operation":       "hitl_resume_executor",
				"plan_id":         plan.PlanID,
				"completed_count": len(completedSteps),
				"remaining_count": len(plan.Steps) - len(completedSteps),
			})
		}
	}

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
					skippedStepIndex := len(result.Steps) - 1
					executed[step.StepID] = true
					result.Success = false
					hasSkipped = true

					// Invoke step completion callbacks for skipped steps too.
					// This ensures UI receives step events for all steps,
					// keeping the progress panel count consistent with AgentsInvolved.
					e.safeInvokeStepCallback(e.onStepComplete, skippedStepIndex, len(plan.Steps), step, skippedResult)
					if ctxCallback := GetStepCallback(ctx); ctxCallback != nil {
						e.safeInvokeStepCallback(ctxCallback, skippedStepIndex, len(plan.Steps), step, skippedResult)
					}
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
			e.logger.DebugWithContext(ctx, "Executing steps in parallel", map[string]interface{}{
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
						panicStepIndex := len(result.Steps) - 1 // Capture index while holding lock

						resultsMutex.Unlock() // Unlock immediately, no defer

						// Invoke step completion callbacks for panicked steps too.
						// This ensures UI receives step events even for failed steps,
						// keeping the progress panel count consistent with AgentsInvolved.
						e.safeInvokeStepCallback(e.onStepComplete, panicStepIndex, len(plan.Steps), s, panicResult)
						if ctxCallback := GetStepCallback(ctx); ctxCallback != nil {
							e.safeInvokeStepCallback(ctxCallback, panicStepIndex, len(plan.Steps), s, panicResult)
						}
					}
					wg.Done()
				}()

				// Build context for step execution
				// Include plan in context for HITL checks
				stepCtx := context.WithValue(ctx, planContextKey, plan)
				stepCtx = e.buildStepContext(stepCtx, s, stepResults)

				// Execute the step
				stepResult := e.executeStep(stepCtx, s)

				// Store result
				resultsMutex.Lock()
				stepResults[s.StepID] = &stepResult
				result.Steps = append(result.Steps, stepResult)
				executed[s.StepID] = true
				stepIndex := len(result.Steps) - 1 // Capture index while holding lock

				if !stepResult.Success {
					result.Success = false
				}
				resultsMutex.Unlock()

				// Invoke step completion callbacks (outside lock to avoid blocking)
				// This enables async task handlers to report per-tool progress.
				// See notes/ASYNC_TASK_DESIGN.md Phase 6 for details.
				// Check both executor-level and context-level callbacks
				// Callbacks are wrapped with panic protection to prevent user callback
				// panics from crashing the executor goroutine.
				//
				// IMPORTANT: Skip callback for HITL-interrupted steps. When a step is
				// paused for human approval, it hasn't actually "failed" - it's just
				// waiting. Sending a failure callback would confuse the UI.
				isHITLInterrupt := stepResult.Metadata != nil && stepResult.Metadata["hitl_checkpoint"] != nil
				if isHITLInterrupt {
					if e.logger != nil {
						e.logger.DebugWithContext(ctx, "Skipping step callback for HITL-interrupted step", map[string]interface{}{
							"operation":     "hitl_step_callback_skip",
							"step_id":       s.StepID,
							"agent_name":    s.AgentName,
							"checkpoint_id": stepResult.Metadata["hitl_checkpoint_id"],
						})
					}
				} else {
					// Log callback invocation for debugging
					if e.logger != nil {
						hasExecutorCallback := e.onStepComplete != nil
						ctxCallback := GetStepCallback(ctx)
						hasCtxCallback := ctxCallback != nil
						e.logger.DebugWithContext(ctx, "Invoking step callbacks", map[string]interface{}{
							"operation":             "step_callback_invoke",
							"step_id":               s.StepID,
							"agent_name":            s.AgentName,
							"success":               stepResult.Success,
							"has_executor_callback": hasExecutorCallback,
							"has_context_callback":  hasCtxCallback,
						})
					}
					e.safeInvokeStepCallback(e.onStepComplete, stepIndex, len(plan.Steps), s, stepResult)
					// Also check for per-request callback from context
					if ctxCallback := GetStepCallback(ctx); ctxCallback != nil {
						e.safeInvokeStepCallback(ctxCallback, stepIndex, len(plan.Steps), s, stepResult)
					}
				}
			}(step)
		}

		// Wait for this batch to complete
		wg.Wait()

		completedSteps := len(executed)
		totalSteps := len(plan.Steps)
		if e.logger != nil {
			e.logger.DebugWithContext(ctx, "Execution batch completed", map[string]interface{}{
				"operation":        "batch_complete",
				"plan_id":          plan.PlanID,
				"completed_steps":  completedSteps,
				"total_steps":      totalSteps,
				"progress_percent": float64(completedSteps) / float64(totalSteps) * 100,
			})
		}

		// Check for HITL step-level interrupts
		// If any step in the batch was interrupted for human approval, propagate ErrInterrupted
		for _, stepResult := range result.Steps {
			if stepResult.Metadata != nil {
				if checkpoint, ok := stepResult.Metadata["hitl_checkpoint"].(*ExecutionCheckpoint); ok {
					// Collect completed steps (steps that finished successfully before the interrupt)
					completedStepResults := make([]StepResult, 0)
					for _, sr := range result.Steps {
						// Include only successful steps that are not the interrupted step
						if sr.Success && sr.StepID != stepResult.StepID {
							completedStepResults = append(completedStepResults, sr)
						}
					}

					// Update checkpoint with completed steps (if interrupt controller is available)
					if e.interruptController != nil && len(completedStepResults) > 0 {
						if err := e.interruptController.UpdateCheckpointProgress(ctx, checkpoint.CheckpointID, completedStepResults); err != nil {
							if e.logger != nil {
								e.logger.WarnWithContext(ctx, "Failed to update checkpoint progress", map[string]interface{}{
									"operation":       "hitl_update_progress",
									"checkpoint_id":   checkpoint.CheckpointID,
									"completed_steps": len(completedStepResults),
									"error":           err.Error(),
								})
							}
							// Non-fatal: continue with interrupt even if update fails
						} else {
							// Update local checkpoint with completed steps for proper DAG visualization
							// (UpdateCheckpointProgress updates the store, but we need the local checkpoint
							// to have the data so it's included when storeExecutionAsync stores the execution)
							checkpoint.CompletedSteps = completedStepResults
							if checkpoint.StepResults == nil {
								checkpoint.StepResults = make(map[string]*StepResult)
							}
							for i := range completedStepResults {
								step := &completedStepResults[i]
								checkpoint.StepResults[step.StepID] = step
							}
						}
					}

					if e.logger != nil {
						e.logger.InfoWithContext(ctx, "Step-level HITL interrupt detected, returning ErrInterrupted", map[string]interface{}{
							"operation":        "hitl_step_interrupt_propagate",
							"checkpoint_id":    checkpoint.CheckpointID,
							"interrupted_step": stepResult.StepID,
							"completed_steps":  len(completedStepResults),
						})
					}

					// Add span event for step-level HITL propagation (visible in distributed traces)
					telemetry.AddSpanEvent(ctx, "hitl.step_interrupt.propagating",
						attribute.String("checkpoint_id", checkpoint.CheckpointID),
						attribute.String("interrupted_step", stepResult.StepID),
						attribute.String("interrupt_point", string(checkpoint.InterruptPoint)),
						attribute.Int("completed_steps", len(completedStepResults)),
						attribute.Int("total_steps", len(result.Steps)),
					)

					// Set span attributes for HITL context
					telemetry.SetSpanAttributes(ctx,
						attribute.Bool("hitl.interrupted", true),
						attribute.String("hitl.checkpoint_id", checkpoint.CheckpointID),
						attribute.String("hitl.interrupt_point", string(checkpoint.InterruptPoint)),
					)

					return nil, &ErrInterrupted{
						CheckpointID: checkpoint.CheckpointID,
						Checkpoint:   checkpoint,
					}
				}
			}
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
		e.logger.InfoWithContext(ctx, "Plan execution finished", map[string]interface{}{
			"operation":    "execute_plan_complete",
			"plan_id":      plan.PlanID,
			"success":      result.Success,
			"failed_steps": failedSteps,
			"total_steps":  len(plan.Steps),
			"duration_ms":  result.TotalDuration.Milliseconds(),
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
	// Results are wrapped in a "response" key to match template syntax: {{stepId.response.field}}
	// This enables templates like {{step-1.response.data.id}} to resolve correctly
	deps := make(map[string]map[string]interface{})
	for _, depID := range step.DependsOn {
		if result, ok := results[depID]; ok && result.Response != "" {
			// Parse the JSON response to enable field access in templates
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result.Response), &parsed); err != nil {
				// Log parsing failure - this could indicate a non-JSON response from a tool
				if e.logger != nil {
					e.logger.WarnWithContext(ctx, "Failed to parse dependency response as JSON for template interpolation", map[string]interface{}{
						"step_id":      step.StepID,
						"dependency":   depID,
						"error":        err.Error(),
						"response_len": len(result.Response),
					})
				}
				// Continue without this dependency - templates referencing it will be unresolved
			} else {
				// Wrap parsed response in "response" key to match template syntax
				// Template: {{stepId.response.field}} -> deps[stepId]["response"][field]
				deps[depID] = map[string]interface{}{"response": parsed}
			}
		}
	}

	return context.WithValue(ctx, dependencyResultsKey, deps)
}

// collectDependencyResults extracts step results from context for the specified dependency IDs.
// This is used by hybrid parameter resolution to get the actual responses from dependent steps.
func (e *SmartExecutor) collectDependencyResults(ctx context.Context, dependsOn []string) map[string]*StepResult {
	results := make(map[string]*StepResult)

	// Get the raw dependency results from context (stored by buildStepContext)
	depData, ok := ctx.Value(dependencyResultsKey).(map[string]map[string]interface{})
	if !ok || depData == nil {
		return results
	}

	// Convert the wrapped response format back to StepResult format
	// buildStepContext stores: deps[stepID] = {"response": parsedJSON}
	// We need to convert this back for the hybrid resolver
	for _, depID := range dependsOn {
		if stepData, exists := depData[depID]; exists {
			if responseData, hasResponse := stepData["response"].(map[string]interface{}); hasResponse {
				// Marshal the response back to JSON for the StepResult
				responseJSON, err := json.Marshal(responseData)
				if err != nil {
					if e.logger != nil {
						e.logger.WarnWithContext(ctx, "Failed to marshal dependency response", map[string]interface{}{
							"dependency": depID,
							"error":      err.Error(),
						})
					}
					continue
				}
				results[depID] = &StepResult{
					StepID:   depID,
					Success:  true,
					Response: string(responseJSON),
				}
			}
		}
	}

	return results
}

// WithResolvedParams adds resolved parameters to the context for HITL checkpoint visibility.
// This allows the HITL controller to include actual parameter values in the checkpoint,
// so users can see real values (e.g., amount: 15000) instead of templates.
func WithResolvedParams(ctx context.Context, params map[string]interface{}) context.Context {
	return context.WithValue(ctx, resolvedParamsKey, params)
}

// GetResolvedParams retrieves resolved parameters from context.
// Returns nil if no resolved parameters are set.
func GetResolvedParams(ctx context.Context) map[string]interface{} {
	if params, ok := ctx.Value(resolvedParamsKey).(map[string]interface{}); ok {
		return params
	}
	return nil
}

// WithPreResolvedParams adds pre-resolved parameters to the context for HITL resume.
// When resuming from a checkpoint, these are the parameters that were approved by the user.
// The executor will use these directly instead of re-resolving from dependencies.
// IMPORTANT: stepID specifies which step these params are for - only that step should use them.
//
// Usage (in agent/orchestrator resume path):
//
//	if checkpoint.ResolvedParameters != nil && checkpoint.CurrentStep != nil {
//	    ctx = WithPreResolvedParams(ctx, checkpoint.ResolvedParameters, checkpoint.CurrentStep.StepID)
//	}
func WithPreResolvedParams(ctx context.Context, params map[string]interface{}, stepID string) context.Context {
	ctx = context.WithValue(ctx, preResolvedParamsKey, params)
	ctx = context.WithValue(ctx, preResolvedStepIDKey, stepID)
	return ctx
}

// GetPreResolvedParams retrieves pre-resolved parameters from context.
// Returns nil if no pre-resolved parameters are set (not resuming from checkpoint).
func GetPreResolvedParams(ctx context.Context) map[string]interface{} {
	if params, ok := ctx.Value(preResolvedParamsKey).(map[string]interface{}); ok {
		return params
	}
	return nil
}

// GetPreResolvedStepID retrieves the step ID that pre-resolved params are for.
// Returns empty string if not set.
func GetPreResolvedStepID(ctx context.Context) string {
	if stepID, ok := ctx.Value(preResolvedStepIDKey).(string); ok {
		return stepID
	}
	return ""
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

	// INFO: Log the start of template interpolation with available context
	if e.logger != nil {
		availableSteps := getDepResultKeys(depResults)
		templateParams := make([]string, 0)
		for key, value := range params {
			if strVal, ok := value.(string); ok && stepOutputTemplatePattern.MatchString(strVal) {
				templateParams = append(templateParams, key)
			}
		}
		if len(templateParams) > 0 {
			e.logger.Info("Template interpolation starting", map[string]interface{}{
				"operation":            "template_interpolation_start",
				"param_count":          len(params),
				"template_params":      templateParams,
				"available_dep_steps":  availableSteps,
				"available_step_count": len(availableSteps),
			})
		}
	}

	interpolated := make(map[string]interface{})
	for key, value := range params {
		interpolated[key] = e.interpolateValue(value, depResults)
	}

	return interpolated
}

// interpolateValue recursively interpolates templates in values.
// Handles strings, arrays, and nested maps to ensure templates are resolved
// at all levels of the parameter structure.
func (e *SmartExecutor) interpolateValue(value interface{}, depResults map[string]map[string]interface{}) interface{} {
	switch v := value.(type) {
	case string:
		// Direct string - substitute templates
		return e.substituteTemplates(v, depResults)

	case []interface{}:
		// Array - recursively interpolate each element
		result := make([]interface{}, len(v))
		for i, elem := range v {
			result[i] = e.interpolateValue(elem, depResults)
		}
		return result

	case map[string]interface{}:
		// Nested map - recursively interpolate each value
		result := make(map[string]interface{})
		for k, val := range v {
			result[k] = e.interpolateValue(val, depResults)
		}
		return result

	default:
		// Other types (numbers, bools, nil) - return as-is
		return value
	}
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

	// INFO: Log template detection with original template and available dependencies
	if e.logger != nil {
		templateRefs := make([]string, len(matches))
		for i, m := range matches {
			templateRefs[i] = m[0]
		}
		e.logger.Info("Template substitution starting", map[string]interface{}{
			"operation":       "template_substitution_start",
			"original_value":  template,
			"templates_found": templateRefs,
			"template_count":  len(matches),
			"available_steps": getDepResultKeys(depResults),
		})
	}

	// Special case: if the entire string is a single template, preserve the value's type
	// This allows numeric values to remain as numbers instead of being converted to strings
	// Critical for parameters like latitude/longitude that must be numbers, not strings
	if len(matches) == 1 && matches[0][0] == template {
		stepID := matches[0][1]
		fieldPath := matches[0][2]

		// Use case-insensitive lookup for step ID (LLMs sometimes use uppercase)
		stepData, actualStepID, stepExists := findStepDataCaseInsensitive(depResults, stepID)
		if !stepExists {
			// Step not found in dependencies - this is likely a configuration error
			if e.logger != nil {
				e.logger.Warn("TEMPLATE RESOLUTION FAILED: Step not found", map[string]interface{}{
					"operation":         "template_step_not_found",
					"template":          template,
					"requested_step_id": stepID,
					"available_steps":   getDepResultKeys(depResults),
					"resolution_status": "failed",
					"failure_reason":    "step_not_in_dependencies",
					"hint":              "Check if the step is in depends_on array and has completed successfully",
				})
			}
			return template
		}

		// INFO: Log if case normalization was applied
		if actualStepID != stepID && e.logger != nil {
			e.logger.Info("Template step ID normalized (case-insensitive)", map[string]interface{}{
				"operation":          "case_normalization_applied",
				"original_step_id":   stepID,
				"normalized_step_id": actualStepID,
				"template":           template,
			})
		}

		// Normalize field path - add "response." prefix if missing (LLMs sometimes omit it)
		normalizedPath, pathNormalized := normalizeFieldPath(fieldPath)
		if pathNormalized && e.logger != nil {
			e.logger.Info("Template field path normalized (added response prefix)", map[string]interface{}{
				"operation":       "path_normalization_applied",
				"original_path":   fieldPath,
				"normalized_path": normalizedPath,
				"template":        template,
			})
		}

		value := extractFieldValue(stepData, normalizedPath)
		if value == nil {
			// Field not found in step response - provide detailed debugging info
			if e.logger != nil {
				// Get a sample of the step data structure for debugging
				stepDataSample := describeStepDataStructure(stepData)
				e.logger.Warn("TEMPLATE RESOLUTION FAILED: Field not found", map[string]interface{}{
					"operation":           "template_field_not_found",
					"template":            template,
					"step_id":             actualStepID,
					"requested_path":      normalizedPath,
					"available_fields":    getFieldKeys(stepData),
					"step_data_structure": stepDataSample,
					"resolution_status":   "failed",
					"failure_reason":      "field_path_not_found",
					"hint":                "Check if the field path matches the actual response structure",
				})
			}
			return template
		}

		// INFO: Log successful resolution
		if e.logger != nil {
			e.logger.Info("TEMPLATE RESOLVED SUCCESSFULLY", map[string]interface{}{
				"operation":         "template_resolution_success",
				"template":          template,
				"step_id":           actualStepID,
				"field_path":        normalizedPath,
				"resolved_value":    value,
				"resolved_type":     fmt.Sprintf("%T", value),
				"resolution_status": "success",
			})
		}

		// Validate that the resolved value is a primitive type suitable for HTTP parameters
		switch v := value.(type) {
		case float64, int, int64, string, bool:
			return value // Safe primitive types
		case map[string]interface{}, []interface{}:
			// Complex types - convert to JSON string for safety
			if e.logger != nil {
				e.logger.Info("Template resolved to complex type, converting to JSON", map[string]interface{}{
					"operation":  "complex_type_conversion",
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
	resolvedCount := 0
	failedCount := 0

	for _, match := range matches {
		fullMatch := match[0]
		stepID := match[1]
		fieldPath := match[2]

		// Use case-insensitive lookup for step ID (LLMs sometimes use uppercase)
		stepData, actualStepID, stepExists := findStepDataCaseInsensitive(depResults, stepID)
		if !stepExists {
			failedCount++
			if e.logger != nil {
				e.logger.Warn("TEMPLATE RESOLUTION FAILED: Step not found (multi-template)", map[string]interface{}{
					"operation":         "template_step_not_found",
					"template":          fullMatch,
					"requested_step_id": stepID,
					"available_steps":   getDepResultKeys(depResults),
					"resolution_status": "failed",
				})
			}
			continue // Leave template unresolved
		}

		// Log if case normalization was applied
		if actualStepID != stepID && e.logger != nil {
			e.logger.Info("Template step ID normalized (case-insensitive)", map[string]interface{}{
				"operation":          "case_normalization_applied",
				"original_step_id":   stepID,
				"normalized_step_id": actualStepID,
				"template":           fullMatch,
			})
		}

		// Normalize field path - add "response." prefix if missing
		normalizedPath, pathNormalized := normalizeFieldPath(fieldPath)
		if pathNormalized && e.logger != nil {
			e.logger.Info("Template field path normalized (added response prefix)", map[string]interface{}{
				"operation":       "path_normalization_applied",
				"original_path":   fieldPath,
				"normalized_path": normalizedPath,
				"template":        fullMatch,
			})
		}

		value := extractFieldValue(stepData, normalizedPath)
		if value != nil {
			resolvedCount++
			resolvedStr := fmt.Sprintf("%v", value)
			result = strings.Replace(result, fullMatch, resolvedStr, 1)

			if e.logger != nil {
				e.logger.Info("TEMPLATE RESOLVED SUCCESSFULLY (multi-template)", map[string]interface{}{
					"operation":         "template_resolution_success",
					"template":          fullMatch,
					"step_id":           actualStepID,
					"field_path":        normalizedPath,
					"resolved_value":    resolvedStr,
					"resolution_status": "success",
				})
			}
		} else {
			failedCount++
			if e.logger != nil {
				e.logger.Warn("TEMPLATE RESOLUTION FAILED: Field not found (multi-template)", map[string]interface{}{
					"operation":         "template_field_not_found",
					"template":          fullMatch,
					"step_id":           actualStepID,
					"field_path":        normalizedPath,
					"available_fields":  getFieldKeys(stepData),
					"resolution_status": "failed",
				})
			}
		}
	}

	// INFO: Log final substitution summary
	if e.logger != nil {
		e.logger.Info("Template substitution completed", map[string]interface{}{
			"operation":       "template_substitution_complete",
			"original_value":  template,
			"final_value":     result,
			"total_templates": len(matches),
			"resolved_count":  resolvedCount,
			"failed_count":    failedCount,
			"all_resolved":    failedCount == 0,
		})
	}

	return result
}

// resolveUnresolvedTemplatesWithLLM resolves parameters that still contain unresolved {{...}} templates
// using LLM semantic inference. This handles cases where template path doesn't exist in source data.
// Example: {{step-2.response.data.country.currency}} fails when geocoding returns country:"France"
// (a string) instead of country:{currency:"EUR"}. The LLM can infer "EUR" from "France".
func (e *SmartExecutor) resolveUnresolvedTemplatesWithLLM(
	ctx context.Context,
	params map[string]interface{},
	depResults map[string]map[string]interface{},
	stepID string,
) map[string]interface{} {
	if params == nil || e.hybridResolver == nil {
		return params
	}

	// Collect all source data from dependencies
	sourceData := make(map[string]interface{})
	for _, result := range depResults {
		for k, v := range result {
			sourceData[k] = v
		}
	}

	// Scan for unresolved templates
	for paramName, paramValue := range params {
		strVal, ok := paramValue.(string)
		if !ok {
			continue
		}

		// Check if value still contains unresolved template pattern
		if !strings.Contains(strVal, "{{") || !strings.Contains(strVal, "}}") {
			continue
		}

		// Found unresolved template - try to resolve semantically
		if e.logger != nil {
			e.logger.Info("SEMANTIC FALLBACK: Attempting LLM resolution for unresolved template", map[string]interface{}{
				"step_id":        stepID,
				"param_name":     paramName,
				"template_value": strVal,
				"source_keys":    getMapKeys(sourceData),
			})
		}

		// Build a semantic hint based on the template pattern
		// Extract what the template was trying to get (e.g., "currency" from "{{step-2.response.data.country.currency}}")
		hint := fmt.Sprintf(
			"The template '%s' tried to extract a value but the path doesn't exist. "+
				"Based on the available data, infer what value was intended for parameter '%s'. "+
				"Look for related information that can be used to determine the correct value.",
			strVal, paramName)

		// Use hybrid resolver's semantic value resolution
		resolved, err := e.hybridResolver.ResolveSemanticValue(ctx, sourceData, paramName, hint, "string", stepID)
		if err != nil {
			if e.logger != nil {
				e.logger.Warn("SEMANTIC FALLBACK FAILED: Could not resolve template", map[string]interface{}{
					"step_id":        stepID,
					"param_name":     paramName,
					"template_value": strVal,
					"error":          err.Error(),
				})
			}
			continue
		}

		// Successfully resolved - update parameter
		params[paramName] = resolved
		if e.logger != nil {
			e.logger.Info("SEMANTIC FALLBACK SUCCESS: Template resolved via LLM", map[string]interface{}{
				"step_id":        stepID,
				"param_name":     paramName,
				"template_value": strVal,
				"resolved_value": resolved,
			})
		}

		// Telemetry for semantic resolution
		telemetry.AddSpanEvent(ctx, "semantic_template_resolution",
			attribute.String("step_id", stepID),
			attribute.String("param_name", paramName),
			attribute.String("original_template", strVal),
		)
		telemetry.Counter("orchestration.semantic_resolution.success",
			"param_name", paramName,
			"module", telemetry.ModuleOrchestration,
		)
	}

	return params
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

// describeStepDataStructure returns a string description of the step data structure for debugging.
// This helps developers understand what fields are actually available when template resolution fails.
// Example output: "response{data{lat,lon,location}}"
func describeStepDataStructure(data map[string]interface{}) string {
	if data == nil {
		return "nil"
	}
	return describeMapStructure(data, 0)
}

// describeMapStructure recursively describes a map structure up to 3 levels deep
func describeMapStructure(m map[string]interface{}, depth int) string {
	if depth > 3 {
		return "..."
	}

	keys := make([]string, 0, len(m))
	for k, v := range m {
		if nested, ok := v.(map[string]interface{}); ok {
			keys = append(keys, k+"{"+describeMapStructure(nested, depth+1)+"}")
		} else {
			keys = append(keys, k)
		}
	}
	return strings.Join(keys, ",")
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

// findStepDataCaseInsensitive finds step data using case-insensitive lookup.
// LLMs sometimes generate uppercase step IDs (e.g., "COUNTRY-INFO" instead of "country-info").
// This function tries exact match first, then falls back to case-insensitive match.
// Returns the step data and the actual key that matched (for logging).
func findStepDataCaseInsensitive(depResults map[string]map[string]interface{}, stepID string) (map[string]interface{}, string, bool) {
	// Try exact match first
	if data, exists := depResults[stepID]; exists {
		return data, stepID, true
	}

	// Try case-insensitive match
	lowerStepID := strings.ToLower(stepID)
	for key, data := range depResults {
		if strings.ToLower(key) == lowerStepID {
			return data, key, true
		}
	}

	return nil, "", false
}

// normalizeFieldPath ensures the field path starts with "response." if it doesn't already.
// LLMs sometimes omit the "response" prefix (e.g., "data.currency" instead of "response.data.currency").
// The executor wraps step results in {"response": ...}, so we need to add the prefix.
// Returns the normalized path and whether normalization was applied.
func normalizeFieldPath(fieldPath string) (string, bool) {
	// If path already starts with "response.", no change needed
	if strings.HasPrefix(strings.ToLower(fieldPath), "response.") {
		return fieldPath, false
	}

	// Add "response." prefix
	return "response." + fieldPath, true
}

// executeStep executes a single routing step
func (e *SmartExecutor) executeStep(ctx context.Context, step RoutingStep) StepResult {
	startTime := time.Now()

	// Add span event for step execution start
	telemetry.AddSpanEvent(ctx, "step_execution_started",
		attribute.String("step_id", step.StepID),
		attribute.String("agent_name", step.AgentName),
	)

	if e.logger != nil {
		e.logger.DebugWithContext(ctx, "Starting step execution", map[string]interface{}{
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

	// =========================================================================
	// PHASE 1: Agent Discovery (before HITL to ensure valid agent)
	// =========================================================================
	// Get agent info from catalog FIRST - no point asking for approval if agent doesn't exist
	agentInfo := e.findAgentByName(step.AgentName)
	if agentInfo == nil {
		err := fmt.Errorf("agent %s not found in catalog", step.AgentName)
		telemetry.RecordSpanError(ctx, err)
		if e.logger != nil {
			e.logger.ErrorWithContext(ctx, "Agent not found in catalog", map[string]interface{}{
				"operation":  "agent_discovery",
				"step_id":    step.StepID,
				"agent_name": step.AgentName,
			})
		}
		result.Success = false
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime)
		return result
	}

	if e.logger != nil {
		e.logger.DebugWithContext(ctx, "Agent discovered successfully", map[string]interface{}{
			"operation":     "agent_discovery",
			"step_id":       step.StepID,
			"agent_name":    step.AgentName,
			"agent_id":      agentInfo.Registration.ID,
			"agent_address": agentInfo.Registration.Address,
		})
	}

	// =========================================================================
	// PHASE 2: Parameter Extraction (before HITL to have params for approval)
	// =========================================================================
	// Extract capability and parameters from metadata
	capability := ""
	var parameters map[string]interface{}

	if cap, ok := step.Metadata["capability"].(string); ok {
		capability = cap
	}
	if params, ok := step.Metadata["parameters"].(map[string]interface{}); ok {
		parameters = params
	}

	// =========================================================================
	// PHASE 3: Parameter Resolution (before HITL to show resolved values)
	// =========================================================================
	// Check if pre-resolved parameters exist (resuming from HITL checkpoint).
	// IMPORTANT: Only use pre-resolved params for the SPECIFIC step that was interrupted.
	// Other steps should resolve their params normally to avoid using wrong parameters.
	preResolved := GetPreResolvedParams(ctx)
	preResolvedStepID := GetPreResolvedStepID(ctx)
	usePreResolved := len(preResolved) > 0 && preResolvedStepID == step.StepID

	if usePreResolved {
		if e.logger != nil {
			e.logger.InfoWithContext(ctx, "Using pre-resolved parameters from HITL checkpoint", map[string]interface{}{
				"operation":            "hitl_resume_params",
				"step_id":              step.StepID,
				"pre_resolved_step_id": preResolvedStepID,
				"pre_resolved":         preResolved,
				"original_params":      parameters,
			})
		}
		telemetry.AddSpanEvent(ctx, "hitl.resume.using_approved_params",
			attribute.String("step_id", step.StepID),
			attribute.Int("param_count", len(preResolved)),
		)
		// Use pre-resolved params directly - these are what the user approved
		parameters = preResolved
	} else {
		// Normal flow: resolve parameters from dependencies
		// Parameter Resolution: Hybrid auto-wiring OR legacy template interpolation
		// When hybrid resolution is enabled, it uses intelligent name-matching and optional LLM
		// micro-resolution instead of brittle template substitution.

		// Direct stderr logging for guaranteed visibility (bypasses logger filtering)
		log.Printf("[GOMIND-EXEC-V2] Step %s: depends=%d, useHybrid=%v, resolverSet=%v",
			step.StepID, len(step.DependsOn), e.useHybridResolution, e.hybridResolver != nil)

		// Also log via framework logger for structured output
		if e.logger != nil {
			e.logger.InfoWithContext(ctx, "Hybrid resolution check", map[string]interface{}{
				"step_id":               step.StepID,
				"depends_on_count":      len(step.DependsOn),
				"use_hybrid_resolution": e.useHybridResolution,
				"hybrid_resolver_set":   e.hybridResolver != nil,
				"will_use_hybrid":       len(step.DependsOn) > 0 && e.useHybridResolution && e.hybridResolver != nil,
			})
		}
		if len(step.DependsOn) > 0 && e.useHybridResolution && e.hybridResolver != nil {
			// Phase 4: Hybrid Parameter Resolution (preferred)
			// Get the capability schema for target parameter types
			capabilityForResolution := e.findCapabilitySchema(agentInfo, capability)
			if e.logger != nil {
				e.logger.InfoWithContext(ctx, "findCapabilitySchema result", map[string]interface{}{
					"step_id":        step.StepID,
					"capability":     capability,
					"schema_found":   capabilityForResolution != nil,
					"agent_name":     step.AgentName,
					"agent_info_nil": agentInfo == nil,
					"agent_cap_count": func() int {
						if agentInfo != nil {
							return len(agentInfo.Capabilities)
						}
						return 0
					}(),
				})
			}
			if capabilityForResolution != nil {
				// Collect step results from dependencies
				stepResultsMap := e.collectDependencyResults(ctx, step.DependsOn)

				// Pass step instruction for ordinal resolution context (e.g., "first", "second", "third")
				resolved, err := e.hybridResolver.ResolveParameters(ctx, stepResultsMap, capabilityForResolution, step.StepID, step.Instruction)
				if err != nil {
					if e.logger != nil {
						e.logger.WarnWithContext(ctx, "Hybrid resolution failed, falling back to template interpolation", map[string]interface{}{
							"operation": "hybrid_resolution_fallback",
							"step_id":   step.StepID,
							"error":     err.Error(),
						})
					}
					// Fall through to template interpolation below
				} else if len(resolved) > 0 {
					// Merge resolved parameters with original parameters
					// Replace template strings with resolved values, preserve hardcoded values
					if parameters == nil {
						parameters = make(map[string]interface{})
					}
					for key, val := range resolved {
						existing, exists := parameters[key]
						if !exists {
							// Key doesn't exist, add resolved value
							parameters[key] = val
						} else if strVal, ok := existing.(string); ok && strings.Contains(strVal, "{{") {
							// Existing value is a template string - replace with resolved value
							parameters[key] = val
						}
						// Otherwise, preserve the existing hardcoded value
					}

					if e.logger != nil {
						e.logger.InfoWithContext(ctx, "HYBRID RESOLUTION APPLIED", map[string]interface{}{
							"operation":       "hybrid_parameter_resolution",
							"step_id":         step.StepID,
							"capability":      capability,
							"resolved_params": resolved,
							"final_params":    parameters,
						})
					}

					// Add telemetry for hybrid resolution success
					telemetry.AddSpanEvent(ctx, "hybrid_resolution_applied",
						attribute.String("step_id", step.StepID),
						attribute.String("capability", capability),
						attribute.Int("resolved_count", len(resolved)),
					)
					telemetry.Counter("orchestration.hybrid_resolution.success",
						"capability", capability,
						"module", telemetry.ModuleOrchestration,
					)
				}
			}
		}

		// Legacy: Template interpolation (fallback when hybrid resolution is disabled or unavailable)
		// This enables templates like {{geocode.latitude}} to be replaced with actual values
		if depResults, ok := ctx.Value(dependencyResultsKey).(map[string]map[string]interface{}); ok && len(depResults) > 0 {
			interpolated := e.interpolateParameters(parameters, depResults)
			if interpolated != nil {
				if e.logger != nil {
					e.logger.DebugWithContext(ctx, "Template parameters interpolated", map[string]interface{}{
						"operation":    "parameter_interpolation",
						"step_id":      step.StepID,
						"original":     parameters,
						"interpolated": interpolated,
					})
				}
				parameters = interpolated
			}

			// Semantic Fallback: Resolve any remaining unresolved templates using LLM inference
			// This handles cases where the template path doesn't exist (e.g., {{step-2.response.data.country.currency}}
			// when geocoding returns country:"France" as a string, not an object with currency field)
			if e.hybridResolver != nil && e.useHybridResolution {
				parameters = e.resolveUnresolvedTemplatesWithLLM(ctx, parameters, depResults, step.StepID)
			}
		}
	} // End of else block for normal parameter resolution (non-resume path)

	// =========================================================================
	// PHASE 4: Type Coercion (before HITL to show coerced values)
	// =========================================================================
	// Layer 2: Schema-Based Type Coercion
	// Coerce parameters to match capability schema types.
	// This fixes LLM-generated parameters that have incorrect types (e.g., "35.6" instead of 35.6).
	// The schema is obtained from the tool's InputSummary via the catalog.
	capabilitySchema := e.findCapabilitySchema(agentInfo, capability)

	// INFO: Diagnostic log to understand coercion entry point
	if e.logger != nil {
		schemaParams := 0
		if capabilitySchema != nil {
			schemaParams = len(capabilitySchema.Parameters)
		}
		e.logger.InfoWithContext(ctx, "TYPE COERCION CHECK", map[string]interface{}{
			"operation":             "type_coercion_entry",
			"step_id":               step.StepID,
			"capability":            capability,
			"parameters":            parameters,
			"schema_found":          capabilitySchema != nil,
			"schema_param_count":    schemaParams,
			"will_attempt_coercion": capabilitySchema != nil && schemaParams > 0,
		})
	}

	if capabilitySchema != nil && len(capabilitySchema.Parameters) > 0 {
		coerced, coercionLog := coerceParameterTypes(parameters, capabilitySchema.Parameters)
		if len(coercionLog) > 0 {
			// Telemetry: Add span event for distributed tracing visibility
			// This allows operators to see coercion events in trace visualization tools
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

			// Structured logging - INFO level for visibility
			// This includes both type coercions (string->number) and object field extractions (map->string)
			if e.logger != nil {
				e.logger.InfoWithContext(ctx, "TYPE COERCION APPLIED", map[string]interface{}{
					"operation":      "type_coercion",
					"step_id":        step.StepID,
					"capability":     capability,
					"coercions":      coercionLog,
					"coercion_count": len(coercionLog),
				})
			}
		}
		parameters = coerced
	}

	// =========================================================================
	// PHASE 5: Store resolved params in context for HITL checkpoint visibility
	// =========================================================================
	// This allows users to see actual parameter values (e.g., amount: 15000)
	// instead of template placeholders when approving step execution.
	ctx = WithResolvedParams(ctx, parameters)

	// =========================================================================
	// PHASE 6: HITL Pre-step approval check (NOW has resolved params!)
	// =========================================================================
	// If interrupt controller is set and policy triggers, pause for human approval.
	// Note: We check controller != nil (not config.HITL.Enabled) - see struct field comment.
	// Parameters are now fully resolved, so checkpoint will contain actual values.
	if e.interruptController != nil {
		// Get plan from context for HITL checks
		var plan *RoutingPlan
		if p, ok := ctx.Value(planContextKey).(*RoutingPlan); ok {
			plan = p
		}

		checkpoint, err := e.interruptController.CheckBeforeStep(ctx, step, plan)
		if err != nil {
			if e.logger != nil {
				e.logger.WarnWithContext(ctx, "HITL pre-step check failed, continuing execution", map[string]interface{}{
					"operation":  "hitl_pre_step_check",
					"step_id":    step.StepID,
					"agent_name": step.AgentName,
					"error":      err.Error(),
				})
			}
			// Non-fatal: continue if HITL check fails (graceful degradation)
		} else if checkpoint != nil {
			// Step requires human approval - return interrupt result
			if e.logger != nil {
				e.logger.InfoWithContext(ctx, "Step execution interrupted for human approval", map[string]interface{}{
					"operation":           "hitl_pre_step_interrupt",
					"step_id":             step.StepID,
					"agent_name":          step.AgentName,
					"checkpoint_id":       checkpoint.CheckpointID,
					"resolved_params":     parameters,
					"has_resolved_params": checkpoint.ResolvedParameters != nil,
				})
			}
			telemetry.AddSpanEvent(ctx, "hitl.step.interrupted",
				attribute.String("step_id", step.StepID),
				attribute.String("checkpoint_id", checkpoint.CheckpointID),
			)
			result.Success = false
			result.Error = fmt.Sprintf("HITL: awaiting approval (checkpoint: %s)", checkpoint.CheckpointID)
			result.EndTime = time.Now()
			result.Duration = time.Since(startTime)
			// Store checkpoint info in metadata for Execute() to detect and propagate
			result.Metadata = map[string]interface{}{
				"hitl_checkpoint_id":   checkpoint.CheckpointID,
				"hitl_interrupt_point": string(checkpoint.InterruptPoint),
				"hitl_checkpoint":      checkpoint, // Full checkpoint for ErrInterrupted
			}
			return result
		}
	}

	// =========================================================================
	// PHASE 7: Find capability endpoint
	// =========================================================================
	// Find the capability endpoint
	endpoint := e.findCapabilityEndpoint(agentInfo, capability)
	if endpoint == "" {
		err := fmt.Errorf("capability %s not found for agent %s", capability, step.AgentName)
		telemetry.RecordSpanError(ctx, err)
		if e.logger != nil {
			e.logger.ErrorWithContext(ctx, "Capability endpoint not found", map[string]interface{}{
				"operation":  "capability_resolution",
				"step_id":    step.StepID,
				"agent_name": step.AgentName,
				"capability": capability,
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
	maxAttempts := e.maxAttempts
	if maxAttempts < 1 {
		maxAttempts = 2 // Fallback default if not set
	}
	validationRetries := 0
	previousErrors := []string{} // Layer 4: tracks error history for semantic retry

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result.Attempts = attempt
		isTransientErrorDetected := false // Reset for each attempt - tracks if LLM identified transient error

		if e.logger != nil {
			e.logger.DebugWithContext(ctx, "Making HTTP call to agent", map[string]interface{}{
				"operation":    "agent_http_call",
				"step_id":      step.StepID,
				"agent_name":   step.AgentName,
				"attempt":      attempt,
				"max_attempts": maxAttempts,
				"url":          url,
				"capability":   capability,
			})
		}

		// Make the HTTP request based on component type
		// Tools expect raw parameters, agents expect wrapped {"data": ...} format
		var response, responseBody string
		var err error
		if agentInfo.Registration.Type == core.ComponentTypeAgent {
			// Agents expect {"data": {...}} wrapper
			response, responseBody, err = e.callAgentService(ctx, url, parameters)
		} else {
			// Tools expect raw parameters (default for backward compatibility)
			response, responseBody, err = e.callTool(ctx, url, parameters)
		}
		if err == nil {
			if e.logger != nil {
				e.logger.DebugWithContext(ctx, "Agent HTTP call successful", map[string]interface{}{
					"operation":       "agent_http_response",
					"step_id":         step.StepID,
					"agent_name":      step.AgentName,
					"attempt":         attempt,
					"response_length": len(response),
				})
			}
			result.Success = true
			result.Response = response
			break
		}

		// Layer 3: LLM-based Error Analysis (Phase 4 Enhancement)
		// When ErrorAnalyzer is configured, use LLM to determine if error can be fixed
		// with different parameters. This replaces the need for tools to set Retryable flags.
		// HTTP Status Routing:
		//   - 400, 404, 409, 422 → LLM Error Analyzer (might be fixable with different input)
		//   - 408, 429, 5xx      → Resilience module (same payload + exponential backoff)
		//   - 401, 403, 405      → Fail immediately (auth/permission issues)
		if e.errorAnalyzer != nil && e.errorAnalyzer.IsEnabled() && validationRetries < e.maxValidationRetries {
			httpStatus := extractHTTPStatusFromError(err)

			// Build error analysis context
			errCtx := &ErrorAnalysisContext{
				HTTPStatus:            httpStatus,
				ErrorResponse:         responseBody,
				OriginalRequest:       parameters,
				UserQuery:             step.Instruction,
				CapabilityName:        capability,
				CapabilityDescription: "",
			}
			if capabilitySchema != nil {
				errCtx.CapabilityDescription = capabilitySchema.Description
			}

			// Analyze error with LLM
			analysisResult, analysisErr := e.errorAnalyzer.AnalyzeError(ctx, errCtx)

			if analysisErr != nil {
				if e.logger != nil {
					e.logger.WarnWithContext(ctx, "LLM error analysis failed", map[string]interface{}{
						"operation":   "error_analysis_failed",
						"step_id":     step.StepID,
						"capability":  capability,
						"http_status": httpStatus,
						"error":       analysisErr.Error(),
					})
				}
				// Fall through to legacy error handling
			} else if analysisResult == nil {
				// nil result means delegate to resilience module (transient error)
				if e.logger != nil {
					e.logger.DebugWithContext(ctx, "Error delegated to resilience module", map[string]interface{}{
						"operation":   "resilience_delegation",
						"step_id":     step.StepID,
						"capability":  capability,
						"http_status": httpStatus,
					})
				}
				// Continue with normal retry logic (same payload)
			} else if analysisResult.ShouldRetry {
				// LLM determined this error is retryable
				// This handles both:
				// 1. Parameter changes needed (SuggestedChanges non-empty)
				// 2. Transient errors where retry with same params may work (SuggestedChanges empty)
				validationRetries++

				// Telemetry: Add span event for LLM error analysis
				// Include error details and suggested changes for complete visibility in distributed traces
				suggestedChangesJSON, _ := json.Marshal(analysisResult.SuggestedChanges)
				telemetry.AddSpanEvent(ctx, "llm_error_analysis_retry",
					attribute.String("step_id", step.StepID),
					attribute.String("capability", capability),
					attribute.Int("http_status", httpStatus),
					attribute.String("reason", analysisResult.Reason),
					attribute.String("error_message", err.Error()),
					attribute.String("error_response", truncateString(responseBody, 500)),
					attribute.String("suggested_changes", string(suggestedChangesJSON)),
					attribute.Int("suggested_changes_count", len(analysisResult.SuggestedChanges)),
				)
				telemetry.Counter("orchestration.error_analysis.retry",
					"capability", capability,
					"http_status", fmt.Sprintf("%d", httpStatus),
					"module", telemetry.ModuleOrchestration,
				)

				// Log appropriately based on whether we have parameter changes
				hasChanges := len(analysisResult.SuggestedChanges) > 0
				if e.logger != nil {
					if hasChanges {
						e.logger.InfoWithContext(ctx, "LLM error analysis suggests retry with parameter changes", map[string]interface{}{
							"operation":         "error_analysis_retry",
							"step_id":           step.StepID,
							"capability":        capability,
							"http_status":       httpStatus,
							"reason":            analysisResult.Reason,
							"suggested_changes": analysisResult.SuggestedChanges,
							"validation_retry":  validationRetries,
						})
					} else {
						e.logger.InfoWithContext(ctx, "LLM error analysis suggests retry with same parameters (transient)", map[string]interface{}{
							"operation":        "error_analysis_retry_same_params",
							"step_id":          step.StepID,
							"capability":       capability,
							"http_status":      httpStatus,
							"reason":           analysisResult.Reason,
							"is_transient":     analysisResult.IsTransientError,
							"validation_retry": validationRetries,
						})
					}
				}

				// Merge suggested changes into parameters (no-op if empty)
				for key, val := range analysisResult.SuggestedChanges {
					parameters[key] = val
				}

				attempt-- // Retry with corrected parameters (don't count as regular retry)
				continue
			} else if !analysisResult.ShouldRetry {
				// LLM determined this error is not fixable
				if e.logger != nil {
					e.logger.InfoWithContext(ctx, "LLM error analysis: not retryable", map[string]interface{}{
						"operation":   "error_analysis_no_retry",
						"step_id":     step.StepID,
						"capability":  capability,
						"http_status": httpStatus,
						"reason":      analysisResult.Reason,
					})
				}

				telemetry.AddSpanEvent(ctx, "llm_error_analysis_no_retry",
					attribute.String("step_id", step.StepID),
					attribute.String("reason", analysisResult.Reason),
				)
				telemetry.Counter("orchestration.error_analysis.no_retry",
					"capability", capability,
					"http_status", fmt.Sprintf("%d", httpStatus),
					"module", telemetry.ModuleOrchestration,
				)

				// ============================================================
				// LAYER 4: Contextual Re-Resolution (Semantic Retry)
				// ErrorAnalyzer said "cannot fix" but it lacks source data.
				// Try ContextualReResolver which has BOTH error AND source data.
				// For independent steps (no dependencies), sourceData will be empty
				// but the LLM still has UserQuery, ErrorResponse, and Capability schema.
				// ============================================================
				if e.contextualReResolver != nil && validationRetries < e.maxSemanticRetries {
					// Collect source data from dependencies (may be empty for independent steps)
					sourceData := e.collectSourceDataFromDependencies(ctx, step.DependsOn)

					// Check if semantic retry for independent steps is enabled
					// When disabled, skip Layer 4 for steps without dependencies (old behavior)
					if len(sourceData) == 0 && !e.semanticRetryForIndependentSteps {
						if e.logger != nil {
							e.logger.DebugWithContext(ctx, "Skipping semantic retry for independent step (disabled by config)", map[string]interface{}{
								"step_id":    step.StepID,
								"capability": capability,
							})
						}
						// Skip to transient error handling below
					} else {
						// Track if this is an independent step for telemetry
						isIndependentStep := len(sourceData) == 0

						// Build execution context with full trajectory
						execCtx := &ExecutionContext{
							UserQuery:       step.Instruction,
							SourceData:      sourceData, // May be empty {} for independent steps
							StepID:          step.StepID,
							Capability:      capabilitySchema,
							AttemptedParams: parameters,
							ErrorResponse:   responseBody,
							HTTPStatus:      httpStatus,
							RetryCount:      validationRetries,
							PreviousErrors:  previousErrors,
						}

						reResult, reErr := e.contextualReResolver.ReResolve(ctx, execCtx)
						if reErr == nil && reResult.ShouldRetry && len(reResult.CorrectedParameters) > 0 {
							validationRetries++
							previousErrors = append(previousErrors, responseBody)

							// Telemetry: Record semantic retry
							correctedJSON, _ := json.Marshal(reResult.CorrectedParameters)
							telemetry.AddSpanEvent(ctx, "semantic_retry_applied",
								attribute.String("step_id", step.StepID),
								attribute.String("capability", capability),
								attribute.String("analysis", reResult.Analysis),
								attribute.String("corrected_params", string(correctedJSON)),
								attribute.Bool("independent_step", isIndependentStep),
							)

							// Separate metric for independent step retries
							if isIndependentStep {
								telemetry.Counter("orchestration.semantic_retry.independent_step",
									"capability", capability,
									"module", telemetry.ModuleOrchestration,
								)
							}

							telemetry.Counter("orchestration.semantic_retry.applied",
								"capability", capability,
								"module", telemetry.ModuleOrchestration,
							)

							if e.logger != nil {
								e.logger.InfoWithContext(ctx, "SEMANTIC RETRY: Applying corrected parameters", map[string]interface{}{
									"operation":            "semantic_retry_applied",
									"step_id":              step.StepID,
									"capability":           capability,
									"analysis":             reResult.Analysis,
									"corrected_parameters": reResult.CorrectedParameters,
									"semantic_retry_count": validationRetries,
									"independent_step":     isIndependentStep,
								})
							}

							// Replace parameters and retry
							parameters = reResult.CorrectedParameters
							attempt-- // Don't count as regular retry
							continue  // Go back to HTTP call with new params
						}

						// Re-resolution failed or said don't retry
						if reErr != nil && e.logger != nil {
							e.logger.WarnWithContext(ctx, "Semantic retry failed", map[string]interface{}{
								"operation":        "semantic_retry_failed",
								"step_id":          step.StepID,
								"error":            reErr.Error(),
								"independent_step": isIndependentStep,
							})
						}
					}
				}
				// END LAYER 4

				// ============================================================
				// TRANSIENT ERROR HANDLING (503 timeouts, service unavailable)
				// If LLM identified this as a transient error, continue with
				// resilience retry (same payload + exponential backoff) instead
				// of breaking out of the retry loop.
				// ============================================================
				if analysisResult.IsTransientError {
					isTransientErrorDetected = true // Flag to skip isNonRetryableToolError check below
					if e.logger != nil {
						e.logger.InfoWithContext(ctx, "Transient error detected, continuing with resilience retry", map[string]interface{}{
							"operation":   "transient_error_resilience_retry",
							"step_id":     step.StepID,
							"capability":  capability,
							"http_status": httpStatus,
							"reason":      analysisResult.Reason,
							"attempt":     attempt,
						})
					}
					telemetry.AddSpanEvent(ctx, "transient_error_resilience_retry",
						attribute.String("step_id", step.StepID),
						attribute.String("capability", capability),
						attribute.Int("http_status", httpStatus),
						attribute.String("reason", analysisResult.Reason),
						attribute.Int("attempt", attempt),
					)
					telemetry.Counter("orchestration.transient_error.resilience_retry",
						"capability", capability,
						"http_status", fmt.Sprintf("%d", httpStatus),
						"module", telemetry.ModuleOrchestration,
					)
					// Don't break - continue to regular retry with exponential backoff
					// (the code below handles waiting and retry logic)
				} else {
					// Break out of retry loop - neither ErrorAnalyzer nor ContextualReResolver could fix
					// and this is not a transient error
					break
				}
			}
		}

		// Legacy Layer 3: Check if this is an error that could be fixed by LLM
		// (fallback when ErrorAnalyzer is not configured)
		// Triggers on:
		// 1. Type-related errors (string patterns like "cannot unmarshal", "type mismatch")
		// 2. Tool errors with Retryable: true (structured ToolResponse.Error.Retryable)
		// This ensures AI correction for ALL retryable errors per INTELLIGENT_ERROR_HANDLING.md
		if e.errorAnalyzer == nil && e.validationFeedbackEnabled && e.correctionCallback != nil &&
			validationRetries < e.maxValidationRetries && shouldAttemptAICorrection(err, responseBody) {

			validationRetries++

			// Telemetry: Add span event for validation feedback attempt
			// Include error details for complete visibility in distributed traces
			telemetry.AddSpanEvent(ctx, "validation_feedback_started",
				attribute.String("step_id", step.StepID),
				attribute.String("capability", capability),
				attribute.Int("validation_retry", validationRetries),
				attribute.String("error_type", "type_mismatch"),
				attribute.String("error_message", err.Error()),
				attribute.String("error_response", truncateString(responseBody, 500)),
			)

			// Telemetry: Record metric for monitoring dashboards
			telemetry.Counter("orchestration.validation_feedback.attempts",
				"capability", capability,
				"module", telemetry.ModuleOrchestration,
			)

			if e.logger != nil {
				e.logger.InfoWithContext(ctx, "Type error detected, requesting LLM correction", map[string]interface{}{
					"operation":        "validation_feedback",
					"step_id":          step.StepID,
					"capability":       capability,
					"validation_retry": validationRetries,
					"max_retries":      e.maxValidationRetries,
					"error":            err.Error(),
				})
			}

			// Request correction from LLM via callback
			correctedParams, corrErr := e.correctionCallback(ctx, step, parameters, err.Error(), capabilitySchema)
			if corrErr == nil && correctedParams != nil {
				// Telemetry: Record successful correction with corrected parameters
				// Serialize corrected params for visibility in distributed traces
				correctedParamsJSON, _ := json.Marshal(correctedParams)
				telemetry.AddSpanEvent(ctx, "validation_feedback_success",
					attribute.String("step_id", step.StepID),
					attribute.Int("retries_used", validationRetries),
					attribute.String("corrected_params", truncateString(string(correctedParamsJSON), 500)),
				)
				telemetry.Counter("orchestration.validation_feedback.success",
					"capability", capability,
					"module", telemetry.ModuleOrchestration,
				)

				if e.logger != nil {
					e.logger.DebugWithContext(ctx, "Parameters corrected by LLM", map[string]interface{}{
						"operation":  "validation_feedback_success",
						"step_id":    step.StepID,
						"capability": capability,
						"new_params": correctedParams,
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
				e.logger.WarnWithContext(ctx, "LLM parameter correction failed", map[string]interface{}{
					"operation":  "validation_feedback_failed",
					"step_id":    step.StepID,
					"capability": capability,
					"error":      corrErr.Error(),
				})
			}
		}

		// Check if error is explicitly non-retryable - don't waste retries on permanent errors
		// This prevents blind retries when a tool explicitly says "don't retry"
		// (e.g., LOCATION_NOT_FOUND, INVALID_SYMBOL, COUNTRY_NOT_FOUND)
		// IMPORTANT: Skip this check if LLM detected a transient error - LLM's assessment
		// takes precedence over the tool's retryable flag for infrastructure issues
		if !isTransientErrorDetected && isNonRetryableToolError(responseBody) {
			// Telemetry: Record that we stopped early due to non-retryable error
			telemetry.AddSpanEvent(ctx, "non_retryable_error_detected",
				attribute.String("step_id", step.StepID),
				attribute.String("capability", capability),
				attribute.Int("attempt", attempt),
			)
			telemetry.Counter("orchestration.non_retryable_errors",
				"capability", capability,
				"module", telemetry.ModuleOrchestration,
			)

			if e.logger != nil {
				e.logger.InfoWithContext(ctx, "Non-retryable error detected, stopping retries", map[string]interface{}{
					"operation":  "non_retryable_error",
					"step_id":    step.StepID,
					"capability": capability,
					"attempt":    attempt,
					"error":      err.Error(),
				})
			}

			// Break out of retry loop - no point retrying with same parameters
			break
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
			}
			if logLevel == "Error" {
				e.logger.ErrorWithContext(ctx, "Agent HTTP call failed after all retries", logData)
			} else {
				e.logger.DebugWithContext(ctx, "Agent HTTP call failed, retrying", logData)
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
				e.logger.DebugWithContext(ctx, "Waiting before retry", map[string]interface{}{
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

	// HITL: Post-step checks
	if e.interruptController != nil {
		if result.Success {
			// Post-step output validation check
			checkpoint, hitlErr := e.interruptController.CheckAfterStep(ctx, step, &result)
			if hitlErr != nil {
				if e.logger != nil {
					e.logger.WarnWithContext(ctx, "HITL post-step check failed", map[string]interface{}{
						"operation":  "hitl_post_step_check",
						"step_id":    step.StepID,
						"agent_name": step.AgentName,
						"error":      hitlErr.Error(),
					})
				}
				// Non-fatal: continue if HITL check fails
			} else if checkpoint != nil {
				// Output requires validation - return interrupt result
				if e.logger != nil {
					e.logger.InfoWithContext(ctx, "Step output requires validation", map[string]interface{}{
						"operation":     "hitl_post_step_interrupt",
						"step_id":       step.StepID,
						"agent_name":    step.AgentName,
						"checkpoint_id": checkpoint.CheckpointID,
					})
				}
				telemetry.AddSpanEvent(ctx, "hitl.step.output_validation",
					attribute.String("step_id", step.StepID),
					attribute.String("checkpoint_id", checkpoint.CheckpointID),
				)
				result.Success = false
				result.Error = fmt.Sprintf("HITL: awaiting output validation (checkpoint: %s)", checkpoint.CheckpointID)
			}
		} else {
			// Error escalation check for failed steps
			checkpoint, hitlErr := e.interruptController.CheckOnError(ctx, step, fmt.Errorf("%s", result.Error), result.Attempts)
			if hitlErr != nil {
				if e.logger != nil {
					e.logger.WarnWithContext(ctx, "HITL error escalation check failed", map[string]interface{}{
						"operation":  "hitl_error_escalation_check",
						"step_id":    step.StepID,
						"agent_name": step.AgentName,
						"error":      hitlErr.Error(),
					})
				}
				// Non-fatal: continue if HITL check fails
			} else if checkpoint != nil {
				// Error escalated to human
				if e.logger != nil {
					e.logger.InfoWithContext(ctx, "Step error escalated for human review", map[string]interface{}{
						"operation":     "hitl_error_escalation",
						"step_id":       step.StepID,
						"agent_name":    step.AgentName,
						"checkpoint_id": checkpoint.CheckpointID,
						"attempts":      result.Attempts,
					})
				}
				telemetry.AddSpanEvent(ctx, "hitl.step.error_escalated",
					attribute.String("step_id", step.StepID),
					attribute.String("checkpoint_id", checkpoint.CheckpointID),
					attribute.Int("attempts", result.Attempts),
				)
				result.Error = fmt.Sprintf("HITL: error escalated (checkpoint: %s) - original error: %s", checkpoint.CheckpointID, result.Error)
			}
		}
	}

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
		e.logger.DebugWithContext(ctx, "Step execution completed", map[string]interface{}{
			"operation":   "step_execution_complete",
			"step_id":     step.StepID,
			"agent_name":  step.AgentName,
			"success":     result.Success,
			"duration_ms": result.Duration.Milliseconds(),
			"error":       result.Error,
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
//   - map -> string: {"code":"EUR","name":"Euro"} -> "EUR" (smart field extraction)
func coerceValue(value interface{}, expectedType string) (interface{}, bool) {
	// Handle string values
	if strVal, isString := value.(string); isString {
		return coerceStringValue(strVal, expectedType)
	}

	// Handle map values - smart field extraction when string is expected
	if mapVal, isMap := value.(map[string]interface{}); isMap {
		if expectedType == "string" {
			if extracted, _, ok := extractPrimaryField(mapVal); ok {
				return extracted, true // Return extracted value and mark as coerced
			}
			// If extraction failed, try JSON serialization as fallback
			// This preserves the original behavior of converting maps to JSON strings
		}
		// For non-string expected types, return map as-is
		return value, false
	}

	// Other non-string types, return as-is
	return value, false
}

// coerceStringValue handles coercion of string values to expected types.
func coerceStringValue(strVal string, expectedType string) (interface{}, bool) {
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
		// Check if this is a JSON object string that we should extract a field from
		// This handles cases where template substitution converted a complex object to JSON
		// e.g., '{"code":"EUR","name":"Euro"}' -> we want to extract "EUR"
		if strings.HasPrefix(strVal, "{") && strings.HasSuffix(strVal, "}") {
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(strVal), &parsed); err == nil {
				if extracted, _, ok := extractPrimaryField(parsed); ok {
					return extracted, true
				}
			}
		}
		// Already a string, no coercion needed
		return strVal, false
	}

	// Coercion failed or not applicable, return original
	return strVal, false
}

// extractPrimaryField attempts to extract a primary/identifier field from a map.
// This handles the common case where LLM resolves a template to a complex object
// (e.g., {"code":"EUR","name":"Euro","symbol":"€"}) but the downstream tool
// expects just the identifier (e.g., "EUR").
//
// Extraction priority (based on common API conventions):
//  1. Single-field maps: use the only field (unambiguous)
//  2. "code" field: common for currencies, countries, statuses
//  3. "id" field: common for identifiers
//  4. "value" field: common for value wrappers
//  5. "name" field: common for labels/names
//  6. "key" field: common for key-value pairs
//
// Returns the extracted value, the field name used, and whether extraction succeeded.
// Only extracts string values to avoid type confusion.
func extractPrimaryField(m map[string]interface{}) (string, string, bool) {
	if len(m) == 0 {
		return "", "", false
	}

	// Priority 1: Single-field maps are unambiguous
	if len(m) == 1 {
		for k, v := range m {
			if strVal, ok := v.(string); ok {
				return strVal, k, true
			}
		}
	}

	// Priority 2-6: Try common identifier fields in order of priority
	priorityFields := []string{"code", "id", "value", "name", "key"}
	for _, field := range priorityFields {
		if v, exists := m[field]; exists {
			if strVal, ok := v.(string); ok {
				return strVal, field, true
			}
		}
	}

	// Also check case-insensitive matches (LLMs sometimes use different cases)
	lowerMap := make(map[string]interface{})
	for k, v := range m {
		lowerMap[strings.ToLower(k)] = v
	}
	for _, field := range priorityFields {
		if v, exists := lowerMap[field]; exists {
			if strVal, ok := v.(string); ok {
				return strVal, field, true
			}
		}
	}

	return "", "", false
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

// shouldAttemptAICorrection determines if an error should trigger LLM-powered correction.
// This is the primary entry point for deciding AI correction, combining multiple strategies:
// 1. Type-related errors: Pattern matching for common type mismatches
// 2. Structured Retryable errors: Parse ToolResponse and check Error.Retryable flag
//
// This function ensures compliance with docs/INTELLIGENT_ERROR_HANDLING.md which states
// that ANY error with Retryable=true should trigger AI correction, not just type errors.
func shouldAttemptAICorrection(err error, responseBody string) bool {
	// Strategy 1: Check for type-related error patterns (legacy support)
	if isTypeRelatedError(err, responseBody) {
		return true
	}

	// Strategy 2: Check for structured ToolResponse with Retryable=true
	if isRetryableToolError(responseBody) {
		return true
	}

	return false
}

// isRetryableToolError parses the response body as a ToolResponse and checks
// if the error is marked as retryable. This enables AI correction for ALL
// errors that tools indicate can be fixed with different input.
//
// Expected format:
//
//	{
//	  "success": false,
//	  "error": {
//	    "code": "INVALID_CURRENCY",
//	    "message": "Currency not found",
//	    "retryable": true,
//	    ...
//	  }
//	}
func isRetryableToolError(responseBody string) bool {
	if responseBody == "" {
		return false
	}

	// Try to parse as ToolResponse structure
	var response struct {
		Success bool `json:"success"`
		Error   *struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			Category  string `json:"category"`
			Retryable bool   `json:"retryable"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(responseBody), &response); err != nil {
		return false
	}

	// Check if it's a failed response with a retryable error
	if !response.Success && response.Error != nil && response.Error.Retryable {
		return true
	}

	return false
}

// isNonRetryableToolError parses the response body as a ToolResponse and checks
// if the error is explicitly marked as non-retryable (retryable: false).
// This is used to prevent blind retries when a tool indicates the error cannot
// be fixed by retrying with the same input.
//
// Examples of non-retryable errors:
//   - LOCATION_NOT_FOUND: Location doesn't exist, retrying won't help
//   - INVALID_SYMBOL: Stock symbol is invalid, no point retrying
//   - COUNTRY_NOT_FOUND: Country doesn't exist in the database
//
// Returns true if the response explicitly has success:false AND retryable:false
func isNonRetryableToolError(responseBody string) bool {
	if responseBody == "" {
		return false
	}

	// Try to parse as ToolResponse structure
	var response struct {
		Success bool `json:"success"`
		Error   *struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			Retryable bool   `json:"retryable"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(responseBody), &response); err != nil {
		return false
	}

	// Check if it's a failed response with error info explicitly marked as non-retryable
	// Note: We check for Error != nil and explicitly !Retryable
	// This only returns true when the tool explicitly said "don't retry"
	if !response.Success && response.Error != nil && !response.Error.Retryable {
		return true
	}

	return false
}

// extractHTTPStatusFromError extracts the HTTP status code from an error message.
// The error format from callComponentWithBody is: "component returned status %d: %s"
// Returns the HTTP status code, or 0 if it cannot be extracted.
func extractHTTPStatusFromError(err error) int {
	if err == nil {
		return 0
	}

	errStr := err.Error()

	// Pattern: "component returned status XXX:"
	statusPattern := regexp.MustCompile(`status (\d{3})`)
	matches := statusPattern.FindStringSubmatch(errStr)
	if len(matches) >= 2 {
		status, parseErr := strconv.Atoi(matches[1])
		if parseErr == nil {
			return status
		}
	}

	return 0
}

// ============================================================================
// Component HTTP Call Functions
// ============================================================================
// These functions implement the HTTP communication layer for tools and agents.
// Tools and agents have different request format expectations:
//   - Tools: expect raw parameters {"location": "Tokyo", "units": "metric"}
//   - Agents: expect parameters wrapped in "data" field {"data": {...params...}}
//
// Architecture:
//   callTool()        → callComponentWithBody() (shared HTTP logic)
//   callAgentService() → callComponentWithBody() (shared HTTP logic)
// ============================================================================

// callComponentWithBody is the shared HTTP logic for calling any component (tool or agent).
// It handles request creation, tracing, response reading, and error handling.
// The body parameter should already be marshaled JSON with the correct format.
// Returns: (successResponse, errorResponseBody, error)
func (e *SmartExecutor) callComponentWithBody(ctx context.Context, url string, body []byte) (string, string, error) {
	// Log request details at DEBUG level
	if e.logger != nil {
		e.logger.DebugWithContext(ctx, "HTTP request to component", map[string]interface{}{
			"operation":   "component_http_request",
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
		return "", respBodyStr, fmt.Errorf("component returned status %d: %s", resp.StatusCode, respBodyStr)
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

// callTool sends an HTTP request to a tool with raw parameters.
// Tools expect flat JSON: {"location": "Tokyo", "units": "metric"}
// This is the standard format for all GoMind tools.
func (e *SmartExecutor) callTool(ctx context.Context, url string, parameters map[string]interface{}) (string, string, error) {
	// Tools receive raw parameters directly
	body, err := json.Marshal(parameters)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal tool parameters: %w", err)
	}

	if e.logger != nil {
		e.logger.DebugWithContext(ctx, "Calling tool with raw parameters", map[string]interface{}{
			"operation":  "call_tool",
			"url":        url,
			"parameters": parameters,
		})
	}

	return e.callComponentWithBody(ctx, url, body)
}

// callAgentService sends an HTTP request to an agent with wrapped parameters.
// Agents expect parameters wrapped in a "data" field: {"data": {...params...}}
// This wrapper format is expected by BaseAgent handlers in the core module.
func (e *SmartExecutor) callAgentService(ctx context.Context, url string, parameters map[string]interface{}) (string, string, error) {
	// Agents expect parameters wrapped in a "data" field
	wrapped := map[string]interface{}{"data": parameters}
	body, err := json.Marshal(wrapped)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal agent parameters: %w", err)
	}

	if e.logger != nil {
		e.logger.DebugWithContext(ctx, "Calling agent with wrapped parameters", map[string]interface{}{
			"operation":  "call_agent_service",
			"url":        url,
			"parameters": parameters,
			"wrapped":    true,
		})
	}

	return e.callComponentWithBody(ctx, url, body)
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
