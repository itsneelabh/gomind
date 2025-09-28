package ui

import (
	"time"

	"github.com/itsneelabh/gomind/core"
)

// DefaultSessionConfig returns the default configuration for session management
func DefaultSessionConfig() SessionConfig {
	return SessionConfig{
		TTL:             30 * time.Minute,
		MaxMessages:     50,
		MaxTokens:       4000,
		RateLimitWindow: time.Minute,
		RateLimitMax:    20,
		CleanupInterval: 5 * time.Minute,
	}
}

// DefaultTransportConfig returns the default configuration for transports
func DefaultTransportConfig() TransportConfig {
	return TransportConfig{
		MaxConnections: 1000,
		Timeout:        30 * time.Second,
		CORS: CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "Authorization"},
			MaxAge:         3600,
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 60,
			BurstSize:         10,
		},
	}
}

// NewChatAgentLogger creates a logger for UI components using core module's ProductionLogger pattern.
// This follows the Intelligent Configuration principle by providing smart defaults that work
// in both development and production environments.
//
// The logger uses the Layered Observability Architecture:
// - Layer 1: Console logging (immediate visibility, always works)
// - Layer 2: Metrics emission (production observability, when telemetry available)
// - Layer 3: Context correlation (distributed tracing integration)
func NewChatAgentLogger(config ChatAgentConfig) core.Logger {
	// Create logging configuration with intelligent defaults
	// Format detection: JSON for structured logging in production, text for development
	loggingConfig := core.LoggingConfig{
		Level:      "info",
		Format:     "json",  // Default to structured logging for production
		Output:     "stdout",
		TimeFormat: "2006-01-02T15:04:05.000Z07:00", // RFC3339 with milliseconds
	}

	// Development mode adjustments for better local experience
	developmentConfig := core.DevelopmentConfig{
		Enabled:      false, // Default to production mode
		DebugLogging: false,
		PrettyLogs:   false,
	}

	// If no explicit name provided, use intelligent default
	serviceName := config.Name
	if serviceName == "" {
		serviceName = "ui-agent"
	}

	// Create logger using core's proven ProductionLogger implementation
	// This maintains architectural compliance (ui → core) and reuses established patterns
	return core.NewProductionLogger(loggingConfig, developmentConfig, serviceName)
}

// NewChatAgentLoggerWithOptions creates a logger with custom logging configuration.
// This provides flexibility while maintaining the framework's intelligent defaults approach.
func NewChatAgentLoggerWithOptions(config ChatAgentConfig, loggingConfig core.LoggingConfig, devConfig core.DevelopmentConfig) core.Logger {
	serviceName := config.Name
	if serviceName == "" {
		serviceName = "ui-agent"
	}

	return core.NewProductionLogger(loggingConfig, devConfig, serviceName)
}
