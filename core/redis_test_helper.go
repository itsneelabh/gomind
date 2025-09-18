package core

import (
	"context"
	"net"
	"testing"
	"time"
)

// requireRedis checks if Redis is available and skips the test if not
// This provides consistent Redis availability checking across all tests
func requireRedis(t *testing.T) {
	t.Helper()

	// Skip in short mode (go test -short)
	if testing.Short() {
		t.Skip("Skipping Redis test in short mode")
	}

	// Quick connectivity check before attempting full Redis connection
	if !isRedisReachable() {
		t.Skip("Redis not available at localhost:6379 (connection refused)")
	}

	// Try to create a Redis discovery connection as final verification
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	discovery, err := NewRedisDiscovery("redis://localhost:6379")
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	// Verify we can actually use it
	if discovery != nil {
		// Try a simple operation to ensure Redis is responsive
		testInfo := &ServiceInfo{
			ID:   "redis-test-" + time.Now().Format("20060102-150405"),
			Name: "redis-availability-test",
			Type: ComponentTypeTool,
		}

		err = discovery.Register(ctx, testInfo)
		if err != nil {
			t.Skipf("Redis not responsive: %v", err)
		}

		// Clean up test data
		_ = discovery.Unregister(ctx, testInfo.ID)
	}
}

// isRedisReachable performs a quick TCP connection check
func isRedisReachable() bool {
	conn, err := net.DialTimeout("tcp", "localhost:6379", 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// requireRedisRegistry is similar to requireRedis but for Registry interface
func requireRedisRegistry(t *testing.T) Registry {
	t.Helper()
	requireRedis(t) // Performs the same availability checks

	registry, err := NewRedisRegistry("redis://localhost:6379")
	if err != nil {
		t.Skipf("Redis registry not available: %v", err)
	}

	return registry
}