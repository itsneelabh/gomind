// Package testing provides mock implementations and testing utilities for the UI framework
package testing

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/ui"
)

// MockTransport is a configurable mock implementation of Transport for testing
type MockTransport struct {
	name         string
	description  string
	priority     int
	capabilities []ui.TransportCapability
	available    bool
	healthy      bool
	
	// Behavior configuration
	InitializeFunc   func(config ui.TransportConfig) error
	StartFunc        func(ctx context.Context) error
	StopFunc         func(ctx context.Context) error
	CreateHandlerFunc func(agent ui.ChatAgent) http.Handler
	HealthCheckFunc  func(ctx context.Context) error
	
	// State tracking
	initialized bool
	started     bool
	stopped     bool
	calls       map[string]int
	mu          sync.RWMutex
}

// NewMockTransport creates a new mock transport with default behavior
func NewMockTransport(name string) *MockTransport {
	return &MockTransport{
		name:         name,
		description:  fmt.Sprintf("Mock transport %s", name),
		priority:     50,
		capabilities: []ui.TransportCapability{ui.CapabilityStreaming},
		available:    true,
		healthy:      true,
		calls:        make(map[string]int),
	}
}

// Initialize implements Transport.Initialize
func (m *MockTransport) Initialize(config ui.TransportConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.calls["Initialize"]++
	
	if m.InitializeFunc != nil {
		return m.InitializeFunc(config)
	}
	
	m.initialized = true
	return nil
}

// Start implements Transport.Start
func (m *MockTransport) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.calls["Start"]++
	
	if m.StartFunc != nil {
		return m.StartFunc(ctx)
	}
	
	if !m.initialized {
		return fmt.Errorf("transport not initialized")
	}
	
	m.started = true
	return nil
}

// Stop implements Transport.Stop
func (m *MockTransport) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.calls["Stop"]++
	
	if m.StopFunc != nil {
		return m.StopFunc(ctx)
	}
	
	m.started = false
	m.stopped = true
	return nil
}

// CreateHandler implements Transport.CreateHandler
func (m *MockTransport) CreateHandler(agent ui.ChatAgent) http.Handler {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.calls["CreateHandler"]++
	
	if m.CreateHandlerFunc != nil {
		return m.CreateHandlerFunc(agent)
	}
	
	// Return a simple handler that responds with transport name
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Mock transport: %s", m.name)
	})
}

// Name implements Transport.Name
func (m *MockTransport) Name() string {
	return m.name
}

// Description implements Transport.Description
func (m *MockTransport) Description() string {
	return m.description
}

// Priority implements Transport.Priority
func (m *MockTransport) Priority() int {
	return m.priority
}

// Capabilities implements Transport.Capabilities
func (m *MockTransport) Capabilities() []ui.TransportCapability {
	return m.capabilities
}

// HealthCheck implements Transport.HealthCheck
func (m *MockTransport) HealthCheck(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.calls["HealthCheck"]++
	
	if m.HealthCheckFunc != nil {
		return m.HealthCheckFunc(ctx)
	}
	
	if !m.healthy {
		return fmt.Errorf("transport unhealthy")
	}
	
	return nil
}

// Available implements Transport.Available
func (m *MockTransport) Available() bool {
	return m.available
}

// GetCallCount returns the number of times a method was called
func (m *MockTransport) GetCallCount(method string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.calls[method]
}

// SetAvailable sets the availability of the transport
func (m *MockTransport) SetAvailable(available bool) {
	m.available = available
}

// SetHealthy sets the health status of the transport
func (m *MockTransport) SetHealthy(healthy bool) {
	m.healthy = healthy
}

// MockSessionManager is a configurable mock implementation of SessionManager
type MockSessionManager struct {
	sessions map[string]*ui.Session
	messages map[string][]ui.Message
	rateLimit map[string]int
	mu       sync.RWMutex
	
	// Behavior configuration
	CreateFunc        func(ctx context.Context, metadata map[string]interface{}) (*ui.Session, error)
	GetFunc           func(ctx context.Context, sessionID string) (*ui.Session, error)
	UpdateFunc        func(ctx context.Context, session *ui.Session) error
	DeleteFunc        func(ctx context.Context, sessionID string) error
	AddMessageFunc    func(ctx context.Context, sessionID string, msg ui.Message) error
	GetMessagesFunc   func(ctx context.Context, sessionID string, limit int) ([]ui.Message, error)
	CheckRateLimitFunc func(ctx context.Context, sessionID string) (bool, time.Time, error)
}

// NewMockSessionManager creates a new mock session manager
func NewMockSessionManager() *MockSessionManager {
	return &MockSessionManager{
		sessions:  make(map[string]*ui.Session),
		messages:  make(map[string][]ui.Message),
		rateLimit: make(map[string]int),
	}
}

// Create implements SessionManager.Create
func (m *MockSessionManager) Create(ctx context.Context, metadata map[string]interface{}) (*ui.Session, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, metadata)
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	session := &ui.Session{
		ID:           uuid.New().String(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		TokenCount:   0,
		MessageCount: 0,
		Metadata:     metadata,
	}
	
	m.sessions[session.ID] = session
	m.messages[session.ID] = []ui.Message{}
	
	return session, nil
}

// Get implements SessionManager.Get
func (m *MockSessionManager) Get(ctx context.Context, sessionID string) (*ui.Session, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, sessionID)
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, ui.ErrSessionNotFound
	}
	
	return session, nil
}

// Update implements SessionManager.Update
func (m *MockSessionManager) Update(ctx context.Context, session *ui.Session) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, session)
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.sessions[session.ID]; !exists {
		return ui.ErrSessionNotFound
	}
	
	session.UpdatedAt = time.Now()
	m.sessions[session.ID] = session
	
	return nil
}

// Delete implements SessionManager.Delete
func (m *MockSessionManager) Delete(ctx context.Context, sessionID string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, sessionID)
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.sessions[sessionID]; !exists {
		return ui.ErrSessionNotFound
	}
	
	delete(m.sessions, sessionID)
	delete(m.messages, sessionID)
	delete(m.rateLimit, sessionID)
	
	return nil
}

// AddMessage implements SessionManager.AddMessage
func (m *MockSessionManager) AddMessage(ctx context.Context, sessionID string, msg ui.Message) error {
	if m.AddMessageFunc != nil {
		return m.AddMessageFunc(ctx, sessionID, msg)
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return ui.ErrSessionNotFound
	}
	
	// Add message
	m.messages[sessionID] = append(m.messages[sessionID], msg)
	
	// Update session
	session.MessageCount++
	session.TokenCount += msg.TokenCount
	session.UpdatedAt = time.Now()
	
	return nil
}

// GetMessages implements SessionManager.GetMessages
func (m *MockSessionManager) GetMessages(ctx context.Context, sessionID string, limit int) ([]ui.Message, error) {
	if m.GetMessagesFunc != nil {
		return m.GetMessagesFunc(ctx, sessionID, limit)
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	messages, exists := m.messages[sessionID]
	if !exists {
		return nil, ui.ErrSessionNotFound
	}
	
	// Return last 'limit' messages
	if limit > 0 && len(messages) > limit {
		return messages[len(messages)-limit:], nil
	}
	
	return messages, nil
}

// CheckRateLimit implements SessionManager.CheckRateLimit
func (m *MockSessionManager) CheckRateLimit(ctx context.Context, sessionID string) (bool, time.Time, error) {
	if m.CheckRateLimitFunc != nil {
		return m.CheckRateLimitFunc(ctx, sessionID)
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.sessions[sessionID]; !exists {
		return false, time.Time{}, ui.ErrSessionNotFound
	}
	
	count := m.rateLimit[sessionID]
	m.rateLimit[sessionID]++
	
	// Simple rate limit: 10 requests per session
	if count >= 10 {
		return false, time.Now().Add(time.Minute), nil
	}
	
	return true, time.Time{}, nil
}

// GetActiveSessionCount implements SessionManager.GetActiveSessionCount
func (m *MockSessionManager) GetActiveSessionCount(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.sessions)), nil
}

// GetSessionsByMetadata implements SessionManager.GetSessionsByMetadata
func (m *MockSessionManager) GetSessionsByMetadata(ctx context.Context, key, value string) ([]*ui.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var results []*ui.Session
	for _, session := range m.sessions {
		if session.Metadata != nil {
			if v, ok := session.Metadata[key]; ok && v == value {
				results = append(results, session)
			}
		}
	}
	
	return results, nil
}

// MockStreamHandler is a configurable mock implementation of StreamHandler
type MockStreamHandler struct {
	StreamFunc func(ctx context.Context, sessionID string, message string) (<-chan ui.ChatEvent, error)
	
	// Default behavior configuration
	EventCount int
	EventDelay time.Duration
	ErrorAfter int
}

// NewMockStreamHandler creates a new mock stream handler
func NewMockStreamHandler() *MockStreamHandler {
	return &MockStreamHandler{
		EventCount: 5,
		EventDelay: 100 * time.Millisecond,
		ErrorAfter: -1, // No error by default
	}
}

// StreamResponse implements StreamHandler.StreamResponse
func (m *MockStreamHandler) StreamResponse(ctx context.Context, sessionID string, message string) (<-chan ui.ChatEvent, error) {
	if m.StreamFunc != nil {
		return m.StreamFunc(ctx, sessionID, message)
	}
	
	// Default streaming behavior
	events := make(chan ui.ChatEvent)
	
	go func() {
		defer close(events)
		
		for i := 0; i < m.EventCount; i++ {
			// Check context cancellation
			select {
			case <-ctx.Done():
				return
			default:
			}
			
			// Simulate error
			if m.ErrorAfter >= 0 && i == m.ErrorAfter {
				events <- ui.ChatEvent{
					Type:      ui.EventError,
					Data:      "simulated error",
					Timestamp: time.Now(),
				}
				return
			}
			
			// Send normal event
			events <- ui.ChatEvent{
				Type:      ui.EventMessage,
				Data:      fmt.Sprintf("Chunk %d of response to: %s", i+1, message),
				Timestamp: time.Now(),
			}
			
			// Simulate processing delay
			if m.EventDelay > 0 {
				time.Sleep(m.EventDelay)
			}
		}
		
		// Send completion event
		events <- ui.ChatEvent{
			Type:      ui.EventDone,
			Data:      "completed",
			Timestamp: time.Now(),
		}
	}()
	
	return events, nil
}

// MockChatAgent is a minimal mock implementation of ChatAgent for testing
type MockChatAgent struct {
	*core.BaseAgent
	sessionManager ui.SessionManager
	transports     map[string]ui.Transport
	mu             sync.RWMutex
}

// NewMockChatAgent creates a new mock chat agent
func NewMockChatAgent(name string) *MockChatAgent {
	return &MockChatAgent{
		BaseAgent:      core.NewBaseAgent(name),
		sessionManager: NewMockSessionManager(),
		transports:     make(map[string]ui.Transport),
	}
}

// RegisterTransport implements ChatAgent.RegisterTransport
func (m *MockChatAgent) RegisterTransport(transport ui.Transport) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	name := transport.Name()
	if _, exists := m.transports[name]; exists {
		return ui.ErrTransportAlreadyExists
	}
	
	m.transports[name] = transport
	return nil
}

// ListTransports implements ChatAgent.ListTransports
func (m *MockChatAgent) ListTransports() []ui.TransportInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var infos []ui.TransportInfo
	for name, transport := range m.transports {
		infos = append(infos, ui.TransportInfo{
			Name:         name,
			Description:  transport.Description(),
			Priority:     transport.Priority(),
			Capabilities: transport.Capabilities(),
			Healthy:      transport.HealthCheck(context.Background()) == nil,
		})
	}
	
	return infos
}

// GetTransport implements ChatAgent.GetTransport
func (m *MockChatAgent) GetTransport(name string) (ui.Transport, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	transport, exists := m.transports[name]
	return transport, exists
}

// GetSessionManager implements ChatAgent.GetSessionManager
func (m *MockChatAgent) GetSessionManager() ui.SessionManager {
	return m.sessionManager
}

// ProcessMessage implements ChatAgent.ProcessMessage
func (m *MockChatAgent) ProcessMessage(ctx context.Context, sessionID string, message string) (<-chan ui.ChatEvent, error) {
	// Simple echo implementation
	events := make(chan ui.ChatEvent, 1)
	events <- ui.ChatEvent{
		Type:      ui.EventMessage,
		Data:      fmt.Sprintf("Echo: %s", message),
		Timestamp: time.Now(),
	}
	close(events)
	return events, nil
}

// CreateSession implements ChatAgent.CreateSession
func (m *MockChatAgent) CreateSession(ctx context.Context) (*ui.Session, error) {
	return m.sessionManager.Create(ctx, nil)
}

// GetSession implements ChatAgent.GetSession
func (m *MockChatAgent) GetSession(ctx context.Context, sessionID string) (*ui.Session, error) {
	return m.sessionManager.Get(ctx, sessionID)
}

// CheckRateLimit implements ChatAgent.CheckRateLimit
func (m *MockChatAgent) CheckRateLimit(ctx context.Context, sessionID string) (bool, error) {
	allowed, _, err := m.sessionManager.CheckRateLimit(ctx, sessionID)
	return allowed, err
}

// StreamResponse implements ChatAgent.StreamResponse
func (m *MockChatAgent) StreamResponse(ctx context.Context, sessionID string, message string) (<-chan ui.ChatEvent, error) {
	// Simple echo implementation
	events := make(chan ui.ChatEvent, 2)
	events <- ui.ChatEvent{
		Type:      ui.EventMessage,
		Data:      fmt.Sprintf("Echo: %s", message),
		Timestamp: time.Now(),
	}
	events <- ui.ChatEvent{
		Type:      ui.EventDone,
		Data:      "completed",
		Timestamp: time.Now(),
	}
	close(events)
	return events, nil
}

// Configure implements ChatAgent.Configure
func (m *MockChatAgent) Configure(config ui.ChatAgentConfig) error {
	// Simple validation
	if config.Name == "" {
		return ui.ErrInvalidConfig
	}
	return nil
}