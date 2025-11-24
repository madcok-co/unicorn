# Perbandingan Framework

> **Disclaimer dari Creator**: Saya malas berdebat dengan orang yang banyak bacot teori. Framework ini dibuat untuk solve real problems, bukan untuk menang argumen di Twitter. Kalau cocok, pakai. Kalau nggak, ya sudah, hidup terus berjalan.

## Overview

Unicorn adalah framework Go yang menggabungkan kesederhanaan web framework dengan kekuatan enterprise patterns. Berikut perbandingan dengan framework Go populer lainnya.

## Tabel Perbandingan

| Fitur | Unicorn | Gin | Echo | Fiber | Chi | Beego | Iris | Buffalo | Go-Kit | Kratos |
|-------|---------|-----|------|-------|-----|-------|------|---------|--------|--------|
| HTTP Server | Ya | Ya | Ya | Ya | Ya | Ya | Ya | Ya | Ya | Ya |
| gRPC Server | Ya | Tidak | Tidak | Tidak | Tidak | Tidak | Tidak | Tidak | Ya | Ya |
| Message Queue | Ya | Tidak | Tidak | Tidak | Tidak | Tidak | Tidak | Tidak | Manual | Ya |
| Cron Jobs | Ya | Tidak | Tidak | Tidak | Tidak | Ya | Tidak | Ya | Manual | Ya |
| Multi-Trigger Handler | Ya | Tidak | Tidak | Tidak | Tidak | Tidak | Tidak | Tidak | Tidak | Tidak |
| Generic Adapters | Ya | Tidak | Tidak | Tidak | Tidak | Sebagian | Tidak | Sebagian | Sebagian | Sebagian |
| Swappable Infra | Ya | Manual | Manual | Manual | Manual | Manual | Manual | Manual | Ya | Ya |
| Circuit Breaker | Ya | Tidak | Tidak | Tidak | Tidak | Tidak | Tidak | Tidak | Ya | Ya |
| Retry/Backoff | Ya | Tidak | Tidak | Tidak | Tidak | Tidak | Tidak | Tidak | Manual | Sebagian |
| Rate Limiting | Ya | Tidak | Sebagian | Sebagian | Tidak | Tidak | Sebagian | Tidak | Manual | Ya |
| Health Checks | Ya | Tidak | Tidak | Tidak | Tidak | Tidak | Tidak | Tidak | Manual | Ya |
| Auth Middleware | Ya | Tidak | Sebagian | Sebagian | Tidak | Ya | Sebagian | Sebagian | Manual | Sebagian |
| CORS Middleware | Ya | Manual | Ya | Ya | Manual | Ya | Ya | Ya | Manual | Manual |
| ORM Built-in | Tidak | Tidak | Tidak | Tidak | Tidak | Ya | Tidak | Ya | Tidak | Tidak |
| MVC Pattern | Tidak | Tidak | Tidak | Tidak | Tidak | Ya | Ya | Ya | Tidak | Tidak |
| Code Generation | Tidak | Tidak | Tidak | Tidak | Tidak | Ya | Tidak | Ya | Tidak | Ya |
| Learning Curve | Sedang | Rendah | Rendah | Rendah | Rendah | Sedang | Sedang | Sedang | Tinggi | Tinggi |
| Performa | Tinggi | Tinggi | Tinggi | Sangat Tinggi | Tinggi | Sedang | Tinggi | Sedang | Sedang | Sedang |
| Boilerplate | Rendah | Rendah | Rendah | Rendah | Sangat Rendah | Sedang | Rendah | Sedang | Tinggi | Sedang |
| GitHub Stars | Baru | ~80k | ~30k | ~35k | ~18k | ~32k | ~25k | ~8k | ~27k | ~24k |

## Perbandingan Detail

### vs Gin/Echo/Fiber (Web Framework)

**Gin, Echo, Fiber** adalah web framework yang excellent untuk HTTP APIs.

```go
// Gin - Simple HTTP handler
r := gin.Default()
r.POST("/users", func(c *gin.Context) {
    var user User
    c.BindJSON(&user)
    // Save ke DB - koneksi dikelola sendiri
    db.Create(&user)
    c.JSON(200, user)
})
r.Run(":8080")
```

```go
// Unicorn - Handler yang sama, multiple triggers
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
        ctx.DB().Create(&user) // DB di-inject otomatis
        return ctx.JSON(200, user)
    },
})
```

**Kapan pakai Gin/Echo/Fiber:**
- Butuh pure HTTP API dengan performa maksimal
- Project kecil sampai medium
- Tim sudah familiar dengan framework tersebut
- Tidak butuh message queue atau gRPC

**Kapan pakai Unicorn:**
- Butuh HTTP + gRPC + Message Queue dalam satu service
- Ingin satu handler untuk multiple triggers
- Butuh swappable infrastructure (ganti Redis ke Memcached tanpa ubah code)
- Building microservices yang perlu scalable

### vs Go-Kit (Microservice Toolkit)

**Go-Kit** adalah toolkit untuk building microservices dengan separation of concerns yang ketat.

```go
// Go-Kit - Banyak boilerplate
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
// Unicorn - Fungsi yang sama, code lebih sedikit
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

**Kapan pakai Go-Kit:**
- Tim besar dengan strict architecture requirements
- Butuh maximum flexibility dan control
- Project enterprise dengan compliance requirements
- Sudah ada investment di Go-Kit ecosystem

**Kapan pakai Unicorn:**
- Ingin productivity tanpa sacrifice flexibility
- Tim kecil-medium yang butuh ship fast
- Tidak ingin maintain banyak boilerplate
- Butuh built-in support untuk common patterns

### vs Chi (Lightweight Router)

**Chi** adalah lightweight router yang composable dan idiomatic Go.

```go
// Chi - Pure router, sangat minimalis
r := chi.NewRouter()
r.Use(middleware.Logger)
r.Post("/users", func(w http.ResponseWriter, r *http.Request) {
    var user User
    json.NewDecoder(r.Body).Decode(&user)
    db.Create(&user) // DB dikelola sendiri
    json.NewEncoder(w).Encode(user)
})
http.ListenAndServe(":8080", r)
```

```go
// Unicorn - Lebih lengkap
app.Handle(&unicorn.Handler{
    Triggers: []unicorn.Trigger{{Type: unicorn.TriggerHTTP, Method: "POST", Path: "/users"}},
    Handler: func(ctx *unicorn.Context) error {
        var user User
        ctx.Bind(&user)
        ctx.DB().Create(&user) // DB di-inject
        return ctx.JSON(200, user)
    },
})
```

**Kapan pakai Chi:**
- Butuh router yang sangat ringan dan cepat
- Prefer standard library style (`http.Handler`)
- Tidak butuh fitur tambahan (manage sendiri)
- Suka composable middleware pattern

**Kapan pakai Unicorn:**
- Butuh lebih dari sekedar routing
- Ingin infrastructure management built-in
- Butuh multiple triggers untuk satu handler

### vs Beego (Full-Stack MVC)

**Beego** adalah full-stack framework dengan MVC pattern, ORM, dan code generation.

```go
// Beego - MVC style dengan controller
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

**Kapan pakai Beego:**
- Suka MVC pattern traditional
- Butuh ORM built-in (Beego ORM)
- Building monolithic web application
- Butuh admin dashboard generator (bee tool)
- Background dari PHP/Rails/Django

**Kapan pakai Unicorn:**
- Prefer functional handler style
- Building microservices bukan monolith
- Ingin flexibility pilih ORM sendiri (GORM, Ent, sqlx, dll)
- Butuh multi-trigger handlers

### vs Iris (Feature-Rich Web Framework)

**Iris** adalah web framework yang feature-rich dengan MVC support.

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

// Atau MVC style
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

**Kapan pakai Iris:**
- Butuh banyak built-in features (websocket, sessions, i18n, dll)
- Ingin flexibility antara functional dan MVC
- Building web application dengan banyak fitur
- Performa tinggi untuk HTTP

**Kapan pakai Unicorn:**
- Focus ke microservices bukan web apps
- Butuh gRPC dan message queue
- Prefer consistent single pattern
- Butuh swappable infrastructure

### vs Buffalo (Rapid Web Development)

**Buffalo** adalah framework untuk rapid web development ala Rails.

```go
// Buffalo - Convention over configuration
// Generate dengan CLI: buffalo new myapp
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

**Kapan pakai Buffalo:**
- Butuh rapid development dengan generators
- Building full web app dengan frontend (templates)
- Suka convention over configuration (Rails-like)
- Butuh asset pipeline dan hot reload

**Kapan pakai Unicorn:**
- Building API-only services
- Prefer explicit over convention
- Butuh multi-protocol support (HTTP + gRPC + MQ)
- Building microservices

### vs Kratos (Microservice Framework)

**Kratos** adalah framework dari Bilibili untuk building microservices.

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

**Kapan pakai Kratos:**
- Butuh protocol-first development (proto files)
- Building services untuk high-traffic systems
- Sudah familiar dengan Kratos ecosystem
- Butuh built-in service discovery dan config management

**Kapan pakai Unicorn:**
- Prefer code-first development
- Ingin flexibility tanpa code generation
- Butuh quick iteration dan prototyping
- Satu handler untuk HTTP dan gRPC

## Fitur Unik Unicorn

### 1. Multi-Trigger Handlers

Satu business logic, multiple entry points:

```go
app.Handle(&unicorn.Handler{
    Name: "ProcessPayment",
    Triggers: []unicorn.Trigger{
        {Type: unicorn.TriggerHTTP, Method: "POST", Path: "/payments"},
        {Type: unicorn.TriggerMessage, Queue: "payment.process"},
        {Type: unicorn.TriggerGRPC, Method: "ProcessPayment"},
        {Type: unicorn.TriggerCron, Schedule: "0 0 * * *"}, // Rekonsiliasi harian
    },
    Handler: processPaymentHandler,
})
```

### 2. Generic Adapter Pattern

Ganti infrastructure tanpa ubah business logic:

```go
// Development
app.SetCache(redis.NewDriver(redisClient))

// Production dengan Memcached
app.SetCache(memcached.NewDriver(mcClient))

// Code handler tetap sama persis
func handler(ctx *unicorn.Context) error {
    ctx.Cache().Set("key", "value", time.Hour)
    return nil
}
```

### 3. Multiple Named Adapters

Support multiple instances untuk scaling:

```go
// Multiple databases
app.SetDB(primaryDB)                    // Default
app.SetDB(analyticsDB, "analytics")     // Named
app.SetDB(replicaDB, "replica")         // Named

// Di handler
func handler(ctx *unicorn.Context) error {
    ctx.DB().Create(&user)                    // Primary
    ctx.DB("analytics").Create(&event)        // Analytics DB
    ctx.DB("replica").Find(&users)            // Read dari replica
    return nil
}
```

### 4. Built-in Observability

Metrics, tracing, dan logging dengan generic adapters:

```go
app.SetMetrics(prometheus.NewDriver())
app.SetTracer(jaeger.NewDriver("service-name"))
app.SetLogger(zap.NewDriver())

// Instrumentasi otomatis di setiap request
```

### 5. Production-Ready Resilience Patterns

Circuit breaker, retry, dan bulkhead patterns built-in:

```go
// Circuit Breaker - Cegah cascade failures
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

// Retry dengan Exponential Backoff
retry.Do(ctx, func() error {
    return externalAPI.Call()
}, retry.WithMaxAttempts(3), retry.WithBackoff(time.Second))
```

### 6. Complete Security Middleware

JWT, API Key, Basic Auth, Rate Limiting, dan CORS:

```go
// JWT Authentication
app.Use(auth.JWT(auth.JWTConfig{
    Secret: []byte("your-secret"),
}))

// Rate Limiting (Memory atau Redis)
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

Built-in liveness dan readiness probes:

```go
health := middleware.NewHealthChecker()
health.AddCheck("database", dbChecker)
health.AddCheck("redis", redisChecker)

app.GET("/health/live", health.LivenessHandler())
app.GET("/health/ready", health.ReadinessHandler())
```

## Panduan Migrasi

### Dari Gin ke Unicorn

```go
// Sebelum (Gin)
r := gin.Default()
r.POST("/users", createUser)
r.GET("/users/:id", getUser)

// Sesudah (Unicorn)
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

### Dari Echo ke Unicorn

```go
// Sebelum (Echo)
e := echo.New()
e.POST("/users", createUser)

// Sesudah (Unicorn)
app := unicorn.New(&unicorn.Config{Name: "my-service"})
app.Handle(&unicorn.Handler{
    Triggers: []unicorn.Trigger{{Type: unicorn.TriggerHTTP, Method: "POST", Path: "/users"}},
    Handler: func(ctx *unicorn.Context) error {
        // Context API yang mirip
        return ctx.JSON(200, user)
    },
})
```

## TL;DR - Kapan Pakai Apa?

| Use Case | Framework yang Direkomendasikan |
|----------|--------------------------------|
| Simple REST API, performa tinggi | Gin, Echo, Fiber |
| Minimalist router, standard lib style | Chi |
| Full-stack web app MVC style | Beego, Buffalo |
| Feature-rich web framework | Iris |
| Enterprise microservices, strict architecture | Go-Kit |
| Protocol-first, high-traffic services | Kratos |
| Multi-protocol microservices (HTTP+gRPC+MQ) | **Unicorn** |
| Swappable infrastructure tanpa ubah code | **Unicorn** |
| Satu handler untuk multiple triggers | **Unicorn** |

## Kesimpulan

Tidak ada framework yang sempurna untuk semua use case. Pilih berdasarkan kebutuhan project:

- **Simple HTTP API**: Gin, Echo, Fiber, Chi
- **Full-stack Web App**: Beego, Buffalo, Iris
- **Complex Microservices dengan strict architecture**: Go-Kit
- **Protocol-first high-traffic services**: Kratos
- **Flexible microservices dengan multi-trigger support**: Unicorn

Setiap framework punya philosophy dan trade-off masing-masing:

| Philosophy | Framework |
|------------|-----------|
| Minimalist & Fast | Chi, Gin |
| Batteries Included | Beego, Buffalo, Iris |
| Enterprise Grade | Go-Kit, Kratos |
| Pragmatic Middle Ground | Echo, Fiber, **Unicorn** |

---

*"Talk is cheap. Show me the code." - Linus Torvalds*

*"Framework yang bagus adalah yang menyelesaikan masalahmu, bukan yang menang benchmark." - Creator Unicorn, yang malas berdebat*

*"Pilih framework itu kayak pilih jodoh - yang penting cocok, bukan yang paling populer." - Wisdom dari developer yang sudah capek refactor*
