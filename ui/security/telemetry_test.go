//go:build security
// +build security

package security

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/itsneelabh/gomind/core"
)

// MockTelemetry tracks telemetry calls for testing
type MockTelemetry struct {
	metrics map[string]float64
	labels  map[string]map[string]string
	spans   []string
}

func NewMockTelemetry() *MockTelemetry {
	return &MockTelemetry{
		metrics: make(map[string]float64),
		labels:  make(map[string]map[string]string),
		spans:   make([]string, 0),
	}
}

func (m *MockTelemetry) StartSpan(ctx context.Context, name string) (context.Context, core.Span) {
	m.spans = append(m.spans, name)
	return ctx, &MockSpan{}
}

func (m *MockTelemetry) RecordMetric(name string, value float64, labels map[string]string) {
	m.metrics[name] = m.metrics[name] + value
	m.labels[name] = labels
}

type MockSpan struct{}

func (s *MockSpan) End()                                       {}
func (s *MockSpan) SetAttribute(key string, value interface{}) {}
func (s *MockSpan) RecordError(err error)                      {}

// MockLogger tracks log calls for testing
type MockLogger struct {
	infoCount  int
	warnCount  int
	errorCount int
	debugCount int
	lastInfo   map[string]interface{}
	lastWarn   map[string]interface{}
	lastError  map[string]interface{}
	lastDebug  map[string]interface{}
}

func NewMockLogger() *MockLogger {
	return &MockLogger{}
}

func (l *MockLogger) Info(msg string, fields map[string]interface{}) {
	l.infoCount++
	l.lastInfo = fields
}

func (l *MockLogger) Error(msg string, fields map[string]interface{}) {
	l.errorCount++
	l.lastError = fields
}

func (l *MockLogger) Warn(msg string, fields map[string]interface{}) {
	l.warnCount++
	l.lastWarn = fields
}

func (l *MockLogger) Debug(msg string, fields map[string]interface{}) {
	l.debugCount++
	l.lastDebug = fields
}

func TestRateLimiterTelemetry(t *testing.T) {
	t.Run("logs and records metrics for rate limit rejection", func(t *testing.T) {
		mockTelemetry := NewMockTelemetry()
		mockLogger := NewMockLogger()

		// Create a limiter that always rejects
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 0, // Always reject
		}

		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		rateLimiter := NewInMemoryRateLimitTransport(mockTransport, config, mockLogger, mockTelemetry)
		handler := rateLimiter.CreateHandler(nil)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Check that rejection was logged
		if mockLogger.warnCount != 1 {
			t.Errorf("Expected 1 warning log, got %d", mockLogger.warnCount)
		}

		// Check that rejection metric was recorded
		if mockTelemetry.metrics["security.rate_limit.rejected"] != 1 {
			t.Errorf("Expected rate_limit.rejected metric to be 1, got %f", mockTelemetry.metrics["security.rate_limit.rejected"])
		}

		// Check response
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("Expected status 429, got %d", rec.Code)
		}
	})

	t.Run("records metrics for allowed requests", func(t *testing.T) {
		mockTelemetry := NewMockTelemetry()
		mockLogger := NewMockLogger()

		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 100, // High limit
		}

		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		rateLimiter := NewInMemoryRateLimitTransport(mockTransport, config, mockLogger, mockTelemetry)
		handler := rateLimiter.CreateHandler(nil)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Check that allowed metric was recorded
		if mockTelemetry.metrics["security.rate_limit.allowed"] != 1 {
			t.Errorf("Expected rate_limit.allowed metric to be 1, got %f", mockTelemetry.metrics["security.rate_limit.allowed"])
		}

		// Check response
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("logs infrastructure bypass", func(t *testing.T) {
		mockTelemetry := NewMockTelemetry()
		mockLogger := NewMockLogger()

		config := RateLimitConfig{
			Enabled:             true,
			RequestsPerMinute:   10,
			SkipIfInfraProvided: true,
		}

		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		rateLimiter := NewInMemoryRateLimitTransport(mockTransport, config, mockLogger, mockTelemetry)
		handler := rateLimiter.CreateHandler(nil)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-RateLimit-Limit", "100") // Infrastructure rate limiting
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Check that bypass was logged
		if mockLogger.debugCount != 1 {
			t.Errorf("Expected 1 debug log for bypass, got %d", mockLogger.debugCount)
		}

		// Check that bypass metric was recorded
		if mockTelemetry.metrics["security.rate_limit.infra_bypass"] != 1 {
			t.Errorf("Expected infra_bypass metric to be 1, got %f", mockTelemetry.metrics["security.rate_limit.infra_bypass"])
		}
	})
}

func TestSecurityHeadersTelemetry(t *testing.T) {
	t.Run("records metrics for applied headers", func(t *testing.T) {
		mockTelemetry := NewMockTelemetry()
		mockLogger := NewMockLogger()

		config := SecurityHeadersConfig{
			Enabled: true,
			Headers: map[string]string{
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options":        "DENY",
			},
		}

		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		headersTransport := NewSecurityHeadersTransport(mockTransport, config, mockLogger, mockTelemetry)
		handler := headersTransport.CreateHandler(nil)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Check that headers applied metric was recorded
		if mockTelemetry.metrics["security.headers.applied"] != 2 {
			t.Errorf("Expected headers.applied metric to be 2, got %f", mockTelemetry.metrics["security.headers.applied"])
		}

		// Check headers were set
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Error("X-Content-Type-Options header not set")
		}
		if rec.Header().Get("X-Frame-Options") != "DENY" {
			t.Error("X-Frame-Options header not set")
		}
	})

	t.Run("records CORS metrics", func(t *testing.T) {
		mockTelemetry := NewMockTelemetry()
		mockLogger := NewMockLogger()

		config := SecurityHeadersConfig{
			Enabled: true,
			CORS: &CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{"GET", "POST"},
			},
		}

		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		headersTransport := NewSecurityHeadersTransport(mockTransport, config, mockLogger, mockTelemetry)
		handler := headersTransport.CreateHandler(nil)

		// Test allowed origin
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Check CORS allowed metric
		if mockTelemetry.metrics["security.cors.allowed"] != 1 {
			t.Errorf("Expected cors.allowed metric to be 1, got %f", mockTelemetry.metrics["security.cors.allowed"])
		}

		// Test rejected origin
		req = httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://evil.com")
		rec = httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Check CORS rejected metric
		if mockTelemetry.metrics["security.cors.rejected"] != 1 {
			t.Errorf("Expected cors.rejected metric to be 1, got %f", mockTelemetry.metrics["security.cors.rejected"])
		}

		// Check warning was logged
		if mockLogger.warnCount != 1 {
			t.Errorf("Expected 1 warning for CORS rejection, got %d", mockLogger.warnCount)
		}
	})

	t.Run("records preflight metrics", func(t *testing.T) {
		mockTelemetry := NewMockTelemetry()
		mockLogger := NewMockLogger()

		config := SecurityHeadersConfig{
			Enabled: true,
			CORS: &CORSConfig{
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{"GET", "POST", "OPTIONS"},
			},
		}

		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Should not be called for OPTIONS
				t.Error("Handler should not be called for OPTIONS")
			}),
		}

		headersTransport := NewSecurityHeadersTransport(mockTransport, config, mockLogger, mockTelemetry)
		handler := headersTransport.CreateHandler(nil)

		req := httptest.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Check preflight metric
		if mockTelemetry.metrics["security.cors.preflight"] != 1 {
			t.Errorf("Expected cors.preflight metric to be 1, got %f", mockTelemetry.metrics["security.cors.preflight"])
		}

		// Check CORS headers
		if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
			t.Error("CORS origin header not set")
		}
		if rec.Header().Get("Access-Control-Allow-Methods") == "" {
			t.Error("CORS methods header not set")
		}
	})
}

func TestSmartTransportTelemetry(t *testing.T) {
	t.Run("logs infrastructure detection", func(t *testing.T) {
		mockTelemetry := NewMockTelemetry()
		mockLogger := NewMockLogger()

		config := SecurityConfig{
			Enabled:    true,
			AutoDetect: true,
			Logger:     mockLogger,
			Telemetry:  mockTelemetry,
		}

		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		wrapped := WithSecurity(mockTransport, config)

		// Should have logged smart security initialization
		if mockLogger.infoCount != 1 {
			t.Errorf("Expected 1 info log for smart security init, got %d", mockLogger.infoCount)
		}

		// Cast to SmartSecurityTransport to test
		smartTransport, ok := wrapped.(*SmartSecurityTransport)
		if !ok {
			t.Fatal("Expected SmartSecurityTransport")
		}

		handler := smartTransport.CreateHandler(nil)

		// Test with gateway headers
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Amzn-Trace-Id", "trace-123")
		req.Header.Set("X-RateLimit-Limit", "100")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Check that infrastructure detection was logged
		if mockLogger.infoCount != 2 { // 1 for init, 1 for detection
			t.Errorf("Expected 2 info logs, got %d", mockLogger.infoCount)
		}

		// Check metrics
		if mockTelemetry.metrics["security.smart.infra_detected"] != 1 {
			t.Errorf("Expected infra_detected metric to be 1, got %f", mockTelemetry.metrics["security.smart.infra_detected"])
		}
	})
}

func TestTelemetryNilSafety(t *testing.T) {
	t.Run("works with nil telemetry and logger", func(t *testing.T) {
		// Test that all components work correctly with nil telemetry/logger
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 10,
		}

		mockTransport := &MockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}

		// Create with nil logger and telemetry
		rateLimiter := NewInMemoryRateLimitTransport(mockTransport, config, nil, nil)
		handler := rateLimiter.CreateHandler(nil)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		// Should not panic
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})
}

// MockTransport is already defined in mock_test.go
