package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisSessionManager implements SessionManager using Redis for distributed session management
type RedisSessionManager struct {
	client *redis.Client
	config SessionConfig
	
	// Graceful shutdown support
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewRedisSessionManager creates a new Redis-backed session manager
func NewRedisSessionManager(redisURL string, config SessionConfig) (*RedisSessionManager, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	manager := &RedisSessionManager{
		client:   client,
		config:   config,
		stopChan: make(chan struct{}),
	}

	// Start cleanup goroutine
	manager.startCleanupRoutine()

	return manager, nil
}

// Create creates a new session
func (r *RedisSessionManager) Create(ctx context.Context, metadata map[string]interface{}) (*Session, error) {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	
	session := &Session{
		ID:          uuid.New().String(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(r.config.TTL),
		TokenCount:  0,
		MessageCount: 0,
		Metadata:    metadata,
	}

	pipe := r.client.Pipeline()

	// Store session data as hash
	sessionKey := r.sessionKey(session.ID)
	hashData := map[string]interface{}{
		"id":            session.ID,
		"created_at":    session.CreatedAt.Unix(),
		"updated_at":    session.UpdatedAt.Unix(),
		"expires_at":    session.ExpiresAt.Unix(),
		"token_count":   session.TokenCount,
		"message_count": session.MessageCount,
	}
	
	// Store metadata as JSON
	if len(session.Metadata) > 0 {
		if metadataJSON, err := json.Marshal(session.Metadata); err == nil {
			hashData["metadata"] = string(metadataJSON)
		}
	}
	
	pipe.HSet(ctx, sessionKey, hashData)

	// Set TTL
	pipe.Expire(ctx, sessionKey, r.config.TTL)

	// Add to active sessions set
	pipe.SAdd(ctx, r.activeSessionsKey(), session.ID)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// Get retrieves a session by ID
func (r *RedisSessionManager) Get(ctx context.Context, sessionID string) (*Session, error) {
	sessionKey := r.sessionKey(sessionID)

	// Get all fields
	result, err := r.client.HGetAll(ctx, sessionKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	session := &Session{
		ID:       sessionID,
		Metadata: make(map[string]interface{}),
	}

	// Parse fields
	if v, ok := result["created_at"]; ok {
		if ts, err := parseUnixTime(v); err == nil {
			session.CreatedAt = ts
		}
	}
	if v, ok := result["updated_at"]; ok {
		if ts, err := parseUnixTime(v); err == nil {
			session.UpdatedAt = ts
		}
	}
	if v, ok := result["expires_at"]; ok {
		if ts, err := parseUnixTime(v); err == nil {
			session.ExpiresAt = ts
		}
	}
	if v, ok := result["token_count"]; ok {
		fmt.Sscanf(v, "%d", &session.TokenCount)
	}
	if v, ok := result["message_count"]; ok {
		fmt.Sscanf(v, "%d", &session.MessageCount)
	}

	return session, nil
}

// Update updates a session
func (r *RedisSessionManager) Update(ctx context.Context, session *Session) error {
	session.UpdatedAt = time.Now()

	sessionKey := r.sessionKey(session.ID)

	pipe := r.client.Pipeline()

	// Update session data
	updateData := map[string]interface{}{
		"updated_at":    session.UpdatedAt.Unix(),
		"token_count":   session.TokenCount,
		"message_count": session.MessageCount,
	}
	
	// Update metadata if present
	if len(session.Metadata) > 0 {
		if metadataJSON, err := json.Marshal(session.Metadata); err == nil {
			updateData["metadata"] = string(metadataJSON)
		}
	}
	
	pipe.HSet(ctx, sessionKey, updateData)

	// Refresh TTL
	pipe.Expire(ctx, sessionKey, r.config.TTL)
	pipe.Expire(ctx, r.messagesKey(session.ID), r.config.TTL)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// Delete deletes a session
func (r *RedisSessionManager) Delete(ctx context.Context, sessionID string) error {
	pipe := r.client.Pipeline()

	// Delete all session-related keys
	pipe.Del(ctx, r.sessionKey(sessionID))
	pipe.Del(ctx, r.messagesKey(sessionID))
	pipe.Del(ctx, r.rateLimitKey(sessionID))

	// Remove from active sessions
	pipe.SRem(ctx, r.activeSessionsKey(), sessionID)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// AddMessage adds a message to a session with sliding window
func (r *RedisSessionManager) AddMessage(ctx context.Context, sessionID string, msg Message) error {
	msg.ID = uuid.New().String()
	msg.SessionID = sessionID
	msg.Timestamp = time.Now()

	// Serialize message
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	messagesKey := r.messagesKey(sessionID)
	sessionKey := r.sessionKey(sessionID)

	pipe := r.client.Pipeline()

	// Add message to list
	pipe.RPush(ctx, messagesKey, data)

	// Maintain sliding window
	pipe.LTrim(ctx, messagesKey, -int64(r.config.MaxMessages), -1)

	// Update session counters
	pipe.HIncrBy(ctx, sessionKey, "message_count", 1)
	pipe.HIncrBy(ctx, sessionKey, "token_count", int64(msg.TokenCount))
	pipe.HSet(ctx, sessionKey, "updated_at", time.Now().Unix())

	// Refresh TTL
	pipe.Expire(ctx, messagesKey, r.config.TTL)
	pipe.Expire(ctx, sessionKey, r.config.TTL)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to add message: %w", err)
	}

	return nil
}

// GetMessages retrieves messages for a session
func (r *RedisSessionManager) GetMessages(ctx context.Context, sessionID string, limit int) ([]Message, error) {
	messagesKey := r.messagesKey(sessionID)

	// Get messages (negative indices for last N messages)
	start := -int64(limit)
	if limit <= 0 {
		start = 0
		limit = r.config.MaxMessages
	}

	results, err := r.client.LRange(ctx, messagesKey, start, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	messages := make([]Message, 0, len(results))
	for _, data := range results {
		var msg Message
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			continue // Skip invalid messages
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// CheckRateLimit checks if a session has exceeded rate limit
func (r *RedisSessionManager) CheckRateLimit(ctx context.Context, sessionID string) (bool, time.Time, error) {
	key := r.rateLimitKey(sessionID)

	pipe := r.client.Pipeline()

	// Increment counter
	incr := pipe.Incr(ctx, key)

	// Set expiry on first request
	pipe.Expire(ctx, key, r.config.RateLimitWindow)

	// Get TTL for reset time
	ttl := pipe.TTL(ctx, key)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to check rate limit: %w", err)
	}

	count, _ := incr.Result()
	ttlDuration, _ := ttl.Result()
	
	// Calculate reset time
	resetAt := time.Now().Add(ttlDuration)
	if ttlDuration <= 0 {
		resetAt = time.Now().Add(r.config.RateLimitWindow)
	}

	return count <= int64(r.config.RateLimitMax), resetAt, nil
}

// GetRateLimit gets current rate limit count for a session
func (r *RedisSessionManager) GetRateLimit(ctx context.Context, sessionID string) (int, error) {
	key := r.rateLimitKey(sessionID)

	count, err := r.client.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get rate limit: %w", err)
	}

	return count, nil
}

// ListActiveSessions returns all active session IDs
func (r *RedisSessionManager) ListActiveSessions(ctx context.Context) ([]string, error) {
	sessions, err := r.client.SMembers(ctx, r.activeSessionsKey()).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list active sessions: %w", err)
	}

	return sessions, nil
}

// CleanupExpiredSessions removes expired sessions
func (r *RedisSessionManager) CleanupExpiredSessions(ctx context.Context) error {
	sessions, err := r.ListActiveSessions(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	for _, sessionID := range sessions {
		session, err := r.Get(ctx, sessionID)
		if err != nil {
			// Session might already be deleted
			r.client.SRem(ctx, r.activeSessionsKey(), sessionID)
			continue
		}

		if now.After(session.ExpiresAt) {
			if err := r.Delete(ctx, sessionID); err != nil {
				// Log error but continue cleanup
				continue
			}
		}
	}

	return nil
}

// GetActiveSessionCount returns the number of active sessions
func (r *RedisSessionManager) GetActiveSessionCount(ctx context.Context) (int64, error) {
	pattern := r.sessionKey("*")
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to count sessions: %w", err)
	}
	return int64(len(keys)), nil
}

// GetSessionsByMetadata retrieves sessions by metadata key-value pair
func (r *RedisSessionManager) GetSessionsByMetadata(ctx context.Context, key, value string) ([]*Session, error) {
	// Get all session keys
	pattern := r.sessionKey("*")
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	var sessions []*Session
	for _, sessionKey := range keys {
		// Get session data
		data, err := r.client.HGetAll(ctx, sessionKey).Result()
		if err != nil {
			continue
		}

		// Parse session
		session, err := r.parseSession(data)
		if err != nil {
			continue
		}

		// Check if metadata matches
		if val, exists := session.Metadata[key]; exists && val == value {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// startCleanupRoutine starts a background goroutine to clean expired sessions
func (r *RedisSessionManager) startCleanupRoutine() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		
		ticker := time.NewTicker(r.config.CleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				r.CleanupExpiredSessions(ctx)
				cancel()
			case <-r.stopChan:
				return
			}
		}
	}()
}

// Key helpers
func (r *RedisSessionManager) sessionKey(sessionID string) string {
	return fmt.Sprintf("gomind:chat:session:%s", sessionID)
}

func (r *RedisSessionManager) messagesKey(sessionID string) string {
	return fmt.Sprintf("gomind:chat:session:%s:msgs", sessionID)
}

func (r *RedisSessionManager) rateLimitKey(sessionID string) string {
	return fmt.Sprintf("gomind:chat:session:%s:rate", sessionID)
}

func (r *RedisSessionManager) activeSessionsKey() string {
	return "gomind:chat:sessions:active"
}

// parseSession parses a session from Redis hash data
func (r *RedisSessionManager) parseSession(data map[string]string) (*Session, error) {
	sessionID := ""
	if id, ok := data["id"]; ok {
		sessionID = id
	}
	
	session := &Session{
		ID:       sessionID,
		Metadata: make(map[string]interface{}),
	}

	// Parse fields
	if v, ok := data["created_at"]; ok {
		if ts, err := parseUnixTime(v); err == nil {
			session.CreatedAt = ts
		}
	}
	if v, ok := data["updated_at"]; ok {
		if ts, err := parseUnixTime(v); err == nil {
			session.UpdatedAt = ts
		}
	}
	if v, ok := data["expires_at"]; ok {
		if ts, err := parseUnixTime(v); err == nil {
			session.ExpiresAt = ts
		}
	}
	if v, ok := data["token_count"]; ok {
		fmt.Sscanf(v, "%d", &session.TokenCount)
	}
	if v, ok := data["message_count"]; ok {
		fmt.Sscanf(v, "%d", &session.MessageCount)
	}
	if v, ok := data["metadata"]; ok && v != "" {
		json.Unmarshal([]byte(v), &session.Metadata)
	}

	return session, nil
}

// parseUnixTime parses a Unix timestamp string
func parseUnixTime(s string) (time.Time, error) {
	var ts int64
	_, err := fmt.Sscanf(s, "%d", &ts)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(ts, 0), nil
}

// Close gracefully shuts down the session manager
func (r *RedisSessionManager) Close() error {
	// Signal cleanup routine to stop
	close(r.stopChan)
	
	// Wait for cleanup routine to finish
	r.wg.Wait()
	
	// Close Redis connection
	return r.client.Close()
}