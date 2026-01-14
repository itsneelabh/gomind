package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/orchestration"
	"github.com/itsneelabh/gomind/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// TravelChatAgent is a streaming chat agent that uses orchestration
// to coordinate travel-related tools and provide real-time responses via SSE.
type TravelChatAgent struct {
	*core.BaseAgent
	orchestrator *orchestration.AIOrchestrator
	sessionStore *SessionStore
	httpClient   *http.Client // Traced HTTP client
	mu           sync.RWMutex
}

// NewTravelChatAgent creates a new travel chat agent with AI and telemetry configured.
func NewTravelChatAgent() (*TravelChatAgent, error) {
	agent := core.NewBaseAgent("travel-chat-agent")

	// Create AI client with provider chain for failover
	// Provider chain: OpenAI (primary) → Anthropic (backup)
	chainClient, err := ai.NewChainClient(
		ai.WithProviderChain("openai", "anthropic"),
		ai.WithChainTelemetry(telemetry.GetTelemetryProvider()),
		ai.WithChainLogger(agent.Logger),
	)
	if err != nil {
		agent.Logger.Warn("Failed to create AI chain client, trying single provider", map[string]interface{}{
			"error": err.Error(),
		})
		// Fallback to single provider - returns core.AIClient interface
		singleClient, err := ai.NewClient()
		if err != nil {
			// AI is optional - some orchestration features still work without it
			agent.Logger.Warn("AI client creation failed, some features will be limited", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			agent.AI = singleClient
		}
	} else {
		agent.AI = chainClient
	}

	// Declare metrics this agent will emit for observability
	telemetry.DeclareMetrics("travel-chat-agent", telemetry.ModuleConfig{
		Metrics: []telemetry.MetricDefinition{
			{
				Name:    "chat.request.duration_ms",
				Type:    "histogram",
				Help:    "Chat request duration in milliseconds",
				Labels:  []string{"session_id", "status"},
				Unit:    "milliseconds",
				Buckets: []float64{100, 500, 1000, 2000, 5000, 10000, 30000},
			},
			{
				Name:   "chat.requests",
				Type:   "counter",
				Help:   "Number of chat requests",
				Labels: []string{"status"},
			},
			{
				Name: "chat.sessions.active",
				Type: "gauge",
				Help: "Number of active chat sessions",
			},
			{
				Name:   "chat.orchestration.tool_calls",
				Type:   "counter",
				Help:   "Number of tool calls made during chat orchestration",
				Labels: []string{"tool_name"},
			},
		},
	})

	// Create traced HTTP client with production settings
	// Increased timeout for complex multi-tool orchestration
	tracedClient := telemetry.NewTracedHTTPClientWithTransport(&http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
		ForceAttemptHTTP2:   true,
	})
	tracedClient.Timeout = 300 * time.Second // Increased for complex orchestration

	// Create Redis-backed session store
	// Uses Redis DB 2 (RedisDBSessions) to isolate from service registry (DB 0)
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL is required for session storage")
	}
	sessionStore, err := NewSessionStore(redisURL, 30*time.Minute, 50, agent.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create session store: %w", err)
	}

	chatAgent := &TravelChatAgent{
		BaseAgent:    agent,
		sessionStore: sessionStore,
		httpClient:   tracedClient,
	}

	// Register capabilities
	chatAgent.registerCapabilities()

	return chatAgent, nil
}

// InitializeOrchestrator sets up the orchestrator after Discovery is available.
func (t *TravelChatAgent) InitializeOrchestrator(discovery core.Discovery) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if discovery == nil {
		return fmt.Errorf("discovery service not available")
	}

	// Create orchestrator config
	config := orchestration.DefaultConfig()
	config.RoutingMode = orchestration.ModeAutonomous
	config.SynthesisStrategy = orchestration.StrategyLLM
	config.MetricsEnabled = true
	config.EnableTelemetry = true

	// Increase timeouts for complex multi-tool orchestration scenarios
	config.ExecutionOptions.TotalTimeout = 5 * time.Minute  // Overall orchestration timeout
	config.ExecutionOptions.StepTimeout = 120 * time.Second // Per-step timeout for AI planning

	// Configure prompt builder for travel domain
	config.PromptConfig = orchestration.PromptConfig{
		// SystemInstructions defines the chat agent's persona and behavioral context.
		// This becomes the primary identity, with the orchestrator role as secondary.
		// Similar to LangChain's system_prompt, AutoGen's system_message, or OpenAI's instructions.
		SystemInstructions: `You are a friendly travel chat assistant.
You help users plan trips by coordinating information from various travel services.
Be conversational and helpful while providing accurate, real-time information.`,

		Domain: "travel",
		AdditionalTypeRules: []orchestration.TypeRule{
			{
				TypeNames:   []string{"latitude", "lat", "longitude", "lon"},
				JsonType:    "JSON numbers",
				Example:     `35.6762`,
				AntiPattern: `"35.6762"`,
				Description: "Geographic coordinates for weather and location lookups",
			},
			{
				TypeNames:   []string{"currency_code", "from_currency", "to_currency", "from", "to"},
				JsonType:    "JSON strings",
				Example:     `"USD"`,
				Description: "ISO 4217 currency codes (USD, EUR, JPY, etc.)",
			},
			{
				TypeNames:   []string{"amount", "price", "cost"},
				JsonType:    "JSON numbers",
				Example:     `1000.50`,
				AntiPattern: `"1000.50"`,
				Description: "Monetary amounts for currency conversion",
			},
			{
				TypeNames:   []string{"location", "destination", "city"},
				JsonType:    "JSON strings",
				Example:     `"Tokyo, Japan"`,
				Description: "Location or destination names",
			},
		},
		CustomInstructions: []string{
			"For weather queries, always geocode the location first to get coordinates",
			"For currency conversion, extract the destination country's currency code",
			"Prefer parallel execution when steps are independent",
		},
	}

	// Create dependencies using factory pattern with dependency injection
	deps := orchestration.OrchestratorDependencies{
		Discovery:           discovery,
		AIClient:            t.AI,
		Logger:              t.Logger,
		Telemetry:           telemetry.GetTelemetryProvider(),
		EnableErrorAnalyzer: true, // Enable LLM-based error analysis
	}

	// Create orchestrator
	orch, err := orchestration.CreateOrchestrator(config, deps)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Start the orchestrator
	ctx := context.Background()
	if err := orch.Start(ctx); err != nil {
		return fmt.Errorf("failed to start orchestrator: %w", err)
	}

	t.orchestrator = orch

	t.Logger.Info("Orchestrator initialized successfully", map[string]interface{}{
		"routing_mode":       config.RoutingMode,
		"synthesis_strategy": config.SynthesisStrategy,
	})

	return nil
}

// formatConversationContext formats conversation history into a prompt context.
// This gives the LLM awareness of prior conversation turns for continuity.
func (t *TravelChatAgent) formatConversationContext(history []Message, currentQuery string) string {
	if len(history) == 0 {
		return currentQuery
	}

	var sb strings.Builder

	// Add conversation history as context
	sb.WriteString("Previous conversation:\n")
	for _, msg := range history {
		role := "User"
		if msg.Role == "assistant" {
			role = "Assistant"
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n", role, msg.Content))
	}

	// Add the current query
	sb.WriteString("\nCurrent request:\n")
	sb.WriteString(currentQuery)

	return sb.String()
}

// ProcessWithStreaming processes a user query and streams progress via callback.
// It uses true streaming when the orchestrator supports it, falling back to
// simulated streaming (chunking the complete response) otherwise.
func (t *TravelChatAgent) ProcessWithStreaming(ctx context.Context, sessionID, query string, callback StreamCallback) error {
	startTime := time.Now()

	t.mu.RLock()
	orch := t.orchestrator
	t.mu.RUnlock()

	if orch == nil {
		return fmt.Errorf("orchestrator not initialized")
	}

	// Retrieve conversation history for context
	// This allows the LLM to understand references to previous messages
	history := t.sessionStore.GetHistory(sessionID)

	// Format the query with conversation context
	queryWithContext := t.formatConversationContext(history, query)

	// Log start with trace context
	t.Logger.InfoWithContext(ctx, "Processing chat request", map[string]interface{}{
		"operation":      "process_chat",
		"session_id":     sessionID,
		"query_len":      len(query),
		"history_turns":  len(history),
		"context_length": len(queryWithContext),
	})

	// Send planning status
	callback.SendStatus("planning", "Analyzing your request...")

	// Add span event for planning start
	telemetry.AddSpanEvent(ctx, "orchestration.started",
		attribute.String("session_id", sessionID),
		attribute.Int("query_length", len(query)),
		attribute.Int("history_turns", len(history)),
	)

	// Use true streaming - tokens are delivered as they're generated by the AI provider
	// The orchestrator handles fallback to simulated streaming internally if needed
	t.Logger.DebugWithContext(ctx, "Using streaming orchestration", map[string]interface{}{
		"operation":  "process_chat",
		"session_id": sessionID,
	})

	// Add per-request step callback to context for real-time tool progress
	// This sends SSE events as each tool completes during execution
	ctx = orchestration.WithStepCallback(ctx, func(stepIndex, totalSteps int, step orchestration.RoutingStep, stepResult orchestration.StepResult) {
		callback.SendStep(
			fmt.Sprintf("step_%d", stepIndex+1),
			step.AgentName,
			stepResult.Success,
			stepResult.Duration.Milliseconds(),
		)
	})

	// Pass queryWithContext (includes history) to the orchestrator
	result, err := orch.ProcessRequestStreaming(ctx, queryWithContext, nil, func(chunk core.StreamChunk) error {
		// Forward content chunks to SSE callback
		if chunk.Content != "" {
			callback.SendChunk(chunk.Content)
		}
		return nil
	})
	if err != nil {
		t.Logger.ErrorWithContext(ctx, "Streaming orchestration failed", map[string]interface{}{
			"operation":   "process_chat",
			"error":       err.Error(),
			"duration_ms": time.Since(startTime).Milliseconds(),
		})
		telemetry.RecordSpanError(ctx, err)
		return fmt.Errorf("streaming orchestration failed: %w", err)
	}

	response := result.Response
	requestID := result.RequestID
	agentsInvolved := result.AgentsInvolved
	confidence := result.Confidence
	executionTime := result.ExecutionTime

	// Log streaming stats
	t.Logger.DebugWithContext(ctx, "Streaming completed", map[string]interface{}{
		"operation":        "process_chat",
		"chunks_delivered": result.ChunksDelivered,
		"stream_completed": result.StreamCompleted,
		"partial_content":  result.PartialContent,
		"finish_reason":    result.FinishReason,
	})

	// Note: Step events are now sent in real-time via the context callback
	// (WithStepCallback above) so we don't need to send them again here.

	// Send usage stats if available
	if result.Usage != nil {
		callback.SendUsage(
			result.Usage.PromptTokens,
			result.Usage.CompletionTokens,
			result.Usage.TotalTokens,
		)
	}

	// Send finish reason if available
	if result.FinishReason != "" {
		callback.SendFinish(result.FinishReason)
	}

	// Add span event for completion
	telemetry.AddSpanEvent(ctx, "orchestration.completed",
		attribute.String("request_id", requestID),
		attribute.Int("agents_used", len(agentsInvolved)),
		attribute.Float64("confidence", confidence),
		attribute.String("execution_time", executionTime.String()),
	)

	// Send completion
	callback.SendDone(requestID, agentsInvolved, time.Since(startTime).Milliseconds())

	// Store assistant message in session
	t.sessionStore.AddMessage(sessionID, Message{
		Role:      "assistant",
		Content:   response,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"request_id":  requestID,
			"tools_used":  agentsInvolved,
			"confidence":  confidence,
			"duration_ms": time.Since(startTime).Milliseconds(),
		},
	})

	t.Logger.InfoWithContext(ctx, "Chat request completed", map[string]interface{}{
		"operation":   "process_chat",
		"session_id":  sessionID,
		"request_id":  requestID,
		"tools_used":  len(agentsInvolved),
		"duration_ms": time.Since(startTime).Milliseconds(),
		"status":      "success",
	})

	return nil
}

// registerCapabilities registers the agent's HTTP endpoints.
func (t *TravelChatAgent) registerCapabilities() {
	// SSE streaming endpoint
	t.RegisterCapability(core.Capability{
		Name:        "chat_stream",
		Description: "SSE streaming chat endpoint for travel queries",
		Endpoint:    "/chat/stream",
		Handler:     NewSSEHandler(t).ServeHTTP,
		Internal:    true, // Don't include in orchestrator's tool catalog
	})

	// Session management endpoints
	t.RegisterCapability(core.Capability{
		Name:        "create_session",
		Description: "Create a new chat session",
		Endpoint:    "/chat/session",
		Handler:     t.handleCreateSession,
		Internal:    true,
	})

	t.RegisterCapability(core.Capability{
		Name:        "get_session",
		Description: "Get session information",
		Endpoint:    "/chat/session/{id}",
		Handler:     t.handleGetSession,
		Internal:    true,
	})

	t.RegisterCapability(core.Capability{
		Name:        "get_history",
		Description: "Get conversation history for a session",
		Endpoint:    "/chat/session/{id}/history",
		Handler:     t.handleGetHistory,
		Internal:    true,
	})

	// Health and discovery
	t.RegisterCapability(core.Capability{
		Name:        "health",
		Description: "Health check with orchestrator status",
		Endpoint:    "/health",
		Handler:     t.handleHealth,
		Internal:    true,
	})

	t.RegisterCapability(core.Capability{
		Name:        "discover",
		Description: "Discover available tools",
		Endpoint:    "/discover",
		Handler:     t.handleDiscover,
		Internal:    true,
	})
}

// GetOrchestrator returns the orchestrator (for handlers that need it).
func (t *TravelChatAgent) GetOrchestrator() *orchestration.AIOrchestrator {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.orchestrator
}

// GetSessionStore returns the session store.
func (t *TravelChatAgent) GetSessionStore() *SessionStore {
	return t.sessionStore
}
