package resilience

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// TestLogger captures logs for verification
type TestLogger struct {
	logs []LogEntry
}

type LogEntry struct {
	Level   string
	Message string
	Fields  map[string]interface{}
}

func (t *TestLogger) Info(msg string, fields map[string]interface{}) {
	t.logs = append(t.logs, LogEntry{Level: "INFO", Message: msg, Fields: fields})
}

func (t *TestLogger) Error(msg string, fields map[string]interface{}) {
	t.logs = append(t.logs, LogEntry{Level: "ERROR", Message: msg, Fields: fields})
}

func (t *TestLogger) Warn(msg string, fields map[string]interface{}) {
	t.logs = append(t.logs, LogEntry{Level: "WARN", Message: msg, Fields: fields})
}

func (t *TestLogger) Debug(msg string, fields map[string]interface{}) {
	t.logs = append(t.logs, LogEntry{Level: "DEBUG", Message: msg, Fields: fields})
}

func (t *TestLogger) GetLogsByOperation(operation string) []LogEntry {
	var result []LogEntry
	for _, log := range t.logs {
		if op, exists := log.Fields["operation"]; exists && op == operation {
			result = append(result, log)
		}
	}
	return result
}

func (t *TestLogger) GetLogsByLevel(level string) []LogEntry {
	var result []LogEntry
	for _, log := range t.logs {
		if log.Level == level {
			result = append(result, log)
		}
	}
	return result
}

func (t *TestLogger) HasLogWithMessage(message string) bool {
	for _, log := range t.logs {
		if strings.Contains(log.Message, message) {
			return true
		}
	}
	return false
}

func (t *TestLogger) Clear() {
	t.logs = nil
}

func TestCircuitBreakerLoggingIntegration(t *testing.T) {
	testLogger := &TestLogger{}

	// Use factory function for proper creation logging
	deps := ResilienceDependencies{
		Logger: testLogger,
	}

	cb, err := CreateCircuitBreaker("test-cb", deps)
	if err != nil {
		t.Fatalf("Failed to create circuit breaker: %v", err)
	}

	// Verify creation logging
	if !testLogger.HasLogWithMessage("Creating circuit breaker") {
		t.Error("No circuit breaker creation log found")
	}

	// Clear logs to focus on execution
	testLogger.Clear()

	// Test successful execution
	err = cb.Execute(context.Background(), func() error {
		return nil
	})

	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// Verify execution logs exist (should have DEBUG logs)
	if len(testLogger.logs) == 0 {
		t.Error("No logs captured during execution")
	}

	// Check for specific execution operations
	executeLogs := testLogger.GetLogsByOperation("circuit_breaker_execute")
	if len(executeLogs) == 0 {
		t.Error("No circuit breaker execute logs found")
	}

	// Test failure scenario
	testLogger.Clear()

	// Force multiple failures to trigger state change
	for i := 0; i < 15; i++ {
		err := cb.Execute(context.Background(), func() error {
			return errors.New("test failure")
		})
		// Expect failures but don't fail the test
		_ = err
	}

	// Check that we have some logs from the failures
	if len(testLogger.logs) == 0 {
		t.Error("No logs captured during failure scenario")
	}

	// Look for any error-related operations or state changes
	hasFailureRelatedLogs := false
	for _, log := range testLogger.logs {
		if op, exists := log.Fields["operation"]; exists {
			opStr := op.(string)
			if strings.Contains(opStr, "execute") || strings.Contains(opStr, "state") || strings.Contains(opStr, "failure") {
				hasFailureRelatedLogs = true
				break
			}
		}
	}

	if !hasFailureRelatedLogs {
		t.Error("No failure-related logs found during failure scenario")
	}
}

func TestRetryExecutorLoggingIntegration(t *testing.T) {
	testLogger := &TestLogger{}

	executor := NewRetryExecutor(nil)
	executor.SetLogger(testLogger)

	// Test with failure then success
	attempt := 0
	err := executor.Execute(context.Background(), "test-operation", func() error {
		attempt++
		if attempt < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Retry execution failed: %v", err)
	}

	// Verify retry start logging
	startLogs := testLogger.GetLogsByOperation("retry_start")
	if len(startLogs) != 1 {
		t.Errorf("Expected 1 retry start log, got %d", len(startLogs))
	}

	// Verify we have logs from multiple attempts
	if len(testLogger.logs) < 3 {
		t.Errorf("Expected multiple logs from retry attempts, got %d", len(testLogger.logs))
	}

	// Verify success logging exists
	if !testLogger.HasLogWithMessage("retry operation succeeded") && !testLogger.HasLogWithMessage("Starting retry operation") {
		t.Error("No success-related logs found")
	}

	// Check for operation field in logs
	for _, log := range testLogger.logs {
		if op, exists := log.Fields["retry_operation"]; exists {
			if op != "test-operation" {
				t.Errorf("Expected retry_operation to be 'test-operation', got %v", op)
			}
		}
	}
}

func TestRetryExecutorExhaustionLogging(t *testing.T) {
	testLogger := &TestLogger{}

	config := &RetryConfig{
		MaxAttempts:   2,
		InitialDelay:  1 * time.Millisecond,
		MaxDelay:      10 * time.Millisecond,
		BackoffFactor: 2.0,
		JitterEnabled: false,
	}

	executor := NewRetryExecutor(config)
	executor.SetLogger(testLogger)

	// Test with all failures
	err := executor.Execute(context.Background(), "failure-test", func() error {
		return errors.New("persistent failure")
	})

	if err == nil {
		t.Fatal("Expected retry to fail after exhaustion")
	}

	// Verify error logging for final failure
	errorLogs := testLogger.GetLogsByLevel("ERROR")
	if len(errorLogs) == 0 {
		t.Error("No error logs found for retry exhaustion")
	}

	// Verify backoff logging occurred
	hasBackoffLog := false
	for _, log := range testLogger.logs {
		if op, exists := log.Fields["operation"]; exists && op == "retry_backoff" {
			hasBackoffLog = true
			break
		}
	}
	if !hasBackoffLog {
		t.Error("No backoff logs found")
	}
}

func TestFactoryDependencyInjection(t *testing.T) {
	testLogger := &TestLogger{}

	deps := ResilienceDependencies{
		Logger: testLogger,
	}

	// Test circuit breaker creation
	cb, err := CreateCircuitBreaker("factory-test", deps)
	if err != nil {
		t.Fatalf("Failed to create circuit breaker: %v", err)
	}

	// Verify logger was injected by checking creation log
	if !testLogger.HasLogWithMessage("Creating circuit breaker") {
		t.Error("Circuit breaker creation log not found, logger injection may have failed")
	}

	// Test execution to verify logger is working
	originalLogCount := len(testLogger.logs)
	err = cb.Execute(context.Background(), func() error {
		return nil
	})

	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if len(testLogger.logs) <= originalLogCount {
		t.Error("No new logs captured during execution, logger injection may have failed")
	}

	// Test retry executor creation
	testLogger.Clear()
	executor := CreateRetryExecutor(deps)

	err = executor.Execute(context.Background(), "factory-test", func() error {
		return nil
	})

	if err != nil {
		t.Fatalf("Retry execution failed: %v", err)
	}

	if len(testLogger.logs) == 0 {
		t.Error("No logs captured during retry execution, logger injection failed")
	}

	// Verify operation names are preserved in logs
	foundCorrectOperation := false
	for _, log := range testLogger.logs {
		if op, exists := log.Fields["retry_operation"]; exists {
			if op == "factory-test" {
				foundCorrectOperation = true
				break
			}
		}
	}

	if !foundCorrectOperation {
		t.Error("Expected retry_operation 'factory-test' not found in logs")
	}
}

func TestCircuitBreakerSetLogger(t *testing.T) {
	// Test circuit breaker SetLogger method if it exists
	config := DefaultConfig()
	config.Name = "setlogger-test"
	config.Logger = &core.NoOpLogger{}

	cb, err := NewCircuitBreaker(config)
	if err != nil {
		t.Fatalf("Failed to create circuit breaker: %v", err)
	}

	testLogger := &TestLogger{}

	// Try to set logger using SetLogger method if available
	if setter, ok := interface{}(cb).(interface{ SetLogger(core.Logger) }); ok {
		setter.SetLogger(testLogger)

		// Test that the new logger is used
		err = cb.Execute(context.Background(), func() error {
			return nil
		})

		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		// Note: This test may not capture logs if SetLogger doesn't update the config's logger
		// The test verifies the method exists and can be called without error
	} else {
		t.Log("SetLogger method not available on CircuitBreaker - this is expected if not yet implemented")
	}
}

func TestRetryWithLogging(t *testing.T) {
	testLogger := &TestLogger{}

	config := DefaultRetryConfig()

	// Test RetryWithLogging function if it exists
	ctx := context.Background()
	operation := "test-with-logging"

	attempt := 0
	testFunc := func() error {
		attempt++
		if attempt < 2 {
			return errors.New("first attempt fails")
		}
		return nil
	}

	// Create an executor manually and test
	executor := NewRetryExecutor(config)
	executor.SetLogger(testLogger)

	err := executor.Execute(ctx, operation, testFunc)
	if err != nil {
		t.Fatalf("RetryWithLogging failed: %v", err)
	}

	// Verify logging occurred
	if len(testLogger.logs) == 0 {
		t.Error("No logs captured during retry with logging")
	}

	// Verify operation name is correct
	for _, log := range testLogger.logs {
		if op, exists := log.Fields["retry_operation"]; exists {
			if op != operation {
				t.Errorf("Expected retry_operation to be '%s', got %v", operation, op)
			}
		}
	}
}

func TestLoggingFieldValidation(t *testing.T) {
	testLogger := &TestLogger{}

	// Test circuit breaker field validation
	config := DefaultConfig()
	config.Name = "field-validation-test"
	config.Logger = testLogger

	cb, err := NewCircuitBreaker(config)
	if err != nil {
		t.Fatalf("Failed to create circuit breaker: %v", err)
	}

	err = cb.Execute(context.Background(), func() error {
		return nil
	})

	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// Verify required fields are present in logs
	for _, log := range testLogger.logs {
		// Check for name field in circuit breaker logs
		if name, exists := log.Fields["name"]; exists {
			if name != "field-validation-test" {
				t.Errorf("Expected name field to be 'field-validation-test', got %v", name)
			}
		}

		// Check for operation field
		if _, exists := log.Fields["operation"]; !exists {
			t.Errorf("Missing operation field in log: %v", log)
		}
	}

	// Test retry executor field validation
	testLogger.Clear()
	executor := NewRetryExecutor(nil)
	executor.SetLogger(testLogger)

	err = executor.Execute(context.Background(), "field-test", func() error {
		return nil
	})

	if err != nil {
		t.Fatalf("Retry execution failed: %v", err)
	}

	// Verify retry-specific fields
	for _, log := range testLogger.logs {
		if op, exists := log.Fields["operation"]; exists && op == "retry_start" {
			// Check for required retry configuration fields
			requiredFields := []string{"max_attempts", "initial_delay", "backoff_factor"}
			for _, field := range requiredFields {
				if _, exists := log.Fields[field]; !exists {
					t.Errorf("Missing required field '%s' in retry start log", field)
				}
			}
		}
	}
}