// Package main demonstrates multi-provider AI configuration with automatic fallback patterns
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"

	// Import AI providers for multi-provider support
	_ "github.com/itsneelabh/gomind/ai/providers/openai"    // OpenAI and compatible services
	_ "github.com/itsneelabh/gomind/ai/providers/anthropic" // Anthropic Claude
	_ "github.com/itsneelabh/gomind/ai/providers/gemini"    // Google Gemini
)

// MultiProviderService demonstrates both Tool and Agent implementations using multi-provider AI
type MultiProviderService struct {
	// Tool mode
	*core.BaseTool
	
	// Agent mode  
	*core.BaseAgent

	// AI clients for different providers
	primaryClient   core.AIClient
	fallbackClient  core.AIClient
	secondaryClient core.AIClient

	// Configuration
	deploymentMode string // "tool", "agent", or "both"
}

// NewMultiProviderService creates a service that can run as tool, agent, or both
func NewMultiProviderService() (*MultiProviderService, error) {
	deploymentMode := getEnvOrDefault("DEPLOYMENT_MODE", "tool")
	
	service := &MultiProviderService{
		deploymentMode: deploymentMode,
	}

	// Initialize components based on deployment mode
	switch deploymentMode {
	case "tool":
		service.BaseTool = core.NewTool("multi-provider-ai-tool")
	case "agent":
		service.BaseAgent = core.NewBaseAgent("multi-provider-ai-agent")
	case "both":
		service.BaseTool = core.NewTool("multi-provider-ai-tool")
		service.BaseAgent = core.NewBaseAgent("multi-provider-ai-agent")
	default:
		return nil, fmt.Errorf("invalid DEPLOYMENT_MODE: %s (use 'tool', 'agent', or 'both')", deploymentMode)
	}

	// Setup multi-provider AI clients
	if err := service.setupAIClients(); err != nil {
		log.Printf("Warning: AI setup failed, continuing with limited functionality: %v", err)
	}

	// Register capabilities for both modes
	service.registerCapabilities()

	return service, nil
}

// setupAIClients configures multiple AI providers with fallback
func (s *MultiProviderService) setupAIClients() error {
	// Primary provider (OpenAI)
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		client, err := ai.NewClient(
			ai.WithProvider("openai"),
			ai.WithAPIKey(apiKey),
			ai.WithModel("gpt-4"),
			ai.WithTemperature(0.3),
		)
		if err == nil {
			s.primaryClient = client
			log.Println("✅ Primary AI provider (OpenAI) configured")
		} else {
			log.Printf("⚠️ Primary provider setup failed: %v", err)
		}
	}

	// Fallback provider (Anthropic)
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		client, err := ai.NewClient(
			ai.WithProvider("anthropic"),
			ai.WithAPIKey(apiKey),
			ai.WithModel("claude-3-sonnet"),
			ai.WithTemperature(0.3),
		)
		if err == nil {
			s.fallbackClient = client
			log.Println("✅ Fallback AI provider (Anthropic) configured")
		} else {
			log.Printf("⚠️ Fallback provider setup failed: %v", err)
		}
	}

	// Secondary provider (Gemini)
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		client, err := ai.NewClient(
			ai.WithProvider("gemini"),
			ai.WithAPIKey(apiKey),
			ai.WithModel("gemini-pro"),
			ai.WithTemperature(0.3),
		)
		if err == nil {
			s.secondaryClient = client
			log.Println("✅ Secondary AI provider (Gemini) configured")
		} else {
			log.Printf("⚠️ Secondary provider setup failed: %v", err)
		}
	}

	// Auto-detect if no explicit providers configured
	if s.primaryClient == nil && s.fallbackClient == nil && s.secondaryClient == nil {
		client, err := ai.NewClient() // Auto-detects from environment
		if err != nil {
			return fmt.Errorf("no AI providers available: %w", err)
		}
		s.primaryClient = client
		log.Println("✅ Auto-detected AI provider configured")
	}

	return nil
}

// registerCapabilities adds capabilities for both tool and agent modes
func (s *MultiProviderService) registerCapabilities() {
	// Tool mode capabilities
	if s.BaseTool != nil {
		s.BaseTool.RegisterCapability(core.Capability{
			Name:        "compare_providers",
			Description: "Compare responses from multiple AI providers",
			InputTypes:  []string{"json"},
			OutputTypes: []string{"json"},
			Handler:     s.handleCompareProviders,
		})

		s.BaseTool.RegisterCapability(core.Capability{
			Name:        "provider_health",
			Description: "Check health and availability of AI providers",
			InputTypes:  []string{"json"},
			OutputTypes: []string{"json"},
			Handler:     s.handleProviderHealth,
		})
	}

	// Agent mode capabilities (if BaseAgent exists)
	if s.BaseAgent != nil {
		s.BaseAgent.RegisterCapability(core.Capability{
			Name:        "intelligent_routing",
			Description: "Route requests to best available AI provider",
			InputTypes:  []string{"json"},
			OutputTypes: []string{"json"},
			Handler:     s.handleIntelligentRouting,
		})

		s.BaseAgent.RegisterCapability(core.Capability{
			Name:        "provider_orchestration",
			Description: "Orchestrate multiple AI providers for complex tasks",
			InputTypes:  []string{"json"},
			OutputTypes: []string{"json"},
			Handler:     s.handleProviderOrchestration,
		})
	}
}

// AIRequest represents input for AI operations
type AIRequest struct {
	Prompt      string            `json:"prompt"`
	Task        string            `json:"task,omitempty"`
	Providers   []string          `json:"providers,omitempty"`
	Options     map[string]interface{} `json:"options,omitempty"`
}

// AIResponse represents AI operation results
type AIResponse struct {
	Results   map[string]interface{} `json:"results"`
	Provider  string                `json:"provider,omitempty"`
	Providers []string              `json:"providers,omitempty"`
	Success   bool                  `json:"success"`
	Error     string                `json:"error,omitempty"`
	Timestamp string                `json:"timestamp"`
}

// handleCompareProviders compares responses from multiple AI providers
func (s *MultiProviderService) handleCompareProviders(w http.ResponseWriter, r *http.Request) {
	var req AIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	results := make(map[string]interface{})
	providers := []string{}

	// Try each provider
	if s.primaryClient != nil {
		if resp, err := s.primaryClient.GenerateResponse(ctx, req.Prompt, &core.AIOptions{
			Temperature: 0.3,
			MaxTokens:   500,
		}); err == nil {
			results["primary"] = map[string]interface{}{
				"content": resp.Content,
				"model":   resp.Model,
				"usage":   resp.Usage,
			}
			providers = append(providers, "primary")
		}
	}

	if s.fallbackClient != nil {
		if resp, err := s.fallbackClient.GenerateResponse(ctx, req.Prompt, &core.AIOptions{
			Temperature: 0.3,
			MaxTokens:   500,
		}); err == nil {
			results["fallback"] = map[string]interface{}{
				"content": resp.Content,
				"model":   resp.Model,
				"usage":   resp.Usage,
			}
			providers = append(providers, "fallback")
		}
	}

	if s.secondaryClient != nil {
		if resp, err := s.secondaryClient.GenerateResponse(ctx, req.Prompt, &core.AIOptions{
			Temperature: 0.3,
			MaxTokens:   500,
		}); err == nil {
			results["secondary"] = map[string]interface{}{
				"content": resp.Content,
				"model":   resp.Model,
				"usage":   resp.Usage,
			}
			providers = append(providers, "secondary")
		}
	}

	response := AIResponse{
		Results:   results,
		Providers: providers,
		Success:   len(results) > 0,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if len(results) == 0 {
		response.Error = "No AI providers available"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleProviderHealth checks AI provider health
func (s *MultiProviderService) handleProviderHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	healthStatus := make(map[string]interface{})

	// Test each provider with a simple prompt
	testPrompt := "Say 'OK' if you can respond"

	if s.primaryClient != nil {
		start := time.Now()
		_, err := s.primaryClient.GenerateResponse(ctx, testPrompt, &core.AIOptions{
			MaxTokens: 10,
		})
		healthStatus["primary"] = map[string]interface{}{
			"available":    err == nil,
			"response_time": time.Since(start).String(),
			"error":        getErrorString(err),
		}
	}

	if s.fallbackClient != nil {
		start := time.Now()
		_, err := s.fallbackClient.GenerateResponse(ctx, testPrompt, &core.AIOptions{
			MaxTokens: 10,
		})
		healthStatus["fallback"] = map[string]interface{}{
			"available":    err == nil,
			"response_time": time.Since(start).String(),
			"error":        getErrorString(err),
		}
	}

	if s.secondaryClient != nil {
		start := time.Now()
		_, err := s.secondaryClient.GenerateResponse(ctx, testPrompt, &core.AIOptions{
			MaxTokens: 10,
		})
		healthStatus["secondary"] = map[string]interface{}{
			"available":    err == nil,
			"response_time": time.Since(start).String(),
			"error":        getErrorString(err),
		}
	}

	response := AIResponse{
		Results:   healthStatus,
		Success:   true,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleIntelligentRouting routes to the best available provider
func (s *MultiProviderService) handleIntelligentRouting(w http.ResponseWriter, r *http.Request) {
	var req AIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	
	// Try providers in order of preference
	clients := []struct {
		name   string
		client core.AIClient
	}{
		{"primary", s.primaryClient},
		{"fallback", s.fallbackClient},
		{"secondary", s.secondaryClient},
	}

	for _, provider := range clients {
		if provider.client != nil {
			resp, err := provider.client.GenerateResponse(ctx, req.Prompt, &core.AIOptions{
				Temperature: 0.3,
				MaxTokens:   1000,
			})
			if err == nil {
				response := AIResponse{
					Results: map[string]interface{}{
						"content": resp.Content,
						"model":   resp.Model,
						"usage":   resp.Usage,
					},
					Provider:  provider.name,
					Success:   true,
					Timestamp: time.Now().Format(time.RFC3339),
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
			log.Printf("Provider %s failed: %v", provider.name, err)
		}
	}

	// All providers failed
	response := AIResponse{
		Success:   false,
		Error:     "All AI providers unavailable",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	w.WriteHeader(http.StatusServiceUnavailable)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleProviderOrchestration coordinates multiple providers for complex tasks
func (s *MultiProviderService) handleProviderOrchestration(w http.ResponseWriter, r *http.Request) {
	var req AIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	
	// Example orchestration: Use different providers for different aspects
	results := make(map[string]interface{})
	
	// Primary for analysis
	if s.primaryClient != nil {
		analysisPrompt := fmt.Sprintf("Analyze this task: %s", req.Prompt)
		if resp, err := s.primaryClient.GenerateResponse(ctx, analysisPrompt, &core.AIOptions{
			Temperature: 0.1, // Lower temperature for analysis
			MaxTokens:   500,
		}); err == nil {
			results["analysis"] = resp.Content
		}
	}

	// Fallback for creative response
	if s.fallbackClient != nil {
		creativePrompt := fmt.Sprintf("Provide a creative perspective on: %s", req.Prompt)
		if resp, err := s.fallbackClient.GenerateResponse(ctx, creativePrompt, &core.AIOptions{
			Temperature: 0.8, // Higher temperature for creativity
			MaxTokens:   500,
		}); err == nil {
			results["creative"] = resp.Content
		}
	}

	// Secondary for summary
	if s.secondaryClient != nil {
		summaryPrompt := fmt.Sprintf("Summarize key points about: %s", req.Prompt)
		if resp, err := s.secondaryClient.GenerateResponse(ctx, summaryPrompt, &core.AIOptions{
			Temperature: 0.3,
			MaxTokens:   300,
		}); err == nil {
			results["summary"] = resp.Content
		}
	}

	response := AIResponse{
		Results:   results,
		Providers: []string{"primary", "fallback", "secondary"},
		Success:   len(results) > 0,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if len(results) == 0 {
		response.Error = "No providers available for orchestration"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	// Create the multi-provider service
	service, err := NewMultiProviderService()
	if err != nil {
		log.Fatalf("Failed to create multi-provider service: %v", err)
	}

	// Initialize and start based on deployment mode
	ctx := context.Background()
	deploymentMode := service.deploymentMode

	switch deploymentMode {
	case "tool":
		log.Println("🔧 Starting in Tool mode...")
		if err := service.BaseTool.Initialize(ctx); err != nil {
			log.Fatalf("Tool initialization failed: %v", err)
		}
		
		toolPort := getIntEnvOrDefault("TOOL_PORT", 8085)
		log.Printf("🚀 Multi-Provider AI Tool starting on port %d", toolPort)
		log.Fatal(service.BaseTool.Start(ctx, toolPort))

	case "agent":
		log.Println("🤖 Starting in Agent mode...")
		if err := service.BaseAgent.Initialize(ctx); err != nil {
			log.Fatalf("Agent initialization failed: %v", err)
		}
		
		agentPort := getIntEnvOrDefault("AGENT_PORT", 8086)
		log.Printf("🚀 Multi-Provider AI Agent starting on port %d", agentPort)
		log.Fatal(service.BaseAgent.Start(ctx, agentPort))

	case "both":
		log.Println("🔧🤖 Starting in Both modes...")
		
		// Start tool in a goroutine
		go func() {
			if err := service.BaseTool.Initialize(ctx); err != nil {
				log.Fatalf("Tool initialization failed: %v", err)
			}
			toolPort := getIntEnvOrDefault("TOOL_PORT", 8085)
			log.Printf("🚀 Multi-Provider AI Tool starting on port %d", toolPort)
			log.Fatal(service.BaseTool.Start(ctx, toolPort))
		}()

		// Start agent in main goroutine
		if err := service.BaseAgent.Initialize(ctx); err != nil {
			log.Fatalf("Agent initialization failed: %v", err)
		}
		agentPort := getIntEnvOrDefault("AGENT_PORT", 8086)
		log.Printf("🚀 Multi-Provider AI Agent starting on port %d", agentPort)
		log.Fatal(service.BaseAgent.Start(ctx, agentPort))

	default:
		log.Fatalf("Invalid deployment mode: %s", deploymentMode)
	}
}

// Helper functions
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnvOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getErrorString(err error) interface{} {
	if err != nil {
		return err.Error()
	}
	return nil
}