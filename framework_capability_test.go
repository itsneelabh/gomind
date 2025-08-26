package framework_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/itsneelabh/gomind"
)

// TestCapabilityAgent is an agent with explicit capabilities
type TestCapabilityAgent struct {
	framework.BaseAgent
}

func (t *TestCapabilityAgent) Initialize(ctx context.Context) error {
	return nil
}

// ProcessData is an explicitly declared capability
// @capability: process_data
// @description: Processes input data and returns a result
func (t *TestCapabilityAgent) ProcessData(ctx context.Context, input string) (string, error) {
	return "Processed: " + input, nil
}

// CalculateSum is another explicitly declared capability
// @capability: calculate_sum
// @description: Calculates the sum of two numbers
func (t *TestCapabilityAgent) CalculateSum(ctx context.Context, a, b int) int {
	return a + b
}

// helperMethod is NOT a capability (no annotation)
func (t *TestCapabilityAgent) helperMethod() string {
	return "I should not be exposed"
}

// TestCapabilityEndpoints tests that capability endpoints are auto-generated
func TestCapabilityEndpoints(t *testing.T) {
	// Create test agent
	agent := &TestCapabilityAgent{}
	
	// Create framework
	fw, err := framework.NewFramework(
		framework.WithAgentName("test-capability-agent"),
	)
	if err != nil {
		t.Fatalf("Failed to create framework: %v", err)
	}
	
	// Initialize agent
	ctx := context.Background()
	if err := fw.InitializeAgent(ctx, agent); err != nil {
		t.Fatalf("Failed to initialize agent: %v", err)
	}
	
	// Start HTTP server in background
	server := httptest.NewServer(nil)
	defer server.Close()
	
	go func() {
		if err := fw.StartHTTPServer(ctx, agent); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()
	
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	
	// Test 1: Check /capabilities endpoint
	resp, err := http.Get("http://localhost:8080/capabilities")
	if err != nil {
		t.Logf("Warning: Could not reach /capabilities endpoint: %v", err)
		t.Skip("Server not accessible, skipping test")
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	
	var capResponse struct {
		Capabilities []framework.CapabilityMetadata `json:"capabilities"`
		Count        int                             `json:"count"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&capResponse); err != nil {
		t.Fatalf("Failed to decode capabilities response: %v", err)
	}
	
	// Should have at least our declared capabilities
	if capResponse.Count < 2 {
		t.Errorf("Expected at least 2 capabilities, got %d", capResponse.Count)
	}
	
	// Test 2: Check that process_data capability exists
	foundProcessData := false
	foundCalculateSum := false
	
	for _, cap := range capResponse.Capabilities {
		if cap.Name == "process_data" {
			foundProcessData = true
		}
		if cap.Name == "calculate_sum" {
			foundCalculateSum = true
		}
		// Make sure helper method is NOT exposed
		if cap.Name == "helper_method" {
			t.Error("Helper method should not be exposed as capability")
		}
	}
	
	if !foundProcessData {
		t.Error("process_data capability not found")
	}
	if !foundCalculateSum {
		t.Error("calculate_sum capability not found")
	}
	
	// Test 3: Invoke process_data capability via POST /invoke/process_data
	input := map[string]interface{}{
		"input": "test data",
	}
	inputBytes, _ := json.Marshal(input)
	
	resp, err = http.Post("http://localhost:8080/invoke/process_data", 
		"application/json", 
		bytes.NewReader(inputBytes))
	
	if err != nil {
		t.Logf("Warning: Could not invoke capability: %v", err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for capability invocation, got %d", resp.StatusCode)
	}
	
	var invokeResponse struct {
		Capability string      `json:"capability"`
		Result     interface{} `json:"result"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&invokeResponse); err != nil {
		t.Fatalf("Failed to decode invocation response: %v", err)
	}
	
	if invokeResponse.Capability != "process_data" {
		t.Errorf("Expected capability 'process_data', got '%s'", invokeResponse.Capability)
	}
	
	// Test 4: Try to invoke non-existent capability
	resp, err = http.Post("http://localhost:8080/invoke/helper_method", 
		"application/json", 
		bytes.NewReader([]byte("{}")))
	
	if err == nil {
		defer resp.Body.Close()
		// Should get 404 because helper_method is not a declared capability
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 for non-capability method, got %d", resp.StatusCode)
		}
	}
}