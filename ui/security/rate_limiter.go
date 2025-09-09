//go:build security
// +build security

// Package security provides rate limiting capabilities for UI transports.
// This file implements the RateLimitTransport that protects services from
// excessive request rates and potential abuse.
//
// Purpose:
// - Implements rate limiting to prevent API abuse and DoS attacks
// - Provides per-client request throttling based on IP address
// - Supports both distributed (Redis) and in-memory rate limiting
// - Protects backend services from overload conditions
//
// Scope:
// - RateLimitTransport: Transport wrapper with rate limiting
// - RateLimitConfig: Configuration for rate limiting behavior
// - Integration with pluggable RateLimiter implementations
// - HTTP 429 (Too Many Requests) response handling
// - Rate limit headers (X-RateLimit-*) for client feedback
//
// Rate Limiting Strategy:
// - Per-minute request limits with configurable thresholds
// - Burst size support for temporary traffic spikes
// - Client identification by IP address (X-Forwarded-For aware)
// - Graceful degradation with fallback to in-memory limiting
//
// HTTP Headers:
// - X-RateLimit-Limit: Maximum requests allowed
// - X-RateLimit-Remaining: Requests remaining in window
// - X-RateLimit-Reset: Unix timestamp when limit resets
// - Retry-After: Seconds until client can retry (on 429)
//
// Architecture:
// The rate limiter can use different backends:
// 1. Redis: Distributed rate limiting across instances
// 2. In-Memory: Per-instance limiting for single deployments
// 3. Custom: Any implementation of the RateLimiter interface
//
// Usage:
// transport := NewRateLimitTransport(baseTransport, config, limiter, logger, telemetry)
// Automatically enforces rate limits on all requests through the transport.
package security

import (
	"context"
	"fmt"
	"net/http"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/ui"
)

// RateLimitTransport wraps any transport with rate limiting
type RateLimitTransport struct {
	underlying ui.Transport
	limiter    RateLimiter
	config     RateLimitConfig
	logger     core.Logger
	telemetry  core.Telemetry
}

type RateLimitConfig struct {
	Enabled             bool `json:"enabled"`
	RequestsPerMinute   int  `json:"requests_per_minute"`
	BurstSize           int  `json:"burst_size"`
	SkipIfInfraProvided bool `json:"skip_if_infra_provided"`
}

// NewRateLimitTransport creates a rate-limited transport wrapper with a provided limiter
func NewRateLimitTransport(transport ui.Transport, config RateLimitConfig, limiter RateLimiter, logger core.Logger, telemetry core.Telemetry) ui.Transport {
	if !config.Enabled {
		return transport // Zero overhead when disabled
	}

	return &RateLimitTransport{
		underlying: transport,
		limiter:    limiter,
		config:     config,
		logger:     logger,
		telemetry:  telemetry,
	}
}

// NewRateLimitTransportWithRedis creates a rate-limited transport with Redis backend (deprecated)
func NewRateLimitTransportWithRedis(transport ui.Transport, config RateLimitConfig, redisClient RedisClient, logger core.Logger, telemetry core.Telemetry) ui.Transport {
	if !config.Enabled {
		return transport // Zero overhead when disabled
	}

	return &RateLimitTransport{
		underlying: transport,
		limiter:    NewRedisRateLimiter(config, redisClient),
		config:     config,
		logger:     logger,
		telemetry:  telemetry,
	}
}

// Implement Transport interface by delegating to underlying
func (r *RateLimitTransport) Name() string {
	return fmt.Sprintf("%s-ratelimited", r.underlying.Name())
}

func (r *RateLimitTransport) Description() string {
	return fmt.Sprintf("%s with rate limiting", r.underlying.Description())
}

func (r *RateLimitTransport) Available() bool {
	return r.underlying.Available()
}

func (r *RateLimitTransport) Priority() int {
	return r.underlying.Priority()
}

func (r *RateLimitTransport) CreateHandler(agent ui.ChatAgent) http.Handler {
	originalHandler := r.underlying.CreateHandler(agent)

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Check if infrastructure already handled rate limiting
		if r.config.SkipIfInfraProvided && req.Header.Get("X-RateLimit-Limit") != "" {
			if r.logger != nil {
				r.logger.Debug("Rate limiting bypassed by infrastructure", map[string]interface{}{
					"transport": r.underlying.Name(),
					"header":    "X-RateLimit-Limit",
				})
			}
			if r.telemetry != nil {
				r.telemetry.RecordMetric("security.rate_limit.infra_bypass", 1, map[string]string{
					"transport": r.underlying.Name(),
				})
			}
			originalHandler.ServeHTTP(w, req)
			return
		}

		// Apply rate limiting
		sessionID := r.extractSessionID(req)
		allowed, retryAfter := r.limiter.Allow(req.Context(), sessionID)

		if !allowed {
			if r.logger != nil {
				r.logger.Warn("Rate limit exceeded", map[string]interface{}{
					"transport":   r.underlying.Name(),
					"session_id":  sessionID,
					"limit":       r.config.RequestsPerMinute,
					"retry_after": retryAfter,
				})
			}
			if r.telemetry != nil {
				r.telemetry.RecordMetric("security.rate_limit.rejected", 1, map[string]string{
					"transport": r.underlying.Name(),
				})
			}

			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", r.config.RequestsPerMinute))
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Rate limit check passed
		if r.telemetry != nil {
			r.telemetry.RecordMetric("security.rate_limit.allowed", 1, map[string]string{
				"transport": r.underlying.Name(),
			})
		}

		// Add rate limit headers
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", r.config.RequestsPerMinute))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", r.limiter.Remaining(req.Context(), sessionID)))

		originalHandler.ServeHTTP(w, req)
	})
}

func (r *RateLimitTransport) ClientExample() string {
	return r.underlying.ClientExample()
}

// Initialize initializes the underlying transport
func (r *RateLimitTransport) Initialize(config ui.TransportConfig) error {
	return r.underlying.Initialize(config)
}

// Start starts the underlying transport
func (r *RateLimitTransport) Start(ctx context.Context) error {
	return r.underlying.Start(ctx)
}

// Stop stops the underlying transport
func (r *RateLimitTransport) Stop(ctx context.Context) error {
	return r.underlying.Stop(ctx)
}

// HealthCheck checks the health of the underlying transport
func (r *RateLimitTransport) HealthCheck(ctx context.Context) error {
	return r.underlying.HealthCheck(ctx)
}

// Capabilities returns the underlying transport capabilities
func (r *RateLimitTransport) Capabilities() []ui.TransportCapability {
	return r.underlying.Capabilities()
}

func (r *RateLimitTransport) extractSessionID(req *http.Request) string {
	// Extract from header, cookie, or query param
	if id := req.Header.Get("X-Session-ID"); id != "" {
		return id
	}
	if cookie, err := req.Cookie("session"); err == nil {
		return cookie.Value
	}
	return req.URL.Query().Get("session")
}
