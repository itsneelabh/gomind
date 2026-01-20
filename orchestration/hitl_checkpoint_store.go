package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// =============================================================================
// RedisCheckpointStore - Reference Implementation
// =============================================================================
//
// RedisCheckpointStore implements CheckpointStore using Redis.
// This is the framework's reference implementation for production use.
// Applications can implement their own store (PostgreSQL, S3, DynamoDB, etc.)
//
// Key format:
//   - Checkpoint: {prefix}:checkpoint:{checkpoint_id}
//   - Pending index: {prefix}:pending (Redis Set)
//   - Request index: {prefix}:request:{request_id} (Redis Set)
//
// Per FRAMEWORK_DESIGN_PRINCIPLES.md, optional dependencies use NoOp defaults.
//
// Usage:
//
//	store := NewRedisCheckpointStore(
//	    WithCheckpointRedisURL("redis://localhost:6379"),
//	    WithCheckpointTTL(24 * time.Hour),
//	    WithCheckpointLogger(logger),
//	)
//
// =============================================================================

// RedisCheckpointStore implements CheckpointStore using Redis.
type RedisCheckpointStore struct {
	client    *redis.Client
	keyPrefix string
	ttl       time.Duration
	redisURL  string // For error messages

	// Optional dependencies (injected per framework patterns)
	logger    core.Logger    // Defaults to NoOp
	telemetry core.Telemetry // Defaults to NoOp
}

// redisCheckpointConfig holds configuration for the checkpoint store
type redisCheckpointConfig struct {
	redisURL  string
	redisDB   int
	keyPrefix string
	ttl       time.Duration
	logger    core.Logger
	telemetry core.Telemetry
}

// RedisCheckpointStoreOption configures the checkpoint store
type RedisCheckpointStoreOption func(*redisCheckpointConfig)

// WithCheckpointRedisURL sets the Redis connection URL
func WithCheckpointRedisURL(url string) RedisCheckpointStoreOption {
	return func(c *redisCheckpointConfig) {
		c.redisURL = url
	}
}

// WithCheckpointRedisDB sets the Redis database number
func WithCheckpointRedisDB(db int) RedisCheckpointStoreOption {
	return func(c *redisCheckpointConfig) {
		c.redisDB = db
	}
}

// WithCheckpointKeyPrefix sets the key prefix for checkpoint storage
func WithCheckpointKeyPrefix(prefix string) RedisCheckpointStoreOption {
	return func(c *redisCheckpointConfig) {
		c.keyPrefix = prefix
	}
}

// WithCheckpointTTL sets the TTL for checkpoint storage
func WithCheckpointTTL(ttl time.Duration) RedisCheckpointStoreOption {
	return func(c *redisCheckpointConfig) {
		c.ttl = ttl
	}
}

// NewRedisCheckpointStore creates a new Redis-backed checkpoint store.
// Returns concrete type per Go idiom "return structs, accept interfaces".
//
// Configuration priority:
//  1. Explicit option (e.g., WithCheckpointRedisURL)
//  2. Environment variable (REDIS_URL, GOMIND_HITL_REDIS_DB)
//  3. Default value
func NewRedisCheckpointStore(opts ...interface{}) (*RedisCheckpointStore, error) {
	// Build key prefix with optional agent name for multi-agent isolation
	// Format: {base_prefix}:{agent_name} or just {base_prefix} if no agent name
	basePrefix := getEnvOrDefault("GOMIND_HITL_KEY_PREFIX", "gomind:hitl")
	agentName := getEnvOrDefault("GOMIND_AGENT_NAME", "")
	keyPrefix := basePrefix
	if agentName != "" {
		keyPrefix = fmt.Sprintf("%s:%s", basePrefix, agentName)
	}

	// Initialize config with defaults
	config := &redisCheckpointConfig{
		redisURL:  getEnvOrDefault("REDIS_URL", "redis://localhost:6379"),
		redisDB:   getEnvIntOrDefault("GOMIND_HITL_REDIS_DB", 6), // Default to DB 6 for HITL
		keyPrefix: keyPrefix,
		ttl:       24 * time.Hour,
		logger:    &core.NoOpLogger{},
	}

	// Apply options - handle both old-style and new-style options
	for _, opt := range opts {
		switch o := opt.(type) {
		case RedisCheckpointStoreOption:
			o(config)
		case CheckpointStoreOption:
			// Create a temporary store to apply the option, then copy values
			// This maintains backward compatibility with the option functions defined in hitl_interfaces.go
			tempStore := &RedisCheckpointStore{}
			o(tempStore)
			if tempStore.logger != nil {
				config.logger = tempStore.logger
			}
			if tempStore.telemetry != nil {
				config.telemetry = tempStore.telemetry
			}
		}
	}

	// Parse Redis URL and create options
	redisOpts, err := redis.ParseURL(config.redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL %s: %w (check REDIS_URL environment variable)", config.redisURL, err)
	}
	redisOpts.DB = config.redisDB

	// Create Redis client
	client := redis.NewClient(redisOpts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w (check REDIS_URL and Redis connectivity)", config.redisURL, err)
	}

	return &RedisCheckpointStore{
		client:    client,
		keyPrefix: config.keyPrefix,
		ttl:       config.ttl,
		redisURL:  config.redisURL,
		logger:    config.logger,
		telemetry: config.telemetry,
	}, nil
}

// -----------------------------------------------------------------------------
// CheckpointStore Implementation
// -----------------------------------------------------------------------------

// SaveCheckpoint persists execution state with trace correlation.
// Per gold standard: use operation field, logger nil check, RecordSpanError, telemetry.Counter.
func (s *RedisCheckpointStore) SaveCheckpoint(ctx context.Context, cp *ExecutionCheckpoint) error {
	key := fmt.Sprintf("%s:checkpoint:%s", s.keyPrefix, cp.CheckpointID)

	data, err := json.Marshal(cp)
	if err != nil {
		telemetry.RecordSpanError(ctx, err)
		return fmt.Errorf("failed to marshal checkpoint %s: %w (check checkpoint data for non-serializable fields)", cp.CheckpointID, err)
	}

	// Store checkpoint with TTL
	if err := s.client.Set(ctx, key, data, s.ttl).Err(); err != nil {
		telemetry.RecordSpanError(ctx, err)
		if s.logger != nil {
			s.logger.ErrorWithContext(ctx, "Failed to save checkpoint", map[string]interface{}{
				"operation":     "hitl_checkpoint_save",
				"checkpoint_id": cp.CheckpointID,
				"request_id":    cp.RequestID,
				"error":         err.Error(),
			})
		}
		return fmt.Errorf("failed to save checkpoint %s to Redis: %w (check REDIS_URL=%s and Redis connectivity)", cp.CheckpointID, err, s.redisURL)
	}

	// Add to pending index if status is pending
	if cp.Status == CheckpointStatusPending {
		indexKey := fmt.Sprintf("%s:pending", s.keyPrefix)
		if err := s.client.SAdd(ctx, indexKey, cp.CheckpointID).Err(); err != nil {
			telemetry.RecordSpanError(ctx, err)
			if s.logger != nil {
				s.logger.ErrorWithContext(ctx, "Failed to add to pending index", map[string]interface{}{
					"operation":     "hitl_pending_index_add",
					"checkpoint_id": cp.CheckpointID,
					"request_id":    cp.RequestID,
					"error":         err.Error(),
				})
			}
			return fmt.Errorf("failed to add checkpoint %s to pending index: %w (Redis SADD failed)", cp.CheckpointID, err)
		}
	}

	// Add to request index for lookup by request_id
	if cp.RequestID != "" {
		requestIndexKey := fmt.Sprintf("%s:request:%s", s.keyPrefix, cp.RequestID)
		if err := s.client.SAdd(ctx, requestIndexKey, cp.CheckpointID).Err(); err != nil {
			// Non-fatal - just log warning
			if s.logger != nil {
				s.logger.WarnWithContext(ctx, "Failed to add to request index", map[string]interface{}{
					"operation":     "hitl_request_index_add",
					"checkpoint_id": cp.CheckpointID,
					"request_id":    cp.RequestID,
					"error":         err.Error(),
				})
			}
		}
	}

	// Add span event for tracing visibility (per DISTRIBUTED_TRACING_GUIDE.md)
	telemetry.AddSpanEvent(ctx, "hitl.checkpoint.saved",
		attribute.String("request_id", cp.RequestID),
		attribute.String("checkpoint_id", cp.CheckpointID),
		attribute.String("status", string(cp.Status)),
	)

	// Record counter metric (per gold standard pattern in executor.go)
	if cp.Decision != nil {
		telemetry.Counter("orchestration.hitl.checkpoint_saved",
			"reason", string(cp.Decision.Reason),
			"module", telemetry.ModuleOrchestration,
		)
	}

	// Log success with operation field (per gold standard pattern)
	if s.logger != nil {
		s.logger.DebugWithContext(ctx, "Checkpoint saved", map[string]interface{}{
			"operation":       "hitl_checkpoint_save_complete",
			"checkpoint_id":   cp.CheckpointID,
			"request_id":      cp.RequestID,
			"status":          cp.Status,
			"interrupt_point": cp.InterruptPoint,
		})
	}

	return nil
}

// LoadCheckpoint retrieves a checkpoint with trace correlation.
// Per gold standard: use operation field, logger nil check, RecordSpanError.
func (s *RedisCheckpointStore) LoadCheckpoint(ctx context.Context, checkpointID string) (*ExecutionCheckpoint, error) {
	key := fmt.Sprintf("%s:checkpoint:%s", s.keyPrefix, checkpointID)

	data, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		if s.logger != nil {
			s.logger.DebugWithContext(ctx, "Checkpoint not found", map[string]interface{}{
				"operation":     "hitl_checkpoint_load",
				"checkpoint_id": checkpointID,
			})
		}
		return nil, &ErrCheckpointNotFound{CheckpointID: checkpointID}
	}
	if err != nil {
		telemetry.RecordSpanError(ctx, err)
		if s.logger != nil {
			s.logger.ErrorWithContext(ctx, "Failed to load checkpoint", map[string]interface{}{
				"operation":     "hitl_checkpoint_load",
				"checkpoint_id": checkpointID,
				"error":         err.Error(),
			})
		}
		return nil, fmt.Errorf("failed to load checkpoint %s from Redis: %w (check REDIS_URL=%s)", checkpointID, err, s.redisURL)
	}

	var cp ExecutionCheckpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		telemetry.RecordSpanError(ctx, err)
		if s.logger != nil {
			s.logger.ErrorWithContext(ctx, "Failed to unmarshal checkpoint", map[string]interface{}{
				"operation":     "hitl_checkpoint_load",
				"checkpoint_id": checkpointID,
				"error":         err.Error(),
			})
		}
		return nil, fmt.Errorf("failed to unmarshal checkpoint %s: %w (checkpoint data may be corrupted)", checkpointID, err)
	}

	// Add span event for successful load (request_id first per gold standard)
	telemetry.AddSpanEvent(ctx, "hitl.checkpoint.loaded",
		attribute.String("request_id", cp.RequestID),
		attribute.String("checkpoint_id", checkpointID),
		attribute.String("status", string(cp.Status)),
	)

	// Log successful load with operation field
	if s.logger != nil {
		s.logger.DebugWithContext(ctx, "Checkpoint loaded", map[string]interface{}{
			"operation":     "hitl_checkpoint_load_complete",
			"checkpoint_id": checkpointID,
			"request_id":    cp.RequestID,
			"status":        cp.Status,
		})
	}

	return &cp, nil
}

// UpdateCheckpointStatus updates the status of a checkpoint.
func (s *RedisCheckpointStore) UpdateCheckpointStatus(ctx context.Context, checkpointID string, status CheckpointStatus) error {
	// Load existing checkpoint
	cp, err := s.LoadCheckpoint(ctx, checkpointID)
	if err != nil {
		return err
	}

	oldStatus := cp.Status
	cp.Status = status

	// Remove from pending index if no longer pending
	if oldStatus == CheckpointStatusPending && status != CheckpointStatusPending {
		indexKey := fmt.Sprintf("%s:pending", s.keyPrefix)
		if err := s.client.SRem(ctx, indexKey, checkpointID).Err(); err != nil {
			if s.logger != nil {
				s.logger.WarnWithContext(ctx, "Failed to remove from pending index", map[string]interface{}{
					"operation":     "hitl_pending_index_remove",
					"checkpoint_id": checkpointID,
					"error":         err.Error(),
				})
			}
		}
	}

	// Save updated checkpoint
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		return err
	}

	// Log status change
	if s.logger != nil {
		s.logger.InfoWithContext(ctx, "Checkpoint status updated", map[string]interface{}{
			"operation":     "hitl_checkpoint_status_update",
			"checkpoint_id": checkpointID,
			"request_id":    cp.RequestID,
			"old_status":    oldStatus,
			"new_status":    status,
		})
	}

	return nil
}

// ListPendingCheckpoints returns checkpoints awaiting human response.
func (s *RedisCheckpointStore) ListPendingCheckpoints(ctx context.Context, filter CheckpointFilter) ([]*ExecutionCheckpoint, error) {
	indexKey := fmt.Sprintf("%s:pending", s.keyPrefix)

	// Get all pending checkpoint IDs
	ids, err := s.client.SMembers(ctx, indexKey).Result()
	if err != nil {
		telemetry.RecordSpanError(ctx, err)
		return nil, fmt.Errorf("failed to list pending checkpoints: %w", err)
	}

	checkpoints := make([]*ExecutionCheckpoint, 0, len(ids))

	// Apply offset
	start := filter.Offset
	if start >= len(ids) {
		return checkpoints, nil
	}

	// Apply limit
	end := len(ids)
	if filter.Limit > 0 && start+filter.Limit < end {
		end = start + filter.Limit
	}

	// Load each checkpoint
	for _, id := range ids[start:end] {
		cp, err := s.LoadCheckpoint(ctx, id)
		if err != nil {
			if IsCheckpointNotFound(err) {
				// Checkpoint expired or deleted - remove from index
				s.client.SRem(ctx, indexKey, id)
				continue
			}
			// Log error but continue
			if s.logger != nil {
				s.logger.WarnWithContext(ctx, "Failed to load checkpoint from pending list", map[string]interface{}{
					"operation":     "hitl_list_pending",
					"checkpoint_id": id,
					"error":         err.Error(),
				})
			}
			continue
		}

		// Apply request_id filter if specified
		if filter.RequestID != "" && cp.RequestID != filter.RequestID {
			continue
		}

		// Apply status filter if specified (though pending index should only have pending)
		if filter.Status != "" && cp.Status != filter.Status {
			continue
		}

		checkpoints = append(checkpoints, cp)
	}

	return checkpoints, nil
}

// DeleteCheckpoint removes a checkpoint after completion.
func (s *RedisCheckpointStore) DeleteCheckpoint(ctx context.Context, checkpointID string) error {
	// Load checkpoint to get request_id for index cleanup
	cp, err := s.LoadCheckpoint(ctx, checkpointID)
	if err != nil && !IsCheckpointNotFound(err) {
		return err
	}

	key := fmt.Sprintf("%s:checkpoint:%s", s.keyPrefix, checkpointID)

	// Delete checkpoint
	if err := s.client.Del(ctx, key).Err(); err != nil {
		telemetry.RecordSpanError(ctx, err)
		return fmt.Errorf("failed to delete checkpoint %s: %w", checkpointID, err)
	}

	// Remove from pending index
	pendingKey := fmt.Sprintf("%s:pending", s.keyPrefix)
	s.client.SRem(ctx, pendingKey, checkpointID)

	// Remove from request index if we have the request_id
	if cp != nil && cp.RequestID != "" {
		requestIndexKey := fmt.Sprintf("%s:request:%s", s.keyPrefix, cp.RequestID)
		s.client.SRem(ctx, requestIndexKey, checkpointID)
	}

	// Add span event
	telemetry.AddSpanEvent(ctx, "hitl.checkpoint.deleted",
		attribute.String("checkpoint_id", checkpointID),
	)

	if s.logger != nil {
		s.logger.DebugWithContext(ctx, "Checkpoint deleted", map[string]interface{}{
			"operation":     "hitl_checkpoint_delete",
			"checkpoint_id": checkpointID,
		})
	}

	return nil
}

// -----------------------------------------------------------------------------
// Helper functions
// -----------------------------------------------------------------------------

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := parseInt(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

// Close closes the Redis connection
func (s *RedisCheckpointStore) Close() error {
	return s.client.Close()
}

// =============================================================================
// InMemoryCheckpointStore - For Testing
// =============================================================================

// InMemoryCheckpointStore implements CheckpointStore in memory for testing.
type InMemoryCheckpointStore struct {
	checkpoints map[string]*ExecutionCheckpoint
	pending     map[string]bool
}

// NewInMemoryCheckpointStore creates a new in-memory checkpoint store.
func NewInMemoryCheckpointStore() *InMemoryCheckpointStore {
	return &InMemoryCheckpointStore{
		checkpoints: make(map[string]*ExecutionCheckpoint),
		pending:     make(map[string]bool),
	}
}

// SaveCheckpoint saves a checkpoint in memory
func (s *InMemoryCheckpointStore) SaveCheckpoint(ctx context.Context, cp *ExecutionCheckpoint) error {
	// Deep copy to avoid mutation issues
	data, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint for deep copy: %w", err)
	}
	var copy ExecutionCheckpoint
	if err := json.Unmarshal(data, &copy); err != nil {
		return fmt.Errorf("failed to unmarshal checkpoint for deep copy: %w", err)
	}

	s.checkpoints[cp.CheckpointID] = &copy
	if cp.Status == CheckpointStatusPending {
		s.pending[cp.CheckpointID] = true
	} else {
		delete(s.pending, cp.CheckpointID)
	}
	return nil
}

// LoadCheckpoint loads a checkpoint from memory
func (s *InMemoryCheckpointStore) LoadCheckpoint(ctx context.Context, checkpointID string) (*ExecutionCheckpoint, error) {
	cp, ok := s.checkpoints[checkpointID]
	if !ok {
		return nil, &ErrCheckpointNotFound{CheckpointID: checkpointID}
	}

	// Deep copy to avoid mutation issues
	data, err := json.Marshal(cp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal checkpoint for deep copy: %w", err)
	}
	var copy ExecutionCheckpoint
	if err := json.Unmarshal(data, &copy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkpoint for deep copy: %w", err)
	}
	return &copy, nil
}

// UpdateCheckpointStatus updates checkpoint status in memory
func (s *InMemoryCheckpointStore) UpdateCheckpointStatus(ctx context.Context, checkpointID string, status CheckpointStatus) error {
	cp, ok := s.checkpoints[checkpointID]
	if !ok {
		return &ErrCheckpointNotFound{CheckpointID: checkpointID}
	}

	cp.Status = status
	if status != CheckpointStatusPending {
		delete(s.pending, checkpointID)
	}
	return nil
}

// ListPendingCheckpoints returns pending checkpoints from memory
func (s *InMemoryCheckpointStore) ListPendingCheckpoints(ctx context.Context, filter CheckpointFilter) ([]*ExecutionCheckpoint, error) {
	var result []*ExecutionCheckpoint
	for id := range s.pending {
		if cp, ok := s.checkpoints[id]; ok {
			if filter.RequestID != "" && cp.RequestID != filter.RequestID {
				continue
			}
			result = append(result, cp)
		}
	}
	return result, nil
}

// DeleteCheckpoint deletes a checkpoint from memory
func (s *InMemoryCheckpointStore) DeleteCheckpoint(ctx context.Context, checkpointID string) error {
	delete(s.checkpoints, checkpointID)
	delete(s.pending, checkpointID)
	return nil
}

// Compile-time interface compliance checks
var (
	_ CheckpointStore = (*RedisCheckpointStore)(nil)
	_ CheckpointStore = (*InMemoryCheckpointStore)(nil)
)
