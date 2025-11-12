package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// handleResearchTopic demonstrates intelligent orchestration
func (r *ResearchAgent) handleResearchTopic(rw http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	ctx := req.Context()

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

	// Log discovered tools with their names
	toolNames := make([]string, 0, len(tools))
	toolCapabilities := make(map[string][]string)
	for _, tool := range tools {
		toolNames = append(toolNames, tool.Name)
		capNames := make([]string, 0, len(tool.Capabilities))
		for _, cap := range tool.Capabilities {
			capNames = append(capNames, cap.Name)
		}
		toolCapabilities[tool.Name] = capNames
	}

	r.Logger.Info("Discovered tools for research", map[string]interface{}{
		"tool_count":         len(tools),
		"tools_discovered":   toolNames,
		"tool_capabilities":  toolCapabilities,
		"topic":              request.Topic,
	})

	// Step 2: Orchestrate tool calls based on topic
	var results []ToolResult
	var toolsUsed []string

	// Look for weather tools if topic is weather-related
	if r.isWeatherRelated(request.Topic) {
		r.Logger.Info("Topic identified as weather-related, selecting weather tool", map[string]interface{}{
			"topic":  request.Topic,
			"reason": "contains weather keywords (weather, temperature, rain, storm, forecast, climate)",
		})
		weatherResult := r.callWeatherTool(ctx, tools, request.Topic)
		if weatherResult != nil {
			r.Logger.Info("Weather tool selected and called", map[string]interface{}{
				"tool":       weatherResult.ToolName,
				"capability": weatherResult.Capability,
				"success":    weatherResult.Success,
				"topic":      request.Topic,
			})
			results = append(results, *weatherResult)
			toolsUsed = append(toolsUsed, weatherResult.ToolName)
		} else {
			r.Logger.Warn("No weather tool found despite weather-related topic", map[string]interface{}{
				"topic": request.Topic,
			})
		}
	}

	// Look for other relevant tools
	for _, tool := range tools {
		if r.isToolRelevant(tool, request.Topic) {
			r.Logger.Info("Tool identified as relevant for topic", map[string]interface{}{
				"tool":   tool.Name,
				"topic":  request.Topic,
				"reason": "tool capabilities match topic keywords",
			})
			result := r.callTool(ctx, tool, request.Topic)
			if result != nil {
				r.Logger.Info("Tool selected and called", map[string]interface{}{
					"tool":       result.ToolName,
					"capability": result.Capability,
					"success":    result.Success,
					"topic":      request.Topic,
				})
				results = append(results, *result)
				toolsUsed = append(toolsUsed, result.ToolName)
			}
		}
	}

	// Step 3: Use AI to synthesize results
	summary := r.createBasicSummary(request.Topic, results)
	var aiAnalysis string

	if request.UseAI && r.aiClient != nil {
		aiAnalysis = r.generateAIAnalysis(ctx, request.Topic, results)
		if aiAnalysis != "" {
			summary = aiAnalysis // Use AI analysis as the summary
			r.Logger.Info("AI analysis completed", map[string]interface{}{
				"topic": request.Topic,
			})
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
			"tools_used":       len(toolsUsed),
			"ai_enabled":       r.aiClient != nil,
		},
	}

	// Cache the result
	r.cacheResult(ctx, request.Topic, response)

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)

	r.Logger.Info("Research topic completed", map[string]interface{}{
		"topic":           request.Topic,
		"tools_used":      len(toolsUsed),
		"processing_time": time.Since(startTime).String(),
	})
}

// handleDiscoverTools shows available tools and their capabilities
func (r *ResearchAgent) handleDiscoverTools(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	r.Logger.Info("Discovering components", map[string]interface{}{
		"path": req.URL.Path,
	})

	// Discover all components
	allComponents, err := r.Discovery.Discover(ctx, core.DiscoveryFilter{})
	if err != nil {
		r.Logger.Error("Discovery failed", map[string]interface{}{
			"error": err.Error(),
		})
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
			"tools":            len(tools),
			"agents":           len(agents),
			"discovery_time":   time.Now().Format(time.RFC3339),
		},
		"tools":  tools,
		"agents": agents,
	}

	r.Logger.Info("Discovery completed", map[string]interface{}{
		"total_components": len(allComponents),
		"tools":            len(tools),
		"agents":           len(agents),
	})

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)
}

// handleAnalyzeData demonstrates AI-powered data analysis
func (r *ResearchAgent) handleAnalyzeData(rw http.ResponseWriter, req *http.Request) {
	if r.aiClient == nil {
		r.Logger.Error("AI analysis requested but AI client not available", nil)
		http.Error(rw, "AI client not available", http.StatusServiceUnavailable)
		return
	}

	r.Logger.Info("Starting AI data analysis", map[string]interface{}{
		"path": req.URL.Path,
	})

	var requestData map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&requestData); err != nil {
		r.Logger.Error("Failed to decode analysis request", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Extract data to analyze
	data, ok := requestData["data"].(string)
	if !ok {
		r.Logger.Error("Missing data field in request", nil)
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

	r.Logger.Info("AI analysis completed", map[string]interface{}{
		"model":       aiResponse.Model,
		"tokens_used": aiResponse.Usage.TotalTokens,
	})

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
		r.Logger.Error("Failed to decode workflow request", map[string]interface{}{
			"error": err.Error(),
		})
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
			"error":       err.Error(),
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

// handleHealth implements health check endpoint
func (r *ResearchAgent) handleHealth(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	startTime := time.Now()

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	}

	// Check Redis connection
	if r.Discovery != nil {
		_, err := r.Discovery.Discover(ctx, core.DiscoveryFilter{})
		if err != nil {
			health["status"] = "degraded"
			health["redis"] = "unavailable"
			r.Logger.Error("Health check: Redis unavailable", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			health["redis"] = "healthy"
		}
	}

	// Check AI provider
	if r.aiClient != nil {
		health["ai_provider"] = "connected"
	} else {
		health["ai_provider"] = "not configured"
	}

	// Set appropriate status code
	statusCode := http.StatusOK
	if health["status"] == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	} else if health["status"] == "degraded" {
		statusCode = http.StatusOK // Still return 200 for degraded but functional
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(health)

	r.Logger.Debug("Health check completed", map[string]interface{}{
		"status":   health["status"],
		"duration": time.Since(startTime).String(),
	})
}
