package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/itsneelabh/gomind/core"
)

// Session represents a chat session with conversation history.
type Session struct {
	ID        string                 `json:"id"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Messages  []Message              `json:"messages"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Message represents a chat message in a session.
type Message struct {
	ID        string                 `json:"id"`
	Role      string                 `json:"role"` // "user" or "assistant"
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SessionStore provides Redis-based session management.
// Uses Redis DB 2 (RedisDBSessions) to isolate from service registry (DB 0).
type SessionStore struct {
	client      *core.RedisClient
	ttl         time.Duration
	maxMessages int
	logger      core.Logger
}

// NewSessionStore creates a new Redis-backed session store.
// It uses Redis DB 2 (RedisDBSessions) with namespace "gomind:sessions"
// to keep session data separate from the service registry (DB 0).
func NewSessionStore(redisURL string, ttl time.Duration, maxMessages int, logger core.Logger) (*SessionStore, error) {
	client, err := core.NewRedisClient(core.RedisClientOptions{
		RedisURL:  redisURL,
		DB:        core.RedisDBSessions, // DB 2 - separate from registry (DB 0)
		Namespace: "gomind:sessions",
		Logger:    logger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis client for sessions: %w", err)
	}

	store := &SessionStore{
		client:      client,
		ttl:         ttl,
		maxMessages: maxMessages,
		logger:      logger,
	}

	if logger != nil {
		logger.Info("Session store initialized", map[string]interface{}{
			"redis_db":     core.RedisDBSessions,
			"namespace":    "gomind:sessions",
			"ttl":          ttl.String(),
			"max_messages": maxMessages,
		})
	}

	return store, nil
}

// Create creates a new session.
func (s *SessionStore) Create(metadata map[string]interface{}) *Session {
	session := &Session{
		ID:        uuid.New().String(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  make([]Message, 0),
		Metadata:  metadata,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.saveSession(ctx, session); err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to save new session", map[string]interface{}{
				"session_id": session.ID,
				"error":      err.Error(),
			})
		}
		// Return session anyway - it exists in memory even if Redis save failed
	}

	return session
}

// Get retrieves a session by ID.
func (s *SessionStore) Get(sessionID string) *Session {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := s.loadSession(ctx, sessionID)
	if err != nil {
		if s.logger != nil {
			s.logger.Debug("Session not found or expired", map[string]interface{}{
				"session_id": sessionID,
				"error":      err.Error(),
			})
		}
		return nil
	}

	return session
}

// Delete removes a session.
func (s *SessionStore) Delete(sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.client.Del(ctx, sessionID); err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to delete session", map[string]interface{}{
				"session_id": sessionID,
				"error":      err.Error(),
			})
		}
	}
}

// AddMessage adds a message to a session.
func (s *SessionStore) AddMessage(sessionID string, msg Message) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Load existing session
	session, err := s.loadSession(ctx, sessionID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to load session for adding message", map[string]interface{}{
				"session_id": sessionID,
				"error":      err.Error(),
			})
		}
		return false
	}

	// Set message ID if not provided
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}

	// Add message
	session.Messages = append(session.Messages, msg)

	// Trim to max messages (sliding window)
	if len(session.Messages) > s.maxMessages {
		session.Messages = session.Messages[len(session.Messages)-s.maxMessages:]
	}

	session.UpdatedAt = time.Now()

	// Save back to Redis
	if err := s.saveSession(ctx, session); err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to save session after adding message", map[string]interface{}{
				"session_id": sessionID,
				"error":      err.Error(),
			})
		}
		return false
	}

	return true
}

// GetMessages retrieves messages from a session.
func (s *SessionStore) GetMessages(sessionID string, limit int) []Message {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := s.loadSession(ctx, sessionID)
	if err != nil {
		return nil
	}

	messages := session.Messages
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}

	// Return a copy to avoid race conditions
	result := make([]Message, len(messages))
	copy(result, messages)
	return result
}

// GetHistory retrieves the full conversation history for a session.
func (s *SessionStore) GetHistory(sessionID string) []Message {
	return s.GetMessages(sessionID, 0)
}

// GetActiveSessionCount returns an approximate count of active sessions.
// Note: This scans keys in Redis which can be expensive with many sessions.
func (s *SessionStore) GetActiveSessionCount() int {
	// For Redis, we would need to scan keys which is expensive
	// Return 0 for now - this is mainly used for health checks
	// A production implementation might use a separate counter
	return 0
}

// Close closes the Redis connection.
func (s *SessionStore) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// saveSession saves a session to Redis.
func (s *SessionStore) saveSession(ctx context.Context, session *Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Key is just the session ID - namespace is handled by RedisClient
	if err := s.client.Set(ctx, session.ID, string(data), s.ttl); err != nil {
		return fmt.Errorf("failed to save session to Redis: %w", err)
	}

	return nil
}

// loadSession loads a session from Redis.
func (s *SessionStore) loadSession(ctx context.Context, sessionID string) (*Session, error) {
	data, err := s.client.Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session from Redis: %w", err)
	}

	var session Session
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}
