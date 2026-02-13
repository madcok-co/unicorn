# API Versioning

Production-ready API versioning support for Unicorn Framework with multiple versioning strategies.

## Features

- **Multiple Strategies**
  - URL-based (/v1/users, /v2/users)
  - Header-based (API-Version: v2)
  - Accept header (application/vnd.api+json;version=2)
  - Query parameter (?version=2)
  - Custom resolver

- **Semantic Versioning** - Parse and compare semantic versions (1.0.0, 2.1.3)
- **Version Validation** - Strict mode for supported versions only
- **Deprecation Support** - Mark versions as deprecated with sunset dates
- **Version Comparison** - Compare versions programmatically
- **Path Helpers** - Extract, remove, build versioned paths

## Installation

```bash
go get github.com/madcok-co/unicorn/contrib/versioning
```

## Quick Start

### URL-Based Versioning (Recommended)

```go
import (
    "github.com/madcok-co/unicorn/contrib/versioning"
    "github.com/madcok-co/unicorn/core/pkg/app"
    "github.com/madcok-co/unicorn/core/pkg/context"
)

func main() {
    // Initialize versioning manager
    vm := versioning.NewManager(&versioning.Config{
        Strategy:          versioning.StrategyURL,
        DefaultVersion:    "v1",
        SupportedVersions: []string{"v1", "v2"},
        StrictMode:        true,
    })

    // Create app
    application := app.New(&app.Config{
        Name:       "versioned-api",
        EnableHTTP: true,
    })

    // Register version-specific handlers
    application.RegisterHandler(GetUserV1).HTTP("GET", "/v1/users/:id").Done()
    application.RegisterHandler(GetUserV2).HTTP("GET", "/v2/users/:id").Done()

    application.Start()
}

// Version 1 handler
func GetUserV1(ctx *context.Context, req GetUserRequest) (*UserV1, error) {
    // V1 response format
    return &UserV1{
        ID:   req.ID,
        Name: "John Doe",
    }, nil
}

// Version 2 handler (enhanced)
func GetUserV2(ctx *context.Context, req GetUserRequest) (*UserV2, error) {
    // V2 response format with additional fields
    return &UserV2{
        ID:        req.ID,
        FirstName: "John",
        LastName:  "Doe",
        Email:     "john@example.com",
    }, nil
}
```

**Requests:**
```
GET /v1/users/123  → UserV1 format
GET /v2/users/123  → UserV2 format
```

## Versioning Strategies

### 1. URL-Based (Recommended)

Best for RESTful APIs, clear and explicit.

```go
vm := versioning.NewManager(&versioning.Config{
    Strategy:       versioning.StrategyURL,
    DefaultVersion: "v1",
})

// Requests:
// GET /v1/users
// GET /v2/posts
// GET /v2.1/items
```

**Pros:**
- Clear and visible
- Easy to test and debug
- Cacheable
- Works with all HTTP clients

**Cons:**
- Longer URLs
- URL structure changes

### 2. Header-Based

Best for APIs with many endpoints, cleaner URLs.

```go
vm := versioning.NewManager(&versioning.Config{
    Strategy:   versioning.StrategyHeader,
    HeaderName: "API-Version",
})

// Client sends:
// GET /users
// Headers:
//   API-Version: v2
```

**Pros:**
- Clean URLs
- No URL structure changes
- Easy to change version per request

**Cons:**
- Not visible in browser
- Requires header support
- Less cacheable

### 3. Accept Header (Media Type)

REST best practice, content negotiation.

```go
vm := versioning.NewManager(&versioning.Config{
    Strategy:   versioning.StrategyHeader,
    HeaderName: "Accept",
})

// Client sends:
// GET /users
// Headers:
//   Accept: application/vnd.api+json;version=2
```

### 4. Query Parameter

Simple but not recommended for production.

```go
vm := versioning.NewManager(&versioning.Config{
    Strategy:   versioning.StrategyQuery,
    QueryParam: "version",
})

// Requests:
// GET /users?version=2
```

### 5. Custom Resolver

For complex versioning logic.

```go
vm := versioning.NewManager(&versioning.Config{
    Strategy: versioning.StrategyCustom,
    Resolver: func(r *http.Request) (string, error) {
        // Custom logic: extract from JWT, subdomain, etc.
        token := extractJWT(r)
        claims := parseJWT(token)
        return claims["api_version"].(string), nil
    },
})
```

## Complete Example with Middleware

```go
package main

import (
    "fmt"
    "net/http"
    
    "github.com/madcok-co/unicorn/contrib/versioning"
    "github.com/madcok-co/unicorn/core/pkg/app"
    "github.com/madcok-co/unicorn/core/pkg/context"
    httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
)

func main() {
    // Initialize versioning
    vm := versioning.NewManager(&versioning.Config{
        Strategy:          versioning.StrategyURL,
        DefaultVersion:    "v1",
        SupportedVersions: []string{"v1", "v2", "v2.1"},
        StrictMode:        true,
    })

    // Create app
    application := app.New(&app.Config{
        Name:       "versioned-api",
        EnableHTTP: true,
        HTTP:       &httpAdapter.Config{Port: 8080},
    })

    // Register versioning service
    application.RegisterService(vm, "versioning")

    // Use versioning middleware
    application.Use(VersioningMiddleware(vm))

    // Register version-specific handlers
    application.RegisterHandler(ListUsersV1).HTTP("GET", "/v1/users").Done()
    application.RegisterHandler(ListUsersV2).HTTP("GET", "/v2/users").Done()
    application.RegisterHandler(CreateUserV1).HTTP("POST", "/v1/users").Done()
    application.RegisterHandler(CreateUserV2).HTTP("POST", "/v2/users").Done()

    application.Start()
}

// Versioning middleware
func VersioningMiddleware(vm *versioning.Manager) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Resolve version
            version, err := vm.ResolveVersion(r)
            if err != nil {
                http.Error(w, "Invalid API version", http.StatusBadRequest)
                return
            }

            // Add version to response headers
            w.Header().Set("API-Version", version)

            // Store version in context
            r = versioning.SetVersionInContext(r, version)

            // Check for deprecated versions
            if version == "v1" {
                versioning.AddDeprecationHeader(w, &versioning.DeprecationInfo{
                    Version:    "v1",
                    SunsetDate: "2025-12-31",
                    Message:    "API v1 is deprecated. Please migrate to v2.",
                    NewVersion: "v2",
                })
            }

            next.ServeHTTP(w, r)
        })
    }
}

// V1 handlers
type UserV1 struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

func ListUsersV1(ctx *context.Context, req struct{}) ([]UserV1, error) {
    return []UserV1{
        {ID: "1", Name: "John Doe"},
        {ID: "2", Name: "Jane Smith"},
    }, nil
}

func CreateUserV1(ctx *context.Context, req CreateUserV1Request) (*UserV1, error) {
    return &UserV1{
        ID:   "123",
        Name: req.Name,
    }, nil
}

// V2 handlers (enhanced schema)
type UserV2 struct {
    ID        string `json:"id"`
    FirstName string `json:"first_name"`
    LastName  string `json:"last_name"`
    Email     string `json:"email"`
}

func ListUsersV2(ctx *context.Context, req struct{}) ([]UserV2, error) {
    return []UserV2{
        {ID: "1", FirstName: "John", LastName: "Doe", Email: "john@example.com"},
        {ID: "2", FirstName: "Jane", LastName: "Smith", Email: "jane@example.com"},
    }, nil
}

func CreateUserV2(ctx *context.Context, req CreateUserV2Request) (*UserV2, error) {
    return &UserV2{
        ID:        "123",
        FirstName: req.FirstName,
        LastName:  req.LastName,
        Email:     req.Email,
    }, nil
}

type CreateUserV1Request struct {
    Name string `json:"name"`
}

type CreateUserV2Request struct {
    FirstName string `json:"first_name"`
    LastName  string `json:"last_name"`
    Email     string `json:"email"`
}
```

## Semantic Versioning

### Parse Versions

```go
v1, _ := versioning.ParseVersion("v1.2.3")
fmt.Printf("Major: %d, Minor: %d, Patch: %d\n", v1.Major, v1.Minor, v1.Patch)
// Output: Major: 1, Minor: 2, Patch: 3

v2, _ := versioning.ParseVersion("2.0")
fmt.Println(v2.String()) // "v2.0"
```

### Compare Versions

```go
v1, _ := versioning.ParseVersion("v1.0.0")
v2, _ := versioning.ParseVersion("v2.0.0")

if v2.IsGreaterThan(v1) {
    fmt.Println("v2 is newer than v1")
}

result := v1.Compare(v2)
// -1: v1 < v2
//  0: v1 == v2
//  1: v1 > v2
```

## Path Helpers

### Extract Version from Path

```go
version := versioning.ExtractVersionFromPath("/v2/users")
// Result: "v2"
```

### Remove Version from Path

```go
path := versioning.RemoveVersionFromPath("/v1/users/123")
// Result: "/users/123"
```

### Build Versioned Path

```go
path := versioning.BuildVersionedPath("v2", "/users")
// Result: "/v2/users"
```

## Deprecation Support

### Mark Version as Deprecated

```go
func VersioningMiddleware(vm *versioning.Manager) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            version, _ := vm.ResolveVersion(r)

            // Add deprecation headers for v1
            if version == "v1" {
                versioning.AddDeprecationHeader(w, &versioning.DeprecationInfo{
                    Version:    "v1",
                    SunsetDate: "2025-12-31",
                    Message:    "API v1 will be sunset on 2025-12-31",
                    NewVersion: "v2",
                })
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

**Response Headers:**
```
Deprecation: true
Sunset: 2025-12-31
Link: <v2>; rel="successor-version"
```

## Version Validation

### Strict Mode

Only allow supported versions:

```go
vm := versioning.NewManager(&versioning.Config{
    Strategy:          versioning.StrategyURL,
    SupportedVersions: []string{"v1", "v2"},
    StrictMode:        true, // Reject unsupported versions
})

// GET /v3/users → 400 Bad Request
// GET /v1/users → 200 OK
```

### Check if Version is Supported

```go
if vm.IsSupported("v2") {
    // Proceed
}
```

### Get Latest Version

```go
latest := vm.GetLatestVersion()
fmt.Println(latest) // "v2"
```

## Migration Strategies

### Gradual Migration

```go
// Support both v1 and v2 simultaneously
application.RegisterHandler(GetUserV1).HTTP("GET", "/v1/users/:id").Done()
application.RegisterHandler(GetUserV2).HTTP("GET", "/v2/users/:id").Done()

// Sunset v1 after migration period
```

### Version Forwarding

```go
func GetUserV1(ctx *context.Context, req GetUserRequest) (*UserV1, error) {
    // Forward v1 requests to v2 internally
    v2Response, err := GetUserV2(ctx, req)
    if err != nil {
        return nil, err
    }

    // Convert v2 response to v1 format
    return &UserV1{
        ID:   v2Response.ID,
        Name: v2Response.FirstName + " " + v2Response.LastName,
    }, nil
}
```

### Adapter Pattern

```go
// Shared business logic
func GetUser(ctx *context.Context, id string) (*User, error) {
    // Core logic
}

// V1 adapter
func GetUserV1(ctx *context.Context, req GetUserRequest) (*UserV1, error) {
    user, err := GetUser(ctx, req.ID)
    return adaptToV1(user), err
}

// V2 adapter
func GetUserV2(ctx *context.Context, req GetUserRequest) (*UserV2, error) {
    user, err := GetUser(ctx, req.ID)
    return adaptToV2(user), err
}
```

## Best Practices

### 1. Use Semantic Versioning

```go
// Good
v1.0.0 → v1.0.1 (patch: bug fixes)
v1.0.0 → v1.1.0 (minor: new features, backward compatible)
v1.0.0 → v2.0.0 (major: breaking changes)

// Avoid
v1 → v1.5 → v2
```

### 2. Document Breaking Changes

```go
// In API documentation
/*
v2.0.0 Breaking Changes:
- User.name split into first_name and last_name
- Date format changed from "DD/MM/YYYY" to ISO 8601
- Removed deprecated /users/search endpoint
*/
```

### 3. Provide Migration Period

```go
// Support old version for 6-12 months
vm := versioning.NewManager(&versioning.Config{
    SupportedVersions: []string{"v1", "v2"},
})

// Add sunset date
versioning.AddDeprecationHeader(w, &versioning.DeprecationInfo{
    Version:    "v1",
    SunsetDate: "2025-06-30",
    NewVersion: "v2",
})
```

### 4. Use URL Versioning for Public APIs

```go
// Clear and visible
GET /v1/users
GET /v2/users
```

### 5. Version at Major Boundaries Only

```go
// Good: Only major versions in URL
/v1/users
/v2/users

// Avoid: Minor versions in URL
/v1.1/users
/v1.2/users
```

## Testing

```bash
# Run tests
cd contrib/versioning
go test -v

# Run with coverage
go test -v -cover
```

## Common Patterns

### Version Detection Middleware

```go
func VersionMiddleware(vm *versioning.Manager) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            version, err := vm.ResolveVersion(r)
            if err != nil {
                version = vm.config.DefaultVersion
            }

            // Add to response
            w.Header().Set("API-Version", version)

            // Store in context
            r = versioning.SetVersionInContext(r, version)

            next.ServeHTTP(w, r)
        })
    }
}
```

### Version-Specific Logic

```go
func GetUser(ctx *context.Context, req GetUserRequest) (interface{}, error) {
    // Get version from context
    version, _ := versioning.GetVersionFromContext(ctx.Request())

    // Version-specific logic
    switch version {
    case "v1":
        return getUserV1(ctx, req)
    case "v2":
        return getUserV2(ctx, req)
    default:
        return nil, fmt.Errorf("unsupported version: %s", version)
    }
}
```

### API Version Endpoint

```go
func GetVersionInfo(ctx *context.Context, req struct{}) (map[string]interface{}, error) {
    return map[string]interface{}{
        "versions": []versioning.VersionInfo{
            {
                Version:    "v1",
                Status:     "deprecated",
                Deprecated: true,
                SunsetDate: "2025-12-31",
            },
            {
                Version: "v2",
                Status:  "stable",
            },
            {
                Version: "v3",
                Status:  "beta",
            },
        },
        "latest": "v2",
    }, nil
}
```

## Migration Checklist

- [ ] Choose versioning strategy (URL recommended)
- [ ] Set up version detection middleware
- [ ] Document breaking changes
- [ ] Implement new version handlers
- [ ] Add deprecation headers to old version
- [ ] Update client documentation
- [ ] Set sunset date (6-12 months)
- [ ] Monitor old version usage
- [ ] Notify users about migration
- [ ] Remove old version after sunset

## License

MIT License - see LICENSE file for details
