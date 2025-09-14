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

	registry := &RedisRegistry{
		client:    client,
		namespace: namespace,
		ttl:       30 * time.Second,
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

	if r.logger != nil {
		r.logger.Debug("Storing service data in Redis", map[string]interface{}{
			"service_id": info.ID,
			"key":        key,
			"data_size":  len(data),
			"ttl":        r.ttl.String(),
		})
	}

	// Store service data with TTL
	if err := r.client.Set(ctx, key, data, r.ttl).Err(); err != nil {
		if r.logger != nil {
			r.logger.Error("Failed to store service in Redis", map[string]interface{}{
				"error":        err,
				"error_type":   fmt.Sprintf("%T", err),
				"service_id":   info.ID,
				"key":          key,
				"ttl":          r.ttl.String(),
			})
		}
		return fmt.Errorf("failed to register service %s: %w", info.ID, err)
	}

	// Add to capability indexes
	if r.logger != nil && len(info.Capabilities) > 0 {
		r.logger.Debug("Adding service to capability indexes", map[string]interface{}{
			"service_id":         info.ID,
			"capabilities_count": len(info.Capabilities),
		})
	}

	for _, capability := range info.Capabilities {
		capKey := fmt.Sprintf("%s:capabilities:%s", r.namespace, capability.Name)
		if err := r.client.SAdd(ctx, capKey, info.ID).Err(); err != nil {
			// Log but don't fail
			if r.logger != nil {
				r.logger.Warn("Failed to add to capability index", map[string]interface{}{
					"capability":   capability.Name,
					"service_id":   info.ID,
					"capability_key": capKey,
					"error":        err,
					"error_type":   fmt.Sprintf("%T", err),
				})
			}
			continue
		}
		// Set expiry on capability set
		if err := r.client.Expire(ctx, capKey, r.ttl*2).Err(); err != nil && r.logger != nil {
			r.logger.Warn("Failed to set capability index expiry", map[string]interface{}{
				"capability":     capability.Name,
				"capability_key": capKey,
				"error":          err,
				"error_type":     fmt.Sprintf("%T", err),
			})
		}
		
		if r.logger != nil {
			r.logger.Debug("Added service to capability index", map[string]interface{}{
				"capability":     capability.Name,
				"capability_key": capKey,
				"service_id":     info.ID,
				"expiry":         (r.ttl * 2).String(),
			})
		}
	}

	// Add to service name index
	nameKey := fmt.Sprintf("%s:names:%s", r.namespace, info.Name)
	if err := r.client.SAdd(ctx, nameKey, info.ID).Err(); err != nil {
		if r.logger != nil {
			r.logger.Warn("Failed to add to name index", map[string]interface{}{
				"service_name": info.Name,
				"service_id":   info.ID,
				"name_key":     nameKey,
				"error":        err,
				"error_type":   fmt.Sprintf("%T", err),
			})
		}
	} else {
		if r.logger != nil {
			r.logger.Debug("Added service to name index", map[string]interface{}{
				"service_name": info.Name,
				"service_id":   info.ID,
				"name_key":     nameKey,
			})
		}
	}
	
	if err := r.client.Expire(ctx, nameKey, r.ttl*2).Err(); err != nil && r.logger != nil {
		r.logger.Warn("Failed to set name index expiry", map[string]interface{}{
			"name_key":   nameKey,
			"error":      err,
			"error_type": fmt.Sprintf("%T", err),
		})
	}

	// Add to type index
	typeKey := fmt.Sprintf("%s:types:%s", r.namespace, info.Type)
	if err := r.client.SAdd(ctx, typeKey, info.ID).Err(); err != nil {
		if r.logger != nil {
			r.logger.Warn("Failed to add to type index", map[string]interface{}{
				"service_type": info.Type,
				"service_id":   info.ID,
				"type_key":     typeKey,
				"error":        err,
				"error_type":   fmt.Sprintf("%T", err),
			})
		}
	} else {
		if r.logger != nil {
			r.logger.Debug("Added service to type index", map[string]interface{}{
				"service_type": info.Type,
				"service_id":   info.ID,
				"type_key":     typeKey,
			})
		}
	}
	
	if err := r.client.Expire(ctx, typeKey, r.ttl*2).Err(); err != nil && r.logger != nil {
		r.logger.Warn("Failed to set type index expiry", map[string]interface{}{
			"type_key":   typeKey,
			"error":      err,
			"error_type": fmt.Sprintf("%T", err),
		})
	}

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