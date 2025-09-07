// Package main - example with all providers EXCEPT Bedrock
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"
	
	// Import all providers EXCEPT Bedrock
	_ "github.com/itsneelabh/gomind/ai/providers/anthropic"
	_ "github.com/itsneelabh/gomind/ai/providers/gemini"
	_ "github.com/itsneelabh/gomind/ai/providers/openai"
)

func main() {
	fmt.Printf("Providers: %v\n", ai.ListProviders())
	
	client, err := ai.NewClient()
	if err != nil {
		log.Printf("Error: %v", err)
	}
	
	if client != nil {
		_, _ = client.GenerateResponse(
			context.Background(),
			"test",
			&core.AIOptions{MaxTokens: 10},
		)
	}
}