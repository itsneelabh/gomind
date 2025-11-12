package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OpenWeatherMap API response structure
type OpenWeatherResponse struct {
	Main struct {
		Temp     float64 `json:"temp"`
		Humidity int     `json:"humidity"`
	} `json:"main"`
	Weather []struct {
		Description string `json:"description"`
	} `json:"weather"`
	Wind struct {
		Speed float64 `json:"speed"`
	} `json:"wind"`
	Name string `json:"name"`
	DT   int64  `json:"dt"`
}

// fetchRealWeatherData calls OpenWeatherMap API for real weather data
func (w *WeatherTool) fetchRealWeatherData(ctx context.Context, location, units string) (WeatherResponse, error) {
	if w.apiKey == "" {
		return WeatherResponse{}, fmt.Errorf("WEATHER_API_KEY not configured")
	}

	// Default to metric if not specified
	if units == "" {
		units = "metric"
	}

	// Build API URL with proper query parameter encoding
	apiURL := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=%s",
		url.QueryEscape(location), w.apiKey, units)

	// Log the API call (without exposing the full API key)
	maskedKey := "***" + w.apiKey[len(w.apiKey)-4:]
	w.Logger.InfoWithContext(ctx, "Calling OpenWeatherMap API", map[string]interface{}{
		"location": location,
		"units":    units,
		"api_key":  maskedKey,
		"url":      fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=%s", url.QueryEscape(location), maskedKey, units),
	})

	// Make HTTP request
	resp, err := http.Get(apiURL)
	if err != nil {
		return WeatherResponse{}, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		w.Logger.ErrorWithContext(ctx, "OpenWeatherMap API returned error", map[string]interface{}{
			"status_code": resp.StatusCode,
			"location":    location,
			"error_body":  string(body),
		})
		return WeatherResponse{}, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp OpenWeatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		w.Logger.ErrorWithContext(ctx, "Failed to parse OpenWeatherMap API response", map[string]interface{}{
			"error":    err.Error(),
			"location": location,
		})
		return WeatherResponse{}, fmt.Errorf("failed to parse API response: %w", err)
	}

	// Convert to our response format
	condition := "clear"
	if len(apiResp.Weather) > 0 {
		condition = apiResp.Weather[0].Description
	}

	result := WeatherResponse{
		Location:    apiResp.Name,
		Temperature: apiResp.Main.Temp,
		Humidity:    apiResp.Main.Humidity,
		Condition:   condition,
		WindSpeed:   apiResp.Wind.Speed,
		Timestamp:   time.Unix(apiResp.DT, 0).Format(time.RFC3339),
		Source:      "OpenWeatherMap API",
	}

	// Log the parsed response
	w.Logger.InfoWithContext(ctx, "Successfully parsed OpenWeatherMap API response", map[string]interface{}{
		"location":    result.Location,
		"temperature": result.Temperature,
		"condition":   result.Condition,
		"humidity":    result.Humidity,
	})

	return result, nil
}

// simulateWeatherData creates realistic weather data for demo purposes (fallback)
// Used when API key is not configured
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
		Source:      "weather-service-v1.0 (simulated)",
	}
}
