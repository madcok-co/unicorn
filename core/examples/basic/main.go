package main

import (
	"fmt"
	"log"

	httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
	"github.com/madcok-co/unicorn/core/pkg/app"
	"github.com/madcok-co/unicorn/core/pkg/context"
)

// ============ Request/Response DTOs ============

type CreateUserRequest struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// ============ Handlers ============
// User hanya perlu fokus ke business logic
// Semua infrastructure diakses via context

// HealthCheck - simple handler tanpa request body
func HealthCheck(ctx *context.Context) (*HealthResponse, error) {
	return &HealthResponse{
		Status:  "healthy",
		Version: "1.0.0",
	}, nil
}

// CreateUser - handler dengan request body
func CreateUser(ctx *context.Context, req CreateUserRequest) (*User, error) {
	// Akses infrastructure via context
	// db := ctx.DB()      // Database
	// cache := ctx.Cache() // Cache
	// log := ctx.Logger()  // Logger
	// queue := ctx.Queue() // Message Queue

	// Pure business logic
	user := &User{
		ID:    "user-123", // In real app: generate UUID
		Name:  req.Name,
		Email: req.Email,
	}

	// Example dengan database:
	// if err := db.Create(user); err != nil {
	//     return nil, fmt.Errorf("failed to create user: %w", err)
	// }

	// Example dengan cache:
	// cache.Set(ctx.Context(), "user:"+user.ID, user, time.Hour)

	// Example dengan logger:
	// log.Info("user created", "id", user.ID, "email", user.Email)

	// Example publish ke Kafka:
	// queue.Publish(ctx.Context(), "user.created", &contracts.Message{
	//     Body: []byte(`{"id": "` + user.ID + `"}`),
	// })

	return user, nil
}

// GetUser - handler dengan path parameter
func GetUser(ctx *context.Context) (*User, error) {
	// Get path parameter
	userID := ctx.Request().Params["id"]

	// In real app, fetch from database
	// var user User
	// if err := ctx.DB().FindByID(ctx.Context(), userID, &user); err != nil {
	//     return nil, err
	// }

	return &User{
		ID:    userID,
		Name:  "John Doe",
		Email: "john@example.com",
	}, nil
}

// ListUsers - handler untuk listing
func ListUsers(ctx *context.Context) ([]*User, error) {
	// Get query parameters
	// page := ctx.Request().Query["page"]
	// limit := ctx.Request().Query["limit"]

	// In real app, fetch from database with pagination
	users := []*User{
		{ID: "1", Name: "Alice", Email: "alice@example.com"},
		{ID: "2", Name: "Bob", Email: "bob@example.com"},
	}

	return users, nil
}

// ProcessUserEvent - handler untuk Kafka consumer
// Sama handler bisa di-trigger dari HTTP dan Kafka!
func ProcessUserEvent(ctx *context.Context, req CreateUserRequest) (*User, error) {
	// Cek trigger type jika perlu logic berbeda
	triggerType := ctx.Request().TriggerType

	fmt.Printf("Processing user event from: %s\n", triggerType)

	// Business logic sama untuk HTTP dan Kafka
	return &User{
		ID:    "processed-user",
		Name:  req.Name,
		Email: req.Email,
	}, nil
}

func main() {
	// Create application
	application := app.New(&app.Config{
		Name:       "unicorn-example",
		Version:    "1.0.0",
		EnableHTTP: true,
		HTTP: &httpAdapter.Config{
			Host: "0.0.0.0",
			Port: 8080,
		},
		// EnableKafka: true,  // Uncomment to enable Kafka
	})

	// Register handlers dengan berbagai trigger
	// Satu handler bisa punya multiple triggers!

	// Health check - hanya HTTP
	application.RegisterHandler(HealthCheck).
		Named("health").
		HTTP("GET", "/health").
		Done()

	// Create user - HTTP dan Kafka trigger
	application.RegisterHandler(CreateUser).
		Named("create-user").
		HTTP("POST", "/users").
		// Kafka("user.create.command").  // Uncomment untuk Kafka trigger
		Done()

	// Get user by ID - HTTP only
	application.RegisterHandler(GetUser).
		Named("get-user").
		HTTP("GET", "/users/:id").
		Done()

	// List users - HTTP only
	application.RegisterHandler(ListUsers).
		Named("list-users").
		HTTP("GET", "/users").
		Done()

	// Process user event - bisa dari HTTP dan Kafka
	application.RegisterHandler(ProcessUserEvent).
		Named("process-user").
		HTTP("POST", "/users/process").
		// Kafka("user.events").           // Uncomment untuk Kafka trigger
		// Cron("0 * * * *").              // Uncomment untuk Cron trigger (setiap jam)
		Done()

	// Startup hook
	application.OnStart(func() error {
		fmt.Println("Application starting...")
		return nil
	})

	// Shutdown hook
	application.OnStop(func() error {
		fmt.Println("Application shutting down...")
		return nil
	})

	// Start!
	log.Println("Starting Unicorn Example on http://localhost:8080")
	log.Println("Endpoints:")
	log.Println("  GET  /health        - Health check")
	log.Println("  GET  /users         - List users")
	log.Println("  GET  /users/:id     - Get user by ID")
	log.Println("  POST /users         - Create user")
	log.Println("  POST /users/process - Process user event")

	if err := application.Start(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
