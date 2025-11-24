# Resilience Patterns

Unicorn provides built-in resilience patterns for building fault-tolerant applications.

## Overview

| Pattern | Use Case |
|---------|----------|
| **Circuit Breaker** | Prevent cascading failures to downstream services |
| **Retry** | Handle transient failures with exponential backoff |
| **Bulkhead** | Limit concurrent operations to prevent overload |
| **Timeout** | Enforce time limits on operations |
| **Fallback** | Provide graceful degradation |

## Circuit Breaker

Prevents cascading failures by stopping requests to a failing service.

### States

```
CLOSED  →  OPEN  →  HALF-OPEN  →  CLOSED
   ↑         │          │           │
   └─────────┴──────────┴───────────┘
```

- **CLOSED**: Normal operation, requests pass through
- **OPEN**: Circuit is open, requests fail immediately
- **HALF-OPEN**: Testing if service recovered

### Basic Usage

```go
import "github.com/madcok-co/unicorn/core/pkg/resilience"

cb := resilience.NewCircuitBreaker(&resilience.CircuitBreakerConfig{
    Name:        "payment-service",
    MaxRequests: 3,                    // Max requests in half-open
    Timeout:     30 * time.Second,     // Time before half-open
    ReadyToTrip: func(counts resilience.Counts) bool {
        return counts.ConsecutiveFailures > 5
    },
})

err := cb.Execute(func() error {
    return paymentService.Charge(amount)
})

if errors.Is(err, resilience.ErrCircuitOpen) {
    // Circuit is open, use fallback
    return handlePaymentFallback()
}
```

### Advanced Configuration

```go
cb := resilience.NewCircuitBreaker(&resilience.CircuitBreakerConfig{
    Name:        "external-api",
    MaxRequests: 3,
    Interval:    60 * time.Second,  // Clear counts every minute in closed state
    Timeout:     30 * time.Second,
    
    // Trip when failure rate > 50% or 5 consecutive failures
    ReadyToTrip: func(counts resilience.Counts) bool {
        failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
        return counts.Requests >= 10 && failureRate > 0.5 ||
               counts.ConsecutiveFailures > 5
    },
    
    // Custom success check (e.g., don't count 404 as failure)
    IsSuccessful: func(err error) bool {
        if err == nil {
            return true
        }
        var httpErr *HTTPError
        if errors.As(err, &httpErr) {
            return httpErr.StatusCode == 404  // 404 is not a failure
        }
        return false
    },
    
    // State change callback
    OnStateChange: func(name string, from, to resilience.State) {
        metrics.CircuitBreakerStateChange(name, to.String())
        if to == resilience.StateOpen {
            alerting.SendAlert("Circuit breaker %s opened", name)
        }
    },
})
```

### Circuit Breaker Registry

Manage multiple circuit breakers:

```go
registry := resilience.NewCircuitBreakerRegistry(&resilience.CircuitBreakerConfig{
    MaxRequests: 3,
    Timeout:     30 * time.Second,
    ReadyToTrip: func(counts resilience.Counts) bool {
        return counts.ConsecutiveFailures > 5
    },
})

// Get or create circuit breaker by name
cb := registry.Get("user-service")
err := cb.Execute(func() error {
    return userService.GetUser(userID)
})

// Get stats for monitoring
stats := registry.Stats()
for name, stat := range stats {
    fmt.Printf("%s: state=%s, failures=%d\n", 
        name, stat.State, stat.Counts.TotalFailures)
}
```

## Retry with Exponential Backoff

Handle transient failures automatically.

### Basic Usage

```go
retryer := resilience.NewRetryer(&resilience.RetryConfig{
    MaxAttempts:     3,
    InitialInterval: 100 * time.Millisecond,
    MaxInterval:     10 * time.Second,
    Multiplier:      2.0,
})

err := retryer.Do(func() error {
    return sendEmail(to, subject, body)
})
```

### With Context

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := retryer.DoWithContext(ctx, func(ctx context.Context) error {
    return httpClient.Do(ctx, request)
})
```

### Advanced Configuration

```go
retryer := resilience.NewRetryer(&resilience.RetryConfig{
    MaxAttempts:         5,
    InitialInterval:     100 * time.Millisecond,
    MaxInterval:         30 * time.Second,
    Multiplier:          2.0,
    RandomizationFactor: 0.5,  // Add jitter to prevent thundering herd
    
    // Only retry specific errors
    RetryIf: func(err error) bool {
        // Retry on network errors and 5xx responses
        var netErr net.Error
        if errors.As(err, &netErr) {
            return true
        }
        var httpErr *HTTPError
        if errors.As(err, &httpErr) {
            return httpErr.StatusCode >= 500
        }
        return false
    },
    
    // Callback on each retry
    OnRetry: func(attempt int, err error, delay time.Duration) {
        log.Warn("retrying operation",
            "attempt", attempt,
            "error", err,
            "next_delay", delay,
        )
    },
})
```

### Convenience Functions

```go
// Simple retry with defaults
err := resilience.Retry(func() error {
    return doSomething()
})

// Retry N times
err := resilience.RetryN(5, func() error {
    return doSomething()
})

// Retry with custom backoff
err := resilience.RetryWithBackoff(3, 100*time.Millisecond, 10*time.Second, func() error {
    return doSomething()
})
```

### Retry Conditions

```go
// Only retry on specific errors
config.RetryIf = resilience.RetryableErrors(
    io.ErrUnexpectedEOF,
    syscall.ECONNRESET,
)

// Don't retry on specific errors
config.RetryIf = resilience.NonRetryableErrors(
    ErrInvalidInput,
    ErrNotFound,
)
```

## Bulkhead

Limits concurrent executions to prevent overload.

```go
// Allow max 10 concurrent requests, 5s timeout to acquire slot
bulkhead := resilience.NewBulkhead(10, 5*time.Second)

err := bulkhead.Execute(func() error {
    return expensiveOperation()
})

if errors.Is(err, resilience.ErrBulkheadFull) {
    return ctx.Error(503, "Service overloaded")
}

// Check available slots
available := bulkhead.Available()
```

## Timeout

Enforce time limits on operations:

```go
// Simple timeout
err := resilience.WithTimeout(5*time.Second, func() error {
    return slowOperation()
})

// With parent context
err := resilience.WithTimeoutContext(ctx, 5*time.Second, func(ctx context.Context) error {
    return slowOperation(ctx)
})

if errors.Is(err, context.DeadlineExceeded) {
    return ctx.Error(504, "Operation timed out")
}
```

## Fallback

Provide graceful degradation:

```go
// Fallback function
result, err := resilience.WithFallback(
    func() (*User, error) {
        return userService.GetUser(userID)
    },
    func(err error) (*User, error) {
        // Return cached user on error
        return cache.GetUser(userID)
    },
)

// Fallback value
result, err := resilience.WithFallbackValue(
    func() (*Config, error) {
        return configService.GetConfig()
    },
    defaultConfig,  // Use default config on error
)
```

## Combining Patterns

### Circuit Breaker + Retry

```go
cb := resilience.NewCircuitBreaker(cbConfig)
retryer := resilience.NewRetryer(retryConfig)

// Combined execution
err := cb.ExecuteWithRetry(retryer, func() error {
    return externalService.Call()
})
```

### Full Resilience Stack

```go
func CallExternalService(ctx context.Context, req *Request) (*Response, error) {
    var resp *Response
    
    // 1. Bulkhead - limit concurrency
    err := bulkhead.ExecuteWithContext(ctx, func(ctx context.Context) error {
        
        // 2. Circuit breaker - prevent cascading failures
        return circuitBreaker.ExecuteWithContext(ctx, func(ctx context.Context) error {
            
            // 3. Retry - handle transient failures
            return retryer.DoWithContext(ctx, func(ctx context.Context) error {
                
                // 4. Timeout - enforce time limit
                return resilience.WithTimeoutContext(ctx, 5*time.Second, func(ctx context.Context) error {
                    var err error
                    resp, err = client.Call(ctx, req)
                    return err
                })
            })
        })
    })
    
    // 5. Fallback - graceful degradation
    if err != nil {
        return getFallbackResponse(req), nil
    }
    
    return resp, nil
}
```

## Best Practices

### Circuit Breaker

1. **Tune thresholds**: Start conservative, adjust based on metrics
2. **Use per-service breakers**: Different services have different failure patterns
3. **Monitor state changes**: Alert when circuits open
4. **Implement fallbacks**: Always have a fallback when circuit is open

### Retry

1. **Use exponential backoff**: Prevent overwhelming recovering services
2. **Add jitter**: Prevent thundering herd problem
3. **Set max attempts**: Don't retry forever
4. **Be selective**: Only retry transient failures
5. **Respect context**: Cancel retries when context is cancelled

### Bulkhead

1. **Size appropriately**: Based on downstream capacity
2. **Set reasonable timeout**: Don't wait forever for a slot
3. **Monitor rejections**: High rejections indicate capacity issues

### General

1. **Combine patterns**: Use circuit breaker + retry + timeout together
2. **Monitor everything**: Track success/failure rates, latencies
3. **Test failure scenarios**: Chaos engineering
4. **Document timeouts**: Make timeout budgets explicit

## Metrics & Monitoring

```go
// Circuit breaker metrics
cb.OnStateChange = func(name string, from, to State) {
    metrics.Gauge("circuit_breaker_state", 
        map[string]string{"name": name},
        float64(to),
    )
}

// Retry metrics
retryer.OnRetry = func(attempt int, err error, delay time.Duration) {
    metrics.Counter("retry_attempts",
        map[string]string{"operation": "external_call"},
        1,
    )
}

// Bulkhead metrics
metrics.Gauge("bulkhead_available", nil, float64(bulkhead.Available()))
```
