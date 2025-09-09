//go:build security
// +build security

// Package security provides in-memory rate limiting for development and testing.
// This file implements the InMemoryRateLimiter as a fallback when Redis is unavailable
// or for single-instance deployments where distributed rate limiting is not required.
//
// Purpose:
// - Provides rate limiting without external dependencies
// - Serves as fallback when Redis is unavailable
// - Enables rate limiting in development and testing environments
// - Supports single-instance deployments efficiently
//
// Scope:
// - InMemoryRateLimiter: Thread-safe in-memory implementation
// - Per-client rate buckets with automatic cleanup
// - Fixed window algorithm for simplicity and performance
// - Concurrent request handling with fine-grained locking
//
// Algorithm:
// Fixed window rate limiting:
// 1. Each client gets a bucket with counter and reset time
// 2. Requests increment counter until limit reached
// 3. Bucket resets after time window expires
// 4. Old buckets cleaned up periodically to prevent memory leaks
//
// Data Structure:
// - sync.Map for lock-free reads of client buckets
// - Per-bucket mutex for thread-safe counter updates
// - Periodic cleanup of expired buckets
//
// Limitations:
// - Not suitable for distributed deployments (per-instance only)
// - Fixed window can allow burst at window boundaries
// - State lost on process restart
// - Memory usage grows with number of unique clients
//
// Performance:
// - O(1) rate limit checks
// - Lock-free bucket lookups
// - Minimal memory allocation
// - Automatic memory cleanup
//
// Usage:
// limiter := NewInMemoryRateLimiter(config)
// Used as fallback by RateLimitTransport when Redis is unavailable.
package security

import (
	"context"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/ui"
)

// InMemoryRateLimiter provides rate limiting without Redis dependency
type InMemoryRateLimiter struct {
	config      RateLimitConfig
	buckets     sync.Map // map[string]*rateBucket
	cleanupMu   sync.Mutex
	lastCleanup time.Time
}

type rateBucket struct {
	mu        sync.Mutex
	count     int
	resetTime time.Time
}

// NewInMemoryRateLimiter creates a new in-memory rate limiter
func NewInMemoryRateLimiter(config RateLimitConfig) RateLimiter {
	return &InMemoryRateLimiter{
		config:      config,
		lastCleanup: time.Now(),
	}
}

// Allow checks if a request is allowed
func (l *InMemoryRateLimiter) Allow(ctx context.Context, key string) (bool, int) {
	now := time.Now()

	// Periodic cleanup of old buckets
	l.cleanupIfNeeded(now)

	// Get or create bucket
	bucketInterface, _ := l.buckets.LoadOrStore(key, &rateBucket{
		count:     0,
		resetTime: now.Add(time.Minute),
	})
	bucket := bucketInterface.(*rateBucket)

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Reset if window expired
	if now.After(bucket.resetTime) {
		bucket.count = 0
		bucket.resetTime = now.Add(time.Minute)
	}

	// Check limit
	if bucket.count >= l.config.RequestsPerMinute {
		retryAfter := int(bucket.resetTime.Sub(now).Seconds())
		return false, retryAfter
	}

	// Increment and allow
	bucket.count++
	return true, 0
}

// Remaining returns remaining requests in the current window
func (l *InMemoryRateLimiter) Remaining(ctx context.Context, key string) int {
	bucketInterface, ok := l.buckets.Load(key)
	if !ok {
		return l.config.RequestsPerMinute
	}

	bucket := bucketInterface.(*rateBucket)
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	if time.Now().After(bucket.resetTime) {
		return l.config.RequestsPerMinute
	}

	remaining := l.config.RequestsPerMinute - bucket.count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// cleanupIfNeeded removes expired buckets to prevent memory leak
func (l *InMemoryRateLimiter) cleanupIfNeeded(now time.Time) {
	// Only cleanup every 5 minutes
	if now.Sub(l.lastCleanup) < 5*time.Minute {
		return
	}

	l.cleanupMu.Lock()
	defer l.cleanupMu.Unlock()

	// Double-check after acquiring lock
	if now.Sub(l.lastCleanup) < 5*time.Minute {
		return
	}

	// Remove expired buckets
	l.buckets.Range(func(key, value interface{}) bool {
		bucket := value.(*rateBucket)
		bucket.mu.Lock()
		expired := now.After(bucket.resetTime.Add(time.Minute))
		bucket.mu.Unlock()

		if expired {
			l.buckets.Delete(key)
		}
		return true
	})

	l.lastCleanup = now
}

// NewInMemoryRateLimitTransport creates a rate-limited transport using in-memory storage
func NewInMemoryRateLimitTransport(transport ui.Transport, config RateLimitConfig, logger core.Logger, telemetry core.Telemetry) ui.Transport {
	if !config.Enabled {
		return transport
	}

	limiter := NewInMemoryRateLimiter(config)

	// Log that we're using in-memory rate limiting
	if logger != nil {
		logger.Info("Using in-memory rate limiter", map[string]interface{}{
			"transport":           transport.Name(),
			"requests_per_minute": config.RequestsPerMinute,
			"burst_size":          config.BurstSize,
		})
	}

	return &RateLimitTransport{
		underlying: transport,
		limiter:    limiter,
		config:     config,
		logger:     logger,
		telemetry:  telemetry,
	}
}
