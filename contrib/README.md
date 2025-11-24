# Unicorn Contrib - Official Driver Implementations

This directory contains official driver implementations for the Unicorn framework.

## Available Drivers

### Database

| Driver | Package | Underlying Library |
|--------|---------|-------------------|
| GORM | `contrib/database/gorm` | [gorm.io/gorm](https://gorm.io) |
| SQLX | `contrib/database/sqlx` | Coming soon |

### Cache

| Driver | Package | Underlying Library |
|--------|---------|-------------------|
| Redis | `contrib/cache/redis` | [go-redis/redis](https://github.com/redis/go-redis) |
| BigCache | `contrib/cache/bigcache` | Coming soon |

### Logger

| Driver | Package | Underlying Library |
|--------|---------|-------------------|
| Zap | `contrib/logger/zap` | [uber-go/zap](https://github.com/uber-go/zap) |
| Zerolog | `contrib/logger/zerolog` | Coming soon |

### Message Broker

| Driver | Package | Underlying Library |
|--------|---------|-------------------|
| Kafka | `contrib/broker/kafka` | [IBM/sarama](https://github.com/IBM/sarama) |
| RabbitMQ | `contrib/broker/rabbitmq` | Coming soon |
| NATS | `contrib/broker/nats` | Coming soon |

### Validator

| Driver | Package | Underlying Library |
|--------|---------|-------------------|
| Playground | `contrib/validator/playground` | [go-playground/validator](https://github.com/go-playground/validator) |

### Tracer

| Driver | Package | Underlying Library |
|--------|---------|-------------------|
| OpenTelemetry | `contrib/tracer/otel` | Coming soon |

### Metrics

| Driver | Package | Underlying Library |
|--------|---------|-------------------|
| Prometheus | `contrib/metrics/prometheus` | Coming soon |

## Installation

Install only the drivers you need:

```bash
# Core framework
go get github.com/madcok-co/unicorn/core

# Database
go get github.com/madcok-co/unicorn/contrib/database/gorm

# Cache
go get github.com/madcok-co/unicorn/contrib/cache/redis

# Logger
go get github.com/madcok-co/unicorn/contrib/logger/zap

# Message Broker
go get github.com/madcok-co/unicorn/contrib/broker/kafka

# Validator
go get github.com/madcok-co/unicorn/contrib/validator/playground
```

## Usage Examples

### Database (GORM)

```go
import (
    "github.com/madcok-co/unicorn/core"
    "github.com/madcok-co/unicorn/contrib/database/gorm"
    "gorm.io/driver/postgres"
    gormpkg "gorm.io/gorm"
)

// Create GORM connection
db, _ := gormpkg.Open(postgres.Open(dsn), &gormpkg.Config{})

// Create Unicorn driver
driver := gorm.NewDriver(db)

// Set in app
app := unicorn.New(&unicorn.Config{Name: "my-service"})
app.SetDB(driver)

// In handler
func handler(ctx *unicorn.Context) error {
    // Use the database
    ctx.DB().Create(&user)
    ctx.DB().FindByID(ctx.Context(), 1, &user)
    
    // Use query builder
    ctx.DB().Query().
        From("users").
        Where("status = ?", "active").
        OrderBy("created_at", "DESC").
        Limit(10).
        Get(ctx.Context(), &users)
    
    // Transaction
    ctx.DB().Transaction(ctx.Context(), func(tx contracts.Database) error {
        tx.Create(&order)
        tx.Update(&inventory)
        return nil
    })
    
    return ctx.JSON(200, users)
}
```

### Cache (Redis)

```go
import (
    "github.com/madcok-co/unicorn/contrib/cache/redis"
    goredis "github.com/redis/go-redis/v9"
)

// Create Redis client
rdb := goredis.NewClient(&goredis.Options{
    Addr: "localhost:6379",
})

// Create Unicorn driver
driver := redis.NewDriver(rdb, redis.WithPrefix("myapp"))
app.SetCache(driver)

// In handler
func handler(ctx *unicorn.Context) error {
    // Basic operations
    ctx.Cache().Set(ctx.Context(), "key", value, time.Hour)
    ctx.Cache().Get(ctx.Context(), "key", &result)
    
    // Remember pattern
    ctx.Cache().Remember(ctx.Context(), "users:active", time.Hour, func() (any, error) {
        return db.GetActiveUsers()
    }, &users)
    
    // Distributed lock
    lock, _ := ctx.Cache().Lock(ctx.Context(), "process:order", time.Minute)
    defer lock.Unlock(ctx.Context())
    
    // Tagged cache
    ctx.Cache().Tags("users", "api").Set(ctx.Context(), "user:1", user, time.Hour)
    ctx.Cache().Tags("users").Flush(ctx.Context()) // Flush all user-related cache
    
    return ctx.JSON(200, users)
}
```

### Logger (Zap)

```go
import (
    "github.com/madcok-co/unicorn/contrib/logger/zap"
)

// Create logger with default config
driver := zap.NewDriver()

// Or with custom config
driver := zap.NewDriverWithConfig(&zap.Config{
    Level:         "debug",
    Format:        "json",
    Output:        "stdout",
    AddCaller:     true,
    AddStacktrace: true,
    DefaultFields: map[string]any{
        "service": "my-service",
        "version": "1.0.0",
    },
})

app.SetLogger(driver)

// In handler
func handler(ctx *unicorn.Context) error {
    ctx.Logger().Info("Processing request", 
        "user_id", userID,
        "action", "create_order",
    )
    
    ctx.Logger().WithError(err).Error("Failed to process order")
    
    // Named sub-logger
    orderLogger := ctx.Logger().Named("orders")
    orderLogger.Debug("Order created", "order_id", order.ID)
    
    return ctx.JSON(200, order)
}
```

### Message Broker (Kafka)

```go
import (
    "github.com/madcok-co/unicorn/contrib/broker/kafka"
)

// Create Kafka driver
driver := kafka.NewDriver(&kafka.Config{
    Brokers: []string{"localhost:9092"},
    GroupID: "my-service",
})

// Connect
driver.Connect(context.Background())
app.SetBroker(driver)

// Publish message
func handler(ctx *unicorn.Context) error {
    msg := contracts.NewBrokerMessage("orders.created", orderJSON)
    msg.SetHeader("correlation_id", correlationID)
    
    ctx.Broker().Publish(ctx.Context(), "orders.created", msg)
    return ctx.JSON(201, order)
}

// Subscribe to messages (in app setup)
app.Handle(&unicorn.Handler{
    Name: "ProcessOrder",
    Triggers: []unicorn.Trigger{
        {Type: unicorn.TriggerMessage, Topic: "orders.created"},
    },
    Handler: func(ctx *unicorn.Context) error {
        // Process the message
        var order Order
        json.Unmarshal(ctx.Request().Body, &order)
        return processOrder(order)
    },
})
```

### Validator (Playground)

```go
import (
    "github.com/madcok-co/unicorn/contrib/validator/playground"
)

// Create validator
driver := playground.NewDriver()

// Or with custom config
driver := playground.NewDriverWithConfig(&playground.Config{
    UseJSONNames: true,
    Messages: map[string]string{
        "required": "{field} wajib diisi",
        "email":    "{field} harus email yang valid",
    },
})

app.SetValidator(driver)

// In handler
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required,min=3,max=100"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"required,gte=18"`
}

func handler(ctx *unicorn.Context) error {
    var req CreateUserRequest
    ctx.Bind(&req)
    
    if err := ctx.Validate(req); err != nil {
        return ctx.JSON(400, map[string]any{
            "errors": err.(contracts.ValidationErrors).ToMap(),
        })
    }
    
    return ctx.JSON(201, user)
}
```

## Contributing

Want to add a new driver? Follow these steps:

1. Create a new directory under the appropriate category
2. Implement the corresponding interface from `core/pkg/contracts`
3. Add tests
4. Update this README
5. Submit a PR

## License

MIT License - Same as the main Unicorn framework.
