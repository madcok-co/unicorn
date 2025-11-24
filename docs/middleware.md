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
