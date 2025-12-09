package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

func (n *NewsTool) handleSearchNews(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Add span event for Jaeger visibility
	telemetry.AddSpanEvent(ctx, "request_received",
		attribute.String("method", r.Method),
		attribute.String("path", r.URL.Path),
	)

	if n.apiKey == "" {
		n.sendError(w, "GNews API key not configured. Get one at https://gnews.io/", http.StatusServiceUnavailable, ErrCodeAPIKeyMissing)
		return
	}

	var req NewsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		n.sendError(w, "Invalid request format", http.StatusBadRequest, ErrCodeInvalidRequest)
		return
	}

	if req.Query == "" {
		n.sendError(w, "Query is required", http.StatusBadRequest, ErrCodeInvalidRequest)
		return
	}

	// Set defaults
	if req.MaxResults <= 0 || req.MaxResults > 10 {
		req.MaxResults = 5
	}
	if req.Language == "" {
		req.Language = "en"
	}

	n.Logger.InfoWithContext(ctx, "Searching news", map[string]interface{}{
		"query":       req.Query,
		"max_results": req.MaxResults,
		"language":    req.Language,
	})

	// Add span event before API call
	telemetry.AddSpanEvent(ctx, "calling_gnews_api",
		attribute.String("query", req.Query),
		attribute.Int("max_results", req.MaxResults),
		attribute.String("language", req.Language),
	)

	result, err := n.callGNewsSearch(ctx, req)
	if err != nil {
		// Record error on span for Jaeger visibility
		telemetry.RecordSpanError(ctx, err)
		n.Logger.ErrorWithContext(ctx, "GNews API failed", map[string]interface{}{
			"error": err.Error(),
			"query": req.Query,
		})
		if strings.Contains(err.Error(), "rate limit") || strings.Contains(err.Error(), "429") {
			n.sendError(w, "API rate limit exceeded (100 req/day on free tier)", http.StatusTooManyRequests, ErrCodeRateLimitExceeded)
		} else {
			n.sendError(w, "News service unavailable", http.StatusServiceUnavailable, ErrCodeServiceUnavailable)
		}
		return
	}

	n.Logger.InfoWithContext(ctx, "News search completed", map[string]interface{}{
		"query":          req.Query,
		"articles_found": len(result.Articles),
	})

	// Add success span event
	telemetry.AddSpanEvent(ctx, "news_retrieved",
		attribute.String("query", req.Query),
		attribute.Int("articles_found", len(result.Articles)),
		attribute.Int("total_articles", result.TotalArticles),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(core.ToolResponse{
		Success: true,
		Data:    result,
	})
}

func (n *NewsTool) callGNewsSearch(ctx context.Context, req NewsRequest) (*NewsResponse, error) {
	// Sanitize query for GNews API compatibility
	// GNews doesn't support commas in search queries - they cause syntax errors
	// Replace commas with spaces to preserve search intent (e.g., "Tokyo, Japan" -> "Tokyo Japan")
	sanitizedQuery := strings.ReplaceAll(req.Query, ",", " ")
	// Collapse multiple spaces that may result from comma removal
	sanitizedQuery = strings.Join(strings.Fields(sanitizedQuery), " ")

	// https://gnews.io/api/v4/search?q=Tokyo+travel&lang=en&max=5&apikey=YOUR_API_KEY
	reqURL := fmt.Sprintf("%s/search?q=%s&lang=%s&max=%d&apikey=%s",
		GNewsBaseURL, url.QueryEscape(sanitizedQuery), req.Language, req.MaxResults, n.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("User-Agent", "GoMind-NewsTool/1.0")

	resp, err := n.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limit exceeded (429)")
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("invalid API key")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp GNewsResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result := &NewsResponse{
		TotalArticles: apiResp.TotalArticles,
		Articles:      make([]Article, 0, len(apiResp.Articles)),
	}

	for _, a := range apiResp.Articles {
		result.Articles = append(result.Articles, Article{
			Title:       a.Title,
			Description: a.Description,
			URL:         a.URL,
			Source:      a.Source.Name,
			PublishedAt: a.PublishedAt,
			ImageURL:    a.Image,
		})
	}

	return result, nil
}

func (n *NewsTool) sendError(w http.ResponseWriter, message string, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(core.ToolResponse{
		Success: false,
		Error: &core.ToolError{
			Code:      code,
			Message:   message,
			Retryable: code == ErrCodeRateLimitExceeded || code == ErrCodeServiceUnavailable,
		},
	})
}
