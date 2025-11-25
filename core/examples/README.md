# Unicorn Framework - Examples

Complete collection of examples demonstrating all features of the Unicorn framework.

## 📁 Examples Directory

### 🎯 Complete Features (`complete-features/`)
Progressive examples from basic to advanced:
- **main.go** - Basic HTTP server with routing
- **main_enhanced.go** - JWT auth + security features  
- **main_complete.go** - Circuit breaker + retry + metrics + message broker

[📖 View Documentation](./complete-features/README.md)

### 🗄️ Database (`database/`)
GORM integration with SQLite:
- ✅ Complete CRUD operations
- ✅ Transactions
- ✅ Relationships (1-to-Many, Many-to-Many)
- ✅ Complex queries (WHERE, LIKE, JOIN)
- ✅ Aggregations (COUNT, SUM, AVG, GROUP BY)
- ✅ Batch operations
- ✅ Pagination

**Quick start:**
```bash
cd database
go run main.go
```

[📖 View Documentation](./database/README.md)

### 📨 Kafka (`kafka/`)
Event-driven architecture with Apache Kafka:
- ✅ Event publishing to topics
- ✅ Consumer groups
- ✅ Order processing workflow
- ✅ Multiple topics (order, payment, inventory, notification)
- ✅ Docker Compose setup
- ✅ Kafka UI for monitoring

**Quick start:**
```bash
cd kafka
docker-compose up -d  # Start Kafka
go run main.go
```

[📖 View Documentation](./kafka/README.md)

### 🔌 gRPC (`grpc/`)
**Status:** Framework support coming soon

Current options:
- Manual integration with standard gRPC
- Use grpc-gateway for HTTP proxy
- Share Unicorn services with gRPC handlers

[📖 View Documentation](./grpc/README.md)

### 🚀 Multi-Service (`multiservice/`)
Multiple service orchestration example

---

## 🎓 Learning Path

### 1️⃣ Start Here: Basic
```bash
cd complete-features
go run main.go
```
**Learn:**
- HTTP routing
- Request/response handling
- Health checks
- Basic metrics

### 2️⃣ Add Security: Enhanced
```bash
cd complete-features
JWT_SECRET="your-secret-key" go run main_enhanced.go
```
**Learn:**
- JWT authentication
- Password hashing
- User registration/login
- Token verification

### 3️⃣ Production Ready: Complete
```bash
cd complete-features
JWT_SECRET="your-secret-key" go run main_complete.go
```
**Learn:**
- Circuit breaker pattern
- Retry with exponential backoff
- Message broker (pub/sub)
- Metrics collection
- Rate limiting

### 4️⃣ Database Integration
```bash
cd database
go run main.go
```
**Learn:**
- ORM with GORM
- Database transactions
- Complex queries
- Relationship management

### 5️⃣ Event-Driven Architecture
```bash
cd kafka
docker-compose up -d
go run main.go
```
**Learn:**
- Kafka integration
- Event publishing
- Consumer groups
- Async processing

---

## 📊 Feature Matrix

| Feature | Basic | Enhanced | Complete | Database | Kafka |
|---------|-------|----------|----------|----------|-------|
| HTTP REST API | ✅ | ✅ | ✅ | ✅ | ✅ |
| Routing | ✅ | ✅ | ✅ | ✅ | ✅ |
| JWT Auth | ❌ | ✅ | ✅ | ❌ | ❌ |
| Password Hashing | ❌ | ✅ | ✅ | ❌ | ❌ |
| Cache | ❌ | ✅ | ✅ | ❌ | ❌ |
| Circuit Breaker | ❌ | ❌ | ✅ | ❌ | ❌ |
| Retry Pattern | ❌ | ❌ | ✅ | ❌ | ❌ |
| Message Broker | ❌ | ❌ | ✅ | ❌ | ✅ |
| Metrics | Basic | Basic | Advanced | ❌ | ✅ |
| Rate Limiting | ❌ | ❌ | ✅ | ❌ | ❌ |
| Database (GORM) | ❌ | ❌ | ❌ | ✅ | ❌ |
| Transactions | ❌ | ❌ | ❌ | ✅ | ❌ |
| Kafka Events | ❌ | ❌ | ❌ | ❌ | ✅ |
| Consumer Groups | ❌ | ❌ | ❌ | ❌ | ✅ |

---

## 🚀 Quick Commands

### Build All Examples
```bash
# Basic
cd complete-features && go build main.go

# Enhanced
cd complete-features && go build main_enhanced.go

# Complete
cd complete-features && go build main_complete.go

# Database
cd database && go build main.go

# Kafka
cd kafka && go build main.go
```

### Run Tests
```bash
# Test all complete-features examples
cd complete-features && ./test-all.sh

# Test specific example
cd complete-features && go run main_complete.go
```

---

## 📖 API Examples

### Create User
```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"john","email":"john@example.com","password":"password123"}'
```

### Login
```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"john@example.com","password":"password123"}'
```

### Create Product
```bash
curl -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{"name":"Laptop","price":1299.99,"stock":10}'
```

### Create Order (Kafka Event)
```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user_123","product_id":"prod_456","amount":99.99,"quantity":2}'
```

---

## 🐛 Troubleshooting

### Port Already in Use
```bash
# Kill process on port 8080
lsof -ti:8080 | xargs kill -9
```

### Database Locked (SQLite)
```bash
# Remove database file
rm example.db
```

### Kafka Not Starting
```bash
# Check Docker
docker-compose ps

# View logs
docker-compose logs kafka
```

### Build Errors
```bash
# Clean and rebuild
go clean
go mod tidy
go build
```

---

## 🎯 Next Steps

1. **Try all examples** - Start with basic, progress to complete
2. **Read the code** - Understand patterns and best practices
3. **Modify examples** - Experiment with configurations
4. **Build your app** - Use examples as templates
5. **Contribute** - Add more examples or improve existing ones

---

## 📚 Learn More

- [Framework Documentation](../README.md)
- [Complete Feature List](./complete-features/FEATURES.md)
- [Architecture Guide](../docs/ARCHITECTURE.md)
- [API Reference](../docs/API.md)

---

## 🤝 Contributing

Found a bug or have an improvement? Please:
1. Open an issue
2. Submit a pull request
3. Share your use case

---

## 📝 License

MIT License - see [LICENSE](../../LICENSE) for details
