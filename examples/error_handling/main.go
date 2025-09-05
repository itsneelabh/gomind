package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/itsneelabh/gomind/core"
)

// Example demonstrating the use of the new standard error types
func main() {
	// Example 1: Using sentinel errors for comparison
	err := processAgent("unknown-agent")
	if errors.Is(err, core.ErrAgentNotFound) {
		log.Println("Agent not found, creating a new one...")
		// Handle agent not found scenario
	}

	// Example 2: Using the IsRetryable helper
	err = callExternalService()
	if core.IsRetryable(err) {
		log.Println("Error is retryable, will attempt retry...")
		// Implement retry logic
	}

	// Example 3: Using FrameworkError for structured errors
	err = performOperation()
	var frameworkErr *core.FrameworkError
	if errors.As(err, &frameworkErr) {
		log.Printf("Operation failed: %s in %s", frameworkErr.Op, frameworkErr.Kind)
	}

	// Example 4: Checking for configuration errors
	err = validateConfig()
	if core.IsConfigurationError(err) {
		log.Println("Configuration error detected, please check your settings")
	}
}

// Example function that returns a standard error
func processAgent(agentID string) error {
	// Simulate agent lookup
	if agentID == "unknown-agent" {
		// Return the standard error that can be checked with errors.Is()
		return core.ErrAgentNotFound
	}
	return nil
}

// Example function that returns a retryable error
func callExternalService() error {
	// Simulate a timeout scenario
	return fmt.Errorf("service call failed: %w", core.ErrTimeout)
}

// Example function that returns a structured error
func performOperation() error {
	// Use FrameworkError for better context
	return &core.FrameworkError{
		Op:      "discovery.Register",
		Kind:    "discovery",
		Message: "failed to register agent-123",
		Err:     core.ErrDiscoveryUnavailable,
	}
}

// Example function that returns a configuration error
func validateConfig() error {
	// Return a wrapped configuration error
	return fmt.Errorf("port value is negative: %w", core.ErrInvalidConfiguration)
}

// Example: How to handle errors in agent implementation
type MyAgent struct {
	*core.BaseAgent
}

func (a *MyAgent) Initialize(ctx context.Context) error {
	// Call parent initialization
	if err := a.BaseAgent.Initialize(ctx); err != nil {
		// Wrap the error with context
		return &core.FrameworkError{
			Op:      "MyAgent.Initialize",
			Kind:    "agent",
			Message: fmt.Sprintf("failed to initialize agent %s", a.GetID()),
			Err:     err,
		}
	}

	// Custom initialization logic
	if err := a.setupCustomResources(); err != nil {
		// Check if it's a known error type
		if errors.Is(err, core.ErrAlreadyStarted) {
			// Handle specific case
			return nil // Already initialized, not an error
		}
		return err
	}

	return nil
}

func (a *MyAgent) setupCustomResources() error {
	// Example setup logic
	return nil
}
