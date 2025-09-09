//go:build security
// +build security

// Package security provides HTTP security headers and CORS management.
// This file implements the SecurityHeadersTransport that adds security headers
// to HTTP responses for enhanced protection against common web vulnerabilities.
//
// Purpose:
// - Adds security headers to protect against XSS, clickjacking, and other attacks
// - Implements CORS (Cross-Origin Resource Sharing) policy enforcement
// - Provides configurable header management with smart defaults
// - Supports infrastructure detection to avoid duplicate headers
//
// Scope:
// - SecurityHeadersTransport: Transport wrapper for security headers
// - SecurityHeadersConfig: Configuration for headers and CORS
// - Default security headers based on OWASP recommendations
// - CORS preflight request handling
// - Wildcard subdomain support in CORS origins
//
// Security Headers Applied:
// - X-Content-Type-Options: Prevents MIME sniffing attacks
// - X-Frame-Options: Protects against clickjacking
// - Strict-Transport-Security: Enforces HTTPS connections
// - Referrer-Policy: Controls referrer information leakage
// - X-XSS-Protection: Legacy XSS protection (disabled by default per modern standards)
//
// CORS Features:
// - Configurable allowed origins with wildcard support
// - Method and header whitelisting
// - Credentials support control
// - Preflight request caching via Max-Age
// - Automatic OPTIONS request handling
//
// Architecture:
// Headers are applied in this order:
// 1. Check if infrastructure already set headers (if SkipIfInfraProvided)
// 2. Apply security headers (respecting OnlySetMissing flag)
// 3. Handle CORS headers and preflight requests
// 4. Pass through to underlying transport
//
// Usage:
// transport := NewSecurityHeadersTransport(baseTransport, config, logger, telemetry)
// Headers are automatically added to all HTTP responses through this transport.
package security

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/ui"
)

// SecurityHeadersTransport wraps any transport with security headers
type SecurityHeadersTransport struct {
	underlying ui.Transport
	config     SecurityHeadersConfig
	logger     core.Logger
	telemetry  core.Telemetry
}

type SecurityHeadersConfig struct {
	Enabled             bool              `json:"enabled"`
	OnlySetMissing      bool              `json:"only_set_missing"`
	Headers             map[string]string `json:"headers"`
	CORS                *CORSConfig       `json:"cors,omitempty"`
	SkipIfInfraProvided bool              `json:"skip_if_infra_provided"`
}

type CORSConfig struct {
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           int      `json:"max_age"`
}

// DefaultSecurityHeaders provides sensible defaults
func DefaultSecurityHeaders() map[string]string {
	return map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"X-XSS-Protection":          "0", // Modern browsers don't need this
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
	}
}

// NewSecurityHeadersTransport creates a transport with security headers
func NewSecurityHeadersTransport(transport ui.Transport, config SecurityHeadersConfig, logger core.Logger, telemetry core.Telemetry) ui.Transport {
	if !config.Enabled {
		return transport // Zero overhead when disabled
	}

	// Use defaults if no headers specified
	if config.Headers == nil {
		config.Headers = DefaultSecurityHeaders()
	}

	if logger != nil {
		logger.Info("Security headers enabled", map[string]interface{}{
			"transport":    transport.Name(),
			"header_count": len(config.Headers),
			"cors_enabled": config.CORS != nil,
		})
	}

	return &SecurityHeadersTransport{
		underlying: transport,
		config:     config,
		logger:     logger,
		telemetry:  telemetry,
	}
}

// Implement Transport interface
func (s *SecurityHeadersTransport) Name() string {
	return fmt.Sprintf("%s-secured", s.underlying.Name())
}

func (s *SecurityHeadersTransport) Description() string {
	return fmt.Sprintf("%s with security headers", s.underlying.Description())
}

func (s *SecurityHeadersTransport) Available() bool {
	return s.underlying.Available()
}

func (s *SecurityHeadersTransport) Priority() int {
	return s.underlying.Priority()
}

func (s *SecurityHeadersTransport) CreateHandler(agent ui.ChatAgent) http.Handler {
	originalHandler := s.underlying.CreateHandler(agent)

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Check if infrastructure already set security headers
		if s.config.SkipIfInfraProvided {
			hasInfraHeaders := w.Header().Get("X-Content-Type-Options") != "" ||
				w.Header().Get("X-Frame-Options") != ""
			if hasInfraHeaders {
				if s.logger != nil {
					s.logger.Debug("Security headers already set by infrastructure", map[string]interface{}{
						"transport": s.underlying.Name(),
					})
				}
				if s.telemetry != nil {
					s.telemetry.RecordMetric("security.headers.infra_bypass", 1, map[string]string{
						"transport": s.underlying.Name(),
					})
				}
				originalHandler.ServeHTTP(w, req)
				return
			}
		}

		// Apply security headers
		headersApplied := 0
		headersSkipped := 0
		for key, value := range s.config.Headers {
			if !s.config.OnlySetMissing || w.Header().Get(key) == "" {
				w.Header().Set(key, value)
				headersApplied++
			} else {
				headersSkipped++
			}
		}

		if s.telemetry != nil {
			if headersApplied > 0 {
				s.telemetry.RecordMetric("security.headers.applied", float64(headersApplied), map[string]string{
					"transport": s.underlying.Name(),
				})
			}
			if headersSkipped > 0 {
				s.telemetry.RecordMetric("security.headers.skipped", float64(headersSkipped), map[string]string{
					"transport": s.underlying.Name(),
				})
			}
		}

		// Handle CORS if configured
		if s.config.CORS != nil {
			if s.handleCORS(w, req) {
				// OPTIONS request was handled, don't call underlying handler
				return
			}
		}

		originalHandler.ServeHTTP(w, req)
	})
}

func (s *SecurityHeadersTransport) handleCORS(w http.ResponseWriter, req *http.Request) bool {
	origin := req.Header.Get("Origin")

	// Check if origin is allowed
	if s.isOriginAllowed(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)

		if s.config.CORS.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight
		if req.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(s.config.CORS.AllowedMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(s.config.CORS.AllowedHeaders, ", "))
			w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", s.config.CORS.MaxAge))

			if s.telemetry != nil {
				s.telemetry.RecordMetric("security.cors.preflight", 1, map[string]string{
					"transport": s.underlying.Name(),
					"allowed":   "true",
				})
			}

			// Write response for OPTIONS and return true to indicate request was handled
			w.WriteHeader(http.StatusOK)
			return true
		}

		if s.logger != nil && req.Method != "OPTIONS" {
			s.logger.Debug("CORS request allowed", map[string]interface{}{
				"transport": s.underlying.Name(),
				"origin":    origin,
			})
		}
		if s.telemetry != nil {
			s.telemetry.RecordMetric("security.cors.allowed", 1, map[string]string{
				"transport": s.underlying.Name(),
			})
		}
	} else if origin != "" {
		// Origin present but not allowed
		if s.logger != nil {
			s.logger.Warn("CORS request rejected", map[string]interface{}{
				"transport": s.underlying.Name(),
				"origin":    origin,
			})
		}
		if s.telemetry != nil {
			s.telemetry.RecordMetric("security.cors.rejected", 1, map[string]string{
				"transport": s.underlying.Name(),
			})
		}

		// For OPTIONS with rejected origin, still handle it and return
		if req.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return true
		}
	}

	return false
}

func (s *SecurityHeadersTransport) isOriginAllowed(origin string) bool {
	if len(s.config.CORS.AllowedOrigins) == 0 {
		return false
	}

	for _, allowed := range s.config.CORS.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
		// Support wildcard subdomains
		if strings.HasPrefix(allowed, "*.") {
			domain := allowed[1:] // Keep the dot: ".example.com"
			if strings.HasSuffix(origin, domain) && origin != "https://"+domain[1:] && origin != "http://"+domain[1:] {
				return true
			}
		}
	}
	return false
}

func (s *SecurityHeadersTransport) ClientExample() string {
	return s.underlying.ClientExample()
}

// Initialize initializes the underlying transport
func (s *SecurityHeadersTransport) Initialize(config ui.TransportConfig) error {
	return s.underlying.Initialize(config)
}

// Start starts the underlying transport
func (s *SecurityHeadersTransport) Start(ctx context.Context) error {
	return s.underlying.Start(ctx)
}

// Stop stops the underlying transport
func (s *SecurityHeadersTransport) Stop(ctx context.Context) error {
	return s.underlying.Stop(ctx)
}

// HealthCheck checks the health of the underlying transport
func (s *SecurityHeadersTransport) HealthCheck(ctx context.Context) error {
	return s.underlying.HealthCheck(ctx)
}

// Capabilities returns the underlying transport capabilities
func (s *SecurityHeadersTransport) Capabilities() []ui.TransportCapability {
	return s.underlying.Capabilities()
}
