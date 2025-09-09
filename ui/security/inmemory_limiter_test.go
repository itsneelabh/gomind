//go:build security
// +build security

package security

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestInMemoryRateLimiter(t *testing.T) {
	ctx := context.Background()

	t.Run("Basic rate limiting", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 5,
		}

		limiter := NewInMemoryRateLimiter(config)
		key := "test-key"

		// First 5 requests should be allowed
		for i := 0; i < 5; i++ {
			allowed, retryAfter := limiter.Allow(ctx, key)
			if !allowed {
				t.Errorf("Request %d should be allowed", i+1)
			}
			if retryAfter != 0 {
				t.Errorf("RetryAfter should be 0 for allowed request, got %d", retryAfter)
			}
		}

		// 6th request should be denied
		allowed, retryAfter := limiter.Allow(ctx, key)
		if allowed {
			t.Error("6th request should be denied")
		}
		if retryAfter <= 0 || retryAfter > 60 {
			t.Errorf("RetryAfter should be between 1-60 seconds, got %d", retryAfter)
		}
	})

	t.Run("Window reset", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 2,
		}

		limiter := NewInMemoryRateLimiter(config).(*InMemoryRateLimiter)
		key := "reset-key"

		// Use up the limit
		limiter.Allow(ctx, key)
		limiter.Allow(ctx, key)

		// Should be denied
		allowed, _ := limiter.Allow(ctx, key)
		if allowed {
			t.Error("Should be rate limited")
		}

		// Manually expire the bucket
		if bucket, ok := limiter.buckets.Load(key); ok {
			b := bucket.(*rateBucket)
			b.mu.Lock()
			b.resetTime = time.Now().Add(-time.Second)
			b.mu.Unlock()
		}

		// Should be allowed after reset
		allowed, _ = limiter.Allow(ctx, key)
		if !allowed {
			t.Error("Should be allowed after window reset")
		}
	})

	t.Run("Remaining count", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 10,
		}

		limiter := NewInMemoryRateLimiter(config)
		key := "remaining-key"

		// Initially should have full capacity
		remaining := limiter.Remaining(ctx, key)
		if remaining != 10 {
			t.Errorf("Expected 10 remaining, got %d", remaining)
		}

		// Use 3 requests
		for i := 0; i < 3; i++ {
			limiter.Allow(ctx, key)
		}

		remaining = limiter.Remaining(ctx, key)
		if remaining != 7 {
			t.Errorf("Expected 7 remaining, got %d", remaining)
		}

		// Use up all remaining
		for i := 0; i < 7; i++ {
			limiter.Allow(ctx, key)
		}

		remaining = limiter.Remaining(ctx, key)
		if remaining != 0 {
			t.Errorf("Expected 0 remaining, got %d", remaining)
		}

		// Try one more (should be denied)
		allowed, _ := limiter.Allow(ctx, key)
		if allowed {
			t.Error("Should be denied when limit reached")
		}

		remaining = limiter.Remaining(ctx, key)
		if remaining != 0 {
			t.Errorf("Expected 0 remaining when over limit, got %d", remaining)
		}
	})

	t.Run("Different keys isolated", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 1,
		}

		limiter := NewInMemoryRateLimiter(config)

		// Use up limit for key1
		allowed1, _ := limiter.Allow(ctx, "key1")
		if !allowed1 {
			t.Error("First request for key1 should be allowed")
		}

		// key1 should be denied
		allowed1, _ = limiter.Allow(ctx, "key1")
		if allowed1 {
			t.Error("Second request for key1 should be denied")
		}

		// key2 should still be allowed
		allowed2, _ := limiter.Allow(ctx, "key2")
		if !allowed2 {
			t.Error("First request for key2 should be allowed")
		}
	})

	t.Run("Concurrent access", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 100,
		}

		limiter := NewInMemoryRateLimiter(config)
		key := "concurrent-key"

		var wg sync.WaitGroup
		allowed := 0
		denied := 0
		var mu sync.Mutex

		// Run 200 concurrent requests (should allow 100, deny 100)
		for i := 0; i < 200; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if ok, _ := limiter.Allow(ctx, key); ok {
					mu.Lock()
					allowed++
					mu.Unlock()
				} else {
					mu.Lock()
					denied++
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		if allowed != 100 {
			t.Errorf("Expected 100 allowed, got %d", allowed)
		}
		if denied != 100 {
			t.Errorf("Expected 100 denied, got %d", denied)
		}
	})
}

func TestInMemoryRateLimiterCleanup(t *testing.T) {
	config := RateLimitConfig{
		Enabled:           true,
		RequestsPerMinute: 1,
	}

	limiter := NewInMemoryRateLimiter(config).(*InMemoryRateLimiter)
	ctx := context.Background()

	// Create some buckets
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("cleanup-key-%d", i)
		limiter.Allow(ctx, key)
	}

	// Verify buckets exist
	count := 0
	limiter.buckets.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	if count != 10 {
		t.Errorf("Expected 10 buckets, got %d", count)
	}

	// Manually expire all buckets
	now := time.Now()
	limiter.buckets.Range(func(key, value interface{}) bool {
		bucket := value.(*rateBucket)
		bucket.mu.Lock()
		bucket.resetTime = now.Add(-2 * time.Minute)
		bucket.mu.Unlock()
		return true
	})

	// Force cleanup
	limiter.lastCleanup = now.Add(-6 * time.Minute)
	limiter.cleanupIfNeeded(now)

	// Verify buckets were cleaned up
	count = 0
	limiter.buckets.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	if count != 0 {
		t.Errorf("Expected 0 buckets after cleanup, got %d", count)
	}
}

func TestInMemoryRateLimitTransport(t *testing.T) {
	mockTransport := &MockTransport{
		name:        "test",
		description: "Test transport",
		available:   true,
		priority:    100,
	}

	t.Run("Disabled config returns original transport", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled: false,
		}

		wrapped := NewInMemoryRateLimitTransport(mockTransport, config, nil, nil)

		// Should return the original transport unchanged
		if wrapped != mockTransport {
			t.Error("Disabled rate limiting should return original transport")
		}
	})

	t.Run("Enabled config wraps transport", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 10,
		}

		wrapped := NewInMemoryRateLimitTransport(mockTransport, config, nil, nil)

		// Should return a wrapped transport
		if wrapped == mockTransport {
			t.Error("Enabled rate limiting should wrap transport")
		}

		// Should be a RateLimitTransport
		if _, ok := wrapped.(*RateLimitTransport); !ok {
			t.Error("Should return a RateLimitTransport")
		}
	})
}

func BenchmarkInMemoryRateLimiter(b *testing.B) {
	config := RateLimitConfig{
		Enabled:           true,
		RequestsPerMinute: 1000000, // High limit to avoid denials
	}

	limiter := NewInMemoryRateLimiter(config)
	ctx := context.Background()

	b.Run("Sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			limiter.Allow(ctx, "bench-key")
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				limiter.Allow(ctx, "bench-key")
			}
		})
	})

	b.Run("Different Keys", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				key := fmt.Sprintf("key-%d", i%100)
				limiter.Allow(ctx, key)
				i++
			}
		})
	})
}
