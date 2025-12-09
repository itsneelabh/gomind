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

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
)

func main() {
	// Validate configuration first
	if err := validateConfig(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Initialize telemetry
	initTelemetry("stock-service")
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := telemetry.Shutdown(ctx); err != nil {
			log.Printf("⚠️  Warning: Telemetry shutdown error: %v", err)
		}
	}()

	// Create stock market tool
	tool := NewStockTool()

	// Get port configuration from environment
	port := 8082 // default (8080 is weather, 8081 might be used)
	if portStr := os.Getenv("PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	// Framework handles all the complexity
	framework, err := core.NewFramework(tool,
		// Core configuration from environment
		core.WithName("stock-service"),
		core.WithPort(port),
		core.WithNamespace(os.Getenv("NAMESPACE")),

		// Discovery configuration (tools can register but not discover)
		core.WithRedisURL(os.Getenv("REDIS_URL")),
		core.WithDiscovery(true, "redis"),

		// CORS for web access
		core.WithCORS([]string{"*"}, true),

		// Development mode from environment
		core.WithDevelopmentMode(os.Getenv("DEV_MODE") == "true"),

		// Distributed tracing middleware for context propagation
		core.WithMiddleware(telemetry.TracingMiddleware("stock-service")),
	)
	if err != nil {
		log.Fatalf("Failed to create framework: %v", err)
	}

	// Display startup information
	log.Println("📈 Stock Market Tool Service Starting...")
	log.Println("📊 Telemetry: Enabled")
	log.Printf("🌐 Server Port: %d\n", port)
	log.Println("📋 Registered endpoints will be shown in framework logs below...")
	log.Println()

	// Set up graceful shutdown
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

	// Run the framework (blocking)
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

	// FINNHUB_API_KEY is required for stock data
	apiKey := os.Getenv("FINNHUB_API_KEY")
	if apiKey == "" {
		log.Println("⚠️  Warning: FINNHUB_API_KEY not set - tool will use mock data")
	}

	// Validate port if set
	if portStr := os.Getenv("PORT"); portStr != "" {
		if _, err := strconv.Atoi(portStr); err != nil {
			return fmt.Errorf("invalid PORT value: %v", err)
		}
	}

	return nil
}

// initTelemetry sets up telemetry based on environment
func initTelemetry(serviceName string) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	var profile telemetry.Profile
	switch env {
	case "production", "prod":
		profile = telemetry.ProfileProduction
	case "staging", "stage", "qa":
		profile = telemetry.ProfileStaging
	default:
		profile = telemetry.ProfileDevelopment
	}

	config := telemetry.UseProfile(profile)
	config.ServiceName = serviceName

	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		config.Endpoint = endpoint
	}

	if err := telemetry.Initialize(config); err != nil {
		log.Printf("⚠️  Warning: Telemetry initialization failed: %v", err)
		log.Printf("   Tool will continue without telemetry")
		return
	}

	// Enable framework integration - this allows core components (redis_registry, discovery)
	// to emit metrics like discovery.registrations, discovery.health_checks, etc.
	telemetry.EnableFrameworkIntegration(nil)

	log.Printf("✅ Telemetry initialized for %s", serviceName)
}
