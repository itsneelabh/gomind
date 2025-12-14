package main

import (
	"net/http"
	"time"

	"github.com/itsneelabh/gomind/core"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// CurrencyTool provides currency conversion capabilities using Frankfurter API
type CurrencyTool struct {
	*core.BaseTool
	httpClient *http.Client
}

// ConvertRequest represents the input for currency conversion
type ConvertRequest struct {
	From   string  `json:"from"`   // Source currency code (e.g., "USD")
	To     string  `json:"to"`     // Target currency code (e.g., "JPY")
	Amount float64 `json:"amount"` // Amount to convert
}

// ConvertResponse represents the currency conversion result
type ConvertResponse struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Amount float64 `json:"amount"`
	Result float64 `json:"result"`
	Rate   float64 `json:"rate"`
	Date   string  `json:"date"`
}

// RatesRequest represents input for getting exchange rates
type RatesRequest struct {
	Base       string   `json:"base"`                 // Base currency (e.g., "USD")
	Currencies []string `json:"currencies,omitempty"` // Target currencies (empty = all)
}

// RatesResponse represents exchange rates result
type RatesResponse struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

// FrankfurterResponse represents the Frankfurter API response
type FrankfurterResponse struct {
	Amount float64            `json:"amount"`
	Base   string             `json:"base"`
	Date   string             `json:"date"`
	Rates  map[string]float64 `json:"rates"`
}

// Error codes for currency tool
const (
	ErrCodeInvalidCurrency    = "INVALID_CURRENCY"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrCodeInvalidRequest     = "INVALID_REQUEST"
)

// FrankfurterBaseURL is the base URL for Frankfurter API
const FrankfurterBaseURL = "https://api.frankfurter.app"

// NewCurrencyTool creates a new currency tool instance
func NewCurrencyTool() *CurrencyTool {
	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
	}

	tool := &CurrencyTool{
		BaseTool: core.NewTool("currency-tool"),
		httpClient: &http.Client{
			Transport: otelhttp.NewTransport(transport),
			Timeout:   30 * time.Second,
		},
	}

	tool.registerCapabilities()
	return tool
}

func (c *CurrencyTool) registerCapabilities() {
	// Capability 1: Convert currency
	c.RegisterCapability(core.Capability{
		Name:        "convert_currency",
		Description: "Converts an amount from one currency to another. Returns the converted amount and exchange rate. Required: from (source currency code), to (target currency code), amount (number to convert).",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     c.handleConvert,

		InputSummary: &core.SchemaSummary{
			RequiredFields: []core.FieldHint{
				{Name: "from", Type: "string", Example: "USD", Description: "Source currency code (e.g., USD, EUR, GBP). Should come from user request."},
				{Name: "to", Type: "string", Example: "EUR", Description: "Target currency code - use from country info's currency.code field (e.g., EUR, GBP). DO NOT use example values."},
				{Name: "amount", Type: "number", Example: "100", Description: "Amount to convert from user request"},
			},
		},
	})

	// Capability 2: Get exchange rates
	c.RegisterCapability(core.Capability{
		Name:        "get_exchange_rates",
		Description: "Gets current exchange rates for a base currency. Returns rates for all or specified currencies. Required: base (currency code). Optional: currencies (array of currency codes).",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     c.handleRates,

		InputSummary: &core.SchemaSummary{
			RequiredFields: []core.FieldHint{
				{Name: "base", Type: "string", Example: "USD", Description: "Base currency code"},
			},
			OptionalFields: []core.FieldHint{
				{Name: "currencies", Type: "array", Example: "[\"EUR\", \"GBP\", \"JPY\"]", Description: "Target currencies (empty = all)"},
			},
		},
	})
}
