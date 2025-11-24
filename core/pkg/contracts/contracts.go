// Package contracts berisi semua generic interfaces untuk Unicorn Framework
// User hanya perlu interact dengan interface ini, bukan implementation
package contracts

// Adapter adalah interface untuk semua adapters
type Adapter interface {
	// Name returns the adapter name
	Name() string

	// Type returns the adapter type (database, cache, logger, queue, etc)
	Type() string

	// Initialize initializes the adapter with config
	Initialize(config any) error

	// Health checks if adapter is healthy
	Health() error

	// Close closes the adapter connection
	Close() error
}

// AdapterFactory untuk membuat adapter instances
type AdapterFactory interface {
	// Create creates a new adapter instance
	Create(config any) (Adapter, error)
}

// AdapterRegistry untuk register dan retrieve adapters
type AdapterRegistry interface {
	// Register registers an adapter factory
	Register(name string, factory AdapterFactory)

	// Get retrieves an adapter by name
	Get(name string) (Adapter, error)

	// List lists all registered adapters
	List() []string
}
