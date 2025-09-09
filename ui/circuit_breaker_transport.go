// Package ui provides resilient transport implementations for the GoMind framework.
// This file implements circuit breaker protection for UI transports to prevent
// cascading failures and provide fault tolerance.
//
// Purpose:
// - Wraps UI transports with circuit breaker protection
// - Prevents cascading failures when downstream services fail
// - Provides automatic recovery testing through half-open state
// - Returns fast failures when circuit is open
// - Integrates with core.CircuitBreaker interface for flexibility
//
// Scope:
// - InterfaceBasedCircuitBreakerTransport: Main circuit breaker wrapper
// - HTTP handler wrapping with circuit breaker logic
// - Error response generation for circuit breaker states
// - Health check integration with circuit state
// - Metrics and state exposure through circuit breaker interface
//
// Circuit Breaker Integration:
// This transport delegates circuit breaker logic to core.CircuitBreaker:
// 1. Closed: Normal operation, requests pass through
// 2. Open: Fast failure with 503 Service Unavailable
// 3. Half-Open: Limited requests to test recovery
//
// Error Handling:
// - Circuit Open: Returns 503 with circuit state information
// - Execution Error: Records failure and returns appropriate HTTP status
// - Success: Records success to help close circuit
//
// Architecture Benefits:
// - Separation of concerns: Transport logic vs circuit breaker logic
// - Pluggable implementations: Any core.CircuitBreaker can be used
// - Testability: Circuit breaker can be mocked for testing
// - Observability: Metrics and state exposed through interface
//
// Usage:
// breaker := someCircuitBreakerImpl // Any core.CircuitBreaker implementation
// transport := NewInterfaceBasedCircuitBreakerTransport(baseTransport, breaker)
// Automatically protects all requests through the transport with circuit breaking.
package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/itsneelabh/gomind/core"
)

// InterfaceBasedCircuitBreakerTransport wraps a transport with circuit breaker protection
// using an injected CircuitBreaker implementation from the core interface.
// This replaces the duplicated circuit breaker logic with a proper abstraction.
type InterfaceBasedCircuitBreakerTransport struct {
	underlying Transport
	breaker    core.CircuitBreaker
	name       string
}

// NewInterfaceBasedCircuitBreakerTransport creates a new circuit breaker transport
// using an injected CircuitBreaker implementation.
func NewInterfaceBasedCircuitBreakerTransport(transport Transport, breaker core.CircuitBreaker) Transport {
	return &InterfaceBasedCircuitBreakerTransport{
		underlying: transport,
		breaker:    breaker,
		name:       fmt.Sprintf("%s-cb", transport.Name()),
	}
}

// Name returns the transport name with circuit breaker suffix
func (t *InterfaceBasedCircuitBreakerTransport) Name() string {
	return t.name
}

// Description returns the description including circuit breaker info
func (t *InterfaceBasedCircuitBreakerTransport) Description() string {
	return fmt.Sprintf("%s (with circuit breaker protection)", t.underlying.Description())
}

// Available checks if the transport is available considering circuit state
func (t *InterfaceBasedCircuitBreakerTransport) Available() bool {
	// Check if circuit breaker would allow execution
	if !t.breaker.CanExecute() {
		return false
	}
	// Also check underlying transport availability
	return t.underlying.Available()
}

// Priority returns the underlying transport priority
func (t *InterfaceBasedCircuitBreakerTransport) Priority() int {
	return t.underlying.Priority()
}

// Initialize initializes the underlying transport
func (t *InterfaceBasedCircuitBreakerTransport) Initialize(config TransportConfig) error {
	return t.underlying.Initialize(config)
}

// Start starts the underlying transport
func (t *InterfaceBasedCircuitBreakerTransport) Start(ctx context.Context) error {
	return t.underlying.Start(ctx)
}

// Stop stops the underlying transport
func (t *InterfaceBasedCircuitBreakerTransport) Stop(ctx context.Context) error {
	return t.underlying.Stop(ctx)
}

// CreateHandler wraps the underlying handler with circuit breaker protection
func (t *InterfaceBasedCircuitBreakerTransport) CreateHandler(agent ChatAgent) http.Handler {
	originalHandler := t.underlying.CreateHandler(agent)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a wrapper to capture the response status
		// Use existing responseWriterWrapper from circuit_breaker.go
		wrapped := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:     200,
		}

		// Execute the handler with circuit breaker protection
		err := t.breaker.Execute(r.Context(), func() error {
			// Call the original handler
			originalHandler.ServeHTTP(wrapped, r)

			// Check if the response indicates a server error
			// Server errors (5xx) should trigger circuit breaker
			// Client errors (4xx) should not
			if wrapped.statusCode >= 500 {
				return fmt.Errorf("server error: HTTP %d", wrapped.statusCode)
			}

			return nil
		})

		// Handle circuit breaker errors
		if err != nil {
			// Check if circuit breaker is open
			if errors.Is(err, core.ErrCircuitBreakerOpen) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Circuit-Breaker", "open")
				w.WriteHeader(http.StatusServiceUnavailable)

				response := map[string]interface{}{
					"error":   "Service temporarily unavailable",
					"message": "Circuit breaker is open due to recent failures",
					"retry":   "Please try again later",
				}
				json.NewEncoder(w).Encode(response)
				return
			}

			// For other errors, return internal server error
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Internal server error",
			})
		}
	})
}

// HealthCheck performs health check with circuit breaker protection
func (t *InterfaceBasedCircuitBreakerTransport) HealthCheck(ctx context.Context) error {
	return t.breaker.Execute(ctx, func() error {
		return t.underlying.HealthCheck(ctx)
	})
}

// Capabilities returns the underlying transport capabilities
func (t *InterfaceBasedCircuitBreakerTransport) Capabilities() []TransportCapability {
	return t.underlying.Capabilities()
}

// ClientExample returns the underlying transport client example
func (t *InterfaceBasedCircuitBreakerTransport) ClientExample() string {
	return t.underlying.ClientExample()
}

// GetCircuitBreakerState returns the current state of the circuit breaker
func (t *InterfaceBasedCircuitBreakerTransport) GetCircuitBreakerState() string {
	return t.breaker.GetState()
}

// GetCircuitBreakerMetrics returns metrics from the circuit breaker
func (t *InterfaceBasedCircuitBreakerTransport) GetCircuitBreakerMetrics() map[string]interface{} {
	return t.breaker.GetMetrics()
}
