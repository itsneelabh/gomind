package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"
)

// ResearchAgent is an intelligent agent that discovers and orchestrates tools
// It demonstrates the active agent pattern - can discover and coordinate other components
type ResearchAgent struct {
	*core.BaseAgent
	aiClient   core.AIClient
	httpClient *http.Client // Shared HTTP client for connection pooling
}

// ResearchRequest represents the input for research operations
type ResearchRequest struct {
	Topic      string            `json:"topic"`
	Sources    []string          `json:"sources,omitempty"`     // Specific sources to use
	MaxResults int               `json:"max_results,omitempty"` // Limit results
	Metadata   map[string]string `json:"metadata,omitempty"`    // Additional parameters
	UseAI      bool              `json:"use_ai,omitempty"`      // Whether to use AI analysis
	WorkflowID string            `json:"workflow_id,omitempty"` // For workflow tracking
}

// ResearchResponse represents the synthesized research output
type ResearchResponse struct {
	Topic          string                 `json:"topic"`
	Summary        string                 `json:"summary"`
	ToolsUsed      []string               `json:"tools_used"`
	Results        []ToolResult           `json:"results"`
	AIAnalysis     string                 `json:"ai_analysis,omitempty"`
	Confidence     float64                `json:"confidence"`
	ProcessingTime string                 `json:"processing_time"`
	WorkflowID     string                 `json:"workflow_id,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ToolResult represents the result from calling a tool
type ToolResult struct {
	ToolName   string      `json:"tool_name"`
	Capability string      `json:"capability"`
	Data       interface{} `json:"data"`
	Success    bool        `json:"success"`
	Error      string      `json:"error,omitempty"`
	Duration   string      `json:"duration"`
}

// NewResearchAgent creates a new AI-powered research assistant
func NewResearchAgent() (*ResearchAgent, error) {
	agent := core.NewBaseAgent("research-assistant")

	// Auto-configured AI client - detects from environment
	aiClient, err := ai.NewClient() // Auto-detects best available provider
	if err != nil {
		log.Printf("AI client creation failed, using mock: %v", err)
		// In production, you might want to fail here or use a fallback
		// For the example, we'll continue without AI for basic orchestration
	}

	// Store AI client in agent
	if aiClient != nil {
		agent.AI = aiClient
	}

	researchAgent := &ResearchAgent{
		BaseAgent:  agent,
		aiClient:   aiClient,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				DisableKeepAlives:   false,
				ForceAttemptHTTP2:   true,
			},
		},
	}

	// Register agent capabilities
	researchAgent.registerCapabilities()
	return researchAgent, nil
}

// registerCapabilities sets up all research-related capabilities
func (r *ResearchAgent) registerCapabilities() {
	// Capability 1: Orchestrated research (AI + tool discovery)
	r.RegisterCapability(core.Capability{
		Name:        "research_topic",
		Description: "Researches a topic by discovering and coordinating relevant tools",
		InputTypes:  []string{"json", "text"},
		OutputTypes: []string{"json"},
		Handler:     r.handleResearchTopic,
		// Phase 2: Field hints for AI-powered payload generation
		InputSummary: &core.SchemaSummary{
			RequiredFields: []core.FieldHint{
				{
					Name:        "topic",
					Type:        "string",
					Example:     "latest developments in renewable energy",
					Description: "The research topic or question to investigate",
				},
			},
			OptionalFields: []core.FieldHint{
				{
					Name:        "sources",
					Type:        "array",
					Example:     `["weather-service", "stock-service"]`,
					Description: "Specific tool names to use for research",
				},
				{
					Name:        "max_results",
					Type:        "number",
					Example:     "5",
					Description: "Maximum number of results to return",
				},
				{
					Name:        "use_ai",
					Type:        "boolean",
					Example:     "true",
					Description: "Whether to use AI for analysis and synthesis",
				},
				{
					Name:        "workflow_id",
					Type:        "string",
					Example:     "research-12345",
					Description: "Optional workflow tracking identifier",
				},
			},
		},
	})

	// Capability 2: Component discovery and status
	r.RegisterCapability(core.Capability{
		Name:        "discover_tools",
		Description: "Discovers available tools and their capabilities",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     r.handleDiscoverTools,
		// Phase 2: Field hints for filtering discovery
		InputSummary: &core.SchemaSummary{
			OptionalFields: []core.FieldHint{
				{
					Name:        "type",
					Type:        "string",
					Example:     "tool",
					Description: "Filter by component type: tool, agent, or workflow",
				},
				{
					Name:        "capabilities",
					Type:        "array",
					Example:     `["weather", "stocks"]`,
					Description: "Filter tools by required capabilities",
				},
			},
		},
	})

	// Capability 3: AI-powered analysis (if AI is available)
	r.RegisterCapability(core.Capability{
		Name:        "analyze_data",
		Description: "Uses AI to analyze and synthesize data from multiple sources",
		InputTypes:  []string{"json", "text"},
		OutputTypes: []string{"json"},
		Handler:     r.handleAnalyzeData,
		// Phase 2: Field hints for analysis requests
		InputSummary: &core.SchemaSummary{
			RequiredFields: []core.FieldHint{
				{
					Name:        "data",
					Type:        "object",
					Example:     `{"results": [...]}`,
					Description: "The data to analyze (can be array or object)",
				},
			},
			OptionalFields: []core.FieldHint{
				{
					Name:        "question",
					Type:        "string",
					Example:     "What are the key trends?",
					Description: "Specific question to answer about the data",
				},
				{
					Name:        "format",
					Type:        "string",
					Example:     "summary",
					Description: "Output format: summary, detailed, or bullet-points",
				},
			},
		},
	})

	// Capability 4: Workflow orchestration
	r.RegisterCapability(core.Capability{
		Name:        "orchestrate_workflow",
		Description: "Orchestrates a multi-step workflow using discovered tools",
		Endpoint:    "/orchestrate", // Custom endpoint
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     r.handleOrchestateWorkflow,
		// Phase 2: Field hints for workflow definitions
		InputSummary: &core.SchemaSummary{
			RequiredFields: []core.FieldHint{
				{
					Name:        "workflow_name",
					Type:        "string",
					Example:     "market-research",
					Description: "Name of the workflow to execute",
				},
				{
					Name:        "steps",
					Type:        "array",
					Example:     `[{"tool": "weather-service", "capability": "current_weather"}]`,
					Description: "Array of workflow steps with tool and capability names",
				},
			},
			OptionalFields: []core.FieldHint{
				{
					Name:        "parallel",
					Type:        "boolean",
					Example:     "false",
					Description: "Whether to execute steps in parallel (default: sequential)",
				},
				{
					Name:        "workflow_id",
					Type:        "string",
					Example:     "workflow-67890",
					Description: "Optional workflow tracking identifier",
				},
			},
		},
	})

	// Capability 5: Health check
	r.RegisterCapability(core.Capability{
		Name:        "health",
		Description: "Health check endpoint with dependency status",
		Endpoint:    "/health",
		InputTypes:  []string{},
		OutputTypes: []string{"json"},
		Handler:     r.handleHealth,
	})
}

// getAIProviderStatus returns the detected AI provider name
func getAIProviderStatus() string {
	// Check for common AI provider environment variables
	providers := []struct {
		name   string
		envVar string
	}{
		{"OpenAI", "OPENAI_API_KEY"},
		{"Groq", "GROQ_API_KEY"},
		{"Anthropic", "ANTHROPIC_API_KEY"},
		{"Gemini", "GEMINI_API_KEY"},
		{"DeepSeek", "DEEPSEEK_API_KEY"},
	}

	for _, provider := range providers {
		if os.Getenv(provider.envVar) != "" {
			return provider.name
		}
	}

	// Check for custom OpenAI-compatible endpoints
	if os.Getenv("OPENAI_BASE_URL") != "" {
		return "Custom OpenAI-Compatible"
	}

	return "None (will use mock responses)"
}
