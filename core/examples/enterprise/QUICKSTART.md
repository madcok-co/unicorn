# Quick Start Guide

Get the Enterprise SaaS Platform running in 5 minutes.

## Prerequisites

- Go 1.21+ installed
- (Optional) Docker and Docker Compose
- (Optional) jq for JSON formatting

## 🚀 Quick Start (Local)

### 1. Clone and Setup

```bash
# Navigate to the example directory
cd unicorn/core/examples/enterprise

# Copy environment file
cp .env.example .env

# Download dependencies
go mod download
```

### 2. (Optional) Setup OAuth2

Skip this step to run with mock authentication, or follow these steps for real OAuth:

```bash
# Open Google Cloud Console
open https://console.cloud.google.com/apis/credentials

# Create OAuth 2.0 Client ID
# - Application type: Web application
# - Authorized redirect URIs: http://localhost:8080/api/v1/auth/callback

# Update .env with your credentials
# GOOGLE_CLIENT_ID=your-client-id
# GOOGLE_CLIENT_SECRET=your-client-secret
```

### 3. Setup Local Hosts

Add these entries to your `/etc/hosts` file:

```bash
# Linux/Mac
sudo sh -c 'echo "127.0.0.1  acme.myapp.com" >> /etc/hosts'
sudo sh -c 'echo "127.0.0.1  techcorp.myapp.com" >> /etc/hosts'

# Or use make command
make setup-hosts
```

**Windows:** Edit `C:\Windows\System32\drivers\etc\hosts` and add:
```
127.0.0.1  acme.myapp.com
127.0.0.1  techcorp.myapp.com
```

### 4. Run the Application

```bash
# Using make
make run

# Or directly with go
go run main.go

# Or with hot reload (requires air)
make dev
```

You should see:
```
Starting Enterprise SaaS Platform v1.0.0 on 0.0.0.0:8080
```

### 5. Test the API

Open a new terminal and run the test script:

```bash
# Run comprehensive tests
./test.sh

# Or use curl manually
make test-curl
```

## 🐳 Quick Start (Docker)

### 1. Setup Environment

```bash
cp .env.example .env
# Edit .env with your OAuth credentials if needed
```

### 2. Start with Docker Compose

```bash
# Build and start all services
make docker-up

# Or manually
docker-compose up -d
```

This starts:
- Application server (port 8080)
- PostgreSQL database (port 5432)
- Redis cache (port 6379)
- Nginx reverse proxy (port 80)

### 3. View Logs

```bash
make docker-logs

# Or manually
docker-compose logs -f app
```

### 4. Test the API

```bash
./test.sh
```

### 5. Stop Services

```bash
make docker-down

# Or manually
docker-compose down
```

## 📝 Try It Out

### Test Multi-Tenancy

```bash
# Acme Corporation tenant
curl -H "Host: acme.myapp.com" http://localhost:8080/api/v1/projects

# Tech Corp tenant
curl -H "Host: techcorp.myapp.com" http://localhost:8080/api/v1/projects
```

### Test Pagination

```bash
# V1 - Offset pagination
curl "http://localhost:8080/api/v1/projects?page=1&limit=10"

# V2 - Cursor pagination
curl "http://localhost:8080/api/v2/projects?limit=10"
```

### Test CRUD Operations

```bash
# Create project
curl -X POST http://acme.myapp.com:8080/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{"name":"My Project","description":"Test project"}'

# Get project
curl http://acme.myapp.com:8080/api/v1/projects/proj-1

# Update project
curl -X PUT http://acme.myapp.com:8080/api/v1/projects/proj-1 \
  -H "Content-Type: application/json" \
  -d '{"status":"inactive"}'

# Delete project
curl -X DELETE http://acme.myapp.com:8080/api/v1/projects/proj-1
```

### Test Batch Operations (V2)

```bash
curl -X POST http://acme.myapp.com:8080/api/v2/projects/batch \
  -H "Content-Type: application/json" \
  -d '{
    "project_ids": ["proj-1", "proj-2"],
    "updates": {"status": "archived"}
  }'
```

## 🧪 Testing with Postman

Import the Postman collection:

```bash
# Import this file in Postman
open postman_collection.json
```

Or manually:
1. Open Postman
2. Click "Import"
3. Select `postman_collection.json`
4. Set environment variables:
   - `baseUrl`: http://localhost:8080
   - `tenantHost`: acme.myapp.com

## 📊 Understanding the Features

### Multi-Tenancy

The application uses **subdomain-based tenant isolation**:
- `acme.myapp.com` → Acme Corporation tenant
- `techcorp.myapp.com` → Tech Corp tenant

Each tenant has isolated data and can have different:
- Feature flags
- Plans (Enterprise, Professional, etc.)
- User limits
- Configuration

### Authorization (RBAC)

5 pre-configured roles with inheritance:

```
super_admin (*)
    ↓
tenant_admin (projects:*, users:*, settings:*)
    ↓
project_manager (projects:read/create/update, users:read)
    ↓
developer (projects:read/update)
    ↓
viewer (*:read)
```

### API Versioning

Two API versions with different features:

| Feature | V1 | V2 |
|---------|----|----|
| Pagination | Offset (page/limit) | Cursor (cursor/limit) |
| Batch Operations | ❌ | ✅ |
| Status | Deprecated (90 days) | Current |

### Configuration

Configuration hierarchy (highest priority first):
1. Environment variables (`APP_` prefix)
2. Configuration files (`config.yaml`, `config.json`)
3. Default values

Hot reload enabled - change config files without restarting!

## 🔧 Customization

### Add Your Own Tenant

```go
mt.CreateTenant(&multitenancy.Tenant{
    ID:     "yourcompany",
    Name:   "Your Company Name",
    Active: true,
    Metadata: map[string]interface{}{
        "plan": "premium",
        "max_users": 200,
    },
})
```

### Add Custom Roles

```go
authz.CreateRole("custom_role", []string{
    "projects:read",
    "projects:create",
    "reports:read",
})
```

### Change Pagination Limits

```bash
# In .env or environment
export APP_PAGINATION_DEFAULT_LIMIT=50
export APP_PAGINATION_MAX_LIMIT=500
```

## 🐛 Troubleshooting

### Application won't start

```bash
# Check if port 8080 is available
lsof -i :8080

# Kill process if needed
kill -9 <PID>
```

### Can't access subdomain hosts

```bash
# Verify hosts file entries
cat /etc/hosts | grep myapp.com

# Test DNS resolution
ping acme.myapp.com

# If ping doesn't work, check firewall
```

### OAuth not working

```bash
# Check environment variables
env | grep GOOGLE

# Verify redirect URL matches exactly
echo $APP_OAUTH_REDIRECT_URL
# Should match: http://localhost:8080/api/v1/auth/callback
```

### Docker containers not starting

```bash
# Check Docker logs
docker-compose logs

# Rebuild containers
docker-compose down
docker-compose build --no-cache
docker-compose up -d
```

## 📚 Next Steps

1. **Read the full README**: See [README.md](./README.md) for detailed documentation
2. **Explore the code**: Understand how enterprise features work together
3. **Extend the example**: Add your own features (webhooks, notifications, etc.)
4. **Production setup**: Review production considerations in README.md

## 🎯 Common Use Cases

### Multi-Tenant SaaS

This example is perfect for:
- B2B SaaS platforms
- White-label solutions
- Enterprise applications
- API-as-a-Service

### Key Features Ready

- ✅ Tenant isolation
- ✅ OAuth2 authentication
- ✅ Role-based permissions
- ✅ API versioning
- ✅ Pagination at scale
- ✅ Configuration management
- ✅ Health checks
- ✅ Metrics ready

## 📞 Need Help?

- **Documentation**: See [README.md](./README.md)
- **Framework Docs**: See main [Unicorn README](../../../README.md)
- **Issues**: Report at [GitHub Issues](https://github.com/madcok-co/unicorn/issues)

---

**Now you're ready to build enterprise-grade APIs with Unicorn! 🦄**
