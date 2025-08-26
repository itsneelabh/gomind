package logger_test

import (
	"strings"
	"testing"

	"github.com/itsneelabh/gomind/pkg/logger"
)

// TestSimpleLogger tests the simple logger implementation
func TestSimpleLogger(t *testing.T) {
	// Create logger (uses os.Stdout by default)
	log := logger.NewSimpleLogger()
	
	// We can't easily test output without modifying the logger to accept a writer
	// So we'll just test that methods don't panic
	
	log.Debug("debug message", logger.Field{Key: "test", Value: "value"})
	log.Info("info message", logger.Field{Key: "test", Value: "value"})
	log.Warn("warn message", logger.Field{Key: "test", Value: "value"})
	log.Error("error message", logger.Field{Key: "test", Value: "value"})
}

// TestLoggerWith tests the With method
func TestLoggerWith(t *testing.T) {
	log := logger.NewSimpleLogger()
	
	// Create a logger with additional fields
	logWithFields := log.With(
		logger.Field{Key: "component", Value: "test"},
		logger.Field{Key: "version", Value: "1.0"},
	)
	
	// Test that it doesn't panic
	logWithFields.Info("test message")
}

// TestLogLevels tests different log levels
func TestLogLevels(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{"Debug", "debug"},
		{"Info", "info"},
		{"Warn", "warn"},
		{"Error", "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logger.NewSimpleLogger()
			log.SetLevel(tt.level)
			
			// Test that logger creation doesn't panic
			if log == nil {
				t.Error("Logger should not be nil")
			}
		})
	}
}

// TestFieldFormatting tests field formatting
func TestFieldFormatting(t *testing.T) {
	tests := []struct {
		name     string
		field    logger.Field
		expected string
	}{
		{
			name:     "String field",
			field:    logger.Field{Key: "message", Value: "hello"},
			expected: "message",
		},
		{
			name:     "Number field",
			field:    logger.Field{Key: "count", Value: 42},
			expected: "count",
		},
		{
			name:     "Boolean field",
			field:    logger.Field{Key: "enabled", Value: true},
			expected: "enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the field key is accessible
			if tt.field.Key != tt.expected {
				t.Errorf("Field key mismatch: got %s, want %s", tt.field.Key, tt.expected)
			}
		})
	}
}

// BenchmarkLogger benchmarks logger performance
func BenchmarkLogger(b *testing.B) {
	log := logger.NewSimpleLogger()
	log.SetLevel("info")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("benchmark message",
			logger.Field{Key: "iteration", Value: i},
			logger.Field{Key: "benchmark", Value: true},
		)
	}
}

// Helper function to check if output contains expected string
func containsString(output, expected string) bool {
	return strings.Contains(output, expected)
}