# Migration Guide — Migrating Existing Services to Unicorn

> **Goal**: Smooth, incremental migration without a full rewrite or downtime.

## Principle

```
Don't migrate the entire service at once.
Migrate handler by handler.
Everything not yet migrated keeps running normally.
```

Unicorn supports **hybrid deployment**: old and new code can coexist in the same binary.

---

## Phase 0: Assessment

| Check | Example |
|-------|---------|
| Current framework | Gin, Echo, Fiber, net/http, Chi |
| Handler signature | `func(c *gin.Context)`, `func(w http.ResponseWriter, r *http.Request)` |
| Middleware stack | Recovery, CORS, Auth, Logging, Rate limiting |
| Infrastructure access | DB, Cache, Logger — direct calls or via interfaces? |
| Route count | Total endpoints to migrate |

```bash
# Estimate effort
grep -r "router\.\|^func.*Handler\|^func.*handler" ./internal/ | wc -l
```

---

## Phase 1: Co-Exist (No Changes to Existing Code)

Strategy: **Unicorn runs ALONGSIDE** the existing service, sharing the same infrastructure.

```
Port 8080 (existing)          Port 8081 (unicorn)
┌──────────────────────┐      ┌──────────────────────┐
│ Gin / Echo / net/http│      │ Unicorn              │
│ POST /users (old)    │      │ GET /users (new)     │
│ GET  /users/:id (old)│      │ POST /orders (new)   │
│ shared infra         │      │ shared infra         │
└──────────┬───────────┘      └──────────┬───────────┘
           │                             │
           └──────────┬──────────────────┘
                      ▼
            DB / Cache / Logger
```

### 1a. Wrap Existing Infrastructure as Unicorn Adapters

Infrastructure is the easiest part to migrate first thanks to the adapter pattern.

```go
// Existing: code directly calls *sql.DB
var db *sql.DB

// Bridge: wrap into a Unicorn Database adapter
type existingDBBridge struct {
    raw *sql.DB
}

func (b *existingDBBridge) Create(ctx context.Context, entity any) error {
    _, err := b.raw.ExecContext(ctx, "INSERT INTO ...", entity)
    return err
}

// ... implement remaining contracts.Database methods

app.SetDB(&existingDBBridge{raw: db})
```

**Benefit:** Single connection pool, two frameworks. No duplicated config or connections.

### 1b. Run Unicorn on a Separate Port

```go
func main() {
    // Unicorn app on port 8081
    unicornApp := unicorn.New(&unicorn.Config{
        Name:       "migrated-service",
        EnableHTTP: true,
        HTTP:       &httpAdapter.Config{Port: 8081},
    })

    // Bridge existing infrastructure
    unicornApp.SetDB(&existingDBBridge{raw: db})
    unicornApp.SetLogger(&existingLoggerBridge{raw: logger})
    unicornApp.SetCache(&existingCacheBridge{raw: cache})

    // Migrate handlers one by one
    unicornApp.RegisterHandler(GetUser).HTTP("GET", "/users/:id").Done()
    unicornApp.RegisterHandler(CreateOrder).HTTP("POST", "/orders").Done()

    // Old service keeps running
    go startOldService(":8080")

    // Unicorn starts
    unicornApp.Start()
}
```

**Test:** `curl localhost:8081/users/123` → Unicorn response.
**Compare:** `curl localhost:8080/users/123` → existing response.

---

## Phase 2: Handler Migration (One at a Time)

### 2a. Handler Signature Mapping Per Framework

```go
// === Gin → Unicorn ===

// Gin
func GetUser(c *gin.Context) {
    id := c.Param("id")
    var user User
    db.Where("id = ?", id).First(&user)
    c.JSON(200, user)
}

// Unicorn
func GetUser(ctx *unicorn.Context, req GetUserRequest) (*User, error) {
    var user User
    ctx.DB().FindByID(ctx.Context(), req.ID, &user)
    return &user, nil
}

type GetUserRequest struct {
    ID string `path:"id"`
}
```

```go
// === Echo → Unicorn ===

// Echo
func GetUser(c echo.Context) error {
    id := c.Param("id")
    return c.JSON(200, user)
}

// Unicorn
func GetUser(ctx *unicorn.Context, req GetUserRequest) (*User, error) {
    // req.ID is automatically populated from the path parameter
    return &User{ID: req.ID}, nil
}
```

```go
// === net/http → Unicorn ===

// net/http
func GetUser(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    json.NewEncoder(w).Encode(user)
}

// Unicorn
func GetUser(ctx *unicorn.Context, req GetUserRequest) (*User, error) {
    ctx.DB().FindByID(ctx.Context(), req.ID, &user)
    return user, nil
}
```

### 2b. Migrate Middleware

```go
// === Gin middleware → Unicorn middleware ===

// Gin
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        if token == "" {
            c.AbortWithStatus(401)
            return
        }
        c.Next()
    }
}

// Unicorn
func AuthMiddleware() context.MiddlewareFunc {
    return func(next context.HandlerFunc) context.HandlerFunc {
        return func(ctx *context.Context) error {
            token := ctx.Request().Header("Authorization")
            if token == "" {
                return ctx.Error(401, "unauthorized")
            }
            return next(ctx)
        }
    }
}
```

### 2c. Response Comparison Test

Before cutting over, verify that old and new responses are identical:

```go
func TestMigration_GetUser(t *testing.T) {
    // Hit old endpoint
    oldResp := hitOldService("/users/123")

    // Hit new Unicorn endpoint
    newResp := hitNewUnicorn("/users/123")

    // Compare
    if !reflect.DeepEqual(oldResp, newResp) {
        t.Errorf("response mismatch:\nold: %+v\nnew: %+v", oldResp, newResp)
    }
}
```

---

## Phase 3: Reverse Proxy + Gradual Cutover

Use a reverse proxy (nginx, Caddy, or a Go reverse proxy) that can route per-endpoint.

```nginx
# nginx.conf — incremental migration

upstream old_service {
    server localhost:8080;
}

upstream new_service {
    server localhost:8081;
}

server {
    listen 80;

    # Migrated endpoints → unicorn
    location /users/ {
        proxy_pass http://new_service;
    }
    location /orders/ {
        proxy_pass http://new_service;
    }

    # Not-yet-migrated endpoints → old service
    location / {
        proxy_pass http://old_service;
    }
}
```

```go
// Or use a Unicorn reverse proxy sidecar
type ReverseProxySidecar struct {
    oldBackend string // "localhost:8080"
}

func (p *ReverseProxySidecar) Name() string { return "migration-proxy" }

func (p *ReverseProxySidecar) Start(ctx context.Context) error {
    // Proxy /users → Unicorn, /products → old service
    // ...
}

app.AddSidecar(ReverseProxySidecar{oldBackend: "localhost:8080"})
```

---

## Phase 4: Cutover

After all handlers are migrated:

1. Update proxy: all traffic → Unicorn port
2. Shut down the old service
3. Remove old code
4. Change Unicorn port to the primary port (8080)

```go
// Final: only Unicorn, no more old service
func main() {
    app := unicorn.New(&unicorn.Config{
        Name:       "fully-migrated",
        EnableHTTP: true,
        HTTP:       &httpAdapter.Config{Port: 8080}, // ← primary port
    })

    // All migrated handlers
    app.RegisterHandler(GetUser).HTTP("GET", "/users/:id").Done()
    app.RegisterHandler(CreateUser).HTTP("POST", "/users").Done()
    app.RegisterHandler(ListUsers).HTTP("GET", "/users").Done()
    app.RegisterHandler(CreateOrder).HTTP("POST", "/orders").Done()
    app.RegisterHandler(GetOrder).HTTP("GET", "/orders/:id").Done()

    app.Start()
}
```

---

## Realistic Timeline

```
Week 1:  Phase 0 — Assessment + setup
Week 2:  Phase 1 — Bridge infra + co-exist
Week 3-4: Phase 2 — Migrate handlers (5-10 handlers/week)
Week 5:  Phase 3 — Reverse proxy + verification
Week 6:  Phase 4 — Cutover + cleanup
```

---

## Anti-Patterns

| ❌ Don't | ✅ Do |
|----------|-------|
| Migrate all handlers at once | One endpoint per PR |
| Change infrastructure + handlers together | Bridge infra first, handlers later |
| Run 2 ports in production without a proxy | Proxy from day one |
| Test only locally | Staging environment with traffic mirroring |
| Cutover on a Friday without a rollback plan | Always have a `switch --back` ready |

---

The key is **Phase 1: Co-Exist**. As long as Unicorn and the old service can run side by side sharing the same infrastructure, migration can proceed handler by handler, under no pressure. No "big bang migration" required.
