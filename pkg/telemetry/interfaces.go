package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// AutoOTEL interface defines telemetry functionality
type AutoOTEL interface {
	CreateSpanWithCapability(ctx context.Context, capability CapabilityMetadata) (context.Context, trace.Span)
	RecordCapabilityMetrics(ctx context.Context, capability CapabilityMetadata, duration time.Duration, err error)
	Shutdown(ctx context.Context) error
}
