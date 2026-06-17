# Migration Guide — Existing Service ke Unicorn

> **Goal**: Smooth, incremental migration tanpa full rewrite dan tanpa downtime.

## Prinsip

```
Jangan migrasi service sekaligus.
Migrasi handler-by-handler.
Semua yang belum dimigrasi tetap berjalan normal.
```

Unicorn mendukung **hybrid deployment**: kode lama dan baru bisa jalan berdampingan dalam binary yang sama.

---

## Phase 0: Assessment

| Yang Perlu Dicek | Contoh |
|-----------------|--------|
| Framework saat ini | Gin, Echo, Fiber, net/http, Chi |
| Handler signature | `func(c *gin.Context)`, `func(w http.ResponseWriter, r *http.Request)` |
| Middleware stack | Recovery, CORS, Auth, Logging, Rate limit |
| Infrastructure access | DB, Cache, Logger — langsung panggil library atau via interface? |
| Route count | Total endpoint yang perlu dimigrasi |

```bash
# Estimasi effort
grep -r "^func.*Handler\|^func.*handler\|router\." ./internal/ | wc -l
```

---

## Phase 1: Co-Exist (Tanpa Ubah Kode Existing)

Strategi: **Unicorn berjalan SAMPING** service existing, sharing infrastructure yang sama.

```
Port 8080 (existing)          Port 8081 (unicorn)
┌──────────────────────┐      ┌──────────────────────┐
│ Gin / Echo / net/http │      │ Unicorn              │
│ POST /users (lama)    │      │ GET /users (baru)    │
│ GET  /users/:id (lama)│      │ POST /orders (baru)  │
│ shared infra          │      │ shared infra         │
└──────────┬───────────┘      └──────────┬───────────┘
           │                             │
           └──────────┬──────────────────┘
                      ▼
            DB / Cache / Logger
```

### 1a. Bungkus Infrastructure Existing Jadi Unicorn Adapter

Infrastructure paling gampang dimigrasi duluan karena Unicorn adapter pattern.

```go
// Existing: kode lamamu langsung panggil *sql.DB
var db *sql.DB

// Bridge: bungkus jadi Unicorn Database adapter
type existingDBBridge struct {
    raw *sql.DB
}

func (b *existingDBBridge) Create(ctx context.Context, entity any) error {
    // Panggil query existing
    _, err := b.raw.ExecContext(ctx, "INSERT INTO ...", entity)
    return err
}

// ... implement contracts.Database lainnya

app.SetDB(&existingDBBridge{raw: db})
```

**Keuntungan:** Satu koneksi pool, dua framework. Tidak perlu duplikasi config atau koneksi.

### 1b. Registrasi Handler Unicorn di Port Berbeda

```go
func main() {
    // App Unicorn di port 8081
    unicornApp := unicorn.New(&unicorn.Config{
        Name:       "migrated-service",
        EnableHTTP: true,
        HTTP: &httpAdapter.Config{Port: 8081},
    })

    // Bridge infrastructure yang sudah ada
    unicornApp.SetDB(&existingDBBridge{raw: db})
    unicornApp.SetLogger(&existingLoggerBridge{raw: logger})
    unicornApp.SetCache(&existingCacheBridge{raw: cache})

    // Migrasi handler satu per satu
    unicornApp.RegisterHandler(GetUser).HTTP("GET", "/users/:id").Done()
    unicornApp.RegisterHandler(CreateOrder).HTTP("POST", "/orders").Done()

    // Service lama tetap jalan
    go startOldService(":8080")

    // Service Unicorn jalan
    unicornApp.Start()
}
```

**Test:** `curl localhost:8081/users/123` → response Unicorn.
**Bandingkan:** `curl localhost:8080/users/123` → response existing.

---

## Phase 2: Handler Migration (Satu Per Satu)

### 2a. Mapping Handler Signature Per Framework

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
    // req.ID sudah terisi otomatis dari path parameter
    return &User{ID: req.ID}, nil
}
```

### 2b. Migrasi Middleware

Unicorn middleware pakai `*context.Context`, bukan `*gin.Context`.

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

### 2c. Testing Bridge — Response Comparison

Sebelum cutover, verifikasi response lama dan baru identik:

```go
func TestMigration_GetUser(t *testing.T) {
    // Hit endpoint lama
    oldResp := hitOldService("/users/123")

    // Hit endpoint baru (Unicorn)
    newResp := hitNewUnicorn("/users/123")

    // Bandingkan
    if !reflect.DeepEqual(oldResp, newResp) {
        t.Errorf("response mismatch:\nold: %+v\nnew: %+v", oldResp, newResp)
    }
}
```

---

## Phase 3: Reverse Proxy + Gradual Cutover

Pasang reverse proxy (nginx, Caddy, atau Go native) yang bisa route per-endpoint.

```nginx
# nginx.conf — migrasi bertahap

upstream old_service {
    server localhost:8080;
}

upstream new_service {
    server localhost:8081;
}

server {
    listen 80;

    # Endpoint yang sudah dimigrasi → unicorn
    location /users/ {
        proxy_pass http://new_service;
    }
    location /orders/ {
        proxy_pass http://new_service;
    }

    # Endpoint yang belum dimigrasi → service lama
    location / {
        proxy_pass http://old_service;
    }
}
```

```go
// Atau pake Unicorn reverse proxy sidecar
type ReverseProxySidecar struct {
    oldBackend string // "localhost:8080"
}

func (p *ReverseProxySidecar) Name() string { return "migration-proxy" }

func (p *ReverseProxySidecar) Start(ctx context.Context) error {
    // Proxy: /users → Unicorn, /products → old service
    // ...
}

app.AddSidecar(ReverseProxySidecar{oldBackend: "localhost:8080"})
```

---

## Phase 4: Cutover

Setelah semua handler dimigrasi:

1. Update proxy: semua traffic → Unicorn port
2. Matikan service lama
3. Hapus code lama
4. Ganti port Unicorn ke port utama (8080)

```go
// Final: cuma Unicorn, nggak ada lagi service lama
func main() {
    app := unicorn.New(&unicorn.Config{
        Name:       "fully-migrated",
        EnableHTTP: true,
        HTTP:       &httpAdapter.Config{Port: 8080}, // ← port utama
    })

    // Semua handler dari hasil migrasi
    app.RegisterHandler(GetUser).HTTP("GET", "/users/:id").Done()
    app.RegisterHandler(CreateUser).HTTP("POST", "/users").Done()
    app.RegisterHandler(ListUsers).HTTP("GET", "/users").Done()
    app.RegisterHandler(CreateOrder).HTTP("POST", "/orders").Done()
    app.RegisterHandler(GetOrder).HTTP("GET", "/orders/:id").Done()

    app.Start()
}
```

---

## Timeline Realistis

```
Minggu 1:  Phase 0 — Assessment + setup
Minggu 2:  Phase 1 — Bridge infra + co-exist
Minggu 3-4: Phase 2 — Migrasi handler (5-10 handler/minggu)
Minggu 5:  Phase 3 — Reverse proxy + verifikasi
Minggu 6:  Phase 4 — Cutover + cleanup
```

---

## Anti-Patterns

| ❌ Jangan | ✅ Lakukan |
|-----------|-----------|
| Migrasi semua handler sekaligus | Satu endpoint per PR |
| Ubah infrastructure + handler barengan | Infrastructure bridge dulu, handler belakangan |
| Buka 2 port di production tanpa proxy | Proxy dari hari pertama |
| Test cuma di lokal | Staging environment dengan traffic mirror |
| Cutover di akhir pekan tanpa rollback plan | Selalu siapkan `switch --back` |

---

Kuncinya ada di **Phase 1: Co-Exist**. Selama Unicorn dan service lama bisa jalan bareng dan sharing infra yang sama, migrasi bisa dilakukan handler-by-handler tanpa tekanan. Tidak perlu "big bang migration."
