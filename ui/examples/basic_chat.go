// Package main demonstrates the GoMind UI module with auto-configured transports
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/ui"
	_ "github.com/itsneelabh/gomind/ui/transports/sse" // Auto-registers SSE transport
)

func main() {
	ctx := context.Background()

	// Configuration
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	// Create AI client (auto-detects provider from environment)
	aiClient, err := ai.NewClient()
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err)
	}

	// Create session manager
	// For production: Use Redis for distributed sessions
	var sessions ui.SessionManager
	if redisURL != "" {
		sessions, err = ui.NewRedisSessionManager(redisURL, ui.DefaultSessionConfig())
		if err != nil {
			log.Printf("Failed to create Redis session manager: %v", err)
			log.Println("Falling back to in-memory sessions")
			sessions = ui.NewMockSessionManager(ui.DefaultSessionConfig())
		}
	} else {
		// For development: Use in-memory sessions
		sessions = ui.NewMockSessionManager(ui.DefaultSessionConfig())
	}

	// Create chat agent with auto-configured transports
	config := ui.DefaultChatAgentConfig("assistant")
	agent := ui.NewChatAgent(config, aiClient, sessions)

	// The agent automatically:
	// 1. Discovers all registered transports
	// 2. Initializes and starts them
	// 3. Creates HTTP endpoints for each transport
	// 4. Provides a discovery endpoint at /chat/transports

	log.Println("Chat agent configured with transports:")
	for _, transport := range agent.ListTransports() {
		log.Printf("  - %s: %s", transport.Name, transport.Endpoint)
	}
	log.Println("Discovery endpoint: /chat/transports")
	log.Println("Health endpoint: /chat/health")

	// Start the agent's HTTP server
	go func() {
		if err := agent.Start(8080); err != nil {
			log.Fatal("Failed to start agent:", err)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Println("Server starting on http://localhost:8080")
	log.Println("Try the following:")
	log.Println("  curl http://localhost:8080/chat/transports")
	log.Println("  Open http://localhost:8080/chat/sse?message=Hello in browser")

	// Wait for shutdown signal
	<-sigChan

	log.Println("Shutting down...")
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := agent.Stop(shutdownCtx); err != nil {
		log.Printf("Error stopping agent: %v", err)
	}
}