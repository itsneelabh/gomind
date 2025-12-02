// Package main demonstrates the built-in AI-powered tools from the GoMind framework
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"

	// Import AI providers for auto-detection - these register themselves via init()
	_ "github.com/itsneelabh/gomind/ai/providers/openai"    // OpenAI and compatible services (Groq, DeepSeek, etc.)
	_ "github.com/itsneelabh/gomind/ai/providers/anthropic" // Anthropic Claude
	_ "github.com/itsneelabh/gomind/ai/providers/gemini"    // Google Gemini
)

// AIToolsShowcase demonstrates the built-in AI-powered tools from the GoMind AI module
// This example shows how to deploy and use the 4 ready-to-use AI tools:
//
// 1. Translation Tool - Professional language translation
// 2. Summarization Tool - Intelligent text summarization
// 3. Sentiment Analysis Tool - Emotion and tone analysis  
// 4. Code Review Tool - AI-powered code quality analysis
//
// Each tool is a fully functional component that can be deployed independently
// or together as shown in this composite deployment example.
type AIToolsShowcase struct {
	translationTool   *ai.AITool
	summarizationTool *ai.AITool
	sentimentTool     *ai.AITool
	codeReviewTool    *ai.AITool
	
	// For composite deployment
	compositeTool *core.BaseTool
	frameworks    []*core.Framework
	mu            sync.Mutex
}

// NewAIToolsShowcase creates a showcase that hosts all 4 built-in AI tools
func NewAIToolsShowcase() (*AIToolsShowcase, error) {
	// Step 1: Get AI API key - required for all AI tools
	// The framework supports multiple providers with auto-detection
	apiKey := getAIAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf(`AI Tools require an API key. Please set one of:
  export OPENAI_API_KEY="sk-your-openai-key"           # OpenAI (recommended)
  export GROQ_API_KEY="gsk-your-groq-key"              # Ultra-fast (free tier available)
  export ANTHROPIC_API_KEY="sk-ant-your-claude-key"    # Claude models
  export DEEPSEEK_API_KEY="sk-your-deepseek-key"       # Advanced reasoning
  export GEMINI_API_KEY="your-google-gemini-key"       # Google AI`)
	}

	log.Println("🤖 Creating AI Tools with provider:", getProviderName())

	// Step 2: Create all 4 built-in AI tools
	// Each tool is a complete, deployable component with its own AI-powered capabilities
	
	// Translation Tool - Handles professional language translation
	translationTool, err := ai.NewTranslationTool(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create translation tool: %w", err)
	}
	log.Println("Translation Tool created successfully")

	// Summarization Tool - Intelligent text summarization and key point extraction
	summarizationTool, err := ai.NewSummarizationTool(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarization tool: %w", err)
	}
	log.Println("Summarization Tool created successfully")

	// Sentiment Analysis Tool - Emotion and tone analysis with confidence scoring
	sentimentTool, err := ai.NewSentimentAnalysisTool(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create sentiment analysis tool: %w", err)
	}
	log.Println("Sentiment Analysis Tool created successfully")

	// Code Review Tool - AI-powered code quality, security, and best practice analysis
	codeReviewTool, err := ai.NewCodeReviewTool(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create code review tool: %w", err)
	}
	log.Println("Code Review Tool created successfully")

	// Step 3: Create a composite tool that aggregates all AI tool capabilities
	// This allows deploying all tools as a single service
	compositeTool := core.NewTool("ai-tools-composite")
	
	// Copy capabilities from each AI tool to the composite tool
	// This preserves the original handlers and endpoints
	for _, cap := range translationTool.GetCapabilities() {
		compositeTool.RegisterCapability(cap)
	}
	for _, cap := range summarizationTool.GetCapabilities() {
		compositeTool.RegisterCapability(cap)
	}
	for _, cap := range sentimentTool.GetCapabilities() {
		compositeTool.RegisterCapability(cap)
	}
	for _, cap := range codeReviewTool.GetCapabilities() {
		compositeTool.RegisterCapability(cap)
	}
	
	// Add an additional showcase capability for interactive demos
	compositeTool.RegisterCapability(core.Capability{
		Name:        "showcase",
		Description: "Interactive showcase of all AI tools with examples",
		Endpoint:    "/showcase",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     handleShowcase,
	})

	return &AIToolsShowcase{
		translationTool:   translationTool,
		summarizationTool: summarizationTool,
		sentimentTool:     sentimentTool,
		codeReviewTool:    codeReviewTool,
		compositeTool:     compositeTool,
		frameworks:        make([]*core.Framework, 0),
	}, nil
}

// DeployAsComposite deploys all tools as a single composite service
// This is the recommended approach for development and when you want all AI capabilities in one place
func (a *AIToolsShowcase) DeployAsComposite(port int) error {
	// Deploy the composite tool with full framework support
	framework, err := core.NewFramework(a.compositeTool,
		// Core configuration
		core.WithName("ai-tools-composite"),
		core.WithPort(port),
		core.WithNamespace("examples"),

		// Service discovery - other agents can discover these AI capabilities
		core.WithRedisURL(getEnvOrDefault("REDIS_URL", "redis://localhost:6379")),
		core.WithDiscovery(true, "redis"),

		// Web configuration
		core.WithCORS([]string{"*"}, true),
		core.WithDevelopmentMode(true),
	)
	if err != nil {
		return fmt.Errorf("failed to create composite framework: %w", err)
	}

	log.Printf("AI Tools Composite Service starting on port %d...", port)
	log.Println("🔧 Available AI capabilities (all tools in one service):")
	log.Println("   - POST /ai/translate                  (Translation)")
	log.Println("   - POST /ai/summarize                  (Summarization)")
	log.Println("   - POST /ai/analyze_sentiment          (Sentiment Analysis)")
	log.Println("   - POST /ai/review_code                (Code Review)")
	log.Println("   - POST /showcase                      (Interactive Showcase)")
	log.Println("   - GET  /api/capabilities              (List all capabilities)")
	log.Println("   - GET  /health                        (Health check)")
	log.Println()
	log.Println("📖 Usage example:")
	log.Printf(`   curl -X POST http://localhost:%d/ai/translate \`, port)
	log.Println(`     -H "Content-Type: text/plain" \`)
	log.Println(`     -d 'Hello world! Translate to Spanish.'`)
	log.Println()

	ctx := context.Background()
	return framework.Run(ctx)
}

// DeployIndividually deploys each AI tool as a separate service
// This approach is better for production where you want to scale tools independently
func (a *AIToolsShowcase) DeployIndividually(basePort int) error {
	// Deploy each tool on a different port
	tools := []struct {
		tool *ai.AITool
		name string
		port int
	}{
		{a.translationTool, "translation-tool", basePort},
		{a.summarizationTool, "summarization-tool", basePort + 1},
		{a.sentimentTool, "sentiment-tool", basePort + 2},
		{a.codeReviewTool, "code-review-tool", basePort + 3},
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(tools))

	for _, t := range tools {
		wg.Add(1)
		go func(tool *ai.AITool, name string, port int) {
			defer wg.Done()

			// Each tool gets its own framework deployment
			framework, err := core.NewFramework(tool,
				core.WithName(name),
				core.WithPort(port),
				core.WithNamespace("examples"),
				core.WithRedisURL(getEnvOrDefault("REDIS_URL", "redis://localhost:6379")),
				core.WithDiscovery(true, "redis"),
				core.WithCORS([]string{"*"}, true),
				core.WithDevelopmentMode(true),
			)
			if err != nil {
				errCh <- fmt.Errorf("failed to create framework for %s: %w", name, err)
				return
			}

			a.mu.Lock()
			a.frameworks = append(a.frameworks, framework)
			a.mu.Unlock()

			log.Printf("%s starting on port %d...", name, port)
			
			ctx := context.Background()
			if err := framework.Run(ctx); err != nil {
				errCh <- fmt.Errorf("%s failed: %w", name, err)
			}
		}(t.tool, t.name, t.port)
	}

	// Check for immediate errors
	select {
	case err := <-errCh:
		return err
	default:
		// All services started successfully
		log.Println("All AI tools deployed individually:")
		log.Printf("   Translation Tool:    http://localhost:%d", basePort)
		log.Printf("   Summarization Tool:  http://localhost:%d", basePort+1)
		log.Printf("   Sentiment Tool:      http://localhost:%d", basePort+2)
		log.Printf("   Code Review Tool:    http://localhost:%d", basePort+3)
	}

	wg.Wait()
	return nil
}

// handleShowcase provides an interactive demonstration of all AI tools
func handleShowcase(rw http.ResponseWriter, r *http.Request) {
	var request struct {
		Tool string                 `json:"tool,omitempty"`
		Demo string                 `json:"demo,omitempty"`
		Data map[string]interface{} `json:"data,omitempty"`
	}

	// Allow both GET (for overview) and POST (for specific demos)
	if r.Method == http.MethodGet {
		// Return overview of available tools and demos
		response := map[string]interface{}{
			"title":       "GoMind AI Tools Showcase",
			"description": "Interactive demonstration of built-in AI-powered tools",
			"tools": []map[string]interface{}{
				{
					"name":        "translation",
					"endpoint":    "/ai/translate",
					"description": "Professional language translation",
					"example":     `curl -X POST /ai/translate -d "Translate to French: Hello world"`,
				},
				{
					"name":        "summarization",
					"endpoint":    "/ai/summarize",
					"description": "Intelligent text summarization",
					"example":     `curl -X POST /ai/summarize -d "Long text to summarize..."`,
				},
				{
					"name":        "sentiment",
					"endpoint":    "/ai/analyze_sentiment",
					"description": "Emotion and tone analysis",
					"example":     `curl -X POST /ai/analyze_sentiment -d "I love this framework!"`,
				},
				{
					"name":        "code_review",
					"endpoint":    "/ai/review_code",
					"description": "AI-powered code review",
					"example":     `curl -X POST /ai/review_code -d "func main() { fmt.Println('hello') }"`,
				},
			},
			"usage": map[string]interface{}{
				"overview":      "GET /showcase",
				"run_demo":      "POST /showcase with {\"tool\": \"translation\", \"demo\": \"example\"}",
				"direct_access": "POST /ai/{tool_name} with text or JSON data",
			},
		}
		respondJSON(rw, response)
		return
	}

	// Handle POST for specific tool demonstrations
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		// If not JSON, treat as plain text for default demo
		request.Tool = "translation"
		request.Demo = "default"
	}

	// Provide sample demonstrations for each tool
	demos := map[string]map[string]interface{}{
		"translation": {
			"description": "Translates text between languages",
			"sample_input": "Hello, how are you today?",
			"sample_output": "Hola, ¿cómo estás hoy?",
			"endpoint": "/ai/translate",
		},
		"summarization": {
			"description": "Summarizes long text into key points",
			"sample_input": "Long article about AI and machine learning...",
			"sample_output": "Key points: 1. AI revolutionizes technology 2. ML enables pattern recognition 3. Applications span industries",
			"endpoint": "/ai/summarize",
		},
		"sentiment": {
			"description": "Analyzes emotional tone of text",
			"sample_input": "I absolutely love this new framework! It makes development so much easier.",
			"sample_output": "POSITIVE (confidence: 95%) - Strong positive sentiment with enthusiasm",
			"endpoint": "/ai/analyze_sentiment",
		},
		"code_review": {
			"description": "Reviews code for quality and best practices",
			"sample_input": "func processData(data []string) { for i := 0; i < len(data); i++ { fmt.Println(data[i]) } }",
			"sample_output": "Suggestions: 1. Use range loop for clarity 2. Add error handling 3. Consider using structured logging",
			"endpoint": "/ai/review_code",
		},
	}

	// If specific tool requested, return its demo
	if request.Tool != "" {
		if demo, exists := demos[request.Tool]; exists {
			respondJSON(rw, demo)
			return
		}
	}

	// Return all demos
	respondJSON(rw, demos)
}

// Utility functions

// getAIAPIKey gets the first available AI API key from environment
func getAIAPIKey() string {
	providers := []string{
		"OPENAI_API_KEY",
		"GROQ_API_KEY", 
		"ANTHROPIC_API_KEY",
		"DEEPSEEK_API_KEY",
		"GEMINI_API_KEY",
		"GOOGLE_API_KEY", // Alternative for Gemini
	}

	for _, provider := range providers {
		if key := os.Getenv(provider); key != "" {
			return key
		}
	}
	return ""
}

// getProviderName returns the name of the detected provider
func getProviderName() string {
	if os.Getenv("OPENAI_API_KEY") != "" {
		return "OpenAI"
	}
	if os.Getenv("GROQ_API_KEY") != "" {
		return "Groq (Ultra-fast)"
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return "Anthropic Claude"
	}
	if os.Getenv("DEEPSEEK_API_KEY") != "" {
		return "DeepSeek"
	}
	if os.Getenv("GEMINI_API_KEY") != "" || os.Getenv("GOOGLE_API_KEY") != "" {
		return "Google Gemini"
	}
	return "Unknown"
}

// respondJSON sends a JSON response
func respondJSON(rw http.ResponseWriter, data interface{}) {
	rw.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(rw).Encode(data); err != nil {
		http.Error(rw, "JSON encoding failed", http.StatusInternalServerError)
	}
}

// getEnvOrDefault gets environment variable with fallback
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	// Create AI tools showcase
	showcase, err := NewAIToolsShowcase()
	if err != nil {
		log.Fatalf("Failed to create AI tools showcase: %v", err)
	}

	// Get deployment mode from environment
	deploymentMode := getEnvOrDefault("DEPLOYMENT_MODE", "composite")
	
	switch deploymentMode {
	case "composite":
		// Deploy all tools as a single service (default)
		port := getPortFromEnv("PORT", 8084)
		log.Printf("🎯 Deploying all AI tools as composite service on port %d", port)
		if err := showcase.DeployAsComposite(port); err != nil {
			log.Fatalf("Composite deployment failed: %v", err)
		}
		
	case "individual":
		// Deploy each tool as a separate service
		basePort := getPortFromEnv("BASE_PORT", 8084)
		log.Printf("🎯 Deploying AI tools individually starting from port %d", basePort)
		if err := showcase.DeployIndividually(basePort); err != nil {
			log.Fatalf("Individual deployment failed: %v", err)
		}
		
	default:
		log.Fatalf("Invalid DEPLOYMENT_MODE: %s. Use 'composite' or 'individual'", deploymentMode)
	}
}

func getPortFromEnv(envVar string, defaultPort int) int {
	if portStr := os.Getenv(envVar); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			return port
		}
	}
	return defaultPort
}