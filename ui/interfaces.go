// Package ui provides a framework for building chat-based user interfaces
// with pluggable transport protocols and distributed session management.
package ui

import (
	"context"
	"net/http"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// Transport defines the contract for all UI communication protocols.
//
// Contract:
// - Initialize must be called before Start
// - Stop must cleanly shutdown all connections within the context deadline
// - HealthCheck must not modify state
// - CreateHandler must be safe to call concurrently
//
// Invariants:
// - A stopped transport can be restarted
// - Priority is immutable after initialization
// - Name must be unique within the registry
//
// Example: See MockTransport in testing package
//
// Testing: Must pass TransportComplianceTest suite
type Transport interface {
	// Lifecycle management
	Initialize(config TransportConfig) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	
	// Core functionality
	CreateHandler(agent ChatAgent) http.Handler
	
	// Metadata
	Name() string
	Description() string
	Priority() int // Higher priority = preferred when multiple transports available
	Capabilities() []TransportCapability
	
	// Health monitoring
	HealthCheck(ctx context.Context) error
	
	// Availability check - can this transport be used in current environment?
	Available() bool
	
	// ClientExample returns example client code for this transport
	ClientExample() string
}

// TransportCapability describes what a transport can do
type TransportCapability string

const (
	// CapabilityStreaming indicates the transport supports streaming responses
	CapabilityStreaming TransportCapability = "streaming"
	
	// CapabilityBidirectional indicates the transport supports bidirectional communication
	CapabilityBidirectional TransportCapability = "bidirectional"
	
	// CapabilityReconnect indicates the transport supports automatic reconnection
	CapabilityReconnect TransportCapability = "reconnect"
	
	// CapabilityMultiplex indicates the transport supports multiple concurrent streams
	CapabilityMultiplex TransportCapability = "multiplex"
)

// TransportConfig configures a transport
type TransportConfig struct {
	// Common configuration
	MaxConnections int           `json:"max_connections"`
	Timeout        time.Duration `json:"timeout"`
	
	// Security
	CORS           CORSConfig     `json:"cors"`
	RateLimit      RateLimitConfig `json:"rate_limit"`
	
	// Transport-specific options
	Options map[string]interface{} `json:"options"`
}

// CORSConfig defines CORS settings
type CORSConfig struct {
	Enabled        bool     `json:"enabled"`
	AllowedOrigins []string `json:"allowed_origins"`
	AllowedMethods []string `json:"allowed_methods"`
	AllowedHeaders []string `json:"allowed_headers"`
	MaxAge         int      `json:"max_age"`
}

// RateLimitConfig defines rate limiting settings
type RateLimitConfig struct {
	Enabled      bool          `json:"enabled"`
	RequestsPerMinute int      `json:"requests_per_minute"`
	BurstSize    int          `json:"burst_size"`
}

// SessionManager manages chat sessions with distributed system support.
//
// Contract:
// - Sessions must be accessible across multiple instances
// - Expired sessions must be automatically cleaned up
// - Concurrent access to same session must be safe
//
// Invariants:
// - Session IDs are globally unique
// - Messages are ordered within a session
// - Token count monotonically increases
//
// Testing: Must pass SessionComplianceTest suite
type SessionManager interface {
	// Session lifecycle
	Create(ctx context.Context, metadata map[string]interface{}) (*Session, error)
	Get(ctx context.Context, sessionID string) (*Session, error)
	Update(ctx context.Context, session *Session) error
	Delete(ctx context.Context, sessionID string) error
	
	// Message management
	AddMessage(ctx context.Context, sessionID string, msg Message) error
	GetMessages(ctx context.Context, sessionID string, limit int) ([]Message, error)
	
	// Rate limiting
	CheckRateLimit(ctx context.Context, sessionID string) (allowed bool, resetAt time.Time, err error)
	
	// Analytics
	GetActiveSessionCount(ctx context.Context) (int64, error)
	GetSessionsByMetadata(ctx context.Context, key, value string) ([]*Session, error)
}

// Session represents a chat session
type Session struct {
	ID           string                 `json:"id"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	ExpiresAt    time.Time              `json:"expires_at"`
	TokenCount   int                    `json:"token_count"`
	MessageCount int                    `json:"message_count"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// Message represents a chat message
type Message struct {
	ID         string                 `json:"id"`
	SessionID  string                 `json:"session_id"`
	Role       string                 `json:"role"` // "user", "assistant", "system"
	Content    string                 `json:"content"`
	TokenCount int                    `json:"token_count"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ChatAgent orchestrates transports and sessions for chat functionality.
//
// Contract:
// - Must support multiple concurrent transports
// - Must handle transport failures gracefully
// - Must maintain session consistency across transports
//
// Invariants:
// - Active transports are healthy
// - Sessions are transport-agnostic
//
// Testing: Must pass AgentComplianceTest suite
type ChatAgent interface {
	core.Agent // Extends core.Agent with discovery capabilities
	
	// Transport management
	RegisterTransport(transport Transport) error
	ListTransports() []TransportInfo
	GetTransport(name string) (Transport, bool)
	
	// Session management
	GetSessionManager() SessionManager
	CreateSession(ctx context.Context) (*Session, error)
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	CheckRateLimit(ctx context.Context, sessionID string) (bool, error)
	
	// Message handling
	ProcessMessage(ctx context.Context, sessionID string, message string) (<-chan ChatEvent, error)
	StreamResponse(ctx context.Context, sessionID string, message string) (<-chan ChatEvent, error)
	
	// Configuration
	Configure(config ChatAgentConfig) error
}

// TransportInfo provides information about a registered transport
type TransportInfo struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Endpoint     string                 `json:"endpoint"`
	Priority     int                    `json:"priority"`
	Capabilities []TransportCapability  `json:"capabilities"`
	Healthy      bool                   `json:"healthy"`
	Example      string                 `json:"example,omitempty"`
}

// ChatEvent represents an event in the chat stream
type ChatEvent struct {
	Type      ChatEventType          `json:"type"`
	Data      string                 `json:"data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// ChatEventType defines types of chat events
type ChatEventType string

const (
	// EventMessage is a regular message chunk
	EventMessage ChatEventType = "message"
	
	// EventError indicates an error occurred
	EventError ChatEventType = "error"
	
	// EventDone indicates streaming is complete
	EventDone ChatEventType = "done"
	
	// EventTyping indicates the assistant is typing
	EventTyping ChatEventType = "typing"
	
	// EventThinking indicates the assistant is processing
	EventThinking ChatEventType = "thinking"
)

// StreamHandler handles streaming responses.
//
// Contract:
// - Channel must be closed when streaming completes
// - Errors must be sent as EventError events
// - Context cancellation must stop streaming
//
// Testing: Must pass StreamComplianceTest suite
type StreamHandler interface {
	StreamResponse(ctx context.Context, sessionID string, message string) (<-chan ChatEvent, error)
}

// SecurityConfig contains security settings
type SecurityConfig struct {
	RateLimit        int      `json:"rate_limit"`        // Messages per minute
	MaxMessageSize   int      `json:"max_message_size"`  // Bytes
	AllowedOrigins   []string `json:"allowed_origins"`   // CORS origins
	RequireAuth      bool     `json:"require_auth"`      // JWT/OAuth required
}

// ChatAgentConfig configures a ChatAgent
type ChatAgentConfig struct {
	// Core settings
	Name        string `json:"name"`
	Description string `json:"description"`
	
	// Session configuration
	SessionConfig SessionConfig `json:"session_config"`
	
	// Security configuration
	SecurityConfig SecurityConfig `json:"security_config"`
	
	// Transport settings - map of transport name to config
	TransportConfigs map[string]TransportConfig `json:"transport_configs"`
	
	// Circuit breaker configuration
	CircuitBreakerEnabled bool                  `json:"circuit_breaker_enabled"`
	CircuitBreakerConfig  CircuitBreakerConfig `json:"circuit_breaker_config"`
	
	// Redis connection (reused from discovery)
	RedisURL string `json:"redis_url"`
}

// SessionConfig configures session management
type SessionConfig struct {
	TTL             time.Duration `json:"ttl"`               // Session expiration
	MaxMessages     int           `json:"max_messages"`      // Sliding window size
	MaxTokens       int           `json:"max_tokens"`        // Cost control
	RateLimitWindow time.Duration `json:"rate_limit_window"` // Rate limit time window
	RateLimitMax    int           `json:"rate_limit_max"`    // Max requests per window
	CleanupInterval time.Duration `json:"cleanup_interval"`  // How often to clean expired sessions
}

// TransportRegistry manages transport registration and discovery.
//
// Contract:
// - Transports must be registered before use
// - Names must be unique
// - Registry is thread-safe
//
// Testing: Must pass RegistryComplianceTest suite
type TransportRegistry interface {
	Register(transport Transport) error
	Unregister(name string) error
	Get(name string) (Transport, bool)
	List() []Transport
	ListAvailable() []Transport
}

// TransportLifecycleEvent represents transport state changes
type TransportLifecycleEvent struct {
	Transport string                 `json:"transport"`
	Event     TransportEventType     `json:"event"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// TransportEventType defines transport lifecycle events
type TransportEventType string

const (
	EventTransportInitialized TransportEventType = "initialized"
	EventTransportStarted     TransportEventType = "started"
	EventTransportStopped     TransportEventType = "stopped"
	EventTransportHealthy     TransportEventType = "healthy"
	EventTransportUnhealthy   TransportEventType = "unhealthy"
)

// TransportEventHandler handles transport lifecycle events
type TransportEventHandler func(event TransportLifecycleEvent)

// ChatAgentFactory creates ChatAgent instances
type ChatAgentFactory interface {
	Create(config ChatAgentConfig) (ChatAgent, error)
}