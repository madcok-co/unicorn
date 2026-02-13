# RBAC (Role-Based Access Control) Authorization Driver

Production-ready RBAC implementation for Unicorn Framework with support for role inheritance, wildcards, and dynamic role loading.

## Features

- **Role-Based Permissions** - Define permissions per role
- **Resource-Level Control** - Fine-grained resource permissions
- **Wildcard Permissions** - Support for `*` wildcards (e.g., `users:*`, `*:own`)
- **Role Inheritance** - Hierarchical roles with permission inheritance
- **Dynamic Loading** - Load roles from database/external source
- **Thread-Safe** - Concurrent-safe role updates
- **Case Sensitivity** - Configurable case-sensitive permissions

## Installation

```bash
go get github.com/madcok-co/unicorn/contrib/authz/rbac
```

## Quick Start

```go
import (
    "github.com/madcok-co/unicorn/contrib/authz/rbac"
    "github.com/madcok-co/unicorn/core/pkg/app"
)

func main() {
    // Initialize RBAC
    authz := rbac.NewDriver(&rbac.Config{
        Roles: map[string]*rbac.Role{
            "admin": {
                Name:        "admin",
                Permissions: []string{"*"}, // All permissions
            },
            "user": {
                Name: "user",
                Permissions: []string{
                    "read:users",
                    "write:posts",
                    "read:posts",
                },
            },
        },
        AllowWildcard: true,
    })

    // Create app and set authorizer
    application := app.New(&app.Config{
        Name:       "my-app",
        EnableHTTP: true,
    })
    
    application.SetAuthz(authz)
    
    // Register protected handlers
    application.RegisterHandler(GetUsers).HTTP("GET", "/users").Done()
    application.RegisterHandler(DeleteUser).HTTP("DELETE", "/users/:id").Done()
    
    application.Start()
}
```

## Permission Format

Permissions follow the format: `action:resource`

Examples:
- `read:users` - Read users
- `write:posts` - Write posts
- `delete:comments` - Delete comments
- `admin:system` - Admin system access

## Wildcard Permissions

### Full Wildcard
```go
Permissions: []string{"*"} // All permissions
```

### Resource Wildcard
```go
Permissions: []string{"read:*"} // Read all resources
```

### Action Wildcard
```go
Permissions: []string{"*:own"} // All actions on own resources
```

### Complex Wildcard
```go
Permissions: []string{"users:*:own"} // All user actions on own resources
```

## Role Inheritance

Roles can inherit permissions from parent roles:

```go
authz := rbac.NewDriver(&rbac.Config{
    Roles: map[string]*rbac.Role{
        "basic": {
            Name:        "basic",
            Permissions: []string{"read:posts"},
        },
        "member": {
            Name:        "member",
            Permissions: []string{"write:posts"},
            Inherits:    []string{"basic"}, // Inherits read:posts
        },
        "moderator": {
            Name:        "moderator",
            Permissions: []string{"delete:posts"},
            Inherits:    []string{"member"}, // Inherits write:posts and read:posts
        },
    },
})
```

## Usage in Handlers

### Check Single Permission

```go
func DeleteUser(ctx *context.Context, req DeleteUserRequest) (map[string]string, error) {
    // Get identity from context (set by auth middleware)
    identity, ok := contracts.GetIdentityFromContext(ctx.Context())
    if !ok {
        return nil, fmt.Errorf("unauthorized")
    }
    
    // Check authorization
    allowed, err := ctx.Authz().Authorize(ctx.Context(), identity, "delete", "users")
    if err != nil {
        return nil, fmt.Errorf("authorization error: %w", err)
    }
    
    if !allowed {
        return nil, fmt.Errorf("forbidden: insufficient permissions")
    }
    
    // Proceed with deletion
    // ...
    
    return map[string]string{"message": "user deleted"}, nil
}
```

### Check Multiple Permissions

```go
func UpdateUserRole(ctx *context.Context, req UpdateRoleRequest) error {
    identity, _ := contracts.GetIdentityFromContext(ctx.Context())
    
    permissions := []contracts.Permission{
        {Action: "read", Resource: "users"},
        {Action: "write", Resource: "users"},
        {Action: "admin", Resource: "roles"},
    }
    
    allowed, err := ctx.Authz().AuthorizeAll(ctx.Context(), identity, permissions)
    if err != nil {
        return fmt.Errorf("authorization error: %w", err)
    }
    
    if !allowed {
        return fmt.Errorf("forbidden: missing required permissions")
    }
    
    // Update role
    // ...
    
    return nil
}
```

## Dynamic Role Management

### Add Role at Runtime

```go
newRole := &rbac.Role{
    Name: "editor",
    Permissions: []string{
        "read:posts",
        "write:posts",
        "update:posts",
    },
}

authz.AddRole(newRole)
```

### Remove Role

```go
authz.RemoveRole("editor")
```

### Add Permission to Role

```go
authz.AddPermissionToRole("editor", "delete:posts")
```

### Remove Permission from Role

```go
authz.RemovePermissionFromRole("editor", "delete:posts")
```

## Dynamic Role Provider

Load roles from database or external source:

```go
type DBRoleProvider struct {
    db contracts.Database
}

func (p *DBRoleProvider) GetRole(ctx context.Context, name string) (*rbac.Role, error) {
    var role rbac.Role
    err := p.db.First(ctx, &role, "name = ?", name)
    if err != nil {
        return nil, err
    }
    return &role, nil
}

func (p *DBRoleProvider) GetRoles(ctx context.Context, names []string) ([]*rbac.Role, error) {
    var roles []*rbac.Role
    err := p.db.Find(ctx, &roles, "name IN ?", names)
    return roles, err
}

// Use provider
authz := rbac.NewDriver(&rbac.Config{
    RoleProvider: &DBRoleProvider{db: db},
})
```

## Complete Example

```go
package main

import (
    "fmt"
    
    "github.com/madcok-co/unicorn/contrib/authz/rbac"
    "github.com/madcok-co/unicorn/core/pkg/app"
    "github.com/madcok-co/unicorn/core/pkg/context"
    "github.com/madcok-co/unicorn/core/pkg/contracts"
    httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
)

func main() {
    // Initialize RBAC with roles
    authz := rbac.NewDriver(&rbac.Config{
        Roles: map[string]*rbac.Role{
            "admin": {
                Name:        "admin",
                Permissions: []string{"*"},
            },
            "moderator": {
                Name: "moderator",
                Permissions: []string{
                    "read:*",
                    "write:posts",
                    "update:posts",
                    "delete:comments",
                },
            },
            "user": {
                Name: "user",
                Permissions: []string{
                    "read:posts",
                    "write:posts:own",
                    "read:comments",
                    "write:comments",
                },
            },
        },
        AllowWildcard: true,
    })

    // Create app
    application := app.New(&app.Config{
        Name:       "rbac-example",
        EnableHTTP: true,
        HTTP:       &httpAdapter.Config{Port: 8080},
    })

    // Set authorizer
    application.SetAuthz(authz)

    // Register handlers
    application.RegisterHandler(GetPosts).HTTP("GET", "/posts").Done()
    application.RegisterHandler(CreatePost).HTTP("POST", "/posts").Done()
    application.RegisterHandler(DeletePost).HTTP("DELETE", "/posts/:id").Done()

    application.Start()
}

// GetPosts - anyone can read posts
func GetPosts(ctx *context.Context, req struct{}) ([]Post, error) {
    identity, ok := contracts.GetIdentityFromContext(ctx.Context())
    if !ok {
        return nil, fmt.Errorf("unauthorized")
    }
    
    // Check read permission
    allowed, err := ctx.Authz().Authorize(ctx.Context(), identity, "read", "posts")
    if err != nil || !allowed {
        return nil, fmt.Errorf("forbidden")
    }
    
    // Fetch posts
    var posts []Post
    // ... fetch from database
    
    return posts, nil
}

// CreatePost - users can create posts
type CreatePostRequest struct {
    Title   string `json:"title"`
    Content string `json:"content"`
}

func CreatePost(ctx *context.Context, req CreatePostRequest) (*Post, error) {
    identity, ok := contracts.GetIdentityFromContext(ctx.Context())
    if !ok {
        return nil, fmt.Errorf("unauthorized")
    }
    
    // Check write permission
    allowed, err := ctx.Authz().Authorize(ctx.Context(), identity, "write", "posts")
    if err != nil || !allowed {
        return nil, fmt.Errorf("forbidden")
    }
    
    // Create post
    post := &Post{
        Title:    req.Title,
        Content:  req.Content,
        AuthorID: identity.ID,
    }
    
    // ... save to database
    
    return post, nil
}

// DeletePost - only admins and moderators can delete
type DeletePostRequest struct {
    ID string `path:"id"`
}

func DeletePost(ctx *context.Context, req DeletePostRequest) (map[string]string, error) {
    identity, ok := contracts.GetIdentityFromContext(ctx.Context())
    if !ok {
        return nil, fmt.Errorf("unauthorized")
    }
    
    // Check delete permission
    allowed, err := ctx.Authz().Authorize(ctx.Context(), identity, "delete", "posts")
    if err != nil || !allowed {
        return nil, fmt.Errorf("forbidden: only admins and moderators can delete posts")
    }
    
    // Delete post
    // ... delete from database
    
    return map[string]string{"message": "post deleted"}, nil
}

type Post struct {
    ID       string
    Title    string
    Content  string
    AuthorID string
}
```

## Configuration Options

```go
type Config struct {
    // Predefined roles
    Roles map[string]*Role

    // Case sensitive permissions (default: false)
    CaseSensitive bool

    // Allow wildcard permissions (default: true)
    AllowWildcard bool

    // Role provider for dynamic loading
    RoleProvider RoleProvider
}
```

## Common Permission Patterns

### CRUD Operations
```go
"create:users"
"read:users"
"update:users"
"delete:users"
```

### Resource Ownership
```go
"read:posts:own"    // Read own posts
"write:posts:own"   // Write own posts
"delete:posts:own"  // Delete own posts
```

### Admin Permissions
```go
"admin:users"       // User administration
"admin:system"      // System administration
"admin:*"           // All admin operations
```

### Multi-Tenant
```go
"read:tenants:123"      // Read specific tenant
"write:tenants:123"     // Write to specific tenant
"admin:tenants:*"       // Admin all tenants
```

## Integration with Middleware

Create authorization middleware:

```go
func RequirePermission(action, resource string) middleware.Middleware {
    return func(next contracts.Handler) contracts.Handler {
        return func(ctx *context.Context, req any) (any, error) {
            identity, ok := contracts.GetIdentityFromContext(ctx.Context())
            if !ok {
                return nil, fmt.Errorf("unauthorized")
            }
            
            allowed, err := ctx.Authz().Authorize(ctx.Context(), identity, action, resource)
            if err != nil {
                return nil, fmt.Errorf("authorization error: %w", err)
            }
            
            if !allowed {
                return nil, fmt.Errorf("forbidden: insufficient permissions")
            }
            
            return next(ctx, req)
        }
    }
}

// Use in handlers
application.RegisterHandler(DeleteUser).
    HTTP("DELETE", "/users/:id").
    Use(RequirePermission("delete", "users")).
    Done()
```

## Testing

```bash
# Run tests
cd contrib/authz/rbac
go test -v

# Run with coverage
go test -v -cover

# Run specific test
go test -v -run TestAuthorize_RoleInheritance
```

## Best Practices

1. **Principle of Least Privilege** - Grant minimum permissions needed
2. **Use Role Inheritance** - Create hierarchical roles to avoid duplication
3. **Resource-Specific Permissions** - Be specific about resources
4. **Audit Permissions** - Regularly review and update role permissions
5. **Test Permissions** - Write tests for authorization logic
6. **Use Wildcards Carefully** - Only for admin/superuser roles

## License

MIT License - see LICENSE file for details
