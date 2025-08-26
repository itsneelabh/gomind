package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisMemory implements the Memory interface using Redis
type RedisMemory struct {
	client     *redis.Client
	namespace  string
	defaultTTL time.Duration
	mu         sync.RWMutex
}

// NewRedisMemory creates a new Redis-based memory store
func NewRedisMemory(redisURL, namespace string) (*RedisMemory, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	if namespace == "" {
		namespace = "memory"
	}

	return &RedisMemory{
		client:     client,
		namespace:  namespace,
		defaultTTL: time.Hour, // Default 1 hour TTL
	}, nil
}

// Set stores a key-value pair with TTL
func (r *RedisMemory) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	finalKey := r.buildKey(key)

	// Serialize value to JSON
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to serialize value: %w", err)
	}

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = r.defaultTTL
	}

	err = r.client.Set(ctx, finalKey, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set key: %w", err)
	}

	return nil
}

// Get retrieves a value by key
func (r *RedisMemory) Get(ctx context.Context, key string) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	finalKey := r.buildKey(key)

	data, err := r.client.Get(ctx, finalKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	var value interface{}
	if err := json.Unmarshal([]byte(data), &value); err != nil {
		// If JSON unmarshal fails, return as string
		return data, nil
	}

	return value, nil
}

// Delete removes a key
func (r *RedisMemory) Delete(ctx context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	finalKey := r.buildKey(key)

	err := r.client.Del(ctx, finalKey).Err()
	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}

	return nil
}

// Exists checks if a key exists
func (r *RedisMemory) Exists(ctx context.Context, key string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	finalKey := r.buildKey(key)

	result, err := r.client.Exists(ctx, finalKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check key existence: %w", err)
	}

	return result > 0, nil
}

// SetTTL sets the default TTL for future operations
func (r *RedisMemory) SetTTL(ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultTTL = ttl
}

// buildKey creates a namespaced key
func (r *RedisMemory) buildKey(key string) string {
	return fmt.Sprintf("%s:%s", r.namespace, key)
}

// Close closes the Redis connection
func (r *RedisMemory) Close() error {
	return r.client.Close()
}

// InMemoryStore provides a simple in-memory implementation for testing
type InMemoryStore struct {
	data       map[string]valueWithExpiry
	defaultTTL time.Duration
	mu         sync.RWMutex
}

type valueWithExpiry struct {
	value  interface{}
	expiry time.Time
}

// NewInMemoryStore creates a new in-memory store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data:       make(map[string]valueWithExpiry),
		defaultTTL: time.Hour,
	}
}

// Set stores a key-value pair with TTL
func (m *InMemoryStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ttl == 0 {
		ttl = m.defaultTTL
	}

	m.data[key] = valueWithExpiry{
		value:  value,
		expiry: time.Now().Add(ttl),
	}

	return nil
}

// Get retrieves a value by key
func (m *InMemoryStore) Get(ctx context.Context, key string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	// Check if expired
	if time.Now().After(entry.expiry) {
		delete(m.data, key)
		return nil, fmt.Errorf("key expired: %s", key)
	}

	return entry.value, nil
}

// Delete removes a key
func (m *InMemoryStore) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
	return nil
}

// Exists checks if a key exists
func (m *InMemoryStore) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.data[key]
	if !exists {
		return false, nil
	}

	// Check if expired
	if time.Now().After(entry.expiry) {
		delete(m.data, key)
		return false, nil
	}

	return true, nil
}

// SetTTL sets the default TTL
func (m *InMemoryStore) SetTTL(ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultTTL = ttl
}
