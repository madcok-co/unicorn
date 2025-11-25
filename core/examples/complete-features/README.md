# Unicorn Framework - Complete Feature Examples

This directory contains comprehensive examples demonstrating all features of the Unicorn framework.

## 📁 Example Files

### Basic Examples
- **`main.go`** - Basic HTTP server with routing and products API
- **`main_enhanced.go`** - Enhanced with JWT authentication, validation, and security features
- **`main_complete.go`** - Complete example with all major features including circuit breaker, retry, metrics, and message broker

### Feature-Specific Examples (Coming)
- **`example_database.go`** - Database operations with GORM (CRUD, transactions, query builder)
- **`example_validation.go`** - Request validation with struct tags
- **`example_middleware.go`** - Production middleware (CORS, Recovery, Timeout)
- **`example_apikey.go`** - API Key authentication
- **`example_encryption.go`** - Data encryption with AES-GCM
- **`example_audit.go`** - Audit logging for security events
- **`example_cron.go`** - Scheduled jobs with cron
- **`example_tracing.go`** - Distributed tracing

## 🚀 Quick Start

### 1. Basic Example
```bash
go run main.go
```

**Features:**
- HTTP REST API
- Product CRUD operations
- Health checks
- Metrics endpoint

### 2. Enhanced Example
```bash
go run main_enhanced.go
```

**Features:**
- JWT authentication
- Password hashing (bcrypt)
- User registration/login
- Token verification
- Cache integration

### 3. Complete Example
```bash
export JWT_SECRET="your-secret-key-min-32-chars-long"
go run main_complete.go
```

**Features:**
- ✅ All features from Basic + Enhanced
- ✅ Circuit Breaker (payment service protection)
- ✅ Retry with exponential backoff
- ✅ Message Broker (pub/sub for events)
- ✅ Metrics (counters, histograms)
- ✅ Rate Limiting
- ✅ Custom service injection
- ✅ Email service integration
- ✅ Order processing workflow

## 🎯 Feature Coverage

| Feature Category | Basic | Enhanced | Complete |
|-----------------|-------|----------|----------|
| **HTTP REST API** | ✅ | ✅ | ✅ |
| **Routing** | ✅ | ✅ | ✅ |
| **JWT Auth** | ❌ | ✅ | ✅ |
| **Password Hashing** | ❌ | ✅ | ✅ |
| **Cache** | ❌ | ✅ | ✅ |
| **Validation** | ❌ | ❌ | ✅ (tags only) |
| **Circuit Breaker** | ❌ | ❌ | ✅ |
| **Retry Pattern** | ❌ | ❌ | ✅ |
| **Message Broker** | ❌ | ❌ | ✅ |
| **Metrics** | Basic | Basic | ✅ Advanced |
| **Rate Limiting** | ❌ | ❌ | ✅ |
| **Custom Services** | ❌ | ❌ | ✅ |
| **Database** | ❌ | ❌ | ❌ |
| **Middleware Stack** | ❌ | ❌ | ❌ |
| **API Keys** | ❌ | ❌ | ❌ |
| **Encryption** | ❌ | ❌ | ❌ |
| **Audit Logging** | ❌ | ❌ | ❌ |
| **Cron Jobs** | ❌ | ❌ | ❌ |
| **Tracing** | ❌ | ❌ | ❌ |

## 📖 API Endpoints

### Basic & Enhanced

#### Health & Metrics
- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics

#### Products
- `POST /products` - Create product
- `GET /products` - List products (with pagination)
- `GET /products/:id` - Get product by ID

#### Authentication (Enhanced & Complete)
- `POST /auth/register` - Register new user
- `POST /auth/login` - Login and get JWT token
- `POST /auth/verify` - Verify JWT token

#### Orders (Complete only)
- `POST /orders` - Create new order

## 🔧 Configuration

### Environment Variables

```bash
# JWT Secret (required for enhanced & complete)
export JWT_SECRET="your-secret-key-min-32-chars-long-for-production"

# Server Port (optional, default: 8080)
export PORT="8080"

# Log Level (optional, default: info)
export LOG_LEVEL="debug"
```

## 🧪 Testing Examples

### Test with curl

```bash
# Health check
curl http://localhost:8080/health

# Register user
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"john","email":"john@example.com","password":"password123"}'

# Login
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"john@example.com","password":"password123"}'

# Create product (with JWT token)
curl -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{"name":"Laptop","description":"Gaming Laptop","price":1299.99,"stock":10}'

# Get products
curl http://localhost:8080/products

# Get product by ID
curl http://localhost:8080/products/prod_1
```

## 🏗️ Architecture Patterns

### Circuit Breaker Pattern (Complete)
```go
// Protects external service calls
paymentService := NewPaymentService() // Has built-in circuit breaker
result, err := paymentService.ProcessPayment(100.00, "USD")
```

### Retry Pattern (Complete)
```go
// Retries with exponential backoff
retryer := resilience.NewRetryer(&resilience.RetryConfig{
    MaxAttempts:     3,
    InitialInterval: 100 * time.Millisecond,
    MaxInterval:     1 * time.Second,
    Multiplier:      2.0,
})
err := retryer.Do(func() error {
    return broker.Publish(ctx, "topic", message)
})
```

### Message Broker Pattern (Complete)
```go
// Publish event
broker.Publish(ctx, "product.created", message)

// Subscribe to events
application.RegisterHandler(HandleProductCreated).
    Named("product-created-handler").
    Message("product.created").
    Done()
```

### Metrics Pattern (Complete)
```go
// Counter
metrics.Counter("orders_created_total", T("status", "success")).Inc()

// Histogram
metrics.Histogram("order_amount", T("currency", "USD")).Observe(totalPrice)
```

## 🐛 Troubleshooting

### Error: "pattern redeclared"
**Cause:** Running `go build` without specifying a file

**Solution:** Always specify which example to build:
```bash
go build main.go              # ✅ Correct
go build main_enhanced.go     # ✅ Correct
go build main_complete.go     # ✅ Correct
go build                      # ❌ Wrong - causes conflicts
```

### Error: "JWT secret required"
**Cause:** Missing JWT_SECRET environment variable

**Solution:**
```bash
export JWT_SECRET="your-secret-key-min-32-chars-long"
go run main_enhanced.go
```

### Error: "port already in use"
**Cause:** Another instance is running

**Solution:**
```bash
# Kill process on port 8080
lsof -ti:8080 | xargs kill -9

# Or use different port
PORT=8081 go run main.go
```

## 📚 Learn More

- [Unicorn Documentation](../../README.md)
- [API Reference](../../docs/API.md)
- [Architecture Guide](../../docs/ARCHITECTURE.md)

## 🤝 Contributing

Found a bug or want to add more examples? Please open an issue or submit a pull request!

## 📝 License

MIT License - see [LICENSE](../../../LICENSE) for details
