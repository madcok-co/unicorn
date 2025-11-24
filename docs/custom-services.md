# Custom Services

This document covers how to inject and use custom interfaces/services in the Unicorn framework.

## Overview

Unicorn allows you to register custom services (interfaces) that are automatically injected into every request context. This enables clean dependency injection without global variables.

## Basic Usage

### 1. Define Your Interface

```go
// services/email.go
type EmailService interface {
    Send(to, subject, body string) error
    SendTemplate(to, template string, data map[string]any) error
}

type smtpEmailService struct {
    host     string
    port     int
    username string
    password string
}

func NewSMTPEmailService(host string, port int, username, password string) EmailService {
    return &smtpEmailService{
        host:     host,
        port:     port,
        username: username,
        password: password,
    }
}

func (s *smtpEmailService) Send(to, subject, body string) error {
    // SMTP implementation
    return nil
}

func (s *smtpEmailService) SendTemplate(to, template string, data map[string]any) error {
    // Template implementation
    return nil
}
```

### 2. Register the Service

```go
func main() {
    app := unicorn.New(&unicorn.Config{
        Name:       "my-app",
        EnableHTTP: true,
    })

    // Create service instance
    emailSvc := NewSMTPEmailService(
        os.Getenv("SMTP_HOST"),
        587,
        os.Getenv("SMTP_USER"),
        os.Getenv("SMTP_PASS"),
    )

    // Register as singleton (same instance for all requests)
    app.RegisterService("email", emailSvc)

    // Register handlers...
    app.Start()
}
```

### 3. Use in Handlers

```go
func CreateUser(ctx *unicorn.Context, req CreateUserRequest) (*User, error) {
    // Method 1: Type assertion
    emailSvc := ctx.GetService("email").(EmailService)
    
    // Method 2: Type-safe generic helper (recommended)
    emailSvc, err := unicorn.GetService[EmailService](ctx, "email")
    if err != nil {
        return nil, err
    }
    
    // Method 3: Panic if not found (use in tests or when guaranteed)
    emailSvc = unicorn.MustGetService[EmailService](ctx, "email")
    
    // Use the service
    user := &User{Name: req.Name, Email: req.Email}
    
    err = emailSvc.Send(user.Email, "Welcome!", "Thanks for signing up!")
    if err != nil {
        return nil, fmt.Errorf("failed to send welcome email: %w", err)
    }
    
    return user, nil
}
```

## Service Registration Options

### Singleton Services

Same instance shared across all requests:

```go
// Create once
paymentGateway := stripe.NewClient(os.Getenv("STRIPE_KEY"))

// Register as singleton
app.RegisterService("payment", paymentGateway)
```

### Factory-Based Services

New instance created for each request:

```go
// Register factory - called for every request
app.RegisterServiceFactory("requestLogger", func(ctx *unicorn.Context) (any, error) {
    return NewRequestLogger(
        ctx.Request().ID,
        ctx.Request().Path,
    ), nil
})
```

Use factory for:
- Request-scoped services
- Services that need request context
- Stateful per-request tracking

## Type-Safe Service Retrieval

### Using Generics (Go 1.18+)

```go
// Returns (T, error)
emailSvc, err := unicorn.GetService[EmailService](ctx, "email")
if err != nil {
    return nil, err
}

// Panics if not found or wrong type
emailSvc := unicorn.MustGetService[EmailService](ctx, "email")
```

### Using Type Assertion

```go
// Returns any (nil if not found)
svc := ctx.GetService("email")
if svc == nil {
    return nil, errors.New("email service not configured")
}

emailSvc, ok := svc.(EmailService)
if !ok {
    return nil, errors.New("invalid email service type")
}
```

## Service Lifecycle

### Boot (ServiceProvider)

Services can implement `ServiceProvider` to receive boot callbacks:

```go
type ServiceProvider interface {
    Boot() error
}

type myService struct{}

func (s *myService) Boot() error {
    // Initialize connections, load config, etc.
    return nil
}
```

### Close (ClosableService)

Services can implement `ClosableService` for cleanup:

```go
type ClosableService interface {
    Close() error
}

type dbPool struct {
    conn *sql.DB
}

func (p *dbPool) Close() error {
    return p.conn.Close()
}
```

Closable services are automatically closed during app shutdown.

### Health Check (HealthCheckable)

Services can implement `HealthCheckable`:

```go
type HealthCheckable interface {
    Health() error
}

type redisCache struct {
    client *redis.Client
}

func (c *redisCache) Health() error {
    return c.client.Ping(context.Background()).Err()
}
```

Check all services health:

```go
results := app.CustomServices().HealthCheckAll()
for name, err := range results {
    if err != nil {
        log.Printf("Service %s unhealthy: %v", name, err)
    }
}
```

## Common Patterns

### Repository Pattern

```go
// Define repository interface
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    FindByID(ctx context.Context, id string) (*User, error)
    FindByEmail(ctx context.Context, email string) (*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
}

// Implement with database
type postgresUserRepo struct {
    db *sql.DB
}

// Register
app.RegisterService("userRepo", NewPostgresUserRepo(db))

// Use in handler
func GetUser(ctx *unicorn.Context) (*User, error) {
    repo := unicorn.MustGetService[UserRepository](ctx, "userRepo")
    id := ctx.Request().Params["id"]
    return repo.FindByID(ctx.Context(), id)
}
```

### Service Layer Pattern

```go
// User service with dependencies
type UserService interface {
    Register(ctx context.Context, req RegisterRequest) (*User, error)
    Login(ctx context.Context, req LoginRequest) (*TokenPair, error)
}

type userServiceImpl struct {
    repo    UserRepository
    auth    unicorn.Authenticator
    email   EmailService
    hasher  unicorn.PasswordHasher
}

func NewUserService(repo UserRepository, auth unicorn.Authenticator, 
                    email EmailService, hasher unicorn.PasswordHasher) UserService {
    return &userServiceImpl{
        repo:   repo,
        auth:   auth,
        email:  email,
        hasher: hasher,
    }
}

// Register with all dependencies
app.RegisterService("userService", NewUserService(
    userRepo,
    jwtAuth,
    emailSvc,
    bcryptHasher,
))

// Handler is thin, delegates to service
func Register(ctx *unicorn.Context, req RegisterRequest) (*User, error) {
    svc := unicorn.MustGetService[UserService](ctx, "userService")
    return svc.Register(ctx.Context(), req)
}
```

### External API Clients

```go
// Third-party API client
type PaymentGateway interface {
    Charge(amount int64, currency, token string) (*Charge, error)
    Refund(chargeID string) (*Refund, error)
}

type stripeGateway struct {
    client *stripe.Client
}

// Register
app.RegisterService("payment", NewStripeGateway(os.Getenv("STRIPE_KEY")))

// Use
func ProcessPayment(ctx *unicorn.Context, req PaymentRequest) (*Order, error) {
    gateway := unicorn.MustGetService[PaymentGateway](ctx, "payment")
    
    charge, err := gateway.Charge(req.Amount, "usd", req.Token)
    if err != nil {
        return nil, err
    }
    
    // Create order with charge info...
}
```

### Notification Services

```go
type NotificationService interface {
    SendEmail(to, subject, body string) error
    SendSMS(phone, message string) error
    SendPush(userID, title, body string) error
}

type multiNotifier struct {
    email EmailService
    sms   SMSService
    push  PushService
}

func (n *multiNotifier) SendEmail(to, subject, body string) error {
    return n.email.Send(to, subject, body)
}

// etc...

app.RegisterService("notifier", &multiNotifier{
    email: emailSvc,
    sms:   twilioSvc,
    push:  firebaseSvc,
})
```

## Testing

### Mock Services

```go
type mockEmailService struct {
    sentEmails []SentEmail
}

func (m *mockEmailService) Send(to, subject, body string) error {
    m.sentEmails = append(m.sentEmails, SentEmail{to, subject, body})
    return nil
}

func TestCreateUser(t *testing.T) {
    // Create context with mock
    ctx := unicorn.NewTestContext()
    mockEmail := &mockEmailService{}
    ctx.RegisterService("email", mockEmail)
    
    // Call handler
    req := CreateUserRequest{Name: "Test", Email: "test@example.com"}
    user, err := CreateUser(ctx, req)
    
    // Assert
    assert.NoError(t, err)
    assert.Len(t, mockEmail.sentEmails, 1)
    assert.Equal(t, "test@example.com", mockEmail.sentEmails[0].To)
}
```

### Test Helpers

```go
// Helper to create test context with common mocks
func NewTestContext() *unicorn.Context {
    ctx := unicorn.New(context.Background())
    ctx.RegisterService("email", &mockEmailService{})
    ctx.RegisterService("payment", &mockPaymentGateway{})
    ctx.RegisterService("userRepo", &mockUserRepo{})
    return ctx
}
```

## Best Practices

### 1. Use Interfaces

```go
// Good: Interface
app.RegisterService("email", emailSvc) // EmailService interface

// Avoid: Concrete type
app.RegisterService("email", &SMTPEmailService{}) // Concrete
```

### 2. Check Service Availability

```go
// For optional services
if ctx.HasService("analytics") {
    analytics := unicorn.MustGetService[AnalyticsService](ctx, "analytics")
    analytics.Track("user_created", user.ID)
}
```

### 3. Document Required Services

```go
// CreateUser requires: email, userRepo
func CreateUser(ctx *unicorn.Context, req CreateUserRequest) (*User, error) {
    email := unicorn.MustGetService[EmailService](ctx, "email")
    repo := unicorn.MustGetService[UserRepository](ctx, "userRepo")
    // ...
}
```

### 4. Use Meaningful Names

```go
// Good
app.RegisterService("userRepository", userRepo)
app.RegisterService("paymentGateway", stripe)
app.RegisterService("emailSender", ses)

// Avoid
app.RegisterService("repo", userRepo)
app.RegisterService("p", stripe)
app.RegisterService("e", ses)
```

### 5. Handle Initialization Errors

```go
// Create with error handling
emailSvc, err := NewSMTPEmailService(config)
if err != nil {
    log.Fatalf("Failed to initialize email service: %v", err)
}

app.RegisterService("email", emailSvc)
```

## API Reference

### Context Methods

| Method | Description |
|--------|-------------|
| `RegisterService(name, svc)` | Register a service |
| `GetService(name)` | Get service (nil if not found) |
| `MustGetService(name)` | Get service (panics if not found) |
| `HasService(name)` | Check if service exists |
| `Services()` | List all service names |
| `CopyServicesFrom(ctx)` | Copy services from another context |

### App Methods

| Method | Description |
|--------|-------------|
| `RegisterService(name, svc)` | Register singleton service |
| `RegisterServiceFactory(name, fn)` | Register factory service |
| `CustomServices()` | Get service registry |

### Generic Helpers

| Function | Description |
|----------|-------------|
| `unicorn.GetService[T](ctx, name)` | Type-safe get (returns error) |
| `unicorn.MustGetService[T](ctx, name)` | Type-safe get (panics) |
