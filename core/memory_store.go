package core

import (
	"context"
	"sync"
	"time"
)

// MemoryStore is an in-memory implementation of the Memory interface
type MemoryStore struct {
	mu     sync.RWMutex
	store  map[string]memoryEntry
	logger Logger
}

type memoryEntry struct {
	value     string
	expiresAt time.Time
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		store:  make(map[string]memoryEntry),
		logger: &NoOpLogger{},
	}
}

// SetLogger configures the logger for this memory store
func (m *MemoryStore) SetLogger(logger Logger) {
	if logger != nil {
		m.logger = logger
	}
}

// Get retrieves a value from memory
func (m *MemoryStore) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 🔥 ADD: Cache lookup logging
	if m.logger != nil {
		m.logger.Debug("Cache lookup", map[string]interface{}{
			"operation": "cache_get",
			"key":       key,
		})
	}

	entry, exists := m.store[key]
	if !exists {
		// 🔥 ADD: Cache miss logging
		if m.logger != nil {
			m.logger.Debug("Cache miss", map[string]interface{}{
				"operation": "cache_get",
				"key":       key,
				"result":    "miss",
			})
		}
		return "", nil
	}

	// Check if expired
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		// 🔥 ADD: Expiration logging
		if m.logger != nil {
			m.logger.Debug("Cache entry expired", map[string]interface{}{
				"operation":  "cache_get",
				"key":        key,
				"result":     "expired",
				"expired_at": entry.expiresAt.Format(time.RFC3339),
			})
		}
		return "", nil
	}

	// 🔥 ADD: Cache hit logging
	if m.logger != nil {
		m.logger.Debug("Cache hit", map[string]interface{}{
			"operation": "cache_get",
			"key":       key,
			"result":    "hit",
		})
	}

	return entry.value, nil
}

// Set stores a value in memory with optional TTL
func (m *MemoryStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 🔥 ADD: Cache set logging with context
	if m.logger != nil {
		logFields := map[string]interface{}{
			"operation":   "cache_set",
			"key":         key,
			"value_size":  len(value),
			"has_ttl":     ttl > 0,
		}
		if ttl > 0 {
			logFields["ttl"] = ttl.String()
			logFields["expires_at"] = time.Now().Add(ttl).Format(time.RFC3339)
		}
		m.logger.Debug("Cache set", logFields)
	}

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

	_, existed := m.store[key]
	delete(m.store, key)

	if m.logger != nil {
		m.logger.Debug("Cache delete", map[string]interface{}{
			"operation": "cache_delete",
			"key":       key,
			"existed":   existed,
		})
	}

	return nil
}

// Exists checks if a key exists in memory
func (m *MemoryStore) Exists(ctx context.Context, key string) (bool, error) {
	if m.logger != nil {
		m.logger.Debug("Cache existence check", map[string]interface{}{
			"operation": "cache_exists",
			"key":       key,
		})
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.store[key]
	if !exists {
		if m.logger != nil {
			m.logger.Debug("Cache existence result", map[string]interface{}{
				"operation": "cache_exists",
				"key":       key,
				"result":    "not_found",
				"exists":    false,
			})
		}
		return false, nil
	}

	// Check if expired
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		if m.logger != nil {
			m.logger.Debug("Cache existence result", map[string]interface{}{
				"operation":  "cache_exists",
				"key":        key,
				"result":     "expired",
				"exists":     false,
				"expired_at": entry.expiresAt.Format(time.RFC3339),
			})
		}
		return false, nil
	}

	if m.logger != nil {
		m.logger.Debug("Cache existence result", map[string]interface{}{
			"operation": "cache_exists",
			"key":       key,
			"result":    "found",
			"exists":    true,
		})
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