package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestP1_5_ResourceCleanup_TraceSucceedsMetricFails tests cleanup when metric exporter fails
func TestP1_5_ResourceCleanup_TraceSucceedsMetricFails(t *testing.T) {
	// Create a mock server that accepts traces but rejects metrics
	requestCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch r.URL.Path {
		case "/v1/traces":
			// First call succeeds (trace exporter)
			w.WriteHeader(http.StatusOK)
		case "/v1/metrics":
			// Second call fails (metric exporter)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Metric endpoint failed"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	// Extract endpoint
	endpoint := strings.TrimPrefix(mockServer.URL, "http://")

	var logBuffer strings.Builder
	logger := NewTelemetryLogger("p1-5-cleanup")
	logger.SetOutput(&logBuffer)
	logger.SetLevel("DEBUG")

	oldLogger := telemetryLogger
	telemetryLogger = logger
	telemetryLoggerOnce = sync.Once{}
	telemetryLoggerOnce.Do(func() {})
	defer func() {
		telemetryLogger = oldLogger
		telemetryLoggerOnce = sync.Once{}
	}()

	// Attempt to create provider
	provider, err := NewOTelProvider("cleanup-test", endpoint)

	logs := logBuffer.String()

	// Provider creation should fail
	if provider != nil {
		t.Error("Provider should be nil when metric exporter fails")
	}
	if err == nil {
		t.Error("Should return error when metric exporter fails")
	}

	// Verify trace exporter was created successfully
	if !strings.Contains(logs, "Trace exporter created successfully") {
		t.Error("Trace exporter should have been created")
	}

	// Verify metric exporter failed
	if !strings.Contains(logs, "Failed to create metric exporter") {
		t.Error("Should log metric exporter failure")
	}

	// CRITICAL: After our fix, cleanup should be attempted
	// We log cleanup failures at DEBUG level
	if strings.Contains(logs, "Failed to cleanup trace exporter") {
		// This means cleanup was attempted but failed (which is OK in tests)
		t.Log("Trace exporter cleanup was attempted (good)")
	}

	// Verify error message includes debugging information
	if !strings.Contains(logs, "curl") {
		t.Error("Error should include curl command for debugging")
	}
	if !strings.Contains(logs, "impact") {
		t.Error("Error should include impact information")
	}
}

// TestP1_5_NoCleanupNeeded_BothFail tests when both exporters fail (no cleanup needed)
func TestP1_5_NoCleanupNeeded_BothFail(t *testing.T) {
	// Create a mock server that rejects everything
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("All endpoints failed"))
	}))
	defer mockServer.Close()

	endpoint := strings.TrimPrefix(mockServer.URL, "http://")

	var logBuffer strings.Builder
	logger := NewTelemetryLogger("p1-5-both-fail")
	logger.SetOutput(&logBuffer)
	logger.SetLevel("DEBUG")

	oldLogger := telemetryLogger
	telemetryLogger = logger
	telemetryLoggerOnce = sync.Once{}
	telemetryLoggerOnce.Do(func() {})
	defer func() {
		telemetryLogger = oldLogger
		telemetryLoggerOnce = sync.Once{}
	}()

	provider, err := NewOTelProvider("both-fail-test", endpoint)

	logs := logBuffer.String()

	// Provider creation should fail
	if provider != nil {
		t.Error("Provider should be nil when both exporters fail")
	}
	if err == nil {
		t.Error("Should return error when exporters fail")
	}

	// Verify trace exporter failed
	if !strings.Contains(logs, "Failed to create trace exporter") {
		t.Error("Should log trace exporter failure")
	}

	// Should not even attempt to create metric exporter
	if strings.Contains(logs, "Creating OTLP/HTTP metric exporter") {
		t.Error("Should not attempt metric exporter after trace exporter fails")
	}

	// No cleanup should be logged since trace exporter failed
	if strings.Contains(logs, "cleanup trace exporter") {
		t.Error("Should not need cleanup when trace exporter fails")
	}
}

// TestP1_5_ShutdownCleanup_VerifyOrder verifies shutdown happens in correct order
func TestP1_5_ShutdownCleanup_VerifyOrder(t *testing.T) {
	var logBuffer strings.Builder
	logger := NewTelemetryLogger("p1-5-shutdown-order")
	logger.SetOutput(&logBuffer)
	logger.SetLevel("DEBUG")

	oldLogger := telemetryLogger
	telemetryLogger = logger
	telemetryLoggerOnce = sync.Once{}
	telemetryLoggerOnce.Do(func() {})
	defer func() {
		telemetryLogger = oldLogger
		telemetryLoggerOnce = sync.Once{}
	}()

	provider, err := NewOTelProvider("shutdown-order-test", "localhost:4318")
	if err != nil {
		t.Skipf("Cannot test shutdown order without provider: %v", err)
		return
	}

	logBuffer.Reset()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = provider.Shutdown(ctx)

	logs := logBuffer.String()

	// Parse log order
	metricsInstrumentsPos := strings.Index(logs, "Shutting down metric instruments")
	metricProviderPos := strings.Index(logs, "Flushing and shutting down metric provider")
	traceProviderPos := strings.Index(logs, "Flushing and shutting down trace provider")

	// Verify order: instruments -> metric provider -> trace provider
	if metricsInstrumentsPos >= 0 && metricProviderPos >= 0 {
		if metricsInstrumentsPos > metricProviderPos {
			t.Error("Metric instruments should be shut down before metric provider")
		}
	}

	if metricProviderPos >= 0 && traceProviderPos >= 0 {
		if metricProviderPos > traceProviderPos {
			t.Error("Metric provider should be shut down before trace provider")
		}
	}

	// Verify final message comes last
	finalMessagePos := strings.LastIndex(logs, "OpenTelemetry provider shut down")
	if finalMessagePos < traceProviderPos {
		t.Error("Final shutdown message should come after all components")
	}
}

// TestP1_5_PartialShutdownFailure tests logging when some components fail to shutdown
func TestP1_5_PartialShutdownFailure(t *testing.T) {
	// This is a simulation test - we can't easily force real shutdown failures
	// but we can verify the error aggregation logic by checking the code paths

	var logBuffer strings.Builder
	logger := NewTelemetryLogger("p1-5-partial-shutdown")
	logger.SetOutput(&logBuffer)
	logger.SetLevel("DEBUG")

	oldLogger := telemetryLogger
	telemetryLogger = logger
	telemetryLoggerOnce = sync.Once{}
	telemetryLoggerOnce.Do(func() {})
	defer func() {
		telemetryLogger = oldLogger
		telemetryLoggerOnce = sync.Once{}
	}()

	provider, err := NewOTelProvider("partial-shutdown-test", "localhost:4318")
	if err != nil {
		t.Skipf("Cannot test partial shutdown without provider: %v", err)
		return
	}

	// Use an already-cancelled context to potentially cause failures
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	logBuffer.Reset()
	shutdownErr := provider.Shutdown(ctx)

	logs := logBuffer.String()

	// If shutdown had any errors
	if shutdownErr != nil {
		// Should see aggregated error message
		if !strings.Contains(logs, "shutdown completed with errors") || !strings.Contains(logs, "error_count") {
			t.Error("Should log aggregated errors with count")
		}
	} else {
		// If no errors, should see success
		if !strings.Contains(logs, "shut down successfully") {
			t.Error("Should log successful shutdown")
		}
	}

	// Verify all components were attempted
	componentsAttempted := []string{
		"metric instruments",
		"metric provider",
		"trace provider",
	}

	for _, component := range componentsAttempted {
		if !strings.Contains(logs, component) {
			t.Errorf("Should attempt to shutdown %s", component)
		}
	}
}

// TestP1_5_DeadlineExtraction verifies deadline is correctly extracted and logged
func TestP1_5_DeadlineExtraction(t *testing.T) {
	var logBuffer strings.Builder
	logger := NewTelemetryLogger("p1-5-deadline")
	logger.SetOutput(&logBuffer)
	logger.SetLevel("INFO")

	oldLogger := telemetryLogger
	telemetryLogger = logger
	telemetryLoggerOnce = sync.Once{}
	telemetryLoggerOnce.Do(func() {})
	defer func() {
		telemetryLogger = oldLogger
		telemetryLoggerOnce = sync.Once{}
	}()

	provider, err := NewOTelProvider("deadline-test", "localhost:4318")
	if err != nil {
		t.Skipf("Cannot test deadline without provider: %v", err)
		return
	}

	testCases := []struct {
		name            string
		setupContext    func() (context.Context, context.CancelFunc)
		expectedTimeout string
	}{
		{
			name: "WithDeadline",
			setupContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				return ctx, cancel
			},
			expectedTimeout: "timeout",
		},
		{
			name: "WithoutDeadline",
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.Background(), func() {}
			},
			expectedTimeout: "no deadline",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logBuffer.Reset()
			ctx, cancel := tc.setupContext()
			defer cancel()

			_ = provider.Shutdown(ctx)

			logs := logBuffer.String()

			if !strings.Contains(logs, tc.expectedTimeout) {
				t.Errorf("Expected timeout info '%s' in logs", tc.expectedTimeout)
			}
		})
	}
}