//go:build security
// +build security

package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkSecurityOverhead measures the overhead of security features
func BenchmarkSecurityOverhead(b *testing.B) {
	mockTransport := &MockTransport{
		name: "test",
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}

	b.Run("Baseline-NoSecurity", func(b *testing.B) {
		handler := mockTransport.CreateHandler(nil)
		req := httptest.NewRequest("GET", "/test", nil)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})

	b.Run("WithSecurityHeaders", func(b *testing.B) {
		config := SecurityHeadersConfig{
			Enabled: true,
			Headers: DefaultSecurityHeaders(),
		}

		wrapped := NewSecurityHeadersTransport(mockTransport, config, nil, nil)
		handler := wrapped.CreateHandler(nil)
		req := httptest.NewRequest("GET", "/test", nil)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})

	b.Run("WithRateLimiting", func(b *testing.B) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 1000000, // High limit to avoid blocking
		}

		wrapped := NewInMemoryRateLimitTransport(mockTransport, config, nil, nil)
		handler := wrapped.CreateHandler(nil)
		req := httptest.NewRequest("GET", "/test", nil)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})

	b.Run("WithSmartSecurity-NoInfra", func(b *testing.B) {
		config := SecurityConfig{
			Enabled:    true,
			AutoDetect: true,
			SecurityHeaders: &SecurityHeadersConfig{
				Enabled: true,
				Headers: DefaultSecurityHeaders(),
			},
		}

		detector := NewInfrastructureDetector()
		smartTransport := &SmartSecurityTransport{
			underlying: mockTransport,
			config:     config,
			detector:   detector,
		}

		handler := smartTransport.CreateHandler(nil)
		req := httptest.NewRequest("GET", "/test", nil)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})

	b.Run("WithSmartSecurity-InfraDetected", func(b *testing.B) {
		config := SecurityConfig{
			Enabled:    true,
			AutoDetect: true,
			SecurityHeaders: &SecurityHeadersConfig{
				Enabled: true,
				Headers: DefaultSecurityHeaders(),
			},
		}

		detector := NewInfrastructureDetector()
		smartTransport := &SmartSecurityTransport{
			underlying: mockTransport,
			config:     config,
			detector:   detector,
		}

		handler := smartTransport.CreateHandler(nil)
		req := httptest.NewRequest("GET", "/test", nil)
		// Add infrastructure headers to trigger bypass
		req.Header.Set("X-RateLimit-Limit", "100")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})

	b.Run("FullStack", func(b *testing.B) {
		config := SecurityConfig{
			Enabled:     true,
			AutoDetect:  false,
			ForceEnable: true,
			RateLimit: &RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 1000000,
			},
			SecurityHeaders: &SecurityHeadersConfig{
				Enabled: true,
				Headers: DefaultSecurityHeaders(),
				CORS: &CORSConfig{
					AllowedOrigins: []string{"*"},
				},
			},
		}

		wrapped := WithSecurity(mockTransport, config)
		handler := wrapped.CreateHandler(nil)
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://example.com")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})
}

// BenchmarkInfrastructureDetection measures detection performance
func BenchmarkInfrastructureDetection(b *testing.B) {
	detector := NewInfrastructureDetector()

	b.Run("NoHeaders", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/test", nil)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			detector.IsSecurityProvidedByInfra(req)
		}
	})

	b.Run("WithHeaders", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-RateLimit-Limit", "100")
		req.Header.Set("X-Amzn-Trace-Id", "trace")
		req.Header.Set("X-B3-TraceId", "b3trace")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			detector.IsSecurityProvidedByInfra(req)
		}
	})

	b.Run("RateLimitDetection", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-RateLimit-Limit", "100")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			detector.IsRateLimitingProvidedByInfra(req)
		}
	})

	b.Run("CORSDetection", func(b *testing.B) {
		headers := http.Header{
			"Access-Control-Allow-Origin": []string{"*"},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			detector.IsCORSProvidedByInfra(headers)
		}
	})
}

// BenchmarkConcurrentRequests measures performance under concurrent load
func BenchmarkConcurrentRequests(b *testing.B) {
	mockTransport := &MockTransport{
		name: "test",
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}

	config := SecurityConfig{
		Enabled: true,
		RateLimit: &RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 1000000,
		},
		SecurityHeaders: &SecurityHeadersConfig{
			Enabled: true,
			Headers: DefaultSecurityHeaders(),
		},
	}

	wrapped := WithSecurity(mockTransport, config)
	handler := wrapped.CreateHandler(nil)

	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest("GET", "/test", nil)
		for pb.Next() {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})
}

// BenchmarkMemoryAllocation measures memory allocations
func BenchmarkMemoryAllocation(b *testing.B) {
	mockTransport := &MockTransport{
		name: "test",
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}

	b.Run("NoSecurity", func(b *testing.B) {
		handler := mockTransport.CreateHandler(nil)
		req := httptest.NewRequest("GET", "/test", nil)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})

	b.Run("WithSecurity", func(b *testing.B) {
		config := SecurityConfig{
			Enabled: true,
			SecurityHeaders: &SecurityHeadersConfig{
				Enabled: true,
				Headers: DefaultSecurityHeaders(),
			},
		}

		wrapped := WithSecurity(mockTransport, config)
		handler := wrapped.CreateHandler(nil)
		req := httptest.NewRequest("GET", "/test", nil)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})
}
