package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"github.com/madcok-co/unicorn/core/pkg/handler"
	"github.com/madcok-co/unicorn/core/pkg/service"

	brokerAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/broker"
	cronAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/cron"
	httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
)

// App adalah main application struct
type App struct {
	name    string
	version string

	// Handler registry (untuk backward compatibility)
	registry *handler.Registry

	// Service registry (untuk multi-service mode)
	services *service.Registry

	// Custom services registry (user-defined interfaces)
	customServices *ucontext.AdvancedServiceRegistry

	// Shared adapters reference (used by all contexts - optimized)
	adapters *ucontext.AppAdapters

	// Trigger adapters
	httpAdapter   *httpAdapter.Adapter
	brokerAdapter *brokerAdapter.Adapter
	cronAdapter   *cronAdapter.Adapter

	// Configuration
	config *Config

	// Lifecycle hooks
	onStart []func() error
	onStop  []func() error

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// Config untuk application
type Config struct {
	Name    string
	Version string

	// HTTP Config
	HTTP *httpAdapter.Config

	// Broker Config (generic message broker)
	Broker *brokerAdapter.Config

	// Feature flags
	EnableHTTP   bool
	EnableBroker bool // Generic message broker
	EnableGRPC   bool
	EnableCron   bool
}

// New creates a new Unicorn application
func New(config *Config) *App {
	if config == nil {
		config = &Config{
			Name:       "unicorn-app",
			Version:    "1.0.0",
			EnableHTTP: true,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		name:           config.Name,
		version:        config.Version,
		registry:       handler.NewRegistry(),
		services:       service.NewRegistry(),
		customServices: ucontext.NewAdvancedServiceRegistry(),
		config:         config,
		onStart:        make([]func() error, 0),
		onStop:         make([]func() error, 0),
		ctx:            ctx,
		cancel:         cancel,
		// Initialize shared adapters (optimized - single allocation)
		adapters: &ucontext.AppAdapters{
			Databases:  make(map[string]contracts.Database),
			Caches:     make(map[string]contracts.Cache),
			Loggers:    make(map[string]contracts.Logger),
			Queues:     make(map[string]contracts.Queue),
			Brokers:    make(map[string]contracts.Broker),
			Validators: make(map[string]contracts.Validator),
			MetricsMap: make(map[string]contracts.Metrics),
			TracersMap: make(map[string]contracts.Tracer),
		},
	}

	return app
}

// ============ Infrastructure Setup ============

// SetDB sets the default database adapter
func (a *App) SetDB(db contracts.Database, name ...string) *App {
	if len(name) > 0 && name[0] != "" {
		a.adapters.Databases[name[0]] = db
	} else {
		a.adapters.DB = db
	}
	return a
}

// SetCache sets the default cache adapter
func (a *App) SetCache(cache contracts.Cache, name ...string) *App {
	if len(name) > 0 && name[0] != "" {
		a.adapters.Caches[name[0]] = cache
	} else {
		a.adapters.Cache = cache
	}
	return a
}

// SetLogger sets the default logger adapter
func (a *App) SetLogger(logger contracts.Logger, name ...string) *App {
	if len(name) > 0 && name[0] != "" {
		a.adapters.Loggers[name[0]] = logger
	} else {
		a.adapters.Logger = logger
	}
	return a
}

// SetQueue sets the queue adapter (legacy)
func (a *App) SetQueue(queue contracts.Queue, name ...string) *App {
	if len(name) > 0 && name[0] != "" {
		a.adapters.Queues[name[0]] = queue
	} else {
		a.adapters.Queue = queue
	}
	return a
}

// SetBroker sets the message broker
// This is the preferred way to set up message handling
func (a *App) SetBroker(broker contracts.Broker, name ...string) *App {
	if len(name) > 0 && name[0] != "" {
		a.adapters.Brokers[name[0]] = broker
	} else {
		a.adapters.Broker = broker
		a.config.EnableBroker = true
	}
	return a
}

// Broker returns the default message broker
func (a *App) Broker(name ...string) contracts.Broker {
	if len(name) > 0 && name[0] != "" {
		return a.adapters.Brokers[name[0]]
	}
	return a.adapters.Broker
}

// SetMetrics sets the metrics collector
func (a *App) SetMetrics(metrics contracts.Metrics, name ...string) *App {
	if len(name) > 0 && name[0] != "" {
		a.adapters.MetricsMap[name[0]] = metrics
	} else {
		a.adapters.Metrics = metrics
	}
	return a
}

// Metrics returns the default metrics collector
func (a *App) Metrics(name ...string) contracts.Metrics {
	if len(name) > 0 && name[0] != "" {
		return a.adapters.MetricsMap[name[0]]
	}
	return a.adapters.Metrics
}

// SetTracer sets the tracer
func (a *App) SetTracer(tracer contracts.Tracer, name ...string) *App {
	if len(name) > 0 && name[0] != "" {
		a.adapters.TracersMap[name[0]] = tracer
	} else {
		a.adapters.Tracer = tracer
	}
	return a
}

// Tracer returns the default tracer
func (a *App) Tracer(name ...string) contracts.Tracer {
	if len(name) > 0 && name[0] != "" {
		return a.adapters.TracersMap[name[0]]
	}
	return a.adapters.Tracer
}

// SetValidator sets the validator
func (a *App) SetValidator(validator contracts.Validator, name ...string) *App {
	if len(name) > 0 && name[0] != "" {
		a.adapters.Validators[name[0]] = validator
	} else {
		a.adapters.Validator = validator
	}
	return a
}

// Validator returns the default validator
func (a *App) Validator(name ...string) contracts.Validator {
	if len(name) > 0 && name[0] != "" {
		return a.adapters.Validators[name[0]]
	}
	return a.adapters.Validator
}

// SetCronScheduler sets the cron scheduler
func (a *App) SetCronScheduler(scheduler cronAdapter.Scheduler) *App {
	a.cronAdapter = cronAdapter.New(a.registry, scheduler, nil)
	a.config.EnableCron = true
	return a
}

// ============ Custom Services Registration ============

// RegisterService registers a singleton custom service
// Services are injected into every request context automatically
//
// Example:
//
//	type EmailService interface {
//	    Send(to, subject, body string) error
//	}
//
//	app.RegisterService("email", myEmailService)
//
//	// In handler:
//	emailSvc := ctx.GetService("email").(EmailService)
func (a *App) RegisterService(name string, service any) *App {
	a.customServices.RegisterSingleton(name, service)
	return a
}

// RegisterServiceFactory registers a factory-based service
// A new instance is created for each request context
//
// Example:
//
//	app.RegisterServiceFactory("requestLogger", func(ctx *ucontext.Context) (any, error) {
//	    return NewRequestLogger(ctx.Request().ID), nil
//	})
func (a *App) RegisterServiceFactory(name string, factory ucontext.ServiceFactory) *App {
	a.customServices.RegisterFactory(name, factory)
	return a
}

// CustomServices returns the custom service registry
func (a *App) CustomServices() *ucontext.AdvancedServiceRegistry {
	return a.customServices
}

// ============ Service Registration ============

// Service gets or creates a service by name
func (a *App) Service(name string) *service.Service {
	return a.services.GetOrCreate(name)
}

// Services returns the service registry
func (a *App) Services() *service.Registry {
	return a.services
}

// ============ Handler Registration (backward compatible) ============

// Register registers a handler function
func (a *App) Register(fn handler.HandlerFunc) *handler.Handler {
	h := handler.New(fn)
	return h
}

// Handle registers a handler and adds it to registry
func (a *App) Handle(h *handler.Handler) error {
	return a.registry.Register(h)
}

// RegisterHandler is a convenience method to register and add handler
func (a *App) RegisterHandler(fn handler.HandlerFunc) *HandlerBuilder {
	h := handler.New(fn)
	return &HandlerBuilder{
		app:     a,
		handler: h,
	}
}

// HandlerBuilder untuk fluent API registration
type HandlerBuilder struct {
	app     *App
	handler *handler.Handler
}

// Named sets handler name
func (b *HandlerBuilder) Named(name string) *HandlerBuilder {
	b.handler.Named(name)
	return b
}

// HTTP registers HTTP trigger
func (b *HandlerBuilder) HTTP(method, path string) *HandlerBuilder {
	b.handler.HTTP(method, path)
	return b
}

// Message registers generic message broker trigger
func (b *HandlerBuilder) Message(topic string, opts ...handler.MessageOption) *HandlerBuilder {
	b.handler.Message(topic, opts...)
	return b
}

// Kafka registers Kafka trigger (legacy, use Message)
func (b *HandlerBuilder) Kafka(topic string, opts ...handler.KafkaOption) *HandlerBuilder {
	b.handler.Kafka(topic, opts...)
	return b
}

// Cron registers Cron trigger
func (b *HandlerBuilder) Cron(schedule string) *HandlerBuilder {
	b.handler.Cron(schedule)
	return b
}

// Done finalizes registration
func (b *HandlerBuilder) Done() error {
	return b.app.registry.Register(b.handler)
}

// ============ Lifecycle Hooks ============

// OnStart adds a startup hook
func (a *App) OnStart(fn func() error) *App {
	a.onStart = append(a.onStart, fn)
	return a
}

// OnStop adds a shutdown hook
func (a *App) OnStop(fn func() error) *App {
	a.onStop = append(a.onStop, fn)
	return a
}

// ============ Application Lifecycle ============

// Start starts the application (all services)
func (a *App) Start() error {
	return a.RunServices()
}

// RunServices runs specific services (or all if none specified)
func (a *App) RunServices(serviceNames ...string) error {
	// Run startup hooks
	for _, fn := range a.onStart {
		if err := fn(); err != nil {
			return fmt.Errorf("startup hook failed: %w", err)
		}
	}

	// Check if using service-based mode
	if a.services.Count() > 0 {
		return a.runServiceMode(serviceNames)
	}

	// Legacy mode - use registry directly
	return a.runLegacyMode()
}

// runServiceMode runs in multi-service mode
func (a *App) runServiceMode(serviceNames []string) error {
	runner := service.NewRunner(a.services, &service.RunnerConfig{
		HTTPHost:      getHTTPHost(a.config),
		HTTPPort:      getHTTPPort(a.config),
		HTTPEnabled:   a.config.EnableHTTP,
		BrokerEnabled: a.config.EnableBroker,
		Services:      serviceNames,
		PortStrategy:  "shared", // default shared mode
	})

	runner.SetDB(a.adapters.DB)
	runner.SetCache(a.adapters.Cache)
	runner.SetLogger(a.adapters.Logger)
	runner.SetQueue(a.adapters.Queue)
	if a.adapters.Broker != nil {
		runner.SetBroker(a.adapters.Broker)
	}

	return runner.Run(a.ctx)
}

// runLegacyMode runs in legacy single-registry mode
func (a *App) runLegacyMode() error {
	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start adapters
	errCh := make(chan error, 4)
	var wg sync.WaitGroup

	// Start HTTP adapter
	if a.config.EnableHTTP && a.registry.HasHTTPHandlers() {
		a.httpAdapter = httpAdapter.New(a.registry, a.config.HTTP)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := a.httpAdapter.Start(a.ctx); err != nil {
				errCh <- fmt.Errorf("HTTP adapter error: %w", err)
			}
		}()

		if a.adapters.Logger != nil {
			a.adapters.Logger.Info("HTTP server started", "address", a.httpAdapter.Address())
		}
	}

	// Start generic broker adapter
	if a.config.EnableBroker && a.adapters.Broker != nil && a.registry.HasMessageHandlers() {
		a.brokerAdapter = brokerAdapter.New(a.adapters.Broker, a.registry, a.config.Broker)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := a.brokerAdapter.Start(a.ctx); err != nil {
				errCh <- fmt.Errorf("Broker adapter error: %w", err)
			}
		}()

		if a.adapters.Logger != nil {
			a.adapters.Logger.Info("Message broker started",
				"broker", a.adapters.Broker.Name(),
				"topics", a.registry.MessageTopics())
		}
	}

	// Start cron adapter
	if a.config.EnableCron && a.cronAdapter != nil && a.registry.HasCronHandlers() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := a.cronAdapter.Start(a.ctx); err != nil {
				errCh <- fmt.Errorf("Cron adapter error: %w", err)
			}
		}()

		if a.adapters.Logger != nil {
			a.adapters.Logger.Info("Cron scheduler started",
				"schedules", a.registry.CronSchedules())
		}
	}

	// Wait for shutdown signal or error
	select {
	case sig := <-sigCh:
		if a.adapters.Logger != nil {
			a.adapters.Logger.Info("Received shutdown signal", "signal", sig.String())
		}
	case err := <-errCh:
		if a.adapters.Logger != nil {
			a.adapters.Logger.Error("Adapter error", "error", err)
		}
		return err
	}

	return a.Shutdown()
}

// Shutdown gracefully shuts down the application
func (a *App) Shutdown() error {
	// Cancel context to stop all adapters
	a.cancel()

	// Run shutdown hooks
	for _, fn := range a.onStop {
		if err := fn(); err != nil {
			if a.adapters.Logger != nil {
				a.adapters.Logger.Error("shutdown hook failed", "error", err)
			}
		}
	}

	// Close custom services
	if a.customServices != nil {
		if err := a.customServices.CloseAll(); err != nil {
			if a.adapters.Logger != nil {
				a.adapters.Logger.Error("failed to close custom services", "error", err)
			}
		}
	}

	// Close infrastructure (best-effort cleanup)
	if a.adapters.DB != nil {
		_ = a.adapters.DB.Close()
	}
	if a.adapters.Cache != nil {
		_ = a.adapters.Cache.Close()
	}
	if a.adapters.Queue != nil {
		_ = a.adapters.Queue.Close()
	}
	if a.adapters.Broker != nil {
		_ = a.adapters.Broker.Disconnect(context.Background())
	}
	if a.adapters.Logger != nil {
		_ = a.adapters.Logger.Sync()
	}

	return nil
}

// ============ Context Factory ============

// NewContext creates a new Unicorn context with all infrastructure (optimized with pooling)
func (a *App) NewContext(ctx context.Context) *ucontext.Context {
	// Use optimized path with object pooling and lazy injection
	uctx := ucontext.Acquire(ctx, a.adapters)

	// Inject custom services
	if a.customServices != nil {
		if err := a.customServices.InjectInto(uctx); err != nil {
			if a.adapters.Logger != nil {
				a.adapters.Logger.Error("failed to inject custom services", "error", err)
			}
		}
	}

	return uctx
}

// Adapters returns the shared adapters reference
func (a *App) Adapters() *ucontext.AppAdapters {
	return a.adapters
}

// ============ Getters ============

// Name returns app name
func (a *App) Name() string {
	return a.name
}

// Version returns app version
func (a *App) Version() string {
	return a.version
}

// Registry returns handler registry
func (a *App) Registry() *handler.Registry {
	return a.registry
}

// ============ Helper functions ============

func getHTTPHost(c *Config) string {
	if c.HTTP != nil {
		return c.HTTP.Host
	}
	return "0.0.0.0"
}

func getHTTPPort(c *Config) int {
	if c.HTTP != nil {
		return c.HTTP.Port
	}
	return 8080
}
