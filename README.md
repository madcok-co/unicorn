# 🦄 UNICORN Framework

**Universal Integration & Connection Orchestrator Runtime Node**

The fastest way to build production-ready Go microservices. Write your business logic, we handle everything else.

## ✨ Features

- 🎯 **Zero Boilerplate** - No main.go needed, just write services
- 🔌 **Multi-Trigger System** - HTTP, gRPC, Kafka, WebSocket, Cron, CLI, SQS, NSQ
- 🧩 **Plugin Architecture** - 20+ built-in plugins for payments, email, SMS, storage
- 🗄️ **Multi-Database** - Multiple PostgreSQL, MySQL connections with pooling
- 💾 **Multi-Redis** - Multiple Redis instances for cache, sessions, queues
- 🔒 **Middleware System** - Auth, rate limiting, CORS, logging, recovery
- ✅ **Validation & Binding** - Auto-bind requests to structs with validation
- 📊 **Observability** - Logging, metrics, tracing out of the box
- 🚀 **Production Ready** - Thread-safe, no resource leaks, A+ code quality

## 🚀 Quick Start

### 1. Install
```bash
go install github.com/madcok-co/unicorn/cmd/unicorn@latest
```

### 2. Create Service
```go
// services/hello.go
package services

import "github.com/madcok-co/unicorn"

type HelloService struct{}

func (s *HelloService) Handle(ctx *unicorn.Context, req interface{}) (interface{}, error) {
    return map[string]interface{}{
        "message": "Hello from UNICORN!",
    }, nil
}

func init() {
    unicorn.MustRegister(&unicorn.Definition{
        Name:        "hello",
        Handler:     &HelloService{},
        EnableHTTP:  true,
    })
}
```
### 3. Configure
```yaml
# config.yaml
app:
  name: my-app
  version: 1.0.0

server:
  http:
    enabled: true
    port: 8080

database:
  - name: primary
    driver: postgres
    host: localhost
    port: 5432
    username: postgres
    password: postgres
    database: mydb
```
### 4. Run
```bash
unicorn --config config.yaml
```
### 5. Test
```bash
curl http://localhost:8080/api/hello
```

### 📚 Documentation

Complete Guide
API Reference
Plugin Development
Examples

### 🏗️ Architecture
```bash
User App
├── config.yaml          ← Configuration only
└── services/            ← Business logic only
    └── your_service.go

UNICORN Framework (handles everything)
├── service.go           ← Service system
├── context.go           ← Request context
├── connection.go        ← Multi-database/Redis
├── plugin.go            ← Plugin system
├── binding.go           ← Request binding
├── validation.go        ← Validation
├── middleware/          ← Built-in middlewares
├── triggers/            ← Multi-trigger system
├── plugins/             ← Built-in plugins
└── cmd/unicorn/         ← CLI runner
```
## 🎯 Core Concepts
### Services
A service is a unit of business logic:
```go
type UserService struct{}

func (s *UserService) Handle(ctx *unicorn.Context, req interface{}) (interface{}, error) {
    // Your business logic here
    return result, nil
}
```
### Multi-Database
Access multiple databases easily:
```go
func (s *Service) Handle(ctx *unicorn.Context, req interface{}) (interface{}, error) {
    // Primary database
    db1, _ := ctx.GetDB("primary")

    // Analytics database
    db2, _ := ctx.GetDB("analytics")

    return result, nil
}
```
### Multi-Redis
Access multiple Redis instances easily:
```go
func (s *Service) Handle(ctx *unicorn.Context, req interface{}) (interface{}, error) {
    // Primary Redis
    redis1, _ := ctx.GetRedis("primary")

    // Analytics Redis
    redis2, _ := ctx.GetRedis("analytics")

    return result, nil
}
```
### Plugins
Use built-in integrations:
```go
func (s *PaymentService) Handle(ctx *unicorn.Context, req interface{}) (interface{}, error) {
    // Get Stripe plugin
    plugin, _ := ctx.UsePlugin("stripe")
    stripe := plugin.(*plugins.StripePlugin)

    // Create payment
    payment, _ := stripe.CreatePaymentIntent(10000, "usd", nil)

    return payment, nil
}
```
### Struct Binding & Validation
Auto-bind and validate requests:
```go
type CreateUserRequest struct {
    Name  string `bind:"name" validate:"required,min=3,max=50"`
    Email string `bind:"email" validate:"required,email"`
    Age   int    `bind:"age" validate:"required,gte=18,lte=120"`
}

func (s *Service) Handle(ctx *unicorn.Context, req interface{}) (interface{}, error) {
    var input CreateUserRequest

    // Auto-bind from URL params, query params, and body
    if err := unicorn.Bind(req.(map[string]interface{}), &input); err != nil {
        return nil, err
    }

    // Validate
    if err := unicorn.Validate(&input); err != nil {
        return nil, err
    }

    // Use validated input
    return result, nil
}
```
## 🔌 Triggers
UNICORN supports 8 trigger types:
TriggerDescriptionUse Case
