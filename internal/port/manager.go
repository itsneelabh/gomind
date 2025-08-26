package port

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// Logger is the interface for logging
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

// Helper functions
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

// Environment represents the deployment environment
type Environment string

const (
	EnvLocal      Environment = "local"
	EnvDocker     Environment = "docker"
	EnvKubernetes Environment = "kubernetes"
	EnvProduction Environment = "production"
)

// ServerConfig holds server configuration with environment awareness
type ServerConfig struct {
	Port         int         `yaml:"port" env:"PORT" default:"auto"`
	Host         string      `yaml:"host" env:"HOST" default:"0.0.0.0"`
	PortRange    string      `yaml:"port_range" env:"PORT_RANGE" default:"8080-8090"`
	AutoDiscover bool        `yaml:"auto_discover" env:"AUTO_DISCOVER" default:"true"`
	Environment  Environment `yaml:"-"` // Auto-detected, not configurable
}

// PortStrategy defines how ports should be handled
type PortStrategy struct {
	Port         int
	AutoDiscover bool
	Source       string
	Environment  Environment
}

// PortManager handles environment-aware port discovery and management
type PortManager struct {
	config *ServerConfig
	logger Logger
}

// NewPortManager creates a new port manager with environment detection
func NewPortManager(logger Logger) *PortManager {
	config := &ServerConfig{
		Host:         getEnvOrDefault("HOST", "0.0.0.0"),
		PortRange:    getEnvOrDefault("PORT_RANGE", "8080-8090"),
		AutoDiscover: getEnvBoolOrDefault("AUTO_DISCOVER", true),
		Environment:  detectEnvironment(),
	}

	// Parse PORT environment variable
	if portEnv := os.Getenv("PORT"); portEnv != "" {
		if portEnv == "auto" {
			config.Port = 0 // Signal for auto-discovery
		} else if port, err := strconv.Atoi(portEnv); err == nil {
			config.Port = port
			config.AutoDiscover = false // Explicit port disables auto-discovery
		}
	}

	return &PortManager{
		config: config,
		logger: logger,
	}
}

// DetectEnvironment automatically detects the deployment environment
func detectEnvironment() Environment {
	// Check for Kubernetes environment
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" ||
		os.Getenv("KUBERNETES_PORT") != "" ||
		fileExists("/var/run/secrets/kubernetes.io/serviceaccount/token") {
		return EnvKubernetes
	}

	// Check for Docker Compose environment
	if os.Getenv("COMPOSE_PROJECT_NAME") != "" {
		return EnvDocker
	}

	// Check for production indicators
	if os.Getenv("NODE_ENV") == "production" ||
		os.Getenv("GO_ENV") == "production" ||
		os.Getenv("ENVIRONMENT") == "production" {
		return EnvProduction
	}

	// Default to local development
	return EnvLocal
}

// GetPortStrategy determines the appropriate port strategy for the current environment
func (pm *PortManager) GetPortStrategy() PortStrategy {
	env := pm.config.Environment

	switch env {
	case EnvKubernetes:
		// Kubernetes: Use fixed port 8080, pods are isolated
		port := 8080
		if pm.config.Port > 0 {
			port = pm.config.Port // Allow override via explicit PORT env var
		}
		return PortStrategy{
			Port:         port,
			AutoDiscover: false,
			Source:       "kubernetes-fixed",
			Environment:  env,
		}

	case EnvDocker:
		// Docker Compose: Use internal port 8080, external mapping handled by compose
		port := 8080
		if pm.config.Port > 0 {
			port = pm.config.Port
		}
		return PortStrategy{
			Port:         port,
			AutoDiscover: false,
			Source:       "docker-compose",
			Environment:  env,
		}

	case EnvProduction:
		// Production: Use fixed port, no auto-discovery
		port := 8080
		if pm.config.Port > 0 {
			port = pm.config.Port
		}
		return PortStrategy{
			Port:         port,
			AutoDiscover: false,
			Source:       "production-fixed",
			Environment:  env,
		}

	case EnvLocal:
		// Local development: Auto-discover available ports
		if pm.config.Port > 0 {
			// Explicit port specified
			return PortStrategy{
				Port:         pm.config.Port,
				AutoDiscover: false,
				Source:       "explicit-port",
				Environment:  env,
			}
		}

		if !pm.config.AutoDiscover {
			// Auto-discovery disabled, use default
			return PortStrategy{
				Port:         8080,
				AutoDiscover: false,
				Source:       "default-port",
				Environment:  env,
			}
		}

		// Auto-discover available port
		port := pm.findAvailablePortInRange(pm.config.PortRange)
		return PortStrategy{
			Port:         port,
			AutoDiscover: true,
			Source:       "auto-discovery",
			Environment:  env,
		}

	default:
		// Fallback to safe defaults
		return PortStrategy{
			Port:         8080,
			AutoDiscover: false,
			Source:       "fallback",
			Environment:  env,
		}
	}
}

// DeterminePort returns the port to use based on environment and configuration
func (pm *PortManager) DeterminePort() int {
	strategy := pm.GetPortStrategy()

	pm.logger.Info("Port strategy determined", map[string]interface{}{
		"port":          strategy.Port,
		"auto_discover": strategy.AutoDiscover,
		"source":        strategy.Source,
		"environment":   string(strategy.Environment),
		"host":          pm.config.Host,
	})

	return strategy.Port
}

// findAvailablePortInRange finds an available port within the specified range
func (pm *PortManager) findAvailablePortInRange(portRange string) int {
	start, end := pm.parsePortRange(portRange)

	for port := start; port <= end; port++ {
		if pm.isPortAvailable(port) {
			return port
		}
	}

	// If no port in range is available, find any available port
	pm.logger.Warn("No ports available in range, finding any available port", map[string]interface{}{
		"range": portRange,
	})
	return pm.findAnyAvailablePort()
}

// parsePortRange parses a port range string like "8080-8090"
func (pm *PortManager) parsePortRange(portRange string) (int, int) {
	parts := strings.Split(portRange, "-")
	if len(parts) != 2 {
		pm.logger.Warn("Invalid port range format, using default", map[string]interface{}{
			"range":   portRange,
			"default": "8080-8090",
		})
		return 8080, 8090
	}

	start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))

	if err1 != nil || err2 != nil || start > end {
		pm.logger.Warn("Invalid port range values, using default", map[string]interface{}{
			"range":   portRange,
			"default": "8080-8090",
		})
		return 8080, 8090
	}

	return start, end
}

// isPortAvailable checks if a port is available on the host
func (pm *PortManager) isPortAvailable(port int) bool {
	address := fmt.Sprintf("%s:%d", pm.config.Host, port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	defer listener.Close()
	return true
}

// findAnyAvailablePort finds any available port starting from 8080
func (pm *PortManager) findAnyAvailablePort() int {
	// Try common ports first
	commonPorts := []int{8080, 8081, 8082, 8083, 8084, 8085, 8090, 8091, 8092, 8093, 8094, 8095}

	for _, port := range commonPorts {
		if pm.isPortAvailable(port) {
			return port
		}
	}

	// If all common ports are taken, let the OS assign one
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:0", pm.config.Host))
	if err != nil {
		pm.logger.Error("Failed to find any available port", map[string]interface{}{
			"error": err.Error(),
		})
		return 8080 // Last resort fallback
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	pm.logger.Info("OS-assigned port", map[string]interface{}{
		"port": port,
	})
	return port
}

// ValidatePort checks if the determined port is actually available
func (pm *PortManager) ValidatePort(port int) error {
	if !pm.isPortAvailable(port) {
		return fmt.Errorf("port %d is not available on %s", port, pm.config.Host)
	}
	return nil
}

// GetServerAddress returns the complete server address
func (pm *PortManager) GetServerAddress(port int) string {
	return fmt.Sprintf("%s:%d", pm.config.Host, port)
}

// GetPublicURL returns the public URL for the server (for logging/display)
func (pm *PortManager) GetPublicURL(port int) string {
	host := pm.config.Host
	if host == "0.0.0.0" || host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}

// fileExists checks if a file exists
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}
