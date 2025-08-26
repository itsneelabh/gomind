package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/pkg/memory"
)

// TestInMemoryStore tests the in-memory storage implementation
func TestInMemoryStore(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()

	// Test Set and Get
	key := "test-key"
	value := "test-value"
	err := store.Set(ctx, key, value, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	retrieved, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	if retrieved != value {
		t.Errorf("Retrieved value doesn't match: got %v, want %v", retrieved, value)
	}

	// Test Exists
	exists, err := store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Failed to check existence: %v", err)
	}
	if !exists {
		t.Error("Key should exist")
	}

	// Test Delete
	err = store.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Failed to delete key: %v", err)
	}

	exists, err = store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Failed to check existence after delete: %v", err)
	}
	if exists {
		t.Error("Key should not exist after deletion")
	}
}

// TestInMemoryStoreExpiration tests TTL expiration
func TestInMemoryStoreExpiration(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()

	// Set a value with short TTL
	key := "expiring-key"
	value := "expiring-value"
	err := store.Set(ctx, key, value, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Value should exist immediately
	exists, err := store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Failed to check existence: %v", err)
	}
	if !exists {
		t.Error("Key should exist immediately after setting")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Value should be expired
	_, err = store.Get(ctx, key)
	if err == nil {
		t.Error("Expected error when getting expired key")
	}
}

// TestInMemoryStoreSetTTL tests the SetTTL method
func TestInMemoryStoreSetTTL(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()

	// Set custom default TTL
	customTTL := 2 * time.Hour
	store.SetTTL(customTTL)

	// Set a value without specifying TTL (should use default)
	key := "default-ttl-key"
	value := "default-ttl-value"
	err := store.Set(ctx, key, value, 0)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Value should exist
	exists, err := store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Failed to check existence: %v", err)
	}
	if !exists {
		t.Error("Key should exist")
	}
}