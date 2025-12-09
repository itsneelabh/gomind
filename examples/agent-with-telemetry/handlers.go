package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// handleResearchTopic demonstrates intelligent orchestration with comprehensive telemetry
func (r *ResearchAgent) handleResearchTopic(rw http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	ctx := req.Context()

	// Add span event for Jaeger visibility
	telemetry.AddSpanEvent(ctx, "request_received",
		attribute.String("method", req.Method),
		attribute.String("path", req.URL.Path),
	)

	// Track overall operation duration with deferred call
	// This ensures the metric is emitted even if the function returns early
	var requestStatus = "success" // Will be set to "error" on failure paths
	defer func() {
		durationMs := float64(time.Since(startTime).Milliseconds())

		// Legacy metric (for backwards compatibility)
		telemetry.Histogram("agent.research.duration_ms", durationMs,
			"topic", "",
			"status", "completed")

		// Unified metric (enables cross-module dashboards)
		telemetry.RecordRequest(telemetry.ModuleAgent, "research", durationMs, requestStatus)
	}()

	r.Logger.Info("Starting research topic orchestration", map[string]interface{}{
		"method": req.Method,
		"path":   req.URL.Path,
	})

	var request ResearchRequest
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		r.Logger.Error("Failed to decode research request", map[string]interface{}{
			"error": err.Error(),
		})
		requestStatus = "error"
		telemetry.RecordRequestError(telemetry.ModuleAgent, "research", "validation")
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
		requestStatus = "error"
		telemetry.RecordRequestError(telemetry.ModuleAgent, "research", "discovery")
		http.Error(rw, "Service discovery failed", http.StatusServiceUnavailable)
		return
	}

	// NEW: Track discovery metrics
	telemetry.Gauge("agent.tools.discovered", float64(len(tools)))

	// Add span event for tools discovered
	telemetry.AddSpanEvent(ctx, "tools_discovered",
		attribute.Int("tool_count", len(tools)),
	)

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
		"tool_count":        len(tools),
		"tools_discovered":  toolNames,
		"tool_capabilities": toolCapabilities,
		"topic":             request.Topic,
	})

	// Step 2: Intelligent tool orchestration with AI-powered routing
	var results []ToolResult
	var toolsUsed []string

	// STRATEGY 1: Multi-Entity Comparison (Highest Priority)
	// Try to extract entities for comparison (e.g., "Compare Amazon vs Google")
	entities, err := r.extractEntitiesForComparison(ctx, request.Topic)
	if err == nil && len(entities) >= 2 {
		r.Logger.Info("Multi-entity comparison query detected", map[string]interface{}{
			"topic":        request.Topic,
			"entity_count": len(entities),
			"entities":     entities,
			"strategy":     "parallel tool calls",
		})

		// Use AI to select the most relevant tool AND capability in ONE call
		selections := r.selectToolsAndCapabilities(ctx, request.Topic, tools)
		if len(selections) > 0 {
			selection := selections[0]

			r.Logger.Info("AI selected tool+capability for multi-entity comparison (1 call)", map[string]interface{}{
				"tool":           selection.Tool.Name,
				"capability":     selection.Capability.Name,
				"entity_count":   len(entities),
				"selection_type": "AI-powered (combined)",
			})

			// NEW: Track tool selection
			telemetry.Counter("agent.research.tools_called",
				"tool_name", selection.Tool.Name)

			// Execute parallel tool calls
			entityResults := r.callToolForEntities(ctx, selection.Tool, selection.Capability, request.Topic, entities)
			if len(entityResults) > 0 {
				results = append(results, entityResults...)
				toolsUsed = append(toolsUsed, selection.Tool.Name)

				r.Logger.Info("Multi-entity comparison completed", map[string]interface{}{
					"entities_requested": len(entities),
					"results_received":   len(entityResults),
					"tool":               selection.Tool.Name,
				})
			}
		} else {
			r.Logger.Warn("No relevant tools found for multi-entity comparison", map[string]interface{}{
				"topic":           request.Topic,
				"entities":        entities,
				"available_tools": len(tools),
			})
		}
	} else {
		// STRATEGY 2: Single-Entity Query
		// AI selects the most relevant tool AND capability in ONE call (50% cost savings)
		selections := r.selectToolsAndCapabilities(ctx, request.Topic, tools)

		if len(selections) > 0 {
			selection := selections[0]

			r.Logger.Info("Calling AI-selected tool+capability (1 call)", map[string]interface{}{
				"tool":       selection.Tool.Name,
				"capability": selection.Capability.Name,
				"topic":      request.Topic,
			})

			// NEW: Track tool selection
			telemetry.Counter("agent.research.tools_called",
				"tool_name", selection.Tool.Name)

			// Call the tool with pre-selected capability (no second AI call needed)
			result := r.callToolWithCapability(ctx, selection.Tool, selection.Capability, request.Topic)
			if result != nil {
				r.Logger.Info("Tool call completed", map[string]interface{}{
					"tool":       result.ToolName,
					"capability": result.Capability,
					"success":    result.Success,
					"topic":      request.Topic,
				})
				results = append(results, *result)
				toolsUsed = append(toolsUsed, result.ToolName)
			}
		} else {
			r.Logger.Warn("No relevant tools found for topic", map[string]interface{}{
				"topic":           request.Topic,
				"available_tools": len(tools),
			})
		}
	}

	// Step 3: Use AI to synthesize results
	summary := r.createBasicSummary(request.Topic, results)
	var aiAnalysis string

	if request.UseAI && r.aiClient != nil {
		aiStart := time.Now()

		aiAnalysis = r.generateAIAnalysis(ctx, request.Topic, results)
		aiDurationMs := float64(time.Since(aiStart).Milliseconds())
		aiStatus := "success"
		if aiAnalysis == "" {
			aiStatus = "error"
		}

		// Legacy metrics (for backwards compatibility)
		telemetry.Histogram("agent.ai_synthesis.duration_ms", aiDurationMs)
		telemetry.Counter("agent.ai.requests",
			"provider", "openai",
			"operation", "synthesis")

		// Unified metrics (enables cross-module AI dashboards)
		telemetry.RecordAIRequest(telemetry.ModuleAgent, "openai", aiDurationMs, aiStatus)

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

	// Add completion span event
	telemetry.AddSpanEvent(ctx, "research_completed",
		attribute.String("topic", request.Topic),
		attribute.Int("tools_used", len(toolsUsed)),
		attribute.Int("results_count", len(results)),
		attribute.Float64("confidence", response.Confidence),
	)

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

// handleHealth implements health check endpoint with telemetry status
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

	// NEW: Check telemetry health
	telemetryHealth := telemetry.GetHealth()
	health["telemetry"] = map[string]interface{}{
		"initialized":     telemetryHealth.Initialized,
		"metrics_emitted": telemetryHealth.MetricsEmitted,
		"circuit_state":   telemetryHealth.CircuitState,
	}
	if telemetryHealth.LastError != "" {
		health["telemetry"].(map[string]interface{})["last_error"] = telemetryHealth.LastError
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
