package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/madcok-co/unicorn/core/pkg/handler"
)

// Service represents a group of handlers that can run independently
type Service struct {
	name        string
	description string

	// Handlers dalam service ini
	handlers []*handler.Handler

	// Service-specific registry
	registry *handler.Registry

	// Service dependencies
	dependencies []string

	// Lifecycle hooks
	onStart []func(ctx context.Context) error
	onStop  []func(ctx context.Context) error

	// Status
	mu      sync.RWMutex
	running bool
}

// New creates a new Service
func New(name string) *Service {
	return &Service{
		name:         name,
		handlers:     make([]*handler.Handler, 0),
		registry:     handler.NewRegistry(),
		dependencies: make([]string, 0),
		onStart:      make([]func(ctx context.Context) error, 0),
		onStop:       make([]func(ctx context.Context) error, 0),
	}
}

// Name returns service name
func (s *Service) Name() string {
	return s.name
}

// Describe sets service description
func (s *Service) Describe(desc string) *Service {
	s.description = desc
	return s
}

// Description returns service description
func (s *Service) Description() string {
	return s.description
}

// DependsOn adds service dependencies
func (s *Service) DependsOn(services ...string) *Service {
	s.dependencies = append(s.dependencies, services...)
	return s
}

// Dependencies returns service dependencies
func (s *Service) Dependencies() []string {
	return s.dependencies
}

// Register adds a handler to this service
func (s *Service) Register(fn handler.HandlerFunc) *HandlerBuilder {
	h := handler.New(fn)
	return &HandlerBuilder{
		service: s,
		handler: h,
	}
}

// AddHandler adds a pre-built handler
func (s *Service) AddHandler(h *handler.Handler) error {
	s.handlers = append(s.handlers, h)
	return s.registry.Register(h)
}

// Handlers returns all handlers in this service
func (s *Service) Handlers() []*handler.Handler {
	return s.handlers
}

// Registry returns the service's handler registry
func (s *Service) Registry() *handler.Registry {
	return s.registry
}

// OnStart adds startup hook
func (s *Service) OnStart(fn func(ctx context.Context) error) *Service {
	s.onStart = append(s.onStart, fn)
	return s
}

// OnStop adds shutdown hook
func (s *Service) OnStop(fn func(ctx context.Context) error) *Service {
	s.onStop = append(s.onStop, fn)
	return s
}

// Start runs all startup hooks
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("service %s already running", s.name)
	}
	s.running = true
	s.mu.Unlock()

	for _, fn := range s.onStart {
		if err := fn(ctx); err != nil {
			return fmt.Errorf("service %s startup hook failed: %w", s.name, err)
		}
	}
	return nil
}

// Stop runs all shutdown hooks
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()

	var errs []error
	for _, fn := range s.onStop {
		if err := fn(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("service %s shutdown errors: %v", s.name, errs)
	}
	return nil
}

// IsRunning returns whether service is running
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// HandlerBuilder untuk fluent API
type HandlerBuilder struct {
	service *Service
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

// Kafka registers Kafka trigger
func (b *HandlerBuilder) Kafka(topic string, opts ...handler.KafkaOption) *HandlerBuilder {
	b.handler.Kafka(topic, opts...)
	return b
}

// GRPC registers gRPC trigger
func (b *HandlerBuilder) GRPC(service, method string) *HandlerBuilder {
	b.handler.GRPC(service, method)
	return b
}

// Cron registers Cron trigger
func (b *HandlerBuilder) Cron(schedule string) *HandlerBuilder {
	b.handler.Cron(schedule)
	return b
}

// Done finalizes and registers handler to service
func (b *HandlerBuilder) Done() *Service {
	_ = b.service.AddHandler(b.handler) // Error ignored - fluent API pattern
	return b.service
}

// And continues with another handler registration (fluent API)
func (b *HandlerBuilder) And() *Service {
	_ = b.service.AddHandler(b.handler) // Error ignored - fluent API pattern
	return b.service
}
