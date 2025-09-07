package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis/v8"
)

// RedisDiscovery provides Redis-based service discovery (implements Discovery interface)
// It embeds RedisRegistry and adds discovery capabilities
type RedisDiscovery struct {
	*RedisRegistry // Embed for registration capabilities
}

// NewRedisDiscovery creates a new Redis discovery client
func NewRedisDiscovery(redisURL string) (*RedisDiscovery, error) {
	return NewRedisDiscoveryWithNamespace(redisURL, "gomind")
}

// NewRedisDiscoveryWithNamespace creates a new Redis discovery client with custom namespace
func NewRedisDiscoveryWithNamespace(redisURL, namespace string) (*RedisDiscovery, error) {
	registry, err := NewRedisRegistryWithNamespace(redisURL, namespace)
	if err != nil {
		return nil, err
	}

	return &RedisDiscovery{
		RedisRegistry: registry,
	}, nil
}

// Discover finds services based on filter criteria (implements Discovery interface)
func (d *RedisDiscovery) Discover(ctx context.Context, filter DiscoveryFilter) ([]*ServiceInfo, error) {
	var services []*ServiceInfo
	var serviceIDs []string

	// Filter by type if specified
	if filter.Type != "" {
		typeKey := fmt.Sprintf("%s:types:%s", d.namespace, filter.Type)
		ids, err := d.client.SMembers(ctx, typeKey).Result()
		if err != nil && err != redis.Nil {
			return nil, fmt.Errorf("failed to find services by type %s: %w", filter.Type, err)
		}
		serviceIDs = append(serviceIDs, ids...)
	}

	// Filter by name if specified
	if filter.Name != "" {
		nameKey := fmt.Sprintf("%s:names:%s", d.namespace, filter.Name)
		ids, err := d.client.SMembers(ctx, nameKey).Result()
		if err != nil && err != redis.Nil {
			return nil, fmt.Errorf("failed to find services by name %s: %w", filter.Name, err)
		}
		
		if filter.Type != "" {
			// Intersect with type filter
			serviceIDs = intersect(serviceIDs, ids)
		} else {
			serviceIDs = append(serviceIDs, ids...)
		}
	}

	// Filter by capabilities if specified
	if len(filter.Capabilities) > 0 {
		var capIDs []string
		for _, capability := range filter.Capabilities {
			capKey := fmt.Sprintf("%s:capabilities:%s", d.namespace, capability)
			ids, err := d.client.SMembers(ctx, capKey).Result()
			if err != nil && err != redis.Nil {
				continue
			}
			capIDs = append(capIDs, ids...)
		}
		
		if len(serviceIDs) > 0 {
			// Intersect with existing filters
			serviceIDs = intersect(serviceIDs, capIDs)
		} else {
			serviceIDs = capIDs
		}
	}

	// If no filters specified, get all services
	if filter.Type == "" && filter.Name == "" && len(filter.Capabilities) == 0 {
		// Get all service keys
		pattern := fmt.Sprintf("%s:services:*", d.namespace)
		keys, err := d.client.Keys(ctx, pattern).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to list all services: %w", err)
		}
		
		for _, key := range keys {
			// Extract service ID from key
			serviceID := key[len(fmt.Sprintf("%s:services:", d.namespace)):]
			serviceIDs = append(serviceIDs, serviceID)
		}
	}

	// Remove duplicates
	seen := make(map[string]bool)
	uniqueIDs := []string{}
	for _, id := range serviceIDs {
		if !seen[id] {
			seen[id] = true
			uniqueIDs = append(uniqueIDs, id)
		}
	}

	// Fetch service info for each ID
	for _, id := range uniqueIDs {
		key := fmt.Sprintf("%s:services:%s", d.namespace, id)
		data, err := d.client.Get(ctx, key).Result()
		if err != nil {
			if err == redis.Nil {
				// Service expired or deleted, skip
				continue
			}
			return nil, fmt.Errorf("failed to get service %s: %w", id, err)
		}

		var info ServiceInfo
		if err := json.Unmarshal([]byte(data), &info); err != nil {
			// Skip malformed entries
			continue
		}

		// Apply metadata filter if specified
		if len(filter.Metadata) > 0 {
			match := true
			for k, v := range filter.Metadata {
				if info.Metadata[k] != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		services = append(services, &info)
	}

	return services, nil
}

// FindService finds services by name (backward compatibility)
func (d *RedisDiscovery) FindService(ctx context.Context, serviceName string) ([]*ServiceInfo, error) {
	return d.Discover(ctx, DiscoveryFilter{Name: serviceName})
}

// FindByCapability finds services by capability (backward compatibility)
func (d *RedisDiscovery) FindByCapability(ctx context.Context, capability string) ([]*ServiceInfo, error) {
	return d.Discover(ctx, DiscoveryFilter{Capabilities: []string{capability}})
}

// intersect returns the intersection of two string slices
func intersect(a, b []string) []string {
	set := make(map[string]bool)
	for _, v := range a {
		set[v] = true
	}
	
	var result []string
	for _, v := range b {
		if set[v] {
			result = append(result, v)
		}
	}
	return result
}