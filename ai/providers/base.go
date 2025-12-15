package providers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
)

// BaseClient provides common functionality for all AI providers
type BaseClient struct {
	// HTTP client with timeout
	HTTPClient *http.Client

	// Logger for debugging
	Logger core.Logger

	// Telemetry for distributed tracing
	Telemetry core.Telemetry

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
		Telemetry:          nil, // Set via SetTelemetry or factory
		MaxRetries:         3,
		RetryDelay:         time.Second,
		DefaultTemperature: 0.7,
		DefaultMaxTokens:   1000,
	}
}

// SetTelemetry sets the telemetry provider for distributed tracing
func (b *BaseClient) SetTelemetry(t core.Telemetry) {
	b.Telemetry = t
}

// SetLogger updates the logger after client creation.
// This is called by Framework.applyConfigToComponent() to propagate
// the real logger to the AI client after framework initialization.
// When the logger implements ComponentAwareLogger, it creates a
// component-specific logger with "framework/ai" prefix for filtering.
func (b *BaseClient) SetLogger(logger core.Logger) {
	if logger == nil {
		b.Logger = &core.NoOpLogger{}
	} else if cal, ok := logger.(core.ComponentAwareLogger); ok {
		b.Logger = cal.WithComponent("framework/ai")
	} else {
		b.Logger = logger
	}
}

// StartSpan starts a new span for AI operations if telemetry is configured.
// Returns the updated context and a span. If telemetry is nil, returns a no-op span.
// Caller is responsible for calling span.End() when the operation completes.
func (b *BaseClient) StartSpan(ctx context.Context, name string) (context.Context, core.Span) {
	if b.Telemetry != nil {
		return b.Telemetry.StartSpan(ctx, name)
	}
	return ctx, &core.NoOpSpan{}
}

// ExecuteWithRetry performs an HTTP request with exponential backoff retry.
// Each retry attempt creates a child span visible in Jaeger for debugging.
func (b *BaseClient) ExecuteWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= b.MaxRetries; attempt++ {
		// Create a span for each attempt (visible in Jaeger as child spans)
		attemptCtx, attemptSpan := b.StartSpan(ctx, "ai.http_attempt")
		attemptSpan.SetAttribute("ai.attempt", attempt+1)
		attemptSpan.SetAttribute("ai.max_retries", b.MaxRetries)
		attemptSpan.SetAttribute("ai.is_retry", attempt > 0)

		if attempt > 0 && b.Logger != nil && lastErr != nil {
			// Record retry metric (no attempt label to avoid high cardinality)
			telemetry.Counter("ai.request.retries",
				"module", telemetry.ModuleAI,
			)

			attemptSpan.SetAttribute("ai.previous_error", lastErr.Error())

			b.Logger.WarnWithContext(attemptCtx, "AI request retry attempt", map[string]interface{}{
				"operation":   "ai_request_retry",
				"attempt":     attempt,
				"max_retries": b.MaxRetries,
				"last_error":  lastErr.Error(),
			})
		}

		// Clone request for retry
		reqClone := req.Clone(attemptCtx)

		// Execute request
		attemptStart := time.Now()
		resp, err := b.HTTPClient.Do(reqClone)
		attemptDuration := time.Since(attemptStart)

		attemptSpan.SetAttribute("ai.attempt_duration_ms", attemptDuration.Milliseconds())

		// Success - return if no error and status is not retryable
		if err == nil && resp.StatusCode < 400 {
			attemptSpan.SetAttribute("ai.attempt_status", "success")
			attemptSpan.SetAttribute("http.status_code", resp.StatusCode)
			attemptSpan.End()

			if b.Logger != nil {
				if attempt > 0 {
					// Retry recovery - log at INFO level
					b.Logger.InfoWithContext(ctx, "AI request succeeded after retry", map[string]interface{}{
						"operation":          "ai_request_recovery",
						"successful_attempt": attempt + 1,
						"total_attempts":     attempt + 1,
					})
				} else {
					// First attempt success - log at DEBUG level
					b.Logger.DebugWithContext(ctx, "AI HTTP request completed", map[string]interface{}{
						"operation":   "ai_http_success",
						"status_code": resp.StatusCode,
						"duration_ms": attemptDuration.Milliseconds(),
					})
				}
			}
			return resp, nil
		}

		// Return non-retryable client errors immediately
		if err == nil && resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			attemptSpan.SetAttribute("ai.attempt_status", "client_error")
			attemptSpan.SetAttribute("http.status_code", resp.StatusCode)
			attemptSpan.SetAttribute("ai.retryable", false)
			attemptSpan.End()

			if b.Logger != nil {
				b.Logger.ErrorWithContext(ctx, "AI request failed with non-retryable error", map[string]interface{}{
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
			attemptSpan.RecordError(err)
			attemptSpan.SetAttribute("ai.attempt_status", "network_error")
		} else {
			lastErr = fmt.Errorf("server error: status %d", resp.StatusCode)
			attemptSpan.RecordError(lastErr)
			attemptSpan.SetAttribute("ai.attempt_status", "server_error")
			attemptSpan.SetAttribute("http.status_code", resp.StatusCode)
			_ = resp.Body.Close() // Error can be safely ignored in error path
		}

		attemptSpan.SetAttribute("ai.retryable", true)
		attemptSpan.End()

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

			if b.Logger != nil {
				b.Logger.WarnWithContext(ctx, "AI request failed, retrying", map[string]interface{}{
					"operation":        "ai_request_retry_wait",
					"attempt":          attempt + 1,
					"max_retries":      b.MaxRetries,
					"retry_delay_ms":   delay.Milliseconds(),
					"error":            lastErr.Error(),
					"error_type":       fmt.Sprintf("%T", lastErr),
					"backoff_strategy": "exponential",
				})
			}

			// Wait before retry
			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				if b.Logger != nil {
					b.Logger.ErrorWithContext(ctx, "AI request cancelled during retry", map[string]interface{}{
						"operation":     "ai_request_cancelled",
						"cancelled_at":  attempt + 1,
						"context_error": ctx.Err().Error(),
					})
				}
				return nil, ctx.Err()
			}
		}
	}

	// Record final failure metric
	telemetry.Counter("ai.request.failures",
		"module", telemetry.ModuleAI,
		"reason", "exhausted_retries",
	)

	if b.Logger != nil {
		b.Logger.ErrorWithContext(ctx, "AI request failed after all retries", map[string]interface{}{
			"operation":      "ai_request_final_failure",
			"total_attempts": b.MaxRetries + 1,
			"final_error":    lastErr.Error(),
			"error_type":     fmt.Sprintf("%T", lastErr),
		})
	}

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

	// Log full prompt content at DEBUG level for troubleshooting
	b.Logger.Debug("AI request prompt content", map[string]interface{}{
		"operation": "ai_request_content",
		"provider":  provider,
		"model":     model,
		"prompt":    prompt,
	})
}

// LogResponse logs API responses
func (b *BaseClient) LogResponse(provider, model string, tokens core.TokenUsage, duration time.Duration) {
	// Record AI request metrics using unified telemetry
	telemetry.RecordAIRequest(telemetry.ModuleAI, provider,
		float64(duration.Milliseconds()), "success")

	// Record token usage
	if tokens.PromptTokens > 0 {
		telemetry.RecordAITokens(telemetry.ModuleAI, provider, "input", int64(tokens.PromptTokens))
	}
	if tokens.CompletionTokens > 0 {
		telemetry.RecordAITokens(telemetry.ModuleAI, provider, "output", int64(tokens.CompletionTokens))
	}

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

// LogResponseContent logs the full response content at DEBUG level
func (b *BaseClient) LogResponseContent(provider, model, content string) {
	b.Logger.Debug("AI response content", map[string]interface{}{
		"operation":        "ai_response_content",
		"provider":         provider,
		"model":            model,
		"response":         content,
		"response_length":  len(content),
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
