package resilience

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// CircuitState represents the state of the circuit breaker
type CircuitState int

const (
	// StateClosed allows all requests through
	StateClosed CircuitState = iota
	// StateOpen blocks all requests
	StateOpen
	// StateHalfOpen allows limited requests for testing
	StateHalfOpen
)

// String returns the string representation of the state
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Logger interface for circuit breaker logging
type Logger interface {
	Error(msg string, fields map[string]interface{})
	Info(msg string, fields map[string]interface{})
	Debug(msg string, fields map[string]interface{})
}

// noopLogger is a no-op logger implementation
type noopLogger struct{}

func (n *noopLogger) Error(msg string, fields map[string]interface{}) {}
func (n *noopLogger) Info(msg string, fields map[string]interface{})  {}
func (n *noopLogger) Debug(msg string, fields map[string]interface{}) {}

// MetricsCollector interface for circuit breaker metrics
type MetricsCollector interface {
	RecordSuccess(name string)
	RecordFailure(name string, errorType string)
	RecordStateChange(name string, from, to string)
	RecordRejection(name string)
}

// noopMetrics is a no-op metrics implementation
type noopMetrics struct{}

func (n *noopMetrics) RecordSuccess(name string)                      {}
func (n *noopMetrics) RecordFailure(name string, errorType string)    {}
func (n *noopMetrics) RecordStateChange(name string, from, to string) {}
func (n *noopMetrics) RecordRejection(name string)                    {}

// ErrorClassifier determines which errors should count toward circuit breaker thresholds
type ErrorClassifier func(error) bool

// DefaultErrorClassifier only counts infrastructure errors, not user errors
func DefaultErrorClassifier(err error) bool {
	if err == nil {
		return false
	}

	// Configuration errors - DON'T count (user error)
	if core.IsConfigurationError(err) {
		return false
	}

	// Not found errors - DON'T count (user error)
	if core.IsNotFound(err) {
		return false
	}

	// State errors - DON'T count (programming error)
	if core.IsStateError(err) {
		return false
	}

	// Context cancellation - DON'T count (client gave up)
	if errors.Is(err, context.Canceled) || errors.Is(err, core.ErrContextCanceled) {
		return false
	}

	// All other errors count as failures (network, timeout, connection issues)
	return true
}

// CircuitBreakerConfig holds configuration for the circuit breaker
type CircuitBreakerConfig struct {
	// Name identifies the circuit breaker
	Name string

	// FailureThreshold is the number of failures before opening (deprecated, use ErrorThreshold)
	FailureThreshold int

	// RecoveryTimeout is how long to wait before attempting recovery (deprecated, use SleepWindow)
	RecoveryTimeout time.Duration

	// ErrorThreshold is the error rate (0.0 to 1.0) that triggers opening
	ErrorThreshold float64

	// VolumeThreshold is the minimum number of requests before evaluation
	VolumeThreshold int

	// SleepWindow is how long to wait before entering half-open state
	SleepWindow time.Duration

	// HalfOpenRequests is the number of test requests in half-open state
	HalfOpenRequests int

	// SuccessThreshold is the success rate needed to close from half-open
	SuccessThreshold float64

	// WindowSize is the sliding window duration for metrics
	WindowSize time.Duration

	// BucketCount is the number of buckets in the sliding window
	BucketCount int

	// ErrorClassifier determines which errors count as failures
	ErrorClassifier ErrorClassifier

	// Logger for circuit breaker events
	Logger Logger

	// Metrics collector for monitoring
	Metrics MetricsCollector
}

// DefaultConfig returns a production-ready default configuration
func DefaultConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Name:             "default",
		ErrorThreshold:   0.5, // 50% error rate
		VolumeThreshold:  10,  // Need 10 requests minimum
		SleepWindow:      30 * time.Second,
		HalfOpenRequests: 5,
		SuccessThreshold: 0.6, // 60% success to recover
		WindowSize:       60 * time.Second,
		BucketCount:      10,
		ErrorClassifier:  DefaultErrorClassifier,
		Logger:           &noopLogger{},
		Metrics:          &noopMetrics{},
	}
}

// ExecutionToken tracks in-flight requests to prevent orphaned executions
type ExecutionToken struct {
	id         uint64
	startTime  time.Time
	isHalfOpen bool
}

// CircuitBreaker addresses all production issues identified in review
type CircuitBreaker struct {
	// Configuration
	config *CircuitBreakerConfig

	// State management (using atomic for frequently accessed state)
	state          atomic.Value // CircuitState
	stateChangedAt atomic.Value // time.Time
	generation     uint64

	// Metrics tracking
	window *SlidingWindow

	// Half-open state management with atomic operations
	halfOpenCount     atomic.Int32
	halfOpenTotal     atomic.Int32 // Total requests allowed in current half-open period
	halfOpenSuccesses atomic.Int32
	halfOpenFailures  atomic.Int32
	halfOpenTokens    sync.Map // map[uint64]ExecutionToken for tracking in-flight requests
	tokenCounter      atomic.Uint64

	// Manual control
	forceOpen   atomic.Bool
	forceClosed atomic.Bool

	// Legacy compatibility
	failureCount atomic.Int32

	// Error type cache to avoid allocations
	errorTypeCache sync.Map // map[error]string

	// State change listeners
	listeners []func(name string, from, to CircuitState)

	// Reduced lock contention - only for state transitions
	mu sync.Mutex

	// Metrics for monitoring
	executionsInFlight atomic.Int32
	totalExecutions    atomic.Uint64
	rejectedExecutions atomic.Uint64
}

// NewCircuitBreaker creates a production-ready circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig) (*CircuitBreaker, error) {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid circuit breaker config: %w", err)
	}

	if config == nil {
		config = DefaultConfig()
	}

	// Apply defaults for missing values
	if config.WindowSize == 0 {
		config.WindowSize = 60 * time.Second
	}
	if config.BucketCount == 0 {
		config.BucketCount = 10
	}
	if config.ErrorClassifier == nil {
		config.ErrorClassifier = DefaultErrorClassifier
	}
	if config.Logger == nil {
		config.Logger = &noopLogger{}
	}
	if config.Metrics == nil {
		config.Metrics = &noopMetrics{}
	}
	if config.SuccessThreshold == 0 {
		config.SuccessThreshold = 0.6
	}
	if config.HalfOpenRequests == 0 {
		config.HalfOpenRequests = 5
	}

	cb := &CircuitBreaker{
		config:     config,
		generation: 0,
		window:     NewSlidingWindow(config.WindowSize, config.BucketCount, true),
		listeners:  make([]func(string, CircuitState, CircuitState), 0),
	}

	// Initialize atomic values
	cb.state.Store(StateClosed)
	cb.stateChangedAt.Store(time.Now())

	return cb, nil
}

// Execute runs the given function with circuit breaker protection and proper timeout handling
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	return cb.ExecuteWithTimeout(ctx, 0, fn)
}

// ExecuteWithTimeout runs the function with timeout protection
func (cb *CircuitBreaker) ExecuteWithTimeout(ctx context.Context, timeout time.Duration, fn func() error) error {
	// Check if we can execute
	token, allowed := cb.startExecution()
	if !allowed {
		cb.rejectedExecutions.Add(1)
		cb.config.Metrics.RecordRejection(cb.config.Name)
		return fmt.Errorf("circuit breaker '%s' is open: %w", cb.config.Name, core.ErrCircuitBreakerOpen)
	}

	// Track in-flight execution
	cb.executionsInFlight.Add(1)
	defer cb.executionsInFlight.Add(-1)
	cb.totalExecutions.Add(1)

	// Setup timeout if specified
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Execute the function in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	// Wait for either completion or timeout
	select {
	case err := <-done:
		// Normal completion
		cb.completeExecution(token, err)
		return err
	case <-ctx.Done():
		// Context cancelled or timeout
		// The function is still running - this creates an orphaned request
		// It will be cleaned up when it eventually completes
		go func() {
			<-done // Wait for function to complete
			cb.completeExecution(token, ctx.Err())
		}()
		return ctx.Err()
	}
}

// startExecution attempts to start an execution and returns a token if allowed
func (cb *CircuitBreaker) startExecution() (ExecutionToken, bool) {
	// Check manual overrides first (atomic read)
	if cb.forceClosed.Load() {
		return ExecutionToken{}, true
	}
	if cb.forceOpen.Load() {
		return ExecutionToken{}, false
	}

	currentState := cb.state.Load().(CircuitState)

	switch currentState {
	case StateClosed:
		// Always allow in closed state
		return ExecutionToken{
			id:         cb.tokenCounter.Add(1),
			startTime:  time.Now(),
			isHalfOpen: false,
		}, true

	case StateOpen:
		// Check if we should transition to half-open
		stateChangedAt := cb.stateChangedAt.Load().(time.Time)
		if time.Since(stateChangedAt) > cb.config.SleepWindow {
			// Try to transition to half-open
			cb.mu.Lock()
			// Double-check state after acquiring lock
			if cb.state.Load().(CircuitState) == StateOpen {
				cb.transitionToUnlocked(StateHalfOpen)
			}
			cb.mu.Unlock()

			// Retry after transition
			return cb.startExecution()
		}
		return ExecutionToken{}, false

	case StateHalfOpen:
		// Atomically check and increment total requests
		for {
			current := cb.halfOpenTotal.Load()
			if cb.config.HalfOpenRequests > 0 && int(current) >= cb.config.HalfOpenRequests {
				// Already at limit
				return ExecutionToken{}, false
			}
			// Try to increment
			if cb.halfOpenTotal.CompareAndSwap(current, current+1) {
				// Successfully reserved a slot
				break
			}
			// Someone else modified it, retry
		}

		// Also track concurrent requests
		cb.halfOpenCount.Add(1)

		token := ExecutionToken{
			id:         cb.tokenCounter.Add(1),
			startTime:  time.Now(),
			isHalfOpen: true,
		}

		// Track this token to prevent orphaned requests
		cb.halfOpenTokens.Store(token.id, token)

		return token, true

	default:
		return ExecutionToken{}, false
	}
}

// completeExecution records the result of an execution
func (cb *CircuitBreaker) completeExecution(token ExecutionToken, err error) {
	// Skip if manually controlled
	if cb.forceClosed.Load() || cb.forceOpen.Load() {
		return
	}

	// Clean up half-open token if applicable
	if token.isHalfOpen {
		cb.halfOpenTokens.Delete(token.id)
		cb.halfOpenCount.Add(-1) // Decrement counter when request completes
	}

	// Record in sliding window
	if err == nil {
		cb.window.RecordSuccess()
		cb.config.Metrics.RecordSuccess(cb.config.Name)

		if token.isHalfOpen {
			cb.halfOpenSuccesses.Add(1)
		}
	} else if cb.config.ErrorClassifier(err) {
		cb.window.RecordFailure()

		// Cache error type to avoid allocation
		errorType := cb.getErrorType(err)
		cb.config.Metrics.RecordFailure(cb.config.Name, errorType)

		// Legacy failure counting
		cb.failureCount.Add(1)

		if token.isHalfOpen {
			cb.halfOpenFailures.Add(1)
		}
	}

	// Evaluate state
	cb.evaluateState()
}

// getErrorType returns cached error type string to avoid allocations
func (cb *CircuitBreaker) getErrorType(err error) string {
	if cached, ok := cb.errorTypeCache.Load(err); ok {
		return cached.(string)
	}

	// Use type assertion for common errors to avoid fmt.Sprintf
	switch err.(type) {
	case *core.FrameworkError:
		return "*core.FrameworkError"
	default:
		// Check for well-known errors
		if errors.Is(err, context.DeadlineExceeded) {
			return "DeadlineExceeded"
		}
		if errors.Is(err, context.Canceled) {
			return "Canceled"
		}
		// Only allocate for unknown error types
		errorType := fmt.Sprintf("%T", err)
		cb.errorTypeCache.Store(err, errorType)
		return errorType
	}
}

// evaluateState checks if state transition is needed
func (cb *CircuitBreaker) evaluateState() {
	currentState := cb.state.Load().(CircuitState)

	switch currentState {
	case StateClosed:
		// Check if we should open
		errorRate := cb.window.GetErrorRate()
		total := cb.window.GetTotal()

		// Use legacy threshold if set
		if cb.config.FailureThreshold > 0 && int(cb.failureCount.Load()) >= cb.config.FailureThreshold {
			cb.mu.Lock()
			cb.transitionToUnlocked(StateOpen)
			cb.mu.Unlock()
			return
		}

		// Use error rate threshold
		// Safe comparison: VolumeThreshold is int, total is uint64
		// #nosec G115 - VolumeThreshold is checked to be > 0 before conversion
		if cb.config.VolumeThreshold > 0 && total >= uint64(cb.config.VolumeThreshold) && errorRate >= cb.config.ErrorThreshold {
			cb.mu.Lock()
			cb.transitionToUnlocked(StateOpen)
			cb.mu.Unlock()
		}

	case StateHalfOpen:
		// Check if we have enough test requests to make a decision
		successes := cb.halfOpenSuccesses.Load()
		failures := cb.halfOpenFailures.Load()
		totalHalfOpen := successes + failures

		if cb.config.HalfOpenRequests > 0 && int(totalHalfOpen) >= cb.config.HalfOpenRequests {
			successRate := float64(successes) / float64(totalHalfOpen)

			cb.mu.Lock()
			if successRate >= cb.config.SuccessThreshold {
				// Enough successes, close the circuit
				cb.transitionToUnlocked(StateClosed)
				cb.failureCount.Store(0) // Reset legacy counter
			} else {
				// Too many failures, reopen
				cb.transitionToUnlocked(StateOpen)
				// Exponential backoff for next attempt
				cb.config.SleepWindow = time.Duration(float64(cb.config.SleepWindow) * 1.5)
				if cb.config.SleepWindow > 5*time.Minute {
					cb.config.SleepWindow = 5 * time.Minute // Cap at 5 minutes
				}
			}
			cb.mu.Unlock()
		}
	}
}

// transitionToUnlocked changes state (must be called with lock held)
func (cb *CircuitBreaker) transitionToUnlocked(newState CircuitState) {
	oldState := cb.state.Load().(CircuitState)
	if oldState == newState {
		return
	}

	cb.state.Store(newState)
	cb.stateChangedAt.Store(time.Now())
	cb.generation++

	// Reset half-open counters when entering half-open
	if newState == StateHalfOpen {
		cb.halfOpenCount.Store(0)
		cb.halfOpenTotal.Store(0) // Reset total count for new half-open period
		cb.halfOpenSuccesses.Store(0)
		cb.halfOpenFailures.Store(0)
		// Clear any orphaned tokens
		cb.halfOpenTokens.Range(func(key, value interface{}) bool {
			cb.halfOpenTokens.Delete(key)
			return true
		})
	}

	// Log state change
	cb.config.Logger.Info("Circuit breaker state changed", map[string]interface{}{
		"name":       cb.config.Name,
		"from":       oldState.String(),
		"to":         newState.String(),
		"error_rate": cb.window.GetErrorRate(),
	})

	// Record metrics
	cb.config.Metrics.RecordStateChange(cb.config.Name, oldState.String(), newState.String())

	// Notify listeners
	for _, listener := range cb.listeners {
		go listener(cb.config.Name, oldState, newState)
	}
}

// AddStateChangeListener adds a listener for state changes
func (cb *CircuitBreaker) AddStateChangeListener(listener func(name string, from, to CircuitState)) {
	cb.mu.Lock()
	cb.listeners = append(cb.listeners, listener)
	cb.mu.Unlock()
}

// GetState returns the current state
func (cb *CircuitBreaker) GetState() string {
	return cb.state.Load().(CircuitState).String()
}

// GetMetrics returns current metrics
func (cb *CircuitBreaker) GetMetrics() map[string]interface{} {
	success, failure := cb.window.GetCounts()
	total := success + failure

	metrics := map[string]interface{}{
		"name":                 cb.config.Name,
		"state":                cb.GetState(),
		"generation":           cb.generation,
		"success":              success,
		"failure":              failure,
		"total":                total,
		"error_rate":           cb.window.GetErrorRate(),
		"force_open":           cb.forceOpen.Load(),
		"force_closed":         cb.forceClosed.Load(),
		"executions_in_flight": cb.executionsInFlight.Load(),
		"total_executions":     cb.totalExecutions.Load(),
		"rejected_executions":  cb.rejectedExecutions.Load(),
	}

	currentState := cb.state.Load().(CircuitState)
	if currentState == StateHalfOpen {
		metrics["half_open_count"] = cb.halfOpenCount.Load()
		metrics["half_open_successes"] = cb.halfOpenSuccesses.Load()
		metrics["half_open_failures"] = cb.halfOpenFailures.Load()

		// Count orphaned tokens
		orphaned := 0
		now := time.Now()
		cb.halfOpenTokens.Range(func(key, value interface{}) bool {
			token := value.(ExecutionToken)
			if now.Sub(token.startTime) > 30*time.Second {
				orphaned++
			}
			return true
		})
		metrics["orphaned_requests"] = orphaned
	}

	return metrics
}

// Reset resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state.Store(StateClosed)
	cb.stateChangedAt.Store(time.Now())
	cb.failureCount.Store(0)
	cb.halfOpenCount.Store(0)
	cb.halfOpenSuccesses.Store(0)
	cb.halfOpenFailures.Store(0)
	cb.window = NewSlidingWindow(cb.config.WindowSize, cb.config.BucketCount, true)

	// Clear tokens
	cb.halfOpenTokens.Range(func(key, value interface{}) bool {
		cb.halfOpenTokens.Delete(key)
		return true
	})

	cb.config.Logger.Info("Circuit breaker reset", map[string]interface{}{
		"name": cb.config.Name,
	})
}

// ForceOpen manually opens the circuit
func (cb *CircuitBreaker) ForceOpen() {
	cb.forceOpen.Store(true)
	cb.forceClosed.Store(false)

	cb.mu.Lock()
	if cb.state.Load().(CircuitState) != StateOpen {
		cb.transitionToUnlocked(StateOpen)
	}
	cb.mu.Unlock()
}

// ForceClosed manually closes the circuit
func (cb *CircuitBreaker) ForceClosed() {
	cb.forceClosed.Store(true)
	cb.forceOpen.Store(false)

	cb.mu.Lock()
	if cb.state.Load().(CircuitState) != StateClosed {
		cb.transitionToUnlocked(StateClosed)
	}
	cb.mu.Unlock()
}

// ClearForce removes manual override
func (cb *CircuitBreaker) ClearForce() {
	cb.forceOpen.Store(false)
	cb.forceClosed.Store(false)
}

// CleanupOrphanedRequests cleans up requests that have been in-flight too long
func (cb *CircuitBreaker) CleanupOrphanedRequests(maxAge time.Duration) int {
	cleaned := 0
	now := time.Now()

	cb.halfOpenTokens.Range(func(key, value interface{}) bool {
		token := value.(ExecutionToken)
		if now.Sub(token.startTime) > maxAge {
			cb.halfOpenTokens.Delete(key)
			// Record as failure
			cb.completeExecution(token, errors.New("request orphaned"))
			cleaned++
		}
		return true
	})

	return cleaned
}

// Validate validates the circuit breaker configuration
func (c *CircuitBreakerConfig) Validate() error {
	if c == nil {
		return errors.New("configuration cannot be nil")
	}

	if c.Name == "" {
		return errors.New("circuit breaker name is required")
	}

	if c.ErrorThreshold < 0 || c.ErrorThreshold > 1 {
		return fmt.Errorf("error threshold must be between 0 and 1, got %f", c.ErrorThreshold)
	}

	if c.VolumeThreshold < 0 {
		return fmt.Errorf("volume threshold must be non-negative, got %d", c.VolumeThreshold)
	}

	if c.SuccessThreshold < 0 || c.SuccessThreshold > 1 {
		return fmt.Errorf("success threshold must be between 0 and 1, got %f", c.SuccessThreshold)
	}

	if c.HalfOpenRequests < 1 {
		return fmt.Errorf("half-open requests must be at least 1, got %d", c.HalfOpenRequests)
	}

	if c.SleepWindow < 0 {
		return fmt.Errorf("sleep window must be non-negative, got %v", c.SleepWindow)
	}

	if c.WindowSize < 0 {
		return fmt.Errorf("window size must be non-negative, got %v", c.WindowSize)
	}

	if c.BucketCount < 1 {
		return fmt.Errorf("bucket count must be at least 1, got %d", c.BucketCount)
	}

	return nil
}

// bucket represents a time bucket in the sliding window
type bucket struct {
	timestamp time.Time
	success   uint64
	failure   uint64
}

// SlidingWindow with time skew protection
type SlidingWindow struct {
	buckets      []bucket
	windowSize   time.Duration
	bucketSize   time.Duration
	currentIdx   int
	lastRotation time.Time
	mu           sync.RWMutex
	monotonic    bool // Use monotonic time to avoid skew
}

// NewSlidingWindow creates a sliding window with time skew protection
func NewSlidingWindow(windowSize time.Duration, bucketCount int, monotonic bool) *SlidingWindow {
	if bucketCount <= 0 {
		bucketCount = 10
	}

	bucketSize := windowSize / time.Duration(bucketCount)
	buckets := make([]bucket, bucketCount)
	now := time.Now()

	for i := range buckets {
		buckets[i].timestamp = now
	}

	return &SlidingWindow{
		buckets:      buckets,
		windowSize:   windowSize,
		bucketSize:   bucketSize,
		lastRotation: now,
		monotonic:    monotonic,
	}
}

// rotateBuckets with time skew protection
func (sw *SlidingWindow) rotateBuckets() {
	now := time.Now()

	var elapsed time.Duration
	if sw.monotonic {
		// Use monotonic time (won't jump backward)
		elapsed = now.Sub(sw.lastRotation)
	} else {
		elapsed = now.Sub(sw.buckets[sw.currentIdx].timestamp)
	}

	// Handle time skew - if time went backward, reset
	if elapsed < 0 {
		sw.reset()
		return
	}

	// Normal rotation
	if elapsed >= sw.bucketSize {
		bucketsToRotate := int(elapsed / sw.bucketSize)
		if bucketsToRotate > len(sw.buckets) {
			bucketsToRotate = len(sw.buckets)
		}

		for i := 0; i < bucketsToRotate; i++ {
			sw.currentIdx = (sw.currentIdx + 1) % len(sw.buckets)
			sw.buckets[sw.currentIdx] = bucket{
				timestamp: now,
				success:   0,
				failure:   0,
			}
		}

		sw.lastRotation = now
	}
}

// reset clears all buckets (used when time skew detected)
func (sw *SlidingWindow) reset() {
	now := time.Now()
	for i := range sw.buckets {
		sw.buckets[i] = bucket{
			timestamp: now,
			success:   0,
			failure:   0,
		}
	}
	sw.currentIdx = 0
	sw.lastRotation = now
}

// RecordSuccess records a successful operation
func (sw *SlidingWindow) RecordSuccess() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.rotateBuckets()
	atomic.AddUint64(&sw.buckets[sw.currentIdx].success, 1)
}

// RecordFailure records a failed operation
func (sw *SlidingWindow) RecordFailure() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.rotateBuckets()
	atomic.AddUint64(&sw.buckets[sw.currentIdx].failure, 1)
}

// GetCounts returns success and failure counts
func (sw *SlidingWindow) GetCounts() (success, failure uint64) {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	cutoff := time.Now().Add(-sw.windowSize)

	for i := range sw.buckets {
		b := &sw.buckets[i]
		if b.timestamp.After(cutoff) {
			success += atomic.LoadUint64(&b.success)
			failure += atomic.LoadUint64(&b.failure)
		}
	}

	return success, failure
}

// GetErrorRate returns the current error rate
func (sw *SlidingWindow) GetErrorRate() float64 {
	success, failure := sw.GetCounts()
	total := success + failure
	if total == 0 {
		return 0
	}
	return float64(failure) / float64(total)
}

// GetTotal returns the total number of requests
func (sw *SlidingWindow) GetTotal() uint64 {
	success, failure := sw.GetCounts()
	return success + failure
}

// CanExecute checks if the circuit breaker allows execution (backward compatibility)
func (cb *CircuitBreaker) CanExecute() bool {
	state := cb.state.Load().(CircuitState)
	if state == StateClosed {
		return true
	}
	if state == StateOpen {
		// Check if we should transition to half-open
		stateChangedAt := cb.stateChangedAt.Load().(time.Time)
		if time.Since(stateChangedAt) > cb.config.SleepWindow {
			// Transition to half-open
			cb.mu.Lock()
			if cb.state.Load().(CircuitState) == StateOpen {
				cb.transitionToUnlocked(StateHalfOpen)
			}
			cb.mu.Unlock()
			return true
		}
		return false
	}
	// Half-open - check if we have capacity
	return cb.config.HalfOpenRequests > 0 && int(cb.halfOpenTotal.Load()) < cb.config.HalfOpenRequests
}

// RecordSuccess records a successful operation (backward compatibility)
func (cb *CircuitBreaker) RecordSuccess() {
	cb.window.RecordSuccess()
	cb.evaluateState()
}

// RecordFailure records a failed operation (backward compatibility)
func (cb *CircuitBreaker) RecordFailure() {
	cb.window.RecordFailure()
	cb.failureCount.Add(1)
	cb.evaluateState()
}

// NewCircuitBreakerWithConfig creates a circuit breaker with config (backward compatibility)
func NewCircuitBreakerWithConfig(config *CircuitBreakerConfig) *CircuitBreaker {
	cb, _ := NewCircuitBreaker(config)
	return cb
}

// NewCircuitBreakerLegacy creates a circuit breaker with legacy parameters (backward compatibility)
func NewCircuitBreakerLegacy(failureThreshold int, recoveryTimeout time.Duration) *CircuitBreaker {
	config := &CircuitBreakerConfig{
		Name:             "legacy",
		FailureThreshold: failureThreshold,
		RecoveryTimeout:  recoveryTimeout,
		SleepWindow:      recoveryTimeout,
		ErrorThreshold:   0.5,
		VolumeThreshold:  1,   // Set to 1 for backward compatibility with old behavior
		HalfOpenRequests: 1,   // Only allow 1 test request in half-open for simpler behavior
		SuccessThreshold: 0.5, // Lower threshold for easier recovery
		WindowSize:       60 * time.Second,
		BucketCount:      10,
		ErrorClassifier:  DefaultErrorClassifier,
		Logger:           &noopLogger{},
		Metrics:          &noopMetrics{},
	}
	cb, _ := NewCircuitBreaker(config)
	return cb
}
