//go:build websocket

package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/itsneelabh/gomind/ui"
	uitesting "github.com/itsneelabh/gomind/ui/testing"
)

func TestWebSocketTransport_Compliance(t *testing.T) {
	// The compliance test framework has a design issue where it expects to run
	// multiple independent tests on the same transport instance, but the tests
	// have interdependencies. For now, we'll implement specific compliance tests.

	t.Run("CompliantMetadata", func(t *testing.T) {
		transport := &WebSocketTransport{}
		if transport.Name() == "" {
			t.Error("Transport.Name() returned empty string")
		}
		if transport.Description() == "" {
			t.Error("Transport.Description() returned empty string")
		}
		if transport.Priority() < 0 {
			t.Errorf("Transport.Priority() returned negative value: %d", transport.Priority())
		}
		if transport.Capabilities() == nil {
			t.Error("Transport.Capabilities() returned nil")
		}
	})

	t.Run("CompliantLifecycle", func(t *testing.T) {
		transport := &WebSocketTransport{}
		ctx := context.Background()

		config := ui.TransportConfig{
			MaxConnections: 10,
			Timeout:        5 * time.Second,
		}

		// Initialize
		if err := transport.Initialize(config); err != nil {
			t.Fatalf("Transport.Initialize() failed: %v", err)
		}

		// Start
		if err := transport.Start(ctx); err != nil {
			t.Fatalf("Transport.Start() failed: %v", err)
		}

		// Stop
		if err := transport.Stop(ctx); err != nil {
			t.Fatalf("Transport.Stop() failed: %v", err)
		}

		// Should be able to restart
		if err := transport.Start(ctx); err != nil {
			t.Fatalf("Transport.Start() after Stop() failed: %v", err)
		}

		transport.Stop(ctx)
	})

	t.Run("CompliantErrorHandling", func(t *testing.T) {
		transport := &WebSocketTransport{}
		ctx := context.Background()

		// Starting without initialization should fail
		if err := transport.Start(ctx); err == nil {
			t.Error("Transport.Start() without Initialize() should fail")
		}

		// Initialize with invalid config should not panic
		invalidConfig := ui.TransportConfig{
			MaxConnections: -1,
			Timeout:        -1 * time.Second,
		}
		transport.Initialize(invalidConfig) // Should not panic
	})
}

func TestWebSocketTransport_Metadata(t *testing.T) {
	transport := &WebSocketTransport{}

	if transport.Name() != "websocket" {
		t.Errorf("Expected name 'websocket', got %s", transport.Name())
	}

	if transport.Description() == "" {
		t.Error("Description should not be empty")
	}

	if transport.Priority() != 150 {
		t.Errorf("Expected priority 150, got %d", transport.Priority())
	}

	capabilities := transport.Capabilities()
	expectedCaps := map[ui.TransportCapability]bool{
		ui.CapabilityStreaming:     true,
		ui.CapabilityBidirectional: true,
		ui.CapabilityReconnect:     true,
		ui.CapabilityMultiplex:     true,
	}

	if len(capabilities) != len(expectedCaps) {
		t.Errorf("Expected %d capabilities, got %d", len(expectedCaps), len(capabilities))
	}

	for _, cap := range capabilities {
		if !expectedCaps[cap] {
			t.Errorf("Unexpected capability: %s", cap)
		}
	}
}

func TestWebSocketTransport_Available(t *testing.T) {
	transport := &WebSocketTransport{}

	// WebSocket should be available when built with websocket tag
	if !transport.Available() {
		t.Error("WebSocket transport should be available when built with websocket tag")
	}
}

func TestWebSocketTransport_Lifecycle(t *testing.T) {
	transport := &WebSocketTransport{}
	ctx := context.Background()

	// Should fail to start without initialization
	err := transport.Start(ctx)
	if err == nil {
		t.Error("Start should fail without initialization")
	}

	// Initialize with valid config
	config := ui.TransportConfig{
		MaxConnections: 100,
		Timeout:        5 * time.Second,
		CORS: ui.CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
		},
		Options: map[string]interface{}{
			"buffer_size": 2048,
		},
	}

	err = transport.Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if !transport.ready {
		t.Error("Transport should be ready after initialization")
	}

	// Start should succeed after initialization
	err = transport.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Health check should pass
	err = transport.HealthCheck(ctx)
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	}

	// Stop should succeed
	err = transport.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if transport.ready {
		t.Error("Transport should not be ready after stop")
	}
}

func TestWebSocketTransport_HandlerCreation(t *testing.T) {
	transport := &WebSocketTransport{}

	// Initialize transport
	config := ui.TransportConfig{
		MaxConnections: 10,
		Timeout:        5 * time.Second,
	}
	transport.Initialize(config)

	// Create mock agent
	agent := uitesting.NewMockChatAgent("test-agent")

	// Create handler
	handler := transport.CreateHandler(agent)
	if handler == nil {
		t.Error("CreateHandler should return a valid handler")
	}
}

func TestWebSocketTransport_Integration(t *testing.T) {
	// Setup transport
	transport := &WebSocketTransport{}
	config := ui.TransportConfig{
		MaxConnections: 10,
		Timeout:        5 * time.Second,
		CORS: ui.CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
		},
	}
	transport.Initialize(config)
	transport.Start(context.Background())
	defer transport.Stop(context.Background())

	// Create mock agent
	agent := uitesting.NewMockChatAgent("test-agent")

	// Create test server
	handler := transport.CreateHandler(agent)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	u, _ := url.Parse(server.URL)
	u.Scheme = "ws"

	t.Run("BasicConnection", func(t *testing.T) {
		testBasicConnection(t, u.String())
	})

	t.Run("ChatMessage", func(t *testing.T) {
		testChatMessage(t, u.String())
	})

	t.Run("SessionManagement", func(t *testing.T) {
		testSessionManagement(t, u.String())
	})

	t.Run("PingPong", func(t *testing.T) {
		testPingPong(t, u.String())
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		testErrorHandling(t, u.String())
	})
}

func testBasicConnection(t *testing.T, wsURL string) {
	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Should receive welcome message
	var event ui.ChatEvent
	err = conn.ReadJSON(&event)
	if err != nil {
		t.Fatalf("Failed to read welcome message: %v", err)
	}

	if event.Type != "connected" {
		t.Errorf("Expected 'connected' event, got %s", event.Type)
	}

	// Verify client_id is present by parsing JSON
	var data map[string]interface{}
	err = json.Unmarshal([]byte(event.Data), &data)
	if err != nil {
		t.Error("Event data should be valid JSON")
	} else if _, hasClientID := data["client_id"]; !hasClientID {
		t.Error("Welcome message should include client_id")
	}
}

func testChatMessage(t *testing.T, wsURL string) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Read welcome message
	var event ui.ChatEvent
	conn.ReadJSON(&event)

	// Send chat message
	msg := wsMessage{
		Type:    "chat",
		Message: "Hello, AI!",
	}
	err = conn.WriteJSON(msg)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Should receive session creation event
	err = conn.ReadJSON(&event)
	if err != nil {
		t.Fatalf("Failed to read session event: %v", err)
	}

	if event.Type != "session_created" {
		t.Errorf("Expected 'session_created' event, got %s", event.Type)
	}

	// Should receive response message (from mock agent)
	err = conn.ReadJSON(&event)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if event.Type != ui.EventMessage {
		t.Errorf("Expected '%s' event, got %s", ui.EventMessage, event.Type)
	}

	// Verify response contains echo
	if !strings.Contains(event.Data, "Echo: Hello, AI!") {
		t.Errorf("Response should contain echo, got: %s", event.Data)
	}
}

func testSessionManagement(t *testing.T, wsURL string) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Read welcome message
	var event ui.ChatEvent
	conn.ReadJSON(&event)

	// Request session creation
	msg := wsMessage{
		Type: "session_create",
	}
	err = conn.WriteJSON(msg)
	if err != nil {
		t.Fatalf("Failed to send session create: %v", err)
	}

	// Should receive session created event
	err = conn.ReadJSON(&event)
	if err != nil {
		t.Fatalf("Failed to read session created: %v", err)
	}

	if event.Type != "session_created" {
		t.Errorf("Expected 'session_created', got %s", event.Type)
	}

	var data map[string]interface{}
	err = json.Unmarshal([]byte(event.Data), &data)
	if err != nil {
		t.Fatal("Session data should be valid JSON")
	}

	sessionID, ok := data["session_id"].(string)
	if !ok || sessionID == "" {
		t.Error("Session should have a valid ID")
	}

	// Request session info
	msg = wsMessage{
		Type:      "session_get",
		SessionID: sessionID,
	}
	err = conn.WriteJSON(msg)
	if err != nil {
		t.Fatalf("Failed to send session get: %v", err)
	}

	// Should receive session info
	err = conn.ReadJSON(&event)
	if err != nil {
		t.Fatalf("Failed to read session info: %v", err)
	}

	if event.Type != "session_info" {
		t.Errorf("Expected 'session_info', got %s", event.Type)
	}
}

func testPingPong(t *testing.T, wsURL string) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Read welcome message
	var event ui.ChatEvent
	conn.ReadJSON(&event)

	// Send ping message
	msg := wsMessage{
		Type: "ping",
	}
	err = conn.WriteJSON(msg)
	if err != nil {
		t.Fatalf("Failed to send ping: %v", err)
	}

	// Should receive pong response
	err = conn.ReadJSON(&event)
	if err != nil {
		t.Fatalf("Failed to read pong: %v", err)
	}

	if event.Type != "pong" {
		t.Errorf("Expected 'pong', got %s", event.Type)
	}
}

func testErrorHandling(t *testing.T, wsURL string) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Read welcome message
	var event ui.ChatEvent
	conn.ReadJSON(&event)

	// Send invalid message type
	msg := wsMessage{
		Type: "invalid_type",
	}
	err = conn.WriteJSON(msg)
	if err != nil {
		t.Fatalf("Failed to send invalid message: %v", err)
	}

	// Should receive error response
	err = conn.ReadJSON(&event)
	if err != nil {
		t.Fatalf("Failed to read error: %v", err)
	}

	if event.Type != ui.EventError {
		t.Errorf("Expected error event, got %s", event.Type)
	}

	// Send empty chat message
	msg = wsMessage{
		Type:    "chat",
		Message: "",
	}
	err = conn.WriteJSON(msg)
	if err != nil {
		t.Fatalf("Failed to send empty message: %v", err)
	}

	// Should receive error response
	err = conn.ReadJSON(&event)
	if err != nil {
		t.Fatalf("Failed to read error: %v", err)
	}

	if event.Type != ui.EventError {
		t.Errorf("Expected error event, got %s", event.Type)
	}
}

func TestWebSocketTransport_ClientExample(t *testing.T) {
	transport := &WebSocketTransport{}
	example := transport.ClientExample()

	if example == "" {
		t.Error("Client example should not be empty")
	}

	// Verify example contains key WebSocket concepts
	expectedTerms := []string{
		"WebSocket",
		"socket.onopen",
		"socket.onmessage",
		"socket.send",
		"JSON.stringify",
		"ping",
		"pong",
		"reconnect",
	}

	for _, term := range expectedTerms {
		if !strings.Contains(example, term) {
			t.Errorf("Client example should contain '%s'", term)
		}
	}
}

func TestWebSocketTransport_CORS(t *testing.T) {
	transport := &WebSocketTransport{}

	// Test with CORS disabled
	config := ui.TransportConfig{
		MaxConnections: 10,
		Timeout:        5 * time.Second,
		CORS: ui.CORSConfig{
			Enabled: false,
		},
	}

	err := transport.Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// CheckOrigin should return true when CORS is disabled
	req := &http.Request{
		Header: http.Header{
			"Origin": []string{"http://evil.example.com"},
		},
	}

	if !transport.upgrader.CheckOrigin(req) {
		t.Error("CheckOrigin should allow all origins when CORS is disabled")
	}

	// Test with CORS enabled and specific origins
	config.CORS.Enabled = true
	config.CORS.AllowedOrigins = []string{"http://localhost:3000", "https://example.com"}

	err = transport.Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Should allow configured origin
	req.Header.Set("Origin", "http://localhost:3000")
	if !transport.upgrader.CheckOrigin(req) {
		t.Error("CheckOrigin should allow configured origin")
	}

	// Should reject unconfigured origin
	req.Header.Set("Origin", "http://evil.example.com")
	if transport.upgrader.CheckOrigin(req) {
		t.Error("CheckOrigin should reject unconfigured origin")
	}

	// Test wildcard origin
	config.CORS.AllowedOrigins = []string{"*"}
	err = transport.Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if !transport.upgrader.CheckOrigin(req) {
		t.Error("CheckOrigin should allow all origins with wildcard")
	}
}

func TestWebSocketTransport_BufferConfiguration(t *testing.T) {
	transport := &WebSocketTransport{}

	// Test custom buffer sizes
	config := ui.TransportConfig{
		MaxConnections: 10,
		Timeout:        5 * time.Second,
		Options: map[string]interface{}{
			"read_buffer_size":  4096,
			"write_buffer_size": 8192,
		},
	}

	err := transport.Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if transport.upgrader.ReadBufferSize != 4096 {
		t.Errorf("Expected read buffer size 4096, got %d", transport.upgrader.ReadBufferSize)
	}

	if transport.upgrader.WriteBufferSize != 8192 {
		t.Errorf("Expected write buffer size 8192, got %d", transport.upgrader.WriteBufferSize)
	}

	// Test fallback to buffer_size option
	config.Options = map[string]interface{}{
		"buffer_size": 2048,
	}

	err = transport.Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if transport.upgrader.ReadBufferSize != 2048 {
		t.Errorf("Expected read buffer size 2048, got %d", transport.upgrader.ReadBufferSize)
	}

	if transport.upgrader.WriteBufferSize != 2048 {
		t.Errorf("Expected write buffer size 2048, got %d", transport.upgrader.WriteBufferSize)
	}
}

// Benchmark WebSocket message handling
func BenchmarkWebSocketTransport_MessageHandling(b *testing.B) {
	transport := &WebSocketTransport{}
	config := ui.TransportConfig{
		MaxConnections: 1000,
		Timeout:        30 * time.Second,
	}
	transport.Initialize(config)
	transport.Start(context.Background())
	defer transport.Stop(context.Background())

	agent := uitesting.NewMockChatAgent("bench-agent")
	handler := transport.CreateHandler(agent)
	server := httptest.NewServer(handler)
	defer server.Close()

	u, _ := url.Parse(server.URL)
	u.Scheme = "ws"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
			if err != nil {
				b.Fatalf("Failed to connect: %v", err)
			}

			// Skip welcome message
			var event ui.ChatEvent
			conn.ReadJSON(&event)

			// Send message
			msg := wsMessage{
				Type:    "chat",
				Message: "benchmark message",
			}
			conn.WriteJSON(msg)

			// Read response
			conn.ReadJSON(&event) // session_created
			conn.ReadJSON(&event) // response

			conn.Close()
		}
	})
}
