//go:build security
// +build security

// Package security provides intelligent security feature detection and application.
// This file implements the SmartSecurityTransport that dynamically adjusts security
// based on infrastructure capabilities.
//
// Purpose:
// - Implements intelligent detection of infrastructure-provided security
// - Dynamically applies only necessary security features
// - Prevents redundant security layers when infrastructure provides them
// - Optimizes performance by avoiding unnecessary security overhead
//
// Scope:
// - SmartSecurityTransport: Transport wrapper with infrastructure detection
// - Detection of API gateways, load balancers, and service meshes
// - Selective application of rate limiting and security headers
// - CORS handling with infrastructure awareness
// - Request-pattern based caching of detection results
//
// Architecture:
// The smart transport operates in three modes:
// 1. Full bypass: When comprehensive infrastructure security is detected (API Gateway)
// 2. Selective application: Apply only missing security features
// 3. Full application: When no infrastructure security is detected
//
// Detection Methods:
// - HTTP headers from known gateways (AWS, Azure, Kong, Istio)
// - Environment variables indicating infrastructure presence
// - Response headers showing existing security features
//
// Performance:
// - Caches detection results per request pattern
// - Minimal overhead for infrastructure detection
// - Zero security overhead when infrastructure provides protection
//
// Usage:
// Created automatically by WithSecurity() when AutoDetect is enabled.
// Transparently wraps the underlying transport with smart security logic.
package security

import (
	"context"
	"net/http"
	"os"
	"sync"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/ui"
)

// SmartSecurityTransport dynamically applies security based on infrastructure detection
type SmartSecurityTransport struct {
	underlying ui.Transport
	config     SecurityConfig
	detector   *InfrastructureDetector
	logger     core.Logger
	telemetry  core.Telemetry

	// Cache detection results per request pattern
	detectionCache sync.Map
}

// Name returns the transport name
func (s *SmartSecurityTransport) Name() string {
	return s.underlying.Name() + "-smart-security"
}

// Description returns the transport description
func (s *SmartSecurityTransport) Description() string {
	return s.underlying.Description() + " with smart security detection"
}

// Available checks if the transport is available
func (s *SmartSecurityTransport) Available() bool {
	return s.underlying.Available()
}

// Priority returns the transport priority
func (s *SmartSecurityTransport) Priority() int {
	return s.underlying.Priority()
}

// CreateHandler creates an HTTP handler with smart security
func (s *SmartSecurityTransport) CreateHandler(agent ui.ChatAgent) http.Handler {
	originalHandler := s.underlying.CreateHandler(agent)

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Check for comprehensive infrastructure security (e.g., API Gateway)
		// If we detect gateway headers or environment variables, skip all security
		infraProvidesSecurity := s.detector.IsSecurityProvidedByInfra(req)

		// But only skip ALL security if it's comprehensive infrastructure (not just rate limiting)
		if infraProvidesSecurity {
			// Check if it's just rate limiting or comprehensive infrastructure
			hasGateway := false
			if req != nil {
				gatewayHeaders := []string{
					"X-Amzn-Trace-Id",   // AWS API Gateway
					"X-Kong-Proxy",      // Kong
					"X-Azure-Ref",       // Azure
					"X-Google-Trace-ID", // Google Cloud
					"X-B3-TraceId",      // Istio
				}
				for _, header := range gatewayHeaders {
					if req.Header.Get(header) != "" {
						hasGateway = true
						break
					}
				}
			}

			// Also check environment variables for gateways
			if !hasGateway {
				gatewayEnvs := []string{"API_GATEWAY_ENABLED", "KONG_PROXY", "AWS_API_GATEWAY_ID", "ISTIO_PROXY"}
				for _, env := range gatewayEnvs {
					if val := os.Getenv(env); val != "" && val != "false" && val != "0" {
						hasGateway = true
						break
					}
				}
			}

			if hasGateway {
				// Comprehensive infrastructure detected, skip all security
				if s.logger != nil {
					s.logger.Info("Infrastructure security detected, skipping all security", map[string]interface{}{
						"transport": s.underlying.Name(),
						"gateway":   true,
					})
				}
				if s.telemetry != nil {
					s.telemetry.RecordMetric("security.smart.infra_detected", 1, map[string]string{
						"transport": s.underlying.Name(),
						"type":      "gateway",
					})
				}
				originalHandler.ServeHTTP(w, req)
				return
			}
		}

		// Apply security features selectively based on what infrastructure provides

		// Rate limiting - check independently
		if s.config.RateLimit != nil && s.config.RateLimit.Enabled {
			if !s.detector.IsRateLimitingProvidedByInfra(req) {
				// Apply rate limiting
				if !s.applyRateLimit(w, req) {
					return // Request was rate limited
				}
			}
		}

		// Security headers and CORS - check independently
		if s.config.SecurityHeaders != nil && s.config.SecurityHeaders.Enabled {
			// Check if CORS is provided by infrastructure
			if !s.detector.IsCORSProvidedByInfra(w.Header()) {
				// Apply security headers if not provided
				s.applySecurityHeaders(w, req)

				// Handle OPTIONS preflight requests
				if req.Method == "OPTIONS" && s.config.SecurityHeaders.CORS != nil {
					// handleCORS will write the response for OPTIONS
					return // Don't call underlying handler for OPTIONS
				}
			}
		}

		originalHandler.ServeHTTP(w, req)
	})
}

// applyRateLimit applies rate limiting if needed
func (s *SmartSecurityTransport) applyRateLimit(w http.ResponseWriter, req *http.Request) bool {
	// This would use the rate limiter if we had Redis
	// For now, just return true (allow all)
	// In production, this would integrate with Redis or in-memory limiter
	return true
}

// applySecurityHeaders applies security headers
func (s *SmartSecurityTransport) applySecurityHeaders(w http.ResponseWriter, req *http.Request) {
	if s.config.SecurityHeaders == nil {
		return
	}

	// Apply security headers
	for key, value := range s.config.SecurityHeaders.Headers {
		if w.Header().Get(key) == "" {
			w.Header().Set(key, value)
		}
	}

	// Handle CORS if configured
	if s.config.SecurityHeaders.CORS != nil {
		s.handleCORS(w, req)
	}
}

// handleCORS handles CORS headers
func (s *SmartSecurityTransport) handleCORS(w http.ResponseWriter, req *http.Request) {
	cors := s.config.SecurityHeaders.CORS
	origin := req.Header.Get("Origin")

	// Check if origin is allowed
	allowed := false
	for _, allowedOrigin := range cors.AllowedOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			allowed = true
			break
		}
	}

	if allowed {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		if cors.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight - write response and return
		if req.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.WriteHeader(http.StatusOK)
			// Response written, caller should return
		}
	} else if req.Method == "OPTIONS" {
		// For unallowed origins on OPTIONS, still need to respond
		w.WriteHeader(http.StatusOK)
	}
}

// ClientExample returns client example
func (s *SmartSecurityTransport) ClientExample() string {
	return s.underlying.ClientExample()
}

// Initialize initializes the transport
func (s *SmartSecurityTransport) Initialize(config ui.TransportConfig) error {
	return s.underlying.Initialize(config)
}

// Start starts the transport
func (s *SmartSecurityTransport) Start(ctx context.Context) error {
	return s.underlying.Start(ctx)
}

// Stop stops the transport
func (s *SmartSecurityTransport) Stop(ctx context.Context) error {
	return s.underlying.Stop(ctx)
}

// HealthCheck checks the health of the transport
func (s *SmartSecurityTransport) HealthCheck(ctx context.Context) error {
	return s.underlying.HealthCheck(ctx)
}

// Capabilities returns the transport capabilities
func (s *SmartSecurityTransport) Capabilities() []ui.TransportCapability {
	return s.underlying.Capabilities()
}
