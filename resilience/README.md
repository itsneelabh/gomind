# GoMind Resilience Module

Bulletproof your agents with production-ready fault tolerance patterns.

## 🎯 What Does This Module Do?

Think of this module as the **safety equipment for your agents** - like airbags and seatbelts in a car. When things go wrong (and they will), this module ensures your system stays up and running instead of crashing.

It provides battle-tested patterns to handle failures gracefully:

1. **Circuit Breaker** - Stops cascading failures before they bring down your system
2. **Retry Logic** - Automatically retries failed operations with smart backoff
3. **Sliding Window Metrics** - Tracks success/failure rates in real-time

### Real-World Analogy: The Smart Power Strip

Imagine a smart power strip that:
- **Detects problems**: If a device keeps shorting, it cuts power (circuit breaker opens)
- **Tests recovery**: After a cooldown, it allows limited power to test if the problem is fixed (half-open state)
- **Restores service**: If tests succeed, full power resumes (circuit closes)
- **Retries intelligently**: If your phone doesn't charge, it tries again with increasing delays

That's exactly how this module protects your agent communications!

## 🚀 Quick Start

### Installation

```go
import "github.com/itsneelabh/gomind/resilience"
```

### Basic Circuit Breaker
```go
// Create a circuit breaker with default production settings
config := resilience.DefaultConfig()
config.Name = "payment-service"
cb := resilience.NewCircuitBreaker(config)

// Wrap any fallible operation
err := cb.Execute(ctx, func() error {
    return callPaymentService()
})

if err != nil {
    if errors.Is(err, core.ErrCircuitBreakerOpen) {
        // Circuit is open - service is down
        return fallbackPaymentMethod()
    }
    // Handle other errors
}
```

### Smart Retry with Backoff
```go
// Configure retry behavior
retryConfig := &resilience.RetryConfig{
    MaxAttempts:   3,
    InitialDelay:  100 * time.Millisecond,
    MaxDelay:      5 * time.Second,
    BackoffFactor: 2.0,      // Double delay each time
    JitterEnabled: true,      // Prevent thundering herd
}

// Retry with exponential backoff
err := resilience.Retry(ctx, retryConfig, func() error {
    return fetchDataFromAPI()
})
```

### Combining Circuit Breaker + Retry
```go
// Best of both worlds: retry with circuit breaker protection
err := resilience.RetryWithCircuitBreaker(
    ctx, 
    retryConfig, 
    cb,
    func() error {
        return riskyOperation()
    },
)
```

## 🧠 How It Works

### The Circuit Breaker State Machine

```
        ┌──────────────┐
        │    CLOSED    │ ← Normal operation
        │ (All good!)  │   All requests pass through
        └──────┬───────┘
               │
               │ Error rate > 50%
               │ (configurable)
               ▼
        ┌──────────────┐
        │     OPEN     │ ← Protection mode
        │ (Oh no!)     │   All requests fail fast
        └──────┬───────┘   
               │
               │ After 30 seconds
               │ (sleep window)
               ▼
        ┌──────────────┐
        │  HALF-OPEN   │ ← Testing recovery
        │ (Maybe ok?)  │   Limited requests to test
        └──────┬───────┘
               │
               │ Success rate > 60%
               │ (configurable)
               ▼
        Back to CLOSED
```

### Why Three States?

#### Closed State - "Everything is fine"
```javascript
Request comes in → Execute normally → Track success/failure
If error rate < 50%: Stay closed
If error rate >= 50%: Open the circuit!
```

#### Open State - "Houston, we have a problem"
```javascript
Request comes in → Immediately fail → Don't even try
Why? To give the failing service time to recover
After 30 seconds → Move to half-open
```

#### Half-Open State - "Is it safe yet?"
```javascript
Allow 5 test requests through
If 60% succeed → Close the circuit (all good!)
If not → Back to open (still broken)
```

### Smart Error Classification

Not all errors are equal! The circuit breaker is smart about what counts as a failure:

```go
// These DON'T trip the circuit (user/programming errors):
✅ Configuration errors (user's fault)
✅ Not found errors (404s)
✅ Invalid state errors (programming bugs)
✅ Context cancellation (client gave up)

// These DO trip the circuit (infrastructure problems):
❌ Network timeouts
❌ Connection refused  
❌ 500 Internal Server Errors
❌ Database connection failures
```

### The Sliding Window - Real-Time Metrics

Instead of counting all failures forever, we use a sliding window:

```
Time:  [====|====|====|====|====|====|====|====|====|====]
        ↑                                               ↑
     10 min ago                                      Now

Each segment tracks successes/failures
Old segments automatically expire
Gives accurate, real-time error rates
```

## 🔧 Advanced Configuration

### Production-Ready Circuit Breaker

```go
config := &resilience.CircuitBreakerConfig{
    // Identity
    Name: "user-service",
    
    // When to open (choose one approach)
    ErrorThreshold:   0.5,      // Open at 50% error rate
    VolumeThreshold:  10,       // Need 10+ requests to evaluate
    
    // Recovery timing
    SleepWindow:      30 * time.Second,  // Wait before half-open
    HalfOpenRequests: 5,                  // Test requests in half-open
    SuccessThreshold: 0.6,                // 60% success to close
    
    // Metrics window
    WindowSize:   60 * time.Second,      // Look at last minute
    BucketCount:  10,                     // 10 buckets of 6 seconds each
    
    // Customize what counts as failure
    ErrorClassifier: func(err error) bool {
        // Only infrastructure errors count
        return !errors.Is(err, ErrUserInput)
    },
    
    // Observability
    Logger:  myLogger,
    Metrics: prometheusCollector,
}

cb := resilience.NewCircuitBreaker(config)
```

### Retry Strategies

```go
// Conservative retry (for critical operations)
conservative := &resilience.RetryConfig{
    MaxAttempts:   2,
    InitialDelay:  1 * time.Second,
    MaxDelay:      5 * time.Second,
    BackoffFactor: 2.0,
    JitterEnabled: false,  // Predictable timing
}

// Aggressive retry (for eventual consistency)
aggressive := &resilience.RetryConfig{
    MaxAttempts:   5,
    InitialDelay:  50 * time.Millisecond,
    MaxDelay:      10 * time.Second,
    BackoffFactor: 3.0,
    JitterEnabled: true,  // Prevent thundering herd
}

// No retry (for user-facing operations)
noRetry := &resilience.RetryConfig{
    MaxAttempts: 1,
}
```

## 🎭 Real-World Examples

### Example 1: Protecting External API Calls

```go
type WeatherService struct {
    cb *resilience.CircuitBreaker
    rc *resilience.RetryConfig
}

func NewWeatherService() *WeatherService {
    // Circuit breaker for the weather API
    config := resilience.DefaultConfig()
    config.Name = "weather-api"
    config.ErrorThreshold = 0.3    // Open at 30% errors
    config.SleepWindow = 1 * time.Minute
    
    return &WeatherService{
        cb: resilience.NewCircuitBreaker(config),
        rc: resilience.DefaultRetryConfig(),
    }
}

func (w *WeatherService) GetWeather(city string) (*Weather, error) {
    var result *Weather
    
    err := resilience.RetryWithCircuitBreaker(
        context.Background(),
        w.rc,
        w.cb,
        func() error {
            resp, err := http.Get(fmt.Sprintf("https://api.weather.com/%s", city))
            if err != nil {
                return err
            }
            defer resp.Body.Close()
            
            // 5xx errors are retryable
            if resp.StatusCode >= 500 {
                return fmt.Errorf("server error: %d", resp.StatusCode)
            }
            
            // 4xx errors are not (don't waste retries on bad requests)
            if resp.StatusCode >= 400 {
                return fmt.Errorf("client error: %d", resp.StatusCode)
            }
            
            return json.NewDecoder(resp.Body).Decode(&result)
        },
    )
    
    if err != nil {
        // Fallback to cached data
        return w.getCachedWeather(city)
    }
    
    return result, nil
}
```

### Example 2: Database Connection Pool

```go
type ResilientDB struct {
    pool *sql.DB
    cb   *resilience.CircuitBreaker
}

func NewResilientDB(dsn string) (*ResilientDB, error) {
    pool, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, err
    }
    
    // Configure circuit breaker for database
    config := resilience.DefaultConfig()
    config.Name = "postgres-main"
    config.ErrorThreshold = 0.2     // Very sensitive - open at 20% errors
    config.VolumeThreshold = 5       // Quick detection
    config.SleepWindow = 10 * time.Second  // Fast recovery attempts
    
    return &ResilientDB{
        pool: pool,
        cb:   resilience.NewCircuitBreaker(config),
    }, nil
}

func (db *ResilientDB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
    var rows *sql.Rows
    
    err := db.cb.Execute(ctx, func() error {
        var err error
        rows, err = db.pool.QueryContext(ctx, query, args...)
        return err
    })
    
    if err != nil {
        if errors.Is(err, core.ErrCircuitBreakerOpen) {
            // Database is down - use read replica or cache
            return db.queryFromReplica(ctx, query, args...)
        }
        return nil, err
    }
    
    return rows, nil
}
```

### Example 3: Microservice Communication

```go
type OrderService struct {
    inventoryCB *resilience.CircuitBreaker
    paymentCB   *resilience.CircuitBreaker
    shippingCB  *resilience.CircuitBreaker
    retry       *resilience.RetryConfig
}

func (s *OrderService) ProcessOrder(order Order) error {
    // Step 1: Check inventory (critical - must succeed)
    err := resilience.RetryWithCircuitBreaker(
        ctx, s.retry, s.inventoryCB,
        func() error {
            return s.checkInventory(order.Items)
        },
    )
    if err != nil {
        return fmt.Errorf("inventory check failed: %w", err)
    }
    
    // Step 2: Process payment (critical - must succeed)
    err = resilience.RetryWithCircuitBreaker(
        ctx, s.retry, s.paymentCB,
        func() error {
            return s.processPayment(order.Payment)
        },
    )
    if err != nil {
        // Rollback inventory
        s.releaseInventory(order.Items)
        return fmt.Errorf("payment failed: %w", err)
    }
    
    // Step 3: Schedule shipping (can be async/eventual)
    go func() {
        resilience.RetryWithCircuitBreaker(
            context.Background(),
            &resilience.RetryConfig{
                MaxAttempts: 10,  // Keep trying
                MaxDelay: 1 * time.Hour,
            },
            s.shippingCB,
            func() error {
                return s.scheduleShipping(order)
            },
        )
    }()
    
    return nil
}
```

## 📊 Monitoring & Observability

### Built-in Metrics Interface

```go
type MyMetricsCollector struct {
    prometheus *prometheus.Registry
}

func (m *MyMetricsCollector) RecordSuccess(name string) {
    m.prometheus.Counter(name + "_success").Inc()
}

func (m *MyMetricsCollector) RecordFailure(name string, errorType string) {
    m.prometheus.Counter(name + "_failure").
        WithLabels("error", errorType).Inc()
}

func (m *MyMetricsCollector) RecordStateChange(name string, from, to string) {
    m.prometheus.Counter(name + "_state_change").
        WithLabels("from", from, "to", to).Inc()
}

// Use it
config.Metrics = &MyMetricsCollector{prometheus: promRegistry}
```

### Logging Integration

```go
type MyLogger struct {
    zap *zap.Logger
}

func (l *MyLogger) Error(msg string, fields map[string]interface{}) {
    l.zap.Error(msg, zap.Any("fields", fields))
}

func (l *MyLogger) Info(msg string, fields map[string]interface{}) {
    l.zap.Info(msg, zap.Any("fields", fields))
}

// Use it
config.Logger = &MyLogger{zap: logger}
```

### Dashboard Queries

```sql
-- Circuit breaker state changes
SELECT 
    name,
    from_state,
    to_state,
    count(*) as transitions,
    max(timestamp) as last_transition
FROM circuit_breaker_events
WHERE timestamp > NOW() - INTERVAL '1 hour'
GROUP BY name, from_state, to_state;

-- Error rates by service
SELECT
    service_name,
    SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as successes,
    SUM(CASE WHEN status = 'failure' THEN 1 ELSE 0 END) as failures,
    ROUND(100.0 * SUM(CASE WHEN status = 'failure' THEN 1 ELSE 0 END) / COUNT(*), 2) as error_rate
FROM service_calls
WHERE timestamp > NOW() - INTERVAL '5 minutes'
GROUP BY service_name
ORDER BY error_rate DESC;
```

## 🎮 Testing Your Resilience

### Unit Testing Circuit Breakers

```go
func TestCircuitBreakerOpensOnFailures(t *testing.T) {
    config := resilience.DefaultConfig()
    config.ErrorThreshold = 0.5
    config.VolumeThreshold = 4
    cb := resilience.NewCircuitBreaker(config)
    
    // Simulate failures
    for i := 0; i < 5; i++ {
        cb.Execute(context.Background(), func() error {
            return errors.New("failure")
        })
    }
    
    // Circuit should be open
    err := cb.Execute(context.Background(), func() error {
        return nil
    })
    
    assert.ErrorIs(t, err, core.ErrCircuitBreakerOpen)
}
```

### Integration Testing

```go
func TestResilientServiceIntegration(t *testing.T) {
    // Start a flaky test server
    callCount := 0
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        callCount++
        if callCount < 3 {
            // Fail first 2 attempts
            w.WriteHeader(500)
            return
        }
        w.WriteHeader(200)
        json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
    }))
    defer server.Close()
    
    // Create resilient client
    client := NewResilientClient(server.URL)
    
    // Should succeed after retries
    result, err := client.Get(context.Background())
    assert.NoError(t, err)
    assert.Equal(t, "ok", result["status"])
    assert.Equal(t, 3, callCount) // Verify it retried
}
```

### Chaos Testing

```go
// Inject failures to test resilience
type ChaosMiddleware struct {
    failureRate float64
    cb          *resilience.CircuitBreaker
}

func (c *ChaosMiddleware) Handle(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        err := c.cb.Execute(r.Context(), func() error {
            if rand.Float64() < c.failureRate {
                return errors.New("chaos injection")
            }
            next.ServeHTTP(w, r)
            return nil
        })
        
        if err != nil {
            w.WriteHeader(503)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "service unavailable",
                "circuit_state": c.cb.State().String(),
            })
        }
    })
}
```

## 🏗️ Common Patterns & Best Practices

### Pattern 1: Service Degradation

```go
func (s *SearchService) Search(query string) ([]Result, error) {
    // Try primary search service
    results, err := s.primarySearch(query)
    if err == nil {
        return results, nil
    }
    
    // If circuit is open, degrade gracefully
    if errors.Is(err, core.ErrCircuitBreakerOpen) {
        // Try backup service
        if results, err := s.backupSearch(query); err == nil {
            return results, nil
        }
        
        // Last resort: return cached results
        return s.getCachedResults(query), nil
    }
    
    return nil, err
}
```

### Pattern 2: Bulkhead Isolation

```go
// Isolate different operations with separate circuit breakers
type APIGateway struct {
    readCB   *resilience.CircuitBreaker  // For GET requests
    writeCB  *resilience.CircuitBreaker  // For POST/PUT/DELETE
    adminCB  *resilience.CircuitBreaker  // For admin operations
}

func (g *APIGateway) handleRequest(r *http.Request) error {
    var cb *resilience.CircuitBreaker
    
    switch r.Method {
    case "GET":
        cb = g.readCB
    case "POST", "PUT", "DELETE":
        cb = g.writeCB
    default:
        cb = g.adminCB
    }
    
    return cb.Execute(r.Context(), func() error {
        return g.forwardRequest(r)
    })
}
```

### Pattern 3: Cascading Timeouts

```go
func (s *Service) CallChain(ctx context.Context) error {
    // Total timeout for entire chain
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    
    // Service A gets 40% of remaining time
    ctxA, _ := context.WithTimeout(ctx, 4*time.Second)
    if err := s.callServiceA(ctxA); err != nil {
        return err
    }
    
    // Service B gets 30% of remaining time  
    ctxB, _ := context.WithTimeout(ctx, 3*time.Second)
    if err := s.callServiceB(ctxB); err != nil {
        return err
    }
    
    // Service C gets the rest
    return s.callServiceC(ctx)
}
```

## ⚡ Performance Tips

### 1. **Tune Your Thresholds**
```go
// For stable services
config.ErrorThreshold = 0.5    // 50% errors
config.VolumeThreshold = 20    // Need good sample size

// For flaky services
config.ErrorThreshold = 0.8    // 80% errors (more tolerant)
config.VolumeThreshold = 5     // Quick detection
```

### 2. **Use Force Controls in Emergencies**
```go
// Emergency controls
cb.ForceOpen()   // Immediately stop all traffic
cb.ForceClosed() // Override and allow all traffic
cb.Reset()       // Clear state and start fresh
```

### 3. **Optimize Window Size**
```go
// For high-traffic services (1000+ req/sec)
config.WindowSize = 10 * time.Second  // Short window
config.BucketCount = 10               // Fine granularity

// For low-traffic services (<10 req/sec)
config.WindowSize = 5 * time.Minute   // Longer window
config.BucketCount = 5                // Coarse granularity
```

## 🔍 Debugging Guide

### When Circuit Won't Close

```go
// Check the metrics
stats := cb.GetStats()
fmt.Printf("Success rate in half-open: %.2f%%\n", stats.HalfOpenSuccessRate)
fmt.Printf("Required rate: %.2f%%\n", config.SuccessThreshold * 100)

// Force close if needed
if manualOverrideAuthorized {
    cb.ForceClosed()
}
```

### When Circuit Opens Too Often

```go
// Adjust sensitivity
config.ErrorThreshold = 0.7     // Increase from 0.5
config.VolumeThreshold = 30     // Increase from 10

// Or use custom classifier
config.ErrorClassifier = func(err error) bool {
    // Ignore specific errors
    if strings.Contains(err.Error(), "rate limit") {
        return false  // Don't count rate limits
    }
    return resilience.DefaultErrorClassifier(err)
}
```

### Tracking Problem Services

```go
// Add logging
config.Logger = &VerboseLogger{}

// Monitor state changes
cb.OnStateChange(func(from, to CircuitState) {
    alert.Send(fmt.Sprintf("Circuit %s: %s -> %s", 
        config.Name, from, to))
})
```

## 🚦 Production Checklist

Before deploying:

- [ ] **Configure error thresholds** based on service SLA
- [ ] **Set appropriate timeouts** for your use case
- [ ] **Add monitoring** for circuit state changes
- [ ] **Test failure scenarios** in staging
- [ ] **Document fallback strategies** for when circuits open
- [ ] **Configure alerts** for prolonged open states
- [ ] **Review retry strategies** to avoid overwhelming services
- [ ] **Set up dashboards** for error rates and circuit states

## 💡 Pro Tips

### Tip 1: Different Configs for Different Environments

```go
func getCircuitBreakerConfig(env string) *resilience.CircuitBreakerConfig {
    base := resilience.DefaultConfig()
    
    switch env {
    case "development":
        base.ErrorThreshold = 0.8      // More tolerant
        base.SleepWindow = 5 * time.Second  // Quick recovery
    case "staging":
        base.ErrorThreshold = 0.6
        base.SleepWindow = 15 * time.Second
    case "production":
        base.ErrorThreshold = 0.5      // Strict
        base.SleepWindow = 30 * time.Second  // Careful recovery
    }
    
    return base
}
```

### Tip 2: Service-Specific Strategies

```go
// Payment service: Conservative
paymentCB := NewCircuitBreaker(&CircuitBreakerConfig{
    ErrorThreshold: 0.3,  // Very sensitive
    SleepWindow: 1 * time.Minute,  // Long recovery
    HalfOpenRequests: 1,  // Single test request
})

// Search service: Aggressive
searchCB := NewCircuitBreaker(&CircuitBreakerConfig{
    ErrorThreshold: 0.7,  // Tolerant
    SleepWindow: 5 * time.Second,  // Quick recovery
    HalfOpenRequests: 10,  // Multiple test requests
})

// Analytics service: Relaxed
analyticsCB := NewCircuitBreaker(&CircuitBreakerConfig{
    ErrorThreshold: 0.9,  // Very tolerant
    SleepWindow: 1 * time.Second,  // Instant recovery
    HalfOpenRequests: 20,  // Many test requests
})
```

### Tip 3: Combine with Other Patterns

```go
// Rate limiter + Circuit breaker + Retry
func (c *Client) Call(ctx context.Context) error {
    // First: Check rate limit
    if !c.rateLimiter.Allow() {
        return ErrRateLimited
    }
    
    // Second: Circuit breaker
    return resilience.RetryWithCircuitBreaker(
        ctx,
        c.retryConfig,
        c.circuitBreaker,
        func() error {
            // Third: Actual call with timeout
            ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
            defer cancel()
            return c.doCall(ctx)
        },
    )
}
```

## 🎓 Summary - What You've Learned

### This Module Gives You Three Superpowers:

#### 1. **Circuit Breaker** - The Guardian
- Prevents cascading failures
- Three intelligent states
- Self-healing with half-open testing
- Smart error classification

#### 2. **Retry Logic** - The Persistent Helper
- Exponential backoff
- Jitter for thundering herd prevention
- Context-aware cancellation
- Maximum delay caps

#### 3. **Sliding Window** - The Analyst
- Real-time success/failure tracking
- Time-skew protection
- Configurable granularity
- Automatic bucket rotation

### Quick Decision Guide

**Use Circuit Breaker when:**
- Calling external services
- Protecting databases
- Preventing cascade failures
- Need fast failure detection

**Use Retry when:**
- Handling transient failures
- Network calls might timeout
- Eventually consistent systems
- Idempotent operations

**Use Both when:**
- Mission-critical services
- Payment processing
- High-availability requirements
- Production microservices

### The Power of Resilience

Remember: **It's not about preventing failures, it's about handling them gracefully.**

Your services WILL fail. The network WILL have issues. Databases WILL go down. With this module, your system keeps running anyway.

---

**🎉 Congratulations!** You now have production-grade resilience patterns. Your agents can handle anything the internet throws at them!