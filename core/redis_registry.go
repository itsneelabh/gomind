package core

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisRegistry provides Redis-based service registration (implements Registry interface)
type RedisRegistry struct {
	client    *redis.Client
	namespace string
	ttl       time.Duration
	logger    Logger // Optional logger for better observability

	// Self-healing state management (internal enhancement)
	registrationState map[string]*ServiceInfo
	stateMutex       sync.RWMutex
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

	if err != nil && r.isServiceNotFoundError(err) {
		// Service expired from Redis - attempt re-registration
		if serviceInfo := r.getStoredRegistrationState(serviceID); serviceInfo != nil {
			// Use jittered backoff to prevent thundering herd during recovery
			jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
			time.Sleep(jitter)

			if regErr := r.Register(ctx, serviceInfo); regErr != nil {
				if r.logger != nil {
					r.logger.Error("Failed to re-register service during recovery", map[string]interface{}{
						"service_id":               serviceID,
						"error":                    regErr,
						"will_retry_next_heartbeat": true,
					})
				}
			} else {
				if r.logger != nil {
					r.logger.Info("Successfully re-registered service after Redis recovery", map[string]interface{}{
						"service_id": serviceID,
					})
				}
			}
		}
	} else if err != nil && r.logger != nil {
		r.logger.Error("Failed to send heartbeat", map[string]interface{}{
			"service_id": serviceID,
			"error":      err.Error(),
		})
	}
}

// StartHeartbeat starts a heartbeat goroutine to keep registration alive (enhanced for self-healing)
func (r *RedisRegistry) StartHeartbeat(ctx context.Context, serviceID string) {
	// Base interval with jitter to distribute load
	baseInterval := r.ttl / 2
	jitter := time.Duration(rand.Intn(int(baseInterval.Milliseconds()/4))) * time.Millisecond
	interval := baseInterval + jitter

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Use enhanced maintenance logic with self-healing
				r.maintainRegistration(ctx, serviceID)
			}
		}
	}()
}