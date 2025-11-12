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

	// Import AI providers for auto-detection
	_ "github.com/itsneelabh/gomind/ai/providers/anthropic"
	_ "github.com/itsneelabh/gomind/ai/providers/gemini"
	_ "github.com/itsneelabh/gomind/ai/providers/openai"
)

func main() {
	// Validate configuration first
	if err := validateConfig(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Create research agent
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

	// Get port configuration
	port := 8090 // default
	if portStr := os.Getenv("PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	// Create framework with configuration
	framework, err := core.NewFramework(agent,
		core.WithName("research-assistant"),
		core.WithPort(port),
		core.WithNamespace(os.Getenv("NAMESPACE")),
		core.WithRedisURL(os.Getenv("REDIS_URL")),
		core.WithDiscovery(true, "redis"),
		core.WithCORS([]string{"*"}, true),
		core.WithDevelopmentMode(os.Getenv("DEV_MODE") == "true"),
	)
	if err != nil {
		log.Fatalf("Failed to create framework: %v", err)
	}

	// Display startup information
	log.Println("🤖 Research Assistant Agent Starting...")
	log.Println("🧠 AI Provider:", getAIProviderStatus())
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

	// Validate port if set
	if portStr := os.Getenv("PORT"); portStr != "" {
		if _, err := strconv.Atoi(portStr); err != nil {
			return fmt.Errorf("invalid PORT value: %v", err)
		}
	}

	return nil
}
