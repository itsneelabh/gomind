package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// handleStockQuote processes stock quote requests
func (s *StockTool) handleStockQuote(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s.Logger.InfoWithContext(ctx, "Processing stock quote request", map[string]interface{}{
		"method": r.Method,
		"path":   r.URL.Path,
	})

	var req StockQuoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.Logger.ErrorWithContext(ctx, "Failed to decode request", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Normalize symbol to uppercase
	req.Symbol = strings.ToUpper(strings.TrimSpace(req.Symbol))

	s.Logger.InfoWithContext(ctx, "Received stock quote request", map[string]interface{}{
		"symbol": req.Symbol,
	})

	// Try to get real data from Finnhub API
	startTime := time.Now()
	quote, err := s.client.GetStockQuote(ctx, req.Symbol)
	apiDuration := time.Since(startTime)

	var response StockQuoteResponse

	if err != nil || quote == nil {
		// Fallback to mock data if API fails
		s.Logger.WarnWithContext(ctx, "Finnhub API call failed, using mock data", map[string]interface{}{
			"error":       err,
			"symbol":      req.Symbol,
			"api_latency": apiDuration.String(),
		})
		response = generateMockQuote(req.Symbol)
	} else {
		// Convert Finnhub response to our response format
		s.Logger.InfoWithContext(ctx, "Finnhub API call successful", map[string]interface{}{
			"symbol":      req.Symbol,
			"api_latency": apiDuration.String(),
		})
		response = StockQuoteResponse{
			Symbol:         req.Symbol,
			CurrentPrice:   quote.C,
			Change:         quote.D,
			PercentChange:  quote.DP,
			High:           quote.H,
			Low:            quote.L,
			Open:           quote.O,
			PreviousClose:  quote.PC,
			Timestamp:      quote.T,
			Source:         "Finnhub API",
		}
	}

	// Cache the result
	cacheKey := fmt.Sprintf("quote:%s", req.Symbol)
	cacheData, _ := json.Marshal(response)
	s.Memory.Set(ctx, cacheKey, string(cacheData), 1*time.Minute)

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)

	s.Logger.InfoWithContext(ctx, "Stock quote request completed", map[string]interface{}{
		"symbol":        req.Symbol,
		"current_price": response.CurrentPrice,
		"change":        response.Change,
		"source":        response.Source,
	})
}

// handleCompanyProfile processes company profile requests
func (s *StockTool) handleCompanyProfile(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s.Logger.InfoWithContext(ctx, "Processing company profile request", map[string]interface{}{
		"method": r.Method,
	})

	var req CompanyProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.Logger.ErrorWithContext(ctx, "Failed to decode request", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	req.Symbol = strings.ToUpper(strings.TrimSpace(req.Symbol))

	s.Logger.InfoWithContext(ctx, "Received company profile request", map[string]interface{}{
		"symbol": req.Symbol,
	})

	// Try to get real data from Finnhub API
	startTime := time.Now()
	profile, err := s.client.GetCompanyProfile(ctx, req.Symbol)
	apiDuration := time.Since(startTime)

	var response CompanyProfileResponse

	if err != nil || profile == nil {
		// Fallback to mock data if API fails
		s.Logger.WarnWithContext(ctx, "Finnhub API call failed, using mock data", map[string]interface{}{
			"error":       err,
			"symbol":      req.Symbol,
			"api_latency": apiDuration.String(),
		})
		response = generateMockProfile(req.Symbol)
	} else {
		// Convert Finnhub response to our response format
		s.Logger.InfoWithContext(ctx, "Finnhub API call successful", map[string]interface{}{
			"symbol":      req.Symbol,
			"api_latency": apiDuration.String(),
		})
		response = CompanyProfileResponse{
			Name:                 profile.Name,
			Ticker:               profile.Ticker,
			Exchange:             profile.Exchange,
			Industry:             profile.FinnhubIndustry,
			Country:              profile.Country,
			Currency:             profile.Currency,
			MarketCapitalization: profile.MarketCapitalization,
			IPO:                  profile.IPO,
			Website:              profile.Weburl,
			Logo:                 profile.Logo,
			Source:               "Finnhub API",
		}
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)

	s.Logger.InfoWithContext(ctx, "Company profile request completed", map[string]interface{}{
		"symbol":     req.Symbol,
		"name":       response.Name,
		"industry":   response.Industry,
		"market_cap": response.MarketCapitalization,
		"source":     response.Source,
	})
}

// handleCompanyNews processes company news requests
func (s *StockTool) handleCompanyNews(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s.Logger.InfoWithContext(ctx, "Processing company news request", map[string]interface{}{
		"method": r.Method,
	})

	var req CompanyNewsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.Logger.ErrorWithContext(ctx, "Failed to decode request", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	req.Symbol = strings.ToUpper(strings.TrimSpace(req.Symbol))

	s.Logger.InfoWithContext(ctx, "Received company news request", map[string]interface{}{
		"symbol": req.Symbol,
		"from":   req.From,
		"to":     req.To,
	})

	// Try to get real data from Finnhub API
	startTime := time.Now()
	newsItems, err := s.client.GetCompanyNews(ctx, req.Symbol, req.From, req.To)
	apiDuration := time.Since(startTime)

	var response CompanyNewsResponse

	if err != nil {
		// Fallback to mock data if API fails
		s.Logger.WarnWithContext(ctx, "Finnhub API call failed, using mock data", map[string]interface{}{
			"error":       err.Error(),
			"symbol":      req.Symbol,
			"api_latency": apiDuration.String(),
		})
		response = CompanyNewsResponse{
			Symbol: req.Symbol,
			News:   generateMockNews(req.Symbol, 5),
			From:   req.From,
			To:     req.To,
			Source: "Mock Data",
		}
	} else {
		// Convert Finnhub response to our response format
		s.Logger.InfoWithContext(ctx, "Finnhub API call successful", map[string]interface{}{
			"symbol":      req.Symbol,
			"news_count":  len(newsItems),
			"api_latency": apiDuration.String(),
		})

		news := make([]NewsItem, 0, len(newsItems))
		for _, item := range newsItems {
			news = append(news, NewsItem{
				Headline:  item.Headline,
				Summary:   item.Summary,
				Source:    item.Source,
				URL:       item.URL,
				Image:     item.Image,
				Published: item.Datetime,
			})
		}

		response = CompanyNewsResponse{
			Symbol: req.Symbol,
			News:   news,
			From:   req.From,
			To:     req.To,
			Source: "Finnhub API",
		}
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)

	s.Logger.InfoWithContext(ctx, "Company news request completed", map[string]interface{}{
		"symbol":     req.Symbol,
		"news_count": len(response.News),
		"source":     response.Source,
	})
}

// handleMarketNews processes market news requests
func (s *StockTool) handleMarketNews(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s.Logger.InfoWithContext(ctx, "Processing market news request", map[string]interface{}{
		"method": r.Method,
	})

	var req MarketNewsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.Logger.ErrorWithContext(ctx, "Failed to decode request", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	s.Logger.InfoWithContext(ctx, "Received market news request", map[string]interface{}{
		"category": req.Category,
	})

	// Try to get real data from Finnhub API
	startTime := time.Now()
	newsItems, err := s.client.GetMarketNews(ctx, req.Category)
	apiDuration := time.Since(startTime)

	var response MarketNewsResponse

	if err != nil {
		// Fallback to mock data if API fails
		s.Logger.WarnWithContext(ctx, "Finnhub API call failed, using mock data", map[string]interface{}{
			"error":       err.Error(),
			"api_latency": apiDuration.String(),
		})
		response = MarketNewsResponse{
			Category: req.Category,
			News:     generateMockNews("market", 10),
			Source:   "Mock Data",
		}
	} else {
		// Convert Finnhub response to our response format
		s.Logger.InfoWithContext(ctx, "Finnhub API call successful", map[string]interface{}{
			"news_count":  len(newsItems),
			"api_latency": apiDuration.String(),
		})

		news := make([]NewsItem, 0, len(newsItems))
		for _, item := range newsItems {
			news = append(news, NewsItem{
				Headline:  item.Headline,
				Summary:   item.Summary,
				Source:    item.Source,
				URL:       item.URL,
				Image:     item.Image,
				Published: item.Datetime,
			})
		}

		response = MarketNewsResponse{
			Category: req.Category,
			News:     news,
			Source:   "Finnhub API",
		}
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)

	s.Logger.InfoWithContext(ctx, "Market news request completed", map[string]interface{}{
		"category":   req.Category,
		"news_count": len(response.News),
		"source":     response.Source,
	})
}

// Mock data generators for fallback when API is unavailable

func generateMockQuote(symbol string) StockQuoteResponse {
	// Generate realistic but fake data
	basePrice := 100.0 + rand.Float64()*400.0
	change := (rand.Float64() - 0.5) * 10.0
	percentChange := (change / basePrice) * 100

	return StockQuoteResponse{
		Symbol:         symbol,
		CurrentPrice:   basePrice + change,
		Change:         change,
		PercentChange:  percentChange,
		High:           basePrice + change + rand.Float64()*5.0,
		Low:            basePrice + change - rand.Float64()*5.0,
		Open:           basePrice,
		PreviousClose:  basePrice,
		Timestamp:      time.Now().Unix(),
		Source:         "Mock Data (API key not configured)",
	}
}

func generateMockProfile(symbol string) CompanyProfileResponse {
	companies := map[string]string{
		"AAPL":  "Apple Inc.",
		"GOOGL": "Alphabet Inc.",
		"MSFT":  "Microsoft Corporation",
		"TSLA":  "Tesla Inc.",
		"AMZN":  "Amazon.com Inc.",
		"NVDA":  "NVIDIA Corporation",
	}

	name := companies[symbol]
	if name == "" {
		name = fmt.Sprintf("%s Corporation", symbol)
	}

	return CompanyProfileResponse{
		Name:                 name,
		Ticker:               symbol,
		Exchange:             "NASDAQ",
		Industry:             "Technology",
		Country:              "US",
		Currency:             "USD",
		MarketCapitalization: 1000000.0 + rand.Float64()*2000000.0,
		IPO:                  "2000-01-01",
		Website:              fmt.Sprintf("https://www.%s.com", strings.ToLower(symbol)),
		Logo:                 "",
		Source:               "Mock Data (API key not configured)",
	}
}

func generateMockNews(symbol string, count int) []NewsItem {
	news := make([]NewsItem, count)
	headlines := []string{
		"%s reports strong quarterly earnings",
		"%s announces new product launch",
		"Analysts upgrade %s stock rating",
		"%s expands into new markets",
		"CEO of %s discusses future strategy",
	}

	for i := 0; i < count; i++ {
		headline := fmt.Sprintf(headlines[i%len(headlines)], symbol)
		news[i] = NewsItem{
			Headline:  headline,
			Summary:   fmt.Sprintf("Mock news article about %s...", symbol),
			Source:    "Mock Financial News",
			URL:       "https://example.com/news",
			Published: time.Now().Add(-time.Duration(i) * time.Hour).Unix(),
		}
	}

	return news
}
