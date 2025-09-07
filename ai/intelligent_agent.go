package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/itsneelabh/gomind/core"
)

// IntelligentAgent extends BaseAgent with AI capabilities
type IntelligentAgent struct {
	*core.BaseAgent
	aiClient core.AIClient
}

// NewIntelligentAgent creates a new agent with AI capabilities
func NewIntelligentAgent(name string, apiKey string) *IntelligentAgent {
	base := core.NewBaseAgent(name)

	// Create AI client using the new API
	aiClient, err := NewClient(
		WithProvider("openai"),
		WithAPIKey(apiKey),
	)
	if err != nil {
		// Fall back to no AI if creation fails
		base.Logger.Error("Failed to create AI client", map[string]interface{}{
			"error": err.Error(),
		})
		aiClient = nil
	}

	return &IntelligentAgent{
		BaseAgent: base,
		aiClient:  aiClient,
	}
}

// EnableAI adds AI capabilities to an existing BaseAgent
func EnableAI(agent *core.BaseAgent, apiKey string) {
	aiClient, err := NewClient(
		WithProvider("openai"),
		WithAPIKey(apiKey),
	)
	if err != nil {
		agent.Logger.Error("Failed to enable AI", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	agent.AI = aiClient
	agent.Logger.Info("AI capabilities enabled", map[string]interface{}{
		"agent": agent.GetName(),
	})
}

// DiscoverAndUseTools demonstrates how an intelligent agent discovers and uses tools
func (a *IntelligentAgent) DiscoverAndUseTools(ctx context.Context, userQuery string) (string, error) {
	// 1. Use AI to understand the user's intent
	intentPrompt := fmt.Sprintf(`Analyze this user query and determine what tools/capabilities would be needed:
Query: "%s"

List the types of capabilities needed (e.g., "database_query", "calculation", "data_transformation").`, userQuery)

	intentResp, err := a.aiClient.GenerateResponse(ctx, intentPrompt, &core.AIOptions{
		Model:       "gpt-4",
		Temperature: 0.3,
		MaxTokens:   200,
	})
	if err != nil {
		return "", &core.FrameworkError{
			Op:   "DiscoverAndUseTools.AnalyzeIntent",
			Kind: "ai",
			Err:  core.ErrAIOperationFailed,
		}
	}

	// 2. Discover available tools using the Discovery service
	if a.Discovery == nil {
		return "No discovery service available to find tools", nil
	}

	// Parse capabilities from AI response (simplified)
	capabilities := strings.Split(intentResp.Content, "\n")

	var availableTools []*core.ServiceRegistration
	for _, cap := range capabilities {
		cap = strings.TrimSpace(strings.ToLower(cap))
		if cap == "" {
			continue
		}

		tools, err := a.Discovery.FindByCapability(ctx, cap)
		if err != nil {
			a.Logger.Warn("Failed to find tools for capability", map[string]interface{}{
				"capability": cap,
				"error":      err.Error(),
			})
			continue
		}

		availableTools = append(availableTools, tools...)
	}

	if len(availableTools) == 0 {
		return "No tools found to handle this request", nil
	}

	// 3. Use AI to plan tool usage
	toolList := ""
	for _, tool := range availableTools {
		toolList += fmt.Sprintf("- %s: %s (capabilities: %v)\n",
			tool.Name, tool.ID, tool.Capabilities)
	}

	planPrompt := fmt.Sprintf(`Given these available tools:
%s

And this user query: "%s"

Create a plan for which tools to call and in what order.`, toolList, userQuery)

	planResp, err := a.aiClient.GenerateResponse(ctx, planPrompt, &core.AIOptions{
		Model:       "gpt-4",
		Temperature: 0.3,
		MaxTokens:   500,
	})
	if err != nil {
		return "", &core.FrameworkError{
			Op:   "DiscoverAndUseTools.CreatePlan",
			Kind: "ai",
			Err:  core.ErrAIOperationFailed,
		}
	}

	// 4. Execute the plan (simplified - in real implementation would call tools via HTTP)
	a.Logger.Info("Executing AI-generated plan", map[string]interface{}{
		"plan":       planResp.Content,
		"tool_count": len(availableTools),
	})

	// 5. Synthesize results using AI
	synthesisPrompt := fmt.Sprintf(`Based on the execution plan:
%s

Generate a response to the original user query: "%s"`, planResp.Content, userQuery)

	finalResp, err := a.aiClient.GenerateResponse(ctx, synthesisPrompt, &core.AIOptions{
		Model:       "gpt-4",
		Temperature: 0.7,
		MaxTokens:   1000,
	})
	if err != nil {
		return "", &core.FrameworkError{
			Op:   "DiscoverAndUseTools.Synthesize",
			Kind: "ai",
			Err:  core.ErrAIOperationFailed,
		}
	}

	return finalResp.Content, nil
}

// Example shows the progression from Tool to Intelligent Agent
func Example() {
	// Level 1: Basic Tool (Core only - 5MB)
	// tool := core.NewBaseAgent("database-tool")
	// tool.RegisterCapability(...)

	// Level 2: Intelligent Agent (Core + AI - 10MB)
	agent := NewIntelligentAgent("smart-agent", "sk-...")

	// The agent can now:
	// - Understand natural language
	// - Discover tools dynamically
	// - Plan tool usage with AI
	// - Synthesize responses

	ctx := context.Background()
	response, _ := agent.DiscoverAndUseTools(ctx, "What were the Q3 sales figures?")
	println(response)
}
