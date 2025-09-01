package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultConfig verifies that DefaultConfig returns valid defaults
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	
	assert.NotNil(t, cfg)
	assert.Equal(t, "gomind-agent", cfg.Name)
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "default", cfg.Namespace)
	
	// HTTP defaults
	assert.Equal(t, 30*time.Second, cfg.HTTP.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.HTTP.WriteTimeout)
	assert.Equal(t, 120*time.Second, cfg.HTTP.IdleTimeout)
	assert.True(t, cfg.HTTP.EnableHealthCheck)
	assert.Equal(t, "/health", cfg.HTTP.HealthCheckPath)
	
	// CORS defaults (should be disabled for security)
	assert.False(t, cfg.HTTP.CORS.Enabled)
	assert.Equal(t, []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}, cfg.HTTP.CORS.AllowedMethods)
	
	// Discovery defaults (disabled for local dev)
	assert.False(t, cfg.Discovery.Enabled)
	assert.Equal(t, "redis", cfg.Discovery.Provider)
	
	// AI defaults (disabled without key)
	assert.False(t, cfg.AI.Enabled)
	assert.Equal(t, "openai", cfg.AI.Provider)
	assert.Equal(t, "gpt-4", cfg.AI.Model)
	
	// Telemetry defaults (disabled by default)
	assert.False(t, cfg.Telemetry.Enabled)
	
	// Memory defaults
	assert.Equal(t, "inmemory", cfg.Memory.Provider)
	assert.Equal(t, 1000, cfg.Memory.MaxSize)
	
	// Logging defaults
	assert.Equal(t, "info", cfg.Logging.Level)
}

// TestDetectEnvironment verifies environment detection logic
func TestDetectEnvironment(t *testing.T) {
	t.Run("Kubernetes environment", func(t *testing.T) {
		// Set Kubernetes environment variable
		_ = os.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
		defer func() { _ = os.Unsetenv("KUBERNETES_SERVICE_HOST") }()
		
		cfg := DefaultConfig()
		
		assert.True(t, cfg.Kubernetes.Enabled)
		assert.Equal(t, "0.0.0.0", cfg.Address)
		assert.True(t, cfg.Discovery.Enabled)
		assert.Equal(t, "redis://redis.default.svc.cluster.local:6379", cfg.Discovery.RedisURL)
		assert.Equal(t, "json", cfg.Logging.Format)
	})
	
	t.Run("Local environment", func(t *testing.T) {
		// Ensure no Kubernetes env var
		_ = os.Unsetenv("KUBERNETES_SERVICE_HOST")
		_ = os.Unsetenv("GOMIND_DEV_MODE")
		
		cfg := DefaultConfig()
		
		assert.False(t, cfg.Kubernetes.Enabled)
		assert.Equal(t, "localhost", cfg.Address)
		assert.Equal(t, "redis://localhost:6379", cfg.Discovery.RedisURL)
		assert.True(t, cfg.Development.Enabled)
		assert.True(t, cfg.Development.PrettyLogs)
		assert.Equal(t, "text", cfg.Logging.Format)
	})
}

// TestLoadFromEnv verifies environment variable loading
func TestLoadFromEnv(t *testing.T) {
	// Set test environment variables
	testEnv := map[string]string{
		"GOMIND_AGENT_NAME":       "test-agent",
		"GOMIND_AGENT_ID":         "test-123",
		"GOMIND_PORT":             "9090",
		"GOMIND_ADDRESS":          "0.0.0.0",
		"GOMIND_NAMESPACE":        "testing",
		"GOMIND_LOG_LEVEL":        "debug",
		"GOMIND_LOG_FORMAT":       "json",
		"GOMIND_CORS_ENABLED":     "true",
		"GOMIND_CORS_ORIGINS":     "https://example.com,https://*.example.com",
		"GOMIND_CORS_CREDENTIALS": "true",
		"GOMIND_REDIS_URL":        "redis://test-redis:6379",
		"GOMIND_DISCOVERY_CACHE":  "false",
		"OPENAI_API_KEY":          "sk-test-key",
		"GOMIND_AI_MODEL":         "gpt-4-turbo",
		"GOMIND_DEV_MODE":         "true",
		"GOMIND_MOCK_AI":          "true",
		"GOMIND_MOCK_DISCOVERY":   "true",
	}
	
	// Set environment variables
	for k, v := range testEnv {
		_ = os.Setenv(k, v)
		defer func() { _ = os.Unsetenv(k) }()
	}
	
	cfg := DefaultConfig()
	err := cfg.LoadFromEnv()
	require.NoError(t, err)
	
	// Verify values loaded from environment
	assert.Equal(t, "test-agent", cfg.Name)
	assert.Equal(t, "test-123", cfg.ID)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, "0.0.0.0", cfg.Address)
	assert.Equal(t, "testing", cfg.Namespace)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Format) // Dev mode sets text format
	
	// CORS configuration
	assert.True(t, cfg.HTTP.CORS.Enabled)
	assert.Equal(t, []string{"https://example.com", "https://*.example.com"}, cfg.HTTP.CORS.AllowedOrigins)
	assert.True(t, cfg.HTTP.CORS.AllowCredentials)
	
	// Discovery configuration
	assert.Equal(t, "redis://test-redis:6379", cfg.Discovery.RedisURL)
	assert.False(t, cfg.Discovery.CacheEnabled)
	
	// AI configuration
	assert.True(t, cfg.AI.Enabled)
	assert.Equal(t, "sk-test-key", cfg.AI.APIKey)
	assert.Equal(t, "gpt-4-turbo", cfg.AI.Model)
	
	// Development configuration
	assert.True(t, cfg.Development.Enabled)
	assert.True(t, cfg.Development.MockAI)
	assert.True(t, cfg.Development.MockDiscovery)
}

// TestLoadFromFile verifies JSON file loading
func TestLoadFromFile(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")
	
	configData := map[string]interface{}{
		"name":      "file-agent",
		"port":      8888,
		"namespace": "file-namespace",
		"http": map[string]interface{}{
			"cors": map[string]interface{}{
				"enabled":         true,
				"allowed_origins": []string{"https://file.example.com"},
			},
		},
		"ai": map[string]interface{}{
			"enabled": true,
			"model":   "gpt-3.5-turbo",
		},
		"logging": map[string]interface{}{
			"level":  "warn",
			"format": "text",
		},
	}
	
	jsonData, err := json.MarshalIndent(configData, "", "  ")
	require.NoError(t, err)
	
	err = os.WriteFile(configFile, jsonData, 0644)
	require.NoError(t, err)
	
	cfg := DefaultConfig()
	err = cfg.LoadFromFile(configFile)
	require.NoError(t, err)
	
	assert.Equal(t, "file-agent", cfg.Name)
	assert.Equal(t, 8888, cfg.Port)
	assert.Equal(t, "file-namespace", cfg.Namespace)
	assert.True(t, cfg.HTTP.CORS.Enabled)
	assert.Equal(t, []string{"https://file.example.com"}, cfg.HTTP.CORS.AllowedOrigins)
	assert.True(t, cfg.AI.Enabled)
	assert.Equal(t, "gpt-3.5-turbo", cfg.AI.Model)
	assert.Equal(t, "warn", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Format)
}

// TestValidate verifies configuration validation
func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Config)
		wantErr string
	}{
		{
			name: "valid configuration",
			setup: func(cfg *Config) {
				cfg.Name = "test-agent"
				cfg.Port = 8080
			},
			wantErr: "",
		},
		{
			name: "invalid port - too low",
			setup: func(cfg *Config) {
				cfg.Port = 0
			},
			wantErr: "invalid port: 0",
		},
		{
			name: "invalid port - too high",
			setup: func(cfg *Config) {
				cfg.Port = 70000
			},
			wantErr: "invalid port: 70000",
		},
		{
			name: "missing agent name",
			setup: func(cfg *Config) {
				cfg.Name = ""
			},
			wantErr: "agent name is required",
		},
		{
			name: "AI enabled without API key",
			setup: func(cfg *Config) {
				cfg.AI.Enabled = true
				cfg.AI.APIKey = ""
				cfg.Development.MockAI = false
			},
			wantErr: "AI API key is required when AI is enabled",
		},
		{
			name: "AI enabled with mock",
			setup: func(cfg *Config) {
				cfg.AI.Enabled = true
				cfg.AI.APIKey = ""
				cfg.Development.MockAI = true
			},
			wantErr: "",
		},
		{
			name: "Telemetry enabled without endpoint",
			setup: func(cfg *Config) {
				cfg.Telemetry.Enabled = true
				cfg.Telemetry.Endpoint = ""
			},
			wantErr: "telemetry endpoint is required when telemetry is enabled",
		},
		{
			name: "Redis discovery without URL",
			setup: func(cfg *Config) {
				cfg.Discovery.Enabled = true
				cfg.Discovery.Provider = "redis"
				cfg.Discovery.RedisURL = ""
				cfg.Development.MockDiscovery = false
			},
			wantErr: "redis URL is required for Redis discovery provider",
		},
		{
			name: "Redis discovery with mock",
			setup: func(cfg *Config) {
				cfg.Discovery.Enabled = true
				cfg.Discovery.Provider = "redis"
				cfg.Discovery.RedisURL = ""
				cfg.Development.MockDiscovery = true
			},
			wantErr: "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.setup(cfg)
			
			err := cfg.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

// TestFunctionalOptions verifies all functional options
func TestFunctionalOptions(t *testing.T) {
	t.Run("WithName", func(t *testing.T) {
		cfg, err := NewConfig(WithName("custom-agent"))
		require.NoError(t, err)
		assert.Equal(t, "custom-agent", cfg.Name)
	})
	
	t.Run("WithPort", func(t *testing.T) {
		cfg, err := NewConfig(WithPort(9999))
		require.NoError(t, err)
		assert.Equal(t, 9999, cfg.Port)
		
		// Test invalid port
		_, err = NewConfig(WithPort(0))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port")
	})
	
	t.Run("WithAddress", func(t *testing.T) {
		cfg, err := NewConfig(WithAddress("127.0.0.1"))
		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1", cfg.Address)
	})
	
	t.Run("WithNamespace", func(t *testing.T) {
		cfg, err := NewConfig(WithNamespace("production"))
		require.NoError(t, err)
		assert.Equal(t, "production", cfg.Namespace)
	})
	
	t.Run("WithCORS", func(t *testing.T) {
		origins := []string{"https://example.com", "https://*.example.com"}
		cfg, err := NewConfig(WithCORS(origins, true))
		require.NoError(t, err)
		assert.True(t, cfg.HTTP.CORS.Enabled)
		assert.Equal(t, origins, cfg.HTTP.CORS.AllowedOrigins)
		assert.True(t, cfg.HTTP.CORS.AllowCredentials)
	})
	
	t.Run("WithCORSDefaults", func(t *testing.T) {
		cfg, err := NewConfig(WithCORSDefaults())
		require.NoError(t, err)
		assert.True(t, cfg.HTTP.CORS.Enabled)
		assert.Equal(t, []string{"*"}, cfg.HTTP.CORS.AllowedOrigins)
		assert.True(t, cfg.HTTP.CORS.AllowCredentials)
	})
	
	t.Run("WithRedisURL", func(t *testing.T) {
		url := "redis://custom-redis:6379"
		cfg, err := NewConfig(WithRedisURL(url))
		require.NoError(t, err)
		assert.Equal(t, url, cfg.Discovery.RedisURL)
		assert.Equal(t, url, cfg.Memory.RedisURL)
		assert.True(t, cfg.Discovery.Enabled)
	})
	
	t.Run("WithDiscovery", func(t *testing.T) {
		cfg, err := NewConfig(WithDiscovery(true, "custom"))
		require.NoError(t, err)
		assert.True(t, cfg.Discovery.Enabled)
		assert.Equal(t, "custom", cfg.Discovery.Provider)
	})
	
	t.Run("WithDiscoveryCacheEnabled", func(t *testing.T) {
		cfg, err := NewConfig(WithDiscoveryCacheEnabled(false))
		require.NoError(t, err)
		assert.False(t, cfg.Discovery.CacheEnabled)
	})
	
	t.Run("WithOpenAIAPIKey", func(t *testing.T) {
		cfg, err := NewConfig(WithOpenAIAPIKey("sk-test"))
		require.NoError(t, err)
		assert.True(t, cfg.AI.Enabled)
		assert.Equal(t, "openai", cfg.AI.Provider)
		assert.Equal(t, "sk-test", cfg.AI.APIKey)
	})
	
	t.Run("WithAI", func(t *testing.T) {
		cfg, err := NewConfig(WithAI(true, "anthropic", "key"))
		require.NoError(t, err)
		assert.True(t, cfg.AI.Enabled)
		assert.Equal(t, "anthropic", cfg.AI.Provider)
		assert.Equal(t, "key", cfg.AI.APIKey)
	})
	
	t.Run("WithAIModel", func(t *testing.T) {
		cfg, err := NewConfig(WithAIModel("gpt-4-turbo"))
		require.NoError(t, err)
		assert.Equal(t, "gpt-4-turbo", cfg.AI.Model)
	})
	
	t.Run("WithTelemetry", func(t *testing.T) {
		cfg, err := NewConfig(WithTelemetry(true, "http://otel:4317"))
		require.NoError(t, err)
		assert.True(t, cfg.Telemetry.Enabled)
		assert.Equal(t, "http://otel:4317", cfg.Telemetry.Endpoint)
	})
	
	t.Run("WithEnableMetrics", func(t *testing.T) {
		cfg, err := NewConfig(
			WithTelemetry(true, "http://otel:4317"),
			WithEnableMetrics(false),
		)
		require.NoError(t, err)
		assert.False(t, cfg.Telemetry.MetricsEnabled)
	})
	
	t.Run("WithEnableTracing", func(t *testing.T) {
		cfg, err := NewConfig(
			WithTelemetry(true, "http://otel:4317"),
			WithEnableTracing(false),
		)
		require.NoError(t, err)
		assert.False(t, cfg.Telemetry.TracingEnabled)
	})
	
	t.Run("WithOTELEndpoint", func(t *testing.T) {
		cfg, err := NewConfig(WithOTELEndpoint("http://jaeger:4317"))
		require.NoError(t, err)
		assert.True(t, cfg.Telemetry.Enabled)
		assert.Equal(t, "otel", cfg.Telemetry.Provider)
		assert.Equal(t, "http://jaeger:4317", cfg.Telemetry.Endpoint)
	})
	
	t.Run("WithLogLevel", func(t *testing.T) {
		cfg, err := NewConfig(WithLogLevel("debug"))
		require.NoError(t, err)
		assert.Equal(t, "debug", cfg.Logging.Level)
	})
	
	t.Run("WithLogFormat", func(t *testing.T) {
		cfg, err := NewConfig(WithLogFormat("text"))
		require.NoError(t, err)
		assert.Equal(t, "text", cfg.Logging.Format)
	})
	
	t.Run("WithMemoryProvider", func(t *testing.T) {
		cfg, err := NewConfig(WithMemoryProvider("redis"))
		require.NoError(t, err)
		assert.Equal(t, "redis", cfg.Memory.Provider)
	})
	
	t.Run("WithCircuitBreaker", func(t *testing.T) {
		cfg, err := NewConfig(WithCircuitBreaker(10, 60*time.Second))
		require.NoError(t, err)
		assert.True(t, cfg.Resilience.CircuitBreaker.Enabled)
		assert.Equal(t, 10, cfg.Resilience.CircuitBreaker.Threshold)
		assert.Equal(t, 60*time.Second, cfg.Resilience.CircuitBreaker.Timeout)
	})
	
	t.Run("WithRetry", func(t *testing.T) {
		cfg, err := NewConfig(WithRetry(5, 2*time.Second))
		require.NoError(t, err)
		assert.Equal(t, 5, cfg.Resilience.Retry.MaxAttempts)
		assert.Equal(t, 2*time.Second, cfg.Resilience.Retry.InitialInterval)
	})
	
	t.Run("WithKubernetes", func(t *testing.T) {
		cfg, err := NewConfig(WithKubernetes(true, true))
		require.NoError(t, err)
		assert.True(t, cfg.Kubernetes.EnableServiceDiscovery)
		assert.True(t, cfg.Kubernetes.EnableLeaderElection)
	})
	
	t.Run("WithDevelopmentMode", func(t *testing.T) {
		cfg, err := NewConfig(WithDevelopmentMode(true))
		require.NoError(t, err)
		assert.True(t, cfg.Development.Enabled)
		assert.True(t, cfg.Development.PrettyLogs)
		assert.Equal(t, "text", cfg.Logging.Format)
		assert.Equal(t, "debug", cfg.Logging.Level)
	})
	
	t.Run("WithMockAI", func(t *testing.T) {
		cfg, err := NewConfig(WithMockAI(true))
		require.NoError(t, err)
		assert.True(t, cfg.Development.MockAI)
		assert.True(t, cfg.AI.Enabled)
	})
	
	t.Run("WithMockDiscovery", func(t *testing.T) {
		cfg, err := NewConfig(WithMockDiscovery(true))
		require.NoError(t, err)
		assert.True(t, cfg.Development.MockDiscovery)
		assert.True(t, cfg.Discovery.Enabled)
	})
}

// TestConfigPriority verifies configuration priority order
func TestConfigPriority(t *testing.T) {
	// Set environment variable
	_ = os.Setenv("GOMIND_PORT", "7777")
	defer func() { _ = os.Unsetenv("GOMIND_PORT") }()
	
	// Create config with functional option (should override env)
	cfg, err := NewConfig(WithPort(8888))
	require.NoError(t, err)
	
	// Functional option should win over environment variable
	assert.Equal(t, 8888, cfg.Port)
}

// TestParseHelpers verifies helper functions
func TestParseHelpers(t *testing.T) {
	t.Run("parseStringList", func(t *testing.T) {
		tests := []struct {
			input    string
			expected []string
		}{
			{"a,b,c", []string{"a", "b", "c"}},
			{"a, b, c", []string{"a", "b", "c"}},
			{"  a  ,  b  ,  c  ", []string{"a", "b", "c"}},
			{"a", []string{"a"}},
			{"", []string{}},
			{",,,", []string{}},
			{"a,,b", []string{"a", "b"}},
		}
		
		for _, tt := range tests {
			result := parseStringList(tt.input)
			assert.Equal(t, tt.expected, result, "input: %s", tt.input)
		}
	})
	
	t.Run("parseBool", func(t *testing.T) {
		tests := []struct {
			input    string
			expected bool
		}{
			{"true", true},
			{"True", true},
			{"TRUE", true},
			{"1", true},
			{"yes", true},
			{"YES", true},
			{"on", true},
			{"ON", true},
			{"false", false},
			{"False", false},
			{"0", false},
			{"no", false},
			{"off", false},
			{"", false},
			{"invalid", false},
		}
		
		for _, tt := range tests {
			result := parseBool(tt.input)
			assert.Equal(t, tt.expected, result, "input: %s", tt.input)
		}
	})
}

// TestConfigWithConfigFile verifies WithConfigFile option
func TestConfigWithConfigFile(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.json")
	
	configData := map[string]interface{}{
		"name": "file-loaded-agent",
		"port": 7777,
		"http": map[string]interface{}{
			"cors": map[string]interface{}{
				"enabled": true,
			},
		},
	}
	
	jsonData, err := json.MarshalIndent(configData, "", "  ")
	require.NoError(t, err)
	
	err = os.WriteFile(configFile, jsonData, 0644)
	require.NoError(t, err)
	
	// Load config from file using option
	cfg, err := NewConfig(
		WithConfigFile(configFile),
		WithPort(8888), // This should override the file
	)
	require.NoError(t, err)
	
	assert.Equal(t, "file-loaded-agent", cfg.Name)
	assert.Equal(t, 8888, cfg.Port) // Option overrides file
	assert.True(t, cfg.HTTP.CORS.Enabled)
}

// BenchmarkNewConfig benchmarks configuration creation
func BenchmarkNewConfig(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewConfig(
			WithName("bench-agent"),
			WithPort(8080),
			WithCORS([]string{"https://example.com"}, true),
			WithRedisURL("redis://localhost:6379"),
		)
	}
}

// BenchmarkLoadFromEnv benchmarks environment variable loading
func BenchmarkLoadFromEnv(b *testing.B) {
	// Set test environment variables
	_ = os.Setenv("GOMIND_AGENT_NAME", "bench-agent")
	_ = os.Setenv("GOMIND_PORT", "8080")
	_ = os.Setenv("GOMIND_CORS_ENABLED", "true")
	defer func() {
		_ = os.Unsetenv("GOMIND_AGENT_NAME")
		_ = os.Unsetenv("GOMIND_PORT")
		_ = os.Unsetenv("GOMIND_CORS_ENABLED")
	}()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg := DefaultConfig()
		_ = cfg.LoadFromEnv()
	}
}

// BenchmarkValidate benchmarks configuration validation
func BenchmarkValidate(b *testing.B) {
	cfg := DefaultConfig()
	cfg.Name = "bench-agent"
	cfg.Port = 8080
	cfg.AI.Enabled = true
	cfg.AI.APIKey = "sk-test"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cfg.Validate()
	}
}

// ExampleNewConfig demonstrates basic configuration usage
func ExampleNewConfig() {
	cfg, err := NewConfig(
		WithName("example-agent"),
		WithPort(8080),
		WithCORS([]string{"https://example.com"}, true),
	)
	if err != nil {
		panic(err)
	}
	
	fmt.Printf("Agent: %s on port %d\n", cfg.Name, cfg.Port)
	// Output: Agent: example-agent on port 8080
}

// ExampleNewConfig_development demonstrates development configuration
func ExampleNewConfig_development() {
	cfg, err := NewConfig(
		WithName("dev-agent"),
		WithPort(8080),
		WithDevelopmentMode(true),
		WithMockDiscovery(true),
		WithMockAI(true),
	)
	if err != nil {
		panic(err)
	}
	
	fmt.Printf("Development mode: %v, Mock AI: %v\n", 
		cfg.Development.Enabled, cfg.Development.MockAI)
	// Output: Development mode: true, Mock AI: true
}

// ExampleNewConfig_production demonstrates production configuration
func ExampleNewConfig_production() {
	cfg, err := NewConfig(
		WithName("prod-agent"),
		WithPort(8080),
		WithAddress("0.0.0.0"),
		WithNamespace("production"),
		WithCORS([]string{
			"https://app.example.com",
			"https://*.example.com",
		}, true),
		WithRedisURL("redis://redis:6379"),
		WithOpenAIAPIKey("sk-test-example"), // Use test key for example
		WithOTELEndpoint("http://jaeger:4317"),
		WithCircuitBreaker(5, 30*time.Second),
	)
	if err != nil {
		panic(err)
	}
	
	fmt.Printf("Production config: %s in %s namespace\n", 
		cfg.Name, cfg.Namespace)
	// Output: Production config: prod-agent in production namespace
}