package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// mockLogger implements core.Logger for testing
type mockLogger struct {
	logs []string
}

func (m *mockLogger) Debug(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, "DEBUG: "+msg)
}

func (m *mockLogger) Info(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, "INFO: "+msg)
}

func (m *mockLogger) Warn(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, "WARN: "+msg)
}

func (m *mockLogger) Error(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, "ERROR: "+msg)
}

func TestNewClient(t *testing.T) {
	logger := &mockLogger{}
	
	tests := []struct {
		name    string
		apiKey  string
		baseURL string
		want    struct {
			apiKey  string
			baseURL string
		}
	}{
		{
			name:   "with custom base URL",
			apiKey: "test-key",
			baseURL: "https://custom.api.com/v1",
			want: struct {
				apiKey  string
				baseURL string
			}{
				apiKey:  "test-key",
				baseURL: "https://custom.api.com/v1",
			},
		},
		{
			name:   "with default base URL",
			apiKey: "test-key",
			baseURL: "",
			want: struct {
				apiKey  string
				baseURL string
			}{
				apiKey:  "test-key",
				baseURL: "https://api.openai.com/v1",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.apiKey, tt.baseURL, logger)
			
			if client.apiKey != tt.want.apiKey {
				t.Errorf("apiKey = %q, want %q", client.apiKey, tt.want.apiKey)
			}
			if client.baseURL != tt.want.baseURL {
				t.Errorf("baseURL = %q, want %q", client.baseURL, tt.want.baseURL)
			}
			if client.DefaultModel != "gpt-3.5-turbo" {
				t.Errorf("DefaultModel = %q, want gpt-3.5-turbo", client.DefaultModel)
			}
		})
	}
}

func TestClient_GenerateResponse(t *testing.T) {
	tests := []struct {
		name           string
		apiKey         string
		prompt         string
		options        *core.AIOptions
		serverResponse string
		serverStatus   int
		wantError      bool
		wantContent    string
		validateReq    func(*testing.T, map[string]interface{})
	}{
		{
			name:   "successful response",
			apiKey: "test-key",
			prompt: "Hello, AI!",
			options: &core.AIOptions{
				Model:       "gpt-3.5-turbo",
				Temperature: 0.7,
				MaxTokens:   100,
			},
			serverResponse: `{
				"id": "chatcmpl-123",
				"object": "chat.completion",
				"created": 1677652288,
				"model": "gpt-3.5-turbo",
				"choices": [{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "Hello! How can I help you today?"
					},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 8,
					"total_tokens": 18
				}
			}`,
			serverStatus: http.StatusOK,
			wantError:    false,
			wantContent:  "Hello! How can I help you today?",
			validateReq: func(t *testing.T, req map[string]interface{}) {
				if req["model"] != "gpt-3.5-turbo" {
					t.Errorf("request model = %v, want gpt-3.5-turbo", req["model"])
				}
				if req["temperature"] != 0.7 {
					t.Errorf("request temperature = %v, want 0.7", req["temperature"])
				}
				if req["max_tokens"] != float64(100) {
					t.Errorf("request max_tokens = %v, want 100", req["max_tokens"])
				}
			},
		},
		{
			name:   "with system prompt",
			apiKey: "test-key",
			prompt: "What is 2+2?",
			options: &core.AIOptions{
				Model:        "gpt-3.5-turbo",
				SystemPrompt: "You are a helpful math tutor.",
				Temperature:  0.5,
				MaxTokens:    50,
			},
			serverResponse: `{
				"choices": [{
					"message": {
						"content": "2+2 equals 4."
					}
				}]
			}`,
			serverStatus: http.StatusOK,
			wantError:    false,
			wantContent:  "2+2 equals 4.",
			validateReq: func(t *testing.T, req map[string]interface{}) {
				messages := req["messages"].([]interface{})
				if len(messages) != 2 {
					t.Fatalf("expected 2 messages, got %d", len(messages))
				}
				
				// Check system message
				systemMsg := messages[0].(map[string]interface{})
				if systemMsg["role"] != "system" {
					t.Errorf("first message role = %v, want system", systemMsg["role"])
				}
				if systemMsg["content"] != "You are a helpful math tutor." {
					t.Errorf("system content = %v, want 'You are a helpful math tutor.'", systemMsg["content"])
				}
				
				// Check user message
				userMsg := messages[1].(map[string]interface{})
				if userMsg["role"] != "user" {
					t.Errorf("second message role = %v, want user", userMsg["role"])
				}
				if userMsg["content"] != "What is 2+2?" {
					t.Errorf("user content = %v, want 'What is 2+2?'", userMsg["content"])
				}
			},
		},
		{
			name:      "missing API key",
			apiKey:    "",
			prompt:    "Hello",
			options:   &core.AIOptions{Model: "gpt-3.5-turbo"},
			wantError: true,
		},
		{
			name:   "API error response",
			apiKey: "test-key",
			prompt: "Hello",
			options: &core.AIOptions{Model: "gpt-3.5-turbo"},
			serverResponse: `{
				"error": {
					"message": "Invalid API key",
					"type": "invalid_request_error",
					"code": "invalid_api_key"
				}
			}`,
			serverStatus: http.StatusUnauthorized,
			wantError:    true,
		},
		{
			name:   "malformed response",
			apiKey: "test-key",
			prompt: "Hello",
			options: &core.AIOptions{Model: "gpt-3.5-turbo"},
			serverResponse: `{invalid json}`,
			serverStatus: http.StatusOK,
			wantError:    true,
		},
		{
			name:   "empty choices array",
			apiKey: "test-key",
			prompt: "Hello",
			options: &core.AIOptions{Model: "gpt-3.5-turbo"},
			serverResponse: `{"choices": []}`,
			serverStatus: http.StatusOK,
			wantError:    true,
		},
		{
			name:   "with usage information",
			apiKey: "test-key",
			prompt: "Hello",
			options: &core.AIOptions{Model: "gpt-3.5-turbo"},
			serverResponse: `{
				"model": "gpt-3.5-turbo",
				"choices": [{
					"message": {"content": "Hi there!"},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 5,
					"completion_tokens": 3,
					"total_tokens": 8
				}
			}`,
			serverStatus: http.StatusOK,
			wantError:    false,
			wantContent:  "Hi there!",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			var capturedRequest map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify headers
				if auth := r.Header.Get("Authorization"); auth != "Bearer "+tt.apiKey && tt.apiKey != "" {
					t.Errorf("Authorization header = %q, want %q", auth, "Bearer "+tt.apiKey)
				}
				if ct := r.Header.Get("Content-Type"); ct != "application/json" {
					t.Errorf("Content-Type header = %q, want application/json", ct)
				}
				
				// Capture request body
				if r.Body != nil {
					body, _ := io.ReadAll(r.Body)
					json.Unmarshal(body, &capturedRequest)
				}
				
				// Send response
				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()
			
			// Create client
			logger := &mockLogger{}
			client := NewClient(tt.apiKey, server.URL, logger)
			
			// Make request
			ctx := context.Background()
			resp, err := client.GenerateResponse(ctx, tt.prompt, tt.options)
			
			// Check error
			if (err != nil) != tt.wantError {
				t.Errorf("GenerateResponse() error = %v, wantError %v", err, tt.wantError)
			}
			
			// If successful, check response
			if !tt.wantError && resp != nil {
				if resp.Content != tt.wantContent {
					t.Errorf("response content = %q, want %q", resp.Content, tt.wantContent)
				}
			}
			
			// Validate request if provided
			if tt.validateReq != nil && capturedRequest != nil {
				tt.validateReq(t, capturedRequest)
			}
		})
	}
}

func TestClient_GenerateResponseWithDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)
		
		// Verify defaults were applied
		if req["model"] != "gpt-3.5-turbo" {
			t.Errorf("model = %v, want gpt-3.5-turbo (default)", req["model"])
		}
		if req["temperature"] != 0.7 {
			t.Errorf("temperature = %v, want 0.7 (default)", req["temperature"])
		}
		if req["max_tokens"] != float64(1000) {
			t.Errorf("max_tokens = %v, want 1000 (default)", req["max_tokens"])
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices": [{"message": {"content": "response"}}]}`))
	}))
	defer server.Close()
	
	logger := &mockLogger{}
	client := NewClient("test-key", server.URL, logger)
	
	// Set defaults
	client.DefaultModel = "gpt-3.5-turbo"
	client.DefaultTemperature = 0.7
	client.DefaultMaxTokens = 1000
	
	// Call with nil options to use defaults
	_, err := client.GenerateResponse(context.Background(), "test", nil)
	if err != nil {
		t.Errorf("GenerateResponse() with defaults failed: %v", err)
	}
}

func TestClient_GenerateResponseContextCancellation(t *testing.T) {
	// Server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices": [{"message": {"content": "too late"}}]}`))
	}))
	defer server.Close()
	
	logger := &mockLogger{}
	client := NewClient("test-key", server.URL, logger)
	
	// Create context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	_, err := client.GenerateResponse(ctx, "test", &core.AIOptions{Model: "gpt-3.5-turbo"})
	if err == nil {
		t.Error("expected error from cancelled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got: %v", err)
	}
}

func TestClient_ResponseParsing(t *testing.T) {
	tests := []struct {
		name           string
		response       string
		wantError      bool
		wantContent    string
		wantModel      string
		wantTokens     int
	}{
		{
			name: "complete response with all fields",
			response: `{
				"id": "chatcmpl-123",
				"model": "gpt-4",
				"choices": [{
					"message": {"content": "Complete response"},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 5,
					"total_tokens": 15
				}
			}`,
			wantContent: "Complete response",
			wantModel:   "gpt-4",
			wantTokens:  15,
		},
		{
			name: "minimal valid response",
			response: `{
				"choices": [{
					"message": {"content": "Minimal"}
				}]
			}`,
			wantContent: "Minimal",
			wantModel:   "",
			wantTokens:  0,
		},
		{
			name: "error response from API",
			response: `{
				"error": {
					"message": "Rate limit exceeded",
					"type": "rate_limit_error"
				}
			}`,
			wantError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(tt.response, "error") {
					w.WriteHeader(http.StatusTooManyRequests)
				} else {
					w.WriteHeader(http.StatusOK)
				}
				w.Write([]byte(tt.response))
			}))
			defer server.Close()
			
			logger := &mockLogger{}
			client := NewClient("test-key", server.URL, logger)
			
			resp, err := client.GenerateResponse(
				context.Background(),
				"test",
				&core.AIOptions{Model: "gpt-3.5-turbo"},
			)
			
			if (err != nil) != tt.wantError {
				t.Errorf("error = %v, wantError %v", err, tt.wantError)
			}
			
			if !tt.wantError && resp != nil {
				if resp.Content != tt.wantContent {
					t.Errorf("content = %q, want %q", resp.Content, tt.wantContent)
				}
				if resp.Model != tt.wantModel {
					t.Errorf("model = %q, want %q", resp.Model, tt.wantModel)
				}
				if resp.Usage.TotalTokens != tt.wantTokens {
					t.Errorf("tokens = %d, want %d", resp.Usage.TotalTokens, tt.wantTokens)
				}
			}
		})
	}
}