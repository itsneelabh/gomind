package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// handleGeocode processes forward geocoding requests (location -> coordinates)
func (g *GeocodingTool) handleGeocode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	g.Logger.InfoWithContext(ctx, "Processing geocode request", map[string]interface{}{
		"method": r.Method,
		"path":   r.URL.Path,
	})

	var req GeocodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.Logger.ErrorWithContext(ctx, "Failed to decode request", map[string]interface{}{
			"error": err.Error(),
		})
		g.sendError(w, "Invalid request format", http.StatusBadRequest, ErrCodeInvalidRequest)
		return
	}

	if req.Location == "" {
		g.sendError(w, "Location is required", http.StatusBadRequest, ErrCodeInvalidRequest)
		return
	}

	g.Logger.InfoWithContext(ctx, "Geocoding location", map[string]interface{}{
		"location": req.Location,
	})

	// Rate limit: Nominatim requires max 1 request per second
	// In production, use a proper rate limiter
	time.Sleep(100 * time.Millisecond)

	// Call Nominatim API
	result, err := g.callNominatimSearch(ctx, req.Location)
	if err != nil {
		g.Logger.ErrorWithContext(ctx, "Nominatim API call failed", map[string]interface{}{
			"error":    err.Error(),
			"location": req.Location,
		})
		g.handleNominatimError(w, err)
		return
	}

	g.Logger.InfoWithContext(ctx, "Geocoding successful", map[string]interface{}{
		"location":     req.Location,
		"lat":          result.Latitude,
		"lon":          result.Longitude,
		"display_name": result.DisplayName,
	})

	w.Header().Set("Content-Type", "application/json")
	response := core.ToolResponse{
		Success: true,
		Data:    result,
	}
	json.NewEncoder(w).Encode(response)
}

// handleReverseGeocode processes reverse geocoding requests (coordinates -> location)
func (g *GeocodingTool) handleReverseGeocode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	g.Logger.InfoWithContext(ctx, "Processing reverse geocode request", map[string]interface{}{
		"method": r.Method,
		"path":   r.URL.Path,
	})

	var req ReverseGeocodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.Logger.ErrorWithContext(ctx, "Failed to decode request", map[string]interface{}{
			"error": err.Error(),
		})
		g.sendError(w, "Invalid request format", http.StatusBadRequest, ErrCodeInvalidRequest)
		return
	}

	// Validate coordinates
	if req.Latitude < -90 || req.Latitude > 90 {
		g.sendError(w, "Latitude must be between -90 and 90", http.StatusBadRequest, ErrCodeInvalidRequest)
		return
	}
	if req.Longitude < -180 || req.Longitude > 180 {
		g.sendError(w, "Longitude must be between -180 and 180", http.StatusBadRequest, ErrCodeInvalidRequest)
		return
	}

	g.Logger.InfoWithContext(ctx, "Reverse geocoding coordinates", map[string]interface{}{
		"lat": req.Latitude,
		"lon": req.Longitude,
	})

	// Rate limit: Nominatim requires max 1 request per second
	time.Sleep(100 * time.Millisecond)

	// Call Nominatim reverse API
	result, err := g.callNominatimReverse(ctx, req.Latitude, req.Longitude)
	if err != nil {
		g.Logger.ErrorWithContext(ctx, "Nominatim reverse API call failed", map[string]interface{}{
			"error": err.Error(),
			"lat":   req.Latitude,
			"lon":   req.Longitude,
		})
		g.handleNominatimError(w, err)
		return
	}

	g.Logger.InfoWithContext(ctx, "Reverse geocoding successful", map[string]interface{}{
		"lat":          req.Latitude,
		"lon":          req.Longitude,
		"display_name": result.DisplayName,
	})

	w.Header().Set("Content-Type", "application/json")
	response := core.ToolResponse{
		Success: true,
		Data:    result,
	}
	json.NewEncoder(w).Encode(response)
}

// callNominatimSearch calls the Nominatim search API
func (g *GeocodingTool) callNominatimSearch(ctx context.Context, location string) (*GeocodeResponse, error) {
	// Build request URL
	// https://nominatim.openstreetmap.org/search?q=Tokyo&format=json&limit=1
	reqURL := fmt.Sprintf("%s/search?q=%s&format=json&limit=1&addressdetails=1",
		NominatimBaseURL, url.QueryEscape(location))

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Nominatim requires a valid User-Agent
	req.Header.Set("User-Agent", "GoMind-GeocodingTool/1.0 (https://github.com/itsneelabh/gomind)")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var results []NominatimResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("location not found: %s", location)
	}

	result := results[0]

	// Parse coordinates
	lat, _ := strconv.ParseFloat(result.Lat, 64)
	lon, _ := strconv.ParseFloat(result.Lon, 64)

	// Extract address components
	country := result.Address["country"]
	countryCode := result.Address["country_code"]
	city := extractCity(result.Address)
	state := result.Address["state"]

	return &GeocodeResponse{
		Latitude:    lat,
		Longitude:   lon,
		DisplayName: result.DisplayName,
		CountryCode: strings.ToLower(countryCode),
		Country:     country,
		City:        city,
		State:       state,
	}, nil
}

// callNominatimReverse calls the Nominatim reverse geocoding API
func (g *GeocodingTool) callNominatimReverse(ctx context.Context, lat, lon float64) (*GeocodeResponse, error) {
	// Build request URL
	// https://nominatim.openstreetmap.org/reverse?lat=35.6762&lon=139.6503&format=json
	reqURL := fmt.Sprintf("%s/reverse?lat=%f&lon=%f&format=json&addressdetails=1",
		NominatimBaseURL, lat, lon)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Nominatim requires a valid User-Agent
	req.Header.Set("User-Agent", "GoMind-GeocodingTool/1.0 (https://github.com/itsneelabh/gomind)")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result NominatimResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Parse coordinates from response
	parsedLat, _ := strconv.ParseFloat(result.Lat, 64)
	parsedLon, _ := strconv.ParseFloat(result.Lon, 64)

	// Extract address components
	country := result.Address["country"]
	countryCode := result.Address["country_code"]
	city := extractCity(result.Address)
	state := result.Address["state"]

	return &GeocodeResponse{
		Latitude:    parsedLat,
		Longitude:   parsedLon,
		DisplayName: result.DisplayName,
		CountryCode: strings.ToLower(countryCode),
		Country:     country,
		City:        city,
		State:       state,
	}, nil
}

// extractCity extracts city name from address, checking multiple possible fields
func extractCity(address map[string]string) string {
	// Nominatim uses different fields for city depending on location
	cityFields := []string{"city", "town", "village", "municipality", "locality"}
	for _, field := range cityFields {
		if city := address[field]; city != "" {
			return city
		}
	}
	return ""
}

// handleNominatimError sends appropriate error response based on the error
func (g *GeocodingTool) handleNominatimError(w http.ResponseWriter, err error) {
	errMsg := err.Error()

	switch {
	case strings.Contains(errMsg, "location not found"):
		g.sendError(w, errMsg, http.StatusNotFound, ErrCodeLocationNotFound)
	case strings.Contains(errMsg, "rate limit"):
		g.sendError(w, "Rate limit exceeded, please try again later", http.StatusTooManyRequests, ErrCodeRateLimitExceeded)
	default:
		g.sendError(w, "Geocoding service unavailable", http.StatusServiceUnavailable, ErrCodeServiceUnavailable)
	}
}

// sendError sends a structured error response
func (g *GeocodingTool) sendError(w http.ResponseWriter, message string, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := core.ToolResponse{
		Success: false,
		Error: &core.ToolError{
			Code:      code,
			Message:   message,
			Retryable: status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable,
		},
	}
	json.NewEncoder(w).Encode(response)
}
