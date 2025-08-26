package agent

import (
	"context"
)

// Agent is the core interface that all agents must implement
// Framework provides zero-config auto-discovery and instrumentation
type Agent interface {
	// Optional: Initialize is called during agent startup
	// Framework provides auto-injection of dependencies via struct tags
	Initialize(ctx context.Context) error
}
