package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/pkg/logger"
	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RedisDiscovery implements service discovery using Redis
type RedisDiscovery struct {
	client    *redis.Client
	agentID   string
	namespace string
	ttl       time.Duration
	logger    logger.Logger
	// Local capability cache for fallback when Redis is unavailable
	cache *DiscoveryCache
	// Background refresh & resilience
	refreshEnabled      bool
	refreshInterval     time.Duration
	backoffInitial      time.Duration
	backoffMax          time.Duration
	cbThreshold         int
	cbCooldown          time.Duration
	warnStale           time.Duration
	consecutiveFailures int
	circuitOpenUntil    time.Time
	// Persistence
	persistEnabled bool
	persistPath    string
	
	// Phase 2: Full catalog support
	fullCatalog      map[string]*AgentRegistration
	catalogMutex     sync.RWMutex
	catalogSyncTime  time.Duration
	lastCatalogSync  time.Time
	catalogSyncErrors int
	catalogSyncCancel context.CancelFunc
}

// DiscoveryCache provides local fallback when Redis is unavailable
type DiscoveryCache struct {
	capabilities map[string][]AgentRegistration
	agents       map[string]AgentRegistration
	lastUpdated  time.Time
	mu           sync.RWMutex
}

// NewRedisDiscovery creates a new Redis-based service discovery
func NewRedisDiscovery(redisURL, agentID, namespace string) (*RedisDiscovery, error) {
	return NewRedisDiscoveryWithLogger(redisURL, agentID, namespace, logger.NewDefaultLogger())
}

// NewRedisDiscoveryWithLogger creates a new Redis-based service discovery with custom logger
func NewRedisDiscoveryWithLogger(redisURL, agentID, namespace string, log logger.Logger) (*RedisDiscovery, error) {
	if log == nil {
		log = logger.NewDefaultLogger()
	}
	
	log.Info("Creating Redis discovery client", map[string]interface{}{
		"agent_id":   agentID,
		"namespace":  namespace,
		"redis_url":  redisURL,
	})
	
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Error("Failed to parse Redis URL", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("invalid Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	if namespace == "" {
		namespace = "agents"
	}

	rd := &RedisDiscovery{
		client:    client,
		agentID:   agentID,
		namespace: namespace,
		ttl:       60 * time.Second, // Default TTL
		logger:    log,
		cache: &DiscoveryCache{
			capabilities: make(map[string][]AgentRegistration),
			agents:       make(map[string]AgentRegistration),
		},
		refreshEnabled:      true,
		refreshInterval:     15 * time.Second,
		backoffInitial:      1 * time.Second,
		backoffMax:          60 * time.Second,
		cbThreshold:         5,
		cbCooldown:          2 * time.Minute,
		warnStale:           10 * time.Minute,
		consecutiveFailures: 0,
		persistEnabled:      false,
		persistPath:         "/data/discovery_snapshot.json",
		// Phase 2: Initialize catalog fields
		fullCatalog:      make(map[string]*AgentRegistration),
		catalogSyncTime:  30 * time.Second, // Default sync interval
		catalogSyncErrors: 0,
	}

	// Test connection with retry logic
	if err := rd.connectWithRetry(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis after retries: %w", err)
	}

	return rd, nil
}

// connectWithRetry attempts to connect to Redis with exponential backoff
func (r *RedisDiscovery) connectWithRetry() error {
	tracer := otel.Tracer("gomind.discovery")
	ctx, span := tracer.Start(context.Background(), "Redis.Connect")
	defer span.End()
	
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := r.client.Ping(pingCtx).Err()
		cancel()

		if err == nil {
			r.logger.Info("Successfully connected to Redis", map[string]interface{}{
				"attempt": attempt + 1,
			})
			span.SetStatus(codes.Ok, "Connected")
			return nil // Success
		}

		r.logger.Warn("Failed to connect to Redis", map[string]interface{}{
			"attempt": attempt + 1,
			"error":   err.Error(),
		})
		span.RecordError(err)
		
		if attempt < maxRetries-1 {
			backoff := time.Duration(math.Pow(2, float64(attempt+1))) * time.Second
			r.logger.Debug("Retrying Redis connection", map[string]interface{}{
				"backoff": backoff.String(),
			})
			time.Sleep(backoff)
		}
	}
	
	span.SetStatus(codes.Error, "Connection failed")
	r.logger.Error("Failed to connect to Redis after all retries", map[string]interface{}{
		"max_retries": maxRetries,
	})
	return fmt.Errorf("failed to connect to Redis after %d attempts", maxRetries)
}

// Register registers an agent in service discovery
func (r *RedisDiscovery) Register(ctx context.Context, registration *AgentRegistration) error {
	tracer := otel.Tracer("gomind.discovery")
	ctx, span := tracer.Start(ctx, "Agent.Register",
		trace.WithAttributes(
			attribute.String("agent.id", registration.ID),
			attribute.String("agent.name", registration.Name),
			attribute.String("agent.namespace", registration.Namespace),
			attribute.Int("capabilities.count", len(registration.Capabilities)),
		),
	)
	defer span.End()
	
	agentKey := fmt.Sprintf("%s:agents:%s", r.namespace, registration.ID)

	r.logger.Info("Registering agent", map[string]interface{}{
		"agent_id":     registration.ID,
		"agent_name":   registration.Name,
		"namespace":    registration.Namespace,
		"capabilities": len(registration.Capabilities),
		"trace_id":     trace.SpanFromContext(ctx).SpanContext().TraceID().String(),
	})

	// Update heartbeat timestamp
	registration.LastHeartbeat = time.Now()

	// Serialize registration
	data, err := json.Marshal(registration)
	if err != nil {
		r.logger.Error("Failed to serialize registration", map[string]interface{}{
			"agent_id": registration.ID,
			"error":    err.Error(),
		})
		span.RecordError(err)
		span.SetStatus(codes.Error, "Serialization failed")
		return fmt.Errorf("failed to serialize registration: %w", err)
	}

	// Store agent registration with TTL
	if err := r.client.Set(ctx, agentKey, data, r.ttl).Err(); err != nil {
		r.logger.Error("Failed to store agent registration", map[string]interface{}{
			"agent_id": registration.ID,
			"key":      agentKey,
			"error":    err.Error(),
		})
		span.RecordError(err)
		span.SetStatus(codes.Error, "Storage failed")
		return fmt.Errorf("failed to register agent: %w", err)
	}

	// Index by capabilities
	pipe := r.client.Pipeline()
	for _, capability := range registration.Capabilities {
		capabilityKey := fmt.Sprintf("%s:capabilities:%s", r.namespace, capability.Name)
		pipe.SAdd(ctx, capabilityKey, registration.ID)
		pipe.Expire(ctx, capabilityKey, r.ttl+10*time.Second) // Slightly longer TTL for indices
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		r.logger.Error("Failed to index capabilities", map[string]interface{}{
			"agent_id": registration.ID,
			"error":    err.Error(),
		})
		span.RecordError(err)
		span.SetStatus(codes.Error, "Indexing failed")
		return fmt.Errorf("failed to index capabilities: %w", err)
	}

	// Update local cache
	r.updateCache(*registration)

	r.logger.Info("Agent registered successfully", map[string]interface{}{
		"agent_id":     registration.ID,
		"capabilities": len(registration.Capabilities),
	})
	span.SetStatus(codes.Ok, "Agent registered")

	return nil
}

// FindCapability finds agents that provide a specific capability
func (r *RedisDiscovery) FindCapability(ctx context.Context, capability string) ([]AgentRegistration, error) {
	tracer := otel.Tracer("gomind.discovery")
	ctx, span := tracer.Start(ctx, "Discovery.FindCapability",
		trace.WithAttributes(
			attribute.String("capability", capability),
		),
	)
	defer span.End()
	
	capabilityKey := fmt.Sprintf("%s:capabilities:%s", r.namespace, capability)

	r.logger.Debug("Finding agents with capability", map[string]interface{}{
		"capability": capability,
		"key":        capabilityKey,
	})

	agentIDs, err := r.client.SMembers(ctx, capabilityKey).Result()
	if err != nil {
		r.logger.Warn("Failed to query Redis, using cache", map[string]interface{}{
			"capability": capability,
			"error":      err.Error(),
		})
		span.RecordError(err)
		span.AddEvent("Falling back to cache")
		// Fallback to local cache if Redis is unavailable
		return r.findCapabilityFromCache(capability), nil
	}

	r.logger.Debug("Found agents in Redis", map[string]interface{}{
		"capability":  capability,
		"agent_count": len(agentIDs),
	})

	var registrations []AgentRegistration
	for _, agentID := range agentIDs {
		registration, err := r.FindAgent(ctx, agentID)
		if err != nil {
			r.logger.Warn("Failed to fetch agent registration", map[string]interface{}{
				"agent_id": agentID,
				"error":    err.Error(),
			})
			// Skip agents that no longer exist
			continue
		}
		registrations = append(registrations, *registration)
	}

	// Update cache with fresh data
	r.updateCapabilityCache(capability, registrations)

	return registrations, nil
}

// findCapabilityFromCache returns agents from local cache
func (r *RedisDiscovery) findCapabilityFromCache(capability string) []AgentRegistration {
	r.cache.mu.RLock()
	defer r.cache.mu.RUnlock()

	if registrations, exists := r.cache.capabilities[capability]; exists {
		// Filter out expired entries (older than 2 minutes)
		var valid []AgentRegistration
		for _, reg := range registrations {
			if time.Since(reg.LastHeartbeat) < 2*time.Minute {
				valid = append(valid, reg)
			}
		}
		return valid
	}
	return []AgentRegistration{}
}

// updateCapabilityCache updates the local cache for a capability
func (r *RedisDiscovery) updateCapabilityCache(capability string, registrations []AgentRegistration) {
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()
	r.cache.capabilities[capability] = registrations
	r.cache.lastUpdated = time.Now()
}

// updateCache updates the local cache with agent registration
func (r *RedisDiscovery) updateCache(registration AgentRegistration) {
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()
	r.cache.agents[registration.ID] = registration

	// Update capability cache
	for _, capability := range registration.Capabilities {
		existing := r.cache.capabilities[capability.Name]
		// Remove old entry for this agent if exists
		filtered := make([]AgentRegistration, 0, len(existing))
		for _, reg := range existing {
			if reg.ID != registration.ID {
				filtered = append(filtered, reg)
			}
		}
		// Add updated registration
		filtered = append(filtered, registration)
		r.cache.capabilities[capability.Name] = filtered
	}
	r.cache.lastUpdated = time.Now()
}

// FindAgent finds a specific agent by ID
func (r *RedisDiscovery) FindAgent(ctx context.Context, agentID string) (*AgentRegistration, error) {
	agentKey := fmt.Sprintf("%s:agents:%s", r.namespace, agentID)

	data, err := r.client.Get(ctx, agentKey).Result()
	if err != nil {
		// Fall back to local cache when Redis is unavailable
		if err == redis.Nil {
			// Not found in Redis; check cache
			r.cache.mu.RLock()
			cached, ok := r.cache.agents[agentID]
			r.cache.mu.RUnlock()
			if ok && time.Since(cached.LastHeartbeat) < 2*time.Minute {
				return &cached, nil
			}
			return nil, fmt.Errorf("agent not found: %s", agentID)
		}
		// For other errors (connection, timeouts), try cache too
		r.cache.mu.RLock()
		cached, ok := r.cache.agents[agentID]
		r.cache.mu.RUnlock()
		if ok && time.Since(cached.LastHeartbeat) < 2*time.Minute {
			return &cached, nil
		}
		return nil, fmt.Errorf("failed to find agent: %w", err)
	}

	var registration AgentRegistration
	if err := json.Unmarshal([]byte(data), &registration); err != nil {
		return nil, fmt.Errorf("failed to deserialize registration: %w", err)
	}

	return &registration, nil
}

// Unregister removes an agent from service discovery
func (r *RedisDiscovery) Unregister(ctx context.Context, agentID string) error {
	// Get agent registration to clean up capability indices
	registration, err := r.FindAgent(ctx, agentID)
	if err != nil {
		// Agent doesn't exist, nothing to unregister
		return nil
	}

	agentKey := fmt.Sprintf("%s:agents:%s", r.namespace, agentID)

	// Remove agent registration
	if err := r.client.Del(ctx, agentKey).Err(); err != nil {
		return fmt.Errorf("failed to unregister agent: %w", err)
	}

	// Remove from capability indices
	pipe := r.client.Pipeline()
	for _, capability := range registration.Capabilities {
		capabilityKey := fmt.Sprintf("%s:capabilities:%s", r.namespace, capability.Name)
		pipe.SRem(ctx, capabilityKey, agentID)
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to clean up capability indices: %w", err)
	}

	return nil
}

// GetHealthStatus returns the health status of the Redis discovery service
func (r *RedisDiscovery) GetHealthStatus(ctx context.Context) HealthStatus {
	err := r.client.Ping(ctx).Err()
	if err != nil {
		return HealthStatus{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("Redis connection failed: %v", err),
			Details:   map[string]string{"error": err.Error()},
			Timestamp: time.Now(),
		}
	}

	return HealthStatus{
		Status:    "healthy",
		Message:   "Redis connection is healthy",
		Details:   map[string]string{"namespace": r.namespace},
		Timestamp: time.Now(),
	}
}

// RefreshHeartbeat updates the heartbeat timestamp for an agent
func (r *RedisDiscovery) RefreshHeartbeat(ctx context.Context, agentID string) error {
	// Get current registration, with cache fallback
	registration, err := r.FindAgent(ctx, agentID)
	if err != nil {
		// Attempt local cache fallback
		r.cache.mu.RLock()
		cached, ok := r.cache.agents[agentID]
		r.cache.mu.RUnlock()
		if !ok {
			return fmt.Errorf("agent not found for heartbeat refresh: %w", err)
		}
		registration = &cached
	}

	// Update heartbeat timestamp
	registration.LastHeartbeat = time.Now()

	// Re-register with updated timestamp and reset TTL
	agentKey := fmt.Sprintf("%s:agents:%s", r.namespace, agentID)

	data, err := json.Marshal(registration)
	if err != nil {
		return fmt.Errorf("failed to serialize registration for heartbeat: %w", err)
	}

	// Update Redis with fresh TTL
	if err := r.client.Set(ctx, agentKey, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("failed to refresh heartbeat: %w", err)
	}

	// Also refresh capability indices TTLs and ensure membership (service-scoped friendly)
	pipe := r.client.Pipeline()
	for _, capability := range registration.Capabilities {
		capabilityKey := fmt.Sprintf("%s:capabilities:%s", r.namespace, capability.Name)
		pipe.SAdd(ctx, capabilityKey, registration.ID)
		pipe.Expire(ctx, capabilityKey, r.ttl+10*time.Second)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		// Non-fatal; log via health status by returning error context
		return fmt.Errorf("failed to refresh capability indices: %w", err)
	}

	// Update local cache
	r.updateCache(*registration)

	return nil
}

// Close closes the Redis connection
func (r *RedisDiscovery) Close() error {
	return r.client.Close()
}

// SetTTL sets the TTL for agent registrations
func (r *RedisDiscovery) SetTTL(ttl time.Duration) {
	r.ttl = ttl
}

// SetAgentID updates the agent ID for the discovery service
func (r *RedisDiscovery) SetAgentID(agentID string) {
	r.agentID = agentID
}

// ConfigureCache sets cache refresh and resilience parameters
func (r *RedisDiscovery) ConfigureCache(enabled bool, refreshInterval, backoffInitial, backoffMax time.Duration, cbThreshold int, cbCooldown, warnStale time.Duration) {
	r.refreshEnabled = enabled
	if refreshInterval > 0 {
		r.refreshInterval = refreshInterval
	}
	if backoffInitial > 0 {
		r.backoffInitial = backoffInitial
	}
	if backoffMax > 0 {
		r.backoffMax = backoffMax
	}
	if cbThreshold > 0 {
		r.cbThreshold = cbThreshold
	}
	if cbCooldown > 0 {
		r.cbCooldown = cbCooldown
	}
	if warnStale > 0 {
		r.warnStale = warnStale
	}
}

// ConfigurePersistence toggles snapshot load/save behavior
func (r *RedisDiscovery) ConfigurePersistence(enabled bool, path string) {
	r.persistEnabled = enabled
	if path != "" {
		r.persistPath = path
	}
}

// Ping returns Redis connectivity status
func (r *RedisDiscovery) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// SnapshotStats returns counts and age for readiness/status
func (r *RedisDiscovery) SnapshotStats() (agents int, capabilities int, age time.Duration, loaded bool) {
	r.cache.mu.RLock()
	defer r.cache.mu.RUnlock()
	agents = len(r.cache.agents)
	capabilities = len(r.cache.capabilities)
	if !r.cache.lastUpdated.IsZero() {
		age = time.Since(r.cache.lastUpdated)
		loaded = agents > 0 || capabilities > 0
	} else {
		age = 0
		loaded = false
	}
	return
}

// IsCircuitOpen indicates if refresh circuit breaker is open
func (r *RedisDiscovery) IsCircuitOpen() bool {
	if r.circuitOpenUntil.IsZero() {
		return false
	}
	if time.Now().Before(r.circuitOpenUntil) {
		return true
	}
	// Expired
	r.circuitOpenUntil = time.Time{}
	return false
}

// ConsecutiveFailures returns current failure streak
func (r *RedisDiscovery) ConsecutiveFailures() int { return r.consecutiveFailures }

// StartBackgroundRefresh runs an independent refresher for the local cache
func (r *RedisDiscovery) StartBackgroundRefresh(ctx context.Context) {
	if !r.refreshEnabled {
		return
	}
	ticker := time.NewTicker(r.refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if r.IsCircuitOpen() {
				// Skip refresh until cooldown ends
				continue
			}
			if err := r.refreshSnapshot(ctx); err != nil {
				r.consecutiveFailures++
				// Open circuit if threshold reached
				if r.consecutiveFailures >= r.cbThreshold {
					r.circuitOpenUntil = time.Now().Add(r.cbCooldown)
				}
				// Backoff before next tick
				backoff := r.backoffInitial
				for i := 1; i < r.consecutiveFailures; i++ {
					backoff *= 2
					if backoff > r.backoffMax {
						backoff = r.backoffMax
						break
					}
				}
				// Sleep backoff but remain cancellable
				timer := time.NewTimer(backoff)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			} else {
				r.consecutiveFailures = 0
				// Optionally persist snapshot
				if r.persistEnabled {
					_ = r.SaveSnapshot()
				}
			}
		}
	}
}

// refreshSnapshot rebuilds the local cache from Redis
func (r *RedisDiscovery) refreshSnapshot(ctx context.Context) error {
	// Scan all agent records: <ns>:agents:*
	pattern := fmt.Sprintf("%s:agents:*", r.namespace)
	var cursor uint64
	newAgents := make(map[string]AgentRegistration)
	for {
		keys, next, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		for _, key := range keys {
			data, err := r.client.Get(ctx, key).Result()
			if err != nil {
				continue
			}
			var reg AgentRegistration
			if err := json.Unmarshal([]byte(data), &reg); err != nil {
				continue
			}
			newAgents[reg.ID] = reg
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	// Build capability map from agent registrations
	newCaps := make(map[string][]AgentRegistration)
	for _, reg := range newAgents {
		for _, cap := range reg.Capabilities {
			list := newCaps[cap.Name]
			list = append(list, reg)
			newCaps[cap.Name] = list
		}
	}
	// Swap into cache
	r.cache.mu.Lock()
	r.cache.agents = newAgents
	r.cache.capabilities = newCaps
	r.cache.lastUpdated = time.Now()
	r.cache.mu.Unlock()
	return nil
}

// SaveSnapshot persists the current cache to disk (best-effort)
func (r *RedisDiscovery) SaveSnapshot() error {
	if !r.persistEnabled || r.persistPath == "" {
		return nil
	}
	r.cache.mu.RLock()
	defer r.cache.mu.RUnlock()
	// Flatten agents to slice
	agents := make([]AgentRegistration, 0, len(r.cache.agents))
	for _, reg := range r.cache.agents {
		agents = append(agents, reg)
	}
	snapshot := struct {
		Agents       []AgentRegistration            `json:"agents"`
		Capabilities map[string][]AgentRegistration `json:"capabilities"`
		LastUpdated  string                         `json:"last_updated"`
	}{
		Agents:       agents,
		Capabilities: r.cache.capabilities,
		LastUpdated:  time.Now().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.persistPath, data, 0o644)
}

// LoadSnapshot loads a snapshot from disk into cache if available
func (r *RedisDiscovery) LoadSnapshot() error {
	if !r.persistEnabled || r.persistPath == "" {
		return nil
	}
	data, err := os.ReadFile(r.persistPath)
	if err != nil {
		return err
	}
	var snapshot struct {
		Agents       []AgentRegistration            `json:"agents"`
		Capabilities map[string][]AgentRegistration `json:"capabilities"`
		LastUpdated  string                         `json:"last_updated"`
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return err
	}
	// Rebuild agents map
	agentsMap := make(map[string]AgentRegistration, len(snapshot.Agents))
	for _, reg := range snapshot.Agents {
		agentsMap[reg.ID] = reg
	}
	r.cache.mu.Lock()
	r.cache.agents = agentsMap
	if snapshot.Capabilities != nil {
		r.cache.capabilities = snapshot.Capabilities
	}
	if ts, err := time.Parse(time.RFC3339, snapshot.LastUpdated); err == nil {
		r.cache.lastUpdated = ts
	} else {
		r.cache.lastUpdated = time.Now()
	}
	r.cache.mu.Unlock()
	return nil
}

// GetAllAgents returns all registered agents (for framework internal use)
func (r *RedisDiscovery) GetAllAgents(ctx context.Context, pattern string) ([]AgentRegistration, error) {
	var agents []AgentRegistration
	var cursor uint64

	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			data, err := r.client.Get(ctx, key).Result()
			if err != nil {
				continue // Skip failed retrievals
			}

			var agent AgentRegistration
			if err := json.Unmarshal([]byte(data), &agent); err != nil {
				continue // Skip invalid data
			}

			agents = append(agents, agent)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return agents, nil
}

// ============================================================================
// Phase 2: Catalog Management Methods
// ============================================================================

// DownloadFullCatalog downloads all agent registrations from Redis
func (r *RedisDiscovery) DownloadFullCatalog(ctx context.Context) error {
	// Check if Redis client is available
	if r.client == nil {
		return fmt.Errorf("Redis client not initialized")
	}
	
	// Use circuit breaker pattern
	if r.IsCircuitOpen() {
		// Use cached catalog when circuit is open
		return fmt.Errorf("circuit breaker open, using cached catalog")
	}

	// Download all agents from Redis
	pattern := fmt.Sprintf("%s:agent:*", r.namespace)
	agents, err := r.GetAllAgents(ctx, pattern)
	if err != nil {
		r.catalogSyncErrors++
		r.consecutiveFailures++
		
		// Check if we should open the circuit
		if r.consecutiveFailures >= r.cbThreshold {
			r.circuitOpenUntil = time.Now().Add(r.cbCooldown)
		}
		
		return fmt.Errorf("failed to download catalog: %w", err)
	}

	// Reset failure counters on success
	r.consecutiveFailures = 0
	r.catalogSyncErrors = 0

	// Update the full catalog with thread-safe access
	r.catalogMutex.Lock()
	defer r.catalogMutex.Unlock()

	// Clear and rebuild catalog
	r.fullCatalog = make(map[string]*AgentRegistration)
	for _, agent := range agents {
		agentCopy := agent // Important: create a copy
		r.fullCatalog[agent.ID] = &agentCopy
		
		// Also update the cache for backward compatibility
		r.cache.mu.Lock()
		r.cache.agents[agent.ID] = agent
		r.cache.mu.Unlock()
	}

	r.lastCatalogSync = time.Now()

	// Persist catalog if enabled
	if r.persistEnabled {
		r.SaveSnapshot()
	}

	return nil
}

// GetFullCatalog returns the current full catalog of agents
func (r *RedisDiscovery) GetFullCatalog() map[string]*AgentRegistration {
	r.catalogMutex.RLock()
	defer r.catalogMutex.RUnlock()

	// Return a copy to prevent external modifications
	catalog := make(map[string]*AgentRegistration)
	for id, agent := range r.fullCatalog {
		catalog[id] = agent
	}

	return catalog
}

// GetCatalogForLLM returns a formatted string of the agent catalog optimized for LLM consumption
func (r *RedisDiscovery) GetCatalogForLLM() string {
	r.catalogMutex.RLock()
	defer r.catalogMutex.RUnlock()

	if len(r.fullCatalog) == 0 {
		return "No agents currently available in the catalog."
	}

	var builder strings.Builder
	builder.WriteString("=== AVAILABLE AGENTS CATALOG ===\n\n")

	// Group agents by namespace for better organization
	agentsByNamespace := make(map[string][]*AgentRegistration)
	for _, agent := range r.fullCatalog {
		ns := agent.Namespace
		if ns == "" {
			ns = "default"
		}
		agentsByNamespace[ns] = append(agentsByNamespace[ns], agent)
	}

	// Format each namespace and its agents
	for namespace, agents := range agentsByNamespace {
		builder.WriteString(fmt.Sprintf("NAMESPACE: %s\n", namespace))
		builder.WriteString(strings.Repeat("-", 50) + "\n")

		for i, agent := range agents {
			// Basic agent info
			builder.WriteString(fmt.Sprintf("\n%d. AGENT: %s (ID: %s)\n", i+1, agent.Name, agent.ID))
			
			// Add description if available
			if agent.Description != "" {
				builder.WriteString(fmt.Sprintf("   Description: %s\n", agent.Description))
			}

			// Add service endpoint for K8s
			if agent.ServiceEndpoint != "" {
				builder.WriteString(fmt.Sprintf("   Endpoint: %s\n", agent.ServiceEndpoint))
			} else if agent.ServiceName != "" && agent.Namespace != "" {
				// Build K8s service endpoint
				endpoint := fmt.Sprintf("%s.%s.svc.cluster.local:8080", agent.ServiceName, agent.Namespace)
				builder.WriteString(fmt.Sprintf("   Endpoint: %s\n", endpoint))
			} else {
				// Fallback to address:port
				builder.WriteString(fmt.Sprintf("   Endpoint: %s:%d\n", agent.Address, agent.Port))
			}

			// Add status
			builder.WriteString(fmt.Sprintf("   Status: %s\n", agent.Status))

			// List capabilities with LLM-friendly descriptions
			if len(agent.Capabilities) > 0 {
				builder.WriteString("   Capabilities:\n")
				for _, cap := range agent.Capabilities {
					builder.WriteString(fmt.Sprintf("      • %s", cap.Name))
					if cap.Description != "" {
						builder.WriteString(fmt.Sprintf(" - %s", cap.Description))
					}
					if cap.LLMPrompt != "" {
						builder.WriteString(fmt.Sprintf("\n        LLM Hint: %s", cap.LLMPrompt))
					}
					builder.WriteString("\n")
				}
			}

			// Add examples if available
			if len(agent.Examples) > 0 {
				builder.WriteString("   Example requests:\n")
				for _, example := range agent.Examples {
					builder.WriteString(fmt.Sprintf("      • \"%s\"\n", example))
				}
			}

			// Add LLM hints if available
			if agent.LLMHints != "" {
				builder.WriteString(fmt.Sprintf("   Routing hints: %s\n", agent.LLMHints))
			}

			// Add last seen time for health awareness
			timeSinceHeartbeat := time.Since(agent.LastHeartbeat)
			if timeSinceHeartbeat < 1*time.Minute {
				builder.WriteString("   Health: ✓ Active (recently seen)\n")
			} else if timeSinceHeartbeat < 5*time.Minute {
				builder.WriteString(fmt.Sprintf("   Health: ⚠ Last seen %v ago\n", timeSinceHeartbeat.Round(time.Second)))
			} else {
				builder.WriteString(fmt.Sprintf("   Health: ✗ Inactive (last seen %v ago)\n", timeSinceHeartbeat.Round(time.Minute)))
			}
		}
		builder.WriteString("\n")
	}

	// Add summary statistics
	builder.WriteString(strings.Repeat("=", 50) + "\n")
	builder.WriteString(fmt.Sprintf("SUMMARY: %d agents across %d namespaces\n", len(r.fullCatalog), len(agentsByNamespace)))
	builder.WriteString(fmt.Sprintf("Last synchronized: %v ago\n", time.Since(r.lastCatalogSync).Round(time.Second)))
	
	if r.catalogSyncErrors > 0 {
		builder.WriteString(fmt.Sprintf("⚠ Warning: %d sync errors occurred\n", r.catalogSyncErrors))
	}

	return builder.String()
}

// StartCatalogSync starts a background goroutine that periodically syncs the catalog
func (r *RedisDiscovery) StartCatalogSync(ctx context.Context, interval time.Duration) {
	// Cancel any existing sync routine
	if r.catalogSyncCancel != nil {
		r.catalogSyncCancel()
	}

	// Create a new context for this sync routine
	syncCtx, cancel := context.WithCancel(ctx)
	r.catalogSyncCancel = cancel

	// Update sync interval if provided
	if interval > 0 {
		r.catalogSyncTime = interval
	}

	// Start the sync goroutine
	go func() {
		// Initial download
		if err := r.DownloadFullCatalog(syncCtx); err != nil {
			// Log error but continue - we might have cached data
			fmt.Printf("Initial catalog download failed: %v\n", err)
		}

		ticker := time.NewTicker(r.catalogSyncTime)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := r.DownloadFullCatalog(syncCtx); err != nil {
					// Log error but continue with cached data
					fmt.Printf("Catalog sync failed: %v\n", err)
				}
			case <-syncCtx.Done():
				return
			}
		}
	}()
}

// GetCatalogStats returns statistics about the catalog
func (r *RedisDiscovery) GetCatalogStats() (agentCount int, lastSync time.Time, syncErrors int) {
	r.catalogMutex.RLock()
	defer r.catalogMutex.RUnlock()

	return len(r.fullCatalog), r.lastCatalogSync, r.catalogSyncErrors
}

// SetCatalogSyncInterval updates the catalog sync interval
func (r *RedisDiscovery) SetCatalogSyncInterval(interval time.Duration) {
	r.catalogSyncTime = interval
}
