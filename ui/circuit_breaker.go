package ui

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// CircuitState represents the state of the circuit breaker
type CircuitState int

const (
	// StateClosed allows requests to pass through (normal operation)
	StateClosed CircuitState = iota
	// StateOpen blocks all requests (failing)
	StateOpen
	// StateHalfOpen allows limited requests to test recovery
	StateHalfOpen
)

// String returns the string representation of the circuit state
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig configures the circuit breaker behavior
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures before opening the circuit
	FailureThreshold int
	// SuccessThreshold is the number of successes in half-open state before closing
	SuccessThreshold int
	// Timeout is how long to wait before attempting recovery
	Timeout time.Duration
	// MaxRequests is the maximum number of requests allowed in half-open state
	MaxRequests int
	
	// Optional telemetry and logging
	Telemetry core.Telemetry
	Logger    core.Logger
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:         30 * time.Second,
		MaxRequests:     1,
	}
}

// CircuitBreakerTransport wraps any transport with circuit breaker functionality
type CircuitBreakerTransport struct {
	// The underlying transport being wrapped
	underlying Transport
	
	// Circuit breaker state
	state          CircuitState
	failureCount   int
	successCount   int
	lastFailTime   time.Time
	halfOpenRequests int
	
	// Configuration
	config CircuitBreakerConfig
	
	// Thread safety
	mu sync.RWMutex
}

// NewCircuitBreakerTransport creates a new circuit breaker wrapper for any transport
func NewCircuitBreakerTransport(transport Transport, config CircuitBreakerConfig) Transport {
	// Set defaults if not provided
	if config.FailureThreshold == 0 {
		config.FailureThreshold = 5
	}
	if config.SuccessThreshold == 0 {
		config.SuccessThreshold = 2
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRequests == 0 {
		config.MaxRequests = 1
	}
	
	// Ensure MaxRequests is sufficient for SuccessThreshold
	// This prevents impossible configurations where we can't make enough
	// requests in HalfOpen state to meet the success threshold
	if config.MaxRequests < config.SuccessThreshold {
		config.MaxRequests = config.SuccessThreshold
	}
	
	return &CircuitBreakerTransport{
		underlying: transport,
		state:     StateClosed,
		config:    config,
	}
}

// Name returns the underlying transport name with circuit breaker suffix
func (cb *CircuitBreakerTransport) Name() string {
	return fmt.Sprintf("%s-cb", cb.underlying.Name())
}

// Description returns the description including circuit breaker info
func (cb *CircuitBreakerTransport) Description() string {
	return fmt.Sprintf("%s (with circuit breaker)", cb.underlying.Description())
}

// Available checks if the transport is available considering circuit state
func (cb *CircuitBreakerTransport) Available() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	// Check circuit state
	if cb.state == StateOpen {
		// Check if we should transition to half-open
		if time.Since(cb.lastFailTime) > cb.config.Timeout {
			return true // Will transition to half-open on next attempt
		}
		return false
	}
	
	// Also check underlying transport availability
	return cb.underlying.Available()
}

// Priority returns the underlying transport priority
func (cb *CircuitBreakerTransport) Priority() int {
	return cb.underlying.Priority()
}

// Initialize initializes the underlying transport
func (cb *CircuitBreakerTransport) Initialize(config TransportConfig) error {
	return cb.underlying.Initialize(config)
}

// Start starts the underlying transport with circuit breaker protection
func (cb *CircuitBreakerTransport) Start(ctx context.Context) error {
	if !cb.canAttempt() {
		return cb.createCircuitOpenError()
	}
	
	err := cb.underlying.Start(ctx)
	cb.recordResult(err)
	
	return err
}

// Stop stops the underlying transport
func (cb *CircuitBreakerTransport) Stop(ctx context.Context) error {
	// Always allow stop operations
	return cb.underlying.Stop(ctx)
}

// CreateHandler wraps the underlying handler with circuit breaker logic
func (cb *CircuitBreakerTransport) CreateHandler(agent ChatAgent) http.Handler {
	originalHandler := cb.underlying.CreateHandler(agent)
	
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check circuit breaker state
		if !cb.canAttempt() {
			cb.handleCircuitOpenResponse(w, r)
			return
		}
		
		// Wrap response writer to monitor failures
		wrapped := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:    200,
		}
		
		// Call underlying handler
		originalHandler.ServeHTTP(wrapped, r)
		
		// Record result based on status code
		if wrapped.statusCode >= 500 {
			cb.recordResult(fmt.Errorf("server error: %d", wrapped.statusCode))
		} else if wrapped.statusCode >= 400 {
			// Client errors don't open the circuit
			cb.recordResult(nil)
		} else {
			cb.recordResult(nil)
		}
	})
}

// HealthCheck checks health with circuit breaker protection
func (cb *CircuitBreakerTransport) HealthCheck(ctx context.Context) error {
	if !cb.canAttempt() {
		return cb.createCircuitOpenError()
	}
	
	err := cb.underlying.HealthCheck(ctx)
	cb.recordResult(err)
	
	return err
}

// Capabilities returns the underlying transport capabilities
func (cb *CircuitBreakerTransport) Capabilities() []TransportCapability {
	return cb.underlying.Capabilities()
}

// ClientExample returns the underlying transport client example
func (cb *CircuitBreakerTransport) ClientExample() string {
	return cb.underlying.ClientExample()
}

// canAttempt checks if a request can be attempted
func (cb *CircuitBreakerTransport) canAttempt() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	switch cb.state {
	case StateClosed:
		return true
		
	case StateOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailTime) > cb.config.Timeout {
			// Transition to half-open
			cb.state = StateHalfOpen
			cb.halfOpenRequests = 1  // Count this as the first request
			cb.logStateTransition(StateOpen, StateHalfOpen)
			return true
		}
		return false
		
	case StateHalfOpen:
		// Allow limited requests in half-open state
		if cb.halfOpenRequests < cb.config.MaxRequests {
			cb.halfOpenRequests++
			return true
		}
		return false
		
	default:
		return false
	}
}

// recordResult records the result of an operation
func (cb *CircuitBreakerTransport) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if err != nil {
		cb.recordFailureLocked()
	} else {
		cb.recordSuccessLocked()
	}
}

// recordFailureLocked records a failure (must be called with lock held)
func (cb *CircuitBreakerTransport) recordFailureLocked() {
	cb.failureCount++
	cb.lastFailTime = time.Now()
	
	switch cb.state {
	case StateClosed:
		if cb.failureCount >= cb.config.FailureThreshold {
			// Open the circuit
			oldState := cb.state
			cb.state = StateOpen
			cb.failureCount = 0  // Reset for consistency with HalfOpen->Open transition
			cb.successCount = 0
			cb.logStateTransition(oldState, StateOpen)
			cb.recordMetric("circuit.opened", 1.0)
		}
		
	case StateHalfOpen:
		// Single failure in half-open returns to open
		oldState := cb.state
		cb.state = StateOpen
		cb.failureCount = 0
		cb.successCount = 0
		cb.halfOpenRequests = 0
		cb.logStateTransition(oldState, StateOpen)
		cb.recordMetric("circuit.reopened", 1.0)
	}
}

// recordSuccessLocked records a success (must be called with lock held)
func (cb *CircuitBreakerTransport) recordSuccessLocked() {
	switch cb.state {
	case StateHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.config.SuccessThreshold {
			// Close the circuit
			oldState := cb.state
			cb.state = StateClosed
			cb.failureCount = 0
			cb.successCount = 0
			cb.halfOpenRequests = 0
			cb.logStateTransition(oldState, StateClosed)
			cb.recordMetric("circuit.closed", 1.0)
		}
		
	case StateClosed:
		// Reset failure count on success
		cb.failureCount = 0
	}
}

// createCircuitOpenError creates an error for when circuit is open
func (cb *CircuitBreakerTransport) createCircuitOpenError() error {
	return NewUIError(
		"CircuitBreaker",
		ErrorKindTransport,
		fmt.Errorf("circuit breaker is open for transport %s", cb.underlying.Name()),
	)
}

// handleCircuitOpenResponse handles HTTP response when circuit is open
func (cb *CircuitBreakerTransport) handleCircuitOpenResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Circuit-Breaker", "open")
	w.WriteHeader(http.StatusServiceUnavailable)
	
	fmt.Fprintf(w, `{"error":"Service temporarily unavailable (circuit breaker open)","transport":"%s"}`, 
		cb.underlying.Name())
	
	cb.recordMetric("circuit.rejected", 1.0)
}

// logStateTransition logs circuit breaker state transitions
func (cb *CircuitBreakerTransport) logStateTransition(from, to CircuitState) {
	if cb.config.Logger != nil {
		cb.config.Logger.Info("Circuit breaker state transition", map[string]interface{}{
			"transport": cb.underlying.Name(),
			"from":      from.String(),
			"to":        to.String(),
			"failures":  cb.failureCount,
			"successes": cb.successCount,
		})
	}
}

// recordMetric records telemetry metrics
func (cb *CircuitBreakerTransport) recordMetric(name string, value float64) {
	if cb.config.Telemetry != nil {
		cb.config.Telemetry.RecordMetric(name, value, map[string]string{
			"transport": cb.underlying.Name(),
		})
	}
}

// GetState returns the current circuit breaker state (for monitoring)
func (cb *CircuitBreakerTransport) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns circuit breaker statistics (for monitoring)
func (cb *CircuitBreakerTransport) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	return map[string]interface{}{
		"state":           cb.state.String(),
		"failure_count":   cb.failureCount,
		"success_count":   cb.successCount,
		"last_fail_time":  cb.lastFailTime,
		"half_open_requests": cb.halfOpenRequests,
	}
}

// responseWriterWrapper wraps http.ResponseWriter to capture status code
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}