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
func TestPhase3_ErrorClassification(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		isClientErr bool
		description string
	}{
		{
			name:        "Authentication error is client error",
			err:         errors.New("authentication failed"),
			isClientErr: true,
			description: "4xx errors shouldn't be retried",
		},
		{
			name:        "Unauthorized is client error",
			err:         errors.New("unauthorized access"),
			isClientErr: true,
			description: "401 errors are client errors",
		},
		{
			name:        "Invalid request is client error",
			err:         errors.New("invalid request parameters"),
			isClientErr: true,
			description: "400 errors are client errors",
		},
		{
			name:        "Bad request is client error",
			err:         errors.New("bad request format"),
			isClientErr: true,
			description: "Malformed requests shouldn't retry",
		},
		{
			name:        "API key error is client error",
			err:         errors.New("api key is invalid"),
			isClientErr: true,
			description: "Missing/invalid API keys are client errors",
		},
		{
			name:        "Not found is client error",
			err:         errors.New("resource not found"),
			isClientErr: true,
			description: "404 errors are client errors",
		},
		{
			name:        "Forbidden is client error",
			err:         errors.New("forbidden access to resource"),
			isClientErr: true,
			description: "403 errors are client errors",
		},
		{
			name:        "Server error is retryable",
			err:         errors.New("internal server error"),
			isClientErr: false,
			description: "5xx errors should be retried",
		},
		{
			name:        "Timeout is retryable",
			err:         errors.New("request timeout"),
			isClientErr: false,
			description: "Timeouts should be retried",
		},
		{
			name:        "Network error is retryable",
			err:         errors.New("network connection failed"),
			isClientErr: false,
			description: "Network errors should be retried",
		},
		{
			name:        "Unknown error is retryable",
			err:         errors.New("some random error"),
			isClientErr: false,
			description: "Conservative: unknown errors should retry",
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
	name        string
	shouldFail  bool
	failWith    error
	callCount   int
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
		name           string
		providers      []core.AIClient
		expectSuccess  bool
		expectedCalls  map[string]int
		description    string
	}{
		{
			name: "First provider succeeds",
			providers: []core.AIClient{
				&chainMockAIClient{name: "provider1", shouldFail: false},
				&chainMockAIClient{name: "provider2", shouldFail: false},
			},
			expectSuccess: true,
			expectedCalls: map[string]int{"provider1": 1, "provider2": 0},
			description:   "Should use first provider only",
		},
		{
			name: "Failover to second provider",
			providers: []core.AIClient{
				&chainMockAIClient{name: "provider1", shouldFail: true, failWith: errors.New("server error")},
				&chainMockAIClient{name: "provider2", shouldFail: false},
			},
			expectSuccess: true,
			expectedCalls: map[string]int{"provider1": 1, "provider2": 1},
			description:   "Should failover on server error",
		},
		{
			name: "Client error stops failover",
			providers: []core.AIClient{
				&chainMockAIClient{name: "provider1", shouldFail: true, failWith: errors.New("invalid api key")},
				&chainMockAIClient{name: "provider2", shouldFail: false},
			},
			expectSuccess: false,
			expectedCalls: map[string]int{"provider1": 1, "provider2": 0},
			description:   "Should not retry client errors",
		},
		{
			name: "All providers fail",
			providers: []core.AIClient{
				&chainMockAIClient{name: "provider1", shouldFail: true, failWith: errors.New("server error")},
				&chainMockAIClient{name: "provider2", shouldFail: true, failWith: errors.New("server error")},
			},
			expectSuccess: false,
			expectedCalls: map[string]int{"provider1": 1, "provider2": 1},
			description:   "Should try all providers before failing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &ChainClient{
				providers: tt.providers,
				logger:    &core.NoOpLogger{},
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
