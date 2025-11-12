package main

import (
	"os"

	"github.com/itsneelabh/gomind/core"
)

// WeatherTool is a focused tool that provides weather-related capabilities
// It demonstrates the passive tool pattern - can register but not discover
type WeatherTool struct {
	*core.BaseTool
	apiKey string
}

// WeatherRequest represents the input structure for weather requests
type WeatherRequest struct {
	Location string `json:"location"`
	Units    string `json:"units,omitempty"` // "metric" or "imperial"
	Days     int    `json:"days,omitempty"`  // For forecast
}

// WeatherResponse represents the output structure
type WeatherResponse struct {
	Location    string  `json:"location"`
	Temperature float64 `json:"temperature"`
	Humidity    int     `json:"humidity"`
	Condition   string  `json:"condition"`
	WindSpeed   float64 `json:"wind_speed"`
	Timestamp   string  `json:"timestamp"`
	Source      string  `json:"source"`
}

// NewWeatherTool creates a new weather analysis tool
func NewWeatherTool() *WeatherTool {
	tool := &WeatherTool{
		BaseTool: core.NewTool("weather-service"),
		apiKey:   os.Getenv("WEATHER_API_KEY"),
	}

	// Register multiple focused capabilities
	tool.registerCapabilities()
	return tool
}

// registerCapabilities sets up all weather-related capabilities
func (w *WeatherTool) registerCapabilities() {
	// Capability 1: Current weather (auto-generated endpoint: /api/capabilities/current_weather)
	// Phase 1: Description for AI-based generation
	// Phase 2: InputSummary with field hints for improved accuracy
	// Phase 3: Schema endpoint auto-generated at /api/capabilities/current_weather/schema
	w.RegisterCapability(core.Capability{
		Name:        "current_weather",
		Description: "Gets current weather conditions for a location. Required: location (city name). Optional: units (metric/imperial, default: metric).",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     w.handleCurrentWeather,

		// Phase 2: Compact field hints for AI payload generation
		InputSummary: &core.SchemaSummary{
			RequiredFields: []core.FieldHint{
				{
					Name:        "location",
					Type:        "string",
					Example:     "London",
					Description: "City name or coordinates (lat,lon)",
				},
			},
			OptionalFields: []core.FieldHint{
				{
					Name:        "units",
					Type:        "string",
					Example:     "metric",
					Description: "Temperature unit: metric or imperial",
				},
			},
		},
	})

	// Capability 2: Weather forecast (auto-generated endpoint: /api/capabilities/forecast)
	w.RegisterCapability(core.Capability{
		Name:        "forecast",
		Description: "Gets 7-day weather forecast for a location. Required: location (city name). Optional: days (number of days, default: 7), units (metric/imperial).",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     w.handleForecast,

		// Phase 2: Field hints for forecast capability
		InputSummary: &core.SchemaSummary{
			RequiredFields: []core.FieldHint{
				{
					Name:        "location",
					Type:        "string",
					Example:     "New York",
					Description: "City name or coordinates",
				},
			},
			OptionalFields: []core.FieldHint{
				{
					Name:        "days",
					Type:        "number",
					Example:     "7",
					Description: "Number of forecast days (1-14)",
				},
				{
					Name:        "units",
					Type:        "string",
					Example:     "metric",
					Description: "Temperature unit: metric or imperial",
				},
			},
		},
	})

	// Capability 3: Weather alerts (custom endpoint)
	w.RegisterCapability(core.Capability{
		Name:        "alerts",
		Description: "Gets severe weather alerts for a location",
		Endpoint:    "/weather/alerts", // Custom endpoint
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     w.handleAlerts,
		// Note: This capability uses query parameters, so no InputSummary needed
	})

	// Capability 4: Historical analysis (no handler = uses generic handler)
	w.RegisterCapability(core.Capability{
		Name:        "historical_analysis",
		Description: "Analyzes historical weather patterns. Required: location (city name), start_date, end_date (YYYY-MM-DD format).",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		// No handler provided - framework provides generic response

		// Phase 2: Field hints even without custom handler (helps agent generate correct payloads)
		InputSummary: &core.SchemaSummary{
			RequiredFields: []core.FieldHint{
				{
					Name:        "location",
					Type:        "string",
					Example:     "Tokyo",
					Description: "City name for historical analysis",
				},
				{
					Name:        "start_date",
					Type:        "string",
					Example:     "2024-01-01",
					Description: "Start date in YYYY-MM-DD format",
				},
				{
					Name:        "end_date",
					Type:        "string",
					Example:     "2024-01-31",
					Description: "End date in YYYY-MM-DD format",
				},
			},
		},
	})
}
