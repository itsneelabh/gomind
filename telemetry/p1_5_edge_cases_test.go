package telemetry

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestP1_5_EdgeCase_EmptyServiceName verifies service name validation
func TestP1_5_EdgeCase_EmptyServiceName(t *testing.T) {
	// Test with empty service name
	provider, err := NewOTelProvider("", "localhost:4318")

	if provider != nil {
		t.Error("Provider should be nil when service name is empty")
	}

	if err == nil {
		t.Error("Should return error for empty service name")
	}

	if err != nil && err.Error() != "service name cannot be empty" {
		t.Errorf("Wrong error message: %v", err)
	}
}

// TestP1_5_EdgeCase_ConcurrentShutdown verifies shutdown is thread-safe and idempotent
func TestP1_5_EdgeCase_ConcurrentShutdown(t *testing.T) {
	provider, err := NewOTelProvider("concurrent-shutdown-test", "localhost:4318")
	if err != nil {
		t.Skipf("Cannot test concurrent shutdown without provider: %v", err)
		return
	}

	// Track shutdown results
	var shutdownErrors []error
	var shutdownCount int
	var mu sync.Mutex

	// Launch multiple goroutines trying to shutdown concurrently
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := provider.Shutdown(ctx)

			mu.Lock()
			shutdownCount++
			if err != nil {
				shutdownErrors = append(shutdownErrors, err)
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	// All shutdowns should complete without panic
	if shutdownCount != numGoroutines {
		t.Errorf("Expected %d shutdowns, got %d", numGoroutines, shutdownCount)
	}

	// Shutdown errors are acceptable (e.g., context deadline exceeded)
	// The key is no panics occurred
	t.Logf("Concurrent shutdown completed with %d errors out of %d attempts",
		len(shutdownErrors), numGoroutines)
}

// TestP1_5_EdgeCase_MethodsAfterShutdown verifies methods handle shutdown gracefully
func TestP1_5_EdgeCase_MethodsAfterShutdown(t *testing.T) {
	provider, err := NewOTelProvider("post-shutdown-test", "localhost:4318")
	if err != nil {
		t.Skipf("Cannot test post-shutdown without provider: %v", err)
		return
	}

	// Shutdown the provider
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = provider.Shutdown(ctx)

	// Now try to use methods after shutdown - should not panic

	// Test StartSpan after shutdown
	spanCtx, span := provider.StartSpan(context.Background(), "test-span")
	if spanCtx == nil {
		t.Error("StartSpan should return valid context even after shutdown")
	}
	if span == nil {
		t.Error("StartSpan should return non-nil span (no-op) after shutdown")
	}

	// Should be safe to call methods on the span
	span.SetAttribute("test", "value")
	span.RecordError(nil)
	span.End()

	// Test RecordMetric after shutdown - should not panic
	provider.RecordMetric("test.metric", 42.0, map[string]string{
		"label": "value",
	})

	// Test another shutdown - should be idempotent
	err = provider.Shutdown(context.Background())
	// Error is acceptable, panic is not
	t.Logf("Second shutdown result: %v", err)
}

// TestP1_5_EdgeCase_NilProviderFields tests defensive programming for nil fields
func TestP1_5_EdgeCase_NilProviderFields(t *testing.T) {
	// Create a provider with nil fields (simulating partial initialization failure)
	provider := &OTelProvider{
		// All fields left as nil/zero
	}

	// These should not panic even with nil internal fields

	// Test StartSpan with nil tracer
	ctx, span := provider.StartSpan(context.Background(), "test")
	if ctx == nil || span == nil {
		t.Error("StartSpan should return valid no-op implementations")
	}

	// Test RecordMetric with nil metrics
	provider.RecordMetric("test.metric", 123.45, nil)

	// Test Shutdown with nil providers
	err := provider.Shutdown(context.Background())
	// Should complete without panic
	t.Logf("Shutdown with nil fields result: %v", err)
}

// TestP1_5_EdgeCase_MultipleShutdownCalls verifies multiple shutdown calls are safe
func TestP1_5_EdgeCase_MultipleShutdownCalls(t *testing.T) {
	provider, err := NewOTelProvider("multiple-shutdown-test", "localhost:4318")
	if err != nil {
		t.Skipf("Cannot test multiple shutdowns without provider: %v", err)
		return
	}

	// Call shutdown multiple times sequentially
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		err := provider.Shutdown(ctx)
		cancel()

		t.Logf("Shutdown call %d result: %v", i+1, err)

		// Should not panic on any call
	}

	// Provider should still be in shutdown state
	// Try to use it - should get no-op behavior
	_, span := provider.StartSpan(context.Background(), "after-multiple-shutdowns")
	span.End() // Should not panic
}

// TestP1_5_EdgeCase_ShutdownWithCancelledContext tests shutdown with already cancelled context
func TestP1_5_EdgeCase_ShutdownWithCancelledContext(t *testing.T) {
	provider, err := NewOTelProvider("cancelled-context-test", "localhost:4318")
	if err != nil {
		t.Skipf("Cannot test with cancelled context without provider: %v", err)
		return
	}

	// Create and immediately cancel context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try shutdown with cancelled context
	err = provider.Shutdown(ctx)

	// Should handle gracefully without panic
	if err == nil {
		t.Log("Shutdown succeeded despite cancelled context")
	} else {
		t.Logf("Shutdown with cancelled context returned: %v", err)
	}
}

// TestP1_5_EdgeCase_ServiceNameValidation tests various service name inputs
func TestP1_5_EdgeCase_ServiceNameValidation(t *testing.T) {
	testCases := []struct {
		name         string
		serviceName  string
		shouldFail   bool
	}{
		{"Empty", "", true},
		{"Whitespace", "   ", false}, // Currently accepts whitespace
		{"Valid", "my-service", false},
		{"With-Special-Chars", "my-service-123_v2", false},
		{"Unicode", "мой-сервис", false},
		{"Very-Long", "this-is-a-very-long-service-name-that-exceeds-typical-limits-but-should-still-work", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := NewOTelProvider(tc.serviceName, "localhost:4318")

			if tc.shouldFail {
				if err == nil {
					t.Errorf("Expected error for service name '%s'", tc.serviceName)
				}
				if provider != nil {
					t.Errorf("Provider should be nil for invalid service name '%s'", tc.serviceName)
				}
			} else {
				// May fail due to endpoint not being available, which is OK
				if err != nil && provider == nil {
					t.Logf("Provider creation failed (OK if OTEL not running): %v", err)
				}
			}
		})
	}
}

// TestP1_5_EdgeCase_RaceConditionOnShutdown tests for race conditions during shutdown
func TestP1_5_EdgeCase_RaceConditionOnShutdown(t *testing.T) {
	provider, err := NewOTelProvider("race-test", "localhost:4318")
	if err != nil {
		t.Skipf("Cannot test race conditions without provider: %v", err)
		return
	}

	// Start goroutines that continuously emit metrics
	stopCh := make(chan struct{})
	var wg sync.WaitGroup

	// Metric emitters
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for {
				select {
				case <-stopCh:
					return
				default:
					provider.RecordMetric("test.metric", float64(id), nil)
					time.Sleep(1 * time.Millisecond)
				}
			}
		}(i)
	}

	// Span creators
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for {
				select {
				case <-stopCh:
					return
				default:
					_, span := provider.StartSpan(context.Background(), "test-span")
					span.SetAttribute("id", id)
					span.End()
					time.Sleep(1 * time.Millisecond)
				}
			}
		}(i)
	}

	// Let them run for a bit
	time.Sleep(100 * time.Millisecond)

	// Now shutdown while operations are happening
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = provider.Shutdown(ctx)
	}()

	// Let shutdown start
	time.Sleep(50 * time.Millisecond)

	// Stop all workers
	close(stopCh)
	wg.Wait()

	// If we got here without panic, the race condition handling works
	t.Log("Race condition test completed successfully")
}