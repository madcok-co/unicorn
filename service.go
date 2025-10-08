// ============================================
// UNICORN Framework - Core Service
// Universal Integration & Connection Orchestrator Runtime Node
// ============================================

package unicorn

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Version information
const (
	Version = "1.0.0"
	Name    = "UNICORN"
)

var (
	// ErrServiceNotFound indicates that a service is not registered
	ErrServiceNotFound = errors.New("service not found")

	// ErrServiceAlreadyExists indicates that a service with the same name already exists
	ErrServiceAlreadyExists = errors.New("service already exists")

	// ErrInvalidServiceName indicates that the service name is invalid
	ErrInvalidServiceName = errors.New("invalid service name: must not be empty")

	// ErrNilHandler indicates that the service handler is nil
	ErrNilHandler = errors.New("service handler cannot be nil")
)

// Service represents a business logic handler.
// Each service implements this interface to process requests.
type Service interface {
	// Handle processes a request and returns a response.
	// The request can be from any trigger (HTTP, gRPC, Kafka, etc.).
	// The context provides access to all framework resources.
	Handle(ctx *Context, request interface{}) (interface{}, error)
}

// Definition contains service metadata and configuration.
// It defines how a service should be registered and triggered.
type Definition struct {
	// Name is the unique identifier for the service
	Name string

	// Handler is the service implementation
	Handler Service

	// Description provides human-readable information about the service
	Description string

	// Version is the service version (semantic versioning recommended)
	Version string

	// Trigger configurations - enable/disable per trigger type
	EnableHTTP      bool // Enable HTTP/REST API trigger
	EnableGRPC      bool // Enable gRPC trigger
	EnableKafka     bool // Enable Kafka consumer trigger
	EnableCron      bool // Enable scheduled/cron trigger
	EnableCLI       bool // Enable CLI command trigger
	EnableWebSocket bool // Enable WebSocket trigger
	EnableSQS       bool // Enable AWS SQS consumer trigger
	EnableNSQ       bool // Enable NSQ consumer trigger

	// Middleware configurations
	Middlewares []string // List of middleware names to apply (e.g., ["auth", "ratelimit"])

	// Timeout configuration (in seconds, 0 means no timeout)
	Timeout int

	// Additional metadata
	Tags     []string               // Tags for service categorization
	Metadata map[string]interface{} // Additional custom metadata
}

// Validate validates the service definition.
// It checks for required fields and valid configurations.
func (d *Definition) Validate() error {
	if d.Name == "" {
		return ErrInvalidServiceName
	}

	if d.Handler == nil {
		return ErrNilHandler
	}

	// At least one trigger must be enabled
	if !d.EnableHTTP && !d.EnableGRPC && !d.EnableKafka &&
		!d.EnableCron && !d.EnableCLI && !d.EnableWebSocket &&
		!d.EnableSQS && !d.EnableNSQ {
		return errors.New("at least one trigger must be enabled")
	}

	return nil
}

// HasTrigger checks if a specific trigger is enabled.
func (d *Definition) HasTrigger(triggerName string) bool {
	switch triggerName {
	case "http":
		return d.EnableHTTP
	case "grpc":
		return d.EnableGRPC
	case "kafka":
		return d.EnableKafka
	case "cron":
		return d.EnableCron
	case "cli":
		return d.EnableCLI
	case "websocket":
		return d.EnableWebSocket
	case "sqs":
		return d.EnableSQS
	case "nsq":
		return d.EnableNSQ
	default:
		return false
	}
}

// Registry manages all registered services.
// It provides thread-safe service registration and lookup.
type Registry struct {
	services map[string]*Definition
	mu       sync.RWMutex
}

// NewRegistry creates a new service registry.
func NewRegistry() *Registry {
	return &Registry{
		services: make(map[string]*Definition),
	}
}

// Register registers a new service.
// It returns an error if the service already exists or validation fails.
func (r *Registry) Register(def *Definition) error {
	if err := def.Validate(); err != nil {
		return fmt.Errorf("service validation failed: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.services[def.Name]; exists {
		return fmt.Errorf("%w: %s", ErrServiceAlreadyExists, def.Name)
	}

	r.services[def.Name] = def
	return nil
}

// MustRegister registers a service and panics on error.
// Use this in init() functions for fail-fast behavior.
func (r *Registry) MustRegister(def *Definition) {
	if err := r.Register(def); err != nil {
		panic(fmt.Sprintf("failed to register service '%s': %v", def.Name, err))
	}
}

// Get retrieves a service by name.
// It returns nil if the service doesn't exist.
func (r *Registry) Get(name string) *Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.services[name]
}

// GetHandler retrieves a service handler by name.
// It returns an error if the service doesn't exist.
func (r *Registry) GetHandler(name string) (Service, error) {
	def := r.Get(name)
	if def == nil {
		return nil, fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	return def.Handler, nil
}

// List returns all registered service names.
// The returned slice is a copy and safe to modify.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.services))
	for name := range r.services {
		names = append(names, name)
	}

	return names
}

// ListByTrigger returns all services that have a specific trigger enabled.
func (r *Registry) ListByTrigger(triggerName string) []*Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Definition
	for _, def := range r.services {
		if def.HasTrigger(triggerName) {
			result = append(result, def)
		}
	}

	return result
}

// Count returns the number of registered services.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.services)
}

// Exists checks if a service is registered.
func (r *Registry) Exists(name string) bool {
	return r.Get(name) != nil
}

// Unregister removes a service from the registry.
// It returns an error if the service doesn't exist.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.services[name]; !exists {
		return fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	delete(r.services, name)
	return nil
}

// Clear removes all services from the registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.services = make(map[string]*Definition)
}

// Global registry for convenient access
var globalRegistry = NewRegistry()

// Register registers a service in the global registry.
// This is the recommended way to register services.
func Register(def *Definition) error {
	return globalRegistry.Register(def)
}

// MustRegister registers a service in the global registry and panics on error.
// Use this in init() functions.
func MustRegister(def *Definition) {
	globalRegistry.MustRegister(def)
}

// GetService retrieves a service from the global registry.
func GetService(name string) *Definition {
	return globalRegistry.Get(name)
}

// GetHandler retrieves a service handler from the global registry.
func GetHandler(name string) (Service, error) {
	return globalRegistry.GetHandler(name)
}

// ListServices returns all registered service names from the global registry.
func ListServices() []string {
	return globalRegistry.List()
}

// ListServicesByTrigger returns all services with a specific trigger enabled.
func ListServicesByTrigger(triggerName string) []*Definition {
	return globalRegistry.ListByTrigger(triggerName)
}

// ServiceExists checks if a service exists in the global registry.
func ServiceExists(name string) bool {
	return globalRegistry.Exists(name)
}

// GetGlobalRegistry returns the global registry instance.
// Use this when you need direct access to the registry.
func GetGlobalRegistry() *Registry {
	return globalRegistry
}

// ExecuteService executes a service with the given context and request.
// This is a convenience function for executing services programmatically.
func ExecuteService(ctx context.Context, serviceName string, request interface{}) (interface{}, error) {
	handler, err := GetHandler(serviceName)
	if err != nil {
		return nil, err
	}

	// Create service context
	sctx := NewContext(ctx)

	// Execute service
	return handler.Handle(sctx, request)
}
