//go:build !websocket

// Package websocket provides a stub implementation when the websocket build tag is not used.
// This ensures graceful degradation when WebSocket dependencies are not available.
package websocket

import (
	"context"
	"fmt"
	"net/http"

	"github.com/itsneelabh/gomind/ui"
)

// WebSocketStub is a stub implementation that reports unavailability
type WebSocketStub struct{}

func init() {
	// Auto-register stub transport
	ui.MustRegister(&WebSocketStub{})
}

// Name returns the transport name
func (s *WebSocketStub) Name() string {
	return "websocket"
}

// Description returns a human-readable description
func (s *WebSocketStub) Description() string {
	return "WebSocket - not available (build with -tags websocket to enable)"
}

// Initialize always returns an error for the stub
func (s *WebSocketStub) Initialize(config ui.TransportConfig) error {
	return fmt.Errorf("WebSocket transport not available - build with -tags websocket to enable")
}

// Start always returns an error for the stub
func (s *WebSocketStub) Start(ctx context.Context) error {
	return fmt.Errorf("WebSocket transport not available - build with -tags websocket to enable")
}

// Stop is a no-op for the stub
func (s *WebSocketStub) Stop(ctx context.Context) error {
	return nil
}

// Available always returns false for the stub
func (s *WebSocketStub) Available() bool {
	return false
}

// Priority returns a low priority since it's not available
func (s *WebSocketStub) Priority() int {
	return 0 // Lowest priority when unavailable
}

// CreateHandler returns a handler that explains the transport is unavailable
func (s *WebSocketStub) CreateHandler(agent ui.ChatAgent) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		response := ui.ToErrorResponse(fmt.Errorf("WebSocket transport not available - build with -tags websocket to enable"))
		w.Write([]byte(fmt.Sprintf(`{
			"error": "%s",
			"code": "TRANSPORT_UNAVAILABLE",
			"details": {
				"transport": "websocket",
				"build_tag_required": "websocket",
				"help": "Add -tags websocket to your go build command to enable WebSocket support"
			}
		}`, response.Error)))
	})
}

// Capabilities returns empty capabilities for the stub
func (s *WebSocketStub) Capabilities() []ui.TransportCapability {
	return []ui.TransportCapability{} // No capabilities when unavailable
}

// HealthCheck always returns an error for the stub
func (s *WebSocketStub) HealthCheck(ctx context.Context) error {
	return fmt.Errorf("WebSocket transport not available")
}

// ClientExample returns instructions for enabling WebSocket support
func (s *WebSocketStub) ClientExample() string {
	return `# WebSocket Transport Not Available

The WebSocket transport is not available because this binary was built without WebSocket support.

## To enable WebSocket support:

1. Install the gorilla/websocket dependency:
   go get github.com/gorilla/websocket

2. Build with the websocket tag:
   go build -tags websocket

3. Then you can use the WebSocket transport:
   const socket = new WebSocket('ws://localhost:8080/chat/websocket');
   
   socket.onmessage = (event) => {
       const data = JSON.parse(event.data);
       console.log('Received:', data);
   };
   
   socket.send(JSON.stringify({
       type: 'chat',
       message: 'Hello!',
       session_id: 'optional-session-id'
   }));

## Alternative transports:

- SSE (Server-Sent Events): Available without additional dependencies
- HTTP polling: Basic request/response pattern`
}
