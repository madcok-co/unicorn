# API Reference

Complete API reference for the Unicorn framework.

## Core Types

### App

```go
// Create new application
func New(config *Config) *App

// Configuration
type Config struct {
    Name         string
    Version      string
    EnableHTTP   bool
    EnableBroker bool
    EnableGRPC   bool
    EnableCron   bool
    HTTP         *HTTPConfig
    Broker       *BrokerConfig
}
```

**App Methods:**

| Method | Description |
|--------|-------------|
| `SetDB(db Database)` | Set database adapter |
| `SetCache(cache Cache)` | Set cache adapter |
| `SetLogger(logger Logger)` | Set logger adapter |
| `SetBroker(broker Broker)` | Set message broker |
| `SetMetrics(metrics Metrics)` | Set metrics collector |
| `SetTracer(tracer Tracer)` | Set tracer |
| `RegisterHandler(fn HandlerFunc)` | Register a handler |
| `Service(name string)` | Get or create a service |
| `OnStart(fn func() error)` | Add startup hook |
| `OnStop(fn func() error)` | Add shutdown hook |
| `Start()` | Start the application |
| `RunServices(names ...string)` | Run specific services |
| `Shutdown()` | Graceful shutdown |

### Context

```go
type Context struct {
    // Methods available
}
```

**Context Methods:**

| Method | Return Type | Description |
|--------|-------------|-------------|
| `Context()` | `context.Context` | Get standard context |
| `Request()` | `*Request` | Get request info |
| `DB()` | `Database` | Get database |
| `Cache()` | `Cache` | Get cache |
| `Logger()` | `Logger` | Get logger |
| `Queue()` | `Queue` | Get queue (legacy) |
| `Broker()` | `Broker` | Get message broker |
| `Metrics()` | `Metrics` | Get metrics |
| `Tracer()` | `Tracer` | Get tracer |
| `Identity()` | `*Identity` | Get authenticated identity |
| `Set(key string, value any)` | - | Set metadata |
| `Get(key string)` | `(any, bool)` | Get metadata |

### Request

```go
type Request struct {
    ID             string              // Unique request ID
    TriggerType    string              // "http", "message", "cron"
    Timestamp      time.Time           // Request timestamp
    
    // HTTP specific
    Method         string
    Path           string
    Params         map[string]string   // Path parameters
    Query          map[string]string   // Query parameters
    Headers        map[string]string
    Body           []byte
    
    // Message specific
    MessageTopic   string
    MessageKey     []byte
    MessageHeaders map[string]string
}
```

### Handler

```go
// Create handler
func New(fn HandlerFunc) *Handler

// HandlerFunc signature
type HandlerFunc = any // func(ctx *Context, req T) (R, error)
```

**Handler Methods:**

| Method | Description |
|--------|-------------|
| `Named(name string)` | Set handler name |
| `Describe(desc string)` | Set description |
| `Use(middleware ...Middleware)` | Add middleware |
| `HTTP(method, path string)` | Add HTTP trigger |
| `Message(topic string, opts ...MessageOption)` | Add message trigger |
| `Cron(schedule string)` | Add cron trigger |
| `GRPC(service, method string)` | Add gRPC trigger |

### Service

```go
type Service struct {
    // Methods available
}
```

**Service Methods:**

| Method | Description |
|--------|-------------|
| `Name()` | Get service name |
| `Describe(desc string)` | Set description |
| `DependsOn(services ...string)` | Declare dependencies |
| `OnStart(fn func(ctx) error)` | Add startup hook |
| `OnStop(fn func(ctx) error)` | Add shutdown hook |
| `Register(fn HandlerFunc)` | Register handler |
| `Handlers()` | Get all handlers |

## HTTP Adapter

### HTTPConfig

```go
type HTTPConfig struct {
    Host            string        // Default: "0.0.0.0"
    Port            int           // Default: 8080
    ReadTimeout     time.Duration // Default: 30s
    WriteTimeout    time.Duration // Default: 30s
    IdleTimeout     time.Duration // Default: 60s
    MaxHeaderBytes  int           // Default: 1MB
    TLS             *TLSConfig
    CORS            *CORSConfig
}
```

### HTTPError

```go
type HTTPError struct {
    StatusCode int
    Message    string  // Exposed to client
    Internal   error   // Logged but not exposed
}
```

## Broker Adapter

### BrokerConfig

```go
type BrokerConfig struct {
    ConsumerGroup  string
    MaxRetries     int           // Default: 3
    RetryDelay     time.Duration // Default: 1s
    MaxConcurrency int           // Default: 10
}
```

### BrokerMessage

```go
type BrokerMessage struct {
    Key       []byte
    Body      []byte
    Headers   map[string]string
    Timestamp time.Time
    Topic     string
    Partition int
    Offset    int64
}
```

## Security

### Authenticator

```go
type Authenticator interface {
    Authenticate(ctx context.Context, creds Credentials) (*Identity, error)
    Validate(ctx context.Context, token string) (*Identity, error)
    Refresh(ctx context.Context, refreshToken string) (*TokenPair, error)
    Revoke(ctx context.Context, token string) error
}
```

### JWTAuthenticator

```go
// Constructor
func NewJWTAuthenticator(config JWTConfig) *JWTAuthenticator

type JWTConfig struct {
    SecretKey          []byte        // Required, min 32 bytes
    Issuer             string
    Audience           []string
    AccessTokenExpiry  time.Duration // Default: 15m
    RefreshTokenExpiry time.Duration // Default: 7d
    SigningMethod      string        // Default: "HS256"
}
```

### APIKeyAuthenticator

```go
// Constructor
func NewAPIKeyAuthenticator(config APIKeyConfig) *APIKeyAuthenticator

type APIKeyConfig struct {
    Store      APIKeyStore
    HeaderName string // Default: "X-API-Key"
    QueryParam string // Optional
}

type APIKeyStore interface {
    Get(apiKey string) (*Identity, error)
    Add(apiKey string, identity *Identity) error
    Remove(apiKey string) error
    List() ([]string, error)
}
```

### Identity

```go
type Identity struct {
    ID        string
    Type      string   // "user", "service", "api_key"
    Name      string
    Email     string
    Roles     []string
    Scopes    []string
    Metadata  map[string]any
    TokenID   string
    ExpiresAt time.Time
    IssuedAt  time.Time
}

// Methods
func (i *Identity) HasRole(role string) bool
func (i *Identity) HasScope(scope string) bool
func (i *Identity) HasAnyRole(roles ...string) bool
func (i *Identity) HasAllScopes(scopes ...string) bool
```

### RateLimiter

```go
type RateLimiter interface {
    Allow(ctx context.Context, key string) (bool, error)
    AllowN(ctx context.Context, key string, n int) (bool, error)
    Remaining(ctx context.Context, key string) (int, error)
    Reset(ctx context.Context, key string) error
}
```

### InMemoryRateLimiter

```go
// Token Bucket
func NewInMemoryRateLimiter(config InMemoryRateLimiterConfig) *InMemoryRateLimiter

// Sliding Window
func NewSlidingWindowRateLimiter(config InMemoryRateLimiterConfig) *SlidingWindowRateLimiter

type InMemoryRateLimiterConfig struct {
    MaxTokens       int           // Max tokens/requests
    RefillRate      int           // Tokens per interval
    RefillInterval  time.Duration // Interval duration
    CleanupInterval time.Duration // Cleanup expired buckets
}
```

### Encryptor

```go
type Encryptor interface {
    Encrypt(plaintext []byte) ([]byte, error)
    Decrypt(ciphertext []byte) ([]byte, error)
    EncryptString(plaintext string) (string, error)
    DecryptString(ciphertext string) (string, error)
    Hash(data []byte) string
    CompareHash(data []byte, hash string) bool
}
```

### AESEncryptor

```go
func NewAESEncryptor(config AESEncryptorConfig) (*AESEncryptor, error)
func NewAESEncryptorFromString(base64Key string) (*AESEncryptor, error)

type AESEncryptorConfig struct {
    Key     []byte // 16, 24, or 32 bytes
    Mode    string // "GCM" (default) or "CBC"
    HMACKey []byte // Required for CBC mode
}
```

### Password Hasher

```go
type PasswordHasher interface {
    Hash(password []byte) ([]byte, error)
    Verify(password, hash []byte) bool
}
```

### BcryptHasher

```go
func NewBcryptHasher(config BcryptConfig) *BcryptHasher

type BcryptConfig struct {
    Cost int // Default: 12, Range: 4-31
}
```

### Argon2Hasher

```go
func NewArgon2Hasher(config Argon2Config) *Argon2Hasher
func DefaultArgon2Config() Argon2Config
func LowMemoryArgon2Config() Argon2Config

type Argon2Config struct {
    Time    uint32 // Iterations
    Memory  uint32 // Memory in KB
    Threads uint8  // Parallelism
    KeyLen  uint32 // Output length
}
```

### SecretManager

```go
type SecretManager interface {
    Get(ctx context.Context, key string) (string, error)
    GetJSON(ctx context.Context, key string, dest any) error
    Set(ctx context.Context, key, value string) error
    Delete(ctx context.Context, key string) error
    List(ctx context.Context, prefix string) ([]string, error)
    Watch(ctx context.Context, key string, callback func(string)) error
}
```

### EnvSecretManager

```go
func NewEnvSecretManager(config EnvSecretManagerConfig) *EnvSecretManager

type EnvSecretManagerConfig struct {
    Prefix       string        // Env variable prefix
    AllowMissing bool          // Don't error on missing
    CacheTTL     time.Duration // Cache duration
}
```

### AuditLogger

```go
type AuditLogger interface {
    Log(ctx context.Context, event *AuditEvent) error
    Query(ctx context.Context, filter *AuditFilter) ([]*AuditEvent, error)
}
```

### InMemoryAuditLogger

```go
func NewInMemoryAuditLogger(config InMemoryAuditLoggerConfig) *InMemoryAuditLogger

type InMemoryAuditLoggerConfig struct {
    MaxEvents       int           // Max events to store
    BufferSize      int           // Channel buffer
    CleanupInterval time.Duration // Cleanup frequency
    RetentionPeriod time.Duration // Event retention
}

// Methods
func (l *InMemoryAuditLogger) Close() error
```

### AuditEvent

```go
type AuditEvent struct {
    ID         string
    Timestamp  time.Time
    Action     string    // "create", "read", "update", "delete", "login", "logout"
    Resource   string
    ResourceID string
    ActorID    string
    ActorType  string
    ActorName  string
    ActorIP    string
    Method     string
    Path       string
    UserAgent  string
    Success    bool
    Error      string
    OldValue   any
    NewValue   any
    Metadata   map[string]any
}
```

### AuditEventBuilder

```go
func NewAuditEvent() *AuditEventBuilder

// Builder methods (chainable)
func (b *AuditEventBuilder) Action(action string) *AuditEventBuilder
func (b *AuditEventBuilder) Resource(resource string) *AuditEventBuilder
func (b *AuditEventBuilder) ResourceID(id string) *AuditEventBuilder
func (b *AuditEventBuilder) Actor(id, typ, name string) *AuditEventBuilder
func (b *AuditEventBuilder) ActorIP(ip string) *AuditEventBuilder
func (b *AuditEventBuilder) Success(success bool) *AuditEventBuilder
func (b *AuditEventBuilder) WithError(err error) *AuditEventBuilder
func (b *AuditEventBuilder) WithMetadata(key string, value any) *AuditEventBuilder
func (b *AuditEventBuilder) Build() *AuditEvent
```

## Infrastructure Contracts

### Database

```go
type Database interface {
    Name() string
    Type() string
    Initialize(config any) error
    Health() error
    Close() error
    
    // CRUD operations
    Create(value any) error
    FindByID(ctx context.Context, id string, dest any) error
    Update(value any) error
    Delete(value any) error
    
    // Query
    Query(ctx context.Context, query string, args ...any) (Rows, error)
    Exec(ctx context.Context, query string, args ...any) (Result, error)
}
```

### Cache

```go
type Cache interface {
    Name() string
    Type() string
    Initialize(config any) error
    Health() error
    Close() error
    
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value any, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
}
```

### Logger

```go
type Logger interface {
    Debug(msg string, keysAndValues ...any)
    Info(msg string, keysAndValues ...any)
    Warn(msg string, keysAndValues ...any)
    Error(msg string, keysAndValues ...any)
    With(keysAndValues ...any) Logger
    Sync() error
}
```

### Broker

```go
type Broker interface {
    Name() string
    Connect(ctx context.Context) error
    Disconnect(ctx context.Context) error
    Health() error
    
    Publish(ctx context.Context, topic string, msg *BrokerMessage) error
    Subscribe(ctx context.Context, topic string, handler MessageHandlerFunc) error
    Unsubscribe(topic string) error
}

type MessageHandlerFunc func(ctx context.Context, msg *BrokerMessage) error
```

### Metrics

```go
type Metrics interface {
    Counter(name string, labels ...string) Counter
    Gauge(name string, labels ...string) Gauge
    Histogram(name string, buckets []float64, labels ...string) Histogram
    Summary(name string, objectives map[float64]float64, labels ...string) Summary
}
```

### Tracer

```go
type Tracer interface {
    StartSpan(name string, opts ...SpanOption) Span
    Extract(carrier any) SpanContext
    Inject(ctx SpanContext, carrier any)
}

type Span interface {
    End()
    SetAttribute(key string, value any)
    RecordError(err error)
    AddEvent(name string, attrs ...any)
    Context() SpanContext
}
```

## Constants

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

### Trigger Types

```go
const (
    TriggerHTTP    TriggerType = "http"
    TriggerMessage TriggerType = "message"
    TriggerKafka   TriggerType = "kafka"  // Legacy
    TriggerGRPC    TriggerType = "grpc"
    TriggerCron    TriggerType = "cron"
    TriggerPubSub  TriggerType = "pubsub" // Legacy
)
```

## Functions

### Version

```go
func Version() string // Returns "0.1.0"
```

### Security Context

```go
func GetIdentityFromContext(ctx context.Context) (*Identity, bool)
func SetIdentityInContext(ctx context.Context, identity *Identity) context.Context
```
