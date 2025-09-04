package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/itsneelabh/gomind/telemetry"
)

// Simulated HTTP request
type Request struct {
	ID       string
	UserID   string
	TenantID string
	Path     string
	Method   string
}

// Simulated Response
type Response struct {
	StatusCode int
	Body       string
}

func main() {
	// Initialize telemetry
	err := telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileDevelopment))
	if err != nil {
		log.Fatalf("Failed to initialize telemetry: %v", err)
	}
	defer telemetry.Shutdown(context.Background())

	// Simulate handling multiple requests
	requests := []Request{
		{ID: "req-001", UserID: "user-123", TenantID: "tenant-abc", Path: "/api/users", Method: "GET"},
		{ID: "req-002", UserID: "user-456", TenantID: "tenant-xyz", Path: "/api/orders", Method: "POST"},
		{ID: "req-003", UserID: "user-789", TenantID: "tenant-abc", Path: "/api/products", Method: "GET"},
	}

	fmt.Println("=== Context Propagation Example ===\n")

	for _, req := range requests {
		fmt.Printf("Processing request %s\n", req.ID)
		handleRequest(req)
		fmt.Println()
	}

	// Show telemetry health
	health := telemetry.GetHealth()
	fmt.Printf("=== Telemetry Summary ===\n")
	fmt.Printf("Metrics Emitted: %d\n", health.MetricsEmitted)
	fmt.Printf("Errors: %d\n", health.Errors)
}

// handleRequest demonstrates context propagation through a request lifecycle
func handleRequest(req Request) Response {
	// Create context with request metadata
	// This is done ONCE at the edge of your system
	ctx := telemetry.WithBaggage(context.Background(),
		"request_id", req.ID,
		"user_id", req.UserID,
		"tenant_id", req.TenantID,
		"path", req.Path,
		"method", req.Method,
	)

	// Track request start (all labels automatically included)
	telemetry.EmitWithContext(ctx, "http.request.started", 1)
	defer func(start time.Time) {
		// Track request completion with duration
		duration := time.Since(start).Milliseconds()
		telemetry.EmitWithContext(ctx, "http.request.duration_ms", float64(duration))
		telemetry.EmitWithContext(ctx, "http.request.completed", 1)
	}(time.Now())

	// Simulate authentication
	if err := authenticateUser(ctx, req.UserID); err != nil {
		telemetry.EmitWithContext(ctx, "auth.failed", 1)
		return Response{StatusCode: 401, Body: "Unauthorized"}
	}

	// Simulate database operations
	data, err := fetchFromDatabase(ctx, req.Path)
	if err != nil {
		telemetry.EmitWithContext(ctx, "database.error", 1)
		return Response{StatusCode: 500, Body: "Internal Server Error"}
	}

	// Simulate cache operations
	cacheData(ctx, req.Path, data)

	// Simulate business logic
	result := processBusinessLogic(ctx, data)

	return Response{StatusCode: 200, Body: result}
}

// authenticateUser demonstrates telemetry deep in the call stack
func authenticateUser(ctx context.Context, userID string) error {
	// Metrics automatically include request_id, user_id, tenant_id from context!
	telemetry.EmitWithContext(ctx, "auth.attempt", 1)

	// Simulate auth check
	time.Sleep(5 * time.Millisecond)

	if rand.Float32() > 0.1 { // 90% success rate
		telemetry.EmitWithContext(ctx, "auth.success", 1)
		return nil
	}

	telemetry.EmitWithContext(ctx, "auth.failure", 1)
	return fmt.Errorf("authentication failed")
}

// fetchFromDatabase shows how context flows through layers
func fetchFromDatabase(ctx context.Context, key string) (string, error) {
	// Add layer-specific context
	ctx = telemetry.WithBaggage(ctx, "db.operation", "select", "db.table", "products")

	telemetry.EmitWithContext(ctx, "db.query.started", 1)
	defer func(start time.Time) {
		duration := time.Since(start).Milliseconds()
		telemetry.EmitWithContext(ctx, "db.query.duration_ms", float64(duration))
	}(time.Now())

	// Simulate database query
	time.Sleep(10 * time.Millisecond)

	// Check cache first (nested function call)
	if cached := checkCache(ctx, key); cached != "" {
		telemetry.EmitWithContext(ctx, "db.cache.hit", 1)
		return cached, nil
	}

	telemetry.EmitWithContext(ctx, "db.cache.miss", 1)

	// Simulate fetching from database
	if rand.Float32() > 0.05 { // 95% success rate
		telemetry.EmitWithContext(ctx, "db.query.success", 1)
		return fmt.Sprintf("data_for_%s", key), nil
	}

	telemetry.EmitWithContext(ctx, "db.query.failure", 1)
	return "", fmt.Errorf("database error")
}

// checkCache demonstrates deeply nested context propagation
func checkCache(ctx context.Context, key string) string {
	// Even this deeply nested function has access to all context labels
	telemetry.EmitWithContext(ctx, "cache.lookup", 1)

	// Simulate cache lookup
	if rand.Float32() > 0.7 { // 30% hit rate
		return fmt.Sprintf("cached_%s", key)
	}
	return ""
}

// cacheData shows adding data to cache
func cacheData(ctx context.Context, key string, value string) {
	// Add cache-specific context
	ctx = telemetry.WithBaggage(ctx, "cache.operation", "set")

	telemetry.EmitWithContext(ctx, "cache.write", 1)
	telemetry.EmitWithContext(ctx, "cache.bytes", float64(len(value)))
}

// processBusinessLogic demonstrates business logic telemetry
func processBusinessLogic(ctx context.Context, data string) string {
	// Add business logic context
	ctx = telemetry.WithBaggage(ctx, "processor", "v2", "algorithm", "optimized")

	telemetry.EmitWithContext(ctx, "business_logic.started", 1)
	defer func(start time.Time) {
		duration := time.Since(start).Milliseconds()
		telemetry.EmitWithContext(ctx, "business_logic.duration_ms", float64(duration))
		telemetry.EmitWithContext(ctx, "business_logic.completed", 1)
	}(time.Now())

	// Simulate processing
	time.Sleep(5 * time.Millisecond)

	// Track specific business metrics
	telemetry.EmitWithContext(ctx, "items.processed", float64(len(data)))

	return fmt.Sprintf("processed_%s", data)
}

// Example output shows how all metrics include the context labels:
// Every metric emitted includes: request_id, user_id, tenant_id, path, method
// Plus any additional labels added at each layer
//
// This means you can:
// 1. Filter all metrics for a specific request: request_id="req-001"
// 2. Analyze per-user behavior: user_id="user-123"
// 3. Multi-tenant metrics: tenant_id="tenant-abc"
// 4. Debug specific request flows through the entire system
// 5. Create dashboards that drill down from request -> database -> cache