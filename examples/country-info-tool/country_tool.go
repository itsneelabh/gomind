package main

import (
	"net/http"
	"time"

	"github.com/itsneelabh/gomind/core"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// CountryTool provides country information using RestCountries API
type CountryTool struct {
	*core.BaseTool
	httpClient *http.Client
}

// CountryRequest represents the input for country info
type CountryRequest struct {
	Country string `json:"country"` // Country name or code
}

// CountryResponse represents country information
type CountryResponse struct {
	Name       string   `json:"name"`
	OfficialN  string   `json:"official_name"`
	Capital    string   `json:"capital"`
	Region     string   `json:"region"`
	Subregion  string   `json:"subregion"`
	Population int64    `json:"population"`
	Area       float64  `json:"area"`
	Languages  []string `json:"languages"`
	Timezones  []string `json:"timezones"`
	Currency   struct {
		Code   string `json:"code"`
		Name   string `json:"name"`
		Symbol string `json:"symbol"`
	} `json:"currency"`
	Flag       string `json:"flag"`
	FlagURL    string `json:"flag_url"`
	CountryCode string `json:"country_code"`
}

// RestCountriesResponse represents the API response
type RestCountriesResponse struct {
	Name struct {
		Common   string `json:"common"`
		Official string `json:"official"`
	} `json:"name"`
	Capital    []string           `json:"capital"`
	Region     string             `json:"region"`
	Subregion  string             `json:"subregion"`
	Population int64              `json:"population"`
	Area       float64            `json:"area"`
	Languages  map[string]string  `json:"languages"`
	Timezones  []string           `json:"timezones"`
	Currencies map[string]struct {
		Name   string `json:"name"`
		Symbol string `json:"symbol"`
	} `json:"currencies"`
	Flag  string `json:"flag"`
	Flags struct {
		PNG string `json:"png"`
		SVG string `json:"svg"`
	} `json:"flags"`
	CCA2 string `json:"cca2"`
}

const (
	ErrCodeCountryNotFound    = "COUNTRY_NOT_FOUND"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrCodeInvalidRequest     = "INVALID_REQUEST"
)

const RestCountriesBaseURL = "https://restcountries.com/v3.1"

func NewCountryTool() *CountryTool {
	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
	}

	tool := &CountryTool{
		BaseTool: core.NewTool("country-info-tool"),
		httpClient: &http.Client{
			Transport: otelhttp.NewTransport(transport),
			Timeout:   30 * time.Second,
		},
	}

	tool.registerCapabilities()
	return tool
}

func (c *CountryTool) registerCapabilities() {
	c.RegisterCapability(core.Capability{
		Name:        "get_country_info",
		Description: "Gets detailed information about a country including capital, population, languages, currency, timezones, and flag. Required: country (name or code).",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     c.handleCountryInfo,

		InputSummary: &core.SchemaSummary{
			RequiredFields: []core.FieldHint{
				{Name: "country", Type: "string", Example: "Japan", Description: "Country name or ISO code (e.g., Japan, JP, JPN)"},
			},
		},
	})
}
