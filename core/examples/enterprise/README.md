# Enterprise SaaS Platform Example

A comprehensive real-world example demonstrating all enterprise features of the Unicorn framework working together.

## Features Demonstrated

This example showcases a multi-tenant SaaS platform with:

- ✅ **OAuth2/OIDC Authentication** - Google OAuth integration
- ✅ **RBAC Authorization** - Role-based access control with 5 roles and inheritance
- ✅ **Multi-Tenancy** - Subdomain-based tenant isolation
- ✅ **Configuration Management** - Viper-based config with hot reload
- ✅ **Pagination** - Both offset (v1) and cursor-based (v2) pagination
- ✅ **API Versioning** - Two API versions with deprecation support

## Architecture

```
Enterprise SaaS Platform
├── Multi-Tenancy (Subdomain Strategy)
│   ├── acme.myapp.com    → Acme Corporation (Enterprise Plan)
│   └── techcorp.myapp.com → Tech Corp (Professional Plan)
│
├── Authentication (OAuth2/OIDC)
│   ├── Google OAuth Provider
│   ├── JWT Token Management
│   └── Refresh Token Flow
│
├── Authorization (RBAC)
│   ├── super_admin      → All permissions (*)
│   ├── tenant_admin     → Tenant management (projects:*, users:*, settings:*)
│   ├── project_manager  → Project operations (projects:read/create/update)
│   ├── developer        → Project read/update
│   └── viewer           → Read-only access (*:read)
│
├── API Versioning
│   ├── v1 → Offset pagination, basic operations
│   └── v2 → Cursor pagination, batch operations (v1 deprecated in 90 days)
│
└── Configuration
    ├── Default values
    ├── Environment variable override (APP_ prefix)
    └── Hot reload support
```

## Role Hierarchy

```
super_admin (*)
    │
tenant_admin (projects:*, users:*, settings:*)
    │
project_manager (projects:read/create/update, users:read)
    │
developer (projects:read/update)
    │
viewer (*:read)
```

## API Endpoints

### Authentication (v1)

```bash
POST   /api/v1/auth/login      # Initiate OAuth login
GET    /api/v1/auth/callback   # OAuth callback
```

### Projects (v1 - Offset Pagination)

```bash
GET    /api/v1/projects         # List projects (offset pagination)
POST   /api/v1/projects         # Create project (requires: projects:create)
GET    /api/v1/projects/:id     # Get project (requires: projects:read)
PUT    /api/v1/projects/:id     # Update project (requires: projects:update)
DELETE /api/v1/projects/:id     # Delete project (requires: projects:delete or ownership)
```

### Projects (v2 - Cursor Pagination + Batch)

```bash
GET    /api/v2/projects         # List projects (cursor pagination)
POST   /api/v2/projects/batch   # Batch update projects (requires: projects:update)
```

## Installation

```bash
# Clone the repository
git clone https://github.com/madcok-co/unicorn
cd unicorn/core/examples/enterprise

# Install dependencies
go mod download

# Set up environment variables (optional)
export APP_OAUTH_CLIENT_ID="your-google-client-id"
export APP_OAUTH_CLIENT_SECRET="your-google-client-secret"
export APP_OAUTH_REDIRECT_URL="http://localhost:8080/api/v1/auth/callback"

# Run the application
go run main.go
```

## Configuration

The application uses the following default configuration:

```yaml
app:
  name: "Enterprise SaaS Platform"
  version: "1.0.0"

http:
  host: "0.0.0.0"
  port: 8080

oauth:
  provider: "google"
  client_id: "your-client-id"          # Override with APP_OAUTH_CLIENT_ID
  client_secret: "your-client-secret"  # Override with APP_OAUTH_CLIENT_SECRET
  redirect_url: "http://localhost:8080/api/v1/auth/callback"

multitenancy:
  strategy: "subdomain"
  domain: "myapp.com"

pagination:
  default_limit: 20
  max_limit: 100
```

All configuration can be overridden with environment variables using the `APP_` prefix:

```bash
export APP_HTTP_PORT=9000
export APP_PAGINATION_DEFAULT_LIMIT=50
```

## Usage Examples

### 1. OAuth2 Authentication

```bash
# Step 1: Get OAuth authorization URL (manually construct)
# https://accounts.google.com/o/oauth2/v2/auth?
#   client_id=YOUR_CLIENT_ID
#   &redirect_uri=http://localhost:8080/api/v1/auth/callback
#   &response_type=code
#   &scope=openid%20email%20profile
#   &state=random_state_string

# Step 2: User authorizes and gets redirected with code
# The callback handler will exchange code for token

# Step 3: Login with code
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "code": "oauth_authorization_code",
    "state": "random_state_string"
  }'

# Response:
{
  "token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "1//0gN...",
  "expires_at": "2026-02-13T15:30:00Z",
  "user": {
    "id": "user-123",
    "email": "dev@acme.com",
    "name": "Developer User",
    "tenant_id": "acme",
    "roles": ["developer"],
    "created_at": "2026-02-13T14:30:00Z"
  }
}
```

### 2. Multi-Tenant Requests

Requests are automatically scoped to tenant based on subdomain:

```bash
# Acme Corporation tenant
curl -H "Host: acme.myapp.com" \
  http://localhost:8080/api/v1/projects

# Tech Corp tenant
curl -H "Host: techcorp.myapp.com" \
  http://localhost:8080/api/v1/projects
```

### 3. Create Project (with Authorization)

```bash
# As developer role (has projects:create via inheritance from project_manager)
curl -X POST http://acme.myapp.com:8080/api/v1/projects \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "name": "New Project",
    "description": "Project description"
  }'

# Response:
{
  "project": {
    "id": "proj-1708012800",
    "name": "New Project",
    "description": "Project description",
    "tenant_id": "acme",
    "owner_id": "user-dev",
    "status": "active",
    "created_at": "2026-02-13T14:30:00Z",
    "updated_at": "2026-02-13T14:30:00Z"
  }
}
```

### 4. List Projects with Pagination

**V1 - Offset Pagination:**

```bash
curl "http://acme.myapp.com:8080/api/v1/projects?page=1&limit=10&sort=created_at&order=desc"

# Response:
{
  "data": [...],
  "total": 45,
  "page": 1,
  "limit": 10,
  "total_pages": 5,
  "has_next": true,
  "has_prev": false,
  "links": {
    "self": "/api/v1/projects?page=1&limit=10",
    "next": "/api/v1/projects?page=2&limit=10",
    "prev": null,
    "first": "/api/v1/projects?page=1&limit=10",
    "last": "/api/v1/projects?page=5&limit=10"
  }
}
```

**V2 - Cursor Pagination:**

```bash
curl "http://acme.myapp.com:8080/api/v2/projects?limit=10&sort=created_at&order=desc"

# Response:
{
  "data": [...],
  "limit": 10,
  "has_next": true,
  "has_prev": false,
  "next_cursor": "eyJpZCI6InByb2otMTAiLCJ2YWx1ZSI6MTcwODAxMjgwMH0=",
  "prev_cursor": null,
  "links": {
    "self": "/api/v2/projects?limit=10",
    "next": "/api/v2/projects?cursor=eyJpZCI6InByb2otMTAiLCJ2YWx1ZSI6MTcwODAxMjgwMH0=&limit=10",
    "prev": null
  }
}
```

### 5. Batch Update (V2 Only)

```bash
curl -X POST http://acme.myapp.com:8080/api/v2/projects/batch \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "project_ids": ["proj-1", "proj-2", "proj-3"],
    "updates": {
      "status": "archived"
    }
  }'

# Response:
{
  "updated": 3,
  "total": 3
}
```

### 6. Authorization Examples

```bash
# As developer - CAN update own projects
curl -X PUT http://acme.myapp.com:8080/api/v1/projects/proj-1 \
  -H "Authorization: Bearer DEV_TOKEN" \
  -d '{"status": "inactive"}'
# ✅ Success (has projects:update via inheritance)

# As developer - CANNOT delete projects
curl -X DELETE http://acme.myapp.com:8080/api/v1/projects/proj-1 \
  -H "Authorization: Bearer DEV_TOKEN"
# ❌ 403 Forbidden (lacks projects:delete)

# As viewer - CAN read projects
curl http://acme.myapp.com:8080/api/v1/projects/proj-1 \
  -H "Authorization: Bearer VIEWER_TOKEN"
# ✅ Success (has *:read)

# As viewer - CANNOT create projects
curl -X POST http://acme.myapp.com:8080/api/v1/projects \
  -H "Authorization: Bearer VIEWER_TOKEN" \
  -d '{"name": "Test"}'
# ❌ 403 Forbidden (lacks projects:create)

# As tenant_admin - CAN do everything
curl -X DELETE http://acme.myapp.com:8080/api/v1/projects/proj-1 \
  -H "Authorization: Bearer ADMIN_TOKEN"
# ✅ Success (has projects:* which includes projects:delete)
```

## Permission Matrix

| Role            | Read | Create | Update | Delete | Batch Update |
|-----------------|------|--------|--------|--------|--------------|
| viewer          | ✅   | ❌     | ❌     | ❌     | ❌           |
| developer       | ✅   | ❌     | ✅     | ❌     | ❌           |
| project_manager | ✅   | ✅     | ✅     | ❌     | ✅           |
| tenant_admin    | ✅   | ✅     | ✅     | ✅     | ✅           |
| super_admin     | ✅   | ✅     | ✅     | ✅     | ✅           |

**Note:** Developers can delete their own projects (owner check), but not others' projects.

## API Versioning & Deprecation

The application demonstrates API versioning with deprecation:

```bash
# V1 endpoints return deprecation headers
curl -i http://acme.myapp.com:8080/api/v1/projects

# Response headers:
HTTP/1.1 200 OK
Deprecation: Wed, 14 May 2026 14:30:00 GMT
Sunset: Wed, 14 May 2026 14:30:00 GMT
Link: </api/v2/projects>; rel="successor-version"
```

**Migration Path:**
1. V1 is currently active but deprecated (90 days)
2. V2 is the current recommended version
3. After deprecation date, V1 may return 410 Gone

**Key Differences:**
- V1: Offset pagination (page/limit)
- V2: Cursor pagination (cursor/limit) + batch operations

## Configuration Hot Reload

The application supports hot configuration reload:

```bash
# Create config file
echo 'pagination:
  default_limit: 50
  max_limit: 200' > config.yaml

# Update config file while app is running
echo 'pagination:
  default_limit: 30' > config.yaml

# Application log will show:
# Config changed: pagination.default_limit = 30
```

## Testing Different Roles

The example includes pre-configured demo users:

```go
// In your tests, use these user IDs
userAdmin := "user-admin"    // Role: tenant_admin
userPM := "user-pm"          // Role: project_manager
userDev := "user-dev"        // Role: developer
```

To test different roles, modify the `getCurrentIdentity()` function to return different user IDs.

## Production Considerations

This is a **demonstration example**. For production use, you should:

1. **Database Integration**
   - Replace mock data functions with real database queries
   - Use `ctx.DB()` with GORM driver
   - Implement proper tenant isolation in queries

2. **JWT Token Validation**
   - Implement proper JWT middleware
   - Validate tokens on every protected endpoint
   - Extract user identity from JWT claims

3. **Error Handling**
   - Add proper error types and codes
   - Implement error middleware
   - Return structured error responses

4. **Caching**
   - Cache tenant configurations
   - Cache role permissions
   - Use Redis for session management

5. **Logging & Monitoring**
   - Add structured logging with context
   - Implement distributed tracing
   - Add metrics collection

6. **Rate Limiting**
   - Add per-tenant rate limits
   - Implement API key management
   - Add request throttling

7. **Security**
   - Implement CSRF protection
   - Add input sanitization
   - Enable CORS with proper origin checking
   - Use HTTPS in production

8. **Testing**
   - Add unit tests for each handler
   - Integration tests for auth flow
   - E2E tests for complete workflows

## File Structure

```
enterprise/
├── main.go              # Main application with all features
└── README.md            # This file
```

## Related Documentation

- [OAuth2 Authentication](../../../contrib/auth/oauth2/README.md)
- [RBAC Authorization](../../../contrib/authz/rbac/README.md)
- [Multi-Tenancy](../../../contrib/multitenancy/README.md)
- [Configuration Management](../../../contrib/config/README.md)
- [Pagination](../../../contrib/pagination/README.md)
- [API Versioning](../../../contrib/versioning/README.md)

## License

MIT License
