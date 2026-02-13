# Handlers & Triggers

This document covers handler patterns, trigger types, and middleware in detail.

## Handler Basics

Handlers are the core of your application. They contain only business logic.

### Handler Signatures

```go
// Full signature: Context + Request -> Response + Error
func CreateUser(ctx *context.Context, req CreateUserRequest) (*User, error)

// No request body: Context -> Response + Error
func GetHealth(ctx *context.Context) (*HealthResponse, error)

// List response
func ListUsers(ctx *context.Context) ([]*User, error)

// No response (side effect only)
func LogEvent(ctx *context.Context, req LogRequest) error
```

### Request DTOs

Use struct tags for validation and serialization:

```go
type CreateUserRequest struct {
    Name     string `json:"name" validate:"required,min=2,max=100"`
    Email    string `json:"email" validate:"required,email"`
    Age      int    `json:"age" validate:"gte=0,lte=150"`
    Password string `json:"password" validate:"required,min=8"`
}
```

### Response DTOs

```go
type User struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    CreatedAt time.Time `json:"created_at"`
}

// Pagination response
type UserListResponse struct {
    Users      []*User `json:"users"`
    Total      int     `json:"total"`
    Page       int     `json:"page"`
    PageSize   int     `json:"page_size"`
}
```

## Handler Registration

### Basic Registration

```go
app.RegisterHandler(CreateUser).
    Named("create-user").
    HTTP("POST", "/users").
    Done()
```

### With Multiple Triggers

```go
app.RegisterHandler(ProcessOrder).
    Named("process-order").
    HTTP("POST", "/orders").
    Message("order.create.command").
    Cron("*/5 * * * *").
    Done()
```

### With Middleware

```go
app.RegisterHandler(AdminAction).
    Named("admin-action").
    Use(authMiddleware, adminOnlyMiddleware).
    HTTP("POST", "/admin/action").
    Done()
```

## Trigger Types

### HTTP Trigger

```go
// Basic HTTP
HTTP("GET", "/users")
HTTP("POST", "/users")
HTTP("PUT", "/users/:id")
HTTP("DELETE", "/users/:id")

// Path parameters
HTTP("GET", "/users/:id")
HTTP("GET", "/orders/:orderId/items/:itemId")
```

**Accessing HTTP Data:**

```go
func GetUser(ctx *context.Context) (*User, error) {
    // Path parameters
    userID := ctx.Request().Params["id"]
    // Or use helper method
    userID := ctx.Request().Param("id")
    
    // Query parameters
    include := ctx.Request().Query["include"]
    // Or use helper method
    include := ctx.Request().QueryParam("include")
    
    // Headers
    authHeader := ctx.Request().Headers["Authorization"]
    // Or use helper method
    authHeader := ctx.Request().Header("Authorization")
    
    // Raw body (if needed)
    body := ctx.Request().Body
    
    return &User{ID: userID}, nil
}
```

### Message Trigger

For message brokers (Kafka, RabbitMQ, Redis, NATS):

> **Note:** The `Message()` trigger is the recommended way to work with message brokers. The older `Kafka()` trigger is deprecated and will be removed in a future version. Please use `Message()` for all new code.

```go
// Basic message subscription
Message("user.created")

// With options
Message("order.created", 
    handler.WithGroup("order-processor"),
    handler.WithAutoAck(false),
    handler.WithRetries(3, time.Second),
)
```

**Message Options:**

| Option | Description |
|--------|-------------|
| `WithGroup(group)` | Set consumer group ID |
| `WithAutoAck(bool)` | Enable/disable auto-acknowledgment |
| `WithRetries(n, backoff)` | Number of retry attempts and backoff duration |
| `WithDeadLetter(topic)` | Dead letter topic for failed messages |

**Accessing Message Data:**

```go
func ProcessUserEvent(ctx *context.Context, req UserEvent) error {
    // Message metadata
    topic := ctx.Request().Topic
    key := ctx.Request().Key
    partition := ctx.Request().Partition
    offset := ctx.Request().Offset
    
    // Check trigger type
    if ctx.Request().TriggerType == "message" {
        // Message-specific logic
    }
    
    return nil
}
```

### Cron Trigger

```go
// Standard cron syntax
Cron("0 * * * *")     // Every hour
Cron("*/5 * * * *")   // Every 5 minutes
Cron("0 0 * * *")     // Daily at midnight
Cron("0 0 * * 0")     // Weekly on Sunday

// Cron expression format:
// ┌───────────── minute (0-59)
// │ ┌───────────── hour (0-23)
// │ │ ┌───────────── day of month (1-31)
// │ │ │ ┌───────────── month (1-12)
// │ │ │ │ ┌───────────── day of week (0-6, Sunday=0)
// │ │ │ │ │
// * * * * *
```

### gRPC Trigger

```go
GRPC("UserService", "CreateUser")
GRPC("OrderService", "ProcessOrder")
```

## Context Usage

### Request Information

```go
func MyHandler(ctx *context.Context, req MyRequest) (*Response, error) {
    // Trigger type: "http", "message", "cron", "grpc"
    triggerType := ctx.Request().TriggerType
    
    // Request metadata
    method := ctx.Request().Method
    path := ctx.Request().Path
    
    // For message triggers
    topic := ctx.Request().Topic
    partition := ctx.Request().Partition
    offset := ctx.Request().Offset
    
    return &Response{}, nil
}
```

### Infrastructure Access

```go
func CreateUser(ctx *context.Context, req CreateUserRequest) (*User, error) {
    user := &User{Name: req.Name, Email: req.Email}
    
    // Database
    if err := ctx.DB().Create(ctx.Context(), &user); err != nil {
        return nil, err
    }
    
    // Cache
    ctx.Cache().Set(ctx.Context(), "user:"+user.ID, user, time.Hour)
    
    // Logger (structured logging)
    ctx.Logger().Info("user created", 
        "id", user.ID, 
        "email", user.Email,
    )
    
    // Message Broker
    msg := &contracts.BrokerMessage{
        Key:  []byte(user.ID),
        Body: []byte(`{"id": "` + user.ID + `"}`),
    }
    ctx.Broker().Publish(ctx.Context(), "user.created", msg)
    
    // Metrics
    ctx.Metrics().Counter("users_created_total").Inc()
    
    // Tracer
    span := ctx.Tracer().StartSpan("create_user")
    defer span.End()
    
    return user, nil
}
```

### Custom Metadata

```go
func MyHandler(ctx *context.Context, req Request) (*Response, error) {
    // Set metadata (thread-safe)
    ctx.Set("tenant_id", "tenant-123")
    ctx.Set("correlation_id", uuid.New().String())
    
    // Get metadata
    tenantID, ok := ctx.Get("tenant_id")
    if ok {
        // Use tenant ID
    }
    
    return &Response{}, nil
}
```

### Identity (After Authentication)

```go
func SecureHandler(ctx *context.Context, req Request) (*Response, error) {
    // Get authenticated identity
    identity := ctx.Identity()
    if identity == nil {
        return nil, errors.New("not authenticated")
    }
    
    // Check roles
    if !identity.HasRole("admin") {
        return nil, errors.New("forbidden")
    }
    
    // Check scopes
    if !identity.HasScope("users:write") {
        return nil, errors.New("insufficient scope")
    }
    
    // Use identity info
    userID := identity.ID
    email := identity.Email
    
    return &Response{}, nil
}
```

## Error Handling

### Returning Errors

```go
func CreateUser(ctx *context.Context, req CreateUserRequest) (*User, error) {
    // Simple error
    if req.Name == "" {
        return nil, errors.New("name is required")
    }
    
    // Wrapped error
    user := &User{Name: req.Name, Email: req.Email}
    if err := ctx.DB().Create(ctx.Context(), user); err != nil {
        return nil, fmt.Errorf("failed to create user: %w", err)
    }
    
    return user, nil
}
```

### HTTP Errors

Use `HTTPError` for specific HTTP status codes:

```go
import httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"

func GetUser(ctx *context.Context) (*User, error) {
    userID := ctx.Request().Param("id")
    
    var user User
    err := ctx.DB().FindByID(ctx.Context(), userID, &user)
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            return nil, &httpAdapter.HTTPError{
                StatusCode: 404,
                Message:    "User not found",
                Internal:   err,
            }
        }
        return nil, &httpAdapter.HTTPError{
            StatusCode: 500,
            Message:    "Internal server error",
            Internal:   err,
        }
    }
    
    return user, nil
}
```

## Multi-Service Mode

### Defining Services

```go
// User service
app.Service("user-service").
    Describe("Handles user management").
    OnStart(func(ctx context.Context) error {
        log.Println("User service starting...")
        return nil
    }).
    OnStop(func(ctx context.Context) error {
        log.Println("User service stopping...")
        return nil
    }).
    Register(CreateUser).HTTP("POST", "/users").Done().
    Register(GetUser).HTTP("GET", "/users/:id").Done()

// Order service with dependency
app.Service("order-service").
    Describe("Handles order processing").
    DependsOn("user-service").
    Register(CreateOrder).HTTP("POST", "/orders").Done()
```

### Running Services

```go
// Run all services
app.Start()

// Run specific services
app.RunServices("user-service", "order-service")

// Run single service
app.RunServices("user-service")
```

### Command Line Selection

```go
func main() {
    servicesFlag := flag.String("services", "", "Comma-separated services")
    flag.Parse()
    
    // ... app setup ...
    
    if *servicesFlag != "" {
        services := strings.Split(*servicesFlag, ",")
        app.RunServices(services...)
    } else {
        app.Start()
    }
}
```

```bash
# Run all
./myapp

# Run specific services
./myapp -services=user-service,order-service
```

## Best Practices

### 1. Keep Handlers Focused

```go
// Good: Single responsibility
func CreateUser(ctx *unicorn.Context, req CreateUserRequest) (*User, error) {
    return userService.Create(ctx, req)
}

// Bad: Too much logic in handler
func CreateUser(ctx *unicorn.Context, req CreateUserRequest) (*User, error) {
    // Validation, database, caching, events, logging all mixed
}
```

### 2. Use Meaningful Names

```go
app.RegisterHandler(CreateUser).
    Named("create-user").  // Descriptive name
    HTTP("POST", "/users").
    Done()
```

### 3. Handle All Error Cases

```go
func GetUser(ctx *unicorn.Context) (*User, error) {
    userID := ctx.Request().Params["id"]
    if userID == "" {
        return nil, &http.HTTPError{StatusCode: 400, Message: "Missing user ID"}
    }
    
    user, err := db.Find(userID)
    if errors.Is(err, ErrNotFound) {
        return nil, &http.HTTPError{StatusCode: 404, Message: "User not found"}
    }
    if err != nil {
        ctx.Logger().Error("database error", "error", err)
        return nil, &http.HTTPError{StatusCode: 500, Message: "Internal error"}
    }
    
    return user, nil
}
```

### 4. Log Important Events

```go
func CreateOrder(ctx *unicorn.Context, req CreateOrderRequest) (*Order, error) {
    log := ctx.Logger()
    
    log.Info("creating order", 
        "user_id", req.UserID,
        "product_id", req.ProductID,
        "amount", req.Amount,
    )
    
    order, err := orderService.Create(req)
    if err != nil {
        log.Error("failed to create order", "error", err)
        return nil, err
    }
    
    log.Info("order created", "order_id", order.ID)
    return order, nil
}
```

## Next Steps

- [Security](./security.md) - Authentication, authorization, rate limiting
- [Observability](./observability.md) - Metrics, tracing, logging
- [Examples](./examples.md) - Complete example applications
