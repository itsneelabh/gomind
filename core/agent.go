package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Agent is the core interface that all agents must implement
type Agent interface {
	Initialize(ctx context.Context) error
	GetID() string
	GetName() string
	GetCapabilities() []Capability
}

// Capability represents a capability that an agent provides
type Capability struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Endpoint    string   `json:"endpoint"`
	InputTypes  []string `json:"input_types"`
	OutputTypes []string `json:"output_types"`
}

// BaseAgent provides the core agent functionality (Tool Builder Kit)
// This is the minimal implementation for building "tools" that AI agents can discover
type BaseAgent struct {
	// Core fields (always available)
	ID           string
	Name         string
	Capabilities []Capability
	Logger       Logger
	Discovery    Discovery
	Memory       Memory

	// Optional fields (set by modules)
	Telemetry Telemetry
	AI        AIClient

	// Configuration
	Config *Config

	// HTTP server
	server *http.Server
	mux    *http.ServeMux
	
	// Handler registration tracking
	registeredPatterns map[string]bool  // Track registered patterns to prevent duplicates
	serverStarted      bool             // Track if server has started
	mu                 sync.RWMutex     // Protect concurrent access
}

// NewBaseAgent creates a new base agent with minimal dependencies
func NewBaseAgent(name string) *BaseAgent {
	return NewBaseAgentWithConfig(&Config{
		Name: name,
	})
}

// NewBaseAgentWithConfig creates a new base agent with configuration
func NewBaseAgentWithConfig(config *Config) *BaseAgent {
	if config == nil {
		config = DefaultConfig()
	}
	
	// Ensure name is set
	if config.Name == "" {
		config.Name = "gomind-agent"
	}
	
	// Generate ID if not set
	if config.ID == "" {
		config.ID = fmt.Sprintf("%s-%s", config.Name, uuid.New().String()[:8])
	}
	
	return &BaseAgent{
		ID:                 config.ID,
		Name:               config.Name,
		Capabilities:       []Capability{},
		Logger:             &NoOpLogger{},  // Will be initialized based on config
		Memory:             NewInMemoryStore(),  // Will be initialized based on config
		Telemetry:          &NoOpTelemetry{},  // Will be initialized based on config
		Config:             config,
		mux:                http.NewServeMux(),
		registeredPatterns: make(map[string]bool),
		serverStarted:      false,
	}
}

// Initialize initializes the agent
func (b *BaseAgent) Initialize(ctx context.Context) error {
	b.Logger.Info("Initializing agent", map[string]interface{}{
		"id":   b.ID,
		"name": b.Name,
	})

	// Initialize components based on config
	if b.Config != nil {
		// Initialize discovery if configured
		if b.Config.Discovery.Enabled && b.Discovery == nil {
			if b.Config.Development.MockDiscovery {
				// Use mock discovery for development
				b.Discovery = NewMockDiscovery()
			} else if b.Config.Discovery.Provider == "redis" && b.Config.Discovery.RedisURL != "" {
				// Initialize Redis discovery
				if discovery, err := NewRedisDiscovery(b.Config.Discovery.RedisURL); err == nil {
					b.Discovery = discovery
				} else {
					b.Logger.Error("Failed to initialize Redis discovery", map[string]interface{}{
						"error": err.Error(),
					})
				}
			}
		}
		
		// Initialize memory based on config
		if b.Config.Memory.Provider == "redis" && b.Config.Memory.RedisURL != "" {
			// TODO: Initialize Redis memory when available
			b.Memory = NewInMemoryStore()
		} else {
			b.Memory = NewInMemoryStore()
		}
	}

	// Register with discovery if available
	if b.Discovery != nil && b.Config.Discovery.Enabled {
		capabilities := make([]string, len(b.Capabilities))
		for i, cap := range b.Capabilities {
			capabilities[i] = cap.Name
		}

		address := b.Config.Address
		if address == "" {
			address = "localhost"
		}

		registration := &ServiceRegistration{
			ID:           b.ID,
			Name:         b.Name,
			Address:      address,
			Port:         b.Config.Port,
			Capabilities: capabilities,
			Health:       HealthHealthy,
			LastSeen:     time.Now(),
			Metadata: map[string]string{
				"namespace": b.Config.Namespace,
			},
		}

		if b.Config.Kubernetes.Enabled {
			registration.Metadata["pod_name"] = b.Config.Kubernetes.PodName
			registration.Metadata["pod_namespace"] = b.Config.Kubernetes.PodNamespace
			registration.Metadata["service_name"] = b.Config.Kubernetes.ServiceName
		}

		if err := b.Discovery.Register(ctx, registration); err != nil {
			b.Logger.Error("Failed to register with discovery", map[string]interface{}{
				"error": err.Error(),
			})
			// Continue anyway - graceful degradation
		}
	}

	return nil
}

// GetID returns the agent ID
func (b *BaseAgent) GetID() string {
	return b.ID
}

// GetName returns the agent name
func (b *BaseAgent) GetName() string {
	return b.Name
}

// GetCapabilities returns the agent capabilities
func (b *BaseAgent) GetCapabilities() []Capability {
	return b.Capabilities
}

// HandleFunc registers a custom HTTP handler for the given pattern.
// This method must be called before Start() is invoked.
// It returns an error if:
//   - The server has already been started
//   - The pattern has already been registered
//
// Example:
//
//	agent := core.NewBaseAgent("my-agent")
//	err := agent.HandleFunc("/api/custom", myHandler)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (b *BaseAgent) HandleFunc(pattern string, handler http.HandlerFunc) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Check if server has already started
	if b.serverStarted {
		return fmt.Errorf("cannot register handler for pattern %s: server already started", pattern)
	}
	
	// Check for duplicate pattern registration
	if b.registeredPatterns[pattern] {
		return fmt.Errorf("handler already registered for pattern: %s", pattern)
	}
	
	// Register the handler
	b.mux.HandleFunc(pattern, handler)
	b.registeredPatterns[pattern] = true
	
	// Log the registration
	b.Logger.Info("Registered custom handler", map[string]interface{}{
		"pattern": pattern,
	})
	
	return nil
}

// RegisterCapability registers a new capability
func (b *BaseAgent) RegisterCapability(cap Capability) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.Capabilities = append(b.Capabilities, cap)

	// Register HTTP endpoint for the capability
	endpoint := fmt.Sprintf("/api/capabilities/%s", cap.Name)
	b.mux.HandleFunc(endpoint, b.handleCapabilityRequest(cap))
	
	// Track this pattern internally (capabilities are system-managed, not custom)
	b.registeredPatterns[endpoint] = true

	b.Logger.Info("Registered capability", map[string]interface{}{
		"name":     cap.Name,
		"endpoint": endpoint,
	})
}

// handleCapabilityRequest creates an HTTP handler for a capability
func (b *BaseAgent) handleCapabilityRequest(cap Capability) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Start telemetry span if available
		if b.Telemetry != nil {
			var span Span
			_, span = b.Telemetry.StartSpan(ctx, fmt.Sprintf("capability.%s", cap.Name))
			defer span.End()
			span.SetAttribute("capability.name", cap.Name)
		}

		// Log request
		b.Logger.Info("Handling capability request", map[string]interface{}{
			"capability": cap.Name,
			"method":     r.Method,
		})

		// Parse request
		var input map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			b.Logger.Error("Failed to parse request", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// TODO: Actual capability implementation would go here
		// This is where tool-specific logic would be implemented

		// Return response
		response := map[string]interface{}{
			"capability": cap.Name,
			"status":     "success",
			"result":     "Tool capability executed successfully",
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			// Log error but response is already partially written
			if b.Logger != nil {
				b.Logger.Error("Failed to encode response", map[string]interface{}{"error": err})
			}
		}
	}
}

// Start starts the HTTP server for the agent
func (b *BaseAgent) Start(port int) error {
	b.mu.Lock()
	
	// Check if already started
	if b.serverStarted {
		b.mu.Unlock()
		return fmt.Errorf("server already started")
	}
	
	if b.Config != nil && b.Config.Port != 0 {
		port = b.Config.Port
	}
	
	addr := fmt.Sprintf("%s:%d", b.Config.Address, port)
	if b.Config.Address == "" {
		addr = fmt.Sprintf(":%d", port)
	}

	// Add health endpoint if enabled
	if b.Config.HTTP.EnableHealthCheck {
		healthPath := b.Config.HTTP.HealthCheckPath
		// Check if health path is already registered (shouldn't be, but be safe)
		if !b.registeredPatterns[healthPath] {
			b.mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(map[string]string{
					"status": "healthy",
					"agent":  b.Name,
					"id":     b.ID,
				}); err != nil {
					// Log error but response is already partially written
					if b.Logger != nil {
						b.Logger.Error("Failed to encode health response", map[string]interface{}{"error": err})
					}
				}
			})
			b.registeredPatterns[healthPath] = true
		}
	}

	// Add capabilities listing endpoint
	capabilitiesPath := "/api/capabilities"
	if !b.registeredPatterns[capabilitiesPath] {
		b.mux.HandleFunc(capabilitiesPath, func(w http.ResponseWriter, r *http.Request) {
			ApplyCORS(w, r, &b.Config.HTTP.CORS)
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(b.Capabilities); err != nil {
				// Log error but response is already partially written
				if b.Logger != nil {
					b.Logger.Error("Failed to encode capabilities", map[string]interface{}{"error": err})
				}
			}
		})
		b.registeredPatterns[capabilitiesPath] = true
	}

	// Create handler with CORS middleware if enabled
	var handler http.Handler = b.mux
	if b.Config.HTTP.CORS.Enabled {
		handler = CORSMiddleware(&b.Config.HTTP.CORS)(handler)
	}

	b.server = &http.Server{
		Addr:           addr,
		Handler:        handler,
		ReadTimeout:    b.Config.HTTP.ReadTimeout,
		WriteTimeout:   b.Config.HTTP.WriteTimeout,
		IdleTimeout:    b.Config.HTTP.IdleTimeout,
		MaxHeaderBytes: b.Config.HTTP.MaxHeaderBytes,
	}

	// Mark server as started (before actually starting to prevent race conditions)
	b.serverStarted = true
	b.mu.Unlock()  // Unlock before blocking ListenAndServe call
	
	b.Logger.Info("Starting HTTP server", map[string]interface{}{
		"address": addr,
		"cors":    b.Config.HTTP.CORS.Enabled,
	})

	return b.server.ListenAndServe()
}

// Stop stops the HTTP server
func (b *BaseAgent) Stop(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if b.server != nil {
		// Use configured shutdown timeout or context deadline
		shutdownCtx := ctx
		if b.Config != nil && b.Config.HTTP.ShutdownTimeout > 0 {
			var cancel context.CancelFunc
			shutdownCtx, cancel = context.WithTimeout(ctx, b.Config.HTTP.ShutdownTimeout)
			defer cancel()
		}
		
		// Unregister from discovery if available
		if b.Discovery != nil && b.Config.Discovery.Enabled {
			if err := b.Discovery.Unregister(shutdownCtx, b.ID); err != nil {
				b.Logger.Error("Failed to unregister from discovery", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}
		
		// Reset server state
		b.serverStarted = false
		
		return b.server.Shutdown(shutdownCtx)
	}
	return nil
}

// Framework provides a simple way to run agents
type Framework struct {
	agent  Agent
	config *Config
}

// NewFramework creates a new framework instance with options
func NewFramework(agent Agent, opts ...Option) (*Framework, error) {
	// Create configuration with options
	config, err := NewConfig(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}
	
	// If agent is a BaseAgent, update its config
	if base, ok := agent.(*BaseAgent); ok {
		base.Config = config
		base.Name = config.Name
		if config.ID != "" {
			base.ID = config.ID
		}
	}
	
	return &Framework{
		agent:  agent,
		config: config,
	}, nil
}

// Run initializes and starts the agent
func (f *Framework) Run(ctx context.Context) error {
	// Initialize agent
	if err := f.agent.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// Start HTTP server if BaseAgent
	if base, ok := f.agent.(*BaseAgent); ok {
		return base.Start(f.config.Port)
	}

	// For custom agents, they need to implement their own server
	<-ctx.Done()
	return ctx.Err()
}
