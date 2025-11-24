# Observability

This document covers metrics, tracing, and logging in the Unicorn framework.

## Overview

Unicorn provides three pillars of observability:

| Pillar | Purpose | Interface |
|--------|---------|-----------|
| **Metrics** | Quantitative measurements | `contracts.Metrics` |
| **Tracing** | Request flow tracking | `contracts.Tracer` |
| **Logging** | Event recording | `contracts.Logger` |

## Metrics

### Metrics Contract

```go
type Metrics interface {
    // Counter for cumulative values
    Counter(name string, labels ...string) Counter
    
    // Gauge for current values
    Gauge(name string, labels ...string) Gauge
    
    // Histogram for distributions
    Histogram(name string, buckets []float64, labels ...string) Histogram
    
    // Summary for percentiles
    Summary(name string, objectives map[float64]float64, labels ...string) Summary
}

type Counter interface {
    Inc()
    Add(float64)
}

type Gauge interface {
    Set(float64)
    Inc()
    Dec()
    Add(float64)
    Sub(float64)
}

type Histogram interface {
    Observe(float64)
}
```

### Using Metrics

```go
func CreateUser(ctx *unicorn.Context, req CreateUserRequest) (*User, error) {
    metrics := ctx.Metrics()
    
    // Count requests
    metrics.Counter("users_created_total", "status").Inc()
    
    // Track active users
    metrics.Gauge("active_users").Inc()
    
    // Measure duration
    start := time.Now()
    user, err := userService.Create(req)
    duration := time.Since(start).Seconds()
    
    metrics.Histogram("user_creation_duration_seconds", 
        []float64{0.01, 0.05, 0.1, 0.5, 1.0}).Observe(duration)
    
    if err != nil {
        metrics.Counter("users_created_total", "error").Inc()
        return nil, err
    }
    
    metrics.Counter("users_created_total", "success").Inc()
    return user, nil
}
```

### Common Metrics Patterns

```go
// HTTP request metrics
metrics.Counter("http_requests_total", "method", "path", "status")
metrics.Histogram("http_request_duration_seconds", defaultBuckets)

// Database metrics
metrics.Counter("db_queries_total", "operation", "table")
metrics.Histogram("db_query_duration_seconds", defaultBuckets)

// Cache metrics
metrics.Counter("cache_hits_total")
metrics.Counter("cache_misses_total")

// Message broker metrics
metrics.Counter("messages_published_total", "topic")
metrics.Counter("messages_consumed_total", "topic", "status")
```

## Tracing

### Tracer Contract

```go
type Tracer interface {
    // Start a new span
    StartSpan(name string, opts ...SpanOption) Span
    
    // Extract span context from carrier
    Extract(carrier any) SpanContext
    
    // Inject span context into carrier
    Inject(ctx SpanContext, carrier any)
}

type Span interface {
    // End the span
    End()
    
    // Add attributes
    SetAttribute(key string, value any)
    
    // Record error
    RecordError(err error)
    
    // Add event
    AddEvent(name string, attrs ...any)
    
    // Get span context
    Context() SpanContext
}
```

### Using Tracing

```go
func CreateOrder(ctx *unicorn.Context, req CreateOrderRequest) (*Order, error) {
    tracer := ctx.Tracer()
    
    // Start parent span
    span := tracer.StartSpan("CreateOrder")
    defer span.End()
    
    span.SetAttribute("user_id", req.UserID)
    span.SetAttribute("product_id", req.ProductID)
    
    // Child span for validation
    validateSpan := tracer.StartSpan("ValidateOrder", 
        tracer.ChildOf(span.Context()))
    err := validateOrder(req)
    if err != nil {
        validateSpan.RecordError(err)
        validateSpan.End()
        return nil, err
    }
    validateSpan.End()
    
    // Child span for database
    dbSpan := tracer.StartSpan("SaveOrder",
        tracer.ChildOf(span.Context()))
    order, err := db.SaveOrder(req)
    if err != nil {
        dbSpan.RecordError(err)
        dbSpan.End()
        return nil, err
    }
    dbSpan.SetAttribute("order_id", order.ID)
    dbSpan.End()
    
    // Add event
    span.AddEvent("order_created", "order_id", order.ID)
    
    return order, nil
}
```

### Distributed Tracing

```go
// Extract trace context from incoming request
func HandleMessage(ctx *unicorn.Context, msg Message) error {
    tracer := ctx.Tracer()
    
    // Extract parent context from message headers
    parentCtx := tracer.Extract(msg.Headers)
    
    // Start span as child of parent
    span := tracer.StartSpan("ProcessMessage",
        tracer.ChildOf(parentCtx))
    defer span.End()
    
    // Process message...
    return nil
}

// Inject trace context into outgoing request
func PublishEvent(ctx *unicorn.Context, event Event) error {
    tracer := ctx.Tracer()
    span := tracer.StartSpan("PublishEvent")
    defer span.End()
    
    // Inject trace context
    headers := make(map[string]string)
    tracer.Inject(span.Context(), headers)
    
    // Publish with trace headers
    return broker.Publish(ctx.Context(), "events", &BrokerMessage{
        Headers: headers,
        Body:    event.Marshal(),
    })
}
```

## Logging

### Logger Contract

```go
type Logger interface {
    // Log levels
    Debug(msg string, keysAndValues ...any)
    Info(msg string, keysAndValues ...any)
    Warn(msg string, keysAndValues ...any)
    Error(msg string, keysAndValues ...any)
    
    // Create child logger with fields
    With(keysAndValues ...any) Logger
    
    // Sync flushes buffered logs
    Sync() error
}
```

### Using Logging

```go
func CreateUser(ctx *unicorn.Context, req CreateUserRequest) (*User, error) {
    log := ctx.Logger()
    
    // Structured logging
    log.Info("creating user",
        "name", req.Name,
        "email", req.Email,
    )
    
    user, err := userService.Create(req)
    if err != nil {
        log.Error("failed to create user",
            "error", err,
            "email", req.Email,
        )
        return nil, err
    }
    
    log.Info("user created",
        "user_id", user.ID,
        "email", user.Email,
    )
    
    return user, nil
}
```

### Child Loggers

```go
func ProcessOrder(ctx *unicorn.Context, req OrderRequest) error {
    // Create child logger with order context
    log := ctx.Logger().With(
        "order_id", req.OrderID,
        "user_id", req.UserID,
    )
    
    log.Info("processing order")  // Includes order_id, user_id
    
    if err := validateOrder(req); err != nil {
        log.Warn("validation failed", "error", err)
        return err
    }
    
    log.Info("order processed")
    return nil
}
```

### Log Levels

| Level | Use Case |
|-------|----------|
| `Debug` | Detailed debugging info, disabled in production |
| `Info` | Normal operational messages |
| `Warn` | Warning conditions that should be addressed |
| `Error` | Error conditions that require attention |

## Middleware Integration

### Observability Middleware

```go
import "github.com/madcok-co/unicorn/pkg/middleware"

// Create observability middleware
observabilityMiddleware := middleware.NewObservabilityMiddleware(
    middleware.ObservabilityConfig{
        Metrics: metricsCollector,
        Tracer:  tracer,
        Logger:  logger,
        
        // Options
        LogRequests:     true,
        LogResponses:    false,  // Don't log response bodies
        TraceAll:        true,
        MetricsPrefix:   "myapp_",
    },
)

// Apply globally
app.Use(observabilityMiddleware)

// Or per-handler
app.RegisterHandler(CreateUser).
    Use(observabilityMiddleware).
    HTTP("POST", "/users").
    Done()
```

### Request Logging

Automatic request logging includes:

```json
{
  "level": "info",
  "msg": "http request",
  "method": "POST",
  "path": "/users",
  "status": 201,
  "duration_ms": 45.2,
  "request_id": "req-abc123",
  "user_id": "user-123",
  "ip": "192.168.1.1"
}
```

## Correlation IDs

### Request ID Propagation

```go
func CreateOrder(ctx *unicorn.Context, req OrderRequest) (*Order, error) {
    // Get request ID (auto-generated or from header)
    requestID := ctx.Request().ID
    
    // Include in all logs
    log := ctx.Logger().With("request_id", requestID)
    
    // Propagate to downstream services
    broker.Publish(ctx.Context(), "orders", &BrokerMessage{
        Headers: map[string]string{
            "X-Request-ID": requestID,
        },
        Body: orderData,
    })
    
    return order, nil
}
```

### Correlation ID Pattern

```go
func MyHandler(ctx *unicorn.Context, req Request) (*Response, error) {
    // Set correlation ID
    correlationID := ctx.Request().Headers["X-Correlation-ID"]
    if correlationID == "" {
        correlationID = uuid.New().String()
    }
    ctx.Set("correlation_id", correlationID)
    
    // Use throughout request lifecycle
    log := ctx.Logger().With("correlation_id", correlationID)
    span := ctx.Tracer().StartSpan("MyHandler")
    span.SetAttribute("correlation_id", correlationID)
    
    return &Response{}, nil
}
```

## Health Checks

### Health Check Handler

```go
type HealthResponse struct {
    Status    string            `json:"status"`
    Version   string            `json:"version"`
    Timestamp time.Time         `json:"timestamp"`
    Checks    map[string]string `json:"checks"`
}

func HealthCheck(ctx *unicorn.Context) (*HealthResponse, error) {
    checks := make(map[string]string)
    
    // Check database
    if db := ctx.DB(); db != nil {
        if err := db.Health(); err != nil {
            checks["database"] = "unhealthy: " + err.Error()
        } else {
            checks["database"] = "healthy"
        }
    }
    
    // Check cache
    if cache := ctx.Cache(); cache != nil {
        if err := cache.Health(); err != nil {
            checks["cache"] = "unhealthy: " + err.Error()
        } else {
            checks["cache"] = "healthy"
        }
    }
    
    // Check broker
    if broker := ctx.Broker(); broker != nil {
        if err := broker.Health(); err != nil {
            checks["broker"] = "unhealthy: " + err.Error()
        } else {
            checks["broker"] = "healthy"
        }
    }
    
    // Determine overall status
    status := "healthy"
    for _, v := range checks {
        if strings.HasPrefix(v, "unhealthy") {
            status = "unhealthy"
            break
        }
    }
    
    return &HealthResponse{
        Status:    status,
        Version:   unicorn.Version(),
        Timestamp: time.Now(),
        Checks:    checks,
    }, nil
}
```

### Readiness vs Liveness

```go
// Liveness: Is the application running?
func LivenessCheck(ctx *unicorn.Context) (map[string]string, error) {
    return map[string]string{"status": "alive"}, nil
}

// Readiness: Is the application ready to serve traffic?
func ReadinessCheck(ctx *unicorn.Context) (*HealthResponse, error) {
    // Check all dependencies
    return HealthCheck(ctx)
}

// Register both
app.RegisterHandler(LivenessCheck).HTTP("GET", "/health/live").Done()
app.RegisterHandler(ReadinessCheck).HTTP("GET", "/health/ready").Done()
```

## Best Practices

### 1. Use Structured Logging

```go
// Good: Structured
log.Info("user created", "user_id", user.ID, "email", user.Email)

// Bad: String formatting
log.Info(fmt.Sprintf("user created: %s (%s)", user.ID, user.Email))
```

### 2. Include Context in Logs

```go
// Always include request context
log.Info("processing request",
    "request_id", ctx.Request().ID,
    "user_id", ctx.Identity().ID,
    "method", ctx.Request().Method,
    "path", ctx.Request().Path,
)
```

### 3. Use Appropriate Log Levels

```go
log.Debug("detailed debug info")     // Development only
log.Info("user logged in")           // Normal operations
log.Warn("rate limit approaching")   // Potential issues
log.Error("database connection lost") // Errors
```

### 4. Track Business Metrics

```go
// Business metrics, not just technical
metrics.Counter("orders_placed_total", "product_type")
metrics.Gauge("cart_value_dollars")
metrics.Histogram("checkout_duration_seconds", buckets)
```

### 5. Trace Critical Paths

```go
// Trace important operations
span := tracer.StartSpan("PaymentProcessing")
span.SetAttribute("amount", payment.Amount)
span.SetAttribute("currency", payment.Currency)
```

## Next Steps

- [API Reference](./api-reference.md) - Complete API documentation
- [Best Practices](./best-practices.md) - Production recommendations
- [Examples](./examples.md) - Complete example applications
