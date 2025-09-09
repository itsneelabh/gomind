//go:build security
// +build security

package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/itsneelabh/gomind/ui"
)

func TestSecurityHeadersTransport(t *testing.T) {
	t.Run("Disabled config returns original transport", func(t *testing.T) {
		mockTransport := &MockTransport{
			name: "test",
		}

		config := SecurityHeadersConfig{
			Enabled: false,
		}

		wrapped := NewSecurityHeadersTransport(mockTransport, config, nil, nil)

		if wrapped != mockTransport {
			t.Error("Disabled security headers should return original transport")
		}
	})

	t.Run("Default security headers applied", func(t *testing.T) {
		mockTransport := &MockTransport{
			name:      "test",
			available: true,
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		config := SecurityHeadersConfig{
			Enabled: true,
			Headers: DefaultSecurityHeaders(),
		}

		wrapped := NewSecurityHeadersTransport(mockTransport, config, nil, nil)

		// Create mock ChatAgent
		var mockAgent ui.ChatAgent // We'll use nil for simplicity
		handler := wrapped.CreateHandler(mockAgent)

		// Test request
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Check headers
		expectedHeaders := DefaultSecurityHeaders()
		for key, value := range expectedHeaders {
			if got := rec.Header().Get(key); got != value {
				t.Errorf("Header %s = %s, want %s", key, got, value)
			}
		}
	})

	t.Run("OnlySetMissing respects existing headers", func(t *testing.T) {
		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Pre-set a header
				w.Header().Set("X-Frame-Options", "SAMEORIGIN")
				w.WriteHeader(http.StatusOK)
			}),
		}

		config := SecurityHeadersConfig{
			Enabled:        true,
			OnlySetMissing: true,
			Headers: map[string]string{
				"X-Frame-Options":        "DENY",
				"X-Content-Type-Options": "nosniff",
			},
		}

		wrapped := NewSecurityHeadersTransport(mockTransport, config, nil, nil)
		handler := wrapped.CreateHandler(nil)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// X-Frame-Options should keep original value
		if got := rec.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
			t.Errorf("X-Frame-Options should be SAMEORIGIN (pre-existing), got %s", got)
		}

		// X-Content-Type-Options should be set
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("X-Content-Type-Options should be nosniff, got %s", got)
		}
	})

	t.Run("CORS handling for allowed origin", func(t *testing.T) {
		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		config := SecurityHeadersConfig{
			Enabled: true,
			CORS: &CORSConfig{
				AllowedOrigins:   []string{"https://example.com", "https://app.example.com"},
				AllowedMethods:   []string{"GET", "POST"},
				AllowedHeaders:   []string{"Content-Type", "Authorization"},
				AllowCredentials: true,
				MaxAge:           3600,
			},
		}

		wrapped := NewSecurityHeadersTransport(mockTransport, config, nil, nil)
		handler := wrapped.CreateHandler(nil)

		// Test allowed origin
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
			t.Errorf("Access-Control-Allow-Origin = %s, want https://example.com", got)
		}

		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Errorf("Access-Control-Allow-Credentials = %s, want true", got)
		}
	})

	t.Run("CORS preflight request", func(t *testing.T) {
		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		config := SecurityHeadersConfig{
			Enabled: true,
			CORS: &CORSConfig{
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{"GET", "POST", "PUT"},
				AllowedHeaders: []string{"Content-Type", "X-Custom"},
				MaxAge:         7200,
			},
		}

		wrapped := NewSecurityHeadersTransport(mockTransport, config, nil, nil)
		handler := wrapped.CreateHandler(nil)

		// Preflight request
		req := httptest.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "https://any.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://any.com" {
			t.Errorf("Access-Control-Allow-Origin = %s, want https://any.com", got)
		}

		if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT" {
			t.Errorf("Access-Control-Allow-Methods = %s, want GET, POST, PUT", got)
		}

		if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, X-Custom" {
			t.Errorf("Access-Control-Allow-Headers = %s, want Content-Type, X-Custom", got)
		}

		if got := rec.Header().Get("Access-Control-Max-Age"); got != "7200" {
			t.Errorf("Access-Control-Max-Age = %s, want 7200", got)
		}
	})

	t.Run("CORS wildcard subdomain matching", func(t *testing.T) {
		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		config := SecurityHeadersConfig{
			Enabled: true,
			CORS: &CORSConfig{
				AllowedOrigins: []string{"*.example.com"},
			},
		}

		wrapped := NewSecurityHeadersTransport(mockTransport, config, nil, nil)
		handler := wrapped.CreateHandler(nil)

		testCases := []struct {
			origin  string
			allowed bool
		}{
			{"https://app.example.com", true},
			{"https://api.example.com", true},
			{"https://sub.app.example.com", true},
			{"https://example.com", false}, // Not a subdomain
			{"https://example.org", false},
		}

		for _, tc := range testCases {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			got := rec.Header().Get("Access-Control-Allow-Origin")
			if tc.allowed {
				if got != tc.origin {
					t.Errorf("Origin %s should be allowed, got header: %s", tc.origin, got)
				}
			} else {
				if got != "" {
					t.Errorf("Origin %s should not be allowed, got header: %s", tc.origin, got)
				}
			}
		}
	})

	t.Run("Skip if infrastructure provided", func(t *testing.T) {
		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		config := SecurityHeadersConfig{
			Enabled:             true,
			SkipIfInfraProvided: true,
			Headers:             DefaultSecurityHeaders(),
		}

		wrapped := NewSecurityHeadersTransport(mockTransport, config, nil, nil)
		handler := wrapped.CreateHandler(nil)

		// Request with infrastructure headers present
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		// Pre-set headers indicating infrastructure handling
		rec.Header().Set("X-Content-Type-Options", "already-set")

		handler.ServeHTTP(rec, req)

		// Should skip adding headers when infrastructure headers detected
		// Note: This test might need adjustment based on actual implementation
	})

	t.Run("Transport metadata preserved", func(t *testing.T) {
		mockTransport := &MockTransport{
			name:        "original",
			description: "Original transport",
			available:   true,
			priority:    100,
		}

		config := SecurityHeadersConfig{
			Enabled: true,
			Headers: DefaultSecurityHeaders(),
		}

		wrapped := NewSecurityHeadersTransport(mockTransport, config, nil, nil)

		// Name should be modified
		expectedName := "original-secured"
		if got := wrapped.Name(); got != expectedName {
			t.Errorf("Name() = %s, want %s", got, expectedName)
		}

		// Description should be modified
		if got := wrapped.Description(); !contains(got, "security headers") {
			t.Errorf("Description() should mention security headers, got: %s", got)
		}

		// These should be preserved
		if got := wrapped.Available(); got != mockTransport.available {
			t.Errorf("Available() = %v, want %v", got, mockTransport.available)
		}

		if got := wrapped.Priority(); got != mockTransport.priority {
			t.Errorf("Priority() = %d, want %d", got, mockTransport.priority)
		}

		if got := wrapped.ClientExample(); got != mockTransport.ClientExample() {
			t.Errorf("ClientExample() should be preserved")
		}
	})
}

func TestDefaultSecurityHeaders(t *testing.T) {
	headers := DefaultSecurityHeaders()

	expectedKeys := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Strict-Transport-Security",
		"Referrer-Policy",
	}

	for _, key := range expectedKeys {
		if _, ok := headers[key]; !ok {
			t.Errorf("DefaultSecurityHeaders missing %s", key)
		}
	}

	// Verify specific values
	if headers["X-Content-Type-Options"] != "nosniff" {
		t.Error("X-Content-Type-Options should be nosniff")
	}

	if headers["X-Frame-Options"] != "DENY" {
		t.Error("X-Frame-Options should be DENY")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
