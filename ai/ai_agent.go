package ai

import (
	"context"
	"fmt"

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

	// Create AI client
	aiClient, err := NewClient(
		WithProvider("openai"),
		WithAPIKey(apiKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI client: %w", err)
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
	client := a.AI
	if client == nil {
		client = a.aiClient
	}
	if client == nil {
		return "", "", fmt.Errorf("no AI client configured")
	}

	// First, think about the task (planning phase)
	thinkPrompt := fmt.Sprintf("Analyze this task and break it down: %s", task)
	thinkResp, err := client.GenerateResponse(ctx, thinkPrompt, nil)
	if err != nil {
		return "", "", fmt.Errorf("thinking failed: %w", err)
	}

	// Then act on the analysis
	actPrompt := fmt.Sprintf("Based on this analysis: %s\nExecute: %s", thinkResp.Content, task)
	actResp, err := client.GenerateResponse(ctx, actPrompt, nil)
	if err != nil {
		return thinkResp.Content, "", fmt.Errorf("action failed: %w", err)
	}

	return thinkResp.Content, actResp.Content, nil
}

// ProcessWithAI uses AI to understand requests and coordinate components
func (a *AIAgent) ProcessWithAI(ctx context.Context, request string) (*core.AIResponse, error) {
	// Discover available tools
	tools, err := a.Discover(ctx, core.DiscoveryFilter{
		Type: core.ComponentTypeTool,
	})
	if err != nil {
		a.Logger.Warn("Failed to discover tools", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Discover other agents if needed
	agents, err := a.Discover(ctx, core.DiscoveryFilter{
		Type: core.ComponentTypeAgent,
	})
	if err != nil {
		a.Logger.Warn("Failed to discover agents", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Build context for AI
	contextPrompt := a.buildContextPrompt(tools, agents, request)

	// Use AI to process request
	client := a.AI
	if client == nil {
		client = a.aiClient
	}
	if client == nil {
		return nil, fmt.Errorf("no AI client configured")
	}
	response, err := client.GenerateResponse(ctx, contextPrompt, &core.AIOptions{
		Model:       "gpt-4",
		Temperature: 0.7,
		MaxTokens:   1000,
	})
	if err != nil {
		return nil, fmt.Errorf("AI processing failed: %w", err)
	}

	return response, nil
}

// DiscoverAndOrchestrate discovers components and orchestrates them using AI
func (a *AIAgent) DiscoverAndOrchestrate(ctx context.Context, userQuery string) (string, error) {
	// 1. Use AI to understand the user's intent
	intentPrompt := fmt.Sprintf(`Analyze this user query and determine what tools/capabilities would be needed:
Query: "%s"

List the types of capabilities needed (e.g., "database_query", "calculation", "data_transformation").`, userQuery)

	client := a.AI
	if client == nil {
		client = a.aiClient
	}
	if client == nil {
		return "", fmt.Errorf("no AI client configured")
	}
	intentResp, err := client.GenerateResponse(ctx, intentPrompt, &core.AIOptions{
		Model:       "gpt-4",
		Temperature: 0.3,
		MaxTokens:   200,
	})
	if err != nil {
		return "", fmt.Errorf("failed to analyze intent: %w", err)
	}

	// 2. Discover available components
	allComponents, err := a.Discover(ctx, core.DiscoveryFilter{})
	if err != nil {
		return "", fmt.Errorf("failed to discover components: %w", err)
	}

	if len(allComponents) == 0 {
		return "No components available to handle this request", nil
	}

	// 3. Use AI to plan component usage
	planPrompt := a.buildPlanPrompt(allComponents, userQuery, intentResp.Content)

	planResp, err := client.GenerateResponse(ctx, planPrompt, &core.AIOptions{
		Model:       "gpt-4",
		Temperature: 0.3,
		MaxTokens:   500,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create plan: %w", err)
	}

	// 4. Log the execution plan
	a.Logger.Info("Executing AI-generated plan", map[string]interface{}{
		"plan":            planResp.Content,
		"component_count": len(allComponents),
	})

	// 5. Synthesize final response
	synthesisPrompt := fmt.Sprintf(`Based on the execution plan:
%s

Generate a response to the original user query: "%s"`, planResp.Content, userQuery)

	finalResp, err := client.GenerateResponse(ctx, synthesisPrompt, &core.AIOptions{
		Model:       "gpt-4",
		Temperature: 0.7,
		MaxTokens:   1000,
	})
	if err != nil {
		return "", fmt.Errorf("failed to synthesize response: %w", err)
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
