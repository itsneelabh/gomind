package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/itsneelabh/gomind/resilience"
	"github.com/itsneelabh/gomind/telemetry"
)

func main() {
	// Initialize telemetry with a profile
	// In production, use telemetry.ProfileProduction
	err := telemetry.Initialize(telemetry.UseProfile(telemetry.ProfileDevelopment))
	if err != nil {
		log.Fatalf("Failed to initialize telemetry: %v", err)
	}
	defer telemetry.Shutdown(context.Background())

	// Example 1: Simple metrics emission
	fmt.Println("Example 1: Simple metrics")
	telemetry.Counter("app.requests", "endpoint", "/api/users", "method", "GET")
	telemetry.Histogram("app.response_time", 125.5, "endpoint", "/api/users")
	telemetry.Gauge("app.active_connections", 42, "server", "api-1")

	// Example 2: Track operation duration
	fmt.Println("\nExample 2: Operation timing")
	done := telemetry.TimeOperation("database.query", "table", "users")
	time.Sleep(50 * time.Millisecond) // Simulate work
	done() // This records the duration

	// Example 3: Error tracking
	fmt.Println("\nExample 3: Error tracking")
	err = simulateOperation()
	if err != nil {
		telemetry.RecordError("app.errors", "timeout", "operation", "fetch_data")
	} else {
		telemetry.RecordSuccess("app.operations", "operation", "fetch_data")
	}

	// Example 4: Circuit breaker with telemetry
	fmt.Println("\nExample 4: Circuit breaker with telemetry")
	cb, err := resilience.NewCircuitBreakerWithTelemetry("api-circuit")
	if err != nil {
		log.Fatalf("Failed to create circuit breaker: %v", err)
	}

	for i := 0; i < 5; i++ {
		err := resilience.ExecuteWithTelemetry(cb, context.Background(), func() error {
			// Simulate API call
			if i < 2 {
				return nil // Success
			}
			return fmt.Errorf("simulated error")
		})
		
		if err != nil {
			fmt.Printf("  Request %d failed: %v\n", i+1, err)
		} else {
			fmt.Printf("  Request %d succeeded\n", i+1)
		}
	}

	// Example 5: Retry with telemetry
	fmt.Println("\nExample 5: Retry with telemetry")
	attempts := 0
	err = resilience.RetryWithTelemetry(
		context.Background(),
		"data_fetch",
		resilience.DefaultRetryConfig(),
		func() error {
			attempts++
			if attempts < 3 {
				return fmt.Errorf("attempt %d failed", attempts)
			}
			return nil // Success on third attempt
		},
	)
	
	if err != nil {
		fmt.Printf("  Retry failed: %v\n", err)
	} else {
		fmt.Printf("  Retry succeeded after %d attempts\n", attempts)
	}

	// Example 6: Advanced emission with options
	fmt.Println("\nExample 6: Advanced telemetry")
	ctx := context.Background()
	telemetry.EmitWithOptions(ctx, "app.custom_metric", 99.9,
		telemetry.WithLabel("env", "production"),
		telemetry.WithLabel("region", "us-west-2"),
		telemetry.WithUnit(telemetry.UnitMilliseconds),
		telemetry.WithSampleRate(0.1), // Sample 10% of these metrics
	)

	// Example 7: Batch emission for efficiency
	fmt.Println("\nExample 7: Batch metrics")
	metrics := []struct {
		Name   string
		Value  float64
		Labels []string
	}{
		{"batch.metric1", 10, []string{"type", "a"}},
		{"batch.metric2", 20, []string{"type", "b"}},
		{"batch.metric3", 30, []string{"type", "c"}},
	}
	telemetry.BatchEmit(metrics)

	// Get telemetry health status
	fmt.Println("\n=== Telemetry Health ===")
	health := telemetry.GetHealth()
	fmt.Printf("Initialized: %v\n", health.Initialized)
	fmt.Printf("Metrics Emitted: %d\n", health.MetricsEmitted)
	fmt.Printf("Errors: %d\n", health.Errors)
	fmt.Printf("Circuit State: %s\n", health.CircuitState)
	fmt.Printf("Uptime: %s\n", health.Uptime)
	
	// Get internal metrics
	internal := telemetry.GetInternalMetrics()
	fmt.Printf("\nInternal Metrics:\n")
	fmt.Printf("  Emitted: %d\n", internal.Emitted)
	fmt.Printf("  Dropped: %d\n", internal.Dropped)
	fmt.Printf("  Errors: %d\n", internal.Errors)
}

func simulateOperation() error {
	// Simulate a successful operation
	return nil
}