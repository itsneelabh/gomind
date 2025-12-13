package orchestration

import (
	"testing"
)

func TestAutoWireParameters_ExactNameMatch(t *testing.T) {
	sourceData := map[string]interface{}{
		"lat": 48.85,
		"lon": 2.35,
	}

	targetParams := []Parameter{
		{Name: "lat", Type: "number", Required: true},
		{Name: "lon", Type: "number", Required: true},
	}

	wirer := NewAutoWirer(nil)
	result, unmapped := wirer.AutoWireParameters(sourceData, targetParams)

	if len(unmapped) != 0 {
		t.Errorf("Expected no unmapped params, got %v", unmapped)
	}

	if result["lat"].(float64) != 48.85 {
		t.Errorf("Expected lat=48.85, got %v", result["lat"])
	}

	if result["lon"].(float64) != 2.35 {
		t.Errorf("Expected lon=2.35, got %v", result["lon"])
	}
}

// TestAutoWireParameters_SemanticMismatch verifies that auto-wiring does NOT
// perform semantic matching. The framework is domain-agnostic; semantic understanding
// (e.g., "latitude" → "lat") is delegated to LLM micro-resolution.
// See PARAMETER_BINDING_FIX.md for the LLM-first design rationale.
func TestAutoWireParameters_SemanticMismatch(t *testing.T) {
	sourceData := map[string]interface{}{
		"latitude":  48.85,
		"longitude": 2.35,
	}

	targetParams := []Parameter{
		{Name: "lat", Type: "number", Required: true},
		{Name: "lon", Type: "number", Required: true},
	}

	wirer := NewAutoWirer(nil)
	result, unmapped := wirer.AutoWireParameters(sourceData, targetParams)

	// Auto-wiring should NOT match "latitude" to "lat" - that's semantic understanding
	// which is handled by the LLM micro-resolver, not auto-wiring
	if len(unmapped) != 2 {
		t.Errorf("Expected 2 unmapped params (lat, lon), got %v", unmapped)
	}

	if len(result) != 0 {
		t.Errorf("Expected no auto-wired results for semantic mismatch, got %v", result)
	}
}

func TestAutoWireParameters_TypeCoercionFromString(t *testing.T) {
	sourceData := map[string]interface{}{
		"lat": "48.85",
		"lon": "2.35",
	}

	targetParams := []Parameter{
		{Name: "lat", Type: "number", Required: true},
		{Name: "lon", Type: "number", Required: true},
	}

	wirer := NewAutoWirer(nil)
	result, unmapped := wirer.AutoWireParameters(sourceData, targetParams)

	if len(unmapped) != 0 {
		t.Errorf("Expected no unmapped params, got %v", unmapped)
	}

	if result["lat"].(float64) != 48.85 {
		t.Errorf("Expected lat=48.85, got %v", result["lat"])
	}

	if result["lon"].(float64) != 2.35 {
		t.Errorf("Expected lon=2.35, got %v", result["lon"])
	}
}

func TestAutoWireParameters_CaseInsensitiveMatch(t *testing.T) {
	sourceData := map[string]interface{}{
		"LAT": 48.85,
		"LON": 2.35,
	}

	targetParams := []Parameter{
		{Name: "lat", Type: "number", Required: true},
		{Name: "lon", Type: "number", Required: true},
	}

	wirer := NewAutoWirer(nil)
	result, unmapped := wirer.AutoWireParameters(sourceData, targetParams)

	if len(unmapped) != 0 {
		t.Errorf("Expected no unmapped params, got %v", unmapped)
	}

	if result["lat"].(float64) != 48.85 {
		t.Errorf("Expected lat=48.85, got %v", result["lat"])
	}
}

func TestAutoWireParameters_UnmappedParams(t *testing.T) {
	sourceData := map[string]interface{}{
		"lat": 48.85,
	}

	targetParams := []Parameter{
		{Name: "lat", Type: "number", Required: true},
		{Name: "lon", Type: "number", Required: true},
		{Name: "unit", Type: "string", Required: false},
	}

	wirer := NewAutoWirer(nil)
	result, unmapped := wirer.AutoWireParameters(sourceData, targetParams)

	if len(unmapped) != 2 {
		t.Errorf("Expected 2 unmapped params, got %v", unmapped)
	}

	if result["lat"].(float64) != 48.85 {
		t.Errorf("Expected lat=48.85, got %v", result["lat"])
	}
}

func TestAutoWireParameters_NestedDataWrapper(t *testing.T) {
	sourceData := map[string]interface{}{
		"data": map[string]interface{}{
			"lat": 48.85,
			"lon": 2.35,
		},
	}

	targetParams := []Parameter{
		{Name: "lat", Type: "number", Required: true},
		{Name: "lon", Type: "number", Required: true},
	}

	wirer := NewAutoWirer(nil)
	result, unmapped := wirer.AutoWireParameters(sourceData, targetParams)

	if len(unmapped) != 0 {
		t.Errorf("Expected no unmapped params (should find in nested 'data'), got %v", unmapped)
	}

	if result["lat"].(float64) != 48.85 {
		t.Errorf("Expected lat=48.85, got %v", result["lat"])
	}
}

func TestAutoWireParameters_GeocodingToWeather(t *testing.T) {
	// Real-world test: geocoding response -> weather request
	geocodingResponse := map[string]interface{}{
		"lat":          35.6768601,
		"lon":          139.7638947,
		"display_name": "Tokyo, Japan",
		"type":         "city",
		"importance":   0.8,
	}

	weatherParams := []Parameter{
		{Name: "lat", Type: "number", Required: true, Description: "Latitude"},
		{Name: "lon", Type: "number", Required: true, Description: "Longitude"},
	}

	wirer := NewAutoWirer(nil)
	result, unmapped := wirer.AutoWireParameters(geocodingResponse, weatherParams)

	if len(unmapped) != 0 {
		t.Errorf("Expected all params wired for geocoding->weather, got unmapped: %v", unmapped)
	}

	lat := result["lat"].(float64)
	lon := result["lon"].(float64)

	if lat != 35.6768601 {
		t.Errorf("Expected lat=35.6768601, got %v", lat)
	}

	if lon != 139.7638947 {
		t.Errorf("Expected lon=139.7638947, got %v", lon)
	}
}

func TestAutoWireParameters_CountryInfoFromGeocoding(t *testing.T) {
	// Test semantic matching for country -> country_name
	geocodingResponse := map[string]interface{}{
		"lat":          48.8566,
		"lon":          2.3522,
		"display_name": "Paris, France",
		"country":      "France",
		"country_code": "FR",
	}

	countryParams := []Parameter{
		{Name: "country", Type: "string", Required: true, Description: "Country name"},
	}

	wirer := NewAutoWirer(nil)
	result, unmapped := wirer.AutoWireParameters(geocodingResponse, countryParams)

	if len(unmapped) != 0 {
		t.Errorf("Expected country to be wired, got unmapped: %v", unmapped)
	}

	if result["country"].(string) != "France" {
		t.Errorf("Expected country=France, got %v", result["country"])
	}
}

func TestAutoWireParameters_CurrencyFromCountryInfo(t *testing.T) {
	// Test semantic matching for currency
	countryResponse := map[string]interface{}{
		"name":          "France",
		"capital":       "Paris",
		"currency":      "EUR",
		"currency_name": "Euro",
		"population":    67390000,
	}

	currencyParams := []Parameter{
		{Name: "currency", Type: "string", Required: true, Description: "Currency code"},
	}

	wirer := NewAutoWirer(nil)
	result, unmapped := wirer.AutoWireParameters(countryResponse, currencyParams)

	if len(unmapped) != 0 {
		t.Errorf("Expected currency to be wired, got unmapped: %v", unmapped)
	}

	if result["currency"].(string) != "EUR" {
		t.Errorf("Expected currency=EUR, got %v", result["currency"])
	}
}

func TestCoerceType(t *testing.T) {
	tests := []struct {
		name       string
		input      interface{}
		targetType string
		expected   interface{}
	}{
		{"string to float", "48.85", "number", float64(48.85)},
		{"int to float", 48, "number", float64(48)},
		{"float to float", 48.85, "number", float64(48.85)},
		{"string to int", "42", "integer", int64(42)},
		{"float to int", 42.7, "integer", int64(42)},
		{"bool true", true, "boolean", true},
		{"string true", "true", "boolean", true},
		{"string 1", "1", "boolean", true},
		{"string false", "false", "boolean", false},
		{"int 0", 0, "boolean", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coerceType(tt.input, tt.targetType)
			if result != tt.expected {
				t.Errorf("coerceType(%v, %s) = %v, want %v", tt.input, tt.targetType, result, tt.expected)
			}
		})
	}
}

// TestGetAliases_EmptyByDesign verifies that SemanticAliases is empty.
// The framework is domain-agnostic; all semantic understanding is delegated
// to the LLM micro-resolver. See PARAMETER_BINDING_FIX.md for rationale.
func TestGetAliases_EmptyByDesign(t *testing.T) {
	// Verify the SemanticAliases map is empty (LLM-first design)
	if len(SemanticAliases) != 0 {
		t.Errorf("SemanticAliases should be empty for domain-agnostic design, got %d entries", len(SemanticAliases))
	}

	// getAliases should return empty for any parameter since there are no aliases
	tests := []string{"lat", "latitude", "lon", "longitude", "country", "currency", "foobar"}

	for _, paramName := range tests {
		t.Run(paramName, func(t *testing.T) {
			aliases := getAliases(paramName)
			if len(aliases) != 0 {
				t.Errorf("getAliases(%s) = %v, expected empty slice (LLM-first design)", paramName, aliases)
			}
		})
	}
}
