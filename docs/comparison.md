# Framework Comparison

> **Creator's Disclaimer**: I'm too lazy to debate people who talk too much theory. This framework was built to solve real problems, not to win arguments on Twitter. If it fits, use it. If not, that's fine, life goes on.

## Overview

Unicorn is a Go framework that combines the simplicity of web frameworks with the power of enterprise patterns. Here's a comparison with other popular Go frameworks.

## Quick Comparison Table

| Feature | Unicorn | Gin | Echo | Fiber | Chi | Beego | Iris | Buffalo | Go-Kit | Kratos |
|---------|---------|-----|------|-------|-----|-------|------|---------|--------|--------|
| HTTP Server | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| gRPC Server | Yes | No | No | No | No | No | No | No | Yes | Yes |
| Message Queue | Yes | No | No | No | No | No | No | No | Manual | Yes |
| Cron Jobs | Yes | No | No | No | No | Yes | No | Yes | Manual | Yes |
| Multi-Trigger Handler | Yes | No | No | No | No | No | No | No | No | No |
| Generic Adapters | Yes | No | No | No | No | Partial | No | Partial | Partial | Partial |
| Swappable Infra | Yes | Manual | Manual | Manual | Manual | Manual | Manual | Manual | Yes | Yes |
| Circuit Breaker | Yes | No | No | No | No | No | No | No | Yes | Yes |
| Retry/Backoff | Yes | No | No | No | No | No | No | No | Manual | Partial |
| Rate Limiting | Yes | No | Partial | Partial | No | No | Partial | No | Manual | Yes |
| Health Checks | Yes | No | No | No | No | No | No | No | Manual | Yes |
| Auth Middleware | Yes | No | Partial | Partial | No | Yes | Partial | Partial | Manual | Partial |
| CORS Middleware | Yes | Manual | Yes | Yes | Manual | Yes | Yes | Yes | Manual | Manual |
| ORM Built-in | No | No | No | No | No | Yes | No | Yes | No | No |
| MVC Pattern | No | No | No | No | No | Yes | Yes | Yes | No | No |
| Code Generation | No | No | No | No | No | Yes | No | Yes | No | Yes |
| Learning Curve | Medium | Low | Low | Low | Low | Medium | Medium | Medium | High | High |
| Performance | High | High | High | Very High | High | Medium | High | Medium | Medium | Medium |
| Boilerplate | Low | Low | Low | Low | Very Low | Medium | Low | Medium | High | Medium |
| GitHub Stars | New | ~80k | ~30k | ~35k | ~18k | ~32k | ~25k | ~8k | ~27k | ~24k |

## Detailed Comparison

### vs Gin/Echo/Fiber (Web Frameworks)

**Gin, Echo, Fiber** are excellent web frameworks for HTTP APIs.

```go
// Gin - Simple HTTP handler
r := gin.Default()
r.POST("/users", func(c *gin.Context) {
    var user User
    c.BindJSON(&user)
    // Save to DB - you manage the connection yourself
    db.Create(&user)
    c.JSON(200, user)
})
r.Run(":8080")
```

```go
// Unicorn - Same handler, multiple triggers
app.Handle(&unicorn.Handler{
    Name: "CreateUser",
    Triggers: []unicorn.Trigger{
        {Type: unicorn.TriggerHTTP, Method: "POST", Path: "/users"},
        {Type: unicorn.TriggerMessage, Queue: "user.create"},
        {Type: unicorn.TriggerGRPC, Method: "CreateUser"},
    },
    Handler: func(ctx *unicorn.Context) error {
        var user User
        ctx.Bind(&user)
        ctx.DB().Create(&user) // DB injected automatically
        return ctx.JSON(200, user)
    },
})
```

**When to use Gin/Echo/Fiber:**
- Need pure HTTP API with maximum performance
- Small to medium projects
- Team is already familiar with the framework
- Don't need message queue or gRPC

**When to use Unicorn:**
- Need HTTP + gRPC + Message Queue in one service
- Want one handler for multiple triggers
- Need swappable infrastructure (switch Redis to Memcached without code changes)
- Building scalable microservices

### vs Go-Kit (Microservice Toolkit)

**Go-Kit** is a toolkit for building microservices with strict separation of concerns.

```go
// Go-Kit - Lots of boilerplate
// 1. Define service interface
type UserService interface {
    CreateUser(ctx context.Context, user User) (User, error)
}

// 2. Implement service
type userService struct {
    db *gorm.DB
}

func (s *userService) CreateUser(ctx context.Context, user User) (User, error) {
    err := s.db.Create(&user).Error
    return user, err
}

// 3. Create endpoint
func makeCreateUserEndpoint(svc UserService) endpoint.Endpoint {
    return func(ctx context.Context, request interface{}) (interface{}, error) {
        req := request.(CreateUserRequest)
        user, err := svc.CreateUser(ctx, req.User)
        return CreateUserResponse{User: user, Err: err}, nil
    }
}

// 4. Create transport (HTTP)
func decodeCreateUserRequest(_ context.Context, r *http.Request) (interface{}, error) {
    var req CreateUserRequest
    json.NewDecoder(r.Body).Decode(&req)
    return req, nil
}

// 5. Wire everything together
// ... more code ...
```

```go
// Unicorn - Same functionality, less code
app.Handle(&unicorn.Handler{
    Name: "CreateUser",
    Triggers: []unicorn.Trigger{
        {Type: unicorn.TriggerHTTP, Method: "POST", Path: "/users"},
    },
    Handler: func(ctx *unicorn.Context) error {
        var user User
        ctx.Bind(&user)
        ctx.DB().Create(&user)
        return ctx.JSON(200, user)
    },
})
```

**When to use Go-Kit:**
- Large team with strict architecture requirements
- Need maximum flexibility and control
- Enterprise project with compliance requirements
- Already invested in Go-Kit ecosystem

**When to use Unicorn:**
- Want productivity without sacrificing flexibility
- Small-medium team that needs to ship fast
- Don't want to maintain lots of boilerplate
- Need built-in support for common patterns

### vs Chi (Lightweight Router)

**Chi** is a lightweight, composable, and idiomatic Go router.

```go
// Chi - Pure router, very minimalist
r := chi.NewRouter()
r.Use(middleware.Logger)
r.Post("/users", func(w http.ResponseWriter, r *http.Request) {
    var user User
    json.NewDecoder(r.Body).Decode(&user)
    db.Create(&user) // Manage DB yourself
    json.NewEncoder(w).Encode(user)
})
http.ListenAndServe(":8080", r)
```

```go
// Unicorn - More batteries included
app.Handle(&unicorn.Handler{
    Triggers: []unicorn.Trigger{{Type: unicorn.TriggerHTTP, Method: "POST", Path: "/users"}},
    Handler: func(ctx *unicorn.Context) error {
        var user User
        ctx.Bind(&user)
        ctx.DB().Create(&user) // DB injected
        return ctx.JSON(200, user)
    },
})
```

**When to use Chi:**
- Need a very lightweight and fast router
- Prefer standard library style (`http.Handler`)
- Don't need extra features (manage everything yourself)
- Like composable middleware pattern

**When to use Unicorn:**
- Need more than just routing
- Want built-in infrastructure management
- Need multiple triggers for one handler

### vs Beego (Full-Stack MVC)

**Beego** is a full-stack framework with MVC pattern, ORM, and code generation.

```go
// Beego - MVC style with controller
type UserController struct {
    beego.Controller
}

func (c *UserController) Post() {
    var user User
    json.Unmarshal(c.Ctx.Input.RequestBody, &user)
    o := orm.NewOrm()
    o.Insert(&user)
    c.Data["json"] = user
    c.ServeJSON()
}

// Router
beego.Router("/users", &UserController{})
```

```go
// Unicorn - Functional style
app.Handle(&unicorn.Handler{
    Triggers: []unicorn.Trigger{{Type: unicorn.TriggerHTTP, Method: "POST", Path: "/users"}},
    Handler: func(ctx *unicorn.Context) error {
        var user User
        ctx.Bind(&user)
        ctx.DB().Create(&user)
        return ctx.JSON(200, user)
    },
})
```

**When to use Beego:**
- Like traditional MVC pattern
- Need built-in ORM (Beego ORM)
- Building monolithic web application
- Need admin dashboard generator (bee tool)
- Background in PHP/Rails/Django

**When to use Unicorn:**
- Prefer functional handler style
- Building microservices, not monoliths
- Want flexibility to choose your own ORM (GORM, Ent, sqlx, etc.)
- Need multi-trigger handlers

### vs Iris (Feature-Rich Web Framework)

**Iris** is a feature-rich web framework with MVC support.

```go
// Iris - Rich features, multiple patterns
app := iris.New()
app.Post("/users", func(ctx iris.Context) {
    var user User
    ctx.ReadJSON(&user)
    db.Create(&user)
    ctx.JSON(user)
})
app.Listen(":8080")

// Or MVC style
type UserController struct{}
func (c *UserController) Post(user User) User {
    db.Create(&user)
    return user
}
```

```go
// Unicorn - Consistent pattern
app.Handle(&unicorn.Handler{
    Triggers: []unicorn.Trigger{{Type: unicorn.TriggerHTTP, Method: "POST", Path: "/users"}},
    Handler: func(ctx *unicorn.Context) error {
        var user User
        ctx.Bind(&user)
        ctx.DB().Create(&user)
        return ctx.JSON(200, user)
    },
})
```

**When to use Iris:**
- Need many built-in features (websocket, sessions, i18n, etc.)
- Want flexibility between functional and MVC
- Building web application with many features
- High performance for HTTP

**When to use Unicorn:**
- Focus on microservices, not web apps
- Need gRPC and message queue
- Prefer consistent single pattern
- Need swappable infrastructure

### vs Buffalo (Rapid Web Development)

**Buffalo** is a framework for Rails-like rapid web development.

```go
// Buffalo - Convention over configuration
// Generate with CLI: buffalo new myapp
// File: actions/users.go

func UsersCreate(c buffalo.Context) error {
    user := &models.User{}
    c.Bind(user)
    tx := c.Value("tx").(*pop.Connection)
    tx.Create(user)
    return c.Render(201, r.JSON(user))
}

// routes.go
app.POST("/users", UsersCreate)
```

```go
// Unicorn - Explicit configuration
app.Handle(&unicorn.Handler{
    Triggers: []unicorn.Trigger{{Type: unicorn.TriggerHTTP, Method: "POST", Path: "/users"}},
    Handler: func(ctx *unicorn.Context) error {
        var user User
        ctx.Bind(&user)
        ctx.DB().Create(&user)
        return ctx.JSON(201, user)
    },
})
```

**When to use Buffalo:**
- Need rapid development with generators
- Building full web app with frontend (templates)
- Like convention over configuration (Rails-like)
- Need asset pipeline and hot reload

**When to use Unicorn:**
- Building API-only services
- Prefer explicit over convention
- Need multi-protocol support (HTTP + gRPC + MQ)
- Building microservices

### vs Kratos (Microservice Framework)

**Kratos** is a microservice framework from Bilibili.

```go
// Kratos - Protocol-first approach
// 1. Define proto file
// 2. Generate code
// 3. Implement service

type UserService struct {
    pb.UnimplementedUserServiceServer
    db *gorm.DB
}

func (s *UserService) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
    user := &User{Name: req.Name}
    s.db.Create(user)
    return &pb.User{Id: user.ID, Name: user.Name}, nil
}
```

```go
// Unicorn - Code-first approach
app.Handle(&unicorn.Handler{
    Name: "CreateUser",
    Triggers: []unicorn.Trigger{
        {Type: unicorn.TriggerHTTP, Method: "POST", Path: "/users"},
        {Type: unicorn.TriggerGRPC, Method: "CreateUser"},
    },
    Handler: func(ctx *unicorn.Context) error {
        var user User
        ctx.Bind(&user)
        ctx.DB().Create(&user)
        return ctx.JSON(200, user)
    },
})
```

**When to use Kratos:**
- Need protocol-first development (proto files)
- Building services for high-traffic systems
- Already familiar with Kratos ecosystem
- Need built-in service discovery and config management

**When to use Unicorn:**
- Prefer code-first development
- Want flexibility without code generation
- Need quick iteration and prototyping
- One handler for HTTP and gRPC

## Unique Unicorn Features

### 1. Multi-Trigger Handlers

One business logic, multiple entry points:

```go
app.Handle(&unicorn.Handler{
    Name: "ProcessPayment",
    Triggers: []unicorn.Trigger{
        {Type: unicorn.TriggerHTTP, Method: "POST", Path: "/payments"},
        {Type: unicorn.TriggerMessage, Queue: "payment.process"},
        {Type: unicorn.TriggerGRPC, Method: "ProcessPayment"},
        {Type: unicorn.TriggerCron, Schedule: "0 0 * * *"}, // Daily reconciliation
    },
    Handler: processPaymentHandler,
})
```

### 2. Generic Adapter Pattern

Swap infrastructure without changing business logic:

```go
// Development
app.SetCache(redis.NewDriver(redisClient))

// Production with Memcached
app.SetCache(memcached.NewDriver(mcClient))

// Handler code stays exactly the same
func handler(ctx *unicorn.Context) error {
    ctx.Cache().Set("key", "value", time.Hour)
    return nil
}
```

### 3. Multiple Named Adapters

Support multiple instances for scaling:

```go
// Multiple databases
app.SetDB(primaryDB)                    // Default
app.SetDB(analyticsDB, "analytics")     // Named
app.SetDB(replicaDB, "replica")         // Named

// In handler
func handler(ctx *unicorn.Context) error {
    ctx.DB().Create(&user)                    // Primary
    ctx.DB("analytics").Create(&event)        // Analytics DB
    ctx.DB("replica").Find(&users)            // Read from replica
    return nil
}
```

### 4. Built-in Observability

Metrics, tracing, and logging with generic adapters:

```go
app.SetMetrics(prometheus.NewDriver())
app.SetTracer(jaeger.NewDriver("service-name"))
app.SetLogger(zap.NewDriver())

// Automatic instrumentation on every request
```

### 5. Production-Ready Resilience Patterns

Built-in circuit breaker, retry, and bulkhead patterns:

```go
// Circuit Breaker - Prevent cascade failures
cb := circuitbreaker.New("payment-service", circuitbreaker.Config{
    MaxRequests: 3,
    Timeout:     30 * time.Second,
    ReadyToTrip: func(counts circuitbreaker.Counts) bool {
        return counts.ConsecutiveFailures > 5
    },
})

result, err := cb.Execute(func() (interface{}, error) {
    return paymentService.Process(payment)
})

// Retry with Exponential Backoff
retry.Do(ctx, func() error {
    return externalAPI.Call()
}, retry.WithMaxAttempts(3), retry.WithBackoff(time.Second))
```

### 6. Complete Security Middleware

JWT, API Key, Basic Auth, Rate Limiting, and CORS:

```go
// JWT Authentication
app.Use(auth.JWT(auth.JWTConfig{
    Secret: []byte("your-secret"),
}))

// Rate Limiting (Memory or Redis)
app.Use(ratelimit.New(ratelimit.Config{
    Max:      100,
    Duration: time.Minute,
}))

// CORS
app.Use(cors.New(cors.Config{
    AllowOrigins: []string{"https://example.com"},
}))
```

### 7. Kubernetes-Ready Health Checks

Built-in liveness and readiness probes:

```go
health := middleware.NewHealthChecker()
health.AddCheck("database", dbChecker)
health.AddCheck("redis", redisChecker)

app.GET("/health/live", health.LivenessHandler())
app.GET("/health/ready", health.ReadinessHandler())
```

## Migration Guide

### From Gin to Unicorn

```go
// Before (Gin)
r := gin.Default()
r.POST("/users", createUser)
r.GET("/users/:id", getUser)

// After (Unicorn)
app := unicorn.New(&unicorn.Config{Name: "my-service"})
app.Handle(&unicorn.Handler{
    Triggers: []unicorn.Trigger{{Type: unicorn.TriggerHTTP, Method: "POST", Path: "/users"}},
    Handler: createUser,
})
app.Handle(&unicorn.Handler{
    Triggers: []unicorn.Trigger{{Type: unicorn.TriggerHTTP, Method: "GET", Path: "/users/:id"}},
    Handler: getUser,
})
```

### From Echo to Unicorn

```go
// Before (Echo)
e := echo.New()
e.POST("/users", createUser)

// After (Unicorn)
app := unicorn.New(&unicorn.Config{Name: "my-service"})
app.Handle(&unicorn.Handler{
    Triggers: []unicorn.Trigger{{Type: unicorn.TriggerHTTP, Method: "POST", Path: "/users"}},
    Handler: func(ctx *unicorn.Context) error {
        // Similar context API
        return ctx.JSON(200, user)
    },
})
```

## TL;DR - When to Use What?

| Use Case | Recommended Framework |
|----------|----------------------|
| Simple REST API, high performance | Gin, Echo, Fiber |
| Minimalist router, standard lib style | Chi |
| Full-stack web app MVC style | Beego, Buffalo |
| Feature-rich web framework | Iris |
| Enterprise microservices, strict architecture | Go-Kit |
| Protocol-first, high-traffic services | Kratos |
| Multi-protocol microservices (HTTP+gRPC+MQ) | **Unicorn** |
| Swappable infrastructure without code changes | **Unicorn** |
| One handler for multiple triggers | **Unicorn** |

## Conclusion

No framework is perfect for all use cases. Choose based on your project needs:

- **Simple HTTP API**: Gin, Echo, Fiber, Chi
- **Full-stack Web App**: Beego, Buffalo, Iris
- **Complex Microservices with strict architecture**: Go-Kit
- **Protocol-first high-traffic services**: Kratos
- **Flexible microservices with multi-trigger support**: Unicorn

Every framework has its own philosophy and trade-offs:

| Philosophy | Frameworks |
|------------|-----------|
| Minimalist & Fast | Chi, Gin |
| Batteries Included | Beego, Buffalo, Iris |
| Enterprise Grade | Go-Kit, Kratos |
| Pragmatic Middle Ground | Echo, Fiber, **Unicorn** |

---

*"Talk is cheap. Show me the code." - Linus Torvalds*

*"A good framework is one that solves your problem, not one that wins benchmarks." - Unicorn Creator, who's too lazy to debate*

*"Choosing a framework is like choosing a partner - pick the one that fits, not the most popular one." - Wisdom from a developer tired of refactoring*
