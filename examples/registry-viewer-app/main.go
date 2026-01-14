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
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Version      string       `json:"version,omitempty"`
	Endpoint     string       `json:"endpoint,omitempty"`
	InputTypes   []string     `json:"input_types,omitempty"`
	OutputTypes  []string     `json:"output_types,omitempty"`
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
// LLM Debug Types (mirrors orchestration/llm_debug_store.go)
// ============================================================================

// LLMDebugRecord stores all LLM interactions for a single orchestration request
type LLMDebugRecord struct {
	RequestID    string            `json:"request_id"`
	TraceID      string            `json:"trace_id"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Interactions []LLMInteraction  `json:"interactions"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// LLMInteraction captures a single LLM call (request + response)
type LLMInteraction struct {
	Type             string    `json:"type"`
	Timestamp        time.Time `json:"timestamp"`
	DurationMs       int64     `json:"duration_ms"`
	Prompt           string    `json:"prompt"`
	SystemPrompt     string    `json:"system_prompt,omitempty"`
	Temperature      float64   `json:"temperature"`
	MaxTokens        int       `json:"max_tokens"`
	Model            string    `json:"model,omitempty"`
	Provider         string    `json:"provider,omitempty"`
	Response         string    `json:"response"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	Success          bool      `json:"success"`
	Error            string    `json:"error,omitempty"`
	Attempt          int       `json:"attempt"`
}

// LLMDebugRecordSummary is a lightweight version for listing
type LLMDebugRecordSummary struct {
	RequestID        string    `json:"request_id"`
	TraceID          string    `json:"trace_id"`
	CreatedAt        time.Time `json:"created_at"`
	InteractionCount int       `json:"interaction_count"`
	TotalTokens      int       `json:"total_tokens"`
	HasErrors        bool      `json:"has_errors"`
}

// LLMDebugListResponse is the API response for listing debug records
type LLMDebugListResponse struct {
	Records   []LLMDebugRecordSummary `json:"records"`
	Total     int                     `json:"total"`
	Timestamp time.Time               `json:"timestamp"`
}

// Redis database constants (mirrors core/redis_client.go)
const (
	RedisDBServiceDiscovery = 0
	RedisDBLLMDebug         = 7
)

// Redis key patterns for LLM Debug (mirrors orchestration/redis_llm_debug_store.go)
const (
	llmDebugKeyPrefix = "gomind:llm:debug:"
	llmDebugIndexKey  = "gomind:llm:debug:index"
)

var (
	useMock   bool
	redisURL  string
	namespace string
	port      int
)

func init() {
	flag.BoolVar(&useMock, "mock", true, "Use mock data instead of Redis")
	flag.StringVar(&redisURL, "redis-url", "redis://localhost:6379", "Redis URL")
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

	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/services", handleServices)
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/llm-debug", handleLLMDebugList)
	mux.HandleFunc("/api/llm-debug/", handleLLMDebugRecord)

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
		opt.DB = RedisDBLLMDebug // Use DB 7 for LLM Debug
		llmDebugClient = redis.NewClient(opt)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := llmDebugClient.Ping(ctx).Err(); err != nil {
			initErr = fmt.Errorf("redis connection failed (DB %d): %w", RedisDBLLMDebug, err)
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

	var records []LLMDebugRecordSummary
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

	var record *LLMDebugRecord
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
func getRedisLLMDebugSummaries(limit int) ([]LLMDebugRecordSummary, error) {
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

	summaries := make([]LLMDebugRecordSummary, 0, len(ids))
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

		summaries = append(summaries, LLMDebugRecordSummary{
			RequestID:        record.RequestID,
			TraceID:          record.TraceID,
			CreatedAt:        record.CreatedAt,
			InteractionCount: len(record.Interactions),
			TotalTokens:      totalTokens,
			HasErrors:        hasErrors,
		})
	}

	return summaries, nil
}

// getRedisLLMDebugRecord fetches a single debug record from Redis
func getRedisLLMDebugRecord(requestID string) (*LLMDebugRecord, error) {
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
func deserializeLLMDebugRecord(data []byte) (*LLMDebugRecord, error) {
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

	var record LLMDebugRecord
	if err := json.Unmarshal(jsonData, &record); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w", err)
	}
	return &record, nil
}

// ============================================================================
// LLM Debug Mock Data
// ============================================================================

// getMockLLMDebugSummaries returns mock summaries for development
func getMockLLMDebugSummaries() []LLMDebugRecordSummary {
	now := time.Now()
	return []LLMDebugRecordSummary{
		{
			RequestID:        "req-abc123",
			TraceID:          "369fecb4e3156c34e0950c61f1f99d62",
			CreatedAt:        now.Add(-5 * time.Minute),
			InteractionCount: 3,
			TotalTokens:      2847,
			HasErrors:        false,
		},
		{
			RequestID:        "req-def456",
			TraceID:          "7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d",
			CreatedAt:        now.Add(-15 * time.Minute),
			InteractionCount: 2,
			TotalTokens:      1523,
			HasErrors:        false,
		},
		{
			RequestID:        "req-ghi789",
			TraceID:          "1234567890abcdef1234567890abcdef",
			CreatedAt:        now.Add(-1 * time.Hour),
			InteractionCount: 4,
			TotalTokens:      4102,
			HasErrors:        true,
		},
	}
}

// getMockLLMDebugRecord returns a mock full record for development
func getMockLLMDebugRecord(requestID string) *LLMDebugRecord {
	now := time.Now()

	// Return different mock data based on request ID
	switch requestID {
	case "req-abc123":
		return &LLMDebugRecord{
			RequestID: "req-abc123",
			TraceID:   "369fecb4e3156c34e0950c61f1f99d62",
			CreatedAt: now.Add(-5 * time.Minute),
			UpdatedAt: now.Add(-4 * time.Minute),
			Interactions: []LLMInteraction{
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
		return &LLMDebugRecord{
			RequestID: "req-def456",
			TraceID:   "7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d",
			CreatedAt: now.Add(-15 * time.Minute),
			UpdatedAt: now.Add(-14 * time.Minute),
			Interactions: []LLMInteraction{
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
		return &LLMDebugRecord{
			RequestID: "req-ghi789",
			TraceID:   "1234567890abcdef1234567890abcdef",
			CreatedAt: now.Add(-1 * time.Hour),
			UpdatedAt: now.Add(-59 * time.Minute),
			Interactions: []LLMInteraction{
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
