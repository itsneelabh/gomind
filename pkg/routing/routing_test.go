package routing

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAIClient for testing
type MockAIClient struct {
	mock.Mock
}

func (m *MockAIClient) GenerateResponse(ctx context.Context, prompt string, options *ai.GenerationOptions) (*ai.AIResponse, error) {
	args := m.Called(ctx, prompt, options)
	if resp := args.Get(0); resp != nil {
		return resp.(*ai.AIResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAIClient) StreamResponse(ctx context.Context, prompt string, options *ai.GenerationOptions) (<-chan ai.AIStreamChunk, error) {
	args := m.Called(ctx, prompt, options)
	if ch := args.Get(0); ch != nil {
		return ch.(<-chan ai.AIStreamChunk), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAIClient) GetProviderInfo() ai.ProviderInfo {
	args := m.Called()
	return args.Get(0).(ai.ProviderInfo)
}

// Test AutonomousRouter
func TestAutonomousRouter(t *testing.T) {
	t.Run("successful routing", func(t *testing.T) {
		mockAI := new(MockAIClient)
		router := NewAutonomousRouter(mockAI, WithModel("gpt-4"), WithTemperature(0.3))
		
		// Set agent catalog
		catalog := `Available agents:
1. calculator-agent (namespace: default) - Performs mathematical calculations
2. weather-agent (namespace: default) - Provides weather information
3. database-agent (namespace: backend) - Handles database queries`
		router.SetAgentCatalog(catalog)
		
		// Mock LLM response
		llmResponse := `{
			"analysis": "User wants to calculate something",
			"selected_agents": ["calculator-agent"],
			"confidence": 0.95,
			"steps": [
				{
					"order": 1,
					"agent_name": "calculator-agent",
					"namespace": "default",
					"instruction": "Calculate 25 * 4",
					"depends_on": [],
					"parallel": false,
					"required": true,
					"reason": "Calculator agent can perform arithmetic"
				}
			],
			"expected_outcome": "The result of 25 * 4"
		}`
		
		mockAI.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything).Return(&ai.AIResponse{
			Content: llmResponse,
			Model:   "gpt-4",
		}, nil)
		
		ctx := context.Background()
		plan, err := router.Route(ctx, "What is 25 times 4?", nil)
		
		assert.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Equal(t, ModeAutonomous, plan.Mode)
		assert.Equal(t, 0.95, plan.Confidence)
		assert.Len(t, plan.Steps, 1)
		assert.Equal(t, "calculator-agent", plan.Steps[0].AgentName)
		mockAI.AssertExpectations(t)
	})
	
	t.Run("no agent catalog", func(t *testing.T) {
		mockAI := new(MockAIClient)
		router := NewAutonomousRouter(mockAI)
		
		ctx := context.Background()
		_, err := router.Route(ctx, "Calculate something", nil)
		
		assert.Error(t, err)
		routingErr, ok := err.(*RoutingError)
		assert.True(t, ok)
		assert.Equal(t, ErrNoAgentsAvailable, routingErr.Code)
	})
	
	t.Run("caching", func(t *testing.T) {
		mockAI := new(MockAIClient)
		router := NewAutonomousRouter(mockAI, WithCacheTTL(1*time.Minute))
		router.SetAgentCatalog("agents: test")
		
		llmResponse := `{
			"analysis": "test",
			"selected_agents": ["test-agent"],
			"confidence": 0.9,
			"steps": [{
				"order": 1,
				"agent_name": "test-agent",
				"namespace": "default",
				"instruction": "test",
				"depends_on": [],
				"parallel": false,
				"required": true
			}],
			"expected_outcome": "test result"
		}`
		
		// First call - should hit LLM
		mockAI.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything).Return(&ai.AIResponse{
			Content: llmResponse,
		}, nil).Once()
		
		ctx := context.Background()
		plan1, err := router.Route(ctx, "test prompt", nil)
		assert.NoError(t, err)
		assert.NotNil(t, plan1)
		
		// Second call - should hit cache
		plan2, err := router.Route(ctx, "test prompt", nil)
		assert.NoError(t, err)
		assert.NotNil(t, plan2)
		assert.Equal(t, plan1.ID, plan2.ID)
		
		// Verify only one LLM call was made
		mockAI.AssertNumberOfCalls(t, "GenerateResponse", 1)
		
		stats := router.GetStats()
		assert.Equal(t, int64(1), stats.CacheHits)
		assert.Equal(t, int64(1), stats.CacheMisses)
	})
}

// Test WorkflowRouter
func TestWorkflowRouter(t *testing.T) {
	t.Run("load and match workflow", func(t *testing.T) {
		// Create temp directory for workflows
		tempDir, err := ioutil.TempDir("", "workflow-test")
		assert.NoError(t, err)
		defer os.RemoveAll(tempDir)
		
		// Create a test workflow
		workflowYAML := `name: data-analysis
description: Analyzes data and generates reports
triggers:
  keywords:
    - analyze
    - data
  patterns:
    - ".*analyze.*data.*"
    - ".*report.*statistics.*"
steps:
  - name: fetch-data
    agent: database-agent
    namespace: backend
    instruction: Fetch data from the database
    required: true
  - name: process-data
    agent: analytics-agent
    namespace: processing
    instruction: Process and analyze the fetched data
    depends_on:
      - fetch-data
    required: true
  - name: generate-report
    agent: report-agent
    namespace: frontend
    instruction: Generate a visual report
    depends_on:
      - process-data
    parallel: false
    required: true`
		
		workflowFile := filepath.Join(tempDir, "data-analysis.yaml")
		err = ioutil.WriteFile(workflowFile, []byte(workflowYAML), 0644)
		assert.NoError(t, err)
		
		// Create router
		router, err := NewWorkflowRouter(tempDir)
		assert.NoError(t, err)
		assert.NotNil(t, router)
		
		// Test matching
		ctx := context.Background()
		plan, err := router.Route(ctx, "I need to analyze the sales data", nil)
		
		assert.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Equal(t, ModeWorkflow, plan.Mode)
		assert.Len(t, plan.Steps, 3)
		
		// Verify step order and dependencies
		assert.Equal(t, "database-agent", plan.Steps[0].AgentName)
		assert.Empty(t, plan.Steps[0].DependsOn)
		
		assert.Equal(t, "analytics-agent", plan.Steps[1].AgentName)
		assert.Equal(t, []int{1}, plan.Steps[1].DependsOn)
		
		assert.Equal(t, "report-agent", plan.Steps[2].AgentName)
		assert.Equal(t, []int{2}, plan.Steps[2].DependsOn)
	})
	
	t.Run("no matching workflow", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "workflow-test")
		assert.NoError(t, err)
		defer os.RemoveAll(tempDir)
		
		router, err := NewWorkflowRouter(tempDir)
		assert.NoError(t, err)
		
		ctx := context.Background()
		_, err = router.Route(ctx, "This won't match anything", nil)
		
		assert.Error(t, err)
		routingErr, ok := err.(*RoutingError)
		assert.True(t, ok)
		assert.Equal(t, ErrWorkflowNotFound, routingErr.Code)
	})
	
	t.Run("template variables", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "workflow-test")
		assert.NoError(t, err)
		defer os.RemoveAll(tempDir)
		
		workflowYAML := `name: template-test
description: Test template processing
triggers:
  keywords:
    - test
variables:
  default_timeout: "30s"
  environment: "production"
steps:
  - name: step1
    agent: test-agent
    instruction: "Process in {{.environment}} environment with user {{.user_id}}"
    timeout: "{{.default_timeout}}"`
		
		workflowFile := filepath.Join(tempDir, "template.yaml")
		err = ioutil.WriteFile(workflowFile, []byte(workflowYAML), 0644)
		assert.NoError(t, err)
		
		router, err := NewWorkflowRouter(tempDir)
		assert.NoError(t, err)
		
		metadata := map[string]interface{}{
			"user_id": "user123",
		}
		
		ctx := context.Background()
		plan, err := router.Route(ctx, "test something", metadata)
		
		assert.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Contains(t, plan.Steps[0].Instruction, "production")
		assert.Contains(t, plan.Steps[0].Instruction, "user123")
	})
}

// Test HybridRouter
func TestHybridRouter(t *testing.T) {
	t.Run("workflow first with fallback", func(t *testing.T) {
		// Setup
		tempDir, err := ioutil.TempDir("", "hybrid-test")
		assert.NoError(t, err)
		defer os.RemoveAll(tempDir)
		
		mockAI := new(MockAIClient)
		router, err := NewHybridRouter(tempDir, mockAI, 
			WithPreferWorkflow(true),
			WithFallback(true))
		assert.NoError(t, err)
		
		// Set agent catalog for autonomous fallback
		router.SetAgentCatalog("test agents")
		
		// Mock autonomous response for fallback
		llmResponse := `{
			"analysis": "No workflow matched, using autonomous",
			"selected_agents": ["generic-agent"],
			"confidence": 0.8,
			"steps": [{
				"order": 1,
				"agent_name": "generic-agent",
				"namespace": "default",
				"instruction": "Handle request",
				"depends_on": [],
				"parallel": false,
				"required": true
			}],
			"expected_outcome": "Request handled"
		}`
		
		mockAI.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything).Return(&ai.AIResponse{
			Content: llmResponse,
		}, nil)
		
		// Route - should fallback to autonomous since no workflows exist
		ctx := context.Background()
		plan, err := router.Route(ctx, "Do something unique", nil)
		
		assert.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Equal(t, ModeHybrid, plan.Mode)
		mockAI.AssertExpectations(t)
	})
	
	t.Run("confidence threshold", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "hybrid-test")
		assert.NoError(t, err)
		defer os.RemoveAll(tempDir)
		
		// Create a simple workflow
		workflowYAML := `name: simple
triggers:
  keywords: ["simple"]
steps:
  - name: step1
    agent: simple-agent
    instruction: "Do simple task"`
		
		workflowFile := filepath.Join(tempDir, "simple.yaml")
		err = ioutil.WriteFile(workflowFile, []byte(workflowYAML), 0644)
		assert.NoError(t, err)
		
		mockAI := new(MockAIClient)
		router, err := NewHybridRouter(tempDir, mockAI,
			WithPreferWorkflow(false), // Prefer autonomous
			WithConfidenceThreshold(0.9),
			WithFallback(true))
		assert.NoError(t, err)
		router.SetAgentCatalog("test agents")
		
		// Mock low confidence autonomous response
		llmResponse := `{
			"analysis": "Low confidence routing",
			"selected_agents": ["uncertain-agent"],
			"confidence": 0.6,
			"steps": [{
				"order": 1,
				"agent_name": "uncertain-agent",
				"namespace": "default",
				"instruction": "Try this",
				"depends_on": [],
				"parallel": false,
				"required": true
			}],
			"expected_outcome": "Maybe works"
		}`
		
		mockAI.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything).Return(&ai.AIResponse{
			Content: llmResponse,
		}, nil)
		
		// Should fallback to workflow due to low confidence
		ctx := context.Background()
		plan, err := router.Route(ctx, "simple task", nil)
		
		assert.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Equal(t, ModeHybrid, plan.Mode)
		// Should use workflow instead due to low confidence
		assert.Equal(t, "simple-agent", plan.Steps[0].AgentName)
	})
}

// Test Cache implementations
func TestSimpleCache(t *testing.T) {
	t.Run("basic operations", func(t *testing.T) {
		cache := NewSimpleCache()
		defer cache.Stop()
		
		plan := &RoutingPlan{
			ID:             "test-123",
			OriginalPrompt: "test prompt",
			Confidence:     0.9,
		}
		
		// Test Set and Get
		cache.Set("test", plan, 1*time.Hour)
		
		retrieved, found := cache.Get("test")
		assert.True(t, found)
		assert.Equal(t, plan.ID, retrieved.ID)
		
		// Test miss
		_, found = cache.Get("nonexistent")
		assert.False(t, found)
		
		// Test Clear
		cache.Clear()
		_, found = cache.Get("test")
		assert.False(t, found)
		
		stats := cache.Stats()
		assert.Equal(t, int64(1), stats.Hits)
		assert.Equal(t, int64(2), stats.Misses)
	})
	
	t.Run("expiration", func(t *testing.T) {
		cache := NewSimpleCache()
		defer cache.Stop()
		
		plan := &RoutingPlan{ID: "expire-test"}
		
		// Set with short TTL
		cache.Set("expire", plan, 100*time.Millisecond)
		
		// Should exist immediately
		_, found := cache.Get("expire")
		assert.True(t, found)
		
		// Wait for expiration
		time.Sleep(150 * time.Millisecond)
		
		// Should be expired
		_, found = cache.Get("expire")
		assert.False(t, found)
	})
}

func TestLRUCache(t *testing.T) {
	t.Run("LRU eviction", func(t *testing.T) {
		cache := NewLRUCache(2) // Capacity of 2
		
		plan1 := &RoutingPlan{ID: "plan1"}
		plan2 := &RoutingPlan{ID: "plan2"}
		plan3 := &RoutingPlan{ID: "plan3"}
		
		// Add two items
		cache.Set("one", plan1, 1*time.Hour)
		cache.Set("two", plan2, 1*time.Hour)
		
		// Access first item (makes it recently used)
		_, found := cache.Get("one")
		assert.True(t, found)
		
		// Add third item - should evict "two" (least recently used)
		cache.Set("three", plan3, 1*time.Hour)
		
		// "one" should still exist
		_, found = cache.Get("one")
		assert.True(t, found)
		
		// "two" should be evicted
		_, found = cache.Get("two")
		assert.False(t, found)
		
		// "three" should exist
		_, found = cache.Get("three")
		assert.True(t, found)
		
		stats := cache.Stats()
		assert.Equal(t, int64(1), stats.Evictions)
	})
	
	t.Run("update existing", func(t *testing.T) {
		cache := NewLRUCache(5)
		
		plan1 := &RoutingPlan{ID: "v1", Confidence: 0.5}
		plan2 := &RoutingPlan{ID: "v2", Confidence: 0.9}
		
		// Set initial value
		cache.Set("key", plan1, 1*time.Hour)
		
		// Update with new value
		cache.Set("key", plan2, 1*time.Hour)
		
		// Should get updated value
		retrieved, found := cache.Get("key")
		assert.True(t, found)
		assert.Equal(t, "v2", retrieved.ID)
		assert.Equal(t, 0.9, retrieved.Confidence)
	})
}

// Test error types
func TestRoutingError(t *testing.T) {
	err := &RoutingError{
		Code:    ErrLLMFailure,
		Message: "LLM request failed",
		Details: "timeout after 30s",
	}
	
	assert.Contains(t, err.Error(), "LLM_FAILURE")
	assert.Contains(t, err.Error(), "LLM request failed")
	
	errWithStep := &RoutingError{
		Code:    ErrPlanGeneration,
		Message: "Failed to generate plan",
		Step:    "analyze-prompt",
	}
	
	assert.Contains(t, errWithStep.Error(), "at step analyze-prompt")
}

// Benchmark tests
func BenchmarkSimpleCache(b *testing.B) {
	cache := NewSimpleCache()
	defer cache.Stop()
	
	plan := &RoutingPlan{ID: "bench-plan"}
	cache.Set("bench", plan, 1*time.Hour)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("bench")
	}
}

func BenchmarkLRUCache(b *testing.B) {
	cache := NewLRUCache(100)
	
	plan := &RoutingPlan{ID: "bench-plan"}
	cache.Set("bench", plan, 1*time.Hour)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("bench")
	}
}

func BenchmarkAutonomousRouting(b *testing.B) {
	mockAI := new(MockAIClient)
	router := NewAutonomousRouter(mockAI, WithCache(nil)) // Disable cache for benchmark
	router.SetAgentCatalog("test agents")
	
	llmResponse := `{"analysis":"test","selected_agents":["test"],"confidence":0.9,"steps":[{"order":1,"agent_name":"test","namespace":"default","instruction":"test","depends_on":[],"parallel":false,"required":true}],"expected_outcome":"test"}`
	
	mockAI.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything).Return(&ai.AIResponse{
		Content: llmResponse,
	}, nil)
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.Route(ctx, fmt.Sprintf("test prompt %d", i), nil)
	}
}