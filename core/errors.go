package core

import (
	"errors"
)

// Standard sentinel errors for comparison using errors.Is()
// These are generic errors that can be wrapped with additional context
var (
	// Agent-related errors
	ErrAgentNotFound      = errors.New("agent not found")
	ErrAgentNotReady      = errors.New("agent not ready")
	ErrAgentAlreadyExists = errors.New("agent already exists")
	
	// Capability-related errors
	ErrCapabilityNotFound   = errors.New("capability not found")
	ErrCapabilityNotEnabled = errors.New("capability not enabled")
	
	// Discovery-related errors
	ErrServiceNotFound      = errors.New("service not found")
	ErrDiscoveryUnavailable = errors.New("discovery service unavailable")
	
	// Configuration errors
	ErrInvalidConfiguration = errors.New("invalid configuration")
	ErrMissingConfiguration = errors.New("missing required configuration")
	ErrPortOutOfRange       = errors.New("port out of range")
	
	// State errors
	ErrAlreadyStarted   = errors.New("already started")
	ErrNotInitialized   = errors.New("not initialized")
	ErrAlreadyRegistered = errors.New("already registered")
	
	// Operation errors
	ErrTimeout          = errors.New("operation timeout")
	ErrContextCanceled  = errors.New("context canceled")
	ErrMaxRetriesExceeded = errors.New("maximum retries exceeded")
	
	// HTTP/Network errors
	ErrConnectionFailed = errors.New("connection failed")
	ErrRequestFailed    = errors.New("request failed")
	
	// Resilience errors
	ErrCircuitBreakerOpen = errors.New("circuit breaker open")
	
	// AI operation errors
	ErrAIOperationFailed = errors.New("AI operation failed")
)


// IsRetryable checks if an error is retryable
// Retryable errors are typically transient network or availability issues
func IsRetryable(err error) bool {
	return errors.Is(err, ErrDiscoveryUnavailable) ||
		errors.Is(err, ErrTimeout) ||
		errors.Is(err, ErrConnectionFailed) ||
		errors.Is(err, ErrServiceNotFound) ||
		errors.Is(err, ErrCircuitBreakerOpen)
}

// IsNotFound checks if an error represents a "not found" condition
func IsNotFound(err error) bool {
	return errors.Is(err, ErrAgentNotFound) ||
		errors.Is(err, ErrCapabilityNotFound) ||
		errors.Is(err, ErrServiceNotFound)
}

// IsConfigurationError checks if an error is configuration-related
func IsConfigurationError(err error) bool {
	return errors.Is(err, ErrInvalidConfiguration) ||
		errors.Is(err, ErrMissingConfiguration)
}

// IsStateError checks if an error is related to invalid state transitions
func IsStateError(err error) bool {
	return errors.Is(err, ErrAlreadyStarted) ||
		errors.Is(err, ErrNotInitialized) ||
		errors.Is(err, ErrAlreadyRegistered) ||
		errors.Is(err, ErrAgentNotReady)
}