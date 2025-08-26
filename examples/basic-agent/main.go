package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/itsneelabh/gomind"
)

// BasicAgent demonstrates a simple agent implementation
type BasicAgent struct {
	framework.BaseAgent
}

// @capability: greet
// @description: Greets users with a friendly message
// @input: name string "Name of the person to greet"
// @output: greeting string "Personalized greeting message"
func (b *BasicAgent) Greet(name string) string {
	if name == "" {
		return "Hello there! How can I help you today?"
	}
	return "Hello " + name + "! Nice to meet you. How can I assist you?"
}

// @capability: echo
// @description: Echoes back the provided message
// @input: message string "Message to echo back"
// @output: echo string "The echoed message"
func (b *BasicAgent) Echo(message string) string {
	return "You said: " + message
}

func main() {
	// Create a new framework instance using NewFramework with options
	fw, err := framework.NewFramework(
		framework.WithPort(8080),
		framework.WithAgentName("basic-agent"),
	)
	if err != nil {
		log.Fatalf("Failed to create framework: %v", err)
	}

	// Create the basic agent
	basicAgent := &BasicAgent{}

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping framework...")
		cancel()
	}()

	log.Println("Starting basic agent on port 8080...")
	log.Println("Visit http://localhost:8080 for the chat interface")

	// Initialize and start the framework with the agent
	if err := fw.InitializeAgent(ctx, basicAgent); err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}

	// Start the HTTP server
	if err := fw.StartHTTPServer(ctx, basicAgent); err != nil {
		log.Fatalf("Framework error: %v", err)
	}

	log.Println("Framework stopped gracefully")
}
