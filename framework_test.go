package framework_test

import (
	"context"
	"testing"

	"github.com/itsneelabh/gomind"
)

// TestFrameworkCreation tests creating a new framework instance
func TestFrameworkCreation(t *testing.T) {
	fw, err := framework.NewFramework(
		framework.WithPort(8080),
		framework.WithAgentName("test-agent"),
	)
	if err != nil {
		t.Fatalf("Failed to create framework: %v", err)
	}
	if fw == nil {
		t.Fatal("Framework is nil")
	}
}

// TestAgent is a simple test agent
type TestAgent struct {
	framework.BaseAgent
	initialized bool
}

func (t *TestAgent) Initialize(ctx context.Context) error {
	t.initialized = true
	return nil
}

// TestMethod is a test capability
// @capability: test
// @description: A test capability
func (t *TestAgent) TestMethod() string {
	return "test response"
}

// TestAgentInitialization tests agent initialization
func TestAgentInitialization(t *testing.T) {
	agent := &TestAgent{}
	
	fw, err := framework.NewFramework(
		framework.WithAgentName("test-agent"),
	)
	if err != nil {
		t.Fatalf("Failed to create framework: %v", err)
	}

	ctx := context.Background()
	err = fw.InitializeAgent(ctx, agent)
	if err != nil {
		t.Fatalf("Failed to initialize agent: %v", err)
	}

	if !agent.initialized {
		t.Error("Agent was not initialized")
	}
}

// TestCapabilityDiscovery tests that capabilities are discovered
func TestCapabilityDiscovery(t *testing.T) {
	agent := &TestAgent{}
	
	fw, err := framework.NewFramework(
		framework.WithAgentName("test-agent"),
	)
	if err != nil {
		t.Fatalf("Failed to create framework: %v", err)
	}

	ctx := context.Background()
	err = fw.InitializeAgent(ctx, agent)
	if err != nil {
		t.Fatalf("Failed to initialize agent: %v", err)
	}

	capabilities := agent.GetCapabilities()
	if len(capabilities) == 0 {
		t.Error("No capabilities discovered")
	}

	// Check if our test capability was discovered
	found := false
	for _, cap := range capabilities {
		t.Logf("Found capability: %s", cap.Name)
		if cap.Name == "test" || cap.Name == "test_method" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Test capability was not discovered (expected 'test' or 'test_method')")
	}
}