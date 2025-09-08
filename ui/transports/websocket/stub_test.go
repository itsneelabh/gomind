//go:build !websocket

package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/ui"
	uitesting "github.com/itsneelabh/gomind/ui/testing"
)

func TestWebSocketStub_Metadata(t *testing.T) {
	stub := &WebSocketStub{}

	if stub.Name() != "websocket" {
		t.Errorf("Expected name 'websocket', got %s", stub.Name())
	}

	desc := stub.Description()
	if !strings.Contains(desc, "not available") {
		t.Error("Description should indicate it's not available")
	}

	if !strings.Contains(desc, "websocket") {
		t.Error("Description should mention the required build tag")
	}

	if stub.Priority() != 0 {
		t.Errorf("Expected priority 0, got %d", stub.Priority())
	}

	capabilities := stub.Capabilities()
	if len(capabilities) != 0 {
		t.Errorf("Expected 0 capabilities, got %d", len(capabilities))
	}
}

func TestWebSocketStub_Available(t *testing.T) {
	stub := &WebSocketStub{}
	
	// Stub should never be available
	if stub.Available() {
		t.Error("WebSocket stub should not be available")
	}
}

func TestWebSocketStub_Lifecycle(t *testing.T) {
	stub := &WebSocketStub{}
	ctx := context.Background()

	// Initialize should fail
	config := ui.TransportConfig{
		MaxConnections: 100,
		Timeout:        5 * time.Second,
	}
	
	err := stub.Initialize(config)
	if err == nil {
		t.Error("Initialize should fail for stub")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Error("Error should mention transport is not available")
	}

	// Start should fail
	err = stub.Start(ctx)
	if err == nil {
		t.Error("Start should fail for stub")
	}

	// Stop should succeed (no-op)
	err = stub.Stop(ctx)
	if err != nil {
		t.Errorf("Stop should succeed for stub, got: %v", err)
	}

	// Health check should fail
	err = stub.HealthCheck(ctx)
	if err == nil {
		t.Error("HealthCheck should fail for stub")
	}
}

func TestWebSocketStub_Handler(t *testing.T) {
	stub := &WebSocketStub{}
	agent := uitesting.NewMockChatAgent("test-agent")
	
	handler := stub.CreateHandler(agent)
	if handler == nil {
		t.Error("CreateHandler should return a handler even for stub")
	}

	// Test the handler response
	req := httptest.NewRequest("GET", "/chat/websocket", nil)
	rec := httptest.NewRecorder()
	
	handler.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected JSON content type, got %s", contentType)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "not available") {
		t.Error("Response should indicate transport is not available")
	}
	if !strings.Contains(body, "websocket") {
		t.Error("Response should mention websocket build tag")
	}
	if !strings.Contains(body, "TRANSPORT_UNAVAILABLE") {
		t.Error("Response should include error code")
	}
}

func TestWebSocketStub_ClientExample(t *testing.T) {
	stub := &WebSocketStub{}
	example := stub.ClientExample()
	
	if example == "" {
		t.Error("Client example should not be empty")
	}
	
	// Verify example explains how to enable WebSocket
	expectedTerms := []string{
		"not available",
		"websocket",
		"build",
		"tags",
		"gorilla/websocket",
		"Alternative transports",
		"SSE",
	}
	
	for _, term := range expectedTerms {
		if !strings.Contains(example, term) {
			t.Errorf("Client example should contain '%s'", term)
		}
	}
}

func TestWebSocketStub_ComplianceSafety(t *testing.T) {
	// Verify stub doesn't break compliance tests
	stub := &WebSocketStub{}
	
	// Basic metadata tests should pass
	if stub.Name() == "" {
		t.Error("Name should not be empty")
	}
	
	if stub.Description() == "" {
		t.Error("Description should not be empty")
	}
	
	if stub.Priority() < 0 {
		t.Error("Priority should not be negative")
	}
	
	// These should not panic
	stub.Available()
	stub.Capabilities()
	stub.ClientExample()
	
	// Handler creation should not panic
	agent := uitesting.NewMockChatAgent("test")
	handler := stub.CreateHandler(agent)
	if handler == nil {
		t.Error("Handler should not be nil")
	}
}

func TestWebSocketStub_RegistryIntegration(t *testing.T) {
	// Test that stub registers properly with transport registry
	registry := ui.NewTransportRegistry()
	stub := &WebSocketStub{}
	
	err := registry.Register(stub)
	if err != nil {
		t.Fatalf("Failed to register stub: %v", err)
	}
	
	// Should be retrievable
	retrieved, exists := registry.Get("websocket")
	if !exists {
		t.Error("Stub should be registered")
	}
	
	if retrieved.Name() != "websocket" {
		t.Error("Retrieved transport should be websocket")
	}
	
	// Should not appear in available transports
	available := registry.ListAvailable()
	for _, transport := range available {
		if transport.Name() == "websocket" {
			t.Error("Stub should not appear in available transports")
		}
	}
	
	// Should appear in all transports
	all := registry.List()
	found := false
	for _, transport := range all {
		if transport.Name() == "websocket" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Stub should appear in all transports")
	}
}