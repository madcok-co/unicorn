// Package multitenancy provides multi-tenant isolation strategies for Unicorn Framework.
//
// Supports multiple tenant identification strategies:
//   - Subdomain (tenant1.app.com, tenant2.app.com)
//   - Header (X-Tenant-ID: tenant1)
//   - Path (/tenant1/users, /tenant2/users)
//   - Custom resolver
//
// Usage:
//
//	import (
//	    "github.com/madcok-co/unicorn/contrib/multitenancy"
//	)
//
//	// Initialize multi-tenancy
//	mt := multitenancy.NewDriver(&multitenancy.Config{
//	    Strategy: multitenancy.StrategySubdomain,
//	    Store:    &myTenantStore{},
//	})
//
//	// Use in middleware
//	app.Use(middleware.MultiTenant(mt))
package multitenancy

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// Strategy represents tenant identification strategy
type Strategy string

const (
	StrategySubdomain Strategy = "subdomain"
	StrategyHeader    Strategy = "header"
	StrategyPath      Strategy = "path"
	StrategyCustom    Strategy = "custom"
)

// Driver implements multi-tenancy isolation
type Driver struct {
	config *Config
}

// Config for multi-tenancy driver
type Config struct {
	// Identification strategy
	Strategy Strategy

	// Header name for StrategyHeader (default: "X-Tenant-ID")
	HeaderName string

	// Path prefix for StrategyPath (default: first segment)
	PathPrefix string

	// Tenant store for validation and metadata
	Store TenantStore

	// Custom resolver for StrategyCustom
	Resolver TenantResolver

	// Allow missing tenant (optional tenant support)
	AllowMissing bool

	// Default tenant ID when missing
	DefaultTenant string
}

// Tenant represents a tenant
type Tenant struct {
	ID       string
	Name     string
	Domain   string // For subdomain strategy
	Metadata map[string]any

	// Database isolation
	DatabaseName string // Separate database per tenant
	SchemaName   string // Separate schema per tenant

	// Feature flags per tenant
	Features map[string]bool

	// Status
	Active   bool
	Disabled bool
}

// TenantStore interface for tenant persistence
type TenantStore interface {
	// GetTenant retrieves tenant by ID
	GetTenant(ctx context.Context, id string) (*Tenant, error)

	// GetTenantByDomain retrieves tenant by domain (for subdomain strategy)
	GetTenantByDomain(ctx context.Context, domain string) (*Tenant, error)

	// ListTenants lists all active tenants
	ListTenants(ctx context.Context) ([]*Tenant, error)

	// CreateTenant creates a new tenant
	CreateTenant(ctx context.Context, tenant *Tenant) error

	// UpdateTenant updates tenant
	UpdateTenant(ctx context.Context, tenant *Tenant) error

	// DeleteTenant deletes tenant
	DeleteTenant(ctx context.Context, id string) error
}

// TenantResolver is custom function to resolve tenant from request
type TenantResolver func(r *http.Request) (string, error)

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Strategy:   StrategyHeader,
		HeaderName: "X-Tenant-ID",
	}
}

// NewDriver creates a new multi-tenancy driver
func NewDriver(cfg *Config) *Driver {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Tenant-ID"
	}

	return &Driver{
		config: cfg,
	}
}

// ResolveTenant resolves tenant ID from HTTP request
func (d *Driver) ResolveTenant(r *http.Request) (string, error) {
	var tenantID string
	var err error

	switch d.config.Strategy {
	case StrategySubdomain:
		tenantID, err = d.resolveFromSubdomain(r)
	case StrategyHeader:
		tenantID, err = d.resolveFromHeader(r)
	case StrategyPath:
		tenantID, err = d.resolveFromPath(r)
	case StrategyCustom:
		if d.config.Resolver == nil {
			return "", fmt.Errorf("custom resolver not configured")
		}
		tenantID, err = d.config.Resolver(r)
	default:
		return "", fmt.Errorf("unknown strategy: %s", d.config.Strategy)
	}

	if err != nil {
		if d.config.AllowMissing && d.config.DefaultTenant != "" {
			return d.config.DefaultTenant, nil
		}
		return "", err
	}

	if tenantID == "" {
		if d.config.AllowMissing {
			return d.config.DefaultTenant, nil
		}
		return "", fmt.Errorf("tenant not found")
	}

	return tenantID, nil
}

// GetTenant retrieves tenant by ID
func (d *Driver) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	if d.config.Store == nil {
		// Return basic tenant without store
		return &Tenant{
			ID:     id,
			Active: true,
		}, nil
	}

	tenant, err := d.config.Store.GetTenant(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	if tenant.Disabled {
		return nil, fmt.Errorf("tenant is disabled: %s", id)
	}

	if !tenant.Active {
		return nil, fmt.Errorf("tenant is not active: %s", id)
	}

	return tenant, nil
}

// GetTenantFromRequest resolves and retrieves tenant from request
func (d *Driver) GetTenantFromRequest(ctx context.Context, r *http.Request) (*Tenant, error) {
	tenantID, err := d.ResolveTenant(r)
	if err != nil {
		return nil, err
	}

	return d.GetTenant(ctx, tenantID)
}

// resolveFromSubdomain extracts tenant from subdomain
func (d *Driver) resolveFromSubdomain(r *http.Request) (string, error) {
	host := r.Host
	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Split by dots
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid subdomain format")
	}

	// First part is tenant
	tenantID := parts[0]

	// Validate with store if configured
	if d.config.Store != nil {
		tenant, err := d.config.Store.GetTenantByDomain(r.Context(), tenantID)
		if err != nil {
			return "", fmt.Errorf("tenant not found for domain: %s", tenantID)
		}
		return tenant.ID, nil
	}

	return tenantID, nil
}

// resolveFromHeader extracts tenant from header
func (d *Driver) resolveFromHeader(r *http.Request) (string, error) {
	tenantID := r.Header.Get(d.config.HeaderName)
	if tenantID == "" {
		return "", fmt.Errorf("tenant header not found: %s", d.config.HeaderName)
	}
	return tenantID, nil
}

// resolveFromPath extracts tenant from URL path
func (d *Driver) resolveFromPath(r *http.Request) (string, error) {
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) == 0 {
		return "", fmt.Errorf("invalid path format")
	}

	// First segment is tenant
	tenantID := parts[0]
	return tenantID, nil
}

// CreateTenant creates a new tenant
func (d *Driver) CreateTenant(ctx context.Context, tenant *Tenant) error {
	if d.config.Store == nil {
		return fmt.Errorf("tenant store not configured")
	}

	if tenant.ID == "" {
		return fmt.Errorf("tenant ID is required")
	}

	tenant.Active = true
	return d.config.Store.CreateTenant(ctx, tenant)
}

// UpdateTenant updates tenant
func (d *Driver) UpdateTenant(ctx context.Context, tenant *Tenant) error {
	if d.config.Store == nil {
		return fmt.Errorf("tenant store not configured")
	}

	return d.config.Store.UpdateTenant(ctx, tenant)
}

// DeleteTenant deletes tenant
func (d *Driver) DeleteTenant(ctx context.Context, id string) error {
	if d.config.Store == nil {
		return fmt.Errorf("tenant store not configured")
	}

	return d.config.Store.DeleteTenant(ctx, id)
}

// ListTenants lists all active tenants
func (d *Driver) ListTenants(ctx context.Context) ([]*Tenant, error) {
	if d.config.Store == nil {
		return nil, fmt.Errorf("tenant store not configured")
	}

	return d.config.Store.ListTenants(ctx)
}

// HasFeature checks if tenant has a feature enabled
func (t *Tenant) HasFeature(feature string) bool {
	if t.Features == nil {
		return false
	}
	enabled, ok := t.Features[feature]
	return ok && enabled
}

// Context keys
type contextKey string

const (
	TenantIDKey contextKey = "tenant_id"
	TenantKey   contextKey = "tenant"
)

// GetTenantIDFromContext extracts tenant ID from context
func GetTenantIDFromContext(ctx context.Context) (string, bool) {
	tenantID, ok := ctx.Value(TenantIDKey).(string)
	return tenantID, ok
}

// SetTenantIDInContext stores tenant ID in context
func SetTenantIDInContext(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, TenantIDKey, tenantID)
}

// GetTenantFromContext extracts tenant from context
func GetTenantFromContext(ctx context.Context) (*Tenant, bool) {
	tenant, ok := ctx.Value(TenantKey).(*Tenant)
	return tenant, ok
}

// SetTenantInContext stores tenant in context
func SetTenantInContext(ctx context.Context, tenant *Tenant) context.Context {
	return context.WithValue(ctx, TenantKey, tenant)
}
