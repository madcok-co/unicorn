# Best Practices

Production recommendations for the Unicorn framework.

## Code Organization

### Handler Organization

```
handlers/
├── user/
│   ├── create.go
│   ├── get.go
│   ├── list.go
│   └── delete.go
├── order/
│   ├── create.go
│   └── process.go
└── health.go
```

### Service Layer Pattern

```go
// handlers/user/create.go
func CreateUser(ctx *unicorn.Context, req CreateUserRequest) (*User, error) {
    // Delegate to service layer
    return userService.Create(ctx, req)
}

// services/user/service.go
type UserService struct {
    db    unicorn.Database
    cache unicorn.Cache
    log   unicorn.Logger
}

func (s *UserService) Create(ctx *unicorn.Context, req CreateUserRequest) (*User, error) {
    // Business logic here
    user := &User{
        ID:    uuid.New().String(),
        Name:  req.Name,
        Email: req.Email,
    }
    
    if err := s.db.Create(user); err != nil {
        return nil, err
    }
    
    s.cache.Set(ctx.Context(), "user:"+user.ID, user, time.Hour)
    s.log.Info("user created", "id", user.ID)
    
    return user, nil
}
```

### Repository Pattern

```go
// repository/user.go
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    FindByID(ctx context.Context, id string) (*User, error)
    FindByEmail(ctx context.Context, email string) (*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
}

// repository/user_postgres.go
type PostgresUserRepository struct {
    db unicorn.Database
}

func (r *PostgresUserRepository) Create(ctx context.Context, user *User) error {
    return r.db.Create(user)
}
```

## Error Handling

### Define Custom Errors

```go
// errors/errors.go
var (
    ErrNotFound       = errors.New("resource not found")
    ErrUnauthorized   = errors.New("unauthorized")
    ErrForbidden      = errors.New("forbidden")
    ErrValidation     = errors.New("validation failed")
    ErrConflict       = errors.New("resource conflict")
    ErrInternal       = errors.New("internal error")
)

// Wrap with context
func ErrUserNotFound(id string) error {
    return fmt.Errorf("user %s: %w", id, ErrNotFound)
}
```

### Map Errors to HTTP Status

```go
func mapErrorToHTTP(err error) *http.HTTPError {
    switch {
    case errors.Is(err, ErrNotFound):
        return &http.HTTPError{StatusCode: 404, Message: "Not found", Internal: err}
    case errors.Is(err, ErrUnauthorized):
        return &http.HTTPError{StatusCode: 401, Message: "Unauthorized", Internal: err}
    case errors.Is(err, ErrForbidden):
        return &http.HTTPError{StatusCode: 403, Message: "Forbidden", Internal: err}
    case errors.Is(err, ErrValidation):
        return &http.HTTPError{StatusCode: 400, Message: err.Error(), Internal: err}
    case errors.Is(err, ErrConflict):
        return &http.HTTPError{StatusCode: 409, Message: "Conflict", Internal: err}
    default:
        return &http.HTTPError{StatusCode: 500, Message: "Internal error", Internal: err}
    }
}
```

### Error Middleware

```go
func ErrorMiddleware(next unicorn.HandlerExecutor) unicorn.HandlerExecutor {
    return func(ctx *unicorn.Context) error {
        err := next(ctx)
        if err != nil {
            // Log internal error
            ctx.Logger().Error("request failed",
                "error", err,
                "path", ctx.Request().Path,
            )
            
            // Return safe error to client
            return mapErrorToHTTP(err)
        }
        return nil
    }
}
```

## Security

### Input Validation

```go
type CreateUserRequest struct {
    Name     string `json:"name" validate:"required,min=2,max=100"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8,max=72"`
    Age      int    `json:"age" validate:"gte=0,lte=150"`
}

// Validate in handler or middleware
func validateRequest(req any) error {
    validate := validator.New()
    return validate.Struct(req)
}
```

### Sanitize Output

```go
// Never expose internal IDs or sensitive data
type UserResponse struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    CreatedAt time.Time `json:"created_at"`
    // Don't include: password hash, internal flags, etc.
}

func toUserResponse(user *User) *UserResponse {
    return &UserResponse{
        ID:        user.ID,
        Name:      user.Name,
        Email:     user.Email,
        CreatedAt: user.CreatedAt,
    }
}
```

### Secrets Management

```go
// Never hardcode secrets
// Bad
jwtSecret := []byte("my-secret-key")

// Good
jwtSecret := []byte(os.Getenv("JWT_SECRET"))

// Better - use secret manager
secretManager := unicorn.NewEnvSecretManager(unicorn.EnvSecretManagerConfig{
    Prefix: "APP_",
})
jwtSecret, _ := secretManager.Get(ctx, "JWT_SECRET")
```

### Rate Limiting by Resource

```go
// Different limits for different resources
func getRateLimitKey(ctx *unicorn.Context) string {
    identity := ctx.Identity()
    path := ctx.Request().Path
    
    if identity != nil {
        // Authenticated: limit by user
        return fmt.Sprintf("user:%s:%s", identity.ID, path)
    }
    
    // Anonymous: limit by IP
    ip := ctx.Request().Headers["X-Forwarded-For"]
    return fmt.Sprintf("ip:%s:%s", ip, path)
}

// Higher limits for authenticated users
func getRateLimit(ctx *unicorn.Context) int {
    if ctx.Identity() != nil {
        return 1000 // per minute
    }
    return 100 // per minute for anonymous
}
```

## Performance

### Use Connection Pooling

```go
// Database connection pool
dbConfig := &DatabaseConfig{
    MaxOpenConns:    25,
    MaxIdleConns:    5,
    ConnMaxLifetime: 5 * time.Minute,
    ConnMaxIdleTime: 1 * time.Minute,
}
```

### Cache Aggressively

```go
func GetUser(ctx *unicorn.Context) (*User, error) {
    userID := ctx.Request().Params["id"]
    cache := ctx.Cache()
    
    // Try cache first
    cacheKey := "user:" + userID
    if cached, err := cache.Get(ctx.Context(), cacheKey); err == nil {
        var user User
        json.Unmarshal(cached, &user)
        return &user, nil
    }
    
    // Fetch from database
    user, err := userRepo.FindByID(ctx.Context(), userID)
    if err != nil {
        return nil, err
    }
    
    // Cache for future requests
    data, _ := json.Marshal(user)
    cache.Set(ctx.Context(), cacheKey, data, time.Hour)
    
    return user, nil
}
```

### Async Operations

```go
func CreateOrder(ctx *unicorn.Context, req CreateOrderRequest) (*Order, error) {
    // Create order synchronously
    order, err := orderRepo.Create(ctx.Context(), req)
    if err != nil {
        return nil, err
    }
    
    // Publish event asynchronously
    go func() {
        broker := ctx.Broker()
        broker.Publish(context.Background(), "order.created", &unicorn.BrokerMessage{
            Key:  []byte(order.ID),
            Body: order.Marshal(),
        })
    }()
    
    return order, nil
}
```

### Pagination

```go
type ListRequest struct {
    Page     int `json:"page" validate:"gte=1"`
    PageSize int `json:"page_size" validate:"gte=1,lte=100"`
}

type ListResponse[T any] struct {
    Items    []T `json:"items"`
    Total    int `json:"total"`
    Page     int `json:"page"`
    PageSize int `json:"page_size"`
    HasMore  bool `json:"has_more"`
}

func ListUsers(ctx *unicorn.Context) (*ListResponse[User], error) {
    page, _ := strconv.Atoi(ctx.Request().Query["page"])
    if page < 1 {
        page = 1
    }
    pageSize, _ := strconv.Atoi(ctx.Request().Query["page_size"])
    if pageSize < 1 || pageSize > 100 {
        pageSize = 20
    }
    
    users, total, err := userRepo.List(ctx.Context(), page, pageSize)
    if err != nil {
        return nil, err
    }
    
    return &ListResponse[User]{
        Items:    users,
        Total:    total,
        Page:     page,
        PageSize: pageSize,
        HasMore:  page*pageSize < total,
    }, nil
}
```

## Testing

### Unit Testing Handlers

```go
func TestCreateUser(t *testing.T) {
    // Mock dependencies
    mockDB := &MockDatabase{}
    mockCache := &MockCache{}
    mockLogger := &MockLogger{}
    
    // Create context with mocks
    ctx := unicorn.NewTestContext()
    ctx.SetDB(mockDB)
    ctx.SetCache(mockCache)
    ctx.SetLogger(mockLogger)
    
    // Set up request
    ctx.SetRequest(&unicorn.Request{
        Method: "POST",
        Path:   "/users",
    })
    
    // Execute handler
    req := CreateUserRequest{Name: "Test", Email: "test@example.com"}
    user, err := CreateUser(ctx, req)
    
    // Assertions
    assert.NoError(t, err)
    assert.Equal(t, "Test", user.Name)
    assert.True(t, mockDB.CreateCalled)
}
```

### Integration Testing

```go
func TestAPI(t *testing.T) {
    // Create test app
    app := unicorn.New(&unicorn.Config{
        Name:       "test-app",
        EnableHTTP: true,
        HTTP: &unicorn.HTTPConfig{Port: 0}, // Random port
    })
    
    // Register handlers
    app.RegisterHandler(CreateUser).HTTP("POST", "/users").Done()
    
    // Start in background
    go app.Start()
    defer app.Shutdown()
    
    // Wait for server
    time.Sleep(100 * time.Millisecond)
    
    // Make request
    resp, err := http.Post(
        "http://localhost:"+app.Port()+"/users",
        "application/json",
        strings.NewReader(`{"name":"Test","email":"test@example.com"}`),
    )
    
    assert.NoError(t, err)
    assert.Equal(t, 201, resp.StatusCode)
}
```

### Table-Driven Tests

```go
func TestValidation(t *testing.T) {
    tests := []struct {
        name    string
        request CreateUserRequest
        wantErr bool
    }{
        {
            name:    "valid request",
            request: CreateUserRequest{Name: "Test", Email: "test@example.com"},
            wantErr: false,
        },
        {
            name:    "missing name",
            request: CreateUserRequest{Email: "test@example.com"},
            wantErr: true,
        },
        {
            name:    "invalid email",
            request: CreateUserRequest{Name: "Test", Email: "invalid"},
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validate(tt.request)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

## Deployment

### Configuration Management

```go
type Config struct {
    Server   ServerConfig   `yaml:"server"`
    Database DatabaseConfig `yaml:"database"`
    Cache    CacheConfig    `yaml:"cache"`
    Security SecurityConfig `yaml:"security"`
}

func LoadConfig() (*Config, error) {
    env := os.Getenv("APP_ENV")
    if env == "" {
        env = "development"
    }
    
    // Load base config
    config := &Config{}
    
    // Load from file
    data, err := os.ReadFile("config/" + env + ".yaml")
    if err != nil {
        return nil, err
    }
    yaml.Unmarshal(data, config)
    
    // Override with env vars
    if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
        config.Database.URL = dbURL
    }
    
    return config, nil
}
```

### Graceful Shutdown

```go
func main() {
    app := unicorn.New(config)
    
    // Setup handlers...
    
    // Handle shutdown signals
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    
    go func() {
        <-sigCh
        log.Println("Shutting down...")
        
        // Give time for in-flight requests
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        app.ShutdownWithContext(ctx)
    }()
    
    app.Start()
}
```

### Health Checks for Kubernetes

```go
// Liveness - is the process alive?
app.RegisterHandler(func(ctx *unicorn.Context) (map[string]string, error) {
    return map[string]string{"status": "alive"}, nil
}).HTTP("GET", "/health/live").Done()

// Readiness - can we serve traffic?
app.RegisterHandler(func(ctx *unicorn.Context) (*HealthResponse, error) {
    checks := map[string]string{}
    
    // Check database
    if err := ctx.DB().Health(); err != nil {
        return nil, &http.HTTPError{StatusCode: 503, Message: "Database unhealthy"}
    }
    checks["database"] = "ok"
    
    // Check cache
    if err := ctx.Cache().Health(); err != nil {
        return nil, &http.HTTPError{StatusCode: 503, Message: "Cache unhealthy"}
    }
    checks["cache"] = "ok"
    
    return &HealthResponse{Status: "ready", Checks: checks}, nil
}).HTTP("GET", "/health/ready").Done()
```

### Docker Deployment

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/server ./cmd/server

# Runtime stage
FROM alpine:3.18
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/server .
COPY config/ config/
EXPOSE 8080
CMD ["./server"]
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: unicorn-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: unicorn-app
  template:
    metadata:
      labels:
        app: unicorn-app
    spec:
      containers:
      - name: app
        image: myapp:latest
        ports:
        - containerPort: 8080
        env:
        - name: APP_ENV
          value: production
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: app-secrets
              key: database-url
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
```

## Resilience Patterns

### Circuit Breaker for External Services

```go
import "github.com/madcok-co/unicorn/core/pkg/resilience/circuitbreaker"

// Create circuit breaker for payment service
paymentCB := circuitbreaker.New("payment-service", circuitbreaker.Config{
    MaxRequests: 3,                    // Requests in half-open state
    Timeout:     30 * time.Second,     // Time to wait before half-open
    ReadyToTrip: func(counts circuitbreaker.Counts) bool {
        return counts.ConsecutiveFailures > 5
    },
    OnStateChange: func(name string, from, to circuitbreaker.State) {
        log.Printf("Circuit breaker %s: %s -> %s", name, from, to)
    },
})

func ProcessPayment(ctx *unicorn.Context, req PaymentRequest) error {
    result, err := paymentCB.Execute(func() (interface{}, error) {
        return paymentGateway.Charge(req)
    })
    if err != nil {
        if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
            // Circuit is open, return cached/fallback response
            return ErrServiceUnavailable
        }
        return err
    }
    return ctx.JSON(200, result)
}
```

### Retry with Exponential Backoff

```go
import "github.com/madcok-co/unicorn/core/pkg/resilience/retry"

func CallExternalAPI(ctx context.Context) error {
    return retry.Do(ctx, func() error {
        return externalAPI.Call()
    },
        retry.WithMaxAttempts(3),
        retry.WithBackoff(time.Second),        // Initial backoff
        retry.WithMaxBackoff(30*time.Second),  // Max backoff cap
        retry.WithRetryIf(func(err error) bool {
            // Only retry on transient errors
            return isTransientError(err)
        }),
    )
}
```

### Bulkhead Pattern

```go
import "github.com/madcok-co/unicorn/core/pkg/resilience/retry"

// Limit concurrent calls to external service
bulkhead := retry.NewBulkhead(10) // Max 10 concurrent

func CallWithBulkhead(ctx context.Context) error {
    return bulkhead.Execute(ctx, func() error {
        return slowExternalService.Call()
    })
}
```

### Timeout Pattern

```go
import "github.com/madcok-co/unicorn/core/pkg/resilience/retry"

func CallWithTimeout(ctx context.Context) error {
    return retry.WithTimeout(ctx, 5*time.Second, func(ctx context.Context) error {
        return externalService.Call(ctx)
    })
}
```

### Fallback Pattern

```go
import "github.com/madcok-co/unicorn/core/pkg/resilience/retry"

func GetUserWithFallback(ctx context.Context, userID string) (*User, error) {
    return retry.WithFallback(
        func() (*User, error) {
            return userService.GetUser(ctx, userID)
        },
        func(err error) (*User, error) {
            // Return cached/default user on failure
            return cache.GetUser(userID)
        },
    )
}
```

### Combining Patterns

```go
func ResilientExternalCall(ctx context.Context, req Request) (*Response, error) {
    // 1. Circuit breaker wraps everything
    result, err := circuitBreaker.Execute(func() (interface{}, error) {
        // 2. Bulkhead limits concurrency
        return bulkhead.Execute(ctx, func() (*Response, error) {
            // 3. Retry with backoff
            var resp *Response
            err := retry.Do(ctx, func() error {
                var err error
                // 4. Timeout per attempt
                err = retry.WithTimeout(ctx, 5*time.Second, func(ctx context.Context) error {
                    resp, err = externalService.Call(ctx, req)
                    return err
                })
                return err
            }, retry.WithMaxAttempts(3))
            return resp, err
        })
    })
    
    if err != nil {
        // 5. Fallback on failure
        return getFallbackResponse(req)
    }
    
    return result.(*Response), nil
}
```

## Monitoring Checklist

- [ ] Structured logging with request IDs
- [ ] Metrics for requests, latency, errors
- [ ] Distributed tracing
- [ ] Health check endpoints
- [ ] Alerting on error rates
- [ ] Dashboard for key metrics
- [ ] Log aggregation
- [ ] Error tracking (Sentry, etc.)
- [ ] Circuit breaker state monitoring
- [ ] Retry attempt metrics
- [ ] Bulkhead rejection metrics
