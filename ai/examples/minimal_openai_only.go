// Package main - minimal example with only OpenAI provider
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"
	
	// Import ONLY OpenAI provider
	_ "github.com/itsneelabh/gomind/ai/providers/openai"
)

func main() {
	client, err := ai.NewClient(ai.WithProvider("openai"))
	if err != nil {
		log.Fatal(err)
	}
	
	response, err := client.GenerateResponse(
		context.Background(),
		"Hello",
		&core.AIOptions{MaxTokens: 10},
	)
	
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Response: %s\n", response.Content)
	}
}