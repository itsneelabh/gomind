package core

import (
	"errors"
	"fmt"
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
)

// FrameworkError provides structured error information with context
// It implements the error interface and supports error wrapping
type FrameworkError struct {
	Op      string // Operation that failed (e.g., "discovery.Register")
	Kind    string // Error kind (e.g., "agent", "discovery", "config")
	ID      string // Optional ID of the entity involved
	Message string // Human-readable message
	Err     error  // Underlying error for wrapping
}

// Error returns the string representation of the error
func (e *FrameworkError) Error() string {
	if e.Op != "" && e.Err != nil {
		if e.ID != "" {
			return fmt.Sprintf("%s [%s]: %v", e.Op, e.ID, e.Err)
		}
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s error", e.Kind)
}

// Unwrap returns the underlying error for use with errors.Is/As
func (e *FrameworkError) Unwrap() error {
	return e.Err
}

// NewFrameworkError creates a new FrameworkError
func NewFrameworkError(op, kind string, err error) *FrameworkError {
	return &FrameworkError{
		Op:   op,
		Kind: kind,
		Err:  err,
	}
}

// IsRetryable checks if an error is retryable
// Retryable errors are typically transient network or availability issues
func IsRetryable(err error) bool {
	return errors.Is(err, ErrDiscoveryUnavailable) ||
		errors.Is(err, ErrTimeout) ||
		errors.Is(err, ErrConnectionFailed) ||
		errors.Is(err, ErrServiceNotFound)
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