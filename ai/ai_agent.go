package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// AIAgent extends BaseAgent with AI capabilities and full discovery powers
// This represents an orchestrating agent that can discover and coordinate other components
type AIAgent struct {
	*core.BaseAgent               // Full agent with discovery
	AI              core.AIClient // Exposed for testing compatibility
	aiClient        core.AIClient // Internal field (deprecated, use AI)
}

// IntelligentAgent is an alias for AIAgent for backward compatibility
type IntelligentAgent = AIAgent

// NewAIAgent creates a new agent with AI capabilities and discovery
func NewAIAgent(name string, apiKey string) (*AIAgent, error) {
	agent := core.NewBaseAgent(name)

	// 🔥 CRITICAL: Create AI client with framework logger
	aiClient, err := NewClient(
		WithProvider("openai"),
		WithAPIKey(apiKey),
		WithLogger(agent.Logger), // Transfer framework logger
	)
	if err != nil {
		// 🔥 ADD: AI client creation error logging
		if agent.Logger != nil {
			agent.Logger.Error("Failed to create AI client for agent", map[string]interface{}{
				"operation":  "ai_agent_creation",
				"agent_name": name,
				"error":      err.Error(),
				"error_type": fmt.Sprintf("%T", err),
				"provider":   "openai",
			})
		}
		return nil, fmt.Errorf("failed to create AI client: %w", err)
	}

	// 🔥 ADD: AI agent creation success logging
	if agent.Logger != nil {
		agent.Logger.Info("AI agent created successfully", map[string]interface{}{
			"operation":        "ai_agent_creation",
			"agent_name":       name,
			"agent_id":         agent.ID,
			"ai_client_type":   fmt.Sprintf("%T", aiClient),
			"capabilities":     len(agent.Capabilities),
			"status":           "success",
		})
	}

	agent.AI = aiClient
	return &AIAgent{
		BaseAgent: agent,
		AI:        aiClient,
		aiClient:  aiClient,
	}, nil
}

// NewIntelligentAgent creates a new intelligent agent for testing compatibility
func NewIntelligentAgent(id string) *IntelligentAgent {
	// Use the id as the name for NewBaseAgent
	agent := core.NewBaseAgent(id)
	// Override the auto-generated ID with the provided one
	agent.ID = id
	return &IntelligentAgent{
		BaseAgent: agent,
		AI:        nil, // AI client should be set later
		aiClient:  nil,
	}
}

// GenerateResponse generates a response using the AI client
func (a *AIAgent) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	client := a.AI
	if client == nil {
		client = a.aiClient
	}
	if client == nil {
		return nil, fmt.Errorf("no AI client configured")
	}
	return client.GenerateResponse(ctx, prompt, options)
}

// SetAI sets the AI client for the agent
func (a *AIAgent) SetAI(client core.AIClient) {
	a.AI = client
	a.aiClient = client
}

// ProcessWithMemory processes input with memory storage
func (a *AIAgent) ProcessWithMemory(ctx context.Context, input string) (string, error) {
	// Store input in memory if available
	if a.Memory != nil {
		// Store with both generic and specific keys for compatibility
		a.Memory.Set(ctx, "last_input", input, 0)
		a.Memory.Set(ctx, fmt.Sprintf("input:%s", input), input, 0)
	}

	// Process with AI
	client := a.AI
	if client == nil {
		client = a.aiClient
	}
	if client == nil {
		return "", fmt.Errorf("no AI client configured")
	}

	response, err := client.GenerateResponse(ctx, "Processed: "+input, nil)
	if err != nil {
		return "", err
	}

	// Store response in memory if available
	if a.Memory != nil {
		// Store with both generic and specific keys for compatibility
		a.Memory.Set(ctx, "last_response", response.Content, 0)
		a.Memory.Set(ctx, fmt.Sprintf("response:%s", input), response.Content, 0)
	}

	return response.Content, nil
}

// ThinkAndAct processes complex reasoning tasks
func (a *AIAgent) ThinkAndAct(ctx context.Context, task string) (string, string, error) {
	startTime := time.Now()

	// 🔥 ADD: ThinkAndAct start logging
	if a.Logger != nil {
		a.Logger.Info("Starting ThinkAndAct process", map[string]interface{}{
			"operation":    "ai_think_and_act_start",
			"agent_id":     a.ID,
			"task":         truncateString(task, 150),
			"task_length":  len(task),
		})
	}

	client := a.AI
	if client == nil {
		client = a.aiClient
	}
	if client == nil {
		// 🔥 ADD: AI client error logging
		if a.Logger != nil {
			a.Logger.Error("ThinkAndAct failed - no AI client configured", map[string]interface{}{
				"operation": "ai_think_and_act_error",
				"agent_id":  a.ID,
				"phase":     "initialization",
				"error":     "no AI client configured",
			})
		}
		return "", "", fmt.Errorf("no AI client configured")
	}

	// First, think about the task (planning phase)
	thinkPrompt := fmt.Sprintf("Analyze this task and break it down: %s", task)

	// 🔥 ADD: Think phase start logging
	if a.Logger != nil {
		a.Logger.Info("Starting think phase", map[string]interface{}{
			"operation":     "ai_think_phase_start",
			"agent_id":      a.ID,
			"prompt_length": len(thinkPrompt),
		})
	}

	thinkResp, err := client.GenerateResponse(ctx, thinkPrompt, nil)
	if err != nil {
		// 🔥 ADD: Think phase error logging
		if a.Logger != nil {
			a.Logger.Error("Think phase failed", map[string]interface{}{
				"operation":  "ai_think_phase_error",
				"agent_id":   a.ID,
				"error":      err.Error(),
				"error_type": fmt.Sprintf("%T", err),
				"phase":      "thinking",
			})
		}
		return "", "", fmt.Errorf("thinking failed: %w", err)
	}

	// 🔥 ADD: Think phase completion logging
	if a.Logger != nil {
		a.Logger.Info("Think phase completed", map[string]interface{}{
			"operation":           "ai_think_phase_complete",
			"agent_id":            a.ID,
			"thought_content":     truncateString(thinkResp.Content, 200),
			"thought_length":      len(thinkResp.Content),
			"prompt_tokens":       thinkResp.Usage.PromptTokens,
			"completion_tokens":   thinkResp.Usage.CompletionTokens,
		})
	}

	// Then act on the analysis
	actPrompt := fmt.Sprintf("Based on this analysis: %s\nExecute: %s", thinkResp.Content, task)

	// 🔥 ADD: Act phase start logging
	if a.Logger != nil {
		a.Logger.Info("Starting act phase", map[string]interface{}{
			"operation":     "ai_act_phase_start",
			"agent_id":      a.ID,
			"prompt_length": len(actPrompt),
		})
	}

	actResp, err := client.GenerateResponse(ctx, actPrompt, nil)
	if err != nil {
		// 🔥 ADD: Act phase error logging with recovery context
		if a.Logger != nil {
			a.Logger.Error("Act phase failed with thinking preserved", map[string]interface{}{
				"operation":        "ai_act_phase_error",
				"agent_id":         a.ID,
				"error":            err.Error(),
				"error_type":       fmt.Sprintf("%T", err),
				"phase":            "acting",
				"thinking_result":  truncateString(thinkResp.Content, 100),
				"partial_success":  true, // thinking succeeded
			})
		}
		return thinkResp.Content, "", fmt.Errorf("action failed: %w", err)
	}

	// 🔥 ADD: ThinkAndAct completion logging with performance metrics
	totalDuration := time.Since(startTime)
	totalTokens := calculateTokensUsage(thinkResp, actResp)

	if a.Logger != nil {
		a.Logger.Info("ThinkAndAct completed successfully", map[string]interface{}{
			"operation":               "ai_think_and_act_complete",
			"agent_id":                a.ID,
			"total_duration_ms":       totalDuration.Milliseconds(),
			"action_content":          truncateString(actResp.Content, 200),
			"action_length":           len(actResp.Content),
			"total_prompt_tokens":     totalTokens.PromptTokens,
			"total_completion_tokens": totalTokens.CompletionTokens,
			"total_tokens":            totalTokens.TotalTokens,
			"ai_calls_made":           2, // think + act
			"status":                  "success",
		})
	}

	return thinkResp.Content, actResp.Content, nil
}

// ProcessWithAI uses AI to understand requests and coordinate components
func (a *AIAgent) ProcessWithAI(ctx context.Context, request string) (*core.AIResponse, error) {
	startTime := time.Now()

	// 🔥 ADD: ProcessWithAI start logging
	if a.Logger != nil {
		a.Logger.Info("Starting AI-assisted component processing", map[string]interface{}{
			"operation":        "ai_process_with_ai_start",
			"agent_id":         a.ID,
			"request":          truncateString(request, 150),
			"request_length":   len(request),
		})
	}

	// Discover available tools
	// 🔥 ADD: Tool discovery start logging
	if a.Logger != nil {
		a.Logger.Info("Discovering tools for AI processing", map[string]interface{}{
			"operation": "ai_tool_discovery_start",
			"agent_id":  a.ID,
			"filter":    "ComponentTypeTool",
		})
	}

	tools, err := a.Discover(ctx, core.DiscoveryFilter{
		Type: core.ComponentTypeTool,
	})
	if err != nil {
		// 🔥 ENHANCED: Tool discovery error logging
		if a.Logger != nil {
			a.Logger.Error("Failed to discover tools for AI processing", map[string]interface{}{
				"operation":  "ai_tool_discovery_error",
				"agent_id":   a.ID,
				"error":      err.Error(),
				"error_type": fmt.Sprintf("%T", err),
				"impact":     "proceeding_without_tools",
			})
		}
	} else if a.Logger != nil {
		// 🔥 ADD: Tool discovery success logging
		a.Logger.Info("Tools discovered for AI processing", map[string]interface{}{
			"operation":   "ai_tool_discovery_complete",
			"agent_id":    a.ID,
			"tool_count":  len(tools),
			"tool_names":  func() []string {
				names := make([]string, len(tools))
				for i, tool := range tools {
					names[i] = tool.Name
				}
				return names
			}(),
		})
	}

	// Discover other agents if needed
	// 🔥 ADD: Agent discovery start logging
	if a.Logger != nil {
		a.Logger.Info("Discovering agents for AI processing", map[string]interface{}{
			"operation": "ai_agent_discovery_start",
			"agent_id":  a.ID,
			"filter":    "ComponentTypeAgent",
		})
	}

	agents, err := a.Discover(ctx, core.DiscoveryFilter{
		Type: core.ComponentTypeAgent,
	})
	if err != nil {
		// 🔥 ENHANCED: Agent discovery error logging
		if a.Logger != nil {
			a.Logger.Error("Failed to discover agents for AI processing", map[string]interface{}{
				"operation":  "ai_agent_discovery_error",
				"agent_id":   a.ID,
				"error":      err.Error(),
				"error_type": fmt.Sprintf("%T", err),
				"impact":     "proceeding_without_agents",
			})
		}
	} else if a.Logger != nil {
		// 🔥 ADD: Agent discovery success logging
		a.Logger.Info("Agents discovered for AI processing", map[string]interface{}{
			"operation":    "ai_agent_discovery_complete",
			"agent_id":     a.ID,
			"agent_count":  len(agents),
			"agent_names":  func() []string {
				names := make([]string, len(agents))
				for i, agent := range agents {
					names[i] = agent.Name
				}
				return names
			}(),
		})
	}

	// Build context for AI
	contextPrompt := a.buildContextPrompt(tools, agents, request)

	// 🔥 ADD: AI processing start logging
	if a.Logger != nil {
		a.Logger.Info("Processing request with AI", map[string]interface{}{
			"operation":       "ai_processing_start",
			"agent_id":        a.ID,
			"prompt_length":   len(contextPrompt),
			"tool_count":      len(tools),
			"agent_count":     len(agents),
			"ai_model":        "gpt-4",
			"temperature":     0.7,
			"max_tokens":      1000,
		})
	}

	// Use AI to process request
	client := a.AI
	if client == nil {
		client = a.aiClient
	}
	if client == nil {
		// 🔥 ADD: AI client error logging
		if a.Logger != nil {
			a.Logger.Error("ProcessWithAI failed - no AI client configured", map[string]interface{}{
				"operation": "ai_processing_error",
				"agent_id":  a.ID,
				"phase":     "ai_processing",
				"error":     "no AI client configured",
			})
		}
		return nil, fmt.Errorf("no AI client configured")
	}

	response, err := client.GenerateResponse(ctx, contextPrompt, &core.AIOptions{
		Model:       "gpt-4",
		Temperature: 0.7,
		MaxTokens:   1000,
	})
	if err != nil {
		// 🔥 ADD: AI processing error logging
		if a.Logger != nil {
			a.Logger.Error("AI processing failed", map[string]interface{}{
				"operation":  "ai_processing_error",
				"agent_id":   a.ID,
				"error":      err.Error(),
				"error_type": fmt.Sprintf("%T", err),
				"phase":      "ai_generation",
			})
		}
		return nil, fmt.Errorf("AI processing failed: %w", err)
	}

	// 🔥 ADD: ProcessWithAI completion logging with performance metrics
	totalDuration := time.Since(startTime)

	if a.Logger != nil {
		a.Logger.Info("AI processing completed successfully", map[string]interface{}{
			"operation":           "ai_process_with_ai_complete",
			"agent_id":            a.ID,
			"total_duration_ms":   totalDuration.Milliseconds(),
			"response_content":    truncateString(response.Content, 200),
			"response_length":     len(response.Content),
			"prompt_tokens":       response.Usage.PromptTokens,
			"completion_tokens":   response.Usage.CompletionTokens,
			"total_tokens":        response.Usage.TotalTokens,
			"tokens_per_second":   float64(response.Usage.TotalTokens) / totalDuration.Seconds(),
			"components_used":     len(tools) + len(agents),
			"status":              "success",
		})
	}

	return response, nil
}

// DiscoverAndOrchestrate discovers components and orchestrates them using AI
func (a *AIAgent) DiscoverAndOrchestrate(ctx context.Context, userQuery string) (string, error) {
	startTime := time.Now()

	// 🔥 ADD: Orchestration start logging
	if a.Logger != nil {
		a.Logger.Info("Starting AI agent orchestration", map[string]interface{}{
			"operation":          "ai_orchestration_start",
			"agent_id":           a.ID,
			"agent_name":         a.Name,
			"user_query":         truncateString(userQuery, 200),
			"user_query_length":  len(userQuery),
			"timestamp":          startTime.Format(time.RFC3339),
		})
	}

	// 1. Use AI to understand the user's intent
	intentPrompt := fmt.Sprintf(`Analyze this user query and determine what tools/capabilities would be needed:
Query: "%s"

List the types of capabilities needed (e.g., "database_query", "calculation", "data_transformation").`, userQuery)

	client := a.AI
	if client == nil {
		client = a.aiClient
	}
	if client == nil {
		// 🔥 ADD: AI client error logging
		if a.Logger != nil {
			a.Logger.Error("AI orchestration failed - no client configured", map[string]interface{}{
				"operation": "ai_orchestration_error",
				"agent_id":  a.ID,
				"phase":     "intent_analysis",
				"error":     "no AI client configured",
			})
		}
		return "", fmt.Errorf("no AI client configured")
	}

	// 🔥 ADD: Intent analysis phase start logging
	if a.Logger != nil {
		a.Logger.Info("Analyzing user intent with AI", map[string]interface{}{
			"operation":       "ai_intent_analysis_start",
			"agent_id":        a.ID,
			"prompt_length":   len(intentPrompt),
			"ai_model":        "gpt-4",
			"temperature":     0.3,
			"max_tokens":      200,
		})
	}

	intentResp, err := client.GenerateResponse(ctx, intentPrompt, &core.AIOptions{
		Model:       "gpt-4",
		Temperature: 0.3,
		MaxTokens:   200,
	})
	if err != nil {
		// 🔥 ADD: Intent analysis error logging
		if a.Logger != nil {
			a.Logger.Error("Intent analysis failed", map[string]interface{}{
				"operation": "ai_intent_analysis_error",
				"agent_id":  a.ID,
				"error":     err.Error(),
				"error_type": fmt.Sprintf("%T", err),
			})
		}
		return "", fmt.Errorf("failed to analyze intent: %w", err)
	}

	// 🔥 ADD: Intent analysis completion logging
	if a.Logger != nil {
		a.Logger.Info("User intent analyzed successfully", map[string]interface{}{
			"operation":           "ai_intent_analysis_complete",
			"agent_id":            a.ID,
			"intent_content":      truncateString(intentResp.Content, 150),
			"intent_length":       len(intentResp.Content),
			"prompt_tokens":       intentResp.Usage.PromptTokens,
			"completion_tokens":   intentResp.Usage.CompletionTokens,
			"total_tokens":        intentResp.Usage.TotalTokens,
		})
	}

	// 2. Discover available components
	// 🔥 ADD: Component discovery start logging
	if a.Logger != nil {
		a.Logger.Info("Starting component discovery", map[string]interface{}{
			"operation": "ai_component_discovery_start",
			"agent_id":  a.ID,
			"filter":    "{}",
		})
	}

	allComponents, err := a.Discover(ctx, core.DiscoveryFilter{})
	if err != nil {
		// 🔥 ADD: Component discovery error logging
		if a.Logger != nil {
			a.Logger.Error("Component discovery failed", map[string]interface{}{
				"operation": "ai_component_discovery_error",
				"agent_id":  a.ID,
				"error":     err.Error(),
				"error_type": fmt.Sprintf("%T", err),
			})
		}
		return "", fmt.Errorf("failed to discover components: %w", err)
	}

	// 🔥 ADD: Component discovery completion logging
	if a.Logger != nil {
		componentTypes := extractComponentTypes(allComponents)
		a.Logger.Info("Component discovery completed", map[string]interface{}{
			"operation":        "ai_component_discovery_complete",
			"agent_id":         a.ID,
			"component_count":  len(allComponents),
			"component_types":  componentTypes,
			"discovered_names": func() []string {
				names := make([]string, len(allComponents))
				for i, comp := range allComponents {
					names[i] = comp.Name
				}
				return names
			}(),
		})
	}

	if len(allComponents) == 0 {
		// 🔥 ADD: No components found logging
		if a.Logger != nil {
			a.Logger.Warn("No components available for orchestration", map[string]interface{}{
				"operation": "ai_orchestration_warning",
				"agent_id":  a.ID,
				"reason":    "no_components_discovered",
				"suggestion": "Check service registry or component availability",
			})
		}
		return "No components available to handle this request", nil
	}

	// 3. Use AI to plan component usage
	planPrompt := a.buildPlanPrompt(allComponents, userQuery, intentResp.Content)

	// 🔥 ADD: Plan creation start logging
	if a.Logger != nil {
		a.Logger.Info("Creating execution plan with AI", map[string]interface{}{
			"operation":       "ai_plan_creation_start",
			"agent_id":        a.ID,
			"prompt_length":   len(planPrompt),
			"component_count": len(allComponents),
			"ai_model":        "gpt-4",
			"temperature":     0.3,
			"max_tokens":      500,
		})
	}

	planResp, err := client.GenerateResponse(ctx, planPrompt, &core.AIOptions{
		Model:       "gpt-4",
		Temperature: 0.3,
		MaxTokens:   500,
	})
	if err != nil {
		// 🔥 ADD: Plan creation error logging
		if a.Logger != nil {
			a.Logger.Error("Execution plan creation failed", map[string]interface{}{
				"operation": "ai_plan_creation_error",
				"agent_id":  a.ID,
				"error":     err.Error(),
				"error_type": fmt.Sprintf("%T", err),
			})
		}
		return "", fmt.Errorf("failed to create plan: %w", err)
	}

	// 4. Enhanced execution plan logging
	if a.Logger != nil {
		a.Logger.Info("Execution plan created successfully", map[string]interface{}{
			"operation":           "ai_plan_creation_complete",
			"agent_id":            a.ID,
			"plan_content":        truncateString(planResp.Content, 300),
			"plan_length":         len(planResp.Content),
			"component_count":     len(allComponents),
			"prompt_tokens":       planResp.Usage.PromptTokens,
			"completion_tokens":   planResp.Usage.CompletionTokens,
			"total_tokens":        planResp.Usage.TotalTokens,
		})
	}

	// 5. Synthesize final response
	synthesisPrompt := fmt.Sprintf(`Based on the execution plan:
%s

Generate a response to the original user query: "%s"`, planResp.Content, userQuery)

	// 🔥 ADD: Response synthesis start logging
	if a.Logger != nil {
		a.Logger.Info("Synthesizing final response", map[string]interface{}{
			"operation":       "ai_synthesis_start",
			"agent_id":        a.ID,
			"prompt_length":   len(synthesisPrompt),
			"ai_model":        "gpt-4",
			"temperature":     0.7,
			"max_tokens":      1000,
		})
	}

	finalResp, err := client.GenerateResponse(ctx, synthesisPrompt, &core.AIOptions{
		Model:       "gpt-4",
		Temperature: 0.7,
		MaxTokens:   1000,
	})
	if err != nil {
		// 🔥 ADD: Response synthesis error logging
		if a.Logger != nil {
			a.Logger.Error("Response synthesis failed", map[string]interface{}{
				"operation": "ai_synthesis_error",
				"agent_id":  a.ID,
				"error":     err.Error(),
				"error_type": fmt.Sprintf("%T", err),
			})
		}
		return "", fmt.Errorf("failed to synthesize response: %w", err)
	}

	// 🔥 ADD: Calculate total performance metrics
	totalDuration := time.Since(startTime)
	totalTokens := calculateTokensUsage(intentResp, planResp, finalResp)

	// 🔥 ADD: Orchestration completion logging with performance metrics
	if a.Logger != nil {
		a.Logger.Info("AI orchestration completed successfully", map[string]interface{}{
			"operation":                "ai_orchestration_complete",
			"agent_id":                 a.ID,
			"total_duration_ms":        totalDuration.Milliseconds(),
			"total_duration_seconds":   totalDuration.Seconds(),
			"component_count":          len(allComponents),
			"response_content":         truncateString(finalResp.Content, 200),
			"response_length":          len(finalResp.Content),
			"total_prompt_tokens":      totalTokens.PromptTokens,
			"total_completion_tokens":  totalTokens.CompletionTokens,
			"total_tokens":             totalTokens.TotalTokens,
			"tokens_per_second":        float64(totalTokens.TotalTokens) / totalDuration.Seconds(),
			"ai_calls_made":            3, // intent, plan, synthesis
			"phases_completed": []string{
				"intent_analysis",
				"component_discovery",
				"plan_creation",
				"response_synthesis",
			},
			"status": "success",
		})
	}

	return finalResp.Content, nil
}

// buildContextPrompt creates a context prompt for the AI with available components
func (a *AIAgent) buildContextPrompt(tools []*core.ServiceInfo, agents []*core.ServiceInfo, request string) string {
	prompt := "Available components:\n\n"

	if len(tools) > 0 {
		prompt += "TOOLS (passive components):\n"
		for _, tool := range tools {
			prompt += fmt.Sprintf("- %s (%s): %s\n", tool.Name, tool.ID, tool.Description)
			for _, cap := range tool.Capabilities {
				prompt += fmt.Sprintf("  * %s: %s\n", cap.Name, cap.Description)
			}
		}
		prompt += "\n"
	}

	if len(agents) > 0 {
		prompt += "AGENTS (active orchestrators):\n"
		for _, agent := range agents {
			prompt += fmt.Sprintf("- %s (%s): %s\n", agent.Name, agent.ID, agent.Description)
			for _, cap := range agent.Capabilities {
				prompt += fmt.Sprintf("  * %s: %s\n", cap.Name, cap.Description)
			}
		}
		prompt += "\n"
	}

	prompt += fmt.Sprintf("\nUser request: %s\n", request)
	prompt += "\nHow would you handle this request using the available components?"

	return prompt
}

// buildPlanPrompt creates a planning prompt for the AI
func (a *AIAgent) buildPlanPrompt(components []*core.ServiceInfo, userQuery, intent string) string {
	componentList := ""
	for _, comp := range components {
		componentList += fmt.Sprintf("- %s (%s): %s\n", comp.Name, comp.Type, comp.Description)
		for _, cap := range comp.Capabilities {
			componentList += fmt.Sprintf("  * %s: %s\n", cap.Name, cap.Description)
		}
	}

	return fmt.Sprintf(`Given these available components:
%s

User intent analysis: %s

And this user query: "%s"

Create a step-by-step plan for which components to use and in what order.
Be specific about which capabilities to invoke and what data to pass between them.`,
		componentList, intent, userQuery)
}

// 🔥 ADD: Helper functions for logging support

// truncateString truncates a string to maxLength with ellipsis if needed
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	if maxLength < 3 {
		return "..."
	}
	return s[:maxLength-3] + "..."
}

// extractComponentTypes extracts unique component types from a list of ServiceInfo
func extractComponentTypes(components []*core.ServiceInfo) []string {
	typeMap := make(map[string]bool)
	for _, comp := range components {
		typeMap[string(comp.Type)] = true
	}

	types := make([]string, 0, len(typeMap))
	for t := range typeMap {
		types = append(types, t)
	}
	return types
}

// calculateTokensUsage calculates total tokens from multiple AI responses
func calculateTokensUsage(responses ...*core.AIResponse) core.TokenUsage {
	var total core.TokenUsage
	for _, resp := range responses {
		if resp != nil {
			total.PromptTokens += resp.Usage.PromptTokens
			total.CompletionTokens += resp.Usage.CompletionTokens
			total.TotalTokens += resp.Usage.TotalTokens
		}
	}
	return total
}
