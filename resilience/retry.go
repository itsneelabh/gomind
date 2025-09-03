package resilience

import (
	"context"
	"fmt"
	"math"
	"time"
	
	"github.com/itsneelabh/gomind/core"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	JitterEnabled   bool
}

// DefaultRetryConfig provides sensible defaults
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
		JitterEnabled: true,
	}
}

// Retry executes a function with retry logic
func Retry(ctx context.Context, config *RetryConfig, fn func() error) error {
	if config == nil {
		config = DefaultRetryConfig()
	}
	
	var lastErr error
	delay := config.InitialDelay
	
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		// Try the function
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		
		// Don't sleep after the last attempt
		if attempt == config.MaxAttempts {
			break
		}
		
		// Calculate next delay with exponential backoff
		if attempt > 1 {
			delay = time.Duration(float64(delay) * config.BackoffFactor)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		}
		
		// Add jitter if enabled to prevent synchronized retries
		// across multiple clients (thundering herd mitigation)
		if config.JitterEnabled {
			jitter := time.Duration(float64(delay) * 0.1 * math.Sin(float64(attempt)))
			delay += jitter
		}
		
		// Sleep with context cancellation
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	
	return fmt.Errorf("max retry attempts (%d) exceeded for %v: %w", config.MaxAttempts, lastErr, core.ErrMaxRetriesExceeded)
}

// RetryWithCircuitBreaker combines retry logic with circuit breaker
func RetryWithCircuitBreaker(ctx context.Context, config *RetryConfig, cb *CircuitBreaker, fn func() error) error {
	return Retry(ctx, config, func() error {
		if !cb.CanExecute() {
			return core.ErrCircuitBreakerOpen
		}
		
		err := fn()
		if err != nil {
			cb.RecordFailure()
			return err
		}
		
		cb.RecordSuccess()
		return nil
	})
}