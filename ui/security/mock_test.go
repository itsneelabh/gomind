//go:build security
// +build security

package security

import (
	"context"
	"net/http"

	"github.com/itsneelabh/gomind/ui"
)

// MockTransport implements ui.Transport for testing
type MockTransport struct {
	name         string
	description  string
	available    bool
	priority     int
	initialized  bool
	started      bool
	stopped      bool
	healthChecks int
	handler      http.Handler
}

func (m *MockTransport) Name() string        { return m.name }
func (m *MockTransport) Description() string { return m.description }
func (m *MockTransport) Available() bool     { return m.available }
func (m *MockTransport) Priority() int       { return m.priority }

func (m *MockTransport) Initialize(config ui.TransportConfig) error {
	m.initialized = true
	return nil
}

func (m *MockTransport) Start(ctx context.Context) error {
	m.started = true
	return nil
}

func (m *MockTransport) Stop(ctx context.Context) error {
	m.stopped = true
	return nil
}

func (m *MockTransport) HealthCheck(ctx context.Context) error {
	m.healthChecks++
	return nil
}

func (m *MockTransport) CreateHandler(agent ui.ChatAgent) http.Handler {
	if m.handler != nil {
		return m.handler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mock response"))
	})
}

func (m *MockTransport) ClientExample() string {
	return "// Mock client example"
}

func (m *MockTransport) Capabilities() []ui.TransportCapability {
	return []ui.TransportCapability{}
}
