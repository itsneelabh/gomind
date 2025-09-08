//go:build websocket

// Package websocket provides WebSocket transport for UI communication.
// This transport requires the 'websocket' build tag to be included.
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/itsneelabh/gomind/ui"
)

// WebSocketTransport implements WebSocket transport with bidirectional communication
type WebSocketTransport struct {
	config      ui.TransportConfig
	ready       bool
	initialized bool
	upgrader    websocket.Upgrader
	clients     map[string]*wsClient
	clientsMu   sync.RWMutex
}

// wsClient represents a WebSocket client connection
type wsClient struct {
	conn      *websocket.Conn
	send      chan ui.ChatEvent
	sessionID string
	agent     ui.ChatAgent
	mu        sync.RWMutex
	closed    bool
}

// wsMessage represents a WebSocket message structure
type wsMessage struct {
	Type      string                 `json:"type"`
	SessionID string                 `json:"session_id,omitempty"`
	Message   string                 `json:"message,omitempty"`
	Data      interface{}            `json:"data,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

func init() {
	// Auto-register with transport registry
	ui.MustRegister(&WebSocketTransport{})
}

// Name returns the transport name
func (t *WebSocketTransport) Name() string {
	return "websocket"
}

// Description returns a human-readable description
func (t *WebSocketTransport) Description() string {
	return "WebSocket - bidirectional, real-time communication with auto-reconnection"
}

// Initialize configures the transport
func (t *WebSocketTransport) Initialize(config ui.TransportConfig) error {
	t.config = config
	
	// Configure WebSocket upgrader
	t.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// Check CORS configuration
			if !config.CORS.Enabled {
				return true
			}
			
			origin := r.Header.Get("Origin")
			if len(config.CORS.AllowedOrigins) == 0 {
				return true
			}
			
			for _, allowed := range config.CORS.AllowedOrigins {
				if allowed == "*" || allowed == origin {
					return true
				}
			}
			return false
		},
		HandshakeTimeout: config.Timeout,
	}
	
	// Configure buffer sizes from options
	if config.Options != nil {
		if bufferSize, ok := config.Options["buffer_size"].(int); ok && bufferSize > 0 {
			t.upgrader.ReadBufferSize = bufferSize
			t.upgrader.WriteBufferSize = bufferSize
		}
		if readBuffer, ok := config.Options["read_buffer_size"].(int); ok && readBuffer > 0 {
			t.upgrader.ReadBufferSize = readBuffer
		}
		if writeBuffer, ok := config.Options["write_buffer_size"].(int); ok && writeBuffer > 0 {
			t.upgrader.WriteBufferSize = writeBuffer
		}
	}
	
	t.clients = make(map[string]*wsClient)
	t.initialized = true
	t.ready = true
	return nil
}

// Start starts the transport
func (t *WebSocketTransport) Start(ctx context.Context) error {
	if !t.initialized {
		return fmt.Errorf("transport not initialized")
	}
	t.ready = true
	return nil
}

// Stop stops the transport and closes all connections
func (t *WebSocketTransport) Stop(ctx context.Context) error {
	t.clientsMu.Lock()
	defer t.clientsMu.Unlock()
	
	// Close all client connections
	for clientID, client := range t.clients {
		client.close()
		delete(t.clients, clientID)
	}
	
	t.ready = false
	return nil
}

// Available checks if this transport can be used
func (t *WebSocketTransport) Available() bool {
	return true // WebSocket is available when compiled with websocket tag
}

// Priority returns the transport priority for auto-selection
func (t *WebSocketTransport) Priority() int {
	return 150 // Higher than SSE (100) because bidirectional
}

// CreateHandler creates an HTTP handler for WebSocket communication
func (t *WebSocketTransport) CreateHandler(agent ui.ChatAgent) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Upgrade HTTP connection to WebSocket
		conn, err := t.upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, fmt.Sprintf("WebSocket upgrade failed: %v", err), http.StatusBadRequest)
			return
		}

		// Create client instance
		client := &wsClient{
			conn:  conn,
			send:  make(chan ui.ChatEvent, 256),
			agent: agent,
		}

		// Generate client ID for tracking
		clientID := fmt.Sprintf("%p", client)
		
		// Register client
		t.clientsMu.Lock()
		t.clients[clientID] = client
		t.clientsMu.Unlock()

		// Start client handlers
		go client.writePump()
		go client.readPump(t, clientID)
		
		// Send welcome message
		welcomeData := map[string]interface{}{
			"client_id": clientID,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		welcomeJSON, _ := json.Marshal(welcomeData)
		client.send <- ui.ChatEvent{
			Type: "connected",
			Data: string(welcomeJSON),
			Timestamp: time.Now(),
		}
	})
}

// Capabilities returns the capabilities this transport provides
func (t *WebSocketTransport) Capabilities() []ui.TransportCapability {
	return []ui.TransportCapability{
		ui.CapabilityStreaming,
		ui.CapabilityBidirectional,
		ui.CapabilityReconnect,
		ui.CapabilityMultiplex,
	}
}

// HealthCheck performs a health check
func (t *WebSocketTransport) HealthCheck(ctx context.Context) error {
	if !t.ready {
		return fmt.Errorf("transport not ready")
	}
	
	// Health check passes if transport is ready
	// Could add more sophisticated health checks here like checking client connections
	return nil
}

// ClientExample returns example client code
func (t *WebSocketTransport) ClientExample() string {
	return `// JavaScript WebSocket Client Example
const socket = new WebSocket('ws://localhost:8080/chat/websocket');

// Connection opened
socket.onopen = (event) => {
    console.log('WebSocket connected');
    
    // Send a chat message
    socket.send(JSON.stringify({
        type: 'chat',
        message: 'Hello, AI!',
        session_id: 'optional-session-id'
    }));
};

// Handle incoming messages
socket.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log('Received:', data);
    
    switch (data.type) {
        case 'connected':
            console.log('Client ID:', data.data.client_id);
            break;
        case 'session_created':
            console.log('Session ID:', data.data.session_id);
            break;
        case 'message':
            console.log('AI Response:', data.data);
            break;
        case 'error':
            console.error('Error:', data.data.message);
            break;
        case 'done':
            console.log('Response complete');
            break;
    }
};

// Handle ping/pong for keep-alive
socket.onping = () => {
    socket.pong();
};

// Handle connection close
socket.onclose = (event) => {
    console.log('WebSocket closed:', event.code, event.reason);
    
    // Auto-reconnect after 3 seconds
    setTimeout(() => {
        console.log('Attempting to reconnect...');
        // Recreate connection
    }, 3000);
};

// Handle errors
socket.onerror = (error) => {
    console.error('WebSocket error:', error);
};

// Send ping every 30 seconds for keep-alive
setInterval(() => {
    if (socket.readyState === WebSocket.OPEN) {
        socket.ping();
    }
}, 30000);`
}

// writePump handles sending messages to the WebSocket client
func (c *wsClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case event, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Channel closed, close WebSocket
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send the chat event as JSON
			if err := c.conn.WriteJSON(event); err != nil {
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump handles incoming messages from the WebSocket client
func (c *wsClient) readPump(transport *WebSocketTransport, clientID string) {
	defer func() {
		// Clean up client when connection closes
		transport.clientsMu.Lock()
		delete(transport.clients, clientID)
		transport.clientsMu.Unlock()
		
		c.close()
	}()

	// Set read deadline and pong handler
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg wsMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log unexpected close errors
			}
			break
		}

		// Handle different message types
		switch msg.Type {
		case "chat":
			c.handleChatMessage(msg)
		case "ping":
			c.handlePing()
		case "session_create":
			c.handleSessionCreate(msg)
		case "session_get":
			c.handleSessionGet(msg)
		default:
			c.sendError(fmt.Sprintf("unknown message type: %s", msg.Type))
		}
	}
}

// handleChatMessage processes a chat message from the client
func (c *wsClient) handleChatMessage(msg wsMessage) {
	if msg.Message == "" {
		c.sendError("message cannot be empty")
		return
	}

	ctx := context.Background()

	// Create or get session
	sessionID := msg.SessionID
	if sessionID == "" {
		session, err := c.agent.CreateSession(ctx)
		if err != nil {
			c.sendError(fmt.Sprintf("failed to create session: %v", err))
			return
		}
		sessionID = session.ID
		c.sessionID = sessionID

		// Notify client of new session
		sessionData := map[string]interface{}{
			"session_id": sessionID,
		}
		sessionJSON, _ := json.Marshal(sessionData)
		c.send <- ui.ChatEvent{
			Type: "session_created",
			Data: string(sessionJSON),
			Timestamp: time.Now(),
		}
	}

	// Check rate limit
	allowed, err := c.agent.CheckRateLimit(ctx, sessionID)
	if err != nil {
		c.sendError(fmt.Sprintf("rate limit check failed: %v", err))
		return
	}
	if !allowed {
		c.sendError("rate limit exceeded")
		return
	}

	// Stream response
	stream, err := c.agent.StreamResponse(ctx, sessionID, msg.Message)
	if err != nil {
		c.sendError(fmt.Sprintf("failed to stream response: %v", err))
		return
	}

	// Forward stream events to WebSocket
	go func() {
		for event := range stream {
			select {
			case c.send <- event:
				// Event sent successfully
			default:
				// Channel full, connection might be dead
				return
			}
		}
	}()
}

// handlePing responds to ping messages
func (c *wsClient) handlePing() {
	pongData := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
	}
	pongJSON, _ := json.Marshal(pongData)
	c.send <- ui.ChatEvent{
		Type: "pong",
		Data: string(pongJSON),
		Timestamp: time.Now(),
	}
}

// handleSessionCreate creates a new session
func (c *wsClient) handleSessionCreate(msg wsMessage) {
	ctx := context.Background()
	
	session, err := c.agent.CreateSession(ctx)
	if err != nil {
		c.sendError(fmt.Sprintf("failed to create session: %v", err))
		return
	}
	
	c.sessionID = session.ID
	sessionCreateData := map[string]interface{}{
		"session_id": session.ID,
		"created_at": session.CreatedAt.Format(time.RFC3339),
		"expires_at": session.ExpiresAt.Format(time.RFC3339),
	}
	sessionCreateJSON, _ := json.Marshal(sessionCreateData)
	c.send <- ui.ChatEvent{
		Type: "session_created",
		Data: string(sessionCreateJSON),
		Timestamp: time.Now(),
	}
}

// handleSessionGet retrieves session information
func (c *wsClient) handleSessionGet(msg wsMessage) {
	if msg.SessionID == "" {
		c.sendError("session_id required")
		return
	}

	ctx := context.Background()
	session, err := c.agent.GetSession(ctx, msg.SessionID)
	if err != nil {
		c.sendError(fmt.Sprintf("failed to get session: %v", err))
		return
	}

	sessionInfoData := map[string]interface{}{
		"session_id":    session.ID,
		"created_at":    session.CreatedAt.Format(time.RFC3339),
		"updated_at":    session.UpdatedAt.Format(time.RFC3339),
		"expires_at":    session.ExpiresAt.Format(time.RFC3339),
		"token_count":   session.TokenCount,
		"message_count": session.MessageCount,
		"metadata":      session.Metadata,
	}
	sessionInfoJSON, _ := json.Marshal(sessionInfoData)
	c.send <- ui.ChatEvent{
		Type: "session_info",
		Data: string(sessionInfoJSON),
		Timestamp: time.Now(),
	}
}

// sendError sends an error message to the client
func (c *wsClient) sendError(message string) {
	errorData := map[string]interface{}{
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	errorJSON, _ := json.Marshal(errorData)
	c.send <- ui.ChatEvent{
		Type: ui.EventError,
		Data: string(errorJSON),
		Timestamp: time.Now(),
	}
}

// close closes the client connection and channels
func (c *wsClient) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if !c.closed {
		c.closed = true
		close(c.send)
		c.conn.Close()
	}
}