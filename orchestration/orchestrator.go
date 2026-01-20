package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// orchestratorContextKey is a custom type for orchestrator context keys to avoid collisions
type orchestratorContextKey string

const (
	// requestIDContextKey holds the orchestrator's request ID for correlation across components
	requestIDContextKey orchestratorContextKey = "orchestrator_request_id"
)

// WithRequestID adds the orchestrator's request ID to the context.
// This enables child components (like TieredCapabilityProvider) to correlate
// their debug recordings with the orchestrator's request.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey, requestID)
}

// GetRequestID retrieves the orchestrator's request ID from context.
// Returns empty string if not set.
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value(requestIDContextKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// resumeModeContextKey holds the checkpoint ID when resuming from a HITL checkpoint.
// This allows CheckPlanApproval to skip HITL checks during resume execution.
const resumeModeContextKey orchestratorContextKey = "orchestrator_resume_mode"

// WithResumeMode marks the context as resuming from a HITL checkpoint.
// When set, HITL checks (CheckPlanApproval, CheckBeforeStep) will be bypassed
// to prevent infinite loops during resume execution.
//
// Usage:
//
//	ctx = orchestration.WithResumeMode(ctx, checkpoint.CheckpointID)
//	result, err := orchestrator.ProcessRequestStreaming(ctx, checkpoint.OriginalRequest, metadata, callback)
func WithResumeMode(ctx context.Context, checkpointID string) context.Context {
	return context.WithValue(ctx, resumeModeContextKey, checkpointID)
}

// IsResumeMode checks if the context is in resume mode.
// Returns the checkpoint ID and true if resuming, empty string and false otherwise.
func IsResumeMode(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	if v := ctx.Value(resumeModeContextKey); v != nil {
		if id, ok := v.(string); ok && id != "" {
			return id, true
		}
	}
	return "", false
}

// metadataContextKey holds request metadata that should be preserved in checkpoints.
const metadataContextKey orchestratorContextKey = "orchestrator_metadata"

// WithMetadata attaches metadata to the context for HITL checkpoint preservation.
// This metadata (e.g., session_id, user_id) will be stored in checkpoint.UserContext
// and can be retrieved when resuming execution.
//
// Usage:
//
//	metadata := map[string]interface{}{"session_id": sessionID, "user_id": userID}
//	ctx = orchestration.WithMetadata(ctx, metadata)
//	result, err := orchestrator.ProcessRequestStreaming(ctx, query, nil, callback)
func WithMetadata(ctx context.Context, metadata map[string]interface{}) context.Context {
	if metadata == nil {
		return ctx
	}
	return context.WithValue(ctx, metadataContextKey, metadata)
}

// GetMetadata retrieves metadata from the context.
// Returns nil if no metadata is set.
func GetMetadata(ctx context.Context) map[string]interface{} {
	if ctx == nil {
		return nil
	}
	if v := ctx.Value(metadataContextKey); v != nil {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

// planOverrideContextKey holds a pre-approved plan for HITL resume flows.
// When set, ProcessRequest/ProcessRequestStreaming will skip LLM plan generation.
const planOverrideContextKey orchestratorContextKey = "orchestrator_plan_override"

// WithPlanOverride injects a pre-approved plan into context for HITL resume.
// When set, the orchestrator will use this plan instead of generating a new one via LLM.
// This is critical for HITL resume flows to ensure step IDs remain stable.
//
// Usage:
//
//	ctx = orchestration.WithPlanOverride(ctx, checkpoint.Plan)
//	result, err := orchestrator.ProcessRequestStreaming(ctx, checkpoint.OriginalRequest, metadata, callback)
func WithPlanOverride(ctx context.Context, plan *RoutingPlan) context.Context {
	if plan == nil {
		return ctx
	}
	return context.WithValue(ctx, planOverrideContextKey, plan)
}

// GetPlanOverride retrieves the injected plan from context.
// Returns nil if no plan override is set.
func GetPlanOverride(ctx context.Context) *RoutingPlan {
	if ctx == nil {
		return nil
	}
	if v := ctx.Value(planOverrideContextKey); v != nil {
		if p, ok := v.(*RoutingPlan); ok {
			return p
		}
	}
	return nil
}

// completedStepsContextKey holds step results from a HITL checkpoint.
// The executor will skip these steps and use the cached results.
const completedStepsContextKey orchestratorContextKey = "orchestrator_completed_steps"

// WithCompletedSteps injects already-completed step results into context.
// The executor will skip these steps and use the provided results for dependency resolution.
// This prevents re-execution of steps that were completed before a HITL checkpoint.
//
// Usage:
//
//	ctx = orchestration.WithCompletedSteps(ctx, checkpoint.StepResults)
//	result, err := orchestrator.ProcessRequestStreaming(ctx, checkpoint.OriginalRequest, metadata, callback)
func WithCompletedSteps(ctx context.Context, results map[string]*StepResult) context.Context {
	if results == nil {
		return ctx
	}
	return context.WithValue(ctx, completedStepsContextKey, results)
}

// GetCompletedSteps retrieves completed step results from context.
// Returns nil if no completed steps are set.
func GetCompletedSteps(ctx context.Context) map[string]*StepResult {
	if ctx == nil {
		return nil
	}
	if v := ctx.Value(completedStepsContextKey); v != nil {
		if r, ok := v.(map[string]*StepResult); ok {
			return r
		}
	}
	return nil
}

// AIOrchestrator is an AI-powered orchestrator that uses LLM for intelligent routing
type AIOrchestrator struct {
	config      *OrchestratorConfig
	discovery   core.Discovery
	aiClient    core.AIClient
	catalog     *AgentCatalog
	executor    *SmartExecutor
	synthesizer *AISynthesizer

	// Capability provider for flexible capability discovery
	capabilityProvider CapabilityProvider

	// Prompt builder for extensible prompt customization (Layer 1-3)
	// If nil, uses the hardcoded default prompt for backwards compatibility
	promptBuilder PromptBuilder

	// LLM Debug Store for full payload visibility
	// When enabled, stores complete prompts/responses for debugging
	debugStore LLMDebugStore
	// debugWg tracks in-flight debug recording goroutines for graceful shutdown
	debugWg sync.WaitGroup
	// debugSeqID provides fallback correlation IDs when TraceID is not available
	debugSeqID atomic.Uint64

	// Observability (follows framework design principles)
	telemetry core.Telemetry // For metrics and tracing
	logger    core.Logger    // For structured logging

	// Metrics and history
	metrics      *OrchestratorMetrics
	history      []ExecutionRecord
	historyMutex sync.RWMutex
	metricsMutex sync.RWMutex

	// Context for background operations
	ctx    context.Context
	cancel context.CancelFunc

	// HITL (Human-in-the-Loop) support
	// When set, enables human oversight at plan/step execution points
	interruptController InterruptController
}

// NewAIOrchestrator creates a new AI-powered orchestrator
func NewAIOrchestrator(config *OrchestratorConfig, discovery core.Discovery, aiClient core.AIClient) *AIOrchestrator {
	// Immediate startup marker - uses stderr for guaranteed output
	log.Printf("[GOMIND-ORCH-V2] NewAIOrchestrator starting - EnableHybridResolution=%v", config != nil && config.EnableHybridResolution)

	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())
	catalog := NewAgentCatalog(discovery)

	o := &AIOrchestrator{
		config:      config,
		discovery:   discovery,
		aiClient:    aiClient,
		catalog:     catalog,
		executor:    NewSmartExecutor(catalog),
		synthesizer: NewAISynthesizer(aiClient),
		metrics:     &OrchestratorMetrics{},
		history:     make([]ExecutionRecord, 0, config.HistorySize),
		ctx:         ctx,
		cancel:      cancel,
		// Default to no-op telemetry
		telemetry: &core.NoOpTelemetry{},
	}

	// Initialize capability provider based on configuration
	switch config.CapabilityProviderType {
	case "service":
		// Use service-based provider for large-scale deployments
		if config.EnableFallback {
			// Use default provider as fallback for graceful degradation
			config.CapabilityService.FallbackProvider = NewDefaultCapabilityProvider(catalog)
		}
		o.capabilityProvider = NewServiceCapabilityProvider(&config.CapabilityService)
	default:
		// Default to catalog-based provider (sends all capabilities to LLM)
		// This is the quick-start default that works without additional setup
		o.capabilityProvider = NewDefaultCapabilityProvider(catalog)
	}

	// Layer 3: Wire up validation feedback if enabled
	if config.ExecutionOptions.ValidationFeedbackEnabled {
		o.executor.SetCorrectionCallback(o.requestParameterCorrection)
		o.executor.SetValidationFeedback(true, config.ExecutionOptions.MaxValidationRetries)
	}

	// Configure hybrid parameter resolution if enabled
	// This uses auto-wiring (schema-based) + micro-resolution (LLM fallback) for parameter binding
	if config.EnableHybridResolution {
		hybridResolver := NewHybridResolver(aiClient, nil) // Logger will be set later via SetLogger
		o.executor.SetHybridResolver(hybridResolver)
		o.executor.EnableHybridResolution(true)
		// Debug log to confirm hybrid resolution was configured
		fmt.Printf("[ORCHESTRATOR] Hybrid resolution enabled: hybridResolver=%v, useHybridResolution=true\n", hybridResolver != nil)
	} else {
		fmt.Printf("[ORCHESTRATOR] Hybrid resolution DISABLED in config (EnableHybridResolution=%v)\n", config.EnableHybridResolution)
	}

	// Layer 4: Wire up Semantic Retry (Contextual Re-Resolution) if enabled
	// This enables the executor to compute derived values when ErrorAnalyzer says "cannot fix"
	// but source data from dependencies is available. See SEMANTIC_RETRY_DESIGN.md for details.
	if config.SemanticRetry.Enabled {
		reResolver := NewContextualReResolver(aiClient, nil) // Logger will be set later via SetLogger
		o.executor.SetContextualReResolver(reResolver)
		o.executor.SetMaxSemanticRetries(config.SemanticRetry.MaxAttempts)
		o.executor.SetSemanticRetryForIndependentSteps(config.SemanticRetry.EnableForIndependentSteps)
	}

	// Wire step completion callback for async progress reporting (v1 addition)
	// This enables async task handlers to receive per-tool progress updates.
	// See notes/ASYNC_TASK_DESIGN.md Phase 6 for details.
	if config.ExecutionOptions.OnStepComplete != nil {
		o.executor.SetOnStepComplete(config.ExecutionOptions.OnStepComplete)
	}

	return o
}

// Start initializes the orchestrator and starts background processes
func (o *AIOrchestrator) Start(ctx context.Context) error {
	// Initial catalog refresh
	if err := o.catalog.Refresh(ctx); err != nil {
		return fmt.Errorf("failed to initialize catalog: %w", err)
	}

	// Start background catalog refresh
	go o.catalogRefreshLoop()

	return nil
}

// Stop gracefully shuts down the orchestrator
func (o *AIOrchestrator) Stop() {
	o.cancel()
}

// SetCapabilityProvider sets a custom capability provider
func (o *AIOrchestrator) SetCapabilityProvider(provider CapabilityProvider) {
	o.capabilityProvider = provider
}

// SetTelemetry sets the telemetry provider (integrates with framework telemetry module)
func (o *AIOrchestrator) SetTelemetry(telemetry core.Telemetry) {
	if telemetry == nil {
		o.telemetry = &core.NoOpTelemetry{}
	} else {
		o.telemetry = telemetry
	}
	// TODO: Propagate telemetry to executor and synthesizer when they support SetTelemetry()
	// - o.executor.SetTelemetry(telemetry) - when implemented
	// - o.synthesizer.SetTelemetry(telemetry) - when implemented
}

// SetLogger sets the logger provider (follows framework design principles)
// The component is always set to "framework/orchestration" to ensure proper log attribution
// regardless of which agent or tool is using the orchestration module.
func (o *AIOrchestrator) SetLogger(logger core.Logger) {
	if logger == nil {
		o.logger = &core.NoOpLogger{}
	} else {
		if cal, ok := logger.(core.ComponentAwareLogger); ok {
			o.logger = cal.WithComponent("framework/orchestration")
		} else {
			o.logger = logger
		}
	}

	// Propagate logger to sub-components (they will apply their own WithComponent)
	if o.executor != nil {
		o.executor.SetLogger(logger)
	}
	if o.catalog != nil {
		o.catalog.SetLogger(logger)
	}
	// TODO: Add synthesizer.SetLogger(logger) when synthesizer supports logging interface
}

// SetPromptBuilder allows runtime injection of a custom prompt builder.
// This follows the existing pattern used by SetCapabilityProvider.
//
// Use cases:
//   - Layer 1: DefaultPromptBuilder with additional type rules
//   - Layer 2: TemplatePromptBuilder for structural customization
//   - Layer 3: Custom PromptBuilder for full control (compliance, audit logging)
func (o *AIOrchestrator) SetPromptBuilder(builder PromptBuilder) {
	if builder != nil {
		o.promptBuilder = builder
		if o.logger != nil {
			o.logger.Info("PromptBuilder updated at runtime", map[string]interface{}{
				"operation": "set_prompt_builder",
			})
		}
	}
}

// SetErrorAnalyzer configures the LLM-based error analyzer for the executor.
// When set, the executor uses LLM to analyze errors and determine if they can be
// fixed with different parameters. This removes the need for tools to set Retryable flags.
// See PARAMETER_BINDING_FIX.md for the complete design rationale.
func (o *AIOrchestrator) SetErrorAnalyzer(analyzer *ErrorAnalyzer) {
	if o.executor != nil && analyzer != nil {
		o.executor.SetErrorAnalyzer(analyzer)
		if o.logger != nil {
			o.logger.Info("Error analyzer configured", map[string]interface{}{
				"operation": "set_error_analyzer",
			})
		}
	}
}

// SetLLMDebugStore sets the LLM debug store for full payload visibility.
// When configured, complete LLM prompts and responses are stored for debugging.
// This enables operators to see exactly what was sent to and received from the LLM.
// The store is propagated to all sub-components that make LLM calls:
// synthesizer, micro_resolver, and contextual_re_resolver.
// See orchestration/notes/LLM_DEBUG_PAYLOAD_DESIGN.md for design rationale.
func (o *AIOrchestrator) SetLLMDebugStore(store LLMDebugStore) {
	if store == nil {
		return
	}

	o.debugStore = store

	// Propagate to synthesizer
	if o.synthesizer != nil {
		o.synthesizer.SetLLMDebugStore(store)
	}

	// Propagate to executor's sub-components
	if o.executor != nil {
		// Propagate to HybridResolver's MicroResolver
		if o.executor.hybridResolver != nil && o.executor.hybridResolver.microResolver != nil {
			o.executor.hybridResolver.microResolver.SetLLMDebugStore(store)
		}
		// Propagate to ContextualReResolver
		if o.executor.contextualReResolver != nil {
			o.executor.contextualReResolver.SetLLMDebugStore(store)
		}
	}

	if o.logger != nil {
		o.logger.Info("LLM debug store configured", map[string]interface{}{
			"operation": "set_llm_debug_store",
		})
	}
}

// GetLLMDebugStore returns the configured LLM debug store (for API handlers).
func (o *AIOrchestrator) GetLLMDebugStore() LLMDebugStore {
	return o.debugStore
}

// SetInterruptController sets the HITL interrupt controller.
// When set, enables human oversight at plan/step execution points.
// The controller is propagated to the executor for step-level checks.
func (o *AIOrchestrator) SetInterruptController(controller InterruptController) {
	if controller == nil {
		return
	}

	o.interruptController = controller

	// Propagate to executor for step-level HITL checks
	if o.executor != nil {
		o.executor.SetInterruptController(controller)
	}

	if o.logger != nil {
		o.logger.Info("HITL interrupt controller configured", map[string]interface{}{
			"operation": "set_interrupt_controller",
		})
	}
}

// GetInterruptController returns the configured interrupt controller (for API handlers).
func (o *AIOrchestrator) GetInterruptController() InterruptController {
	return o.interruptController
}

// recordDebugInteraction stores an LLM interaction for debugging.
// Runs asynchronously to avoid blocking orchestration. Errors are logged, not propagated.
// Uses WaitGroup to track in-flight recordings for graceful shutdown.
// This follows FRAMEWORK_DESIGN_PRINCIPLES.md: Resilient Runtime Behavior.
func (o *AIOrchestrator) recordDebugInteraction(ctx context.Context, requestID string, interaction LLMInteraction) {
	if o.debugStore == nil {
		return
	}

	// Track this goroutine for graceful shutdown
	o.debugWg.Add(1)

	// Run async to avoid blocking orchestration
	go func() {
		defer o.debugWg.Done()

		// Use original context with timeout to preserve baggage (original_request_id).
		// This allows LLM debug records from HITL resume requests to be correlated
		// with the original conversation's request_id.
		recordCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := o.debugStore.RecordInteraction(recordCtx, requestID, interaction); err != nil {
			// Log but don't fail - debug is observability, not critical path
			if o.logger != nil {
				o.logger.Warn("Failed to record LLM debug interaction", map[string]interface{}{
					"request_id": requestID,
					"type":       interaction.Type,
					"error":      err.Error(),
				})
			}
		}
	}()
}

// Shutdown gracefully shuts down the orchestrator, waiting for pending debug recordings.
// This follows FRAMEWORK_DESIGN_PRINCIPLES.md: Component Lifecycle Rules.
func (o *AIOrchestrator) Shutdown(ctx context.Context) error {
	// Stop background operations first
	o.cancel()

	// Wait for pending debug recordings with timeout
	done := make(chan struct{})
	go func() {
		o.debugWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if o.logger != nil {
			o.logger.Info("Orchestrator shutdown complete", map[string]interface{}{
				"operation": "shutdown",
			})
		}
		return nil
	case <-ctx.Done():
		if o.logger != nil {
			o.logger.Warn("Orchestrator shutdown timed out, some debug recordings may be lost", map[string]interface{}{
				"operation": "shutdown",
				"error":     ctx.Err().Error(),
			})
		}
		return ctx.Err()
	}
}

// generateFallbackRequestID generates a request ID when TraceID is not available.
// Uses atomic counter for uniqueness.
func (o *AIOrchestrator) generateFallbackRequestID() string {
	seq := o.debugSeqID.Add(1)
	return fmt.Sprintf("debug-%d-%d", time.Now().UnixNano(), seq)
}

// requestParameterCorrection asks the LLM to fix parameters based on type error feedback.
// This is the Layer 3 (Validation Feedback) mechanism that enables recovery from type errors
// that slip through Layers 1 and 2.
//
// The method constructs a correction prompt that includes:
//   - Original parameters that caused the error
//   - Error message from the tool
//   - Expected parameter schema from the capability definition
//
// Returns corrected parameters or an error if correction fails.
func (o *AIOrchestrator) requestParameterCorrection(
	ctx context.Context,
	step RoutingStep,
	originalParams map[string]interface{},
	errorMessage string,
	capabilitySchema *EnhancedCapability,
) (map[string]interface{}, error) {
	if o.aiClient == nil {
		return nil, fmt.Errorf("AI client not available for parameter correction")
	}

	// Build schema JSON for the prompt
	var schemaJSON []byte
	if capabilitySchema != nil && len(capabilitySchema.Parameters) > 0 {
		schemaJSON, _ = json.MarshalIndent(capabilitySchema.Parameters, "", "  ")
	}
	paramsJSON, _ := json.MarshalIndent(originalParams, "", "  ")

	// Build the correction prompt
	correctionPrompt := fmt.Sprintf(`The following tool call failed with a type error. Please fix the parameters.

Tool: %s
Capability: %s
Error: %s

Original Parameters (INCORRECT - caused the error above):
%s

Expected Parameter Schema:
%s

CRITICAL RULES for correction:
1. Numbers (type: number, float64, integer, int) must NOT be in quotes
   CORRECT: "lat": 35.6897
   WRONG:   "lat": "35.6897"

2. Booleans (type: boolean, bool) must NOT be in quotes
   CORRECT: "enabled": true
   WRONG:   "enabled": "true"

3. Only strings should be quoted

Respond with ONLY the corrected JSON parameters object. No explanation, no markdown, just the JSON object.`,
		step.AgentName,
		step.Metadata["capability"],
		errorMessage,
		string(paramsJSON),
		string(schemaJSON),
	)

	if o.logger != nil {
		o.logger.DebugWithContext(ctx, "Requesting LLM parameter correction", map[string]interface{}{
			"operation":  "layer3_correction_request",
			"step_id":    step.StepID,
			"capability": step.Metadata["capability"],
		})
	}

	// Get request ID from context baggage for debug correlation
	requestID := ""
	if baggage := telemetry.GetBaggage(ctx); baggage != nil {
		requestID = baggage["request_id"]
	}
	if requestID == "" {
		requestID = o.generateFallbackRequestID()
	}

	// Call LLM for correction
	llmStartTime := time.Now()
	response, err := o.aiClient.GenerateResponse(ctx, correctionPrompt, nil)
	llmDuration := time.Since(llmStartTime)

	if err != nil {
		// LLM Debug: Record failed correction attempt
		o.recordDebugInteraction(ctx, requestID, LLMInteraction{
			Type:       "correction",
			Timestamp:  llmStartTime,
			DurationMs: llmDuration.Milliseconds(),
			Prompt:     correctionPrompt,
			Success:    false,
			Error:      err.Error(),
			Attempt:    1,
		})
		return nil, fmt.Errorf("LLM correction request failed: %w", err)
	}

	// LLM Debug: Record successful correction attempt
	o.recordDebugInteraction(ctx, requestID, LLMInteraction{
		Type:             "correction",
		Timestamp:        llmStartTime,
		DurationMs:       llmDuration.Milliseconds(),
		Prompt:           correctionPrompt,
		Response:         response.Content,
		Model:            response.Model,
		Provider:         response.Provider,
		PromptTokens:     response.Usage.PromptTokens,
		CompletionTokens: response.Usage.CompletionTokens,
		TotalTokens:      response.Usage.TotalTokens,
		Success:          true,
		Attempt:          1,
	})

	// Extract JSON from response (handle potential markdown wrapping)
	content := response.Content
	content = extractJSON(content)

	// Parse corrected parameters
	var correctedParams map[string]interface{}
	if err := json.Unmarshal([]byte(content), &correctedParams); err != nil {
		return nil, fmt.Errorf("failed to parse corrected parameters: %w", err)
	}

	if o.logger != nil {
		o.logger.DebugWithContext(ctx, "LLM parameter correction successful", map[string]interface{}{
			"operation":        "layer3_correction_success",
			"step_id":          step.StepID,
			"corrected_params": correctedParams,
		})
	}

	return correctedParams, nil
}

// extractJSON attempts to extract a JSON object from text that might be wrapped in markdown.
func extractJSON(text string) string {
	// Trim whitespace
	text = strings.TrimSpace(text)

	// Check for markdown code blocks
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
		if idx := strings.Index(text, "```"); idx != -1 {
			text = text[:idx]
		}
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		if idx := strings.Index(text, "```"); idx != -1 {
			text = text[:idx]
		}
	}

	return strings.TrimSpace(text)
}

// catalogRefreshLoop periodically refreshes the agent catalog
func (o *AIOrchestrator) catalogRefreshLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := o.catalog.Refresh(o.ctx); err != nil {
				// Log error but continue
				if o.logger != nil {
					o.logger.Error("Catalog refresh error", map[string]interface{}{
						"operation": "catalog_refresh",
						"error":     err.Error(),
					})
				}
			}
		case <-o.ctx.Done():
			return
		}
	}
}

// ProcessRequest handles a natural language request using AI-powered orchestration
func (o *AIOrchestrator) ProcessRequest(ctx context.Context, request string, metadata map[string]interface{}) (*OrchestratorResponse, error) {
	// Start telemetry span if telemetry is available
	var span core.Span
	if o.telemetry != nil {
		ctx, span = o.telemetry.StartSpan(ctx, "orchestrator.process_request")
		defer span.End()
	} else {
		// Create a no-op span
		span = &core.NoOpSpan{}
	}

	startTime := time.Now()
	requestID := generateRequestID()

	// Add request_id to context baggage so downstream components (AI client, etc.)
	// can access it via telemetry.GetBaggage() and include it in their logs
	ctx = telemetry.WithBaggage(ctx, "request_id", requestID)

	// Set original_request_id for trace correlation across HITL resumes.
	// On initial requests: original_request_id = request_id (same value)
	// On resume requests: original_request_id is already set via header, don't overwrite
	if bag := telemetry.GetBaggage(ctx); bag == nil || bag["original_request_id"] == "" {
		ctx = telemetry.WithBaggage(ctx, "original_request_id", requestID)
	}

	// Store metadata in context for HITL checkpoint creation
	// This preserves session_id, user_id, etc. when creating checkpoints
	ctx = WithMetadata(ctx, metadata)

	if o.logger != nil {
		o.logger.InfoWithContext(ctx, "Starting request processing", map[string]interface{}{
			"operation":      "process_request",
			"request_id":     requestID,
			"request_length": len(request),
			"metadata_keys":  getMapKeys(metadata),
		})
	}

	if span != nil {
		span.SetAttribute("request_id", requestID)
		span.SetAttribute("request_length", len(request))
		// Set original_request_id for trace correlation - will be same as request_id on initial,
		// or the original value on resumes (preserved from baggage set by handler)
		if bag := telemetry.GetBaggage(ctx); bag != nil && bag["original_request_id"] != "" {
			span.SetAttribute("original_request_id", bag["original_request_id"])
		}
	}

	// Record metric for request count if telemetry is available
	if o.telemetry != nil {
		o.telemetry.RecordMetric("orchestrator.requests.total", 1, map[string]string{
			"mode": string(o.config.RoutingMode),
		})
	}

	// Step 1: Get execution plan (from override or LLM)
	// Check for plan override first (HITL resume flow uses stored plan)
	plan := GetPlanOverride(ctx)
	if plan != nil {
		// Resume flow: use stored plan from checkpoint
		if o.logger != nil {
			o.logger.InfoWithContext(ctx, "Using plan override from checkpoint", map[string]interface{}{
				"operation":  "hitl_resume_plan_override",
				"request_id": requestID,
				"plan_id":    plan.PlanID,
				"step_count": len(plan.Steps),
			})
		}
		if span != nil {
			span.SetAttribute("plan_override", true)
		}
	} else {
		// Normal flow: generate plan via LLM
		var err error
		plan, err = o.generateExecutionPlan(ctx, request, requestID)
		if err != nil {
			if o.logger != nil {
				o.logger.ErrorWithContext(ctx, "Plan generation failed", map[string]interface{}{
					"operation":   "plan_generation",
					"request_id":  requestID,
					"error":       err.Error(),
					"duration_ms": time.Since(startTime).Milliseconds(),
				})
			}
			if span != nil {
				span.RecordError(err)
			}
			if o.telemetry != nil {
				o.telemetry.RecordMetric("orchestrator.requests.failed", 1, map[string]string{
					"stage": "planning",
				})
			}
			o.updateMetrics(time.Since(startTime), false)
			return nil, fmt.Errorf("failed to generate execution plan: %w", err)
		}
	}

	if o.logger != nil {
		// Extract tools and build plan summary for INFO level
		toolsUsed := o.extractAgentsFromPlan(plan)
		parallelGroups := countParallelGroups(plan)

		o.logger.InfoWithContext(ctx, "Plan generated successfully", map[string]interface{}{
			"operation":          "plan_generation",
			"request_id":         requestID,
			"plan_id":            plan.PlanID,
			"step_count":         len(plan.Steps),
			"tools_selected":     toolsUsed,
			"parallel_groups":    parallelGroups,
			"generation_time_ms": time.Since(startTime).Milliseconds(),
		})

		// Log full plan structure at DEBUG level
		o.logger.DebugWithContext(ctx, "Plan structure details", map[string]interface{}{
			"operation":  "plan_structure",
			"request_id": requestID,
			"plan_id":    plan.PlanID,
			"steps":      formatPlanSteps(plan),
		})
	}

	if span != nil {
		span.SetAttribute("plan_steps", len(plan.Steps))
	}

	// Step 2: Validate the plan
	if err := o.validatePlan(plan); err != nil {
		// Try to regenerate with error feedback
		plan, err = o.regeneratePlan(ctx, request, requestID, err)
		if err != nil {
			o.updateMetrics(time.Since(startTime), false)
			return nil, fmt.Errorf("failed to generate valid plan: %w", err)
		}
	}

	// Step 2.5: HITL Plan Approval Check
	// If HITL is enabled and interrupt controller is set, check if plan needs approval
	if o.config.HITL.Enabled && o.interruptController != nil {
		// Add request_id to context for HITL controller
		hitlCtx := WithRequestID(ctx, requestID)
		checkpoint, err := o.interruptController.CheckPlanApproval(hitlCtx, plan)
		if err != nil {
			// Fail-fast: HITL check failure halts execution
			// This ensures sensitive operations cannot bypass approval due to HITL system issues
			if o.logger != nil {
				o.logger.ErrorWithContext(ctx, "HITL plan approval check failed", map[string]interface{}{
					"operation":  "hitl_plan_approval",
					"request_id": requestID,
					"plan_id":    plan.PlanID,
					"error":      err.Error(),
				})
			}
			if span != nil {
				span.RecordError(err)
			}
			o.updateMetrics(time.Since(startTime), false)
			return nil, fmt.Errorf("HITL plan check failed: %w", err)
		}
		if checkpoint != nil {
			// Plan requires human approval - return interrupt error
			if o.logger != nil {
				o.logger.InfoWithContext(ctx, "Plan execution interrupted for human approval", map[string]interface{}{
					"operation":     "hitl_plan_approval",
					"request_id":    requestID,
					"plan_id":       plan.PlanID,
					"checkpoint_id": checkpoint.CheckpointID,
				})
			}
			if span != nil {
				span.SetAttribute("hitl.interrupted", true)
				span.SetAttribute("hitl.checkpoint_id", checkpoint.CheckpointID)
			}
			return nil, &ErrInterrupted{
				CheckpointID: checkpoint.CheckpointID,
				Checkpoint:   checkpoint,
			}
		}
	}

	// Step 3: Execute the plan
	result, err := o.executor.Execute(ctx, plan)
	if err != nil {
		// Check for step-level HITL interrupt - propagate directly without wrapping
		if IsInterrupted(err) {
			if o.logger != nil {
				checkpoint := GetCheckpoint(err)
				logFields := map[string]interface{}{
					"operation":     "plan_execution_hitl_interrupt",
					"request_id":    requestID,
					"checkpoint_id": GetCheckpointID(err),
				}
				if checkpoint != nil && checkpoint.CurrentStep != nil {
					logFields["step_id"] = checkpoint.CurrentStep.StepID
				}
				o.logger.InfoWithContext(ctx, "Step-level HITL interrupt, returning to agent", logFields)
			}
			// Add span event and attributes for step-level HITL (visible in Jaeger)
			checkpoint := GetCheckpoint(err)
			if checkpoint != nil {
				telemetry.AddSpanEvent(ctx, "hitl.step_interrupt.orchestrator_propagating",
					attribute.String("checkpoint_id", GetCheckpointID(err)),
					attribute.String("interrupt_point", string(checkpoint.InterruptPoint)),
				)
				span.SetAttribute("hitl.interrupted", true)
				span.SetAttribute("hitl.checkpoint_id", GetCheckpointID(err))
				span.SetAttribute("hitl.interrupt_point", string(checkpoint.InterruptPoint))
			}
			return nil, err // Return ErrInterrupted directly
		}
		if o.logger != nil {
			o.logger.ErrorWithContext(ctx, "Plan execution failed", map[string]interface{}{
				"operation":   "plan_execution",
				"request_id":  requestID,
				"plan_id":     plan.PlanID,
				"error":       err.Error(),
				"duration_ms": time.Since(startTime).Milliseconds(),
			})
		}
		o.updateMetrics(time.Since(startTime), false)
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	if o.logger != nil {
		failedSteps := 0
		if result != nil && !result.Success {
			// Count failed steps from result
			for _, step := range result.Steps {
				if !step.Success {
					failedSteps++
				}
			}
		}
		o.logger.InfoWithContext(ctx, "Plan execution completed", map[string]interface{}{
			"operation":    "plan_execution",
			"request_id":   requestID,
			"plan_id":      plan.PlanID,
			"success":      result != nil && result.Success,
			"failed_steps": failedSteps,
			"duration_ms":  time.Since(startTime).Milliseconds(),
		})
	}

	// Step 4: Synthesize results using AI
	synthesizedResponse, err := o.synthesizer.Synthesize(ctx, request, result)
	if err != nil {
		o.updateMetrics(time.Since(startTime), false)
		return nil, fmt.Errorf("synthesis failed: %w", err)
	}

	// Build response
	response := &OrchestratorResponse{
		RequestID:       requestID,
		OriginalRequest: request,
		Response:        synthesizedResponse,
		RoutingMode:     o.config.RoutingMode,
		ExecutionTime:   time.Since(startTime),
		AgentsInvolved:  o.extractAgentsFromPlan(plan),
		Metadata:        metadata,
		Confidence:      0.95, // TODO: Calculate based on execution success
	}

	// Update metrics and history
	o.updateMetrics(response.ExecutionTime, true)
	o.addToHistory(response)

	if o.logger != nil {
		o.logger.InfoWithContext(ctx, "Request processing completed successfully", map[string]interface{}{
			"operation":         "process_request_complete",
			"request_id":        requestID,
			"success":           true,
			"total_duration_ms": time.Since(startTime).Milliseconds(),
		})
	}

	// Record success metrics if telemetry is available
	if o.telemetry != nil {
		o.telemetry.RecordMetric("orchestrator.requests.success", 1, map[string]string{
			"mode": string(o.config.RoutingMode),
		})
		o.telemetry.RecordMetric("orchestrator.latency_ms", float64(time.Since(startTime).Milliseconds()), map[string]string{
			"operation": "process_request",
		})
	}

	return response, nil
}

// ProcessRequestStreaming processes a request with streaming response
// This enables real-time token-by-token delivery for chat-based applications.
//
// The streaming flow:
// 1. Generate execution plan (same as ProcessRequest)
// 2. Execute plan steps (same as ProcessRequest)
// 3. Stream the synthesis response token-by-token via callback
//
// Parameters:
//   - ctx: Context for cancellation and tracing
//   - request: Natural language request
//   - metadata: Additional request metadata
//   - callback: Function called for each streaming chunk
//
// Returns:
//   - StreamingOrchestratorResponse with final state and metrics
//   - error if processing fails before streaming starts
func (o *AIOrchestrator) ProcessRequestStreaming(
	ctx context.Context,
	request string,
	metadata map[string]interface{},
	callback core.StreamCallback,
) (*StreamingOrchestratorResponse, error) {
	startTime := time.Now()
	// Use custom prefix if configured, otherwise default to "orch"
	prefix := "orch"
	if o.config.RequestIDPrefix != "" {
		prefix = o.config.RequestIDPrefix
	}
	requestID := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())

	// Add request_id to context baggage so downstream components (AI client, etc.)
	// can access it via telemetry.GetBaggage() and include it in their logs
	ctx = telemetry.WithBaggage(ctx, "request_id", requestID)

	// Set original_request_id for trace correlation across HITL resumes.
	// On initial requests: original_request_id = request_id (same value)
	// On resume requests: original_request_id is already set via header, don't overwrite
	if bag := telemetry.GetBaggage(ctx); bag == nil || bag["original_request_id"] == "" {
		ctx = telemetry.WithBaggage(ctx, "original_request_id", requestID)
	}

	// Add request_id to context for GetRequestID() - used by HITL controller
	// when creating checkpoints during execution (e.g., step-level interrupts)
	ctx = WithRequestID(ctx, requestID)

	// Store metadata in context for HITL checkpoint creation
	// This preserves session_id, user_id, etc. when creating checkpoints
	ctx = WithMetadata(ctx, metadata)

	// Start tracing span (nil-safe per FRAMEWORK_DESIGN_PRINCIPLES.md)
	var span core.Span
	if o.telemetry != nil {
		ctx, span = o.telemetry.StartSpan(ctx, "orchestrator.process_request_streaming")
		defer span.End()
	} else {
		span = &core.NoOpSpan{}
	}

	span.SetAttribute("request_id", requestID)
	span.SetAttribute("streaming", true)
	// Set original_request_id for trace correlation - will be same as request_id on initial,
	// or the original value on resumes (preserved from baggage set by handler)
	if bag := telemetry.GetBaggage(ctx); bag != nil && bag["original_request_id"] != "" {
		span.SetAttribute("original_request_id", bag["original_request_id"])
	}

	// Check if AI client supports streaming
	streamingClient, ok := o.aiClient.(core.StreamingAIClient)
	if !ok || !streamingClient.SupportsStreaming() {
		// Fall back to non-streaming and simulate streaming
		if o.logger != nil {
			o.logger.WarnWithContext(ctx, "AI client does not support streaming, using simulated streaming", map[string]interface{}{
				"operation":  "streaming_fallback",
				"request_id": requestID,
			})
		}

		// Use regular ProcessRequest and chunk the response
		response, err := o.ProcessRequest(ctx, request, metadata)
		if err != nil {
			return nil, err
		}

		// Simulate streaming by chunking the response
		chunkSize := 50
		chunkIndex := 0
		for i := 0; i < len(response.Response); i += chunkSize {
			end := i + chunkSize
			if end > len(response.Response) {
				end = len(response.Response)
			}

			chunk := core.StreamChunk{
				Content: response.Response[i:end],
				Delta:   true,
				Index:   chunkIndex,
			}
			chunkIndex++

			if err := callback(chunk); err != nil {
				// Callback requested stop
				return &StreamingOrchestratorResponse{
					OrchestratorResponse: OrchestratorResponse{
						RequestID:       requestID,
						OriginalRequest: request,
						Response:        response.Response[:end],
						RoutingMode:     o.config.RoutingMode,
						ExecutionTime:   time.Since(startTime),
						AgentsInvolved:  response.AgentsInvolved,
						Metadata:        response.Metadata,
						Confidence:      response.Confidence,
					},
					ChunksDelivered: chunkIndex,
					StreamCompleted: false,
					PartialContent:  true,
					FinishReason:    "cancelled",
				}, nil
			}
		}

		// Send final chunk
		finalChunk := core.StreamChunk{
			Delta:        false,
			Index:        chunkIndex,
			FinishReason: "stop",
		}
		_ = callback(finalChunk)

		return &StreamingOrchestratorResponse{
			OrchestratorResponse: *response,
			ChunksDelivered:      chunkIndex,
			StreamCompleted:      true,
			PartialContent:       false,
			FinishReason:         "stop",
		}, nil
	}

	// Generate execution plan (or use override from HITL resume)
	plan := GetPlanOverride(ctx)
	if plan != nil {
		// Resume flow: use stored plan from checkpoint
		if o.logger != nil {
			o.logger.InfoWithContext(ctx, "Using plan override from checkpoint", map[string]interface{}{
				"operation":  "hitl_resume_plan_override",
				"request_id": requestID,
				"plan_id":    plan.PlanID,
				"step_count": len(plan.Steps),
			})
		}
		span.SetAttribute("plan_override", true)
	} else {
		// Normal flow: generate plan via LLM
		var err error
		plan, err = o.generateExecutionPlan(ctx, request, requestID)
		if err != nil {
			if o.logger != nil {
				o.logger.ErrorWithContext(ctx, "Plan generation failed", map[string]interface{}{
					"operation":  "streaming_plan_error",
					"request_id": requestID,
					"error":      err.Error(),
				})
			}
			return nil, fmt.Errorf("failed to generate execution plan: %w", err)
		}
	}

	// HITL Plan Approval Check (streaming mode)
	// Mirror of ProcessRequest HITL check - ensures streaming requests also respect human oversight
	if o.config.HITL.Enabled && o.interruptController != nil {
		// Add request_id to context for HITL controller
		hitlCtx := WithRequestID(ctx, requestID)
		checkpoint, err := o.interruptController.CheckPlanApproval(hitlCtx, plan)
		if err != nil {
			// Fail-fast: HITL check failure halts execution
			// This ensures sensitive operations cannot bypass approval due to HITL system issues
			if o.logger != nil {
				o.logger.ErrorWithContext(ctx, "HITL plan approval check failed", map[string]interface{}{
					"operation":  "streaming_hitl_plan_approval",
					"request_id": requestID,
					"plan_id":    plan.PlanID,
					"error":      err.Error(),
				})
			}
			span.RecordError(err)
			return nil, fmt.Errorf("HITL plan check failed: %w", err)
		}
		if checkpoint != nil {
			// Plan requires human approval - return interrupt error
			if o.logger != nil {
				o.logger.InfoWithContext(ctx, "Streaming execution interrupted for human approval", map[string]interface{}{
					"operation":     "streaming_hitl_plan_approval",
					"request_id":    requestID,
					"plan_id":       plan.PlanID,
					"checkpoint_id": checkpoint.CheckpointID,
				})
			}
			span.SetAttribute("hitl.interrupted", true)
			span.SetAttribute("hitl.checkpoint_id", checkpoint.CheckpointID)
			return nil, &ErrInterrupted{
				CheckpointID: checkpoint.CheckpointID,
				Checkpoint:   checkpoint,
			}
		}
	}

	// Execute the plan
	result, err := o.executor.Execute(ctx, plan)
	if err != nil {
		// Check for step-level HITL interrupt - propagate directly without wrapping
		if IsInterrupted(err) {
			if o.logger != nil {
				checkpoint := GetCheckpoint(err)
				logFields := map[string]interface{}{
					"operation":     "streaming_execution_hitl_interrupt",
					"request_id":    requestID,
					"checkpoint_id": GetCheckpointID(err),
				}
				if checkpoint != nil && checkpoint.CurrentStep != nil {
					logFields["step_id"] = checkpoint.CurrentStep.StepID
				}
				o.logger.InfoWithContext(ctx, "Step-level HITL interrupt, returning to agent", logFields)
			}
			// Add span event and attributes for step-level HITL (visible in Jaeger)
			checkpoint := GetCheckpoint(err)
			if checkpoint != nil {
				telemetry.AddSpanEvent(ctx, "hitl.step_interrupt.orchestrator_propagating",
					attribute.String("checkpoint_id", GetCheckpointID(err)),
					attribute.String("interrupt_point", string(checkpoint.InterruptPoint)),
				)
				span.SetAttribute("hitl.interrupted", true)
				span.SetAttribute("hitl.checkpoint_id", GetCheckpointID(err))
				span.SetAttribute("hitl.interrupt_point", string(checkpoint.InterruptPoint))
			}
			return nil, err // Return ErrInterrupted directly
		}
		if o.logger != nil {
			o.logger.ErrorWithContext(ctx, "Plan execution failed", map[string]interface{}{
				"operation":  "streaming_execution_error",
				"request_id": requestID,
				"error":      err.Error(),
			})
		}
		return nil, fmt.Errorf("failed to execute plan: %w", err)
	}

	// Build synthesis prompt
	synthesisPrompt := o.buildSynthesisPrompt(request, result)

	// Collect agents involved before streaming
	agentsInvolved := make([]string, 0, len(result.Steps))
	for _, step := range result.Steps {
		agentsInvolved = append(agentsInvolved, step.AgentName)
	}

	// Stream the synthesis response
	// Capture start time for LLM debug recording
	synthesisStart := time.Now()
	systemPrompt := "You are a helpful assistant synthesizing responses from multiple agents."

	var fullContent strings.Builder
	chunkIndex := 0
	var finishReason string
	streamCallback := func(chunk core.StreamChunk) error {
		if chunk.Content != "" {
			fullContent.WriteString(chunk.Content)
		}
		// Capture finish reason from final chunk
		if chunk.FinishReason != "" {
			finishReason = chunk.FinishReason
		}
		chunkIndex++
		return callback(chunk)
	}

	aiResponse, err := streamingClient.StreamResponse(ctx, synthesisPrompt, &core.AIOptions{
		Temperature:  0.7,
		MaxTokens:    2000,
		SystemPrompt: systemPrompt,
	}, streamCallback)

	// Handle streaming errors
	if err != nil {
		if err == core.ErrStreamPartiallyCompleted {
			// LLM Debug: Record partial streaming synthesis (interrupted but has content)
			o.recordDebugInteraction(ctx, requestID, LLMInteraction{
				Type:             "synthesis_streaming",
				Timestamp:        synthesisStart,
				DurationMs:       time.Since(synthesisStart).Milliseconds(),
				Prompt:           synthesisPrompt,
				SystemPrompt:     systemPrompt,
				Temperature:      0.7,
				MaxTokens:        2000,
				Model:            aiResponse.Model,
				Provider:         aiResponse.Provider,
				Response:         aiResponse.Content,
				PromptTokens:     aiResponse.Usage.PromptTokens,     // May be partial
				CompletionTokens: aiResponse.Usage.CompletionTokens, // May be partial
				TotalTokens:      aiResponse.Usage.TotalTokens,      // May be partial
				Success:          true,                              // Partial success - we have content
				Error:            "stream partially completed",
				Attempt:          1,
			})

			return &StreamingOrchestratorResponse{
				OrchestratorResponse: OrchestratorResponse{
					RequestID:       requestID,
					OriginalRequest: request,
					Response:        fullContent.String(),
					RoutingMode:     o.config.RoutingMode,
					ExecutionTime:   time.Since(startTime),
					AgentsInvolved:  agentsInvolved,
					Errors:          []string{"stream partially completed"},
					Confidence:      0.7,
				},
				ChunksDelivered: chunkIndex,
				StreamCompleted: false,
				PartialContent:  true,
				StepResults:     result.Steps,
				FinishReason:    "cancelled",
			}, nil
		}

		// LLM Debug: Record failed streaming synthesis
		o.recordDebugInteraction(ctx, requestID, LLMInteraction{
			Type:         "synthesis_streaming",
			Timestamp:    synthesisStart,
			DurationMs:   time.Since(synthesisStart).Milliseconds(),
			Prompt:       synthesisPrompt,
			SystemPrompt: systemPrompt,
			Temperature:  0.7,
			MaxTokens:    2000,
			Success:      false,
			Error:        err.Error(),
			Attempt:      1,
		})

		return nil, fmt.Errorf("synthesis streaming failed: %w", err)
	}

	// Build final response with all enhanced fields
	response := &StreamingOrchestratorResponse{
		OrchestratorResponse: OrchestratorResponse{
			RequestID:       requestID,
			OriginalRequest: request,
			Response:        aiResponse.Content,
			RoutingMode:     o.config.RoutingMode,
			ExecutionTime:   time.Since(startTime),
			AgentsInvolved:  agentsInvolved,
			Confidence:      0.9,
		},
		ChunksDelivered: chunkIndex,
		StreamCompleted: true,
		PartialContent:  false,
		StepResults:     result.Steps,
		Usage:           &aiResponse.Usage,
		FinishReason:    finishReason,
	}

	if o.logger != nil {
		o.logger.InfoWithContext(ctx, "Streaming request completed", map[string]interface{}{
			"operation":        "streaming_complete",
			"request_id":       requestID,
			"chunks_delivered": chunkIndex,
			"response_length":  len(aiResponse.Content),
			"duration_ms":      time.Since(startTime).Milliseconds(),
		})
	}

	// LLM Debug: Record successful streaming synthesis
	o.recordDebugInteraction(ctx, requestID, LLMInteraction{
		Type:             "synthesis_streaming",
		Timestamp:        synthesisStart,
		DurationMs:       time.Since(synthesisStart).Milliseconds(),
		Prompt:           synthesisPrompt,
		SystemPrompt:     systemPrompt,
		Temperature:      0.7,
		MaxTokens:        2000,
		Model:            aiResponse.Model,
		Provider:         aiResponse.Provider,
		Response:         aiResponse.Content,
		PromptTokens:     aiResponse.Usage.PromptTokens,
		CompletionTokens: aiResponse.Usage.CompletionTokens,
		TotalTokens:      aiResponse.Usage.TotalTokens,
		Success:          true,
		Attempt:          1,
	})

	return response, nil
}

// buildSynthesisPrompt creates the prompt for synthesizing agent responses
func (o *AIOrchestrator) buildSynthesisPrompt(request string, result *ExecutionResult) string {
	var sb strings.Builder
	sb.WriteString("User Request: ")
	sb.WriteString(request)
	sb.WriteString("\n\nAgent Responses:\n")

	for _, step := range result.Steps {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", step.AgentName, step.Response))
	}

	sb.WriteString("\nPlease synthesize these responses into a coherent, helpful answer for the user.")
	return sb.String()
}

// generateExecutionPlan uses LLM to create an execution plan
func (o *AIOrchestrator) generateExecutionPlan(ctx context.Context, request string, requestID string) (*RoutingPlan, error) {
	planGenStart := time.Now()

	if o.logger != nil {
		o.logger.DebugWithContext(ctx, "Starting plan generation", map[string]interface{}{
			"operation":  "plan_generation_start",
			"request_id": requestID,
		})
	}
	// Check if AI client is available
	if o.aiClient == nil {
		return nil, fmt.Errorf("AI client not configured")
	}

	// Inject requestID into context for child components (e.g., TieredCapabilityProvider)
	// to correlate their debug recordings with this orchestrator request.
	ctx = WithRequestID(ctx, requestID)

	// Build initial prompt with capability information
	prompt, err := o.buildPlanningPrompt(ctx, request)
	if err != nil {
		return nil, err
	}

	// Determine max attempts: 1 initial + retries (if enabled)
	maxAttempts := 1
	if o.config != nil && o.config.PlanParseRetryEnabled {
		maxAttempts = 1 + o.config.PlanParseMaxRetries
	}

	var lastParseErr error
	var totalTokensUsed int

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if o.logger != nil {
			o.logger.DebugWithContext(ctx, "LLM prompt constructed", map[string]interface{}{
				"operation":        "prompt_construction",
				"request_id":       requestID,
				"prompt_length":    len(prompt),
				"estimated_tokens": len(prompt) / 4, // Rough estimate: 4 chars per token
				"attempt":          attempt,
				"max_attempts":     maxAttempts,
			})
		}

		if o.logger != nil {
			o.logger.DebugWithContext(ctx, "Calling LLM for plan generation", map[string]interface{}{
				"operation":   "llm_call",
				"request_id":  requestID,
				"temperature": 0.3,
				"max_tokens":  2000,
				"attempt":     attempt,
			})
		}

		// Telemetry: Record LLM prompt for visibility in Jaeger
		telemetry.AddSpanEvent(ctx, "llm.plan_generation.request",
			attribute.String("request_id", requestID),
			attribute.String("prompt", truncateString(prompt, 2000)),
			attribute.Int("prompt_length", len(prompt)),
			attribute.Float64("temperature", 0.3),
			attribute.Int("max_tokens", 2000),
			attribute.Int("attempt", attempt),
		)

		// Call LLM
		llmStartTime := time.Now()
		aiResponse, err := o.aiClient.GenerateResponse(ctx, prompt, &core.AIOptions{
			Temperature:  0.3, // Lower temperature for more deterministic planning
			MaxTokens:    2000,
			SystemPrompt: "You are an intelligent orchestrator that creates execution plans for multi-agent systems.",
		})
		llmDuration := time.Since(llmStartTime)

		if err != nil {
			telemetry.AddSpanEvent(ctx, "llm.plan_generation.error",
				attribute.String("request_id", requestID),
				attribute.String("error", err.Error()),
				attribute.Int64("duration_ms", llmDuration.Milliseconds()),
				attribute.Int("attempt", attempt),
			)
			// Unified Metrics: Record failed AI request
			telemetry.RecordAIRequest(telemetry.ModuleOrchestration, "plan_generation",
				float64(llmDuration.Milliseconds()), "error")
			// Record overall plan generation failure (orchestration-local metrics)
			telemetry.Histogram("plan_generation.duration_ms", float64(time.Since(planGenStart).Milliseconds()),
				"module", telemetry.ModuleOrchestration, "status", "error")
			telemetry.Counter("plan_generation.total",
				"module", telemetry.ModuleOrchestration, "status", "error")

			// LLM Debug: Record failed interaction (includes prompt for debugging)
			o.recordDebugInteraction(ctx, requestID, LLMInteraction{
				Type:         "plan_generation",
				Timestamp:    llmStartTime,
				DurationMs:   llmDuration.Milliseconds(),
				Prompt:       prompt,
				SystemPrompt: "You are an intelligent orchestrator that creates execution plans for multi-agent systems.",
				Temperature:  0.3,
				MaxTokens:    2000,
				Success:      false,
				Error:        err.Error(),
				Attempt:      attempt,
			})

			return nil, err
		}

		totalTokensUsed += aiResponse.Usage.TotalTokens

		// Telemetry: Record LLM response for visibility in Jaeger
		telemetry.AddSpanEvent(ctx, "llm.plan_generation.response",
			attribute.String("request_id", requestID),
			attribute.String("response", truncateString(aiResponse.Content, 2000)),
			attribute.Int("response_length", len(aiResponse.Content)),
			attribute.Int("prompt_tokens", aiResponse.Usage.PromptTokens),
			attribute.Int("completion_tokens", aiResponse.Usage.CompletionTokens),
			attribute.Int("total_tokens", aiResponse.Usage.TotalTokens),
			attribute.Int64("duration_ms", llmDuration.Milliseconds()),
			attribute.Int("attempt", attempt),
		)

		// Unified Metrics: Record successful AI request
		telemetry.RecordAIRequest(telemetry.ModuleOrchestration, "plan_generation",
			float64(llmDuration.Milliseconds()), "success")
		// Record token usage (input and output separately)
		telemetry.RecordAITokens(telemetry.ModuleOrchestration, "plan_generation",
			"input", int64(aiResponse.Usage.PromptTokens))
		telemetry.RecordAITokens(telemetry.ModuleOrchestration, "plan_generation",
			"output", int64(aiResponse.Usage.CompletionTokens))

		// LLM Debug: Record successful interaction with full prompt and response
		o.recordDebugInteraction(ctx, requestID, LLMInteraction{
			Type:             "plan_generation",
			Timestamp:        llmStartTime,
			DurationMs:       llmDuration.Milliseconds(),
			Prompt:           prompt,
			SystemPrompt:     "You are an intelligent orchestrator that creates execution plans for multi-agent systems.",
			Temperature:      0.3,
			MaxTokens:        2000,
			Model:            aiResponse.Model,
			Provider:         aiResponse.Provider,
			Response:         aiResponse.Content,
			PromptTokens:     aiResponse.Usage.PromptTokens,
			CompletionTokens: aiResponse.Usage.CompletionTokens,
			TotalTokens:      aiResponse.Usage.TotalTokens,
			Success:          true,
			Attempt:          attempt,
		})

		if o.logger != nil {
			o.logger.DebugWithContext(ctx, "LLM response received", map[string]interface{}{
				"operation":       "llm_response",
				"request_id":      requestID,
				"tokens_used":     aiResponse.Usage.TotalTokens,
				"response_length": len(aiResponse.Content),
				"attempt":         attempt,
			})
		}

		// Parse the LLM response into a plan
		plan, parseErr := o.parsePlan(aiResponse.Content)
		if parseErr == nil {
			// Success!
			if o.logger != nil {
				o.logger.DebugWithContext(ctx, "Plan generation completed successfully", map[string]interface{}{
					"operation":        "plan_generation_complete",
					"request_id":       requestID,
					"plan_id":          plan.PlanID,
					"step_count":       len(plan.Steps),
					"total_time_ms":    time.Since(planGenStart).Milliseconds(),
					"tokens_used":      totalTokensUsed,
					"attempts_used":    attempt,
					"retries_required": attempt - 1,
				})
			}
			// Metrics: Record successful plan generation (orchestration-local)
			telemetry.Histogram("plan_generation.duration_ms", float64(time.Since(planGenStart).Milliseconds()),
				"module", telemetry.ModuleOrchestration, "status", "success")
			telemetry.Counter("plan_generation.total",
				"module", telemetry.ModuleOrchestration, "status", "success")
			return plan, nil
		}

		// Parse failed - check if we should retry
		lastParseErr = parseErr

		// Telemetry: Record parse failure
		willRetry := attempt < maxAttempts
		telemetry.AddSpanEvent(ctx, "llm.plan_generation.parse_error",
			attribute.String("request_id", requestID),
			attribute.String("error", parseErr.Error()),
			attribute.Int("attempt", attempt),
			attribute.Bool("will_retry", willRetry),
		)
		// Metrics: Record parse error counter (orchestration-local)
		willRetryStr := "false"
		if willRetry {
			willRetryStr = "true"
		}
		telemetry.Counter("plan_generation.parse_errors",
			"module", telemetry.ModuleOrchestration, "will_retry", willRetryStr)

		if o.logger != nil {
			o.logger.WarnWithContext(ctx, "Plan parsing failed", map[string]interface{}{
				"operation":    "plan_parse_error",
				"request_id":   requestID,
				"error":        parseErr.Error(),
				"attempt":      attempt,
				"max_attempts": maxAttempts,
				"will_retry":   willRetry,
			})
		}

		// If we have retries left, build a new prompt with error feedback
		if willRetry {
			// Metrics: Record retry attempt (orchestration-local)
			telemetry.Counter("plan_generation.retries", "module", telemetry.ModuleOrchestration)
			prompt, err = o.buildPlanningPromptWithParseError(ctx, request, parseErr)
			if err != nil {
				// If we can't build the retry prompt, return the original parse error
				telemetry.Histogram("plan_generation.duration_ms", float64(time.Since(planGenStart).Milliseconds()),
					"module", telemetry.ModuleOrchestration, "status", "error")
				telemetry.Counter("plan_generation.total",
					"module", telemetry.ModuleOrchestration, "status", "error")
				return nil, lastParseErr
			}
		}
	}

	// All attempts exhausted
	if o.logger != nil {
		o.logger.ErrorWithContext(ctx, "Plan generation failed after all retries", map[string]interface{}{
			"operation":     "plan_generation_exhausted",
			"request_id":    requestID,
			"error":         lastParseErr.Error(),
			"attempts_made": maxAttempts,
			"total_tokens":  totalTokensUsed,
			"total_time_ms": time.Since(planGenStart).Milliseconds(),
		})
	}

	// Metrics: Record final plan generation failure after all retries (orchestration-local)
	telemetry.Histogram("plan_generation.duration_ms", float64(time.Since(planGenStart).Milliseconds()),
		"module", telemetry.ModuleOrchestration, "status", "error")
	telemetry.Counter("plan_generation.total",
		"module", telemetry.ModuleOrchestration, "status", "error")
	return nil, lastParseErr
}

// buildPlanningPrompt constructs the prompt for the LLM using capability provider
// and optional PromptBuilder for customization.
// Note: No child span is created here so that tiered_selection and plan_generation
// events are all recorded on the parent orchestrator span for unified visibility.
func (o *AIOrchestrator) buildPlanningPrompt(ctx context.Context, request string) (string, error) {
	// Check if capability provider is available
	if o.capabilityProvider == nil {
		return "", fmt.Errorf("capability provider not configured")
	}

	// Get capabilities from provider
	// Note: TieredCapabilityProvider adds span events to the current (parent) span
	capabilityInfo, err := o.capabilityProvider.GetCapabilities(ctx, request, nil)
	if err != nil {
		telemetry.RecordSpanError(ctx, err)
		return "", fmt.Errorf("failed to get capabilities: %w", err)
	}

	if o.logger != nil {
		o.logger.DebugWithContext(ctx, "Capability information retrieved", map[string]interface{}{
			"operation":       "capability_query",
			"capability_size": len(capabilityInfo),
			"provider_type":   o.config.CapabilityProviderType,
		})
	}

	telemetry.SetSpanAttributes(ctx, attribute.Int("capability_info_size", len(capabilityInfo)))

	// Use PromptBuilder if available (Layer 1-3 customization)
	if o.promptBuilder != nil {
		input := PromptInput{
			CapabilityInfo: capabilityInfo,
			Request:        request,
			Metadata:       nil, // Can be extended to pass request metadata
		}
		prompt, err := o.promptBuilder.BuildPlanningPrompt(ctx, input)
		if err != nil {
			if o.logger != nil {
				o.logger.WarnWithContext(ctx, "PromptBuilder failed, falling back to default prompt", map[string]interface{}{
					"operation": "prompt_builder_fallback",
					"error":     err.Error(),
				})
			}
			// Fall through to default prompt
		} else {
			telemetry.SetSpanAttributes(ctx, attribute.Bool("prompt_builder_used", true))
			return prompt, nil
		}
	}

	// Default hardcoded prompt (backwards compatibility)
	return fmt.Sprintf(`You are an AI orchestrator managing a multi-agent system.

%s

User Request: %s

Create an execution plan in JSON format with the following structure:
{
  "plan_id": "unique-id",
  "original_request": "the user request",
  "mode": "autonomous",
  "steps": [
    {
      "step_id": "step-1",
      "agent_name": "agent-name-from-catalog",
      "namespace": "default",
      "instruction": "specific instruction for this agent",
      "depends_on": [],
      "metadata": {
        "capability": "capability-name",
        "parameters": {
          "string_param": "text value",
          "number_param": 42.5,
          "integer_param": 10,
          "boolean_param": true
        }
      }
    }
  ]
}

CRITICAL - Parameter Type Rules:
- Parameters with type "number" or "float64" MUST be JSON numbers (e.g., 35.6897), NOT strings (e.g., "35.6897")
- Parameters with type "integer" or "int" MUST be JSON integers (e.g., 10), NOT strings (e.g., "10")
- Parameters with type "boolean" or "bool" MUST be JSON booleans (e.g., true), NOT strings (e.g., "true")
- Parameters with type "string" should be JSON strings (e.g., "value")

Important:
1. Only use agents and capabilities that exist in the catalog
2. Ensure parameter names AND TYPES match exactly what the capability expects
3. Order steps based on dependencies
4. Include all necessary steps to fulfill the request
5. Be specific in instructions
6. For coordinates (lat/lon), use numeric values like 35.6897 not "35.6897"

CRITICAL FORMAT RULES:
- Output ONLY valid JSON - no markdown, no code blocks, no backticks
- Do NOT use markdown formatting like ** or * in any values
- Do NOT wrap the JSON in code fences

Response (JSON only):`, capabilityInfo, request), nil
}

// buildPlanningPromptWithParseError constructs a retry prompt that includes the parse error
// context to help the LLM generate valid JSON on subsequent attempts.
func (o *AIOrchestrator) buildPlanningPromptWithParseError(ctx context.Context, request string, parseErr error) (string, error) {
	// Get the base prompt first
	basePrompt, err := o.buildPlanningPrompt(ctx, request)
	if err != nil {
		return "", err
	}

	// Construct error feedback section
	errorFeedback := fmt.Sprintf(`
IMPORTANT: Your previous response could not be parsed as valid JSON.

Parse Error: %s

Common JSON mistakes to avoid:
- NO arithmetic expressions: "amount": 100 * price is INVALID JSON
- NO markdown formatting: **bold** and *italic* are INVALID in JSON strings
- NO code blocks: Do not wrap JSON in triple backticks
- NO trailing commas: {"a": 1,} is INVALID (remove trailing comma)
- NO comments: // comments are INVALID in JSON
- ALL string values must be in double quotes
- ALL keys must be in double quotes

Please regenerate a VALID JSON execution plan. Start with { and end with }.`,
		parseErr.Error())

	// Insert error feedback before the base prompt's final instruction
	return errorFeedback + "\n\n" + basePrompt, nil
}

// parsePlan parses the LLM response into a RoutingPlan
func (o *AIOrchestrator) parsePlan(llmResponse string) (*RoutingPlan, error) {
	// Step 1: Clean the response - strip markdown code blocks
	cleaned := stripMarkdownCodeBlocks(llmResponse)

	// Step 2: Extract JSON from the response (LLM might include extra text)
	jsonStart := findJSONStart(cleaned)
	if jsonStart == -1 {
		// Log for debugging
		if o.logger != nil {
			o.logger.Warn("No JSON found in LLM response", map[string]interface{}{
				"operation":       "plan_parsing",
				"response_length": len(llmResponse),
				"response_prefix": truncateString(llmResponse, 200),
			})
		}
		return nil, fmt.Errorf("no JSON found in LLM response")
	}

	jsonEnd := findJSONEndStringSafe(cleaned, jsonStart)
	if jsonEnd == -1 {
		if o.logger != nil {
			o.logger.Warn("Invalid JSON structure in LLM response", map[string]interface{}{
				"operation":       "plan_parsing",
				"json_start":      jsonStart,
				"response_length": len(cleaned),
			})
		}
		return nil, fmt.Errorf("invalid JSON structure in LLM response")
	}

	jsonStr := cleaned[jsonStart:jsonEnd]

	// Step 3: Try to parse the JSON
	var plan RoutingPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		// Log the actual JSON that failed to parse for debugging
		if o.logger != nil {
			o.logger.Warn("Failed to parse plan JSON", map[string]interface{}{
				"operation":   "plan_parsing",
				"error":       err.Error(),
				"json_length": len(jsonStr),
				"json_prefix": truncateString(jsonStr, 300),
			})
		}
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	// Set creation time
	plan.CreatedAt = time.Now()

	return &plan, nil
}

// validatePlan checks if the plan is executable
func (o *AIOrchestrator) validatePlan(plan *RoutingPlan) error {
	// Check if discovery is available
	if o.discovery == nil {
		return fmt.Errorf("discovery service not configured")
	}

	if o.logger != nil {
		o.logger.Debug("Validating execution plan", map[string]interface{}{
			"operation":  "plan_validation",
			"plan_id":    plan.PlanID,
			"step_count": len(plan.Steps),
		})
	}

	for _, step := range plan.Steps {
		// Check if agent exists
		agents, err := o.discovery.FindService(context.Background(), step.AgentName)
		if err != nil || len(agents) == 0 {
			if o.logger != nil {
				o.logger.Debug("Agent not found during validation", map[string]interface{}{
					"operation":  "capability_validation",
					"step_id":    step.StepID,
					"agent_name": step.AgentName,
					"status":     "agent_not_found",
				})
			}
			return fmt.Errorf("agent %s not found", step.AgentName)
		}

		// Check if capability exists
		if capName, ok := step.Metadata["capability"].(string); ok {
			agentInfo := o.catalog.GetAgent(agents[0].ID)
			if agentInfo == nil {
				return fmt.Errorf("agent %s not in catalog", step.AgentName)
			}

			found := false
			availableCaps := make([]string, len(agentInfo.Capabilities))
			for i, cap := range agentInfo.Capabilities {
				availableCaps[i] = cap.Name
				if cap.Name == capName {
					found = true
				}
			}

			if o.logger != nil {
				o.logger.Debug("Capability validation", map[string]interface{}{
					"operation":              "capability_validation",
					"step_id":                step.StepID,
					"agent_name":             step.AgentName,
					"requested_capability":   capName,
					"available_capabilities": availableCaps,
					"found":                  found,
				})
			}

			if !found {
				return fmt.Errorf("capability %s not found for agent %s", capName, step.AgentName)
			}
		}

		// Check dependencies
		for _, dep := range step.DependsOn {
			found := false
			for _, s := range plan.Steps {
				if s.StepID == dep {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("dependency %s not found for step %s", dep, step.StepID)
			}
		}
	}

	if o.logger != nil {
		o.logger.Info("Plan validation successful", map[string]interface{}{
			"operation":  "plan_validation",
			"plan_id":    plan.PlanID,
			"step_count": len(plan.Steps),
			"status":     "valid",
		})
	}

	return nil
}

// regeneratePlan attempts to fix a plan based on validation errors
func (o *AIOrchestrator) regeneratePlan(ctx context.Context, request string, requestID string, validationErr error) (*RoutingPlan, error) {
	// Check if AI client is available
	if o.aiClient == nil {
		return nil, fmt.Errorf("AI client not configured for plan regeneration")
	}

	// Inject requestID into context for child components
	ctx = WithRequestID(ctx, requestID)

	basePrompt, err := o.buildPlanningPrompt(ctx, request)
	if err != nil {
		return nil, err
	}

	prompt := fmt.Sprintf(`%s

The previous plan failed validation with error: %s

Please generate a corrected plan that addresses this error.`,
		basePrompt, validationErr.Error())

	aiResponse, err := o.aiClient.GenerateResponse(ctx, prompt, &core.AIOptions{
		Temperature: 0.2,
		MaxTokens:   2000,
	})
	if err != nil {
		return nil, err
	}

	return o.parsePlan(aiResponse.Content)
}

// extractAgentsFromPlan gets list of agents involved in a plan
func (o *AIOrchestrator) extractAgentsFromPlan(plan *RoutingPlan) []string {
	agentSet := make(map[string]bool)
	for _, step := range plan.Steps {
		agentSet[step.AgentName] = true
	}

	agents := make([]string, 0, len(agentSet))
	for agent := range agentSet {
		agents = append(agents, agent)
	}
	return agents
}

// ExecutePlan executes a pre-defined routing plan
func (o *AIOrchestrator) ExecutePlan(ctx context.Context, plan *RoutingPlan) (*ExecutionResult, error) {
	if o.executor == nil {
		return nil, fmt.Errorf("executor not configured")
	}
	return o.executor.Execute(ctx, plan)
}

// GetExecutionHistory returns recent execution history
func (o *AIOrchestrator) GetExecutionHistory() []ExecutionRecord {
	o.historyMutex.RLock()
	defer o.historyMutex.RUnlock()

	historyCopy := make([]ExecutionRecord, len(o.history))
	copy(historyCopy, o.history)
	return historyCopy
}

// GetMetrics returns orchestrator metrics
func (o *AIOrchestrator) GetMetrics() OrchestratorMetrics {
	o.metricsMutex.RLock()
	defer o.metricsMutex.RUnlock()

	return *o.metrics
}

// Helper functions

func (o *AIOrchestrator) updateMetrics(duration time.Duration, success bool) {
	o.metricsMutex.Lock()
	defer o.metricsMutex.Unlock()

	o.metrics.TotalRequests++
	if success {
		o.metrics.SuccessfulRequests++
	} else {
		o.metrics.FailedRequests++
	}

	// Update latency metrics (simplified for MVP)
	if o.metrics.AverageLatency == 0 {
		o.metrics.AverageLatency = duration
	} else {
		o.metrics.AverageLatency = (o.metrics.AverageLatency + duration) / 2
	}
	o.metrics.LastRequestTime = time.Now()
}

func (o *AIOrchestrator) addToHistory(response *OrchestratorResponse) {
	o.historyMutex.Lock()
	defer o.historyMutex.Unlock()

	record := ExecutionRecord{
		RequestID:      response.RequestID,
		Timestamp:      time.Now(),
		Request:        response.OriginalRequest,
		Response:       response.Response,
		RoutingMode:    response.RoutingMode,
		AgentsInvolved: response.AgentsInvolved,
		ExecutionTime:  response.ExecutionTime,
		Success:        len(response.Errors) == 0,
		Errors:         response.Errors,
	}

	o.history = append(o.history, record)

	// Trim history if needed
	if len(o.history) > o.config.HistorySize {
		o.history = o.history[1:]
	}
}

// findJSONStart finds the start of JSON in a string
func findJSONStart(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '{' {
			return i
		}
	}
	return -1
}

// findJSONEndStringSafe finds the end of JSON while properly handling strings.
// This correctly skips braces that appear inside quoted strings.
func findJSONEndStringSafe(s string, start int) int {
	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(s); i++ {
		c := s[i]

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' && inString {
			escaped = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue // Skip characters inside strings
		}

		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// stripMarkdownCodeBlocks removes markdown code block fences from LLM responses.
// Handles both ```json and ``` formats.
var markdownCodeBlockRegex = regexp.MustCompile("(?s)```(?:json)?\\s*([\\s\\S]*?)\\s*```")

// markdownBoldRegex matches **bold** patterns inside JSON string values
var markdownBoldRegex = regexp.MustCompile(`\*\*([^*]+)\*\*`)

// Note: Italic handling uses manual parsing in stripMarkdownFromJSON() rather than regex
// because single asterisks are harder to match reliably without false positives.

// cleanLLMResponse aggressively cleans LLM responses to extract valid JSON.
// It handles:
// - Markdown code blocks (```json ... ```)
// - Bold markers inside string values (**text** → text)
// - Italic markers inside string values (*text* → text)
// - Intro text like "Here's the plan:"
//
// This is a defensive measure since LLMs (especially Gemini) often add markdown
// formatting despite explicit instructions not to. See research:
// - https://community.openai.com/t/how-to-prevent-gpt-from-outputting-responses-in-markdown-format/961314
// - https://datachain.ai/blog/enforcing-json-outputs-in-commercial-llms
func cleanLLMResponse(s string) string {
	// Step 1: Try to extract from code blocks first (most reliable)
	if matches := markdownCodeBlockRegex.FindStringSubmatch(s); len(matches) > 1 {
		s = strings.TrimSpace(matches[1])
	} else {
		// Step 2: Find the JSON object directly by locating { and its matching }
		// This handles cases where LLM wraps JSON in other text
		jsonStart := strings.Index(s, "{")
		if jsonStart == -1 {
			return s
		}

		// Find the matching closing brace using string-safe detection
		jsonEnd := findJSONEndStringSafe(s, jsonStart)
		if jsonEnd == -1 {
			return s
		}

		// Extract just the JSON portion
		s = strings.TrimSpace(s[jsonStart:jsonEnd])
	}

	// Step 3: Strip markdown formatting from string values
	// This handles cases where LLM puts **bold** or *italic* inside JSON strings
	s = stripMarkdownFromJSON(s)

	return s
}

// stripMarkdownFromJSON removes markdown bold/italic formatting from JSON string values.
// Converts "**Paris**" → "Paris" and "*weather*" → "weather"
// This is safe for JSON because:
// - ** and * inside quoted strings are the only place markdown appears
// - We only strip when the pattern matches complete words
func stripMarkdownFromJSON(s string) string {
	// Strip bold: **text** → text
	s = markdownBoldRegex.ReplaceAllString(s, "$1")

	// Strip italic: *text* → text (but not **)
	// We need to be more careful here to avoid breaking valid content
	// Only strip if it looks like markdown (word boundaries)
	result := strings.Builder{}
	result.Grow(len(s))

	i := 0
	for i < len(s) {
		// Look for potential italic marker
		if s[i] == '*' && i+1 < len(s) && s[i+1] != '*' {
			// Check if this looks like italic markdown: *word*
			// Must have matching * and contain actual content
			endIdx := strings.Index(s[i+1:], "*")
			if endIdx > 0 && endIdx < 100 { // Reasonable word length
				// Check the end isn't a double asterisk
				fullEndIdx := i + 1 + endIdx
				if fullEndIdx+1 >= len(s) || s[fullEndIdx+1] != '*' {
					// Check content doesn't contain special chars that would make this not markdown
					content := s[i+1 : fullEndIdx]
					if !strings.ContainsAny(content, "\n\t{}[]\"") && len(strings.TrimSpace(content)) > 0 {
						// This looks like italic markdown, strip the asterisks
						result.WriteString(content)
						i = fullEndIdx + 1
						continue
					}
				}
			}
		}
		result.WriteByte(s[i])
		i++
	}

	return result.String()
}

func stripMarkdownCodeBlocks(s string) string {
	// Use the more comprehensive cleaning function
	return cleanLLMResponse(s)
}

// truncateString truncates a string to maxLen characters for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// generateID generates a unique ID
func generateID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Nanosecond())
}

// generateRequestID generates a unique request ID (alias for generateID for specification compatibility)
func generateRequestID() string {
	return generateID()
}

// getMapKeys extracts keys from a map for logging
func getMapKeys(m map[string]interface{}) []string {
	if m == nil {
		return []string{}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// countParallelGroups counts how many parallel execution groups exist in the plan
// Steps with no dependencies or the same dependencies can run in parallel
func countParallelGroups(plan *RoutingPlan) int {
	if plan == nil || len(plan.Steps) == 0 {
		return 0
	}

	// Build dependency levels
	levels := make(map[string]int)
	maxLevel := 0

	// First pass: assign level 0 to steps with no dependencies
	for _, step := range plan.Steps {
		if len(step.DependsOn) == 0 {
			levels[step.StepID] = 0
		}
	}

	// Iteratively assign levels based on dependencies
	changed := true
	for changed {
		changed = false
		for _, step := range plan.Steps {
			if _, ok := levels[step.StepID]; ok {
				continue // Already assigned
			}

			// Check if all dependencies have levels assigned
			allDepsAssigned := true
			maxDepLevel := 0
			for _, dep := range step.DependsOn {
				if depLevel, ok := levels[dep]; ok {
					if depLevel > maxDepLevel {
						maxDepLevel = depLevel
					}
				} else {
					allDepsAssigned = false
					break
				}
			}

			if allDepsAssigned {
				levels[step.StepID] = maxDepLevel + 1
				if levels[step.StepID] > maxLevel {
					maxLevel = levels[step.StepID]
				}
				changed = true
			}
		}
	}

	return maxLevel + 1 // +1 because levels are 0-indexed
}

// formatPlanSteps formats plan steps for DEBUG logging
func formatPlanSteps(plan *RoutingPlan) []map[string]interface{} {
	if plan == nil {
		return nil
	}

	steps := make([]map[string]interface{}, len(plan.Steps))
	for i, step := range plan.Steps {
		stepInfo := map[string]interface{}{
			"step_id":    step.StepID,
			"agent_name": step.AgentName,
			"depends_on": step.DependsOn,
		}

		// Add capability if present in metadata
		if cap, ok := step.Metadata["capability"].(string); ok {
			stepInfo["capability"] = cap
		}

		// Add parameters if present
		if params, ok := step.Metadata["parameters"].(map[string]interface{}); ok {
			stepInfo["parameters"] = params
		}

		steps[i] = stepInfo
	}

	return steps
}
