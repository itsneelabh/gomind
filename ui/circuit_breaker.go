// Package ui provides resilient transport implementations for the GoMind framework.
// This file implements circuit breaker protection for UI transports to prevent
// cascading failures and provide fault tolerance.
//
// Purpose:
// - Wraps UI transports with circuit breaker protection using dependency injection
// - Prevents cascading failures when downstream services fail
// - Provides automatic recovery testing through the injected circuit breaker
// - Returns fast failures when circuit is open (503 Service Unavailable)
// - Integrates with core.CircuitBreaker interface for maximum flexibility
//
// Architecture:
// The CircuitBreakerTransport acts as a decorator pattern, wrapping any Transport
// implementation with circuit breaker protection. It delegates all circuit breaker
// logic to an injected core.CircuitBreaker implementation, maintaining clean
// separation of concerns:
//
//	UI Module (this file) → uses → core.CircuitBreaker interface
//	                                      ↑
//	                               implemented by
//	                                      ↑
//	                          Resilience Module (or custom impl)
//
// Circuit States (managed by injected breaker):
// 1. Closed: Normal operation, requests pass through
// 2. Open: Fast failure with 503 Service Unavailable
// 3. Half-Open: Limited requests to test recovery
//
// Error Handling:
// - Circuit Open: Returns 503 with circuit state information in headers
// - Server Errors (5xx): Recorded as failures in circuit breaker
// - Client Errors (4xx): Not counted as circuit breaker failures
// - Success (2xx, 3xx): Recorded as successes
//
// Usage Example:
//
//	// Create or obtain a circuit breaker implementation
//	breaker := resilience.NewCircuitBreaker(config)
//
//	// Wrap transport with circuit breaker protection
//	transport, err := ui.NewCircuitBreakerTransport(baseTransport, breaker, logger)
//	if err != nil {
//	    // Handle configuration error
//	    return err
//	}
//
//	// Use the protected transport normally
//	handler := transport.CreateHandler(agent)
//
// The circuit breaker is completely pluggable - any implementation of the
// core.CircuitBreaker interface can be used, allowing for custom policies,
// monitoring integration, and service-specific behavior.
package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/itsneelabh/gomind/core"
)

// CircuitBreakerTransport wraps a transport with circuit breaker protection
// using an injected CircuitBreaker implementation from the core interface.
// This provides fault tolerance and prevents cascading failures in distributed systems.
type CircuitBreakerTransport struct {
	underlying Transport
	breaker    core.CircuitBreaker
	name       string
	logger     core.Logger
}

// NewCircuitBreakerTransport creates a new circuit breaker transport
// using an injected CircuitBreaker implementation.
//
// Parameters:
//   - transport: The underlying transport to protect (required)
//   - breaker: The circuit breaker implementation (required)
//   - logger: Logger for operational visibility (optional, uses NoOpLogger if nil)
//
// Returns:
//   - Transport: The wrapped transport with circuit breaker protection
//   - error: An error if required parameters are missing
//
// The circuit breaker will monitor the health of the underlying transport
// and automatically open the circuit when failure thresholds are exceeded.
func NewCircuitBreakerTransport(transport Transport, breaker core.CircuitBreaker, logger core.Logger) (Transport, error) {
	if transport == nil {
		return nil, fmt.Errorf("transport is required for CircuitBreakerTransport")
	}
	if breaker == nil {
		return nil, fmt.Errorf("circuit breaker is required for CircuitBreakerTransport")
	}
	if logger == nil {
		logger = &core.NoOpLogger{}
	}
	return &CircuitBreakerTransport{
		underlying: transport,
		breaker:    breaker,
		name:       fmt.Sprintf("%s-cb", transport.Name()),
		logger:     logger,
	}, nil
}

// SetLogger sets the logger provider (follows framework design principles)
// The component is always set to "framework/ui" to ensure proper log attribution
// regardless of which agent or tool is using the UI module.
func (t *CircuitBreakerTransport) SetLogger(logger core.Logger) {
	if logger == nil {
		t.logger = &core.NoOpLogger{}
	} else {
		if cal, ok := logger.(core.ComponentAwareLogger); ok {
			t.logger = cal.WithComponent("framework/ui")
		} else {
			t.logger = logger
		}
	}
}

// Name returns the transport name with circuit breaker suffix
func (t *CircuitBreakerTransport) Name() string {
	return t.name
}

// Description returns the description including circuit breaker info
func (t *CircuitBreakerTransport) Description() string {
	return fmt.Sprintf("%s (with circuit breaker protection)", t.underlying.Description())
}

// Available checks if the transport is available considering circuit state
func (t *CircuitBreakerTransport) Available() bool {
	// Check if circuit breaker would allow execution
	if !t.breaker.CanExecute() {
		return false
	}
	// Also check underlying transport availability
	return t.underlying.Available()
}

// Priority returns the underlying transport priority
func (t *CircuitBreakerTransport) Priority() int {
	return t.underlying.Priority()
}

// Initialize initializes the underlying transport
func (t *CircuitBreakerTransport) Initialize(config TransportConfig) error {
	return t.underlying.Initialize(config)
}

// Start starts the underlying transport with circuit breaker protection
func (t *CircuitBreakerTransport) Start(ctx context.Context) error {
	// Starting a transport is a critical operation that should be protected
	return t.breaker.Execute(ctx, func() error {
		return t.underlying.Start(ctx)
	})
}

// Stop stops the underlying transport (always allowed, even if circuit is open)
func (t *CircuitBreakerTransport) Stop(ctx context.Context) error {
	// Stopping should always be allowed regardless of circuit state
	return t.underlying.Stop(ctx)
}

// CreateHandler wraps the underlying handler with circuit breaker protection
func (t *CircuitBreakerTransport) CreateHandler(agent ChatAgent) http.Handler {
	originalHandler := t.underlying.CreateHandler(agent)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a wrapper to capture the response status
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
			// Client errors (4xx) should not affect circuit state
			if wrapped.statusCode >= 500 {
				return fmt.Errorf("server error: HTTP %d", wrapped.statusCode)
			}

			return nil
		})

		// Handle circuit breaker errors
		if err != nil {
			// Check if circuit breaker is open
			if errors.Is(err, core.ErrCircuitBreakerOpen) {
				// If response wasn't written yet, write circuit open response
				if wrapped.statusCode == 200 {
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("X-Circuit-Breaker", "open")
					w.Header().Set("X-Circuit-State", t.breaker.GetState())
					w.WriteHeader(http.StatusServiceUnavailable)

					response := map[string]interface{}{
						"error":   "Service temporarily unavailable",
						"message": "Circuit breaker is open due to recent failures",
						"state":   t.breaker.GetState(),
						"retry":   "Please try again later",
					}

					if encErr := json.NewEncoder(w).Encode(response); encErr != nil {
						t.logger.Error("Failed to encode circuit breaker response", map[string]interface{}{
							"operation":  "circuit_breaker_response",
							"transport":  t.name,
							"error":      encErr.Error(),
							"path":       r.URL.Path,
							"method":     r.Method,
						})
					}
				}
				return
			}

			// For other errors during execution (not circuit breaker related)
			// These have already been handled by the wrapped handler
			// Just log for observability
			if wrapped.statusCode == 200 {
				// Handler didn't write a response, so we should
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				if encErr := json.NewEncoder(w).Encode(map[string]string{
					"error": "Internal server error",
				}); encErr != nil {
					t.logger.Error("Failed to encode error response", map[string]interface{}{
						"operation":  "error_response_encoding",
						"transport":  t.name,
						"error":      encErr.Error(),
						"path":       r.URL.Path,
						"method":     r.Method,
					})
				}
			}
		}
	})
}

// HealthCheck performs health check with circuit breaker protection
func (t *CircuitBreakerTransport) HealthCheck(ctx context.Context) error {
	return t.breaker.Execute(ctx, func() error {
		return t.underlying.HealthCheck(ctx)
	})
}

// Capabilities returns the underlying transport capabilities
func (t *CircuitBreakerTransport) Capabilities() []TransportCapability {
	return t.underlying.Capabilities()
}

// ClientExample returns the underlying transport client example
func (t *CircuitBreakerTransport) ClientExample() string {
	return t.underlying.ClientExample()
}

// GetCircuitBreakerState returns the current state of the circuit breaker
func (t *CircuitBreakerTransport) GetCircuitBreakerState() string {
	return t.breaker.GetState()
}

// GetCircuitBreakerMetrics returns metrics from the circuit breaker
func (t *CircuitBreakerTransport) GetCircuitBreakerMetrics() map[string]interface{} {
	return t.breaker.GetMetrics()
}

// responseWriterWrapper wraps http.ResponseWriter to capture status code
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	if !w.written {
		w.statusCode = statusCode
		w.written = true
		w.ResponseWriter.WriteHeader(statusCode)
	}
}

func (w *responseWriterWrapper) Write(data []byte) (int, error) {
	if !w.written {
		w.written = true
		// WriteHeader wasn't called, default to 200
		w.ResponseWriter.WriteHeader(w.statusCode)
	}
	return w.ResponseWriter.Write(data)
}