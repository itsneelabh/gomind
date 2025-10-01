package core

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// HeartbeatStats tracks heartbeat statistics for periodic summaries
type HeartbeatStats struct {
	SuccessCount  int64
	FailureCount  int64
	LastSuccess   time.Time
	LastFailure   time.Time
	StartedAt     time.Time
	LastSummaryAt time.Time // Track when we last logged summary
}

// RedisRegistry provides Redis-based service registration (implements Registry interface)
type RedisRegistry struct {
	client    *redis.Client
	namespace string
	ttl       time.Duration
	logger    Logger // Optional logger for better observability

	// Self-healing state management (internal enhancement)
	registrationState map[string]*ServiceInfo
	stateMutex       sync.RWMutex

	// Heartbeat tracking for periodic summaries
	heartbeatStats map[string]*HeartbeatStats
	heartbeatMutex sync.RWMutex
}

// NewRedisRegistry creates a new Redis registry client
func NewRedisRegistry(redisURL string) (*RedisRegistry, error) {
	return NewRedisRegistryWithNamespace(redisURL, "gomind")
}

// NewRedisRegistryWithNamespace creates a new Redis registry client with custom namespace
func NewRedisRegistryWithNamespace(redisURL, namespace string) (*RedisRegistry, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL: %w", ErrInvalidConfiguration)
	}

	// Enhanced: Production-grade connection settings (internal enhancement)
	opt.PoolSize = 10                            // Handle 10 concurrent operations
	opt.MinIdleConns = 5                         // Keep 5 connections warm
	opt.MaxRetries = 3                           // Retry failed operations 3 times
	opt.MinRetryBackoff = time.Millisecond * 100 // Start with 100ms delay
	opt.MaxRetryBackoff = time.Second * 1        // Cap delay at 1 second
	opt.DialTimeout = time.Second * 5            // 5s to establish connection
	opt.ReadTimeout = time.Second * 5            // 5s for read operations
	opt.WriteTimeout = time.Second * 5           // 5s for write operations
	opt.PoolTimeout = time.Second * 10           // 10s to get connection from pool

	client := redis.NewClient(opt)

	// Enhanced: Connection verification with retry (internal enhancement)
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = client.Ping(ctx).Err()
		cancel()

		if err == nil {
			break
		}

		if i < 2 { // Exponential backoff
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis after retries: %w", ErrConnectionFailed)
	}

	registry := &RedisRegistry{
		client:            client,
		namespace:         namespace,
		ttl:               30 * time.Second,
		registrationState: make(map[string]*ServiceInfo), // Enhanced: state storage
		stateMutex:        sync.RWMutex{},                // Enhanced: thread safety
		heartbeatStats:    make(map[string]*HeartbeatStats),
		heartbeatMutex:    sync.RWMutex{},
	}

	// Note: Logger will be set later via SetLogger method if needed

	return registry, nil
}

// Register registers a service with the registry (implements Registry interface)
func (r *RedisRegistry) Register(ctx context.Context, info *ServiceInfo) error {
	if r.logger != nil {
		r.logger.Info("Registering service", map[string]interface{}{
			"service_id":         info.ID,
			"service_name":       info.Name,
			"service_type":       info.Type,
			"capabilities_count": len(info.Capabilities),
			"address":            info.Address,
			"port":               info.Port,
			"ttl":                r.ttl.String(),
		})
	}

	// Store registration state for potential recovery (internal enhancement)
	r.storeRegistrationState(info)

	// Use atomic transactions (Issue #1 fix)
	pipe := r.client.TxPipeline()

	// Store main service data
	key := fmt.Sprintf("%s:services:%s", r.namespace, info.ID)
	data, err := json.Marshal(info)
	if err != nil {
		if r.logger != nil {
			r.logger.Error("Failed to marshal service info", map[string]interface{}{
				"error":        err,
				"error_type":   fmt.Sprintf("%T", err),
				"service_id":   info.ID,
				"service_name": info.Name,
			})
		}
		return fmt.Errorf("failed to marshal service info for %s: %w", info.ID, err)
	}
	pipe.Set(ctx, key, data, r.ttl)

	// Add to all indexes atomically
	for _, capability := range info.Capabilities {
		capKey := fmt.Sprintf("%s:capabilities:%s", r.namespace, capability.Name)
		pipe.SAdd(ctx, capKey, info.ID)
		pipe.Expire(ctx, capKey, r.ttl*2)
	}

	nameKey := fmt.Sprintf("%s:names:%s", r.namespace, info.Name)
	pipe.SAdd(ctx, nameKey, info.ID)
	pipe.Expire(ctx, nameKey, r.ttl*2)

	typeKey := fmt.Sprintf("%s:types:%s", r.namespace, info.Type)
	pipe.SAdd(ctx, typeKey, info.ID)
	pipe.Expire(ctx, typeKey, r.ttl*2)

	// Execute all operations atomically
	_, err = pipe.Exec(ctx)
	if err != nil {
		if r.logger != nil {
			r.logger.Error("Failed to register service atomically", map[string]interface{}{
				"error":        err,
				"error_type":   fmt.Sprintf("%T", err),
				"service_id":   info.ID,
				"service_name": info.Name,
			})
		}
		return fmt.Errorf("failed to register service atomically: %w", err)
	}

	if r.logger != nil {
		r.logger.Info("Service registered successfully", map[string]interface{}{
			"service_id":         info.ID,
			"service_name":       info.Name,
			"service_type":       info.Type,
			"capabilities_count": len(info.Capabilities),
		})
	}

	return nil
}

// UpdateHealth updates service health status  
func (r *RedisRegistry) UpdateHealth(ctx context.Context, serviceID string, status HealthStatus) error {
	if r.logger != nil {
		r.logger.Debug("Updating service health", map[string]interface{}{
			"service_id": serviceID,
			"status":     status,
		})
	}

	key := fmt.Sprintf("%s:services:%s", r.namespace, serviceID)

	// Get current registration
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			if r.logger != nil {
				r.logger.Warn("Service not found for health update", map[string]interface{}{
					"service_id": serviceID,
					"key":        key,
				})
			}
			return fmt.Errorf("service %s: %w", serviceID, ErrServiceNotFound)
		}
		if r.logger != nil {
			r.logger.Error("Failed to get service for health update", map[string]interface{}{
				"error":      err,
				"error_type": fmt.Sprintf("%T", err),
				"service_id": serviceID,
				"key":        key,
			})
		}
		return fmt.Errorf("failed to get service %s: %w", serviceID, err)
	}

	var info ServiceInfo
	if err := json.Unmarshal([]byte(data), &info); err != nil {
		if r.logger != nil {
			r.logger.Error("Failed to unmarshal service data for health update", map[string]interface{}{
				"error":      err,
				"error_type": fmt.Sprintf("%T", err),
				"service_id": serviceID,
				"key":        key,
				"data_size":  len(data),
			})
		}
		return fmt.Errorf("failed to unmarshal service data for %s: %w", serviceID, err)
	}

	// Update health and timestamp
	previousHealth := info.Health
	info.Health = status
	info.LastSeen = time.Now()

	// Marshal updated data
	updatedData, err := json.Marshal(info)
	if err != nil {
		if r.logger != nil {
			r.logger.Error("Failed to marshal health data", map[string]interface{}{
				"error":      err,
				"error_type": fmt.Sprintf("%T", err),
				"service_id": serviceID,
				"status":     status,
			})
		}
		return fmt.Errorf("failed to marshal health data for %s: %w", serviceID, err)
	}

	// Update with TTL
	if err := r.client.Set(ctx, key, updatedData, r.ttl).Err(); err != nil {
		if r.logger != nil {
			r.logger.Error("Failed to store health update", map[string]interface{}{
				"error":      err,
				"error_type": fmt.Sprintf("%T", err),
				"service_id": serviceID,
				"key":        key,
				"status":     status,
				"ttl":        r.ttl.String(),
			})
		}
		return fmt.Errorf("failed to update health for %s: %w", serviceID, err)
	}

	// Refresh index set TTLs to prevent healthy services from disappearing
	// This fixes the critical bug where services become undiscoverable after 60s
	// even when they're healthy and sending heartbeats
	r.refreshIndexSetTTLs(ctx, &info)

	if r.logger != nil {
		r.logger.Debug("Service health updated", map[string]interface{}{
			"service_id":      serviceID,
			"previous_status": previousHealth,
			"new_status":      status,
			"last_seen":       info.LastSeen,
			"ttl":             r.ttl.String(),
		})
	}

	return nil
}

// Unregister removes a service from the registry
func (r *RedisRegistry) Unregister(ctx context.Context, serviceID string) error {
	if r.logger != nil {
		r.logger.Info("Unregistering service", map[string]interface{}{
			"service_id": serviceID,
		})
	}

	key := fmt.Sprintf("%s:services:%s", r.namespace, serviceID)

	// Get service data to find capabilities
	data, err := r.client.Get(ctx, key).Result()
	if err == nil {
		var info ServiceInfo
		if err := json.Unmarshal([]byte(data), &info); err == nil {
			if r.logger != nil {
				r.logger.Debug("Removing service from indexes", map[string]interface{}{
					"service_id":         serviceID,
					"service_name":       info.Name,
					"service_type":       info.Type,
					"capabilities_count": len(info.Capabilities),
				})
			}
			
			// Remove from capability indexes
			for _, capability := range info.Capabilities {
				capKey := fmt.Sprintf("%s:capabilities:%s", r.namespace, capability.Name)
				if err := r.client.SRem(ctx, capKey, serviceID).Err(); err != nil && r.logger != nil {
					r.logger.Warn("Failed to remove from capability index", map[string]interface{}{
						"capability":     capability.Name,
						"capability_key": capKey,
						"service_id":     serviceID,
						"error":          err,
						"error_type":     fmt.Sprintf("%T", err),
					})
				}
			}
			// Remove from name index
			nameKey := fmt.Sprintf("%s:names:%s", r.namespace, info.Name)
			if err := r.client.SRem(ctx, nameKey, serviceID).Err(); err != nil && r.logger != nil {
				r.logger.Warn("Failed to remove from name index", map[string]interface{}{
					"name_key":   nameKey,
					"service_id": serviceID,
					"error":      err,
					"error_type": fmt.Sprintf("%T", err),
				})
			}
			// Remove from type index
			typeKey := fmt.Sprintf("%s:types:%s", r.namespace, info.Type)
			if err := r.client.SRem(ctx, typeKey, serviceID).Err(); err != nil && r.logger != nil {
				r.logger.Warn("Failed to remove from type index", map[string]interface{}{
					"type_key":   typeKey,
					"service_id": serviceID,
					"error":      err,
					"error_type": fmt.Sprintf("%T", err),
				})
			}
		} else {
			if r.logger != nil {
				r.logger.Warn("Failed to unmarshal service data for unregistration", map[string]interface{}{
					"error":      err,
					"error_type": fmt.Sprintf("%T", err),
					"service_id": serviceID,
					"key":        key,
					"data_size":  len(data),
				})
			}
		}
	} else if err != redis.Nil && r.logger != nil {
		r.logger.Warn("Failed to get service data for unregistration", map[string]interface{}{
			"error":      err,
			"error_type": fmt.Sprintf("%T", err),
			"service_id": serviceID,
			"key":        key,
		})
	}

	// Delete service key
	if err := r.client.Del(ctx, key).Err(); err != nil {
		if r.logger != nil {
			r.logger.Error("Failed to delete service key", map[string]interface{}{
				"error":      err,
				"error_type": fmt.Sprintf("%T", err),
				"service_id": serviceID,
				"key":        key,
			})
		}
		return fmt.Errorf("failed to unregister service %s: %w", serviceID, err)
	}

	// Clean up registration state (prevents memory leaks)
	r.stateMutex.Lock()
	delete(r.registrationState, serviceID)
	r.stateMutex.Unlock()

	if r.logger != nil {
		r.logger.Info("Service unregistered successfully", map[string]interface{}{
			"service_id": serviceID,
			"key":        key,
		})
	}

	return nil
}

// refreshIndexSetTTLs extends TTL for all index sets this service belongs to
// This prevents healthy services from becoming undiscoverable when index sets expire
// before the service keys. Called during heartbeat to keep index sets alive.
func (r *RedisRegistry) refreshIndexSetTTLs(ctx context.Context, info *ServiceInfo) {
	if r.logger != nil {
		r.logger.Debug("Refreshing index set TTLs", map[string]interface{}{
			"service_id":         info.ID,
			"service_name":       info.Name,
			"service_type":       info.Type,
			"capabilities_count": len(info.Capabilities),
			"ttl":                (r.ttl * 2).String(),
		})
	}
	
	// Refresh capability indexes
	for _, capability := range info.Capabilities {
		capKey := fmt.Sprintf("%s:capabilities:%s", r.namespace, capability.Name)
		if err := r.client.Expire(ctx, capKey, r.ttl*2).Err(); err != nil {
			if r.logger != nil {
				r.logger.Debug("Failed to refresh capability index TTL", map[string]interface{}{
					"capability":     capability.Name,
					"capability_key": capKey,
					"error":          err,
					"error_type":     fmt.Sprintf("%T", err),
				})
			}
			// Continue with other indexes even if one fails
		}
	}
	
	// Refresh name index
	nameKey := fmt.Sprintf("%s:names:%s", r.namespace, info.Name)
	if err := r.client.Expire(ctx, nameKey, r.ttl*2).Err(); err != nil {
		if r.logger != nil {
			r.logger.Debug("Failed to refresh name index TTL", map[string]interface{}{
				"name":       info.Name,
				"name_key":   nameKey,
				"error":      err,
				"error_type": fmt.Sprintf("%T", err),
			})
		}
	}
	
	// Refresh type index  
	typeKey := fmt.Sprintf("%s:types:%s", r.namespace, info.Type)
	if err := r.client.Expire(ctx, typeKey, r.ttl*2).Err(); err != nil {
		if r.logger != nil {
			r.logger.Debug("Failed to refresh type index TTL", map[string]interface{}{
				"type":       info.Type,
				"type_key":   typeKey,
				"error":      err,
				"error_type": fmt.Sprintf("%T", err),
			})
		}
	}
	
	if r.logger != nil {
		r.logger.Debug("Index set TTL refresh completed", map[string]interface{}{
			"service_id":   info.ID,
			"service_name": info.Name,
			"type":         info.Type,
		})
	}
}

// SetLogger sets the logger for the registry client
func (r *RedisRegistry) SetLogger(logger Logger) {
	r.logger = logger
}

// storeRegistrationState stores service info for potential re-registration (internal helper)
func (r *RedisRegistry) storeRegistrationState(serviceInfo *ServiceInfo) {
	r.stateMutex.Lock()
	defer r.stateMutex.Unlock()

	// Store minimal copy for re-registration
	r.registrationState[serviceInfo.ID] = &ServiceInfo{
		ID:           serviceInfo.ID,
		Name:         serviceInfo.Name,
		Type:         serviceInfo.Type,
		Address:      serviceInfo.Address,
		Port:         serviceInfo.Port,
		Capabilities: append([]Capability{}, serviceInfo.Capabilities...),
		Health:       HealthHealthy,
		Metadata:     copyMetadata(serviceInfo.Metadata),
	}
}

// getStoredRegistrationState retrieves stored service info for re-registration (internal helper)
func (r *RedisRegistry) getStoredRegistrationState(serviceID string) *ServiceInfo {
	r.stateMutex.RLock()
	defer r.stateMutex.RUnlock()

	if info, exists := r.registrationState[serviceID]; exists {
		// Return copy to prevent external modification
		return &ServiceInfo{
			ID:           info.ID,
			Name:         info.Name,
			Type:         info.Type,
			Address:      info.Address,
			Port:         info.Port,
			Capabilities: append([]Capability{}, info.Capabilities...),
			Health:       HealthHealthy,
			Metadata:     copyMetadata(info.Metadata),
		}
	}
	return nil
}

// copyMetadata creates a deep copy of metadata map (internal helper)
func copyMetadata(metadata map[string]interface{}) map[string]interface{} {
	if metadata == nil {
		return nil
	}
	result := make(map[string]interface{})
	for k, v := range metadata {
		result[k] = v
	}
	return result
}

// isServiceNotFoundError checks if error indicates service not found (internal helper)
func (r *RedisRegistry) isServiceNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// Handle UpdateHealth errors
	if err == redis.Nil {
		return true
	}

	// Handle atomic registration errors
	errorStr := err.Error()
	return strings.Contains(errorStr, "service not found") ||
		strings.Contains(errorStr, "key does not exist")
}

// maintainRegistration provides intelligent heartbeat with self-healing (internal helper)
func (r *RedisRegistry) maintainRegistration(ctx context.Context, serviceID string) {
	// Try lightweight health update first (normal operation)
	err := r.UpdateHealth(ctx, serviceID, HealthHealthy)

	// Update stats and capture values for logging (under lock for thread safety)
	var failureCount int64
	var lastSuccessTime time.Time

	r.heartbeatMutex.Lock()
	stats := r.heartbeatStats[serviceID]
	if stats != nil {
		if err == nil {
			stats.SuccessCount++
			stats.LastSuccess = time.Now()
		} else {
			stats.FailureCount++
			stats.LastFailure = time.Now()
		}
		// Capture values under lock for safe logging later
		failureCount = stats.FailureCount
		lastSuccessTime = stats.LastSuccess
	}
	r.heartbeatMutex.Unlock()

	if err != nil && r.isServiceNotFoundError(err) {
		// Service expired from Redis - attempt re-registration
		if serviceInfo := r.getStoredRegistrationState(serviceID); serviceInfo != nil {
			// Use jittered backoff to prevent thundering herd during recovery
			jitterMs, _ := rand.Int(rand.Reader, big.NewInt(1000))
			jitter := time.Duration(jitterMs.Int64()) * time.Millisecond
			time.Sleep(jitter)

			if regErr := r.Register(ctx, serviceInfo); regErr != nil {
				if r.logger != nil {
					r.logger.Error("Failed to re-register service during recovery", map[string]interface{}{
						"service_id":                serviceID,
						"error":                     regErr,
						"will_retry_next_heartbeat": true,
						"total_failures":            failureCount,
					})
				}
			} else {
				if r.logger != nil {
					downtime := time.Duration(0)
					if !lastSuccessTime.IsZero() {
						downtime = time.Since(lastSuccessTime)
					}
					r.logger.Info("Successfully re-registered service after Redis recovery", map[string]interface{}{
						"service_id":        serviceID,
						"downtime_seconds":  int(downtime.Seconds()),
						"missed_heartbeats": int(downtime.Seconds() / (r.ttl.Seconds() / 2)),
					})
				}
				// Reset failure count after successful recovery
				r.heartbeatMutex.Lock()
				if stats := r.heartbeatStats[serviceID]; stats != nil {
					stats.FailureCount = 0
					stats.LastSuccess = time.Now()
				}
				r.heartbeatMutex.Unlock()
			}
		}
	} else if err != nil && r.logger != nil {
		r.logger.Error("Failed to send heartbeat", map[string]interface{}{
			"service_id":     serviceID,
			"error":          err.Error(),
			"total_failures": failureCount,
		})
	}
}

// checkAndLogPeriodicSummary logs heartbeat health every 5 minutes
func (r *RedisRegistry) checkAndLogPeriodicSummary(serviceID string) {
	// Check if it's time to log (read LastSummaryAt under lock)
	r.heartbeatMutex.RLock()
	stats := r.heartbeatStats[serviceID]
	if stats == nil || r.logger == nil {
		r.heartbeatMutex.RUnlock()
		return
	}
	shouldLog := time.Since(stats.LastSummaryAt) >= 5*time.Minute
	r.heartbeatMutex.RUnlock()

	// Log summary if needed
	if shouldLog {
		r.logHeartbeatSummary(serviceID, false)

		// Update LastSummaryAt (re-check stats exists under lock)
		r.heartbeatMutex.Lock()
		if stats := r.heartbeatStats[serviceID]; stats != nil {
			stats.LastSummaryAt = time.Now()
		}
		r.heartbeatMutex.Unlock()
	}
}

// logHeartbeatSummary logs heartbeat statistics
func (r *RedisRegistry) logHeartbeatSummary(serviceID string, isFinal bool) {
	// Capture consistent snapshot of stats under RLock
	var snapshot struct {
		successCount int64
		failureCount int64
		lastSuccess  time.Time
		lastFailure  time.Time
		startedAt    time.Time
	}

	r.heartbeatMutex.RLock()
	stats := r.heartbeatStats[serviceID]
	if stats == nil {
		r.heartbeatMutex.RUnlock()
		return
	}
	// Copy all fields under lock for consistent snapshot
	snapshot.successCount = stats.SuccessCount
	snapshot.failureCount = stats.FailureCount
	snapshot.lastSuccess = stats.LastSuccess
	snapshot.lastFailure = stats.LastFailure
	snapshot.startedAt = stats.StartedAt
	r.heartbeatMutex.RUnlock()

	if r.logger == nil {
		return
	}

	// Get service info for better context (not protected by heartbeatMutex)
	serviceInfo := r.getStoredRegistrationState(serviceID)
	serviceName := "unknown"
	serviceType := "unknown"
	if serviceInfo != nil {
		serviceName = serviceInfo.Name
		serviceType = string(serviceInfo.Type)
	}

	// Calculate metrics from consistent snapshot
	uptime := time.Since(snapshot.startedAt)
	total := snapshot.successCount + snapshot.failureCount
	successRate := float64(0)
	if total > 0 {
		successRate = float64(snapshot.successCount) / float64(total) * 100
	}

	logData := map[string]interface{}{
		"service_id":     serviceID,
		"service_name":   serviceName,
		"service_type":   serviceType,
		"success_count":  snapshot.successCount,
		"failure_count":  snapshot.failureCount,
		"success_rate":   fmt.Sprintf("%.2f%%", successRate),
		"uptime_minutes": int(uptime.Minutes()),
	}

	// Only add timestamps if they're valid
	if !snapshot.lastSuccess.IsZero() {
		logData["time_since_last_success_sec"] = int(time.Since(snapshot.lastSuccess).Seconds())
	}

	if snapshot.failureCount > 0 && !snapshot.lastFailure.IsZero() {
		logData["time_since_last_failure_sec"] = int(time.Since(snapshot.lastFailure).Seconds())
	}

	if isFinal {
		r.logger.Info("Heartbeat final summary (service shutting down)", logData)
	} else {
		r.logger.Info("Heartbeat health summary", logData)
	}
}

// StartHeartbeat starts a heartbeat goroutine to keep registration alive (enhanced for self-healing)
func (r *RedisRegistry) StartHeartbeat(ctx context.Context, serviceID string) {
	// Initialize heartbeat stats
	r.heartbeatMutex.Lock()
	r.heartbeatStats[serviceID] = &HeartbeatStats{
		StartedAt:     time.Now(),
		LastSummaryAt: time.Now(),
	}
	r.heartbeatMutex.Unlock()

	// Base interval with jitter to distribute load
	baseInterval := r.ttl / 2
	maxJitter := int64(baseInterval.Milliseconds() / 4)
	jitterMs, _ := rand.Int(rand.Reader, big.NewInt(maxJitter))
	jitter := time.Duration(jitterMs.Int64()) * time.Millisecond
	interval := baseInterval + jitter

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				// Log final stats on shutdown
				r.logHeartbeatSummary(serviceID, true)
				// Clean up stats
				r.heartbeatMutex.Lock()
				delete(r.heartbeatStats, serviceID)
				r.heartbeatMutex.Unlock()
				return
			case <-ticker.C:
				// Use enhanced maintenance logic with self-healing
				r.maintainRegistration(ctx, serviceID)
				// Check if it's time for periodic summary (every 5 minutes)
				r.checkAndLogPeriodicSummary(serviceID)
			}
		}
	}()
}