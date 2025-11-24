package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/madcok-co/unicorn/core/pkg/app"
	ucontext "github.com/madcok-co/unicorn/core/pkg/context"

	httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
)

// ============ DTOs ============

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type CreateOrderRequest struct {
	UserID    string  `json:"user_id"`
	ProductID string  `json:"product_id"`
	Amount    float64 `json:"amount"`
}

type Order struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	ProductID string  `json:"product_id"`
	Amount    float64 `json:"amount"`
	Status    string  `json:"status"`
}

type SendNotificationRequest struct {
	UserID  string `json:"user_id"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

type NotificationResult struct {
	Success bool   `json:"success"`
	Channel string `json:"channel"`
}

// ============ User Service Handlers ============

func CreateUser(ctx *ucontext.Context, req CreateUserRequest) (*User, error) {
	// Business logic for creating user
	log := ctx.Logger()
	if log != nil {
		log.Info("creating user", "name", req.Name, "email", req.Email)
	}

	user := &User{
		ID:    "user-" + fmt.Sprintf("%d", ctx.Context().Value("request_id")),
		Name:  req.Name,
		Email: req.Email,
	}

	// In real app: save to DB
	// ctx.DB().Create(user)

	return user, nil
}

func GetUser(ctx *ucontext.Context) (*User, error) {
	userID := ctx.Request().Params["id"]

	// In real app: fetch from DB
	return &User{
		ID:    userID,
		Name:  "John Doe",
		Email: "john@example.com",
	}, nil
}

func ListUsers(ctx *ucontext.Context) ([]*User, error) {
	// In real app: fetch from DB with pagination
	return []*User{
		{ID: "1", Name: "Alice", Email: "alice@example.com"},
		{ID: "2", Name: "Bob", Email: "bob@example.com"},
	}, nil
}

// ============ Order Service Handlers ============

func CreateOrder(ctx *ucontext.Context, req CreateOrderRequest) (*Order, error) {
	log := ctx.Logger()
	if log != nil {
		log.Info("creating order", "user_id", req.UserID, "product_id", req.ProductID)
	}

	order := &Order{
		ID:        "order-123",
		UserID:    req.UserID,
		ProductID: req.ProductID,
		Amount:    req.Amount,
		Status:    "pending",
	}

	// In real app:
	// 1. Validate user exists
	// 2. Validate product exists
	// 3. Save order to DB
	// 4. Publish event to Kafka
	// ctx.Queue().Publish(ctx.Context(), "order.created", &contracts.Message{...})

	return order, nil
}

func GetOrder(ctx *ucontext.Context) (*Order, error) {
	orderID := ctx.Request().Params["id"]

	return &Order{
		ID:        orderID,
		UserID:    "user-1",
		ProductID: "product-1",
		Amount:    99.99,
		Status:    "completed",
	}, nil
}

func ProcessOrderEvent(ctx *ucontext.Context, req CreateOrderRequest) (*Order, error) {
	// Handler ini bisa di-trigger dari HTTP atau Kafka
	triggerType := ctx.Request().TriggerType
	fmt.Printf("Processing order from: %s\n", triggerType)

	return &Order{
		ID:        "processed-order",
		UserID:    req.UserID,
		ProductID: req.ProductID,
		Amount:    req.Amount,
		Status:    "processing",
	}, nil
}

// ============ Notification Service Handlers ============

func SendNotification(ctx *ucontext.Context, req SendNotificationRequest) (*NotificationResult, error) {
	log := ctx.Logger()
	if log != nil {
		log.Info("sending notification", "user_id", req.UserID, "type", req.Type)
	}

	// In real app: send actual notification (email, push, SMS)

	return &NotificationResult{
		Success: true,
		Channel: req.Type,
	}, nil
}

func ProcessNotificationEvent(ctx *ucontext.Context, req SendNotificationRequest) (*NotificationResult, error) {
	// Triggered from Kafka when order is completed
	fmt.Printf("Processing notification event for user: %s\n", req.UserID)

	return &NotificationResult{
		Success: true,
		Channel: "email",
	}, nil
}

// ============ Health Check (shared) ============

func HealthCheck(ctx *ucontext.Context) (map[string]string, error) {
	return map[string]string{
		"status":  "healthy",
		"service": "unicorn-multiservice",
	}, nil
}

func main() {
	// Parse command line flags
	var (
		servicesFlag = flag.String("services", "", "Comma-separated list of services to run (empty = all)")
		portFlag     = flag.Int("port", 8080, "HTTP port")
	)
	flag.Parse()

	// Create application
	application := app.New(&app.Config{
		Name:       "unicorn-multiservice",
		Version:    "1.0.0",
		EnableHTTP: true,
		HTTP: &httpAdapter.Config{
			Host: "0.0.0.0",
			Port: *portFlag,
		},
		// EnableKafka: true,  // Enable for Kafka triggers
	})

	// ============ Define Services ============

	// User Service - handles user management
	application.Service("user-service").
		Describe("Handles user registration and management").
		OnStart(func(ctx context.Context) error {
			fmt.Println("[user-service] Starting...")
			return nil
		}).
		OnStop(func(ctx context.Context) error {
			fmt.Println("[user-service] Stopping...")
			return nil
		}).
		Register(CreateUser).
		Named("create-user").
		HTTP("POST", "/users").
		Done().
		Register(GetUser).
		Named("get-user").
		HTTP("GET", "/users/:id").
		Done().
		Register(ListUsers).
		Named("list-users").
		HTTP("GET", "/users").
		Done()

	// Order Service - handles order processing
	application.Service("order-service").
		Describe("Handles order creation and processing").
		DependsOn("user-service"). // Order service depends on user service
		OnStart(func(ctx context.Context) error {
			fmt.Println("[order-service] Starting...")
			return nil
		}).
		Register(CreateOrder).
		Named("create-order").
		HTTP("POST", "/orders").
		// Kafka("order.create.command").  // Also trigger from Kafka
		Done().
		Register(GetOrder).
		Named("get-order").
		HTTP("GET", "/orders/:id").
		Done().
		Register(ProcessOrderEvent).
		Named("process-order-event").
		HTTP("POST", "/orders/process").
		// Kafka("order.events").  // Process Kafka events
		Done()

	// Notification Service - handles notifications
	application.Service("notification-service").
		Describe("Handles sending notifications").
		OnStart(func(ctx context.Context) error {
			fmt.Println("[notification-service] Starting...")
			return nil
		}).
		Register(SendNotification).
		Named("send-notification").
		HTTP("POST", "/notifications").
		Done().
		Register(ProcessNotificationEvent).
		Named("process-notification-event").
		// Kafka("notification.send").      // Trigger from Kafka
		// Cron("*/5 * * * *").             // Also run every 5 minutes
		HTTP("POST", "/notifications/process"). // Also available via HTTP
		Done()

	// Shared health check (part of all services via shared endpoint)
	application.RegisterHandler(HealthCheck).
		Named("health").
		HTTP("GET", "/health").
		Done()

	// ============ Start Application ============

	// Determine which services to run
	var servicesToRun []string
	if *servicesFlag != "" {
		servicesToRun = strings.Split(*servicesFlag, ",")
		for i := range servicesToRun {
			servicesToRun[i] = strings.TrimSpace(servicesToRun[i])
		}
	}

	// Print startup info
	fmt.Println("===========================================")
	fmt.Println("  Unicorn Multi-Service Example")
	fmt.Println("===========================================")
	fmt.Printf("Port: %d\n", *portFlag)

	if len(servicesToRun) > 0 {
		fmt.Printf("Services: %s\n", strings.Join(servicesToRun, ", "))
	} else {
		fmt.Println("Services: all")
		fmt.Println("\nRegistered services:")
		for _, svc := range application.Services().All() {
			fmt.Printf("  - %s: %s\n", svc.Name(), svc.Description())
			fmt.Printf("    Handlers: %d\n", len(svc.Handlers()))
		}
	}

	fmt.Println("\nEndpoints:")
	fmt.Println("  GET  /health              - Health check")
	fmt.Println("  GET  /users               - List users")
	fmt.Println("  GET  /users/:id           - Get user")
	fmt.Println("  POST /users               - Create user")
	fmt.Println("  GET  /orders/:id          - Get order")
	fmt.Println("  POST /orders              - Create order")
	fmt.Println("  POST /orders/process      - Process order event")
	fmt.Println("  POST /notifications       - Send notification")
	fmt.Println("  POST /notifications/process - Process notification event")
	fmt.Println("===========================================")

	// Run services
	if err := application.RunServices(servicesToRun...); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
