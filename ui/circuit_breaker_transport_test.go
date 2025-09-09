package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// mockCircuitBreaker implements core.CircuitBreaker for testing
type mockCircuitBreaker struct {
	executeFunc        func(context.Context, func() error) error
	executeWithTimeout func(context.Context, time.Duration, func() error) error
	state              string
	metrics            map[string]interface{}
	canExecute         bool
	executeCalls       int
	resetCalls         int
}

func (m *mockCircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	m.executeCalls++
	if m.executeFunc != nil {
		return m.executeFunc(ctx, fn)
	}
	// Default: just execute the function
	return fn()
}

func (m *mockCircuitBreaker) ExecuteWithTimeout(ctx context.Context, timeout time.Duration, fn func() error) error {
	if m.executeWithTimeout != nil {
		return m.executeWithTimeout(ctx, timeout, fn)
	}
	return m.Execute(ctx, fn)
}

func (m *mockCircuitBreaker) GetState() string {
	if m.state == "" {
		return "closed"
	}
	return m.state
}

func (m *mockCircuitBreaker) GetMetrics() map[string]interface{} {
	if m.metrics == nil {
		return map[string]interface{}{"calls": m.executeCalls}
	}
	return m.metrics
}

func (m *mockCircuitBreaker) Reset() {
	m.resetCalls++
	m.state = "closed"
}

func (m *mockCircuitBreaker) CanExecute() bool {
	return m.canExecute
}

// mockTransport for testing
type mockTransport struct {
	name        string
	available   bool
	handler     http.Handler
	healthError error
}

func (m *mockTransport) Name() string                     { return m.name }
func (m *mockTransport) Description() string              { return "mock transport" }
func (m *mockTransport) Available() bool                  { return m.available }
func (m *mockTransport) Priority() int                    { return 100 }
func (m *mockTransport) Initialize(TransportConfig) error { return nil }
func (m *mockTransport) Start(context.Context) error      { return nil }
func (m *mockTransport) Stop(context.Context) error       { return nil }
func (m *mockTransport) CreateHandler(ChatAgent) http.Handler {
	if m.handler != nil {
		return m.handler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
}
func (m *mockTransport) HealthCheck(ctx context.Context) error {
	return m.healthError
}
func (m *mockTransport) Capabilities() []TransportCapability {
	return []TransportCapability{CapabilityStreaming}
}
func (m *mockTransport) ClientExample() string {
	return "mock example"
}

func TestInterfaceBasedCircuitBreakerTransport(t *testing.T) {
	t.Run("wraps transport with circuit breaker", func(t *testing.T) {
		mockTransport := &mockTransport{
			name:      "test",
			available: true,
		}

		mockBreaker := &mockCircuitBreaker{
			canExecute: true,
		}

		cbTransport := NewInterfaceBasedCircuitBreakerTransport(mockTransport, mockBreaker)

		// Check name includes suffix
		if cbTransport.Name() != "test-cb" {
			t.Errorf("Expected name 'test-cb', got '%s'", cbTransport.Name())
		}

		// Check description mentions circuit breaker
		desc := cbTransport.Description()
		if desc != "mock transport (with circuit breaker protection)" {
			t.Errorf("Unexpected description: %s", desc)
		}
	})

	t.Run("handler executes through circuit breaker", func(t *testing.T) {
		executionCount := 0
		mockTransport := &mockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				executionCount++
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			}),
		}

		mockBreaker := &mockCircuitBreaker{
			canExecute: true,
		}

		cbTransport := NewInterfaceBasedCircuitBreakerTransport(mockTransport, mockBreaker)
		handler := cbTransport.CreateHandler(nil)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Check handler was called
		if executionCount != 1 {
			t.Errorf("Expected handler to be called once, got %d", executionCount)
		}

		// Check circuit breaker was used
		if mockBreaker.executeCalls != 1 {
			t.Errorf("Expected circuit breaker Execute to be called once, got %d", mockBreaker.executeCalls)
		}

		// Check response
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		if rec.Body.String() != "success" {
			t.Errorf("Expected body 'success', got '%s'", rec.Body.String())
		}
	})

	t.Run("returns 503 when circuit is open", func(t *testing.T) {
		mockTransport := &mockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("Handler should not be called when circuit is open")
			}),
		}

		mockBreaker := &mockCircuitBreaker{
			canExecute: false,
			executeFunc: func(ctx context.Context, fn func() error) error {
				return core.ErrCircuitBreakerOpen
			},
		}

		cbTransport := NewInterfaceBasedCircuitBreakerTransport(mockTransport, mockBreaker)
		handler := cbTransport.CreateHandler(nil)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Check response
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected status 503, got %d", rec.Code)
		}

		// Check circuit breaker header
		if rec.Header().Get("X-Circuit-Breaker") != "open" {
			t.Errorf("Expected X-Circuit-Breaker header to be 'open', got '%s'", rec.Header().Get("X-Circuit-Breaker"))
		}
	})

	t.Run("treats 5xx as failures", func(t *testing.T) {
		var capturedError error
		mockTransport := &mockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}),
		}

		mockBreaker := &mockCircuitBreaker{
			canExecute: true,
			executeFunc: func(ctx context.Context, fn func() error) error {
				err := fn()
				capturedError = err
				return err
			},
		}

		cbTransport := NewInterfaceBasedCircuitBreakerTransport(mockTransport, mockBreaker)
		handler := cbTransport.CreateHandler(nil)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Check error was captured
		if capturedError == nil {
			t.Error("Expected error for 500 response")
		} else if capturedError.Error() != "server error: HTTP 500" {
			t.Errorf("Unexpected error: %v", capturedError)
		}
	})

	t.Run("does not treat 4xx as failures", func(t *testing.T) {
		var capturedError error
		mockTransport := &mockTransport{
			name: "test",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			}),
		}

		mockBreaker := &mockCircuitBreaker{
			canExecute: true,
			executeFunc: func(ctx context.Context, fn func() error) error {
				err := fn()
				capturedError = err
				return err
			},
		}

		cbTransport := NewInterfaceBasedCircuitBreakerTransport(mockTransport, mockBreaker)
		handler := cbTransport.CreateHandler(nil)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Check no error for client error
		if capturedError != nil {
			t.Errorf("Expected no error for 400 response, got: %v", capturedError)
		}

		// Check response passed through
		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("availability depends on circuit state", func(t *testing.T) {
		mockTransport := &mockTransport{
			name:      "test",
			available: true,
		}

		mockBreaker := &mockCircuitBreaker{
			canExecute: false, // Circuit is open
		}

		cbTransport := NewInterfaceBasedCircuitBreakerTransport(mockTransport, mockBreaker)

		// Should be unavailable when circuit is open
		if cbTransport.Available() {
			t.Error("Expected transport to be unavailable when circuit is open")
		}

		// Now close the circuit
		mockBreaker.canExecute = true

		// Should be available when circuit is closed
		if !cbTransport.Available() {
			t.Error("Expected transport to be available when circuit is closed")
		}
	})

	t.Run("health check uses circuit breaker", func(t *testing.T) {
		mockTransport := &mockTransport{
			name:        "test",
			healthError: nil,
		}

		mockBreaker := &mockCircuitBreaker{
			canExecute: true,
		}

		cbTransport := NewInterfaceBasedCircuitBreakerTransport(mockTransport, mockBreaker)

		ctx := context.Background()
		err := cbTransport.HealthCheck(ctx)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		// Check circuit breaker was used
		if mockBreaker.executeCalls != 1 {
			t.Errorf("Expected circuit breaker Execute to be called once, got %d", mockBreaker.executeCalls)
		}
	})

	t.Run("exposes circuit breaker state and metrics", func(t *testing.T) {
		mockTransport := &mockTransport{name: "test"}

		mockBreaker := &mockCircuitBreaker{
			state: "half-open",
			metrics: map[string]interface{}{
				"failures":  5,
				"successes": 10,
			},
		}

		cbTransport := NewInterfaceBasedCircuitBreakerTransport(mockTransport, mockBreaker).(*InterfaceBasedCircuitBreakerTransport)

		// Check state
		if cbTransport.GetCircuitBreakerState() != "half-open" {
			t.Errorf("Expected state 'half-open', got '%s'", cbTransport.GetCircuitBreakerState())
		}

		// Check metrics
		metrics := cbTransport.GetCircuitBreakerMetrics()
		if metrics["failures"] != 5 {
			t.Errorf("Expected failures=5, got %v", metrics["failures"])
		}
		if metrics["successes"] != 10 {
			t.Errorf("Expected successes=10, got %v", metrics["successes"])
		}
	})
}

func TestCircuitBreakerIntegration(t *testing.T) {
	t.Run("chat agent uses injected circuit breaker", func(t *testing.T) {
		config := DefaultChatAgentConfig("test-agent")
		config.CircuitBreakerEnabled = true

		// Use the mock session manager from test file
		sessions := NewMockSessionManager(DefaultSessionConfig())

		mockBreaker := &mockCircuitBreaker{
			canExecute: true,
		}

		deps := ChatAgentDependencies{
			CircuitBreaker: mockBreaker,
			Logger:         &core.NoOpLogger{},
			Telemetry:      &core.NoOpTelemetry{},
		}

		agent := NewChatAgentWithDependencies(config, sessions, deps)

		// Check circuit breaker was injected
		if agent.circuitBreaker != mockBreaker {
			t.Error("Circuit breaker was not properly injected")
		}
	})

	t.Run("functional options work", func(t *testing.T) {
		config := DefaultChatAgentConfig("test-agent")
		sessions := NewMockSessionManager(DefaultSessionConfig())

		mockBreaker := &mockCircuitBreaker{
			canExecute: true,
		}

		agent := NewChatAgentWithOptions(
			config,
			sessions,
			WithCircuitBreaker(mockBreaker),
		)

		// Check circuit breaker was set
		if agent.circuitBreaker != mockBreaker {
			t.Error("Circuit breaker was not set via functional option")
		}
	})
}
