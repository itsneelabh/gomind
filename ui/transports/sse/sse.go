// Package sse provides Server-Sent Events transport for UI communication.
// This transport is always included (zero dependencies beyond standard library).
package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/itsneelabh/gomind/ui"
)

// SSETransport implements Server-Sent Events transport
type SSETransport struct {
	config ui.TransportConfig
	ready  bool
}

func init() {
	// Auto-register with transport registry
	ui.RegisterTransport(&SSETransport{})
}

// Name returns the transport name
func (t *SSETransport) Name() string {
	return "sse"
}

// Description returns a human-readable description
func (t *SSETransport) Description() string {
	return "Server-Sent Events - lightweight, one-way streaming"
}

// Initialize configures the transport
func (t *SSETransport) Initialize(config ui.TransportConfig) error {
	t.config = config
	t.ready = true
	return nil
}

// Start starts the transport
func (t *SSETransport) Start(ctx context.Context) error {
	if !t.ready {
		return fmt.Errorf("transport not initialized")
	}
	return nil
}

// Stop stops the transport
func (t *SSETransport) Stop(ctx context.Context) error {
	t.ready = false
	return nil
}

// Available checks if this transport can be used
func (t *SSETransport) Available() bool {
	return true // SSE is always available
}

// Priority returns the transport priority for auto-selection
func (t *SSETransport) Priority() int {
	return 100 // Default priority
}

// CreateHandler creates an HTTP handler for SSE communication
func (t *SSETransport) CreateHandler(agent ui.ChatAgent) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if SSE is supported
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // Disable Nginx buffering
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Parse request
		message := r.FormValue("message")
		sessionID := r.FormValue("session")

		if message == "" {
			t.sendError(w, flusher, "message parameter required")
			return
		}

		// Create or get session
		if sessionID == "" {
			session, err := agent.CreateSession(r.Context())
			if err != nil {
				t.sendError(w, flusher, fmt.Sprintf("failed to create session: %v", err))
				return
			}
			sessionID = session.ID

			// Send session info
			t.sendEvent(w, flusher, "session", map[string]string{
				"id": sessionID,
			})
		}

		// Check rate limit
		allowed, err := agent.CheckRateLimit(r.Context(), sessionID)
		if err != nil {
			t.sendError(w, flusher, fmt.Sprintf("rate limit check failed: %v", err))
			return
		}
		if !allowed {
			t.sendError(w, flusher, "rate limit exceeded")
			return
		}

		// Stream response
		stream, err := agent.StreamResponse(r.Context(), sessionID, message)
		if err != nil {
			t.sendError(w, flusher, fmt.Sprintf("failed to stream response: %v", err))
			return
		}

		// Stream events
		for event := range stream {
			if err := t.sendEvent(w, flusher, string(event.Type), event.Data); err != nil {
				// Client disconnected
				return
			}
		}

		// Send completion event
		t.sendEvent(w, flusher, "done", map[string]bool{"finished": true})
	})
}

// Capabilities returns the capabilities this transport provides
func (t *SSETransport) Capabilities() []ui.TransportCapability {
	return []ui.TransportCapability{
		ui.CapabilityStreaming,
		ui.CapabilityReconnect,
	}
}

// HealthCheck performs a health check
func (t *SSETransport) HealthCheck(ctx context.Context) error {
	if !t.ready {
		return fmt.Errorf("transport not ready")
	}
	return nil
}

// ClientExample returns example client code
func (t *SSETransport) ClientExample() string {
	return `// JavaScript SSE Client Example
const eventSource = new EventSource('/chat/sse?message=Hello&session=123');

// Handle messages
eventSource.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log('Received:', data);
};

// Handle specific events
eventSource.addEventListener('session', (event) => {
    const session = JSON.parse(event.data);
    console.log('Session ID:', session.id);
});

eventSource.addEventListener('error', (event) => {
    const error = JSON.parse(event.data);
    console.error('Error:', error.message);
});

eventSource.addEventListener('done', (event) => {
    console.log('Stream complete');
    eventSource.close();
});

// Handle connection errors
eventSource.onerror = (error) => {
    console.error('Connection error:', error);
    eventSource.close();
};`
}

// sendEvent sends an SSE event
func (t *SSETransport) sendEvent(w http.ResponseWriter, flusher http.Flusher, eventType string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonData)
	if err != nil {
		return err
	}

	flusher.Flush()
	return nil
}

// sendError sends an error event
func (t *SSETransport) sendError(w http.ResponseWriter, flusher http.Flusher, message string) {
	t.sendEvent(w, flusher, "error", map[string]string{
		"message": message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}