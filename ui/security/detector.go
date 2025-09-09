//go:build security
// +build security

// Package security provides infrastructure detection capabilities.
// This file implements detection logic to identify when security features
// are already provided by infrastructure components.
//
// Purpose:
// - Detects presence of API gateways, load balancers, and service meshes
// - Identifies when rate limiting is handled by infrastructure
// - Recognizes CORS headers set by upstream components
// - Prevents redundant security layers for optimal performance
//
// Scope:
// - InfrastructureDetector: Main detection component
// - Environment variable detection for common gateways
// - HTTP header analysis for proxy/gateway signatures
// - Service mesh detection (Istio, Envoy, Linkerd)
// - CORS and security header detection
//
// Detection Strategies:
// 1. Environment Variables: Check for gateway/proxy indicators
// 2. Request Headers: Identify signatures from known infrastructure
// 3. Response Headers: Detect existing security headers
// 4. Service Mesh: Recognize sidecar proxy patterns
//
// Supported Infrastructure:
// - API Gateways: AWS, Azure, GCP, Kong
// - Service Meshes: Istio, Envoy, Linkerd
// - Ingress Controllers: NGINX, Traefik, HAProxy
// - CDNs: CloudFlare, Fastly, Akamai
//
// Usage:
// The detector is used by SmartSecurityTransport to make intelligent
// decisions about which security features to apply based on what the
// infrastructure already provides.
package security

import (
	"net/http"
	"os"
	"strings"
)

// InfrastructureDetector detects if security is already handled by infrastructure
type InfrastructureDetector struct {
	// Detection strategies
	detectEnvVars     bool
	detectHeaders     bool
	detectServiceMesh bool
}

// NewInfrastructureDetector creates a detector with sensible defaults
func NewInfrastructureDetector() *InfrastructureDetector {
	return &InfrastructureDetector{
		detectEnvVars:     true,
		detectHeaders:     true,
		detectServiceMesh: true,
	}
}

// IsSecurityProvidedByInfra checks if infrastructure handles security
func (d *InfrastructureDetector) IsSecurityProvidedByInfra(req *http.Request) bool {
	// Check environment variables that indicate infrastructure security
	if d.detectEnvVars {
		// Common indicators that we're behind a gateway/proxy
		indicators := []string{
			"API_GATEWAY_ENABLED",
			"KONG_PROXY",
			"AWS_API_GATEWAY_ID",
			"AZURE_API_MANAGEMENT",
			"GCP_API_GATEWAY",
			"ISTIO_PROXY",
			"ENVOY_PROXY",
			"NGINX_INGRESS",
			"TRAEFIK_ENABLED",
		}

		for _, env := range indicators {
			if val := os.Getenv(env); val != "" && val != "false" && val != "0" {
				return true
			}
		}
	}

	// Check request headers that indicate infrastructure security
	if d.detectHeaders && req != nil {
		// Headers typically set by API gateways and proxies
		securityHeaders := []string{
			"X-RateLimit-Limit",        // Rate limiting by gateway
			"X-Kong-Proxy",             // Kong API Gateway
			"X-Amzn-Trace-Id",          // AWS API Gateway
			"X-Azure-Ref",              // Azure API Management
			"X-Google-Trace-ID",        // Google Cloud
			"X-Envoy-External-Address", // Envoy proxy
			"X-Forwarded-By",           // Generic proxy
			"X-API-Key",                // API key validation by gateway
			"X-Request-Id",             // Usually set by gateways
		}

		for _, header := range securityHeaders {
			if req.Header.Get(header) != "" {
				return true
			}
		}

		// Check for service mesh sidecar headers
		if d.detectServiceMesh {
			if req.Header.Get("X-B3-TraceId") != "" || // Istio/Envoy
				req.Header.Get("X-Request-ID") != "" || // Linkerd
				req.Header.Get("L5d-Ctx-Trace") != "" { // Linkerd specific
				return true
			}
		}
	}

	return false
}

// IsRateLimitingProvidedByInfra specifically checks for rate limiting
func (d *InfrastructureDetector) IsRateLimitingProvidedByInfra(req *http.Request) bool {
	if req == nil {
		return false
	}

	// Specific rate limiting headers
	rateLimitHeaders := []string{
		"X-RateLimit-Limit",
		"X-RateLimit-Remaining",
		"X-RateLimit-Reset",
		"RateLimit-Limit",
		"RateLimit-Remaining",
		"RateLimit-Reset",
		"X-Rate-Limit-Limit",
		"X-Rate-Limit-Remaining",
	}

	for _, header := range rateLimitHeaders {
		if req.Header.Get(header) != "" {
			return true
		}
	}

	// Check for rate limiting environment variables
	rateLimitEnvs := []string{
		"RATE_LIMITING_ENABLED",
		"API_GATEWAY_RATE_LIMIT",
		"ISTIO_RATE_LIMIT",
	}

	for _, env := range rateLimitEnvs {
		if val := os.Getenv(env); val != "" && val != "false" && val != "0" {
			return true
		}
	}

	return false
}

// IsCORSProvidedByInfra checks if CORS is handled by infrastructure
func (d *InfrastructureDetector) IsCORSProvidedByInfra(headers http.Header) bool {
	// If CORS headers are already set, infrastructure handles it
	corsHeaders := []string{
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Methods",
		"Access-Control-Allow-Headers",
	}

	for _, header := range corsHeaders {
		if headers.Get(header) != "" {
			return true
		}
	}

	// Check environment for CORS handling
	corsEnvs := []string{
		"CORS_ENABLED",
		"API_GATEWAY_CORS",
		"NGINX_CORS",
	}

	for _, env := range corsEnvs {
		if val := os.Getenv(env); strings.ToLower(val) == "true" || val == "1" {
			return true
		}
	}

	return false
}
