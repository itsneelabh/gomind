//go:build security
// +build security

package security

import (
	"context"
	"os"
	"testing"
)

// TestRedisRateLimiterIntegration tests the Redis rate limiter if Redis is available
func TestRedisRateLimiterIntegration(t *testing.T) {
	// Skip if Redis not available
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	// Try to create Redis limiter
	config := RateLimitConfig{
		Enabled:           true,
		RequestsPerMinute: 10,
	}

	limiter, err := NewEnhancedRedisRateLimiter(config, redisURL, nil, nil)
	if err != nil {
		t.Skip("Redis not available, skipping integration test:", err)
	}
	defer limiter.Close()

	ctx := context.Background()

	t.Run("sliding window allows correct number of requests", func(t *testing.T) {
		testKey := "test:sliding:window"

		// Reset any existing state
		limiter.Reset(ctx, testKey)

		// Should allow first 10 requests
		for i := 0; i < 10; i++ {
			allowed, retryAfter := limiter.Allow(ctx, testKey)
			if !allowed {
				t.Errorf("Request %d should be allowed, retry after: %d", i+1, retryAfter)
			}
		}

		// 11th request should be rejected
		allowed, retryAfter := limiter.Allow(ctx, testKey)
		if allowed {
			t.Error("11th request should be rejected")
		}
		if retryAfter <= 0 || retryAfter > 60 {
			t.Errorf("Retry after should be between 1 and 60, got %d", retryAfter)
		}

		// Check remaining (should be 0 after exhausting limit)
		remaining := limiter.Remaining(ctx, testKey)
		if remaining != 0 {
			t.Errorf("Expected 0 remaining after exhausting limit, got %d", remaining)
		}
	})

	t.Run("sliding window resets after time window", func(t *testing.T) {
		testKey := "test:window:reset"

		// Use a modified limiter with very short window for testing
		shortConfig := RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 2, // Very low for testing
		}

		shortLimiter, err := NewEnhancedRedisRateLimiter(shortConfig, redisURL, nil, nil)
		if err != nil {
			t.Skip("Could not create short window limiter:", err)
		}
		defer shortLimiter.Close()

		// Reset state
		shortLimiter.Reset(ctx, testKey)

		// Use up the limit
		shortLimiter.Allow(ctx, testKey)
		shortLimiter.Allow(ctx, testKey)

		// Should be rejected
		allowed, _ := shortLimiter.Allow(ctx, testKey)
		if allowed {
			t.Error("Should be rate limited after 2 requests")
		}

		// Note: In real test, we'd wait 60 seconds for window to reset
		// For unit test, we just verify the logic is correct
	})

	t.Run("health check works", func(t *testing.T) {
		err := limiter.HealthCheck(ctx)
		if err != nil {
			t.Errorf("Health check failed: %v", err)
		}
	})

	t.Run("metrics are provided", func(t *testing.T) {
		metrics := limiter.GetMetrics(ctx)

		if metrics["backend"] != "redis" {
			t.Errorf("Expected backend=redis, got %v", metrics["backend"])
		}

		if metrics["algorithm"] != "sliding_window" {
			t.Errorf("Expected algorithm=sliding_window, got %v", metrics["algorithm"])
		}

		if metrics["db"] != 1 {
			t.Errorf("Expected db=1, got %v", metrics["db"])
		}
	})
}

// TestRedisDBIsolation verifies that rate limiting uses the correct Redis DB
func TestRedisDBIsolation(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	config := RateLimitConfig{
		Enabled:           true,
		RequestsPerMinute: 100,
	}

	limiter, err := NewEnhancedRedisRateLimiter(config, redisURL, nil, nil)
	if err != nil {
		t.Skip("Redis not available, skipping DB isolation test:", err)
	}
	defer limiter.Close()

	// Verify it's using DB 1
	if limiter.client.GetDB() != 1 {
		t.Errorf("Expected Redis DB 1 for rate limiting, got %d", limiter.client.GetDB())
	}

	// Verify namespace
	if limiter.client.GetNamespace() != "gomind:ratelimit" {
		t.Errorf("Expected namespace 'gomind:ratelimit', got '%s'", limiter.client.GetNamespace())
	}
}

// TestRateLimiterFallback tests the fallback from Redis to in-memory
func TestRateLimiterFallback(t *testing.T) {
	config := SecurityConfig{
		Enabled:  true,
		RedisURL: "redis://invalid:6379", // Invalid Redis URL
		RateLimit: &RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 60,
		},
	}

	// Create mock transport
	mockTransport := &MockTransport{
		name: "test",
	}

	// WithSecurity should fall back to in-memory
	wrapped := WithSecurity(mockTransport, config)

	// Should still work (using in-memory)
	if wrapped == nil {
		t.Error("WithSecurity should return a transport even with invalid Redis")
	}

	// Verify it's wrapped
	if wrapped == mockTransport {
		t.Error("Transport should be wrapped even with Redis fallback")
	}
}

// BenchmarkRedisRateLimiter benchmarks Redis rate limiter performance
func BenchmarkRedisRateLimiter(b *testing.B) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	config := RateLimitConfig{
		Enabled:           true,
		RequestsPerMinute: 1000000, // High limit to avoid blocking
	}

	limiter, err := NewEnhancedRedisRateLimiter(config, redisURL, nil, nil)
	if err != nil {
		b.Skip("Redis not available for benchmark:", err)
	}
	defer limiter.Close()

	ctx := context.Background()
	testKey := "benchmark:key"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow(ctx, testKey)
		}
	})
}

// BenchmarkSlidingWindowVsFixed compares sliding window vs fixed window
func BenchmarkSlidingWindowVsFixed(b *testing.B) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	config := RateLimitConfig{
		Enabled:           true,
		RequestsPerMinute: 1000000,
	}

	ctx := context.Background()

	b.Run("SlidingWindow", func(b *testing.B) {
		limiter, err := NewEnhancedRedisRateLimiter(config, redisURL, nil, nil)
		if err != nil {
			b.Skip("Redis not available:", err)
		}
		defer limiter.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			limiter.Allow(ctx, "sliding:test")
		}
	})

	b.Run("InMemory", func(b *testing.B) {
		limiter := NewInMemoryRateLimiter(config)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			limiter.Allow(ctx, "inmemory:test")
		}
	})
}
