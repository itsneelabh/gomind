// Package ui provides user interface components for the GoMind framework.
// This file defines dependency injection structures and patterns for UI components.
//
// Purpose:
// - Provides centralized dependency management for UI components
// - Enables clean separation of concerns through dependency injection
// - Allows optional dependencies with graceful fallbacks to no-op implementations
// - Supports both full framework integration and standalone usage
//
// Scope:
// - Dependencies struct: Main container for all UI component dependencies
// - ChatAgentDependencies: Specialized subset for chat functionality
// - Functional options pattern for configuring ChatAgent instances
// - Default implementations for optional dependencies (Logger, Telemetry, Memory)
//
// Architecture:
// The dependency injection pattern here allows UI components to:
// 1. Work with minimal dependencies (all are optional)
// 2. Integrate seamlessly with core framework services when available
// 3. Provide testability through interface-based dependencies
// 4. Support progressive enhancement based on available services
//
// Usage:
// Components should accept Dependencies or ChatAgentDependencies in constructors,
// call WithDefaults() to ensure safe fallbacks, and use functional options
// (WithLogger, WithTelemetry, etc.) for fine-grained configuration.
package ui

import "github.com/itsneelabh/gomind/core"

// Dependencies provides external dependencies for UI components.
// This allows proper dependency injection without direct module imports.
type Dependencies struct {
	// Logger for logging events (optional, uses NoOpLogger if nil)
	Logger core.Logger

	// Telemetry for metrics and tracing (optional, uses NoOpTelemetry if nil)
	Telemetry core.Telemetry

	// CircuitBreaker for fault tolerance (optional, circuit breaking disabled if nil)
	CircuitBreaker core.CircuitBreaker

	// AIClient for AI operations (optional, AI features disabled if nil)
	AIClient core.AIClient

	// Memory for state storage (optional, uses in-memory store if nil)
	Memory core.Memory
}

// ChatAgentDependencies provides dependencies specifically for ChatAgent.
// This is a subset of Dependencies focused on chat functionality.
type ChatAgentDependencies struct {
	// Logger for logging chat events
	Logger core.Logger

	// Telemetry for chat metrics and tracing
	Telemetry core.Telemetry

	// CircuitBreaker for protecting chat transports
	CircuitBreaker core.CircuitBreaker

	// AIClient for AI-powered responses
	AIClient core.AIClient
}

// WithDefaults returns dependencies with default implementations for nil fields
func (d Dependencies) WithDefaults() Dependencies {
	result := d

	if result.Logger == nil {
		result.Logger = &core.NoOpLogger{}
	}

	if result.Telemetry == nil {
		result.Telemetry = &core.NoOpTelemetry{}
	}

	if result.Memory == nil {
		result.Memory = core.NewInMemoryStore()
	}

	// CircuitBreaker and AIClient remain nil if not provided
	// This allows features to be disabled when dependencies are not available

	return result
}

// Validate checks if required dependencies are present
func (d Dependencies) Validate() error {
	// Currently all dependencies are optional
	// Add validation logic here if some become required
	return nil
}

// ChatAgentOption is a functional option for configuring ChatAgent
type ChatAgentOption func(*DefaultChatAgent)

// WithLogger sets the logger for the chat agent
func WithLogger(logger core.Logger) ChatAgentOption {
	return func(agent *DefaultChatAgent) {
		if logger != nil {
			agent.BaseAgent.Logger = logger
		}
	}
}

// WithTelemetry sets the telemetry provider for the chat agent
func WithTelemetry(telemetry core.Telemetry) ChatAgentOption {
	return func(agent *DefaultChatAgent) {
		if telemetry != nil {
			agent.BaseAgent.Telemetry = telemetry
		}
	}
}

// WithCircuitBreaker sets the circuit breaker for the chat agent
func WithCircuitBreaker(cb core.CircuitBreaker) ChatAgentOption {
	return func(agent *DefaultChatAgent) {
		agent.circuitBreaker = cb
	}
}

// WithAIClient sets the AI client for the chat agent
func WithAIClient(client core.AIClient) ChatAgentOption {
	return func(agent *DefaultChatAgent) {
		agent.aiClient = client
	}
}
