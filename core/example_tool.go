//go:build example
// +build example

package core

import (
	"context"
	"log"
)

// ExampleDatabaseQueryTool shows how to build a simple tool with core module only
// This creates a deterministic service that AI agents can discover and use
// Binary size: ~5MB, Memory: ~10MB
func ExampleDatabaseQueryTool() {
	// Create a tool (no AI, just core functionality)
	tool := NewBaseAgent("database-query-tool")

	// Set up Redis discovery if available
	discovery, err := NewRedisDiscovery("redis://localhost:6379", "tools")
	if err != nil {
		log.Printf("Discovery not available, running standalone: %v", err)
		// Tool still works without discovery
	} else {
		tool.Discovery = discovery
	}

	// Register capabilities that AI agents can discover
	tool.RegisterCapability(Capability{
		Name:        "query_sales",
		Description: "Query sales data by product",
		Endpoint:    "/api/capabilities/query_sales",
		InputTypes:  []string{"product_id", "date_range"},
		OutputTypes: []string{"sales_data"},
	})

	tool.RegisterCapability(Capability{
		Name:        "aggregate_metrics",
		Description: "Aggregate business metrics",
		Endpoint:    "/api/capabilities/aggregate_metrics",
		InputTypes:  []string{"metric_type", "period"},
		OutputTypes: []string{"aggregated_data"},
	})

	// Start the tool
	framework := NewFramework(tool, 8080)
	if err := framework.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

// The tool registers in Redis as:
// {
//   "type": "tool",
//   "name": "database-query-tool",
//   "capabilities": ["query_sales", "aggregate_metrics"],
//   "endpoint": "http://localhost:8080"
// }
//
// AI agents can discover and use this tool without the tool itself having any AI capabilities
