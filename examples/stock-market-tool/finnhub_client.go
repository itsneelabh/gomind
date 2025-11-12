package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	finnhubBaseURL = "https://finnhub.io/api/v1"
)

// FinnhubClient handles API communication with Finnhub.io
type FinnhubClient struct {
	apiKey     string
	httpClient *http.Client
}

// FinnhubQuote represents the raw API response for stock quote
type FinnhubQuote struct {
	C  float64 `json:"c"`  // Current price
	D  float64 `json:"d"`  // Change
	DP float64 `json:"dp"` // Percent change
	H  float64 `json:"h"`  // High price of the day
	L  float64 `json:"l"`  // Low price of the day
	O  float64 `json:"o"`  // Open price of the day
	PC float64 `json:"pc"` // Previous close price
	T  int64   `json:"t"`  // Timestamp
}

// FinnhubCompanyProfile represents the raw API response for company profile
type FinnhubCompanyProfile struct {
	Country              string  `json:"country"`
	Currency             string  `json:"currency"`
	Exchange             string  `json:"exchange"`
	FinnhubIndustry      string  `json:"finnhubIndustry"`
	IPO                  string  `json:"ipo"`
	Logo                 string  `json:"logo"`
	MarketCapitalization float64 `json:"marketCapitalization"`
	Name                 string  `json:"name"`
	Phone                string  `json:"phone"`
	ShareOutstanding     float64 `json:"shareOutstanding"`
	Ticker               string  `json:"ticker"`
	Weburl               string  `json:"weburl"`
}

// FinnhubNewsItem represents a single news article from the API
type FinnhubNewsItem struct {
	Category string `json:"category"`
	Datetime int64  `json:"datetime"`
	Headline string `json:"headline"`
	ID       int64  `json:"id"`
	Image    string `json:"image"`
	Related  string `json:"related"`
	Source   string `json:"source"`
	Summary  string `json:"summary"`
	URL      string `json:"url"`
}

// NewFinnhubClient creates a new Finnhub API client
func NewFinnhubClient(apiKey string) *FinnhubClient {
	return &FinnhubClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// GetStockQuote fetches real-time quote for a given symbol
func (c *FinnhubClient) GetStockQuote(ctx context.Context, symbol string) (*FinnhubQuote, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("Finnhub API key not configured")
	}

	endpoint := fmt.Sprintf("%s/quote", finnhubBaseURL)

	params := url.Values{}
	params.Add("symbol", symbol)
	params.Add("token", c.apiKey)

	fullURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var quote FinnhubQuote
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check if the quote has valid data (c = current price should be > 0)
	if quote.C == 0 {
		return nil, fmt.Errorf("invalid symbol or no data available")
	}

	return &quote, nil
}

// GetCompanyProfile fetches company information for a given symbol
func (c *FinnhubClient) GetCompanyProfile(ctx context.Context, symbol string) (*FinnhubCompanyProfile, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("Finnhub API key not configured")
	}

	endpoint := fmt.Sprintf("%s/stock/profile2", finnhubBaseURL)

	params := url.Values{}
	params.Add("symbol", symbol)
	params.Add("token", c.apiKey)

	fullURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var profile FinnhubCompanyProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check if profile has valid data
	if profile.Name == "" {
		return nil, fmt.Errorf("invalid symbol or no data available")
	}

	return &profile, nil
}

// GetCompanyNews fetches news articles for a specific company
func (c *FinnhubClient) GetCompanyNews(ctx context.Context, symbol, from, to string) ([]FinnhubNewsItem, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("Finnhub API key not configured")
	}

	// Default to last 7 days if dates not provided
	if from == "" || to == "" {
		now := time.Now()
		to = now.Format("2006-01-02")
		from = now.AddDate(0, 0, -7).Format("2006-01-02")
	}

	endpoint := fmt.Sprintf("%s/company-news", finnhubBaseURL)

	params := url.Values{}
	params.Add("symbol", symbol)
	params.Add("from", from)
	params.Add("to", to)
	params.Add("token", c.apiKey)

	fullURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var news []FinnhubNewsItem
	if err := json.NewDecoder(resp.Body).Decode(&news); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return news, nil
}

// GetMarketNews fetches general market news
func (c *FinnhubClient) GetMarketNews(ctx context.Context, category string) ([]FinnhubNewsItem, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("Finnhub API key not configured")
	}

	// Default to general if not specified
	if category == "" {
		category = "general"
	}

	endpoint := fmt.Sprintf("%s/news", finnhubBaseURL)

	params := url.Values{}
	params.Add("category", category)
	params.Add("token", c.apiKey)

	fullURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var news []FinnhubNewsItem
	if err := json.NewDecoder(resp.Body).Decode(&news); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return news, nil
}
