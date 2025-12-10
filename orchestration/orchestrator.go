package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/core"
)

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
}

// NewAIOrchestrator creates a new AI-powered orchestrator
func NewAIOrchestrator(config *OrchestratorConfig, discovery core.Discovery, aiClient core.AIClient) *AIOrchestrator {
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
func (o *AIOrchestrator) SetLogger(logger core.Logger) {
	if logger == nil {
		o.logger = &core.NoOpLogger{}
	} else {
		o.logger = logger
	}

	// Propagate logger to sub-components (follows dependency injection pattern)
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
		o.logger.Debug("Requesting LLM parameter correction", map[string]interface{}{
			"operation":  "layer3_correction_request",
			"step_id":    step.StepID,
			"capability": step.Metadata["capability"],
		})
	}

	// Call LLM for correction
	response, err := o.aiClient.GenerateResponse(ctx, correctionPrompt, nil)
	if err != nil {
		return nil, fmt.Errorf("LLM correction request failed: %w", err)
	}

	// Extract JSON from response (handle potential markdown wrapping)
	content := response.Content
	content = extractJSON(content)

	// Parse corrected parameters
	var correctedParams map[string]interface{}
	if err := json.Unmarshal([]byte(content), &correctedParams); err != nil {
		return nil, fmt.Errorf("failed to parse corrected parameters: %w", err)
	}

	if o.logger != nil {
		o.logger.Debug("LLM parameter correction successful", map[string]interface{}{
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

	if o.logger != nil {
		o.logger.Info("Starting request processing", map[string]interface{}{
			"operation":     "process_request",
			"request_id":    requestID,
			"request_length": len(request),
			"metadata_keys": getMapKeys(metadata),
		})
	}

	if span != nil {
		span.SetAttribute("request_id", requestID)
		span.SetAttribute("request_length", len(request))
	}
	
	// Record metric for request count if telemetry is available
	if o.telemetry != nil {
		o.telemetry.RecordMetric("orchestrator.requests.total", 1, map[string]string{
			"mode": string(o.config.RoutingMode),
		})
	}

	// Step 1: Get execution plan from LLM
	plan, err := o.generateExecutionPlan(ctx, request, requestID)
	if err != nil {
		if o.logger != nil {
			o.logger.Error("Plan generation failed", map[string]interface{}{
				"operation":  "plan_generation",
				"request_id": requestID,
				"error":      err.Error(),
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

	if o.logger != nil {
		// Extract tools and build plan summary for INFO level
		toolsUsed := o.extractAgentsFromPlan(plan)
		parallelGroups := countParallelGroups(plan)

		o.logger.Info("Plan generated successfully", map[string]interface{}{
			"operation":        "plan_generation",
			"request_id":       requestID,
			"plan_id":          plan.PlanID,
			"step_count":       len(plan.Steps),
			"tools_selected":   toolsUsed,
			"parallel_groups":  parallelGroups,
			"generation_time_ms": time.Since(startTime).Milliseconds(),
		})

		// Log full plan structure at DEBUG level
		o.logger.Debug("Plan structure details", map[string]interface{}{
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

	// Step 3: Execute the plan
	result, err := o.executor.Execute(ctx, plan)
	if err != nil {
		if o.logger != nil {
			o.logger.Error("Plan execution failed", map[string]interface{}{
				"operation":  "plan_execution",
				"request_id": requestID,
				"plan_id":    plan.PlanID,
				"error":      err.Error(),
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
		o.logger.Info("Plan execution completed", map[string]interface{}{
			"operation":     "plan_execution",
			"request_id":    requestID,
			"plan_id":       plan.PlanID,
			"success":       result != nil && result.Success,
			"failed_steps":  failedSteps,
			"duration_ms":   time.Since(startTime).Milliseconds(),
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
		o.logger.Info("Request processing completed successfully", map[string]interface{}{
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

// generateExecutionPlan uses LLM to create an execution plan
func (o *AIOrchestrator) generateExecutionPlan(ctx context.Context, request string, requestID string) (*RoutingPlan, error) {
	planGenStart := time.Now()

	if o.logger != nil {
		o.logger.Debug("Starting plan generation", map[string]interface{}{
			"operation":  "plan_generation_start",
			"request_id": requestID,
		})
	}
	// Check if AI client is available
	if o.aiClient == nil {
		return nil, fmt.Errorf("AI client not configured")
	}
	
	// Build prompt with capability information
	prompt, err := o.buildPlanningPrompt(ctx, request)
	if err != nil {
		return nil, err
	}

	if o.logger != nil {
		o.logger.Debug("LLM prompt constructed", map[string]interface{}{
			"operation":       "prompt_construction",
			"request_id":      requestID,
			"prompt_length":   len(prompt),
			"estimated_tokens": len(prompt) / 4, // Rough estimate: 4 chars per token
		})
	}

	if o.logger != nil {
		o.logger.Debug("Calling LLM for plan generation", map[string]interface{}{
			"operation":   "llm_call",
			"request_id":  requestID,
			"temperature": 0.3,
			"max_tokens":  2000,
		})
	}

	// Call LLM
	aiResponse, err := o.aiClient.GenerateResponse(ctx, prompt, &core.AIOptions{
		Temperature:  0.3, // Lower temperature for more deterministic planning
		MaxTokens:    2000,
		SystemPrompt: "You are an intelligent orchestrator that creates execution plans for multi-agent systems.",
	})
	if err != nil {
		return nil, err
	}

	if o.logger != nil {
		o.logger.Debug("LLM response received", map[string]interface{}{
			"operation":       "llm_response",
			"request_id":      requestID,
			"tokens_used":     aiResponse.Usage.TotalTokens,
			"response_length": len(aiResponse.Content),
		})
	}

	// Parse the LLM response into a plan
	plan, err := o.parsePlan(aiResponse.Content)
	if err != nil {
		return nil, err
	}

	if o.logger != nil {
		o.logger.Debug("Plan generation completed successfully", map[string]interface{}{
			"operation":      "plan_generation_complete",
			"request_id":     requestID,
			"plan_id":        plan.PlanID,
			"step_count":     len(plan.Steps),
			"total_time_ms":  time.Since(planGenStart).Milliseconds(),
			"tokens_used":    aiResponse.Usage.TotalTokens,
		})
	}

	return plan, nil
}

// buildPlanningPrompt constructs the prompt for the LLM using capability provider
// and optional PromptBuilder for customization
func (o *AIOrchestrator) buildPlanningPrompt(ctx context.Context, request string) (string, error) {
	// Start telemetry span if available
	var span core.Span
	if o.telemetry != nil {
		ctx, span = o.telemetry.StartSpan(ctx, "orchestrator.build_prompt")
		defer span.End()
	} else {
		span = &core.NoOpSpan{}
	}

	// Check if capability provider is available
	if o.capabilityProvider == nil {
		return "", fmt.Errorf("capability provider not configured")
	}

	// Get capabilities from provider
	capabilityInfo, err := o.capabilityProvider.GetCapabilities(ctx, request, nil)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		return "", fmt.Errorf("failed to get capabilities: %w", err)
	}

	if o.logger != nil {
		o.logger.Debug("Capability information retrieved", map[string]interface{}{
			"operation":       "capability_query",
			"capability_size": len(capabilityInfo),
			"provider_type":   o.config.CapabilityProviderType,
		})
	}

	if span != nil {
		span.SetAttribute("capability_info_size", len(capabilityInfo))
	}

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
				o.logger.Warn("PromptBuilder failed, falling back to default prompt", map[string]interface{}{
					"operation": "prompt_builder_fallback",
					"error":     err.Error(),
				})
			}
			// Fall through to default prompt
		} else {
			if span != nil {
				span.SetAttribute("prompt_builder_used", true)
			}
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

Response (JSON only, no explanation):`, capabilityInfo, request), nil
}

// parsePlan parses the LLM response into a RoutingPlan
func (o *AIOrchestrator) parsePlan(llmResponse string) (*RoutingPlan, error) {
	// Extract JSON from the response (LLM might include markdown)
	jsonStart := findJSONStart(llmResponse)
	if jsonStart == -1 {
		return nil, fmt.Errorf("no JSON found in LLM response")
	}

	jsonEnd := findJSONEnd(llmResponse, jsonStart)
	if jsonEnd == -1 {
		return nil, fmt.Errorf("invalid JSON in LLM response")
	}

	jsonStr := llmResponse[jsonStart:jsonEnd]

	var plan RoutingPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
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

// findJSONEnd finds the end of JSON in a string
func findJSONEnd(s string, start int) int {
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
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

