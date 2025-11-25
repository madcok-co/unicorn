# Changelog

All notable changes to the Unicorn framework will be documented in this file.

## [Unreleased] - 2025-11-25

### Added

#### Core Framework Improvements
- **Go 1.22+ Method-Based Routing**: Updated HTTP adapter to use modern ServeMux with method-specific patterns (`GET /users`, `POST /users`)
- **Path Parameter Conversion**: Automatic conversion from Express-style (`:id`) to Go 1.22+ style (`{id}`) for seamless migration
- **Colorful Startup Logs**: Added comprehensive ANSI-colored console output showing:
  - Application banner with name and version
  - HTTP/HTTPS server status with address
  - Registered routes grouped by method
  - Message broker topics
  - Cron schedules
  - Graceful shutdown messages

#### Complete Example Suite
- **Three-Tier Example System**:
  - `main.go` (Basic) - Simple HTTP server with basic routing
  - `main_enhanced.go` (Enhanced) - Security features with JWT, validation, rate limiting
  - `main_complete.go` (Complete) - ALL framework features in one comprehensive example

#### Infrastructure & DevOps
- **Docker Compose Stack** with 8 services:
  - PostgreSQL 15 (database)
  - Redis 7 (cache)
  - Kafka + Zookeeper (message broker)
  - Kafka UI (message inspection)
  - Prometheus (metrics collection)
  - Grafana (visualization)
  - Jaeger (distributed tracing)
  - Adminer (database management)
- **Makefile**: Development workflow automation
- **Dockerfile**: Multi-stage build for optimized images
- **Environment Configuration**: `.env.example` template
- **Prometheus Configuration**: Scraping setup for application metrics

#### Testing
- **Comprehensive Test Suite** (`test-complete.sh`):
  - 20+ automated test cases
  - Health check and metrics validation
  - Complete authentication flow testing
  - CRUD operations for products
  - Order processing with payment integration
  - Error handling and edge cases
  - Concurrent request testing
  - Basic load testing
  - Color-coded output with timing

#### Documentation
- **Enhanced README** with:
  - Quick start guide (3 commands to get started)
  - Infrastructure service URLs and credentials
  - Complete API documentation
  - Code examples for key patterns
  - Feature comparison table
  - Progressive learning path

### Fixed
- **Route Conflict Panic**: Resolved duplicate route registration error when same path has multiple HTTP methods
- **Path Parameters 404**: Fixed routing for parameterized paths like `/users/:id`
- **Adapter Initialization**: Corrected function signatures for cache and logger adapters
- **Port Conflicts**: Added cleanup script to kill processes on occupied ports

### Changed
- **HTTP Adapter**: Refactored to use Go 1.22+ features exclusively
- **Parameter Extraction**: Switched from manual parsing to efficient `r.PathValue()` API
- **Logger Output**: Enhanced with ANSI colors and structured formatting

## Technical Details

### Route Registration (Before)
```go
// Old: Caused panic with same path, different methods
mux.HandleFunc(path, httpHandler)
```

### Route Registration (After)
```go
// New: Method-specific patterns
pattern := fmt.Sprintf("%s %s", method, convertedPath)
mux.HandleFunc(pattern, httpHandler)
```

### Path Parameter Conversion
```go
// Converts Express-style to Go 1.22+ style
/users/:id → /users/{id}
/posts/:postId/comments/:commentId → /posts/{postId}/comments/{commentId}
```

### Complete Feature Coverage in main_complete.go

**Security**:
- JWT authentication with token generation and validation
- Password hashing with bcrypt
- Request validation middleware
- Rate limiting

**Resilience**:
- Circuit breaker for external service calls
- Retry with exponential backoff
- Timeout management
- Graceful error handling

**Observability**:
- Prometheus metrics (counters, gauges, histograms)
- Structured logging
- Request tracing
- Health checks

**Messaging**:
- Event publishing and subscription
- Message handling with retry
- Topic-based routing

**Scheduling**:
- Cron jobs for periodic tasks
- Background job execution

**Architecture**:
- Dependency injection for custom services
- Clean architecture separation
- Domain-driven design patterns

## Infrastructure URLs

| Service | URL | Purpose |
|---------|-----|---------|
| Application | http://localhost:8080 | Main API |
| Prometheus | http://localhost:9090 | Metrics |
| Grafana | http://localhost:3000 | Dashboards |
| Jaeger | http://localhost:16686 | Tracing |
| Kafka UI | http://localhost:8090 | Message inspection |
| Adminer | http://localhost:8081 | Database management |

## Quick Start

```bash
# Setup environment
make setup

# Start all services
make up

# Run application
make run

# Run tests
make test

# Clean up
make clean
```

## Contributors
- Ahmad Fajar (@ahmadfajar)
