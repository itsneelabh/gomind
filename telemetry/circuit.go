package telemetry

import (
	"sync"
	"sync/atomic"
	"time"
)

// TelemetryCircuitBreaker protects telemetry backend from overload
type TelemetryCircuitBreaker struct {
	config CircuitConfig

	state           atomic.Value // string: "closed", "open", "half-open"
	failures        atomic.Int64
	successes       atomic.Int64
	lastFailureTime atomic.Value // time.Time

	mu sync.Mutex
}

// CircuitConfig configures the telemetry circuit breaker
type CircuitConfig struct {
	Enabled      bool
	MaxFailures  int
	RecoveryTime time.Duration
	HalfOpenMax  int // Max requests in half-open state
}

// NewTelemetryCircuitBreaker creates a new circuit breaker
func NewTelemetryCircuitBreaker(config CircuitConfig) *TelemetryCircuitBreaker {
	if !config.Enabled {
		return nil
	}

	// Set defaults
	if config.MaxFailures == 0 {
		config.MaxFailures = 10
	}
	if config.RecoveryTime == 0 {
		config.RecoveryTime = 30 * time.Second
	}
	if config.HalfOpenMax == 0 {
		config.HalfOpenMax = 5
	}

	cb := &TelemetryCircuitBreaker{
		config: config,
	}
	cb.state.Store("closed")
	cb.lastFailureTime.Store(time.Time{})

	return cb
}

// Allow checks if a request should be allowed
func (cb *TelemetryCircuitBreaker) Allow() bool {
	if cb == nil {
		return true // No circuit breaker configured
	}

	state := cb.State()

	switch state {
	case "open":
		// Check if we should transition to half-open
		lastFailure := cb.lastFailureTime.Load().(time.Time)
		if time.Since(lastFailure) > cb.config.RecoveryTime {
			cb.mu.Lock()
			// Double-check after acquiring lock
			if cb.state.Load().(string) == "open" {
				cb.state.Store("half-open")
				cb.successes.Store(0)
			}
			cb.mu.Unlock()
			return true
		}
		return false

	case "half-open":
		// Allow limited requests in half-open state
		return cb.successes.Load() < int64(cb.config.HalfOpenMax)

	default: // closed
		return true
	}
}

// RecordSuccess records a successful operation.
// In half-open state, enough successes will close the circuit.
// In closed state, this resets the failure counter.
// This helps the circuit breaker recover after the backend is healthy again.
func (cb *TelemetryCircuitBreaker) RecordSuccess() {
	if cb == nil {
		return
	}

	cb.successes.Add(1)
	state := cb.State()

	if state == "half-open" {
		// Check if we should close the circuit
		if cb.successes.Load() >= int64(cb.config.HalfOpenMax) {
			cb.mu.Lock()
			if cb.state.Load().(string) == "half-open" {
				cb.state.Store("closed")
				cb.failures.Store(0)
			}
			cb.mu.Unlock()
		}
	}
}

// RecordFailure records a failed operation
func (cb *TelemetryCircuitBreaker) RecordFailure() {
	if cb == nil {
		return
	}

	failures := cb.failures.Add(1)
	cb.lastFailureTime.Store(time.Now())

	if failures >= int64(cb.config.MaxFailures) {
		cb.mu.Lock()
		if cb.state.Load().(string) != "open" {
			cb.state.Store("open")
			cb.successes.Store(0)
		}
		cb.mu.Unlock()
	}
}

// State returns the current circuit breaker state
func (cb *TelemetryCircuitBreaker) State() string {
	if cb == nil {
		return "disabled"
	}
	return cb.state.Load().(string)
}

// Reset resets the circuit breaker
func (cb *TelemetryCircuitBreaker) Reset() {
	if cb == nil {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state.Store("closed")
	cb.failures.Store(0)
	cb.successes.Store(0)
	cb.lastFailureTime.Store(time.Time{})
}
