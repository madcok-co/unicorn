// Package rbac provides Role-Based Access Control (RBAC) implementation of the unicorn Authorizer interface.
//
// Supports:
//   - Role-based permissions
//   - Resource-level permissions
//   - Wildcard permissions
//   - Role inheritance (hierarchical roles)
//   - Dynamic permission loading
//
// Usage:
//
//	import (
//	    "github.com/madcok-co/unicorn/contrib/authz/rbac"
//	)
//
//	// Initialize RBAC
//	authz := rbac.NewDriver(&rbac.Config{
//	    Roles: map[string]*rbac.Role{
//	        "admin": {
//	            Name: "admin",
//	            Permissions: []string{"*"}, // All permissions
//	        },
//	        "user": {
//	            Name: "user",
//	            Permissions: []string{"users:read", "users:update:own"},
//	        },
//	    },
//	})
//
//	app.SetAuthz(authz)
package rbac

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// Driver implements contracts.Authorizer using RBAC
type Driver struct {
	config *Config
	mu     sync.RWMutex // For thread-safe role updates
}

// Config for creating a new RBAC driver
type Config struct {
	// Predefined roles
	Roles map[string]*Role

	// Case sensitive permissions
	CaseSensitive bool

	// Allow wildcard permissions (e.g., "users:*", "*")
	AllowWildcard bool

	// Role provider for dynamic loading
	RoleProvider RoleProvider
}

// Role represents a role with permissions
type Role struct {
	Name        string
	Permissions []string
	Inherits    []string // Parent roles to inherit from
	Metadata    map[string]any
}

// RoleProvider interface for dynamic role loading
type RoleProvider interface {
	// GetRole loads a role by name
	GetRole(ctx context.Context, name string) (*Role, error)

	// GetRoles loads multiple roles
	GetRoles(ctx context.Context, names []string) ([]*Role, error)
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		Roles:         make(map[string]*Role),
		CaseSensitive: false,
		AllowWildcard: true,
	}
}

// NewDriver creates a new RBAC driver
func NewDriver(cfg *Config) *Driver {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	if cfg.Roles == nil {
		cfg.Roles = make(map[string]*Role)
	}

	return &Driver{
		config: cfg,
	}
}

// Authorize checks if identity can perform action on resource
func (d *Driver) Authorize(ctx context.Context, identity *contracts.Identity, action, resource string) (bool, error) {
	if identity == nil {
		return false, fmt.Errorf("identity is nil")
	}

	// Build permission string
	permission := d.buildPermission(action, resource)

	// Check if identity has any roles
	if len(identity.Roles) == 0 {
		return false, nil
	}

	// Get all permissions for user's roles
	permissions, err := d.getPermissionsForRoles(ctx, identity.Roles)
	if err != nil {
		return false, fmt.Errorf("failed to get permissions: %w", err)
	}

	// Check if user has permission
	return d.hasPermission(permissions, permission), nil
}

// AuthorizeAll checks multiple permissions
func (d *Driver) AuthorizeAll(ctx context.Context, identity *contracts.Identity, permissions []contracts.Permission) (bool, error) {
	if identity == nil {
		return false, fmt.Errorf("identity is nil")
	}

	if len(permissions) == 0 {
		return true, nil
	}

	// Get all permissions for user's roles
	userPerms, err := d.getPermissionsForRoles(ctx, identity.Roles)
	if err != nil {
		return false, fmt.Errorf("failed to get permissions: %w", err)
	}

	// Check each permission
	for _, perm := range permissions {
		permStr := d.buildPermission(perm.Action, perm.Resource)
		if !d.hasPermission(userPerms, permStr) {
			return false, nil
		}
	}

	return true, nil
}

// AddRole adds or updates a role
func (d *Driver) AddRole(role *Role) error {
	if role == nil || role.Name == "" {
		return fmt.Errorf("invalid role")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.config.Roles[role.Name] = role
	return nil
}

// RemoveRole removes a role
func (d *Driver) RemoveRole(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.config.Roles, name)
	return nil
}

// GetRole retrieves a role by name
func (d *Driver) GetRole(ctx context.Context, name string) (*Role, error) {
	d.mu.RLock()
	role, exists := d.config.Roles[name]
	d.mu.RUnlock()

	if exists {
		return role, nil
	}

	// Try dynamic provider if configured
	if d.config.RoleProvider != nil {
		return d.config.RoleProvider.GetRole(ctx, name)
	}

	return nil, fmt.Errorf("role not found: %s", name)
}

// AddPermissionToRole adds a permission to an existing role
func (d *Driver) AddPermissionToRole(roleName, permission string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	role, exists := d.config.Roles[roleName]
	if !exists {
		return fmt.Errorf("role not found: %s", roleName)
	}

	role.Permissions = append(role.Permissions, permission)
	return nil
}

// RemovePermissionFromRole removes a permission from a role
func (d *Driver) RemovePermissionFromRole(roleName, permission string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	role, exists := d.config.Roles[roleName]
	if !exists {
		return fmt.Errorf("role not found: %s", roleName)
	}

	newPerms := make([]string, 0, len(role.Permissions))
	for _, p := range role.Permissions {
		if p != permission {
			newPerms = append(newPerms, p)
		}
	}
	role.Permissions = newPerms
	return nil
}

// buildPermission builds permission string from action and resource
func (d *Driver) buildPermission(action, resource string) string {
	if !d.config.CaseSensitive {
		action = strings.ToLower(action)
		resource = strings.ToLower(resource)
	}
	return action + ":" + resource
}

// getPermissionsForRoles retrieves all permissions for given roles (including inherited)
func (d *Driver) getPermissionsForRoles(ctx context.Context, roleNames []string) ([]string, error) {
	visited := make(map[string]bool)
	allPermissions := make([]string, 0)

	for _, roleName := range roleNames {
		perms, err := d.getPermissionsForRole(ctx, roleName, visited)
		if err != nil {
			return nil, err
		}
		allPermissions = append(allPermissions, perms...)
	}

	return allPermissions, nil
}

// getPermissionsForRole recursively gets permissions for a role (including inherited roles)
func (d *Driver) getPermissionsForRole(ctx context.Context, roleName string, visited map[string]bool) ([]string, error) {
	// Prevent infinite recursion
	if visited[roleName] {
		return nil, nil
	}
	visited[roleName] = true

	// Get role
	role, err := d.GetRole(ctx, roleName)
	if err != nil {
		return nil, err
	}

	permissions := make([]string, len(role.Permissions))
	copy(permissions, role.Permissions)

	// Get inherited permissions
	for _, parentRole := range role.Inherits {
		parentPerms, err := d.getPermissionsForRole(ctx, parentRole, visited)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, parentPerms...)
	}

	return permissions, nil
}

// hasPermission checks if permission list contains the required permission
func (d *Driver) hasPermission(permissions []string, required string) bool {
	if !d.config.CaseSensitive {
		required = strings.ToLower(required)
	}

	for _, perm := range permissions {
		if !d.config.CaseSensitive {
			perm = strings.ToLower(perm)
		}

		// Exact match
		if perm == required {
			return true
		}

		// Wildcard matching
		if d.config.AllowWildcard && d.matchWildcard(perm, required) {
			return true
		}
	}

	return false
}

// matchWildcard checks if a wildcard permission matches required permission
func (d *Driver) matchWildcard(pattern, value string) bool {
	// Full wildcard
	if pattern == "*" {
		return true
	}

	// Wildcard at end (e.g., "users:*" matches "users:read", "users:write")
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(value, prefix)
	}

	// Wildcard in middle (e.g., "users:*:own" matches "users:read:own", "users:write:own")
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) != 2 {
			return false
		}
		return strings.HasPrefix(value, parts[0]) && strings.HasSuffix(value, parts[1])
	}

	return false
}

// ListRoles returns all configured roles
func (d *Driver) ListRoles() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	roles := make([]string, 0, len(d.config.Roles))
	for name := range d.config.Roles {
		roles = append(roles, name)
	}
	return roles
}

// GetPermissionsForIdentity returns all permissions for an identity
func (d *Driver) GetPermissionsForIdentity(ctx context.Context, identity *contracts.Identity) ([]string, error) {
	if identity == nil {
		return nil, fmt.Errorf("identity is nil")
	}

	return d.getPermissionsForRoles(ctx, identity.Roles)
}

// Ensure Driver implements contracts.Authorizer
var _ contracts.Authorizer = (*Driver)(nil)
