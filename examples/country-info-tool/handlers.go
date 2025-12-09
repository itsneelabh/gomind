package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/itsneelabh/gomind/core"
)

func (c *CountryTool) handleCountryInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CountryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.sendError(w, "Invalid request format", http.StatusBadRequest, ErrCodeInvalidRequest)
		return
	}

	if req.Country == "" {
		c.sendError(w, "Country name is required", http.StatusBadRequest, ErrCodeInvalidRequest)
		return
	}

	c.Logger.InfoWithContext(ctx, "Fetching country info", map[string]interface{}{
		"country": req.Country,
	})

	result, err := c.callRestCountries(ctx, req.Country)
	if err != nil {
		c.Logger.ErrorWithContext(ctx, "RestCountries API failed", map[string]interface{}{
			"error":   err.Error(),
			"country": req.Country,
		})
		if strings.Contains(err.Error(), "not found") {
			c.sendError(w, fmt.Sprintf("Country '%s' not found", req.Country), http.StatusNotFound, ErrCodeCountryNotFound)
		} else {
			c.sendError(w, "Country service unavailable", http.StatusServiceUnavailable, ErrCodeServiceUnavailable)
		}
		return
	}

	c.Logger.InfoWithContext(ctx, "Country info retrieved", map[string]interface{}{
		"country":    result.Name,
		"population": result.Population,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(core.ToolResponse{
		Success: true,
		Data:    result,
	})
}

func (c *CountryTool) callRestCountries(ctx context.Context, country string) (*CountryResponse, error) {
	// https://restcountries.com/v3.1/name/japan?fields=name,capital,region,subregion,population,area,languages,timezones,currencies,flag,flags,cca2
	reqURL := fmt.Sprintf("%s/name/%s?fields=name,capital,region,subregion,population,area,languages,timezones,currencies,flag,flags,cca2",
		RestCountriesBaseURL, url.PathEscape(country))

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "GoMind-CountryTool/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("country not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp []RestCountriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp) == 0 {
		return nil, fmt.Errorf("country not found")
	}

	data := apiResp[0]

	result := &CountryResponse{
		Name:        data.Name.Common,
		OfficialN:   data.Name.Official,
		Region:      data.Region,
		Subregion:   data.Subregion,
		Population:  data.Population,
		Area:        data.Area,
		Timezones:   data.Timezones,
		Flag:        data.Flag,
		FlagURL:     data.Flags.PNG,
		CountryCode: data.CCA2,
	}

	if len(data.Capital) > 0 {
		result.Capital = data.Capital[0]
	}

	// Extract languages
	for _, lang := range data.Languages {
		result.Languages = append(result.Languages, lang)
	}

	// Extract currency (first one)
	for code, curr := range data.Currencies {
		result.Currency.Code = code
		result.Currency.Name = curr.Name
		result.Currency.Symbol = curr.Symbol
		break
	}

	return result, nil
}

func (c *CountryTool) sendError(w http.ResponseWriter, message string, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(core.ToolResponse{
		Success: false,
		Error: &core.ToolError{
			Code:      code,
			Message:   message,
			Retryable: strings.Contains(code, "UNAVAILABLE"),
		},
	})
}
