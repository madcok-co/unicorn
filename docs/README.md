# Unicorn Framework Documentation

Unicorn is a batteries-included Go framework where developers only need to focus on business logic. All infrastructure (database, cache, queue, logger, security) is accessed through a unified Context, and handlers can be triggered from multiple sources (HTTP, Message Broker, gRPC, Cron) with the same code.

## Table of Contents

### Getting Started
1. [Getting Started](./getting-started.md) - Installation and first app

### Core Concepts
2. [Architecture](./architecture.md) - Framework design and concepts
3. [Handlers & Triggers](./handlers.md) - Handler patterns and multi-trigger
4. [Custom Services](./custom-services.md) - Dependency injection

### Infrastructure
5. [Contrib Drivers](../contrib/README.md) - Official driver implementations
6. [Security](./security.md) - Authentication, authorization, encryption
7. [Observability](./observability.md) - Metrics, tracing, logging

### Production Features
8. [Middleware](./middleware.md) - Production-ready middleware
9. [Resilience Patterns](./resilience.md) - Circuit breaker, retry, bulkhead

### Reference
10. [API Reference](./api-reference.md) - Complete API documentation
11. [Benchmarks](./benchmarks.md) - Performance benchmarks
12. [Framework Comparison](./comparison.md) - vs Gin, Echo, Fiber, etc.
13. [Best Practices](./best-practices.md) - Production recommendations
14. [Examples](./examples.md) - Complete example applications

## Quick Start

```go
package main

import (
    "github.com/madcok-co/unicorn/core"
    "github.com/madcok-co/unicorn/contrib/database/gorm"
    "github.com/madcok-co/unicorn/contrib/cache/redis"
    "github.com/madcok-co/unicorn/contrib/logger/zap"
)

type CreateUserRequest struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func CreateUser(ctx *unicorn.Context) error {
    var req CreateUserRequest
    ctx.Bind(&req)
    
    // Validate
    if err := ctx.Validate(req); err != nil {
        return ctx.Error(400, err.Error())
    }
    
    // Pure business logic - infrastructure accessed via context
    user := &User{ID: "user-123", Name: req.Name, Email: req.Email}
    ctx.DB().Create(ctx.Context(), user)
    ctx.Cache().Set(ctx.Context(), "user:"+user.ID, user, time.Hour)
    ctx.Logger().Info("user created", "id", user.ID)
    
    return ctx.JSON(201, user)
}

func main() {
    app := unicorn.New(&unicorn.Config{
        Name:       "my-app",
        EnableHTTP: true,
    })

    // Setup infrastructure with contrib drivers
    app.SetDB(gorm.NewDriver(db))
    app.SetCache(redis.NewDriver(rdb))
    app.SetLogger(zap.NewDriver())

    // One handler, multiple triggers!
    app.RegisterHandler(CreateUser).
        HTTP("POST", "/users").
        Message("user.create").
        Done()

    app.Start()
}
```

## Key Features

- **Focus on Business Logic**: Write handlers that only contain business logic
- **Multi-Trigger Support**: Same handler works for HTTP, Kafka, RabbitMQ, gRPC, Cron
- **Generic Adapter Pattern**: Swap infrastructure (DB, Cache, Logger) without code changes
- **Multiple Named Adapters**: Support multiple databases, caches, brokers per app
- **Custom Service Injection**: Inject your own interfaces with type-safe generics
- **Built-in Security**: JWT, API Key auth, rate limiting, encryption, audit logging
- **Production Middleware**: Recovery, CORS, Timeout, Rate Limiting, Auth
- **Resilience Patterns**: Circuit Breaker, Retry with Backoff, Bulkhead
- **Observability**: Metrics, tracing, structured logging, health checks
- **Multi-Service Mode**: Run multiple services independently or together
- **High Performance**: Zero-allocation context pooling (~38ns per request)

## Project Structure

```
github.com/madcok-co/unicorn/
├── core/                       # Core framework
│   ├── pkg/
│   │   ├── app/                # Application lifecycle
│   │   ├── context/            # Request context (optimized)
│   │   ├── contracts/          # Interface definitions
│   │   ├── middleware/         # Production middleware
│   │   ├── resilience/         # Resilience patterns
│   │   └── adapters/           # Built-in adapters
│   └── examples/               # Example applications
│
├── contrib/                    # Official driver implementations
│   ├── database/gorm/          # GORM driver
│   ├── cache/redis/            # Redis driver
│   ├── logger/zap/             # Zap driver
│   ├── broker/kafka/           # Kafka driver
│   └── validator/playground/   # Validator driver
│
└── docs/                       # Documentation
```

## Installation

```bash
# Core framework
go get github.com/madcok-co/unicorn/core

# Install drivers you need
go get github.com/madcok-co/unicorn/contrib/database/gorm
go get github.com/madcok-co/unicorn/contrib/cache/redis
go get github.com/madcok-co/unicorn/contrib/logger/zap
go get github.com/madcok-co/unicorn/contrib/broker/kafka
go get github.com/madcok-co/unicorn/contrib/validator/playground
```

## Available Drivers

| Category | Driver | Package |
|----------|--------|---------|
| Database | GORM | `contrib/database/gorm` |
| Cache | Redis | `contrib/cache/redis` |
| Logger | Zap | `contrib/logger/zap` |
| Broker | Kafka | `contrib/broker/kafka` |
| Validator | Playground | `contrib/validator/playground` |

See [Contrib Drivers](../contrib/README.md) for full documentation.

## Version

Current version: **0.1.0**

## License

MIT License
