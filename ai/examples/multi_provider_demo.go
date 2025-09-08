//go:build ignore
// +build ignore

// Package main demonstrates the multi-provider AI system
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"
	
	// Import all providers
	_ "github.com/itsneelabh/gomind/ai/providers/anthropic"
	_ "github.com/itsneelabh/gomind/ai/providers/gemini"
	_ "github.com/itsneelabh/gomind/ai/providers/openai"
	
	// Import AWS Bedrock provider (only compiled with -tags bedrock)
	// Uncomment if you need AWS Bedrock support:
	// _ "github.com/itsneelabh/gomind/ai/providers/bedrock"
)

func main() {
	// 1. List all registered providers
	fmt.Println("=== Registered Providers ===")
	providers := ai.ListProviders()
	for _, p := range providers {
		fmt.Printf("  - %s\n", p)
	}
	fmt.Println()
	
	// 2. Show detailed provider information
	fmt.Println("=== Provider Information ===")
	info := ai.GetProviderInfo()
	for _, p := range info {
		status := "❌ unavailable"
		if p.Available {
			status = "✅ available"
		}
		fmt.Printf("  %s: %s\n    Status: %s (priority: %d)\n",
			p.Name, p.Description, status, p.Priority)
	}
	fmt.Println()
	
	// 3. Auto-detect best available provider
	fmt.Println("=== Auto-Detection ===")
	client, err := ai.NewClient() // Auto-detects from environment
	if err != nil {
		fmt.Printf("Auto-detection failed: %v\n", err)
		fmt.Println("Please set one of: OPENAI_API_KEY, ANTHROPIC_API_KEY, GEMINI_API_KEY")
		fmt.Println("Or ensure Ollama is running locally")
	} else {
		fmt.Println("Successfully created client with auto-detected provider")
		testClient(client, "auto-detected")
	}
	fmt.Println()
	
	// 4. Test different provider types
	fmt.Println("=== Testing Different Provider Types ===")
	
	// A. Universal OpenAI Provider (handles 20+ services)
	fmt.Println("\n--- Universal OpenAI Provider ---")
	
	// OpenAI (original)
	if os.Getenv("OPENAI_API_KEY") != "" {
		client, err := ai.NewClient(ai.WithProvider("openai"))
		if err != nil {
			log.Printf("Failed to create OpenAI client: %v", err)
		} else {
			testClient(client, "OpenAI")
		}
	}
	
	// Groq (OpenAI-compatible)
	if os.Getenv("GROQ_API_KEY") != "" {
		client, err := ai.NewClient(
			ai.WithProvider("openai"),
			ai.WithBaseURL("https://api.groq.com/openai/v1"),
			ai.WithAPIKey(os.Getenv("GROQ_API_KEY")),
		)
		if err != nil {
			log.Printf("Failed to create Groq client: %v", err)
		} else {
			testClient(client, "Groq (via universal OpenAI provider)")
		}
	}
	
	// Ollama (local, OpenAI-compatible)
	client, err = ai.NewClient(
		ai.WithProvider("openai"),
		ai.WithBaseURL("http://localhost:11434/v1"),
	)
	if err != nil {
		log.Printf("Failed to create Ollama client: %v", err)
	} else {
		testClient(client, "Ollama (via universal OpenAI provider)")
	}
	
	// B. Native Anthropic Provider
	fmt.Println("\n--- Native Anthropic Provider ---")
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		client, err := ai.NewClient(ai.WithProvider("anthropic"))
		if err != nil {
			log.Printf("Failed to create Anthropic client: %v", err)
		} else {
			testClient(client, "Anthropic (native Messages API)")
		}
	} else {
		fmt.Println("  ANTHROPIC_API_KEY not set - skipping")
	}
	
	// C. Native Gemini Provider
	fmt.Println("\n--- Native Gemini Provider ---")
	if os.Getenv("GEMINI_API_KEY") != "" || os.Getenv("GOOGLE_API_KEY") != "" {
		client, err := ai.NewClient(ai.WithProvider("gemini"))
		if err != nil {
			log.Printf("Failed to create Gemini client: %v", err)
		} else {
			testClient(client, "Gemini (native GenerateContent API)")
		}
	} else {
		fmt.Println("  GEMINI_API_KEY/GOOGLE_API_KEY not set - skipping")
	}
	
	// D. AWS Bedrock Provider
	fmt.Println("\n--- AWS Bedrock Provider ---")
	// Check if AWS credentials are available
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" || os.Getenv("AWS_PROFILE") != "" || os.Getenv("AWS_EXECUTION_ENV") != "" {
		client, err := ai.NewClient(
			ai.WithProvider("bedrock"),
			ai.WithModel("anthropic.claude-3-sonnet-20240229-v1:0"), // Specify a model
		)
		if err != nil {
			log.Printf("Failed to create Bedrock client: %v", err)
		} else {
			testClient(client, "AWS Bedrock (unified access to multiple models)")
		}
	} else {
		fmt.Println("  AWS credentials not configured - skipping")
		fmt.Println("  Set AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY or configure AWS CLI")
	}
	
	// 5. Show configuration flexibility
	fmt.Println("\n=== Configuration Examples ===")
	fmt.Println("You can configure providers in multiple ways:")
	fmt.Println()
	fmt.Println("1. Auto-detection (checks environment variables):")
	fmt.Println("   client, _ := ai.NewClient()")
	fmt.Println()
	fmt.Println("2. Explicit provider selection:")
	fmt.Println("   client, _ := ai.NewClient(ai.WithProvider(\"anthropic\"))")
	fmt.Println()
	fmt.Println("3. Universal OpenAI provider for compatible services:")
	fmt.Println("   client, _ := ai.NewClient(")
	fmt.Println("       ai.WithProvider(\"openai\"),")
	fmt.Println("       ai.WithBaseURL(\"https://api.groq.com/openai/v1\"),")
	fmt.Println("       ai.WithAPIKey(os.Getenv(\"GROQ_API_KEY\")),")
	fmt.Println("   )")
	fmt.Println()
	fmt.Println("4. AWS Bedrock for enterprise multi-model access:")
	fmt.Println("   client, _ := ai.NewClient(")
	fmt.Println("       ai.WithProvider(\"bedrock\"),")
	fmt.Println("       ai.WithModel(\"anthropic.claude-3-sonnet-20240229-v1:0\"),")
	fmt.Println("       // Uses AWS credential chain (IAM role, env vars, ~/.aws/credentials)")
	fmt.Println("   )")
}

func testClient(client core.AIClient, provider string) {
	fmt.Printf("\nTesting %s provider:\n", provider)
	
	ctx := context.Background()
	response, err := client.GenerateResponse(
		ctx,
		"Say 'Hello from ' followed by your model name in 10 words or less",
		&core.AIOptions{
			Temperature: 0.5,
			MaxTokens:   50,
		},
	)
	
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		return
	}
	
	fmt.Printf("  Response: %s\n", response.Content)
	if response.Model != "" {
		fmt.Printf("  Model: %s\n", response.Model)
	}
}