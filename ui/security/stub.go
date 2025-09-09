//go:build !security
// +build !security

// Package security provides zero-overhead stubs when security features are not compiled in.
// To enable security features, build with: go build -tags security
package security

import (
	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/ui"
)

// SecurityConfig stub - all fields are no-ops when security is disabled
type SecurityConfig struct {
	Enabled             bool                   `json:"enabled"`
	AutoDetect          bool                   `json:"auto_detect"`
	ForceEnable         bool                   `json:"force_enable"`
	RateLimit           *RateLimitConfig       `json:"rate_limit,omitempty"`
	SecurityHeaders     *SecurityHeadersConfig `json:"security_headers,omitempty"`
	RedisClientProvider func() RedisClient     `json:"-"`

	// Redis configuration for distributed features
	RedisURL      string `json:"redis_url,omitempty" env:"REDIS_URL"`
	ForceInMemory bool   `json:"force_in_memory,omitempty"`

	// Telemetry and logging (optional) - no-ops when security disabled
	Logger    core.Logger    `json:"-"`
	Telemetry core.Telemetry `json:"-"`
}

// RateLimitConfig stub
type RateLimitConfig struct {
	Enabled             bool `json:"enabled"`
	RequestsPerMinute   int  `json:"requests_per_minute"`
	BurstSize           int  `json:"burst_size"`
	SkipIfInfraProvided bool `json:"skip_if_infra_provided"`
}

// SecurityHeadersConfig stub
type SecurityHeadersConfig struct {
	Enabled             bool              `json:"enabled"`
	OnlySetMissing      bool              `json:"only_set_missing"`
	Headers             map[string]string `json:"headers"`
	CORS                *CORSConfig       `json:"cors,omitempty"`
	SkipIfInfraProvided bool              `json:"skip_if_infra_provided"`
}

// CORSConfig stub
type CORSConfig struct {
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           int      `json:"max_age"`
}

// RedisClient stub interface
type RedisClient interface {
	// Empty interface when security is disabled
}

// WithSecurity returns the transport UNCHANGED when security is disabled
// This ensures ZERO overhead - no wrapper objects, no function calls, nothing
func WithSecurity(transport ui.Transport, config SecurityConfig) ui.Transport {
	return transport // Direct return - zero overhead
}

// DefaultSecurityConfig returns a disabled config
// Users can still configure it, but it will have no effect without the build tag
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		Enabled:    false, // Disabled by default when build tag not present
		AutoDetect: false,
	}
}

// DefaultSecurityHeaders returns nil when security is disabled
func DefaultSecurityHeaders() map[string]string {
	return nil
}

// IsSecurityEnabled returns false when the security build tag is not present
func IsSecurityEnabled() bool {
	return false
}
