package providers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// mockLogger for testing
type mockLogger struct {
	debugCalls []map[string]interface{}
	infoCalls  []map[string]interface{}
	errorCalls []map[string]interface{}
}

func (m *mockLogger) Debug(msg string, fields map[string]interface{}) {
	m.debugCalls = append(m.debugCalls, fields)
}

func (m *mockLogger) Info(msg string, fields map[string]interface{}) {
	m.infoCalls = append(m.infoCalls, fields)
}

func (m *mockLogger) Error(msg string, fields map[string]interface{}) {
	m.errorCalls = append(m.errorCalls, fields)
}

func (m *mockLogger) Warn(msg string, fields map[string]interface{}) {}

func (m *mockLogger) DebugWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	m.debugCalls = append(m.debugCalls, fields)
}

func (m *mockLogger) InfoWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	m.infoCalls = append(m.infoCalls, fields)
}

func (m *mockLogger) WarnWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
}

func (m *mockLogger) ErrorWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	m.errorCalls = append(m.errorCalls, fields)
}

func TestNewBaseClient(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		logger  core.Logger
	}{
		{
			name:    "with logger",
			timeout: 180 * time.Second,
			logger:  &mockLogger{},
		},
		{
			name:    "without logger",
			timeout: 60 * time.Second,
			logger:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewBaseClient(tt.timeout, tt.logger)

			if client == nil {
				t.Fatal("expected non-nil client")
			}

			if client.HTTPClient.Timeout != tt.timeout {
				t.Errorf("expected timeout %v, got %v", tt.timeout, client.HTTPClient.Timeout)
			}

			if tt.logger == nil {
				// When no logger is provided, we expect a NoOpLogger to be set
				if _, ok := client.Logger.(*core.NoOpLogger); !ok {
					t.Error("expected NoOpLogger when no logger provided")
				}
			}

			if tt.logger != nil && client.Logger != tt.logger {
				t.Error("logger not set correctly")
			}

			if client.MaxRetries != 3 {
				t.Errorf("expected default MaxRetries 3, got %d", client.MaxRetries)
			}
		})
	}
}

func TestBaseClient_ApplyDefaults(t *testing.T) {
	client := NewBaseClient(180*time.Second, nil)
	client.DefaultModel = "default-model"
	client.DefaultMaxTokens = 1000
	client.DefaultTemperature = 0.7
	client.DefaultSystemPrompt = "You are helpful"

	tests := []struct {
		name     string
		input    *core.AIOptions
		expected *core.AIOptions
	}{
		{
			name:  "nil options",
			input: nil,
			expected: &core.AIOptions{
				Model:        "default-model",
				MaxTokens:    1000,
				Temperature:  0.7,
				SystemPrompt: "You are helpful",
			},
		},
		{
			name:  "empty options",
			input: &core.AIOptions{},
			expected: &core.AIOptions{
				Model:        "default-model",
				MaxTokens:    1000,
				Temperature:  0.7,
				SystemPrompt: "You are helpful",
			},
		},
		{
			name: "partial options",
			input: &core.AIOptions{
				Model:       "custom-model",
				Temperature: 0.9,
			},
			expected: &core.AIOptions{
				Model:        "custom-model",
				MaxTokens:    1000,
				Temperature:  0.9,
				SystemPrompt: "You are helpful",
			},
		},
		{
			name: "full options",
			input: &core.AIOptions{
				Model:        "custom-model",
				MaxTokens:    500,
				Temperature:  0.5,
				SystemPrompt: "Custom prompt",
			},
			expected: &core.AIOptions{
				Model:        "custom-model",
				MaxTokens:    500,
				Temperature:  0.5,
				SystemPrompt: "Custom prompt",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.ApplyDefaults(tt.input)

			if result.Model != tt.expected.Model {
				t.Errorf("expected model %q, got %q", tt.expected.Model, result.Model)
			}
			if result.MaxTokens != tt.expected.MaxTokens {
				t.Errorf("expected MaxTokens %d, got %d", tt.expected.MaxTokens, result.MaxTokens)
			}
			if result.Temperature != tt.expected.Temperature {
				t.Errorf("expected Temperature %f, got %f", tt.expected.Temperature, result.Temperature)
			}
			if result.SystemPrompt != tt.expected.SystemPrompt {
				t.Errorf("expected SystemPrompt %q, got %q", tt.expected.SystemPrompt, result.SystemPrompt)
			}
		})
	}
}

func TestBaseClient_ExecuteWithRetry(t *testing.T) {
	tests := []struct {
		name           string
		serverBehavior func(w http.ResponseWriter, r *http.Request)
		maxRetries     int
		wantErr        bool
		expectedCalls  int
	}{
		{
			name: "success on first try",
			serverBehavior: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			},
			maxRetries:    3,
			wantErr:       false,
			expectedCalls: 1,
		},
		{
			name: "success after retry",
			serverBehavior: func() func(w http.ResponseWriter, r *http.Request) {
				count := 0
				return func(w http.ResponseWriter, r *http.Request) {
					count++
					if count < 2 {
						w.WriteHeader(http.StatusTooManyRequests)
						w.Write([]byte("rate limited"))
					} else {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("success"))
					}
				}
			}(),
			maxRetries:    3,
			wantErr:       false,
			expectedCalls: 2,
		},
		{
			name: "max retries exceeded",
			serverBehavior: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("server error"))
			},
			maxRetries:    2,
			wantErr:       true,
			expectedCalls: 3, // Initial + 2 retries
		},
		{
			name: "non-retryable error",
			serverBehavior: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("bad request"))
			},
			maxRetries:    3,
			wantErr:       false, // Returns response even for non-retryable
			expectedCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				tt.serverBehavior(w, r)
			}))
			defer server.Close()

			client := NewBaseClient(180*time.Second, nil)
			client.MaxRetries = tt.maxRetries
			client.RetryDelay = 10 * time.Millisecond // Short delay for tests

			req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			resp, err := client.ExecuteWithRetry(context.Background(), req)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if resp != nil {
					resp.Body.Close()
				}
			}

			if callCount != tt.expectedCalls {
				t.Errorf("expected %d calls, got %d", tt.expectedCalls, callCount)
			}
		})
	}
}

func TestBaseClient_HandleError(t *testing.T) {
	client := NewBaseClient(180*time.Second, nil)

	tests := []struct {
		name       string
		statusCode int
		body       []byte
		provider   string
		wantErr    string
	}{
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"error": "invalid request"}`),
			provider:   "TestProvider",
			wantErr:    "TestProvider API error: invalid request - {\"error\": \"invalid request\"}",
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       []byte(`{"error": "invalid api key"}`),
			provider:   "TestProvider",
			wantErr:    "TestProvider API error: invalid or missing API key",
		},
		{
			name:       "rate limit",
			statusCode: http.StatusTooManyRequests,
			body:       []byte(`{"error": "rate limit exceeded"}`),
			provider:   "TestProvider",
			wantErr:    "TestProvider API error: rate limit exceeded",
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			body:       []byte(`{"error": "internal server error"}`),
			provider:   "TestProvider",
			wantErr:    "TestProvider API error: service temporarily unavailable (status 500)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.HandleError(tt.statusCode, tt.body, tt.provider)

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if err.Error() != tt.wantErr {
				t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestBaseClient_Logging(t *testing.T) {
	logger := &mockLogger{}
	client := NewBaseClient(180*time.Second, logger)

	// Test LogRequest
	client.LogRequest("test-provider", "test-model", "test prompt")

	if len(logger.infoCalls) != 1 {
		t.Errorf("expected 1 info call, got %d", len(logger.infoCalls))
	}

	fields := logger.infoCalls[0]
	if fields["provider"] != "test-provider" {
		t.Errorf("expected provider test-provider, got %v", fields["provider"])
	}
	if fields["model"] != "test-model" {
		t.Errorf("expected model test-model, got %v", fields["model"])
	}

	// Test LogResponse
	usage := core.TokenUsage{
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
	}
	client.LogResponse(context.Background(), "test-provider", "test-model", usage, 100*time.Millisecond)

	if len(logger.infoCalls) != 2 {
		t.Errorf("expected 2 info calls, got %d", len(logger.infoCalls))
	}

	fields = logger.infoCalls[1] // Second info call is LogResponse
	if fields["provider"] != "test-provider" {
		t.Errorf("expected provider test-provider, got %v", fields["provider"])
	}
	if fields["total_tokens"] != 30 {
		t.Errorf("expected total_tokens 30, got %v", fields["total_tokens"])
	}

	// Test LogError
	client.LogError("test-provider", errors.New("test error"))

	if len(logger.errorCalls) != 1 {
		t.Errorf("expected 1 error call, got %d", len(logger.errorCalls))
	}

	fields = logger.errorCalls[0]
	if fields["provider"] != "test-provider" {
		t.Errorf("expected provider test-provider, got %v", fields["provider"])
	}
	if fields["error"] != "test error" {
		t.Errorf("expected error 'test error', got %v", fields["error"])
	}
}

func TestBaseClient_ContextCancellation(t *testing.T) {
	// Test that context cancellation is respected
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewBaseClient(180*time.Second, nil)

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)

	// Cancel context immediately
	cancel()

	_, err := client.ExecuteWithRetry(ctx, req)
	if err == nil {
		t.Error("expected error due to cancelled context, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

// mockTelemetry tracks telemetry calls for testing
type mockTelemetry struct {
	spanStarted bool
	spanName    string
}

func (m *mockTelemetry) StartSpan(ctx context.Context, name string) (context.Context, core.Span) {
	m.spanStarted = true
	m.spanName = name
	return ctx, &mockSpan{}
}

func (m *mockTelemetry) RecordMetric(name string, value float64, labels map[string]string) {}

// mockSpan tracks span operations for testing
type mockSpan struct {
	ended      bool
	attributes map[string]interface{}
	errors     []error
}

func (m *mockSpan) End() {
	m.ended = true
}

func (m *mockSpan) SetAttribute(key string, value interface{}) {
	if m.attributes == nil {
		m.attributes = make(map[string]interface{})
	}
	m.attributes[key] = value
}

func (m *mockSpan) RecordError(err error) {
	m.errors = append(m.errors, err)
}

func TestBaseClient_SetTelemetry(t *testing.T) {
	client := NewBaseClient(180*time.Second, nil)

	// Initially telemetry should be nil
	if client.Telemetry != nil {
		t.Error("expected nil telemetry initially")
	}

	// Set telemetry
	telemetry := &mockTelemetry{}
	client.SetTelemetry(telemetry)

	if client.Telemetry != telemetry {
		t.Error("telemetry not set correctly")
	}

	// Set to nil
	client.SetTelemetry(nil)
	if client.Telemetry != nil {
		t.Error("expected nil telemetry after setting nil")
	}
}

func TestBaseClient_StartSpan(t *testing.T) {
	tests := []struct {
		name           string
		telemetry      core.Telemetry
		spanName       string
		expectSpanType string
	}{
		{
			name:           "with telemetry",
			telemetry:      &mockTelemetry{},
			spanName:       "test.operation",
			expectSpanType: "*providers.mockSpan",
		},
		{
			name:           "without telemetry",
			telemetry:      nil,
			spanName:       "test.operation",
			expectSpanType: "*core.NoOpSpan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewBaseClient(180*time.Second, nil)
			client.Telemetry = tt.telemetry

			ctx := context.Background()
			newCtx, span := client.StartSpan(ctx, tt.spanName)

			if newCtx == nil {
				t.Error("expected non-nil context")
			}

			if span == nil {
				t.Error("expected non-nil span")
			}

			// Verify span type
			spanType := ""
			switch span.(type) {
			case *mockSpan:
				spanType = "*providers.mockSpan"
			case *core.NoOpSpan:
				spanType = "*core.NoOpSpan"
			}

			if spanType != tt.expectSpanType {
				t.Errorf("expected span type %s, got %s", tt.expectSpanType, spanType)
			}

			// Verify telemetry was called when present
			if mt, ok := tt.telemetry.(*mockTelemetry); ok {
				if !mt.spanStarted {
					t.Error("expected span to be started")
				}
				if mt.spanName != tt.spanName {
					t.Errorf("expected span name %q, got %q", tt.spanName, mt.spanName)
				}
			}
		})
	}
}

func TestBaseClient_LogResponseContent(t *testing.T) {
	logger := &mockLogger{}
	client := NewBaseClient(180*time.Second, logger)

	// Call LogResponseContent
	client.LogResponseContent("test-provider", "test-model", "This is a test response")

	// Verify debug call was made
	if len(logger.debugCalls) != 1 {
		t.Errorf("expected 1 debug call, got %d", len(logger.debugCalls))
	}

	fields := logger.debugCalls[0]
	if fields["provider"] != "test-provider" {
		t.Errorf("expected provider test-provider, got %v", fields["provider"])
	}
	if fields["model"] != "test-model" {
		t.Errorf("expected model test-model, got %v", fields["model"])
	}
	if fields["response"] != "This is a test response" {
		t.Errorf("expected response 'This is a test response', got %v", fields["response"])
	}
	if fields["response_length"] != 23 {
		t.Errorf("expected response_length 23, got %v", fields["response_length"])
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	// Verify defaults
	if config.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", config.MaxRetries)
	}

	if config.RetryDelay != time.Second {
		t.Errorf("expected RetryDelay 1s, got %v", config.RetryDelay)
	}

	if config.ShouldRetry == nil {
		t.Fatal("expected non-nil ShouldRetry function")
	}

	// Test ShouldRetry function with various scenarios
	tests := []struct {
		name     string
		resp     *http.Response
		err      error
		expected bool
	}{
		{
			name:     "network error",
			resp:     nil,
			err:      errors.New("network error"),
			expected: true,
		},
		{
			name:     "500 server error",
			resp:     &http.Response{StatusCode: 500},
			err:      nil,
			expected: true,
		},
		{
			name:     "502 bad gateway",
			resp:     &http.Response{StatusCode: 502},
			err:      nil,
			expected: true,
		},
		{
			name:     "429 rate limit",
			resp:     &http.Response{StatusCode: 429},
			err:      nil,
			expected: true,
		},
		{
			name:     "400 bad request",
			resp:     &http.Response{StatusCode: 400},
			err:      nil,
			expected: false,
		},
		{
			name:     "200 success",
			resp:     &http.Response{StatusCode: 200},
			err:      nil,
			expected: false,
		},
		{
			name:     "401 unauthorized",
			resp:     &http.Response{StatusCode: 401},
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.ShouldRetry(tt.resp, tt.err)
			if result != tt.expected {
				t.Errorf("expected ShouldRetry=%v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHandleError_DefaultCase(t *testing.T) {
	client := NewBaseClient(180*time.Second, nil)

	// Test with an unknown status code (not handled by specific cases)
	err := client.HandleError(418, []byte("I'm a teapot"), "TestProvider")

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expected := "TestProvider API error (status 418): I'm a teapot"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}
