package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"
)

// OrchestrationAgent demonstrates multi-modal orchestration patterns
// This example shows how to coordinate multiple agents using different routing strategies:
// 1. Autonomous Mode: AI-driven dynamic routing based on request analysis  
// 2. Workflow Mode: Predefined recipe-based execution with dependencies
// 3. Hybrid Mode: Combines both approaches for complex scenarios
//
// Note: This example provides mock responses. For real orchestration, you would:
// 1. Create an AIOrchestrator with: orchestration.NewAIOrchestrator(config, discovery, aiClient)
// 2. Use orchestrator.ProcessRequest() to handle actual multi-agent coordination
type OrchestrationAgent struct {
	*core.BaseAgent
	aiClient core.AIClient
}

// NewOrchestrationAgent creates a new orchestration agent with framework integration
func NewOrchestrationAgent() *OrchestrationAgent {
	// Step 1: Create the base agent using current framework API
	baseAgent := core.NewBaseAgent("orchestration-agent")
	
	agent := &OrchestrationAgent{
		BaseAgent: baseAgent,
	}
	
	// Step 2: Initialize AI client if available
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		aiClient, err := ai.NewClient(
			ai.WithProvider("openai"),
			ai.WithAPIKey(apiKey),
		)
		if err != nil {
			agent.Logger.Warn("Failed to initialize AI client", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			agent.aiClient = aiClient
			agent.Logger.Info("AI client initialized successfully", map[string]interface{}{
				"provider": "openai",
				"model":    "gpt-4",
			})
		}
	} else {
		agent.Logger.Info("No OpenAI API key provided, AI features disabled", map[string]interface{}{})
	}
	
	// Step 3: Register capabilities
	// Note: In a real implementation, you would initialize an AIOrchestrator here:
	// discovery := ... // Get from framework
	// config := orchestration.DefaultConfig()
	// orchestrator := orchestration.NewAIOrchestrator(config, discovery, agent.aiClient)
	
	// Step 4: Register capabilities
	agent.registerCapabilities()
	
	return agent
}

// registerCapabilities registers the orchestration capabilities
func (a *OrchestrationAgent) registerCapabilities() {
	// Capability 1: Process requests using AI orchestration
	a.RegisterCapability(core.Capability{
		Name:        "process_request",
		Description: "Process complex requests using intelligent orchestration",
		Handler:     a.handleProcessRequest,
	})
	
	// Capability 2: Execute predefined workflows
	a.RegisterCapability(core.Capability{
		Name:        "execute_workflow",
		Description: "Execute predefined workflows with dependency management",
		Handler:     a.handleExecuteWorkflow,
	})
	
	// Capability 3: Get orchestration status
	a.RegisterCapability(core.Capability{
		Name:        "status",
		Description: "Get orchestration agent status and metrics",
		Handler:     a.handleStatus,
	})
	
	a.Logger.Info("Registered orchestration capabilities", map[string]interface{}{
		"capabilities": 3,
	})
}

// handleProcessRequest processes requests using AI orchestration
func (a *OrchestrationAgent) handleProcessRequest(w http.ResponseWriter, r *http.Request) {
	
	var request struct {
		Query    string                 `json:"query"`
		Context  map[string]interface{} `json:"context,omitempty"`
		Priority string                 `json:"priority,omitempty"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	
	a.Logger.Info("Processing orchestration request", map[string]interface{}{
		"query":    request.Query,
		"priority": request.Priority,
	})
	
	// MOCK RESPONSE: This example provides a mock response for demonstration.
	// To use real orchestration:
	// 1. Initialize discovery (e.g., Redis) in the framework
	// 2. Create AIOrchestrator: orchestration.NewAIOrchestrator(config, discovery, aiClient)
	// 3. Call orchestrator.ProcessRequest(r.Context(), request.Query, request.Context)
	// 4. Ensure other agents are running and registered in discovery
	//
	// Example of real implementation:
	// response, err := a.orchestrator.ProcessRequest(r.Context(), request.Query, request.Context)
	// if err != nil {
	//     http.Error(w, err.Error(), http.StatusInternalServerError)
	//     return
	// }
	response := map[string]interface{}{
		"status":    "success",
		"query":     request.Query,
		"response":  fmt.Sprintf("Processed request: %s", request.Query),
		"mode":      "autonomous",
		"timestamp": time.Now(),
		"agents":    []string{"analysis-agent", "data-agent"},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleExecuteWorkflow executes predefined workflows
func (a *OrchestrationAgent) handleExecuteWorkflow(w http.ResponseWriter, r *http.Request) {
	
	var request struct {
		WorkflowName string                 `json:"workflow_name"`
		Inputs       map[string]interface{} `json:"inputs,omitempty"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	
	a.Logger.Info("Executing workflow", map[string]interface{}{
		"workflow": request.WorkflowName,
		"inputs":   len(request.Inputs),
	})
	
	// Mock workflow execution - in a real implementation this would
	// execute actual workflow steps
	response := map[string]interface{}{
		"status":        "completed",
		"workflow_name": request.WorkflowName,
		"execution_id":  fmt.Sprintf("exec-%d", time.Now().Unix()),
		"mode":          "workflow",
		"steps": []map[string]interface{}{
			{
				"step_id": "step-1",
				"status":  "completed",
				"agent":   "data-fetcher",
			},
			{
				"step_id": "step-2", 
				"status":  "completed",
				"agent":   "analyzer",
			},
		},
		"duration":  "2.5s",
		"timestamp": time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStatus returns orchestration agent status
func (a *OrchestrationAgent) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"agent_name":      a.Name,
		"status":          "healthy",
		"ai_enabled":      a.aiClient != nil,
		"capabilities":    []string{"process_request", "execute_workflow", "status"},
		"orchestrator":    "active",
		"uptime":         time.Since(time.Now().Add(-time.Hour)).String(), // Mock uptime
		"timestamp":      time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func main() {
	// Step 1: Create the orchestration agent
	agent := NewOrchestrationAgent()
	
	// Step 2: Setup framework with proper configuration
	framework, err := core.NewFramework(agent,
		// Core configuration
		core.WithName("orchestration-agent"),
		core.WithPort(8080),
		core.WithNamespace("examples"),

		// Service discovery
		core.WithRedisURL(getEnvOrDefault("REDIS_URL", "redis://localhost:6379")),
		core.WithDiscovery(true, "redis"),

		// Web configuration  
		core.WithCORS([]string{"*"}, true),
		core.WithDevelopmentMode(true),
	)
	if err != nil {
		log.Fatalf("Failed to create framework: %v", err)
	}
	
	// Step 3: Setup context
	ctx := context.Background()
	
	// Step 4: Setup HTTP server for demonstration
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	
	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "orchestration-agent",
			"timestamp": time.Now(),
			"modes":     []string{"autonomous", "workflow", "hybrid"},
		})
	})
	
	// Process request endpoint - AI orchestration  
	router.POST("/orchestrate/process", func(c *gin.Context) {
		agent.handleProcessRequest(c.Writer, c.Request)
	})
	
	// Execute workflow endpoint
	router.POST("/orchestrate/workflow", func(c *gin.Context) {
		agent.handleExecuteWorkflow(c.Writer, c.Request)
	})
	
	// Status endpoint
	router.GET("/status", func(c *gin.Context) {
		agent.handleStatus(c.Writer, c.Request)
	})
	
	// Capabilities endpoint (automatic via framework)
	router.GET("/api/capabilities", func(c *gin.Context) {
		capabilities := agent.GetCapabilities()
		c.JSON(http.StatusOK, capabilities)
	})
	
	// Step 5: Start the HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	
	log.Printf("Starting Orchestration Agent on port %s", port)
	log.Printf("Available endpoints:")
	log.Printf("  GET  /health - Service health check")
	log.Printf("  POST /orchestrate/process - AI-driven request processing")
	log.Printf("  POST /orchestrate/workflow - Execute predefined workflows")
	log.Printf("  GET  /status - Agent status and metrics")
	log.Printf("  GET  /api/capabilities - List agent capabilities")
	log.Printf("")
	log.Printf("Orchestration modes supported:")
	log.Printf("  - Autonomous: AI analyzes requests and routes dynamically")
	log.Printf("  - Workflow: Predefined templates with dependency management")
	log.Printf("  - Hybrid: Combines AI flexibility with workflow structure")
	log.Printf("")
	
	// Start the framework in the background
	go func() {
		if err := framework.Run(ctx); err != nil {
			log.Printf("Framework error: %v", err)
		}
	}()
	
	// Start HTTP server
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

// getEnvOrDefault gets environment variable with fallback
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}