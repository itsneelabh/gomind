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

// WorkflowAgent demonstrates YAML-based workflow execution patterns
// This example shows how to coordinate multiple agents using predefined workflows
//
// Note: This example provides mock responses. For real workflow execution, you would:
// 1. Create an AIOrchestrator with: orchestration.NewAIOrchestrator(config, discovery, aiClient)
// 2. Use orchestrator.ExecutePlan() to execute predefined workflows
type WorkflowAgent struct {
	*core.BaseAgent
	aiClient core.AIClient
}

// NewWorkflowAgent creates a new workflow agent with framework integration
func NewWorkflowAgent() *WorkflowAgent {
	// Step 1: Create the base agent using current framework API
	baseAgent := core.NewBaseAgent("workflow-agent")
	
	agent := &WorkflowAgent{
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
			})
		}
	} else {
		agent.Logger.Info("No OpenAI API key provided, AI features disabled", map[string]interface{}{})
	}
	
	// Step 3: Register capabilities
	// Note: In a real implementation, you would initialize an AIOrchestrator here:
	// discovery := ... // Get from framework
	// config := orchestration.DefaultConfig()
	// config.ExecutionOptions.MaxConcurrency = 3
	// orchestrator := orchestration.NewAIOrchestrator(config, discovery, agent.aiClient)
	
	// Step 4: Register capabilities
	agent.registerCapabilities()
	
	return agent
}

// registerCapabilities registers the workflow capabilities
func (a *WorkflowAgent) registerCapabilities() {
	// Capability 1: Execute predefined workflows
	a.RegisterCapability(core.Capability{
		Name:        "execute_workflow",
		Description: "Execute predefined workflows with step-by-step orchestration",
		Handler:     a.handleExecuteWorkflow,
	})
	
	// Capability 2: List available workflows
	a.RegisterCapability(core.Capability{
		Name:        "list_workflows",
		Description: "List all available workflow templates",
		Handler:     a.handleListWorkflows,
	})
	
	// Capability 3: Get workflow status
	a.RegisterCapability(core.Capability{
		Name:        "workflow_status",
		Description: "Get status of workflow executions",
		Handler:     a.handleWorkflowStatus,
	})
	
	a.Logger.Info("Registered workflow capabilities", map[string]interface{}{
		"capabilities": 3,
	})
}

// handleExecuteWorkflow executes predefined workflows
func (a *WorkflowAgent) handleExecuteWorkflow(w http.ResponseWriter, r *http.Request) {
	var request struct {
		WorkflowName string                 `json:"workflow_name"`
		Inputs       map[string]interface{} `json:"inputs,omitempty"`
		Priority     string                 `json:"priority,omitempty"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	
	a.Logger.Info("Executing workflow", map[string]interface{}{
		"workflow": request.WorkflowName,
		"inputs":   len(request.Inputs),
		"priority": request.Priority,
	})
	
	// Mock workflow execution - in a real implementation this would
	// load and execute actual YAML workflow definitions
	response := a.executeWorkflowSteps(request.WorkflowName, request.Inputs)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleListWorkflows lists available workflow templates
func (a *WorkflowAgent) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	workflows := []map[string]interface{}{
		{
			"name":        "data-processing",
			"description": "Process data through validation, transformation, and storage steps",
			"steps":       4,
			"duration":    "5-10 minutes",
		},
		{
			"name":        "deployment-pipeline",
			"description": "Deploy application with testing, security scan, and rollout",
			"steps":       6,
			"duration":    "10-15 minutes",
		},
		{
			"name":        "user-onboarding",
			"description": "Onboard new users with account setup and verification",
			"steps":       3,
			"duration":    "2-5 minutes",
		},
	}
	
	response := map[string]interface{}{
		"workflows":     workflows,
		"total_count":   len(workflows),
		"agent":         a.Name,
		"timestamp":     time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleWorkflowStatus returns workflow execution status
func (a *WorkflowAgent) handleWorkflowStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"agent_name":       a.Name,
		"status":           "healthy",
		"ai_enabled":       a.aiClient != nil,
		"orchestrator":     "active",
		"active_workflows": 0, // Mock count
		"completed_today":  12, // Mock count
		"uptime":          time.Since(time.Now().Add(-2*time.Hour)).String(), // Mock uptime
		"timestamp":       time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// executeWorkflowSteps simulates executing a workflow with multiple steps
func (a *WorkflowAgent) executeWorkflowSteps(workflowName string, inputs map[string]interface{}) map[string]interface{} {
	startTime := time.Now()
	
	// Mock workflow step execution based on workflow name
	var steps []map[string]interface{}
	
	switch workflowName {
	case "data-processing":
		steps = []map[string]interface{}{
			{"step_id": "validate", "status": "completed", "agent": "validator", "duration": "1.2s"},
			{"step_id": "transform", "status": "completed", "agent": "transformer", "duration": "2.1s"},
			{"step_id": "enrich", "status": "completed", "agent": "enricher", "duration": "1.8s"},
			{"step_id": "store", "status": "completed", "agent": "storage", "duration": "0.9s"},
		}
	case "deployment-pipeline":
		steps = []map[string]interface{}{
			{"step_id": "build", "status": "completed", "agent": "builder", "duration": "3.2s"},
			{"step_id": "test", "status": "completed", "agent": "tester", "duration": "2.5s"},
			{"step_id": "security-scan", "status": "completed", "agent": "security", "duration": "1.9s"},
			{"step_id": "deploy-staging", "status": "completed", "agent": "deployer", "duration": "2.8s"},
			{"step_id": "health-check", "status": "completed", "agent": "monitor", "duration": "1.1s"},
			{"step_id": "deploy-prod", "status": "completed", "agent": "deployer", "duration": "3.4s"},
		}
	case "user-onboarding":
		steps = []map[string]interface{}{
			{"step_id": "create-account", "status": "completed", "agent": "account-manager", "duration": "0.8s"},
			{"step_id": "send-verification", "status": "completed", "agent": "notification", "duration": "0.5s"},
			{"step_id": "setup-preferences", "status": "completed", "agent": "preferences", "duration": "0.7s"},
		}
	default:
		steps = []map[string]interface{}{
			{"step_id": "unknown-workflow", "status": "failed", "error": "Workflow not found"},
		}
	}
	
	executionTime := time.Since(startTime)
	
	return map[string]interface{}{
		"workflow_name":   workflowName,
		"execution_id":    fmt.Sprintf("exec-%d", time.Now().Unix()),
		"status":          "completed",
		"mode":            "workflow",
		"steps":           steps,
		"total_steps":     len(steps),
		"execution_time":  executionTime.String(),
		"inputs_received": len(inputs),
		"timestamp":       time.Now(),
	}
}

func main() {
	// Step 1: Create the workflow agent
	agent := NewWorkflowAgent()
	
	// Step 2: Setup framework with proper configuration
	framework, err := core.NewFramework(agent,
		// Core configuration
		core.WithName("workflow-agent"),
		core.WithPort(8081),
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
	
	// Step 3: Setup HTTP server for demonstration
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	
	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "workflow-agent",
			"timestamp": time.Now(),
			"workflows": []string{"data-processing", "deployment-pipeline", "user-onboarding"},
		})
	})
	
	// Execute workflow endpoint
	router.POST("/workflows/execute", func(c *gin.Context) {
		agent.handleExecuteWorkflow(c.Writer, c.Request)
	})
	
	// List workflows endpoint
	router.GET("/workflows", func(c *gin.Context) {
		agent.handleListWorkflows(c.Writer, c.Request)
	})
	
	// Workflow status endpoint
	router.GET("/workflows/status", func(c *gin.Context) {
		agent.handleWorkflowStatus(c.Writer, c.Request)
	})
	
	// Capabilities endpoint (automatic via framework)
	router.GET("/api/capabilities", func(c *gin.Context) {
		capabilities := agent.GetCapabilities()
		c.JSON(http.StatusOK, capabilities)
	})
	
	// Step 4: Start the HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	
	log.Printf("Starting Workflow Agent on port %s", port)
	log.Printf("Available endpoints:")
	log.Printf("  GET  /health - Service health check")
	log.Printf("  POST /workflows/execute - Execute predefined workflows")
	log.Printf("  GET  /workflows - List available workflow templates")
	log.Printf("  GET  /workflows/status - Get workflow execution status")
	log.Printf("  GET  /api/capabilities - List agent capabilities")
	log.Printf("")
	log.Printf("Available workflow templates:")
	log.Printf("  - data-processing: Process data through validation and transformation")
	log.Printf("  - deployment-pipeline: Deploy applications with full CI/CD pipeline")
	log.Printf("  - user-onboarding: Onboard new users with account setup")
	log.Printf("")
	
	// Start the framework in the background
	go func() {
		ctx := context.Background()
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