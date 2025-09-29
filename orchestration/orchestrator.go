package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
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
	// Propagate telemetry to executor and synthesizer if they support it
	if o.executor != nil {
		// If executor supports telemetry, set it here
		// o.executor.SetTelemetry(telemetry)
	}
	if o.synthesizer != nil {
		// If synthesizer supports telemetry, set it here
		// o.synthesizer.SetTelemetry(telemetry)
	}
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
	if o.synthesizer != nil {
		// If synthesizer supports logging, set it here
		// o.synthesizer.SetLogger(logger)
	}
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
				fmt.Printf("Catalog refresh error: %v\n", err)
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
		o.logger.Info("Plan generated successfully", map[string]interface{}{
			"operation":  "plan_generation",
			"request_id": requestID,
			"plan_id":    plan.PlanID,
			"step_count": len(plan.Steps),
			"generation_time_ms": time.Since(startTime).Milliseconds(),
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
          "param1": "value1"
        }
      }
    }
  ]
}

Important:
1. Only use agents and capabilities that exist in the catalog
2. Ensure parameter names match exactly what the capability expects
3. Order steps based on dependencies
4. Include all necessary steps to fulfill the request
5. Be specific in instructions

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
	
	for _, step := range plan.Steps {
		// Check if agent exists
		agents, err := o.discovery.FindService(context.Background(), step.AgentName)
		if err != nil || len(agents) == 0 {
			return fmt.Errorf("agent %s not found", step.AgentName)
		}

		// Check if capability exists
		if capName, ok := step.Metadata["capability"].(string); ok {
			agentInfo := o.catalog.GetAgent(agents[0].ID)
			if agentInfo == nil {
				return fmt.Errorf("agent %s not in catalog", step.AgentName)
			}

			found := false
			for _, cap := range agentInfo.Capabilities {
				if cap.Name == capName {
					found = true
					break
				}
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

