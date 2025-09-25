package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// WeatherTool is a focused tool that provides weather-related capabilities
// It demonstrates the passive tool pattern - can register but not discover
type WeatherTool struct {
	*core.BaseTool
	apiKey string
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
	w.RegisterCapability(core.Capability{
		Name:        "current_weather",
		Description: "Gets current weather conditions for a location",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     w.handleCurrentWeather,
	})

	// Capability 2: Weather forecast (auto-generated endpoint: /api/capabilities/forecast)
	w.RegisterCapability(core.Capability{
		Name:        "forecast",
		Description: "Gets 7-day weather forecast for a location",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     w.handleForecast,
	})

	// Capability 3: Weather alerts (custom endpoint)
	w.RegisterCapability(core.Capability{
		Name:        "alerts",
		Description: "Gets severe weather alerts for a location",
		Endpoint:    "/weather/alerts", // Custom endpoint
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     w.handleAlerts,
	})

	// Capability 4: Historical analysis (no handler = uses generic handler)
	w.RegisterCapability(core.Capability{
		Name:        "historical_analysis",
		Description: "Analyzes historical weather patterns",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		// No handler provided - framework provides generic response
	})
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

// handleCurrentWeather processes current weather requests
func (w *WeatherTool) handleCurrentWeather(rw http.ResponseWriter, r *http.Request) {
	// Log the request (framework auto-injects logger)
	w.Logger.Info("Processing current weather request", map[string]interface{}{
		"method": r.Method,
		"path":   r.URL.Path,
	})

	var req WeatherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Logger.Error("Failed to decode request", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Simulate API call (in real implementation, call external weather API)
	weather := w.simulateWeatherData(req.Location, "current")

	// Store in memory for caching (framework auto-injects memory)
	cacheKey := fmt.Sprintf("current:%s", strings.ToLower(req.Location))
	cacheData, _ := json.Marshal(weather)
	w.Memory.Set(r.Context(), cacheKey, string(cacheData), 5*time.Minute)

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(weather)

	w.Logger.Info("Current weather request completed", map[string]interface{}{
		"location": req.Location,
	})
}

// handleForecast processes forecast requests
func (w *WeatherTool) handleForecast(rw http.ResponseWriter, r *http.Request) {
	w.Logger.Info("Processing forecast request", map[string]interface{}{
		"method": r.Method,
	})

	var req WeatherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Default to 7 days if not specified
	if req.Days <= 0 {
		req.Days = 7
	}

	// Generate forecast data
	var forecasts []WeatherResponse
	for i := 0; i < req.Days; i++ {
		forecast := w.simulateWeatherData(req.Location, "forecast")
		forecast.Timestamp = time.Now().AddDate(0, 0, i).Format("2006-01-02")
		forecasts = append(forecasts, forecast)
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]interface{}{
		"location": req.Location,
		"days":     req.Days,
		"forecast": forecasts,
	})
}

// handleAlerts processes weather alert requests
func (w *WeatherTool) handleAlerts(rw http.ResponseWriter, r *http.Request) {
	location := r.URL.Query().Get("location")
	if location == "" {
		http.Error(rw, "location parameter is required", http.StatusBadRequest)
		return
	}

	// Simulate alert data
	alerts := []map[string]interface{}{
		{
			"type":        "thunderstorm",
			"severity":    "moderate",
			"description": "Thunderstorms possible this afternoon",
			"start_time":  time.Now().Format(time.RFC3339),
			"end_time":    time.Now().Add(4 * time.Hour).Format(time.RFC3339),
		},
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]interface{}{
		"location": location,
		"alerts":   alerts,
	})
}

// simulateWeatherData creates realistic weather data for demo purposes
func (w *WeatherTool) simulateWeatherData(location, requestType string) WeatherResponse {
	// Simulate different weather based on location
	baseTemp := 20.0
	if strings.Contains(strings.ToLower(location), "alaska") {
		baseTemp = -5.0
	} else if strings.Contains(strings.ToLower(location), "florida") {
		baseTemp = 28.0
	}

	// Add some variation for forecast vs current
	variation := 0.0
	if requestType == "forecast" {
		variation = float64((time.Now().Unix() % 10) - 5) // ±5°C variation
	}

	return WeatherResponse{
		Location:    location,
		Temperature: baseTemp + variation,
		Humidity:    65 + int(time.Now().Unix()%30), // 65-95%
		Condition:   "partly cloudy",
		WindSpeed:   10.5,
		Timestamp:   time.Now().Format(time.RFC3339),
		Source:      "weather-service-v1.0",
	}
}

func main() {
	// Create weather tool
	tool := NewWeatherTool()

	// Framework handles all the complexity
	framework, err := core.NewFramework(tool,
		// Core configuration
		core.WithName("weather-service"),
		core.WithPort(8080),
		core.WithNamespace("examples"),

		// Discovery configuration (tools can register but not discover)
		core.WithRedisURL(getEnvOrDefault("REDIS_URL", "redis://localhost:6379")),
		core.WithDiscovery(true, "redis"),

		// CORS for web access
		core.WithCORS([]string{"*"}, true),

		// Development mode for easier debugging
		core.WithDevelopmentMode(true),
	)
	if err != nil {
		log.Fatalf("Failed to create framework: %v", err)
	}

	log.Println("🌤️  Weather Service Tool Starting...")
	log.Println("📍 Endpoints available:")
	log.Println("   - POST /api/capabilities/current_weather")
	log.Println("   - POST /api/capabilities/forecast")
	log.Println("   - GET  /weather/alerts?location=<location>")
	log.Println("   - POST /api/capabilities/historical_analysis (generic handler)")
	log.Println("   - GET  /api/capabilities (list all capabilities)")
	log.Println("   - GET  /health (health check)")
	log.Println()
	log.Println("🔗 Test commands:")
	log.Println(`   curl -X POST http://localhost:8080/api/capabilities/current_weather \`)
	log.Println(`     -H "Content-Type: application/json" \`)
	log.Println(`     -d '{"location":"New York","units":"metric"}'`)
	log.Println()

	// Run the framework (blocking)
	ctx := context.Background()
	if err := framework.Run(ctx); err != nil {
		log.Fatalf("Framework execution failed: %v", err)
	}
}

// getEnvOrDefault gets environment variable with fallback
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getPortFromEnv gets port from environment with fallback
func getPortFromEnv() int {
	if portStr := os.Getenv("PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			return port
		}
	}
	return 8080
}