package ui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// MockCircuitBreaker implements core.CircuitBreaker for testing
type MockCircuitBreaker struct {
	state          string
	executeFunc    func(context.Context, func() error) error
	canExecuteFunc func() bool
	metrics        map[string]interface{}
	mu             sync.Mutex
}

func NewMockCircuitBreaker() *MockCircuitBreaker {
	return &MockCircuitBreaker{
		state: "closed",
		metrics: map[string]interface{}{
			"requests": 0,
			"failures": 0,
			"success":  0,
		},
	}
}

func (m *MockCircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.executeFunc != nil {
		return m.executeFunc(ctx, fn)
	}

	// Default implementation
	if m.state == "open" {
		return core.ErrCircuitBreakerOpen
	}

	m.metrics["requests"] = m.metrics["requests"].(int) + 1

	err := fn()
	if err != nil {
		m.metrics["failures"] = m.metrics["failures"].(int) + 1
	} else {
		m.metrics["success"] = m.metrics["success"].(int) + 1
	}

	return err
}

func (m *MockCircuitBreaker) ExecuteWithTimeout(ctx context.Context, timeout time.Duration, fn func() error) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return m.Execute(ctx, fn)
}

func (m *MockCircuitBreaker) CanExecute() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.canExecuteFunc != nil {
		return m.canExecuteFunc()
	}
	return m.state != "open"
}

func (m *MockCircuitBreaker) GetState() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *MockCircuitBreaker) GetMetrics() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return a copy to avoid race conditions
	result := make(map[string]interface{})
	for k, v := range m.metrics {
		result[k] = v
	}
	return result
}

func (m *MockCircuitBreaker) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state = "closed"
	m.metrics = map[string]interface{}{
		"requests": 0,
		"failures": 0,
		"success":  0,
	}
}

func (m *MockCircuitBreaker) RecordSuccess() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics["success"] = m.metrics["success"].(int) + 1
}

func (m *MockCircuitBreaker) RecordFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics["failures"] = m.metrics["failures"].(int) + 1
}

func (m *MockCircuitBreaker) SetState(state string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
}

// TestMockTransport is a test-specific mock transport with failure simulation
type TestMockTransport struct {
	name        string
	available   bool
	failNext    int // Number of next calls to fail
	statusCode  int // HTTP status code to return
	initialized bool
	started     bool
	mu          sync.Mutex
}

func (m *TestMockTransport) Initialize(config TransportConfig) error {
	m.initialized = true
	return nil
}

func (m *TestMockTransport) Start(ctx context.Context) error {
	if !m.initialized {
		return fmt.Errorf("not initialized")
	}
	m.started = true
	return nil
}

func (m *TestMockTransport) Stop(ctx context.Context) error {
	m.started = false
	return nil
}

func (m *TestMockTransport) CreateHandler(agent ChatAgent) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		statusCode := m.statusCode
		m.mu.Unlock()

		if statusCode == 0 {
			statusCode = 200
		}

		w.WriteHeader(statusCode)
		w.Write([]byte(fmt.Sprintf("status: %d", statusCode)))
	})
}

func (m *TestMockTransport) HealthCheck(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failNext > 0 {
		m.failNext--
		return fmt.Errorf("health check failed")
	}
	return nil
}

func (m *TestMockTransport) Name() string        { return m.name }
func (m *TestMockTransport) Description() string { return "test transport" }
func (m *TestMockTransport) Available() bool     { return m.available }
func (m *TestMockTransport) Priority() int       { return 1 }
func (m *TestMockTransport) Capabilities() []TransportCapability {
	return []TransportCapability{CapabilityStreaming}
}
func (m *TestMockTransport) ClientExample() string { return "test example" }

// TestCircuitBreakerTransport tests the circuit breaker transport wrapper
func TestCircuitBreakerTransport(t *testing.T) {
	t.Run("blocks requests when circuit is open", func(t *testing.T) {
		mockTransport := &TestMockTransport{
			name:       "test",
			available:  true,
			statusCode: 200,
		}
		mockTransport.Initialize(TransportConfig{})

		mockBreaker := NewMockCircuitBreaker()
		mockBreaker.SetState("open")

		cb, err := NewCircuitBreakerTransport(mockTransport, mockBreaker, &core.NoOpLogger{})
		if err != nil {
			t.Fatalf("Failed to create circuit breaker transport: %v", err)
		}

		// Should not be available when circuit is open
		if cb.Available() {
			t.Error("Expected transport to be unavailable when circuit is open")
		}

		// Health check should fail
		err = cb.HealthCheck(context.Background())
		if !errors.Is(err, core.ErrCircuitBreakerOpen) {
			t.Errorf("Expected circuit breaker open error, got: %v", err)
		}
	})

	t.Run("allows requests when circuit is closed", func(t *testing.T) {
		mockTransport := &TestMockTransport{
			name:       "test",
			available:  true,
			statusCode: 200,
		}
		mockTransport.Initialize(TransportConfig{})

		mockBreaker := NewMockCircuitBreaker()
		mockBreaker.SetState("closed")

		cb, err := NewCircuitBreakerTransport(mockTransport, mockBreaker, &core.NoOpLogger{})
		if err != nil {
			t.Fatalf("Failed to create circuit breaker transport: %v", err)
		}

		// Should be available when circuit is closed
		if !cb.Available() {
			t.Error("Expected transport to be available when circuit is closed")
		}

		// Health check should succeed
		err = cb.HealthCheck(context.Background())
		if err != nil {
			t.Errorf("Expected health check to succeed, got error: %v", err)
		}
	})

	t.Run("HTTP handler returns 503 when circuit is open", func(t *testing.T) {
		mockTransport := &TestMockTransport{
			name:       "test",
			available:  true,
			statusCode: 200,
		}
		mockTransport.Initialize(TransportConfig{})

		mockBreaker := NewMockCircuitBreaker()
		mockBreaker.SetState("open")

		cb, err := NewCircuitBreakerTransport(mockTransport, mockBreaker, &core.NoOpLogger{})
		if err != nil {
			t.Fatalf("Failed to create circuit breaker transport: %v", err)
		}

		handler := cb.CreateHandler(nil)
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected status 503, got %d", rec.Code)
		}

		// Check headers
		if rec.Header().Get("X-Circuit-Breaker") != "open" {
			t.Error("Expected X-Circuit-Breaker header to be 'open'")
		}

		if rec.Header().Get("X-Circuit-State") != "open" {
			t.Error("Expected X-Circuit-State header to be 'open'")
		}
	})

	t.Run("records server errors as failures", func(t *testing.T) {
		mockTransport := &TestMockTransport{
			name:       "test",
			available:  true,
			statusCode: 500,
		}
		mockTransport.Initialize(TransportConfig{})

		failureRecorded := false
		mockBreaker := NewMockCircuitBreaker()
		mockBreaker.executeFunc = func(ctx context.Context, fn func() error) error {
			err := fn()
			if err != nil {
				failureRecorded = true
			}
			return err
		}

		cb, err := NewCircuitBreakerTransport(mockTransport, mockBreaker, &core.NoOpLogger{})
		if err != nil {
			t.Fatalf("Failed to create circuit breaker transport: %v", err)
		}

		handler := cb.CreateHandler(nil)
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if !failureRecorded {
			t.Error("Expected server error to be recorded as failure")
		}
	})

	t.Run("does not record client errors as failures", func(t *testing.T) {
		mockTransport := &TestMockTransport{
			name:       "test",
			available:  true,
			statusCode: 404,
		}
		mockTransport.Initialize(TransportConfig{})

		failureRecorded := false
		mockBreaker := NewMockCircuitBreaker()
		mockBreaker.executeFunc = func(ctx context.Context, fn func() error) error {
			err := fn()
			if err != nil {
				failureRecorded = true
			}
			return err
		}

		cb, err := NewCircuitBreakerTransport(mockTransport, mockBreaker, &core.NoOpLogger{})
		if err != nil {
			t.Fatalf("Failed to create circuit breaker transport: %v", err)
		}

		handler := cb.CreateHandler(nil)
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if failureRecorded {
			t.Error("Expected client error to NOT be recorded as failure")
		}

		if rec.Code != 404 {
			t.Errorf("Expected status 404, got %d", rec.Code)
		}
	})

	t.Run("preserves transport metadata", func(t *testing.T) {
		mockTransport := &TestMockTransport{
			name:      "test-transport",
			available: true,
		}

		mockBreaker := NewMockCircuitBreaker()
		cb, err := NewCircuitBreakerTransport(mockTransport, mockBreaker, &core.NoOpLogger{})
		if err != nil {
			t.Fatalf("Failed to create circuit breaker transport: %v", err)
		}

		// Check name includes circuit breaker suffix
		if cb.Name() != "test-transport-cb" {
			t.Errorf("Expected name 'test-transport-cb', got '%s'", cb.Name())
		}

		// Check description mentions circuit breaker
		if cb.Description() != "test transport (with circuit breaker protection)" {
			t.Errorf("Unexpected description: %s", cb.Description())
		}

		// Check priority is preserved
		if cb.Priority() != 1 {
			t.Errorf("Expected priority 1, got %d", cb.Priority())
		}

		// Check capabilities are preserved
		caps := cb.Capabilities()
		if len(caps) != 1 || caps[0] != CapabilityStreaming {
			t.Error("Capabilities not preserved")
		}
	})

	t.Run("Start is protected by circuit breaker", func(t *testing.T) {
		mockTransport := &TestMockTransport{
			name:      "test",
			available: true,
		}
		mockTransport.Initialize(TransportConfig{})

		mockBreaker := NewMockCircuitBreaker()
		mockBreaker.SetState("open")

		cb, err := NewCircuitBreakerTransport(mockTransport, mockBreaker, &core.NoOpLogger{})
		if err != nil {
			t.Fatalf("Failed to create circuit breaker transport: %v", err)
		}

		err = cb.Start(context.Background())
		if !errors.Is(err, core.ErrCircuitBreakerOpen) {
			t.Errorf("Expected circuit breaker open error on Start, got: %v", err)
		}
	})

	t.Run("Stop is always allowed", func(t *testing.T) {
		mockTransport := &TestMockTransport{
			name:      "test",
			available: true,
		}

		mockBreaker := NewMockCircuitBreaker()
		mockBreaker.SetState("open") // Circuit is open

		cb, err := NewCircuitBreakerTransport(mockTransport, mockBreaker, &core.NoOpLogger{})
		if err != nil {
			t.Fatalf("Failed to create circuit breaker transport: %v", err)
		}

		// Stop should succeed even with open circuit
		err = cb.Stop(context.Background())
		if err != nil {
			t.Errorf("Stop should always be allowed, got error: %v", err)
		}
	})

	t.Run("exposes circuit breaker state and metrics", func(t *testing.T) {
		mockTransport := &TestMockTransport{
			name:      "test",
			available: true,
		}

		mockBreaker := NewMockCircuitBreaker()
		mockBreaker.SetState("half-open")

		cb, err := NewCircuitBreakerTransport(mockTransport, mockBreaker, &core.NoOpLogger{})
		if err != nil {
			t.Fatalf("Failed to create circuit breaker transport: %v", err)
		}
		cbTransport := cb.(*CircuitBreakerTransport)

		if cbTransport.GetCircuitBreakerState() != "half-open" {
			t.Errorf("Expected state 'half-open', got '%s'", cbTransport.GetCircuitBreakerState())
		}

		metrics := cbTransport.GetCircuitBreakerMetrics()
		if metrics == nil {
			t.Error("Expected metrics to be non-nil")
		}
	})

	t.Run("returns error if circuit breaker is nil", func(t *testing.T) {
		mockTransport := &TestMockTransport{name: "test"}

		_, err := NewCircuitBreakerTransport(mockTransport, nil, nil)
		if err == nil {
			t.Error("Expected error when circuit breaker is nil")
		}
		if err != nil && err.Error() != "circuit breaker is required for CircuitBreakerTransport" {
			t.Errorf("Expected specific error message, got: %v", err)
		}
	})

	t.Run("returns error if transport is nil", func(t *testing.T) {
		mockBreaker := NewMockCircuitBreaker()

		_, err := NewCircuitBreakerTransport(nil, mockBreaker, nil)
		if err == nil {
			t.Error("Expected error when transport is nil")
		}
		if err != nil && err.Error() != "transport is required for CircuitBreakerTransport" {
			t.Errorf("Expected specific error message, got: %v", err)
		}
	})

	t.Run("concurrent request handling", func(t *testing.T) {
		mockTransport := &TestMockTransport{
			name:       "test",
			available:  true,
			statusCode: 200,
		}
		mockTransport.Initialize(TransportConfig{})

		mockBreaker := NewMockCircuitBreaker()
		cb, err := NewCircuitBreakerTransport(mockTransport, mockBreaker, &core.NoOpLogger{})
		if err != nil {
			t.Fatalf("Failed to create circuit breaker transport: %v", err)
		}

		handler := cb.CreateHandler(nil)

		// Run multiple concurrent requests
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := httptest.NewRequest("GET", "/test", nil)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != 200 {
					t.Errorf("Expected status 200, got %d", rec.Code)
				}
			}()
		}
		wg.Wait()

		metrics := mockBreaker.GetMetrics()
		if metrics["requests"].(int) != 10 {
			t.Errorf("Expected 10 requests, got %d", metrics["requests"])
		}
	})

	t.Run("half-open state allows limited requests", func(t *testing.T) {
		mockTransport := &TestMockTransport{
			name:       "test",
			available:  true,
			statusCode: 200,
		}

		mockBreaker := NewMockCircuitBreaker()
		mockBreaker.SetState("half-open")

		// Simulate half-open behavior
		requestCount := 0
		mockBreaker.canExecuteFunc = func() bool {
			requestCount++
			return requestCount <= 2 // Allow first 2 requests
		}

		cb, err := NewCircuitBreakerTransport(mockTransport, mockBreaker, &core.NoOpLogger{})
		if err != nil {
			t.Fatalf("Failed to create circuit breaker transport: %v", err)
		}

		// First check should be allowed
		if !cb.Available() {
			t.Error("Expected first request to be allowed in half-open state")
		}

		// Second check should be allowed
		if !cb.Available() {
			t.Error("Expected second request to be allowed in half-open state")
		}

		// Third check should NOT be allowed
		if cb.Available() {
			t.Error("Expected third request to be blocked in half-open state")
		}
	})

	t.Run("transport state transitions are handled correctly", func(t *testing.T) {
		mockTransport := &TestMockTransport{
			name:      "test",
			available: true,
		}

		mockBreaker := NewMockCircuitBreaker()
		cb, err := NewCircuitBreakerTransport(mockTransport, mockBreaker, &core.NoOpLogger{})
		if err != nil {
			t.Fatalf("Failed to create circuit breaker transport: %v", err)
		}

		// Simulate state transitions
		states := []string{"closed", "open", "half-open", "closed"}
		for _, state := range states {
			mockBreaker.SetState(state)

			// Cast to concrete type to access GetCircuitBreakerState
			cbTransport := cb.(*CircuitBreakerTransport)
			actualState := cbTransport.GetCircuitBreakerState()
			if actualState != state {
				t.Errorf("Expected state %s, got %s", state, actualState)
			}
		}
	})

	t.Run("logging is properly initialized", func(t *testing.T) {
		mockTransport := &TestMockTransport{
			name: "test",
		}

		mockBreaker := NewMockCircuitBreaker()

		// Test with nil logger - should use NoOpLogger
		cb1, err := NewCircuitBreakerTransport(mockTransport, mockBreaker, nil)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker transport: %v", err)
		}
		cb1Transport := cb1.(*CircuitBreakerTransport)
		if cb1Transport.logger == nil {
			t.Error("Expected logger to be initialized to NoOpLogger when nil")
		}

		// Test with provided logger
		logger := &core.NoOpLogger{}
		cb2, err := NewCircuitBreakerTransport(mockTransport, mockBreaker, logger)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker transport: %v", err)
		}
		cb2Transport := cb2.(*CircuitBreakerTransport)
		if cb2Transport.logger != logger {
			t.Error("Expected logger to be the provided logger")
		}

		// Test SetLogger
		newLogger := &core.NoOpLogger{}
		cb2Transport.SetLogger(newLogger)
		if cb2Transport.logger != newLogger {
			t.Error("SetLogger should update the logger")
		}
	})
}