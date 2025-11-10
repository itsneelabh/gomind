package providers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// BaseClient provides common functionality for all AI providers
type BaseClient struct {
	// HTTP client with timeout
	HTTPClient *http.Client

	// Logger for debugging
	Logger core.Logger

	// Retry configuration
	MaxRetries int
	RetryDelay time.Duration

	// Default configuration
	DefaultModel        string
	DefaultTemperature  float32
	DefaultMaxTokens    int
	DefaultSystemPrompt string
}

// NewBaseClient creates a new base client with defaults
func NewBaseClient(timeout time.Duration, logger core.Logger) *BaseClient {
	if logger == nil {
		logger = &core.NoOpLogger{}
	}

	return &BaseClient{
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
		Logger:             logger,
		MaxRetries:         3,
		RetryDelay:         time.Second,
		DefaultTemperature: 0.7,
		DefaultMaxTokens:   1000,
	}
}

// ExecuteWithRetry performs an HTTP request with exponential backoff retry
func (b *BaseClient) ExecuteWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= b.MaxRetries; attempt++ {
		if attempt > 0 && b.Logger != nil && lastErr != nil {
			b.Logger.Warn("AI request retry attempt", map[string]interface{}{
				"operation":   "ai_request_retry",
				"attempt":     attempt,
				"max_retries": b.MaxRetries,
				"last_error":  lastErr.Error(),
			})
		}

		// Clone request for retry
		reqClone := req.Clone(ctx)

		// Add context
		reqClone = reqClone.WithContext(ctx)

		// Execute request
		resp, err := b.HTTPClient.Do(reqClone)

		// Success - return if no error and status is not retryable
		if err == nil && resp.StatusCode < 400 {
			if attempt > 0 && b.Logger != nil {
				b.Logger.Info("AI request succeeded after retry", map[string]interface{}{
					"operation":          "ai_request_recovery",
					"successful_attempt": attempt + 1,
					"total_attempts":     attempt + 1,
				})
			}
			return resp, nil
		}

		// Return non-retryable client errors immediately
		if err == nil && resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			if b.Logger != nil {
				b.Logger.Error("AI request failed with non-retryable error", map[string]interface{}{
					"operation":   "ai_request_error",
					"status_code": resp.StatusCode,
					"error_type":  "client_error",
					"retryable":   false,
				})
			}
			return resp, nil
		}

		// Save error for potential return
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("server error: status %d", resp.StatusCode)
			_ = resp.Body.Close() // Error can be safely ignored in error path
		}

		// Check if we should retry
		if attempt < b.MaxRetries {
			// Calculate delay with exponential backoff
			// Ensure safe conversion to uint to prevent overflow
			var shiftAmount uint
			if attempt >= 0 && attempt < 32 {
				shiftAmount = uint(attempt)
			} else {
				shiftAmount = 31 // Cap at max reasonable value
			}
			delay := b.RetryDelay * time.Duration(1<<shiftAmount)

			b.Logger.Warn("AI request failed, retrying", map[string]interface{}{
				"operation":        "ai_request_retry_wait",
				"attempt":          attempt + 1,
				"max_retries":      b.MaxRetries,
				"retry_delay_ms":   delay.Milliseconds(),
				"error":            lastErr.Error(),
				"error_type":       fmt.Sprintf("%T", lastErr),
				"backoff_strategy": "exponential",
			})

			// Wait before retry
			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				b.Logger.Error("AI request cancelled during retry", map[string]interface{}{
					"operation":      "ai_request_cancelled",
					"cancelled_at":   attempt + 1,
					"context_error":  ctx.Err().Error(),
				})
				return nil, ctx.Err()
			}
		}
	}

	b.Logger.Error("AI request failed after all retries", map[string]interface{}{
		"operation":       "ai_request_final_failure",
		"total_attempts":  b.MaxRetries + 1,
		"final_error":     lastErr.Error(),
		"error_type":      fmt.Sprintf("%T", lastErr),
	})

	return nil, fmt.Errorf("request failed after %d retries: %w", b.MaxRetries, lastErr)
}

// LogError logs an error with provider context
func (b *BaseClient) LogError(provider string, err error) {
	b.Logger.Error("Provider error", map[string]interface{}{
		"provider": provider,
		"error":    err.Error(),
	})
}

// ApplyDefaults applies default values to options if not set
func (b *BaseClient) ApplyDefaults(options *core.AIOptions) *core.AIOptions {
	if options == nil {
		options = &core.AIOptions{}
	}

	// Apply defaults for unset values
	if options.Model == "" && b.DefaultModel != "" {
		options.Model = b.DefaultModel
	}

	if options.Temperature == 0 {
		options.Temperature = b.DefaultTemperature
	}

	if options.MaxTokens == 0 {
		options.MaxTokens = b.DefaultMaxTokens
	}

	if options.SystemPrompt == "" && b.DefaultSystemPrompt != "" {
		options.SystemPrompt = b.DefaultSystemPrompt
	}

	return options
}

// isRetryableError determines if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Check for specific HTTP status codes that are retryable
	if strings.Contains(errStr, "(429)") || // Rate limit
		strings.Contains(errStr, "(500)") || // Internal server error
		strings.Contains(errStr, "(502)") || // Bad gateway
		strings.Contains(errStr, "(503)") || // Service unavailable
		strings.Contains(errStr, "(504)") { // Gateway timeout
		return true
	}

	// Check for context timeout/deadline
	if err == context.DeadlineExceeded {
		return true
	}

	return false
}

// HandleError processes API errors consistently
func (b *BaseClient) HandleError(statusCode int, body []byte, provider string) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("%s API error: invalid or missing API key", provider)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%s API error: rate limit exceeded", provider)
	case http.StatusBadRequest:
		return fmt.Errorf("%s API error: invalid request - %s", provider, string(body))
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return fmt.Errorf("%s API error: service temporarily unavailable (status %d)", provider, statusCode)
	default:
		return fmt.Errorf("%s API error (status %d): %s", provider, statusCode, string(body))
	}
}

// LogRequest logs outgoing API requests
func (b *BaseClient) LogRequest(provider, model, prompt string) {
	b.Logger.Info("AI request initiated", map[string]interface{}{
		"operation":     "ai_request",
		"provider":      provider,
		"model":         model,
		"prompt_length": len(prompt),
		"max_tokens":    b.DefaultMaxTokens,
		"temperature":   b.DefaultTemperature,
	})
}

// LogResponse logs API responses
func (b *BaseClient) LogResponse(provider, model string, tokens core.TokenUsage, duration time.Duration) {
	b.Logger.Info("AI response received", map[string]interface{}{
		"operation":         "ai_response",
		"provider":          provider,
		"model":             model,
		"prompt_tokens":     tokens.PromptTokens,
		"completion_tokens": tokens.CompletionTokens,
		"total_tokens":      tokens.TotalTokens,
		"duration_ms":       duration.Milliseconds(),
		"tokens_per_second": float64(tokens.TotalTokens) / duration.Seconds(),
		"status":            "success",
	})
}

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxRetries int
	RetryDelay time.Duration
	// Optional: custom retry predicate
	ShouldRetry func(resp *http.Response, err error) bool
}

// DefaultRetryConfig returns sensible retry defaults
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		RetryDelay: time.Second,
		ShouldRetry: func(resp *http.Response, err error) bool {
			// Retry on network errors
			if err != nil {
				return true
			}
			// Retry on 5xx errors
			if resp != nil && resp.StatusCode >= 500 {
				return true
			}
			// Retry on rate limit (with backoff)
			if resp != nil && resp.StatusCode == 429 {
				return true
			}
			return false
		},
	}
}
