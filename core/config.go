package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration options for the GoMind framework.
// It supports three-layer configuration priority:
//  1. Default values (lowest priority)
//  2. Environment variables (medium priority)
//  3. Functional options (highest priority)
//
// The configuration automatically detects the execution environment (Kubernetes vs local)
// and adjusts defaults accordingly.
//
// Example usage:
//
//	cfg, err := NewConfig(
//	    WithName("my-agent"),
//	    WithPort(8080),
//	    WithCORS([]string{"https://example.com"}, true),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
type Config struct {
	// Core configuration
	Name      string `json:"name" env:"GOMIND_AGENT_NAME"`
	ID        string `json:"id" env:"GOMIND_AGENT_ID"`
	Port      int    `json:"port" env:"GOMIND_PORT" default:"8080"`
	Address   string `json:"address" env:"GOMIND_ADDRESS"`
	Namespace string `json:"namespace" env:"GOMIND_NAMESPACE" default:"default"`

	// HTTP Server configuration
	HTTP HTTPConfig `json:"http"`

	// Discovery configuration
	Discovery DiscoveryConfig `json:"discovery"`

	// AI configuration (optional module)
	AI AIConfig `json:"ai"`

	// Telemetry configuration (optional module)
	Telemetry TelemetryConfig `json:"telemetry"`

	// Memory configuration
	Memory MemoryConfig `json:"memory"`

	// Resilience configuration
	Resilience ResilienceConfig `json:"resilience"`

	// Logging configuration
	Logging LoggingConfig `json:"logging"`

	// Development configuration
	Development DevelopmentConfig `json:"development"`

	// Kubernetes specific configuration
	Kubernetes KubernetesConfig `json:"kubernetes"`

	// Logger instance for configuration operations (excluded from JSON)
	logger Logger `json:"-"`
}

// HTTPConfig contains HTTP server configuration including timeouts, limits, and CORS settings.
// All timeout values use time.Duration for flexibility.
type HTTPConfig struct {
	ReadTimeout       time.Duration `json:"read_timeout" env:"GOMIND_HTTP_READ_TIMEOUT" default:"30s"`
	ReadHeaderTimeout time.Duration `json:"read_header_timeout" env:"GOMIND_HTTP_READ_HEADER_TIMEOUT" default:"10s"`
	WriteTimeout      time.Duration `json:"write_timeout" env:"GOMIND_HTTP_WRITE_TIMEOUT" default:"30s"`
	IdleTimeout       time.Duration `json:"idle_timeout" env:"GOMIND_HTTP_IDLE_TIMEOUT" default:"120s"`
	MaxHeaderBytes    int           `json:"max_header_bytes" env:"GOMIND_HTTP_MAX_HEADER_BYTES" default:"1048576"`
	ShutdownTimeout   time.Duration `json:"shutdown_timeout" env:"GOMIND_HTTP_SHUTDOWN_TIMEOUT" default:"10s"`
	EnableHealthCheck bool          `json:"enable_health_check" env:"GOMIND_HTTP_HEALTH_CHECK" default:"true"`
	HealthCheckPath   string        `json:"health_check_path" env:"GOMIND_HTTP_HEALTH_PATH" default:"/health"`
	CORS              CORSConfig    `json:"cors"`
}

// CORSConfig contains Cross-Origin Resource Sharing (CORS) configuration.
// Supports wildcard domains (e.g., *.example.com) and wildcard ports (e.g., http://localhost:*).
//
// Security note: Be cautious with AllowCredentials=true and ensure AllowedOrigins
// is properly restricted in production environments.
type CORSConfig struct {
	Enabled          bool     `json:"enabled" env:"GOMIND_CORS_ENABLED" default:"false"`
	AllowedOrigins   []string `json:"allowed_origins" env:"GOMIND_CORS_ORIGINS"`
	AllowedMethods   []string `json:"allowed_methods" env:"GOMIND_CORS_METHODS" default:"GET,POST,PUT,DELETE,OPTIONS"`
	AllowedHeaders   []string `json:"allowed_headers" env:"GOMIND_CORS_HEADERS" default:"Content-Type,Authorization"`
	ExposedHeaders   []string `json:"exposed_headers" env:"GOMIND_CORS_EXPOSED_HEADERS"`
	AllowCredentials bool     `json:"allow_credentials" env:"GOMIND_CORS_CREDENTIALS" default:"false"`
	MaxAge           int      `json:"max_age" env:"GOMIND_CORS_MAX_AGE" default:"86400"`
}

// DiscoveryConfig contains service discovery configuration.
// Currently supports Redis as the discovery backend with optional caching.
// When MockDiscovery is enabled in Development mode, an in-memory discovery is used instead.
type DiscoveryConfig struct {
	Enabled           bool          `json:"enabled" env:"GOMIND_DISCOVERY_ENABLED" default:"false"`
	Provider          string        `json:"provider" env:"GOMIND_DISCOVERY_PROVIDER" default:"redis"`
	RedisURL          string        `json:"redis_url" env:"GOMIND_REDIS_URL,REDIS_URL"`
	CacheEnabled      bool          `json:"cache_enabled" env:"GOMIND_DISCOVERY_CACHE" default:"true"`
	CacheTTL          time.Duration `json:"cache_ttl" env:"GOMIND_DISCOVERY_CACHE_TTL" default:"5m"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval" env:"GOMIND_DISCOVERY_HEARTBEAT" default:"10s"`
	TTL               time.Duration `json:"ttl" env:"GOMIND_DISCOVERY_TTL" default:"30s"`
}

// AIConfig contains AI client configuration for LLM integration.
// This is an optional module - AI features are only initialized when Enabled=true.
// Supports OpenAI and compatible APIs. When MockAI is enabled in Development mode,
// returns canned responses without making actual API calls.
type AIConfig struct {
	Enabled       bool          `json:"enabled" env:"GOMIND_AI_ENABLED" default:"false"`
	Provider      string        `json:"provider" env:"GOMIND_AI_PROVIDER" default:"openai"`
	APIKey        string        `json:"api_key" env:"GOMIND_AI_API_KEY,OPENAI_API_KEY"`
	BaseURL       string        `json:"base_url" env:"GOMIND_AI_BASE_URL"`
	Model         string        `json:"model" env:"GOMIND_AI_MODEL" default:"gpt-4"`
	Temperature   float32       `json:"temperature" env:"GOMIND_AI_TEMPERATURE" default:"0.7"`
	MaxTokens     int           `json:"max_tokens" env:"GOMIND_AI_MAX_TOKENS" default:"2000"`
	Timeout       time.Duration `json:"timeout" env:"GOMIND_AI_TIMEOUT" default:"30s"`
	RetryAttempts int           `json:"retry_attempts" env:"GOMIND_AI_RETRY_ATTEMPTS" default:"3"`
	RetryDelay    time.Duration `json:"retry_delay" env:"GOMIND_AI_RETRY_DELAY" default:"1s"`
}

// TelemetryConfig contains observability configuration for metrics and distributed tracing.
// This is an optional module - telemetry is only initialized when Enabled=true.
// Supports OpenTelemetry (OTEL) protocol. The endpoint should be the OTLP receiver address.
type TelemetryConfig struct {
	Enabled        bool    `json:"enabled" env:"GOMIND_TELEMETRY_ENABLED" default:"false"`
	Provider       string  `json:"provider" env:"GOMIND_TELEMETRY_PROVIDER" default:"otel"`
	Endpoint       string  `json:"endpoint" env:"GOMIND_TELEMETRY_ENDPOINT,OTEL_EXPORTER_OTLP_ENDPOINT"`
	ServiceName    string  `json:"service_name" env:"GOMIND_TELEMETRY_SERVICE_NAME,OTEL_SERVICE_NAME"`
	MetricsEnabled bool    `json:"metrics_enabled" env:"GOMIND_TELEMETRY_METRICS" default:"true"`
	TracingEnabled bool    `json:"tracing_enabled" env:"GOMIND_TELEMETRY_TRACING" default:"true"`
	SamplingRate   float64 `json:"sampling_rate" env:"GOMIND_TELEMETRY_SAMPLING_RATE" default:"1.0"`
	Insecure       bool    `json:"insecure" env:"GOMIND_TELEMETRY_INSECURE" default:"true"`
}

// MemoryConfig contains state storage configuration.
// Supports in-memory storage (default) or Redis for distributed state.
// The MaxSize limit only applies to in-memory storage.
type MemoryConfig struct {
	Provider        string        `json:"provider" env:"GOMIND_MEMORY_PROVIDER" default:"inmemory"`
	RedisURL        string        `json:"redis_url" env:"GOMIND_MEMORY_REDIS_URL,REDIS_URL"`
	MaxSize         int           `json:"max_size" env:"GOMIND_MEMORY_MAX_SIZE" default:"1000"`
	DefaultTTL      time.Duration `json:"default_ttl" env:"GOMIND_MEMORY_DEFAULT_TTL" default:"1h"`
	CleanupInterval time.Duration `json:"cleanup_interval" env:"GOMIND_MEMORY_CLEANUP_INTERVAL" default:"10m"`
}

// ResilienceConfig contains fault tolerance and resilience patterns configuration.
// These patterns help protect the system from cascading failures and improve reliability.
type ResilienceConfig struct {
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker"`
	Retry          RetryConfig          `json:"retry"`
	Timeout        TimeoutConfig        `json:"timeout"`
}

// CircuitBreakerConfig defines circuit breaker pattern settings.
// The circuit breaker prevents cascading failures by failing fast when a threshold
// of errors is reached. After a timeout period, it allows limited requests to test
// if the service has recovered.
type CircuitBreakerConfig struct {
	Enabled          bool          `json:"enabled" env:"GOMIND_CB_ENABLED" default:"false"`
	Threshold        int           `json:"threshold" env:"GOMIND_CB_THRESHOLD" default:"5"`
	Timeout          time.Duration `json:"timeout" env:"GOMIND_CB_TIMEOUT" default:"30s"`
	HalfOpenRequests int           `json:"half_open_requests" env:"GOMIND_CB_HALF_OPEN" default:"3"`
}

// RetryConfig defines retry pattern settings with exponential backoff.
// The retry interval increases exponentially up to MaxInterval.
// Formula: interval = min(InitialInterval * (Multiplier ^ attempt), MaxInterval)
type RetryConfig struct {
	MaxAttempts     int           `json:"max_attempts" env:"GOMIND_RETRY_MAX_ATTEMPTS" default:"3"`
	InitialInterval time.Duration `json:"initial_interval" env:"GOMIND_RETRY_INITIAL_INTERVAL" default:"1s"`
	MaxInterval     time.Duration `json:"max_interval" env:"GOMIND_RETRY_MAX_INTERVAL" default:"30s"`
	Multiplier      float64       `json:"multiplier" env:"GOMIND_RETRY_MULTIPLIER" default:"2.0"`
}

// TimeoutConfig defines timeout settings for various operations.
// These timeouts prevent operations from hanging indefinitely.
type TimeoutConfig struct {
	DefaultTimeout time.Duration `json:"default_timeout" env:"GOMIND_TIMEOUT_DEFAULT" default:"30s"`
	MaxTimeout     time.Duration `json:"max_timeout" env:"GOMIND_TIMEOUT_MAX" default:"5m"`
}

// LoggingConfig contains logging configuration.
// Supports structured (JSON) and human-readable (text) formats.
// In Kubernetes environments, JSON format is recommended for log aggregation.
type LoggingConfig struct {
	Level      string `json:"level" env:"GOMIND_LOG_LEVEL" default:"info"`
	Format     string `json:"format" env:"GOMIND_LOG_FORMAT" default:"json"`
	Output     string `json:"output" env:"GOMIND_LOG_OUTPUT" default:"stdout"`
	TimeFormat string `json:"time_format" env:"GOMIND_LOG_TIME_FORMAT" default:"2006-01-02T15:04:05.000Z07:00"`
}

// DevelopmentConfig contains settings for local development and testing.
// When Enabled=true, the framework uses development-friendly defaults:
// human-readable logs, mock services, and debug logging.
//
// WARNING: Never enable development mode in production!
type DevelopmentConfig struct {
	Enabled       bool `json:"enabled" env:"GOMIND_DEV_MODE" default:"false"`
	MockAI        bool `json:"mock_ai" env:"GOMIND_MOCK_AI" default:"false"`
	MockDiscovery bool `json:"mock_discovery" env:"GOMIND_MOCK_DISCOVERY" default:"false"`
	DebugLogging  bool `json:"debug_logging" env:"GOMIND_DEBUG" default:"false"`
	PrettyLogs    bool `json:"pretty_logs" env:"GOMIND_PRETTY_LOGS" default:"false"`
}

// KubernetesConfig contains Kubernetes-specific settings.
// The framework automatically detects Kubernetes environments by checking
// for the KUBERNETES_SERVICE_HOST environment variable.
// When running in Kubernetes, the framework adjusts defaults for
// containerized environments (e.g., binding to 0.0.0.0, JSON logging).
type KubernetesConfig struct {
	Enabled                bool   `json:"enabled" env:"KUBERNETES_SERVICE_HOST"`
	ServiceName            string `json:"service_name" env:"GOMIND_K8S_SERVICE_NAME"`
	ServicePort            int    `json:"service_port" env:"GOMIND_K8S_SERVICE_PORT" default:"80"`
	PodName                string `json:"pod_name" env:"HOSTNAME"`
	PodNamespace           string `json:"pod_namespace" env:"GOMIND_K8S_NAMESPACE"`
	PodIP                  string `json:"pod_ip" env:"GOMIND_K8S_POD_IP"`
	NodeName               string `json:"node_name" env:"GOMIND_K8S_NODE_NAME"`
	ServiceAccountPath     string `json:"service_account_path" env:"GOMIND_K8S_SA_PATH" default:"/var/run/secrets/kubernetes.io/serviceaccount"`
	EnableServiceDiscovery bool   `json:"enable_service_discovery" env:"GOMIND_K8S_SERVICE_DISCOVERY" default:"true"`
	EnableLeaderElection   bool   `json:"enable_leader_election" env:"GOMIND_K8S_LEADER_ELECTION" default:"false"`
}

// Option is a functional option for configuring the framework.
// Options are applied in order and can return an error if the configuration is invalid.
//
// Example:
//
//	func WithCustomTimeout(timeout time.Duration) Option {
//	    return func(c *Config) error {
//	        if timeout <= 0 {
//	            return fmt.Errorf("timeout must be positive")
//	        }
//	        c.HTTP.ReadTimeout = timeout
//	        return nil
//	    }
//	}
type Option func(*Config) error

// DefaultConfig returns a configuration with sensible defaults.
// The defaults are adjusted based on the detected environment:
//   - Kubernetes: 0.0.0.0 binding, JSON logging, discovery enabled
//   - Local: localhost binding, text logging, development mode
//
// These defaults can be overridden using functional options or environment variables.
func DefaultConfig() *Config {
	cfg := &Config{
		Name:      "gomind-agent",
		Port:      8080,
		Address:   "", // Will be set based on environment detection
		Namespace: "default",
		HTTP: HTTPConfig{
			ReadTimeout:       30 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
			MaxHeaderBytes:    1 << 20, // 1MB
			ShutdownTimeout:   10 * time.Second,
			EnableHealthCheck: true,
			HealthCheckPath:   "/health",
			CORS: CORSConfig{
				Enabled:          false,
				AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders:   []string{"Content-Type", "Authorization"},
				AllowCredentials: false,
				MaxAge:           86400,
			},
		},
		Discovery: DiscoveryConfig{
			Enabled:           false, // Disabled by default for local development
			Provider:          "redis",
			CacheEnabled:      true,
			CacheTTL:          5 * time.Minute,
			HeartbeatInterval: 10 * time.Second,
			TTL:               30 * time.Second,
		},
		AI: AIConfig{
			Enabled:       false,
			Provider:      "openai",
			Model:         "gpt-4",
			Temperature:   0.7,
			MaxTokens:     2000,
			Timeout:       30 * time.Second,
			RetryAttempts: 3,
			RetryDelay:    1 * time.Second,
		},
		Telemetry: TelemetryConfig{
			Enabled:        false,
			Provider:       "otel",
			MetricsEnabled: true,
			TracingEnabled: true,
			SamplingRate:   1.0,
			Insecure:       true,
		},
		Memory: MemoryConfig{
			Provider:        "inmemory",
			MaxSize:         1000,
			DefaultTTL:      1 * time.Hour,
			CleanupInterval: 10 * time.Minute,
		},
		Resilience: ResilienceConfig{
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:          false,
				Threshold:        5,
				Timeout:          30 * time.Second,
				HalfOpenRequests: 3,
			},
			Retry: RetryConfig{
				MaxAttempts:     3,
				InitialInterval: 1 * time.Second,
				MaxInterval:     30 * time.Second,
				Multiplier:      2.0,
			},
			Timeout: TimeoutConfig{
				DefaultTimeout: 30 * time.Second,
				MaxTimeout:     5 * time.Minute,
			},
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			TimeFormat: time.RFC3339Nano,
		},
		Development: DevelopmentConfig{
			Enabled:       false,
			MockAI:        false,
			MockDiscovery: false,
			DebugLogging:  false,
			PrettyLogs:    false,
		},
		Kubernetes: KubernetesConfig{
			ServicePort:            80,
			ServiceAccountPath:     "/var/run/secrets/kubernetes.io/serviceaccount",
			EnableServiceDiscovery: true,
			EnableLeaderElection:   false,
		},
	}

	// Detect environment and adjust defaults
	cfg.DetectEnvironment()

	return cfg
}

// DetectEnvironment automatically adjusts configuration based on the detected environment.
// This method is called automatically by DefaultConfig() and should not be called directly
// unless you're implementing custom environment detection logic.
//
// Detection criteria:
//   - Kubernetes: KUBERNETES_SERVICE_HOST environment variable is set
//   - Local: No Kubernetes environment variables detected
func (c *Config) DetectEnvironment() {
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		// Kubernetes environment detected
		c.Kubernetes.Enabled = true
		c.Address = "0.0.0.0"      // Bind to all interfaces in K8s
		c.Discovery.Enabled = true // Enable discovery in K8s
		c.Discovery.RedisURL = "redis://redis.default.svc.cluster.local:6379"
		c.Logging.Format = "json" // Structured logs for K8s
	} else {
		// Local development environment
		c.Address = "localhost"
		c.Discovery.RedisURL = "redis://localhost:6379"

		// Enable development mode for local
		if os.Getenv("GOMIND_DEV_MODE") == "" {
			c.Development.Enabled = true
			c.Development.PrettyLogs = true
			c.Logging.Format = "text" // Human-readable logs
		}
	}
}

// LoadFromEnv loads configuration from environment variables and validates the result.
// Environment variables take precedence over defaults but are overridden by functional options.
//
// Variable naming convention:
//   - Framework-specific: GOMIND_<SETTING>
//   - Standard variables: REDIS_URL, OPENAI_API_KEY, OTEL_EXPORTER_OTLP_ENDPOINT
//
// Returns an error if environment variables contain invalid values or if validation fails.
func (c *Config) LoadFromEnv() error {
	if c.logger != nil {
		c.logger.Info("Loading configuration from environment", map[string]interface{}{
			"config_source": "environment_variables",
		})
	}

	envVarsLoaded := 0

	// Core settings
	if v := os.Getenv("GOMIND_AGENT_NAME"); v != "" {
		c.Name = v
		envVarsLoaded++
		if c.logger != nil {
			c.logger.Debug("Configuration loaded", map[string]interface{}{
				"setting": "agent_name",
				"source":  "GOMIND_AGENT_NAME",
				"set":     true,
			})
		}
	}
	if v := os.Getenv("GOMIND_AGENT_ID"); v != "" {
		c.ID = v
		envVarsLoaded++
		if c.logger != nil {
			c.logger.Debug("Configuration loaded", map[string]interface{}{
				"setting": "agent_id",
				"source":  "GOMIND_AGENT_ID",
				"set":     true,
			})
		}
	}
	if v := os.Getenv("GOMIND_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Port = port
			envVarsLoaded++
			if c.logger != nil {
				c.logger.Debug("Configuration loaded", map[string]interface{}{
					"setting": "port",
					"source":  "GOMIND_PORT",
					"set":     true,
				})
			}
		} else if c.logger != nil {
			c.logger.Warn("Invalid port in environment variable", map[string]interface{}{
				"GOMIND_PORT": v,
				"error":       err,
				"error_type":  fmt.Sprintf("%T", err),
			})
		}
	}
	if v := os.Getenv("GOMIND_ADDRESS"); v != "" {
		c.Address = v
		envVarsLoaded++
		if c.logger != nil {
			c.logger.Debug("Configuration loaded", map[string]interface{}{
				"setting": "address",
				"source":  "GOMIND_ADDRESS",
				"set":     true,
			})
		}
	}
	if v := os.Getenv("GOMIND_NAMESPACE"); v != "" {
		c.Namespace = v
		envVarsLoaded++
		if c.logger != nil {
			c.logger.Debug("Configuration loaded", map[string]interface{}{
				"setting": "namespace",
				"source":  "GOMIND_NAMESPACE",
				"set":     true,
			})
		}
	}

	// HTTP settings
	if v := os.Getenv("GOMIND_HTTP_READ_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.HTTP.ReadTimeout = d
		}
	}
	if v := os.Getenv("GOMIND_HTTP_WRITE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.HTTP.WriteTimeout = d
		}
	}

	// CORS settings
	if v := os.Getenv("GOMIND_CORS_ENABLED"); v != "" {
		c.HTTP.CORS.Enabled = parseBool(v)
	}
	if v := os.Getenv("GOMIND_CORS_ORIGINS"); v != "" {
		c.HTTP.CORS.AllowedOrigins = parseStringList(v)
	}
	if v := os.Getenv("GOMIND_CORS_METHODS"); v != "" {
		c.HTTP.CORS.AllowedMethods = parseStringList(v)
	}
	if v := os.Getenv("GOMIND_CORS_HEADERS"); v != "" {
		c.HTTP.CORS.AllowedHeaders = parseStringList(v)
	}
	if v := os.Getenv("GOMIND_CORS_CREDENTIALS"); v != "" {
		c.HTTP.CORS.AllowCredentials = parseBool(v)
	}

	// Discovery settings
	if v := os.Getenv("GOMIND_DISCOVERY_ENABLED"); v != "" {
		c.Discovery.Enabled = parseBool(v)
	}
	if v := os.Getenv("GOMIND_DISCOVERY_PROVIDER"); v != "" {
		c.Discovery.Provider = v
	}
	if v := os.Getenv("GOMIND_REDIS_URL"); v != "" {
		c.Discovery.RedisURL = v
		c.Memory.RedisURL = v // Also use for memory if not separately configured
		envVarsLoaded++
		if c.logger != nil {
			c.logger.Debug("Configuration loaded", map[string]interface{}{
				"setting": "redis_url",
				"source":  "GOMIND_REDIS_URL",
				"set":     true,
			})
		}
	} else if v := os.Getenv("REDIS_URL"); v != "" {
		c.Discovery.RedisURL = v
		c.Memory.RedisURL = v
		envVarsLoaded++
		if c.logger != nil {
			c.logger.Debug("Configuration loaded", map[string]interface{}{
				"setting": "redis_url",
				"source":  "REDIS_URL",
				"set":     true,
			})
		}
	}
	if v := os.Getenv("GOMIND_DISCOVERY_CACHE"); v != "" {
		c.Discovery.CacheEnabled = parseBool(v)
	}

	// AI settings
	if v := os.Getenv("GOMIND_AI_ENABLED"); v != "" {
		c.AI.Enabled = parseBool(v)
	}
	if v := os.Getenv("GOMIND_AI_API_KEY"); v != "" {
		c.AI.APIKey = v
		c.AI.Enabled = true // Auto-enable if API key is provided
		envVarsLoaded++
		if c.logger != nil {
			c.logger.Debug("Configuration loaded", map[string]interface{}{
				"setting": "ai_api_key",
				"source":  "GOMIND_AI_API_KEY",
				"set":     true,
			})
		}
	} else if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		c.AI.APIKey = v
		c.AI.Enabled = true // Auto-enable if OpenAI key is present
		envVarsLoaded++
		if c.logger != nil {
			c.logger.Debug("Configuration loaded", map[string]interface{}{
				"setting": "ai_api_key",
				"source":  "OPENAI_API_KEY",
				"set":     true,
			})
		}
	}
	if v := os.Getenv("GOMIND_AI_MODEL"); v != "" {
		c.AI.Model = v
	}
	if v := os.Getenv("GOMIND_AI_BASE_URL"); v != "" {
		c.AI.BaseURL = v
	}

	// Telemetry settings
	if v := os.Getenv("GOMIND_TELEMETRY_ENABLED"); v != "" {
		c.Telemetry.Enabled = parseBool(v)
	}
	if v := os.Getenv("GOMIND_TELEMETRY_ENDPOINT"); v != "" {
		c.Telemetry.Endpoint = v
		c.Telemetry.Enabled = true // Auto-enable if endpoint is provided
		envVarsLoaded++
		if c.logger != nil {
			c.logger.Debug("Configuration loaded", map[string]interface{}{
				"setting": "telemetry_endpoint",
				"source":  "GOMIND_TELEMETRY_ENDPOINT",
				"set":     true,
			})
		}
	} else if v := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); v != "" {
		c.Telemetry.Endpoint = v
		c.Telemetry.Enabled = true // Auto-enable if OTEL endpoint is present
		envVarsLoaded++
		if c.logger != nil {
			c.logger.Debug("Configuration loaded", map[string]interface{}{
				"setting": "telemetry_endpoint",
				"source":  "OTEL_EXPORTER_OTLP_ENDPOINT",
				"set":     true,
			})
		}
	}
	if v := os.Getenv("GOMIND_TELEMETRY_SERVICE_NAME"); v != "" {
		c.Telemetry.ServiceName = v
	} else if v := os.Getenv("OTEL_SERVICE_NAME"); v != "" {
		c.Telemetry.ServiceName = v
	} else if c.Telemetry.ServiceName == "" {
		c.Telemetry.ServiceName = c.Name // Default to agent name
	}

	// Memory settings
	if v := os.Getenv("GOMIND_MEMORY_PROVIDER"); v != "" {
		c.Memory.Provider = v
	}
	if v := os.Getenv("GOMIND_MEMORY_REDIS_URL"); v != "" {
		c.Memory.RedisURL = v
	}

	// Logging settings
	if v := os.Getenv("GOMIND_LOG_LEVEL"); v != "" {
		c.Logging.Level = v
	}
	if v := os.Getenv("GOMIND_LOG_FORMAT"); v != "" {
		c.Logging.Format = v
	}

	// Development settings
	if v := os.Getenv("GOMIND_DEV_MODE"); v != "" {
		c.Development.Enabled = parseBool(v)
		if c.Development.Enabled {
			c.Development.PrettyLogs = true
			c.Logging.Level = "debug"
			c.Logging.Format = "text"
		}
	}
	if v := os.Getenv("GOMIND_MOCK_AI"); v != "" {
		c.Development.MockAI = parseBool(v)
	}
	if v := os.Getenv("GOMIND_MOCK_DISCOVERY"); v != "" {
		c.Development.MockDiscovery = parseBool(v)
	}
	if v := os.Getenv("GOMIND_DEBUG"); v != "" {
		c.Development.DebugLogging = parseBool(v)
		if c.Development.DebugLogging {
			c.Logging.Level = "debug"
		}
	}

	// Kubernetes settings (auto-detect)
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		c.Kubernetes.Enabled = true
		if v := os.Getenv("HOSTNAME"); v != "" {
			c.Kubernetes.PodName = v
		}
		if v := os.Getenv("GOMIND_K8S_NAMESPACE"); v != "" {
			c.Kubernetes.PodNamespace = v
		}
		// Try to read namespace from service account
		if c.Kubernetes.PodNamespace == "" {
			if data, err := os.ReadFile(c.Kubernetes.ServiceAccountPath + "/namespace"); err == nil {
				c.Kubernetes.PodNamespace = strings.TrimSpace(string(data))
			}
		}
		if v := os.Getenv("GOMIND_K8S_SERVICE_NAME"); v != "" {
			c.Kubernetes.ServiceName = v
		}
		if v := os.Getenv("GOMIND_K8S_SERVICE_PORT"); v != "" {
			if port, err := strconv.Atoi(v); err == nil && port > 0 && port <= 65535 {
				c.Kubernetes.ServicePort = port
			}
		}
		if v := os.Getenv("GOMIND_K8S_POD_IP"); v != "" {
			c.Kubernetes.PodIP = v
		}
		if v := os.Getenv("GOMIND_K8S_NODE_NAME"); v != "" {
			c.Kubernetes.NodeName = v
		}
	}

	if err := c.Validate(); err != nil {
		if c.logger != nil {
			c.logger.Error("Configuration validation failed", map[string]interface{}{
				"error":         err.Error(),
				"error_type":    fmt.Sprintf("%T", err),
				"config_source": "environment_variables",
			})
		}
		return err
	}

	// Note: LOGGING_SOLUTION_ANALYSIS.md:1434 references "EnableDevelopmentMode" but actual field is "Enabled"
	if c.logger != nil {
		c.logger.Info("Configuration loading completed", map[string]interface{}{
			"discovery_enabled": c.Discovery.Enabled,
			"logging_level":     c.Logging.Level,
			"namespace":         c.Namespace,
			"development_mode":  c.Development.Enabled,
		})
	}

	return nil
}

// LoadFromFile loads configuration from a JSON file.
// The file should contain a JSON object matching the Config struct.
// File settings override environment variables but are overridden by functional options.
//
// Example JSON:
//
//	{
//	    "name": "my-agent",
//	    "port": 8080,
//	    "http": {
//	        "cors": {
//	            "enabled": true,
//	            "allowed_origins": ["https://example.com"]
//	        }
//	    }
//	}
func (c *Config) LoadFromFile(path string) error {
	if c.logger != nil {
		c.logger.Info("Loading configuration from file", map[string]interface{}{
			"file_path": path,
		})
	}

	// Clean the path to prevent directory traversal attacks
	cleanPath := filepath.Clean(path)

	// Verify the file has a safe extension
	ext := filepath.Ext(cleanPath)
	if ext != ".json" && ext != ".yaml" && ext != ".yml" {
		if c.logger != nil {
			c.logger.Error("Unsupported config file extension", map[string]interface{}{
				"file_path":         path,
				"clean_path":        cleanPath,
				"extension":         ext,
				"supported_formats": []string{".json", ".yaml", ".yml"},
			})
		}
		return fmt.Errorf("unsupported config file extension %s: %w", ext, ErrInvalidConfiguration)
	}

	// Check if the path is absolute and within expected directories
	if !filepath.IsAbs(cleanPath) {
		// If relative, resolve it relative to current directory
		wd, err := os.Getwd()
		if err != nil {
			if c.logger != nil {
				c.logger.Error("Failed to get working directory for relative config path", map[string]interface{}{
					"error":      err,
					"error_type": fmt.Sprintf("%T", err),
					"clean_path": cleanPath,
				})
			}
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		cleanPath = filepath.Join(wd, cleanPath)
		
		if c.logger != nil {
			c.logger.Debug("Resolved relative config path", map[string]interface{}{
				"original_path": path,
				"resolved_path": cleanPath,
				"working_dir":   wd,
			})
		}
	}

	if c.logger != nil {
		c.logger.Debug("Reading configuration file", map[string]interface{}{
			"file_path": cleanPath,
			"extension": ext,
		})
	}

	// Read the file with the cleaned path
	data, err := os.ReadFile(filepath.Clean(cleanPath)) // nosec G304 -- path is validated
	if err != nil {
		if c.logger != nil {
			c.logger.Error("Failed to read config file", map[string]interface{}{
				"error":      err,
				"error_type": fmt.Sprintf("%T", err),
				"file_path":  cleanPath,
			})
		}
		return fmt.Errorf("failed to read config file %s: %w", cleanPath, err)
	}

	if c.logger != nil {
		c.logger.Debug("Config file read successfully", map[string]interface{}{
			"file_path": cleanPath,
			"file_size": len(data),
		})
	}

	// Parse based on extension
	switch ext {
	case ".json":
		if c.logger != nil {
			c.logger.Debug("Parsing JSON configuration file", map[string]interface{}{
				"file_path": cleanPath,
			})
		}
		
		if err := json.Unmarshal(data, c); err != nil {
			if c.logger != nil {
				c.logger.Error("Failed to parse JSON config file", map[string]interface{}{
					"error":      err,
					"error_type": fmt.Sprintf("%T", err),
					"file_path":  cleanPath,
					"file_size":  len(data),
				})
			}
			return fmt.Errorf("failed to parse JSON config file: %w", ErrInvalidConfiguration)
		}
		
		if c.logger != nil {
			c.logger.Info("Configuration file loaded successfully", map[string]interface{}{
				"file_path": cleanPath,
				"format":    "JSON",
				"file_size": len(data),
			})
		}
		
	case ".yaml", ".yml":
		if c.logger != nil {
			c.logger.Error("YAML configuration files not supported", map[string]interface{}{
				"file_path":         cleanPath,
				"extension":         ext,
				"supported_formats": []string{".json"},
			})
		}
		// For YAML support, we'd need to import gopkg.in/yaml.v3
		// For now, return an error for YAML files
		return fmt.Errorf("YAML config files not yet supported: %w", ErrInvalidConfiguration)
	}

	return nil
}

// Validate checks if the configuration is valid and returns an error if not.
// This method is called automatically by NewConfig() but can also be called
// manually after modifying configuration.
//
// Validation rules:
//   - Port must be between 1 and 65535
//   - Agent name is required
//   - AI API key is required when AI is enabled (unless using mock)
//   - Telemetry endpoint is required when telemetry is enabled
//   - Redis URL is required when Redis discovery is enabled (unless using mock)
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		// Preserve exact message for test compatibility
		return &FrameworkError{
			Op:      "Config.Validate",
			Kind:    "config",
			Message: fmt.Sprintf("invalid port: %d", c.Port),
			Err:     ErrInvalidConfiguration,
		}
	}

	if c.Name == "" {
		// Preserve exact message for test compatibility
		return &FrameworkError{
			Op:      "Config.Validate",
			Kind:    "config",
			Message: "agent name is required",
			Err:     ErrMissingConfiguration,
		}
	}

	if c.AI.Enabled && c.AI.APIKey == "" && !c.Development.MockAI {
		// Preserve exact message for test compatibility
		return &FrameworkError{
			Op:      "Config.Validate",
			Kind:    "config",
			Message: "AI API key is required when AI is enabled (or use mock AI in development)",
			Err:     ErrMissingConfiguration,
		}
	}

	if c.Telemetry.Enabled && c.Telemetry.Endpoint == "" {
		// Preserve exact message for test compatibility
		return &FrameworkError{
			Op:      "Config.Validate",
			Kind:    "config",
			Message: "telemetry endpoint is required when telemetry is enabled",
			Err:     ErrMissingConfiguration,
		}
	}

	if c.Discovery.Enabled && c.Discovery.Provider == "redis" && c.Discovery.RedisURL == "" && !c.Development.MockDiscovery {
		// Preserve exact message for test compatibility
		return &FrameworkError{
			Op:      "Config.Validate",
			Kind:    "config",
			Message: "redis URL is required for Redis discovery provider (or use mock discovery in development)",
			Err:     ErrMissingConfiguration,
		}
	}

	return nil
}

// Helper functions

// parseStringList splits a comma-separated string into a slice of strings.
// Whitespace is trimmed from each element, and empty strings are filtered out.
// Example: "a, b, c" -> ["a", "b", "c"]
func parseStringList(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// parseBool converts a string to a boolean value.
// Accepts: "true", "1", "yes", "on" (case-insensitive) as true.
// Everything else is false.
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes" || s == "on"
}

// Functional Options

// WithName sets the agent name.
// The name is used for identification in service discovery and logging.
// If not set, defaults to "gomind-agent".
func WithName(name string) Option {
	return func(c *Config) error {
		c.Name = name
		return nil
	}
}

// WithPort sets the HTTP server port.
// Must be between 1 and 65535.
// Returns an error if the port is invalid.
func WithPort(port int) Option {
	return func(c *Config) error {
		if port < 1 || port > 65535 {
			// Preserve exact message for test compatibility
			return &FrameworkError{
				Op:      "WithPort",
				Kind:    "config",
				Message: fmt.Sprintf("invalid port: %d", port),
				Err:     ErrInvalidConfiguration,
			}
		}
		c.Port = port
		return nil
	}
}

// WithAddress sets the bind address for the HTTP server.
// Common values:
//   - "localhost" or "127.0.0.1" for local only
//   - "0.0.0.0" for all interfaces (required in containers)
//   - Specific IP for multi-homed hosts
func WithAddress(address string) Option {
	return func(c *Config) error {
		c.Address = address
		return nil
	}
}

// WithNamespace sets the logical namespace for the agent.
// Used for multi-tenancy and environment separation (e.g., "production", "staging").
// This is a logical grouping, not a Kubernetes namespace.
func WithNamespace(namespace string) Option {
	return func(c *Config) error {
		c.Namespace = namespace
		return nil
	}
}

// WithCORS enables CORS with specific allowed origins.
// Supports wildcard patterns:
//   - "*" allows all origins (not recommended for production)
//   - "*.example.com" allows all subdomains
//   - "http://localhost:*" allows any localhost port
//
// The credentials parameter controls whether cookies and auth headers are allowed.
// Be cautious when enabling credentials with wildcard origins.
func WithCORS(origins []string, credentials bool) Option {
	return func(c *Config) error {
		c.HTTP.CORS.Enabled = true
		c.HTTP.CORS.AllowedOrigins = origins
		c.HTTP.CORS.AllowCredentials = credentials
		return nil
	}
}

// WithCORSDefaults enables CORS with permissive defaults.
// Allows all origins, methods, and headers with credentials.
//
// WARNING: This is intended for development only!
// Never use this in production as it bypasses CORS security.
func WithCORSDefaults() Option {
	return func(c *Config) error {
		c.HTTP.CORS.Enabled = true
		c.HTTP.CORS.AllowedOrigins = []string{"*"}
		c.HTTP.CORS.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
		c.HTTP.CORS.AllowedHeaders = []string{"*"}
		c.HTTP.CORS.AllowCredentials = true
		return nil
	}
}

// WithRedisURL sets the Redis connection URL for both discovery and memory storage.
// Format: redis://[user:password@]host:port/db
// Examples:
//   - redis://localhost:6379
//   - redis://user:pass@redis.example.com:6379/0
//   - redis://redis.default.svc.cluster.local:6379
//
// This automatically enables discovery when set.
func WithRedisURL(url string) Option {
	return func(c *Config) error {
		c.Discovery.RedisURL = url
		c.Memory.RedisURL = url
		c.Discovery.Enabled = true // Auto-enable discovery when Redis is configured
		return nil
	}
}

// WithDiscovery enables or disables service discovery with the specified provider.
// Currently supported providers:
//   - "redis": Redis-based discovery (auto-configures RedisURL from environment or defaults to localhost)
//   - "mock": In-memory mock for testing
//
// When disabled, the agent runs in standalone mode without discovery.
func WithDiscovery(enabled bool, provider string) Option {
	return func(c *Config) error {
		c.Discovery.Enabled = enabled
		c.Discovery.Provider = provider
		
		// Auto-configure Redis URL for Redis provider
		if enabled && provider == "redis" {
			// Check if RedisURL was explicitly set by user configuration
			// We can distinguish this from LoadFromEnv by checking if it's not one of the common env values
			currentURL := c.Discovery.RedisURL
			wasExplicitlySet := currentURL != "" && 
				currentURL != os.Getenv("REDIS_URL") && 
				currentURL != os.Getenv("GOMIND_REDIS_URL")
			
			if !wasExplicitlySet {
				// Apply proper precedence: REDIS_URL takes precedence over GOMIND_REDIS_URL
				redisURL := os.Getenv("REDIS_URL")
				if redisURL != "" {
					c.Discovery.RedisURL = redisURL
				} else if gomindRedisURL := os.Getenv("GOMIND_REDIS_URL"); gomindRedisURL != "" {
					c.Discovery.RedisURL = gomindRedisURL
				} else if currentURL == "" {
					// Use sensible default for development only if no URL was set
					c.Discovery.RedisURL = "redis://localhost:6379"
				}
			}
		} else if !enabled || provider != "redis" {
			// Clear RedisURL if discovery is disabled or non-Redis provider
			c.Discovery.RedisURL = ""
		}
		return nil
	}
}

// WithRedisDiscovery is a convenience function that configures Redis-based discovery
// with the specified Redis URL. This is equivalent to calling:
//   WithDiscovery(true, "redis") + WithRedisURL(redisURL)
// but more explicit and convenient for Redis-specific setups.
func WithRedisDiscovery(redisURL string) Option {
	return func(c *Config) error {
		c.Discovery.Enabled = true
		c.Discovery.Provider = "redis"
		c.Discovery.RedisURL = redisURL
		c.Memory.RedisURL = redisURL // Also use for memory if not already set
		return nil
	}
}

// WithDiscoveryCacheEnabled enables or disables discovery result caching.
// When enabled, discovery results are cached for CacheTTL duration to reduce
// load on the discovery backend. Recommended for production.
func WithDiscoveryCacheEnabled(enabled bool) Option {
	return func(c *Config) error {
		c.Discovery.CacheEnabled = enabled
		return nil
	}
}

// WithOpenAIAPIKey sets the OpenAI API key and automatically enables AI features.
// The key should be a valid OpenAI API key starting with "sk-".
// This is a convenience method equivalent to:
//
//	WithAI(true, "openai", key)
//
// For security, prefer loading the key from environment variables or secrets.
func WithOpenAIAPIKey(key string) Option {
	return func(c *Config) error {
		c.AI.Enabled = true
		c.AI.Provider = "openai"
		c.AI.APIKey = key
		return nil
	}
}

// WithAI configures AI client settings.
// Parameters:
//   - enabled: Whether to initialize AI features
//   - provider: AI provider ("openai", "anthropic", "mock")
//   - apiKey: API key for the provider (not needed for "mock")
//
// When enabled=false, AI features are completely disabled regardless of other settings.
func WithAI(enabled bool, provider, apiKey string) Option {
	return func(c *Config) error {
		c.AI.Enabled = enabled
		c.AI.Provider = provider
		c.AI.APIKey = apiKey
		return nil
	}
}

// WithAIModel sets the AI model to use.
// Common values:
//   - OpenAI: "gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"
//   - Anthropic: "claude-3-5-sonnet-20241022", "claude-3-opus-20240229"
//
// Check your provider's documentation for available models.
func WithAIModel(model string) Option {
	return func(c *Config) error {
		c.AI.Model = model
		return nil
	}
}

// WithTelemetry enables telemetry with the specified endpoint.
// The endpoint should be an OpenTelemetry Protocol (OTLP) receiver.
// Examples:
//   - "http://localhost:4317" (local Jaeger)
//   - "http://otel-collector:4317" (Kubernetes)
//   - "https://otel.example.com:443" (cloud provider)
//
// When enabled, both metrics and tracing are collected by default.
func WithTelemetry(enabled bool, endpoint string) Option {
	return func(c *Config) error {
		c.Telemetry.Enabled = enabled
		c.Telemetry.Endpoint = endpoint
		if c.Telemetry.ServiceName == "" {
			c.Telemetry.ServiceName = c.Name
		}
		return nil
	}
}

// WithEnableMetrics enables or disables metrics collection.
// Metrics include request counts, latencies, error rates, etc.
// Requires telemetry to be enabled with an endpoint.
// Metrics are exported via OpenTelemetry protocol.
func WithEnableMetrics(enabled bool) Option {
	return func(c *Config) error {
		c.Telemetry.MetricsEnabled = enabled
		if enabled && c.Telemetry.Endpoint != "" {
			c.Telemetry.Enabled = true
		}
		return nil
	}
}

// WithEnableTracing enables or disables distributed tracing.
// Tracing provides detailed request flow across services.
// Requires telemetry to be enabled with an endpoint.
// Traces are exported via OpenTelemetry protocol.
func WithEnableTracing(enabled bool) Option {
	return func(c *Config) error {
		c.Telemetry.TracingEnabled = enabled
		if enabled && c.Telemetry.Endpoint != "" {
			c.Telemetry.Enabled = true
		}
		return nil
	}
}

// WithOTELEndpoint sets the OpenTelemetry endpoint and automatically enables telemetry.
// This is a convenience method equivalent to:
//
//	WithTelemetry(true, endpoint)
//
// The endpoint should be an OTLP receiver address.
func WithOTELEndpoint(endpoint string) Option {
	return func(c *Config) error {
		c.Telemetry.Enabled = true
		c.Telemetry.Provider = "otel"
		c.Telemetry.Endpoint = endpoint
		return nil
	}
}

// WithLogLevel sets the minimum logging level.
// Valid levels (from least to most verbose):
//   - "error": Only errors
//   - "warn": Warnings and above
//   - "info": Informational messages and above (default)
//   - "debug": Debug messages and above
//
// Debug level should not be used in production due to performance impact.
func WithLogLevel(level string) Option {
	return func(c *Config) error {
		c.Logging.Level = level
		return nil
	}
}

// WithLogFormat sets the logging output format.
// Valid formats:
//   - "json": Structured JSON for log aggregation (recommended for production)
//   - "text": Human-readable format (recommended for development)
//
// JSON format is automatically selected in Kubernetes environments.
func WithLogFormat(format string) Option {
	return func(c *Config) error {
		c.Logging.Format = format
		return nil
	}
}

// WithMemoryProvider sets the state storage provider.
// Valid providers:
//   - "inmemory": Local in-memory storage (default, not distributed)
//   - "redis": Redis-based storage (requires WithRedisURL)
//
// Use Redis for distributed state across multiple agent instances.
func WithMemoryProvider(provider string) Option {
	return func(c *Config) error {
		c.Memory.Provider = provider
		return nil
	}
}

// WithCircuitBreaker enables the circuit breaker pattern for fault tolerance.
// Parameters:
//   - threshold: Number of consecutive failures before opening the circuit
//   - timeout: Duration to wait before attempting to close the circuit
//
// The circuit breaker prevents cascading failures by failing fast when
// a service is unhealthy, giving it time to recover.
func WithCircuitBreaker(threshold int, timeout time.Duration) Option {
	return func(c *Config) error {
		c.Resilience.CircuitBreaker.Enabled = true
		c.Resilience.CircuitBreaker.Threshold = threshold
		c.Resilience.CircuitBreaker.Timeout = timeout
		return nil
	}
}

// WithRetry configures automatic retry with exponential backoff.
// Parameters:
//   - maxAttempts: Maximum number of retry attempts (including initial)
//   - initialInterval: Initial delay between retries
//
// The retry interval doubles after each failure up to MaxInterval.
// Use this for transient failures like network issues.
func WithRetry(maxAttempts int, initialInterval time.Duration) Option {
	return func(c *Config) error {
		c.Resilience.Retry.MaxAttempts = maxAttempts
		c.Resilience.Retry.InitialInterval = initialInterval
		return nil
	}
}

// WithKubernetes enables Kubernetes-specific features.
// Parameters:
//   - serviceDiscovery: Use Kubernetes service discovery instead of Redis
//   - leaderElection: Enable leader election for singleton patterns
//
// These features require proper RBAC permissions in the cluster.
// The framework automatically detects Kubernetes environments, so this
// is only needed to enable specific features.
func WithKubernetes(serviceDiscovery, leaderElection bool) Option {
	return func(c *Config) error {
		c.Kubernetes.EnableServiceDiscovery = serviceDiscovery
		c.Kubernetes.EnableLeaderElection = leaderElection
		return nil
	}
}

// WithConfigFile loads configuration from a JSON file.
// The file path can be absolute or relative to the working directory.
// File configuration is applied before other options, so options
// can override file settings.
//
// This is useful for complex configurations or environment-specific settings.
func WithConfigFile(path string) Option {
	return func(c *Config) error {
		return c.LoadFromFile(path)
	}
}

// WithDevelopmentMode enables development mode with developer-friendly defaults.
// When enabled:
//   - Pretty (human-readable) logs
//   - Debug log level
//   - Text log format
//   - Relaxed validation
//
// WARNING: Never enable in production! This mode sacrifices
// performance and security for developer convenience.
func WithDevelopmentMode(enabled bool) Option {
	return func(c *Config) error {
		c.Development.Enabled = enabled
		if enabled {
			c.Development.PrettyLogs = true
			c.Logging.Format = "text"
			c.Logging.Level = "debug"
		}
		return nil
	}
}

// WithMockAI enables mock AI responses for testing without API calls.
// When enabled, the AI client returns predetermined responses instead
// of making actual API calls. Useful for:
//   - Unit testing
//   - Development without API keys
//   - Cost savings during development
//
// Mock responses are deterministic but not intelligent.
func WithMockAI(enabled bool) Option {
	return func(c *Config) error {
		c.Development.MockAI = enabled
		if enabled {
			c.AI.Enabled = true // Enable AI with mock provider
		}
		return nil
	}
}

// WithMockDiscovery enables in-memory mock discovery for testing.
// When enabled, service discovery uses local memory instead of Redis.
// Useful for:
//   - Unit testing
//   - Local development without Redis
//   - Isolated testing environments
//
// Note: Mock discovery is not distributed across instances.
func WithMockDiscovery(enabled bool) Option {
	return func(c *Config) error {
		c.Development.MockDiscovery = enabled
		if enabled {
			c.Discovery.Enabled = true // Enable discovery with mock provider
		}
		return nil
	}
}

// WithLogger sets a logger for configuration operations.
// This logger will be used for logging during config loading, parsing, and validation.
// If not set, configuration operations will be performed silently.
//
// Example:
//
//	cfg, err := NewConfig(
//	    WithLogger(myLogger),
//	    WithName("my-agent"),
//	)
func WithLogger(logger Logger) Option {
	return func(c *Config) error {
		c.logger = logger
		return nil
	}
}

// NewConfig creates a new configuration with the provided options.
// Configuration is applied in the following order:
//  1. Default values from DefaultConfig()
//  2. Environment variables via LoadFromEnv()
//  3. Functional options (highest priority)
//  4. Validation via Validate()
//
// Returns an error if any option fails or if the final configuration is invalid.
//
// Example:
//
//	cfg, err := NewConfig(
//	    WithName("my-agent"),
//	    WithPort(8080),
//	    WithRedisURL("redis://localhost:6379"),
//	)
//	if err != nil {
//	    return err
//	}
func NewConfig(opts ...Option) (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()

	// Load from environment first (includes validation per spec)
	if err := cfg.LoadFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to load env config: %w", err)
	}

	// Apply functional options (these override env vars)
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	if cfg.logger == nil {
		logger := NewProductionLogger(cfg.Logging, cfg.Development, cfg.Name)

		// Track for metrics enabling when telemetry available
		if prodLogger, ok := logger.(*ProductionLogger); ok {
			trackLogger(prodLogger)
		}

		cfg.logger = logger
	}

	// Validate final configuration after options applied
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// ============================================================================
// ProductionLogger Implementation - Layered Observability Architecture
// ============================================================================

// ProductionLogger provides layered observability for framework operations
type ProductionLogger struct {
	level       string
	debug       bool
	serviceName string
	format      string
	output      io.Writer

	// Metrics layer (enabled when telemetry available)
	metricsEnabled bool
}

// NewProductionLogger creates a logger from LoggingConfig
func NewProductionLogger(logging LoggingConfig, dev DevelopmentConfig, serviceName string) Logger {
	var output io.Writer = os.Stdout
	if logging.Output == "stderr" {
		output = os.Stderr
	}

	return &ProductionLogger{
		level:          strings.ToLower(logging.Level),
		debug:          dev.DebugLogging || logging.Level == "debug",
		serviceName:    serviceName,
		format:         logging.Format,
		output:         output,
		metricsEnabled: false, // Enabled by telemetry module when available
	}
}

// EnableMetrics is called by telemetry module to enable metrics layer
func (p *ProductionLogger) EnableMetrics() {
	p.metricsEnabled = true
}

func (p *ProductionLogger) Info(msg string, fields map[string]interface{}) {
	p.logEvent("INFO", msg, fields, nil)
}

func (p *ProductionLogger) InfoWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	p.logEvent("INFO", msg, fields, ctx)
}

func (p *ProductionLogger) Error(msg string, fields map[string]interface{}) {
	p.logEvent("ERROR", msg, fields, nil)
}

func (p *ProductionLogger) ErrorWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	p.logEvent("ERROR", msg, fields, ctx)
}

func (p *ProductionLogger) Warn(msg string, fields map[string]interface{}) {
	p.logEvent("WARN", msg, fields, nil)
}

func (p *ProductionLogger) Debug(msg string, fields map[string]interface{}) {
	if p.debug {
		p.logEvent("DEBUG", msg, fields, nil)
	}
}

// Core logging implementation with all three layers
func (p *ProductionLogger) logEvent(level, msg string, fields map[string]interface{}, ctx context.Context) {
	timestamp := time.Now().Format(time.RFC3339)

	if p.format == "json" {
		// Structured logging for production log aggregation
		logEntry := map[string]interface{}{
			"timestamp": timestamp,
			"level":     level,
			"service":   p.serviceName,
			"component": "framework",
			"message":   msg,
		}

		// LAYER 3: Add trace context when available
		if ctx != nil && p.metricsEnabled {
			if baggage := getContextBaggage(ctx); len(baggage) > 0 {
				for k, v := range baggage {
					logEntry["trace."+k] = v
				}
			}
		}

		// Add all fields
		for k, v := range fields {
			logEntry[k] = v
		}

		if data, err := json.Marshal(logEntry); err == nil {
			fmt.Fprintln(p.output, string(data))
		}
	} else {
		// Human-readable for local development
		traceInfo := ""
		if ctx != nil && p.metricsEnabled {
			if baggage := getContextBaggage(ctx); baggage["request_id"] != "" {
				traceInfo = fmt.Sprintf("[req=%s] ", baggage["request_id"])
			}
		}

		var fieldStr strings.Builder
		if len(fields) > 0 {
			fieldStr.WriteString(" ")
			for k, v := range fields {
				fieldStr.WriteString(fmt.Sprintf("%s=%v ", k, v))
			}
		}

		fmt.Fprintf(p.output, "%s [%s] [%s] %s%s%s\n",
			timestamp, level, p.serviceName, traceInfo, msg, fieldStr.String())
	}

	if p.metricsEnabled {
		p.emitFrameworkMetric(level, msg, fields, ctx)
	}
}

// Metrics emission with cardinality protection
func (p *ProductionLogger) emitFrameworkMetric(level, msg string, fields map[string]interface{}, ctx context.Context) {
	// Build labels with cardinality awareness
	labels := []string{
		"level", level,
		"service", p.serviceName,
		"component", "framework",
	}

	// Add only low-cardinality fields as labels
	for k, v := range fields {
		switch k {
		case "operation", "status", "error_type", "service_type", "provider":
			labels = append(labels, k, fmt.Sprintf("%v", v))
		}
	}

	// Emit with context when available (enables correlation)
	if ctx != nil {
		emitMetricWithContext(ctx, "gomind.framework.operations", 1.0, labels...)
	} else {
		emitMetric("gomind.framework.operations", 1.0, labels...)
	}
}

// Helper functions for weak coupling to telemetry
func emitMetric(name string, value float64, labels ...string) {
	if globalMetricsRegistry != nil {
		globalMetricsRegistry.Counter(name, labels...)
	}
}

func emitMetricWithContext(ctx context.Context, name string, value float64, labels ...string) {
	if globalMetricsRegistry != nil {
		globalMetricsRegistry.EmitWithContext(ctx, name, value, labels...)
	}
}

func getContextBaggage(ctx context.Context) map[string]string {
	if globalMetricsRegistry != nil {
		return globalMetricsRegistry.GetBaggage(ctx)
	}
	return make(map[string]string)
}
