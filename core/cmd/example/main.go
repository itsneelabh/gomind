package main

import (
	"context"
	"log"

	"github.com/itsneelabh/gomind/core"
)

func main() {
	// Create a simple tool with core module only
	tool := core.NewBaseAgent("database-query-tool")

	// Optional: Set up Redis discovery
	discovery, err := core.NewRedisDiscovery("redis://localhost:6379")
	if err != nil {
		log.Printf("Discovery not available: %v", err)
	} else {
		tool.Discovery = discovery
	}

	// Register a capability
	tool.RegisterCapability(core.Capability{
		Name:        "query_sales",
		Description: "Query sales data by product",
		InputTypes:  []string{"product_id"},
		OutputTypes: []string{"sales_data"},
	})

	// Initialize and start
	ctx := context.Background()
	if err := tool.Initialize(ctx); err != nil {
		log.Fatal(err)
	}

	log.Println("Starting tool on port 8080...")
	if err := tool.Start(8080); err != nil {
		log.Fatal(err)
	}
}
