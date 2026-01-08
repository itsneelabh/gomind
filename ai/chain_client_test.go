package ai

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/itsneelabh/gomind/core"
)

// ================================
// Phase 3 Unit Tests: Chain Client (Pure Unit Tests Only)
// ================================
//
// Integration tests requiring provider registration are in:
//   - chain_client_integration_test.go (run with: go test -tags=integration)
//

// TestPhase3_ConfigurationValidation verifies fail-fast configuration validation
func TestPhase3_ConfigurationValidation(t *testing.T) {
	tests := []struct {
		name          string
		opts          []ChainOption
		expectError   bool
		errorContains string
		description   string
	}{
		{
			name:          "Empty chain fails fast",
			opts:          []ChainOption{},
			expectError:   true,
			errorContains: "at least one provider required",
			description:   "Configuration error: empty chain",
		},
		{
			name: "Invalid provider alias fails fast",
			opts: []ChainOption{
				WithProviderChain("invalid-provider"),
			},
			expectError:   true,
			errorContains: "unknown provider alias",
			description:   "Configuration error: invalid alias",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewChainClient(tt.opts...)

			if tt.expectError {
				if err == nil {
					t.Errorf("%s: Expected error, got nil", tt.description)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("%s: Expected error containing %q, got %q",
						tt.description, tt.errorContains, err.Error())
				}
			} else if err != nil {
				t.Errorf("%s: Unexpected error: %v", tt.description, err)
			}
		})
	}
}

// TestPhase3_NoProvidersAvailableFails verifies fail-fast when no providers work
func TestPhase3_NoProvidersAvailableFails(t *testing.T) {
	// Clean environment - no API keys
	originalVars := saveChainEnvironment()
	defer restoreChainEnvironment(originalVars)
	clearAllChainEnvVars()

	// Try to create chain without any API keys
	client, err := NewChainClient(
		WithProviderChain("openai", "openai.deepseek"),
	)

	// Should fail - no providers available
	if err == nil {
		t.Error("Expected error when no providers available")
	}
	if client != nil {
		t.Error("Expected nil client when creation fails")
	}
	if err != nil && !strings.Contains(err.Error(), "no providers could be initialized") {
		t.Errorf("Expected 'no providers could be initialized' error, got: %v", err)
	}
}

// TestPhase3_ErrorClassification verifies isClientError function
//
// IMPORTANT: In a provider chain, auth errors SHOULD trigger failover because
// each provider has its own API key. Auth failure on OpenAI should try Anthropic.
//
// Non-retryable (isClientError=true): bad request, content policy, invalid parameter, malformed
// Retryable (isClientError=false): auth errors, server errors, timeouts, network errors
func TestPhase3_ErrorClassification(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		isClientErr bool
		description string
	}{
		// Auth errors SHOULD failover (each provider has different API key)
		{
			name:        "Authentication error allows failover",
			err:         errors.New("authentication failed"),
			isClientErr: false,
			description: "Auth errors should try next provider",
		},
		{
			name:        "Unauthorized allows failover",
			err:         errors.New("unauthorized access"),
			isClientErr: false,
			description: "401 errors should try next provider",
		},
		{
			name:        "API key error allows failover",
			err:         errors.New("api key is invalid"),
			isClientErr: false,
			description: "API key errors should try next provider",
		},
		{
			name:        "401 status allows failover",
			err:         errors.New("status code 401"),
			isClientErr: false,
			description: "401 status should try next provider",
		},
		// True client errors should NOT failover (same bad input fails everywhere)
		{
			name:        "Invalid parameter is client error",
			err:         errors.New("invalid parameter value"),
			isClientErr: true,
			description: "Invalid params would fail on any provider",
		},
		{
			name:        "Bad request is client error",
			err:         errors.New("bad request format"),
			isClientErr: true,
			description: "Malformed requests would fail on any provider",
		},
		{
			name:        "Content policy is client error",
			err:         errors.New("content policy violation"),
			isClientErr: true,
			description: "Policy violations would fail on any provider",
		},
		{
			name:        "Malformed input is client error",
			err:         errors.New("malformed JSON input"),
			isClientErr: true,
			description: "Malformed input would fail on any provider",
		},
		// Server/network errors should failover
		{
			name:        "Server error is retryable",
			err:         errors.New("internal server error"),
			isClientErr: false,
			description: "5xx errors should try next provider",
		},
		{
			name:        "Timeout is retryable",
			err:         errors.New("request timeout"),
			isClientErr: false,
			description: "Timeouts should try next provider",
		},
		{
			name:        "Network error is retryable",
			err:         errors.New("network connection failed"),
			isClientErr: false,
			description: "Network errors should try next provider",
		},
		{
			name:        "Unknown error is retryable",
			err:         errors.New("some random error"),
			isClientErr: false,
			description: "Conservative: unknown errors should try next provider",
		},
		// Edge cases - not found and forbidden are NOT in client error patterns
		// so they default to retryable (conservative approach)
		{
			name:        "Not found allows failover",
			err:         errors.New("resource not found"),
			isClientErr: false,
			description: "Not found defaults to retryable",
		},
		{
			name:        "Forbidden allows failover",
			err:         errors.New("forbidden access to resource"),
			isClientErr: false,
			description: "Forbidden defaults to retryable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isClientError(tt.err)
			if result != tt.isClientErr {
				t.Errorf("%s: isClientError(%q) = %v, want %v",
					tt.description, tt.err.Error(), result, tt.isClientErr)
			}
		})
	}
}

// TestPhase3_ChainOptions verifies configuration options
func TestPhase3_ChainOptions(t *testing.T) {
	t.Run("WithProviderChain sets aliases", func(t *testing.T) {
		config := &ChainConfig{}
		option := WithProviderChain("openai", "anthropic", "gemini")
		option(config)

		if len(config.ProviderAliases) != 3 {
			t.Errorf("Expected 3 aliases, got %d", len(config.ProviderAliases))
		}
		if config.ProviderAliases[0] != "openai" {
			t.Errorf("Expected first alias 'openai', got %q", config.ProviderAliases[0])
		}
		if config.ProviderAliases[1] != "anthropic" {
			t.Errorf("Expected second alias 'anthropic', got %q", config.ProviderAliases[1])
		}
		if config.ProviderAliases[2] != "gemini" {
			t.Errorf("Expected third alias 'gemini', got %q", config.ProviderAliases[2])
		}
	})

	t.Run("WithChainLogger sets logger", func(t *testing.T) {
		config := &ChainConfig{}
		logger := &testLogger{logs: make([]string, 0)}
		option := WithChainLogger(logger)
		option(config)

		if config.Logger == nil {
			t.Error("Expected logger to be set")
		}
	})
}

// ================================
// Mock Implementations for Testing
// ================================

// testLogger captures log messages for verification
type testLogger struct {
	logs []string
}

func (l *testLogger) Debug(msg string, fields map[string]interface{}) {
	l.logs = append(l.logs, "DEBUG: "+msg)
}

func (l *testLogger) Info(msg string, fields map[string]interface{}) {
	l.logs = append(l.logs, "INFO: "+msg)
}

func (l *testLogger) Warn(msg string, fields map[string]interface{}) {
	l.logs = append(l.logs, "WARN: "+msg)
}

func (l *testLogger) Error(msg string, fields map[string]interface{}) {
	l.logs = append(l.logs, "ERROR: "+msg)
}

func (l *testLogger) DebugWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	l.logs = append(l.logs, "DEBUG: "+msg)
}

func (l *testLogger) InfoWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	l.logs = append(l.logs, "INFO: "+msg)
}

func (l *testLogger) WarnWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	l.logs = append(l.logs, "WARN: "+msg)
}

func (l *testLogger) ErrorWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	l.logs = append(l.logs, "ERROR: "+msg)
}

// chainMockAIClient for testing failover behavior (renamed to avoid conflicts)
type chainMockAIClient struct {
	name       string
	shouldFail bool
	failWith   error
	callCount  int
}

func (m *chainMockAIClient) GenerateResponse(ctx context.Context, prompt string, opts *core.AIOptions) (*core.AIResponse, error) {
	m.callCount++
	if m.shouldFail {
		return nil, m.failWith
	}
	return &core.AIResponse{
		Content: "Mock response from " + m.name,
		Model:   m.name,
	}, nil
}

// TestPhase3_FailoverBehavior verifies automatic failover logic
func TestPhase3_FailoverBehavior(t *testing.T) {
	tests := []struct {
		name            string
		providers       []core.AIClient
		providerAliases []string // Required: must match providers length
		expectSuccess   bool
		expectedCalls   map[string]int
		description     string
	}{
		{
			name: "First provider succeeds",
			providers: []core.AIClient{
				&chainMockAIClient{name: "provider1", shouldFail: false},
				&chainMockAIClient{name: "provider2", shouldFail: false},
			},
			providerAliases: []string{"provider1", "provider2"},
			expectSuccess:   true,
			expectedCalls:   map[string]int{"provider1": 1, "provider2": 0},
			description:     "Should use first provider only",
		},
		{
			name: "Failover to second provider",
			providers: []core.AIClient{
				&chainMockAIClient{name: "provider1", shouldFail: true, failWith: errors.New("server error")},
				&chainMockAIClient{name: "provider2", shouldFail: false},
			},
			providerAliases: []string{"provider1", "provider2"},
			expectSuccess:   true,
			expectedCalls:   map[string]int{"provider1": 1, "provider2": 1},
			description:     "Should failover on server error",
		},
		{
			name: "Auth error allows failover",
			providers: []core.AIClient{
				&chainMockAIClient{name: "provider1", shouldFail: true, failWith: errors.New("invalid api key")},
				&chainMockAIClient{name: "provider2", shouldFail: false},
			},
			providerAliases: []string{"provider1", "provider2"},
			expectSuccess:   true,
			expectedCalls:   map[string]int{"provider1": 1, "provider2": 1},
			description:     "Auth errors should try next provider (different API keys)",
		},
		{
			name: "True client error stops failover",
			providers: []core.AIClient{
				&chainMockAIClient{name: "provider1", shouldFail: true, failWith: errors.New("bad request: invalid prompt format")},
				&chainMockAIClient{name: "provider2", shouldFail: false},
			},
			providerAliases: []string{"provider1", "provider2"},
			expectSuccess:   false,
			expectedCalls:   map[string]int{"provider1": 1, "provider2": 0},
			description:     "Bad request errors should not retry (same input fails everywhere)",
		},
		{
			name: "All providers fail",
			providers: []core.AIClient{
				&chainMockAIClient{name: "provider1", shouldFail: true, failWith: errors.New("server error")},
				&chainMockAIClient{name: "provider2", shouldFail: true, failWith: errors.New("server error")},
			},
			providerAliases: []string{"provider1", "provider2"},
			expectSuccess:   false,
			expectedCalls:   map[string]int{"provider1": 1, "provider2": 1},
			description:     "Should try all providers before failing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &ChainClient{
				providers:       tt.providers,
				providerAliases: tt.providerAliases,
				logger:          &core.NoOpLogger{},
			}

			ctx := context.Background()
			resp, err := client.GenerateResponse(ctx, "test prompt", nil)

			if tt.expectSuccess {
				if err != nil {
					t.Errorf("%s: Expected success, got error: %v", tt.description, err)
				}
				if resp == nil {
					t.Errorf("%s: Expected response, got nil", tt.description)
				}
			} else {
				if err == nil {
					t.Errorf("%s: Expected error, got success", tt.description)
				}
			}

			// Verify call counts
			for _, provider := range tt.providers {
				mock := provider.(*chainMockAIClient)
				expectedCalls := tt.expectedCalls[mock.name]
				if mock.callCount != expectedCalls {
					t.Errorf("%s: Provider %s: expected %d calls, got %d",
						tt.description, mock.name, expectedCalls, mock.callCount)
				}
			}
		})
	}
}

// ================================
// Helper Functions
// ================================

// saveChainEnvironment saves all environment variables
func saveChainEnvironment() map[string]string {
	vars := []string{
		"OPENAI_API_KEY", "OPENAI_BASE_URL",
		"GROQ_API_KEY", "GROQ_BASE_URL",
		"DEEPSEEK_API_KEY", "DEEPSEEK_BASE_URL",
		"XAI_API_KEY", "XAI_BASE_URL",
		"QWEN_API_KEY", "QWEN_BASE_URL",
		"TOGETHER_API_KEY", "TOGETHER_BASE_URL",
		"OLLAMA_BASE_URL",
		"ANTHROPIC_API_KEY",
		"GEMINI_API_KEY",
	}
	saved := make(map[string]string)
	for _, v := range vars {
		saved[v] = os.Getenv(v)
	}
	return saved
}

// restoreChainEnvironment restores environment variables
func restoreChainEnvironment(saved map[string]string) {
	for k, v := range saved {
		if v == "" {
			os.Unsetenv(k)
		} else {
			os.Setenv(k, v)
		}
	}
}

// clearAllChainEnvVars clears all provider environment variables
func clearAllChainEnvVars() {
	vars := []string{
		"OPENAI_API_KEY", "OPENAI_BASE_URL",
		"GROQ_API_KEY", "GROQ_BASE_URL",
		"DEEPSEEK_API_KEY", "DEEPSEEK_BASE_URL",
		"XAI_API_KEY", "XAI_BASE_URL",
		"QWEN_API_KEY", "QWEN_BASE_URL",
		"TOGETHER_API_KEY", "TOGETHER_BASE_URL",
		"OLLAMA_BASE_URL",
		"ANTHROPIC_API_KEY",
		"GEMINI_API_KEY",
	}
	for _, v := range vars {
		os.Unsetenv(v)
	}
}

// ================================
// Tests for cloneAIOptions (Options Mutation Bug Fix)
// ================================
//
// These tests verify the cloneAIOptions function that prevents mutation
// bleeding during chain failover. See: ai/MODEL_ALIAS_CROSS_PROVIDER_PROPOSAL.md
//

// TestCloneAIOptions_NilInput verifies nil input returns nil output
func TestCloneAIOptions_NilInput(t *testing.T) {
	result := cloneAIOptions(nil)
	if result != nil {
		t.Errorf("Expected nil output for nil input, got %+v", result)
	}
}

// TestCloneAIOptions_CopiesAllFields verifies all fields are copied correctly
func TestCloneAIOptions_CopiesAllFields(t *testing.T) {
	original := &core.AIOptions{
		Model:        "gpt-4",
		Temperature:  0.7,
		MaxTokens:    1000,
		SystemPrompt: "You are a helpful assistant.",
	}

	clone := cloneAIOptions(original)

	// Verify clone is not nil
	if clone == nil {
		t.Fatal("Expected non-nil clone, got nil")
	}

	// Verify clone is a different pointer
	if clone == original {
		t.Error("Clone should be a different pointer than original")
	}

	// Verify all fields are copied
	if clone.Model != original.Model {
		t.Errorf("Model mismatch: got %q, want %q", clone.Model, original.Model)
	}
	if clone.Temperature != original.Temperature {
		t.Errorf("Temperature mismatch: got %v, want %v", clone.Temperature, original.Temperature)
	}
	if clone.MaxTokens != original.MaxTokens {
		t.Errorf("MaxTokens mismatch: got %d, want %d", clone.MaxTokens, original.MaxTokens)
	}
	if clone.SystemPrompt != original.SystemPrompt {
		t.Errorf("SystemPrompt mismatch: got %q, want %q", clone.SystemPrompt, original.SystemPrompt)
	}
}

// TestCloneAIOptions_MutationIsolation verifies mutation of clone doesn't affect original
// This is the critical behavior that fixes the chain failover bug
func TestCloneAIOptions_MutationIsolation(t *testing.T) {
	original := &core.AIOptions{
		Model:        "smart",
		Temperature:  0.5,
		MaxTokens:    500,
		SystemPrompt: "Original prompt",
	}

	clone := cloneAIOptions(original)

	// Mutate the clone (simulates what ApplyDefaults does)
	clone.Model = "gpt-4.1-mini-2025-04-14" // Resolved model name
	clone.Temperature = 0.8
	clone.MaxTokens = 2000
	clone.SystemPrompt = "Modified prompt"

	// Verify original is unchanged
	if original.Model != "smart" {
		t.Errorf("Original Model was mutated: got %q, want %q", original.Model, "smart")
	}
	if original.Temperature != 0.5 {
		t.Errorf("Original Temperature was mutated: got %v, want %v", original.Temperature, 0.5)
	}
	if original.MaxTokens != 500 {
		t.Errorf("Original MaxTokens was mutated: got %d, want %d", original.MaxTokens, 500)
	}
	if original.SystemPrompt != "Original prompt" {
		t.Errorf("Original SystemPrompt was mutated: got %q, want %q", original.SystemPrompt, "Original prompt")
	}
}

// TestCloneAIOptions_EmptyOptions verifies cloning works with zero-value options
func TestCloneAIOptions_EmptyOptions(t *testing.T) {
	original := &core.AIOptions{} // All zero values

	clone := cloneAIOptions(original)

	if clone == nil {
		t.Fatal("Expected non-nil clone, got nil")
	}
	if clone == original {
		t.Error("Clone should be a different pointer than original")
	}

	// Verify zero values are preserved
	if clone.Model != "" {
		t.Errorf("Expected empty Model, got %q", clone.Model)
	}
	if clone.Temperature != 0 {
		t.Errorf("Expected zero Temperature, got %v", clone.Temperature)
	}
	if clone.MaxTokens != 0 {
		t.Errorf("Expected zero MaxTokens, got %d", clone.MaxTokens)
	}
	if clone.SystemPrompt != "" {
		t.Errorf("Expected empty SystemPrompt, got %q", clone.SystemPrompt)
	}
}
