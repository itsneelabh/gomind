package framework

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

	"github.com/itsneelabh/gomind/internal/chat"
	"github.com/itsneelabh/gomind/internal/conversation"
	"github.com/itsneelabh/gomind/internal/port"
	"github.com/itsneelabh/gomind/pkg/agent"
	aiPkg "github.com/itsneelabh/gomind/pkg/ai"
	capabilitiesPkg "github.com/itsneelabh/gomind/pkg/capabilities"
	communicationPkg "github.com/itsneelabh/gomind/pkg/communication"
	discoveryPkg "github.com/itsneelabh/gomind/pkg/discovery"
	loggerPkg "github.com/itsneelabh/gomind/pkg/logger"
	memoryPkg "github.com/itsneelabh/gomind/pkg/memory"
	routingPkg "github.com/itsneelabh/gomind/pkg/routing"
	telemetryPkg "github.com/itsneelabh/gomind/pkg/telemetry"
)

// RunAgent is the zero-config entry point that automatically configures and runs an agent
func RunAgent(agent agent.Agent, options ...Option) error {
	ctx := context.Background()

	// Create framework instance
	fw, err := NewFramework(options...)
	if err != nil {
		return fmt.Errorf("failed to create framework: %w", err)
	}
	defer fw.Shutdown(ctx)

	// Initialize the agent
	if err := fw.InitializeAgent(ctx, agent); err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// Start HTTP server
	return fw.StartHTTPServer(ctx, agent)
}

// Framework provides the core GoMind framework functionality
type Framework struct {
	agentID             string
	registrationID      string
	discovery           discoveryPkg.Discovery
	memory              memoryPkg.Memory
	logger              loggerPkg.Logger
	telemetry           telemetryPkg.AutoOTEL
	httpServer          *http.Server
	portManager         *port.PortManager
	config              *Config
	aiClient            aiPkg.AIClient
	conversationManager *conversation.ConversationConnectionManager
	customHandlers      map[string]http.HandlerFunc
	capabilities        []CapabilityMetadata // Store discovered capabilities
	router              routingPkg.Router    // Routing system for multi-agent orchestration
}

// Config holds framework configuration
type Config struct {
	AgentName     string
	Port          int    // Legacy: kept for backward compatibility
	Environment   string // Runtime environment (dev, staging, prod)
	RedisURL      string
	DiscoveryNS   string
	MemoryNS      string
	LogLevel      string
	MetricsPath   string // Path for metrics endpoint (default: /metrics)
	OTELEndpoint  string
	EnableMetrics bool
	EnableDocs    bool
	// AI Configuration
	OpenAIAPIKey   string
	AIProvider     string // "openai", "claude", "ollama"
	DefaultAIModel string
	EnableAI       bool
	// Discovery registration scope: "service" (default) or "pod"
	RegistrationScope string
	// Kubernetes Service-facing configuration (used when RegistrationScope=service)
	AgentServiceName string
	ServicePort      int
	// Discovery cache and readiness configuration
	DiscoveryCacheEnabled           bool
	DiscoveryCacheRefreshInterval   time.Duration
	DiscoveryCachePersistEnabled    bool
	DiscoveryCachePersistPath       string
	DiscoveryCacheBackoffInitial    time.Duration
	DiscoveryCacheBackoffMax        time.Duration
	DiscoveryCacheCBThreshold       int
	DiscoveryCacheCBCooldown        time.Duration
	DiscoveryCacheWarnStale         time.Duration
	DiscoveryStrictStartup          bool
	DiscoveryRequireSnapshotAtStart bool
	DiscoveryMinSnapshotSize        int
	// Routing configuration
	RoutingMode                string        // "autonomous", "workflow", "hybrid"
	WorkflowPath               string        // Path to workflow YAML files
	RoutingCacheEnabled        bool          // Enable routing cache
	RoutingCacheTTL            time.Duration // Cache TTL for routing decisions
	RoutingConfidenceThreshold float64       // Minimum confidence for autonomous routing
	RoutingPreferWorkflow      bool          // Prefer workflow over autonomous in hybrid mode
	RoutingEnhanceWithLLM      bool          // Enhance workflow plans with LLM
}

// Option represents a configuration option
type Option func(*Config)

// WithAgentName sets the agent name
func WithAgentName(name string) Option {
	return func(c *Config) {
		c.AgentName = name
	}
}

// WithPort sets the HTTP port (also sets PORT environment variable for PortManager)
func WithPort(port int) Option {
	return func(c *Config) {
		c.Port = port
		// Set environment variable so PortManager uses this port
		os.Setenv("PORT", fmt.Sprintf("%d", port))
	}
}

// WithRedisURL sets the Redis URL
func WithRedisURL(url string) Option {
	return func(c *Config) {
		c.RedisURL = url
	}
}

// NewFramework creates a new framework instance with auto-configuration
func NewFramework(options ...Option) (*Framework, error) {
	// Default configuration with environment variables
	config := &Config{
		AgentName:     getEnvOrDefault("AGENT_NAME", "gomind-agent"),
		Port:          getEnvIntOrDefault("PORT", 8080), // Legacy support
		RedisURL:      getEnvOrDefault("REDIS_URL", "redis://localhost:6379"),
		DiscoveryNS:   getEnvOrDefault("REDIS_DISCOVERY_NAMESPACE", "agents"),
		MemoryNS:      getEnvOrDefault("REDIS_MEMORY_NAMESPACE", "memory"),
		LogLevel:      getEnvOrDefault("LOG_LEVEL", "INFO"),
		OTELEndpoint:  os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		EnableMetrics: getEnvBoolOrDefault("ENABLE_METRICS", true),
		EnableDocs:    getEnvBoolOrDefault("ENABLE_DOCS", true),
		// AI Configuration
		OpenAIAPIKey:   os.Getenv("OPENAI_API_KEY"),
		AIProvider:     getEnvOrDefault("AI_PROVIDER", "openai"),
		DefaultAIModel: getEnvOrDefault("DEFAULT_AI_MODEL", "gpt-4"),
		EnableAI:       getEnvBoolOrDefault("ENABLE_AI", false),
		// Discovery registration (sane defaults)
		RegistrationScope: strings.ToLower(getEnvOrDefault("DISCOVERY_REGISTRATION_SCOPE", "service")),
		AgentServiceName:  getEnvOrDefault("AGENT_SERVICE_NAME", ""),
		ServicePort:       getEnvIntOrDefault("SERVICE_PORT", 0),
		// Discovery cache defaults
		DiscoveryCacheEnabled:           getEnvBoolOrDefault("DISCOVERY_CACHE_ENABLED", true),
		DiscoveryCacheRefreshInterval:   getEnvDurationOrDefault("DISCOVERY_CACHE_REFRESH_INTERVAL", 15*time.Second),
		DiscoveryCachePersistEnabled:    getEnvBoolOrDefault("DISCOVERY_CACHE_PERSIST_ENABLED", false),
		DiscoveryCachePersistPath:       getEnvOrDefault("DISCOVERY_CACHE_PERSIST_PATH", "/data/discovery_snapshot.json"),
		DiscoveryCacheBackoffInitial:    getEnvDurationOrDefault("DISCOVERY_CACHE_BACKOFF_INITIAL", 1*time.Second),
		DiscoveryCacheBackoffMax:        getEnvDurationOrDefault("DISCOVERY_CACHE_BACKOFF_MAX", 60*time.Second),
		DiscoveryCacheCBThreshold:       getEnvIntOrDefault("DISCOVERY_CACHE_CB_THRESHOLD", 5),
		DiscoveryCacheCBCooldown:        getEnvDurationOrDefault("DISCOVERY_CACHE_CB_COOLDOWN", 2*time.Minute),
		DiscoveryCacheWarnStale:         getEnvDurationOrDefault("DISCOVERY_CACHE_WARN_STALE", 10*time.Minute),
		DiscoveryStrictStartup:          getEnvBoolOrDefault("DISCOVERY_STRICT_STARTUP", false),
		DiscoveryRequireSnapshotAtStart: getEnvBoolOrDefault("DISCOVERY_REQUIRE_SNAPSHOT_AT_STARTUP", false),
		DiscoveryMinSnapshotSize:        getEnvIntOrDefault("DISCOVERY_MIN_SNAPSHOT_SIZE", 1),
		// Routing defaults
		RoutingMode:                getEnvOrDefault("ROUTING_MODE", "hybrid"),
		WorkflowPath:               getEnvOrDefault("WORKFLOW_PATH", "./workflows"),
		RoutingCacheEnabled:        getEnvBoolOrDefault("ROUTING_CACHE_ENABLED", true),
		RoutingCacheTTL:            getEnvDurationOrDefault("ROUTING_CACHE_TTL", 10*time.Minute),
		RoutingConfidenceThreshold: getEnvFloatOrDefault("ROUTING_CONFIDENCE_THRESHOLD", 0.7),
		RoutingPreferWorkflow:      getEnvBoolOrDefault("ROUTING_PREFER_WORKFLOW", true),
		RoutingEnhanceWithLLM:      getEnvBoolOrDefault("ROUTING_ENHANCE_WITH_LLM", false),
	}

	// Apply options
	for _, opt := range options {
		opt(config)
	}

	// Create logger first
	logger := loggerPkg.NewSimpleLogger()
	logger.SetLevel(config.LogLevel)

	// Initialize port manager with environment-aware configuration
	portManager := port.NewPortManager(logger)

	// Initialize telemetry (OpenTelemetry)
	telemetry, err := telemetryPkg.NewAutoOTEL(config.AgentName, "", []string{})
	if err != nil {
		logger.Error("Failed to initialize telemetry", map[string]interface{}{
			"error": err.Error(),
		})
		// Continue without telemetry - graceful degradation
	}

	// Initialize discovery service
	var discoveryService discoveryPkg.Discovery
	discoveryService, err = discoveryPkg.NewRedisDiscovery(config.RedisURL, config.AgentName, config.DiscoveryNS)
	if err != nil {
		logger.Warn("Failed to initialize Redis discovery, agent will run in standalone mode", map[string]interface{}{
			"error": err.Error(),
		})
		discoveryService = nil // Explicitly set to nil for graceful degradation
	}

	// Initialize memory service with fallback
	var memoryService memoryPkg.Memory
	if config.RedisURL != "" {
		memoryService, err = memoryPkg.NewRedisMemory(config.RedisURL, config.MemoryNS)
		if err != nil {
			logger.Warn("Failed to initialize Redis memory, falling back to in-memory storage", map[string]interface{}{
				"error": err.Error(),
			})
			memoryService = memoryPkg.NewInMemoryStore()
		}
	} else {
		memoryService = memoryPkg.NewInMemoryStore()
	}

	// Initialize AI client if API key provided
	var aiClient aiPkg.AIClient
	if config.OpenAIAPIKey != "" && config.EnableAI {
		switch config.AIProvider {
		case "openai", "":
			aiClient = aiPkg.NewOpenAIClient(config.OpenAIAPIKey, logger)
			logger.Info("AI client initialized", map[string]interface{}{
				"provider": "openai",
				"model":    config.DefaultAIModel,
			})
		default:
			logger.Warn("Unsupported AI provider", map[string]interface{}{
				"provider": config.AIProvider,
			})
		}
	} else if config.OpenAIAPIKey != "" && !config.EnableAI {
		logger.Info("AI capabilities available but disabled - set ENABLE_AI=true to activate", nil)
	}

	// Initialize router if AI is enabled
	var router routingPkg.Router
	if aiClient != nil && config.EnableAI {
		switch config.RoutingMode {
		case "autonomous":
			router = routingPkg.NewAutonomousRouter(aiClient,
				routingPkg.WithModel(config.DefaultAIModel),
				routingPkg.WithCacheTTL(config.RoutingCacheTTL))
			logger.Info("Autonomous router initialized", nil)
			
		case "workflow":
			workflowRouter, err := routingPkg.NewWorkflowRouter(config.WorkflowPath,
				routingPkg.WithWorkflowCacheTTL(config.RoutingCacheTTL))
			if err != nil {
				logger.Warn("Failed to initialize workflow router", map[string]interface{}{
					"error": err.Error(),
				})
			} else {
				router = workflowRouter
				logger.Info("Workflow router initialized", map[string]interface{}{
					"path": config.WorkflowPath,
				})
			}
			
		case "hybrid", "":
			hybridRouter, err := routingPkg.NewHybridRouter(config.WorkflowPath, aiClient,
				routingPkg.WithPreferWorkflow(config.RoutingPreferWorkflow),
				routingPkg.WithEnhancement(config.RoutingEnhanceWithLLM),
				routingPkg.WithConfidenceThreshold(config.RoutingConfidenceThreshold))
			if err != nil {
				logger.Warn("Failed to initialize hybrid router, falling back to autonomous", map[string]interface{}{
					"error": err.Error(),
				})
				router = routingPkg.NewAutonomousRouter(aiClient)
			} else {
				router = hybridRouter
				logger.Info("Hybrid router initialized", map[string]interface{}{
					"prefer_workflow": config.RoutingPreferWorkflow,
					"enhance_llm":     config.RoutingEnhanceWithLLM,
				})
			}
			
		default:
			logger.Warn("Unknown routing mode, router not initialized", map[string]interface{}{
				"mode": config.RoutingMode,
			})
		}
	}

	return &Framework{
		discovery:           discoveryService,
		memory:              memoryService,
		logger:              logger,
		telemetry:           telemetry,
		portManager:         portManager,
		config:              config,
		aiClient:            aiClient,
		conversationManager: conversation.NewConversationConnectionManager(),
		customHandlers:      make(map[string]http.HandlerFunc),
		router:              router,
	}, nil
}

// New creates a framework instance with explicit configuration (for direct control pattern)
func New(config Config) (*Framework, error) {
	options := []Option{
		WithAgentName(config.AgentName),
		WithPort(config.Port),
		WithRedisURL(config.RedisURL),
	}

	return NewFramework(options...)
}

// InitializeAgent initializes the agent with framework services
func (f *Framework) InitializeAgent(ctx context.Context, agent agent.Agent) error {
	// Generate agent ID first
	f.agentID = fmt.Sprintf("%s-%d", f.config.AgentName, time.Now().Unix())

	// Auto-discover capabilities using reflection
	capabilities := f.discoverCapabilities(agent)
	f.capabilities = capabilities // Store for later use

	// Inject dependencies and capabilities into BaseAgent
	// Using reflection to handle embedded BaseAgent fields
	agentVal := reflect.ValueOf(agent)
	if agentVal.Kind() == reflect.Ptr {
		agentVal = agentVal.Elem()
	}
	
	// Create communicator for inter-agent communication
	var communicator communicationPkg.AgentCommunicator
	if f.discovery != nil {
		namespace := getEnvOrDefault("NAMESPACE", "default")
		communicator = communicationPkg.NewK8sCommunicator(f.discovery, f.logger, namespace)
		
		// Set additional configuration if needed
		if k8sComm, ok := communicator.(*communicationPkg.K8sCommunicator); ok {
			if port := f.config.ServicePort; port > 0 {
				k8sComm.SetServicePort(port)
			}
		}
	}
	
	// Look for embedded BaseAgent field
	if agentVal.Kind() == reflect.Struct {
		for i := 0; i < agentVal.NumField(); i++ {
			field := agentVal.Field(i)
			if field.Type() == reflect.TypeOf(BaseAgent{}) && field.CanSet() {
				// Found embedded BaseAgent, set its fields
				baseAgent := field.Addr().Interface().(*BaseAgent)
				baseAgent.discovery = f.discovery
				baseAgent.memory = f.memory
				baseAgent.logger = f.logger
				baseAgent.telemetry = f.telemetry
				baseAgent.aiClient = f.aiClient
				baseAgent.agentID = f.agentID
				baseAgent.capabilities = capabilities
				baseAgent.communicator = communicator
				break
			}
		}
	}
	
	// Also handle direct *BaseAgent type
	if baseAgent, ok := agent.(*BaseAgent); ok {
		baseAgent.discovery = f.discovery
		baseAgent.memory = f.memory
		baseAgent.logger = f.logger
		baseAgent.telemetry = f.telemetry
		baseAgent.aiClient = f.aiClient
		baseAgent.agentID = f.agentID
		baseAgent.capabilities = capabilities
		baseAgent.communicator = communicator
	}

	// Update discovery with the proper agent ID if discovery is initialized
	if f.discovery != nil {
		if redisDiscovery, ok := f.discovery.(*discoveryPkg.RedisDiscovery); ok {
			redisDiscovery.SetAgentID(f.agentID)
		}
	}

	// Register agent with discovery service
	if f.discovery != nil {
		// If Redis discovery, configure cache refresher & persistence and start it
		if rd, ok := f.discovery.(*discoveryPkg.RedisDiscovery); ok {
			rd.ConfigureCache(
				f.config.DiscoveryCacheEnabled,
				f.config.DiscoveryCacheRefreshInterval,
				f.config.DiscoveryCacheBackoffInitial,
				f.config.DiscoveryCacheBackoffMax,
				f.config.DiscoveryCacheCBThreshold,
				f.config.DiscoveryCacheCBCooldown,
				f.config.DiscoveryCacheWarnStale,
			)
			rd.ConfigurePersistence(f.config.DiscoveryCachePersistEnabled, f.config.DiscoveryCachePersistPath)
			// Load snapshot if available (for startup readiness)
			_ = rd.LoadSnapshot()
			// Start background refresh loop (independent of heartbeat)
			go rd.StartBackgroundRefresh(ctx)
			// Phase 2: Start catalog sync for agent discovery
			rd.StartCatalogSync(ctx, 30*time.Second)
		}
		// Determine registration scope and network address
		scope := strings.ToLower(f.config.RegistrationScope)

		var regID string
		var host string
		var port int

		if scope == "service" {
			regID = f.config.AgentName // single logical entry per agent
			// Derive service name and port with sensible defaults
			svcName := f.config.AgentServiceName
			if svcName == "" {
				svcName = f.config.AgentName
			}
			host = svcName
			if f.config.ServicePort > 0 {
				port = f.config.ServicePort
			} else {
				port = f.config.Port
			}
		} else { // pod scope (per-instance)
			regID = f.agentID
			host = getEnvOrDefault("POD_IP", "localhost")
			port = f.config.Port
		}

		f.registrationID = regID

		// Convert capabilities to discovery format
		discoveryCapabilities := f.convertToDiscoveryCapabilities(capabilities)
		
		// Phase 2: Enhance registration with K8s and NL fields
		namespace := getEnvOrDefault("NAMESPACE", getEnvOrDefault("POD_NAMESPACE", "default"))
		serviceName := f.config.AgentName
		serviceEndpoint := fmt.Sprintf("%s.%s.svc.cluster.local:%d", serviceName, namespace, port)
		
		// Generate natural language description
		description := fmt.Sprintf("%s agent with %d capabilities", f.config.AgentName, len(capabilities))
		examples := []string{}
		llmHints := ""
		
		// Extract hints from capabilities
		capNames := []string{}
		for _, cap := range capabilities {
			if cap.Description != "" && len(examples) < 3 {
				examples = append(examples, cap.Description)
			}
			capNames = append(capNames, cap.Name)
		}
		
		if len(capNames) > 0 {
			llmHints = fmt.Sprintf("Specializes in: %s", strings.Join(capNames, ", "))
		}

		registration := &discoveryPkg.AgentRegistration{
			ID:            regID,
			Name:          f.config.AgentName,
			Address:       host, // host only; Port carries the port
			Port:          port,
			Capabilities:  discoveryCapabilities,
			Status:        StatusHealthy,
			LastHeartbeat: time.Now(),
			// Phase 2 fields
			ServiceName:     serviceName,
			Namespace:       namespace,
			ServiceEndpoint: serviceEndpoint,
			Description:     description,
			Examples:        examples,
			LLMHints:        llmHints,
		}

		if err := f.discovery.Register(ctx, registration); err != nil {
			f.logger.Error("Failed to register agent", map[string]interface{}{
				"error": err.Error(),
			})
			// Continue without registration - graceful degradation
		} else {
			f.logger.Info("Agent registered successfully", map[string]interface{}{
				"agent_id":     f.agentID,
				"capabilities": len(capabilities),
			})

			// Start heartbeat goroutine for ongoing TTL refresh
			go f.startHeartbeat(ctx)
		}
	}

	// Initialize agent
	if err := agent.Initialize(ctx); err != nil {
		return fmt.Errorf("agent initialization failed: %w", err)
	}

	f.logger.Info("Agent initialized successfully", map[string]interface{}{
		"agent_id":     f.agentID,
		"agent_name":   f.config.AgentName,
		"capabilities": len(capabilities),
	})

	return nil
}

// discoverCapabilities automatically discovers agent capabilities using reflection and dual metadata system
func (f *Framework) discoverCapabilities(agent agent.Agent) []CapabilityMetadata {
	var capabilities []CapabilityMetadata

	agentType := reflect.TypeOf(agent)

	// Enhanced discovery using dual metadata system
	commentParser := capabilitiesPkg.NewCommentParser()
	yamlLoader := capabilitiesPkg.NewYAMLLoader()
	merger := capabilitiesPkg.NewMetadataMerger()

	// Extract metadata from comments
	commentMetadata, err := commentParser.ExtractFromComments(agentType)
	if err != nil {
		f.logger.Warn("Comment metadata extraction failed, continuing with other sources", map[string]interface{}{
			"error":     err,
			"agentType": agentType.Name(),
			"message":   "This is normal if agent source files are not accessible or agent uses YAML-only metadata",
		})
		commentMetadata = []CapabilityMetadata{} // Continue with empty metadata
	} else if len(commentMetadata) > 0 {
		f.logger.Info("Successfully extracted comment metadata", map[string]interface{}{
			"agentType":    agentType.Name(),
			"capabilities": len(commentMetadata),
		})
	}

	// Extract metadata from YAML files
	// Use current working directory or derive from agent package
	agentDir := "." // Simplified - in practice, derive from agent location
	yamlMetadata, err := yamlLoader.LoadMetadata(agentDir)
	if err != nil {
		f.logger.Warn("YAML metadata loading failed, continuing with other sources", map[string]interface{}{
			"error":    err,
			"agentDir": agentDir,
			"message":  "This is normal if no capabilities.yaml file exists or agent uses comment-only metadata",
		})
		yamlMetadata = []CapabilityMetadata{} // Continue with empty metadata
	} else if len(yamlMetadata) > 0 {
		f.logger.Info("Successfully loaded YAML metadata", map[string]interface{}{
			"agentDir":     agentDir,
			"capabilities": len(yamlMetadata),
		})
	}

	// Merge metadata from both sources
	if len(commentMetadata) > 0 || len(yamlMetadata) > 0 {
		capabilities = merger.MergeMetadata(commentMetadata, yamlMetadata)
		f.logger.Info("Enhanced capability discovery completed", map[string]interface{}{
			"comment_capabilities": len(commentMetadata),
			"yaml_capabilities":    len(yamlMetadata),
			"merged_capabilities":  len(capabilities),
		})
	}

	// Fallback to reflection-based discovery if no metadata found
	if len(capabilities) == 0 {
		f.logger.Info("No metadata found, falling back to reflection-based discovery", map[string]interface{}{
			"agentType": agentType.Name(),
		})
		capabilities = f.discoverCapabilitiesReflection(agent)
	} else {
		// Enhance with reflection data for capabilities that don't have Method info
		capabilities = f.enhanceWithReflection(agent, capabilities)
	}

	return capabilities
}

// discoverCapabilitiesReflection provides the original reflection-based discovery as fallback
func (f *Framework) discoverCapabilitiesReflection(agent agent.Agent) []CapabilityMetadata {
	var capabilities []CapabilityMetadata

	agentType := reflect.TypeOf(agent)

	// For pointer types, we want to scan methods on the pointer type
	// because methods with pointer receivers are associated with the pointer type

	// Scan for public methods that could be capabilities
	for i := 0; i < agentType.NumMethod(); i++ {
		method := agentType.Method(i)

		// Skip framework methods
		if isFrameworkMethod(method.Name) {
			continue
		}

		// Only consider exported methods
		if method.IsExported() {
			capability := CapabilityMetadata{
				Name:          toSnakeCase(method.Name),
				Description:   fmt.Sprintf("Auto-discovered capability: %s", method.Name),
				InputTypes:    []string{"json"},
				OutputFormats: []string{"json"},
				Method:        method,
				Source: &MetadataSource{
					Type:        "reflection",
					LastUpdated: time.Now().Format(time.RFC3339),
				},
			}
			capabilities = append(capabilities, capability)
		}
	}

	return capabilities
}

// startHeartbeat starts a goroutine that periodically refreshes the agent's heartbeat in Redis
func (f *Framework) startHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Refresh every 30 seconds (TTL is 60s)
	defer ticker.Stop()

	f.logger.Info("Started heartbeat goroutine", map[string]interface{}{
		"agent_id": f.agentID,
		"interval": "30s",
	})

	for {
		select {
		case <-ctx.Done():
			f.logger.Info("Heartbeat goroutine stopping due to context cancellation", nil)
			return
		case <-ticker.C:
			if f.discovery != nil {
				if err := f.discovery.RefreshHeartbeat(context.Background(), f.registrationID); err != nil {
					f.logger.Warn("Failed to refresh heartbeat", map[string]interface{}{
						"agent_id": f.registrationID,
						"error":    err.Error(),
					})
				} else {
					f.logger.Debug("Heartbeat refreshed successfully", map[string]interface{}{
						"agent_id": f.registrationID,
					})
				}
			}
		}
	}
}

// enhanceWithReflection adds reflection Method information to capabilities
func (f *Framework) enhanceWithReflection(agent agent.Agent, capabilities []CapabilityMetadata) []CapabilityMetadata {
	agentType := reflect.TypeOf(agent)
	methodMap := make(map[string]reflect.Method)

	// Build method lookup map
	for i := 0; i < agentType.NumMethod(); i++ {
		method := agentType.Method(i)
		if method.IsExported() && !isFrameworkMethod(method.Name) {
			methodMap[toSnakeCase(method.Name)] = method
			methodMap[method.Name] = method // Also map original name
		}
	}

	// Enhance capabilities with method information
	for i := range capabilities {
		if method, exists := methodMap[capabilities[i].Name]; exists {
			capabilities[i].Method = method
		}
	}

	return capabilities
}

// isFrameworkMethod checks if a method name is a framework method
func isFrameworkMethod(name string) bool {
	frameworkMethods := []string{
		"Initialize", "Shutdown", "GetCapabilities",
		"HandleRequest", "StartConversation", "SendMessage",
	}

	for _, fm := range frameworkMethods {
		if name == fm {
			return true
		}
	}
	return false
}

// truncateString truncates a string to a maximum length
func (f *Framework) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// StartHTTPServer starts the HTTP server with auto-generated endpoints
func (f *Framework) StartHTTPServer(ctx context.Context, agent agent.Agent) error {
	mux := http.NewServeMux()

	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","agent_id":"%s","name":"%s"}`, f.agentID, f.config.AgentName)
	})

	// Add process endpoint for inter-agent communication
	mux.HandleFunc("/process", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Read the instruction from request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		instruction := string(body)
		
		// Log the incoming request
		f.logger.Info("Received inter-agent request", map[string]interface{}{
			"from_agent":   r.Header.Get("X-From-Agent"),
			"request_id":   r.Header.Get("X-Request-ID"),
			"trace_id":     r.Header.Get("X-Trace-ID"),
			"instruction_preview":  f.truncateString(instruction, 100),
		})

		// Check if agent implements ProcessRequestAgent interface
		if procAgent, ok := agent.(ProcessRequestAgent); ok {
			ctx := r.Context()
			// Add trace ID to context if available
			if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
				ctx = context.WithValue(ctx, "trace_id", traceID)
			}

			// Process the request
			response, err := procAgent.ProcessRequest(ctx, instruction)
			if err != nil {
				f.logger.Error("Failed to process request", map[string]interface{}{
					"error": err.Error(),
					"from_agent": r.Header.Get("X-From-Agent"),
				})
				http.Error(w, fmt.Sprintf("Failed to process request: %v", err), http.StatusInternalServerError)
				return
			}

			// Send the response
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))

			f.logger.Info("Processed inter-agent request successfully", map[string]interface{}{
				"from_agent":     r.Header.Get("X-From-Agent"),
				"response_size":  len(response),
			})
		} else {
			// Agent doesn't implement ProcessRequestAgent
			f.logger.Warn("Agent does not implement ProcessRequestAgent interface", map[string]interface{}{
				"agent_type": fmt.Sprintf("%T", agent),
			})
			http.Error(w, "This agent does not support natural language processing", http.StatusNotImplemented)
		}
	})

	// Add routing endpoint for creating routing plans
	mux.HandleFunc("/route", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse request body
		var routeRequest struct {
			Prompt   string                 `json:"prompt"`
			Metadata map[string]interface{} `json:"metadata,omitempty"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&routeRequest); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if routeRequest.Prompt == "" {
			http.Error(w, "Prompt is required", http.StatusBadRequest)
			return
		}

		// Create routing plan
		plan, err := f.RouteRequest(r.Context(), routeRequest.Prompt, routeRequest.Metadata)
		if err != nil {
			f.logger.Error("Failed to create routing plan", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, fmt.Sprintf("Failed to create routing plan: %v", err), http.StatusInternalServerError)
			return
		}

		// Return the routing plan as JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(plan)

		f.logger.Info("Created routing plan", map[string]interface{}{
			"plan_id":    plan.ID,
			"mode":       plan.Mode,
			"steps":      len(plan.Steps),
			"confidence": plan.Confidence,
		})
	})

	// Add routing stats endpoint
	mux.HandleFunc("/route/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		stats := f.GetRoutingStats()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(stats)
	})

	// Add readiness endpoint honoring strict startup policy
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ready := true
		msg := "ready"
		if f.config.DiscoveryStrictStartup {
			if rd, ok := f.discovery.(*discoveryPkg.RedisDiscovery); ok {
				redisOK := rd.Ping(r.Context()) == nil
				agents, _, _, loaded := rd.SnapshotStats()
				min := f.config.DiscoveryMinSnapshotSize
				cond := redisOK || (loaded && agents >= min)
				if !cond {
					ready = false
					msg = "waiting for discovery (redis or snapshot)"
				}
			}
		}
		if ready {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"ready","message":"%s"}`, msg)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"not_ready","message":"%s"}`, msg)
	})

	// Add status endpoint with discovery/cache metrics
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := map[string]interface{}{
			"agent_id":   f.agentID,
			"agent_name": f.config.AgentName,
			"timestamp":  time.Now().Format(time.RFC3339),
		}
		if rd, ok := f.discovery.(*discoveryPkg.RedisDiscovery); ok {
			registryConnected := rd.Ping(r.Context()) == nil
			agents, caps, age, loaded := rd.SnapshotStats()
			status["registry_connected"] = registryConnected
			status["snapshot_loaded"] = loaded
			status["snapshot_age_seconds"] = int(age.Seconds())
			status["catalog_agents_total"] = agents
			status["catalog_capabilities_total"] = caps
			status["cb_open"] = rd.IsCircuitOpen()
			status["refresh_consecutive_failures"] = rd.ConsecutiveFailures()
		}
		json.NewEncoder(w).Encode(status)
	})

	// Add capabilities endpoint
	// Use capabilities discovered during initialization
	allCapabilities := f.capabilities
	
	// Filter to only include properly documented capabilities
	var documentedCapabilities []CapabilityMetadata
	for _, cap := range allCapabilities {
		if cap.Description != "" && 
		   !strings.HasPrefix(cap.Description, "Auto-discovered capability:") &&
		   strings.TrimSpace(cap.Description) != "" {
			documentedCapabilities = append(documentedCapabilities, cap)
		}
	}
	
	mux.HandleFunc("/capabilities", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := map[string]interface{}{
			"agent_id":     f.agentID,
			"capabilities": documentedCapabilities,
			"count":        len(documentedCapabilities),
			"total_discovered": len(allCapabilities),
			"undocumented_hidden": len(allCapabilities) - len(documentedCapabilities),
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode capabilities", http.StatusInternalServerError)
		}
	})

	// Auto-generate capability invocation endpoints
	// Only for properly documented capabilities
	for _, capability := range documentedCapabilities {
		// Create a copy of capability for closure
		cap := capability
		
		// Generate endpoint path: /invoke/{capability_name}
		endpoint := fmt.Sprintf("/invoke/%s", cap.Name)
		
		f.logger.Info("Registering capability endpoint", map[string]interface{}{
			"endpoint": endpoint,
			"capability": cap.Name,
			"description": cap.Description,
		})
		
		mux.HandleFunc(endpoint, f.createCapabilityHandler(agent, cap))
	}
	
	// Log summary of capability registration
	undocumentedCount := len(allCapabilities) - len(documentedCapabilities)
	if undocumentedCount > 0 {
		f.logger.Warn("Capability registration summary", map[string]interface{}{
			"registered": len(documentedCapabilities),
			"skipped_undocumented": undocumentedCount,
			"total_discovered": len(allCapabilities),
			"message": "Some capabilities were skipped due to missing descriptions. Add @description annotation or YAML metadata.",
		})
	} else if len(documentedCapabilities) > 0 {
		f.logger.Info("All capabilities registered successfully", map[string]interface{}{
			"count": len(documentedCapabilities),
		})
	}
	
	// Also support generic invocation endpoint
	mux.HandleFunc("/invoke", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		// Parse request to get capability name
		var req struct {
			Capability string          `json:"capability"`
			Input      json.RawMessage `json:"input"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		
		// Find the capability
		var targetCap *CapabilityMetadata
		for _, cap := range allCapabilities {
			if cap.Name == req.Capability {
				// Check if this capability has a proper description
				if cap.Description == "" || 
				   strings.HasPrefix(cap.Description, "Auto-discovered capability:") ||
				   strings.TrimSpace(cap.Description) == "" {
					// Capability exists but is not properly documented
					http.Error(w, fmt.Sprintf("Capability '%s' is not available (missing description)", req.Capability), http.StatusForbidden)
					return
				}
				targetCap = &cap
				break
			}
		}
		
		if targetCap == nil {
			http.Error(w, fmt.Sprintf("Capability '%s' not found", req.Capability), http.StatusNotFound)
			return
		}
		
		// Delegate to the capability handler
		f.invokeCapability(w, r, agent, *targetCap, req.Input)
	})

	// Add discovery endpoints
	mux.HandleFunc("/discovery/agents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if f.discovery == nil {
			http.Error(w, "Discovery service not available", http.StatusServiceUnavailable)
			return
		}

		// Fallback implementation: scan Redis keys directly using SCAN and configured namespace
		ns := f.config.DiscoveryNS
		if ns == "" { // default safeguard
			ns = "agents"
		}
		pattern := ns + ":agents:*"

		// Use type assertion to access Redis client directly
		redisDiscovery, ok := f.discovery.(*discoveryPkg.RedisDiscovery)
		if !ok {
			http.Error(w, "GetAllAgents not implemented for this discovery type", http.StatusNotImplemented)
			return
		}

		// Use the new public method to get all agents
		agents, err := redisDiscovery.GetAllAgents(ctx, pattern)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get agents: %v", err), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"requesting_agent": f.agentID,
			"agents":           agents,
			"count":            len(agents),
			"timestamp":        time.Now().Format(time.RFC3339),
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode agents", http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/discovery/capabilities", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if f.discovery == nil {
			http.Error(w, "Discovery service not available", http.StatusServiceUnavailable)
			return
		}

		// Get capability from query parameter
		capability := r.URL.Query().Get("capability")
		if capability == "" {
			http.Error(w, "Missing 'capability' query parameter", http.StatusBadRequest)
			return
		}

		agents, err := f.discovery.FindCapability(ctx, capability)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to find capability: %v", err), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"requesting_agent": f.agentID,
			"capability":       capability,
			"agents":           agents,
			"count":            len(agents),
			"timestamp":        time.Now().Format(time.RFC3339),
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode capability results", http.StatusInternalServerError)
		}
	})

	// Add metrics endpoint (if enabled)
	if f.config.EnableMetrics {
		mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)

			// Generate basic Prometheus-style metrics
			fmt.Fprintf(w, "# HELP agent_capabilities_total Total number of capabilities registered\n")
			fmt.Fprintf(w, "# TYPE agent_capabilities_total counter\n")
			fmt.Fprintf(w, "agent_capabilities_total{agent_id=\"%s\",agent_name=\"%s\"} %d\n", f.agentID, f.config.AgentName, len(f.capabilities))

			fmt.Fprintf(w, "# HELP agent_uptime_seconds Agent uptime in seconds\n")
			fmt.Fprintf(w, "# TYPE agent_uptime_seconds gauge\n")
			fmt.Fprintf(w, "agent_uptime_seconds{agent_id=\"%s\",agent_name=\"%s\"} %.0f\n", f.agentID, f.config.AgentName, time.Since(time.Now()).Seconds()) // Simplified - should track actual start time

			fmt.Fprintf(w, "# HELP agent_info Agent information\n")
			fmt.Fprintf(w, "# TYPE agent_info gauge\n")
			fmt.Fprintf(w, "agent_info{agent_id=\"%s\",agent_name=\"%s\",version=\"1.0.0\"} 1\n", f.agentID, f.config.AgentName)

			// Add capability-specific metrics
			for _, capability := range f.capabilities {
				fmt.Fprintf(w, "# HELP capability_registered Capability registration status\n")
				fmt.Fprintf(w, "# TYPE capability_registered gauge\n")
				fmt.Fprintf(w, "capability_registered{agent_id=\"%s\",capability=\"%s\"} 1\n", f.agentID, capability.Name)
			}
		})
	}

	// Add documentation endpoints (if enabled)
	if f.config.EnableDocs {
		// Generate OpenAPI/Swagger documentation
		docHandler := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			// Generate comprehensive OpenAPI spec with rich metadata
			openAPISpec := fmt.Sprintf(`{
				"openapi": "3.0.0",
				"info": {
					"title": "%s API",
					"version": "1.0.0",
					"description": "Auto-generated API documentation for %s with rich capability metadata",
					"contact": {
						"name": "GoMind Framework",
						"url": "https://github.com/gomind-framework/framework"
					}
				},
				"servers": [
					{
						"url": "http://localhost:%d",
						"description": "Local development server"
					}
				],
				"components": {
					"schemas": {
						"HealthResponse": {
							"type": "object",
							"properties": {
								"status": {"type": "string", "example": "healthy"},
								"agent_id": {"type": "string", "example": "gomind-agent-123456"},
								"name": {"type": "string", "example": "%s"}
							},
							"required": ["status", "agent_id", "name"]
						},
						"CapabilitiesResponse": {
							"type": "object",
							"properties": {
								"capabilities": {"type": "integer", "example": %d},
								"agent_id": {"type": "string", "example": "gomind-agent-123456"},
								"details": {
									"type": "array",
									"items": {
										"type": "object",
										"properties": {
											"name": {"type": "string"},
											"description": {"type": "string"},
											"domain": {"type": "string"},
											"complexity": {"type": "string"},
											"business_value": {"type": "array", "items": {"type": "string"}},
											"input_types": {"type": "array", "items": {"type": "string"}},
											"output_formats": {"type": "array", "items": {"type": "string"}}
										}
									}
								}
							}
						},
						"ErrorResponse": {
							"type": "object",
							"properties": {
								"error": {"type": "string"},
								"message": {"type": "string"},
								"code": {"type": "integer"},
								"timestamp": {"type": "string", "format": "date-time"}
							},
							"required": ["error", "message"]
						}
					},
					"securitySchemes": {
						"ApiKeyAuth": {
							"type": "apiKey",
							"in": "header",
							"name": "X-API-Key",
							"description": "API key for authentication (if enabled)"
						},
						"BearerAuth": {
							"type": "http",
							"scheme": "bearer",
							"bearerFormat": "JWT",
							"description": "JWT token authentication (if enabled)"
						}
					}
				},
				"paths": {
					"/health": {
						"get": {
							"summary": "Health check endpoint",
							"description": "Returns the health status of the agent instance",
							"operationId": "getHealth",
							"tags": ["System"],
							"responses": {
								"200": {
									"description": "Agent is healthy and operational",
									"content": {
										"application/json": {
											"schema": {"$ref": "#/components/schemas/HealthResponse"},
											"example": {
												"status": "healthy",
												"agent_id": "gomind-agent-123456",
												"name": "%s"
											}
										}
									}
								},
								"503": {
									"description": "Agent is unhealthy",
									"content": {
										"application/json": {
											"schema": {"$ref": "#/components/schemas/ErrorResponse"}
										}
									}
								}
							}
						}
					},
					"/capabilities": {
						"get": {
							"summary": "List agent capabilities",
							"description": "Returns detailed information about all available agent capabilities",
							"operationId": "getCapabilities",
							"tags": ["System"],
							"responses": {
								"200": {
									"description": "List of agent capabilities with metadata",
									"content": {
										"application/json": {
											"schema": {"$ref": "#/components/schemas/CapabilitiesResponse"}
										}
									}
								}
							}
						}
					}`, f.config.AgentName, f.config.AgentName, f.config.Port, f.config.AgentName, len(f.capabilities), f.config.AgentName)

			// Add capability endpoints to OpenAPI spec with rich metadata
			for _, capability := range f.capabilities {
				// Generate input schema based on capability metadata
				inputSchema := `{
					"type": "object",
					"description": "Input data for the capability"
				}`

				// Generate more specific schema if we have input type information
				if len(capability.InputTypes) > 0 {
					switch capability.InputTypes[0] {
					case "text", "string":
						inputSchema = `{
							"type": "object",
							"properties": {
								"input": {
									"type": "string",
									"description": "Text input for processing",
									"example": "sample text data"
								}
							},
							"required": ["input"]
						}`
					case "json":
						inputSchema = `{
							"type": "object",
							"properties": {
								"data": {
									"type": "object",
									"description": "JSON data for processing",
									"additionalProperties": true
								}
							},
							"required": ["data"]
						}`
					case "number", "integer":
						inputSchema = `{
							"type": "object",
							"properties": {
								"value": {
									"type": "number",
									"description": "Numeric value for processing",
									"example": 42
								}
							},
							"required": ["value"]
						}`
					}
				}

				// Generate output schema based on capability metadata
				outputSchema := `{
					"type": "object",
					"properties": {
						"result": {
							"type": "object",
							"description": "Processed result from the capability"
						},
						"metadata": {
							"type": "object",
							"properties": {
								"agent_id": {"type": "string"},
								"capability": {"type": "string"},
								"timestamp": {"type": "string", "format": "date-time"},
								"processing_time_ms": {"type": "number"}
							}
						}
					}
				}`

				// Create tags based on domain
				tags := fmt.Sprintf(`["%s"]`, capability.Domain)
				if capability.Domain == "" {
					tags = `["Capabilities"]`
				}

				// Build business context description
				businessContext := capability.Description
				if len(capability.BusinessValue) > 0 {
					businessContext += fmt.Sprintf("\n\n**Business Value:** %v", capability.BusinessValue)
				}
				if len(capability.UseCases) > 0 {
					businessContext += fmt.Sprintf("\n\n**Use Cases:** %v", capability.UseCases)
				}
				if capability.Complexity != "" {
					businessContext += fmt.Sprintf("\n\n**Complexity:** %s", capability.Complexity)
				}
				if capability.Latency != "" {
					businessContext += fmt.Sprintf("\n\n**Expected Latency:** %s", capability.Latency)
				}
				if capability.ConfidenceLevel > 0 {
					businessContext += fmt.Sprintf("\n\n**Confidence Level:** %.2f", capability.ConfidenceLevel)
				}

				openAPISpec += fmt.Sprintf(`,
					"/%s": {
						"post": {
							"summary": "Execute %s capability",
							"description": "%s",
							"operationId": "execute_%s",
							"tags": %s,
							"requestBody": {
								"required": true,
								"content": {
									"application/json": {
										"schema": %s,
										"examples": {
											"basic": {
												"summary": "Basic usage example",
												"description": "Standard input for %s capability",
												"value": %s
											}
										}
									}
								}
							},
							"responses": {
								"200": {
									"description": "Capability executed successfully",
									"content": {
										"application/json": {
											"schema": %s,
											"examples": {
												"success": {
													"summary": "Successful execution",
													"description": "Example successful response from %s",
													"value": {
														"result": "processed data",
														"status": "success",
														"metadata": {
															"agent_id": "gomind-agent-123456",
															"capability": "%s",
															"timestamp": "2024-01-01T12:00:00Z",
															"processing_time_ms": 150
														}
													}
												}
											}
										}
									}
								},
								"400": {
									"description": "Invalid input parameters",
									"content": {
										"application/json": {
											"schema": {"$ref": "#/components/schemas/ErrorResponse"},
											"example": {
												"error": "validation_error",
												"message": "Invalid input format for %s capability",
												"code": 400,
												"timestamp": "2024-01-01T12:00:00Z"
											}
										}
									}
								},
								"500": {
									"description": "Internal server error during capability execution",
									"content": {
										"application/json": {
											"schema": {"$ref": "#/components/schemas/ErrorResponse"}
										}
									}
								}
							}
						}
					}`,
					capability.Name,
					capability.Name,
					businessContext,
					capability.Name,
					tags,
					inputSchema,
					capability.Name,
					f.generateExampleInput(capability),
					outputSchema,
					capability.Name,
					capability.Name,
					capability.Name)
			}

			openAPISpec += `
				}
			}`

			fmt.Fprint(w, openAPISpec)
		}

		// Multiple endpoints for documentation
		mux.HandleFunc("/docs", docHandler)
		mux.HandleFunc("/swagger", docHandler)
		mux.HandleFunc("/api/docs", docHandler)

		// Add a simple HTML docs viewer
		mux.HandleFunc("/docs/ui", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)

			html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>%s API Documentation</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .endpoint { margin: 20px 0; padding: 15px; border: 1px solid #ddd; border-radius: 5px; }
        .method { font-weight: bold; color: #007acc; }
        .path { font-family: monospace; background: #f5f5f5; padding: 2px 5px; }
        pre { background: #f5f5f5; padding: 10px; border-radius: 3px; overflow-x: auto; }
    </style>
</head>
<body>
    <h1>%s API Documentation</h1>
    <p>Agent ID: <code>%s</code></p>
    <p>Base URL: <code>http://localhost:%d</code></p>
    
    <h2>Framework Endpoints</h2>
    
    <div class="endpoint">
        <h3><span class="method">GET</span> <span class="path">/health</span></h3>
        <p>Health check endpoint - returns agent status</p>
    </div>
    
    <div class="endpoint">
        <h3><span class="method">GET</span> <span class="path">/capabilities</span></h3>
        <p>List agent capabilities and metadata</p>
    </div>
    
    <div class="endpoint">
        <h3><span class="method">GET</span> <span class="path">/metrics</span></h3>
        <p>Prometheus-style metrics endpoint</p>
    </div>
    
    <h2>Agent Capabilities (%d total)</h2>`,
				f.config.AgentName, f.config.AgentName, f.agentID, f.config.Port, len(f.capabilities))

			for _, capability := range f.capabilities {
				html += fmt.Sprintf(`
    <div class="endpoint">
        <h3><span class="method">POST</span> <span class="path">/%s</span></h3>
        <p>%s</p>
        <p><strong>Input Types:</strong> %s</p>
        <p><strong>Output Formats:</strong> %s</p>
    </div>`, capability.Name, capability.Description,
					fmt.Sprintf("%v", capability.InputTypes),
					fmt.Sprintf("%v", capability.OutputFormats))
			}

			html += `
</body>
</html>`

			fmt.Fprint(w, html)
		})
	}

	// Auto-generate endpoints for discovered capabilities with actual method execution
	for _, capability := range f.capabilities {
		// Capture capability in closure to avoid loop variable issue
		cap := capability
		capabilityName := cap.Name

		mux.HandleFunc("/"+capabilityName, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			// Parse request body for input
			var input map[string]interface{}
			if r.Body != nil {
				defer r.Body.Close()
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					// If JSON parsing fails, create empty input
					f.logger.Error("Failed to parse JSON request body", map[string]interface{}{"error": err.Error()})
					input = make(map[string]interface{})
				}
			} else {
				input = make(map[string]interface{})
			}

			f.logger.Info("Route handler parsed input", map[string]interface{}{
				"capability": capabilityName,
				"input":      input,
				"inputKeys": func() []string {
					keys := make([]string, 0, len(input))
					for k := range input {
						keys = append(keys, k)
					}
					return keys
				}(),
			})

			// Check if this is an AI-powered request
			var result interface{}
			var err error

			if useAI, ok := input["use_ai"].(bool); ok && useAI && f.aiClient != nil {
				// Use AI-powered execution
				result, err = f.executeAICapability(ctx, agent, cap, input)
			} else {
				// Use reflection-based execution with parsed input
				result, err = f.executeCapabilityWithInput(ctx, agent, cap, input)
			}

			if err != nil {
				f.logger.Error("Capability execution failed", map[string]interface{}{
					"capability": capabilityName,
					"error":      err.Error(),
				})
				http.Error(w, fmt.Sprintf("Capability execution failed: %v", err), http.StatusInternalServerError)
				return
			}

			// Return the actual result from the method
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(result); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		})
	}

	// Add conversation endpoint for conversational agents
	if convAgent, ok := agent.(ConversationalAgent); ok {
		// Create adapter to bridge the types
		adapter := conversation.NewConversationalAgentAdapter(convAgent)
		f.conversationManager.SetAgent(adapter)

		mux.HandleFunc("/conversation", f.conversationManager.ServeConversationHTTP)

		// Add chat UI endpoint
		mux.HandleFunc("/chat", chat.ServeChatUI(f.config.AgentName))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				// Redirect root to chat UI for conversational agents
				http.Redirect(w, r, "/chat", http.StatusFound)
				return
			}
			http.NotFound(w, r)
		})

		// Add session management endpoints
		mux.HandleFunc("/conversation/session", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost:
				// Create new session
				session := f.conversationManager.CreateSession()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"sessionId": session.ID,
					"created":   session.StartTime,
				})
			case http.MethodGet:
				// Get session info
				sessionID := r.URL.Query().Get("sessionId")
				if sessionID == "" {
					http.Error(w, "sessionId parameter required", http.StatusBadRequest)
					return
				}

				session, exists := f.conversationManager.GetSession(sessionID)
				if !exists {
					http.Error(w, "Session not found", http.StatusNotFound)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"sessionId":    session.ID,
					"created":      session.StartTime,
					"lastMessage":  session.LastMessage,
					"messageCount": len(session.GetMessages()),
				})
			case http.MethodDelete:
				// Remove session
				sessionID := r.URL.Query().Get("sessionId")
				if sessionID == "" {
					http.Error(w, "sessionId parameter required", http.StatusBadRequest)
					return
				}

				f.conversationManager.RemoveSession(sessionID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{
					"status": "deleted",
				})
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})

		// Add session history endpoint
		mux.HandleFunc("/conversation/history", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			sessionID := r.URL.Query().Get("sessionId")
			if sessionID == "" {
				http.Error(w, "sessionId parameter required", http.StatusBadRequest)
				return
			}

			session, exists := f.conversationManager.GetSession(sessionID)
			if !exists {
				http.Error(w, "Session not found", http.StatusNotFound)
				return
			}

			// Get recent messages (limit parameter)
			limitStr := r.URL.Query().Get("limit")
			limit := 50 // default
			if limitStr != "" {
				if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
					limit = l
				}
			}

			messages := session.GetRecentMessages(limit)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"sessionId": session.ID,
				"messages":  messages,
				"count":     len(messages),
			})
		})
	}

	// Add AI chat endpoint if AI client is available
	if f.aiClient != nil {
		mux.HandleFunc("/ai/chat", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			var input struct {
				Message      string            `json:"message"`
				SystemPrompt string            `json:"system_prompt,omitempty"`
				Temperature  float64           `json:"temperature,omitempty"`
				MaxTokens    int               `json:"max_tokens,omitempty"`
				Model        string            `json:"model,omitempty"`
				Metadata     map[string]string `json:"metadata,omitempty"`
			}

			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				http.Error(w, "Invalid JSON input", http.StatusBadRequest)
				return
			}

			if input.Message == "" {
				http.Error(w, "Message is required", http.StatusBadRequest)
				return
			}

			// Set defaults
			if input.Model == "" {
				input.Model = f.config.DefaultAIModel
			}
			if input.Temperature == 0 {
				input.Temperature = 0.7
			}
			if input.MaxTokens == 0 {
				input.MaxTokens = 1000
			}

			options := &GenerationOptions{
				Model:        input.Model,
				SystemPrompt: input.SystemPrompt,
				Temperature:  input.Temperature,
				MaxTokens:    input.MaxTokens,
				Metadata:     input.Metadata,
			}

			response, err := f.aiClient.GenerateResponse(r.Context(), input.Message, options)
			if err != nil {
				f.logger.Error("AI chat request failed", map[string]interface{}{
					"error": err.Error(),
				})
				http.Error(w, fmt.Sprintf("AI request failed: %v", err), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response":   response.Content,
				"model":      response.Model,
				"usage":      response.Usage,
				"confidence": response.Confidence,
				"metadata": map[string]interface{}{
					"agent_id":      f.agentID,
					"finish_reason": response.FinishReason,
					"timestamp":     time.Now().Format(time.RFC3339),
				},
			})
		})

		// Add AI streaming chat endpoint
		mux.HandleFunc("/ai/stream", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			// Set headers for Server-Sent Events
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Access-Control-Allow-Origin", "*")

			var input struct {
				Message      string            `json:"message"`
				SystemPrompt string            `json:"system_prompt,omitempty"`
				Temperature  float64           `json:"temperature,omitempty"`
				MaxTokens    int               `json:"max_tokens,omitempty"`
				Model        string            `json:"model,omitempty"`
				Metadata     map[string]string `json:"metadata,omitempty"`
			}

			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				http.Error(w, "Invalid JSON input", http.StatusBadRequest)
				return
			}

			if input.Message == "" {
				http.Error(w, "Message is required", http.StatusBadRequest)
				return
			}

			// Set defaults
			if input.Model == "" {
				input.Model = f.config.DefaultAIModel
			}
			if input.Temperature == 0 {
				input.Temperature = 0.7
			}
			if input.MaxTokens == 0 {
				input.MaxTokens = 1000
			}

			options := &GenerationOptions{
				Model:        input.Model,
				SystemPrompt: input.SystemPrompt,
				Temperature:  input.Temperature,
				MaxTokens:    input.MaxTokens,
				Metadata:     input.Metadata,
			}

			stream, err := f.aiClient.StreamResponse(r.Context(), input.Message, options)
			if err != nil {
				f.logger.Error("AI streaming request failed", map[string]interface{}{
					"error": err.Error(),
				})
				http.Error(w, fmt.Sprintf("AI streaming failed: %v", err), http.StatusInternalServerError)
				return
			}

			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "Streaming not supported", http.StatusInternalServerError)
				return
			}

			for chunk := range stream {
				if chunk.Error != nil {
					fmt.Fprintf(w, "data: %s\n\n", fmt.Sprintf(`{"error": "%s"}`, chunk.Error.Error()))
					flusher.Flush()
					break
				}

				chunkData := map[string]interface{}{
					"content":     chunk.Content,
					"is_complete": chunk.IsComplete,
					"chunk_type":  chunk.ChunkType,
					"metadata":    chunk.Metadata,
				}

				jsonData, _ := json.Marshal(chunkData)
				fmt.Fprintf(w, "data: %s\n\n", jsonData)
				flusher.Flush()

				if chunk.IsComplete {
					break
				}
			}
		})
	}

	// Register custom handlers
	for pattern, handler := range f.customHandlers {
		mux.HandleFunc(pattern, handler)
		f.logger.Info("Registered custom handler", map[string]interface{}{
			"pattern": pattern,
		})
	}

	// Wrap with telemetry if available
	var handler http.Handler = mux
	if f.telemetry != nil {
		handler = otelhttp.NewHandler(mux, "agent-server")
	}

	// Get port strategy and determine actual port to use
	strategy := f.portManager.GetPortStrategy()
	port := f.portManager.DeterminePort()

	// Create HTTP server
	f.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}

	f.logger.Info("Starting HTTP server", map[string]interface{}{
		"port":          port,
		"source":        strategy.Source,
		"environment":   strategy.Environment,
		"auto_discover": strategy.AutoDiscover,
		"capabilities":  len(f.capabilities),
	})

	// Start background conversation session cleanup for conversational agents
	if _, ok := agent.(ConversationalAgent); ok && f.conversationManager != nil {
		go func() {
			ticker := time.NewTicker(30 * time.Minute) // Cleanup every 30 minutes
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// Remove sessions older than 2 hours
					f.conversationManager.CleanupExpiredSessions(2 * time.Hour)
					f.logger.Info("Cleaned up expired conversation sessions", nil)
				}
			}
		}()
	}

	// Create channel to communicate server startup status
	serverStarted := make(chan error, 1)

	// Start server in background
	go func() {
		defer close(serverStarted)
		if err := f.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			f.logger.Error("HTTP server error", map[string]interface{}{
				"error": err.Error(),
			})
			serverStarted <- err
			return
		}
		serverStarted <- nil
	}()

	// Give server a moment to start up and check for immediate errors
	select {
	case err := <-serverStarted:
		if err != nil {
			return fmt.Errorf("HTTP server failed to start: %w", err)
		}
		// Server started successfully but immediately exited (shouldn't happen)
		return fmt.Errorf("HTTP server exited unexpectedly")
	case <-time.After(100 * time.Millisecond):
		// Server seems to have started successfully
		f.logger.Info("HTTP server started successfully", map[string]interface{}{
			"port":    f.config.Port,
			"address": fmt.Sprintf("http://localhost:%d", f.config.Port),
		})
	}

	// Wait for either context cancellation or server error
	select {
	case <-ctx.Done():
		f.logger.Info("Received shutdown signal", nil)
		return nil
	case err := <-serverStarted:
		if err != nil {
			return fmt.Errorf("HTTP server error: %w", err)
		}
		return fmt.Errorf("HTTP server stopped unexpectedly")
	}
}

// createCapabilityHandler creates an HTTP handler for a specific capability
func (f *Framework) createCapabilityHandler(agent agent.Agent, capability CapabilityMetadata) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only allow POST for capability invocation
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		// Parse the request body
		var input json.RawMessage
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				// If body is not valid JSON, treat as empty input
				input = json.RawMessage("{}")
			}
		} else {
			input = json.RawMessage("{}")
		}
		
		// Invoke the capability
		f.invokeCapability(w, r, agent, capability, input)
	}
}

// invokeCapability handles the actual invocation of a capability
func (f *Framework) invokeCapability(w http.ResponseWriter, r *http.Request, agent agent.Agent, capability CapabilityMetadata, input json.RawMessage) {
	ctx := r.Context()
	
	// Start telemetry span if available
	if f.telemetry != nil {
		telemetryCap := f.convertToTelemetryCapability(capability)
		var span trace.Span
		ctx, span = f.telemetry.CreateSpanWithCapability(ctx, telemetryCap)
		if span != nil {
			defer span.End()
		}
	}
	
	// Log the invocation
	f.logger.Info("Invoking capability", map[string]interface{}{
		"capability": capability.Name,
		"agent_id":   f.agentID,
	})
	
	// Create a new request with the input as body for executeCapability
	bodyReader := bytes.NewReader(input)
	newReq, err := http.NewRequestWithContext(ctx, r.Method, r.URL.String(), bodyReader)
	if err != nil {
		f.logger.Error("Failed to create request for capability execution", map[string]interface{}{
			"capability": capability.Name,
			"error":      err.Error(),
		})
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	// Copy headers from original request
	newReq.Header = r.Header
	
	// Execute the capability
	result, err := f.executeCapability(ctx, agent, capability, newReq)
	if err != nil {
		f.logger.Error("Capability execution failed", map[string]interface{}{
			"capability": capability.Name,
			"error":      err.Error(),
		})
		
		// Return error response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":      "Capability execution failed",
			"capability": capability.Name,
			"message":    err.Error(),
		})
		return
	}
	
	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	response := map[string]interface{}{
		"capability": capability.Name,
		"agent_id":   f.agentID,
		"result":     result,
		"timestamp":  time.Now().Format(time.RFC3339),
	}
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		f.logger.Error("Failed to encode response", map[string]interface{}{
			"capability": capability.Name,
			"error":      err.Error(),
		})
	}
}

// Shutdown gracefully shuts down the framework
func (f *Framework) Shutdown(ctx context.Context) error {
	f.logger.Info("Shutting down framework", nil)

	// Cleanup conversation sessions
	if f.conversationManager != nil {
		f.conversationManager.CleanupExpiredSessions(0) // Clean all sessions on shutdown
	}

	// Deregister from discovery
	if f.discovery != nil && f.registrationID != "" {
		if err := f.discovery.Unregister(ctx, f.registrationID); err != nil {
			f.logger.Error("Failed to deregister agent", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Shutdown HTTP server
	if f.httpServer != nil {
		f.httpServer.Shutdown(ctx)
	}

	// Shutdown telemetry
	if f.telemetry != nil {
		f.telemetry.Shutdown(ctx)
	}

	return nil
}

// Router returns an HTTP router with framework routes for direct control pattern
func (f *Framework) Router() http.Handler {
	mux := http.NewServeMux()

	// Add default framework routes
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s"}`, time.Now().UTC().Format(time.RFC3339))
	})

	return mux
}

// HandleFunc registers a custom HTTP handler for direct control pattern
func (f *Framework) HandleFunc(pattern string, handler http.HandlerFunc) {
	// Store custom handlers to be registered in StartHTTPServer
	f.customHandlers[pattern] = handler
	f.logger.Info("Registering custom handler", map[string]interface{}{
		"pattern": pattern,
	})
}

// Utility functions
// executeAICapability executes a capability using AI if available
func (f *Framework) executeAICapability(ctx context.Context, agent agent.Agent, capability CapabilityMetadata, input map[string]interface{}) (interface{}, error) {
	if f.aiClient == nil {
		return nil, fmt.Errorf("AI client not configured - set OPENAI_API_KEY environment variable")
	}

	// Extract prompt from input
	prompt, ok := input["prompt"].(string)
	if !ok {
		prompt = fmt.Sprintf("Process this data for %s capability: %v", capability.Name, input)
	}

	// Use capability metadata to enhance the prompt
	systemPrompt := fmt.Sprintf(`You are an AI agent with the capability: %s
Description: %s
Domain: %s
Business Context: Your responses should be focused on %v
Confidence Level: Maintain at least %.2f confidence in your responses
Response Format: %s`,
		capability.Name,
		capability.Description,
		capability.Domain,
		capability.BusinessValue,
		capability.ConfidenceLevel,
		strings.Join(capability.OutputFormats, ", "))

	options := &GenerationOptions{
		Model:        f.config.DefaultAIModel,
		SystemPrompt: systemPrompt,
		Temperature:  0.7,
		MaxTokens:    1000,
		Metadata: map[string]string{
			"capability": capability.Name,
			"agent_id":   f.agentID,
			"domain":     capability.Domain,
		},
	}

	response, err := f.aiClient.GenerateResponse(ctx, prompt, options)
	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	return map[string]interface{}{
		"result":     response.Content,
		"model":      response.Model,
		"confidence": response.Confidence,
		"usage":      response.Usage,
		"metadata": map[string]interface{}{
			"agent_id":      f.agentID,
			"capability":    capability.Name,
			"finish_reason": response.FinishReason,
			"timestamp":     time.Now().Format(time.RFC3339),
		},
	}, nil
}

// executeCapability executes an agent's capability method using reflection
func (f *Framework) executeCapability(ctx context.Context, agent agent.Agent, capability CapabilityMetadata, r *http.Request) (interface{}, error) {
	// Check if the capability has a valid Method
	if capability.Method.Name == "" {
		return nil, fmt.Errorf("capability %s has no valid method", capability.Name)
	}

	// Parse request body for input
	var input map[string]interface{}
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			// If JSON parsing fails, create empty input
			input = make(map[string]interface{})
		}
	} else {
		input = make(map[string]interface{})
	}

	// Prepare method arguments
	method := capability.Method
	methodType := method.Type

	// Validate method signature - should have at least receiver and context
	if methodType.NumIn() < 2 {
		return nil, fmt.Errorf("method %s must have at least receiver and context parameters", method.Name)
	}

	// Build argument list
	args := make([]reflect.Value, methodType.NumIn())

	// Arg 0: receiver (the agent)
	args[0] = reflect.ValueOf(agent)

	// Arg 1: context
	if methodType.In(1) == reflect.TypeOf((*context.Context)(nil)).Elem() {
		args[1] = reflect.ValueOf(ctx)
	} else {
		return nil, fmt.Errorf("method %s second parameter must be context.Context", method.Name)
	}

	// Arg 2+: method-specific parameters
	for i := 2; i < methodType.NumIn(); i++ {
		paramType := methodType.In(i)

		// Handle different parameter types intelligently
		switch paramType.Kind() {
		case reflect.String:
			// Look for string parameters in input
			if str, ok := input["data"].(string); ok {
				args[i] = reflect.ValueOf(str)
			} else if str, ok := input["query"].(string); ok {
				args[i] = reflect.ValueOf(str)
			} else if str, ok := input["message"].(string); ok {
				args[i] = reflect.ValueOf(str)
			} else {
				args[i] = reflect.ValueOf("") // Default empty string
			}
		case reflect.Interface:
			// For interface{} or map[string]interface{} parameters
			if paramType == reflect.TypeOf((*interface{})(nil)).Elem() {
				args[i] = reflect.ValueOf(input)
			} else if paramType == reflect.TypeOf((*map[string]interface{})(nil)).Elem() {
				args[i] = reflect.ValueOf(input)
			} else {
				args[i] = reflect.ValueOf(input)
			}
		case reflect.Map:
			// For map types, pass the entire input
			args[i] = reflect.ValueOf(input)
		case reflect.Struct:
			// For struct types, convert JSON input to struct
			structValue := reflect.New(paramType).Elem()
			if err := f.convertMapToStruct(input, structValue); err != nil {
				return nil, fmt.Errorf("failed to convert input to struct %s: %w", paramType.Name(), err)
			}
			args[i] = structValue
		case reflect.Ptr:
			// For pointer types, check if it's a pointer to struct
			if paramType.Elem().Kind() == reflect.Struct {
				structValue := reflect.New(paramType.Elem())
				if err := f.convertMapToStruct(input, structValue.Elem()); err != nil {
					return nil, fmt.Errorf("failed to convert input to struct %s: %w", paramType.Elem().Name(), err)
				}
				args[i] = structValue
			} else {
				// For other pointer types, try to pass the input as-is
				args[i] = reflect.ValueOf(input)
			}
		default:
			// For other types, try to pass the input as-is
			args[i] = reflect.ValueOf(input)
		}
	}

	// Execute the method
	f.logger.Info("Executing capability method", map[string]interface{}{
		"capability": capability.Name,
		"method":     method.Name,
		"agent_id":   f.agentID,
	})

	results := method.Func.Call(args)

	// Process results
	if len(results) == 0 {
		return map[string]interface{}{
			"message": fmt.Sprintf("Capability %s executed successfully", capability.Name),
			"status":  "success",
		}, nil
	}

	// Handle methods that return (result, error)
	if len(results) == 2 {
		// Check if last result is an error
		if results[1].Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			if !results[1].IsNil() {
				err := results[1].Interface().(error)
				return nil, fmt.Errorf("method execution failed: %w", err)
			}
		}

		// Return the first result
		return results[0].Interface(), nil
	}

	// Handle methods that return just a result
	if len(results) == 1 {
		// Check if it's an error
		if results[0].Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			if !results[0].IsNil() {
				err := results[0].Interface().(error)
				return nil, fmt.Errorf("method execution failed: %w", err)
			}
			return map[string]interface{}{
				"message": fmt.Sprintf("Capability %s executed successfully", capability.Name),
				"status":  "success",
			}, nil
		}

		return results[0].Interface(), nil
	}

	// Fallback for multiple results - return them as an array
	resultValues := make([]interface{}, len(results))
	for i, result := range results {
		resultValues[i] = result.Interface()
	}

	return resultValues, nil
}

// executeCapabilityWithInput executes an agent's capability method using reflection with pre-parsed input
func (f *Framework) executeCapabilityWithInput(ctx context.Context, agent agent.Agent, capability CapabilityMetadata, input map[string]interface{}) (interface{}, error) {
	// Check if the capability has a valid Method
	if capability.Method.Name == "" {
		return nil, fmt.Errorf("capability %s has no valid method", capability.Name)
	}

	// Prepare method arguments
	method := capability.Method
	methodType := method.Type

	// Validate method signature - should have at least receiver and context
	if methodType.NumIn() < 2 {
		return nil, fmt.Errorf("method %s must have at least receiver and context parameters", method.Name)
	}

	// Build argument list
	args := make([]reflect.Value, methodType.NumIn())

	// Arg 0: receiver (the agent)
	args[0] = reflect.ValueOf(agent)

	// Arg 1: context
	if methodType.In(1) == reflect.TypeOf((*context.Context)(nil)).Elem() {
		args[1] = reflect.ValueOf(ctx)
	} else {
		return nil, fmt.Errorf("method %s second parameter must be context.Context", method.Name)
	}

	// Arg 2+: method-specific parameters
	for i := 2; i < methodType.NumIn(); i++ {
		paramType := methodType.In(i)

		// Handle different parameter types intelligently
		switch paramType.Kind() {
		case reflect.String:
			// Look for string parameters in input
			if str, ok := input["data"].(string); ok {
				args[i] = reflect.ValueOf(str)
			} else if str, ok := input["query"].(string); ok {
				args[i] = reflect.ValueOf(str)
			} else if str, ok := input["message"].(string); ok {
				args[i] = reflect.ValueOf(str)
			} else {
				args[i] = reflect.ValueOf("") // Default empty string
			}
		case reflect.Interface:
			// For interface{} or map[string]interface{} parameters
			if paramType == reflect.TypeOf((*interface{})(nil)).Elem() {
				args[i] = reflect.ValueOf(input)
			} else if paramType == reflect.TypeOf((*map[string]interface{})(nil)).Elem() {
				args[i] = reflect.ValueOf(input)
			} else {
				args[i] = reflect.ValueOf(input)
			}
		case reflect.Map:
			// For map types, pass the entire input
			args[i] = reflect.ValueOf(input)
		case reflect.Struct:
			// For struct types, convert JSON input to struct
			structValue := reflect.New(paramType).Elem()
			if err := f.convertMapToStruct(input, structValue); err != nil {
				return nil, fmt.Errorf("failed to convert input to struct %s: %w", paramType.Name(), err)
			}
			args[i] = structValue
		case reflect.Ptr:
			// For pointer types, check if it's a pointer to struct
			if paramType.Elem().Kind() == reflect.Struct {
				structValue := reflect.New(paramType.Elem())
				if err := f.convertMapToStruct(input, structValue.Elem()); err != nil {
					return nil, fmt.Errorf("failed to convert input to struct %s: %w", paramType.Elem().Name(), err)
				}
				args[i] = structValue
			} else {
				args[i] = reflect.Zero(paramType)
			}
		default:
			// For other types, use zero value
			args[i] = reflect.Zero(paramType)
		}
	}

	f.logger.Info("Executing capability method", map[string]interface{}{
		"method": method.Name,
		"args":   len(args),
	})

	// Call the method
	results := method.Func.Call(args)

	// Handle return values (same as original executeCapability method)
	if len(results) == 0 {
		return nil, nil
	}

	// Check for error return value (typically the last return value)
	if len(results) >= 2 {
		if errInterface := results[len(results)-1].Interface(); errInterface != nil {
			if err, ok := errInterface.(error); ok && err != nil {
				return nil, fmt.Errorf("method execution failed: %w", err)
			}
		}

		return results[0].Interface(), nil
	}

	// Fallback for multiple results - return them as an array
	resultValues := make([]interface{}, len(results))
	for i, result := range results {
		resultValues[i] = result.Interface()
	}

	return resultValues, nil
}

// convertMapToStruct converts a map[string]interface{} to a struct using JSON marshaling/unmarshaling
func (f *Framework) convertMapToStruct(input map[string]interface{}, structValue reflect.Value) error {
	// Convert input map to JSON
	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal input to JSON: %w", err)
	}

	// Create a pointer to the struct value for unmarshaling
	structPtr := structValue.Addr().Interface()

	// Unmarshal JSON into the struct
	if err := json.Unmarshal(jsonBytes, structPtr); err != nil {
		return fmt.Errorf("failed to unmarshal JSON to struct: %w", err)
	}

	return nil
}

// GetRouter returns the configured router instance
func (f *Framework) GetRouter() routingPkg.Router {
	return f.router
}

// RouteRequest uses the configured router to create a routing plan
func (f *Framework) RouteRequest(ctx context.Context, prompt string, metadata map[string]interface{}) (*routingPkg.RoutingPlan, error) {
	if f.router == nil {
		return nil, fmt.Errorf("router not initialized - ensure AI is enabled and configured")
	}
	
	// Update router with latest agent catalog if using autonomous or hybrid mode
	if f.discovery != nil {
		catalog := f.discovery.GetCatalogForLLM()
		f.router.SetAgentCatalog(catalog)
	}
	
	return f.router.Route(ctx, prompt, metadata)
}

// UpdateRouterCatalog manually updates the router's agent catalog
func (f *Framework) UpdateRouterCatalog() {
	if f.router != nil && f.discovery != nil {
		catalog := f.discovery.GetCatalogForLLM()
		f.router.SetAgentCatalog(catalog)
		f.logger.Debug("Updated router agent catalog", nil)
	}
}

// GetRoutingStats returns routing statistics
func (f *Framework) GetRoutingStats() routingPkg.RouterStats {
	if f.router != nil {
		return f.router.GetStats()
	}
	return routingPkg.RouterStats{}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

func getEnvFloatOrDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func toSnakeCase(str string) string {
	// Simple snake_case conversion
	var result strings.Builder
	for i, r := range str {
		if i > 0 && 'A' <= r && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// generateExampleInput creates example input data based on capability metadata
func (f *Framework) generateExampleInput(capability CapabilityMetadata) string {
	if len(capability.InputTypes) == 0 {
		return `{"data": "example input"}`
	}

	switch capability.InputTypes[0] {
	case "text", "string":
		return `{"input": "sample text for processing"}`
	case "json":
		return `{"data": {"key": "value", "items": ["item1", "item2"]}}`
	case "number", "integer":
		return `{"value": 42}`
	case "array":
		return `{"items": ["item1", "item2", "item3"]}`
	case "boolean":
		return `{"flag": true}`
	default:
		return fmt.Sprintf(`{"input": "example data for %s capability"}`, capability.Name)
	}
}

// convertToDiscoveryCapabilities converts framework CapabilityMetadata to discovery CapabilityMetadata
func (f *Framework) convertToDiscoveryCapabilities(capabilities []CapabilityMetadata) []discoveryPkg.CapabilityMetadata {
	var discoveryCapabilities []discoveryPkg.CapabilityMetadata
	for _, cap := range capabilities {
		discoveryCap := discoveryPkg.CapabilityMetadata{
			Name:        cap.Name,
			Description: cap.Description,
			Domain:      cap.Domain,
			// Map other fields as needed
		}
		discoveryCapabilities = append(discoveryCapabilities, discoveryCap)
	}
	return discoveryCapabilities
}

// convertToTelemetryCapability converts framework CapabilityMetadata to telemetry CapabilityMetadata
func (f *Framework) convertToTelemetryCapability(capability CapabilityMetadata) telemetryPkg.CapabilityMetadata {
	return telemetryPkg.CapabilityMetadata{
		Name:            capability.Name,
		Domain:          capability.Domain,
		Complexity:      capability.Complexity,
		ConfidenceLevel: capability.ConfidenceLevel,
		BusinessValue:   capability.BusinessValue,
		LLMPrompt:       capability.LLMPrompt,
		Specialties:     capability.Specialties,
	}
}
