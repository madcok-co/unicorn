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
| `Start()` | Start the application (all adapters + sidecars) |
| `RunServices(names ...string)` | Run specific services |
| `RunSidecars()` | Start only sidecars (no built-in adapters) |
| `AddSidecar(s Sidecar)` | Add a sidecar process |
| `Registry()` | Get the handler registry |
| `Adapters()` | Get the app adapters reference |
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
    TriggerType    string              // "http", "message", "cron"
    
    // Common fields
    Body           []byte
    Headers        map[string]string
    
    // HTTP specific
    Method         string
    Path           string
    Params         map[string]string   // Path parameters
    Query          map[string]string   // Query parameters
    Cookies        map[string]string
    
    // Message specific
    Topic          string
    Partition      int
    Offset         int64
    Key            []byte
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
    MaxBodySize     int64         // Max request body size (0 = no limit)
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
    // CRUD operations
    Create(ctx context.Context, entity any) error
    FindByID(ctx context.Context, id any, dest any) error
    FindOne(ctx context.Context, dest any, query string, args ...any) error
    FindAll(ctx context.Context, dest any, query string, args ...any) error
    Update(ctx context.Context, entity any) error
    Delete(ctx context.Context, entity any) error
    
    // Query builder (fluent API)
    Query() QueryBuilder
    
    // Transaction
    Transaction(ctx context.Context, fn func(tx Database) error) error
    
    // Raw query (escape hatch)
    Raw(ctx context.Context, query string, args ...any) (Result, error)
    Exec(ctx context.Context, query string, args ...any) (ExecResult, error)
    
    // Connection
    Ping(ctx context.Context) error
    Close() error
}

type QueryBuilder interface {
    Select(columns ...string) QueryBuilder
    From(table string) QueryBuilder
    Where(condition string, args ...any) QueryBuilder
    WhereIn(column string, values ...any) QueryBuilder
    OrderBy(column string, direction string) QueryBuilder
    Limit(limit int) QueryBuilder
    Offset(offset int) QueryBuilder
    Join(table string, condition string) QueryBuilder
    LeftJoin(table string, condition string) QueryBuilder
    GroupBy(columns ...string) QueryBuilder
    Having(condition string, args ...any) QueryBuilder
    Get(ctx context.Context, dest any) error
    First(ctx context.Context, dest any) error
    Count(ctx context.Context) (int64, error)
    Exists(ctx context.Context) (bool, error)
}
```

### Cache

```go
type Cache interface {
    // Basic operations
    Get(ctx context.Context, key string, dest any) error
    Set(ctx context.Context, key string, value any, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    
    // Multiple keys
    GetMany(ctx context.Context, keys []string) (map[string]any, error)
    SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error
    DeleteMany(ctx context.Context, keys ...string) error
    
    // Atomic operations
    Increment(ctx context.Context, key string, delta int64) (int64, error)
    Decrement(ctx context.Context, key string, delta int64) (int64, error)
    
    // TTL management
    Expire(ctx context.Context, key string, ttl time.Duration) error
    TTL(ctx context.Context, key string) (time.Duration, error)
    
    // Pattern operations
    Keys(ctx context.Context, pattern string) ([]string, error)
    Flush(ctx context.Context) error
    
    // Distributed lock
    Lock(ctx context.Context, key string, ttl time.Duration) (Lock, error)
    
    // Remember pattern - get from cache or compute
    Remember(ctx context.Context, key string, ttl time.Duration, fn func() (any, error), dest any) error
    
    // Tags for cache invalidation
    Tags(tags ...string) TaggedCache
    
    // Connection
    Ping(ctx context.Context) error
    Close() error
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
    // Publishing
    Publish(ctx context.Context, topic string, msg *BrokerMessage) error
    PublishBatch(ctx context.Context, topic string, msgs []*BrokerMessage) error
    
    // Subscribing
    Subscribe(ctx context.Context, topic string, handler MessageHandlerFunc) error
    SubscribeMultiple(ctx context.Context, topics []string, handler MessageHandlerFunc) error
    Unsubscribe(topic string) error
    
    // Consumer Group (load balancing across instances)
    ConsumeGroup(ctx context.Context, group string, topics []string, handler MessageHandlerFunc) error
    LeaveGroup(group string) error
    
    // Queue operations
    QueueLength(ctx context.Context, queue string) (int64, error)
    
    // Acknowledge messages (for explicit ack brokers)
    Ack(ctx context.Context, msg *BrokerMessage) error
    Nack(ctx context.Context, msg *BrokerMessage, requeue bool) error
    
    // Connection management
    Connect(ctx context.Context) error
    Disconnect(ctx context.Context) error
    Ping(ctx context.Context) error
    IsConnected() bool
    
    // Info
    Name() string
}

type MessageHandlerFunc func(ctx context.Context, msg *BrokerMessage) error
```

### Metrics

```go
type Metrics interface {
    Counter(name string, tags ...Tag) Counter
    Gauge(name string, tags ...Tag) Gauge
    Histogram(name string, tags ...Tag) Histogram
    Timer(name string, tags ...Tag) Timer
    WithTags(tags ...Tag) Metrics
    Handler() any
    Close() error
}

type Tag struct {
    Key   string
    Value string
}
```

### Tracer

```go
type Tracer interface {
    Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)
    Extract(ctx context.Context, carrier Carrier) context.Context
    Inject(ctx context.Context, carrier Carrier) error
    Close() error
}

type Span interface {
    End()
    SetName(name string)
    SetStatus(code SpanStatus, message string)
    SetAttributes(attrs ...Attribute)
    RecordError(err error)
    AddEvent(name string, attrs ...Attribute)
    SpanContext() SpanContext
    IsRecording() bool
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

### Security Context

```go
func GetIdentityFromContext(ctx context.Context) (*Identity, bool)
func SetIdentityInContext(ctx context.Context, identity *Identity) context.Context
```
