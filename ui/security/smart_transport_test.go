//go:build security
// +build security

package security

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/itsneelabh/gomind/ui"
)

func TestSmartSecurityTransport(t *testing.T) {
	t.Run("Applies security when no infrastructure detected", func(t *testing.T) {
		// Ensure no infrastructure env vars
		os.Unsetenv("API_GATEWAY_ENABLED")
		os.Unsetenv("ISTIO_PROXY")

		mockTransport := &MockTransport{
			name:        "test",
			description: "Test transport",
			available:   true,
			priority:    100,
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
					"X-Custom-Security": "enabled",
				},
			},
		}

		detector := NewInfrastructureDetector()
		smartTransport := &SmartSecurityTransport{
			underlying: mockTransport,
			config:     config,
			detector:   detector,
		}

		handler := smartTransport.CreateHandler(nil)

		// Test request without infrastructure headers
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Security headers should be applied
		if got := rec.Header().Get("X-Custom-Security"); got != "enabled" {
			t.Errorf("Security header not applied, got: %s", got)
		}
	})

	t.Run("Skips security when infrastructure detected", func(t *testing.T) {
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
					"X-Custom-Security": "enabled",
				},
			},
		}

		detector := NewInfrastructureDetector()
		smartTransport := &SmartSecurityTransport{
			underlying: mockTransport,
			config:     config,
			detector:   detector,
		}

		handler := smartTransport.CreateHandler(nil)

		// Test request WITH infrastructure headers
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-RateLimit-Limit", "100") // Infrastructure rate limiting
		req.Header.Set("X-Amzn-Trace-Id", "trace") // AWS API Gateway
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Security headers should NOT be applied
		if got := rec.Header().Get("X-Custom-Security"); got != "" {
			t.Errorf("Security header should not be applied when infrastructure detected, got: %s", got)
		}
	})

	t.Run("CORS handling with OPTIONS", func(t *testing.T) {
		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Should not reach here for OPTIONS
				t.Error("Handler called for OPTIONS request")
			}),
		}

		config := SecurityConfig{
			Enabled:    true,
			AutoDetect: true,
			SecurityHeaders: &SecurityHeadersConfig{
				Enabled: true,
				CORS: &CORSConfig{
					AllowedOrigins:   []string{"https://example.com"},
					AllowedMethods:   []string{"GET", "POST"},
					AllowedHeaders:   []string{"Content-Type"},
					AllowCredentials: true,
				},
			},
		}

		detector := NewInfrastructureDetector()
		smartTransport := &SmartSecurityTransport{
			underlying: mockTransport,
			config:     config,
			detector:   detector,
		}

		handler := smartTransport.CreateHandler(nil)

		// OPTIONS preflight request
		req := httptest.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Check CORS headers
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
			t.Errorf("Access-Control-Allow-Origin = %s, want https://example.com", got)
		}

		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Errorf("Access-Control-Allow-Credentials = %s, want true", got)
		}

		if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
			t.Errorf("Access-Control-Allow-Methods = %s, want GET, POST, OPTIONS", got)
		}

		if rec.Code != http.StatusOK {
			t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("Metadata preservation", func(t *testing.T) {
		mockTransport := &MockTransport{
			name:        "original",
			description: "Original transport",
			available:   true,
			priority:    100,
		}

		config := SecurityConfig{
			Enabled:    true,
			AutoDetect: true,
		}

		detector := NewInfrastructureDetector()
		smartTransport := &SmartSecurityTransport{
			underlying: mockTransport,
			config:     config,
			detector:   detector,
		}

		// Name should be modified
		expectedName := "original-smart-security"
		if got := smartTransport.Name(); got != expectedName {
			t.Errorf("Name() = %s, want %s", got, expectedName)
		}

		// Description should be modified
		expectedDesc := "Original transport with smart security detection"
		if got := smartTransport.Description(); got != expectedDesc {
			t.Errorf("Description() = %s, want %s", got, expectedDesc)
		}

		// These should be preserved
		if got := smartTransport.Available(); got != mockTransport.available {
			t.Errorf("Available() = %v, want %v", got, mockTransport.available)
		}

		if got := smartTransport.Priority(); got != mockTransport.priority {
			t.Errorf("Priority() = %d, want %d", got, mockTransport.priority)
		}
	})

	t.Run("Lifecycle methods delegated", func(t *testing.T) {
		mockTransport := &MockTransport{
			name: "test",
		}

		config := SecurityConfig{
			Enabled:    true,
			AutoDetect: true,
		}

		detector := NewInfrastructureDetector()
		smartTransport := &SmartSecurityTransport{
			underlying: mockTransport,
			config:     config,
			detector:   detector,
		}

		// Test Initialize
		err := smartTransport.Initialize(ui.TransportConfig{})
		if err != nil {
			t.Errorf("Initialize() error = %v", err)
		}
		if !mockTransport.initialized {
			t.Error("Initialize not delegated to underlying transport")
		}

		// Test Start
		ctx := context.Background()
		err = smartTransport.Start(ctx)
		if err != nil {
			t.Errorf("Start() error = %v", err)
		}
		if !mockTransport.started {
			t.Error("Start not delegated to underlying transport")
		}

		// Test Stop
		err = smartTransport.Stop(ctx)
		if err != nil {
			t.Errorf("Stop() error = %v", err)
		}
		if !mockTransport.stopped {
			t.Error("Stop not delegated to underlying transport")
		}

		// Test HealthCheck
		err = smartTransport.HealthCheck(ctx)
		if err != nil {
			t.Errorf("HealthCheck() error = %v", err)
		}
		if mockTransport.healthChecks != 1 {
			t.Errorf("HealthCheck not delegated, count = %d", mockTransport.healthChecks)
		}
	})

	t.Run("Selective feature application", func(t *testing.T) {
		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		config := SecurityConfig{
			Enabled:    true,
			AutoDetect: true,
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

		detector := NewInfrastructureDetector()
		smartTransport := &SmartSecurityTransport{
			underlying: mockTransport,
			config:     config,
			detector:   detector,
		}

		handler := smartTransport.CreateHandler(nil)

		// Test with partial infrastructure (rate limiting provided, but not CORS)
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-RateLimit-Limit", "100") // Infrastructure provides rate limiting
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Security headers should still be applied (CORS not provided by infra)
		if got := rec.Header().Get("X-Security"); got != "enabled" {
			t.Errorf("Security headers should be applied when not provided by infra, got: %s", got)
		}
	})
}
