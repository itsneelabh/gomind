// Package orchestration provides intelligent parameter binding for multi-step workflows.
//
// # LLM-First Parameter Resolution Design
//
// This file implements hybrid resolution where LLM micro-resolution is the PRIMARY
// approach for semantic parameter binding. Auto-wiring is now limited to trivial
// matching only (exact names, case-insensitive) to avoid unnecessary LLM calls for
// obvious cases.
//
// Design principles:
//  1. Framework agnosticism: No domain-specific heuristics (weather, currency, etc.)
//  2. LLM-powered semantics: All semantic understanding (e.g., "latitude" → "lat",
//     "country" → "EUR") is delegated to LLM micro-resolution
//  3. Cost optimization: Only use auto-wiring for trivial cases where names match exactly
//
// What auto-wiring handles (no LLM needed):
//   - Exact name match: "lat" → "lat"
//   - Case-insensitive match: "LAT" → "lat"
//   - Nested extraction: {code: "EUR"} → "EUR"
//   - Type coercion: "35.6" → 35.6
//
// What LLM micro-resolution handles:
//   - Semantic equivalence: "latitude" → "lat"
//   - Domain inference: "France" → "EUR" (currency)
//   - Complex mappings: Any case where names don't match
//
// This ensures the framework handles novel domains automatically without hardcoded
// mappings, while still avoiding wasteful LLM calls for trivial parameter binding.
package orchestration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/itsneelabh/gomind/core"
)

// HybridResolver combines auto-wiring and micro-resolution for parameter binding.
// It first attempts to auto-wire parameters by name matching, then falls back to
// LLM-based micro-resolution for any parameters that couldn't be matched.
type HybridResolver struct {
	autoWirer     *AutoWirer
	microResolver *MicroResolver
	logger        core.Logger

	// Configuration
	enableMicroResolution bool // Whether to use LLM fallback (default: true)
}

// HybridResolverOption configures a HybridResolver
type HybridResolverOption func(*HybridResolver)

// WithMicroResolution enables or disables LLM-based micro-resolution fallback
func WithMicroResolution(enabled bool) HybridResolverOption {
	return func(h *HybridResolver) {
		h.enableMicroResolution = enabled
	}
}

// NewHybridResolver creates a new hybrid resolver
func NewHybridResolver(aiClient core.AIClient, logger core.Logger, opts ...HybridResolverOption) *HybridResolver {
	h := &HybridResolver{
		autoWirer:             NewAutoWirer(logger),
		logger:                logger,
		enableMicroResolution: true, // Default: enable LLM fallback
	}

	// Create micro-resolver if AI client is provided
	if aiClient != nil {
		h.microResolver = NewMicroResolver(aiClient, logger)
	}

	// Apply options
	for _, opt := range opts {
		opt(h)
	}

	return h
}

// ResolveParameters resolves parameters from dependency results to target capability.
// This is the main entry point for parameter resolution in the executor.
//
// Resolution strategy (LLM-first design):
//  1. Auto-wire trivial matches (exact name, case-insensitive) - no LLM cost
//  2. If required parameters remain unmapped → use LLM micro-resolution
//  3. LLM handles all semantic understanding (e.g., "latitude" → "lat")
//
// This ensures domain-agnostic behavior while avoiding unnecessary LLM calls
// for trivial parameter mappings.
func (h *HybridResolver) ResolveParameters(
	ctx context.Context,
	dependencyResults map[string]*StepResult,
	targetCapability *EnhancedCapability,
) (map[string]interface{}, error) {
	// Collect source data from all dependencies
	sourceData := h.collectSourceData(dependencyResults)

	if len(sourceData) == 0 {
		h.logDebug("No source data available for parameter resolution", map[string]interface{}{
			"capability": targetCapability.Name,
		})
		return nil, nil // No dependencies have data
	}

	// Phase 1: Try auto-wiring (fast, no LLM cost)
	params, unmapped := h.autoWirer.AutoWireParameters(sourceData, targetCapability.Parameters)

	h.logDebug("Auto-wiring result", map[string]interface{}{
		"capability":  targetCapability.Name,
		"wired_count": len(params),
		"unmapped":    unmapped,
		"source_keys": getMapKeys(sourceData),
	})

	// Phase 2: If all required parameters resolved, we're done!
	if len(unmapped) == 0 {
		h.logInfo("All parameters auto-wired successfully", map[string]interface{}{
			"capability": targetCapability.Name,
			"params":     params,
		})
		return params, nil
	}

	// Check if all unmapped are optional
	allUnmappedOptional := true
	for _, paramName := range unmapped {
		for _, param := range targetCapability.Parameters {
			if param.Name == paramName && param.Required {
				allUnmappedOptional = false
				break
			}
		}
	}

	if allUnmappedOptional {
		h.logInfo("All required parameters auto-wired, optional params unmapped", map[string]interface{}{
			"capability":        targetCapability.Name,
			"params":            params,
			"optional_unmapped": unmapped,
		})
		return params, nil
	}

	// Phase 3: Use micro-resolution for remaining required parameters
	if !h.enableMicroResolution || h.microResolver == nil {
		h.logWarn("Micro-resolution disabled or unavailable, returning partial results", map[string]interface{}{
			"capability": targetCapability.Name,
			"params":     params,
			"unmapped":   unmapped,
		})
		return params, nil
	}

	h.logInfo("Using micro-resolution for unmapped parameters", map[string]interface{}{
		"capability": targetCapability.Name,
		"unmapped":   unmapped,
	})

	hint := fmt.Sprintf("Need to extract values for required parameters: %v", unmapped)
	resolved, err := h.microResolver.ResolveParameters(ctx, sourceData, targetCapability, hint)
	if err != nil {
		// Micro-resolution failed, return what we have
		h.logWarn("Micro-resolution failed, using partial auto-wired results", map[string]interface{}{
			"error":      err.Error(),
			"capability": targetCapability.Name,
			"params":     params,
		})
		return params, nil
	}

	// Merge results (auto-wired takes priority to avoid overwriting with LLM guesses)
	for k, v := range resolved {
		if _, exists := params[k]; !exists {
			params[k] = v
		}
	}

	h.logInfo("Hybrid resolution completed", map[string]interface{}{
		"capability":     targetCapability.Name,
		"final_params":   params,
		"auto_wired":     len(params) - len(resolved),
		"micro_resolved": len(resolved),
	})

	return params, nil
}

// collectSourceData merges all dependency results into a single map
func (h *HybridResolver) collectSourceData(dependencyResults map[string]*StepResult) map[string]interface{} {
	sourceData := make(map[string]interface{})

	for stepID, result := range dependencyResults {
		if result == nil || result.Response == "" {
			continue
		}
		if !result.Success {
			h.logDebug("Skipping failed step in source data", map[string]interface{}{
				"step_id": stepID,
			})
			continue
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result.Response), &parsed); err != nil {
			h.logWarn("Failed to parse step response for parameter resolution", map[string]interface{}{
				"step_id": stepID,
				"error":   err.Error(),
			})
			continue
		}

		// Merge into sourceData (later steps may override earlier for same keys)
		for k, v := range parsed {
			sourceData[k] = v
		}
	}

	return sourceData
}

// SetLogger sets the logger for the hybrid resolver and its sub-components
// The component is always set to "framework/orchestration" to ensure proper log attribution
// regardless of which agent or tool is using the orchestration module.
func (h *HybridResolver) SetLogger(logger core.Logger) {
	if logger == nil {
		h.logger = nil
	} else {
		if cal, ok := logger.(core.ComponentAwareLogger); ok {
			h.logger = cal.WithComponent("framework/orchestration")
		} else {
			h.logger = logger
		}
	}
	// Propagate to sub-components (they will apply their own WithComponent)
	if h.autoWirer != nil {
		h.autoWirer.SetLogger(logger)
	}
	if h.microResolver != nil {
		h.microResolver.SetLogger(logger)
	}
}

// ResolveSemanticValue resolves a single value semantically using LLM inference.
// This is used when template substitution fails (path doesn't exist) but we need to
// infer the intended value from available data.
// Example: template "{{step-2.response.data.country.currency}}" fails because
// geocoding returns country:"France" (string), not country:{currency:"EUR"}.
// This method can infer "EUR" from "France" using LLM.
func (h *HybridResolver) ResolveSemanticValue(
	ctx context.Context,
	sourceData map[string]interface{},
	paramName string,
	paramHint string,
	expectedType string,
) (interface{}, error) {
	if h.microResolver == nil || !h.enableMicroResolution {
		return nil, fmt.Errorf("micro-resolution not available")
	}

	// Create a minimal capability for single-value extraction
	targetCap := &EnhancedCapability{
		Name: "extract_value",
		Parameters: []Parameter{
			{
				Name:        paramName,
				Type:        expectedType,
				Required:    true,
				Description: paramHint,
			},
		},
	}

	h.logInfo("Semantic value resolution starting", map[string]interface{}{
		"param_name":  paramName,
		"param_hint":  paramHint,
		"source_keys": getMapKeys(sourceData),
	})

	resolved, err := h.microResolver.ResolveParameters(ctx, sourceData, targetCap, paramHint)
	if err != nil {
		h.logWarn("Semantic value resolution failed", map[string]interface{}{
			"error":      err.Error(),
			"param_name": paramName,
		})
		return nil, err
	}

	if val, ok := resolved[paramName]; ok {
		h.logInfo("Semantic value resolved successfully", map[string]interface{}{
			"param_name":     paramName,
			"resolved_value": val,
		})
		return val, nil
	}

	return nil, fmt.Errorf("parameter %s not found in micro-resolution result", paramName)
}

// Logging helpers
func (h *HybridResolver) logDebug(msg string, fields map[string]interface{}) {
	if h.logger != nil {
		h.logger.Debug(msg, fields)
	}
}

func (h *HybridResolver) logInfo(msg string, fields map[string]interface{}) {
	if h.logger != nil {
		h.logger.Info(msg, fields)
	}
}

func (h *HybridResolver) logWarn(msg string, fields map[string]interface{}) {
	if h.logger != nil {
		h.logger.Warn(msg, fields)
	}
}
