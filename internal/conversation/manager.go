package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ConversationConnectionManager handles HTTP-based conversational connections
// This provides a foundation for conversational agents without external WebSocket dependencies
type ConversationConnectionManager struct {
	sessions map[string]*ConversationSession
	mutex    sync.RWMutex
	agent    ConversationalAgent // Reference to the conversational agent
}

// ConversationSession represents an active conversation session
type ConversationSession struct {
	ID          string
	StartTime   time.Time
	LastMessage time.Time
	Messages    []ChatMessage
	Context     map[string]interface{}
	mutex       sync.RWMutex
}

// ChatMessage represents a chat message structure
type ChatMessage struct {
	Type      string                 `json:"type"`
	Content   string                 `json:"content"`
	SessionID string                 `json:"sessionId"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ConversationRequest represents an incoming conversation request
type ConversationRequest struct {
	SessionID string                 `json:"sessionId"`
	Message   string                 `json:"message"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// ConversationResponse represents a conversation response
type ConversationResponse struct {
	SessionID string                 `json:"sessionId"`
	Response  string                 `json:"response"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewConversationConnectionManager creates a new conversation connection manager
func NewConversationConnectionManager() *ConversationConnectionManager {
	return &ConversationConnectionManager{
		sessions: make(map[string]*ConversationSession),
		agent:    nil, // Will be set later when agent is available
	}
}

// SetAgent sets the conversational agent for this manager
func (ccm *ConversationConnectionManager) SetAgent(agent ConversationalAgent) {
	ccm.mutex.Lock()
	defer ccm.mutex.Unlock()
	ccm.agent = agent
}

// CreateSession creates a new conversation session
func (ccm *ConversationConnectionManager) CreateSession() *ConversationSession {
	session := &ConversationSession{
		ID:          generateSessionID(),
		StartTime:   time.Now(),
		LastMessage: time.Now(),
		Messages:    make([]ChatMessage, 0),
		Context:     make(map[string]interface{}),
	}

	ccm.mutex.Lock()
	ccm.sessions[session.ID] = session
	ccm.mutex.Unlock()

	return session
}

// GetSession retrieves a conversation session by ID
func (ccm *ConversationConnectionManager) GetSession(sessionID string) (*ConversationSession, bool) {
	ccm.mutex.RLock()
	defer ccm.mutex.RUnlock()

	session, exists := ccm.sessions[sessionID]
	return session, exists
}

// GetOrCreateSession gets an existing session or creates a new one
func (ccm *ConversationConnectionManager) GetOrCreateSession(sessionID string) *ConversationSession {
	if sessionID == "" {
		return ccm.CreateSession()
	}

	if session, exists := ccm.GetSession(sessionID); exists {
		return session
	}

	// Create session with specific ID
	session := &ConversationSession{
		ID:          sessionID,
		StartTime:   time.Now(),
		LastMessage: time.Now(),
		Messages:    make([]ChatMessage, 0),
		Context:     make(map[string]interface{}),
	}

	ccm.mutex.Lock()
	ccm.sessions[session.ID] = session
	ccm.mutex.Unlock()

	return session
}

// RemoveSession removes a conversation session
func (ccm *ConversationConnectionManager) RemoveSession(sessionID string) {
	ccm.mutex.Lock()
	defer ccm.mutex.Unlock()

	delete(ccm.sessions, sessionID)
}

// CleanupExpiredSessions removes sessions that haven't been active for a specified duration
func (ccm *ConversationConnectionManager) CleanupExpiredSessions(maxAge time.Duration) {
	ccm.mutex.Lock()
	defer ccm.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for sessionID, session := range ccm.sessions {
		session.mutex.RLock()
		expired := session.LastMessage.Before(cutoff)
		session.mutex.RUnlock()

		if expired {
			delete(ccm.sessions, sessionID)
		}
	}
}

// AddMessage adds a message to a session
func (cs *ConversationSession) AddMessage(message ChatMessage) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	message.SessionID = cs.ID
	message.Timestamp = time.Now()
	cs.Messages = append(cs.Messages, message)
	cs.LastMessage = time.Now()
}

// GetMessages returns all messages in the session
func (cs *ConversationSession) GetMessages() []ChatMessage {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	// Return a copy to prevent concurrent modification
	messages := make([]ChatMessage, len(cs.Messages))
	copy(messages, cs.Messages)
	return messages
}

// GetRecentMessages returns the last N messages
func (cs *ConversationSession) GetRecentMessages(count int) []ChatMessage {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	if count >= len(cs.Messages) {
		messages := make([]ChatMessage, len(cs.Messages))
		copy(messages, cs.Messages)
		return messages
	}

	start := len(cs.Messages) - count
	messages := make([]ChatMessage, count)
	copy(messages, cs.Messages[start:])
	return messages
}

// UpdateContext updates the session context
func (cs *ConversationSession) UpdateContext(key string, value interface{}) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	cs.Context[key] = value
}

// GetContext retrieves a value from the session context
func (cs *ConversationSession) GetContext(key string) (interface{}, bool) {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	value, exists := cs.Context[key]
	return value, exists
}

// GetContextCopy returns a copy of the entire context
func (cs *ConversationSession) GetContextCopy() map[string]interface{} {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	contextCopy := make(map[string]interface{})
	for k, v := range cs.Context {
		contextCopy[k] = v
	}
	return contextCopy
}

// HandleConversationRequest processes a conversation request and returns a response
func (ccm *ConversationConnectionManager) HandleConversationRequest(req ConversationRequest) (*ConversationResponse, error) {
	// Get or create session
	session := ccm.GetOrCreateSession(req.SessionID)

	// Add the incoming message to the session
	userMessage := ChatMessage{
		Type:      "user",
		Content:   req.Message,
		SessionID: session.ID,
		Timestamp: time.Now(),
		Metadata:  req.Context,
	}
	session.AddMessage(userMessage)

	// Update session context if provided
	if req.Context != nil {
		for k, v := range req.Context {
			session.UpdateContext(k, v)
		}
	}

	// Process the message (this would integrate with the conversational agent)
	responseContent := ccm.processMessage(session, req.Message)

	// Add the response message to the session
	responseMessage := ChatMessage{
		Type:      "assistant",
		Content:   responseContent,
		SessionID: session.ID,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"processed": true,
		},
	}
	session.AddMessage(responseMessage)

	// Create response
	response := &ConversationResponse{
		SessionID: session.ID,
		Response:  responseContent,
		Context:   session.GetContextCopy(),
		Timestamp: time.Now(),
	}

	return response, nil
}

// processMessage processes a user message and generates a response
func (ccm *ConversationConnectionManager) processMessage(session *ConversationSession, message string) string {
	// If we have a conversational agent, use it
	if ccm.agent != nil {
		// Convert session context to agent message format
		agentMessage := Message{
			Text:      message,
			SessionID: session.ID,
			UserID:    "web-user", // Default user ID for web interface
			Metadata:  session.GetContextCopy(),
		}

		// Call the agent's conversation handler
		response, err := ccm.agent.HandleConversation(context.Background(), agentMessage)
		if err != nil {
			return fmt.Sprintf("Sorry, I encountered an error processing your message: %v", err)
		}

		// Update session context with any metadata from the response
		if response.Metadata != nil {
			for k, v := range response.Metadata {
				session.UpdateContext(k, v)
			}
		}

		return response.Text
	}

	// Fallback implementation if no agent is provided
	recentMessages := session.GetRecentMessages(5)

	// Simple echo response with context awareness
	if len(recentMessages) == 1 {
		return fmt.Sprintf("Hello! You said: %s. How can I help you today?", message)
	}

	return fmt.Sprintf("I understand you're saying: %s. Based on our conversation, I can see we've exchanged %d messages.", message, len(recentMessages))
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	// Simple ID generation - in production, use a proper UUID library
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}

// ServeConversationHTTP provides an HTTP handler for conversation requests
func (ccm *ConversationConnectionManager) ServeConversationHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	response, err := ccm.HandleConversationRequest(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error processing request: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}
