package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/itsneelabh/gomind/core"
)

// mockFactory is a test factory for client testing
type mockFactory struct {
	name      string
	priority  int
	available bool
	client    core.AIClient
	createErr error
}

func (f *mockFactory) Name() string {
	return f.name
}

func (f *mockFactory) Description() string {
	return "Mock provider for testing"
}

func (f *mockFactory) Priority() int {
	return f.priority
}

func (f *mockFactory) Create(config *AIConfig) core.AIClient {
	if f.createErr != nil {
		return &errorClient{err: f.createErr}
	}
	return f.client
}

func (f *mockFactory) DetectEnvironment() (int, bool) {
	return f.priority, f.available
}

// mockAIClient is a test implementation of core.AIClient
type mockAIClient struct {
	generateFunc func(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error)
}

func (c *mockAIClient) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	if c.generateFunc != nil {
		return c.generateFunc(ctx, prompt, options)
	}
	return &core.AIResponse{
		Content: "mock response",
		Model:   "mock-model",
		Usage:   core.TokenUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	}, nil
}

// errorClient for testing error cases
type errorClient struct {
	err error
}

func (e *errorClient) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	return nil, e.err
}

func TestNewClient(t *testing.T) {
	// Save original registry
	originalRegistry := registry
	defer func() { registry = originalRegistry }()

	tests := []struct {
		name     string
		options  []AIOption
		setup    func()
		wantErr  bool
		errMsg   string
		validate func(*testing.T, core.AIClient)
	}{
		{
			name: "auto-detect with available provider",
			setup: func() {
				registry = &ProviderRegistry{
					providers: make(map[string]ProviderFactory),
				}
				registry.providers["mock1"] = &mockFactory{
					name:      "mock1",
					priority:  100,
					available: true,
					client:    &mockAIClient{},
				}
			},
			wantErr: false,
		},
		{
			name: "auto-detect with no available providers",
			setup: func() {
				registry = &ProviderRegistry{
					providers: make(map[string]ProviderFactory),
				}
				registry.providers["mock1"] = &mockFactory{
					name:      "mock1",
					priority:  100,
					available: false,
				}
			},
			wantErr: true,
			errMsg:  "no AI provider available",
		},
		{
			name: "explicit provider selection",
			options: []AIOption{
				WithProvider("mock2"),
			},
			setup: func() {
				registry = &ProviderRegistry{
					providers: make(map[string]ProviderFactory),
				}
				registry.providers["mock2"] = &mockFactory{
					name:   "mock2",
					client: &mockAIClient{},
				}
			},
			wantErr: false,
		},
		{
			name: "unknown provider",
			options: []AIOption{
				WithProvider("unknown"),
			},
			setup: func() {
				registry = &ProviderRegistry{
					providers: make(map[string]ProviderFactory),
				}
			},
			wantErr: true,
			errMsg:  "provider 'unknown' not registered",
		},
		{
			name: "provider with custom config",
			options: []AIOption{
				WithProvider("mock3"),
				WithAPIKey("test-key"),
				WithBaseURL("https://test.com"),
				WithModel("test-model"),
				WithTemperature(0.7),
				WithMaxTokens(1000),
			},
			setup: func() {
				registry = &ProviderRegistry{
					providers: make(map[string]ProviderFactory),
				}
				registry.providers["mock3"] = &mockFactory{
					name: "mock3",
					client: &mockAIClient{
						generateFunc: func(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
							// Verify config was passed through
							if options.Model != "test-model" {
								t.Errorf("expected model test-model, got %s", options.Model)
							}
							return &core.AIResponse{Content: "configured"}, nil
						},
					},
				}
			},
			wantErr: false,
			validate: func(t *testing.T, client core.AIClient) {
				resp, err := client.GenerateResponse(context.Background(), "test", &core.AIOptions{
					Model: "test-model",
				})
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if resp.Content != "configured" {
					t.Errorf("expected configured response, got %s", resp.Content)
				}
			},
		},
		{
			name: "auto-detect chooses highest priority",
			setup: func() {
				registry = &ProviderRegistry{
					providers: make(map[string]ProviderFactory),
				}
				registry.providers["low"] = &mockFactory{
					name:      "low",
					priority:  50,
					available: true,
					client: &mockAIClient{
						generateFunc: func(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
							return &core.AIResponse{Content: "low priority"}, nil
						},
					},
				}
				registry.providers["high"] = &mockFactory{
					name:      "high",
					priority:  150,
					available: true,
					client: &mockAIClient{
						generateFunc: func(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
							return &core.AIResponse{Content: "high priority"}, nil
						},
					},
				}
			},
			wantErr: false,
			validate: func(t *testing.T, client core.AIClient) {
				resp, err := client.GenerateResponse(context.Background(), "test", nil)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if resp.Content != "high priority" {
					t.Errorf("expected high priority provider, got %s", resp.Content)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			client, err := NewClient(tt.options...)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Error("expected client, got nil")
				return
			}

			if tt.validate != nil {
				tt.validate(t, client)
			}
		})
	}
}

func TestWithOptions(t *testing.T) {
	config := &AIConfig{}

	// Test WithProvider
	WithProvider("test-provider")(config)
	if config.Provider != "test-provider" {
		t.Errorf("expected provider test-provider, got %s", config.Provider)
	}

	// Test WithAPIKey
	WithAPIKey("test-key")(config)
	if config.APIKey != "test-key" {
		t.Errorf("expected API key test-key, got %s", config.APIKey)
	}

	// Test WithBaseURL
	WithBaseURL("https://test.com")(config)
	if config.BaseURL != "https://test.com" {
		t.Errorf("expected base URL https://test.com, got %s", config.BaseURL)
	}

	// Test WithModel
	WithModel("test-model")(config)
	if config.Model != "test-model" {
		t.Errorf("expected model test-model, got %s", config.Model)
	}

	// Test WithTemperature
	WithTemperature(0.8)(config)
	if config.Temperature != 0.8 {
		t.Errorf("expected temperature 0.8, got %f", config.Temperature)
	}

	// Test WithMaxTokens
	WithMaxTokens(2000)(config)
	if config.MaxTokens != 2000 {
		t.Errorf("expected max tokens 2000, got %d", config.MaxTokens)
	}

	// Test WithRegion
	WithRegion("us-west-2")(config)
	if config.Extra["region"] != "us-west-2" {
		t.Errorf("expected region us-west-2, got %v", config.Extra["region"])
	}

	// Test WithAWSCredentials
	WithAWSCredentials("access", "secret", "token")(config)
	if config.Extra["aws_access_key_id"] != "access" {
		t.Errorf("expected access key 'access', got %v", config.Extra["aws_access_key_id"])
	}
	if config.Extra["aws_secret_access_key"] != "secret" {
		t.Errorf("expected secret key 'secret', got %v", config.Extra["aws_secret_access_key"])
	}
	if config.Extra["aws_session_token"] != "token" {
		t.Errorf("expected session token 'token', got %v", config.Extra["aws_session_token"])
	}
}

func TestAutoDetectProvider(t *testing.T) {
	// Save original registry
	originalRegistry := registry
	defer func() { registry = originalRegistry }()

	tests := []struct {
		name          string
		factories     []ProviderFactory
		expectedName  string
		expectedError string
	}{
		{
			name: "single available provider",
			factories: []ProviderFactory{
				&mockFactory{name: "provider1", priority: 100, available: true},
			},
			expectedName: "provider1",
		},
		{
			name: "multiple providers, highest priority wins",
			factories: []ProviderFactory{
				&mockFactory{name: "provider1", priority: 50, available: true},
				&mockFactory{name: "provider2", priority: 100, available: true},
				&mockFactory{name: "provider3", priority: 75, available: true},
			},
			expectedName: "provider2",
		},
		{
			name: "only unavailable providers",
			factories: []ProviderFactory{
				&mockFactory{name: "provider1", priority: 100, available: false},
				&mockFactory{name: "provider2", priority: 200, available: false},
			},
			expectedError: "no provider detected in environment",
		},
		{
			name:          "no providers registered",
			factories:     []ProviderFactory{},
			expectedError: "no provider detected in environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup registry with test factories
			registry = &ProviderRegistry{
				providers: make(map[string]ProviderFactory),
			}
			for _, f := range tt.factories {
				registry.providers[f.Name()] = f
			}

			providerName, err := detectBestProvider(nil)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("expected error %q, got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError {
					t.Errorf("expected error %q, got %q", tt.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if providerName != tt.expectedName {
				t.Errorf("expected provider %s, got %s", tt.expectedName, providerName)
			}
		})
	}
}
