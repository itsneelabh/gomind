//go:build security
// +build security

// Package security provides production-ready security features for UI transports.
// This file serves as the main entry point for applying security layers to any transport.
//
// Purpose:
// - Provides unified security configuration through SecurityConfig
// - Implements intelligent security feature detection and auto-configuration
// - Wraps transports with rate limiting and security headers as needed
// - Supports both distributed (Redis) and in-memory rate limiting
// - Enables smart detection of infrastructure-provided security
//
// Scope:
// - SecurityConfig: Comprehensive configuration for all security features
// - WithSecurity: Main function to wrap transports with security layers
// - Infrastructure detection to avoid redundant security layers
// - Rate limiting with Redis/in-memory fallback
// - Security headers and CORS configuration
//
// Architecture:
// The wrapper pattern allows security features to be:
// 1. Applied conditionally based on configuration and environment
// 2. Layered in the correct order (rate limiting first, headers last)
// 3. Automatically disabled when infrastructure provides equivalent protection
// 4. Tested independently through build tags (security vs non-security builds)
//
// Usage:
// transport := WithSecurity(baseTransport, DefaultSecurityConfig())
// The wrapper automatically detects and applies appropriate security based on
// the environment (development, staging, production) and infrastructure.
//
// Build Tags:
// This file is only compiled when the 'security' build tag is present.
// Without the tag, a no-op version is used for zero overhead.
package security

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/ui"
)

// SecurityConfig combines all security features
type SecurityConfig struct {
	Enabled             bool                   `json:"enabled"`
	AutoDetect          bool                   `json:"auto_detect"`  // Auto-detect infrastructure security
	ForceEnable         bool                   `json:"force_enable"` // Override auto-detection
	RateLimit           *RateLimitConfig       `json:"rate_limit,omitempty"`
	SecurityHeaders     *SecurityHeadersConfig `json:"security_headers,omitempty"`
	RedisClientProvider func() RedisClient     `json:"-"` // Optional Redis provider (deprecated)

	// Redis configuration for distributed features
	RedisURL      string `json:"redis_url,omitempty" env:"REDIS_URL"` // Redis connection URL
	ForceInMemory bool   `json:"force_in_memory,omitempty"`           // Force in-memory (testing only)

	// Telemetry and logging (optional)
	Logger    core.Logger    `json:"-"` // For logging security events
	Telemetry core.Telemetry `json:"-"` // For metrics and tracing
}

var (
	detector     *InfrastructureDetector
	detectorOnce sync.Once
)

// WithSecurity wraps a transport with security features ONLY if needed
// This is the main entry point for adding security to any transport
func WithSecurity(transport ui.Transport, config SecurityConfig) ui.Transport {
	// If security is completely disabled, return original transport
	if !config.Enabled {
		return transport
	}

	// Initialize detector once
	detectorOnce.Do(func() {
		detector = NewInfrastructureDetector()
	})

	// Auto-detect if infrastructure provides security
	if config.AutoDetect && !config.ForceEnable {
		// We'll check this dynamically in the handlers
		if config.Logger != nil {
			config.Logger.Info("Smart security enabled with auto-detection", map[string]interface{}{
				"transport": transport.Name(),
			})
		}
		return &SmartSecurityTransport{
			underlying: transport,
			config:     config,
			detector:   detector,
			logger:     config.Logger,
			telemetry:  config.Telemetry,
		}
	}

	// Apply security features in order
	result := transport

	// Rate limiting first (to reject early)
	if config.RateLimit != nil && config.RateLimit.Enabled {
		rateLimiterApplied := false

		// Try Redis first (unless forced to use in-memory)
		if !config.ForceInMemory {
			// Try new RedisURL config first
			if config.RedisURL != "" {
				rl, err := NewEnhancedRedisRateLimiter(*config.RateLimit, config.RedisURL, config.Logger, config.Telemetry)
				if err == nil {
					result = NewRateLimitTransport(result, *config.RateLimit, rl, config.Logger, config.Telemetry)
					rateLimiterApplied = true
					if config.Logger != nil {
						config.Logger.Info("Using Redis rate limiter (sliding window)", map[string]interface{}{
							"db":        core.RedisDBRateLimiting,
							"algorithm": "sliding_window",
						})
					}
				} else if config.Logger != nil {
					config.Logger.Warn("Failed to connect to Redis for rate limiting", map[string]interface{}{
						"error":    err.Error(),
						"fallback": "in-memory",
					})
				}
			}

			// Try deprecated RedisClientProvider for backward compatibility
			if !rateLimiterApplied && config.RedisClientProvider != nil {
				redisClient := config.RedisClientProvider()
				result = NewRateLimitTransportWithRedis(result, *config.RateLimit, redisClient, config.Logger, config.Telemetry)
				rateLimiterApplied = true
				if config.Logger != nil {
					config.Logger.Info("Using Redis rate limiter (legacy provider)", nil)
				}
			}
		}

		// Fall back to in-memory with warning
		if !rateLimiterApplied {
			inMemoryLimiter := NewInMemoryRateLimiter(*config.RateLimit)
			result = NewRateLimitTransport(result, *config.RateLimit, inMemoryLimiter, config.Logger, config.Telemetry)
			if config.Logger != nil {
				config.Logger.Warn("Using in-memory rate limiter - NOT SUITABLE FOR PRODUCTION", map[string]interface{}{
					"reason":         "Redis not available or ForceInMemory=true",
					"warning":        "Rate limits are per-instance, not distributed",
					"recommendation": "Set REDIS_URL environment variable for production",
				})
			}
		}
	}

	// Security headers last (so they're set even on rate limit responses)
	if config.SecurityHeaders != nil && config.SecurityHeaders.Enabled {
		result = NewSecurityHeadersTransport(result, *config.SecurityHeaders, config.Logger, config.Telemetry)
	}

	return result
}

// DefaultSecurityConfig provides production-ready defaults
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		Enabled:     true,
		AutoDetect:  true,  // Smart detection of infrastructure security
		ForceEnable: false, // Don't override infrastructure
		RedisURL:    getEnvOrDefault("REDIS_URL", "redis://localhost:6379"),
		RateLimit: &RateLimitConfig{
			Enabled:             true,
			RequestsPerMinute:   60,
			BurstSize:           10,
			SkipIfInfraProvided: true,
		},
		SecurityHeaders: &SecurityHeadersConfig{
			Enabled:             true,
			OnlySetMissing:      true,
			Headers:             DefaultSecurityHeaders(),
			SkipIfInfraProvided: true,
			CORS: &CORSConfig{
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
				AllowedHeaders:   []string{"Content-Type", "Authorization"},
				AllowCredentials: false,
				MaxAge:           3600,
			},
		},
	}
}

// getEnvOrDefault gets an environment variable or returns a default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// RateLimiter interface for pluggable implementations
type RateLimiter interface {
	Allow(ctx context.Context, key string) (allowed bool, retryAfter int)
	Remaining(ctx context.Context, key string) int
}

// RedisRateLimiter uses existing Redis connection from discovery
type RedisRateLimiter struct {
	config RateLimitConfig
	client RedisClient
}

// RedisClient interface to avoid direct Redis dependency here
type RedisClient interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, seconds int) error
	Get(ctx context.Context, key string) (string, error)
	TTL(ctx context.Context, key string) (int64, error)
}

func NewRedisRateLimiter(config RateLimitConfig, client RedisClient) RateLimiter {
	return &RedisRateLimiter{
		config: config,
		client: client,
	}
}

func (r *RedisRateLimiter) Allow(ctx context.Context, key string) (bool, int) {
	rateLimitKey := fmt.Sprintf("gomind:ratelimit:%s", key)

	// Increment counter
	count, err := r.client.Incr(ctx, rateLimitKey)
	if err != nil {
		// On error, allow the request (fail open)
		return true, 0
	}

	// Set expiry on first request in window
	if count == 1 {
		r.client.Expire(ctx, rateLimitKey, 60) // 1 minute window
	}

	// Check if limit exceeded
	if count > int64(r.config.RequestsPerMinute) {
		// Get TTL for retry-after header
		ttl, _ := r.client.TTL(ctx, rateLimitKey)
		return false, int(ttl)
	}

	return true, 0
}

// IsSecurityEnabled returns true when the security build tag is present
func IsSecurityEnabled() bool {
	return true
}

func (r *RedisRateLimiter) Remaining(ctx context.Context, key string) int {
	rateLimitKey := fmt.Sprintf("gomind:ratelimit:%s", key)

	// Get current count
	val, err := r.client.Get(ctx, rateLimitKey)
	if err != nil {
		return r.config.RequestsPerMinute
	}

	count := 0
	fmt.Sscanf(val, "%d", &count)

	remaining := r.config.RequestsPerMinute - count
	if remaining < 0 {
		return 0
	}
	return remaining
}
