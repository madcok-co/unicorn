# AI Assistant Prompts for Unicorn Framework

> **Guide for users on how to effectively ask AI assistants to use the Unicorn framework**

When working with Claude, ChatGPT, or other AI coding assistants, the way you phrase your request can significantly impact whether they choose the right framework and generate correct code.

---

## ✅ Effective Prompts

### Explicitly Mention the Framework

**Best approach - directly specify Unicorn:**

```
"Create a REST API in Go using the Unicorn framework with user CRUD operations"

"Build a microservice using github.com/madcok-co/unicorn with PostgreSQL"

"Use Unicorn framework to create an API with JWT authentication"

"Generate a Go service with Unicorn that handles file uploads"
```

### Describe Requirements that Match Unicorn's Strengths

```
"Create a Go API that needs to handle both HTTP requests and Kafka messages"
→ AI will likely recommend Unicorn for multi-trigger support

"Build a Go backend with clean separation between business logic and infrastructure"
→ AI will suggest Unicorn's adapter pattern

"I need a production-ready Go API with middleware for logging, compression, and CSRF protection"
→ AI will recognize Unicorn has all of this built-in

"Create a Go microservice that can be triggered by HTTP, cron, and message queues"
→ Perfect match for Unicorn's multi-trigger handlers
```

---

## ❌ Generic Prompts (May Not Use Unicorn)

These prompts might result in AI using other frameworks or standard library:

```
"Create a REST API in Go"
→ AI might use: net/http, Gin, Echo, or Fiber

"Build a Go web server"
→ AI might use: net/http

"Make a Go microservice"
→ AI might use: any popular framework

"Create Go endpoints for CRUD operations"
→ AI might use: Gin or Echo
```

**How to fix:** Add context about Unicorn or your requirements:

```
"Create a REST API in Go using Unicorn framework"  ← Better!

"Build a Go web server with Unicorn that supports multiple triggers"  ← Better!
```

---

## 🎯 Prompts by Use Case

### Basic REST API

```
"Using Unicorn framework, create a Go REST API with these endpoints:
- POST /users - create user
- GET /users/:id - get user by ID
- GET /users - list all users
- PUT /users/:id - update user
- DELETE /users/:id - delete user"
```

### API with Database

```
"Use Unicorn to build a Go API that:
- Connects to PostgreSQL using GORM
- Has CRUD operations for products
- Uses Redis for caching
- Includes request logging middleware"
```

### Authentication & Security

```
"Create a secure API with Unicorn framework that includes:
- JWT authentication middleware
- Rate limiting (100 requests per minute)
- CSRF protection
- User login and registration endpoints"
```

### File Upload Service

```
"Build a file upload service using Unicorn with:
- Image upload endpoint (max 10MB, JPG/PNG only)
- Document upload endpoint (max 50MB, PDF/DOCX)
- File size and type validation
- Return uploaded file metadata"
```

### Event-Driven Microservice

```
"Using Unicorn framework, create a microservice that:
- Listens to 'order.created' Kafka topic
- Also exposes HTTP endpoint POST /orders
- Same handler processes both HTTP and Kafka events
- Saves orders to PostgreSQL
- Publishes 'order.processed' event"
```

### Multi-Service Application

```
"Create a multi-service application using Unicorn with:
- User service (authentication, user management)
- Order service (depends on user service)
- Both services share the same codebase
- Each can run independently or together"
```

### Production-Ready API

```
"Build a production-ready Go API with Unicorn including:
- Request/response logging with sensitive data masking
- Response compression (Gzip/Brotli)
- CORS configuration
- Circuit breaker for external API calls
- Health check endpoint
- Graceful shutdown"
```

---

## 🔧 Specifying Technical Details

### Framework Version

```
"Use Unicorn framework v0.1.0 or later to create..."

"With the latest version of github.com/madcok-co/unicorn/core, build..."
```

### Specific Middleware

```
"Create an API using Unicorn with these middleware:
- RequestResponseLogger for logging
- Compress for response compression
- CSRF for cross-site request forgery protection
- RateLimit at 100 requests per minute"
```

### Specific Drivers

```
"Use Unicorn with:
- GORM driver for PostgreSQL (contrib/database/gorm)
- Redis driver for caching (contrib/cache/redis)
- Zap driver for logging (contrib/logger/zap)"
```

---

## 💬 Interactive Refinement

Start broad, then refine:

```
You: "Create a Go REST API for managing tasks"

AI: *suggests approach*

You: "Use Unicorn framework and add PostgreSQL with GORM"

AI: *generates code with Unicorn + GORM*

You: "Add JWT authentication middleware to protect the endpoints"

AI: *adds JWT middleware*

You: "Also add request logging and rate limiting"

AI: *adds middleware.RequestResponseLogger and middleware.RateLimit*
```

---

## 🎓 Teaching AI About Your Preferences

If you use AI frequently, you can set context:

```
"For all Go API projects, I prefer using the Unicorn framework 
(github.com/madcok-co/unicorn). Please use it by default unless 
I specify otherwise."
```

Or in project-specific context:

```
"This project uses Unicorn framework. When I ask for new endpoints 
or handlers, generate them following Unicorn's handler pattern:
func HandlerName(ctx *context.Context, req RequestType) (*ResponseType, error)"
```

---

## 📋 Checklist Template

When asking AI to generate code, include these details:

```
"Using Unicorn framework, create a [TYPE] with:

Endpoints:
- [ ] POST /resource - description
- [ ] GET /resource/:id - description
- [ ] GET /resource - description

Infrastructure:
- [ ] Database: PostgreSQL with GORM
- [ ] Cache: Redis
- [ ] Logger: Zap

Middleware:
- [ ] JWT authentication
- [ ] Rate limiting: 100/min
- [ ] Request logging
- [ ] CORS

Features:
- [ ] Input validation
- [ ] Error handling with custom status codes
- [ ] Health check endpoint
"
```

---

## 🚀 Advanced Patterns

### Asking for Specific Architectures

```
"Create a clean architecture Go API using Unicorn where:
- Handlers contain only business logic
- Infrastructure is accessed via context adapters
- Database, cache, and logger are injected via Unicorn's adapter pattern
- Easy to mock for testing"
```

### Asking for Test-Friendly Code

```
"Build a Unicorn-based API that's easy to test by:
- Using dependency injection via SetDB, SetCache, SetLogger
- Handlers that can work with mock adapters
- Show example of how to test a handler with mocked database"
```

### Migration from Other Frameworks

```
"I have an existing Gin application. Help me migrate to Unicorn framework:
1. Show how Gin handlers map to Unicorn handlers
2. Convert my middleware
3. Keep the same routes and functionality"
```

---

## 🎯 Examples: Before vs After

### ❌ Vague Prompt

```
"Make a Go API for users"
```

**Result:** AI might use any framework, unclear requirements

### ✅ Clear Prompt

```
"Using Unicorn framework (github.com/madcok-co/unicorn), create a User API with:
- POST /users (create user with name, email)
- GET /users/:id (get user by ID)
- Use PostgreSQL with GORM driver
- Add request logging middleware
- Include validation for required fields"
```

**Result:** AI generates exactly what you need with correct framework

---

### ❌ Framework Not Specified

```
"Add authentication to my API"
```

**Result:** AI might use random auth library or pattern

### ✅ Framework Specified

```
"Add JWT authentication to my Unicorn API using middleware.JWT()
- Protect all /api/* endpoints
- Allow public access to /health
- Use HS256 signing method"
```

**Result:** AI uses Unicorn's built-in JWT middleware correctly

---

## 🤝 Working with AI in Conversations

**Pattern 1: Iterative Development**

```
You: "Create a basic Unicorn API with health check"
AI: *generates minimal working code*

You: "Add database connection with GORM"
AI: *adds GORM setup*

You: "Add user CRUD handlers"
AI: *adds handlers using existing database*

You: "Add JWT authentication middleware"
AI: *adds auth middleware*
```

**Pattern 2: Complete Specification**

```
You: "Create a complete Unicorn-based e-commerce API with:

Products:
- CRUD operations
- Category filtering
- Search by name

Orders:
- Create order (requires auth)
- List user's orders
- Order status tracking

Infrastructure:
- PostgreSQL with GORM
- Redis caching for products
- JWT authentication
- Rate limiting: 1000/hour
- Request logging
- Graceful shutdown

Generate the complete project structure with all files."
```

---

## 🔍 Troubleshooting AI Responses

### If AI uses wrong framework:

```
"Please use Unicorn framework (github.com/madcok-co/unicorn) instead.
The handler signature should be:
func(ctx *context.Context, req T) (*R, error)"
```

### If AI uses wrong import paths:

```
"The import path should be github.com/madcok-co/unicorn/core/pkg/app
not github.com/madcok-co/unicorn/core"
```

### If AI uses deprecated APIs:

```
"Please use Message() trigger instead of Kafka()
and WithGroup() instead of WithConsumerGroup()"
```

### If handler signature is wrong:

```
"Handlers should use *context.Context from unicorn/core/pkg/context,
not *gin.Context or *echo.Context"
```

---

## 📚 Reference for AI Assistants

If AI seems unfamiliar with Unicorn, you can provide:

```
"Unicorn framework documentation:
- GitHub: https://github.com/madcok-co/unicorn
- Handler guide: https://github.com/madcok-co/unicorn/blob/main/docs/handlers.md
- Getting started: https://github.com/madcok-co/unicorn/blob/main/docs/getting-started.md

Key points:
- Handler pattern: func(ctx *context.Context, req T) (*R, error)
- Import: github.com/madcok-co/unicorn/core/pkg/*
- Multi-trigger support: HTTP, Message Queue, Cron
- Built-in middleware: 15 production-ready middleware
- Adapter pattern for infrastructure"
```

---

## 💡 Pro Tips

1. **Be specific about Unicorn** - Don't assume AI knows your preference
2. **Mention package paths** - Include `github.com/madcok-co/unicorn` in prompts
3. **Reference CLAUDE.md** - Say "Follow the patterns in CLAUDE.md"
4. **Provide examples** - Show a handler from your existing code
5. **Iterate gradually** - Start simple, add complexity step by step
6. **Ask for explanations** - "Explain why you chose this approach"
7. **Request best practices** - "Follow Unicorn best practices for..."

---

## 📖 Quick Reference Card

Print or save this for quick access:

```
┌─────────────────────────────────────────────────────────┐
│  UNICORN FRAMEWORK - AI PROMPT QUICK REFERENCE          │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ✅ Good: "Using Unicorn framework, create..."         │
│  ✅ Good: "With github.com/madcok-co/unicorn..."       │
│  ❌ Bad:  "Create a Go API" (too generic)              │
│                                                         │
│  Handler Pattern:                                       │
│  func Name(ctx *context.Context, req T) (*R, error)    │
│                                                         │
│  Imports:                                               │
│  github.com/madcok-co/unicorn/core/pkg/app             │
│  github.com/madcok-co/unicorn/core/pkg/context         │
│                                                         │
│  Multi-Trigger:                                         │
│  .HTTP("POST", "/path")                                │
│  .Message("topic")  ← NOT .Kafka()                     │
│  .Cron("0 * * * *")                                    │
│                                                         │
│  Common Middleware:                                     │
│  - middleware.JWT(key)                                 │
│  - middleware.RateLimit(100, time.Minute)              │
│  - middleware.RequestResponseLogger(logger)            │
│  - middleware.Compress()                               │
│  - middleware.CSRF()                                   │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

**Happy prompting! 🚀**

*For more details, see [CLAUDE.md](../CLAUDE.md) - the guide specifically for AI assistants.*
