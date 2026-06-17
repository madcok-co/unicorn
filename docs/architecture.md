# Architecture

This document describes the architecture and design principles of the Unicorn framework.

## Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        Unicorn App                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │   HTTP       │  │   Message    │  │    Cron      │           │
│  │   Adapter    │  │   Broker     │  │   Adapter    │           │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘           │
│         │                 │                 │                   │
│         └─────────────────┼─────────────────┘                   │
│                           ▼                                     │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                   Handler Registry                      │    │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐     │    │
│  │  │Handler 1│  │Handler 2│  │Handler 3│  │Handler N│     │    │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘     │    │
│  └─────────────────────────────────────────────────────────┘    │
│                           │                                     │
│                           ▼                                     │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                   Unicorn Context                       │    │
│  │  ┌────────┐ ┌───────┐ ┌────────┐ ┌───────┐ ┌────────┐   │    │
│  │  │   DB   │ │ Cache │ │ Logger │ │ Queue │ │ Broker │   │    │
│  │  └────────┘ └───────┘ └────────┘ └───────┘ └────────┘   │    │
│  │  ┌─────────┐ ┌────────┐ ┌──────────────────────────┐    │    │
│  │  │ Metrics │ │ Tracer │ │       Security           │    │    │
│  │  └─────────┘ └────────┘ └──────────────────────────┘    │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Core Concepts

### 1. Handlers

Handlers are pure business logic functions. They:
- Accept a `*unicorn.Context` as the first parameter
- Optionally accept a request DTO as the second parameter
- Return a response and error

```go
func CreateUser(ctx *unicorn.Context, req CreateUserRequest) (*User, error) {
    // Pure business logic only
    return &User{Name: req.Name}, nil
}
```

### 2. Triggers

Triggers define how handlers are invoked. A single handler can have multiple triggers:

| Trigger | Description | Example |
|---------|-------------|---------|
| HTTP | REST API endpoints | `HTTP("POST", "/users")` |
| Message | Message broker topics | `Message("user.create")` |
| Cron | Scheduled execution | `Cron("0 * * * *")` |
| gRPC | gRPC service methods | `GRPC("UserService", "Create")` |

### 3. Context

The Unicorn Context provides access to all infrastructure:

```go
type Context struct {
    // Standard context
    ctx context.Context
    
    // Request info
    request *Request
    
    // Infrastructure
    db      Database
    cache   Cache
    logger  Logger
    queue   Queue
    broker  Broker
    metrics Metrics
    tracer  Tracer
    
    // Security
    identity *Identity
    
    // Custom metadata
    metadata map[string]any
}
```

### 4. Adapters

Adapters implement infrastructure contracts. This allows swapping implementations without changing business logic.

```
┌────────────────────┐
│     Contract       │  ← Interface definition
├────────────────────┤
│  Database          │
│  Cache             │
│  Logger            │
│  Broker            │
│  Authenticator     │
│  RateLimiter       │
└────────────────────┘
         │
         ▼
┌────────────────────┐
│     Adapters       │  ← Implementations
├────────────────────┤
│  PostgreSQL        │
│  Redis             │
│  Zap Logger        │
│  Kafka/RabbitMQ    │
│  JWT/APIKey        │
│  Memory/Redis      │
└────────────────────┘
```

## Package Structure

```
github.com/madcok-co/unicorn/
├── unicorn.go              # Main entry point, re-exports
├── pkg/
│   ├── app/                # Application lifecycle
│   │   └── app.go
│   ├── context/            # Unicorn context
│   │   └── context.go
│   ├── contracts/          # Interface definitions
│   │   ├── contracts.go
│   │   ├── database.go
│   │   ├── cache.go
│   │   ├── logger.go
│   │   ├── broker.go
│   │   ├── security.go
│   │   ├── metrics.go
│   │   └── tracer.go
│   ├── handler/            # Handler registry & execution
│   │   ├── handler.go
│   │   ├── registry.go
│   │   ├── executor.go
│   │   └── triggers.go
│   ├── service/            # Multi-service support
│   │   ├── service.go
│   │   ├── registry.go
│   │   └── runner.go
│   ├── middleware/         # Production middleware
│   │   ├── recovery.go     # Panic recovery
│   │   ├── cors.go         # CORS handling
│   │   ├── timeout.go      # Request timeout
│   │   ├── auth.go         # JWT, API Key, Basic Auth
│   │   ├── ratelimit.go    # Rate limiting (memory/Redis)
│   │   ├── health.go       # Health checks
│   │   └── telemetry.go    # OpenTelemetry tracing/metrics
│   ├── resilience/         # Resilience patterns
│   │   ├── circuitbreaker.go  # Circuit breaker
│   │   └── retry.go        # Retry, bulkhead, timeout, fallback
│   └── adapters/           # Infrastructure implementations
│       ├── http/           # HTTP server adapter
│       ├── broker/         # Message broker adapters
│       │   ├── adapter.go
│       │   ├── memory/
│       │   └── kafka/
│       └── security/       # Security adapters
│           ├── auth/       # JWT, API Key
│           ├── ratelimiter/
│           ├── encryptor/
│           ├── hasher/
│           ├── secrets/
│           └── audit/
├── examples/
│   ├── basic/
│   └── multiservice/
└── docs/
```

## Request Flow

### HTTP Request Flow

```
HTTP Request
     │
     ▼
┌─────────────────┐
│  HTTP Adapter   │
│  - Parse JSON   │
│  - Extract params│
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Middleware    │
│  - Auth         │
│  - Rate Limit   │
│  - Logging      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│    Handler      │
│  - Business     │
│    Logic        │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  HTTP Response  │
│  - JSON encode  │
└─────────────────┘
```

### Message Broker Flow

```
Message Received
     │
     ▼
┌─────────────────┐
│ Broker Adapter  │
│  - Deserialize  │
│  - Create ctx   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│    Handler      │
│  - Business     │
│    Logic        │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Ack/Nack Msg   │
│  - Retry logic  │
└─────────────────┘
```

## Multi-Service Mode

Services can be organized and run independently:

```go
// Define services
app.Service("user-service").
    Register(CreateUser).HTTP("POST", "/users").Done().
    Register(GetUser).HTTP("GET", "/users/:id").Done()

app.Service("order-service").
    DependsOn("user-service").
    Register(CreateOrder).HTTP("POST", "/orders").Done()

// Run all services together
app.Start()

// Or run specific services
app.RunServices("user-service")
```

### Service Isolation

```
┌─────────────────────────────────────────────────────────────┐
│                        Application                          │
│  ┌───────────────────┐  ┌───────────────────┐               │
│  │   user-service    │  │   order-service   │               │
│  │  ┌─────────────┐  │  │  ┌─────────────┐  │               │
│  │  │ CreateUser  │  │  │  │ CreateOrder │  │               │
│  │  │ GetUser     │  │  │  │ GetOrder    │  │               │
│  │  │ ListUsers   │  │  │  └─────────────┘  │               │
│  │  └─────────────┘  │  │   DependsOn:      │               │
│  └───────────────────┘  │   user-service    │               │
│                         └───────────────────┘               │
└─────────────────────────────────────────────────────────────┘
```

## Hybrid Deployment: Triggers as Sidecars

Triggers can run as sidecores alongside the main app for production isolation:

```
┌── 1 Process, No K8s ─────────────────────────────┐
│                                                  │
│  [Sidecar] HTTP Trigger   :8080 (isolated)       │
│  [Sidecar] Broker Trigger        (isolated)      │
│  [Sidecar] Cron Trigger          (isolated)      │
│  [Sidecar] Management     :9090 (health/metrics) │
│  [Sidecar] Custom ESL      :8021 (freeswitch)    │
│                                                  │
│  App core: handler registry (shared)             │
└──────────────────────────────────────────────────┘
```

**Why?** HTTP crash doesn't kill broker. Health check survives load spike.
Add/remove triggers at runtime. Custom protocol consumers as sidecars.

```go
// Sidecar mode
app := unicorn.New(&unicorn.Config{Name: "telephony"})
app.RegisterHandler(OnCallStarted).Message("freeswitch.events").Done()

httpSidecar := unicorn.NewHTTPSidecar(app.Registry(), &http.Config{Port: 8080})
httpSidecar.Adapter.SetAppAdapters(app.Adapters())
app.AddSidecar(httpSidecar)

app.RunSidecars() // no built-in adapters — everything is a sidecar
```

## Design Principles

### 1. Separation of Concerns

- **Handlers**: Pure business logic
- **Adapters**: Infrastructure implementation
- **Context**: Dependency injection container
- **Triggers**: How handlers are invoked

### 2. Contract-First Design

All infrastructure is defined by interfaces (contracts). This enables:
- Easy testing with mocks
- Swappable implementations
- Clear API boundaries

### 3. Convention over Configuration

Sensible defaults reduce boilerplate:
- Default HTTP port: 8080
- Default JSON content type
- Auto request/response serialization

### 4. Thread Safety

All components are designed for concurrent use:
- Context metadata uses `sync.RWMutex`
- Rate limiter uses per-key mutexes
- Audit logger uses buffered channels

### 5. Graceful Shutdown

- Context cancellation propagates to all components
- `sync.WaitGroup` ensures goroutines complete
- Lifecycle hooks for cleanup

## Resilience Patterns

Unicorn includes production-ready resilience patterns:

```
┌─────────────────────────────────────────────────────────────┐
│                   Resilience Layer                          │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────┐  │
│  │ Circuit Breaker │  │  Retry/Backoff  │  │  Bulkhead   │  │
│  │  Closed→Open→   │  │  Exponential    │  │  Concurrency│  │
│  │  HalfOpen       │  │  with Jitter    │  │  Limiting   │  │
│  └─────────────────┘  └─────────────────┘  └─────────────┘  │
│  ┌─────────────────┐  ┌─────────────────┐                   │
│  │    Timeout      │  │    Fallback     │                   │
│  │  Context-based  │  │  Default values │                   │
│  └─────────────────┘  └─────────────────┘                   │
└─────────────────────────────────────────────────────────────┘
```

## Next Steps

- [Handlers & Triggers](./handlers.md) - Deep dive into handler patterns
- [Middleware](./middleware.md) - Production-ready middleware
- [Resilience Patterns](./resilience.md) - Circuit breaker, retry, bulkhead
- [Security](./security.md) - Security architecture
- [API Reference](./api-reference.md) - Complete API documentation
