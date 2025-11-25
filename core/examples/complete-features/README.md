# Unicorn Framework - Complete Examples

This directory contains comprehensive examples demonstrating all features of the Unicorn Framework.

## 📚 Examples Overview

| Example | Description | Port | Features Demonstrated |
|---------|-------------|------|----------------------|
| **main.go** | Complete CRUD API | 8080 | HTTP handlers, validation, caching, query params, path params, error handling |
| **middleware-example.go** | Middleware usage | 8081 | CORS, Recovery, Timeout, Rate Limiting, Authentication |
| **custom-services-example.go** | Custom service injection | 8082 | DI, singleton services, factory services, business logic separation |
| **resilience-example.go** | Resilience patterns | 8083 | Circuit Breaker, Retry with backoff, failure handling |

## 🚀 Quick Start

### Run the Main Example

```bash
cd core/examples/complete-features
go run main.go
```

Then test the endpoints:

```bash
# Health check
curl http://localhost:8080/health

# List products
curl http://localhost:8080/products

# Create product
curl -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Laptop",
    "description": "High-performance laptop for developers",
    "price": 999.99,
    "stock": 50
  }'

# Get product by ID
curl http://localhost:8080/products/prod-123

# Search products
curl "http://localhost:8080/products/search?q=laptop&category=electronics"

# Update product
curl -X PUT http://localhost:8080/products/prod-123 \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Gaming Laptop",
    "description": "High-end gaming laptop",
    "price": 1499.99,
    "stock": 30
  }'

# Delete product
curl -X DELETE http://localhost:8080/products/prod-123
```

## 📖 Detailed Examples

### 1. Complete CRUD API (main.go)

**Features:**
- ✅ CRUD operations (Create, Read, Update, Delete)
- ✅ Request validation
- ✅ Path parameters (`/products/:id`)
- ✅ Query parameters (`?page=1&limit=10`)
- ✅ Cache integration
- ✅ Logging
- ✅ Message broker integration
- ✅ Error handling
- ✅ Health checks
- ✅ Metrics endpoint

**Endpoints:**

```bash
# CRUD Operations
POST   /products           # Create product with validation
GET    /products           # List products with pagination
GET    /products/:id       # Get product by ID
PUT    /products/:id       # Update product
DELETE /products/:id       # Delete product

# Search & Filter
GET    /products/search    # Search with query params

# Events
POST   /events/product-created  # Trigger product created event

# Monitoring
GET    /health             # Health check
GET    /metrics            # Application metrics

# Testing
GET    /error?type=X       # Simulate errors (validation|not_found|server)
```

**Example Handlers:**

```go
// Handler with request validation
func CreateProduct(ctx *context.Context, req CreateProductRequest) (*Product, error) {
    // Automatic validation happens before handler is called
    // Access infrastructure
    logger := ctx.Logger()
    cache := ctx.Cache()
    
    // Pure business logic
    product := &Product{...}
    
    // Cache the result
    cache.Set(ctx.Context(), "product:"+product.ID, product, 1*time.Hour)
    
    logger.Info("product created", "id", product.ID)
    return product, nil
}

// Handler with path parameters
func GetProduct(ctx *context.Context) (*Product, error) {
    productID := ctx.Request().Params["id"]
    // ... business logic
}

// Handler with query parameters
func ListProducts(ctx *context.Context) (map[string]interface{}, error) {
    page := ctx.Request().Query["page"]
    limit := ctx.Request().Query["limit"]
    // ... business logic
}
```

### 2. Middleware Examples (middleware-example.go)

**Features:**
- ✅ CORS (Cross-Origin Resource Sharing)
- ✅ Panic Recovery with stack traces
- ✅ Request Timeout
- ✅ Rate Limiting (per-IP)
- ✅ Authentication (JWT, API Key)
- ✅ Request/Response logging

**Run:**

```bash
go run middleware-example.go
```

**Test:**

```bash
# Public endpoint (no auth)
curl http://localhost:8081/public

# Protected endpoint (requires auth)
curl http://localhost:8081/protected \
  -H "Authorization: Bearer your-jwt-token"

# Slow endpoint (timeout demo)
curl "http://localhost:8081/slow?duration=3s"

# Panic recovery demo
curl "http://localhost:8081/panic?panic=true"

# Rate limited endpoint
for i in {1..15}; do 
  curl http://localhost:8081/ratelimited
done
```

**Middleware Configuration:**

```go
// CORS Middleware
corsMiddleware := middleware.CORS(&middleware.CORSConfig{
    AllowedOrigins:   []string{"*"},
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
    AllowedHeaders:   []string{"*"},
    AllowCredentials: true,
    MaxAge:           3600,
})

// Recovery Middleware
recoveryMiddleware := middleware.Recovery(middleware.RecoveryConfig{
    Logger:           logger,
    StackTrace:       true,
})

// Timeout Middleware
timeoutMiddleware := middleware.Timeout(5 * time.Second)

// Rate Limiting
rateLimiter := ratelimit.NewMemoryRateLimiter(10, time.Minute)
rateLimitMiddleware := middleware.RateLimit(middleware.RateLimitConfig{
    RateLimiter: rateLimiter,
    KeyFunc: func(ctx *context.Context) string {
        return ctx.Request().Headers["X-Real-IP"]
    },
})
```

### 3. Custom Service Injection (custom-services-example.go)

**Features:**
- ✅ Dependency Injection
- ✅ Singleton services (shared across requests)
- ✅ Factory services (new instance per request)
- ✅ Custom business logic interfaces
- ✅ Service composition

**Run:**

```bash
go run custom-services-example.go
```

**Test:**

```bash
# Create order (uses email, payment, notification services)
curl -X POST http://localhost:8082/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "product_id": "prod456",
    "quantity": 2,
    "total_amount": 99.99
  }'

# Cancel order (refund via payment service)
curl -X DELETE http://localhost:8082/orders/order-123

# Send notification
curl -X POST "http://localhost:8082/orders/order-123/notify?type=push"

# Factory service example
curl http://localhost:8082/factory
```

**Custom Services:**

```go
// Define your service interface
type EmailService interface {
    SendEmail(to, subject, body string) error
    SendTemplateEmail(to, template string, data map[string]interface{}) error
}

// Implement the service
type emailService struct {
    from   string
    apiKey string
}

func (s *emailService) SendEmail(to, subject, body string) error {
    // Your implementation
}

// Register as singleton (same instance for all requests)
emailSvc := NewEmailService("noreply@example.com", "api-key")
app.RegisterService("email", emailSvc)

// Register as factory (new instance per request)
app.RegisterServiceFactory("requestLogger", func(ctx *context.Context) (any, error) {
    return NewRequestLogger(ctx)
})

// Use in handlers
func CreateOrder(ctx *context.Context, req CreateOrderRequest) (*Order, error) {
    // Get injected services
    emailSvc := ctx.GetService("email").(EmailService)
    paymentSvc := ctx.GetService("payment").(PaymentService)
    notificationSvc := ctx.GetService("notification").(NotificationService)
    
    // Use them
    paymentResult, _ := paymentSvc.ProcessPayment(req.TotalAmount, "USD", "card")
    emailSvc.SendEmail("user@example.com", "Order Confirmed", "...")
    notificationSvc.SendPushNotification(req.UserID, "Order Placed", "...")
    
    return order, nil
}
```

### 4. Resilience Patterns (resilience-example.go)

**Features:**
- ✅ Circuit Breaker pattern
- ✅ Retry with exponential backoff
- ✅ Timeout handling
- ✅ Failure rate simulation
- ✅ Circuit breaker monitoring

**Run:**

```bash
go run resilience-example.go
```

**Test:**

```bash
# Call external service (circuit breaker protection)
for i in {1..10}; do 
  curl http://localhost:8083/external
  sleep 0.5
done

# Check circuit breaker status
curl http://localhost:8083/status/circuit-breakers

# Retry example
curl http://localhost:8083/retry

# Database query with CB + Retry
curl http://localhost:8083/database

# Set failure rate to 80% (for testing)
curl -X POST "http://localhost:8083/testing/failure-rate?service=external&rate=0.8"

# Reset circuit breaker
curl -X POST "http://localhost:8083/circuit-breaker/reset?name=external"
```

**Circuit Breaker Usage:**

```go
// Create circuit breaker
cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
    Name:              "external-service",
    MaxFailures:       3,              // Open after 3 failures
    Timeout:           10 * time.Second, // Try again after 10s
    HalfOpenRequests:  2,              // Allow 2 requests in half-open
})

// Use it
result, err := cb.Execute(func() (interface{}, error) {
    return externalService.Call()
})
```

**Retry with Backoff:**

```go
retryConfig := resilience.RetryConfig{
    MaxAttempts:     5,
    InitialInterval: 100 * time.Millisecond,
    MaxInterval:     2 * time.Second,
    Multiplier:      2.0,
    OnRetry: func(attempt int, err error) {
        logger.Warn("retrying", "attempt", attempt, "error", err)
    },
}

result, err := resilience.Retry(retryConfig, func() (interface{}, error) {
    return unstableOperation()
})
```

**Combined Pattern (CB + Retry):**

```go
// First apply circuit breaker, then retry
result, err := circuitBreaker.Execute(func() (interface{}, error) {
    return resilience.Retry(retryConfig, func() (interface{}, error) {
        return database.Query(sql)
    })
})
```

## 🎯 Key Concepts Demonstrated

### 1. Handler Patterns

```go
// Simple handler (no request body)
func HealthCheck(ctx *context.Context) (map[string]interface{}, error) {
    return map[string]interface{}{"status": "ok"}, nil
}

// Handler with request body (automatic validation)
func CreateProduct(ctx *context.Context, req CreateProductRequest) (*Product, error) {
    // req is already validated
    return &Product{...}, nil
}

// Handler with path parameters
func GetProduct(ctx *context.Context) (*Product, error) {
    id := ctx.Request().Params["id"]
    return fetchProduct(id)
}

// Handler returning custom type
func ListProducts(ctx *context.Context) ([]*Product, error) {
    return []*Product{...}, nil
}
```

### 2. Multi-Trigger Handlers

```go
// Same handler works for HTTP and Message Queue!
app.RegisterHandler(ProcessEvent).
    Named("process-event").
    HTTP("POST", "/events").      // Trigger via HTTP
    Message("events.topic").       // Trigger via message broker
    Cron("0 * * * *").            // Trigger via cron
    Done()
```

### 3. Infrastructure Access

```go
func MyHandler(ctx *context.Context) (*Result, error) {
    // All infrastructure available via context
    logger := ctx.Logger()
    cache := ctx.Cache()
    db := ctx.DB()
    broker := ctx.Broker()
    validator := ctx.Validator()
    
    // Custom services
    emailSvc := ctx.GetService("email").(EmailService)
    
    // Request data
    params := ctx.Request().Params
    query := ctx.Request().Query
    headers := ctx.Request().Headers
    body := ctx.Request().Body
    
    // Business logic here
}
```

### 4. Error Handling

```go
func MyHandler(ctx *context.Context) (*Result, error) {
    // Return errors directly
    if err := validate(data); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }
    
    // Framework handles error response
    return result, nil
}
```

## 🔧 Configuration

### Application Config

```go
app := app.New(&app.Config{
    Name:       "my-app",
    Version:    "1.0.0",
    EnableHTTP: true,
    HTTP: &httpAdapter.Config{
        Host:         "0.0.0.0",
        Port:         8080,
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 30 * time.Second,
        MaxBodySize:  10 << 20, // 10MB
    },
    EnableBroker: true,
    Broker: &brokerAdapter.Config{
        GroupID: "my-consumer-group",
    },
    EnableCron: true,
})
```

### Infrastructure Setup

```go
// Logger
logger := loggerAdapter.NewConsoleLogger()
app.SetLogger(logger)

// Cache
cache := cacheAdapter.NewMemoryCache()
app.SetCache(cache)

// Validator
validator := validatorAdapter.NewNoOpValidator()
app.SetValidator(validator)

// Message Broker
broker := memoryBroker.NewMemoryBroker()
app.SetBroker(broker)

// Multiple named adapters
app.SetDB(primaryDB, "primary")
app.SetDB(replicaDB, "replica")
app.SetCache(redisCache, "redis")
app.SetCache(memcache, "memcached")
```

## 📊 Best Practices

### 1. Separation of Concerns

```go
// ✅ GOOD: Handler only contains business logic
func CreateProduct(ctx *context.Context, req CreateProductRequest) (*Product, error) {
    // Infrastructure via context
    db := ctx.DB()
    cache := ctx.Cache()
    
    // Pure business logic
    product := &Product{...}
    db.Create(product)
    cache.Set(ctx.Context(), "product:"+product.ID, product, time.Hour)
    
    return product, nil
}

// ❌ BAD: Handler creates dependencies
func CreateProduct(req CreateProductRequest) (*Product, error) {
    db := gorm.Open(...) // Don't do this!
    redis := redis.NewClient(...) // Don't do this!
}
```

### 2. Use Custom Services for Business Logic

```go
// ✅ GOOD: Inject business services
type OrderService interface {
    CreateOrder(req CreateOrderRequest) (*Order, error)
    CancelOrder(orderID string) error
}

app.RegisterService("orderService", NewOrderService())

func CreateOrder(ctx *context.Context, req CreateOrderRequest) (*Order, error) {
    orderSvc := ctx.GetService("orderService").(OrderService)
    return orderSvc.CreateOrder(req)
}
```

### 3. Error Handling

```go
// ✅ GOOD: Return descriptive errors
func CreateProduct(ctx *context.Context, req CreateProductRequest) (*Product, error) {
    if req.Price <= 0 {
        return nil, fmt.Errorf("price must be greater than 0")
    }
    
    if err := db.Create(product); err != nil {
        return nil, fmt.Errorf("failed to create product: %w", err)
    }
    
    return product, nil
}
```

### 4. Use Resilience Patterns

```go
// ✅ GOOD: Protect external calls with circuit breaker
cb := resilience.NewCircuitBreaker(config)
result, err := cb.Execute(func() (interface{}, error) {
    return externalService.Call()
})

// ✅ GOOD: Retry transient failures
result, err := resilience.Retry(retryConfig, func() (interface{}, error) {
    return database.Query(sql)
})
```

## 🧪 Testing Examples

Each example can be tested independently:

```bash
# Run specific example
go run main.go
go run middleware-example.go
go run custom-services-example.go
go run resilience-example.go

# Or build and run
go build -o complete-example main.go
./complete-example
```

## 📝 Notes

- All examples use **memory adapters** for demo purposes
- In production, replace with real adapters (Redis, PostgreSQL, Kafka, etc.)
- Check `contrib/` directory for production-ready driver implementations
- Examples demonstrate **framework capabilities**, not production patterns

## 🔗 Related Documentation

- [Framework Architecture](../../../docs/architecture.md)
- [Handler Documentation](../../../docs/handlers.md)
- [Middleware Guide](../../../docs/middleware.md)
- [Best Practices](../../../docs/best-practices.md)
- [Contrib Drivers](../../../contrib/README.md)

## 💡 Tips

1. **Start with main.go** to understand basic CRUD operations
2. **Then explore custom-services-example.go** for dependency injection
3. **Study resilience-example.go** for production-ready patterns
4. **Finally check middleware-example.go** for cross-cutting concerns

Happy coding with Unicorn! 🦄
