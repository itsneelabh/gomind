package telemetry

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// TestFrameworkIntegration tests that telemetry registers itself with core
func TestFrameworkIntegration(t *testing.T) {
	// Reset initialization for this test
	initOnce = sync.Once{}
	globalRegistry.Store(nil)

	// Initialize telemetry with test configuration
	config := Config{
		ServiceName:      "framework-integration-test",
		Endpoint:         "localhost:4318",
		CardinalityLimit: 1000,
		Provider:         "otel",
	}

	err := Initialize(config)
	if err != nil {
		t.Logf("Initialization error (expected if OTEL not running): %v", err)
		// Continue test even if OTEL is not running
	}

	// Verify that core's metrics registry is set
	registry := core.GetGlobalMetricsRegistry()
	if registry == nil {
		t.Fatal("Framework integration failed: core.GetGlobalMetricsRegistry() returned nil")
	}

	// Test Counter method
	t.Run("Counter", func(t *testing.T) {
		// This should not panic even if telemetry backend is not available
		registry.Counter("test.counter", "label1", "value1")
	})

	// Test EmitWithContext method
	t.Run("EmitWithContext", func(t *testing.T) {
		ctx := context.Background()
		// Add some baggage
		ctx = WithBaggage(ctx,
			"request_id", "test-123",
			"user_id", "user-456",
		)

		// This should not panic
		registry.EmitWithContext(ctx, "test.metric", 100.5, "env", "test")
	})

	// Test GetBaggage method
	t.Run("GetBaggage", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithBaggage(ctx,
			"trace_id", "trace-789",
			"span_id", "span-012",
		)

		retrieved := registry.GetBaggage(ctx)
		if retrieved == nil {
			t.Error("GetBaggage returned nil")
		}
		if retrieved["trace_id"] != "trace-789" {
			t.Errorf("Expected trace_id=trace-789, got %s", retrieved["trace_id"])
		}
	})

	// Clean up
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	Shutdown(ctx)
}

// TestFrameworkIntegrationLogging tests that framework integration logs correctly
func TestFrameworkIntegrationLogging(t *testing.T) {
	// Create a test logger with captured output
	logger := NewTelemetryLogger("integration-test")
	logger.SetLevel("DEBUG")

	// Create framework registry
	registry := NewFrameworkMetricsRegistry(logger)

	// Test that debug logging works
	var logOutput strings.Builder
	logger.SetOutput(&logOutput)

	// Test Counter logging
	registry.Counter("test.counter", "key", "value")
	output := logOutput.String()
	if !strings.Contains(output, "Framework metric emission") {
		t.Errorf("Expected 'Framework metric emission' log, got: %s", output)
	}
	if !strings.Contains(output, `"source":"framework"`) {
		t.Errorf("Expected source=framework in log, got: %s", output)
	}

	// Reset output
	logOutput.Reset()

	// Test EmitWithContext logging
	ctx := WithBaggage(context.Background(), "request_id", "req-123")
	registry.EmitWithContext(ctx, "test.metric", 42.0, "tag", "value")
	output = logOutput.String()
	if !strings.Contains(output, "Framework context-aware emission") {
		t.Errorf("Expected 'Framework context-aware emission' log, got: %s", output)
	}
	if !strings.Contains(output, `"request_id":"req-123"`) {
		t.Errorf("Expected request_id in log, got: %s", output)
	}
}

// TestFrameworkIntegrationWithoutTelemetry tests core behavior when telemetry is not initialized
func TestFrameworkIntegrationWithoutTelemetry(t *testing.T) {
	// Reset core's global registry
	core.SetMetricsRegistry(nil)

	// Get the registry - should be nil
	registry := core.GetGlobalMetricsRegistry()
	if registry != nil {
		t.Error("Expected nil registry when telemetry not initialized")
	}

	// Core components should handle nil registry gracefully
	// This is tested in core module, but we verify the contract here
}