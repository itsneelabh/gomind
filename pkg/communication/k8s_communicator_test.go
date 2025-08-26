package communication

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/pkg/discovery"
	"github.com/itsneelabh/gomind/pkg/logger"
)

// mockLogger implements the logger.Logger interface for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...interface{}) {}
func (m *mockLogger) Info(msg string, fields ...interface{})  {}
func (m *mockLogger) Warn(msg string, fields ...interface{})  {}
func (m *mockLogger) Error(msg string, fields ...interface{}) {}
func (m *mockLogger) SetLevel(level string)                   {}
func (m *mockLogger) WithField(key string, value interface{}) logger.Logger {
	return m
}
func (m *mockLogger) WithFields(fields map[string]interface{}) logger.Logger {
	return m
}
func (m *mockLogger) With(fields ...logger.Field) logger.Logger {
	return m
}

// mockDiscovery implements the discovery.Discovery interface for testing
type mockDiscovery struct{}

func (m *mockDiscovery) Register(ctx context.Context, registration *discovery.AgentRegistration) error {
	return nil
}

func (m *mockDiscovery) FindCapability(ctx context.Context, capability string) ([]discovery.AgentRegistration, error) {
	return []discovery.AgentRegistration{}, nil
}

func (m *mockDiscovery) FindAgent(ctx context.Context, agentID string) (*discovery.AgentRegistration, error) {
	return nil, fmt.Errorf("agent not found")
}

func (m *mockDiscovery) Unregister(ctx context.Context, agentID string) error {
	return nil
}

func (m *mockDiscovery) GetHealthStatus(ctx context.Context) discovery.HealthStatus {
	return discovery.HealthStatus{
		Status: "healthy",
		Message: "mock discovery is healthy",
	}
}

func (m *mockDiscovery) RefreshHeartbeat(ctx context.Context, agentID string) error {
	return nil
}

func (m *mockDiscovery) Close() error {
	return nil
}

// Phase 2: Catalog management methods (mock implementations)
func (m *mockDiscovery) DownloadFullCatalog(ctx context.Context) error {
	return nil
}

func (m *mockDiscovery) GetFullCatalog() map[string]*discovery.AgentRegistration {
	return make(map[string]*discovery.AgentRegistration)
}

func (m *mockDiscovery) GetCatalogForLLM() string {
	return "Mock catalog for LLM"
}

func (m *mockDiscovery) StartCatalogSync(ctx context.Context, interval time.Duration) {
	// Mock implementation - no-op
}

func (m *mockDiscovery) GetCatalogStats() (agentCount int, lastSync time.Time, syncErrors int) {
	return 0, time.Now(), 0
}

func TestK8sCommunicator_CallAgent(t *testing.T) {
	tests := []struct {
		name           string
		agentID        string
		instruction    string
		serverResponse string
		serverStatus   int
		expectError    bool
		expectedResult string
	}{
		{
			name:           "successful call",
			agentID:        "test-agent",
			instruction:    "hello world",
			serverResponse: "response from agent",
			serverStatus:   http.StatusOK,
			expectError:    false,
			expectedResult: "response from agent",
		},
		{
			name:           "agent returns error status",
			agentID:        "test-agent",
			instruction:    "bad request",
			serverResponse: "error message",
			serverStatus:   http.StatusBadRequest,
			expectError:    true,
			expectedResult: "",
		},
		{
			name:           "agent with namespace",
			agentID:        "test-agent.custom-namespace",
			instruction:    "hello",
			serverResponse: "response",
			serverStatus:   http.StatusOK,
			expectError:    false,
			expectedResult: "response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}
				if r.URL.Path != "/process" {
					t.Errorf("Expected /process path, got %s", r.URL.Path)
				}

				// Read body
				body, _ := io.ReadAll(r.Body)
				if string(body) != tt.instruction {
					t.Errorf("Expected body %s, got %s", tt.instruction, string(body))
				}

				// Check headers
				if r.Header.Get("Content-Type") != "text/plain" {
					t.Errorf("Expected Content-Type: text/plain, got %s", r.Header.Get("Content-Type"))
				}

				// Send response
				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			// Create communicator with custom serviceURLBuilder
			comm := NewK8sCommunicator(&mockDiscovery{}, &mockLogger{}, "default")
			comm.serviceURLBuilder = func(agentName, namespace string) string {
				return server.URL
			}

			// Call agent
			ctx := context.Background()
			result, err := comm.CallAgent(ctx, tt.agentID, tt.instruction)

			// Check results
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != tt.expectedResult {
				t.Errorf("Expected result %s, got %s", tt.expectedResult, result)
			}
		})
	}
}

func TestK8sCommunicator_CallAgentWithTimeout(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("slow response"))
	}))
	defer server.Close()

	comm := NewK8sCommunicator(&mockDiscovery{}, &mockLogger{}, "default")
	comm.serviceURLBuilder = func(agentName, namespace string) string {
		return server.URL
	}

	// Call with short timeout - should fail
	ctx := context.Background()
	_, err := comm.CallAgentWithTimeout(ctx, "test-agent", "hello", 100*time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error but got none")
	}

	// Verify it's a communication error
	if _, ok := err.(*CommunicationError); !ok {
		t.Errorf("Expected CommunicationError, got %T", err)
	}
}

func TestK8sCommunicator_RetryLogic(t *testing.T) {
	attempts := 0
	
	// Create server that fails first 2 attempts
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success after retries"))
		}
	}))
	defer server.Close()

	comm := NewK8sCommunicator(&mockDiscovery{}, &mockLogger{}, "default")
	comm.serviceURLBuilder = func(agentName, namespace string) string {
		return server.URL
	}

	// Call agent - should succeed after retries
	ctx := context.Background()
	result, err := comm.CallAgent(ctx, "test-agent", "test message")

	if err != nil {
		t.Errorf("Expected success after retries, got error: %v", err)
	}
	if result != "success after retries" {
		t.Errorf("Expected 'success after retries', got %s", result)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestK8sCommunicator_ParseAgentIdentifier(t *testing.T) {
	comm := NewK8sCommunicator(&mockDiscovery{}, &mockLogger{}, "default-ns")

	tests := []struct {
		identifier        string
		expectedName      string
		expectedNamespace string
	}{
		{
			identifier:        "agent-name",
			expectedName:      "agent-name",
			expectedNamespace: "default-ns",
		},
		{
			identifier:        "agent-name.custom-ns",
			expectedName:      "agent-name",
			expectedNamespace: "custom-ns",
		},
		{
			identifier:        "complex-agent-name.production",
			expectedName:      "complex-agent-name",
			expectedNamespace: "production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.identifier, func(t *testing.T) {
			name, namespace := comm.parseAgentIdentifier(tt.identifier)
			if name != tt.expectedName {
				t.Errorf("Expected name %s, got %s", tt.expectedName, name)
			}
			if namespace != tt.expectedNamespace {
				t.Errorf("Expected namespace %s, got %s", tt.expectedNamespace, namespace)
			}
		})
	}
}

func TestK8sCommunicator_BuildServiceURL(t *testing.T) {
	comm := NewK8sCommunicator(&mockDiscovery{}, &mockLogger{}, "default")

	tests := []struct {
		agentName    string
		namespace    string
		expectedURL  string
	}{
		{
			agentName:   "test-agent",
			namespace:   "default",
			expectedURL: "http://test-agent.default.svc.cluster.local:8080",
		},
		{
			agentName:   "portfolio-service",
			namespace:   "finance",
			expectedURL: "http://portfolio-service.finance.svc.cluster.local:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.agentName, func(t *testing.T) {
			url := comm.buildServiceURL(tt.agentName, tt.namespace)
			if url != tt.expectedURL {
				t.Errorf("Expected URL %s, got %s", tt.expectedURL, url)
			}
		})
	}
}

func TestK8sCommunicator_BuildServiceURLWithCustomSettings(t *testing.T) {
	comm := NewK8sCommunicator(&mockDiscovery{}, &mockLogger{}, "default")
	
	// Test custom port
	comm.SetServicePort(9090)
	url := comm.buildServiceURL("test-agent", "default")
	if !strings.Contains(url, ":9090") {
		t.Errorf("Expected URL to contain port 9090, got %s", url)
	}

	// Test custom cluster domain
	comm.SetClusterDomain("custom.local")
	url = comm.buildServiceURL("test-agent", "default")
	if !strings.Contains(url, ".custom.local:") {
		t.Errorf("Expected URL to contain custom.local, got %s", url)
	}

	// Test custom namespace
	comm.SetDefaultNamespace("production")
	_, namespace := comm.parseAgentIdentifier("test-agent")
	if namespace != "production" {
		t.Errorf("Expected default namespace 'production', got %s", namespace)
	}
}

func TestK8sCommunicator_Ping(t *testing.T) {
	tests := []struct {
		name         string
		serverStatus int
		expectError  bool
	}{
		{
			name:         "healthy agent",
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:         "unhealthy agent",
			serverStatus: http.StatusServiceUnavailable,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/health" {
					t.Errorf("Expected /health path, got %s", r.URL.Path)
				}
				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			comm := NewK8sCommunicator(&mockDiscovery{}, &mockLogger{}, "default")
			comm.serviceURLBuilder = func(agentName, namespace string) string {
				return server.URL
			}

			// Ping agent
			ctx := context.Background()
			err := comm.Ping(ctx, "test-agent")

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestCommunicationError(t *testing.T) {
	// Test error without cause
	err1 := &CommunicationError{
		Agent:   "test-agent",
		Message: "connection failed",
	}
	expectedMsg1 := "communication with test-agent failed: connection failed"
	if err1.Error() != expectedMsg1 {
		t.Errorf("Expected error message %s, got %s", expectedMsg1, err1.Error())
	}

	// Test error with cause
	cause := fmt.Errorf("network timeout")
	err2 := &CommunicationError{
		Agent:   "test-agent",
		Message: "connection failed",
		Cause:   cause,
	}
	expectedMsg2 := "communication with test-agent failed: connection failed: network timeout"
	if err2.Error() != expectedMsg2 {
		t.Errorf("Expected error message %s, got %s", expectedMsg2, err2.Error())
	}

	// Test unwrap
	if err2.Unwrap() != cause {
		t.Error("Unwrap did not return the correct cause")
	}
}

func TestDefaultCommunicationOptions(t *testing.T) {
	opts := DefaultCommunicationOptions()

	if opts.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", opts.Timeout)
	}
	if opts.Retries != 3 {
		t.Errorf("Expected 3 retries, got %d", opts.Retries)
	}
	if opts.RetryDelay != 1*time.Second {
		t.Errorf("Expected retry delay 1s, got %v", opts.RetryDelay)
	}
	if opts.Headers == nil {
		t.Error("Expected Headers map to be initialized")
	}
}

func TestK8sCommunicator_GetAvailableAgents(t *testing.T) {
	comm := NewK8sCommunicator(&mockDiscovery{}, &mockLogger{}, "default")
	
	ctx := context.Background()
	agents, err := comm.GetAvailableAgents(ctx)
	
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// For now, the implementation returns an empty list
	if len(agents) != 0 {
		t.Errorf("Expected empty agent list, got %d agents", len(agents))
	}
}