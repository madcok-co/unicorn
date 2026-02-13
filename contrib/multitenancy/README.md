# Multi-Tenancy Support

Production-ready multi-tenant isolation for Unicorn Framework supporting multiple tenant identification strategies and data isolation patterns.

## Features

- **Multiple Identification Strategies**
  - Subdomain (tenant1.app.com, tenant2.app.com)
  - Header (X-Tenant-ID: tenant1)
  - Path (/tenant1/users, /tenant2/users)
  - Custom resolver (query param, JWT claim, etc.)

- **Flexible Data Isolation**
  - Database per tenant
  - Schema per tenant
  - Row-level isolation with tenant_id column

- **Tenant Management**
  - Create, update, delete tenants
  - Enable/disable tenants
  - Per-tenant feature flags
  - Custom metadata

- **Thread-Safe** - Concurrent tenant operations
- **Extensible** - Custom tenant stores and resolvers

## Installation

```bash
go get github.com/madcok-co/unicorn/contrib/multitenancy
```

## Quick Start

### Header-Based Multi-Tenancy

```go
import (
    "github.com/madcok-co/unicorn/contrib/multitenancy"
    "github.com/madcok-co/unicorn/core/pkg/app"
)

func main() {
    // Initialize multi-tenancy
    mt := multitenancy.NewDriver(&multitenancy.Config{
        Strategy:   multitenancy.StrategyHeader,
        HeaderName: "X-Tenant-ID",
    })

    // Create app
    application := app.New(&app.Config{
        Name:       "multi-tenant-app",
        EnableHTTP: true,
    })

    // Register handlers
    application.RegisterHandler(GetUsers).HTTP("GET", "/users").Done()
    application.RegisterHandler(CreateUser).HTTP("POST", "/users").Done()

    application.Start()
}
```

### Subdomain-Based Multi-Tenancy

```go
mt := multitenancy.NewDriver(&multitenancy.Config{
    Strategy: multitenancy.StrategySubdomain,
    Store:    &myTenantStore{}, // Your tenant persistence
})

// Requests:
// tenant1.app.com/users → tenant1
// tenant2.app.com/users → tenant2
```

### Path-Based Multi-Tenancy

```go
mt := multitenancy.NewDriver(&multitenancy.Config{
    Strategy: multitenancy.StrategyPath,
})

// Requests:
// /tenant1/users → tenant1
// /tenant2/users → tenant2
```

## Tenant Identification Strategies

### 1. Header Strategy (Recommended for APIs)

```go
mt := multitenancy.NewDriver(&multitenancy.Config{
    Strategy:   multitenancy.StrategyHeader,
    HeaderName: "X-Tenant-ID",
})

// Client sends:
// GET /users
// Headers:
//   X-Tenant-ID: tenant1
```

**Pros:**
- Clean URLs
- Works with any endpoint
- Easy to implement in API clients

**Cons:**
- Requires client to set header
- Not browser-friendly without custom logic

### 2. Subdomain Strategy (Recommended for Web Apps)

```go
mt := multitenancy.NewDriver(&multitenancy.Config{
    Strategy: multitenancy.StrategySubdomain,
    Store:    tenantStore, // Validates subdomain
})

// Clients access:
// https://acme.app.com/dashboard
// https://contoso.app.com/dashboard
```

**Pros:**
- User-friendly URLs
- Natural tenant isolation
- Works well with wildcard SSL

**Cons:**
- Requires DNS/wildcard setup
- SSL certificate considerations

### 3. Path Strategy

```go
mt := multitenancy.NewDriver(&multitenancy.Config{
    Strategy: multitenancy.StrategyPath,
})

// Clients access:
// /acme/dashboard
// /contoso/dashboard
```

**Pros:**
- Simple to implement
- No DNS configuration needed
- Works with single domain

**Cons:**
- Longer URLs
- Tenant ID visible in all URLs

### 4. Custom Strategy

```go
mt := multitenancy.NewDriver(&multitenancy.Config{
    Strategy: multitenancy.StrategyCustom,
    Resolver: func(r *http.Request) (string, error) {
        // Extract from JWT claim
        token := extractJWT(r)
        claims := parseJWT(token)
        return claims["tenant_id"].(string), nil
    },
})
```

## Using Multi-Tenancy in Handlers

### Resolve Tenant from Request

```go
func GetUsers(ctx *context.Context, req struct{}) ([]User, error) {
    // Get multi-tenancy driver
    mt := ctx.Service("multitenancy").(*multitenancy.Driver)
    
    // Resolve tenant from request
    tenantID, err := mt.ResolveTenant(ctx.Request())
    if err != nil {
        return nil, fmt.Errorf("invalid tenant: %w", err)
    }
    
    // Use tenant ID in query
    var users []User
    ctx.DB().Find(ctx.Context(), &users, "tenant_id = ?", tenantID)
    
    return users, nil
}
```

### Get Full Tenant Object

```go
func GetTenantInfo(ctx *context.Context, req struct{}) (*multitenancy.Tenant, error) {
    mt := ctx.Service("multitenancy").(*multitenancy.Driver)
    
    // Get complete tenant with metadata
    tenant, err := mt.GetTenantFromRequest(ctx.Context(), ctx.Request())
    if err != nil {
        return nil, err
    }
    
    return tenant, nil
}
```

### Check Tenant Features

```go
func AdvancedAnalytics(ctx *context.Context, req struct{}) (interface{}, error) {
    mt := ctx.Service("multitenancy").(*multitenancy.Driver)
    tenant, _ := mt.GetTenantFromRequest(ctx.Context(), ctx.Request())
    
    // Check if tenant has feature enabled
    if !tenant.HasFeature("advanced-analytics") {
        return nil, fmt.Errorf("feature not available for your plan")
    }
    
    // Proceed with advanced analytics
    // ...
}
```

## Data Isolation Strategies

### 1. Row-Level Isolation (Shared Database)

Most common approach - all tenants share same database with tenant_id column:

```go
type User struct {
    ID       string `gorm:"primaryKey"`
    TenantID string `gorm:"index"` // Tenant isolation
    Name     string
    Email    string
}

func CreateUser(ctx *context.Context, req CreateUserRequest) (*User, error) {
    mt := ctx.Service("multitenancy").(*multitenancy.Driver)
    tenantID, _ := mt.ResolveTenant(ctx.Request())
    
    user := &User{
        TenantID: tenantID,
        Name:     req.Name,
        Email:    req.Email,
    }
    
    ctx.DB().Create(ctx.Context(), user)
    return user, nil
}

func GetUsers(ctx *context.Context, req struct{}) ([]User, error) {
    mt := ctx.Service("multitenancy").(*multitenancy.Driver)
    tenantID, _ := mt.ResolveTenant(ctx.Request())
    
    var users []User
    // Always filter by tenant_id
    ctx.DB().Find(ctx.Context(), &users, "tenant_id = ?", tenantID)
    return users, nil
}
```

### 2. Database Per Tenant

Each tenant has dedicated database:

```go
type Tenant struct {
    ID           string
    DatabaseName string // "tenant1_db", "tenant2_db"
}

func GetUsersFromTenantDB(ctx *context.Context, req struct{}) ([]User, error) {
    mt := ctx.Service("multitenancy").(*multitenancy.Driver)
    tenant, _ := mt.GetTenantFromRequest(ctx.Context(), ctx.Request())
    
    // Use named database adapter for this tenant
    db := ctx.DB(tenant.DatabaseName)
    
    var users []User
    db.Find(ctx.Context(), &users)
    return users, nil
}
```

### 3. Schema Per Tenant

Each tenant has dedicated schema within shared database:

```go
type Tenant struct {
    ID         string
    SchemaName string // "tenant1_schema", "tenant2_schema"
}

func GetUsersFromTenantSchema(ctx *context.Context, req struct{}) ([]User, error) {
    mt := ctx.Service("multitenancy").(*multitenancy.Driver)
    tenant, _ := mt.GetTenantFromRequest(ctx.Context(), ctx.Request())
    
    // Set schema for this query
    db := ctx.DB()
    db.Exec(ctx.Context(), "SET search_path TO "+tenant.SchemaName)
    
    var users []User
    db.Find(ctx.Context(), &users)
    return users, nil
}
```

## Implementing Tenant Store

### In-Memory Store (Development)

```go
type MemoryTenantStore struct {
    tenants map[string]*multitenancy.Tenant
    mu      sync.RWMutex
}

func (s *MemoryTenantStore) GetTenant(ctx context.Context, id string) (*multitenancy.Tenant, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    tenant, exists := s.tenants[id]
    if !exists {
        return nil, fmt.Errorf("tenant not found: %s", id)
    }
    return tenant, nil
}

func (s *MemoryTenantStore) GetTenantByDomain(ctx context.Context, domain string) (*multitenancy.Tenant, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    for _, tenant := range s.tenants {
        if tenant.Domain == domain {
            return tenant, nil
        }
    }
    return nil, fmt.Errorf("tenant not found for domain: %s", domain)
}

func (s *MemoryTenantStore) ListTenants(ctx context.Context) ([]*multitenancy.Tenant, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    tenants := make([]*multitenancy.Tenant, 0, len(s.tenants))
    for _, tenant := range s.tenants {
        if tenant.Active {
            tenants = append(tenants, tenant)
        }
    }
    return tenants, nil
}

func (s *MemoryTenantStore) CreateTenant(ctx context.Context, tenant *multitenancy.Tenant) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    s.tenants[tenant.ID] = tenant
    return nil
}

func (s *MemoryTenantStore) UpdateTenant(ctx context.Context, tenant *multitenancy.Tenant) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if _, exists := s.tenants[tenant.ID]; !exists {
        return fmt.Errorf("tenant not found: %s", tenant.ID)
    }
    s.tenants[tenant.ID] = tenant
    return nil
}

func (s *MemoryTenantStore) DeleteTenant(ctx context.Context, id string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    delete(s.tenants, id)
    return nil
}
```

### Database Store (Production)

```go
type DatabaseTenantStore struct {
    db contracts.Database
}

func (s *DatabaseTenantStore) GetTenant(ctx context.Context, id string) (*multitenancy.Tenant, error) {
    var tenant multitenancy.Tenant
    err := s.db.First(ctx, &tenant, "id = ?", id)
    if err != nil {
        return nil, err
    }
    return &tenant, nil
}

func (s *DatabaseTenantStore) GetTenantByDomain(ctx context.Context, domain string) (*multitenancy.Tenant, error) {
    var tenant multitenancy.Tenant
    err := s.db.First(ctx, &tenant, "domain = ?", domain)
    if err != nil {
        return nil, err
    }
    return &tenant, nil
}

func (s *DatabaseTenantStore) ListTenants(ctx context.Context) ([]*multitenancy.Tenant, error) {
    var tenants []*multitenancy.Tenant
    err := s.db.Find(ctx, &tenants, "active = ?", true)
    return tenants, err
}

func (s *DatabaseTenantStore) CreateTenant(ctx context.Context, tenant *multitenancy.Tenant) error {
    return s.db.Create(ctx, tenant)
}

func (s *DatabaseTenantStore) UpdateTenant(ctx context.Context, tenant *multitenancy.Tenant) error {
    return s.db.Update(ctx, tenant)
}

func (s *DatabaseTenantStore) DeleteTenant(ctx context.Context, id string) error {
    return s.db.Delete(ctx, &multitenancy.Tenant{}, "id = ?", id)
}
```

## Complete Example with Middleware

```go
package main

import (
    "fmt"
    "net/http"
    
    "github.com/madcok-co/unicorn/contrib/multitenancy"
    "github.com/madcok-co/unicorn/core/pkg/app"
    "github.com/madcok-co/unicorn/core/pkg/context"
    httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
)

func main() {
    // Initialize tenant store
    tenantStore := &MemoryTenantStore{
        tenants: map[string]*multitenancy.Tenant{
            "acme": {
                ID:     "acme",
                Name:   "Acme Corp",
                Domain: "acme",
                Active: true,
                Features: map[string]bool{
                    "advanced-analytics": true,
                    "api-access":         true,
                },
            },
            "contoso": {
                ID:     "contoso",
                Name:   "Contoso Ltd",
                Domain: "contoso",
                Active: true,
                Features: map[string]bool{
                    "advanced-analytics": false,
                    "api-access":         true,
                },
            },
        },
    }

    // Initialize multi-tenancy
    mt := multitenancy.NewDriver(&multitenancy.Config{
        Strategy:   multitenancy.StrategyHeader,
        HeaderName: "X-Tenant-ID",
        Store:      tenantStore,
    })

    // Create app
    application := app.New(&app.Config{
        Name:       "saas-app",
        EnableHTTP: true,
        HTTP:       &httpAdapter.Config{Port: 8080},
    })

    // Register multi-tenancy service
    application.RegisterService(mt, "multitenancy")

    // Use multi-tenant middleware
    application.Use(MultiTenantMiddleware(mt))

    // Register handlers
    application.RegisterHandler(GetUsers).HTTP("GET", "/users").Done()
    application.RegisterHandler(CreateUser).HTTP("POST", "/users").Done()
    application.RegisterHandler(GetTenantInfo).HTTP("GET", "/tenant").Done()

    application.Start()
}

// Multi-tenant middleware
func MultiTenantMiddleware(mt *multitenancy.Driver) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Resolve tenant
            tenant, err := mt.GetTenantFromRequest(r.Context(), r)
            if err != nil {
                http.Error(w, "Invalid tenant", http.StatusUnauthorized)
                return
            }

            // Add tenant to context
            ctx := multitenancy.SetTenantInContext(r.Context(), tenant)
            ctx = multitenancy.SetTenantIDInContext(ctx, tenant.ID)

            // Continue with tenant in context
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// Handlers
type User struct {
    ID       string
    TenantID string
    Name     string
    Email    string
}

func GetUsers(ctx *context.Context, req struct{}) ([]User, error) {
    // Get tenant ID from context
    tenantID, ok := multitenancy.GetTenantIDFromContext(ctx.Context())
    if !ok {
        return nil, fmt.Errorf("tenant not in context")
    }

    // Query with tenant isolation
    var users []User
    ctx.DB().Find(ctx.Context(), &users, "tenant_id = ?", tenantID)

    return users, nil
}

type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func CreateUser(ctx *context.Context, req CreateUserRequest) (*User, error) {
    tenantID, _ := multitenancy.GetTenantIDFromContext(ctx.Context())

    user := &User{
        TenantID: tenantID,
        Name:     req.Name,
        Email:    req.Email,
    }

    ctx.DB().Create(ctx.Context(), user)
    return user, nil
}

func GetTenantInfo(ctx *context.Context, req struct{}) (*multitenancy.Tenant, error) {
    tenant, ok := multitenancy.GetTenantFromContext(ctx.Context())
    if !ok {
        return nil, fmt.Errorf("tenant not in context")
    }

    return tenant, nil
}
```

## Tenant Management

### Create New Tenant

```go
func CreateTenant(ctx *context.Context, req CreateTenantRequest) (*multitenancy.Tenant, error) {
    mt := ctx.Service("multitenancy").(*multitenancy.Driver)
    
    tenant := &multitenancy.Tenant{
        ID:     req.ID,
        Name:   req.Name,
        Domain: req.Subdomain,
        Features: map[string]bool{
            "api-access": true,
        },
    }
    
    err := mt.CreateTenant(ctx.Context(), tenant)
    if err != nil {
        return nil, err
    }
    
    return tenant, nil
}
```

### Update Tenant

```go
func UpdateTenant(ctx *context.Context, req UpdateTenantRequest) (*multitenancy.Tenant, error) {
    mt := ctx.Service("multitenancy").(*multitenancy.Driver)
    
    tenant, err := mt.GetTenant(ctx.Context(), req.ID)
    if err != nil {
        return nil, err
    }
    
    tenant.Name = req.Name
    tenant.Features = req.Features
    
    err = mt.UpdateTenant(ctx.Context(), tenant)
    return tenant, err
}
```

## Testing

```bash
# Run tests
cd contrib/multitenancy
go test -v

# Test with coverage
go test -v -cover
```

## Best Practices

1. **Always Filter by Tenant ID** - Never forget tenant isolation in queries
2. **Validate Tenant Early** - Use middleware to validate tenant on every request
3. **Use Indexes** - Index tenant_id column for better performance
4. **Cache Tenant Data** - Cache tenant lookups to reduce database queries
5. **Monitor Per-Tenant** - Track metrics and usage per tenant
6. **Secure Tenant Data** - Ensure complete isolation between tenants
7. **Test Isolation** - Write tests to verify tenant data doesn't leak

## Common Pitfalls

❌ **Forgetting Tenant Filter**
```go
// BAD - No tenant filter!
var users []User
db.Find(ctx, &users)
```

✅ **Always Include Tenant Filter**
```go
// GOOD - Tenant isolated
var users []User
db.Find(ctx, &users, "tenant_id = ?", tenantID)
```

❌ **Hardcoded Tenant ID**
```go
// BAD - Hardcoded
users, _ := db.Find(ctx, &users, "tenant_id = 'acme'")
```

✅ **Dynamic Tenant ID**
```go
// GOOD - From context
tenantID, _ := multitenancy.GetTenantIDFromContext(ctx)
users, _ := db.Find(ctx, &users, "tenant_id = ?", tenantID)
```

## License

MIT License - see LICENSE file for details
