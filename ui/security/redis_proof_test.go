//go:build security
// +build security

package security

import (
	"testing"
)

// TestRedisProofDemo runs the Redis demonstration
func TestRedisProofDemo(t *testing.T) {
	err := DemoRedisRateLimiting()
	if err != nil {
		t.Fatalf("Demo failed: %v", err)
	}
}
