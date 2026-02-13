# Middleware

Unicorn provides production-ready middleware for common use cases.

## Quick Start

```go
import "github.com/madcok-co/unicorn/core/pkg/middleware"

// Use individual middleware
app.Use(middleware.Recovery())
app.Use(middleware.CORS())

// Or use production stack
app.Use(middleware.ProductionStack(&middleware.ProductionStackConfig{
    Timeout: 30 * time.Second,
}))
```

## Available Middleware

### Recovery

Recovers from panics and returns 500 error:

```go
// Basic usage
app.Use(middleware.Recovery())

// With custom config
app.Use(middleware.RecoveryWithConfig(&middleware.RecoveryConfig{
    EnableStackTrace: true,
    Logger:           logger,
    OnPanic: func(ctx *context.Context, err interface{}, stack []byte) {
        // Send alert to monitoring
        alerting.SendPanicAlert(err, stack)
    },
}))
```

### Timeout

Enforces request timeout:

```go
// 30 second timeout
app.Use(middleware.Timeout(30 * time.Second))

// With custom config
app.Use(middleware.TimeoutWithConfig(&middleware.TimeoutConfig{
    Timeout: 30 * time.Second,
    OnTimeout: func(ctx *context.Context) {
        ctx.Logger().Warn("request timed out", "path", ctx.Request().Path)
    },
    Skipper: func(ctx *context.Context) bool {
        // Skip timeout for file uploads
        return strings.HasPrefix(ctx.Request().Path, "/upload")
    },
}))
```

### CORS

Cross-Origin Resource Sharing:

```go
// Default CORS (allows all origins)
app.Use(middleware.CORS())

// Restricted CORS
app.Use(middleware.CORSWithConfig(&middleware.CORSConfig{
    AllowOrigins:     []string{"https://example.com", "https://app.example.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Authorization", "Content-Type"},
    AllowCredentials: true,
    MaxAge:           86400, // 24 hours
}))

// Dynamic origin validation
app.Use(middleware.CORSWithConfig(&middleware.CORSConfig{
    AllowOriginFunc: func(origin string) bool {
        return strings.HasSuffix(origin, ".example.com")
    },
}))
```

### Compress

Response compression with Gzip and Brotli support:

```go
// Default compression (brotli preferred, falls back to gzip)
app.Use(middleware.Compress())

// With custom config
app.Use(middleware.CompressWithConfig(&middleware.CompressConfig{
    Level:     middleware.BestCompression,
    MinLength: 1024, // Only compress responses >= 1KB
    CompressionTypes: []string{
        "text/html",
        "text/css",
        "application/json",
        "application/javascript",
    },
    ExcludedExtensions: []string{
        ".jpg", ".png", ".gif", ".mp4", ".zip",
    },
    ExcludedPaths: []string{
        "/metrics", "/health",
    },
    EnableBrotli: true, // Prefer brotli over gzip
    Skipper: func(ctx *context.Context) bool {
        return ctx.Request().Path == "/stream"
    },
}))

// Compress only specific content types
app.Use(middleware.CompressWithTypes("application/json", "text/html"))

// Compress only responses above minimum size
app.Use(middleware.CompressWithMinLength(2048)) // 2KB
```

**Features:**
- Automatic algorithm selection (brotli > gzip)
- Smart compression (only compresses when beneficial)
- Content-type filtering
- File extension exclusion
- Path exclusion
- Configurable compression levels

### Logger

Request/Response logging with sensitive data masking:

```go
// Request/Response Logger (default variant)
app.Use(middleware.RequestResponseLogger(logger))

// Compact logger (one-line per request)
app.Use(middleware.CompactLogger(logger))

// Detailed logger (full request/response bodies)
app.Use(middleware.DetailedLogger(logger))

// Audit logger (captures everything for compliance)
app.Use(middleware.AuditLogger(logger))

// With custom config
app.Use(middleware.LoggerWithConfig(&middleware.LoggerConfig{
    Logger:      logger,
    LogRequest:  true,
    LogResponse: true,
    LogHeaders:  true,
    LogBody:     true,
    MaxBodySize: 4096, // Max 4KB logged
    SkipPaths:   []string{"/health", "/metrics"},
    Skipper: func(ctx *context.Context) bool {
        return strings.HasPrefix(ctx.Request().Path, "/internal")
    },
    SensitiveFields: []string{
        "password", "token", "api_key", "secret",
        "credit_card", "ssn", "authorization",
    },
    CustomFields: func(ctx *context.Context) map[string]interface{} {
        return map[string]interface{}{
            "tenant_id": ctx.Get("tenant_id"),
            "user_id":   ctx.Get("user_id"),
        }
    },
}))
```

**Features:**
- Auto-masking of 21 sensitive field patterns
- Multiple log formats (compact, detailed, audit)
- Request/Response body logging
- Configurable max body size
- Path exclusion
- Custom field extraction
- Performance metrics (latency, status code)

### CSRF Protection

Token-based CSRF protection:

```go
// Default CSRF protection
app.Use(middleware.CSRF())

// With custom config
app.Use(middleware.CSRFWithConfig(&middleware.CSRFConfig{
    TokenLength:   32,
    TokenLookup:   "header:X-CSRF-Token",  // Where to look for token
    CookieName:    "_csrf",
    CookiePath:    "/",
    CookieSecure:  true,
    CookieHTTPOnly: true,
    CookieSameSite: "Strict",
    Skipper: middleware.SkipMethods("GET", "HEAD", "OPTIONS"),
    ErrorHandler: func(ctx *context.Context, err error) error {
        return ctx.JSON(403, map[string]string{
            "error": "CSRF token validation failed",
        })
    },
}))

// Referer-based CSRF protection (for same-origin checks)
app.Use(middleware.CSRFFromReferer([]string{
    "https://example.com",
    "https://app.example.com",
}))
```

**Token Sources:**
- `header:X-CSRF-Token` - Header-based
- `form:csrf_token` - Form field
- `query:csrf_token` - Query parameter

**Features:**
- Secure token generation
- Constant-time validation (prevents timing attacks)
- Cookie-based token storage
- Multiple token sources
- Method-based skipping (skip GET/HEAD/OPTIONS)
- Path-based skipping

### Upload

File upload handling with validation:

```go
// Default upload (10MB max)
app.Use(middleware.Upload())

// With custom config
app.Use(middleware.UploadWithConfig(&middleware.UploadConfig{
    MaxSize: 50 * 1024 * 1024, // 50MB
    AllowedExtensions: []string{".jpg", ".png", ".pdf"},
    AllowedMimeTypes: []string{
        "image/jpeg",
        "image/png",
        "application/pdf",
    },
    FieldName: "file",
    Skipper: func(ctx *context.Context) bool {
        return ctx.Request().Path != "/upload"
    },
    OnUpload: func(ctx *context.Context, filename string, size int64) {
        ctx.Logger().Info("file uploaded",
            "filename", filename,
            "size", size,
        )
    },
}))

// Preset: Image uploads only
app.Use(middleware.UploadImage())

// Preset: Document uploads only
app.Use(middleware.UploadDocument())

// Multiple files upload
app.Use(middleware.UploadMultiple(5)) // Max 5 files
```

**Presets:**
- `UploadImage()` - JPG, PNG, GIF, WebP (10MB max)
- `UploadDocument()` - PDF, DOC, DOCX, XLS, XLSX (50MB max)

**Features:**
- File size validation
- Extension filtering
- MIME type validation
- Multiple file support
- Custom validation callbacks
- Progress logging

### Rate Limiting

Protect against abuse:

```go
// Basic rate limiting (100 requests per minute)
app.Use(middleware.RateLimit(100, time.Minute))

// Rate limit by IP
app.Use(middleware.RateLimitByIP(100, time.Minute))

// Rate limit by user ID
app.Use(middleware.RateLimitByUserID(1000, time.Hour, "user"))

// With Redis backend (distributed)
app.Use(middleware.RateLimitWithConfig(&middleware.RateLimitConfig{
    Limit:  100,
    Window: time.Minute,
    Store:  middleware.NewRedisRateLimitStore(cache, "ratelimit:"),
    KeyFunc: func(ctx *context.Context) string {
        // Custom key extraction
        if apiKey := ctx.Request().Header("X-API-Key"); apiKey != "" {
            return "apikey:" + apiKey
        }
        return "ip:" + ctx.Request().Header("X-Forwarded-For")
    },
    ErrorHandler: func(ctx *context.Context, retryAfter time.Duration) error {
        return ctx.JSON(429, map[string]interface{}{
            "error":       "Rate limit exceeded",
            "retry_after": retryAfter.Seconds(),
        })
    },
}))
```

## Authentication Middleware

### JWT Authentication

```go
// Basic JWT with secret key
app.Use(middleware.JWT([]byte("your-secret-key")))

// With custom validator
app.Use(middleware.JWTWithConfig(&middleware.JWTConfig{
    SigningKey: []byte("your-secret-key"),
    Validator: func(token string) (map[string]interface{}, error) {
        // Parse and validate JWT token
        claims, err := jwt.Parse(token, secretKey)
        if err != nil {
            return nil, middleware.ErrInvalidToken
        }
        return claims, nil
    },
    ContextKey: "user",
    Skipper: middleware.SkipPaths("/health", "/login", "/register"),
    ErrorHandler: func(ctx *context.Context, err error) error {
        if errors.Is(err, middleware.ErrTokenExpired) {
            return ctx.JSON(401, map[string]string{"error": "Token expired"})
        }
        return ctx.JSON(401, map[string]string{"error": "Unauthorized"})
    },
}))

// Access claims in handler
func handler(ctx *context.Context) error {
    claims, _ := ctx.Get("user")
    userClaims := claims.(map[string]interface{})
    userID := userClaims["sub"].(string)
    // ...
}
```

### API Key Authentication

```go
app.Use(middleware.APIKey(func(key string) (interface{}, error) {
    // Validate API key against database
    apiKey, err := db.FindAPIKey(key)
    if err != nil {
        return nil, middleware.ErrInvalidToken
    }
    return apiKey, nil
}))

// With custom config
app.Use(middleware.APIKeyWithConfig(&middleware.APIKeyConfig{
    KeyLookup:  "header:X-API-Key",  // or "query:api_key"
    ContextKey: "api_key",
    Validator: func(key string) (interface{}, error) {
        return validateAPIKey(key)
    },
}))
```

### Basic Authentication

```go
app.Use(middleware.BasicAuth(func(username, password string) (interface{}, error) {
    user, err := db.FindUser(username)
    if err != nil || !user.CheckPassword(password) {
        return nil, middleware.ErrUnauthorized
    }
    return user, nil
}))

// With custom realm
app.Use(middleware.BasicAuthWithConfig(&middleware.BasicAuthConfig{
    Realm: "Admin Area",
    Validator: validateCredentials,
}))
```

## Health Checks

Kubernetes-ready health endpoints:

```go
health := middleware.NewHealthHandler(&middleware.HealthConfig{
    Path:          "/health",
    LivenessPath:  "/health/live",
    ReadinessPath: "/health/ready",
    Timeout:       5 * time.Second,
    CacheDuration: 10 * time.Second, // Cache health results
})

// Add component checkers
health.AddChecker("database", middleware.DatabaseChecker(db))
health.AddChecker("cache", middleware.CacheChecker(cache))
health.AddChecker("external-api", middleware.URLChecker("https://api.example.com/health", 5*time.Second))

// Custom checker
health.AddChecker("queue", func(ctx context.Context) middleware.HealthCheckResult {
    if err := queue.Ping(ctx); err != nil {
        return middleware.HealthCheckResult{
            Status:  middleware.HealthStatusDown,
            Message: err.Error(),
        }
    }
    return middleware.HealthCheckResult{
        Status: middleware.HealthStatusUp,
    }
})

// Register endpoints
app.GET("/health", health.Handler())
app.GET("/health/live", health.LivenessHandler())
app.GET("/health/ready", health.ReadinessHandler())
```

Response format:

```json
{
  "status": "up",
  "timestamp": "2024-11-23T10:00:00Z",
  "components": {
    "database": {
      "status": "up",
      "duration_ms": 5
    },
    "cache": {
      "status": "up", 
      "duration_ms": 2
    }
  }
}
```

## Telemetry Middleware

### Distributed Tracing

```go
// With OpenTelemetry tracer
app.Use(middleware.Tracing(otelTracer))

// With custom config
app.Use(middleware.TracingWithConfig(&middleware.TelemetryConfig{
    Tracer:         otelTracer,
    ServiceName:    "my-service",
    ServiceVersion: "1.0.0",
    SkipPaths:      []string{"/health", "/metrics"},
    SpanNameFormatter: func(ctx *context.Context) string {
        return ctx.Request().Method + " " + ctx.Request().Path
    },
}))
```

### Metrics Collection

```go
// With meter provider
app.Use(middleware.Metrics(meterProvider))

// Collected metrics:
// - http_requests_total (counter)
// - http_request_duration_seconds (histogram)
// - http_request_size_bytes (histogram)
// - http_response_size_bytes (histogram)
// - http_requests_active (gauge)
```

## Middleware Chaining

```go
// Chain multiple middleware
app.Use(middleware.Chain(
    middleware.Recovery(),
    middleware.CORS(),
    middleware.Timeout(30 * time.Second),
    middleware.RateLimit(100, time.Minute),
))

// Conditional middleware
app.Use(middleware.ConditionalMiddleware(
    func(ctx *context.Context) bool {
        return ctx.Request().Path != "/public"
    },
    middleware.JWT(secretKey),
))

// Path-specific middleware
app.Use(middleware.PathMiddleware(
    []string{"/admin", "/admin/*"},
    middleware.BasicAuth(adminValidator),
))
```

## Skipping Middleware

Most middleware support a `Skipper` function:

```go
// Skip by path
Skipper: middleware.SkipPaths("/health", "/metrics", "/public/*")

// Skip by path prefix
Skipper: middleware.SkipPathPrefixes("/public/", "/static/")

// Custom skipper
Skipper: func(ctx *context.Context) bool {
    // Skip for internal requests
    return ctx.Request().Header("X-Internal") == "true"
}
```

## Creating Custom Middleware

```go
func MyMiddleware() context.MiddlewareFunc {
    return func(next context.HandlerFunc) context.HandlerFunc {
        return func(ctx *context.Context) error {
            // Before handler
            start := time.Now()
            
            // Call next middleware/handler
            err := next(ctx)
            
            // After handler
            duration := time.Since(start)
            ctx.Logger().Info("request processed",
                "duration", duration,
                "status", ctx.Response().StatusCode,
            )
            
            return err
        }
    }
}
```

## Best Practices

1. **Order matters**: Recovery should be first, auth before business logic
2. **Use skipper**: Skip middleware for health checks and public endpoints
3. **Configure timeouts**: Always set reasonable timeouts
4. **Rate limit strategically**: Different limits for different endpoints
5. **Cache health checks**: Reduce load on health endpoints
6. **Log panics**: Always configure panic logging in Recovery
