package main

import (
	"os"

	"github.com/itsneelabh/gomind/core"
)

// StockTool is a focused tool that provides stock market capabilities via Finnhub API
// It demonstrates the passive tool pattern - can register but not discover
type StockTool struct {
	*core.BaseTool
	apiKey string
	client *FinnhubClient
}

// StockQuoteRequest represents the input for stock quote requests
type StockQuoteRequest struct {
	Symbol string `json:"symbol"` // Stock ticker symbol (e.g., "AAPL", "GOOGL")
}

// CompanyProfileRequest represents the input for company profile requests
type CompanyProfileRequest struct {
	Symbol string `json:"symbol"` // Stock ticker symbol
}

// CompanyNewsRequest represents the input for company news requests
type CompanyNewsRequest struct {
	Symbol string `json:"symbol"`         // Stock ticker symbol
	From   string `json:"from,omitempty"` // Start date (YYYY-MM-DD)
	To     string `json:"to,omitempty"`   // End date (YYYY-MM-DD)
}

// MarketNewsRequest represents the input for market news requests
type MarketNewsRequest struct {
	Category string `json:"category,omitempty"` // News category (general, forex, crypto, merger)
}

// StockQuoteResponse represents the output for stock quote
type StockQuoteResponse struct {
	Symbol         string  `json:"symbol"`
	CurrentPrice   float64 `json:"current_price"`
	Change         float64 `json:"change"`
	PercentChange  float64 `json:"percent_change"`
	High           float64 `json:"high"`
	Low            float64 `json:"low"`
	Open           float64 `json:"open"`
	PreviousClose  float64 `json:"previous_close"`
	Timestamp      int64   `json:"timestamp"`
	Source         string  `json:"source"`
}

// CompanyProfileResponse represents the output for company profile
type CompanyProfileResponse struct {
	Name                 string  `json:"name"`
	Ticker               string  `json:"ticker"`
	Exchange             string  `json:"exchange"`
	Industry             string  `json:"industry"`
	Country              string  `json:"country"`
	Currency             string  `json:"currency"`
	MarketCapitalization float64 `json:"market_capitalization"`
	IPO                  string  `json:"ipo"`
	Website              string  `json:"website"`
	Logo                 string  `json:"logo"`
	Source               string  `json:"source"`
}

// NewsItem represents a single news article
type NewsItem struct {
	Headline  string `json:"headline"`
	Summary   string `json:"summary"`
	Source    string `json:"source"`
	URL       string `json:"url"`
	Image     string `json:"image,omitempty"`
	Published int64  `json:"published"`
}

// CompanyNewsResponse represents the output for company news
type CompanyNewsResponse struct {
	Symbol string     `json:"symbol"`
	News   []NewsItem `json:"news"`
	From   string     `json:"from,omitempty"`
	To     string     `json:"to,omitempty"`
	Source string     `json:"source"`
}

// MarketNewsResponse represents the output for market news
type MarketNewsResponse struct {
	Category string     `json:"category,omitempty"`
	News     []NewsItem `json:"news"`
	Source   string     `json:"source"`
}

// NewStockTool creates a new stock market analysis tool
func NewStockTool() *StockTool {
	apiKey := os.Getenv("FINNHUB_API_KEY")

	tool := &StockTool{
		BaseTool: core.NewTool("stock-service"),
		apiKey:   apiKey,
		client:   NewFinnhubClient(apiKey),
	}

	// Register multiple focused capabilities
	tool.registerCapabilities()
	return tool
}

// registerCapabilities sets up all stock market-related capabilities
func (s *StockTool) registerCapabilities() {
	// Capability 1: Stock Quote (real-time price data)
	// Auto-generated endpoint: /api/capabilities/stock_quote
	// Schema endpoint: /api/capabilities/stock_quote/schema
	s.RegisterCapability(core.Capability{
		Name:        "stock_quote",
		Description: "Gets real-time stock quote including current price, change, high, low, and trading volume. Required: symbol (stock ticker like AAPL, GOOGL, MSFT).",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     s.handleStockQuote,

		// Phase 2: Field hints for AI payload generation
		InputSummary: &core.SchemaSummary{
			RequiredFields: []core.FieldHint{
				{
					Name:        "symbol",
					Type:        "string",
					Example:     "AAPL",
					Description: "Stock ticker symbol (e.g., AAPL for Apple, GOOGL for Google)",
				},
			},
		},
	})

	// Capability 2: Company Profile
	// Auto-generated endpoint: /api/capabilities/company_profile
	s.RegisterCapability(core.Capability{
		Name:        "company_profile",
		Description: "Gets comprehensive company information including name, industry, market cap, IPO date, and website. Required: symbol (stock ticker).",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     s.handleCompanyProfile,

		// Phase 2: Field hints
		InputSummary: &core.SchemaSummary{
			RequiredFields: []core.FieldHint{
				{
					Name:        "symbol",
					Type:        "string",
					Example:     "TSLA",
					Description: "Stock ticker symbol for company information",
				},
			},
		},
	})

	// Capability 3: Company News
	// Auto-generated endpoint: /api/capabilities/company_news
	s.RegisterCapability(core.Capability{
		Name:        "company_news",
		Description: "Gets recent news articles for a specific company. Required: symbol (stock ticker). Optional: from and to dates (YYYY-MM-DD format).",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     s.handleCompanyNews,

		// Phase 2: Field hints
		InputSummary: &core.SchemaSummary{
			RequiredFields: []core.FieldHint{
				{
					Name:        "symbol",
					Type:        "string",
					Example:     "NVDA",
					Description: "Stock ticker symbol for company news",
				},
			},
			OptionalFields: []core.FieldHint{
				{
					Name:        "from",
					Type:        "string",
					Example:     "2024-01-01",
					Description: "Start date in YYYY-MM-DD format",
				},
				{
					Name:        "to",
					Type:        "string",
					Example:     "2024-01-31",
					Description: "End date in YYYY-MM-DD format",
				},
			},
		},
	})

	// Capability 4: Market News
	// Auto-generated endpoint: /api/capabilities/market_news
	s.RegisterCapability(core.Capability{
		Name:        "market_news",
		Description: "Gets general market news and headlines. Optional: category (general, forex, crypto, merger).",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     s.handleMarketNews,

		// Phase 2: Field hints
		InputSummary: &core.SchemaSummary{
			OptionalFields: []core.FieldHint{
				{
					Name:        "category",
					Type:        "string",
					Example:     "general",
					Description: "News category: general, forex, crypto, or merger",
				},
			},
		},
	})
}
