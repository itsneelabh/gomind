package openai

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/ai"
)

func TestFactory_Name(t *testing.T) {
	factory := &Factory{}
	if factory.Name() != "openai" {
		t.Errorf("expected name 'openai', got %q", factory.Name())
	}
}

func TestFactory_Description(t *testing.T) {
	factory := &Factory{}
	desc := factory.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
	if desc != "Universal OpenAI-compatible provider (OpenAI, Groq, DeepSeek, Qwen, local models, etc.)" {
		t.Errorf("unexpected description: %q", desc)
	}
}

func TestFactory_Priority(t *testing.T) {
	factory := &Factory{}
	// OpenAI factory should have default priority of 100
	prio, _ := factory.DetectEnvironment()
	if prio != 0 && prio != 100 {
		t.Errorf("expected priority 0 (no env) or 100, got %d", prio)
	}
}

func TestFactory_DetectEnvironment(t *testing.T) {
	factory := &Factory{}

	tests := []struct {
		name      string
		setup     func()
		cleanup   func()
		wantPrio  int
		wantAvail bool
	}{
		{
			name: "with OPENAI_API_KEY",
			setup: func() {
				os.Setenv("OPENAI_API_KEY", "test-key")
			},
			cleanup: func() {
				os.Unsetenv("OPENAI_API_KEY")
				os.Unsetenv("OPENAI_BASE_URL")
			},
			wantPrio:  100,
			wantAvail: true,
		},
		{
			name: "with GROQ_API_KEY",
			setup: func() {
				os.Setenv("GROQ_API_KEY", "test-key")
			},
			cleanup: func() {
				os.Unsetenv("GROQ_API_KEY")
				os.Unsetenv("OPENAI_BASE_URL")
				os.Unsetenv("OPENAI_API_KEY")
			},
			wantPrio:  95,
			wantAvail: true,
		},
		{
			name: "with DEEPSEEK_API_KEY",
			setup: func() {
				os.Setenv("DEEPSEEK_API_KEY", "test-key")
			},
			cleanup: func() {
				os.Unsetenv("DEEPSEEK_API_KEY")
				os.Unsetenv("OPENAI_BASE_URL")
				os.Unsetenv("OPENAI_API_KEY")
			},
			wantPrio:  90,
			wantAvail: true,
		},
		{
			name: "with XAI_API_KEY",
			setup: func() {
				os.Setenv("XAI_API_KEY", "test-key")
			},
			cleanup: func() {
				os.Unsetenv("XAI_API_KEY")
				os.Unsetenv("OPENAI_BASE_URL")
				os.Unsetenv("OPENAI_API_KEY")
			},
			wantPrio:  85,
			wantAvail: true,
		},
		{
			name: "with QWEN_API_KEY",
			setup: func() {
				os.Setenv("QWEN_API_KEY", "test-key")
			},
			cleanup: func() {
				os.Unsetenv("QWEN_API_KEY")
				os.Unsetenv("OPENAI_BASE_URL")
				os.Unsetenv("OPENAI_API_KEY")
			},
			wantPrio:  80,
			wantAvail: true,
		},
		{
			name:      "no environment variables",
			setup:     func() {},
			cleanup:   func() {},
			wantPrio:  0,
			wantAvail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all relevant env vars first
			os.Unsetenv("OPENAI_API_KEY")
			os.Unsetenv("OPENAI_BASE_URL")
			os.Unsetenv("GROQ_API_KEY")
			os.Unsetenv("DEEPSEEK_API_KEY")
			os.Unsetenv("XAI_API_KEY")
			os.Unsetenv("QWEN_API_KEY")

			tt.setup()
			defer tt.cleanup()

			prio, avail := factory.DetectEnvironment()

			if prio != tt.wantPrio {
				t.Errorf("expected priority %d, got %d", tt.wantPrio, prio)
			}
			if avail != tt.wantAvail {
				t.Errorf("expected available %v, got %v", tt.wantAvail, avail)
			}
		})
	}
}

func TestFactory_Create(t *testing.T) {
	factory := &Factory{}

	tests := []struct {
		name   string
		config *ai.AIConfig
		verify func(*testing.T, *Client)
	}{
		{
			name: "with API key from config",
			config: &ai.AIConfig{
				APIKey: "config-key",
			},
			verify: func(t *testing.T, c *Client) {
				if c.apiKey != "config-key" {
					t.Errorf("expected API key 'config-key', got %q", c.apiKey)
				}
			},
		},
		{
			name: "with base URL from config",
			config: &ai.AIConfig{
				BaseURL: "https://custom.api.com/v1",
			},
			verify: func(t *testing.T, c *Client) {
				if c.baseURL != "https://custom.api.com/v1" {
					t.Errorf("expected base URL 'https://custom.api.com/v1', got %q", c.baseURL)
				}
			},
		},
		{
			name: "with timeout configuration",
			config: &ai.AIConfig{
				Timeout: 60 * time.Second,
			},
			verify: func(t *testing.T, c *Client) {
				if c.HTTPClient.Timeout != 60*time.Second {
					t.Errorf("expected timeout 60s, got %v", c.HTTPClient.Timeout)
				}
			},
		},
		{
			name: "with retry configuration",
			config: &ai.AIConfig{
				MaxRetries: 5,
			},
			verify: func(t *testing.T, c *Client) {
				if c.MaxRetries != 5 {
					t.Errorf("expected MaxRetries 5, got %d", c.MaxRetries)
				}
			},
		},
		{
			name: "with model configuration",
			config: &ai.AIConfig{
				Model: "gpt-4-turbo",
			},
			verify: func(t *testing.T, c *Client) {
				if c.DefaultModel != "gpt-4-turbo" {
					t.Errorf("expected model 'gpt-4-turbo', got %q", c.DefaultModel)
				}
			},
		},
		{
			name: "with temperature configuration",
			config: &ai.AIConfig{
				Temperature: 0.8,
			},
			verify: func(t *testing.T, c *Client) {
				if c.DefaultTemperature != 0.8 {
					t.Errorf("expected temperature 0.8, got %f", c.DefaultTemperature)
				}
			},
		},
		{
			name: "with max tokens configuration",
			config: &ai.AIConfig{
				MaxTokens: 2000,
			},
			verify: func(t *testing.T, c *Client) {
				if c.DefaultMaxTokens != 2000 {
					t.Errorf("expected max tokens 2000, got %d", c.DefaultMaxTokens)
				}
			},
		},
		{
			name:   "with API key from environment",
			config: &ai.AIConfig{},
			verify: func(t *testing.T, c *Client) {
				// Set env var for this test
				os.Setenv("OPENAI_API_KEY", "env-key")
				defer os.Unsetenv("OPENAI_API_KEY")

				// Recreate client to pick up env var
				newClient := factory.Create(&ai.AIConfig{})
				if openaiClient, ok := newClient.(*Client); ok {
					if openaiClient.apiKey != "env-key" {
						t.Errorf("expected API key from env 'env-key', got %q", openaiClient.apiKey)
					}
				}
			},
		},
		{
			name:   "with base URL from environment",
			config: &ai.AIConfig{},
			verify: func(t *testing.T, c *Client) {
				// Set env var for this test
				os.Setenv("OPENAI_BASE_URL", "https://env.api.com/v1")
				defer os.Unsetenv("OPENAI_BASE_URL")

				// Recreate client to pick up env var
				newClient := factory.Create(&ai.AIConfig{})
				if openaiClient, ok := newClient.(*Client); ok {
					if openaiClient.baseURL != "https://env.api.com/v1" {
						t.Errorf("expected base URL from env, got %q", openaiClient.baseURL)
					}
				}
			},
		},
		{
			name:   "default configuration",
			config: &ai.AIConfig{},
			verify: func(t *testing.T, c *Client) {
				// DefaultModel is now "default" alias which gets resolved at request-time
				// This enables runtime model override via GOMIND_OPENAI_MODEL_DEFAULT env var
				if c.DefaultModel != "default" {
					t.Errorf("expected default model 'default' (alias), got %q", c.DefaultModel)
				}
				if c.DefaultMaxTokens != 1000 {
					t.Errorf("expected default max tokens 1000, got %d", c.DefaultMaxTokens)
				}
				if c.MaxRetries != 3 {
					t.Errorf("expected default max retries 3, got %d", c.MaxRetries)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := factory.Create(tt.config)

			if client == nil {
				t.Fatal("expected non-nil client")
			}

			// Type assert to access internal fields for verification
			openaiClient, ok := client.(*Client)
			if !ok {
				t.Fatal("expected *Client type")
			}

			tt.verify(t, openaiClient)
		})
	}
}

func TestFactory_CreateWithHeaders(t *testing.T) {
	factory := &Factory{}

	config := &ai.AIConfig{
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
			"Authorization":   "Bearer custom-token",
		},
	}

	client := factory.Create(config)

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	// Verify that custom headers are applied via transport
	openaiClient, ok := client.(*Client)
	if !ok {
		t.Fatal("expected *Client type")
	}

	// Check that transport was set
	if openaiClient.HTTPClient.Transport == nil {
		t.Error("expected custom transport for headers, got nil")
	}

	// Verify it's a headerTransport
	if _, ok := openaiClient.HTTPClient.Transport.(*headerTransport); !ok {
		t.Error("expected headerTransport type")
	}
}

func TestHeaderTransport_RoundTrip(t *testing.T) {
	headers := map[string]string{
		"X-Custom-Header": "test-value",
		"X-Another":       "another-value",
	}

	transport := &headerTransport{
		headers: headers,
		base:    &mockRoundTripper{},
	}

	req, _ := http.NewRequest("GET", "http://test.com", nil)

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	// Verify headers were added
	if req.Header.Get("X-Custom-Header") != "test-value" {
		t.Errorf("expected header X-Custom-Header='test-value', got %q", req.Header.Get("X-Custom-Header"))
	}
	if req.Header.Get("X-Another") != "another-value" {
		t.Errorf("expected header X-Another='another-value', got %q", req.Header.Get("X-Another"))
	}
}

// mockRoundTripper for testing headerTransport
type mockRoundTripper struct{}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200}, nil
}
