package core

import (
	"fmt"
	"net/http"
)

// ============================================================================
// Intelligent Error Handling - Protocol Definitions
// ============================================================================
//
// This file provides the standardized error protocol for tool-to-agent
// communication. Tools use these types to report errors with context;
// agents decide how to handle them (including AI-powered retry logic).
//
// What's here (protocol definitions):
//   - ErrorCategory: Shared vocabulary for error classification
//   - ToolError: Structured error with context for AI analysis
//   - ToolResponse: Standard response envelope
//   - HTTPStatusForCategory: Utility for consistent HTTP status codes
//
// What's NOT here (agent-specific behavior):
//   - Retry logic (agents implement their own strategies)
//   - AI error analysis (agents use their own AI providers)
//   - RetryConfig/ErrorContext (agent configuration)
//
// See: docs/INTELLIGENT_ERROR_HANDLING_ARCHITECTURE.md

// ErrorCategory classifies errors for agent retry logic decisions.
// This is the shared vocabulary that tools and agents must agree on.
type ErrorCategory string

const (
	// CategoryInputError indicates the request payload was malformed
	// Example: Missing required field, invalid JSON structure
	CategoryInputError ErrorCategory = "INPUT_ERROR"

	// CategoryNotFound indicates the requested resource doesn't exist
	// (but might exist with corrected parameters)
	// Example: City not found, stock symbol unknown
	CategoryNotFound ErrorCategory = "NOT_FOUND"

	// CategoryRateLimit indicates the tool's API quota was exceeded
	// Agents should check Details["retry_after"] for backoff duration
	CategoryRateLimit ErrorCategory = "RATE_LIMIT"

	// CategoryAuthError indicates authentication/authorization failure
	// Typically NOT retryable - requires configuration fix
	CategoryAuthError ErrorCategory = "AUTH_ERROR"

	// CategoryServiceError indicates the tool's backend service failed
	// Usually transient - retry with same payload after backoff
	CategoryServiceError ErrorCategory = "SERVICE_ERROR"
)

// ToolError represents a structured error from a tool capability invocation.
// Tools use this to report errors with context; agents decide how to handle them.
//
// Usage in tools:
//
//	return &core.ToolError{
//	    Code:      "LOCATION_NOT_FOUND",
//	    Message:   "City 'Flower Mound, TX' not found",
//	    Category:  core.CategoryNotFound,
//	    Retryable: true,
//	    Details: map[string]string{
//	        "original_location": "Flower Mound, TX",
//	        "hint": "Try 'City, Country' format",
//	    },
//	}
type ToolError struct {
	// Code is a machine-readable error identifier (e.g., "LOCATION_NOT_FOUND")
	// Tool-specific codes within standard categories
	Code string `json:"code"`

	// Message is a human-readable error description
	Message string `json:"message"`

	// Category groups errors for routing decisions
	Category ErrorCategory `json:"category"`

	// Retryable indicates if the agent should attempt to retry with corrected input
	// When true, agents may use AI to analyze and fix the request
	Retryable bool `json:"retryable"`

	// Details provides additional context for AI analysis
	// Common keys: "original_input", "hint", "api_error", "retry_after"
	Details map[string]string `json:"details,omitempty"`
}

// Error implements the error interface
func (e *ToolError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// ToolResponse is the standard response envelope for tool capability invocations.
// All GoMind tools should wrap their responses in this format for consistent
// error handling across the framework.
//
// Success response:
//
//	ToolResponse{Success: true, Data: weatherData}
//
// Error response:
//
//	ToolResponse{Success: false, Error: &ToolError{...}}
type ToolResponse struct {
	// Success indicates whether the capability invocation succeeded
	Success bool `json:"success"`

	// Data contains the successful response payload (tool-specific)
	// Use interface{} to allow any tool-specific response type
	Data interface{} `json:"data,omitempty"`

	// Error contains structured error information when Success is false
	Error *ToolError `json:"error,omitempty"`
}

// HTTPStatusForCategory returns the appropriate HTTP status code for an error category.
// Tools should use this when returning errors via HTTP to ensure consistency
// across the framework.
//
// Mapping:
//   - CategoryInputError   → 400 Bad Request
//   - CategoryNotFound     → 404 Not Found
//   - CategoryAuthError    → 401 Unauthorized
//   - CategoryRateLimit    → 429 Too Many Requests
//   - CategoryServiceError → 503 Service Unavailable
//   - Unknown              → 500 Internal Server Error
func HTTPStatusForCategory(category ErrorCategory) int {
	switch category {
	case CategoryInputError:
		return http.StatusBadRequest // 400
	case CategoryNotFound:
		return http.StatusNotFound // 404
	case CategoryAuthError:
		return http.StatusUnauthorized // 401
	case CategoryRateLimit:
		return http.StatusTooManyRequests // 429
	case CategoryServiceError:
		return http.StatusServiceUnavailable // 503
	default:
		return http.StatusInternalServerError // 500
	}
}
