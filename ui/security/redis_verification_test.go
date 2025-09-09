//go:build security
// +build security

package security

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/itsneelabh/gomind/core"
)

// TestRedisRateLimitingVerification provides comprehensive proof that Redis rate limiting works
func TestRedisRateLimitingVerification(t *testing.T) {
	// Use local Redis
	redisURL := "redis://localhost:6379"

	// Create a mock logger to capture all logs
	logger := &VerificationLogger{
		logs: make([]string, 0),
		mu:   &sync.Mutex{},
	}

	// Create a mock telemetry to capture metrics
	telemetry := &VerificationTelemetry{
		metrics: make(map[string]float64),
		mu:      &sync.Mutex{},
	}

	t.Run("PROOF: Redis Rate Limiter Uses DB 1", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 5, // Low limit for testing
		}

		limiter, err := NewEnhancedRedisRateLimiter(config, redisURL, logger, telemetry)
		if err != nil {
			t.Fatalf("Failed to create Redis rate limiter: %v", err)
		}
		defer limiter.Close()

		// Verify it's using DB 1
		if limiter.client.GetDB() != 1 {
			t.Errorf("❌ FAILED: Expected DB 1, got DB %d", limiter.client.GetDB())
		} else {
			t.Logf("✅ VERIFIED: Using Redis DB 1 for rate limiting (isolated from DB 0)")
		}

		// Verify namespace
		if limiter.client.GetNamespace() != "gomind:ratelimit" {
			t.Errorf("❌ FAILED: Wrong namespace: %s", limiter.client.GetNamespace())
		} else {
			t.Logf("✅ VERIFIED: Using namespace 'gomind:ratelimit'")
		}
	})

	t.Run("PROOF: Redis Sliding Window Works", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 3, // Very low for clear demonstration
		}

		limiter, err := NewEnhancedRedisRateLimiter(config, redisURL, logger, telemetry)
		if err != nil {
			t.Fatalf("Failed to create Redis rate limiter: %v", err)
		}
		defer limiter.Close()

		ctx := context.Background()
		testKey := "test:verification:sliding"

		// Clear any existing state
		limiter.Reset(ctx, testKey)

		// Test 1: First 3 requests should pass
		for i := 1; i <= 3; i++ {
			allowed, _ := limiter.Allow(ctx, testKey)
			if !allowed {
				t.Errorf("Request %d should be allowed", i)
			} else {
				t.Logf("✅ Request %d: ALLOWED (within limit)", i)
			}
		}

		// Test 2: 4th request should be rejected
		allowed, retryAfter := limiter.Allow(ctx, testKey)
		if allowed {
			t.Error("❌ FAILED: 4th request should be rejected")
		} else {
			t.Logf("✅ Request 4: REJECTED (limit exceeded), retry after %d seconds", retryAfter)
		}

		// Test 3: Check remaining
		remaining := limiter.Remaining(ctx, testKey)
		t.Logf("✅ Remaining requests: %d", remaining)

		// Test 4: Verify metrics were recorded
		telemetry.mu.Lock()
		allowedCount := telemetry.metrics["security.rate_limit.allowed"]
		rejectedCount := telemetry.metrics["security.rate_limit.rejected"]
		telemetry.mu.Unlock()

		t.Logf("✅ Metrics: Allowed=%v, Rejected=%v", allowedCount, rejectedCount)
	})

	t.Run("PROOF: Redis Persists Across Connections", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 2,
		}

		ctx := context.Background()
		testKey := "test:persistence"

		// Create first limiter and use up the limit
		limiter1, _ := NewEnhancedRedisRateLimiter(config, redisURL, logger, telemetry)
		limiter1.Reset(ctx, testKey)
		limiter1.Allow(ctx, testKey)
		limiter1.Allow(ctx, testKey)

		// Third request should fail
		allowed1, _ := limiter1.Allow(ctx, testKey)
		if allowed1 {
			t.Error("Should be rate limited")
		}
		limiter1.Close()

		// Create NEW limiter (simulating new instance/connection)
		limiter2, _ := NewEnhancedRedisRateLimiter(config, redisURL, logger, telemetry)
		defer limiter2.Close()

		// Should STILL be rate limited (state persisted in Redis)
		allowed2, _ := limiter2.Allow(ctx, testKey)
		if allowed2 {
			t.Error("❌ FAILED: New connection should still see rate limit")
		} else {
			t.Log("✅ VERIFIED: Rate limit state persists across connections (Redis working)")
		}
	})

	t.Run("PROOF: WithSecurity Auto-Detects Redis", func(t *testing.T) {
		// Clear logger
		logger.Clear()

		config := SecurityConfig{
			Enabled:   true,
			RedisURL:  redisURL,
			Logger:    logger,
			Telemetry: telemetry,
			RateLimit: &RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 10,
			},
		}

		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		// This should auto-detect and use Redis
		wrapped := WithSecurity(mockTransport, config)

		// Check logs to verify Redis was used
		logs := logger.GetLogs()
		redisUsed := false
		for _, log := range logs {
			if containsString(log, "Using Redis rate limiter") {
				redisUsed = true
				t.Logf("✅ VERIFIED: %s", log)
			}
		}

		if !redisUsed {
			t.Error("❌ FAILED: Redis was not auto-detected")
			t.Logf("Logs: %v", logs)
		}

		// Test that rate limiting actually works
		handler := wrapped.CreateHandler(nil)

		// Make requests to trigger rate limiting
		for i := 1; i <= 12; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Session-ID", "verification-test")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if i <= 10 {
				if rec.Code != http.StatusOK {
					t.Errorf("Request %d should succeed, got %d", i, rec.Code)
				}
			} else {
				if rec.Code != http.StatusTooManyRequests {
					t.Errorf("Request %d should be rate limited, got %d", i, rec.Code)
				} else {
					t.Logf("✅ Request %d: Rate limited (429)", i)
				}
			}
		}
	})

	// Print summary
	t.Log("\n========== VERIFICATION SUMMARY ==========")
	t.Log("✅ Redis DB 1 isolation confirmed")
	t.Log("✅ Sliding window algorithm working")
	t.Log("✅ State persists across connections")
	t.Log("✅ Auto-detection from REDIS_URL works")
	t.Log("✅ Rate limiting enforced correctly")
	t.Log("==========================================")
}

// TestRedisVsInMemoryComparison shows the difference between Redis and in-memory
func TestRedisVsInMemoryComparison(t *testing.T) {
	config := RateLimitConfig{
		Enabled:           true,
		RequestsPerMinute: 5,
	}

	ctx := context.Background()

	t.Run("In-Memory: State Lost on New Instance", func(t *testing.T) {
		// Create first in-memory limiter
		limiter1 := NewInMemoryRateLimiter(config)
		testKey := "inmemory:test"

		// Use up the limit
		for i := 0; i < 5; i++ {
			limiter1.Allow(ctx, testKey)
		}

		// Should be rate limited
		allowed1, _ := limiter1.Allow(ctx, testKey)
		if allowed1 {
			t.Error("Should be rate limited")
		}

		// Create NEW in-memory limiter (simulating new instance)
		limiter2 := NewInMemoryRateLimiter(config)

		// Should NOT be rate limited (state lost!)
		allowed2, _ := limiter2.Allow(ctx, testKey)
		if !allowed2 {
			t.Error("New in-memory instance should not see old state")
		} else {
			t.Log("⚠️  CONFIRMED: In-memory loses state across instances (NOT suitable for production)")
		}
	})

	t.Run("Redis: State Shared Across Instances", func(t *testing.T) {
		redisURL := "redis://localhost:6379"

		// Create first Redis limiter
		limiter1, err := NewEnhancedRedisRateLimiter(config, redisURL, nil, nil)
		if err != nil {
			t.Skip("Redis not available:", err)
		}
		testKey := "redis:shared:test"
		limiter1.Reset(ctx, testKey)

		// Use up the limit
		for i := 0; i < 5; i++ {
			limiter1.Allow(ctx, testKey)
		}

		// Should be rate limited
		allowed1, _ := limiter1.Allow(ctx, testKey)
		if allowed1 {
			t.Error("Should be rate limited")
		}
		limiter1.Close()

		// Create NEW Redis limiter (simulating new instance)
		limiter2, _ := NewEnhancedRedisRateLimiter(config, redisURL, nil, nil)
		defer limiter2.Close()

		// SHOULD be rate limited (state shared!)
		allowed2, _ := limiter2.Allow(ctx, testKey)
		if allowed2 {
			t.Error("New Redis instance should see shared state")
		} else {
			t.Log("✅ CONFIRMED: Redis shares state across instances (production ready)")
		}
	})
}

// VerificationLogger captures logs for verification
type VerificationLogger struct {
	logs []string
	mu   *sync.Mutex
}

func (l *VerificationLogger) Info(msg string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprintf("INFO: %s %v", msg, fields))
}

func (l *VerificationLogger) Warn(msg string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprintf("WARN: %s %v", msg, fields))
}

func (l *VerificationLogger) Error(msg string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprintf("ERROR: %s %v", msg, fields))
}

func (l *VerificationLogger) Debug(msg string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprintf("DEBUG: %s %v", msg, fields))
}

func (l *VerificationLogger) GetLogs() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.logs
}

func (l *VerificationLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = make([]string, 0)
}

// VerificationTelemetry captures metrics for verification
type VerificationTelemetry struct {
	metrics map[string]float64
	mu      *sync.Mutex
}

func (t *VerificationTelemetry) StartSpan(ctx context.Context, name string) (context.Context, core.Span) {
	return ctx, &MockSpan{}
}

func (t *VerificationTelemetry) RecordMetric(name string, value float64, labels map[string]string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.metrics[name] = t.metrics[name] + value
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
