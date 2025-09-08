package ui

import "time"

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

