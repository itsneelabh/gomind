// Package main - minimal example with NO providers imported
package main

import (
	"fmt"

	"github.com/itsneelabh/gomind/ai"
)

func main() {
	// Just list providers (should be empty)
	providers := ai.ListProviders()
	fmt.Printf("Registered providers: %v\n", providers)
	
	// Try to create client (should fail)
	_, err := ai.NewClient()
	if err != nil {
		fmt.Printf("Expected error: %v\n", err)
	}
}