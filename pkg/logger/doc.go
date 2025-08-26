// Package logger provides structured logging capabilities for the GoMind Agent Framework.
//
// This package offers a simple yet powerful logging interface that supports structured
// fields, multiple log levels, and contextual logging for better observability and debugging.
//
// # Logger Interface
//
// The Logger interface defines the contract for all logging implementations:
//
//	type Logger interface {
//	    Debug(msg string, fields map[string]interface{})
//	    Info(msg string, fields map[string]interface{})
//	    Warn(msg string, fields map[string]interface{})
//	    Error(msg string, fields map[string]interface{})
//	    With(fields map[string]interface{}) Logger
//	}
//
// # Log Levels
//
// Supported log levels in order of severity:
//   - DEBUG: Detailed information for debugging
//   - INFO: General informational messages
//   - WARN: Warning messages for potentially harmful situations
//   - ERROR: Error messages for serious problems
//
// # Structured Logging
//
// All log methods accept structured fields for rich context:
//
//	logger.Info("Processing request", map[string]interface{}{
//	    "user_id": "123",
//	    "action": "create_order",
//	    "duration_ms": 145,
//	})
//
// # Contextual Logging
//
// Create child loggers with persistent fields:
//
//	requestLogger := logger.With(map[string]interface{}{
//	    "request_id": "abc-123",
//	    "user_id": "456",
//	})
//	
//	// All logs from requestLogger will include request_id and user_id
//	requestLogger.Info("Starting processing", nil)
//	requestLogger.Info("Processing complete", map[string]interface{}{
//	    "items_processed": 10,
//	})
//
// # Simple Logger Implementation
//
// The package provides SimpleLogger, a production-ready implementation:
//   - JSON or text output format
//   - Configurable log levels
//   - Timestamp inclusion
//   - Field formatting and sanitization
//
// # Configuration
//
// Loggers can be configured through environment variables:
//   - LOG_LEVEL: Minimum log level (debug, info, warn, error)
//   - LOG_FORMAT: Output format (json, text)
//
// # Integration with Agents
//
// Loggers are automatically injected into agents that embed BaseAgent,
// providing immediate access to logging capabilities:
//
//	func (a *MyAgent) ProcessData(input string) error {
//	    a.Logger().Info("Processing data", map[string]interface{}{
//	        "input_length": len(input),
//	    })
//	    // ... processing logic ...
//	}
//
// # Best Practices
//
//   - Use appropriate log levels to control verbosity
//   - Include relevant context through structured fields
//   - Avoid logging sensitive information (passwords, tokens, PII)
//   - Use child loggers for request-scoped logging
//   - Keep log messages concise and actionable
package logger