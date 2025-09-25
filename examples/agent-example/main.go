package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"

	// Import AI providers for auto-detection
	_ "github.com/itsneelabh/gomind/ai/providers/openai"    // OpenAI and compatible services
	_ "github.com/itsneelabh/gomind/ai/providers/anthropic" // Anthropic Claude  
	_ "github.com/itsneelabh/gomind/ai/providers/gemini"    // Google Gemini
)

// ResearchAgent is an intelligent agent that discovers and orchestrates tools
// It demonstrates the active agent pattern - can discover and coordinate other components
type ResearchAgent struct {
	*core.BaseAgent
	aiClient core.AIClient
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
		BaseAgent: agent,
		aiClient:  aiClient,
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
	})

	// Capability 2: Component discovery and status
	r.RegisterCapability(core.Capability{
		Name:        "discover_tools",
		Description: "Discovers available tools and their capabilities",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     r.handleDiscoverTools,
	})

	// Capability 3: AI-powered analysis (if AI is available)
	r.RegisterCapability(core.Capability{
		Name:        "analyze_data",
		Description: "Uses AI to analyze and synthesize data from multiple sources",
		InputTypes:  []string{"json", "text"},
		OutputTypes: []string{"json"},
		Handler:     r.handleAnalyzeData,
	})

	// Capability 4: Workflow orchestration
	r.RegisterCapability(core.Capability{
		Name:        "orchestrate_workflow",
		Description: "Orchestrates a multi-step workflow using discovered tools",
		Endpoint:    "/orchestrate", // Custom endpoint
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     r.handleOrchestateWorkflow,
	})
}

// ResearchRequest represents the input for research operations
type ResearchRequest struct {
	Topic       string            `json:"topic"`
	Sources     []string          `json:"sources,omitempty"`     // Specific sources to use
	MaxResults  int               `json:"max_results,omitempty"` // Limit results
	Metadata    map[string]string `json:"metadata,omitempty"`    // Additional parameters
	UseAI       bool              `json:"use_ai,omitempty"`      // Whether to use AI analysis
	WorkflowID  string            `json:"workflow_id,omitempty"` // For workflow tracking
}

// ResearchResponse represents the synthesized research output
type ResearchResponse struct {
	Topic          string                   `json:"topic"`
	Summary        string                   `json:"summary"`
	ToolsUsed      []string                 `json:"tools_used"`
	Results        []ToolResult             `json:"results"`
	AIAnalysis     string                   `json:"ai_analysis,omitempty"`
	Confidence     float64                  `json:"confidence"`
	ProcessingTime string                   `json:"processing_time"`
	WorkflowID     string                   `json:"workflow_id,omitempty"`
	Metadata       map[string]interface{}   `json:"metadata,omitempty"`
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

// handleResearchTopic demonstrates intelligent orchestration
func (r *ResearchAgent) handleResearchTopic(rw http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	
	r.Logger.Info("Starting research topic orchestration", map[string]interface{}{
		"method": req.Method,
		"path":   req.URL.Path,
	})

	var request ResearchRequest
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		r.Logger.Error("Failed to decode research request", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	ctx := req.Context()

	// Step 1: Discover available tools
	tools, err := r.Discovery.Discover(ctx, core.DiscoveryFilter{
		Type: core.ComponentTypeTool, // Only look for tools
	})
	if err != nil {
		r.Logger.Error("Failed to discover tools", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "Service discovery failed", http.StatusServiceUnavailable)
		return
	}

	r.Logger.Info("Discovered tools for research", map[string]interface{}{
		"tool_count": len(tools),
		"topic":      request.Topic,
	})

	// Step 2: Orchestrate tool calls based on topic
	var results []ToolResult
	var toolsUsed []string

	// Look for weather tools if topic is weather-related
	if r.isWeatherRelated(request.Topic) {
		weatherResult := r.callWeatherTool(ctx, tools, request.Topic)
		if weatherResult != nil {
			results = append(results, *weatherResult)
			toolsUsed = append(toolsUsed, weatherResult.ToolName)
		}
	}

	// Look for other relevant tools
	for _, tool := range tools {
		if r.isToolRelevant(tool, request.Topic) {
			result := r.callTool(ctx, tool, request.Topic)
			if result != nil {
				results = append(results, *result)
				toolsUsed = append(toolsUsed, result.ToolName)
			}
		}
	}

	// Step 3: Use AI to synthesize results (if available and requested)
	summary := r.createBasicSummary(request.Topic, results)
	var aiAnalysis string
	
	if request.UseAI && r.aiClient != nil {
		aiAnalysis = r.generateAIAnalysis(ctx, request.Topic, results)
		if aiAnalysis != "" {
			summary = aiAnalysis // Use AI analysis as the summary
		}
	}

	// Step 4: Build response
	response := ResearchResponse{
		Topic:          request.Topic,
		Summary:        summary,
		ToolsUsed:      toolsUsed,
		Results:        results,
		AIAnalysis:     aiAnalysis,
		Confidence:     r.calculateConfidence(results),
		ProcessingTime: time.Since(startTime).String(),
		WorkflowID:     request.WorkflowID,
		Metadata: map[string]interface{}{
			"tools_discovered": len(tools),
			"tools_used":      len(toolsUsed),
			"ai_enabled":      r.aiClient != nil,
		},
	}

	// Cache the result
	r.cacheResult(ctx, request.Topic, response)

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)

	r.Logger.Info("Research topic completed", map[string]interface{}{
		"topic":          request.Topic,
		"tools_used":     len(toolsUsed),
		"processing_time": time.Since(startTime).String(),
	})
}

// handleDiscoverTools shows available tools and their capabilities
func (r *ResearchAgent) handleDiscoverTools(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	// Discover all components
	allComponents, err := r.Discovery.Discover(ctx, core.DiscoveryFilter{})
	if err != nil {
		http.Error(rw, fmt.Sprintf("Discovery failed: %v", err), http.StatusServiceUnavailable)
		return
	}

	// Organize by type
	tools := make([]*core.ServiceInfo, 0)
	agents := make([]*core.ServiceInfo, 0)

	for _, component := range allComponents {
		switch component.Type {
		case core.ComponentTypeTool:
			tools = append(tools, component)
		case core.ComponentTypeAgent:
			// Don't include ourselves in the list
			if component.ID != r.GetID() {
				agents = append(agents, component)
			}
		}
	}

	response := map[string]interface{}{
		"discovery_summary": map[string]interface{}{
			"total_components": len(allComponents),
			"tools":           len(tools),
			"agents":          len(agents),
			"discovery_time":  time.Now().Format(time.RFC3339),
		},
		"tools":  tools,
		"agents": agents,
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)
}

// handleAnalyzeData demonstrates AI-powered data analysis
func (r *ResearchAgent) handleAnalyzeData(rw http.ResponseWriter, req *http.Request) {
	if r.aiClient == nil {
		http.Error(rw, "AI client not available", http.StatusServiceUnavailable)
		return
	}

	var requestData map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&requestData); err != nil {
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Extract data to analyze
	data, ok := requestData["data"].(string)
	if !ok {
		http.Error(rw, "Missing 'data' field in request", http.StatusBadRequest)
		return
	}

	// Create analysis prompt
	prompt := fmt.Sprintf(`Analyze the following data and provide insights:

%s

Please provide:
1. Key findings
2. Patterns or trends
3. Recommendations
4. Confidence level in your analysis`, data)

	// Call AI service
	aiResponse, err := r.aiClient.GenerateResponse(req.Context(), prompt, &core.AIOptions{
		Temperature: 0.3, // Lower temperature for more analytical response
		MaxTokens:   1000,
	})
	if err != nil {
		r.Logger.Error("AI analysis failed", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "AI analysis failed", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"analysis":    aiResponse.Content,
		"model":       aiResponse.Model,
		"tokens_used": aiResponse.Usage.TotalTokens,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)
}

// handleOrchestateWorkflow demonstrates complex workflow orchestration
func (r *ResearchAgent) handleOrchestateWorkflow(rw http.ResponseWriter, req *http.Request) {
	var workflowReq struct {
		WorkflowType string                 `json:"workflow_type"`
		Parameters   map[string]interface{} `json:"parameters"`
		Steps        []string               `json:"steps,omitempty"`
	}

	if err := json.NewDecoder(req.Body).Decode(&workflowReq); err != nil {
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	ctx := req.Context()
	workflowID := fmt.Sprintf("workflow-%d", time.Now().Unix())

	r.Logger.Info("Starting workflow orchestration", map[string]interface{}{
		"workflow_id":   workflowID,
		"workflow_type": workflowReq.WorkflowType,
	})

	// Execute based on workflow type
	var result interface{}
	var err error

	switch workflowReq.WorkflowType {
	case "weather_analysis":
		result, err = r.orchestrateWeatherAnalysis(ctx, workflowReq.Parameters)
	case "data_pipeline":
		result, err = r.orchestrateDataPipeline(ctx, workflowReq.Parameters)
	default:
		result, err = r.orchestrateGenericWorkflow(ctx, workflowReq.WorkflowType, workflowReq.Parameters)
	}

	if err != nil {
		r.Logger.Error("Workflow orchestration failed", map[string]interface{}{
			"workflow_id": workflowID,
			"error":      err.Error(),
		})
		http.Error(rw, fmt.Sprintf("Workflow failed: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"workflow_id":   workflowID,
		"workflow_type": workflowReq.WorkflowType,
		"result":        result,
		"status":        "completed",
		"completed_at":  time.Now().Format(time.RFC3339),
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)

	r.Logger.Info("Workflow orchestration completed", map[string]interface{}{
		"workflow_id": workflowID,
	})
}

// Helper methods for orchestration logic

func (r *ResearchAgent) isWeatherRelated(topic string) bool {
	keywords := []string{"weather", "temperature", "rain", "storm", "forecast", "climate"}
	topic = strings.ToLower(topic)
	for _, keyword := range keywords {
		if strings.Contains(topic, keyword) {
			return true
		}
	}
	return false
}

func (r *ResearchAgent) isToolRelevant(tool *core.ServiceInfo, topic string) bool {
	// Simple relevance matching - in production, this could be more sophisticated
	topic = strings.ToLower(topic)
	
	// Check tool name and capabilities for relevance
	for _, capability := range tool.Capabilities {
		if strings.Contains(strings.ToLower(capability.Name), topic) ||
		   strings.Contains(strings.ToLower(capability.Description), topic) {
			return true
		}
	}
	return false
}

func (r *ResearchAgent) callWeatherTool(ctx context.Context, tools []*core.ServiceInfo, topic string) *ToolResult {
	for _, tool := range tools {
		if strings.Contains(strings.ToLower(tool.Name), "weather") {
			return r.callTool(ctx, tool, topic)
		}
	}
	return nil
}

func (r *ResearchAgent) callTool(ctx context.Context, tool *core.ServiceInfo, topic string) *ToolResult {
	startTime := time.Now()
	
	if len(tool.Capabilities) == 0 {
		return &ToolResult{
			ToolName:   tool.Name,
			Capability: "unknown",
			Success:    false,
			Error:      "No capabilities available",
			Duration:   time.Since(startTime).String(),
		}
	}

	// Try to call the first capability
	capability := tool.Capabilities[0]
	endpoint := capability.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("/api/capabilities/%s", capability.Name)
	}

	// Build request URL
	url := fmt.Sprintf("http://%s:%d%s", tool.Address, tool.Port, endpoint)
	
	// Create request payload
	requestData := map[string]interface{}{
		"query": topic,
		"source": "research-assistant",
	}
	
	// For weather tools, structure the request properly
	if strings.Contains(strings.ToLower(tool.Name), "weather") {
		requestData = map[string]interface{}{
			"location": extractLocation(topic),
			"units": "metric",
		}
	}

	jsonData, _ := json.Marshal(requestData)

	// Make HTTP call to the tool
	httpCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return &ToolResult{
			ToolName:   tool.Name,
			Capability: capability.Name,
			Success:    false,
			Error:      fmt.Sprintf("Request creation failed: %v", err),
			Duration:   time.Since(startTime).String(),
		}
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &ToolResult{
			ToolName:   tool.Name,
			Capability: capability.Name,
			Success:    false,
			Error:      fmt.Sprintf("HTTP call failed: %v", err),
			Duration:   time.Since(startTime).String(),
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ToolResult{
			ToolName:   tool.Name,
			Capability: capability.Name,
			Success:    false,
			Error:      fmt.Sprintf("Response reading failed: %v", err),
			Duration:   time.Since(startTime).String(),
		}
	}

	// Parse response
	var responseData interface{}
	if err := json.Unmarshal(body, &responseData); err != nil {
		// If JSON parsing fails, use raw response
		responseData = string(body)
	}

	return &ToolResult{
		ToolName:   tool.Name,
		Capability: capability.Name,
		Data:       responseData,
		Success:    resp.StatusCode >= 200 && resp.StatusCode < 300,
		Duration:   time.Since(startTime).String(),
	}
}

func (r *ResearchAgent) generateAIAnalysis(ctx context.Context, topic string, results []ToolResult) string {
	if r.aiClient == nil {
		return ""
	}

	// Build analysis prompt
	prompt := fmt.Sprintf(`I need you to analyze research results for the topic: "%s"

Results from various tools:
`, topic)

	for _, result := range results {
		prompt += fmt.Sprintf("\nTool: %s\nCapability: %s\nSuccess: %t\nData: %v\n", 
			result.ToolName, result.Capability, result.Success, result.Data)
	}

	prompt += `
Please provide:
1. A comprehensive summary of the findings
2. Key insights from the data
3. Any correlations or patterns
4. Confidence level in the analysis

Keep the response concise and focused.`

	response, err := r.aiClient.GenerateResponse(ctx, prompt, &core.AIOptions{
		Temperature: 0.4,
		MaxTokens:   800,
	})
	if err != nil {
		r.Logger.Error("AI analysis generation failed", map[string]interface{}{
			"error": err.Error(),
		})
		return ""
	}

	return response.Content
}

func (r *ResearchAgent) createBasicSummary(topic string, results []ToolResult) string {
	successful := 0
	for _, result := range results {
		if result.Success {
			successful++
		}
	}

	return fmt.Sprintf("Research completed for '%s'. Successfully gathered data from %d out of %d tools. "+
		"Results include information from various sources.", topic, successful, len(results))
}

func (r *ResearchAgent) calculateConfidence(results []ToolResult) float64 {
	if len(results) == 0 {
		return 0.0
	}

	successful := 0
	for _, result := range results {
		if result.Success {
			successful++
		}
	}

	return float64(successful) / float64(len(results))
}

func (r *ResearchAgent) cacheResult(ctx context.Context, topic string, result ResearchResponse) {
	cacheKey := fmt.Sprintf("research:%s", strings.ToLower(strings.ReplaceAll(topic, " ", "_")))
	cacheData, _ := json.Marshal(result)
	r.Memory.Set(ctx, cacheKey, string(cacheData), 15*time.Minute)
}

func (r *ResearchAgent) orchestrateWeatherAnalysis(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Example weather analysis workflow
	tools, err := r.Discovery.Discover(ctx, core.DiscoveryFilter{
		Type: core.ComponentTypeTool,
	})
	if err != nil {
		return nil, err
	}

	var weatherData interface{}
	for _, tool := range tools {
		if strings.Contains(strings.ToLower(tool.Name), "weather") {
			result := r.callTool(ctx, tool, "current weather analysis")
			if result != nil && result.Success {
				weatherData = result.Data
				break
			}
		}
	}

	return map[string]interface{}{
		"analysis_type": "weather",
		"data":         weatherData,
		"parameters":   params,
		"timestamp":    time.Now().Format(time.RFC3339),
	}, nil
}

func (r *ResearchAgent) orchestrateDataPipeline(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Example data pipeline workflow
	return map[string]interface{}{
		"pipeline_type": "data_processing",
		"status":       "completed",
		"parameters":   params,
		"processed_at": time.Now().Format(time.RFC3339),
	}, nil
}

func (r *ResearchAgent) orchestrateGenericWorkflow(ctx context.Context, workflowType string, params map[string]interface{}) (interface{}, error) {
	// Generic workflow handler
	return map[string]interface{}{
		"workflow_type": workflowType,
		"status":       "completed",
		"parameters":   params,
		"message":      fmt.Sprintf("Generic workflow '%s' executed successfully", workflowType),
	}, nil
}

func extractLocation(topic string) string {
	// Simple location extraction - in production, use NLP
	words := strings.Fields(strings.ToLower(topic))
	locations := []string{"new york", "london", "tokyo", "paris", "sydney"}
	
	for _, location := range locations {
		for _, word := range words {
			if strings.Contains(location, word) {
				return location
			}
		}
	}
	return "New York" // Default location
}

func main() {
	// Create research agent
	agent, err := NewResearchAgent()
	if err != nil {
		log.Fatalf("Failed to create research agent: %v", err)
	}

	// Framework handles all the complexity including auto-injection of Discovery
	framework, err := core.NewFramework(agent,
		// Core configuration
		core.WithName("research-assistant"),
		core.WithPort(8090),
		core.WithNamespace("examples"),

		// Discovery configuration (agents get full discovery powers)
		core.WithRedisURL(getEnvOrDefault("REDIS_URL", "redis://localhost:6379")),
		core.WithDiscovery(true, "redis"),

		// CORS for web access
		core.WithCORS([]string{"*"}, true),

		// Development mode
		core.WithDevelopmentMode(true),
	)
	if err != nil {
		log.Fatalf("Failed to create framework: %v", err)
	}

	log.Println("🤖 Research Assistant Agent Starting...")
	log.Println("🧠 AI Provider:", getAIProviderStatus())
	log.Println("📍 Endpoints available:")
	log.Println("   - POST /api/capabilities/research_topic")
	log.Println("   - POST /api/capabilities/discover_tools")
	log.Println("   - POST /api/capabilities/analyze_data")
	log.Println("   - POST /orchestrate")
	log.Println("   - GET  /api/capabilities (list all capabilities)")
	log.Println("   - GET  /health (health check)")
	log.Println()
	log.Println("🔗 Test commands:")
	log.Println(`   # Discover available tools`)
	log.Println(`   curl -X POST http://localhost:8090/api/capabilities/discover_tools \`)
	log.Println(`     -H "Content-Type: application/json" -d '{}'`)
	log.Println()
	log.Println(`   # Research a topic (orchestrates multiple tools)`)
	log.Println(`   curl -X POST http://localhost:8090/api/capabilities/research_topic \`)
	log.Println(`     -H "Content-Type: application/json" \`)
	log.Println(`     -d '{"topic":"weather in New York","use_ai":true}'`)
	log.Println()

	// Run the framework (blocking)
	ctx := context.Background()
	if err := framework.Run(ctx); err != nil {
		log.Fatalf("Framework execution failed: %v", err)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getAIProviderStatus() string {
	// Check for common AI provider environment variables
	providers := []struct {
		name string
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