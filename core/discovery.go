package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisDiscovery provides Redis-based service discovery
type RedisDiscovery struct {
	client    *redis.Client
	namespace string
	ttl       time.Duration
}

// NewRedisDiscovery creates a new Redis discovery client
func NewRedisDiscovery(redisURL string) (*RedisDiscovery, error) {
	return NewRedisDiscoveryWithNamespace(redisURL, "gomind")
}

// NewRedisDiscoveryWithNamespace creates a new Redis discovery client with custom namespace
func NewRedisDiscoveryWithNamespace(redisURL, namespace string) (*RedisDiscovery, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL: %w", err)
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisDiscovery{
		client:    client,
		namespace: namespace,
		ttl:       30 * time.Second,
	}, nil
}

// Register registers a service with discovery
func (d *RedisDiscovery) Register(ctx context.Context, registration *ServiceRegistration) error {
	key := fmt.Sprintf("%s:services:%s", d.namespace, registration.ID)

	data, err := json.Marshal(registration)
	if err != nil {
		return fmt.Errorf("failed to marshal registration: %w", err)
	}

	// Store service data with TTL
	if err := d.client.Set(ctx, key, data, d.ttl).Err(); err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// Add to capability indexes
	for _, capability := range registration.Capabilities {
		capKey := fmt.Sprintf("%s:capabilities:%s", d.namespace, capability)
		if err := d.client.SAdd(ctx, capKey, registration.ID).Err(); err != nil {
			// Log but don't fail
			continue
		}
		// Set expiry on capability set
		d.client.Expire(ctx, capKey, d.ttl*2)
	}

	// Add to service name index
	nameKey := fmt.Sprintf("%s:names:%s", d.namespace, registration.Name)
	if err := d.client.SAdd(ctx, nameKey, registration.ID).Err(); err != nil {
		// Log but don't fail
	}
	d.client.Expire(ctx, nameKey, d.ttl*2)

	return nil
}

// Unregister removes a service from discovery
func (d *RedisDiscovery) Unregister(ctx context.Context, serviceID string) error {
	key := fmt.Sprintf("%s:services:%s", d.namespace, serviceID)

	// Get service data first to clean up indexes
	data, err := d.client.Get(ctx, key).Result()
	if err == nil {
		var registration ServiceRegistration
		if json.Unmarshal([]byte(data), &registration) == nil {
			// Remove from capability indexes
			for _, capability := range registration.Capabilities {
				capKey := fmt.Sprintf("%s:capabilities:%s", d.namespace, capability)
				d.client.SRem(ctx, capKey, serviceID)
			}
			// Remove from name index
			nameKey := fmt.Sprintf("%s:names:%s", d.namespace, registration.Name)
			d.client.SRem(ctx, nameKey, serviceID)
		}
	}

	// Remove service data
	return d.client.Del(ctx, key).Err()
}

// FindService finds services by name
func (d *RedisDiscovery) FindService(ctx context.Context, serviceName string) ([]*ServiceRegistration, error) {
	nameKey := fmt.Sprintf("%s:names:%s", d.namespace, serviceName)

	// Get service IDs
	ids, err := d.client.SMembers(ctx, nameKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to find services: %w", err)
	}

	var services []*ServiceRegistration
	for _, id := range ids {
		key := fmt.Sprintf("%s:services:%s", d.namespace, id)
		data, err := d.client.Get(ctx, key).Result()
		if err != nil {
			continue // Service might have expired
		}

		var registration ServiceRegistration
		if err := json.Unmarshal([]byte(data), &registration); err != nil {
			continue
		}

		services = append(services, &registration)
	}

	return services, nil
}

// FindByCapability finds services by capability
func (d *RedisDiscovery) FindByCapability(ctx context.Context, capability string) ([]*ServiceRegistration, error) {
	capKey := fmt.Sprintf("%s:capabilities:%s", d.namespace, capability)

	// Get service IDs
	ids, err := d.client.SMembers(ctx, capKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to find services by capability: %w", err)
	}

	var services []*ServiceRegistration
	for _, id := range ids {
		key := fmt.Sprintf("%s:services:%s", d.namespace, id)
		data, err := d.client.Get(ctx, key).Result()
		if err != nil {
			continue // Service might have expired
		}

		var registration ServiceRegistration
		if err := json.Unmarshal([]byte(data), &registration); err != nil {
			continue
		}

		services = append(services, &registration)
	}

	return services, nil
}

// UpdateHealth updates service health status
func (d *RedisDiscovery) UpdateHealth(ctx context.Context, serviceID string, status HealthStatus) error {
	key := fmt.Sprintf("%s:services:%s", d.namespace, serviceID)

	// Get current registration
	data, err := d.client.Get(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("service not found: %w", err)
	}

	var registration ServiceRegistration
	if err := json.Unmarshal([]byte(data), &registration); err != nil {
		return fmt.Errorf("failed to unmarshal registration: %w", err)
	}

	// Update health and timestamp
	registration.Health = status
	registration.LastSeen = time.Now()

	// Save updated registration
	updatedData, err := json.Marshal(registration)
	if err != nil {
		return fmt.Errorf("failed to marshal registration: %w", err)
	}

	return d.client.Set(ctx, key, updatedData, d.ttl).Err()
}

// StartHeartbeat starts a heartbeat goroutine to keep registration alive
func (d *RedisDiscovery) StartHeartbeat(ctx context.Context, serviceID string) {
	ticker := time.NewTicker(d.ttl / 2)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := d.UpdateHealth(ctx, serviceID, HealthHealthy); err != nil {
					// Log error but continue health check loop
					// Health check failures are expected in distributed systems
					continue
				}
			}
		}
	}()
}

// MockDiscovery provides an in-memory mock discovery for development/testing
type MockDiscovery struct {
	mu           sync.RWMutex
	services     map[string]*ServiceRegistration
	capabilities map[string][]string  // capability -> service IDs
	names        map[string][]string  // name -> service IDs
}

// NewMockDiscovery creates a new mock discovery client
func NewMockDiscovery() *MockDiscovery {
	return &MockDiscovery{
		services:     make(map[string]*ServiceRegistration),
		capabilities: make(map[string][]string),
		names:        make(map[string][]string),
	}
}

// Register registers a service with mock discovery
func (m *MockDiscovery) Register(ctx context.Context, registration *ServiceRegistration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Store service
	m.services[registration.ID] = registration

	// Update capability index
	for _, cap := range registration.Capabilities {
		if !contains(m.capabilities[cap], registration.ID) {
			m.capabilities[cap] = append(m.capabilities[cap], registration.ID)
		}
	}

	// Update name index
	if !contains(m.names[registration.Name], registration.ID) {
		m.names[registration.Name] = append(m.names[registration.Name], registration.ID)
	}

	return nil
}

// Unregister removes a service from mock discovery
func (m *MockDiscovery) Unregister(ctx context.Context, serviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get service to clean up indexes
	registration, exists := m.services[serviceID]
	if !exists {
		return nil
	}

	// Remove from capability index
	for _, cap := range registration.Capabilities {
		m.capabilities[cap] = removeString(m.capabilities[cap], serviceID)
	}

	// Remove from name index
	m.names[registration.Name] = removeString(m.names[registration.Name], serviceID)

	// Remove service
	delete(m.services, serviceID)

	return nil
}

// FindService finds services by name in mock discovery
func (m *MockDiscovery) FindService(ctx context.Context, serviceName string) ([]*ServiceRegistration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.names[serviceName]
	var services []*ServiceRegistration

	for _, id := range ids {
		if service, exists := m.services[id]; exists {
			// Create a copy to avoid race conditions
			serviceCopy := *service
			services = append(services, &serviceCopy)
		}
	}

	return services, nil
}

// FindByCapability finds services by capability in mock discovery
func (m *MockDiscovery) FindByCapability(ctx context.Context, capability string) ([]*ServiceRegistration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.capabilities[capability]
	var services []*ServiceRegistration

	for _, id := range ids {
		if service, exists := m.services[id]; exists {
			// Create a copy to avoid race conditions
			serviceCopy := *service
			services = append(services, &serviceCopy)
		}
	}

	return services, nil
}

// UpdateHealth updates service health status in mock discovery
func (m *MockDiscovery) UpdateHealth(ctx context.Context, serviceID string, status HealthStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if service, exists := m.services[serviceID]; exists {
		service.Health = status
		service.LastSeen = time.Now()
	}

	return nil
}

// Helper functions for mock discovery
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func removeString(slice []string, item string) []string {
	var result []string
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}
