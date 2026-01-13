package orchestration

import (
	"context"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// MemoryLLMDebugStore Tests
// =============================================================================

func TestMemoryLLMDebugStore_RecordInteraction(t *testing.T) {
	store := NewMemoryLLMDebugStore()
	ctx := context.Background()
	requestID := "test-request-1"

	interaction := LLMInteraction{
		Type:             "plan_generation",
		Timestamp:        time.Now(),
		DurationMs:       100,
		Prompt:           "Test prompt",
		Response:         "Test response",
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
		Success:          true,
	}

	// Record first interaction
	err := store.RecordInteraction(ctx, requestID, interaction)
	if err != nil {
		t.Fatalf("RecordInteraction failed: %v", err)
	}

	// Verify record was created
	record, err := store.GetRecord(ctx, requestID)
	if err != nil {
		t.Fatalf("GetRecord failed: %v", err)
	}

	if record.RequestID != requestID {
		t.Errorf("Expected RequestID %s, got %s", requestID, record.RequestID)
	}

	if len(record.Interactions) != 1 {
		t.Errorf("Expected 1 interaction, got %d", len(record.Interactions))
	}

	if record.Interactions[0].Prompt != "Test prompt" {
		t.Errorf("Expected prompt 'Test prompt', got %s", record.Interactions[0].Prompt)
	}

	// Record second interaction
	interaction2 := LLMInteraction{
		Type:       "synthesis",
		Timestamp:  time.Now(),
		DurationMs: 50,
		Prompt:     "Second prompt",
		Response:   "Second response",
		Success:    true,
	}

	err = store.RecordInteraction(ctx, requestID, interaction2)
	if err != nil {
		t.Fatalf("Second RecordInteraction failed: %v", err)
	}

	// Verify both interactions are present
	record, err = store.GetRecord(ctx, requestID)
	if err != nil {
		t.Fatalf("GetRecord after second interaction failed: %v", err)
	}

	if len(record.Interactions) != 2 {
		t.Errorf("Expected 2 interactions, got %d", len(record.Interactions))
	}
}

func TestMemoryLLMDebugStore_GetRecord_NotFound(t *testing.T) {
	store := NewMemoryLLMDebugStore()
	ctx := context.Background()

	_, err := store.GetRecord(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent record")
	}
}

func TestMemoryLLMDebugStore_SetMetadata(t *testing.T) {
	store := NewMemoryLLMDebugStore()
	ctx := context.Background()
	requestID := "test-request-2"

	// First create a record
	err := store.RecordInteraction(ctx, requestID, LLMInteraction{
		Type:    "plan_generation",
		Success: true,
	})
	if err != nil {
		t.Fatalf("RecordInteraction failed: %v", err)
	}

	// Set metadata
	err = store.SetMetadata(ctx, requestID, "investigation", "high_priority")
	if err != nil {
		t.Fatalf("SetMetadata failed: %v", err)
	}

	// Verify metadata
	record, err := store.GetRecord(ctx, requestID)
	if err != nil {
		t.Fatalf("GetRecord failed: %v", err)
	}

	if record.Metadata["investigation"] != "high_priority" {
		t.Errorf("Expected metadata 'high_priority', got %s", record.Metadata["investigation"])
	}
}

func TestMemoryLLMDebugStore_SetMetadata_NotFound(t *testing.T) {
	store := NewMemoryLLMDebugStore()
	ctx := context.Background()

	err := store.SetMetadata(ctx, "non-existent", "key", "value")
	if err == nil {
		t.Error("Expected error for non-existent record")
	}
}

func TestMemoryLLMDebugStore_ExtendTTL(t *testing.T) {
	store := NewMemoryLLMDebugStore()
	ctx := context.Background()
	requestID := "test-request-3"

	// Create a record
	err := store.RecordInteraction(ctx, requestID, LLMInteraction{
		Type:    "plan_generation",
		Success: true,
	})
	if err != nil {
		t.Fatalf("RecordInteraction failed: %v", err)
	}

	// ExtendTTL should succeed for existing record
	err = store.ExtendTTL(ctx, requestID, 24*time.Hour)
	if err != nil {
		t.Errorf("ExtendTTL failed for existing record: %v", err)
	}

	// ExtendTTL should fail for non-existent record
	err = store.ExtendTTL(ctx, "non-existent", 24*time.Hour)
	if err == nil {
		t.Error("Expected error for non-existent record")
	}
}

func TestMemoryLLMDebugStore_ListRecent(t *testing.T) {
	store := NewMemoryLLMDebugStore()
	ctx := context.Background()

	// Create multiple records
	for i := 0; i < 5; i++ {
		requestID := "test-request-" + string(rune('a'+i))
		success := i%2 == 0 // Alternate success/failure

		err := store.RecordInteraction(ctx, requestID, LLMInteraction{
			Type:        "plan_generation",
			TotalTokens: (i + 1) * 100,
			Success:     success,
		})
		if err != nil {
			t.Fatalf("RecordInteraction failed: %v", err)
		}

		// Small delay to ensure different timestamps
		time.Sleep(1 * time.Millisecond)
	}

	// List all records
	summaries, err := store.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("ListRecent failed: %v", err)
	}

	if len(summaries) != 5 {
		t.Errorf("Expected 5 summaries, got %d", len(summaries))
	}

	// Verify ordered by creation time (newest first)
	for i := 1; i < len(summaries); i++ {
		if summaries[i].CreatedAt.After(summaries[i-1].CreatedAt) {
			t.Error("Records should be ordered by creation time (newest first)")
		}
	}

	// Test limit
	summaries, err = store.ListRecent(ctx, 2)
	if err != nil {
		t.Fatalf("ListRecent with limit failed: %v", err)
	}

	if len(summaries) != 2 {
		t.Errorf("Expected 2 summaries with limit, got %d", len(summaries))
	}
}

func TestMemoryLLMDebugStore_ClearAndCount(t *testing.T) {
	store := NewMemoryLLMDebugStore()
	ctx := context.Background()

	// Add some records
	for i := 0; i < 3; i++ {
		store.RecordInteraction(ctx, "request-"+string(rune('a'+i)), LLMInteraction{
			Type:    "plan_generation",
			Success: true,
		})
	}

	if store.Count() != 3 {
		t.Errorf("Expected count 3, got %d", store.Count())
	}

	store.Clear()

	if store.Count() != 0 {
		t.Errorf("Expected count 0 after clear, got %d", store.Count())
	}
}

func TestMemoryLLMDebugStore_ConcurrentAccess(t *testing.T) {
	store := NewMemoryLLMDebugStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				requestID := "concurrent-request"
				store.RecordInteraction(ctx, requestID, LLMInteraction{
					Type:    "plan_generation",
					Attempt: goroutineID*numOperations + j,
					Success: true,
				})
			}
		}(i)
	}

	wg.Wait()

	// Verify record exists
	record, err := store.GetRecord(ctx, "concurrent-request")
	if err != nil {
		t.Fatalf("GetRecord failed: %v", err)
	}

	expectedInteractions := numGoroutines * numOperations
	if len(record.Interactions) != expectedInteractions {
		t.Errorf("Expected %d interactions, got %d", expectedInteractions, len(record.Interactions))
	}
}

// =============================================================================
// NoOpLLMDebugStore Tests
// =============================================================================

func TestNoOpLLMDebugStore_RecordInteraction(t *testing.T) {
	store := NewNoOpLLMDebugStore()
	ctx := context.Background()

	// Should always succeed silently
	err := store.RecordInteraction(ctx, "any-request", LLMInteraction{
		Type:    "plan_generation",
		Success: true,
	})

	if err != nil {
		t.Errorf("NoOp RecordInteraction should not return error, got: %v", err)
	}
}

func TestNoOpLLMDebugStore_GetRecord(t *testing.T) {
	store := NewNoOpLLMDebugStore()
	ctx := context.Background()

	// Should return error indicating not configured
	_, err := store.GetRecord(ctx, "any-request")
	if err == nil {
		t.Error("NoOp GetRecord should return error")
	}
}

func TestNoOpLLMDebugStore_SetMetadata(t *testing.T) {
	store := NewNoOpLLMDebugStore()
	ctx := context.Background()

	// Should always succeed silently
	err := store.SetMetadata(ctx, "any-request", "key", "value")
	if err != nil {
		t.Errorf("NoOp SetMetadata should not return error, got: %v", err)
	}
}

func TestNoOpLLMDebugStore_ExtendTTL(t *testing.T) {
	store := NewNoOpLLMDebugStore()
	ctx := context.Background()

	// Should always succeed silently
	err := store.ExtendTTL(ctx, "any-request", 24*time.Hour)
	if err != nil {
		t.Errorf("NoOp ExtendTTL should not return error, got: %v", err)
	}
}

func TestNoOpLLMDebugStore_ListRecent(t *testing.T) {
	store := NewNoOpLLMDebugStore()
	ctx := context.Background()

	// Should return empty slice
	summaries, err := store.ListRecent(ctx, 10)
	if err != nil {
		t.Errorf("NoOp ListRecent should not return error, got: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("NoOp ListRecent should return empty slice, got %d items", len(summaries))
	}
}

// =============================================================================
// recordDebugInteraction Tests
// =============================================================================

func TestOrchestrator_recordDebugInteraction_NilStore(t *testing.T) {
	// Create orchestrator without debug store
	aiClient := NewMockAIClient()
	discovery := &MockDiscovery{}
	orchestrator := NewAIOrchestrator(nil, discovery, aiClient)

	// Should return immediately without panic
	orchestrator.recordDebugInteraction(context.Background(), "test-request", LLMInteraction{
		Type:    "plan_generation",
		Success: true,
	})

	// No assertion needed - just verify it doesn't panic
}

func TestOrchestrator_recordDebugInteraction_WithStore(t *testing.T) {
	// Create orchestrator with memory store
	aiClient := NewMockAIClient()
	discovery := &MockDiscovery{}
	orchestrator := NewAIOrchestrator(nil, discovery, aiClient)

	store := NewMemoryLLMDebugStore()
	orchestrator.SetLLMDebugStore(store)

	requestID := "test-request-record"
	interaction := LLMInteraction{
		Type:       "plan_generation",
		Prompt:     "Test prompt for recording",
		Response:   "Test response",
		Success:    true,
		DurationMs: 150,
	}

	// Record interaction
	orchestrator.recordDebugInteraction(context.Background(), requestID, interaction)

	// Wait for async goroutine to complete
	time.Sleep(100 * time.Millisecond)

	// Verify interaction was recorded
	record, err := store.GetRecord(context.Background(), requestID)
	if err != nil {
		t.Fatalf("GetRecord failed: %v", err)
	}

	if len(record.Interactions) != 1 {
		t.Errorf("Expected 1 interaction, got %d", len(record.Interactions))
	}

	if record.Interactions[0].Prompt != "Test prompt for recording" {
		t.Errorf("Prompt mismatch: got %s", record.Interactions[0].Prompt)
	}
}

func TestSynthesizer_recordDebugInteraction_WithStore(t *testing.T) {
	aiClient := NewMockAIClient()
	synthesizer := NewAISynthesizer(aiClient)

	store := NewMemoryLLMDebugStore()
	synthesizer.SetLLMDebugStore(store)

	requestID := "test-synth-request"
	interaction := LLMInteraction{
		Type:     "synthesis",
		Prompt:   "Synthesize these results",
		Response: "Synthesized response",
		Success:  true,
	}

	// Record interaction
	synthesizer.recordDebugInteraction(context.Background(), requestID, interaction)

	// Wait for async goroutine
	time.Sleep(100 * time.Millisecond)

	// Verify
	record, err := store.GetRecord(context.Background(), requestID)
	if err != nil {
		t.Fatalf("GetRecord failed: %v", err)
	}

	if len(record.Interactions) != 1 {
		t.Errorf("Expected 1 interaction, got %d", len(record.Interactions))
	}

	if record.Interactions[0].Type != "synthesis" {
		t.Errorf("Expected type 'synthesis', got %s", record.Interactions[0].Type)
	}
}

func TestMicroResolver_recordDebugInteraction_WithStore(t *testing.T) {
	aiClient := NewMockAIClient()
	microResolver := NewMicroResolver(aiClient, nil)

	store := NewMemoryLLMDebugStore()
	microResolver.SetLLMDebugStore(store)

	requestID := "test-micro-request"
	interaction := LLMInteraction{
		Type:     "micro_resolution",
		Prompt:   "Resolve parameters",
		Response: `{"param": "value"}`,
		Success:  true,
	}

	// Record interaction
	microResolver.recordDebugInteraction(context.Background(), requestID, interaction)

	// Wait for async goroutine
	time.Sleep(100 * time.Millisecond)

	// Verify
	record, err := store.GetRecord(context.Background(), requestID)
	if err != nil {
		t.Fatalf("GetRecord failed: %v", err)
	}

	if record.Interactions[0].Type != "micro_resolution" {
		t.Errorf("Expected type 'micro_resolution', got %s", record.Interactions[0].Type)
	}
}

func TestContextualReResolver_recordDebugInteraction_WithStore(t *testing.T) {
	aiClient := NewMockAIClient()
	reResolver := NewContextualReResolver(aiClient, nil)

	store := NewMemoryLLMDebugStore()
	reResolver.SetLLMDebugStore(store)

	requestID := "test-reresolver-request"
	interaction := LLMInteraction{
		Type:     "semantic_retry",
		Prompt:   "Re-resolve parameters",
		Response: `{"should_retry": true}`,
		Success:  true,
		Attempt:  2,
	}

	// Record interaction
	reResolver.recordDebugInteraction(context.Background(), requestID, interaction)

	// Wait for async goroutine
	time.Sleep(100 * time.Millisecond)

	// Verify
	record, err := store.GetRecord(context.Background(), requestID)
	if err != nil {
		t.Fatalf("GetRecord failed: %v", err)
	}

	if record.Interactions[0].Type != "semantic_retry" {
		t.Errorf("Expected type 'semantic_retry', got %s", record.Interactions[0].Type)
	}

	if record.Interactions[0].Attempt != 2 {
		t.Errorf("Expected attempt 2, got %d", record.Interactions[0].Attempt)
	}
}

// =============================================================================
// SetLLMDebugStore Propagation Tests
// =============================================================================

func TestOrchestrator_SetLLMDebugStore_Propagation(t *testing.T) {
	aiClient := NewMockAIClient()
	discovery := &MockDiscovery{}

	// Create config with hybrid resolution enabled to ensure sub-components are created
	config := DefaultConfig()
	config.EnableHybridResolution = true
	config.SemanticRetry.Enabled = true

	orchestrator := NewAIOrchestrator(config, discovery, aiClient)

	// Create store and set it
	store := NewMemoryLLMDebugStore()
	orchestrator.SetLLMDebugStore(store)

	// Verify store was set on orchestrator
	if orchestrator.debugStore != store {
		t.Error("debugStore not set on orchestrator")
	}

	// Verify store was propagated to synthesizer
	if orchestrator.synthesizer != nil && orchestrator.synthesizer.debugStore != store {
		t.Error("debugStore not propagated to synthesizer")
	}
}

func TestOrchestrator_SetLLMDebugStore_NilGuard(t *testing.T) {
	aiClient := NewMockAIClient()
	discovery := &MockDiscovery{}
	orchestrator := NewAIOrchestrator(nil, discovery, aiClient)

	// First set a real store
	store := NewMemoryLLMDebugStore()
	orchestrator.SetLLMDebugStore(store)

	if orchestrator.debugStore == nil {
		t.Error("debugStore should be set")
	}

	// Try to set nil - should be ignored
	orchestrator.SetLLMDebugStore(nil)

	// Store should still be the original
	if orchestrator.debugStore != store {
		t.Error("debugStore should not be replaced with nil")
	}
}

// =============================================================================
// Shutdown Tests
// =============================================================================

func TestOrchestrator_Shutdown_WaitsForRecordings(t *testing.T) {
	aiClient := NewMockAIClient()
	discovery := &MockDiscovery{}
	orchestrator := NewAIOrchestrator(nil, discovery, aiClient)

	store := NewMemoryLLMDebugStore()
	orchestrator.SetLLMDebugStore(store)

	// Record multiple interactions
	for i := 0; i < 5; i++ {
		orchestrator.recordDebugInteraction(context.Background(), "shutdown-test", LLMInteraction{
			Type:    "plan_generation",
			Attempt: i,
			Success: true,
		})
	}

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := orchestrator.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Verify all interactions were recorded
	record, err := store.GetRecord(context.Background(), "shutdown-test")
	if err != nil {
		t.Fatalf("GetRecord failed: %v", err)
	}

	if len(record.Interactions) != 5 {
		t.Errorf("Expected 5 interactions after shutdown, got %d", len(record.Interactions))
	}
}

func TestOrchestrator_Shutdown_Timeout(t *testing.T) {
	aiClient := NewMockAIClient()
	discovery := &MockDiscovery{}
	orchestrator := NewAIOrchestrator(nil, discovery, aiClient)

	// Create a slow store that simulates delay
	slowStore := &slowDebugStore{delay: 2 * time.Second}
	orchestrator.SetLLMDebugStore(slowStore)

	// Record an interaction
	orchestrator.recordDebugInteraction(context.Background(), "timeout-test", LLMInteraction{
		Type:    "plan_generation",
		Success: true,
	})

	// Shutdown with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := orchestrator.Shutdown(ctx)
	if err == nil {
		t.Error("Expected timeout error from Shutdown")
	}
}

func TestSynthesizer_Shutdown(t *testing.T) {
	aiClient := NewMockAIClient()
	synthesizer := NewAISynthesizer(aiClient)

	store := NewMemoryLLMDebugStore()
	synthesizer.SetLLMDebugStore(store)

	// Record an interaction
	synthesizer.recordDebugInteraction(context.Background(), "synth-shutdown", LLMInteraction{
		Type:    "synthesis",
		Success: true,
	})

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := synthesizer.Shutdown(ctx)
	if err != nil {
		t.Errorf("Synthesizer Shutdown failed: %v", err)
	}

	// Verify recording completed
	record, err := store.GetRecord(context.Background(), "synth-shutdown")
	if err != nil {
		t.Fatalf("GetRecord failed: %v", err)
	}

	if len(record.Interactions) != 1 {
		t.Error("Expected interaction to be recorded before shutdown")
	}
}

func TestMicroResolver_Shutdown(t *testing.T) {
	aiClient := NewMockAIClient()
	microResolver := NewMicroResolver(aiClient, nil)

	store := NewMemoryLLMDebugStore()
	microResolver.SetLLMDebugStore(store)

	// Record an interaction
	microResolver.recordDebugInteraction(context.Background(), "micro-shutdown", LLMInteraction{
		Type:    "micro_resolution",
		Success: true,
	})

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := microResolver.Shutdown(ctx)
	if err != nil {
		t.Errorf("MicroResolver Shutdown failed: %v", err)
	}
}

func TestContextualReResolver_Shutdown(t *testing.T) {
	aiClient := NewMockAIClient()
	reResolver := NewContextualReResolver(aiClient, nil)

	store := NewMemoryLLMDebugStore()
	reResolver.SetLLMDebugStore(store)

	// Record an interaction
	reResolver.recordDebugInteraction(context.Background(), "reresolver-shutdown", LLMInteraction{
		Type:    "semantic_retry",
		Success: true,
	})

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := reResolver.Shutdown(ctx)
	if err != nil {
		t.Errorf("ContextualReResolver Shutdown failed: %v", err)
	}
}

// =============================================================================
// generateFallbackRequestID Tests
// =============================================================================

func TestOrchestrator_generateFallbackRequestID_Unique(t *testing.T) {
	aiClient := NewMockAIClient()
	discovery := &MockDiscovery{}
	orchestrator := NewAIOrchestrator(nil, discovery, aiClient)

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := orchestrator.generateFallbackRequestID()
		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestSynthesizer_generateFallbackRequestID_Unique(t *testing.T) {
	synthesizer := &AISynthesizer{}

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := synthesizer.generateFallbackRequestID()
		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}

// =============================================================================
// Factory Options Tests
// =============================================================================

func TestWithLLMDebug_Enabled(t *testing.T) {
	config := DefaultConfig()
	WithLLMDebug(true)(config)

	if !config.LLMDebug.Enabled {
		t.Error("LLMDebug should be enabled")
	}
}

func TestWithLLMDebug_Disabled(t *testing.T) {
	config := DefaultConfig()
	config.LLMDebug.Enabled = true // First enable it
	WithLLMDebug(false)(config)

	if config.LLMDebug.Enabled {
		t.Error("LLMDebug should be disabled")
	}
}

func TestWithLLMDebugStore(t *testing.T) {
	config := DefaultConfig()
	store := NewMemoryLLMDebugStore()
	WithLLMDebugStore(store)(config)

	if !config.LLMDebug.Enabled {
		t.Error("LLMDebug should be auto-enabled when store is set")
	}

	if config.LLMDebugStore != store {
		t.Error("LLMDebugStore should be set")
	}
}

func TestWithLLMDebugTTL(t *testing.T) {
	config := DefaultConfig()
	customTTL := 48 * time.Hour
	WithLLMDebugTTL(customTTL)(config)

	if config.LLMDebug.TTL != customTTL {
		t.Errorf("Expected TTL %v, got %v", customTTL, config.LLMDebug.TTL)
	}
}

func TestWithLLMDebugErrorTTL(t *testing.T) {
	config := DefaultConfig()
	customTTL := 14 * 24 * time.Hour
	WithLLMDebugErrorTTL(customTTL)(config)

	if config.LLMDebug.ErrorTTL != customTTL {
		t.Errorf("Expected ErrorTTL %v, got %v", customTTL, config.LLMDebug.ErrorTTL)
	}
}

// =============================================================================
// GetLLMDebugStore Test
// =============================================================================

func TestOrchestrator_GetLLMDebugStore(t *testing.T) {
	aiClient := NewMockAIClient()
	discovery := &MockDiscovery{}
	orchestrator := NewAIOrchestrator(nil, discovery, aiClient)

	// Initially nil
	if orchestrator.GetLLMDebugStore() != nil {
		t.Error("GetLLMDebugStore should return nil initially")
	}

	// After setting
	store := NewMemoryLLMDebugStore()
	orchestrator.SetLLMDebugStore(store)

	if orchestrator.GetLLMDebugStore() != store {
		t.Error("GetLLMDebugStore should return the configured store")
	}
}

// =============================================================================
// Test Helpers
// =============================================================================

// slowDebugStore is a test helper that simulates slow storage operations
type slowDebugStore struct {
	delay time.Duration
}

func (s *slowDebugStore) RecordInteraction(ctx context.Context, requestID string, interaction LLMInteraction) error {
	time.Sleep(s.delay)
	return nil
}

func (s *slowDebugStore) GetRecord(ctx context.Context, requestID string) (*LLMDebugRecord, error) {
	return nil, nil
}

func (s *slowDebugStore) SetMetadata(ctx context.Context, requestID string, key, value string) error {
	return nil
}

func (s *slowDebugStore) ExtendTTL(ctx context.Context, requestID string, duration time.Duration) error {
	return nil
}

func (s *slowDebugStore) ListRecent(ctx context.Context, limit int) ([]LLMDebugRecordSummary, error) {
	return nil, nil
}
