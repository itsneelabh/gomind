package ui

import (
	"context"
	"errors"
	"fmt"
)

// Error categories for the UI module
var (
	// Transport errors
	ErrTransportNotFound      = errors.New("transport not found")
	ErrTransportAlreadyExists = errors.New("transport already registered")
	ErrTransportNotAvailable  = errors.New("transport not available")
	ErrTransportNotStarted    = errors.New("transport not started")
	ErrTransportShutdown      = errors.New("transport is shutting down")
	ErrTransportUnhealthy     = errors.New("transport health check failed")
	
	// Session errors
	ErrSessionNotFound     = errors.New("session not found")
	ErrSessionExpired      = errors.New("session expired")
	ErrSessionRateLimited  = errors.New("session rate limited")
	ErrSessionTokenLimit   = errors.New("session token limit exceeded")
	ErrSessionInvalid      = errors.New("invalid session")
	
	// Message errors
	ErrMessageTooLarge     = errors.New("message too large")
	ErrMessageInvalid      = errors.New("invalid message format")
	ErrMessageEmpty        = errors.New("message cannot be empty")
	
	// Configuration errors
	ErrInvalidConfig       = errors.New("invalid configuration")
	ErrMissingRedis        = errors.New("redis connection required")
	ErrMissingAIClient     = errors.New("AI client required")
	
	// Stream errors
	ErrStreamClosed        = errors.New("stream closed")
	ErrStreamTimeout       = errors.New("stream timeout")
	ErrStreamCancelled     = errors.New("stream cancelled")
)

// UIError provides structured error information
type UIError struct {
	Op       string      // Operation that failed
	Kind     ErrorKind   // Category of error
	Err      error       // Underlying error
	Message  string      // Human-readable message
	Metadata interface{} // Additional context
}

// ErrorKind categorizes errors
type ErrorKind string

const (
	ErrorKindTransport     ErrorKind = "transport"
	ErrorKindSession       ErrorKind = "session"
	ErrorKindMessage       ErrorKind = "message"
	ErrorKindConfiguration ErrorKind = "configuration"
	ErrorKindStream        ErrorKind = "stream"
	ErrorKindInternal      ErrorKind = "internal"
)

// Error implements the error interface
func (e *UIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s: %s", e.Op, e.Kind, e.Message)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Kind, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Kind)
}

// Unwrap returns the underlying error
func (e *UIError) Unwrap() error {
	return e.Err
}

// Is checks if the error matches the target
func (e *UIError) Is(target error) bool {
	if e.Err != nil {
		return errors.Is(e.Err, target)
	}
	return false
}

// NewUIError creates a new UIError
func NewUIError(op string, kind ErrorKind, err error) *UIError {
	return &UIError{
		Op:   op,
		Kind: kind,
		Err:  err,
	}
}

// IsRetryable determines if an error should be retried
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	
	// Check for specific retryable errors
	switch {
	case errors.Is(err, ErrTransportUnhealthy):
		return true
	case errors.Is(err, ErrSessionRateLimited):
		return true
	case errors.Is(err, ErrStreamTimeout):
		return true
	case errors.Is(err, context.DeadlineExceeded):
		return true
	}
	
	// Check if it's a UIError with retryable kind
	var uiErr *UIError
	if errors.As(err, &uiErr) {
		switch uiErr.Kind {
		case ErrorKindTransport, ErrorKindStream:
			return true
		}
	}
	
	return false
}

// IsFatal determines if an error is fatal and should not be retried
func IsFatal(err error) bool {
	if err == nil {
		return false
	}
	
	// Check for specific fatal errors
	switch {
	case errors.Is(err, ErrInvalidConfig):
		return true
	case errors.Is(err, ErrMissingRedis):
		return true
	case errors.Is(err, ErrMissingAIClient):
		return true
	case errors.Is(err, ErrSessionTokenLimit):
		return true
	case errors.Is(err, ErrMessageTooLarge):
		return true
	}
	
	// Check if it's a UIError with fatal kind
	var uiErr *UIError
	if errors.As(err, &uiErr) {
		switch uiErr.Kind {
		case ErrorKindConfiguration:
			return true
		}
	}
	
	return false
}

// ErrorResponse represents an error response to send to clients
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Code    string                 `json:"code,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ToErrorResponse converts an error to an ErrorResponse
func ToErrorResponse(err error) ErrorResponse {
	if err == nil {
		return ErrorResponse{Error: "unknown error"}
	}
	
	response := ErrorResponse{
		Error: err.Error(),
	}
	
	// Add specific error codes
	switch {
	case errors.Is(err, ErrTransportNotFound):
		response.Code = "TRANSPORT_NOT_FOUND"
	case errors.Is(err, ErrSessionNotFound):
		response.Code = "SESSION_NOT_FOUND"
	case errors.Is(err, ErrSessionExpired):
		response.Code = "SESSION_EXPIRED"
	case errors.Is(err, ErrSessionRateLimited):
		response.Code = "RATE_LIMITED"
	case errors.Is(err, ErrMessageTooLarge):
		response.Code = "MESSAGE_TOO_LARGE"
	case errors.Is(err, ErrInvalidConfig):
		response.Code = "INVALID_CONFIG"
	}
	
	// Add metadata if it's a UIError
	var uiErr *UIError
	if errors.As(err, &uiErr) && uiErr.Metadata != nil {
		if details, ok := uiErr.Metadata.(map[string]interface{}); ok {
			response.Details = details
		}
	}
	
	return response
}