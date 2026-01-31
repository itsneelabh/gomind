package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/orchestration"
)

//go:embed static/*
var staticFiles embed.FS

// ServiceInfo represents a registered service in the registry
type ServiceInfo struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"` // "tool" or "agent"
	Description  string                 `json:"description"`
	Address      string                 `json:"address"`
	Port         int                    `json:"port"`
	Capabilities []Capability           `json:"capabilities"`
	Metadata     map[string]interface{} `json:"metadata"`
	Health       string                 `json:"health"`
	LastSeen     time.Time              `json:"last_seen"`
}

// Capability represents a service capability
type Capability struct {
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Version      string        `json:"version,omitempty"`
	Endpoint     string        `json:"endpoint,omitempty"`
	InputTypes   []string      `json:"input_types,omitempty"`
	OutputTypes  []string      `json:"output_types,omitempty"`
	InputSummary *InputSummary `json:"input_summary,omitempty"`
}

// InputSummary contains parameter information
type InputSummary struct {
	Required []ParamInfo `json:"required,omitempty"`
	Optional []ParamInfo `json:"optional,omitempty"`
}

// ParamInfo describes a parameter
type ParamInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Example     string `json:"example,omitempty"`
}

// RegistryResponse is the API response format
type RegistryResponse struct {
	Services   []ServiceInfo `json:"services"`
	TotalCount int           `json:"totalCount"`
	ToolCount  int           `json:"toolCount"`
	AgentCount int           `json:"agentCount"`
	Timestamp  time.Time     `json:"timestamp"`
}

// ============================================================================
// LLM Debug Types - using orchestration package types
// ============================================================================

// LLMDebugListResponse is the API response for listing debug records
type LLMDebugListResponse struct {
	Records   []orchestration.LLMDebugRecordSummary `json:"records"`
	Total     int                                   `json:"total"`
	Timestamp time.Time                             `json:"timestamp"`
}

// Redis database constants
// Most constants come from core package; HITL is local (no core constant exists)
const (
	RedisDBHITL = 6 // HITL uses DB 6 (no core constant, matches GOMIND_HITL_REDIS_DB default)
)

// Redis key patterns for LLM Debug (mirrors orchestration/redis_llm_debug_store.go)
const (
	llmDebugKeyPrefix = "gomind:llm:debug:"
	llmDebugIndexKey  = "gomind:llm:debug:index"
)

// Redis key patterns for HITL (mirrors orchestration/hitl_checkpoint_store.go)
const (
	hitlKeyPrefix    = "gomind:hitl"
	hitlPendingIndex = "gomind:hitl:pending"
)

// ============================================================================
// HITL Checkpoint Types (local types for API responses)
// Note: These are kept local due to UI-specific fields and structural differences
// from framework types (typed strings vs plain strings, different field names)
// ============================================================================

// HITLCheckpoint represents an execution checkpoint awaiting human approval
type HITLCheckpoint struct {
	CheckpointID       string                 `json:"checkpoint_id"`
	RequestID          string                 `json:"request_id"`
	InterruptPoint     string                 `json:"interrupt_point"`
	Decision           *InterruptDecision     `json:"decision,omitempty"`
	Plan               *RoutingPlan           `json:"plan,omitempty"`
	CompletedSteps     []StepResult           `json:"completed_steps,omitempty"`
	CurrentStep        *RoutingStep           `json:"current_step,omitempty"`
	CurrentStepResult  *StepResult            `json:"current_step_result,omitempty"`
	StepResults        map[string]*StepResult `json:"step_results,omitempty"`
	ResolvedParameters map[string]interface{} `json:"resolved_parameters,omitempty"`
	OriginalRequest    string                 `json:"original_request"`
	UserContext        map[string]interface{} `json:"user_context,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
	ExpiresAt          time.Time              `json:"expires_at"`
	Status             string                 `json:"status"`
	AgentName          string                 `json:"agent_name,omitempty"` // Extracted from key prefix
}

// InterruptDecision contains the decision context for an interrupt
type InterruptDecision struct {
	ShouldInterrupt bool                   `json:"should_interrupt"`
	Reason          string                 `json:"reason"`
	Message         string                 `json:"message"`
	Priority        string                 `json:"priority"`
	Timeout         int64                  `json:"timeout,omitempty"`
	DefaultAction   string                 `json:"default_action,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// RoutingPlan represents an LLM-generated execution plan
type RoutingPlan struct {
	PlanID            string        `json:"plan_id,omitempty"`
	RequestID         string        `json:"request_id,omitempty"`
	OriginalRequest   string        `json:"original_request,omitempty"`
	Mode              string        `json:"mode,omitempty"`
	Steps             []RoutingStep `json:"steps"`
	SynthesisStrategy string        `json:"synthesis_strategy,omitempty"`
	Rationale         string        `json:"rationale,omitempty"`
	CreatedAt         *time.Time    `json:"created_at,omitempty"`
}

// RoutingStep represents a single step in an execution plan
type RoutingStep struct {
	StepID         string                 `json:"step_id"`
	Capability     string                 `json:"capability"`
	ServiceName    string                 `json:"service_name,omitempty"`
	AgentName      string                 `json:"agent_name,omitempty"`
	Namespace      string                 `json:"namespace,omitempty"`
	CapabilityName string                 `json:"capability_name,omitempty"`
	Instruction    string                 `json:"instruction,omitempty"`
	Parameters     map[string]interface{} `json:"parameters,omitempty"`
	DependsOn      []string               `json:"depends_on,omitempty"`
	Description    string                 `json:"description,omitempty"`
	ExpectedOutput string                 `json:"expected_output,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// StepResult represents the result of executing a step
type StepResult struct {
	StepID       string                 `json:"step_id"`
	Capability   string                 `json:"capability"`
	ServiceName  string                 `json:"service_name,omitempty"`
	AgentName    string                 `json:"agent_name,omitempty"`
	Namespace    string                 `json:"namespace,omitempty"`
	Instruction  string                 `json:"instruction,omitempty"`
	Success      bool                   `json:"success"`
	Response     interface{}            `json:"response,omitempty"`
	ResponseText string                 `json:"response_text,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Duration     int64                  `json:"duration,omitempty"`
	DurationMs   int64                  `json:"duration_ms,omitempty"`
	Attempts     int                    `json:"attempts,omitempty"`
	StartTime    *time.Time             `json:"start_time,omitempty"`
	EndTime      *time.Time             `json:"end_time,omitempty"`
	Skipped      bool                   `json:"skipped,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// HITLCheckpointSummary is a lightweight version for listing
type HITLCheckpointSummary struct {
	CheckpointID    string    `json:"checkpoint_id"`
	RequestID       string    `json:"request_id"`
	InterruptPoint  string    `json:"interrupt_point"`
	Reason          string    `json:"reason"`
	Priority        string    `json:"priority"`
	Message         string    `json:"message"`
	OriginalRequest string    `json:"original_request"`
	StepCount       int       `json:"step_count"`
	CompletedCount  int       `json:"completed_count"`
	CurrentStep     string    `json:"current_step,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	Status          string    `json:"status"`
	AgentName       string    `json:"agent_name,omitempty"` // Extracted from key prefix
}

// HITLCheckpointListResponse is the API response for listing checkpoints
type HITLCheckpointListResponse struct {
	Checkpoints []HITLCheckpointSummary `json:"checkpoints"`
	Total       int                     `json:"total"`
	Timestamp   time.Time               `json:"timestamp"`
}

// ============================================================================
// Execution DAG Types (mirrors orchestration/execution_store.go)
// ============================================================================

// Redis key patterns for Execution DAG (mirrors orchestration/redis_execution_store.go)
const (
	executionKeyPrefix   = "gomind:execution:debug:"
	executionIndexKey    = "gomind:execution:debug:index"
	executionTracePrefix = "gomind:execution:debug:trace:"
)

// StoredExecution contains everything needed for DAG visualization
type StoredExecution struct {
	RequestID         string            `json:"request_id"`
	OriginalRequestID string            `json:"original_request_id,omitempty"`
	TraceID           string            `json:"trace_id"`
	AgentName         string            `json:"agent_name,omitempty"`
	OriginalRequest   string            `json:"original_request"`
	Plan              *RoutingPlan      `json:"plan"`
	Result            *ExecutionResult  `json:"result"`
	Interrupted       bool              `json:"interrupted,omitempty"` // True if execution was interrupted for HITL
	Checkpoint        *HITLCheckpoint   `json:"checkpoint,omitempty"`  // Checkpoint data if interrupted
	CreatedAt         time.Time         `json:"created_at"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

// ExecutionResult represents the outcome of plan execution
type ExecutionResult struct {
	PlanID        string                 `json:"plan_id"`
	Steps         []StepResult           `json:"steps"`
	Success       bool                   `json:"success"`
	TotalDuration int64                  `json:"total_duration"` // nanoseconds
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// ExecutionSummary is a lightweight version for listing
type ExecutionSummary struct {
	RequestID         string    `json:"request_id"`
	OriginalRequestID string    `json:"original_request_id,omitempty"`
	TraceID           string    `json:"trace_id"`
	AgentName         string    `json:"agent_name,omitempty"`
	OriginalRequest   string    `json:"original_request"`
	Success           bool      `json:"success"`
	Interrupted       bool      `json:"interrupted,omitempty"` // True if execution was interrupted for HITL
	StepCount         int       `json:"step_count"`
	FailedSteps       int       `json:"failed_steps"`
	TotalDurationMs   int64     `json:"total_duration_ms"`
	CreatedAt         time.Time `json:"created_at"`
}

// ExecutionListResponse is the API response for listing executions
type ExecutionListResponse struct {
	Executions []ExecutionSummary `json:"executions"`
	Total      int                `json:"total"`
	HasMore    bool               `json:"has_more"`
	Timestamp  time.Time          `json:"timestamp"`
}

// DAGNode represents a node in the DAG visualization
type DAGNode struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Instruction string `json:"instruction"`
	Status      string `json:"status"`
	DurationMs  int64  `json:"duration_ms"`
	Level       int    `json:"level"`
}

// DAGEdge represents an edge in the DAG visualization
type DAGEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// DAGStatistics contains computed statistics for the DAG
type DAGStatistics struct {
	TotalNodes     int `json:"total_nodes"`
	CompletedNodes int `json:"completed_nodes"`
	FailedNodes    int `json:"failed_nodes"`
	SkippedNodes   int `json:"skipped_nodes"`
	MaxParallelism int `json:"max_parallelism"`
	Depth          int `json:"depth"`
}

// DAGResponse is the computed DAG structure for visualization
type DAGResponse struct {
	Nodes      []DAGNode     `json:"nodes"`
	Edges      []DAGEdge     `json:"edges"`
	Levels     [][]string    `json:"levels"`
	Statistics DAGStatistics `json:"statistics"`
}

// UnifiedExecutionView combines all related data for a single request view
// This provides a "one-stop shop" for debugging and understanding request execution
type UnifiedExecutionView struct {
	// Core execution data
	RequestID         string           `json:"request_id"`
	OriginalRequestID string           `json:"original_request_id,omitempty"`
	TraceID           string           `json:"trace_id,omitempty"`
	AgentName         string           `json:"agent_name,omitempty"`
	OriginalRequest   string           `json:"original_request"`
	CreatedAt         time.Time        `json:"created_at"`
	Success           bool             `json:"success"`
	TotalDurationMs   int64            `json:"total_duration_ms"`
	Plan              *RoutingPlan     `json:"plan,omitempty"`
	Result            *ExecutionResult `json:"result,omitempty"`
	Interrupted       bool             `json:"interrupted,omitempty"` // True if execution was interrupted for HITL
	Checkpoint        *HITLCheckpoint  `json:"checkpoint,omitempty"`  // Checkpoint data if interrupted (includes completed_steps, step_results)

	// Computed DAG structure
	DAG *DAGResponse `json:"dag,omitempty"`

	// LLM interactions (from LLM Debug store)
	LLMInteractions []orchestration.LLMInteraction `json:"llm_interactions,omitempty"`
	LLMDebugSummary *LLMDebugSummary               `json:"llm_debug_summary,omitempty"`

	// HITL checkpoints (if any)
	HITLCheckpoints []HITLCheckpoint `json:"hitl_checkpoints,omitempty"`

	// Metadata for UI
	HasLLMData  bool `json:"has_llm_data"`
	HasHITLData bool `json:"has_hitl_data"`
}

// LLMDebugSummary provides a summary of LLM interactions
type LLMDebugSummary struct {
	TotalCalls        int            `json:"total_calls"`
	TotalTokensIn     int            `json:"total_tokens_in"`
	TotalTokensOut    int            `json:"total_tokens_out"`
	TotalDurationMs   int64          `json:"total_duration_ms"`
	ProviderBreakdown map[string]int `json:"provider_breakdown"` // provider -> call count
}

var (
	useMock   bool
	redisURL  string
	namespace string
	port      int
)

func init() {
	flag.BoolVar(&useMock, "mock", true, "Use mock data instead of Redis")
	flag.StringVar(&redisURL, "redis-url", "", "Redis/Valkey URL (required when -mock=false, or set REDIS_URL env var)")
	flag.StringVar(&namespace, "namespace", "gomind", "Redis key namespace")
	flag.IntVar(&port, "port", 8100, "HTTP server port")
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getEnvBool returns environment variable as bool
func getEnvBool(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	// Accept various truthy/falsy values
	switch strings.ToLower(val) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	}
	return defaultVal
}

// getEnvInt returns environment variable as int
func getEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	if i, err := strconv.Atoi(val); err == nil {
		return i
	}
	return defaultVal
}

func main() {
	flag.Parse()

	// Environment variables override command-line flags
	// This allows K8s ConfigMaps/Secrets to configure the app
	if envRedisURL := os.Getenv("REDIS_URL"); envRedisURL != "" {
		redisURL = envRedisURL
	}
	if envNamespace := os.Getenv("REDIS_NAMESPACE"); envNamespace != "" {
		namespace = envNamespace
	}
	if envPort := getEnvInt("PORT", 0); envPort != 0 {
		port = envPort
	}
	// USE_MOCK env var: "false" or "0" disables mock mode
	if envMock := os.Getenv("USE_MOCK"); envMock != "" {
		useMock = getEnvBool("USE_MOCK", useMock)
	}

	// Validate Redis URL is provided when not in mock mode
	if !useMock && redisURL == "" {
		log.Fatalf("REDIS_URL environment variable or -redis-url flag is required when not using mock mode")
	}

	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/services", handleServices)
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/llm-debug", handleLLMDebugList)
	mux.HandleFunc("/api/llm-debug/", handleLLMDebugRecord)
	mux.HandleFunc("/api/hitl/checkpoints", handleHITLCheckpointList)
	mux.HandleFunc("/api/hitl/checkpoints/", handleHITLCheckpoint)
	mux.HandleFunc("/api/executions", handleExecutionList)
	mux.HandleFunc("/api/executions/search", handleExecutionSearch)
	mux.HandleFunc("/api/executions/", handleExecution) // Handles both /{id} and /{id}/dag

	// Static files - use fs.Sub to strip "static/" prefix from embedded FS
	staticContent, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to create static file server: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticContent)))

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting Registry Viewer on http://localhost%s", addr)
	log.Printf("Mode: %s", map[bool]string{true: "MOCK", false: "REDIS"}[useMock])
	if !useMock {
		log.Printf("Redis URL: %s", redisURL)
		log.Printf("Redis Namespace: %s", namespace)
	}

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleServices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var services []ServiceInfo
	var err error

	if useMock {
		services = getMockServices()
	} else {
		services, err = getRedisServices()
		if err != nil {
			http.Error(w, fmt.Sprintf("Redis error: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Sort by type (agents first), then by name
	sort.Slice(services, func(i, j int) bool {
		if services[i].Type != services[j].Type {
			return services[i].Type == "agent"
		}
		return services[i].Name < services[j].Name
	})

	toolCount := 0
	agentCount := 0
	for _, s := range services {
		if s.Type == "tool" {
			toolCount++
		} else {
			agentCount++
		}
	}

	response := RegistryResponse{
		Services:   services,
		TotalCount: len(services),
		ToolCount:  toolCount,
		AgentCount: agentCount,
		Timestamp:  time.Now(),
	}

	json.NewEncoder(w).Encode(response)
}

// getMockServices returns mock service data for development
func getMockServices() []ServiceInfo {
	now := time.Now()
	return []ServiceInfo{
		{
			ID:          "weather-tool-abc123",
			Name:        "weather-tool",
			Type:        "tool",
			Description: "Provides current weather information for any location",
			Address:     "weather-tool-service.gomind-examples",
			Port:        80,
			Capabilities: []Capability{
				{Name: "get-weather", Description: "Get current weather for a location", Version: "1.0.0"},
				{Name: "get-forecast", Description: "Get weather forecast", Version: "1.0.0"},
			},
			Metadata: map[string]interface{}{
				"provider": "openweathermap",
				"version":  "2.1.0",
			},
			Health:   "healthy",
			LastSeen: now.Add(-5 * time.Second),
		},
		{
			ID:          "geocoding-tool-def456",
			Name:        "geocoding-tool",
			Type:        "tool",
			Description: "Converts addresses to coordinates and vice versa",
			Address:     "geocoding-tool-service.gomind-examples",
			Port:        80,
			Capabilities: []Capability{
				{Name: "geocode", Description: "Convert address to coordinates", Version: "1.0.0"},
				{Name: "reverse-geocode", Description: "Convert coordinates to address", Version: "1.0.0"},
			},
			Metadata: map[string]interface{}{
				"provider": "nominatim",
				"version":  "1.5.0",
			},
			Health:   "healthy",
			LastSeen: now.Add(-8 * time.Second),
		},
		{
			ID:          "stock-market-tool-ghi789",
			Name:        "stock-market-tool",
			Type:        "tool",
			Description: "Provides stock market data and quotes",
			Address:     "stock-market-tool-service.gomind-examples",
			Port:        80,
			Capabilities: []Capability{
				{Name: "get-quote", Description: "Get stock quote", Version: "1.0.0"},
				{Name: "get-history", Description: "Get historical prices", Version: "1.0.0"},
			},
			Metadata: map[string]interface{}{
				"provider": "alphavantage",
				"version":  "1.2.0",
			},
			Health:   "healthy",
			LastSeen: now.Add(-12 * time.Second),
		},
		{
			ID:          "news-tool-jkl012",
			Name:        "news-tool",
			Type:        "tool",
			Description: "Fetches latest news articles",
			Address:     "news-tool-service.gomind-examples",
			Port:        80,
			Capabilities: []Capability{
				{Name: "search-news", Description: "Search for news articles", Version: "1.0.0"},
				{Name: "get-headlines", Description: "Get top headlines", Version: "1.0.0"},
			},
			Metadata: map[string]interface{}{
				"provider": "newsapi",
				"version":  "1.0.0",
			},
			Health:   "unhealthy",
			LastSeen: now.Add(-45 * time.Second),
		},
		{
			ID:          "currency-tool-mno345",
			Name:        "currency-tool",
			Type:        "tool",
			Description: "Currency conversion and exchange rates",
			Address:     "currency-tool-service.gomind-examples",
			Port:        80,
			Capabilities: []Capability{
				{Name: "convert", Description: "Convert between currencies", Version: "1.0.0"},
				{Name: "get-rates", Description: "Get exchange rates", Version: "1.0.0"},
			},
			Metadata: map[string]interface{}{
				"provider": "exchangerate-api",
				"version":  "1.1.0",
			},
			Health:   "healthy",
			LastSeen: now.Add(-3 * time.Second),
		},
		{
			ID:          "research-agent-pqr678",
			Name:        "research-agent",
			Type:        "agent",
			Description: "AI agent that performs research tasks using available tools",
			Address:     "research-agent-service.gomind-examples",
			Port:        8090,
			Capabilities: []Capability{
				{Name: "research", Description: "Conduct research on a topic", Version: "1.0.0"},
				{Name: "summarize", Description: "Summarize information", Version: "1.0.0"},
			},
			Metadata: map[string]interface{}{
				"llm_provider": "openai",
				"model":        "gpt-4",
				"version":      "3.0.0",
			},
			Health:   "healthy",
			LastSeen: now.Add(-2 * time.Second),
		},
		{
			ID:          "travel-agent-stu901",
			Name:        "travel-agent",
			Type:        "agent",
			Description: "AI agent that helps plan travel itineraries",
			Address:     "travel-agent-service.gomind-examples",
			Port:        8090,
			Capabilities: []Capability{
				{Name: "plan-trip", Description: "Plan a travel itinerary", Version: "1.0.0"},
				{Name: "find-flights", Description: "Search for flights", Version: "1.0.0"},
				{Name: "book-hotel", Description: "Find and book hotels", Version: "1.0.0"},
			},
			Metadata: map[string]interface{}{
				"llm_provider": "anthropic",
				"model":        "claude-3",
				"version":      "2.0.0",
			},
			Health:   "healthy",
			LastSeen: now.Add(-7 * time.Second),
		},
	}
}

// Redis client singleton
var (
	redisClient     *redis.Client
	redisClientOnce sync.Once
)

func getRedisClient() (*redis.Client, error) {
	var initErr error
	redisClientOnce.Do(func() {
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			initErr = fmt.Errorf("invalid redis URL: %w", err)
			return
		}
		opt.DB = 0 // Service discovery uses DB 0
		redisClient = redis.NewClient(opt)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			initErr = fmt.Errorf("redis connection failed: %w", err)
			redisClient = nil
		}
	})
	if initErr != nil {
		return nil, initErr
	}
	if redisClient == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}
	return redisClient, nil
}

// getRedisServices fetches services from Redis registry
func getRedisServices() ([]ServiceInfo, error) {
	client, err := getRedisClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Scan for all service keys
	pattern := fmt.Sprintf("%s:services:*", namespace)
	var services []ServiceInfo
	var cursor uint64

	for {
		keys, nextCursor, err := client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		for _, key := range keys {
			data, err := client.Get(ctx, key).Result()
			if err == redis.Nil {
				continue // Key expired
			}
			if err != nil {
				log.Printf("Warning: failed to get key %s: %v", key, err)
				continue
			}

			var service ServiceInfo
			if err := json.Unmarshal([]byte(data), &service); err != nil {
				log.Printf("Warning: failed to parse service data for %s: %v", key, err)
				continue
			}

			// Extract ID from key if not set
			if service.ID == "" {
				parts := strings.Split(key, ":")
				if len(parts) >= 3 {
					service.ID = parts[len(parts)-1]
				}
			}

			services = append(services, service)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return services, nil
}

// ============================================================================
// LLM Debug Redis Client and Handlers
// ============================================================================

// LLM Debug Redis client singleton (separate from service discovery client)
var (
	llmDebugClient     *redis.Client
	llmDebugClientOnce sync.Once
)

func getLLMDebugClient() (*redis.Client, error) {
	var initErr error
	llmDebugClientOnce.Do(func() {
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			initErr = fmt.Errorf("invalid redis URL: %w", err)
			return
		}
		opt.DB = core.RedisDBLLMDebug // Use DB 7 for LLM Debug
		llmDebugClient = redis.NewClient(opt)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := llmDebugClient.Ping(ctx).Err(); err != nil {
			initErr = fmt.Errorf("redis connection failed (DB %d): %w", core.RedisDBLLMDebug, err)
			llmDebugClient = nil
		}
	})
	if initErr != nil {
		return nil, initErr
	}
	if llmDebugClient == nil {
		return nil, fmt.Errorf("llm debug redis client not initialized")
	}
	return llmDebugClient, nil
}

// Execution Debug Redis client singleton (uses DB 8)
var (
	executionDebugClient     *redis.Client
	executionDebugClientOnce sync.Once
)

func getExecutionDebugClient() (*redis.Client, error) {
	var initErr error
	executionDebugClientOnce.Do(func() {
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			initErr = fmt.Errorf("invalid redis URL: %w", err)
			return
		}
		opt.DB = core.RedisDBExecutionDebug // Use DB 8 for Execution Debug
		executionDebugClient = redis.NewClient(opt)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := executionDebugClient.Ping(ctx).Err(); err != nil {
			initErr = fmt.Errorf("redis connection failed (DB %d): %w", core.RedisDBExecutionDebug, err)
			executionDebugClient = nil
		}
	})
	if initErr != nil {
		return nil, initErr
	}
	if executionDebugClient == nil {
		return nil, fmt.Errorf("execution debug redis client not initialized")
	}
	return executionDebugClient, nil
}

// handleLLMDebugList returns a list of recent LLM debug records
func handleLLMDebugList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Parse limit parameter (default 50)
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	var records []orchestration.LLMDebugRecordSummary
	var err error

	if useMock {
		records = getMockLLMDebugSummaries()
	} else {
		records, err = getRedisLLMDebugSummaries(limit)
		if err != nil {
			http.Error(w, fmt.Sprintf("Redis error: %v", err), http.StatusInternalServerError)
			return
		}
	}

	response := LLMDebugListResponse{
		Records:   records,
		Total:     len(records),
		Timestamp: time.Now(),
	}

	json.NewEncoder(w).Encode(response)
}

// handleLLMDebugRecord returns a specific LLM debug record by ID
func handleLLMDebugRecord(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Extract request ID from URL path: /api/llm-debug/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/llm-debug/")
	requestID := strings.TrimSpace(path)

	if requestID == "" {
		http.Error(w, "request_id is required", http.StatusBadRequest)
		return
	}

	var record *orchestration.LLMDebugRecord
	var err error

	if useMock {
		record = getMockLLMDebugRecord(requestID)
		if record == nil {
			http.Error(w, fmt.Sprintf("record not found: %s", requestID), http.StatusNotFound)
			return
		}
	} else {
		record, err = getRedisLLMDebugRecord(requestID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, fmt.Sprintf("record not found: %s", requestID), http.StatusNotFound)
			} else {
				http.Error(w, fmt.Sprintf("Redis error: %v", err), http.StatusInternalServerError)
			}
			return
		}
	}

	json.NewEncoder(w).Encode(record)
}

// getRedisLLMDebugSummaries fetches recent debug record summaries from Redis
func getRedisLLMDebugSummaries(limit int) ([]orchestration.LLMDebugRecordSummary, error) {
	client, err := getLLMDebugClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get recent request IDs from sorted set (newest first)
	ids, err := client.ZRevRange(ctx, llmDebugIndexKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list debug records: %w", err)
	}

	summaries := make([]orchestration.LLMDebugRecordSummary, 0, len(ids))
	for _, id := range ids {
		record, err := getRedisLLMDebugRecord(id)
		if err != nil {
			log.Printf("Warning: skipping record %s: %v", id, err)
			continue // Skip missing records (TTL expired)
		}

		totalTokens := 0
		hasErrors := false
		for _, interaction := range record.Interactions {
			totalTokens += interaction.TotalTokens
			if !interaction.Success {
				hasErrors = true
			}
		}

		summaries = append(summaries, orchestration.LLMDebugRecordSummary{
			RequestID:         record.RequestID,
			OriginalRequestID: record.OriginalRequestID,
			TraceID:           record.TraceID,
			CreatedAt:         record.CreatedAt,
			InteractionCount:  len(record.Interactions),
			TotalTokens:       totalTokens,
			HasErrors:         hasErrors,
		})
	}

	return summaries, nil
}

// getRedisLLMDebugRecord fetches a single debug record from Redis
func getRedisLLMDebugRecord(requestID string) (*orchestration.LLMDebugRecord, error) {
	client, err := getLLMDebugClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := llmDebugKeyPrefix + requestID
	data, err := client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("record not found: %s", requestID)
	}
	if err != nil {
		return nil, fmt.Errorf("redis get failed: %w", err)
	}

	return deserializeLLMDebugRecord(data)
}

// deserializeLLMDebugRecord deserializes a debug record with optional gzip decompression
// Format: first byte is compression flag (0=raw, 1=gzip), rest is JSON
func deserializeLLMDebugRecord(data []byte) (*orchestration.LLMDebugRecord, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	var jsonData []byte
	if data[0] == 1 { // Compressed
		gz, err := gzip.NewReader(bytes.NewReader(data[1:]))
		if err != nil {
			return nil, fmt.Errorf("gzip reader failed: %w", err)
		}
		defer gz.Close()

		var buf bytes.Buffer
		if _, err := buf.ReadFrom(gz); err != nil {
			return nil, fmt.Errorf("gzip decompress failed: %w", err)
		}
		jsonData = buf.Bytes()
	} else {
		jsonData = data[1:]
	}

	var record orchestration.LLMDebugRecord
	if err := json.Unmarshal(jsonData, &record); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w", err)
	}
	return &record, nil
}

// ============================================================================
// LLM Debug Mock Data
// ============================================================================

// getMockLLMDebugSummaries returns mock summaries for development
func getMockLLMDebugSummaries() []orchestration.LLMDebugRecordSummary {
	now := time.Now()
	return []orchestration.LLMDebugRecordSummary{
		{
			RequestID:         "req-abc123",
			OriginalRequestID: "req-abc123", // Same as RequestID for initial request
			TraceID:           "369fecb4e3156c34e0950c61f1f99d62",
			CreatedAt:         now.Add(-5 * time.Minute),
			InteractionCount:  3,
			TotalTokens:       2847,
			HasErrors:         false,
		},
		{
			RequestID:         "req-def456",
			OriginalRequestID: "req-abc123", // Part of same HITL conversation (resume)
			TraceID:           "7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d",
			CreatedAt:         now.Add(-15 * time.Minute),
			InteractionCount:  2,
			TotalTokens:       1523,
			HasErrors:         false,
		},
		{
			RequestID:         "req-ghi789",
			OriginalRequestID: "req-ghi789", // Different conversation
			TraceID:           "1234567890abcdef1234567890abcdef",
			CreatedAt:         now.Add(-1 * time.Hour),
			InteractionCount:  4,
			TotalTokens:       4102,
			HasErrors:         true,
		},
	}
}

// ============================================================================
// HITL Checkpoint Redis Client and Handlers
// ============================================================================

// HITL Redis client singleton (separate from other clients)
var (
	hitlClient     *redis.Client
	hitlClientOnce sync.Once
)

func getHITLClient() (*redis.Client, error) {
	var initErr error
	hitlClientOnce.Do(func() {
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			initErr = fmt.Errorf("invalid redis URL: %w", err)
			return
		}
		opt.DB = RedisDBHITL // Use DB 6 for HITL
		hitlClient = redis.NewClient(opt)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := hitlClient.Ping(ctx).Err(); err != nil {
			initErr = fmt.Errorf("redis connection failed (DB %d): %w", RedisDBHITL, err)
			hitlClient = nil
		}
	})
	if initErr != nil {
		return nil, initErr
	}
	if hitlClient == nil {
		return nil, fmt.Errorf("hitl redis client not initialized")
	}
	return hitlClient, nil
}

// handleHITLCheckpointList returns a list of pending HITL checkpoints
func handleHITLCheckpointList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var checkpoints []HITLCheckpointSummary
	var err error

	if useMock {
		checkpoints = getMockHITLCheckpointSummaries()
	} else {
		checkpoints, err = getRedisHITLCheckpointSummaries()
		if err != nil {
			http.Error(w, fmt.Sprintf("Redis error: %v", err), http.StatusInternalServerError)
			return
		}
	}

	response := HITLCheckpointListResponse{
		Checkpoints: checkpoints,
		Total:       len(checkpoints),
		Timestamp:   time.Now(),
	}

	json.NewEncoder(w).Encode(response)
}

// handleHITLCheckpoint returns a specific HITL checkpoint by ID
func handleHITLCheckpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Extract checkpoint ID from URL path: /api/hitl/checkpoints/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/hitl/checkpoints/")
	checkpointID := strings.TrimSpace(path)

	if checkpointID == "" {
		http.Error(w, "checkpoint_id is required", http.StatusBadRequest)
		return
	}

	var checkpoint *HITLCheckpoint
	var err error

	if useMock {
		checkpoint = getMockHITLCheckpoint(checkpointID)
		if checkpoint == nil {
			http.Error(w, fmt.Sprintf("checkpoint not found: %s", checkpointID), http.StatusNotFound)
			return
		}
	} else {
		checkpoint, err = getRedisHITLCheckpoint(checkpointID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, fmt.Sprintf("checkpoint not found: %s", checkpointID), http.StatusNotFound)
			} else {
				http.Error(w, fmt.Sprintf("Redis error: %v", err), http.StatusInternalServerError)
			}
			return
		}
	}

	json.NewEncoder(w).Encode(checkpoint)
}

// ============================================================================
// Execution DAG Handlers
// ============================================================================

// handleExecutionList returns a list of recent executions
func handleExecutionList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	var summaries []ExecutionSummary
	var err error

	if useMock {
		summaries = getMockExecutionSummaries()
	} else {
		summaries, err = getRedisExecutionSummaries(limit)
		if err != nil {
			http.Error(w, fmt.Sprintf("Redis error: %v", err), http.StatusInternalServerError)
			return
		}
	}

	response := ExecutionListResponse{
		Executions: summaries,
		Total:      len(summaries),
		HasMore:    len(summaries) >= limit,
		Timestamp:  time.Now(),
	}

	json.NewEncoder(w).Encode(response)
}

// ExecutionSearchResponse is the API response for search results
type ExecutionSearchResponse struct {
	Executions []ExecutionSummary `json:"executions"`
	Query      string             `json:"query"`
	Total      int                `json:"total"`
	Timestamp  time.Time          `json:"timestamp"`
}

// handleExecutionSearch searches executions by original request content
func handleExecutionSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Parse query parameters
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	var summaries []ExecutionSummary
	var err error

	if useMock {
		summaries = searchMockExecutions(query, limit)
	} else {
		summaries, err = searchRedisExecutions(query, limit)
		if err != nil {
			http.Error(w, fmt.Sprintf("Redis error: %v", err), http.StatusInternalServerError)
			return
		}
	}

	response := ExecutionSearchResponse{
		Executions: summaries,
		Query:      query,
		Total:      len(summaries),
		Timestamp:  time.Now(),
	}

	json.NewEncoder(w).Encode(response)
}

// searchMockExecutions searches mock executions by original request content
func searchMockExecutions(query string, limit int) []ExecutionSummary {
	allSummaries := getMockExecutionSummaries()
	queryLower := strings.ToLower(query)

	var results []ExecutionSummary
	for _, summary := range allSummaries {
		if strings.Contains(strings.ToLower(summary.OriginalRequest), queryLower) {
			results = append(results, summary)
			if len(results) >= limit {
				break
			}
		}
	}
	return results
}

// searchRedisExecutions searches Redis executions by original request content
func searchRedisExecutions(query string, limit int) ([]ExecutionSummary, error) {
	// Get recent executions and filter by query
	// Note: For production, consider using Redis Search or a dedicated search index
	allSummaries, err := getRedisExecutionSummaries(1000) // Fetch more to search through
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	var results []ExecutionSummary
	for _, summary := range allSummaries {
		if strings.Contains(strings.ToLower(summary.OriginalRequest), queryLower) {
			results = append(results, summary)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// handleExecution handles GET /api/executions/{id}, /{id}/dag, and /{id}/unified
func handleExecution(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Parse URL path: /api/executions/{id}, /{id}/dag, or /{id}/unified
	path := strings.TrimPrefix(r.URL.Path, "/api/executions/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "request_id is required", http.StatusBadRequest)
		return
	}

	requestID := parts[0]
	isDAGRequest := len(parts) > 1 && parts[1] == "dag"
	isUnifiedRequest := len(parts) > 1 && parts[1] == "unified"

	var execution *StoredExecution
	var err error

	if useMock {
		execution = getMockExecution(requestID)
		if execution == nil {
			http.Error(w, fmt.Sprintf("execution not found: %s", requestID), http.StatusNotFound)
			return
		}
	} else {
		execution, err = getRedisExecution(requestID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, fmt.Sprintf("execution not found: %s", requestID), http.StatusNotFound)
			} else {
				http.Error(w, fmt.Sprintf("Redis error: %v", err), http.StatusInternalServerError)
			}
			return
		}
	}

	if isUnifiedRequest {
		// Return unified view combining execution, LLM debug, and HITL data
		unified := buildUnifiedView(execution)
		json.NewEncoder(w).Encode(unified)
	} else if isDAGRequest {
		// Return computed DAG structure
		dag := computeDAG(execution)
		json.NewEncoder(w).Encode(dag)
	} else {
		// Return full execution record
		json.NewEncoder(w).Encode(execution)
	}
}

// computeDAG builds the DAG structure from a stored execution
func computeDAG(execution *StoredExecution) *DAGResponse {
	if execution == nil || execution.Plan == nil {
		return &DAGResponse{
			Nodes:  []DAGNode{},
			Edges:  []DAGEdge{},
			Levels: [][]string{},
		}
	}

	// Build step result map for quick lookup
	stepResults := make(map[string]*StepResult)
	if execution.Result != nil {
		for i := range execution.Result.Steps {
			step := &execution.Result.Steps[i]
			stepResults[step.StepID] = step
		}
	}

	// Build adjacency list and in-degree map for topological sort
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for _, step := range execution.Plan.Steps {
		if _, exists := inDegree[step.StepID]; !exists {
			inDegree[step.StepID] = 0
		}
		for _, dep := range step.DependsOn {
			inDegree[step.StepID]++
			dependents[dep] = append(dependents[dep], step.StepID)
		}
	}

	// Compute levels using BFS (Kahn's algorithm)
	levels := [][]string{}
	currentLevel := []string{}

	// Find initial nodes (no dependencies)
	for _, step := range execution.Plan.Steps {
		if inDegree[step.StepID] == 0 {
			currentLevel = append(currentLevel, step.StepID)
		}
	}

	levelMap := make(map[string]int)
	for len(currentLevel) > 0 {
		levels = append(levels, currentLevel)
		levelIdx := len(levels) - 1
		for _, stepID := range currentLevel {
			levelMap[stepID] = levelIdx
		}

		nextLevel := []string{}
		for _, stepID := range currentLevel {
			for _, dep := range dependents[stepID] {
				inDegree[dep]--
				if inDegree[dep] == 0 {
					nextLevel = append(nextLevel, dep)
				}
			}
		}
		currentLevel = nextLevel
	}

	// Build nodes
	nodes := make([]DAGNode, 0, len(execution.Plan.Steps))
	statistics := DAGStatistics{
		TotalNodes: len(execution.Plan.Steps),
		Depth:      len(levels),
	}

	for _, step := range execution.Plan.Steps {
		status := "pending"
		var durationMs int64

		if result, ok := stepResults[step.StepID]; ok {
			if result.Success {
				status = "completed"
				statistics.CompletedNodes++
			} else if result.Error != "" {
				status = "failed"
				statistics.FailedNodes++
			} else if result.Skipped {
				status = "skipped"
				statistics.SkippedNodes++
			}
			durationMs = result.DurationMs
		}

		nodes = append(nodes, DAGNode{
			ID:          step.StepID,
			Label:       step.AgentName,
			Instruction: step.Instruction,
			Status:      status,
			DurationMs:  durationMs,
			Level:       levelMap[step.StepID],
		})
	}

	// Build edges
	edges := make([]DAGEdge, 0)
	for _, step := range execution.Plan.Steps {
		for _, dep := range step.DependsOn {
			edges = append(edges, DAGEdge{
				Source: dep,
				Target: step.StepID,
			})
		}
	}

	// Calculate max parallelism
	for _, level := range levels {
		if len(level) > statistics.MaxParallelism {
			statistics.MaxParallelism = len(level)
		}
	}

	return &DAGResponse{
		Nodes:      nodes,
		Edges:      edges,
		Levels:     levels,
		Statistics: statistics,
	}
}

// buildUnifiedView combines execution data with LLM debug and HITL checkpoint data
func buildUnifiedView(execution *StoredExecution) *UnifiedExecutionView {
	if execution == nil {
		return nil
	}

	unified := &UnifiedExecutionView{
		RequestID:         execution.RequestID,
		OriginalRequestID: execution.OriginalRequestID,
		TraceID:           execution.TraceID,
		AgentName:         execution.AgentName,
		OriginalRequest:   execution.OriginalRequest,
		CreatedAt:         execution.CreatedAt,
		Plan:              execution.Plan,
		Result:            execution.Result,
		Interrupted:       execution.Interrupted,
		Checkpoint:        execution.Checkpoint,
	}

	// Compute success and duration from result
	if execution.Result != nil {
		unified.Success = execution.Result.Success
		unified.TotalDurationMs = execution.Result.TotalDuration / 1_000_000 // ns to ms
	}

	// Compute DAG structure
	unified.DAG = computeDAG(execution)

	// Fetch LLM debug data (non-blocking - errors are logged but don't fail the request)
	if !useMock {
		llmRecord, err := getRedisLLMDebugRecord(execution.RequestID)
		if err == nil && llmRecord != nil {
			unified.LLMInteractions = llmRecord.Interactions
			unified.HasLLMData = len(llmRecord.Interactions) > 0

			// Build summary
			if len(llmRecord.Interactions) > 0 {
				summary := &LLMDebugSummary{
					TotalCalls:        len(llmRecord.Interactions),
					ProviderBreakdown: make(map[string]int),
				}
				for _, interaction := range llmRecord.Interactions {
					summary.TotalTokensIn += interaction.PromptTokens
					summary.TotalTokensOut += interaction.CompletionTokens
					summary.TotalDurationMs += interaction.DurationMs
					provider := interaction.Provider
					if provider == "" {
						provider = "unknown"
					}
					summary.ProviderBreakdown[provider]++
				}
				unified.LLMDebugSummary = summary
			}
		} else if err != nil && !strings.Contains(err.Error(), "not found") {
			log.Printf("Warning: failed to fetch LLM debug data for %s: %v", execution.RequestID, err)
		}

		// Fetch HITL checkpoints by request ID
		checkpoints, err := getHITLCheckpointsByRequestID(execution.RequestID)
		if err == nil && len(checkpoints) > 0 {
			unified.HITLCheckpoints = checkpoints
			unified.HasHITLData = true
		} else if err != nil {
			log.Printf("Warning: failed to fetch HITL checkpoints for %s: %v", execution.RequestID, err)
		}

		// Also check by original_request_id if different (for resumed HITL conversations)
		if execution.OriginalRequestID != "" && execution.OriginalRequestID != execution.RequestID {
			moreCheckpoints, err := getHITLCheckpointsByRequestID(execution.OriginalRequestID)
			if err == nil && len(moreCheckpoints) > 0 {
				unified.HITLCheckpoints = append(unified.HITLCheckpoints, moreCheckpoints...)
				unified.HasHITLData = true
			}
		}
	}

	return unified
}

// getHITLCheckpointsByRequestID finds all HITL checkpoints for a given request ID
func getHITLCheckpointsByRequestID(requestID string) ([]HITLCheckpoint, error) {
	client, err := getHITLClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Search for checkpoints across all prefixes that match this request ID
	// Pattern: gomind:hitl:*:checkpoint:*
	patterns := []string{
		fmt.Sprintf("%s:checkpoint:*", hitlKeyPrefix),   // Base prefix
		fmt.Sprintf("%s:*:checkpoint:*", hitlKeyPrefix), // Agent-specific prefix
	}

	var checkpoints []HITLCheckpoint
	seenIDs := make(map[string]bool)

	for _, pattern := range patterns {
		keys, err := client.Keys(ctx, pattern).Result()
		if err != nil {
			log.Printf("Warning: failed to scan HITL keys with pattern %s: %v", pattern, err)
			continue
		}

		for _, key := range keys {
			data, err := client.Get(ctx, key).Bytes()
			if err != nil {
				continue
			}

			var checkpoint HITLCheckpoint
			if err := json.Unmarshal(data, &checkpoint); err != nil {
				continue
			}

			// Check if this checkpoint belongs to the requested request ID
			if checkpoint.RequestID == requestID && !seenIDs[checkpoint.CheckpointID] {
				seenIDs[checkpoint.CheckpointID] = true
				// Extract agent name from key if not set
				if checkpoint.AgentName == "" {
					checkpoint.AgentName = extractAgentNameFromKey(key)
				}
				checkpoints = append(checkpoints, checkpoint)
			}
		}
	}

	// Sort by created_at (newest first)
	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].CreatedAt.After(checkpoints[j].CreatedAt)
	})

	return checkpoints, nil
}

// extractAgentNameFromKey extracts the agent name from a HITL checkpoint key
// Key format: "gomind:hitl:agent-name:checkpoint:cp-xxx" or "gomind:hitl:checkpoint:cp-xxx"
func extractAgentNameFromKey(key string) string {
	parts := strings.Split(key, ":")
	// gomind:hitl:agent-name:checkpoint:cp-xxx -> agent-name
	// gomind:hitl:checkpoint:cp-xxx -> ""
	if len(parts) >= 5 && parts[3] == "checkpoint" {
		return parts[2]
	}
	return ""
}

// getRedisExecutionSummaries fetches recent execution summaries from Redis
func getRedisExecutionSummaries(limit int) ([]ExecutionSummary, error) {
	client, err := getExecutionDebugClient() // Uses Redis DB 8 for Execution Debug
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get recent request IDs from sorted set (newest first)
	requestIDs, err := client.ZRevRangeByScore(ctx, executionIndexKey, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    "+inf",
		Offset: 0,
		Count:  int64(limit),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}

	summaries := make([]ExecutionSummary, 0, len(requestIDs))
	for _, requestID := range requestIDs {
		execution, err := getRedisExecution(requestID)
		if err != nil {
			log.Printf("Warning: skipping execution %s: %v", requestID, err)
			continue
		}

		summary := ExecutionSummary{
			RequestID:         execution.RequestID,
			OriginalRequestID: execution.OriginalRequestID,
			TraceID:           execution.TraceID,
			AgentName:         execution.AgentName,
			OriginalRequest:   execution.OriginalRequest,
			Interrupted:       execution.Interrupted,
			CreatedAt:         execution.CreatedAt,
		}

		if execution.Result != nil {
			summary.Success = execution.Result.Success
			summary.TotalDurationMs = execution.Result.TotalDuration / 1_000_000 // ns to ms
			summary.StepCount = len(execution.Result.Steps)
			for _, step := range execution.Result.Steps {
				if !step.Success {
					summary.FailedSteps++
				}
			}
		}

		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// getRedisExecution fetches a single execution from Redis
func getRedisExecution(requestID string) (*StoredExecution, error) {
	client, err := getExecutionDebugClient() // Uses Redis DB 8 for Execution Debug
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := executionKeyPrefix + requestID
	data, err := client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("execution not found: %s", requestID)
	}
	if err != nil {
		return nil, fmt.Errorf("redis get failed: %w", err)
	}

	return deserializeExecution(data)
}

// deserializeExecution deserializes an execution with optional gzip decompression
// Format: first byte is compression flag (0=raw, 1=gzip), rest is JSON
func deserializeExecution(data []byte) (*StoredExecution, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	var jsonData []byte

	// Check compression flag (first byte)
	if data[0] == 1 {
		// Gzip compressed
		reader, err := gzip.NewReader(bytes.NewReader(data[1:]))
		if err != nil {
			return nil, fmt.Errorf("gzip reader failed: %w", err)
		}
		defer reader.Close()

		var buf bytes.Buffer
		if _, err := buf.ReadFrom(reader); err != nil {
			return nil, fmt.Errorf("gzip decompress failed: %w", err)
		}
		jsonData = buf.Bytes()
	} else if data[0] == 0 {
		// Raw JSON (skip flag byte)
		jsonData = data[1:]
	} else {
		// Legacy format (no flag byte, raw JSON)
		jsonData = data
	}

	var execution StoredExecution
	if err := json.Unmarshal(jsonData, &execution); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w", err)
	}

	return &execution, nil
}

// getMockExecutionSummaries returns mock execution summaries for development
func getMockExecutionSummaries() []ExecutionSummary {
	now := time.Now()
	return []ExecutionSummary{
		{
			RequestID:         "orch-1705312800123456789",
			OriginalRequestID: "orch-1705312800123456789",
			TraceID:           "abc123def456",
			OriginalRequest:   "What's the weather in Tokyo and convert to Celsius?",
			Success:           true,
			StepCount:         2,
			FailedSteps:       0,
			TotalDurationMs:   2450,
			CreatedAt:         now.Add(-5 * time.Minute),
		},
		{
			RequestID:         "orch-1705312700987654321",
			OriginalRequestID: "orch-1705312700987654321",
			TraceID:           "xyz789abc012",
			OriginalRequest:   "Book a flight from NYC to London and check the weather",
			Success:           false,
			StepCount:         3,
			FailedSteps:       1,
			TotalDurationMs:   4230,
			CreatedAt:         now.Add(-15 * time.Minute),
		},
		{
			RequestID:         "orch-1705312600111222333",
			OriginalRequestID: "orch-1705312600111222333",
			TraceID:           "mno345pqr678",
			OriginalRequest:   "Get stock prices for AAPL, GOOGL, and MSFT",
			Success:           true,
			StepCount:         3,
			FailedSteps:       0,
			TotalDurationMs:   1850,
			CreatedAt:         now.Add(-30 * time.Minute),
		},
	}
}

// getMockExecution returns a mock execution for development
func getMockExecution(requestID string) *StoredExecution {
	now := time.Now()

	switch requestID {
	case "orch-1705312800123456789":
		startTime1 := now.Add(-5*time.Minute - 2450*time.Millisecond)
		endTime1 := startTime1.Add(1200 * time.Millisecond)
		startTime2 := endTime1.Add(50 * time.Millisecond)
		endTime2 := startTime2.Add(1200 * time.Millisecond)
		planCreatedAt := startTime1.Add(-100 * time.Millisecond)

		return &StoredExecution{
			RequestID:         requestID,
			OriginalRequestID: requestID,
			TraceID:           "abc123def456",
			OriginalRequest:   "What's the weather in Tokyo and convert to Celsius?",
			Plan: &RoutingPlan{
				PlanID:          requestID,
				OriginalRequest: "What's the weather in Tokyo and convert to Celsius?",
				Mode:            "autonomous",
				Steps: []RoutingStep{
					{
						StepID:      "step-1",
						AgentName:   "weather-tool",
						Capability:  "get_weather",
						Instruction: "Get current weather for Tokyo",
						DependsOn:   []string{},
					},
					{
						StepID:      "step-2",
						AgentName:   "unit-converter",
						Capability:  "convert_temperature",
						Instruction: "Convert temperature from Fahrenheit to Celsius",
						DependsOn:   []string{"step-1"},
					},
				},
				CreatedAt: &planCreatedAt,
			},
			Result: &ExecutionResult{
				PlanID:        requestID,
				Success:       true,
				TotalDuration: 2450000000, // 2.45 seconds in nanoseconds
				Steps: []StepResult{
					{
						StepID:     "step-1",
						AgentName:  "weather-tool",
						Capability: "get_weather",
						Success:    true,
						Response:   map[string]interface{}{"temp": 72, "unit": "F", "condition": "sunny"},
						DurationMs: 1200,
						StartTime:  &startTime1,
						EndTime:    &endTime1,
						Attempts:   1,
					},
					{
						StepID:     "step-2",
						AgentName:  "unit-converter",
						Capability: "convert_temperature",
						Success:    true,
						Response:   map[string]interface{}{"temp": 22.2, "unit": "C"},
						DurationMs: 1200,
						StartTime:  &startTime2,
						EndTime:    &endTime2,
						Attempts:   1,
					},
				},
			},
			CreatedAt: now.Add(-5 * time.Minute),
		}

	case "orch-1705312700987654321":
		startTime1 := now.Add(-15*time.Minute - 4230*time.Millisecond)
		endTime1 := startTime1.Add(1500 * time.Millisecond)
		startTime2 := endTime1.Add(30 * time.Millisecond)
		endTime2 := startTime2.Add(2200 * time.Millisecond)
		startTime3 := startTime1 // Parallel with step 1
		endTime3 := startTime3.Add(500 * time.Millisecond)
		planCreatedAt := startTime1.Add(-100 * time.Millisecond)

		return &StoredExecution{
			RequestID:         requestID,
			OriginalRequestID: requestID,
			TraceID:           "xyz789abc012",
			OriginalRequest:   "Book a flight from NYC to London and check the weather",
			Plan: &RoutingPlan{
				PlanID:          requestID,
				OriginalRequest: "Book a flight from NYC to London and check the weather",
				Mode:            "autonomous",
				Steps: []RoutingStep{
					{
						StepID:      "step-1",
						AgentName:   "flight-booking",
						Capability:  "search_flights",
						Instruction: "Search for flights from NYC to London",
						DependsOn:   []string{},
					},
					{
						StepID:      "step-2",
						AgentName:   "flight-booking",
						Capability:  "book_flight",
						Instruction: "Book the selected flight",
						DependsOn:   []string{"step-1"},
					},
					{
						StepID:      "step-3",
						AgentName:   "weather-tool",
						Capability:  "get_weather",
						Instruction: "Get weather forecast for London",
						DependsOn:   []string{},
					},
				},
				CreatedAt: &planCreatedAt,
			},
			Result: &ExecutionResult{
				PlanID:        requestID,
				Success:       false,
				TotalDuration: 4230000000,
				Steps: []StepResult{
					{
						StepID:     "step-1",
						AgentName:  "flight-booking",
						Capability: "search_flights",
						Success:    true,
						Response:   map[string]interface{}{"flights": []string{"BA178", "AA101"}, "prices": []int{450, 520}},
						DurationMs: 1500,
						StartTime:  &startTime1,
						EndTime:    &endTime1,
						Attempts:   1,
					},
					{
						StepID:     "step-2",
						AgentName:  "flight-booking",
						Capability: "book_flight",
						Success:    false,
						Error:      "Payment gateway timeout after 3 retries",
						DurationMs: 2200,
						StartTime:  &startTime2,
						EndTime:    &endTime2,
						Attempts:   3,
					},
					{
						StepID:     "step-3",
						AgentName:  "weather-tool",
						Capability: "get_weather",
						Success:    true,
						Response:   map[string]interface{}{"temp": 12, "unit": "C", "condition": "cloudy"},
						DurationMs: 500,
						StartTime:  &startTime3,
						EndTime:    &endTime3,
						Attempts:   1,
					},
				},
			},
			CreatedAt: now.Add(-15 * time.Minute),
		}

	default:
		return nil
	}
}

// getRedisHITLCheckpointSummaries fetches pending checkpoint summaries from Redis
// Supports multi-agent key prefixes (e.g., gomind:hitl:agent-name:pending)
func getRedisHITLCheckpointSummaries() ([]HITLCheckpointSummary, error) {
	client, err := getHITLClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find all pending indexes (supports multi-agent prefixes)
	// Pattern matches: gomind:hitl:pending AND gomind:hitl:*:pending
	pendingIndexes, err := client.Keys(ctx, "gomind:hitl:*pending").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to scan pending indexes: %w", err)
	}

	// Also check the base pending index
	baseIndexExists, _ := client.Exists(ctx, hitlPendingIndex).Result()
	if baseIndexExists > 0 {
		// Add if not already in the list
		found := false
		for _, idx := range pendingIndexes {
			if idx == hitlPendingIndex {
				found = true
				break
			}
		}
		if !found {
			pendingIndexes = append(pendingIndexes, hitlPendingIndex)
		}
	}

	log.Printf("Found %d HITL pending indexes: %v", len(pendingIndexes), pendingIndexes)

	// Collect checkpoint IDs from all pending indexes, tracking their prefixes
	type checkpointRef struct {
		id     string
		prefix string // The key prefix used (e.g., "gomind:hitl:agent-with-human-approval")
	}
	var checkpointRefs []checkpointRef

	for _, indexKey := range pendingIndexes {
		// Extract prefix from index key (e.g., "gomind:hitl:agent-name:pending" -> "gomind:hitl:agent-name")
		prefix := strings.TrimSuffix(indexKey, ":pending")

		ids, err := client.SMembers(ctx, indexKey).Result()
		if err != nil {
			log.Printf("Warning: failed to read pending index %s: %v", indexKey, err)
			continue
		}

		for _, id := range ids {
			checkpointRefs = append(checkpointRefs, checkpointRef{id: id, prefix: prefix})
		}
	}

	log.Printf("Found %d pending checkpoint references", len(checkpointRefs))

	summaries := make([]HITLCheckpointSummary, 0, len(checkpointRefs))
	for _, ref := range checkpointRefs {
		checkpoint, err := getRedisHITLCheckpointWithPrefix(ref.id, ref.prefix)
		if err != nil {
			log.Printf("Warning: skipping checkpoint %s (prefix=%s): %v", ref.id, ref.prefix, err)
			continue // Skip missing checkpoints (TTL expired)
		}

		// Extract agent name from prefix
		// Format: "gomind:hitl" (base) or "gomind:hitl:agent-name" (multi-agent)
		agentName := extractAgentNameFromPrefix(ref.prefix)

		// Build summary from full checkpoint
		summary := HITLCheckpointSummary{
			CheckpointID:    checkpoint.CheckpointID,
			RequestID:       checkpoint.RequestID,
			InterruptPoint:  checkpoint.InterruptPoint,
			OriginalRequest: checkpoint.OriginalRequest,
			CreatedAt:       checkpoint.CreatedAt,
			ExpiresAt:       checkpoint.ExpiresAt,
			Status:          checkpoint.Status,
			AgentName:       agentName,
		}

		if checkpoint.Decision != nil {
			summary.Reason = checkpoint.Decision.Reason
			summary.Priority = checkpoint.Decision.Priority
			summary.Message = checkpoint.Decision.Message
		}

		if checkpoint.Plan != nil {
			summary.StepCount = len(checkpoint.Plan.Steps)
		}

		summary.CompletedCount = len(checkpoint.CompletedSteps)

		if checkpoint.CurrentStep != nil {
			summary.CurrentStep = checkpoint.CurrentStep.Capability
		}

		summaries = append(summaries, summary)
	}

	// Sort by created_at descending (newest first)
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAt.After(summaries[j].CreatedAt)
	})

	return summaries, nil
}

// extractAgentNameFromPrefix extracts the agent name from a Redis key prefix
// Examples:
//   - "gomind:hitl" -> "" (base prefix, no agent name)
//   - "gomind:hitl:agent-with-human-approval" -> "agent-with-human-approval"
//   - "gomind:hitl:payment-service" -> "payment-service"
func extractAgentNameFromPrefix(prefix string) string {
	// Base prefix is "gomind:hitl"
	basePrefix := hitlKeyPrefix // "gomind:hitl"

	if prefix == basePrefix {
		return "" // No agent name for base prefix
	}

	// Check if prefix starts with base prefix + ":"
	agentPrefix := basePrefix + ":"
	if strings.HasPrefix(prefix, agentPrefix) {
		return strings.TrimPrefix(prefix, agentPrefix)
	}

	return "" // Unknown format
}

// extractPrefixFromCheckpointKey extracts the prefix from a full checkpoint key
// Examples:
//   - "gomind:hitl:checkpoint:cp-xxx" -> "gomind:hitl"
//   - "gomind:hitl:agent-name:checkpoint:cp-xxx" -> "gomind:hitl:agent-name"
func extractPrefixFromCheckpointKey(key, checkpointID string) string {
	// Remove the ":checkpoint:cp-xxx" suffix to get the prefix
	suffix := ":checkpoint:" + checkpointID
	return strings.TrimSuffix(key, suffix)
}

// getRedisHITLCheckpointWithPrefix fetches a checkpoint from Redis using a specific prefix
func getRedisHITLCheckpointWithPrefix(checkpointID, prefix string) (*HITLCheckpoint, error) {
	client, err := getHITLClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("%s:checkpoint:%s", prefix, checkpointID)
	data, err := client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("checkpoint not found: %s (key=%s)", checkpointID, key)
	}
	if err != nil {
		return nil, fmt.Errorf("redis get failed: %w", err)
	}

	var checkpoint HITLCheckpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w", err)
	}

	// Set agent name from prefix (if not already set in the checkpoint data)
	if checkpoint.AgentName == "" {
		checkpoint.AgentName = extractAgentNameFromPrefix(prefix)
	}

	return &checkpoint, nil
}

// getRedisHITLCheckpoint fetches a single checkpoint from Redis
// Searches across multiple prefixes to support multi-agent deployments
func getRedisHITLCheckpoint(checkpointID string) (*HITLCheckpoint, error) {
	client, err := getHITLClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First try the base prefix
	key := fmt.Sprintf("%s:checkpoint:%s", hitlKeyPrefix, checkpointID)
	data, err := client.Get(ctx, key).Bytes()
	if err == nil {
		var checkpoint HITLCheckpoint
		if err := json.Unmarshal(data, &checkpoint); err != nil {
			return nil, fmt.Errorf("json unmarshal failed: %w", err)
		}
		// Base prefix has no agent name
		return &checkpoint, nil
	}

	// If not found, search for the checkpoint across all prefixes
	pattern := fmt.Sprintf("gomind:hitl:*:checkpoint:%s", checkpointID)
	keys, err := client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to search for checkpoint: %w", err)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("checkpoint not found: %s", checkpointID)
	}

	// Use the first matching key
	foundKey := keys[0]
	data, err = client.Get(ctx, foundKey).Bytes()
	if err != nil {
		return nil, fmt.Errorf("redis get failed for %s: %w", foundKey, err)
	}

	var checkpoint HITLCheckpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w", err)
	}

	// Extract agent name from the found key
	// Key format: "gomind:hitl:agent-name:checkpoint:cp-xxx"
	if checkpoint.AgentName == "" {
		prefix := extractPrefixFromCheckpointKey(foundKey, checkpointID)
		checkpoint.AgentName = extractAgentNameFromPrefix(prefix)
	}

	return &checkpoint, nil
}

// ============================================================================
// HITL Checkpoint Mock Data
// ============================================================================

// getMockHITLCheckpointSummaries returns mock summaries for development
func getMockHITLCheckpointSummaries() []HITLCheckpointSummary {
	now := time.Now()
	return []HITLCheckpointSummary{
		{
			CheckpointID:    "cp-abc123-plan",
			RequestID:       "req-travel-001",
			InterruptPoint:  "plan_generated",
			Reason:          "plan_approval",
			Priority:        "normal",
			Message:         "Execution plan requires approval before proceeding",
			OriginalRequest: "What's the weather in Tokyo and book me a flight there?",
			StepCount:       3,
			CompletedCount:  0,
			CurrentStep:     "",
			CreatedAt:       now.Add(-2 * time.Minute),
			ExpiresAt:       now.Add(5 * time.Minute),
			Status:          "pending",
			AgentName:       "travel-agent",
		},
		{
			CheckpointID:    "cp-def456-step",
			RequestID:       "req-stock-002",
			InterruptPoint:  "before_step",
			Reason:          "sensitive_operation",
			Priority:        "high",
			Message:         "About to execute sensitive operation: stock_trade.execute_trade",
			OriginalRequest: "Buy 100 shares of AAPL",
			StepCount:       2,
			CompletedCount:  1,
			CurrentStep:     "stock_trade.execute_trade",
			CreatedAt:       now.Add(-5 * time.Minute),
			ExpiresAt:       now.Add(10 * time.Minute),
			Status:          "pending",
			AgentName:       "trading-bot",
		},
		{
			CheckpointID:    "cp-ghi789-error",
			RequestID:       "req-payment-003",
			InterruptPoint:  "on_error",
			Reason:          "escalation",
			Priority:        "critical",
			Message:         "Payment processing failed after 3 retries - human intervention required",
			OriginalRequest: "Process refund for order #12345",
			StepCount:       1,
			CompletedCount:  0,
			CurrentStep:     "payment.process_refund",
			CreatedAt:       now.Add(-10 * time.Minute),
			ExpiresAt:       now.Add(30 * time.Minute),
			Status:          "pending",
			AgentName:       "payment-service",
		},
	}
}

// getMockHITLCheckpoint returns a mock full checkpoint for development
func getMockHITLCheckpoint(checkpointID string) *HITLCheckpoint {
	now := time.Now()

	switch checkpointID {
	case "cp-abc123-plan":
		return &HITLCheckpoint{
			CheckpointID:   "cp-abc123-plan",
			RequestID:      "req-travel-001",
			InterruptPoint: "plan_generated",
			Decision: &InterruptDecision{
				ShouldInterrupt: true,
				Reason:          "plan_approval",
				Message:         "Execution plan requires approval before proceeding",
				Priority:        "normal",
				Timeout:         300000000000, // 5 minutes in nanoseconds
				DefaultAction:   "approve",
			},
			Plan: &RoutingPlan{
				RequestID: "req-travel-001",
				Steps: []RoutingStep{
					{
						StepID:         "step-1",
						Capability:     "weather-tool.get_weather",
						ServiceName:    "weather-tool",
						CapabilityName: "get_weather",
						Parameters:     map[string]interface{}{"location": "Tokyo, Japan"},
						DependsOn:      []string{},
						Description:    "Get current weather for Tokyo",
					},
					{
						StepID:         "step-2",
						Capability:     "flight-search.search_flights",
						ServiceName:    "flight-search",
						CapabilityName: "search_flights",
						Parameters:     map[string]interface{}{"destination": "Tokyo", "date": "2024-03-15"},
						DependsOn:      []string{},
						Description:    "Search for available flights to Tokyo",
					},
					{
						StepID:         "step-3",
						Capability:     "flight-booking.book_flight",
						ServiceName:    "flight-booking",
						CapabilityName: "book_flight",
						Parameters:     map[string]interface{}{"flight_id": "{{step-2.flights[0].id}}"},
						DependsOn:      []string{"step-2"},
						Description:    "Book the selected flight",
					},
				},
				SynthesisStrategy: "llm",
				Rationale:         "User wants weather info and flight booking. Steps 1 and 2 can run in parallel, step 3 depends on step 2 results.",
			},
			CompletedSteps:  []StepResult{},
			OriginalRequest: "What's the weather in Tokyo and book me a flight there?",
			UserContext: map[string]interface{}{
				"user_id":    "user-123",
				"session_id": "session-abc",
			},
			CreatedAt: now.Add(-2 * time.Minute),
			ExpiresAt: now.Add(5 * time.Minute),
			Status:    "pending",
			AgentName: "travel-agent",
		}

	case "cp-def456-step":
		return &HITLCheckpoint{
			CheckpointID:   "cp-def456-step",
			RequestID:      "req-stock-002",
			InterruptPoint: "before_step",
			Decision: &InterruptDecision{
				ShouldInterrupt: true,
				Reason:          "sensitive_operation",
				Message:         "About to execute sensitive operation: stock_trade.execute_trade",
				Priority:        "high",
				Metadata: map[string]interface{}{
					"capability":   "stock_trade.execute_trade",
					"risk_level":   "high",
					"amount_limit": 10000,
				},
			},
			Plan: &RoutingPlan{
				RequestID: "req-stock-002",
				Steps: []RoutingStep{
					{
						StepID:         "step-1",
						Capability:     "stock-market.get_quote",
						ServiceName:    "stock-market",
						CapabilityName: "get_quote",
						Parameters:     map[string]interface{}{"symbol": "AAPL"},
						DependsOn:      []string{},
						Description:    "Get current stock quote for AAPL",
					},
					{
						StepID:         "step-2",
						Capability:     "stock_trade.execute_trade",
						ServiceName:    "stock_trade",
						CapabilityName: "execute_trade",
						Parameters:     map[string]interface{}{"symbol": "AAPL", "quantity": 100, "action": "buy"},
						DependsOn:      []string{"step-1"},
						Description:    "Execute buy order for 100 shares of AAPL",
					},
				},
				SynthesisStrategy: "simple",
			},
			CompletedSteps: []StepResult{
				{
					StepID:     "step-1",
					Capability: "stock-market.get_quote",
					Success:    true,
					Response: map[string]interface{}{
						"symbol": "AAPL",
						"price":  178.50,
						"change": 2.35,
					},
					DurationMs: 234,
				},
			},
			CurrentStep: &RoutingStep{
				StepID:         "step-2",
				Capability:     "stock_trade.execute_trade",
				ServiceName:    "stock_trade",
				CapabilityName: "execute_trade",
				Parameters:     map[string]interface{}{"symbol": "AAPL", "quantity": 100, "action": "buy"},
				DependsOn:      []string{"step-1"},
				Description:    "Execute buy order for 100 shares of AAPL",
			},
			ResolvedParameters: map[string]interface{}{
				"symbol":   "AAPL",
				"quantity": 100,
				"action":   "buy",
				"price":    178.50,
				"total":    17850.00,
			},
			OriginalRequest: "Buy 100 shares of AAPL",
			UserContext: map[string]interface{}{
				"user_id":      "user-456",
				"account_type": "premium",
			},
			CreatedAt: now.Add(-5 * time.Minute),
			ExpiresAt: now.Add(10 * time.Minute),
			Status:    "pending",
			AgentName: "trading-bot",
		}

	case "cp-ghi789-error":
		return &HITLCheckpoint{
			CheckpointID:   "cp-ghi789-error",
			RequestID:      "req-payment-003",
			InterruptPoint: "on_error",
			Decision: &InterruptDecision{
				ShouldInterrupt: true,
				Reason:          "escalation",
				Message:         "Payment processing failed after 3 retries - human intervention required",
				Priority:        "critical",
				Metadata: map[string]interface{}{
					"retry_count":   3,
					"last_error":    "Payment gateway timeout",
					"order_id":      "#12345",
					"refund_amount": 99.99,
				},
			},
			Plan: &RoutingPlan{
				RequestID: "req-payment-003",
				Steps: []RoutingStep{
					{
						StepID:         "step-1",
						Capability:     "payment.process_refund",
						ServiceName:    "payment",
						CapabilityName: "process_refund",
						Parameters:     map[string]interface{}{"order_id": "#12345", "amount": 99.99},
						DependsOn:      []string{},
						Description:    "Process refund for the order",
					},
				},
				SynthesisStrategy: "simple",
			},
			CompletedSteps: []StepResult{},
			CurrentStep: &RoutingStep{
				StepID:         "step-1",
				Capability:     "payment.process_refund",
				ServiceName:    "payment",
				CapabilityName: "process_refund",
				Parameters:     map[string]interface{}{"order_id": "#12345", "amount": 99.99},
				DependsOn:      []string{},
				Description:    "Process refund for the order",
			},
			CurrentStepResult: &StepResult{
				StepID:     "step-1",
				Capability: "payment.process_refund",
				Success:    false,
				Error:      "Payment gateway timeout after 30s - gateway returned 504",
				DurationMs: 30234,
			},
			OriginalRequest: "Process refund for order #12345",
			UserContext: map[string]interface{}{
				"user_id":  "user-789",
				"order_id": "#12345",
				"customer": "John Doe",
				"email":    "john@example.com",
			},
			CreatedAt: now.Add(-10 * time.Minute),
			ExpiresAt: now.Add(30 * time.Minute),
			Status:    "pending",
			AgentName: "payment-service",
		}

	default:
		return nil
	}
}

// getMockLLMDebugRecord returns a mock full record for development
func getMockLLMDebugRecord(requestID string) *orchestration.LLMDebugRecord {
	now := time.Now()

	// Return different mock data based on request ID
	switch requestID {
	case "req-abc123":
		return &orchestration.LLMDebugRecord{
			RequestID:         "req-abc123",
			OriginalRequestID: "req-abc123", // Same as RequestID for initial request
			TraceID:           "369fecb4e3156c34e0950c61f1f99d62",
			CreatedAt:         now.Add(-5 * time.Minute),
			UpdatedAt:         now.Add(-4 * time.Minute),
			Interactions: []orchestration.LLMInteraction{
				{
					Type:             "plan_generation",
					Timestamp:        now.Add(-5 * time.Minute),
					DurationMs:       1247,
					Prompt:           "You are an intelligent orchestrator. Given the user request and available capabilities, create an execution plan.\n\nUser Request: \"What's the weather in Tokyo and convert 100 USD to JPY?\"\n\nAvailable Capabilities:\n- weather-tool.get-weather: Get current weather for a location\n- currency-tool.convert: Convert between currencies\n\nCreate a routing plan with the necessary steps.",
					SystemPrompt:     "You must respond with valid JSON only. Do not include any explanation.",
					Temperature:      0.3,
					MaxTokens:        2000,
					Model:            "gpt-4o-mini",
					Provider:         "openai",
					Response:         "{\"routing_plan\":{\"steps\":[{\"id\":\"step1\",\"capability\":\"weather-tool.get-weather\",\"parameters\":{\"location\":\"Tokyo\"}},{\"id\":\"step2\",\"capability\":\"currency-tool.convert\",\"parameters\":{\"from\":\"USD\",\"to\":\"JPY\",\"amount\":100}}]}}",
					PromptTokens:     247,
					CompletionTokens: 156,
					TotalTokens:      403,
					Success:          true,
					Attempt:          1,
				},
				{
					Type:             "micro_resolution",
					Timestamp:        now.Add(-4*time.Minute - 30*time.Second),
					DurationMs:       523,
					Prompt:           "Extract parameters for the \"get-weather\" function.\n\nAvailable data from previous step:\n{\"location\": \"Tokyo\"}\n\nReturn the extracted parameter values.",
					Temperature:      0.0,
					MaxTokens:        500,
					Model:            "gpt-4o-mini",
					Provider:         "openai",
					Response:         "{\"location\": \"Tokyo\", \"units\": \"metric\"}",
					PromptTokens:     89,
					CompletionTokens: 24,
					TotalTokens:      113,
					Success:          true,
					Attempt:          1,
				},
				{
					Type:             "synthesis",
					Timestamp:        now.Add(-4 * time.Minute),
					DurationMs:       1892,
					Prompt:           "User Request: What's the weather in Tokyo and convert 100 USD to JPY?\n\nAgent Responses:\n\nAgent: weather-tool\nTask: Get weather for Tokyo\nResponse:\n{\n  \"location\": \"Tokyo\",\n  \"temperature\": 22,\n  \"conditions\": \"Partly cloudy\",\n  \"humidity\": 65\n}\n\nAgent: currency-tool\nTask: Convert 100 USD to JPY\nResponse:\n{\n  \"from\": \"USD\",\n  \"to\": \"JPY\",\n  \"amount\": 100,\n  \"result\": 15234.50,\n  \"rate\": 152.345\n}\n\nSynthesize these responses into a helpful answer.",
					SystemPrompt:     "You are an AI that synthesizes multiple agent responses into coherent, helpful answers.",
					Temperature:      0.5,
					MaxTokens:        1500,
					Model:            "gpt-4o-mini",
					Provider:         "openai",
					Response:         "Based on the information gathered:\n\n**Weather in Tokyo:**\nThe current weather in Tokyo is partly cloudy with a temperature of 22°C (72°F). The humidity is at 65%.\n\n**Currency Conversion:**\n100 USD equals approximately 15,234.50 JPY at the current exchange rate of 152.345 JPY per USD.\n\nIs there anything else you'd like to know about Tokyo or currency conversions?",
					PromptTokens:     423,
					CompletionTokens: 108,
					TotalTokens:      531,
					Success:          true,
					Attempt:          1,
				},
			},
			Metadata: map[string]string{
				"user_id":    "user-123",
				"session_id": "session-abc",
			},
		}

	case "req-def456":
		return &orchestration.LLMDebugRecord{
			RequestID:         "req-def456",
			OriginalRequestID: "req-abc123", // Part of same HITL conversation (resume)
			TraceID:           "7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d",
			CreatedAt:         now.Add(-15 * time.Minute),
			UpdatedAt:         now.Add(-14 * time.Minute),
			Interactions: []orchestration.LLMInteraction{
				{
					Type:             "plan_generation",
					Timestamp:        now.Add(-15 * time.Minute),
					DurationMs:       987,
					Prompt:           "You are an intelligent orchestrator. Given the user request and available capabilities, create an execution plan.\n\nUser Request: \"Get the latest news about AI\"\n\nAvailable Capabilities:\n- news-tool.search-news: Search for news articles\n- news-tool.get-headlines: Get top headlines\n\nCreate a routing plan.",
					SystemPrompt:     "You must respond with valid JSON only.",
					Temperature:      0.3,
					MaxTokens:        2000,
					Model:            "gpt-4o-mini",
					Provider:         "openai",
					Response:         "{\"routing_plan\":{\"steps\":[{\"id\":\"step1\",\"capability\":\"news-tool.search-news\",\"parameters\":{\"query\":\"AI artificial intelligence\",\"limit\":5}}]}}",
					PromptTokens:     198,
					CompletionTokens: 87,
					TotalTokens:      285,
					Success:          true,
					Attempt:          1,
				},
				{
					Type:             "synthesis",
					Timestamp:        now.Add(-14 * time.Minute),
					DurationMs:       1238,
					Prompt:           "User Request: Get the latest news about AI\n\nAgent Responses:\n\nAgent: news-tool\nTask: Search news about AI\nResponse:\n{\n  \"articles\": [\n    {\"title\": \"OpenAI Announces GPT-5\", \"source\": \"TechCrunch\"},\n    {\"title\": \"AI Regulation in EU\", \"source\": \"Reuters\"}\n  ]\n}\n\nSynthesize into a helpful response.",
					SystemPrompt:     "You are an AI that synthesizes multiple agent responses into coherent, helpful answers.",
					Temperature:      0.5,
					MaxTokens:        1500,
					Model:            "gpt-4o-mini",
					Provider:         "openai",
					Response:         "Here are the latest AI news headlines:\n\n1. **OpenAI Announces GPT-5** (TechCrunch)\n2. **AI Regulation in EU** (Reuters)\n\nWould you like more details on any of these articles?",
					PromptTokens:     312,
					CompletionTokens: 56,
					TotalTokens:      368,
					Success:          true,
					Attempt:          1,
				},
			},
		}

	case "req-ghi789":
		return &orchestration.LLMDebugRecord{
			RequestID:         "req-ghi789",
			OriginalRequestID: "req-ghi789", // Different conversation
			TraceID:           "1234567890abcdef1234567890abcdef",
			CreatedAt:         now.Add(-1 * time.Hour),
			UpdatedAt:         now.Add(-59 * time.Minute),
			Interactions: []orchestration.LLMInteraction{
				{
					Type:             "plan_generation",
					Timestamp:        now.Add(-1 * time.Hour),
					DurationMs:       1523,
					Prompt:           "You are an intelligent orchestrator. Given the user request and available capabilities, create an execution plan.\n\nUser Request: \"Book a flight from NYC to London\"\n\nAvailable Capabilities:\n- travel-agent.find-flights: Search for flights\n- travel-agent.book-hotel: Find and book hotels\n\nCreate a routing plan.",
					SystemPrompt:     "You must respond with valid JSON only.",
					Temperature:      0.3,
					MaxTokens:        2000,
					Model:            "gpt-4o-mini",
					Provider:         "openai",
					Response:         "{\"routing_plan\":{\"steps\":[{\"id\":\"step1\",\"capability\":\"travel-agent.find-flights\",\"parameters\":{\"from\":\"NYC\",\"to\":\"London\"}}]}}",
					PromptTokens:     234,
					CompletionTokens: 98,
					TotalTokens:      332,
					Success:          true,
					Attempt:          1,
				},
				{
					Type:             "micro_resolution",
					Timestamp:        now.Add(-59*time.Minute - 45*time.Second),
					DurationMs:       412,
					Prompt:           "Extract parameters for the \"find-flights\" function.\n\nAvailable data:\n{\"from\": \"NYC\", \"to\": \"London\"}\n\nRequired parameters: departure_airport (IATA code), arrival_airport (IATA code), date\n\nReturn extracted values.",
					Temperature:      0.0,
					MaxTokens:        500,
					Model:            "gpt-4o-mini",
					Provider:         "openai",
					Response:         "{\"departure_airport\": \"JFK\", \"arrival_airport\": \"LHR\"}",
					PromptTokens:     156,
					CompletionTokens: 32,
					TotalTokens:      188,
					Success:          true,
					Attempt:          1,
				},
				{
					Type:         "correction",
					Timestamp:    now.Add(-59*time.Minute - 30*time.Second),
					DurationMs:   623,
					Prompt:       "The API returned an error: \"Missing required parameter: date\"\n\nOriginal parameters: {\"departure_airport\": \"JFK\", \"arrival_airport\": \"LHR\"}\n\nPlease provide corrected parameters including the missing 'date' field.",
					Temperature:  0.2,
					MaxTokens:    500,
					Model:        "gpt-4o-mini",
					Provider:     "openai",
					Response:     "",
					PromptTokens: 178,
					TotalTokens:  178,
					Success:      false,
					Error:        "LLM API timeout after 30s",
					Attempt:      1,
				},
				{
					Type:             "correction",
					Timestamp:        now.Add(-59 * time.Minute),
					DurationMs:       534,
					Prompt:           "The API returned an error: \"Missing required parameter: date\"\n\nOriginal parameters: {\"departure_airport\": \"JFK\", \"arrival_airport\": \"LHR\"}\n\nPlease provide corrected parameters including the missing 'date' field.",
					Temperature:      0.2,
					MaxTokens:        500,
					Model:            "gpt-4o-mini",
					Provider:         "openai",
					Response:         "{\"departure_airport\": \"JFK\", \"arrival_airport\": \"LHR\", \"date\": \"2024-02-15\"}",
					PromptTokens:     178,
					CompletionTokens: 45,
					TotalTokens:      223,
					Success:          true,
					Attempt:          2,
				},
			},
			Metadata: map[string]string{
				"error_count": "1",
				"retried":     "true",
			},
		}

	default:
		return nil
	}
}
