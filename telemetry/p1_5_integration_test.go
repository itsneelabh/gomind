package telemetry

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestP1_5_Integration_FullLifecycle tests the complete lifecycle with a mock OTEL collector
func TestP1_5_Integration_FullLifecycle(t *testing.T) {
	// Create a mock OTEL collector
	mockCollector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate OTEL collector endpoints
		switch r.URL.Path {
		case "/v1/traces":
			w.WriteHeader(http.StatusOK)
		case "/v1/metrics":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockCollector.Close()

	// Extract host and port
	host, port, _ := net.SplitHostPort(mockCollector.Listener.Addr().String())
	endpoint := fmt.Sprintf("%s:%s", host, port)

	// Setup logging
	var logBuffer strings.Builder
	logger := NewTelemetryLogger("p1-5-integration")
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

	// Test creation
	provider, err := NewOTelProvider("integration-test", endpoint)
	if err != nil {
		t.Fatalf("Failed to create provider with mock collector: %v", err)
	}

	creationLogs := logBuffer.String()

	// Verify all creation logs
	if !strings.Contains(creationLogs, "Creating OpenTelemetry provider") {
		t.Error("Missing provider creation start log")
	}
	if !strings.Contains(creationLogs, "OpenTelemetry provider created successfully") {
		t.Error("Missing provider creation success log")
	}
	if !strings.Contains(creationLogs, "initialization_ms") {
		t.Error("Missing initialization timing")
	}

	// Clear buffer for shutdown test
	logBuffer.Reset()

	// Test shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = provider.Shutdown(ctx)
	if err != nil {
		t.Errorf("Unexpected shutdown error: %v", err)
	}

	shutdownLogs := logBuffer.String()

	// Verify all shutdown logs
	if !strings.Contains(shutdownLogs, "Shutting down OpenTelemetry provider") {
		t.Error("Missing shutdown start log")
	}
	if !strings.Contains(shutdownLogs, "OpenTelemetry provider shut down successfully") {
		t.Error("Missing shutdown success log")
	}
	if !strings.Contains(shutdownLogs, "shutdown_ms") {
		t.Error("Missing shutdown timing")
	}
}

// TestP1_5_Integration_FailureScenarios tests various failure scenarios
func TestP1_5_Integration_FailureScenarios(t *testing.T) {
	testCases := []struct {
		name            string
		endpoint        string
		expectedErrors  []string
		expectedActions []string
	}{
		{
			name:            "InvalidEndpoint",
			endpoint:        "nonexistent.invalid:12345",
			expectedErrors:  []string{"Failed to create"},
			expectedActions: []string{"action", "Verify OTEL collector"},
		},
		{
			name:            "InvalidPort",
			endpoint:        "localhost:99999",
			expectedErrors:  []string{"Failed to create"},
			expectedActions: []string{"curl"},
		},
		{
			name:            "EmptyEndpointUsesDefault",
			endpoint:        "",
			expectedErrors:  []string{}, // May or may not fail depending on local setup
			expectedActions: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var logBuffer strings.Builder
			logger := NewTelemetryLogger(fmt.Sprintf("p1-5-%s", tc.name))
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

			_, err := NewOTelProvider(tc.name, tc.endpoint)

			logs := logBuffer.String()

			// Check for expected error logging
			if err != nil {
				for _, expectedError := range tc.expectedErrors {
					if !strings.Contains(logs, expectedError) {
						t.Errorf("Expected error log containing '%s'", expectedError)
					}
				}

				// Verify actionable information
				for _, expectedAction := range tc.expectedActions {
					if !strings.Contains(logs, expectedAction) {
						t.Errorf("Expected action information containing '%s'", expectedAction)
					}
				}

				// Verify curl command is provided for debugging
				if strings.Contains(logs, "Failed to create") && !strings.Contains(logs, "curl") {
					t.Error("Error logs should include curl command for debugging")
				}
			}
		})
	}
}

// TestP1_5_Integration_ConcurrentProviderCreation tests thread safety of logging
func TestP1_5_Integration_ConcurrentProviderCreation(t *testing.T) {
	// This tests that concurrent provider creation doesn't cause logging issues
	var wg sync.WaitGroup
	numGoroutines := 5

	// Use a thread-safe writer
	var logBuffer safeBuffer
	logger := NewTelemetryLogger("p1-5-concurrent")
	logger.SetOutput(&logBuffer)
	logger.SetLevel("INFO") // Reduce noise

	oldLogger := telemetryLogger
	telemetryLogger = logger
	telemetryLoggerOnce = sync.Once{}
	telemetryLoggerOnce.Do(func() {})
	defer func() {
		telemetryLogger = oldLogger
		telemetryLoggerOnce = sync.Once{}
	}()

	providers := make([]*OTelProvider, 0, numGoroutines)
	var providersMu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			provider, err := NewOTelProvider(fmt.Sprintf("concurrent-%d", id), "localhost:4318")
			if err == nil && provider != nil {
				providersMu.Lock()
				providers = append(providers, provider)
				providersMu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Check that we got logs (no panics)
	logs := logBuffer.String()
	if logs == "" {
		t.Error("Expected some logs from concurrent creation")
	}

	// Clean up providers
	ctx := context.Background()
	for _, p := range providers {
		_ = p.Shutdown(ctx)
	}
}

// safeBuffer is a thread-safe buffer for concurrent logging tests
type safeBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// TestP1_5_Integration_ShutdownTimeout tests behavior when shutdown times out
func TestP1_5_Integration_ShutdownTimeout(t *testing.T) {
	var logBuffer strings.Builder
	logger := NewTelemetryLogger("p1-5-timeout")
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

	provider, err := NewOTelProvider("timeout-test", "localhost:4318")
	if err != nil {
		t.Skipf("Cannot test timeout without provider: %v", err)
		return
	}

	// Use an extremely short timeout to force timeout behavior
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Let context timeout
	time.Sleep(2 * time.Millisecond)

	logBuffer.Reset()
	_ = provider.Shutdown(ctx)

	logs := logBuffer.String()

	// Should still log shutdown attempt
	if !strings.Contains(logs, "Shutting down OpenTelemetry provider") {
		t.Error("Should log shutdown attempt even with timeout")
	}

	// Should log the timeout condition
	if !strings.Contains(logs, "timeout") {
		t.Error("Should log timeout information")
	}
}

// TestP1_5_Integration_MetricsInstrumentCreation verifies metric instruments are initialized
func TestP1_5_Integration_MetricsInstrumentCreation(t *testing.T) {
	var logBuffer strings.Builder
	logger := NewTelemetryLogger("p1-5-metrics")
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

	provider, err := NewOTelProvider("metrics-test", "localhost:4318")

	logs := logBuffer.String()

	// Verify metric instruments initialization is logged
	if !strings.Contains(logs, "Initializing metric instruments") {
		t.Error("Should log metric instruments initialization")
	}

	if err == nil && provider != nil {
		// Verify metrics field is set
		if provider.metrics == nil {
			t.Error("Provider metrics should be initialized")
		}

		// Clean up
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = provider.Shutdown(ctx)
	}
}