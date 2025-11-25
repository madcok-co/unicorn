# Unicorn Framework - Complete Examples

This directory contains comprehensive examples demonstrating all features of the Unicorn Framework with full infrastructure setup using Docker Compose.

## 📚 Examples Overview

| Example | Description | Port | Features Demonstrated |
|---------|-------------|------|----------------------|
| **main.go** | Basic CRUD API | 8080 | HTTP handlers, caching, query params, path params, message broker |
| **main_enhanced.go** | Enhanced with Security | 8080 | JWT auth, password hashing, protected endpoints, env config |

## 🚀 Quick Start

### Prerequisites

- Go 1.21+
- Docker & Docker Compose
- Make (optional, for convenience)

### Setup with Docker Compose

The easiest way to get started is using Docker Compose which sets up all required infrastructure:

```bash
# 1. Setup environment
make setup
# or manually: cp .env.example .env

# 2. Start all services (PostgreSQL, Redis, Kafka, etc.)
make up

# 3. Run the application
make run
# or: go run main_enhanced.go
```

That's it! The application will start with all infrastructure ready.

### Available Services

After running `make up`, you'll have access to:

| Service | URL | Credentials |
|---------|-----|-------------|
| **PostgreSQL** | localhost:5432 | unicorn / unicorn_pass |
| **Redis** | localhost:6379 | (no password) |
| **Kafka** | localhost:9092 | - |
| **Kafka UI** | http://localhost:8090 | - |
| **Prometheus** | http://localhost:9090 | - |
| **Grafana** | http://localhost:3000 | admin / admin |
| **Jaeger UI** | http://localhost:16686 | - |
| **Adminer (DB UI)** | http://localhost:8081 | - |

### Run the Main Example

```bash
cd core/examples/complete-features
go run main.go
```

### Test the Enhanced Example

```bash
# 1. Register a new user
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john_doe",
    "email": "john@example.com",
    "password": "SecurePass123!"
  }'

# 2. Login to get JWT token
TOKEN=$(curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john@example.com",
    "password": "SecurePass123!"
  }' | jq -r '.token')

echo "Token: $TOKEN"

# 3. Verify token
curl -X POST http://localhost:8080/auth/verify \
  -H "Authorization: Bearer $TOKEN"

# 4. Get user profile
curl http://localhost:8080/auth/profile \
  -H "Authorization: Bearer $TOKEN"

# 5. Create product (protected endpoint)
curl -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "Laptop",
    "description": "High-performance laptop",
    "price": 999.99,
    "stock": 50
  }'

# 6. List products
curl http://localhost:8080/products

# 7. Get product by ID
curl http://localhost:8080/products/prod_123

# 8. Health check
curl http://localhost:8080/health
```

### Using Make Commands

```bash
# View all available commands
make help

# Start infrastructure
make up

# View logs
make logs

# Stop all services
make down

# Test endpoints
make curl-health
make curl-products

# Clean up everything (removes volumes)
make clean
```

## 📖 Examples Detailed

### 1. Basic Example (main.go)

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

### 2. Enhanced Example (main_enhanced.go)

**Features:**
- ✅ **JWT Authentication** - Secure token-based auth
- ✅ **Password Hashing** - bcrypt password hashing
- ✅ **User Registration & Login** - Complete auth flow
- ✅ **Token Verification** - JWT token validation
- ✅ **Protected Endpoints** - Authentication required routes
- ✅ **Environment Configuration** - .env file support
- ✅ **Service Injection** - Custom services (passwordHasher, jwtAuth)

**Endpoints:**

```bash
# Authentication
POST   /auth/register    # Register new user
POST   /auth/login       # Login and get JWT token
POST   /auth/verify      # Verify JWT token
GET    /auth/profile     # Get user profile (requires auth)

# Products (with auth)
POST   /products         # Create product (requires auth)
GET    /products         # List products
GET    /products/:id     # Get product by ID

# Health
GET    /health           # Health check
```

**Example Usage:**

```go
// Password Hashing
passwordHasher := hasher.NewPasswordHasher()
hashedPassword, _ := passwordHasher.Hash("MyPassword123")
err := passwordHasher.Verify("MyPassword123", hashedPassword)

// JWT Authentication
jwtAuth := auth.NewJWTAuth(auth.JWTConfig{
    Secret:     []byte("your-secret-key"),
    Expiration: 24 * time.Hour,
})

token, _ := jwtAuth.GenerateToken(map[string]interface{}{
    "user_id": "123",
    "email":   "user@example.com",
})

claims, _ := jwtAuth.VerifyToken(token)
```

## 💡 Additional Features Available

While these examples focus on core features, Unicorn Framework also supports:

### Middleware (via HTTP adapter)
```go
// Available middleware in core/pkg/middleware:
- CORS - Cross-origin resource sharing
- Recovery - Panic recovery with stack traces  
- Timeout - Request timeout handling
- RateLimit - Rate limiting per IP/user
- Health - Health check endpoints
```

### Custom Service Injection
```go
// Register your own services
type EmailService interface {
    SendEmail(to, subject, body string) error
}

app.RegisterService("email", myEmailService)

// Use in handlers
emailSvc := ctx.GetService("email").(EmailService)
```

### Resilience Patterns
```go
// Circuit Breaker (in core/pkg/resilience)
cb := resilience.NewCircuitBreaker(config)
result, err := cb.Execute(func() (interface{}, error) {
    return externalService.Call()
})

// Retry with exponential backoff
result, err := resilience.Retry(retryConfig, func() (interface{}, error) {
    return unstableOperation()
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
