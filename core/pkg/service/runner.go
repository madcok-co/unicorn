package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"github.com/madcok-co/unicorn/core/pkg/handler"

	brokerAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/broker"
	httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
)

// Runner manages running services
type Runner struct {
	registry *Registry
	config   *RunnerConfig

	// Infrastructure (shared across services)
	db     contracts.Database
	cache  contracts.Cache
	logger contracts.Logger
	queue  contracts.Queue
	broker contracts.Broker

	// Active adapters
	httpAdapters   map[string]*httpAdapter.Adapter
	brokerAdapters map[string]*brokerAdapter.Adapter

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
}

// RunnerConfig configuration for runner
type RunnerConfig struct {
	// HTTP settings
	HTTPHost    string
	HTTPPort    int
	HTTPEnabled bool

	// Generic broker settings
	BrokerEnabled bool
	BrokerGroupID string

	// Which services to run (empty = all)
	Services []string

	// Port allocation strategy for multiple HTTP services
	// "shared" = all services share one port
	// "separate" = each service gets its own port (base + offset)
	PortStrategy string
	BasePort     int

	// Legacy (kept for backward compatibility but not used)
	KafkaBrokers []string
	KafkaGroupID string
	KafkaEnabled bool
}

// NewRunner creates a new service runner
func NewRunner(registry *Registry, config *RunnerConfig) *Runner {
	if config == nil {
		config = &RunnerConfig{
			HTTPHost:     "0.0.0.0",
			HTTPPort:     8080,
			HTTPEnabled:  true,
			PortStrategy: "shared",
			BasePort:     8080,
		}
	}

	return &Runner{
		registry:       registry,
		config:         config,
		httpAdapters:   make(map[string]*httpAdapter.Adapter),
		brokerAdapters: make(map[string]*brokerAdapter.Adapter),
	}
}

// SetDB sets shared database
func (r *Runner) SetDB(db contracts.Database) *Runner {
	r.db = db
	return r
}

// SetCache sets shared cache
func (r *Runner) SetCache(cache contracts.Cache) *Runner {
	r.cache = cache
	return r
}

// SetLogger sets shared logger
func (r *Runner) SetLogger(logger contracts.Logger) *Runner {
	r.logger = logger
	return r
}

// SetQueue sets shared queue
func (r *Runner) SetQueue(queue contracts.Queue) *Runner {
	r.queue = queue
	return r
}

// SetBroker sets shared message broker
func (r *Runner) SetBroker(broker contracts.Broker) *Runner {
	r.broker = broker
	return r
}

// Run starts the specified services (or all if none specified)
func (r *Runner) Run(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("runner already running")
	}
	r.running = true
	r.mu.Unlock()

	ctx, r.cancel = context.WithCancel(ctx)

	// Determine which services to run
	var services []*Service
	var err error

	if len(r.config.Services) > 0 {
		services, err = r.registry.ResolveDependencies(r.config.Services)
		if err != nil {
			return fmt.Errorf("failed to resolve dependencies: %w", err)
		}
	} else {
		services = r.registry.All()
	}

	if len(services) == 0 {
		return fmt.Errorf("no services to run")
	}

	// Log services being started
	if r.logger != nil {
		names := make([]string, len(services))
		for i, s := range services {
			names[i] = s.Name()
		}
		r.logger.Info("starting services", "services", names)
	}

	// Start services
	for _, svc := range services {
		if err := svc.Start(ctx); err != nil {
			return err
		}
	}

	// Build combined registry or separate per service
	if r.config.PortStrategy == "shared" {
		err = r.runSharedMode(ctx, services)
	} else {
		err = r.runSeparateMode(ctx, services)
	}

	if err != nil {
		return err
	}

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	select {
	case <-ctx.Done():
	case sig := <-sigCh:
		if r.logger != nil {
			r.logger.Info("received signal", "signal", sig.String())
		}
	}

	return r.Shutdown(context.Background(), services)
}

// runSharedMode runs all services on shared ports
func (r *Runner) runSharedMode(ctx context.Context, services []*Service) error {
	// Combine all handlers into one registry
	combinedRegistry := handler.NewRegistry()

	for _, svc := range services {
		for _, h := range svc.Handlers() {
			if err := combinedRegistry.Register(h); err != nil {
				return fmt.Errorf("failed to register handler from %s: %w", svc.Name(), err)
			}
		}
	}

	// Start HTTP adapter if enabled and has HTTP handlers
	if r.config.HTTPEnabled && combinedRegistry.HasHTTPHandlers() {
		adapter := httpAdapter.New(combinedRegistry, &httpAdapter.Config{
			Host: r.config.HTTPHost,
			Port: r.config.HTTPPort,
		})
		r.httpAdapters["shared"] = adapter

		go func() {
			if err := adapter.Start(ctx); err != nil {
				if r.logger != nil {
					r.logger.Error("HTTP adapter error", "error", err)
				}
			}
		}()

		if r.logger != nil {
			r.logger.Info("HTTP server started", "address", adapter.Address(), "routes", len(combinedRegistry.HTTPRoutes()))
		}
	}

	// Start generic broker adapter if enabled and has message handlers
	if r.config.BrokerEnabled && r.broker != nil && combinedRegistry.HasMessageHandlers() {
		groupID := r.config.BrokerGroupID
		if groupID == "" {
			groupID = "unicorn-consumer"
		}

		adapter := brokerAdapter.New(r.broker, combinedRegistry, &brokerAdapter.Config{
			GroupID: groupID,
		})
		r.brokerAdapters["shared"] = adapter

		go func() {
			if err := adapter.Start(ctx); err != nil {
				if r.logger != nil {
					r.logger.Error("Broker adapter error", "error", err)
				}
			}
		}()

		if r.logger != nil {
			r.logger.Info("Message broker started",
				"broker", r.broker.Name(),
				"topics", combinedRegistry.MessageTopics())
		}
	}

	return nil
}

// runSeparateMode runs each service on separate ports
func (r *Runner) runSeparateMode(ctx context.Context, services []*Service) error {
	portOffset := 0

	for _, svc := range services {
		registry := svc.Registry()

		// Start HTTP adapter for this service
		if r.config.HTTPEnabled && registry.HasHTTPHandlers() {
			port := r.config.BasePort + portOffset
			adapter := httpAdapter.New(registry, &httpAdapter.Config{
				Host: r.config.HTTPHost,
				Port: port,
			})
			r.httpAdapters[svc.Name()] = adapter

			go func(s *Service, a *httpAdapter.Adapter) {
				if err := a.Start(ctx); err != nil {
					if r.logger != nil {
						r.logger.Error("HTTP adapter error", "service", s.Name(), "error", err)
					}
				}
			}(svc, adapter)

			if r.logger != nil {
				r.logger.Info("HTTP server started",
					"service", svc.Name(),
					"address", adapter.Address(),
					"routes", len(registry.HTTPRoutes()))
			}

			portOffset++
		}

		// Start generic broker adapter for this service
		if r.config.BrokerEnabled && r.broker != nil && registry.HasMessageHandlers() {
			groupID := r.config.BrokerGroupID
			if groupID == "" {
				groupID = svc.Name()
			} else {
				groupID = fmt.Sprintf("%s-%s", groupID, svc.Name())
			}

			adapter := brokerAdapter.New(r.broker, registry, &brokerAdapter.Config{
				GroupID: groupID,
			})
			r.brokerAdapters[svc.Name()] = adapter

			go func(s *Service, a *brokerAdapter.Adapter) {
				if err := a.Start(ctx); err != nil {
					if r.logger != nil {
						r.logger.Error("Broker adapter error", "service", s.Name(), "error", err)
					}
				}
			}(svc, adapter)

			if r.logger != nil {
				r.logger.Info("Message broker started",
					"service", svc.Name(),
					"broker", r.broker.Name(),
					"topics", registry.MessageTopics(),
					"groupID", groupID)
			}
		}
	}

	return nil
}

// Shutdown gracefully stops all services
func (r *Runner) Shutdown(ctx context.Context, services []*Service) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return nil
	}
	r.running = false

	if r.cancel != nil {
		r.cancel()
	}

	// Stop HTTP adapters
	for name, adapter := range r.httpAdapters {
		if err := adapter.Shutdown(ctx); err != nil {
			if r.logger != nil {
				r.logger.Error("failed to stop HTTP adapter", "name", name, "error", err)
			}
		}
	}

	// Stop broker adapters
	for name, adapter := range r.brokerAdapters {
		if err := adapter.Stop(ctx); err != nil {
			if r.logger != nil {
				r.logger.Error("failed to stop broker adapter", "name", name, "error", err)
			}
		}
	}

	// Stop services in reverse order
	for i := len(services) - 1; i >= 0; i-- {
		if err := services[i].Stop(ctx); err != nil {
			if r.logger != nil {
				r.logger.Error("failed to stop service", "service", services[i].Name(), "error", err)
			}
		}
	}

	// Close infrastructure (best-effort cleanup)
	if r.db != nil {
		_ = r.db.Close()
	}
	if r.cache != nil {
		_ = r.cache.Close()
	}
	if r.queue != nil {
		_ = r.queue.Close()
	}
	if r.broker != nil {
		_ = r.broker.Disconnect(ctx)
	}
	if r.logger != nil {
		_ = r.logger.Sync()
	}

	return nil
}

// RunServices is a convenience function to run specific services
func (r *Runner) RunServices(ctx context.Context, serviceNames ...string) error {
	r.config.Services = serviceNames
	return r.Run(ctx)
}
