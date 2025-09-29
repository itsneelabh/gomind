package telemetry

import (
	"context"
	"github.com/itsneelabh/gomind/core"
)

// FrameworkMetricsRegistry implements core.MetricsRegistry
// This enables all framework components to emit metrics through telemetry
type FrameworkMetricsRegistry struct {
	logger *TelemetryLogger
}

// NewFrameworkMetricsRegistry creates a new framework metrics registry
func NewFrameworkMetricsRegistry(logger *TelemetryLogger) *FrameworkMetricsRegistry {
	return &FrameworkMetricsRegistry{
		logger: logger,
	}
}

// Counter implements core.MetricsRegistry
func (f *FrameworkMetricsRegistry) Counter(name string, labels ...string) {
	// Debug log framework emissions
	if f.logger != nil && f.logger.debug {
		f.logger.Debug("Framework metric emission", map[string]interface{}{
			"metric_name": name,
			"type":        "counter",
			"label_count": len(labels) / 2,
			"source":      "framework",
		})
	}

	// Delegate to telemetry's global emission
	Emit(name, 1.0, labels...)
}

// EmitWithContext implements core.MetricsRegistry
func (f *FrameworkMetricsRegistry) EmitWithContext(ctx context.Context, name string, value float64, labels ...string) {
	// Extract context for correlation
	baggage := GetBaggage(ctx)

	if f.logger != nil && f.logger.debug {
		// Log with context awareness
		requestID := ""
		if baggage != nil {
			if id, ok := baggage["request_id"]; ok {
				requestID = id
			}
		}

		f.logger.Debug("Framework context-aware emission", map[string]interface{}{
			"metric_name":  name,
			"value":        value,
			"has_baggage":  len(baggage) > 0,
			"request_id":   requestID,
			"label_count":  len(labels) / 2,
			"source":       "framework",
		})
	}

	// Use telemetry's context-aware emission
	EmitWithContext(ctx, name, value, labels...)
}

// GetBaggage implements core.MetricsRegistry
func (f *FrameworkMetricsRegistry) GetBaggage(ctx context.Context) map[string]string {
	// GetBaggage returns Baggage type (map[string]string), so direct conversion works
	return GetBaggage(ctx)
}

// EnableFrameworkIntegration registers the telemetry module with core
// This must be called after telemetry initialization to enable framework-wide metrics
func EnableFrameworkIntegration(logger *TelemetryLogger) {
	registry := NewFrameworkMetricsRegistry(logger)

	// Register with core to enable framework-wide metrics
	core.SetMetricsRegistry(registry)

	if logger != nil {
		logger.Info("Framework integration enabled", map[string]interface{}{
			"integration": "core.MetricsRegistry",
			"impact":      "All framework components can now emit metrics",
			"methods":     []string{"Counter", "EmitWithContext", "GetBaggage"},
		})
	}
}