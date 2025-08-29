# GoMind Resilience Module

The resilience module provides basic fault tolerance patterns for the GoMind framework. Currently implements circuit breaker and retry mechanisms to help agents handle failures gracefully.

## Table of Contents
- [Current Features](#current-features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Components](#components)
- [Examples](#examples)
- [API Reference](#api-reference)
- [Roadmap](#roadmap)
- [Contributing](#contributing)

## Current Features

✅ **Implemented:**
- Circuit breaker pattern with configurable thresholds
- Retry mechanism with exponential backoff and jitter
- Integration between circuit breaker and retry patterns
- Basic state management (Open/Closed states)
- Failure counting and automatic recovery

⚠️ **Limitations:**
- No rate limiting
- No bulkhead pattern
- No advanced timeout management
- No health check integration
- No metrics or monitoring hooks
- Limited configuration options

## Installation

```bash
go get github.com/itsneelabh/gomind/resilience
```

## Quick Start

### Using Circuit Breaker

```go
package main

import (
    "context"
    "time"
    "github.com/itsneelabh/gomind/resilience"
)

func main() {
    // Create circuit breaker
    // Parameters: failure threshold, timeout duration
    cb := resilience.NewCircuitBreaker(5, 30*time.Second)
    
    // Wrap your function call
    err := cb.Execute(context.Background(), func() error {
        // Your potentially failing operation
        return callExternalService()
    })
    
    if err != nil {
        // Handle error - might be original error or circuit open error
        log.Printf("Operation failed: %v", err)
    }
}
```

### Using Retry

```go
package main

import (
    "context"
    "github.com/itsneelabh/gomind/resilience"
)

func main() {
    // Create retry with max attempts
    retry := resilience.NewRetry(3)
    
    // Execute with automatic retries
    result, err := retry.Execute(context.Background(), func() (interface{}, error) {
        // Your operation that might need retrying
        return fetchData()
    })
    
    if err != nil {
        log.Printf("Failed after %d attempts: %v", retry.maxAttempts, err)
    }
}
```

### Combining Circuit Breaker with Retry

```go
package main

import (
    "context"
    "time"
    "github.com/itsneelabh/gomind/resilience"
)

func main() {
    // Create both patterns
    cb := resilience.NewCircuitBreaker(5, 30*time.Second)
    retry := resilience.NewRetry(3)
    
    // Use the combined helper function
    result, err := resilience.RetryWithCircuitBreaker(
        context.Background(),
        retry,
        cb,
        func() (interface{}, error) {
            return callService()
        },
    )
    
    if err != nil {
        log.Printf("Operation failed: %v", err)
    }
}
```

## Components

### 1. Circuit Breaker

The circuit breaker prevents cascading failures by monitoring operation failures and temporarily blocking calls when a threshold is exceeded.

**States:**
- **Closed**: Normal operation, requests pass through
- **Open**: Failure threshold exceeded, requests fail immediately

**Key Features:**
- Configurable failure threshold
- Automatic timeout-based recovery
- Thread-safe operation

```go
type CircuitBreaker struct {
    threshold      int           // Number of failures before opening
    timeout        time.Duration // How long to stay open
    failures       int           // Current failure count
    lastFailTime   time.Time     // Last failure timestamp
    state          State         // Current state (Open/Closed)
}
```

### 2. Retry

The retry mechanism automatically retries failed operations with exponential backoff and jitter to avoid thundering herd problems.

**Key Features:**
- Configurable maximum attempts
- Exponential backoff between retries
- Random jitter to spread retry load
- Context support for cancellation

```go
type Retry struct {
    maxAttempts int // Maximum number of retry attempts
}
```

**Backoff Calculation:**
- Base delay: 100ms * 2^(attempt-1)
- Random jitter: 0-100ms added
- Example: Attempt 1: ~100ms, Attempt 2: ~200ms, Attempt 3: ~400ms

## Examples

### Example 1: HTTP Client with Circuit Breaker

```go
func createResilientHTTPClient() *http.Client {
    cb := resilience.NewCircuitBreaker(3, 10*time.Second)
    
    return &http.Client{
        Transport: &resilientTransport{
            base: http.DefaultTransport,
            cb:   cb,
        },
    }
}

type resilientTransport struct {
    base http.RoundTripper
    cb   *resilience.CircuitBreaker
}

func (t *resilientTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    var resp *http.Response
    err := t.cb.Execute(req.Context(), func() error {
        var err error
        resp, err = t.base.RoundTrip(req)
        return err
    })
    return resp, err
}
```

### Example 2: Database Operations with Retry

```go
func queryWithRetry(db *sql.DB, query string) (*sql.Rows, error) {
    retry := resilience.NewRetry(3)
    
    result, err := retry.Execute(context.Background(), func() (interface{}, error) {
        return db.Query(query)
    })
    
    if err != nil {
        return nil, err
    }
    
    return result.(*sql.Rows), nil
}
```

### Example 3: Protecting External API Calls

```go
type APIClient struct {
    cb    *resilience.CircuitBreaker
    retry *resilience.Retry
}

func NewAPIClient() *APIClient {
    return &APIClient{
        cb:    resilience.NewCircuitBreaker(10, 1*time.Minute),
        retry: resilience.NewRetry(3),
    }
}

func (c *APIClient) CallAPI(endpoint string) ([]byte, error) {
    result, err := resilience.RetryWithCircuitBreaker(
        context.Background(),
        c.retry,
        c.cb,
        func() (interface{}, error) {
            resp, err := http.Get(endpoint)
            if err != nil {
                return nil, err
            }
            defer resp.Body.Close()
            
            if resp.StatusCode >= 500 {
                return nil, fmt.Errorf("server error: %d", resp.StatusCode)
            }
            
            return io.ReadAll(resp.Body)
        },
    )
    
    if err != nil {
        return nil, err
    }
    
    return result.([]byte), nil
}
```

## API Reference

### CircuitBreaker

| Method | Description |
|--------|-------------|
| `NewCircuitBreaker(threshold int, timeout time.Duration)` | Create new circuit breaker |
| `Execute(ctx context.Context, fn func() error)` | Execute function with circuit breaker protection |
| `GetState()` | Get current state (Open/Closed) |
| `Reset()` | Manually reset the circuit breaker |

### Retry

| Method | Description |
|--------|-------------|
| `NewRetry(maxAttempts int)` | Create new retry handler |
| `Execute(ctx context.Context, fn func() (interface{}, error))` | Execute function with retry logic |

### Helper Functions

| Function | Description |
|----------|-------------|
| `RetryWithCircuitBreaker(ctx, retry, cb, fn)` | Combine retry and circuit breaker patterns |

## Roadmap

### Near-term (Planned)
- [ ] Half-Open state for circuit breaker
- [ ] Configurable backoff strategies
- [ ] Success threshold for circuit recovery
- [ ] Metrics collection interface
- [ ] More granular error handling

### Medium-term (Under Consideration)
- [ ] Rate limiting with token bucket
- [ ] Bulkhead pattern for resource isolation
- [ ] Timeout wrapper with cascading cancellation
- [ ] Health check integration
- [ ] Fallback strategies

### Long-term (Future)
- [ ] Adaptive resilience based on system load
- [ ] Distributed circuit breaker state
- [ ] Advanced patterns (Retry with circuit breaker per endpoint)
- [ ] Integration with monitoring systems
- [ ] Configuration hot-reload

## Performance Considerations

- **Minimal Overhead**: Simple patterns with low CPU/memory usage
- **Thread-Safe**: All components are safe for concurrent use
- **No External Dependencies**: Pure Go implementation
- **Efficient Backoff**: Jitter prevents thundering herd

## Testing

```go
// Example test setup
func TestCircuitBreaker(t *testing.T) {
    cb := resilience.NewCircuitBreaker(2, 100*time.Millisecond)
    
    // Simulate failures
    for i := 0; i < 3; i++ {
        err := cb.Execute(context.Background(), func() error {
            return errors.New("simulated failure")
        })
        
        if i < 2 && err == nil {
            t.Error("Expected error")
        }
        if i == 2 && err != resilience.ErrCircuitOpen {
            t.Error("Expected circuit to be open")
        }
    }
    
    // Wait for timeout
    time.Sleep(150 * time.Millisecond)
    
    // Should work again
    err := cb.Execute(context.Background(), func() error {
        return nil
    })
    if err != nil {
        t.Error("Circuit should be closed after timeout")
    }
}
```

## Contributing

We welcome contributions! Current priorities:
1. Implementing Half-Open state for circuit breaker
2. Adding rate limiting
3. Implementing bulkhead pattern
4. Adding metrics hooks
5. Improving configuration options

Please ensure:
- All code includes tests
- Thread-safety is maintained
- Documentation is updated

## License

See the main GoMind repository for license information.