package orchestration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/pkg/ai"
	"github.com/itsneelabh/gomind/pkg/communication"
	"github.com/itsneelabh/gomind/pkg/logger"
	"github.com/itsneelabh/gomind/pkg/routing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations
type MockRouter struct {
	mock.Mock
}

func (m *MockRouter) Route(ctx context.Context, prompt string, metadata map[string]interface{}) (*routing.RoutingPlan, error) {
	args := m.Called(ctx, prompt, metadata)
	if plan := args.Get(0); plan != nil {
		return plan.(*routing.RoutingPlan), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRouter) GetMode() routing.RouterMode {
	args := m.Called()
	return args.Get(0).(routing.RouterMode)
}

func (m *MockRouter) SetAgentCatalog(catalog string) {
	m.Called(catalog)
}

func (m *MockRouter) GetStats() routing.RouterStats {
	args := m.Called()
	return args.Get(0).(routing.RouterStats)
}

type MockCommunicator struct {
	mock.Mock
}

func (m *MockCommunicator) CallAgent(ctx context.Context, agentIdentifier string, instruction string) (string, error) {
	args := m.Called(ctx, agentIdentifier, instruction)
	return args.String(0), args.Error(1)
}

func (m *MockCommunicator) CallAgentWithTimeout(ctx context.Context, agentIdentifier string, instruction string, timeout time.Duration) (string, error) {
	args := m.Called(ctx, agentIdentifier, instruction, timeout)
	return args.String(0), args.Error(1)
}

func (m *MockCommunicator) GetAvailableAgents(ctx context.Context) ([]communication.AgentInfo, error) {
	args := m.Called(ctx)
	if agents := args.Get(0); agents != nil {
		return agents.([]communication.AgentInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockCommunicator) Ping(ctx context.Context, agentIdentifier string) error {
	args := m.Called(ctx, agentIdentifier)
	return args.Error(0)
}

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

type MockLogger struct{}

func (m *MockLogger) Debug(msg string, fields ...interface{}) {}
func (m *MockLogger) Info(msg string, fields ...interface{}) {}
func (m *MockLogger) Warn(msg string, fields ...interface{}) {}
func (m *MockLogger) Error(msg string, fields ...interface{}) {}
func (m *MockLogger) SetLevel(level string) {}
func (m *MockLogger) WithFields(fields map[string]interface{}) logger.Logger { return m }
func (m *MockLogger) WithField(key string, value interface{}) logger.Logger { return m }
func (m *MockLogger) With(fields ...logger.Field) logger.Logger { return m }

// Tests
func TestOrchestrator_ProcessRequest(t *testing.T) {
	t.Run("successful orchestration", func(t *testing.T) {
		// Setup mocks
		mockRouter := new(MockRouter)
		mockCommunicator := new(MockCommunicator)
		mockAI := new(MockAIClient)
		mockLogger := &MockLogger{}
		
		// Create routing plan
		plan := &routing.RoutingPlan{
			ID:         "test-plan-123",
			Mode:       routing.ModeAutonomous,
			Confidence: 0.95,
			Steps: []routing.RoutingStep{
				{
					Order:       1,
					StepID:      "step-1",
					AgentName:   "agent1",
					Namespace:   "default",
					Instruction: "Do task 1",
					Required:    true,
				},
				{
					Order:       2,
					StepID:      "step-2",
					AgentName:   "agent2",
					Namespace:   "default",
					Instruction: "Do task 2",
					Required:    true,
				},
			},
		}
		
		// Mock expectations
		mockRouter.On("Route", mock.Anything, "test request", mock.Anything).Return(plan, nil)
		mockCommunicator.On("CallAgent", mock.Anything, "agent1.default", "Do task 1").Return("Result 1", nil)
		mockCommunicator.On("CallAgent", mock.Anything, "agent2.default", "Do task 2").Return("Result 2", nil)
		mockAI.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything).Return(&ai.AIResponse{
			Content: "Synthesized response based on results",
		}, nil)
		
		// Create orchestrator
		config := DefaultConfig()
		config.CacheEnabled = false
		orchestrator := NewOrchestrator(mockRouter, mockCommunicator, mockAI, mockLogger, config)
		
		// Execute request
		ctx := context.Background()
		response, err := orchestrator.ProcessRequest(ctx, "test request", nil)
		
		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "test request", response.OriginalRequest)
		assert.Equal(t, "Synthesized response based on results", response.Response)
		assert.Equal(t, routing.ModeAutonomous, response.RoutingMode)
		assert.Len(t, response.AgentsInvolved, 2)
		assert.Equal(t, 0.95, response.Confidence)
		
		mockRouter.AssertExpectations(t)
		mockCommunicator.AssertExpectations(t)
		mockAI.AssertExpectations(t)
	})
	
	t.Run("routing failure", func(t *testing.T) {
		mockRouter := new(MockRouter)
		mockCommunicator := new(MockCommunicator)
		mockAI := new(MockAIClient)
		mockLogger := &MockLogger{}
		
		// Mock routing failure
		mockRouter.On("Route", mock.Anything, "test request", mock.Anything).Return(nil, errors.New("routing failed"))
		
		config := DefaultConfig()
		config.ExecutionOptions.CircuitBreaker = false
		orchestrator := NewOrchestrator(mockRouter, mockCommunicator, mockAI, mockLogger, config)
		
		ctx := context.Background()
		response, err := orchestrator.ProcessRequest(ctx, "test request", nil)
		
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "routing")
	})
	
	t.Run("partial execution failure with synthesis", func(t *testing.T) {
		mockRouter := new(MockRouter)
		mockCommunicator := new(MockCommunicator)
		mockAI := new(MockAIClient)
		mockLogger := &MockLogger{}
		
		plan := &routing.RoutingPlan{
			ID:         "test-plan-456",
			Mode:       routing.ModeWorkflow,
			Confidence: 0.9,
			Steps: []routing.RoutingStep{
				{
					Order:       1,
					StepID:      "step-1",
					AgentName:   "agent1",
					Namespace:   "default",
					Instruction: "Do task 1",
					Required:    false,
				},
				{
					Order:       2,
					StepID:      "step-2",
					AgentName:   "agent2",
					Namespace:   "default",
					Instruction: "Do task 2",
					Required:    false,
				},
			},
		}
		
		// Mock expectations - first agent fails, second succeeds
		mockRouter.On("Route", mock.Anything, "test request", mock.Anything).Return(plan, nil)
		mockCommunicator.On("CallAgent", mock.Anything, "agent1.default", "Do task 1").Return("", errors.New("agent1 failed"))
		mockCommunicator.On("CallAgent", mock.Anything, "agent2.default", "Do task 2").Return("Result 2", nil)
		mockAI.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything).Return(&ai.AIResponse{
			Content: "Partial results synthesized",
		}, nil)
		
		config := DefaultConfig()
		orchestrator := NewOrchestrator(mockRouter, mockCommunicator, mockAI, mockLogger, config)
		
		ctx := context.Background()
		response, err := orchestrator.ProcessRequest(ctx, "test request", nil)
		
		// Should succeed with partial results
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "Partial results synthesized", response.Response)
		assert.Len(t, response.Errors, 1)
	})
}

func TestPlanExecutor(t *testing.T) {
	t.Run("sequential execution", func(t *testing.T) {
		mockCommunicator := new(MockCommunicator)
		mockLogger := &MockLogger{}
		
		options := &ExecutionOptions{
			MaxConcurrency: 5,
			StepTimeout:    10 * time.Second,
		}
		
		executor := NewPlanExecutor(mockCommunicator, mockLogger, options)
		
		plan := &routing.RoutingPlan{
			ID: "test-plan",
			Steps: []routing.RoutingStep{
				{
					Order:       1,
					StepID:      "step-1",
					AgentName:   "agent1",
					Instruction: "Task 1",
					Parallel:    false,
				},
				{
					Order:       2,
					StepID:      "step-2",
					AgentName:   "agent2",
					Instruction: "Task 2",
					Parallel:    false,
					DependsOn:   []int{1},
				},
			},
		}
		
		mockCommunicator.On("CallAgent", mock.Anything, "agent1", "Task 1").Return("Result 1", nil)
		mockCommunicator.On("CallAgent", mock.Anything, "agent2", "Task 2").Return("Result 2", nil)
		
		ctx := context.Background()
		result, err := executor.Execute(ctx, plan)
		
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Len(t, result.Steps, 2)
		assert.Equal(t, "Result 1", result.Steps[0].Response)
		assert.Equal(t, "Result 2", result.Steps[1].Response)
		
		mockCommunicator.AssertExpectations(t)
	})
	
	t.Run("parallel execution", func(t *testing.T) {
		mockCommunicator := new(MockCommunicator)
		mockLogger := &MockLogger{}
		
		options := &ExecutionOptions{
			MaxConcurrency: 2,
			StepTimeout:    10 * time.Second,
		}
		
		executor := NewPlanExecutor(mockCommunicator, mockLogger, options)
		
		plan := &routing.RoutingPlan{
			ID: "test-plan",
			Steps: []routing.RoutingStep{
				{
					Order:       1,
					StepID:      "step-1",
					AgentName:   "agent1",
					Instruction: "Task 1",
					Parallel:    true,
				},
				{
					Order:       1,
					StepID:      "step-2",
					AgentName:   "agent2",
					Instruction: "Task 2",
					Parallel:    true,
				},
			},
		}
		
		mockCommunicator.On("CallAgent", mock.Anything, "agent1", "Task 1").Return("Result 1", nil)
		mockCommunicator.On("CallAgent", mock.Anything, "agent2", "Task 2").Return("Result 2", nil)
		
		ctx := context.Background()
		result, err := executor.Execute(ctx, plan)
		
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Len(t, result.Steps, 2)
		
		mockCommunicator.AssertExpectations(t)
	})
	
	t.Run("retry on failure", func(t *testing.T) {
		mockCommunicator := new(MockCommunicator)
		mockLogger := &MockLogger{}
		
		options := &ExecutionOptions{
			MaxConcurrency: 1,
			RetryAttempts:  2,
			RetryDelay:     10 * time.Millisecond,
		}
		
		executor := NewPlanExecutor(mockCommunicator, mockLogger, options)
		
		step := routing.RoutingStep{
			Order:       1,
			StepID:      "step-1",
			AgentName:   "agent1",
			Instruction: "Task 1",
		}
		
		// First call fails, second succeeds
		mockCommunicator.On("CallAgent", mock.Anything, "agent1", "Task 1").Return("", errors.New("temporary error")).Once()
		mockCommunicator.On("CallAgent", mock.Anything, "agent1", "Task 1").Return("Result after retry", nil).Once()
		
		ctx := context.Background()
		result, err := executor.ExecuteStep(ctx, step)
		
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Equal(t, "Result after retry", result.Response)
		assert.Equal(t, 2, result.Attempts)
		
		mockCommunicator.AssertExpectations(t)
	})
}

func TestResponseSynthesizer(t *testing.T) {
	t.Run("LLM synthesis", func(t *testing.T) {
		mockAI := new(MockAIClient)
		mockLogger := &MockLogger{}
		
		synthesizer := NewResponseSynthesizer(mockAI, mockLogger, StrategyLLM)
		
		results := &ExecutionResult{
			PlanID:  "test-plan",
			Success: true,
			Steps: []StepResult{
				{
					StepID:    "step-1",
					AgentName: "agent1",
					Response:  "Result 1",
					Success:   true,
				},
				{
					StepID:    "step-2",
					AgentName: "agent2",
					Response:  "Result 2",
					Success:   true,
				},
			},
		}
		
		mockAI.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything).Return(&ai.AIResponse{
			Content: "Synthesized: Result 1 and Result 2 combined",
		}, nil)
		
		ctx := context.Background()
		response, err := synthesizer.Synthesize(ctx, "test request", results)
		
		assert.NoError(t, err)
		assert.Equal(t, "Synthesized: Result 1 and Result 2 combined", response)
		
		mockAI.AssertExpectations(t)
	})
	
	t.Run("simple synthesis", func(t *testing.T) {
		mockLogger := &MockLogger{}
		
		synthesizer := NewResponseSynthesizer(nil, mockLogger, StrategySimple)
		
		results := &ExecutionResult{
			PlanID:  "test-plan",
			Success: true,
			Steps: []StepResult{
				{
					StepID:    "step-1",
					AgentName: "agent1",
					Namespace: "ns1",
					Response:  "Result 1",
					Success:   true,
				},
				{
					StepID:    "step-2",
					AgentName: "agent2",
					Namespace: "ns2",
					Response:  "Result 2",
					Success:   true,
				},
			},
		}
		
		ctx := context.Background()
		response, err := synthesizer.Synthesize(ctx, "test request", results)
		
		assert.NoError(t, err)
		assert.Contains(t, response, "agent1")
		assert.Contains(t, response, "Result 1")
		assert.Contains(t, response, "agent2")
		assert.Contains(t, response, "Result 2")
	})
	
	t.Run("template synthesis", func(t *testing.T) {
		mockLogger := &MockLogger{}
		
		synthesizer := NewResponseSynthesizer(nil, mockLogger, StrategyTemplate)
		
		results := &ExecutionResult{
			PlanID:  "test-plan",
			Success: true,
			Steps: []StepResult{
				{
					StepID:    "step-1",
					AgentName: "analyzer",
					Response:  "Analysis complete",
					Success:   true,
				},
			},
		}
		
		ctx := context.Background()
		response, err := synthesizer.Synthesize(ctx, "analyze data", results)
		
		assert.NoError(t, err)
		assert.Contains(t, response, "analyze data")
		assert.Contains(t, response, "Analysis complete")
	})
}

func TestCircuitBreaker(t *testing.T) {
	t.Run("circuit opens after threshold", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 100*time.Millisecond)
		
		// Should be closed initially
		assert.True(t, cb.CanExecute())
		
		// Record failures
		cb.RecordFailure()
		cb.RecordFailure()
		assert.True(t, cb.CanExecute()) // Still closed
		
		cb.RecordFailure() // Reaches threshold
		assert.False(t, cb.CanExecute()) // Circuit opens
	})
	
	t.Run("circuit recovers after timeout", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 50*time.Millisecond)
		
		// Open the circuit
		cb.RecordFailure()
		cb.RecordFailure()
		assert.False(t, cb.CanExecute())
		
		// Wait for recovery timeout
		time.Sleep(60 * time.Millisecond)
		assert.True(t, cb.CanExecute()) // Should allow retry
		
		// Success should reset circuit
		cb.RecordSuccess()
		assert.True(t, cb.CanExecute())
	})
}

// Benchmarks
func BenchmarkOrchestrator_ProcessRequest(b *testing.B) {
	mockRouter := new(MockRouter)
	mockCommunicator := new(MockCommunicator)
	mockAI := new(MockAIClient)
	mockLogger := &MockLogger{}
	
	plan := &routing.RoutingPlan{
		ID:         "bench-plan",
		Mode:       routing.ModeAutonomous,
		Confidence: 0.9,
		Steps: []routing.RoutingStep{
			{
				Order:       1,
				StepID:      "step-1",
				AgentName:   "agent1",
				Instruction: "Task",
			},
		},
	}
	
	mockRouter.On("Route", mock.Anything, mock.Anything, mock.Anything).Return(plan, nil)
	mockCommunicator.On("CallAgent", mock.Anything, mock.Anything, mock.Anything).Return("Result", nil)
	mockAI.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything).Return(&ai.AIResponse{
		Content: "Synthesized",
	}, nil)
	
	config := DefaultConfig()
	config.CacheEnabled = false
	orchestrator := NewOrchestrator(mockRouter, mockCommunicator, mockAI, mockLogger, config)
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		orchestrator.ProcessRequest(ctx, "benchmark request", nil)
	}
}

func BenchmarkExecutor_Parallel(b *testing.B) {
	mockCommunicator := new(MockCommunicator)
	mockLogger := &MockLogger{}
	
	options := &ExecutionOptions{
		MaxConcurrency: 10,
	}
	
	executor := NewPlanExecutor(mockCommunicator, mockLogger, options)
	
	// Create plan with 10 parallel steps
	steps := make([]routing.RoutingStep, 10)
	for i := 0; i < 10; i++ {
		steps[i] = routing.RoutingStep{
			Order:       1,
			StepID:      fmt.Sprintf("step-%d", i),
			AgentName:   fmt.Sprintf("agent%d", i),
			Instruction: "Task",
			Parallel:    true,
		}
		mockCommunicator.On("CallAgent", mock.Anything, mock.Anything, mock.Anything).Return("Result", nil)
	}
	
	plan := &routing.RoutingPlan{
		ID:    "bench-plan",
		Steps: steps,
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.Execute(ctx, plan)
	}
}