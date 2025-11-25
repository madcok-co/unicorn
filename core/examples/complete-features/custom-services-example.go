package main

import (
	"fmt"
	"log"
	"time"

	httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
	"github.com/madcok-co/unicorn/core/pkg/app"
	"github.com/madcok-co/unicorn/core/pkg/context"

	loggerAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/logger"
)

// ============================================================
// EXAMPLE: Custom Service Injection
// ============================================================
// This example demonstrates how to inject your own custom
// services/interfaces into handlers via context
// ============================================================

// ============================================================
// Define Your Custom Services (Business Logic)
// ============================================================

// EmailService - Custom service interface
type EmailService interface {
	SendEmail(to, subject, body string) error
	SendTemplateEmail(to, template string, data map[string]interface{}) error
}

// emailService - Implementation
type emailService struct {
	from   string
	apiKey string
}

func NewEmailService(from, apiKey string) EmailService {
	return &emailService{
		from:   from,
		apiKey: apiKey,
	}
}

func (s *emailService) SendEmail(to, subject, body string) error {
	// In real app: call SendGrid, Mailgun, etc.
	fmt.Printf("📧 Sending email to %s: %s\n", to, subject)
	return nil
}

func (s *emailService) SendTemplateEmail(to, template string, data map[string]interface{}) error {
	fmt.Printf("📧 Sending template email '%s' to %s\n", template, to)
	return nil
}

// PaymentService - Custom payment service interface
type PaymentService interface {
	ProcessPayment(amount float64, currency, method string) (*PaymentResult, error)
	RefundPayment(transactionID string) error
}

type PaymentResult struct {
	TransactionID string
	Status        string
	Amount        float64
	ProcessedAt   time.Time
}

// paymentService - Implementation
type paymentService struct {
	apiKey string
}

func NewPaymentService(apiKey string) PaymentService {
	return &paymentService{
		apiKey: apiKey,
	}
}

func (s *paymentService) ProcessPayment(amount float64, currency, method string) (*PaymentResult, error) {
	// In real app: call Stripe, PayPal, etc.
	fmt.Printf("💳 Processing payment: %.2f %s via %s\n", amount, currency, method)

	return &PaymentResult{
		TransactionID: fmt.Sprintf("txn_%d", time.Now().Unix()),
		Status:        "success",
		Amount:        amount,
		ProcessedAt:   time.Now(),
	}, nil
}

func (s *paymentService) RefundPayment(transactionID string) error {
	fmt.Printf("💰 Refunding transaction: %s\n", transactionID)
	return nil
}

// NotificationService - Push notifications
type NotificationService interface {
	SendPushNotification(userID, title, message string) error
	SendSMS(phoneNumber, message string) error
}

type notificationService struct{}

func NewNotificationService() NotificationService {
	return &notificationService{}
}

func (s *notificationService) SendPushNotification(userID, title, message string) error {
	fmt.Printf("🔔 Push notification to %s: %s\n", userID, title)
	return nil
}

func (s *notificationService) SendSMS(phoneNumber, message string) error {
	fmt.Printf("📱 SMS to %s: %s\n", phoneNumber, message)
	return nil
}

// ============================================================
// Request/Response DTOs
// ============================================================

type CreateOrderRequest struct {
	UserID      string  `json:"user_id" validate:"required"`
	ProductID   string  `json:"product_id" validate:"required"`
	Quantity    int     `json:"quantity" validate:"required,gt=0"`
	TotalAmount float64 `json:"total_amount" validate:"required,gt=0"`
}

type Order struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	ProductID     string    `json:"product_id"`
	Quantity      int       `json:"quantity"`
	TotalAmount   float64   `json:"total_amount"`
	Status        string    `json:"status"`
	TransactionID string    `json:"transaction_id"`
	CreatedAt     time.Time `json:"created_at"`
}

// ============================================================
// Handlers Using Custom Services
// ============================================================

// CreateOrder - Uses multiple custom services
func CreateOrder(ctx *context.Context, req CreateOrderRequest) (*Order, error) {
	// Get custom services from context
	emailSvc := ctx.GetService("email").(EmailService)
	paymentSvc := ctx.GetService("payment").(PaymentService)
	notificationSvc := ctx.GetService("notification").(NotificationService)

	logger := ctx.Logger()

	// 1. Process payment
	paymentResult, err := paymentSvc.ProcessPayment(
		req.TotalAmount,
		"USD",
		"credit_card",
	)
	if err != nil {
		return nil, fmt.Errorf("payment failed: %w", err)
	}

	// 2. Create order
	order := &Order{
		ID:            fmt.Sprintf("order_%d", time.Now().Unix()),
		UserID:        req.UserID,
		ProductID:     req.ProductID,
		Quantity:      req.Quantity,
		TotalAmount:   req.TotalAmount,
		Status:        "confirmed",
		TransactionID: paymentResult.TransactionID,
		CreatedAt:     time.Now(),
	}

	// 3. Send confirmation email
	err = emailSvc.SendTemplateEmail(
		"user@example.com",
		"order_confirmation",
		map[string]interface{}{
			"order_id": order.ID,
			"amount":   order.TotalAmount,
		},
	)
	if err != nil {
		logger.Error("failed to send email", "error", err)
	}

	// 4. Send push notification
	err = notificationSvc.SendPushNotification(
		req.UserID,
		"Order Confirmed",
		fmt.Sprintf("Your order %s has been confirmed!", order.ID),
	)
	if err != nil {
		logger.Error("failed to send notification", "error", err)
	}

	logger.Info("order created successfully",
		"order_id", order.ID,
		"user_id", req.UserID,
		"amount", req.TotalAmount)

	return order, nil
}

// CancelOrder - Uses payment service for refund
func CancelOrder(ctx *context.Context) (map[string]interface{}, error) {
	orderID := ctx.Request().Params["id"]

	// Get services
	paymentSvc := ctx.GetService("payment").(PaymentService)
	emailSvc := ctx.GetService("email").(EmailService)

	// Mock: get transaction ID (in real app, fetch from DB)
	transactionID := "txn_123456"

	// Process refund
	err := paymentSvc.RefundPayment(transactionID)
	if err != nil {
		return nil, fmt.Errorf("refund failed: %w", err)
	}

	// Send cancellation email
	emailSvc.SendEmail(
		"user@example.com",
		"Order Cancelled",
		fmt.Sprintf("Your order %s has been cancelled and refunded.", orderID),
	)

	ctx.Logger().Info("order cancelled", "order_id", orderID)

	return map[string]interface{}{
		"message":  "Order cancelled successfully",
		"order_id": orderID,
		"refunded": true,
	}, nil
}

// SendOrderNotification - Uses notification service
func SendOrderNotification(ctx *context.Context) (map[string]interface{}, error) {
	orderID := ctx.Request().Params["id"]
	notificationType := ctx.Request().Query["type"] // push or sms

	notificationSvc := ctx.GetService("notification").(NotificationService)

	switch notificationType {
	case "sms":
		err := notificationSvc.SendSMS(
			"+1234567890",
			fmt.Sprintf("Your order %s is ready for pickup!", orderID),
		)
		if err != nil {
			return nil, err
		}
	default:
		err := notificationSvc.SendPushNotification(
			"user123",
			"Order Update",
			fmt.Sprintf("Your order %s has been shipped!", orderID),
		)
		if err != nil {
			return nil, err
		}
	}

	return map[string]interface{}{
		"message": "Notification sent",
		"type":    notificationType,
	}, nil
}

// ============================================================
// Factory-based Service Example
// ============================================================

// RequestLogger - Per-request service (created fresh for each request)
type RequestLogger struct {
	requestID string
	startTime time.Time
}

func NewRequestLogger(ctx *context.Context) (*RequestLogger, error) {
	return &RequestLogger{
		requestID: fmt.Sprintf("req_%d", time.Now().UnixNano()),
		startTime: time.Now(),
	}, nil
}

func (rl *RequestLogger) Log(message string) {
	fmt.Printf("[%s] %s (elapsed: %v)\n",
		rl.requestID,
		message,
		time.Since(rl.startTime))
}

// UseFactoryService - Handler using factory-based service
func UseFactoryService(ctx *context.Context) (map[string]interface{}, error) {
	// Get per-request service
	reqLogger := ctx.GetService("requestLogger").(*RequestLogger)

	reqLogger.Log("Processing request...")

	// Simulate some work
	time.Sleep(100 * time.Millisecond)

	reqLogger.Log("Request completed")

	return map[string]interface{}{
		"request_id": reqLogger.requestID,
		"elapsed":    time.Since(reqLogger.startTime).String(),
	}, nil
}

// ============================================================
// Main Application
// ============================================================

func runCustomServicesExample() {
	// Create application
	application := app.New(&app.Config{
		Name:       "custom-services-example",
		Version:    "1.0.0",
		EnableHTTP: true,
		HTTP: &httpAdapter.Config{
			Host: "0.0.0.0",
			Port: 8082,
		},
	})

	// Setup infrastructure
	logger := loggerAdapter.NewConsoleLogger()
	application.SetLogger(logger)

	// Register custom services (Singleton pattern - same instance for all requests)
	emailService := NewEmailService("noreply@example.com", "email-api-key")
	application.RegisterService("email", emailService)

	paymentService := NewPaymentService("payment-api-key")
	application.RegisterService("payment", paymentService)

	notificationService := NewNotificationService()
	application.RegisterService("notification", notificationService)

	// Register factory-based service (new instance per request)
	application.RegisterServiceFactory("requestLogger", func(ctx *context.Context) (any, error) {
		return NewRequestLogger(ctx)
	})

	// Register handlers
	application.RegisterHandler(CreateOrder).
		Named("create-order").
		HTTP("POST", "/orders").
		Done()

	application.RegisterHandler(CancelOrder).
		Named("cancel-order").
		HTTP("DELETE", "/orders/:id").
		Done()

	application.RegisterHandler(SendOrderNotification).
		Named("send-notification").
		HTTP("POST", "/orders/:id/notify").
		Done()

	application.RegisterHandler(UseFactoryService).
		Named("factory-example").
		HTTP("GET", "/factory").
		Done()

	// Startup hook
	application.OnStart(func() error {
		fmt.Println("🎯 Custom Services Example Started!")
		fmt.Println("\n📚 Available Endpoints:")
		fmt.Println("  POST   /orders                  - Create order (uses email, payment, notification)")
		fmt.Println("  DELETE /orders/:id              - Cancel order (uses payment, email)")
		fmt.Println("  POST   /orders/:id/notify?type= - Send notification (push|sms)")
		fmt.Println("  GET    /factory                 - Factory service example")
		fmt.Println()
		fmt.Println("🔧 Registered Custom Services:")
		fmt.Println("  ✓ EmailService (singleton)")
		fmt.Println("  ✓ PaymentService (singleton)")
		fmt.Println("  ✓ NotificationService (singleton)")
		fmt.Println("  ✓ RequestLogger (factory - new per request)")
		fmt.Println()
		fmt.Println("💡 Example Request:")
		fmt.Println(`  curl -X POST http://localhost:8082/orders \
    -H "Content-Type: application/json" \
    -d '{
      "user_id": "user123",
      "product_id": "prod456",
      "quantity": 2,
      "total_amount": 99.99
    }'`)
		fmt.Println()
		return nil
	})

	// Start
	log.Println("Starting Custom Services Example on http://localhost:8082")
	if err := application.Start(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
