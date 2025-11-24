package context

import (
	"fmt"
	"reflect"
)

// ServiceRegistry holds registered services at the application level
// This is used to inject services into each request context
type ServiceRegistry struct {
	services map[string]any
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string]any),
	}
}

// Register registers a service with a name
func (r *ServiceRegistry) Register(name string, service any) *ServiceRegistry {
	r.services[name] = service
	return r
}

// Get retrieves a service by name
func (r *ServiceRegistry) Get(name string) any {
	return r.services[name]
}

// Has checks if a service exists
func (r *ServiceRegistry) Has(name string) bool {
	_, ok := r.services[name]
	return ok
}

// All returns all registered services
func (r *ServiceRegistry) All() map[string]any {
	return r.services
}

// Names returns all service names
func (r *ServiceRegistry) Names() []string {
	names := make([]string, 0, len(r.services))
	for name := range r.services {
		names = append(names, name)
	}
	return names
}

// InjectInto injects all services into a context
func (r *ServiceRegistry) InjectInto(ctx *Context) {
	for name, svc := range r.services {
		ctx.RegisterService(name, svc)
	}
}

// GetService is a generic helper function to retrieve a typed service from context
// This provides compile-time type safety for service retrieval
//
// Example:
//
//	type EmailService interface {
//	    Send(to, subject, body string) error
//	}
//
//	// In handler:
//	emailSvc, err := context.GetService[EmailService](ctx, "email")
//	if err != nil {
//	    return err
//	}
//	emailSvc.Send("user@example.com", "Hello", "World")
func GetService[T any](ctx *Context, name string) (T, error) {
	var zero T
	svc := ctx.GetService(name)
	if svc == nil {
		return zero, fmt.Errorf("service not found: %s", name)
	}

	typed, ok := svc.(T)
	if !ok {
		return zero, fmt.Errorf("service %s is not of type %s, got %s",
			name, reflect.TypeOf(zero), reflect.TypeOf(svc))
	}

	return typed, nil
}

// MustGetService is a generic helper that panics if service is not found or wrong type
//
// Example:
//
//	emailSvc := context.MustGetService[EmailService](ctx, "email")
//	emailSvc.Send("user@example.com", "Hello", "World")
func MustGetService[T any](ctx *Context, name string) T {
	svc, err := GetService[T](ctx, name)
	if err != nil {
		panic(err)
	}
	return svc
}

// ServiceProvider is an interface that custom services can implement
// to receive lifecycle callbacks
type ServiceProvider interface {
	// Boot is called when the service is first registered
	Boot() error
}

// ClosableService is an interface for services that need cleanup
type ClosableService interface {
	// Close is called during application shutdown
	Close() error
}

// HealthCheckable is an interface for services that support health checks
type HealthCheckable interface {
	// Health returns nil if healthy, error otherwise
	Health() error
}

// ServiceFactory creates a new instance of a service
// Useful for request-scoped services
type ServiceFactory func(ctx *Context) (any, error)

// ServiceDefinition holds service configuration
type ServiceDefinition struct {
	Name      string
	Instance  any            // Singleton instance
	Factory   ServiceFactory // Factory for per-request instances
	Singleton bool           // If true, use Instance; if false, use Factory
}

// AdvancedServiceRegistry supports both singleton and factory-based services
type AdvancedServiceRegistry struct {
	definitions map[string]*ServiceDefinition
}

// NewAdvancedServiceRegistry creates a new advanced service registry
func NewAdvancedServiceRegistry() *AdvancedServiceRegistry {
	return &AdvancedServiceRegistry{
		definitions: make(map[string]*ServiceDefinition),
	}
}

// RegisterSingleton registers a singleton service
func (r *AdvancedServiceRegistry) RegisterSingleton(name string, instance any) *AdvancedServiceRegistry {
	r.definitions[name] = &ServiceDefinition{
		Name:      name,
		Instance:  instance,
		Singleton: true,
	}
	return r
}

// RegisterFactory registers a factory-based service (created per request)
func (r *AdvancedServiceRegistry) RegisterFactory(name string, factory ServiceFactory) *AdvancedServiceRegistry {
	r.definitions[name] = &ServiceDefinition{
		Name:      name,
		Factory:   factory,
		Singleton: false,
	}
	return r
}

// InjectInto injects services into a context
// Singletons are injected directly, factories are called to create instances
func (r *AdvancedServiceRegistry) InjectInto(ctx *Context) error {
	for name, def := range r.definitions {
		if def.Singleton {
			ctx.RegisterService(name, def.Instance)
		} else if def.Factory != nil {
			instance, err := def.Factory(ctx)
			if err != nil {
				return fmt.Errorf("failed to create service %s: %w", name, err)
			}
			ctx.RegisterService(name, instance)
		}
	}
	return nil
}

// CloseAll closes all closable services
func (r *AdvancedServiceRegistry) CloseAll() error {
	var lastErr error
	for name, def := range r.definitions {
		if def.Singleton && def.Instance != nil {
			if closable, ok := def.Instance.(ClosableService); ok {
				if err := closable.Close(); err != nil {
					lastErr = fmt.Errorf("failed to close service %s: %w", name, err)
				}
			}
		}
	}
	return lastErr
}

// HealthCheckAll runs health checks on all healthcheckable services
func (r *AdvancedServiceRegistry) HealthCheckAll() map[string]error {
	results := make(map[string]error)
	for name, def := range r.definitions {
		if def.Singleton && def.Instance != nil {
			if healthable, ok := def.Instance.(HealthCheckable); ok {
				results[name] = healthable.Health()
			}
		}
	}
	return results
}
