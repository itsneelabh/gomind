package main

import (
	"log"
	"net/http"
	"time"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
)

// ResearchAgent is an intelligent agent that demonstrates the active agent pattern with telemetry.
// It can discover available tools via Redis, orchestrate multiple tool calls, and
// synthesize results using AI, while emitting comprehensive metrics for observability.
//
// Key Features:
//   - Tool Discovery: Automatically finds available tools in the service mesh
//   - Smart Orchestration: Routes requests to appropriate tools based on topic analysis
//   - Multi-Entity Support: Detects comparison queries and makes parallel tool calls
//   - Hybrid AI Mode: Uses tools when available, falls back to direct AI when not
//   - Performance: Connection pooling, caching, and parallel execution
//   - Telemetry: Comprehensive metrics, tracing, and health monitoring
type ResearchAgent struct {
	*core.BaseAgent
	aiClient    core.AIClient
	httpClient  *http.Client     // Shared HTTP client for connection pooling
	SchemaCache core.SchemaCache // Optional schema cache for Phase 3 validation
}

// ResearchRequest represents the input for research operations.
// This is the main request format accepted by the research_topic capability.
type ResearchRequest struct {
	// Topic is the research query or question (required)
	// Examples: "weather in Paris", "Compare SF vs LA weather"
	Topic string `json:"topic"`

	// Sources optionally specifies which tools to use
	// If empty, agent automatically discovers and selects tools
	Sources []string `json:"sources,omitempty"`

	// MaxResults limits the number of results to return (default: 5)
	MaxResults int `json:"max_results,omitempty"`

	// Metadata provides additional context or parameters
	Metadata map[string]string `json:"metadata,omitempty"`

	// UseAI enables AI-powered analysis and synthesis
	// When true with no tools: AI answers directly (hybrid mode)
	// When true with tools: AI synthesizes tool results
	// When false: Returns raw tool data only
	UseAI bool `json:"use_ai,omitempty"`

	// WorkflowID enables tracking across multiple related requests
	WorkflowID string `json:"workflow_id,omitempty"`
}

// ResearchResponse represents the synthesized research output.
// Contains both raw tool results and AI-generated analysis when enabled.
type ResearchResponse struct {
	Topic          string                 `json:"topic"`                 // Original research topic
	Summary        string                 `json:"summary"`               // Text summary of findings
	ToolsUsed      []string               `json:"tools_used"`            // Names of tools that were called
	Results        []ToolResult           `json:"results"`               // Detailed results from each tool
	AIAnalysis     string                 `json:"ai_analysis,omitempty"` // AI-generated insights
	Confidence     float64                `json:"confidence"`            // Confidence score (0-1)
	ProcessingTime string                 `json:"processing_time"`       // Total time taken
	WorkflowID     string                 `json:"workflow_id,omitempty"` // Workflow tracking ID
	Metadata       map[string]interface{} `json:"metadata,omitempty"`    // Additional metadata
}

// ToolResult represents the result from a single tool call.
// For multi-entity queries, there will be one result per entity.
type ToolResult struct {
	ToolName   string      `json:"tool_name"`       // Name of the tool that was called
	Capability string      `json:"capability"`      // Specific capability used
	Data       interface{} `json:"data"`            // Tool-specific response data
	Success    bool        `json:"success"`         // Whether the call succeeded
	Error      string      `json:"error,omitempty"` // Error message if failed
	Duration   string      `json:"duration"`        // Time taken for this call
}

// NewResearchAgent creates a new AI-powered research assistant with telemetry
func NewResearchAgent() (*ResearchAgent, error) {
	agent := core.NewBaseAgent("research-assistant-telemetry")

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

	// NEW: Declare metrics this agent will emit
	// These declarations help with validation and documentation
	telemetry.DeclareMetrics("research-agent", telemetry.ModuleConfig{
		Metrics: []telemetry.MetricDefinition{
			{
				Name:    "agent.research.duration_ms",
				Type:    "histogram",
				Help:    "Research operation duration in milliseconds",
				Labels:  []string{"topic", "status"},
				Unit:    "milliseconds",
				Buckets: []float64{100, 500, 1000, 5000, 10000, 30000},
			},
			{
				Name:   "agent.research.tools_called",
				Type:   "counter",
				Help:   "Number of tool calls made during research",
				Labels: []string{"tool_name"},
			},
			{
				Name:   "agent.research.ai_tokens",
				Type:   "gauge",
				Help:   "AI tokens used in synthesis",
				Labels: []string{"provider", "operation"},
			},
			{
				Name:    "agent.tool_call.duration_ms",
				Type:    "histogram",
				Help:    "Individual tool call duration in milliseconds",
				Labels:  []string{"tool"},
				Unit:    "milliseconds",
				Buckets: []float64{50, 100, 250, 500, 1000, 2000, 5000},
			},
			{
				Name:   "agent.tool_call.errors",
				Type:   "counter",
				Help:   "Tool call failures by error type",
				Labels: []string{"tool", "error_type"},
			},
			{
				Name:   "agent.tool_call.success",
				Type:   "counter",
				Help:   "Successful tool calls",
				Labels: []string{"tool"},
			},
			{
				Name: "agent.tools.discovered",
				Type: "gauge",
				Help: "Number of tools discovered via service discovery",
			},
			{
				Name:    "agent.ai_synthesis.duration_ms",
				Type:    "histogram",
				Help:    "AI synthesis operation duration",
				Unit:    "milliseconds",
				Buckets: []float64{100, 500, 1000, 2000, 5000},
			},
			{
				Name:   "agent.ai.requests",
				Type:   "counter",
				Help:   "AI API requests made",
				Labels: []string{"provider", "operation"},
			},
		},
	})

	researchAgent := &ResearchAgent{
		BaseAgent: agent,
		aiClient:  aiClient,
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
