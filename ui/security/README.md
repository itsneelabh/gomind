# GoMind UI Security Module - Zero-Bloat Optional Security

## Architecture Overview

This security module provides **optional, zero-overhead** security features for GoMind UI transports using the **Transport Wrapper Pattern** - the same pattern used by CircuitBreaker in the UI module.

## Key Design Principles

### 1. **Zero Overhead When Not Used**
```bash
# Default build - NO security code included
go build ./cmd/agent
# Binary size: ~7MB (no security code)

# With security features
go build -tags security ./cmd/agent  
# Binary size: ~7.2MB (minimal increase)
```

### 2. **No Changes to Existing Code**
The security features wrap existing transports without modifying ChatAgent or requiring any code changes:

```go
// Existing code remains unchanged
agent := ui.NewChatAgent("bot", aiClient, discovery)

// Security is applied as a wrapper if needed
if productionMode {
    securedSSE := security.WithSecurity(sseTransport, config)
    agent.ReplaceTransport("sse", securedSSE)
}
```

### 3. **Infrastructure-First Philosophy**
Security features automatically defer to infrastructure when present:

```go
type RateLimitConfig struct {
    SkipIfInfraProvided bool  // Skip if X-RateLimit headers present
}

type SecurityHeadersConfig struct {
    OnlySetMissing bool       // Only set headers not already present
}
```

## How It Works

### Transport Wrapper Pattern
Following the existing CircuitBreaker pattern in `ui/circuit_breaker.go`:

```go
// Original transport
sseTransport := &SSETransport{}

// Wrapped with rate limiting
rateLimited := NewRateLimitTransport(sseTransport, rateLimitConfig)

// Further wrapped with security headers
secured := NewSecurityHeadersTransport(rateLimited, headersConfig)

// Or use the convenience function
secured := WithSecurity(sseTransport, SecurityConfig{
    RateLimit: &rateLimitConfig,
    SecurityHeaders: &headersConfig,
})
```

### Build Tag Architecture

**With `security` build tag:**
- Full implementations in `security/*.go`
- Rate limiting using Redis (already required by framework)
- Security headers with CORS support
- ~200KB additional binary size

**Without `security` build tag:**
- Stub implementations in `security/stub.go`
- All functions return original transport unchanged
- Zero runtime overhead
- Zero binary size increase

## Usage Examples

### 1. Development (No Security)
```go
// No security imports or tags needed
agent := ui.NewChatAgent("bot", aiClient, discovery)
agent.Start(8080) // Works without any security
```

### 2. Production (With Security)
```go
//go:build security

import "github.com/itsneelabh/gomind/ui/security"

agent := ui.NewChatAgent("bot", aiClient, discovery)

// Apply security to all transports
for name, transport := range agent.GetTransports() {
    secured := security.WithSecurity(transport, security.DefaultSecurityConfig())
    agent.ReplaceTransport(name, secured)
}
```

### 3. Environment-Based Configuration
```go
// Security applies only when needed
if os.Getenv("ENABLE_SECURITY") == "true" {
    config := security.SecurityConfig{
        RateLimit: &security.RateLimitConfig{
            Enabled: true,
            RequestsPerMinute: 60,
            SkipIfInfraProvided: true, // Respect API Gateway
        },
    }
    // Apply security...
}
```

## Integration with Infrastructure

### Kubernetes Ingress Example
```yaml
# When using nginx-ingress with rate limiting
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/limit-rps: "10"
    nginx.ingress.kubernetes.io/enable-cors: "true"
spec:
  # ...
```

The framework security will automatically detect infrastructure headers and skip redundant processing.

### API Gateway Example
When deployed behind Kong, AWS API Gateway, or similar:
- Framework detects `X-RateLimit-*` headers
- Skips application-level rate limiting
- Only sets missing security headers

## Performance Characteristics

| Scenario | Overhead | Binary Size | Memory |
|----------|----------|-------------|---------|
| No security build tag | 0ns | 0KB | 0B |
| Security enabled, infra handles | ~10ns (header check) | +200KB | ~1KB/transport |
| Security enabled, framework handles | ~100μs (Redis call) | +200KB | ~10KB/transport |

## Why This Architecture?

### Advantages Over Framework-Level Implementation

1. **True Zero Overhead**: Code doesn't exist when not needed
2. **No API Changes**: Existing agents work unchanged
3. **Progressive Enhancement**: Add security without modifying code
4. **Infrastructure Friendly**: Respects enterprise patterns

### Advantages Over Pure Infrastructure

1. **Developer Experience**: Works locally without infrastructure
2. **Defense in Depth**: Framework provides baseline protection
3. **Portability**: Security travels with the code
4. **Flexibility**: Can override infrastructure when needed

## Comparison with Other Patterns

| Pattern | Bloat | Flexibility | Complexity |
|---------|-------|-------------|------------|
| **Middleware in ChatAgent** | Always present | Limited | Modifies core |
| **Separate Security Module** | Import complexity | High | Multiple modules |
| **Transport Wrapper (This)** | Zero when disabled | Maximum | Simple wrapper |
| **Infrastructure Only** | Zero | Limited | External config |

## Summary

This architecture provides:
- ✅ **Zero bloat** when not needed (build tags)
- ✅ **No code changes** required (wrapper pattern)  
- ✅ **Infrastructure respect** (skip if provided)
- ✅ **Progressive enhancement** (add when needed)
- ✅ **GoMind philosophy** (modular, optional, clean)

The security module follows established GoMind patterns (Transport Registry, CircuitBreaker) to provide optional security without compromising the framework's lightweight nature or forcing unnecessary complexity on users who don't need these features.