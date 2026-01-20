package orchestration

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
)

const (
	// Redis key patterns
	llmDebugKeyPrefix = "gomind:llm:debug:"
	llmDebugIndexKey  = "gomind:llm:debug:index"

	// Size thresholds for compression
	compressionThreshold = 100 * 1024  // 100KB
	maxPayloadSize       = 1024 * 1024 // 1MB

	// Default TTLs
	defaultDebugTTL = 24 * time.Hour
	errorDebugTTL   = 7 * 24 * time.Hour
)

// RedisLLMDebugStoreOption configures the Redis debug store
type RedisLLMDebugStoreOption func(*redisDebugStoreConfig)

type redisDebugStoreConfig struct {
	redisURL       string
	redisDB        int
	logger         core.Logger
	circuitBreaker core.CircuitBreaker // Interface - injected by application (optional)
	ttl            time.Duration
	errorTTL       time.Duration
}

// WithDebugRedisURL sets the Redis connection URL
func WithDebugRedisURL(url string) RedisLLMDebugStoreOption {
	return func(c *redisDebugStoreConfig) {
		c.redisURL = url
	}
}

// WithDebugRedisDB sets the Redis database number (default: 7)
func WithDebugRedisDB(db int) RedisLLMDebugStoreOption {
	return func(c *redisDebugStoreConfig) {
		c.redisDB = db
	}
}

// WithDebugLogger sets the logger for debug store operations
func WithDebugLogger(logger core.Logger) RedisLLMDebugStoreOption {
	return func(c *redisDebugStoreConfig) {
		c.logger = logger
	}
}

// WithDebugCircuitBreaker sets a circuit breaker for Redis operations.
// The circuit breaker must implement core.CircuitBreaker interface.
// If not provided, built-in Layer 1 resilience (simple retry with backoff) is used.
// This follows ARCHITECTURE.md: circuit breaker is injected by application, not created internally.
func WithDebugCircuitBreaker(cb core.CircuitBreaker) RedisLLMDebugStoreOption {
	return func(c *redisDebugStoreConfig) {
		c.circuitBreaker = cb
	}
}

// WithDebugTTL sets custom TTL for successful debug records
func WithDebugTTL(ttl time.Duration) RedisLLMDebugStoreOption {
	return func(c *redisDebugStoreConfig) {
		c.ttl = ttl
	}
}

// WithDebugErrorTTL sets custom TTL for error debug records
func WithDebugErrorTTL(ttl time.Duration) RedisLLMDebugStoreOption {
	return func(c *redisDebugStoreConfig) {
		c.errorTTL = ttl
	}
}

// RedisLLMDebugStore is a Redis-backed implementation of LLMDebugStore.
// It provides persistent storage with TTL-based cleanup, compression for large payloads,
// and resilience protection.
//
// Resilience follows the Three-Layer Architecture from ARCHITECTURE.md:
// - Layer 1: Built-in simple retry with exponential backoff (always active)
// - Layer 2: Optional circuit breaker (injected via WithDebugCircuitBreaker)
// - Layer 3: Fallback to NoOp on persistent failures (handled by factory)
type RedisLLMDebugStore struct {
	client         *redis.Client
	logger         core.Logger
	circuitBreaker core.CircuitBreaker // Optional - injected by application
	ttl            time.Duration
	errorTTL       time.Duration

	// Layer 1 resilience state (simple failure tracking)
	failureCount int
	failureMu    sync.Mutex
	lastFailure  time.Time
}

// NewRedisLLMDebugStore creates a Redis-backed debug store with intelligent defaults.
// Environment variable precedence: explicit options > REDIS_URL > GOMIND_REDIS_URL > localhost:6379
func NewRedisLLMDebugStore(opts ...RedisLLMDebugStoreOption) (*RedisLLMDebugStore, error) {
	// Apply intelligent defaults
	cfg := &redisDebugStoreConfig{
		redisURL: getRedisURLWithFallback(),
		redisDB:  getEnvInt("GOMIND_LLM_DEBUG_REDIS_DB", core.RedisDBLLMDebug),
		logger:   &core.NoOpLogger{},
		ttl:      getEnvDuration("GOMIND_LLM_DEBUG_TTL", defaultDebugTTL),
		errorTTL: getEnvDuration("GOMIND_LLM_DEBUG_ERROR_TTL", errorDebugTTL),
	}

	// Apply explicit options (override defaults)
	for _, opt := range opts {
		opt(cfg)
	}

	// Parse Redis URL and create client
	redisOpt, err := redis.ParseURL(cfg.redisURL)
	if err != nil {
		// Try treating it as a simple address if URL parsing fails
		redisOpt = &redis.Options{
			Addr: cfg.redisURL,
		}
	}
	redisOpt.DB = cfg.redisDB

	client := redis.NewClient(redisOpt)

	// Verify connection with actionable error message
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed at %s (DB %d): %w\n"+
			"Hint: Check REDIS_URL or GOMIND_REDIS_URL environment variables, "+
			"or use WithDebugRedisURL() option", cfg.redisURL, cfg.redisDB, err)
	}

	// Note: Circuit breaker is optional and injected by application (per ARCHITECTURE.md)
	// If not provided, built-in Layer 1 resilience (simple retry) is used

	cfg.logger.Info("Redis LLM debug store initialized", map[string]interface{}{
		"redis_addr":      redisOpt.Addr,
		"redis_db":        cfg.redisDB,
		"ttl":             cfg.ttl.String(),
		"error_ttl":       cfg.errorTTL.String(),
		"circuit_breaker": cfg.circuitBreaker != nil,
		"resilience":      "layer1_builtin", // Always has Layer 1
	})

	return &RedisLLMDebugStore{
		client:         client,
		logger:         cfg.logger,
		circuitBreaker: cfg.circuitBreaker,
		ttl:            cfg.ttl,
		errorTTL:       cfg.errorTTL,
	}, nil
}

// RecordInteraction appends an LLM interaction to the debug record.
// Uses Layer 2 circuit breaker if injected, otherwise falls back to Layer 1 simple retry.
func (s *RedisLLMDebugStore) RecordInteraction(ctx context.Context, requestID string, interaction LLMInteraction) error {
	operation := func() error {
		key := llmDebugKeyPrefix + requestID

		// Get or create record
		record, err := s.getOrCreateRecord(ctx, key, requestID)
		if err != nil {
			s.logger.Warn("Failed to get debug record, creating new", map[string]interface{}{
				"request_id": requestID,
				"error":      err.Error(),
			})
			// Capture original_request_id from baggage for HITL correlation
			originalRequestID := requestID
			if bag := telemetry.GetBaggage(ctx); bag != nil {
				if origID := bag["original_request_id"]; origID != "" {
					originalRequestID = origID
				}
			}
			// Create fresh record on error (don't fail the whole operation)
			record = &LLMDebugRecord{
				RequestID:         requestID,
				OriginalRequestID: originalRequestID,
				TraceID:           telemetry.GetTraceContext(ctx).TraceID,
				CreatedAt:         time.Now(),
				Interactions:      []LLMInteraction{},
				Metadata:          make(map[string]string),
			}
		}

		// Append interaction
		record.Interactions = append(record.Interactions, interaction)
		record.UpdatedAt = time.Now()

		// Serialize with optional compression
		data, err := s.serialize(record)
		if err != nil {
			return fmt.Errorf("serialization failed: %w", err)
		}

		// Determine TTL based on success/error
		ttl := s.ttl
		if !interaction.Success {
			ttl = s.errorTTL
		}

		// Store
		if err := s.client.Set(ctx, key, data, ttl).Err(); err != nil {
			return fmt.Errorf("redis set failed: %w", err)
		}

		// Update index for listing (sorted set by timestamp) - best effort
		if err := s.client.ZAdd(ctx, llmDebugIndexKey, &redis.Z{
			Score:  float64(record.CreatedAt.Unix()),
			Member: requestID,
		}).Err(); err != nil {
			s.logger.Warn("Failed to update debug index", map[string]interface{}{
				"request_id": requestID,
				"error":      err.Error(),
			})
			// Don't fail - index is for convenience, not critical
		}

		return nil
	}

	// Layer 2: Use injected circuit breaker if available
	if s.circuitBreaker != nil {
		return s.circuitBreaker.Execute(ctx, operation)
	}

	// Layer 1: Built-in simple retry with exponential backoff
	return s.executeWithRetry(ctx, operation)
}

// GetRecord retrieves the complete debug record for a request.
func (s *RedisLLMDebugStore) GetRecord(ctx context.Context, requestID string) (*LLMDebugRecord, error) {
	key := llmDebugKeyPrefix + requestID

	data, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("record not found: %s", requestID)
	}
	if err != nil {
		return nil, fmt.Errorf("redis get failed: %w", err)
	}

	return s.deserialize(data)
}

// SetMetadata adds metadata to an existing record.
// Uses Layer 2 circuit breaker if injected, otherwise falls back to Layer 1 simple retry.
func (s *RedisLLMDebugStore) SetMetadata(ctx context.Context, requestID string, key, value string) error {
	operation := func() error {
		redisKey := llmDebugKeyPrefix + requestID

		record, err := s.GetRecord(ctx, requestID)
		if err != nil {
			return err
		}

		if record.Metadata == nil {
			record.Metadata = make(map[string]string)
		}
		record.Metadata[key] = value
		record.UpdatedAt = time.Now()

		data, err := s.serialize(record)
		if err != nil {
			return err
		}

		// Get current TTL to preserve it
		ttl, err := s.client.TTL(ctx, redisKey).Result()
		if err != nil || ttl < 0 {
			ttl = s.ttl
		}

		return s.client.Set(ctx, redisKey, data, ttl).Err()
	}

	// Layer 2: Use injected circuit breaker if available
	if s.circuitBreaker != nil {
		return s.circuitBreaker.Execute(ctx, operation)
	}

	// Layer 1: Built-in simple retry with exponential backoff
	return s.executeWithRetry(ctx, operation)
}

// ExtendTTL extends retention for investigation.
func (s *RedisLLMDebugStore) ExtendTTL(ctx context.Context, requestID string, duration time.Duration) error {
	key := llmDebugKeyPrefix + requestID
	return s.client.Expire(ctx, key, duration).Err()
}

// ListRecent returns recent records ordered by creation time.
func (s *RedisLLMDebugStore) ListRecent(ctx context.Context, limit int) ([]LLMDebugRecordSummary, error) {
	// Get recent request IDs from sorted set (newest first)
	ids, err := s.client.ZRevRange(ctx, llmDebugIndexKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	summaries := make([]LLMDebugRecordSummary, 0, len(ids))
	for _, id := range ids {
		record, err := s.GetRecord(ctx, id)
		if err != nil {
			continue // Skip missing records (TTL expired)
		}

		totalTokens := 0
		hasErrors := false
		for _, interaction := range record.Interactions {
			totalTokens += interaction.TotalTokens
			if !interaction.Success {
				hasErrors = true
			}
		}

		summaries = append(summaries, LLMDebugRecordSummary{
			RequestID:         record.RequestID,
			OriginalRequestID: record.OriginalRequestID,
			TraceID:           record.TraceID,
			CreatedAt:         record.CreatedAt,
			InteractionCount:  len(record.Interactions),
			TotalTokens:       totalTokens,
			HasErrors:         hasErrors,
		})
	}

	return summaries, nil
}

// Close closes the Redis connection.
func (s *RedisLLMDebugStore) Close() error {
	return s.client.Close()
}

// Layer 1 Resilience Constants
const (
	layer1MaxRetries     = 3
	layer1InitialBackoff = 100 * time.Millisecond
	layer1MaxBackoff     = 2 * time.Second
	layer1FailureWindow  = 30 * time.Second
	layer1MaxFailures    = 5
)

// executeWithRetry implements Layer 1 built-in resilience with simple retry and exponential backoff.
// This is always available, even without an injected circuit breaker.
// Per ARCHITECTURE.md Layer 1: "3 retries with exponential backoff, simple failure tracking"
func (s *RedisLLMDebugStore) executeWithRetry(ctx context.Context, operation func() error) error {
	// Check if we're in cooldown due to too many failures
	s.failureMu.Lock()
	if s.failureCount >= layer1MaxFailures && time.Since(s.lastFailure) < layer1FailureWindow {
		s.failureMu.Unlock()
		s.logger.Warn("Layer 1 resilience: in cooldown period", map[string]interface{}{
			"failures":     s.failureCount,
			"cooldown_sec": layer1FailureWindow.Seconds(),
		})
		return fmt.Errorf("debug store in cooldown after %d failures", s.failureCount)
	}
	s.failureMu.Unlock()

	var lastErr error
	backoff := layer1InitialBackoff

	for attempt := 1; attempt <= layer1MaxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := operation()
		if err == nil {
			// Success - reset failure count
			s.failureMu.Lock()
			s.failureCount = 0
			s.failureMu.Unlock()
			return nil
		}

		lastErr = err
		s.logger.Warn("Layer 1 resilience: operation failed, retrying", map[string]interface{}{
			"attempt": attempt,
			"max":     layer1MaxRetries,
			"backoff": backoff.String(),
			"error":   err.Error(),
		})

		// Don't sleep on last attempt
		if attempt < layer1MaxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			// Exponential backoff with cap
			backoff *= 2
			if backoff > layer1MaxBackoff {
				backoff = layer1MaxBackoff
			}
		}
	}

	// All retries failed - track failure
	s.failureMu.Lock()
	s.failureCount++
	s.lastFailure = time.Now()
	s.failureMu.Unlock()

	return fmt.Errorf("operation failed after %d attempts: %w", layer1MaxRetries, lastErr)
}

// serialize with optional gzip compression
func (s *RedisLLMDebugStore) serialize(record *LLMDebugRecord) ([]byte, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}

	// Compress if over threshold
	if len(data) > compressionThreshold {
		var buf bytes.Buffer
		buf.WriteByte(1) // Compression flag
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(data); err != nil {
			return nil, err
		}
		if err := gz.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	// Prepend 0 byte to indicate no compression
	return append([]byte{0}, data...), nil
}

// deserialize with optional gzip decompression
func (s *RedisLLMDebugStore) deserialize(data []byte) (*LLMDebugRecord, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	var jsonData []byte
	if data[0] == 1 { // Compressed
		gz, err := gzip.NewReader(bytes.NewReader(data[1:]))
		if err != nil {
			return nil, err
		}
		defer func() { _ = gz.Close() }() // Error intentionally ignored for reader

		var buf bytes.Buffer
		if _, err := buf.ReadFrom(gz); err != nil {
			return nil, err
		}
		jsonData = buf.Bytes()
	} else {
		jsonData = data[1:]
	}

	var record LLMDebugRecord
	if err := json.Unmarshal(jsonData, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

// getOrCreateRecord retrieves existing record or creates a new one
func (s *RedisLLMDebugStore) getOrCreateRecord(ctx context.Context, key, requestID string) (*LLMDebugRecord, error) {
	data, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		// Capture original_request_id from baggage for HITL correlation.
		// For initial requests: original_request_id == requestID
		// For resume requests: original_request_id is the conversation's first requestID
		originalRequestID := requestID
		if bag := telemetry.GetBaggage(ctx); bag != nil {
			if origID := bag["original_request_id"]; origID != "" {
				originalRequestID = origID
			}
		}

		// Create new record
		return &LLMDebugRecord{
			RequestID:         requestID,
			OriginalRequestID: originalRequestID,
			TraceID:           telemetry.GetTraceContext(ctx).TraceID,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
			Interactions:      []LLMInteraction{},
			Metadata:          make(map[string]string),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return s.deserialize(data)
}

// Helper functions for environment variable parsing

// getRedisURLWithFallback returns Redis URL with environment variable precedence
func getRedisURLWithFallback() string {
	if url := os.Getenv("REDIS_URL"); url != "" {
		return url
	}
	if url := os.Getenv("GOMIND_REDIS_URL"); url != "" {
		return url
	}
	return "localhost:6379"
}

// getEnvInt parses an integer from environment variable with fallback
func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if result, err := strconv.Atoi(val); err == nil {
			return result
		}
	}
	return defaultVal
}

// getEnvDuration parses a duration from environment variable with fallback
func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if result, err := time.ParseDuration(val); err == nil {
			return result
		}
	}
	return defaultVal
}

// Ensure RedisLLMDebugStore implements LLMDebugStore
var _ LLMDebugStore = (*RedisLLMDebugStore)(nil)
