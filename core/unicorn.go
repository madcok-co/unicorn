// Package unicorn is a batteries-included framework for Go
// where users only need to focus on business logic.
//
// All infrastructure (database, cache, queue, logger) is accessed
// through a unified Context, and handlers can be triggered from
// multiple sources (HTTP, Message Broker, gRPC, Cron) with the same code.
//
// # Single Service Mode
//
//	func CreateUser(ctx *unicorn.Context, req CreateUserRequest) (*User, error) {
//	    db := ctx.DB()
//	    user := &User{Name: req.Name}
//	    db.Create(user)
//	    return user, nil
//	}
//
//	app.RegisterHandler(CreateUser).
//	    HTTP("POST", "/users").
//	    Message("user.create.command").  // Generic broker (Kafka, RabbitMQ, etc)
//	    Done()
//
//	app.Start()
//
// # Multi-Service Mode
//
// Handlers can be grouped into services that run independently or together:
//
//	// Define services
//	app.Service("user-service").
//	    Register(CreateUser).HTTP("POST", "/users").Done().
//	    Register(GetUser).HTTP("GET", "/users/:id").Done()
//
//	app.Service("order-service").
//	    DependsOn("user-service").
//	    Register(CreateOrder).HTTP("POST", "/orders").Message("order.create").Done()
//
//	// Run all services
//	app.Start()
//
//	// Or run specific services
//	app.RunServices("user-service", "order-service")
package unicorn

import (
	"github.com/madcok-co/unicorn/core/pkg/app"
	"github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"github.com/madcok-co/unicorn/core/pkg/handler"
	"github.com/madcok-co/unicorn/core/pkg/service"

	// Trigger adapters
	brokerAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/broker"
	cronAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/cron"
	httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"

	// Infrastructure adapters
	cacheAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/cache"
	databaseAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/database"
	loggerAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/logger"
	metricsAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/metrics"
	tracerAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/tracer"
	validatorAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/validator"

	// Security adapters
	"github.com/madcok-co/unicorn/core/pkg/adapters/security/audit"
	"github.com/madcok-co/unicorn/core/pkg/adapters/security/auth"
	"github.com/madcok-co/unicorn/core/pkg/adapters/security/encryptor"
	"github.com/madcok-co/unicorn/core/pkg/adapters/security/hasher"
	"github.com/madcok-co/unicorn/core/pkg/adapters/security/ratelimiter"
	"github.com/madcok-co/unicorn/core/pkg/adapters/security/secrets"
)

// Re-export main types for convenience
type (
	// App is the main application
	App = app.App

	// Config is application configuration
	Config = app.Config

	// Context is the unicorn context with all infrastructure
	Context = context.Context

	// Handler wraps user function with triggers
	Handler = handler.Handler

	// HandlerFunc is the type for handler functions
	HandlerFunc = handler.HandlerFunc

	// Service represents a group of handlers that can run independently
	Service = service.Service

	// ServiceRegistry manages all services
	ServiceRegistry = service.Registry

	// Custom service types
	CustomServiceRegistry         = context.ServiceRegistry
	AdvancedCustomServiceRegistry = context.AdvancedServiceRegistry
	ServiceFactory                = context.ServiceFactory
	ServiceDefinition             = context.ServiceDefinition
)

// Re-export contracts
type (
	Database      = contracts.Database
	Cache         = contracts.Cache
	Logger        = contracts.Logger
	Queue         = contracts.Queue
	Broker        = contracts.Broker
	BrokerMessage = contracts.BrokerMessage
	Validator     = contracts.Validator
	HTTPClient    = contracts.HTTPClient
)

// Re-export security contracts
type (
	Authenticator = contracts.Authenticator
	Authorizer    = contracts.Authorizer
	SecretManager = contracts.SecretManager
	Encryptor     = contracts.Encryptor
	RateLimiter   = contracts.RateLimiter
	AuditLogger   = contracts.AuditLogger
	Identity      = contracts.Identity
	Credentials   = contracts.Credentials
	TokenPair     = contracts.TokenPair
	AuditEvent    = contracts.AuditEvent
	AuditFilter   = contracts.AuditFilter
	TLSConfig     = contracts.TLSConfig
	CORSConfig    = contracts.CORSConfig
)

// Re-export adapter configs
type (
	HTTPConfig   = httpAdapter.Config
	BrokerConfig = brokerAdapter.Config
)

// Re-export infrastructure adapter types
type (
	// Database adapter types
	DatabaseDriver        = databaseAdapter.Driver
	DatabaseCRUD          = databaseAdapter.CRUD
	DatabaseQueryExecutor = databaseAdapter.QueryExecutor
	StandardSQLDriver     = databaseAdapter.StandardSQLDriver
	SimpleQueryBuilder    = databaseAdapter.SimpleQueryBuilder

	// Cache adapter types
	CacheDriver       = cacheAdapter.Driver
	CacheMultiKey     = cacheAdapter.MultiKeyDriver
	CacheAtomic       = cacheAdapter.AtomicDriver
	CacheLock         = cacheAdapter.LockDriver
	MemoryCacheDriver = cacheAdapter.MemoryDriver

	// Logger adapter types
	LoggerDriver    = loggerAdapter.Driver
	LoggerLevel     = loggerAdapter.Level
	StdLoggerDriver = loggerAdapter.StdDriver
	MultiLogger     = loggerAdapter.MultiDriver

	// Cron adapter types
	CronScheduler       = cronAdapter.Scheduler
	CronConfig          = cronAdapter.Config
	CronJob             = cronAdapter.Job
	SimpleCronScheduler = cronAdapter.SimpleScheduler

	// Validator adapter types
	ValidatorDriver  = validatorAdapter.Driver
	SimpleValidator  = validatorAdapter.SimpleValidator
	ValidationErrors = validatorAdapter.ValidationErrors

	// Metrics adapter types
	MetricsDriver       = metricsAdapter.Driver
	MetricsCounter      = metricsAdapter.CounterDriver
	MetricsGauge        = metricsAdapter.GaugeDriver
	MetricsHistogram    = metricsAdapter.HistogramDriver
	MemoryMetricsDriver = metricsAdapter.MemoryDriver

	// Tracer adapter types
	TracerDriver       = tracerAdapter.Driver
	TracerSpan         = tracerAdapter.SpanDriver
	MemoryTracerDriver = tracerAdapter.MemoryDriver
	ConsoleTracer      = tracerAdapter.ConsoleDriver
)

// Re-export service runner config
type (
	RunnerConfig = service.RunnerConfig
)

// Re-export security adapter types
type (
	// JWT
	JWTAuthenticator = auth.JWTAuthenticator
	JWTConfig        = auth.JWTConfig

	// API Key
	APIKeyAuthenticator = auth.APIKeyAuthenticator
	APIKeyConfig        = auth.APIKeyConfig
	APIKeyStore         = auth.APIKeyStore

	// Rate Limiter
	InMemoryRateLimiter       = ratelimiter.InMemoryRateLimiter
	InMemoryRateLimiterConfig = ratelimiter.InMemoryRateLimiterConfig
	SlidingWindowRateLimiter  = ratelimiter.SlidingWindowRateLimiter

	// Encryptor
	AESEncryptor       = encryptor.AESEncryptor
	AESEncryptorConfig = encryptor.AESEncryptorConfig

	// Password Hasher
	PasswordHasher = hasher.PasswordHasher
	BcryptHasher   = hasher.BcryptHasher
	BcryptConfig   = hasher.BcryptConfig
	Argon2Hasher   = hasher.Argon2Hasher
	Argon2Config   = hasher.Argon2Config

	// Secret Manager
	EnvSecretManager       = secrets.EnvSecretManager
	EnvSecretManagerConfig = secrets.EnvSecretManagerConfig

	// Audit Logger
	InMemoryAuditLogger       = audit.InMemoryAuditLogger
	InMemoryAuditLoggerConfig = audit.InMemoryAuditLoggerConfig
	AuditEventBuilder         = audit.AuditEventBuilder
)

// Security adapter constructors
var (
	// JWT
	NewJWTAuthenticator = auth.NewJWTAuthenticator
	DefaultJWTConfig    = auth.DefaultJWTConfig

	// API Key
	NewAPIKeyAuthenticator = auth.NewAPIKeyAuthenticator
	DefaultAPIKeyConfig    = auth.DefaultAPIKeyConfig
	NewInMemoryAPIKeyStore = auth.NewInMemoryAPIKeyStore

	// Rate Limiter
	NewInMemoryRateLimiter           = ratelimiter.NewInMemoryRateLimiter
	DefaultInMemoryRateLimiterConfig = ratelimiter.DefaultInMemoryRateLimiterConfig
	NewSlidingWindowRateLimiter      = ratelimiter.NewSlidingWindowRateLimiter

	// Encryptor
	NewAESEncryptor           = encryptor.NewAESEncryptor
	NewAESEncryptorFromString = encryptor.NewAESEncryptorFromString

	// Password Hasher
	NewBcryptHasher       = hasher.NewBcryptHasher
	DefaultBcryptConfig   = hasher.DefaultBcryptConfig
	NewArgon2Hasher       = hasher.NewArgon2Hasher
	DefaultArgon2Config   = hasher.DefaultArgon2Config
	LowMemoryArgon2Config = hasher.LowMemoryArgon2Config
	NewMultiHasher        = hasher.NewMultiHasher

	// Secret Manager
	NewEnvSecretManager           = secrets.NewEnvSecretManager
	DefaultEnvSecretManagerConfig = secrets.DefaultEnvSecretManagerConfig

	// Audit Logger
	NewInMemoryAuditLogger           = audit.NewInMemoryAuditLogger
	DefaultInMemoryAuditLoggerConfig = audit.DefaultInMemoryAuditLoggerConfig
	NewAuditEvent                    = audit.NewAuditEvent
	NewCompositeAuditLogger          = audit.NewCompositeAuditLogger
)

// Audit action constants
const (
	AuditActionCreate = audit.ActionCreate
	AuditActionRead   = audit.ActionRead
	AuditActionUpdate = audit.ActionUpdate
	AuditActionDelete = audit.ActionDelete
	AuditActionLogin  = audit.ActionLogin
	AuditActionLogout = audit.ActionLogout
)

// Infrastructure adapter constructors
var (
	// Database
	NewStandardSQLDriver  = databaseAdapter.NewStandardSQLDriver
	NewSimpleQueryBuilder = databaseAdapter.NewSimpleQueryBuilder

	// Cache
	NewMemoryCacheDriver = cacheAdapter.NewMemoryDriver

	// Logger
	NewStdLoggerDriver = loggerAdapter.NewStdDriver
	NewMultiLogger     = loggerAdapter.NewMultiDriver

	// Cron
	NewSimpleCronScheduler = cronAdapter.NewSimpleScheduler
	WrapRobfigCron         = cronAdapter.WrapRobfigCron

	// Validator
	NewSimpleValidator = validatorAdapter.NewSimpleValidator

	// Metrics
	NewMemoryMetricsDriver = metricsAdapter.NewMemoryDriver

	// Tracer
	NewMemoryTracerDriver = tracerAdapter.NewMemoryDriver
	NewConsoleTracer      = tracerAdapter.NewConsoleDriver
)

// Logger level constants
const (
	LogLevelDebug = loggerAdapter.LevelDebug
	LogLevelInfo  = loggerAdapter.LevelInfo
	LogLevelWarn  = loggerAdapter.LevelWarn
	LogLevelError = loggerAdapter.LevelError
)

// New creates a new Unicorn application
func New(config *Config) *App {
	return app.New(config)
}

// Version returns the framework version
func Version() string {
	return "0.1.0"
}

// ============ Custom Service Helpers ============

// GetService is a generic helper to retrieve a typed service from context
// Provides compile-time type safety for service retrieval
//
// Example:
//
//	type EmailService interface {
//	    Send(to, subject, body string) error
//	}
//
//	// In handler:
//	emailSvc, err := unicorn.GetService[EmailService](ctx, "email")
//	if err != nil {
//	    return err
//	}
//	emailSvc.Send("user@example.com", "Hello", "World")
func GetService[T any](ctx *Context, name string) (T, error) {
	return context.GetService[T](ctx, name)
}

// MustGetService is a generic helper that panics if service is not found or wrong type
//
// Example:
//
//	emailSvc := unicorn.MustGetService[EmailService](ctx, "email")
//	emailSvc.Send("user@example.com", "Hello", "World")
func MustGetService[T any](ctx *Context, name string) T {
	return context.MustGetService[T](ctx, name)
}

// NewServiceRegistry creates a new simple service registry
func NewServiceRegistry() *CustomServiceRegistry {
	return context.NewServiceRegistry()
}

// NewAdvancedServiceRegistry creates a new advanced service registry
// with support for singletons and factories
func NewAdvancedServiceRegistry() *AdvancedCustomServiceRegistry {
	return context.NewAdvancedServiceRegistry()
}
