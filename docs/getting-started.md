# Getting Started

This guide will help you get started with the Unicorn framework.

## Prerequisites

- Go 1.21 or later
- Basic understanding of Go

## Installation

```bash
# Core framework
go get github.com/madcok-co/unicorn/core

# Install drivers you need
go get github.com/madcok-co/unicorn/contrib/database/gorm
go get github.com/madcok-co/unicorn/contrib/cache/redis
go get github.com/madcok-co/unicorn/contrib/logger/zap
go get github.com/madcok-co/unicorn/contrib/validator/playground
```

## Your First Application

### 1. Create a New Project

```bash
mkdir my-unicorn-app
cd my-unicorn-app
go mod init my-unicorn-app
go get github.com/madcok-co/unicorn/core
```

### 2. Create main.go

```go
package main

import (
    "log"
    "github.com/madcok-co/unicorn/core"
)

// Request DTO
type GreetRequest struct {
    Name string `json:"name"`
}

// Response DTO
type GreetResponse struct {
    Message string `json:"message"`
}

// Handler - pure business logic
func Greet(ctx *unicorn.Context) error {
    var req GreetRequest
    ctx.Bind(&req)
    
    return ctx.JSON(200, &GreetResponse{
        Message: "Hello, " + req.Name + "!",
    })
}

func main() {
    app := unicorn.New(&unicorn.Config{
        Name:       "greeter",
        Version:    "1.0.0",
        EnableHTTP: true,
        HTTP: &unicorn.HTTPConfig{
            Host: "0.0.0.0",
            Port: 8080,
        },
    })

    app.RegisterHandler(Greet).
        Named("greet").
        HTTP("POST", "/greet").
        Done()

    log.Println("Server starting on :8080")
    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
```

### 3. Run the Application

```bash
go run main.go
```

### 4. Test the Endpoint

```bash
curl -X POST http://localhost:8080/greet \
  -H "Content-Type: application/json" \
  -d '{"name": "World"}'
```

Response:
```json
{"message": "Hello, World!"}
```

## Adding Infrastructure (Drivers)

Unicorn uses a generic adapter pattern. Install only the drivers you need:

### Database (GORM)

```go
import (
    "github.com/madcok-co/unicorn/core"
    "github.com/madcok-co/unicorn/contrib/database/gorm"
    "gorm.io/driver/postgres"
    gormpkg "gorm.io/gorm"
)

func main() {
    app := unicorn.New(&unicorn.Config{Name: "my-app"})
    
    // Setup database
    db, _ := gormpkg.Open(postgres.Open(dsn), &gormpkg.Config{})
    app.SetDB(gorm.NewDriver(db))
    
    // ...
}
```

### Cache (Redis)

```go
import (
    "github.com/madcok-co/unicorn/contrib/cache/redis"
    goredis "github.com/redis/go-redis/v9"
)

func main() {
    app := unicorn.New(&unicorn.Config{Name: "my-app"})
    
    // Setup cache
    rdb := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
    app.SetCache(redis.NewDriver(rdb))
    
    // ...
}
```

### Logger (Zap)

```go
import (
    "github.com/madcok-co/unicorn/contrib/logger/zap"
)

func main() {
    app := unicorn.New(&unicorn.Config{Name: "my-app"})
    
    // Setup logger
    app.SetLogger(zap.NewDriver())
    
    // Or with custom config
    app.SetLogger(zap.NewDriverWithConfig(&zap.Config{
        Level:  "debug",
        Format: "json",
    }))
    
    // ...
}
```

### Validator (Playground)

```go
import (
    "github.com/madcok-co/unicorn/contrib/validator/playground"
)

func main() {
    app := unicorn.New(&unicorn.Config{Name: "my-app"})
    
    // Setup validator
    app.SetValidator(playground.NewDriver())
    
    // ...
}
```

## Full Example with All Drivers

```go
package main

import (
    "log"
    "time"
    
    "github.com/madcok-co/unicorn/core"
    "github.com/madcok-co/unicorn/contrib/database/gorm"
    "github.com/madcok-co/unicorn/contrib/cache/redis"
    "github.com/madcok-co/unicorn/contrib/logger/zap"
    "github.com/madcok-co/unicorn/contrib/validator/playground"
    
    "gorm.io/driver/postgres"
    gormpkg "gorm.io/gorm"
    goredis "github.com/redis/go-redis/v9"
)

type CreateUserRequest struct {
    Name  string `json:"name" validate:"required,min=3"`
    Email string `json:"email" validate:"required,email"`
}

type User struct {
    ID    uint   `json:"id" gorm:"primaryKey"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func CreateUser(ctx *unicorn.Context) error {
    var req CreateUserRequest
    ctx.Bind(&req)
    
    // Validate request
    if err := ctx.Validate(req); err != nil {
        return ctx.Error(400, err.Error())
    }
    
    // Create user in database
    user := &User{Name: req.Name, Email: req.Email}
    if err := ctx.DB().Create(ctx.Context(), user); err != nil {
        ctx.Logger().Error("failed to create user", "error", err)
        return ctx.Error(500, "Failed to create user")
    }
    
    // Cache the user
    ctx.Cache().Set(ctx.Context(), "user:"+string(user.ID), user, time.Hour)
    
    // Log success
    ctx.Logger().Info("user created", "id", user.ID, "email", user.Email)
    
    return ctx.JSON(201, user)
}

func main() {
    app := unicorn.New(&unicorn.Config{
        Name:       "user-service",
        EnableHTTP: true,
    })

    // Setup database
    db, err := gormpkg.Open(postgres.Open("host=localhost user=postgres dbname=myapp"), &gormpkg.Config{})
    if err != nil {
        log.Fatal(err)
    }
    db.AutoMigrate(&User{})
    app.SetDB(gorm.NewDriver(db))
    
    // Setup cache
    rdb := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
    app.SetCache(redis.NewDriver(rdb))
    
    // Setup logger
    app.SetLogger(zap.NewDriver())
    
    // Setup validator
    app.SetValidator(playground.NewDriver())

    // Register handlers
    app.RegisterHandler(CreateUser).
        HTTP("POST", "/users").
        Done()

    log.Println("Server starting on :8080")
    app.Start()
}
```

## Handler Patterns

### Accessing Context Data

```go
func MyHandler(ctx *unicorn.Context) error {
    // Path parameters
    userID := ctx.Request().Params["id"]
    
    // Query parameters
    page := ctx.Request().Query["page"]
    
    // Headers
    auth := ctx.Request().Headers["Authorization"]
    
    // Request body
    body := ctx.Request().Body
    
    return ctx.JSON(200, data)
}
```

### Using Infrastructure

```go
func MyHandler(ctx *unicorn.Context) error {
    // Database
    ctx.DB().Create(ctx.Context(), &entity)
    ctx.DB().FindByID(ctx.Context(), id, &result)
    
    // Cache
    ctx.Cache().Set(ctx.Context(), "key", value, time.Hour)
    ctx.Cache().Get(ctx.Context(), "key", &result)
    
    // Logger
    ctx.Logger().Info("message", "key", "value")
    ctx.Logger().WithError(err).Error("failed")
    
    // Validator
    ctx.Validate(request)
    
    // Message Broker
    ctx.Broker().Publish(ctx.Context(), "topic", msg)
    
    return ctx.JSON(200, result)
}
```

## Multiple Triggers

Same handler responds to multiple triggers:

```go
app.RegisterHandler(ProcessOrder).
    Named("process-order").
    HTTP("POST", "/orders").           // HTTP trigger
    Message("order.create.command").   // Message broker trigger
    Cron("0 * * * *").                 // Cron trigger (hourly)
    Done()
```

## Lifecycle Hooks

```go
app.OnStart(func() error {
    log.Println("Application starting...")
    return nil
})

app.OnStop(func() error {
    log.Println("Application shutting down...")
    return nil
})
```

## Adding Production Middleware

Unicorn provides production-ready middleware out of the box:

```go
import (
    "github.com/madcok-co/unicorn/core/pkg/middleware"
)

func main() {
    app := unicorn.New(&unicorn.Config{Name: "my-app"})
    
    // Recovery - catch panics
    app.Use(middleware.Recovery())
    
    // CORS - handle cross-origin requests
    app.Use(middleware.CORS(middleware.CORSConfig{
        AllowOrigins: []string{"https://example.com"},
    }))
    
    // Timeout - prevent slow requests
    app.Use(middleware.Timeout(30 * time.Second))
    
    // Rate limiting
    app.Use(middleware.RateLimit(middleware.RateLimitConfig{
        Max:      100,
        Duration: time.Minute,
    }))
    
    // ...
}
```

## Next Steps

- [Architecture Overview](./architecture.md) - Understand the framework design
- [Handlers & Triggers](./handlers.md) - Deep dive into handler patterns
- [Middleware](./middleware.md) - Production-ready middleware
- [Resilience Patterns](./resilience.md) - Circuit breaker, retry, bulkhead
- [Custom Services](./custom-services.md) - Dependency injection
- [Security](./security.md) - Add authentication and authorization
- [Contrib Drivers](../contrib/README.md) - All available drivers
- [Examples](./examples.md) - More complete examples
