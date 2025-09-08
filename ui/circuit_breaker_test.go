package ui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestMockTransport is a test-specific mock transport with failure simulation
type TestMockTransport struct {
	name         string
	available    bool
	failNext     int  // Number of next calls to fail
	statusCode   int  // HTTP status code to return
	initialized  bool
	started      bool
	mu           sync.Mutex
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
		fmt.Fprintf(w, "Mock response")
	})
}

func (m *TestMockTransport) Name() string { return m.name }
func (m *TestMockTransport) Description() string { return "Mock transport for testing" }
func (m *TestMockTransport) Priority() int { return 50 }
func (m *TestMockTransport) Capabilities() []TransportCapability {
	return []TransportCapability{CapabilityStreaming}
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

func (m *TestMockTransport) Available() bool { return m.available }
func (m *TestMockTransport) ClientExample() string { return "// Mock client example" }

// TestCircuitBreakerStates tests the circuit breaker state transitions
func TestCircuitBreakerStates(t *testing.T) {
	// Create mock transport that can be configured to fail
	mockTransport := &TestMockTransport{
		name:      "test-transport",
		available: true,
	}
	
	// Create circuit breaker with low thresholds for testing
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:         100 * time.Millisecond,
		MaxRequests:     3,  // Need at least 2 for the success threshold
	}
	
	cb := NewCircuitBreakerTransport(mockTransport, config).(*CircuitBreakerTransport)
	
	// Test initial state is CLOSED
	if cb.GetState() != StateClosed {
		t.Errorf("Initial state should be CLOSED, got %v", cb.GetState())
	}
	
	// Simulate failures to open the circuit
	mockTransport.failNext = 3
	ctx := context.Background()
	
	for i := 0; i < 3; i++ {
		err := cb.HealthCheck(ctx)
		if err == nil {
			t.Error("Expected health check to fail")
		}
	}
	
	// Circuit should now be OPEN
	if cb.GetState() != StateOpen {
		t.Errorf("State should be OPEN after %d failures, got %v", config.FailureThreshold, cb.GetState())
	}
	
	// Attempts should fail immediately when circuit is open
	err := cb.HealthCheck(ctx)
	if err == nil {
		t.Error("Expected health check to fail when circuit is open")
	}
	
	// Wait for timeout to transition to HALF_OPEN
	time.Sleep(config.Timeout + 10*time.Millisecond)
	
	// Next attempt should be allowed (HALF_OPEN state)
	mockTransport.failNext = 0 // Make it succeed
	err = cb.HealthCheck(ctx)
	if err != nil {
		t.Errorf("Expected health check to succeed in half-open state: %v", err)
	}
	
	// Circuit should be in HALF_OPEN
	if cb.GetState() != StateHalfOpen {
		t.Errorf("State should be HALF_OPEN, got %v", cb.GetState())
	}
	
	// One more success should close the circuit
	err = cb.HealthCheck(ctx)
	if err != nil {
		t.Errorf("Expected health check to succeed: %v", err)
	}
	
	// Circuit should now be CLOSED
	if cb.GetState() != StateClosed {
		t.Errorf("State should be CLOSED after %d successes, got %v", config.SuccessThreshold, cb.GetState())
	}
}

// TestCircuitBreakerHalfOpenFailure tests that circuit reopens on failure in half-open state
func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	mockTransport := &TestMockTransport{
		name:      "test-transport",
		available: true,
	}
	
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:         50 * time.Millisecond,
		MaxRequests:     1,
	}
	
	cb := NewCircuitBreakerTransport(mockTransport, config).(*CircuitBreakerTransport)
	ctx := context.Background()
	
	// Open the circuit
	mockTransport.failNext = 2
	for i := 0; i < 2; i++ {
		cb.HealthCheck(ctx)
	}
	
	if cb.GetState() != StateOpen {
		t.Fatalf("Circuit should be open")
	}
	
	// Wait for timeout to transition to half-open
	time.Sleep(config.Timeout + 10*time.Millisecond)
	
	// Fail in half-open state
	mockTransport.failNext = 1
	cb.HealthCheck(ctx)
	
	// Circuit should be open again
	if cb.GetState() != StateOpen {
		t.Errorf("Circuit should reopen on failure in half-open state, got %v", cb.GetState())
	}
}

// TestCircuitBreakerHTTPHandler tests the HTTP handler wrapper
func TestCircuitBreakerHTTPHandler(t *testing.T) {
	mockTransport := &TestMockTransport{
		name:      "test-transport",
		available: true,
	}
	
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		Timeout:         50 * time.Millisecond,
	}
	
	cb := NewCircuitBreakerTransport(mockTransport, config).(*CircuitBreakerTransport)
	
	// Create a test agent
	sessions := NewMockSessionManager(DefaultSessionConfig())
	agent := &DefaultChatAgent{
		sessions: sessions,
	}
	
	handler := cb.CreateHandler(agent)
	
	// Test normal operation
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	// Open the circuit by simulating failures
	cb.mu.Lock()
	cb.state = StateOpen
	cb.lastFailTime = time.Now()
	cb.mu.Unlock()
	
	// Test when circuit is open
	req = httptest.NewRequest("GET", "/test", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 when circuit is open, got %d", rec.Code)
	}
	
	// Check for circuit breaker header
	if rec.Header().Get("X-Circuit-Breaker") != "open" {
		t.Error("Expected X-Circuit-Breaker header to be 'open'")
	}
}

// TestCircuitBreakerAvailability tests the Available() method
func TestCircuitBreakerAvailability(t *testing.T) {
	mockTransport := &TestMockTransport{
		name:      "test-transport",
		available: true,
	}
	
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:         50 * time.Millisecond,
	}
	
	cb := NewCircuitBreakerTransport(mockTransport, config).(*CircuitBreakerTransport)
	
	// Initially available
	if !cb.Available() {
		t.Error("Circuit breaker should be available initially")
	}
	
	// Open the circuit
	cb.mu.Lock()
	cb.state = StateOpen
	cb.lastFailTime = time.Now()
	cb.mu.Unlock()
	
	// Should not be available when open (within timeout)
	if cb.Available() {
		t.Error("Circuit breaker should not be available when open")
	}
	
	// Wait for timeout
	time.Sleep(config.Timeout + 10*time.Millisecond)
	
	// Should be available after timeout (will transition to half-open)
	if !cb.Available() {
		t.Error("Circuit breaker should be available after timeout")
	}
	
	// Test when underlying transport is not available
	mockTransport.available = false
	cb.mu.Lock()
	cb.state = StateClosed
	cb.mu.Unlock()
	
	if cb.Available() {
		t.Error("Circuit breaker should not be available when underlying transport is not available")
	}
}

// TestCircuitBreakerStats tests the GetStats() method
func TestCircuitBreakerStats(t *testing.T) {
	mockTransport := &TestMockTransport{
		name: "test-transport",
	}
	
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
	}
	
	cb := NewCircuitBreakerTransport(mockTransport, config).(*CircuitBreakerTransport)
	
	// Get initial stats
	stats := cb.GetStats()
	
	if stats["state"] != "CLOSED" {
		t.Errorf("Expected state CLOSED, got %v", stats["state"])
	}
	
	if stats["failure_count"].(int) != 0 {
		t.Errorf("Expected failure_count 0, got %v", stats["failure_count"])
	}
	
	// Record some failures
	ctx := context.Background()
	mockTransport.failNext = 2
	cb.HealthCheck(ctx)
	cb.HealthCheck(ctx)
	
	stats = cb.GetStats()
	if stats["failure_count"].(int) != 2 {
		t.Errorf("Expected failure_count 2, got %v", stats["failure_count"])
	}
}

// TestCircuitBreakerMaxRequests tests the max requests limit in half-open state
func TestCircuitBreakerMaxRequests(t *testing.T) {
	mockTransport := &TestMockTransport{
		name:      "test-transport",
		available: true,
	}
	
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:         50 * time.Millisecond,
		MaxRequests:     2, // Allow 2 requests in half-open
	}
	
	cb := NewCircuitBreakerTransport(mockTransport, config).(*CircuitBreakerTransport)
	ctx := context.Background()
	
	// Open the circuit
	mockTransport.failNext = 1
	cb.HealthCheck(ctx)
	
	// Wait for timeout
	time.Sleep(config.Timeout + 10*time.Millisecond)
	
	// First request should be allowed
	mockTransport.failNext = 0
	if !cb.canAttempt() {
		t.Error("First request should be allowed in half-open state")
	}
	
	// Second request should be allowed
	if !cb.canAttempt() {
		t.Error("Second request should be allowed in half-open state")
	}
	
	// Third request should be blocked
	if cb.canAttempt() {
		t.Error("Third request should be blocked in half-open state")
	}
}

// TestCircuitBreakerConcurrency tests thread safety
func TestCircuitBreakerConcurrency(t *testing.T) {
	mockTransport := &TestMockTransport{
		name:      "test-transport",
		available: true,
	}
	
	config := CircuitBreakerConfig{
		FailureThreshold: 10,
		SuccessThreshold: 5,
	}
	
	cb := NewCircuitBreakerTransport(mockTransport, config).(*CircuitBreakerTransport)
	ctx := context.Background()
	
	// Run concurrent operations
	done := make(chan bool, 20)
	
	for i := 0; i < 10; i++ {
		go func() {
			cb.HealthCheck(ctx)
			done <- true
		}()
		
		go func() {
			cb.Available()
			done <- true
		}()
	}
	
	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
	
	// Circuit should still be in a valid state
	state := cb.GetState()
	if state != StateClosed && state != StateOpen && state != StateHalfOpen {
		t.Errorf("Invalid state after concurrent operations: %v", state)
	}
}

// TestCircuitBreakerStateTransitionCounts tests that counts are reset consistently
func TestCircuitBreakerStateTransitionCounts(t *testing.T) {
	mockTransport := &TestMockTransport{
		name:      "test-transport",
		available: true,
	}
	
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:         50 * time.Millisecond,
	}
	
	cb := NewCircuitBreakerTransport(mockTransport, config).(*CircuitBreakerTransport)
	ctx := context.Background()
	
	// Cause failures to open circuit from Closed state
	mockTransport.failNext = 2
	cb.HealthCheck(ctx)
	cb.HealthCheck(ctx)
	
	// Check that failureCount is reset after Closed->Open transition
	stats := cb.GetStats()
	if stats["failure_count"].(int) != 0 {
		t.Errorf("Expected failure_count to be reset to 0 after Closed->Open, got %v", stats["failure_count"])
	}
	
	// Wait for timeout and transition to HalfOpen
	time.Sleep(config.Timeout + 10*time.Millisecond)
	
	// Fail in HalfOpen to reopen
	mockTransport.failNext = 1
	cb.HealthCheck(ctx)
	
	// Check that failureCount is still 0 after HalfOpen->Open transition
	stats = cb.GetStats()
	if stats["failure_count"].(int) != 0 {
		t.Errorf("Expected failure_count to be 0 after HalfOpen->Open, got %v", stats["failure_count"])
	}
}

// TestCircuitBreakerConfigValidation tests that configuration is properly validated
func TestCircuitBreakerConfigValidation(t *testing.T) {
	mockTransport := &TestMockTransport{
		name:      "test-transport",
		available: true,
	}
	
	// Test that MaxRequests is adjusted to meet SuccessThreshold
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 5,
		MaxRequests:     2,  // Less than SuccessThreshold
	}
	
	cb := NewCircuitBreakerTransport(mockTransport, config).(*CircuitBreakerTransport)
	
	// MaxRequests should be adjusted to at least SuccessThreshold
	if cb.config.MaxRequests < cb.config.SuccessThreshold {
		t.Errorf("MaxRequests should be at least SuccessThreshold, got MaxRequests=%d, SuccessThreshold=%d",
			cb.config.MaxRequests, cb.config.SuccessThreshold)
	}
	
	// Test with zero values (defaults)
	config2 := CircuitBreakerConfig{}
	cb2 := NewCircuitBreakerTransport(mockTransport, config2).(*CircuitBreakerTransport)
	
	// Check defaults are set properly
	if cb2.config.FailureThreshold == 0 {
		t.Error("FailureThreshold should have a default value")
	}
	if cb2.config.SuccessThreshold == 0 {
		t.Error("SuccessThreshold should have a default value")
	}
	if cb2.config.MaxRequests < cb2.config.SuccessThreshold {
		t.Errorf("Default MaxRequests should be at least SuccessThreshold, got MaxRequests=%d, SuccessThreshold=%d",
			cb2.config.MaxRequests, cb2.config.SuccessThreshold)
	}
}

// TestCircuitBreakerErrorTypes tests that only server errors open the circuit
func TestCircuitBreakerErrorTypes(t *testing.T) {
	mockTransport := &TestMockTransport{
		name:      "test-transport",
		available: true,
	}
	
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
	}
	
	cb := NewCircuitBreakerTransport(mockTransport, config).(*CircuitBreakerTransport)
	
	// Create test agent
	sessions := NewMockSessionManager(DefaultSessionConfig())
	agent := &DefaultChatAgent{
		sessions: sessions,
	}
	
	handler := cb.CreateHandler(agent)
	
	// Client error (4xx) should not open circuit
	mockTransport.statusCode = 400
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
	
	if cb.GetState() != StateClosed {
		t.Error("Client errors should not open the circuit")
	}
	
	// Server error (5xx) should open circuit
	mockTransport.statusCode = 500
	
	// Directly test the logic by recording errors
	cb.recordResult(fmt.Errorf("server error: 500"))
	cb.recordResult(fmt.Errorf("server error: 500"))
	
	if cb.GetState() != StateOpen {
		t.Error("Server errors should open the circuit")
	}
}