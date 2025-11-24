# Unicorn Framework

A batteries-included Go framework where developers only need to focus on business logic.

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Features

- **Focus on Business Logic** - Write handlers that only contain business logic
- **Multi-Trigger Support** - Same handler works for HTTP, Kafka, RabbitMQ, gRPC, Cron
- **Generic Adapter Pattern** - Swap infrastructure (DB, Cache, Logger, etc.) without code changes
- **Multiple Named Adapters** - Support multiple databases, caches, brokers per app
- **Custom Service Injection** - Inject your own interfaces with type-safe generics
- **Built-in Security** - JWT, API Key auth, rate limiting, encryption, audit logging
- **Production-Ready Middleware** - Recovery, CORS, Timeout, Rate Limiting, Auth
- **Resilience Patterns** - Circuit Breaker, Retry with Backoff, Bulkhead
- **Observability** - Metrics, tracing, structured logging, health checks
- **Multi-Service Mode** - Run multiple services independently or together
- **High Performance** - Zero-allocation context pooling (~38ns/op)

## Project Structure

```
github.com/madcok-co/unicorn/
├── core/                       # Core framework
│   ├── pkg/
│   │   ├── app/                # Application lifecycle
│   │   ├── context/            # Request context (optimized)
│   │   ├── contracts/          # Interface definitions
│   │   ├── handler/            # Handler registry
│   │   ├── middleware/         # Production middleware
│   │   ├── resilience/         # Resilience patterns
│   │   └── adapters/           # Built-in adapters
│   ├── cmd/unicorn/            # CLI tool
│   └── examples/               # Example applications
│
├── contrib/                    # Official driver implementations
│   ├── database/gorm/          # GORM database driver
│   ├── cache/redis/            # Redis cache driver
│   ├── logger/zap/             # Zap logger driver
│   ├── broker/kafka/           # Kafka message broker driver
│   └── validator/playground/   # go-playground/validator driver
│
└── docs/                       # Documentation
```

## Quick Start

### Installation

```bash
# Core framework
go get github.com/madcok-co/unicorn/core

# Install drivers you need
go get github.com/madcok-co/unicorn/contrib/database/gorm
go get github.com/madcok-co/unicorn/contrib/cache/redis
go get github.com/madcok-co/unicorn/contrib/logger/zap
```

### Basic Example

```go
package main

import (
    "github.com/madcok-co/unicorn/core"
    "github.com/madcok-co/unicorn/contrib/database/gorm"
    "github.com/madcok-co/unicorn/contrib/cache/redis"
    "github.com/madcok-co/unicorn/contrib/logger/zap"
    
    "gorm.io/driver/postgres"
    gormpkg "gorm.io/gorm"
    goredis "github.com/redis/go-redis/v9"
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

func main() {
    app := unicorn.New(&unicorn.Config{
        Name:       "my-app",
        EnableHTTP: true,
    })

    // Setup infrastructure with contrib drivers
    db, _ := gormpkg.Open(postgres.Open(dsn), &gormpkg.Config{})
    app.SetDB(gorm.NewDriver(db))
    
    rdb := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
    app.SetCache(redis.NewDriver(rdb))
    
    app.SetLogger(zap.NewDriver())

    // Register handler with multiple triggers
    app.RegisterHandler(CreateUser).
        HTTP("POST", "/users").
        Message("user.create").  // Also trigger from message broker
        Done()

    app.Start()
}

// Handler - pure business logic!
func CreateUser(ctx *unicorn.Context) error {
    var req CreateUserRequest
    ctx.Bind(&req)
    
    // Validate
    if err := ctx.Validate(req); err != nil {
        return ctx.Error(400, err.Error())
    }
    
    // Use infrastructure through context
    user := &User{ID: "user-123", Name: req.Name, Email: req.Email}
    ctx.DB().Create(ctx.Context(), user)
    ctx.Cache().Set(ctx.Context(), "user:"+user.ID, user, time.Hour)
    ctx.Logger().Info("user created", "id", user.ID)
    
    return ctx.JSON(201, user)
}
```

## Core Concepts

### Multi-Trigger Handlers

Same handler responds to multiple triggers:

```go
app.RegisterHandler(ProcessOrder).
    HTTP("POST", "/orders").           // REST API
    Message("order.create.command").   // Message broker
    Cron("*/5 * * * *").               // Every 5 minutes
    Done()
```

### Generic Adapter Pattern

Swap infrastructure without changing business logic:

```go
// Development - use in-memory
app.SetCache(memory.NewDriver())

// Production - use Redis
app.SetCache(redis.NewDriver(redisClient))

// Handler code stays the same!
func handler(ctx *unicorn.Context) error {
    ctx.Cache().Set(ctx.Context(), "key", "value", time.Hour)
    return nil
}
```

### Multiple Named Adapters

Support multiple instances for scaling:

```go
// Multiple databases
app.SetDB(gorm.NewDriver(primaryDB))                    // Default
app.SetDB(gorm.NewDriver(analyticsDB), "analytics")     // Named
app.SetDB(gorm.NewDriver(replicaDB), "replica")         // Named

// In handler
func handler(ctx *unicorn.Context) error {
    ctx.DB().Create(ctx.Context(), &user)                    // Primary
    ctx.DB("analytics").Create(ctx.Context(), &event)        // Analytics
    ctx.DB("replica").FindAll(ctx.Context(), &users, "")     // Replica
    return nil
}
```

### Multi-Service Mode

Organize handlers into services:

```go
// User Service
app.Service("user-service").
    Register(CreateUser).HTTP("POST", "/users").Done().
    Register(GetUser).HTTP("GET", "/users/:id").Done()

// Order Service
app.Service("order-service").
    DependsOn("user-service").
    Register(CreateOrder).HTTP("POST", "/orders").Done()

// Run all or specific services
app.Start()                              // All services
app.RunServices("user-service")          // Specific service
```

## Available Drivers

| Category | Driver | Package |
|----------|--------|---------|
| Database | GORM | `contrib/database/gorm` |
| Cache | Redis | `contrib/cache/redis` |
| Logger | Zap | `contrib/logger/zap` |
| Broker | Kafka | `contrib/broker/kafka` |
| Validator | Playground | `contrib/validator/playground` |

See [contrib/README.md](./contrib/README.md) for full driver documentation.

## Production Middleware

Built-in middleware for production use:

```go
import "github.com/madcok-co/unicorn/core/pkg/middleware"

// Apply middleware stack
app.Use(
    middleware.Recovery(),                           // Panic recovery
    middleware.CORS(),                               // CORS handling
    middleware.Timeout(30 * time.Second),            // Request timeout
    middleware.RateLimit(100, time.Minute),          // Rate limiting
)

// Authentication middleware
app.Use(middleware.JWT([]byte("secret")))            // JWT auth
app.Use(middleware.APIKey(validateAPIKey))           // API key auth
app.Use(middleware.BasicAuth(validateCredentials))   // Basic auth

// Health checks (Kubernetes-ready)
health := middleware.NewHealthHandler(nil)
health.AddChecker("database", middleware.DatabaseChecker(db))
health.AddChecker("cache", middleware.CacheChecker(cache))

app.GET("/health", health.Handler())
app.GET("/health/live", health.LivenessHandler())
app.GET("/health/ready", health.ReadinessHandler())
```

## Resilience Patterns

Built-in resilience patterns for fault tolerance:

```go
import "github.com/madcok-co/unicorn/core/pkg/resilience"

// Circuit Breaker - prevents cascading failures
cb := resilience.NewCircuitBreaker(&resilience.CircuitBreakerConfig{
    MaxRequests: 3,
    Timeout:     30 * time.Second,
    ReadyToTrip: func(counts resilience.Counts) bool {
        return counts.ConsecutiveFailures > 5
    },
})

err := cb.Execute(func() error {
    return callExternalService()
})

// Retry with exponential backoff
retryer := resilience.NewRetryer(&resilience.RetryConfig{
    MaxAttempts:     3,
    InitialInterval: 100 * time.Millisecond,
    MaxInterval:     10 * time.Second,
    Multiplier:      2.0,
})

err := retryer.Do(func() error {
    return unreliableOperation()
})

// Bulkhead - limits concurrent executions
bulkhead := resilience.NewBulkhead(10, 5*time.Second)
err := bulkhead.Execute(func() error {
    return processRequest()
})

// Combine patterns
err := cb.ExecuteWithRetry(retryer, func() error {
    return callExternalService()
})
```

## Performance

Unicorn is optimized for high performance:

- **Zero-allocation** context pooling
- **Lazy adapter injection** - no copying per request
- **~38ns** per context acquire/release

```
BenchmarkContextAcquire-8    30683319    38.26 ns/op    0 B/op    0 allocs/op
```

See [docs/benchmarks.md](./docs/benchmarks.md) for detailed benchmarks.

## Documentation

- [Getting Started](./docs/getting-started.md) - Installation and first app
- [Architecture](./docs/architecture.md) - Framework design
- [Handlers & Triggers](./docs/handlers.md) - Handler patterns
- [Custom Services](./docs/custom-services.md) - Dependency injection
- [Security](./docs/security.md) - Authentication, authorization, encryption
- [Observability](./docs/observability.md) - Metrics, tracing, logging
- [Benchmarks](./docs/benchmarks.md) - Performance benchmarks
- [Framework Comparison](./docs/comparison.md) - vs Gin, Echo, Fiber, etc.
- [API Reference](./docs/api-reference.md) - Complete API documentation
- [Best Practices](./docs/best-practices.md) - Production recommendations
- [Examples](./docs/examples.md) - Complete example applications

## Examples

```bash
# Basic example
cd core/examples/basic
go run main.go

# Multi-service example
cd core/examples/multiservice
go run main.go
```

## License

MIT License

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting PRs.

---

## Creator's Note

This framework was built based on real-world production experience, combining battle-tested patterns from various ecosystems (Spring Boot, NestJS, Laravel) adapted for Go's philosophy.

**Before you criticize:**

1. **"Why not just use Gin/Echo/Fiber?"** - Those are routers, not frameworks. Unicorn is a full application framework with built-in support for multi-trigger handlers, infrastructure abstraction, resilience patterns, and production middleware. Different tools for different problems.

2. **"This is over-engineered!"** - If you're building a simple CRUD API, yes, use something simpler. Unicorn is designed for complex, multi-service production systems where you need consistent patterns across teams.

3. **"Go should be simple!"** - The handlers ARE simple. The complexity is in the framework so YOUR code stays clean. That's the whole point.

4. **"I can build this myself!"** - Great, do it. But when you've spent 6 months reinventing circuit breakers, health checks, graceful shutdown, and adapter patterns, remember this exists.

5. **"Where are the benchmarks against X?"** - See [docs/benchmarks.md](./docs/benchmarks.md). Spoiler: it's fast enough. If nanoseconds matter more than developer productivity, you're optimizing the wrong thing.

6. **"This doesn't follow DDD/Clean Architecture/Hexagonal!"** - Cool. Those are guidelines, not gospel. If your 500-line CRUD service needs 47 layers of abstraction, bounded contexts, aggregate roots, and a domain expert consultation, you have different problems. Unicorn gives you clean separation where it matters (infrastructure vs business logic) without forcing you into architecture astronaut territory. Use DDD when you actually need it, not because someone on Medium said so.

7. **"You should use Repository Pattern!"** - The adapter pattern IS a repository pattern, just not named that way to satisfy your design pattern bingo card. `ctx.DB()` abstracts your data layer. Done. No need for `UserRepositoryInterface`, `UserRepositoryImpl`, `UserRepositoryFactory`, and 15 other files for a single table.

8. **"Where's the Service Layer?"** - Your handler IS your service. If you need more abstraction, create your own services and inject them via `app.Set()`. We're not forcing 3-tier architecture on a 200-line microservice.

9. **"This violates SOLID principles!"** - Which one? Single Responsibility? Handlers do one thing. Open/Closed? Use middleware. Liskov? Adapters are swappable. Interface Segregation? Check `contracts/`. Dependency Inversion? The whole framework is built on it. Next.

10. **"Real engineers use stdlib only!"** - Real engineers ship products. If you want to write your own HTTP router, connection pooling, circuit breaker, and graceful shutdown from scratch every project, go ahead. Some of us have deadlines.

11. **"Microservices should be micro!"** - This framework supports both monolith and microservices. The multi-service mode lets you split when you NEED to, not because some conference talk said so. Premature distribution is the root of all evil.

12. **"You're not using Context correctly!"** - We extend `context.Context` for developer ergonomics while maintaining full compatibility. If passing 47 parameters to every function or using context.Value for everything is your preference, enjoy your type assertions.

13. **"Global state is bad!"** - There's no global state. The app instance holds everything. You can create multiple apps if you want. The `ctx.DB()` pattern is dependency injection, not global access.

14. **"What about testing?"** - Mock the adapters. That's literally why the adapter pattern exists. `app.SetDB(mockDB)` and you're done. No complex DI container or test framework required.

15. **"This isn't idiomatic Go!"** - "Idiomatic Go" is not a religion. If your idiomatic code requires 3x more boilerplate for the same result, maybe the idiom needs updating. Go itself has evolved (generics, anyone?).

**The philosophy is simple:** Write business logic, not infrastructure code. Ship products, not architecture diagrams. If you disagree, that's fine - mass unfollow, mass block. Use what works for you.

**To the haters:** The mass unfollow and mass block is real, fuck off and mass block my ass. I don't mass care.

---

*Built with frustation from years of writing the same boilerplate across different projects.*
