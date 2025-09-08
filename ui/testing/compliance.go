package testing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/ui"
)

// TransportComplianceTest runs a comprehensive test suite that all Transport implementations must pass
func TransportComplianceTest(t *testing.T, transport ui.Transport) {
	t.Run("Metadata", testTransportMetadata(transport))
	t.Run("Lifecycle", testTransportLifecycle(transport))
	t.Run("ErrorHandling", testTransportErrorHandling(transport))
	t.Run("Concurrency", testTransportConcurrency(transport))
	t.Run("HealthCheck", testTransportHealthCheck(transport))
}

func testTransportMetadata(transport ui.Transport) func(*testing.T) {
	return func(t *testing.T) {
		// Name must not be empty
		if transport.Name() == "" {
			t.Error("Transport.Name() returned empty string")
		}
		
		// Description must not be empty
		if transport.Description() == "" {
			t.Error("Transport.Description() returned empty string")
		}
		
		// Priority must be positive
		if transport.Priority() < 0 {
			t.Errorf("Transport.Priority() returned negative value: %d", transport.Priority())
		}
		
		// Capabilities must not be nil
		if transport.Capabilities() == nil {
			t.Error("Transport.Capabilities() returned nil")
		}
	}
}

func testTransportLifecycle(transport ui.Transport) func(*testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		config := ui.TransportConfig{
			MaxConnections: 10,
			Timeout:        5 * time.Second,
		}
		
		// Test initialization
		if err := transport.Initialize(config); err != nil {
			t.Fatalf("Transport.Initialize() failed: %v", err)
		}
		
		// Test start
		if err := transport.Start(ctx); err != nil {
			t.Fatalf("Transport.Start() failed: %v", err)
		}
		
		// Starting again should either succeed (idempotent) or return a specific error
		err := transport.Start(ctx)
		if err != nil && err.Error() != "transport already started" {
			// Allow idempotent start or specific error
			t.Logf("Transport.Start() called twice: %v", err)
		}
		
		// Test stop
		if err := transport.Stop(ctx); err != nil {
			t.Fatalf("Transport.Stop() failed: %v", err)
		}
		
		// Stopping again should be idempotent
		if err := transport.Stop(ctx); err != nil {
			t.Logf("Transport.Stop() called twice (should be idempotent): %v", err)
		}
		
		// Should be able to restart
		if err := transport.Start(ctx); err != nil {
			t.Fatalf("Transport.Start() after Stop() failed: %v", err)
		}
		
		// Clean up
		transport.Stop(ctx)
	}
}

func testTransportErrorHandling(transport ui.Transport) func(*testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		
		// Starting without initialization should fail
		if err := transport.Start(ctx); err == nil {
			t.Error("Transport.Start() without Initialize() should fail")
		}
		
		// Initialize with invalid config
		invalidConfig := ui.TransportConfig{
			MaxConnections: -1, // Invalid
			Timeout:        -1 * time.Second, // Invalid
		}
		
		// Should handle invalid config gracefully (not panic)
		_ = transport.Initialize(invalidConfig)
		
		// Context cancellation during stop
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		// Stop with cancelled context should handle gracefully
		_ = transport.Stop(cancelCtx)
	}
}

func testTransportConcurrency(transport ui.Transport) func(*testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		config := ui.TransportConfig{
			MaxConnections: 10,
			Timeout:        5 * time.Second,
		}
		
		// Initialize for concurrent testing
		if err := transport.Initialize(config); err != nil {
			t.Fatalf("Transport.Initialize() failed: %v", err)
		}
		
		// Test concurrent health checks
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				_ = transport.HealthCheck(ctx)
				done <- true
			}()
		}
		
		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("Concurrent HealthCheck() timed out")
			}
		}
		
		// Test concurrent metadata access
		done = make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				_ = transport.Name()
				_ = transport.Priority()
				_ = transport.Capabilities()
				done <- true
			}()
		}
		
		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("Concurrent metadata access timed out")
			}
		}
	}
}

func testTransportHealthCheck(transport ui.Transport) func(*testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		
		// Health check should not modify state
		initialName := transport.Name()
		initialPriority := transport.Priority()
		
		// Perform health check
		_ = transport.HealthCheck(ctx)
		
		// Verify state unchanged
		if transport.Name() != initialName {
			t.Error("HealthCheck() modified Name")
		}
		if transport.Priority() != initialPriority {
			t.Error("HealthCheck() modified Priority")
		}
		
		// Context timeout should be respected
		timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
		defer cancel()
		
		// Short timeout might cause failure, but should not panic
		_ = transport.HealthCheck(timeoutCtx)
	}
}

// SessionComplianceTest runs a comprehensive test suite that all SessionManager implementations must pass
func SessionComplianceTest(t *testing.T, manager ui.SessionManager) {
	t.Run("Lifecycle", testSessionLifecycle(manager))
	t.Run("Messages", testSessionMessages(manager))
	t.Run("RateLimit", testSessionRateLimit(manager))
	t.Run("Concurrency", testSessionConcurrency(manager))
	t.Run("Metadata", testSessionMetadata(manager))
}

func testSessionLifecycle(manager ui.SessionManager) func(*testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		
		// Create session
		metadata := map[string]interface{}{"test": "value"}
		session, err := manager.Create(ctx, metadata)
		if err != nil {
			t.Fatalf("SessionManager.Create() failed: %v", err)
		}
		
		// Session ID must not be empty
		if session.ID == "" {
			t.Error("Session.ID is empty")
		}
		
		// Get session
		retrieved, err := manager.Get(ctx, session.ID)
		if err != nil {
			t.Fatalf("SessionManager.Get() failed: %v", err)
		}
		
		if retrieved.ID != session.ID {
			t.Error("Retrieved session ID doesn't match")
		}
		
		// Update session
		session.TokenCount = 100
		if err := manager.Update(ctx, session); err != nil {
			t.Fatalf("SessionManager.Update() failed: %v", err)
		}
		
		// Delete session
		if err := manager.Delete(ctx, session.ID); err != nil {
			t.Fatalf("SessionManager.Delete() failed: %v", err)
		}
		
		// Get deleted session should fail
		_, err = manager.Get(ctx, session.ID)
		if err == nil {
			t.Error("Getting deleted session should fail")
		}
	}
}

func testSessionMessages(manager ui.SessionManager) func(*testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		
		// Create session
		session, err := manager.Create(ctx, nil)
		if err != nil {
			t.Fatalf("SessionManager.Create() failed: %v", err)
		}
		
		// Add messages
		messages := []ui.Message{
			{
				ID:         "msg1",
				Role:       "user",
				Content:    "Hello",
				TokenCount: 1,
				Timestamp:  time.Now(),
			},
			{
				ID:         "msg2",
				Role:       "assistant",
				Content:    "Hi there!",
				TokenCount: 2,
				Timestamp:  time.Now(),
			},
		}
		
		for _, msg := range messages {
			if err := manager.AddMessage(ctx, session.ID, msg); err != nil {
				t.Fatalf("SessionManager.AddMessage() failed: %v", err)
			}
		}
		
		// Get messages
		retrieved, err := manager.GetMessages(ctx, session.ID, 10)
		if err != nil {
			t.Fatalf("SessionManager.GetMessages() failed: %v", err)
		}
		
		if len(retrieved) != len(messages) {
			t.Errorf("Expected %d messages, got %d", len(messages), len(retrieved))
		}
		
		// Test limit
		limited, err := manager.GetMessages(ctx, session.ID, 1)
		if err != nil {
			t.Fatalf("SessionManager.GetMessages() with limit failed: %v", err)
		}
		
		if len(limited) > 1 {
			t.Errorf("Expected at most 1 message, got %d", len(limited))
		}
		
		// Clean up
		manager.Delete(ctx, session.ID)
	}
}

func testSessionRateLimit(manager ui.SessionManager) func(*testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		
		// Create session
		session, err := manager.Create(ctx, nil)
		if err != nil {
			t.Fatalf("SessionManager.Create() failed: %v", err)
		}
		defer manager.Delete(ctx, session.ID)
		
		// Check rate limit
		allowed, resetAt, err := manager.CheckRateLimit(ctx, session.ID)
		if err != nil {
			t.Fatalf("SessionManager.CheckRateLimit() failed: %v", err)
		}
		
		// New session should not be rate limited initially
		if !allowed {
			t.Error("New session should not be rate limited")
		}
		
		// If rate limited, resetAt should be in the future
		if !allowed && !resetAt.After(time.Now()) {
			t.Error("Rate limit reset time should be in the future")
		}
	}
}

func testSessionConcurrency(manager ui.SessionManager) func(*testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		
		// Create session
		session, err := manager.Create(ctx, nil)
		if err != nil {
			t.Fatalf("SessionManager.Create() failed: %v", err)
		}
		defer manager.Delete(ctx, session.ID)
		
		// Concurrent message additions
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(idx int) {
				msg := ui.Message{
					ID:         fmt.Sprintf("msg%d", idx),
					Role:       "user",
					Content:    fmt.Sprintf("Message %d", idx),
					TokenCount: 1,
					Timestamp:  time.Now(),
				}
				_ = manager.AddMessage(ctx, session.ID, msg)
				done <- true
			}(i)
		}
		
		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("Concurrent AddMessage() timed out")
			}
		}
		
		// Verify messages were added
		messages, _ := manager.GetMessages(ctx, session.ID, 100)
		if len(messages) != 10 {
			t.Errorf("Expected 10 messages after concurrent adds, got %d", len(messages))
		}
	}
}

func testSessionMetadata(manager ui.SessionManager) func(*testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		
		// Create sessions with metadata
		metadata1 := map[string]interface{}{"user": "alice", "type": "chat"}
		session1, _ := manager.Create(ctx, metadata1)
		defer manager.Delete(ctx, session1.ID)
		
		metadata2 := map[string]interface{}{"user": "bob", "type": "chat"}
		session2, _ := manager.Create(ctx, metadata2)
		defer manager.Delete(ctx, session2.ID)
		
		metadata3 := map[string]interface{}{"user": "alice", "type": "support"}
		session3, _ := manager.Create(ctx, metadata3)
		defer manager.Delete(ctx, session3.ID)
		
		// Query by metadata
		aliceSessions, err := manager.GetSessionsByMetadata(ctx, "user", "alice")
		if err != nil {
			t.Fatalf("SessionManager.GetSessionsByMetadata() failed: %v", err)
		}
		
		if len(aliceSessions) != 2 {
			t.Errorf("Expected 2 sessions for alice, got %d", len(aliceSessions))
		}
		
		// Get active session count
		count, err := manager.GetActiveSessionCount(ctx)
		if err != nil {
			t.Fatalf("SessionManager.GetActiveSessionCount() failed: %v", err)
		}
		
		if count != 3 {
			t.Errorf("Expected 3 active sessions, got %d", count)
		}
	}
}

// StreamComplianceTest runs a comprehensive test suite that all StreamHandler implementations must pass
func StreamComplianceTest(t *testing.T, handler ui.StreamHandler) {
	t.Run("BasicStreaming", testBasicStreaming(handler))
	t.Run("ContextCancellation", testStreamContextCancellation(handler))
	t.Run("ErrorHandling", testStreamErrorHandling(handler))
}

func testBasicStreaming(handler ui.StreamHandler) func(*testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		
		// Start streaming
		events, err := handler.StreamResponse(ctx, "session123", "test message")
		if err != nil {
			t.Fatalf("StreamHandler.StreamResponse() failed: %v", err)
		}
		
		// Collect events
		var collected []ui.ChatEvent
		for event := range events {
			collected = append(collected, event)
		}
		
		// Should have at least one event
		if len(collected) == 0 {
			t.Error("No events received from stream")
		}
		
		// Check for completion event
		hasCompletion := false
		for _, event := range collected {
			if event.Type == ui.EventDone {
				hasCompletion = true
			}
		}
		
		if !hasCompletion {
			t.Error("Stream should end with EventDone")
		}
	}
}

func testStreamContextCancellation(handler ui.StreamHandler) func(*testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		
		// Start streaming
		events, err := handler.StreamResponse(ctx, "session123", "test message")
		if err != nil {
			t.Fatalf("StreamHandler.StreamResponse() failed: %v", err)
		}
		
		// Cancel context after receiving first event
		gotFirst := false
		for event := range events {
			if !gotFirst {
				gotFirst = true
				cancel() // Cancel after first event
			}
			_ = event
		}
		
		// Stream should have stopped after cancellation
		select {
		case _, ok := <-events:
			if ok {
				t.Error("Stream should be closed after context cancellation")
			}
		case <-time.After(1 * time.Second):
			// Channel properly closed
		}
	}
}

func testStreamErrorHandling(handler ui.StreamHandler) func(*testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		
		// Test with empty message
		events, err := handler.StreamResponse(ctx, "session123", "")
		if err == nil {
			// If no error returned, should handle gracefully in stream
			for range events {
				// Drain events
			}
		}
		
		// Test with empty session ID
		events, err = handler.StreamResponse(ctx, "", "message")
		if err == nil {
			// If no error returned, should handle gracefully in stream
			for range events {
				// Drain events
			}
		}
	}
}