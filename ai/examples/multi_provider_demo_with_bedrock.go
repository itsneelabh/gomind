//go:build bedrock
// +build bedrock

// Package main demonstrates the multi-provider AI system INCLUDING AWS Bedrock
// Build with: go build -tags bedrock multi_provider_demo_with_bedrock.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"
	
	// Import all providers INCLUDING Bedrock
	_ "github.com/itsneelabh/gomind/ai/providers/anthropic"
	_ "github.com/itsneelabh/gomind/ai/providers/bedrock"  // Requires -tags bedrock
	_ "github.com/itsneelabh/gomind/ai/providers/gemini"
	_ "github.com/itsneelabh/gomind/ai/providers/openai"
)

func main() {
	// List all registered providers
	fmt.Println("=== Registered Providers (with Bedrock) ===")
	providers := ai.ListProviders()
	for _, p := range providers {
		fmt.Printf("  - %s\n", p)
	}
	fmt.Println()
	
	// Test AWS Bedrock if available
	fmt.Println("=== AWS Bedrock Provider Test ===")
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" || os.Getenv("AWS_PROFILE") != "" {
		client, err := ai.NewClient(
			ai.WithProvider("bedrock"),
			ai.WithModel("anthropic.claude-3-sonnet-20240229-v1:0"),
		)
		if err != nil {
			log.Printf("Failed to create Bedrock client: %v", err)
		} else {
			fmt.Println("AWS Bedrock provider is available and configured!")
			
			response, err := client.GenerateResponse(
				context.Background(),
				"Say hello in 5 words",
				&core.AIOptions{MaxTokens: 50},
			)
			
			if err != nil {
				fmt.Printf("  Error: %v\n", err)
			} else {
				fmt.Printf("  Response: %s\n", response.Content)
			}
		}
	} else {
		fmt.Println("  AWS credentials not configured")
	}
}