package rbac

import (
	"context"
	"fmt"
	"testing"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

func TestNewDriver(t *testing.T) {
	cfg := &Config{
		Roles: map[string]*Role{
			"admin": {
				Name:        "admin",
				Permissions: []string{"*"},
			},
		},
	}

	driver := NewDriver(cfg)

	if driver == nil {
		t.Fatal("expected driver to be non-nil")
	}

	if len(driver.config.Roles) != 1 {
		t.Errorf("expected 1 role, got %d", len(driver.config.Roles))
	}
}

func TestAuthorize_AdminWildcard(t *testing.T) {
	driver := NewDriver(&Config{
		Roles: map[string]*Role{
			"admin": {
				Name:        "admin",
				Permissions: []string{"*"},
			},
		},
		AllowWildcard: true,
	})

	identity := &contracts.Identity{
		ID:    "1",
		Roles: []string{"admin"},
	}

	allowed, err := driver.Authorize(context.Background(), identity, "delete", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !allowed {
		t.Error("expected admin with wildcard to be allowed")
	}
}

func TestAuthorize_ExactMatch(t *testing.T) {
	driver := NewDriver(&Config{
		Roles: map[string]*Role{
			"user": {
				Name:        "user",
				Permissions: []string{"read:users", "write:posts"},
			},
		},
	})

	identity := &contracts.Identity{
		ID:    "1",
		Roles: []string{"user"},
	}

	// Should be allowed
	allowed, err := driver.Authorize(context.Background(), identity, "read", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected exact match to be allowed")
	}

	// Should be denied
	allowed, err = driver.Authorize(context.Background(), identity, "delete", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("expected non-matching permission to be denied")
	}
}

func TestAuthorize_ResourceWildcard(t *testing.T) {
	driver := NewDriver(&Config{
		Roles: map[string]*Role{
			"editor": {
				Name:        "editor",
				Permissions: []string{"read:*", "write:posts"},
			},
		},
		AllowWildcard: true,
	})

	identity := &contracts.Identity{
		ID:    "1",
		Roles: []string{"editor"},
	}

	// Should match "read:*"
	tests := []struct {
		action   string
		resource string
		expected bool
	}{
		{"read", "users", true},
		{"read", "posts", true},
		{"read", "comments", true},
		{"write", "posts", true},
		{"write", "users", false},
		{"delete", "posts", false},
	}

	for _, tt := range tests {
		t.Run(tt.action+":"+tt.resource, func(t *testing.T) {
			allowed, err := driver.Authorize(context.Background(), identity, tt.action, tt.resource)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if allowed != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, allowed)
			}
		})
	}
}

func TestAuthorize_RoleInheritance(t *testing.T) {
	driver := NewDriver(&Config{
		Roles: map[string]*Role{
			"basic": {
				Name:        "basic",
				Permissions: []string{"read:posts"},
			},
			"member": {
				Name:        "member",
				Permissions: []string{"write:posts"},
				Inherits:    []string{"basic"},
			},
			"moderator": {
				Name:        "moderator",
				Permissions: []string{"delete:posts"},
				Inherits:    []string{"member"},
			},
		},
	})

	identity := &contracts.Identity{
		ID:    "1",
		Roles: []string{"moderator"},
	}

	// Should inherit permissions from member and basic
	tests := []struct {
		action   string
		resource string
		expected bool
	}{
		{"read", "posts", true},   // from basic
		{"write", "posts", true},  // from member
		{"delete", "posts", true}, // from moderator
		{"admin", "posts", false}, // not granted
	}

	for _, tt := range tests {
		t.Run(tt.action+":"+tt.resource, func(t *testing.T) {
			allowed, err := driver.Authorize(context.Background(), identity, tt.action, tt.resource)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if allowed != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, allowed)
			}
		})
	}
}

func TestAuthorize_MultipleRoles(t *testing.T) {
	driver := NewDriver(&Config{
		Roles: map[string]*Role{
			"reader": {
				Name:        "reader",
				Permissions: []string{"read:users", "read:posts"},
			},
			"writer": {
				Name:        "writer",
				Permissions: []string{"write:posts"},
			},
		},
	})

	identity := &contracts.Identity{
		ID:    "1",
		Roles: []string{"reader", "writer"},
	}

	tests := []struct {
		action   string
		resource string
		expected bool
	}{
		{"read", "users", true},
		{"read", "posts", true},
		{"write", "posts", true},
		{"delete", "posts", false},
	}

	for _, tt := range tests {
		t.Run(tt.action+":"+tt.resource, func(t *testing.T) {
			allowed, err := driver.Authorize(context.Background(), identity, tt.action, tt.resource)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if allowed != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, allowed)
			}
		})
	}
}

func TestAuthorize_NilIdentity(t *testing.T) {
	driver := NewDriver(DefaultConfig())

	_, err := driver.Authorize(context.Background(), nil, "read", "users")
	if err == nil {
		t.Error("expected error for nil identity")
	}
}

func TestAuthorize_NoRoles(t *testing.T) {
	driver := NewDriver(DefaultConfig())

	identity := &contracts.Identity{
		ID:    "1",
		Roles: []string{},
	}

	allowed, err := driver.Authorize(context.Background(), identity, "read", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if allowed {
		t.Error("expected identity with no roles to be denied")
	}
}

func TestAuthorizeAll(t *testing.T) {
	driver := NewDriver(&Config{
		Roles: map[string]*Role{
			"user": {
				Name:        "user",
				Permissions: []string{"read:users", "write:posts", "read:posts"},
			},
		},
	})

	identity := &contracts.Identity{
		ID:    "1",
		Roles: []string{"user"},
	}

	permissions := []contracts.Permission{
		{Action: "read", Resource: "users"},
		{Action: "read", Resource: "posts"},
	}

	allowed, err := driver.AuthorizeAll(context.Background(), identity, permissions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !allowed {
		t.Error("expected all permissions to be allowed")
	}

	// Test with one denied permission
	permissions = append(permissions, contracts.Permission{Action: "delete", Resource: "users"})
	allowed, err = driver.AuthorizeAll(context.Background(), identity, permissions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if allowed {
		t.Error("expected at least one permission to be denied")
	}
}

func TestAuthorizeAll_EmptyPermissions(t *testing.T) {
	driver := NewDriver(DefaultConfig())

	identity := &contracts.Identity{
		ID:    "1",
		Roles: []string{"user"},
	}

	allowed, err := driver.AuthorizeAll(context.Background(), identity, []contracts.Permission{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !allowed {
		t.Error("expected empty permissions to be allowed")
	}
}

func TestAddRole(t *testing.T) {
	driver := NewDriver(DefaultConfig())

	role := &Role{
		Name:        "new-role",
		Permissions: []string{"read:data"},
	}

	err := driver.AddRole(role)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retrievedRole, err := driver.GetRole(context.Background(), "new-role")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrievedRole.Name != "new-role" {
		t.Errorf("expected role name to be new-role, got %s", retrievedRole.Name)
	}
}

func TestAddRole_Invalid(t *testing.T) {
	driver := NewDriver(DefaultConfig())

	err := driver.AddRole(nil)
	if err == nil {
		t.Error("expected error for nil role")
	}

	err = driver.AddRole(&Role{Name: ""})
	if err == nil {
		t.Error("expected error for empty role name")
	}
}

func TestRemoveRole(t *testing.T) {
	driver := NewDriver(&Config{
		Roles: map[string]*Role{
			"temp": {Name: "temp"},
		},
	})

	err := driver.RemoveRole("temp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = driver.GetRole(context.Background(), "temp")
	if err == nil {
		t.Error("expected error when getting removed role")
	}
}

func TestAddPermissionToRole(t *testing.T) {
	driver := NewDriver(&Config{
		Roles: map[string]*Role{
			"user": {
				Name:        "user",
				Permissions: []string{"read:users"},
			},
		},
	})

	err := driver.AddPermissionToRole("user", "write:users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	role, _ := driver.GetRole(context.Background(), "user")
	if len(role.Permissions) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(role.Permissions))
	}
}

func TestRemovePermissionFromRole(t *testing.T) {
	driver := NewDriver(&Config{
		Roles: map[string]*Role{
			"user": {
				Name:        "user",
				Permissions: []string{"read:users", "write:users"},
			},
		},
	})

	err := driver.RemovePermissionFromRole("user", "write:users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	role, _ := driver.GetRole(context.Background(), "user")
	if len(role.Permissions) != 1 {
		t.Errorf("expected 1 permission, got %d", len(role.Permissions))
	}

	if role.Permissions[0] != "read:users" {
		t.Errorf("expected read:users, got %s", role.Permissions[0])
	}
}

func TestCaseSensitive(t *testing.T) {
	// Case insensitive (default)
	driver := NewDriver(&Config{
		Roles: map[string]*Role{
			"user": {
				Name:        "user",
				Permissions: []string{"READ:Users"},
			},
		},
		CaseSensitive: false,
	})

	identity := &contracts.Identity{
		ID:    "1",
		Roles: []string{"user"},
	}

	allowed, err := driver.Authorize(context.Background(), identity, "read", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected case-insensitive match to be allowed")
	}

	// Case sensitive
	driverCS := NewDriver(&Config{
		Roles: map[string]*Role{
			"user": {
				Name:        "user",
				Permissions: []string{"READ:Users"},
			},
		},
		CaseSensitive: true,
	})

	allowed, err = driverCS.Authorize(context.Background(), identity, "read", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("expected case-sensitive mismatch to be denied")
	}
}

func TestListRoles(t *testing.T) {
	driver := NewDriver(&Config{
		Roles: map[string]*Role{
			"admin":  {Name: "admin"},
			"user":   {Name: "user"},
			"editor": {Name: "editor"},
		},
	})

	roles := driver.ListRoles()
	if len(roles) != 3 {
		t.Errorf("expected 3 roles, got %d", len(roles))
	}
}

func TestGetPermissionsForIdentity(t *testing.T) {
	driver := NewDriver(&Config{
		Roles: map[string]*Role{
			"user": {
				Name:        "user",
				Permissions: []string{"read:users", "write:posts"},
			},
		},
	})

	identity := &contracts.Identity{
		ID:    "1",
		Roles: []string{"user"},
	}

	perms, err := driver.GetPermissionsForIdentity(context.Background(), identity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(perms) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(perms))
	}
}

func TestWildcardMatching(t *testing.T) {
	driver := NewDriver(&Config{
		AllowWildcard: true,
	})

	tests := []struct {
		pattern  string
		value    string
		expected bool
	}{
		{"*", "anything", true},
		{"users:*", "users:read", true},
		{"users:*", "users:write", true},
		{"users:*", "posts:read", false},
		{"*:own", "users:own", true},
		{"*:own", "posts:own", true},
		{"*:own", "users:all", false},
		{"users:*:own", "users:read:own", true},
		{"users:*:own", "users:write:own", true},
		{"users:*:own", "users:read:all", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.value, func(t *testing.T) {
			result := driver.matchWildcard(tt.pattern, tt.value)
			if result != tt.expected {
				t.Errorf("matchWildcard(%q, %q) = %v, expected %v", tt.pattern, tt.value, result, tt.expected)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Roles == nil {
		t.Error("expected default roles to be initialized")
	}

	if cfg.CaseSensitive {
		t.Error("expected case insensitive by default")
	}

	if !cfg.AllowWildcard {
		t.Error("expected wildcard to be allowed by default")
	}
}

// Mock RoleProvider for testing
type mockRoleProvider struct {
	roles map[string]*Role
}

func (m *mockRoleProvider) GetRole(ctx context.Context, name string) (*Role, error) {
	role, exists := m.roles[name]
	if !exists {
		return nil, fmt.Errorf("role not found: %s", name)
	}
	return role, nil
}

func (m *mockRoleProvider) GetRoles(ctx context.Context, names []string) ([]*Role, error) {
	roles := make([]*Role, 0, len(names))
	for _, name := range names {
		role, err := m.GetRole(ctx, name)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, nil
}

func TestDynamicRoleProvider(t *testing.T) {
	provider := &mockRoleProvider{
		roles: map[string]*Role{
			"dynamic": {
				Name:        "dynamic",
				Permissions: []string{"read:dynamic"},
			},
		},
	}

	driver := NewDriver(&Config{
		Roles:        map[string]*Role{},
		RoleProvider: provider,
	})

	identity := &contracts.Identity{
		ID:    "1",
		Roles: []string{"dynamic"},
	}

	allowed, err := driver.Authorize(context.Background(), identity, "read", "dynamic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !allowed {
		t.Error("expected dynamic role to be loaded and allowed")
	}
}
