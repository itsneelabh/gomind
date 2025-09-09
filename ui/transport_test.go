package ui

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// MockTransport implements Transport interface for testing
type MockTransport struct {
	name        string
	priority    int
	available   bool
	initialized bool
	started     bool
}

func (m *MockTransport) Name() string        { return m.name }
func (m *MockTransport) Description() string { return "Mock transport for testing" }

func (m *MockTransport) Initialize(config TransportConfig) error {
	m.initialized = true
	return nil
}

func (m *MockTransport) Start(ctx context.Context) error {
	if !m.initialized {
		return fmt.Errorf("not initialized")
	}
	m.started = true
	return nil
}

func (m *MockTransport) Stop(ctx context.Context) error {
	m.started = false
	return nil
}

func (m *MockTransport) Available() bool { return m.available }
func (m *MockTransport) Priority() int   { return m.priority }

func (m *MockTransport) CreateHandler(agent ChatAgent) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mock transport"))
	})
}

func (m *MockTransport) Capabilities() []TransportCapability {
	return []TransportCapability{
		CapabilityStreaming,
	}
}

func (m *MockTransport) HealthCheck(ctx context.Context) error {
	if !m.started {
		return fmt.Errorf("not started")
	}
	return nil
}

func (m *MockTransport) ClientExample() string {
	return "// Mock client example"
}

// TestTransportRegistry tests the transport registry functionality
func TestTransportRegistry(t *testing.T) {
	// Create a new registry for isolated testing
	registry := NewTransportRegistry()

	t.Run("RegisterTransport", func(t *testing.T) {
		transport := &MockTransport{
			name:      "test-transport",
			priority:  100,
			available: true,
		}

		registry.Register(transport)

		retrieved, exists := registry.Get("test-transport")
		if !exists {
			t.Error("Transport should be registered")
		}
		if retrieved.Name() != "test-transport" {
			t.Error("Retrieved wrong transport")
		}
	})

	t.Run("ListAvailableTransports", func(t *testing.T) {
		// Create new registry for this test
		registry := NewTransportRegistry()

		transports := []*MockTransport{
			{name: "high-priority", priority: 200, available: true},
			{name: "low-priority", priority: 50, available: true},
			{name: "unavailable", priority: 150, available: false},
		}

		for _, tr := range transports {
			registry.Register(tr)
		}

		available := registry.ListAvailable()

		// Should only have 2 available transports
		if len(available) != 2 {
			t.Errorf("Expected 2 available transports, got %d", len(available))
		}

		// Should be sorted by priority (high to low)
		if available[0].Name() != "high-priority" {
			t.Error("First transport should be high-priority")
		}
		if available[1].Name() != "low-priority" {
			t.Error("Second transport should be low-priority")
		}
	})

	t.Run("GetNonExistentTransport", func(t *testing.T) {
		registry := NewTransportRegistry()
		_, exists := registry.Get("non-existent")
		if exists {
			t.Error("Non-existent transport should not be found")
		}
	})
}

// TestTransportLifecycle tests transport lifecycle management
func TestTransportLifecycle(t *testing.T) {
	transport := &MockTransport{
		name:      "lifecycle-test",
		available: true,
	}

	ctx := context.Background()

	// Should fail to start without initialization
	err := transport.Start(ctx)
	if err == nil {
		t.Error("Start should fail without initialization")
	}

	// Initialize
	err = transport.Initialize(TransportConfig{})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if !transport.initialized {
		t.Error("Transport should be initialized")
	}

	// Start
	err = transport.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !transport.started {
		t.Error("Transport should be started")
	}

	// Health check should pass
	err = transport.HealthCheck(ctx)
	if err != nil {
		t.Error("Health check should pass when started")
	}

	// Stop
	err = transport.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if transport.started {
		t.Error("Transport should be stopped")
	}

	// Health check should fail
	err = transport.HealthCheck(ctx)
	if err == nil {
		t.Error("Health check should fail when stopped")
	}
}

// RunTransportComplianceTests defines tests that all transports must pass
func RunTransportComplianceTests(t *testing.T, transport Transport) {
	ctx := context.Background()

	t.Run("Metadata", func(t *testing.T) {
		if transport.Name() == "" {
			t.Error("Transport must have a name")
		}
		if transport.Description() == "" {
			t.Error("Transport must have a description")
		}
		if transport.ClientExample() == "" {
			t.Error("Transport must provide client example")
		}
	})

	t.Run("Lifecycle", func(t *testing.T) {
		// Initialize
		config := TransportConfig{
			MaxConnections: 100,
			Timeout:        5 * time.Second,
			Options: map[string]interface{}{
				"buffer_size": 1024,
			},
		}
		err := transport.Initialize(config)
		if err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		// Start
		startCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		err = transport.Start(startCtx)
		if err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		// Health check
		healthCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		err = transport.HealthCheck(healthCtx)
		if err != nil {
			t.Errorf("Health check failed: %v", err)
		}

		// Stop
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		err = transport.Stop(stopCtx)
		if err != nil {
			t.Fatalf("Stop failed: %v", err)
		}
	})

	t.Run("HandlerCreation", func(t *testing.T) {
		// Create mock agent
		sessions := NewMockSessionManager(DefaultSessionConfig())
		agent := &DefaultChatAgent{
			BaseAgent:  core.NewBaseAgent("test-agent"),
			sessions:   sessions,
			transports: make(map[string]Transport),
			config:     DefaultChatAgentConfig("test"),
			stopChan:   make(chan struct{}),
		}

		handler := transport.CreateHandler(agent)
		if handler == nil {
			t.Error("CreateHandler must return a valid handler")
		}
	})

	t.Run("Capabilities", func(t *testing.T) {
		capabilities := transport.Capabilities()
		if len(capabilities) == 0 {
			t.Error("Transport should have at least one capability")
		}

		for _, cap := range capabilities {
			if string(cap) == "" {
				t.Error("Capability must not be empty")
			}
		}
	})

	t.Run("Priority", func(t *testing.T) {
		priority := transport.Priority()
		if priority < 0 {
			t.Error("Priority should not be negative")
		}
	})

	t.Run("Availability", func(t *testing.T) {
		// Availability should return consistent result
		avail1 := transport.Available()
		avail2 := transport.Available()
		if avail1 != avail2 {
			t.Error("Available() should return consistent result")
		}
	})
}
