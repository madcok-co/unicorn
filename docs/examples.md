# Examples

Complete examples demonstrating Unicorn framework features.

## Basic REST API

A simple CRUD API for users:

```go
package main

import (
    "errors"
    "log"
    "time"

    httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
    "github.com/madcok-co/unicorn/core/pkg/app"
    "github.com/madcok-co/unicorn/core/pkg/context"
)

// DTOs
type User struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    CreatedAt time.Time `json:"created_at"`
}

type CreateUserRequest struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

type UpdateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

// In-memory store (use real database in production)
var users = make(map[string]*User)

// Handlers
func CreateUser(ctx *context.Context, req CreateUserRequest) (*User, error) {
    user := &User{
        ID:        "user-" + time.Now().Format("20060102150405"),
        Name:      req.Name,
        Email:     req.Email,
        CreatedAt: time.Now(),
    }
    users[user.ID] = user
    
    ctx.Logger().Info("user created", "id", user.ID)
    return user, nil
}

func GetUser(ctx *context.Context) (*User, error) {
    id := ctx.Request().Params["id"]
    user, ok := users[id]
    if !ok {
        return nil, &httpAdapter.HTTPError{
            StatusCode: 404,
            Message:    "User not found",
        }
    }
    return user, nil
}

func ListUsers(ctx *context.Context) ([]*User, error) {
    result := make([]*User, 0, len(users))
    for _, u := range users {
        result = append(result, u)
    }
    return result, nil
}

func UpdateUser(ctx *context.Context, req UpdateUserRequest) (*User, error) {
    id := ctx.Request().Params["id"]
    user, ok := users[id]
    if !ok {
        return nil, &httpAdapter.HTTPError{StatusCode: 404, Message: "User not found"}
    }
    
    if req.Name != "" {
        user.Name = req.Name
    }
    if req.Email != "" {
        user.Email = req.Email
    }
    
    return user, nil
}

func DeleteUser(ctx *context.Context) (map[string]string, error) {
    id := ctx.Request().Params["id"]
    if _, ok := users[id]; !ok {
        return nil, &httpAdapter.HTTPError{StatusCode: 404, Message: "User not found"}
    }
    
    delete(users, id)
    return map[string]string{"message": "User deleted"}, nil
}

func HealthCheck(ctx *context.Context) (map[string]string, error) {
    return map[string]string{"status": "healthy"}, nil
}

func main() {
    application := app.New(&app.Config{
        Name:       "user-api",
        Version:    "1.0.0",
        EnableHTTP: true,
        HTTP: &httpAdapter.Config{
            Host: "0.0.0.0",
            Port: 8080,
        },
    })

    // Register routes
    application.RegisterHandler(HealthCheck).HTTP("GET", "/health").Done()
    application.RegisterHandler(CreateUser).HTTP("POST", "/users").Done()
    application.RegisterHandler(ListUsers).HTTP("GET", "/users").Done()
    application.RegisterHandler(GetUser).HTTP("GET", "/users/:id").Done()
    application.RegisterHandler(UpdateUser).HTTP("PUT", "/users/:id").Done()
    application.RegisterHandler(DeleteUser).HTTP("DELETE", "/users/:id").Done()

    log.Println("Starting server on :8080")
    if err := application.Start(); err != nil {
        log.Fatal(err)
    }
}
```

## JWT Authentication

API with JWT authentication:

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/madcok-co/unicorn/core"
    "github.com/madcok-co/unicorn/pkg/adapters/http"
)

var jwtAuth *unicorn.JWTAuthenticator

type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type LoginResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int64  `json:"expires_in"`
}

type RefreshRequest struct {
    RefreshToken string `json:"refresh_token"`
}

type ProfileResponse struct {
    ID    string `json:"id"`
    Email string `json:"email"`
    Roles []string `json:"roles"`
}

// Public: Login
func Login(ctx *unicorn.Context, req LoginRequest) (*LoginResponse, error) {
    // In production: verify against database
    if req.Email != "admin@example.com" || req.Password != "password123" {
        return nil, &http.HTTPError{StatusCode: 401, Message: "Invalid credentials"}
    }
    
    tokenPair, err := jwtAuth.Authenticate(ctx.Context(), unicorn.Credentials{
        Type:     "jwt",
        Username: req.Email,
        Metadata: map[string]string{
            "user_id": "user-123",
            "role":    "admin",
        },
    })
    if err != nil {
        return nil, err
    }
    
    return &LoginResponse{
        AccessToken:  tokenPair.AccessToken,
        RefreshToken: tokenPair.RefreshToken,
        ExpiresIn:    tokenPair.ExpiresIn,
    }, nil
}

// Public: Refresh token
func RefreshToken(ctx *unicorn.Context, req RefreshRequest) (*LoginResponse, error) {
    tokenPair, err := jwtAuth.Refresh(ctx.Context(), req.RefreshToken)
    if err != nil {
        return nil, &http.HTTPError{StatusCode: 401, Message: "Invalid refresh token"}
    }
    
    return &LoginResponse{
        AccessToken:  tokenPair.AccessToken,
        RefreshToken: tokenPair.RefreshToken,
        ExpiresIn:    tokenPair.ExpiresIn,
    }, nil
}

// Protected: Get profile
func GetProfile(ctx *unicorn.Context) (*ProfileResponse, error) {
    identity := ctx.Identity()
    if identity == nil {
        return nil, &http.HTTPError{StatusCode: 401, Message: "Unauthorized"}
    }
    
    return &ProfileResponse{
        ID:    identity.ID,
        Email: identity.Email,
        Roles: identity.Roles,
    }, nil
}

// Protected: Logout
func Logout(ctx *unicorn.Context) (map[string]string, error) {
    token := ctx.Request().Headers["Authorization"]
    if len(token) > 7 {
        token = token[7:] // Remove "Bearer "
    }
    
    if err := jwtAuth.Revoke(ctx.Context(), token); err != nil {
        return nil, err
    }
    
    return map[string]string{"message": "Logged out"}, nil
}

// Auth middleware
func authMiddleware(next unicorn.HandlerExecutor) unicorn.HandlerExecutor {
    return func(ctx *unicorn.Context) error {
        token := ctx.Request().Headers["Authorization"]
        if len(token) > 7 {
            token = token[7:] // Remove "Bearer "
        }
        
        identity, err := jwtAuth.Validate(ctx.Context(), token)
        if err != nil {
            return &http.HTTPError{StatusCode: 401, Message: "Unauthorized"}
        }
        
        ctx.SetIdentity(identity)
        return next(ctx)
    }
}

func main() {
    // Initialize JWT authenticator
    jwtAuth = unicorn.NewJWTAuthenticator(unicorn.JWTConfig{
        SecretKey:          []byte("your-256-bit-secret-key-here!!!"),
        Issuer:             "my-app",
        AccessTokenExpiry:  15 * time.Minute,
        RefreshTokenExpiry: 7 * 24 * time.Hour,
    })

    application := app.New(&app.Config{
        Name:       "auth-api",
        EnableHTTP: true,
        HTTP:       &httpAdapter.Config{Port: 8080},
    })

    // Public routes
    application.RegisterHandler(Login).HTTP("POST", "/auth/login").Done()
    application.RegisterHandler(RefreshToken).HTTP("POST", "/auth/refresh").Done()
    
    // Protected routes
    application.RegisterHandler(GetProfile).
        Use(authMiddleware).
        HTTP("GET", "/profile").
        Done()
    
    application.RegisterHandler(Logout).
        Use(authMiddleware).
        HTTP("POST", "/auth/logout").
        Done()

    log.Println("Auth API starting on :8080")
    application.Start()
}
```

## Rate Limited API

API with rate limiting:

```go
package main

import (
    "log"
    "time"

    httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
    "github.com/madcok-co/unicorn/core/pkg/app"
    "github.com/madcok-co/unicorn/core/pkg/context"
    "github.com/madcok-co/unicorn/core"
)

var rateLimiter *unicorn.InMemoryRateLimiter

func rateLimitMiddleware(next context.HandlerFunc) context.HandlerFunc {
    return func(ctx *context.Context) error {
        // Use IP as rate limit key
        ip := ctx.Request().Headers["X-Forwarded-For"]
        if ip == "" {
            ip = "unknown"
        }
        
        allowed, err := rateLimiter.Allow(ctx.Context(), ip)
        if err != nil {
            return err
        }
        
        if !allowed {
            remaining, _ := rateLimiter.Remaining(ctx.Context(), ip)
            return &httpAdapter.HTTPError{
                StatusCode: 429,
                Message:    "Rate limit exceeded",
                Internal:   nil,
            }
        }
        
        return next(ctx)
    }
}

func ExpensiveOperation(ctx *context.Context) (map[string]string, error) {
    time.Sleep(100 * time.Millisecond) // Simulate work
    return map[string]string{"result": "success"}, nil
}

func main() {
    // 10 requests per minute
    rateLimiter = unicorn.NewInMemoryRateLimiter(unicorn.InMemoryRateLimiterConfig{
        MaxTokens:       10,
        RefillRate:      10,
        RefillInterval:  time.Minute,
        CleanupInterval: 5 * time.Minute,
    })

    application := app.New(&app.Config{
        Name:       "rate-limited-api",
        EnableHTTP: true,
        HTTP:       &httpAdapter.Config{Port: 8080},
    })

    application.RegisterHandler(ExpensiveOperation).
        Use(rateLimitMiddleware).
        HTTP("POST", "/expensive").
        Done()

    log.Println("Rate limited API on :8080")
    application.Start()
}
```

## Multi-Service Application

Microservices in a monolith:

```go
package main

import (
    "context"
    "flag"
    "log"
    "strings"

    "github.com/madcok-co/unicorn/core/pkg/app"
    "github.com/madcok-co/unicorn/core"
)

// User Service DTOs
type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

// Order Service DTOs
type Order struct {
    ID     string  `json:"id"`
    UserID string  `json:"user_id"`
    Total  float64 `json:"total"`
    Status string  `json:"status"`
}

type CreateOrderRequest struct {
    UserID string  `json:"user_id"`
    Total  float64 `json:"total"`
}

// User Service Handlers
func CreateUser(ctx *unicorn.Context, req CreateUserRequest) (*User, error) {
    return &User{ID: "user-1", Name: req.Name, Email: req.Email}, nil
}

func GetUser(ctx *unicorn.Context) (*User, error) {
    id := ctx.Request().Params["id"]
    return &User{ID: id, Name: "John", Email: "john@example.com"}, nil
}

// Order Service Handlers
func CreateOrder(ctx *unicorn.Context, req CreateOrderRequest) (*Order, error) {
    return &Order{
        ID:     "order-1",
        UserID: req.UserID,
        Total:  req.Total,
        Status: "pending",
    }, nil
}

func GetOrder(ctx *unicorn.Context) (*Order, error) {
    id := ctx.Request().Params["id"]
    return &Order{ID: id, UserID: "user-1", Total: 99.99, Status: "completed"}, nil
}

// Notification Service Handler
func SendNotification(ctx *unicorn.Context, req map[string]string) (map[string]string, error) {
    log.Printf("Sending notification to %s: %s", req["user_id"], req["message"])
    return map[string]string{"status": "sent"}, nil
}

func main() {
    services := flag.String("services", "", "Services to run (comma-separated)")
    port := flag.Int("port", 8080, "HTTP port")
    flag.Parse()

    application := app.New(&app.Config{
        Name:       "multiservice-app",
        EnableHTTP: true,
        HTTP:       &httpAdapter.Config{Port: *port},
    })

    // User Service
    application.Service("user-service").
        Describe("User management").
        OnStart(func(ctx context.Context) error {
            log.Println("[user-service] Starting")
            return nil
        }).
        Register(CreateUser).HTTP("POST", "/users").Done().
        Register(GetUser).HTTP("GET", "/users/:id").Done()

    // Order Service
    application.Service("order-service").
        Describe("Order processing").
        DependsOn("user-service").
        OnStart(func(ctx context.Context) error {
            log.Println("[order-service] Starting")
            return nil
        }).
        Register(CreateOrder).HTTP("POST", "/orders").Done().
        Register(GetOrder).HTTP("GET", "/orders/:id").Done()

    // Notification Service
    application.Service("notification-service").
        Describe("Send notifications").
        Register(SendNotification).HTTP("POST", "/notifications").Done()

    // Run specific services or all
    var servicesToRun []string
    if *services != "" {
        servicesToRun = strings.Split(*services, ",")
    }

    log.Printf("Starting on :%d", *port)
    if len(servicesToRun) > 0 {
        log.Printf("Running services: %v", servicesToRun)
        application.RunServices(servicesToRun...)
    } else {
        log.Println("Running all services")
        application.Start()
    }
}
```

Run specific services:
```bash
# Run all
./app

# Run only user service
./app -services=user-service

# Run user and order services
./app -services=user-service,order-service
```

## Message Broker Integration

Event-driven architecture:

```go
package main

import (
    "context"
    "log"

    brokerMem "github.com/madcok-co/unicorn/core/pkg/adapters/broker/memory"
    "github.com/madcok-co/unicorn/core/pkg/app"
    "github.com/madcok-co/unicorn/core/pkg/context"
    "github.com/madcok-co/unicorn/core/pkg/contracts"
)

type OrderCreatedEvent struct {
    OrderID string  `json:"order_id"`
    UserID  string  `json:"user_id"`
    Total   float64 `json:"total"`
}

type CreateOrderRequest struct {
    UserID string  `json:"user_id"`
    Total  float64 `json:"total"`
}

type Order struct {
    ID     string  `json:"id"`
    UserID string  `json:"user_id"`
    Total  float64 `json:"total"`
}

// HTTP Handler - creates order and publishes event
func CreateOrder(ctx *context.Context, req CreateOrderRequest) (*Order, error) {
    order := &Order{
        ID:     "order-123",
        UserID: req.UserID,
        Total:  req.Total,
    }
    
    // Publish event to message broker
    broker := ctx.Broker()
    if broker != nil {
        event := OrderCreatedEvent{
            OrderID: order.ID,
            UserID:  order.UserID,
            Total:   order.Total,
        }
        
        broker.Publish(ctx.Context(), "order.created", &contracts.BrokerMessage{
            Key:  []byte(order.ID),
            Body: mustJSON(event),
        })
    }
    
    return order, nil
}

// Message Handler - processes order events
func ProcessOrderEvent(ctx *context.Context, event OrderCreatedEvent) error {
    log.Printf("Processing order: %s for user: %s, total: %.2f",
        event.OrderID, event.UserID, event.Total)
    
    // Business logic: send email, update inventory, etc.
    return nil
}

// Message Handler - sends notification on order
func SendOrderNotification(ctx *context.Context, event OrderCreatedEvent) error {
    log.Printf("Sending notification for order: %s to user: %s",
        event.OrderID, event.UserID)
    return nil
}

func main() {
    // Create in-memory broker (use Kafka/RabbitMQ in production)
    broker := brokerMem.New()
    broker.Connect(context.Background())

    application := app.New(&app.Config{
        Name:         "event-driven-app",
        EnableHTTP:   true,
        EnableBroker: true,
        HTTP:         &httpAdapter.Config{Port: 8080},
    })
    
    application.SetBroker(broker)

    // HTTP endpoint to create orders
    application.RegisterHandler(CreateOrder).
        HTTP("POST", "/orders").
        Done()

    // Message handlers for order.created topic
    application.RegisterHandler(ProcessOrderEvent).
        Message("order.created").
        Done()

    application.RegisterHandler(SendOrderNotification).
        Message("order.created").
        Done()

    log.Println("Event-driven app on :8080")
    application.Start()
}

func mustJSON(v any) []byte {
    // Simple JSON marshaling
    return []byte(`{}`)
}
```

## Full Security Example

Complete security implementation:

```go
package main

import (
    "log"
    "time"

    httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
    "github.com/madcok-co/unicorn/core/pkg/app"
    "github.com/madcok-co/unicorn/core/pkg/context"
    "github.com/madcok-co/unicorn/core"
)

var (
    jwtAuth     *unicorn.JWTAuthenticator
    rateLimiter *unicorn.InMemoryRateLimiter
    auditLogger *unicorn.InMemoryAuditLogger
    hasher      *unicorn.BcryptHasher
)

// DTOs
type RegisterRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
    Name     string `json:"name"`
}

type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type AuthResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
}

// In-memory user store
var users = make(map[string]*StoredUser)

type StoredUser struct {
    ID           string
    Email        string
    Name         string
    PasswordHash []byte
    Roles        []string
}

// Handlers
func Register(ctx *context.Context, req RegisterRequest) (*AuthResponse, error) {
    // Check if user exists
    if _, exists := users[req.Email]; exists {
        return nil, &httpAdapter.HTTPError{StatusCode: 409, Message: "User already exists"}
    }
    
    // Hash password
    hash, err := hasher.Hash([]byte(req.Password))
    if err != nil {
        return nil, err
    }
    
    // Create user
    user := &StoredUser{
        ID:           "user-" + time.Now().Format("20060102150405"),
        Email:        req.Email,
        Name:         req.Name,
        PasswordHash: hash,
        Roles:        []string{"user"},
    }
    users[req.Email] = user
    
    // Audit log
    auditLogger.Log(ctx.Context(), unicorn.NewAuditEvent().
        Action(unicorn.AuditActionCreate).
        Resource("users").
        ResourceID(user.ID).
        Actor(user.ID, "user", user.Name).
        Success(true).
        Build())
    
    // Generate tokens
    tokenPair, err := jwtAuth.Authenticate(ctx.Context(), unicorn.Credentials{
        Type:     "jwt",
        Username: user.Email,
        Metadata: map[string]string{
            "user_id": user.ID,
            "role":    "user",
        },
    })
    if err != nil {
        return nil, err
    }
    
    return &AuthResponse{
        AccessToken:  tokenPair.AccessToken,
        RefreshToken: tokenPair.RefreshToken,
    }, nil
}

func Login(ctx *context.Context, req LoginRequest) (*AuthResponse, error) {
    user, exists := users[req.Email]
    if !exists {
        auditLogger.Log(ctx.Context(), unicorn.NewAuditEvent().
            Action(unicorn.AuditActionLogin).
            Resource("auth").
            Success(false).
            WithError(nil).
            Build())
        return nil, &httpAdapter.HTTPError{StatusCode: 401, Message: "Invalid credentials"}
    }
    
    // Verify password
    if !hasher.Verify([]byte(req.Password), user.PasswordHash) {
        return nil, &httpAdapter.HTTPError{StatusCode: 401, Message: "Invalid credentials"}
    }
    
    // Generate tokens
    tokenPair, err := jwtAuth.Authenticate(ctx.Context(), unicorn.Credentials{
        Type:     "jwt",
        Username: user.Email,
        Metadata: map[string]string{
            "user_id": user.ID,
            "role":    user.Roles[0],
        },
    })
    if err != nil {
        return nil, err
    }
    
    // Audit log
    auditLogger.Log(ctx.Context(), unicorn.NewAuditEvent().
        Action(unicorn.AuditActionLogin).
        Resource("auth").
        Actor(user.ID, "user", user.Name).
        Success(true).
        Build())
    
    return &AuthResponse{
        AccessToken:  tokenPair.AccessToken,
        RefreshToken: tokenPair.RefreshToken,
    }, nil
}

func GetProfile(ctx *context.Context) (map[string]any, error) {
    identity := ctx.Identity()
    return map[string]any{
        "id":    identity.ID,
        "email": identity.Email,
        "roles": identity.Roles,
    }, nil
}

// Middleware
func authMiddleware(next context.HandlerFunc) context.HandlerFunc {
    return func(ctx *context.Context) error {
        auth := ctx.Request().Headers["Authorization"]
        if len(auth) < 8 {
            return &httpAdapter.HTTPError{StatusCode: 401, Message: "Missing token"}
        }
        
        token := auth[7:] // Remove "Bearer "
        identity, err := jwtAuth.Validate(ctx.Context(), token)
        if err != nil {
            return &httpAdapter.HTTPError{StatusCode: 401, Message: "Invalid token"}
        }
        
        ctx.SetIdentity(identity)
        return next(ctx)
    }
}

func rateLimitMiddleware(next context.HandlerFunc) context.HandlerFunc {
    return func(ctx *context.Context) error {
        ip := ctx.Request().Headers["X-Forwarded-For"]
        if ip == "" {
            ip = "unknown"
        }
        
        allowed, _ := rateLimiter.Allow(ctx.Context(), ip)
        if !allowed {
            return &httpAdapter.HTTPError{StatusCode: 429, Message: "Too many requests"}
        }
        
        return next(ctx)
    }
}

func main() {
    // Initialize security components
    jwtAuth = unicorn.NewJWTAuthenticator(unicorn.JWTConfig{
        SecretKey:          []byte("your-super-secret-256-bit-key!!"),
        Issuer:             "secure-app",
        AccessTokenExpiry:  15 * time.Minute,
        RefreshTokenExpiry: 7 * 24 * time.Hour,
    })
    
    rateLimiter = unicorn.NewInMemoryRateLimiter(unicorn.InMemoryRateLimiterConfig{
        MaxTokens:       100,
        RefillRate:      100,
        RefillInterval:  time.Minute,
        CleanupInterval: 5 * time.Minute,
    })
    
    auditLogger = unicorn.NewInMemoryAuditLogger(unicorn.InMemoryAuditLoggerConfig{
        MaxEvents:       10000,
        BufferSize:      100,
        CleanupInterval: time.Hour,
        RetentionPeriod: 30 * 24 * time.Hour,
    })
    defer auditLogger.Close()
    
    hasher = unicorn.NewBcryptHasher(unicorn.BcryptConfig{Cost: 12})

    application := app.New(&app.Config{
        Name:       "secure-app",
        EnableHTTP: true,
        HTTP:       &httpAdapter.Config{Port: 8080},
    })

    // Public routes (with rate limiting)
    application.RegisterHandler(Register).
        Use(rateLimitMiddleware).
        HTTP("POST", "/auth/register").
        Done()

    application.RegisterHandler(Login).
        Use(rateLimitMiddleware).
        HTTP("POST", "/auth/login").
        Done()

    // Protected routes
    application.RegisterHandler(GetProfile).
        Use(rateLimitMiddleware, authMiddleware).
        HTTP("GET", "/profile").
        Done()

    log.Println("Secure API on :8080")
    application.Start()
}
```

## Testing Example

```go
package handlers_test

import (
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/madcok-co/unicorn/core/pkg/context"
)

func TestCreateUser(t *testing.T) {
    tests := []struct {
        name    string
        request CreateUserRequest
        wantErr bool
    }{
        {
            name: "valid user",
            request: CreateUserRequest{
                Name:  "John Doe",
                Email: "john@example.com",
            },
            wantErr: false,
        },
        {
            name: "missing name",
            request: CreateUserRequest{
                Email: "john@example.com",
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx := context.New(context.Background())
            
            user, err := CreateUser(ctx, tt.request)
            
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.NotEmpty(t, user.ID)
                assert.Equal(t, tt.request.Name, user.Name)
            }
        })
    }
}
```

## Resilience Patterns Example

```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"

    "github.com/madcok-co/unicorn/core"
    "github.com/madcok-co/unicorn/core/pkg/middleware"
    "github.com/madcok-co/unicorn/core/pkg/resilience/circuitbreaker"
    "github.com/madcok-co/unicorn/core/pkg/resilience/retry"
)

// External service client
type PaymentGateway struct {
    baseURL string
    cb      *circuitbreaker.CircuitBreaker
}

func NewPaymentGateway(baseURL string) *PaymentGateway {
    cb := circuitbreaker.New("payment-gateway", circuitbreaker.Config{
        MaxRequests: 3,
        Timeout:     30 * time.Second,
        ReadyToTrip: func(counts circuitbreaker.Counts) bool {
            return counts.ConsecutiveFailures > 5
        },
        OnStateChange: func(name string, from, to circuitbreaker.State) {
            log.Printf("Circuit %s: %s -> %s", name, from, to)
        },
    })
    
    return &PaymentGateway{baseURL: baseURL, cb: cb}
}

func (p *PaymentGateway) Charge(ctx context.Context, amount float64) error {
    _, err := p.cb.Execute(func() (interface{}, error) {
        // Retry with exponential backoff
        return nil, retry.Do(ctx, func() error {
            // Timeout per attempt
            return retry.WithTimeout(ctx, 5*time.Second, func(ctx context.Context) error {
                // Actual HTTP call
                req, _ := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/charge", nil)
                resp, err := http.DefaultClient.Do(req)
                if err != nil {
                    return err
                }
                defer resp.Body.Close()
                
                if resp.StatusCode >= 500 {
                    return retry.ErrRetryable // Will be retried
                }
                return nil
            })
        },
            retry.WithMaxAttempts(3),
            retry.WithBackoff(500*time.Millisecond),
        )
    })
    return err
}

// Handler with resilience
type PaymentRequest struct {
    Amount float64 `json:"amount"`
}

func ProcessPayment(ctx *context.Context, req PaymentRequest) (*map[string]string, error) {
    gateway := ctx.Get("payment_gateway").(*PaymentGateway)
    
    if err := gateway.Charge(ctx.Context(), req.Amount); err != nil {
        if err == circuitbreaker.ErrCircuitOpen {
            return nil, ctx.Error(503, "Payment service temporarily unavailable")
        }
        return nil, ctx.Error(500, "Payment failed")
    }
    
    return &map[string]string{"status": "success"}, nil
}

// Health check with dependency checks
func setupHealthChecks(application *app.App) {
    health := middleware.NewHealthHandler()
    
    // Database check
    health.AddCheck("database", func(ctx context.Context) error {
        // db.PingContext(ctx)
        return nil
    })
    
    // Redis check
    health.AddCheck("redis", func(ctx context.Context) error {
        // redis.Ping(ctx)
        return nil
    })
    
    // Payment gateway check (uses circuit breaker state)
    health.AddCheck("payment_gateway", func(ctx context.Context) error {
        // Check circuit breaker state
        return nil
    })
    
    app.RegisterHandler(health.LivenessHandler()).
        HTTP("GET", "/health/live").Done()
    app.RegisterHandler(health.ReadinessHandler()).
        HTTP("GET", "/health/ready").Done()
}

func main() {
    application := app.New(&app.Config{
        Name:       "payment-service",
        EnableHTTP: true,
    })
    
    // Production middleware applied at handler level
    application.RegisterHandler(ProcessPayment).
        Use(middleware.Recovery()).
        Use(middleware.CORS(middleware.CORSConfig{
            AllowOrigins: []string{"*"},
        })).
        Use(middleware.Timeout(30 * time.Second)).
        Use(middleware.RateLimit(middleware.RateLimitConfig{
            Max:      100,
            Duration: time.Minute,
        })).
        HTTP("POST", "/payments").
        Done()
    
    // Inject payment gateway with circuit breaker
    gateway := NewPaymentGateway("https://api.payment.com")
    application.Set("payment_gateway", gateway)
    
    // Setup health checks
    setupHealthChecks(application)
    
    application.Start()
}
```

## Next Steps

- Review the [API Reference](./api-reference.md) for complete documentation
- Check [Best Practices](./best-practices.md) for production recommendations
- Explore [Security](./security.md) for advanced security features
- Learn about [Middleware](./middleware.md) for production-ready middleware
- Understand [Resilience Patterns](./resilience.md) for fault tolerance
