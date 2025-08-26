package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	framework "github.com/itsneelabh/gomind"
	"github.com/itsneelabh/gomind/pkg/ai"
	"github.com/itsneelabh/gomind/pkg/communication"
	"github.com/itsneelabh/gomind/pkg/discovery"
	"github.com/itsneelabh/gomind/pkg/logger"
	"github.com/itsneelabh/gomind/pkg/orchestration"
	"github.com/itsneelabh/gomind/pkg/routing"
)

// OrchestratorAgent is a specialized agent that coordinates multi-agent workflows
type OrchestratorAgent struct {
	*framework.BaseAgent
	orchestrator orchestration.Orchestrator
	logger       logger.Logger
}

// NewOrchestratorAgent creates a new orchestrator agent
func NewOrchestratorAgent() (*OrchestratorAgent, error) {
	// Create base agent
	baseAgent := &framework.BaseAgent{}
	
	// Create logger
	logger := logger.NewSimpleLogger()
	logger.SetLevel("INFO")
	
	// Initialize AI client
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}
	aiClient := ai.NewOpenAIClient(apiKey, logger)
	
	// Initialize discovery
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}
	discovery, err := discovery.NewRedisDiscovery(redisURL, "orchestrator", "agents")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize discovery: %w", err)
	}
	
	// Initialize communicator
	communicator := communication.NewK8sCommunicator(discovery, logger, "agents")
	
	// Initialize router based on mode
	routingMode := os.Getenv("ROUTING_MODE")
	if routingMode == "" {
		routingMode = "hybrid"
	}
	
	var router routing.Router
	switch routingMode {
	case "autonomous":
		router = routing.NewAutonomousRouter(aiClient,
			routing.WithModel("gpt-4"),
			routing.WithTemperature(0.3))
		
	case "workflow":
		workflowPath := os.Getenv("WORKFLOW_PATH")
		if workflowPath == "" {
			workflowPath = "./workflows"
		}
		workflowRouter, err := routing.NewWorkflowRouter(workflowPath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize workflow router: %w", err)
		}
		router = workflowRouter
		
	case "hybrid":
		workflowPath := os.Getenv("WORKFLOW_PATH")
		if workflowPath == "" {
			workflowPath = "./workflows"
		}
		hybridRouter, err := routing.NewHybridRouter(workflowPath, aiClient,
			routing.WithPreferWorkflow(true),
			routing.WithConfidenceThreshold(0.7))
		if err != nil {
			return nil, fmt.Errorf("failed to initialize hybrid router: %w", err)
		}
		router = hybridRouter
		
	default:
		return nil, fmt.Errorf("unknown routing mode: %s", routingMode)
	}
	
	// Update router with agent catalog
	if discovery != nil {
		catalog := discovery.GetCatalogForLLM()
		router.SetAgentCatalog(catalog)
	}
	
	// Create orchestrator configuration
	config := orchestration.DefaultConfig()
	config.RoutingMode = routing.RouterMode(routingMode)
	
	// Override with environment variables if set
	if os.Getenv("MAX_CONCURRENCY") != "" {
		fmt.Sscanf(os.Getenv("MAX_CONCURRENCY"), "%d", &config.ExecutionOptions.MaxConcurrency)
	}
	if os.Getenv("SYNTHESIS_STRATEGY") != "" {
		config.SynthesisStrategy = orchestration.SynthesisStrategy(os.Getenv("SYNTHESIS_STRATEGY"))
	}
	
	// Create orchestrator
	orchestrator := orchestration.NewOrchestrator(
		router,
		communicator,
		aiClient,
		logger,
		config,
	)
	
	return &OrchestratorAgent{
		BaseAgent:    baseAgent,
		orchestrator: orchestrator,
		logger:       logger,
	}, nil
}

// ProcessRequest handles natural language requests by orchestrating multiple agents
func (o *OrchestratorAgent) ProcessRequest(ctx context.Context, instruction string) (string, error) {
	o.logger.Info("Processing orchestrator request", map[string]interface{}{
		"instruction": instruction,
	})
	
	// Extract metadata if provided
	metadata := make(map[string]interface{})
	metadata["timestamp"] = time.Now().Unix()
	metadata["source"] = "orchestrator-agent"
	
	// Process request through orchestrator
	response, err := o.orchestrator.ProcessRequest(ctx, instruction, metadata)
	if err != nil {
		o.logger.Error("Orchestration failed", map[string]interface{}{
			"error": err.Error(),
		})
		return "", err
	}
	
	// Log metrics
	metrics := o.orchestrator.GetMetrics()
	o.logger.Info("Orchestration completed", map[string]interface{}{
		"request_id":      response.RequestID,
		"execution_time":  response.ExecutionTime,
		"agents_involved": len(response.AgentsInvolved),
		"total_requests":  metrics.TotalRequests,
		"success_rate":    float64(metrics.SuccessfulRequests) / float64(metrics.TotalRequests),
	})
	
	return response.Response, nil
}

// GetName returns the agent name
func (o *OrchestratorAgent) GetName() string {
	return "orchestrator"
}

// GetVersion returns the agent version
func (o *OrchestratorAgent) GetVersion() string {
	return "1.0.0"
}

// GetCapabilities returns the agent capabilities
func (o *OrchestratorAgent) GetCapabilities() []framework.CapabilityMetadata {
	return []framework.CapabilityMetadata{
		{
			Name:        "orchestrate",
			Description: "Orchestrate multi-agent workflows",
		},
		{
			Name:        "route",
			Description: "Route requests to appropriate agents",
		},
		{
			Name:        "synthesize",
			Description: "Synthesize responses from multiple agents",
		},
	}
}

// Initialize is called when the agent starts
func (o *OrchestratorAgent) Initialize(ctx context.Context) error {
	o.logger.Info("Initializing orchestrator agent", nil)
	return nil
}

// Shutdown is called when the agent stops
func (o *OrchestratorAgent) Shutdown(ctx context.Context) error {
	o.logger.Info("Shutting down orchestrator agent", nil)
	return nil
}

// Custom HTTP handlers
func (o *OrchestratorAgent) setupCustomHandlers(mux *http.ServeMux) {
	// Metrics endpoint
	mux.HandleFunc("/orchestrator/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := o.orchestrator.GetMetrics()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	})
	
	// History endpoint
	mux.HandleFunc("/orchestrator/history", func(w http.ResponseWriter, r *http.Request) {
		history := o.orchestrator.GetExecutionHistory()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(history)
	})
	
	// Orchestrate endpoint (alternative to /process)
	mux.HandleFunc("/orchestrate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		var request struct {
			Prompt   string                 `json:"prompt"`
			Metadata map[string]interface{} `json:"metadata,omitempty"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		
		response, err := o.orchestrator.ProcessRequest(r.Context(), request.Prompt, request.Metadata)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
}

func main() {
	// Create orchestrator agent
	orchAgent, err := NewOrchestratorAgent()
	if err != nil {
		log.Fatalf("Failed to create orchestrator agent: %v", err)
	}
	
	// Run agent with framework
	if err := framework.RunAgent(orchAgent,
		framework.WithAgentName("orchestrator"),
		framework.WithPort(8080),
	); err != nil {
		log.Fatalf("Failed to run orchestrator agent: %v", err)
	}
}