package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// handleCurrentWeather processes current weather requests
func (w *WeatherTool) handleCurrentWeather(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Log the request (framework auto-injects logger)
	// Using context-aware logging for distributed tracing and request correlation
	w.Logger.InfoWithContext(ctx, "Processing current weather request", map[string]interface{}{
		"method": r.Method,
		"path":   r.URL.Path,
	})

	var req WeatherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Logger.ErrorWithContext(ctx, "Failed to decode request", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Log the incoming request details
	w.Logger.InfoWithContext(ctx, "Received weather request", map[string]interface{}{
		"location": req.Location,
		"units":    req.Units,
	})

	// Try to get real weather data from API
	startTime := time.Now()
	weather, err := w.fetchRealWeatherData(ctx, req.Location, req.Units)
	apiDuration := time.Since(startTime)
	if err != nil {
		// Fallback to simulated data if API fails
		w.Logger.WarnWithContext(ctx, "Weather API call failed, using simulated data", map[string]interface{}{
			"error":       err.Error(),
			"location":    req.Location,
			"api_latency": apiDuration.String(),
		})
		weather = w.simulateWeatherData(req.Location, "current")
	} else {
		// Log successful API call
		w.Logger.InfoWithContext(ctx, "Weather API call successful", map[string]interface{}{
			"location":    req.Location,
			"api_latency": apiDuration.String(),
			"source":      weather.Source,
		})
	}

	// Store in memory for caching (framework auto-injects memory)
	cacheKey := fmt.Sprintf("current:%s", strings.ToLower(req.Location))
	cacheData, _ := json.Marshal(weather)
	w.Memory.Set(ctx, cacheKey, string(cacheData), 5*time.Minute)

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(weather)

	// Log the response data
	w.Logger.InfoWithContext(ctx, "Current weather request completed", map[string]interface{}{
		"location":    req.Location,
		"temperature": weather.Temperature,
		"condition":   weather.Condition,
		"humidity":    weather.Humidity,
		"source":      weather.Source,
	})
}

// handleForecast processes forecast requests
func (w *WeatherTool) handleForecast(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	w.Logger.InfoWithContext(ctx, "Processing forecast request", map[string]interface{}{
		"method": r.Method,
	})

	var req WeatherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Logger.ErrorWithContext(ctx, "Failed to decode forecast request", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Default to 7 days if not specified
	if req.Days <= 0 {
		req.Days = 7
	}

	// Log the incoming request details
	w.Logger.InfoWithContext(ctx, "Received forecast request", map[string]interface{}{
		"location": req.Location,
		"days":     req.Days,
		"units":    req.Units,
	})

	// Generate forecast data
	var forecasts []WeatherResponse
	for i := 0; i < req.Days; i++ {
		forecast := w.simulateWeatherData(req.Location, "forecast")
		forecast.Timestamp = time.Now().AddDate(0, 0, i).Format("2006-01-02")
		forecasts = append(forecasts, forecast)
	}

	response := map[string]interface{}{
		"location": req.Location,
		"days":     req.Days,
		"forecast": forecasts,
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)

	// Log the response summary
	w.Logger.InfoWithContext(ctx, "Forecast request completed", map[string]interface{}{
		"location":      req.Location,
		"days":          req.Days,
		"forecast_days": len(forecasts),
	})
}

// handleAlerts processes weather alert requests
func (w *WeatherTool) handleAlerts(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	location := r.URL.Query().Get("location")
	if location == "" {
		w.Logger.WarnWithContext(ctx, "Alert request missing location parameter", nil)
		http.Error(rw, "location parameter is required", http.StatusBadRequest)
		return
	}

	w.Logger.InfoWithContext(ctx, "Received weather alerts request", map[string]interface{}{
		"location": location,
	})

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

	response := map[string]interface{}{
		"location": location,
		"alerts":   alerts,
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)

	// Log the response summary
	w.Logger.InfoWithContext(ctx, "Weather alerts request completed", map[string]interface{}{
		"location":    location,
		"alert_count": len(alerts),
		"has_alerts":  len(alerts) > 0,
	})
}
