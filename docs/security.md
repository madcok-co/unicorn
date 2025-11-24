# Security

This document covers all security features in the Unicorn framework.

## Overview

Unicorn provides a comprehensive security layer:

| Feature | Description | Adapters |
|---------|-------------|----------|
| Authentication | Verify identity | JWT, API Key |
| Authorization | Permission checking | Role-based, Scope-based |
| Rate Limiting | Prevent abuse | Token Bucket, Sliding Window |
| Encryption | Data protection | AES-GCM, AES-CBC+HMAC |
| Password Hashing | Secure password storage | Bcrypt, Argon2id |
| Secret Management | Secure configuration | Environment variables |
| Audit Logging | Security audit trail | In-memory, File, Database |

## Authentication

### JWT Authentication

```go
import "github.com/madcok-co/unicorn/core"

// Create JWT authenticator
jwtAuth := unicorn.NewJWTAuthenticator(unicorn.JWTConfig{
    SecretKey:          []byte("your-256-bit-secret"),
    Issuer:             "my-app",
    Audience:           []string{"my-app-users"},
    AccessTokenExpiry:  15 * time.Minute,
    RefreshTokenExpiry: 7 * 24 * time.Hour,
})

// Issue tokens
tokenPair, err := jwtAuth.Authenticate(ctx, unicorn.Credentials{
    Type:     "jwt",
    Username: "user@example.com",
    Password: "password123",
    Metadata: map[string]string{
        "user_id": "user-123",
        "role":    "admin",
    },
})
// tokenPair.AccessToken, tokenPair.RefreshToken

// Validate token
identity, err := jwtAuth.Validate(ctx, accessToken)
// identity.ID, identity.Roles, identity.Scopes

// Refresh token
newTokenPair, err := jwtAuth.Refresh(ctx, refreshToken)

// Revoke token
err = jwtAuth.Revoke(ctx, accessToken)
```

**JWT Configuration Options:**

```go
type JWTConfig struct {
    // Required
    SecretKey []byte // Must be at least 32 bytes for HS256

    // Token settings
    Issuer             string
    Audience           []string
    AccessTokenExpiry  time.Duration // Default: 15 minutes
    RefreshTokenExpiry time.Duration // Default: 7 days

    // Algorithm (default: HS256)
    SigningMethod string // "HS256", "HS384", "HS512"
}
```

### API Key Authentication

```go
// Create API key store
store := unicorn.NewInMemoryAPIKeyStore()

// Add API keys
store.Add("api-key-1", &unicorn.Identity{
    ID:     "service-1",
    Type:   "service",
    Name:   "Payment Service",
    Roles:  []string{"service"},
    Scopes: []string{"payments:read", "payments:write"},
})

// Create authenticator
apiKeyAuth := unicorn.NewAPIKeyAuthenticator(unicorn.APIKeyConfig{
    Store:      store,
    HeaderName: "X-API-Key", // or "Authorization"
    QueryParam: "api_key",   // optional query parameter
})

// Validate API key
identity, err := apiKeyAuth.Validate(ctx, "api-key-1")
```

**Custom API Key Store:**

```go
// Implement the APIKeyStore interface
type APIKeyStore interface {
    Get(apiKey string) (*Identity, error)
    Add(apiKey string, identity *Identity) error
    Remove(apiKey string) error
    List() ([]string, error)
}

// Example: Database-backed store
type DatabaseAPIKeyStore struct {
    db *sql.DB
}

func (s *DatabaseAPIKeyStore) Get(apiKey string) (*Identity, error) {
    // Query database for API key
}
```

## Rate Limiting

### Token Bucket Rate Limiter

Classic token bucket algorithm:

```go
rateLimiter := unicorn.NewInMemoryRateLimiter(unicorn.InMemoryRateLimiterConfig{
    MaxTokens:       100,              // Maximum tokens
    RefillRate:      10,               // Tokens added per interval
    RefillInterval:  time.Second,      // Refill every second
    CleanupInterval: 5 * time.Minute,  // Cleanup expired buckets
})

// Check if request is allowed
allowed, err := rateLimiter.Allow(ctx, "user:123")
if !allowed {
    return errors.New("rate limit exceeded")
}

// Allow N requests at once
allowed, err = rateLimiter.AllowN(ctx, "user:123", 5)

// Check remaining tokens
remaining, err := rateLimiter.Remaining(ctx, "user:123")

// Reset limit for a key
err = rateLimiter.Reset(ctx, "user:123")
```

### Sliding Window Rate Limiter

More accurate rate limiting:

```go
rateLimiter := unicorn.NewSlidingWindowRateLimiter(unicorn.InMemoryRateLimiterConfig{
    MaxTokens:       100,              // Max requests per window
    RefillInterval:  time.Minute,      // Window duration
    CleanupInterval: 5 * time.Minute,
})
```

### Rate Limiting Strategies

| Strategy | Use Case | Description |
|----------|----------|-------------|
| Token Bucket | General API | Allows bursts, smooth over time |
| Sliding Window | Strict limits | Accurate per-window counting |

## Encryption

### AES-GCM Encryption (Recommended)

Authenticated encryption with associated data:

```go
// Create from 32-byte key
key := make([]byte, 32)
crypto_rand.Read(key)
encryptor, err := unicorn.NewAESEncryptor(unicorn.AESEncryptorConfig{
    Key:  key,
    Mode: "GCM", // Default
})

// Or from base64 string
encryptor, err := unicorn.NewAESEncryptorFromString("base64-encoded-32-byte-key")

// Encrypt
ciphertext, err := encryptor.Encrypt([]byte("sensitive data"))

// Decrypt
plaintext, err := encryptor.Decrypt(ciphertext)

// String helpers
encrypted, err := encryptor.EncryptString("my secret")
decrypted, err := encryptor.DecryptString(encrypted)
```

### AES-CBC+HMAC Encryption

For compatibility with legacy systems:

```go
encryptor, err := unicorn.NewAESEncryptor(unicorn.AESEncryptorConfig{
    Key:     key,
    Mode:    "CBC",
    HMACKey: hmacKey, // 32-byte HMAC key for authentication
})
```

**Security Features:**

- Random IV for each encryption
- Constant-time comparison to prevent timing attacks
- HMAC authentication for CBC mode (Encrypt-then-MAC)

## Password Hashing

### Bcrypt (Recommended for Most Cases)

```go
hasher := unicorn.NewBcryptHasher(unicorn.BcryptConfig{
    Cost: 12, // Higher = more secure but slower
})

// Hash password
hash, err := hasher.Hash([]byte("user-password"))

// Verify password
valid := hasher.Verify([]byte("user-password"), hash)
```

### Argon2id (Maximum Security)

```go
// Default config
hasher := unicorn.NewArgon2Hasher(unicorn.DefaultArgon2Config())

// High security config
hasher := unicorn.NewArgon2Hasher(unicorn.Argon2Config{
    Time:    3,            // Iterations
    Memory:  64 * 1024,    // 64 MB
    Threads: 4,            // Parallelism
    KeyLen:  32,           // Output length
})

// Low memory environments
hasher := unicorn.NewArgon2Hasher(unicorn.LowMemoryArgon2Config())
```

### Multi-Hasher (Migration Support)

Support multiple hash algorithms during migration:

```go
// Create multi-hasher with Argon2 as primary, Bcrypt for legacy
multiHasher := unicorn.NewMultiHasher(
    argon2Hasher, // Primary for new passwords
    bcryptHasher, // Legacy support
)

// Hash always uses primary (Argon2)
hash, _ := multiHasher.Hash([]byte("password"))

// Verify tries all hashers
valid := multiHasher.Verify([]byte("password"), legacyBcryptHash)
```

## Secret Management

### Environment Variables

```go
secretManager := unicorn.NewEnvSecretManager(unicorn.EnvSecretManagerConfig{
    Prefix:       "APP_",           // Look for APP_* variables
    AllowMissing: false,            // Error on missing secrets
    CacheTTL:     5 * time.Minute,  // Cache secrets
})

// Get secret
dbPassword, err := secretManager.Get(ctx, "DB_PASSWORD")
// Looks for APP_DB_PASSWORD

// Get JSON secret
var config DatabaseConfig
err = secretManager.GetJSON(ctx, "DB_CONFIG", &config)

// List secrets
keys, err := secretManager.List(ctx, "DB_")
```

## Audit Logging

### In-Memory Audit Logger

```go
auditLogger := unicorn.NewInMemoryAuditLogger(unicorn.InMemoryAuditLoggerConfig{
    MaxEvents:       10000,           // Max events to store
    BufferSize:      1000,            // Channel buffer size
    CleanupInterval: time.Hour,       // Cleanup old events
    RetentionPeriod: 30 * 24 * time.Hour, // Keep 30 days
})
defer auditLogger.Close()

// Log event using builder
event := unicorn.NewAuditEvent().
    Action(unicorn.AuditActionCreate).
    Resource("users").
    ResourceID("user-123").
    Actor("admin-1", "user", "Admin User").
    ActorIP("192.168.1.1").
    Success(true).
    WithMetadata("email", "user@example.com").
    Build()

err := auditLogger.Log(ctx, event)

// Query events
events, err := auditLogger.Query(ctx, &unicorn.AuditFilter{
    Resource:  "users",
    Action:    "create",
    StartTime: time.Now().Add(-24 * time.Hour),
    Limit:     100,
})
```

### Audit Actions

```go
const (
    AuditActionCreate = "create"
    AuditActionRead   = "read"
    AuditActionUpdate = "update"
    AuditActionDelete = "delete"
    AuditActionLogin  = "login"
    AuditActionLogout = "logout"
)
```

### Composite Audit Logger

Log to multiple destinations:

```go
compositeLogger := unicorn.NewCompositeAuditLogger(
    memoryLogger,
    fileLogger,
    databaseLogger,
)
```

## Middleware Integration

### Authentication Middleware

```go
import "github.com/madcok-co/unicorn/pkg/middleware"

// Create auth middleware
authMiddleware := middleware.NewAuthMiddleware(jwtAuth)

// Apply to handler
app.RegisterHandler(SecureHandler).
    Use(authMiddleware).
    HTTP("GET", "/secure").
    Done()
```

### Rate Limit Middleware

```go
rateLimitMiddleware := middleware.NewRateLimitMiddleware(rateLimiter, 
    middleware.RateLimitConfig{
        KeyFunc: func(ctx *unicorn.Context) string {
            // Rate limit by user ID or IP
            if identity := ctx.Identity(); identity != nil {
                return "user:" + identity.ID
            }
            return "ip:" + ctx.Request().Headers["X-Forwarded-For"]
        },
        ExceededHandler: func(ctx *unicorn.Context) error {
            return &http.HTTPError{
                StatusCode: 429,
                Message:    "Too many requests",
            }
        },
    },
)
```

### Combined Security Middleware

```go
app.RegisterHandler(AdminAction).
    Use(
        authMiddleware,           // Authenticate
        roleMiddleware("admin"),  // Authorize
        rateLimitMiddleware,      // Rate limit
        auditMiddleware,          // Audit log
    ).
    HTTP("POST", "/admin/action").
    Done()
```

## Security Best Practices

### 1. Use Strong Secrets

```go
// Generate secure random key
key := make([]byte, 32)
if _, err := crypto_rand.Read(key); err != nil {
    panic(err)
}

// Never hardcode secrets
jwtSecret := os.Getenv("JWT_SECRET")
```

### 2. Validate All Input

```go
type CreateUserRequest struct {
    Email    string `validate:"required,email,max=255"`
    Password string `validate:"required,min=8,max=72"` // bcrypt limit
    Name     string `validate:"required,min=2,max=100"`
}
```

### 3. Use HTTPS in Production

```go
app := unicorn.New(&unicorn.Config{
    HTTP: &unicorn.HTTPConfig{
        TLS: &unicorn.TLSConfig{
            Enabled:  true,
            CertFile: "/path/to/cert.pem",
            KeyFile:  "/path/to/key.pem",
            MinVersion: tls.VersionTLS12,
        },
    },
})
```

### 4. Implement Proper Error Handling

```go
// Don't expose internal errors
func GetUser(ctx *unicorn.Context) (*User, error) {
    user, err := db.Find(userID)
    if err != nil {
        // Log internal error
        ctx.Logger().Error("database error", "error", err)
        
        // Return generic message
        return nil, &http.HTTPError{
            StatusCode: 500,
            Message:    "Internal server error",
            Internal:   err, // Logged but not exposed
        }
    }
    return user, nil
}
```

### 5. Regular Token Rotation

```go
// Short-lived access tokens
jwtAuth := unicorn.NewJWTAuthenticator(unicorn.JWTConfig{
    AccessTokenExpiry:  15 * time.Minute,  // Short
    RefreshTokenExpiry: 7 * 24 * time.Hour, // Longer
})
```

### 6. Audit Sensitive Operations

```go
func DeleteUser(ctx *unicorn.Context) error {
    userID := ctx.Request().Params["id"]
    identity := ctx.Identity()
    
    // Audit before action
    auditLogger.Log(ctx, unicorn.NewAuditEvent().
        Action(unicorn.AuditActionDelete).
        Resource("users").
        ResourceID(userID).
        Actor(identity.ID, identity.Type, identity.Name).
        Build(),
    )
    
    return db.DeleteUser(userID)
}
```

## Next Steps

- [Observability](./observability.md) - Metrics, tracing, logging
- [API Reference](./api-reference.md) - Complete API documentation
- [Best Practices](./best-practices.md) - Production recommendations
