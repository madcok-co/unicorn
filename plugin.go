// ============================================
// UNICORN Framework - Plugin System
// Extensible plugin architecture for integrations
// ============================================

package unicorn

import (
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrPluginAlreadyRegistered indicates a plugin with the same name already exists
	ErrPluginAlreadyRegistered = errors.New("plugin already registered")

	// ErrPluginInitFailed indicates plugin initialization failed
	ErrPluginInitFailed = errors.New("plugin initialization failed")
)

// Plugin represents an external service integration.
// Plugins provide functionality like payment processing, email sending, etc.
type Plugin interface {
	// Name returns the plugin name
	Name() string

	// Version returns the plugin version
	Version() string

	// Initialize initializes the plugin with configuration
	Initialize(config map[string]interface{}) error

	// Close closes the plugin and releases resources
	Close() error
}

// PluginFactory creates new plugin instances.
type PluginFactory func(config map[string]interface{}) (Plugin, error)

// PluginRegistry manages plugin factories.
type PluginRegistry struct {
	factories map[string]PluginFactory
	mu        sync.RWMutex
}

// NewPluginRegistry creates a new plugin registry.
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		factories: make(map[string]PluginFactory),
	}
}

// Register registers a plugin factory.
func (r *PluginRegistry) Register(name string, factory PluginFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("%w: %s", ErrPluginAlreadyRegistered, name)
	}

	r.factories[name] = factory
	return nil
}

// MustRegister registers a plugin factory and panics on error.
func (r *PluginRegistry) MustRegister(name string, factory PluginFactory) {
	if err := r.Register(name, factory); err != nil {
		panic(fmt.Sprintf("failed to register plugin '%s': %v", name, err))
	}
}

// Get retrieves a plugin factory by name.
func (r *PluginRegistry) Get(name string) PluginFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.factories[name]
}

// List returns all registered plugin names.
func (r *PluginRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}

	return names
}

// PluginManager manages active plugin instances.
type PluginManager struct {
	plugins  map[string]Plugin
	registry *PluginRegistry
	mu       sync.RWMutex
}

// NewPluginManager creates a new plugin manager.
func NewPluginManager(registry *PluginRegistry) *PluginManager {
	if registry == nil {
		registry = NewPluginRegistry()
	}

	return &PluginManager{
		plugins:  make(map[string]Plugin),
		registry: registry,
	}
}

// Initialize initializes a plugin from config.
func (m *PluginManager) Initialize(name string, config map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already initialized
	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("plugin already initialized: %s", name)
	}

	// Get factory
	factory := m.registry.Get(name)
	if factory == nil {
		return fmt.Errorf("plugin factory not found: %s", name)
	}

	// Create plugin instance
	plugin, err := factory(config)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrPluginInitFailed, name, err)
	}

	// Initialize plugin
	if err := plugin.Initialize(config); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrPluginInitFailed, name, err)
	}

	m.plugins[name] = plugin
	return nil
}

// Get retrieves an initialized plugin by name.
func (m *PluginManager) Get(name string) Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.plugins[name]
}

// List returns all initialized plugin names.
func (m *PluginManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.plugins))
	for name := range m.plugins {
		names = append(names, name)
	}

	return names
}

// Close closes all plugins and releases resources.
func (m *PluginManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error

	for name, plugin := range m.plugins {
		if err := plugin.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing plugin %s: %w", name, err))
		}
	}

	m.plugins = make(map[string]Plugin)

	if len(errs) > 0 {
		return fmt.Errorf("plugin cleanup errors: %v", errs)
	}

	return nil
}

// Global plugin registry
var globalPluginRegistry = NewPluginRegistry()

// RegisterPlugin registers a plugin factory in the global registry.
func RegisterPlugin(name string, factory PluginFactory) error {
	return globalPluginRegistry.Register(name, factory)
}

// MustRegisterPlugin registers a plugin factory and panics on error.
func MustRegisterPlugin(name string, factory PluginFactory) {
	globalPluginRegistry.MustRegister(name, factory)
}

// GetPluginFactory retrieves a plugin factory from the global registry.
func GetPluginFactory(name string) PluginFactory {
	return globalPluginRegistry.Get(name)
}

// ListPlugins returns all registered plugin names.
func ListPlugins() []string {
	return globalPluginRegistry.List()
}

// GetGlobalPluginRegistry returns the global plugin registry.
func GetGlobalPluginRegistry() *PluginRegistry {
	return globalPluginRegistry
}
