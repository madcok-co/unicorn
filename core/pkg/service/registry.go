package service

import (
	"fmt"
	"sync"
)

// Registry manages all services
type Registry struct {
	mu       sync.RWMutex
	services map[string]*Service
	order    []string // untuk maintain registration order
}

// NewRegistry creates a new service registry
func NewRegistry() *Registry {
	return &Registry{
		services: make(map[string]*Service),
		order:    make([]string, 0),
	}
}

// Register adds a service to registry
func (r *Registry) Register(svc *Service) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.services[svc.Name()]; exists {
		return fmt.Errorf("service already registered: %s", svc.Name())
	}

	r.services[svc.Name()] = svc
	r.order = append(r.order, svc.Name())
	return nil
}

// Get returns a service by name
func (r *Registry) Get(name string) (*Service, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	svc, ok := r.services[name]
	return svc, ok
}

// GetOrCreate returns existing service or creates new one
func (r *Registry) GetOrCreate(name string) *Service {
	r.mu.Lock()
	defer r.mu.Unlock()

	if svc, ok := r.services[name]; ok {
		return svc
	}

	svc := New(name)
	r.services[name] = svc
	r.order = append(r.order, name)
	return svc
}

// All returns all services in registration order
func (r *Registry) All() []*Service {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Service, 0, len(r.order))
	for _, name := range r.order {
		if svc, ok := r.services[name]; ok {
			result = append(result, svc)
		}
	}
	return result
}

// Names returns all service names
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, len(r.order))
	copy(result, r.order)
	return result
}

// Filter returns services matching given names
func (r *Registry) Filter(names ...string) []*Service {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}

	result := make([]*Service, 0)
	for _, name := range r.order {
		if nameSet[name] {
			if svc, ok := r.services[name]; ok {
				result = append(result, svc)
			}
		}
	}
	return result
}

// Count returns number of services
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.services)
}

// ResolveDependencies returns services in dependency order
// Services with dependencies come after their dependencies
func (r *Registry) ResolveDependencies(names []string) ([]*Service, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Build set of requested services
	requested := make(map[string]bool)
	for _, n := range names {
		requested[n] = true
	}

	// Topological sort
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	result := make([]*Service, 0)

	var visit func(name string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		if visiting[name] {
			return fmt.Errorf("circular dependency detected: %s", name)
		}

		visiting[name] = true

		svc, ok := r.services[name]
		if !ok {
			return fmt.Errorf("service not found: %s", name)
		}

		// Visit dependencies first
		for _, dep := range svc.Dependencies() {
			if err := visit(dep); err != nil {
				return err
			}
		}

		visiting[name] = false
		visited[name] = true

		// Only add if requested (or is a dependency)
		result = append(result, svc)
		return nil
	}

	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return result, nil
}
