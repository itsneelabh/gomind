package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MockSessionManager implements SessionManager interface for testing
// without Redis dependency
type MockSessionManager struct {
	mu           sync.RWMutex
	sessions     map[string]*Session
	messages     map[string][]Message
	rateLimits   map[string]*rateLimitEntry
	config       SessionConfig
	shouldFail   bool // For testing error conditions
	failPattern  string // Specific method to fail
}

type rateLimitEntry struct {
	count     int
	resetTime time.Time
}

// NewMockSessionManager creates a new mock session manager for testing
func NewMockSessionManager(config SessionConfig) *MockSessionManager {
	return &MockSessionManager{
		sessions:   make(map[string]*Session),
		messages:   make(map[string][]Message),
		rateLimits: make(map[string]*rateLimitEntry),
		config:     config,
	}
}

// SetFailure configures the mock to fail for testing
func (m *MockSessionManager) SetFailure(shouldFail bool, pattern string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = shouldFail
	m.failPattern = pattern
}

// Create creates a new session
func (m *MockSessionManager) Create(ctx context.Context, metadata map[string]interface{}) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail && (m.failPattern == "" || m.failPattern == "Create") {
		return nil, fmt.Errorf("mock failure: Create")
	}

	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	session := &Session{
		ID:          uuid.New().String(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(m.config.TTL),
		TokenCount:  0,
		MessageCount: 0,
		Metadata:    metadata,
	}

	m.sessions[session.ID] = session
	m.messages[session.ID] = make([]Message, 0)

	return session, nil
}

// Get retrieves a session by ID
func (m *MockSessionManager) Get(ctx context.Context, sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldFail && (m.failPattern == "" || m.failPattern == "Get") {
		return nil, fmt.Errorf("mock failure: Get")
	}

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Check if expired
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session expired: %s", sessionID)
	}

	// Return a copy to avoid concurrent modification
	sessionCopy := *session
	return &sessionCopy, nil
}

// Update updates a session
func (m *MockSessionManager) Update(ctx context.Context, session *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail && (m.failPattern == "" || m.failPattern == "Update") {
		return fmt.Errorf("mock failure: Update")
	}

	if _, exists := m.sessions[session.ID]; !exists {
		return fmt.Errorf("session not found: %s", session.ID)
	}

	session.UpdatedAt = time.Now()
	m.sessions[session.ID] = session

	return nil
}

// Delete deletes a session
func (m *MockSessionManager) Delete(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail && (m.failPattern == "" || m.failPattern == "Delete") {
		return fmt.Errorf("mock failure: Delete")
	}

	delete(m.sessions, sessionID)
	delete(m.messages, sessionID)
	delete(m.rateLimits, sessionID)

	return nil
}

// AddMessage adds a message to a session
func (m *MockSessionManager) AddMessage(ctx context.Context, sessionID string, msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail && (m.failPattern == "" || m.failPattern == "AddMessage") {
		return fmt.Errorf("mock failure: AddMessage")
	}

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	msg.ID = uuid.New().String()
	msg.SessionID = sessionID
	msg.Timestamp = time.Now()

	// Add message
	if m.messages[sessionID] == nil {
		m.messages[sessionID] = make([]Message, 0)
	}
	m.messages[sessionID] = append(m.messages[sessionID], msg)

	// Maintain sliding window
	if len(m.messages[sessionID]) > m.config.MaxMessages {
		m.messages[sessionID] = m.messages[sessionID][len(m.messages[sessionID])-m.config.MaxMessages:]
	}

	// Update session counters
	session.MessageCount++
	session.TokenCount += msg.TokenCount
	session.UpdatedAt = time.Now()

	return nil
}

// GetMessages retrieves messages for a session
func (m *MockSessionManager) GetMessages(ctx context.Context, sessionID string, limit int) ([]Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldFail && (m.failPattern == "" || m.failPattern == "GetMessages") {
		return nil, fmt.Errorf("mock failure: GetMessages")
	}

	messages, exists := m.messages[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	if limit <= 0 || limit > len(messages) {
		limit = len(messages)
	}

	// Return last N messages
	start := len(messages) - limit
	if start < 0 {
		start = 0
	}

	result := make([]Message, len(messages[start:]))
	copy(result, messages[start:])

	return result, nil
}

// CheckRateLimit checks if a session has exceeded rate limit
func (m *MockSessionManager) CheckRateLimit(ctx context.Context, sessionID string) (bool, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail && (m.failPattern == "" || m.failPattern == "CheckRateLimit") {
		return false, time.Time{}, fmt.Errorf("mock failure: CheckRateLimit")
	}

	if _, exists := m.sessions[sessionID]; !exists {
		return false, time.Time{}, fmt.Errorf("session not found: %s", sessionID)
	}

	now := time.Now()
	entry, exists := m.rateLimits[sessionID]

	if !exists || now.After(entry.resetTime) {
		// Create new rate limit window
		resetTime := now.Add(m.config.RateLimitWindow)
		m.rateLimits[sessionID] = &rateLimitEntry{
			count:     1,
			resetTime: resetTime,
		}
		return true, resetTime, nil
	}

	entry.count++
	return entry.count <= m.config.RateLimitMax, entry.resetTime, nil
}

// GetRateLimit gets current rate limit count for a session
func (m *MockSessionManager) GetRateLimit(ctx context.Context, sessionID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldFail && (m.failPattern == "" || m.failPattern == "GetRateLimit") {
		return 0, fmt.Errorf("mock failure: GetRateLimit")
	}

	entry, exists := m.rateLimits[sessionID]
	if !exists {
		return 0, nil
	}

	if time.Now().After(entry.resetTime) {
		return 0, nil
	}

	return entry.count, nil
}

// ListActiveSessions returns all active session IDs
func (m *MockSessionManager) ListActiveSessions(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldFail && (m.failPattern == "" || m.failPattern == "ListActiveSessions") {
		return nil, fmt.Errorf("mock failure: ListActiveSessions")
	}

	now := time.Now()
	sessions := make([]string, 0, len(m.sessions))

	for id, session := range m.sessions {
		if now.Before(session.ExpiresAt) {
			sessions = append(sessions, id)
		}
	}

	return sessions, nil
}

// CleanupExpiredSessions removes expired sessions
func (m *MockSessionManager) CleanupExpiredSessions(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail && (m.failPattern == "" || m.failPattern == "CleanupExpiredSessions") {
		return fmt.Errorf("mock failure: CleanupExpiredSessions")
	}

	now := time.Now()
	for id, session := range m.sessions {
		if now.After(session.ExpiresAt) {
			delete(m.sessions, id)
			delete(m.messages, id)
			delete(m.rateLimits, id)
		}
	}

	return nil
}

// GetAllSessions returns all sessions (testing helper)
func (m *MockSessionManager) GetAllSessions() map[string]*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Session)
	for k, v := range m.sessions {
		result[k] = v
	}
	return result
}

// GetAllMessages returns all messages (testing helper)
func (m *MockSessionManager) GetAllMessages() map[string][]Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]Message)
	for k, v := range m.messages {
		result[k] = v
	}
	return result
}

// GetActiveSessionCount returns the number of active sessions
func (m *MockSessionManager) GetActiveSessionCount(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldFail && (m.failPattern == "" || m.failPattern == "GetActiveSessionCount") {
		return 0, fmt.Errorf("mock failure: GetActiveSessionCount")
	}

	// Count non-expired sessions
	count := int64(0)
	now := time.Now()
	for _, session := range m.sessions {
		if session.ExpiresAt.After(now) {
			count++
		}
	}
	return count, nil
}

// GetSessionsByMetadata retrieves sessions by metadata key-value pair
func (m *MockSessionManager) GetSessionsByMetadata(ctx context.Context, key, value string) ([]*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldFail && (m.failPattern == "" || m.failPattern == "GetSessionsByMetadata") {
		return nil, fmt.Errorf("mock failure: GetSessionsByMetadata")
	}

	var sessions []*Session
	for _, session := range m.sessions {
		if val, exists := session.Metadata[key]; exists && val == value {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}