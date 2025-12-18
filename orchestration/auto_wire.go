// Package orchestration provides intelligent parameter binding for multi-step workflows.
// This file implements auto-wiring: automatic parameter resolution based on name matching
// and semantic aliases, without requiring LLM involvement.
package orchestration

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/itsneelabh/gomind/core"
)

// SemanticAliases is intentionally empty to maintain framework agnosticism.
// The framework relies on LLM micro-resolution for intelligent parameter binding
// rather than domain-specific hardcoded mappings.
//
// This design ensures:
// 1. Framework remains use-case agnostic (no weather, currency, stock assumptions)
// 2. Parameter binding is flexible and handles novel domains automatically
// 3. Tools self-describe their schemas; the LLM interprets them
//
// Auto-wiring still supports:
// - Exact name matching
// - Case-insensitive matching
// - Nested object extraction
// - Type coercion
//
// For semantic understanding (e.g., "latitude" → "lat"), use HybridResolver
// which falls back to LLM micro-resolution.
var SemanticAliases = map[string][]string{}

// AutoWirer handles automatic parameter resolution from source data
type AutoWirer struct {
	logger core.Logger
}

// NewAutoWirer creates a new auto-wirer instance
func NewAutoWirer(logger core.Logger) *AutoWirer {
	return &AutoWirer{logger: logger}
}

// SetLogger sets the logger for the auto-wirer
// The component is always set to "framework/orchestration" to ensure proper log attribution
// regardless of which agent or tool is using the orchestration module.
func (w *AutoWirer) SetLogger(logger core.Logger) {
	if logger == nil {
		w.logger = nil
	} else {
		if cal, ok := logger.(core.ComponentAwareLogger); ok {
			w.logger = cal.WithComponent("framework/orchestration")
		} else {
			w.logger = logger
		}
	}
}

// AutoWireParameters automatically maps source data to target parameters
// Returns the successfully wired parameters and a list of unmapped parameter names
func (w *AutoWirer) AutoWireParameters(
	sourceData map[string]interface{},
	targetParams []Parameter,
) (map[string]interface{}, []string) {
	result := make(map[string]interface{})
	unmapped := []string{}

	for _, param := range targetParams {
		value, found := w.findMatchingValue(sourceData, param.Name, param.Type)
		if found {
			result[param.Name] = value
			if w.logger != nil {
				w.logger.Debug("Auto-wired parameter", map[string]interface{}{
					"param_name":  param.Name,
					"param_type":  param.Type,
					"value":       value,
					"source_keys": getMapKeys(sourceData),
				})
			}
		} else {
			unmapped = append(unmapped, param.Name)
		}
	}

	return result, unmapped
}

// AutoWireFromMultipleSources merges data from multiple source results and performs auto-wiring
func (w *AutoWirer) AutoWireFromMultipleSources(
	sourceResults map[string]string, // stepID -> JSON response
	targetParams []Parameter,
) (map[string]interface{}, []string) {
	// Merge all source data
	mergedData := make(map[string]interface{})
	for stepID, response := range sourceResults {
		if response == "" {
			continue
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(response), &parsed); err != nil {
			if w.logger != nil {
				w.logger.Warn("Failed to parse source response for auto-wiring", map[string]interface{}{
					"step_id": stepID,
					"error":   err.Error(),
				})
			}
			continue
		}
		// Merge into combined data (later steps override earlier for same keys)
		for k, v := range parsed {
			mergedData[k] = v
		}
	}

	return w.AutoWireParameters(mergedData, targetParams)
}

// findMatchingValue searches for a value matching the parameter name in the source data
func (w *AutoWirer) findMatchingValue(data map[string]interface{}, paramName, paramType string) (interface{}, bool) {
	// Strategy 1: Exact name match
	if val, ok := data[paramName]; ok {
		extracted := extractNestedValue(val, paramType)
		return coerceType(extracted, paramType), true
	}

	// Strategy 2: Case-insensitive match
	paramLower := strings.ToLower(paramName)
	for key, val := range data {
		if strings.ToLower(key) == paramLower {
			extracted := extractNestedValue(val, paramType)
			return coerceType(extracted, paramType), true
		}
	}

	// Strategy 3: Semantic alias match
	aliases := getAliases(paramName)
	for _, alias := range aliases {
		// Check exact alias match
		if val, ok := data[alias]; ok {
			extracted := extractNestedValue(val, paramType)
			return coerceType(extracted, paramType), true
		}
		// Check case-insensitive alias match
		for key, val := range data {
			if strings.EqualFold(key, alias) {
				extracted := extractNestedValue(val, paramType)
				return coerceType(extracted, paramType), true
			}
		}
	}

	// Strategy 4: Search in nested "data" or "response" wrappers
	if dataWrapper, ok := data["data"].(map[string]interface{}); ok {
		if val, found := w.findMatchingValue(dataWrapper, paramName, paramType); found {
			return val, true
		}
	}
	if responseWrapper, ok := data["response"].(map[string]interface{}); ok {
		if val, found := w.findMatchingValue(responseWrapper, paramName, paramType); found {
			return val, true
		}
	}

	return nil, false
}

// extractNestedValue extracts the most appropriate value from nested objects.
// When the source value is a map (object) and target type is string, this function
// attempts to extract common nested fields like "code", "id", "name" that typically
// represent the canonical string identifier.
//
// Common patterns handled:
//   - currency: {"code": "EUR", "name": "Euro", "symbol": "€"} -> extracts "EUR"
//   - country: {"code": "FR", "name": "France"} -> extracts "FR"
//
// This enables proper parameter binding when tools return structured objects but
// downstream tools expect simple string values.
func extractNestedValue(val interface{}, targetType string) interface{} {
	// Only apply nested extraction when target is string and source is a map
	if strings.ToLower(targetType) != "string" {
		return val
	}

	mapVal, isMap := val.(map[string]interface{})
	if !isMap {
		return val
	}

	// Priority order for extracting string identifier from nested objects:
	// 1. "code" - most common for currency, country codes (ISO standards)
	// 2. "id" - common identifier field
	// 3. "value" - generic value field
	// 4. "name" - fallback to name if no code/id exists
	extractionPriority := []string{"code", "id", "value", "name"}

	for _, field := range extractionPriority {
		if nestedVal, exists := mapVal[field]; exists {
			// Only extract if the nested value is a string
			if strVal, isStr := nestedVal.(string); isStr {
				return strVal
			}
		}
	}

	// No suitable nested field found - return original value
	return val
}

// getAliases returns all known aliases for a parameter name
func getAliases(paramName string) []string {
	paramLower := strings.ToLower(paramName)

	// Check if paramName is a canonical name
	if aliases, ok := SemanticAliases[paramLower]; ok {
		return aliases
	}

	// Check if paramName is an alias of a canonical name
	for canonical, aliases := range SemanticAliases {
		for _, alias := range aliases {
			if strings.EqualFold(paramName, alias) {
				// Return all aliases for this canonical name
				return append(SemanticAliases[canonical], canonical)
			}
		}
	}

	return []string{}
}

// coerceType converts a value to the target type
func coerceType(val interface{}, targetType string) interface{} {
	switch strings.ToLower(targetType) {
	case "number", "float", "float64", "double":
		return toFloat64(val)
	case "integer", "int", "int64":
		return toInt64(val)
	case "string":
		return toString(val)
	case "boolean", "bool":
		return toBool(val)
	default:
		return val
	}
}

// toFloat64 converts various types to float64
func toFloat64(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0
		}
		return f
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}

// toInt64 converts various types to int64
func toInt64(val interface{}) int64 {
	switch v := val.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0
		}
		return i
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0
		}
		return i
	default:
		return 0
	}
}

// toString converts various types to string
func toString(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		// Remove quotes from simple values
		s := string(b)
		if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
		return s
	}
}

// toBool converts various types to bool
func toBool(val interface{}) bool {
	switch v := val.(type) {
	case bool:
		return v
	case string:
		lower := strings.ToLower(v)
		return lower == "true" || lower == "1" || lower == "yes" || lower == "on"
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

// === Standalone functions for simpler usage ===

// AutoWireParameters is a convenience function that creates a temporary AutoWirer
// and performs auto-wiring without requiring an instance
func AutoWireParameters(
	sourceData map[string]interface{},
	targetParams []Parameter,
) (map[string]interface{}, []string) {
	wirer := &AutoWirer{logger: nil}
	return wirer.AutoWireParameters(sourceData, targetParams)
}
