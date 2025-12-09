// Package main implements a research assistant agent with comprehensive telemetry
// and observability using the GoMind framework's telemetry module.
//
// This example demonstrates how to add production-grade monitoring to an agent with
// minimal code changes. It builds on the agent-example by adding:
//   - Comprehensive metrics collection (counters, histograms, gauges)
//   - Distributed tracing with OpenTelemetry
//   - Environment-based telemetry profiles (dev/staging/prod)
//   - Integration with Prometheus, Jaeger, and Grafana
//
// Environment Variables:
//
//	REDIS_URL                      - Redis connection URL (required)
//	PORT                           - HTTP server port (default: 8092)
//	NAMESPACE                      - Kubernetes namespace for service discovery
//	OPENAI_API_KEY                 - OpenAI API key for AI capabilities
//	APP_ENV                        - Environment: development, staging, production
//	OTEL_EXPORTER_OTLP_ENDPOINT    - OpenTelemetry collector endpoint
//	DEV_MODE                       - Enable development mode (true/false)
//
// Example Usage:
//
//	export REDIS_URL="redis://localhost:6379"
//	export OPENAI_API_KEY="sk-..."
//	export APP_ENV="development"
//	export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318"
//	go run .
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"

	// Import AI providers for auto-detection
	_ "github.com/itsneelabh/gomind/ai/providers/anthropic"
	_ "github.com/itsneelabh/gomind/ai/providers/gemini"
	_ "github.com/itsneelabh/gomind/ai/providers/openai"
)

func main() {
	// Track startup time for metrics
	startupStart := time.Now()

	// 1. Validate configuration first (fail fast)
	if err := validateConfig(); err != nil {
		log.Fatalf("❌ Configuration error: %v", err)
	}

	// 2. Initialize telemetry BEFORE creating agent
	// This ensures all agent operations are instrumented from the start
	initTelemetry("research-assistant-telemetry")
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := telemetry.Shutdown(ctx); err != nil {
			log.Printf("⚠️  Warning: Telemetry shutdown error: %v", err)
		}
	}()

	// 3. Create research agent
	agent, err := NewResearchAgent()
	if err != nil {
		log.Fatalf("Failed to create research agent: %v", err)
	}

	// Initialize schema cache with Redis (for Phase 3 validation caching)
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		// Parse Redis options from URL
		redisOpt, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Printf("⚠️  Warning: Failed to parse REDIS_URL for schema cache: %v", err)
			log.Println("   Schema caching will be disabled")
		} else {
			// Create Redis client for schema cache
			redisClient := redis.NewClient(redisOpt)

			// Test connection
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if err := redisClient.Ping(ctx).Err(); err != nil {
				log.Printf("⚠️  Warning: Redis connection failed for schema cache: %v", err)
				log.Println("   Schema caching will be disabled")
				redisClient.Close()
			} else {
				// Initialize schema cache with Redis backend
				agent.SchemaCache = core.NewSchemaCache(redisClient)
				log.Println("✅ Schema cache initialized with Redis backend")
			}
		}
	} else {
		log.Println("ℹ️  Schema caching disabled (no REDIS_URL)")
	}

	// 4. Get port configuration
	port := 8092 // default for agent-with-telemetry
	if portStr := os.Getenv("PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	// 5. Create framework with configuration
	framework, err := core.NewFramework(agent,
		core.WithName("research-assistant-telemetry"),
		core.WithPort(port),
		core.WithNamespace(os.Getenv("NAMESPACE")),
		core.WithRedisURL(os.Getenv("REDIS_URL")),
		core.WithDiscovery(true, "redis"),
		core.WithCORS([]string{"*"}, true),
		core.WithDevelopmentMode(os.Getenv("DEV_MODE") == "true"),

		// Distributed tracing middleware for context propagation
		core.WithMiddleware(telemetry.TracingMiddleware("research-assistant-telemetry")),
	)
	if err != nil {
		log.Fatalf("Failed to create framework: %v", err)
	}

	// 6. Emit startup metrics
	startupDuration := time.Since(startupStart)
	// Use Microseconds and convert to float ms to preserve precision
	startupMs := float64(startupDuration.Microseconds()) / 1000.0
	telemetry.Histogram("agent.startup.duration_ms",
		startupMs,
		"agent", "research-assistant-telemetry",
		"status", "success",
	)
	telemetry.Gauge("agent.capabilities.count",
		float64(len(agent.Capabilities)),
		"agent", "research-assistant-telemetry",
	)

	// 6b. Perform initial service discovery to populate metrics
	// This triggers discovery.services.found and discovery.lookups metrics
	initCtx, initCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer initCancel()
	if agent.Discovery != nil {
		tools, err := agent.Discovery.Discover(initCtx, core.DiscoveryFilter{
			Type: core.ComponentTypeTool,
		})
		if err != nil {
			log.Printf("⚠️  Initial tool discovery failed: %v", err)
		} else {
			log.Printf("🔍 Discovered %d tools at startup", len(tools))
			// Emit services found gauge
			telemetry.Gauge("discovery.services.found",
				float64(len(tools)),
				"type", "tool",
			)
		}
	}

	// 7. Display startup information
	log.Println("🤖 Research Assistant Agent (with Telemetry) Starting...")
	log.Println("📊 Telemetry: Enabled")
	log.Println("🧠 AI Provider:", getAIProviderStatus())
	log.Printf("🌐 Server Port: %d\n", port)
	log.Printf("📊 Capabilities Registered: %d\n", len(agent.Capabilities))
	log.Printf("⏱️  Startup Duration: %v\n", startupDuration)
	log.Println("📋 Registered endpoints will be shown in framework logs below...")
	log.Println()

	// 7. Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n⚠️  Shutting down gracefully...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		cancel()

		select {
		case <-shutdownCtx.Done():
			log.Println("❌ Shutdown timeout exceeded")
			os.Exit(1)
		case <-time.After(1 * time.Second):
			// Give framework time to clean up
		}

		log.Println("✅ Shutdown completed")
		os.Exit(0)
	}()

	// 8. Run the framework (blocking)
	if err := framework.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("Framework error: %v", err)
	}
}

// validateConfig validates all required configuration at startup
func validateConfig() error {
	// REDIS_URL is required for discovery
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return fmt.Errorf("REDIS_URL environment variable required")
	}

	// Validate Redis URL format
	if !strings.HasPrefix(redisURL, "redis://") && !strings.HasPrefix(redisURL, "rediss://") {
		return fmt.Errorf("invalid REDIS_URL format (must start with redis:// or rediss://)")
	}

	// Validate port if set
	if portStr := os.Getenv("PORT"); portStr != "" {
		if _, err := strconv.Atoi(portStr); err != nil {
			return fmt.Errorf("invalid PORT value: %v", err)
		}
	}

	return nil
}

// initTelemetry sets up telemetry based on environment with graceful degradation
func initTelemetry(serviceName string) {
	// Detect environment from APP_ENV variable
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development" // Safe default
	}

	// Select the appropriate telemetry profile
	var profile telemetry.Profile
	switch env {
	case "production", "prod":
		profile = telemetry.ProfileProduction
	case "staging", "stage", "qa":
		profile = telemetry.ProfileStaging
	default:
		profile = telemetry.ProfileDevelopment
	}

	// Use the profile to get base configuration
	config := telemetry.UseProfile(profile)

	// Override with service name
	config.ServiceName = serviceName

	// Allow environment variables to override specific settings
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		config.Endpoint = endpoint
	}

	// Initialize telemetry
	if err := telemetry.Initialize(config); err != nil {
		// IMPORTANT: Don't let telemetry failures crash your app!
		log.Printf("⚠️  Warning: Telemetry initialization failed: %v", err)
		log.Printf("   Application will continue without telemetry")
		return
	}

	// Enable framework integration - this allows core components (redis_registry, discovery)
	// to emit metrics like discovery.registrations, discovery.health_checks, etc.
	telemetry.EnableFrameworkIntegration(nil)

	log.Printf("✅ Telemetry initialized successfully")
	log.Printf("   Environment: %s", env)
	log.Printf("   Profile: %s", profile)
	log.Printf("   Service: %s", serviceName)
	if config.Endpoint != "" {
		log.Printf("   Endpoint: %s", config.Endpoint)
	}
}

// getAIProviderStatus returns the detected AI provider name
func getAIProviderStatus() string {
	// Check for common AI provider environment variables
	providers := []struct {
		name   string
		envVar string
	}{
		{"OpenAI", "OPENAI_API_KEY"},
		{"Groq", "GROQ_API_KEY"},
		{"Anthropic", "ANTHROPIC_API_KEY"},
		{"Gemini", "GEMINI_API_KEY"},
		{"DeepSeek", "DEEPSEEK_API_KEY"},
	}

	for _, provider := range providers {
		if os.Getenv(provider.envVar) != "" {
			return provider.name
		}
	}

	// Check for custom OpenAI-compatible endpoints
	if os.Getenv("OPENAI_BASE_URL") != "" {
		return "Custom OpenAI-Compatible"
	}

	return "None (will use mock responses)"
}
