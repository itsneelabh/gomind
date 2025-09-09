// Package core provides Redis client abstractions for the GoMind framework.
// This file implements a simplified Redis client wrapper with database isolation,
// namespacing, and connection management for various framework components.
//
// Purpose:
// - Provides unified Redis access across framework modules
// - Implements database isolation for different use cases
// - Supports key namespacing to prevent collisions
// - Offers simplified API for common Redis operations
// - Manages connection lifecycle and error handling
//
// Scope:
// - RedisClient: Wrapper around go-redis with added features
// - Database isolation using Redis DB selection (0-15)
// - Key namespacing for logical separation
// - Connection pooling and health checking
// - Common operations: Get, Set, Delete, Increment, Lists, Sets
//
// Database Allocation:
// The framework uses different Redis databases for isolation:
// - DB 0: Discovery and service registry
// - DB 1: Rate limiting
// - DB 2: Session management
// - DB 3: Circuit breaker state
// - DB 4-15: Available for extensions
//
// Namespacing:
// All keys are automatically prefixed with the namespace:
// - Discovery: "gomind:discovery:*"
// - Rate Limiting: "gomind:ratelimit:*"
// - Sessions: "gomind:sessions:*"
//
// Connection Management:
// - Automatic connection pooling
// - Connection health checks with Ping
// - Configurable timeouts
// - Graceful shutdown support
//
// Usage:
// client, err := NewRedisClient(RedisClientOptions{
//     RedisURL: "redis://localhost:6379",
//     DB: RedisDBRateLimiting,
//     Namespace: "gomind:ratelimit",
// })
// Used throughout the framework for distributed state management.
package core

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisClient provides a simplified Redis interface for modules with DB isolation
type RedisClient struct {
	client    *redis.Client
	dbID      int
	namespace string
	logger    Logger // Optional logger
}

// RedisClientOptions configures the Redis client
type RedisClientOptions struct {
	RedisURL  string
	DB        int    // Redis DB number for isolation (0-15)
	Namespace string // Key namespace for organization
	Logger    Logger // Optional logger
}

// NewRedisClient creates a new Redis client with specified options
func NewRedisClient(opts RedisClientOptions) (*RedisClient, error) {
	if opts.RedisURL == "" {
		return nil, fmt.Errorf("Redis URL is required: %w", ErrInvalidConfiguration)
	}

	// Parse Redis URL
	redisOpt, err := redis.ParseURL(opts.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL: %w", ErrInvalidConfiguration)
	}

	// Override DB for isolation
	if opts.DB >= 0 && opts.DB <= 15 {
		redisOpt.DB = opts.DB
	}

	client := redis.NewClient(redisOpt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis DB %d: %w", opts.DB, ErrConnectionFailed)
	}

	rc := &RedisClient{
		client:    client,
		dbID:      opts.DB,
		namespace: opts.Namespace,
		logger:    opts.Logger,
	}

	if rc.logger != nil {
		rc.logger.Info("Redis client connected", map[string]interface{}{
			"db":        opts.DB,
			"namespace": opts.Namespace,
		})
	}

	return rc, nil
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// GetDB returns the DB number being used
func (r *RedisClient) GetDB() int {
	return r.dbID
}

// GetNamespace returns the namespace being used
func (r *RedisClient) GetNamespace() string {
	return r.namespace
}

// formatKey formats a key with the namespace
func (r *RedisClient) formatKey(key string) string {
	if r.namespace != "" {
		return fmt.Sprintf("%s:%s", r.namespace, key)
	}
	return key
}

// --- Rate Limiting Operations ---

// Incr increments a counter
func (r *RedisClient) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, r.formatKey(key)).Result()
}

// IncrBy increments a counter by a specific amount
func (r *RedisClient) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return r.client.IncrBy(ctx, r.formatKey(key), value).Result()
}

// Expire sets a TTL on a key
func (r *RedisClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Expire(ctx, r.formatKey(key), ttl).Err()
}

// Get retrieves a value
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, r.formatKey(key)).Result()
}

// Set stores a value with optional TTL
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.client.Set(ctx, r.formatKey(key), value, ttl).Err()
}

// Del deletes keys
func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	formattedKeys := make([]string, len(keys))
	for i, key := range keys {
		formattedKeys[i] = r.formatKey(key)
	}
	return r.client.Del(ctx, formattedKeys...).Err()
}

// TTL gets the TTL of a key
func (r *RedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, r.formatKey(key)).Result()
}

// --- Sorted Set Operations (for sliding window) ---

// ZAdd adds members to a sorted set
func (r *RedisClient) ZAdd(ctx context.Context, key string, members ...*redis.Z) error {
	return r.client.ZAdd(ctx, r.formatKey(key), members...).Err()
}

// ZRemRangeByScore removes members by score range
func (r *RedisClient) ZRemRangeByScore(ctx context.Context, key string, min, max string) error {
	return r.client.ZRemRangeByScore(ctx, r.formatKey(key), min, max).Err()
}

// ZCard gets the cardinality of a sorted set
func (r *RedisClient) ZCard(ctx context.Context, key string) (int64, error) {
	return r.client.ZCard(ctx, r.formatKey(key)).Result()
}

// ZCount counts members in a score range
func (r *RedisClient) ZCount(ctx context.Context, key string, min, max string) (int64, error) {
	return r.client.ZCount(ctx, r.formatKey(key), min, max).Result()
}

// --- Pipeline Operations (for efficiency) ---

// Pipeline creates a pipeline for batched operations
func (r *RedisClient) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

// --- Health Check ---

// HealthCheck verifies Redis connectivity
func (r *RedisClient) HealthCheck(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// --- Standard Redis DB Allocation ---

const (
	// RedisDBServiceDiscovery is for service registry (default)
	RedisDBServiceDiscovery = 0
	
	// RedisDBRateLimiting is for rate limiting (isolated)
	RedisDBRateLimiting = 1
	
	// RedisDBSessions is for session storage
	RedisDBSessions = 2
	
	// RedisDBCache is for general caching
	RedisDBCache = 3
	
	// RedisDBCircuitBreaker is for circuit breaker state
	RedisDBCircuitBreaker = 4
	
	// RedisDBMetrics is for metrics buffering
	RedisDBMetrics = 5
	
	// RedisDBTelemetry is for telemetry data
	RedisDBTelemetry = 6
	
	// RedisDBReserved7 through RedisDBReserved15 are reserved for future use
	RedisDBReserved7  = 7
	RedisDBReserved8  = 8
	RedisDBReserved9  = 9
	RedisDBReserved10 = 10
	RedisDBReserved11 = 11
	RedisDBReserved12 = 12
	RedisDBReserved13 = 13
	RedisDBReserved14 = 14
	RedisDBReserved15 = 15
)

// GetRedisDBName returns a human-readable name for the Redis DB
func GetRedisDBName(db int) string {
	switch db {
	case RedisDBServiceDiscovery:
		return "Service Discovery"
	case RedisDBRateLimiting:
		return "Rate Limiting"
	case RedisDBSessions:
		return "Sessions"
	case RedisDBCache:
		return "Cache"
	case RedisDBCircuitBreaker:
		return "Circuit Breaker"
	case RedisDBMetrics:
		return "Metrics"
	case RedisDBTelemetry:
		return "Telemetry"
	default:
		return fmt.Sprintf("DB %d", db)
	}
}