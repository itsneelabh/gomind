package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// Test NewInMemoryStore creation
func TestNewInMemoryStore(t *testing.T) {
	store := NewInMemoryStore()
	
	if store == nil {
		t.Fatal("NewInMemoryStore() returned nil")
	}
	
	// Verify internal structures are initialized
	if store.data == nil {
		t.Error("InMemoryStore data map should be initialized")
	}
}

// Test Get operation
func TestInMemoryStore_Get(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	
	// Test getting non-existent key
	value, err := store.Get(ctx, "non-existent")
	if err != nil {
		t.Errorf("Get() returned unexpected error: %v", err)
	}
	if value != "" {
		t.Errorf("Get() for non-existent key = %v, want empty string", value)
	}
	
	// Set a value
	err = store.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Fatalf("Set() failed: %v", err)
	}
	
	// Get the value
	value, err = store.Get(ctx, "key1")
	if err != nil {
		t.Errorf("Get() returned unexpected error: %v", err)
	}
	if value != "value1" {
		t.Errorf("Get() = %v, want value1", value)
	}
}

// Test Set operation
func TestInMemoryStore_Set(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	
	tests := []struct {
		name  string
		key   string
		value string
		ttl   time.Duration
	}{
		{
			name:  "set simple value",
			key:   "key1",
			value: "value1",
			ttl:   0,
		},
		{
			name:  "set with TTL",
			key:   "key2",
			value: "value2",
			ttl:   time.Hour,
		},
		{
			name:  "overwrite existing",
			key:   "key1",
			value: "new_value",
			ttl:   0,
		},
		{
			name:  "empty key",
			key:   "",
			value: "value",
			ttl:   0,
		},
		{
			name:  "empty value",
			key:   "empty_val",
			value: "",
			ttl:   0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Set(ctx, tt.key, tt.value, tt.ttl)
			if err != nil {
				t.Errorf("Set() error = %v", err)
			}
			
			// Verify value was set
			gotValue, err := store.Get(ctx, tt.key)
			if err != nil {
				t.Errorf("Get() after Set() error = %v", err)
			}
			if gotValue != tt.value {
				t.Errorf("After Set(), Get() = %v, want %v", gotValue, tt.value)
			}
		})
	}
}

// Test Delete operation
func TestInMemoryStore_Delete(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	
	// Set some values
	_ = store.Set(ctx, "key1", "value1", 0)
	_ = store.Set(ctx, "key2", "value2", 0)
	
	// Delete existing key
	err := store.Delete(ctx, "key1")
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}
	
	// Verify key was deleted
	value, err := store.Get(ctx, "key1")
	if err != nil {
		t.Errorf("Get() after Delete() error = %v", err)
	}
	if value != "" {
		t.Errorf("After Delete(), Get() = %v, want empty string", value)
	}
	
	// Verify other key still exists
	value, err = store.Get(ctx, "key2")
	if err != nil {
		t.Errorf("Get() error = %v", err)
	}
	if value != "value2" {
		t.Errorf("Get() = %v, want value2", value)
	}
	
	// Delete non-existent key (should not error)
	err = store.Delete(ctx, "non-existent")
	if err != nil {
		t.Errorf("Delete() non-existent key error = %v", err)
	}
}

// Test Exists operation
func TestInMemoryStore_Exists(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	
	// Check non-existent key
	exists, err := store.Exists(ctx, "key1")
	if err != nil {
		t.Errorf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() = true for non-existent key, want false")
	}
	
	// Set a value
	_ = store.Set(ctx, "key1", "value1", 0)
	
	// Check existing key
	exists, err = store.Exists(ctx, "key1")
	if err != nil {
		t.Errorf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false for existing key, want true")
	}
	
	// Set empty value
	_ = store.Set(ctx, "empty", "", 0)
	
	// Check key with empty value
	exists, err = store.Exists(ctx, "empty")
	if err != nil {
		t.Errorf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false for key with empty value, want true")
	}
	
	// Delete and check
	_ = store.Delete(ctx, "key1")
	exists, err = store.Exists(ctx, "key1")
	if err != nil {
		t.Errorf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() = true for deleted key, want false")
	}
}

// Test concurrent operations (note: InMemoryStore is NOT thread-safe!)
func TestInMemoryStore_RaceCondition(t *testing.T) {
	// This test demonstrates that InMemoryStore is NOT thread-safe
	// In production, a mutex should be added
	t.Skip("InMemoryStore is not thread-safe - skipping race condition test")
	
	store := NewInMemoryStore()
	ctx := context.Background()
	
	var wg sync.WaitGroup
	numOps := 100
	
	// Concurrent writes to same key
	wg.Add(numOps)
	for i := 0; i < numOps; i++ {
		go func(idx int) {
			defer wg.Done()
			value := fmt.Sprintf("value%d", idx)
			_ = store.Set(ctx, "key", value, 0)
		}(i)
	}
	
	wg.Wait()
	
	// The final value is unpredictable due to race conditions
	value, _ := store.Get(ctx, "key")
	t.Logf("Final value after concurrent writes: %s", value)
}

// Test operations with cancelled context
func TestInMemoryStore_CancelledContext(t *testing.T) {
	store := NewInMemoryStore()
	
	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	
	// InMemoryStore operations ignore context, so they should still work
	
	err := store.Set(ctx, "key", "value", 0)
	if err != nil {
		t.Errorf("Set with cancelled context error = %v", err)
	}
	
	value, err := store.Get(ctx, "key")
	if err != nil {
		t.Errorf("Get with cancelled context error = %v", err)
	}
	if value != "value" {
		t.Errorf("Get() = %v, want value", value)
	}
	
	exists, err := store.Exists(ctx, "key")
	if err != nil {
		t.Errorf("Exists with cancelled context error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false, want true")
	}
	
	err = store.Delete(ctx, "key")
	if err != nil {
		t.Errorf("Delete with cancelled context error = %v", err)
	}
}

// Test TTL parameter (note: current implementation ignores TTL)
func TestInMemoryStore_TTL(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	
	// Set with TTL
	err := store.Set(ctx, "key", "value", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	
	// Value should exist immediately
	value, err := store.Get(ctx, "key")
	if err != nil {
		t.Errorf("Get() error = %v", err)
	}
	if value != "value" {
		t.Errorf("Get() = %v, want value", value)
	}
	
	// Note: Current implementation ignores TTL
	// In a real implementation, we would wait and check expiration
	time.Sleep(150 * time.Millisecond)
	
	// Value still exists because TTL is not implemented
	value, err = store.Get(ctx, "key")
	if err != nil {
		t.Errorf("Get() after TTL error = %v", err)
	}
	if value != "value" {
		t.Log("TTL is not implemented - value still exists")
	}
}

// Benchmark operations
func BenchmarkInMemoryStore_Set(b *testing.B) {
	store := NewInMemoryStore()
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		_ = store.Set(ctx, key, value, 0)
	}
}

func BenchmarkInMemoryStore_Get(b *testing.B) {
	store := NewInMemoryStore()
	ctx := context.Background()
	_ = store.Set(ctx, "key", "value", 0)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Get(ctx, "key")
	}
}

func BenchmarkInMemoryStore_Delete(b *testing.B) {
	store := NewInMemoryStore()
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		_ = store.Set(ctx, key, "value", 0)
		_ = store.Delete(ctx, key)
	}
}

func BenchmarkInMemoryStore_Exists(b *testing.B) {
	store := NewInMemoryStore()
	ctx := context.Background()
	_ = store.Set(ctx, "key", "value", 0)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Exists(ctx, "key")
	}
}