# Error Handling Example

A comprehensive demonstration of error handling patterns and best practices using the GoMind framework's standardized error types and utilities.

## 🎯 What This Example Demonstrates

- **Standard Error Types**: Using framework-provided sentinel errors for consistent error handling
- **Error Classification**: Leveraging `IsRetryable()` and `IsConfigurationError()` for intelligent error handling
- **Structured Errors**: Using `FrameworkError` for detailed error context and debugging
- **Error Wrapping**: Proper error wrapping and unwrapping patterns with `errors.Is()` and `errors.As()`
- **Agent Error Patterns**: Best practices for error handling in agent implementations
- **Production Error Handling**: Real-world patterns for robust error management

## 🏗️ Error Types Covered

### Sentinel Errors
- `ErrAgentNotFound` - Standard error for missing agents
- `ErrTimeout` - Network and operation timeouts
- `ErrDiscoveryUnavailable` - Service discovery failures
- `ErrInvalidConfiguration` - Configuration validation errors
- `ErrAlreadyStarted` - Component lifecycle errors

### Utility Functions
- `IsRetryable(err)` - Determines if an error should trigger retry logic
- `IsConfigurationError(err)` - Identifies configuration-related errors
- `errors.Is()` - Standard Go error comparison
- `errors.As()` - Type assertion for structured errors

### Structured Errors
- `FrameworkError` - Rich error context with operation, kind, and message

## 📝 Code Structure

```
error-handling-example/
├── main.go          # Complete demonstration of all error patterns
├── go.mod          # Module configuration with local dependencies
└── README.md       # This documentation
```

## 🚀 Running the Example

### Prerequisites
- Go 1.25 or later
- GoMind framework (automatically resolved via local replace directive)

### Quick Start

```bash
# Navigate to the example directory
cd examples/error_handling

# Run the demonstration
env GOWORK=off go run .
```

### Expected Output
The example demonstrates various error scenarios:
- Agent not found handling
- Retryable error detection
- Structured error information
- Configuration error identification

```
2025/09/24 21:31:07 Agent not found, creating a new one...
2025/09/24 21:31:07 Error is retryable, will attempt retry...
2025/09/24 21:31:07 Operation failed: discovery.Register in discovery
2025/09/24 21:31:07 Configuration error detected, please check your settings
```

## 🔍 Key Learning Points

### 1. Consistent Error Comparison
```go
// Use errors.Is() for sentinel errors
if errors.Is(err, core.ErrAgentNotFound) {
    // Handle specific error case
}
```

### 2. Intelligent Error Classification
```go
// Use framework utilities for decision making
if core.IsRetryable(err) {
    // Implement retry logic
}
```

### 3. Rich Error Context
```go
// Use FrameworkError for detailed context
return &core.FrameworkError{
    Op:      "MyAgent.Initialize",
    Kind:    "agent",
    Message: "failed to initialize agent",
    Err:     err,
}
```

### 4. Type-Safe Error Handling
```go
// Use errors.As() for structured errors
var frameworkErr *core.FrameworkError
if errors.As(err, &frameworkErr) {
    log.Printf("Operation: %s, Kind: %s", frameworkErr.Op, frameworkErr.Kind)
}
```

## 🛠️ Customization

This example serves as a reference implementation. You can:

1. **Extend Error Types**: Add custom error types following the same patterns
2. **Enhance Error Context**: Add more fields to FrameworkError for your use case
3. **Implement Custom Utilities**: Create domain-specific error classification functions
4. **Add Error Metrics**: Integrate with telemetry for error tracking and monitoring

## 📚 Related Examples

- [Context Propagation Example](../context_propagation/) - Complementary telemetry and context handling
- [Agent Example](../agent-example/) - Real-world agent implementation with error handling
- [AI Agent Example](../ai-agent-example/) - AI integration with comprehensive error management

## 🔗 Framework Documentation

- [Core Error Types](../../core/errors.go) - Complete error type definitions
- [Error Utilities](../../core/errors_test.go) - Additional usage examples and tests