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

	// Import AI providers for auto-detection - these register themselves via init()
	_ "github.com/itsneelabh/gomind/ai/providers/openai"    // OpenAI and compatible services (Groq, DeepSeek, etc.)
	_ "github.com/itsneelabh/gomind/ai/providers/anthropic" // Anthropic Claude  
	_ "github.com/itsneelabh/gomind/ai/providers/gemini"    // Google Gemini
)

// ResearchAgent demonstrates an ENHANCED intelligent agent where EVERY capability uses AI
// This is the "enhanced" version where all 4 capabilities demonstrate different AI integration patterns
type ResearchAgent struct {
	*core.BaseAgent
	aiClient core.AIClient
}

// NewResearchAgent creates a new AI-enhanced research assistant
// Every capability will use AI to provide intelligent responses
func NewResearchAgent() (*ResearchAgent, error) {
	// Step 1: Create the base agent - this gives us Discovery powers (can find and coordinate other components)
	agent := core.NewBaseAgent("research-assistant-enhanced")

	// Step 2: Auto-configure AI client - the framework detects best available provider
	// Priority order: OpenAI → Groq → DeepSeek → Anthropic → Gemini → Custom endpoints
	aiClient, err := ai.NewClient() // Magic happens here - auto-detects from environment
	if err != nil {
		// In production, you might want to fail here, but for demo we'll continue
		// The agent will work but responses will be less intelligent
		log.Printf("⚠️  AI client creation failed: %v", err)
		log.Printf("💡 Agent will work but responses will be basic. Set OPENAI_API_KEY or GROQ_API_KEY for AI-powered responses.")
	}

	// Step 3: Store AI client in both places for consistency with framework patterns
	if aiClient != nil {
		agent.AI = aiClient // Framework standard field
	}

	researchAgent := &ResearchAgent{
		BaseAgent: agent,
		aiClient:  aiClient, // Our internal reference for direct use
	}

	// Step 4: Register ALL capabilities as AI-enhanced
	// This is the key difference from basic agent - every capability uses AI
	researchAgent.registerAIEnhancedCapabilities()
	return researchAgent, nil
}

// registerAIEnhancedCapabilities sets up all capabilities with AI integration
// Each capability demonstrates a different AI usage pattern
func (r *ResearchAgent) registerAIEnhancedCapabilities() {
	// 🎯 CAPABILITY 1: AI-Powered Topic Research (Tool Discovery + AI Synthesis)
	// Pattern: Discover tools → Call tools → AI analyzes and synthesizes results
	r.RegisterCapability(core.Capability{
		Name:        "ai_research_topic",
		Description: "AI-powered research that discovers tools, gathers data, and provides intelligent synthesis",
		InputTypes:  []string{"json", "text"},
		OutputTypes: []string{"json"},
		Handler:     r.handleAIResearchTopic,
	})

	// 🧠 CAPABILITY 2: AI-Enhanced Service Discovery (AI Categorization + Recommendations)
	// Pattern: Discover components → AI categorizes and provides usage recommendations
	r.RegisterCapability(core.Capability{
		Name:        "ai_discover_services",
		Description: "Discovers services and uses AI to categorize and recommend optimal usage patterns",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     r.handleAIDiscoverServices,
	})

	// 💭 CAPABILITY 3: Pure AI Analysis (Direct LLM Processing)
	// Pattern: Direct AI call for analysis and insights
	r.RegisterCapability(core.Capability{
		Name:        "ai_analyze_data",
		Description: "Uses AI to analyze data and provide intelligent insights and recommendations",
		InputTypes:  []string{"json", "text"},
		OutputTypes: []string{"json"},
		Handler:     r.handleAIAnalyzeData,
	})

	// 🎭 CAPABILITY 4: AI Workflow Orchestration (AI Plans Execution)
	// Pattern: AI creates execution plan → Execute plan → AI synthesizes results
	r.RegisterCapability(core.Capability{
		Name:        "ai_orchestrate_workflow",
		Description: "Uses AI to plan and execute complex multi-step workflows dynamically",
		Endpoint:    "/ai-orchestrate", // Custom endpoint example
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     r.handleAIOrchestateWorkflow,
	})
}

// 🎯 CAPABILITY 1: AI-Powered Topic Research
// This demonstrates the full agent → tool → AI → response flow
func (r *ResearchAgent) handleAIResearchTopic(rw http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	
	// Step 1: Extract and validate request
	r.Logger.Info("Starting AI-powered topic research", map[string]interface{}{
		"method": req.Method,
		"path":   req.URL.Path,
	})

	var request struct {
		Topic       string            `json:"topic"`
		MaxResults  int               `json:"max_results,omitempty"`
		UseAI       bool              `json:"use_ai,omitempty"`       // Whether to use AI synthesis (default true for this enhanced version)
		AIMode      string            `json:"ai_mode,omitempty"`      // "analysis", "summary", "recommendations"
		Context     string            `json:"context,omitempty"`      // Additional context for AI
		Metadata    map[string]string `json:"metadata,omitempty"`
	}

	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		r.Logger.Error("Failed to decode research request", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Default to AI-powered analysis if not specified
	if !request.UseAI {
		request.UseAI = true // This is the enhanced version - always use AI
	}
	if request.AIMode == "" {
		request.AIMode = "comprehensive" // Default mode
	}

	ctx := req.Context()

	// Step 2: Use AI to understand the request and plan the research approach
	var researchPlan string
	if r.aiClient != nil {
		planPrompt := fmt.Sprintf(`As a research assistant, I need to research the topic: "%s"

Context: %s
Goal: %s analysis

Please create a research plan that identifies:
1. What specific information should I gather?
2. What type of tools or services would be most relevant?
3. What key questions should I try to answer?
4. How should I structure the final analysis?

Respond with a clear research plan.`, request.Topic, request.Context, request.AIMode)

		planResponse, err := r.aiClient.GenerateResponse(ctx, planPrompt, &core.AIOptions{
			Temperature: 0.4, // Balanced creativity for planning
			MaxTokens:   500,
		})
		if err == nil {
			researchPlan = planResponse.Content
			r.Logger.Info("AI generated research plan", map[string]interface{}{
				"topic": request.Topic,
				"plan_length": len(researchPlan),
			})
		}
	}

	// Step 3: Discover available tools using the framework's Discovery capabilities
	// This is what makes agents different from tools - they can discover and coordinate other components
	tools, err := r.Discovery.Discover(ctx, core.DiscoveryFilter{
		Type: core.ComponentTypeTool, // Only look for tools (passive components)
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
		"has_ai_plan": researchPlan != "",
	})

	// Step 4: AI-powered tool selection - use AI to determine which tools are relevant
	var selectedTools []*core.ServiceInfo
	if r.aiClient != nil && len(tools) > 0 {
		// Create tool descriptions for AI analysis
		toolDescriptions := make([]string, len(tools))
		for i, tool := range tools {
			capNames := make([]string, len(tool.Capabilities))
			for j, cap := range tool.Capabilities {
				capNames[j] = cap.Name
			}
			toolDescriptions[i] = fmt.Sprintf("%s: %s (capabilities: %s)", 
				tool.Name, tool.Description, strings.Join(capNames, ", "))
		}

		selectionPrompt := fmt.Sprintf(`I'm researching: "%s"

Available tools:
%s

Research plan: %s

Which tools would be most relevant for this research? Consider:
1. Relevance to the topic
2. Complementary capabilities
3. Potential for comprehensive coverage

Respond with a JSON array of relevant tool names, like: ["tool1", "tool2"]`, 
			request.Topic, strings.Join(toolDescriptions, "\n"), researchPlan)

		selectionResponse, err := r.aiClient.GenerateResponse(ctx, selectionPrompt, &core.AIOptions{
			Temperature: 0.2, // Lower temperature for factual selection
			MaxTokens:   200,
		})
		if err == nil {
			// Parse AI response to get selected tool names
			var selectedNames []string
			if json.Unmarshal([]byte(selectionResponse.Content), &selectedNames) == nil {
				// Find tools that match AI selection
				for _, tool := range tools {
					for _, selectedName := range selectedNames {
						if strings.Contains(strings.ToLower(tool.Name), strings.ToLower(selectedName)) {
							selectedTools = append(selectedTools, tool)
							break
						}
					}
				}
			}
		}
	}

	// Fallback: if AI selection didn't work, use heuristic selection
	if len(selectedTools) == 0 {
		for _, tool := range tools {
			if r.isToolRelevantHeuristic(tool, request.Topic) {
				selectedTools = append(selectedTools, tool)
			}
		}
	}

	// Step 5: Execute tool calls in parallel where possible
	// This demonstrates the orchestration capabilities of agents
	var results []ToolResult
	var toolsUsed []string

	for _, tool := range selectedTools {
		result := r.callToolWithAI(ctx, tool, request.Topic, request.Context)
		if result != nil {
			results = append(results, *result)
			toolsUsed = append(toolsUsed, result.ToolName)
		}
	}

	// Step 6: AI-powered synthesis of all results
	// This is where the real intelligence happens - combining multiple data sources
	var finalAnalysis string
	var confidence float64 = 0.5 // Default confidence

	if r.aiClient != nil && len(results) > 0 {
		synthesisPrompt := r.buildSynthesisPrompt(request.Topic, request.AIMode, researchPlan, results, request.Context)
		
		synthesisResponse, err := r.aiClient.GenerateResponse(ctx, synthesisPrompt, &core.AIOptions{
			Temperature: 0.6, // Higher creativity for synthesis
			MaxTokens:   1500, // More tokens for comprehensive analysis
		})
		if err == nil {
			finalAnalysis = synthesisResponse.Content
			confidence = r.calculateAIConfidence(results, synthesisResponse.Content)
		}
	}

	// Fallback to basic summary if AI synthesis fails
	if finalAnalysis == "" {
		finalAnalysis = r.createBasicSummary(request.Topic, results)
		confidence = r.calculateBasicConfidence(results)
	}

	// Step 7: Build comprehensive response
	response := map[string]interface{}{
		"topic":           request.Topic,
		"analysis":        finalAnalysis,
		"research_plan":   researchPlan,
		"tools_discovered": len(tools),
		"tools_used":      toolsUsed,
		"results":         results,
		"confidence":      confidence,
		"ai_enhanced":     r.aiClient != nil,
		"processing_time": time.Since(startTime).String(),
		"metadata": map[string]interface{}{
			"ai_mode":        request.AIMode,
			"tool_selection": "ai_powered",
			"synthesis":      "ai_powered",
		},
	}

	// Step 8: Cache the result for future use
	r.cacheResult(ctx, fmt.Sprintf("ai_research:%s", request.Topic), response)

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)

	r.Logger.Info("AI research completed", map[string]interface{}{
		"topic":           request.Topic,
		"tools_used":      len(toolsUsed),
		"confidence":      confidence,
		"processing_time": time.Since(startTime).String(),
	})
}

// 🧠 CAPABILITY 2: AI-Enhanced Service Discovery
// This shows how AI can add intelligence to basic discovery operations
func (r *ResearchAgent) handleAIDiscoverServices(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	// Step 1: Discover all components using framework Discovery
	allComponents, err := r.Discovery.Discover(ctx, core.DiscoveryFilter{})
	if err != nil {
		http.Error(rw, fmt.Sprintf("Discovery failed: %v", err), http.StatusServiceUnavailable)
		return
	}

	// Step 2: Organize components by type
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

	// Step 3: AI-powered categorization and recommendations
	var aiInsights string
	var recommendedWorkflows []map[string]interface{}
	
	if r.aiClient != nil {
		// Create component descriptions for AI analysis
		componentDescriptions := make([]string, 0, len(allComponents))
		for _, comp := range allComponents {
			capNames := make([]string, len(comp.Capabilities))
			for i, cap := range comp.Capabilities {
				capNames[i] = cap.Name
			}
			componentDescriptions = append(componentDescriptions, 
				fmt.Sprintf("- %s (%s): %s | Capabilities: %s", 
					comp.Name, comp.Type, comp.Description, strings.Join(capNames, ", ")))
		}

		analysisPrompt := fmt.Sprintf(`I have discovered the following components in the system:

%s

Please provide:
1. Categorization of components by domain (e.g., data processing, analysis, communication)
2. Potential workflow combinations that would create value
3. Recommendations for system orchestration patterns
4. Identification of any capability gaps

Format your response as a comprehensive analysis.`, 
			strings.Join(componentDescriptions, "\n"))

		analysisResponse, err := r.aiClient.GenerateResponse(ctx, analysisPrompt, &core.AIOptions{
			Temperature: 0.5,
			MaxTokens:   1200,
		})
		if err == nil {
			aiInsights = analysisResponse.Content
		}

		// Generate specific workflow recommendations
		workflowPrompt := fmt.Sprintf(`Based on these available components:

%s

Suggest 3 specific workflows that would demonstrate powerful combinations. For each workflow, provide:
1. Name and purpose
2. Required components in order
3. Expected outcome
4. Business value

Respond in JSON format with an array of workflow objects.`, 
			strings.Join(componentDescriptions, "\n"))

		workflowResponse, err := r.aiClient.GenerateResponse(ctx, workflowPrompt, &core.AIOptions{
			Temperature: 0.4,
			MaxTokens:   800,
		})
		if err == nil {
			// Try to parse JSON response
			json.Unmarshal([]byte(workflowResponse.Content), &recommendedWorkflows)
		}
	}

	// Step 4: Build enhanced discovery response
	response := map[string]interface{}{
		"discovery_summary": map[string]interface{}{
			"total_components": len(allComponents),
			"tools":           len(tools),
			"agents":          len(agents),
			"discovery_time":  time.Now().Format(time.RFC3339),
			"ai_enhanced":     r.aiClient != nil,
		},
		"components": map[string]interface{}{
			"tools":  tools,
			"agents": agents,
		},
		"ai_insights":            aiInsights,
		"recommended_workflows":  recommendedWorkflows,
		"orchestration_suggestions": r.generateOrchestrationSuggestions(tools, agents),
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)
}

// 💭 CAPABILITY 3: Pure AI Analysis
// This demonstrates direct AI processing without tool orchestration
func (r *ResearchAgent) handleAIAnalyzeData(rw http.ResponseWriter, req *http.Request) {
	// Check if AI is available - this capability requires AI
	if r.aiClient == nil {
		http.Error(rw, "AI client not available. Please configure an AI provider (OPENAI_API_KEY, GROQ_API_KEY, etc.)", http.StatusServiceUnavailable)
		return
	}

	var requestData struct {
		Data        string `json:"data"`
		AnalysisType string `json:"analysis_type,omitempty"` // "summary", "insights", "recommendations", "full"
		Context     string `json:"context,omitempty"`
		Domain      string `json:"domain,omitempty"`        // "business", "technical", "financial", etc.
	}

	if err := json.NewDecoder(req.Body).Decode(&requestData); err != nil {
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Default analysis type
	if requestData.AnalysisType == "" {
		requestData.AnalysisType = "full"
	}

	// Create domain-specific analysis prompt
	analysisPrompt := r.buildAnalysisPrompt(requestData.Data, requestData.AnalysisType, requestData.Context, requestData.Domain)

	// Call AI service with appropriate configuration for analysis
	aiResponse, err := r.aiClient.GenerateResponse(req.Context(), analysisPrompt, &core.AIOptions{
		Temperature: 0.3, // Lower temperature for analytical responses
		MaxTokens:   1500, // More tokens for comprehensive analysis
	})
	if err != nil {
		r.Logger.Error("AI analysis failed", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "AI analysis failed", http.StatusInternalServerError)
		return
	}

	// If the domain is business or financial, add additional AI-powered insights
	var additionalInsights string
	if requestData.Domain == "business" || requestData.Domain == "financial" {
		insightPrompt := fmt.Sprintf(`Based on this analysis:

%s

Provide 3 specific actionable recommendations and potential risks to consider.`, aiResponse.Content)

		insightResponse, err := r.aiClient.GenerateResponse(req.Context(), insightPrompt, &core.AIOptions{
			Temperature: 0.4,
			MaxTokens:   500,
		})
		if err == nil {
			additionalInsights = insightResponse.Content
		}
	}

	response := map[string]interface{}{
		"analysis":           aiResponse.Content,
		"additional_insights": additionalInsights,
		"model_used":         aiResponse.Model,
		"tokens_used":        aiResponse.Usage.TotalTokens,
		"analysis_type":      requestData.AnalysisType,
		"domain":            requestData.Domain,
		"confidence":        r.assessAnalysisConfidence(aiResponse.Content),
		"timestamp":         time.Now().Format(time.RFC3339),
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)
}

// 🎭 CAPABILITY 4: AI Workflow Orchestration
// This demonstrates AI creating and executing dynamic workflows
func (r *ResearchAgent) handleAIOrchestateWorkflow(rw http.ResponseWriter, req *http.Request) {
	if r.aiClient == nil {
		http.Error(rw, "AI client not available for workflow orchestration", http.StatusServiceUnavailable)
		return
	}

	var workflowRequest struct {
		Goal          string                 `json:"goal"`           // What the user wants to achieve
		Context       string                 `json:"context"`        // Additional context
		Parameters    map[string]interface{} `json:"parameters"`     // Input parameters
		MaxSteps      int                   `json:"max_steps"`      // Limit complexity
		Parallel      bool                  `json:"parallel"`       // Allow parallel execution
	}

	if err := json.NewDecoder(req.Body).Decode(&workflowRequest); err != nil {
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	ctx := req.Context()
	workflowID := fmt.Sprintf("ai_workflow_%d", time.Now().Unix())

	// Default values
	if workflowRequest.MaxSteps == 0 {
		workflowRequest.MaxSteps = 5
	}

	r.Logger.Info("Starting AI workflow orchestration", map[string]interface{}{
		"workflow_id": workflowID,
		"goal":       workflowRequest.Goal,
		"max_steps":  workflowRequest.MaxSteps,
	})

	// Step 1: Discover available components
	availableComponents, err := r.Discovery.Discover(ctx, core.DiscoveryFilter{})
	if err != nil {
		http.Error(rw, fmt.Sprintf("Component discovery failed: %v", err), http.StatusServiceUnavailable)
		return
	}

	// Step 2: AI creates the execution plan
	executionPlan, err := r.generateAIExecutionPlan(ctx, workflowRequest, availableComponents)
	if err != nil {
		r.Logger.Error("AI execution plan generation failed", map[string]interface{}{
			"workflow_id": workflowID,
			"error":      err.Error(),
		})
		http.Error(rw, fmt.Sprintf("Workflow planning failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Step 3: Execute the AI-generated plan
	executionResult, err := r.executeAIWorkflowPlan(ctx, executionPlan, workflowRequest.Parallel)
	if err != nil {
		r.Logger.Error("AI workflow execution failed", map[string]interface{}{
			"workflow_id": workflowID,
			"error":      err.Error(),
		})
		http.Error(rw, fmt.Sprintf("Workflow execution failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Step 4: AI synthesizes the final result
	finalResult, err := r.synthesizeAIWorkflowResults(ctx, workflowRequest.Goal, executionPlan, executionResult)
	if err != nil {
		r.Logger.Error("AI workflow synthesis failed", map[string]interface{}{
			"workflow_id": workflowID,
			"error":      err.Error(),
		})
		// Continue with basic result if AI synthesis fails
		finalResult = map[string]interface{}{
			"result": "Workflow completed but AI synthesis failed",
			"steps":  executionResult,
		}
	}

	response := map[string]interface{}{
		"workflow_id":     workflowID,
		"goal":           workflowRequest.Goal,
		"execution_plan": executionPlan,
		"result":         finalResult,
		"status":         "completed",
		"ai_generated":   true,
		"completed_at":   time.Now().Format(time.RFC3339),
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)

	r.Logger.Info("AI workflow orchestration completed", map[string]interface{}{
		"workflow_id": workflowID,
		"goal":       workflowRequest.Goal,
	})
}

// Helper Methods - These show how a senior developer would structure supporting code

// buildSynthesisPrompt creates a comprehensive prompt for synthesizing multiple tool results
func (r *ResearchAgent) buildSynthesisPrompt(topic, mode, researchPlan string, results []ToolResult, contextStr string) string {
	prompt := fmt.Sprintf(`I need you to synthesize research results for the topic: "%s"

Research Plan: %s
Analysis Mode: %s
Context: %s

Results from various tools:
`, topic, researchPlan, mode, contextStr)

	for _, result := range results {
		prompt += fmt.Sprintf(`
Tool: %s
Capability: %s
Success: %t
Data: %v
Duration: %s
`, result.ToolName, result.Capability, result.Success, result.Data, result.Duration)
	}

	switch mode {
	case "summary":
		prompt += `
Please provide a concise summary of the key findings from all sources.`
	case "analysis":
		prompt += `
Please provide a detailed analysis including:
1. Key findings from each source
2. Correlations and patterns across sources
3. Confidence assessment for each finding`
	case "recommendations":
		prompt += `
Please provide actionable recommendations based on the data:
1. What actions should be taken?
2. What are the priorities?
3. What risks should be considered?`
	default: // comprehensive
		prompt += `
Please provide a comprehensive analysis including:
1. Executive summary of key findings
2. Detailed analysis of each data source
3. Cross-source correlations and patterns
4. Actionable recommendations
5. Confidence levels and limitations
6. Suggested next steps

Keep the response well-structured and actionable.`
	}

	return prompt
}

// buildAnalysisPrompt creates domain-specific prompts for data analysis
func (r *ResearchAgent) buildAnalysisPrompt(data, analysisType, contextStr, domain string) string {
	basePrompt := fmt.Sprintf(`Analyze the following data:

%s

Context: %s`, data, contextStr)

	// Add domain-specific analysis instructions
	switch domain {
	case "business":
		basePrompt += `

Please provide a business analysis including:
1. Key business metrics and their implications
2. Market trends and competitive positioning
3. Revenue and growth opportunities
4. Risk assessment
5. Strategic recommendations`

	case "financial":
		basePrompt += `

Please provide a financial analysis including:
1. Financial performance indicators
2. Profitability and efficiency metrics
3. Cash flow implications
4. Investment attractiveness
5. Financial risks and opportunities`

	case "technical":
		basePrompt += `

Please provide a technical analysis including:
1. Technical architecture and design patterns
2. Performance characteristics
3. Scalability considerations
4. Technical risks and limitations
5. Implementation recommendations`

	default:
		basePrompt += `

Please provide a comprehensive analysis including:
1. Key findings and insights
2. Patterns and trends
3. Implications and significance
4. Recommendations for action
5. Areas for further investigation`
	}

	// Customize by analysis type
	switch analysisType {
	case "summary":
		basePrompt += `

Focus on providing a concise, high-level summary of the most important points.`
	case "insights":
		basePrompt += `

Focus on extracting deep insights and non-obvious patterns from the data.`
	case "recommendations":
		basePrompt += `

Focus on providing specific, actionable recommendations based on the analysis.`
	}

	return basePrompt
}

// callToolWithAI makes an intelligent tool call enhanced with AI context
func (r *ResearchAgent) callToolWithAI(ctx context.Context, tool *core.ServiceInfo, topic, contextStr string) *ToolResult {
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

	// Use AI to select the best capability if multiple are available
	selectedCapability := tool.Capabilities[0] // Default to first
	if r.aiClient != nil && len(tool.Capabilities) > 1 {
		capabilityNames := make([]string, len(tool.Capabilities))
		capabilityDescriptions := make([]string, len(tool.Capabilities))
		
		for i, cap := range tool.Capabilities {
			capabilityNames[i] = cap.Name
			capabilityDescriptions[i] = fmt.Sprintf("%s: %s", cap.Name, cap.Description)
		}

		selectionPrompt := fmt.Sprintf(`For the research topic "%s" using the %s tool, which capability would be most appropriate?

Available capabilities:
%s

Context: %s

Respond with just the capability name.`, 
			topic, tool.Name, strings.Join(capabilityDescriptions, "\n"), contextStr)

		selection, err := r.aiClient.GenerateResponse(ctx, selectionPrompt, &core.AIOptions{
			Temperature: 0.1,
			MaxTokens:   50,
		})
		if err == nil {
			selectedCapName := strings.TrimSpace(selection.Content)
			for _, cap := range tool.Capabilities {
				if strings.EqualFold(cap.Name, selectedCapName) {
					selectedCapability = cap
					break
				}
			}
		}
	}

	// Build endpoint URL
	endpoint := selectedCapability.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("/api/capabilities/%s", selectedCapability.Name)
	}
	url := fmt.Sprintf("http://%s:%d%s", tool.Address, tool.Port, endpoint)

	// Use AI to create optimized request payload
	var requestData interface{}
	if r.aiClient != nil {
		payloadPrompt := fmt.Sprintf(`Create an optimal request payload for calling the "%s" capability of the %s tool.

Topic: %s
Context: %s
Tool Description: %s
Capability Description: %s

Respond with a JSON object that would be most effective for this tool call.`, 
			selectedCapability.Name, tool.Name, topic, contextStr, tool.Description, selectedCapability.Description)

		payload, err := r.aiClient.GenerateResponse(ctx, payloadPrompt, &core.AIOptions{
			Temperature: 0.2,
			MaxTokens:   200,
		})
		if err == nil {
			json.Unmarshal([]byte(payload.Content), &requestData)
		}
	}

	// Fallback to basic request data
	if requestData == nil {
		if strings.Contains(strings.ToLower(tool.Name), "weather") {
			requestData = map[string]interface{}{
				"location": r.extractLocation(topic),
				"units":    "metric",
			}
		} else {
			requestData = map[string]interface{}{
				"query":  topic,
				"source": "ai-research-assistant",
			}
		}
	}

	// Make the HTTP call
	jsonData, _ := json.Marshal(requestData)
	httpCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return &ToolResult{
			ToolName:   tool.Name,
			Capability: selectedCapability.Name,
			Success:    false,
			Error:      fmt.Sprintf("Request creation failed: %v", err),
			Duration:   time.Since(startTime).String(),
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Research-Context", contextStr) // Pass context in header

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &ToolResult{
			ToolName:   tool.Name,
			Capability: selectedCapability.Name,
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
			Capability: selectedCapability.Name,
			Success:    false,
			Error:      fmt.Sprintf("Response reading failed: %v", err),
			Duration:   time.Since(startTime).String(),
		}
	}

	// Parse response
	var responseData interface{}
	if err := json.Unmarshal(body, &responseData); err != nil {
		responseData = string(body)
	}

	return &ToolResult{
		ToolName:   tool.Name,
		Capability: selectedCapability.Name,
		Data:       responseData,
		Success:    resp.StatusCode >= 200 && resp.StatusCode < 300,
		Duration:   time.Since(startTime).String(),
		Metadata: map[string]interface{}{
			"ai_selected_capability": selectedCapability.Name,
			"ai_generated_payload":   requestData,
		},
	}
}

// Supporting data structures and helper methods

type ToolResult struct {
	ToolName   string                 `json:"tool_name"`
	Capability string                 `json:"capability"`
	Data       interface{}            `json:"data"`
	Success    bool                   `json:"success"`
	Error      string                 `json:"error,omitempty"`
	Duration   string                 `json:"duration"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// AI Workflow orchestration supporting methods

func (r *ResearchAgent) generateAIExecutionPlan(ctx context.Context, request struct {
	Goal          string                 `json:"goal"`
	Context       string                 `json:"context"`
	Parameters    map[string]interface{} `json:"parameters"`
	MaxSteps      int                   `json:"max_steps"`
	Parallel      bool                  `json:"parallel"`
}, components []*core.ServiceInfo) (map[string]interface{}, error) {
	
	componentDescriptions := make([]string, len(components))
	for i, comp := range components {
		capNames := make([]string, len(comp.Capabilities))
		for j, cap := range comp.Capabilities {
			capNames[j] = cap.Name
		}
		componentDescriptions[i] = fmt.Sprintf("- %s (%s): %s [%s]", 
			comp.Name, comp.Type, comp.Description, strings.Join(capNames, ", "))
	}

	planPrompt := fmt.Sprintf(`Create an execution plan to achieve this goal: "%s"

Context: %s
Parameters: %v
Available components:
%s

Create a step-by-step plan (max %d steps) that uses available components effectively. 
If parallel execution is enabled (%t), identify steps that can run concurrently.

Respond with a JSON object containing:
{
  "steps": [
    {
      "id": "step1",
      "component": "component-name",
      "capability": "capability-name", 
      "description": "what this step does",
      "inputs": {...},
      "depends_on": ["step0"],
      "can_parallel": true
    }
  ],
  "execution_order": [["step1"], ["step2", "step3"]],
  "reasoning": "why this plan will achieve the goal"
}`, 
		request.Goal, request.Context, request.Parameters, 
		strings.Join(componentDescriptions, "\n"), 
		request.MaxSteps, request.Parallel)

	response, err := r.aiClient.GenerateResponse(ctx, planPrompt, &core.AIOptions{
		Temperature: 0.4,
		MaxTokens:   1000,
	})
	if err != nil {
		return nil, err
	}

	var executionPlan map[string]interface{}
	if err := json.Unmarshal([]byte(response.Content), &executionPlan); err != nil {
		return nil, fmt.Errorf("failed to parse execution plan: %v", err)
	}

	return executionPlan, nil
}

func (r *ResearchAgent) executeAIWorkflowPlan(ctx context.Context, plan map[string]interface{}, allowParallel bool) (map[string]interface{}, error) {
	// This is a simplified execution - in production, you'd want more sophisticated orchestration
	steps, ok := plan["steps"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid plan format")
	}

	results := make(map[string]interface{})
	
	for _, stepInterface := range steps {
		step, ok := stepInterface.(map[string]interface{})
		if !ok {
			continue
		}

		stepID := step["id"].(string)
		componentName := step["component"].(string)
		capability := step["capability"].(string)

		// Find the component
		components, err := r.Discovery.Discover(ctx, core.DiscoveryFilter{})
		if err != nil {
			continue
		}

		var targetComponent *core.ServiceInfo
		for _, comp := range components {
			if comp.Name == componentName {
				targetComponent = comp
				break
			}
		}

		if targetComponent != nil {
			result := r.callToolWithAI(ctx, targetComponent, capability, "AI workflow execution")
			results[stepID] = result
		}
	}

	return results, nil
}

func (r *ResearchAgent) synthesizeAIWorkflowResults(ctx context.Context, goal string, plan, results map[string]interface{}) (map[string]interface{}, error) {
	synthesisPrompt := fmt.Sprintf(`Synthesize the results of this workflow execution:

Goal: %s
Execution Plan: %v
Results: %v

Provide:
1. Whether the goal was achieved
2. Key outcomes and insights
3. Success metrics
4. Any issues encountered
5. Recommendations for future improvements

Format as a JSON object with "achieved", "summary", "insights", "recommendations" fields.`, 
		goal, plan, results)

	response, err := r.aiClient.GenerateResponse(ctx, synthesisPrompt, &core.AIOptions{
		Temperature: 0.3,
		MaxTokens:   800,
	})
	if err != nil {
		return nil, err
	}

	var synthesis map[string]interface{}
	if err := json.Unmarshal([]byte(response.Content), &synthesis); err != nil {
		return map[string]interface{}{
			"achieved": false,
			"summary":  response.Content,
			"error":   "Failed to parse structured response",
		}, nil
	}

	return synthesis, nil
}

// Utility methods for confidence calculation and heuristics

func (r *ResearchAgent) calculateAIConfidence(results []ToolResult, aiAnalysis string) float64 {
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}
	
	baseConfidence := float64(successCount) / float64(len(results))
	
	// Adjust based on AI analysis quality (simple heuristic)
	analysisQuality := 0.5
	if len(aiAnalysis) > 500 { // Longer analysis tends to be more comprehensive
		analysisQuality = 0.8
	}
	if len(aiAnalysis) > 1000 {
		analysisQuality = 0.9
	}
	
	return (baseConfidence + analysisQuality) / 2.0
}

func (r *ResearchAgent) calculateBasicConfidence(results []ToolResult) float64 {
	if len(results) == 0 {
		return 0.0
	}
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}
	return float64(successCount) / float64(len(results))
}

func (r *ResearchAgent) assessAnalysisConfidence(analysis string) float64 {
	// Simple heuristic based on analysis characteristics
	confidence := 0.5
	
	// Longer analysis suggests more thorough thinking
	if len(analysis) > 800 {
		confidence += 0.2
	}
	
	// Presence of specific indicators suggests structured thinking
	indicators := []string{"analysis", "recommendation", "conclusion", "evidence", "data"}
	for _, indicator := range indicators {
		if strings.Contains(strings.ToLower(analysis), indicator) {
			confidence += 0.05
		}
	}
	
	if confidence > 1.0 {
		confidence = 1.0
	}
	
	return confidence
}

func (r *ResearchAgent) isToolRelevantHeuristic(tool *core.ServiceInfo, topic string) bool {
	topic = strings.ToLower(topic)
	toolName := strings.ToLower(tool.Name)
	
	// Basic keyword matching
	keywords := []string{"weather", "data", "analysis", "news", "financial", "search"}
	for _, keyword := range keywords {
		if strings.Contains(topic, keyword) && strings.Contains(toolName, keyword) {
			return true
		}
	}
	
	// Check capabilities
	for _, cap := range tool.Capabilities {
		capName := strings.ToLower(cap.Name)
		if strings.Contains(topic, capName) || strings.Contains(capName, topic) {
			return true
		}
	}
	
	return false
}

func (r *ResearchAgent) extractLocation(topic string) string {
	// Simple location extraction - in production, use NLP
	words := strings.Fields(strings.ToLower(topic))
	locations := []string{"new york", "london", "tokyo", "paris", "sydney", "san francisco", "chicago", "miami"}
	
	for _, location := range locations {
		for _, word := range words {
			if strings.Contains(location, word) {
				return location
			}
		}
	}
	return "New York" // Default
}

func (r *ResearchAgent) createBasicSummary(topic string, results []ToolResult) string {
	successful := 0
	for _, result := range results {
		if result.Success {
			successful++
		}
	}

	return fmt.Sprintf("Research completed for '%s'. Successfully gathered data from %d out of %d tools. "+
		"Results include information from various sources. AI synthesis was not available.", 
		topic, successful, len(results))
}

func (r *ResearchAgent) cacheResult(ctx context.Context, key string, data interface{}) {
	if r.Memory != nil {
		cacheData, _ := json.Marshal(data)
		r.Memory.Set(ctx, key, string(cacheData), 15*time.Minute)
	}
}

func (r *ResearchAgent) generateOrchestrationSuggestions(tools, agents []*core.ServiceInfo) []map[string]interface{} {
	suggestions := []map[string]interface{}{}
	
	if len(tools) > 0 && len(agents) > 0 {
		suggestions = append(suggestions, map[string]interface{}{
			"pattern": "Tool → Agent Orchestration",
			"description": "Use tools to gather data, then agents to process and analyze",
			"example": "weather-tool → analysis-agent → recommendation",
		})
	}
	
	if len(tools) >= 2 {
		suggestions = append(suggestions, map[string]interface{}{
			"pattern": "Multi-Tool Aggregation",
			"description": "Gather data from multiple tools simultaneously",
			"example": "weather-tool + news-tool → combined analysis",
		})
	}
	
	return suggestions
}

func main() {
	// Create enhanced research agent with full AI integration
	agent, err := NewResearchAgent()
	if err != nil {
		log.Fatalf("Failed to create research agent: %v", err)
	}

	// Framework handles all the infrastructure including auto-injection of Discovery
	framework, err := core.NewFramework(agent,
		// Core configuration
		core.WithName("research-assistant-enhanced"),
		core.WithPort(8091), // Different port from basic agent
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

	log.Println("🤖 Enhanced Research Assistant Agent Starting...")
	log.Println("🧠 AI Provider:", getAIProviderStatus())
	log.Println("📍 Endpoints available:")
	log.Println("   - POST /api/capabilities/ai_research_topic")
	log.Println("   - POST /api/capabilities/ai_discover_services")
	log.Println("   - POST /api/capabilities/ai_analyze_data")
	log.Println("   - POST /ai-orchestrate")
	log.Println("   - GET  /api/capabilities (list all capabilities)")
	log.Println("   - GET  /health (health check)")
	log.Println()
	log.Println("🔗 Test commands:")
	log.Println(`   # AI-powered topic research`)
	log.Println(`   curl -X POST http://localhost:8091/api/capabilities/ai_research_topic \`)
	log.Println(`     -H "Content-Type: application/json" \`)
	log.Println(`     -d '{"topic":"AI trends in healthcare","ai_mode":"comprehensive","use_ai":true}'`)
	log.Println()
	log.Println(`   # AI-enhanced service discovery`)
	log.Println(`   curl -X POST http://localhost:8091/api/capabilities/ai_discover_services \`)
	log.Println(`     -H "Content-Type: application/json" -d '{}'`)
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
	
	if os.Getenv("OPENAI_BASE_URL") != "" {
		return "Custom OpenAI-Compatible"
	}
	
	return "None (agent will work but responses will be basic)"
}