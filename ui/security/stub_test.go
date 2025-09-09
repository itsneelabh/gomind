//go:build !security
// +build !security

package security

import (
	"context"
	"net/http"
	"testing"

	"github.com/itsneelabh/gomind/ui"
)

// MockTransport for stub testing (minimal version)
type MockTransport struct{}

func (m *MockTransport) Name() string                                  { return "mock" }
func (m *MockTransport) Description() string                           { return "mock" }
func (m *MockTransport) Available() bool                               { return true }
func (m *MockTransport) Priority() int                                 { return 100 }
func (m *MockTransport) Initialize(config ui.TransportConfig) error    { return nil }
func (m *MockTransport) Start(ctx context.Context) error               { return nil }
func (m *MockTransport) Stop(ctx context.Context) error                { return nil }
func (m *MockTransport) HealthCheck(ctx context.Context) error         { return nil }
func (m *MockTransport) CreateHandler(agent ui.ChatAgent) http.Handler { return nil }
func (m *MockTransport) ClientExample() string                         { return "" }
func (m *MockTransport) Capabilities() []ui.TransportCapability        { return nil }

func TestStubReturnsOriginalTransport(t *testing.T) {
	mockTransport := &MockTransport{}

	// Test WithSecurity returns original
	config := SecurityConfig{
		Enabled: true, // Even when enabled, stub should return original
	}

	wrapped := WithSecurity(mockTransport, config)

	if wrapped != mockTransport {
		t.Error("Stub WithSecurity should return original transport unchanged")
	}
}

func TestDefaultSecurityConfigDisabled(t *testing.T) {
	config := DefaultSecurityConfig()

	if config.Enabled {
		t.Error("Stub DefaultSecurityConfig should be disabled")
	}
}

func TestIsSecurityEnabledReturnsFalse(t *testing.T) {
	if IsSecurityEnabled() {
		t.Error("Stub IsSecurityEnabled should return false")
	}
}

func TestDefaultSecurityHeadersReturnsNil(t *testing.T) {
	headers := DefaultSecurityHeaders()

	if headers != nil {
		t.Error("Stub DefaultSecurityHeaders should return nil")
	}
}
