package core

import (
	"context"
	"sync"
	"time"
)

// MemoryStore is an in-memory implementation of the Memory interface
type MemoryStore struct {
	mu    sync.RWMutex
	store map[string]memoryEntry
}

type memoryEntry struct {
	value     string
	expiresAt time.Time
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		store: make(map[string]memoryEntry),
	}
}

// Get retrieves a value from memory
func (m *MemoryStore) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	entry, exists := m.store[key]
	if !exists {
		return "", nil
	}
	
	// Check if expired
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		return "", nil
	}
	
	return entry.value, nil
}

// Set stores a value in memory with optional TTL
func (m *MemoryStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	entry := memoryEntry{
		value: value,
	}
	
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	
	m.store[key] = entry
	return nil
}

// Delete removes a value from memory
func (m *MemoryStore) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.store, key)
	return nil
}

// Exists checks if a key exists in memory
func (m *MemoryStore) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	entry, exists := m.store[key]
	if !exists {
		return false, nil
	}
	
	// Check if expired
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		return false, nil
	}
	
	return true, nil
}

// Store is an alias for Set for backward compatibility
func (m *MemoryStore) Store(ctx context.Context, key string, value interface{}) error {
	// Convert value to string
	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	default:
		strValue = ""
	}
	return m.Set(ctx, key, strValue, 0)
}

// Retrieve is an alias for Get for backward compatibility
func (m *MemoryStore) Retrieve(ctx context.Context, key string) (interface{}, error) {
	return m.Get(ctx, key)
}