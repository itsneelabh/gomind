package ui

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestSessionManagerCompliance tests that both Redis and Mock implementations
// comply with the SessionManager interface
func TestSessionManagerCompliance(t *testing.T) {
	tests := []struct {
		name    string
		manager SessionManager
	}{
		{
			name: "MockSessionManager",
			manager: NewMockSessionManager(SessionConfig{
				TTL:             30 * time.Second,
				MaxMessages:     10,
				MaxTokens:       1000,
				RateLimitWindow: time.Minute,
				RateLimitMax:    5,
			}),
		},
		// Redis test would require actual Redis connection
		// Skipped to avoid external dependencies in tests
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testSessionManagerOperations(t, tt.manager)
		})
	}
}

func testSessionManagerOperations(t *testing.T, manager SessionManager) {
	ctx := context.Background()

	// Test Create
	t.Run("Create", func(t *testing.T) {
		session, err := manager.Create(ctx, nil)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		if session.ID == "" {
			t.Error("Session ID should not be empty")
		}
		if session.CreatedAt.IsZero() {
			t.Error("CreatedAt should be set")
		}
		if session.TokenCount != 0 {
			t.Error("Initial token count should be 0")
		}
	})

	// Test Get
	t.Run("Get", func(t *testing.T) {
		session1, _ := manager.Create(ctx, nil)

		session2, err := manager.Get(ctx, session1.ID)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if session2.ID != session1.ID {
			t.Error("Session IDs should match")
		}
	})

	// Test Get non-existent
	t.Run("GetNonExistent", func(t *testing.T) {
		_, err := manager.Get(ctx, "non-existent")
		if err == nil {
			t.Error("Get should fail for non-existent session")
		}
	})

	// Test Update
	t.Run("Update", func(t *testing.T) {
		session, _ := manager.Create(ctx, nil)
		originalUpdate := session.UpdatedAt

		time.Sleep(10 * time.Millisecond) // Ensure time difference

		session.TokenCount = 100
		err := manager.Update(ctx, session)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		updated, _ := manager.Get(ctx, session.ID)
		if updated.TokenCount != 100 {
			t.Error("Token count should be updated")
		}
		if !updated.UpdatedAt.After(originalUpdate) {
			t.Error("UpdatedAt should be updated")
		}
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		session, _ := manager.Create(ctx, nil)

		err := manager.Delete(ctx, session.ID)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, err = manager.Get(ctx, session.ID)
		if err == nil {
			t.Error("Get should fail after delete")
		}
	})

	// Test AddMessage
	t.Run("AddMessage", func(t *testing.T) {
		session, _ := manager.Create(ctx, nil)

		msg := Message{
			Role:       "user",
			Content:    "Hello",
			TokenCount: 2,
		}

		err := manager.AddMessage(ctx, session.ID, msg)
		if err != nil {
			t.Fatalf("AddMessage failed: %v", err)
		}

		messages, err := manager.GetMessages(ctx, session.ID, 10)
		if err != nil {
			t.Fatalf("GetMessages failed: %v", err)
		}

		if len(messages) != 1 {
			t.Errorf("Expected 1 message, got %d", len(messages))
		}
		if messages[0].Content != "Hello" {
			t.Error("Message content mismatch")
		}
	})

	// Test message sliding window
	t.Run("MessageSlidingWindow", func(t *testing.T) {
		config := SessionConfig{
			TTL:             30 * time.Second,
			MaxMessages:     3,
			MaxTokens:       1000,
			RateLimitWindow: time.Minute,
			RateLimitMax:    5,
		}
		manager := NewMockSessionManager(config)
		session, _ := manager.Create(ctx, nil)

		// Add more messages than max
		for i := 0; i < 5; i++ {
			msg := Message{
				Role:    "user",
				Content: fmt.Sprintf("Message %d", i),
			}
			manager.AddMessage(ctx, session.ID, msg)
		}

		messages, _ := manager.GetMessages(ctx, session.ID, 10)
		if len(messages) != 3 {
			t.Errorf("Expected 3 messages (max), got %d", len(messages))
		}

		// Should have the last 3 messages
		if messages[0].Content != "Message 2" {
			t.Error("Sliding window should keep most recent messages")
		}
	})

	// Test rate limiting
	t.Run("RateLimiting", func(t *testing.T) {
		config := SessionConfig{
			TTL:             30 * time.Second,
			MaxMessages:     10,
			MaxTokens:       1000,
			RateLimitWindow: 100 * time.Millisecond,
			RateLimitMax:    3,
		}
		manager := NewMockSessionManager(config)
		session, _ := manager.Create(ctx, nil)

		// Should allow first 3 requests
		for i := 0; i < 3; i++ {
			allowed, _, err := manager.CheckRateLimit(ctx, session.ID)
			if err != nil {
				t.Fatalf("CheckRateLimit failed: %v", err)
			}
			if !allowed {
				t.Errorf("Request %d should be allowed", i+1)
			}
		}

		// 4th request should be denied
		allowed, _, _ := manager.CheckRateLimit(ctx, session.ID)
		if allowed {
			t.Error("4th request should be denied (rate limit)")
		}

		// Wait for window to reset
		time.Sleep(150 * time.Millisecond)

		// Should allow again
		allowed, _, _ = manager.CheckRateLimit(ctx, session.ID)
		if !allowed {
			t.Error("Request should be allowed after window reset")
		}
	})

	// Test ListActiveSessions
	t.Run("ListActiveSessions", func(t *testing.T) {
		// Clean start
		manager := NewMockSessionManager(DefaultSessionConfig())

		// Create multiple sessions
		var sessionIDs []string
		for i := 0; i < 3; i++ {
			session, _ := manager.Create(ctx, nil)
			sessionIDs = append(sessionIDs, session.ID)
		}

		active, err := manager.ListActiveSessions(ctx)
		if err != nil {
			t.Fatalf("ListActiveSessions failed: %v", err)
		}

		if len(active) != 3 {
			t.Errorf("Expected 3 active sessions, got %d", len(active))
		}

		// Check all created sessions are in active list
		activeMap := make(map[string]bool)
		for _, id := range active {
			activeMap[id] = true
		}
		for _, id := range sessionIDs {
			if !activeMap[id] {
				t.Errorf("Session %s not in active list", id)
			}
		}
	})

	// Test CleanupExpiredSessions
	t.Run("CleanupExpiredSessions", func(t *testing.T) {
		config := SessionConfig{
			TTL:             100 * time.Millisecond,
			MaxMessages:     10,
			MaxTokens:       1000,
			RateLimitWindow: time.Minute,
			RateLimitMax:    5,
		}
		manager := NewMockSessionManager(config)

		// Create session
		session, _ := manager.Create(ctx, nil)

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)

		// Run cleanup
		err := manager.CleanupExpiredSessions(ctx)
		if err != nil {
			t.Fatalf("CleanupExpiredSessions failed: %v", err)
		}

		// Session should be gone
		_, err = manager.Get(ctx, session.ID)
		if err == nil {
			t.Error("Expired session should be cleaned up")
		}
	})
}

// TestMockSessionManagerFailures tests failure simulation
func TestMockSessionManagerFailures(t *testing.T) {
	ctx := context.Background()
	manager := NewMockSessionManager(DefaultSessionConfig())

	// Test specific method failure
	manager.SetFailure(true, "Create")

	_, err := manager.Create(ctx, nil)
	if err == nil {
		t.Error("Create should fail when configured")
	}

	// Reset and test another method
	manager.SetFailure(false, "")
	session, _ := manager.Create(ctx, nil)

	manager.SetFailure(true, "Get")
	_, err = manager.Get(ctx, session.ID)
	if err == nil {
		t.Error("Get should fail when configured")
	}
}