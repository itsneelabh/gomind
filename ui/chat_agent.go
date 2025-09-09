package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// DefaultChatAgent implements an AI-powered chat agent with pluggable transports
// It implements the ChatAgent interface defined in interfaces.go
type DefaultChatAgent struct {
	*core.BaseAgent
	aiClient   core.AIClient
	sessions   SessionManager
	transports map[string]Transport
	config     ChatAgentConfig
	mu         sync.RWMutex

	// Injected dependencies
	circuitBreaker core.CircuitBreaker

	// Resource lifecycle management
	server   *http.Server
	stopping bool
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// DefaultChatAgentConfig returns default configuration
func DefaultChatAgentConfig(name string) ChatAgentConfig {
	return ChatAgentConfig{
		Name:          name,
		SessionConfig: DefaultSessionConfig(),
		SecurityConfig: SecurityConfig{
			RateLimit:      20,
			MaxMessageSize: 4096,
			AllowedOrigins: []string{"*"},
			RequireAuth:    false,
		},
		TransportConfigs:      make(map[string]TransportConfig),
		CircuitBreakerEnabled: true,
		CircuitBreakerConfig:  DefaultCircuitBreakerConfig(),
	}
}

// NewChatAgent creates a new chat agent with auto-configured transports
func NewChatAgent(config ChatAgentConfig, aiClient core.AIClient, sessions SessionManager) *DefaultChatAgent {
	agent := &DefaultChatAgent{
		BaseAgent:  core.NewBaseAgent(config.Name),
		aiClient:   aiClient,
		sessions:   sessions,
		transports: make(map[string]Transport),
		config:     config,
		stopChan:   make(chan struct{}),
	}

	// Auto-configure available transports
	agent.AutoConfigureTransports()

	// Add discovery endpoint
	agent.RegisterCapability(core.Capability{
		Name:        "transports",
		Description: "Discover available transports",
		Endpoint:    "/chat/transports",
		Handler:     agent.HandleTransportDiscovery,
	})

	// Add health endpoint
	agent.RegisterCapability(core.Capability{
		Name:        "health",
		Description: "Health check",
		Endpoint:    "/chat/health",
		Handler:     agent.HandleHealth,
	})

	return agent
}

// NewChatAgentWithDependencies creates a new chat agent with injected dependencies.
// This is the preferred constructor for production use as it allows proper
// dependency injection without direct module imports.
func NewChatAgentWithDependencies(
	config ChatAgentConfig,
	sessions SessionManager,
	deps ChatAgentDependencies,
) *DefaultChatAgent {
	agent := &DefaultChatAgent{
		BaseAgent:      core.NewBaseAgent(config.Name),
		aiClient:       deps.AIClient,
		sessions:       sessions,
		transports:     make(map[string]Transport),
		config:         config,
		circuitBreaker: deps.CircuitBreaker,
		stopChan:       make(chan struct{}),
	}

	// Set logger and telemetry if provided
	if deps.Logger != nil {
		agent.BaseAgent.Logger = deps.Logger
	}
	if deps.Telemetry != nil {
		agent.BaseAgent.Telemetry = deps.Telemetry
	}

	// Auto-configure available transports
	agent.AutoConfigureTransports()

	// Add discovery endpoint
	agent.RegisterCapability(core.Capability{
		Name:        "transports",
		Description: "Discover available transports",
		Endpoint:    "/chat/transports",
		Handler:     agent.HandleTransportDiscovery,
	})

	// Add health endpoint
	agent.RegisterCapability(core.Capability{
		Name:        "health",
		Description: "Health check",
		Endpoint:    "/chat/health",
		Handler:     agent.HandleHealth,
	})

	return agent
}

// NewChatAgentWithOptions creates a new chat agent with functional options.
// This provides a flexible way to configure the agent.
func NewChatAgentWithOptions(
	config ChatAgentConfig,
	sessions SessionManager,
	opts ...ChatAgentOption,
) *DefaultChatAgent {
	agent := &DefaultChatAgent{
		BaseAgent:  core.NewBaseAgent(config.Name),
		sessions:   sessions,
		transports: make(map[string]Transport),
		config:     config,
		stopChan:   make(chan struct{}),
	}

	// Apply options
	for _, opt := range opts {
		opt(agent)
	}

	// Auto-configure available transports
	agent.AutoConfigureTransports()

	// Add discovery endpoint
	agent.RegisterCapability(core.Capability{
		Name:        "transports",
		Description: "Discover available transports",
		Endpoint:    "/chat/transports",
		Handler:     agent.HandleTransportDiscovery,
	})

	// Add health endpoint
	agent.RegisterCapability(core.Capability{
		Name:        "health",
		Description: "Health check",
		Endpoint:    "/chat/health",
		Handler:     agent.HandleHealth,
	})

	return agent
}

// AutoConfigureTransports automatically configures available transports
func (c *DefaultChatAgent) AutoConfigureTransports() {
	available := ListAvailableTransports()

	for _, transport := range available {
		// Initialize transport with config
		config := c.config.TransportConfigs[transport.Name()]
		if config.Options == nil {
			config = TransportConfig{
				MaxConnections: 1000,
				Timeout:        30 * time.Second,
			}
		}

		if err := transport.Initialize(config); err != nil {
			c.Logger.Error("Failed to initialize transport", map[string]interface{}{
				"transport": transport.Name(),
				"error":     err.Error(),
			})
			continue
		}

		// Wrap with circuit breaker if enabled
		if c.config.CircuitBreakerEnabled {
			// Use injected circuit breaker if available (preferred)
			if c.circuitBreaker != nil {
				transport = NewInterfaceBasedCircuitBreakerTransport(transport, c.circuitBreaker)

				c.Logger.Info("Wrapped transport with injected circuit breaker", map[string]interface{}{
					"transport": transport.Name(),
					"state":     c.circuitBreaker.GetState(),
				})
			} else {
				// Fall back to legacy built-in circuit breaker (deprecated)
				// TODO: Remove this fallback in next major version
				cbConfig := c.config.CircuitBreakerConfig
				cbConfig.Logger = c.BaseAgent.Logger
				cbConfig.Telemetry = c.BaseAgent.Telemetry

				transport = NewCircuitBreakerTransport(transport, cbConfig)

				c.Logger.Info("Wrapped transport with legacy circuit breaker (deprecated)", map[string]interface{}{
					"transport":         transport.Name(),
					"failure_threshold": cbConfig.FailureThreshold,
					"timeout":           cbConfig.Timeout,
					"warning":           "Please use dependency injection for circuit breaker",
				})
			}
		}

		// Start transport
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := transport.Start(ctx); err != nil {
			c.Logger.Error("Failed to start transport", map[string]interface{}{
				"transport": transport.Name(),
				"error":     err.Error(),
			})
			cancel()
			continue
		}
		cancel()

		// Create endpoint and handler
		endpoint := fmt.Sprintf("/chat/%s", transport.Name())
		handler := transport.CreateHandler(c)

		// Add capability - need to wrap the handler in a HandlerFunc
		c.RegisterCapability(core.Capability{
			Name:        fmt.Sprintf("chat_%s", transport.Name()),
			Description: transport.Description(),
			Endpoint:    endpoint,
			Handler: func(w http.ResponseWriter, r *http.Request) {
				handler.ServeHTTP(w, r)
			},
		})

		// Store transport
		c.transports[transport.Name()] = transport

		c.Logger.Info("Registered transport", map[string]interface{}{
			"transport": transport.Name(),
			"endpoint":  endpoint,
			"priority":  transport.Priority(),
		})
	}
}

// HandleTransportDiscovery allows clients to discover available transports
func (c *DefaultChatAgent) HandleTransportDiscovery(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var transports []TransportInfo
	for name, t := range c.transports {
		transports = append(transports, TransportInfo{
			Name:         name,
			Endpoint:     fmt.Sprintf("/chat/%s", name),
			Priority:     t.Priority(),
			Description:  t.Description(),
			Example:      t.ClientExample(),
			Capabilities: t.Capabilities(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"transports": transports,
		"config": map[string]interface{}{
			"rate_limit":       c.config.SecurityConfig.RateLimit,
			"max_message_size": c.config.SecurityConfig.MaxMessageSize,
		},
	})
}

// HandleHealth provides health status
func (c *DefaultChatAgent) HandleHealth(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	health := map[string]interface{}{
		"status":     "healthy",
		"transports": make(map[string]interface{}),
	}

	// Check each transport health
	for name, transport := range c.transports {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		err := transport.HealthCheck(ctx)
		cancel()

		transportHealth := map[string]interface{}{
			"available": transport.Available(),
			"healthy":   err == nil,
		}
		if err != nil {
			transportHealth["error"] = err.Error()
		}
		health["transports"].(map[string]interface{})[name] = transportHealth
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// CreateSession creates a new chat session
func (c *DefaultChatAgent) CreateSession(ctx context.Context) (*Session, error) {
	// Start telemetry span if available
	if c.BaseAgent.Telemetry != nil {
		var span core.Span
		ctx, span = c.BaseAgent.Telemetry.StartSpan(ctx, "chat.session.create")
		defer span.End()
	}

	// Create session with empty metadata
	session, err := c.sessions.Create(ctx, nil)

	// Record metrics
	if c.BaseAgent.Telemetry != nil {
		if err != nil {
			c.BaseAgent.Telemetry.RecordMetric("chat.session.create.error", 1.0, nil)
		} else {
			c.BaseAgent.Telemetry.RecordMetric("chat.session.create.success", 1.0, nil)
		}
	}

	return session, err
}

// GetSession retrieves a session
func (c *DefaultChatAgent) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	return c.sessions.Get(ctx, sessionID)
}

// CheckRateLimit checks if a session has exceeded rate limit
func (c *DefaultChatAgent) CheckRateLimit(ctx context.Context, sessionID string) (bool, error) {
	// Get the full rate limit info from session manager
	allowed, _, err := c.sessions.CheckRateLimit(ctx, sessionID)

	// Record rate limit metrics
	if c.BaseAgent.Telemetry != nil {
		if err != nil {
			c.BaseAgent.Telemetry.RecordMetric("chat.ratelimit.check.error", 1.0, nil)
		} else if !allowed {
			c.BaseAgent.Telemetry.RecordMetric("chat.ratelimit.exceeded", 1.0, map[string]string{
				"session_id": sessionID,
			})
		}
	}

	return allowed, err
}

// StreamResponse streams AI responses for a message
func (c *DefaultChatAgent) StreamResponse(ctx context.Context, sessionID string, message string) (<-chan ChatEvent, error) {
	// Start telemetry span if available
	if c.BaseAgent.Telemetry != nil {
		var span core.Span
		ctx, span = c.BaseAgent.Telemetry.StartSpan(ctx, "chat.stream_response")
		defer span.End()
		span.SetAttribute("session_id", sessionID)
		span.SetAttribute("message_length", len(message))
	}

	// Validate message size
	if len(message) > c.config.SecurityConfig.MaxMessageSize {
		if c.BaseAgent.Telemetry != nil {
			c.BaseAgent.Telemetry.RecordMetric("chat.message.rejected", 1.0, map[string]string{
				"reason": "size_exceeded",
			})
		}
		return nil, fmt.Errorf("message exceeds maximum size")
	}

	// Get or create session
	session, err := c.sessions.Get(ctx, sessionID)
	if err != nil {
		// Try to create new session with empty metadata
		session, err = c.sessions.Create(ctx, nil)
		if err != nil {
			if c.BaseAgent.Telemetry != nil {
				c.BaseAgent.Telemetry.RecordMetric("chat.session.create.failed", 1.0, nil)
			}
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
		sessionID = session.ID

		if c.BaseAgent.Telemetry != nil {
			c.BaseAgent.Telemetry.RecordMetric("chat.session.auto_created", 1.0, nil)
		}
	}

	// Add user message to session
	userMsg := Message{
		Role:       "user",
		Content:    message,
		TokenCount: len(message) / 4, // Rough estimate
	}
	if err := c.sessions.AddMessage(ctx, sessionID, userMsg); err != nil {
		return nil, fmt.Errorf("failed to add message: %w", err)
	}

	// Create response channel
	events := make(chan ChatEvent, 100)

	// Stream AI response in background
	go func() {
		defer close(events)

		// Get conversation history
		messages, _ := c.sessions.GetMessages(ctx, sessionID, 10)

		// Build prompt with history
		var prompt string
		for _, msg := range messages {
			if msg.Role == "user" {
				prompt += fmt.Sprintf("User: %s\n", msg.Content)
			} else if msg.Role == "assistant" {
				prompt += fmt.Sprintf("Assistant: %s\n", msg.Content)
			}
		}

		// Create AI request using core.AIOptions
		options := &core.AIOptions{
			SystemPrompt: "You are a helpful AI assistant.",
			Temperature:  0.7,
			MaxTokens:    1000,
		}

		// Generate AI response
		startTime := time.Now()
		response, err := c.aiClient.GenerateResponse(ctx, prompt, options)

		// Record AI generation metrics
		if c.BaseAgent.Telemetry != nil {
			duration := time.Since(startTime)
			c.BaseAgent.Telemetry.RecordMetric("chat.ai.generation.duration", float64(duration.Milliseconds()), nil)

			if err != nil {
				c.BaseAgent.Telemetry.RecordMetric("chat.ai.generation.error", 1.0, nil)
			} else {
				c.BaseAgent.Telemetry.RecordMetric("chat.ai.generation.success", 1.0, nil)
				if response.Usage.TotalTokens > 0 {
					c.BaseAgent.Telemetry.RecordMetric("chat.ai.tokens.used", float64(response.Usage.TotalTokens), nil)
				}
			}
		}

		if err != nil {
			events <- ChatEvent{
				Type:      EventError,
				Data:      err.Error(),
				Timestamp: time.Now(),
			}
			return
		}

		// Send response to client
		events <- ChatEvent{
			Type: EventMessage,
			Data: response.Content,
			Metadata: map[string]interface{}{
				"role":  "assistant",
				"model": response.Model,
			},
			Timestamp: time.Now(),
		}

		fullResponse := response.Content
		tokenCount := response.Usage.TotalTokens

		// Save assistant response to session
		assistantMsg := Message{
			Role:       "assistant",
			Content:    fullResponse,
			TokenCount: tokenCount,
		}
		c.sessions.AddMessage(ctx, sessionID, assistantMsg)

		// Update session
		session.TokenCount += tokenCount
		c.sessions.Update(ctx, session)
	}()

	return events, nil
}

// ListTransports returns list of configured transports
func (c *DefaultChatAgent) ListTransports() []TransportInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	infos := make([]TransportInfo, 0, len(c.transports))
	for name, t := range c.transports {
		infos = append(infos, TransportInfo{
			Name:         name,
			Description:  t.Description(),
			Endpoint:     fmt.Sprintf("/chat/%s", name),
			Priority:     t.Priority(),
			Capabilities: t.Capabilities(),
			Healthy:      true, // Could do actual health check here
		})
	}
	return infos
}

// GetSessionManager returns the session manager
func (c *DefaultChatAgent) GetSessionManager() SessionManager {
	return c.sessions
}

// RegisterTransport registers a new transport
func (c *DefaultChatAgent) RegisterTransport(transport Transport) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.transports[transport.Name()]; exists {
		return fmt.Errorf("transport %s already registered", transport.Name())
	}

	c.transports[transport.Name()] = transport
	return nil
}

// GetTransport retrieves a transport by name
func (c *DefaultChatAgent) GetTransport(name string) (Transport, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	transport, exists := c.transports[name]
	return transport, exists
}

// ProcessMessage processes a message and returns a stream of events
func (c *DefaultChatAgent) ProcessMessage(ctx context.Context, sessionID string, message string) (<-chan ChatEvent, error) {
	return c.StreamResponse(ctx, sessionID, message)
}

// Configure configures the chat agent
func (c *DefaultChatAgent) Configure(config ChatAgentConfig) error {
	c.config = config
	return nil
}

// Initialize initializes the chat agent and registers with service discovery
func (c *DefaultChatAgent) Initialize(ctx context.Context) error {
	// Start telemetry span if available
	var span core.Span
	if c.BaseAgent.Telemetry != nil {
		ctx, span = c.BaseAgent.Telemetry.StartSpan(ctx, "chat.agent.initialize")
		defer span.End()
	}

	// First call base initialization
	if err := c.BaseAgent.Initialize(ctx); err != nil {
		if span != nil {
			span.RecordError(err)
		}
		return err
	}

	// Register with discovery if available
	if c.BaseAgent.Discovery != nil && c.BaseAgent.Config != nil && c.BaseAgent.Config.Discovery.Enabled {
		// Build metadata for service discovery
		metadata := c.buildServiceMetadata()

		// Get address and port from BaseAgent config
		address, port := core.ResolveServiceAddress(c.BaseAgent.Config, c.BaseAgent.Logger)

		registration := &core.ServiceInfo{
			ID:           c.BaseAgent.ID,
			Name:         c.BaseAgent.Name,
			Type:         core.ComponentTypeAgent,
			Address:      address,
			Port:         port,
			Capabilities: c.BaseAgent.Capabilities,
			Health:       core.HealthHealthy,
			LastSeen:     time.Now(),
			Metadata:     metadata,
		}

		if err := c.BaseAgent.Discovery.Register(ctx, registration); err != nil {
			c.BaseAgent.Logger.Error("Failed to register chat agent with discovery", map[string]interface{}{
				"error":      err.Error(),
				"agent_id":   c.BaseAgent.ID,
				"agent_name": c.BaseAgent.Name,
			})
			// Continue anyway - graceful degradation
		} else {
			c.BaseAgent.Logger.Info("Chat agent registered with discovery", map[string]interface{}{
				"agent_id":   c.BaseAgent.ID,
				"agent_name": c.BaseAgent.Name,
				"address":    address,
				"port":       port,
			})
		}
	}

	// Validate configuration
	if err := c.validateConfig(); err != nil {
		return err
	}

	// Start health monitoring if we have transports
	if len(c.transports) > 0 {
		c.startHealthMonitoring(ctx)
	}

	return nil
}

// Start starts the HTTP server on the specified port
func (c *DefaultChatAgent) Start(port int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create HTTP server
	mux := http.NewServeMux()

	// Register transport endpoints
	for name, transport := range c.transports {
		handler := transport.CreateHandler(c)
		if handler != nil {
			// Create endpoint path based on transport name
			endpoint := fmt.Sprintf("/chat/%s", name)
			mux.Handle(endpoint, handler)
		}
	}

	// Register discovery and health endpoints
	mux.HandleFunc("/chat/transports", c.HandleTransportDiscovery)
	mux.HandleFunc("/chat/health", c.HandleHealth)

	// Create server
	c.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start server
	return c.server.ListenAndServe()
}

// buildServiceMetadata builds metadata for service discovery
func (c *DefaultChatAgent) buildServiceMetadata() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	transportList := make([]string, 0, len(c.transports))
	for name := range c.transports {
		transportList = append(transportList, name)
	}

	metadata := map[string]interface{}{
		"type":         "chat_agent",
		"transports":   transportList,
		"rate_limit":   c.config.SecurityConfig.RateLimit,
		"max_msg_size": c.config.SecurityConfig.MaxMessageSize,
		"require_auth": c.config.SecurityConfig.RequireAuth,
	}

	// Add session backend type
	if c.sessions != nil {
		switch c.sessions.(type) {
		case *RedisSessionManager:
			metadata["session_backend"] = "redis"
		case *MockSessionManager:
			metadata["session_backend"] = "memory"
		default:
			metadata["session_backend"] = "custom"
		}
	}

	return metadata
}

// validateConfig validates the chat agent configuration
func (c *DefaultChatAgent) validateConfig() error {
	if c.config.SecurityConfig.RateLimit < 0 {
		return fmt.Errorf("invalid rate limit: %d (must be >= 0)", c.config.SecurityConfig.RateLimit)
	}

	if c.config.SecurityConfig.MaxMessageSize < 0 {
		return fmt.Errorf("invalid max message size: %d (must be >= 0)", c.config.SecurityConfig.MaxMessageSize)
	}

	if c.config.SessionConfig.TTL <= 0 {
		return fmt.Errorf("invalid session TTL: %v (must be > 0)", c.config.SessionConfig.TTL)
	}

	if c.config.SessionConfig.MaxMessages <= 0 {
		return fmt.Errorf("invalid max messages: %d (must be > 0)", c.config.SessionConfig.MaxMessages)
	}

	if c.config.SessionConfig.RateLimitMax < 0 {
		return fmt.Errorf("invalid rate limit max: %d (must be >= 0)", c.config.SessionConfig.RateLimitMax)
	}

	if c.config.SessionConfig.RateLimitWindow <= 0 {
		return fmt.Errorf("invalid rate limit window: %v (must be > 0)", c.config.SessionConfig.RateLimitWindow)
	}

	return nil
}

// startHealthMonitoring starts background health monitoring for transports
func (c *DefaultChatAgent) startHealthMonitoring(ctx context.Context) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.checkTransportHealth(ctx)
			case <-c.stopChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// checkTransportHealth checks the health of all transports
func (c *DefaultChatAgent) checkTransportHealth(ctx context.Context) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for name, transport := range c.transports {
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := transport.HealthCheck(checkCtx)
		cancel()

		if err != nil {
			c.BaseAgent.Logger.Warn("Transport health check failed", map[string]interface{}{
				"transport": name,
				"error":     err.Error(),
			})

			// Record telemetry if available
			if c.BaseAgent.Telemetry != nil {
				c.BaseAgent.Telemetry.RecordMetric("transport.health.failed", 1.0, map[string]string{
					"transport": name,
				})
			}
		}
	}
}

// Stop stops the chat agent and all transports with graceful shutdown
func (c *DefaultChatAgent) Stop(ctx context.Context) error {
	c.mu.Lock()

	// Check if already stopping
	if c.stopping {
		c.mu.Unlock()
		return nil
	}
	c.stopping = true

	// Signal all goroutines to stop
	close(c.stopChan)
	c.mu.Unlock()

	// Stop all transports in parallel
	var wg sync.WaitGroup
	for name, transport := range c.transports {
		wg.Add(1)
		go func(n string, t Transport) {
			defer wg.Done()

			// Create timeout context for transport shutdown
			transportCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			if err := t.Stop(transportCtx); err != nil {
				c.BaseAgent.Logger.Error("Failed to stop transport", map[string]interface{}{
					"transport": n,
					"error":     err.Error(),
				})
			}
		}(name, transport)
	}

	// Wait for all transports to stop or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All transports stopped successfully
	case <-ctx.Done():
		// Context cancelled, forced shutdown
		c.BaseAgent.Logger.Warn("Forced shutdown due to context cancellation", nil)
	}

	// Stop session manager if it supports graceful shutdown
	if closer, ok := c.sessions.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			c.BaseAgent.Logger.Error("Failed to close session manager", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Wait for all background goroutines to finish
	c.wg.Wait()

	// Unregister from discovery if available
	if c.BaseAgent.Discovery != nil && c.BaseAgent.Config != nil && c.BaseAgent.Config.Discovery.Enabled {
		unregCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := c.BaseAgent.Discovery.Unregister(unregCtx, c.BaseAgent.ID); err != nil {
			c.BaseAgent.Logger.Error("Failed to unregister from discovery", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Call base stop
	return c.BaseAgent.Stop(ctx)
}

// Shutdown is an alias for Stop for backward compatibility
func (c *DefaultChatAgent) Shutdown(ctx context.Context) error {
	return c.Stop(ctx)
}
