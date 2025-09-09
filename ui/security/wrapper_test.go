//go:build security
// +build security

package security

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestWithSecurity(t *testing.T) {
	t.Run("Disabled config returns original transport", func(t *testing.T) {
		mockTransport := &MockTransport{
			name: "test",
		}

		config := SecurityConfig{
			Enabled: false,
		}

		wrapped := WithSecurity(mockTransport, config)

		if wrapped != mockTransport {
			t.Error("Disabled security should return original transport")
		}
	})

	t.Run("Smart security with auto-detect", func(t *testing.T) {
		mockTransport := &MockTransport{
			name: "test",
		}

		config := SecurityConfig{
			Enabled:    true,
			AutoDetect: true,
			RateLimit: &RateLimitConfig{
				Enabled: true,
			},
			SecurityHeaders: &SecurityHeadersConfig{
				Enabled: true,
			},
		}

		wrapped := WithSecurity(mockTransport, config)

		// Should return SmartSecurityTransport
		if _, ok := wrapped.(*SmartSecurityTransport); !ok {
			t.Error("Should return SmartSecurityTransport when AutoDetect is true")
		}
	})

	t.Run("Layered security without auto-detect", func(t *testing.T) {
		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		config := SecurityConfig{
			Enabled:     true,
			AutoDetect:  false,
			ForceEnable: true,
			RateLimit: &RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 10,
			},
			SecurityHeaders: &SecurityHeadersConfig{
				Enabled: true,
				Headers: map[string]string{
					"X-Security": "enabled",
				},
			},
		}

		wrapped := WithSecurity(mockTransport, config)

		// Should apply both rate limiting and security headers
		// The wrapped transport should be SecurityHeadersTransport wrapping RateLimitTransport
		if _, ok := wrapped.(*SecurityHeadersTransport); !ok {
			t.Error("Should have SecurityHeadersTransport as outer wrapper")
		}
	})

	t.Run("Rate limiting only", func(t *testing.T) {
		mockTransport := &MockTransport{
			name: "test",
		}

		config := SecurityConfig{
			Enabled: true,
			RateLimit: &RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 100,
			},
			// No security headers
		}

		wrapped := WithSecurity(mockTransport, config)

		// Should only wrap with rate limiting
		if _, ok := wrapped.(*RateLimitTransport); !ok {
			// Could also be InMemoryRateLimitTransport wrapper
			if wrapped == mockTransport {
				t.Error("Should wrap transport when rate limiting is enabled")
			}
		}
	})

	t.Run("Security headers only", func(t *testing.T) {
		mockTransport := &MockTransport{
			name: "test",
		}

		config := SecurityConfig{
			Enabled: true,
			SecurityHeaders: &SecurityHeadersConfig{
				Enabled: true,
				Headers: DefaultSecurityHeaders(),
			},
			// No rate limiting
		}

		wrapped := WithSecurity(mockTransport, config)

		// Should only wrap with security headers
		if _, ok := wrapped.(*SecurityHeadersTransport); !ok {
			t.Error("Should wrap with SecurityHeadersTransport when headers enabled")
		}
	})
}

func TestDefaultSecurityConfig(t *testing.T) {
	config := DefaultSecurityConfig()

	if !config.Enabled {
		t.Error("Default config should be enabled")
	}

	if !config.AutoDetect {
		t.Error("Default config should have AutoDetect enabled")
	}

	if config.ForceEnable {
		t.Error("Default config should not force enable")
	}

	if config.RateLimit == nil || !config.RateLimit.Enabled {
		t.Error("Default config should have rate limiting enabled")
	}

	if config.RateLimit.RequestsPerMinute != 60 {
		t.Errorf("Default rate limit should be 60/min, got %d", config.RateLimit.RequestsPerMinute)
	}

	if config.SecurityHeaders == nil || !config.SecurityHeaders.Enabled {
		t.Error("Default config should have security headers enabled")
	}

	if config.SecurityHeaders.CORS == nil {
		t.Error("Default config should include CORS configuration")
	}
}

func TestIsSecurityEnabled(t *testing.T) {
	// This test verifies the build tag is working
	if !IsSecurityEnabled() {
		t.Error("IsSecurityEnabled() should return true when built with security tag")
	}
}

func TestEndToEndSecurity(t *testing.T) {
	// This test simulates a real request through the full security stack
	mockTransport := &MockTransport{
		name: "test",
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("OK"))
		}),
	}

	config := DefaultSecurityConfig()
	config.AutoDetect = false // Force security to be applied
	config.ForceEnable = true

	wrapped := WithSecurity(mockTransport, config)
	handler := wrapped.CreateHandler(nil)

	// Test multiple requests to trigger rate limiting
	for i := 0; i < 70; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if i < 60 {
			// First 60 requests should succeed
			if rec.Code != http.StatusOK {
				t.Errorf("Request %d should succeed, got status %d", i+1, rec.Code)
			}

			// Check security headers are present
			if i == 0 {
				for key := range DefaultSecurityHeaders() {
					if rec.Header().Get(key) == "" {
						t.Errorf("Security header %s not set", key)
					}
				}
			}
		} else {
			// Requests after 60 should be rate limited
			if rec.Code != http.StatusTooManyRequests {
				t.Errorf("Request %d should be rate limited, got status %d", i+1, rec.Code)
			}
		}
	}
}

func TestSecurityWithEnvironmentDetection(t *testing.T) {
	tests := []struct {
		name           string
		envVars        map[string]string
		requestHeaders map[string]string
		expectSecurity bool
		description    string
	}{
		{
			name:           "No infrastructure",
			envVars:        map[string]string{},
			requestHeaders: map[string]string{},
			expectSecurity: true,
			description:    "Should apply security when no infrastructure detected",
		},
		{
			name: "API Gateway detected",
			envVars: map[string]string{
				"API_GATEWAY_ENABLED": "true",
			},
			requestHeaders: map[string]string{},
			expectSecurity: false,
			description:    "Should skip security when API Gateway detected",
		},
		{
			name:    "Rate limiting in headers",
			envVars: map[string]string{},
			requestHeaders: map[string]string{
				"X-RateLimit-Limit": "100",
			},
			expectSecurity: true, // Security headers should still be applied when only rate limiting is provided
			description:    "Should apply security headers when only rate limiting provided by infra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			mockTransport := &MockTransport{
				name: "test",
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}),
			}

			config := SecurityConfig{
				Enabled:    true,
				AutoDetect: true,
				SecurityHeaders: &SecurityHeadersConfig{
					Enabled: true,
					Headers: map[string]string{
						"X-Test-Security": "applied",
					},
				},
			}

			wrapped := WithSecurity(mockTransport, config)
			handler := wrapped.CreateHandler(nil)

			req := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tt.requestHeaders {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			securityApplied := rec.Header().Get("X-Test-Security") != ""

			if securityApplied != tt.expectSecurity {
				t.Errorf("%s: security applied = %v, want %v",
					tt.description, securityApplied, tt.expectSecurity)
			}
		})
	}
}
