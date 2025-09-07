package core

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisRegistry provides Redis-based service registration (implements Registry interface)
type RedisRegistry struct {
	client    *redis.Client
	namespace string
	ttl       time.Duration
	logger    Logger // Optional logger for better observability
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

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", ErrConnectionFailed)
	}

	return &RedisRegistry{
		client:    client,
		namespace: namespace,
		ttl:       30 * time.Second,
	}, nil
}

// Register registers a service with the registry (implements Registry interface)
func (r *RedisRegistry) Register(ctx context.Context, info *ServiceInfo) error {
	key := fmt.Sprintf("%s:services:%s", r.namespace, info.ID)

	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal service info for %s: %w", info.ID, err)
	}

	// Store service data with TTL
	if err := r.client.Set(ctx, key, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("failed to register service %s: %w", info.ID, err)
	}

	// Add to capability indexes
	for _, capability := range info.Capabilities {
		capKey := fmt.Sprintf("%s:capabilities:%s", r.namespace, capability.Name)
		if err := r.client.SAdd(ctx, capKey, info.ID).Err(); err != nil {
			// Log but don't fail
			if r.logger != nil {
				r.logger.Warn("Failed to add to capability index", map[string]interface{}{
					"capability": capability.Name,
					"service_id": info.ID,
					"error":      err.Error(),
				})
			}
			continue
		}
		// Set expiry on capability set
		r.client.Expire(ctx, capKey, r.ttl*2)
	}

	// Add to service name index
	nameKey := fmt.Sprintf("%s:names:%s", r.namespace, info.Name)
	_ = r.client.SAdd(ctx, nameKey, info.ID).Err()
	_ = r.client.Expire(ctx, nameKey, r.ttl*2)

	// Add to type index
	typeKey := fmt.Sprintf("%s:types:%s", r.namespace, info.Type)
	_ = r.client.SAdd(ctx, typeKey, info.ID).Err()
	_ = r.client.Expire(ctx, typeKey, r.ttl*2)

	if r.logger != nil {
		r.logger.Debug("Service registered", map[string]interface{}{
			"service_id":   info.ID,
			"service_name": info.Name,
			"type":         info.Type,
		})
	}

	return nil
}

// UpdateHealth updates service health status  
func (r *RedisRegistry) UpdateHealth(ctx context.Context, serviceID string, status HealthStatus) error {
	key := fmt.Sprintf("%s:services:%s", r.namespace, serviceID)

	// Get current registration
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("service %s: %w", serviceID, ErrServiceNotFound)
		}
		return fmt.Errorf("failed to get service %s: %w", serviceID, err)
	}

	var info ServiceInfo
	if err := json.Unmarshal([]byte(data), &info); err != nil {
		return fmt.Errorf("failed to unmarshal service data for %s: %w", serviceID, err)
	}

	// Update health and timestamp
	info.Health = status
	info.LastSeen = time.Now()

	// Marshal updated data
	updatedData, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal health data for %s: %w", serviceID, err)
	}

	// Update with TTL
	if err := r.client.Set(ctx, key, updatedData, r.ttl).Err(); err != nil {
		return fmt.Errorf("failed to update health for %s: %w", serviceID, err)
	}

	if r.logger != nil {
		r.logger.Debug("Service health updated", map[string]interface{}{
			"service_id": serviceID,
			"status":     status,
		})
	}

	return nil
}

// Unregister removes a service from the registry
func (r *RedisRegistry) Unregister(ctx context.Context, serviceID string) error {
	key := fmt.Sprintf("%s:services:%s", r.namespace, serviceID)

	// Get service data to find capabilities
	data, err := r.client.Get(ctx, key).Result()
	if err == nil {
		var info ServiceInfo
		if json.Unmarshal([]byte(data), &info) == nil {
			// Remove from capability indexes
			for _, capability := range info.Capabilities {
				capKey := fmt.Sprintf("%s:capabilities:%s", r.namespace, capability.Name)
				r.client.SRem(ctx, capKey, serviceID)
			}
			// Remove from name index
			nameKey := fmt.Sprintf("%s:names:%s", r.namespace, info.Name)
			r.client.SRem(ctx, nameKey, serviceID)
			// Remove from type index
			typeKey := fmt.Sprintf("%s:types:%s", r.namespace, info.Type)
			r.client.SRem(ctx, typeKey, serviceID)
		}
	}

	// Delete service key
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to unregister service %s: %w", serviceID, err)
	}

	if r.logger != nil {
		r.logger.Debug("Service unregistered", map[string]interface{}{
			"service_id": serviceID,
		})
	}

	return nil
}

// SetLogger sets the logger for the registry client
func (r *RedisRegistry) SetLogger(logger Logger) {
	r.logger = logger
}

// StartHeartbeat starts a heartbeat goroutine to keep registration alive
func (r *RedisRegistry) StartHeartbeat(ctx context.Context, serviceID string) {
	ticker := time.NewTicker(r.ttl / 2)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := r.UpdateHealth(ctx, serviceID, HealthHealthy); err != nil {
					if r.logger != nil {
						r.logger.Error("Failed to send heartbeat", map[string]interface{}{
							"service_id": serviceID,
							"error":      err.Error(),
						})
					}
				}
			}
		}
	}()
}